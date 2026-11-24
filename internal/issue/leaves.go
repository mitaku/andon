package issue

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

const (
	ShowUsage = "usage: andon issue show <id>\n" +
		"予約キー（既定で埋めたもの）・report.json・レビューの照合を JSON で出す（読むだけ）。\n"
	ReportUsage = "usage: andon issue report <id> [--pr <url>] [--data <json>] < report.md\n" +
		"report.json を書き（commit は HEAD）、stdin の Markdown で kanban に報告する。\n" +
		"--data は report.json の任意のキー（pr・estimate・plan・waived・acceptance・risk・next・observation・remember・agents_md・gaps）の JSON object。\n" +
		"--pr を省くと前の report.json の pr を、--data に estimate・plan・gaps が無ければ前のそれを引き継ぐ（gaps は空の配列で取り下げる）。\n" +
		"review_request は、HEAD が前の report.json の commit と同じ間だけ引き継ぐ（HEAD が進めば消える。頼み直しは review-request）。\n"
	ReviewRequestUsage = "usage: andon issue review-request <id> [< report.md]\n" +
		"report.json に review_request を足して報告する（レビューを頼む。回数・前回の commit は command が数える）。\n" +
		"stdin を省くと前の報告の本文にレビュー依頼の行を足す。渡せば置き換える。\n" +
		"レビューが要るかは gate と同じ条件で決める: review: on か、PR の差分がレベル B・C（review: off でも）。\n"
	ReadyUsage = "usage: andon issue ready <id>\n" +
		"HEAD・report.json・PR の head・レビューが揃えば kanban を review にする。\n" +
		"deliverable: candidates なら PR の代わりに candidates.json を確かめる。\n" +
		"レビューが要るかは gate と同じ条件で決める: review: on か、PR の差分がレベル B・C（review: off でも）。\n"
	AskUsage = "usage: andon issue ask <id> <question.json|->\n" +
		"       andon issue ask --new (--now | --later \"<理由>\" [--until <YYYY-MM-DD>]) [--parent <id>] [--after <id>,…] <question.json|->\n" +
		"問いの JSON を検査し、人向けの text を描いて kanban に問い、library/questions/<n>.json に置く。\n" +
		"--new はオーナーの課題を足し（タイトルは問いの文に「（オーナーの判断待ち）」、依頼書は問いの写し）、\n" +
		"who: person を書いて問いを置き、<id>#<n> を出す。--after を付けると課題は queued で作られ、その課題にも問いを置く。\n" +
		"--now（呼ぶ）か --later（置いておく。理由を later に、--until の日を later_until に書く）のどちらか 1 つを必ず付ける。\n" +
		"どちらも無い・両方ある・--until だけ・日付の形が違う・--later を Chief of Staff でない端末で打った、のどれでも何も足さない。\n" +
		"JSON の形が違うか kind が scope なら何も足さない。同じタイトルの片付いていない課題が問いを持たずにあれば、\n" +
		"足し直さずにその課題へ続きを置く（途中で落ちた回の打ち直し）。queued の課題に問いを置けるのはオーナーの課題（who: person）だけ。\n"
	CandidatesUsage = "usage: andon issue candidates <id> <candidates.json|->\n" +
		"候補の一覧を検査して library/candidates.json に書く（repo があれば commit は command が HEAD を書く）。\n"
	RecordUsage = "usage: andon issue record <id> [--expect pass|fail] -- <command> [<arg>…]\n" +
		"worktree で command を走らせ、記録を library/runs/<n>.json・<n>.log に書く。\n" +
		"期待は既定で pass（終了コード 0）。fail は誤りを戻すと落ちることを示す記録。\n" +
		"期待と違っても記録は書いて終了コード 1。起動できなければ書かない。\n"
	FinishUsage = "usage: andon issue finish <id>\n" +
		"端末と worktree を片付け、origin に入った branch を消して done にする（MERGED を確かめてから打つ）。\n" +
		"受け取られていない問いがあれば何も消さずに止まる。\n" +
		"links に write_back: true の出所の issue があれば、結果を1コメント書く。\n"
	SettleUsage = "usage: andon issue settle <id>\n" +
		"review・doing の課題の PR をオーナーがマージしていたら、gate の問いを閉じて片付ける。\n" +
		"PR ごとに1分に1回 gh pr view。積む通知を JSON で出す。2回打っても同じ。\n"
	MergeUsage = "usage: andon issue merge <id>\n" +
		"review の課題に gate を回し、approve ＋ merge: auto ＋ 委譲の範囲内ならマージして片付ける。\n" +
		"review に入った行ごとに1回。fix は担当へ、needs_human は問い。積む通知を JSON で出す。\n"
	ImpactUsage = "usage: andon issue impact <id>\n" +
		"マージした PR と、同じ repo の動いている課題の重なり・衝突を JSON で出す（読むだけ）。\n"
	StartUsage = "usage: andon issue start <id> [--dry-run [--json]]\n" +
		"担当を起動する: worktree と branch を切り、hc run で起動し、kanban start で結ぶ（Chief of Staff が打つ）。\n" +
		"--dry-run は確かめだけ通し、hc を呼ばず何も作らずに、渡すもの（最初のメッセージ・依頼書・ラベル・端末の名前・\n" +
		"worktree・branch・プロファイル・確かめの結果）を出す。\n"
	ResumeUsage = "usage: andon issue resume <id> [--fresh] [--dry-run [--json]]\n" +
		"止まった担当・Chief Engineer を続きから起動する（doing で端末が無い・終わった・素のシェル）。\n" +
		"worktree が無ければ戻し、<profile>-resume の template でセッションの続きから起動する。\n" +
		"記録が無い・最初のメッセージが届かなければ kanban を変えずに終了コード 1。\n"
	SessionUsage = "usage: andon issue session\n" +
		"自分の端末（HC_TERMINAL_ID）のラベル session を出す（template の command・resume の行が読む）。\n"
	StopCheckUsage = "usage: andon issue stop-check [<id>] < フックの入力の JSON\n" +
		"担当・レビュアーの Stop フック: doing で報告も問いも無ければ止める（contract.md §9）。\n" +
		"id を省くと端末のラベル issue= から引く。担当でなければ何も出さない。\n"
)

