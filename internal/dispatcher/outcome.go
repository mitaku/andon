package dispatcher

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/launch"
	"andon/internal/ledger"
	"andon/internal/otel"
	"andon/internal/sessionlog"
	"andon/internal/trace"
)

// 結果の記録（contract.md §19。ADR-0028 決めたこと 7）: done になった課題ごとに1回だけ library/outcome.json を書く。
// 書き手は dispatcher だけ。結果（PR がマージされた・閉じられた・開いたまま・PR 無し）・head ごとの gate の最後の判定・
// レビュアーの子の課題の数・実働・起動の記録（launches.jsonl）にある全部のセッションのコストを1つにまとめる。
// 片付けたあとに読み直すのは PR の状態だけで、コストはセッションログ（30日で消える）からその時に読む。
// 読めないもの（gh・kanban・記録）があれば書かず、次の見回りで1回だけ打ち直す。それも駄目ならログに1行で諦める
// （ファイルが無いのが「集めていない」。読み手の振り返りは無い欄を null として読む）。オーナーの課題（who: person）には書かない。
// 内訳 breakdown（ADR-0060 決めたこと 15）は、ハーネスの生の記録（30日で消える）をその時に読んでまとめる。記録が無いか読めなければ
// null（読めないときはログに1行）。生の記録は受け口が止まれば欠けるので、読めなくても結果の記録は書く。

const outcomeFile = "outcome.json"

const outcomeTries = 2

const (
	ResultMerged = "merged"
	ResultClosed = "closed"
	ResultOpen   = "open"
	ResultNone   = "none"
)

// Outcome は結果の記録（schema/outcome.schema.json）。
type Outcome struct {
	At            string            `json:"at"`
	Issue         string            `json:"issue"`
	Result        string            `json:"result"`
	PR            *string           `json:"pr"`
	MergeCommit   *string           `json:"merge_commit"`
	MergedAt      *string           `json:"merged_at"`
	Gates         []OutcomeGate     `json:"gates"`
	Reviewers     int               `json:"reviewers"`
	WorkedMinutes int               `json:"worked_minutes"`
	Sessions      *[]OutcomeSession `json:"sessions"`
	Breakdown     *trace.Breakdown  `json:"breakdown"`
}

type OutcomeGate struct {
	Head     string   `json:"head"`
	At       string   `json:"at"`
	Tier     string   `json:"tier"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
	Merged   bool     `json:"merged"`
}

type OutcomeSession struct {
	Harness string       `json:"harness"`
	Session *string      `json:"session"`
	Cost    *OutcomeCost `json:"cost"`
}

type OutcomeCost struct {
	Models  []string         `json:"models"`
	Efforts []string         `json:"efforts"`
	Turns   *int             `json:"turns"`
	Tokens  sessionlog.Usage `json:"tokens"`
}

type prState struct {
	State       string `json:"state"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	MergedAt string `json:"mergedAt"`
}

func Result(hasPR bool, state string) string {
	if !hasPR {
		return ResultNone
	}
	switch state {
	case "MERGED":
		return ResultMerged
	case "CLOSED":
		return ResultClosed
	case "OPEN":
		return ResultOpen
	}
	return ""
}

func (p *project) outcomes() {
	if len(p.st.Outcomes) == 0 {
		return
	}
	if until, err := time.Parse(time.RFC3339, p.st.GHReset); err == nil && time.Now().Before(until) {
		return
	}
	p.st.Outcomes.Each(p.outcome)
}

func (p *project) outcome(id string) bool {
	path := filepath.Join(p.l.Library(id), outcomeFile)
	if _, err := os.Stat(path); err == nil {
		return true
	}
	t, err := p.issue(id)
	if err == nil && t.Str("who") == "person" {
		return true
	}
	var o *Outcome
	if err == nil {
		o, err = p.collectOutcome(id, t)
	}
	if err == nil {
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			err = contract.Write(path, contract.SchemaOutcome, o)
			if err != nil && o.Breakdown != nil {
				// 内訳は送り手の記録から数えるので、schema の外の値が混ざりうる。内訳のために結果の記録を止めない
				p.logf("%s の内訳が schema を通らない（breakdown は null で書く）: %v", id, err)
				o.Breakdown = nil
				err = contract.Write(path, contract.SchemaOutcome, o)
			}
		}
	}
	if err == nil {
		p.logf("%s の結果（%s）を outcome.json に書いた", id, o.Result)
		return true
	}
	if p.st.Outcomes.Fail(id, outcomeTries) {
		p.logf("%s の結果を集められない（%d 回打って失敗。諦める。outcome.json は書いていない）: %v", id, outcomeTries, err)
		return true
	}
	p.logf("%s の結果を集められない（次の見回りで1回だけ打ち直す）: %v", id, err)
	return false
}

