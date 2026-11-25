package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"time"
)

// group は置き場の 1 行の resource 1 つと scope 1 つ（--otlp で resource と scope をそのまま出すため）。
type group struct {
	resource any
	scope    any
}

// span はハーネスの span 1 つ。trace・id・parent は送り手が付けた元の値。
type span struct {
	raw    map[string]any
	group  *group
	trace  string
	id     string
	parent string
	name   string
	start  time.Time
	end    time.Time
	attrs  map[string]any
	status int // OTLP の status.code（2 が ERROR）
}

// kindOf は span の名前から節の種類を決める（Claude Code の span の名前。ほかは span）。
func kindOf(name string) string {
	switch name {
	case "claude_code.interaction":
		return KindInteraction
	case "claude_code.llm_request":
		return KindLLM
	case "claude_code.tool":
		return KindTool
	}
	return KindSpan
}

// attach は課題 id のハーネスの記録を読み、根の span（プロンプト 1 つ分）を起動の下に入れる。
// 起動は session.id で選ぶ。同じ session の起動が無ければ（Codex は起動のときに session が決まらない）、
// span が始まる前の最後の起動に入れる。起動が 1 つも無ければ、session ごとの「起動の記録に無いセッション」の節を作って返す。
func (r *reader) attach(id string, launches []*Node) ([]*Node, Harness) {
	rel := filepath.ToSlash(filepath.Join("issues", id, "traces.jsonl"))
	h := Harness{Issue: id, File: rel, State: "none"}
	if r.o.Otel == "" {
		return nil, h
	}
	spans, st := r.spans(filepath.Join(r.o.Otel, "issues", id, "traces.jsonl"))
	switch st {
	case fileMissing:
		return nil, h
	case fileUnreadable:
		h.State = "unreadable"
		return nil, h
	}
	h.Spans = len(spans)
	if len(spans) == 0 {
		h.State = "empty"
		return nil, h
	}
	h.State = "ok"
	roots := tree(spans)
	var extra []*Node
	orphans := map[string]*Node{}
	for _, root := range roots {
		owner := pick(launches, root)
		if owner == nil {
			sid, _ := root.span.attrs["session.id"].(string)
			if owner = orphans[sid]; owner == nil {
				owner = &Node{Kind: KindLaunch, Name: "起動の記録に無いセッション " + sid, Issue: id, Start: root.Start, Attrs: map[string]any{"session": sid}}
				orphans[sid] = owner
				extra = append(extra, owner)
			}
			if root.End != nil && (owner.End == nil || owner.End.Before(*root.End)) {
				owner.End = tp(*root.End)
			}
		}
		owner.Children = append(owner.Children, root)
	}
	for _, l := range append(append([]*Node{}, launches...), extra...) {
		sortNodes(l.Children)
		var sum Summary
		for _, c := range l.Children {
			if c.Summary != nil {
				sum.add(*c.Summary)
			}
		}
		if len(l.Children) > 0 {
			l.Summary = &sum
		}
	}
	return extra, h
}

// pick は根の span を入れる起動（無ければ nil）。
func pick(launches []*Node, root *Node) *Node {
	if len(launches) == 0 {
		return nil
	}
	sid, _ := root.span.attrs["session.id"].(string)
	if sid != "" {
		var hit *Node
		for _, l := range launches {
			if s, _ := l.Attrs["session"].(string); s == sid && !l.Start.After(root.span.start) {
				hit = l // 同じ session を続きから起動し直したら、後の起動
			}
		}
		if hit != nil {
			return hit
		}
		for _, l := range launches {
			if s, _ := l.Attrs["session"].(string); s == sid {
				return l
			}
		}
	}
	var last *Node
	for _, l := range launches {
		if !l.Start.After(root.span.start) {
			last = l
		}
	}
	if last == nil {
		last = launches[0]
	}
	return last
}

