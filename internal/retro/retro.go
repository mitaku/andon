// Package retro は振り返り（ADR-0028 決めたこと 8）の入力を集める（andon retro collect）。
//
// 読むだけで、kanban にもセッションログにも書かない。kanban のファイル（issue.yaml・log.jsonl・library/ の記録）を直に読み、
// 期間に片付いた担当の課題ごとに記録を1件に束ね（レビュアーの子は親へ）、期間にオーナーが打った文をセッションログから抜き、
// 前の振り返りが切った直しのうちマージから 14 日以内のものを並べ、マージ後のこと（マージ commit の checks・revert の PR）を
// GitHub から読む。出力の文字列は全部、秘密の形に合うものを伏せてから書く。形は docs/contract.md §20。
//
// 束は、前の振り返りが集めたあと（境目より後）に片付いたものを全文で、それより前のものを要約で出す（ADR-0039 決めたこと 6）。
// 新しい束が fullLimit 件を超えたら、新しい順に fullLimit 件を全文にし、残りを要約にする。全文の束1件は --issue で引ける。
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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/contract"
	"andon/internal/launch"
	"andon/internal/ledger"
	"andon/internal/otel"
	"andon/internal/pkgversion"
	"andon/internal/redact"
	"andon/internal/sessionlog"
	"andon/internal/trace"
)

const (
	CollectUsage = "usage: andon retro collect [--days N] [--issue <id>]\n" +
		"振り返りの入力を JSON に集めて出す（読むだけ。docs/contract.md §20）。\n" +
		"直近 N 日（既定 7）に片付いた担当の課題ごとに、起動の記録・結果の記録と内訳・gate の判定と理由・レビュアーの指摘（子の課題を親に束ねる）・\n" +
		"見送り・オーナーの手直し・見込みと実働・Remember・For AGENTS.md・日報・マージ後のこと（GitHub から）・当て直しの印を1件にまとめる。\n" +
		"前の振り返りが集めたあとに片付いた束は全文（新しい順に 100 件まで）、それより前の束は要約（タイトル・PR・つまずきの跡の見出し）で出す。\n" +
		"加えて、期間にオーナーが端末で打った文（Claude Code のセッションログから）と、前の振り返りが切った直しのうちマージから 14 日以内のもの\n" +
		"（前の振り返りが書いた再発の数と、マージの前後に片付いた担当の課題の数字つき）と、期間の Chief of Staff のトークン\n" +
		"（ハーネスの役ごとの生の記録から）を出す。--issue は期間の束1件を全文で出す。秘密の形に合う文字列は伏せてから出す。\n"
	PatternsUsage = "usage: andon retro patterns <id> [--days N] <patterns.json|->\n" +
		"パターンの一覧を検査して library/retro.json に書く（振り返りの担当。docs/contract.md §21）。\n" +
		"collect と同じ入力を集め直し、根拠が別々の2件以上か・入力にあるか・前の振り返りより後のものがあるか・\n" +
		"skill と文面の直しに上の段で止められない理由があるか・全部に判定があるか・前の直しの見立てが揃うかを確かめる（fix の数は確かめない）。\n"
	ApplyUsage = "usage: andon retro apply <id> [--dry-run]\n" +
		"パターンの一覧の fix を直す課題に切る（Chief of Staff。docs/contract.md §21）。\n" +
		"library/retro.json を同じ窓で照らし直し、fix ごとに一覧の順で直す課題（parent は振り返りの課題）を1件足す。\n" +
		"直す先が Andon のパッケージで、その repo をこのプロジェクトが持たなければ、オーナーの課題にする。記録は library/retro-applied.json。\n"
)

const (
	defaultDays = 7  // 読む期間（ADR-0028 決めたこと 13）
	maxDays     = 90 // セッションログは 30 日で消えるので、それより長く読んでもオーナーの文は出ない
	fixDays     = 14 // 前の直しを出す期間（決めたこと 6・受け入れ条件 18）
	retroFile   = "retro.json"
	retroName   = "retro"
	journalFile = "journal.md"
	lineMax     = 160
)

// fullLimit は全文で出す新しい束の数の上限（ADR-0039 決めたこと 6 の細部）。テストが小さくする。
var fullLimit = 100

