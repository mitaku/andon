// Package chiefengineer は andon chief-engineer: 人と仕様・設計を対話する Chief Engineer を
// 台帳のタスクとして起こす（docs/design.md §1.8）。
//
//	andon chief-engineer "<題>"                           projects/<slug>/ で打つ。repo は台帳の repos が1つならそれ
//	andon chief-engineer --repo <host/owner/repo> "<題>"   台帳の repos が2つ以上（か無い）とき
//
// することは4つ: 台帳にタスクを足す（kanban add）・案件の repo に worktree と branch を切る
// （文書の PR をこのタスクから出すため）・端末の土台でプロファイル chief-engineer を起こす（hc run）・
// タスクを doing にして agent をその端末にする（kanban start）。台帳の CLI は kanban。
// 1プロジェクトに Chief Engineer は1人（WIP 制限）なので、動いていれば起こさずに止まる。
//
// 置き場（andon task start と同じ形）:
//
//	worktree  ~/.herdr/worktrees/<repo 名>/<slug>-<id>
//	branch    <slug>/<id>-<題の英字の短い形>。origin/<既定の branch> から --no-track で切る
//	repo      $GHQ_ROOT（無ければ ghq root）/<host/owner/repo>
package chiefengineer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"andon/internal/ledger"
)

// Usage は andon chief-engineer の使い方。
const Usage = "usage: andon chief-engineer [--repo <host/owner/repo>] \"<題>\"\n" +
	"projects/<slug>/ で打つ。--repo を省くと台帳の config.yaml の repos（1つのとき）。2つ以上なら --repo が要る。\n" +
	"台帳にタスクを足し（プロファイル chief-engineer）、repo に worktree を切り、\n" +
	"hc で Chief Engineer を起こし、タスクを doing にする。文書の PR はそのタスクから出る。\n" +
	"Chief Engineer が動いていれば起こさずに止まる（1プロジェクトに1人）。\n"

// Profile は Chief Engineer のプロファイル（hc の template の名前）。
const Profile = "chief-engineer"

const (
	exitFail  = 1
	exitUsage = 2
	exitFirst = 3 // 起きたが、最初のメッセージが届いていない
)

