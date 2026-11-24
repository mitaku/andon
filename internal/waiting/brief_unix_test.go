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
