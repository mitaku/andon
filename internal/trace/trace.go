// Package trace は課題 1 件の流れを 1 本の木に組んで出す（andon issue trace。ADR-0060 決めたこと 12・13・14）。
//
// 段は kanban の記録から読むときに導く（新しく書く記録は無い。決めたこと 12 の表）:
//
//	列ごとの待ちと動き  log.jsonl
//	起動、起動し直し    library/launches.jsonl
//	走らせた command    library/runs/<n>.json
//	レビュー            parent がこの課題のレビュアーの課題（issue.yaml）と、その library/findings.json
//	gate の判定         library/gate/<head>.json
//	問いとオーナー待ち  questions.yaml と library/questions/<n>.json
//	端末が死んでいた間  dispatcher の控えの gone
//	マージ              library/outcome.json の merged_at
//
// 起動の下には、ハーネスの生の記録（受け口の置き場の issues/<id>/traces.jsonl。docs/contract.md §30）の span を、
// session.id で結んで入れる。レビュアーの課題の起動と記録は、親の課題の木のレビューの段の下に入る（決めたこと 13）。
// Chief Engineer のテーマと信号は、parent と links の id を出すだけにする。
//
// 読むだけ。kanban にも置き場にも書かない。読めない記録は止まらずに飛ばし、飛ばした所と理由を出す。
package trace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/ledger"
)

// 木の節の種類（--json の kind）。上から、課題・段・起動・プロンプト 1 つ分・モデルの呼び出しとツールの順に入る。
const (
	KindIssue       = "issue"
	KindColumn      = "column"
	KindLaunch      = "launch"
	KindRun         = "run"
	KindQuestion    = "question"
	KindReview      = "review"
	KindGate        = "gate"
	KindGone        = "gone"
	KindMerge       = "merge"
	KindInteraction = "interaction"
	KindLLM         = "llm_request"
	KindTool        = "tool"
	KindSpan        = "span"
)

// order は同じ時刻に始まる段の並び（記録の起きた順に近いもの）。
var order = map[string]int{KindColumn: 0, KindLaunch: 1, KindRun: 2, KindQuestion: 3, KindReview: 4, KindGate: 5, KindGone: 6, KindMerge: 7}

// Node は木の節。End が nil の段は続いている（まだ閉じていない）。点の段（gate・マージ・done の列）は Start と End が同じ。
type Node struct {
	Kind     string         `json:"kind"`
	Name     string         `json:"name"`
	Issue    string         `json:"issue,omitempty"`
	Start    *time.Time     `json:"start"`
	End      *time.Time     `json:"end"`
	Attrs    map[string]any `json:"attrs,omitempty"`
	Summary  *Summary       `json:"summary,omitempty"`
	Children []*Node        `json:"children,omitempty"`

	span *span // ハーネスの span（ハーネスの節だけ）
}

// Summary はハーネスの記録の数え（起動と、プロンプト 1 つ分の根の節に付ける。下の span を全部数える）。
type Summary struct {
	Spans       int    `json:"spans"`
	LLMRequests int    `json:"llm_requests"`
	Tools       int    `json:"tools"`
	Errors      int    `json:"errors"`
	Tokens      Tokens `json:"tokens"`
}

type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

// Harness は課題 1 件分のハーネスの記録の有無（記録が無いのと 0 件を分ける。決めたこと 14）。
type Harness struct {
	Issue string `json:"issue"`
	File  string `json:"file"`  // 置き場の根からの相対パス
	State string `json:"state"` // none（ファイルが無い）・unreadable（あるが読めない）・empty（span が 0）・ok
	Spans int    `json:"spans"`
}

// Skip は読めずに飛ばした記録。
type Skip struct {
	File   string `json:"file"`
	Line   int    `json:"line,omitempty"`
	Reason string `json:"reason"`
}

// Trace は andon issue trace --json の出力。
type Trace struct {
	Project string    `json:"project"`
	Issue   string    `json:"issue"`
	Tree    *Node     `json:"tree"`
	Harness []Harness `json:"harness"`
	Skipped []Skip    `json:"skipped"`
}

