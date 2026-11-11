package issue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"andon/internal/boot"
	"andon/internal/chiefengineer"
	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// resumePlan は andon issue resume が起こす前に決めるもの。--dry-run はこれを出して終わる。
type resumePlan struct {
	DryRun       bool         `json:"dry_run"`
	Fresh        bool         `json:"fresh"`
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Profile      string       `json:"profile"`
	Template     string       `json:"template"`
	Terminal     string       `json:"terminal"`
	Agent        string       `json:"agent,omitempty"` // 台帳の agent（止まった端末）
	AgentState   string       `json:"agent_state"`     // none・not_found・finished・bare_shell
	Labels       []string     `json:"labels"`
	Cwd          string       `json:"cwd"`
	Repo         string       `json:"repo,omitempty"`
	Worktree     string       `json:"worktree,omitempty"`
	Branch       string       `json:"branch,omitempty"`
	Commit       string       `json:"commit,omitempty"`   // レビュアー: detached worktree を戻す commit（依頼書の commit <sha>）
	Reviewer     bool         `json:"reviewer,omitempty"` // profile *-review の子課題（cwd issues/<rev>/。ADR-0012 決めたこと 8）
	Restore      string       `json:"restore,omitempty"`  // add（git worktree add で戻す）・keep（あるので触らない）
	Session      string       `json:"session,omitempty"`
	FirstMessage string       `json:"first_message"`
	BriefPath    string       `json:"brief_path"`
	Checks       []startCheck `json:"checks"`
	repoDir      string
}

func (p *resumePlan) check(name, result, format string, a ...any) {
	p.Checks = append(p.Checks, startCheck{Name: name, Result: result, Detail: fmt.Sprintf(format, a...)})
}

// ResumeMessage は resume で起こした端末への最初のメッセージ（ADR-0012 決めたこと 3）。
func ResumeMessage(slug, id string) string {
	return fmt.Sprintf("課題 `%s/%s` の続き。セルが起き直した。worktree は同じ場所に戻した。台帳（`deck list`）と git の状態を確かめ、続きから。報告・問いは今までどおり。", slug, id)
}

// FreshMessage は --fresh で起こした端末への最初のメッセージ: start の2行に、前の担当の報告を読む1行（ADR-0012 決めたこと 7）。
func FreshMessage(slug, id, brief, report string) string {
	return spawn.FirstMessage(slug, id, brief) + fmt.Sprintf("\n前の担当の会話は続けられなかった。前の担当の報告 `%s`（あれば）を読み、worktree と台帳の今の状態から続ける。", report)
}

