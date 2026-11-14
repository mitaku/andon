package dispatcher

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
)

// 偽の hc（プロバイダに切られたときの再送を見る）: get は $D/<addr>.json（無ければ not_found）、read は $D/<addr>.screen。attach は $D/log に1行。
// send は $D/log に「send <addr> <本文>」を1行。label も1行（通れば $D/<addr>.unsent を消す。get が読むのは json の labels だけ）。
const retryHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) [ -e "$d/$2.json" ] || { echo '{"error":{"kind":"not_found"}}' >&2; exit 3; }; cat "$d/$2.json" ;;
  read) cat "$d/$2.screen" 2>/dev/null ;;
  attach) printf 'attach %s\n' "${*:2}" >>"$d/log" ;;
  send) printf 'send %s %s\n' "$2" "$(printf '%s' "$3" | tr '\n' '|')" >>"$d/log" ;;
  label) printf 'label %s\n' "${*:2}" >>"$d/log" ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

// 実測した Claude Code 2.1.319 の status.msg（偽の API が 529 を返したとき）。
const cutMsg = "API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment."

type retryFixture struct {
	t   *testing.T
	d   string
	p   *project
	log bytes.Buffer
	t0  time.Time
}

func newRetry(t *testing.T) *retryFixture {
	t.Helper()
	f := &retryFixture{t: t, d: t.TempDir(), t0: time.UnixMilli(1_800_000_000_000)}
	bin := filepath.Join(f.d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(retryHc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", f.d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.d, "state"))
	st := &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
	st.ensure()
	f.p = &project{c: &Dispatcher{Log: &f.log}, l: ledger.At(filepath.Join(f.d, "projects", "sample-site")), st: st,
		tasks: []ledger.Issue{{"id": "S-7", "status": "doing", "agent": "t42"}}}
	return f
}

// term は端末 addr の hc get の答えを置く。at は status_ms（t0 からの分）。
func (f *retryFixture) term(addr, state, msg string, at time.Duration, unsent bool) {
	f.t.Helper()
	labels := `{}`
	if unsent {
		labels = `{"first_message.sent":"false"}`
	}
	j := fmt.Sprintf(`{"state":"live","status":{"state":%q,"msg":%q},"status_ms":%d,"turn":1,"labels":%s,"argv":["claude"]}`,
		state, msg, f.t0.Add(at).UnixMilli(), labels)
	if err := os.WriteFile(filepath.Join(f.d, addr+".json"), []byte(j), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *retryFixture) screen(addr, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.d, addr+".screen"), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// at は t0 から m 分の見回り（watch と Chief of Staff の分）を1回して、その回に打たれたものを返す。
func (f *retryFixture) at(m time.Duration, cos string) []string {
	f.t.Helper()
	os.Remove(filepath.Join(f.d, "log"))
	now := f.t0.Add(m)
	f.p.watch(now, cos)
	if cos != "" {
		f.p.chiefCut(cos)
	}
	b, _ := os.ReadFile(filepath.Join(f.d, "log"))
	if s := strings.TrimSpace(string(b)); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

// sent は偽の hc の log（打ったもの）を返す。
func (f *retryFixture) sent() string {
	b, _ := os.ReadFile(filepath.Join(f.d, "log"))
	return strings.TrimSpace(string(b))
}

func (f *retryFixture) kinds() []string {
	var k []string
	for _, n := range f.p.st.Pending {
		k = append(k, n.Issue+":"+n.Kind)
	}
	return k
}

var resend = "send t42 " + RetryText

// 切られた 1〜4 回目は 1・2・4・8 分あとに続けて、5 回目に切られたら打たずに Chief of Staff へ error を1回だけ知らせる。
func TestRetryWorkerIntervals(t *testing.T) {
	f := newRetry(t)
	cut := time.Duration(0) // 切られた時刻（t0 からの分）
	for i, wait := range []time.Duration{1, 2, 4, 8} {
		f.term("t42", "error", cutMsg, cut*time.Minute, false)
		if got := f.at(cut*time.Minute, ""); got != nil { // 切られたのを見る（ここから測る）
			t.Fatalf("%d 回目: 見たその回に打った: %q", i+1, got)
		}
		if got := f.at((cut+wait)*time.Minute-time.Second, ""); got != nil {
			t.Fatalf("%d 回目: %v 分たたないうちに打った: %q", i+1, wait, got)
		}
		if got := f.at((cut+wait)*time.Minute, ""); len(got) != 1 || got[0] != resend {
			t.Fatalf("%d 回目: %v 分で打っていない: %q", i+1, wait, got)
		}
		// 打った直後の見回り（まだ同じ error のまま）では二度打たない
		if got := f.at((cut+wait)*time.Minute+5*time.Second, ""); got != nil {
			t.Fatalf("%d 回目: 打った直後にまた打った: %q", i+1, got)
		}
		// 続けて working になり、しばらくして また切られる
		f.term("t42", "working", "", (cut+wait)*time.Minute+10*time.Second, false)
		if got := f.at((cut+wait)*time.Minute+20*time.Second, ""); got != nil {
			t.Fatalf("%d 回目: working に打った: %q", i+1, got)
		}
		cut += wait + 3
	}
	if r := f.p.st.Retry["S-7|t42"]; r.N != 4 {
		t.Fatalf("4 回打ったはず: %+v", r)
	}
	if len(f.p.st.Pending) != 0 {
		t.Fatalf("打ち切る前に知らせた: %q", f.kinds())
	}
	// 5 回目に切られた: もう打たず、error を知らせる
	f.term("t42", "error", cutMsg, cut*time.Minute, false)
	if got := f.at(cut*time.Minute, ""); got != nil {
		t.Fatalf("5 回目に切られたのに打った: %q", got)
	}
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:error" {
		t.Fatalf("打ち切ったことを知らせていない: %q", k)
	}
	f.p.st.Pending = []Notification{}
	if got := f.at((cut+40)*time.Minute, ""); got != nil || len(f.p.st.Pending) != 0 {
		t.Fatalf("打ち切ったあとにまた打った・知らせた: %q %q", got, f.kinds())
	}
	// idle に戻れば区切る（次に切られたらまた 1 分から）
	f.term("t42", "idle", "", (cut+41)*time.Minute, false)
	f.at((cut+42)*time.Minute, "")
	if _, ok := f.p.st.Retry["S-7|t42"]; ok {
		t.Fatal("idle に戻ったのに回数が残っている")
	}
}

// プロバイダでない error・認証（blocked）・使用量の上限には打たない。
func TestRetryWorkerNotProvider(t *testing.T) {
	for _, c := range []struct{ state, msg, screen, kind string }{
		{"error", "Error: tool crashed", "", "error"},
		{"error", "You've hit your usage limit · resets 3pm", "", "error"},
		{"blocked", "Invalid API key · Fix external API key", "", "blocked"},
		{"error", "", "› hello\n■ unexpected status 401 Unauthorized: Missing bearer or basic authentication in header\n", "error"},
	} {
		f := newRetry(t)
		f.term("t42", c.state, c.msg, 0, false)
		f.screen("t42", c.screen)
		if got := f.at(30*time.Minute, ""); got != nil {
			t.Errorf("%s %q: 打った: %q", c.state, c.msg, got)
		}
		if k := f.kinds(); len(k) != 1 || k[0] != "S-7:"+c.kind {
			t.Errorf("%s %q: 知らせ方: %q", c.state, c.msg, k)
		}
	}
	// Codex は msg を出さないので画面で見分ける（実測の文）
	f := newRetry(t)
	f.term("t42", "error", "", 0, false)
	f.screen("t42", "› hello\n■ We’re currently experiencing high demand, which may cause temporary errors.\n")
	f.at(0, "")
	if got := f.at(time.Minute, ""); len(got) != 1 || got[0] != resend {
		t.Fatalf("Codex の切断に打っていない: %q", got)
	}
}

// 最初のメッセージの送り直しと重ならない: 届いていない担当には「続けて」を打たず（error で知らせる）、最初のメッセージも error の間は送らない。
// 届いた（控えの first にある）担当は、ラベルが残っていても続きを打つ。
func TestRetryNotWithFirstMessage(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "error", cutMsg, 0, true)
	for _, m := range []time.Duration{1, 3, 7, 15, 30} {
		if got := f.at(m*time.Minute, ""); got != nil {
			t.Fatalf("%v 分: 最初のメッセージが届いていない担当に打った: %q", m, got)
		}
	}
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:error" {
		t.Fatalf("知らせていない: %q", k)
	}
	if r := f.p.st.Retry["S-7|t42"]; r.N != 0 {
		t.Fatalf("打っていないのに回数を数えた: %+v", r)
	}
	// 送ったがラベルを外せなかった（控えの first）: 届いているので続きを打つ。最初のメッセージは送り直さない
	f.p.st.First["S-7|t42"] = true
	got := f.at(31*time.Minute, "")
	if len(got) != 2 || got[0] != "label t42 first_message.sent=" || got[1] != resend {
		t.Fatalf("控えに送ったとある担当: %q", got)
	}
}

// stalled とは重ならない: 打ったあとの working は 20 分までは静か、画面も msg も 20 分動かなければ stalled を知らせるが打たない。
func TestRetryThenStalled(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	if got := f.at(time.Minute, ""); len(got) != 1 {
		t.Fatalf("打っていない: %q", got)
	}
	f.term("t42", "working", "", time.Minute+time.Second, false)
	f.at(2*time.Minute, "") // working を見た（画面と msg を控える）
	if got := f.at(20*time.Minute, ""); got != nil || len(f.p.st.Pending) != 0 {
		t.Fatalf("20 分たたないのに: %q %q", got, f.kinds())
	}
	if got := f.at(22*time.Minute, ""); got != nil {
		t.Fatalf("stalled に打った: %q", got)
	}
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:stalled" {
		t.Fatalf("stalled を知らせていない: %q", k)
	}
}