// Options は読む所。
type Options struct {
	Root  string    // プロジェクトの根（kanban/ の親）
	Otel  string    // ハーネスの記録のプロジェクトの置き場（~/.local/state/andon/otel/<slug>）。空なら読まない
	State string    // dispatcher の控え（gone だけ読む）。空なら読まない
	Now   time.Time // 続いている段の終わり（--otlp だけが使う）
}

type reader struct {
	o       Options
	dir     string // kanban/
	skipped []Skip
}

func (r *reader) skip(path string, line int, format string, a ...any) {
	r.skipped = append(r.skipped, Skip{File: r.rel(path), Line: line, Reason: fmt.Sprintf(format, a...)})
}

// rel は読んだファイルのパスを、kanban か置き場か控えの根からの相対にする（絶対パスを出さない）。
func (r *reader) rel(path string) string {
	for _, base := range []string{r.o.Root, r.o.Otel} {
		if base == "" {
			continue
		}
		if rel, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(path)
}

// read はファイルを読む。無ければ ok が false（まだ無い記録）。ほかの理由で読めなければ飛ばした記録にする。
func (r *reader) read(path string) ([]byte, bool) {
	b, err := os.ReadFile(path)
	if err == nil {
		return b, true
	}
	if !errors.Is(err, fs.ErrNotExist) {
		why := "読み取りの誤り"
		var pe *fs.PathError
		if errors.As(err, &pe) {
			why = pe.Err.Error()
		}
		r.skip(path, 0, "読めない（%s）", why)
	}
	return nil, false
}

// ファイルの読み方の結果。
const (
	fileMissing    = iota // ファイルが無い
	fileRead              // 読んだ（途中の誤りは飛ばした所に出す）
	fileUnreadable        // あるが読めない（権限・フォルダ・読み切れない）
)

// lines は JSON Lines のファイルを 1 行ずつ渡す（空の行は飛ばす）。
func (r *reader) lines(path string, f func(n int, b []byte)) int {
	fh, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fileMissing
		}
		why := "読み取りの誤り"
		var pe *fs.PathError
		if errors.As(err, &pe) {
			why = pe.Err.Error()
		}
		r.skip(path, 0, "読めない（%s）", why)
		return fileUnreadable
	}
	defer fh.Close() //nolint:errcheck // 読むだけのファイル
	s := bufio.NewScanner(fh)
	s.Buffer(make([]byte, 64<<10), 64<<20)
	n := 0
	for s.Scan() {
		n++
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		f(n, s.Bytes())
	}
	if s.Err() != nil {
		r.skip(path, n+1, "読み切れない（%v）", s.Err())
		return fileUnreadable
	}
	return fileRead
}

func parseAt(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	return t.UTC(), err == nil
}

func tp(t time.Time) *time.Time { return &t }

// issueDoc は issue.yaml のうち、木に要る項目。
type issueDoc struct {
	ID      string
	Title   string   `yaml:"title"`
	Status  string   `yaml:"status"`
	Parent  string   `yaml:"parent"`
	Profile string   `yaml:"profile"`
	Links   []string `yaml:"links"`
	Created string   `yaml:"created"`
}

func (r *reader) issue(id string) (*issueDoc, bool) {
	path := filepath.Join(r.dir, "issues", id, "issue.yaml")
	b, ok := r.read(path)
	if !ok {
		return nil, false
	}
	var d issueDoc
	if err := yaml.Unmarshal(b, &d); err != nil {
		r.skip(path, 0, "issue.yaml が YAML でない")
		return nil, false
	}
	d.ID = id
	d.Parent = ledger.CanonID(d.Parent)
	return &d, true
}

