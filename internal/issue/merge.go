package issue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/dispatcher"
	"andon/internal/gate"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// merge は andon issue merge: review の課題に gate を回し（review に入った log.jsonl の行ごとに1回）、approve ＋ merge: auto ＋
// 委譲の範囲内ならマージして片付ける。fix は理由を担当へ送り、needs_human は問い（from: gate）を出す（dispatcher.md §5 の P6a）。
// 積む通知は stdout の JSON で返す（積むのは dispatcher）。2回打っても同じ結果になるよう、行ごとに決めたことを自分の記録に持つ
// （ADR-0017 決めたこと 5・6）。記録は dispatcher も読む（approve のまま 10 分。dispatcher.md §3.2）。
func (c *ctx) merge(stdout, stderr io.Writer, now time.Time) error {
	path := dispatcher.MergePath(c.l.Slug)
	rec := map[string]contract.Merging{}
	fresh := false
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &rec); err != nil {
			return fail("merge の記録が読めない（%s）: %v", path, err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		fresh = true
	} else {
		return err
	}
	if err := forgetOthers(c, rec, "review"); err != nil {
		return err
	}
	res := contract.Merge{Issue: c.id, State: "skip", Log: []string{}, Notify: []contract.Notice{}}
	w := c.mergeOne(rec[c.id], fresh, &res, stderr, now)
	if w == nil {
		delete(rec, c.id)
	} else {
		rec[c.id] = *w
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := contract.WriteState(path, rec); err != nil {
		return fail("merge の記録を書けない: %v", err)
	}
	_, err := stdout.Write(contract.Marshal(res))
	return err
}

func forgetOthers[T any](c *ctx, rec map[string]T, keep ...string) error {
	others := false
	for id := range rec {
		others = others || id != c.id
	}
	if !others {
		return nil
	}
	tasks, err := c.l.List()
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	for _, t := range tasks {
		kept[t.Str("id")] = slices.Contains(keep, t.Str("status"))
	}
	for id := range rec {
		if id != c.id && !kept[id] {
			delete(rec, id)
		}
	}
	return nil
}

func (c *ctx) mergeOne(w contract.Merging, fresh bool, res *contract.Merge, stderr io.Writer, now time.Time) *contract.Merging {
	if c.issue.Str("status") != "review" {
		return nil
	}
	rows, logErr := c.l.ReadLog()
	row := 0
	for i, r := range rows {
		if r.ID == c.id && r.Change["status"] == "review" {
			row = i + 1
		}
	}
	rep, err := c.previous()
	if row == 0 || err != nil || rep == nil {
		return nil
	}
	if rep.PR == "" {
		// 報告の課題（deliverable: report を書いた時点の Chief of Staff が書いた課題。検証・点検・マージ後のレビュー）は PR が無くてよい。
		// 報告の commit を見ずに gate を回さず、fix を送らず、merge の記録にも書かない（ADR-0066 決めたこと 7）。
		// PR の課題（照合の外れた report を含む）は、PR が無く、報告の commit が既定の branch に入っていて push するものが無いときだけ
		// 同じに回さない（直す課題が直さずに返した。PR #630）。自分の commit があるのに PR が無い課題は、これまでどおり gate が fix を返す。
		// どちらも返ったことは dispatcher の collect が通知 report で Chief of Staff に届ける
		if form, _ := c.l.Deliverable(c.issue); form == "report" || c.onDefaultBranch(rep.Commit) {
			return nil
		}
	}
	if reason, held, err := c.l.Hold(rows, logErr, c.issue); err != nil || held {
		if w.Row != row {
			w = contract.Merging{Row: row}
		}
		c.holdDecision(&w, reason, err, res)
		return &w
	}
	if w.Row == row && RegateWatched(w) {
		if reason, ok := c.heldAfterDecision(rows, row, w.Sha); ok {
			c.discardForHold(&w, reason, res)
		}
	}
	lost := false
	if w.Row != row {
		w = contract.Merging{Row: row}
		if fresh {
			if g := c.gatedSince(rep.Commit, rows[row-1].At); g != nil {
				_, held := c.heldAfterDecision(rows, row, g.Head)
				switch {
				case held && g.Decision != "fix":
				case g.Decision != "approve" || c.decidedApprove(rows, row, g):
					w.Sha, w.Decision = g.Head, g.Decision
					return &w
				default:
					w.Sha, lost = g.Head, true
				}
			}
		}
	}
	prev, prevDone := c.stackedPrevDone(rows)
	prevFails := 0
	if w.Decision != "" {
		if w.Merged == "" && contract.IsSHA(w.Sha) {
			c.closeStaleGateQuestions(w.Sha, res)
		}
		due := pollDue(w.Polled, now) // headMoved が聞けば Polled を今にするので、先に読む
		if !c.headMoved(&w, rep, res, now) {
			// 前の PR 待ちのあと前の課題が done になったら、同じ head でも 1 回だけ gate を回し直す（ADR-0067 決めたこと 12・14）。
			// gate が、前の課題が main に入っていれば載せ替えを頼む直しを、マージされずに終わっていれば理由を替えた behind を決める
			if w.Decision != "behind" || !prevDone || w.Prev != "" {
				return &w
			}
			// 判定できないまま prevTries 回落ちて知らせたあとは、1 分に 1 回だけ回し直す（GitHub が戻れば載せ替えの直しが届く）
			if w.PrevFails >= prevTries && !due {
				return &w
			}
			// open でない PR（前の課題の branch へマージされた・閉じた）は回し直さない。片付けの道が Chief of Staff に知らせる
			// （担当に直しを送らない。決めたこと 15。モデルの回し直しも open の PR だけ）
			open, err := c.prOpen(rep)
			if err != nil {
				res.Log = append(res.Log, fmt.Sprintf("%s の前の課題 %s が done になったが、PR が open かを読めない（次の巡で聞き直す）: %v", c.id, prev, err))
				return &w
			}
			if !open {
				res.Log = append(res.Log, fmt.Sprintf("%s の前の課題 %s が done になったが、PR が open でないので gate を回し直さない（片付けの道に任せる）", c.id, prev))
				w.Prev = prev
				return &w
			}
			res.Log = append(res.Log, fmt.Sprintf("%s の前の課題 %s が done になった。前の PR 待ちの gate を回し直す", c.id, prev))
			prevFails = w.PrevFails
		}
		w = contract.Merging{Row: row}
	}
	if prevDone {
		w.Prev = prev
	}
	if w.Wait != "" && c.stillWaiting(rows, rep.PR, w.Wait, now) {
		res.State = "wait"
		res.Log = append(res.Log, fmt.Sprintf("%s は GitHub の checks を待つ（gate は回さない）: head %s の checks がまだ揃っていない", c.id, short12(w.Wait)))
		return &w
	}
	w.Wait = ""
	if (w.Retries > 0 || lost) && c.mergedWhileRetrying(&w, rep, res, now) {
		return &w
	}
	var out, errb bytes.Buffer
	code := gate.Main([]string{c.id}, nil, &out, &errb)
	if code == 3 && strings.Contains(strings.ToLower(errb.String()), "rate limit") {
		res.State = "rate-limited"
		return &w
	}
	var gr gateResult
	if (code != 0 && code != 1 && code != 4 && code != 5 && code != 6) || json.Unmarshal(out.Bytes(), &gr) != nil {
		w.Approved, w.Retries = "", 0
		why := fmt.Sprintf("andon issue gate %s: 終了コード %d %s", c.id, code, firstLine(errb.String()))
		res.Log = append(res.Log, fmt.Sprintf("%s の gate が判定できない: %s", c.id, why))
		// 積んだ課題では、判定できないときも前の PR 待ちに置く（オーナーに押せないマージの問いを出さない。ADR-0067 決めたこと 8）。
		// 記録は無いので、報告の commit を判定した head として控え、head が動けば回し直す
		if prev, _, ok := ledger.Stacked(rows, c.issue, c.l.ChiefOfStaff()); ok {
			w.Decision, res.State = "behind", "behind"
			if contract.IsSHA(rep.Commit) {
				w.Sha = rep.Commit
			}
			w.Polled = now.UTC().Format("2006-01-02T15:04:05Z")
			// 前の課題が done になってからの gate が落ちた（前の課題の PR を一時に読めないなど）: 印を立てずに次の巡で回し直す。
			// prevTries 回目に落ちたら 1 回だけ知らせ、そのあとも 1 分に 1 回回し直す（載せ替えの直しが黙って届かないままにしない）
			if w.Prev != "" {
				w.Prev, w.PrevFails = "", prevFails+1
				if w.PrevFails == prevTries {
					res.Notify = append(res.Notify, contract.Notice{Kind: "gate", Detail: map[string]any{"decision": "behind",
						"reasons": []string{PrevUnreadReason(prev), "gate が判定できない: " + why}}})
				}
				return &w
			}
			res.Notify = append(res.Notify, contract.Notice{Kind: "gate",
				Detail: map[string]any{"decision": "behind", "reasons": []string{gate.BehindReason(prev), "gate が判定できない: " + why}}})
			return &w
		}
		w.Decision, res.State = "needs_human", "needs_human"
		c.askGate(res, GateFailedQuestion(c.id, c.issue.Str("title"), why))
		return &w
	}
	if gr.Decision == "wait" {
		res.State = "wait"
		res.Log = append(res.Log, fmt.Sprintf("%s の gate は GitHub の checks を待つ: %s", c.id, strings.Join(gr.Reasons, "／")))
		if contract.IsSHA(gr.Head) {
			w.Wait = gr.Head
		}
		return &w
	}
	record := ""
	if contract.IsSHA(gr.Head) {
		c.closeStaleGateQuestions(gr.Head, res)
		record = filepath.Join(c.l.Library(c.id), "gate", gr.Head+".json")
		res.Auto = gr.Decision == "approve" && c.autoMerge(rows, gr.Delegation.OK)
		res.Notify = append(res.Notify, contract.Notice{Kind: "gate",
			Detail: map[string]any{"decision": gr.Decision, "reasons": nonNil(gr.Reasons), "record": record}})
	}
	switch gr.Decision {
	case "approve":
		if res.Auto && w.Approved == "" {
			w.Approved = now.UTC().Format("2006-01-02T15:04:05Z")
		}
		if c.approve(rep, gr, record, &w, res, stderr) {
			w.Sha, res.State = gr.Head, "regate"
			return &w
		}
	case "fix":
		c.sendFix(gr, record, res)
	case "behind":
		// 前の PR 待ち（ADR-0067 決めたこと 8）: マージもオーナーへの問いもしない。通知 gate（上で積んだ）が Chief of Staff に届き、
		// 枠が空いたことを知らせる。head が動くか止められたら、approve と同じに判定を捨てて回し直す（RegateWatched）
	default:
		kind := "irreversible"
		if gr.Tier == "C" {
			kind = "line"
		}
		facts := append([]string{}, gr.Reasons...)
		if rep.PR != "" {
			facts = append(facts, "PR "+rep.PR)
		}
		if record != "" {
			facts = append(facts, gate.RecordFact(record))
		}
		c.askGate(res, contract.Question{
			Kind: kind, Question: fmt.Sprintf("%s（%s）の PR は gate がオーナーへ回した。マージするか", c.id, short(c.issue.Str("title"))),
			Facts: facts, Reason: "gate の決定式が needs_human を返した（委譲・レベル・オーナーが見る PR の事情）", From: "gate",
			Options: []contract.Option{{Label: "オーナーが PR を読んでマージする", Pick: ptr(true)}, {Label: "担当に直させる（理由を Chief of Staff が返す）"}}})
	}
	w.Sha, w.Decision, w.Approved, w.Retries = gr.Head, gr.Decision, "", 0
	w.Polled = now.UTC().Format("2006-01-02T15:04:05Z")
	res.State = gr.Decision
	if w.Merged != "" {
		res.State = "merged"
	}
	return &w
}

// PrevUnreadReason は、前の課題が done になってからの gate が続けて判定できないときに Chief of Staff へ知らせる理由の 1 行目。
func PrevUnreadReason(prev string) string {
	return "前の課題 " + prev + " が done になったが、main に入ったかを gate が判定できない（1 分に 1 回回し直す）"
}

// pollDue は、Polled（PR を最後に聞いた時刻）から prPoll たったか。読めなければたったと読む。
func pollDue(polled string, now time.Time) bool {
	at, err := time.Parse("2006-01-02T15:04:05Z", polled)
	return err != nil || now.Sub(at) >= prPoll
}

// prOpen は、報告の PR が GitHub で open か。PR が名指せない（報告に pr が無い・形が違う）ときは open と読み、gate に任せる。
func (c *ctx) prOpen(rep *contract.Report) (bool, error) {
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok {
		return true, nil
	}
	state, _, err := prOpenHead(number, owner+"/"+name)
	return state == "OPEN", err
}

// prevTries は、前の課題が done になってからの gate の回し直しを、判定できないまま打ち直す回数の上限。
const prevTries = 3

// stackedPrevDone は、課題が積んだ課題（信じられる stacked_on がある）で、前の課題が kanban で done かを読む（ADR-0067 決めたこと 12）。
// 前の課題が読めなければ done でないと読む（gate が前の課題を読めずに誤りを返し、前の PR 待ちのまま置く）。
func (c *ctx) stackedPrevDone(rows []ledger.Row) (string, bool) {
	on, _, ok := ledger.Stacked(rows, c.issue, c.l.ChiefOfStaff())
	if !ok {
		return "", false
	}
	p, err := c.l.Show(on)
	return on, err == nil && p.Str("status") == "done"
}

// holdDecision は、Chief of Staff が止めている課題（予約キー hold）の回（ADR-0056 決めたこと 9）: gate を回さず、GitHub も読まない。
// gate が通したかオーナー待ちと判定したあと（dispatcher がマージしていない）に止められたら、PR の head が動いたときと同じに
// 判定を捨て、gate の問い（from: gate）を閉じる（止めている PR のマージをオーナーに頼まない）。外したら次の巡で gate から回し直す。
// 差し戻し（fix）は担当が直して ready し直すので捨てない。止めを読めなければ止めているものとして待つ。
func (c *ctx) holdDecision(w *contract.Merging, reason string, err error, res *contract.Merge) {
	res.State = "hold"
	if err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の止めを読めない（止めているものとして gate を回さない）: %v", c.id, err))
		return
	}
	c.discardForHold(w, reason, res)
}

