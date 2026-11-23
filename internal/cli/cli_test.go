package cli

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := Run(args, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

var wordRe = regexp.MustCompile(`^[a-z][a-z-]*$`)

// stubLeaves は表の葉の中身を、打たれた名前と引数を控えるだけのものに差し替える。表を回す検査は葉を本物で打たない
// （引数を取る葉 chief-engineer・init などは、本物ではプロジェクトに課題を足し、エージェントを起動する）。
func stubLeaves(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	saved := slices.Clone(leaves)
	for i := range leaves {
		name := leaves[i].name
		leaves[i].run = func(args []string, _ io.Reader, stdout, _ io.Writer) int {
			calls = append(calls, strings.Join(append([]string{name}, args...), " "))
			fmt.Fprintln(stdout, "stub "+name)
			return 0
		}
	}
	t.Cleanup(func() { copy(leaves, saved) })
	return &calls
}

// shapeErrors は葉の形の決まり（ADR-0053 決めたこと 5）から外れた葉を返す: 2語であること、1語は open・help・version・boot
// だけ、1語の葉を名詞にしないこと。名前を変えるまでの今の名前（renames）だけは外れていてよい。
func shapeErrors(ls []leaf) []string {
	var errs []string
	single := map[string]bool{}
	for _, l := range ls {
		if len(l.words()) == 1 {
			single[l.name] = true
		}
	}
	seen := map[string]bool{}
	for _, l := range ls {
		w := strings.Fields(l.name)
		_, pending := renames[l.name]
		switch {
		case seen[l.name]:
			errs = append(errs, l.name+": 2回載っている")
		case len(w) == 0 || l.name != strings.Join(w, " ") || slices.ContainsFunc(w, func(s string) bool { return !wordRe.MatchString(s) }):
			errs = append(errs, l.name+": 名前の形が違う")
		case len(w) > 2:
			errs = append(errs, l.name+": 3語以上")
		case len(w) == 1 && !oneWord[l.name] && !pending:
			errs = append(errs, l.name+": 1語で打てるのは open・help・version・boot だけ")
		case len(w) == 2 && single[w[0]] && !pending:
			if _, nounPending := renames[w[0]]; !nounPending {
				errs = append(errs, l.name+": 1語の葉を名詞にしている")
			}
		}
		seen[l.name] = true
	}
	return errs
}

// A1: 表の全部の葉が2語で、1語は open・help・version・boot だけ（名前を変えるまでの今の名前を除く）。
func TestLeafShape(t *testing.T) {
	if errs := shapeErrors(leaves); len(errs) > 0 {
		t.Errorf("葉の形の決まりから外れる:\n%s", strings.Join(errs, "\n"))
	}
	for old, neu := range renames {
		if _, ok := find(old); !ok {
			t.Errorf("renames の %s が表に無い（例外の表は今の名前だけを持つ）", old)
		}
		if w := strings.Fields(neu); len(w) > 2 || (len(w) == 1 && !oneWord[neu]) {
			t.Errorf("renames の新しい名前 %s が決まりの形でない", neu)
		}
	}
}

// A1: 決まりから外れる葉を表に足すと落ちる。
func TestLeafShapeRejects(t *testing.T) {
	for _, bad := range []string{"digest auth slack", "hello", "open project", "issue  start", "Issue start", "issue start x"} {
		ls := append(slices.Clone(leaves), leaf{name: bad, who: []who{role}})
		if errs := shapeErrors(ls); len(errs) == 0 {
			t.Errorf("%q を足しても落ちない", bad)
		}
	}
	// 名前を変えるまでの例外は renames にあるものだけ。例外から外せば、今の名前は落ちる
	saved := renames["measure"]
	delete(renames, "measure")
	defer func() { renames["measure"] = saved }()
	if errs := shapeErrors(leaves); !slices.ContainsFunc(errs, func(e string) bool { return strings.HasPrefix(e, "measure:") }) {
		t.Errorf("例外から外した measure が落ちない: %v", errs)
	}
}

// 葉の表の欄が揃っていること（決めたこと 11）。
func TestLeafFields(t *testing.T) {
	for _, l := range leaves {
		if len(l.who) == 0 || l.about == "" || len(l.examples) == 0 {
			t.Errorf("%s: 打つ者・説明・例のどれかが無い", l.name)
		}
		if l.run == nil && l.name != "help" {
			t.Errorf("%s: 中身が無い", l.name)
		}
		if !strings.HasPrefix(l.usage, "usage: andon "+l.name) {
			t.Errorf("%s: 使い方が usage: andon %s で始まらない: %q", l.name, l.name, l.usage)
		}
		for _, e := range l.examples {
			if e != l.name && !strings.HasPrefix(e, l.name+" ") {
				t.Errorf("%s: 例 %q がこの葉を打っていない", l.name, e)
			}
		}
		if strings.Contains(l.about, "\n") {
			t.Errorf("%s: 説明が1行でない", l.name)
		}
	}
	for _, n := range featured {
		if l, ok := find(n); !ok || l.who[0] != owner {
			t.Errorf("help の例の %s がオーナーの葉でない", n)
		}
	}
}

// A2: 表に載った葉は全部、--help と -h が終了コード 0 で使い方を出す。
func TestLeafHelp(t *testing.T) {
	calls := stubLeaves(t)
	for _, l := range leaves {
		for _, h := range []string{"--help", "-h"} {
			args := append(l.words(), h)
			rc, out, errs := run(args...)
			if rc != 0 || errs != "" || !strings.HasPrefix(out, "usage: andon "+l.name) || !strings.Contains(out, "\n打つ者: ") || !strings.Contains(out, "\n終了コード: 0 ") {
				t.Errorf("%v: rc=%d out=%q err=%q", args, rc, out, errs)
			}
		}
	}
	// -- より前ならどこでも help、-- より後は葉に渡す
	if rc, out, _ := run("issue", "record", "S-0001", "--help"); rc != 0 || !strings.HasPrefix(out, "usage: andon issue record") {
		t.Errorf("引数のあとの --help: rc=%d out=%q", rc, out)
	}
	if len(*calls) > 0 {
		t.Errorf("--help で葉を打った: %v", *calls)
	}
	if rc, out, _ := run("schema", "check", "--", "-h"); rc != 0 || out != "stub schema check\n" {
		t.Errorf("-- のあとの -h を help に取った: rc=%d out=%q", rc, out)
	}
}

// A2: 表に載っていない2語は終了コード 2。
func TestUnlistedLeaf(t *testing.T) {
	calls := stubLeaves(t)
	for _, l := range leaves {
		if len(l.words()) == 1 {
			continue // 名詞でない1語の葉は2語目を引数として受ける（andon open <slug>）
		}
		args := []string{l.noun(), "zzzz"}
		if rc, out, errs := run(args...); rc != 2 || out != "" || !strings.Contains(errs, "知らないコマンド: "+l.noun()+" zzzz") {
			t.Errorf("%v: rc=%d out=%q err=%q", args, rc, out, errs)
		}
	}
	if rc, out, errs := run("bogus"); rc != 2 || out != "" || !strings.Contains(errs, "知らないコマンド: bogus") || !strings.Contains(errs, "andon help") {
		t.Errorf("bogus: rc=%d out=%q err=%q", rc, out, errs)
	}
	if len(*calls) > 0 {
		t.Errorf("表に無い名前で葉を打った: %v", *calls)
	}
}

// A2: 名詞だけを打つと、その名詞の葉の一覧が出る（名詞だけで動く今の葉 waiting・dispatcher と、1語の葉 boot・version を除く）。
func TestNounLists(t *testing.T) {
	stubLeaves(t)
	nouns := map[string]bool{}
	for _, l := range leaves {
		if len(l.words()) == 2 {
			nouns[l.noun()] = true
		}
	}
	for n := range nouns {
		if _, isLeaf := find(n); isLeaf {
			continue
		}
		for _, args := range [][]string{{n}, {n, "--help"}, {n, "-h"}} {
			rc, out, errs := run(args...)
			if rc != 0 || errs != "" || !strings.HasPrefix(out, "usage: andon "+n+" <動詞>") {
				t.Errorf("%v: rc=%d out=%q err=%q", args, rc, out, errs)
			}
			for _, k := range children(n) {
				if !regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(k.name) + `  `).MatchString(out) {
					t.Errorf("%v に %s の行が無い:\n%s", args, k.name, out)
				}
			}
		}
	}
	// 名詞だけで動く葉は、名詞の --help に同じ名詞の葉も並べる
	if _, out, _ := run("waiting", "--help"); !strings.Contains(out, "\n  waiting answer  ") {
		t.Errorf("waiting --help に waiting answer が無い:\n%s", out)
	}
}

