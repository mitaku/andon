package dispatcher

import (
	"andon/internal/ledger"
)

// 止まった担当の自動 resume（ADR-0012 決めたこと 4・13 の 3。dispatcher.md §2.3）。
// 見回りで doing の課題の端末が Gone（無い・live でない・素のシェル）なら andon issue resume <id> を1回打つ。
// 1つの死につき1回: 控えの Resumed に「死んだのを見た端末」と「起動した端末」を持ち、起動した端末が働かないまま
// （turn 0 のまま）また Gone なら打たずに finished を知らせる（人の go へ）。働き始めたら（turn ≥ 1）控えを消し、
// 次に死んだらまた1回打つ。初めて見る台帳では打たない（控えに「見た」とだけ持つ）。
// resume・finish・start は同じ lock（andon issue の <slug>.issue.lock）を命令の間ずっと持つので、resume の間 finish は待つ。

// Resumed は自動 resume の控えの1件（課題 id → これ）。
type Resumed struct {
	From  string `json:"from"`            // Gone を見た端末（そのときの台帳の agent）。この端末にはもう打たない
	Agent string `json:"agent,omitempty"` // resume で起動した端末。空は打たなかった（初めて見た台帳）か、起動できなかった
}

// ResumeGone は Gone の端末 agent を見たとき打つか（prev は控え、had は控えにあるか、first は初めて見る台帳か）。
func ResumeGone(prev Resumed, had bool, agent string, first bool) bool {
	switch {
	case first:
		return false
	case had && prev.From == agent:
		return false // この死にはもう打った（起動できなかったも含む）
	case had && prev.Agent == agent:
		return false // 起動した端末が働かないまま死んだ
	}
	return true
}

// resumeGone は Gone の担当 agent の課題 t に resume を打つ（ResumeGone が打つと言うとき）。
// 打って起動できたら記録だけの通知 resumed を書いて true（finished を知らせない）。
// 打たない・起動できなかったら false（呼び手が今どおり finished を知らせる）。
func (p *project) resumeGone(t ledger.Issue, agent string) bool {
	id := t.Str("id")
	prev, had := p.st.Resumed[id]
	if !ResumeGone(prev, had, agent, p.first) {
		if p.first && !had {
			p.st.Resumed[id] = Resumed{From: agent} // 初めて見た死には打たない。次の見回りでも打たない
		}
		return false
	}
	// 打つ前に控えへ書いて保存する（打っている間に dispatcher が落ちても、再開後に同じ死へ二度打たない）
	p.st.Resumed[id] = Resumed{From: agent}
	if err := p.st.Save(p.l.Slug); err != nil {
		if had {
			p.st.Resumed[id] = prev
		} else {
			delete(p.st.Resumed, id)
		}
		p.logf("%s の担当 %s は Gone。resume を打たない（控えを書けない。次の見回りで）: %v", id, agent, err)
		return true // この回は知らせない（次の見回りで打つか知らせる）
	}
	if _, _, err := p.andon("", "issue", "resume", id); err != nil {
		p.logf("%s の担当 %s は Gone。resume で起動できない（finished で知らせる）: %v", id, agent, err)
		return false
	}
	now, err := p.l.Show(id)
	next := ""
	if err == nil {
		next = now.Str("agent")
	}
	if next == "" || next == agent {
		p.logf("%s を resume したが、台帳の agent が読めない・替わっていない（finished で知らせる）: %v", id, err)
		return false
	}
	p.st.Resumed[id] = Resumed{From: agent, Agent: next}
	p.notify(id, "resumed", map[string]any{"agent": next, "from": agent})
	// 起動した端末もその場で保存する（巡の最後の保存の前に落ちて、起動した端末が働かずに死んでも二度打たない）
	if err := p.st.Save(p.l.Slug); err != nil {
		p.logf("%s の resume の控えを保存できない（巡の最後にもう一度保存する）: %v", id, err)
	}
	p.logf("%s の担当 %s は Gone。resume して %s で起動した", id, agent, next)
	return true
}

// worked は resume で起動した端末が働き始めた（turn ≥ 1）課題 id の控えを消し、その場で保存する
// （巡の最後の保存の前に dispatcher が落ちても〈起き直しでは担当と一緒に落ちる〉、次に死んだときまた1回打てるように）。
// 保存できなければ控えを戻し、次の見回りでもう一度消す。
func (p *project) worked(id string) {
	prev := p.st.Resumed[id]
	delete(p.st.Resumed, id)
	if err := p.st.Save(p.l.Slug); err != nil {
		p.st.Resumed[id] = prev
		p.logf("%s の resume の控えを消せない（次の見回りで）: %v", id, err)
	}
}

// keepResumed は見回りの終わりに、控えの Resumed から doing でなくなった課題の分を捨てる。
func (p *project) keepResumed() {
	for id := range p.st.Resumed {
		if p.status(id) != "doing" {
			delete(p.st.Resumed, id)
		}
	}
}
