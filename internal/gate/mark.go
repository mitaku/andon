package gate

import (
	"strconv"
	"strings"
)

// Mark は PR の head の commit に付ける印（commit status andon/gate）の状態と説明文。
// 状態は success（緑）・failure（赤）・pending（黄）で、空は印が無い（ADR-0056 決めたこと 3）。
type Mark struct {
	State       string
	Description string
}

// 判定のほかの印の説明文（外の人が読む文。ADR-0026 決めたこと 9・ADR-0056 決めたこと 3・6）。
// 止めた理由の課題の名前は書かない（PR は外の人も読む）。doing に戻ったときの赤は、ADR-0056 の文（「担当が」）を、
// 外の読み手に内輪の語を出さない決まりに合わせて、差し戻しの見出しと同じ「作った側が」と書く。
const (
	heldDescription     = CommentPrefix + "メンテナーの確認を待っています"
	reworkDescription   = CommentPrefix + "作った側が直しています"
	documentDescription = CommentPrefix + "仕様・設計の文書です。メンテナーがマージします"
	// behindDescription は前の PR 待ち（behind）の黄。理由を足さない（前の課題の名前を外に出さない。ADR-0067 決めたこと 8）
	behindDescription = CommentPrefix + "前の PR のマージを待っている"
)

// descriptionLimit は commit status の説明文の上限（GitHub は 140 字を超えると断る）。
const descriptionLimit = 140

// DecisionMark は、通した（approve）とオーナー待ち（needs_human）は緑、差し戻し（fix）は赤、前の PR 待ち（behind）は黄。
// wait と知らない判定は書かない（false）。
func DecisionMark(decision string, reasons []string) (Mark, bool) {
	switch decision {
	case "behind":
		return Mark{"pending", behindDescription}, true
	case "approve", "needs_human":
		return Mark{"success", describe(decision, reasons)}, true
	case "fix":
		return Mark{"failure", describe(decision, reasons)}, true
	}
	return Mark{}, false
}

// describe は判定の説明文: 「andon gate: <見出し>（<最初の理由>）」。理由が 2 件以上なら「、ほか N 件」を足す。
// approve と理由の無い判定は見出しだけ。140 字を超えるときは理由を「…」で切る（ADR-0054 決めたこと 5）。
// 理由は PR のコメントと同じ外向けの文（outsideReason）で、記録の文をそのまま出さない。
func describe(decision string, reasons []string) string {
	head := CommentPrefix + headline(decision)
	if decision == "approve" || len(reasons) == 0 {
		return head
	}
	more := ""
	if len(reasons) > 1 {
		more = "、ほか " + strconv.Itoa(len(reasons)-1) + " 件"
	}
	reason := []rune(outsideReason(reasons[0]))
	room := descriptionLimit - len([]rune(head+"（"+more+"）"))
	if len(reason) > room {
		reason = append(reason[:max(room-1, 0)], '…')
	}
	return head + "（" + string(reason) + more + "）"
}

// MarkFacts は、dispatcher が PR を読んだときに、今の head にあるべき印を決める事実。
type MarkFacts struct {
	Status   string // 課題の status（review・doing。ほかは書かない）
	Engineer bool   // Chief Engineer の文書の PR（gate を通らない。review に入ったら緑。ADR-0056 決めたこと 6）
	Decision string // dispatcher が持つ、今の head を gate が判定した結果（無ければ空）
	Reasons  []string
	Held     bool
	Have     Mark
	// HaveDescUnknown は、今ある印の説明文を読めていないこと（gh pr view の statusCheckRollup は説明文を出さない）。状態だけで比べる
	HaveDescUnknown bool
}

// WantMark は、今の head にあるべき印と、書くかどうかを返す（純粋関数。ADR-0056 決めたこと 3・5・6）。
// 書かない（false）のは、あるべき印が無いとき（判定の前・wait・止めていて渡していない・doing で緑が残っていない）と、
// 同じ状態と説明文の印が既にあるとき。
//
//	止めていて、渡している（判定が approve か needs_human・CE の文書の PR が review・今の印が緑） → 黄（pending）
//	止めていて、渡していない                                                                   → 書かない
//	doing で、今の印が緑                                                                         → 赤（担当が直しています）
//	CE の文書の PR が review                                                                     → 緑（仕様・設計の文書です）
//	review の判定                                                                                → DecisionMark（approve・needs_human は緑、fix は赤、ほかは書かない）
func WantMark(f MarkFacts) (Mark, bool) {
	want, ok := wantMark(f)
	if !ok || Same(want, f.Have, f.HaveDescUnknown) {
		return Mark{}, false
	}
	return want, true
}

func wantMark(f MarkFacts) (Mark, bool) {
	released := f.Have.State == "success" || f.Decision == "approve" || f.Decision == "needs_human" || (f.Engineer && f.Status == "review")
	switch {
	case f.Held:
		if released {
			return Mark{"pending", heldDescription}, true
		}
		return Mark{}, false
	case f.Status == "doing":
		if f.Have.State == "success" {
			return Mark{"failure", reworkDescription}, true
		}
		return Mark{}, false
	case f.Status != "review":
		return Mark{}, false
	case f.Engineer:
		return Mark{"success", documentDescription}, true
	}
	return DecisionMark(f.Decision, f.Reasons)
}

func Same(want, have Mark, descUnknown bool) bool {
	return want.State == have.State && (descUnknown || want.Description == have.Description)
}

// GateMark は statusCheckRollup の中の gate 自身の印（context が andon/gate の StatusContext の最新）。無ければ空。
func GateMark(items []CheckItem) Mark {
	for _, it := range Latest(items) {
		if it.Typename == "StatusContext" && it.Context == GateContext {
			return Mark{strings.ToLower(it.State), it.Description}
		}
	}
	return Mark{}
}

// GateLink は gate 自身の印（GateMark と同じもの）のリンク（target_url）。無ければ空。
// 前のバージョンはリンクの無い印を書いたので、状態と説明文が同じでもリンクが無ければ gate は書き直す（ADR-0054 決めたこと 6）。
func GateLink(items []CheckItem) string {
	for _, it := range Latest(items) {
		if it.Typename == "StatusContext" && it.Context == GateContext {
			return it.TargetURL
		}
	}
	return ""
}