// discardForHold は、approve か needs_human か behind を決めたあとに止められた判定を捨て、gate の問いを閉じる（ADR-0056 決めたこと 9）。
// 止めている間の巡（holdDecision）と、止めてから外すまでに dispatcher が一度も見なかった巡（heldAfterDecision）が呼ぶ。
func (c *ctx) discardForHold(w *contract.Merging, reason string, res *contract.Merge) {
	if !RegateWatched(*w) {
		return
	}
	why := fmt.Sprintf("Chief of Staff が PR を止めた（理由の課題 %s）。外したら gate が今の head で回し直す", reason)
	res.Log = append(res.Log, fmt.Sprintf("%s は gate の判定（%s）のあとに止められた。判定を捨てて gate の問いを閉じる: %s", c.id, w.Decision, why))
	c.closeGateQuestions(why, res)
	*w = contract.Merging{Row: w.Row}
}

func (c *ctx) heldAfterDecision(rows []ledger.Row, row int, sha string) (string, bool) {
	reason, at, ok := ledger.LastHoldSet(rows, c.id, row)
	if !ok {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "gate", sha+".json"))
	var g gate.Record
	if err != nil || json.Unmarshal(b, &g) != nil {
		return reason, true
	}
	decided, err1 := time.Parse(time.RFC3339, g.At)
	held, err2 := time.Parse(time.RFC3339, at)
	if err1 != nil || err2 != nil {
		return reason, true
	}
	return reason, !held.Before(decided)
}

