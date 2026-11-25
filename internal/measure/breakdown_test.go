package measure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger/ledgertest"
)

// cosSpan は役ごとの置き場の traces.jsonl の 1 行（claude_code.llm_request の span 1 つ。docs/contract.md §30 の形）。
func cosSpan(traceID, spanID string, in, out, cr, cw int) string {
	attr := func(k string, v int) string { return fmt.Sprintf(`{"key":%q,"value":{"intValue":"%d"}}`, k, v) }
	return `{"resourceSpans":[{"resource":{"attributes":[{"key":"andon.project","value":{"stringValue":"sample-site"}},` +
		`{"key":"andon.role","value":{"stringValue":"chief-of-staff"}}]},"scopeSpans":[{"scope":{"name":"com.anthropic.claude_code.tracing"},` +
		`"spans":[{"traceId":"` + traceID + `","spanId":"` + spanID + `","name":"claude_code.llm_request","kind":1,` +
		`"startTimeUnixNano":"1795493633959000000","endTimeUnixNano":"1795493637451433805","attributes":[` +
		attr("input_tokens", in) + "," + attr("output_tokens", out) + "," + attr("cache_read_tokens", cr) + "," + attr("cache_creation_tokens", cw) + `]}]}]}]}`
}

// breakdownLedger は内訳のある課題と無い課題を混ぜた偽の kanban と、Chief of Staff の役ごとの置き場（期間 2026-11-01〜02）。
//
// 手で数えた値:
//
//	S-0001 担当（claude）。doing 01:00 → review 03:00 → done 04:00（実働 120 分）。内訳あり（worker）。gate fix・approve。レビュアー 1。
//	S-0001-r1 レビュアー（codex-review）。03:00 に足され 03:30 に done。内訳あり（reviewer）。
//	S-0002 担当（codex）。doing 02:00 → done 02:30（実働 30 分）。outcome.json の breakdown が null。
//	S-0003 担当（claude）。doing 05:00 → done 06:00（実働 60 分）。outcome.json が無い。
//	S-0004 オーナーの課題（done）。数えない。
//	S-0005 doing のまま。数えない。
//	S-0006 期間の前に done（内訳あり）。数えない。
//	S-0007 調べる課題（profile なし）。doing 07:00 → done 07:10（実働 10 分）。outcome.json が壊れている（飛ばして内訳無しに数える）。
//
//	内訳: 片付いた課題 5（S-0001・S-0001-r1・S-0002・S-0003・S-0007）、内訳無し 3。担当の待ち 1 件分。役は worker と reviewer の 2 行。
//	Chief of Staff: 11-01 に span a/1（10・20・300・40）、11-02 に a/2（1・2・30・4）と a/1 の重ね（数えない）と壊れた行。10-31 は期間の外。
//	  → 11・22・330・44、2 日。全体: worker 1000+500+8000+500=10000、reviewer 200+100+1500+200=2000、Chief of Staff 407 → 407/12407 = 0.03。
//	プロファイル: claude 2 件（実働 120・60 → 90。トークンは S-0001 の 10000 だけ。gate 2 件のうち fix 1 → 0.5。周回 1・0 → 0.5）、
//	  codex 1 件（実働 30。トークン 0 件。gate 0 件。周回 0）、profile なし 1 件（実働 10）。
func breakdownLedger(t *testing.T) (string, string) {
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: S\n")
	issue := func(id, extra string) {
		put(t, filepath.Join(k, "issues", id, "issue.yaml"), "id: "+id+"\ntitle: x\nstatus: done\ncreated: \"2026-11-01T00:00:00Z\"\n"+extra)
	}
	issue("S-0001", "profile: claude\n")
	issue("S-0001-r1", "profile: codex-review\nparent: S-0001\n")
	issue("S-0002", "profile: codex\n")
	issue("S-0003", "profile: claude\n")
	issue("S-0004", "who: person\n")
	put(t, filepath.Join(k, "issues/S-0005/issue.yaml"), "id: S-0005\ntitle: x\nstatus: doing\nprofile: claude\ncreated: \"2026-11-01T00:00:00Z\"\n")
	issue("S-0006", "profile: claude\n")
	issue("S-0007", "")
	worker := `{"role": "worker", "coverage": {"sessions": 1, "traced": 1}, "interactions": 3, "llm_ms": 1000, "tool_ms": 500, "blocked_ms": 0,
	  "tokens": {"input": 1000, "output": 500, "cache_read": 8000, "cache_write": 500}, "errors": {"llm": 1, "tool": 2},
	  "tools": [{"name": "Bash", "argv0": "go", "count": 4, "ms": 400, "errors": 1}, {"name": "Read", "argv0": null, "count": 9, "ms": 90, "errors": 0}],
	  "waits": {"question_ms": 0, "review_ms": 1800000, "ci_ms": 600000, "gone_ms": 0}}`
	reviewer := `{"role": "reviewer", "coverage": {"sessions": 1, "traced": 1}, "interactions": 1, "llm_ms": 300, "tool_ms": 200, "blocked_ms": 0,
	  "tokens": {"input": 200, "output": 100, "cache_read": 1500, "cache_write": 200}, "errors": {"llm": 0, "tool": 1},
	  "tools": [{"name": "Bash", "argv0": "go", "count": 2, "ms": 300, "errors": 0}, {"name": "Grep", "argv0": null, "count": 5, "ms": 50, "errors": 0}],
	  "waits": {"question_ms": 0, "review_ms": 0, "ci_ms": 0, "gone_ms": 0}}`
	put(t, filepath.Join(k, "issues/S-0001/library/outcome.json"), `{"issue": "S-0001", "breakdown": `+worker+`}`)
	put(t, filepath.Join(k, "issues/S-0001-r1/library/outcome.json"), `{"issue": "S-0001-r1", "breakdown": `+reviewer+`}`)
	put(t, filepath.Join(k, "issues/S-0002/library/outcome.json"), `{"issue": "S-0002", "breakdown": null}`)
	put(t, filepath.Join(k, "issues/S-0006/library/outcome.json"), `{"issue": "S-0006", "breakdown": `+worker+`}`)
	put(t, filepath.Join(k, "issues/S-0007/library/outcome.json"), `{"issue": "S-0007", "breakdown": `)
	put(t, filepath.Join(k, "issues/S-0001/library/gate/aaaa.json"),
		`{"at": "2026-11-01T02:00:00Z", "id": "S-0001", "pr": "`+repoURL+`1", "tier": "B", "decision": "fix", "merged": false}`)
	put(t, filepath.Join(k, "issues/S-0001/library/gate/bbbb.json"),
		`{"at": "2026-11-01T03:40:00Z", "id": "S-0001", "pr": "`+repoURL+`1", "tier": "B", "decision": "approve", "merged": false}`)
	put(t, filepath.Join(k, "log.jsonl"), ledgertest.Log(t,
		`{"at":"2026-10-30T01:00:00Z","by":"terminal:t9","id":"S-0006","change":{"status":"doing","agent":"t9"}}`,
		`{"at":"2026-10-30T02:00:00Z","by":"terminal:t9","id":"S-0006","change":{"status":"done"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"doing","agent":"t2"}}`,
		`{"at":"2026-11-01T02:00:00Z","by":"terminal:t3","id":"S-0002","change":{"status":"doing","agent":"t3"}}`,
		`{"at":"2026-11-01T02:30:00Z","by":"terminal:t3","id":"S-0002","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"review"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:dispatcher","id":"S-0001-r1","change":{"add":"review"}}`,
		`{"at":"2026-11-01T03:30:00Z","by":"terminal:r1","id":"S-0001-r1","change":{"status":"done"}}`,
		`{"at":"2026-11-01T04:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"done"}}`,
		`{"at":"2026-11-01T05:00:00Z","by":"terminal:t4","id":"S-0003","change":{"status":"doing","agent":"t4"}}`,
		`{"at":"2026-11-01T06:00:00Z","by":"terminal:t4","id":"S-0003","change":{"status":"done"}}`,
		`{"at":"2026-11-01T06:30:00Z","by":"terminal:t1","id":"S-0004","change":{"status":"done"}}`,
		`{"at":"2026-11-01T06:40:00Z","by":"terminal:t5","id":"S-0005","change":{"status":"doing","agent":"t5"}}`,
		`{"at":"2026-11-01T07:00:00Z","by":"terminal:t7","id":"S-0007","change":{"status":"doing","agent":"t7"}}`,
		`{"at":"2026-11-01T07:10:00Z","by":"terminal:t7","id":"S-0007","change":{"status":"done"}}`,
	))
	otel := filepath.Join(t.TempDir(), "otel", "sample-site")
	roles := filepath.Join(otel, "roles", "chief-of-staff")
	put(t, filepath.Join(roles, "2026-10-31.traces.jsonl"), cosSpan("a", "0", 999, 999, 999, 999)+"\n")
	put(t, filepath.Join(roles, "2026-11-01.traces.jsonl"), cosSpan("a", "1", 10, 20, 300, 40)+"\n")
	put(t, filepath.Join(roles, "2026-11-02.traces.jsonl"), cosSpan("a", "2", 1, 2, 30, 4)+"\n"+cosSpan("a", "1", 10, 20, 300, 40)+"\n"+`{"resourceSpans":[`+"\n")
	put(t, filepath.Join(roles, "2026-11-02.logs.jsonl"), `{"resourceLogs":[]}`+"\n")
	put(t, filepath.Join(otel, "roles", "worker", "2026-11-01.traces.jsonl"), cosSpan("b", "1", 5000, 5000, 5000, 5000)+"\n")
	return root, otel
}

