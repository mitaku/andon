package issue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"andon/internal/ledger"
	"andon/internal/spawn"
)

// worktreeHomes は finish が受ける置き場の根: 今の置き場と旧の置き場（1 バージョンだけ。ADR-0012 決めたこと 6）。
func worktreeHomes() []string {
	return []string{spawn.WorktreeHome(), spawn.LegacyWorktreeHome()}
}

// finish は端末と worktree を片付け、マージ済みなら branch を消し、kanban を done にする（仮設の yard task finish を移したもの）。
// kanban の worktree と branch が、andon issue start がこの課題に作るもの（spawn.WorktreePath: <セルの repo>/.worktrees/<repo 名>/<slug>-<id> と
// <slug>/<id>[-…]）と合うときだけ片付ける。合わなければ端末にも触らずに止まる（kanban の書き違いで別の課題の作業物を消さない）。
// 旧の置き場（~/.herdr/worktrees/<repo 名>/…。ADR-0012 決めたこと 6 の前に起きた課題）の値も対として受ける。
// レビュアーの子課題（parent がこの課題で profile が *-review）と、dispatcher が作った detached worktree
// （<置き場>/<repo 名>/<slug>-<id>-r<回>）も片付ける。
// マージしたかは確かめない（確かめてから打つのは打つ側。dispatcher は MERGED を見てから打つ）。
func (c *ctx) finish(stdout io.Writer) error {
	for _, cmd := range []string{"git", "hc"} {
		if _, err := exec.LookPath(cmd); err != nil {
			return fail("%s が無い", cmd)
		}
	}
	tasks, err := c.l.List()
	if err != nil {
		return err
	}
	ids := []string{c.id}
	for _, t := range tasks {
		if t.Str("parent") == c.id && strings.HasSuffix(t.Str("profile"), "-review") && t.Str("status") != "done" {
			ids = append(ids, t.Str("id"))
		}
	}
	if err := c.closable(ids); err != nil {
		return err
	}

	slug, repo := c.l.Slug, c.issue.Str("repo")
	wt, branch := c.issue.Path("worktree"), c.issue.Str("branch")
	repoDir := ""
	if repo != "" {
		d, err := repoDirOf(repo)
		if err != nil {
			return err
		}
		repoDir = d
	}

	if repo == "" && wt != "" {
		top, err := git(filepath.Join(c.l.Dir, "issues", c.id), "rev-parse", "--show-toplevel")
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
		if MainCheckout(repoDir, wt) {
			return fail("%s の worktree が repo の本体の checkout: %s（消さない。kanban の worktree・branch を確かめる）", c.id, wt)
		}
		want := ""
		for _, home := range worktreeHomes() {
			dir := filepath.Join(home, filepath.Base(repoDir))
			if p := c.issue.Str("parent"); p != "" && strings.HasSuffix(c.issue.Str("profile"), "-review") && branch == "" && reviewRound(wt, filepath.Join(dir, slug+"-"+p+"-r")) {
				want, detached = wt, true
				break
			}
			if p := filepath.Join(dir, slug+"-"+c.id); wt == p {
				want = p
				break
			}
		}
		if want == "" {
			want = spawn.WorktreePath(repoDir, slug, c.id)
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
			own = true
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

	for _, t := range tasks {
		if t.Str("parent") != c.id || !strings.HasSuffix(t.Str("profile"), "-review") || t.Str("status") == "done" {
			continue
		}
		child := &ctx{l: c.l, id: t.Str("id"), issue: t}
		if err := child.finish(stdout); err != nil {
			return fail("レビュアーの課題 %s を片付けられない: %v", child.id, err)
		}
	}
	if repoDir != "" {
		var olds []string
		for _, home := range worktreeHomes() {
			m, _ := filepath.Glob(filepath.Join(home, filepath.Base(repoDir), slug+"-"+c.id+"-r*"))
			olds = append(olds, m...)
		}
		for _, p := range olds {
			if Pair(repoDir, p, "detached") && !MainCheckout(repoDir, p) {
				if _, err := git(repoDir, "worktree", "remove", "--force", p); err != nil {
					return fail("レビュアーの detached worktree を消せない: %s", p)
				}
			}
		}
	}

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
		git(repoDir, "worktree", "prune") //nolint:errcheck // prune は消えた worktree の印を掃くだけで、試すだけ
		if branch != "" {
			if _, err := git(repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
				switch def := defaultBranch(repoDir); {
				case !own:
					kept = fmt.Sprintf(" branch %s は worktree と組で確かめられないので残した", branch)
				case def == "":
					kept = fmt.Sprintf(" branch %s は origin の既定の branch が分からないので残した", branch)
				default:
					if _, err := git(repoDir, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/remotes/origin/"+def); err == nil {
						kept = deleteBranch(repoDir, branch)
					} else if tip, why := c.mergedPRHead(repoDir, branch); tip != "" {
						// squash マージは branch の commit が main に入らない。MERGED の PR の head と先が同じなら作業は失われない。
						// 確かめた SHA を期待値にして消す（確かめたあとに branch が進んでいたら消えない）
						if branchCheckedOut(repoDir, branch) {
							kept = fmt.Sprintf(" branch %s はほかの worktree が使っているので残した", branch)
						} else if _, err := git(repoDir, "update-ref", "-d", "refs/heads/"+branch, tip); err != nil {
							kept = fmt.Sprintf(" branch %s は確かめたあとに動いたので残した", branch)
						}
					} else {
						kept = fmt.Sprintf(" branch %s は origin/%s に入っていないので残した（%s）", branch, def, why)
					}
				}
			}
		}
	}
	if _, err := c.l.Run("", "done", c.id); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "andon issue: %s を片付けた（done）。%s\n", c.id, kept)
	return nil
}

