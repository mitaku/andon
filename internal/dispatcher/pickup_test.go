package dispatcher

import (
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
		"API Error: 401 unauthorized · Please run /login":    false, // 認証の失敗は提供元のエラーでない
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
		return "■ We’re currently experiencing high demand, which may cause temporary errors."
	}
	other := func() string { return "panic: x" }
	none := func() string { t.Fatal("画面を読まなくてよいのに読んだ"); return "" }
	cut := "API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment."
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
		{"提供元でない error", Seen{Status: "error", StatusMs: 5}, other, Resend{}, Verdict{Kind: "error", Key: "error|5"}},
		{"msg が提供元でなければ画面は読まない", Seen{Status: "error", Msg: "Error: tool crashed", StatusMs: 5}, none, Resend{}, Verdict{Kind: "error", Key: "error|5"}},
		{"msg で見分ける（Claude Code）・1分たった", Seen{Status: "error", Msg: cut, StatusMs: min(1)}, none, Resend{}, Verdict{Retry: true}},
		{"画面で見分ける（Codex）・1分たった", Seen{Status: "error", StatusMs: min(1)}, busy, Resend{}, Verdict{Retry: true}},
		{"1回目は1分たたない", Seen{Status: "error", Msg: cut, StatusMs: min(0)}, none, Resend{}, Verdict{}},
		// 2回目からは、切られた時刻（status_ms）と前に打った時刻の遅いほうから測る
		{"2回目・切られて1分（2分待つ）", Seen{Status: "error", Msg: cut, StatusMs: min(1)}, none, Resend{N: 1, At: min(5)}, Verdict{}},
		{"2回目・切られて2分", Seen{Status: "error", Msg: cut, StatusMs: min(2)}, none, Resend{N: 1, At: min(5)}, Verdict{Retry: true}},
		{"2回目・同じ error のまま打って2分", Seen{Status: "error", Msg: cut, StatusMs: min(30)}, none, Resend{N: 1, At: min(2)}, Verdict{Retry: true}},
		{"3回目は4分", Seen{Status: "error", Msg: cut, StatusMs: min(3)}, none, Resend{N: 2, At: min(9)}, Verdict{}},
		{"3回目・4分たった", Seen{Status: "error", Msg: cut, StatusMs: min(4)}, none, Resend{N: 2, At: min(9)}, Verdict{Retry: true}},
		{"4回目は8分", Seen{Status: "error", Msg: cut, StatusMs: min(7)}, none, Resend{N: 3, At: min(15)}, Verdict{}},
		{"4回目・8分たった", Seen{Status: "error", Msg: cut, StatusMs: min(8)}, none, Resend{N: 3, At: min(15)}, Verdict{Retry: true}},
		{"5回目に切られたら打たずに知らせる", Seen{Status: "error", Msg: cut, StatusMs: min(60)}, none, Resend{N: 4, At: min(70)}, Verdict{Kind: "error", Key: "retry-out"}},
		{"4回打った直後・まだ前の切断の error", Seen{Status: "error", Msg: cut, StatusMs: min(20)}, none, Resend{N: 4, At: min(1)}, Verdict{}},
		{"4回打っても8分同じ error のまま", Seen{Status: "error", Msg: cut, StatusMs: min(20)}, none, Resend{N: 4, At: min(8)}, Verdict{Kind: "error", Key: "retry-out"}},
		{"最初のメッセージが届いていなければ打たずに知らせる", Seen{Status: "error", Msg: cut, StatusMs: min(5), Unsent: true}, none, Resend{}, Verdict{Kind: "error", Key: "error|" + itoa(min(5))}},
		{"打ったあと working のまま（stalled の前）", Seen{Status: "working", StatusMs: min(3)}, none, Resend{N: 1, At: min(3)}, Verdict{}},
		{"idle 15分", Seen{Status: "idle", StatusMs: min(15)}, none, Resend{}, Verdict{Kind: "stalled", Key: "idle|" + itoa(min(15))}},
		{"done 14分", Seen{Status: "done", StatusMs: min(14)}, none, Resend{}, Verdict{}},
		{"idle 15分・レビュー待ち", Seen{Status: "idle", StatusMs: min(15), Reviewing: true}, none, Resend{}, Verdict{}},
		{"working 20分", Seen{Status: "working", StatusMs: min(20)}, none, Resend{}, Verdict{Kind: "stalled", Key: "working|" + itoa(min(20))}},
		{"working 19分", Seen{Status: "working", StatusMs: min(19)}, none, Resend{}, Verdict{}},
	}
	for _, c := range cases {
		if got := Judge(c.s, c.screen, c.r, now); got != c.want {
			t.Errorf("%s: Judge = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
