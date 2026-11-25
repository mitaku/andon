package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 偽の kanban と偽の置き場（ADR-0060 受け入れ条件 8〜10 の継ぎ目）。セルの本物は読まない。

type fixture struct {
	root  string // プロジェクトの根（kanban/ の親。名前が slug）
	otel  string // 置き場のプロジェクトのフォルダ
	state string // dispatcher の控え
}

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func at(hhmm string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05Z", "2026-11-01T"+hhmm+"Z")
	if err != nil {
		panic(err)
	}
	return t
}

// spec は置き場の span 1 つ（受け口が書く OTLP の JSON の形にする）。
type spec struct {
	trace, id, parent, name, session string
	from, to                         string // HH:MM:SS
	attrs                            map[string]any
}

func (s spec) json() map[string]any {
	attrs := []any{map[string]any{"key": "session.id", "value": map[string]any{"stringValue": s.session}}}
	for k, v := range s.attrs {
		var val map[string]any
		switch x := v.(type) {
		case int:
			val = map[string]any{"intValue": fmt.Sprint(x)}
		case bool:
			val = map[string]any{"boolValue": x}
		default:
			val = map[string]any{"stringValue": fmt.Sprint(x)}
		}
		attrs = append(attrs, map[string]any{"key": k, "value": val})
	}
	return map[string]any{
		"traceId": s.trace, "spanId": s.id, "parentSpanId": s.parent, "name": s.name, "kind": 1,
		"startTimeUnixNano": fmt.Sprint(at(s.from).UnixNano()), "endTimeUnixNano": fmt.Sprint(at(s.to).UnixNano()),
		"attributes": attrs,
	}
}

