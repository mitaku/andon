// Package cli は andon の葉を、葉の表（leaves.go）から振り分ける（ADR-0053 決めたこと 11・12・16）。
//
//	andon [help]                       例と、オーナーが打つ葉の一覧（役と機械は名詞の1行）
//	andon help --all [--markdown|--bash]  全部の葉（--markdown は docs/commands.md、--bash は bash の補完）
//	andon <名詞>                       その名詞の葉の一覧
//	andon <名詞> <動詞> [引数…]        葉に渡す。-h・--help は -- より前のどこでも使い方を出す
//
// 表に無い名詞と動詞は、綴りの近い葉を1つ勧めて終了コード 2。
package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

const helpUsage = "usage: andon help [--all [--markdown|--bash]]\n" +
	"例と、オーナーが打つ葉の一覧を出す。役と機械が打つものは名詞の1行で出す。\n" +
	"--all は全部の葉を打つ者と一緒に出す。--markdown は docs/commands.md の中身、--bash は bash の補完のスクリプトを出す。\n"

var whoName = map[who]string{owner: "オーナー", role: "役", machine: "機械"}

func (l leaf) words() []string { return strings.Fields(l.name) }
func (l leaf) noun() string    { return l.words()[0] }

func (l leaf) whoText() string {
	var s []string
	for _, w := range l.who {
		s = append(s, whoName[w])
	}
	return strings.Join(s, "・")
}

func find(name string) (leaf, bool) {
	for _, l := range leaves {
		if l.name == name {
			return l, true
		}
	}
	return leaf{}, false
}

func children(noun string) []leaf {
	var out []leaf
	for _, l := range leaves {
		if w := l.words(); len(w) > 1 && w[0] == noun {
			out = append(out, l)
		}
	}
	return out
}

func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		help(stdout)
		return 0
	}
	if args[0] == "-h" || args[0] == "--help" {
		args = append([]string{"help"}, args[1:]...)
	}
	if args[0] == "help" {
		return helpCmd(args[1:], stdout, stderr)
	}
	noun := args[0]
	kids := children(noun)
	verb := len(args) > 1 && !strings.HasPrefix(args[1], "-")
	if rc, ok := refuseRetired(args, kids, verb, stderr); ok {
		return rc
	}
	if verb && len(kids) > 0 {
		l, ok := find(noun + " " + args[1])
		if !ok {
			return unknown(args[:2], stderr)
		}
		return runLeaf(l, args[2:], stdin, stdout, stderr)
	}
	if l, ok := find(noun); ok {
		return runLeaf(l, args[1:], stdin, stdout, stderr)
	}
	if len(kids) > 0 {
		if len(args) == 1 || wantsHelp(args[1:]) {
			nounHelp(noun, stdout)
			return 0
		}
		return unknown(args[:1], stderr)
	}
	if verb {
		return unknown(args[:2], stderr)
	}
	return unknown(args[:1], stderr)
}

// refuseRetired は名前を変えた葉の古い名前（retired）を、実行せずに新しい名前を1行で言って終了コード 2 で断る（決めたこと 17）。
// 古い名詞がまだ名詞として生きている（waiting）ときは、名詞だけを打ったときと --help の無いフラグだけを打ったときに断る。
// 古い名詞が消えた（delegate）ときは、動詞が何でも新しい名詞を言う。古い名前の1語目が1語の葉のまま残る（version stamp の
// version）ときは、名詞が消えたとは読まず、その1語の葉を打つ。
func refuseRetired(args []string, kids []leaf, verb bool, stderr io.Writer) (int, bool) {
	typed, neu := "", ""
	if len(args) > 1 {
		if n, ok := retired[args[0]+" "+args[1]]; ok {
			typed, neu = args[0]+" "+args[1], n
		}
	}
	if neu == "" {
		if n, ok := retired[args[0]]; ok && (len(kids) == 0 || (!verb && !wantsHelp(args[1:]))) {
			typed, neu = args[0], n
		}
	}
	if _, leaf := find(args[0]); neu == "" && len(kids) == 0 && !leaf {
		for old, n := range retired {
			if w := strings.Fields(old); len(w) == 2 && w[0] == args[0] {
				typed, neu = args[0], strings.Fields(n)[0]
				break
			}
		}
	}
	if neu == "" {
		return 0, false
	}
	fmt.Fprintf(stderr, "andon: andon %s は andon %s に名前が変わった（古い名前では動かない）。andon %s で打ち直す\n", typed, neu, neu)
	return 2, true
}