type Options struct {
	Root string // プロジェクトの根（kanban/ の親）
	Now  time.Time
	Days int
	// GH は gh を打つ（引数は gh の後ろ）。nil なら GitHub を読まない（マージ後のことは null、skipped に1行）。
	GH func(args ...string) ([]byte, error)
	// ClaudeDir は Claude Code の設定の置き場（projects/ の親）。空ならオーナーが打った文を抜かない。
	ClaudeDir string
	// Package はパッケージの置き場（applied.json を読む）。空なら読まない。
	Package string
	// Exclude は前の振り返りに数えない課題（パターンの一覧を書く・当てる振り返りの課題自身）。
	Exclude string
	// Issue があれば、期間の束のうちその課題だけを全文で集める（--issue）。オーナーが打った文と前の直しは集めない。
	Issue string
	// Otel はハーネスの記録のプロジェクトの置き場（~/.local/state/andon/otel/<slug>。docs/contract.md §30）。
	// Chief of Staff の役ごとの生の記録だけを読む。空なら読まない（Chief of Staff のトークンは null）。
	Otel string
}

// Input は andon retro collect の出力（schema/retro.schema.json）。
type Input struct {
	Project       string       `json:"project"`
	At            string       `json:"at"`
	Since         string       `json:"since"`
	Until         string       `json:"until"`
	PreviousRetro *PrevRetroIn `json:"previous_retro"`
	Boundary      *string      `json:"boundary"`
	Issues        []Bundle     `json:"issues"`
	Summaries     []Summary    `json:"summaries"`
	Cut           *Cut         `json:"cut"`
	Typed         []Typed      `json:"typed"`
	Questions     []Question   `json:"questions"`
	Fixes         []Fix        `json:"fixes"`
	ChiefOfStaff  ChiefOfStaff `json:"chief_of_staff"`
	Redacted      int          `json:"redacted"`
	Skipped       []Skip       `json:"skipped"`
}

type PrevRetro struct {
	Issue  string `json:"issue"`
	DoneAt string `json:"done_at"`
}

// PrevRetroIn は入力の前の振り返り。Until は前の振り返りが集めた時刻（その retro.json の until）で、読めなければ nil。
type PrevRetroIn struct {
	Issue  string  `json:"issue"`
	DoneAt string  `json:"done_at"`
	Until  *string `json:"until"`
}

type Cut struct {
	New  int `json:"new"`
	Full int `json:"full"`
}

type Summary struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	DoneAt      string   `json:"done_at"`
	PR          *string  `json:"pr"`
	Findings    []string `json:"findings"`
	GateFixes   []string `json:"gate_fixes"`
	Corrections *bool    `json:"corrections"`
	Remember    int      `json:"remember"`
	Gaps        []GapRef `json:"gaps"`
}

// gapKinds は申告の種類（schema/report.schema.json の gaps の kind）。ほかの値の申告は要約に載せない（載せると出力が schema に落ちる）。
var gapKinds = map[string]bool{"missing": true, "wrong": true, "hard_to_find": true}

// GapRef は要約の束に載せる申告1件の種類と先（本文は載せない。ADR-0061 決めたこと 9）。
type GapRef struct {
	Kind  string `json:"kind"`
	Where string `json:"where"`
}

// Launch は起動の記録1行のうち、パターンを拾うのに使う所（ADR-0039 決めたこと 6 の細部）。全文は library の launches.jsonl。
type Launch struct {
	At      string  `json:"at"`
	Profile string  `json:"profile"`
	Model   *string `json:"model"`
	Commit  *string `json:"commit"`
}

type OutcomeView struct {
	Result        string `json:"result"`
	WorkedMinutes *int   `json:"worked_minutes"`
	Cost          *Cost  `json:"cost"`
}

type Cost struct {
	Sessions int            `json:"sessions"`
	Measured int            `json:"measured"`
	Tokens   map[string]int `json:"tokens"`
}

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
	Launches         []Launch         `json:"launches"`
	Outcome          *OutcomeView     `json:"outcome"`
	Breakdown        *trace.Breakdown `json:"breakdown"`
	Gates            []Gate           `json:"gates"`
	Reviews          []Review         `json:"reviews"`
	Waived           []any            `json:"waived"`
	Corrections      []any            `json:"corrections"`
	Remember         []string         `json:"remember"`
	AgentsMD         []string         `json:"agents_md"`
	Gaps             []any            `json:"gaps"`
	Journal          *string          `json:"journal"`
	AfterMerge       *AfterMerge      `json:"after_merge"`
	Reapplied        *bool            `json:"reapplied"`
	ReappliedReasons []string         `json:"reapplied_reasons"`

	mergeCommit string // outcome.json の merge_commit（GitHub がマージ commit を返さないとき、マージ後のことに使う）
}