// line は置き場の 1 行（resource 1 つ分）。
func line(t *testing.T, issue, role string, spans ...spec) string {
	t.Helper()
	var ss []any
	for _, s := range spans {
		ss = append(ss, s.json())
	}
	res := map[string]any{"attributes": []any{
		map[string]any{"key": "andon.project", "value": map[string]any{"stringValue": "sample-site"}},
		map[string]any{"key": "andon.issue", "value": map[string]any{"stringValue": issue}},
		map[string]any{"key": "andon.role", "value": map[string]any{"stringValue": role}},
		map[string]any{"key": "service.name", "value": map[string]any{"stringValue": "claude-code"}},
	}}
	b, err := json.Marshal(map[string]any{"resourceSpans": []any{map[string]any{
		"resource":   res,
		"scopeSpans": []any{map[string]any{"scope": map[string]any{"name": "com.anthropic.claude_code.tracing"}, "spans": ss}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

const (
	sessA = "0f8fad5b-d9cb-469f-a165-70867728950e"
	sessB = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	sessR = "3d2e1f08-9c4e-1b0d-7a3f-5e2c8b6d4a1f"
	head  = "3d2e1f089c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a"
)

// newFixture は課題 S-0001 の一生を偽の記録で作る。担当は 2 回起動され（2 回目は新しいセッション）、
// レビュアーの課題 S-0001-r1 が 1 回レビューし、gate が approve して、マージされた。
func newFixture(t *testing.T, harness bool) fixture {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	t.Setenv("ANDON_WORKSPACE", filepath.Join(base, "workspace"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	f := fixture{
		root:  filepath.Join(base, "projects", "sample-site"),
		otel:  filepath.Join(base, "state", "andon", "otel", "sample-site"),
		state: filepath.Join(base, "state", "andon", "dispatcher", "sample-site.json"),
	}
	k := filepath.Join(f.root, "kanban")
	t.Setenv("KANBAN_ROOT", f.root)
	put(t, filepath.Join(k, "config.yaml"), "version: 3\n")
	put(t, filepath.Join(k, "log.jsonl"), strings.Join([]string{
		`{"at":"2026-11-01T00:50:00Z","by":"terminal:cos","id":"S-0001","change":{"add":"返事の下書きボタン"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"doing","agent":"t1"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"doing"}}`,
		`{"at":"2026-11-01T01:40:00Z","by":"terminal:dispatcher","id":"S-0001-r1","change":{"add":"S-0001 のレビュー（1 回目）"}}`,
		`{"at":"2026-11-01T01:41:00Z","by":"terminal:r1","id":"S-0001-r1","change":{"status":"doing","agent":"r1"}}`,
		`{"at":"2026-11-01T01:50:00Z","by":"terminal:r1","id":"S-0001-r1","change":{"status":"review"}}`,
		`{"at":"2026-11-01T02:10:00Z","by":"terminal:t2","id":"S-0001","change":{"agent":"t2"}}`,
		`{"at":"2026-11-01T02:30:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"review"}}`,
		`{"at":"2026-11-01T02:31:00Z","by":"terminal:dispatcher","id":"S-0001","change":{"question":1,"status":"open"}}`,
		`{"at":"2026-11-01T02:50:00Z","by":"terminal:dispatcher","id":"S-0001-r1","change":{"status":"done"}}`,
		`{"at":"2026-11-01T02:51:00Z","by":"terminal:dispatcher","id":"S-0001","change":{"status":"done"}}`,
	}, "\n")+"\n")
	put(t, filepath.Join(k, "issues", "S-0001", "issue.yaml"),
		"id: S-0001\ntitle: 返事の下書きボタン\nstatus: done\nparent: C-0001\nprofile: claude\nlinks: [sig-1]\ncreated: \"2026-11-01T00:50:00Z\"\n")
	lib := filepath.Join(k, "issues", "S-0001", "library")
	put(t, filepath.Join(lib, "launches.jsonl"),
		`{"at":"2026-11-01T01:00:00Z","role":"worker","launch":"start","issue":"S-0001","profile":"claude","template":"claude","command":null,"terminal":"t1","session":"`+sessA+`","harness":{"name":"claude","version":"2.1.331 (Claude Code)"},"package":null}`+"\n"+
			`{"at":"2026-11-01T02:10:00Z","role":"worker","launch":"fresh","issue":"S-0001","profile":"claude","template":"claude","command":null,"terminal":"t2","session":"`+sessB+`","harness":{"name":"claude","version":null},"package":null}`+"\n")
	put(t, filepath.Join(lib, "runs", "1.json"),
		`{"id":1,"command":["go","test","./..."],"expect":"pass","exit_code":0,"commit":"`+head+`","dirty":false,"started_at":"2026-11-01T01:30:00Z","finished_at":"2026-11-01T01:31:30Z","output":"runs/1.log","output_bytes":10,"truncated":false}`)
	put(t, filepath.Join(lib, "gate", head+".json"),
		`{"at":"2026-11-01T02:40:00Z","head":"`+head+`","pr":"https://github.com/example/sample-site/pull/12","tier":"B","decision":"approve","reasons":[],"merged":true}`)
	put(t, filepath.Join(k, "issues", "S-0001", "questions.yaml"),
		"- n: 1\n  text: マージするか\n  kind: line\n  asked: \"2026-11-01T02:31:00Z\"\n  answered: \"2026-11-01T02:45:00Z\"\n  status: received\n  decidedBy: owner\n")
	put(t, filepath.Join(lib, "questions", "1.json"), `{"kind":"line","question":"マージするか","options":[{"label":"する","pick":true},{"label":"しない"}],"from":"gate"}`)
	put(t, filepath.Join(lib, "outcome.json"),
		`{"at":"2026-11-01T02:52:00Z","issue":"S-0001","result":"merged","pr":"https://github.com/example/sample-site/pull/12","merge_commit":"`+head+`","merged_at":"2026-11-01T02:46:00Z","gates":[],"reviewers":1,"worked_minutes":80,"sessions":null}`)
	put(t, f.state, fmt.Sprintf(`{"gone": {"S-0001": [{"agent": "t1", "at": %d}]}}`, at("02:00:00").UnixMilli()))

	rk := filepath.Join(k, "issues", "S-0001-r1")
	put(t, filepath.Join(rk, "issue.yaml"),
		"id: S-0001-r1\ntitle: S-0001 のレビュー（1 回目）\nstatus: done\nparent: S-0001\nprofile: claude-review\ncreated: \"2026-11-01T01:40:00Z\"\n")
	put(t, filepath.Join(rk, "library", "launches.jsonl"),
		`{"at":"2026-11-01T01:41:00Z","role":"reviewer","launch":"start","issue":"S-0001-r1","profile":"claude-review","template":"claude-review","command":null,"terminal":"r1","session":"`+sessR+`","harness":{"name":"claude","version":null},"package":null}`+"\n")
	put(t, filepath.Join(rk, "library", "findings.json"), `{"commit":"`+head+`","findings":[{"severity":"major","file":"a.go","line":3},{"severity":"minor","file":"b.go","line":1}]}`)
	// 親の違うほかの課題（木に入らない）
	put(t, filepath.Join(k, "issues", "S-0002-r1", "issue.yaml"), "id: S-0002-r1\ntitle: よそのレビュー\nparent: S-0002\nprofile: claude-review\n")

	if harness {
		a := "c43ee97f7dd8f491fa3848a4438c5d81"
		b := "9a1f0e9c7b5a3d2e1f089c4e1b0d7a3f"
		r := "5e2c8b6d4a1f0e9c7b5a3d2e1f089c4e"
		w1 := line(t, "S-0001", "worker",
			spec{a, "549d37a123c5b63a", "", "claude_code.interaction", sessA, "01:01:00", "01:05:00", map[string]any{"interaction.sequence": 1}},
			spec{a, "8a71becae261911e", "549d37a123c5b63a", "claude_code.llm_request", sessA, "01:01:01", "01:01:30",
				map[string]any{"input_tokens": 2, "output_tokens": 54, "cache_read_tokens": 60008, "cache_creation_tokens": 18424, "success": true}},
			spec{a, "e697f90cc69f184d", "549d37a123c5b63a", "claude_code.tool", sessA, "01:01:31", "01:02:00", map[string]any{"tool_name": "Bash"}},
			spec{a, "6a787bba44dd65cb", "e697f90cc69f184d", "claude_code.tool.execution", sessA, "01:01:32", "01:01:59", nil},
		)
		w2 := line(t, "S-0001", "worker",
			spec{b, "1111111111111111", "", "claude_code.interaction", sessB, "02:11:00", "02:20:00", map[string]any{"interaction.sequence": 1}},
			spec{b, "2222222222222222", "1111111111111111", "claude_code.llm_request", sessB, "02:11:01", "02:12:00",
				map[string]any{"input_tokens": 10, "output_tokens": 100, "cache_read_tokens": 1000, "cache_creation_tokens": 10, "success": false}},
			// 同じ span id が別の trace にもある（--otlp で 1 本に縫い直しても重ならないこと）
			spec{b, "e697f90cc69f184d", "1111111111111111", "claude_code.tool", sessB, "02:12:01", "02:13:00", map[string]any{"tool_name": "Read"}},
		)
		// 受け口が同じ要求を 2 度書いた行（1 つにする）と、書く途中で切れた最後の行（飛ばす）
		put(t, filepath.Join(f.otel, "issues", "S-0001", "traces.jsonl"), w1+w2+w1+`{"resourceSpans":[{"resource":`)
		put(t, filepath.Join(f.otel, "issues", "S-0001-r1", "traces.jsonl"), line(t, "S-0001-r1", "reviewer",
			spec{r, "3333333333333333", "", "claude_code.interaction", sessR, "01:42:00", "01:48:00", map[string]any{"interaction.sequence": 1}},
			spec{r, "4444444444444444", "3333333333333333", "claude_code.llm_request", sessR, "01:42:01", "01:43:00",
				map[string]any{"input_tokens": 5, "output_tokens": 50, "cache_read_tokens": 500, "cache_creation_tokens": 5}},
		))
	}
	return f
}

func run(t *testing.T, f fixture, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb, func(slug string) string {
		if slug != "sample-site" {
			t.Errorf("控えを引く slug が %q", slug)
		}
		return f.state
	})
	return out.String(), errb.String(), code
}

func decode(t *testing.T, s string) *Trace {
	t.Helper()
	var tr Trace
	if err := json.Unmarshal([]byte(s), &tr); err != nil {
		t.Fatalf("--json が読めない: %v\n%s", err, s)
	}
	return &tr
}

func find(ns []*Node, kind string) []*Node {
	var out []*Node
	for _, n := range ns {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// 受け入れ条件 8: 決めたこと 12 の表の段が時刻の順に出る。起動の下にその session.id の interaction が入り、
// その下にモデルの呼び出しとツールが入る。レビュアーの課題の起動と、レビュアーの課題のフォルダの記録も同じ木に入る。
func TestTraceJSONTree(t *testing.T) {
	f := newFixture(t, true)
	out, errs, code := run(t, f, "S-1", "--json")
	if code != 0 {
		t.Fatalf("終了コード %d: %s", code, errs)
	}
	tr := decode(t, out)
	if tr.Issue != "S-0001" || tr.Project != "sample-site" || tr.Tree.Kind != KindIssue || tr.Tree.Name != "返事の下書きボタン" {
		t.Fatalf("根が違う: %+v", tr.Tree)
	}
	if tr.Tree.Attrs["parent"] != "C-0001" || fmt.Sprint(tr.Tree.Attrs["links"]) != "[sig-1]" {
		t.Errorf("parent と links を出していない: %v", tr.Tree.Attrs)
	}
	stages := tr.Tree.Children
	// 表の 8 つの段が全部ある
	for _, kind := range []string{KindColumn, KindLaunch, KindRun, KindReview, KindGate, KindQuestion, KindGone, KindMerge} {
		if len(find(stages, kind)) == 0 {
			t.Errorf("段 %s が無い", kind)
		}
	}
	// 時刻の順
	for i := 1; i < len(stages); i++ {
		if stages[i].Start.Before(*stages[i-1].Start) {
			t.Errorf("段が時刻の順でない: %s（%s）が %s（%s）の後", stages[i].Name, stages[i].Start, stages[i-1].Name, stages[i-1].Start)
		}
	}
	var names []string
	for _, s := range stages {
		names = append(names, s.Name)
	}
	want := []string{
		"列 足した（todo か queued）", "列 doing", "起動 start t1（worker・claude）", "走らせた 1: go test ./...",
		"レビュー S-0001-r1", "端末が死んでいた（t1）", "起動 fresh t2（worker・claude）", "列 review", "問い 1（line）",
		"gate approve（B）", "マージ", "列 done",
	}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Errorf("段の並びが違う:\n got  %q\n want %q", names, want)
	}
	// 段の区間
	cols := find(stages, KindColumn)
	if !cols[1].Start.Equal(at("01:00:00")) || !cols[1].End.Equal(at("02:30:00")) {
		t.Errorf("列 doing の区間が違う（同じ列の行が続いても 1 つ）: %v〜%v", cols[1].Start, cols[1].End)
	}
	if g := find(stages, KindGone)[0]; !g.End.Equal(at("02:10:00")) {
		t.Errorf("端末が死んでいた間の終わりが次の agent の行でない: %v", g.End)
	}
	if q := find(stages, KindQuestion)[0]; !q.End.Equal(at("02:45:00")) || q.Attrs["from"] != "gate" || q.Attrs["decided_by"] != "owner" {
		t.Errorf("問いの段が違う: %v %v", q.End, q.Attrs)
	}
	launches := find(stages, KindLaunch)
	if !launches[0].End.Equal(at("02:10:00")) || !launches[1].End.Equal(at("02:51:00")) {
		t.Errorf("起動の区間が次の起動と課題の終わりまでになっていない: %v %v", launches[0].End, launches[1].End)
	}
	// 起動の下にその session の interaction、その下にモデルの呼び出しとツール
	for i, sess := range []string{sessA, sessB} {
		l := launches[i]
		if len(l.Children) != 1 || l.Children[0].Kind != KindInteraction || l.Children[0].Attrs["session.id"] != sess {
			t.Fatalf("起動 %d の下に session %s の interaction が 1 つでない: %+v", i, sess, l.Children)
		}
		kids := l.Children[0].Children
		if len(kids) != 2 || kids[0].Kind != KindLLM || kids[1].Kind != KindTool {
			t.Errorf("interaction の下がモデルの呼び出しとツールの順でない: %+v", kids)
		}
	}
	if ex := launches[0].Children[0].Children[1].Children; len(ex) != 1 || ex[0].Name != "claude_code.tool.execution" {
		t.Errorf("ツールの下の span が入っていない: %+v", ex)
	}
	if s := launches[0].Summary; s == nil || s.LLMRequests != 1 || s.Tools != 1 || s.Tokens.CacheRead != 60008 || s.Spans != 4 {
		t.Errorf("同じ要求が 2 度書かれても 1 つに数える: %+v", s)
	}
	if s := launches[1].Summary; s == nil || s.Errors != 1 {
		t.Errorf("success が false のモデルの呼び出しを失敗に数えていない: %+v", s)
	}
	// レビュアーの課題の起動と記録が、レビューの段の下に入る
	rv := find(stages, KindReview)
	if len(rv) != 1 || rv[0].Issue != "S-0001-r1" || !rv[0].End.Equal(at("01:50:00")) || rv[0].Attrs["findings"] != float64(2) {
		t.Fatalf("レビューの段が違う（よその課題のレビュアーを入れていないか）: %+v", rv)
	}
	rl := rv[0].Children
	if len(rl) != 1 || rl[0].Kind != KindLaunch || rl[0].Attrs["session"] != sessR {
		t.Fatalf("レビューの段の下にレビュアーの起動が無い: %+v", rl)
	}
	if len(rl[0].Children) != 1 || rl[0].Children[0].Attrs["session.id"] != sessR || rl[0].Children[0].Children[0].Kind != KindLLM {
		t.Errorf("レビュアーの起動の下に、レビュアーの課題のフォルダの記録が無い: %+v", rl[0].Children)
	}
	// 記録の有無と、飛ばした行
	if len(tr.Harness) != 2 || tr.Harness[0].State != "ok" || tr.Harness[0].Spans != 7 || tr.Harness[1].Issue != "S-0001-r1" || tr.Harness[1].State != "ok" {
		t.Errorf("ハーネスの記録の有無が違う: %+v", tr.Harness)
	}
	if len(tr.Skipped) != 1 || tr.Skipped[0].File != "issues/S-0001/traces.jsonl" || tr.Skipped[0].Line != 4 {
		t.Errorf("切れた行を飛ばした記録が違う: %+v", tr.Skipped)
	}
	// 表も同じ木から出る（プロンプト 1 つ分まで）
	table, _, code := run(t, f, "S-0001")
	if code != 0 || !strings.Contains(table, "プロンプト 1") || strings.Contains(table, "claude_code.tool.execution") ||
		!strings.Contains(table, "    起動 start r1（reviewer・claude-review）") || !strings.Contains(table, "ハーネスの記録: S-0001-r1 span 2 件") {
		t.Errorf("表が違う:\n%s", table)
	}
}

// 受け入れ条件 9: --otlp の全部の span の traceId が同じで、親の無い span が 1 つだけ。2 回打っても traceId が同じ。
func TestTraceOTLPOneTrace(t *testing.T) {
	f := newFixture(t, true)
	first, errs, code := run(t, f, "S-0001", "--otlp")
	if code != 0 {
		t.Fatalf("終了コード %d: %s", code, errs)
	}
	if strings.Count(first, "\n") != 1 {
		t.Errorf("--otlp が 1 行でない（閲覧器の JSON の行の形）")
	}
	type otlp struct {
		ResourceSpans []struct {
			Resource   map[string]any `json:"resource"`
			ScopeSpans []struct {
				Spans []struct {
					TraceID string           `json:"traceId"`
					SpanID  string           `json:"spanId"`
					Parent  string           `json:"parentSpanId"`
					Name    string           `json:"name"`
					Start   string           `json:"startTimeUnixNano"`
					End     string           `json:"endTimeUnixNano"`
					Attrs   []map[string]any `json:"attributes"`
				} `json:"spans"`
			} `json:"scopeSpans"`
		} `json:"resourceSpans"`
	}
	read := func(s string) (traces map[string]bool, ids map[string]string, roots []string, n int) {
		var v otlp
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			t.Fatalf("--otlp が読めない: %v", err)
		}
		traces, ids = map[string]bool{}, map[string]string{}
		parents := map[string]string{}
		for _, rs := range v.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, sp := range ss.Spans {
					n++
					traces[sp.TraceID] = true
					if _, dup := ids[sp.SpanID]; dup {
						t.Errorf("span の id が重なる: %s", sp.SpanID)
					}
					ids[sp.SpanID] = sp.Name
					parents[sp.SpanID] = sp.Parent
					if sp.Parent == "" {
						roots = append(roots, sp.Name)
					}
					if len(sp.TraceID) != 32 || len(sp.SpanID) != 16 {
						t.Errorf("id の長さが OTLP の形でない: %q %q", sp.TraceID, sp.SpanID)
					}
				}
			}
		}
		for id, p := range parents {
			if _, ok := ids[p]; p != "" && !ok {
				t.Errorf("span %s（%s）の親 %s が出力に無い", id, ids[id], p)
			}
		}
		return traces, ids, roots, n
	}
	traces, ids, roots, n := read(first)
	if len(traces) != 1 {
		t.Errorf("traceId が 1 つでない: %v", traces)
	}
	if len(roots) != 1 || roots[0] != "返事の下書きボタン" {
		t.Errorf("親の無い span が課題の 1 つでない: %v", roots)
	}
	// 段 12（担当の起動 2 とレビュー 1 を含む）・レビュアーの起動 1・課題 1 = 14、ハーネス 9（2 度書かれた 4 は 1 回）
	if n != 23 {
		t.Errorf("span の数が %d（段と起動と課題 14 とハーネス 9 の 23 のはず）: %v", n, ids)
	}
	second, _, _ := run(t, f, "S-0001", "--otlp")
	traces2, ids2, _, _ := read(second)
	for id := range traces {
		if !traces2[id] || id != TraceID("sample-site", "S-0001") {
			t.Errorf("2 回目の traceId が違う: %v %v", traces, traces2)
		}
	}
	if len(ids2) != len(ids) {
		t.Errorf("2 回目の span の数が違う")
	}
	for id := range ids {
		if _, ok := ids2[id]; !ok {
			t.Errorf("2 回目に span %s が無い（span の id も課題から決まる値）", id)
		}
	}
	// ハーネスの根の span の親は起動の span、元の属性は残る
	var v otlp
	_ = json.Unmarshal([]byte(first), &v)
	byID := map[string]string{}
	for _, rs := range v.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				byID[sp.SpanID] = sp.Name
			}
		}
	}
	for _, rs := range v.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				if sp.Name == "claude_code.interaction" && !strings.HasPrefix(byID[sp.Parent], "起動 ") {
					t.Errorf("interaction の親が起動でない: %s", byID[sp.Parent])
				}
			}
		}
	}
	if !strings.Contains(first, `"service.name","value":{"stringValue":"claude-code"}`) {
		t.Errorf("送り手の resource を残していない")
	}
}

