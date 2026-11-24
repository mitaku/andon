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

	"go.yaml.in/yaml/v3"
)

// 通知の元（docs/dispatcher.md §2.2・§2.3）: log.jsonl の続き・ready の差・doing の担当の端末の状態を拾って控えに積む。
// 初めて見る元（控えに位置や前回の ready が無い）は、今を控えるだけで積まない。

const (
	idleAfter    = 15 * time.Minute
	workingAfter = 20 * time.Minute
)

// retryAfter はプロバイダに切られた端末に「続けて」を打つまでの間。i 回目に切られたら retryAfter[i-1] あとに打つ。
// 個数（4）を打ち切ると、5 回目に切られたところで打つのをやめて知らせる（dispatcher.md §2.4。shepherdr〈MIT〉と同じ間隔）。
var retryAfter = []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute}

const RetryText = "一時のエラーで止まった。やり直しに承認は要らない。止まる前の作業を続けて（dispatcher から）"

var (
	authErrRe      = regexp.MustCompile(`(?i)unauthori[sz]ed|\b401\b|\b403\b|invalid api key|/login|authenticat|oauth token`)
	allowanceErrRe = regexp.MustCompile(`(?i)usage limit|hit your [a-z ]*limit|\bquota\b|billing|credit balance`)
	providerErrRe  = regexp.MustCompile(`(?i)overloaded|at capacity|rate.?limit|too many requests|\b429\b|API Error: 5[0-9]{2}|\b(500|502|503|504|529)\b|internal server error|service unavailable|bad gateway|connection (error|reset|refused|closed)|ECONNRESET|stream disconnected|stopped arriving|connection lost|request timed out|high demand|混雑`)
	agentRe        = regexp.MustCompile(`^(?:[A-Za-z0-9][A-Za-z0-9_.-]{0,62}/)?[A-Za-z0-9_-]{1,40}$`) // <address> か <machine>/<address>（contract.md §8）
)

// ProviderError は文（hc の status.msg か画面）が、プロバイダの混雑・rate limit・接続・5xx で切られたものか。
// 待てば通る見込みのものだけで、認証の失敗と使用量の上限（待っても直らない）は含めない。dispatcher.md §2.4。
func ProviderError(text string) bool {
	return !authErrRe.MatchString(text) && !allowanceErrRe.MatchString(text) && providerErrRe.MatchString(text)
}

func AllowanceError(text string) bool {
	return !authErrRe.MatchString(text) && allowanceErrRe.MatchString(text)
}

func (p *project) collect(cos string) {
	p.readReady()
	p.readLog(cos)
	now := time.Now()
	p.watch(now, cos)
	p.estimate(now, cos) // 見込みとのズレ（dispatcher.md §2.10）
	p.chiefCut(cos)
	p.signals(now) // 外の信号（dispatcher.md §2.1.1 の 4e）
	p.memory()     // auto memory の量の線（dispatcher.md §2.11）
}

func (p *project) readLog(cos string) {
	path := filepath.Join(p.l.Dir, "log.jsonl")
	f, err := os.Open(path)
	var size int64
	if err == nil {
		defer f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
		if fi, err := f.Stat(); err == nil {
			size = fi.Size()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		p.logf("log.jsonl が読めない: %v", err)
		return
	}
	head := logHead(f)
	if p.st.LogPos == nil {
		p.st.LogPos, p.st.LogHead = &size, head
		return
	}
	pos := *p.st.LogPos
	switch {
	case size < pos:
		p.logf("log.jsonl が短くなった。新しいファイルとして先頭から読む")
		pos, p.st.LogHold = 0, nil
	case pos > 0 && p.st.LogHead != "" && head != p.st.LogHead:
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
		buf = nil
	}
	by := "terminal:" + cos
	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		line := buf[:i]
		if !p.logRow(line, by, pos) {
			break
		}
		pos += int64(i + 1)
		buf = buf[i+1:]
	}
	p.st.LogPos = &pos
}

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

