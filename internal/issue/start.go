package issue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"andon/internal/boot"
	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

const exitFirst = 3 // 起きたが、最初のメッセージが届いていない

// notSent は担当が起きたが最初のメッセージが届いていないこと（終了コード 3）。msg は人か Chief of Staff への案内。
type notSent struct{ msg string }

func (n notSent) Error() string { return n.msg }

var profileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// startPlan は andon issue start が起こす前に決めるもの（確かめを通ったあと）。--dry-run はこれを出して終わる。
type startPlan struct {
	DryRun       bool         `json:"dry_run"`
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Profile      string       `json:"profile"`
	Terminal     string       `json:"terminal"`
	Labels       []string     `json:"labels"`
	Cwd          string       `json:"cwd"`
	Repo         string       `json:"repo,omitempty"`
	Worktree     string       `json:"worktree,omitempty"`
	Branch       string       `json:"branch,omitempty"`
	Session      string       `json:"session,omitempty"`
	FirstMessage string       `json:"first_message"`
	BriefPath    string       `json:"brief_path"`
	Brief        string       `json:"brief"`
	Checks       []startCheck `json:"checks"`
	repoDir      string
}

// startCheck は起こす前の確かめ1つの結果。result は ok（通った）か skipped（--dry-run では確かめない: hc を呼ぶもの・作るもの）。
// 通らなかった確かめは startCheck にならず、本番と同じ理由1行と終了コード 1 で止まる。
type startCheck struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	Detail string `json:"detail"`
}

func (p *startPlan) check(name, result, format string, a ...any) {
	p.Checks = append(p.Checks, startCheck{Name: name, Result: result, Detail: fmt.Sprintf(format, a...)})
}

// start は台帳の課題に担当を起こす（仮設の yard task start を移したもの。docs/contract.md §8）:
// repo があれば（台帳の config.yaml の repos と、セルの repo の HEAD の project.json の repos の両方にあるときだけ）worktree と branch を切り、hc run でプロファイルを起こし、kanban start で doing にして agent をその端末にする。
// 打つのは Chief of Staff（projects/<slug>/ で）。起こせなかったら作った worktree と branch を戻す（やり直せるように）。
// 最初のメッセージが届かなかったとき（信頼の画面など）は、台帳は doing・agent はその端末のまま、送り直す hc send を案内して終了コード 3。
// dry は --dry-run: 確かめ（plan）だけを本番と同じ順・同じ理由で通し、hc を呼ばず、何も作らず台帳も書かずに、起こすときに渡すものを出す。
func (c *ctx) start(stdout, stderr io.Writer, dry, asJSON bool) error {
	p, err := c.plan(dry)
	if err != nil {
		return err
	}
	if dry {
		if asJSON {
			stdout.Write(contract.Marshal(p)) // gate --dry-run と同じ書き方（HTML の文字を逃がさない）
		} else {
			p.print(stdout)
		}
		return nil
	}

	id := c.id
	undo := func() {}
	if p.Repo != "" {
		wt, err := spawn.NewWorktree(p.repoDir, c.l.Slug, id, p.Title)
		if err != nil {
			return fail("%v", err)
		}
		if _, err := c.l.Run("", "set", id, "branch="+wt.Branch, "worktree="+wt.Path); err != nil {
			wt.Remove()
			return err
		}
		undo = func() {
			wt.Remove()
			c.l.Run("", "set", id, "branch=", "worktree=")
		}
		p.Cwd, p.Branch = wt.Path, wt.Branch
	}

	// 信頼の画面を出さない: boot のあとに迎えた repo の根も、起こす前に足す（boot と同じ手。足りていれば何も書かない）
	if err := boot.TrustFromOS(io.Discard); err != nil {
		fmt.Fprintf(stderr, "andon issue: 信頼の設定を足せない（続ける。信頼の画面が出れば答える）: %v\n", err)
	}
	run, err := spawn.Run(p.Terminal, p.Labels, p.Cwd, p.Profile, p.FirstMessage)
	if err != nil {
		undo()
		return fail("%v。作った worktree と branch を戻した。%s は todo のまま", err, id)
	}

	// 台帳を doing にし、agent をその端末にする（担当の cwd で、その端末として打つ）
	if _, err := spawn.OutputIn(p.Cwd, []string{"KANBAN_ROOT=" + c.l.Root, "HC_TERMINAL_ID=" + run.Address}, "", "kanban", "start", id); err != nil {
		return fail("kanban start %s に失敗: %v（端末 %s は起きている。kanban start は担当が打てばよい）", id, err, run.Address)
	}
	if p.Session != "" {
		if _, err := c.l.Run("", "set", id, "session="+p.Session); err != nil {
			return fail("kanban set %s session=%s に失敗: %v（端末 %s は起きている。resume するには台帳に session を手で書く）", id, p.Session, err, run.Address)
		}
	}
	place := id + " " + p.Title + " / " + id // 人に端末を指す言い方（ADR-0002 決めたこと 17・ADR-0003）
	if !run.First.Delivered() {
		return notSent{fmt.Sprintf("%s（%s）に最初のメッセージが届いていない: %s。%s（台帳は doing・agent %s のまま。App では %s）",
			run.Address, p.Terminal, run.Reason(), spawn.ResendHint(run.Address, p.FirstMessage), run.Address, place)}
	}
	fmt.Fprintf(stdout, "andon issue: %s を %s（%s・%s）で起こした。cwd %s", id, run.Address, p.Terminal, p.Profile, p.Cwd)
	if p.Branch != "" {
		fmt.Fprintf(stdout, " branch %s", p.Branch)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "andon issue: 出た場所 %s（%s）\n", place, run.Address)
	return nil
}

