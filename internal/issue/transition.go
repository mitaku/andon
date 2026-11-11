package issue

import (
	"fmt"
	"slices"
	"strings"

	"andon"
	"andon/internal/process"

	"go.yaml.in/yaml/v3"
)

// moves は台帳の status を移す（か、移さずに書く）andon issue のサブコマンド。process/states/issue.yaml の
// transitions[].command に「andon issue <サブコマンド>」の形で出るものだけを打てる（ADR-0017 決めたこと 15・
// 受け入れ条件 11）。show・impact・resume・session・stop-check は移り方ではない（読むだけ・起こし直すだけ）。
var moves = []string{"start", "report", "review-request", "candidates", "ask", "ready", "finish"}

// froms は定義（issue.yaml）から、サブコマンドごとに打ってよい status を引く。by: deck の移り方は
// kanban のコマンドなので入らない（断らない）。command の「andon issue a・b」は a と b の両方。
func froms(def []byte) (map[string][]string, error) {
	var sm process.StateMachine
	if err := yaml.Unmarshal(def, &sm); err != nil {
		return nil, fmt.Errorf("process/states/issue.yaml が読めない: %w", err)
	}
	out := map[string][]string{}
	for _, t := range sm.Transitions {
		rest, ok := strings.CutPrefix(t.Command, "andon issue ")
		if !ok {
			continue
		}
		for _, sub := range strings.Split(rest, "・") {
			sub = strings.TrimSpace(sub)
			if !slices.Contains(out[sub], t.From) {
				out[sub] = append(out[sub], t.From)
			}
		}
	}
	return out, nil
}

// allowed は status の課題に andon issue <sub> を打ってよいかを確かめる。定義に無ければ理由1行の誤り。
func allowed(sub, id, status string) error {
	if !slices.Contains(moves, sub) {
		return nil
	}
	f, err := froms(andon.IssueStates)
	if err != nil {
		return err
	}
	if slices.Contains(f[sub], status) {
		return nil
	}
	if len(f[sub]) == 0 {
		return fail("%s は %s（andon issue %s の移り方が process/states/issue.yaml に無い）", id, status, sub)
	}
	return fail("%s は %s（andon issue %s は %s の課題にだけ打てる。process/states/issue.yaml）", id, status, sub, strings.Join(f[sub], "・"))
}
