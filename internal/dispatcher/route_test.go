package dispatcher

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// 扱いの表は ADR-0010 決めたこと 1（と 7）と同じ。
func TestRouteOf(t *testing.T) {
	g := func(d string) map[string]any { return map[string]any{"decision": d} }
	for _, c := range []struct {
		name string
		kind string
		d    map[string]any
		f    Facts
		want Route
	}{
		{"review-requested", "review-requested", nil, Facts{}, Record},
		{"担当の review（ゲートを回す）", "review", nil, Facts{Gated: true}, Record},
		{"Chief Engineer・調べる課題の review（ゲートを回さない）", "review", nil, Facts{}, Deliver},
		{"gate fix", "gate", g("fix"), Facts{}, Record},
		{"gate needs_human", "gate", g("needs_human"), Facts{}, Record},
		{"gate approve（マージしない）", "gate", g("approve"), Facts{}, Deliver},
		{"gate approve（merge: auto）", "gate", g("approve"), Facts{AutoMerge: true}, Wait},
		{"merged", "merged", map[string]any{"pr": "x"}, Facts{}, Record},
		{"merged（人・片付けた）", "merged", map[string]any{"by": "person"}, Facts{}, Record},
		{"merged（片付けていない）", "merged", map[string]any{"by": "person", "held": "x"}, Facts{}, Deliver},
		{"担当の report", "report", nil, Facts{}, Deliver},
		{"レビュアーの report", "report", nil, Facts{Reviewer: true}, Record},
		{"ready", "ready", nil, Facts{}, Deliver},
		{"asked", "asked", nil, Facts{}, Deliver},
		{"impact", "impact", nil, Facts{}, Deliver},
		{"closed", "closed", nil, Facts{}, Deliver},
		{"corrections", "corrections", nil, Facts{}, Deliver},
		{"signal", "signal", nil, Facts{}, Deliver},
		{"blocked", "blocked", nil, Facts{}, Deliver},
		{"error", "error", nil, Facts{}, Deliver},
		{"finished", "finished", nil, Facts{}, Deliver},
		{"stalled", "stalled", nil, Facts{}, Deliver},
		{"Chief Engineer の stalled", "stalled", nil, Facts{Engineer: true}, Record},
		{"Chief Engineer の blocked", "blocked", nil, Facts{Engineer: true}, Record},
		{"Chief Engineer の ready", "ready", nil, Facts{Engineer: true}, Record},
		{"Chief Engineer の error", "error", nil, Facts{Engineer: true}, Deliver},
		{"Chief Engineer の finished", "finished", nil, Facts{Engineer: true}, Deliver},
		{"Chief Engineer の review（文書の PR）", "review", nil, Facts{Engineer: true}, Deliver},
		{"Chief Engineer の report", "report", nil, Facts{Engineer: true}, Deliver},
		{"Chief Engineer の asked", "asked", nil, Facts{Engineer: true}, Deliver},
	} {
		if got := RouteOf(Notification{Kind: c.kind, Detail: c.d}, c.f); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func routeProject(t *testing.T) *project {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	l := ledger.At(t.TempDir())
	l.Slug = "demo"
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\n"), 0o644)
	st, _, err := LoadState("demo")
	if err != nil {
		t.Fatal(err)
	}
	return &project{c: &Dispatcher{Log: os.Stderr}, l: l, st: st, tasks: []ledger.Issue{
		{"id": "D-0001", "status": "review", "profile": "claude", "repo": "github.com/example/app"},
		{"id": "D-0002", "status": "review", "profile": "chief-engineer", "repo": "github.com/example/app"},
		{"id": "D-0001-r1", "status": "review", "profile": "codex-review", "parent": "D-0001"},
		{"id": "D-0003", "status": "review", "profile": "claude", "repo": "github.com/example/app"},
	}}
}

func recorded(t *testing.T) []Notification {
	f, err := os.Open(RecordedPath("demo"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []Notification
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var n Notification
		if err := json.Unmarshal(sc.Bytes(), &n); err != nil {
			t.Fatalf("記録の行が通知の JSON でない: %q", sc.Text())
		}
		out = append(out, n)
	}
	return out
}

// review-requested・merged・gate: fix だけが起きた見回りは <news> を1つも渡さず、記録に1行ずつ書く。
func TestRecordOnly(t *testing.T) {
	p := routeProject(t)
	p.notify("D-0001", "review-requested", map[string]any{"round": 1, "reviewer": "codex-review", "reviewer_issue": "D-0001-r1"})
	p.notify("D-0001", "gate", map[string]any{"decision": "fix", "reasons": []string{"x"}, "record": "/r.json"})
	p.notify("D-0001", "merged", map[string]any{"pr": "https://github.com/example/app/pull/7", "sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"})
	p.notify("D-0001-r1", "report", map[string]any{"report": "/x/report.md"})
	p.notify("D-0001", "review", map[string]any{"report": "/x/report.md", "pr": "https://github.com/example/app/pull/7"})
	if len(p.st.Pending) != 0 {
		t.Fatalf("記録だけの通知を積んだ: %+v", p.st.Pending)
	}
	got := recorded(t)
	if len(got) != 5 || got[0].Kind != "review-requested" || got[1].Kind != "gate" || got[2].Kind != "merged" || got[3].Kind != "report" || got[4].Kind != "review" {
		t.Fatalf("記録が1行ずつでない: %+v", got)
	}
	if filepath.Base(RecordedPath("demo")) != "demo.recorded.jsonl" || filepath.Dir(RecordedPath("demo")) != filepath.Dir(StatePath("demo")) {
		t.Fatalf("置き場が控えの横でない: %s", RecordedPath("demo"))
	}
	// Chief Engineer の課題の review と担当の report は今どおり届く
	p.notify("D-0002", "review", map[string]any{"report": "/x/report.md", "pr": "https://github.com/example/app/pull/8"})
	p.notify("D-0003", "report", map[string]any{"report": "/x/report.md"})
	if len(p.st.Pending) != 2 || p.st.Pending[0].Issue != "D-0002" || p.st.Pending[1].Kind != "report" {
		t.Fatalf("届ける通知を積まない: %+v", p.st.Pending)
	}
	if len(recorded(t)) != 5 {
		t.Fatal("届けた通知を記録に書いた")
	}
}

// Chief Engineer の課題（profile chief-engineer）の stalled・blocked・ready は記録だけ。error・finished と、
// 担当の同じ通知は今どおり届く（A-0036）。
func TestEngineerRecordOnly(t *testing.T) {
	p := routeProject(t)
	at := map[string]any{"agent": "t9", "status_ms": 1}
	p.notify("D-0002", "stalled", at)
	p.notify("D-0002", "blocked", at)
	p.notify("D-0002", "ready", map[string]any{"title": "設計"})
	if len(p.st.Pending) != 0 {
		t.Fatalf("Chief Engineer の止まり・ready を積んだ: %+v", p.st.Pending)
	}
	if got := recorded(t); len(got) != 3 || got[0].Kind != "stalled" || got[1].Kind != "blocked" || got[2].Kind != "ready" {
		t.Fatalf("記録が1行ずつでない: %+v", got)
	}
	p.notify("D-0002", "error", at)
	p.notify("D-0002", "finished", at)
	p.notify("D-0003", "stalled", at)
	p.notify("D-0003", "blocked", at)
	p.notify("D-0003", "ready", map[string]any{"title": "x"})
	if len(p.st.Pending) != 5 {
		t.Fatalf("届ける通知を積まない: %+v", p.st.Pending)
	}
}

// merge: auto の approve は記録だけにし、andon issue merge の記録の approved から approveWait たっても決まらなければ
// 通知 gate を1回だけ届ける。決まった（approved が消えた）・review にいない課題は届けない。merge: person の approve はすぐ届ける。
func TestApproveStuck(t *testing.T) {
	p := routeProject(t)
	p.tasks = append(p.tasks, ledger.Issue{"id": "D-0004", "status": "done", "profile": "claude", "repo": "github.com/example/app"})
	head := "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"
	at := "2026-11-14T00:00:00Z"
	writeMerge := func(rec map[string]contract.Merging) {
		if err := os.MkdirAll(filepath.Dir(MergePath("demo")), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := contract.Write(MergePath("demo"), rec); err != nil {
			t.Fatal(err)
		}
	}
	gateRecord := filepath.Join(p.l.Library("D-0001"), "gate", head+".json")
	os.MkdirAll(filepath.Dir(gateRecord), 0o755)
	os.WriteFile(gateRecord, []byte(`{"decision":"approve","reasons":["L0"]}`), 0o644)
	writeMerge(map[string]contract.Merging{
		"D-0001": {Row: 3, Sha: head, Approved: at},                      // マージに進み始めて決まらない
		"D-0003": {Row: 5, Sha: head, Decision: "approve", Merged: head}, // マージした
		"D-0004": {Row: 7, Sha: head, Approved: at},                      // review にいない
	})
	p.put(Notification{At: at, Project: "demo", Issue: "D-0001", Kind: "gate", Detail: map[string]any{"decision": "approve", "reasons": []string{}, "record": gateRecord}}, Facts{Gated: true, AutoMerge: true})
	if len(p.st.Pending) != 0 || len(recorded(t)) != 1 {
		t.Fatalf("merge: auto の approve を記録だけにしない: pending %+v", p.st.Pending)
	}
	start, _ := time.Parse("2006-01-02T15:04:05Z", at)
	p.stuck(start.Add(approveWait - time.Second))
	if len(p.st.Pending) != 0 {
		t.Fatalf("%s の前に届けた: %+v", approveWait, p.st.Pending)
	}
	p.stuck(start.Add(approveWait))
	want := Notification{At: at, Project: "demo", Issue: "D-0001", Kind: "gate", Detail: map[string]any{"decision": "approve", "reasons": []string{"L0"}, "record": gateRecord}}
	if len(p.st.Pending) != 1 || !reflect.DeepEqual(p.st.Pending[0], want) {
		t.Fatalf("approve のまま止まったのを1回だけ届けない（決まった・review にいない課題も届けた）:\n got %+v\nwant %+v", p.st.Pending, want)
	}
	p.st.Pending = nil
	p.stuck(start.Add(2 * approveWait))
	if len(p.st.Pending) != 0 {
		t.Fatalf("2回目を届けた: %+v", p.st.Pending)
	}
	// 回し直して、また決まらないまま新しい approved になったら、それは届ける
	later := "2026-11-14T01:00:00Z"
	writeMerge(map[string]contract.Merging{"D-0001": {Row: 9, Sha: head, Approved: later}})
	p.stuck(start.Add(time.Hour + approveWait))
	if len(p.st.Pending) != 1 || p.st.Pending[0].At != later {
		t.Fatalf("新しい approve のまま止まったのを届けない: %+v", p.st.Pending)
	}
	// merge: person（マージしない）の approve は同じ巡で届ける
	p.st.Pending = nil
	p.put(Notification{At: at, Project: "demo", Issue: "D-0003", Kind: "gate", Detail: map[string]any{"decision": "approve", "reasons": []string{}, "record": gateRecord}}, Facts{Gated: true})
	if len(p.st.Pending) != 1 || p.st.Pending[0].Issue != "D-0003" {
		t.Fatalf("merge: person の approve をすぐ届けない: %+v", p.st.Pending)
	}
}

// 記録の置き場に書けなければ控えに残して届けず、書けるようになったら1行だけ書く。
func TestRecordRetry(t *testing.T) {
	p := routeProject(t)
	path := RecordedPath("demo")
	if err := os.MkdirAll(path, 0o755); err != nil { // 置き場に同名のディレクトリ
		t.Fatal(err)
	}
	p.notify("D-0001", "gate", map[string]any{"decision": "fix", "reasons": []string{"x"}, "record": "/r.json"})
	if len(p.st.Pending) != 0 || len(p.st.Unrecorded) != 1 {
		t.Fatalf("書けなかった記録を控えに残さない: pending %+v unrecorded %+v", p.st.Pending, p.st.Unrecorded)
	}
	if err := p.st.Save("demo"); err != nil {
		t.Fatal(err)
	}
	st, _, err := LoadState("demo")
	if err != nil || len(st.Unrecorded) != 1 {
		t.Fatalf("控えを読み直すと残っていない: %+v %v", st.Unrecorded, err)
	}
	p.st = st
	os.Remove(path)
	p.stuck(time.Now())
	got := recorded(t)
	if len(got) != 1 || got[0].Kind != "gate" || len(p.st.Unrecorded) != 0 {
		t.Fatalf("書けるようになったのに1行だけ書かない: %+v unrecorded %+v", got, p.st.Unrecorded)
	}
	p.stuck(time.Now())
	if len(recorded(t)) != 1 {
		t.Fatal("同じ記録を二度書いた")
	}
}

// 一部だけ書いて失敗したら半端な行を残さず、書き直しは書けていない行だけを足す（控えを保存する前に落ちても二重に書かない）。
func TestAppendLinesPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.recorded.jsonl")
	a, b, c := []byte(`{"n":1}`), []byte(`{"n":2}`), []byte(`{"n":3}`)
	if err := appendLines(path, [][]byte{a}); err != nil {
		t.Fatal(err)
	}
	defer func(w func(*os.File, []byte) (int, error)) { write = w }(write)
	write = func(f *os.File, p []byte) (int, error) {
		n, _ := f.Write(p[:len(p)/2])
		return n, os.ErrDeadlineExceeded
	}
	if err := appendLines(path, [][]byte{b, c}); err == nil {
		t.Fatal("一部だけ書いた失敗を返さない")
	}
	if got, _ := os.ReadFile(path); string(got) != "{\"n\":1}\n" {
		t.Fatalf("半端な行を残した: %q", got)
	}
	write = func(f *os.File, p []byte) (int, error) { return f.Write(p) }
	for i := 0; i < 2; i++ { // 2回目は「追記のあと控えを保存する前に落ちた」書き直し
		if err := appendLines(path, [][]byte{b, c}); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := os.ReadFile(path); string(got) != "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n" {
		t.Fatalf("書き直しで二重に書いた・順が違う: %q", got)
	}
	// 先頭の1行だけ書けていた（その位置が行の頭）なら、残りだけを足す
	if err := appendLines(path, [][]byte{c, []byte(`{"n":4}`)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n{\"n\":4}\n" {
		t.Fatalf("書けていた行を飛ばさない: %q", got)
	}
	// 行の途中と一致しただけ（"x{"n":5}" の末尾）は書けていたことにしない
	os.WriteFile(path, []byte("{\"m\":0,\"n\":5}\n"), 0o644)
	if err := appendLines(path, [][]byte{[]byte(`"n":5}`)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "{\"m\":0,\"n\":5}\n\"n\":5}\n" {
		t.Fatalf("行の途中の一致で飛ばした: %q", got)
	}
}
