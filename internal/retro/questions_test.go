package retro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func questionsFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	is := func(id string) string { return filepath.Join(f.root, "kanban", "issues", id) }
	write(t, filepath.Join(is("A-0004"), "library", "report.json"), `{"gaps":[
 {"kind":"wrong","where":"AGENTS.md","text":"重い検査の一覧が実物と 1 つ違う","proposal":"一覧を合わせる"},
 {"kind":"missing","where":"docs/operate.md","text":"トークンを置き直す順が無い"}]}`)
	q := func(n int, asked, answer string) string {
		s := "- n: " + string(rune('0'+n)) + "\n  text: 問い\n  asked: \"" + asked + "\"\n  askedBy: terminal:t1\n"
		if answer != "" {
			s += "  status: received\n  answer: " + answer + "\n"
		} else {
			s += "  status: open\n"
		}
		return s
	}
	write(t, filepath.Join(is("A-0001"), "questions.yaml"), q(1, ago(2*day), "マージした"))
	write(t, filepath.Join(is("A-0001"), "library", "questions", "1.json"),
		`{"kind":"line","question":"マージするか","options":[{"label":"マージする","pick":true},{"label":"しない"}],"from":"gate"}`)
	write(t, filepath.Join(is("A-0003"), "questions.yaml"), q(1, ago(2*day), "c1"))
	write(t, filepath.Join(is("A-0003"), "library", "questions", "1.json"),
		`{"kind":"scope","question":"どれを直すか","options":[{"label":"c1","pick":true},{"label":"c2"}],"from":"issue"}`)
	write(t, filepath.Join(is("A-0005"), "questions.yaml"), q(1, ago(1*day), "CSV と Excel")+q(2, ago(9*day), "")+q(3, ago(1*day), ""))
	write(t, filepath.Join(is("A-0005"), "library", "questions", "1.json"),
		`{"kind":"users","question":"書き出しの形をどうするか","options":[{"label":"CSV だけ","pick":true},{"label":"CSV と Excel"}],"from":"chief-of-staff"}`)
	write(t, filepath.Join(is("A-0005"), "library", "questions", "2.json"),
		`{"kind":"users","question":"期間の外","options":[{"label":"a","pick":true},{"label":"b"}],"from":"chief-of-staff"}`)
	return f
}

// ADR-0061 受け入れ条件 5: 全文の束に gaps があり、questions[] に Chief of Staff の問いだけがあり、出力が schema に通る。
func TestQuestionsAndGaps(t *testing.T) {
	in, out := questionsFixture(t).collect(t)
	b := bundle(t, in, "A-0004")
	if len(b.Gaps) != 2 {
		t.Fatalf("A-0004 の束の gaps = %v（申告 2 件）", b.Gaps)
	}
	if g, _ := b.Gaps[0].(map[string]any); g["kind"] != "wrong" || g["where"] != "AGENTS.md" || g["proposal"] != "一覧を合わせる" {
		t.Errorf("申告は写す: %v", b.Gaps[0])
	}
	if b1 := bundle(t, in, "A-0001"); b1.Gaps == nil || len(b1.Gaps) != 0 {
		t.Errorf("申告の無い報告の gaps は空の配列: %v", b1.Gaps)
	}
	if len(in.Questions) != 1 {
		t.Fatalf("questions = %+v（Chief of Staff の問いだけ）", in.Questions)
	}
	x := in.Questions[0]
	if x.Issue != "A-0005" || x.N != 1 || x.From == nil || *x.From != "chief-of-staff" || x.Kind != "users" ||
		x.Question != "書き出しの形をどうするか" || len(x.Options) != 2 || !x.Options[0].Pick || x.Options[1].Pick ||
		x.Answer == nil || *x.Answer != "CSV と Excel" || x.AskedAt != ago(1*day) {
		t.Errorf("問い = %+v", x)
	}
	var skipped []string
	for _, s := range in.Skipped {
		skipped = append(skipped, s.Source)
	}
	if got := strings.Join(skipped, " "); !strings.Contains(got, filepath.Join("kanban", "issues", "A-0005", "library", "questions", "3.json")) {
		t.Errorf("問いの JSON が無い問いは skipped に出す: %v", in.Skipped)
	}
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if qs, _ := raw["questions"].([]any); len(qs) != 1 {
		t.Errorf("出力の questions = %v", raw["questions"])
	}
}

// 問いの無いプロジェクトでも questions は空の配列で出る（schema の必須）。
func TestQuestionsEmpty(t *testing.T) {
	in, out := newFixture(t).collect(t)
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if qs, ok := raw["questions"].([]any); !ok || len(qs) != 0 || len(in.Questions) != 0 {
		t.Errorf("questions = %v", raw["questions"])
	}
}

// ADR-0061 決めたこと 9: 要約の束には申告の種類と先だけを載せ、本文は載せない。kind か where の無い申告は載せない。
func TestSummaryGaps(t *testing.T) {
	r := &reader{o: Options{Root: t.TempDir()}}
	s := r.summary(Bundle{Library: "/x/library", Gaps: []any{
		map[string]any{"kind": "hard_to_find", "where": "wiki/topics/tool-quirks.md", "text": "本文"},
		map[string]any{"text": "先の無い申告"},
	}})
	if len(s.Gaps) != 1 || s.Gaps[0] != (GapRef{Kind: "hard_to_find", Where: "wiki/topics/tool-quirks.md"}) {
		t.Errorf("要約の gaps = %+v", s.Gaps)
	}
	if len(r.skipped) != 1 {
		t.Errorf("形の崩れた申告は skipped に1行: %+v", r.skipped)
	}
	if s := r.summary(Bundle{}); s.Gaps == nil || len(s.Gaps) != 0 {
		t.Errorf("申告が無ければ空の配列: %+v", s.Gaps)
	}
}