// 受け入れ条件 10: 置き場に記録の無い課題では、段が出て、ハーネスの記録が無いという 1 行が出て、終了コードが 0。
// 0 件（ファイルはあるが span が無い）とは分ける。
func TestTraceNoHarness(t *testing.T) {
	f := newFixture(t, false)
	out, errs, code := run(t, f, "S-0001")
	if code != 0 {
		t.Fatalf("終了コード %d: %s", code, errs)
	}
	for _, s := range []string{"列 doing", "起動 start t1（worker・claude）", "gate approve（B）", "マージ",
		"ハーネスの記録が無い: S-0001（置き場に issues/S-0001/traces.jsonl が無い）",
		"ハーネスの記録が無い: S-0001-r1（置き場に issues/S-0001-r1/traces.jsonl が無い）"} {
		if !strings.Contains(out, s) {
			t.Errorf("表に %q が無い:\n%s", s, out)
		}
	}
	js, _, code := run(t, f, "S-0001", "--json")
	tr := decode(t, js)
	if code != 0 || tr.Harness[0].State != "none" || len(tr.Tree.Children) == 0 {
		t.Errorf("--json の記録の有無が none でない: %+v", tr.Harness)
	}
	otlpOut, _, code := run(t, f, "S-0001", "--otlp")
	if code != 0 || !strings.Contains(otlpOut, `"name":"列 doing"`) {
		t.Errorf("記録の無い課題でも --otlp に段が出る: %s", otlpOut)
	}

	put(t, filepath.Join(f.otel, "issues", "S-0001", "traces.jsonl"), "")
	out, _, code = run(t, f, "S-0001")
	if code != 0 || !strings.Contains(out, "ハーネスの記録が 0 件: S-0001") || strings.Contains(out, "ハーネスの記録が無い: S-0001（") {
		t.Errorf("空のファイルを「記録が無い」と分けていない:\n%s", out)
	}
}

