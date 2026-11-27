package cli

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// cell は一時のセルに slugs のプロジェクト（projects/<slug>/kanban/config.yaml）を作り、ANDON_WORKSPACE をそこへ向ける。
// KANBAN_ROOT は空にする（振り分けが書いた値も、検査の終わりに戻る）。kanban の CLI は root を返さない形に差し替える。
func cell(t *testing.T, slugs ...string) string {
	t.Helper()
	ws := t.TempDir()
	for _, s := range slugs {
		if err := os.MkdirAll(filepath.Join(ws, "projects", s, "kanban"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "projects", s, "kanban", "config.yaml"), []byte("version: 3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ANDON_WORKSPACE", ws)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("HOME", t.TempDir())
	saved := kanbanRoot
	kanbanRoot = func() (string, bool) { return "", true }
	t.Cleanup(func() { kanbanRoot = saved })
	return ws
}

// spyLeaves は葉の中身を、打たれた引数と、そのときの KANBAN_ROOT を控えるだけのものに差し替える。
func spyLeaves(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	saved := slices.Clone(leaves)
	for i := range leaves {
		name := leaves[i].name
		leaves[i].run = func(args []string, _ io.Reader, _, _ io.Writer) int {
			calls = append(calls, strings.Join(append(append([]string{name}, args...), "@"+os.Getenv("KANBAN_ROOT")), " "))
			return 0
		}
	}
	t.Cleanup(func() { copy(leaves, saved) })
	return &calls
}

func projectLeaves() []leaf {
	var out []leaf
	for _, l := range leaves {
		if l.project {
			out = append(out, l)
		}
	}
	return out
}

// A11: プロジェクトが決まらないとき、プロジェクトを読む葉はどれも終了コード 2 を返し、セルのプロジェクトの一覧と
// --project を付ける1行を出す。葉は打たない。フックが打つ葉とプロジェクトが無くてよい葉（publish・tray status。ADR-0064 決めたこと 8）は
// 2 を返さずに葉に渡し、省くと全部を回る dispatcher は決めない。
func TestProjectUndecided(t *testing.T) {
	cell(t, "a", "b")
	calls := spyLeaves(t)
	if len(projectLeaves()) < 30 {
		t.Fatalf("プロジェクトを読む葉が %d しかない（表の欄が読めていない）", len(projectLeaves()))
	}
	for _, l := range projectLeaves() {
		*calls = nil
		rc, out, errs := run(l.words()...)
		switch {
		case l.hook || l.every || l.optional:
			if rc != 0 || len(*calls) != 1 {
				t.Errorf("%s: フックか全部を回る葉かプロジェクトが無くてよい葉なのに止めた: rc=%d calls=%v err=%q", l.name, rc, *calls, errs)
			}
		case rc != 2 || out != "" || len(*calls) != 0:
			t.Errorf("%s: rc=%d out=%q calls=%v（2 で葉を打たずに断る）", l.name, rc, out, *calls)
		case !strings.Contains(errs, "プロジェクトが決まらない") || !strings.Contains(errs, "セルのプロジェクト: a・b") ||
			!strings.Contains(errs, "andon "+l.name+" --project <slug>"):
			t.Errorf("%s: 一覧と --project の1行が無い: %q", l.name, errs)
		}
	}
	// プロジェクトが無いセルでも 2 で、立てる葉を言う
	cell(t)
	if rc, _, errs := run("issue", "show", "S-0001"); rc != 2 || !strings.Contains(errs, "andon project create") {
		t.Errorf("プロジェクトが無いセル: rc=%d err=%q", rc, errs)
	}
}

// A5: プロジェクトを読む葉の全部が --project を受ける。動詞のあとのどこでも、-- の前で受け、葉が自分で読まないものは外して
// KANBAN_ROOT に渡す。無いプロジェクトは 2 で断る。
func TestProjectFlag(t *testing.T) {
	ws := cell(t, "a", "b")
	calls := spyLeaves(t)
	rootA := filepath.Join(ws, "projects", "a")
	for _, l := range projectLeaves() {
		*calls = nil
		t.Setenv("KANBAN_ROOT", "")
		pos := []string{"S-0001"}
		if len(l.words()) == 1 {
			pos = nil // 1語の葉は2語目を動詞に読む
		}
		rc, _, errs := run(slices.Concat(l.words(), pos, []string{"--project", "a"})...)
		want := strings.Join(slices.Concat([]string{l.name}, pos, []string{"@" + rootA}), " ")
		if l.ownFlag {
			want = strings.Join(slices.Concat([]string{l.name}, pos, []string{"--project", "a", "@"}), " ")
		}
		if rc != 0 || len(*calls) != 1 || (*calls)[0] != want {
			t.Errorf("%s --project a: rc=%d calls=%v want=%q err=%q", l.name, rc, *calls, want, errs)
		}
		*calls = nil
		rc, _, errs = run(append(l.words(), "--project", "nosuch")...)
		if l.hook {
			if rc != 0 || len(*calls) != 1 {
				t.Errorf("%s --project nosuch: フックなのに止めた: rc=%d calls=%v", l.name, rc, *calls)
			}
		} else if rc != 2 || len(*calls) != 0 || !strings.Contains(errs, "プロジェクト nosuch は無い") || !strings.Contains(errs, "a・b") {
			t.Errorf("%s --project nosuch: rc=%d calls=%v err=%q", l.name, rc, *calls, errs)
		}
	}
	// 動詞の直後でも、-- のあとは葉の引数（issue record の command の中の --project は受けない）
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"issue", "show", "--project", "b", "S-0002"}, "issue show S-0002 @" + filepath.Join(ws, "projects", "b")},
		{[]string{"issue", "record", "S-0002", "--project", "a", "--", "go", "test", "--project", "x"}, "issue record S-0002 -- go test --project x @" + rootA},
	} {
		*calls = nil
		t.Setenv("KANBAN_ROOT", "")
		if rc, _, errs := run(c.args...); rc != 0 || len(*calls) != 1 || (*calls)[0] != c.want {
			t.Errorf("%v: rc=%d calls=%v want=%q err=%q", c.args, rc, *calls, c.want, errs)
		}
	}
	// -- のあとの --project だけでは決まらない。値の無い --project は 2
	*calls = nil
	t.Setenv("KANBAN_ROOT", "")
	if rc, _, _ := run("issue", "record", "S-0002", "--", "go", "--project", "a"); rc != 2 || len(*calls) != 0 {
		t.Errorf("-- のあとの --project を受けた: rc=%d calls=%v", rc, *calls)
	}
	if rc, _, errs := run("issue", "show", "S-0002", "--project"); rc != 2 || !strings.Contains(errs, "--project に slug が要る") {
		t.Errorf("値の無い --project: rc=%d err=%q", rc, errs)
	}
	// --project は KANBAN_ROOT に勝つ
	*calls = nil
	t.Setenv("KANBAN_ROOT", rootA)
	if rc, _, _ := run("issue", "show", "S-0002", "--project", "b"); rc != 0 || len(*calls) != 1 || !strings.HasSuffix((*calls)[0], "@"+filepath.Join(ws, "projects", "b")) {
		t.Errorf("--project と KANBAN_ROOT: rc=%d calls=%v", rc, *calls)
	}
}

