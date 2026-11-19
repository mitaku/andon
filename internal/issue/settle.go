package issue

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/dispatcher"
	"andon/internal/ledger"
)

// prPoll は同じ PR を GitHub に聞く間隔（dispatcher は待ちがあると5秒ごとに打つので、gh の上限を食わない。dispatcher.md §2.7）。
const prPoll = time.Minute

// settlePath は settle の記録（~/.local/state/andon/dispatcher/<slug>.settle.json。書き手は andon issue settle だけ。ADR-0017 決めたこと 5）。
func settlePath(slug string) string {
	return filepath.Join(filepath.Dir(dispatcher.StatePath(slug)), slug+".settle.json")
}

// settle は andon issue settle: review の課題の PR を人がマージしたかを見て、マージしていたら dispatcher がマージしたときと
// 同じ道（ゲートの問いを閉じて finish）で片付ける。マージはしない。積む通知は stdout の JSON で返す（積むのは dispatcher）。
// 2回打っても同じ結果になるよう、聞いた時刻・MERGED／CLOSED・知らせた理由を自分の記録に持つ（dispatcher.md §2.7）。
//
//	MERGED（ゲートの記録の merged が true）   → dispatcher がマージした。何もしない（approve の道が片付ける）
//	MERGED                                   → ゲートの問い（from: gate）を「人がマージした」と閉じて finish、通知 merged（by: person）
//	MERGED でゲートの外の問いが開いている     → 片付けずに通知 merged（held）。同じ理由では二度知らせない
//	CLOSED                                   → 片付けない。通知 closed を1回だけ
func (c *ctx) settle(stdout, stderr io.Writer, now time.Time) error {
	path := settlePath(c.l.Slug)
	rec := map[string]contract.Settled{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &rec); err != nil {
			return fail("settle の記録が読めない（%s）: %v", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := forgetOthers(c, rec); err != nil {
		return err
	}
	res := contract.Settle{Issue: c.id, State: "skip", Notify: []contract.Notice{}}
	w, err := c.settleOne(rec[c.id], &res, stderr, now)
	if w == nil {
		delete(rec, c.id)
	} else {
		rec[c.id] = *w
	}
	if err2 := os.MkdirAll(filepath.Dir(path), 0o755); err2 != nil {
		return err2
	}
	if err2 := contract.Write(path, rec); err2 != nil {
		return fail("settle の記録を書けない: %v", err2)
	}
	if err != nil {
		return err
	}
	_, err = stdout.Write(contract.Marshal(res))
	return err
}

// settleOne は1つの課題を見て res を埋め、残す記録を返す（nil なら記録から外す）。
func (c *ctx) settleOne(w contract.Settled, res *contract.Settle, stderr io.Writer, now time.Time) (*contract.Settled, error) {
	if c.issue.Str("status") != "review" {
		return nil, nil
	}
	rep, err := c.previous()
	if err != nil || rep == nil || rep.PR == "" {
		return nil, err
	}
	// 見るのは issue.yaml の repo の PR だけ（ほかの repo の PR がマージされても、この課題は片付かない）
	owner, name, number, ok := contract.ParsePR(rep.PR)
	if !ok || "github.com/"+owner+"/"+name != c.issue.Str("repo") {
		return nil, nil
	}
	if w.PR != rep.PR || w.Commit != rep.Commit {
		w = contract.Settled{PR: rep.PR, Commit: rep.Commit}
	}
	res.State = "watching"
	if w.State == "" {
		if at, err := time.Parse("2006-01-02T15:04:05Z", w.At); err == nil && now.Sub(at) < prPoll {
			return &w, nil
		}
		w.At = now.UTC().Format("2006-01-02T15:04:05Z")
		state, head, sha, err := prState(number, owner+"/"+name)
		if err != nil {
			return &w, fail("%s の PR の状態が読めない（次に打ったとき聞き直す）: %v", c.id, err)
		}
		switch state {
		case "MERGED":
			w.State, w.Head, w.Sha = state, head, sha
		case "CLOSED":
			w.State = state
			res.Notify = append(res.Notify, contract.Notice{Kind: "closed", Detail: map[string]any{"pr": rep.PR}})
		}
	}
	switch w.State {
	case "CLOSED":
		res.State = "closed"
		return &w, nil
	case "MERGED":
	default:
		return &w, nil
	}
	// dispatcher が自分でマージした PR は ゲートの記録の merged が true（dispatcher の控えは読まない。ADR-0017 決めたこと 5 の (d)）
	if mergedByGate(filepath.Join(c.l.Library(c.id), "gate", w.Head+".json")) {
		res.State = "auto"
		return &w, nil
	}
	if why := c.closeAndFinish(w.PR, stderr); why != "" {
		res.State = "held"
		if why != w.Held {
			res.Notify = append(res.Notify, contract.Notice{Kind: "merged", Detail: map[string]any{"pr": w.PR, "sha": w.Sha, "by": "person", "held": why}})
			w.Held = why
		}
		return &w, nil
	}
	res.State = "finished"
	res.Notify = append(res.Notify, contract.Notice{Kind: "merged", Detail: map[string]any{"pr": w.PR, "sha": w.Sha, "by": "person"}})
	return nil, nil
}

// closeAndFinish はゲートの問いを「人がマージした」と閉じて finish する。片付けなかったら理由を返す。
func (c *ctx) closeAndFinish(pr string, stderr io.Writer) string {
	gateQs, others, err := c.openQuestions()
	if err != nil {
		return err.Error()
	}
	if len(others) > 0 {
		return "ゲートの外の問いが開いている: " + strings.Join(others, "・")
	}
	for _, q := range gateQs {
		name := fmt.Sprintf("%s#%d", c.id, q.N)
		if q.Status == "open" {
			if _, err := c.l.Run("", "answer", name, "人がマージした（"+pr+"）"); err != nil {
				return "ゲートの問い " + name + " を閉じられない: " + err.Error()
			}
		}
		if _, err := c.l.Run("", "received", name); err != nil {
			return "ゲートの問い " + name + " を閉じられない: " + err.Error()
		}
	}
	if err := allowed("finish", c.id, c.issue.Str("status")); err != nil {
		return err.Error()
	}
	if err := c.finish(stderr); err != nil {
		return err.Error()
	}
	c.writeBack(stderr) // done になってから。書けなくても finish は戻さない（ADR-0011 決めたこと 15）
	return ""
}

// question は kanban questions --json の1件（open と answered。received は出ない）。
type question struct {
	ID     string `json:"id"`
	N      int    `json:"n"`
	Status string `json:"status"`
}

// openQuestions はこの課題のまだ受け取られていない問いを、ゲートが出したもの（library/questions/<n>.json の from: gate）と
// それ以外（名前 <id>#<n>）に分ける。問いの JSON が無い・読めないものはゲートの外に数える（閉じない）。
func (c *ctx) openQuestions() ([]question, []string, error) {
	out, err := c.l.Run("", "questions", "--json")
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
		if ledger.CanonID(q.ID) != c.id || q.Status == "received" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(c.l.Library(c.id), "questions", strconv.Itoa(q.N)+".json"))
		var v struct {
			From string `json:"from"`
		}
		if err == nil && json.Unmarshal(b, &v) == nil && v.From == "gate" {
			gate = append(gate, q)
		} else {
			others = append(others, fmt.Sprintf("%s#%d", c.id, q.N))
		}
	}
	sort.Strings(others)
	return gate, others, nil
}

