package dispatcher

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
)

// 偽の hc: get は $D/<addr>.json（無ければ not_found）。
const resumeHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) [ -e "$d/$2.json" ] || { echo '{"error":{"kind":"not_found"}}' >&2; exit 3; }; cat "$d/$2.json" ;;
  read) ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

// 偽の kanban: show は $D/show.json。
const resumeKanban = `#!/usr/bin/env bash
case $1 in
  show) cat "$FAKE_HC/show.json" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

// 偽の andon: 打たれた引数を $D/andon に1行。$D/resume-rc があればその終了コードで落ち、
// 通れば $D/next の address を台帳の agent にする（show.json を書き直す）。
const resumeAndon = `#!/usr/bin/env bash
d=$FAKE_HC
echo "$*" >>"$d/andon"
[ -e "$d/resume-rc" ] && { echo "起こせない" >&2; exit "$(cat "$d/resume-rc")"; }
printf '{"issue":{"id":"%s","status":"doing","agent":"%s"}}\n' "$3" "$(cat "$d/next")" >"$d/show.json"
`

type resumeFixture struct {
	t   *testing.T
	d   string
	p   *project
	log bytes.Buffer
}

func newResume(t *testing.T) *resumeFixture {
	t.Helper()
	f := &resumeFixture{t: t, d: t.TempDir()}
	bin := filepath.Join(f.d, "bin")
	root := filepath.Join(f.d, "projects", "sample-site")
	for _, dir := range []string{bin, root} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"hc": resumeHc, "kanban": resumeKanban, "andon": resumeAndon} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", f.d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.d, "state"))
	st := &State{Reviewers: map[string]Reviewer{}, Gates: map[string]Gate{}, Pending: []Notification{}}
	st.ensure()
	f.p = &project{c: &Dispatcher{Log: &f.log, Andon: filepath.Join(bin, "andon")}, l: ledger.At(root), st: st}
	f.agent("t42")
	return f
}

// agent は台帳の S-7（doing）の agent を addr にする（この回に読んだ一覧と、kanban show の両方）。
func (f *resumeFixture) agent(addr string) {
	f.t.Helper()
	f.p.tasks = []ledger.Issue{{"id": "S-7", "status": "doing", "agent": addr, "profile": "claude"}}
	f.put("show.json", `{"issue":{"id":"S-7","status":"doing","agent":"`+addr+`"}}`)
}

func (f *resumeFixture) put(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.d, name), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// live は端末 addr を live（turn）にする。
func (f *resumeFixture) live(addr string, turn int) {
	f.put(addr+".json", `{"state":"live","status":{"state":"working"},"status_ms":1,"turn":`+strconv.Itoa(turn)+`,"labels":{},"argv":["claude"]}`)
}

// round は見回りを1回して、その回に打たれた andon の行を返す。
func (f *resumeFixture) round() []string {
	f.t.Helper()
	os.Remove(filepath.Join(f.d, "andon"))
	f.p.watch(time.Now())
	b, _ := os.ReadFile(filepath.Join(f.d, "andon"))
	if s := strings.TrimSpace(string(b)); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

func (f *resumeFixture) pending() []string {
	var k []string
	for _, n := range f.p.st.Pending {
		k = append(k, n.Issue+":"+n.Kind+":"+n.Detail["agent"].(string))
	}
	return k
}

// recorded は記録だけの置き場の resumed の行。
func (f *resumeFixture) recorded() []Notification {
	f.t.Helper()
	fh, err := os.Open(RecordedPath(f.p.l.Slug))
	if err != nil {
		return nil
	}
	defer fh.Close()
	var out []Notification
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		var n Notification
		if json.Unmarshal(sc.Bytes(), &n) == nil && n.Kind == "resumed" {
			out = append(out, n)
		}
	}
	return out
}

func TestResumeGoneRule(t *testing.T) {
	for _, c := range []struct {
		name  string
		prev  Resumed
		had   bool
		first bool
		want  bool
	}{
		{"初めて見る死", Resumed{}, false, false, true},
		{"初めて見る台帳", Resumed{}, false, true, false},
		{"同じ死にもう打った", Resumed{From: "t42"}, true, false, false},
		{"同じ死にもう打って起こせた（台帳が戻った）", Resumed{From: "t42", Agent: "t43"}, true, false, false},
		{"起こした端末が働かずに死んだ", Resumed{From: "t41", Agent: "t42"}, true, false, false},
		{"控えは別の死（台帳の agent が替わった）", Resumed{From: "t40", Agent: "t41"}, true, false, true},
	} {
		if got := ResumeGone(c.prev, c.had, "t42", c.first); got != c.want {
			t.Errorf("%s: %v（want %v）", c.name, got, c.want)
		}
	}
}

// Gone を見たら resume を1回打ち、記録だけの resumed を書き、finished は知らせない。起こした端末が働かないまま
// また Gone なら打たずに finished を知らせる。
func TestResumeOnceThenFinished(t *testing.T) {
	f := newResume(t)
	f.put("next", "t43")
	if got := f.round(); len(got) != 1 || got[0] != "issue resume S-7" {
		t.Fatalf("Gone の担当に resume を打たない: %q（%s）", got, f.log.String())
	}
	if p := f.pending(); len(p) != 0 {
		t.Fatalf("resume で起こしたのに知らせた: %q", p)
	}
	if r := f.recorded(); len(r) != 1 || r[0].Issue != "S-7" || r[0].Detail["agent"] != "t43" || r[0].Detail["from"] != "t42" {
		t.Fatalf("記録だけの resumed が無い・形が違う: %+v", r)
	}
	if r := f.p.st.Resumed["S-7"]; r != (Resumed{From: "t42", Agent: "t43"}) {
		t.Fatalf("控え: %+v", r)
	}
	// 起こした端末が live で turn 0（まだ働いていない）: 控えは残る
	f.agent("t43")
	f.live("t43", 0)
	f.round()
	if _, ok := f.p.st.Resumed["S-7"]; !ok {
		t.Fatal("働く前に控えを消した")
	}
	// 働かないまま死んだ: 打たずに finished
	os.Remove(filepath.Join(f.d, "t43.json"))
	if got := f.round(); got != nil {
		t.Fatalf("働かずに死んだ端末にまた打った: %q", got)
	}
	if p := f.pending(); len(p) != 1 || p[0] != "S-7:finished:t43" {
		t.Fatalf("finished を知らせない: %q", p)
	}
	// 次の見回りでも打たない・二度知らせない
	f.p.st.Pending = []Notification{}
	if got := f.round(); got != nil || len(f.p.st.Pending) != 0 {
		t.Fatalf("同じ死にまた打った・知らせた: %q %q", got, f.pending())
	}
}

// 働いたあと（turn ≥ 1）に死んだら、また1回打つ。
func TestResumeAgainAfterWork(t *testing.T) {
	f := newResume(t)
	f.put("next", "t43")
	f.round()
	f.agent("t43")
	f.live("t43", 1)
	f.round()
	if _, ok := f.p.st.Resumed["S-7"]; ok {
		t.Fatal("働き始めたのに控えを消さない")
	}
	os.Remove(filepath.Join(f.d, "t43.json"))
	f.put("next", "t44")
	if got := f.round(); len(got) != 1 || got[0] != "issue resume S-7" {
		t.Fatalf("働いたあと死んだ端末に resume を打たない: %q", got)
	}
	if r := f.p.st.Resumed["S-7"]; r != (Resumed{From: "t43", Agent: "t44"}) {
		t.Fatalf("控え: %+v", r)
	}
	if len(f.recorded()) != 2 || len(f.p.st.Pending) != 0 {
		t.Fatalf("2回目の resumed を記録しない・知らせた: %+v %q", f.recorded(), f.pending())
	}
}

// 初めて見る台帳では打たない（次の見回りでも、その死には打たない）。
func TestResumeNotOnFirst(t *testing.T) {
	f := newResume(t)
	f.put("next", "t43")
	f.p.first = true
	if got := f.round(); got != nil {
		t.Fatalf("初めて見る台帳で打った: %q", got)
	}
	f.p.first = false
	if got := f.round(); got != nil {
		t.Fatalf("初めて見た死に次の見回りで打った: %q", got)
	}
	if len(f.p.st.Pending) != 0 {
		t.Fatalf("初めて見た台帳の死を知らせた: %q", f.pending())
	}
}

// resume が起こせなければ今どおり finished を知らせ、同じ死にはもう打たない。doing でなくなったら控えを捨てる。
func TestResumeFails(t *testing.T) {
	f := newResume(t)
	f.put("resume-rc", "1")
	if got := f.round(); len(got) != 1 {
		t.Fatalf("resume を打たない: %q", got)
	}
	if p := f.pending(); len(p) != 1 || p[0] != "S-7:finished:t42" {
		t.Fatalf("起こせないのに finished を知らせない: %q", p)
	}
	if !strings.Contains(f.log.String(), "resume で起こせない") {
		t.Fatalf("ログに理由が無い: %s", f.log.String())
	}
	if got := f.round(); got != nil {
		t.Fatalf("起こせなかった死にまた打った: %q", got)
	}
	f.p.tasks[0]["status"] = "done"
	f.round()
	if _, ok := f.p.st.Resumed["S-7"]; ok {
		t.Fatal("doing でなくなった課題の控えが残る")
	}
}

// resumed は記録だけ。
func TestRouteResumed(t *testing.T) {
	if RouteOf(Notification{Kind: "resumed"}, Facts{}) != Record {
		t.Fatal("resumed を届ける")
	}
}
