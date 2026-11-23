package dispatcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
)

// idleRig は空きの段を、偽の kanban（routine run と lint を答える）と偽の時計・偽の枠・偽の未処理で回す。
type idleRig struct {
	t        *testing.T
	p        *project
	tools    *scripted
	log      *strings.Builder
	clk      time.Time
	free     int
	unsorted int
}

func newIdleRig(t *testing.T) *idleRig {
	t.Helper()
	r := &idleRig{t: t, log: &strings.Builder{}, clk: time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), free: 2}
	r.tools = newScripted(t, "kanban")
	r.tools.reply("kanban lint --json", `{"ok":true,"problems":[]}`)
	r.tools.reply("kanban routine run triage", "T-0090")
	r.tools.reply("kanban routine run retro", "T-0091")
	for _, name := range []string{"check", "a-check", "b-check"} {
		r.tools.reply("kanban routine run "+name, "T-0092")
	}
	r.p = gluedProject(t, r.log)
	r.p.c.WS = t.TempDir() // project.json が無い（idle_minutes は既定の 15、retro_every は既定の 25）
	r.p.c.Now = func() time.Time { return r.clk }
	ready := []string{}
	r.p.st.Ready = &ready
	slots, unsorted := slotsOf, unsortedOf
	t.Cleanup(func() { slotsOf, unsortedOf = slots, unsorted })
	slotsOf = func(string, time.Time) (int, error) { return r.free, nil }
	unsortedOf = func(string, time.Time) (int, error) { return r.unsorted, nil }
	return r
}

