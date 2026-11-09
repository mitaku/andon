package issue

import (
	"os"
	"strings"

	"andon/internal/ledger"
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

// squash マージの PR の branch は、MERGED で PR の head と先が同じときだけ消せる。
// マージされていない、PR のあとに commit が足された、PR が読めないときは消さずに理由を返す
func TestMergedPRHead(t *testing.T) {
	for _, c := range []string{"git", "sh"} {
		if _, err := exec.LookPath(c); err != nil {
			t.Skipf("%s が無い", c)
		}
	}
	tmp := t.TempDir()
	repo, bin := filepath.Join(tmp, "repo"), filepath.Join(tmp, "bin")
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=a", "-c", "user.email=a@example.com"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(filepath.Join(tmp, "kanban", "issues", "T-0001", "library"), 0o755)
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	run("commit", "-q", "--allow-empty", "-m", "base")
	run("branch", "x/T-0001")
	tip := run("rev-parse", "refs/heads/x/T-0001")
	os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\n[ ! -f \"$FAKE\"/fail ] || exit 1\ncat \"$FAKE\"/view\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE", tmp)
	l := &ledger.Ledger{Root: tmp, Slug: "x", Dir: filepath.Join(tmp, "kanban")}
	c := &ctx{l: l, id: "T-0001"}
	report := func(s string) { os.WriteFile(c.reportPath(), []byte(s), 0o644) }
	view := func(state, head string) {
		os.WriteFile(filepath.Join(tmp, "view"), []byte(`{"state":"`+state+`","headRefOid":"`+head+`"}`), 0o644)
	}
	report(`{"commit": "` + tip + `", "pr": "https://github.com/example/app/pull/7"}`)

	view("MERGED", tip)
	if ok, why := c.mergedPRHead(repo, "x/T-0001"); !ok {
		t.Errorf("MERGED で head が先と同じなのに消せない: %s", why)
	}
	view("OPEN", tip)
	if ok, why := c.mergedPRHead(repo, "x/T-0001"); ok || !strings.Contains(why, "MERGED でない") {
		t.Errorf("OPEN の PR の branch を消そうとした: %v %q", ok, why)
	}
	view("MERGED", tip)
	run("commit", "-q", "--allow-empty", "-m", "after the PR")
	run("checkout", "-q", "-B", "x/T-0001")
	if ok, why := c.mergedPRHead(repo, "x/T-0001"); ok || !strings.Contains(why, "commit が足されている") {
		t.Errorf("PR のあとに commit が足された branch を消そうとした: %v %q", ok, why)
	}
	os.WriteFile(filepath.Join(tmp, "fail"), nil, 0o644)
	if ok, _ := c.mergedPRHead(repo, "x/T-0001"); ok {
		t.Error("PR が読めないのに消そうとした")
	}
}
