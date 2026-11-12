package dispatcher

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitFixture は待ち役の試し: 偽の hc（retryHc）と控えの置き場。Chief of Staff の端末は t1（ラベル role=chief-of-staff・project=sample-site）。
func waitFixture(t *testing.T) *retryFixture {
	t.Helper()
	f := newRetry(t)
	old := waitPoll
	waitPoll = 20 * time.Millisecond
	t.Cleanup(func() { waitPoll = old })
	t.Setenv("HC_TERMINAL_ID", "t1")
	f.cos("idle")
	return f
}

func (f *retryFixture) cos(state string) {
	f.t.Helper()
	j := fmt.Sprintf(`{"state":"live","status":{"state":%q},"turn":3,"labels":{"role":"chief-of-staff","project":"sample-site"}}`, state)
	if err := os.WriteFile(filepath.Join(f.d, "t1.json"), []byte(j), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *retryFixture) pending(n int) {
	f.t.Helper()
	f.p.st.Pending = nil
	for i := 0; i < n; i++ {
		f.p.st.Pending = append(f.p.st.Pending, Notification{At: "2026-11-11T00:00:00Z", Project: "sample-site", Issue: fmt.Sprintf("S-%d", i+1), Kind: "report", Detail: map[string]any{}})
	}
	if f.p.st.Pending == nil {
		f.p.st.Pending = []Notification{}
	}
	if err := f.p.st.Save("sample-site"); err != nil {
		f.t.Fatal(err)
	}
}

type waitRun struct {
	code   chan int
	stderr *bytes.Buffer
}

func startWait(args ...string) *waitRun {
	r := &waitRun{code: make(chan int, 1), stderr: &bytes.Buffer{}}
	go func() { r.code <- WaitMain(args, nil, r.stderr) }()
	return r
}

func (r *waitRun) done(t *testing.T, within time.Duration) (int, bool) {
	t.Helper()
	select {
	case c := <-r.code:
		return c, true
	case <-time.After(within):
		return 0, false
	}
}

// 届ける通知が待ちにあり、Chief of Staff が idle・done なら、合図の1行で 2。待ちと控えは変えない（取り出さない・書かない）。
func TestWaitSignals(t *testing.T) {
	for _, state := range []string{"idle", "done"} {
		f := waitFixture(t)
		f.cos(state)
		f.pending(2)
		before, _ := os.ReadFile(StatePath("sample-site"))
		r := startWait("--timeout", "60")
		code, ok := r.done(t, 2*time.Second)
		if !ok || code != 2 || strings.TrimSpace(r.stderr.String()) != WaitSignal {
			t.Fatalf("%s: 合図で終わらない: %v %d %q", state, ok, code, r.stderr.String())
		}
		after, _ := os.ReadFile(StatePath("sample-site"))
		if !bytes.Equal(before, after) {
			t.Fatalf("%s: 待ち役が控えを書き替えた", state)
		}
		if _, err := os.Stat(waitPath("sample-site")); !os.IsNotExist(err) {
			t.Fatalf("%s: 終わった待ち役の届けが残った", state)
		}
		if f.sent() != "" {
			t.Fatalf("%s: 待ち役が hc を打った: %q", state, f.sent())
		}
	}
}

// 待ちが無い間・Chief of Staff が working の間は終わらない。後から起きた待ち役に替わったら 0 で終わり、新しいほうが残る。
func TestWaitHoldsAndIsReplaced(t *testing.T) {
	f := waitFixture(t)
	f.pending(0)
	old := startWait("--timeout", "60")
	if _, ok := old.done(t, 200*time.Millisecond); ok {
		t.Fatal("待ちが無いのに終わった")
	}
	f.cos("working")
	f.pending(1)
	if _, ok := old.done(t, 200*time.Millisecond); ok {
		t.Fatal("working の間に終わった")
	}
	if !liveWaiter("sample-site", "t1") {
		t.Fatal("生きている待ち役を見分けない")
	}
	next := startWait("--timeout", "60")
	if code, ok := old.done(t, 2*time.Second); !ok || code != 0 || old.stderr.Len() != 0 {
		t.Fatalf("前の待ち役が静かに 0 で終わらない: %v %d %q", ok, code, old.stderr.String())
	}
	if _, ok := next.done(t, 200*time.Millisecond); ok {
		t.Fatal("新しい待ち役が終わった")
	}
	f.cos("done")
	if code, ok := next.done(t, 2*time.Second); !ok || code != 2 {
		t.Fatalf("新しい待ち役が合図しない: %v %d", ok, code)
	}
}

// timeout の 9 割まで打つ機会が無ければ、idle・done なら張り直しの合図で 2。working なら静かに 0（そのターンの Stop で次が起きる）。
func TestWaitRearms(t *testing.T) {
	f := waitFixture(t)
	f.pending(0)
	r := startWait("--timeout", "1")
	code, ok := r.done(t, 3*time.Second)
	if !ok || code != 2 || strings.TrimSpace(r.stderr.String()) != WaitRearm {
		t.Fatalf("張り直しの合図で終わらない: %v %d %q", ok, code, r.stderr.String())
	}
	f.cos("working")
	r = startWait("--timeout", "1")
	if code, ok := r.done(t, 3*time.Second); !ok || code != 0 || r.stderr.Len() != 0 {
		t.Fatalf("working の Chief of Staff に張り直しの合図を打った: %v %d %q", ok, code, r.stderr.String())
	}
}

// 張り直しの時刻に端末が一時に読めなくても待ち役を失わない: 読めるようになれば張り直す。95% まで読めなければ合図を打たずに 0。
func TestWaitRearmSurvivesHcError(t *testing.T) {
	f := waitFixture(t)
	old := waitUnit
	waitUnit = 50 * time.Millisecond // --timeout 60 → 3 秒。張り直し 2.7 秒、諦め 2.85 秒
	t.Cleanup(func() { waitUnit = old })
	f.pending(0)
	start := time.Now()
	r := startWait("--timeout", "60")
	time.Sleep(100 * time.Millisecond) // 起きた後に壊す（起きるときの読みは通す）
	broken := filepath.Join(f.d, "t1.json")
	if err := os.WriteFile(broken, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2750*time.Millisecond - time.Since(start)) // 張り直しの時刻 2.7 秒を過ぎるまで読めない
	if _, ok := r.done(t, time.Millisecond); ok {
		t.Fatal("張り直しの時刻に端末が読めないだけで待ち役が終わった")
	}
	f.cos("idle")
	code, ok := r.done(t, 2*time.Second)
	if !ok || code != 2 || strings.TrimSpace(r.stderr.String()) != WaitRearm {
		t.Fatalf("読めるようになっても張り直さない: %v %d %q", ok, code, r.stderr.String())
	}

	if err := os.WriteFile(broken, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = startWait("--timeout", "60") // 起きるときに読めなければ 0
	if code, ok := r.done(t, 10*time.Second); !ok || code != 0 {
		t.Fatalf("起きるときに端末が読めないのに待った: %v %d", ok, code)
	}
	f.cos("idle")
	start = time.Now()
	r = startWait("--timeout", "60")
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(broken, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, ok = r.done(t, 4*time.Second)
	if !ok || code != 0 || r.stderr.Len() != 0 {
		t.Fatalf("読めないまま諦めの時刻を過ぎても 0 で終わらない: %v %d %q", ok, code, r.stderr.String())
	}
	if d := time.Since(start); d < 2800*time.Millisecond || d >= 3*time.Second {
		t.Fatalf("諦めた時刻が 95%% の前後でない: %v", d)
	}
}

// Chief of Staff の端末でなければ待たずに 0。控えが読めなければ 0（待ちは①②で届く）。
func TestWaitPassesThrough(t *testing.T) {
	f := waitFixture(t)
	f.pending(1)
	t.Setenv("HC_TERMINAL_ID", "t42")
	if err := os.WriteFile(filepath.Join(f.d, "t42.json"), []byte(`{"state":"live","status":{"state":"idle"},"labels":{"project":"sample-site","issue":"S-7"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, ok := startWait().done(t, 10*time.Second); !ok || code != 0 {
		t.Fatalf("担当の端末で待った: %v %d", ok, code)
	}
	t.Setenv("HC_TERMINAL_ID", "t1")
	if err := os.WriteFile(StatePath("sample-site"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, ok := startWait().done(t, 10*time.Second); !ok || code != 0 {
		t.Fatalf("控えが読めないのに通さない: %v %d", ok, code)
	}
}

// 見回りは Chief of Staff の入力欄へ <news> を打たない（hc send も attach --observe も呼ばない）。
// 待ち役がいなければログに1行（同じ待ちには1回）、いればログも書かない。
func TestPatrolDoesNotSendToChief(t *testing.T) {
	f := waitFixture(t)
	f.pending(1)
	os.Remove(filepath.Join(f.d, "log"))
	f.p.unwaited("t1")
	f.p.unwaited("t1")
	if got := f.sent(); got != "" {
		t.Fatalf("見回りが Chief of Staff に打った: %q", got)
	}
	if n := strings.Count(f.log.String(), "待ち役がいない"); n != 1 {
		t.Fatalf("待ち役がいないことを1回だけログに書かない: %d %q", n, f.log.String())
	}
	f.cos("working")
	f.pending(0)
	r := startWait("--timeout", "60")
	for i := 0; i < 1000 && !liveWaiter("sample-site", "t1"); i++ { // 遅い CI でも待つ（上限 10 秒）
		time.Sleep(10 * time.Millisecond)
	}
	if !liveWaiter("sample-site", "t1") {
		t.Fatal("待ち役が起動しない")
	}
	f.cos("idle")
	f.pending(1)
	f.p.st.Pending[0].Issue = "S-9"
	f.log.Reset()
	f.p.unwaited("t1")
	if strings.Contains(f.log.String(), "待ち役がいない") {
		t.Fatalf("待ち役がいるのにいないと書いた: %q", f.log.String())
	}
	r.done(t, 10*time.Second)
}
