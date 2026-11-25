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

const resumeHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) [ -e "$d/$2.json" ] || { echo '{"error":{"kind":"not_found"}}' >&2; exit 3; }; cat "$d/$2.json" ;;
  read) ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

const resumeKanban = `#!/usr/bin/env bash
case $1 in
  show) cat "$FAKE_HC/show.json" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const resumeAndon = `#!/usr/bin/env bash
d=$FAKE_HC
echo "$*" >>"$d/andon"
[ -e "$d/resume-rc" ] && { echo "起動できない" >&2; exit "$(cat "$d/resume-rc")"; }
[ ! -e "$d/resume-warn" ] || cat "$d/resume-warn" >&2
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
	st := &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
	st.ensure()
	f.p = &project{c: &Dispatcher{Log: &f.log, Andon: filepath.Join(bin, "andon")}, l: ledger.At(root), st: st}
	f.agent("t42")
	return f
}

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

func (f *resumeFixture) live(addr string, turn int) {
	f.put(addr+".json", `{"state":"live","status":{"state":"working"},"status_ms":1,"turn":`+strconv.Itoa(turn)+`,"labels":{},"argv":["claude"]}`)
}

func (f *resumeFixture) round() []string {
	f.t.Helper()
	os.Remove(filepath.Join(f.d, "andon"))
	f.p.watch(time.Now(), "")
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
		{"初めて見る kanban", Resumed{}, false, true, false},
		{"同じ死にもう打った", Resumed{From: "t42"}, true, false, false},
		{"同じ死にもう打って起動できた（kanban が戻った）", Resumed{From: "t42", Agent: "t43"}, true, false, false},
		{"起動した端末が働かずに死んだ", Resumed{From: "t41", Agent: "t42"}, true, false, false},
		{"控えは別の死（kanban の agent が替わった）", Resumed{From: "t40", Agent: "t41"}, true, false, true},
	} {
		if got := ResumeGone(c.prev, c.had, "t42", c.first); got != c.want {
			t.Errorf("%s: %v（want %v）", c.name, got, c.want)
		}
	}
}

func TestResumeOnceThenFinished(t *testing.T) {
	f := newResume(t)
	f.put("next", "t43")
	if got := f.round(); len(got) != 1 || got[0] != "issue resume S-7" {
		t.Fatalf("Gone の担当に resume を打たない: %q（%s）", got, f.log.String())
	}
	if p := f.pending(); len(p) != 0 {
		t.Fatalf("resume で起動したのに知らせた: %q", p)
	}
	if r := f.recorded(); len(r) != 1 || r[0].Issue != "S-7" || r[0].Detail["agent"] != "t43" || r[0].Detail["from"] != "t42" {
		t.Fatalf("記録だけの resumed が無い・形が違う: %+v", r)
	}
	if r := f.p.st.Resumed["S-7"]; r != (Resumed{From: "t42", Agent: "t43"}) {
		t.Fatalf("控え: %+v", r)
	}
	if saved, _, err := LoadState(f.p.l.Slug); err != nil || saved.Resumed["S-7"] != (Resumed{From: "t42", Agent: "t43"}) {
		t.Fatalf("起動した端末をその場で保存していない: %+v %v", saved, err)
	}
	f.agent("t43")
	f.live("t43", 0)
	f.round()
	if _, ok := f.p.st.Resumed["S-7"]; !ok {
		t.Fatal("働く前に控えを消した")
	}
	os.Remove(filepath.Join(f.d, "t43.json"))
	if got := f.round(); got != nil {
		t.Fatalf("働かずに死んだ端末にまた打った: %q", got)
	}
	if p := f.pending(); len(p) != 1 || p[0] != "S-7:finished:t43" {
		t.Fatalf("finished を知らせない: %q", p)
	}
	f.p.st.Pending = []Notification{}
	if got := f.round(); got != nil || len(f.p.st.Pending) != 0 {
		t.Fatalf("同じ死にまた打った・知らせた: %q %q", got, f.pending())
	}
}

