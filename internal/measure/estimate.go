package measure

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"andon/internal/ledger"
)

// estimates は期間に最後の列へ動いた担当の課題（レビュアー・Chief Engineer・人の課題を除く。調べる課題を含む）ごとの
// 見込みと実際と、全体の収まった割合（ADR-0014 決めたこと 8・9・11 の 6。contract.md §13）。
// 見込みと実働は dispatcher と同じもの（ledger.Estimate・ledger.Worked）を読む。死んでいた区間は dispatcher の控えの gone。
func (r *reader) estimates(o Options, ids []string, byID map[string]*issue, doneAt map[string]time.Time, in func(time.Time) bool) (Estimate, []Issue) {
	sum, out := Estimate{}, []Issue{}
	l := ledger.At(o.Root)
	rows := l.Log()
	cos := l.ChiefOfStaff()
	gone := r.gone(o.State)
	tasks := make([]ledger.Issue, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, byID[id].raw)
	}
	for _, id := range ids {
		t := byID[id]
		if d, ok := doneAt[id]; !ok || !in(d) || t.reviewer || t.profile == "chief-engineer" || t.raw.Str("who") == "person" {
			continue
		}
		worked := ledger.Worked(rows, tasks, id, cos, gone[id], o.Now)
		x := Issue{Issue: id, WorkedMinutes: int(worked.Minutes())}
		if m := ledger.EstimateMinutes(t.agentEst); m > 0 {
			x.AgentEstimateMinutes = &m
		}
		if m := ledger.EstimateMinutes(l.Estimate(rows, t.raw)); m > 0 {
			x.EstimateMinutes = &m
			rt := round(worked.Minutes() / float64(m))
			x.Ratio = &rt
			sum.Count++
			if worked <= time.Duration(m)*time.Minute {
				sum.Within++
			}
		} else {
			sum.None++
		}
		out = append(out, x)
	}
	sum.Rate = ratio(sum.Within, sum.Count)
	return sum, out
}

// gone は dispatcher の控えの gone（課題 → 担当の端末が Gone だと見た時刻）。無ければ空。
func (r *reader) gone(path string) map[string][]time.Time {
	out := map[string][]time.Time{}
	if path == "" {
		return out
	}
	b, ok := r.read(path)
	if !ok {
		return out
	}
	var v struct {
		Gone map[string][]struct {
			At int64 `json:"at"`
		} `json:"gone"`
	}
	if json.Unmarshal(b, &v) != nil {
		r.skip(r.rel(path), 0, "JSON の object でない。端末が死んでいた区間を実働から引いていない")
		return out
	}
	for id, gs := range v.Gone {
		for _, g := range gs {
			out[ledger.CanonID(id)] = append(out[ledger.CanonID(id)], time.UnixMilli(g.At))
		}
	}
	return out
}

// estimateTable は見込みの節（期間に閉じた担当の課題ごと。見込みの無い課題は「見込み無し」で、0 と分ける）。
func estimateTable(w io.Writer, s Estimate, xs []Issue) {
	fmt.Fprintln(w, "\n見込み（期間に閉じた担当の課題。実働は待ちを除く。分）")
	fmt.Fprintf(w, "  見込みの中に収まった割合       %s・見込み無し %d件\n", pct(s.Rate, s.Within, s.Count), s.None)
	for _, x := range xs {
		est, agent, ratio := "見込み無し", "", ""
		if x.EstimateMinutes != nil {
			est = fmt.Sprintf("見込み %d", *x.EstimateMinutes)
			ratio = fmt.Sprintf("・比 %.2f", *x.Ratio)
		}
		if x.AgentEstimateMinutes != nil {
			agent = fmt.Sprintf("（担当の見込み %d）", *x.AgentEstimateMinutes)
		}
		fmt.Fprintf(w, "  %-12s %s%s・実働 %d%s\n", x.Issue, est, agent, x.WorkedMinutes, ratio)
	}
}