var errUsage = errors.New("使い方の誤り")

func leaf(sub, usage string, args []string, stderr io.Writer, body func(id string, rest []string) error) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	id, rest := ledger.CanonID(args[0]), args[1:]
	if !idRe.MatchString(id) {
		fmt.Fprintf(stderr, "andon issue %s: id が不正: %q\n", sub, id)
		return exitUsage
	}
	err := body(id, rest)
	if errors.Is(err, errUsage) {
		fmt.Fprint(stderr, usage)
		return exitUsage
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

// guard は定義（process/states/issue.yaml）に無い移り方を、何かを書く前に断る（ADR-0017 決めたこと 15）。
// start は自分の確かめの順の中で見る（start.go。--dry-run の確かめの一覧に載せる）。
func guard(sub string, f func(*ctx) error) func(*ctx) error {
	return func(c *ctx) error {
		if err := allowed(sub, c.id, c.issue.Str("status")); err != nil {
			return err
		}
		return f(c)
	}
}

func Show(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("show", ShowUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withIssue(id, func(c *ctx) error { return c.show(stdout) })
	})
}

func Report(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return leaf("report", ReportUsage, args, stderr, func(id string, rest []string) error {
		var pr, data string
		for len(rest) > 0 {
			if len(rest) < 2 || (rest[0] != "--pr" && rest[0] != "--data") {
				return errUsage
			}
			if rest[0] == "--pr" {
				pr = rest[1]
			} else {
				data = rest[1]
			}
			rest = rest[2:]
		}
		return withIssue(id, guard("report", func(c *ctx) error { return c.report(stdin, stdout, pr, data) }))
	})
}

func ReviewRequest(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return leaf("review-request", ReviewRequestUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withIssue(id, guard("review-request", func(c *ctx) error { return c.reviewRequest(stdin, stdout) }))
	})
}

func Ready(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("ready", ReadyUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withIssue(id, guard("ready", func(c *ctx) error { return c.ready(stdout) }))
	})
}

func Ask(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "--new" {
		return askNew(args[1:], stdin, stdout, stderr)
	}
	return leaf("ask", AskUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) != 1 {
			return errUsage
		}
		return withIssue(id, guard("ask", func(c *ctx) error {
			if err := c.askQueued(); err != nil {
				return err
			}
			return c.ask(stdin, stdout, rest[0])
		}))
	})
}

