package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCronNext(t *testing.T) {
	utc := time.UTC
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, c := range []struct {
		expr, after, want string
		loc               *time.Location
	}{
		{"* * * * *", "2026-11-02T10:00:30Z", "2026-11-02T10:01:00Z", utc},
		{"0 9 * * *", "2026-11-02T09:00:00Z", "2026-11-03T09:00:00Z", utc},
		{"0 9 * * *", "2026-11-02T08:59:59Z", "2026-11-02T09:00:00Z", utc},
		{"*/15 * * * *", "2026-11-02T10:01:00Z", "2026-11-02T10:15:00Z", utc},
		{"0 9 * * 1-5", "2026-11-06T09:00:00Z", "2026-11-09T09:00:00Z", utc}, // Friday -> Monday
		{"0 0 * * 7", "2026-11-02T00:00:00Z", "2026-11-08T00:00:00Z", utc},   // 7 is Sunday
		{"0 0 1 * 0", "2026-11-02T00:00:00Z", "2026-11-08T00:00:00Z", utc},   // day of month OR day of week
		{"0 0 1 */2 *", "2026-11-02T00:00:00Z", "2027-01-01T00:00:00Z", utc},
		{"5,10-20/5 3 * * *", "2026-11-02T03:06:00Z", "2026-11-02T03:10:00Z", utc},
		{"0 0 29 2 *", "2026-03-01T00:00:00Z", "2028-02-29T00:00:00Z", utc},
		{"0 9 * * *", "2026-11-01T23:59:59Z", "2026-11-02T00:00:00Z", tokyo}, // 09:00 in Tokyo is 00:00 UTC
		{"0 9 * * *", "2026-11-02T00:00:00Z", "2026-11-03T00:00:00Z", tokyo},
	} {
		s, err := parseCron(c.expr)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		after := at(c.after)
		got, ok := s.next(after, c.loc)
		if !ok || got.UTC().Format(time.RFC3339) != c.want {
			t.Errorf("%s after %s in %s: got %v %v, want %s", c.expr, after.Format(time.RFC3339), c.loc, got.UTC(), ok, c.want)
		}
	}
	for _, bad := range []string{"", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *", "* * * * 8",
		"*/0 * * * *", "5-1 * * * *", "a * * * *", "0 9 * * MON", "@daily", "1,,2 * * * *", "-1 * * * *"} {
		if _, err := parseCron(bad); err == nil {
			t.Errorf("%q was read", bad)
		}
	}
	if s, _ := parseCron("0 0 30 2 *"); s != nil {
		if _, ok := s.next(at("2026-01-01T00:00:00Z"), utc); ok {
			t.Error("Feb 30 has a next time")
		}
	}
}

// routineLedger is a ledger with a clock set by DECK_NOW.
func routineLedger(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "")
	t.Setenv("DECK_NOW", "2026-11-02T08:00:00Z")
	kanban(t, t.TempDir(), "init", "--root", root, "--repo", "github.com/x/app")
	os.MkdirAll(filepath.Join(root, "kanban", "routines"), 0o755)
	return root
}