// 1つのターンで長く働いている担当（working のまま。status.msg か画面が動いている）には stalled を届けない。
// 画面も msg も 20 分動かなくなったら届ける（A-0036）。
func TestWorkingMoving(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "working", "go test を回す", 0, false)
	for i, m := range []time.Duration{1, 10, 19, 28, 37, 46} {
		f.screen("t42", fmt.Sprintf("✻ Working… (%dm)\n%s", m, strings.Repeat("Edit x.go\n", i+1)))
		f.at(m*time.Minute, "")
	}
	if k := f.kinds(); len(k) != 0 {
		t.Fatalf("進んでいる担当に stalled を届けた: %q", k)
	}
	f.screen("t42", "✻ Working… (60m)\n"+strings.Repeat("Edit x.go\n", 6)) // 時計だけ進む
	f.at(60*time.Minute, "")
	if k := f.kinds(); len(k) != 0 {
		t.Fatalf("動いて 20 分たたないのに: %q", k)
	}
	f.at(66*time.Minute, "")
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:stalled" {
		t.Fatalf("画面も msg も 20 分動かないのに stalled を届けない: %q", k)
	}
}

// hc read が失敗し続けても、status.msg が替わっていれば進んでいるとみて stalled を届けない（A-0036 のレビュー 1 回目）。
func TestWorkingMovingMsgOnly(t *testing.T) {
	f := newRetry(t) // 画面のファイルを置かない: 偽の hc read は失敗する
	for i, m := range []time.Duration{1, 15, 30, 45} {
		f.term("t42", "working", fmt.Sprintf("ツール %c を回す", 'a'+i), 0, false)
		f.at(m*time.Minute, "")
	}
	if k := f.kinds(); len(k) != 0 {
		t.Fatalf("msg が替わっている担当に stalled を届けた: %q", k)
	}
	f.at(66*time.Minute, "")
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:stalled" {
		t.Fatalf("msg も画面も 20 分動かないのに stalled を届けない: %q", k)
	}
}