// A5: --project を省くと、KANBAN_ROOT、今いるフォルダ（kanban の CLI が見つけるもの。1つだけのプロジェクトも kanban が見つける）の順。
func TestProjectOmitted(t *testing.T) {
	ws := cell(t, "a", "b")
	calls := spyLeaves(t)
	rootB := filepath.Join(ws, "projects", "b")
	t.Setenv("KANBAN_ROOT", rootB)
	if rc, _, _ := run("issue", "show", "S-0002"); rc != 0 || len(*calls) != 1 || (*calls)[0] != "issue show S-0002 @"+rootB {
		t.Errorf("KANBAN_ROOT: rc=%d calls=%v", rc, *calls)
	}
	t.Setenv("KANBAN_ROOT", filepath.Join(ws, "projects", "nosuch"))
	if rc, _, errs := run("issue", "show", "S-0002"); rc != 2 || !strings.Contains(errs, "KANBAN_ROOT") {
		t.Errorf("kanban の無い KANBAN_ROOT: rc=%d err=%q", rc, errs)
	}
	t.Setenv("KANBAN_ROOT", "")
	*calls = nil
	kanbanRoot = func() (string, bool) { return rootB, true }
	if rc, _, _ := run("waiting", "list"); rc != 0 || len(*calls) != 1 || (*calls)[0] != "waiting list @" {
		t.Errorf("今いるフォルダ: rc=%d calls=%v", rc, *calls)
	}
	// kanban の CLI が無ければ、決めずに葉に渡す（葉が kanban が無いと言う）
	*calls = nil
	kanbanRoot = func() (string, bool) { return "", false }
	if rc, _, _ := run("issue", "show", "S-0002"); rc != 0 || len(*calls) != 1 {
		t.Errorf("kanban の CLI が無い: rc=%d calls=%v", rc, *calls)
	}
	// projects/<slug>/ の下は、kanban が無くても今いるフォルダで決まる（委譲と信号の葉は project.json だけを読む）。
	// その外で kanban が見つからなければ決まらない
	kanbanRoot = func() (string, bool) { return "", true }
	only := filepath.Join(ws, "projects", "c", "notes")
	if err := os.MkdirAll(only, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(only)
	*calls = nil
	if rc, _, errs := run("signal", "collect"); rc != 0 || len(*calls) != 1 || (*calls)[0] != "signal collect @" {
		t.Errorf("projects/c/notes: rc=%d calls=%v err=%q", rc, *calls, errs)
	}
	t.Chdir(ws)
	*calls = nil
	if rc, _, _ := run("signal", "collect"); rc != 2 || len(*calls) != 0 {
		t.Errorf("セルの根: rc=%d calls=%v", rc, *calls)
	}
	t.Chdir(filepath.Join(ws, "projects"))
	if rc, _, _ := run("signal", "collect"); rc != 2 || len(*calls) != 0 {
		t.Errorf("projects/ そのもの: rc=%d calls=%v", rc, *calls)
	}
}

// 欄の付け間違いは、期待を同じ欄から作る検査では見えない。欄ごとの葉をここに書き切って突き合わせる。
// hook を外すと、フックの合図の終了コード 2 を振り分けが返す（決めたこと 15 の例外が破れる）。ownFlag を誤って付けると、
// 葉が知らない --project を渡して usage で落ちる。外すと、葉が読む --project を振り分けが消す。
func TestProjectColumns(t *testing.T) {
	want := map[string][]string{
		"hook":  {"dispatcher drain", "dispatcher rewake", "dispatcher wait", "issue stop-check"},
		"every": {"dispatcher run"},
		"ownFlag": {"delegation grant", "delegation narrow", "delegation revoke", "delegation show", "dispatcher drain",
			"dispatcher rewake", "dispatcher run", "dispatcher wait", "issue board", "issue history", "signal collect", "signal mark-delivered",
			"waiting answer", "waiting list"},
	}
	got := map[string][]string{}
	for _, l := range leaves {
		for col, on := range map[string]bool{"hook": l.hook, "every": l.every, "ownFlag": l.ownFlag} {
			if on {
				got[col] = append(got[col], l.name)
			}
			if on && !l.project {
				t.Errorf("%s: %s なのにプロジェクトを読む葉でない", l.name, col)
			}
		}
	}
	for col, w := range want {
		g := slices.Sorted(slices.Values(got[col]))
		if !slices.Equal(g, w) {
			t.Errorf("%s の葉: %v（%v のはず）", col, g, w)
		}
	}
}

// --help はプロジェクトを決める前に出し、プロジェクトを読む葉の --help は --project の行を持つ。
func TestProjectHelp(t *testing.T) {
	cell(t, "a", "b")
	calls := spyLeaves(t)
	for _, l := range leaves {
		rc, out, _ := run(append(l.words(), "--help")...)
		if rc != 0 || strings.Contains(out, "\nプロジェクト: --project <slug>") != l.project {
			t.Errorf("%s --help: rc=%d project=%v out=%q", l.name, rc, l.project, out)
		}
	}
	if len(*calls) > 0 {
		t.Errorf("--help で葉を打った: %v", *calls)
	}
}
