package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func breakdownJSON(t *testing.T, b *Breakdown) string {
	t.Helper()
	out, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// 内訳は課題の置き場だけを数える。モデルの呼び出しは全部（サブエージェントのものも）、ツールの時間はほかのツールの下でない
// ものだけ、ツールの一覧は全部の深さで名前と最初の語ごとに、失敗は下の execution の失敗も数え、CI 待ちは重なりを1回にする。
func TestBreakdownCounts(t *testing.T) {
	dir := t.TempDir()
	const tr = "c43ee97f7dd8f491fa3848a4438c5d81"
	spans := []spec{
		{tr, "i1", "", "claude_code.interaction", sessA, "01:00:00", "01:10:00", nil},
		{tr, "l1", "i1", "claude_code.llm_request", sessA, "01:00:00", "01:00:10", map[string]any{"input_tokens": 2, "output_tokens": 50, "cache_read_tokens": 1000, "cache_creation_tokens": 100}},
		{tr, "l2", "i1", "claude_code.llm_request", sessA, "01:00:10", "01:00:15", map[string]any{"input_tokens": 1, "output_tokens": 5, "success": false}},
		// 本線の Bash 2 つ（go test は失敗した execution を下に持つ。checks を待つ 2 つは時間が重なる）
		{tr, "t1", "i1", "claude_code.tool", sessA, "01:01:00", "01:02:00", map[string]any{"tool_name": "Bash", "bash_argv0": "go", "full_command": "go test ./..."}},
		{tr, "e1", "t1", "claude_code.tool.execution", sessA, "01:01:00", "01:02:00", map[string]any{"success": false}},
		{tr, "b1", "t1", "claude_code.tool.blocked_on_user", sessA, "01:01:00", "01:01:03", nil},
		{tr, "t2", "i1", "claude_code.tool", sessA, "01:03:00", "01:05:00", map[string]any{"tool_name": "Bash", "full_command": "gh pr checks 12 --watch"}},
		{tr, "t3", "i1", "claude_code.tool", sessA, "01:04:00", "01:06:00", map[string]any{"tool_name": "Bash", "full_command": "cd x && gh run watch 5"}},
		// サブエージェント: Agent の下の execution の下に、サブエージェントのモデルの呼び出しとツール
		{tr, "a1", "i1", "claude_code.tool", sessA, "01:06:00", "01:09:00", map[string]any{"tool_name": "Agent"}},
		{tr, "ae", "a1", "claude_code.tool.execution", sessA, "01:06:00", "01:09:00", nil},
		{tr, "al", "ae", "claude_code.llm_request", sessA, "01:06:00", "01:06:20", map[string]any{"input_tokens": 3, "output_tokens": 7}},
		{tr, "at", "ae", "claude_code.tool", sessA, "01:07:00", "01:08:00", map[string]any{"tool_name": "Read"}},
	}
	put(t, filepath.Join(dir, "issues", "S-0001", "traces.jsonl"), line(t, "S-0001", "worker", spans...)+"{\"resourceSpans\": [\n")
	put(t, filepath.Join(dir, "issues", "S-0001-r1", "traces.jsonl"), line(t, "S-0001-r1", "reviewer",
		spec{tr, "x1", "", "claude_code.llm_request", sessR, "02:00:00", "02:00:01", map[string]any{"input_tokens": 999}}))
	b, err := BreakdownOf(dir, "S-0001", []string{sessA, sessB, ""}, "worker", "")
	if err != nil || b == nil {
		t.Fatalf("内訳: %v %v", b, err)
	}
	// ツールの並びは時間の長い順（同じ時間は名前、最初の語の順）
	want := `{"role":"worker","coverage":{"sessions":3,"traced":1},"interactions":1,"llm_ms":35000,"tool_ms":480000,"blocked_ms":3000,` +
		`"tokens":{"input":6,"output":62,"cache_read":1000,"cache_write":100},"errors":{"llm":1,"tool":1},` +
		`"tools":[{"name":"Agent","argv0":null,"count":1,"ms":180000,"errors":0},{"name":"Bash","argv0":"cd","count":1,"ms":120000,"errors":0},` +
		`{"name":"Bash","argv0":"gh","count":1,"ms":120000,"errors":0},{"name":"Bash","argv0":"go","count":1,"ms":60000,"errors":1},` +
		`{"name":"Read","argv0":null,"count":1,"ms":60000,"errors":0}],` +
		`"waits":{"question_ms":0,"review_ms":0,"ci_ms":180000,"gone_ms":0},"knowledge":{"index_reads":0,"pages":[]}}`
	if got := breakdownJSON(t, b); got != want {
		t.Errorf("内訳:\n got  %s\n want %s", got, want)
	}
	r, err := BreakdownOf(dir, "S-0001-r1", []string{sessR}, "", "")
	if err != nil || r == nil || r.Tokens.Input != 999 || r.Role == nil || *r.Role != "reviewer" || r.Coverage != (Coverage{1, 1}) {
		t.Errorf("レビュアーの課題の内訳（役は記録の andon.role から）: %s %v", breakdownJSON(t, r), err)
	}
}

// ツールの一覧は時間の長い順に 10 個まで。親子が輪になった span でも止まる。
func TestBreakdownTopToolsAndCycle(t *testing.T) {
	dir := t.TempDir()
	const tr = "c43ee97f7dd8f491fa3848a4438c5d81"
	var spans []spec
	for i := 0; i < 12; i++ {
		spans = append(spans, spec{tr, fmt.Sprintf("t%02d", i), "", "claude_code.tool", sessA, "01:00:00", fmt.Sprintf("01:%02d:00", i+1), map[string]any{"tool_name": fmt.Sprintf("T%02d", i)}})
	}
	spans = append(spans,
		spec{tr, "c1", "c2", "claude_code.tool", sessA, "02:00:00", "02:00:01", map[string]any{"tool_name": "Loop"}},
		spec{tr, "c2", "c1", "claude_code.llm_request", sessA, "02:00:00", "02:00:01", nil})
	put(t, filepath.Join(dir, "issues", "S-0001", "traces.jsonl"), line(t, "S-0001", "worker", spans...))
	b, err := BreakdownOf(dir, "S-0001", nil, "", "")
	if err != nil || b == nil {
		t.Fatalf("内訳: %v %v", b, err)
	}
	if len(b.Tools) != 10 || b.Tools[0].Name != "T11" || b.Tools[9].Name != "T02" {
		t.Errorf("上から 10 個でない: %s", breakdownJSON(t, b))
	}
	if b.Coverage != (Coverage{0, 0}) || b.Role == nil || *b.Role != "worker" {
		t.Errorf("起動の記録の無い課題: %s", breakdownJSON(t, b))
	}
}

// 記録が 1 行も無ければ null、ログだけでも内訳はある。Codex の conversation.id でもセッションに結ぶ。読めなければ誤り。
func TestBreakdownPresence(t *testing.T) {
	dir := t.TempDir()
	if b, err := BreakdownOf(dir, "S-0001", []string{sessA}, "worker", ""); b != nil || err != nil {
		t.Errorf("置き場の無い課題: %v %v", b, err)
	}
	put(t, filepath.Join(dir, "issues", "S-0001", "traces.jsonl"), "\n\n")
	if b, err := BreakdownOf(dir, "S-0001", []string{sessA}, "worker", ""); b != nil || err != nil {
		t.Errorf("空の行だけの記録: %v %v", b, err)
	}
	put(t, filepath.Join(dir, "issues", "S-0001", "logs.jsonl"), `{"resourceLogs":[]}`+"\n")
	b, err := BreakdownOf(dir, "S-0001", []string{sessA}, "worker", "")
	if err != nil || b == nil || b.Coverage != (Coverage{1, 0}) || b.Tools == nil {
		t.Errorf("ログだけの課題: %s %v", breakdownJSON(t, b), err)
	}
	const cx = "01a20eb5-67c7-7b22-a965-66c47ba0f6d9"
	s := spec{"c43ee97f7dd8f491fa3848a4438c5d81", "s1", "", "run_turn", "", "01:00:00", "01:00:01", map[string]any{"conversation.id": cx}}
	put(t, filepath.Join(dir, "issues", "S-0002", "traces.jsonl"), line(t, "S-0002", "reviewer", s))
	if b, err := BreakdownOf(dir, "S-0002", []string{cx}, "reviewer", ""); err != nil || b == nil || b.Coverage != (Coverage{1, 1}) {
		t.Errorf("Codex のセッション: %s %v", breakdownJSON(t, b), err)
	}
	if err := os.Chmod(filepath.Join(dir, "issues", "S-0001", "logs.jsonl"), 0); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		if b, err := BreakdownOf(dir, "S-0001", []string{sessA}, "worker", ""); err == nil {
			t.Errorf("読めない記録で誤りにならない: %s", breakdownJSON(t, b))
		}
	}
}

