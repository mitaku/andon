package dispatcher

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
	"unicode"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// 通知の元（docs/dispatcher.md §2.2・§2.3）: log.jsonl の続き・ready の差・doing の担当の端末の状態を拾って控えに積む。
// 初めて見る元（控えに位置や前回の ready が無い）は、今を控えるだけで積まない。

const (
	idleAfter    = 15 * time.Minute // idle／done のまま、これ以上で stalled
	workingAfter = 20 * time.Minute // working のまま、これ以上で stalled
)

// retryAfter は提供元に切られた端末に「続けて」を打つまでの間。i 回目に切られたら retryAfter[i-1] あとに打つ。
// 個数（4）を打ち切ると、5 回目に切られたところで打つのをやめて知らせる（dispatcher.md §2.4。shepherdr〈MIT〉と同じ間隔）。
var retryAfter = []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute}

// RetryText は提供元に切られた端末に打つ文。やり直しに承認が要らないことと、送り手を書く。
const RetryText = "一時のエラーで止まった。やり直しに承認は要らない。止まる前の作業を続けて（dispatcher から）"

var (
	authErrRe      = regexp.MustCompile(`(?i)unauthori[sz]ed|\b401\b|\b403\b|invalid api key|/login|authenticat|oauth token`)
	allowanceErrRe = regexp.MustCompile(`(?i)usage limit|hit your [a-z ]*limit|\bquota\b|billing|credit balance`)
	providerErrRe  = regexp.MustCompile(`(?i)overloaded|at capacity|rate.?limit|too many requests|\b429\b|API Error: 5[0-9]{2}|\b(500|502|503|504|529)\b|internal server error|service unavailable|bad gateway|connection (error|reset|refused|closed)|ECONNRESET|stream disconnected|stopped arriving|request timed out|high demand|混雑`)
	agentRe        = regexp.MustCompile(`^(?:[A-Za-z0-9][A-Za-z0-9_.-]{0,62}/)?[A-Za-z0-9_-]{1,40}$`) // <address> か <machine>/<address>（contract.md §8）
)

// ProviderError は文（hc の status.msg か画面）が、提供元の混雑・rate limit・接続・5xx で切られたものか。
// 待てば通る見込みのものだけで、認証の失敗と使用量の上限（待っても直らない）は含めない。dispatcher.md §2.4。
func ProviderError(text string) bool {
	return !authErrRe.MatchString(text) && !allowanceErrRe.MatchString(text) && providerErrRe.MatchString(text)
}

// AllowanceError は文が使用量の上限（usage limit・quota・billing など。待っても直らない）で止まったものか（認証の失敗は含めない）。
func AllowanceError(text string) bool {
	return !authErrRe.MatchString(text) && allowanceErrRe.MatchString(text)
}

// collect は通知の元を拾う（pass が控えの lock を持って呼ぶ）。
func (p *project) collect(cos string) {
	p.readReady()
	p.readLog(cos)
	now := time.Now()
	p.watch(now, cos)
	p.estimate(now, cos) // 見込みとのズレ（dispatcher.md §2.10）
	p.chiefCut(cos)
}

// ---------------------------------------------------------------- log.jsonl の続き