// reviewers は parent が id でプロファイルがレビュアーの課題（id の順。頼み直しも全部）。
func (r *reader) reviewers(id string) []*issueDoc {
	dir := filepath.Join(r.dir, "issues")
	ents, err := os.ReadDir(dir)
	if err != nil {
		why := "読み取りの誤り"
		var pe *fs.PathError
		if errors.As(err, &pe) {
			why = pe.Err.Error()
		}
		r.skip(dir, 0, "一覧できない（%s）。レビュアーの課題を木に入れていない", why)
		return nil
	}
	var out []*issueDoc
	for _, e := range ents {
		if !e.IsDir() || e.Name() == id {
			continue
		}
		d, ok := r.peek(e.Name())
		if !ok || d.Parent != id || !strings.HasSuffix(d.Profile, "-review") {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return idLess(out[i].ID, out[j].ID) })
	return out
}

// peek は issue.yaml を読む。読めなくても飛ばした記録にしない（ほかの課題の issue.yaml を見て回るだけなので）。
func (r *reader) peek(id string) (*issueDoc, bool) {
	b, err := os.ReadFile(filepath.Join(r.dir, "issues", id, "issue.yaml"))
	if err != nil {
		return nil, false
	}
	var d issueDoc
	if yaml.Unmarshal(b, &d) != nil {
		return nil, false
	}
	d.ID = id
	d.Parent = ledger.CanonID(d.Parent)
	return &d, true
}

// idLess はレビュアーの回を数で並べる（A-0001-r2 < A-0001-r10）。
func idLess(a, b string) bool {
	pa, ka, oka := ledger.SplitRound(a)
	pb, kb, okb := ledger.SplitRound(b)
	if oka && okb && pa == pb {
		return ka < kb
	}
	return a < b
}

// Build は課題 id の木を組む。課題が無ければ誤り。
func Build(o Options, id string) (*Trace, error) {
	r := &reader{o: o, dir: filepath.Join(o.Root, "kanban")}
	id = ledger.CanonID(id)
	if _, err := os.Stat(filepath.Join(r.dir, "issues", id, "issue.yaml")); err != nil {
		return nil, fmt.Errorf("課題 %s が無い（kanban/issues/%s/issue.yaml）", id, id)
	}
	doc, ok := r.issue(id)
	if !ok {
		return nil, fmt.Errorf("課題 %s の issue.yaml が読めない", id)
	}
	rows := r.log()
	gone := r.gone(id)

	root := &Node{Kind: KindIssue, Name: doc.Title, Issue: id, Attrs: map[string]any{}}
	for k, v := range map[string]string{"status": doc.Status, "profile": doc.Profile, "parent": doc.Parent} {
		if v != "" {
			root.Attrs[k] = v
		}
	}
	if len(doc.Links) > 0 {
		root.Attrs["links"] = doc.Links
	}
	start, end := span1(rows, id)
	if start == nil {
		if t, ok := parseAt(doc.Created); ok {
			start = &t
		}
	}
	root.Start, root.End = start, end

	var stages []*Node
	stages = append(stages, columns(rows, id)...)
	lib := filepath.Join(r.dir, "issues", id, "library")
	launches := r.launches(id, end)
	stages = append(stages, launches...)
	stages = append(stages, r.runs(lib)...)
	stages = append(stages, r.questions(id)...)
	stages = append(stages, r.gates(lib)...)
	stages = append(stages, goneStages(rows, id, gone, end)...)
	stages = append(stages, r.merge(lib)...)

	extra, h := r.attach(id, launches)
	stages = append(stages, extra...)
	harness := []Harness{h}
	for _, rv := range r.reviewers(id) {
		node, h := r.review(rv, rows)
		stages = append(stages, node)
		harness = append(harness, h)
	}
	sortNodes(stages)
	root.Children = stages
	if r.skipped == nil {
		r.skipped = []Skip{}
	}
	return &Trace{Project: filepath.Base(o.Root), Issue: id, Tree: root, Harness: harness, Skipped: r.skipped}, nil
}

