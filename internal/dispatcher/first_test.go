package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
	"andon/internal/spawn"
)

const firstHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get)
    labels='{}'; [ -e "$d/unsent" ] && labels='{"first_message.sent":"false"}'
    fg=null; [ -e "$d/fg" ] && fg=$(cat "$d/fg")
    argv='["sh","-c","claude"]'; [ -e "$d/argv" ] && argv=$(cat "$d/argv")
    printf '{"state":"live","status":{"state":"%s"},"status_ms":%s,"turn":0,"labels":%s,"argv":%s,"foreground":%s}\n' "$(cat "$d/status" 2>/dev/null || echo idle)" "$(date +%s%3N)" "$labels" "$argv" "$fg" ;;
  send) printf 'send %s %s\n' "$2" "$(printf '%s' "$3" | tr '\n' '|')" >>"$d/log"
    [ -e "$d/send-rc" ] && exit "$(cat "$d/send-rc")"; exit 0 ;;
  read) [ -e "$d/read-rc" ] && exit "$(cat "$d/read-rc")"
    if [ -e "$d/screen" ]; then cat "$d/screen"; else printf '────\n❯ \n────\n'; fi ;;
  keys) printf 'keys %s\n' "${*:2}" >>"$d/log"
    [ -e "$d/send-rc" ] && exit "$(cat "$d/send-rc")"; exit 0 ;;
  label) printf 'label %s\n' "${*:2}" >>"$d/log"
    [ -e "$d/label-rc" ] && exit "$(cat "$d/label-rc")"; rm -f "$d/unsent" ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

type firstFixture struct {
	t   *testing.T
	d   string
	p   *project
	log bytes.Buffer
}

