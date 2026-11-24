package retro

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// questionsFixture は newFixture に、申告を持つ担当の課題（A-0004）と、gate の問い・範囲の問い・Chief of Staff の問いを
// 1 件ずつ足す。ほかに、期間の外の Chief of Staff の問いと、問いの JSON の無い問いを足す。
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
	// gate の問い（A-0001）
	write(t, filepath.Join(is("A-0001"), "questions.yaml"), q(1, ago(2*day), "マージした"))
	write(t, filepath.Join(is("A-0001"), "library", "questions", "1.json"),
		`{"kind":"line","question":"マージするか","options":[{"label":"マージする","pick":true},{"label":"しない"}],"from":"gate"}`)
	// 範囲の問い（A-0003）
	write(t, filepath.Join(is("A-0003"), "questions.yaml"), q(1, ago(2*day), "c1"))
	write(t, filepath.Join(is("A-0003"), "library", "questions", "1.json"),
		`{"kind":"scope","question":"どれを直すか","options":[{"label":"c1","pick":true},{"label":"c2"}],"from":"issue"}`)
	// Chief of Staff の問い（オーナーの課題 A-0005）。2 つ目は期間の外、3 つ目は問いの JSON が無い
	write(t, filepath.Join(is("A-0005"), "questions.yaml"), q(1, ago(1*day), "CSV と Excel")+q(2, ago(9*day), "")+q(3, ago(1*day), ""))
	write(t, filepath.Join(is("A-0005"), "library", "questions", "1.json"),
		`{"kind":"users","question":"書き出しの形をどうするか","options":[{"label":"CSV だけ","pick":true},{"label":"CSV と Excel"}],"from":"chief-of-staff"}`)
	write(t, filepath.Join(is("A-0005"), "library", "questions", "2.json"),
		`{"kind":"users","question":"期間の外","options":[{"label":"a","pick":true},{"label":"b"}],"from":"chief-of-staff"}`)
	return f
}

// ADR-0061 受け入れ条件 5: 全文の束に gaps があり、questions[] に Chief of Staff の問いだけがあり、出力が schema に通る。
func TestQuestionsAndGaps(t *testing.T) {
	in, out := questionsFixture(t).collect(t) // collect は Redacted で schema/retro.schema.json に照らす
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