// routine は kanban にルーチン name を置く。
func (r *idleRig) routine(name, extra string) {
	r.t.Helper()
	dir := filepath.Join(r.p.l.Dir, "routines")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte("title: "+name+"\nschedule: \"0 0 1 1 *\"\n"+extra), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// look は時計を d 進めて空きの段を1回回し、打った kanban の CLI の引数を返す。
func (r *idleRig) look(d time.Duration, retroFired bool) []string {
	r.t.Helper()
	r.clk = r.clk.Add(d)
	r.p.idle(retroFired)
	return r.tools.calls()
}

// routineRuns は打った kanban routine run の名前。
func routineRuns(calls []string) []string {
	var out []string
	for _, c := range calls {
		if name, ok := strings.CutPrefix(c, "kanban routine run "); ok {
			out = append(out, name)
		}
	}
	return out
}

// idles は待ちに積んだ通知 idle。
func (r *idleRig) idles() []Notification {
	var out []Notification
	for _, n := range r.p.st.Pending {
		if n.Kind == "idle" {
			out = append(out, n)
		}
	}
	return out
}

// 受け入れ条件 1: 前の回も今の回も枠が空いていて、仕分けていない項目が 50 件以上あり、片付いていないトリアージが無いとき、
// kanban routine run triage を1回打つ。49 件のとき、片付いていないトリアージがあるとき、ルーチンが paused のときは打たない。
func TestIdleFiresTriage(t *testing.T) {
	for _, c := range []struct {
		name     string
		unsorted int
		paused   bool
		tasks    []ledger.Issue
		fire     bool
	}{
		{name: "50 件", unsorted: 50, fire: true},
		{name: "49 件", unsorted: 49},
		{name: "paused", unsorted: 80, paused: true},
		{name: "ルーチンのトリアージが残っている", unsorted: 80, tasks: []ledger.Issue{{"id": "T-0001", "title": "トリアージ", "status": "doing", "routine": "triage"}}},
		{name: "臨時のトリアージが残っている", unsorted: 80, tasks: []ledger.Issue{{"id": "T-0002", "title": "トリアージ: memory を減らす", "status": "todo"}}},
		{name: "トリアージで始まる直す課題は数えない", unsorted: 80, fire: true, tasks: []ledger.Issue{{"id": "T-0003", "title": "トリアージの集めを直す", "status": "doing", "repo": "github.com/example/app"}}},
		{name: "片付いたトリアージは数えない", unsorted: 80, fire: true, tasks: []ledger.Issue{{"id": "T-0004", "title": "トリアージ", "status": "done", "routine": "triage"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newIdleRig(t)
			extra := ""
			if c.paused {
				extra = "paused: true\n"
			}
			r.routine("triage", extra)
			r.unsorted = c.unsorted
			r.p.tasks = c.tasks
			if got := routineRuns(r.look(0, false)); len(got) != 0 {
				t.Fatalf("初めて見る回に撃った: %v", got)
			}
			got := routineRuns(r.look(15*time.Minute, false))
			if want := map[bool][]string{true: {"triage"}}[c.fire]; !slices.Equal(got, want) {
				t.Errorf("撃ったルーチン %v（%v のはず）", got, want)
			}
		})
	}
}

// 受け入れ条件 2: 前の回に枠が空いていなかったとき、今の回が空いていても、何も打たず、通知も積まない。
// 見る時刻の前・前の回から間が2倍を超えた回・初めて見る回・起動し直した最初の回も、控えるだけ。
func TestIdleNeedsTwoFreeLooks(t *testing.T) {
	r := newIdleRig(t)
	r.routine("triage", "")
	r.unsorted = 80
	ready := []string{"S-0001"}
	r.p.st.Ready = &ready
	r.p.tasks = []ledger.Issue{{"id": "S-0001", "title": "起動できる", "status": "todo"}}
	r.free = 0
	if c := r.look(0, false); len(c) != 0 {
		t.Fatalf("初めて見る回に打った: %v", c)
	}
	r.free = 2
	if c := r.look(15*time.Minute, false); len(c) != 0 || len(r.idles()) != 0 {
		t.Fatalf("前の回に枠が空いていなかったのに打った・積んだ: %v %v", c, r.idles())
	}
	if c := r.look(14*time.Minute, false); len(c) != 0 || len(r.idles()) != 0 {
		t.Fatalf("見る時刻の前に打った・積んだ: %v %v", c, r.idles())
	}
	if c := r.look(31*time.Minute+16*time.Minute, false); len(c) != 0 || len(r.idles()) != 0 {
		t.Fatalf("前の回から間が2倍を超えた回に打った・積んだ: %v %v", c, r.idles())
	}
	r.p.c.idleSeen = nil // dispatcher が起動し直した
	if c := r.look(15*time.Minute, false); len(c) != 0 || len(r.idles()) != 0 {
		t.Fatalf("起動し直した最初の回に打った・積んだ: %v %v", c, r.idles())
	}
	if got := routineRuns(r.look(15*time.Minute, false)); !slices.Equal(got, []string{"triage"}) || len(r.idles()) != 1 {
		t.Fatalf("2 回続けて空いた回に撃たない・積まない: %v %v", got, r.idles())
	}
	r.free = 0
	r.tools.reply("kanban routine run triage", "T-0092")
	r.p.st.Pending = nil
	if c := r.look(15*time.Minute, false); len(c) != 0 || len(r.idles()) != 0 {
		t.Fatalf("今の回に枠が空いていないのに打った・積んだ: %v %v", c, r.idles())
	}
}

// 受け入れ条件 3: ルーチンが足した課題を除いた片付いた担当の課題が retro_every の半分（25 なら 13）以上で、片付いていない振り返りが
// 無いとき、kanban routine run retro を1回打つ。ルーチンの課題だけで半分に届くとき、同じ巡で件数のきっかけが撃っていたとき、
// 振り返りの課題が残っているときは打たない。
func TestIdleFiresRetro(t *testing.T) {
	done := func(n int, routine string) ([]ledger.Issue, []ledger.Row) {
		var tasks []ledger.Issue
		var rows []ledger.Row
		at := "2026-11-22T23:00:00Z" // 偽の時計（2026-11-23T00:00Z から）の 1 時間前
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("S-%04d", i+1)
			if routine != "" {
				id = fmt.Sprintf("S-%04d", i+501)
			}
			t := ledger.Issue{"id": id, "title": "直す", "status": "done", "profile": "claude"}
			if routine != "" {
				t["routine"] = routine
			}
			tasks = append(tasks, t)
			rows = append(rows, ledger.Row{At: at, ID: id, Change: map[string]any{"status": "done"}})
		}
		return tasks, rows
	}
	for _, c := range []struct {
		name       string
		own, other int
		retroFired bool
		open       bool
		fire       bool
	}{
		{name: "13 件", own: 13, fire: true},
		{name: "12 件", own: 12},
		{name: "ルーチンの課題だけで 13 件", own: 0, other: 13},
		{name: "ルーチンの課題を足しても 12 件", own: 12, other: 5},
		{name: "同じ巡で件数のきっかけが撃った", own: 13, retroFired: true},
		{name: "振り返りが残っている", own: 13, open: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newIdleRig(t)
			r.routine("retro", "")
			own, rows := done(c.own, "")
			other, rows2 := done(c.other, "triage")
			r.p.tasks = append(own, other...)
			r.p.rows = append(rows, rows2...)
			if c.open {
				r.p.tasks = append(r.p.tasks, ledger.Issue{"id": "R-0001", "title": "振り返り", "status": "todo", "routine": "retro"})
			}
			r.look(0, false)
			got := routineRuns(r.look(15*time.Minute, c.retroFired))
			if want := map[bool][]string{true: {"retro"}}[c.fire]; !slices.Equal(got, want) {
				t.Errorf("撃ったルーチン %v（%v のはず）", got, want)
			}
		})
	}
}

