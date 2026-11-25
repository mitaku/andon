package dispatcher

import (
	"strings"

	"andon/internal/ledger"
)

// 止まった担当の自動 resume（ADR-0012 決めたこと 4・13 の 3。dispatcher.md §2.3）。
// 見回りで doing の課題の端末が Gone（無い・live でない・素のシェル）なら andon issue resume <id> を1回打つ。
// 1つの死につき1回: 控えの Resumed に「死んだのを見た端末」と「起動した端末」を持ち、起動した端末が働かないまま
// （turn 0 のまま）また Gone なら打たずに finished を知らせる（オーナーの go へ）。働き始めたら（turn ≥ 1）控えを消し、
// 次に死んだらまた1回打つ。初めて見る kanban では打たない（控えに「見た」とだけ持つ）。
// resume・finish・start は同じ lock（andon issue の <slug>.issue.lock）を命令の間ずっと持つので、resume の間 finish は待つ。

type Resumed struct {
	From  string `json:"from"`
	Agent string `json:"agent,omitempty"`
}

func ResumeGone(prev Resumed, had bool, agent string, first bool) bool {
	switch {
	case first:
		return false
	case had && prev.From == agent:
		return false
	case had && prev.Agent == agent:
		return false
	}
	return true
}

func (p *project) resumeGone(t ledger.Issue, agent string) bool {
	id := t.Str("id")
	prev, had := p.st.Resumed[id]
	if !ResumeGone(prev, had, agent, p.first) {
		if p.first && !had {
			p.st.Resumed[id] = Resumed{From: agent}
		}
		return false
	}
	p.st.Resumed[id] = Resumed{From: agent}
	if err := p.st.Save(p.l.Slug); err != nil {
		if had {
			p.st.Resumed[id] = prev
		} else {
			delete(p.st.Resumed, id)
		}
		p.logf("%s の担当 %s は Gone。resume を打たない（控えを書けない。次の見回りで）: %v", id, agent, err)
		return true
	}
	_, warn, _, err := p.andonStderr("", "issue", "resume", id)
	if err != nil {
		p.logf("%s の担当 %s は Gone。resume で起動できない（finished で知らせる）: %v", id, agent, err)
		return false
	}
	// 起動できた回の警告（道具を入れられない・入れた。ADR-0063 決めたこと 7）は、ほかに届く所が無いのでログに残す
	for _, line := range strings.Split(strings.TrimSpace(warn), "\n") {
		if line != "" {
			p.logf("%s の resume: %s", id, line)
		}
	}
	now, err := p.l.Show(id)
	next := ""
	if err == nil {
		next = now.Str("agent")
	}
	if next == "" || next == agent {
		p.logf("%s を resume したが、kanban の agent が読めない・替わっていない（finished で知らせる）: %v", id, err)
		return false
	}
	p.st.Resumed[id] = Resumed{From: agent, Agent: next}
	p.notify(id, "resumed", map[string]any{"agent": next, "from": agent})
	if err := p.st.Save(p.l.Slug); err != nil {
		p.logf("%s の resume の控えを保存できない（巡の最後にもう一度保存する）: %v", id, err)
	}
	p.logf("%s の担当 %s は Gone。resume して %s で起動した", id, agent, next)
	return true
}

func (p *project) worked(id string) {
	prev := p.st.Resumed[id]
	delete(p.st.Resumed, id)
	if err := p.st.Save(p.l.Slug); err != nil {
		p.st.Resumed[id] = prev
		p.logf("%s の resume の控えを消せない（次の見回りで）: %v", id, err)
	}
}

func (p *project) keepResumed() {
	for id := range p.st.Resumed {
		if p.status(id) != "doing" {
			delete(p.st.Resumed, id)
		}
	}
}
