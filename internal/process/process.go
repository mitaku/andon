// Package process は流れと状態の定義（process/ の YAML。ADR-0017 決めたこと 1・2・16）を読み、
// 人が読む Markdown（docs/process/<名>.md の表と Mermaid の図）を作る（andon process render）。
//
// 定義の形の正は schema/process/*.schema.json で、検査は tests/process.sh が schema_check.py でする
// （andon process json が YAML を JSON にして渡す）。ここは知らないキーで止まるだけで、schema を写さない。
// dispatcher は定義を読まない（決めたこと 1）。
package process

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Usage は andon process の使い方。
const Usage = "usage: andon process render [--check] [<repo の根>]\n" +
	"       andon process json <file.yaml>\n" +
	"render: process/workstreams/*.yaml と process/states/*.yaml から docs/process/<名>.md（表と Mermaid の図）を作る。\n" +
	"  docs/process/ の README.md 以外の .md は生成物で、定義の無いものは消す。--check は書かずに、作り直すと差分が出れば\n" +
	"  そのファイルを並べて終了コード 1。<repo の根> の既定は今のディレクトリ。\n" +
	"json: YAML の定義を JSON にして出す（tests/process.sh が schema/process/ に通すため）。\n"

// Main は andon process を走らせる。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, Usage)
		return 2
	}
	switch args[0] {
	case "render":
		fs := flag.NewFlagSet("andon process render", flag.ContinueOnError)
		fs.SetOutput(stderr)
		check := fs.Bool("check", false, "書かずに差分を確かめる")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		root := "."
		switch fs.NArg() {
		case 0:
		case 1:
			root = fs.Arg(0)
		default:
			fmt.Fprint(stderr, Usage)
			return 2
		}
		diff, err := Render(root, *check)
		if err != nil {
			fmt.Fprintf(stderr, "andon process render: %v\n", err)
			return 1
		}
		if *check && len(diff) > 0 {
			for _, p := range diff {
				fmt.Fprintf(stderr, "andon process render: 作り直すと変わる: %s\n", p)
			}
			fmt.Fprintln(stderr, "andon process render: andon process render で作り直して commit する")
			return 1
		}
		if !*check {
			for _, p := range diff {
				fmt.Fprintln(stdout, p)
			}
		}
		return 0
	case "json":
		if len(args) != 2 {
			fmt.Fprint(stderr, Usage)
			return 2
		}
		out, err := ToJSON(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "andon process json: %v\n", err)
			return 1
		}
		stdout.Write(out)
		return 0
	}
	fmt.Fprint(stderr, Usage)
	return 2
}

// ToJSON は YAML のファイルを JSON にする（1 文書だけ。値の型は YAML のまま）。
func ToJSON(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return append(out, '\n'), nil
}

// Strings は 1 つの文字列か文字列の並び（YAML のどちらの書き方も受ける）。
type Strings []string

// UnmarshalYAML は scalar を 1 件の並びにする。
func (s *Strings) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		*s = Strings{n.Value}
		return nil
	}
	var l []string
	if err := n.Decode(&l); err != nil {
		return err
	}
	*s = l
	return nil
}

// Workstream は process/workstreams/<名>.yaml（schema/process/workstream.schema.json）。
type Workstream struct {
	Name     string    `yaml:"name"`
	Title    string    `yaml:"title"`
	Purpose  string    `yaml:"purpose"`
	Triggers []Trigger `yaml:"triggers"`
	Steps    []Step    `yaml:"steps"`
	Counts   []string  `yaml:"counts"`
}

// Trigger はきっかけ。
type Trigger struct {
	Name         string `yaml:"name"`
	Kind         string `yaml:"kind"`
	Notification string `yaml:"notification"`
	Every        any    `yaml:"every"`
	Start        string `yaml:"start"`
}