// 起動できた resume の警告（道具を入れられない・入れた。ADR-0063 決めたこと 7）は、dispatcher のログに残す。
// 捨てると、作り直しのあとの自動の resume で警告が誰にも届かない
func TestResumeLogsWarnings(t *testing.T) {
	f := newResume(t)
	f.put("next", "t43")
	warn := "andon issue: /wt の mise.toml の道具を入れられない（起動は続ける）: boom"
	f.put("resume-warn", warn+"\n")
	if got := f.round(); len(got) != 1 || got[0] != "issue resume S-7" {
		t.Fatalf("resume を打たない: %q（%s）", got, f.log.String())
	}
	if !strings.Contains(f.log.String(), "S-7 の resume: "+warn) {
		t.Errorf("resume の警告がログに無い: %q", f.log.String())
	}
}

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
	saved, _, err := LoadState(f.p.l.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := saved.Resumed["S-7"]; ok {
		t.Fatal("働き始めて控えを消したことを保存していない")
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

func TestResumeNotOnFirst(t *testing.T) {
	f := newResume(t)
	f.put("next", "t43")
	f.p.first = true
	if got := f.round(); got != nil {
		t.Fatalf("初めて見る kanban で打った: %q", got)
	}
	if len(f.p.st.Pending) != 0 {
		t.Fatalf("初めて見る kanban で知らせた: %q", f.pending())
	}
	f.p.first = false
	if got := f.round(); got != nil {
		t.Fatalf("初めて見た死に次の見回りで打った: %q", got)
	}
	if p := f.pending(); len(p) != 1 || p[0] != "S-7:finished:t42" {
		t.Fatalf("初めて見た kanban の死を次の見回りで finished で知らせない: %q", p)
	}
	for i := 0; i < 2; i++ {
		if got := f.round(); got != nil {
			t.Fatalf("巡 %d で打った: %q", i+3, got)
		}
		if p := f.pending(); len(p) != 1 {
			t.Fatalf("巡 %d で知らせ直した: %q", i+3, p)
		}
	}
	if !f.p.st.Sent["S-7|t42|gone"] {
		t.Fatalf("知らせたのに Sent に gone が無い: %v", f.p.st.Sent)
	}
}

func TestResumeFails(t *testing.T) {
	f := newResume(t)
	f.put("resume-rc", "1")
	if got := f.round(); len(got) != 1 {
		t.Fatalf("resume を打たない: %q", got)
	}
	if p := f.pending(); len(p) != 1 || p[0] != "S-7:finished:t42" {
		t.Fatalf("起動できないのに finished を知らせない: %q", p)
	}
	if !strings.Contains(f.log.String(), "resume で起動できない") {
		t.Fatalf("ログに理由が無い: %s", f.log.String())
	}
	if got := f.round(); got != nil {
		t.Fatalf("起動できなかった死にまた打った: %q", got)
	}
	f.p.tasks[0]["status"] = "done"
	f.round()
	if _, ok := f.p.st.Resumed["S-7"]; ok {
		t.Fatal("doing でなくなった課題の控えが残る")
	}
}

func TestRouteResumed(t *testing.T) {
	if RouteOf(Notification{Kind: "resumed"}, Facts{}) != Record {
		t.Fatal("resumed を届ける")
	}
}

func TestPastChiefOfStaffIssueNotWatched(t *testing.T) {
	cosRow := func(addr string) ledger.Row {
		return ledger.Row{At: "2026-11-01T00:00:00Z", By: "andon", Config: map[string]any{"chief_of_staff": addr}}
	}
	agentRow := ledger.Row{At: "2026-11-01T00:01:00Z", By: "terminal:t42", ID: "S-7", Change: map[string]any{"status": "doing", "agent": "t42"}}
	round := func(f *resumeFixture) []string {
		f.t.Helper()
		os.Remove(filepath.Join(f.d, "andon"))
		f.p.watch(time.Now(), "t50")
		b, _ := os.ReadFile(filepath.Join(f.d, "andon"))
		if s := strings.TrimSpace(string(b)); s != "" {
			return strings.Split(s, "\n")
		}
		return nil
	}

	f := newResume(t)
	f.put("next", "t43")
	f.p.rows = []ledger.Row{cosRow("t42"), agentRow, cosRow("t50")}
	for i := 0; i < 2; i++ {
		if got := round(f); got != nil {
			t.Fatalf("%d 回目: 前の Chief of Staff の課題に打った: %q（%s）", i+1, got, f.log.String())
		}
	}
	if p := f.pending(); len(p) != 0 {
		t.Fatalf("前の Chief of Staff の課題を知らせた: %q", p)
	}
	if _, ok := f.p.st.Resumed["S-7"]; ok {
		t.Fatalf("前の Chief of Staff の死を控えた: %+v", f.p.st.Resumed)
	}

	f = newResume(t)
	f.put("next", "t43")
	f.p.rows = []ledger.Row{{At: "2026-11-01T00:00:00Z", By: "andon", Config: map[string]any{"coordinator": "t42"}}, agentRow, cosRow("t50")}
	for i := 0; i < 2; i++ {
		if got := round(f); got != nil {
			t.Fatalf("coordinator の行: %d 回目: 前の Chief of Staff の課題に打った: %q（%s）", i+1, got, f.log.String())
		}
	}
	if p := f.pending(); len(p) != 0 {
		t.Fatalf("coordinator の行: 前の Chief of Staff の課題を知らせた: %q", p)
	}

	// 当たる場合も確かめる: 同じアドレスを Chief of Staff だったあとに担当が使ったら、担当の死として resume する
	f = newResume(t)
	f.put("next", "t43")
	f.p.rows = []ledger.Row{cosRow("t42"), cosRow("t50"), agentRow}
	if got := round(f); len(got) != 1 || got[0] != "issue resume S-7" {
		t.Fatalf("担当の死に resume を打たない: %q（%s）", got, f.log.String())
	}
}