// 受け入れ条件 4: idle_hours: 24 のルーチンは、最後に課題を足してから 24 時間以上で、空いているときに撃たれる。23 時間では撃たれない。
// 撃ち損ねの行だけが新しいときは撃たれる。1 度も足していなければ撃たれる。値が 1 以上の整数でないルーチンは撃たれず、ログに 1 行が出る
// （同じ理由が続く間は 1 行）。idle_hours の無いルーチン・paused・課題が残っているルーチンは撃たれない。
func TestIdleFiresHoursRoutine(t *testing.T) {
	// 偽の時計は 2026-11-23T00:00Z から。2 回目の look（15 分あと）が撃つ回
	fired := func(ago time.Duration) ledger.Row {
		at := time.Date(2026, 11, 23, 0, 15, 0, 0, time.UTC).Add(-ago).Format(time.RFC3339)
		return ledger.Row{At: at, By: "routine:check", Routine: "check", Fired: "S-0700"}
	}
	skipped := func(ago time.Duration) ledger.Row {
		at := time.Date(2026, 11, 23, 0, 15, 0, 0, time.UTC).Add(-ago).Format(time.RFC3339)
		return ledger.Row{At: at, By: "routine:check", Routine: "check"} // skipped: open（ledger.Row は skipped を読まない）
	}
	for _, c := range []struct {
		name  string
		extra string
		rows  []ledger.Row
		tasks []ledger.Issue
		fire  bool
		log   string
	}{
		{name: "24 時間", extra: "idle_hours: 24\n", rows: []ledger.Row{fired(24 * time.Hour)}, fire: true},
		{name: "23 時間", extra: "idle_hours: 24\n", rows: []ledger.Row{fired(23 * time.Hour)}},
		{name: "いちばん新しい足した行で数える", extra: "idle_hours: 24\n", rows: []ledger.Row{fired(23 * time.Hour), fired(48 * time.Hour)}},
		{name: "撃ち損ねの行だけが新しい", extra: "idle_hours: 24\n", rows: []ledger.Row{fired(30 * time.Hour), skipped(time.Hour)}, fire: true},
		{name: "1 度も足していない", extra: "idle_hours: 24\n", fire: true},
		{name: "ほかのルーチンの行は数えない", extra: "idle_hours: 24\n", rows: []ledger.Row{{At: "2026-11-23T00:00:00Z", By: "routine:other", Routine: "other", Fired: "S-0701"}}, fire: true},
		{name: "時間に直すとあふれるほど大きい", extra: "idle_hours: 3000000\n", rows: []ledger.Row{fired(time.Hour)}},
		{name: "idle_hours が無い", extra: ""},
		{name: "小数", extra: "idle_hours: 1.5\n", log: "ルーチン check の idle_hours が 1 以上の整数でない"},
		{name: "文字列", extra: "idle_hours: \"24\"\n", log: "ルーチン check の idle_hours が 1 以上の整数でない"},
		{name: "0", extra: "idle_hours: 0\n", log: "ルーチン check の idle_hours が 1 以上の整数でない"},
		{name: "paused", extra: "idle_hours: 24\npaused: true\n"},
		{name: "課題が残っている", extra: "idle_hours: 24\n", tasks: []ledger.Issue{{"id": "S-0700", "title": "点検", "status": "todo", "routine": "check"}}},
		{name: "片付いた課題は数えない", extra: "idle_hours: 24\n", tasks: []ledger.Issue{{"id": "S-0700", "title": "点検", "status": "done", "routine": "check"}}, fire: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newIdleRig(t)
			r.routine("check", c.extra)
			r.p.rows, r.p.tasks = c.rows, c.tasks
			r.look(0, false)
			got := routineRuns(r.look(15*time.Minute, false))
			if want := map[bool][]string{true: {"check"}}[c.fire]; !slices.Equal(got, want) {
				t.Errorf("撃ったルーチン %v（%v のはず）", got, want)
			}
			if c.fire && !strings.Contains(r.log.String(), "check のルーチンを撃った") {
				t.Errorf("撃ったログが無い: %s", r.log.String())
			}
			r.look(15*time.Minute, false)
			if n := strings.Count(r.log.String(), "idle_hours が 1 以上の整数でない"); c.log == "" && n != 0 || c.log != "" && n != 1 {
				t.Errorf("idle_hours の値のログが %d 行（%q を 1 行のはず）: %s", n, c.log, r.log.String())
			}
		})
	}
}

