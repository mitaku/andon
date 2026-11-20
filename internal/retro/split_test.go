package retro

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger/ledgertest"
)

// 全文と要約の分け方（ADR-0039 決めたこと 6・受け入れ条件 4〜9）。

// split は束を多く並べる kanban。束は同じ形で作るので、全文の大きさは束の数に比例する。
type split struct {
	t    *testing.T
	root string
	log  []string
	gh   func(args ...string) ([]byte, error)
}

func newSplit(t *testing.T) *split {
	t.Helper()
	f := &split{t: t, root: filepath.Join(t.TempDir(), "sample-site")}
	write(t, filepath.Join(f.k(), "config.yaml"), "version: 3\nprefix: A\n")
	return f
}

func (f *split) k() string           { return filepath.Join(f.root, "kanban") }
func (f *split) is(id string) string { return filepath.Join(f.k(), "issues", id) }
func (f *split) done(id string, at string) {
	f.log = append(f.log, fmt.Sprintf(`{"at":%q,"by":"terminal:t1","id":%q,"change":{"status":"done"}}`, at, id))
}

// issue は担当の課題1件を、全文が重くなる記録（日報・起動の記録・結果の記録・レビュアーの子の major・gate の fix・手直し）つきで置く。
func (f *split) issue(id string, doneAt string) {
	t := f.t
	write(t, filepath.Join(f.is(id), "issue.yaml"), issueYAML(id, "done", "profile: claude\n"))
	lib := filepath.Join(f.is(id), "library")
	write(t, filepath.Join(lib, "journal.md"), "- 理解: "+strings.Repeat("迷った。", 300)+"\n")
	write(t, filepath.Join(lib, "launches.jsonl"), fmt.Sprintf(`{"at":%q,"role":"worker","launch":"start","profile":"claude","command":"claude --model opus --effort high","package":{"commit":%q,"root":"/opt/andon"},"memory_index":%q}`+"\n",
		doneAt, sha1, strings.Repeat("- 目録の行\n", 50)))
	write(t, filepath.Join(lib, "outcome.json"), `{"result":"merged","worked_minutes":30,"gates":[],"sessions":[`+
		`{"harness":"claude","session":"s1","cost":{"models":["m"],"efforts":["high"],"turns":3,"tokens":{"input":10,"output":20,"cache_read":30,"cache_write":40}}},`+
		`{"harness":"claude","session":"s2","cost":null}]}`)
	write(t, filepath.Join(lib, "report.json"), `{"remember":["一つ目の学び","二つ目の学び"]}`)
	write(t, filepath.Join(lib, "gate", sha1+".json"), `{"head":"`+sha1+`","at":"`+doneAt+`","tier":"B","decision":"fix","reasons":["Must fix 以上が 1 件"],"merged":false}`)
	write(t, filepath.Join(lib, "corrections.json"), `{"at":"x","corrections":[{"kind":"override","question":1,"recommended":"x","answer":"人がマージした（PR #1、abc）"}]}`)
	r := id + "-r1"
	write(t, filepath.Join(f.is(r), "issue.yaml"), issueYAML(r, "done", "profile: codex-review\nparent: "+id+"\n"))
	write(t, filepath.Join(f.is(r), "library", "findings.json"), `{"commit":"`+sha1+`","findings":[{"severity":"major","file":"a.go","line":3},{"severity":"minor","file":"b.go","line":1}]}`)
	write(t, filepath.Join(f.is(r), "report.md"), "Commit: "+sha1+"\n## Report\n- major — a.go:3\n  空の入力で落ちる。\n- minor — b.go:1\n")
	f.done(id, doneAt)
}