// plan は起こす前の確かめを通し、起こすときに渡すものを決める。何も作らず、台帳も書かない。
// dry のときは hc を呼ぶ確かめ（template・動いている端末）を飛ばし、worktree と branch は作らずに名前だけ決める
// （origin の branch・worktree の置き場の確かめは、本番で作る直前にする）。通らなければ本番と同じ理由1行で止まる。
func (c *ctx) plan(dry bool) (*startPlan, error) {
	tools := []string{"git", "hc"}
	if dry {
		tools = []string{"git"}
	}
	for _, cmd := range tools {
		if _, err := exec.LookPath(cmd); err != nil {
			return nil, fail("%s が無い", cmd)
		}
	}
	slug, id := c.l.Slug, c.id
	p := &startPlan{DryRun: dry, ID: id, Title: c.issue.Str("title")}
	if !spawn.SlugRe.MatchString(slug) {
		return nil, fail("slug が不正: '%s'（台帳の root のフォルダ名。英小文字・数字・_・-）", slug)
	}
	if err := allowed("start", id, c.issue.Str("status")); err != nil {
		return nil, err
	}
	p.check("status", "ok", "%s", c.issue.Str("status"))
	// kanban start の開始の条件（after が全部済んでいる）を、何かを作る前に確かめる
	if ok, err := c.startable(); err != nil {
		return nil, err
	} else if !ok {
		return nil, fail("%s は after（%s）が済んでいない。済んでから起こす", id, strings.Join(c.after(), ", "))
	}
	if a := c.after(); len(a) > 0 {
		p.check("after", "ok", "済んでいる（%s）", strings.Join(a, ", "))
	} else {
		p.check("after", "ok", "after なし")
	}
	if wt, br := c.issue.Str("worktree"), c.issue.Str("branch"); wt != "" || br != "" {
		return nil, fail("%s は起こし済み（worktree=%s branch=%s）。やり直すなら先に andon issue finish", id, wt, br)
	}
	if m := c.issue.Str("machine"); m != "" {
		// 別のマシンで起こす（hc run --on）はまだ扱わない。黙って指揮側で起こさない
		return nil, fail("%s は machine=%s（別のマシンで起こす）。andon issue start はまだ扱わない。machine を消すか、人が起こす", id, m)
	}
	p.Profile = c.issue.Str("profile")
	if p.Profile == "" {
		return nil, fail("%s に profile が無い（kanban set %s profile=<名前>）", id, id)
	}
	if !profileRe.MatchString(p.Profile) {
		return nil, fail("profile が不正: '%s'", p.Profile)
	}
	p.BriefPath = filepath.Join(c.l.Dir, "issues", id, "brief.md")
	brief, err := os.ReadFile(p.BriefPath)
	if err != nil || len(brief) == 0 {
		return nil, fail("%s に依頼書（brief.md）が無い", id)
	}
	p.Brief = string(brief)
	p.check("brief", "ok", "%s（%d バイト）", p.BriefPath, len(brief))
	p.Terminal = strings.ToLower(slug + "-" + id)
	if dry {
		p.check("template", "skipped", "hc templates を呼ばない（プロファイル %s の template があるかは確かめていない）", p.Profile)
	} else {
		found, err := spawn.HasTemplate(p.Profile)
		if err != nil {
			return nil, fail("%v", err)
		}
		if !found {
			return nil, fail("プロファイル %s の template が無い（hc templates で確かめる。作るのは人）", p.Profile)
		}
	}
	if !spawn.NameRe.MatchString(p.Terminal) {
		return nil, fail("端末の名前 %s が hc の決まり（a-z 0-9 - _ で32字まで）に合わない", p.Terminal)
	}
	if dry {
		p.check("terminal", "skipped", "hc ls を呼ばない（端末 %s が動いていないかは確かめていない）", p.Terminal)
	} else if live, err := liveNames(); err != nil {
		return nil, err
	} else if live[p.Terminal] {
		return nil, fail("端末 %s がもう動いている（hc ls で確かめる）", p.Terminal)
	}

	p.Cwd = filepath.Join(c.l.Dir, "issues", id)
	if repo := c.issue.Str("repo"); repo != "" {
		repoDir, err := c.writableRepo(repo, p.check)
		if err != nil {
			return nil, err
		}
		p.Repo, p.repoDir = repo, repoDir
		p.Worktree, p.Branch = spawn.WorktreePath(repoDir, slug, id), spawn.BranchName(slug, id, p.Title)
		if dry {
			p.Cwd = p.Worktree
			p.check("worktree", "skipped", "worktree と branch は作らない（origin の branch・置き場が空いているかは確かめていない）")
		}
	}
	p.Labels = workLabels(slug, id, p.Title)
	// Claude 系はセッションの id を Andon が決めて渡し（template の command が端末のラベルから読む）、台帳の session に書く。
	// Codex 系は id を決める口が無いので、resume のときに記録から引く（ADR-0012 決めたこと 11）
	if ledger.Brand(p.Profile) == "claude" {
		p.Session = spawn.NewSession()
		p.Labels = append(p.Labels, spawn.SessionLabel+"="+p.Session)
	}
	p.FirstMessage = spawn.FirstMessage(slug, id, p.BriefPath) + spawn.Guide()
	return p, nil
}

