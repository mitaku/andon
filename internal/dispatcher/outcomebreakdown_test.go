package dispatcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/otel"
)

// postSample は受け口の見本（Claude Code v2.1.331 が出した形）の resource 属性の課題と役を替えて、受け口に POST する。
// traceID が空でなければ、全部の span の traceId をそれに替える（同じ span の送り直しとして 1 つにまとめられないように）。
// 置き場には受け口が書く（個人の属性を落として伏せた行）。返す値は見本の claude_code.llm_request のトークンの合計。
func postSample(t *testing.T, r *otel.Receiver, project, issue, role, traceID string) map[string]int64 {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "otel", "testdata", "traces.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	sum := map[string]int64{}
	for _, rs := range m["resourceSpans"].([]any) {
		res := rs.(map[string]any)["resource"].(map[string]any)
		for _, a := range res["attributes"].([]any) {
			kv := a.(map[string]any)
			switch kv["key"] {
			case "andon.project":
				kv["value"] = map[string]any{"stringValue": project}
			case "andon.issue":
				kv["value"] = map[string]any{"stringValue": issue}
			case "andon.role":
				kv["value"] = map[string]any{"stringValue": role}
			}
		}
		for _, ss := range rs.(map[string]any)["scopeSpans"].([]any) {
			for _, sp := range ss.(map[string]any)["spans"].([]any) {
				span := sp.(map[string]any)
				if traceID != "" {
					span["traceId"] = traceID
				}
				if span["name"] != "claude_code.llm_request" {
					continue
				}
				for _, a := range span["attributes"].([]any) {
					kv := a.(map[string]any)
					k, _ := kv["key"].(string)
					switch k {
					case "input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens":
						// OTLP の JSON の 64 ビットの整数は文字列か数
						n, err := strconv.ParseInt(fmt.Sprint(kv["value"].(map[string]any)["intValue"]), 10, 64)
						if err != nil {
							t.Fatal(err)
						}
						sum[k] += n
					}
				}
			}
		}
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("受け口が %d を返した: %s", w.Code, w.Body.String())
	}
	return sum
}

// ADR-0060 受け入れ条件 11: dispatcher が片付けを拾うと、outcome.json に breakdown が入り、トークンの合計が見本の
// claude_code.llm_request の合計と同じ。置き場に記録の無い課題では null。レビュアーの課題には、レビュアーの課題の
// フォルダの記録だけを数えた breakdown が入り、親の課題の breakdown にその分が入らない。どれも schema を通る。
func TestOutcomeBreakdown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	const (
		w, r, none = "D-0001", "D-0001-r1", "D-0002"
		sess       = "56401e3a-30ce-4815-a5da-9acdc5bcf49f" // 見本の session.id
	)
	q := func(at, by, status string) string {
		return `{"at":"` + at + `","by":"terminal:` + by + `","id":"` + w + `","change":{"question":1,"status":"` + status + `"}}`
	}
	p := outcomeProject(t,
		statusRow(w, "2026-11-19T09:00:00Z", "doing"),
		q("2026-11-19T09:02:00Z", "t7", "open"),
		q("2026-11-19T09:05:00Z", "t5", "answered"),
		`{"at":"2026-11-19T09:06:00Z","by":"dispatcher","id":"`+r+`","change":{"add":"D-0001 のレビュー（1 回目）"}}`,
		statusRow(r, "2026-11-19T09:06:30Z", "doing"),
		statusRow(r, "2026-11-19T09:08:00Z", "review"),
		statusRow(w, "2026-11-19T09:10:00Z", "done"),
		statusRow(r, "2026-11-19T09:11:00Z", "done"),
		statusRow(none, "2026-11-19T09:00:00Z", "doing"),
		statusRow(none, "2026-11-19T09:10:00Z", "done"),
	)
	p.l.Slug = "demo"
	p.tasks = []ledger.Issue{{"id": w, "profile": "claude"}, {"id": r, "profile": "codex-review", "parent": w}, {"id": none, "profile": "claude"}}
	writeFile(t, filepath.Join(p.l.Library(w), "launches.jsonl"), `{"role":"worker","session":"`+sess+`","harness":{"name":"claude"},"repo":null}`+"\n")
	writeFile(t, filepath.Join(p.l.Library(r), "launches.jsonl"), `{"role":"reviewer","session":"`+sess+`","harness":{"name":"claude"},"repo":null}`+"\n")
	rec := &otel.Receiver{Root: otel.Root(), Log: io.Discard}
	want := postSample(t, rec, "demo", w, "worker", "")
	postSample(t, rec, "demo", r, "reviewer", "")
	postSample(t, rec, "demo", r, "reviewer", "0123456789abcdef0123456789abcdef") // レビュアーの課題にだけ、別のトレースでもう 1 回分
	for _, id := range []string{w, r, none} {
		p.st.Outcomes.Add(id)
	}
	p.outcomes()
	read := func(id string) (map[string]any, []byte) {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), outcomeFile))
		if err != nil {
			t.Fatalf("%s に outcome.json を書いていない: %v", id, err)
		}
		if err := contract.SchemaOutcome.Check(b); err != nil {
			t.Errorf("%s の outcome.json が schema を通らない: %v", id, err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v, b
	}
	tokens := func(o map[string]any) map[string]any {
		bd, _ := o["breakdown"].(map[string]any)
		tk, _ := bd["tokens"].(map[string]any)
		return tk
	}
	ow, raw := read(w)
	tk := tokens(ow)
	if tk == nil || tk["input"] != float64(want["input_tokens"]) || tk["output"] != float64(want["output_tokens"]) ||
		tk["cache_read"] != float64(want["cache_read_tokens"]) || tk["cache_write"] != float64(want["cache_creation_tokens"]) || want["output_tokens"] == 0 {
		t.Errorf("担当の課題の breakdown のトークンが見本の合計 %v と違う: %s", want, raw)
	}
	bd := ow["breakdown"].(map[string]any)
	if bd["role"] != "worker" || asJSON(t, bd["coverage"]) != `{"sessions":1,"traced":1}` || bd["interactions"] != 1.0 ||
		asJSON(t, bd["waits"]) != `{"ci_ms":0,"gone_ms":0,"question_ms":180000,"review_ms":120000}` ||
		!strings.Contains(asJSON(t, bd["tools"]), `"name":"Read"`) {
		t.Errorf("担当の課題の breakdown が違う: %s", raw)
	}
	or, rawr := read(r)
	if tk := tokens(or); tk == nil || tk["input"] != float64(2*want["input_tokens"]) || tk["output"] != float64(2*want["output_tokens"]) {
		t.Errorf("レビュアーの課題の breakdown が自分の置き場の 2 回分でない: %s", rawr)
	}
	if bd := or["breakdown"].(map[string]any); bd["role"] != "reviewer" {
		t.Errorf("レビュアーの課題の役: %s", rawr)
	}
	on, rawn := read(none)
	if v, ok := on["breakdown"]; !ok || v != nil {
		t.Errorf("記録の無い課題の breakdown が null でない: %s", rawn)
	}
}

