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

// 中身だけが違うバイナリは、差分でも違う（--binary を外すと「Binary files differ」だけになり同じに見える）。
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

// 文字列の中の空白だけが違う差分は同じと言わない（git patch-id は空白を無視して同じ id を返す）。
// レビューが見た commit の raw string の `a b` が、head で `ab` に変わった形。
func TestSamePatchWhitespaceInString(t *testing.T) {
	g := newGitRepo(t)
	g.commit("base", map[string]string{"main.go": "package main\n"})
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "-b", "topic")
	reviewed := g.commit("raw", map[string]string{"main.go": "package main\n\nconst s = `a b`\n"})
	head := g.commit("raw", map[string]string{"main.go": "package main\n\nconst s = `ab`\n"})
	if SamePatch(g.dir, reviewed, head) {
		t.Fatal("文字列の中の空白だけが違う差分を同じと言った")
	}
}

// main が同じファイルの離れた所を変えて rebase しても（行番号と blob の id はずれる）、PR の差分は同じと分かる。
func TestSamePatchSameFileMoved(t *testing.T) {
	g := newGitRepo(t)
	body := strings.Repeat("line\n", 20)
	g.commit("base", map[string]string{"a.txt": body})
	g.run("checkout", "-q", "-b", "topic")
	x := g.commit("topic", map[string]string{"a.txt": body + "tail\n"})
	g.run("checkout", "-q", "main")
	g.commit("main moves", map[string]string{"a.txt": "head\n" + body})
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "topic")
	g.run("rebase", "-q", "origin/main")
	y := g.run("rev-parse", "HEAD")
	if !SamePatch(g.dir, x, y) {
		t.Fatal("同じファイルの離れた所だけが main で変わった rebase を、同じ差分と言わない")
	}
}

// rebaseAfter は、base の上で課題の branch が topic を commit し、main が main を commit したあと、課題の branch を main に
// rebase した repo を作る。resolve が nil でなければ衝突を解いたのと同じにする（main の上に resolve の中身を積む）。
// 返すのは repo と、レビューが見た commit と、rebase のあとの head。origin/HEAD は進んだ main を指す。
func rebaseAfter(t *testing.T, base, topic, main, resolve map[string]string) (*gitRepo, string, string) {
	t.Helper()
	g := newGitRepo(t)
	g.commit("base", base)
	g.run("checkout", "-q", "-b", "topic")
	x := g.commit("topic", topic)
	g.run("checkout", "-q", "main")
	g.commit("main moves", main)
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "topic")
	if resolve == nil {
		g.run("rebase", "-q", "origin/main")
		return g, x, g.run("rev-parse", "HEAD")
	}
	g.run("reset", "-q", "--hard", "origin/main")
	return g, x, g.commit("topic", resolve)
}

// main が PR の変えた行の近く（コンテキストの行）を変えた rebase や、衝突を解いた rebase でも、PR が足した行と消した行が
// 同じなら同じ差分と分かる。足した行か消した行が変わったら同じと言わない。
func TestSamePatchIgnoresContextLines(t *testing.T) {
	// 表の行を PR が変え、main が2行下の行の語を替えた（衝突しない。コンテキストの行だけが変わる）
	table := "| 決まり | 守らせるもの |\n|---|---|\n| 一 | 無し。 |\n| 二 | 無し。 |\n| 三 | 古い語を見る |\n| 四 | 無し。 |\n"
	tableTopic := strings.Replace(table, "| 一 | 無し。 |", "| 一 | tests/run。 |", 1)
	tableMain := strings.Replace(table, "古い語を見る", "新しい語を見る", 1)
	// PR が足した行のすぐ上を main が変えた（衝突する）
	conflictBase := "a\nb\n"
	conflictTopic := "a\nA\nb\n"
	conflictMain := "a2\nb\n"
	// 最後の行に改行の無いファイルで、PR が8行目を変え、main が最後の行に改行を足した（コンテキストの「No newline」の行だけが変わる）
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, string(rune('a'+i-1)))
	}
	noEOL := strings.Join(lines, "\n")
	noEOLTopic := strings.Replace(noEOL, "\nh\n", "\nH\n", 1)
	for _, c := range []struct {
		name                       string
		base, topic, main, resolve map[string]string
		want                       bool
	}{
		{"main が近くの行を変えた rebase", map[string]string{"t.md": table}, map[string]string{"t.md": tableTopic},
			map[string]string{"t.md": tableMain}, nil, true},
		{"衝突を解いて足した行が同じ rebase", map[string]string{"a.txt": conflictBase}, map[string]string{"a.txt": conflictTopic},
			map[string]string{"a.txt": conflictMain}, map[string]string{"a.txt": "a2\nA\nb\n"}, true},
		{"衝突を解いて足した行が変わった rebase", map[string]string{"a.txt": conflictBase}, map[string]string{"a.txt": conflictTopic},
			map[string]string{"a.txt": conflictMain}, map[string]string{"a.txt": "a2\nA2\nb\n"}, false},
		{"衝突を解いて消した行が変わった rebase", map[string]string{"a.txt": "a\nb\nc\n"}, map[string]string{"a.txt": "a\nB\nc\n"},
			map[string]string{"a.txt": "a\nb2\nc\n"}, map[string]string{"a.txt": "a\nB\nc\n"}, false},
		{"main が最後の行に改行を足した rebase", map[string]string{"n.txt": noEOL}, map[string]string{"n.txt": noEOLTopic},
			map[string]string{"n.txt": noEOL + "\n"}, nil, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, x, y := rebaseAfter(t, c.base, c.topic, c.main, c.resolve)
			if x == y {
				t.Fatal("rebase で head が変わっていない")
			}
			if got := SamePatch(g.dir, x, y); got != c.want {
				t.Fatalf("SamePatch = %v, want %v", got, c.want)
			}
		})
	}
}