// Step は手順の 1 件。
type Step struct {
	ID      string   `yaml:"id"`
	Title   string   `yaml:"title"`
	By      string   `yaml:"by"`
	Input   []string `yaml:"input"`
	Output  []string `yaml:"output"`
	Next    []Next   `yaml:"next"`
	Command string   `yaml:"command"`
	Record  string   `yaml:"record"`
	Once    bool     `yaml:"once"`
	Skill   Strings  `yaml:"skill"`
	Decides []string `yaml:"decides"`
	Asks    []Ask    `yaml:"asks"`
}

// Next は続く手順。
type Next struct {
	ID   string `yaml:"id"`
	When string `yaml:"when"`
}

// Ask は人に聞くこと。
type Ask struct {
	Question string `yaml:"question"`
	Kind     string `yaml:"kind"`
}

// StateMachine は process/states/<名>.yaml（schema/process/state.schema.json）。
type StateMachine struct {
	Name        string       `yaml:"name"`
	Title       string       `yaml:"title"`
	Source      string       `yaml:"source"`
	States      []State      `yaml:"states"`
	Positions   []string     `yaml:"positions"`
	Transitions []Transition `yaml:"transitions"`
}

// State は状態の 1 件。
type State struct {
	ID     string  `yaml:"id"`
	Title  string  `yaml:"title"`
	Record Strings `yaml:"record"`
}

// Transition は移り方の 1 件。
type Transition struct {
	From    string `yaml:"from"`
	To      string `yaml:"to"`
	On      string `yaml:"on"`
	By      string `yaml:"by"`
	Command string `yaml:"command"`
	Once    bool   `yaml:"once"`
}

