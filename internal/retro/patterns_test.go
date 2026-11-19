package retro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// patternsInput は根拠の照合に使う、集めた入力の代わり。
// S-0011 は前の振り返り（S-0010、2日前に片付いた）より前、S-0012・S-0013 は後に片付いた。
// h-a・h-b は Chief of Staff の同じセッション、h-c は別のセッション、h-w は S-0012 の担当のセッション、h-r は S-0012 のレビュアーのセッション。
func patternsInput() *Input {
	issue := func(id string) *string { return &id }
	pr := "https://github.com/example/sample-site/pull/12"
	return &Input{
		Since: ago(7 * day), Until: ago(0),
		PreviousRetro: &PrevRetro{Issue: "S-0010", DoneAt: ago(2 * day)},
		Issues: []Bundle{
			{ID: "S-0011", Title: "S-0011 の題", DoneAt: ago(3 * day)},
			{ID: "S-0012", Title: "S-0012 の題", DoneAt: ago(1 * day), PR: &pr},
			{ID: "S-0013", Title: "S-0013 の題", DoneAt: ago(1 * day)},
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

func pattern(evidence ...string) Pattern {
	return Pattern{ID: "p1", Summary: "PR の題に課題番号が入る", Evidence: evidence, Level: "test", Action: "add",
		Target: Target{Kind: "project", Repo: "github.com/example/sample-site", Paths: []string{"tests/pr-title.sh"}},
		Effect: "題に課題番号が入った PR が CI で落ちる", Risk: "題の検査が誤検知する", Decision: "fix", Reason: "テストで止められる"}
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
		"別々の課題2件":      {"S-0012", "S-0013"},
		"課題1件と人の文1件":   {"S-0013", "h-b"},
		"別々のセッションの文2つ": {"h-b", "h-c"},
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
	in.PreviousRetro = nil
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

// 受け入れ条件 14: 振り返り自身を直す先にできない。
func TestNotSelf(t *testing.T) {
	for _, path := range []string{"package/skills/retro/SKILL.md", "package/skills/retro", "internal/retro/patterns.go", "./internal/retro/../retro/x.go"} {
		p := pattern("S-0012", "S-0013")
		p.Target = Target{Kind: "andon", Repo: "github.com/example/andon", Paths: []string{"package/skills/worker/SKILL.md", path}}
		wantRefused(t, checkOne(t, p, patternsInput()), "振り返り自身")
	}
	p := pattern("S-0012", "S-0013")
	p.Target = Target{Kind: "andon", Paths: []string{"package/skills/retrospect/SKILL.md"}}
	if err := checkOne(t, p, patternsInput()); err != nil {
		t.Fatalf("名前が似ているだけの別の skill は通る: %v", err)
	}
}

// 受け入れ条件 15（形の側）: 判定の無いパターンと、fix が上限を超えた一覧は断る。
func TestDecisionAndFixLimit(t *testing.T) {
	p := pattern("S-0012", "S-0013")
	p.Decision = ""
	wantRefused(t, checkOne(t, p, patternsInput()), "判定が無い")
	var ps []Pattern
	for i := 1; i <= 3; i++ {
		x := pattern("S-0012", "S-0013")
		x.ID = "p" + string(rune('0'+i))
		ps = append(ps, x)
	}
	wantRefused(t, CheckPatterns(ps, []FixView{}, patternsInput(), repos, true), "fix が 3 件")
	ps[2].Decision = "drop"
	if err := CheckPatterns(ps, []FixView{}, patternsInput(), repos, true); err != nil {
		t.Fatalf("fix 2 件・drop 1 件は通る: %v", err)
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

// 受け入れ条件 16・17: 依頼書に要約・根拠・直す先・期待する効果・リスクが入り、人の文の本文は入らない。
// Andon のパッケージの直しで、その repo をこのプロジェクトが持たなければ人の課題。
func TestPlan(t *testing.T) {
	in := patternsInput()
	in.Typed[1].Text = "人が打った本文"
	p := pattern("S-0012", "h-b")
	pl := planOf("S-0020", p, in, repos)
	if pl.Who != "worker" || pl.Repo != "github.com/example/sample-site" || pl.Title != p.Summary {
		t.Fatalf("直す課題: %+v", pl)
	}
	for _, want := range []string{"解決すべき問題: " + p.Summary, "直す先: テスト・lint の段で足す", "tests/pr-title.sh",
		"期待する効果: " + p.Effect, "リスク: " + p.Risk, "- S-0012: S-0012 の題", "- h-b: 人が打った文",
		"PR の本文に書いてよい根拠の PR: https://github.com/example/sample-site/pull/12", "S-0020 の library/retro.json のパターン p1", "ラベル retro"} {
		if !strings.Contains(pl.Brief, want) {
			t.Errorf("依頼書に %q が無い:\n%s", want, pl.Brief)
		}
	}
	if strings.Contains(pl.Brief, "人が打った本文") {
		t.Errorf("依頼書に人の文の本文が入った")
	}

	p.Target = Target{Kind: "andon", Repo: "github.com/example/andon", Paths: []string{"package/skills/worker/SKILL.md"}}
	p.Level, p.NotUpstream = "skill", "手順の抜けで、機械が確かめる値が無い"
	for name, rs := range map[string][]string{"repos に無い": repos, "repos が空": nil} {
		pl := planOf("S-0020", p, in, rs)
		if pl.Who != "person" || pl.Repo != "" || !strings.HasPrefix(pl.Title, "Andon へ伝えるか: ") {
			t.Errorf("%s: 人の課題でない: %+v", name, pl)
		}
	}
	pl = planOf("S-0020", p, in, append(repos, "github.com/example/andon"))
	if pl.Who != "worker" || pl.Repo != "github.com/example/andon" || !strings.Contains(pl.Brief, "上の段で止められない理由: 手順の抜け") {
		t.Errorf("Andon の repo を持つプロジェクトでは直す課題: %+v", pl)
	}
	p.Target.Repo = ""
	if pl := planOf("S-0020", p, in, append(repos, "github.com/example/andon")); pl.Who != "person" {
		t.Errorf("repo の無い Andon の直しは人の課題: %+v", pl)
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
	write(t, filepath.Join(k, "log.jsonl"), string(log)+`{"at":"`+ago(1*day)+`","by":"terminal:t1","id":"A-0011","change":{"status":"done"}}`+"\n")
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
		{"patterns", "A-0001", "--dry-run", "-"}, {"patterns", "../x", "-"}, {"bogus"}} {
		var out, errs strings.Builder
		if rc := Main(args, nil, &out, &errs); rc != 2 {
			t.Errorf("%v: rc=%d %q", args, rc, errs.String())
		}
	}
}
