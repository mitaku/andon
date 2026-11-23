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

// forgetOthers は keep のどれにもいない課題の記録を捨てる（片付いた・doing に戻った・やめた）。打たれた課題の分は呼び手が決める。
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

// mergeOne は1つの課題を見て res を埋め、残す記録を返す（nil なら記録から外す）。
func (c *ctx) mergeOne(w contract.Merging, fresh bool, res *contract.Merge, stderr io.Writer, now time.Time) *contract.Merging {
	if c.issue.Str("status") != "review" {
		return nil
	}
	rows := c.l.Log()
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
	if w.Row != row {
		w = contract.Merging{Row: row}
		// 記録が無い（初めて打つ・消えた）ときは、この行のあとに書かれた今の head の gate の記録を引き継ぐ（同じ行に2度目の問いや通知を出さない）
		if fresh {
			if g := c.gatedSince(rep.Commit, rows[row-1].At); g != nil {
				w.Sha, w.Decision = g.Head, g.Decision
				return &w
			}
		}
	}
	if w.Decision != "" {
		if w.Merged == "" && contract.IsSHA(w.Sha) {
			// gate が決めたときに閉じられなかった前の head の問いを閉じ直す（閉じる問いが無ければ一覧を読むだけ）
			c.closeStaleGateQuestions(w.Sha, res)
		}
		if !c.headMoved(&w, rep, res, now) {
			return &w
		}
		// approve か needs_human を決めたあとに PR の head が動いた: 決めたことを捨て、今の head で gate から回し直す
		w = contract.Merging{Row: row}
	}
	if w.Wait != "" && c.stillWaiting(rows, rep.PR, w.Wait, now) {
		// wait を返した head の checks がまだ揃っていない: gate を回さない（GraphQL の checks だけを読み、REST を使わない）
		res.State = "wait"
		res.Log = append(res.Log, fmt.Sprintf("%s は GitHub の checks を待つ（gate は回さない）: head %s の checks がまだ揃っていない", c.id, short12(w.Wait)))
		return &w
	}
	w.Wait = ""
	if w.Retries > 0 && c.mergedWhileRetrying(&w, rep, res, now) {
		return &w
	}
	var out, errb bytes.Buffer
	code := gate.Main([]string{c.id}, nil, &out, &errb)
	if code == 3 && strings.Contains(strings.ToLower(errb.String()), "rate limit") {
		// gh の上限で落ちた: 問いにもせず、決めたことも書かない（いつ回し直すかは dispatcher が kanban ごとに決める）
		res.State = "rate-limited"
		return &w
	}
	var gr gateResult
	if (code != 0 && code != 1 && code != 4 && code != 5) || json.Unmarshal(out.Bytes(), &gr) != nil {
		w.Decision, res.State = "needs_human", "needs_human"
		w.Approved, w.Retries = "", 0 // 問いで決まった。approve のまま止まったとは届けない
		why := fmt.Sprintf("andon gate %s: 終了コード %d %s", c.id, code, firstLine(errb.String()))
		res.Log = append(res.Log, fmt.Sprintf("%s の gate が判定できない: %s", c.id, why))
		c.askGate(res, GateFailedQuestion(c.id, c.issue.Str("title"), why))
		return &w
	}
	if gr.Decision == "wait" {
		// GitHub の checks が揃っていない: 決めたことは書かず、次に打ったとき gate から回し直す（通知もしない）
		res.State = "wait"
		res.Log = append(res.Log, fmt.Sprintf("%s の gate は GitHub の checks を待つ: %s", c.id, strings.Join(gr.Reasons, "／")))
		if contract.IsSHA(gr.Head) {
			w.Wait = gr.Head
		}
		return &w
	}
	record := ""
	if contract.IsSHA(gr.Head) {
		// gate が今の head で決めた: 前の head について出した gate の問いは、この判定で決め直したので閉じる
		// （担当が doing に戻って rebase し、ready が数えずに通した問い。ready の staleGateQuestions）
		c.closeStaleGateQuestions(gr.Head, res)
		record = filepath.Join(c.l.Library(c.id), "gate", gr.Head+".json")
		res.Auto = gr.Decision == "approve" && c.autoMerge(rows, gr)
		res.Notify = append(res.Notify, contract.Notice{Kind: "gate",
			Detail: map[string]any{"decision": gr.Decision, "reasons": nonNil(gr.Reasons), "record": record}})
	}
	switch gr.Decision {
	case "approve":
		if res.Auto && w.Approved == "" { // 回し直した gate の approve では測り直さない
			w.Approved = now.UTC().Format("2006-01-02T15:04:05Z")
		}
		if c.approve(rep, gr, record, &w, res, stderr) {
			w.Sha, res.State = gr.Head, "regate" // 決めたことは書かない: 次に打ったとき gate から回し直す
			return &w
		}
	case "fix":
		c.sendFix(gr, record, res)
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
	w.Polled = now.UTC().Format("2006-01-02T15:04:05Z") // gate が今 head を読んだ。次に聞くのは1分たってから
	res.State = gr.Decision
	if w.Merged != "" {
		res.State = "merged"
	}
	return &w
}

// mergedWhileRetrying は、base が動いたマージの打ち直しを待つ間に、オーナーが gate の判定した head のままマージしたか。
// していれば gate を回さず（回すと「PR が open でない」の fix が担当へ飛ぶ）、gh pr merge の前にマージされていたとき（MergedBefore）と同じに
// approve のまま決めて settle に渡す。読めなければ false（gate が読み直す）。
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
// approve か needs_human で、dispatcher がマージしておらず、判定した head が sha のとき。
func RegateWatched(w contract.Merging) bool {
	return (w.Decision == "approve" || w.Decision == "needs_human") && w.Merged == "" && contract.IsSHA(w.Sha)
}

// HeadMoved は、聞いた PR の state と head が、gate の判定した head から動いた（まだ open の PR に push された）ことを示すか（純粋関数）。
// MERGED・CLOSED は見ない（オーナーのマージと閉じた PR は settle の道）。
func HeadMoved(w contract.Merging, state, head string) bool {
	return RegateWatched(w) && state == "OPEN" && contract.IsSHA(head) && head != w.Sha
}

// closeGateQuestions は、この課題のまだ受け取られていない gate の問い（from: gate）を、open なら why と答えて受け取る
// （前の head についての問いは、回し直した gate が今の head で決め直す）。gate の外の問いには触らない。閉じられなければログに1行。
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

// closeStaleGateQuestions は、この課題のまだ受け取られていない gate の問いのうち、head と違う head について出た問い
// （どの head の問いかは事実の gate の記録の行から読む。questionHead）を、open なら理由を答えて受け取る。答えには前と今の head を書き、
// 判断の記録を残す。どの head の問いか読めない問い・gate の外の問いには触らない。閉じられなければログに1行
// （gate が決めたあとも、merge は打たれるたびに決めた head でこれを呼び、閉じ直す）。
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

// prOpenHead は PR の state（OPEN・MERGED・CLOSED）と head（gh pr view --json state,headRefOid。1回）。
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

// gatedSince は head（report.json の commit ＝ PR の head）の gate の記録 library/gate/<head>.json が since 以後に書かれていれば返す。
// 見るのは今の head だけ（前の回の記録は同じ秒に書かれていても拾わない）。無ければ nil。
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

// gateResult は andon gate の出力のうち merge が読むもの（記録 contract.md §7。信じられる PR が無いときは decision・reasons だけ）。
type gateResult struct {
	Decision   string   `json:"decision"`
	Reasons    []string `json:"reasons"`
	Head       string   `json:"head"`
	Tier       string   `json:"tier"`
	PR         string   `json:"pr"`
	Delegation struct {
		Merge string `json:"merge"`
		OK    bool   `json:"ok"`
	} `json:"delegation"`
}

// autoMerge は gate の approve のあとマージに進むか（merge: auto を Chief of Staff が書いた・委譲の範囲内）。
func (c *ctx) autoMerge(rows []ledger.Row, gr gateResult) bool {
	return ledger.SettingsOf(c.issue).Merge == "auto" && c.l.Trusted(rows, c.issue, "merge") && gr.Delegation.OK
}

// stillWaiting は、PR の head が wait を返したときの head のままで、gate を回しても wait になる checks のままか（GraphQL を1回）。
// checks が走っている（pending）か、1つも無く（none）review に入ってから github_checks_wait_minutes を過ぎていない。
// 読めない・過ぎた・揃ったなら false（gate を回す。期限を過ぎた none は gate が needs_human を返す）。
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

// sendFix は理由を担当へ返す（担当はオーナーが打つ端末ではないので、離席の守りは要らない）。
// 返せなければ dispatcher が Chief of Staff に届けるよう、通知 `gate` の detail に印を書く: 担当がいなければ no_agent、
// 送れなければ（担当の端末が無い）unsent（担当の address）。gate が無ければ（PR の head が読めない）通知 report。
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
	msg += "kanban start で doing に戻して直し、push して andon issue review-request（レビューが要るとき）か andon issue ready からやり直す。"
	// 先頭に送り手の印を置く（端末の外から送るので hc の包みが付かない。spawn.Mark）
	if out, err := exec.Command("hc", "send", agent, spawn.Mark(spawn.FromGate, msg)).CombinedOutput(); err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の担当 %s に fix を送れない（Chief of Staff に届ける）: %v %s", c.id, agent, err, firstLine(string(out))))
		// 担当の端末が無い（セルの起き直しなど）: 通知 `gate` に送れなかった担当を書き、dispatcher が Chief of Staff に届ける
		c.markGate(res, "unsent", agent)
	}
}

