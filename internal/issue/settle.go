package issue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"andon/internal/contract"
	"andon/internal/dispatcher"
	"andon/internal/gate"
	"andon/internal/ledger"
)

// prPoll は同じ PR を GitHub に聞く間隔（dispatcher は待ちがあると5秒ごとに打つので、gh の上限を食わない。dispatcher.md §2.7）。
const prPoll = time.Minute

// settleOn は settle が PR を見る課題の status（doing のうちにオーナーがマージした PR も片付ける。dispatcher.md §2.7）。
var settleOn = []string{"review", "doing"}

// settlePath は settle の記録（~/.local/state/andon/dispatcher/<slug>.settle.json。書き手は andon issue settle だけ。ADR-0017 決めたこと 5）。
func settlePath(slug string) string {
	return filepath.Join(filepath.Dir(dispatcher.StatePath(slug)), slug+".settle.json")
}

// settle は andon issue settle: review か doing の課題の PR をオーナーがマージしたかを見て、マージしていたら dispatcher がマージしたときと
// 同じ道（gate の問いを閉じて finish）で片付ける。マージはしない。積む通知は stdout の JSON で返す（積むのは dispatcher）。
// 2回打っても同じ結果になるよう、聞いた時刻・MERGED／CLOSED・知らせた理由を自分の記録に持つ（dispatcher.md §2.7）。
//
//	MERGED（gate の記録の merged が true）   → dispatcher がマージした。何もしない（approve の道が片付ける）
//	MERGED                                   → gate の問い（from: gate）を「オーナーがマージした」と閉じて finish、通知 merged（by: person）
//	MERGED で gate の外の問いが開いている     → 片付けずに通知 merged（held）。同じ理由では二度知らせない
//	CLOSED                                   → 片付けない。通知 closed を閉じるたびに1回。開き直されうるので聞き続ける
//
// doing の課題（担当が働いているうちにオーナーがマージした）も同じ道で片付ける。受け取られていない問い・commit していない変更が
// あれば片付けずに held で知らせる（finish の拒みを越えない）。
func (c *ctx) settle(stdout, stderr io.Writer, now time.Time) error {
	path := settlePath(c.l.Slug)
	rec := map[string]contract.Settled{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &rec); err != nil {
			return fail("settle の記録が読めない（%s）: %v", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := forgetOthers(c, rec, settleOn...); err != nil {
		return err
	}
	res := contract.Settle{Issue: c.id, State: "skip", Notify: []contract.Notice{}}
	w, err := c.settleOne(rec[c.id], &res, stderr, now)
	if w == nil {
		delete(rec, c.id)
	} else {
		rec[c.id] = *w
	}
	if err2 := os.MkdirAll(filepath.Dir(path), 0o755); err2 != nil {
		return err2
	}
	if err2 := contract.WriteState(path, rec); err2 != nil {
		return fail("settle の記録を書けない: %v", err2)
	}
	if err != nil {
		return err
	}
	_, err = stdout.Write(contract.Marshal(res))
	return err
}

// settleOne は1つの課題を見て res を埋め、残す記録を返す（nil なら記録から外す）。
func (c *ctx) settleOne(w contract.Settled, res *contract.Settle, stderr io.Writer, now time.Time) (*contract.Settled, error) {
	if !slices.Contains(settleOn, c.issue.Str("status")) {
		return nil, nil
	}
	rep, err := c.previous()
	if err != nil || rep == nil || rep.PR == "" {
		return nil, err
	}
	// 見るのは issue.yaml の repo の PR だけ（ほかの repo の PR がマージされても、この課題は片付かない）
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok || "github.com/"+owner+"/"+name != c.issue.Str("repo") {
		return nil, nil
	}
	if w.PR != rep.PR || w.Commit != rep.Commit {
		w = contract.Settled{PR: rep.PR, Commit: rep.Commit}
	}
	// Chief Engineer の課題は、doing の間（オーナーと話している途中）は片付けず、閉じたことも知らせない。PR の印だけを揃え、
	// マージされていれば記録して、review に入ったときに片付ける（dispatcher.md §2.7・ADR-0056 決めたこと 6）
	engineerDoing := c.issue.Str("profile") == "chief-engineer" && c.issue.Str("status") == "doing"
	res.State = "watching"
	// CLOSED は開き直されうるので、MERGED になるまで OPEN と同じ間隔で聞き直す（知らせるのは閉じるたびに1回）
	if w.State != "MERGED" {
		if w.State == "CLOSED" {
			res.State = "closed"
		}
		if at, err := time.Parse("2006-01-02T15:04:05Z", w.At); err == nil && now.Sub(at) < prPoll {
			return &w, nil
		}
		w.At = now.UTC().Format("2006-01-02T15:04:05Z")
		state, head, sha, mark, err := prState(number, owner+"/"+name)
		if err != nil {
			return &w, fail("%s の PR の状態が読めない（次に打ったとき聞き直す）: %v", c.id, err)
		}
		switch state {
		case "MERGED":
			w.State, w.Head, w.Sha = state, head, sha
		case "CLOSED":
			if engineerDoing {
				break
			}
			if w.State != "CLOSED" {
				res.Notify = append(res.Notify, contract.Notice{Kind: "closed", Detail: map[string]any{"pr": rep.PR}})
			}
			w.State = state
			res.State = "closed"
		case "OPEN":
			w.State = "" // 開き直された。また閉じたら知らせる
			res.State = "watching"
			c.alignMark(owner+"/"+name, head, mark, res)
		}
	}
	if w.State != "MERGED" || engineerDoing {
		return &w, nil
	}
	// dispatcher が自分でマージした PR は gate の記録の merged が true（dispatcher の控えは読まない。ADR-0017 決めたこと 5 の (d)）
	if mergedByGate(filepath.Join(c.l.Library(c.id), "gate", w.Head+".json")) {
		res.State = "auto"
		return &w, nil
	}
	if why := c.closeAndFinish(w.PR, stderr); why != "" {
		res.State = "held"
		if why != readOld(w.Held) {
			res.Notify = append(res.Notify, contract.Notice{Kind: "merged", Detail: map[string]any{"pr": w.PR, "sha": w.Sha, "by": "person", "held": why}})
			w.Held = why
		}
		return &w, nil
	}
	res.State = "finished"
	res.Notify = append(res.Notify, contract.Notice{Kind: "merged", Detail: map[string]any{"pr": w.PR, "sha": w.Sha, "by": "person"}})
	return nil, nil
}

// oldWords は、2026-11-20 までの文が使っていた語と、それに替えた語（ADR-0040 決めたこと 2〜4・7）。
// 記録に残した前の理由の文は古い語のことがあるので、読み替えてから今の理由と比べる。
var oldWords = [][2]string{{"台帳", "kanban"}, {"ゲート", "gate"}, {"版", "バージョン"}}

// keptBan は、読み替えない「版」の前の語（製品や文書の種類を言う形。ADR-0040 決めたこと 4）。
var keptBan = []string{"最新", "初", "英語", "更新", "出"}

// readOld は文 s の古い語を新しい語に読み替える。英字の語は、前後が空白・句読点・括弧・文の端でなければ半角の空白で区切る
// （「ゲートの外の問い」→「gate の外の問い」）。読み替えないと、片付けを待っている課題ごとに、上げた直後に同じ理由をもう一度知らせる。
func readOld(s string) string {
	for _, w := range oldWords {
		parts := strings.Split(s, w[0])
		if len(parts) == 1 {
			continue
		}
		var b strings.Builder
		b.WriteString(parts[0])
		for _, p := range parts[1:] {
			before := b.String()
			r, _ := utf8.DecodeLastRuneInString(before)
			switch {
			case w[1] == "バージョン" && (keepsBan(before) || r < utf8.RuneSelf && r != ' ' && before != ""):
				b.WriteString(w[0]) // 最新版・初版・Go 版など。読み替えない
			case w[1][0] < utf8.RuneSelf:
				if before != "" && !separates(r) {
					b.WriteByte(' ')
				}
				b.WriteString(w[1])
				if n, _ := utf8.DecodeRuneInString(p); p != "" && !separates(n) {
					b.WriteByte(' ')
				}
			default:
				b.WriteString(w[1])
			}
			b.WriteString(p)
		}
		s = b.String()
	}
	return s
}

// keepsBan は、s の終わりが読み替えない「版」の前の語か（数字・英字のあとは readOld が見る）。
func keepsBan(s string) bool {
	return slices.ContainsFunc(keptBan, func(k string) bool { return strings.HasSuffix(s, k) })
}

// separates は、英字の語の隣に空白を置かなくてよい字か（空白・句読点・括弧）。
func separates(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) || strings.ContainsRune("（）「」『』〈〉【】、。・：", r)
}

// closeAndFinish は gate の問いを「オーナーがマージした」と閉じて finish する。片付けなかったら理由を返す。
func (c *ctx) closeAndFinish(pr string, stderr io.Writer) string {
	gateQs, others, err := c.openQuestions()
	if err != nil {
		return err.Error()
	}
	if len(others) > 0 {
		return "gate の外の問いが開いている: " + strings.Join(others, "・")
	}
	for _, q := range gateQs {
		name := fmt.Sprintf("%s#%d", c.id, q.N)
		if err := c.closeQuestion(name, q.Status, "オーナーがマージした（"+pr+"）"); err != nil {
			return "gate の問い " + name + " を閉じられない: " + err.Error()
		}
	}
	if err := allowed("finish", c.id, c.issue.Str("status")); err != nil {
		return err.Error()
	}
	if err := c.finish(stderr); err != nil {
		return err.Error()
	}
	c.writeBack(stderr) // done になってから。書けなくても finish は戻さない（ADR-0011 決めたこと 15）
	return ""
}

// question は kanban questions --json の1件（open と answered。received は出ない）。
type question struct {
	ID     string `json:"id"`
	N      int    `json:"n"`
	Status string `json:"status"`
}

// openQuestions はこの課題のまだ受け取られていない問いを、gate が出したもの（library/questions/<n>.json の from: gate）と
// それ以外（名前 <id>#<n>）に分ける。問いの JSON が無い・読めないものは gate の外に数える（閉じない）。
func (c *ctx) openQuestions() ([]question, []string, error) {
	out, err := c.l.Run("", "questions", "--json")
	if err != nil {
		return nil, nil, err
	}
	var all []question
	if out != "" {
		if err := json.Unmarshal([]byte(out), &all); err != nil {
			return nil, nil, fmt.Errorf("問いの一覧が読めない")
		}
	}
	var gate []question
	var others []string
	for _, q := range all {
		if ledger.CanonID(q.ID) != c.id || q.Status == "received" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "questions", strconv.Itoa(q.N)+".json"))
		var v struct {
			From string `json:"from"`
		}
		if err == nil && json.Unmarshal(b, &v) == nil && v.From == "gate" {
			gate = append(gate, q)
		} else {
			others = append(others, fmt.Sprintf("%s#%d", c.id, q.N))
		}
	}
	sort.Strings(others)
	return gate, others, nil
}

