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

const retryHarness = `#!/bin/sh
[ "$1" = --version ] || { echo "unexpected $(basename "$0") $*" >&2; exit 9; }
echo "$(basename "$0") 0.0.0-fake"
`

func newRetry(t *testing.T) *retryFixture {
	t.Helper()
	f := &retryFixture{t: t, d: t.TempDir(), t0: time.UnixMilli(1_800_000_000_000)}
	bin := filepath.Join(f.d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"hc": retryHc, "claude": retryHarness, "codex": retryHarness} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", f.d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.d, "state"))
	t.Setenv("ANDON_WORKSPACE", filepath.Join(f.d, "workspace"))
	t.Setenv("HOME", filepath.Join(f.d, "home"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	st := &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
	st.ensure()
	f.p = &project{c: &Dispatcher{Log: &f.log}, l: ledger.At(filepath.Join(f.d, "projects", "sample-site")), st: st,
		tasks: []ledger.Issue{{"id": "S-7", "status": "doing", "agent": "t42"}}}
	return f
}

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

var resend = "send t42 [dispatcher] " + RetryText

func TestRetryWorkerIntervals(t *testing.T) {
	f := newRetry(t)
	cut := time.Duration(0)
	for i, wait := range []time.Duration{1, 2, 4, 8} {
		f.term("t42", "error", cutMsg, cut*time.Minute, false)
		if got := f.at(cut*time.Minute, ""); got != nil {
			t.Fatalf("%d 回目: 見たその回に打った: %q", i+1, got)
		}
		if got := f.at((cut+wait)*time.Minute-time.Second, ""); got != nil {
			t.Fatalf("%d 回目: %v 分たたないうちに打った: %q", i+1, wait, got)
		}
		if got := f.at((cut+wait)*time.Minute, ""); len(got) != 1 || got[0] != resend {
			t.Fatalf("%d 回目: %v 分で打っていない: %q", i+1, wait, got)
		}
		if got := f.at((cut+wait)*time.Minute+5*time.Second, ""); got != nil {
			t.Fatalf("%d 回目: 打った直後にまた打った: %q", i+1, got)
		}
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
	f.term("t42", "idle", "", (cut+41)*time.Minute, false)
	f.at((cut+42)*time.Minute, "")
	if _, ok := f.p.st.Retry["S-7|t42"]; ok {
		t.Fatal("idle に戻ったのに回数が残っている")
	}
}

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
	// 応答の途中で接続が切れた文（実測。A-0096 の担当が約12時間止まった）
	g := newRetry(t)
	g.term("t42", "error", "API Error: Connection lost mid-response.", 0, false)
	g.at(0, "")
	if got := g.at(time.Minute, ""); len(got) != 1 || got[0] != resend {
		t.Fatalf("Connection lost mid-response に打っていない: %q", got)
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
	f.p.st.First["S-7|t42"] = true
	got := f.at(31*time.Minute, "")
	if len(got) != 2 || got[0] != "label t42 first_message.sent=" || got[1] != resend {
		t.Fatalf("控えに送ったとある担当: %q", got)
	}
}

func TestRetryThenStalled(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	if got := f.at(time.Minute, ""); len(got) != 1 {
		t.Fatalf("打っていない: %q", got)
	}
	f.term("t42", "working", "", time.Minute+time.Second, false)
	f.at(2*time.Minute, "")
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
	f.screen("t42", "✻ Working… (60m)\n"+strings.Repeat("Edit x.go\n", 6))
	f.at(60*time.Minute, "")
	if k := f.kinds(); len(k) != 0 {
		t.Fatalf("動いて 20 分たたないのに: %q", k)
	}
	f.at(66*time.Minute, "")
	if k := f.kinds(); len(k) != 1 || k[0] != "S-7:stalled" {
		t.Fatalf("画面も msg も 20 分動かないのに stalled を届けない: %q", k)
	}
}

func TestWorkingMovingMsgOnly(t *testing.T) {
	f := newRetry(t)
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
	f.term("t1", "working", "", 3*time.Hour, false)
	f.at(3*time.Hour, "t1")
	f.term("t1", "error", cutMsg, 4*time.Hour, false)
	f.at(4*time.Hour, "t1")
	if n := logged(); n != 1 {
		t.Fatalf("working を挟んだ同じ止まりで %d 行: %s", n, f.log.String())
	}
	f.term("t1", "done", "", 5*time.Hour, false)
	f.at(5*time.Hour, "t1")
	f.term("t1", "error", cutMsg, 6*time.Hour, false)
	f.at(6*time.Hour, "t1")
	if n := logged(); n != 2 {
		t.Fatalf("区切ったあとの止まりで %d 行: %s", n, f.log.String())
	}
	f = newRetry(t)
	f.p.tasks = nil
	f.term("t1", "error", "API Error: 401 unauthorized · Please run /login", 0, false)
	if got := f.at(time.Hour, "t1"); got != nil || logged() != 0 {
		t.Fatalf("プロバイダでない error で打った・ログに書いた: %q %s", got, f.log.String())
	}
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
	f = newRetry(t)
	f.p.tasks, f.p.first = nil, true
	f.term("t1", "error", cutMsg, 0, false)
	if got := f.at(30*time.Minute, "t1"); got != nil || logged() != 0 {
		t.Fatalf("初めて見る kanban で打った・書いた: %q", got)
	}
}

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

func TestRetrySavedBeforeSend(t *testing.T) {
	f := newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	if got := f.at(time.Minute, ""); len(got) != 1 {
		t.Fatalf("打っていない: %q", got)
	}
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
	f = newRetry(t)
	f.term("t42", "error", cutMsg, 0, false)
	f.at(0, "")
	dir := filepath.Join(f.d, "state", "andon", "dispatcher")
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := f.at(time.Minute, ""); got != nil {
		t.Fatalf("控えを書けないのに打った: %q", got)
	}
	if r := f.p.st.Retry["S-7|t42"]; r.N != 0 {
		t.Fatalf("書けなかった試行を数えた: %+v", r)
	}
}

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

func TestPassChiefCut(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	os.MkdirAll(bin, 0o755)
	hc := "#!/bin/sh\necho \"$*\" >> \"$FAKE_KANBAN/hc\"\n" +
		"[ \"$1 $2\" = 'get t6' ] && { printf '%s\\n' '{\"state\":\"live\",\"status\":{\"state\":\"error\",\"msg\":\"" + cutMsg + "\"},\"status_ms\":1,\"turn\":1,\"labels\":{}}'; exit 0; }\n" +
		"[ \"$1\" = get ] && { echo '{\"error\":{\"kind\":\"not_found\",\"message\":\"x\"}}' >&2; exit 3; }\nexit 0\n"
	for name, body := range map[string]string{"kanban": tickKanban, "hc": hc} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_KANBAN", d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	ws := filepath.Join(d, "ws")
	led := filepath.Join(ws, "projects", "sample-site", "kanban")
	os.MkdirAll(led, 0o755)
	os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t6\n"), 0o644)
	var logb bytes.Buffer
	c := &Dispatcher{WS: ws, Log: &logb}
	c.Pass("")
	c.Pass("")
	c.Pass("")
	if n := strings.Count(logb.String(), "t6 はプロバイダに切られて止まった。続きは打たない"); n != 1 {
		t.Errorf("切られた Chief of Staff のログが %d 行（1 行のはず）: %q", n, logb.String())
	}
	b, _ := os.ReadFile(filepath.Join(d, "hc"))
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "send ") || strings.HasPrefix(l, "attach ") || strings.HasPrefix(l, "keys ") {
			t.Errorf("切られた Chief of Staff に打った・入力欄を読んだ: %q", l)
		}
	}
}

func TestWatchNotifiesStopped(t *testing.T) {
	f := newRetry(t)
	f.p.l.Slug = "sample-site"
	f.p.tasks = []ledger.Issue{
		{"id": "S-1", "status": "doing", "agent": "t21"},
		{"id": "S-2", "status": "doing", "agent": "t22"},
		{"id": "S-3", "status": "doing", "agent": "t24"},
		{"id": "S-4", "status": "doing", "agent": "t25"},
		{"id": "S-5", "status": "doing", "agent": "t26"},
		{"id": "S-5-r1", "status": "todo", "parent": "S-5", "profile": "codex-review"},
		{"id": "S-6", "status": "doing", "agent": "t28"},
	}
	f.term("t22", "blocked", "", 15*time.Minute, false)
	f.term("t24", "error", "API Error: 401 unauthorized · Please run /login", 15*time.Minute, false)
	f.term("t25", "idle", "", 0, false)
	f.term("t26", "done", "", 0, false)
	f.term("t28", "idle", "", 15*time.Minute, false)
	ms := func(m time.Duration) int64 { return f.t0.Add(m).UnixMilli() }
	if got := f.at(16*time.Minute, ""); len(got) != 0 {
		t.Fatalf("止まった担当に打った: %q", got)
	}
	got := map[string]string{}
	for _, n := range f.p.st.Pending {
		got[n.Issue+":"+n.Kind] = asJSON(t, n.Detail)
	}
	want := map[string]string{
		"S-1:finished": `{"agent":"t21","status_ms":0}`,
		"S-2:blocked":  fmt.Sprintf(`{"agent":"t22","status_ms":%d}`, ms(15*time.Minute)),
		"S-3:error":    fmt.Sprintf(`{"agent":"t24","status_ms":%d}`, ms(15*time.Minute)),
		"S-4:stalled":  fmt.Sprintf(`{"agent":"t25","status_ms":%d}`, ms(0)),
	}
	if len(got) != len(want) {
		t.Fatalf("知らせたものが違う（レビュー待ち・短い idle を知らせた？）: %v", got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s の detail が %s（%s のはず）", k, got[k], w)
		}
	}
	if err := f.p.st.Save(f.p.l.Slug); err != nil {
		t.Fatalf("積んだ通知が書く前の照合を通らない: %v", err)
	}
	f.p.st.Pending = []Notification{}
	f.at(17*time.Minute, "")
	if k := f.kinds(); len(k) != 0 {
		t.Fatalf("同じ状態で二度知らせた: %q", k)
	}
	f.term("t22", "blocked", "", 17*time.Minute, false)
	f.at(18*time.Minute, "")
	if k := f.kinds(); len(k) != 1 || k[0] != "S-2:blocked" {
		t.Fatalf("新しい status_ms の blocked を知らせない: %q", k)
	}
}

// オーナーの課題（who: person）は、親の担当の課題のレビュアーではない。andon issue ask --new --parent <id> で作った
// オーナーの課題が todo の間も、親の担当が idle のまま 15 分たてば stalled を知らせる（ADR-0057 決めたこと 7）。
// レビュアーの子の課題（who が無い）は今までどおり stalled を止める。
func TestOwnerChildDoesNotHideStalled(t *testing.T) {
	for _, c := range []struct {
		name  string
		child ledger.Issue
		want  int
	}{
		{"オーナーの課題", ledger.Issue{"id": "S-8", "status": "todo", "parent": "S-7", "who": "person"}, 1},
		{"レビュアーの課題", ledger.Issue{"id": "S-7-r1", "status": "doing", "parent": "S-7", "profile": "codex-review"}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newRetry(t)
			f.p.tasks = append(f.p.tasks, c.child)
			f.term("t42", "idle", "", 0, false)
			f.at(16*time.Minute, "")
			if k := f.kinds(); len(k) != c.want || (c.want == 1 && k[0] != "S-7:stalled") {
				t.Fatalf("子の課題が %s のとき、stalled の通知 %q（want %d 件）", c.name, k, c.want)
			}
		})
	}
}
