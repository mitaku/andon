package dispatcher

import (
	"errors"
	"time"

	"andon/internal/delegate"
	"andon/internal/otel"
)

// otelPrune はハーネスの生の記録の寿命の片付け（1 巡の 5f。dispatcher.md §2.14・§7.2 #30。ADR-0060 決めたこと 10・11）:
// プロジェクトの置き場（~/.local/state/andon/otel/<slug>/）を、前の片付けから 1 日たっていれば、project.json の otel の寿命と上限
// （無ければ 30 日と 10 GB）で片付ける。project.json が信じられない・形が違うときは既定で片付け、ログに 1 行。
// 定期の処理を走らせるのは dispatcher だけで、受け口は片付けない。失敗はログに 1 行（同じ理由は繰り返さない）で、巡は止めない。
func (p *project) otelPrune() {
	prune := p.c.OtelPrune
	if prune == nil {
		prune = otel.PruneDue
	}
	lim := otel.Default
	var readErr error
	limits := func() otel.Limits {
		pj, err := delegate.ReadProject(p.c.WS, p.l.Slug)
		switch {
		case errors.Is(err, delegate.ErrAbsent):
		case err != nil:
			readErr = err
		default:
			lim = otel.LimitsOf(pj.Otel)
		}
		return lim
	}
	ran, removed, err := prune(p.l.Slug, time.Now(), limits)
	if readErr != nil {
		p.logf("project.json の otel が読めないので、ハーネスの生の記録を既定の %d 日・%d GB で片付けた: %v", lim.Days, lim.MaxBytes>>30, readErr)
	}
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
		p.logf("ハーネスの生の記録を %d 本消した（寿命 %d 日・上限 %d GB）", removed, lim.Days, lim.MaxBytes>>30)
	}
}