// idle_hours を持つルーチンは、トリアージ・振り返りのあとに名前の順で、空いた枠の数まで撃つ。トリアージと振り返りに
// idle_hours があっても、その線（決めたこと 7・8）のほかでは撃たない。
func TestIdleHoursRoutinesOrderAndBudget(t *testing.T) {
	for _, c := range []struct {
		free int
		want []string
	}{
		{1, []string{"triage"}},
		{2, []string{"triage", "a-check"}},
		{3, []string{"triage", "a-check", "b-check"}},
		{4, []string{"triage", "a-check", "b-check"}},
	} {
		r := newIdleRig(t)
		r.routine("triage", "idle_hours: 1\n")
		r.routine("retro", "idle_hours: 1\n") // 片付いた担当の課題が 0 件で、振り返りの線に届かない
		r.routine("b-check", "idle_hours: 24\n")
		r.routine("a-check", "idle_hours: 72\n")
		r.routine("plain", "")
		r.unsorted = 80
		r.free = c.free
		r.look(0, false)
		if got := routineRuns(r.look(15*time.Minute, false)); !slices.Equal(got, c.want) {
			t.Errorf("空いた枠 %d: 撃ったルーチン %v（%v のはず）", c.free, got, c.want)
		}
	}
	r := newIdleRig(t)
	r.routine("triage", "idle_hours: 1\n")
	r.unsorted = 10 // 線の 50 件に届かない
	r.look(0, false)
	if got := routineRuns(r.look(15*time.Minute, false)); len(got) != 0 {
		t.Errorf("idle_hours のあるトリアージを、線の外で撃った: %v", got)
	}
}

// kanban routine run が失敗を返しても、課題と履歴の行は書いたあと（state/routines.json の保存の失敗など）のことがある。
// 失敗した試みも空いた枠を使ったものとして数え、空いた枠より多く撃たない。
func TestIdleFailedRunUsesSlot(t *testing.T) {
	r := newIdleRig(t)
	r.routine("a-check", "idle_hours: 24\n")
	r.routine("b-check", "idle_hours: 24\n")
	os.Remove(r.tools.key("kanban routine run a-check") + ".out") // 答えが無いと偽の kanban は 1 で落ちる
	r.free = 1
	r.look(0, false)
	if got := routineRuns(r.look(15*time.Minute, false)); !slices.Equal(got, []string{"a-check"}) {
		t.Errorf("空いた枠 1 で、失敗した a-check のあとに撃った: %v（a-check だけのはず）", got)
	}
	if !strings.Contains(r.log.String(), "kanban routine run a-check が通らない") {
		t.Errorf("失敗のログが無い: %s", r.log.String())
	}
}