// mergedByGate は gate の記録の merged が true か（dispatcher がマージした印。contract.md §7）。
func mergedByGate(record string) bool {
	b, err := os.ReadFile(record)
	var v struct {
		Merged bool `json:"merged"`
	}
	return err == nil && json.Unmarshal(b, &v) == nil && v.Merged
}

// alignMark は、開いている PR の head の印（commit status andon/gate）を、課題の今の状態に揃える（ADR-0056 決めたこと 5）。
// settle が PR を聞く 1 分に 1 回の読みに乗せ、GitHub に聞き足さない。書くのは揃っていないときだけ:
// review の課題は、その head を gate が判定した結果（gate の記録 library/gate/<head>.json）の印。判定の無い head（gate の前・wait・
// gate のあとに動いた head）には書かない。前のバージョンが書いた、オーナー待ちの赤い印もここで緑に直る。
// doing に戻った課題は、緑が残っていれば赤（作った側が直しています）。Chief Engineer の文書の PR は、review なら緑（gate を通らない。決めたこと 6）。
// 止めている課題（予約キー hold。決めたこと 7・10）は、渡している（緑が残っている・判定が通したかオーナー待ち）なら黄。
// kanban set で hold を直に書いたときと、andon issue hold が黄を書けなかったときは、ここで黄になる。
// gh pr view の statusCheckRollup は説明文を出さないので、今の印とは状態だけで比べる。書けなければ記録に残して次の読みで直す。
// settle は課題の lock の中で走るので、dispatcher の巡が回す同じ課題の gate（andon issue merge の中）とは重ならない。
// 手で打つ andon gate は gate の lock だけを取るので重なりうる（印は次に聞いたとき揃え直す。リンクは付かない）。
func (c *ctx) alignMark(slug, head string, have gate.Mark, res *contract.Settle) {
	if !contract.IsSHA(head) {
		return
	}
	f := c.markFacts(head, have)
	reason, held, err := c.l.Hold(c.l.Log(), c.issue)
	// 止めを読めなければ止めているものとして揃える（緑を残さない側。ADR-0056 決めたこと 9）
	f.Held = held || err != nil
	m, ok := gate.WantMark(f)
	if !ok {
		return
	}
	if err := postMark(slug, head, m); err != nil {
		res.Log = append(res.Log, fmt.Sprintf("%s の PR の印を揃えられない（次に聞いたとき揃え直す）: %v", c.id, err))
		return
	}
	why := ""
	if f.Held {
		why = "。止めている（理由の課題 " + orNone(reason) + "）"
	}
	res.Log = append(res.Log, fmt.Sprintf("%s の PR の印を %s に揃えた（%s。前は %s%s）", c.id, m.State, m.Description, orNone(have.State), why))
}

