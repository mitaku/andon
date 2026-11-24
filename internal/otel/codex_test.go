package otel

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Codex の見本（testdata/codex-*.json）は、Codex v0.166.0 のソース（codex-rs/otel の provider.rs・events/shared.rs・
// tool_result.rs と、SDK opentelemetry_sdk 0.31.0 の Resource::builder）が組む形を写したもので、動かして撮ったものではない。
// トレースは andon. の属性を span ごと（span_attributes）にだけ持ち、resource には持たない。ログは andon. の属性を
// resource（環境変数 OTEL_RESOURCE_ATTRIBUTES）に持ち、codex.tool_result にツールの出力の写し output と個人の属性を持つ。

func spansOf(m map[string]any) []any {
	scope := m["resourceSpans"].([]any)[0].(map[string]any)["scopeSpans"].([]any)[0].(map[string]any)
	return scope["spans"].([]any)
}

func attrValue(list []any, key string) (string, bool) {
	for _, a := range list {
		kv := a.(map[string]any)
		if kv["key"] == key {
			s, _ := kv["value"].(map[string]any)["stringValue"].(string)
			return s, true
		}
	}
	return "", false
}

// ADR-0060 決めたこと 19: resource に andon. の属性の無い Codex のトレースは、全部の span が同じ andon. の属性
// （span_attributes）を持てば、その値を resource の属性に写して、その課題の traces.jsonl に書く。
func TestReceiveCodexTraceSpanAttributes(t *testing.T) {
	c := newCell(t)
	in := sample(t, "codex-traces.json")
	if got := resourceAttrs(in["resourceSpans"].([]any)[0]); len(got) != 0 {
		t.Fatalf("見本の resource に andon. の属性がある（span から写すことを確かめられない）: %v", got)
	}
	want := count(in, "resourceSpans", "scopeSpans", "spans")
	if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	lines := readLines(t, filepath.Join(c.root, "sample-site", "issues", "S-0002", "traces.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("足された行が %d（1 のはず）", len(lines))
	}
	if got := count(lines[0], "resourceSpans", "scopeSpans", "spans"); got != want {
		t.Errorf("span の数が %d（見本は %d）", got, want)
	}
	res := lines[0]["resourceSpans"].([]any)[0].(map[string]any)["resource"].(map[string]any)["attributes"].([]any)
	for k, v := range map[string]string{"andon.project": "sample-site", "andon.issue": "S-0002", "andon.role": "reviewer", "service.name": "codex_cli_rs"} {
		if got, _ := attrValue(res, k); got != v {
			t.Errorf("resource の %s が %q（%q のはず）", k, got, v)
		}
	}
	for _, sp := range spansOf(lines[0]) {
		if got, _ := attrValue(sp.(map[string]any)["attributes"].([]any), "andon.issue"); got != "S-0002" {
			t.Errorf("span の andon.issue が %q（span の属性は残す）", got)
		}
	}
	if c.r.Dropped() != 0 {
		t.Errorf("捨てた数が %d", c.r.Dropped())
	}
}

func TestReceiveCodexTraceSpanAttributesDisagree(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(spans []any)
		reason string
	}{
		{"1 つの span だけ課題が違う", func(spans []any) {
			sp := spans[1].(map[string]any)
			sp["attributes"] = append(withoutKey(sp["attributes"].([]any), "andon.issue"),
				map[string]any{"key": "andon.issue", "value": map[string]any{"stringValue": "S-0003"}})
		}, "揃わない"},
		{"1 つの span だけ課題が無い", func(spans []any) {
			sp := spans[1].(map[string]any)
			sp["attributes"] = withoutKey(sp["attributes"].([]any), "andon.issue")
		}, "揃わない"},
		{"どの span にも無い", func(spans []any) {
			for _, s := range spans {
				sp := s.(map[string]any)
				for _, k := range andonKeys {
					sp["attributes"] = withoutKey(sp["attributes"].([]any), k)
				}
			}
		}, "andon.project が無い"},
		{"span の値の形が違う", func(spans []any) {
			for _, s := range spans {
				sp := s.(map[string]any)
				sp["attributes"] = append(withoutKey(sp["attributes"].([]any), "andon.issue"),
					map[string]any{"key": "andon.issue", "value": map[string]any{"stringValue": "../x"}})
			}
		}, "andon.issue の形が違う"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCell(t)
			in := sample(t, "codex-traces.json")
			tc.change(spansOf(in))
			if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
				t.Fatalf("POST が %d", got)
			}
			if f := c.files(t); len(f) != 0 {
				t.Errorf("書いた: %v", f)
			}
			if c.r.Dropped() != 1 {
				t.Errorf("捨てた数が %d（1 のはず）", c.r.Dropped())
			}
			if !strings.Contains(c.logb.String(), tc.reason) {
				t.Errorf("ログに理由 %q が無い: %s", tc.reason, c.logb.String())
			}
		})
	}
}

