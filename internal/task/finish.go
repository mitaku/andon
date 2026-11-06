package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// finish は端末と worktree を片付け、マージ済みなら branch を消し、台帳を done にする（仮設の yard task finish を移したもの）。
// 台帳の worktree と branch が、andon task start がこのタスクに作るもの（~/.herdr/worktrees/<repo 名>/<slug>-<id> と
// <slug>/<id>[-…]）と合うときだけ片付ける。合わなければ端末にも触らずに止まる（台帳の書き違いで別のタスクの作業物を消さない）。
// レビュアーの子タスク（parent がこのタスクで profile が *-review）と、dispatcher が作った detached worktree
// （~/.herdr/worktrees/<repo 名>/<slug>-<id>-r<回>）も片付ける。
// マージしたかは確かめない（確かめてから打つのは打つ側。dispatcher は MERGED を見てから打つ）。
func (c *ctx) finish(stdout io.Writer) error {
	for _, cmd := range []string{"git", "hc"} {
		if _, err := exec.LookPath(cmd); err != nil {
			return fail("%s が無い", cmd)
		}
	}
	slug, repo := c.l.Slug, c.task.Str("repo")
	wt, branch := c.task.Path("worktree"), c.task.Str("branch")
	repoDir := ""
	if repo != "" {
		root := os.Getenv("GHQ_ROOT")
		if root == "" {
			out, err := exec.Command("ghq", "root").Output()
			if err != nil {
				return fail("GHQ_ROOT が無い")
			}
			root = strings.TrimSpace(string(out))
		}
		repoDir = filepath.Join(root, repo)
	}

	// repo の無いタスク（レビュアーなど）は、担当の deck start が cwd（tasks/<id>/）を包む checkout を書く。
	// start が作ったものではないので、git には触らず台帳から消すだけ（その checkout と一致するときだけ）
	if repo == "" && wt != "" {
		top, err := git(filepath.Join(c.l.Dir, "tasks", c.id), "rev-parse", "--show-toplevel")
		if err == nil && top == wt {
			if _, err := c.l.Run("", "set", c.id, "worktree=", "branch="); err != nil {
				return err
			}
			wt, branch = "", ""
		}
	}
	own, detached := false, false
	if wt != "" {
		if repoDir == "" {
			return fail("%s に worktree があるが repo が無い", c.id)
		}
		// 本体の checkout（git worktree list の先頭）は、置き場の検査より先に、明示的に消さない
		if MainCheckout(repoDir, wt) {
			return fail("%s の worktree が repo の本体の checkout: %s（消さない。台帳の worktree・branch を確かめる）", c.id, wt)
		}
		dir := filepath.Join(os.Getenv("HOME"), ".herdr", "worktrees", filepath.Base(repoDir))
		want := filepath.Join(dir, slug+"-"+c.id)
		// レビュアーの子タスクは、dispatcher が kanban start をその detached worktree（<slug>-<parent>-r<回>）で打つ。branch は無い
		if p := c.task.Str("parent"); p != "" && strings.HasSuffix(c.task.Str("profile"), "-review") && branch == "" && reviewRound(wt, filepath.Join(dir, slug+"-"+p+"-r")) {
			want, detached = wt, true
		}
		if wt != want {
			return fail("%s の worktree が %s でない: %s", c.id, want, wt)
		}
		if fi, err := os.Lstat(wt); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			return fail("%s の worktree が symlink: %s", c.id, wt)
		}
	}
	if branch != "" {
		if repoDir == "" {
			return fail("%s に branch があるが repo が無い", c.id)
		}
		if branch != slug+"/"+c.id && !strings.HasPrefix(branch, slug+"/"+c.id+"-") {
			return fail("%s の branch が %s/%s- で始まらない: %s", c.id, slug, c.id, branch)
		}
	}
	if wt != "" {
		if _, err := os.Stat(wt); err == nil && detached {
			if !Pair(repoDir, wt, "detached") {
				return fail("%s は %s の detached worktree でない", wt, repoDir)
			}
			own = true // レビュアーは読むだけ。検査の残り物があっても消す（親の片付けと同じく --force）
		} else if err == nil {
			if branch == "" {
				return fail("%s に worktree があるが branch が無い", c.id)
			}
			if !Pair(repoDir, wt, "branch refs/heads/"+branch) {
				return fail("%s は %s の worktree（branch %s）でない", wt, repoDir, branch)
			}
			own = true
			dirty, err := git(wt, "status", "--porcelain")
			if err != nil {
				return fail("worktree を読めない: %s", wt)
			}
			if dirty != "" {
				return fail("worktree に commit していない変更がある: %s", wt)
			}
		}
	}

	// レビュアーの子タスクと detached worktree
	tasks, err := c.l.List()
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.Str("parent") != c.id || !strings.HasSuffix(t.Str("profile"), "-review") || t.Str("status") == "done" {
			continue
		}
		child := &ctx{l: c.l, id: t.Str("id"), task: t}
		if err := child.finish(stdout); err != nil {
			return fail("レビュアーのタスク %s を片付けられない: %v", child.id, err)
		}
	}
	if repoDir != "" {
		olds, _ := filepath.Glob(filepath.Join(os.Getenv("HOME"), ".herdr", "worktrees", filepath.Base(repoDir), slug+"-"+c.id+"-r*"))
		for _, p := range olds {
			if Pair(repoDir, p, "detached") && !MainCheckout(repoDir, p) {
				if _, err := git(repoDir, "worktree", "remove", "--force", p); err != nil {
					return fail("レビュアーの detached worktree を消せない: %s", p)
				}
			}
		}
	}

	// 端末: このタスクのラベルのものだけ
	if err := removeTerminals(slug, c.id); err != nil {
		return err
	}

	kept := ""
	if repoDir != "" {
		if own {
			args := []string{"worktree", "remove", wt}
			if detached {
				args = []string{"worktree", "remove", "--force", wt}
			}
			if _, err := git(repoDir, args...); err != nil {
				return fail("worktree を消せない: %s", wt)
			}
		}
		git(repoDir, "worktree", "prune")
		if branch != "" {
			if _, err := git(repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
				// 消すのは、worktree と組で確かめた branch のうち origin の既定の branch に入っているものだけ
				switch def := defaultBranch(repoDir); {
				case !own:
					kept = fmt.Sprintf(" branch %s は worktree と組で確かめられないので残した", branch)
				case def == "":
					kept = fmt.Sprintf(" branch %s は origin の既定の branch が分からないので残した", branch)
				default:
					if _, err := git(repoDir, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/remotes/origin/"+def); err == nil {
						git(repoDir, "branch", "-D", branch)
					} else {
						kept = fmt.Sprintf(" branch %s は origin/%s に入っていないので残した", branch, def)
					}
				}
			}
		}
	}
	if _, err := c.l.Run("", "done", c.id); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "andon task: %s を片付けた（done）。%s\n", c.id, kept)
	return nil
}

