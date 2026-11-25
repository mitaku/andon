package trace

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 結果の記録の内訳 breakdown（docs/contract.md §19。ADR-0060 決めたこと 15）のうち、ハーネスの生の記録から数える部分。
// dispatcher が課題を片付けて outcome.json を書くとき、その課題の置き場（issues/<id>/）だけを読んで数える。
// レビュアーの課題の記録はレビュアーの課題の置き場にあるので、親の課題の内訳には入らない。
// 待ちのうち kanban の記録から数える分（問い・レビュー・端末が死んでいた間）は dispatcher が足す。

// topTools は内訳に残すツールの数（時間の長い順）。
const topTools = 10

// ciCommand は PR の checks を待つ command（Bash の全文 full_command に含まれれば、そのツールの時間を CI 待ちに数える）。
var ciCommand = regexp.MustCompile(`\bgh\s+(?:pr\s+checks|run\s+watch)\b`)

// launchRoles は起動の記録の役（schema/launch.schema.json の role）。置き場の andon.role はこのどれかのときだけ使う。
var launchRoles = map[string]bool{"worker": true, "reviewer": true, "chief-engineer": true, "chief-of-staff": true}

// Breakdown は outcome.json の breakdown（schema/outcome.schema.json）。
type Breakdown struct {
	Role         *string        `json:"role"`
	Coverage     Coverage       `json:"coverage"`
	Interactions int            `json:"interactions"`
	LLMMs        int64          `json:"llm_ms"`
	ToolMs       int64          `json:"tool_ms"`
	BlockedMs    int64          `json:"blocked_ms"`
	Tokens       Tokens         `json:"tokens"`
	Errors       BreakdownError `json:"errors"`
	Tools        []ToolUse      `json:"tools"`
	Waits        BreakdownWaits `json:"waits"`
}

// Coverage は起動の記録のセッションのうち、生の記録に span が 1 つでもあったものの数。
type Coverage struct {
	Sessions int `json:"sessions"`
	Traced   int `json:"traced"`
}

type BreakdownError struct {
	LLM  int `json:"llm"`
	Tool int `json:"tool"`
}

// ToolUse はツールの名前（Bash は最初の語も）ごとの数え。
type ToolUse struct {
	Name   string  `json:"name"`
	Argv0  *string `json:"argv0"`
	Count  int     `json:"count"`
	Ms     int64   `json:"ms"`
	Errors int     `json:"errors"`
}

// BreakdownWaits は待ちの内訳。CIMs だけをここで数え、ほかは dispatcher が kanban の記録から足す。
type BreakdownWaits struct {
	QuestionMs int64 `json:"question_ms"`
	ReviewMs   int64 `json:"review_ms"`
	CIMs       int64 `json:"ci_ms"`
	GoneMs     int64 `json:"gone_ms"`
}

