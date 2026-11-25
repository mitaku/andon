package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"andon/internal/ledger"
	"andon/internal/otel"
)

const Usage = "usage: andon issue trace <id> [--json | --otlp]\n" +
	"課題 1 件の流れを、kanban の記録から導いた段（列・起動・走らせた command・問い・レビュー・gate・端末が死んでいた間・マージ）と、\n" +
	"その下のハーネスのトレース（受け口の置き場の記録。プロンプト 1 つ分・モデルの呼び出し・ツール）の 1 本の木にして出す。\n" +
	"レビュアーの課題の起動と記録は、レビューの段の下に入る。読むだけで、何も書かない。\n" +
	"既定は人が読む表で、プロンプト 1 つ分までを 1 行ずつ出す。--json は木の全部を出す。\n" +
	"--otlp は同じ木を OTLP の JSON の 1 本のトレース（1 行）で出す。閲覧器で開く手順は docs/operate.md。\n"

// Main は andon issue trace（ADR-0060 の andon trace。葉の形は ADR-0053 決めたこと 5 の2語）。statePath は dispatcher の控えのパス（gone を読む。dispatcher に依らないよう呼び手が渡す）。
func Main(args []string, stdout, stderr io.Writer, statePath func(slug string) string) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return 2 }
	var id, mode string
	for _, a := range args {
		switch a {
		case "--json", "--otlp":
			if mode != "" {
				return usage()
			}
			mode = a
		case "-h", "--help":
			fmt.Fprint(stdout, Usage)
			return 0
		default:
			if id != "" || strings.HasPrefix(a, "-") {
				return usage()
			}
			id = a
		}
	}
	if id == "" {
		return usage()
	}
	root := os.Getenv("KANBAN_ROOT")
	if root == "" {
		l, err := ledger.Open()
		if err != nil {
			fmt.Fprintf(stderr, "andon issue trace: %v\n", err)
			return 1
		}
		root = l.Root
	}
	slug := filepath.Base(root)
	o := Options{Root: root, Otel: filepath.Join(otel.Root(), slug), Now: time.Now().UTC()}
	if statePath != nil {
		o.State = statePath(slug)
	}
	t, err := Build(o, id)
	if err != nil {
		fmt.Fprintf(stderr, "andon issue trace: %v\n", err)
		return 1
	}
	switch mode {
	case "--json":
		err = write(stdout, t, true)
	case "--otlp":
		err = write(stdout, OTLP(t, o.Now), false)
	default:
		Table(stdout, t)
	}
	if err != nil {
		fmt.Fprintf(stderr, "andon issue trace: %v\n", err)
		return 1
	}
	return 0
}

