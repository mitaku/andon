package dispatcher

import (
	"encoding/json"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// humanMerge は review の課題の PR を人がマージしたかを andon issue settle に見させ、返った通知を積む（dispatcher.md §2.7）。
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
		p.logf("%s の人のマージを見られない（次の見回りで打ち直す）: %v", id, err)
		return
	}
	switch res.State {
	case "finished":
		p.logf("%s の PR %s は人がマージしたので片付けた", id, rep.PR)
	case "held":
		p.logf("%s の PR %s は人がマージしたが片付けない", id, rep.PR)
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