func (p *project) issue(id string) (ledger.Issue, error) {
	for _, t := range p.tasks {
		if t.Str("id") == id {
			return t, nil
		}
	}
	return p.l.Show(id)
}

func (p *project) collectOutcome(id string, t ledger.Issue) (*Outcome, error) {
	lib := p.l.Library(id)
	o := &Outcome{At: now(), Issue: id, Gates: []OutcomeGate{}}
	rep, err := contract.ReadReport(filepath.Join(lib, "report.json"))
	if errors.Is(err, os.ErrNotExist) {
		rep, err = nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("report.json が読めない: %v", err)
	}
	if rep != nil && rep.PR != "" {
		owner, name, number, ok := contract.ParsePR(rep.PR)
		if !ok {
			return nil, fmt.Errorf("report.json の pr が読めない: %s", rep.PR)
		}
		out, _, err := run("", nil, "", "gh", "pr", "view", number, "--repo", owner+"/"+name, "--json", "state,mergeCommit,mergedAt")
		if err != nil {
			return nil, err
		}
		var v prState
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			return nil, fmt.Errorf("gh pr view の出力が読めない: %v", err)
		}
		if o.Result = Result(true, v.State); o.Result == "" {
			return nil, fmt.Errorf("PR の状態を知らない: %q", v.State)
		}
		pr := rep.PR
		o.PR = &pr
		if o.Result == ResultMerged {
			if v.MergeCommit != nil && v.MergeCommit.Oid != "" {
				sha := v.MergeCommit.Oid
				o.MergeCommit = &sha
			}
			if v.MergedAt != "" {
				at := v.MergedAt
				o.MergedAt = &at
			}
		}
	} else {
		o.Result = Result(false, "")
	}
	if o.Gates, err = gates(filepath.Join(lib, "gate")); err != nil {
		return nil, err
	}
	for _, x := range p.tasks {
		if x.Str("parent") == id && isReviewer(x) {
			o.Reviewers++
		}
	}
	worked, waits, err := p.workedWaits(id)
	if err != nil {
		return nil, err
	}
	o.WorkedMinutes = int(worked.Minutes())
	launches, err := readLaunches(filepath.Join(lib, launch.File))
	if err != nil {
		return nil, err
	}
	if launches != nil {
		s := sessions(launches, p.cwds(id, t, launches), sessionlog.ClaudeDir(), sessionlog.CodexSessions())
		o.Sessions = &s
	}
	o.Breakdown = p.breakdown(id, launches, o.Sessions, waits)
	return o, nil
}

// breakdown は課題 id の内訳（contract.md §19）。生の記録は課題の置き場だけを読む（レビュアーの課題の分は入らない）。
// 待ちのうち問い・レビュー・端末が死んでいた間は実働と同じ数え方（ledger.WorkedWaits）。役は起動の記録の最後の行の役。
func (p *project) breakdown(id string, launches []launchRow, ss *[]OutcomeSession, waits ledger.Waits) *trace.Breakdown {
	ids := []string{}
	if ss != nil {
		for _, s := range *ss {
			v := ""
			if s.Session != nil {
				v = *s.Session
			}
			ids = append(ids, v)
		}
	}
	role := ""
	if n := len(launches); n > 0 {
		role = launches[n-1].Role
	}
	b, err := trace.BreakdownOf(filepath.Join(otel.Root(), p.l.Slug), id, ids, role, trace.WikiDir())
	if err != nil {
		p.logf("%s の内訳を数えられない（breakdown は null で書く）: %v", id, err)
		return nil
	}
	if b != nil {
		b.Waits.QuestionMs = waits.Question.Milliseconds()
		b.Waits.ReviewMs = waits.Review.Milliseconds()
		b.Waits.GoneMs = waits.Gone.Milliseconds()
	}
	return b
}

func gates(dir string) ([]OutcomeGate, error) {
	out := []OutcomeGate{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gate の記録の置き場が読めない: %v", err)
	}
	for _, e := range entries {
		if ok, _ := filepath.Match("*.json", e.Name()); !ok {
			continue
		}
		f := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("gate の記録 %s が読めない: %v", filepath.Base(f), err)
		}
		var r gate.Record
		if err := json.Unmarshal(b, &r); err != nil || r.Head == "" || r.Decision == "" {
			return nil, fmt.Errorf("gate の記録 %s が読めない", filepath.Base(f))
		}
		reasons := r.Reasons
		if reasons == nil {
			reasons = []string{}
		}
		out = append(out, OutcomeGate{Head: r.Head, At: r.At, Tier: r.Tier, Decision: r.Decision, Reasons: reasons, Merged: r.Merged})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Head < out[j].Head
	})
	return out, nil
}

