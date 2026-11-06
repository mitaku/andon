package courier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// 通知の元（docs/courier.md §2.2・§2.3）: log.jsonl の続き・ready の差・doing の担当の端末の状態を拾って控えに積む。
// 初めて見る元（控えに位置や前回の ready が無い）は、今を控えるだけで積まない。

const (
	idleAfter    = 15 * time.Minute // idle／done のまま、これ以上で stalled
	workingAfter = 20 * time.Minute // working のまま、これ以上で stalled
)

// retryAfter は提供元のエラーで止まった担当に続きを打つまでの間（回数＝個数。courier.md §2.3）。
var retryAfter = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// RetryText は提供元のエラーで止まった担当に打つ文。
const RetryText = "提供元のエラーで止まった。続けて"

var (
	authErrRe     = regexp.MustCompile(`(?i)unauthori[sz]ed|\b401\b|\b403\b|invalid api key|/login|authenticat`)
	providerErrRe = regexp.MustCompile(`(?i)overloaded|rate.?limit|too many requests|\b429\b|API Error: 5[0-9]{2}|\b(500|502|503|504|529)\b|internal server error|service unavailable|bad gateway|connection (error|reset|refused|closed)|ECONNRESET|stream disconnected|request timed out|high demand|混雑`)
	agentRe       = regexp.MustCompile(`^(?:[A-Za-z0-9][A-Za-z0-9_.-]{0,62}/)?[A-Za-z0-9_-]{1,40}$`) // <address> か <machine>/<address>（contract.md §8）
)

// ProviderError は担当の画面が提供元の混雑・rate limit・接続・5xx のエラーか。認証の失敗は含めない。
func ProviderError(screen string) bool {
	return !authErrRe.MatchString(screen) && providerErrRe.MatchString(screen)
}

// collect は通知の元を拾う（pass が控えの lock を持って呼ぶ）。
func (p *project) collect(coordinator string) {
	p.readReady()
	p.readLog(coordinator)
	p.watch(time.Now())
}

// ---------------------------------------------------------------- log.jsonl の続き

// readLog は log.jsonl を前回の続き（バイト位置）から、最後の改行まで読む。Chief of Staff 自身の行は知らせない。
func (p *project) readLog(coordinator string) {
	path := filepath.Join(p.l.Dir, "log.jsonl")
	f, err := os.Open(path)
	var size int64
	if err == nil {
		defer f.Close()
		if fi, err := f.Stat(); err == nil {
			size = fi.Size()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		p.logf("log.jsonl が読めない: %v", err)
		return
	}
	head := logHead(f)
	if p.st.LogPos == nil { // 初めて: 位置を控えるだけ
		p.st.LogPos, p.st.LogHead = &size, head
		return
	}
	pos := *p.st.LogPos
	switch {
	case size < pos:
		p.logf("log.jsonl が短くなった。新しいファイルとして先頭から読む")
		pos, p.st.LogHold = 0, nil
	case pos > 0 && p.st.LogHead != "" && head != p.st.LogHead:
		// 台帳の log は追記だけ。先頭の行が替わったのは手で書き直したときなので、新しいファイルとして読む
		p.logf("log.jsonl の先頭の行が替わった。新しいファイルとして先頭から読む")
		pos, p.st.LogHold = 0, nil
	}
	p.st.LogHead = head
	if size == pos || f == nil {
		p.st.LogPos = &pos
		return
	}
	buf := make([]byte, size-pos)
	if _, err := f.ReadAt(buf, pos); err != nil && !errors.Is(err, io.EOF) {
		p.logf("log.jsonl が読めない: %v", err)
		return
	}
	if i := bytes.LastIndexByte(buf, '\n'); i >= 0 {
		buf = buf[:i+1]
	} else {
		buf = nil // 書きかけの行だけ
	}
	by := "terminal:" + coordinator
	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		line := buf[:i]
		if !p.logRow(line, by, pos) {
			break // 問いの JSON をまだ待つ: この行から次の見回りで読み直す
		}
		pos += int64(i + 1)
		buf = buf[i+1:]
	}
	p.st.LogPos = &pos
}

// logHead は log.jsonl の最初の行（改行まで）の sha256。まだ1行も無ければ空。
func logHead(f *os.File) string {
	if f == nil {
		return ""
	}
	buf := make([]byte, 64<<10)
	n, _ := f.ReadAt(buf, 0)
	i := bytes.IndexByte(buf[:n], '\n')
	if i < 0 {
		return ""
	}
	sum := sha256.Sum256(buf[:i])
	return hex.EncodeToString(sum[:])
}