// resume は止まった担当・Chief Engineer を、同じ worktree・branch・cwd でハーネスのセッションの続きから起こす（ADR-0012 決めたこと 3・7・9・11・13）。
// 起こせるのは doing で、端末（台帳の agent）が無い・終わっている・素のシェルの課題だけ。worktree が無ければ戻し（あれば触らない）、
// <profile>-resume の template（Claude 系は --resume <session>、Codex 系は resume <id>）で起こす。--fresh は <profile> の template で新しいセッション。
// 記録が無い・最初のメッセージが届かないなら非 0 で止まり、台帳を変えない。届いたら台帳の agent（と session）を書く。
func (c *ctx) resume(stdout, stderr io.Writer, dry, asJSON, fresh bool) error {
	p, err := c.resumePlan(dry, fresh)
	if err != nil {
		return err
	}
	if dry {
		if asJSON {
			stdout.Write(contract.Marshal(p))
		} else {
			p.print(stdout)
		}
		return nil
	}
	id := c.id

	if p.Restore == "add" {
		git(p.repoDir, "worktree", "prune")
		mk := func(path string) error { return os.MkdirAll(filepath.Dir(path), 0o755) }
		if strings.HasPrefix(p.Worktree, spawn.WorktreeHome()+string(filepath.Separator)) {
			mk = spawn.MakeWorktreeDir
		}
		if err := mk(p.Worktree); err != nil {
			return fail("worktree の置き場を作れない: %v", err)
		}
		add, at := []string{"worktree", "add", "-q", p.Worktree, p.Branch}, "branch "+p.Branch
		if p.Reviewer {
			add, at = []string{"worktree", "add", "-q", "--detach", p.Worktree, p.Commit}, "commit "+p.Commit+"（detached）"
		}
		if _, err := git(p.repoDir, add...); err != nil {
			return fail("worktree を戻せない: %v。台帳は変えていない", err)
		}
		fmt.Fprintf(stdout, "andon issue: worktree %s を %s で戻した\n", p.Worktree, at)
	}

	if err := boot.TrustFromOS(io.Discard); err != nil {
		fmt.Fprintf(stderr, "andon issue: 信頼の設定を足せない（続ける。信頼の画面が出れば答える）: %v\n", err)
	}
	// 起こす直前にもう一度読む（lock の外で台帳が書き換えられていれば起こさない）
	now, err := c.l.Show(id)
	if err != nil {
		return err
	}
	if s, a := now.Str("status"), now.Str("agent"); s != "doing" || a != p.Agent {
		return fail("%s の台帳が変わった（status %s・agent %s）。起こしていない", id, s, a)
	}
	// hc の名前は live の端末の中で1つなので、素のシェルの端末は起こす前に片付ける（中身はシェルだけで、失うものは無い）
	if p.AgentState == "bare_shell" {
		if err := spawn.Discard(p.Agent); err != nil {
			return fail("素のシェルの端末 %s を片付けられない: %v。台帳は変えていない", p.Agent, err)
		}
	}
	run, err := spawn.Run(p.Terminal, p.Labels, p.Cwd, p.Template, p.FirstMessage)
	if err != nil {
		return fail("%v。台帳は変えていない", err)
	}
	if !run.First.Delivered() {
		why := run.Reason()
		if err := spawn.Discard(run.Address); err != nil {
			return fail("%s（%s）に最初のメッセージが届かない: %s。起こした端末を片付けられない（%v）。台帳は変えていない", run.Address, p.Terminal, why, err)
		}
		return fail("%s（%s）に最初のメッセージが届かない: %s。起こした端末は片付けた。台帳は変えていない", run.Address, p.Terminal, why)
	}
	set := []string{"set", id, "agent=" + run.Address}
	if p.Session != c.issue.Str("session") {
		set = append(set, "session="+p.Session) // 空なら消す（Codex の --fresh）
	}
	if _, err := c.l.Run("", set...); err != nil {
		return fail("kanban %s に失敗: %v（端末 %s は起きている。台帳の agent を手で直す）", strings.Join(set, " "), err, run.Address)
	}
	if p.AgentState == "finished" {
		spawn.Output(nil, "", "hc", "rm", p.Agent) // 終わった端末の残りは消してよい（失敗しても続ける）
	}
	how := "続きから"
	if fresh {
		how = "新しいセッションで"
	}
	fmt.Fprintf(stdout, "andon issue: %s を %s（%s・%s）で%s起こした。cwd %s", id, run.Address, p.Terminal, p.Template, how, p.Cwd)
	if p.Session != "" {
		fmt.Fprintf(stdout, " session %s", p.Session)
	}
	fmt.Fprintln(stdout)
	return nil
}