// retro は振り返りの課題を置く。untilAgo が 0 でなければ retro.json（集めた時刻 until と前の直しの再発の数）を書く。
func (f *split) retro(id string, doneAt time.Time, until time.Time, fixes string) {
	t := f.t
	write(t, filepath.Join(f.is(id), "issue.yaml"), issueYAML(id, "done", "profile: claude\nroutine: retro\n"))
	if !until.IsZero() {
		u := until.UTC().Format(time.RFC3339)
		s := until.Add(-7 * day).UTC().Format(time.RFC3339)
		write(t, filepath.Join(f.is(id), "library", retroFile),
			`{"retro":"`+id+`","at":"`+u+`","since":"`+s+`","until":"`+u+`","days":7,"previous_retro":null,"patterns":[],"fixes":[`+fixes+`]}`)
	}
	f.done(id, doneAt.UTC().Format(time.RFC3339))
}

func (f *split) collect(o Options) *Input {
	f.t.Helper()
	write(f.t, filepath.Join(f.k(), "log.jsonl"), ledgertest.Log(f.t, f.log...))
	o.Root, o.Now, o.Days, o.GH = f.root, now, 7, f.gh
	in, err := Collect(o)
	if err != nil {
		f.t.Fatal(err)
	}
	return in
}

func size(t *testing.T, in *Input) int {
	t.Helper()
	b, _, err := Redacted(in)
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}

func idsOf(in *Input) (full, summ []string) {
	for _, b := range in.Issues {
		full = append(full, b.ID)
	}
	for _, s := range in.Summaries {
		summ = append(summ, s.ID)
	}
	return full, summ
}

// 前の振り返り A-0900 が 2 日前に片付き、その 1 時間前に集めた。古い束 300 件と新しい束 25 件。
// A-0800 は前の振り返りが集めてから片付くまでの間に片付いた。
func boundaryFixture(t *testing.T) (*split, time.Time) {
	f := newSplit(t)
	until := now.Add(-2*day - time.Hour)
	f.retro("A-0900", now.Add(-2*day), until, "")
	for i := 1; i <= 300; i++ {
		f.issue(fmt.Sprintf("A-%04d", i), now.Add(-3*day-time.Duration(i)*time.Minute).Format(time.RFC3339))
	}
	for i := 1; i <= 24; i++ {
		f.issue(fmt.Sprintf("A-%04d", 400+i), now.Add(-day-time.Duration(i)*time.Minute).Format(time.RFC3339))
	}
	f.issue("A-0800", now.Add(-2*day-30*time.Minute).Format(time.RFC3339))
	return f, until
}

// 受け入れ条件 4・5: 境目は前の振り返りが集めた時刻。その後に片付いた束が全文、前の束が要約。
// 集めてから片付くまでの間に片付いた束も全文。出力は全部を全文で出したときの 4 分の 1 以下。
func TestSplitAtPreviousUntil(t *testing.T) {
	f, until := boundaryFixture(t)
	in := f.collect(Options{})
	full, summ := idsOf(in)
	if len(full) != 25 || len(summ) != 300 || in.Cut != nil {
		t.Fatalf("全文 %d 件・要約 %d 件・切り %+v（want 25・300・nil）", len(full), len(summ), in.Cut)
	}
	if full[0] != "A-0800" {
		t.Errorf("集めてから片付くまでの間に片付いた束が全文の先頭でない: %v", full[:3])
	}
	if in.Boundary == nil || *in.Boundary != until.Format(time.RFC3339) || in.PreviousRetro.Until == nil || *in.PreviousRetro.Until != *in.Boundary {
		t.Errorf("境目 = %v・前の振り返り = %+v（want %s）", in.Boundary, in.PreviousRetro, until.Format(time.RFC3339))
	}
	// 同じ kanban を全部全文で出す: 前の振り返りを数えず、全文の上限を外す
	defer func(n int) { fullLimit = n }(fullLimit)
	fullLimit = 1000
	all := f.collect(Options{Exclude: "A-0900"})
	if len(all.Issues) != 325 || len(all.Summaries) != 0 {
		t.Fatalf("全部全文: %d・%d", len(all.Issues), len(all.Summaries))
	}
	got, whole := size(t, in), size(t, all)
	t.Logf("出力 %d バイト・全部を全文で %d バイト", got, whole)
	if got*4 > whole {
		t.Errorf("出力 %d バイトが、全部を全文で出した %d バイトの 4 分の 1 を超える", got, whole)
	}
	// 5: その束と古い束を根拠にしたパターンが通る（新しい出来事に数える）
	if err := checkOne(t, pattern("A-0800", "A-0001"), in); err != nil {
		t.Errorf("境目の後の束を新しい出来事に数えない: %v", err)
	}
	wantRefused(t, checkOne(t, pattern("A-0002", "A-0001"), in), "境目")
}

