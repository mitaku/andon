package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestSamePatchIgnoresContextLines(t *testing.T) {
	table := "| 決まり | 守らせるもの |\n|---|---|\n| 一 | 無し。 |\n| 二 | 無し。 |\n| 三 | 古い語を見る |\n| 四 | 無し。 |\n"
	tableTopic := strings.Replace(table, "| 一 | 無し。 |", "| 一 | tests/run。 |", 1)
	tableMain := strings.Replace(table, "古い語を見る", "新しい語を見る", 1)
	conflictBase := "func f() {\n\ta\n\tb\n}\n"
	conflictTopic := "func f() {\n\ta\n\tA\n\tb\n}\n"
	conflictMain := "func f() {\n\ta2\n\tb\n}\n"
	rowsBase := "| 一 |\n| 二 |\n| 三 |\n| 四 |\n"
	rowsTopic := "| 一 |\n| 二。 |\n| 三。 |\n| 四 |\n"
	rowsMain := "| 一 |\n| 二 |\n| 足した |\n| 三 |\n| 四 |\n"
	twoFuncs := "func safe() {\n\tx()\n}\n\nfunc unsafe() {\n\ty()\n}\n"
	longFunc := "func f() {\n\ta\n\tb\n\tc\n\td\n\te\n}\n"
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
			map[string]string{"a.txt": conflictMain}, map[string]string{"a.txt": "func f() {\n\ta2\n\tA\n\tb\n}\n"}, true},
		{"衝突を解いて main が PR の書き換えた2行の間に足した行を残した rebase", map[string]string{"t.md": rowsBase}, map[string]string{"t.md": rowsTopic},
			map[string]string{"t.md": rowsMain}, map[string]string{"t.md": "| 一 |\n| 二。 |\n| 足した |\n| 三。 |\n| 四 |\n"}, true},
		{"衝突を解いて足した行が変わった rebase", map[string]string{"a.txt": conflictBase}, map[string]string{"a.txt": conflictTopic},
			map[string]string{"a.txt": conflictMain}, map[string]string{"a.txt": "func f() {\n\ta2\n\tA2\n\tb\n}\n"}, false},
		{"衝突を解いて消した行が変わった rebase", map[string]string{"a.txt": "a\nb\nc\n"}, map[string]string{"a.txt": "a\nB\nc\n"},
			map[string]string{"a.txt": "a\nb2\nc\n"}, map[string]string{"a.txt": "a\nB\nc\n"}, false},
		{"衝突を解いて足した行を別のファイルに置いた rebase", map[string]string{"a.txt": conflictBase, "b.txt": "b\n"}, map[string]string{"a.txt": conflictTopic},
			map[string]string{"a.txt": conflictMain}, map[string]string{"b.txt": "b\n\tA\n"}, false},
		{"衝突を解いて足した2行の順が入れ替わった rebase", map[string]string{"a.txt": conflictBase}, map[string]string{"a.txt": "func f() {\n\ta\n\tA\n\tB\n\tb\n}\n"},
			map[string]string{"a.txt": conflictMain}, map[string]string{"a.txt": "func f() {\n\ta2\n\tB\n\tA\n\tb\n}\n"}, false},
		{"足した行を同じファイルの別の関数へ移した head", map[string]string{"f.go": twoFuncs},
			map[string]string{"f.go": strings.Replace(twoFuncs, "\tx()", "\trequireAuth()\n\tx()", 1)},
			map[string]string{"f.go": strings.Replace(twoFuncs, "\ty()", "\ty2()", 1)},
			map[string]string{"f.go": strings.Replace(twoFuncs, "\ty()", "\trequireAuth()\n\ty2()", 1)}, false},
		{"main が足した行を囲む関数の宣言の行を変えた rebase", map[string]string{"f.go": longFunc},
			map[string]string{"f.go": strings.Replace(longFunc, "\td\n", "\td\n\tD\n", 1)},
			map[string]string{"f.go": strings.Replace(longFunc, "func f() {", "func f(x int) {", 1)}, nil, false},
		{"足した最後の行の改行の有無だけが違う head", map[string]string{"e.txt": "x\n", "o.txt": "o\n"}, map[string]string{"e.txt": "x\nA"},
			map[string]string{"o.txt": "o\nO\n"}, map[string]string{"e.txt": "x\nA\n", "o.txt": "o\nO\n"}, false},
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