// 送り手の誤りで span の親子が輪になっても止まらず、全部の span を出し、--otlp の親の無い span は 1 つのまま。
func TestTraceSpanCycle(t *testing.T) {
	f := newFixture(t, false)
	c := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	put(t, filepath.Join(f.otel, "issues", "S-0001", "traces.jsonl"), line(t, "S-0001", "worker",
		spec{c, "000000000000000a", "000000000000000b", "claude_code.tool", sessA, "01:10:00", "01:11:00", nil},
		spec{c, "000000000000000b", "000000000000000a", "claude_code.llm_request", sessA, "01:10:10", "01:10:20", nil},
		spec{c, "000000000000000c", "000000000000000c", "claude_code.interaction", sessA, "01:12:00", "01:13:00", nil},
	))
	done := make(chan struct{})
	var js, ot string
	go func() {
		defer close(done)
		js, _, _ = run(t, f, "S-0001", "--json")
		ot, _, _ = run(t, f, "S-0001", "--otlp")
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("輪になった span で止まらない")
	}
	tr := decode(t, js)
	l := find(tr.Tree.Children, KindLaunch)[0]
	if l.Summary == nil || l.Summary.Spans != 3 {
		t.Errorf("輪の span を全部出していない: %+v", l.Summary)
	}
	if strings.Count(ot, `"parentSpanId":""`) != 1 {
		t.Errorf("--otlp の親の無い span が 1 つでない")
	}
}