// BreakdownOf は、ハーネスの記録のプロジェクトの置き場 dir の、課題 id の生の記録（traces.jsonl・logs.jsonl）を内訳にまとめる。
// sessions は起動の記録のセッション（outcome.json の sessions[] と同じ並び。id の決まらないものは ""）、role は起動の記録の役
// （無ければ ""。そのときは記録の resource 属性 andon.role が起動の記録の役のどれかなら、それを使う）。
// 生の記録が 1 行も無ければ nil（breakdown は null）。ファイルがあるのに読めなければ誤り。JSON として読めない行は飛ばす。
// 同じ span（traceId と spanId が同じ）は 1 つに数える。dispatcher の見回りの中で読むので、数えに使う属性だけを持って読む
// （Codex のレビュアーの記録は 1 本で数十 MB になる）。
func BreakdownOf(dir, id string, sessions []string, role string) (*Breakdown, error) {
	r := &reader{o: Options{Otel: dir}}
	base := filepath.Join(dir, "issues", id)
	nlogs := 0
	if r.lines(filepath.Join(base, "logs.jsonl"), func(int, []byte) { nlogs++ }) == fileUnreadable {
		return nil, fmt.Errorf("ハーネスの記録 %s が読めない", r.skipped[len(r.skipped)-1].File)
	}
	var (
		spans    []*span
		seen     = map[string]bool{} // 数える span（trace/span の id）
		sessSeen = map[string]bool{} // span が運んだセッションの id
		resRole  string
		ntraces  int
	)
	st := r.lines(filepath.Join(base, "traces.jsonl"), func(n int, b []byte) {
		ntraces++
		var line lightLine
		if json.Unmarshal(b, &line) != nil {
			return
		}
		for _, rs := range line.ResourceSpans {
			res := rs.Resource.attrs()
			if s, _ := res["andon.role"].(string); resRole == "" && launchRoles[s] {
				resRole = s
			}
			for _, ss := range rs.ScopeSpans {
				for _, x := range ss.Spans {
					attrs := attrsLight(x.Attributes)
					if _, ok := attrs["session.id"]; !ok {
						if v, ok := res["session.id"]; ok {
							attrs["session.id"] = v
						}
					}
					for _, k := range sessionKeys {
						if v, ok := attrs[k].(string); ok && v != "" {
							sessSeen[v] = true
						}
					}
					if !strings.HasPrefix(x.Name, "claude_code.") || x.TraceID == "" || x.SpanID == "" {
						continue
					}
					start, ok1 := lightNanos(x.Start)
					end, ok2 := lightNanos(x.End)
					if !ok1 || !ok2 || seen[x.TraceID+"/"+x.SpanID] {
						continue
					}
					seen[x.TraceID+"/"+x.SpanID] = true
					code, _ := lightInt(x.Status.Code)
					spans = append(spans, &span{trace: x.TraceID, id: x.SpanID, parent: x.Parent, name: x.Name,
						start: start, end: end, attrs: attrs, status: int(code)})
				}
			}
		}
	})
	if st == fileUnreadable {
		return nil, fmt.Errorf("ハーネスの記録 %s が読めない", r.skipped[len(r.skipped)-1].File)
	}
	if nlogs == 0 && ntraces == 0 {
		return nil, nil
	}
	b := count(spans)
	b.Coverage.Sessions = len(sessions)
	for _, s := range sessions {
		if s != "" && sessSeen[s] {
			b.Coverage.Traced++
		}
	}
	if role == "" {
		role = resRole
	}
	if role != "" {
		b.Role = &role
	}
	return b, nil
}

// sessionKeys は span がセッションの id を運ぶ属性（Claude Code は session.id、Codex は conversation.id）。
var sessionKeys = []string{"session.id", "gen_ai.conversation.id", "conversation.id"}

// keptAttrs は内訳の数えに使う属性（ほかは持たない）。
var keptAttrs = map[string]bool{
	"session.id": true, "gen_ai.conversation.id": true, "conversation.id": true, "andon.role": true,
	"tool_name": true, "gen_ai.tool.name": true, "bash_argv0": true, "full_command": true, "success": true,
	"input_tokens": true, "output_tokens": true, "cache_read_tokens": true, "cache_creation_tokens": true,
}

