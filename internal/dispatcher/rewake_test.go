package dispatcher

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// rewakeFixture は StopFailure のフックの試し: 待ち役と同じ偽の hc。Chief of Staff の端末 t1 は error で止まっている。待つ1単位は 20ms。
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

// server_error・rate_limit で切られた 1〜4 回目は 1・2・4・8 単位待って合図の1行で 2。5 回目は待たずに 0 とログ1行（回数は控え）。
// 待ち・控え（<slug>.json）は書かず、hc も打たない。
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
		code, msg, took := rewake(t, `{"error":"`+kind+`"}`)
		if code != 0 || msg != "" || took >= rewakeUnit {
			t.Fatalf("%s: 5 回目に起こした・待った: %d %q %v", kind, code, msg, took)
		}
		if n := strings.Count(rewakeLogText(), "\n"); n != 1 || !strings.Contains(rewakeLogText(), "t1 は提供元に切られて 5 回目（"+kind+"）。起こさない") {
			t.Fatalf("%s: 5 回目のログが1行でない: %q", kind, rewakeLogText())
		}
		if after, _ := os.ReadFile(StatePath("sample-site")); !bytes.Equal(before, after) {
			t.Fatalf("%s: 控えを書き替えた", kind)
		}
		if f.sent() != "" {
			t.Fatalf("%s: hc を打った: %q", kind, f.sent())
		}
		// ターンが通れば（Stop の drain が消す）数え直す
		ResetRewake("sample-site")
		if code, _, _ := rewake(t, `{"error":"`+kind+`"}`); code != 2 {
			t.Fatalf("%s: 数え直した 1 回目が合図にならない: %d", kind, code)
		}
	}
}

// ほかの error・読めない入力・Chief of Staff でない端末は、待たず数えずに 0。
func TestRewakeIgnores(t *testing.T) {
	f := rewakeFixture(t)
	for _, in := range []string{`{"error":"authentication_failed"}`, `{"error":"billing_error"}`, `{"error":"invalid_request"}`, `{}`, `not json`, ``} {
		if code, msg, took := rewake(t, in); code != 0 || msg != "" || took >= rewakeUnit {
			t.Fatalf("%q で起こした・待った: %d %q %v", in, code, msg, took)
		}
	}
	t.Setenv("HC_TERMINAL_ID", "t42") // 担当の端末（偽の hc にラベルが無い）
	f.term("t42", "error", "API Error: 529 Overloaded.", 0, false)
	if code, msg, took := rewake(t, `{"error":"server_error"}`); code != 0 || msg != "" || took >= rewakeUnit {
		t.Fatalf("担当の端末で起こした・待った: %d %q %v", code, msg, took)
	}
	if _, err := os.Stat(rewakePath("sample-site")); !os.IsNotExist(err) {
		t.Fatal("起こさない入力で回数を数えた")
	}
}

// 待つ間に人が打てば合図を打たずに 0: 動いている（working）・通って終わった（done・idle）・端末が読めない・
// 通ったターンの Stop で回数が消えた・また切られて次のフックが回数を進めた。
func TestRewakeSkipsWhenNoLongerStopped(t *testing.T) {
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
		rewakeUnit = 300 * time.Millisecond
		go func() { time.Sleep(50 * time.Millisecond); during(f) }()
		if code, msg, _ := rewake(t, `{"error":"server_error"}`); code != 0 || msg != "" {
			t.Fatalf("%s: 止まったままでない Chief of Staff に合図を打った: %d %q", name, code, msg)
		}
	}
}

// 待ち役（Stop）とこのフック（StopFailure）が同時に 2 で終わっても、どちらも合図だけなので <news> は1回しか届かない。
// 起きたターンの終わりの Stop の drain が1回渡し、もう1つの合図で起きたターンの Stop・次の人の送信では出ない。
func TestRewakeAndWaitSignalTogetherDeliverOnce(t *testing.T) {
	f := rewakeFixture(t)
	f.pending(1)
	before, _ := os.ReadFile(StatePath("sample-site"))
	code, msg, _ := rewake(t, `{"error":"server_error"}`) // error のままの Chief of Staff を起こす
	f.cos("done")                                         // 起きたターンが通った直後に、待ち役も合図する
	w := startWait("--timeout", "60")
	wcode, ok := w.done(t, 2*time.Second)
	if code != 2 || msg != RewakeSignal || !ok || wcode != 2 || strings.TrimSpace(w.stderr.String()) != WaitSignal {
		t.Fatalf("2つとも合図で 2 にならない: rewake %d %q / wait %v %d %q", code, msg, ok, wcode, w.stderr.String())
	}
	if after, _ := os.ReadFile(StatePath("sample-site")); !bytes.Equal(before, after) {
		t.Fatal("合図が控えを書き替えた（待ちを取り出した）")
	}
	// ここからは Claude Code のターンの終わり（フックの外の hc・andon は無い）
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
		{false, hookInput{SessionID: "s1", PromptID: "p1"}},                       // 1つ目の合図で起きたターンの終わり: <news> を渡す
		{false, hookInput{SessionID: "s1", PromptID: "p1", StopHookActive: true}}, // その続きの終わり: 確かめ済み
		{false, hookInput{SessionID: "s1", PromptID: "p2"}},                       // 2つ目の合図で起きたターンの終わり
		{true, hookInput{SessionID: "s1", PromptID: "p3"}},                        // 次の人の送信
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
