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
	"andon/internal/chiefengineer"
	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/launch"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

const exitFirst = 3 // 起きたが、最初のメッセージが届いていない

// notSent は担当が起きたが最初のメッセージが届いていないこと（終了コード 3）。msg はオーナーか Chief of Staff への案内。
type notSent struct{ msg string }

func (n notSent) Error() string { return n.msg }

var profileRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// startPlan は andon issue start が起動する前に決めるもの（確かめを通ったあと）。--dry-run はこれを出して終わる。
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
	Fill         []string     `json:"fill,omitempty"` // 起動の前に書く予約キー（ルーチンが足した課題の無いキー。checkSettings）
	repoDir      string
	command      *string // template の command（起動の記録の起動行。dry では読まない）
}

// startCheck は起動する前の確かめ1つの結果。result は ok（通った）か skipped（--dry-run では確かめない: hc を呼ぶもの・作るもの）。
// 通らなかった確かめは startCheck にならず、本番と同じ理由1行と終了コード 1 で止まる。
type startCheck struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	Detail string `json:"detail"`
}

func (p *startPlan) check(name, result, format string, a ...any) {
	p.Checks = append(p.Checks, startCheck{Name: name, Result: result, Detail: fmt.Sprintf(format, a...)})
}

// start は kanban の課題に担当を起動する（仮設の yard task start を移したもの。docs/contract.md §8）:
// repo があれば（kanban の config.yaml の repos と、セルの repo の HEAD の project.json の repos の両方にあるときだけ）worktree と branch を切り、hc run でプロファイルを起動し、`kanban start` で doing にして agent をその端末にする。
// 打つのは Chief of Staff（projects/<slug>/ で）。起動できなかったら作った worktree と branch を戻す（やり直せるように）。
// 最初のメッセージが届かなかったとき（信頼の画面など）は、kanban は doing・agent はその端末のまま、送り直す hc send を案内して終了コード 3。
// dry は --dry-run: 確かめ（plan）だけを本番と同じ順・同じ理由で通し、hc を呼ばず、何も作らず kanban も書かずに、起動するときに渡すものを出す。
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
	if len(p.Fill) > 0 {
		// ルーチンが足した課題の無い予約キーを、Chief of Staff の端末で書く（gate と dispatcher の履歴の照合も通る）
		if _, err := c.l.Run("", append([]string{"set", id}, p.Fill...)...); err != nil {
			return err
		}
	}
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
			c.l.Run("", "set", id, "branch=", "worktree=") //nolint:errcheck // 戻しは試すだけで、先の失敗を返す
		}
		p.Cwd, p.Branch, p.Worktree = wt.Path, wt.Branch, wt.Path
	}

	// 信頼の画面を出さない: boot のあとに追加した repo の根も、起動する前に足す（boot と同じ手。足りていれば何も書かない）
	if err := boot.TrustFromOS(io.Discard); err != nil {
		fmt.Fprintf(stderr, "andon issue: 信頼の設定を足せない（続ける。信頼の画面が出れば答える）: %v\n", err)
	}
	rec := launch.Gather(launch.Spec{Role: roleOf(p.Profile), Launch: launch.LaunchStart, Issue: id, Profile: p.Profile, Template: p.Profile,
		Command: p.command, Session: p.Session, Repo: p.Repo, RepoPath: p.Worktree, Brand: ledger.Brand(p.Profile)})
	run, err := spawn.Run(p.Terminal, p.Labels, p.Cwd, p.Profile, p.FirstMessage)
	if err != nil {
		undo()
		return fail("%v。作った worktree と branch を戻した。%s は todo のまま", err, id)
	}
	c.appendLaunch(stderr, rec, run.Address)

	// kanban を doing にし、agent をその端末にする（担当の cwd で、その端末として打つ）
	if _, err := spawn.OutputIn(p.Cwd, []string{"KANBAN_ROOT=" + c.l.Root, "HC_TERMINAL_ID=" + run.Address}, "", "kanban", "start", id); err != nil {
		return fail("kanban start %s に失敗: %v（端末 %s は起きている。kanban start は担当が打てばよい）", id, err, run.Address)
	}
	if p.Session != "" {
		if _, err := c.l.Run("", "set", id, "session="+p.Session); err != nil {
			return fail("kanban set %s session=%s に失敗: %v（端末 %s は起きている。resume するには kanban に session を手で書く）", id, p.Session, err, run.Address)
		}
	}
	place := id + " " + p.Title + " / " + id // オーナーに端末を指す言い方（ADR-0002 決めたこと 17・ADR-0003）
	if !run.First.Delivered() {
		// 手で送る行（ResendHint の最後）は shell にそのまま貼れるよう、文の最後に置く
		return notSent{fmt.Sprintf("%s（%s）に最初のメッセージが届いていない: %s。kanban は doing・agent %s のまま（App では %s）。%s",
			run.Address, p.Terminal, run.Reason(), run.Address, place, spawn.ResendHint(run.Address, p.Profile, p.FirstMessage))}
	}
	fmt.Fprintf(stdout, "andon issue: %s を %s（%s・%s）で起動した。cwd %s", id, run.Address, p.Terminal, p.Profile, p.Cwd)
	if p.Branch != "" {
		fmt.Fprintf(stdout, " branch %s", p.Branch)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "andon issue: 出た場所 %s（%s）\n", place, run.Address)
	return nil
}

