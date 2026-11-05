// Package chiefengineer は andon chief-engineer: 人と仕様・設計を対話する Chief Engineer を
// 台帳のタスクとして起こす（docs/design.md §1.8）。
//
//	andon chief-engineer "<題>"                           projects/<slug>/ で打つ。repo は台帳の repos が1つならそれ
//	andon chief-engineer --repo <host/owner/repo> "<題>"   台帳の repos が2つ以上（か無い）とき
//
// することは4つ: 台帳にタスクを足す（kanban add）・案件の repo に worktree と branch を切る
// （文書の PR をこのタスクから出すため）・端末の土台でプロファイル chief-engineer を起こす（hc run）・
// タスクを doing にして agent をその端末にする（kanban start）。台帳の CLI は kanban。
// 端末にはラベル herdr.space=andon-<slug>・herdr.tab=andon-<slug>-<タスク id> を付け、人の Herdr App の指揮側の Space に
// タスクの tab として出す。hc attach は打たず、最後に出た場所を「<slug> / CE <id> <題>」で出す（ADR-0002 決めたこと 8・14・17）。
// 1プロジェクトに Chief Engineer は1人（WIP 制限）なので、動いていれば起こさずに止まる。
//
// 置き場（andon task start と同じ形。internal/spawn）:
//
//	worktree  ~/.herdr/worktrees/<repo 名>/<slug>-<id>
//	branch    <slug>/<id>-<題の英字の短い形>。origin/<既定の branch> から --no-track で切る
//	repo      $GHQ_ROOT（無ければ ghq root）/<host/owner/repo>
package chiefengineer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"andon/internal/ledger"
	"andon/internal/spawn"
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
	slugRe = spawn.SlugRe
	repoRe = spawn.RepoRe
	idRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	nameRe = spawn.NameRe
)

var output = spawn.Output

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
	repoDir, err := spawn.RepoPath(repo)
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
		return die("Chief Engineer が動いている: %s（%s）。新しい題は、前の題を渡してから", where(slug, t.Labels["task"], t.Labels["title"]), t.Address)
	}

	found, err := spawn.HasTemplate(Profile)
	if err != nil {
		return die("%v", err)
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
	wt, err := spawn.NewWorktree(repoDir, slug, id, title)
	if err != nil {
		return die("%v。タスク %s は todo のまま残る", err, id)
	}
	if _, err := output(env, "", cli, "set", id, "branch="+wt.Branch, "worktree="+wt.Path); err != nil {
		wt.Remove()
		return die("%s set %s branch/worktree に失敗: %v。タスク %s は todo のまま残る", cli, id, err, id)
	}
	undo := func() {
		wt.Remove()
		output(env, "", cli, "set", id, "branch=", "worktree=")
	}
	cwd := wt.Path
	// skill chief-engineer はプロファイルの command（--append-system-prompt-file）が system prompt に足すので、最初のメッセージはタスクの2行だけ（ADR-0001 決定 3）
	msg := fmt.Sprintf("タスク `%s/%s`。\n依頼書は `%s`。",
		slug, id, filepath.Join(root, ".flightdeck", "tasks", id, "brief.md"))

	labels := []string{"role=" + Profile, "project=" + slug, "task=" + id, "title=" + title,
		"herdr.space=andon-" + slug, "herdr.tab=andon-" + slug + "-" + id}
	run, err := spawn.Run(name, labels, cwd, Profile, msg)
	if err != nil {
		undo()
		return die("%v。worktree と branch を戻した。タスク %s は todo のまま残る", err, id)
	}

	if _, err := spawn.OutputIn(cwd, append(env, "HC_TERMINAL_ID="+run.Address), "", cli, "start", id); err != nil {
		return die("%s start %s に失敗: %v（端末 %s は起きている）", cli, id, err, run.Address)
	}

	place := where(slug, id, title)
	if !run.First.Sent {
		fmt.Fprintf(stderr, "andon chief-engineer: %s（%s）に最初のメッセージが届いていない: %s。App の %s で答えてから、次を送る: hc send %s %q\n",
			run.Address, name, run.Reason(), place, run.Address, msg)
		fmt.Fprintf(stdout, "andon chief-engineer: 出た場所 %s（%s）\n", place, run.Address)
		return exitFirst
	}
	fmt.Fprintf(stdout, "andon chief-engineer: %s（%s）を %s（%s）で起こした。cwd %s branch %s\n", id, title, run.Address, name, cwd, wt.Branch)
	fmt.Fprintf(stdout, "andon chief-engineer: 出た場所 %s（%s）\n", place, run.Address)
	return 0
}

// where は人に端末を指す言い方「<Space 名> / <tab 名>」（ADR-0002 決めたこと 16・17）。名前は App の mod が付ける。
func where(slug, id, title string) string {
	return slug + " / CE " + id + " " + title
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
