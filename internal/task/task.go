// Package task は担当が打つ andon task …（docs/contract.md §4・§5）。機械向けの約束（report.json・問いの JSON）を
// 書いて検査し、台帳の CLI（report・review・ask）を打つ。担当は JSON を手で書かず、一致の確認も command がする。
//
//	show <id>             予約キー（既定で埋めたもの）・report.json・レビューの照合を JSON で出す（読むだけ）
//	report <id>           report.json を書き（commit は HEAD）、stdin の Markdown で台帳に報告する
//	review-request <id>   report.json に review_request を足して報告する（回数・前回の commit は command が数える）
//	ready <id>            report.json・HEAD・PR の head・レビューの commit が揃っていれば台帳を review にする
//	ask <id> <file|->     問いの JSON を検査し、人向けの text を描いて台帳に問い、library/questions/<n>.json に置く
//	start <id>            worktree と branch を切り、hc run で担当を起こし、kanban start で結ぶ（Chief of Staff が打つ）
//	finish <id>           端末と worktree を片付け、origin に入った branch を消し、台帳を done にする（Chief of Staff・dispatcher が打つ）
//	stop-check [<id>]     担当・レビュアーの Stop フック: doing で報告・review・問いが無ければ止める（contract.md §9）
//
// 合わなければ終了コード 1 と理由1行。使い方の誤りは 2。start で最初のメッセージが届かなければ 3。
package task

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// Usage は andon task --help の中身。
const Usage = "usage: andon task show <id>                                  予約キー・report.json・レビューの照合を JSON で出す\n" +
	"       andon task report <id> [--pr <url>] [--data <json>] < report.md\n" +
	"                                                          report.json を書き（commit は HEAD）、台帳に報告する\n" +
	"       andon task review-request <id> < report.md           review_request を足して報告する（レビューを頼む）\n" +
	"       andon task ready <id>                                 HEAD・report.json・PR の head・レビューが揃えば review にする\n" +
	"       andon task ask <id> <question.json|->                 問いの JSON を検査して台帳に問う\n" +
	"       andon task finish <id>                                端末と worktree を片付けて done にする（MERGED を確かめてから打つ）\n" +
	"       andon task start <id>                                 担当を起こす: worktree と branch・hc run・kanban start（Chief of Staff が打つ）\n" +
	"                                                          （最初のメッセージが届かなければ終了コード 3。dispatcher が送り直す）\n" +
	"       andon task stop-check [<id>] < フックの入力の JSON         担当の Stop フック: doing で報告も問いも無ければ止める\n" +
	"                                                          （id を省くと端末のラベル task=。担当でなければ何も出さない）\n" +
	"--data は report.json の任意のキー（pr・waived・acceptance・next・remember・agents_md）の JSON object。\n" +
	"--pr を省くと前の report.json の pr を引き継ぐ。\n"