func (p *project) logRow(line []byte, by string, pos int64) bool {
	var r ledger.Row
	if json.Unmarshal(line, &r) != nil || r.Change == nil || !idRe.MatchString(r.ID) {
		return true
	}
	r.ID = ledger.CanonID(r.ID)
	c, id := r.Change, r.ID
	if c["status"] == "done" {
		p.st.Impact.Add(id)
		p.st.Corrections.Add(id)
		p.st.Outcomes.Add(id)
		// 止めた理由の課題が済んでも止めは外れない。残っていれば Chief of Staff に1回知らせる（done の行は1回だけ読む。ADR-0056 決めたこと 7）
		p.heldOn(id)
		return true
	}
	if r.By == by {
		return true
	}
	switch {
	case c["report"] == "write" || c["report"] == "append":
		p.notify(id, "report", map[string]any{"report": p.reportPath(id)})
	case c["question"] != nil && c["status"] == "open":
		if r.By == ledger.DispatcherBy {
			if k := askedKey(id, c["question"]); p.st.Asked[k] {
				delete(p.st.Asked, k)
				return true
			}
		}
		return p.asked(id, c["question"], pos)
	case c["question"] != nil && c["status"] == "answered":
		// 端末の外から書かれた答え（オーナーのペインの andon waiting answer）だけを Chief of Staff に届ける。
		// 端末の中で書いた答え（Chief of Staff・dispatcher）は通知にしない（ADR-0042 決めたこと 18・19）
		if !strings.HasPrefix(r.By, "terminal:") {
			return p.answered(id, c["question"], pos)
		}
	case c["status"] == "review":
		rep, err := contract.ReadReport(filepath.Join(p.l.Library(id), "report.json"))
		if err == nil && rep.PR != "" {
			p.notify(id, "review", map[string]any{"report": p.reportPath(id), "pr": rep.PR})
		} else {
			p.notify(id, "report", map[string]any{"report": p.reportPath(id)})
		}
	}
	return true
}

func (p *project) markAsked(id string, n any) {
	k := askedKey(id, n)
	if k == "" {
		return
	}
	if p.st.Asked == nil {
		p.st.Asked = map[string]bool{}
	}
	p.st.Asked[k] = true
}

func askedKey(id string, n any) string {
	var i int
	switch v := n.(type) {
	case int:
		i = v
	case float64:
		if v != float64(int(v)) {
			return ""
		}
		i = int(v)
	default:
		return ""
	}
	if i < 1 {
		return ""
	}
	return id + "#" + strconv.Itoa(i)
}

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

func (p *project) answered(id string, v any, pos int64) bool {
	f, ok := v.(float64)
	n := int(f)
	if !ok || f != float64(n) || n < 1 {
		return true
	}
	b, err := os.ReadFile(filepath.Join(p.l.Dir, "issues", id, "questions.yaml"))
	var qs []answerRow
	if err == nil {
		err = yaml.Unmarshal(b, &qs)
	}
	if err != nil {
		if p.st.LogHold == nil || *p.st.LogHold != pos {
			p.st.LogHold = &pos
			return false
		}
		p.logf("%s の questions.yaml が読めない（答え %d を知らせない）: %v", id, n, err)
		p.st.LogHold = nil
		return true
	}
	p.st.LogHold = nil
	i := slices.IndexFunc(qs, func(q answerRow) bool { return q.N == n })
	if i < 0 || qs[i].Status != "answered" || qs[i].Answer == "" {
		return true
	}
	kind := qs[i].Kind
	if b, err := os.ReadFile(filepath.Join(p.l.Library(id), "questions", strconv.Itoa(n)+".json")); err == nil {
		if q, err := contract.ParseQuestion(b); err == nil {
			kind = q.Kind
		}
	}
	if !slices.Contains(questionKinds, kind) {
		p.logf("%s の問い %d の種類 %q が読めない（答えを知らせない）", id, n, kind)
		return true
	}
	p.notify(id, "answered", map[string]any{"n": n, "kind": kind, "answer": qs[i].Answer})
	return true
}

type answerRow struct {
	N      int    `yaml:"n"`
	Kind   string `yaml:"kind"`
	Status string `yaml:"status"`
	Answer string `yaml:"answer"`
}

