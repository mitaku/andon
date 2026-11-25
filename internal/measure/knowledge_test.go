package measure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger/ledgertest"
)

// knowledgeBreakdown は knowledge を持つ内訳（knowledge が空の文字列なら持たない。Codex の内訳の形）。
func knowledgeBreakdown(role, knowledge string) string {
	k := ""
	if knowledge != "" {
		k = `, "knowledge": ` + knowledge
	}
	return `{"role": "` + role + `", "coverage": {"sessions": 1, "traced": 1}, "interactions": 1, "llm_ms": 10, "tool_ms": 10, "blocked_ms": 0,
	  "tokens": {"input": 1, "output": 1, "cache_read": 1, "cache_write": 1}, "errors": {"llm": 0, "tool": 0}, "tools": [],
	  "waits": {"question_ms": 0, "review_ms": 0, "ci_ms": 0, "gone_ms": 0}` + k + `}`
}

// knowledgeLedger は受け入れ条件 10 の偽の kanban と wiki の置き場（期間 2026-11-01〜02）。
//
// 手で数えた値:
//
//	S-0001 担当。依頼書に wiki/topics/shell.md。内訳に topics/shell.md 2 回・entities/owner.md 1 回・目録 1 回。申告 missing AGENTS.md。
//	S-0002 担当。依頼書に <wiki>/topics/ci.md の絶対パス。内訳に目録 3 回だけ。申告 wrong ./AGENTS.md と hard_to_find unknown。
//	S-0003 担当。依頼書に wiki/index.md だけ（数えない）。outcome.json の breakdown が null。申告 wrong topics/shell.md と kind の壊れた申告。
//	S-0004 担当（codex）。内訳に knowledge が無い。申告 hard_to_find "" 。
//	S-0005 担当。内訳に topics/shell.md 1 回。
//	S-0005-r1 レビュアー。内訳に topics/ci.md 9 回。数えない。
//	S-0006 Chief Engineer。依頼書に wiki/topics/ci.md。申告 missing AGENTS.md。数えない。
//	S-0007 担当。期間の前に done。内訳に topics/ci.md。数えない。
//
//	→ 課題 5（S-0001〜S-0005）、依頼書 2（S-0001・S-0002）、読んだ 2（S-0001・S-0005）、目録だけ 1（S-0002）、記録なし 2（S-0003・S-0004）。
//	  ページ: topics/shell.md 3 回・2 件、entities/owner.md 1 回・1 件。
//	  読まれなかった: entities/tool.md・topics/ci.md・topics/sub/deep.md（overview.md・index.md・log.md・sources/・raw/・.md でないファイルは入らない）。
//	  申告 5（missing 1・wrong 2・hard_to_find 2。壊れた 1 件は飛ばす）。課題 4。先: AGENTS.md 2（S-0001・S-0002）、unknown 2（S-0002・S-0004）、
//	  topics/shell.md 1。繰り返し: AGENTS.md だけ（unknown は入れない）。
func knowledgeLedger(t *testing.T) (root, wiki string) {
	root = filepath.Join(t.TempDir(), "sample-site")
	wiki = filepath.Join(t.TempDir(), "wiki")
	for _, p := range []string{"index.md", "log.md", "overview.md", "entities/owner.md", "entities/tool.md", "topics/shell.md",
		"topics/ci.md", "topics/sub/deep.md", "topics/notes.txt", "sources/s1.md", "raw/r1.md"} {
		put(t, filepath.Join(wiki, p), "# x\n")
	}
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "version: 3\nprefix: S\n")
	issue := func(id, extra string) {
		put(t, filepath.Join(k, "issues", id, "issue.yaml"), "id: "+id+"\ntitle: x\nstatus: done\ncreated: \"2026-11-01T00:00:00Z\"\n"+extra)
	}
	outcome := func(id, breakdown string) {
		put(t, filepath.Join(k, "issues", id, "library", "outcome.json"), `{"issue": "`+id+`", "breakdown": `+breakdown+`}`)
	}
	report := func(id, gaps string) {
		put(t, filepath.Join(k, "issues", id, "library", "report.json"), `{"commit": "`+strings.Repeat("a", 40)+`", "gaps": [`+gaps+`]}`)
	}
	brief := func(id, body string) { put(t, filepath.Join(k, "issues", id, "brief.md"), body) }
	for _, id := range []string{"S-0001", "S-0002", "S-0003", "S-0005", "S-0007"} {
		issue(id, "profile: claude\n")
	}
	issue("S-0004", "profile: codex\n")
	issue("S-0005-r1", "profile: codex-review\nparent: S-0005\n")
	issue("S-0006", "profile: chief-engineer\n")

	brief("S-0001", "先に `wiki/topics/shell.md` を読む。\n")
	outcome("S-0001", knowledgeBreakdown("worker", `{"index_reads": 1, "pages": [{"path": "topics/shell.md", "reads": 2}, {"path": "entities/owner.md", "reads": 1}]}`))
	report("S-0001", `{"kind": "missing", "where": "AGENTS.md", "text": "x"}`)

	brief("S-0002", "参考: "+filepath.Join(wiki, "topics/ci.md")+"\n")
	outcome("S-0002", knowledgeBreakdown("worker", `{"index_reads": 3, "pages": []}`))
	report("S-0002", `{"kind": "wrong", "where": "./AGENTS.md", "text": "x"}, {"kind": "hard_to_find", "where": "unknown", "text": "y"}`)

	brief("S-0003", "目録 wiki/index.md を読む。\n")
	outcome("S-0003", "null")
	report("S-0003", `{"kind": "wrong", "where": "topics/shell.md", "text": "x"}, {"kind": "stale", "where": "AGENTS.md", "text": "y"}`)

	outcome("S-0004", knowledgeBreakdown("worker", ""))
	report("S-0004", `{"kind": "hard_to_find", "where": " ", "text": "x"}`)

	outcome("S-0005", knowledgeBreakdown("worker", `{"index_reads": 0, "pages": [{"path": "topics/shell.md", "reads": 1}]}`))
	outcome("S-0005-r1", knowledgeBreakdown("reviewer", `{"index_reads": 0, "pages": [{"path": "topics/ci.md", "reads": 9}]}`))

	brief("S-0006", "wiki/topics/ci.md\n")
	report("S-0006", `{"kind": "missing", "where": "AGENTS.md", "text": "x"}`)

	outcome("S-0007", knowledgeBreakdown("worker", `{"index_reads": 0, "pages": [{"path": "topics/ci.md", "reads": 1}]}`))

	put(t, filepath.Join(k, "log.jsonl"), ledgertest.Log(t,
		`{"at":"2026-10-30T01:00:00Z","by":"terminal:t7","id":"S-0007","change":{"status":"doing","agent":"t7"}}`,
		`{"at":"2026-10-30T02:00:00Z","by":"terminal:t7","id":"S-0007","change":{"status":"done"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"doing","agent":"t1"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t2","id":"S-0002","change":{"status":"doing","agent":"t2"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t3","id":"S-0003","change":{"status":"doing","agent":"t3"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t4","id":"S-0004","change":{"status":"doing","agent":"t4"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t5","id":"S-0005","change":{"status":"doing","agent":"t5"}}`,
		`{"at":"2026-11-01T01:00:00Z","by":"terminal:t6","id":"S-0006","change":{"status":"doing","agent":"t6"}}`,
		`{"at":"2026-11-01T02:00:00Z","by":"terminal:dispatcher","id":"S-0005-r1","change":{"add":"review"}}`,
		`{"at":"2026-11-01T02:30:00Z","by":"terminal:r1","id":"S-0005-r1","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t2","id":"S-0002","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t3","id":"S-0003","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t4","id":"S-0004","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t5","id":"S-0005","change":{"status":"done"}}`,
		`{"at":"2026-11-01T03:00:00Z","by":"terminal:t6","id":"S-0006","change":{"status":"done"}}`,
	))
	return root, wiki
}