func sortNodes(ns []*Node) {
	sort.SliceStable(ns, func(i, j int) bool {
		a, b := ns[i], ns[j]
		switch {
		case a.Start == nil && b.Start == nil:
		case a.Start == nil:
			return false
		case b.Start == nil:
			return true
		case !a.Start.Equal(*b.Start):
			return a.Start.Before(*b.Start)
		}
		return order[a.Kind] < order[b.Kind]
	})
}

// log は kanban の log.jsonl を読む。JSON として読めない行（書き足しの途中で切れた最後の行など）は、飛ばした所に出す。
func (r *reader) log() []ledger.Row {
	var rows []ledger.Row
	path := filepath.Join(r.dir, "log.jsonl")
	r.lines(path, func(n int, b []byte) {
		var row ledger.Row
		if json.Unmarshal(b, &row) != nil {
			r.skip(path, n, "JSON として読めない。この行の出来事は段に入っていない")
			return
		}
		row.ID = ledger.CanonID(row.ID)
		rows = append(rows, row)
	})
	return rows
}

// statusOf は課題の列の行の status（問いの行と status の無い行は空）。
func statusOf(row ledger.Row) string {
	if row.Change == nil || row.Change["question"] != nil {
		return ""
	}
	s, _ := row.Change["status"].(string)
	return s
}

// span1 は課題が足された時刻と、最後に done になった時刻（今 done でなければ nil）。
func span1(rows []ledger.Row, id string) (start, end *time.Time) {
	last := ""
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		at, ok := parseAt(row.At)
		if !ok {
			continue
		}
		if _, add := row.Change["add"]; add && start == nil {
			start = tp(at)
		}
		if s := statusOf(row); s != "" {
			last = s
			if s == "done" {
				end = tp(at)
			}
		}
	}
	if last != "done" {
		end = nil
	}
	return start, end
}

// columns は列ごとの待ちと動き。足した行から最初の列の行までは「足した」（kanban は最初の列を行に書かない）。
// done は点の段にする。同じ列の行が続いても 1 つの段にする。
func columns(rows []ledger.Row, id string) []*Node {
	var out []*Node
	var cur *Node
	closeAt := func(at time.Time) {
		if cur != nil && cur.End == nil {
			cur.End = tp(at)
		}
	}
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		at, ok := parseAt(row.At)
		if !ok {
			continue
		}
		if _, add := row.Change["add"]; add && cur == nil {
			cur = &Node{Kind: KindColumn, Name: "列 足した（todo か queued）", Issue: id, Start: tp(at), Attrs: map[string]any{"column": ""}}
			out = append(out, cur)
			continue
		}
		s := statusOf(row)
		if s == "" || (cur != nil && cur.End == nil && cur.Attrs["column"] == s) {
			continue
		}
		closeAt(at)
		cur = &Node{Kind: KindColumn, Name: "列 " + s, Issue: id, Start: tp(at), Attrs: map[string]any{"column": s}}
		if s == "done" {
			cur.End = tp(at)
		}
		out = append(out, cur)
	}
	return out
}

// launch は起動の記録の 1 行のうち、木に要る項目（docs/contract.md §18）。
type launch struct {
	At       string  `json:"at"`
	Role     string  `json:"role"`
	Launch   string  `json:"launch"`
	Profile  string  `json:"profile"`
	Terminal string  `json:"terminal"`
	Session  *string `json:"session"`
	Harness  *struct {
		Name    string  `json:"name"`
		Version *string `json:"version"`
	} `json:"harness"`
	Package *struct {
		Commit *string `json:"commit"`
	} `json:"package"`
}