func (c *ctx) mergedWhileRetrying(w *contract.Merging, rep *contract.Report, res *contract.Merge, now time.Time) bool {
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok || "github.com/"+owner+"/"+name != c.issue.Str("repo") {
		return false
	}
	out, err := gh("pr", "view", number, "--repo", owner+"/"+name, "--json", "state,mergeCommit,headRefOid")
	if err != nil {
		return false
	}
	sha, ok := MergedBefore(out, w.Sha)
	if !ok {
		return false
	}
	res.Log = append(res.Log, fmt.Sprintf("%s の PR %s は打ち直しを待つ間に同じ head でマージされていた（%s）。gate を回さずオーナーのマージとして片付ける", c.id, rep.PR, short12(sha)))
	w.Decision, w.Approved, w.Retries, res.State = "approve", "", 0, "approve"
	w.Polled = now.UTC().Format("2006-01-02T15:04:05Z")
	return true
}

// headMoved は、gate が approve か needs_human を決めたあと（dispatcher がマージしていない）に、PR の head が判定した head から
// 動いたかを見る。GitHub に聞くのは課題ごとに1分に1回（gh pr view を1回。聞いた時刻は記録の polled）。動いていれば
// 前の head の gate の問い（from: gate）を閉じて true を返す（呼び手が決めたことを捨て、gate を回し直す。dispatcher.md §5 の P6a）。
// fix は担当が doing に戻って ready し直すので見ない。読めなければ次に聞く巡まで待つ（gh の上限なら rate-limited）。
func (c *ctx) headMoved(w *contract.Merging, rep *contract.Report, res *contract.Merge, now time.Time) bool {
	if !RegateWatched(*w) {
		return false
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok || "github.com/"+owner+"/"+name != c.issue.Str("repo") {
		return false
	}
	if at, err := time.Parse("2006-01-02T15:04:05Z", w.Polled); err == nil && now.Sub(at) < prPoll {
		return false
	}
	w.Polled = now.UTC().Format("2006-01-02T15:04:05Z")
	state, head, err := prOpenHead(number, owner+"/"+name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "rate limit") {
			res.State = "rate-limited"
		}
		res.Log = append(res.Log, fmt.Sprintf("%s の PR の head が読めない（次に聞く巡で聞き直す）: %v", c.id, err))
		return false
	}
	if !HeadMoved(*w, state, head) {
		return false
	}
	res.Log = append(res.Log, fmt.Sprintf("%s の PR %s の head が gate の判定（%s）のあとに動いた（%s → %s）。gate を回し直す",
		c.id, rep.PR, w.Decision, short12(w.Sha), short12(head)))
	c.closeGateQuestions(fmt.Sprintf("PR の head が gate のあとに動いた（%s → %s）。gate が今の head で回し直す", short12(w.Sha), short12(head)), res)
	return true
}

