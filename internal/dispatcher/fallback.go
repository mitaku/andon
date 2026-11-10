package dispatcher

import (
	"fmt"

	"andon/internal/ledger"
	"andon/internal/spawn"
)

// レビュアーが使用量の上限で止まったときの逃げ口（dispatcher.md §2.8。decisions/2026-11-09-reviewer-allowance-fallback.md）。

// otherReview はもう一方のレビューのプロファイル。
var otherReview = map[string]string{"codex-review": "claude-review", "claude-review": "codex-review"}

// Fallback は、プロファイル reviewer のレビュアーが使用量の上限で止まったときに起こし直すプロファイル。
// 担当（元の課題）のプロファイル impl と別ブランドが保てるときだけ ok（同じブランドのレビューは独立レビューに数えない）。
func Fallback(impl, reviewer string) (string, bool) {
	alt := otherReview[reviewer]
	if alt == "" || ledger.Brand(alt) == ledger.Brand(impl) {
		return "", false
	}
	return alt, true
}

// fallback はレビュアーの課題 t（端末 agent が使用量の上限で止まった）を、もう一方のレビューのプロファイルで起こし直す。
// 止まった端末を片付け、課題の profile を替えて逃げた理由（fallback）を台帳に書き（by は terminal:dispatcher。log.jsonl に残る）、
// 同じ課題・同じ detached worktree・同じ最初のメッセージで起こして agent を替える。
// 起こし直さない（別ブランドが保てない・途中で失敗した）ときは false で、呼び手は今どおり error を知らせる
// （端末を片付けたあとの失敗は、次の見回りで端末が無い〈finished〉として知らせる）。
func (p *project) fallback(t ledger.Issue, agent string) bool {
	rev, from := t.Str("id"), t.Str("profile")
	var parent ledger.Issue
	for _, c := range p.tasks {
		if c.Str("id") == t.Str("parent") {
			parent = c
		}
	}
	if parent == nil {
		p.logf("%s のレビュアー %s が使用量の上限で止まった。元の課題 %s が台帳に無いので起こし直さない", rev, agent, t.Str("parent"))
		return false
	}
	to, ok := Fallback(parent.Str("profile"), from)
	if !ok {
		p.logf("%s のレビュアー %s（%s）が使用量の上限で止まった。担当（%s）と別ブランドのプロファイルが残らないので起こし直さない（知らせる）", rev, agent, from, orNone(parent.Str("profile")))
		return false
	}
	for _, args := range [][]string{{"kill", agent}, {"rm", agent}} { // 同じ名前（<slug>-<rev>）で起こし直すので片付ける
		if _, _, err := run("", nil, "", "hc", args...); err != nil {
			p.logf("%s のレビュアー %s を片付けられない（起こし直さない）: %v", rev, agent, err)
			return false
		}
	}
	why := fmt.Sprintf("%s → %s（使用量の上限で止まった。端末 %s）", from, to, agent)
	if _, err := p.l.Run("", "set", rev, "profile="+to, "fallback="+why); err != nil {
		p.logf("%s の profile を替えられない（端末 %s は片付けた。次の見回りで finished として知らせる）: %v", rev, agent, err)
		return false
	}
	started, err := p.runReviewer(parent, rev, t.Str("title"), to)
	if err != nil {
		p.logf("%s のレビュアーを %s で起こせない（次の見回りで finished として知らせる）: %v", rev, to, err)
		return false
	}
	if _, err := p.l.Run("", "set", rev, "agent="+started.Address); err != nil {
		// 台帳が新しい端末を指さなければ見回りが追えない。起こした端末を片付け、課題は止まった端末を指したまま
		// （次の見回りで端末が無い〈finished〉として Chief of Staff に知らせる）
		for _, args := range [][]string{{"kill", started.Address}, {"rm", started.Address}} {
			if _, _, kerr := run("", nil, "", "hc", args...); kerr != nil {
				p.logf("%s の起こし直した端末 %s を片付けられない: %v", rev, started.Address, kerr)
			}
		}
		p.logf("%s の agent を %s にできない（起こした端末を片付けた。次の見回りで finished として知らせる）: %v", rev, started.Address, err)
		return false
	}
	if !started.First.Delivered() {
		// 届いていなければ見回りの sendFirst に1回だけの送り直しを預ける（startReviewer と同じ）
		if err := spawn.MarkUnsent(started.Address); err != nil {
			p.logf("%s（%s）に最初のメッセージが届いていない。ラベルを付けられない: %v", rev, started.Address, err)
		}
	}
	p.logf("%s のレビュアーを %s で起こし直した: %s", rev, started.Address, why)
	return true
}
