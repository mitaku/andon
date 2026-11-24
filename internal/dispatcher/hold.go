package dispatcher

import "andon/internal/ledger"

// heldOn は、課題 reason が done になった行を読んだとき、それを理由に止めている課題（予約キー hold）ごとに通知 hold を積む
// （ADR-0056 決めたこと 7）。止めは理由の課題が済んでも外れない（外すのは andon issue unhold だけ）。Chief of Staff が
// 答えを担当へ伝えて外すのを忘れないように、done の行ごとに1回だけ知らせる。片付いた課題は数えない。
func (p *project) heldOn(reason string) {
	cos := p.l.ChiefOfStaff()
	for _, t := range p.tasks {
		if t.Str("status") == "done" {
			continue
		}
		if r, held := ledger.HoldOf(p.rows, p.tasks, cos, t); held && ledger.CanonID(r) == reason {
			p.notify(t.Str("id"), "hold", map[string]any{"reason": reason})
		}
	}
}
