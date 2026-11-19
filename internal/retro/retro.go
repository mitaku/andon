// Package retro は振り返り（ADR-0028 決めたこと 8）の入力を集める（andon retro collect）。
//
// 読むだけで、台帳にもセッションログにも書かない。台帳のファイル（issue.yaml・log.jsonl・library/ の記録）を直に読み、
// 期間に片付いた担当の課題ごとに記録を1件に束ね（レビュアーの子は親へ）、期間に人が打った文をセッションログから抜き、
// 前の振り返りが切った直しのうちマージから 14 日以内のものを並べ、マージ後のこと（マージ commit の checks・revert の PR）を
// GitHub から読む。出力の文字列は全部、秘密の形に合うものを伏せてから書く。形は docs/contract.md §20。
package retro

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/contract"
	"andon/internal/launch"
	"andon/internal/ledger"
	"andon/internal/pkgversion"
	"andon/internal/sessionlog"
)

// Usage は andon retro の使い方。
const Usage = "usage: andon retro collect [--days N]                       振り返りの入力を JSON に集めて出す（読むだけ。docs/contract.md §20）\n" +
	"       andon retro patterns <id> [--days N] <patterns.json|->  パターンの一覧を検査して library/retro.json に書く（振り返りの担当。§21）\n" +
	"       andon retro apply <id> [--dry-run]                      パターンの一覧の fix を直す課題に切る（Chief of Staff。§21）\n" +
	"collect: 直近 N 日（既定 7）に片付いた担当の課題ごとに、起動の記録・結果の記録・ゲートの判定と理由・レビュアーの指摘（子の課題を親に束ねる）・\n" +
	"見送り・人の手直し・見込みと実働・Remember・For AGENTS.md・日報・マージ後のこと（GitHub から）・当て直しの印を1件にまとめる。\n" +
	"加えて、期間に人が端末で打った文（Claude Code のセッションログから）と、前の振り返りが切った直しのうちマージから 14 日以内のものを出す。\n" +
	"秘密の形に合う文字列は伏せてから出す。\n" +
	"patterns: collect と同じ入力を集め直し、根拠が別々の2件以上か・入力にあるか・前の振り返りより後のものがあるか・\n" +
	"skill と文面の直しに上の段で止められない理由があるか・振り返り自身を直さないか・全部に判定があり fix が 2 件までか・前の直しの見立てが揃うかを確かめる。\n" +
	"apply: library/retro.json を同じ窓で照らし直し、fix ごとに直す課題（parent は振り返りの課題）を1件足す。\n" +
	"直す先が Andon のパッケージで、その repo をこのプロジェクトが持たなければ、人の課題にする。記録は library/retro-applied.json。\n" +
	"projects/<slug>/ か担当の worktree で打つ（KANBAN_ROOT でも向けられる）。\n"

// 既定と上限。
const (
	defaultDays = 7  // 読む期間（ADR-0028 決めたこと 13）
	maxDays     = 90 // セッションログは 30 日で消えるので、それより長く読んでも人の文は出ない
	fixDays     = 14 // 前の直しを出す期間（決めたこと 6・受け入れ条件 18）
	retroFile   = "retro.json"
	retroName   = "retro" // 振り返りのルーチンの名前（issue.yaml の routine）
	journalFile = "journal.md"
)

// Options は集める入力。
type Options struct {
	Root string    // プロジェクトの根（kanban/ の親）
	Now  time.Time // 期間の終わり
	Days int       // 期間の日数
	// GH は gh を打つ（引数は gh の後ろ）。nil なら GitHub を読まない（マージ後のことは null、skipped に1行）。
	GH func(args ...string) ([]byte, error)
	// ClaudeDir は Claude Code の設定の置き場（projects/ の親）。空なら人が打った文を抜かない。
	ClaudeDir string
	// Package はパッケージの置き場（applied.json を読む）。空なら読まない。
	Package string
	// Exclude は前の振り返りに数えない課題（パターンの一覧を書く・当てる振り返りの課題自身）。
	Exclude string
}

