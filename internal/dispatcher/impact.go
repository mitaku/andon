package dispatcher

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"andon/internal/contract"
)

// impactTries は andon task impact が失敗したときに打ち直す回数の上限（fetch などの一時の失敗を見回りで打ち直す）。
const impactTries = 3

// impacts は done になったタスク（控えの Impact）ごとに andon task impact を1回だけ打ち、影響のあるタスクがあれば
// 通知 impact を積む（dispatcher.md §2.5）。判断（影響なし・rebase を頼む・止めて向きを確かめる）は Chief of Staff。
// PR の無いタスク・MERGED でない PR（やめたタスク）は打つ前か終了コード 4 で飛ばす。
func (p *project) impacts() {
	if len(p.st.Impact) == 0 {
		return
	}
	ids := make([]string, 0, len(p.st.Impact))
	for id := range p.st.Impact {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if p.impact(id) {
			delete(p.st.Impact, id)
		}
	}
	if len(p.st.Impact) == 0 {
		p.st.Impact = nil
	}
}

// impact は1つのタスクについて打つ。true は済んだ（控えから外す）とき。
func (p *project) impact(id string) bool {
	// 影響を見るのは、PR を出した案件 repo の担当のタスクだけ（レビュアー・Chief Engineer の文書・repo の無いタスクは飛ばす。
	// 写し〈巡の 3〉に無いのは、この巡で足されて done になったものだけで、それも report.json で見分ける）
	for _, t := range p.tasks {
		if t.Str("id") == id && (isReviewer(t) || t.Str("profile") == "chief-engineer" || t.Str("repo") == "") {
			return true
		}
	}
	rep, err := contract.ReadReport(filepath.Join(p.l.Library(id), "report.json"))
	if err != nil || rep.PR == "" {
		return true
	}
	// 集められなかったときも Chief of Staff に知らせる（待ちの通知は届くまで控えに残るので、手で確かめるきっかけは失われない）
	gaveUp := func(why string) bool {
		p.logf("%s の影響を集められない（諦めて Chief of Staff に知らせる。andon task impact %s で読める）: %s", id, id, why)
		p.notify(id, "impact", map[string]any{"pr": rep.PR, "error": clean(why, 200)})
		return true
	}
	out, code, err := p.andon("", "task", "impact", id)
	switch {
	case code == 4:
		p.logf("%s の PR はマージされていないので影響を見ない（マージせずにやめたタスク）", id)
		return true
	case err != nil:
		p.st.Impact[id]++
		if p.st.Impact[id] >= impactTries {
			return gaveUp(fmt.Sprintf("%d 回打って失敗: %v", impactTries, err))
		}
		p.logf("%s の影響を集められない（次の見回りで打ち直す）: %v", id, err)
		return false
	}
	var res contract.Impact
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return gaveUp("andon task impact の出力が読めない: " + err.Error())
	}
	detail := ImpactDetail(res)
	if detail == nil {
		p.logf("%s のマージ（%s）は動いているタスク %d 件のどれとも重ならない", id, short12(res.Sha), len(res.Tasks))
		return true
	}
	p.notify(id, "impact", detail)
	return true
}

// ImpactDetail は andon task impact の出力から通知 impact の detail を作る（純粋関数）。影響のあるタスクが無ければ nil。
// 並べるファイルは1つのタスクにつき 20 まで（全部は andon task impact で読む）。
func ImpactDetail(res contract.Impact) map[string]any {
	var rows []any
	for _, t := range res.Tasks {
		if !t.Affected() {
			continue
		}
		row := map[string]any{"task": t.Task, "conflict": t.Conflict, "overlap": cap20(t.Overlap), "brief_refs": cap20(t.BriefRefs)}
		if t.Error != "" {
			row["error"] = t.Error
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	// 控えから読み戻したときと同じ形（[]any・map[string]any）にそろえる（<news> の描き方が1つで済む）
	var out map[string]any
	b, _ := json.Marshal(map[string]any{"pr": res.PR, "sha": res.Sha, "tasks": rows})
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
