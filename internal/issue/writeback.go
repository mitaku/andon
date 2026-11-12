package issue

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"andon/internal/delegate"
)

// githubIssueRe は GitHub の issue の URL（https://github.com/<owner>/<repo>/issues/<n>）。組 1 が <host>/<owner>/<repo>。
var githubIssueRe = regexp.MustCompile(`^https://(github\.com/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+)/issues/[1-9][0-9]*$`)

// writeBack は課題が done になったあと、links にある write_back: true の出所のトラッカーへ1コメント書く
// （課題 id・PR の URL・結果。ADR-0011 決めたこと 15・contract.md §11）。状態は変えない（打つのは gh issue comment だけ）。
// 書くのは GitHub の issue（github-issues の出所）だけ。書けなくても finish は戻さず、理由を1行出す。
// finish は done の課題には打てない（process/states/issue.yaml）ので、1つの課題で書くのは1回。
func (c *ctx) writeBack(stdout io.Writer) {
	links := c.links()
	if len(links) == 0 || strings.HasSuffix(c.issue.Str("profile"), "-review") {
		return
	}
	p, err := delegate.ReadProject(delegate.Workspace(), c.l.Slug)
	if errors.Is(err, delegate.ErrAbsent) {
		return
	}
	if err != nil {
		fmt.Fprintf(stdout, "andon issue: %s のトラッカーへは書かない（project.json を読めない: %v）\n", c.id, err)
		return
	}
	targets := trackers(p, links)
	if len(targets) == 0 {
		return
	}
	pr, result, err := c.outcome()
	if err != nil {
		fmt.Fprintf(stdout, "andon issue: %s のトラッカーへ書けなかった（%v）。手で書く: %s\n", c.id, err, strings.Join(targets, " "))
		return
	}
	if pr == "" {
		pr = "無し"
	}
	body := fmt.Sprintf("Andon: 課題 %s が終わった。\n- PR: %s\n- 結果: %s\n", c.id, pr, result)
	for _, u := range targets {
		var errb bytes.Buffer
		cmd := exec.Command("gh", "issue", "comment", u, "--body", body)
		cmd.Stderr = &errb
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(stdout, "andon issue: %s のトラッカーへ書けなかった（gh issue comment: %v %s）。手で書く: %s\n", c.id, err, strings.TrimSpace(errb.String()), u)
			continue
		}
		fmt.Fprintf(stdout, "andon issue: %s の結果を %s に書いた\n", c.id, u)
	}
}

// trackers は links のうち、write_back: true の github-issues の出所の issue の URL（重なりを除く）。
// 出所の repo が無ければ project.json の repos の全部が出所（contract.md §11）。
func trackers(p *delegate.Project, links []string) []string {
	var out []string
	for _, u := range links {
		m := githubIssueRe.FindStringSubmatch(u)
		if m == nil || slices.Contains(out, u) {
			continue
		}
		for _, s := range p.Signals {
			if s.Source != "github-issues" || s.WriteBack == nil || !*s.WriteBack {
				continue
			}
			if (s.Repo == nil && slices.Contains(p.Repos, m[1])) || (s.Repo != nil && *s.Repo == m[1]) {
				out = append(out, u)
				break
			}
		}
	}
	return out
}

// outcome は report.json の pr と結果（MERGED ならマージ、CLOSED なら閉じ）。PR が無ければ閉じ。
// PR が読めない・OPEN など MERGED でも CLOSED でもなければ誤り（結果を書き違えない。外へのコメントは戻せない）。
func (c *ctx) outcome() (string, string, error) {
	rep, err := c.previous()
	if err != nil {
		return "", "", err
	}
	if rep == nil || rep.PR == "" {
		return "", "閉じ（PR 無し）", nil
	}
	out, err := exec.Command("gh", "pr", "view", rep.PR, "--json", "state", "-q", ".state").Output()
	if err != nil {
		return "", "", fmt.Errorf("PR を読めない: %s", rep.PR)
	}
	switch st := strings.TrimSpace(string(out)); st {
	case "MERGED":
		return rep.PR, "マージ", nil
	case "CLOSED":
		return rep.PR, "閉じ（マージせず）", nil
	default: // OPEN など: マージも閉じもしていない PR の結果は書けない（やめた課題の PR が開いたまま、など）
		return "", "", fmt.Errorf("PR が MERGED でも CLOSED でもない: %s %s", rep.PR, st)
	}
}

// links は issue.yaml の links（並び）。
func (c *ctx) links() []string {
	var out []string
	switch v := c.issue["links"].(type) {
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		out = append(out, v)
	}
	return out
}