// spans は置き場の traces.jsonl を読む。同じ span（trace と span の id が同じ）が 2 度あれば 1 つにする。
// JSON として読めない行（書く途中で切れた最後の行など）と、id の無い span は飛ばす。返す値の 2 つ目は lines の結果。
func (r *reader) spans(path string) ([]*span, int) {
	var spans []*span
	seen := map[string]bool{}
	st := r.lines(path, func(n int, b []byte) {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var line struct {
			ResourceSpans []struct {
				Resource   any `json:"resource"`
				ScopeSpans []struct {
					Scope any              `json:"scope"`
					Spans []map[string]any `json:"spans"`
				} `json:"scopeSpans"`
			} `json:"resourceSpans"`
		}
		if dec.Decode(&line) != nil {
			r.skip(path, n, "OTLP の JSON として読めない")
			return
		}
		for _, rs := range line.ResourceSpans {
			resAttrs := attrsOf(rs.Resource)
			for _, ss := range rs.ScopeSpans {
				g := &group{resource: rs.Resource, scope: ss.Scope}
				for _, raw := range ss.Spans {
					sp, why := parseSpan(raw, resAttrs)
					if why != "" {
						r.skip(path, n, "%s", why)
						continue
					}
					key := sp.trace + "/" + sp.id
					if seen[key] {
						continue
					}
					seen[key] = true
					sp.group = g
					spans = append(spans, sp)
				}
			}
		}
	})
	return spans, st
}

func parseSpan(raw map[string]any, resAttrs map[string]any) (*span, string) {
	sp := &span{raw: raw}
	sp.trace, _ = raw["traceId"].(string)
	sp.id, _ = raw["spanId"].(string)
	sp.parent, _ = raw["parentSpanId"].(string)
	sp.name, _ = raw["name"].(string)
	if sp.trace == "" || sp.id == "" {
		return nil, "span に traceId か spanId が無い"
	}
	var ok1, ok2 bool
	sp.start, ok1 = nanos(raw["startTimeUnixNano"])
	sp.end, ok2 = nanos(raw["endTimeUnixNano"])
	if !ok1 || !ok2 {
		return nil, fmt.Sprintf("span %s の時刻が読めない", sp.id)
	}
	sp.attrs = attrsOf(raw)
	if _, ok := sp.attrs["session.id"]; !ok {
		if s, ok := resAttrs["session.id"]; ok {
			sp.attrs["session.id"] = s
		}
	}
	if st, ok := raw["status"].(map[string]any); ok {
		if c, ok := number(st["code"]); ok {
			sp.status = int(c)
		}
	}
	return sp, ""
}

// nanos は OTLP の JSON の時刻（Unix の nano 秒。文字列か数）を読む。
func nanos(v any) (time.Time, bool) {
	n, ok := number(v)
	if !ok || n <= 0 {
		return time.Time{}, false
	}
	return time.Unix(0, n).UTC(), true
}

// number は OTLP の JSON の 64 ビットの整数（文字列か数）を読む。
func number(v any) (int64, bool) {
	switch x := v.(type) {
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return n, err == nil
	case float64:
		return int64(x), true
	}
	return 0, false
}

// attrsOf は記録（span か resource）の attributes を、キーから Go の値への map にする。
func attrsOf(rec any) map[string]any {
	out := map[string]any{}
	m, _ := rec.(map[string]any)
	list, _ := m["attributes"].([]any)
	for _, a := range list {
		kv, _ := a.(map[string]any)
		k, _ := kv["key"].(string)
		if k == "" {
			continue
		}
		out[k] = anyValue(kv["value"])
	}
	return out
}

