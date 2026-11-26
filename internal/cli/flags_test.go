package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A6: JSON を出せる葉は表の json の欄で決まり、どれも --json を受ける（決めたこと 13）。いつも JSON を出す葉（ADR-0053 事実 8 の 9 つ）は
// 葉が --json を読まないので、振り分けが -- より前の --json を外して渡す。--json を読む葉には、そのまま渡す。
func TestJSONLeaves(t *testing.T) {
	want := map[jsonForm][]string{
		jsonAlways: {"gate", "issue impact", "issue merge", "issue settle", "issue show", "process json", "retro collect", "triage collect", "version"},
		jsonFlag:   {"delegation show", "issue board", "issue history", "issue trace", "metrics show", "waiting list"},
		jsonDryRun: {"issue resume", "issue start"},
	}
	got := map[jsonForm][]string{}
	for _, l := range leaves {
		if l.json != noJSON {
			got[l.json] = append(got[l.json], l.name)
		}
	}
	for f, names := range want {
		slices.Sort(got[f])
		if !slices.Equal(got[f], names) {
			t.Errorf("json の欄 %d の葉: got %v want %v", f, got[f], names)
		}
	}

	ws := cell(t, "a")
	t.Setenv("KANBAN_ROOT", filepath.Join(ws, "projects", "a"))
	calls := spyLeaves(t)
	for _, l := range leaves {
		if l.json == noJSON {
			continue
		}
		*calls = nil
		// 1語の葉（version）は、引数を先に置くと動詞と読まれるので、--json を先に置く
		args := append(l.words(), "--json", "S-0001", "--", "--json")
		if rc, _, errs := run(args...); rc != 0 || len(*calls) != 1 {
			t.Errorf("%s --json: rc=%d err=%q calls=%v", l.name, rc, errs, *calls)
			continue
		}
		pass := l.name + " --json S-0001 -- --json"
		if l.json == jsonAlways {
			pass = l.name + " S-0001 -- --json"
		}
		if got := strings.TrimSuffix((*calls)[0], " @"+filepath.Join(ws, "projects", "a")); got != pass {
			t.Errorf("%s に渡した引数: got %q want %q", l.name, got, pass)
		}
		_, help, _ := run(append(l.words(), "--help")...)
		if !strings.Contains(help, "\nJSON: "+jsonText[l.json]+"\n") {
			t.Errorf("%s --help に JSON の行が無い:\n%s", l.name, help)
		}
	}
	// JSON を出せない葉の --json は外さない（葉が使い方の誤りとして断る）
	*calls = nil
	if rc, _, _ := run("issue", "finish", "S-0001", "--json"); rc != 0 || len(*calls) != 1 || !strings.HasPrefix((*calls)[0], "issue finish S-0001 --json @") {
		t.Errorf("JSON を出せない葉の --json: rc=%d calls=%v", rc, *calls)
	}
	if _, help, _ := run("issue", "finish", "--help"); strings.Contains(help, "\nJSON: ") {
		t.Errorf("JSON を出せない葉の --help に JSON の行がある:\n%s", help)
	}
}

// A11: 3 以上の終了コードを使う葉は、--help にその数と意味が出る（決めたこと 15）。表の exits は、葉のパッケージが返す数と
// 突き合わせて書いたもの（open・design start・issue start の 3、issue impact・triage collect の 4、gate の 3・4・5）。
// schema check の 2 は「検査できない」で、使い方の誤りと別の意味を持つので、同じく help に出す。
func TestExitsInHelp(t *testing.T) {
	want := map[string][]int{
		"open": {3}, "design start": {3}, "issue start": {3}, "issue impact": {4}, "triage collect": {4},
		"gate": {1, 3, 4, 5}, "schema check": {1, 2},
	}
	got := map[string][]int{}
	for _, l := range leaves {
		for _, e := range l.exits {
			got[l.name] = append(got[l.name], e.code)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("表の終了コード: got %v want %v", got, want)
	}
	for _, l := range leaves {
		_, help, _ := run(append(l.words(), "--help")...)
		line := ""
		for _, s := range strings.Split(help, "\n") {
			if strings.HasPrefix(s, "終了コード: ") {
				line = s
			}
		}
		for _, e := range l.exits {
			if !strings.Contains(line, fmt.Sprintf("%d %s", e.code, e.means)) {
				t.Errorf("%s --help の終了コードの行に %d %s が無い: %q", l.name, e.code, e.means, line)
			}
		}
		if len(l.exits) == 0 && line != "終了コード: 0 成功・1 失敗か断り・2 使い方の誤り" {
			t.Errorf("%s --help の終了コードの行: %q", l.name, line)
		}
	}
}