func runLeaf(l leaf, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		leafHelp(l, stdout)
		return 0
	}
	if l.project {
		var rc int
		var ok bool
		if args, rc, ok = project(l, args, stderr); !ok {
			return rc
		}
	}
	if l.json == jsonAlways {
		args = dropJSON(args)
	}
	return l.run(args, stdin, stdout, stderr)
}

// dropJSON は -- より前の --json を外す。いつも JSON を出す葉は --json を読まないので、振り分けが受けて外す（決めたこと 13）。
func dropJSON(args []string) []string {
	var out []string
	for i, a := range args {
		if a == "--" {
			return append(out, args[i:]...)
		}
		if a != "--json" {
			out = append(out, a)
		}
	}
	return out
}

// unknown は表に無い名前を断り、綴りの近い葉を1つ勧める（決めたこと 12）。typed は打った1語か2語。
// 2語で近い葉が無ければ1語目で探す（andon isue S-0012 に issue を勧める）。
func unknown(typed []string, stderr io.Writer) int {
	near := nearest(strings.Join(typed, " "))
	if near == "" && len(typed) > 1 {
		near = nearest(typed[0])
	}
	if near != "" {
		fmt.Fprintf(stderr, "andon: 知らないコマンド: %s。近いのは andon %s\n", strings.Join(typed, " "), near)
	} else {
		fmt.Fprintf(stderr, "andon: 知らないコマンド: %s\n", strings.Join(typed, " "))
	}
	fmt.Fprintln(stderr, "andon help で一覧を出す。")
	return 2
}

// nearest は typed との編集距離が 2 以内でいちばん近い葉か名詞（同じ距離なら表の先）。無ければ空。
func nearest(typed string) string {
	best, bestD := "", 3
	seen := map[string]bool{}
	for _, l := range leaves {
		for _, c := range []string{l.name, l.noun()} {
			if seen[c] {
				continue
			}
			seen[c] = true
			if d := distance(typed, c); d < bestD {
				best, bestD = c, d
			}
		}
	}
	return best
}

func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func helpCmd(args []string, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		l, _ := find("help")
		leafHelp(l, stdout)
		return 0
	}
	var all, md, bash bool
	for _, a := range args {
		switch a {
		case "--all":
			all = true
		case "--markdown":
			md = true
		case "--bash":
			bash = true
		default:
			fmt.Fprint(stderr, helpUsage)
			return 2
		}
	}
	switch {
	case (md || bash) && (!all || md == bash):
		fmt.Fprint(stderr, helpUsage)
		return 2
	case md:
		markdown(stdout)
	case bash:
		completion(stdout)
	case all:
		helpAll(stdout)
	default:
		help(stdout)
	}
	return 0
}

func byPrimary(w who) []leaf {
	var out []leaf
	for _, l := range leaves {
		if l.who[0] == w {
			out = append(out, l)
		}
	}
	return out
}

func rows(w io.Writer, ls []leaf, withWho bool) {
	nameW, whoW := 0, 0
	for _, l := range ls {
		nameW = max(nameW, len(l.name))
		whoW = max(whoW, len([]rune(l.whoText())))
	}
	for _, l := range ls {
		if withWho {
			pad := strings.Repeat("　", whoW-len([]rune(l.whoText())))
			fmt.Fprintf(w, "  %-*s  %s%s  %s\n", nameW, l.name, l.whoText(), pad, l.about)
		} else {
			fmt.Fprintf(w, "  %-*s  %s\n", nameW, l.name, l.about)
		}
	}
}

