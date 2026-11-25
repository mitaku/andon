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
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	const slug = "demo"
	st, _, err := LoadState(slug)
	if err != nil {
		t.Fatal(err)
	}
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}}}
	if err := st.Save(slug); err != nil {
		t.Fatal(err)
	}
	out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p1"})
	if err != nil || out == nil {
		t.Fatalf("Stop で渡さない: %v %v", out, err)
	}
	unlock, err := lockState(slug, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p1", StopHookActive: true}); err == nil || out != nil {
		t.Fatalf("lock が取れないのに通さない: %v %v", out, err)
	}
	unlock()
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

func TestDrainStopUnderLockOtherTurnRetries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	defer func(w time.Duration) { lockWait = w }(lockWait)
	lockWait = 50 * time.Millisecond
	const slug = "demo"
	st, _, _ := LoadState(slug)
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}}}
	st.Save(slug)
	if out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p1"}); err != nil || out == nil {
		t.Fatalf("送信で渡さない: %v %v", out, err)
	}
	unlock, _ := lockState(slug, 0)
	drain(slug, false, hookInput{SessionID: "s1", PromptID: "p0"})
	unlock()
	out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"})
	if err != nil || out == nil || !strings.Contains(asContext(out), "A-0001 report") {
		t.Fatalf("終わりを見ていない <news> を届け直さない: %v %v", out, err)
	}
}

func TestDrainRetriesOnlyOnce(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	const slug = "demo"
	st, _, _ := LoadState(slug)
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}}}
	if err := st.Save(slug); err != nil {
		t.Fatal(err)
	}
	if out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p1"}); err != nil || !strings.Contains(asContext(out), "A-0001 report") {
		t.Fatalf("送信で渡さない: %v %v", out, err)
	}
	if out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"}); err != nil || !strings.Contains(asContext(out), "A-0001 report") {
		t.Fatalf("終わりを見ていない <news> を届け直さない: %v %v", out, err)
	}
	if st, _, _ := LoadState(slug); st.Hook == nil || len(st.Hook.Again) != 1 || len(st.Hook.News) != 0 {
		t.Fatalf("届け直した通知が Again の印になっていない: %+v", st.Hook)
	}
	out, err := drain(slug, true, hookInput{SessionID: "s1", PromptID: "p3"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asContext(out), "A-0001 report") {
		t.Fatalf("届け直した <news> を次の別の turn でもう一度渡した: %s", asContext(out))
	}
	if st, _, _ := LoadState(slug); st.Hook != nil || len(st.Pending) != 0 {
		t.Fatalf("届け直した通知が控えに残った: hook=%+v pending=%+v", st.Hook, st.Pending)
	}
}

func asContext(out any) string {
	m, _ := out.(map[string]any)
	h, _ := m["hookSpecificOutput"].(map[string]any)
	s, _ := h["additionalContext"].(string)
	return s
}

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
			{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}},
			{At: "2026-11-11T00:00:00Z", Project: slug, Kind: "error", Detail: map[string]any{"agent": "t1", "status_ms": 1}},
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
	pend()
	drain(slug, true, hookInput{SessionID: "s1", PromptID: "p1"})
	drain(slug, true, hookInput{SessionID: "s1", PromptID: "p2"})
	if l := lines(); len(l) != 0 {
		t.Fatalf("終わりを見ていない turn を書いた: %q", l)
	}
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

func TestLogDeliveredAcked(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	h := &HookNews{News: []Notification{{Issue: "A-0002", Kind: "ready"}}}
	logDelivered("demo", h, true)
	logDelivered("demo", h, true)
	logDelivered("demo", &HookNews{}, true)
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

// answered と hold を含むターンも、一緒に渡したほかの通知ごと届けた通知の記録に1行残る
// （記録の schema の kind の一覧が通知の一覧とずれると、EncodeLine が断って行ごと落ちる）。
func TestLogDeliveredAnsweredAndHold(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	h := &HookNews{News: []Notification{{Issue: "A-0001", Kind: "report"}, {Issue: "A-0002", Kind: "answered"}, {Issue: "A-0003", Kind: "hold"}}}
	logDelivered("demo", h, false)
	b, _ := os.ReadFile(DeliveredPath("demo"))
	var d Delivered
	if err := json.Unmarshal(b, &d); err != nil || len(d.News) != 3 || d.News[1] != (DeliveredNews{Issue: "A-0002", Kind: "answered"}) || d.News[2] != (DeliveredNews{Issue: "A-0003", Kind: "hold"}) {
		t.Fatalf("answered と hold を含むターンの記録が違う: %q %v", b, err)
	}
}

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
		st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}}}
		st.Save(slug)
		if out, err := drain(slug, false, hookInput{SessionID: "s1", PromptID: "p1"}); err != nil || out == nil {
			t.Fatalf("Stop で渡さない: %v %v", out, err)
		}
		os.WriteFile(filepath.Join(bin, "acked"), nil, 0o644)
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

