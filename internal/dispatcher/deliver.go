package dispatcher

import (
	"os/exec"
	"regexp"
)

var slugRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,60}$`)

// unwaited は経路③の見張り（dispatcher.md §3・§4。ADR-0019 決めたこと 1・3）: dispatcher は Chief of Staff の入力欄へ <news> を打たない。
// 離席中に Chief of Staff を起こすのは待ち役（andon dispatcher wait。wait.go）。届ける待ちがあり、Chief of Staff が入力を待っているのに
// 待ち役がいなければ、ログに1行残すだけ（同じ待ちには1回。人が打つか次のターンの終わりに①②で届く）。控えの lock を持って呼ぶ。
func (p *project) unwaited(coord string) {
	key := p.l.Slug
	if len(p.st.Pending) == 0 {
		delete(p.c.unwaited, key)
		return
	}
	c, err := hcGet(coord)
	if err != nil || (c.Status.State != "idle" && c.Status.State != "done") || liveWaiter(p.l.Slug, coord) {
		return
	}
	mark := coord + "\x00" + p.st.Pending[0].At + "\x00" + p.st.Pending[0].Issue + "\x00" + p.st.Pending[0].Kind
	if p.c.unwaited[key] == mark {
		return
	}
	if p.c.unwaited == nil {
		p.c.unwaited = map[string]string{}
	}
	p.c.unwaited[key] = mark
	p.logf("%s に届ける待ちが %d 件あるが、待ち役がいない（人が打つか、次のターンの終わりに届く）", coord, len(p.st.Pending))
}

func hcOut(args ...string) (string, error) {
	cmd := exec.Command("hc", args...)
	cmd.Env = withoutTerminal()
	out, err := cmd.Output()
	return string(out), err
}
