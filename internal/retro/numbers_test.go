package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"andon"
	"andon/internal/contract"
	"andon/internal/ledger"
	"andon/internal/ledger/ledgertest"
)

// 振り返りの入力の内訳と、直しのマージの前後の数字（ADR-0060 決めたこと 17・受け入れ条件 13）。

// bdJSON は内訳 1 件（入力のトークン in・モデルの呼び出しの時間 llm）。
func bdJSON(in, llm int) string {
	return fmt.Sprintf(`{"role":"worker","coverage":{"sessions":1,"traced":1},"interactions":2,"llm_ms":%d,"tool_ms":10,"blocked_ms":0,`+
		`"tokens":{"input":%d,"output":0,"cache_read":0,"cache_write":0},"errors":{"llm":0,"tool":0},`+
		`"tools":[{"name":"Bash","argv0":"go","count":1,"ms":10,"errors":0}],"waits":{"question_ms":0,"review_ms":0,"ci_ms":0,"gone_ms":0}}`, llm, in)
}

// baKanban は、振り返りの課題 A-0100 が切った直し A-0101（マージは 5 日前）と、マージの前に before 件・後に after 件の
// 担当の課題がある偽の kanban を作る。前の課題は実働 10 分・内訳のトークン 100・gate の記録 1 件（fix）・レビュアー 1 人、
// 後の課題は実働 20 分・トークン 200・gate の記録 1 件（approve）・レビュアー無し。マージから遠い課題ほど値を大きくずらす
// （近い 30 件だけを数えたかが中央値に出る）。
func baKanban(t *testing.T, before, after int) (string, time.Time) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	is := func(id string) string { return filepath.Join(k, "issues", id) }
	write(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: A\n")
	merged := now.Add(-5 * day)
	var log []string
	done := func(id string, at time.Time) {
		log = append(log, fmt.Sprintf(`{"at":%q,"by":"terminal:t1","id":%q,"change":{"status":"done"}}`, at.Format(time.RFC3339), id))
	}
	write(t, filepath.Join(is("A-0100"), "issue.yaml"), issueYAML("A-0100", "done", "profile: claude\nroutine: retro\n"))
	done("A-0100", merged.Add(-20*day))
	write(t, filepath.Join(is("A-0101"), "issue.yaml"), issueYAML("A-0101", "done", "profile: claude\nparent: A-0100\n"))
	write(t, filepath.Join(is("A-0101"), "library", "outcome.json"), `{"result":"merged","pr":"`+pr7+`","merged_at":"`+merged.Format(time.RFC3339)+`","worked_minutes":5}`)
	done("A-0101", merged)
	worker := func(id string, at time.Time, worked, tokens int, decision string, reviewed bool) {
		write(t, filepath.Join(is(id), "issue.yaml"), issueYAML(id, "done", "profile: claude\n"))
		write(t, filepath.Join(is(id), "library", "outcome.json"), fmt.Sprintf(`{"result":"merged","worked_minutes":%d,"breakdown":%s}`, worked, bdJSON(tokens, 1000)))
		write(t, filepath.Join(is(id), "library", "gate", sha1+".json"), `{"head":"`+sha1+`","at":"`+at.Format(time.RFC3339)+`","tier":"B","decision":"`+decision+`","reasons":[],"merged":true}`)
		done(id, at)
		if reviewed {
			r := id + "-r1"
			write(t, filepath.Join(is(r), "issue.yaml"), issueYAML(r, "done", "profile: codex-review\nparent: "+id+"\n"))
			done(r, at)
		}
	}
	for i := 1; i <= before; i++ { // i が大きいほどマージから遠い
		worked, tokens := 10, 100
		if i > sideMax {
			worked, tokens = 1000, 100000
		}
		worker(fmt.Sprintf("A-%04d", 200+i), merged.Add(-time.Duration(i)*time.Hour), worked, tokens, "fix", true)
	}
	for i := 1; i <= after; i++ {
		worked, tokens := 20, 200
		if i > sideMax {
			worked, tokens = 1000, 100000
		}
		worker(fmt.Sprintf("A-%04d", 400+i), merged.Add(time.Duration(i)*time.Hour), worked, tokens, "approve", false)
	}
	// マージと同じ時刻に片付いた課題は、前にも後にも入れない
	worker("A-0500", merged, 999, 999, "fix", true)
	write(t, filepath.Join(k, "log.jsonl"), ledgertest.Log(t, log...))
	return root, merged
}

