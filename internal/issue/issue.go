// Package issue は担当が打つ andon issue …（docs/contract.md §4・§5）。機械向けの約束（report.json・問いの JSON）を
// 書いて検査し、台帳の CLI（report・review・ask）を打つ。担当は JSON を手で書かず、一致の確認も command がする。
//
//	show <id>             予約キー（既定で埋めたもの）・report.json・レビューの照合を JSON で出す（読むだけ）
//	report <id>           report.json を書き（commit は HEAD）、stdin の Markdown で台帳に報告する
//	review-request <id>   report.json に review_request を足して報告する（回数・前回の commit は command が数える）
//	ready <id>            report.json・HEAD・PR の head・レビューの commit が揃っていれば台帳を review にする
//	                      （deliverable: candidates なら PR の代わりに candidates.json を確かめる）
//	ask <id> <file|->     問いの JSON を検査し、人向けの text を描いて台帳に問い、library/questions/<n>.json に置く
//	candidates <id> <file|->  候補の一覧の JSON を検査し、library/candidates.json に置く（repo があれば commit は HEAD）
//	record <id> [--expect fail] -- <command…>  worktree で command を走らせ、出力と終了コード・commit・commit していない変更の有無・時刻を
//	                      library/runs/<n>.json と <n>.log に残す（record.go。証拠の記録）
//	start <id>            worktree と branch を切り、hc run で担当を起動し、kanban start で結ぶ（Chief of Staff が打つ）
//	                      （--dry-run は確かめだけ通し、起動するときに渡すものを出す。--json で JSON）
//	resume <id>           止まった担当・Chief Engineer を同じ worktree でセッションの続きから起動する（--fresh は新しいセッション。ADR-0012）
//	session               自分の端末のラベル session を出す（template の command・resume の行が読む）
//	finish <id>           端末と worktree を片付け、origin に入った branch を消し、台帳を done にする（Chief of Staff・dispatcher が打つ）
//	                      （links に write_back: true の出所の issue があれば、そこに結果を1コメント書く。writeback.go）
//	settle <id>           review・doing の課題の PR を人がマージしていたら、ゲートの問いを閉じて finish する（積む通知を JSON で出す。dispatcher が打つ）
//	merge <id>            review の課題にゲートを回し、approve ＋ merge: auto ＋ 委譲の範囲内ならマージして finish する（積む通知を JSON で出す。dispatcher が打つ）
//	impact <id>           マージした PR が、同じ repo の doing／review の課題にぶつかる・重なるかの事実を JSON で出す（読むだけ。dispatcher が打つ）
//	stop-check [<id>]     担当・レビュアーの Stop フック: doing で報告・review・問いが無ければ止める（contract.md §9）
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
)