func knowledgeOpts(root, wiki string) Options {
	return Options{Root: root, Since: day("2026-11-01"), Until: day("2026-11-02"), Now: day("2026-11-09"), Wiki: wiki,
		GH: func(string) ([]byte, error) { return []byte(`[]`), nil }}
}

// 受け入れ条件 10（ADR-0061）: 依頼書に載せた数・読んだ数・申告の数。目録だけの課題は読んだ数に入らず、内訳が null の課題は記録なしに入る。
// 同じ先への申告が 2 件あると繰り返しに出る。出力が schema/measure.schema.json に通る。
func TestKnowledgeSectionMatchesHandCount(t *testing.T) {
	root, wiki := knowledgeLedger(t)
	res, err := Measure(knowledgeOpts(root, wiki))
	if err != nil {
		t.Fatal(err)
	}
	b, err := contract.Encode(contract.SchemaMeasure, res)
	if err != nil {
		t.Fatalf("schema/measure.schema.json に通らない: %v", err)
	}
	var got struct {
		Knowledge map[string]any `json:"knowledge"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(`{"issues": 5, "briefed": 2,
	  "read": {"issues": 2, "index_only": 1, "no_record": 2},
	  "pages": [{"path": "topics/shell.md", "reads": 3, "issues": 2}, {"path": "entities/owner.md", "reads": 1, "issues": 1}],
	  "unread": ["entities/tool.md", "topics/ci.md", "topics/sub/deep.md"],
	  "gaps": {"count": 5, "issues": 4, "by_kind": {"missing": 1, "wrong": 2, "hard_to_find": 2},
	    "wheres": [{"where": "AGENTS.md", "count": 2, "issues": ["S-0001", "S-0002"]},
	               {"where": "unknown", "count": 2, "issues": ["S-0002", "S-0004"]},
	               {"where": "topics/shell.md", "count": 1, "issues": ["S-0003"]}],
	    "repeated": [{"where": "AGENTS.md", "count": 2, "issues": ["S-0001", "S-0002"]}]}}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Knowledge, want) {
		g, _ := json.MarshalIndent(got.Knowledge, "", "  ")
		t.Fatalf("知識の節が手で数えた値と違う:\n%s", g)
	}
	found := false
	for _, s := range res.Skipped {
		if strings.HasSuffix(s.Source, "S-0003/library/report.json") && strings.Contains(s.Reason, "gaps[1]") {
			found = true
		}
	}
	if !found {
		t.Fatalf("kind の壊れた申告が飛ばした記録に出ない: %+v", res.Skipped)
	}
	var out bytes.Buffer
	Table(&out, res, columnsOrder(res))
	for _, s := range []string{"知識（", "依頼書に wiki のページを載せた課題 2件 / 5件", "wiki のページを読んだ課題      2件・目録だけ 1件・読みの記録なし 2件",
		"topics/shell.md", "1 回も読まれなかった Topics と Entities のページ  3枚", "AGENTS.md 2件（繰り返し）", "unknown 2件 S-0002"} {
		if !strings.Contains(out.String(), s) {
			t.Fatalf("表に %q が無い:\n%s", s, out.String())
		}
	}
}

