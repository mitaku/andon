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
	"andon/internal/sessionlog"
)

// 結果の記録（contract.md §19。ADR-0028 決めたこと 7）: done になった課題ごとに1回だけ library/outcome.json を書く。
// 書き手は dispatcher だけ。結果（PR がマージされた・閉じられた・開いたまま・PR 無し）・head ごとのゲートの最後の判定・
// レビュアーの子の課題の数・実働・起動の記録（launches.jsonl）にある全部のセッションのコストを1つにまとめる。
// 片付けたあとに読み直すのは PR の状態だけで、コストはセッションログ（30日で消える）からその時に読む。
// 読めないもの（gh・台帳・記録）があれば書かず、次の見回りで1回だけ打ち直す。それも駄目ならログに1行で諦める
// （ファイルが無いのが「集めていない」。読み手の振り返りは無い欄を null として読む）。人の課題（who: person）には書かない。

// outcomeFile は結果の記録のファイルの名前。
const outcomeFile = "outcome.json"

// outcomeTries は結果の記録を集めようとする回数の上限（初めの1回 ＋ 次の見回りの1回）。
const outcomeTries = 2

// 結果（result）の値。
const (
	ResultMerged = "merged" // PR がマージされた
	ResultClosed = "closed" // PR がマージされずに閉じられた
	ResultOpen   = "open"   // PR が開いたまま片付いた
	ResultNone   = "none"   // PR 無しで片付いた（report.json に pr が無い）
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
}

// OutcomeGate は head ごとのゲートの最後の判定（library/gate/<head>.json の写し）。
type OutcomeGate struct {
	Head     string   `json:"head"`
	At       string   `json:"at"`
	Tier     string   `json:"tier"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
	Merged   bool     `json:"merged"`
}

// OutcomeSession は起動の記録にあるセッション1つと、そのコスト（セッションログが無ければ null）。
type OutcomeSession struct {
	Harness string       `json:"harness"`
	Session *string      `json:"session"`
	Cost    *OutcomeCost `json:"cost"`
}

// OutcomeCost はセッションログから読んだコスト（数え方は andon measure と同じ。sessionlog）。
type OutcomeCost struct {
	Models  []string         `json:"models"`
	Efforts []string         `json:"efforts"`
	Turns   *int             `json:"turns"`
	Tokens  sessionlog.Usage `json:"tokens"`
}

// prState は gh pr view --json state,mergeCommit,mergedAt の読む所。
type prState struct {
	State       string `json:"state"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	MergedAt string `json:"mergedAt"`
}

// Result は PR の状態から結果の値を決める（純粋関数）。pr が無ければ none。知らない状態は空（書かない）。
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

// outcomes は done になった課題（控えの Outcomes）ごとに outcome.json を書く。gh の上限で止めている間は打たない（数えない）。
func (p *project) outcomes() {
	if len(p.st.Outcomes) == 0 {
		return
	}
	if until, err := time.Parse(time.RFC3339, p.st.GHReset); err == nil && time.Now().Before(until) {
		return
	}
	ids := make([]string, 0, len(p.st.Outcomes))
	for id := range p.st.Outcomes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if p.outcome(id) {
			delete(p.st.Outcomes, id)
		}
	}
	if len(p.st.Outcomes) == 0 {
		p.st.Outcomes = nil
	}
}

// outcome は1つの課題について集めて書く。true は済んだ（控えから外す）とき。
func (p *project) outcome(id string) bool {
	path := filepath.Join(p.l.Library(id), outcomeFile)
	if _, err := os.Stat(path); err == nil {
		return true // もう書いた（同じ finish を2度拾っても書き直さない）
	}
	t, err := p.issue(id)
	if err == nil && t.Str("who") == "person" {
		return true // 人の課題（エージェントが動いていない）
	}
	var o *Outcome
	if err == nil {
		o, err = p.collectOutcome(id, t)
	}
	if err == nil {
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			err = contract.Write(path, o)
		}
	}
	if err == nil {
		p.logf("%s の結果（%s）を outcome.json に書いた", id, o.Result)
		return true
	}
	p.st.Outcomes[id]++
	if p.st.Outcomes[id] >= outcomeTries {
		p.logf("%s の結果を集められない（%d 回打って失敗。諦める。outcome.json は書いていない）: %v", id, outcomeTries, err)
		return true
	}
	p.logf("%s の結果を集められない（次の見回りで1回だけ打ち直す）: %v", id, err)
	return false
}

