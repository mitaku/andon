package waiting

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// stackView は積めるかを決めるのに読むもの（count と StackOf が同じに組む）。
type stackView struct {
	l        *ledger.Ledger
	byID     map[string]*issue
	raws     []ledger.Issue
	rows     []ledger.Row
	cos      string
	reviewAt map[string]string // 課題が最後に review に入った行の時刻
	done     string
	today    string // config.yaml の timezone（無ければ UTC）の今日
}

// Onto は積める課題の前の課題と、その PR と、gate が通した head（前の課題の報告の commit）。
type Onto struct {
	On   string
	PR   string
	Head string
}

// of は課題 t が積めるか（ADR-0067 決めたこと 4）。積めれば前の課題の id と PR と head を返し、理由は空。積めなければ why に
// 理由の 1 行（andon issue start が断る文）。記録の上での見込みで、GitHub は読まず git も呼ばない（PR が open か、PR の今の head が
// 報告の commit と同じかは andon issue start が確かめる）。
func (v stackView) of(t *issue) (Onto, string) {
	switch {
	case t.status != "queued":
		return Onto{}, fmt.Sprintf("%s は %s（積めるのは queued の課題だけ）", t.id, t.status)
	case !t.worker():
		return Onto{}, fmt.Sprintf("%s は担当の課題でない（積めるのは担当の課題だけ）", t.id)
	case t.raw.Str("repo") == "":
		return Onto{}, fmt.Sprintf("%s に repo が無い（積めるのは repo のある課題だけ）", t.id)
	}
	if _, held := ledger.HoldOf(v.rows, v.raws, v.cos, t.raw); held {
		return Onto{}, fmt.Sprintf("%s は止められている（hold）", t.id)
	}
	if day, err := time.Parse("2006-01-02", t.notBefore); err == nil && v.today < day.Format("2006-01-02") {
		return Onto{}, fmt.Sprintf("%s の not_before（%s）がまだ来ていない", t.id, t.notBefore)
	}
	var left []string
	for _, a := range t.after {
		if o := v.byID[a]; o == nil || o.status != v.done {
			left = append(left, a)
		}
	}
	if len(left) != 1 {
		return Onto{}, fmt.Sprintf("%s の待つ相手が %d つ残る（%s。積めるのは 1 つだけ残る課題）", t.id, len(left), strings.Join(left, ", "))
	}
	p := v.byID[left[0]]
	switch {
	case p == nil:
		return Onto{}, fmt.Sprintf("前の課題 %s が kanban に無い", left[0])
	case !p.worker():
		return Onto{}, fmt.Sprintf("前の課題 %s は担当の課題でない", p.id)
	case p.raw.Str("repo") != t.raw.Str("repo"):
		return Onto{}, fmt.Sprintf("前の課題 %s の repo（%s）が %s の repo（%s）と違う", p.id, p.raw.Str("repo"), t.id, t.raw.Str("repo"))
	case p.status != "review":
		return Onto{}, fmt.Sprintf("前の課題 %s が review でない（%s）", p.id, p.status)
	}
	if _, held := ledger.HoldOf(v.rows, v.raws, v.cos, p.raw); held {
		return Onto{}, fmt.Sprintf("前の課題 %s が止められている（hold）", p.id)
	}
	lib := v.l.Library(p.id)
	rep, _ := contract.ReadReport(filepath.Join(lib, "report.json"))
	if rep == nil || rep.PR == "" || !contract.IsSHA(rep.Commit) {
		return Onto{}, fmt.Sprintf("前の課題 %s の報告（report.json）に PR か commit が無い", p.id)
	}
	rec := gateRecord(lib, rep.Commit)
	if rec == nil || v.reviewAt[p.id] == "" || rec.At < v.reviewAt[p.id] {
		return Onto{}, fmt.Sprintf("前の課題 %s の今の head（%s）に、今の review の回の gate の記録が無い", p.id, rep.Commit[:12])
	}
	switch {
	case rec.Merged:
		return Onto{}, fmt.Sprintf("前の課題 %s の PR は dispatcher がもうマージした", p.id)
	case rec.Decision != "approve" && rec.Decision != "needs_human" && rec.Decision != "behind":
		return Onto{}, fmt.Sprintf("前の課題 %s の今の head の gate の判定が %s（積めるのは approve・needs_human・behind のとき）", p.id, rec.Decision)
	}
	return Onto{On: p.id, PR: rep.PR, Head: rep.Commit}, ""
}

// StackOf は kanban の根 root の課題 id が積めるかを、andon waiting list の stackable と同じ決まりで決める（ADR-0067 決めたこと 4・5）。
// andon issue start が、queued の課題を起動する前に打つ。積めなければ why に理由の 1 行。err は kanban が読めないとき。
func StackOf(root, id string, now time.Time) (onto Onto, why string, err error) {
	k, _, err := load(root)
	if err != nil {
		return Onto{}, "", err
	}
	sv := newStackView(k, now)
	t := sv.byID[ledger.CanonID(id)]
	if t == nil {
		return Onto{}, "", fmt.Errorf("%s が kanban に無い", id)
	}
	onto, why = sv.of(t)
	return onto, why, nil
}

func newStackView(k kanban, now time.Time) stackView {
	done := "done"
	if len(k.cfg.Columns) > 0 {
		done = k.cfg.Columns[len(k.cfg.Columns)-1]
	}
	byID := map[string]*issue{}
	raws := make([]ledger.Issue, len(k.issues))
	for i, t := range k.issues {
		byID[t.id] = t
		raws[i] = t.raw
	}
	reviewAt := map[string]string{}
	for _, r := range k.rows {
		if r.Change != nil && r.Change["status"] == "review" {
			reviewAt[r.ID] = r.At
		}
	}
	return stackView{l: k.l, byID: byID, raws: raws, rows: k.rows, cos: k.cfg.ChiefOfStaff, reviewAt: reviewAt, done: done,
		today: now.In(location(k.cfg)).Format("2006-01-02")}
}

func location(cfg ledger.Config) *time.Location {
	if cfg.Timezone != "" {
		if z, err := time.LoadLocation(cfg.Timezone); err == nil {
			return z
		}
	}
	return time.UTC
}
