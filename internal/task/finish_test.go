package task

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// 本体の checkout（git worktree list の先頭）は、repo の側から見ても path の側から見ても分かる。worktree は本体でない
func TestMainCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	tmp := t.TempDir()
	repo, wt, other := filepath.Join(tmp, "repo"), filepath.Join(tmp, "wt-r1"), filepath.Join(tmp, "plant")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "-c", "user.name=a", "-c", "user.email=a@example.com", "commit", "-q", "--allow-empty", "-m", "base"},
		{"-C", repo, "worktree", "add", "-q", "--detach", wt},
		{"init", "-q", other},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	for _, c := range []struct {
		path string
		want bool
	}{
		{repo, true},                     // repo の本体
		{wt, false},                      // detached worktree
		{other, true},                    // 別の repo の本体（台帳を包むセルの checkout など）
		{filepath.Join(tmp, "x"), false}, // 無い
	} {
		if got := MainCheckout(repo, c.path); got != c.want {
			t.Errorf("MainCheckout(%s) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestReviewRound(t *testing.T) {
	p := "/h/.herdr/worktrees/app/demo-D-0001-r"
	for wt, want := range map[string]bool{
		p + "1":                                  true,
		p + "12":                                 true,
		p:                                        false,
		p + "1x":                                 false,
		"/h/.herdr/worktrees/app/demo-D-0001":    false,
		"/h/.herdr/worktrees/app/demo-D-0010-r1": false,
	} {
		if got := reviewRound(wt, p); got != want {
			t.Errorf("reviewRound(%s) = %v, want %v", wt, got, want)
		}
	}
}
