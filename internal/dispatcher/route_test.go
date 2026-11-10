package dispatcher

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		{"blocked", "blocked", nil, Facts{}, Deliver},
		{"error", "error", nil, Facts{}, Deliver},
		{"finished", "finished", nil, Facts{}, Deliver},
		{"stalled", "stalled", nil, Facts{}, Deliver},
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

// approve（merge: auto）のあと approveWait たっても merged が来なければ1回だけ届け、2回目は届けない。
func TestApproveStuck(t *testing.T) {
	p := routeProject(t)
	approve := func(id string, auto bool) {
		f := p.facts(id)
		f.AutoMerge = auto
		p.put(Notification{At: now(), Project: "demo", Issue: id, Kind: "gate", Detail: map[string]any{"decision": "approve", "reasons": []string{}, "record": "/r.json"}}, f)
	}
	approve("D-0001", true)
	approve("D-0003", true)
	if len(p.st.Pending) != 0 || len(recorded(t)) != 2 || len(p.st.Approved) != 2 {
		t.Fatalf("merge: auto の approve を記録だけにして測らない: pending %+v approved %+v", p.st.Pending, p.st.Approved)
	}
	at := p.st.Approved["D-0001"].At
	approve("D-0001", true) // 回し直したゲートでは測り直さない
	if p.st.Approved["D-0001"].At != at {
		t.Fatal("回し直したゲートで測り直した")
	}
	p.notify("D-0003", "merged", map[string]any{"pr": "https://github.com/example/app/pull/7", "sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"})
	start, _ := time.Parse("2006-01-02T15:04:05Z", at)
	p.stuck(start.Add(approveWait - time.Second))
	if len(p.st.Pending) != 0 {
		t.Fatalf("%s の前に届けた: %+v", approveWait, p.st.Pending)
	}
	p.stuck(start.Add(approveWait))
	if len(p.st.Pending) != 1 || p.st.Pending[0].Issue != "D-0001" || p.st.Pending[0].Kind != "gate" {
		t.Fatalf("approve のまま止まったのを1回届けない（merged の来た課題も届けた）: %+v", p.st.Pending)
	}
	p.st.Pending = nil
	p.stuck(start.Add(2 * approveWait))
	if len(p.st.Pending) != 0 {
		t.Fatalf("2回目を届けた: %+v", p.st.Pending)
	}
	// merge: person（マージしない）の approve は同じ巡で届ける
	approve("D-0003", false)
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
