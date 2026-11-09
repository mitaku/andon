// Package chiefengineer は andon chief-engineer: 人と仕様・設計を対話する Chief Engineer を
// 台帳の課題として起こす（docs/design.md §1.8）。
//
//	andon chief-engineer "<題>"                           projects/<slug>/ で打つ。repo は台帳の repos が1つならそれ
//	andon chief-engineer --repo <host/owner/repo> "<題>"   台帳の repos が2つ以上（か無い）とき
//	andon chief-engineer --brief - "<題>"                 経緯（人の言葉の写し・読んだもの・決めたこと・出典）を stdin で渡す
//
// することは4つ: 台帳に課題を足す（kanban add）・案件の repo に worktree と branch を切る
// （文書の PR をこの課題から出すため）・端末の土台でプロファイル chief-engineer を起こす（hc run）・
// 課題を doing にして agent をその端末にする（kanban start）。台帳の CLI は kanban。
// 端末にはラベル herdr.space=andon-<slug>・herdr.tab=andon-<slug>-<課題 id> を付け、人の Herdr App の指揮側の Space に
// 課題の tab として出す。hc attach は打たず、最後に出た場所を「<slug> / CE <id> <題>」で出す（ADR-0002 決めたこと 8・14・17）。
// --brief - の文は依頼書の既定の行の下に続ける（Chief Engineer が最初に読むのは依頼書なので、送り直しが要っても残る）。
// Chief Engineer は1プロジェクトに同時に複数いてよい（ADR-0009 決めたこと 3）。動いている端末があっても起こす。
// 作り直しで戻った端末（top-level の argv ごと素のシェル）だけは片付けてから起こす。それ以外の端末には触らない。
//
// 置き場（andon issue start と同じ形。internal/spawn）:
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

	"andon/internal/boot"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// Usage は andon chief-engineer の使い方。
