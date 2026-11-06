package ledger

import (
	"fmt"
	"path/filepath"
	"strings"

	"andon/internal/contract"
)

// Review は独立レビューの照合の結果（ゲートの記録の review と同じ形。contract.md §7）。
type Review struct {
	State       string `json:"state"` // current / stale / missing / off
	Task        string `json:"task,omitempty"`
	Commit      string `json:"commit,omitempty"`
	MustFixOpen int    `json:"must_fix_open"`
	Waived      int    `json:"waived"`
	WaivedHeavy int    `json:"-"` // must_fix 以上なのに waived にある指摘（担当がレビュアーを覆した。人が判断する）
	Reason      string `json:"-"` // missing・stale の理由（人が読む1行）
}

// CheckReview は、タスク t の最新のレビュアーの findings.json が head を指すかを確かめ、
// must_fix 以上で waived に無い指摘を数える。must_fix が空なら数えない（読み手が安全側に倒す）。
func (l *Ledger) CheckReview(rows []Row, tasks []Task, t Task, head, mustFix string, waived []contract.Waived) Review {
	r := Review{State: "missing", Waived: len(waived)}
	reviewer, others := l.Reviewer(rows, tasks, t)
	if reviewer == "" {
		r.Reason = "レビューが無い（Chief of Staff にレビューを頼む）"
		if len(others) > 0 {
			r.Reason = fmt.Sprintf("子タスク %s はレビューに数えない（作成が coordinator・dispatcher でない、または profile が実装と別ブランドの *-review でない）。Chief of Staff に頼み直す", strings.Join(others, ","))
		}
		return r
	}
	f, err := contract.ReadFindings(filepath.Join(l.Library(reviewer), "findings.json"))
	if err != nil {
		r.Reason = fmt.Sprintf("レビュー %s の library/findings.json が無い・形が違う（%v）", reviewer, err)
		return r
	}
	r.Task, r.Commit = reviewer, f.Commit
	if f.Commit != head {
		r.State = "stale"
		r.Reason = fmt.Sprintf("レビュー %s は commit %s を見た（head は %s）。頼み直す", reviewer, short(f.Commit), short(head))
		return r
	}
	r.State = "current"
	limit := SeverityRank(mustFix)
	if limit < 0 {
		return r
	}
	for _, x := range f.Findings {
		if SeverityRank(x.Severity) > limit {
			continue
		}
		if isWaived(x, waived) {
			r.WaivedHeavy++
		} else {
			r.MustFixOpen++
		}
	}
	return r
}

func isWaived(x contract.Finding, waived []contract.Waived) bool {
	for _, w := range waived {
		if w.Severity == x.Severity && w.File == x.File && w.Line != nil && x.Line != nil && *w.Line == *x.Line {
			return true
		}
	}
	return false
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
