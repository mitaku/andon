package triage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryFrom(t *testing.T) {
	cases := map[string]string{
		"feedback_a.md": "feedback_a",
		"note c.md":     "note-c",
		"_x.md":         "m_x",
		".md":           "m",
		"メモ.md":         "m------",
	}
	for in, want := range cases {
		if got := memoryFrom(in); got != want || !idRe.MatchString(got) {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestMemoryItemsMissingFolder(t *testing.T) {
	l := &ledger{slug: "demo", memory: filepath.Join(t.TempDir(), "memory"), now: time.Now()}
	got, err := l.memoryItems()
	if err != nil || len(got) != 0 {
		t.Errorf("got %v %v", got, err)
	}
}

func TestMemoryItemsSkipsIndexAndOthers(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"a.md": "A\n", "b.md": "B\n", "MEMORY.md": "- [A](a.md)\n", "c.txt": "C"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	l := &ledger{slug: "demo", memory: dir, now: time.Now()}
	got, err := l.memoryItems()
	if err != nil || len(got) != 2 || got[0].From != "a" || got[0].Text != "A\n" || got[0].Key != itemKey("demo", "a", "memory", "A\n") || got[1].From != "b" {
		t.Errorf("got %+v %v", got, err)
	}
}

func TestRemoveMemory(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	os.WriteFile(a, []byte("A"), 0o644)
	os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# 目録\n- [A](a.md) — a\n- [AB](ab.md) — ab\n"), 0o644)
	if err := removeMemory(a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Error("file remains")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	if string(b) != "# 目録\n- [AB](ab.md) — ab\n" {
		t.Errorf("index %q", b)
	}
	// 目録もファイルも無くても落ちない。
	if err := removeMemory(filepath.Join(t.TempDir(), "gone.md")); err != nil {
		t.Error(err)
	}
}