func writeRoutine(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "kanban", "routines", name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func logLines(t *testing.T, root string) []map[string]any {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(root, "kanban", "log.jsonl"))
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func routineLog(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, m := range logLines(t, root) {
		if r, ok := m["routine"]; ok {
			v := m["fired"]
			k := "fired"
			if v == nil {
				v, k = m["skipped"], "skipped"
			}
			out = append(out, r.(string)+" "+k+" "+v.(string)+" by "+m["by"].(string))
		}
	}
	return out
}

func taskCount(root string) int {
	entries, _ := os.ReadDir(issuesDir(root))
	return len(entries)
}

func readState(t *testing.T, root string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "kanban", "state", "routines.json"))
	if err != nil {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestTickFiresOnScheduleOnce(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "triage", "# a person's note\ntitle: Look at untriaged tasks\nbrief: |\n  List them.\nprofile: sonnet\nrepo: github.com/x/app\nschedule: \"0 9 * * *\"\nowner: me\n")

	// The first tick only notes when kanban first saw the routine; it doesn't fire.
	kanban(t, root, "tick")
	if n := taskCount(root); n != 0 {
		t.Fatalf("first tick added %d tasks", n)
	}
	if s := readState(t, root); s["triage"] != "2026-11-02T08:00:00Z" {
		t.Fatalf("state %v", s)
	}
	t.Setenv("DECK_NOW", "2026-11-02T08:59:00Z")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 0 {
		t.Fatalf("tick before 9:00 added %d", n)
	}
	t.Setenv("DECK_NOW", "2026-11-02T09:00:00Z")
	if out := kanban(t, root, "tick"); !strings.Contains(out, "triage") || !strings.Contains(out, "W-0001") {
		t.Fatalf("tick at 9:00: %q", out)
	}
	kanban(t, root, "tick") // the same minute again: nothing
	t.Setenv("DECK_NOW", "2026-11-02T09:05:00Z")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 1 {
		t.Fatalf("%d tasks after repeated ticks", n)
	}
	got, _ := os.ReadFile(taskFile(root, "W-0001"))
	want := "id: W-0001\ntitle: Look at untriaged tasks\nstatus: todo\nrepo: github.com/x/app\nprofile: sonnet\nroutine: triage\ncreated: \"2026-11-02T09:00:00Z\"\n"
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("task:\n%s\nwant prefix:\n%s", got, want)
	}
	if b, _ := os.ReadFile(filepath.Join(taskDir(root, "W-0001"), "brief.md")); string(b) != "List them.\n" {
		t.Fatalf("brief %q", b)
	}
	if l := routineLog(t, root); len(l) != 1 || l[0] != "triage fired W-0001 by routine:triage" {
		t.Fatalf("log %v", l)
	}
	if s := readState(t, root); s["triage"] != "2026-11-02T09:00:00Z" {
		t.Fatalf("state %v", s)
	}
	// The routine file is only read.
	if b, _ := os.ReadFile(filepath.Join(root, "kanban", "routines", "triage.yaml")); !strings.Contains(string(b), "# a person's note") {
		t.Fatalf("routine file changed: %q", b)
	}

	// Three days missed: one task, not three (after the first is done).
	kanban(t, root, "done", "W-0001")
	t.Setenv("DECK_NOW", "2026-11-05T12:00:00Z")
	kanban(t, root, "tick")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 2 {
		t.Fatalf("%d tasks after missed days, want 2", n)
	}
}

func TestOpenStopsUntilDone(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "hourly", "title: Hourly check\nschedule: \"0 * * * *\"\n")
	kanban(t, root, "tick")
	t.Setenv("DECK_NOW", "2026-11-02T09:00:00Z")
	kanban(t, root, "tick") // W-0001
	t.Setenv("DECK_NOW", "2026-11-02T10:00:00Z")
	if out := kanban(t, root, "tick"); !strings.Contains(out, "open") {
		t.Fatalf("tick with one open: %q", out)
	}
	if n := taskCount(root); n != 1 {
		t.Fatalf("%d tasks while one is open", n)
	}
	kanban(t, root, "done", "W-0001")
	t.Setenv("DECK_NOW", "2026-11-02T10:05:00Z")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 1 {
		t.Fatalf("the skipped 10:00 fired later: %d tasks", n)
	}
	t.Setenv("DECK_NOW", "2026-11-02T11:00:00Z")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 2 {
		t.Fatalf("%d tasks after done, want 2", n)
	}
	want := []string{"hourly fired W-0001 by routine:hourly", "hourly skipped open by routine:hourly", "hourly fired W-0002 by routine:hourly"}
	if l := routineLog(t, root); strings.Join(l, "|") != strings.Join(want, "|") {
		t.Fatalf("log %v", l)
	}

	// open: 2 lets two be not done.
	writeRoutine(t, root, "hourly", "title: Hourly check\nschedule: \"0 * * * *\"\nopen: 2\n")
	t.Setenv("DECK_NOW", "2026-11-02T12:00:00Z")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 3 {
		t.Fatalf("open 2: %d tasks", n)
	}
}

func TestEventFires(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "after-done", "title: Look at what was done\non: [done]\n")
	writeRoutine(t, root, "on-review", "title: Review it\non: review\n")
	kanban(t, root, "add", "Work")                  // W-0001
	kanban(t, root, "add", "Other")                 // W-0002
	kanban(t, root, "start", "W-0001")              // doing: nothing listens
	kanban(t, root, "review", "W-0001")             // on-review -> W-0003
	kanban(t, root, "done", "W-0001")               // after-done -> W-0004
	kanban(t, root, "start", "W-0003")              //
	kanban(t, root, "done", "W-0003")               // after-done open (W-0004 not done): skipped
	kanban(t, root, "done", "W-0004")               // its own task: nothing
	kanban(t, root, "set", "W-0002", "status=done") // after-done -> W-0005
	kanban(t, root, "set", "W-0002", "status=done") // not a move: nothing
	want := []string{
		"on-review fired W-0003 by routine:on-review",
		"after-done fired W-0004 by routine:after-done",
		"after-done skipped open by routine:after-done",
		"after-done fired W-0005 by routine:after-done",
	}
	if l := routineLog(t, root); strings.Join(l, "|") != strings.Join(want, "|") {
		t.Fatalf("log:\n%s\nwant:\n%s", strings.Join(l, "\n"), strings.Join(want, "\n"))
	}
	if b, _ := os.ReadFile(taskFile(root, "W-0004")); !strings.Contains(string(b), "routine: after-done") {
		t.Fatalf("W-0004: %s", b)
	}
	if s := readState(t, root); s["after-done"] == "" || s["on-review"] == "" {
		t.Fatalf("state %v", s)
	}
}