// questionKinds は問いの種類（schema/question.schema.json の kind）。
var questionKinds = []string{"business", "users", "promise", "irreversible", "line", "scope"}

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
		if t.Str("who") == "person" || isReviewer(t) || !idRe.MatchString(id) {
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
		gone, unsure := p.waitedGone(t)
		if len(gone) > 0 {
			d["abandoned"] = gone
		}
		if len(unsure) > 0 {
			d["unchecked"] = unsure
		}
		p.notify(id, "ready", d)
	}
	p.st.Ready = &ids
}

// Resend はプロバイダに切られて「続けて」を打った回数と時刻（dispatcher.md §2.4）。打てなかった試行も数える。
// 時刻はどれも dispatcher の時計（Unix ミリ秒）。端末の status_ms は hc の時計なので、時刻としては比べず、
// 切断の見分け（同じ error か・新しく切られたか）にだけ使う。
type Resend struct {
	N     int   `json:"n"`
	At    int64 `json:"at"`
	Cut   int64 `json:"cut,omitempty"`
	Since int64 `json:"since,omitempty"`
	Sent  int64 `json:"sent,omitempty"`
}

func (r Resend) Saw(s Seen, nowMs int64) Resend {
	if s.Status == "error" && s.StatusMs != r.Cut {
		r.Cut, r.Since = s.StatusMs, nowMs
	}
	return r
}

func (r Resend) kept() bool { return r.N > 0 || r.At > 0 || r.Cut != 0 }

type Seen struct {
	Gone      bool
	Status    string
	Msg       string // status.msg（program が状態に添えた文。Claude Code は error のとき API のエラーの文を入れる。Codex は入れない）
	StatusMs  int64
	Reviewing bool
	Asking    bool
	Moved     int64
	Unsent    bool // 最初のメッセージがまだ届いていない（ラベル first_message.sent=false で、控えの first にも無い。dispatcher.md §5）
	Turn      int
}

type Verdict struct {
	Kind      string
	Key       string
	Retry     bool
	Allowance bool
}

// ScreenCut は画面から、Codex がターンを止めたときに出すエラーの行（行頭が ■ の最後の行。実測 Codex 0.166.0）だけを返す。
// 無ければ空。画面のほかの行（エージェントの出力・ファイルの中身）は、プロバイダの語を含んでいても見分けに使わない。
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
// プロバイダに切られたかは status.msg で見分け、無ければ（Codex）screen の ■ の行（ScreenCut）を読む。screen は error で msg が無いときだけ読む。
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
		if s.Unsent || !ProviderError(text) {
			return Verdict{Kind: "error", Key: key, Allowance: !s.Unsent && AllowanceError(text)}
		}
		r = r.Saw(s, nowMs)
		if r.N >= len(retryAfter) {
			if r.Cut != r.Sent || time.Duration(nowMs-r.At)*time.Millisecond >= retryAfter[len(retryAfter)-1] {
				return Verdict{Kind: "error", Key: "retry-out"}
			}
			return Verdict{}
		}
		if time.Duration(nowMs-max(r.Since, r.At))*time.Millisecond >= retryAfter[r.N] {
			return Verdict{Retry: true}
		}
	case (s.Status == "idle" || s.Status == "done") && age >= idleAfter && !s.Reviewing && !s.Asking:
		return Verdict{Kind: "stalled", Key: key}
	case s.Status == "working" && age >= workingAfter && (s.Moved == 0 || time.Duration(nowMs-s.Moved)*time.Millisecond >= workingAfter):
		return Verdict{Kind: "stalled", Key: key}
	}
	return Verdict{}
}

type Moved struct {
	Msg    string `json:"msg"`
	Screen string `json:"screen,omitempty"`
	At     int64  `json:"at"`
}

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

func Asking(rows []ledger.Row) map[string]bool {
	last := map[string]string{}
	for _, r := range rows {
		k := askedKey(ledger.CanonID(r.ID), r.Change["question"])
		st, ok := r.Change["status"].(string)
		if k == "" || !ok {
			continue
		}
		last[k] = st
	}
	asking := map[string]bool{}
	for k, st := range last {
		if st == "open" || st == "answered" {
			asking[k[:strings.LastIndex(k, "#")]] = true
		}
	}
	return asking
}