type Gate struct {
	Head     string   `json:"head"`
	At       string   `json:"at"`
	Tier     string   `json:"tier"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
	Merged   bool     `json:"merged"`
}

type Review struct {
	ID       string         `json:"id"`
	Profile  string         `json:"profile"`
	Reason   *string        `json:"reason"`
	Findings map[string]any `json:"findings"`
	Outcome  map[string]any `json:"outcome"`
}

type Skip struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

type Fix struct {
	Issue       string `json:"issue"`
	Title       string `json:"title"`
	Retro       string `json:"retro"`
	PR          string `json:"pr"`
	MergedAt    string `json:"merged_at"`
	IssuesAfter int    `json:"issues_after"`
	// RecurredBefore は前の振り返りが書いた再発の数（その retro.json の fixes[].recurred）。マージが境目より後なら 0
	// （境目より前に数える出来事が無い）。どちらでもなければ nil で、担当は要約の束でも数える（ADR-0039 決めたこと 6 の細部）。
	RecurredBefore *int `json:"recurred_before"`
	// BeforeAfter はマージの前と後の数字（ADR-0060 決めたこと 17）。振り返りの担当が見立てを書く材料で、判定の入力にしない。
	BeforeAfter BeforeAfter `json:"before_after"`
}

type task struct {
	id, title, status, profile, parent, who, repo, worktree, routine, estimate string
}

func (t *task) reviewer() bool {
	_, _, round := ledger.SplitRound(t.id)
	return round || strings.HasSuffix(t.profile, "-review")
}

func (t *task) worker() bool {
	return !t.reviewer() && t.who != "person" && t.profile != "chief-engineer"
}

type reader struct {
	o       Options
	dir     string
	skipped []Skip
}

// skip は読めなかったものを 1 行足す。同じ記録を束と前後の数字の両方で読むので、同じ行は 1 つにする。
func (r *reader) skip(src, format string, a ...any) {
	s := Skip{Source: src, Reason: fmt.Sprintf(format, a...)}
	if !slices.Contains(r.skipped, s) {
		r.skipped = append(r.skipped, s)
	}
}

func (r *reader) rel(p string) string {
	if s, err := filepath.Rel(r.o.Root, p); err == nil && !strings.HasPrefix(s, "..") {
		return s
	}
	return p
}

// Collect は振り返りの入力を集める。kanban が読めなければ err。ほかの読めないものは skipped に並べて続ける。
func Collect(o Options) (*Input, error) {
	r := &reader{o: o, dir: filepath.Join(o.Root, "kanban")}
	if st, err := os.Stat(r.dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("kanban（%s）が無い", r.dir)
	}
	until := o.Now.UTC()
	since := until.Add(-time.Duration(o.Days) * 24 * time.Hour)
	in := &Input{Project: filepath.Base(o.Root), At: until.Format(time.RFC3339), Since: since.Format(time.RFC3339),
		Until: until.Format(time.RFC3339), Issues: []Bundle{}, Summaries: []Summary{}, Typed: []Typed{}, Questions: []Question{}, Fixes: []Fix{}}

	tasks := r.tasks()
	byID := map[string]*task{}
	for _, t := range tasks {
		byID[t.id] = t
	}
	done := doneTimes(r.log())
	retros := r.retros(tasks)
	e := r.edgeOf(byID, retros, done, o.Exclude, until)
	prev, boundary := e.prev, e.boundary
	recurred := map[string]int{}
	if prev != nil {
		in.PreviousRetro = &PrevRetroIn{Issue: prev.Issue, DoneAt: prev.DoneAt}
		if e.pat != nil {
			in.PreviousRetro.Until = &e.pat.Until
			for _, f := range e.pat.Fixes {
				recurred[f.Issue] = *f.Recurred
			}
		}
		b := boundary.UTC().Format(time.RFC3339)
		in.Boundary = &b
	}

	gh := newGitHub(r, o.GH)
	applied, appliedErr := readApplied(o.Package)
	var cands []*task
	for _, t := range tasks {
		if !inWindow(t, done, retros, since, until) {
			continue
		}
		if o.Issue != "" && t.id != o.Issue {
			continue
		}
		cands = append(cands, t)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		ai, aj := done[cands[i].id], done[cands[j].id]
		if !ai.Equal(aj) {
			return ai.Before(aj)
		}
		return cands[i].id < cands[j].id
	})
	full := map[string]bool{}
	var fresh []*task
	for _, t := range cands {
		if o.Issue != "" || e.fresh(done[t.id]) {
			fresh = append(fresh, t)
		}
	}
	if o.Issue == "" && len(fresh) > fullLimit {
		in.Cut = &Cut{New: len(fresh), Full: fullLimit}
		fresh = fresh[len(fresh)-fullLimit:]
	}
	for _, t := range fresh {
		full[t.id] = true
	}
	for _, t := range cands {
		b, raw := r.bundle(t, done[t.id], tasks)
		if !full[t.id] {
			in.Summaries = append(in.Summaries, r.summary(b))
			continue
		}
		b.AfterMerge = gh.afterMerge(b)
		b.Reapplied, b.ReappliedReasons = reapplied(raw, b.DoneAt, applied, appliedErr)
		in.Issues = append(in.Issues, b)
	}

	if o.Issue == "" {
		ns := r.nums(tasks)
		for _, t := range tasks {
			if !retros[t.parent] || t.reviewer() || t.status != "done" {
				continue
			}
			f, ok := r.fix(t, gh, until)
			if !ok {
				continue
			}
			merged := mustTime(f.MergedAt)
			var workers []doneTask // 前後の数字に数える担当の課題（片付いた時刻の順）
			for _, x := range tasks {
				if at, ok := done[x.id]; ok && x.status == "done" && x.worker() && !retros[x.id] && x.id != t.id {
					workers = append(workers, doneTask{id: x.id, at: at})
					if at.After(merged) {
						f.IssuesAfter++
					}
				}
			}
			sort.SliceStable(workers, func(i, j int) bool {
				if !workers[i].at.Equal(workers[j].at) {
					return workers[i].at.Before(workers[j].at)
				}
				return workers[i].id < workers[j].id
			})
			f.BeforeAfter = ns.beforeAfter(workers, merged)
			if n, ok := recurred[f.Issue]; ok {
				f.RecurredBefore = &n
			} else if prev == nil || !merged.Before(boundary) {
				zero := 0
				f.RecurredBefore = &zero
			}
			in.Fixes = append(in.Fixes, f)
		}
		sort.SliceStable(in.Fixes, func(i, j int) bool { return in.Fixes[i].MergedAt < in.Fixes[j].MergedAt })

		if o.ClaudeDir != "" {
			in.Typed = r.typed(tasks, since, until)
		}
		in.ChiefOfStaff.Tokens, in.ChiefOfStaff.Days = trace.RoleTokens(o.Otel, "chief-of-staff", since, until, func(file string, line int, reason string) {
			src := "otel/" + file
			if line > 0 {
				src += ":" + strconv.Itoa(line)
			}
			r.skip(src, "%s", reason)
		})
		in.Questions = r.questions(tasks, since, until)
	}
	in.Skipped = r.skipped
	if in.Skipped == nil {
		in.Skipped = []Skip{}
	}
	return in, nil
}

func (r *reader) prevPatterns(id string) *Patterns {
	path := filepath.Join(r.dir, "issues", id, "library", retroFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.skip(r.rel(path), "読めない。境目は片付いた時刻にする")
		return nil
	}
	p, err := ParsePatterns(b)
	if err != nil {
		r.skip(r.rel(path), "形が違う（%v）。境目は片付いた時刻にする", err)
		return nil
	}
	return p
}

func (r *reader) retros(tasks []*task) map[string]bool {
	out := map[string]bool{}
	for _, t := range tasks {
		if t.routine == retroName || exists(filepath.Join(r.dir, "issues", t.id, "library", retroFile)) {
			out[t.id] = true
		}
	}
	return out
}

// edge は前の振り返りと境目（ADR-0039 決めたこと 6）。pat は境目にした前の振り返りの retro.json（until を境目にしたときだけ）。
type edge struct {
	prev     *PrevRetro
	pat      *Patterns
	boundary time.Time
}

func (e edge) fresh(at time.Time) bool {
	return e.prev == nil || at.After(e.boundary)
}

func (r *reader) edgeOf(byID map[string]*task, retros map[string]bool, done map[string]time.Time, exclude string, until time.Time) edge {
	var e edge
	for id := range retros {
		at, ok := done[id]
		if !ok || byID[id].status != "done" || id == exclude || at.After(until) {
			continue
		}
		if e.prev == nil || at.After(mustTime(e.prev.DoneAt)) || (at.Equal(mustTime(e.prev.DoneAt)) && id > e.prev.Issue) {
			e.prev = &PrevRetro{Issue: id, DoneAt: at.UTC().Format(time.RFC3339)}
		}
	}
	if e.prev == nil {
		return e
	}
	e.boundary = mustTime(e.prev.DoneAt)
	if p := r.prevPatterns(e.prev.Issue); p != nil {
		if pu := mustTime(p.Until); !pu.After(e.boundary) {
			e.boundary, e.pat = pu, p
		} else {
			r.skip(r.rel(filepath.Join(r.dir, "issues", e.prev.Issue, "library", retroFile)), "until が片付いた時刻より後。境目は片付いた時刻にする")
		}
	}
	return e
}

func inWindow(t *task, done map[string]time.Time, retros map[string]bool, since, until time.Time) bool {
	at, ok := done[t.id]
	return ok && t.status == "done" && t.worker() && !retros[t.id] && !at.Before(since) && !at.After(until)
}

// Fresh は、次の振り返りが全文で読む束の数を数える（dispatcher の巡の件数のきっかけ。ADR-0039 決めたこと 8）。
// 数えるのは、直近 defaultDays 日に片付いた担当の課題のうち、境目より後に片付いたもの。選び方と境目は collect と同じで、
// fullLimit では切らない。issues は kanban の課題の一覧（kanban list --all --json）、rows は log.jsonl。
// ファイルは課題ごとの library/retro.json の有無と、前の振り返りの retro.json だけを読む。
func Fresh(root string, issues []ledger.Issue, rows []ledger.Row, now time.Time) int {
	return fresh(root, issues, rows, now, false)
}

// FreshOwn は Fresh の束のうち、ルーチンが足した課題（issue.yaml の routine を持つ）を除いた数（空きの振り返りの線。
// ADR-0047 決めたこと 8）。空きが撃ったトリアージや点検の課題だけで線に届かないように除く。境目と期間は Fresh と同じ。
func FreshOwn(root string, issues []ledger.Issue, rows []ledger.Row, now time.Time) int {
	return fresh(root, issues, rows, now, true)
}

func fresh(root string, issues []ledger.Issue, rows []ledger.Row, now time.Time, own bool) int {
	r := &reader{o: Options{Root: root}, dir: filepath.Join(root, "kanban")}
	until := now.UTC()
	since := until.Add(-defaultDays * 24 * time.Hour)
	tasks := make([]*task, 0, len(issues))
	byID := map[string]*task{}
	for _, raw := range issues {
		id := ledger.CanonID(raw.Str("id"))
		if id == "" {
			continue
		}
		t := taskOf(id, raw)
		tasks = append(tasks, t)
		byID[id] = t
	}
	done := doneTimes(rows)
	retros := r.retros(tasks)
	e := r.edgeOf(byID, retros, done, "", until)
	n := 0
	for _, t := range tasks {
		if inWindow(t, done, retros, since, until) && e.fresh(done[t.id]) && (!own || t.routine == "") {
			n++
		}
	}
	return n
}

func taskOf(id string, raw ledger.Issue) *task {
	return &task{id: id, title: raw.Str("title"), status: raw.Str("status"), profile: raw.Str("profile"),
		parent: ledger.CanonID(raw.Str("parent")), who: raw.Str("who"), repo: raw.Str("repo"),
		worktree: raw.Path("worktree"), routine: raw.Str("routine"), estimate: raw.Str("estimate")}
}

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
		out = append(out, taskOf(e.Name(), raw))
	}
	sort.Slice(out, func(i, j int) bool { return ledger.IDLess(out[i].id, out[j].id) })
	return out
}

func (r *reader) log() []ledger.Row {
	return ledger.At(r.o.Root).Log()
}

func doneTimes(rows []ledger.Row) map[string]time.Time {
	out := map[string]time.Time{}
	moved := ledger.MovedTo(rows, "done")
	for i, row := range rows {
		if !moved[i] {
			continue
		}
		if at, err := time.Parse(time.RFC3339, row.At); err == nil {
			out[row.ID] = at
		}
	}
	return out
}

func (r *reader) bundle(t *task, doneAt time.Time, tasks []*task) (Bundle, []map[string]any) {
	lib := filepath.Join(r.dir, "issues", t.id, "library")
	if a, err := filepath.Abs(lib); err == nil {
		lib = a
	}
	b := Bundle{ID: t.id, Title: t.title, Profile: t.profile, Repo: strPtr(t.repo), DoneAt: doneAt.UTC().Format(time.RFC3339),
		Library: lib, Estimate: strPtr(t.estimate), Gates: []Gate{}, Reviews: []Review{}, Waived: []any{},
		Remember: []string{}, AgentsMD: []string{}, Gaps: []any{}, ReappliedReasons: []string{}}
	raw := r.launches(filepath.Join(lib, launch.File))
	if raw != nil {
		b.Launches = []Launch{}
		for _, l := range raw {
			b.Launches = append(b.Launches, launchView(l))
		}
	}
	if o := r.object(filepath.Join(lib, "outcome.json")); o != nil {
		b.Outcome = outcomeView(o)
		b.mergeCommit, _ = o["merge_commit"].(string)
		b.WorkedMinutes = b.Outcome.WorkedMinutes
		b.Breakdown = r.breakdown(lib)
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
		if g, ok := rep["gaps"].([]any); ok {
			b.Gaps = g
		}
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
	for i := range b.Reviews {
		b.Reviews[i].Reason = r.reviewReason(b.Reviews[i].ID, b.Reviews)
	}
	return b, raw
}

func (r *reader) reviewReason(id string, siblings []Review) *string {
	b, err := os.ReadFile(filepath.Join(r.dir, "issues", id, "brief.md"))
	if err != nil {
		return nil
	}
	prev, rebased := ledger.BriefReReview(string(b))
	reason := "rerequested"
	switch {
	case prev == "":
		if _, k, ok := ledger.SplitRound(id); ok && k > 1 {
			return nil
		}
		reason = "first"
	case slices.ContainsFunc(siblings, func(s Review) bool { return s.ID != id && s.Findings["commit"] == prev && mustFix(s.Findings) }):
		reason = "fixed"
	case rebased:
		reason = "rebased"
	}
	return &reason
}

func mustFix(f map[string]any) bool {
	fs, _ := f["findings"].([]any)
	for _, x := range fs {
		m, _ := x.(map[string]any)
		if sev, _ := m["severity"].(string); (sev == "critical" || sev == "major") && m["preexisting"] != true {
			return true
		}
	}
	return false
}

func launchView(l map[string]any) Launch {
	v := Launch{}
	v.At, _ = l["at"].(string)
	v.Profile, _ = l["profile"].(string)
	if p, ok := l["package"].(map[string]any); ok {
		if c, ok := p["commit"].(string); ok && c != "" {
			v.Commit = &c
		}
	}
	if c, ok := l["command"].(string); ok {
		v.Model = modelOf(c)
	}
	return v
}

func modelOf(command string) *string {
	f := strings.Fields(command)
	for i, x := range f {
		var m string
		switch {
		case (x == "--model" || x == "-m") && i+1 < len(f):
			m = f[i+1]
		case strings.HasPrefix(x, "--model="):
			m = strings.TrimPrefix(x, "--model=")
		default:
			continue
		}
		if m = strings.Trim(m, `"'`); m != "" {
			return &m
		}
	}
	return nil
}