// help は andon と andon help の中身（決めたこと 12）: 1行の説明・オーナーがよく打つ例・オーナーが打つ葉・
// 役と機械が打つ名詞の1行・続きの引き方。
func help(w io.Writer) {
	fmt.Fprint(w, "usage: andon <名詞> <動詞> [引数…]\n")
	fmt.Fprint(w, "andon はセルの道具。プロジェクトの Chief of Staff・Chief Engineer・担当を起動し、ラインを回す。\n\n")
	fmt.Fprint(w, "例:\n")
	exW := 0
	for _, n := range featured {
		l, _ := find(n)
		exW = max(exW, width(l.examples[0]))
	}
	for _, n := range featured {
		l, _ := find(n)
		pad := strings.Repeat(" ", exW-width(l.examples[0]))
		fmt.Fprintf(w, "  andon %s%s  %s\n", l.examples[0], pad, l.about)
	}
	fmt.Fprint(w, "\nオーナーが打つ:\n")
	rows(w, byPrimary(owner), false)
	fmt.Fprintf(w, "\n役が打つ: %s\n", strings.Join(nouns(role), "・"))
	fmt.Fprintf(w, "機械が打つ: %s\n", strings.Join(nouns(machine), "・"))
	fmt.Fprint(w, "\nandon help --all で全部の葉、andon <名詞> --help でその名詞の葉、andon <名詞> <動詞> --help で使い方を出す。\n")
	fmt.Fprint(w, "課題の読み書きは kanban、端末は hc で打つ。\n")
}