func decode(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Render は root の process/ を読み、docs/process/ の生成物を作る。check なら書かない。
// 返すのは変わる（check でないときは変えた）ファイルの root からの相対パス。
func Render(root string, check bool) ([]string, error) {
	want := map[string][]byte{} // docs/process/<名>.md → 中身
	add := func(name, src string, body []byte) error {
		if base := strings.TrimSuffix(filepath.Base(src), ".yaml"); name != base {
			return fmt.Errorf("%s: name %q がファイルの名前（%s）と違う", src, name, base)
		}
		p := filepath.Join("docs", "process", name+".md")
		if _, dup := want[p]; dup {
			return fmt.Errorf("%s: 名前 %q がほかの定義と重なる", src, name)
		}
		want[p] = body
		return nil
	}
	ws, err := filepath.Glob(filepath.Join(root, "process", "workstreams", "*.yaml"))
	if err != nil {
		return nil, err
	}
	for _, f := range ws {
		var w Workstream
		if err := decode(f, &w); err != nil {
			return nil, err
		}
		src, _ := filepath.Rel(root, f)
		if err := add(w.Name, src, RenderWorkstream(w, filepath.ToSlash(src))); err != nil {
			return nil, err
		}
	}
	ss, err := filepath.Glob(filepath.Join(root, "process", "states", "*.yaml"))
	if err != nil {
		return nil, err
	}
	for _, f := range ss {
		var s StateMachine
		if err := decode(f, &s); err != nil {
			return nil, err
		}
		src, _ := filepath.Rel(root, f)
		if err := add(s.Name, src, RenderStates(s, filepath.ToSlash(src))); err != nil {
			return nil, err
		}
	}

	// 書く・消す先が symlink なら止まる（repo の外へ書いたり、外のファイルを消したりしない）。読む・書く・消すは
	// os.Root を通すので、確かめたあとに差し替えられても root の外へは出ない。
	if err := noLink(root, filepath.Join("docs", "process")); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var changed []string
	have, err := fs.Glob(r.FS(), "docs/process/*.md")
	if err != nil {
		return nil, err
	}
	for _, rel := range have {
		if path.Base(rel) == "README.md" {
			continue
		}
		if _, ok := want[filepath.FromSlash(rel)]; ok {
			continue
		}
		if err := noLink(root, rel); err != nil {
			return nil, err
		}
		changed = append(changed, rel) // 定義の無い生成物
		if !check {
			afterCheck()
			if err := r.Remove(rel); err != nil {
				return nil, err
			}
		}
	}
	for rel, body := range want {
		if err := noLink(root, rel); err != nil {
			return nil, err
		}
		slash := filepath.ToSlash(rel)
		old, err := r.ReadFile(slash)
		if err == nil && bytes.Equal(old, body) {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		changed = append(changed, slash)
		if !check {
			afterCheck()
			if err := r.MkdirAll("docs/process", 0o755); err != nil {
				return nil, err
			}
			if err := r.WriteFile(slash, body, 0o644); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// afterCheck はテストが確かめと書く・消すの間に差し替えをはさむ口。
var afterCheck = func() {}

// noLink は root の下の rel のどの段（docs・docs/process・ファイル）も symlink でないことを確かめる。無い段はよい。
func noLink(root, rel string) error {
	p := root
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		p = filepath.Join(p, part)
		fi, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			r, _ := filepath.Rel(root, p)
			return fmt.Errorf("%s が symlink（生成物の置き場は repo の中の普通のディレクトリとファイルだけ）", filepath.ToSlash(r))
		}
	}
	return nil
}

var byName = map[string]string{"machine": "機械", "ai": "AI", "person": "人", "kanban": "台帳の CLI"}

var triggerKind = map[string]string{
	"person": "人の依頼", "notification": "通知", "interval": "間隔", "date": "日付", "event": "出来事",
}

func header(b *strings.Builder, title, src string) {
	fmt.Fprintf(b, "# %s\n\n", title)
	fmt.Fprintf(b, "> このファイルは `andon process render` が [`%s`](../../%s) から作ったもの。直すときは YAML を直して作り直す（手で直さない）。\n\n", src, src)
}

// cell は表の 1 マス（| と改行を逃がす）。
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", " ")
}

func cells(l []string) string {
	if len(l) == 0 {
		return ""
	}
	q := make([]string, len(l))
	for i, s := range l {
		q[i] = "`" + cell(s) + "`"
	}
	return strings.Join(q, "・")
}

// label は Mermaid の文字（" を逃がす）。
func label(s string) string {
	s = strings.ReplaceAll(s, `"`, "#quot;")
	return strings.ReplaceAll(s, "\n", " ")
}

// node は Mermaid の識別子（- は使えるが予約語とぶつからないよう前に字を付ける）。
func node(prefix, id string) string {
	return prefix + "_" + strings.ReplaceAll(id, "-", "_")
}

func union(steps []Step, pick func(Step) []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range steps {
		for _, v := range pick(s) {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	return out
}

// RenderWorkstream はワークストリームの Markdown を作る。src は YAML の repo からの相対パス。
func RenderWorkstream(w Workstream, src string) []byte {
	var b strings.Builder
	header(&b, w.Title, src)
	fmt.Fprintf(&b, "目的: %s\n", w.Purpose)
	if len(w.Triggers) == 0 && len(w.Steps) == 0 {
		b.WriteString("\n中身（きっかけと手順）はまだ書いていない。\n")
		return []byte(b.String())
	}

	if len(w.Triggers) > 0 {
		b.WriteString("\n## きっかけ\n\n| きっかけ | 種類 | 中身 | 最初の手順 |\n|---|---|---|---|\n")
		for _, t := range w.Triggers {
			what := ""
			switch t.Kind {
			case "notification":
				what = "通知 `" + cell(t.Notification) + "`"
			case "interval":
				switch v := t.Every.(type) {
				case string:
					what = "`project.json` の `" + cell(v) + "`"
				case nil:
				default:
					what = fmt.Sprintf("%v 分ごと", v)
				}
			}
			fmt.Fprintf(&b, "| %s | %s | %s | `%s` |\n", cell(t.Name), triggerKind[t.Kind], what, cell(t.Start))
		}
	}

	if len(w.Steps) > 0 {
		b.WriteString("\n## 流れ\n\n")
		writeFlowchart(&b, w)
		b.WriteString("\n## 手順\n\n| id | 手順 | やる者 | 入るもの | 出るもの | 次 |\n|---|---|---|---|---|---|\n")
		for _, s := range w.Steps {
			var next []string
			for _, n := range s.Next {
				if n.When != "" {
					next = append(next, "`"+cell(n.ID)+"`（"+cell(n.When)+"）")
				} else {
					next = append(next, "`"+cell(n.ID)+"`")
				}
			}
			if len(next) == 0 {
				next = []string{"終わり"}
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n", cell(s.ID), cell(s.Title), byName[s.By],
				cells(s.Input), cells(s.Output), strings.Join(next, "・"))
		}

		var machine, ai []Step
		for _, s := range w.Steps {
			switch s.By {
			case "machine":
				machine = append(machine, s)
			case "ai":
				ai = append(ai, s)
			}
		}
		if len(machine) > 0 {
			b.WriteString("\n## 機械がやる手順\n\n| id | コマンド | 記録 | 2 回起きてはいけない |\n|---|---|---|---|\n")
			for _, s := range machine {
				once := ""
				if s.Once {
					once = "はい"
				}
				fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", cell(s.ID), cells(nonEmpty(s.Command)), cells(nonEmpty(s.Record)), once)
			}
		}
		if len(ai) > 0 {
			b.WriteString("\n## AI がやる手順\n\n| id | どうやるか（SKILL の節） | 自分で決めてよいこと | 人に聞くこと |\n|---|---|---|---|\n")
			for _, s := range ai {
				var asks []string
				for _, a := range s.Asks {
					asks = append(asks, cell(a.Question)+"（"+a.Kind+"）")
				}
				fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", cell(s.ID), cells(s.Skill), cellList(s.Decides), strings.Join(asks, "・"))
			}
		}
		if in := union(w.Steps, func(s Step) []string { return s.Input }); len(in) > 0 {
			fmt.Fprintf(&b, "\n見るもの（手順の入るものの和）: %s\n", cells(in))
		}
		if out := union(w.Steps, func(s Step) []string { return s.Output }); len(out) > 0 {
			fmt.Fprintf(&b, "\n残すもの（手順の出るものの和）: %s\n", cells(out))
		}
	}
	if len(w.Counts) > 0 {
		fmt.Fprintf(&b, "\n## 数えるもの\n\n%s\n", cells(w.Counts))
	}
	return []byte(b.String())
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func cellList(l []string) string {
	q := make([]string, len(l))
	for i, s := range l {
		q[i] = cell(s)
	}
	return strings.Join(q, "・")
}

// writeFlowchart は BPMN の形をまねた flowchart を書く（決めたこと 16）: 出来事は ((…))、やることは (…) で
// やる者の種類で色を分け、分かれ道は {…}、やる者の種類ごとに subgraph の帯。
func writeFlowchart(b *strings.Builder, w Workstream) {
	b.WriteString("```mermaid\nflowchart LR\n")
	b.WriteString("  classDef machine fill:#dbeafe,stroke:#1d4ed8,color:#0b1b3a\n")
	b.WriteString("  classDef ai fill:#dcfce7,stroke:#15803d,color:#052e16\n")
	b.WriteString("  classDef person fill:#fef3c7,stroke:#b45309,color:#3b1d02\n")
	b.WriteString("  classDef event fill:#f3f4f6,stroke:#4b5563,color:#111827\n")
	for _, t := range w.Triggers {
		fmt.Fprintf(b, "  %s((\"%s\")):::event\n", node("t", t.Name), label(t.Name))
	}
	ends := false
	for _, by := range []string{"machine", "ai", "person"} {
		var in []Step
		for _, s := range w.Steps {
			if s.By == by {
				in = append(in, s)
			}
		}
		if len(in) == 0 {
			continue
		}
		fmt.Fprintf(b, "  subgraph lane_%s[\"%s\"]\n", by, byName[by])
		for _, s := range in {
			fmt.Fprintf(b, "    %s(\"%s\"):::%s\n", node("s", s.ID), label(s.Title), by)
			if len(s.Next) > 1 {
				fmt.Fprintf(b, "    %s{\"分かれ道\"}\n", node("g", s.ID))
			}
		}
		b.WriteString("  end\n")
	}
	for _, t := range w.Triggers {
		fmt.Fprintf(b, "  %s --> %s\n", node("t", t.Name), node("s", t.Start))
	}
	for _, s := range w.Steps {
		out := label(strings.Join(s.Output, "・"))
		from := node("s", s.ID)
		switch {
		case len(s.Next) == 0:
			ends = true
			fmt.Fprintf(b, "  %s%s fin\n", from, arrow(out))
		case len(s.Next) == 1:
			fmt.Fprintf(b, "  %s%s %s\n", from, arrow(firstNonEmpty(label(s.Next[0].When), out)), node("s", s.Next[0].ID))
		default:
			fmt.Fprintf(b, "  %s%s %s\n", from, arrow(out), node("g", s.ID))
			for _, n := range s.Next {
				fmt.Fprintf(b, "  %s%s %s\n", node("g", s.ID), arrow(label(n.When)), node("s", n.ID))
			}
		}
	}
	if ends {
		b.WriteString("  fin((\"終わり\")):::event\n")
	}
	b.WriteString("```\n")
}

func arrow(text string) string {
	if text == "" {
		return " -->"
	}
	return " -->|\"" + text + "\"|"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// RenderStates はステートマシンの Markdown を作る。
func RenderStates(s StateMachine, src string) []byte {
	var b strings.Builder
	header(&b, s.Title, src)
	fmt.Fprintf(&b, "出所: %s\n", s.Source)
	if len(s.States) == 0 && len(s.Transitions) == 0 && len(s.Positions) == 0 {
		b.WriteString("\n中身（状態と移り方）はまだ書いていない。\n")
		return []byte(b.String())
	}
	if len(s.States) > 0 || len(s.Transitions) > 0 {
		b.WriteString("\n## 図\n\n```mermaid\nstateDiagram-v2\n")
		for _, st := range s.States {
			fmt.Fprintf(&b, "  state \"%s\" as %s\n", label(st.Title), node("s", st.ID))
		}
		for _, t := range s.Transitions {
			fmt.Fprintf(&b, "  %s --> %s: %s\n", stateNode(t.From), stateNode(t.To), strings.ReplaceAll(label(t.On), ":", "："))
		}
		b.WriteString("```\n")
	}
	if len(s.States) > 0 {
		b.WriteString("\n## 状態\n\n| id | 状態 | 記録 |\n|---|---|---|\n")
		for _, st := range s.States {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", cell(st.ID), cell(st.Title), cells(st.Record))
		}
	}
	if len(s.Positions) > 0 {
		fmt.Fprintf(&b, "\n状態でない項目（拾う位置とデータ）: %s\n", cells(s.Positions))
	}
	if len(s.Transitions) > 0 {
		b.WriteString("\n## 移り方\n\n| から | へ | 何が起きたら | 誰が | コマンド | 2 回起きてはいけない |\n|---|---|---|---|---|---|\n")
		for _, t := range s.Transitions {
			once := ""
			if t.Once {
				once = "はい"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", stateCell(t.From), stateCell(t.To), cell(t.On), byName[t.By],
				cells(nonEmpty(t.Command)), once)
		}
	}
	return []byte(b.String())
}

func stateNode(id string) string {
	if id == "[*]" {
		return id
	}
	return node("s", id)
}

func stateCell(id string) string {
	if id == "[*]" {
		return "（始まり・終わり）"
	}
	return "`" + cell(id) + "`"
}