func outcomeView(o map[string]any) *OutcomeView {
	v := &OutcomeView{}
	v.Result, _ = o["result"].(string)
	if n, ok := o["worked_minutes"].(float64); ok {
		m := int(n)
		v.WorkedMinutes = &m
	}
	ss, ok := o["sessions"].([]any)
	if !ok {
		return v
	}
	c := &Cost{Sessions: len(ss), Tokens: map[string]int{}}
	for _, k := range []string{"input", "output", "cache_read", "cache_write"} {
		c.Tokens[k] = 0
	}
	for _, x := range ss {
		s, _ := x.(map[string]any)
		cost, _ := s["cost"].(map[string]any)
		tok, ok := cost["tokens"].(map[string]any)
		if !ok {
			continue
		}
		c.Measured++
		for k := range c.Tokens {
			if n, ok := tok[k].(float64); ok {
				c.Tokens[k] += int(n)
			}
		}
	}
	v.Cost = c
	return v
}

// summary は束を要約にする: id・タイトル・片付いた時刻・PR と、つまずきの跡の見出し（ADR-0039 決めたこと 6 の細部）。
//   - findings: レビュアーの子の major 以上の指摘（課題の前からあった指摘は除く）ごとに1行。レビュアーの報告で、その指摘の
//     ファイル:行を含む最初の行（字下げした次の行が続けば足す）。報告に無ければ「<重さ> <ファイル>:<行>」
//   - gate_fixes: gate が fix を返した理由（重なりは1つ）
//   - corrections: gate の問いをオーナーがマージして閉じた答えのほかに手直しがあるか。手直しの記録が無ければ null（集めていない）
//   - remember: Remember の行数
func (r *reader) summary(b Bundle) Summary {
	s := Summary{ID: b.ID, Title: b.Title, DoneAt: b.DoneAt, PR: b.PR, Findings: []string{}, GateFixes: []string{}, Remember: len(b.Remember), Gaps: []GapRef{}}
	for _, x := range b.Gaps {
		g, _ := x.(map[string]any)
		kind, _ := g["kind"].(string)
		where, _ := g["where"].(string)
		if !gapKinds[kind] || where == "" {
			r.skip(r.rel(filepath.Join(b.Library, "report.json")), "申告の kind が知らない値か、where が無い（要約に載せていない）")
			continue
		}
		s.Gaps = append(s.Gaps, GapRef{Kind: kind, Where: where})
	}
	for _, rv := range b.Reviews {
		fs, _ := rv.Findings["findings"].([]any)
		var report []string
		for _, x := range fs {
			f, _ := x.(map[string]any)
			sev, _ := f["severity"].(string)
			if (sev != "critical" && sev != "major") || f["preexisting"] == true {
				continue
			}
			file, _ := f["file"].(string)
			n, _ := f["line"].(float64)
			loc := file + ":" + strconv.Itoa(int(n))
			if report == nil {
				report = r.lines(filepath.Join(r.dir, "issues", rv.ID, "report.md"))
			}
			s.Findings = append(s.Findings, clip(findingLine(report, loc, sev)))
		}
	}
	seen := map[string]bool{}
	for _, g := range b.Gates {
		if g.Decision != "fix" {
			continue
		}
		for _, x := range g.Reasons {
			if x = clip(x); !seen[x] {
				seen[x] = true
				s.GateFixes = append(s.GateFixes, x)
			}
		}
	}
	if b.Corrections != nil {
		v := false
		for _, c := range b.Corrections {
			if !ownerMerged(c) {
				v = true
				break
			}
		}
		s.Corrections = &v
	}
	return s
}