// width は端末に出したときの幅。全角の字（U+1100 から）を 2 と数える。
func width(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x1100 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func nouns(w who) []string {
	var out []string
	for _, l := range byPrimary(w) {
		if !slices.Contains(out, l.noun()) {
			out = append(out, l.noun())
		}
	}
	return out
}

var groups = []who{owner, role, machine}

func helpAll(w io.Writer) {
	fmt.Fprint(w, "usage: andon <名詞> <動詞> [引数…]\n")
	for _, g := range groups {
		fmt.Fprintf(w, "\n%sが打つ:\n", whoName[g])
		rows(w, byPrimary(g), true)
	}
	fmt.Fprint(w, "\nandon <名詞> <動詞> --help で使い方を出す。課題の読み書きは kanban、端末は hc で打つ。\n")
}

func nounHelp(noun string, w io.Writer) {
	fmt.Fprintf(w, "usage: andon %s <動詞> [引数…]\n", noun)
	kids := children(noun)
	for _, g := range groups {
		var ls []leaf
		for _, l := range kids {
			if l.who[0] == g {
				ls = append(ls, l)
			}
		}
		if len(ls) > 0 {
			fmt.Fprintf(w, "\n%sが打つ:\n", whoName[g])
			rows(w, ls, false)
		}
	}
	fmt.Fprintf(w, "\nandon %s <動詞> --help で使い方を出す。\n", noun)
}

// leafHelp は andon <葉> --help の中身: 使い方・打つ者・例・終了コード（決めたこと 12・15）。
func leafHelp(l leaf, w io.Writer) {
	fmt.Fprint(w, l.usage)
	fmt.Fprintf(w, "\n打つ者: %s\n", l.whoText())
	if l.project {
		fmt.Fprintf(w, "プロジェクト: %s\n", projectText(l))
	}
	if t, ok := jsonText[l.json]; ok {
		fmt.Fprintf(w, "JSON: %s\n", t)
	}
	fmt.Fprint(w, "例:\n")
	for _, e := range l.examples {
		fmt.Fprintf(w, "  andon %s\n", e)
	}
	fmt.Fprintf(w, "終了コード: %s\n", exitText(l))
	if kids := children(l.name); len(kids) > 0 {
		fmt.Fprintf(w, "\n同じ名詞の葉:\n")
		rows(w, kids, false)
	}
}

// jsonText は andon <葉> --help の JSON の行（決めたこと 13）。
var jsonText = map[jsonForm]string{
	jsonAlways: "--json を付けても付けなくても、JSON を標準出力に出す",
	jsonFlag:   "--json で JSON を標準出力に出す。付けないと人向けに出す",
	jsonDryRun: "--dry-run --json で、書くはずのことを JSON で標準出力に出す（--dry-run の無い --json は使い方の誤り）",
}

func exitText(l leaf) string {
	means := map[int]string{0: "成功", 1: "失敗か断り", 2: "使い方の誤り"}
	codes := []int{0, 1, 2}
	if slices.ContainsFunc(l.exits, func(e exit) bool { return e.code == 0 }) {
		means, codes = map[int]string{}, nil
	}
	for _, e := range l.exits {
		if _, ok := means[e.code]; !ok {
			codes = append(codes, e.code)
		}
		means[e.code] = e.means
	}
	slices.Sort(codes)
	var s []string
	for _, c := range codes {
		s = append(s, fmt.Sprintf("%d %s", c, means[c]))
	}
	return strings.Join(s, "・")
}

// markdown は docs/commands.md の中身（決めたこと 16）。andon help --all と同じ葉を、同じ群と順で出す。
func markdown(w io.Writer) {
	esc := strings.NewReplacer("<", "&lt;", ">", "&gt;", "|", `\|`)
	fmt.Fprint(w, "# andon のコマンド\n\n")
	fmt.Fprint(w, "andon の葉（実際に打てる1つのコマンド）の一覧。`andon help --all --markdown` が葉の表から作るので、手で直さない。\n")
	fmt.Fprint(w, "使い方・フラグ・終了コードは、セルの中で `andon <名詞> <動詞> --help` を打って読む。\n")
	for _, g := range groups {
		fmt.Fprintf(w, "\n## %sが打つ\n\n", whoName[g])
		fmt.Fprint(w, "| コマンド | 打つ者 | 説明 |\n|---|---|---|\n")
		for _, l := range byPrimary(g) {
			fmt.Fprintf(w, "| `andon %s` | %s | %s |\n", l.name, l.whoText(), esc.Replace(l.about))
		}
	}
}

// completion は bash の補完のスクリプト（決めたこと 11。image/completions/andon に置く）。1語目と2語目を補い、
// slug を取る葉にはプロジェクトの slug を補う。フラグは補わない。
func completion(w io.Writer) {
	var first []string
	for _, l := range leaves {
		if !slices.Contains(first, l.noun()) {
			first = append(first, l.noun())
		}
	}
	fmt.Fprint(w, "# bash completion for andon。andon help --all --bash が葉の表から作るので、手で直さない。\n")
	fmt.Fprint(w, "# image/Dockerfile が /usr/share/bash-completion/completions/ に置く\n")
	fmt.Fprint(w, "_andon() {\n")
	fmt.Fprint(w, "  local cur=${COMP_WORDS[COMP_CWORD]}\n")
	fmt.Fprint(w, "  if [ \"$COMP_CWORD\" -eq 1 ]; then\n")
	fmt.Fprintf(w, "    COMPREPLY=($(compgen -W \"%s\" -- \"$cur\"))\n", strings.Join(first, " "))
	fmt.Fprint(w, "    return 0\n  fi\n")
	fmt.Fprint(w, "  [ \"$COMP_CWORD\" -eq 2 ] || return 0\n")
	fmt.Fprint(w, "  case ${COMP_WORDS[1]} in\n")
	var slugs []string
	for _, n := range first {
		if l, ok := find(n); ok && l.slugArg {
			slugs = append(slugs, n)
			continue
		}
		var verbs []string
		for _, k := range children(n) {
			verbs = append(verbs, k.words()[1])
		}
		if len(verbs) > 0 {
			fmt.Fprintf(w, "    %s) COMPREPLY=($(compgen -W \"%s\" -- \"$cur\")) ;;\n", n, strings.Join(verbs, " "))
		}
	}
	if len(slugs) > 0 {
		fmt.Fprintf(w, "    %s)\n", strings.Join(slugs, "|"))
		fmt.Fprint(w, "      local ws=${ANDON_WORKSPACE:-/workspace} d s=\n")
		fmt.Fprint(w, "      for d in \"$ws\"/projects/*/kanban/config.yaml; do\n")
		fmt.Fprint(w, "        [ -e \"$d\" ] && s=\"$s $(basename \"$(dirname \"$(dirname \"$d\")\")\")\"\n")
		fmt.Fprint(w, "      done\n")
		fmt.Fprint(w, "      COMPREPLY=($(compgen -W \"$s\" -- \"$cur\")) ;;\n")
	}
	fmt.Fprint(w, "  esac\n}\ncomplete -F _andon andon\n")
}
