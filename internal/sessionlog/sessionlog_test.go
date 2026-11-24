package sessionlog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assistant(req, id, model, effort string, in, out int) string {
	r := ""
	if req != "" {
		r = `"requestId":"` + req + `",`
	}
	return fmt.Sprintf(`{"type":"assistant",%s"effort":"%s","message":{"id":"%s","model":"%s","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`,
		r, effort, id, model, in, out)
}

// 本線の記録とサブエージェントの記録:
//
//	モデル: 本線の opus（req_1・req_2）と、サブエージェントの haiku。<synthetic>（API の誤りの行）は数えない
//	effort: high（本線）と low（サブエージェント）。同じ要求の続きの行は数えない
//	ターン: 本線の user の行の promptId p1・p2（p1 は道具の結果の行にも付く）。isSidechain の行とサブエージェントの promptId は数えない
func TestClaude(t *testing.T) {
	dir := t.TempDir()
	s := "aaaaaaaa-0000-4000-8000-000000000001"
	main := filepath.Join(dir, "projects", "-w", s+".jsonl")
	put(t, main, strings.Join([]string{
		`{"type":"user","promptId":"p1","message":{"content":"x"}}`,
		assistant("req_1", "m1", "claude-opus-5-5", "high", 1, 10),
		assistant("req_1", "m1", "claude-opus-5-5", "high", 1, 10),
		`{"type":"user","promptId":"p1","message":{"content":[{"type":"tool_result"}]}}`,
		`{"type":"user","promptId":"p9","isSidechain":true,"message":{"content":"y"}}`,
		`{"type":"user","promptId":"p2","message":{"content":"z"}}`,
		assistant("req_2", "m2", "claude-opus-5-5", "high", 2, 20),
		assistant("", "", "<synthetic>", "", 9, 9),
	}, "\n")+"\n")
	put(t, filepath.Join(dir, "projects", "-w", s, "subagents", "agent-1.jsonl"), strings.Join([]string{
		`{"type":"user","promptId":"p7","message":{"content":"q"}}`,
		assistant("req_3", "m3", "claude-haiku-5-5", "low", 3, 30),
	}, "\n")+"\n")
	l := Claude(dir, s, nil)
	if l == nil {
		t.Fatal("記録があるのに nil")
	}
	turns := 2
	want := &Log{Models: []string{"claude-opus-5-5", "claude-haiku-5-5"}, Efforts: []string{"high", "low"}, Turns: &turns,
		Usage: Usage{Input: 6, Output: 60}}
	if !reflect.DeepEqual(l, want) {
		t.Errorf("Claude: %+v（turns %v）、want %+v", l, deref(l.Turns), want)
	}
	if Claude(dir, "bbbbbbbb-0000-4000-8000-000000000002", nil) != nil || Claude(dir, "", nil) != nil || Claude(dir, "*", nil) != nil {
		t.Error("記録の無いセッション・空・glob の文字で nil でない")
	}
}

func TestClaudeNoPromptID(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "projects", "-w", "s1.jsonl"), `{"type":"user","message":{"content":"x"}}`+"\n"+assistant("r", "m", "claude-opus-5-5", "", 1, 1)+"\n")
	l := Claude(dir, "s1", nil)
	if l == nil || l.Turns != nil || len(l.Efforts) != 0 || l.Usage.Input != 1 {
		t.Errorf("古い記録: %+v", l)
	}
}

func TestClaudeBrokenLineIsSkipped(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "projects", "-w", "s1.jsonl"), assistant("r", "m", "x", "", 1, 1)+"\n"+`{"type":"assistant","usage":`+"\n")
	var got []string
	Claude(dir, "s1", func(src string, line int, reason string) {
		got = append(got, fmt.Sprintf("%s:%d %s", src, line, reason))
	})
	if !reflect.DeepEqual(got, []string{"claude/s1.jsonl:2 JSON の object でない"}) {
		t.Errorf("飛ばした記録: %v", got)
	}
}