// logRow は log.jsonl の1行（pos はその行の頭）から通知を積む。false はこの行で止まって次の見回りで読み直すとき。
func (p *project) logRow(line []byte, by string, pos int64) bool {
	var r ledger.Row
	if json.Unmarshal(line, &r) != nil || r.Change == nil || r.By == by || !idRe.MatchString(r.ID) {
		return true
	}
	c, id := r.Change, r.ID
	switch {
	case c["report"] == "write" || c["report"] == "append":
		p.notify(id, "report", map[string]any{"report": p.reportPath(id)})
	case c["question"] != nil && c["status"] == "open":
		return p.asked(id, c["question"], pos)
	case c["status"] == "review":
		// detail の pr は report.json から。PR の無い報告（レビュアー・repo の無いタスク）は report として知らせる
		rep, err := contract.ReadReport(filepath.Join(p.l.Library(id), "report.json"))
		if err == nil && rep.PR != "" {
			p.notify(id, "review", map[string]any{"report": p.reportPath(id), "pr": rep.PR})
		} else {
			p.notify(id, "report", map[string]any{"report": p.reportPath(id)})
		}
	}
	return true
}

// asked は問いの行を通知 asked にする（中身は library/questions/<n>.json）。andon task ask は台帳に書いてから JSON を置くので、
// JSON がまだ無ければ1回だけ次の見回りまで待つ。それでも無い・読めなければ飛ばしてログに残す。
func (p *project) asked(id string, v any, pos int64) bool {
	f, ok := v.(float64)
	n := int(f)
	if !ok || f != float64(n) || n < 1 {
		return true
	}
	b, err := os.ReadFile(filepath.Join(p.l.Library(id), "questions", strconv.Itoa(n)+".json"))
	var q *contract.Question
	if err == nil {
		q, err = contract.ParseQuestion(b)
	}
	if err != nil {
		if p.st.LogHold == nil || *p.st.LogHold != pos {
			p.st.LogHold = &pos
			return false
		}
		p.logf("%s の問い %d の JSON が読めない（知らせない）: %v", id, n, err)
		p.st.LogHold = nil
		return true
	}
	p.st.LogHold = nil
	p.notify(id, "asked", map[string]any{"n": n, "kind": q.Kind, "question": q.Question})
	return true
}

// ---------------------------------------------------------------- ready の差

// readReady は kanban ready --json の人のタスクを除いた id の集まりを前回と比べ、増えたものを積む。
func (p *project) readReady() {
	out, err := p.l.Run("", "ready", "--json")
	var tasks []ledger.Task
	if err == nil {
		err = json.Unmarshal([]byte(out), &tasks)
	}
	if err != nil {
		p.logf("kanban ready が読めない（前回の ready を残す）: %v", err)
		return
	}
	ids := []string{}
	for _, t := range tasks {
		id := t.Str("id")
		if t.Str("who") == "person" || !idRe.MatchString(id) {
			continue
		}
		ids = append(ids, id)
		if p.st.Ready == nil || slices.Contains(*p.st.Ready, id) {
			continue
		}
		d := map[string]any{"title": t.Str("title")}
		if r := t.Str("routine"); r != "" {
			d["routine"] = r
		}
		p.notify(id, "ready", d)
	}
	p.st.Ready = &ids
}

// ---------------------------------------------------------------- 止まった担当

// Resend は提供元のエラーで続きを打った回数と最後に打った時刻（Unix ミリ秒）。
type Resend struct {
	N  int   `json:"n"`
	At int64 `json:"at"`
}

// Seen は担当の端末の1回の様子。
type Seen struct {
	Gone      bool   // 端末が無い・終わった
	Status    string // status.state
	StatusMs  int64
	Reviewing bool // 子のレビュアーのタスクが todo／doing
	Unsent    bool // ラベル first_message.sent=false（最初のメッセージが届いていない。courier.md §5）
}

// Verdict は Judge の答え。Kind が空なら知らせない。Retry なら続きを打つ。
type Verdict struct {
	Kind  string
	Key   string // 同じ状態につき1回だけ知らせるための鍵
	Retry bool
}

