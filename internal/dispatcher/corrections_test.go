package dispatcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"andon/internal/contract"
)

func q(kind, from string, labels ...string) *contract.Question {
	t := true
	opts := []contract.Option{{Label: labels[0], Pick: &t}}
	for _, l := range labels[1:] {
		opts = append(opts, contract.Option{Label: l})
	}
	return &contract.Question{Kind: kind, Question: "x", Options: opts, From: from}
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCorrections(t *testing.T) {
	sha := func(c byte) string { return strings.Repeat(string(c), 40) }
	line := 3
	pr := func(state string, oids ...string) *PRView {
		v := &PRView{State: state}
		for _, o := range oids {
			v.Commits = append(v.Commits, struct {
				Oid             string `json:"oid"`
				MessageHeadline string `json:"messageHeadline"`
			}{o, "msg " + o[:1]})
		}
		return v
	}
	cases := []struct {
		name  string
		qs    []QA
		rep   *contract.Report
		must  string
		pr    *PRView
		human []HumanCommit
		want  string
	}{
		{"手直しが無い（推奨どおりの答え・gate のコメントだけ・軽い waived）",
			[]QA{{N: 1, Status: "received", Answer: " CSV だけ \n", Q: q("users", "issue", "CSV だけ", "CSV と Excel")}},
			&contract.Report{Waived: []contract.Waived{{Severity: "minor", File: "a.go", Line: &line, Reason: "r"}}}, "major",
			func() *PRView {
				v := pr("MERGED", sha('a'))
				v.Comments = append(v.Comments, struct {
					Author struct {
						Login string `json:"login"`
					} `json:"author"`
					Body      string `json:"body"`
					CreatedAt string `json:"createdAt"`
				}{Body: "andon gate: **approve**（commit x）"})
				return v
			}(), nil, `[]`},
		{"推奨と違う答え → answer",
			[]QA{{N: 2, Status: "answered", Answer: "CSV と Excel", Q: q("users", "issue", "CSV だけ", "CSV と Excel")}},
			nil, "", nil, nil,
			`[{"kind":"answer","question":2,"decision_kind":"users","recommended":"CSV だけ","picked":"CSV と Excel"}]`},
		{"gate の問いへの答え → override（推奨どおりでも）・scope は数えない・答えの無い問いと JSON の無い問いも数えない",
			[]QA{{N: 1, Status: "received", Answer: "オーナーが PR を読んでマージする", Q: q("line", "gate", "オーナーが PR を読んでマージする", "直させる")},
				{N: 2, Status: "received", Answer: "c2 だけ", Q: q("scope", "issue", "c1 と c2", "c2 だけ")},
				{N: 3, Status: "open", Q: q("users", "issue", "A", "B")},
				{N: 4, Status: "received", Answer: "B"}},
			nil, "", nil, nil,
			`[{"kind":"override","question":1,"recommended":"オーナーが PR を読んでマージする","answer":"オーナーが PR を読んでマージする"}]`},
		{"機械が閉じた gate の問い（settle のマージの答え・head が動いて gate を回し直すときの答え2種）は数えない・オーナーの答えは推奨と違っても同じでも数える",
			[]QA{{N: 1, Status: "received", Answer: "PR の head が動いた（9818f9fa1df4 → 1ed1e1eb3bea）。gate が今の head で決め直した（記録 /k/issues/S-0001/library/gate/" + sha('1') + ".json）", Q: q("line", "gate", "オーナーが PR を読んでマージする", "直させる")},
				{N: 2, Status: "received", Answer: "オーナーがマージした（https://github.com/example/sample-site/pull/438）", Q: q("line", "gate", "オーナーが PR を読んでマージする", "直させる")},
				{N: 3, Status: "received", Answer: "PR の head が gate のあとに動いた（9818f9fa1df4 → 1ed1e1eb3bea）。gate が今の head で回し直す", Q: q("line", "gate", "オーナーが PR を読んでマージする", "直させる")},
				{N: 4, Status: "received", Answer: "直させる", Q: q("line", "gate", "オーナーが PR を読んでマージする", "直させる")},
				{N: 5, Status: "received", Answer: "オーナーがマージした（PR は読んだ）。でも次からは直させる", Q: q("line", "gate", "オーナーが PR を読んでマージする", "直させる")}},
			nil, "", nil, nil,
			`[{"kind":"override","question":4,"recommended":"オーナーが PR を読んでマージする","answer":"直させる"},` +
				`{"kind":"override","question":5,"recommended":"オーナーが PR を読んでマージする","answer":"オーナーがマージした（PR は読んだ）。でも次からは直させる"}]`},
		{"オーナーの commit がある PR → pr",
			nil, &contract.Report{Commit: sha('a')}, "major", pr("MERGED", sha('a'), sha('b')),
			[]HumanCommit{{Sha: sha('b'), Headline: "msg b", Files: []string{"docs/a.md"}}},
			`[{"kind":"pr","comments":[],"human_commits":[{"sha":"` + sha('b') + `","headline":"msg b","files":["docs/a.md"]}]}]`},
		{"オーナーが waive した指摘（must_fix 以上・マージ済み）→ override",
			nil, &contract.Report{Waived: []contract.Waived{{Severity: "major", File: "a.go", Line: &line, Reason: "オーナーが許した"}}}, "major", pr("MERGED"), nil,
			`[{"kind":"override","finding":{"severity":"major","file":"a.go","line":3},"reason":"オーナーが許した"}]`},
		{"インラインのレビューコメントとレビューの本文 → pr（Bot のものと gate のインラインコメントは数えない）",
			nil, &contract.Report{Commit: sha('a')}, "major",
			func() *PRView {
				v := pr("MERGED", sha('a'))
				v.Reviews = append(v.Reviews, struct {
					Author struct {
						Login string `json:"login"`
					} `json:"author"`
					Body        string `json:"body"`
					SubmittedAt string `json:"submittedAt"`
				}{Body: "全体に語を揃える", SubmittedAt: "2026-11-09T01:00:00Z"}, struct {
					Author struct {
						Login string `json:"login"`
					} `json:"author"`
					Body        string `json:"body"`
					SubmittedAt string `json:"submittedAt"`
				}{Body: ""})
				var human, bot InlineComment
				human.User.Login, human.Body, human.CreatedAt, human.Path, human.OriginalLine = "o", "ここは kanban の語", "2026-11-09T02:00:00Z", "docs/a.md", &line
				bot.User.Login, bot.User.Type, bot.Body = "ci", "Bot", "lint"
				var gate InlineComment
				gate.User.Login, gate.User.Type, gate.Body = "o", "User", "andon gate: **fix**"
				v.Inline = []InlineComment{human, bot, gate}
				return v
			}(), nil,
			`[{"kind":"pr","comments":[{"author":"?","at":"2026-11-09T01:00:00Z","body":"全体に語を揃える"},{"author":"o","at":"2026-11-09T02:00:00Z","body":"ここは kanban の語","file":"docs/a.md","line":3}],"human_commits":[]}]`},
		{"マージしていない PR の waived は数えない",
			nil, &contract.Report{Waived: []contract.Waived{{Severity: "major", File: "a.go", Line: &line, Reason: "r"}}}, "major", pr("CLOSED"), nil, `[]`},
		{"must_fix が無ければ waived は全部数える（多めに拾う）",
			nil, &contract.Report{Waived: []contract.Waived{{Severity: "trivial", File: "a.go", Line: &line, Reason: "r"}}}, "", pr("MERGED"), nil,
			`[{"kind":"override","finding":{"severity":"trivial","file":"a.go","line":3},"reason":"r"}]`},
	}
	for _, c := range cases {
		if got := asJSON(t, Corrections(c.qs, c.rep, c.must, c.pr, c.human)); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestHumanCommits(t *testing.T) {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	v := &PRView{}
	for _, o := range []string{a, b, c} {
		v.Commits = append(v.Commits, struct {
			Oid             string `json:"oid"`
			MessageHeadline string `json:"messageHeadline"`
		}{o, o[:1]})
	}
	if got := HumanCommits(a, v); len(got) != 2 || got[0].Sha != b || got[1].Sha != c || got[0].Files == nil {
		t.Errorf("a の後は b・c: %v", got)
	}
	if got := HumanCommits(c, v); len(got) != 0 {
		t.Errorf("最後の commit が報告ならオーナーの commit は無い: %v", got)
	}
	if got := HumanCommits(strings.Repeat("d", 40), v); got != nil {
		t.Errorf("報告の commit が PR に無ければ数えない: %v", got)
	}
	if got := HumanCommits("", v); got != nil {
		t.Errorf("報告の commit が無ければ数えない: %v", got)
	}
}

func TestCorrectionsLine(t *testing.T) {
	got := line(Notification{Issue: "A-1", Kind: "corrections", Detail: map[string]any{"error": "2 回打って失敗: HTTP 502"}})
	want := "A-1 corrections: オーナーの手直しを集められなかった（2 回打って失敗: HTTP 502）。corrections.json は書いていない"
	if got != want {
		t.Errorf("line =\n %s\nwant\n %s", got, want)
	}
}

// TestMachineGateAnswersMatchWriters は、機械が gate の問いを閉じる答えの書き方（internal/issue の settle と merge）を
// ソースから読み、machineGateAnswer がそれを機械の答えと見分けることを確かめる。書き方を変えて見分けがずれるか、
// gate の問いを閉じる書き手が増えると、機械が閉じた問いがまたオーナーの手直しに数えられるので、ここで落とす。
func TestMachineGateAnswersMatchWriters(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "issue", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	// 書き手の数: gate の問いに答えを書くのは closeQuestion だけで、その呼び手は settle の closeAndFinish・merge の
	// closeGateQuestions（呼び手は headMoved の1つ）・closeStaleGateQuestions。増えたら、その答えの形を machineGateAnswers と下に足す。
	files, err := filepath.Glob(filepath.Join("..", "issue", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]int{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src := read(filepath.Base(f))
		calls["closeQuestion"] += strings.Count(src, "c.closeQuestion(")
		calls["closeGateQuestions"] += strings.Count(src, "c.closeGateQuestions(")
		calls["answer"] += strings.Count(src, `"answer", `)
	}
	if want := map[string]int{"closeQuestion": 3, "closeGateQuestions": 1, "answer": 1}; fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Errorf("gate の問いを閉じる書き手の数が変わった: %v（前は %v）。足した書き手の答えを machineGateAnswers に足す", calls, want)
	}
	sha := strings.Repeat("1", 40)
	// settle.go: c.closeQuestion(name, q.Status, "オーナーがマージした（"+pr+"）")
	m := regexp.MustCompile(`closeQuestion\(name, q\.Status, "([^"]*)"\+pr\+"([^"]*)"\)`).FindStringSubmatch(read("settle.go"))
	if m == nil {
		t.Fatal("settle.go に gate の問いをマージで閉じる答えが見つからない")
	}
	merged := m[1] + "https://github.com/example/sample-site/pull/7" + m[2]
	// merge.go の closeStaleGateQuestions（頭の語が変わると見つからずに落ちる）: why := fmt.Sprintf("PR の head が動いた（%s → %s）。gate が今の head で決め直した（記録 %s）", …)
	f := regexp.MustCompile(`why := fmt\.Sprintf\("(PR の head が動いた[^"]*)",`).FindStringSubmatch(read("merge.go"))
	if f == nil {
		t.Fatal("merge.go に前の head の gate の問いを閉じる答えが見つからない")
	}
	regated := fmt.Sprintf(f[1], sha[:12], strings.Repeat("2", 12), "/k/issues/S-0001/library/gate/"+sha+".json")
	// merge.go の headMoved: c.closeGateQuestions(fmt.Sprintf("PR の head が gate のあとに動いた（%s → %s）。gate が今の head で回し直す", …), res)
	g := regexp.MustCompile(`c\.closeGateQuestions\(fmt\.Sprintf\("([^"]*)",`).FindStringSubmatch(read("merge.go"))
	if g == nil {
		t.Fatal("merge.go に head が動いて gate の問いを閉じる答えが見つからない")
	}
	moved := fmt.Sprintf(g[1], sha[:12], strings.Repeat("2", 12))
	for _, a := range []string{merged, regated, moved} {
		if !machineGateAnswer(a) {
			t.Errorf("機械が閉じた答えを見分けない: %q", a)
		}
	}
	// 見分けが広すぎないこと: オーナーが書いた答え（推奨案・ほかの案・文の頭だけ同じもの）は機械の答えでない
	for _, a := range []string{"オーナーが PR を読んでマージする", "担当に直させる（理由を Chief of Staff が返す）", merged + "。でも次からは直させる", "PR の head が動いた（前のはだめ）"} {
		if machineGateAnswer(a) {
			t.Errorf("オーナーの答えを機械の答えと見分けた: %q", a)
		}
	}
}
