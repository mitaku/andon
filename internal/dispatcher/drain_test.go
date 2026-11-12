package dispatcher

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// 見回りが控えの lock を持っている間に、Stop で渡した <news> の続きのターンの Stop が来ても、
// 次の UserPromptSubmit で同じ <news> をもう一度渡さない（ADR-0019 決めたこと 7 の手がかり R8）。
func TestDrainStopUnderLockConfirms(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // hc も andon も無い（フックの外の確かめは落ちて通す）
	t.Setenv("HC_TERMINAL_ID", "")
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	const slug = "demo"
	st, _, err := LoadState(slug)
	if err != nil {
		t.Fatal(err)
	}
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report"}}
	if err := st.Save(slug); err != nil {
		t.Fatal(err)
	}
	// 1. Stop で渡す
	out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p1"})
	if err != nil || out == nil {
		t.Fatalf("Stop で渡さない: %v %v", out, err)
	}
	// 2. 見回りが lock を持っている間に、続きのターンの Stop が来る
	unlock, err := lockState(slug, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p1", StopHookActive: true}); err == nil || out != nil {
		t.Fatalf("lock が取れないのに通さない: %v %v", out, err)
	}
	unlock()
	// 3. 次の人の送信で、渡した <news> をもう一度添えない
	out, err = drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"})
	if err != nil {
		t.Fatal(err)
	}
	if out != nil && strings.Contains(asContext(out), "A-0001 report") {
		t.Fatalf("Stop で確かめた <news> を次の送信でもう一度渡した: %s", asContext(out))
	}
	if st, _, _ := LoadState(slug); st.Hook != nil {
		t.Fatalf("確かめた <news> の印が控えに残った: %+v", st.Hook)
	}
}

// 確かめの印が残っても、渡した turn と違う turn の Stop だったなら、終わりを見ていないので1回だけ届け直す（12b と同じ）。
func TestDrainStopUnderLockOtherTurnRetries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	const slug = "demo"
	st, _, _ := LoadState(slug)
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report"}}
	st.Save(slug)
	if out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p1"}); err != nil || out == nil {
		t.Fatalf("送信で渡さない: %v %v", out, err)
	}
	unlock, _ := lockState(slug, 0)
	drain(slug, false, hookInput{SessionID: "s1", PromptID: "p0"}) // 別の turn の Stop
	unlock()
	out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"})
	if err != nil || out == nil || !strings.Contains(asContext(out), "A-0001 report") {
		t.Fatalf("終わりを見ていない <news> を届け直さない: %v %v", out, err)
	}
}

func asContext(out any) string {
	m, _ := out.(map[string]any)
	h, _ := m["hookSpecificOutput"].(map[string]any)
	s, _ := h["additionalContext"].(string)
	return s
}

// 渡した turn の終わりを見たら、届けた通知の記録（<slug>.delivered.jsonl）に1行足す。lock の外で見た終わり（takeStop）も同じ。
// ack が無ければ acked は false（hc が無いのでラベルを読めない）。終わりを見ていない turn は書かない。
func TestDrainLogsDelivered(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	const slug = "demo"
	pend := func() {
		st, _, _ := LoadState(slug)
		st.Pending = []Notification{
			{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report"},
			{At: "2026-11-11T00:00:00Z", Project: slug, Kind: "error"},
		}
		st.Save(slug)
	}
	lines := func() []string {
		b, _ := os.ReadFile(DeliveredPath(slug))
		if s := strings.TrimSpace(string(b)); s != "" {
			return strings.Split(s, "\n")
		}
		return nil
	}
	// 1. 送信で渡し、別の turn が来た（終わりを見ていない）: 書かない
	pend()
	drain(slug, true, hookInput{SessionID: "s1", PromptID: "p1"})
	drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"})
	if l := lines(); len(l) != 0 {
		t.Fatalf("終わりを見ていない turn を書いた: %q", l)
	}
	// 2. Stop で渡し、続きの turn の Stop を lock の外で見て、次の drain で確かめ済みにする: 1行
	os.Remove(DeliveredPath(slug))
	st, _, _ := LoadState(slug)
	st.Hook = nil
	st.Save(slug)
	pend()
	drain(slug, false, hookInput{SessionID: "s1", PromptID: "p3"})
	unlock, _ := lockState(slug, 0)
	drain(slug, false, hookInput{SessionID: "s1", PromptID: "p3", StopHookActive: true})
	unlock()
	drain(slug, true, hookInput{SessionID: "s1", PromptID: "p4"})
	l := lines()
	if len(l) != 1 {
		t.Fatalf("届けた通知の記録が1行でない: %q", l)
	}
	var d Delivered
	if err := json.Unmarshal([]byte(l[0]), &d); err != nil {
		t.Fatal(err)
	}
	if d.Project != slug || d.Acked || len(d.News) != 2 || d.News[0] != (DeliveredNews{Issue: "A-0001", Kind: "report"}) || d.News[1] != (DeliveredNews{Kind: "error"}) {
		t.Fatalf("記録の中身が違う: %+v", d)
	}
}

// ack のあったターンは acked が true。
func TestLogDeliveredAcked(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	logDelivered("demo", &HookNews{News: []Notification{{Issue: "A-0002", Kind: "ready"}}}, true)
	logDelivered("demo", &HookNews{}, true) // 渡したものが無ければ書かない
	b, _ := os.ReadFile(DeliveredPath("demo"))
	var d Delivered
	if err := json.Unmarshal(b, &d); err != nil || !d.Acked || len(d.News) != 1 || d.News[0].Kind != "ready" || strings.Count(string(b), "\n") != 1 {
		t.Fatalf("ack のターンの記録が違う: %q %v", b, err)
	}
}
