package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBin は name の名前で、台帳の根に dir を答える偽の CLI を置いた PATH を返す。
func fakeBin(t *testing.T, dir string, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, n := range names {
		body := "#!/bin/sh\necho " + dir + "\n"
		if err := os.WriteFile(filepath.Join(bin, n), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

// 別名の deck だけが PATH にあっても使わない（版違いの deck を名前だけで選ばない）。入れ方を案内して止める。
func TestOpenRefusesDeckOnly(t *testing.T) {
	t.Setenv("PATH", fakeBin(t, t.TempDir(), "deck"))
	_, err := Open()
	if err == nil || !strings.Contains(err.Error(), "kanban が無い") || !strings.Contains(err.Error(), "go install ./cmd/kanban") {
		t.Fatalf("Open() with deck only: err = %v; want kanban が無い and how to install it", err)
	}
}

// kanban があればそれを打つ（deck が並んでいても）。
func TestOpenUsesKanban(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", fakeBin(t, root, "kanban", "deck"))
	l, err := Open()
	if err != nil || l.Root != root {
		t.Fatalf("Open() = %+v, %v; want root %s", l, err, root)
	}
}

// 人が短く打った id と、移す前の log.jsonl の id は 4 桁に揃えて読む。
func TestCanonID(t *testing.T) {
	for in, want := range map[string]string{"A-5": "A-0005", "A-05": "A-0005", "A-0005": "A-0005", "A-12345": "A-12345", "Y-T-7": "Y-T-0007", "A-0": "A-0", "A-x": "A-x", "abc": "abc", "-5": "-5", "A-": "A-"} {
		if got := CanonID(in); got != want {
			t.Errorf("CanonID(%q) = %q, want %q", in, got, want)
		}
	}
}

// 移す前の行（id: A-5・by: Chief of Staff）は、移したあとのタスク A-0005 の照合に数える。
func TestTrustedReadsOldIds(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 2\nchief_of_staff: t5 # the lead\n"), 0o644)
	os.WriteFile(filepath.Join(l.Dir, "log.jsonl"), []byte(`{"at":"x","by":"terminal:t5","id":"A-5","change":{"review":"on"}}`+"\n"), 0o644)
	if l.ChiefOfStaff() != "t5" {
		t.Fatalf("ChiefOfStaff() = %q", l.ChiefOfStaff())
	}
	if !l.Trusted(l.Log(), Task{"id": "A-0005", "review": "on"}, "review") {
		t.Fatal("the row written as A-5 does not count for A-0005")
	}
}