// Usage は andon issue --help の中身。
const Usage = "usage: andon issue show <id>                                  予約キー・report.json・レビューの照合を JSON で出す\n" +
	"       andon issue report <id> [--pr <url>] [--data <json>] < report.md\n" +
	"                                                          report.json を書き（commit は HEAD）、台帳に報告する\n" +
	"       andon issue review-request <id> < report.md           review_request を足して報告する（レビューを頼む）\n" +
	"       andon issue ready <id>                                 HEAD・report.json・PR の head・レビューが揃えば review にする\n" +
	"                                                          （deliverable: candidates なら PR の代わりに candidates.json）\n" +
	"       andon issue ask <id> <question.json|->                 問いの JSON を検査して台帳に問う\n" +
	"       andon issue candidates <id> <candidates.json|->        候補の一覧を検査して library/candidates.json に書く（commit は command が書く）\n" +
	"       andon issue record <id> [--expect pass|fail] -- <command> [<arg>…]\n" +
	"                                                          worktree で command を走らせ、記録を library/runs/<n>.json・<n>.log に書く\n" +
	"                                                          （期待は既定で pass〈終了コード 0〉。fail は誤りを戻すと落ちることを示す記録。\n" +
	"                                                          期待と違っても記録は書いて終了コード 1。起動できなければ書かない）\n" +
	"       andon issue finish <id>                                端末と worktree を片付けて done にする（MERGED を確かめてから打つ。受け取られていない問いがあれば何も消さずに止まる）\n" +
	"                                                          （links に write_back: true の出所の issue があれば、結果を1コメント書く）\n" +
	"       andon issue settle <id>                                review・doing の課題の PR を人がマージしていたら、ゲートの問いを閉じて片付ける\n" +
	"                                                          （PR ごとに1分に1回 gh pr view。積む通知を JSON で出す。2回打っても同じ）\n" +
	"       andon issue merge <id>                                 review の課題にゲートを回し、approve ＋ merge: auto ＋ 委譲の範囲内ならマージして片付ける\n" +
	"                                                          （review に入った行ごとに1回。fix は担当へ、needs_human は問い。積む通知を JSON で出す）\n" +
	"       andon issue impact <id>                                マージした PR と、同じ repo の動いている課題の重なり・衝突を JSON で出す\n" +
	"                                                          （PR がまだ MERGED でなければ終了コード 4）\n" +
	"       andon issue start <id> [--dry-run [--json]]            担当を起動する: worktree と branch・hc run・kanban start（Chief of Staff が打つ）\n" +
	"                                                          （最初のメッセージが届かなければ終了コード 3。dispatcher が送り直す）\n" +
	"                                                          （--dry-run は確かめだけ通し、hc を呼ばず何も作らずに、渡すもの〈最初のメッセージ・依頼書・\n" +
	"                                                          ラベル・端末の名前・worktree・branch・プロファイル・確かめの結果〉を出す）\n" +
	"       andon issue resume <id> [--fresh] [--dry-run [--json]]   止まった担当・Chief Engineer を続きから起動する（doing で端末が無い・終わった・素のシェル）\n" +
	"                                                          （worktree が無ければ戻し、<profile>-resume の template でセッションの続きから。\n" +
	"                                                          記録が無い・最初のメッセージが届かなければ台帳を変えずに終了コード 1。\n" +
	"                                                          --fresh は同じ worktree で新しいセッション〈人の go のあと〉）\n" +
	"       andon issue session                                    自分の端末（HC_TERMINAL_ID）のラベル session を出す（template が読む）\n" +
	"       andon issue stop-check [<id>] < フックの入力の JSON         担当の Stop フック: doing で報告も問いも無ければ止める\n" +
	"                                                          （id を省くと端末のラベル issue=。担当でなければ何も出さない）\n" +
	"--data は report.json の任意のキー（pr・estimate・plan・waived・acceptance・risk・next・observation・remember・agents_md）の JSON object。\n" +
	"--pr を省くと前の report.json の pr を、--data に estimate・plan が無ければ前のそれを引き継ぐ。\n" +
	"review_request は、HEAD が前の report.json の commit と同じ間だけ引き継ぐ（HEAD が進めば消える。頼み直しは review-request）。\n"