// plan は起動する前の確かめを通し、起動するときに渡すものを決める。何も作らず、kanban も書かない。
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
		return nil, fail("slug が不正: '%s'（kanban の root のフォルダ名。英小文字・数字・_・-）", slug)
	}
	if err := allowed("start", id, c.issue.Str("status")); err != nil {
		return nil, err
	}
	p.check("status", "ok", "%s", c.issue.Str("status"))
	// kanban start の開始の条件（after が全部済んでいる）を、何かを作る前に確かめる
	if ok, err := c.startable(); err != nil {
		return nil, err
	} else if !ok {
		return nil, fail("%s は after（%s）が済んでいない。済んでから起動する", id, strings.Join(c.after(), ", "))
	}
	if a := c.after(); len(a) > 0 {
		p.check("after", "ok", "済んでいる（%s）", strings.Join(a, ", "))
	} else {
		p.check("after", "ok", "after なし")
	}
	if wt, br := c.issue.Str("worktree"), c.issue.Str("branch"); wt != "" || br != "" {
		return nil, fail("%s は起動済み（worktree=%s branch=%s）。やり直すなら先に andon issue finish", id, wt, br)
	}
	if m := c.issue.Str("machine"); m != "" {
		// 別のマシンで起動する（hc run --on）はまだ扱わない。黙って指揮側で起動しない
		return nil, fail("%s は machine=%s（別のマシンで起動する）。andon issue start はまだ扱わない。machine を消すか、オーナーが起動する", id, m)
	}
	p.Profile = c.issue.Str("profile")
	if p.Profile == "" {
		return nil, fail("%s に profile が無い（kanban set %s profile=<名前>）", id, id)
	}
	if !profileRe.MatchString(p.Profile) {
		return nil, fail("profile が不正: '%s'", p.Profile)
	}
	if parent := c.issue.Str("parent"); parent != "" && strings.HasSuffix(p.Profile, "-review") {
		// レビュアーの子課題を起動するのは dispatcher だけ（元の課題の Space に置き、レビューする commit の worktree を切る）
		return nil, fail("%s は %s のレビュアーの課題（profile %s）。起動するのは dispatcher だけで、andon issue start は打たない", id, parent, p.Profile)
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
		cmd, found, err := spawn.TemplateCommand(p.Profile)
		if err != nil {
			return nil, fail("%v", err)
		}
		p.command = &cmd
		if !found {
			return nil, fail("プロファイル %s の template が無い（hc templates で確かめる。作るのはオーナー）", p.Profile)
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
	if err := c.checkSettings(p); err != nil {
		return nil, err
	}
	p.Labels = workLabels(slug, id, p.Title)
	// Claude 系はセッションの id を Andon が決めて渡し（template の command が端末のラベルから読む）、kanban の session に書く。
	// Codex 系は id を決める口が無いので、resume のときに記録から引く（ADR-0012 決めたこと 11）
	if ledger.Brand(p.Profile) == "claude" {
		p.Session = spawn.NewSession()
		p.Labels = append(p.Labels, spawn.SessionLabel+"="+p.Session)
	}
	p.FirstMessage = spawn.FirstMessage(slug, id, p.BriefPath) + spawn.Guide()
	return p, nil
}

// settingChoices は予約キーの値の選び方（断るときに出す kanban set の行で、無いキーの値の場所に置く。値は Chief of Staff が選ぶ）。
// settingValue は SettingsOf がそのキーを読んだ値（知らない値は空か既定になり、書いてある値と合わない）。
var (
	settingChoices = map[string]string{"review": "<on|off>", "must_fix": "<critical|major|minor|trivial>", "reviewer": "<codex-review|claude-review>",
		"review_rounds": "<1 以上の数>", "merge": "<auto|person>", "estimate": "<small|medium|large>"}
	settingValue = map[string]func(ledger.Settings) string{
		"review": func(s ledger.Settings) string { return s.Review }, "must_fix": func(s ledger.Settings) string { return s.MustFix },
		"reviewer": func(s ledger.Settings) string { return s.Reviewer }, "review_rounds": func(s ledger.Settings) string { return s.ReviewRounds },
		"merge": func(s ledger.Settings) string { return s.Merge }, "estimate": func(s ledger.Settings) string { return s.Estimate }}
)

// requiredSettings は、課題を起動する前に確かめる予約キー（contract.md §2）を、確かめる順に返す。
// review と estimate はどの担当の課題にも要る。must_fix と merge は gate とマージが読むので、PR を出す課題
// （repo があって deliverable が candidates でない。dispatcher の gated と同じ）だけ。reviewer と review_rounds は
// レビュアーを起動するときに読むので、そのうえ review が off でないときだけ要る。review が off でも差分がレベル B・C なら
// dispatcher はレビュアーを起動し、値のある reviewer・review_rounds には照合を求める（dispatcher の trusted）ので、
// 値があれば確かめる（optional。無ければ求めない）。
func requiredSettings(t ledger.Issue) (keys []string, optional map[string]bool) {
	keys = []string{"review"}
	if t.Str("repo") != "" && ledger.SettingsOf(t).Deliverable != "candidates" {
		keys = append(keys, "must_fix", "reviewer", "review_rounds", "merge")
		if t.Str("review") == "off" {
			optional = map[string]bool{"reviewer": true, "review_rounds": true}
		}
	}
	return append(keys, "estimate"), optional
}

// checkSettings は予約キーが揃っているかを確かめる: 要るキー（requiredSettings）が全部あって知っている値で、
// 最後に書いたのが今の Chief of Staff の端末か（ledger.Trusted。gate・dispatcher と同じ履歴の照合）。
// 揃っていなければ起動しない: 担当は予約キーを書けず、実装を終えてから review-request・ready・gate で止まるため。
// 値は決めずに書かない。直す kanban set の行を出し、無いキーの値は選ぶ場所を残す（値を決めるのは Chief of Staff）。
// ただしルーチンが足した課題（ledger.FiredBy）は、Chief of Staff の端末で打つときだけ、無いキーを既定（ledger.DefaultSettings。
// review は off）で埋める: 足すのは dispatcher の kanban tick で、Chief of Staff の書いた値を持てないため。埋めるキーは p.Fill に置き、
// start が起動の前に kanban set で書く（--dry-run は書かない）。書いてあるキーは変えない。
// Chief Engineer の課題は見ない（予約キーを置かない）。レビュアーの課題は plan がこの前に断る。
func (c *ctx) checkSettings(p *startPlan) error {
	if p.Profile == chiefengineer.Profile {
		p.check("settings", "ok", "Chief Engineer の課題は予約キーを見ない")
		return nil
	}
	rows, s, cos := c.l.Log(), ledger.SettingsOf(c.issue), c.l.ChiefOfStaff()
	defaults := map[string]string{}
	routine := ledger.FiredBy(rows, c.issue)
	// 要るキーは埋めたあとの課題で数える（review を off で埋めれば reviewer・review_rounds は要らない）
	filled := c.issue
	if routine != "" && cos != "" && os.Getenv("HC_TERMINAL_ID") == cos {
		review := "off" // Chief of Staff が review=on を書いていれば、reviewer はいまの profile から選ぶ
		if c.issue.Str("review") == "on" {
			review = "on"
		}
		filled = ledger.Issue{}
		for k, v := range c.issue {
			filled[k] = v
		}
		for _, kv := range ledger.DefaultSettings(p.Profile, review) {
			k, v, _ := strings.Cut(kv, "=")
			defaults[k] = v
			if c.issue.Str(k) == "" {
				filled[k] = v
			}
		}
	}
	keys, optional := requiredSettings(filled)
	var need, missing, untrusted, args []string
	for _, k := range keys {
		v := c.issue.Str(k)
		if v == "" && optional[k] {
			continue
		}
		need = append(need, k)
		switch {
		case v == "" && defaults[k] != "":
			p.Fill = append(p.Fill, k+"="+defaults[k])
		case v == "" || settingValue[k](s) != v:
			missing = append(missing, k)
			args = append(args, "'"+k+"="+settingChoices[k]+"'")
		case !c.l.Trusted(rows, c.issue, k):
			untrusted = append(untrusted, k)
			args = append(args, k+"="+v)
		}
	}
	if len(args) == 0 {
		if len(p.Fill) > 0 {
			p.check("settings", "ok", "揃っている（%s）。ルーチン %s が足した課題なので、無いキーを既定で書く: %s", strings.Join(need, "・"), routine, strings.Join(p.Fill, " "))
			return nil
		}
		p.check("settings", "ok", "揃っている（%s。最後に書いたのは Chief of Staff の端末 %s）", strings.Join(need, "・"), cos)
		return nil
	}
	msg := c.id + " の予約キーが揃っていないので起動しない（担当は予約キーを書けず、実装を終えてから review-request・ready・gate で止まる）。"
	if len(missing) > 0 {
		msg += "無い・知らない値: " + strings.Join(missing, "・") + "。"
	}
	if len(untrusted) > 0 {
		msg += fmt.Sprintf("最後に書いたのが今の Chief of Staff の端末（%s）でない: %s。", cos, strings.Join(untrusted, "・"))
	}
	msg += "Chief of Staff の端末で打つ"
	if len(missing) > 0 {
		msg += "（<…> は brief.md の表から値を選んで埋める）"
	}
	// 打つ行は shell にそのまま貼れるよう、文の最後に置く
	return fail("%s: kanban set %s %s", msg, c.id, strings.Join(args, " "))
}

// writableRepo は課題の repo を書いてよいかを確かめ、clone の置き場を返す（start と resume の確かめ）:
// 形・kanban の config.yaml の repos・セルの repo の HEAD の project.json の repos（オーナーだけが書く。author 照合）・clone があること。
func (c *ctx) writableRepo(repo string, check func(name, result, format string, a ...any)) (string, error) {
	slug := c.l.Slug
	if !spawn.RepoRe.MatchString(repo) {
		return "", fail("repo が不正: '%s'（<host>/<owner>/<repo>）", repo)
	}
	if !slices.Contains(c.l.Repos(), repo) {
		return "", fail("repo %s が kanban の config.yaml の repos に無い（このプロジェクトの repo でない）", repo)
	}
	check("repos", "ok", "%s は kanban の config.yaml の repos にある", repo)
	// 書いてよい repo の正はセルの repo の HEAD の project.json（オーナーだけが書く。author 照合）。読めなければ repo 無し（安全側）
	ws := delegate.Workspace()
	proj, err := delegate.ReadProject(ws, slug)
	if errors.Is(err, delegate.ErrAbsent) {
		return "", fail("repo %s を書いてよいか分からない: セルの repo の HEAD に %s が無い（オーナーが書いて commit する）", repo, delegate.ProjectRel(slug))
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
	fmt.Fprintf(w, "andon issue: %s を起動する乾いた実行（hc を呼ばず、worktree・branch・端末・ラベルを作らず、kanban も書かない）\n", p.ID)
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
	fmt.Fprintln(w, "起動する前の確かめ:")
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

// workLabels は担当の端末のラベル。名前は a-z 0-9 - _ の32字までなので、オーナーに見せる題はラベル title に載せる。
// herdr.space・herdr.tab でオーナーの Herdr App に課題ごとの Space（tab は担当の1つ）として出す（ADR-0003。id の作り方は ADR-0002 決めたこと 6）。
// 値は slug・id・題だけで決まり、どのマシンで起動するかに依らない（別のマシンならその machine の Space に出る。前提 7）。
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

// startable は kanban の CLI の ready（todo か queued で、after が全部済んでいる）にこの課題があるか。
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

// roleOf はプロファイルから起動の記録の役を決める（chief-engineer は Chief Engineer、*-review はレビュアー、ほかは担当）。
func roleOf(profile string) string {
	switch {
	case profile == chiefengineer.Profile:
		return launch.RoleChiefEngineer
	case strings.HasSuffix(profile, "-review"):
		return launch.RoleReviewer
	}
	return launch.RoleWorker
}

// appendLaunch は起動できた端末 addr の起動の記録（contract.md §18）を課題の library/launches.jsonl に1行足す。
// 書けなくても起動は戻さない（端末はもう起きている）。stderr に1行出す。
func (c *ctx) appendLaunch(stderr io.Writer, rec launch.Record, addr string) {
	rec.Terminal = addr
	if err := launch.Append(launch.IssuePath(c.l.Dir, c.id), rec); err != nil {
		fmt.Fprintf(stderr, "andon issue: 起動の記録を書けない（起動は続ける）: %v\n", err)
	}
}