// markFacts は、PR の今の head と今ある印 have から、印を決める事実を集める（止めているかは呼び手が足す）。
// review の課題は、その head を今の回の gate が判定した結果（gate の記録 library/gate/<head>.json）を持つ。
func (c *ctx) markFacts(head string, have gate.Mark) gate.MarkFacts {
	f := gate.MarkFacts{Status: c.issue.Str("status"), Engineer: c.issue.Str("profile") == "chief-engineer", Have: have, HaveDescUnknown: true}
	if b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "gate", head+".json")); err == nil {
		var g gate.Record
		if json.Unmarshal(b, &g) == nil && g.Head == head && c.decidedThisRound(head) {
			f.Decision, f.Reasons = g.Decision, g.Reasons
		}
	}
	return f
}

// decidedThisRound は、課題が今 review に入っている行（log.jsonl で最後に review に入った行）について、dispatcher の
// merge の記録がこの head の gate の判定を持つか。同じ head のまま doing から review に入り直し、今の回の gate がまだ
// 決めていない（wait・落ちた）とき、前の回の判定から印を書き戻さないため（ADR-0056 決めたこと 3 の「判定の前」）。
// 時刻で比べないのは、gate が review に入ったのと同じ秒に判定を書くことがあり、秒までの時刻では前の回と見分けられないから。
// merge の記録が無い・読めない・行が違う・判定が空（決めたあとに head が動いて捨てた）なら持たない（印を書かない）。
func (c *ctx) decidedThisRound(head string) bool {
	row := 0
	for i, r := range c.l.Log() {
		if r.ID == c.id && r.Change["status"] == "review" {
			row = i + 1
		}
	}
	b, err := os.ReadFile(dispatcher.MergePath(c.l.Slug))
	rec := map[string]contract.Merging{}
	if row == 0 || err != nil || json.Unmarshal(b, &rec) != nil {
		return false
	}
	w, ok := rec[c.id]
	return ok && w.Row == row && w.Decision != "" && w.Sha == head
}