// resumePlan は起こす前の確かめを通し、起こすときに渡すものを決める。何も作らず、台帳も書かない。
// dry でも hc get（止まった端末の様子）と hc ls（名前）は打つ（読むだけ）。template の有無は dry では確かめない。
func (c *ctx) resumePlan(dry, fresh bool) (*resumePlan, error) {
	for _, cmd := range []string{"git", "hc"} {
		if _, err := exec.LookPath(cmd); err != nil {
			return nil, fail("%s が無い", cmd)
		}
	}
	slug, id := c.l.Slug, c.id
	p := &resumePlan{DryRun: dry, Fresh: fresh, ID: id, Title: c.issue.Str("title")}
	if !spawn.SlugRe.MatchString(slug) {
		return nil, fail("slug が不正: '%s'（台帳の root のフォルダ名。英小文字・数字・_・-）", slug)
	}
	if s := c.issue.Str("status"); s != "doing" {
		return nil, fail("%s は %s（起こし直せるのは doing だけ。todo なら andon issue start）", id, s)
	}
	p.check("status", "ok", "doing")
	if m := c.issue.Str("machine"); m != "" {
		return nil, fail("%s は machine=%s（別のマシンの端末）。andon issue resume はまだ扱わない", id, m)
	}
	p.Profile = c.issue.Str("profile")
	switch {
	case p.Profile == "":
		return nil, fail("%s に profile が無い", id)
	case !profileRe.MatchString(p.Profile):
		return nil, fail("profile が不正: '%s'", p.Profile)
	case strings.HasSuffix(p.Profile, "-review"):
		// レビュアー: 元の課題（parent）の子。detached worktree を依頼書の commit で戻し、cwd issues/<rev>/ で起こす（ADR-0012 決めたこと 8・13 の 5）
		if c.issue.Str("parent") == "" {
			return nil, fail("%s は profile %s だが parent が無い（dispatcher が起こしたレビュアーでない）", id, p.Profile)
		}
		p.Reviewer = true
	}
	p.BriefPath = filepath.Join(c.l.Dir, "issues", id, "brief.md")
	if b, err := os.ReadFile(p.BriefPath); err != nil || len(b) == 0 {
		return nil, fail("%s に依頼書（brief.md）が無い", id)
	}
	p.Terminal = strings.ToLower(slug + "-" + id)
	if !spawn.NameRe.MatchString(p.Terminal) {
		return nil, fail("端末の名前 %s が hc の決まり（a-z 0-9 - _ で32字まで）に合わない", p.Terminal)
	}

	// 止まった端末: 無い・終わっている・素のシェルだけ起こし直す。動いていれば止まる
	p.Agent, p.AgentState = c.issue.Str("agent"), "none"
	if p.Agent != "" {
		t, err := lookTerm(p.Agent)
		if err != nil {
			return nil, fail("%v", err)
		}
		if t.State == "live" {
			return nil, fail("端末 %s がもう動いている（%s。hc get %s で確かめる）", p.Agent, t.describe(), p.Agent)
		}
		p.AgentState = t.State
	}
	p.check("agent", "ok", "端末 %s は %s", or(p.Agent, "（無し）"), p.AgentState)
	live, err := liveTerms()
	if err != nil {
		return nil, err
	}
	if a, ok := live[p.Terminal]; ok && !(a == p.Agent && p.AgentState == "bare_shell") {
		return nil, fail("端末 %s（%s）がもう動いている（hc ls で確かめる）", p.Terminal, a)
	}

	// cwd と worktree
	p.Cwd = filepath.Join(c.l.Dir, "issues", id)
	if repo := c.issue.Str("repo"); repo != "" {
		repoDir, err := c.writableRepo(repo, p.check)
		if err != nil {
			return nil, err
		}
		p.Repo, p.repoDir = repo, repoDir
		if p.Reviewer {
			if err := c.reviewerRestorePlan(p); err != nil {
				return nil, err
			}
		} else {
			if err := c.restorePlan(p); err != nil {
				return nil, err
			}
			p.Cwd = p.Worktree
		}
	}

	// セッション
	brand := ledger.Brand(p.Profile)
	switch {
	case fresh:
		p.Template = p.Profile
		if brand == "claude" {
			p.Session = spawn.NewSession()
		}
		p.check("session", "ok", "--fresh: 新しいセッション（前の会話は続けない）")
	case brand == "claude":
		p.Template = p.Profile + "-resume"
		p.Session = c.issue.Str("session")
		if p.Session == "" {
			return nil, fail("%s の台帳に session が無い（セッションを記録する前に起きた課題）。続けられないので、人の go で andon issue resume --fresh %s", id, id)
		}
		if !spawn.SessionRe.MatchString(p.Session) {
			return nil, fail("%s の台帳の session が UUID でない: '%s'", id, p.Session)
		}
		rec, ok := claudeRecord(p.Session)
		if !ok {
			return nil, fail("セッション %s の記録が無い（%s）。続けられないので、人の go で andon issue resume --fresh %s", p.Session, rec, id)
		}
		p.check("session", "ok", "記録 %s", rec)
	default:
		p.Template = p.Profile + "-resume"
		s, n, err := codexRecord(p.Cwd)
		if err != nil {
			return nil, fail("Codex の記録を読めない: %v", err)
		}
		if s == "" {
			return nil, fail("cwd %s の Codex の記録が無い（%s の rollout-*.jsonl を %d 本見た）。続けられないので、人の go で andon issue resume --fresh %s", p.Cwd, codexSessions(), n, id)
		}
		p.Session = s
		p.check("session", "ok", "cwd %s の最新の記録 %s", p.Cwd, s)
	}
	if dry {
		p.check("template", "skipped", "hc templates を呼ばない（template %s があるかは確かめていない）", p.Template)
	} else if found, err := spawn.HasTemplate(p.Template); err != nil {
		return nil, fail("%v", err)
	} else if !found {
		return nil, fail("template %s が無い（andon boot が resume: の行から作る。templates.local のプロファイルに resume: が無ければ --fresh で起こす）", p.Template)
	}

	switch {
	case p.Profile == chiefengineer.Profile:
		p.Labels = chiefengineer.Labels(slug, id, p.Title)
	case p.Reviewer:
		p.Labels = ReviewerLabels(slug, id, p.Title, c.issue.Str("parent"))
	default:
		p.Labels = workLabels(slug, id, p.Title)
	}
	if p.Session != "" {
		p.Labels = append(p.Labels, spawn.SessionLabel+"="+p.Session)
	}
	if fresh {
		p.FirstMessage = FreshMessage(slug, id, p.BriefPath, filepath.Join(c.l.Dir, "issues", id, "report.md"))
	} else {
		p.FirstMessage = ResumeMessage(slug, id)
	}
	return p, nil
}

