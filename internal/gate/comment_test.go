package gate

import (
	"regexp"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// insideWords は PR を読む外の人に通じない内輪の語（ADR-0026 受け入れ条件 A5）。
var insideWords = regexp.MustCompile(`(?i)needs_human|tier|delegation|report\.json|issue\.yaml|条件表|台帳|Must fix|screen_paths|size_(?:human|review)_lines|one_way|担当|セル`)

// recordReasons は記録の reasons に入りうる文の全部（Decide の出力と、run が Facts に入れる文）。
// run の文を足したり変えたりしたら、ここにも足す（外向けの文の表に無ければ下の検査が落ちる）。
func recordReasons(t *testing.T) []string {
	good := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"go test ./...", true}}, Review: ledger.Review{State: "current"},
		GitHubChecks: GitHubChecks{State: "success", Names: []string{}}, ChecksWait: 30}
	mods := []func(*Facts){
		func(f *Facts) {
			f.LedgerReasons = []string{"台帳の課題が review でない（doing）", "PR の branch（feat）が issue.yaml の branch と違う",
				"report.json の commit が PR の head と違う（andon issue ready からやり直す）"}
		},
		func(f *Facts) { f.PROpen = false },
		func(f *Facts) {
			f.Blockers = []string{"files の件数が changedFiles と合わない（一覧の上限・取得中の変更）", "conflict", "fork の PR",
				"base が既定ブランチでない", "作者がセルの gh アカウントでない"}
		},
		func(f *Facts) {
			f.SettingsReason = "issue.yaml の review・must_fix が無い・知らない値（Chief of Staff が kanban set する）"
		},
		func(f *Facts) {
			f.SettingsReason = "issue.yaml の review・must_fix を最後に書いたのが Chief of Staff（config.yaml の chief_of_staff）でない・記録と値が違う"
		},
		func(f *Facts) { f.RepoKnown = false },
		func(f *Facts) { f.Tier = "C" },
		func(f *Facts) { f.Checks[0].OK = false },
		func(f *Facts) { f.GitHubChecks = GitHubChecks{"failure", []string{"test"}} },
		func(f *Facts) { f.GitHubChecks = GitHubChecks{"unknown", []string{"test"}} },
		func(f *Facts) { f.Review = ledger.Review{State: "missing", Reason: "findings.json が無い"} },
		func(f *Facts) { f.Review = ledger.Review{State: "stale", Reason: "findings.json の commit が古い"} },
		func(f *Facts) { f.Review.MustFixOpen = 2 },
		func(f *Facts) { f.Review.MustFixOpen = 2; f.PreviousMustFix = true },
		func(f *Facts) { f.Review.WaivedHeavy = 1 },
		func(f *Facts) { f.AcceptanceFail = 1 },
		func(f *Facts) { f.ScreenMissing = []string{"web/a.css"} },
		func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"test"}} },
		func(f *Facts) { f.GitHubChecks = GitHubChecks{State: "none"}; f.ChecksOverdue = true },
		func(f *Facts) { f.GitHubChecks = GitHubChecks{State: "none"} },
	}
	var out []string
	for _, m := range mods {
		f := good
		f.Checks = append([]Check{}, good.Checks...)
		m(&f)
		if _, r := Decide(f); len(r) == 0 {
			t.Fatalf("理由が出ない: %+v", f)
		} else {
			out = append(out, r...)
		}
	}
	return out
}

// recordTierReasons は記録の tier_reasons に入りうる文の全部（Classify と FromReport の出力）。
func recordTierReasons(t *testing.T) []string {
	tb := pkgTable(t)
	human, review := 3, 1
	tb.SizeHumanLines, tb.SizeReviewLines = &human, &review
	tb.TierCAddedLinePatterns = []string{`curl .*\| *sh`}
	repo := RepoTable{TierOverrides: map[string]string{"ops/**": "C", "lib/**": "B"}}
	files := []File{{Path: "package/gate.json", Additions: 5}, {Path: "internal/x.go", Additions: 1}, {Path: "ops/run.sh"}, {Path: "lib/a.txt"}}
	var out []string
	cl, err := tb.Classify(repo, files, "+curl https://example.com/i | sh\n", true)
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, cl.TierReasons...)
	cl, err = tb.Classify(repo, []File{{Path: "internal/x.go", Additions: 2}, {Path: "lib/a.txt"}}, "", true)
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, cl.TierReasons...)
	yes := true
	cl, _ = FromReport(Classified{Tier: "A", TierReasons: []string{}}, &contract.Report{
		Acceptance: []contract.Acceptance{{ID: "A1", Result: "manual"}}, Risk: &contract.Risk{OneWay: &yes}})
	return append(out, cl.TierReasons...)
}

// A5: コメントと commit status の説明文に内輪の語が出ない。記録の文には出る（検査が何かを見ていることの確かめ）
func TestCommentHasNoInsideWords(t *testing.T) {
	reasons, tierReasons := recordReasons(t), recordTierReasons(t)
	for _, want := range []string{"needs_human", "tier", "delegation", "report.json", "issue.yaml", "条件表"} {
		if !insideWords.MatchString(strings.Join(append(append([]string{want}, reasons...), tierReasons...), "\n")) {
			t.Fatalf("内輪の語の検査が %q に当たらない", want)
		}
	}
	if !insideWords.MatchString(strings.Join(reasons, "\n")) || !insideWords.MatchString(strings.Join(tierReasons, "\n")) {
		t.Fatal("記録の文に内輪の語が無い（検査が何も見ていない）")
	}
	for _, r := range reasons {
		if got := outsideReason(r); got == otherReason {
			t.Errorf("記録の理由に外向けの文が無い: %q", r)
		}
	}
	for _, r := range tierReasons {
		if got := outsideTierReason(r); got == otherReason {
			t.Errorf("記録の区分の理由に外向けの文が無い: %q", r)
		}
	}
	for _, d := range []string{"approve", "fix", "needs_human"} {
		for _, tier := range []string{"A", "B", "C"} {
			rec := &Record{Head: strings.Repeat("a", 40), At: "2026-11-19T00:00:00Z", Tier: tier, TierReasons: tierReasons,
				ChangedLines: 10, Checks: []Check{{"go test ./...", false}}, GitHubChecks: GitHubChecks{"failure", []string{"test"}},
				Review: ledger.Review{State: "current", MustFixOpen: 1}, Decision: d, Reasons: reasons}
			rec.Delegation.Merge = "none"
			for name, s := range map[string]string{"コメント": CommentBody(rec), "commit status": StatusDescription(rec)} {
				if !strings.HasPrefix(s, "andon gate: ") {
					t.Errorf("%s %s: 頭が andon gate: でない: %q", d, name, s)
				}
				if m := insideWords.FindString(s); m != "" {
					t.Errorf("%s %s に内輪の語 %q:\n%s", d, name, m, s)
				}
			}
			if n := len([]rune(StatusDescription(rec))); n > 140 {
				t.Errorf("commit status の説明文が GitHub の上限 140 字を超える: %d", n)
			}
		}
	}
}

// 判定ごとに見出しが違う（外の人が判定を読める）
func TestCommentHeadlinePerDecision(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range []string{"approve", "fix", "needs_human"} {
		h := strings.SplitN(CommentBody(&Record{Decision: d, Tier: "B"}), "\n", 2)[0]
		if seen[h] {
			t.Errorf("%s の見出しがほかの判定と同じ: %q", d, h)
		}
		seen[h] = true
	}
}
