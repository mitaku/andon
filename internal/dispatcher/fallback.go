package dispatcher

import (
	"fmt"

	"andon/internal/launch"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// レビュアーが使用量の上限で止まったときのフォールバック（dispatcher.md §2.8。decisions/2026-11-09-reviewer-allowance-fallback.md）。

var otherReview = map[string]string{"codex-review": "claude-review", "claude-review": "codex-review"}

func Fallback(impl, reviewer string) (string, bool) {
	alt := otherReview[reviewer]
	if alt == "" || ledger.Brand(alt) == ledger.Brand(impl) {
		return "", false
	}
	return alt, true
}

func (p *project) fallback(t ledger.Issue, agent string) bool {
	rev, from := t.Str("id"), t.Str("profile")
	var parent ledger.Issue
	for _, c := range p.tasks {
		if c.Str("id") == t.Str("parent") {
			parent = c
		}
	}
	if parent == nil {
		p.logf("%s のレビュアー %s が使用量の上限で止まった。元の課題 %s が kanban に無いので起動し直さない", rev, agent, t.Str("parent"))
		return false
	}
	to, ok := Fallback(parent.Str("profile"), from)
	if !ok {
		p.logf("%s のレビュアー %s（%s）が使用量の上限で止まった。担当（%s）と別ブランドのプロファイルが残らないので起動し直さない（知らせる）", rev, agent, from, orNone(parent.Str("profile")))
		return false
	}
	for _, args := range [][]string{{"kill", agent}, {"rm", agent}} {
		if _, _, err := run("", nil, "", "hc", args...); err != nil {
			p.logf("%s のレビュアー %s を片付けられない（起動し直さない）: %v", rev, agent, err)
			return false
		}
	}
	why := fmt.Sprintf("%s → %s（使用量の上限で止まった。端末 %s）", from, to, agent)
	if _, err := p.l.Run("", "set", rev, "profile="+to, "fallback="+why); err != nil {
		p.logf("%s の profile を替えられない（端末 %s は片付けた。次の見回りで finished として知らせる）: %v", rev, agent, err)
		return false
	}
	started, session, err := p.runReviewer(parent, rev, t.Str("title"), to, launch.LaunchFallback, t.Str("worktree"))
	if err != nil {
		p.logf("%s のレビュアーを %s で起動できない（次の見回りで finished として知らせる）: %v", rev, to, err)
		return false
	}
	// session は新しい端末のもの（Codex 系は空にして消す。resume が記録から引く）
	if _, err := p.l.Run("", "set", rev, "agent="+started.Address, "session="+session); err != nil {
		for _, args := range [][]string{{"kill", started.Address}, {"rm", started.Address}} {
			if _, _, kerr := run("", nil, "", "hc", args...); kerr != nil {
				p.logf("%s の起動し直した端末 %s を片付けられない: %v", rev, started.Address, kerr)
			}
		}
		p.logf("%s の agent を %s にできない（起動した端末を片付けた。次の見回りで finished として知らせる）: %v", rev, started.Address, err)
		return false
	}
	if !started.First.Delivered() {
		if err := spawn.MarkUnsent(started.Address); err != nil {
			p.logf("%s（%s）に最初のメッセージが届いていない。ラベルを付けられない: %v", rev, started.Address, err)
		}
	}
	p.logf("%s のレビュアーを %s で起動し直した: %s", rev, started.Address, why)
	return true
}
