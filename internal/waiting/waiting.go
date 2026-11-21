// Package waiting は andon waiting: オーナー待ち（マージの依頼・判断・手作業）と並行の枠を数える（ADR-0042 決めたこと 8・9）。
// 読むだけで、何も書かない。kanban のファイル（config.yaml・issues/<id>/issue.yaml・questions.yaml・log.jsonl）と、
// 課題の library/（report.json・gate/<head>.json・questions/<n>.json）と、project.json の chief_of_staff.parallel を直に読む。
// GitHub は読まず、git も呼ばない。kanban の CLI は、数えるのには使わない（置き場が分からないときに入口が kanban root で根を聞くだけ）。
// 数える式はこの package だけが持つ（決めたこと 14）。
package waiting

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/gate"
	"andon/internal/ledger"

	"go.yaml.in/yaml/v3"
)

// Version は出力の version（docs/contract.md §25・schema/waiting.schema.json）。
const Version = 1

// DefaultParallel は project.json に chief_of_staff.parallel が無いときの並行の上限（contract.md §11）。
const DefaultParallel = 2

// Result は andon waiting --json の出力。
type Result struct {
	Version int    `json:"version"`
	At      string `json:"at"`
	Project string `json:"project"`
	Slots   Slots  `json:"slots"`
	Items   []Item `json:"items"`
}

// Slots は並行の枠（決めたこと 9・10）。
type Slots struct {
	Limit       int  `json:"limit"`
	Working     int  `json:"working"`
	Waiting     int  `json:"waiting"`
	Free        int  `json:"free"`
	WaitingFull bool `json:"waiting_full"`
}

// Item はオーナー待ちの1行。
type Item struct {
	Kind       string             `json:"kind"` // merge・decision・task
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Since      string             `json:"since"`
	Question   string             `json:"question,omitempty"` // <id>#<n>
	PR         string             `json:"pr,omitempty"`
	Reason     string             `json:"reason,omitempty"`
	Ask        *contract.Question `json:"ask,omitempty"`        // 判断の行の問いの JSON の写し
	Answerable *bool              `json:"answerable,omitempty"` // 判断の行だけ
	Holds      []string           `json:"holds"`
}

// lightKinds は判断に数える問いの種類（灯りの決断の種類。scope は Chief of Staff が答えるので入れない）。
var lightKinds = []string{"business", "users", "promise", "irreversible", "line"}

const timeLayout = "2006-01-02T15:04:05Z"

// issue は数えるのに読む課題1件。
type issue struct {
	raw       ledger.Issue
	id        string
	title     string
	status    string
	profile   string
	who       string
	after     []string
	notBefore string
	created   string
	updated   string
	questions []question
}

// question は questions.yaml の1件。
type question struct {
	N      int    `yaml:"n"`
	Kind   string `yaml:"kind"`
	Status string `yaml:"status"`
	Asked  string `yaml:"asked"`
}

func (t *issue) reviewer() bool {
	_, _, round := ledger.SplitRound(t.id)
	return round || strings.HasSuffix(t.profile, "-review")
}

// worker は並行の枠に数える担当の課題か（レビュアーの子課題・Chief Engineer の課題・オーナーの課題でない）。
func (t *issue) worker() bool {
	return !t.reviewer() && t.profile != "chief-engineer" && t.who != "person"
}

// Read は kanban の根 root（projects/<slug>/。kanban/ の親）を読み、now の時点のオーナー待ちと枠を返す。
// warn は読めなかったが数えるのを止めないもの（project.json が読めないなど）。
func Read(root string, now time.Time) (res Result, warn []string, err error) {
	dir := filepath.Join(root, "kanban")
	cfg, err := ledger.ReadConfig(dir)
	if err != nil {
		return Result{}, nil, fmt.Errorf("kanban が読めない（%s）: %v", filepath.Join(dir, "config.yaml"), err)
	}
	limit, w := parallel(root)
	if w != "" {
		warn = append(warn, w)
	}
	issues, w2 := readIssues(dir)
	warn = append(warn, w2...)
	l := ledger.At(root)
	rows, err := l.ReadLog()
	// 履歴が無い・読めないと、課題が review に入った時刻と merge の書き手が分からず、approve で止まった PR を動いている仕事に
	// 数えてしまう。課題が1つも無い kanban の、まだ無い log.jsonl だけは空の履歴として読む
	if err != nil && (!errors.Is(err, fs.ErrNotExist) || len(issues) > 0) {
		return Result{}, warn, fmt.Errorf("kanban の履歴が読めない（%s）。枠を数えられない: %v", filepath.Join(dir, "log.jsonl"), err)
	}
	return count(l, cfg, issues, rows, limit, now), warn, nil
}