// A2: 綴りを1字誤った葉を打つと、正しい葉を含む1行が標準エラーに出る。
func TestSuggestsNearLeaf(t *testing.T) {
	calls := stubLeaves(t)
	defer func() {
		if len(*calls) > 0 {
			t.Errorf("誤った綴りで葉を打った: %v", *calls)
		}
	}()
	for _, l := range leaves {
		w := l.words()
		last := w[len(w)-1]
		for i, typo := range []string{last[:len(last)-1], last[:1] + "x" + last[2:], last + "s"} {
			typed := append(slices.Clone(w[:len(w)-1]), typo)
			if _, ok := find(strings.Join(typed, " ")); ok {
				continue
			}
			rc, out, errs := run(typed...)
			want := "andon " + l.name
			line := ""
			for _, s := range strings.Split(errs, "\n") {
				if strings.HasSuffix(s, want) {
					line = s
				}
			}
			if rc != 2 || out != "" || line == "" {
				t.Errorf("%s の誤り %d %v: rc=%d out=%q err=%q", l.name, i, typed, rc, out, errs)
			}
		}
	}
	// 名詞の誤り（2語目が引数でも勧める）
	for _, args := range [][]string{{"isue", "start", "S-0001"}, {"gat", "S-0001"}, {"triag", "collect"}} {
		rc, _, errs := run(args...)
		if rc != 2 || !regexp.MustCompile(`近いのは andon (issue start|gate|triage collect)\n`).MatchString(errs) {
			t.Errorf("%v: rc=%d err=%q", args, rc, errs)
		}
	}
}