func (p *project) watch(now time.Time, cos string) {
	nowMs := now.UnixMilli()
	// 子のレビュアーの課題が todo／doing の親は止まって見えても待っている。オーナーの課題（who: person。
	// andon issue ask --new --parent で親を持つ）はレビュアーでないので数えない（ADR-0057 決めたこと 7）
	reviewing := map[string]bool{}
	for _, t := range p.tasks {
		if s := t.Str("status"); t.Str("parent") != "" && t.Str("who") != "person" && (s == "todo" || s == "doing") {
			reviewing[t.Str("parent")] = true
		}
	}
	asking := Asking(p.rows)
	sent, retry, first, mv := map[string]bool{}, map[string]Resend{}, map[string]bool{}, map[string]Moved{}
	for _, t := range p.tasks {
		id, agent := t.Str("id"), t.Str("agent")
		if t.Str("status") != "doing" || t.Str("who") == "person" || agent == "" {
			continue
		}
		if agent == cos {
			continue // Chief of Staff の端末はオーナーが打つので、担当として見ない（打たない。chiefCut が見る。ADR-0019 決めたこと 4）
		}
		if ledger.TakenByChiefOfStaff(p.rows, id, agent) {
			continue
		}
		if !idRe.MatchString(id) || !agentRe.MatchString(agent) {
			continue
		}
		rk := id + "|" + agent
		r := p.st.Retry[rk]
		s, ok := p.seen(agent)
		if !ok {
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
			p.sawGone(id, agent, now)
		}
		if s.Unsent && !p.first && p.sendFirst(t, agent, s.Status, p.st.First[rk]) {
			first[rk] = true
		}
		s.Unsent = s.Unsent && !p.st.First[rk] && !first[rk]
		s.Reviewing = reviewing[id]
		s.Asking = asking[id]
		if rs, had := p.st.Resumed[id]; had && !s.Gone && rs.Agent == agent && s.Turn >= 1 {
			p.worked(id)
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
					p.logf("%s の担当 %s に続きを打った（プロバイダに切られた、%d 回目）", id, agent, r.N)
				} else {
					p.logf("%s の担当 %s に続きを打てなかった（%d 回目に数える）", id, agent, r.N)
				}
			} else {
				p.logf("%s の担当 %s に続きを打たない（控えを書けない。次の見回りで）", id, agent)
			}
		}
		if (s.Status == "error" || s.Status == "working") && !s.Gone && r.kept() {
			retry[rk] = r
		}
		if v.Kind == "" {
			continue
		}
		if s.Gone && p.resumeGone(t, agent) {
			continue
		}
		if s.Gone && p.first {
			// 初めて見た死は Resumed に「見た」とだけ持ち、Sent には書かない（次の見回りで打たずに finished を知らせる。
			// Sent に書くと、resume も finished も出ないまま死んだ担当が残る。dispatcher.md §2.9）
			continue
		}
		k := rk + "|" + v.Key
		if v.Allowance && isReviewer(t) && !p.first && !p.st.Sent[k] && p.fallback(t, agent) {
			continue
		}
		if !p.st.Sent[k] && !p.first {
			p.notify(id, v.Kind, map[string]any{"agent": agent, "status_ms": s.StatusMs})
		}
		sent[k] = true
	}
	for k := range p.st.Sent {
		if strings.HasPrefix(k, cosKey) {
			sent[k] = true
		}
	}
	p.st.Sent, p.st.Retry, p.st.First, p.st.Moved = sent, retry, first, mv
	p.keepResumed()
}

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

const cosKey = ":cos|"