// Chief Engineer（profile chief-engineer）の端末の stalled・blocked は記録だけで、error は届ける（A-0036）。
func TestEngineerWatch(t *testing.T) {
	f := newRetry(t)
	f.p.l.Slug = "sample-site"
	f.p.tasks[0]["profile"] = "chief-engineer"
	f.term("t42", "idle", "", 0, false)
	f.at(30*time.Minute, "")
	f.term("t42", "blocked", "", 31*time.Minute, false)
	f.at(32*time.Minute, "")
	if k := f.kinds(); len(k) != 0 {
		t.Fatalf("Chief Engineer の止まりを届けた: %q", k)
	}
	f.term("t42", "error", "Error: tool crashed", 33*time.Minute, false)
	f.at(34*time.Minute, "")
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:error" {
		t.Fatalf("Chief Engineer の error を届けない: %q", k)
	}
	b, err := os.ReadFile(RecordedPath("sample-site"))
	if err != nil || strings.Count(string(b), "\n") != 2 || !strings.Contains(string(b), `"stalled"`) || !strings.Contains(string(b), `"blocked"`) {
		t.Fatalf("記録に stalled と blocked が無い: %q %v", b, err)
	}
}

// Chief of Staff には「続けて」を打たない（ADR-0019 決めたこと 4）: プロバイダに切られて止まったら、hc send も attach --observe も
// 呼ばず、同じ止まりにログを1行だけ残す（通知も積まない）。error・working の間は持ち越し、ほかに戻ったら区切る。
func TestChiefCut(t *testing.T) {
	f := newRetry(t)
	f.p.tasks = nil
	logged := func() int { return strings.Count(f.log.String(), "続きは打たない") }
	f.term("t1", "error", cutMsg, 0, false)
	for _, m := range []time.Duration{0, time.Minute, 30 * time.Minute, 2 * time.Hour} {
		for _, got := range f.at(m, "t1") {
			if strings.HasPrefix(got, "send ") || strings.HasPrefix(got, "attach ") {
				t.Fatalf("%v: Chief of Staff に打った・入力欄を読んだ: %q", m, got)
			}
		}
	}
	if n := logged(); n != 1 {
		t.Fatalf("切られた止まりのログが %d 行: %s", n, f.log.String())
	}
	if len(f.p.st.Pending) != 0 || len(f.p.st.Retry) != 0 {
		t.Fatalf("Chief of Staff の分を通知・回数に積んだ: %q %+v", f.kinds(), f.p.st.Retry)
	}
	// Claude Code 自身のやり直し（working）を挟んでまた切られても同じ止まり。担当の見回り（watch）を挟んでも控えは消えない
	f.term("t1", "working", "", 3*time.Hour, false)
	f.at(3*time.Hour, "t1")
	f.term("t1", "error", cutMsg, 4*time.Hour, false)
	f.at(4*time.Hour, "t1")
	if n := logged(); n != 1 {
		t.Fatalf("working を挟んだ同じ止まりで %d 行: %s", n, f.log.String())
	}
	// done に戻れば区切り、次に切られたらもう1行
	f.term("t1", "done", "", 5*time.Hour, false)
	f.at(5*time.Hour, "t1")
	f.term("t1", "error", cutMsg, 6*time.Hour, false)
	f.at(6*time.Hour, "t1")
	if n := logged(); n != 2 {
		t.Fatalf("区切ったあとの止まりで %d 行: %s", n, f.log.String())
	}
	// プロバイダでない error（認証）はログに書かない（人は Herdr App の error で気づく）
	f = newRetry(t)
	f.p.tasks = nil
	f.term("t1", "error", "API Error: 401 unauthorized · Please run /login", 0, false)
	if got := f.at(time.Hour, "t1"); got != nil || logged() != 0 {
		t.Fatalf("プロバイダでない error で打った・ログに書いた: %q %s", got, f.log.String())
	}
	// Chief of Staff の端末が doing の課題の agent でも、担当の見回り（watch）から「続けて」を打たない
	f = newRetry(t)
	f.p.tasks = []ledger.Issue{{"id": "S-8", "status": "doing", "agent": "t1"}}
	f.term("t1", "error", cutMsg, 0, false)
	for _, m := range []time.Duration{0, time.Minute, 3 * time.Minute, 7 * time.Minute, 15 * time.Minute, time.Hour} {
		if got := f.at(m, "t1"); got != nil {
			t.Fatalf("%v: 課題の agent になった Chief of Staff に打った: %q", m, got)
		}
	}
	if logged() != 1 || len(f.p.st.Pending) != 0 {
		t.Fatalf("課題の agent になった Chief of Staff のログ・通知: %s %q", f.log.String(), f.kinds())
	}
	// 初めて見る台帳では書かない
	f = newRetry(t)
	f.p.tasks, f.p.first = nil, true
	f.term("t1", "error", cutMsg, 0, false)
	if got := f.at(30*time.Minute, "t1"); got != nil || logged() != 0 {
		t.Fatalf("初めて見る台帳で打った・書いた: %q", got)
	}
}

