package courier

import (
	"strconv"
	"testing"
	"time"
)

func TestProviderError(t *testing.T) {
	for s, want := range map[string]bool{
		`API Error: 529 {"type":"overloaded_error"}`:         true,
		"Rate limit reached. Try again later":                true,
		"API Error: 503 service unavailable":                 true,
		"connection reset by peer":                           true,
		"API Error: 401 unauthorized · Please run /login":    false, // 認証の失敗は提供元のエラーでない
		"API Error: 529 overloaded · authentication_error":   false,
		"Error: ENOENT: no such file or directory, open 'x'": false,
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
	busy := func() string { return "API Error: 529 overloaded" }
	other := func() string { return "panic: x" }
	none := func() string { t.Fatal("error でないのに画面を読んだ"); return "" }
	cases := []struct {
		name   string
		s      Seen
		screen func() string
		r      Resend
		want   Verdict
	}{
		{"端末が無い", Seen{Gone: true}, none, Resend{}, Verdict{Kind: "finished", Key: "gone"}},
		{"blocked", Seen{Status: "blocked", StatusMs: 5}, none, Resend{}, Verdict{Kind: "blocked", Key: "blocked|5"}},
		{"提供元でない error", Seen{Status: "error", StatusMs: 5}, other, Resend{}, Verdict{Kind: "error", Key: "error|5"}},
		{"提供元の error・1分たった", Seen{Status: "error", StatusMs: min(1)}, busy, Resend{}, Verdict{Retry: true}},
		{"提供元の error・1分たたない", Seen{Status: "error", StatusMs: min(0)}, busy, Resend{}, Verdict{}},
		{"2回目は前に打ってから5分", Seen{Status: "error", StatusMs: min(30)}, busy, Resend{N: 1, At: min(4)}, Verdict{}},
		{"2回目・5分たった", Seen{Status: "error", StatusMs: min(30)}, busy, Resend{N: 1, At: min(5)}, Verdict{Retry: true}},
		{"3回目・15分たった", Seen{Status: "error", StatusMs: min(30)}, busy, Resend{N: 2, At: min(15)}, Verdict{Retry: true}},
		{"3回打っても戻らない", Seen{Status: "error", StatusMs: min(30)}, busy, Resend{N: 3, At: min(15)}, Verdict{Kind: "error", Key: "retry-out"}},
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