// 受け入れ条件 5: 1回に撃つルーチンの数は slots.free を超えない。kanban ready に担当の課題が残っていても、撃つ数は減らない。
func TestIdleFiresUpToFreeSlots(t *testing.T) {
	var tasks []ledger.Issue
	var rows []ledger.Row
	at := "2026-11-22T23:00:00Z" // 偽の時計（2026-11-23T00:00Z から）の 1 時間前
	for i := 0; i < 13; i++ {
		id := fmt.Sprintf("S-%04d", i+1)
		tasks = append(tasks, ledger.Issue{"id": id, "title": "直す", "status": "done", "profile": "claude"})
		rows = append(rows, ledger.Row{At: at, ID: id, Change: map[string]any{"status": "done"}})
	}
	ready := []string{"S-0101", "S-0102", "S-0103"}
	for _, id := range ready {
		tasks = append(tasks, ledger.Issue{"id": id, "title": "起動できる", "status": "todo", "profile": "claude"})
	}
	// retroFired は同じ巡で件数のきっかけが振り返りを撃ったか。撃った 1 つも空いた枠を使うので、空きで撃てる数から引く
	for _, c := range []struct {
		free       int
		retroFired bool
		want       []string
	}{
		{1, false, []string{"triage"}},
		{2, false, []string{"triage", "retro"}},
		{3, false, []string{"triage", "retro"}},
		{1, true, nil},
		{2, true, []string{"triage"}},
	} {
		r := newIdleRig(t)
		r.routine("triage", "")
		r.routine("retro", "")
		r.unsorted = 80
		r.free = c.free
		r.p.tasks, r.p.rows = tasks, rows
		rd := append([]string{}, ready...)
		r.p.st.Ready = &rd
		r.look(0, false)
		if got := routineRuns(r.look(15*time.Minute, c.retroFired)); !slices.Equal(got, c.want) {
			t.Errorf("空いた枠 %d・件数のきっかけが撃った %v・起動できる課題 3 件: 撃ったルーチン %v（%v のはず）", c.free, c.retroFired, got, c.want)
		}
	}
}

// 受け入れ条件 6（Go の側）: 前の回にも今の回にも起動できる課題の一覧に出ている課題を通知 idle に 1 回だけ入れる。
// もう起動された課題は入れない。届く前に次の idle を積むと、前の unstarted と fired が新しいほうに残る。
// 一覧から消えた id は知らせた控えから外す。
func TestIdleTellsUnstartedOnce(t *testing.T) {
	r := newIdleRig(t)
	r.routine("triage", "")
	set := func(ids ...string) {
		rd := append([]string{}, ids...)
		r.p.st.Ready = &rd
	}
	r.p.tasks = []ledger.Issue{
		{"id": "S-0001", "title": "a", "status": "todo"}, {"id": "S-0002", "title": "b", "status": "todo"},
		{"id": "S-0003", "title": "c", "status": "doing"}, {"id": "S-0004", "title": "d", "status": "todo"},
	}
	set("S-0001", "S-0002", "S-0003")
	r.look(0, false)
	set("S-0001", "S-0003", "S-0004")
	r.unsorted = 80
	r.look(15*time.Minute, false)
	got := r.idles()
	if len(got) != 1 {
		t.Fatalf("idle が %d 件（1 件のはず）: %v", len(got), r.p.st.Pending)
	}
	if u := idList(got[0].Detail["unstarted"]); !slices.Equal(u, []string{"S-0001"}) {
		t.Errorf("unstarted %v（前の回にも今の回にも出ていて、起動されていない S-0001 だけのはず）", u)
	}
	if f := idList(got[0].Detail["fired"]); !slices.Equal(f, []string{"triage"}) || got[0].Issue != "" || got[0].Detail["free"] != 2 {
		t.Errorf("fired・issue・free が違う: %+v", got[0])
	}
	if err := r.p.st.Save(r.p.l.Slug); err != nil {
		t.Fatalf("idle が schema に合わない: %v", err)
	}
	// 届く前の次の回: S-0001 はもう知らせた。S-0004 が新しく 2 回続けて出た。トリアージは残っているので撃たない
	r.p.tasks = append(r.p.tasks, ledger.Issue{"id": "T-0090", "title": "トリアージ", "status": "todo", "routine": "triage"})
	r.look(15*time.Minute, false)
	got = r.idles()
	if len(got) != 1 {
		t.Fatalf("idle が %d 件（置き換えで 1 件のはず）", len(got))
	}
	if u, f := idList(got[0].Detail["unstarted"]), idList(got[0].Detail["fired"]); !slices.Equal(u, []string{"S-0001", "S-0004"}) || !slices.Equal(f, []string{"triage"}) {
		t.Errorf("前の idle の unstarted・fired が残らない: unstarted %v fired %v", u, f)
	}
	// 届いたあと: 同じ id では積まない
	r.p.st.Pending = nil
	r.look(15*time.Minute, false)
	if len(r.idles()) != 0 {
		t.Errorf("知らせた id で、もう一度積んだ: %v", r.idles())
	}
	// 一覧から消えた id は控えから外す。また出て 2 回続けば、もう一度知らせる
	set("S-0004")
	r.look(15*time.Minute, false)
	if slices.Contains(r.p.st.Idle.Told, "S-0001") {
		t.Errorf("一覧から消えた id を控えに残した: %v", r.p.st.Idle.Told)
	}
	set("S-0001", "S-0004")
	r.look(15*time.Minute, false)
	set("S-0001", "S-0004")
	r.look(15*time.Minute, false)
	if got := r.idles(); len(got) != 1 || !slices.Equal(idList(got[0].Detail["unstarted"]), []string{"S-0001"}) {
		t.Errorf("一覧に戻って 2 回続いた id を知らせない: %v", got)
	}
}

