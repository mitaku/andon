package issue

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"andon/internal/contract"
	"andon/internal/flock"
	"andon/internal/gate"
	"andon/internal/ledger"
)

// HoldUsage と UnholdUsage は andon issue hold・unhold の使い方（ADR-0056 決めたこと 7・8）。
const (
	HoldUsage = "usage: andon issue hold <id> <理由の課題>\n" +
		"課題の PR を止める（Chief of Staff が打つ）: 予約キー hold に、答えを待つ理由の課題の id を書く。\n" +
		"dispatcher のマージと同じ鍵と gate の鍵を取り、今の commit を渡していれば（gate が通したかオーナー待ちと判定した・印が緑）\n" +
		"PR の印 andon/gate を黄（pending）に書き換えてから返る。黄を書けない・PR を読めなければ hold を書いたまま終了コード 1。\n" +
		"止めている間、gate は回らず、dispatcher はマージしない。外すのは andon issue unhold だけ（理由の課題が done でも外れない）。\n"
	UnholdUsage = "usage: andon issue unhold <id>\n" +
		"止めを外す（Chief of Staff が打つ）: hold と同じ鍵を取り、予約キー hold を消す。印は書かない（次の巡で gate が回り直して書く）。\n" +
		"答えを担当へ伝え、答えのとおりに直ってから打つ。\n"
)

// Hold は andon issue hold。
func Hold(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("hold", HoldUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) != 1 {
			return errUsage
		}
		reason := ledger.CanonID(rest[0])
		if !idRe.MatchString(reason) {
			return errUsage
		}
		return withLockedIssue(id, func(c *ctx) error { return c.hold(stdout, reason) })
	})
}

// Unhold は andon issue unhold。
func Unhold(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return leaf("unhold", UnholdUsage, args, stderr, func(id string, rest []string) error {
		if len(rest) != 0 {
			return errUsage
		}
		return withLockedIssue(id, func(c *ctx) error { return c.unhold(stdout) })
	})
}

// lockGate は gate の鍵（library/.gate.lock。andon gate が記録を書く間持つもの）を取る。課題の鍵のあとに取る
// （dispatcher のマージは課題の鍵の中で gate を回すので、同じ順にして重ならないようにする）。
func (c *ctx) lockGate() (func(), error) {
	return flock.Lock(filepath.Join(c.l.Library(c.id), ".gate.lock"))
}

// hold は課題の鍵と gate の鍵の中で、予約キー hold を書き、今の commit を渡していれば印を黄にする（ADR-0056 決めたこと 8）。
// 理由の課題が無い・片付いている、止める課題が片付いているときは何も書かずに断る。
func (c *ctx) hold(stdout io.Writer, reason string) error {
	if c.issue.Str("status") == "done" {
		return fail("%s は片付いている（止める PR が無い）", c.id)
	}
	if reason == c.id {
		return fail("理由の課題に %s 自身は書けない（答えを待つ問いの課題を書く）", c.id)
	}
	r, err := c.l.Show(reason)
	if err != nil {
		return fail("理由の課題 %s が無い: %v", reason, err)
	}
	if r.Str("status") == "done" {
		return fail("理由の課題 %s はもう片付いている（答えを待つ問いの課題を書く）", reason)
	}
	unlock, err := c.lockGate()
	if err != nil {
		return err
	}
	defer unlock()
	if c.issue.Str("hold") != reason {
		if _, err := c.l.Run("", "set", c.id, "hold="+reason); err != nil {
			return fail("hold を書けない: %v", err)
		}
		if t, err := c.l.Show(c.id); err == nil {
			c.issue = t
		}
	}
	msg, err := c.holdMark()
	if err != nil {
		return fail("%s を止めた（hold=%s）が、%v。hold は書いたまま（dispatcher が次に PR を読むとき揃える）。オーナーにはまだ待つよう伝えない", c.id, reason, err)
	}
	fmt.Fprintf(stdout, "%s を止めた（理由の課題 %s）。%s\n", c.id, reason, msg)
	return nil
}

// holdMark は、止めた課題の PR の今の commit を渡していれば、印を黄に書き換える。PR が無ければ何も書かない。
// PR を読めない・印を書けなければ誤り（止まったことを確かめられない）。返すのは何をしたかの1文。
func (c *ctx) holdMark() (string, error) {
	rep, err := c.previous()
	if err != nil || rep == nil || rep.PR == "" {
		return "PR がまだ無いので印は書かない", nil
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok || "github.com/"+owner+"/"+name != c.issue.Str("repo") {
		return "report.json の pr がこの課題の repo の PR でないので印は書かない", nil
	}
	slug := owner + "/" + name
	state, head, _, have, err := prState(number, slug)
	if err != nil {
		return "", fmt.Errorf("PR を読めない（%v）", err)
	}
	if state != "OPEN" {
		return fmt.Sprintf("PR は %s なので印は書かない", state), nil
	}
	f := c.markFacts(head, have)
	f.Held = true
	m, write := gate.WantMark(f)
	if !write {
		if have.State == "pending" {
			return "PR の印はもう黄", nil
		}
		return "今の commit はまだ渡していないので印は書かない", nil
	}
	if err := postMark(slug, head, m); err != nil {
		return "", fmt.Errorf("PR の印を黄にできない（%v）", err)
	}
	return fmt.Sprintf("PR の印を黄にした（commit %s）", short12(head)), nil
}

// unhold は課題の鍵と gate の鍵の中で、予約キー hold を消す。印は書かない（ADR-0056 決めたこと 8）。
// 担当が消した hold（止めているものとして読む。決めたこと 10）も、Chief of Staff が消し直して外す。
func (c *ctx) unhold(stdout io.Writer) error {
	unlock, err := c.lockGate()
	if err != nil {
		return err
	}
	defer unlock()
	reason, held, err := c.l.Hold(c.l.Log(), c.issue)
	if err != nil {
		return fail("止めを読めない: %v", err)
	}
	if !held {
		fmt.Fprintf(stdout, "%s は止めていない\n", c.id)
		return nil
	}
	if _, err := c.l.Run("", "set", c.id, "hold="); err != nil {
		return fail("hold を消せない: %v", err)
	}
	fmt.Fprintf(stdout, "%s の止めを外した（理由の課題 %s）。印は次の巡で gate が書く\n", c.id, reason)
	return nil
}

// postMark は PR の head に印（commit status andon/gate）を書く（gh api。1回）。
func postMark(slug, head string, m gate.Mark) error {
	cmd := exec.Command("gh", "api", "-X", "POST", "repos/"+slug+"/statuses/"+head, "-f", "context="+gate.GateContext, "-f", "state="+m.State, "-f", "description="+m.Description)
	var errb strings.Builder
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v %s", err, firstLine(errb.String()))
	}
	return nil
}