func TestDoneFreeingQueuedFires(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "on-todo", "title: Something is ready\non: [todo]\n")
	kanban(t, root, "add", "First")                       // W-0001 todo (add is not a move)
	kanban(t, root, "add", "Second", "--after", "W-0001") // W-0002 queued
	if n := taskCount(root); n != 2 {
		t.Fatalf("add fired: %d tasks", n)
	}
	kanban(t, root, "done", "W-0001") // W-0002 queued -> todo: on-todo -> W-0003
	if l := routineLog(t, root); len(l) != 1 || l[0] != "on-todo fired W-0003 by routine:on-todo" {
		t.Fatalf("log %v", l)
	}
}

func TestPausedAndRun(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "nightly", "title: Nightly\nschedule: \"0 0 * * *\"\npaused: true\n")
	writeRoutine(t, root, "manual", "title: By hand\non: [review]\n")
	kanban(t, root, "tick")
	t.Setenv("DECK_NOW", "2026-11-03T00:00:00Z")
	kanban(t, root, "tick")
	if n := taskCount(root); n != 0 {
		t.Fatalf("paused fired: %d", n)
	}
	if out := kanban(t, root, "routine", "run", "nightly"); !strings.Contains(out, "paused") {
		t.Fatalf("run paused: %q", out)
	}
	if out := kanban(t, root, "routine", "run", "manual"); !strings.Contains(out, "W-0001") {
		t.Fatalf("run: %q", out)
	}
	if out := kanban(t, root, "routine", "run", "manual"); !strings.Contains(out, "open") {
		t.Fatalf("run while open: %q", out)
	}
	kanbanFails(t, root, "", "routine", "run", "nope")
	kanbanFails(t, root, "", "routine", "run", "../x")
	kanbanFails(t, root, "", "routine", "run")
	kanbanFails(t, root, "", "routine", "what")
	want := []string{"nightly skipped paused by routine:nightly", "nightly skipped paused by routine:nightly",
		"manual fired W-0001 by routine:manual", "manual skipped open by routine:manual"}
	if l := routineLog(t, root); strings.Join(l, "|") != strings.Join(want, "|") {
		t.Fatalf("log %v", l)
	}
}

func TestRoutineList(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "triage", "title: Triage\nschedule: \"0 9 * * 1-5\"\n")
	writeRoutine(t, root, "after", "title: After\non: [done]\nopen: 2\npaused: true\n")
	os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 3\nrepos: [github.com/x/app]\ntimezone: Asia/Tokyo\n"), 0o644)
	out := kanban(t, root, "routine", "list", "--json")
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got) != 2 || got[0]["name"] != "after" || got[1]["name"] != "triage" {
		t.Fatalf("list %s", out)
	}
	// 2026-11-02 is a Monday; 09:00 in Tokyo is 00:00 UTC, so the next one after 08:00 UTC is Tuesday's.
	if got[1]["last"] != "2026-11-02T08:00:00Z" || got[1]["next"] != "2026-11-03T00:00:00Z" || got[1]["notDone"] != float64(0) {
		t.Fatalf("triage %v", got[1])
	}
	if got[0]["paused"] != true || got[0]["open"] != float64(2) || got[0]["next"] != nil {
		t.Fatalf("after %v", got[0])
	}
	if s := readState(t, root); len(s) != 2 {
		t.Fatalf("list didn't note the routines: %v", s)
	}
	if text := kanban(t, root, "routine", "list"); !strings.Contains(text, "triage") || !strings.Contains(text, "paused") {
		t.Fatalf("text %q", text)
	}
}

// paused is a YAML boolean: True pauses as true does, and false doesn't.
func TestPausedIsReadAsABoolean(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "capital", "title: Capital\non: [review]\npaused: True\n")
	writeRoutine(t, root, "off", "title: Off\non: [review]\npaused: false\n")
	if out := kanban(t, root, "routine", "run", "capital"); !strings.Contains(out, "paused") {
		t.Fatalf("paused: True fired: %q", out)
	}
	if out := kanban(t, root, "routine", "run", "off"); !strings.Contains(out, "W-0001") {
		t.Fatalf("paused: false didn't fire: %q", out)
	}
}

