//go:build unix

package waiting

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// 依頼書が普通のファイルでない（FIFO・装置を指す symlink）と、読むと止まるか際限なく読む。andon waiting と dispatcher を止めないよう、
// 普通のファイルでない依頼書は読まずに行だけを出す。大きすぎる依頼書は頭だけを読む。
func TestTaskBriefNotRegular(t *testing.T) {
	f := newFake(t, 4)
	f.issue("S-0001", "todo", "who: person", "only_owner: FIFO の依頼書")
	if err := os.MkdirAll(filepath.Join(f.root, "kanban/issues/S-0001"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(f.root, "kanban/issues/S-0001/brief.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.issue("S-0002", "todo", "who: person", "only_owner: 装置を指す依頼書")
	if err := os.Symlink("/dev/zero", filepath.Join(f.root, "kanban/issues/S-0002/brief.md")); err != nil {
		t.Fatal(err)
	}
	f.issue("S-0003", "todo", "who: person", "only_owner: 大きすぎる依頼書")
	f.write("kanban/issues/S-0003/brief.md", "https://example.com/head "+strings.Repeat("あ", maxBriefBytes))
	done := make(chan Result, 1)
	go func() { done <- f.read() }()
	select {
	case r := <-done:
		want(t, r, "S-0001=task", "S-0002=task", "S-0003=task")
		for _, it := range r.Items {
			switch it.ID {
			case "S-0001", "S-0002":
				if it.Steps != "" || it.Links != nil {
					t.Errorf("%s = %+v, want 普通のファイルでない依頼書は読まない", it.ID, it)
				}
			case "S-0003":
				if !strings.HasSuffix(it.Steps, stepsCut) || len(it.Links) != 1 {
					t.Errorf("大きすぎる依頼書 = steps %d 文字・links %q, want 頭だけを読んで切る", len([]rune(it.Steps)), it.Links)
				}
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("普通のファイルでない依頼書を読んで止まった")
	}
}

// 普通のファイルと見たあとに FIFO へ差し替えられても止まらない: 開く口そのものが、FIFO を書き手を待たずに開いて断る
// （開く前に見る形では、見てから開くまでの差し替えで開くところが止まる）。
func TestOpenRegularFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "brief.md")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain.md")
	if err := os.WriteFile(plain, []byte("手順\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		fh, err := openRegular(fifo)
		if err == nil {
			fh.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO を普通のファイルとして開いた")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("FIFO を開くところで止まった")
	}
	// 同じ口が普通のファイルは開いて読める
	b, err := readBrief(plain)
	if err != nil || string(b) != "手順\n" {
		t.Fatalf("普通のファイルを読めない: %q %v", b, err)
	}
}