func orNone(s string) string {
	if s == "" {
		return "無し"
	}
	return s
}

// prState は PR の state（OPEN・MERGED・CLOSED）と、MERGED なら head とマージの commit、OPEN なら head と gate 自身の印（gh pr view。1回）。
func prState(number, slug string) (state, head, sha string, mark gate.Mark, err error) {
	cmd := exec.Command("gh", "pr", "view", number, "--repo", slug, "--json", "state,mergeCommit,headRefOid,statusCheckRollup")
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", gate.Mark{}, fmt.Errorf("gh pr view: %v %s", err, firstLine(errb.String()))
	}
	var view struct {
		State       string `json:"state"`
		HeadRefOid  string `json:"headRefOid"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
		Rollup []gate.CheckItem `json:"statusCheckRollup"`
	}
	if json.Unmarshal(out, &view) != nil {
		return "", "", "", gate.Mark{}, fmt.Errorf("gh pr view の返事が読めない")
	}
	switch view.State {
	case "MERGED":
		if view.MergeCommit == nil || !contract.IsSHA(view.MergeCommit.Oid) {
			return "", "", "", gate.Mark{}, fmt.Errorf("MERGED だがマージの commit が読めない")
		}
		return view.State, view.HeadRefOid, view.MergeCommit.Oid, gate.Mark{}, nil
	case "OPEN":
		return view.State, view.HeadRefOid, "", gate.GateMark(view.Rollup), nil
	case "CLOSED":
		return view.State, "", "", gate.Mark{}, nil
	}
	return "", "", "", gate.Mark{}, fmt.Errorf("PR の state が知らない値: %q", view.State)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