func TestLintRoutines(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "ok", "title: Fine\nschedule: \"*/5 * * * *\"\nrepo: github.com/x/app\n")
	writeRoutine(t, root, "notitle", "schedule: \"0 9 * * *\"\n")
	writeRoutine(t, root, "notrigger", "title: x\n")
	writeRoutine(t, root, "badcron", "title: x\nschedule: \"0 25 * * *\"\n")
	writeRoutine(t, root, "badrepo", "title: x\non: [done]\nrepo: github.com/x/other\n")
	writeRoutine(t, root, "zero", "title: x\non: [done]\nopen: 0\n")
	writeRoutine(t, root, "pausedyes", "title: x\non: [done]\npaused: yes\n")
	writeRoutine(t, root, "broken", "title: [unclosed\n")
	badName := runtime.GOOS != "windows" // Windows has no file named a:b (it is a stream of a)
	if badName {
		writeRoutine(t, root, "a:b", "title: x\non: [done]\n")
	}
	var out, errb bytes.Buffer
	code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb)
	for _, want := range []string{
		"routines/notitle.yaml: no title",
		"routines/notrigger.yaml: needs schedule or on",
		"routines/badcron.yaml: schedule",
		"routines/badrepo.yaml: repo github.com/x/other is not in config.yaml repos",
		"routines/zero.yaml: open must be a whole number of 1 or more",
		"routines/pausedyes.yaml: paused must be true or false",
		"routines/broken.yaml: ",
		"routines/a:b.yaml: ",
	} {
		if want == "routines/a:b.yaml: " && !badName {
			continue
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("lint doesn't say %q:\n%s", want, out.String())
		}
	}
	if code != 1 || strings.Contains(out.String(), "routines/ok.yaml") {
		t.Errorf("exit %d:\n%s", code, out.String())
	}
	// A broken routine doesn't stop tick for the others, but tick says so.
	kanbanFails(t, root, "", "tick")
	if s := readState(t, root); s["ok"] == "" {
		t.Fatalf("tick skipped the good routine: %v", s)
	}
	os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 3\ntimezone: Mars/Base\n"), 0o644)
	out.Reset()
	run([]string{"lint"}, root, strings.NewReader(""), &out, &errb)
	if !strings.Contains(out.String(), "config.yaml: timezone") {
		t.Errorf("lint doesn't report the time zone:\n%s", out.String())
	}
}

func TestRoutineLinksAreRefused(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "r", "title: R\nschedule: \"* * * * *\"\n")
	outDir := t.TempDir()
	stateDir := filepath.Join(root, "kanban", "state")
	symlink(t, outDir, stateDir)
	kanbanFails(t, root, "", "tick")
	kanbanFails(t, root, "", "routine", "list")
	if entries, _ := os.ReadDir(outDir); len(entries) != 0 {
		t.Fatal("wrote through the state link")
	}
	os.Remove(stateDir)
	os.MkdirAll(stateDir, 0o755)
	out := sentinelFile(t, "routines.json", "{}\n")
	symlink(t, out, filepath.Join(stateDir, "routines.json"))
	kanbanFails(t, root, "", "tick")
	assertUnchanged(t, out, "{}\n")

	// A routine file that is a link, or a routines/ that is a link, is not read.
	root2 := routineLedger(t)
	elsewhere := t.TempDir()
	os.WriteFile(filepath.Join(elsewhere, "x.yaml"), []byte("title: X\non: [done]\n"), 0o644)
	symlink(t, filepath.Join(elsewhere, "x.yaml"), filepath.Join(root2, "kanban", "routines", "x.yaml"))
	kanban(t, root2, "add", "Work")
	kanban(t, root2, "done", "W-0001")
	if n := taskCount(root2); n != 1 {
		t.Fatalf("a linked routine fired: %d", n)
	}
	os.RemoveAll(filepath.Join(root2, "kanban", "routines"))
	symlink(t, elsewhere, filepath.Join(root2, "kanban", "routines"))
	kanbanFails(t, root2, "", "routine", "list")
}

func TestRoutineCommandsRefuseOtherVersions(t *testing.T) {
	root := routineLedger(t)
	writeRoutine(t, root, "r", "title: R\nschedule: \"* * * * *\"\n")
	os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 4\n"), 0o644)
	kanbanFails(t, root, "", "tick")
	kanbanFails(t, root, "", "routine", "list")
	kanbanFails(t, root, "", "routine", "run", "r")
	if exists(filepath.Join(root, "kanban", "state")) || exists(filepath.Join(root, "kanban", ".lock")) {
		t.Fatal("wrote to a ledger of another version")
	}
}

func TestBadDeckNowIsRefused(t *testing.T) {
	root := routineLedger(t)
	t.Setenv("DECK_NOW", "tomorrow")
	kanbanFails(t, root, "", "tick")
	kanbanFails(t, root, "", "add", "x")
}
