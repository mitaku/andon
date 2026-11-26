package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
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

// A11: 3 以上の終了コードは、葉のパッケージに名前が exit で始まる定数（exitFirst = 3 など）で書かれる。その定数の数と、表の exits の
// 3 以上の数が葉ごとに揃う。定数を足して表を直さないと、--help に出ない終了コードになるので落ちる（help は表から作るので、表と help の
// 比べだけではコードとのずれを見ない）
func TestExitsMatchCode(t *testing.T) {
	owner := map[string]string{ // 3 以上の終了コードの定数を持つファイル → その数を返す葉
		"internal/chiefofstaff/chiefofstaff.go":   "open",
		"internal/chiefengineer/chiefengineer.go": "design start",
		"internal/issue/start.go":                 "issue start",
		"internal/issue/impact.go":                "issue impact",
		"internal/gate/gate.go":                   "gate",
		"internal/triage/triage.go":               "triage collect",
	}
	root := filepath.Join("..", "..")
	found := map[string][]int{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			for _, decl := range f.Decls {
				g, ok := decl.(*ast.GenDecl)
				if !ok || g.Tok != token.CONST {
					continue
				}
				for _, sp := range g.Specs {
					vs := sp.(*ast.ValueSpec)
					for i, n := range vs.Names {
						if !strings.HasPrefix(strings.ToLower(n.Name), "exit") || i >= len(vs.Values) {
							continue
						}
						lit, ok := vs.Values[i].(*ast.BasicLit)
						if !ok || lit.Kind != token.INT {
							t.Errorf("%s の %s: 終了コードの定数は数で書く（式や iota だと突き合わせられない）", rel, n.Name)
							continue
						}
						v, _ := strconv.Atoi(lit.Value)
						if v < 3 {
							continue
						}
						leaf, ok := owner[rel]
						if !ok {
							t.Errorf("%s の %s = %d: 3 以上の終了コードの定数を持つファイルが、この検査の owner の表に無い", rel, n.Name, v)
							continue
						}
						found[leaf] = append(found[leaf], v)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) != len(owner) {
		t.Errorf("owner の表のファイルに 3 以上の終了コードの定数が無いものがある（拾ったのは %v）", found)
	}
	for _, l := range leaves {
		var want []int
		for _, e := range l.exits {
			if e.code >= 3 {
				want = append(want, e.code)
			}
		}
		got := found[l.name]
		slices.Sort(got)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: コードの 3 以上の終了コード %v と、表の exits の 3 以上 %v が違う", l.name, got, want)
		}
	}
}