func (r *reader) lines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			r.skip(r.rel(path), "読めない")
		}
		return []string{}
	}
	return strings.Split(string(b), "\n")
}

func findingLine(report []string, loc, sev string) string {
	for i, l := range report {
		if !strings.Contains(l, loc) {
			continue
		}
		out := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "- "))
		if i+1 < len(report) && strings.HasPrefix(report[i+1], " ") && strings.TrimSpace(report[i+1]) != "" {
			out += " " + strings.TrimSpace(report[i+1])
		}
		return out
	}
	return sev + " " + loc
}

func ownerMerged(c any) bool {
	m, _ := c.(map[string]any)
	a, _ := m["answer"].(string)
	return m["kind"] == "override" && m["question"] != nil &&
		(strings.HasPrefix(a, "人がマージした") || strings.HasPrefix(a, "オーナーがマージした"))
}

func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if rs := []rune(s); len(rs) > lineMax {
		return string(rs[:lineMax]) + "…"
	}
	return s
}

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

func (r *reader) gates(dir string) []Gate {
	out := []Gate{}
	// filepath.Glob は読めない置き場を空と返すので、置き場を読んで、無いのと読めないのを分ける
	// （読めない置き場を記録 0 件と黙って数えると、前後の数字の差し戻しの率が黙って変わる）
	ents, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		r.skip(r.rel(dir), "置き場が読めない。gate の記録は 0 件として数えた")
	}
	var files []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
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