// write は JSON を 1 つ書く（--json は人が読めるよう字下げし、--otlp は JSON Lines の 1 行にする）。
func write(w io.Writer, v any, indent bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if indent {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// Table は人が読む表。段・起動・プロンプト 1 つ分までを 1 行ずつ、時刻の順に字下げして出す。
func Table(w io.Writer, t *Trace) {
	root := t.Tree
	fmt.Fprintf(w, "課題 %s %s（%s）\n", t.Issue, root.Name, t.Project)
	if p, _ := root.Attrs["parent"].(string); p != "" {
		fmt.Fprintf(w, "親の課題 %s\n", p)
	}
	if links, _ := root.Attrs["links"].([]string); len(links) > 0 {
		fmt.Fprintf(w, "つながり %s\n", strings.Join(links, "・"))
	}
	fmt.Fprintln(w)
	line := func(start, length string, depth int, name string, sum *Summary) {
		s := pad(start, 19) + "  " + pad(length, 10) + "  " + strings.Repeat("  ", depth) + name
		if sum != nil {
			s += "  " + sum.String()
		}
		fmt.Fprintln(w, s)
	}
	line("始まり（UTC）", "長さ", 0, "段", nil)
	var row func(n *Node, depth int)
	row = func(n *Node, depth int) {
		name := n.Name
		if n.Kind == KindInteraction {
			name = "プロンプト"
			if seq, ok := n.Attrs["interaction.sequence"].(int64); ok {
				name = fmt.Sprintf("プロンプト %d", seq)
			}
		}
		line(stamp(n.Start), length(n), depth, name, n.Summary)
		if n.span != nil {
			return // ハーネスの節はプロンプト 1 つ分まで（下は --json と --otlp で）
		}
		// プロンプト 1 つ分でない根の span（Codex の span など）は、起動ごとに 1 行にまとめる
		var rest []*Node
		for _, c := range n.Children {
			if c.span != nil && c.Kind != KindInteraction {
				rest = append(rest, c)
				continue
			}
			row(c, depth+1)
		}
		if len(rest) > 0 {
			var sum Summary
			for _, c := range rest {
				sum.add(*c.Summary)
			}
			line(stamp(rest[0].Start), "", depth+1, fmt.Sprintf("プロンプトの外の span の根 %d 個（--json で見る）", len(rest)), &sum)
		}
	}
	for _, c := range root.Children {
		row(c, 0)
	}
	fmt.Fprintln(w)
	for _, h := range t.Harness {
		switch h.State {
		case "none":
			fmt.Fprintf(w, "ハーネスの記録が無い: %s（置き場に %s が無い）\n", h.Issue, h.File)
		case "empty":
			fmt.Fprintf(w, "ハーネスの記録が 0 件: %s（%s に span が無い）\n", h.Issue, h.File)
		case "unreadable":
			fmt.Fprintf(w, "ハーネスの記録が読めない: %s（置き場の %s はあるが読めない。理由は飛ばした記録の行）\n", h.Issue, h.File)
		default:
			fmt.Fprintf(w, "ハーネスの記録: %s span %d 件\n", h.Issue, h.Spans)
		}
	}
	if len(t.Skipped) > 0 {
		fmt.Fprintf(w, "読めずに飛ばした記録 %d 件\n", len(t.Skipped))
		for _, s := range t.Skipped {
			if s.Line > 0 {
				fmt.Fprintf(w, "  %s:%d %s\n", s.File, s.Line, s.Reason)
			} else {
				fmt.Fprintf(w, "  %s %s\n", s.File, s.Reason)
			}
		}
	}
}

func (s Summary) String() string {
	if s.LLMRequests == 0 && s.Tools == 0 && s.Errors == 0 {
		return fmt.Sprintf("span %d 個", s.Spans)
	}
	out := fmt.Sprintf("モデル %d 回・ツール %d 回", s.LLMRequests, s.Tools)
	if s.Errors > 0 {
		out += fmt.Sprintf("・失敗 %d", s.Errors)
	}
	return out + fmt.Sprintf("・トークン 入力 %d 出力 %d キャッシュ読み %d 書き %d", s.Tokens.Input, s.Tokens.Output, s.Tokens.CacheRead, s.Tokens.CacheWrite)
}

// pad は表示の幅で右を埋める（全角の文字は 2 桁に数える）。
func pad(s string, width int) string {
	w := 0
	for _, r := range s {
		if r >= 0x1100 && (r <= 0x115f || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe4f) || (r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6)) {
			w += 2
		} else {
			w++
		}
	}
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func stamp(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

// length は段の長さ。点の段は「点」、閉じていない段は「続いている」。
func length(n *Node) string {
	switch {
	case n.Start == nil:
		return "-"
	case n.End == nil:
		return "続いている"
	case n.End.Equal(*n.Start) && (n.Kind == KindGate || n.Kind == KindMerge || n.Kind == KindColumn):
		return "点"
	}
	d := n.End.Sub(*n.Start)
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// TraceID は課題から決まるトレースの id（同じ課題なら何度打っても同じ。決めたこと 14）。
func TraceID(project, issue string) string {
	return digest("andon trace", project, issue)[:32]
}

func digest(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}

// OTLP は木を OTLP の JSON の 1 本のトレースにする（決めたこと 14）。トレースの id は課題から決まる値にし、
// 段を span にし、ハーネスの根の span の親を起動の span に付け替える。親の無い span は課題の span 1 つだけ。
// ハーネスの span の id は元の trace と span の id から決まる値に替え（元のトレースをまたいで重ならないように）、
// resource と scope と属性は送り手のものをそのまま出す。続いている段の終わりは now にする。
func OTLP(t *Trace, now time.Time) map[string]any {
	traceID := TraceID(t.Project, t.Issue)
	var andonSpans []any
	type out struct {
		g     *group
		spans []any
	}
	var groups []*out
	byGroup := map[*group]*out{}
	var walk func(n *Node, path, parent string)
	walk = func(n *Node, path, parent string) {
		var id string
		if sp := n.span; sp != nil {
			id = digest("andon trace span", sp.trace, sp.id)[:16]
			raw := make(map[string]any, len(sp.raw))
			for k, v := range sp.raw {
				raw[k] = v
			}
			raw["traceId"], raw["spanId"], raw["parentSpanId"] = traceID, id, parent
			o := byGroup[sp.group]
			if o == nil {
				o = &out{g: sp.group}
				byGroup[sp.group] = o
				groups = append(groups, o)
			}
			o.spans = append(o.spans, raw)
		} else {
			id = digest("andon trace node", t.Project, t.Issue, path)[:16]
			andonSpans = append(andonSpans, andonSpan(n, traceID, id, parent, now))
		}
		for i, c := range n.Children {
			walk(c, fmt.Sprintf("%s.%d", path, i), id)
		}
	}
	walk(t.Tree, "0", "")
	res := []any{map[string]any{
		"resource": map[string]any{"attributes": []any{
			kv("service.name", "andon"), kv("andon.project", t.Project), kv("andon.issue", t.Issue),
		}},
		"scopeSpans": []any{map[string]any{"scope": map[string]any{"name": "andon.trace"}, "spans": andonSpans}},
	}}
	for _, o := range groups {
		ss := map[string]any{"spans": o.spans}
		if o.g.scope != nil {
			ss["scope"] = o.g.scope
		}
		rs := map[string]any{"scopeSpans": []any{ss}}
		if o.g.resource != nil {
			rs["resource"] = o.g.resource
		}
		res = append(res, rs)
	}
	return map[string]any{"resourceSpans": res}
}

// andonSpan は段と起動と課題の節を span にする（kind は INTERNAL）。属性は andon.kind・andon.issue と、節の属性。
func andonSpan(n *Node, traceID, id, parent string, now time.Time) map[string]any {
	start, end := now, now
	if n.Start != nil {
		start = *n.Start
	}
	if n.End != nil {
		end = *n.End
	}
	if end.Before(start) {
		end = start
	}
	attrs := []any{kv("andon.kind", n.Kind)}
	if n.Issue != "" {
		attrs = append(attrs, kv("andon.issue", n.Issue))
	}
	if n.End == nil {
		attrs = append(attrs, kv("andon.open", true))
	}
	for _, k := range sortedKeys(n.Attrs) {
		attrs = append(attrs, kv("andon."+k, n.Attrs[k]))
	}
	return map[string]any{
		"traceId": traceID, "spanId": id, "parentSpanId": parent, "name": n.Name, "kind": 1,
		"startTimeUnixNano": fmt.Sprint(start.UnixNano()), "endTimeUnixNano": fmt.Sprint(end.UnixNano()),
		"attributes": attrs,
	}
}

// kv は OTLP の属性 1 つ。
func kv(k string, v any) map[string]any {
	return map[string]any{"key": k, "value": otlpValue(v)}
}

func otlpValue(v any) map[string]any {
	switch x := v.(type) {
	case string:
		return map[string]any{"stringValue": x}
	case bool:
		return map[string]any{"boolValue": x}
	case int:
		return map[string]any{"intValue": fmt.Sprint(x)}
	case int64:
		return map[string]any{"intValue": fmt.Sprint(x)}
	case float64:
		return map[string]any{"doubleValue": x}
	case []string:
		vals := make([]any, 0, len(x))
		for _, s := range x {
			vals = append(vals, otlpValue(s))
		}
		return map[string]any{"arrayValue": map[string]any{"values": vals}}
	case map[string]int:
		vals := make([]any, 0, len(x))
		for _, k := range sortedKeys(x) {
			vals = append(vals, kv(k, x[k]))
		}
		return map[string]any{"kvlistValue": map[string]any{"values": vals}}
	}
	return map[string]any{"stringValue": fmt.Sprint(v)}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