// workedWaits は課題 id の実働と待ちの内訳（最後の done の行まで）。
func (p *project) workedWaits(id string) (time.Duration, ledger.Waits, error) {
	rows, err := p.l.ReadLog()
	if err != nil {
		return 0, ledger.Waits{}, fmt.Errorf("log.jsonl が読めない: %v", err)
	}
	i, at, ok := ledger.LastStatus(rows, id, "done")
	if !ok {
		return 0, ledger.Waits{}, fmt.Errorf("log.jsonl に %s の done の行が無い", id)
	}
	var gone []time.Time
	for _, g := range p.st.Gone[id] {
		gone = append(gone, time.UnixMilli(g.At))
	}
	worked, waits := ledger.WorkedWaits(rows[:i+1], p.tasks, id, p.l.ChiefOfStaff(), gone, at)
	return worked, waits, nil
}

// launchRow は起動の記録の1行のうち読む所（contract.md §18）。
type launchRow struct {
	Role    string  `json:"role"`
	Session *string `json:"session"`
	Harness struct {
		Name string `json:"name"`
	} `json:"harness"`
	Repo *struct {
		Path string `json:"path"`
	} `json:"repo"`
}

func readLaunches(path string) ([]launchRow, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("起動の記録が読めない: %v", err)
	}
	out := []launchRow{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r launchRow
		if err := json.Unmarshal(line, &r); err != nil || r.Harness.Name == "" {
			return nil, fmt.Errorf("起動の記録の %d 行目が読めない", n)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("起動の記録が読めない: %v", err)
	}
	return out, nil
}

func (p *project) cwds(id string, t ledger.Issue, launches []launchRow) []string {
	dir := filepath.Dir(p.l.Library(id))
	if a, err := filepath.Abs(dir); err == nil {
		dir = a
	}
	out := []string{dir}
	add := func(s string) {
		if s != "" && !slices.Contains(out, filepath.Clean(s)) {
			out = append(out, filepath.Clean(s))
		}
	}
	add(t.Str("worktree"))
	for _, l := range launches {
		if l.Repo != nil {
			add(l.Repo.Path)
		}
	}
	return out
}

// sessions は起動の記録にある全部のセッションのコストを並べる（起動の記録の順。同じセッションは1つ）。
//   - Claude Code: 記録の session ごと。session の無い起動は1つにまとめて cost は null。
//   - Codex: 起動のときに id が決まらないので、cwd が課題の cwd の rollout を session_meta の id ごとに。1つも無ければ
//     session も cost も null の1件。
//
// セッションログが無いか読み切れないセッションは cost が null。
func sessions(launches []launchRow, cwds []string, claudeDir, codexDir string) []OutcomeSession {
	out := []OutcomeSession{}
	seen := map[string]bool{}
	codex := false
	for _, l := range launches {
		if l.Harness.Name == "codex" {
			codex = true
			continue
		}
		key := l.Harness.Name + "|"
		if l.Session != nil {
			key += *l.Session
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		x := OutcomeSession{Harness: l.Harness.Name}
		if l.Session != nil && *l.Session != "" {
			s := *l.Session
			x.Session = &s
			if l.Harness.Name == "claude" {
				x.Cost = cost(sessionlog.Claude(claudeDir, s, nil))
			}
		}
		out = append(out, x)
	}
	if codex {
		byCwd := sessionlog.CodexByCwd(codexDir, nil)
		var files []string
		for _, d := range cwds {
			files = append(files, byCwd[d]...)
		}
		ids, logs := sessionlog.CodexLogs(files, nil)
		for _, id := range ids {
			x := OutcomeSession{Harness: "codex", Cost: cost(logs[id])}
			if id != "" {
				s := id
				x.Session = &s
			}
			out = append(out, x)
		}
		if len(ids) == 0 {
			out = append(out, OutcomeSession{Harness: "codex"})
		}
	}
	return out
}

func cost(l *sessionlog.Log) *OutcomeCost {
	if l == nil || l.Unread {
		return nil
	}
	c := &OutcomeCost{Models: l.Models, Efforts: l.Efforts, Turns: l.Turns, Tokens: l.Usage}
	if c.Models == nil {
		c.Models = []string{}
	}
	if c.Efforts == nil {
		c.Efforts = []string{}
	}
	return c
}
