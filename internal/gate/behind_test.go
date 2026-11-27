package gate

import (
	"reflect"
	"testing"

	"andon/internal/ledger"
)

// TestDecideBehind は ADR-0067 受け入れ条件 4・5 の判定の表: 積んだ課題（信じられる stacked_on がある）の PR は、
// 担当へ返す理由が無いとき、今なら approve かオーナー待ちになる判定を behind（前の PR 待ち）に置き換える。
// オーナーへ回す理由は判定を止めずに写し、担当へ返す理由（fix）と待ち（wait）は今と同じに返す。
func TestDecideBehind(t *testing.T) {
	good := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"bash tests/*.sh", true}}, Review: ledger.Review{State: "current"},
		GitHubChecks: GitHubChecks{State: "success", Names: []string{}}, ChecksWait: 30,
		StackedOn: "S-0001", StackedDraft: true, StackedBaseOK: true}
	cases := []struct {
		name string
		mod  func(*Facts)
		want string
		held string // behind の reasons の 2 行目（写したオーナーへ回す理由。空なら 1 行だけ）
	}{
		{"揃えば behind", func(*Facts) {}, "behind", ""},
		// 受け入れ条件 4: 今ならオーナーへ回す理由があっても behind（理由は写す）
		{"Must fix が同じ head で2度目", func(f *Facts) { f.Review.MustFixOpen = 1; f.PreviousMustFix = true }, "behind",
			"Must fix 以上が 1 件、同じ head で2度目（担当は直さないと判断した）"},
		{"マージがオーナーのもの（委譲の外）", func(f *Facts) { f.Tier = "C" }, "behind", "delegation: merge=B < tier C"},
		{"作者がセルのアカウントでない", func(f *Facts) { f.Blockers = []string{"作者がセルの gh アカウントでない"} }, "behind", "作者がセルの gh アカウントでない"},
		{"条件表の設定が信じられない", func(f *Facts) {
			f.SettingsReason = "issue.yaml の review・must_fix を最後に書いたのが Chief of Staff でない"
		}, "behind",
			"issue.yaml の review・must_fix を最後に書いたのが Chief of Staff でない"},
		{"repo が条件表に無い", func(f *Facts) { f.RepoKnown = false }, "behind", "条件表の repos にこの repo が無い（検査を回せない）"},
		{"Must fix を見送った", func(f *Facts) { f.Review.WaivedHeavy = 1 }, "behind", "Must fix 以上を 1 件見送った（担当がレビュアーを覆した）"},
		{"通すか決められない checks", func(f *Facts) { f.GitHubChecks = GitHubChecks{"unknown", []string{"x"}} }, "behind",
			"GitHub の checks に通すか決められない状態がある: x"},
		{"checks が無いまま時間が過ぎた", func(f *Facts) { f.GitHubChecks = GitHubChecks{State: "none"}; f.ChecksOverdue = true }, "behind",
			"GitHub の checks が1つも無いまま 30 分過ぎた（条件表の github_checks: optional でない）"},
		// 受け入れ条件 4: 古い base の 2 つの向きでは rebase を求めない（前の課題の branch が進んでいても）
		{"PR が検査の置き場を変えて base が進んだ", func(f *Facts) { f.OwnChecks = []string{"tests/a.sh"}; f.BaseAhead = 3 }, "behind", ""},
		{"base で検査の置き場が動いた", func(f *Facts) { f.StaleBase = []string{"tests/a.sh"}; f.BaseUnread = true }, "behind", ""},
		// 受け入れ条件 5: 担当へ返す
		{"draft でない", func(f *Facts) { f.StackedDraft = false }, "fix", ""},
		{"向き先が前の課題の branch でも既定の branch でもない", func(f *Facts) { f.StackedBaseOK = false }, "fix", ""},
		{"PR が open でない", func(f *Facts) { f.PROpen = false }, "fix", ""},
		{"検査が落ちた", func(f *Facts) { f.Checks[0].OK = false }, "fix", ""},
		{"Must fix が残る", func(f *Facts) { f.Review.MustFixOpen = 1 }, "fix", ""},
		{"レビューが無い", func(f *Facts) { f.Review.State = "missing" }, "fix", ""},
		{"GitHub の checks が落ちた", func(f *Facts) { f.GitHubChecks = GitHubChecks{"failure", []string{"test"}} }, "fix", ""},
		{"オーナーへ回す理由があっても担当へ返す理由を先に返す", func(f *Facts) {
			f.Blockers = []string{"作者がセルの gh アカウントでない"}
			f.Review.MustFixOpen = 1
		}, "fix", ""},
		{"受け入れ条件の fail", func(f *Facts) { f.AcceptanceFail = 1 }, "fix", ""},
		// 待ちは今のまま
		{"checks が走っている", func(f *Facts) { f.GitHubChecks = GitHubChecks{"pending", []string{"test"}} }, "wait", ""},
		{"checks がまだ無い", func(f *Facts) { f.GitHubChecks = GitHubChecks{State: "none"} }, "wait", ""},
	}
	for _, c := range cases {
		f := good
		f.Checks = append([]Check{}, good.Checks...)
		c.mod(&f)
		d, reasons := Decide(f)
		if d != c.want {
			t.Errorf("%s: %s %v want %s", c.name, d, reasons, c.want)
			continue
		}
		if d != "behind" {
			continue
		}
		want := []string{"前の課題 S-0001 の PR がまだ main に入っていない"}
		if c.held != "" {
			want = append(want, c.held)
		}
		if !reflect.DeepEqual(reasons, want) {
			t.Errorf("%s: 理由 %q want %q", c.name, reasons, want)
		}
		if ExitCode(d) != 6 {
			t.Errorf("%s: behind の終了コード %d", c.name, ExitCode(d))
		}
	}
	// 積んでいない課題（stacked_on を信じない課題を含む）は、今までどおり draft を返し、オーナーへ回す
	f := good
	f.StackedOn = ""
	f.Blockers = []string{"base が既定ブランチでない"}
	if d, _ := Decide(f); d != "needs_human" {
		t.Errorf("積んでいない課題の base の理由: %s want needs_human", d)
	}
}