const Usage = "usage: andon chief-engineer [--repo <host/owner/repo>] [--brief -] \"<題>\"\n" +
	"projects/<slug>/ で打つ。--repo を省くと台帳の config.yaml の repos（1つのとき）。2つ以上なら --repo が要る。\n" +
	"台帳に課題を足し（プロファイル chief-engineer）、repo に worktree を切り、\n" +
	"hc で Chief Engineer を起こし、課題を doing にする。文書の PR はその課題から出る。\n" +
	"--brief - は stdin の文（経緯）を依頼書の既定の行の下に足す。\n" +
	"Chief Engineer は1プロジェクトに複数いてよい。作り直しで戻った端末だけ片付けてから起こす。\n"

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
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	repo, title, withBrief, ok := parseArgs(args)
	if !ok {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	extra := ""
	if withBrief {
		b, err := io.ReadAll(stdin)
		extra = strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n"))
		if err != nil || extra == "" {
			fmt.Fprintf(stderr, "andon chief-engineer: --brief - の stdin が空（経緯を渡すか、--brief を省く）\n")
			return exitUsage
		}
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
		repos, ok := ledgerRepos(filepath.Join(root, "kanban", "config.yaml"))
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
	env := []string{"KANBAN_ROOT=" + root}

	// 作り直しで戻った端末を探す（動いている Chief Engineer は数えない。ADR-0009 決めたこと 3）
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
	// ラベルと live だけでは決めない: 作り直しで戻った端末（top-level の argv ごと素のシェル）だけ片付ける（前の課題は台帳に残る）。
	// 動いている・どちらとも言えない端末には触らない
	var shells []*spawn.Term
	for _, lt := range live.Terminals {
		t, err := spawn.Look(lt.Address)
		if err != nil {
			return die("%v", err)
		}
		t.Address = lt.Address
		if t.Labels == nil {
			t.Labels = lt.Labels
		}
		if t.Kind() == spawn.Restored {
			shells = append(shells, t)
		}
	}
	for _, t := range shells {
		if err := spawn.Discard(t.Address); err != nil {
			return die("%s（Chief Engineer のラベルで中身がシェル）を片付けられない: %v", t.Address, err)
		}
		fmt.Fprintf(stdout, "andon chief-engineer: %s（%s）は中身がシェルに替わっていた（作り直しで戻った端末）ので片付けた。課題 %s は台帳に残る\n",
			where(slug, t.Labels["issue"], t.Labels["title"]), t.Address, t.Labels["issue"])
	}

	found, err := spawn.HasTemplate(Profile)
	if err != nil {
		return die("%v", err)
	}
	if !found {
		return die("プロファイル %s の template が無い（hc templates で確かめる。作るのは人）", Profile)
	}

	brief := "# " + title + "\n\n" +
		"Chief Engineer の課題。人と直接話して仕様・設計を詰め、文書にして Chief of Staff へ渡す。\n" +
		"題は人の1行。手順は skill chief-engineer。\n"
	if extra != "" {
		brief += "\n" + extra + "\n"
	}
	id, err := output(env, brief, cli, "add", title, "--repo", repo, "--profile", Profile, "--brief", "-")
	if err != nil || !idRe.MatchString(id) {
		return die("%s add に失敗: %v %q", cli, err, id)
	}

	name := strings.ToLower(slug + "-" + id)
	if !nameRe.MatchString(name) {
		return die("端末の名前 %s が hc の決まり（a-z 0-9 - _ で32字まで）に合わない。課題 %s は todo のまま残る", name, id)
	}
	wt, err := spawn.NewWorktree(repoDir, slug, id, title)
	if err != nil {
		return die("%v。課題 %s は todo のまま残る", err, id)
	}
	if _, err := output(env, "", cli, "set", id, "branch="+wt.Branch, "worktree="+wt.Path); err != nil {
		wt.Remove()
		return die("%s set %s branch/worktree に失敗: %v。課題 %s は todo のまま残る", cli, id, err, id)
	}
	undo := func() {
		wt.Remove()
		output(env, "", cli, "set", id, "branch=", "worktree=")
	}
	cwd := wt.Path
	// skill chief-engineer はプロファイルの command（--append-system-prompt-file）が system prompt に足す
	msg := spawn.FirstMessage(slug, id, filepath.Join(root, "kanban", "issues", id, "brief.md"))

	// 信頼の画面を出さない: boot のあとに迎えた repo の根も、起こす前に足す（boot と同じ手。足りていれば何も書かない）
	if err := boot.TrustFromOS(io.Discard); err != nil {
		fmt.Fprintf(stderr, "andon chief-engineer: 信頼の設定を足せない（続ける。信頼の画面が出れば答える）: %v\n", err)
	}
	labels := []string{"role=" + Profile, "project=" + slug, "issue=" + id, "title=" + title,
		"herdr.space=andon-" + slug, "herdr.tab=andon-" + slug + "-" + id}
	run, err := spawn.Run(name, labels, cwd, Profile, msg)
	if err != nil {
		undo()
		return die("%v。worktree と branch を戻した。課題 %s は todo のまま残る", err, id)
	}

	if _, err := spawn.OutputIn(cwd, append(env, "HC_TERMINAL_ID="+run.Address), "", cli, "start", id); err != nil {
		return die("%s start %s に失敗: %v（端末 %s は起きている）", cli, id, err, run.Address)
	}

	place := where(slug, id, title)
	if !run.First.Delivered() {
		fmt.Fprintf(stderr, "andon chief-engineer: %s（%s）に最初のメッセージが届いていない: %s。%s（App では %s）\n",
			run.Address, name, run.Reason(), spawn.ResendHint(run.Address, msg), place)
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

// parseArgs は --repo <r>（--repo=<r> も）・--brief -（--brief=- も。stdin のほかは受けない）と題1つを受ける。
func parseArgs(args []string) (repo, title string, brief, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case (a == "--brief" && i+1 < len(args) && args[i+1] == "-" || a == "--brief=-") && !brief:
			brief = true
			if a == "--brief" {
				i++
			}
		case a == "--repo" && i+1 < len(args) && repo == "":
			repo = args[i+1]
			i++
		case strings.HasPrefix(a, "--repo=") && repo == "":
			repo = strings.TrimPrefix(a, "--repo=")
		case strings.HasPrefix(a, "-") || title != "":
			return "", "", false, false
		default:
			title = strings.TrimSpace(a)
			if title == "" || strings.ContainsAny(a, "\r\n") {
				return "", "", false, false
			}
		}
	}
	return repo, title, brief, title != ""
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
