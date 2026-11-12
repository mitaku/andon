package dispatcher

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestProviderError(t *testing.T) {
	for s, want := range map[string]bool{
		// 実測（Claude Code 2.1.319 の status.msg・Codex 0.166.0 の画面とセッションの記録。docs/dispatcher.md §2.4）
		"API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment.": true,
		"API Error: The response stopped arriving. The response above may be incomplete.":                    true,
		"■ We’re currently experiencing high demand, which may cause temporary errors.":                      true,
		"Selected model is at capacity. Please try a different model.":                                       true,
		"unexpected status 401 Unauthorized: Missing bearer or basic authentication in header":               false,
		"Login expired · Please run /login":                                                                  false,
		"Invalid API key · Fix external API key":                                                             false,
		// 前からの例
		`API Error: 529 {"type":"overloaded_error"}`:         true,
		"Rate limit reached. Try again later":                true,
		"API Error: 503 service unavailable":                 true,
		"connection reset by peer":                           true,
		"API Error: 401 unauthorized · Please run /login":    false, // 認証の失敗はプロバイダのエラーでない
		"API Error: 529 overloaded · authentication_error":   false,
		"Error: ENOENT: no such file or directory, open 'x'": false,
		// 使用量の上限は待っても直らない
		"You've hit your usage limit · resets 3pm": false,
		"Claude AI usage limit reached|1793999999": false,
		"": false,
	} {
		if got := ProviderError(s); got != want {
			t.Errorf("ProviderError(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestJudge(t *testing.T) {
	const now = int64(10_000_000_000)
	min := func(m int) int64 { return now - int64(m)*int64(time.Minute/time.Millisecond) }
	busy := func() string {
		return "› hello\n■ We’re currently experiencing high demand, which may cause temporary errors.\n› Ask Codex to do anything\n"
	}
	other := func() string { return "panic: x" }
	none := func() string { t.Fatal("画面を読まなくてよいのに読んだ"); return "" }
	cut := "API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment."
	// 切断の見分けは status_ms（端末の時計）、間は控えの Since・At（dispatcher の時計）で測る。
	// 端末の時計はずれていてよいので、status_ms はわざと dispatcher の時計と関係の無い値にする
	const c1, c2 = int64(7), int64(9)
	cases := []struct {
		name   string
		s      Seen
		screen func() string
		r      Resend
		want   Verdict
	}{
		{"端末が無い", Seen{Gone: true}, none, Resend{}, Verdict{Kind: "finished", Key: "gone"}},
		{"blocked", Seen{Status: "blocked", StatusMs: 5}, none, Resend{}, Verdict{Kind: "blocked", Key: "blocked|5"}},
		{"認証は blocked（Claude Code の実測）", Seen{Status: "blocked", Msg: "Invalid API key · Fix external API key", StatusMs: 5}, none, Resend{}, Verdict{Kind: "blocked", Key: "blocked|5"}},
		{"プロバイダでない error", Seen{Status: "error", StatusMs: 5}, other, Resend{}, Verdict{Kind: "error", Key: "error|5"}},
		{"msg がプロバイダでなければ画面は読まない", Seen{Status: "error", Msg: "Error: tool crashed", StatusMs: 5}, none, Resend{}, Verdict{Kind: "error", Key: "error|5"}},
		{"初めて見た切断はすぐには打たない（端末の時計が遅れていても）", Seen{Status: "error", Msg: cut, StatusMs: 1}, none, Resend{}, Verdict{}},
		{"msg で見分ける（Claude Code）・見て1分", Seen{Status: "error", Msg: cut, StatusMs: c1}, none, Resend{Cut: c1, Since: min(1)}, Verdict{Retry: true}},
		{"画面の ■ の行で見分ける（Codex）・見て1分", Seen{Status: "error", StatusMs: c1}, busy, Resend{Cut: c1, Since: min(1)}, Verdict{Retry: true}},
		{"1回目は見て1分たたない", Seen{Status: "error", Msg: cut, StatusMs: c1}, none, Resend{Cut: c1, Since: min(0)}, Verdict{}},
		{"控えと違う status_ms は新しい切断（見た時刻から測り直す）", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{Cut: c1, Since: min(30)}, Verdict{}},
		{"2回目・見て1分（2分待つ）", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{N: 1, At: min(5), Cut: c2, Since: min(1), Sent: c1}, Verdict{}},
		{"2回目・見て2分", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{N: 1, At: min(5), Cut: c2, Since: min(2), Sent: c1}, Verdict{Retry: true}},
		{"2回目・同じ error のまま打って2分", Seen{Status: "error", Msg: cut, StatusMs: c1}, none, Resend{N: 1, At: min(2), Cut: c1, Since: min(30), Sent: c1}, Verdict{Retry: true}},
		{"3回目は4分", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{N: 2, At: min(9), Cut: c2, Since: min(3), Sent: c1}, Verdict{}},
		{"3回目・4分たった", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{N: 2, At: min(9), Cut: c2, Since: min(4), Sent: c1}, Verdict{Retry: true}},
		{"4回目は8分", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{N: 3, At: min(15), Cut: c2, Since: min(7), Sent: c1}, Verdict{}},
		{"4回目・8分たった", Seen{Status: "error", Msg: cut, StatusMs: c2}, none, Resend{N: 3, At: min(15), Cut: c2, Since: min(8), Sent: c1}, Verdict{Retry: true}},
		{"5回目に切られたらすぐ知らせる（端末の時計が遅れていても）", Seen{Status: "error", Msg: cut, StatusMs: 1}, none, Resend{N: 4, At: min(3), Cut: c1, Since: min(10), Sent: c1}, Verdict{Kind: "error", Key: "retry-out"}},
		{"4回打った直後・まだ前の切断の error", Seen{Status: "error", Msg: cut, StatusMs: c1}, none, Resend{N: 4, At: min(1), Cut: c1, Since: min(10), Sent: c1}, Verdict{}},
		{"4回打っても8分同じ error のまま", Seen{Status: "error", Msg: cut, StatusMs: c1}, none, Resend{N: 4, At: min(8), Cut: c1, Since: min(20), Sent: c1}, Verdict{Kind: "error", Key: "retry-out"}},
		{"最初のメッセージが届いていなければ打たずに知らせる", Seen{Status: "error", Msg: cut, StatusMs: c1, Unsent: true}, none, Resend{Cut: c1, Since: min(5)}, Verdict{Kind: "error", Key: "error|" + itoa(c1)}},
		{"打ったあと working のまま（stalled の前）", Seen{Status: "working", StatusMs: min(3)}, none, Resend{N: 1, At: min(3)}, Verdict{}},
		{"idle 15分", Seen{Status: "idle", StatusMs: min(15)}, none, Resend{}, Verdict{Kind: "stalled", Key: "idle|" + itoa(min(15))}},
		{"done 14分", Seen{Status: "done", StatusMs: min(14)}, none, Resend{}, Verdict{}},
		{"idle 15分・レビュー待ち", Seen{Status: "idle", StatusMs: min(15), Reviewing: true}, none, Resend{}, Verdict{}},
		{"working 20分", Seen{Status: "working", StatusMs: min(20)}, none, Resend{}, Verdict{Kind: "stalled", Key: "working|" + itoa(min(20))}},
		{"working 19分", Seen{Status: "working", StatusMs: min(19)}, none, Resend{}, Verdict{}},
		{"working 30分・画面か msg が3分前に動いた（1ターンで長く働いている）", Seen{Status: "working", StatusMs: min(30), Moved: min(3)}, none, Resend{}, Verdict{}},
		{"working 30分・画面も msg も20分動かない", Seen{Status: "working", StatusMs: min(30), Moved: min(20)}, none, Resend{}, Verdict{Kind: "stalled", Key: "working|" + itoa(min(30))}},
		{"working 20分・進みを見たのは今（見回りの起点）", Seen{Status: "working", StatusMs: min(20), Moved: now}, none, Resend{}, Verdict{}},
	}
	for _, c := range cases {
		if got := Judge(c.s, c.screen, c.r, now); got != c.want {
			t.Errorf("%s: Judge = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// working の進み: status.msg か画面の字が替われば動いた時刻を今にする。経過の時計・回る印・数字だけの替わりは進みに数えない。
// 画面が読めなければ前の控えのまま（A-0036）。
func TestMoved(t *testing.T) {
	const now = int64(10_000_000_000)
	scr := func(s string) func() (string, error) { return func() (string, error) { return s, nil } }
	fail := func() (string, error) { return "", errors.New("hc read") }
	first := moved(Moved{}, false, "go test を回す", scr("✻ Thinking… (3m 2s · ↓ 1.2k tokens)\nRead x.go"), now-60_000)
	if first.At != now-60_000 || first.Screen == "" || first.Msg == "" {
		t.Fatalf("初めて見たら今を起点にする: %+v", first)
	}
	if m := moved(first, true, "go test を回す", scr("✶ Thinking… (24m 9s · ↓ 8.4k tokens)\nRead x.go"), now); m != first {
		t.Fatalf("時計・印・数字だけの替わりを進みに数えた: %+v", m)
	}
	if m := moved(first, true, "go test を回す", scr("✻ Thinking… (3m 2s · ↓ 1.2k tokens)\nRead x.go\nEdit y.go"), now); m.At != now {
		t.Fatalf("画面の替わりを進みに数えない: %+v", m)
	}
	if m := moved(first, true, "テストを読む", scr("✻ Thinking… (3m 2s · ↓ 1.2k tokens)\nRead x.go"), now); m.At != now {
		t.Fatalf("status.msg の替わりを進みに数えない: %+v", m)
	}
	if m := moved(first, true, "go test を回す", fail, now); m != first {
		t.Fatalf("画面が読めず msg も同じなのに控えを替えた: %+v", m)
	}
	// 画面が読めない間も msg の替わりは進みに数え、画面の要約は持ち越す
	m := moved(first, true, "テストを読む", fail, now)
	if m.At != now || m.Screen != first.Screen {
		t.Fatalf("画面が読めない回に msg の替わりを進みに数えない: %+v", m)
	}
	if m2 := moved(m, true, "テストを読む", fail, now+60_000); m2 != m {
		t.Fatalf("画面が読めず msg も同じなのに控えを替えた: %+v", m2)
	}
	if m := moved(Moved{}, false, "", fail, now); m.At != now {
		t.Fatalf("控えも画面も無いときは今を起点にする: %+v", m)
	}
}

// 画面で見るのは Codex のエラーの行（行頭が ■ の最後の行）だけ。エージェントの出力やファイルの中身にプロバイダの語があっても見ない。
func TestScreenCut(t *testing.T) {
	for screen, want := range map[string]bool{
		"› hello\n■ We’re currently experiencing high demand, which may cause temporary errors.\n› Ask Codex to do anything\n":     true,
		"• Ran curl …\n  └ HTTP/1.1 503 Service Unavailable\n    rate limit exceeded; high demand\n■ hook failed: exit status 2\n": false,
		"README.md: retries on 429 and 503 (high demand)\n› Ask Codex to do anything\n":                                            false,
		"■ Selected model is at capacity. Please try a different model.\n■ unexpected status 401 Unauthorized\n":                   false, // 最後の行が今の止まり
	} {
		if got := ProviderError(ScreenCut(screen)); got != want {
			t.Errorf("ProviderError(ScreenCut(%q)) = %v, want %v", screen, got, want)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
