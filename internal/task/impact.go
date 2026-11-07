package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// exitNotMerged は andon task impact で、タスクの PR がまだ MERGED でないとき（何も出さない。dispatcher は飛ばす）。
const exitNotMerged = 4

type notMerged struct{ state string }

func (n notMerged) Error() string { return "PR が MERGED でない（" + n.state + "）" }

// impact はタスクの PR がマージされたあと、同じ repo の動いているタスクへの影響の事実を集めて JSON で出す。
// 書くのは git fetch だけ（台帳・worktree・branch には触らない）。何度打っても同じ事実から同じ出力になる。
func (c *ctx) impact(stdout io.Writer) error {
	repo := c.task.Str("repo")
	if repo == "" {
		return fail("%s に repo が無い", c.id)
	}
	rep, err := c.previous()
	if err != nil {
		return err
	}
	if rep == nil || rep.PR == "" {
		return fail("%s の report.json に pr が無い", c.id)
	}
	var view struct {
		State       string `json:"state"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	var errb bytes.Buffer
	cmd := exec.Command("gh", "pr", "view", rep.PR, "--json", "state,mergeCommit,files")
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil || json.Unmarshal(out, &view) != nil {
		return fail("PR を読めない: %s %s", rep.PR, strings.TrimSpace(errb.String()))
	}
	if view.State != "MERGED" {
		return notMerged{view.State}
	}
	if view.MergeCommit == nil || !contract.IsSHA(view.MergeCommit.Oid) {
		return fail("PR のマージの commit が読めない: %s", rep.PR)
	}
	sha := view.MergeCommit.Oid
	repoDir, err := repoDirOf(repo)
	if err != nil {
		return err
	}
	if _, err := git(repoDir, "fetch", "--quiet", "origin"); err != nil {
		return fail("origin を取り直せない: %v", err)
	}
	if _, err := git(repoDir, "cat-file", "-e", sha+"^{commit}"); err != nil {
		return fail("マージの commit %s が %s に無い", sha, repoDir)
	}
	res := contract.Impact{Task: c.id, PR: rep.PR, Sha: sha, Files: []string{}, Tasks: []contract.ImpactTask{}}
	for _, f := range view.Files {
		res.Files = append(res.Files, f.Path)
	}
	sort.Strings(res.Files)
	tasks, err := c.l.List()
	if err != nil {
		return err
	}
	for _, t := range tasks {
		s := t.Str("status")
		if t.Str("id") == c.id || t.Str("repo") != repo || (s != "doing" && s != "review") || t.Str("branch") == "" ||
			(t.Str("parent") != "" && strings.HasSuffix(t.Str("profile"), "-review")) {
			continue
		}
		res.Tasks = append(res.Tasks, c.impactOf(t, repoDir, sha, res.Files))
	}
	_, err = stdout.Write(contract.Marshal(res))
	return err
}

// impactOf は1つのタスクについて、マージの commit との事実を集める。読めないところは Error に書いて続ける。
func (c *ctx) impactOf(t ledger.Task, repoDir, sha string, merged []string) contract.ImpactTask {
	id := t.Str("id")
	it := contract.ImpactTask{Task: id, Title: t.Str("title"), Status: t.Str("status"), Agent: t.Str("agent"), Branch: t.Str("branch"),
		ConflictFiles: []string{}, Overlap: []string{}, BriefRefs: []string{}}
	if r, err := contract.ReadReport(filepath.Join(c.l.Library(id), "report.json")); err == nil {
		it.PR = r.PR
	}
	if b, err := os.ReadFile(filepath.Join(c.l.Dir, "tasks", id, "brief.md")); err == nil {
		it.BriefRefs = BriefRefs(string(b), merged)
	}
	head, err := git(repoDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+it.Branch+"^{commit}")
	if err != nil || !contract.IsSHA(head) {
		it.Error = "branch " + it.Branch + " が読めない"
		return it
	}
	it.Head = head
	if _, err := git(repoDir, "merge-base", "--is-ancestor", sha, head); err == nil {
		it.Contains = true
		return it
	}
	base, err := git(repoDir, "merge-base", head, sha)
	if err != nil {
		it.Error = "マージの commit との merge-base が無い"
		return it
	}
	changed, err := git(repoDir, "diff", "--name-only", "--no-renames", base, head)
	if err != nil {
		it.Error = "branch の差分が読めない"
		return it
	}
	files := lines(changed)
	// worktree の commit していない変更（追跡している変更と、まだ git add していない新しいファイル）も数える。
	// worktree が無い（片付けた・別のマシン）ときは branch の分だけ。あるのに読めなければ error（影響なしに倒さない）
	if wt := t.Path("worktree"); wt != "" {
		if _, err := os.Stat(wt); err == nil {
			dirty, err1 := git(wt, "diff", "--name-only", "--no-renames", "HEAD")
			untracked, err2 := git(wt, "ls-files", "--others", "--exclude-standard")
			if err1 != nil || err2 != nil {
				it.Error = "worktree の commit していない変更が読めない: " + wt
				return it
			}
			files = append(files, lines(dirty)...)
			files = append(files, lines(untracked)...)
		}
	}
	it.Overlap = Intersect(merged, files)
	conflict, cf, err := mergeTree(repoDir, head, sha)
	if err != nil {
		it.Error = err.Error()
		return it
	}
	it.Conflict, it.ConflictFiles = conflict, cf
	return it
}

// mergeTree は head と sha を git merge-tree で合わせてみる（どの worktree にも触らない）。ぶつかればそのファイルを返す。
func mergeTree(repoDir, head, sha string) (bool, []string, error) {
	cmd := exec.Command("git", "-C", repoDir, "merge-tree", "--write-tree", "--name-only", "--no-messages", head, sha)
	out, err := cmd.Output()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return false, []string{}, nil
	case errors.As(err, &ee) && ee.ExitCode() == 1:
		ls := lines(string(out))
		if len(ls) > 0 {
			ls = ls[1:] // 1行目は木の oid
		}
		sort.Strings(ls)
		return true, Intersect(ls, ls), nil
	}
	return false, nil, fmt.Errorf("git merge-tree に失敗")
}

// BriefRefs は、files のうち依頼書の文に path そのものか、名前（base name）が出てくるものを返す（並びは files の順）。
func BriefRefs(brief string, files []string) []string {
	out := []string{}
	for _, f := range files {
		if strings.Contains(brief, f) || strings.Contains(brief, filepath.Base(f)) {
			out = append(out, f)
		}
	}
	return out
}

// Intersect は a と b の両方にあるものを、a の順に重ねずに返す。
func Intersect(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range b {
		in[x] = true
	}
	out, seen := []string{}, map[string]bool{}
	for _, x := range a {
		if in[x] && !seen[x] {
			out = append(out, x)
			seen[x] = true
		}
	}
	return out
}

func lines(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// repoDirOf は台帳の repo（host/owner/name）の ghq の checkout。
func repoDirOf(repo string) (string, error) {
	root := os.Getenv("GHQ_ROOT")
	if root == "" {
		out, err := exec.Command("ghq", "root").Output()
		if err != nil {
			return "", fail("GHQ_ROOT が無い")
		}
		root = strings.TrimSpace(string(out))
	}
	return filepath.Join(root, repo), nil
}