// parallel は project.json の chief_of_staff.parallel。無ければ DefaultParallel。読めない・形が違うときも DefaultParallel と理由
// （安全側。上限を広げない）。git を呼ばないため、作者の照合（delegate.ReadProject）は通さずファイルを読む。
func parallel(root string) (int, string) {
	path := filepath.Join(root, "project.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultParallel, ""
	}
	if err != nil {
		return DefaultParallel, fmt.Sprintf("%s が読めない（上限は %d で数える）: %v", path, DefaultParallel, err)
	}
	p, err := delegate.ParseProject(b, filepath.Base(root))
	if err != nil {
		return DefaultParallel, fmt.Sprintf("%s の形が違う（上限は %d で数える）: %v", path, DefaultParallel, err)
	}
	if p.ChiefOfStaff == nil || p.ChiefOfStaff.Parallel == nil {
		return DefaultParallel, ""
	}
	return *p.ChiefOfStaff.Parallel, ""
}

// readIssues は issues/*/issue.yaml と questions.yaml を読む。読めない課題は飛ばして理由を返す。
func readIssues(dir string) ([]*issue, []string) {
	ents, err := os.ReadDir(filepath.Join(dir, "issues"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("%s が読めない: %v", filepath.Join(dir, "issues"), err)}
	}
	var out []*issue
	var warn []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		id := ledger.CanonID(e.Name())
		b, err := os.ReadFile(filepath.Join(dir, "issues", e.Name(), "issue.yaml"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var raw ledger.Issue
		if err != nil || yaml.Unmarshal(b, &raw) != nil || raw == nil {
			warn = append(warn, fmt.Sprintf("%s の issue.yaml が読めない（数えない）", id))
			continue
		}
		raw["id"] = id
		t := &issue{raw: raw, id: id, title: raw.Str("title"), status: raw.Str("status"), profile: raw.Str("profile"),
			who: raw.Str("who"), notBefore: raw.Str("not_before"), created: raw.Str("created"), updated: raw.Str("updated")}
		if list, ok := raw["after"].([]any); ok {
			for _, a := range list {
				if s, ok := a.(string); ok && s != "" {
					t.after = append(t.after, ledger.CanonID(s))
				}
			}
		}
		if qb, err := os.ReadFile(filepath.Join(dir, "issues", e.Name(), "questions.yaml")); err == nil {
			if yaml.Unmarshal(qb, &t.questions) != nil {
				warn = append(warn, fmt.Sprintf("%s の questions.yaml が読めない（問いを数えない）", id))
				t.questions = nil
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return ledger.IDLess(out[i].id, out[j].id) })
	return out, warn
}

// count は読んだ kanban から出力を作る。
func count(l *ledger.Ledger, cfg ledger.Config, issues []*issue, rows []ledger.Row, limit int, now time.Time) Result {
	done := "done"
	if len(cfg.Columns) > 0 {
		done = cfg.Columns[len(cfg.Columns)-1]
	}
	byID := map[string]*issue{}
	for _, t := range issues {
		byID[t.id] = t
	}
	// 課題が最後に review に入った行の時刻（gate の記録はこのあとのものだけを見る。internal/issue の gatedSince と同じ引き方）
	reviewAt := map[string]string{}
	for _, r := range rows {
		if r.Change != nil && r.Change["status"] == "review" {
			reviewAt[r.ID] = r.At
		}
	}
	loc := time.UTC
	if cfg.Timezone != "" {
		if z, err := time.LoadLocation(cfg.Timezone); err == nil {
			loc = z
		}
	}
	items := []Item{}
	waiting := map[string]bool{}
	for _, t := range issues {
		if t.status == done {
			continue
		}
		lib := l.Library(t.id)
		if it, ok := mergeItem(l, t, rows, reviewAt[t.id], lib); ok {
			items = append(items, it)
			waiting[t.id] = true
		}
		for _, it := range decisionItems(t, lib) {
			items = append(items, it)
			waiting[t.id] = true
		}
		if it, ok := taskItem(t, byID, done, now.In(loc)); ok {
			items = append(items, it)
		}
	}
	holds := holders(issues, done)
	for i := range items {
		items[i].Holds = holds(items[i].ID)
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if len(a.Holds) != len(b.Holds) {
			return len(a.Holds) > len(b.Holds)
		}
		if a.Since != b.Since {
			return a.Since < b.Since
		}
		if a.ID != b.ID {
			return ledger.IDLess(a.ID, b.ID)
		}
		return a.Question < b.Question
	})
	s := Slots{Limit: limit}
	for _, t := range issues {
		if (t.status != "doing" && t.status != "review") || !t.worker() {
			continue
		}
		if waiting[t.id] {
			s.Waiting++
		} else {
			s.Working++
		}
	}
	s.WaitingFull = s.Waiting >= s.Limit
	if !s.WaitingFull {
		s.Free = max(s.Limit-s.Working, 0)
	}
	return Result{Version: Version, At: now.UTC().Format(timeLayout), Project: l.Slug, Slots: s, Items: items}
}

// mergeItem はマージの依頼（決めたこと 8）。課題が review にいて、次のどれか: 今の head について gate が出した問いが開いている／
// 今の head の gate の判定が approve で、merge: auto でないか、merge: auto を最後に書いたのが Chief of Staff でない／
// Chief Engineer の課題で、報告に PR がある。gate の記録にマージ済みとある課題は数えない。
func mergeItem(l *ledger.Ledger, t *issue, rows []ledger.Row, since, lib string) (Item, bool) {
	if t.status != "review" {
		return Item{}, false
	}
	rep, _ := contract.ReadReport(filepath.Join(lib, "report.json"))
	head, pr := "", ""
	if rep != nil {
		head, pr = rep.Commit, rep.PR
	}
	rec := gateRecord(lib, head)
	if rec != nil && rec.Merged {
		return Item{}, false
	}
	it := Item{Kind: "merge", ID: t.id, Title: t.title, PR: pr}
	for _, q := range t.questions {
		if q.Status != "open" {
			continue
		}
		ask := readAsk(lib, q.N)
		if ask == nil || ask.From != "gate" {
			continue
		}
		// どの head の問いか読めない問いは今の head の問いとして数える（andon issue ready の staleGateQuestions と同じ）
		if h := gate.QuestionHead(ask.Facts); h != "" && h != head {
			continue
		}
		it.Question, it.Since = fmt.Sprintf("%s#%d", t.id, q.N), firstOf(q.Asked, t.updated, t.created)
		it.Reason = ask.Question
		if rec != nil && len(rec.Reasons) > 0 {
			it.Reason = strings.Join(rec.Reasons, "／")
		}
		return it, true
	}
	if rec != nil && since != "" && rec.At >= since && rec.Decision == "approve" {
		s := ledger.SettingsOf(t.raw)
		switch {
		case s.Merge != "auto":
			it.Reason = "gate は approve。merge が auto でない（オーナーがマージする）"
		case !l.Trusted(rows, t.raw, "merge"):
			it.Reason = "gate は approve。merge: auto を最後に書いたのが Chief of Staff でない"
		default:
			return Item{}, false // dispatcher が自動でマージする途中
		}
		it.Since = firstOf(rec.At, t.updated, t.created)
		return it, true
	}
	if t.profile == "chief-engineer" && pr != "" {
		it.Reason = "Chief Engineer の文書の PR"
		it.Since = firstOf(since, t.updated, t.created)
		return it, true
	}
	return Item{}, false
}

// decisionItems は判断（決めたこと 8）: 灯りの種類の問いが開いていて、出したのが gate でない。問い1件に1行。
func decisionItems(t *issue, lib string) []Item {
	var out []Item
	for _, q := range t.questions {
		if q.Status != "open" {
			continue
		}
		ask := readAsk(lib, q.N)
		if ask != nil && ask.From == "gate" {
			continue
		}
		kind := q.Kind
		if ask != nil {
			kind = ask.Kind // 写す問いの JSON の種類で決める（questions.yaml と食い違っても ask と行の種類を揃える）
		}
		if !slices.Contains(lightKinds, kind) {
			continue
		}
		it := Item{Kind: "decision", ID: t.id, Title: t.title, Since: firstOf(q.Asked, t.updated, t.created),
			Question: fmt.Sprintf("%s#%d", t.id, q.N), Ask: ask}
		answerable := ask != nil && len(ask.Options) > 0 && ask.Kind != "irreversible"
		it.Answerable = &answerable
		out = append(out, it)
	}
	return out
}

// taskItem は手作業（決めたこと 8）: オーナーの課題で、起動できるか動いていて、受け取られていない問いを持たない。
func taskItem(t *issue, byID map[string]*issue, done string, now time.Time) (Item, bool) {
	if t.who != "person" {
		return Item{}, false
	}
	for _, q := range t.questions {
		if q.Status == "open" || q.Status == "answered" {
			return Item{}, false
		}
	}
	switch t.status {
	case "doing":
	case "todo", "queued":
		for _, a := range t.after {
			if o := byID[a]; o == nil || o.status != done {
				return Item{}, false // 前の課題を待っている（まだオーナーの番でない）
			}
		}
		if day, err := time.Parse("2006-01-02", t.notBefore); err == nil && now.Format("2006-01-02") < day.Format("2006-01-02") {
			return Item{}, false // 日付を待っている
		}
	default:
		return Item{}, false
	}
	return Item{Kind: "task", ID: t.id, Title: t.title, Since: firstOf(t.created, t.updated)}, true
}

// holders は、課題の id から、それを after で待つ済んでいない課題（直接と、その後ろで待つもの全部）を id の順で返す関数を作る。
func holders(issues []*issue, done string) func(string) []string {
	waitsOn := map[string][]string{}
	for _, t := range issues {
		if t.status == done {
			continue
		}
		for _, a := range t.after {
			waitsOn[a] = append(waitsOn[a], t.id)
		}
	}
	return func(id string) []string {
		seen := map[string]bool{id: true}
		out := []string{}
		next := []string{id}
		for len(next) > 0 {
			cur := next[0]
			next = next[1:]
			for _, d := range waitsOn[cur] {
				if !seen[d] {
					seen[d] = true
					out = append(out, d)
					next = append(next, d)
				}
			}
		}
		sort.Slice(out, func(i, j int) bool { return ledger.IDLess(out[i], out[j]) })
		return out
	}
}

// gateRecord は今の head の gate の記録（library/gate/<head>.json）。無い・読めなければ nil。
func gateRecord(lib, head string) *gate.Record {
	if !contract.IsSHA(head) {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(lib, "gate", head+".json"))
	var g gate.Record
	if err != nil || json.Unmarshal(b, &g) != nil || g.Decision == "" {
		return nil
	}
	return &g
}

// readAsk は問いの JSON（library/questions/<n>.json）。無い・形が違えば nil（問いの JSON を持たない問い）。
func readAsk(lib string, n int) *contract.Question {
	b, err := os.ReadFile(filepath.Join(lib, "questions", strconv.Itoa(n)+".json"))
	if err != nil {
		return nil
	}
	q, err := contract.ParseQuestion(b)
	if err != nil {
		return nil
	}
	return q
}

// firstOf は最初の空でない時刻（UTC の YYYY-MM-DDTHH:MM:SSZ）。どれも形が違えば 1970 年の頭。
func firstOf(vs ...string) string {
	for _, v := range vs {
		if _, err := time.Parse(timeLayout, v); err == nil {
			return v
		}
	}
	return "1970-01-01T00:00:00Z"
}
