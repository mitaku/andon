package gate

import (
	"reflect"
	"testing"

	"andon/internal/ledger"
)

// TestDecidePrevDone は ADR-0067 決めたこと 12・14 の判定の表: 前の課題が done になった積んだ課題の PR は、
// 前の課題が main に入っていれば、ほかの理由より先に載せ替えを頼む直し（fix）を返し、マージされずに終わっていれば
// 待ちのほかは behind のまま 1 行目を替える。積んでいない課題と、open でない PR には載せ替えを頼まない。
func TestDecidePrevDone(t *testing.T) {
	good := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"bash tests/*.sh", true}}, Review: ledger.Review{State: "current"},
		GitHubChecks: GitHubChecks{State: "success", Names: []string{}}, ChecksWait: 30,
		StackedOn: "S-0001", StackedDraft: true, StackedBaseOK: true}
	restack := []string{"前の課題 S-0001 が main に入った。andon issue restack で載せ替える"}
	gone := "前の課題 S-0001 がマージされずに終わった"
	cases := []struct {
		name    string
		mod     func(*Facts)
		want    string
		reasons []string
		restack bool
	}{
		{"前の課題が main に入った", func(f *Facts) { f.PrevMerged = true }, "fix", restack, true},
		// ほかの理由より先: 担当へ返す理由・オーナーへ回す理由・待ち・kanban の食い違い・向き先・draft より先に載せ替えを頼む
		{"検査が落ちていても載せ替えが先", func(f *Facts) { f.PrevMerged = true; f.Checks[0].OK = false }, "fix", restack, true},
		{"Must fix が残っていても載せ替えが先", func(f *Facts) { f.PrevMerged = true; f.Review.MustFixOpen = 1 }, "fix", restack, true},
		{"checks が走っていても載せ替えが先", func(f *Facts) {
			f.PrevMerged = true
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
		}, "fix", restack, true},
		{"kanban と PR が合わなくても載せ替えが先", func(f *Facts) {
			f.PrevMerged = true
			f.LedgerReasons = []string{"kanban の課題が review でない（doing）"}
		}, "fix", restack, true},
		{"向き先が既定の branch に付け替わり draft のままでも載せ替えが先", func(f *Facts) { f.PrevMerged = true; f.StackedDraft = false }, "fix", restack, true},
		// open でない PR の担当には頼まない（決めたこと 15）
		{"PR が open でない", func(f *Facts) { f.PrevMerged = true; f.PROpen = false }, "fix", []string{"PR が open でない・draft"}, false},
		// マージされずに終わった（決めたこと 14）: behind のまま 1 行目を替え、ほかの理由を写す
		{"マージされずに終わった", func(f *Facts) { f.PrevGone = true }, "behind", []string{gone}, false},
		{"マージされずに終わり、オーナーへ回す理由がある", func(f *Facts) { f.PrevGone = true; f.Tier = "C" }, "behind",
			[]string{gone, "delegation: merge=B < tier C"}, false},
		{"マージされずに終わり、担当へ返す理由がある", func(f *Facts) { f.PrevGone = true; f.Review.MustFixOpen = 1 }, "behind",
			[]string{gone, "Must fix 以上が 1 件"}, false},
		{"マージされずに終わっても checks の待ちは待つ", func(f *Facts) {
			f.PrevGone = true
			f.GitHubChecks = GitHubChecks{"pending", []string{"test"}}
		}, "wait", []string{"GitHub の checks がまだ走っている: test"}, false},
		// 前の課題がまだ done でない（今まで）
		{"前の課題が done でない", func(*Facts) {}, "behind", []string{"前の課題 S-0001 の PR がまだ main に入っていない"}, false},
	}
	for _, c := range cases {
		f := good
		f.Checks = append([]Check{}, good.Checks...)
		c.mod(&f)
		d, reasons := Decide(f)
		if d != c.want || !reflect.DeepEqual(reasons, c.reasons) {
			t.Errorf("%s: %s %q want %s %q", c.name, d, reasons, c.want, c.reasons)
		}
		if got := d == "fix" && Restacking(f); got != c.restack {
			t.Errorf("%s: 載せ替えの印 %v want %v", c.name, got, c.restack)
		}
	}
	// 積んでいない課題は、PrevMerged が立っていても載せ替えを頼まない（信じられない stacked_on は Facts に入らない）
	f := good
	f.StackedOn, f.PrevMerged = "", true
	if d, r := Decide(f); d != "approve" || Restacking(f) {
		t.Errorf("積んでいない課題: %s %q want approve", d, r)
	}
}