// Codex: セッション x の rollout 2本（起き直し）と、セッション y の1本。トークンは rollout ごとの最後の積み上がった数を足す。
func TestCodexLogs(t *testing.T) {
	dir := t.TempDir()
	meta := func(id string) string { return `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/w"}}` }
	ctx := func(turn, model, effort string) string {
		return `{"type":"turn_context","payload":{"turn_id":"` + turn + `","model":"` + model + `","effort":"` + effort + `"}}`
	}
	count := func(in, cr, out int) string {
		return fmt.Sprintf(`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"cache_write_input_tokens":0,"output_tokens":%d}}}}`, in, cr, out)
	}
	put(t, filepath.Join(dir, "a", "rollout-1.jsonl"), strings.Join([]string{meta("x"), ctx("t1", "gpt-6", "max"), count(100, 50, 5), ctx("t2", "gpt-6", "max"), count(300, 100, 9)}, "\n")+"\n")
	put(t, filepath.Join(dir, "a", "rollout-2.jsonl"), strings.Join([]string{meta("x"), ctx("t3", "gpt-6", "high"), count(50, 0, 1)}, "\n")+"\n")
	put(t, filepath.Join(dir, "b", "rollout-3.jsonl"), strings.Join([]string{meta("y"), count(10, 0, 1)}, "\n")+"\n")
	byCwd := CodexByCwd(dir, nil)
	if len(byCwd["/w"]) != 3 {
		t.Fatalf("cwd で分けた: %v", byCwd)
	}
	ids, logs := CodexLogs(byCwd["/w"], nil)
	if !reflect.DeepEqual(ids, []string{"x", "y"}) {
		t.Fatalf("ids: %v", ids)
	}
	three := 3
	if x := logs["x"]; !reflect.DeepEqual(x, &Log{Models: []string{"gpt-6"}, Efforts: []string{"max", "high"}, Turns: &three, Usage: Usage{Input: 250, Output: 10, CacheRead: 100}}) {
		t.Errorf("x: %+v（turns %v）", x, deref(x.Turns))
	}
	if y := logs["y"]; y.Turns != nil || len(y.Models) != 0 || y.Usage != (Usage{Input: 10, Output: 1}) {
		t.Errorf("y: %+v", y)
	}
	if u := Codex(byCwd["/w"], nil); *u != (Usage{Input: 260, Output: 11, CacheRead: 100}) {
		t.Errorf("Codex の合計: %+v", *u)
	}
	if Codex(nil, nil) != nil {
		t.Error("rollout が無いのに nil でない")
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestClaudeUnreadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "projects", "-w", "s1.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	var skipped []string
	l := Claude(dir, "s1", func(src string, line int, reason string) {
		skipped = append(skipped, fmt.Sprintf("%s:%d:%s", src, line, reason))
	})
	if l == nil || !l.Unread || l.Usage != (Usage{}) {
		t.Fatalf("Claude = %+v、want Unread", l)
	}
	if !reflect.DeepEqual(skipped, []string{"claude/s1.jsonl:0:読めない"}) {
		t.Errorf("飛ばした記録: %v", skipped)
	}
	put(t, filepath.Join(dir, "projects", "-v", "s2.jsonl"), assistant("r1", "m1", "claude-opus-5-5", "high", 1, 2)+"\n")
	if l := Claude(dir, "s2", nil); l == nil || l.Unread || l.Usage.Output != 2 {
		t.Errorf("読める記録で Unread: %+v", l)
	}
}

func TestCodexLogsUnreadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rollout-1.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "rollout-2.jsonl"), `{"type":"session_meta","payload":{"id":"y","cwd":"/w"}}`+"\n")
	var skipped []string
	ids, logs := CodexLogs([]string{filepath.Join(dir, "rollout-1.jsonl"), filepath.Join(dir, "rollout-2.jsonl")}, func(src string, line int, reason string) {
		skipped = append(skipped, fmt.Sprintf("%s:%d:%s", src, line, reason))
	})
	if !reflect.DeepEqual(ids, []string{"", "y"}) {
		t.Fatalf("ids: %v", ids)
	}
	if l := logs[""]; !l.Unread || l.Usage != (Usage{}) {
		t.Errorf("読み切れない rollout: %+v、want Unread", l)
	}
	if l := logs["y"]; l.Unread {
		t.Errorf("読める rollout で Unread: %+v", l)
	}
	if !reflect.DeepEqual(skipped, []string{"codex/rollout-1.jsonl:0:読めない"}) {
		t.Errorf("飛ばした記録: %v", skipped)
	}
}

func TestClaudeSubagentsUnlistable(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "projects", "-w", "s1.jsonl"), `{"type":"user","promptId":"p1"}`+"\n")
	sub := filepath.Join(dir, "projects", "-w", "s1", "subagents")
	put(t, filepath.Join(sub, "a.jsonl"), assistant("r1", "m1", "claude-opus-5-5", "high", 1, 2)+"\n")
	if l := Claude(dir, "s1", nil); l == nil || l.Unread || l.Usage.Output != 2 {
		t.Fatalf("読める置き場: %+v", l)
	}
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sub, 0o755) })
	if _, err := os.ReadDir(sub); err == nil {
		t.Skip("権限を外しても一覧できる（root で走っている）")
	}
	var skipped []string
	l := Claude(dir, "s1", func(src string, line int, reason string) {
		skipped = append(skipped, fmt.Sprintf("%s:%d:%s", src, line, reason))
	})
	if l == nil || !l.Unread {
		t.Fatalf("Claude = %+v、want Unread", l)
	}
	if !reflect.DeepEqual(skipped, []string{"claude/s1/subagents:0:読めない"}) {
		t.Errorf("飛ばした記録: %v", skipped)
	}
}
