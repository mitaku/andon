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