// issue は巡の写しから課題を引く（写しに無ければ台帳から読む）。
func (p *project) issue(id string) (ledger.Issue, error) {
	for _, t := range p.tasks {
		if t.Str("id") == id {
			return t, nil
		}
	}
	return p.l.Show(id)
}

// collectOutcome は台帳・gh・セッションログから1つの課題の結果の記録を集める。読めないものがあれば err（書かない）。
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
	o.WorkedMinutes = p.workedMinutes(id)
	launches, err := readLaunches(filepath.Join(lib, launch.File))
	if err != nil {
		return nil, err
	}
	if launches != nil {
		s := sessions(launches, p.cwds(id, t, launches), sessionlog.ClaudeDir(), sessionlog.CodexSessions())
		o.Sessions = &s
	}
	return o, nil
}

// gates は library/gate/*.json を、判定の時刻の順に写す。置き場が無ければ空。読めない記録があれば err。
func gates(dir string) ([]OutcomeGate, error) {
	out := []OutcomeGate{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("ゲートの記録 %s が読めない: %v", filepath.Base(f), err)
		}
		var r gate.Record
		if err := json.Unmarshal(b, &r); err != nil || r.Head == "" || r.Decision == "" {
			return nil, fmt.Errorf("ゲートの記録 %s が読めない", filepath.Base(f))
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

// workedMinutes は課題の実働（分。待ちを除く。ledger.Worked）を、最後に done になった時刻まで数える（andon measure と同じ）。
func (p *project) workedMinutes(id string) int {
	rows := p.rows
	at, ok := doneAt(rows, id)
	if !ok { // 写しを読んだあとに書かれた done の行（同じ巡）
		rows = p.l.Log()
		at, ok = doneAt(rows, id)
	}
	if !ok {
		at = time.Now()
	}
	var gone []time.Time
	for _, g := range p.st.Gone[id] {
		gone = append(gone, time.UnixMilli(g.At))
	}
	var upto []ledger.Row
	for _, r := range rows {
		if t, err := time.Parse(time.RFC3339, r.At); err == nil && t.After(at) {
			break
		}
		upto = append(upto, r)
	}
	return int(ledger.Worked(upto, p.tasks, id, p.l.ChiefOfStaff(), gone, at).Minutes())
}

// doneAt は log.jsonl の課題 id の最後の done の行の時刻。
func doneAt(rows []ledger.Row, id string) (time.Time, bool) {
	var at time.Time
	ok := false
	for _, r := range rows {
		if ledger.CanonID(r.ID) != id || r.Change == nil || r.Change["status"] != "done" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, r.At); err == nil {
			at, ok = t, true
		}
	}
	return at, ok
}

// launchRow は起動の記録の1行のうち読む所（contract.md §18）。
type launchRow struct {
	Session *string `json:"session"`
	Harness struct {
		Name string `json:"name"`
	} `json:"harness"`
	Repo *struct {
		Path string `json:"path"`
	} `json:"repo"`
}

// readLaunches は起動の記録を読む。ファイルが無ければ nil（この仕組みの前の課題）。読めない行があれば err。
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

// cwds は Codex の記録を引く cwd: 台帳の issues/<id>/・issue.yaml の worktree・起動の記録の repo.path（レビュアーの detached worktree）。
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
// セッションログが無いセッションは cost が null。
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
	if l == nil {
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
