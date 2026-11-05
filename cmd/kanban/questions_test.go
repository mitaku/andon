package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"
)

func questionsOf(t *testing.T, root, id string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(taskDir(root, id), "questions.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func questionLog(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, m := range logLines(t, root) {
		c, _ := m["change"].(map[string]any)
		if n, ok := c["question"]; ok {
			out = append(out, fmt.Sprintf("%s#%v %v", m["id"], n, c["status"]))
		}
	}
	return out
}

func deckIn(t *testing.T, cwd, stdin string, argv ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(argv, cwd, strings.NewReader(stdin), &out, &errb); code != 0 {
		t.Fatalf("kanban %v: exit %d: %s", argv, code, errb.String())
	}
	return out.String()
}

// One question all the way: ask → questions → answer → received, and no step skipped.
func TestQuestionLifecycle(t *testing.T) {
	root := routineLedger(t) // the clock is DECK_NOW
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "add", "Export")
	if got := kanban(t, root, "ask", "W-1", "CSV only, or CSV and Excel?", "--kind", "business"); got != "W-1#1\n" {
		t.Fatalf("ask: %q", got)
	}
	deckFails(t, root, "", "ask", "W-1", "x", "--kind", "money")
	deckFails(t, root, "", "ask", "W-1", "  ")
	deckFails(t, root, "", "ask", "W-9", "x")
	q := questionsOf(t, root, "W-1")
	if len(q) != 1 || q[0]["n"] != 1 || q[0]["status"] != "open" || q[0]["kind"] != "business" || q[0]["asked"] != "2026-11-02T08:00:00Z" || q[0]["askedBy"] != "terminal:t3" {
		t.Fatalf("after ask: %v", q)
	}

	t.Setenv("DECK_NOW", "2026-11-02T11:00:00Z")
	if got := kanban(t, root, "questions"); !strings.HasPrefix(got, "W-1#1") || !strings.Contains(got, "open") || !strings.Contains(got, "3h") || !strings.Contains(got, "Export") || !strings.Contains(got, "CSV only") {
		t.Fatalf("questions: %q", got)
	}
	// No step skipped: received needs answered, answer needs open.
	deckFails(t, root, "", "received", "W-1#1")
	t.Setenv("HC_TERMINAL_ID", "t2")
	deckIn(t, root, "CSV only for now.\n", "answer", "W-1#1", "-")
	deckFails(t, root, "", "answer", "W-1#1", "again")
	if got := kanban(t, root, "questions", "--open"); got != "(no questions)\n" {
		t.Fatalf("questions --open after answer: %q", got)
	}
	var listed []map[string]any
	json.Unmarshal([]byte(kanban(t, root, "questions", "--json")), &listed)
	if len(listed) != 1 || listed[0]["name"] != "W-1#1" || listed[0]["status"] != "answered" || listed[0]["answer"] != "CSV only for now." || listed[0]["title"] != "Export" {
		t.Fatalf("questions --json: %v", listed)
	}
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "received", "W-1#1")
	deckFails(t, root, "", "received", "W-1#1")
	deckFails(t, root, "", "answer", "W-1#1", "late")
	deckFails(t, root, "", "answer", "W-1#2", "none")
	deckFails(t, root, "", "answer", "W-1", "no number")
	if got := kanban(t, root, "questions"); got != "(no questions)\n" {
		t.Fatalf("questions after received: %q", got)
	}
	q1 := questionsOf(t, root, "W-1")[0]
	if q1["status"] != "received" || q1["answered"] != "2026-11-02T11:00:00Z" || q1["answeredBy"] != "terminal:t2" || q1["received"] != "2026-11-02T11:00:00Z" {
		t.Fatalf("after received: %v", q1)
	}
	// The log has the number and status, never the text or the answer.
	if got := questionLog(t, root); !slices.Equal(got, []string{"W-1#1 open", "W-1#1 answered", "W-1#1 received"}) {
		t.Fatalf("log: %v", got)
	}
	if data, _ := os.ReadFile(filepath.Join(root, ".flightdeck", "log.jsonl")); bytes.Contains(data, []byte("CSV")) {
		t.Fatalf("the log has the text: %s", data)
	}
	// show counts them; --json has them all.
	if got := kanban(t, root, "show", "W-1"); !strings.Contains(got, "questions: 0 open, 0 answered, 1 received") {
		t.Fatalf("show: %q", got)
	}
	var shown struct{ Questions []map[string]any }
	json.Unmarshal([]byte(kanban(t, root, "show", "W-1", "--json")), &shown)
	if len(shown.Questions) != 1 || shown.Questions[0]["answer"] != "CSV only for now." {
		t.Fatalf("show --json: %v", shown)
	}
	kanban(t, root, "lint")
}