// Judge は担当の様子 s から、知らせるか・続きを打つかを決める（courier.md §2.3）。screen は error のときだけ読む。
func Judge(s Seen, screen func() string, r Resend, nowMs int64) Verdict {
	key := s.Status + "|" + strconv.FormatInt(s.StatusMs, 10)
	age := time.Duration(nowMs-s.StatusMs) * time.Millisecond
	switch {
	case s.Gone:
		return Verdict{Kind: "finished", Key: "gone"}
	case s.Status == "blocked":
		return Verdict{Kind: "blocked", Key: key}
	case s.Status == "error":
		if !ProviderError(screen()) {
			return Verdict{Kind: "error", Key: key}
		}
		if r.N >= len(retryAfter) {
			return Verdict{Kind: "error", Key: "retry-out"}
		}
		// 再送しても同じ error のまま status_ms が変わらないことがあるので、遅いほうから測る
		if time.Duration(nowMs-max(s.StatusMs, r.At))*time.Millisecond >= retryAfter[r.N] {
			return Verdict{Retry: true}
		}
	case (s.Status == "idle" || s.Status == "done") && age >= idleAfter && !s.Reviewing:
		return Verdict{Kind: "stalled", Key: key}
	case s.Status == "working" && age >= workingAfter:
		return Verdict{Kind: "stalled", Key: key}
	}
	return Verdict{}
}

// watch は doing のタスクの担当を端末の土台で見る（見回りで hc get）。
func (p *project) watch(now time.Time) {
	nowMs := now.UnixMilli()
	reviewing := map[string]bool{}
	for _, t := range p.tasks {
		if s := t.Str("status"); t.Str("parent") != "" && (s == "todo" || s == "doing") {
			reviewing[t.Str("parent")] = true
		}
	}
	sent, retry, first := map[string]bool{}, map[string]Resend{}, map[string]bool{}
	for _, t := range p.tasks {
		id, agent := t.Str("id"), t.Str("agent")
		if t.Str("status") != "doing" || t.Str("who") == "person" || agent == "" {
			continue
		}
		if !idRe.MatchString(id) || !agentRe.MatchString(agent) {
			continue
		}
		rk := id + "|" + agent
		r := p.st.Retry[rk]
		s, ok := p.seen(agent)
		if !ok { // 取れない間は前の状態（知らせた状態・続きを打った回数）を保つ
			for k := range p.st.Sent {
				if strings.HasPrefix(k, rk+"|") {
					sent[k] = true
				}
			}
			if _, had := p.st.Retry[rk]; had {
				retry[rk] = r
			}
			if p.st.First[rk] {
				first[rk] = true
			}
			continue
		}
		if s.Unsent && !p.first && p.sendFirst(t, agent, s.Status, p.st.First[rk]) {
			first[rk] = true
		}
		s.Reviewing = reviewing[id]
		v := Judge(s, func() string { out, _ := hcOut("read", agent, "--lines", "40"); return out }, r, nowMs)
		if v.Retry && !p.first {
			// 送れなかった試行も回数と時刻に数える（送れないまま見回りごとに打ち続けない）
			r = Resend{N: r.N + 1, At: nowMs}
			if p.retry(agent) {
				p.logf("%s の担当 %s に続きを打った（提供元のエラー、%d 回目）", id, agent, r.N)
			} else {
				p.logf("%s の担当 %s に続きを打てなかった（%d 回目に数える）", id, agent, r.N)
			}
		}
		// 回数は error・working の間は持ち越し（「続けて」を数える）、idle／done に戻ったら区切る
		if s.Status != "idle" && s.Status != "done" && !s.Gone && (r.N > 0 || r.At > 0) {
			retry[rk] = r
		}
		if v.Kind == "" {
			continue
		}
		k := rk + "|" + v.Key
		if !p.st.Sent[k] && !p.first {
			p.notify(id, v.Kind, map[string]any{"agent": agent, "status_ms": s.StatusMs})
		}
		sent[k] = true
	}
	p.st.Sent, p.st.Retry, p.st.First = sent, retry, first
}