// MainCheckout は path が repo の本体の checkout（git worktree list の先頭）か。repoDir と path の両方から見る
// （path が repoDir とは別の repo の本体のこともある）。シンボリックリンクは解いて比べる。
func MainCheckout(repoDir, path string) bool {
	real := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	want := real(path)
	for _, d := range []string{repoDir, path} {
		out, err := git(d, "worktree", "list", "--porcelain")
		if err != nil {
			continue
		}
		first, _, _ := strings.Cut(out, "\n")
		if m, ok := strings.CutPrefix(first, "worktree "); ok && real(m) == want {
			return true
		}
	}
	return false
}

// reviewRound は wt が prefix（…/<slug>-<parent>-r）に回の数を足したものか。
func reviewRound(wt, prefix string) bool {
	n, ok := strings.CutPrefix(wt, prefix)
	if !ok || n == "" {
		return false
	}
	for _, r := range n {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Pair は path が repoDir の worktree の一覧にあり、同じ項目に line（"branch refs/heads/…" か "detached"）があるか。
func Pair(repoDir, path, line string) bool {
	out, err := git(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	for _, item := range strings.Split(out, "\n\n") {
		lines := strings.Split(item, "\n")
		w, b := false, false
		for _, l := range lines {
			w = w || l == "worktree "+path
			b = b || l == line
		}
		if w && b {
			return true
		}
	}
	return false
}

// removeTerminals は project=<slug>・task=<id> のラベルの端末を止めて消す。
func removeTerminals(slug, id string) error {
	out, err := exec.Command("hc", "ls", "--all", "--json", "--label", "project="+slug, "--label", "task="+id).Output()
	if err != nil {
		return fail("hc ls に失敗")
	}
	var ls struct {
		Terminals []struct {
			Address string `json:"address"`
			State   string `json:"state"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal(out, &ls); err != nil {
		return fail("hc ls の返事が読めない")
	}
	for _, t := range ls.Terminals {
		if t.State == "live" {
			if err := exec.Command("hc", "kill", t.Address).Run(); err != nil {
				return fail("hc kill %s に失敗", t.Address)
			}
		}
		if err := exec.Command("hc", "rm", t.Address).Run(); err != nil {
			return fail("hc rm %s に失敗", t.Address)
		}
	}
	return nil
}

// defaultBranch は origin の既定の branch を聞き、refs/remotes/origin/<それ> を取り直す。分からなければ空。
func defaultBranch(repoDir string) string {
	sym, err := git(repoDir, "-c", "url.https://github.com/.insteadOf=git@github.com:", "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(sym, "\n") {
		if f := strings.Fields(ln); len(f) == 3 && f[0] == "ref:" && f[2] == "HEAD" && strings.HasPrefix(f[1], "refs/heads/") {
			def := strings.TrimPrefix(f[1], "refs/heads/")
			if _, err := git(repoDir, "fetch", "-q", "origin", "+refs/heads/"+def+":refs/remotes/origin/"+def); err != nil {
				return ""
			}
			return def
		}
	}
	return ""
}

// git は dir で git を打ち、stdout を返す（前後の空白を除く）。
func git(dir string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var errb bytes.Buffer
	c.Stderr = &errb
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
