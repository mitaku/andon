package dispatcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"andon/internal/ledger"
)

// estimate の検査: 台帳（課題の一覧と log.jsonl の行）は手で作り、見回りの時刻を進めて p.estimate を打つ。
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
	st := &State{Reviewers: map[string]Reviewer{}, Gates: map[string]Gate{}, Pending: []Notification{}}
	st.ensure()
	f := &estimateFixture{t: t, t0: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)}
	f.p = &project{c: &Dispatcher{Log: os.Stderr}, l: ledger.At(root), st: st}
	f.p.tasks = []ledger.Issue{{"id": "S-7", "status": "doing", "agent": "t7", "profile": "claude", "estimate": "small"}}
	f.row(0, "t5", "S-7", map[string]any{"estimate": "small"})
	f.row(0, "t5", "S-7", map[string]any{"status": "doing"})
	return f
}

// row は log.jsonl の行を足す（t0 から min 分あと）。
func (f *estimateFixture) row(min int, by, id string, change map[string]any) {
	f.p.rows = append(f.p.rows, ledger.Row{At: f.at(min).Format(time.RFC3339), By: "terminal:" + by, ID: id, Change: change})
}

func (f *estimateFixture) at(min int) time.Time { return f.t0.Add(time.Duration(min) * time.Minute) }

// round は t0 から min 分あとの見回りで estimate を打ち、その回に積んだ estimate の段を返す（無ければ 0）。
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

// 見込み小（30 分）で、実働が 1・2・4 倍を超えた見回りでそれぞれ1回だけ積み、同じ段では積まない。
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

// detail は見込みの段・分・実際の分・比・段。
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

// 離席中（届けない間）に 1 倍と 2 倍を続けて超えると、控えには両方の段が残り、待ちの通知は新しい段の1件になる。
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
	// 見回りが止まっていて、一度に 1 倍と 2 倍を超えていたら、上の段を1件だけ積む
	g := newEstimate(t)
	if got := g.round(70); got != 2 {
		t.Fatalf("一度に超えた: 段 %d", got)
	}
	if got := g.round(71); got != 0 {
		t.Fatalf("下の段をあとで積んだ: 段 %d", got)
	}
}

// 見込みを改めたら段は数え直し。
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

// 初めて見る台帳では今の段を控えるだけで積まない（次の見回りでも同じ段は鳴らさない）。
func TestEstimateFirstSight(t *testing.T) {
	f := newEstimate(t)
	f.p.first = true
	if got := f.round(70); got != 0 {
		t.Fatalf("初めて見る台帳で積んだ: %d", got)
	}
	f.p.first = false
	if got := f.round(71); got != 0 {
		t.Fatalf("控えた段を次の見回りで積んだ: %d", got)
	}
	if got := f.round(121); got != 4 {
		t.Fatalf("その上の段: %d", got)
	}
}

// 見込みの無い課題・Chief of Staff でない者が書いた見込み・知らない値・レビュアー・Chief Engineer・人の課題には積まない。
func TestEstimateSkips(t *testing.T) {
	for name, edit := range map[string]func(f *estimateFixture){
		"見込み無し":          func(f *estimateFixture) { delete(f.p.tasks[0], "estimate"); f.p.rows = f.p.rows[1:] },
		"担当が書いた見込み":      func(f *estimateFixture) { f.row(1, "t7", "S-7", map[string]any{"estimate": "small"}) },
		"知らない値":          func(f *estimateFixture) { f.p.tasks[0]["estimate"] = "中" },
		"レビュアー":          func(f *estimateFixture) { f.p.tasks[0]["parent"] = "S-6"; f.p.tasks[0]["profile"] = "codex-review" },
		"Chief Engineer": func(f *estimateFixture) { f.p.tasks[0]["profile"] = "chief-engineer" },
		"人の課題":           func(f *estimateFixture) { f.p.tasks[0]["who"] = "person" },
		"review にいる":     func(f *estimateFixture) { f.p.tasks[0]["status"] = "review" },
	} {
		f := newEstimate(t)
		edit(f)
		if got := f.round(200); got != 0 {
			t.Errorf("%s: 段 %d を積んだ", name, got)
		}
	}
}

// 実働は待ちを除く: 端末が死んでいた区間（Gone を見た時刻から次の agent の行まで）は数えない。
func TestEstimateExcludesGone(t *testing.T) {
	f := newEstimate(t)
	f.p.sawGone("S-7", "t7", f.at(10))
	f.p.sawGone("S-7", "t7", f.at(12)) // 同じ死は最初の時刻だけ
	f.row(30, "t5", "S-7", map[string]any{"agent": "t8"})
	if got := f.round(45); got != 0 { // 45 - (30 - 10) = 25 分
		t.Fatalf("死んでいた区間を実働に数えた: 段 %d", got)
	}
	if got := f.round(51); got != 1 { // 31 分
		t.Fatalf("31 分で積まない: %d", got)
	}
}

// 最初の列に戻った（todo・queued・done・台帳から消えた）課題の控えは捨てる。review の間は持ち越す。
func TestEstimateForget(t *testing.T) {
	f := newEstimate(t)
	f.round(200)
	f.p.sawGone("S-7", "t7", f.at(200))
	f.p.tasks[0]["status"] = "review"
	f.round(201)
	if len(f.p.st.Estimate) != 3 || len(f.p.st.Gone["S-7"]) != 1 {
		t.Fatalf("review の間に控えを捨てた: %v %v", f.p.st.Estimate, f.p.st.Gone)
	}
	f.p.tasks[0]["status"] = "todo"
	f.round(202)
	if len(f.p.st.Estimate) != 0 || len(f.p.st.Gone) != 0 {
		t.Fatalf("todo に戻った課題の控えが残る: %v %v", f.p.st.Estimate, f.p.st.Gone)
	}
}