// restorePlan は台帳の worktree を戻す手を決める。パスは台帳の値をそのまま使う（組み直さない）が、
// andon issue start がこの課題に作る置き場（今の置き場か旧の置き場の <repo 名>/<slug>-<id>）でなければ止まる。
// あれば、その branch の worktree であることだけ確かめて触らない（commit していない変更を消さない）。
func (c *ctx) restorePlan(p *resumePlan) error {
	slug, id := c.l.Slug, c.id
	p.Worktree, p.Branch = c.issue.Path("worktree"), c.issue.Str("branch")
	if p.Worktree == "" || p.Branch == "" {
		return fail("%s の台帳に worktree か branch が無い（worktree=%s branch=%s）", id, p.Worktree, p.Branch)
	}
	ok := false
	for _, home := range worktreeHomes() {
		if p.Worktree == filepath.Join(home, filepath.Base(p.repoDir), slug+"-"+id) {
			ok = true
		}
	}
	if !ok {
		return fail("%s の worktree が置き場（%s）でない: %s", id, spawn.WorktreePath(p.repoDir, slug, id), p.Worktree)
	}
	if p.Branch != slug+"/"+id && !strings.HasPrefix(p.Branch, slug+"/"+id+"-") {
		return fail("%s の branch が %s/%s- で始まらない: %s", id, slug, id, p.Branch)
	}
	fi, err := os.Lstat(p.Worktree)
	switch {
	case err == nil && fi.Mode()&os.ModeSymlink != 0:
		return fail("%s の worktree が symlink: %s", id, p.Worktree)
	case err == nil:
		if !Pair(p.repoDir, p.Worktree, "branch refs/heads/"+p.Branch) {
			return fail("%s は %s の worktree（branch %s）でない。触らない", p.Worktree, p.repoDir, p.Branch)
		}
		p.Restore = "keep"
		p.check("worktree", "ok", "%s はある（触らない）", p.Worktree)
	case os.IsNotExist(err):
		if _, err := git(p.repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+p.Branch); err != nil {
			return fail("worktree %s が無く、branch %s も %s に無い（戻せない）", p.Worktree, p.Branch, p.repoDir)
		}
		p.Restore = "add"
		p.check("worktree", "ok", "%s が無い。git worktree add で branch %s から戻す", p.Worktree, p.Branch)
	default:
		return fail("worktree を読めない: %v", err)
	}
	return nil
}

// ReviewerLabels はレビュアーの端末のラベル: 自分の課題と、元の課題の担当の Space・tab（dispatcher の runReviewer が担当の端末から写すのと同じ値。
// 担当の端末は andon issue start の workLabels で andon-<slug>-<元の id>）。
func ReviewerLabels(slug, id, title, parent string) []string {
	return []string{"project=" + slug, "issue=" + id, "title=" + title,
		"herdr.space=andon-" + slug + "-" + parent, "herdr.tab=andon-" + slug + "-" + parent}
}

