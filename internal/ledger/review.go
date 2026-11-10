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
	Issue       string `json:"issue,omitempty"`
	Commit      string `json:"commit,omitempty"`
	MustFixOpen int    `json:"must_fix_open"`
	Waived      int    `json:"waived"`
	WaivedHeavy int    `json:"-"` // must_fix 以上なのに waived にある指摘（担当がレビュアーを覆した。人が判断する）
	Reason      string `json:"-"` // missing・stale の理由（人が読む1行）
}

// CheckReview は、課題 t の最新のレビュアーの findings.json が head を指すかを確かめ、
// must_fix 以上で waived に無い指摘を数える。must_fix が空なら数えない（読み手が安全側に倒す）。
// 数えない指摘が2つある（decisions/2026-11-09-review-preexisting.md・2026-11-09-review-later-rounds.md）:
//   - preexisting: true の指摘。ただし added（PR の差分で足した行。file → 行）に当たる場所なら数える
//     （課題で足した行が課題の前からあったはずはない。申告でゲートを緩めない）。added が nil なら場所を確かめない（担当の command の下見）。
//   - 2回目以降（前の回のレビューがある）の critical でない指摘で、前の回の findings.json に同じ file の指摘が無いもの（新しい指摘）。
func (l *Ledger) CheckReview(rows []Row, tasks []Issue, t Issue, head, mustFix string, waived []contract.Waived, added map[string]map[int]bool) Review {
	r := Review{State: "missing", Waived: len(waived)}
	picked, others := l.reviewers(rows, tasks, t)
	if len(picked) == 0 {
		r.Reason = "レビューが無い（Chief of Staff にレビューを頼む）"
		if len(others) > 0 {
			r.Reason = fmt.Sprintf("子課題 %s はレビューに数えない（作成が Chief of Staff・dispatcher でない、または profile が実装と別ブランドの *-review でない）。Chief of Staff に頼み直す", strings.Join(others, ","))
		}
		return r
	}
	reviewer := picked[len(picked)-1]
	f, err := contract.ReadFindings(filepath.Join(l.Library(reviewer), "findings.json"))
	if err != nil {
		r.Reason = fmt.Sprintf("レビュー %s の library/findings.json が無い・形が違う（%v）", reviewer, err)
		return r
	}
	r.Issue, r.Commit = reviewer, f.Commit
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
	prev := l.previousFiles(picked[:len(picked)-1], f.Commit)
	for _, x := range f.Findings {
		if SeverityRank(x.Severity) > limit {
			continue
		}
		if x.Preexisting && (added == nil || x.Line == nil || !added[x.File][*x.Line]) {
			continue
		}
		if prev != nil && x.Severity != "critical" && !prev[x.File] {
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

// previousFiles は前の回のレビュー（earlier を新しい順に見て、findings.json が読めて commit が今の回と違う最初のもの）の
// 指摘の file の集まり。前の回が無ければ nil（1回目）。今の回と同じ commit を見た頼み直しは同じ回なので飛ばす（andon issue review-request の数え方と同じ）。
func (l *Ledger) previousFiles(earlier []string, commit string) map[string]bool {
	for i := len(earlier) - 1; i >= 0; i-- {
		f, err := contract.ReadFindings(filepath.Join(l.Library(earlier[i]), "findings.json"))
		if err != nil || f.Commit == commit {
			continue
		}
		files := map[string]bool{}
		for _, x := range f.Findings {
			files[x.File] = true
		}
		return files
	}
	return nil
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