const (
	exitFail  = 1
	exitUsage = 2
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func fail(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

// Main は andon task。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return exitUsage }
	if len(args) > 0 && args[0] == "stop-check" {
		return stopCheck(args[1:], stdin, stdout, stderr)
	}
	if len(args) < 2 {
		return usage()
	}
	sub, id, rest := args[0], ledger.CanonID(args[1]), args[2:]
	if !idRe.MatchString(id) {
		fmt.Fprintf(stderr, "andon task %s: id が不正: %q\n", sub, id)
		return exitUsage
	}
	var err error
	switch sub {
	case "show":
		if len(rest) > 0 {
			return usage()
		}
		err = withTask(id, func(c *ctx) error { return c.show(stdout) })
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
		err = withTask(id, func(c *ctx) error { return c.report(stdin, stdout, pr, data) })
	case "review-request":
		if len(rest) > 0 {
			return usage()
		}
		err = withTask(id, func(c *ctx) error { return c.reviewRequest(stdin, stdout) })
	case "ready":
		if len(rest) > 0 {
			return usage()
		}
		err = withTask(id, func(c *ctx) error { return c.ready(stdout) })
	case "ask":
		if len(rest) != 1 {
			return usage()
		}
		err = withTask(id, func(c *ctx) error { return c.ask(stdin, stdout, rest[0]) })
	case "finish":
		if len(rest) > 0 {
			return usage()
		}
		err = withTask(id, func(c *ctx) error { return c.finish(stdout) })
	case "start":
		if len(rest) > 0 {
			return usage()
		}
		err = withTask(id, func(c *ctx) error { return c.start(stdout, stderr) })
	default:
		return usage()
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon task %s: %v\n", sub, err)
		if errors.As(err, new(notSent)) {
			return exitFirst
		}
		return exitFail
	}
	return 0
}

// ctx は1つのタスクについて読んだもの。
type ctx struct {
	l    *ledger.Ledger
	id   string
	task ledger.Task
}

func withTask(id string, f func(*ctx) error) error {
	l, err := ledger.Open()
	if err != nil {
		return err
	}
	t, err := l.Show(id)
	if err != nil {
		return err
	}
	return f(&ctx{l: l, id: id, task: t})
}

func (c *ctx) reportPath() string { return filepath.Join(c.l.Library(c.id), "report.json") }

// head は担当の worktree（task.yaml の worktree。無ければ cwd）の HEAD。
func (c *ctx) head() (string, error) {
	dir := c.task.Path("worktree")
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
	out := map[string]any{"id": c.id, "status": c.task.Str("status"), "settings": ledger.SettingsOf(c.task), "report": nil}
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
		s := ledger.SettingsOf(c.task)
		out["head"] = head
		out["review"] = c.l.CheckReview(rows, tasks, c.task, head, s.MustFix, waived)
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
	if err := contract.Write(c.reportPath(), r); err != nil {
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
	head, err := c.head()
	if err != nil {
		return err
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
	if r.PR == "" {
		prev, err := c.previous()
		if err != nil {
			return err
		}
		if prev != nil {
			r.PR = prev.PR
		}
	}
	return c.write(r, md, stdout)
}

func (c *ctx) reviewRequest(stdin io.Reader, stdout io.Writer) error {
	md, err := readMarkdown(stdin)
	if err != nil {
		return err
	}
	s := ledger.SettingsOf(c.task)
	if s.Review != "on" {
		return fail("task.yaml の review が on でない（%q）。レビューは頼めない", c.task.Str("review"))
	}
	if c.task.Str("review_rounds") != "" && c.task.Str("review_rounds") != s.ReviewRounds {
		return fail("task.yaml の review_rounds が 1 以上の数でない: %q", c.task.Str("review_rounds"))
	}
	prev, err := c.previous()
	if err != nil {
		return err
	}
	if prev == nil || prev.PR == "" {
		return fail("report.json に pr が無い。先に andon task report <id> --pr <url> する")
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

// rounds は今度の回と前回の commit を数える: レビュアーの子タスク（profile が *-review）の findings.json の
// commit のうち、違うものの数 ＋ 1。古い commit を見たレビューは前の回と同じ commit なので数が増えない
// （同じ回のまま頼み直す）。HEAD のレビューがもうあれば頼まない。
func (c *ctx) rounds(head string) (int, *string, error) {
	tasks, err := c.l.List()
	if err != nil {
		return 0, nil, err
	}
	seen := map[string]bool{}
	var last *string
	lastAt := ""
	for _, t := range tasks {
		if t.Str("parent") != c.id || !strings.HasSuffix(t.Str("profile"), "-review") {
			continue
		}
		f, err := contract.ReadFindings(filepath.Join(c.l.Library(t.Str("id")), "findings.json"))
		if err != nil {
			continue
		}
		if f.Commit == head {
			return 0, nil, fail("commit %s のレビューはもうある（%s）。andon task ready へ進む", head[:12], t.Str("id"))
		}
		seen[f.Commit] = true
		if at := t.Str("updated"); last == nil || at >= lastAt {
			commit := f.Commit
			last, lastAt = &commit, at
		}
	}
	return len(seen) + 1, last, nil
}

func (c *ctx) ready(stdout io.Writer) error {
	r, err := contract.ReadReport(c.reportPath())
	if errors.Is(err, os.ErrNotExist) {
		return fail("library/report.json が無い。andon task report で書く")
	}
	if err != nil {
		return fail("library/report.json の形が違う: %v", err)
	}
	if r.PR == "" {
		return fail("report.json に pr が無い（andon task report <id> --pr <url>）")
	}
	head, err := c.head()
	if err != nil {
		return err
	}
	if r.Commit != head {
		return fail("report.json の commit（%s）が HEAD（%s）と違う。andon task report で書き直す", r.Commit[:12], head[:12])
	}
	ph, err := prHead(r.PR)
	if err != nil {
		return err
	}
	if ph != head {
		return fail("PR の head（%s）が HEAD（%s）と違う。push する", ph[:12], head[:12])
	}
	s := ledger.SettingsOf(c.task)
	switch s.Review {
	case "on":
		tasks, err := c.l.List()
		if err != nil {
			return err
		}
		rv := c.l.CheckReview(c.l.Log(), tasks, c.task, head, s.MustFix, r.Waived)
		if rv.State != "current" {
			return fail("レビューが %s: %s", rv.State, rv.Reason)
		}
	case "off":
	default:
		return fail("task.yaml の review が on/off でない（%q）。Chief of Staff に書いてもらう", c.task.Str("review"))
	}
	out, err := c.l.Run("", "review", c.id)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}

var askedRe = regexp.MustCompile(`#([1-9][0-9]*)$`)

func (c *ctx) ask(stdin io.Reader, stdout io.Writer, src string) error {
	var b []byte
	var err error
	if src == "-" {
		b, err = io.ReadAll(stdin)
	} else {
		b, err = os.ReadFile(src)
	}
	if err != nil {
		return err
	}
	q, err := contract.ParseQuestion(b)
	if err != nil {
		return fail("問いの JSON の形が違う: %v", err)
	}
	args := []string{"ask", c.id, "-"}
	if k := q.LedgerKind(); k != "" {
		args = append(args, "--kind", k)
	}
	out, err := c.l.Run(q.Text(), args...)
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
	if err := contract.Write(filepath.Join(dir, m[1]+".json"), q); err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}
