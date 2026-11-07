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
	"andon/internal/delegate"
	"andon/internal/spawn"
)

const exitFirst = 3 // 起きたが、最初のメッセージが届いていない

// notSent は担当が起きたが最初のメッセージが届いていないこと（終了コード 3）。msg は人か Chief of Staff への案内。
type notSent struct{ msg string }

func (n notSent) Error() string { return n.msg }

var profileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// start は台帳の課題に担当を起こす（仮設の yard task start を移したもの。docs/contract.md §8）:
// repo があれば（台帳の config.yaml の repos と、セルの repo の HEAD の project.json の repos の両方にあるときだけ）worktree と branch を切り、hc run でプロファイルを起こし、kanban start で doing にして agent をその端末にする。
// 打つのは Chief of Staff（projects/<slug>/ で）。起こせなかったら作った worktree と branch を戻す（やり直せるように）。
// 最初のメッセージが届かなかったとき（信頼の画面など）は、台帳は doing・agent はその端末のまま、送り直す hc send を案内して終了コード 3。
func (c *ctx) start(stdout, stderr io.Writer) error {
	for _, cmd := range []string{"git", "hc"} {
		if _, err := exec.LookPath(cmd); err != nil {
			return fail("%s が無い", cmd)
		}
	}
	slug, id := c.l.Slug, c.id
	if !spawn.SlugRe.MatchString(slug) {
		return fail("slug が不正: '%s'（台帳の root のフォルダ名。英小文字・数字・_・-）", slug)
	}
	if s := c.issue.Str("status"); s != "todo" {
		return fail("%s は %s（起こせるのは todo だけ）", id, s)
	}
	// kanban start の開始の条件（after が全部済んでいる）を、何かを作る前に確かめる
	if ok, err := c.startable(); err != nil {
		return err
	} else if !ok {
		return fail("%s は after（%s）が済んでいない。済んでから起こす", id, strings.Join(c.after(), ", "))
	}
	if wt, br := c.issue.Str("worktree"), c.issue.Str("branch"); wt != "" || br != "" {
		return fail("%s は起こし済み（worktree=%s branch=%s）。やり直すなら先に andon issue finish", id, wt, br)
	}
	if m := c.issue.Str("machine"); m != "" {
		// 別のマシンで起こす（hc run --on）はまだ扱わない。黙って指揮側で起こさない
		return fail("%s は machine=%s（別のマシンで起こす）。andon issue start はまだ扱わない。machine を消すか、人が起こす", id, m)
	}
	profile := c.issue.Str("profile")
	if profile == "" {
		return fail("%s に profile が無い（kanban set %s profile=<名前>）", id, id)
	}
	if !profileRe.MatchString(profile) {
		return fail("profile が不正: '%s'", profile)
	}
	brief := filepath.Join(c.l.Dir, "issues", id, "brief.md")
	if fi, err := os.Stat(brief); err != nil || fi.Size() == 0 {
		return fail("%s に依頼書（brief.md）が無い", id)
	}
	found, err := spawn.HasTemplate(profile)
	if err != nil {
		return fail("%v", err)
	}
	if !found {
		return fail("プロファイル %s の template が無い（hc templates で確かめる。作るのは人）", profile)
	}
	name := strings.ToLower(slug + "-" + id)
	if !spawn.NameRe.MatchString(name) {
		return fail("端末の名前 %s が hc の決まり（a-z 0-9 - _ で32字まで）に合わない", name)
	}
	if live, err := liveNames(); err != nil {
		return err
	} else if live[name] {
		return fail("端末 %s がもう動いている（hc ls で確かめる）", name)
	}

	title := c.issue.Str("title")
	cwd := filepath.Join(c.l.Dir, "issues", id)
	undo := func() {}
	branch := ""
	if repo := c.issue.Str("repo"); repo != "" {
		if !spawn.RepoRe.MatchString(repo) {
			return fail("repo が不正: '%s'（<host>/<owner>/<repo>）", repo)
		}
		if !slices.Contains(c.l.Repos(), repo) {
			return fail("repo %s が台帳の config.yaml の repos に無い（このプロジェクトの repo でない）", repo)
		}
		// 書いてよい repo の正はセルの repo の HEAD の project.json（人だけが書く。author 照合）。読めなければ repo 無し（安全側）
		ws := delegate.Workspace()
		proj, err := delegate.ReadProject(ws, slug)
		if errors.Is(err, delegate.ErrAbsent) {
			return fail("repo %s を書いてよいか分からない: セルの repo の HEAD に %s が無い（人が書いて commit する）", repo, delegate.ProjectRel(slug))
		} else if err != nil {
			return fail("repo %s を書いてよいか分からない: %v", repo, err)
		}
		if !slices.Contains(proj.Repos, repo) {
			return fail("repo %s が %s の repos に無い（このプロジェクトが書いてよい repo でない）", repo, delegate.ProjectRel(slug))
		}
		repoDir, err := spawn.RepoPath(repo)
		if err != nil {
			return fail("%v", err)
		}
		if _, err := git(repoDir, "rev-parse", "--git-dir"); err != nil {
			return fail("リポジトリが無い: %s（ghq get する）", repoDir)
		}
		wt, err := spawn.NewWorktree(repoDir, slug, id, title)
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
		cwd, branch = wt.Path, wt.Branch
	}

	// 信頼の画面を出さない: boot のあとに迎えた repo の根も、起こす前に足す（boot と同じ手。足りていれば何も書かない）
	if err := boot.TrustFromOS(io.Discard); err != nil {
		fmt.Fprintf(stderr, "andon issue: 信頼の設定を足せない（続ける。信頼の画面が出れば答える）: %v\n", err)
	}
	msg := spawn.FirstMessage(slug, id, brief)
	run, err := spawn.Run(name, workLabels(slug, id, title), cwd, profile, msg)
	if err != nil {
		undo()
		return fail("%v。作った worktree と branch を戻した。%s は todo のまま", err, id)
	}

	// 台帳を doing にし、agent をその端末にする（担当の cwd で、その端末として打つ）
	if _, err := spawn.OutputIn(cwd, []string{"KANBAN_ROOT=" + c.l.Root, "HC_TERMINAL_ID=" + run.Address}, "", "kanban", "start", id); err != nil {
		return fail("kanban start %s に失敗: %v（端末 %s は起きている。kanban start は担当が打てばよい）", id, err, run.Address)
	}
	place := id + " " + title + " / " + id // 人に端末を指す言い方（ADR-0002 決めたこと 17・ADR-0003）
	if !run.First.Delivered() {
		return notSent{fmt.Sprintf("%s（%s）に最初のメッセージが届いていない: %s。%s（台帳は doing・agent %s のまま。App では %s）",
			run.Address, name, run.Reason(), spawn.ResendHint(run.Address, msg), run.Address, place)}
	}
	fmt.Fprintf(stdout, "andon issue: %s を %s（%s・%s）で起こした。cwd %s", id, run.Address, name, profile, cwd)
	if branch != "" {
		fmt.Fprintf(stdout, " branch %s", branch)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "andon issue: 出た場所 %s（%s）\n", place, run.Address)
	return nil
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
