package retro_test

import (
	"testing"

	"andon/internal/dispatcher"
	"andon/internal/issue"
	"andon/internal/retro"
	"andon/internal/spawn"
)

const resumeBody = "Andon の担当として働く。system prompt に入った skill worker の決まりに従う。\n課題（依頼書の場所）はこの下に続く。"

func TestMachineText(t *testing.T) {
	for s, want := range map[string]bool{
		spawn.Mark(spawn.FromDispatcher, dispatcher.RetryText):                       true,
		spawn.Mark(spawn.FromGate, "A-0017（PR #262 の衝突を解く）: gate が fix を返した。理由: x。"): true,
		"<pasted_content id=\"1\">\n[gate] A-0017（タイトル）: gate が fix を返した。":           true,
		issue.ResumeMessage("sample-site", "S-7"):                                    true,
		// hc run は template の本文、空行、最初のメッセージの順に入力欄へ貼る。印は最後の段落の頭に来る
		resumeBody + "\n\n" + issue.ResumeMessage("sample-site", "S-7"):                                                                              true,
		"\n\n<pasted_content id=\"b23d\">\n" + resumeBody + "\n\n" + issue.ResumeMessage("sample-site", "S-7") + "\n</pasted_content id=\"b23d\">\n": true,
		"段落1\n\n[resume] の印は何？":                               true,
		"段落1\n\n印の [resume] は何？":                              false,
		resumeBody + "\n\n課題 `sample-site/S-7` の続き。セルが起き直した。": false,
		"  [dispatcher] 課題 `sample-site/S-7`。":                true,
		"[dispatcher]の文はどこから？":                                false,
		"[cos] オーナーが書いた角括弧":                                   false,
		"gate の [gate] 印は何？":                                  false,
		"A-0017 の gate が fix を返したのはなぜ？":                       false,
	} {
		if got := retro.MachineText(s); got != want {
			t.Errorf("%q → %v", s, got)
		}
	}
}
