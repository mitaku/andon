package courier

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
)

// 偽の hc: get は $D/status（無ければ idle）とラベル（$D/unsent があれば first_message.sent=false）を返す。
// send は $D/send-rc があればその終了コードで落ちる。label は $D/label-rc があれば落ち、通れば $D/unsent を消す。
// 打たれたものは $D/log に1行ずつ（send は本文の改行を | にして）。
const firstHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get)
    labels='{}'; [ -e "$d/unsent" ] && labels='{"first_message.sent":"false"}'
    printf '{"state":"live","status":{"state":"%s"},"status_ms":%s,"turn":0,"labels":%s}\n' "$(cat "$d/status" 2>/dev/null || echo idle)" "$(date +%s%3N)" "$labels" ;;
  send) printf 'send %s %s\n' "$2" "$(printf '%s' "$3" | tr '\n' '|')" >>"$d/log"
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
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.d, "state")) // 送る前の控えの保存を一時の置き場へ（本物の控えに触らない）
	st := &State{Reviewers: map[string]Reviewer{}, Gates: map[string]Gate{}, Pending: []Notification{}}
	st.ensure()
	f.p = &project{c: &Courier{Log: &f.log}, l: ledger.At(filepath.Join(f.d, "projects", "sample-site")), st: st,
		tasks: []ledger.Task{{"id": "S-7", "status": "doing", "agent": "t42"}}}
	f.put("unsent", "")
	return f
}

func (f *firstFixture) put(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.d, name), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *firstFixture) rm(name string) { os.Remove(filepath.Join(f.d, name)) }

// round は見回りを1回して、その回に打たれた hc send・label の行を返す。
func (f *firstFixture) round() []string {
	f.t.Helper()
	f.rm("log")
	f.p.watch(time.Now())
	b, _ := os.ReadFile(filepath.Join(f.d, "log"))
	if s := strings.TrimSpace(string(b)); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

func TestFirstMessageResent(t *testing.T) {
	f := newFirst(t)
	brief := filepath.Join(f.d, "projects", "sample-site", ".flightdeck", "tasks", "S-7", "brief.md")
	send := "send t42 タスク `sample-site/S-7`。|依頼書は `" + brief + "`。"

	// 信頼の画面などで止まっている間は送らない（次の見回りへ持ち越す）
	f.put("status", "blocked")
	if got := f.round(); got != nil {
		t.Fatalf("blocked なのに打った: %q", got)
	}
	// 入力を受けられるようになったが hc send が拒んだ: ラベルは残し、次の見回りでまた送る
	f.put("status", "idle")
	f.put("send-rc", "1")
	if got := f.round(); len(got) != 1 || got[0] != send {
		t.Fatalf("1回目: %q", got)
	}
	if !strings.Contains(f.log.String(), "最初のメッセージを送れない（次の見回りで送り直す）") {
		t.Errorf("送れなかったことがログに無い: %s", f.log.String())
	}
	// 送れた: ラベルを外し、ログに1行
	f.rm("send-rc")
	if got := f.round(); len(got) != 2 || got[0] != send || got[1] != "label t42 first_message.sent=" {
		t.Fatalf("2回目: %q", got)
	}
	if !strings.Contains(f.log.String(), "S-7 の担当 t42 に最初のメッセージを送った") {
		t.Errorf("送ったことがログに無い: %s", f.log.String())
	}
	// ラベルが外れたので、もう送らない（1回だけ）
	if got := f.round(); got != nil {
		t.Fatalf("送ったあとにまた打った: %q", got)
	}
}

func TestFirstMessageNotTwice(t *testing.T) {
	f := newFirst(t)
	// 送れたがラベルを外せない: 控えに「送った」を持ち、ラベルが残っていても二度と送らない
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
	// 人が手で送って動き出していれば送らず、ラベルだけ外す
	f.put("status", "working")
	if got := f.round(); len(got) != 1 || got[0] != "label t42 first_message.sent=" {
		t.Fatalf("動いている担当: %q", got)
	}
	// 初めて見る台帳では動かさない
	f = newFirst(t)
	f.p.first = true
	if got := f.round(); got != nil {
		t.Fatalf("初めて見る台帳で打った: %q", got)
	}
	// ラベルの無い担当には何もしない
	f = newFirst(t)
	f.rm("unsent")
	if got := f.round(); got != nil {
		t.Fatalf("ラベルの無い担当に打った: %q", got)
	}
}

// 送ってからラベルを外す前にクーリエが落ちても、再開後に二度送らない（送る前に控えへ「送りかけ」を保存する）。
func TestFirstMessageCrashBeforeUnlabel(t *testing.T) {
	f := newFirst(t)
	// 送れたがラベルを外せないまま落ちた、とみなす（控えは送る前に保存されている）
	f.put("label-rc", "1")
	f.round()
	b, err := os.ReadFile(StatePath("sample-site"))
	if err != nil || !strings.Contains(string(b), `"S-7|t42": true`) {
		t.Fatalf("送る前に控えへ送りかけを保存していない: %v %s", err, b)
	}
	// 再開: 保存された控えから読み直す。ラベルは残っている
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

// 送れなかったら、送りかけをその場で控えから消して保存する（pass の最後の保存より前に落ちても、再開後に送り直す）。
func TestFirstMessageSendFailsThenCrash(t *testing.T) {
	f := newFirst(t)
	f.put("send-rc", "1")
	if got := f.round(); len(got) != 1 || !strings.HasPrefix(got[0], "send t42 ") {
		t.Fatalf("1回目: %q", got)
	}
	// pass の最後の保存はしないまま落ちた、とみなし、保存された控えから再開する
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

// クーリエが起こしたレビュアー（startReviewer がラベルを付けたもの）にも、見回りで1回だけ送り直す。文はレビュアーの最初のメッセージ。
// hc run が送れなかったときも、送ったが動き出さなかった（入力欄が空のまま idle）ときも、端末のラベルは同じ。
func TestFirstMessageReviewer(t *testing.T) {
	f := newFirst(t)
	f.p.tasks = []ledger.Task{{"id": "S-8", "status": "doing", "agent": "t43", "parent": "S-7", "profile": "codex-review"}}
	brief := filepath.Join(f.d, "projects", "sample-site", ".flightdeck", "tasks", "S-8", "brief.md")
	send := "send t43 タスク `sample-site/S-8`。|依頼書は `" + brief + "`。|先に `" + filepath.Join(home(), "skills", "worker", "SKILL.md") + "` を読む。"
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
