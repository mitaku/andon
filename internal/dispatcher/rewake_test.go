package dispatcher

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func rewakeFixture(t *testing.T) *retryFixture {
	t.Helper()
	f := waitFixture(t)
	f.cos("error")
	old := rewakeUnit
	rewakeUnit = 20 * time.Millisecond
	t.Cleanup(func() { rewakeUnit = old })
	return f
}

func rewake(t *testing.T, input string) (int, string, time.Duration) {
	t.Helper()
	var stderr bytes.Buffer
	start := time.Now()
	code := RewakeMain(nil, strings.NewReader(input), &stderr)
	return code, strings.TrimSpace(stderr.String()), time.Since(start)
}

func rewakeLogText() string {
	b, _ := os.ReadFile(rewakePath("sample-site") + ".log")
	return string(b)
}

func TestRewakeBacksOffThenStops(t *testing.T) {
	for _, kind := range []string{"server_error", "rate_limit"} {
		f := rewakeFixture(t)
		f.pending(1)
		before, _ := os.ReadFile(StatePath("sample-site"))
		for i, units := range []int{1, 2, 4, 8} {
			code, msg, took := rewake(t, `{"hook_event_name":"StopFailure","error":"`+kind+`","last_assistant_message":"API Error: 529 Overloaded."}`)
			if code != 2 || msg != RewakeSignal {
				t.Fatalf("%s: %d 回目が合図で 2 にならない: %d %q", kind, i+1, code, msg)
			}
			if want := time.Duration(units) * rewakeUnit; took < want {
				t.Fatalf("%s: %d 回目の待ちが %v より短い: %v", kind, i+1, want, took)
			}
		}
		rewakeUnit = 300 * time.Millisecond
		code, msg, took := rewake(t, `{"error":"`+kind+`"}`)
		if code != 0 || msg != "" || took >= rewakeUnit {
			t.Fatalf("%s: 5 回目に起動した・待った: %d %q %v", kind, code, msg, took)
		}
		if n := strings.Count(rewakeLogText(), "\n"); n != 1 || !strings.Contains(rewakeLogText(), "t1 はプロバイダに切られて 5 回目（"+kind+"）。起動しない") {
			t.Fatalf("%s: 5 回目のログが1行でない: %q", kind, rewakeLogText())
		}
		if after, _ := os.ReadFile(StatePath("sample-site")); !bytes.Equal(before, after) {
			t.Fatalf("%s: 控えを書き替えた", kind)
		}
		if f.sent() != "" {
			t.Fatalf("%s: hc を打った: %q", kind, f.sent())
		}
		ResetRewake("sample-site")
		if code, _, _ := rewake(t, `{"error":"`+kind+`"}`); code != 2 {
			t.Fatalf("%s: 数え直した 1 回目が合図にならない: %d", kind, code)
		}
	}
}

func TestRewakeIgnores(t *testing.T) {
	f := rewakeFixture(t)
	rewakeUnit = 300 * time.Millisecond
	for _, in := range []string{`{"error":"authentication_failed"}`, `{"error":"billing_error"}`, `{"error":"invalid_request"}`, `{}`, `not json`, ``} {
		if code, msg, took := rewake(t, in); code != 0 || msg != "" || took >= rewakeUnit {
			t.Fatalf("%q で起動した・待った: %d %q %v", in, code, msg, took)
		}
	}
	t.Setenv("HC_TERMINAL_ID", "t42")
	f.term("t42", "error", "API Error: 529 Overloaded.", 0, false)
	if code, msg, took := rewake(t, `{"error":"server_error"}`); code != 0 || msg != "" || took >= rewakeUnit {
		t.Fatalf("担当の端末で起動した・待った: %d %q %v", code, msg, took)
	}
	if _, err := os.Stat(rewakePath("sample-site")); !os.IsNotExist(err) {
		t.Fatal("起動しない入力で回数を数えた")
	}
}

func TestRewakeSkipsWhenNoLongerStopped(t *testing.T) {
	old := rewakeSleep
	t.Cleanup(func() { rewakeSleep = old })
	for name, during := range map[string]func(f *retryFixture){
		"working":    func(f *retryFixture) { f.cos("working") },
		"done":       func(f *retryFixture) { f.cos("done") },
		"idle":       func(f *retryFixture) { f.cos("idle") },
		"読めない":       func(f *retryFixture) { os.WriteFile(f.d+"/t1.json", []byte("{"), 0o644) },
		"Stop で数え直し": func(f *retryFixture) { ResetRewake("sample-site") },
		"また切られた": func(f *retryFixture) {
			os.WriteFile(rewakePath("sample-site"), []byte(`{"count":2}`), 0o644)
		},
	} {
		f := rewakeFixture(t)
		var waited []time.Duration
		rewakeSleep = func(d time.Duration) { waited = append(waited, d); during(f) }
		code, msg, _ := rewake(t, `{"error":"server_error"}`)
		if len(waited) != 1 || waited[0] != rewakeUnit {
			t.Fatalf("%s: 1 単位の待ちに1回入っていない: %v", name, waited)
		}
		if code != 0 || msg != "" {
			t.Fatalf("%s: 止まったままでない Chief of Staff に合図を打った: %d %q", name, code, msg)
		}
	}
}

func TestRewakeAndWaitSignalTogetherDeliverOnce(t *testing.T) {
	f := rewakeFixture(t)
	f.pending(1)
	before, _ := os.ReadFile(StatePath("sample-site"))
	code, msg, _ := rewake(t, `{"error":"server_error"}`)
	f.cos("done")
	w := startWait("--timeout", "60")
	wcode, ok := w.done(t, 2*time.Second)
	if code != 2 || msg != RewakeSignal || !ok || wcode != 2 || strings.TrimSpace(w.stderr.String()) != WaitSignal {
		t.Fatalf("2つとも合図で 2 にならない: rewake %d %q / wait %v %d %q", code, msg, ok, wcode, w.stderr.String())
	}
	if after, _ := os.ReadFile(StatePath("sample-site")); !bytes.Equal(before, after) {
		t.Fatal("合図が控えを書き替えた（待ちを取り出した）")
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "")
	news := 0
	count := func(out any) {
		news += strings.Count(asContext(out), "S-1 report")
	}
	for _, in := range []struct {
		context bool
		h       hookInput
	}{
		{false, hookInput{SessionID: "s1", PromptID: "p1"}},
		{false, hookInput{SessionID: "s1", PromptID: "p1", StopHookActive: true}},
		{false, hookInput{SessionID: "s1", PromptID: "p2"}},
		{true, hookInput{SessionID: "s1", PromptID: "p3"}},
	} {
		out, err := drain("sample-site", in.context, in.h)
		if err != nil {
			t.Fatal(err)
		}
		count(out)
	}
	if news != 1 {
		t.Fatalf("<news> が %d 回届いた（1回のはず）", news)
	}
}