// reviewCommitRe は dispatcher がレビュアーの依頼書に書く対象の行（「対象: worktree `…`（detached）。commit <sha>。」）の commit。
var reviewCommitRe = regexp.MustCompile(`(?m)^対象: .*?commit ([0-9a-f]{40})。`)

// reviewerRestorePlan はレビュアーの detached worktree を戻す手を決める（ADR-0012 決めたこと 8・13 の 5）。
// 戻す commit は親の review_request.commit でなく、そのレビュアーの依頼書の commit（回がずれない）。パスは台帳の値をそのまま使うが、
// dispatcher がこの課題に作る置き場（今か旧の置き場の <repo 名>/<slug>-<rev>）でなければ止まる。
// あれば detached の worktree であることだけ確かめて触らない。cwd は issues/<rev>/（台帳の Dir の下。worktree ではない）。
func (c *ctx) reviewerRestorePlan(p *resumePlan) error {
	slug, id := c.l.Slug, c.id
	b, _ := os.ReadFile(p.BriefPath)
	m := reviewCommitRe.FindSubmatch(b)
	if m == nil {
		return fail("%s の依頼書に対象の commit（「対象: … commit <40桁の sha>。」の行）が無い", id)
	}
	p.Commit = string(m[1])
	p.Worktree = c.issue.Path("worktree")
	if p.Worktree == "" {
		return fail("%s の台帳に worktree が無い", id)
	}
	ok := false
	for _, home := range worktreeHomes() {
		if p.Worktree == filepath.Join(home, filepath.Base(p.repoDir), slug+"-"+id) {
			ok = true
		}
	}
	if !ok {
		return fail("%s の worktree が置き場（%s）でない: %s", id, spawn.WorktreePath(p.repoDir, slug, id), p.Worktree)
	}
	fi, err := os.Lstat(p.Worktree)
	switch {
	case err == nil && fi.Mode()&os.ModeSymlink != 0:
		return fail("%s の worktree が symlink: %s", id, p.Worktree)
	case err == nil:
		if !Pair(p.repoDir, p.Worktree, "detached") {
			return fail("%s は %s の detached の worktree でない。触らない", p.Worktree, p.repoDir)
		}
		p.Restore = "keep"
		p.check("worktree", "ok", "%s はある（触らない）", p.Worktree)
	case os.IsNotExist(err):
		if _, err := git(p.repoDir, "cat-file", "-e", p.Commit+"^{commit}"); err != nil {
			return fail("worktree %s が無く、commit %s も %s に無い（戻せない）", p.Worktree, p.Commit, p.repoDir)
		}
		p.Restore = "add"
		p.check("worktree", "ok", "%s が無い。git worktree add --detach で commit %s から戻す", p.Worktree, p.Commit)
	default:
		return fail("worktree を読めない: %v", err)
	}
	return nil
}

