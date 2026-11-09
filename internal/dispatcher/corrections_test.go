package dispatcher

import (
	"encoding/json"
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
		{"手直しが無い（推奨どおりの答え・ゲートのコメントだけ・軽い waived）",
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
		{"ゲートの問いへの答え → override（推奨どおりでも）・scope は数えない・答えの無い問いと JSON の無い問いも数えない",
			[]QA{{N: 1, Status: "received", Answer: "人が PR を読んでマージする", Q: q("line", "gate", "人が PR を読んでマージする", "直させる")},
				{N: 2, Status: "received", Answer: "c2 だけ", Q: q("scope", "issue", "c1 と c2", "c2 だけ")},
				{N: 3, Status: "open", Q: q("users", "issue", "A", "B")},
				{N: 4, Status: "received", Answer: "B"}},
			nil, "", nil, nil,
			`[{"kind":"override","question":1,"recommended":"人が PR を読んでマージする","answer":"人が PR を読んでマージする"}]`},
		{"人の commit がある PR → pr",
			nil, &contract.Report{Commit: sha('a')}, "major", pr("MERGED", sha('a'), sha('b')),
			[]HumanCommit{{Sha: sha('b'), Headline: "msg b", Files: []string{"docs/a.md"}}},
			`[{"kind":"pr","comments":[],"human_commits":[{"sha":"` + sha('b') + `","headline":"msg b","files":["docs/a.md"]}]}]`},
		{"人が waive した指摘（must_fix 以上・マージ済み）→ override",
			nil, &contract.Report{Waived: []contract.Waived{{Severity: "major", File: "a.go", Line: &line, Reason: "人が許した"}}}, "major", pr("MERGED"), nil,
			`[{"kind":"override","finding":{"severity":"major","file":"a.go","line":3},"reason":"人が許した"}]`},
		{"インラインのレビューコメントとレビューの本文 → pr（Bot のものとゲートのインラインコメントは数えない）",
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
				human.User.Login, human.Body, human.CreatedAt, human.Path, human.OriginalLine = "o", "ここは台帳の語", "2026-11-09T02:00:00Z", "docs/a.md", &line
				bot.User.Login, bot.User.Type, bot.Body = "ci", "Bot", "lint"
				var gate InlineComment
				gate.User.Login, gate.User.Type, gate.Body = "o", "User", "andon gate: **fix**"
				v.Inline = []InlineComment{human, bot, gate}
				return v
			}(), nil,
			`[{"kind":"pr","comments":[{"author":"?","at":"2026-11-09T01:00:00Z","body":"全体に語を揃える"},{"author":"o","at":"2026-11-09T02:00:00Z","body":"ここは台帳の語","file":"docs/a.md","line":3}],"human_commits":[]}]`},
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
		t.Errorf("最後の commit が報告なら人の commit は無い: %v", got)
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
	want := "A-1 corrections: 人の手直しを集められなかった（2 回打って失敗: HTTP 502）。corrections.json は書いていない"
	if got != want {
		t.Errorf("line =\n %s\nwant\n %s", got, want)
	}
}