// RegateWatched は、決めたことのある merge の記録 w が、PR の head が動いたら gate を回し直す対象か（純粋関数）。
// approve か needs_human か behind（前の PR 待ち。ADR-0067 決めたこと 8）で、dispatcher がマージしておらず、判定した head が sha のとき。
func RegateWatched(w contract.Merging) bool {
	return (w.Decision == "approve" || w.Decision == "needs_human" || w.Decision == "behind") && w.Merged == "" && contract.IsSHA(w.Sha)
}

// HeadMoved は、聞いた PR の state と head が、gate の判定した head から動いた（まだ open の PR に push された）ことを示すか（純粋関数）。
// MERGED・CLOSED は見ない（オーナーのマージと閉じた PR は settle の道）。
func HeadMoved(w contract.Merging, state, head string) bool {
	return RegateWatched(w) && state == "OPEN" && contract.IsSHA(head) && head != w.Sha
}

func (c *ctx) closeGateQuestions(why string, res *contract.Merge) {
	gateQs, _, err := c.openQuestions()
	if err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の gate の問いを読めない: %v", c.id, err))
		return
	}
	for _, q := range gateQs {
		name := fmt.Sprintf("%s#%d", c.id, q.N)
		if err := c.closeQuestion(name, q.Status, why); err != nil {
			res.Log = append(res.Log, fmt.Sprintf("%s の gate の問い %s を閉じられない: %v", c.id, name, err))
		}
	}
}

// closeQuestion は問い name（一覧を読んだときの状態 status）を、open なら why と答えて受け取る。一覧を読んだあとに
// オーナーや Chief of Staff が答えていれば answer は断られるが、答えのある問いは受け取れるので、そのまま received に進む
// （オーナーの答えは上書きしない）。受け取れなければ誤りを返す（呼び手は次に打たれたときに閉じ直す）。
// 答えは command が事実を見て書くので、印は machine（ADR-0049 決めたこと 6）。
func (c *ctx) closeQuestion(name, status, why string) error {
	var answerErr error
	if status == "open" {
		_, answerErr = c.l.Run("", "answer", "--decided-by", "machine", "--", name, why)
	}
	if _, err := c.l.Run("", "received", name); err != nil {
		if answerErr != nil {
			return fmt.Errorf("%v（answer: %v）", err, answerErr)
		}
		return err
	}
	return nil
}

