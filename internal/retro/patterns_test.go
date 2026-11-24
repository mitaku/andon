package retro

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/ledger/ledgertest"
)

// patternsInput は根拠の照合に使う、集めた入力の代わり。
// S-0011 は前の振り返り（S-0010、2日前に片付いた）より前、S-0012・S-0013 は後に片付いた。
// h-a・h-b は Chief of Staff の同じセッション、h-c は別のセッション、h-w は S-0012 の担当のセッション、h-r は S-0012 のレビュアーのセッション。
func patternsInput() *Input {
	issue := func(id string) *string { return &id }
	pr := "https://github.com/example/sample-site/pull/12"
	return &Input{
		Since: ago(7 * day), Until: ago(0),
		PreviousRetro: &PrevRetroIn{Issue: "S-0010", DoneAt: ago(2 * day)},
		Boundary:      ptr(ago(2 * day)),
		Issues: []Bundle{
			{ID: "S-0011", Title: "S-0011 のタイトル", DoneAt: ago(3 * day)},
			{ID: "S-0012", Title: "S-0012 のタイトル", DoneAt: ago(1 * day), PR: &pr},
			{ID: "S-0013", Title: "S-0013 のタイトル", DoneAt: ago(1 * day)},
		},
		Typed: []Typed{
			{ID: "h-a", Session: cosSess, At: ago(3 * day), Role: "chief-of-staff"},
			{ID: "h-b", Session: cosSess, At: ago(1 * day), Role: "chief-of-staff"},
			{ID: "h-c", Session: otherSess, At: ago(4 * day), Role: "chief-of-staff"},
			{ID: "h-w", Session: workSess, At: ago(1 * day), Role: "worker", Issue: issue("S-0012")},
			{ID: "h-r", Session: workSess, At: ago(1 * day), Role: "reviewer", Issue: issue("S-0012-r1")},
		},
		Fixes: []Fix{},
	}
}

func ptr(s string) *string { return &s }

func pattern(evidence ...string) Pattern {
	return Pattern{ID: "p1", Summary: "PR のタイトルに課題番号が入る", Evidence: evidence, Level: "test", Action: "add",
		Target: Target{Kind: "project", Repo: "github.com/example/sample-site", Paths: []string{"tests/pr-title.sh"}},
		Effect: "タイトルに課題番号が入った PR が CI で落ちる", Risk: "タイトルの検査が誤検知する", Decision: "fix", Reason: "テストで止められる"}
}

var repos = []string{"github.com/example/sample-site"}

func checkOne(t *testing.T, p Pattern, in *Input) error {
	t.Helper()
	return CheckPatterns([]Pattern{p}, []FixView{}, in, repos, true)
}

func wantRefused(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("断らなかった（want %q）", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("理由 %q に %q が無い", err, want)
	}
}

