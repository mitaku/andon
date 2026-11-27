package dispatcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// 積んだ課題のレビュアー（ADR-0067 決めたこと 9・受け入れ条件 10）: 1回目の依頼書は分かれ目から `git diff` で読ませ、`gh pr diff` を使わせない。
// 載せ替えのあとの再レビューは、前の回の起点をレビュアーの課題に控えた review_base に取り、範囲に前の課題の commit を出さない。
func TestStackedReviewRange(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("a.txt", "a\n")
	write("p.txt", "p\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("switch", "-q", "-c", "prev")
	write("p.txt", "p\nP\n")
	git("commit", "-q", "-am", "prev change")
	fork := git("rev-parse", "HEAD")
	git("switch", "-q", "-c", "stk")
	write("a.txt", "a\nA\n")
	git("commit", "-q", "-am", "stacked change")
	reviewed := git("rev-parse", "HEAD")
	git("switch", "-q", "main")
	git("merge", "-q", "--squash", "prev")
	git("commit", "-q", "-m", "prev (squash)")
	mainTip := git("rev-parse", "HEAD")
	git("switch", "-q", "stk")
	git("rebase", "-q", "--onto", "main", fork)
	write("a.txt", "a\nA!\n")
	git("commit", "-q", "-am", "fix")
	head := git("rev-parse", "HEAD")

	bin := t.TempDir()
	gh := "#!/bin/sh\n[ \"$*\" = 'pr view https://github.com/example/app/pull/7 --json baseRefOid -q .baseRefOid' ] || exit 9\necho " + mainTip + "\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	pr, url := "https://github.com/example/app/pull/7", filepath.Join(t.TempDir(), "nowhere")
	l := ledger.At(filepath.Join(t.TempDir(), "demo"))

	// 1回目: 積んだ課題の依頼書は分かれ目から読ませる
	rep := &contract.Report{PR: pr, ReviewRequest: &contract.ReviewRequest{Round: 1, Limit: 2, Commit: reviewed, Reviewer: "codex-review"}}
	b := reviewerBrief(l, ledger.Issue{"id": "D-2", "agent": "t7"}, rep, Reviewer{Issue: "D-2", Round: 1, Worktree: "/w/demo-D-2-r1"}, span{}, "example/app", "7", fork)
	if !strings.Contains(b, "`git diff "+fork+".."+reviewed+"`") || strings.Contains(b, "gh pr diff "+pr) || strings.Contains(b, "baseRefOid") {
		t.Errorf("積んだ課題の1回目の依頼書が分かれ目から読ませない:\n%s", b)
	}

	// 載せ替えのあとの再レビュー: 前の回の起点は review_base（分かれ目）。範囲の range-diff に前の課題の commit が出ない
	sp := reviewSpan(dir, url, pr, reviewed, head, fork, "")
	if sp != (span{Rebased: true, Old: fork, New: mainTip}) {
		t.Fatalf("載せ替えをまたいだ範囲 = %+v（want Old %s New %s）", sp, fork, mainTip)
	}
	rd := "range-diff " + sp.Old + ".." + reviewed + " " + sp.New + ".." + head
	if out := git(strings.Fields(rd)...); strings.Contains(out, "prev change") || !strings.Contains(out, "stacked change") {
		t.Errorf("載せ替えのあとの再レビューの範囲に前の課題の commit が出た・積んだ課題の commit が無い:\n%s", out)
	}
	// 否定の確かめが当たる場合があることを同じ所で見る: review_base を控えていないと、前の回の範囲に前の課題の commit が出る
	old := reviewSpan(dir, url, pr, reviewed, head, "", "")
	if out := git("range-diff", old.Old+".."+reviewed, old.New+".."+head); !strings.Contains(out, "prev change") {
		t.Errorf("review_base の無い範囲に前の課題の commit が出ない（否定の確かめが何も見ていない）:\n%s", out)
	}
	rep.ReviewRequest = &contract.ReviewRequest{Round: 2, Limit: 2, Commit: head, Previous: &reviewed, Reviewer: "codex-review"}
	b = reviewerBrief(l, ledger.Issue{"id": "D-2", "agent": "t7"}, rep, Reviewer{Issue: "D-2", Round: 2, Worktree: "/w/demo-D-2-r2"}, sp, "example/app", "7", "")
	if !strings.Contains(b, "git range-diff "+fork+".."+reviewed+" "+mainTip+".."+head) {
		t.Errorf("載せ替えのあとの再レビューの依頼書の範囲:\n%s", b)
	}

	// 積んだままの再レビュー（前の回の commit が今の祖先）: 今までどおり `git diff 前..今`、全体を読むのは分かれ目から
	next := git("rev-parse", "HEAD")
	if sp := reviewSpan(dir, url, pr, reviewed, reviewed, fork, fork); sp != (span{}) {
		t.Errorf("祖先の再レビューの範囲 = %+v", sp)
	}
	rep.ReviewRequest = &contract.ReviewRequest{Round: 2, Limit: 2, Commit: next, Previous: &reviewed, Reviewer: "codex-review"}
	b = reviewerBrief(l, ledger.Issue{"id": "D-2", "agent": "t7"}, rep, Reviewer{Issue: "D-2", Round: 2, Worktree: "/w/demo-D-2-r2"}, span{Rebased: true}, "example/app", "7", fork)
	if strings.Contains(b, "gh pr diff "+pr) || !strings.Contains(b, "`git diff "+fork+".."+next+"`") {
		t.Errorf("積んだままの再レビューの依頼書が gh pr diff を読ませた:\n%s", b)
	}
}

// 前の回のレビュアーの課題に控えた review_base を、dispatcher が書いた値だけ読む（前の回の findings.json の commit で前の回を選ぶ）。
func TestPreviousReviewBase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "demo")
	l := ledger.At(root)
	base, prev := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, rev := range []string{"D-0002-r1", "D-0002-r2"} {
		lib := l.Library(rev)
		if err := os.MkdirAll(lib, 0o755); err != nil {
			t.Fatal(err)
		}
		commit := prev
		if rev == "D-0002-r2" {
			commit = strings.Repeat("c", 40)
		}
		if err := os.WriteFile(filepath.Join(lib, "findings.json"), []byte(`{"commit":"`+commit+`","findings":[]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tasks := []ledger.Issue{
		{"id": "D-0002-r1", "parent": "D-0002", "profile": "codex-review", "review_base": base, "updated": "2026-11-27T00:00:00Z"},
		{"id": "D-0002-r2", "parent": "D-0002", "profile": "codex-review", "review_base": strings.Repeat("d", 40), "updated": "2026-11-27T01:00:00Z"},
	}
	for _, c := range []struct {
		name string
		by   string
		want string
	}{
		{"dispatcher が書いた", ledger.DispatcherBy, base},
		{"担当の端末が書いた", "terminal:t9", ""},
	} {
		rows := []ledger.Row{{ID: "D-0002-r1", By: c.by, Change: map[string]any{"review_base": base}},
			{ID: "D-0002-r2", By: ledger.DispatcherBy, Change: map[string]any{"review_base": strings.Repeat("d", 40)}}}
		p := &project{l: l, tasks: tasks, rows: rows}
		if got := p.previousReviewBase("D-0002", prev); got != c.want {
			t.Errorf("%s: %q（want %q）", c.name, got, c.want)
		}
	}
}

// 積んだ課題の詰まりの兆し（大きい差分）は、分かれ目から数える。前の課題の大きい変更を、積んだ課題の大きい差分と読まない。
func TestSignsFromStackedBase(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("update-ref", "refs/remotes/origin/main", "main")
	git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	git("switch", "-q", "-c", "prev")
	os.WriteFile(filepath.Join(dir, "big.txt"), []byte(strings.Repeat("x\n", diffLimit+1)), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "prev change")
	fork := git("rev-parse", "HEAD")
	git("switch", "-q", "-c", "stk")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\nA\n"), 0o644)
	git("commit", "-q", "-am", "stacked change")
	now := time.Now()
	if s := signs(dir, "", 60, now); !slices.Contains(s, "large-diff") {
		t.Fatalf("前の課題の変更を含む merge-base からの差分が large-diff にならない（確かめが何も見ていない）: %v", s)
	}
	if s := signs(dir, fork, 60, now); slices.Contains(s, "large-diff") {
		t.Errorf("分かれ目から数えた積んだ課題の差分を large-diff と読んだ: %v", s)
	}
}