// writableRepo は課題の repo を書いてよいかを確かめ、clone の置き場を返す（start と resume の確かめ）:
// 形・台帳の config.yaml の repos・セルの repo の HEAD の project.json の repos（人だけが書く。author 照合）・clone があること。
func (c *ctx) writableRepo(repo string, check func(name, result, format string, a ...any)) (string, error) {
	slug := c.l.Slug
	if !spawn.RepoRe.MatchString(repo) {
		return "", fail("repo が不正: '%s'（<host>/<owner>/<repo>）", repo)
	}
	if !slices.Contains(c.l.Repos(), repo) {
		return "", fail("repo %s が台帳の config.yaml の repos に無い（このプロジェクトの repo でない）", repo)
	}
	check("repos", "ok", "%s は台帳の config.yaml の repos にある", repo)
	// 書いてよい repo の正はセルの repo の HEAD の project.json（人だけが書く。author 照合）。読めなければ repo 無し（安全側）
	ws := delegate.Workspace()
	proj, err := delegate.ReadProject(ws, slug)
	if errors.Is(err, delegate.ErrAbsent) {
		return "", fail("repo %s を書いてよいか分からない: セルの repo の HEAD に %s が無い（人が書いて commit する）", repo, delegate.ProjectRel(slug))
	} else if err != nil {
		return "", fail("repo %s を書いてよいか分からない: %v", repo, err)
	}
	if !slices.Contains(proj.Repos, repo) {
		return "", fail("repo %s が %s の repos に無い（このプロジェクトが書いてよい repo でない）", repo, delegate.ProjectRel(slug))
	}
	check("delegation", "ok", "%s は %s の repos にある（書いてよい）", repo, delegate.ProjectRel(slug))
	repoDir, err := spawn.RepoPath(repo)
	if err != nil {
		return "", fail("%v", err)
	}
	if _, err := git(repoDir, "rev-parse", "--git-dir"); err != nil {
		return "", fail("リポジトリが無い: %s（ghq get する）", repoDir)
	}
	return repoDir, nil
}