// applied はパッケージの当て直しの記録（ADR-0027。<置き場>/applied.json）。最後の1回と、前に当てた分の並び previous
// （新しい順。image/apply-main.sh が appliedKept 件まで残す）を読む。previous の無い前のバージョンの記録は最後の1回だけ。
type applied struct {
	Commit    string        `json:"commit"`
	AppliedAt string        `json:"applied_at"`
	Previous  []appliedPrev `json:"previous"`
	cut       string
}

type appliedPrev struct {
	Commit    string `json:"commit"`
	AppliedAt string `json:"applied_at"`
}

// appliedKept は image/apply-main.sh が previous に残す件数の上限（`.[:20]`）。並びがこの件数なら、もっと前の当て直しが落ちている。
const appliedKept = 20

func (a *applied) times() []string {
	out := []string{a.AppliedAt}
	for _, p := range a.Previous {
		out = append(out, p.AppliedAt)
	}
	return out
}

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
	if len(a.Previous) >= appliedKept {
		a.cut = "当て直しの記録が " + strconv.Itoa(appliedKept) + " 件で切れていて"
	}
	for i, p := range a.Previous {
		if _, err := time.Parse(time.RFC3339, p.AppliedAt); err != nil {
			a.Previous = a.Previous[:i]
			a.cut = fmt.Sprintf("当て直しの記録の previous[%d] の applied_at（%q）が時刻でなく、それより前を読めないので", i, p.AppliedAt)
			break
		}
	}
	return &a, nil
}