// lightLine は置き場の traces.jsonl の 1 行のうち、内訳に使う所。
type lightLine struct {
	ResourceSpans []struct {
		Resource   lightAttrs `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				TraceID    string          `json:"traceId"`
				SpanID     string          `json:"spanId"`
				Parent     string          `json:"parentSpanId"`
				Name       string          `json:"name"`
				Start      json.RawMessage `json:"startTimeUnixNano"`
				End        json.RawMessage `json:"endTimeUnixNano"`
				Attributes []lightAttr     `json:"attributes"`
				Status     struct {
					Code json.RawMessage `json:"code"`
				} `json:"status"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

type lightAttrs struct {
	Attributes []lightAttr `json:"attributes"`
}

func (a lightAttrs) attrs() map[string]any { return attrsLight(a.Attributes) }

type lightAttr struct {
	Key   string `json:"key"`
	Value struct {
		String *string         `json:"stringValue"`
		Int    json.RawMessage `json:"intValue"`
		Bool   *bool           `json:"boolValue"`
	} `json:"value"`
}

// attrsLight は属性の並びのうち keptAttrs のものを、文字列・int64・真偽値の値で返す。
func attrsLight(list []lightAttr) map[string]any {
	out := map[string]any{}
	for _, a := range list {
		if !keptAttrs[a.Key] {
			continue
		}
		switch {
		case a.Value.String != nil:
			out[a.Key] = *a.Value.String
		case a.Value.Bool != nil:
			out[a.Key] = *a.Value.Bool
		case len(a.Value.Int) > 0:
			if n, ok := lightInt(a.Value.Int); ok {
				out[a.Key] = n
			}
		}
	}
	return out
}

// lightInt は OTLP の JSON の 64 ビットの整数（文字列か数）を読む。
func lightInt(raw json.RawMessage) (int64, bool) {
	s := strings.Trim(string(raw), `"`)
	if s == "" || s == "null" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

func lightNanos(raw json.RawMessage) (time.Time, bool) {
	n, ok := lightInt(raw)
	if !ok || n <= 0 {
		return time.Time{}, false
	}
	return time.Unix(0, n).UTC(), true
}

// count は span を数える。サブエージェントの span は親のセッションの span の下に入るので、同じ課題の数に入る。
//   - interactions: claude_code.interaction の数（プロンプトの数）。
//   - llm_ms・tokens・errors.llm: claude_code.llm_request の全部（サブエージェントのものも）。
//   - tool_ms: ほかのツールの下でない claude_code.tool の時間の合計（サブエージェントを走らせるツールの時間は、その中を含む）。
//   - tools・errors.tool: claude_code.tool の全部。失敗は span の success が false か status が ERROR、または下の
//     claude_code.tool.execution がそうであるもの。
//   - blocked_ms: claude_code.tool.blocked_on_user の時間の合計（権限の確かめを待った時間）。
//   - waits.ci_ms: Bash の全文が PR の checks を待つ command のツールの時間を、重なりを1回にして合わせたもの。
func count(spans []*span) *Breakdown {
	b := &Breakdown{Tools: []ToolUse{}}
	byID := map[string]*span{}
	for _, sp := range spans {
		byID[sp.trace+"/"+sp.id] = sp
	}
	underTool := map[*span]bool{}
	var nested func(sp *span, depth int) bool
	nested = func(sp *span, depth int) bool {
		if v, ok := underTool[sp]; ok {
			return v
		}
		underTool[sp] = false // 輪になった親子（送り手の誤り）で止まる
		p := byID[sp.trace+"/"+sp.parent]
		v := false
		if p != nil && p != sp && sp.parent != "" && depth < len(spans) {
			v = p.name == "claude_code.tool" || nested(p, depth+1)
		}
		underTool[sp] = v
		return v
	}
	failedExec := map[*span]bool{}
	for _, sp := range spans {
		if sp.name == "claude_code.tool.execution" && failed(sp) {
			if p := byID[sp.trace+"/"+sp.parent]; p != nil {
				failedExec[p] = true
			}
		}
	}
	type key struct{ name, argv0 string }
	tools := map[key]*ToolUse{}
	var ci []interval
	for _, sp := range spans {
		d := ms(sp)
		switch sp.name {
		case "claude_code.interaction":
			b.Interactions++
		case "claude_code.llm_request":
			b.LLMMs += d
			add(&b.Tokens.Input, intAttr(sp.attrs, "input_tokens"))
			add(&b.Tokens.Output, intAttr(sp.attrs, "output_tokens"))
			add(&b.Tokens.CacheRead, intAttr(sp.attrs, "cache_read_tokens"))
			add(&b.Tokens.CacheWrite, intAttr(sp.attrs, "cache_creation_tokens"))
			if failed(sp) {
				b.Errors.LLM++
			}
		case "claude_code.tool.blocked_on_user":
			b.BlockedMs += d
		case "claude_code.tool":
			if !nested(sp, 0) {
				b.ToolMs += d
			}
			bad := failed(sp) || failedExec[sp]
			if bad {
				b.Errors.Tool++
			}
			name, argv0 := toolName(sp)
			k := key{name, ""}
			if argv0 != nil {
				k.argv0 = *argv0
			}
			t := tools[k]
			if t == nil {
				t = &ToolUse{Name: name, Argv0: argv0}
				tools[k] = t
			}
			t.Count++
			t.Ms += d
			if bad {
				t.Errors++
			}
			if cmd, _ := sp.attrs["full_command"].(string); ciCommand.MatchString(cmd) {
				ci = append(ci, interval{sp.start, sp.end})
			}
		}
	}
	for _, t := range tools {
		b.Tools = append(b.Tools, *t)
	}
	sort.Slice(b.Tools, func(i, j int) bool {
		x, y := b.Tools[i], b.Tools[j]
		if x.Ms != y.Ms {
			return x.Ms > y.Ms
		}
		if x.Name != y.Name {
			return x.Name < y.Name
		}
		return argv(x) < argv(y)
	})
	if len(b.Tools) > topTools {
		b.Tools = b.Tools[:topTools]
	}
	b.Waits.CIMs = covered(ci)
	return b
}

func argv(t ToolUse) string {
	if t.Argv0 == nil {
		return ""
	}
	return "\x00" + *t.Argv0 // nil を "" の argv0 より前にする
}

// add は送り手の数を足す。負の値（送り手の誤り）は数えに入れず、和は int64 の上限で止める（内訳は schema の 0 以上を守る）。
func add(sum *int64, v int64) {
	if v <= 0 {
		return
	}
	if *sum > math.MaxInt64-v {
		*sum = math.MaxInt64
		return
	}
	*sum += v
}

// toolName はツールの名前と、Bash なら最初の語（bash_argv0、無ければ full_command の最初の語）。
func toolName(sp *span) (string, *string) {
	name, _ := sp.attrs["tool_name"].(string)
	if name == "" {
		name, _ = sp.attrs["gen_ai.tool.name"].(string)
	}
	if name == "" {
		name = "unknown"
	}
	if name != "Bash" {
		return name, nil
	}
	a, _ := sp.attrs["bash_argv0"].(string)
	if a == "" {
		cmd, _ := sp.attrs["full_command"].(string)
		if f := strings.Fields(cmd); len(f) > 0 {
			a = f[0]
		}
	}
	if a == "" {
		return name, nil
	}
	return name, &a
}

func failed(sp *span) bool {
	ok, has := sp.attrs["success"].(bool)
	return (has && !ok) || sp.status == 2
}

// ms は span の長さ（ミリ秒。終わりが始まりより前なら 0）。
func ms(sp *span) int64 {
	if d := sp.end.Sub(sp.start); d > 0 {
		return d.Milliseconds()
	}
	return 0
}

type interval struct{ from, to time.Time }

// covered は区間を重なりを1回にして合わせた長さ（ミリ秒）。
func covered(in []interval) int64 {
	sort.Slice(in, func(i, j int) bool { return in[i].from.Before(in[j].from) })
	var total time.Duration
	var cur *interval
	for i := range in {
		x := in[i]
		if !x.to.After(x.from) {
			continue
		}
		if cur != nil && !x.from.After(cur.to) {
			if x.to.After(cur.to) {
				cur.to = x.to
			}
			continue
		}
		if cur != nil {
			total += cur.to.Sub(cur.from)
		}
		c := x
		cur = &c
	}
	if cur != nil {
		total += cur.to.Sub(cur.from)
	}
	return total.Milliseconds()
}