// n is never reused, even after a person removes questions from the file by hand.
func TestQuestionNumbersAreNotReused(t *testing.T) {
	root := ledger(t)
	kanban(t, root, "ask", "W-1", "one")
	kanban(t, root, "ask", "W-1", "two")
	file := filepath.Join(taskDir(root, "W-1"), "questions.yaml")
	os.WriteFile(file, []byte("# kept by hand\n- n: 1\n  text: one\n  status: open\n  asked: \"2026-11-02T08:00:00Z\"\n  mine: kept\n"), 0o644)
	if got := kanban(t, root, "ask", "W-1", "three"); got != "W-1#3\n" {
		t.Fatalf("after removing #2: %q", got)
	}
	os.WriteFile(file, []byte(""), 0o644)
	if got := kanban(t, root, "ask", "W-1", "four"); got != "W-1#4\n" {
		t.Fatalf("after emptying the file: %q", got)
	}
	// A task of its own counts from 1.
	kanban(t, root, "add", "Second")
	if got := kanban(t, root, "ask", "W-2", "first of W-2"); got != "W-2#1\n" {
		t.Fatalf("W-2: %q", got)
	}
	// A person's comment and unknown key survive a write.
	os.WriteFile(file, []byte("# kept by hand\n- n: 1\n  text: one\n  status: open\n  asked: \"2026-11-02T08:00:00Z\"\n  mine: kept\n"), 0o644)
	kanban(t, root, "answer", "W-1#1", "yes")
	got, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(got), "# kept by hand\n- n: 1\n") || !strings.Contains(string(got), "mine: kept\n  answer: yes\n") {
		t.Fatalf("file:\n%s", got)
	}
}

// done stops on a question not received; --force closes it and logs which.
func TestDoneStopsOnQuestions(t *testing.T) {
	root := ledger(t)
	kanban(t, root, "ask", "W-1", "one")
	kanban(t, root, "ask", "W-1", "two")
	kanban(t, root, "answer", "W-1#2", "yes")
	msg := deckFails(t, root, "", "done", "W-1")
	if !strings.Contains(msg, "W-1#1 (open)") || !strings.Contains(msg, "W-1#2 (answered)") || !strings.Contains(msg, "--force") {
		t.Fatalf("done: %q", msg)
	}
	if s := readYAML(t, taskFile(root, "W-1"))["status"]; s != "todo" {
		t.Fatalf("status after a refused done: %v", s)
	}
	kanban(t, root, "done", "W-1", "--force")
	lines := logLines(t, root)
	last := lines[len(lines)-1]["change"].(map[string]any)
	if last["status"] != "done" || fmt.Sprint(last["forced"]) != "[1 2]" {
		t.Fatalf("log: %v", last)
	}
	// With every question received, done needs no --force and logs no forced.
	kanban(t, root, "add", "Second")
	kanban(t, root, "ask", "W-2", "one")
	kanban(t, root, "answer", "W-2#1", "yes")
	kanban(t, root, "received", "W-2#1")
	kanban(t, root, "done", "W-2")
	lines = logLines(t, root)
	if c := lines[len(lines)-1]["change"].(map[string]any); c["forced"] != nil {
		t.Fatalf("forced without --force: %v", c)
	}
}

// kanban questions lists the questions of every task, oldest first.
func TestQuestionsOldestFirstAcrossTasks(t *testing.T) {
	root := routineLedger(t)
	kanban(t, root, "add", "First")
	kanban(t, root, "add", "Second")
	for _, s := range []struct{ at, id string }{
		{"2026-11-02T09:00:00Z", "W-2"}, {"2026-11-02T10:00:00Z", "W-1"}, {"2026-11-02T11:00:00Z", "W-2"}, {"2026-11-02T12:00:00Z", "W-1"},
	} {
		t.Setenv("DECK_NOW", s.at)
		kanban(t, root, "ask", s.id, "asked at "+s.at)
	}
	kanban(t, root, "answer", "W-1#1", "yes")
	var names []string
	for _, l := range strings.Split(strings.TrimSpace(kanban(t, root, "questions")), "\n") {
		names = append(names, strings.Fields(l)[0])
	}
	if want := []string{"W-2#1", "W-1#1", "W-2#2", "W-1#2"}; !slices.Equal(names, want) {
		t.Fatalf("questions: %v, want %v", names, want)
	}
	var open []map[string]any
	json.Unmarshal([]byte(kanban(t, root, "questions", "--open", "--json")), &open)
	if len(open) != 3 || open[0]["name"] != "W-2#1" || open[1]["name"] != "W-2#2" {
		t.Fatalf("--open --json: %v", open)
	}
}