// ADR-0061 決めたこと 16: 知識の読みは、Read の file_path と Bash の全文に出た wiki の置き場の下のパスを、ツールの記録ごとに数える。
// 目録はページと分け、wiki の外のファイル・log.md・raw/・別の置き場の wiki/ は数えない。1 つの Bash に同じページが 2 度出ても 1 回。
// サブエージェントのツールも数える。Claude Code の span が無い内訳（Codex）には知識の読みが無い。
func TestBreakdownKnowledge(t *testing.T) {
	dir := t.TempDir()
	const (
		tr   = "c43ee97f7dd8f491fa3848a4438c5d81"
		wiki = "/cell/wiki"
	)
	read := func(id, path string) spec {
		return spec{tr, id, "i1", "claude_code.tool", sessA, "01:00:00", "01:00:01", map[string]any{"tool_name": "Read", "file_path": path}}
	}
	bash := func(id, cmd string) spec {
		return spec{tr, id, "i1", "claude_code.tool", sessA, "01:00:00", "01:00:01", map[string]any{"tool_name": "Bash", "full_command": cmd}}
	}
	spans := []spec{
		{tr, "i1", "", "claude_code.interaction", sessA, "01:00:00", "01:10:00", nil},
		read("r1", "/cell/wiki/topics/tool-quirks.md"),
		read("r2", "/cell/wiki/topics/../topics/tool-quirks.md"),
		read("r3", "/cell/wiki/index.md"),
		read("r4", "/cell/wiki/log.md"),
		read("r5", "/cell/wiki/raw/A-0001/report.md"),
		read("r6", "/cell/repo/docs/design.md"),
		read("r7", "/cell/wikis/topics/x.md"),
		bash("b1", "sed -n '/^## Entities/,/^## Sources/p' /cell/wiki/index.md"),
		bash("b2", "cat /cell/wiki/entities/owner.md; grep -n x /cell/wiki/entities/owner.md /cell/wiki/overview.md"),
		bash("b3", "cat docs/wiki/topics/a.md /cell/wiki/topics/a.mdx /cell/wiki/topics/"),
		bash("b4", "cd /cell && cat wiki/sources/2026-11-21-x.md"),
		// サブエージェントの Read（Agent の下）
		{tr, "a1", "i1", "claude_code.tool", sessA, "01:02:00", "01:03:00", map[string]any{"tool_name": "Agent"}},
		{tr, "ae", "a1", "claude_code.tool.execution", sessA, "01:02:00", "01:03:00", nil},
		{tr, "ar", "ae", "claude_code.tool", sessA, "01:02:10", "01:02:11", map[string]any{"tool_name": "Read", "file_path": "/cell/wiki/entities/owner.md"}},
	}
	put(t, filepath.Join(dir, "issues", "S-0001", "traces.jsonl"), line(t, "S-0001", "worker", spans...))
	b, err := BreakdownOf(dir, "S-0001", []string{sessA}, "worker", wiki)
	if err != nil || b == nil || b.Knowledge == nil {
		t.Fatalf("内訳: %v %v", b, err)
	}
	got, err := json.Marshal(b.Knowledge)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"index_reads":2,"pages":[{"path":"entities/owner.md","reads":2},{"path":"topics/tool-quirks.md","reads":2},` +
		`{"path":"overview.md","reads":1},{"path":"sources/2026-11-21-x.md","reads":1}]}`
	if string(got) != want {
		t.Errorf("知識の読み:\n got  %s\n want %s", got, want)
	}
	// Codex の記録（claude_code. の span が無い）には知識の読みが無い
	put(t, filepath.Join(dir, "issues", "S-0002", "traces.jsonl"), line(t, "S-0002", "worker",
		spec{tr, "c1", "", "codex.tool", sessA, "01:00:00", "01:00:01", map[string]any{"tool_name": "Read", "file_path": "/cell/wiki/topics/a.md"}}))
	if c, err := BreakdownOf(dir, "S-0002", []string{sessA}, "worker", wiki); err != nil || c == nil || c.Knowledge != nil {
		t.Errorf("Claude Code の span が無い内訳に知識の読みがある: %s %v", breakdownJSON(t, c), err)
	}
}

// 依頼書の文から wiki のページを拾う。絶対パスと、語の切れ目から始まる wiki/ の相対パスを拾い、目録は分けて返す。
func TestWikiRefs(t *testing.T) {
	for _, c := range []struct {
		text  string
		pages string
		index bool
	}{
		{"`/cell/wiki/topics/a.md` と wiki/entities/owner.md（読む）", "entities/owner.md topics/a.md", false},
		{"目録 /cell/wiki/index.md だけ", "", true},
		{"docs/wiki/topics/a.md と /other/wiki/topics/b.md と wiki/log.md と wiki/raw/x.md", "", false},
		{"[x](wiki/topics/a.md)", "topics/a.md", false},
		{"「wiki/topics/b.md」を読む", "topics/b.md", false},
		{"/cell/wiki/topics/a.mdx と /cell/wiki/topics/.md", "", false},
	} {
		pages, index := WikiRefs(c.text, "/cell/wiki")
		if strings.Join(pages, " ") != c.pages || index != c.index {
			t.Errorf("%q: got %q %v, want %q %v", c.text, pages, index, c.pages, c.index)
		}
	}
}