var (
	slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`) // schema の project と同じ
	repoRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	idRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	nameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

// Main は andon chief-engineer を走らせ、終了コードを返す。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	repo, title, ok := parseArgs(args)
	if !ok {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	die := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "andon chief-engineer: "+format+"\n", a...)
		return exitFail
	}

	if repo != "" && !repoRe.MatchString(repo) {
		fmt.Fprintf(stderr, "andon chief-engineer: repo が不正: '%s'（<host>/<owner>/<repo>）\n", repo)
		return exitUsage
	}
	cli := ledger.CLI
	if err := ledger.Check(); err != nil {
		return die("%v", err)
	}
	for _, c := range []string{"hc", "git"} {
		if _, err := exec.LookPath(c); err != nil {
			return die("%s が無い", c)
		}
	}

	root, err := output(nil, "", cli, "root")
	if err != nil || root == "" {
		return die("台帳が見つからない（projects/<slug>/ で打つ）")
	}
	slug := filepath.Base(root)
	if !slugRe.MatchString(slug) {
		return die("slug が不正: '%s'（台帳の root のフォルダ名。英小文字・数字・_・-）", slug)
	}
	if repo == "" {
		repos, ok := ledgerRepos(filepath.Join(root, ".flightdeck", "config.yaml"))
		switch {
		case !ok:
			fmt.Fprintf(stderr, "andon chief-engineer: 台帳の config.yaml の repos が読めない。--repo <host/owner/repo> で渡す\n")
			return exitUsage
		case len(repos) != 1:
			fmt.Fprintf(stderr, "andon chief-engineer: 台帳の repos が %d 個（%s）。--repo <host/owner/repo> で選ぶ\n", len(repos), strings.Join(repos, "・"))
			return exitUsage
		}
		repo = repos[0]
		if !repoRe.MatchString(repo) {
			return die("台帳の repos の値が不正: '%s'", repo)
		}
	}
	repoDir, err := repoPath(repo)
	if err != nil {
		return die("%v", err)
	}
	if _, err := output(nil, "", "git", "-C", repoDir, "rev-parse", "--git-dir"); err != nil {
		return die("リポジトリが無い: %s（ghq get する）", repoDir)
	}
	env := []string{"FLIGHTDECK_ROOT=" + root}

	// 1プロジェクトに1人
	ls, err := output(nil, "", "hc", "ls", "--json", "--label", "role="+Profile, "--label", "project="+slug)
	if err != nil {
		return die("hc ls に失敗: %v", err)
	}
	var live struct {
		Terminals []struct {
			Address string            `json:"address"`
			Labels  map[string]string `json:"labels"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(ls), &live); err != nil {
		return die("hc ls の返事が読めない: %v", err)
	}
	if len(live.Terminals) > 0 {
		t := live.Terminals[0]
		return die("Chief Engineer が動いている: %s（タスク %s）。話すには hc attach %s。新しい題は、前の題を渡してから", t.Address, t.Labels["task"], t.Address)
	}

	tpl, err := output(nil, "", "hc", "templates", "--json")
	if err != nil {
		return die("hc templates に失敗: %v", err)
	}
	var tpls struct {
		Templates []struct {
			Name string `json:"name"`
		} `json:"templates"`
	}
	if err := json.Unmarshal([]byte(tpl), &tpls); err != nil {
		return die("hc templates の返事が読めない: %v", err)
	}
	found := false
	for _, t := range tpls.Templates {
		found = found || t.Name == Profile
	}
	if !found {
		return die("プロファイル %s の template が無い（hc templates で確かめる。作るのは人）", Profile)
	}

	brief := "# " + title + "\n\n" +
		"Chief Engineer のタスク。人と直接話して仕様・設計を詰め、文書にして Chief of Staff へ渡す。\n" +
		"題は人の1行。手順は skill chief-engineer。\n"
	id, err := output(env, brief, cli, "add", title, "--repo", repo, "--profile", Profile, "--brief", "-")
	if err != nil || !idRe.MatchString(id) {
		return die("%s add に失敗: %v %q", cli, err, id)
	}

	name := strings.ToLower(slug + "-" + id)
	if !nameRe.MatchString(name) {
		return die("端末の名前 %s が hc の決まり（a-z 0-9 - _ で32字まで）に合わない。タスク %s は todo のまま残る", name, id)
	}
	wt, err := newWorktree(repoDir, slug, id, title)
	if err != nil {
		return die("%v。タスク %s は todo のまま残る", err, id)
	}
	if _, err := output(env, "", cli, "set", id, "branch="+wt.branch, "worktree="+wt.path); err != nil {
		wt.remove()
		return die("%s set %s branch/worktree に失敗: %v。タスク %s は todo のまま残る", cli, id, err, id)
	}
	undo := func() {
		wt.remove()
		output(env, "", cli, "set", id, "branch=", "worktree=")
	}
	cwd := wt.path
	msg := fmt.Sprintf("タスク `%s/%s`。\n依頼書は `%s`。\n先に `%s` を読む。",
		slug, id, filepath.Join(root, ".flightdeck", "tasks", id, "brief.md"), filepath.Join(home(), "skills", "chief-engineer", "SKILL.md"))

	// --no-notify: 止まった・終わったはクーリエが端末の土台から拾う（contract.md §8）
	out, err := output(nil, "", "hc", "run", "--json", "--bg", "--outlive-parent", "--no-notify", "--name", name,
		"--label", "role="+Profile, "--label", "project="+slug, "--label", "task="+id, "--label", "title="+title,
		"--cwd", cwd, Profile, msg)
	if err != nil {
		undo()
		return die("hc run に失敗: %v。worktree と branch を戻した。タスク %s は todo のまま残る", err, id)
	}
	var run struct {
		Address      string `json:"address"`
		FirstMessage struct {
			Sent   bool   `json:"sent"`
			Reason string `json:"reason"`
		} `json:"first_message"`
	}
	if err := json.Unmarshal([]byte(out), &run); err != nil || run.Address == "" {
		undo()
		return die("hc run の返事に address が無い: %s。タスク %s は todo のまま残る", out, id)
	}

	if _, err := outputIn(cwd, append(env, "HC_TERMINAL_ID="+run.Address), "", cli, "start", id); err != nil {
		return die("%s start %s に失敗: %v（端末 %s は起きている）", cli, id, err, run.Address)
	}

	if !run.FirstMessage.Sent {
		reason := run.FirstMessage.Reason
		if reason == "" {
			reason = "理由なし"
		}
		fmt.Fprintf(stderr, "andon chief-engineer: %s（%s）に最初のメッセージが届いていない: %s。hc attach %s で画面に答えてから、次を送る: hc send %s %q\n",
			run.Address, name, reason, run.Address, run.Address, msg)
		return exitFirst
	}
	fmt.Fprintf(stdout, "andon chief-engineer: %s（%s）を %s（%s）で起こした。cwd %s branch %s。話すには hc attach %s\n", id, title, run.Address, name, cwd, wt.branch, run.Address)
	return 0
}

// parseArgs は --repo <r>（--repo=<r> も）と題1つを受ける。
func parseArgs(args []string) (repo, title string, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--repo" && i+1 < len(args) && repo == "":
			repo = args[i+1]
			i++
		case strings.HasPrefix(a, "--repo=") && repo == "":
			repo = strings.TrimPrefix(a, "--repo=")
		case strings.HasPrefix(a, "-") || title != "":
			return "", "", false
		default:
			title = strings.TrimSpace(a)
			if title == "" || strings.ContainsAny(a, "\r\n") {
				return "", "", false
			}
		}
	}
	return repo, title, title != ""
}

