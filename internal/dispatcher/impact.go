package dispatcher

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"andon/internal/contract"
)

const impactTries = 3

// impacts は done になった課題（控えの Impact）ごとに andon issue impact を1回だけ打ち、影響のある課題があれば
// 通知 impact を積む（dispatcher.md §2.5）。判断（影響なし・rebase を頼む・止めて向きを確かめる）は Chief of Staff。
// PR の無い課題・MERGED でない PR（やめた課題）は打つ前か終了コード 4 で飛ばす。
func (p *project) impacts() {
	p.st.Impact.Each(p.impact)
}

func (p *project) impact(id string) bool {
	for _, t := range p.tasks {
		if t.Str("id") == id && (isReviewer(t) || isEngineer(t) || t.Str("repo") == "") {
			return true
		}
	}
	rep, err := contract.ReadReport(filepath.Join(p.l.Library(id), "report.json"))
	if err != nil || rep.PR == "" {
		return true
	}
	gaveUp := func(why string) bool {
		p.logf("%s の影響を集められない（諦めて Chief of Staff に知らせる。andon issue impact %s で読める）: %s", id, id, why)
		p.notify(id, "impact", map[string]any{"pr": rep.PR, "error": clean(why, 200)})
		return true
	}
	out, code, err := p.andon("", "issue", "impact", id)
	switch {
	case code == 4:
		p.logf("%s の PR はマージされていないので影響を見ない（マージせずにやめた課題）", id)
		return true
	case err != nil:
		if p.st.Impact.Fail(id, impactTries) {
			return gaveUp(fmt.Sprintf("%d 回打って失敗: %v", impactTries, err))
		}
		p.logf("%s の影響を集められない（次の見回りで打ち直す）: %v", id, err)
		return false
	}
	var res contract.Impact
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return gaveUp("andon issue impact の出力が読めない: " + err.Error())
	}
	detail := ImpactDetail(res)
	if detail == nil {
		p.logf("%s のマージ（%s）は動いている課題 %d 件のどれとも重ならない", id, short12(res.Sha), len(res.Issues))
		return true
	}
	p.notify(id, "impact", detail)
	return true
}

func ImpactDetail(res contract.Impact) map[string]any {
	var rows []any
	for _, t := range res.Issues {
		if !t.Affected(res.Issue) {
			continue
		}
		row := map[string]any{"issue": t.Issue, "conflict": t.Conflict, "overlap": cap20(t.Overlap), "brief_refs": cap20(t.BriefRefs),
			"checks_moved": cap20(t.ChecksMoved)}
		if t.Error != "" {
			row["error"] = t.Error
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	var out map[string]any
	b, _ := json.Marshal(map[string]any{"pr": res.PR, "sha": res.Sha, "issues": rows})
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

func cap20(s []string) []string {
	if s == nil {
		return []string{}
	}
	if len(s) > 20 {
		return s[:20]
	}
	return s
}