const (
	exitFail  = 1
	exitUsage = 2
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func fail(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

// Main は andon issue。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return exitUsage }
	if len(args) > 0 && args[0] == "stop-check" {
		return stopCheck(args[1:], stdin, stdout, stderr)
	}
	if len(args) == 1 && args[0] == "session" {
		return session(stdout, stderr)
	}
	if len(args) < 2 {
		return usage()
	}
	sub, id, rest := args[0], ledger.CanonID(args[1]), args[2:]
	if !idRe.MatchString(id) {
		fmt.Fprintf(stderr, "andon issue %s: id が不正: %q\n", sub, id)
		return exitUsage
	}
	// guard は定義（process/states/issue.yaml）に無い移り方を、何かを書く前に断る（ADR-0017 決めたこと 15）。
	// start は自分の確かめの順の中で見る（start.go。--dry-run の確かめの一覧に載せる）。
	guard := func(f func(*ctx) error) func(*ctx) error {
		return func(c *ctx) error {
			if err := allowed(sub, c.id, c.issue.Str("status")); err != nil {
				return err
			}
			return f(c)
		}
	}
	var err error
	switch sub {
	case "show":
		if len(rest) > 0 {
			return usage()
		}
		err = withIssue(id, func(c *ctx) error { return c.show(stdout) })
	case "report":
		var pr, data string
		for len(rest) > 0 {
			if len(rest) < 2 || (rest[0] != "--pr" && rest[0] != "--data") {
				return usage()
			}
			if rest[0] == "--pr" {
				pr = rest[1]
			} else {
				data = rest[1]
			}
			rest = rest[2:]
		}
		err = withIssue(id, guard(func(c *ctx) error { return c.report(stdin, stdout, pr, data) }))
	case "review-request":
		if len(rest) > 0 {
			return usage()
		}
		err = withIssue(id, guard(func(c *ctx) error { return c.reviewRequest(stdin, stdout) }))
	case "ready":
		if len(rest) > 0 {
			return usage()
		}
		err = withIssue(id, guard(func(c *ctx) error { return c.ready(stdout) }))
	case "ask":
		if len(rest) != 1 {
			return usage()
		}
		err = withIssue(id, guard(func(c *ctx) error { return c.ask(stdin, stdout, rest[0]) }))
	case "candidates":
		if len(rest) != 1 {
			return usage()
		}
		err = withIssue(id, guard(func(c *ctx) error { return c.candidates(stdin, stdout, rest[0]) }))
	case "record":
		expect := "pass"
		if len(rest) >= 2 && rest[0] == "--expect" {
			expect, rest = rest[1], rest[2:]
		}
		if (expect != "pass" && expect != "fail") || len(rest) < 2 || rest[0] != "--" || rest[1] == "" {
			return usage()
		}
		argv := rest[1:]
		err = withIssue(id, guard(func(c *ctx) error { return c.record(stdout, expect, argv) }))
	case "finish":
		if len(rest) > 0 {
			return usage()
		}
		err = withLockedIssue(id, guard(func(c *ctx) error {
			if err := c.finish(stdout); err != nil {
				return err
			}
			c.writeBack(stdout) // done になってから。書けなくても finish は戻さない（ADR-0011 決めたこと 15）
			return nil
		}))
	case "settle":
		if len(rest) > 0 {
			return usage()
		}
		err = withLockedIssue(id, func(c *ctx) error { return c.settle(stdout, stderr, time.Now()) })
	case "merge":
		if len(rest) > 0 {
			return usage()
		}
		err = withLockedIssue(id, func(c *ctx) error { return c.merge(stdout, stderr, time.Now()) })
	case "impact":
		if len(rest) > 0 {
			return usage()
		}
		err = withIssue(id, func(c *ctx) error { return c.impact(stdout) })
	case "start":
		var dry, asJSON bool
		for _, a := range rest {
			switch {
			case a == "--dry-run" && !dry:
				dry = true
			case a == "--json" && !asJSON:
				asJSON = true
			default:
				return usage()
			}
		}
		if asJSON && !dry {
			return usage()
		}
		with := withLockedIssue
		if dry {
			with = withIssue
		}
		err = with(id, func(c *ctx) error { return c.start(stdout, stderr, dry, asJSON) })
	case "resume":
		var dry, asJSON, fresh bool
		for _, a := range rest {
			switch {
			case a == "--dry-run" && !dry:
				dry = true
			case a == "--json" && !asJSON:
				asJSON = true
			case a == "--fresh" && !fresh:
				fresh = true
			default:
				return usage()
			}
		}
		if asJSON && !dry {
			return usage()
		}
		with := withLockedIssue
		if dry {
			with = withIssue
		}
		err = with(id, func(c *ctx) error { return c.resume(stdout, stderr, dry, asJSON, fresh) })
	default:
		return usage()
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon issue %s: %v\n", sub, err)
		if errors.As(err, new(notSent)) {
			return exitFirst
		}
		if errors.As(err, new(notMerged)) {
			return exitNotMerged
		}
		return exitFail
	}
	return 0
}

// ctx は1つの課題について読んだもの。
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

// hasRepo は repo のある課題か（issue.yaml の repo）。worktree では見ない: kanban start は cwd の git の根を
// worktree に書くので、repo の無い課題でも台帳が git の checkout の中にあれば worktree が入る。
// repo の無い課題の報告と候補は commit を持たない。
func (c *ctx) hasRepo() bool { return c.issue.Str("repo") != "" }

// readInput は <file|-> の中身。
func readInput(stdin io.Reader, src string) ([]byte, error) {
	if src == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(src)
}

// head は担当の worktree（issue.yaml の worktree。無ければ cwd）の HEAD。
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

// prHead は PR の head の commit（gh pr view）。
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

// previous は前の report.json（無ければ nil）。あるのに形が壊れていれば誤り。
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

// readMarkdown は stdin の人用の報告。空なら誤り（報告の Markdown は人と Chief of Staff が読む）。
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

// write は report.json を検査して置き、台帳の CLI で報告する（dispatcher は report の書き込みを合図に report.json を読む）。
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
	r.Commit = head // repo の無い課題では空（書かない）
	if pr != "" {
		r.PR = pr
	}
	// 前の report.json が壊れていても、pr・estimate・plan を全部渡せば書き直せる（引き継ぐものが要るときだけ断る）
	prev, err := c.previous()
	if err != nil && (r.PR == "" || r.Estimate == nil || r.Plan == nil) {
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
		r.ReviewRequest = carriedReviewRequest(prev, head)
	}
	if err := c.samePRRepo(r.PR); err != nil {
		return err
	}
	return c.write(r, md, stdout)
}

