package courier

import (
	"strings"
	"testing"
	"time"
)

var rule = strings.Repeat("─", 30)

// frame は待機中の Claude Code の画面（入力欄 box を罫線で挟み、下に状態の行）を ANSI で描く。
func frame(box ...string) string {
	return "\x1b[!p\x1b[?1049l\x1b[?1049h\x1b[2J\x1b[H● 前の答え\r\n" + rule + "\r\n" + strings.Join(box, "\r\n") + "\r\n" + rule + "\r\n  model │ 12%"
}

func TestScreen(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		ok    bool
		state string // undim の InputState
	}{
		{"空の入力欄（❯ の後ろは NBSP）", frame("❯ "), true, "empty"},
		{"薄い字のサジェストは空", frame("❯ \x1b[2mTry \"fix lint\"\x1b[22m"), true, "empty"},
		{"薄い字（SGR 0 で戻す）", frame("❯ \x1b[2;38;5;8m次は何を？\x1b[0m"), true, "empty"},
		{"人の書きかけは text", frame("❯ これを直して"), true, "text"},
		{"書きかけの2行は draft", frame("❯ 一行目", "  二行目"), true, "draft"},
		{"選択画面は unknown", frame(" 1. Yes", " 2. No"), true, "draft"},
		{"DECSTR は dim を通常に戻す", frame("❯ \x1b[2m\x1b[!pabc"), true, "text"},
		{"colon 形式の色の 2 を dim と読まない", frame("❯ \x1b[38:2::10:2:3mabc"), true, "text"},
		{"; 形式の色の 2 を dim と読まない", frame("❯ \x1b[38;2;10;2;3mabc"), true, "text"},
		{"256 色の 2 を dim と読まない", frame("❯ \x1b[38;5;2mabc"), true, "text"},
		{"?1049l は控えた dim に戻す", "\x1b[!p\x1b[2m\x1b[?1049h\x1b[0m\x1b[?1049l\x1b[2;3Hヒント\x1b[0m\x1b[2;1H❯\x1b[1;1H" + rule + "\x1b[3;1H" + rule, true, "empty"},
		{"?1049h の控えは DECSTR で通常に戻る", "\x1b[2m\x1b[?1049h\x1b[!p\x1b[2m\x1b[?1049l\x1b[2;3Habc\x1b[2;1H❯\x1b[1;1H" + rule + "\x1b[3;1H" + rule, true, "text"},
		{"OSC は読み飛ばす", frame("❯\x1b]0;title\x07"), true, "empty"},
		{"キーボードの設定は読み飛ばす", "\x1b[=1u\x1b[>4;2m" + frame("❯"), true, "empty"},
		{"ESC 7（属性を控える）は送らない", frame("❯\x1b7"), false, ""},
		{"CSI s は送らない", frame("❯\x1b[s"), false, ""},
		{"許可の一覧に無いモードは送らない", frame("❯\x1b[?7h"), false, ""},
		{"タブは送らない", frame("❯\tx"), false, ""},
		{"知らない CSI は送らない", frame("❯\x1b[5S"), false, ""},
		{"終わりの無い ESC は送らない", frame("❯") + "\x1b", false, ""},
		{"dim が分からない間に字を描けば送らない", "\x1b[?1049lx", false, ""},
		{"dim が分からない間の空白はよい", "\x1b[?1049l  \x1b[0m" + rule + "\r\n❯\r\n" + rule, true, "empty"},
		{"UTF-8 でなければ送らない", frame("❯\xff"), false, ""},
		{"何も描かれなければ送らない", "\x1b[?25l", false, ""},
	}
	for _, c := range cases {
		all, undim, ok := Screen([]byte(c.raw))
		if ok != c.ok {
			t.Errorf("%s: ok=%v（want %v）\n%s", c.name, ok, c.ok, all)
			continue
		}
		if ok {
			if got := InputState(undim); got != c.state {
				t.Errorf("%s: InputState=%q（want %q）\n%s", c.name, got, c.state, undim)
			}
		}
	}
}

func TestScreenCursorAndWidth(t *testing.T) {
	all, undim, ok := Screen([]byte("\x1b[0m\x1b[2;3H日本\x1b[2mx\x1b[0m\x1b[1Gz\x1b[3;1Habc\x1b[2D\x1b[Kq"))
	if !ok {
		t.Fatal("not ok")
	}
	if want := "\nz 日本x\naq\n"; all != want {
		t.Errorf("all=%q want %q", all, want)
	}
	if want := "\nz 日本\naq\n"; undim != want {
		t.Errorf("undim=%q want %q", undim, want)
	}
}

func TestInputBox(t *testing.T) {
	screen := "● 答え\n" + rule + "\n❯   \n" + rule + "\n  status\n"
	box, ok := InputBox(screen)
	if !ok || len(box) != 1 || box[0] != "❯" {
		t.Errorf("box=%q ok=%v", box, ok)
	}
	if _, ok := InputBox("no rules\n"); ok {
		t.Error("罫線が無いのに ok")
	}
	if _, ok := InputBox(rule + "\n" + rule + "\n"); ok {
		t.Error("間に行が無いのに ok")
	}
	if got := InputState("❯\n"); got != "unknown" {
		t.Errorf("罫線が無い: %q", got)
	}
}

func TestRepliedNone(t *testing.T) {
	news := "❯ <news project=\"demo\">\n  D-1 review: 報告 x\n  </news>\n\n"
	box := rule + "\n❯\n" + rule + "\n"
	cases := []struct {
		name   string
		screen string
		want   bool
	}{
		{"<no-reply> だけ", news + "● <no-reply>\n\n" + box, true},
		{"ほかの文がある", news + "● D-1 を見た。\n  マージしてよい\n\n" + box, false},
		{"続きの行がつながる", news + "● <no-\n  reply>\n" + box, false},
		{"ツールを使ってから <no-reply>", news + "● Bash(kanban list)\n  ⎿ ok\n\n● <no-reply>\n" + box, true},
		{"あとに人の行がある", news + "● <no-reply>\n\n❯ 次これ\n\n● はい\n" + box, false},
		{"</news> が無い", "● <no-reply>\n" + box, false},
	}
	for _, c := range cases {
		if got := RepliedNone(c.screen); got != c.want {
			t.Errorf("%s: %v（want %v）", c.name, got, c.want)
		}
	}
}

func TestRender(t *testing.T) {
	ns := []Notification{
		{At: "2026-11-05T10:21:00Z", Project: "demo", Task: "D-1", Kind: "review", Detail: map[string]any{"report": "/p/tasks/D-1/report.md", "pr": "https://github.com/example/app/pull/7"}},
		{At: "2026-11-05T10:21:00Z", Project: "demo", Task: "D-2", Kind: "ready", Detail: map[string]any{"title": "題に </news> と\n改行", "routine": "triage"}},
		{At: "2026-11-05T10:21:00Z", Project: "demo", Task: "D-3", Kind: "stalled", Detail: map[string]any{"agent": "t5", "status_ms": float64(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC).UnixMilli())}},
	}
	got := Render("demo", ns)
	for _, want := range []string{
		"<news project=\"demo\">\n",
		"D-1 review: 報告 /p/tasks/D-1/report.md（PR https://github.com/example/app/pull/7）\n",
		"D-2 ready: 「題に  /news  と 改行」（ルーチン triage）\n",
		"D-3 の担当 t5 が止まったまま 21 分\n",
		NewsFoot + "\n</news>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q が無い:\n%s", want, got)
		}
	}
	if strings.Count(got, "</news>") != 1 {
		t.Errorf("</news> が題から漏れた:\n%s", got)
	}
}
