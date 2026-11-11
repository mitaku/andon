package dispatcher

import (
	"context"
	"os/exec"
	"regexp"
	"slices"
	"time"
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

// inputEmpty は入力欄が空か（dispatcher.md §4.4・§4.5）。文字の正は hc read。attach で組み直した入力欄（薄い字も残したもの）が
// hc read の入力欄と一字も違わないときだけ、薄い字を除いて決める。違う・取れない・読めないなら false（送らない）。
// hc read は observe（最大 3 秒）のあとに読む: 文字の確かめを hc send の直前に寄せる（observe の間に人が打ち始めれば、ここで食い違う。§4.6）。
func inputEmpty(addr string) bool {
	raw, err := observe(addr)
	if err != nil {
		return false
	}
	text, err := hcOut("read", addr)
	if err != nil {
		return false
	}
	all, undim, ok := Screen(raw)
	if !ok {
		return false
	}
	box, ok := InputBox(text)
	if !ok {
		return false
	}
	if abox, ok := InputBox(all); !ok || !slices.Equal(box, abox) {
		return false
	}
	return InputState(undim) == "empty"
}

// observe は hc attach --observe（打たない・大きさも変えない）で ANSI 付きの画面を取る。最初の再描画が届いてから 0.25 秒静かになるか、
// 3 秒で止める。静かになっても描き終えたとは限らないので、文字の正は hc read に置く（inputEmpty が突き合わせる）。
func observe(addr string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hc", "attach", "--observe", addr)
	cmd.Env = withoutTerminal()
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	chunks := make(chan []byte)
	go func() {
		defer close(chunks)
		for {
			buf := make([]byte, 65536)
			n, err := out.Read(buf)
			if n > 0 {
				select {
				case chunks <- buf[:n]:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	var raw []byte
	quiet := time.NewTimer(time.Hour)
	defer quiet.Stop()
	for {
		select {
		case b, ok := <-chunks:
			if !ok {
				return raw, nil
			}
			raw = append(raw, b...)
			quiet.Reset(250 * time.Millisecond)
		case <-quiet.C:
			return raw, nil
		case <-ctx.Done():
			return raw, nil
		}
	}
}