// 受け入れ条件 7（Go の側）: kanban lint が problem を出すとき、通知 idle の lint に件数が入る。同じ problem のままの次の回には積まない。
// problem が 0 件に戻ったら印を消し、また出たら知らせる。
func TestIdleTellsLintOnce(t *testing.T) {
	r := newIdleRig(t)
	bad := `{"ok":false,"problems":["S-0001: no title","S-0002: parent S-9, which doesn't exist"]}`
	r.tools.reply("kanban lint --json", bad)
	r.look(0, false)
	r.look(15*time.Minute, false)
	if got := r.idles(); len(got) != 1 || got[0].Detail["lint"] != 2 {
		t.Fatalf("lint の件数が idle に入らない: %v", got)
	}
	r.p.st.Pending = nil
	r.look(15*time.Minute, false)
	if got := r.idles(); len(got) != 0 {
		t.Fatalf("同じ problem のままで、もう一度積んだ: %v", got)
	}
	r.tools.reply("kanban lint --json", `{"ok":true,"problems":[]}`)
	r.look(15*time.Minute, false)
	if r.p.st.Idle.Lint != "" || len(r.idles()) != 0 {
		t.Fatalf("problem が 0 件に戻ったのに印を残した・積んだ: %q %v", r.p.st.Idle.Lint, r.idles())
	}
	r.tools.reply("kanban lint --json", bad)
	r.look(15*time.Minute, false)
	if got := r.idles(); len(got) != 1 || got[0].Detail["lint"] != 2 {
		t.Errorf("0 件に戻ったあと、また出た problem を知らせない: %v", got)
	}
}

// 届く前の idle に lint の件数があるとき、同じ problem のままの次の回が別の理由で idle を積んでも、件数を残す
// （印は「知らせた」として控えに残るので、ここで落とすと problem の並びが変わるまで二度と知らせない）。
func TestIdleKeepsLintOfPendingIdle(t *testing.T) {
	r := newIdleRig(t)
	r.tools.reply("kanban lint --json", `{"ok":false,"problems":["S-0001: no title","S-0002: parent S-9, which doesn't exist","S-0003: no title"]}`)
	r.p.tasks = []ledger.Issue{{"id": "S-0001", "title": "a", "status": "todo"}}
	r.look(0, false)
	r.look(15*time.Minute, false)
	if got := r.idles(); len(got) != 1 || got[0].Detail["lint"] != 3 {
		t.Fatalf("前提: lint の件数が idle に入らない: %v", got)
	}
	// 控えを保存して読み直す（読み直した detail の数は float64 になる。次の回は別のプロセスのことがある）
	if err := r.p.st.Save(r.p.l.Slug); err != nil {
		t.Fatal(err)
	}
	st, _, err := LoadState(r.p.l.Slug)
	if err != nil {
		t.Fatal(err)
	}
	r.p.st = st
	rd := []string{"S-0001"}
	r.p.st.Ready = &rd
	r.look(15*time.Minute, false)
	r.look(15*time.Minute, false)
	got := r.idles()
	if len(got) != 1 || !slices.Equal(idList(got[0].Detail["unstarted"]), []string{"S-0001"}) {
		t.Fatalf("前提: 次の回が unstarted で idle を積まない: %v", got)
	}
	if got[0].Detail["lint"] != 3 {
		t.Errorf("届く前の idle の lint の件数が、次の idle で消えた: %v", got[0].Detail)
	}
}

