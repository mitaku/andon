// gopkgs.go: 機能ごとに、変えたら動かす Go の package の置き場（repo からの相対のフォルダ）を出す。
//
//	go run gopkgs.go <repo> <commit>… < 機能とパス（1行1つ。「<機能><TAB><パス>」）
//
// パスの書き方は features.sh と同じ（「<dir>/」はその下の全部、「*」は同じ段の中だけ、それ以外はそのパス）。
// パスが名指す Go のファイルの package（同じフォルダ）と、そのフォルダの下の package を起点にし、
// 起点が repo の中で推移的に import する package を足す。出すのは「<機能><TAB><フォルダ>」の行。
// import は commit ごとの木（git の中身）から読み、全部の commit の分を合わせる（前と後のどちらかで import していれば辿る）。
// commit に空の文字列を渡すと、作業ツリー（git ls-files）を読む。
// _test.go・testdata/・「.」か「_」で始まるフォルダは go と同じく読まない。build の制約は見ない（どの OS の分も辿る側に倒す）。
// 字句に分けられないファイルは import を持たないものとして読む（同じフォルダの package には数える）。
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

type tree struct {
	files map[string]bool            // Go のファイル（テストを除く）
	deps  map[string]map[string]bool // フォルダ → import するフォルダ（repo の中だけ）
}

func gofile(p string) bool {
	if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
		return false
	}
	for _, seg := range strings.Split(path.Dir(p), "/") {
		if seg == "testdata" || strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "_") {
			return false
		}
	}
	return true
}

func git(repo string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
}

func read(repo, rev, p string) []byte {
	if rev == "" {
		b, _ := os.ReadFile(path.Join(repo, p))
		return b
	}
	b, _ := git(repo, "show", rev+":"+p)
	return b
}

func module(repo, rev string) string {
	for _, l := range strings.Split(string(read(repo, rev, "go.mod")), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[0] == "module" {
			if m, err := strconv.Unquote(f[1]); err == nil {
				return m
			}
			return f[1]
		}
	}
	return ""
}

func load(repo, rev string, t *tree) error {
	var out []byte
	var err error
	if rev == "" {
		out, err = git(repo, "-c", "core.quotepath=off", "ls-files", "-co", "--exclude-standard")
	} else {
		out, err = git(repo, "-c", "core.quotepath=off", "ls-tree", "-r", "--name-only", rev)
	}
	if err != nil {
		return fmt.Errorf("%s の木を読めない: %v", rev, err)
	}
	mod := module(repo, rev)
	for _, p := range strings.Split(string(bytes.TrimSpace(out)), "\n") {
		if !gofile(p) {
			continue
		}
		t.files[p] = true
		dir := path.Dir(p)
		if t.deps[dir] == nil {
			t.deps[dir] = map[string]bool{}
		}
		if mod == "" {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, read(repo, rev, p), parser.ImportsOnly)
		if err != nil || f == nil {
			continue
		}
		for _, im := range f.Imports {
			ip, err := strconv.Unquote(im.Path.Value)
			if err == nil && strings.HasPrefix(ip, mod+"/") {
				t.deps[dir][strings.TrimPrefix(ip, mod+"/")] = true
			}
		}
	}
	return nil
}

// hits は features.sh の hits と同じ読み方。
func hits(part, p string) bool {
	switch {
	case strings.HasSuffix(part, "/"):
		return strings.HasPrefix(p, part)
	case strings.Contains(part, "*"):
		ok, _ := path.Match(part, p)
		return ok && path.Dir(p) == path.Dir(part)
	default:
		return p == part
	}
}

func roots(t *tree, part string) []string {
	var got []string
	switch {
	case strings.HasSuffix(part, "/"):
		for dir := range t.deps {
			if dir+"/" == part || strings.HasPrefix(dir+"/", part) {
				got = append(got, dir)
			}
		}
	case strings.Contains(part, "*"):
		for p := range t.files {
			if hits(part, p) {
				got = append(got, path.Dir(p))
			}
		}
	case strings.HasSuffix(part, ".go") && !strings.HasSuffix(part, "_test.go"):
		got = append(got, path.Dir(part)) // 消えたファイルでも、置き場の package は残る
	}
	return got
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "使い方: go run gopkgs.go <repo> <commit>… < 機能とパス（1行1つ。<機能><TAB><パス>）")
		os.Exit(2)
	}
	repo := os.Args[1]
	t := &tree{files: map[string]bool{}, deps: map[string]map[string]bool{}}
	for _, rev := range os.Args[2:] {
		if err := load(repo, rev, t); err != nil {
			fmt.Fprintln(os.Stderr, "gopkgs:", err)
			os.Exit(1)
		}
	}
	reach := map[string]map[string]bool{}
	var names []string
	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		name, part, ok := strings.Cut(in.Text(), "\t")
		if !ok || part == "" {
			continue
		}
		if reach[name] == nil {
			reach[name] = map[string]bool{}
			names = append(names, name)
		}
		stack := roots(t, part)
		for len(stack) > 0 {
			dir := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if reach[name][dir] {
				continue // 循環と重なりで止まる
			}
			reach[name][dir] = true
			for d := range t.deps[dir] {
				stack = append(stack, d)
			}
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "gopkgs:", err)
		os.Exit(1)
	}
	for _, name := range names {
		dirs := make([]string, 0, len(reach[name]))
		for d := range reach[name] {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			fmt.Printf("%s\t%s\n", name, d)
		}
	}
}
