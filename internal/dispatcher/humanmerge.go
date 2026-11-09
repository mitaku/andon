package dispatcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// prPoll は review の課題の PR を GitHub に聞く間隔（待ちの無い見回りと同じ1分。dispatcher.md §2.7）。
// 待ちがあると見回りは5秒ごとに回るので、課題ごとに打った時刻を控えて gh の上限を食わない。
const prPoll = time.Minute

// Watch は review の課題の PR を人がマージしたかの控え（控えの「issue → PR の状態」）。
type Watch struct {
	PR    string `json:"pr"`
	At    string `json:"at,omitempty"`    // 最後に gh pr view を打った時刻
	State string `json:"state,omitempty"` // MERGED・CLOSED（どちらも、もう GitHub に聞かない）
	Sha   string `json:"sha,omitempty"`   // マージの commit
	Held  string `json:"held,omitempty"`  // 片付けずに知らせた理由（同じ理由では二度知らせない）
}

// humanMerge は review の課題の PR が GitHub でマージされたかを見て、人がマージしたなら dispatcher がマージしたときと
// 同じ道（finish → 通知 merged → 次の巡で impact・corrections）を通す（dispatcher.md §2.7）。マージはしない。
//
//	MERGED                         → ゲートの問い（from: gate）を「人がマージした」と答えて閉じ、andon issue finish、通知 merged（by: person）
//	MERGED でゲートの外の問いが開いている → 片付けずに通知 merged（held）。問いが閉じたら次の巡で片付ける（GitHub には聞き直さない）
//	CLOSED                         → 片付けない。通知 closed を1回だけ
func (p *project) humanMerge(t ledger.Issue, rep *contract.Report, now time.Time) {
	id := t.Str("id")
	if rep.PR == "" {
		return
	}
	for _, g := range p.st.Gates {
		if g.Issue == id && g.Merged != "" {
			return // dispatcher が自分でマージした（approve の道が片付ける）
		}
	}
	w := p.st.Watch[id]
	if w.PR != rep.PR {
		w = Watch{PR: rep.PR}
	}
	finished := false
	defer func() {
		if finished {
			delete(p.st.Watch, id)
		} else {
			p.st.Watch[id] = w
		}
	}()
	if w.State == "" {
		if at, err := time.Parse("2006-01-02T15:04:05Z", w.At); err == nil && now.Sub(at) < prPoll {
			return
		}
		w.At = now.UTC().Format("2006-01-02T15:04:05Z")
		state, sha, err := prState(rep.PR)
		if err != nil {
			p.logf("%s の PR の状態が読めない（次の見回りで聞き直す）: %v", id, err)
			return
		}
		switch state {
		case "MERGED":
			w.State, w.Sha = state, sha
			p.logf("%s の PR %s は人がマージした（%s）", id, rep.PR, short12(sha))
		case "CLOSED":
			w.State = state
			p.logf("%s の PR %s はマージされずに閉じた（片付けない）", id, rep.PR)
			p.notify(id, "closed", map[string]any{"pr": rep.PR})
			return
		default:
			return
		}
	}
	if w.State != "MERGED" {
		return
	}
	if why := p.settle(id, w); why != "" {
		if why != w.Held {
			p.logf("%s は人がマージしたが片付けない: %s", id, why)
			p.notify(id, "merged", map[string]any{"pr": w.PR, "sha": w.Sha, "by": "person", "held": clean(why, 200)})
			w.Held = why
		}
		return
	}
	p.logf("%s は人がマージしたので片付けた", id)
	p.notify(id, "merged", map[string]any{"pr": w.PR, "sha": w.Sha, "by": "person"})
	finished = true
}

// settle はゲートの問いを閉じて finish する。片付けなかったら理由を返す。
func (p *project) settle(id string, w Watch) string {
	gateQs, others, err := p.openQuestions(id)
	if err != nil {
		return err.Error()
	}
	if len(others) > 0 {
		return "ゲートの外の問いが開いている: " + strings.Join(others, "・")
	}
	for _, n := range gateQs {
		name := fmt.Sprintf("%s#%d", id, n.N)
		if n.Status == "open" {
			if _, err := p.l.Run("", "answer", name, "人がマージした（"+w.PR+"）"); err != nil {
				return "ゲートの問い " + name + " を閉じられない: " + err.Error()
			}
		}
		if _, err := p.l.Run("", "received", name); err != nil {
			return "ゲートの問い " + name + " を閉じられない: " + err.Error()
		}
	}
	return p.finish(id)
}

// question は kanban questions --json の1件（open と answered。received は出ない）。
type question struct {
	ID     string `json:"id"`
	N      int    `json:"n"`
	Status string `json:"status"`
}

// openQuestions は課題 id のまだ受け取られていない問いを、ゲートが出したもの（library/questions/<n>.json の from: gate）と
// それ以外（名前 <id>#<n>）に分ける。問いの JSON が無い・読めないものはゲートの外に数える（閉じない）。
func (p *project) openQuestions(id string) ([]question, []string, error) {
	out, err := p.l.Run("", "questions", "--json")
	if err != nil {
		return nil, nil, err
	}
	var all []question
	if out != "" {
		if err := json.Unmarshal([]byte(out), &all); err != nil {
			return nil, nil, fmt.Errorf("問いの一覧が読めない")
		}
	}
	var gate []question
	var others []string
	for _, q := range all {
		if ledger.CanonID(q.ID) != id || q.Status == "received" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), "questions", strconv.Itoa(q.N)+".json"))
		var v struct {
			From string `json:"from"`
		}
		if err == nil && json.Unmarshal(b, &v) == nil && v.From == "gate" {
			gate = append(gate, q)
		} else {
			others = append(others, fmt.Sprintf("%s#%d", id, q.N))
		}
	}
	sort.Strings(others)
	return gate, others, nil
}

// forget は review にいない課題の控え（Watch）を捨てる（片付いた・doing に戻った）。
func (p *project) forget() {
	for id := range p.st.Watch {
		if p.status(id) != "review" {
			delete(p.st.Watch, id)
		}
	}
}

// prState は PR の state（OPEN・MERGED・CLOSED）とマージの commit（gh pr view。1回）。
func prState(pr string) (string, string, error) {
	owner, name, number, ok := contract.ParsePR(pr)
	if !ok {
		return "", "", fmt.Errorf("PR の URL が読めない: %s", pr)
	}
	out, _, err := run("", nil, "", "gh", "pr", "view", number, "--repo", owner+"/"+name, "--json", "state,mergeCommit")
	if err != nil {
		return "", "", err
	}
	var view struct {
		State       string `json:"state"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if json.Unmarshal([]byte(out), &view) != nil {
		return "", "", fmt.Errorf("gh pr view の返事が読めない")
	}
	switch view.State {
	case "MERGED":
		if view.MergeCommit == nil || !contract.IsSHA(view.MergeCommit.Oid) {
			return "", "", fmt.Errorf("MERGED だがマージの commit が読めない")
		}
		return view.State, view.MergeCommit.Oid, nil
	case "OPEN", "CLOSED":
		return view.State, "", nil
	}
	return "", "", fmt.Errorf("PR の state が知らない値: %q", view.State)
}