// hc の時計（status_ms）が dispatcher の時計とずれていても、間は dispatcher が切断を見た時刻から測る。
// 打ってから status_ms が変われば、その値が打った時刻より前（端末の時計が遅れている）でも新しい切断とみる。
func TestRetryClockSkew(t *testing.T) {
	for _, skew := range []time.Duration{-30 * time.Minute, 30 * time.Minute} {
		f := newRetry(t)
		f.term("t42", "error", cutMsg, skew, false)
		if got := f.at(0, ""); got != nil {
			t.Fatalf("skew %v: 初めて見た切断にすぐ打った: %q", skew, got)
		}
		if got := f.at(59*time.Second, ""); got != nil {
			t.Fatalf("skew %v: 1 分たたないうちに打った: %q", skew, got)
		}
		if got := f.at(time.Minute, ""); len(got) != 1 {
			t.Fatalf("skew %v: 1 分で打っていない: %q", skew, got)
		}
		// 4 回打ったところから、また切られた（status_ms は打った時刻より前に見える値）
		r := f.p.st.Retry["S-7|t42"]
		r.N = 4
		f.p.st.Retry["S-7|t42"] = r
		f.term("t42", "error", cutMsg, skew-time.Hour, false)
		if got := f.at(time.Minute+5*time.Second, ""); got != nil {
			t.Fatalf("skew %v: 5 回目に切られたのに打った: %q", skew, got)
		}
		if k := f.kinds(); len(k) != 1 || k[0] != "S-7:error" {
			t.Fatalf("skew %v: 5 回目の切断をすぐ知らせない: %q", skew, k)
		}
	}
}