// 読みの記録が 1 件も無い期間と wiki の置き場が無いときは、読まれなかったページを null（記録なし）にする。全部を読まれなかったと出さない。
func TestKnowledgeUnreadIsNullWithoutRecords(t *testing.T) {
	root, wiki := knowledgeLedger(t)
	t.Run("置き場が無い", func(t *testing.T) {
		res, err := Measure(knowledgeOpts(root, filepath.Join(t.TempDir(), "none")))
		if err != nil {
			t.Fatal(err)
		}
		if res.Knowledge.Unread != nil {
			t.Fatalf("置き場が無いのに一覧がある: %v", res.Knowledge.Unread)
		}
		if res.Knowledge.Read.Issues != 2 {
			t.Fatalf("読んだ課題は置き場に依らず内訳から数える: %+v", res.Knowledge.Read)
		}
	})
	t.Run("読みの記録が無い", func(t *testing.T) {
		k := filepath.Join(root, "kanban", "issues")
		for _, id := range []string{"S-0001", "S-0002", "S-0005"} {
			put(t, filepath.Join(k, id, "library", "outcome.json"), `{"issue": "`+id+`", "breakdown": null}`)
		}
		res, err := Measure(knowledgeOpts(root, wiki))
		if err != nil {
			t.Fatal(err)
		}
		if res.Knowledge.Unread != nil || res.Knowledge.Read.NoRecord != 5 {
			t.Fatalf("記録の無い期間に一覧を出した: %+v", res.Knowledge)
		}
		b, err := contract.Encode(contract.SchemaMeasure, res)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"unread": null`) {
			t.Fatalf("unread が null でない: %s", b)
		}
	})
}

// 読まれたページは上から 10 個に切る（同じ回数は名前の順）。
func TestKnowledgePagesTopTen(t *testing.T) {
	root, wiki := knowledgeLedger(t)
	var ps []string
	for i := 0; i < 12; i++ {
		ps = append(ps, fmt.Sprintf(`{"path": "topics/p%02d.md", "reads": %d}`, i, 1+i%3))
	}
	put(t, filepath.Join(root, "kanban", "issues", "S-0001", "library", "outcome.json"),
		`{"issue": "S-0001", "breakdown": `+knowledgeBreakdown("worker", `{"index_reads": 0, "pages": [`+strings.Join(ps, ",")+`]}`)+`}`)
	res, err := Measure(knowledgeOpts(root, wiki))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range res.Knowledge.Pages {
		got = append(got, fmt.Sprintf("%s=%d", p.Path, p.Reads))
	}
	want := []string{"topics/p02.md=3", "topics/p05.md=3", "topics/p08.md=3", "topics/p11.md=3",
		"topics/p01.md=2", "topics/p04.md=2", "topics/p07.md=2", "topics/p10.md=2", "topics/p00.md=1", "topics/p03.md=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("上から 10 個が違う: %v", got)
	}
}

// 読めない report.json は飛ばした記録に 1 行だけ出す（見込みの読みと申告の読みで 2 回読まない）。
func TestKnowledgeUnreadableReportSkippedOnce(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root は権限で読めないファイルを作れない")
	}
	root, wiki := knowledgeLedger(t)
	p := filepath.Join(root, "kanban", "issues", "S-0001", "library", "report.json")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	res, err := Measure(knowledgeOpts(root, wiki))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range res.Skipped {
		if strings.HasSuffix(s.Source, "S-0001/library/report.json") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("読めない report.json が飛ばした記録に %d 行出た: %+v", n, res.Skipped)
	}
}

// 日本語の名前のページも、読まれなかったページに並べて schema に通る（wiki のページの名前は ASCII に限らない）。
func TestKnowledgeUnreadNonASCIIPagePassesSchema(t *testing.T) {
	root, wiki := knowledgeLedger(t)
	put(t, filepath.Join(wiki, "topics", "セットアップ.md"), "# x\n")
	res, err := Measure(knowledgeOpts(root, wiki))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Knowledge.Unread, "topics/セットアップ.md") {
		t.Fatalf("日本語の名前のページが一覧に無い: %v", res.Knowledge.Unread)
	}
	if _, err := contract.Encode(contract.SchemaMeasure, res); err != nil {
		t.Fatalf("schema/measure.schema.json に通らない: %v", err)
	}
}