func (c *ctx) closable(ids []string) error {
	held, err := c.unreceived(ids, nil)
	if err != nil {
		return err
	}
	if len(held) > 0 {
		return fail("%s は受け取られていない問いがあるので片付けない（端末・worktree に触っていない）: %s。問いを received にしてから打ち直す", c.id, strings.Join(held, "・"))
	}
	return nil
}

func (c *ctx) unreceived(ids []string, skip map[string]string) ([]string, error) {
	out, err := c.l.Run("", "questions", "--json")
	if err != nil {
		return nil, err
	}
	var all []struct {
		ID     string `json:"id"`
		N      int    `json:"n"`
		Status string `json:"status"`
		Broken string `json:"broken"`
	}
	if out != "" {
		if err := json.Unmarshal([]byte(out), &all); err != nil {
			return nil, fail("問いの一覧が読めない")
		}
	}
	var held []string
	for _, q := range all {
		id := ledger.CanonID(q.ID)
		if !slices.Contains(ids, id) {
			continue
		}
		switch {
		case q.Broken != "":
			held = append(held, fmt.Sprintf("%s（questions.yaml が読めない: %s）", id, q.Broken))
		case (q.Status == "open" || q.Status == "answered") && skip[fmt.Sprintf("%s#%d", id, q.N)] == "":
			held = append(held, fmt.Sprintf("%s#%d（%s）", id, q.N, q.Status))
		}
	}
	return held, nil
}

func (c *ctx) mergedPRHead(repoDir, branch string) (string, string) {
	rep, err := c.previous()
	if err != nil || rep == nil || rep.PR == "" {
		return "", "report.json に pr が無い"
	}
	var view struct {
		State      string `json:"state"`
		HeadRefOid string `json:"headRefOid"`
	}
	out, err := exec.Command("gh", "pr", "view", rep.PR, "--json", "state,headRefOid").Output()
	if err != nil || json.Unmarshal(out, &view) != nil {
		return "", "PR を読めない"
	}
	if view.State != "MERGED" {
		return "", "PR が MERGED でない: " + view.State
	}
	tip, err := git(repoDir, "rev-parse", "refs/heads/"+branch)
	if err != nil || tip != view.HeadRefOid {
		return "", "PR の head のあとに commit が足されている"
	}
	return tip, ""
}

// branchCheckedOut は branch をどれかの worktree が checkout しているか（git branch -D の保護と同じ。update-ref -d には無い）。
// 一覧が読めなければ使われているものとして扱う。
func branchCheckedOut(repoDir, branch string) bool {
	out, err := git(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return true
	}
	for _, l := range strings.Split(out, "\n") {
		if l == "branch refs/heads/"+branch {
			return true
		}
	}
	return false
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

func removeTerminals(slug, id string) error {
	out, err := exec.Command("hc", "ls", "--all", "--json", "--label", "project="+slug, "--label", "issue="+id).Output()
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

func deleteBranch(repoDir, branch string) string {
	if _, err := git(repoDir, "branch", "-D", branch); err != nil {
		return fmt.Sprintf(" branch %s は消せなかったので残した（%v）", branch, err)
	}
	return ""
}

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
