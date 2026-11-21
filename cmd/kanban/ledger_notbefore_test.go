package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readyIDs(t *testing.T, root string) string {
	t.Helper()
	return kanban(t, root, "ready", "--json")
}

// ADR-0011 acceptance 6: an issue with not_before is out of `kanban ready` before that day and in from that day;
// with after as well, both must be met.
func TestNotBeforeHidesFromReady(t *testing.T) {
	root := routineLedger(t) // DECK_NOW 2026-11-02T08:00:00Z
	kanban(t, root, "add", "later")
	kanban(t, root, "set", "W-0001", "not_before=2026-11-05")

	if out := readyIDs(t, root); strings.Contains(out, "W-0001") {
		t.Fatalf("not_before ahead, yet ready: %s", out)
	}
	t.Setenv("DECK_NOW", "2026-11-04T23:59:00Z")
	if out := readyIDs(t, root); strings.Contains(out, "W-0001") {
		t.Fatalf("the day before, yet ready: %s", out)
	}
	t.Setenv("DECK_NOW", "2026-11-05T00:00:00Z")
	if out := readyIDs(t, root); !strings.Contains(out, "W-0001") {
		t.Fatalf("on the day, not ready: %s", out)
	}

	// queued with after: both conditions
	t.Setenv("DECK_NOW", "2026-11-02T08:00:00Z")
	kanban(t, root, "add", "first")
	kanban(t, root, "add", "watch", "--after", "W-0002")
	kanban(t, root, "set", "W-0003", "not_before=2026-11-05")
	kanban(t, root, "done", "W-0002") // frees W-0003 to todo, but its day hasn't come
	if out := readyIDs(t, root); strings.Contains(out, "W-0003") {
		t.Fatalf("after met, day not: %s", out)
	}
	t.Setenv("DECK_NOW", "2026-11-06T00:00:00Z")
	if out := readyIDs(t, root); !strings.Contains(out, "W-0003") {
		t.Fatalf("both met, not ready: %s", out)
	}

	// queued, day come, after not met
	kanban(t, root, "add", "blocker")
	kanban(t, root, "add", "w2", "--after", "W-0004")
	kanban(t, root, "set", "W-0005", "not_before=2026-11-05")
	if out := readyIDs(t, root); strings.Contains(out, "W-0005") {
		t.Fatalf("day come, after not met: %s", out)
	}
}

func TestNotBeforeReadsConfigTimezone(t *testing.T) {
	root := routineLedger(t)
	os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 3\nrepos: [github.com/x/app]\ntimezone: Asia/Tokyo\n"), 0o644)
	kanban(t, root, "add", "later")
	kanban(t, root, "set", "W-0001", "not_before=2026-11-03")
	t.Setenv("DECK_NOW", "2026-11-02T14:59:00Z") // 23:59 on the 2nd in Tokyo
	if out := readyIDs(t, root); strings.Contains(out, "W-0001") {
		t.Fatalf("still the 2nd in the timezone, yet ready: %s", out)
	}
	t.Setenv("DECK_NOW", "2026-11-02T15:00:00Z") // 00:00 on the 3rd in Tokyo
	if out := readyIDs(t, root); !strings.Contains(out, "W-0001") {
		t.Fatalf("the 3rd in the timezone, not ready: %s", out)
	}
}

func TestLintReportsBadNotBefore(t *testing.T) {
	root := routineLedger(t)
	kanban(t, root, "add", "x")
	kanban(t, root, "set", "W-0001", "not_before=soon")
	var out, errb bytes.Buffer
	if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(out.String(), "W-0001: not_before") {
		t.Fatalf("lint: exit %d, %q %q", code, out.String(), errb.String())
	}
	if !strings.Contains(readyIDs(t, root), "W-0001") {
		t.Fatal("a bad not_before hides the issue")
	}
}

// kanban done と kanban set の after が待ちを外した課題でも、not_before が先なら「now ready」に出さない
// （todo には動かす。kanban ready に出ないものを ready と言わない）。日が来ていれば出す。
func TestNotBeforeHidesFromNowReady(t *testing.T) {
	root := routineLedger(t) // DECK_NOW 2026-11-02T08:00:00Z
	kanban(t, root, "add", "first")
	kanban(t, root, "add", "later", "--after", "W-0001")
	kanban(t, root, "set", "W-0002", "not_before=2026-11-05")
	kanban(t, root, "add", "now", "--after", "W-0001")
	out := kanban(t, root, "done", "W-0001")
	if strings.Contains(out, "W-0002") || !strings.Contains(out, "now ready: W-0003") {
		t.Fatalf("done: %q, want now ready: W-0003 only", out)
	}

	kanban(t, root, "add", "blocker")
	kanban(t, root, "add", "w5", "--after", "W-0004")
	kanban(t, root, "set", "W-0005", "not_before=2026-11-05")
	if out := kanban(t, root, "set", "W-0005", "after="); strings.Contains(out, "now ready") {
		t.Fatalf("set after=: %q, want no now ready", out)
	}
	if s := readYAML(t, taskFile(root, "W-0005"))["status"]; s != "todo" {
		t.Fatalf("set after=: status %v, want todo (the move stays)", s)
	}
	kanban(t, root, "add", "w6", "--after", "W-0004")
	kanban(t, root, "set", "W-0006", "not_before=2026-11-02")
	if out := kanban(t, root, "set", "W-0006", "after="); !strings.Contains(out, "now ready: W-0006") {
		t.Fatalf("set after= on the day: %q, want now ready: W-0006", out)
	}

	// --json の ready も同じ
	kanban(t, root, "add", "blocker2")
	kanban(t, root, "add", "w8", "--after", "W-0007")
	kanban(t, root, "set", "W-0008", "not_before=2026-11-05")
	if js := kanban(t, root, "done", "W-0007", "--json"); !strings.Contains(js, `"ready": []`) {
		t.Fatalf("done --json: %s, want ready []", js)
	}
}
