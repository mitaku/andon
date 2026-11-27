package contract

import "testing"

// ADR-0067 決めたこと 11: 積んだ課題（StackedOn がある）は、ぶつかり・重なり・検査の置き場の動きでは影響ありにせず、
// 依頼書が指すファイルの変更だけで影響ありにする。マージしたのが積んだ相手そのものなら、依頼書が指していても影響なし。
func TestAffectedStacked(t *testing.T) {
	moved := ImpactIssue{Status: "review", Conflict: true, Overlap: []string{"a.go"}, ChecksMoved: []string{"tests/x.sh"}}
	if !moved.Affected("A-0001") {
		t.Fatalf("積んでいない課題がぶつかり・重なり・検査の置き場の動きで影響ありにならない（見たいものを見ていない）")
	}
	moved.StackedOn = "A-0002"
	if moved.Affected("A-0001") {
		t.Errorf("積んだ課題がぶつかり・重なり・検査の置き場の動きで影響ありになった")
	}
	refs := ImpactIssue{Status: "doing", StackedOn: "A-0002", BriefRefs: []string{"docs/a.md"}}
	if !refs.Affected("A-0001") {
		t.Errorf("積んだ課題で、ほかの課題のマージが依頼書の指すファイルを変えても影響ありにならない")
	}
	if refs.Affected("A-0002") {
		t.Errorf("積んだ相手そのもののマージで影響ありになった（gate が載せ替えを頼む）")
	}
	bad := ImpactIssue{StackedOn: "A-0002", Error: "branch x が読めない"}
	if !bad.Affected("A-0002") {
		t.Errorf("読めなかった積んだ課題を影響なしに倒した")
	}
}
