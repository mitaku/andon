// dispatcher は retro を読む（巡の件数のきっかけ）ので、dispatcher の文を使う検査は外の検査の package に置く（import の循環を避ける）。
package retro_test

import (
	"testing"

	"andon/internal/dispatcher"
	"andon/internal/issue"
	"andon/internal/retro"
	"andon/internal/spawn"
)

// 端末の外から送った文は、送り手の印（spawn.Mark）で照らす。印の無い文と、文の中ほどの角括弧はオーナーの文として残す。
func TestMachineText(t *testing.T) {
	for s, want := range map[string]bool{
		spawn.Mark(spawn.FromDispatcher, dispatcher.RetryText):                       true,
		spawn.Mark(spawn.FromGate, "A-0017（PR #262 の衝突を解く）: gate が fix を返した。理由: x。"): true,
		"<pasted_content id=\"1\">\n[gate] A-0017（タイトル）: gate が fix を返した。":           true,
		issue.ResumeMessage("sample-site", "S-7"):                                    true,
		"  [dispatcher] 課題 `sample-site/S-7`。":                                       true,
		"[dispatcher]の文はどこから？":                                                       false,
		"[cos] オーナーが書いた角括弧":                                                          false,
		"gate の [gate] 印は何？":                                                         false,
		"A-0017 の gate が fix を返したのはなぜ？":                                              false,
	} {
		if got := retro.MachineText(s); got != want {
			t.Errorf("%q → %v", s, got)
		}
	}
}