func TestCheckReviewLatestRoundAfterEmptyReturn(t *testing.T) {
	const head = "1111111111111111111111111111111111111111"
	l := At(t.TempDir())
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	task := Issue{"id": "D-0001", "status": "doing", "profile": "claude"}
	tasks := []Issue{task,
		{"id": "D-0001-r1", "status": "done", "profile": "codex-review", "parent": "D-0001", "updated": "2026-11-20T00:00:00Z"},
		{"id": "D-0001-r2", "status": "review", "profile": "codex-review", "parent": "D-0001", "updated": "2026-11-20T00:10:00Z"}}
	rows := []Row{{ID: "D-0001-r1", By: DispatcherBy, Change: map[string]any{"add": "x"}}, {ID: "D-0001-r2", By: DispatcherBy, Change: map[string]any{"add": "x"}}}
	lib := l.Library("D-0001-r2")
	os.MkdirAll(lib, 0o755)
	if err := os.WriteFile(filepath.Join(lib, "findings.json"), []byte(`{"commit":"`+head+`","findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := l.CheckReview(rows, tasks, task, head, "major", nil, nil); r.State != "current" || r.Issue != "D-0001-r2" {
		t.Errorf("起動し直した回の結果をレビューに数えない: %+v", r)
	}
}

// stackedRepo は、前の課題の branch（prev）の上に積んだ課題の commit（S）を持つ repo。前の課題の変更は p.txt、積んだ課題の変更は
// a.txt の関数の中の行。fix が true なら、前の課題に、積んだ課題の変更を囲む関数の宣言の行を変える直しを足してから main に入れる。
// how は前の課題の入れ方（squash か merge）。積んだ課題を載せ替えた（git rebase --onto origin/main <分かれ目>）head も返す。
func stackedRepo(t *testing.T, how string, fix bool) (g *gitRepo, fork, reviewed, restacked string) {
	body := "func A() {\n" + strings.Repeat("\tx\n", 8) + "\ty\n}\n"
	g = newGitRepo(t)
	g.commit("base", map[string]string{"a.txt": body, "p.txt": "p\n"})
	g.run("checkout", "-q", "-b", "prev")
	fork = g.commit("prev", map[string]string{"p.txt": "p\nP\n"})
	g.run("checkout", "-q", "-b", "stk")
	reviewed = g.commit("stacked", map[string]string{"a.txt": strings.Replace(body, "\ty\n", "\tY\n", 1)})
	g.run("checkout", "-q", "prev")
	if fix {
		g.commit("prev fix", map[string]string{"a.txt": strings.Replace(body, "func A() {", "func A(n int) {", 1)})
	}
	g.run("checkout", "-q", "main")
	switch how {
	case "squash":
		g.run("merge", "-q", "--squash", "prev")
		g.run("commit", "-q", "-m", "prev (squash)")
	default:
		g.run("merge", "-q", "--no-ff", "-m", "prev (merge)", "prev")
	}
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "stk")
	g.run("rebase", "-q", "--onto", "origin/main", fork)
	return g, fork, reviewed, g.run("rev-parse", "HEAD")
}

// 積んでいた間のレビュー（起点は分かれ目。レビュアーの課題の review_base）は、衝突の無い載せ替えのあとの head に引き継がれる
// （ADR-0067 決めたこと 9・受け入れ条件 10）。前の課題が squash で入っても、マージ commit で入っても同じ。前の課題のあとからの
// 直しが囲む行を変えたときは引き継がない。担当の端末が書いた review_base は信じず、既定の branch との merge-base から数える。
func TestCheckReviewAcrossRestack(t *testing.T) {
	for _, c := range []struct {
		name string
		how  string
		fix  bool
		by   string
		want string
	}{
		{"squash で入った", "squash", false, DispatcherBy, "current"},
		{"マージ commit で入った", "merge", false, DispatcherBy, "current"},
		{"前の課題の直しが囲む行を変えた", "squash", true, DispatcherBy, "stale"},
		{"担当の端末が書いた review_base", "squash", false, "terminal:t9", "stale"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, fork, reviewed, head := stackedRepo(t, c.how, c.fix)
			if reviewed == head {
				t.Fatal("載せ替えで head が変わっていない")
			}
			l := At(t.TempDir())
			os.MkdirAll(l.Dir, 0o755)
			os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
			task := Issue{"id": "D-0002", "status": "doing", "profile": "claude", "worktree": g.dir}
			rev := Issue{"id": "D-0002-r1", "status": "review", "profile": "codex-review", "parent": "D-0002", "review_base": fork}
			rows := []Row{{ID: "D-0002-r1", By: DispatcherBy, Change: map[string]any{"add": "x"}},
				{ID: "D-0002-r1", By: c.by, Change: map[string]any{"review_base": fork}}}
			lib := l.Library("D-0002-r1")
			os.MkdirAll(lib, 0o755)
			os.WriteFile(filepath.Join(lib, "findings.json"), []byte(`{"commit":"`+reviewed+`","findings":[]}`), 0o644)
			if r := l.CheckReview(rows, []Issue{task, rev}, task, head, "major", nil, nil); r.State != c.want {
				t.Errorf("state %s（want %s）: %s", r.State, c.want, r.Reason)
			}
		})
	}
}

// 積んでいる間（信じられる stacked_base が在る間）、今の head の差分は分かれ目から数える。前の課題の branch が書き直されても、
// 積んだ課題の自分の commit が同じなら、前の回のレビューを引き継ぐ。stacked_base を信じなければ（担当の端末が書いた）引き継がない。
func TestCheckReviewWhileStacked(t *testing.T) {
	g := newGitRepo(t)
	g.commit("base", map[string]string{"a.txt": "a\n", "p.txt": "p\n"})
	g.run("update-ref", "refs/remotes/origin/main", "main")
	g.run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	g.run("checkout", "-q", "-b", "prev")
	fork := g.commit("prev", map[string]string{"p.txt": "p\nP\n"})
	g.run("checkout", "-q", "-b", "stk")
	reviewed := g.commit("stacked", map[string]string{"a.txt": "a\nA\n"})
	// 担当が commit を作り直した（git commit --amend。差分は同じで head だけが変わる）
	g.run("commit", "-q", "--amend", "-m", "stacked (amended)")
	head := g.run("rev-parse", "HEAD")
	for _, c := range []struct {
		name string
		by   string
		want string
	}{
		{"Chief of Staff の端末が書いた分かれ目", "terminal:t5", "current"},
		{"担当の端末が書いた分かれ目", "terminal:t9", "stale"},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := At(t.TempDir())
			os.MkdirAll(l.Dir, 0o755)
			os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
			task := Issue{"id": "D-0002", "status": "doing", "profile": "claude", "worktree": g.dir, "stacked_on": "D-0001", "stacked_base": fork}
			rev := Issue{"id": "D-0002-r1", "status": "review", "profile": "codex-review", "parent": "D-0002", "review_base": fork}
			rows := []Row{{ID: "D-0002", By: c.by, Change: map[string]any{"stacked_on": "D-0001", "stacked_base": fork}},
				{ID: "D-0002-r1", By: DispatcherBy, Change: map[string]any{"add": "x"}},
				{ID: "D-0002-r1", By: DispatcherBy, Change: map[string]any{"review_base": fork}}}
			lib := l.Library("D-0002-r1")
			os.MkdirAll(lib, 0o755)
			os.WriteFile(filepath.Join(lib, "findings.json"), []byte(`{"commit":"`+reviewed+`","findings":[]}`), 0o644)
			if r := l.CheckReview(rows, []Issue{task, rev}, task, head, "major", nil, nil); r.State != c.want {
				t.Errorf("state %s（want %s）: %s", r.State, c.want, r.Reason)
			}
		})
	}
}