// ledgerRepos は台帳の config.yaml の repos を読む。台帳の CLI に repos を出す口が無いので、
// 台帳の CLI が書く形（行頭の "repos:" の下に "  - <repo>" が並ぶ。無ければ "repos: []"）だけを読む。
// それ以外の形（flow の並び・入れ子・読めない）は ok=false（安全側: 呼び手は --repo を求める）。
func ledgerRepos(path string) (repos []string, ok bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	in, seen := false, false
	for _, ln := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		switch {
		case ln == "repos:":
			if seen {
				return nil, false
			}
			in, seen = true, true
		case ln == "repos: []":
			if seen {
				return nil, false
			}
			seen = true
		case in && strings.HasPrefix(ln, "  - "):
			v := strings.TrimSpace(strings.TrimPrefix(ln, "  - "))
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			if v == "" {
				return nil, false
			}
			repos = append(repos, v)
		case in && (ln == "" || !strings.HasPrefix(ln, " ")):
			in = false
		case in:
			return nil, false
		case strings.HasPrefix(ln, "repos:"):
			return nil, false
		}
	}
	return repos, seen
}

// repoPath は $GHQ_ROOT（無ければ ghq root）/<repo>。
func repoPath(repo string) (string, error) {
	root := os.Getenv("GHQ_ROOT")
	if root == "" {
		if out, err := output(nil, "", "ghq", "root"); err == nil {
			root = out
		}
	}
	if root == "" {
		return "", fmt.Errorf("GHQ_ROOT が無い")
	}
	return filepath.Join(root, repo), nil
}

type worktree struct{ repoDir, path, branch string }

// newWorktree は origin の既定の branch の最新から、upstream の無い branch と worktree を切る（andon task start と同じ置き場）。
func newWorktree(repoDir, slug, id, title string) (*worktree, error) {
	g := func(args ...string) (string, error) {
		return output(nil, "", "git", append([]string{"-c", "url.https://github.com/.insteadOf=git@github.com:", "-C", repoDir}, args...)...)
	}
	sym, err := g("ls-remote", "--symref", "origin", "HEAD")
	def := ""
	for _, ln := range strings.Split(sym, "\n") {
		if f := strings.Fields(ln); len(f) == 3 && f[0] == "ref:" && f[2] == "HEAD" && strings.HasPrefix(f[1], "refs/heads/") {
			def = strings.TrimPrefix(f[1], "refs/heads/")
		}
	}
	if err != nil || def == "" {
		return nil, fmt.Errorf("origin の既定の branch が分からない（%s）", repoDir)
	}
	if _, err := g("fetch", "-q", "origin", "+refs/heads/"+def+":refs/remotes/origin/"+def); err != nil {
		return nil, fmt.Errorf("origin/%s を fetch できない（%s）", def, repoDir)
	}
	branch := slug + "/" + id
	if short := shortTitle(title); short != "" {
		branch += "-" + short
	}
	if _, err := g("check-ref-format", "--branch", branch); err != nil {
		return nil, fmt.Errorf("branch 名が不正: %s", branch)
	}
	if _, err := g("show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return nil, fmt.Errorf("branch %s がもうある", branch)
	}
	path := filepath.Join(os.Getenv("HOME"), ".herdr", "worktrees", filepath.Base(repoDir), slug+"-"+id)
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("worktree の置き場がもうある: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if _, err := g("worktree", "add", "-q", "--no-track", "-b", branch, path, "refs/remotes/origin/"+def); err != nil {
		return nil, fmt.Errorf("worktree を作れない: %s", path)
	}
	return &worktree{repoDir: repoDir, path: path, branch: branch}, nil
}

// remove は newWorktree が作ったものを戻す（起こせなかったとき。やり直せるように）。
func (w *worktree) remove() {
	output(nil, "", "git", "-C", w.repoDir, "worktree", "remove", "--force", w.path)
	output(nil, "", "git", "-C", w.repoDir, "branch", "-D", w.branch)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// shortTitle は題の英字の短い形: ASCII の英数字以外を - にして小文字・30字まで（日本語だけの題なら空）。
func shortTitle(title string) string {
	s := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(s) > 30 {
		s = strings.TrimRight(s[:30], "-")
	}
	return s
}

// home はパッケージの置き場。ANDON_HOME（checkout の package/ を指して試す）が無ければ /opt/andon（design.md §2.5）。
func home() string {
	if h := os.Getenv("ANDON_HOME"); h != "" {
		return h
	}
	return "/opt/andon"
}

// output はコマンドを打ち、stdout を返す（前後の空白を除く）。stderr は失敗のときだけ err に入れる。
func output(env []string, stdin, name string, args ...string) (string, error) {
	return outputIn("", env, stdin, name, args...)
}

// outputIn は dir で output する。
func outputIn(dir string, env []string, stdin, name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	c.Stdin = strings.NewReader(stdin)
	var errb bytes.Buffer
	c.Stderr = &errb
	b, err := c.Output()
	if err != nil {
		if s := strings.TrimSpace(errb.String()); s != "" {
			return "", fmt.Errorf("%w: %s", err, firstLine(s))
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
