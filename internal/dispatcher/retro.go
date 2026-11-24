package dispatcher

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"andon/internal/delegate"
	"andon/internal/retro"
)

const retroRoutine = "retro"

// retroEvery は件数のきっかけの既定（project.json の chief_of_staff.retro_every。ADR-0039 決めたこと 8）。
const retroEvery = 25

// retroDue は件数のきっかけ（1巡の 3a。dispatcher.md §2.1.1）。前の振り返りが集めたあとに片付いた担当の課題
// （次の振り返りが全文で読む束。数え方は retro.Fresh）が設定の件数以上なら、kanban routine run retro を1回打つ。
// ルーチン retro がkanban に無い・読めない・paused のとき、または振り返りの課題が片付かずに残っているときは、数えも撃ちもしない
// （kanban は撃ち損ねのたびに履歴へ1行を書くので、巡ごとに打つと履歴が埋まる）。失敗はログに1行（同じ理由は1回）で、巡は止めない。
// 撃ったら true を返す（同じ巡の空きの段は、振り返りを撃たない。ADR-0047 決めたこと 5）。
func (p *project) retroDue() bool {
	if !routineOn(p.l.Dir, retroRoutine) || p.routineOpen(retroRoutine) {
		return false
	}
	n := retro.Fresh(p.l.Root, p.tasks, p.rows, time.Now())
	if n == 0 {
		return false
	}
	every, err := retroEveryOf(p.c.WS, p.l.Slug)
	if err != nil {
		p.retroOnce("set", fmt.Sprintf("project.json の chief_of_staff.retro_every を読めない（既定の %d 件で数える）: %v", retroEvery, err))
	} else {
		p.retroOnce("set", "")
	}
	if n < every {
		return false
	}
	out, err := p.l.Run("", "routine", "run", retroRoutine)
	if err != nil {
		p.retroOnce("run", fmt.Sprintf("kanban routine run %s が通らない（巡は続ける）: %v", retroRoutine, err))
		return false
	}
	p.retroOnce("run", "")
	p.logf("片付いた担当の課題が %d 件（設定 %d 件）たまったので、振り返りのルーチンを撃った: %s", n, every, strings.TrimSpace(out))
	return true
}

func (p *project) retroOnce(kind, reason string) {
	key := p.l.Slug + "\x00" + kind
	if p.c.retroLogged[key] == reason {
		return
	}
	if p.c.retroLogged == nil {
		p.c.retroLogged = map[string]string{}
	}
	p.c.retroLogged[key] = reason
	if reason != "" {
		p.logf("%s", reason)
	}
}

func retroEveryOf(ws, slug string) (int, error) {
	pj, err := delegate.ReadProject(ws, slug)
	switch {
	case errors.Is(err, delegate.ErrAbsent):
		return retroEvery, nil
	case err != nil:
		return retroEvery, err
	case pj.ChiefOfStaff == nil || pj.ChiefOfStaff.RetroEvery == nil:
		return retroEvery, nil
	}
	return *pj.ChiefOfStaff.RetroEvery, nil
}