func TestHelp(t *testing.T) {
	stubLeaves(t)
	rc, out, errs := run("help")
	if rc != 0 || errs != "" || !strings.HasPrefix(out, "usage: andon <名詞> <動詞>") {
		t.Fatalf("rc=%d err=%q out=%q", rc, errs, out)
	}
	for _, args := range [][]string{nil, {"-h"}, {"--help"}} {
		if rc2, out2, _ := run(args...); rc2 != 0 || out2 != out {
			t.Errorf("%v: rc=%d, help と違う", args, rc2)
		}
	}
	// 例が先、オーナーの葉、役と機械の名詞の1行、続きの引き方の順（決めたこと 12）
	order := []string{"\n例:\n  andon open ", "\nオーナーが打つ:\n", "\n役が打つ: issue", "\n機械が打つ: ", "andon help --all", "kanban", "hc"}
	at := 0
	for _, s := range order {
		i := strings.Index(out[at:], s)
		if i < 0 {
			t.Fatalf("help に %q が順に無い:\n%s", s, out)
		}
		at += i
	}
	for _, l := range leaves {
		has := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(l.name) + `  `).MatchString(out)
		if (l.who[0] == owner) != has {
			t.Errorf("help の %s の行: あり=%v（主に打つ者 %s）", l.name, has, l.whoText())
		}
	}
	for _, args := range [][]string{{"help", "x"}, {"help", "--markdown"}, {"help", "--all", "--markdown", "--bash"}} {
		if rc, _, errs := run(args...); rc != 2 || !strings.HasPrefix(errs, "usage: andon help") {
			t.Errorf("%v: rc=%d err=%q", args, rc, errs)
		}
	}
}