// readLog は log.jsonl を前回の続き（バイト位置）から、最後の改行まで読む。Chief of Staff 自身の行は知らせない。
func (p *project) readLog(cos string) {
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
	by := "terminal:" + cos
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
	if json.Unmarshal(line, &r) != nil || r.Change == nil || !idRe.MatchString(r.ID) {
		return true
	}
	r.ID = ledger.CanonID(r.ID) // 移す前の行は A-5 と書いてある（kanban migrate は log.jsonl を書き換えない）
	c, id := r.Change, r.ID
	// done は Chief of Staff が打った finish（人がマージしたあと）も拾う: 影響（andon issue impact）は Chief of Staff もまだ見ていない事実
	if c["status"] == "done" {
		if p.st.Impact == nil {
			p.st.Impact = map[string]int{}
		}
		if _, ok := p.st.Impact[id]; !ok {
			p.st.Impact[id] = 0
		}
		// 人の手直しも同じ done の行から1回だけ集める（corrections.go）
		if p.st.Corrections == nil {
			p.st.Corrections = map[string]int{}
		}
		if _, ok := p.st.Corrections[id]; !ok {
			p.st.Corrections[id] = 0
		}
		return true
	}
	if r.By == by {
		return true
	}
	switch {
	case c["report"] == "write" || c["report"] == "append":
		p.notify(id, "report", map[string]any{"report": p.reportPath(id)})
	case c["question"] != nil && c["status"] == "open":
		return p.asked(id, c["question"], pos)
	case c["status"] == "review":
		// detail の pr は report.json から。PR の無い報告（レビュアー・repo の無い課題）は report として知らせる
		rep, err := contract.ReadReport(filepath.Join(p.l.Library(id), "report.json"))
		if err == nil && rep.PR != "" {
			p.notify(id, "review", map[string]any{"report": p.reportPath(id), "pr": rep.PR})
		} else {
			p.notify(id, "report", map[string]any{"report": p.reportPath(id)})
		}
	}
	return true
}

// asked は問いの行を通知 asked にする（中身は library/questions/<n>.json）。andon issue ask は台帳に書いてから JSON を置くので、
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

// readReady は kanban ready --json の人の課題を除いた id の集まりを前回と比べ、増えたものを積む。
func (p *project) readReady() {
	out, err := p.l.Run("", "ready", "--json")
	var tasks []ledger.Issue
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

// Resend は提供元に切られて「続けて」を打った回数と時刻（dispatcher.md §2.4）。打てなかった試行も数える。
// 時刻はどれも dispatcher の時計（Unix ミリ秒）。端末の status_ms は端末の土台の時計なので、時刻としては比べず、
// 切断の見分け（同じ error か・新しく切られたか）にだけ使う。
type Resend struct {
	N     int   `json:"n"`
	At    int64 `json:"at"`              // 最後に打った（打とうとした）時刻
	Cut   int64 `json:"cut,omitempty"`   // いま見ている error の status_ms（切断の見分け）
	Since int64 `json:"since,omitempty"` // その error を dispatcher が初めて見た時刻
	Sent  int64 `json:"sent,omitempty"`  // 最後に打ったときの Cut（これと違う error なら、打ったあとにまた切られた）
}

// Saw は error の様子 s を控えに映す: 新しい status_ms の error なら、それを dispatcher が初めて見た時刻を nowMs で持つ。
func (r Resend) Saw(s Seen, nowMs int64) Resend {
	if s.Status == "error" && s.StatusMs != r.Cut {
		r.Cut, r.Since = s.StatusMs, nowMs
	}
	return r
}

// kept は控えに残すものがあるか。
func (r Resend) kept() bool { return r.N > 0 || r.At > 0 || r.Cut != 0 }

// Seen は担当の端末の1回の様子。
type Seen struct {
	Gone      bool   // 端末が無い・終わった
	Status    string // status.state
	Msg       string // status.msg（program が状態に添えた文。Claude Code は error のとき API のエラーの文を入れる。Codex は入れない）
	StatusMs  int64
	Reviewing bool  // 子のレビュアーの課題が todo／doing
	Moved     int64 // working の間に status.msg か画面が最後に動いた時刻（dispatcher の時計の Unix ミリ秒）。0 は見ていない
	Unsent    bool  // 最初のメッセージがまだ届いていない（ラベル first_message.sent=false で、控えの first にも無い。dispatcher.md §5）
	Turn      int   // hc の turn（終えたターンの数。resume した端末が働き始めたかの見分け）
}

// Verdict は Judge の答え。Kind が空なら知らせない。Retry なら続きを打つ。
type Verdict struct {
	Kind      string
	Key       string // 同じ状態につき1回だけ知らせるための鍵
	Retry     bool
	Allowance bool // error が使用量の上限（待っても直らない）。レビュアーなら fallback で起こし直せるか見る
}

// ScreenCut は画面から、Codex がターンを止めたときに出すエラーの行（行頭が ■ の最後の行。実測 Codex 0.166.0）だけを返す。
// 無ければ空。画面のほかの行（エージェントの出力・ファイルの中身）は、提供元の語を含んでいても見分けに使わない。
func ScreenCut(screen string) string {
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(strings.ReplaceAll(lines[i], "\u00a0", " ")); strings.HasPrefix(l, "■") {
			return l
		}
	}
	return ""
}