func TestTraceUsage(t *testing.T) {
	f := newFixture(t, false)
	for _, args := range [][]string{nil, {"--json"}, {"S-0001", "--json", "--otlp"}, {"S-0001", "S-0002"}, {"S-0001", "--bogus"}} {
		if _, _, code := run(t, f, args...); code != 2 {
			t.Errorf("%q の終了コードが 2 でない: %d", args, code)
		}
	}
	if _, errs, code := run(t, f, "S-0099"); code != 1 || !strings.Contains(errs, "課題 S-0099 が無い") {
		t.Errorf("無い課題: %d %s", code, errs)
	}
}

// 壊れた・欠けた・読めない記録: 黙って誤った区間や「記録が無い」を出さず、飛ばした所を出す。
// レビュアーの課題の端末が死んでいた間も、レビューの段の下に入る（決めたこと 12 の段を、レビュアーの側でも欠かさない）。
func TestTraceBrokenRecords(t *testing.T) {
	f := newFixture(t, true)
	k := filepath.Join(f.root, "kanban")
	b, err := os.ReadFile(filepath.Join(k, "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for i, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if strings.Contains(l, `"S-0001-r1","change":{"add"`) {
			continue // レビュアーの課題の足した行が無い（ルーチンが足した課題など）
		}
		rows = append(rows, l)
		if i == 1 {
			rows = append(rows, `{"at":"2026-11-01T01:00:30Z","by":"terminal:t1","id":"S-0001","chan`) // 書き足しの途中で切れた行
		}
	}
	put(t, filepath.Join(k, "log.jsonl"), strings.Join(rows, "\n")+"\n")
	put(t, f.state, fmt.Sprintf(`{"gone": {"S-0001": [{"agent": "t1", "at": %d}], "S-0001-r1": [{"agent": "r1", "at": %d}]}}`,
		at("02:00:00").UnixMilli(), at("01:45:00").UnixMilli()))

	js, errs, code := run(t, f, "S-0001", "--json")
	if code != 0 {
		t.Fatalf("終了コード %d: %s", code, errs)
	}
	tr := decode(t, js)
	var logSkip bool
	for _, s := range tr.Skipped {
		if s.File == "kanban/log.jsonl" && s.Line == 3 {
			logSkip = true
		}
	}
	if !logSkip {
		t.Errorf("log.jsonl の切れた行を飛ばした記録に出さない: %+v", tr.Skipped)
	}
	rv := find(tr.Tree.Children, KindReview)
	if len(rv) != 1 || rv[0].End == nil || !rv[0].End.Equal(at("01:50:00")) {
		t.Fatalf("足した行の無いレビュアーの課題のレビューが、review の行で終わらない: %+v", rv)
	}
	if g := find(rv[0].Children, KindGone); len(g) != 1 || !g[0].Start.Equal(at("01:45:00")) || g[0].End == nil || !g[0].End.Equal(at("01:50:00")) {
		t.Errorf("レビュアーの端末が死んでいた間がレビューの段の下に無い: %+v", rv[0].Children)
	}

	// 置き場のファイルはあるが読めない: 「記録が無い」と出さない
	rf := filepath.Join(f.otel, "issues", "S-0001-r1", "traces.jsonl")
	if os.Geteuid() != 0 {
		if err := os.Chmod(rf, 0); err != nil {
			t.Fatal(err)
		}
		out, _, code := run(t, f, "S-0001")
		if code != 0 || !strings.Contains(out, "ハーネスの記録が読めない: S-0001-r1") || strings.Contains(out, "ハーネスの記録が無い: S-0001-r1") {
			t.Errorf("読めないファイルを「記録が無い」と出す:\n%s", out)
		}
		_ = os.Chmod(rf, 0o644)
	}
	if err := os.Remove(rf); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rf, 0o755); err != nil {
		t.Fatal(err)
	}
	js, _, _ = run(t, f, "S-0001", "--json")
	if tr := decode(t, js); tr.Harness[1].State != "unreadable" {
		t.Errorf("ファイルの代わりにフォルダがあるのを 0 件か無いと出す: %+v", tr.Harness[1])
	}
}