// ADR-0061 受け入れ条件 6: 根拠に問いを指したパターンは、今までどおり断る（問いは根拠の単位でない）。
func TestPatternEvidenceRefusesQuestion(t *testing.T) {
	in := patternsInput()
	in.Questions = []Question{{Issue: "S-0050", N: 1, From: ptr("chief-of-staff"), Kind: "users", Question: "形をどうするか",
		Options: []QOption{{Label: "a", Pick: true}, {Label: "b"}}, AskedAt: ago(1 * day)}}
	for _, ev := range []string{"S-0050", "S-0050#1"} {
		wantRefused(t, checkOne(t, pattern("S-0012", ev), in), "集めた入力")
	}
}

// 要約の束の申告の kind が schema の値の外（前後のバージョン・手で書いた report.json）なら、要約に載せず skipped に回す。
// 載せると出力が schema/retro.schema.json に落ち、collect 全体が出ない。
func TestSummaryGapsUnknownKind(t *testing.T) {
	r := &reader{o: Options{Root: t.TempDir()}}
	s := r.summary(Bundle{Library: "/x/library", Gaps: []any{map[string]any{"kind": "stale", "where": "AGENTS.md", "text": "x"}}})
	if len(s.Gaps) != 0 || len(r.skipped) != 1 {
		t.Errorf("知らない種類の申告: gaps = %+v、skipped = %+v", s.Gaps, r.skipped)
	}
}

func TestQuestionsUnknownFromOrKind(t *testing.T) {
	f := questionsFixture(t)
	is := filepath.Join(f.root, "kanban", "issues", "A-0009")
	write(t, filepath.Join(is, "questions.yaml"), "- n: 1\n  text: 問い\n  asked: \""+ago(1*day)+"\"\n- n: 2\n  text: 問い\n  asked: \""+ago(1*day)+"\"\n")
	write(t, filepath.Join(is, "library", "questions", "1.json"), `{"kind":"line","question":"古い","options":[{"label":"a","pick":true},{"label":"b"}],"from":"task"}`)
	write(t, filepath.Join(is, "library", "questions", "2.json"), `{"kind":"future","question":"新しい種類","options":[{"label":"a","pick":true},{"label":"b"}],"from":"chief-engineer"}`)
	in, _ := f.collect(t)
	if len(in.Questions) != 1 || in.Questions[0].Issue != "A-0005" {
		t.Errorf("questions = %+v", in.Questions)
	}
	n := 0
	for _, s := range in.Skipped {
		if strings.Contains(s.Source, filepath.Join("A-0009", "library", "questions")) {
			n++
		}
	}
	if n != 2 {
		t.Errorf("知らない from・kind の問いは skipped に1行ずつ: %+v", in.Skipped)
	}
}

// 申告の種類と問いの kind・from の表が、元の schema（report・question）と振り返りの入力の schema の値と揃っている。
// 元の schema に値を足して表と振り返りの schema を直し忘れると落ちる。
func TestKindTablesMatchSchemas(t *testing.T) {
	load := func(name string) any {
		b, err := os.ReadFile(filepath.Join("..", "..", "schema", name+".schema.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	at := func(m any, path ...any) any {
		for _, k := range path {
			switch k := k.(type) {
			case string:
				m = m.(map[string]any)[k]
			case int:
				m = m.([]any)[k]
			}
		}
		return m
	}
	set := func(v any, drop string) []string {
		var out []string
		for _, x := range v.([]any) {
			if s := x.(string); s != drop {
				out = append(out, s)
			}
		}
		sort.Strings(out)
		return out
	}
	keys := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			if k != "" {
				out = append(out, k)
			}
		}
		sort.Strings(out)
		return out
	}
	report, question, retro := load("report"), load("question"), load("retro")
	for _, c := range []struct {
		name      string
		got, want []string
	}{
		{"申告の種類（report）", keys(gapKinds), set(at(report, "properties", "gaps", "items", "properties", "kind", "enum"), "")},
		{"申告の種類（retro の要約）", keys(gapKinds), set(at(retro, "$defs", "summary", "properties", "gaps", "items", "properties", "kind", "enum"), "")},
		{"問いの kind（question から scope を除く）", keys(questionKinds), set(at(question, "properties", "kind", "enum"), "scope")},
		{"問いの kind（retro）", keys(questionKinds), set(at(retro, "$defs", "question", "properties", "kind", "enum"), "")},
		{"問いの from（question から gate を除く）", keys(questionFroms), set(at(question, "properties", "from", "enum"), "gate")},
		{"問いの from（retro）", keys(questionFroms), set(at(retro, "$defs", "question", "properties", "from", "anyOf", 0, "enum"), "")},
	} {
		if strings.Join(c.got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s: 表 %v、schema %v", c.name, c.got, c.want)
		}
	}
}