// launches は課題の起動の段。1 つの起動は次の起動まで（最後の起動は課題の終わりまで）。
func (r *reader) launches(id string, end *time.Time) []*Node {
	path := filepath.Join(r.dir, "issues", id, "library", "launches.jsonl")
	var out []*Node
	r.lines(path, func(n int, b []byte) {
		var l launch
		if err := json.Unmarshal(b, &l); err != nil {
			r.skip(path, n, "JSON の object でない")
			return
		}
		at, ok := parseAt(l.At)
		if !ok {
			r.skip(path, n, "at が時刻でない")
			return
		}
		attrs := map[string]any{"launch": l.Launch, "role": l.Role, "profile": l.Profile, "terminal": l.Terminal}
		if l.Session != nil {
			attrs["session"] = *l.Session
		}
		if l.Harness != nil {
			attrs["harness"] = l.Harness.Name
			if l.Harness.Version != nil {
				attrs["harness_version"] = *l.Harness.Version
			}
		}
		if l.Package != nil && l.Package.Commit != nil {
			attrs["package_commit"] = *l.Package.Commit
		}
		name := fmt.Sprintf("起動 %s %s（%s・%s）", l.Launch, l.Terminal, l.Role, l.Profile)
		out = append(out, &Node{Kind: KindLaunch, Name: name, Issue: id, Start: tp(at), Attrs: attrs})
	})
	sortNodes(out)
	for i, l := range out {
		if i+1 < len(out) {
			l.End = out[i+1].Start
		} else if end != nil {
			l.End = tp(*end)
		}
	}
	return out
}

// runs は走らせた command の段（docs/contract.md §24）。
func (r *reader) runs(lib string) []*Node {
	var out []*Node
	for _, path := range r.files(filepath.Join(lib, "runs"), ".json") {
		b, ok := r.read(path)
		if !ok {
			continue
		}
		var v struct {
			ID         int      `json:"id"`
			Command    []string `json:"command"`
			Expect     string   `json:"expect"`
			ExitCode   *int     `json:"exit_code"`
			Commit     string   `json:"commit"`
			Dirty      *bool    `json:"dirty"`
			StartedAt  string   `json:"started_at"`
			FinishedAt string   `json:"finished_at"`
		}
		if json.Unmarshal(b, &v) != nil {
			r.skip(path, 0, "JSON の object でない")
			continue
		}
		from, ok1 := parseAt(v.StartedAt)
		to, ok2 := parseAt(v.FinishedAt)
		if !ok1 || !ok2 {
			r.skip(path, 0, "started_at か finished_at が時刻でない")
			continue
		}
		attrs := map[string]any{"id": v.ID, "command": v.Command, "expect": v.Expect}
		if v.ExitCode != nil {
			attrs["exit_code"] = *v.ExitCode
		}
		if v.Commit != "" {
			attrs["commit"] = v.Commit
		}
		if v.Dirty != nil {
			attrs["dirty"] = *v.Dirty
		}
		name := fmt.Sprintf("走らせた %d: %s", v.ID, strings.Join(v.Command, " "))
		out = append(out, &Node{Kind: KindRun, Name: name, Start: tp(from), End: tp(to), Attrs: attrs})
	}
	return out
}

// files はフォルダの中の ext で終わるファイル（名前の順）。フォルダが無ければ空。
func (r *reader) files(dir, ext string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			r.skip(dir, 0, "フォルダを読めない")
		}
		return nil
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// questions は問いの段（asked から answered まで。答えが無ければ続いている）。kind と from は questions/<n>.json からも読む。
func (r *reader) questions(id string) []*Node {
	path := filepath.Join(r.dir, "issues", id, "questions.yaml")
	b, ok := r.read(path)
	if !ok {
		return nil
	}
	var qs []struct {
		N         int    `yaml:"n"`
		Kind      string `yaml:"kind"`
		Asked     string `yaml:"asked"`
		Answered  string `yaml:"answered"`
		Status    any    `yaml:"status"`
		DecidedBy any    `yaml:"decidedBy"`
	}
	if yaml.Unmarshal(b, &qs) != nil {
		r.skip(path, 0, "questions.yaml を読めない")
		return nil
	}
	var out []*Node
	for _, q := range qs {
		from, ok := parseAt(q.Asked)
		if !ok {
			r.skip(path, 0, "問い %d の asked が時刻でない", q.N)
			continue
		}
		attrs := map[string]any{"n": q.N}
		if q.Kind != "" {
			attrs["kind"] = q.Kind
		}
		if s, ok := q.Status.(string); ok {
			attrs["status"] = s
		}
		if s, ok := q.DecidedBy.(string); ok {
			attrs["decided_by"] = s
		}
		if qb, ok := r.read(filepath.Join(r.dir, "issues", id, "library", "questions", strconv.Itoa(q.N)+".json")); ok {
			var v struct {
				Kind string `json:"kind"`
				From string `json:"from"`
			}
			if json.Unmarshal(qb, &v) == nil {
				if v.Kind != "" {
					attrs["kind"] = v.Kind
				}
				if v.From != "" {
					attrs["from"] = v.From
				}
			}
		}
		node := &Node{Kind: KindQuestion, Name: fmt.Sprintf("問い %d", q.N), Start: tp(from), Attrs: attrs}
		if k, _ := attrs["kind"].(string); k != "" {
			node.Name += "（" + k + "）"
		}
		if to, ok := parseAt(q.Answered); ok {
			node.End = tp(to)
		}
		out = append(out, node)
	}
	return out
}