// Two kanban ask at once get different numbers: the lock.
func TestConcurrentAsksGetDifferentNumbers(t *testing.T) {
	root := ledger(t)
	var wg sync.WaitGroup
	outs := make([]string, 10)
	codes := make([]int, 10)
	for i := range outs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out, errb bytes.Buffer
			codes[i] = run([]string{"ask", "W-1", fmt.Sprintf("question %d", i)}, root, strings.NewReader(""), &out, &errb)
			outs[i] = strings.TrimSpace(out.String() + errb.String())
		}()
	}
	wg.Wait()
	slices.Sort(outs)
	if len(slices.Compact(slices.Clone(outs))) != 10 || slices.Contains(codes, 1) {
		t.Fatalf("asks: %v %v", codes, outs)
	}
	if q := questionsOf(t, root, "W-1"); len(q) != 10 {
		t.Fatalf("questions.yaml has %d", len(q))
	}
}

func TestLintQuestions(t *testing.T) {
	root := ledger(t)
	os.WriteFile(filepath.Join(taskDir(root, "W-1"), "questions.yaml"), []byte(`- n: 1
  text: fine
  status: open
  asked: "2026-11-02T08:00:00Z"
- n: 1
  text: twice
  status: open
  asked: "2026-11-02T08:00:00Z"
- n: two
  text: not a number
  status: open
  asked: "2026-11-02T08:00:00Z"
- n: 3
  status: maybe
  kind: money
  asked: yesterday
- n: 4
  text: received without an answer
  status: received
  asked: "2026-11-02T08:00:00Z"
  received: "2026-11-02T09:00:00Z"
- n: 5
  text: open with an answer
  status: open
  asked: "2026-11-02T08:00:00Z"
  answer: x
- just a string
`), 0o644)
	var out, errb bytes.Buffer
	if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 {
		t.Fatalf("lint exit %d: %s", code, errb.String())
	}
	for _, p := range []string{
		"W-1#1: n is used twice", "W-1: questions.yaml entry 3: n is not a whole number", "W-1#3: no text",
		`W-1#3: status "maybe"`, `W-1#3: kind "money"`, `W-1#3: asked "yesterday" is not a UTC time`,
		"W-1#4: received but no answer", "W-1#4: received but no answered", "W-1#5: open but has answer",
		"W-1: questions.yaml entry 7 is not a mapping",
	} {
		if !strings.Contains(out.String(), p) {
			t.Errorf("lint lacks %q:\n%s", p, out.String())
		}
	}
	if strings.Contains(out.String(), "W-1#1: no text") {
		t.Errorf("lint flagged a good question:\n%s", out.String())
	}
	// kanban doesn't write a file it can't read whole.
	deckFails(t, root, "", "ask", "W-1", "more")
	os.WriteFile(filepath.Join(taskDir(root, "W-1"), "questions.yaml"), []byte("n: 1\n"), 0o644)
	deckFails(t, root, "", "ask", "W-1", "more")
	deckFails(t, root, "", "done", "W-1")
}

func TestQuestionsLinkIsRefused(t *testing.T) {
	root := ledger(t)
	out := sentinelFile(t, "questions.yaml", "- n: 1\n  text: outside\n  status: open\n")
	symlink(t, out, filepath.Join(taskDir(root, "W-1"), "questions.yaml"))
	deckFails(t, root, "", "ask", "W-1", "x")
	deckFails(t, root, "", "answer", "W-1#1", "x")
	deckFails(t, root, "", "done", "W-1")
	assertUnchanged(t, out, "- n: 1\n  text: outside\n  status: open\n")
	if got := kanban(t, root, "questions"); !strings.Contains(got, "unreadable") {
		t.Fatalf("questions: %q", got)
	}
}
