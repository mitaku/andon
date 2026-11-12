package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// ack のあったターンは acked が true。同じ秒に同じ通知を渡した別のターンも、それぞれ1行（内容が同じでも飛ばさない）。
func TestLogDeliveredAcked(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	h := &HookNews{News: []Notification{{Issue: "A-0002", Kind: "ready"}}}
	logDelivered("demo", h, true)
	logDelivered("demo", h, true)
	logDelivered("demo", &HookNews{}, true) // 渡したものが無ければ書かない
	b, _ := os.ReadFile(DeliveredPath("demo"))
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("同じ内容の2つのターンが2行でない: %q", b)
	}
	var d Delivered
	if err := json.Unmarshal([]byte(lines[1]), &d); err != nil || !d.Acked || len(d.News) != 1 || d.News[0].Kind != "ready" {
		t.Fatalf("ack のターンの記録が違う: %q %v", b, err)
	}
}

// news.ack が渡したときの turn と同じなら、news.replied のラベルを書けなくても（hc label が失敗）acked は true。
// Stop の中で見た終わりと、lock の外で見た終わり（noteStop → takeStop）の両方。news.replied の数（控えの replied）は数えない。
func TestDrainAckedEvenIfLabelFails(t *testing.T) {
	bin := t.TempDir()
	hc := "#!/bin/sh\n" +
		"if [ \"$1 $2 $3\" = 'get t1 --json' ]; then\n" +
		"  if [ -e \"$FAKE_DIR/acked\" ]; then ack=',\"news.ack\":\"5\"'; else ack=''; fi\n" +
		"  echo \"{\\\"turn\\\":5,\\\"labels\\\":{\\\"role\\\":\\\"chief-of-staff\\\",\\\"project\\\":\\\"demo\\\"$ack}}\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(hc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_DIR", bin)
	t.Setenv("HC_TERMINAL_ID", "t1")
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	const slug = "demo"
	for _, underLock := range []bool{false, true} {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		os.Remove(filepath.Join(bin, "acked"))
		st, _, _ := LoadState(slug)
		st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report"}}
		st.Save(slug)
		if out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p1"}); err != nil || out == nil {
			t.Fatalf("Stop で渡さない: %v %v", out, err)
		}
		os.WriteFile(filepath.Join(bin, "acked"), nil, 0o644) // そのターンで andon news ack を打った
		end := hookInput{SessionID: "s1", PromptID: "p1", StopHookActive: true}
		if underLock {
			unlock, _ := lockState(slug, 0)
			drain(slug, false, end)
			unlock()
			drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"})
		} else {
			drain(slug, false, end)
		}
		b, _ := os.ReadFile(DeliveredPath(slug))
		var d Delivered
		if err := json.Unmarshal(b, &d); err != nil || !d.Acked || strings.Count(string(b), "\n") != 1 {
			t.Fatalf("lock の外=%v: ラベルを書けなかったターンが acked でない: %q %v", underLock, b, err)
		}
		if st, _, _ := LoadState(slug); st.Replied["ack"] != 0 {
			t.Errorf("lock の外=%v: news.replied を書けないのに数えた: %v", underLock, st.Replied)
		}
	}
}
