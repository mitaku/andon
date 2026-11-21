package ledger

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"andon/internal/contract"
)

// Review は独立レビューの照合の結果（gate の記録の review と同じ形。contract.md §7）。
type Review struct {
	State       string `json:"state"` // current / stale / missing / off
	Issue       string `json:"issue,omitempty"`
	Commit      string `json:"commit,omitempty"`
	MustFixOpen int    `json:"must_fix_open"`
	Waived      int    `json:"waived"`
	WaivedHeavy int    `json:"-"` // must_fix 以上なのに waived にある指摘（担当がレビュアーを覆した。オーナーが判断する）
	Reason      string `json:"-"` // missing・stale の理由（人が読む1行）
}

// CheckReview は、課題 t の最新のレビュアーの findings.json が head を指すかを確かめ、
// must_fix 以上で waived に無い指摘を数える。must_fix が空なら数えない（読み手が安全側に倒す）。
// findings.json の commit が head と違っても、PR の差分が同じ（SamePatch。rebase で head が変わっても、PR が足した行と消した行が同じ）なら
// そのレビューを head に引き継いで current にする（decisions/2026-11-20-review-carry-same-patch.md・2026-11-21-review-carry-ignores-context-lines.md）。Commit は見た commit のまま。
// 数えない指摘が2つある（decisions/2026-11-09-review-preexisting.md・2026-11-09-review-later-rounds.md）:
//   - preexisting: true の指摘。ただし added（PR の差分で足した行。file → 行）に当たる場所なら数える
//     （課題で足した行が課題の前からあったはずはない。申告で gate を緩めない）。added が nil なら場所を確かめない（担当の command の下見）。
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
	if f.Commit != head && !SamePatch(t.Path("worktree"), f.Commit, head) {
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

// SamePatch は、課題の worktree dir の git で、commit a と b の PR の差分が同じか。
// PR の差分は、その commit と origin/HEAD（既定の branch）の merge-base から commit までの diff とする。
// 比べるのは、ファイルの見出しと、足した行・消した行だけで、空白も含めて比べる（prDiff）。hunk の見出し・index の行・
// 前後の文脈の行は比べない。文脈の行は main の側の文で、main が近くの行を変えた rebase や、衝突を解いた rebase でも、
// PR が足した行と消した行が同じなら同じ差分とみなす（decisions/2026-11-21-review-carry-ignores-context-lines.md）。
// git patch-id は空白の違いを無視するので使わない（文字列の中の空白だけが変わった差分を同じと見て、見ていない変更のレビューを引き継ぐ）。
// worktree が無い・commit が無い・origin/HEAD が引けない・差分が空のときは同じと言わない（引き継がずに頼み直す側に倒す）。
func SamePatch(dir, a, b string) bool {
	if dir == "" || a == "" || b == "" {
		return false
	}
	pa, ok := prDiff(dir, a)
	if !ok {
		return false
	}
	pb, ok := prDiff(dir, b)
	return ok && bytes.Equal(pa, pb)
}

// prDiff は commit の PR の差分から、index の行と、hunk の中の見出し（@@ の行）・文脈の行（先頭が空白）を除いたもの。
// 文脈の行に続く「\ No newline at end of file」も文脈の側なので除く。hunk の見出しを除くので、文脈が変わって hunk が
// 割れたり繋がったりしても同じになる。差分はバイナリの中身も含める（--binary。含めないと中身の違うバイナリが同じに見える）。
// rename は git の設定に依らず見ない（--no-renames）。
func prDiff(dir, commit string) ([]byte, bool) {
	git := func(args ...string) ([]byte, bool) {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		return out, err == nil
	}
	base, ok := git("merge-base", commit+"^{commit}", "refs/remotes/origin/HEAD")
	if !ok {
		return nil, false
	}
	diff, ok := git("diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--binary", "--full-index", strings.TrimSpace(string(base)), commit)
	if !ok || len(bytes.TrimSpace(diff)) == 0 {
		return nil, false
	}
	var out []byte
	inHunk, dropped := false, false
	for _, ln := range bytes.SplitAfter(diff, []byte("\n")) {
		switch {
		case bytes.HasPrefix(ln, []byte("diff --git ")):
			inHunk = false
		case bytes.HasPrefix(ln, []byte("index ")):
			continue
		case bytes.HasPrefix(ln, []byte("@@ ")):
			inHunk, dropped = true, true
			continue
		case inHunk && bytes.HasPrefix(ln, []byte(" ")):
			dropped = true
			continue
		case inHunk && bytes.HasPrefix(ln, []byte("\\")):
			if dropped {
				continue
			}
		}
		dropped = false
		out = append(out, ln...)
	}
	return out, true
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

// BriefReviews は、レビュアーの依頼書 brief（dispatcher の reviewerBrief が書く）が commit を見るよう頼んでいるか。
// 依頼書は「commit <sha>。」の形で commit を書く。dispatcher の Adopt と andon issue review-request が同じ読み方をする。
func BriefReviews(brief, commit string) bool {
	return commit != "" && strings.Contains(brief, "commit "+commit+"。")
}