// 前の振り返りに retro.json が無ければ、片付いた時刻を境目にする。
func TestSplitAtDoneWithoutRetroJSON(t *testing.T) {
	f := newSplit(t)
	f.retro("A-0900", now.Add(-2*day), time.Time{}, "")
	f.issue("A-0001", now.Add(-3*day).Format(time.RFC3339))
	f.issue("A-0002", now.Add(-2*day-30*time.Minute).Format(time.RFC3339))
	f.issue("A-0003", now.Add(-day).Format(time.RFC3339))
	in := f.collect(Options{})
	full, summ := idsOf(in)
	if strings.Join(full, " ") != "A-0003" || strings.Join(summ, " ") != "A-0001 A-0002" {
		t.Errorf("全文 %v・要約 %v", full, summ)
	}
	if in.Boundary == nil || *in.Boundary != ago(2*day) || in.PreviousRetro.Until != nil {
		t.Errorf("境目 = %v・前の振り返り = %+v", in.Boundary, in.PreviousRetro)
	}
}

// 受け入れ条件 6・7: 要約の束も根拠に指せ、全文を id で引ける。要約には、つまずきの跡の見出しだけが残る。
func TestSummaryEvidenceAndIssue(t *testing.T) {
	f, _ := boundaryFixture(t)
	in := f.collect(Options{})
	if err := checkOne(t, pattern("A-0001", "A-0401"), in); err != nil {
		t.Errorf("要約の束と全文の束を根拠にしたパターン: %v", err)
	}
	var s *Summary
	for i := range in.Summaries {
		if in.Summaries[i].ID == "A-0001" {
			s = &in.Summaries[i]
		}
	}
	if s == nil {
		t.Fatal("A-0001 が要約に無い")
	}
	if strings.Join(s.Findings, "|") != "major — a.go:3 空の入力で落ちる。" || strings.Join(s.GateFixes, "|") != "Must fix 以上が 1 件" ||
		s.Corrections == nil || *s.Corrections || s.Remember != 2 || s.DoneAt == "" || s.Title != "A-0001 の題" {
		t.Errorf("要約 = %+v", *s)
	}
	b, _, err := Redacted(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "- 理解: 迷った。") != 25 {
		t.Error("日報の本文が全文の束の外（要約）にも出ている")
	}
	// --issue: 要約の束の全文を引く
	one := f.collect(Options{Issue: "A-0001"})
	if len(one.Issues) != 1 || len(one.Summaries) != 0 || one.Issues[0].Journal == nil || len(one.Issues[0].Reviews) != 1 ||
		one.Issues[0].Reviews[0].Findings == nil {
		t.Fatalf("--issue A-0001: 全文 %d・要約 %d・%+v", len(one.Issues), len(one.Summaries), one.Issues)
	}
	if none := f.collect(Options{Issue: "A-0900"}); len(none.Issues) != 0 {
		t.Errorf("振り返りの課題は引けない: %v", none.Issues)
	}
}

// 要約の手直しの印: 「人がマージした」と「オーナーがマージした」の答えだけなら false、ほかがあれば true、記録が無ければ null。
func TestSummaryCorrections(t *testing.T) {
	merged := map[string]any{"kind": "override", "question": float64(1), "answer": "オーナーがマージした（PR #2）"}
	other := map[string]any{"kind": "answer", "question": float64(2), "picked": "x"}
	r := &reader{}
	if s := r.summary(Bundle{Corrections: []any{merged}}); s.Corrections == nil || *s.Corrections {
		t.Errorf("マージだけ: %v", s.Corrections)
	}
	if s := r.summary(Bundle{Corrections: []any{merged, other}}); s.Corrections == nil || !*s.Corrections {
		t.Errorf("ほかの手直し: %v", s.Corrections)
	}
	if s := r.summary(Bundle{}); s.Corrections != nil {
		t.Errorf("記録が無い: %v", *s.Corrections)
	}
	if got := clip(strings.Repeat("あ", 200)); len([]rune(got)) != lineMax+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("切る長さ: %d", len([]rune(got)))
	}
}

