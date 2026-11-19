package dispatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResult(t *testing.T) {
	cases := []struct {
		hasPR bool
		state string
		want  string
	}{
		{false, "", "none"}, {false, "MERGED", "none"},
		{true, "MERGED", "merged"}, {true, "CLOSED", "closed"}, {true, "OPEN", "open"},
		{true, "", ""}, {true, "DRAFT", ""},
	}
	for _, c := range cases {
		if got := Result(c.hasPR, c.state); got != c.want {
			t.Errorf("Result(%v, %q) = %q、want %q", c.hasPR, c.state, got, c.want)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 起動の記録: Claude の s1（start）・s2（fresh）・s1（resume）・session の無い Claude・Codex 2回（session は起動のときに決まらない）。
// Claude は記録の順にセッションごと1つ（s1 は2回でも1つ・s2 はログが無いので cost が null・session の無い起動は1つ）。
// Codex は cwd が課題の worktree の rollout を id ごとに（ほかの cwd のものは数えない）。
func TestOutcomeSessions(t *testing.T) {
	dir := t.TempDir()
	claude, codex := filepath.Join(dir, "claude"), filepath.Join(dir, "codex")
	writeFile(t, filepath.Join(claude, "projects", "-w", "s1.jsonl"),
		`{"type":"user","promptId":"p1"}`+"\n"+`{"type":"assistant","requestId":"r1","effort":"high","message":{"model":"claude-opus-5-5","usage":{"input_tokens":1,"output_tokens":2}}}`+"\n")
	writeFile(t, filepath.Join(codex, "2026", "rollout-a.jsonl"), strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"cx-1","cwd":"/wt"}}`,
		`{"type":"turn_context","payload":{"turn_id":"t1","model":"gpt-6","effort":"max"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3}}}}`,
	}, "\n")+"\n")
	writeFile(t, filepath.Join(codex, "2026", "rollout-b.jsonl"), `{"type":"session_meta","payload":{"id":"cx-9","cwd":"/elsewhere"}}`+"\n")
	s := func(v string) *string { return &v }
	launches := []launchRow{
		{Session: s("s1")}, {Session: s("s2")}, {Session: s("s1")}, {}, {}, {},
	}
	for i := range launches {
		launches[i].Harness.Name = "claude"
	}
	launches[4].Harness.Name, launches[5].Harness.Name = "codex", "codex"
	got := asJSON(t, sessions(launches, []string{"/issues/S-1", "/wt"}, claude, codex))
	want := `[{"harness":"claude","session":"s1","cost":{"models":["claude-opus-5-5"],"efforts":["high"],"turns":1,"tokens":{"input":1,"output":2,"cache_read":0,"cache_write":0}}},` +
		`{"harness":"claude","session":"s2","cost":null},` +
		`{"harness":"claude","session":null,"cost":null},` +
		`{"harness":"codex","session":"cx-1","cost":{"models":["gpt-6"],"efforts":["max"],"turns":1,"tokens":{"input":6,"output":3,"cache_read":4,"cache_write":0}}}]`
	if got != want {
		t.Errorf("sessions:\n got  %s\n want %s", got, want)
	}
	// rollout が1つも無い Codex は session も cost も null の1件
	if got := asJSON(t, sessions(launches[4:5], []string{"/nowhere"}, claude, codex)); got != `[{"harness":"codex","session":null,"cost":null}]` {
		t.Errorf("rollout の無い Codex: %s", got)
	}
}

func TestOutcomeGates(t *testing.T) {
	dir := t.TempDir()
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	writeFile(t, filepath.Join(dir, a+".json"), `{"at":"2026-11-19T10:00:00Z","head":"`+a+`","tier":"B","decision":"approve","reasons":null,"merged":true}`)
	writeFile(t, filepath.Join(dir, b+".json"), `{"at":"2026-11-19T09:00:00Z","head":"`+b+`","tier":"B","decision":"fix","reasons":["x"],"merged":false}`)
	got, err := gates(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"head":"` + b + `","at":"2026-11-19T09:00:00Z","tier":"B","decision":"fix","reasons":["x"],"merged":false},` +
		`{"head":"` + a + `","at":"2026-11-19T10:00:00Z","tier":"B","decision":"approve","reasons":[],"merged":true}]`
	if s := asJSON(t, got); s != want {
		t.Errorf("gates:\n got  %s\n want %s", s, want)
	}
	if got, err := gates(filepath.Join(dir, "none")); err != nil || len(got) != 0 || got == nil {
		t.Errorf("置き場が無いのに空の配列でない: %v %v", got, err)
	}
	writeFile(t, filepath.Join(dir, "c.json"), `{`)
	if _, err := gates(dir); err == nil {
		t.Error("読めない記録があるのに err でない")
	}
}

func TestReadLaunches(t *testing.T) {
	dir := t.TempDir()
	if r, err := readLaunches(filepath.Join(dir, "none.jsonl")); r != nil || err != nil {
		t.Errorf("無いファイル: %v %v", r, err)
	}
	p := filepath.Join(dir, "launches.jsonl")
	writeFile(t, p, `{"session":"s1","harness":{"name":"claude"},"repo":{"path":"/wt"}}`+"\n\n"+`{"session":null,"harness":{"name":"codex"},"repo":null}`+"\n")
	r, err := readLaunches(p)
	if err != nil || len(r) != 2 || *r[0].Session != "s1" || r[0].Repo.Path != "/wt" || r[1].Session != nil || r[1].Harness.Name != "codex" {
		t.Errorf("読んだもの: %+v %v", r, err)
	}
	writeFile(t, p, `{"harness":{}}`+"\n")
	if _, err := readLaunches(p); err == nil {
		t.Error("ハーネスの無い行で err でない")
	}
}