// anyValue は OTLP の AnyValue を Go の値にする。
func anyValue(v any) any {
	m, _ := v.(map[string]any)
	switch {
	case m == nil:
		return nil
	case m["stringValue"] != nil:
		return m["stringValue"]
	case m["intValue"] != nil:
		if n, ok := number(m["intValue"]); ok {
			return n
		}
		return m["intValue"]
	case m["doubleValue"] != nil:
		if n, ok := m["doubleValue"].(json.Number); ok {
			if f, err := n.Float64(); err == nil {
				return f
			}
		}
		return m["doubleValue"]
	case m["boolValue"] != nil:
		return m["boolValue"]
	case m["arrayValue"] != nil:
		arr, _ := m["arrayValue"].(map[string]any)
		vals, _ := arr["values"].([]any)
		out := make([]any, 0, len(vals))
		for _, x := range vals {
			out = append(out, anyValue(x))
		}
		return out
	case m["kvlistValue"] != nil:
		kl, _ := m["kvlistValue"].(map[string]any)
		return attrsOf(map[string]any{"attributes": kl["values"]})
	case m["bytesValue"] != nil:
		return m["bytesValue"]
	}
	return nil
}

// tree は span を親子で結び、根（親の無い span と、親が置き場に無い span）の節を返す。数えは根ごとに付ける。
func tree(spans []*span) []*Node {
	nodes := map[string]*Node{}
	for _, sp := range spans {
		nodes[sp.trace+"/"+sp.id] = &Node{Kind: kindOf(sp.name), Name: sp.name, Start: tp(sp.start), End: tp(sp.end), Attrs: sp.attrs, span: sp}
	}
	var roots []*Node
	parent := map[*Node]*Node{}
	for _, sp := range spans {
		n := nodes[sp.trace+"/"+sp.id]
		if p, ok := nodes[sp.trace+"/"+sp.parent]; ok && sp.parent != "" && p != n {
			p.Children = append(p.Children, n)
			parent[n] = p
			continue
		}
		roots = append(roots, n)
	}
	// 親子が輪になった span（送り手の誤り）は根から届かない。届かない span を 1 つずつ親から外して根にし、輪を切る。
	reached := map[*Node]bool{}
	var mark func(n *Node)
	mark = func(n *Node) {
		if reached[n] {
			return
		}
		reached[n] = true
		for _, c := range n.Children {
			mark(c)
		}
	}
	for _, root := range roots {
		mark(root)
	}
	for _, sp := range spans {
		n := nodes[sp.trace+"/"+sp.id]
		if reached[n] {
			continue
		}
		p := parent[n]
		p.Children = slices.DeleteFunc(p.Children, func(c *Node) bool { return c == n })
		roots = append(roots, n)
		mark(n)
	}
	var walk func(n *Node, seen map[*Node]bool) Summary
	walk = func(n *Node, seen map[*Node]bool) Summary {
		var s Summary
		if seen[n] {
			return s
		}
		seen[n] = true
		s.count(n)
		sort.SliceStable(n.Children, func(i, j int) bool { return n.Children[i].Start.Before(*n.Children[j].Start) })
		for _, c := range n.Children {
			s.add(walk(c, seen))
		}
		return s
	}
	for _, root := range roots {
		s := walk(root, map[*Node]bool{})
		root.Summary = &s
	}
	return roots
}

// count は span 1 つ分を数える。
func (s *Summary) count(n *Node) {
	sp := n.span
	s.Spans++
	switch n.Kind {
	case KindLLM:
		s.LLMRequests++
		s.Tokens.Input += intAttr(sp.attrs, "input_tokens")
		s.Tokens.Output += intAttr(sp.attrs, "output_tokens")
		s.Tokens.CacheRead += intAttr(sp.attrs, "cache_read_tokens")
		s.Tokens.CacheWrite += intAttr(sp.attrs, "cache_creation_tokens")
	case KindTool:
		s.Tools++
	}
	if ok, has := sp.attrs["success"].(bool); (has && !ok) || sp.status == 2 {
		s.Errors++
	}
}

func (s *Summary) add(o Summary) {
	s.Spans += o.Spans
	s.LLMRequests += o.LLMRequests
	s.Tools += o.Tools
	s.Errors += o.Errors
	s.Tokens.Input += o.Tokens.Input
	s.Tokens.Output += o.Tokens.Output
	s.Tokens.CacheRead += o.Tokens.CacheRead
	s.Tokens.CacheWrite += o.Tokens.CacheWrite
}

func intAttr(attrs map[string]any, k string) int64 {
	n, _ := attrs[k].(int64)
	return n
}