// 受け入れ条件 8: 新しい束が全文の上限を超えたら、新しい順に上限の件数だけが全文になり、切ったことを書く。
func TestSplitCutNewest(t *testing.T) {
	f := newSplit(t)
	for i := 1; i <= 130; i++ {
		f.issue(fmt.Sprintf("A-%04d", i), now.Add(-3*day+time.Duration(i)*time.Minute).Format(time.RFC3339))
	}
	in := f.collect(Options{})
	full, summ := idsOf(in)
	if len(full) != 100 || len(summ) != 30 || in.Cut == nil || in.Cut.New != 130 || in.Cut.Full != 100 || in.Boundary != nil {
		t.Fatalf("全文 %d・要約 %d・切り %+v・境目 %v", len(full), len(summ), in.Cut, in.Boundary)
	}
	if full[0] != "A-0031" || summ[len(summ)-1] != "A-0030" {
		t.Errorf("新しい順に 100 件でない: 全文の先頭 %s・要約の最後 %s", full[0], summ[len(summ)-1])
	}
}

// 受け入れ条件 9: 前の直しに、前の振り返りが書いた再発の数を添える。
func TestFixesRecurredBefore(t *testing.T) {
	f := newSplit(t)
	pr := func(n int) string { return fmt.Sprintf("https://github.com/example/sample-site/pull/%d", n) }
	// A-0900: 古い振り返り。A-0901: 前の振り返り（集めたのは 2 日前）。
	f.retro("A-0900", now.Add(-10*day), now.Add(-10*day-time.Hour), "")
	f.retro("A-0901", now.Add(-2*day), now.Add(-2*day-time.Hour), `{"issue":"A-0011","recurred":2,"view":"再発した"}`)
	for _, x := range []struct {
		id, parent string
		n          int
	}{{"A-0011", "A-0900", 11}, {"A-0012", "A-0900", 12}, {"A-0013", "A-0901", 13}} {
		write(t, filepath.Join(f.is(x.id), "issue.yaml"), issueYAML(x.id, "done", "profile: claude\nparent: "+x.parent+"\n"))
		write(t, filepath.Join(f.is(x.id), "library", "report.json"), `{"pr":"`+pr(x.n)+`"}`)
		f.done(x.id, ago(6*day))
	}
	f.gh = func(args ...string) ([]byte, error) {
		if args[0] == "pr" && args[1] == "list" {
			return []byte(`[{"number":11,"url":"` + pr(11) + `","title":"a","body":"","mergedAt":"` + ago(5*day) + `","mergeCommit":null},
 {"number":12,"url":"` + pr(12) + `","title":"b","body":"","mergedAt":"` + ago(4*day) + `","mergeCommit":null},
 {"number":13,"url":"` + pr(13) + `","title":"c","body":"","mergedAt":"` + ago(1*day) + `","mergeCommit":null}]`), nil
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}
	in := f.collect(Options{})
	got := map[string]string{}
	for _, x := range in.Fixes {
		got[x.Issue] = "null"
		if x.RecurredBefore != nil {
			got[x.Issue] = fmt.Sprint(*x.RecurredBefore)
		}
	}
	// A-0011: 前の振り返りが 2 と書いた。A-0012: 境目より前のマージで、前の振り返りが数えていない（null）。A-0013: 境目より後のマージ（0）
	if want := map[string]string{"A-0011": "2", "A-0012": "null", "A-0013": "0"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("前の再発の数 = %v（want %v）", got, want)
	}
	if size(t, in) == 0 {
		t.Fatal("schema に合わない")
	}
}
