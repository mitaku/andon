package dispatcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"andon/internal/delegate"
	"andon/internal/retro"
)

// retroRoutine は振り返りのルーチンの名前（kanban/routines/retro.yaml。issue.yaml の routine）。
const retroRoutine = "retro"

// retroEvery は件数のきっかけの既定（project.json の chief_of_staff.retro_every。ADR-0039 決めたこと 8）。
const retroEvery = 25

// retroDue は件数のきっかけ（1巡の 3a。dispatcher.md §2.1.1）。前の振り返りが集めたあとに片付いた担当の課題
// （次の振り返りが全文で読む束。数え方は retro.Fresh）が設定の件数以上なら、kanban routine run retro を1回打つ。
// ルーチン retro がkanban に無い・読めない・paused のとき、または振り返りの課題が片付かずに残っているときは、数えも撃ちもしない
// （kanban は撃ち損ねのたびに履歴へ1行を書くので、巡ごとに打つと履歴が埋まる）。失敗はログに1行（同じ理由は1回）で、巡は止めない。
func (p *project) retroDue() {
	if !retroOn(p.l.Dir) {
		return
	}
	for _, t := range p.tasks {
		if t.Str("routine") == retroRoutine && t.Str("status") != "done" {
			return
		}
	}
	n := retro.Fresh(p.l.Root, p.tasks, p.rows, time.Now())
	if n == 0 {
		return
	}
	every, err := retroEveryOf(p.c.WS, p.l.Slug)
	if err != nil {
		p.retroOnce("set", fmt.Sprintf("project.json の chief_of_staff.retro_every を読めない（既定の %d 件で数える）: %v", retroEvery, err))
	}
	if n < every {
		return
	}
	out, err := p.l.Run("", "routine", "run", retroRoutine)
	if err != nil {
		p.retroOnce("run", fmt.Sprintf("kanban routine run %s が通らない（巡は続ける）: %v", retroRoutine, err))
		return
	}
	p.retroOnce("run", "")
	p.logf("片付いた担当の課題が %d 件（設定 %d 件）たまったので、振り返りのルーチンを撃った: %s", n, every, strings.TrimSpace(out))
}

// retroOnce は件数のきっかけの失敗をログに1行だけ書く（kanban と種類ごとに、同じ理由が続く間は書かない）。reason が空なら覚えを消す。
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

// retroOn は kanban にルーチン retro があり、止める印（paused: true）が無いか。読めない・paused が真偽でないファイルは、
// kanban の CLI も撃たない（kanban lint の problem）ので無いと同じに扱う。
func retroOn(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "routines", retroRoutine+".yaml"))
	if err != nil {
		return false
	}
	var r map[string]any
	if yaml.Unmarshal(b, &r) != nil || r == nil {
		return false
	}
	v, ok := r["paused"]
	if !ok {
		return true
	}
	paused, isBool := v.(bool)
	return isBool && !paused
}

// retroEveryOf は件数を、セルの repo の HEAD の project.json の chief_of_staff.retro_every から読む（author 照合は
// delegate.ReadProject）。project.json か値が無ければ既定。信じられない・形が違うときは既定と、その理由を返す。
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
