package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo は一時の repo で git を打つ補助。HOME と全体の設定は読まない。
type gitRepo struct {
	t   *testing.T
	dir string
}

func newGitRepo(t *testing.T) *gitRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	g := &gitRepo{t: t, dir: t.TempDir()}
	g.run("init", "-q", "-b", "main")
	return g
}

func (g *gitRepo) run(args ...string) string {
	g.t.Helper()
	c := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := c.CombinedOutput()
	if err != nil {
		g.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit は files（名前 → 中身）を書いて commit し、その sha を返す。
func (g *gitRepo) commit(msg string, files map[string]string) string {
	g.t.Helper()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(g.dir, name), []byte(body), 0o644); err != nil {
			g.t.Fatal(err)
		}
	}
	g.run("add", "-A")
	g.run("commit", "-q", "-m", msg)
	return g.run("rev-parse", "HEAD")
}

// rebased は、課題の branch（commit x）を、main が進んだあと（衝突しない変更）に rebase した repo を作る。
// 返すのは repo と、レビューが見た x と、rebase のあとの head。origin/HEAD は進んだ main を指す（担当が fetch した後）。
func rebased(t *testing.T) (*gitRepo, string, string) {
	g := newGitRepo(t)
	g.commit("base", map[string]string{"a.txt": "a\n", "c.txt": "c\n"})
	g.run("checkout", "-q", "-b", "topic")
	x := g.commit("topic", map[string]string{"a.txt": "a\nA\n"})
	g.run("checkout", "-q", "main")
	g.commit("main moves", map[string]string{"c.txt": "c\nC\n"})
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "topic")
	g.run("rebase", "-q", "origin/main")
	return g, x, g.run("rev-parse", "HEAD")
}

// 衝突の無い rebase で head だけが変わったとき、前のレビューが見た commit と PR の差分が同じと分かる。
// 差分の変わった rebase・origin/HEAD が無い・commit が無い・worktree が無いときは同じと言わない。
func TestSamePatch(t *testing.T) {
	g, x, y := rebased(t)
	if x == y {
		t.Fatal("rebase で head が変わっていない")
	}
	if !SamePatch(g.dir, x, y) {
		t.Fatal("衝突の無い rebase の前後で差分が同じと言わない")
	}
	if SamePatch("", x, y) {
		t.Fatal("worktree が無いのに同じと言った")
	}
	if SamePatch(g.dir, "0000000000000000000000000000000000000001", y) {
		t.Fatal("無い commit と同じと言った")
	}
	z := g.commit("fix", map[string]string{"a.txt": "a\nA2\n"})
	if SamePatch(g.dir, x, z) {
		t.Fatal("差分が変わったのに同じと言った")
	}
	g.run("symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
	if SamePatch(g.dir, x, y) {
		t.Fatal("origin/HEAD が無いのに同じと言った")
	}
}

// 中身だけが違うバイナリは、差分の patch-id でも違う（--binary を外すと「Binary files differ」だけになり同じに見える）。
func TestSamePatchBinary(t *testing.T) {
	g := newGitRepo(t)
	g.commit("base", map[string]string{"a.txt": "a\n"})
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "-b", "one")
	one := g.commit("bin", map[string]string{"b.bin": "\x00\x01one"})
	g.run("checkout", "-q", "main")
	g.run("checkout", "-q", "-b", "two")
	two := g.commit("bin", map[string]string{"b.bin": "\x00\x01two"})
	if SamePatch(g.dir, one, two) {
		t.Fatal("中身の違うバイナリを同じと言った")
	}
}