// 受け入れ条件 11: 1回しか起きていないパターンは通らない。
func TestEvidenceTwoDistinct(t *testing.T) {
	in := patternsInput()
	for name, ev := range map[string][]string{
		"根拠が1件":              {"S-0012"},
		"同じ課題を2回":            {"S-0012", "S-0012"},
		"同じセッションの文を2つ":       {"h-a", "h-b"},
		"課題とその担当の文":          {"S-0012", "h-w"},
		"課題とそのレビュアーの文":       {"S-0012", "h-r"},
		"担当の文とレビュアーの文（同じ課題）": {"h-w", "h-r"},
	} {
		t.Run(name, func(t *testing.T) { wantRefused(t, checkOne(t, pattern(ev...), in), "別々の2件に満たない") })
	}
	for name, ev := range map[string][]string{
		"別々の課題2件":       {"S-0012", "S-0013"},
		"課題1件とオーナーの文1件": {"S-0013", "h-b"},
		"別々のセッションの文2つ":  {"h-b", "h-c"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkOne(t, pattern(ev...), in); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEvidenceMustBeCollected(t *testing.T) {
	wantRefused(t, checkOne(t, pattern("S-0012", "S-0099"), patternsInput()), "集めた入力")
	wantRefused(t, checkOne(t, pattern("S-0012", "h-zz"), patternsInput()), "集めた入力")
}

// 受け入れ条件 12: 新しい出来事が無ければ出せない。
func TestEvidenceNewerThanPreviousRetro(t *testing.T) {
	in := patternsInput()
	wantRefused(t, checkOne(t, pattern("S-0011", "h-c"), in), "前の振り返り S-0010")
	if err := checkOne(t, pattern("S-0011", "S-0012"), in); err != nil {
		t.Fatalf("1件だけ後のもの: %v", err)
	}
	in.PreviousRetro, in.Boundary = nil, nil
	if err := checkOne(t, pattern("S-0011", "h-c"), in); err != nil {
		t.Fatalf("前の振り返りが無ければ新しさは問わない: %v", err)
	}
}

// 受け入れ条件 13: skill か文面を直す先にするなら、上の段で止められない理由が要る。
func TestNotUpstreamRequired(t *testing.T) {
	for _, level := range []string{"skill", "text"} {
		p := pattern("S-0012", "S-0013")
		p.Level = level
		wantRefused(t, checkOne(t, p, patternsInput()), "上の段で止められない理由")
		p.NotUpstream = "依頼書の読み方の誤りで、機械が確かめる値が無い"
		if err := checkOne(t, p, patternsInput()); err != nil {
			t.Fatalf("%s に理由があれば通る: %v", level, err)
		}
	}
}

// ADR-0039 受け入れ条件 13: 振り返り自身（skill retro と andon retro）も直す先にできる（決めたこと 12）。
// skill の段に上の段で止められない理由が要る決まりは、振り返り自身にも同じに効く。
func TestSelfIsTarget(t *testing.T) {
	skill := pattern("S-0012", "S-0013")
	skill.Level = "skill"
	skill.Target = Target{Kind: "andon", Repo: "github.com/example/andon", Paths: []string{"package/skills/retro/SKILL.md"}}
	wantRefused(t, checkOne(t, skill, patternsInput()), "上の段で止められない理由")
	skill.NotUpstream = "パターンの拾い方の手順の抜けで、機械が確かめる値が無い"
	if err := checkOne(t, skill, patternsInput()); err != nil {
		t.Fatalf("理由のある skill retro のパターンは通る: %v", err)
	}
	for _, path := range []string{"internal/retro/patterns.go", "internal/retro", "package/skills/retro"} {
		p := pattern("S-0012", "S-0013")
		p.Level = "design"
		p.Target = Target{Kind: "andon", Repo: "github.com/example/andon", Paths: []string{path}}
		if err := checkOne(t, p, patternsInput()); err != nil {
			t.Fatalf("andon retro（%s）のパターンは通る: %v", path, err)
		}
	}
}

// ADR-0039 受け入れ条件 1・2（形の側）: 判定の無いパターンは断る。fix の数は問わない（決めたこと 1）。
func TestDecisionWithoutFixLimit(t *testing.T) {
	p := pattern("S-0012", "S-0013")
	p.Decision = ""
	wantRefused(t, checkOne(t, p, patternsInput()), "判定が無い")
	var ps []Pattern
	for i := 1; i <= 6; i++ {
		x := pattern("S-0012", "S-0013")
		x.ID = fmt.Sprintf("p%d", i)
		ps = append(ps, x)
	}
	ps[5].Decision = "drop"
	if err := CheckPatterns(ps, []FixView{}, patternsInput(), repos, true); err != nil {
		t.Fatalf("fix 5 件・drop 1 件は通る: %v", err)
	}
	ps[5].Decision = "fix"
	if err := CheckPatterns(ps, []FixView{}, patternsInput(), repos, true); err != nil {
		t.Fatalf("fix 6 件も通る: %v", err)
	}
}

func TestShape(t *testing.T) {
	for name, mod := range map[string]func(*Pattern){
		"id の形":              func(p *Pattern) { p.ID = "x1" },
		"summary が空":         func(p *Pattern) { p.Summary = " " },
		"summary が - で始まる":   func(p *Pattern) { p.Summary = "-x" },
		"effect に改行":         func(p *Pattern) { p.Effect = "a\nb" },
		"知らない段":              func(p *Pattern) { p.Level = "prompt" },
		"知らない action":        func(p *Pattern) { p.Action = "rewrite" },
		"知らない kind":          func(p *Pattern) { p.Target.Kind = "cell" },
		"project に repo が無い": func(p *Pattern) { p.Target.Repo = "" },
		"repos に無い repo":     func(p *Pattern) { p.Target.Repo = "github.com/example/other" },
		"paths が空":           func(p *Pattern) { p.Target.Paths = nil },
		"paths が外を指す":        func(p *Pattern) { p.Target.Paths = []string{"../x"} },
		"paths が絶対":          func(p *Pattern) { p.Target.Paths = []string{"/opt/andon/x"} },
		"根拠が無い":              func(p *Pattern) { p.Evidence = nil },
	} {
		t.Run(name, func(t *testing.T) {
			p := pattern("S-0012", "S-0013")
			mod(&p)
			if err := checkOne(t, p, patternsInput()); err == nil {
				t.Fatal("断らなかった")
			}
		})
	}
}

// 前の直しの見立ては、集めた入力の前の直しと過不足なく合う（決めたこと 6）。当てるときは照らさない。
func TestFixViews(t *testing.T) {
	in := patternsInput()
	in.Fixes = []Fix{{Issue: "S-0007"}}
	zero := 0
	ps := []Pattern{pattern("S-0012", "S-0013")}
	wantRefused(t, CheckPatterns(ps, []FixView{}, in, repos, true), "S-0007 の再発の数と見立て")
	wantRefused(t, CheckPatterns(ps, []FixView{{Issue: "S-0007", Recurred: &zero, View: "再発なし"}, {Issue: "S-0008", Recurred: &zero, View: "x"}}, in, repos, true), "S-0008 が集めた入力")
	wantRefused(t, CheckPatterns(ps, []FixView{{Issue: "S-0007", View: "再発なし"}}, in, repos, true), "recurred")
	if err := CheckPatterns(ps, []FixView{{Issue: "S-0007", Recurred: &zero, View: "再発なし"}}, in, repos, true); err != nil {
		t.Fatal(err)
	}
	if err := CheckPatterns(ps, []FixView{}, in, repos, false); err != nil {
		t.Fatalf("当てるときは前の直しを照らさない: %v", err)
	}
	// 再発の数は足し算: 前の振り返りが数えた数より少なければ断る（ADR-0039 決めたこと 6）
	two, three := 2, 3
	in.Fixes[0].RecurredBefore = &two
	wantRefused(t, CheckPatterns(ps, []FixView{{Issue: "S-0007", Recurred: &zero, View: "再発なし"}}, in, repos, true), "前の振り返りが数えた 2 より少ない")
	if err := CheckPatterns(ps, []FixView{{Issue: "S-0007", Recurred: &three, View: "また起きた"}}, in, repos, true); err != nil {
		t.Fatalf("前の数に足した数: %v", err)
	}
}

func TestParsePatternsInputStrict(t *testing.T) {
	for name, s := range map[string]string{
		"知らないキー":       `{"patterns": [], "fixes": [], "retro": "S-0001"}`,
		"patterns が無い": `{"fixes": []}`,
		"fixes が null": `{"patterns": [], "fixes": null}`,
		"後ろに余り":        `{"patterns": [], "fixes": []} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePatternsInput([]byte(s)); err == nil {
				t.Fatal("断らなかった")
			}
		})
	}
	if _, err := ParsePatternsInput([]byte(`{"patterns": [], "fixes": []}`)); err != nil {
		t.Fatal(err)
	}
}

// 受け入れ条件 16・17: 依頼書に要約・根拠・直す先・期待する効果・リスクが入り、オーナーの文の本文は入らない。
// Andon のパッケージの直しで、その repo をこのプロジェクトが持たなければオーナーの課題。
func TestPlan(t *testing.T) {
	in := patternsInput()
	in.Typed[1].Text = "オーナーが打った本文"
	p := pattern("S-0012", "h-b")
	pl := planOf("S-0020", p, in, repos, "")
	if pl.Who != "worker" || pl.Repo != "github.com/example/sample-site" || pl.Title != p.Summary {
		t.Fatalf("直す課題: %+v", pl)
	}
	for _, want := range []string{"解決すべき問題: " + p.Summary, "直す先: テスト・lint の段で足す", "tests/pr-title.sh",
		"期待する効果: " + p.Effect, "リスク: " + p.Risk, "- S-0012: S-0012 のタイトル", "- h-b: オーナーが打った文",
		"PR の本文に書いてよい根拠の PR: https://github.com/example/sample-site/pull/12", "S-0020 の library/retro.json のパターン p1", "ラベル retro"} {
		if !strings.Contains(pl.Brief, want) {
			t.Errorf("依頼書に %q が無い:\n%s", want, pl.Brief)
		}
	}
	if strings.Contains(pl.Brief, "オーナーが打った本文") {
		t.Errorf("依頼書にオーナーの文の本文が入った")
	}
	// 守ることの行が paths の範囲だけを言うと、paths の文を固定する検査と同じ文の写しを直してよいかで担当が迷う。
	// 行は、paths の文を字面で確かめる検査と paths の文の写しを同じ PR で直してよいと書き、それより広くは許さない。
	// 振る舞いを止めて落ちる検査と、書き換えない ADR・決定の記録は許す所に入らない
	keep := ""
	for _, line := range strings.Split(pl.Brief, "\n") {
		if strings.HasPrefix(line, "守ること: ") {
			keep = line
		}
	}
	for _, want := range []string{"paths の範囲で直す", "paths の文を字面で確かめる検査", "paths の文をそのまま写した所", "同じ PR で直してよい",
		"docs/adr/", "docs/decisions/", "それより外は直さず"} {
		if !strings.Contains(keep, want) {
			t.Errorf("守ることの行に %q が無い: %q", want, keep)
		}
	}

	p.Target = Target{Kind: "andon", Repo: "github.com/example/andon", Paths: []string{"package/skills/worker/SKILL.md"}}
	p.Level, p.NotUpstream = "skill", "手順の抜けで、機械が確かめる値が無い"
	for name, rs := range map[string][]string{"repos に無い": repos, "repos が空": nil} {
		pl := planOf("S-0020", p, in, rs, "")
		if pl.Who != "person" || pl.Repo != "" || !strings.HasPrefix(pl.Title, "Andon へ伝えるか: ") {
			t.Errorf("%s: オーナーの課題でない: %+v", name, pl)
		}
	}
	pl = planOf("S-0020", p, in, append(repos, "github.com/example/andon"), "")
	if pl.Who != "worker" || pl.Repo != "github.com/example/andon" || !strings.Contains(pl.Brief, "上の段で止められない理由: 手順の抜け") {
		t.Errorf("Andon の repo を持つプロジェクトでは直す課題: %+v", pl)
	}
	p.Target.Repo = ""
	if pl := planOf("S-0020", p, in, append(repos, "github.com/example/andon"), ""); pl.Who != "person" {
		t.Errorf("repo の無い Andon の直しはオーナーの課題: %+v", pl)
	}
}

// 前の振り返りは、期間の終わりまでに片付いたものだけ。パターンの一覧を書く・当てる振り返りの課題自身は数えない。
func TestPreviousRetroExcludesSelfAndLater(t *testing.T) {
	f := newFixture(t)
	k := filepath.Join(f.root, "kanban")
	// A-0011: 今回の振り返り（retro.json がある）。昨日、一度片付いた
	write(t, filepath.Join(k, "issues", "A-0011", "issue.yaml"), issueYAML("A-0011", "done", "profile: claude\n"))
	write(t, filepath.Join(k, "issues", "A-0011", "library", "retro.json"), "{}")
	log, _ := os.ReadFile(filepath.Join(k, "log.jsonl"))
	write(t, filepath.Join(k, "log.jsonl"), string(log)+ledgertest.Log(t, `{"at":"`+ago(1*day)+`","by":"terminal:t1","id":"A-0011","change":{"status":"done"}}`))
	col := func(o Options) string {
		in, err := Collect(o)
		if err != nil {
			t.Fatal(err)
		}
		if in.PreviousRetro == nil {
			return ""
		}
		return in.PreviousRetro.Issue
	}
	if got := col(Options{Root: f.root, Now: now, Days: 7}); got != "A-0011" {
		t.Errorf("自分を外さなければ A-0011: %q", got)
	}
	if got := col(Options{Root: f.root, Now: now, Days: 7, Exclude: "A-0011"}); got != "A-0006" {
		t.Errorf("自分を外す: %q", got)
	}
	if got := col(Options{Root: f.root, Now: now.Add(-2 * day), Days: 7}); got != "A-0006" {
		t.Errorf("期間の終わりの後に片付いた振り返りは数えない: %q", got)
	}
}

func TestMainPatternsApplyUsage(t *testing.T) {
	for _, args := range [][]string{{"patterns"}, {"patterns", "A-0001"}, {"apply"}, {"apply", "A-0001", "--days", "3"},
		{"patterns", "A-0001", "--dry-run", "-"}, {"patterns", "../x", "-"}} {
		var out, errs strings.Builder
		if rc := Main(args, nil, &out, &errs); rc != 2 {
			t.Errorf("%v: rc=%d %q", args, rc, errs.String())
		}
	}
}

// オーナーの課題になる fix のパターン（直す先が Andon のパッケージで、その repo を持たない）が同じ summary を持つと、2 つ目は
// 同じ問いのオーナーの課題になり、apply が 1 つ目を足したあとで断られて当て終わらない。先に断る（ADR-0057 決めたこと 11）。
// 担当の課題になるパターンと、drop のパターンは、summary が重なっても通る。
func TestOwnerPatternsWithSameSummaryRefused(t *testing.T) {
	ps := []Pattern{pattern("S-0012", "S-0013"), pattern("S-0012", "S-0013")}
	ps[1].ID = "p2"
	if err := CheckPatterns(ps, []FixView{}, patternsInput(), repos, true); err != nil {
		t.Fatalf("担当の課題になるパターンは summary が重なっても通る: %v", err)
	}
	for i := range ps {
		ps[i].Target = Target{Kind: "andon", Paths: []string{"package/skills/worker/SKILL.md"}}
	}
	wantRefused(t, CheckPatterns(ps, []FixView{}, patternsInput(), repos, true), "パターン p1 と p2 の summary が同じ")
	ps[1].Decision = "drop"
	if err := CheckPatterns(ps, []FixView{}, patternsInput(), repos, true); err != nil {
		t.Fatalf("drop のパターンは課題を足さないので通る: %v", err)
	}
}
