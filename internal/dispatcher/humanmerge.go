package dispatcher

import (
	"encoding/json"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// humanMerge は review・doing の課題の PR をオーナーがマージしたかを andon issue settle に見させ、返った通知を積む（dispatcher.md §2.7）。
// settle は同じ読みで、開いている PR の印（andon/gate）を課題の状態に揃え、そのことを Log に返す（ADR-0056 決めたこと 5）。
// 拾う・片付ける・自分の記録を持つのは settle で、dispatcher は打って積むだけ（ADR-0017 決めたこと 5・6）。
// 失敗はログに1行で、次の見回りで打ち直す（settle が PR ごとに1分に1回だけ GitHub に聞く）。
func (p *project) humanMerge(t ledger.Issue, rep *contract.Report) {
	if rep.PR == "" {
		return
	}
	id := t.Str("id")
	out, _, err := p.andon("", "issue", "settle", id)
	var res contract.Settle
	if err == nil {
		err = json.Unmarshal([]byte(out), &res)
	}
	if err != nil {
		p.logf("%s のオーナーのマージを見られない（次の見回りで打ち直す）: %v", id, err)
		return
	}
	for _, l := range res.Log { // PR の印を揃えた・揃えられなかった（ADR-0056 決めたこと 5）
		p.logf("%s", clean(l, 300))
	}
	switch res.State {
	case "finished":
		p.logf("%s の PR %s はオーナーがマージしたので片付けた", id, rep.PR)
	case "held":
		p.logf("%s の PR %s はオーナーがマージしたが片付けない", id, rep.PR)
	case "closed":
		p.logf("%s の PR %s はマージされずに閉じた（片付けない）", id, rep.PR)
	}
	for _, n := range res.Notify {
		if held, ok := n.Detail["held"]; ok {
			n.Detail["held"] = clean(held, 200)
		}
		p.notify(id, n.Kind, n.Detail)
	}
}
