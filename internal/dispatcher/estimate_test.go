package dispatcher

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
)

type estimateFixture struct {
	t  *testing.T
	p  *project
	t0 time.Time
}

func newEstimate(t *testing.T) *estimateFixture {
	t.Helper()
	d := t.TempDir()
	root := filepath.Join(d, "projects", "sample-site")
	if err := os.MkdirAll(filepath.Join(root, "kanban"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("chief_of_staff: t5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	st := &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
	st.ensure()
	f := &estimateFixture{t: t, t0: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)}
	f.p = &project{c: &Dispatcher{Log: os.Stderr}, l: ledger.At(root), st: st}
	f.p.tasks = []ledger.Issue{{"id": "S-7", "status": "doing", "agent": "t7", "profile": "claude", "estimate": "small"}}
	f.row(0, "t5", "S-7", map[string]any{"estimate": "small"})
	f.row(0, "t5", "S-7", map[string]any{"status": "doing"})
	return f
}

func (f *estimateFixture) row(min int, by, id string, change map[string]any) {
	f.p.rows = append(f.p.rows, ledger.Row{At: f.at(min).Format(time.RFC3339), By: "terminal:" + by, ID: id, Change: change})
}

func (f *estimateFixture) at(min int) time.Time { return f.t0.Add(time.Duration(min) * time.Minute) }

func (f *estimateFixture) round(min int) int {
	f.t.Helper()
	f.p.st.Pending = []Notification{}
	f.p.estimate(f.at(min), "t5")
	var got int
	for _, n := range f.p.st.Pending {
		if n.Kind != "estimate" {
			continue
		}
		if got != 0 {
			f.t.Fatalf("1回の見回りで estimate を2つ積んだ: %+v", f.p.st.Pending)
		}
		got = n.Detail["stage"].(int)
	}
	return got
}

func TestEstimateStagesOnce(t *testing.T) {
	f := newEstimate(t)
	for _, c := range []struct{ min, want int }{{29, 0}, {30, 0}, {31, 1}, {45, 0}, {61, 2}, {90, 0}, {121, 4}, {300, 0}} {
		if got := f.round(c.min); got != c.want {
			t.Fatalf("%d 分の見回り: 段 %d（want %d）", c.min, got, c.want)
		}
	}
	f.p.st.Pending = []Notification{}
	f.p.estimate(f.at(31), "t5")
	f.p.estimate(f.at(31), "t5")
	if len(f.p.st.Pending) != 0 {
		t.Fatalf("知らせた段をまた積んだ: %+v", f.p.st.Pending)
	}
}

func TestEstimateDetail(t *testing.T) {
	f := newEstimate(t)
	f.p.st.Pending = []Notification{}
	f.p.estimate(f.at(31), "t5")
	if len(f.p.st.Pending) != 1 {
		t.Fatalf("積まない: %+v", f.p.st.Pending)
	}
	n := f.p.st.Pending[0]
	want := map[string]any{"estimate": "small", "estimate_minutes": 30, "worked_minutes": 31, "ratio": 1.03, "stage": 1}
	if n.Issue != "S-7" || n.Kind != "estimate" || len(n.Detail) != len(want) {
		t.Fatalf("形が違う: %+v", n)
	}
	for k, v := range want {
		if n.Detail[k] != v {
			t.Errorf("detail.%s = %v（want %v）", k, n.Detail[k], v)
		}
	}
	if l := line(n); l != "S-7 estimate: 実働 31 分が見込み（small 30 分）の 1 倍を超えた（比 1.03）" {
		t.Errorf("<news> の1行: %s", l)
	}
}

func TestEstimateWhileAway(t *testing.T) {
	f := newEstimate(t)
	f.p.estimate(f.at(31), "t5")
	f.p.estimate(f.at(61), "t5")
	if len(f.p.st.Pending) != 1 || f.p.st.Pending[0].Detail["stage"] != 2 {
		t.Fatalf("待ちの通知: %+v", f.p.st.Pending)
	}
	if !f.p.st.Estimate["S-7|estimate|small|1"] || !f.p.st.Estimate["S-7|estimate|small|2"] {
		t.Fatalf("控えに両方の段が無い: %v", f.p.st.Estimate)
	}
	g := newEstimate(t)
	if got := g.round(70); got != 2 {
		t.Fatalf("一度に超えた: 段 %d", got)
	}
	if got := g.round(71); got != 0 {
		t.Fatalf("下の段をあとで積んだ: 段 %d", got)
	}
}

func TestEstimateRecountOnChange(t *testing.T) {
	f := newEstimate(t)
	if got := f.round(31); got != 1 {
		t.Fatalf("小の 1 倍: %d", got)
	}
	f.p.tasks[0]["estimate"] = "medium"
	f.row(40, "t5", "S-7", map[string]any{"estimate": "medium"})
	if got := f.round(45); got != 0 {
		t.Fatalf("中の 1 倍を超える前に積んだ: %d", got)
	}
	if f.p.st.Estimate["S-7|estimate|small|1"] {
		t.Fatal("古い見込みの控えが残る")
	}
	if got := f.round(61); got != 1 {
		t.Fatalf("中の 1 倍: %d", got)
	}
}

func TestEstimateFirstSight(t *testing.T) {
	f := newEstimate(t)
	f.p.first = true
	if got := f.round(70); got != 0 {
		t.Fatalf("初めて見る kanban で積んだ: %d", got)
	}
	f.p.first = false
	if got := f.round(71); got != 0 {
		t.Fatalf("控えた段を次の見回りで積んだ: %d", got)
	}
	if got := f.round(121); got != 4 {
		t.Fatalf("その上の段: %d", got)
	}
}

func TestEstimateSkips(t *testing.T) {
	for name, edit := range map[string]func(f *estimateFixture){
		"見込み無し":          func(f *estimateFixture) { delete(f.p.tasks[0], "estimate"); f.p.rows = f.p.rows[1:] },
		"担当が書いた見込み":      func(f *estimateFixture) { f.row(1, "t7", "S-7", map[string]any{"estimate": "small"}) },
		"知らない値":          func(f *estimateFixture) { f.p.tasks[0]["estimate"] = "中" },
		"レビュアー":          func(f *estimateFixture) { f.p.tasks[0]["parent"] = "S-6"; f.p.tasks[0]["profile"] = "codex-review" },
		"Chief Engineer": func(f *estimateFixture) { f.p.tasks[0]["profile"] = "chief-engineer" },
		"オーナーの課題":        func(f *estimateFixture) { f.p.tasks[0]["who"] = "person" },
		"review にいる":     func(f *estimateFixture) { f.p.tasks[0]["status"] = "review" },
		"前の Chief of Staff が受けた課題": func(f *estimateFixture) {
			f.p.rows = append([]ledger.Row{{At: f.at(0).Format(time.RFC3339), By: "terminal:t3", Config: map[string]any{"chief_of_staff": "t3"}}}, f.p.rows...)
			f.row(0, "t3", "S-7", map[string]any{"agent": "t3"})
			f.p.rows = append(f.p.rows, ledger.Row{At: f.at(1).Format(time.RFC3339), By: "terminal:t5", Config: map[string]any{"chief_of_staff": "t5"}})
			f.p.tasks[0]["agent"] = "t3"
		},
	} {
		f := newEstimate(t)
		edit(f)
		if got := f.round(200); got != 0 {
			t.Errorf("%s: 段 %d を積んだ", name, got)
		}
	}
}

func TestEstimateExcludesGone(t *testing.T) {
	f := newEstimate(t)
	f.p.sawGone("S-7", "t7", f.at(10))
	f.p.sawGone("S-7", "t7", f.at(12))
	f.row(30, "t5", "S-7", map[string]any{"agent": "t8"})
	if got := f.round(45); got != 0 {
		t.Fatalf("死んでいた区間を実働に数えた: 段 %d", got)
	}
	if got := f.round(51); got != 1 {
		t.Fatalf("31 分で積まない: %d", got)
	}
}

func TestEstimateForget(t *testing.T) {
	f := newEstimate(t)
	f.round(200)
	f.p.sawGone("S-7", "t7", f.at(200))
	f.p.tasks[0]["status"] = "review"
	f.round(201)
	if len(f.p.st.Estimate) != 3 || len(f.p.st.Gone["S-7"]) != 1 {
		t.Fatalf("review の間に控えを捨てた: %v %v", f.p.st.Estimate, f.p.st.Gone)
	}
	f.p.tasks[0]["status"] = "done"
	f.round(202)
	if len(f.p.st.Estimate) != 0 || len(f.p.st.Gone["S-7"]) != 1 {
		t.Fatalf("done で段を残した・Gone を捨てた（measure が読む）: %v %v", f.p.st.Estimate, f.p.st.Gone)
	}
	f.p.tasks[0]["status"] = "todo"
	f.round(203)
	if len(f.p.st.Estimate) != 0 || len(f.p.st.Gone) != 0 {
		t.Fatalf("todo に戻った課題の控えが残る: %v %v", f.p.st.Estimate, f.p.st.Gone)
	}
}

func (f *estimateFixture) worktree(at int, files map[string]int) string {
	f.t.Helper()
	d := f.t.TempDir()
	f.t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	f.t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	git := func(dir string, min int, args ...string) {
		f.t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=a", "-c", "user.email=a@example.com", "-C", dir}, args...)...)
		date := f.at(min).Format(time.RFC3339)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			f.t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	origin, wt := filepath.Join(d, "origin"), filepath.Join(d, "wt")
	git(d, 0, "init", "-q", "-b", "main", origin)
	if err := os.WriteFile(filepath.Join(origin, "README.md"), []byte("a\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	git(origin, -10, "add", ".")
	git(origin, -10, "commit", "-q", "-m", "base")
	git(d, 0, "clone", "-q", origin, wt)
	git(wt, 0, "switch", "-q", "-c", "work")
	for name, n := range files {
		p := filepath.Join(wt, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x\n", n)), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
	git(wt, 0, "add", ".")
	if at >= 0 {
		git(wt, at, "commit", "-q", "-m", "work")
	}
	return wt
}

func (f *estimateFixture) signOf(min int) (int, any) {
	f.t.Helper()
	stage := f.round(min)
	for _, n := range f.p.st.Pending {
		if n.Kind == "estimate" {
			return stage, n.Detail["sign"]
		}
	}
	return stage, nil
}

// 詰まりの兆し（ADR-0017 受け入れ条件 9）: 偽の worktree で、見込み（小 30 分）の分だけ commit が無い・テストを除いた変更が
// 500 行を超えたとき、段の estimate の detail.sign に no-commit・large-diff が載る。段の間では通知が出ない。
func TestEstimateSign(t *testing.T) {
	f := newEstimate(t)
	f.p.tasks[0]["worktree"] = f.worktree(5, map[string]int{"main.go": 300, "web/app.js": 201})
	if stage, sign := f.signOf(31); stage != 1 || !reflect.DeepEqual(sign, []string{"large-diff"}) {
		t.Fatalf("commit から 26 分・501 行: 段 %d sign %v", stage, sign)
	}
	if stage, _ := f.signOf(50); stage != 0 {
		t.Fatalf("段の間に積んだ: %d", stage)
	}
	if stage, sign := f.signOf(61); stage != 2 || !reflect.DeepEqual(sign, []string{"no-commit", "large-diff"}) {
		t.Fatalf("commit から 56 分・501 行: 段 %d sign %v", stage, sign)
	}
	n := f.p.st.Pending[0]
	b, _ := json.Marshal(n)
	var back Notification
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	for _, m := range []Notification{n, back} {
		if l := line(m); l != "S-7 estimate: 実働 61 分が見込み（small 30 分）の 2 倍を超えた（比 2.03）。兆し: no-commit・large-diff" {
			t.Errorf("<news> の1行: %s", l)
		}
	}

	f = newEstimate(t)
	f.p.tasks[0]["worktree"] = f.worktree(5, map[string]int{"main.go": 500, "tests/a.sh": 900, "pkg/tests/b.sh": 900, "x_test.go": 900})
	if stage, sign := f.signOf(31); stage != 1 || sign != nil {
		t.Fatalf("テストを除いて 500 行・commit から 26 分: 段 %d sign %v", stage, sign)
	}

	f = newEstimate(t)
	f.p.tasks[0]["worktree"] = f.worktree(-1, map[string]int{"main.go": 10})
	if stage, sign := f.signOf(31); stage != 1 || !reflect.DeepEqual(sign, []string{"no-commit"}) {
		t.Fatalf("commit 無し: 段 %d sign %v", stage, sign)
	}

	f = newEstimate(t)
	f.p.tasks[0]["worktree"] = f.worktree(-1, map[string]int{"main.go": 900})
	delete(f.p.tasks[0], "estimate")
	f.p.rows = f.p.rows[1:]
	if stage, _ := f.signOf(200); stage != 0 {
		t.Fatalf("見込みの無い課題に積んだ: %d", stage)
	}
}