// 打つ前に回数と時刻を控えに保存する: 打ってから見回りの最後の保存の前に落ちても、再開後に同じ切断へ打ち直さない。
func TestRetrySavedBeforeSend(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	if got := f.at(time.Minute, ""); len(got) != 1 {
		t.Fatalf("打っていない: %q", got)
	}
	// 見回りの最後の保存はしないまま落ちた、とみなし、保存された控えから再開する
	st, _, err := LoadState("sample-site")
	if err != nil {
		t.Fatal(err)
	}
	if r := st.Retry["S-7|t42"]; r.N != 1 {
		t.Fatalf("打つ前に控えへ保存していない: %+v", r)
	}
	f.p.st = st
	if got := f.at(time.Minute+5*time.Second, ""); got != nil {
		t.Fatalf("再開後に同じ切断へ打ち直した: %q", got)
	}
	// 控えを書けなければ打たない
	f = newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	dir := filepath.Join(f.d, "state", "andon", "dispatcher")
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, nil, 0o644); err != nil { // 置き場をファイルで塞ぐ
		t.Fatal(err)
	}
	if got := f.at(time.Minute, ""); got != nil {
		t.Fatalf("控えを書けないのに打った: %q", got)
	}
	if r := f.p.st.Retry["S-7|t42"]; r.N != 0 {
		t.Fatalf("書けなかった試行を数えた: %+v", r)
	}
}

// 回数は error・working の間だけ持ち越す: 4 回打ったあと blocked（認証など）を通って working に戻り、また切られたら、1 分待ちからやり直す。
func TestRetryResetThroughBlocked(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	f.p.st.Retry["S-7|t42"] = Resend{N: 4, At: f.t0.UnixMilli(), Cut: f.t0.UnixMilli(), Since: f.t0.UnixMilli(), Sent: f.t0.UnixMilli()}
	f.term("t42", "blocked", "Invalid API key · Fix external API key", time.Minute, false)
	f.at(time.Minute, "")
	if r, ok := f.p.st.Retry["S-7|t42"]; ok {
		t.Fatalf("blocked を通ったのに回数が残っている: %+v", r)
	}
	f.term("t42", "working", "", 2*time.Minute, false)
	f.at(2*time.Minute, "")
	f.p.st.Pending = []Notification{}
	f.term("t42", "error", cutMsg, 5*time.Minute, false)
	if got := f.at(5*time.Minute, ""); got != nil || len(f.p.st.Pending) != 0 {
		t.Fatalf("戻ったあとの切断をすぐ打った・打ち切った: %q %q", got, f.kinds())
	}
	if got := f.at(6*time.Minute, ""); len(got) != 1 || got[0] != resend {
		t.Fatalf("戻ったあとの切断に 1 分で打っていない: %q", got)
	}
}
