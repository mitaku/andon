// Package issue は担当が打つ andon issue …（docs/contract.md §4・§5）。機械向けの約束（report.json・問いの JSON）を
// 書いて検査し、kanban の CLI（report・review・ask）を打つ。担当は JSON を手で書かず、一致の確認も command がする。
//
// 定義（process/states/issue.yaml）に無い移り方（今の status で打てないサブコマンド）は断る（transition.go）。
// 合わなければ終了コード 1 と理由1行。使い方の誤りは 2。start で最初のメッセージが届かなければ 3。impact で PR がまだ MERGED でなければ 4。
package issue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

const (
	exitFail  = 1
	exitUsage = 2
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func fail(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

type ctx struct {
	l     *ledger.Ledger
	id    string
	issue ledger.Issue
}

func withIssue(id string, f func(*ctx) error) error {
	l, err := ledger.Open()
	if err != nil {
		return err
	}
	t, err := l.Show(id)
	if err != nil {
		return err
	}
	return f(&ctx{l: l, id: id, issue: t})
}

func (c *ctx) reportPath() string { return filepath.Join(c.l.Library(c.id), "report.json") }

func (c *ctx) candidatesPath() string { return filepath.Join(c.l.Library(c.id), "candidates.json") }

func (c *ctx) hasRepo() bool { return c.issue.Str("repo") != "" }

func readInput(stdin io.Reader, src string) ([]byte, error) {
	if src == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(src)
}

func (c *ctx) head() (string, error) {
	dir := c.issue.Path("worktree")
	args := []string{"rev-parse", "HEAD"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	sha := strings.TrimSpace(string(out))
	if err != nil || !contract.IsSHA(sha) {
		return "", fail("HEAD が読めない（worktree: %s）", dir)
	}
	return sha, nil
}

func prHead(pr string) (string, error) {
	var errb bytes.Buffer
	c := exec.Command("gh", "pr", "view", pr, "--json", "headRefOid", "-q", ".headRefOid")
	c.Stderr = &errb
	out, err := c.Output()
	sha := strings.TrimSpace(string(out))
	if err != nil || !contract.IsSHA(sha) {
		return "", fail("PR の head が読めない: %s %s", pr, strings.TrimSpace(errb.String()))
	}
	return sha, nil
}

// prHeadTries と prHeadWait は、PR の head が HEAD に追いつくのを待つ回数と間隔（検査は短くする）。
// push の直後は GitHub の PR の head が古いまま十数秒残ることがある。上限は (回数-1)×間隔 の 27 秒。
var (
	prHeadTries = 10
	prHeadWait  = 3 * time.Second
)

func (c *ctx) waitPRHead(pr, head string) (string, error) {
	for i := 1; ; i++ {
		ph, err := prHead(pr)
		if err != nil || ph == head || i >= prHeadTries || !c.maybePushed(head) {
			return ph, err
		}
		time.Sleep(prHeadWait)
	}
}

func (c *ctx) maybePushed(head string) bool {
	args := []string{"rev-parse", "--verify", "-q", "@{u}"}
	if dir := c.issue.Path("worktree"); dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	return err != nil || strings.TrimSpace(string(out)) == head
}

func (c *ctx) previous() (*contract.Report, error) {
	r, err := contract.ReadReport(c.reportPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fail("library/report.json の形が違う: %v", err)
	}
	return r, nil
}

func (c *ctx) show(stdout io.Writer) error {
	out := map[string]any{"id": c.id, "status": c.issue.Str("status"), "settings": ledger.SettingsOf(c.issue), "report": nil}
	prev, err := c.previous()
	if err != nil {
		return err
	}
	if prev != nil {
		out["report"] = prev
	}
	if head, err := c.head(); err == nil {
		rows := c.l.Log()
		tasks, err := c.l.List()
		if err != nil {
			return err
		}
		var waived []contract.Waived
		if prev != nil {
			waived = prev.Waived
		}
		s := ledger.SettingsOf(c.issue)
		out["head"] = head
		out["review"] = c.l.CheckReview(rows, tasks, c.issue, head, s.MustFix, waived, nil)
	}
	_, err = stdout.Write(contract.Marshal(out))
	return err
}

func readMarkdown(stdin io.Reader) (string, error) {
	b, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", fail("人が読む報告（Markdown）を stdin で渡す")
	}
	return string(b), nil
}

func (c *ctx) write(r *contract.Report, md string, stdout io.Writer) error {
	if err := r.Check(); err != nil {
		return fail("report.json の形が違う: %v", err)
	}
	if err := os.MkdirAll(c.l.Library(c.id), 0o755); err != nil {
		return err
	}
	if err := contract.Write(c.reportPath(), contract.SchemaReport, r); err != nil {
		return err
	}
	out, err := c.l.Run(md, "report", c.id)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	fmt.Fprintf(stdout, "%s/library/report.json written\n", c.id)
	return nil
}

func (c *ctx) report(stdin io.Reader, stdout io.Writer, pr, data string) error {
	md, err := readMarkdown(stdin)
	if err != nil {
		return err
	}
	head := ""
	if c.hasRepo() {
		if head, err = c.head(); err != nil {
			return err
		}
	}
	r := &contract.Report{}
	if data != "" {
		if err := contract.Decode([]byte(data), r); err != nil {
			return fail("--data が読めない: %v", err)
		}
		if r.Commit != "" || r.ReviewRequest != nil {
			return fail("--data に commit・review_request は書かない（command が書く）")
		}
	}
	r.Commit = head
	if pr != "" {
		r.PR = pr
	}
	prev, err := c.previous()
	if err != nil && (r.PR == "" || r.Estimate == nil || r.Plan == nil || r.Gaps == nil) {
		return err
	}
	if prev != nil {
		if r.PR == "" {
			r.PR = prev.PR
		}
		if r.Estimate == nil {
			r.Estimate = prev.Estimate
		}
		if r.Plan == nil {
			r.Plan = prev.Plan
		}
		if r.Gaps == nil { // 空の配列（取り下げ）は nil でないので引き継がない（ADR-0061 決めたこと 2）
			r.Gaps = prev.Gaps
		}
		r.ReviewRequest = carriedReviewRequest(prev, head)
	}
	if err := c.samePRRepo(r.PR); err != nil {
		return err
	}
	return c.write(r, md, stdout)
}

func carriedReviewRequest(prev *contract.Report, head string) *contract.ReviewRequest {
	q := prev.ReviewRequest
	if q == nil || head == "" || prev.Commit != head || q.Commit != head {
		return nil
	}
	return q
}

// samePRRepo は pr が課題の repo（issue.yaml の repo）の PR かを確かめる（ADR-0034 決めたこと 11）。
// --pr・--data の pr・前の報告からの引き継ぎのどれで来た pr も、report.json に書く前にここを通る。
// dispatcher と gate は issue.yaml の repo の PR だけを見るので、違う repo の PR を書くと課題が片付かない。
// 形の違う pr は Check が断るので、ここでは repo だけを見る。
func (c *ctx) samePRRepo(pr string) error {
	owner, name, _, ok := contract.ParsePR(pr)
	if pr == "" || !ok {
		return nil
	}
	repo := c.issue.Str("repo")
	if repo == "" {
		return fail("repo の無い課題に PR は書かない: %s", pr)
	}
	if got := "github.com/" + owner + "/" + name; got != repo {
		return fail("pr の repo（%s）が課題の repo（%s）と違う: %s", got, repo, pr)
	}
	return nil
}

func (c *ctx) reviewRequest(stdin io.Reader, stdout io.Writer) error {
	md, err := c.reviewMarkdown(stdin)
	if err != nil {
		return err
	}
	s := ledger.SettingsOf(c.issue)
	if s.Review == "" {
		return fail("issue.yaml の review が on/off でない（%q）。レビューは頼めない（Chief of Staff に書いてもらう）", c.issue.Str("review"))
	}
	if c.issue.Str("review_rounds") != "" && c.issue.Str("review_rounds") != s.ReviewRounds {
		return fail("issue.yaml の review_rounds が 1 以上の数でない: %q", c.issue.Str("review_rounds"))
	}
	prev, err := c.previous()
	if err != nil {
		return err
	}
	if prev == nil || prev.PR == "" {
		return fail("report.json に pr が無い。先に andon issue report <id> --pr <url> する")
	}
	head, err := c.head()
	if err != nil {
		return err
	}
	ph, err := c.waitPRHead(prev.PR, head)
	if err != nil {
		return err
	}
	if ph != head {
		return fail("PR の head（%s）が HEAD（%s）と違う。push してから頼む", ph[:12], head[:12])
	}
	need, cl, err := gate.NeedsReview(c.issue.Str("repo"), s.Review, prev)
	if err != nil {
		return fail("差分のレベルを決められない（レビューが要るか分からない）: %v", err)
	}
	if !need {
		return fail("issue.yaml の review が off で、差分は%s。gate はレビューを求めないので頼めない。andon issue ready へ進む", gate.TierLine(cl))
	}
	round, previous, err := c.rounds(head)
	if err != nil {
		return err
	}
	limit, _ := strconv.Atoi(s.ReviewRounds)
	rr := &contract.ReviewRequest{Round: round, Limit: limit, Commit: head, Previous: previous, Reviewer: s.Reviewer, Extra: round > limit}
	r := *prev
	r.Commit, r.ReviewRequest = head, rr
	line := fmt.Sprintf("%d 回目／上限 %d", round, limit)
	if rr.Extra {
		line += "（追加）"
	}
	line += fmt.Sprintf("。commit %s。PR %s。Reviewer %s", head, prev.PR, s.Reviewer)
	if s.Review == "off" {
		line += "。review は off だが、差分が" + gate.TierLine(cl) + "なので gate はレビューを求める"
	}
	if previous != nil {
		line += fmt.Sprintf("。前回 %s（差分だけ）", *previous)
	}
	return c.write(&r, reviewHeader+line+"\n\n"+md, stdout)
}

const reviewHeader = "## レビュー依頼\n"

func (c *ctx) reviewMarkdown(stdin io.Reader) (string, error) {
	in := ""
	if f, ok := stdin.(*os.File); !ok || !spawn.IsTerminal(f) {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		in = string(b)
	}
	if strings.TrimSpace(in) != "" {
		return in, nil
	}
	b, err := os.ReadFile(filepath.Join(c.l.Dir, "issues", c.id, "report.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	md := string(b)
	for strings.HasPrefix(md, reviewHeader) {
		i := strings.Index(md, "\n\n")
		if i < 0 {
			md = ""
			break
		}
		md = md[i+2:]
	}
	if strings.TrimSpace(md) == "" {
		return "", fail("stdin が空で、前の報告（report.md）も空。人が読む報告（Markdown）を stdin で渡す")
	}
	return md, nil
}

var briefCommitRe = regexp.MustCompile(`commit ([0-9a-f]{40})。`)

// rounds は今度の回と前回の commit を数える: レビュアーの子課題（profile が *-review）の findings.json の
// commit のうち違うものの数と、kanban にあるレビュアーの課題（<id>-rN）の最大の N の、大きいほう ＋ 1。
// レビュアーの課題は結果が古い commit を見ていても（stale）kanban に残るので、頼み直すたびに回は 1 つ進む。
// HEAD のレビューがもうあれば頼まない。照合が読むレビューの PR の差分が HEAD と同じ（衝突の無い rebase）でも頼まない（引き継ぐ）。
// その課題のレビュアーの課題がまだ動いていれば（todo・doing）、見ている commit に関わらず頼み直さない（ADR-0032 決めたこと 8）:
//   - HEAD を見ている: 頼み直すと回だけが進み、dispatcher は起動した commit を二度起動しないので、知らせなしに捨てる（担当がレビューの回を1回失う）。
//   - 古い commit を見ている: 頼み直すとレビュアーが 2 人走る。照合は最後に終わったレビュアーだけを読むので、古い commit の
//     レビュアーが後に終わると ready は stale で断り、頼み直しは「HEAD のレビューはもうある」で断る（取り消しのほかに進めない。
//     model/ の規則 R7 と見本 BrokenReviewTwice）。
func (c *ctx) rounds(head string) (int, *string, error) {
	tasks, err := c.l.List()
	if err != nil {
		return 0, nil, err
	}
	seen := map[string]bool{}
	taken := 0
	var last *string
	lastAt := ""
	for _, t := range tasks {
		if t.Str("parent") != c.id || !strings.HasSuffix(t.Str("profile"), "-review") {
			continue
		}
		if _, n, ok := ledger.SplitRound(t.Str("id")); ok && n > taken {
			taken = n
		}
		if st := t.Str("status"); st == "todo" || st == "doing" {
			b, _ := os.ReadFile(filepath.Join(c.l.Dir, "issues", t.Str("id"), "brief.md"))
			if ledger.BriefReviews(string(b), head) {
				return 0, nil, fail("commit %s のレビューはもう頼んである（%s が %s）。回は進めない。結果が届くのを待つ", head[:12], t.Str("id"), st)
			}
			seeing := "見ている commit は依頼書から読めない"
			if m := briefCommitRe.FindSubmatch(b); m != nil {
				seeing = "commit " + string(m[1])[:12] + " を見ている"
			}
			return 0, nil, fail("レビュアー %s がまだ動いている（%s。%s）。動いている間は別の commit でも頼み直さない（レビュアーが 2 人走ると照合が古いほうを読み、ready も頼み直しも断られる）。結果が届いてから頼み直す", t.Str("id"), st, seeing)
		}
		f, err := contract.ReadFindings(filepath.Join(c.l.Library(t.Str("id")), "findings.json"))
		if err != nil {
			continue
		}
		if f.Commit == head {
			return 0, nil, fail("commit %s のレビューはもうある（%s）。andon issue ready へ進む", head[:12], t.Str("id"))
		}
		seen[f.Commit] = true
		if at := t.Str("updated"); last == nil || at >= lastAt {
			commit := f.Commit
			last, lastAt = &commit, at
		}
	}
	// rebase で head が変わっても、照合が読むレビューと PR の差分（足した行と消した行）が同じなら、そのレビューを head に
	// 引き継ぐ（ledger.CheckReview）。頼むとレビュアーが同じ差分をもう一度見るので頼まない（decisions/2026-11-21-review-carry-ignores-context-lines.md）
	if rv := c.l.CheckReview(c.l.Log(), tasks, c.issue, head, "", nil, nil); rv.State == "current" {
		return 0, nil, fail("HEAD（%s）の PR の差分は、レビュー %s が見た commit %.12s と同じ（足した行と消した行が空白も含めて同じ）。そのレビューを引き継ぐので頼まない。andon issue report で書き直して andon issue ready へ進む", head[:12], rv.Issue, rv.Commit)
	}
	return max(len(seen), taken) + 1, last, nil
}

func (c *ctx) ready(stdout io.Writer) error {
	stale, err := c.staleGateQuestions()
	if err != nil {
		return err
	}
	held, err := c.unreceived([]string{c.id}, stale)
	if err != nil {
		return err
	}
	if len(held) > 0 {
		return fail("受け取られていない問いがあるので review にしない: %s。答えを受けて問いを received にしてから打ち直す", strings.Join(held, "・"))
	}
	r, err := contract.ReadReport(c.reportPath())
	if errors.Is(err, os.ErrNotExist) {
		return fail("library/report.json が無い。andon issue report で書く")
	}
	if err != nil {
		return fail("library/report.json の形が違う: %v", err)
	}
	// 課題が受け入れ条件の番号を持てば、その番号の行が報告に無いと断る（ADR-0035 決めたこと 3）。番号を持たない課題は今のまま
	if err := checkAcceptanceRows(c.issue, r.Acceptance); err != nil {
		return err
	}
	// 受け入れ条件の行が記録を指すときだけ、その記録を確かめる（ADR-0035 決めたこと 2・11）。
	// 記録の commit が HEAD より古いのは断らない（gate が記録に出す）
	if problems, _ := contract.CheckRuns(c.l.Library(c.id), r.Acceptance, ""); len(problems) > 0 {
		return fail("受け入れ条件の行が指す記録が証拠にならないので review にしない: %s。andon issue record で撮り直し、andon issue report で書き直してから打ち直す", strings.Join(problems, "・"))
	}
	if ledger.SettingsOf(c.issue).Deliverable == "candidates" {
		// 照合（Chief of Staff が書いたか）は見ない: Chief of Staff が起き直したあと書き直す前でも PR を求めない。
		// review に入った調べる課題は gate を回さず Chief of Staff に届く（ADR-0006 決めたこと 17）
		return c.readyCandidates(r, stdout)
	}
	if r.PR == "" {
		return fail("report.json に pr が無い（andon issue report <id> --pr <url>）")
	}
	head, err := c.head()
	if err != nil {
		return err
	}
	if r.Commit != head {
		return fail("report.json の commit（%.12s）が HEAD（%s）と違う。andon issue report で書き直す", r.Commit, head[:12])
	}
	ph, err := c.waitPRHead(r.PR, head)
	if err != nil {
		return err
	}
	if ph != head {
		return fail("PR の head（%s）が HEAD（%s）と違う。push する", ph[:12], head[:12])
	}
	s := ledger.SettingsOf(c.issue)
	if s.Review == "" {
		return fail("issue.yaml の review が on/off でない（%q）。Chief of Staff に書いてもらう", c.issue.Str("review"))
	}
	need, cl, err := gate.NeedsReview(c.issue.Str("repo"), s.Review, r)
	if err != nil {
		return fail("差分のレベルを決められない（gate がレビューを求めるか分からない）: %v", err)
	}
	if need {
		tasks, err := c.l.List()
		if err != nil {
			return err
		}
		rv := c.l.CheckReview(c.l.Log(), tasks, c.issue, head, s.MustFix, r.Waived, nil)
		switch {
		case rv.State == "current":
		case s.Review == "on":
			return fail("レビューが %s: %s", rv.State, rv.Reason)
		default:
			return fail("レビューが %s: %s。review は off だが、差分が%sなので gate はレビューを求める。andon issue review-request で頼む", rv.State, rv.Reason, gate.TierLine(cl))
		}
	}
	review := []string{"review", c.id}
	if len(stale) > 0 {
		review = append(review, "--past", strings.Join(slices.Sorted(maps.Keys(stale)), ","))
	}
	out, err := c.l.Run("", review...)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	for _, q := range slices.Sorted(maps.Keys(stale)) {
		fmt.Fprintf(stdout, "%s は gate が前の head（%.12s）について出した問いなので数えない（gate が今の head で回ったときに閉じる）\n", q, stale[q])
	}
	return nil
}

func checkAcceptanceRows(t ledger.Issue, rows []contract.Acceptance) error {
	ids, ok := ledger.AcceptanceOf(t)
	if !ok {
		return fail("issue.yaml の acceptance の形が違う（%v）。受け入れ条件の番号をコンマで並べる（4,13）。Chief of Staff に書き直してもらう", t["acceptance"])
	}
	var missing []string
	for _, id := range ids {
		if !slices.ContainsFunc(rows, func(a contract.Acceptance) bool { return ledger.AcceptanceKey(a.ID) == ledger.AcceptanceKey(id) }) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fail("課題が名指す受け入れ条件の行が report.json の acceptance に無いので review にしない: %s。行を足して andon issue report で書き直してから打ち直す", strings.Join(missing, "・"))
	}
	return nil
}

func (c *ctx) gateQuestionHeads() (map[string]string, error) {
	gateQs, _, err := c.openQuestions()
	if err != nil {
		return nil, err
	}
	heads := map[string]string{}
	for _, q := range gateQs {
		if h := c.questionHead(q.N); h != "" {
			heads[fmt.Sprintf("%s#%d", c.id, q.N)] = h
		}
	}
	return heads, nil
}

func (c *ctx) questionHead(n int) string {
	b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "questions", strconv.Itoa(n)+".json"))
	var v struct {
		Facts []string `json:"facts"`
	}
	if err != nil || json.Unmarshal(b, &v) != nil {
		return ""
	}
	return gate.QuestionHead(v.Facts)
}

func (c *ctx) staleGateQuestions() (map[string]string, error) {
	heads, err := c.gateQuestionHeads()
	if err != nil || len(heads) == 0 {
		return nil, err
	}
	head, err := c.head()
	if err != nil {
		return nil, err
	}
	maps.DeleteFunc(heads, func(_, h string) bool { return h == head })
	return heads, nil
}

// readyCandidates は調べる課題（deliverable: candidates。ADR-0006 決めたこと 7）の ready: PR とレビューの代わりに、
// candidates.json が在って形が合い、repo があれば report.json と candidates.json の commit が HEAD と同じことを確かめる。
func (c *ctx) readyCandidates(r *contract.Report, stdout io.Writer) error {
	cs, err := contract.ReadCandidates(c.candidatesPath())
	if errors.Is(err, os.ErrNotExist) {
		return fail("deliverable が candidates なのに library/candidates.json が無い。andon issue candidates で書く")
	}
	if err != nil {
		return fail("library/candidates.json の形が違う: %v", err)
	}
	if c.hasRepo() {
		head, err := c.head()
		if err != nil {
			return err
		}
		if r.Commit != head {
			return fail("report.json の commit（%.12s）が HEAD（%s）と違う。andon issue report で書き直す", r.Commit, head[:12])
		}
		if cs.Commit != head {
			return fail("candidates.json の commit（%.12s）が HEAD（%s）と違う。andon issue candidates で書き直す", cs.Commit, head[:12])
		}
	}
	out, err := c.l.Run("", "review", c.id)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}

// candidates は候補の一覧（contract.md §12）を検査して library/candidates.json に置く。commit は command が書く
// （repo があれば HEAD、無ければ書かない）。kanban には書かない（報告と問いが候補を指す）。
func (c *ctx) candidates(stdin io.Reader, stdout io.Writer, src string) error {
	b, err := readInput(stdin, src)
	if err != nil {
		return err
	}
	if me, agent := os.Getenv("HC_TERMINAL_ID"), c.issue.Str("agent"); me == "" || me != agent {
		return fail("%s の担当（agent: %q）の端末でない（HC_TERMINAL_ID: %q）。候補の一覧は自分の課題にだけ書く", c.id, agent, me)
	}
	cs, err := contract.ParseCandidates(b)
	if err != nil {
		return fail("候補の一覧の JSON の形が違う: %v", err)
	}
	if cs.Commit != "" {
		return fail("候補の一覧に commit は書かない（command が書く）")
	}
	if c.hasRepo() {
		if cs.Commit, err = c.head(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(c.l.Library(c.id), 0o755); err != nil {
		return err
	}
	if err := contract.Write(c.candidatesPath(), contract.SchemaCandidates, cs); err != nil {
		return err
	}
	in := 0
	for _, x := range cs.Candidates {
		if *x.InScope {
			in++
		}
	}
	fmt.Fprintf(stdout, "%s/library/candidates.json written（候補 %d 件、範囲の中 %d 件）\n", c.id, len(cs.Candidates), in)
	return nil
}

var askedRe = regexp.MustCompile(`#([1-9][0-9]*)$`)

func (c *ctx) ask(stdin io.Reader, stdout io.Writer, src string) error {
	b, err := readInput(stdin, src)
	if err != nil {
		return err
	}
	q, err := contract.ParseQuestion(b)
	if err != nil {
		return fail("問いの JSON の形が違う: %v", err)
	}
	out, err := c.put(q)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}

func (c *ctx) put(q *contract.Question) (string, error) { return c.l.Ask(c.id, q) }

// askQueued は queued の課題に問いを置けるかを確かめる。置けるのはオーナーの課題（who: person）だけで、
// 担当の課題は前の課題を待つ間は問いを出さない（ADR-0057 決めたこと 7）。
func (c *ctx) askQueued() error {
	if c.issue.Str("status") == "queued" && c.issue.Str("who") != "person" {
		return fail("%s は queued（queued の課題に問いを置けるのはオーナーの課題〈who: person〉だけ。process/states/issue.yaml）", c.id)
	}
	return nil
}
