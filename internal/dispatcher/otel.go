package dispatcher

import (
	"time"

	"andon/internal/otel"
)

// otelPrune はハーネスの生の記録の寿命の片付け（1 巡の 5f。dispatcher.md §2.14・§7.2 #30。ADR-0060 決めたこと 10）:
// プロジェクトの置き場（~/.local/state/andon/otel/<slug>/）を、前の片付けから 1 日たっていれば、30 日と 10 GB で片付ける。
// 定期の処理を走らせるのは dispatcher だけで、受け口は片付けない。失敗はログに 1 行（同じ理由は繰り返さない）で、巡は止めない。
func (p *project) otelPrune() {
	prune := p.c.OtelPrune
	if prune == nil {
		prune = otel.PruneDue
	}
	ran, removed, err := prune(p.l.Slug, time.Now())
	if err != nil {
		reason := err.Error()
		if p.c.pruneLogged[p.l.Slug] != reason {
			if p.c.pruneLogged == nil {
				p.c.pruneLogged = map[string]string{}
			}
			p.c.pruneLogged[p.l.Slug] = reason
			p.logf("ハーネスの生の記録を片付けられない（巡は続ける。次の日に走らせ直す）: %v", err)
		}
		return
	}
	delete(p.c.pruneLogged, p.l.Slug)
	if ran && removed > 0 {
		p.logf("ハーネスの生の記録を %d 本消した（寿命 %d 日・上限 %d GB）", removed, otel.Default.Days, otel.Default.MaxBytes>>30)
	}
}