func collectFix(t *testing.T, root string) (*Input, Fix) {
	t.Helper()
	in, err := Collect(Options{Root: root, Now: now, Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Redacted(in); err != nil {
		t.Fatalf("出力が schema/retro.schema.json に合わない: %v", err)
	}
	if len(in.Fixes) != 1 || in.Fixes[0].Issue != "A-0101" {
		t.Fatalf("前の直しが A-0101 の 1 件でない: %+v", in.Fixes)
	}
	return in, in.Fixes[0]
}

func f64(v float64) *float64 { return &v }

// 受け入れ条件 13: マージの前に 12 件、後に 10 件の担当の課題があれば、その直しに前後の数字が出る。
func TestBeforeAfterNumbers(t *testing.T) {
	root, _ := baKanban(t, 12, 10)
	_, f := collectFix(t, root)
	ba := f.BeforeAfter
	if ba.Short != nil {
		t.Fatalf("12 件と 10 件で数字が出ない: %s", *ba.Short)
	}
	if ba.Before.Issues != 12 || ba.After.Issues != 10 {
		t.Fatalf("前後の件数が 12 と 10 でない: %d・%d", ba.Before.Issues, ba.After.Issues)
	}
	want := map[string]*Numbers{
		"before": {WorkedMinutes: Median{12, f64(10)}, Tokens: Median{12, f64(100)}, Gate: GateStats{Records: 12, Fix: 12, Rate: f64(1)}, ReviewRounds: Median{12, f64(1)}},
		"after":  {WorkedMinutes: Median{10, f64(20)}, Tokens: Median{10, f64(200)}, Gate: GateStats{Records: 10, Rate: f64(0)}, ReviewRounds: Median{10, f64(0)}},
	}
	for side, got := range map[string]*Numbers{"before": ba.Before.Numbers, "after": ba.After.Numbers} {
		if !reflect.DeepEqual(got, want[side]) {
			g, _ := json.Marshal(got)
			w, _ := json.Marshal(want[side])
			t.Errorf("%s の数字が違う:\n got %s\nwant %s", side, g, w)
		}
	}
	if f.IssuesAfter != 10 {
		t.Errorf("issues_after が 10 でない: %d", f.IssuesAfter)
	}
}

// 受け入れ条件 13: 後が 9 件なら、数字が無く、件数が足りないという理由が出る。
func TestBeforeAfterShort(t *testing.T) {
	root, _ := baKanban(t, 12, 9)
	_, f := collectFix(t, root)
	ba := f.BeforeAfter
	if ba.Before.Numbers != nil || ba.After.Numbers != nil {
		t.Fatalf("後が 9 件なのに数字が出た: %+v", ba)
	}
	if ba.Before.Issues != 12 || ba.After.Issues != 9 {
		t.Fatalf("前後の件数が 12 と 9 でない: %d・%d", ba.Before.Issues, ba.After.Issues)
	}
	if ba.Short == nil || !strings.Contains(*ba.Short, "後が 9 件") || !strings.Contains(*ba.Short, "10 件に満たない") || strings.Contains(*ba.Short, "前が") {
		t.Fatalf("件数が足りないという理由が無いか、後のことだけを言っていない: %v", ba.Short)
	}
	root, _ = baKanban(t, 9, 12)
	if _, f := collectFix(t, root); f.BeforeAfter.Short == nil || !strings.Contains(*f.BeforeAfter.Short, "前が 9 件") || f.BeforeAfter.After.Numbers != nil {
		t.Fatalf("前が 9 件のときに理由が無いか、後の数字が出た: %+v", f.BeforeAfter)
	}
}

// 前後はマージに近いほうから 30 件まで。遠い課題の値は中央値に入らない。
func TestBeforeAfterNearest30(t *testing.T) {
	root, _ := baKanban(t, 35, 33)
	_, f := collectFix(t, root)
	ba := f.BeforeAfter
	if ba.Before.Issues != sideMax || ba.After.Issues != sideMax || ba.Short != nil {
		t.Fatalf("前後が 30 件ずつでない: %+v", ba)
	}
	if b, a := ba.Before.Numbers, ba.After.Numbers; *b.WorkedMinutes.Median != 10 || *b.Tokens.Median != 100 || *a.WorkedMinutes.Median != 20 || b.WorkedMinutes.Count != 30 {
		t.Fatalf("マージから遠い課題が数字に入った: 前 %+v・後 %+v", b, a)
	}
	if f.IssuesAfter != 33 {
		t.Errorf("issues_after は 30 件で切らない: %d", f.IssuesAfter)
	}
}

// 前後の数字は、内訳の無い課題と、モデルの呼び出しの記録の無い内訳（Codex）をトークンに数えず、実働の無い課題を実働に数えない。
func TestBeforeAfterSkipsMissing(t *testing.T) {
	root, _ := baKanban(t, 10, 10)
	lib := func(i int) string {
		return filepath.Join(root, "kanban", "issues", fmt.Sprintf("A-%04d", 400+i), "library")
	}
	write(t, filepath.Join(lib(1), "outcome.json"), `{"result":"merged","breakdown":null}`)
	codex := strings.Replace(strings.Replace(bdJSON(0, 0), `"interactions":2`, `"interactions":0`, 1), `"role":"worker"`, `"role":"reviewer"`, 1)
	write(t, filepath.Join(lib(2), "outcome.json"), `{"result":"merged","worked_minutes":20,"breakdown":`+codex+`}`)
	write(t, filepath.Join(lib(3), "outcome.json"), `{"result":"merged","worked_minutes":20,"breakdown":{"role":"worker"}}`)
	in, f := collectFix(t, root)
	a := f.BeforeAfter.After.Numbers
	if a == nil || a.WorkedMinutes.Count != 9 || a.Tokens.Count != 7 {
		t.Fatalf("実働は 9 件、トークンは 7 件を数えるはず: %+v", a)
	}
	found := false
	for _, s := range in.Skipped {
		if strings.HasSuffix(s.Source, filepath.Join("A-0403", "library", "outcome.json")) && strings.Contains(s.Reason, "breakdown が schema/outcome.schema.json の形でない") {
			if found {
				t.Errorf("同じ飛ばした記録が 2 行ある: %+v", in.Skipped)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("形の違う内訳が飛ばした記録に出ない: %+v", in.Skipped)
	}
}

// gate の記録の置き場が読めないとき、記録 0 件と黙って数えず、飛ばした記録に出す（差し戻しの率が黙って変わらない）。
func TestBeforeAfterUnreadableGates(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root は権限の無い置き場も読める")
	}
	root, _ := baKanban(t, 10, 10)
	dir := filepath.Join(root, "kanban", "issues", "A-0201", "library", "gate")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	in, _ := collectFix(t, root)
	for _, s := range in.Skipped {
		if strings.HasSuffix(s.Source, filepath.Join("A-0201", "library", "gate")) && strings.Contains(s.Reason, "読めない") {
			return
		}
	}
	t.Fatalf("読めない gate の置き場が飛ばした記録に出ない: %+v", in.Skipped)
}

// 全文の束に、結果の記録の内訳をそのまま写す。無ければ null、形が違えば null と飛ばした記録の 1 行。
func TestBundleBreakdown(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sample-site")
	k := filepath.Join(root, "kanban")
	write(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: A\n")
	var log []string
	for i, body := range []string{
		`{"result":"merged","worked_minutes":3,"breakdown":` + strings.TrimSuffix(strings.Replace(bdJSON(9223372036854775807, 5), `"output":0`, `"output":7`, 1), "}") +
			`,"knowledge":{"index_reads":1,"pages":[{"path":"topics/tool-quirks.md","reads":2}]}}}`,
		`{"result":"merged","worked_minutes":3,"breakdown":null}`,
		`{"result":"merged","worked_minutes":3}`,
		`{"result":"merged","worked_minutes":3,"breakdown":{"role":"x"}}`,
		``,
	} {
		id := fmt.Sprintf("A-%04d", i+1)
		write(t, filepath.Join(k, "issues", id, "issue.yaml"), issueYAML(id, "done", "profile: claude\n"))
		if body != "" {
			write(t, filepath.Join(k, "issues", id, "library", "outcome.json"), body)
		}
		log = append(log, fmt.Sprintf(`{"at":%q,"by":"terminal:t1","id":%q,"change":{"status":"done"}}`, ago(day), id))
	}
	write(t, filepath.Join(k, "log.jsonl"), ledgertest.Log(t, log...))
	in, err := Collect(Options{Root: root, Now: now, Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := Redacted(in)
	if err != nil {
		t.Fatalf("出力が schema に合わない: %v", err)
	}
	var v struct {
		Issues []struct {
			ID        string          `json:"id"`
			Breakdown json.RawMessage `json:"breakdown"`
		} `json:"issues"`
		Skipped []Skip `json:"skipped"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, b := range v.Issues {
		got[b.ID] = string(b.Breakdown)
	}
	// 上限で止めた数を float64 に落とさずに写す
	if !strings.Contains(got["A-0001"], `"input": 9223372036854775807`) || !strings.Contains(got["A-0001"], `"output": 7`) || !strings.Contains(got["A-0001"], `"role": "worker"`) {
		t.Errorf("A-0001 の内訳が写っていない: %s", got["A-0001"])
	}
	// wiki の読み（knowledge）も写す（retro の schema が knowledge を知らないと、Redacted で collect の全体が落ちる）
	if !strings.Contains(got["A-0001"], `"path": "topics/tool-quirks.md"`) || !strings.Contains(got["A-0001"], `"index_reads": 1`) {
		t.Errorf("A-0001 の内訳に wiki の読みが写っていない: %s", got["A-0001"])
	}
	for _, id := range []string{"A-0002", "A-0003", "A-0004", "A-0005"} {
		if got[id] != "null" {
			t.Errorf("%s の内訳が null でない: %s", id, got[id])
		}
	}
	n := 0
	for _, s := range v.Skipped {
		if strings.Contains(s.Source, "A-0004") && strings.Contains(s.Reason, "breakdown が schema/outcome.schema.json の形でない") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("形の違う内訳の飛ばした記録が 1 行でない（%d 行）: %+v", n, v.Skipped)
	}
}

// 期間の Chief of Staff のトークンは、役ごとの生の記録から数え、数えられた日数を添える。記録が無ければ null と 0 日。
func TestChiefOfStaffTokens(t *testing.T) {
	root, _ := baKanban(t, 0, 0)
	in, err := Collect(Options{Root: root, Now: now, Days: 7, Otel: filepath.Join(t.TempDir(), "none")})
	if err != nil {
		t.Fatal(err)
	}
	if in.ChiefOfStaff.Tokens != nil || in.ChiefOfStaff.Days != 0 {
		t.Fatalf("記録が無いのに数えた: %+v", in.ChiefOfStaff)
	}
	otelDir := t.TempDir()
	span := func(id string, in, out int) string {
		return fmt.Sprintf(`{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"t1","spanId":%q,"name":"claude_code.llm_request","attributes":[`+
			`{"key":"input_tokens","value":{"intValue":"%d"}},{"key":"output_tokens","value":{"intValue":"%d"}}]}]}]}]}`+"\n", id, in, out)
	}
	dir := filepath.Join(otelDir, "roles", "chief-of-staff")
	write(t, filepath.Join(dir, now.Add(-2*day).Format("2006-01-02")+".traces.jsonl"), span("s1", 5, 6))
	write(t, filepath.Join(dir, now.Format("2006-01-02")+".traces.jsonl"), span("s2", 1, 2)+"{壊れた行\n")
	write(t, filepath.Join(dir, now.Add(-20*day).Format("2006-01-02")+".traces.jsonl"), span("s3", 100, 100)) // 期間の外
	in, err = Collect(Options{Root: root, Now: now, Days: 7, Otel: otelDir})
	if err != nil {
		t.Fatal(err)
	}
	c := in.ChiefOfStaff
	if c.Days != 2 || c.Tokens == nil || c.Tokens.Input != 6 || c.Tokens.Output != 8 {
		t.Fatalf("Chief of Staff のトークンが 2 日分の入力 6・出力 8 でない: %+v %+v", c, c.Tokens)
	}
	if _, _, err := Redacted(in); err != nil {
		t.Fatalf("出力が schema に合わない: %v", err)
	}
	found := false
	for _, s := range in.Skipped {
		found = found || (strings.HasPrefix(s.Source, "otel/roles/chief-of-staff/") && strings.HasSuffix(s.Source, ":2"))
	}
	if !found {
		t.Errorf("壊れた行が飛ばした記録に出ない: %+v", in.Skipped)
	}
}

// 振り返りの入力の schema の内訳は、結果の記録の schema の内訳の写し。片方だけを直すと、写した内訳が入力の schema に落ちる。
func TestRetroBreakdownSchemaIsOutcomeCopy(t *testing.T) {
	defs := func(s contract.Schema) map[string]json.RawMessage {
		b, err := andon.Schemas.ReadFile(s.File())
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Defs map[string]json.RawMessage `json:"$defs"`
		}
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v.Defs
	}
	r, o := defs(contract.SchemaRetro), defs(contract.SchemaOutcome)
	for _, k := range []string{"breakdown", "tool", "page_read"} {
		var a, b any
		if json.Unmarshal(r[k], &a) != nil || json.Unmarshal(o[k], &b) != nil || !reflect.DeepEqual(a, b) {
			t.Errorf("schema/retro.schema.json の $defs.%s が schema/outcome.schema.json の写しでない", k)
		}
	}
}

// andon retro patterns は、期間の Chief of Staff のトークンを retro.json に残す（collect の出力は保存しないので、
// 生の記録が 30 日で消えたあとも、この数を振り返りの課題のフォルダから引けるように。ADR-0060 決めたこと 17）。
func TestPatternsKeepsChiefOfStaffTokens(t *testing.T) {
	if _, err := exec.LookPath(ledger.CLI); err != nil {
		t.Skipf("%s が無い（CI の go の job。tests/andon-retro-apply.sh が同じことを見る）", ledger.CLI)
	}
	root, _ := baKanban(t, 0, 0)
	k := filepath.Join(root, "kanban")
	write(t, filepath.Join(k, "issues", "A-0900", "issue.yaml"), issueYAML("A-0900", "doing", "profile: claude\nroutine: retro\nagent: t9\n"))
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("HC_TERMINAL_ID", "t9")
	day0 := time.Now().UTC().Format("2006-01-02")
	write(t, filepath.Join(state, "andon", "otel", "sample-site", "roles", "chief-of-staff", day0+".traces.jsonl"),
		`{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"t1","spanId":"s1","name":"claude_code.llm_request","attributes":[`+
			`{"key":"input_tokens","value":{"intValue":"5"}},{"key":"output_tokens","value":{"intValue":"6"}}]}]}]}]}`+"\n")
	in := `{"patterns": [], "fixes": [{"issue": "A-0101", "recurred": 0, "view": "前後の件数が足りず、数字は無い。残す"}]}`
	var out strings.Builder
	if err := runPatterns(root, "A-0900", 7, "-", strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(k, "issues", "A-0900", "library", retroFile))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		ChiefOfStaff *ChiefOfStaff `json:"chief_of_staff"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if c := v.ChiefOfStaff; c == nil || c.Days != 1 || c.Tokens == nil || c.Tokens.Input != 5 || c.Tokens.Output != 6 {
		t.Fatalf("retro.json に Chief of Staff のトークン（1 日分・入力 5・出力 6）が無い: %s", b)
	}
	if _, err := ParsePatterns(b); err != nil {
		t.Fatalf("書いた retro.json を読み直せない: %v", err)
	}
	// 前のバージョンが書いた retro.json（chief_of_staff が無い）も読める
	var old map[string]any
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	delete(old, "chief_of_staff")
	ob, _ := json.Marshal(old)
	if p, err := ParsePatterns(ob); err != nil || p.ChiefOfStaff != nil {
		t.Fatalf("chief_of_staff の無い retro.json を読めない: %v", err)
	}
}