// 生の記録の値が schema の外（負のトークン・足すと int64 を越える数）でも、結果の記録は書く。
// 数えるほうは負の値を数えに入れず、和を上限で止める。それでも schema を通らない内訳は null にしてログに 1 行残す。
func TestOutcomeBreakdownBadValues(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	const neg, big = "D-0003", "D-0004"
	p := outcomeProject(t,
		statusRow(neg, "2026-11-19T09:00:00Z", "doing"), statusRow(neg, "2026-11-19T09:10:00Z", "done"),
		statusRow(big, "2026-11-19T09:00:00Z", "doing"), statusRow(big, "2026-11-19T09:10:00Z", "done"),
	)
	p.l.Slug = "demo"
	p.tasks = []ledger.Issue{{"id": neg, "profile": "claude"}, {"id": big, "profile": "claude"}}
	span := func(id, tokens string) string {
		return `{"traceId":"c43ee97f7dd8f491fa3848a4438c5d81","spanId":"` + id + `","name":"claude_code.llm_request",` +
			`"startTimeUnixNano":"1763542800000000000","endTimeUnixNano":"1763542801000000000","attributes":[` +
			`{"key":"input_tokens","value":{"intValue":"` + tokens + `"}},{"key":"output_tokens","value":{"intValue":"7"}}]}`
	}
	put := func(id string, spans ...string) {
		writeFile(t, filepath.Join(otel.Root(), "demo", "issues", id, "traces.jsonl"),
			`{"resourceSpans":[{"resource":{"attributes":[]},"scopeSpans":[{"spans":[`+strings.Join(spans, ",")+`]}]}]}`+"\n")
	}
	put(neg, span("a1", "-5"), span("a2", "3"))
	put(big, span("b1", "9223372036854775807"), span("b2", "9223372036854775807"))
	p.st.Outcomes.Add(neg)
	p.st.Outcomes.Add(big)
	p.outcomes()
	for id, want := range map[string]string{neg: `{"input":3,"output":14,"cache_read":0,"cache_write":0}`,
		big: `{"input":9223372036854775807,"output":14,"cache_read":0,"cache_write":0}`} {
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), outcomeFile))
		if err != nil {
			t.Fatalf("%s の値の外れた記録で outcome.json を書いていない: %v", id, err)
		}
		var v struct {
			Breakdown *struct {
				Tokens json.RawMessage `json:"tokens"`
			} `json:"breakdown"`
		}
		if err := json.Unmarshal(b, &v); err != nil || v.Breakdown == nil || compactJSON(t, v.Breakdown.Tokens) != want {
			t.Errorf("%s の内訳のトークンが %s でない: %s", id, want, b)
		}
	}
}

func compactJSON(t *testing.T, raw []byte) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