func breakdownOpts(root, otel string) Options {
	return Options{Root: root, Since: day("2026-11-01"), Until: day("2026-11-02"), Now: day("2026-11-09"), Otel: otel,
		GH: func(string) ([]byte, error) { return []byte(`[]`), nil }}
}

func TestBreakdownSectionMatchesHandCount(t *testing.T) {
	root, otel := breakdownLedger(t)
	res, err := Measure(breakdownOpts(root, otel))
	if err != nil {
		t.Fatal(err)
	}
	b, err := contract.Encode(contract.SchemaMeasure, res)
	if err != nil {
		t.Fatalf("schema/measure.schema.json に通らない: %v", err)
	}
	var got struct {
		Breakdown map[string]any `json:"breakdown"`
		Profiles  []any          `json:"profiles"`
		Skipped   []Skip         `json:"skipped"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{}
	if err := json.Unmarshal([]byte(`{
	  "issues": 5, "none": 3,
	  "waits": {"issues": 1, "question_ms": 0, "review_ms": 1800000, "ci_ms": 600000, "gone_ms": 0},
	  "roles": [
	    {"role": "worker", "issues": 1, "token_issues": 1, "tokens": {"input": 1000, "output": 500, "cache_read": 8000, "cache_write": 500}, "cache_read_rate": 0.84, "errors": {"llm": 1, "tool": 2}},
	    {"role": "reviewer", "issues": 1, "token_issues": 1, "tokens": {"input": 200, "output": 100, "cache_read": 1500, "cache_write": 200}, "cache_read_rate": 0.79, "errors": {"llm": 0, "tool": 1}}],
	  "tools": [
	    {"name": "Bash", "argv0": "go", "count": 6, "ms": 700, "errors": 1},
	    {"name": "Read", "argv0": null, "count": 9, "ms": 90, "errors": 0},
	    {"name": "Grep", "argv0": null, "count": 5, "ms": 50, "errors": 0}],
	  "chief_of_staff": {"days": 2, "tokens": {"input": 11, "output": 22, "cache_read": 330, "cache_write": 44}, "share": 0.03}}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Breakdown, want) {
		t.Errorf("breakdown:\n got %s\nwant %s", js(t, got.Breakdown), js(t, want))
	}
	var wantP []any
	if err := json.Unmarshal([]byte(`[
	  {"profile": "claude", "issues": 2, "worked_minutes": {"count": 2, "median": 90}, "tokens": {"count": 1, "median": 10000},
	   "gate": {"records": 2, "fix": 1, "needs_human": 0, "rate": 0.5}, "review_rounds": {"count": 2, "median": 0.5}},
	  {"profile": "codex", "issues": 1, "worked_minutes": {"count": 1, "median": 30}, "tokens": {"count": 0, "median": null},
	   "gate": {"records": 0, "fix": 0, "needs_human": 0, "rate": null}, "review_rounds": {"count": 1, "median": 0}},
	  {"profile": null, "issues": 1, "worked_minutes": {"count": 1, "median": 10}, "tokens": {"count": 0, "median": null},
	   "gate": {"records": 0, "fix": 0, "needs_human": 0, "rate": null}, "review_rounds": {"count": 1, "median": 0}}]`), &wantP); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Profiles, wantP) {
		t.Errorf("profiles:\n got %s\nwant %s", js(t, got.Profiles), js(t, wantP))
	}
	srcs := map[string]int{}
	for _, s := range got.Skipped {
		srcs[s.Source] = s.Line
	}
	if line, ok := srcs["otel/roles/chief-of-staff/2026-11-02.traces.jsonl"]; !ok || line != 3 {
		t.Errorf("壊れた Chief of Staff の行が飛ばした記録に無い: %v", got.Skipped)
	}
	if _, ok := srcs["kanban/issues/S-0007/library/outcome.json"]; !ok {
		t.Errorf("壊れた outcome.json が飛ばした記録に無い: %v", got.Skipped)
	}
	for _, s := range got.Skipped {
		if strings.Contains(s.Source, otel) || strings.Contains(s.Reason, otel) {
			t.Errorf("飛ばした記録に絶対パスがある: %+v", s)
		}
	}
}

