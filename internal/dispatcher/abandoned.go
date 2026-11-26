package dispatcher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// 待つ相手がやめになった（dispatcher.md §2.2）: 引き取れるようになった課題の待つ相手（after）のうち、PR を成果にする課題が
// マージされずに done になったものを ready の知らせに添える。kanban done は、待つ相手がマージで片付いたか・やめになったかを
// 見分けずに queued の課題を todo へ移す（移す動きは変えない）。Chief of Staff が前の課題の成果を前提に起動しないための事実。

const (
	mergedYes    = "yes"
	mergedNo     = "no"
	mergedUnsure = ""
)

func (p *project) waitedGone(t ledger.Issue) (gone, unsure []string) {
	cos := ""
	for _, id := range afterOf(t) {
		a, err := p.issue(id)
		if err != nil || !prRole(a) {
			continue
		}
		if a.Str("deliverable") == "report" {
			if p.rowsErr != nil {
				// log.jsonl を読み切れない巡では、report を書いたのが Chief of Staff か確かめられない。やめと言い切らない
				unsure = append(unsure, id)
				continue
			}
			if cos == "" {
				cos = p.l.ChiefOfStaff()
			}
		}
		if ledger.DeliverableOf(p.rows, a, cos) != "pr" {
			continue
		}
		switch p.mergedState(id) {
		case mergedNo:
			gone = append(gone, id)
		case mergedUnsure:
			unsure = append(unsure, id)
		}
	}
	return gone, unsure
}

func afterOf(t ledger.Issue) []string {
	var out []string
	switch v := t["after"].(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, ledger.CanonID(s))
			}
		}
	case []string:
		for _, s := range v {
			if s != "" {
				out = append(out, ledger.CanonID(s))
			}
		}
	case string:
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, ledger.CanonID(s))
			}
		}
	}
	return out
}

// prRole は、待つ相手 t が PR を成果にしうる課題か（repo があり、レビュアー・オーナーの課題でない）。そのうえ成果の形
// （ledger.DeliverableOf。contract.md §2 の「成果の形の読み方」）が pr の課題だけを見分ける。調べる課題と、書いた時点の
// Chief of Staff が report を書いた報告の課題は、done が成果の出来上がりなので見分けない（ADR-0066 決めたこと 6・10）。
func prRole(t ledger.Issue) bool {
	return t.Str("repo") != "" && !isReviewer(t) && t.Str("who") != "person"
}

// mergedState は課題 id の PR がマージされたかを、手元の記録から先に、無ければ gh で決める。
//  1. 結果の記録（outcome.json。contract.md §19）があれば、その result（merged ならマージ、ほかはやめ）
//  2. gate の記録に merged が true のものがあれば、dispatcher がマージした
//  3. report.json に pr が無ければ、PR を出さずに終えた（やめ）
//  4. gh pr view の state（MERGED ならマージ、CLOSED・OPEN はやめ）。gh の上限で止めている間と、読めないときは確かめられない
func (p *project) mergedState(id string) string {
	lib := p.l.Library(id)
	if b, err := os.ReadFile(filepath.Join(lib, outcomeFile)); err == nil {
		var o Outcome
		if json.Unmarshal(b, &o) == nil && o.Result != "" {
			if o.Result == ResultMerged {
				return mergedYes
			}
			return mergedNo
		}
	}
	if gs, err := gates(filepath.Join(lib, "gate")); err == nil {
		for _, g := range gs {
			if g.Merged {
				return mergedYes
			}
		}
	}
	rep, err := contract.ReadReport(filepath.Join(lib, "report.json"))
	if errors.Is(err, os.ErrNotExist) || (err == nil && rep.PR == "") {
		return mergedNo
	}
	if err != nil {
		p.logf("%s の report.json が読めない（待つ相手がマージされたか確かめられない）: %v", id, err)
		return mergedUnsure
	}
	if until, err := time.Parse(time.RFC3339, p.st.GHReset); err == nil && time.Now().Before(until) {
		return mergedUnsure
	}
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok {
		p.logf("%s の report.json の pr が読めない（待つ相手がマージされたか確かめられない）: %s", id, rep.PR)
		return mergedUnsure
	}
	out, _, err := run("", nil, "", "gh", "pr", "view", number, "--repo", owner+"/"+name, "--json", "state,mergeCommit,mergedAt")
	var v prState
	if err == nil {
		err = json.Unmarshal([]byte(out), &v)
	}
	if err != nil {
		p.logf("%s の PR の状態が読めない（待つ相手がマージされたか確かめられない）: %v", id, err)
		return mergedUnsure
	}
	switch Result(true, v.State) {
	case ResultMerged:
		return mergedYes
	case ResultClosed, ResultOpen:
		return mergedNo
	}
	return mergedUnsure
}

func idList(v any) []string {
	var out []string
	switch x := v.(type) {
	case []string:
		for _, s := range x {
			out = append(out, clean(s, 40))
		}
	case []any:
		for _, s := range x {
			out = append(out, clean(s, 40))
		}
	}
	return out
}