// carriedReviewRequest は報告の打ち直しが引き継ぐ review_request。HEAD が前の報告の commit とも頼んだ commit とも
// 同じときだけ引き継ぐ（同じ commit のレビュー依頼を、報告を書き直しただけで消さない）。HEAD が進んだら引き継がない
// （古い commit の依頼を残すと、dispatcher が PR の head と違う commit のレビュアーを起動しに行く）。repo の無い課題（head が空）も引き継がない。
func carriedReviewRequest(prev *contract.Report, head string) *contract.ReviewRequest {
	q := prev.ReviewRequest
	if q == nil || head == "" || prev.Commit != head || q.Commit != head {
		return nil
	}
	return q
}

// samePRRepo は pr が課題の repo（issue.yaml の repo）の PR かを確かめる（ADR-0034 決めたこと 11）。
// --pr・--data の pr・前の報告からの引き継ぎのどれで来た pr も、report.json に書く前にここを通る。
// dispatcher とゲートは issue.yaml の repo の PR だけを見るので、違う repo の PR を書くと課題が片付かない。
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
	md, err := readMarkdown(stdin)
	if err != nil {
		return err
	}
	s := ledger.SettingsOf(c.issue)
	if s.Review != "on" {
		return fail("issue.yaml の review が on でない（%q）。レビューは頼めない", c.issue.Str("review"))
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
	ph, err := prHead(prev.PR)
	if err != nil {
		return err
	}
	if ph != head {
		return fail("PR の head（%s）が HEAD（%s）と違う。push してから頼む", ph[:12], head[:12])
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
	if previous != nil {
		line += fmt.Sprintf("。前回 %s（差分だけ）", *previous)
	}
	return c.write(&r, "## レビュー依頼\n"+line+"\n\n"+md, stdout)
}

// briefCommitRe はレビュアーの依頼書の commit（「commit <sha>。」の形。ledger.BriefReviews と同じ読み方）。
var briefCommitRe = regexp.MustCompile(`commit ([0-9a-f]{40})。`)

// rounds は今度の回と前回の commit を数える: レビュアーの子課題（profile が *-review）の findings.json の
// commit のうち違うものの数と、台帳にあるレビュアーの課題（<id>-rN）の最大の N の、大きいほう ＋ 1。
// レビュアーの課題は結果が古い commit を見ていても（stale）台帳に残るので、頼み直すたびに回は 1 つ進む。
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
	taken := 0 // 結果の有無に関わらず、台帳にあるレビュアーの課題（<id>-rN）の最大の N
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
	// 衝突の無い rebase で head だけが変わり、照合が読むレビューと PR の差分が同じなら、そのレビューを head に
	// 引き継ぐ（ledger.CheckReview）。頼むとレビュアーが同じ差分をもう一度見るので頼まない（decisions/2026-11-20-review-carry-same-patch.md）
	if rv := c.l.CheckReview(c.l.Log(), tasks, c.issue, head, "", nil, nil); rv.State == "current" {
		return 0, nil, fail("HEAD（%s）の PR の差分は、レビュー %s が見た commit %.12s と同じ（行番号のほかは空白も含めて同じ）。そのレビューを引き継ぐので頼まない。andon issue report で書き直して andon issue ready へ進む", head[:12], rv.Issue, rv.Commit)
	}
	// 結果を残さず止まったレビュアーの課題も台帳に残り、その N は使えない（dispatcher の kanban add --round が落ちる）。
	// 数え方の決まりは変えず（previous も結果のある回だけ）、番号が被るときだけ空いた先へ送る
	return max(len(seen), taken) + 1, last, nil
}

