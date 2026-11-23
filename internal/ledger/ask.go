package ledger

import (
	"encoding/json"
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
// タイトルは問いの文に「（オーナーの判断待ち）」を付けたもの、依頼書は問いの JSON から書く写し。
// トリアージと振り返りの apply も同じ関数を呼べるよう、andon issue の外（この package）に置く（internal/issue は
// dispatcher を通して triage・retro を import するので、そちらに置くと循環する）。
// 足した id と kanban の CLI の返事（<id>#<n>）を返す。
func (l *Ledger) NewOwnerQuestion(q *contract.Question, parent string, after []string) (string, string, error) {
	return l.NewOwnerIssue(q, ownerTitle(q), ownerBrief(q), parent, after)
}

// NewOwnerIssue は NewOwnerQuestion の、タイトルと依頼書を呼ぶ側が決める口（振り返りの apply は今のタイトルと依頼書のまま
// 足す。ADR-0057 決めたこと 11）。
// 問いは確かめたもの（contract.ParseQuestion）を渡す。scope の問い（Chief of Staff が答える）は置かない。
// after があれば課題は queued で作られ、その課題にも問いを置く。
// 同じタイトルで同じ parent の片付いていない課題が、問いを持たずにあれば、足し直さずにその課題へ続きを置く（足したあと who を
// 書く前や問いを置く前に落ちた回の打ち直し）。問いを持っていれば、同じ問いを 2 つ足さないよう断る。
// 足した id と kanban の CLI の返事（<id>#<n>）を返す。課題を足したあとで落ちたら、足した id と打ち直す command を誤りに書く。
func (l *Ledger) NewOwnerIssue(q *contract.Question, title, brief, parent string, after []string) (string, string, error) {
	if q.Kind == "scope" {
		return "", "", fmt.Errorf("scope の問いはオーナーの課題に置かない（Chief of Staff が答える問い。kind は business/users/promise/irreversible/line）")
	}
	id, err := l.unfinished(title, parent)
	if err != nil {
		return "", "", err
	}
	if id == "" {
		args := []string{"add", "--brief", "-"}
		if parent != "" {
			args = append(args, "--parent", parent)
		}
		if len(after) > 0 {
			args = append(args, "--after", strings.Join(after, ","))
		}
		if id, err = l.Run(brief, append(args, "--", title)...); err != nil {
			return "", "", err
		}
		if !addedRe.MatchString(id) {
			return "", "", fmt.Errorf("kanban の CLI の返事から課題の id が読めない: %q", id)
		}
	}
	if _, err := l.Run("", "set", id, "who=person"); err != nil {
		return id, "", fmt.Errorf("課題 %s を足したが who: person を書けなかった（%v）。同じ command を打ち直すと続きから置く", id, err)
	}
	out, err := l.Ask(id, q)
	if err != nil {
		return id, "", fmt.Errorf("オーナーの課題 %s を足したが問いを置けなかった（%v）。同じ command を打ち直すと続きから置く", id, err)
	}
	return id, out, nil
}

// unfinished は、タイトルが title で parent が同じ（どちらも無いも同じに数える）片付いていない（done でない）課題のうち、
// 問いを持たないものの id を返す。無ければ空。問いを持つものがあれば、同じ問いのオーナーの課題がもうあるとして誤りを返す。
// parent を見るのは、別の振り返りが足した同じタイトルの課題を、この振り返りの課題と取り違えないため。
func (l *Ledger) unfinished(title, parent string) (string, error) {
	tasks, err := l.List()
	if err != nil {
		return "", err
	}
	for _, t := range tasks {
		if t.Str("title") != title || t.Str("status") == "done" || CanonID(t.Str("parent")) != CanonID(parent) {
			continue
		}
		id := t.Str("id")
		n, err := l.Questions(id)
		if err != nil {
			return "", err
		}
		if n > 0 {
			return "", fmt.Errorf("同じ問いのオーナーの課題 %s がもうある（問いを足すなら andon issue ask %s -）", id, id)
		}
		return id, nil
	}
	return "", nil
}

// Questions は課題 id に置かれた問いの数（状態を問わない）。
func (l *Ledger) Questions(id string) (int, error) {
	out, err := l.Run("", "show", id, "--json")
	if err != nil {
		return 0, err
	}
	var v struct {
		Questions []json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return 0, fmt.Errorf("課題 %s が読めない", id)
	}
	return len(v.Questions), nil
}