// print は --dry-run の人向けの出力。
func (p *resumePlan) print(w io.Writer) {
	fmt.Fprintf(w, "andon issue: %s を起こし直す乾いた実行（hc run を打たず、worktree・端末を作らず台帳も書かない）\n", p.ID)
	row := func(k, v string) {
		if v != "" {
			fmt.Fprintf(w, "  %s: %s\n", k, v)
		}
	}
	row("題", p.Title)
	row("プロファイル", p.Profile)
	row("template", p.Template)
	row("端末の名前", p.Terminal)
	row("止まった端末", strings.TrimSpace(p.Agent+" "+p.AgentState))
	row("cwd", p.Cwd)
	row("repo", p.Repo)
	row("worktree", strings.TrimSpace(p.Worktree+" "+map[string]string{"add": "（戻す）", "keep": "（ある）"}[p.Restore]))
	row("branch", p.Branch)
	row("commit", p.Commit)
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
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// seenTerm は hc get --json のうち、止まった端末の見分けに読むもの。State は not_found・finished・bare_shell・live。
type seenTerm struct {
	State      string            `json:"state"`
	Argv       []string          `json:"argv"`
	Labels     map[string]string `json:"labels"`
	Foreground *struct {
		Argv []string `json:"argv"`
	} `json:"foreground"`
}

func (t *seenTerm) describe() string {
	fg := "取れない"
	if t.Foreground != nil {
		fg = strings.Join(t.Foreground.Argv, " ")
	}
	return "argv " + strings.Join(t.Argv, " ") + "・前に " + fg
}

// lookTerm は hc get addr。not_found なら State not_found。live で top-level の argv も前に出ているものも引数の無いシェルなら bare_shell
// （作り直しで戻った端末。dispatcher の Gone と同じ見分け）。live でなければ finished。
func lookTerm(addr string) (*seenTerm, error) {
	cmd := exec.Command("hc", "get", addr, "--json")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		var e struct {
			Error struct {
				Kind string `json:"kind"`
			} `json:"error"`
		}
		if json.Unmarshal(bytes.TrimSpace(errb.Bytes()), &e) == nil && e.Error.Kind == "not_found" {
			return &seenTerm{State: "not_found"}, nil
		}
		return nil, fmt.Errorf("hc get %s に失敗: %v %s", addr, err, strings.TrimSpace(errb.String()))
	}
	var t seenTerm
	if err := json.Unmarshal(out, &t); err != nil || t.State == "" {
		return nil, fmt.Errorf("hc get %s の返事が読めない", addr)
	}
	switch {
	case t.State != "live":
		t.State = "finished"
	case spawn.BareShell(t.Argv) && t.Foreground != nil && spawn.BareShell(t.Foreground.Argv):
		t.State = "bare_shell"
	}
	return &t, nil
}

// liveTerms は動いている端末の名前 → address。
func liveTerms() (map[string]string, error) {
	out, err := spawn.Output(nil, "", "hc", "ls", "--json")
	if err != nil {
		return nil, fail("hc ls に失敗: %v", err)
	}
	var ls struct {
		Terminals []struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(out), &ls); err != nil {
		return nil, fail("hc ls の返事が読めない")
	}
	m := map[string]string{}
	for _, t := range ls.Terminals {
		m[t.Name] = t.Address
	}
	return m, nil
}

// claudeDir は Claude Code の設定の置き場（CLAUDE_CONFIG_DIR。無ければ ~/.claude）。
func claudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".claude")
}

// claudeRecord は Claude Code のセッションの記録（<claude>/projects/*/<id>.jsonl）があるか。中身は読まない（ADR-0012 決めたこと 11）。
// 無ければ探した形を返す。
func claudeRecord(id string) (string, bool) {
	pat := filepath.Join(claudeDir(), "projects", "*", id+".jsonl")
	if m, _ := filepath.Glob(pat); len(m) > 0 {
		return m[0], true
	}
	return pat, false
}

// codexSessions は Codex の記録の置き場（CODEX_HOME。無ければ ~/.codex）/sessions。
func codexSessions() string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home = filepath.Join(os.Getenv("HOME"), ".codex")
	}
	return filepath.Join(home, "sessions")
}

// codexRecord は Codex の記録（sessions/**/rollout-*.jsonl）の先頭行（session_meta）の cwd と id だけを読み、
// cwd が同じもののうち最新（timestamp）の id を返す（ADR-0012 決めたこと 11）。n は見た記録の数。無ければ空。
func codexRecord(cwd string) (id string, n int, err error) {
	want := map[string]bool{filepath.Clean(cwd): true}
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		want[r] = true
	}
	type hit struct{ at, id string }
	var hits []hit
	root := codexSessions()
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == root && os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		n++
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		line, _ := bufio.NewReader(f).ReadBytes('\n')
		f.Close()
		var meta struct {
			Type    string `json:"type"`
			Payload struct {
				ID        string `json:"id"`
				Cwd       string `json:"cwd"`
				Timestamp string `json:"timestamp"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &meta) != nil || meta.Type != "session_meta" || !spawn.SessionRe.MatchString(meta.Payload.ID) {
			return nil
		}
		if want[filepath.Clean(meta.Payload.Cwd)] {
			hits = append(hits, hit{meta.Payload.Timestamp, meta.Payload.ID})
		}
		return nil
	})
	if err != nil {
		return "", n, err
	}
	if len(hits) == 0 {
		return "", n, nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].at < hits[j].at })
	return hits[len(hits)-1].id, n, nil
}
