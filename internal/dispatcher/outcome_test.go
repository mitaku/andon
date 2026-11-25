package dispatcher

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
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

// outcomeProject は結果の記録を集める偽のプロジェクト。ハーネスの生の記録の置き場（XDG_STATE_HOME の下）は一時に向ける。
func outcomeProject(t *testing.T, rows ...string) *project {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	l := ledger.At(t.TempDir())
	writeFile(t, filepath.Join(l.Dir, "config.yaml"), "version: 3\nchief_of_staff: t5\n")
	writeFile(t, filepath.Join(l.Dir, "log.jsonl"), ledgertest.Log(t, rows...))
	p := &project{c: &Dispatcher{Log: io.Discard}, l: l, st: &State{}}
	p.rows = l.Log()
	return p
}

func statusRow(id, at, status string) string {
	return `{"at":"` + at + `","by":"terminal:t7","id":"` + id + `","change":{"status":"` + status + `"}}`
}

func TestOutcomeWorkedSameSecondAfterDone(t *testing.T) {
	const id = "S-0001"
	p := outcomeProject(t,
		statusRow(id, "2026-11-19T00:00:00Z", "doing"),
		statusRow(id, "2026-11-19T00:10:00Z", "done"),
		statusRow(id, "2026-11-19T00:10:00Z", "todo"),
	)
	o, err := p.collectOutcome(id, ledger.Issue{"id": id})
	if err != nil {
		t.Fatal(err)
	}
	if o.WorkedMinutes != 10 {
		t.Errorf("worked_minutes = %d、want 10", o.WorkedMinutes)
	}
}

func TestOutcomeWorkedPickedDoneAfterSnapshot(t *testing.T) {
	const id = "S-0001"
	p := outcomeProject(t,
		statusRow(id, "2026-11-19T00:00:00Z", "doing"),
		statusRow(id, "2026-11-19T00:10:00Z", "done"),
		statusRow(id, "2026-11-19T00:20:00Z", "todo"),
		statusRow(id, "2026-11-19T00:30:00Z", "doing"),
	)
	p.st.Outcomes = map[string]int{id: 1}
	done := statusRow(id, "2026-11-19T00:50:00Z", "done")
	f, err := os.OpenFile(filepath.Join(p.l.Dir, "log.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(ledgertest.Log(t, done))
	f.Close()
	if !p.logRow([]byte(done), ledger.DispatcherBy, 0) {
		t.Fatal("logRow が done の行で止まった")
	}
	if _, ok := p.st.Outcomes[id]; !ok {
		t.Fatal("拾った done が控えの Outcomes に無い")
	}
	o, err := p.collectOutcome(id, ledger.Issue{"id": id})
	if err != nil {
		t.Fatal(err)
	}
	if o.WorkedMinutes != 20 {
		t.Errorf("worked_minutes = %d、want 20（2回目の回）", o.WorkedMinutes)
	}
}

func TestOutcomeGatesUnlistable(t *testing.T) {
	const id = "S-0001"
	p := outcomeProject(t, statusRow(id, "2026-11-19T00:00:00Z", "doing"), statusRow(id, "2026-11-19T00:10:00Z", "done"))
	o, err := p.collectOutcome(id, ledger.Issue{"id": id})
	if err != nil || o.Gates == nil || len(o.Gates) != 0 {
		t.Fatalf("置き場の無い課題: gates %v、err %v", o, err)
	}
	writeFile(t, filepath.Join(p.l.Library(id), "gate"), "not a directory")
	if o, err := p.collectOutcome(id, ledger.Issue{"id": id}); err == nil {
		t.Errorf("library/gate が一覧できないのに err でない: gates %v", o.Gates)
	}
}

func TestOutcomeUnreadableClaudeLog(t *testing.T) {
	const id = "S-0001"
	p := outcomeProject(t, statusRow(id, "2026-11-19T00:00:00Z", "doing"), statusRow(id, "2026-11-19T00:10:00Z", "done"))
	claude := filepath.Join(t.TempDir(), "claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	writeFile(t, filepath.Join(p.l.Library(id), "launches.jsonl"), `{"session":"s1","harness":{"name":"claude"}}`+"\n")
	if err := os.MkdirAll(filepath.Join(claude, "projects", "-w", "s1.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	o, err := p.collectOutcome(id, ledger.Issue{"id": id})
	if err != nil {
		t.Fatal(err)
	}
	if got := asJSON(t, o.Sessions); got != `[{"harness":"claude","session":"s1","cost":null}]` {
		t.Errorf("sessions: %s", got)
	}
	os.Remove(filepath.Join(claude, "projects", "-w", "s1.jsonl"))
	writeFile(t, filepath.Join(claude, "projects", "-w", "s1.jsonl"), `{"type":"user","promptId":"p1"}`+"\n")
	if o, err = p.collectOutcome(id, ledger.Issue{"id": id}); err != nil {
		t.Fatal(err)
	}
	if got := asJSON(t, o.Sessions); !strings.Contains(got, `"tokens":{"input":0,"output":0,"cache_read":0,"cache_write":0}`) {
		t.Errorf("読めるログの0トークンが書かれない: %s", got)
	}
}

func TestOutcomeWorkedLogUnreadable(t *testing.T) {
	const id = "S-0001"
	p := outcomeProject(t, statusRow(id, "2026-11-19T00:00:00Z", "doing"), statusRow(id, "2026-11-19T00:10:00Z", "done"))
	if o, err := p.collectOutcome(id, ledger.Issue{"id": id}); err != nil || o.WorkedMinutes != 10 {
		t.Fatalf("読める log: %v %v", o, err)
	}
	log := filepath.Join(p.l.Dir, "log.jsonl")
	writeFile(t, log, ledgertest.Log(t, statusRow(id, "2026-11-19T00:00:00Z", "doing")))
	if o, err := p.collectOutcome(id, ledger.Issue{"id": id}); err == nil {
		t.Errorf("done の行の無い log で err でない: worked_minutes %d", o.WorkedMinutes)
	}
	os.Remove(log)
	if o, err := p.collectOutcome(id, ledger.Issue{"id": id}); err == nil {
		t.Errorf("消えた log で err でない: worked_minutes %d", o.WorkedMinutes)
	}
}