// Claude Code が渡すフックの入力の形（2.1.330 で測ったもの。drain.go の sameTurn の注釈）を、Drain と同じ読み方で読んで照らす。
// prompt_id の照合が無いと、中断した turn のあとの別の turn の Stop を、渡した turn の終わりと見て確かめ済みにする。
func TestSameTurnClaudeCodeInputs(t *testing.T) {
	read := func(s string) hookInput {
		t.Helper()
		var in hookInput
		if err := json.Unmarshal([]byte(s), &in); err != nil {
			t.Fatal(err)
		}
		return in
	}
	from := func(via string, in hookInput) *HookNews {
		return &HookNews{Terminal: "t1", Via: via, SessionID: in.SessionID, PromptID: in.PromptID}
	}
	const (
		promptA = `{"session_id":"s1","prompt_id":"pa","hook_event_name":"UserPromptSubmit","prompt":"<hc>…</hc>"}`
		stopA   = `{"session_id":"s1","prompt_id":"pa","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"はい"}`
		contA   = `{"session_id":"s1","prompt_id":"pa","hook_event_name":"Stop","stop_hook_active":true,"last_assistant_message":"了解"}`
		promptB = `{"session_id":"s1","prompt_id":"pb","hook_event_name":"UserPromptSubmit","prompt":"<hc>…</hc>"}`
		stopB   = `{"session_id":"s1","prompt_id":"pb","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"次"}`
		wakeC   = `{"session_id":"s1","prompt_id":"pc","hook_event_name":"UserPromptSubmit","prompt":"<task-notification>…</task-notification>"}`
		stopC   = `{"session_id":"s1","prompt_id":"pc","hook_event_name":"Stop","stop_hook_active":true,"last_assistant_message":"起きた"}`
	)
	for _, c := range []struct {
		name string
		h    *HookNews
		stop string
		want bool
	}{
		{"UserPromptSubmit で渡した turn の Stop", from("prompt", read(promptA)), stopA, true},
		{"Stop で渡した続きの turn の Stop（同じ prompt_id）", from("stop", read(stopA)), contA, true},
		{"UserPromptSubmit で渡した turn を中断し、次の turn の Stop", from("prompt", read(promptA)), stopB, false},
		{"Stop で渡した続きの turn を中断し、次の turn の Stop", from("stop", read(stopA)), stopB, false},
		{"asyncRewake で始まった turn の Stop（stop_hook_active が true）", from("prompt", read(wakeC)), stopC, true},
		{"渡した turn の次の turn の UserPromptSubmit で渡し直した印と、その Stop", from("prompt", read(promptB)), stopB, true},
	} {
		if got := sameTurn(c.h, "t1", read(c.stop)); got != c.want {
			t.Errorf("%s: sameTurn = %v、want %v", c.name, got, c.want)
		}
	}
	noID := read(`{"session_id":"s1","hook_event_name":"Stop"}`)
	if sameTurn(from("prompt", read(promptA)), "t1", noID) {
		t.Error("prompt_id の無い Stop を、prompt_id のある turn の終わりと見た")
	}
	if sameTurn(from("prompt", read(`{"session_id":"s1","hook_event_name":"UserPromptSubmit"}`)), "t1", read(stopB)) {
		t.Error("prompt_id のある Stop を、prompt_id の無い turn の終わりと見た")
	}
	if !sameTurn(from("prompt", read(`{"session_id":"s1","hook_event_name":"UserPromptSubmit"}`)), "t1", noID) {
		t.Error("どちらにも prompt_id の無い Stop を、渡した turn の終わりと見ない")
	}
}

