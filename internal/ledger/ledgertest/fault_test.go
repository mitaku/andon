package ledgertest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFaultStopsAtEachPointOfAWrite(t *testing.T) {
	f := NewFault(t)
	for _, c := range []struct {
		at         int
		issue, row bool
	}{
		{1, false, false},
		{2, true, false},
		{3, true, true},
		{0, true, true},
	} {
		root := filepath.Join(t.TempDir(), "sample-site")
		f.Kanban(root, "", nil, "init", "--root", root, "--prefix", "T")
		f.Arm(c.at)
		cmd := exec.Command("kanban", "add", "ボタンを足す")
		cmd.Env = append(os.Environ(), "KANBAN_ROOT="+root)
		err := cmd.Run()
		if (err == nil) != (c.at == 0) {
			t.Errorf("%d 番目で止めた: 誤り %v", c.at, err)
		}
		_, statErr := os.Stat(filepath.Join(root, "kanban", "issues", "T-0001", "issue.yaml"))
		log, _ := os.ReadFile(filepath.Join(root, "kanban", "log.jsonl"))
		if (statErr == nil) != c.issue || strings.Contains(string(log), `"add":"ボタンを足す"`) != c.row {
			t.Errorf("%d 番目で止めた: issue.yaml %v（want %v）、log.jsonl %q（行 want %v）", c.at, statErr == nil, c.issue, log, c.row)
		}
		want := 3
		if c.at > 0 {
			want = c.at
		}
		if got := f.Points(); got != want {
			t.Errorf("%d 番目で止めた: 数えた所 %d（want %d）", c.at, got, want)
		}
		f.Arm(0)
		show := exec.Command("kanban", "list", "--all", "--json")
		show.Env = append(os.Environ(), "KANBAN_ROOT="+root)
		if err := show.Run(); err != nil || f.Points() != 0 {
			t.Errorf("読むだけの command を数えた: %d（%v）", f.Points(), err)
		}
	}
}

func TestFaultRefusesAWriteWithoutKanbanRoot(t *testing.T) {
	NewFault(t)
	dir := t.TempDir()
	cmd := exec.Command("kanban", "init", "--root", filepath.Join(dir, "sample-site"))
	cmd.Env = append(os.Environ(), "KANBAN_ROOT=")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "KANBAN_ROOT が無い") {
		t.Errorf("KANBAN_ROOT の無い書く command を打った: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "sample-site", "kanban")); err == nil {
		t.Error("断ったのに kanban を作った")
	}
}