func (c *ctx) ready(stdout io.Writer) error {
	// 受け取られていない問いを残して review に入ると、ゲートとマージのあとの片付け（finish）が問いで断られる。
	// 確かめの形は finish と kanban done（closing）と同じ。ただしゲートが前の head について出した問いは数えない
	// （rebase や直しで head が動いた。回し直したゲートが今の head で決め直し、そのとき閉じる。merge.go closeStaleGateQuestions）
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
	// 受け入れ条件の行が記録を指すときだけ、その記録を確かめる（ADR-0035 決めたこと 2・11）。
	// 記録の commit が HEAD より古いのは断らない（ゲートが記録に出す）
	if problems, _ := contract.CheckRuns(c.l.Library(c.id), r.Acceptance, ""); len(problems) > 0 {
		return fail("受け入れ条件の行が指す記録が証拠にならないので review にしない: %s。andon issue record で撮り直し、andon issue report で書き直してから打ち直す", strings.Join(problems, "・"))
	}
	if ledger.SettingsOf(c.issue).Deliverable == "candidates" {
		// 照合（Chief of Staff が書いたか）は見ない: Chief of Staff が起き直したあと書き直す前でも PR を求めない。
		// review に入った調べる課題はゲートを回さず Chief of Staff に届く（ADR-0006 決めたこと 17）
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
	ph, err := prHead(r.PR)
	if err != nil {
		return err
	}
	if ph != head {
		return fail("PR の head（%s）が HEAD（%s）と違う。push する", ph[:12], head[:12])
	}
	s := ledger.SettingsOf(c.issue)
	switch s.Review {
	case "on":
		tasks, err := c.l.List()
		if err != nil {
			return err
		}
		rv := c.l.CheckReview(c.l.Log(), tasks, c.issue, head, s.MustFix, r.Waived, nil)
		if rv.State != "current" {
			return fail("レビューが %s: %s", rv.State, rv.Reason)
		}
	case "off":
	default:
		return fail("issue.yaml の review が on/off でない（%q）。Chief of Staff に書いてもらう", c.issue.Str("review"))
	}
	review := []string{"review", c.id}
	if len(stale) > 0 {
		// kanban review も受け取られていない問いで断る。越えてよいのは外した問いだけなので、その番号を名指す。
		// --force にすると、上の確かめのあとに kanban ask で足された問いまで越えてしまう（kanban は名指しの外の問いで、
		// 台帳の lock の中で断る）。越えた問いの番号は log.jsonl の行の forced に残る
		review = append(review, "--past", strings.Join(slices.Sorted(maps.Keys(stale)), ","))
	}
	out, err := c.l.Run("", review...)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	for _, q := range slices.Sorted(maps.Keys(stale)) {
		fmt.Fprintf(stdout, "%s はゲートが前の head（%.12s）について出した問いなので数えない（ゲートが今の head で回ったときに閉じる）\n", q, stale[q])
	}
	return nil
}

// gateQuestionHeads は、この課題のまだ受け取られていないゲートの問い（library/questions/<n>.json の from: gate）のうち、
// どの head について出たかが読めるもの（gate.QuestionHead）を「<id>#<n>」→ head で返す。読めない問いは入れない。
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

// questionHead は、この課題の問い n（library/questions/<n>.json）がゲートのどの head について出たか。読めなければ空。
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

// staleGateQuestions は、まだ受け取られていないゲートの問いのうち、担当の HEAD と違う head についての問いを
// 「<id>#<n>」→ その head で返す（ready が数えない問い）。どの head の問いか読めない問いは入れない（今の head の問いとして数える）。
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
// （repo があれば HEAD、無ければ書かない）。台帳には書かない（報告と問いが候補を指す）。
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
	out, err := c.l.Run(q.Text(), "ask", c.id, "-", "--kind", q.Kind)
	if err != nil {
		return err
	}
	m := askedRe.FindStringSubmatch(out)
	if m == nil {
		return fail("台帳の CLI の返事から問いの番号が読めない: %q", out)
	}
	dir := filepath.Join(c.l.Library(c.id), "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := contract.Write(filepath.Join(dir, m[1]+".json"), contract.SchemaQuestion, q); err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}