func (c *ctx) closeStaleGateQuestions(head string, res *contract.Merge) {
	gateQs, _, err := c.openQuestions()
	if err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の gate の問いを読めない: %v", c.id, err))
		return
	}
	for _, q := range gateQs {
		old := c.questionHead(q.N)
		if old == "" || old == head {
			continue
		}
		name := fmt.Sprintf("%s#%d", c.id, q.N)
		why := fmt.Sprintf("PR の head が動いた（%s → %s）。gate が今の head で決め直した（記録 %s）",
			short12(old), short12(head), filepath.Join(c.l.Library(c.id), "gate", head+".json"))
		if err := c.closeQuestion(name, q.Status, why); err != nil {
			res.Log = append(res.Log, fmt.Sprintf("%s の gate の問い %s を閉じられない（次に打たれたときに閉じ直す）: %v", c.id, name, err))
			continue
		}
		res.Log = append(res.Log, fmt.Sprintf("%s の前の head（%s）の gate の問い %s を閉じた: %s", c.id, short12(old), name, why))
	}
}

func prOpenHead(number, slug string) (state, head string, err error) {
	out, err := gh("pr", "view", number, "--repo", slug, "--json", "state,headRefOid")
	if err != nil {
		return "", "", err
	}
	var v struct {
		State      string `json:"state"`
		HeadRefOid string `json:"headRefOid"`
	}
	if json.Unmarshal([]byte(out), &v) != nil {
		return "", "", fmt.Errorf("gh pr view の返事が読めない")
	}
	return v.State, v.HeadRefOid, nil
}

func (c *ctx) gatedSince(head, since string) *gate.Record {
	if !contract.IsSHA(head) {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "gate", head+".json"))
	var g gate.Record
	if err != nil || json.Unmarshal(b, &g) != nil || g.Decision == "" || g.At < since {
		return nil
	}
	return &g
}

// gateResult は andon issue gate の出力のうち merge が読むもの（記録 contract.md §7。信じられる PR が無いときは decision・reasons だけ）。
type gateResult struct {
	Decision   string   `json:"decision"`
	Restack    bool     `json:"restack"` // 載せ替えを頼む直し（ADR-0067 決めたこと 12）
	Reasons    []string `json:"reasons"`
	Head       string   `json:"head"`
	Tier       string   `json:"tier"`
	PR         string   `json:"pr"`
	Delegation struct {
		Merge string `json:"merge"`
		OK    bool   `json:"ok"`
	} `json:"delegation"`
}

func (c *ctx) autoMerge(rows []ledger.Row, delegated bool) bool {
	return ledger.SettingsOf(c.issue).Merge == "auto" && c.l.Trusted(rows, c.issue, "merge") && delegated
}

func (c *ctx) decidedApprove(rows []ledger.Row, row int, g *gate.Record) bool {
	if g.Merged || !c.autoMerge(rows, g.Delegation.OK) {
		return true
	}
	for _, r := range rows[row:] {
		n, ok := r.Change["question"].(float64)
		if r.ID != c.id || !ok || r.Change["status"] != "open" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "questions", strconv.Itoa(int(n))+".json"))
		var q struct {
			From  string   `json:"from"`
			Facts []string `json:"facts"`
		}
		if err != nil || json.Unmarshal(b, &q) != nil || q.From != "gate" {
			continue
		}
		if h := gate.QuestionHead(q.Facts); h == g.Head || h == "" {
			return true
		}
	}
	return false
}

func (c *ctx) stillWaiting(rows []ledger.Row, pr, waited string, now time.Time) bool {
	owner, name, number, ok := contract.ParsePR(pr)
	if !ok {
		return false
	}
	head, checks, err := gate.ReadChecks(number, owner+"/"+name)
	if err != nil || head != waited {
		return false
	}
	switch checks.State {
	case "pending":
		return true
	case "none":
		table, _, err := gate.LoadTable(pkgHome(), delegate.Workspace())
		return err == nil && !gate.ChecksOverdue(rows, c.id, now.UTC(), *table.GitHubChecksWait)
	}
	return false
}

func (c *ctx) sendFix(gr gateResult, record string, res *contract.Merge) {
	agent := c.issue.Str("agent")
	if agent == "" {
		res.Log = append(res.Log, fmt.Sprintf("%s の gate が fix だが担当（agent）がいない（Chief of Staff に届ける）", c.id))
		c.markGate(res, "no_agent", true)
		return
	}
	msg := fmt.Sprintf("%s（%s）: gate が fix を返した。理由: %s。", c.id, short(c.issue.Str("title")), strings.Join(gr.Reasons, "／"))
	if record != "" {
		msg += "記録は " + record + "。"
	}
	if gr.Restack {
		// 載せ替えの直し（ADR-0067 決めたこと 12・13）: 検査やレビューより先に、自分の commit を既定の branch の先へ移す
		msg += "kanban start で doing に戻して andon issue restack で既定の branch の先へ載せ替え、検査を通して andon issue review-request（レビューが引き継がれないとき）か andon issue submit からやり直す。"
	} else {
		msg += "kanban start で doing に戻して直し、push して andon issue review-request（レビューが要るとき）か andon issue submit からやり直す。"
	}
	// 先頭に送り手の印を置く（端末の外から送るので hc の包みが付かない。spawn.Mark）
	if out, err := exec.Command("hc", "send", agent, spawn.Mark(spawn.FromGate, msg)).CombinedOutput(); err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の担当 %s に fix を送れない（Chief of Staff に届ける）: %v %s", c.id, agent, err, firstLine(string(out))))
		c.markGate(res, "unsent", agent)
	}
}

