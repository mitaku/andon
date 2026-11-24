package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"andon/internal/contract"
)

var askedRe = regexp.MustCompile(`#([1-9][0-9]*)$`)

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

func ownerTitle(q *contract.Question) string { return q.Question + "（オーナーの判断待ち）" }

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

// Mark は、オーナーの課題を足すときに書く置いておく印（予約キー later と later_until。ADR-0062 決めたこと 3・10）。
// 空の Later は呼ぶ課題で、何も書かない。印を信じるのは Chief of Staff の端末が書いたときだけなので、呼ぶ側が端末を確かめる。
type Mark struct {
	Later string
	Until string
}

// keys は kanban set に渡す印。left は続きを置く課題（打ち直し）が今持つキーで、m が書かないキーを持っていれば消す
// （前の回と旗が違う打ち直しで、前の回の印を残さない。持たないキーは触らず、履歴に行を足さない）。
func (m Mark) keys(left Issue) []string {
	var kv []string
	for _, k := range []struct{ key, val string }{{"later", m.Later}, {"later_until", m.Until}} {
		if k.val != "" {
			kv = append(kv, k.key+"="+k.val)
		} else if _, ok := left[k.key]; ok {
			kv = append(kv, k.key+"=")
		}
	}
	return kv
}

// NewOwnerQuestion はオーナーの課題を足し、who: person と印 m を書き、問いを置く（andon issue ask --new。ADR-0057 決めたこと 7）。
// 足した id と kanban の CLI の返事（<id>#<n>）を返す。
func (l *Ledger) NewOwnerQuestion(q *contract.Question, parent string, after []string, m Mark) (string, string, error) {
	return l.NewOwnerIssue(q, ownerTitle(q), ownerBrief(q), parent, after, m)
}

// NewOwnerIssue は NewOwnerQuestion の、タイトルと依頼書を呼ぶ側が決める口（振り返りの apply は今のタイトルと依頼書のまま
// 足す。ADR-0057 決めたこと 11）。
// 問いは確かめたもの（contract.ParseQuestion）を渡す。scope の問い（Chief of Staff が答える）は置かない。
// after があれば課題は queued で作られ、その課題にも問いを置く。
// 同じタイトルで同じ parent の片付いていない課題が、問いを持たずにあれば、足し直さずにその課題へ続きを置く（足したあと who を
// 書く前や問いを置く前に落ちた回の打ち直し）。問いを持っていれば、同じ問いを 2 つ足さないよう断る。ただし問いが q と同じ 1 つだけで、
// 問いの控えが無ければ（kanban ask のあと控えを書く前に落ちた回）、控えだけを書き直して続ける（RestoreQuestion）。
// 足した id と kanban の CLI の返事（<id>#<n>）を返す。課題を足したあとで落ちたら、足した id と打ち直す command を誤りに書く。
// 印 m は who と同じ kanban set で、問いを置く前に書く（問いを置いたあとに落ちると、打ち直しは同じ問いとして断るので、印が書けない）。
func (l *Ledger) NewOwnerIssue(q *contract.Question, title, brief, parent string, after []string, m Mark) (string, string, error) {
	if q.Kind == "scope" {
		return "", "", fmt.Errorf("scope の問いはオーナーの課題に置かない（Chief of Staff が答える問い。kind は business/users/promise/irreversible/line）")
	}
	id, left, asked, err := l.unfinished(title, parent)
	if err != nil {
		return "", "", err
	}
	if asked > 0 {
		if asked == 1 {
			restored, err := l.RestoreQuestion(id, q)
			if err != nil {
				return id, "", err
			}
			if restored {
				return id, id + "#1", nil
			}
		}
		return "", "", fmt.Errorf("同じ問いのオーナーの課題 %s がもうある（問いを足すなら andon issue ask %s -）", id, id)
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
	if _, err := l.Run("", append([]string{"set", id, "who=person"}, m.keys(left)...)...); err != nil {
		return id, "", fmt.Errorf("課題 %s を足したが who: person と印を書けなかった（%v）。同じ command を打ち直すと続きから置く", id, err)
	}
	out, err := l.Ask(id, q)
	if err != nil {
		return id, "", fmt.Errorf("オーナーの課題 %s を足したが問いを置けなかった（%v）。同じ command を打ち直すと続きから置く", id, err)
	}
	return id, out, nil
}

func (l *Ledger) unfinished(title, parent string) (string, Issue, int, error) {
	tasks, err := l.List()
	if err != nil {
		return "", nil, 0, err
	}
	for _, t := range tasks {
		if t.Str("title") != title || t.Str("status") == "done" || CanonID(t.Str("parent")) != CanonID(parent) {
			continue
		}
		id := t.Str("id")
		n, err := l.Questions(id)
		if err != nil {
			return "", nil, 0, err
		}
		return id, t, n, nil
	}
	return "", nil, 0, nil
}

// RestoreQuestion は、課題 id の 1 つ目の問いの控え（library/questions/1.json）が無く、kanban の 1 つ目の問いの文が q の文
// （q.Text()）と同じなら、控えを q で書き、true を返す。kanban ask のあと控えを書く前に落ちた回の打ち直しで呼ぶ（控えが無いと
// andon waiting に選択肢が出ず、オーナーが答えられない）。控えがあれば何も書かずに false。控えが無いのに文が違う（打ち直しの前に
// 問いが変わった）・1 つ目の問いが無ければ、書き直せないので誤りを返す（控えの無い問いを残したまま進まない）。
func (l *Ledger) RestoreQuestion(id string, q *contract.Question) (bool, error) {
	p := filepath.Join(l.Library(id), "questions", "1.json")
	if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	out, err := l.Run("", "show", id, "--json")
	if err != nil {
		return false, err
	}
	var v struct {
		Questions []struct {
			N    int    `json:"n"`
			Text string `json:"text"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return false, fmt.Errorf("課題 %s が読めない", id)
	}
	if len(v.Questions) == 0 || v.Questions[0].N != 1 || v.Questions[0].Text != strings.TrimRight(q.Text(), " \t\r\n") {
		return false, fmt.Errorf("課題 %s の問いの控え（library/questions/1.json）が無く、kanban の問いの文が置く問いと違うので書き直せない"+
			"（kanban show %s で問いを見て、控えを andon issue ask の JSON で書き直すか、問いを置き直す）", id, id)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return false, err
	}
	if err := contract.Write(p, contract.SchemaQuestion, q); err != nil {
		return false, err
	}
	return true, nil
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