// print は --dry-run の人向けの出力。
func (p *startPlan) print(w io.Writer) {
	fmt.Fprintf(w, "andon issue: %s を起こす乾いた実行（hc を呼ばず、worktree・branch・端末・ラベルを作らず、台帳も書かない）\n", p.ID)
	row := func(k, v string) {
		if v != "" {
			fmt.Fprintf(w, "  %s: %s\n", k, v)
		}
	}
	row("題", p.Title)
	row("プロファイル", p.Profile)
	row("端末の名前", p.Terminal)
	row("cwd", p.Cwd)
	row("repo", p.Repo)
	row("worktree", p.Worktree)
	row("branch", p.Branch)
	row("session", p.Session)
	fmt.Fprintln(w, "ラベル:")
	for _, l := range p.Labels {
		fmt.Fprintf(w, "  %s\n", l)
	}
	fmt.Fprintln(w, "起こす前の確かめ:")
	for _, ch := range p.Checks {
		fmt.Fprintf(w, "  %-7s %-10s %s\n", ch.Result, ch.Name, ch.Detail)
	}
	fmt.Fprintln(w, "最初のメッセージ（hc run に渡す文）:")
	fmt.Fprintln(w, p.FirstMessage)
	fmt.Fprintf(w, "依頼書（%s）:\n", p.BriefPath)
	fmt.Fprint(w, p.Brief)
	if !strings.HasSuffix(p.Brief, "\n") {
		fmt.Fprintln(w)
	}
}

// workLabels は担当の端末のラベル。名前は a-z 0-9 - _ の32字までなので、人に見せる題はラベル title に載せる。
// herdr.space・herdr.tab で人の Herdr App に課題ごとの Space（tab は担当の1つ）として出す（ADR-0003。id の作り方は ADR-0002 決めたこと 6）。
// 値は slug・id・題だけで決まり、どのマシンで起こすかに依らない（別のマシンならその machine の Space に出る。前提 7）。
func workLabels(slug, id, title string) []string {
	return []string{"project=" + slug, "issue=" + id, "title=" + title,
		"herdr.space=andon-" + slug + "-" + id, "herdr.tab=andon-" + slug + "-" + id}
}

// liveNames は動いている端末の名前（hc の名前は動いているものの中で1つ）。
func liveNames() (map[string]bool, error) {
	out, err := spawn.Output(nil, "", "hc", "ls", "--json")
	if err != nil {
		return nil, fail("hc ls に失敗: %v", err)
	}
	var ls struct {
		Terminals []struct {
			Name string `json:"name"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(out), &ls); err != nil {
		return nil, fail("hc ls の返事が読めない")
	}
	names := map[string]bool{}
	for _, t := range ls.Terminals {
		names[t.Name] = true
	}
	return names, nil
}

// startable は台帳の CLI の ready（todo か queued で、after が全部済んでいる）にこの課題があるか。
func (c *ctx) startable() (bool, error) {
	out, err := c.l.Run("", "ready", "--json")
	if err != nil {
		return false, err
	}
	var ts []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &ts); err != nil {
		return false, fail("kanban ready の返事が読めない")
	}
	for _, t := range ts {
		if t.ID == c.id {
			return true, nil
		}
	}
	return false, nil
}

// after は issue.yaml の after（並び）。
func (c *ctx) after() []string {
	var out []string
	switch v := c.issue["after"].(type) {
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		out = append(out, v)
	}
	return out
}