// markGate は、この回の通知 `gate` の detail に key を書く（dispatcher が Chief of Staff に届ける印）。通知 `gate` が無ければ通知 report を積む。
func (c *ctx) markGate(res *contract.Merge, key string, v any) {
	for i := len(res.Notify) - 1; i >= 0; i-- {
		if res.Notify[i].Kind == "gate" {
			res.Notify[i].Detail[key] = v
			return
		}
	}
	res.Notify = append(res.Notify, contract.Notice{Kind: "report", Detail: map[string]any{"report": filepath.Join(c.l.Dir, "issues", c.id, "report.md")}})
}

// approve は merge: auto ＋ 委譲の範囲内ならマージして片付ける。それ以外は通知 `gate` のまま止まる（オーナーがマージする）。
// マージの直前に、課題の Settings・履歴・委譲を読み直し、gate のときと変わっていればマージせず regate を返す
// （呼び手は決めたことを書かず、次に打ったとき gate から回し直す）。gh pr merge が base の動きで通らなかったとき（RetryMerge）も
// regate を返す: その場では打ち直さず、次に打ったとき gate と直前の読み直しを全部通してから打ち直す。
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
	// マージするのは gate が判定した PR と head だけ（判定のあとに report.json が書き換わっていたら打たない）
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
	// PR の head と GitHub の checks も読み直す（gate のあとに push された・checks が変わったら打たない）
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
		out, verr := gh("pr", "view", number, "--repo", slug, "--json", "state,mergeCommit,headRefOid")
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
	out, err := gh("pr", "view", number, "--repo", slug, "--json", "state,mergeCommit")
	var view struct {
		State       string `json:"state"`
		MergeCommit struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err != nil || json.Unmarshal([]byte(out), &view) != nil || view.State != "MERGED" || !contract.IsSHA(view.MergeCommit.Oid) {
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

// finishMerged は finish を同じプロセスで打つ（lock を持ったまま。別プロセスにすると同じ lock を待って止まる）。失敗したら理由を返す。
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

// askGate は問いを kanban に出し（andon issue ask と同じ道）、通知 asked を res に足す。出せなければログに1行。
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
	rows := c.l.Log()
	trusted := c.l.Trusted(rows, now, "merge") && c.l.Trusted(rows, now, "review") && c.l.Trusted(rows, now, "must_fix")
	merge, _ := gate.Delegated(delegate.Read(delegate.Workspace(), c.l.Slug, time.Now().UTC().Format("2006-01-02")), c.id, rep)
	return Authorized(ledger.SettingsOf(c.issue), ledger.SettingsOf(now), now.Str("status"), trusted, gr.Delegation.Merge, merge, gr.Tier)
}

// Authorized は、gate のときの事実（before・gateMerge）とマージ直前に読み直した事実（now・status・trusted・nowMerge）から、
// dispatcher の道でマージしてよいかを決める（純粋関数）。どれか1つでも変わっていれば、いまはマージしない。
func Authorized(before, now ledger.Settings, status string, trusted bool, gateMerge, nowMerge, tier string) (bool, string) {
	switch {
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

// mergeRetries は base の動きで通らなかったマージを打ち直す上限（仮の値）。間は dispatcher の見回り（review の課題があれば
// 5秒ごと）と gate の所要で、数秒の重なりをやり過ごす。
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

// gh は gh を打ち、stdout を返す（失敗は stderr の1行目つき）。
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

// pkgHome はパッケージの置き場（ANDON_HOME。無ければ /opt/andon）。
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

// short は題を問いの1行に入れる形に切る（改行を除き40字まで）。
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}