// 受け入れ条件 8: Chief of Staff が留守の回と、chief_of_staff.idle_minutes が 0 のとき、空きを見る段は何も打たない。
// 留守の間は前の回の結果を消すので、戻った最初の回も控えるだけ。
func TestIdleSkipsAwayAndOff(t *testing.T) {
	r := newIdleRig(t)
	r.routine("triage", "")
	r.unsorted = 80
	r.look(0, false)
	r.p.away = true
	if c := r.look(15*time.Minute, false); len(c) != 0 || len(r.idles()) != 0 {
		t.Fatalf("留守の回に打った・積んだ: %v %v", c, r.idles())
	}
	r.p.away = false
	if c := r.look(15*time.Minute, false); len(c) != 0 {
		t.Fatalf("留守から戻った最初の回に打った: %v", c)
	}
	if got := routineRuns(r.look(15*time.Minute, false)); !slices.Equal(got, []string{"triage"}) {
		t.Fatalf("戻ってから 2 回続けて空いた回に撃たない: %v", got)
	}

	// idle_minutes: 0（オーナーの commit の project.json）
	r2 := newIdleRig(t)
	r2.routine("triage", "")
	r2.unsorted = 80
	ws := r2.p.c.WS
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", ws}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	rel := filepath.Join("projects", "demo", "project.json")
	os.MkdirAll(filepath.Join(ws, "projects", "demo"), 0o755)
	os.WriteFile(filepath.Join(ws, rel), []byte(`{"version":1,"project":{"title":"Demo","prefix":"S"},"repos":[],"chief_of_staff":{"idle_minutes":0}}`), 0o644)
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte("owner = \"owner@example.com\"\n"), 0o644)
	git("init", "-q")
	git("add", rel)
	git("-c", "user.name=o", "-c", "user.email=owner@example.com", "commit", "-q", "-m", "project")
	for i, d := range []time.Duration{0, 15 * time.Minute, 15 * time.Minute} {
		if c := r2.look(d, false); len(c) != 0 || len(r2.idles()) != 0 {
			t.Fatalf("idle_minutes が 0 の %d 回目に打った・積んだ: %v %v", i+1, c, r2.idles())
		}
	}
	if r2.p.st.Idle != nil {
		t.Errorf("idle_minutes が 0 なのに前の回の結果を控えた: %+v", r2.p.st.Idle)
	}
	if strings.Contains(r2.log.String(), "idle_minutes を読めない") {
		t.Errorf("オーナーの project.json を読めないとログした: %s", r2.log.String())
	}
}

// 枠を数えられない回はログに1行（同じ理由の間は1行）で飛ばし、前の回の結果を消す。
func TestIdleSlotsUnreadable(t *testing.T) {
	r := newIdleRig(t)
	r.routine("triage", "")
	r.unsorted = 80
	r.look(0, false)
	slotsOf = func(string, time.Time) (int, error) { return 0, os.ErrPermission }
	r.look(15*time.Minute, false)
	r.look(15*time.Minute, false)
	if n := strings.Count(r.log.String(), "枠を数えられない"); n != 1 {
		t.Errorf("枠を数えられないログが %d 行（1 行のはず）: %s", n, r.log.String())
	}
	slotsOf = func(string, time.Time) (int, error) { return r.free, nil }
	if c := r.look(15*time.Minute, false); len(c) != 0 {
		t.Errorf("数えられない回のあとの最初の回に打った（前の回の結果を消していない）: %v", c)
	}
}