func askNew(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var parent string
	var after []string
	var now, later, until bool
	var mark ledger.Mark
	for len(args) > 1 && strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "--parent":
			parent = ledger.CanonID(args[1])
		case "--after":
			for _, a := range strings.Split(args[1], ",") {
				after = append(after, ledger.CanonID(strings.TrimSpace(a)))
			}
		case "--now":
			if now {
				return askNewRefuse(stderr, "--now が 2 つある")
			}
			now = true
			args = args[1:]
			continue
		case "--later":
			if later {
				return askNewRefuse(stderr, "--later が 2 つある")
			}
			later, mark.Later = true, args[1]
		case "--until":
			if until {
				return askNewRefuse(stderr, "--until が 2 つある")
			}
			until, mark.Until = true, args[1]
		default:
			fmt.Fprint(stderr, AskUsage)
			return exitUsage
		}
		args = args[2:]
	}
	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		fmt.Fprint(stderr, AskUsage)
		return exitUsage
	}
	switch {
	case until && !later:
		return askNewRefuse(stderr, "--until は --later と一緒のときだけ付ける")
	case now == later:
		return askNewRefuse(stderr, "呼ぶか置いておくかを言い切る: --now か --later \"<理由>\" のどちらか 1 つを付ける")
	case later && (strings.TrimSpace(mark.Later) == "" || strings.ContainsAny(mark.Later, "\r\n")):
		return askNewRefuse(stderr, "--later の理由は空でない 1 行にする")
	}
	if until {
		if _, err := time.Parse("2006-01-02", mark.Until); err != nil {
			return askNewRefuse(stderr, fmt.Sprintf("--until の日付は YYYY-MM-DD にする: %q", mark.Until))
		}
	}
	ids := slices.Clone(after)
	if parent != "" {
		ids = append(ids, parent)
	}
	for _, id := range ids {
		if !idRe.MatchString(id) {
			fmt.Fprintf(stderr, "andon issue ask: id が不正: %q\n", id)
			return exitUsage
		}
	}
	err := func() error {
		b, err := readInput(stdin, args[0])
		if err != nil {
			return err
		}
		q, err := contract.ParseQuestion(b)
		if err != nil {
			return fail("問いの JSON の形が違う: %v", err)
		}
		l, err := ledger.Open()
		if err != nil {
			return err
		}
		// 印を信じるのは Chief of Staff の端末が書いたときだけ。ほかの端末の印は黙って書かず、何も足さずに断る（ADR-0062 決めたこと 10）。
		if later {
			if cos, me := l.ChiefOfStaff(), os.Getenv("HC_TERMINAL_ID"); cos == "" || me != cos {
				return fail("--later で印を書くのは Chief of Staff の端末だけ（config.yaml の chief_of_staff: %q・HC_TERMINAL_ID: %q）。呼ぶなら --now", cos, me)
			}
		}
		_, out, err := l.NewOwnerQuestion(q, parent, after, mark)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, out)
		return nil
	}()
	if err != nil {
		fmt.Fprintf(stderr, "andon issue ask: %v\n", err)
		return exitFail
	}
	return 0
}

func askNewRefuse(stderr io.Writer, why string) int {
	fmt.Fprintf(stderr, "andon issue ask --new: %s\n", why)
	return exitUsage
}

func Candidates(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return leaf("candidates", CandidatesUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) != 1 {
			return errUsage
		}
		return withIssue(id, guard("candidates", func(c *ctx) error { return c.candidates(stdin, stdout, rest[0]) }))
	})
}

func Record(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("record", RecordUsage, args, stderr, func(id string, rest []string) error {
		expect := "pass"
		if len(rest) >= 2 && rest[0] == "--expect" {
			expect, rest = rest[1], rest[2:]
		}
		if (expect != "pass" && expect != "fail") || len(rest) < 2 || rest[0] != "--" {
			return errUsage
		}
		argv := rest[1:]
		if slices.Contains(argv, "") {
			return errUsage
		}
		return withIssue(id, guard("record", func(c *ctx) error { return c.record(stdout, expect, argv) }))
	})
}

func Finish(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("finish", FinishUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withLockedIssue(id, guard("finish", func(c *ctx) error {
			if err := c.finish(stdout); err != nil {
				return err
			}
			c.writeBack(stdout) // done になってから。書けなくても finish は戻さない（ADR-0011 決めたこと 15）
			return nil
		}))
	})
}

func Settle(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("settle", SettleUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withLockedIssue(id, func(c *ctx) error { return c.settle(stdout, stderr, time.Now()) })
	})
}

func Merge(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("merge", MergeUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withLockedIssue(id, func(c *ctx) error { return c.merge(stdout, stderr, time.Now()) })
	})
}

func Impact(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("impact", ImpactUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) > 0 {
			return errUsage
		}
		return withIssue(id, func(c *ctx) error { return c.impact(stdout) })
	})
}

func Start(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("start", StartUsage, args, stderr, func(id string, rest []string) error {
		var dry, asJSON bool
		for _, a := range rest {
			switch {
			case a == "--dry-run" && !dry:
				dry = true
			case a == "--json" && !asJSON:
				asJSON = true
			default:
				return errUsage
			}
		}
		if asJSON && !dry {
			return errUsage
		}
		with := withLockedIssue
		if dry {
			with = withIssue
		}
		return with(id, func(c *ctx) error { return c.start(stdout, stderr, dry, asJSON) })
	})
}

func Resume(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("resume", ResumeUsage, args, stderr, func(id string, rest []string) error {
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
				return errUsage
			}
		}
		if asJSON && !dry {
			return errUsage
		}
		with := withLockedIssue
		if dry {
			with = withIssue
		}
		return with(id, func(c *ctx) error { return c.resume(stdout, stderr, dry, asJSON, fresh) })
	})
}

func Session(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprint(stderr, SessionUsage)
		return exitUsage
	}
	return session(stdout, stderr)
}

func StopCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return stopCheck(args, stdin, stdout, stderr)
}