func TestDrainStopWithoutPromptIDRetriesOnce(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	const slug = "demo"
	st, _, _ := LoadState(slug)
	st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/X/report.md"}}}
	if err := st.Save(slug); err != nil {
		t.Fatal(err)
	}
	run := func(context bool, in string) string {
		t.Helper()
		args := []string{"--project", slug}
		if context {
			args = append(args, "--context")
		}
		var out, errb strings.Builder
		if code := Drain(args, strings.NewReader(in), &out, &errb); code != 0 {
			t.Fatalf("Drain が %d で終わった: %s", code, errb.String())
		}
		return out.String()
	}
	if out := run(true, `{"session_id":"s1","prompt_id":"pa","hook_event_name":"UserPromptSubmit"}`); !strings.Contains(out, "A-0001 report") {
		t.Fatalf("送信で渡さない: %s", out)
	}
	if out := run(false, `{"session_id":"s1","hook_event_name":"Stop","stop_hook_active":false}`); !strings.Contains(out, "A-0001 report") {
		t.Fatalf("prompt_id の無い Stop で、渡した <news> を確かめ済みにした: %q", out)
	}
	if out := run(false, `{"session_id":"s1","hook_event_name":"Stop","stop_hook_active":true}`); strings.Contains(out, "A-0001 report") {
		t.Fatalf("届け直した <news> をもう一度渡した: %s", out)
	}
	if st, _, _ := LoadState(slug); st.Hook != nil || len(st.Pending) != 0 {
		t.Fatalf("確かめた通知が控えに残った: hook=%+v pending=%+v", st.Hook, st.Pending)
	}
}

func TestDrainSecondAckInSameTurn(t *testing.T) {
	for _, ack2 := range []bool{true, false} {
		bin := t.TempDir()
		hc := "#!/bin/sh\n" +
			"if [ \"$1 $2 $3\" = 'get t1 --json' ]; then\n" +
			"  if [ -e \"$FAKE_DIR/acked\" ]; then ack=',\"news.ack\":\"5\"'; else ack=''; fi\n" +
			"  echo \"{\\\"turn\\\":5,\\\"labels\\\":{\\\"role\\\":\\\"chief-of-staff\\\",\\\"project\\\":\\\"demo\\\"$ack}}\"\n" +
			"  exit 0\n" +
			"fi\n" +
			"if [ \"$1 $2 $3\" = 'label t1 news.ack=' ]; then rm -f \"$FAKE_DIR/acked\"; exit 0; fi\n" +
			"if [ \"$1 $2\" = 'label t1' ]; then exit 0; fi\n" +
			"exit 1\n"
		if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(hc), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+":/usr/bin:/bin")
		t.Setenv("FAKE_DIR", bin)
		t.Setenv("HC_TERMINAL_ID", "t1")
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		const slug = "demo"
		push := func(issue string) {
			st, _, _ := LoadState(slug)
			st.Pending = []Notification{{At: "2026-11-11T00:00:00Z", Project: slug, Issue: issue, Kind: "report", Detail: map[string]any{"report": "/w/kanban/issues/" + issue + "/report.md"}}}
			st.Save(slug)
		}
		ack := func() { os.WriteFile(filepath.Join(bin, "acked"), nil, 0o644) }
		in := hookInput{SessionID: "s1", PromptID: "p1", StopHookActive: true}
		push("A-0001")
		if out, err := drain(slug, false, in); err != nil || out == nil {
			t.Fatalf("1回目を渡さない: %v %v", out, err)
		}
		ack()
		push("A-0002")
		if out, err := drain(slug, false, in); err != nil || out == nil {
			t.Fatalf("2回目を渡さない: %v %v", out, err)
		}
		if ack2 {
			ack()
		}
		drain(slug, false, in)
		b, _ := os.ReadFile(DeliveredPath(slug))
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) != 2 {
			t.Fatalf("ack2=%v: 記録が2行でない: %q", ack2, b)
		}
		var d1, d2 Delivered
		if json.Unmarshal([]byte(lines[0]), &d1) != nil || json.Unmarshal([]byte(lines[1]), &d2) != nil {
			t.Fatalf("記録が読めない: %q", b)
		}
		if !d1.Acked {
			t.Errorf("ack2=%v: 1回目が acked でない", ack2)
		}
		if d2.Acked != ack2 {
			t.Logf("%s", b)
			t.Errorf("ack2=%v: 2回目の acked = %v", ack2, d2.Acked)
		}
	}
}