// sendFirst は最初のメッセージが届いていない担当（ラベル first_message.sent=false）へ、見回りで1回だけ送る
// （courier.md §5 の「起こしそこねたら続きから」。andon task start・andon chief-engineer が終了コード 3 で預けたものと、
// クーリエが起こしたレビュアーで届かなかったもの。hc run が送れなかったときも、送ったが動き出さなかったときも）。
// 送る前に控えに「送りかけ」を書いて保存し、送ったら（人が手で送って動き出していても）ラベルを外す。
// 外しそこねたら true を返し、控えに「送った」を持って二度と送らない。
// 止まっている（blocked・error）間は送らず次の見回りへ。hc send も、画面が入力を受けられなければ拒む。
func (p *project) sendFirst(t ledger.Task, agent, status string, done bool) (keep bool) {
	id := t.Str("id")
	switch {
	case done:
	case status == "working" || status == "done":
		p.logf("%s の担当 %s は最初のメッセージのラベルが残っているが、もう動いている（手で送られた）。送らない", id, agent)
	case status == "idle" || status == "":
		msg := spawn.FirstMessage(p.l.Slug, id, filepath.Join(p.l.Dir, "tasks", id, "brief.md"))
		if isReviewer(t) {
			msg = p.reviewerFirst(id) // 伝令が起こしたレビュアー（startReviewer と同じ文）
		}
		// 送る前に控えへ「送りかけ」を書いて保存する（送ってからラベルを外す前にクーリエが落ちても、再開後に二度送らない。
		// 送っている間に落ちたときは届いたか分からないまま送らない側に倒れる。担当は idle のままなので stalled で Chief of Staff に届く）
		rk := id + "|" + agent
		p.st.First[rk] = true
		if err := p.st.Save(p.l.Slug); err != nil {
			delete(p.st.First, rk)
			p.logf("%s の担当 %s に最初のメッセージを送らない（控えを書けない。次の見回りで）: %v", id, agent, err)
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "hc", "send", agent, msg)
		cmd.Env = withoutTerminal()
		if err := cmd.Run(); err != nil {
			// 送る前の保存と対: 送りかけをその場で消して保存する（pass の最後の保存より前に落ちても、次の見回りで送り直す）
			delete(p.st.First, rk)
			if serr := p.st.Save(p.l.Slug); serr != nil {
				p.logf("%s の担当 %s の送りかけを控えから消せない: %v", id, agent, serr)
			}
			p.logf("%s の担当 %s に最初のメッセージを送れない（次の見回りで送り直す）: %v", id, agent, err)
			return false
		}
		p.logf("%s の担当 %s に最初のメッセージを送った（起こしたときに届かなかった分）", id, agent)
	default:
		return false // blocked・error: 画面が入力を受けられるようになるまで待つ
	}
	cmd := exec.Command("hc", "label", agent, spawn.FirstLabel+"=")
	cmd.Env = withoutTerminal()
	if err := cmd.Run(); err != nil {
		p.logf("%s の担当 %s のラベル %s を外せない（控えに送ったと持つ）: %v", id, agent, spawn.FirstLabel, err)
		return true
	}
	return false
}

// seen は担当の端末の様子。hc get が not_found を返せば端末が無い。ほかの失敗・形が読めないときは ok=false（この回は見ない）。
func (p *project) seen(agent string) (Seen, bool) {
	cmd := exec.Command("hc", "get", agent, "--json")
	cmd.Env = withoutTerminal()
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
			return Seen{Gone: true}, true
		}
		p.logf("hc get %s が失敗した（この回は前の状態を保つ）: %v", agent, err)
		return Seen{}, false
	}
	var t terminal
	if json.Unmarshal(out, &t) != nil || t.State == "" {
		p.logf("hc get %s の形が読めない", agent)
		return Seen{}, false
	}
	if t.State != "live" {
		return Seen{Gone: true, StatusMs: t.StatusMs}, true
	}
	if spawn.BareShell(t.Argv) && t.Foreground != nil && spawn.BareShell(t.Foreground.Argv) {
		// 作り直しで top-level の argv ごと素のシェルに替わって戻った端末（状態を報告しないので stalled にもならない）。終わった端末として知らせる
		return Seen{Gone: true, StatusMs: t.StatusMs}, true
	}
	return Seen{Status: t.Status.State, StatusMs: t.StatusMs, Unsent: t.Labels[spawn.FirstLabel] == "false"}, true
}

// retry は担当に続きを打つ（担当は人が打つ端末ではないので、離席の守りは要らない）。
func (p *project) retry(agent string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hc", "send", agent, RetryText)
	cmd.Env = withoutTerminal()
	return cmd.Run() == nil
}

// ensure は控えの map を作る（古い控えにも無い）。
func (s *State) ensure() {
	if s.Sent == nil {
		s.Sent = map[string]bool{}
	}
	if s.Retry == nil {
		s.Retry = map[string]Resend{}
	}
	if s.First == nil {
		s.First = map[string]bool{}
	}
}