// Judge は担当の様子 s から、知らせるか・続きを打つかを決める（dispatcher.md §2.3・§2.4）。
// 提供元に切られたかは status.msg で見分け、無ければ（Codex）screen の ■ の行（ScreenCut）を読む。screen は error で msg が無いときだけ読む。
func Judge(s Seen, screen func() string, r Resend, nowMs int64) Verdict {
	key := s.Status + "|" + strconv.FormatInt(s.StatusMs, 10)
	age := time.Duration(nowMs-s.StatusMs) * time.Millisecond
	switch {
	case s.Gone:
		return Verdict{Kind: "finished", Key: "gone"}
	case s.Status == "blocked":
		return Verdict{Kind: "blocked", Key: key}
	case s.Status == "error":
		text := s.Msg
		if text == "" {
			text = ScreenCut(screen())
		}
		// 最初のメッセージが届いていない端末に「続けて」は意味が無い（続ける仕事を受け取っていない）。知らせる
		if s.Unsent || !ProviderError(text) {
			return Verdict{Kind: "error", Key: key, Allowance: !s.Unsent && AllowanceError(text)}
		}
		r = r.Saw(s, nowMs) // 呼び手が映していなくても同じ答えにする
		if r.N >= len(retryAfter) {
			// 打ち切るのは、最後に打ったあとにまた切られた（5 回目の切断。status_ms が打ったときと違う）か、
			// 打っても最後の間だけ待って同じ error のままのとき。打った直後の見回りで、まだ前の切断の error を見ているだけなら待つ
			if r.Cut != r.Sent || time.Duration(nowMs-r.At)*time.Millisecond >= retryAfter[len(retryAfter)-1] {
				return Verdict{Kind: "error", Key: "retry-out"}
			}
			return Verdict{}
		}
		// その error を初めて見た時刻と前に打った時刻の遅いほうから測る（どちらも dispatcher の時計。
		// 打っても同じ error のまま status_ms が変わらないことがある）
		if time.Duration(nowMs-max(r.Since, r.At))*time.Millisecond >= retryAfter[r.N] {
			return Verdict{Retry: true}
		}
	case (s.Status == "idle" || s.Status == "done") && age >= idleAfter && !s.Reviewing:
		return Verdict{Kind: "stalled", Key: key}
	case s.Status == "working" && age >= workingAfter && (s.Moved == 0 || time.Duration(nowMs-s.Moved)*time.Millisecond >= workingAfter):
		// 1つのターンで長く働いているだけ（status.msg か画面が動いている）なら知らせない
		return Verdict{Kind: "stalled", Key: key}
	}
	return Verdict{}
}

// Moved は working の担当の進みの控え: status.msg と画面の要約（別々に持つ。画面が読めない回も msg は比べる）と、
// どちらかが最後に替わった時刻（dispatcher の時計）。
type Moved struct {
	Msg    string `json:"msg"`
	Screen string `json:"screen,omitempty"` // 空は画面をまだ読めていない
	At     int64  `json:"at"`
}