func TestReceiveCodexTraceResourceWins(t *testing.T) {
	c := newCell(t)
	in := sample(t, "codex-traces.json")
	for _, k := range []string{"andon.project", "andon.issue", "andon.role"} {
		v, _ := attrValue(spansOf(in)[0].(map[string]any)["attributes"].([]any), k)
		setResourceAttr(in, "resourceSpans", k, v)
	}
	if got := c.post(t, "/v1/traces", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	lines := readLines(t, filepath.Join(c.root, "sample-site", "issues", "S-0002", "traces.jsonl"))
	res := lines[0]["resourceSpans"].([]any)[0].(map[string]any)["resource"]
	n := map[string]int{}
	keys(res, n)
	if n["andon.issue"] != 1 || n["andon.project"] != 1 || n["andon.role"] != 1 {
		t.Errorf("resource の andon. の属性の数: %v（1 つずつのはず）", n)
	}
}

// ADR-0060 決めたこと 8・19: Codex のログは resource の andon. の属性でその課題の logs.jsonl に書き、
// 個人の属性（user.email・user.account_id・user.id）とツールの出力の写し（codex.tool_result の output）を落とす。
// ツールの引数（arguments）は、ツールの詳細として残す（決めたこと 7）。
func TestReceiveCodexLogs(t *testing.T) {
	c := newCell(t)
	in := sample(t, "codex-logs.json")
	before := map[string]int{}
	keys(in, before)
	for _, k := range []string{"user.email", "user.account_id", "user.id", "output", "arguments"} {
		if before[k] == 0 {
			t.Fatalf("見本に %s が無い（落とすか残すかを確かめられない）", k)
		}
	}
	if got := c.post(t, "/v1/logs", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	path := filepath.Join(c.root, "sample-site", "issues", "S-0002", "logs.jsonl")
	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("足された行が %d（1 のはず）", len(lines))
	}
	if got, want := count(lines[0], "resourceLogs", "scopeLogs", "logRecords"), count(in, "resourceLogs", "scopeLogs", "logRecords"); got != want {
		t.Errorf("log record の数が %d（見本は %d）", got, want)
	}
	after := map[string]int{}
	keys(lines[0], after)
	for _, k := range []string{"user.email", "user.account_id", "user.id", "output"} {
		if after[k] != 0 {
			t.Errorf("書いた行に %s が %d 個残っている", k, after[k])
		}
	}
	for _, k := range []string{"arguments", "conversation.id", "tool_name", "prompt_length"} {
		if after[k] != before[k] {
			t.Errorf("落とさない属性 %s の数が変わった: %d → %d", k, before[k], after[k])
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"example.com", "andon/internal/otel\\t1.204s"} {
		if strings.Contains(string(b), s) {
			t.Errorf("書いた行に %q が残っている", s)
		}
	}
}

func TestReceiveKeepsOutputOfOtherEvents(t *testing.T) {
	c := newCell(t)
	in := sample(t, "codex-logs.json")
	rec := in["resourceLogs"].([]any)[0].(map[string]any)["scopeLogs"].([]any)[0].(map[string]any)["logRecords"].([]any)[0].(map[string]any)
	rec["attributes"] = append(rec["attributes"].([]any), map[string]any{"key": "output", "value": map[string]any{"stringValue": "kept"}})
	if got := c.post(t, "/v1/logs", "application/json", encode(t, in)); got != http.StatusOK {
		t.Fatalf("POST が %d", got)
	}
	b, err := os.ReadFile(filepath.Join(c.root, "sample-site", "issues", "S-0002", "logs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"key":"output","value":{"stringValue":"kept"}`) {
		t.Error("codex.user_prompt の output まで落とした")
	}
}

func withoutKey(list []any, key string) []any {
	var out []any
	for _, a := range list {
		if a.(map[string]any)["key"] != key {
			out = append(out, a)
		}
	}
	return out
}