func TestBreakdownSectionWithoutRecords(t *testing.T) {
	root, _ := breakdownLedger(t)
	res, err := Measure(breakdownOpts(root, filepath.Join(t.TempDir(), "none")))
	if err != nil {
		t.Fatal(err)
	}
	if c := res.Breakdown.ChiefOfStaff; c.Days != 0 || c.Tokens != nil || c.Share != nil {
		t.Errorf("記録の無い Chief of Staff は 0 日・null: %+v", c)
	}
	// 置き場を渡さない（Options.Otel が空）ときも読まない
	res, err = Measure(breakdownOpts(root, ""))
	if err != nil {
		t.Fatal(err)
	}
	if c := res.Breakdown.ChiefOfStaff; c.Days != 0 || c.Tokens != nil {
		t.Errorf("置き場が空なら読まない: %+v", c)
	}
	empty, err := Measure(Options{Root: emptyRoot(t), Since: day("2026-11-01"), Until: day("2026-11-02"), Now: day("2026-11-09")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contract.Encode(contract.SchemaMeasure, empty); err != nil {
		t.Fatalf("空の kanban の出力が schema に通らない: %v", err)
	}
	if e := empty.Breakdown; e.Issues != 0 || e.None != 0 || len(e.Roles) != 0 || len(e.Tools) != 0 || len(empty.Profiles) != 0 {
		t.Errorf("空の kanban の内訳とプロファイルは空: %+v %+v", e, empty.Profiles)
	}
}

func TestBreakdownTable(t *testing.T) {
	root, otel := breakdownLedger(t)
	res, err := Measure(breakdownOpts(root, otel))
	if err != nil {
		t.Fatal(err)
	}
	var w bytes.Buffer
	Table(&w, res, columnsOrder(res))
	out := w.String()
	for _, line := range []string{
		"  片付いた課題                   5件・内訳の無い課題 3件\n",
		"  担当の待ち（合計。1件）        問い 0.0 時間・レビュー 0.5 時間・CI 0.2 時間・端末が死んでいた間 0.0 時間\n",
		"  worker         1件（トークンの記録 1件）入力 1000・出力 500・キャッシュ読み 8000・キャッシュ書き 500（キャッシュ読み 84%）・失敗 モデル 1・ツール 2\n",
		"  reviewer       1件（トークンの記録 1件）入力 200・出力 100・キャッシュ読み 1500・キャッシュ書き 200（キャッシュ読み 79%）・失敗 モデル 0・ツール 1\n",
		"  chief-of-staff 2日分 入力 11・出力 22・キャッシュ読み 330・キャッシュ書き 44（全体の 3%）\n",
		"    Bash go              6回・0.0 時間・失敗 1\n",
		"  claude         2件・実働 90（2件）・トークン 10000（1件）・gate の差し戻し 50%（1 / 2）・レビューの周回 0.5（2件）\n",
		"  codex          1件・実働 30（1件）・トークン 記録なし・gate の差し戻し 記録なし・レビューの周回 0（1件）\n",
		"  プロファイル無し 1件・実働 10（1件）・トークン 記録なし・gate の差し戻し 記録なし・レビューの周回 0（1件）\n",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("表に %q が無い:\n%s", line, out)
		}
	}
	// 通知の節が最後の節のまま
	if i, j := strings.Index(out, "\nプロファイル（"), strings.Index(out, "\n通知（"); i < 0 || j < i {
		t.Errorf("プロファイルの節は通知の節の前: %s", out)
	}
}

func js(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func emptyRoot(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "empty")
	put(t, filepath.Join(root, "kanban", "config.yaml"), "version: 3\n")
	return root
}

// 読み切れない Chief of Staff の記録（ファイルの代わりにフォルダ）は飛ばした記録に出し、理由にも絶対パスを書かない。
func TestBreakdownUnreadableRoleRecordHasNoAbsolutePath(t *testing.T) {
	root, otel := breakdownLedger(t)
	if err := os.MkdirAll(filepath.Join(otel, "roles", "chief-of-staff", "2026-11-01.traces.jsonl.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(otel, "roles", "chief-of-staff", "2026-11-01.traces.jsonl")
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Measure(breakdownOpts(root, otel))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range res.Skipped {
		if s.Source == "otel/roles/chief-of-staff/2026-11-01.traces.jsonl" {
			found = true
		}
		if strings.Contains(s.Source, otel) || strings.Contains(s.Reason, otel) || strings.Contains(s.Reason, "/") {
			t.Errorf("飛ばした記録に絶対パスがある: %+v", s)
		}
	}
	if !found {
		t.Errorf("読めない Chief of Staff の記録が飛ばした記録に無い: %+v", res.Skipped)
	}
	if c := res.Breakdown.ChiefOfStaff; c.Days != 1 {
		t.Errorf("読めた日だけを数える（11-02 の 1 日）: %+v", c)
	}
}

// 形の合わない breakdown（schema/outcome.schema.json を通らない）は合わせず、飛ばした記録に出して内訳の無い課題に数える。
// 出力全体は schema を通る（1 件の記録のせいで measure が止まらない）。
func TestBreakdownMalformedIsSkippedNotZero(t *testing.T) {
	for name, b := range map[string]string{
		"空の役":       `{"role": "", "tokens": {"input": 1}}`,
		"欠けた内訳":     `{"role": "worker"}`,
		"空の object": `{}`,
		"名前の無いツール": `{"role": "worker", "coverage": {"sessions": 1, "traced": 1}, "interactions": 1, "llm_ms": 1, "tool_ms": 1, "blocked_ms": 0,
		  "tokens": {"input": 1, "output": 1, "cache_read": 1, "cache_write": 1}, "errors": {"llm": 0, "tool": 0},
		  "tools": [{"name": "", "argv0": null, "count": 1, "ms": 1, "errors": 0}],
		  "waits": {"question_ms": 0, "review_ms": 0, "ci_ms": 0, "gone_ms": 0}}`,
	} {
		t.Run(name, func(t *testing.T) {
			root, otel := breakdownLedger(t)
			put(t, filepath.Join(root, "kanban/issues/S-0003/library/outcome.json"),
				`{"at": "2026-11-01T06:00:00Z", "issue": "S-0003", "result": "none", "pr": null, "merge_commit": null, "merged_at": null,
				  "gates": [], "reviewers": 0, "worked_minutes": 60, "sessions": null, "breakdown": `+b+`}`)
			res, err := Measure(breakdownOpts(root, otel))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := contract.Encode(contract.SchemaMeasure, res); err != nil {
				t.Fatalf("1 件の壊れた内訳で出力が schema を通らない: %v", err)
			}
			if res.Breakdown.None != 3 || len(res.Breakdown.Roles) != 2 || res.Breakdown.Roles[0].Issues != 1 {
				t.Errorf("壊れた内訳は内訳の無い課題に数える: none %d, roles %+v", res.Breakdown.None, res.Breakdown.Roles)
			}
			if _, ok := skippedSources(res)["kanban/issues/S-0003/library/outcome.json"]; !ok {
				t.Errorf("壊れた内訳が飛ばした記録に無い: %+v", res.Skipped)
			}
		})
	}
}

// モデルの呼び出しの記録が無い内訳（Codex の課題。受け口は Codex の span をトークンに数えない）は、課題の数には入れるが、
// トークンの数え（役のトークンの課題の数・プロファイルのトークンの中央値）には 0 として入れない。
func TestBreakdownWithoutModelCallsIsNotZeroTokens(t *testing.T) {
	root, otel := breakdownLedger(t)
	codex := `{"role": "worker", "coverage": {"sessions": 1, "traced": 1}, "interactions": 0, "llm_ms": 0, "tool_ms": 0, "blocked_ms": 0,
	  "tokens": {"input": 0, "output": 0, "cache_read": 0, "cache_write": 0}, "errors": {"llm": 0, "tool": 0}, "tools": [],
	  "waits": {"question_ms": 0, "review_ms": 0, "ci_ms": 0, "gone_ms": 0}}`
	put(t, filepath.Join(root, "kanban/issues/S-0002/library/outcome.json"), `{"issue": "S-0002", "breakdown": `+codex+`}`)
	res, err := Measure(breakdownOpts(root, otel))
	if err != nil {
		t.Fatal(err)
	}
	w := res.Breakdown.Roles[0]
	if w.Issues != 2 || w.TokenIssues != 1 {
		t.Errorf("担当の行は課題 2・トークンを数えた課題 1: %+v", w)
	}
	for _, p := range res.Profiles {
		if p.Profile != nil && *p.Profile == "codex" && (p.Tokens.Count != 0 || p.Tokens.Median != nil) {
			t.Errorf("モデルの呼び出しの記録の無い codex の課題をトークン 0 と数えた: %+v", p.Tokens)
		}
	}
}

// 時間を使ったツールは上から 10 個で切る。Chief of Staff の記録は期間の終わりの日の翌日のものを数えない。
func TestBreakdownToolsTopTenAndPeriodEnd(t *testing.T) {
	root, otel := breakdownLedger(t)
	var tools []string
	for i := 1; i <= 12; i++ {
		tools = append(tools, fmt.Sprintf(`{"name": "T%02d", "argv0": null, "count": 1, "ms": %d, "errors": 0}`, i, 1000+i))
	}
	put(t, filepath.Join(root, "kanban/issues/S-0003/library/outcome.json"), `{"issue": "S-0003", "breakdown": {"role": "worker",
	  "coverage": {"sessions": 1, "traced": 1}, "interactions": 1, "llm_ms": 1, "tool_ms": 1, "blocked_ms": 0,
	  "tokens": {"input": 1, "output": 1, "cache_read": 1, "cache_write": 1}, "errors": {"llm": 0, "tool": 0},
	  "tools": [`+strings.Join(tools, ",")+`], "waits": {"question_ms": 0, "review_ms": 0, "ci_ms": 0, "gone_ms": 0}}}`)
	put(t, filepath.Join(otel, "roles", "chief-of-staff", "2026-11-03.traces.jsonl"), cosSpan("c", "1", 7000, 7000, 7000, 7000)+"\n")
	res, err := Measure(breakdownOpts(root, otel))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(res.Breakdown.Tools); n != 10 || res.Breakdown.Tools[0].Name != "T12" || res.Breakdown.Tools[9].Name != "T03" {
		t.Errorf("ツールは時間の長い順に上から 10 個: %d 個 %+v", n, res.Breakdown.Tools)
	}
	if c := res.Breakdown.ChiefOfStaff; c.Days != 2 || c.Tokens == nil || c.Tokens.Input != 11 {
		t.Errorf("期間の翌日（11-03）の Chief of Staff の記録を数えた: %+v %+v", c, c.Tokens)
	}
}