// Progress は画面の要約。字だけを見る（数字・記号・空白を除く。経過の時計・トークン数・回る印は進みに数えない）。
func Progress(screen string) string {
	var b strings.Builder
	for _, r := range screen {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return digest(b.String())
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// moved は working の担当の進みを控えに映す（prev が前の控え）。status.msg か画面が替われば今を、替わらなければ前の時刻を持つ。
// 画面が読めない回は画面を比べず、前の画面の要約を持ち越す（msg は比べる）。初めて見たら今を起点にする。
func moved(prev Moved, had bool, msg string, screen func() (string, error), nowMs int64) Moved {
	next := Moved{Msg: digest(msg), Screen: prev.Screen, At: prev.At}
	if text, err := screen(); err == nil {
		next.Screen = Progress(text)
	}
	if !had || next.Msg != prev.Msg || next.Screen != prev.Screen {
		next.At = nowMs
	}
	return next
}

// watch は doing の課題の担当を端末の土台で見る（見回りで hc get）。
func (p *project) watch(now time.Time, cos string) {
	nowMs := now.UnixMilli()
	reviewing := map[string]bool{}
	for _, t := range p.tasks {
		if s := t.Str("status"); t.Str("parent") != "" && (s == "todo" || s == "doing") {
			reviewing[t.Str("parent")] = true
		}
	}
	sent, retry, first, mv := map[string]bool{}, map[string]Resend{}, map[string]bool{}, map[string]Moved{}
	for _, t := range p.tasks {
		id, agent := t.Str("id"), t.Str("agent")
		if t.Str("status") != "doing" || t.Str("who") == "person" || agent == "" {
			continue
		}
		if agent == cos {
			continue // Chief of Staff の端末は人が打つので、担当として見ない（打たない。chiefCut が見る。ADR-0019 決めたこと 4）
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
			if m, had := p.st.Moved[rk]; had {
				mv[rk] = m
			}
			continue
		}
		if s.Gone {
			p.sawGone(id, agent, now) // 実働から死んでいた区間を引く（estimate.go）
		}
		if s.Unsent && !p.first && p.sendFirst(t, agent, s.Status, p.st.First[rk]) {
			first[rk] = true
		}
		// 控えに「送った」があればラベルが残っていても届いたとみる（続きを打ってよい）
		s.Unsent = s.Unsent && !p.st.First[rk] && !first[rk]
		s.Reviewing = reviewing[id]
		if rs, had := p.st.Resumed[id]; had && !s.Gone && rs.Agent == agent && s.Turn >= 1 {
			p.worked(id) // 起こした端末が働き始めた。控えを消してその場で保存する
		}
		if s.Status == "working" && !s.Gone {
			prev, had := p.st.Moved[rk]
			m := moved(prev, had, s.Msg, func() (string, error) { return hcOut("read", agent, "--lines", "40") }, nowMs)
			mv[rk], s.Moved = m, m.At
		}
		r = r.Saw(s, nowMs)
		v := Judge(s, func() string { out, _ := hcOut("read", agent, "--lines", "40"); return out }, r, nowMs)
		if v.Retry && !p.first {
			if next, ok := p.countResend(rk, r, nowMs); ok {
				r = next
				if p.retry(agent) {
					p.logf("%s の担当 %s に続きを打った（提供元に切られた、%d 回目）", id, agent, r.N)
				} else {
					p.logf("%s の担当 %s に続きを打てなかった（%d 回目に数える）", id, agent, r.N)
				}
			} else {
				p.logf("%s の担当 %s に続きを打たない（控えを書けない。次の見回りで）", id, agent)
			}
		}
		// 回数は error・working の間だけ持ち越し（「続けて」を数える）、ほかの状態（idle・done・blocked など）に戻ったら区切る
		if (s.Status == "error" || s.Status == "working") && !s.Gone && r.kept() {
			retry[rk] = r
		}
		if v.Kind == "" {
			continue
		}
		if s.Gone && p.resumeGone(t, agent) {
			continue // resume で起こした（次の見回りからは新しい端末を見る）。起こせなければ今どおり finished
		}
		k := rk + "|" + v.Key
		if v.Allowance && isReviewer(t) && !p.first && !p.st.Sent[k] && p.fallback(t, agent) {
			continue // 起こし直した（次の見回りからは新しい端末を見る）
		}
		if !p.st.Sent[k] && !p.first {
			p.notify(id, v.Kind, map[string]any{"agent": agent, "status_ms": s.StatusMs})
		}
		sent[k] = true
	}
	// Chief of Staff の分（chiefCut が持つ。キーは cosKey で始まり、課題の id とは重ならない）は持ち越す
	for k := range p.st.Sent {
		if strings.HasPrefix(k, cosKey) {
			sent[k] = true
		}
	}
	p.st.Sent, p.st.Retry, p.st.First, p.st.Moved = sent, retry, first, mv
	p.keepResumed()
}

// countResend は打つ前に、回数と時刻を1つ進めた控えを保存する（打ってから pass の最後の保存の前に dispatcher が落ちても、
// 再開後に同じ切断へ打ち直さず、上限も超えない。送れなかった・届いたか分からない試行も1回に数える）。
// 保存できなければ控えを戻して false（打たない）。
func (p *project) countResend(key string, r Resend, nowMs int64) (Resend, bool) {
	prev, had := p.st.Retry[key]
	next := r
	next.N, next.At, next.Sent = r.N+1, nowMs, r.Cut
	p.st.Retry[key] = next
	if err := p.st.Save(p.l.Slug); err != nil {
		if had {
			p.st.Retry[key] = prev
		} else {
			delete(p.st.Retry, key)
		}
		return r, false
	}
	return next, true
}

// cosKey は控えの Sent で Chief of Staff の端末を指すキーの頭（":cos|<address>"。: は課題の id に使えない）。
const cosKey = ":cos|"

// chiefCut は Chief of Staff の端末が提供元に切られて error で止まったとき、ログに1行だけ残す（dispatcher.md §2.4）。
// 「続けて」は打たない: Chief of Staff は人が打つ端末で、hc send は人の書きかけに続けて送ってしまう（ADR-0019 決めたこと 4）。
// 起こすのは StopFailure のフック（rewake.go。ADR-0019 決めたこと 10）。知らせる相手が止まっているので通知も積まない。同じ止まりには1回
// （error・working の間は持ち越し、ほかに戻ったら区切る。Claude Code 自身のやり直しで working を挟んでも1回）。
func (p *project) chiefCut(cos string) {
	key := cosKey + cos
	for k := range p.st.Sent { // 替わった前の Chief of Staff の分は捨てる
		if strings.HasPrefix(k, cosKey) && !strings.HasPrefix(k, key+"|") {
			delete(p.st.Sent, k)
		}
	}
	if p.first || !agentRe.MatchString(cos) {
		return
	}
	c, err := hcGet(cos)
	if err != nil || c.State != "live" {
		return // 取れない間は前の控えを保つ
	}
	if c.Status.State != "error" && c.Status.State != "working" {
		delete(p.st.Sent, key+"|cut")
		return
	}
	if c.Status.State != "error" || p.st.Sent[key+"|cut"] {
		return
	}
	text := c.Status.Msg
	if text == "" {
		out, _ := hcOut("read", cos, "--lines", "40")
		text = ScreenCut(out)
	}
	if !ProviderError(text) {
		return
	}
	p.st.Sent[key+"|cut"] = true
	p.logf("Chief of Staff %s は提供元に切られて止まった。続きは打たない（StopFailure のフックが起こす。5 回で止まれば人が Herdr App の error で気づく）", cos)
}

// sendFirst は最初のメッセージが届いていない担当（ラベル first_message.sent=false）へ、見回りで1回だけ送る
// （dispatcher.md §5 の「起こしそこねたら続きから」。andon issue start・andon chief-engineer が終了コード 3 で預けたものと、
// dispatcher が起こしたレビュアーで届かなかったもの。hc run が送れなかったときも、送ったが動き出さなかったときも）。
// 送る前に控えに「送りかけ」を書いて保存し、送ったら（人が手で送って動き出していても）ラベルを外す。
// 外しそこねたら true を返し、控えに「送った」を持って二度と送らない。
// 止まっている（blocked・error）間は送らず次の見回りへ。hc send も、画面が入力を受けられなければ拒む。
// 送る前に入力欄を読み（hc read）、前に貼った文の1行目が残っていれば貼り直さず Enter だけ押す（LeftInInput。ADR-0007 R6）。
func (p *project) sendFirst(t ledger.Issue, agent, status string, done bool) (keep bool) {
	id := t.Str("id")
	switch {
	case done:
	case status == "working" || status == "done":
		p.logf("%s の担当 %s は最初のメッセージのラベルが残っているが、もう動いている（手で送られた）。送らない", id, agent)
	case status == "idle" || status == "":
		msg := spawn.FirstMessage(p.l.Slug, id, filepath.Join(p.l.Dir, "issues", id, "brief.md")) + spawn.Guide()
		if isReviewer(t) {
			msg = p.reviewerFirst(id) // dispatcher が起こしたレビュアー（startReviewer と同じ文）
		}
		// 前に貼った文が入力欄に残っていれば（hc send が貼ったが Enter が効かなかった。ADR-0007 B5）、貼り直さず Enter だけ押す。
		// 読めなければ送らない（重ねないほうに倒す。次の見回りで読み直す）
		screen, err := hcOut("read", agent)
		if err != nil {
			p.logf("%s の担当 %s の入力欄を読めない（最初のメッセージは次の見回りで送る）: %v", id, agent, err)
			return false
		}
		left := LeftInInput(screen, strings.SplitN(msg, "\n", 2)[0])
		// 送る前に控えへ「送りかけ」を書いて保存する（送ってからラベルを外す前に dispatcher が落ちても、再開後に二度送らない。
		// 送っている間に落ちたときは届いたか分からないまま送らない側に倒れる。担当は idle のままなので stalled で Chief of Staff に届く）
		rk := id + "|" + agent
		p.st.First[rk] = true
		if err := p.st.Save(p.l.Slug); err != nil {
			delete(p.st.First, rk)
			p.logf("%s の担当 %s に最初のメッセージを送らない（控えを書けない。次の見回りで）: %v", id, agent, err)
			return false
		}
		args := []string{"send", agent, msg}
		if left {
			args = []string{"keys", agent, "enter"}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "hc", args...)
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
		if left {
			p.logf("%s の担当 %s の入力欄に最初のメッセージが残っていた。Enter だけ押した（貼り直さない）", id, agent)
		} else {
			p.logf("%s の担当 %s に最初のメッセージを送った（起こしたときに届かなかった分）", id, agent)
		}
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

// LeftInInput は画面（hc read）の入力欄の行に、最初のメッセージの1行目 first が残っているか。
// 入力欄の行は、プロンプトの印（Claude Code の ❯、Codex の ›）で始まる最後の行（その上の同じ印の行は送った文の写し）。
// 入力欄の薄い字のサジェストは色が無いと文と見分けられないが、1行目は課題の slug と id を含むので、まだ会話に入っていない文と
// 同じ行がサジェストに出ることはない。印の無い画面（読めない形）は残っていないとみる（今までどおり送る）。
func LeftInInput(screen, first string) bool {
	if first = strings.TrimSpace(first); first == "" {
		return false
	}
	lines := strings.Split(screen, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "❯") || strings.HasPrefix(l, "›") {
			return strings.Contains(l, first)
		}
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
	return Seen{Status: t.Status.State, Msg: t.Status.Msg, StatusMs: t.StatusMs, Unsent: t.Labels[spawn.FirstLabel] == "false", Turn: t.Turn}, true
}

// retry は担当・レビュアーの端末に続きを打つ（人が打つ端末ではない）。Chief of Staff には打たない（chiefCut）。
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
	if s.Watch == nil {
		s.Watch = map[string]Watch{}
	}
	if s.Approved == nil {
		s.Approved = map[string]Notification{}
	}
	if s.Resumed == nil {
		s.Resumed = map[string]Resumed{}
	}
	if s.Waits == nil {
		s.Waits = map[string]string{}
	}
	if s.Estimate == nil {
		s.Estimate = map[string]bool{}
	}
	if s.Gone == nil {
		s.Gone = map[string][]GoneAt{}
	}
}