func (c *ctx) markGate(res *contract.Merge, key string, v any) {
	for i := len(res.Notify) - 1; i >= 0; i-- {
		if res.Notify[i].Kind == "gate" {
			res.Notify[i].Detail[key] = v
			return
		}
	}
	res.Notify = append(res.Notify, contract.Notice{Kind: "report", Detail: map[string]any{"report": filepath.Join(c.l.Dir, "issues", c.id, "report.md")}})
}

func (c *ctx) approve(rep *contract.Report, gr gateResult, record string, w *contract.Merging, res *contract.Merge, stderr io.Writer) (regate bool) {
	if !res.Auto {
		return false
	}
	logf := func(format string, a ...any) { res.Log = append(res.Log, c.id+" "+fmt.Sprintf(format, a...)) }
	stop := func(why string) {
		logf("をマージしない: %s", why)
		c.askGate(res, contract.Question{
			Kind: "irreversible", Question: fmt.Sprintf("%s（%s）は gate が approve したが、dispatcher がマージできなかった。オーナーがマージするか", c.id, short(c.issue.Str("title"))),
			Facts: []string{why, "PR " + rep.PR, gate.RecordFact(record)}, From: "gate",
			Options: []contract.Option{{Label: "オーナーが PR を読んでマージする", Pick: ptr(true)}, {Label: "Chief of Staff が原因を直してから回し直す"}}})
	}
	table, warn, err := gate.LoadTable(pkgHome(), delegate.Workspace())
	if err != nil {
		stop("条件表が読めない: " + err.Error())
		return false
	}
	if warn != "" {
		res.Log = append(res.Log, warn)
	}
	repo := c.issue.Str("repo")
	method := table.Repos[repo].Merge
	if method == "" {
		stop(fmt.Sprintf("条件表の repos.%s.merge（マージの方式）が無い", repo))
		return false
	}
	owner, name, number, ok := contract.ParsePR(gr.PR)
	switch {
	case !ok || !contract.IsSHA(gr.Head):
		stop("gate の記録の pr・head が読めない")
		return false
	case gr.PR != rep.PR:
		stop(fmt.Sprintf("gate が判定した PR（%s）と report.json の pr（%s）が違う", gr.PR, rep.PR))
		return false
	case "github.com/"+owner+"/"+name != repo:
		stop(fmt.Sprintf("gate が判定した PR の repo が issue.yaml の repo（%s）と違う", repo))
		return false
	}
	slug := owner + "/" + name
	if ok, why := c.stillAuthorized(gr, rep); !ok {
		logf("をいまはマージしない（次に打ったとき gate から回し直す）: %s", why)
		return true
	}
	nowHead, checks, err := gate.ReadChecks(number, slug)
	if err != nil {
		logf("をいまはマージしない（次に打ったとき gate から回し直す）: %v", err)
		return true
	}
	if ok, why := ChecksStill(gr.Head, nowHead, checks, table.Repos[repo].GitHubChecks == "optional"); !ok {
		logf("をいまはマージしない（次に打ったとき gate から回し直す）: %s", why)
		return true
	}
	if _, err := gh("pr", "merge", number, "--repo", slug, "--"+method, "--match-head-commit", gr.Head); err != nil {
		// オーナーが先にマージしていた（gate が判定した head のまま MERGED）なら誤報の問いを出さず、オーナーがマージしたときの道（§2.7）へ渡す。
		// 同じ巡の andon issue settle が片付ける。gate の記録の merged は書かない（dispatcher がマージした印）
		out, verr := ghReread("pr", "view", number, "--repo", slug, "--json", "state,mergeCommit,headRefOid")
		if sha, ok := MergedBefore(out, gr.Head); verr == nil && ok {
			logf("の PR %s は gh pr merge の前に同じ head でマージされていた（%s）。問いを出さずオーナーのマージとして片付ける", rep.PR, short12(sha))
			return false
		}
		if RetryMerge(err.Error(), w.Retries) {
			w.Retries++
			why := "base が動いて"
			if strings.Contains(err.Error(), policyProhibits) {
				why = "必須の check がまだ揃わず"
			}
			logf("をいまはマージしない（次に打ったとき gate から回し直して打ち直す。%d/%d 回）: %s gh pr merge が通らなかった: %v", w.Retries, mergeRetries, why, err)
			return true
		}
		stop("gh pr merge に失敗: " + err.Error())
		return false
	}
	// gh pr merge は通った。読み直しの gh pr view が一時に落ちただけで問いにしない（ghReread が読み直す）。MERGED を確かめずに片付けない
	out, err := ghReread("pr", "view", number, "--repo", slug, "--json", "state,mergeCommit")
	var view struct {
		State       string `json:"state"`
		MergeCommit struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err != nil {
		stop(fmt.Sprintf("マージを打ったが PR を読み直せない（%d 回）: %v", ghRereads, err))
		return false
	}
	if json.Unmarshal([]byte(out), &view) != nil || view.State != "MERGED" || !contract.IsSHA(view.MergeCommit.Oid) {
		stop(fmt.Sprintf("マージを打ったが PR が MERGED にならない（%s）", view.State))
		return false
	}
	w.Merged = view.MergeCommit.Oid
	if err := markMerged(record); err != nil {
		logf("の gate の記録に merged を書けない: %v", err)
	}
	logf("の PR %s をマージした（%s）", rep.PR, short12(view.MergeCommit.Oid))
	if why := c.finishMerged(stderr); why != "" {
		c.askGate(res, contract.Question{
			Kind: "irreversible", Question: fmt.Sprintf("%s（%s）はマージしたが片付けに失敗した。Chief of Staff が片付けるか", c.id, short(c.issue.Str("title"))),
			Facts: []string{why, "PR " + rep.PR}, From: "gate",
			Options: []contract.Option{{Label: "Chief of Staff が理由を読んで andon issue finish する", Pick: ptr(true)}, {Label: "オーナーが片付ける"}}})
	}
	res.Notify = append(res.Notify, contract.Notice{Kind: "merged", Detail: map[string]any{"pr": rep.PR, "sha": view.MergeCommit.Oid}})
	return false
}

func (c *ctx) finishMerged(stderr io.Writer) string {
	if err := allowed("finish", c.id, c.issue.Str("status")); err != nil {
		return err.Error()
	}
	if err := c.finish(stderr); err != nil {
		return err.Error()
	}
	c.writeBack(stderr) // done になってから。書けなくても finish は戻さない（ADR-0011 決めたこと 15）
	return ""
}

func (c *ctx) askGate(res *contract.Merge, q contract.Question) {
	b, _ := json.Marshal(q)
	var out bytes.Buffer
	if err := c.ask(bytes.NewReader(b), &out, "-"); err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の問いを出せない: %v", c.id, err))
		return
	}
	m := askedRe.FindStringSubmatch(strings.TrimSpace(out.String()))
	if m == nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の問いの番号が読めない: %q", c.id, out.String()))
		return
	}
	var n int
	fmt.Sscan(m[1], &n) //nolint:errcheck // askedRe が 1 から始まる数字だけを取る
	res.Notify = append(res.Notify, contract.Notice{Kind: "asked", Detail: map[string]any{"n": n, "kind": q.Kind, "question": q.Question}})
}