// chiefCut は Chief of Staff の端末がプロバイダに切られて error で止まったとき、ログに1行だけ残す（dispatcher.md §2.4）。
// 「続けて」は打たない: Chief of Staff はオーナーが打つ端末で、hc send はオーナーの書きかけに続けて送ってしまう（ADR-0019 決めたこと 4）。
// 起動するのは StopFailure のフック（rewake.go。ADR-0019 決めたこと 10）。知らせる相手が止まっているので通知も積まない。同じ止まりには1回
// （error・working の間は持ち越し、ほかに戻ったら区切る。Claude Code 自身のやり直しで working を挟んでも1回）。
func (p *project) chiefCut(cos string) {
	key := cosKey + cos
	for k := range p.st.Sent {
		if strings.HasPrefix(k, cosKey) && !strings.HasPrefix(k, key+"|") {
			delete(p.st.Sent, k)
		}
	}
	if p.first || !agentRe.MatchString(cos) {
		return
	}
	c, err := hcGet(cos)
	if err != nil || c.State != "live" {
		return
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
	p.logf("Chief of Staff %s はプロバイダに切られて止まった。続きは打たない（StopFailure のフックが起動する。5 回で止まればオーナーが Herdr App の error で気づく）", cos)
}

// sendFirst は最初のメッセージが届いていない担当（ラベル first_message.sent=false）へ、見回りで1回だけ送る
// （dispatcher.md §5 の「起動しそこねたら続きから」。andon issue start・andon chief-engineer が終了コード 3 で預けたものと、
// dispatcher が起動したレビュアーで届かなかったもの。hc run が送れなかったときも、送ったが動き出さなかったときも）。
// 送る前に控えに「送りかけ」を書いて保存し、送ったら（オーナーが手で送って動き出していても）ラベルを外す。
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
		if !isEngineer(t) {
			msg += spawn.Wiki() // wiki の目録の行は担当だけ。Chief Engineer には足さない（ADR-0061 決めたこと 12）
		}
		if isReviewer(t) {
			msg = p.reviewerFirst(id)
		}
		// hc run なら template の本文が前に付く。hc send は付けないので、起動のときに届くはずだった文に揃える
		msg = spawn.WithBody(t.Str("profile"), msg)
		// 前に貼った文が入力欄に残っていれば（hc send が貼ったが Enter が効かなかった。ADR-0007 B5）、貼り直さず Enter だけ押す。
		// 読めなければ送らない（重ねないほうに倒す。次の見回りで読み直す）
		screen, err := hcOut("read", agent)
		if err != nil {
			p.logf("%s の担当 %s の入力欄を読めない（最初のメッセージは次の見回りで送る）: %v", id, agent, err)
			return false
		}
		left := LeftInInput(screen, strings.SplitN(msg, "\n", 2)[0])
		rk := id + "|" + agent
		p.st.First[rk] = true
		if err := p.st.Save(p.l.Slug); err != nil {
			delete(p.st.First, rk)
			p.logf("%s の担当 %s に最初のメッセージを送らない（控えを書けない。次の見回りで）: %v", id, agent, err)
			return false
		}
		// 先頭に送り手の印を置く（端末の外から送るので hc の包みが付かない。spawn.Mark）。
		// 入力欄に残った文は印の無い1行目で照らす（hc run が貼った文には印が無く、印つきの行にも1行目は含まれる）
		args := []string{"send", agent, spawn.Mark(spawn.FromDispatcher, msg)}
		if left {
			args = []string{"keys", agent, "enter"}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "hc", args...)
		cmd.Env = withoutTerminal()
		if err := cmd.Run(); err != nil {
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
			p.logf("%s の担当 %s に最初のメッセージを送った（起動したときに届かなかった分）", id, agent)
		}
	default:
		return false
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
// 入力欄の薄い字のサジェストは色が無いと文と見分けられないが、1行目（template の本文の1行目。本文が無ければ課題の slug と id の行）は
// まだ会話に入っていない文なので、同じ行がサジェストに出ることはない。印の無い画面（読めない形）は残っていないとみる（今までどおり送る）。
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

func (p *project) retry(agent string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hc", "send", agent, spawn.Mark(spawn.FromDispatcher, RetryText))
	cmd.Env = withoutTerminal()
	return cmd.Run() == nil
}

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
	if s.Stuck == nil {
		s.Stuck = map[string]string{}
	}
	if s.Resumed == nil {
		s.Resumed = map[string]Resumed{}
	}
	if s.Estimate == nil {
		s.Estimate = map[string]bool{}
	}
	if s.Gone == nil {
		s.Gone = map[string][]GoneAt{}
	}
}