// reapplied は課題の途中でパッケージが当て直されたかの印（ADR-0028 決めたこと 14 の 5）。
//   - 当て直しの記録の applied_at（最後の1回か previous のどれか）が、最初の起動と片付けの間にある
//   - 起動の記録のパッケージの commit が、起動ごとに違う（previous の無い前のバージョンの記録や、previous から落ちた古い当て直しはこちらで拾う）
//
// どちらでもなく、どちらかを確かめられない（起動の記録が無い・当て直しの記録が読めない・previous が上限まで埋まっているか
// 時刻の読めない1件で途切れていて、最初の起動の時刻まで届かない）なら null。
// revert（記録を消す）や previous を書かない前のバージョンの apply-main.sh で履歴が切れた記録は、途切れた印を持たないので見分けない。
func reapplied(launches []map[string]any, doneAt string, a *applied, aerr error) (*bool, []string) {
	reasons := []string{}
	yes, unknown := false, false
	var start time.Time
	commits := map[string]bool{}
	for _, l := range launches {
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
	end := mustTime(doneAt)
	switch {
	case aerr != nil:
		unknown = true
		reasons = append(reasons, "当て直しの記録が読めない: "+aerr.Error())
	case start.IsZero():
		unknown = true
		reasons = append(reasons, "起動の記録が無いので、起動の時刻が分からない")
	case a != nil:
		times := a.times()
		for _, s := range times {
			if at := mustTime(s); at.After(start) && at.Before(end) {
				yes = true
				reasons = append(reasons, "パッケージを当て直した時刻（"+s+"）が、起動と片付けの間にある")
			}
		}
		if oldest := mustTime(times[len(times)-1]); !yes && a.cut != "" && oldest.After(start) {
			unknown = true
			reasons = append(reasons, a.cut+"、起動の時刻（"+start.UTC().Format(time.RFC3339)+"）まで届かない")
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

func CollectMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("collect", args, stdin, stdout, stderr)
}

func PatternsMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("patterns", args, stdin, stdout, stderr)
}

func ApplyMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("apply", args, stdin, stdout, stderr)
}

func run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := func() int {
		fmt.Fprint(stderr, map[string]string{"collect": CollectUsage, "patterns": PatternsUsage, "apply": ApplyUsage}[cmd])
		return 2
	}
	days, id, src, dry, one := defaultDays, "", "", false, ""
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
		case args[0] == "--issue" && len(args) >= 2 && cmd == "collect" && one == "":
			one = ledger.CanonID(args[1])
			if !idRe.MatchString(one) {
				fmt.Fprintf(stderr, "andon retro: id %q が不正\n", args[1])
				return 2
			}
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
		err = runCollect(root, days, one, stdout, stderr)
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

func runCollect(root string, days int, issue string, stdout, stderr io.Writer) error {
	o := Options{Root: root, Now: time.Now(), Days: days, GH: GH, Package: pkgversion.Home(), Issue: issue,
		Otel: filepath.Join(otel.Root(), filepath.Base(root))}
	if issue == "" {
		o.ClaudeDir = sessionlog.ClaudeDir()
	}
	in, err := Collect(o)
	if err != nil {
		return err
	}
	if issue != "" && len(in.Issues) == 0 {
		return fmt.Errorf("%s は期間（%s〜%s）に片付いた担当の課題でない", issue, in.Since, in.Until)
	}
	b, _, err := Redacted(in)
	if err != nil {
		return err
	}
	if issue != "" {
		var v struct {
			Issues  []json.RawMessage `json:"issues"`
			Skipped []Skip            `json:"skipped"`
		}
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		for _, sk := range v.Skipped {
			fmt.Fprintf(stderr, "andon retro: 読めなかった: %s: %s\n", sk.Source, sk.Reason)
		}
		var out bytes.Buffer
		if err := json.Indent(&out, v.Issues[0], "", "  "); err != nil {
			return err
		}
		out.WriteByte('\n')
		b = out.Bytes()
	}
	_, err = stdout.Write(b)
	return err
}

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

// Redacted は in を、秘密の形に合う文字列を伏せた JSON（2字下げ・末尾に改行）にし、schema/retro.schema.json と照らす。伏せた数を in.Redacted に入れて返す。
func Redacted(in *Input) ([]byte, int, error) {
	in.Redacted = 0
	_, n := redact.JSON(contract.Marshal(in))
	in.Redacted = n
	out, _ := redact.JSON(contract.Marshal(in))
	if !json.Valid(out) {
		return nil, 0, errors.New("秘密を伏せたあとの出力が JSON でない（伏せる形の誤り）")
	}
	// 書くのは伏せたあとの形なので、照らすのもその形（ADR-0034 決めたこと 6）
	if err := contract.SchemaRetro.Check(out); err != nil {
		return nil, 0, err
	}
	return out, n, nil
}