func newFirst(t *testing.T) *firstFixture {
	t.Helper()
	f := &firstFixture{t: t, d: t.TempDir()}
	bin := filepath.Join(f.d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(firstHc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", f.d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.d, "state"))
	t.Setenv("HOME", filepath.Join(f.d, "home"))
	t.Setenv("ANDON_WORKSPACE", filepath.Join(f.d, "ws"))
	st := &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
	st.ensure()
	f.p = &project{c: &Dispatcher{Log: &f.log}, l: ledger.At(filepath.Join(f.d, "projects", "sample-site")), st: st,
		tasks: []ledger.Issue{{"id": "S-7", "status": "doing", "agent": "t42"}}}
	f.put("unsent", "")
	return f
}

func (f *firstFixture) put(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.d, name), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *firstFixture) template(name, body string) {
	f.t.Helper()
	dir := filepath.Join(f.d, "home", ".config", "herdr-core", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *firstFixture) rm(name string) { os.Remove(filepath.Join(f.d, name)) }

func (f *firstFixture) round() []string {
	f.t.Helper()
	f.rm("log")
	f.p.watch(time.Now(), "")
	b, _ := os.ReadFile(filepath.Join(f.d, "log"))
	if s := strings.TrimSpace(string(b)); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

func TestFirstMessageResent(t *testing.T) {
	f := newFirst(t)
	brief := filepath.Join(f.d, "projects", "sample-site", "kanban", "issues", "S-7", "brief.md")
	send := "send t42 [dispatcher] 課題 `sample-site/S-7`。|依頼書は `" + brief + "`。"

	f.put("status", "blocked")
	if got := f.round(); got != nil {
		t.Fatalf("blocked なのに打った: %q", got)
	}
	f.put("status", "idle")
	f.put("send-rc", "1")
	if got := f.round(); len(got) != 1 || got[0] != send {
		t.Fatalf("1回目: %q", got)
	}
	if !strings.Contains(f.log.String(), "最初のメッセージを送れない（次の見回りで送り直す）") {
		t.Errorf("送れなかったことがログに無い: %s", f.log.String())
	}
	f.rm("send-rc")
	if got := f.round(); len(got) != 2 || got[0] != send || got[1] != "label t42 first_message.sent=" {
		t.Fatalf("2回目: %q", got)
	}
	if !strings.Contains(f.log.String(), "S-7 の担当 t42 に最初のメッセージを送った") {
		t.Errorf("送ったことがログに無い: %s", f.log.String())
	}
	if got := f.round(); got != nil {
		t.Fatalf("送ったあとにまた打った: %q", got)
	}
}

func TestFirstMessageResentWithBody(t *testing.T) {
	f := newFirst(t)
	f.template("claude", "---\nname: claude\ndescription: d\ncommand: claude\n---\n役目の1文目。\n役目の2文目。\n")
	f.p.tasks[0]["profile"] = "claude"
	brief := filepath.Join(f.d, "projects", "sample-site", "kanban", "issues", "S-7", "brief.md")
	want := "send t42 [dispatcher] 役目の1文目。|役目の2文目。||課題 `sample-site/S-7`。|依頼書は `" + brief + "`。"
	if got := f.round(); len(got) != 2 || got[0] != want || got[1] != "label t42 first_message.sent=" {
		t.Fatalf("本文つきで送っていない: %q", got)
	}

	f = newFirst(t)
	f.template("claude", "---\nname: claude\ndescription: d\ncommand: claude\n---\n役目の1文目。\n役目の2文目。\n")
	f.p.tasks[0]["profile"] = "claude"
	f.put("screen", "\n────\n❯ 役目の1文目。\n  役目の2文目。\n\n  課題 `sample-site/S-7`。\n────\n")
	if got := f.round(); len(got) != 2 || got[0] != "keys t42 enter" {
		t.Fatalf("残っている入力欄: %q", got)
	}
}

func TestFirstMessageNotTwice(t *testing.T) {
	f := newFirst(t)
	f.put("label-rc", "1")
	if got := f.round(); len(got) != 2 || !strings.HasPrefix(got[0], "send t42 ") {
		t.Fatalf("1回目: %q", got)
	}
	if !f.p.st.First["S-7|t42"] {
		t.Fatal("ラベルを外せなかったのに控えに送ったと持っていない")
	}
	if got := f.round(); len(got) != 1 || got[0] != "label t42 first_message.sent=" {
		t.Fatalf("2回目はラベルを外し直すだけのはず: %q", got)
	}
	f.rm("label-rc")
	if got := f.round(); len(got) != 1 || got[0] != "label t42 first_message.sent=" {
		t.Fatalf("3回目: %q", got)
	}
	if f.p.st.First["S-7|t42"] {
		t.Error("ラベルを外せたのに控えに残っている")
	}
	if got := f.round(); got != nil {
		t.Fatalf("片付いたあとに打った: %q", got)
	}
}

func TestFirstMessageSentByHand(t *testing.T) {
	f := newFirst(t)
	f.put("status", "working")
	if got := f.round(); len(got) != 1 || got[0] != "label t42 first_message.sent=" {
		t.Fatalf("動いている担当: %q", got)
	}
	f = newFirst(t)
	f.p.first = true
	if got := f.round(); got != nil {
		t.Fatalf("初めて見る kanban で打った: %q", got)
	}
	f = newFirst(t)
	f.rm("unsent")
	if got := f.round(); got != nil {
		t.Fatalf("ラベルの無い担当に打った: %q", got)
	}
}

func TestFirstMessageCrashBeforeUnlabel(t *testing.T) {
	f := newFirst(t)
	f.put("label-rc", "1")
	f.round()
	b, err := os.ReadFile(StatePath("sample-site"))
	if err != nil || !strings.Contains(string(b), `"S-7|t42": true`) {
		t.Fatalf("送る前に控えへ送りかけを保存していない: %v %s", err, b)
	}
	st, _, err := LoadState("sample-site")
	if err != nil {
		t.Fatal(err)
	}
	f.p.st = st
	f.rm("label-rc")
	if got := f.round(); len(got) != 1 || got[0] != "label t42 first_message.sent=" {
		t.Fatalf("再開後に送り直した: %q", got)
	}
}

func TestFirstMessageSendFailsThenCrash(t *testing.T) {
	f := newFirst(t)
	f.put("send-rc", "1")
	if got := f.round(); len(got) != 1 || !strings.HasPrefix(got[0], "send t42 ") {
		t.Fatalf("1回目: %q", got)
	}
	st, _, err := LoadState("sample-site")
	if err != nil {
		t.Fatal(err)
	}
	if st.First["S-7|t42"] {
		t.Fatal("送れなかったのに控えに送りかけが残っている")
	}
	f.p.st = st
	f.rm("send-rc")
	if got := f.round(); len(got) != 2 || !strings.HasPrefix(got[0], "send t42 ") || got[1] != "label t42 first_message.sent=" {
		t.Fatalf("再開後に送り直していない: %q", got)
	}
}

func TestFirstMessageReviewer(t *testing.T) {
	f := newFirst(t)
	f.p.tasks = []ledger.Issue{{"id": "S-8", "status": "doing", "agent": "t43", "parent": "S-7", "profile": "codex-review"}}
	brief := filepath.Join(f.d, "projects", "sample-site", "kanban", "issues", "S-8", "brief.md")
	send := "send t43 [dispatcher] 課題 `sample-site/S-8`。|依頼書は `" + brief + "`。|先に `" + filepath.Join(home(), "skills", "worker", "SKILL.md") + "` を読む。"
	if got := f.round(); len(got) != 2 || got[0] != send || got[1] != "label t43 first_message.sent=" {
		t.Fatalf("1回目: %q", got)
	}
	if !strings.Contains(f.log.String(), "S-8 の担当 t43 に最初のメッセージを送った") {
		t.Errorf("送ったことがログに無い: %s", f.log.String())
	}
	if got := f.round(); got != nil {
		t.Fatalf("送ったあとにまた打った: %q", got)
	}
}

func TestSeenBareShellIsGone(t *testing.T) {
	f := newFirst(t)
	f.put("argv", `["/bin/bash"]`)
	f.put("fg", `{"name":"bash","argv":["/bin/bash"]}`)
	if s, ok := f.p.seen("t42"); !ok || !s.Gone {
		t.Fatalf("中身がシェルの端末を生きているとみた: %+v %v", s, ok)
	}
	f.rm("argv")
	if s, ok := f.p.seen("t42"); !ok || s.Gone {
		t.Fatalf("top-level がエージェントの端末を終わったとみた: %+v %v", s, ok)
	}
	f.put("fg", `{"name":"sh","argv":["sh","-c","claude"]}`)
	if s, ok := f.p.seen("t42"); !ok || s.Gone {
		t.Fatalf("起動したエージェントを終わったとみた: %+v %v", s, ok)
	}
}

// 前に貼った最初のメッセージが入力欄に残っていれば、貼り直さず Enter だけ押す（ADR-0007 R6）。
func TestFirstMessageLeftInInput(t *testing.T) {
	f := newFirst(t)
	brief := filepath.Join(f.d, "projects", "sample-site", "kanban", "issues", "S-7", "brief.md")
	f.put("screen", "\n────\n❯ 課題 `sample-site/S-7`。\n  依頼書は `"+brief+"`。\n────\n  ? for shortcuts\n")
	if got := f.round(); len(got) != 2 || got[0] != "keys t42 enter" || got[1] != "label t42 first_message.sent=" {
		t.Fatalf("残っている入力欄: %q", got)
	}
	if !strings.Contains(f.log.String(), "Enter だけ押した") {
		t.Errorf("Enter だけ押したことがログに無い: %s", f.log.String())
	}
	if got := f.round(); got != nil {
		t.Fatalf("押したあとにまた打った: %q", got)
	}

	f = newFirst(t)
	brief = filepath.Join(f.d, "projects", "sample-site", "kanban", "issues", "S-7", "brief.md")
	f.put("screen", "\n────\n❯ [dispatcher] 課題 `sample-site/S-7`。\n  依頼書は `"+brief+"`。\n────\n")
	if got := f.round(); len(got) != 2 || got[0] != "keys t42 enter" {
		t.Fatalf("印つきで残っている入力欄: %q", got)
	}

	f = newFirst(t)
	f.put("screen", "────\n❯ Try \"fix lint errors\"\n────\n")
	if got := f.round(); len(got) != 2 || !strings.HasPrefix(got[0], "send t42 [dispatcher] 課題 ") {
		t.Fatalf("サジェストの入力欄: %q", got)
	}

	f = newFirst(t)
	f.put("screen", "❯ 課題 `sample-site/S-7`。\n\n⏺ 読む\n────\n❯ \n────\n")
	if got := f.round(); len(got) != 2 || !strings.HasPrefix(got[0], "send t42 [dispatcher] 課題 ") {
		t.Fatalf("入力欄が空: %q", got)
	}

	f = newFirst(t)
	f.put("screen", "› 課題 `sample-site/S-7`。\n  依頼書は `x`。\n")
	f.put("send-rc", "1")
	if got := f.round(); len(got) != 1 || got[0] != "keys t42 enter" {
		t.Fatalf("押せない: %q", got)
	}
	if f.p.st.First["S-7|t42"] {
		t.Error("押せなかったのに控えに送ったと持っている")
	}

	f = newFirst(t)
	f.put("read-rc", "1")
	if got := f.round(); got != nil {
		t.Fatalf("読めないのに打った: %q", got)
	}
	f.rm("read-rc")
	if got := f.round(); len(got) != 2 || !strings.HasPrefix(got[0], "send t42 ") {
		t.Fatalf("読めるようになったのに送らない: %q", got)
	}
}

func TestLeftInInput(t *testing.T) {
	first := "課題 `sample-site/S-7`。"
	for _, c := range []struct {
		name, screen string
		want         bool
	}{
		{"Claude Code の入力欄に残っている", "────\n❯ 課題 `sample-site/S-7`。\n  依頼書は `b`。\n────\n", true},
		{"Codex の入力欄に残っている", "› 課題 `sample-site/S-7`。\n  依頼書は `b`。\n\n  ⏎ send\n", true},
		{"空の入力欄", "────\n❯ \n────\n", false},
		{"サジェスト", "────\n❯ Try \"refactor <filepath>\"\n────\n", false},
		{"Codex のサジェスト", "› Improve documentation in @filename\n", false},
		{"ほかの課題の文", "❯ 課題 `sample-site/S-70`。\n", false},
		{"上に写しがあり入力欄は空", "❯ 課題 `sample-site/S-7`。\n⏺ ok\n❯ \n", false},
		{"印の無い画面", "課題 `sample-site/S-7`。\n", false},
	} {
		if got := LeftInInput(c.screen, first); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestReviewerFirstGuide(t *testing.T) {
	f := newFirst(t)
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	if m := f.p.reviewerFirst("S-8"); strings.Contains(m, spawn.GuideFile) {
		t.Errorf("ファイルが無いのに指している: %q", m)
	}
	os.WriteFile(filepath.Join(ws, spawn.GuideFile), []byte("x"), 0o644)
	if m := f.p.reviewerFirst("S-8"); !strings.Contains(m, filepath.Join(ws, spawn.GuideFile)) {
		t.Errorf("ファイルがあるのに指していない: %q", m)
	}
}

// dispatcher が送り直す最初のメッセージは、担当のときだけ、目録の Topics にページがあれば wiki の目録の行を持つ。
// レビュアーと Chief Engineer には、どちらのときも無い（ADR-0061 受け入れ条件 7）。
func TestFirstMessageResentWiki(t *testing.T) {
	for _, c := range []struct {
		name string
		task ledger.Issue
		want bool
	}{
		{"担当", ledger.Issue{"id": "S-7", "status": "doing", "agent": "t42", "profile": "claude"}, true},
		{"レビュアー", ledger.Issue{"id": "S-7", "status": "doing", "agent": "t42", "parent": "S-6", "profile": "codex-review"}, false},
		{"Chief Engineer", ledger.Issue{"id": "S-7", "status": "doing", "agent": "t42", "profile": "chief-engineer"}, false},
	} {
		for _, pages := range []bool{false, true} {
			f := newFirst(t)
			f.p.tasks = []ledger.Issue{c.task}
			ws := os.Getenv("ANDON_WORKSPACE")
			body := "# wiki\n\n## Entities\n\n## Topics\n\n## Sources\n\n- [sources/a.md](sources/a.md) — a\n"
			if pages {
				body = strings.Replace(body, "## Topics\n", "## Topics\n\n- [topics/a.md](topics/a.md) — a\n", 1)
			}
			os.MkdirAll(filepath.Join(ws, "wiki"), 0o755)
			os.WriteFile(filepath.Join(ws, spawn.WikiIndex), []byte(body), 0o644)
			got := f.round()
			if len(got) != 2 {
				t.Fatalf("%s: 送っていない: %q", c.name, got)
			}
			has := strings.Contains(got[0], filepath.Join(ws, spawn.WikiIndex))
			if has != (c.want && pages) {
				t.Errorf("%s（Topics にページ %v）: 目録の行 %v: %q", c.name, pages, has, got[0])
			}
		}
	}
}