func TestHelpAll(t *testing.T) {
	rc, out, _ := run("help", "--all")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	for _, l := range leaves {
		if !regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(l.name) + ` +` + regexp.QuoteMeta(l.whoText()) + `[　]*  ` + regexp.QuoteMeta(l.about) + `$`).MatchString(out) {
			t.Errorf("help --all に %s の行が無い", l.name)
		}
	}
	_, md, _ := run("help", "--all", "--markdown")
	for _, l := range leaves {
		if !strings.Contains(md, fmt.Sprintf("| `andon %s` | %s |", l.name, l.whoText())) {
			t.Errorf("--markdown に %s の行が無い", l.name)
		}
	}
	_, bash, _ := run("help", "--all", "--bash")
	if !strings.Contains(bash, "complete -F _andon andon") || !strings.Contains(bash, "issue) COMPREPLY=($(compgen -W \"start ") || !strings.Contains(bash, "    open)\n") {
		t.Errorf("--bash:\n%s", bash)
	}
}

// 1語で動く葉と、引数を取る1語の葉は、今と同じに引数を受ける。
func TestPassesArgsToLeaf(t *testing.T) {
	if rc, _, errs := run("dispatcher", "--bogus"); rc != 2 || !strings.HasPrefix(errs, "usage: andon dispatcher") {
		t.Errorf("dispatcher: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("chief-of-staff", "start"); rc != 2 || !strings.HasPrefix(errs, "usage: andon chief-of-staff") {
		t.Errorf("chief-of-staff: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("issue", "start", "X-1", "extra"); rc != 2 || !strings.HasPrefix(errs, "usage: andon issue start") {
		t.Errorf("issue start: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("triage", "verdict", "X-1"); rc != 2 || !strings.HasPrefix(errs, "usage: andon triage verdict") {
		t.Errorf("triage verdict: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("chief-engineer"); rc != 2 || !strings.HasPrefix(errs, "usage: andon chief-engineer") {
		t.Errorf("chief-engineer: rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := run("publish", "/nonexistent/none.html"); rc != 1 || !strings.Contains(errs, "not a file") {
		t.Errorf("publish: rc=%d err=%q", rc, errs)
	}
	// フックが打つ wait と rewake は、使い方の誤りでも 0（決めたこと 15）
	for _, v := range []string{"wait", "rewake"} {
		if rc, _, _ := run("dispatcher", v, "--bogus"); rc != 0 {
			t.Errorf("dispatcher %s --bogus: rc=%d", v, rc)
		}
	}
}

// ADR-0008 決めたこと 7: andon task は別名を持たない。知らないコマンドとして止まり、一覧と issue の一覧に task が出ない。
func TestNoTaskCommand(t *testing.T) {
	stubLeaves(t)
	for _, args := range [][]string{{"task"}, {"task", "start", "X-1"}, {"task", "--help"}} {
		if rc, out, errs := run(args...); rc != 2 || out != "" || !strings.Contains(errs, "知らないコマンド: task") {
			t.Errorf("%v: rc=%d out=%q err=%q", args, rc, out, errs)
		}
	}
	_, help, _ := run("help", "--all")
	_, issue, _ := run("issue", "--help")
	for name, s := range map[string]string{"help --all": help, "issue --help": issue} {
		if regexp.MustCompile(`(?i)\btask`).MatchString(s) {
			t.Errorf("%s に task が出る:\n%s", name, s)
		}
	}
}