// Input は andon retro collect の出力（schema/retro.schema.json）。
type Input struct {
	Project       string     `json:"project"`
	At            string     `json:"at"`
	Since         string     `json:"since"`
	Until         string     `json:"until"`
	PreviousRetro *PrevRetro `json:"previous_retro"`
	Issues        []Bundle   `json:"issues"`
	Typed         []Typed    `json:"typed"`
	Fixes         []Fix      `json:"fixes"`
	Redacted      int        `json:"redacted"`
	Skipped       []Skip     `json:"skipped"`
}

// PrevRetro は前の振り返りの課題（片付いたもののうち最後）。
type PrevRetro struct {
	Issue  string `json:"issue"`
	DoneAt string `json:"done_at"`
}

// Bundle は片付いた担当の課題1件に束ねた記録。根拠として指す id は課題の id。
type Bundle struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Profile          string           `json:"profile"`
	Repo             *string          `json:"repo"`
	DoneAt           string           `json:"done_at"`
	Library          string           `json:"library"`
	Estimate         *string          `json:"estimate"`
	AgentEstimate    *string          `json:"agent_estimate"`
	WorkedMinutes    *int             `json:"worked_minutes"`
	PR               *string          `json:"pr"`
	Launches         []map[string]any `json:"launches"`
	Outcome          map[string]any   `json:"outcome"`
	Gates            []Gate           `json:"gates"`
	Reviews          []Review         `json:"reviews"`
	Waived           []any            `json:"waived"`
	Corrections      []any            `json:"corrections"`
	Remember         []string         `json:"remember"`
	AgentsMD         []string         `json:"agents_md"`
	Journal          *string          `json:"journal"`
	AfterMerge       *AfterMerge      `json:"after_merge"`
	Reapplied        *bool            `json:"reapplied"`
	ReappliedReasons []string         `json:"reapplied_reasons"`
}