// stillAuthorized はマージの直前に課題と履歴と委譲を読み直し、gate のときの認可がまだ効いているか。
// 委譲の merge は、gate と同じに課題ごとの値（名指しの grant を含む。担当が one_way を申告していれば名指しを読まない）で読む。
// gate のあとに名指しが取り消されるか期限が切れれば、値が gate のときと違って断る（ADR-0041 決めたこと 6）。
func (c *ctx) stillAuthorized(gr gateResult, rep *contract.Report) (bool, string) {
	now, err := c.l.Show(c.id)
	if err != nil {
		return false, "課題を読み直せない: " + err.Error()
	}
	rows, logErr := c.l.ReadLog()
	trusted := c.l.Trusted(rows, now, "merge") && c.l.Trusted(rows, now, "review") && c.l.Trusted(rows, now, "must_fix")
	merge, _ := gate.Delegated(delegate.Read(delegate.Workspace(), c.l.Slug, time.Now().UTC().Format("2006-01-02")), c.id, rep)
	hold, held, err := c.l.Hold(rows, logErr, now)
	if err != nil {
		hold, held = "（読めない: "+err.Error()+"）", true
	}
	return Authorized(ledger.SettingsOf(c.issue), ledger.SettingsOf(now), now.Str("status"), trusted, gr.Delegation.Merge, merge, gr.Tier, hold, held)
}

// Authorized は、gate のときの事実（before・gateMerge）とマージ直前に読み直した事実（now・status・trusted・nowMerge・止め）から、
// dispatcher の道でマージしてよいかを決める（純粋関数）。どれか1つでも変わっていれば、いまはマージしない。
// 止めている（held。理由の課題 hold）ならマージしない（ADR-0056 決めたこと 9。gate のあとに kanban set で直に止められたとき）。
func Authorized(before, now ledger.Settings, status string, trusted bool, gateMerge, nowMerge, tier, hold string, held bool) (bool, string) {
	switch {
	case held:
		return false, fmt.Sprintf("Chief of Staff が PR を止めた（理由の課題 %s）", hold)
	case status != "review":
		return false, fmt.Sprintf("課題が review でなくなった（%s）", status)
	case now != before:
		return false, "issue.yaml の予約キーが gate のあとに変わった"
	case now.Merge != "auto" || !trusted:
		return false, "merge: auto でない、または予約キーを最後に書いたのが Chief of Staff でない"
	case nowMerge != gateMerge:
		return false, fmt.Sprintf("委譲の merge が gate のあとに変わった（%s → %s）", gateMerge, nowMerge)
	case !gate.Within(tier, nowMerge):
		return false, fmt.Sprintf("delegation: merge=%s < tier %s", nowMerge, tier)
	}
	return true, ""
}

