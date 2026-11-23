package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"andon/internal/contract"
)

// askedRe は kanban ask の返事（<id>#<n>）の番号。
var askedRe = regexp.MustCompile(`#([1-9][0-9]*)$`)

// addedRe は kanban add の返事（足した id）。
var addedRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Ask は確かめた問い（contract.ParseQuestion）を課題 id に kanban ask で問い、library/questions/<n>.json に置く。
// kanban の CLI の返事（<id>#<n>）を返す。andon issue ask・gate の問い・andon issue ask --new が通る道。
// トリアージと振り返りの apply も、課題を足したあとに問いだけを置くときはこれを呼ぶ（ADR-0057 決めたこと 7・9・11）。
func (l *Ledger) Ask(id string, q *contract.Question) (string, error) {
	out, err := l.Run(q.Text(), "ask", id, "-", "--kind", q.Kind)
	if err != nil {
		return "", err
	}
	m := askedRe.FindStringSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("kanban の CLI の返事から問いの番号が読めない: %q", out)
	}
	dir := filepath.Join(l.Library(id), "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := contract.Write(filepath.Join(dir, m[1]+".json"), contract.SchemaQuestion, q); err != nil {
		return "", err
	}
	return out, nil
}

// ownerTitle は andon issue ask --new が問いの文から作るオーナーの課題のタイトル。
func ownerTitle(q *contract.Question) string { return q.Question + "（オーナーの判断待ち）" }

// ownerBrief は andon issue ask --new が問いの JSON から書く依頼書（人が読む写し。問いの正は library/questions/）。
func ownerBrief(q *contract.Question) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", ownerTitle(q))
	b.WriteString("オーナーの判断を待つ課題。andon issue ask --new が問いの JSON から書いた写しで、問いの正は library/questions/ にある。\n\n")
	if len(q.Facts) > 0 {
		fmt.Fprintf(&b, "出どころ: %s\n", strings.Join(q.Facts, " / "))
	}
	fmt.Fprintf(&b, "kind: %s\n", q.Kind)
	fmt.Fprintf(&b, "推奨案: %s\n", q.Options[0].Label)
	var others []string
	for _, o := range q.Options[1:] {
		others = append(others, o.Label)
	}
	fmt.Fprintf(&b, "ほかの案: %s\n", strings.Join(others, " / "))
	if q.Reason != "" {
		fmt.Fprintf(&b, "理由: %s\n", q.Reason)
	}
	return b.String()
}

// NewOwnerQuestion はオーナーの課題を足し、who: person を書き、問いを置く（andon issue ask --new。ADR-0057 決めたこと 7）。
// トリアージと振り返りの apply も同じ関数を呼べるよう、andon issue の外（この package）に置く（internal/issue は
// dispatcher を通して triage・retro を import するので、そちらに置くと循環する）。
// 問いは確かめたもの（contract.ParseQuestion）を渡す。after があれば課題は queued で作られ、その課題にも問いを置く。
// 足した id と kanban の CLI の返事（<id>#<n>）を返す。課題を足したあとで落ちたら、足した id と打ち直す command を誤りに書く
// （who: person を書いたあとなら、残った課題は問いも印も無いので、andon waiting が判断に数えて灯す）。
func (l *Ledger) NewOwnerQuestion(q *contract.Question, parent string, after []string) (string, string, error) {
	args := []string{"add", "--brief", "-"}
	if parent != "" {
		args = append(args, "--parent", parent)
	}
	if len(after) > 0 {
		args = append(args, "--after", strings.Join(after, ","))
	}
	id, err := l.Run(ownerBrief(q), append(args, "--", ownerTitle(q))...)
	if err != nil {
		return "", "", err
	}
	if !addedRe.MatchString(id) {
		return "", "", fmt.Errorf("kanban の CLI の返事から課題の id が読めない: %q", id)
	}
	if _, err := l.Run("", "set", id, "who=person"); err != nil {
		return id, "", fmt.Errorf("課題 %s を足したが who: person を書けなかった（%v）。kanban set %s who=person のあと andon issue ask %s - で問いを置き直す", id, err, id, id)
	}
	out, err := l.Ask(id, q)
	if err != nil {
		return id, "", fmt.Errorf("オーナーの課題 %s を足したが問いを置けなかった（%v）。andon issue ask %s - で問いを置き直す", id, err, id)
	}
	return id, out, nil
}