// Gate はゲートの記録1枚（head ごとの最後の判定）の写し。
type Gate struct {
	Head     string   `json:"head"`
	At       string   `json:"at"`
	Tier     string   `json:"tier"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
	Merged   bool     `json:"merged"`
}

// Review はレビュアーの子の課題1件（親に束ねる）。
type Review struct {
	ID       string         `json:"id"`
	Profile  string         `json:"profile"`
	Findings map[string]any `json:"findings"`
	Outcome  map[string]any `json:"outcome"`
}

// Skip は読み飛ばしたもの。
type Skip struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// Fix は前の振り返りが切った直しで、マージから fixDays 日以内のもの。
type Fix struct {
	Issue       string `json:"issue"`
	Title       string `json:"title"`
	Retro       string `json:"retro"`
	PR          string `json:"pr"`
	MergedAt    string `json:"merged_at"`
	IssuesAfter int    `json:"issues_after"`
}

// task は issue.yaml から読む所。
type task struct {
	id, title, status, profile, parent, who, repo, worktree, routine, estimate string
}

func (t *task) reviewer() bool {
	_, _, round := ledger.SplitRound(t.id)
	return round || strings.HasSuffix(t.profile, "-review")
}

// worker は束ねる課題か（担当の課題: レビュアー・人・Chief Engineer の課題でない）。
func (t *task) worker() bool {
	return !t.reviewer() && t.who != "person" && t.profile != "chief-engineer"
}

type reader struct {
	o       Options
	dir     string // <root>/kanban
	skipped []Skip
}

func (r *reader) skip(src, format string, a ...any) {
	r.skipped = append(r.skipped, Skip{Source: src, Reason: fmt.Sprintf(format, a...)})
}

func (r *reader) rel(p string) string {
	if s, err := filepath.Rel(r.o.Root, p); err == nil && !strings.HasPrefix(s, "..") {
		return s
	}
	return p
}

// Collect は振り返りの入力を集める。台帳が読めなければ err。ほかの読めないものは skipped に並べて続ける。
func Collect(o Options) (*Input, error) {
	r := &reader{o: o, dir: filepath.Join(o.Root, "kanban")}
	if st, err := os.Stat(r.dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("台帳（%s）が無い", r.dir)
	}
	until := o.Now.UTC()
	since := until.Add(-time.Duration(o.Days) * 24 * time.Hour)
	in := &Input{Project: filepath.Base(o.Root), At: until.Format(time.RFC3339), Since: since.Format(time.RFC3339),
		Until: until.Format(time.RFC3339), Issues: []Bundle{}, Typed: []Typed{}, Fixes: []Fix{}}

	tasks := r.tasks()
	byID := map[string]*task{}
	for _, t := range tasks {
		byID[t.id] = t
	}
	done := doneTimes(r.log())
	retros := map[string]bool{}
	for _, t := range tasks {
		if t.routine == retroName || exists(filepath.Join(r.dir, "issues", t.id, "library", retroFile)) {
			retros[t.id] = true
		}
	}
	// 前の振り返り: 期間の終わりまでに片付いた振り返りの課題のうち最後（自分自身は数えない）
	for id := range retros {
		at, ok := done[id]
		if !ok || byID[id].status != "done" || id == o.Exclude || at.After(until) {
			continue
		}
		if in.PreviousRetro == nil || at.After(mustTime(in.PreviousRetro.DoneAt)) ||
			(at.Equal(mustTime(in.PreviousRetro.DoneAt)) && id > in.PreviousRetro.Issue) {
			in.PreviousRetro = &PrevRetro{Issue: id, DoneAt: at.UTC().Format(time.RFC3339)}
		}
	}

	gh := newGitHub(r, o.GH)
	applied, appliedErr := readApplied(o.Package)
	for _, t := range tasks {
		at, ok := done[t.id]
		if t.status != "done" || !t.worker() || retros[t.id] || !ok || at.Before(since) || at.After(until) {
			continue
		}
		b := r.bundle(t, at, tasks)
		b.AfterMerge = gh.afterMerge(b)
		b.Reapplied, b.ReappliedReasons = reapplied(b, done, applied, appliedErr)
		in.Issues = append(in.Issues, b)
	}
	sort.SliceStable(in.Issues, func(i, j int) bool {
		if in.Issues[i].DoneAt != in.Issues[j].DoneAt {
			return in.Issues[i].DoneAt < in.Issues[j].DoneAt
		}
		return in.Issues[i].ID < in.Issues[j].ID
	})

	// 前の直し
	for _, t := range tasks {
		if !retros[t.parent] || t.reviewer() || t.status != "done" {
			continue
		}
		f, ok := r.fix(t, gh, until)
		if !ok {
			continue
		}
		merged := mustTime(f.MergedAt)
		for _, x := range tasks {
			if at, ok := done[x.id]; ok && x.status == "done" && x.worker() && !retros[x.id] && x.id != t.id && at.After(merged) {
				f.IssuesAfter++
			}
		}
		in.Fixes = append(in.Fixes, f)
	}
	sort.SliceStable(in.Fixes, func(i, j int) bool { return in.Fixes[i].MergedAt < in.Fixes[j].MergedAt })

	if o.ClaudeDir != "" {
		in.Typed = r.typed(tasks, since, until)
	}
	in.Skipped = r.skipped
	if in.Skipped == nil {
		in.Skipped = []Skip{}
	}
	return in, nil
}

// tasks は issues/*/issue.yaml を id の順に読む。読めない課題は飛ばす。
func (r *reader) tasks() []*task {
	ents, err := os.ReadDir(filepath.Join(r.dir, "issues"))
	if err != nil {
		r.skip("kanban/issues", "読めない")
		return nil
	}
	var out []*task
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(r.dir, "issues", e.Name(), "issue.yaml")
		b, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				r.skip(r.rel(path), "読めない")
			}
			continue
		}
		var raw ledger.Issue
		if yaml.Unmarshal(b, &raw) != nil || raw == nil {
			r.skip(r.rel(path), "issue.yaml が YAML でない")
			continue
		}
		t := &task{id: e.Name(), title: raw.Str("title"), status: raw.Str("status"), profile: raw.Str("profile"),
			parent: ledger.CanonID(raw.Str("parent")), who: raw.Str("who"), repo: raw.Str("repo"),
			worktree: raw.Path("worktree"), routine: raw.Str("routine"), estimate: raw.Str("estimate")}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return ledger.IDLess(out[i].id, out[j].id) })
	return out
}

func (r *reader) log() []ledger.Row {
	return ledger.At(r.o.Root).Log()
}

// doneTimes は課題ごとの最後の done の時刻（log.jsonl）。
func doneTimes(rows []ledger.Row) map[string]time.Time {
	out := map[string]time.Time{}
	for _, row := range rows {
		if row.Change == nil || row.Change["status"] != "done" {
			continue
		}
		if at, err := time.Parse(time.RFC3339, row.At); err == nil {
			out[row.ID] = at
		}
	}
	return out
}

// bundle は課題1件の記録を束ねる。
func (r *reader) bundle(t *task, doneAt time.Time, tasks []*task) Bundle {
	lib := filepath.Join(r.dir, "issues", t.id, "library")
	if a, err := filepath.Abs(lib); err == nil {
		lib = a
	}
	b := Bundle{ID: t.id, Title: t.title, Profile: t.profile, Repo: strPtr(t.repo), DoneAt: doneAt.UTC().Format(time.RFC3339),
		Library: lib, Estimate: strPtr(t.estimate), Gates: []Gate{}, Reviews: []Review{}, Waived: []any{},
		Remember: []string{}, AgentsMD: []string{}, ReappliedReasons: []string{}}
	b.Launches = r.launches(filepath.Join(lib, launch.File))
	b.Outcome = r.object(filepath.Join(lib, "outcome.json"))
	if b.Outcome != nil {
		if n, ok := b.Outcome["worked_minutes"].(float64); ok {
			m := int(n)
			b.WorkedMinutes = &m
		}
	}
	b.Gates = r.gates(filepath.Join(lib, "gate"))
	if rep := r.object(filepath.Join(lib, "report.json")); rep != nil {
		if s, ok := rep["pr"].(string); ok && s != "" {
			b.PR = &s
		}
		if e, ok := rep["estimate"].(map[string]any); ok {
			if s, ok := e["size"].(string); ok && s != "" {
				b.AgentEstimate = &s
			}
		}
		if w, ok := rep["waived"].([]any); ok {
			b.Waived = w
		}
		b.Remember = strings1(rep["remember"])
		b.AgentsMD = strings1(rep["agents_md"])
	}
	if c := r.object(filepath.Join(lib, "corrections.json")); c != nil {
		if xs, ok := c["corrections"].([]any); ok {
			b.Corrections = xs
		} else {
			r.skip(r.rel(filepath.Join(lib, "corrections.json")), "corrections が配列でない")
		}
	}
	if j, err := os.ReadFile(filepath.Join(lib, journalFile)); err == nil {
		s := string(j)
		b.Journal = &s
	} else if !errors.Is(err, fs.ErrNotExist) {
		r.skip(r.rel(filepath.Join(lib, journalFile)), "読めない")
	}
	for _, c := range tasks {
		if c.parent != t.id || !c.reviewer() {
			continue
		}
		clib := filepath.Join(r.dir, "issues", c.id, "library")
		b.Reviews = append(b.Reviews, Review{ID: c.id, Profile: c.profile,
			Findings: r.object(filepath.Join(clib, "findings.json")), Outcome: r.object(filepath.Join(clib, "outcome.json"))})
	}
	return b
}

// object は JSON の object のファイルを読む。無ければ nil。読めなければ skipped に足して nil。
func (r *reader) object(path string) map[string]any {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.skip(r.rel(path), "読めない")
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil || v == nil {
		r.skip(r.rel(path), "JSON の object でない")
		return nil
	}
	return v
}

// launches は起動の記録を読む（memory_index は大きいので落とす。全文は library の launches.jsonl）。無ければ nil。
func (r *reader) launches(path string) []map[string]any {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.skip(r.rel(path), "読めない")
		return nil
	}
	out := []map[string]any{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var v map[string]any
		if err := json.Unmarshal(line, &v); err != nil || v == nil {
			r.skip(r.rel(path)+":"+strconv.Itoa(n), "JSON の object でない")
			continue
		}
		delete(v, "memory_index")
		out = append(out, v)
	}
	return out
}

// gates はゲートの記録を判定の時刻の順に写す。読めない記録は飛ばす。
func (r *reader) gates(dir string) []Gate {
	out := []Gate{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, f := range files {
		v := r.object(f)
		if v == nil {
			continue
		}
		g := Gate{Reasons: []string{}}
		g.Head, _ = v["head"].(string)
		g.At, _ = v["at"].(string)
		g.Tier, _ = v["tier"].(string)
		g.Decision, _ = v["decision"].(string)
		g.Merged, _ = v["merged"].(bool)
		g.Reasons = strings1(v["reasons"])
		if g.Head == "" || g.Decision == "" {
			r.skip(r.rel(f), "head か decision が無い")
			continue
		}
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Head < out[j].Head
	})
	return out
}

// fix は前の振り返りが切った直し t の PR のマージの時刻を読み、now から fixDays 日以内なら返す。
func (r *reader) fix(t *task, gh *github, now time.Time) (Fix, bool) {
	lib := filepath.Join(r.dir, "issues", t.id, "library")
	pr, mergedAt := "", ""
	if o := r.object(filepath.Join(lib, "outcome.json")); o != nil {
		pr, _ = o["pr"].(string)
		mergedAt, _ = o["merged_at"].(string)
	}
	if pr == "" {
		if rep := r.object(filepath.Join(lib, "report.json")); rep != nil {
			pr, _ = rep["pr"].(string)
		}
	}
	if pr == "" {
		return Fix{}, false
	}
	if p, ok := gh.merged(pr); ok && p.MergedAt != "" {
		mergedAt = p.MergedAt
	}
	at, err := time.Parse(time.RFC3339, mergedAt)
	if err != nil || at.After(now) || now.Sub(at) > fixDays*24*time.Hour {
		return Fix{}, false
	}
	return Fix{Issue: t.id, Title: t.title, Retro: t.parent, PR: pr, MergedAt: at.UTC().Format(time.RFC3339)}, true
}

// applied はパッケージの当て直しの記録（ADR-0027。<置き場>/applied.json。最後の1回だけ）。
type applied struct {
	Commit    string `json:"commit"`
	AppliedAt string `json:"applied_at"`
}

// readApplied は当て直しの記録を読む。無ければ nil, nil。読めなければ err。
func readApplied(root string) (*applied, error) {
	if root == "" {
		return nil, nil
	}
	b, err := os.ReadFile(filepath.Join(root, "applied.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var a applied
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	if _, err := time.Parse(time.RFC3339, a.AppliedAt); err != nil {
		return nil, fmt.Errorf("applied_at が時刻でない: %q", a.AppliedAt)
	}
	return &a, nil
}

// reapplied は課題の途中でパッケージが当て直されたかの印（ADR-0028 決めたこと 14 の 5）。
//   - 当て直しの記録の applied_at が、最初の起動と片付けの間にある
//   - 起動の記録のパッケージの commit が、起動ごとに違う（当て直しの記録は最後の1回しか残らないので、前の当て直しはこちらで拾う）
//
// どちらでもなく、どちらかを確かめられない（起動の記録が無い・当て直しの記録が読めない）なら null。
func reapplied(b Bundle, _ map[string]time.Time, a *applied, aerr error) (*bool, []string) {
	reasons := []string{}
	yes, unknown := false, false
	var start time.Time
	commits := map[string]bool{}
	for _, l := range b.Launches {
		if s, ok := l["at"].(string); ok {
			if at, err := time.Parse(time.RFC3339, s); err == nil && (start.IsZero() || at.Before(start)) {
				start = at
			}
		}
		if p, ok := l["package"].(map[string]any); ok {
			if c, ok := p["commit"].(string); ok && c != "" {
				commits[c] = true
			}
		}
	}
	if len(commits) > 1 {
		yes = true
		reasons = append(reasons, fmt.Sprintf("起動の記録のパッケージの commit が %d 通りある", len(commits)))
	}
	end := mustTime(b.DoneAt)
	switch {
	case aerr != nil:
		unknown = true
		reasons = append(reasons, "当て直しの記録が読めない: "+aerr.Error())
	case start.IsZero():
		unknown = true
		reasons = append(reasons, "起動の記録が無いので、起動の時刻が分からない")
	case a != nil:
		at := mustTime(a.AppliedAt)
		if at.After(start) && at.Before(end) {
			yes = true
			reasons = append(reasons, "パッケージを当て直した時刻（"+a.AppliedAt+"）が、起動と片付けの間にある")
		}
	}
	if yes {
		v := true
		return &v, reasons
	}
	if unknown {
		return nil, reasons
	}
	v := false
	return &v, []string{}
}

func mustTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// strings1 は文字列の配列を読む（文字列でない要素は飛ばす）。無ければ空。
func strings1(v any) []string {
	out := []string{}
	xs, _ := v.([]any)
	for _, x := range xs {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Main は andon retro。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return 2 }
	if len(args) == 0 {
		return usage()
	}
	cmd, args := args[0], args[1:]
	days, id, src, dry := defaultDays, "", "", false
	if cmd == "patterns" || cmd == "apply" {
		if len(args) == 0 {
			return usage()
		}
		id, args = ledger.CanonID(args[0]), args[1:]
		if !idRe.MatchString(id) {
			fmt.Fprintf(stderr, "andon retro: id %q が不正\n", id)
			return 2
		}
	} else if cmd != "collect" {
		return usage()
	}
	for len(args) > 0 {
		switch {
		case args[0] == "--days" && len(args) >= 2 && cmd != "apply":
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 || n > maxDays {
				fmt.Fprintf(stderr, "andon retro: --days は 1〜%d の数（%q）\n", maxDays, args[1])
				return 2
			}
			days = n
			args = args[2:]
		case args[0] == "--dry-run" && cmd == "apply":
			dry = true
			args = args[1:]
		case cmd == "patterns" && src == "" && (args[0] == "-" || !strings.HasPrefix(args[0], "-")):
			src = args[0]
			args = args[1:]
		default:
			return usage()
		}
	}
	if cmd == "patterns" && src == "" {
		return usage()
	}
	root := os.Getenv("KANBAN_ROOT")
	if root == "" {
		l, err := ledger.Open()
		if err != nil {
			fmt.Fprintf(stderr, "andon retro: %v\n", err)
			return 1
		}
		root = l.Root
	}
	var err error
	switch cmd {
	case "collect":
		err = runCollect(root, days, stdout)
	case "patterns":
		err = runPatterns(root, id, days, src, stdin, stdout)
	case "apply":
		err = runApply(root, id, dry, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon retro: %v\n", err)
		return 1
	}
	return 0
}

func runCollect(root string, days int, stdout io.Writer) error {
	in, err := Collect(Options{Root: root, Now: time.Now(), Days: days, GH: GH,
		ClaudeDir: sessionlog.ClaudeDir(), Package: pkgversion.Home()})
	if err != nil {
		return err
	}
	b, _, err := Redacted(in)
	if err != nil {
		return err
	}
	_, err = stdout.Write(b)
	return err
}

// GH は gh を打つ。
func GH(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			msg := strings.TrimSpace(string(ee.Stderr))
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				msg = msg[:i]
			}
			return nil, fmt.Errorf("gh %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("gh %s: %v", args[0], err)
	}
	return out, nil
}

// Redacted は in を、秘密の形に合う文字列を伏せた JSON（2字下げ・末尾に改行）にする。伏せた数を in.Redacted に入れて返す。
// 伏せるのは書き出した JSON の文字列の中だけで（redact）、伏せたあとも JSON として読めることを確かめる。
func Redacted(in *Input) ([]byte, int, error) {
	in.Redacted = 0
	_, n := redact(contract.Marshal(in))
	in.Redacted = n
	out, _ := redact(contract.Marshal(in))
	if !json.Valid(out) {
		return nil, 0, errors.New("秘密を伏せたあとの出力が JSON でない（伏せる形の誤り）")
	}
	return out, n, nil
}
