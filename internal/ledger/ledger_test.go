package ledger

import (
	"os"
	"path/filepath"
	"sort"
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
	for in, want := range map[string]string{"A-5": "A-0005", "A-05": "A-0005", "A-0005": "A-0005", "A-12345": "A-12345", "Y-T-7": "Y-T-0007", "A-0": "A-0", "A-x": "A-x", "abc": "abc", "-5": "-5", "A-": "A-",
		"A-5-r1": "A-0005-r1", "A-0005-r01": "A-0005-r1", "A-0005-r0": "A-0005-r0", "A-0005-r": "A-0005-r", "A-0005-rx": "A-0005-rx"} {
		if got := CanonID(in); got != want {
			t.Errorf("CanonID(%q) = %q, want %q", in, got, want)
		}
	}
}

// レビュアーの課題は親のすぐあとに回の順で並ぶ（番号が 4 桁を超えても）。
func TestIdLess(t *testing.T) {
	ids := []string{"A-10000", "A-0006", "A-0005-r10", "A-0005-r2", "A-0005", "A-0005-r1"}
	sort.SliceStable(ids, func(i, j int) bool { return idLess(ids[i], ids[j]) })
	if got := strings.Join(ids, " "); got != "A-0005 A-0005-r1 A-0005-r2 A-0005-r10 A-0006 A-10000" {
		t.Fatalf("got %s", got)
	}
}

// 移す前の行（id: A-5・by: Chief of Staff）は、移したあとの課題 A-0005 の照合に数える。
func TestTrustedReadsOldIds(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5 # the lead\n"), 0o644)
	os.WriteFile(filepath.Join(l.Dir, "log.jsonl"), []byte(`{"at":"x","by":"terminal:t5","id":"A-5","change":{"review":"on"}}`+"\n"), 0o644)
	if l.ChiefOfStaff() != "t5" {
		t.Fatalf("ChiefOfStaff() = %q", l.ChiefOfStaff())
	}
	if !l.Trusted(l.Log(), Issue{"id": "A-0005", "review": "on"}, "review") {
		t.Fatal("the row written as A-5 does not count for A-0005")
	}
}

// deliverable: candidates は Chief of Staff が最後に書いたときだけ信じる（ADR-0006 決めたこと 7）。
func TestCandidates(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	row := func(id, by, v string) Row { return Row{ID: id, By: by, Change: map[string]any{"deliverable": v}} }
	rows := []Row{
		row("A-0001", "terminal:t5", "candidates"),
		row("A-0002", "terminal:t7", "candidates"),                                             // 担当が書いた
		row("A-0003", "terminal:t5", "candidates"), row("A-0003", "terminal:t7", "candidates"), // 最後が担当
		row("A-0004", "terminal:t5", "pr"),
	}
	for _, c := range []struct {
		task Issue
		want bool
	}{
		{Issue{"id": "A-0001", "deliverable": "candidates"}, true},
		{Issue{"id": "A-0002", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0003", "deliverable": "candidates"}, false},
		{Issue{"id": "A-0004", "deliverable": "candidates"}, false}, // 履歴の値と違う
		{Issue{"id": "A-0004", "deliverable": "pr"}, false},
		{Issue{"id": "A-0005"}, false}, // 無い＝pr
	} {
		if got := l.Candidates(rows, c.task); got != c.want {
			t.Errorf("Candidates(%v) = %v, want %v", c.task, got, c.want)
		}
	}
}
