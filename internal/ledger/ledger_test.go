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

// kanban が PATH に無ければ、入れ方を案内して止める。
func TestOpenRefusesWithoutKanban(t *testing.T) {
	t.Setenv("PATH", fakeBin(t, t.TempDir()))
	_, err := Open()
	if err == nil || !strings.Contains(err.Error(), "kanban が無い") || !strings.Contains(err.Error(), "go install ./cmd/kanban") {
		t.Fatalf("Open() without kanban: err = %v; want kanban が無い and how to install it", err)
	}
}

// kanban があればそれを打つ。
func TestOpenUsesKanban(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", fakeBin(t, root, "kanban"))
	l, err := Open()
	if err != nil || l.Root != root {
		t.Fatalf("Open() = %+v, %v; want root %s", l, err, root)
	}
}

// 人が短く打った id と、移す前の log.jsonl の id は 4 桁に揃えて読む。
func TestCanonID(t *testing.T) {
	for in, want := range map[string]string{"A-5": "A-0005", "A-05": "A-0005", "A-0005": "A-0005", "A-12345": "A-12345", "Y-T-7": "Y-T-0007", "A-0": "A-0", "A-x": "A-x", "abc": "abc", "-5": "-5", "A-": "A-",
		"A-5-r1": "A-0005-r1", "A-0005-r01": "A-0005-r1", "A-0005-r0": "A-0005-r0", "A-0005-r": "A-0005-r", "A-0005-rx": "A-0005-rx", "A-12345-r3": "A-12345-r3"} {
		if got := CanonID(in); got != want {
			t.Errorf("CanonID(%q) = %q, want %q", in, got, want)
		}
	}
}

// レビュアーの課題は親のすぐあとに回の順で並ぶ（番号が 4 桁を超えても）。
func TestIdLess(t *testing.T) {
	ids := []string{"A-10000", "A-0006", "A-0005-r10", "A-0005-r2", "A-0005", "A-0005-r1"}
	sort.SliceStable(ids, func(i, j int) bool { return IDLess(ids[i], ids[j]) })
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

// Log は kanban が書く長さ（16 MiB まで）の行を読み、その先の行も読む。長い行で読むのを止めると、
// 後の行（Chief of Staff が書いた予約キー）が無いことになる。
func TestLogReadsLongLinesAndWhatFollows(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	long := `{"at":"x","by":"terminal:t9","id":"A-0001","change":{"report":"` + strings.Repeat("a", 8<<20) + `"}}`
	last := `{"at":"y","by":"terminal:t5","id":"A-0001","change":{"review":"on"}}`
	os.WriteFile(filepath.Join(l.Dir, "log.jsonl"), []byte(long+"\n"+last+"\n"), 0o644)
	rows := l.Log()
	if len(rows) != 2 || rows[1].By != "terminal:t5" {
		t.Fatalf("rows: %d", len(rows))
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

// estimate は small・medium・large だけを読み、Chief of Staff が最後に書いたときだけ信じる（ADR-0014 受け入れ条件 1）。
func TestEstimate(t *testing.T) {
	root := t.TempDir()
	l := At(root)
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	row := func(id, by, v string) Row { return Row{ID: id, By: by, Change: map[string]any{"estimate": v}} }
	rows := []Row{
		row("A-0001", "terminal:t5", "medium"),
		row("A-0002", "terminal:t7", "large"),                                        // 担当が書いた
		row("A-0003", "terminal:t5", "small"), row("A-0003", "terminal:t7", "large"), // 最後が担当
		row("A-0004", "terminal:t5", "中"), // 段の名は値でない
		row("A-0005", "terminal:t5", "small"),
	}
	for _, c := range []struct {
		task     Issue
		settings string
		estimate string
	}{
		{Issue{"id": "A-0001", "estimate": "medium"}, "medium", "medium"},
		{Issue{"id": "A-0002", "estimate": "large"}, "large", ""},
		{Issue{"id": "A-0003", "estimate": "large"}, "large", ""},
		{Issue{"id": "A-0004", "estimate": "中"}, "", ""},
		{Issue{"id": "A-0005", "estimate": "large"}, "large", ""}, // 履歴の値と違う
		{Issue{"id": "A-0006"}, "", ""},                           // 無い＝見込み無し（既定で埋めない）
	} {
		if got := SettingsOf(c.task).Estimate; got != c.settings {
			t.Errorf("SettingsOf(%v).Estimate = %q, want %q", c.task, got, c.settings)
		}
		if got := l.Estimate(rows, c.task); got != c.estimate {
			t.Errorf("Estimate(%v) = %q, want %q", c.task, got, c.estimate)
		}
	}
}

// config.yaml の repos は block の並びも flow の並び（repos: [x]）も読む。chief_of_staff・timezone は引用符と行末のコメントを除く。
func TestConfigReadsReposAnyForm(t *testing.T) {
	for body, want := range map[string]string{
		"version: 3\nrepos:\n  - github.com/example/a\n  - \"github.com/example/b\"\n": "github.com/example/a github.com/example/b",
		"version: 3\nrepos: [github.com/example/a, 'github.com/example/b']\n":           "github.com/example/a github.com/example/b",
		"version: 3\nrepos: []\n": "",
		"version: 3\n":            "",
	} {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "kanban"), 0o755)
		os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte(body+"chief_of_staff: \"t1\" # the CoS\ntimezone: Asia/Tokyo\n"), 0o644)
		l := At(root)
		if got := strings.Join(l.Repos(), " "); got != want {
			t.Errorf("%q: Repos = %q, want %q", body, got, want)
		}
		if l.ChiefOfStaff() != "t1" || l.Timezone() != "Asia/Tokyo" {
			t.Errorf("%q: chief_of_staff %q timezone %q", body, l.ChiefOfStaff(), l.Timezone())
		}
	}
}