// mergedByGate はゲートの記録の merged が true か（dispatcher がマージした印。contract.md §7）。
func mergedByGate(record string) bool {
	b, err := os.ReadFile(record)
	var v struct {
		Merged bool `json:"merged"`
	}
	return err == nil && json.Unmarshal(b, &v) == nil && v.Merged
}

// prState は PR の state（OPEN・MERGED・CLOSED）と、MERGED なら head とマージの commit（gh pr view。1回）。
func prState(number, slug string) (state, head, sha string, err error) {
	cmd := exec.Command("gh", "pr", "view", number, "--repo", slug, "--json", "state,mergeCommit,headRefOid")
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("gh pr view: %v %s", err, firstLine(errb.String()))
	}
	var view struct {
		State       string `json:"state"`
		HeadRefOid  string `json:"headRefOid"`
		MergeCommit *struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if json.Unmarshal(out, &view) != nil {
		return "", "", "", fmt.Errorf("gh pr view の返事が読めない")
	}
	switch view.State {
	case "MERGED":
		if view.MergeCommit == nil || !contract.IsSHA(view.MergeCommit.Oid) {
			return "", "", "", fmt.Errorf("MERGED だがマージの commit が読めない")
		}
		return view.State, view.HeadRefOid, view.MergeCommit.Oid, nil
	case "OPEN", "CLOSED":
		return view.State, "", "", nil
	}
	return "", "", "", fmt.Errorf("PR の state が知らない値: %q", view.State)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