// gates は gate の判定の段（点。docs/contract.md §7）。
func (r *reader) gates(lib string) []*Node {
	var out []*Node
	for _, path := range r.files(filepath.Join(lib, "gate"), ".json") {
		b, ok := r.read(path)
		if !ok {
			continue
		}
		var v struct {
			At       string   `json:"at"`
			Head     string   `json:"head"`
			PR       string   `json:"pr"`
			Tier     string   `json:"tier"`
			Decision string   `json:"decision"`
			Reasons  []string `json:"reasons"`
			Merged   bool     `json:"merged"`
		}
		if json.Unmarshal(b, &v) != nil {
			r.skip(path, 0, "gate の記録が JSON の object でない")
			continue
		}
		at, ok := parseAt(v.At)
		if !ok || v.Decision == "" {
			r.skip(path, 0, "gate の記録に at か decision が無い")
			continue
		}
		head := v.Head
		if head == "" {
			head = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		attrs := map[string]any{"head": head, "decision": v.Decision, "tier": v.Tier, "merged": v.Merged}
		if v.PR != "" {
			attrs["pr"] = v.PR
		}
		if len(v.Reasons) > 0 {
			attrs["reasons"] = v.Reasons
		}
		out = append(out, &Node{Kind: KindGate, Name: fmt.Sprintf("gate %s（%s）", v.Decision, v.Tier), Start: tp(at), End: tp(at), Attrs: attrs})
	}
	return out
}

// gone は dispatcher の控えの gone のうち課題 id の分（端末が Gone だと見た時刻）。
func (r *reader) gone(id string) []goneAt {
	if r.o.State == "" {
		return nil
	}
	b, ok := r.read(r.o.State)
	if !ok {
		return nil
	}
	var v struct {
		Gone map[string][]goneAt `json:"gone"`
	}
	if json.Unmarshal(b, &v) != nil {
		r.skip(r.o.State, 0, "dispatcher の控えが JSON の object でない。端末が死んでいた間を出していない")
		return nil
	}
	var out []goneAt
	for k, gs := range v.Gone {
		if ledger.CanonID(k) == id {
			out = append(out, gs...)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

type goneAt struct {
	Agent string `json:"agent"`
	At    int64  `json:"at"` // ミリ秒
}

// goneStages は端末が死んでいた間の段。終わりは課題の次の agent の行（無ければ課題の終わり）。数え方は ledger.Worked と同じ。
func goneStages(rows []ledger.Row, id string, gone []goneAt, end *time.Time) []*Node {
	var agents []time.Time
	for _, row := range rows {
		if row.ID != id || row.Change == nil {
			continue
		}
		if _, ok := row.Change["agent"]; ok {
			if at, ok := parseAt(row.At); ok {
				agents = append(agents, at)
			}
		}
	}
	var out []*Node
	for _, g := range gone {
		from := time.UnixMilli(g.At).UTC()
		node := &Node{Kind: KindGone, Name: "端末が死んでいた（" + g.Agent + "）", Start: tp(from), Attrs: map[string]any{"agent": g.Agent}}
		for _, a := range agents {
			if !a.Before(from) {
				node.End = tp(a)
				break
			}
		}
		if node.End == nil && end != nil {
			node.End = tp(*end)
		}
		out = append(out, node)
	}
	return out
}

// merge はマージの段（点。outcome.json の merged_at。docs/contract.md §19）。
func (r *reader) merge(lib string) []*Node {
	path := filepath.Join(lib, "outcome.json")
	b, ok := r.read(path)
	if !ok {
		return nil
	}
	var v struct {
		Result      string  `json:"result"`
		PR          *string `json:"pr"`
		MergeCommit *string `json:"merge_commit"`
		MergedAt    *string `json:"merged_at"`
	}
	if json.Unmarshal(b, &v) != nil {
		r.skip(path, 0, "JSON の object でない")
		return nil
	}
	if v.MergedAt == nil {
		return nil
	}
	at, ok := parseAt(*v.MergedAt)
	if !ok {
		r.skip(path, 0, "merged_at が時刻でない")
		return nil
	}
	attrs := map[string]any{"result": v.Result}
	if v.PR != nil {
		attrs["pr"] = *v.PR
	}
	if v.MergeCommit != nil {
		attrs["merge_commit"] = *v.MergeCommit
	}
	return []*Node{{Kind: KindMerge, Name: "マージ", Start: tp(at), End: tp(at), Attrs: attrs}}
}

// review はレビュアーの課題 1 件の段。足した行から、最初の review か done の行まで（ledger.Worked のレビュー待ちと同じ区間）。
// 下にはレビュアーの起動と、その起動のハーネスの記録が入る。
func (r *reader) review(d *issueDoc, rows []ledger.Row) (*Node, Harness) {
	node := &Node{Kind: KindReview, Name: "レビュー " + d.ID, Issue: d.ID, Attrs: map[string]any{"profile": d.Profile}}
	if d.Status != "" {
		node.Attrs["status"] = d.Status
	}
	for _, row := range rows {
		if row.ID != d.ID {
			continue
		}
		at, ok := parseAt(row.At)
		if !ok {
			continue
		}
		if _, add := row.Change["add"]; add && node.Start == nil {
			node.Start = tp(at)
			continue
		}
		// 足した行が無い課題（ルーチンが足した課題・行が読めなかった課題）でも、最初の review か done の行で終える
		if s := statusOf(row); node.End == nil && (s == "review" || s == "done") && (node.Start == nil || !at.Before(*node.Start)) {
			node.End = tp(at)
		}
	}
	if node.Start == nil {
		if t, ok := parseAt(d.Created); ok {
			node.Start = &t
		}
	}
	if node.Start != nil && node.End != nil && node.End.Before(*node.Start) {
		node.End = tp(*node.Start)
	}
	path := filepath.Join(r.dir, "issues", d.ID, "library", "findings.json")
	if b, ok := r.read(path); ok {
		var v struct {
			Commit   string `json:"commit"`
			Findings []struct {
				Severity    string `json:"severity"`
				Preexisting bool   `json:"preexisting"`
			} `json:"findings"`
		}
		if json.Unmarshal(b, &v) != nil {
			r.skip(path, 0, "JSON の object でない")
		} else {
			bySev := map[string]int{}
			for _, f := range v.Findings {
				bySev[f.Severity]++
			}
			node.Attrs["commit"] = v.Commit
			node.Attrs["findings"] = len(v.Findings)
			node.Attrs["severity"] = bySev
		}
	}
	launches := r.launches(d.ID, node.End)
	extra, h := r.attach(d.ID, launches)
	node.Children = append(launches, extra...)
	node.Children = append(node.Children, goneStages(rows, d.ID, r.gone(d.ID), node.End)...)
	sortNodes(node.Children)
	return node, h
}