// ChecksStill は、マージの直前に読み直した PR の head と GitHub の checks で、まだマージしてよいか（純粋関数）。
// head が gate の判定した head と違う（gate のあとに push された）か、checks が通す状態でなければマージしない。
func ChecksStill(gateHead, nowHead string, checks gate.GitHubChecks, optional bool) (bool, string) {
	switch {
	case nowHead != gateHead:
		return false, fmt.Sprintf("PR の head が gate のあとに動いた（%s → %s）", short12(gateHead), short12(nowHead))
	case !checks.Passed(optional):
		return false, fmt.Sprintf("GitHub の checks が gate のあとに変わった（%s: %s）", checks.State, strings.Join(checks.Names, ", "))
	}
	return true, ""
}

// baseModified は、ほかの PR のマージと重なって base が動いたときに gh pr merge が返す文。
// head が動いたときの文（Head branch was modified. …）や衝突の文には当たらない。
const baseModified = "Base branch was modified. Review and try the merge again"

// policyProhibits は、PR の mergeStateStatus が BLOCKED のとき gh pr merge が要求を送らずに返す文。
// 必須の check（andon/gate）を書いた直後は、GitHub の側の計算が追いつかず BLOCKED のままのことがある（ADR-0056 決めたこと 12）。
const policyProhibits = "the base branch policy prohibits the merge"

const mergeRetries = 3

// RetryMerge は、gh pr merge の失敗 failure を、次に打ったとき gate から照合し直して打ち直すか（純粋関数）。
// base が動いた失敗と、必須の check で断られた失敗（policyProhibits）だけで、打ち直した回数 retries が上限に届いていないときだけ。
// 回し直しでは、印は既に緑なので gate は書き直さない（ADR-0056 決めたこと 3・12）。
func RetryMerge(failure string, retries int) bool {
	return retries < mergeRetries && (strings.Contains(failure, baseModified) || strings.Contains(failure, policyProhibits))
}

// GateFailedQuestion は、gate が判定できなかった（終了コードが決まった値でない）ときにオーナーへ出す問い。
// 印が無いと必須の check でオーナーもマージできないので、先頭（推奨）は Chief of Staff が原因を直して回し直すこと（ADR-0056 決めたこと 14）。
func GateFailedQuestion(id, title, why string) contract.Question {
	return contract.Question{
		Kind: "irreversible", Question: fmt.Sprintf("%s（%s）の gate が判定できなかった。どう進めるか", id, short(title)),
		Facts: []string{why}, From: "gate",
		Options: []contract.Option{{Label: "Chief of Staff が原因を直してから回し直す", Pick: ptr(true)}, {Label: "オーナーが PR を読んでマージを決める"}}}
}

// MergedBefore は、gh pr merge が失敗したあとに読み直した PR（gh pr view --json state,mergeCommit,headRefOid の返事）が、
// gate の判定した head のまま、もうマージされていたか（純粋関数。dispatcher.md §7.2 #5）。そうならマージの commit を返す。
// MERGED でない・head が違う・返事が読めないときは false（今までどおり問いにする）。
func MergedBefore(view, gateHead string) (string, bool) {
	var v struct {
		State       string `json:"state"`
		HeadRefOid  string `json:"headRefOid"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if json.Unmarshal([]byte(view), &v) != nil || v.State != "MERGED" || !contract.IsSHA(gateHead) || v.HeadRefOid != gateHead ||
		v.MergeCommit == nil || !contract.IsSHA(v.MergeCommit.Oid) {
		return "", false
	}
	return v.MergeCommit.Oid, true
}

// markMerged は gate の記録の merged を true にする（dispatcher の道でマージした印。settle が読む。contract.md §7）。
func markMerged(record string) error {
	b, err := os.ReadFile(record)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	m["merged"] = true
	return contract.Write(record, contract.SchemaGate, m)
}

// ghRereads は、マージの前後に PR を読み直す gh pr view を、一時の失敗（502 など）で打ち直す回数の上限（初めの1回を含む）。
// 間は ghRereadWait。読めなければ最後の失敗を返し、呼び手は今までどおり問いにする（dispatcher.md §7.2 #5）。
const (
	ghRereads    = 3
	ghRereadWait = 2 * time.Second
)

// ghReread は gh を打ち、失敗したら ghRereadWait おいて ghRereads 回まで打ち直す（読むだけの gh pr view に使う）。
func ghReread(args ...string) (string, error) {
	out, err := gh(args...)
	for i := 1; i < ghRereads && err != nil; i++ {
		time.Sleep(ghRereadWait)
		out, err = gh(args...)
	}
	return out, err
}

func gh(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return string(out), fmt.Errorf("gh %s: %v %s", args[0]+" "+args[1], err, firstLine(errb.String()))
	}
	return string(out), nil
}

func pkgHome() string {
	if h := os.Getenv("ANDON_HOME"); h != "" {
		return h
	}
	return "/opt/andon"
}

func ptr[T any](v T) *T { return &v }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func short12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}
