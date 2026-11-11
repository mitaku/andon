package dispatcher

import (
	"context"
	"os/exec"
	"regexp"
	"slices"
	"time"
)

var slugRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,60}$`)

// deliver は経路③（dispatcher.md §3・§4）: Chief of Staff がターンを終えて入力を待っていて、人も打っていないときだけ
// 待ちを hc send で届ける。迷ったら送らない（送らなかった通知はフックでも届く）。控えの lock を持って呼ぶ。
func (p *project) deliver(coord string) {
	c, err := hcGet(coord)
	if err != nil {
		return
	}
	status := c.Status.State
	if status != "idle" && status != "done" { // a
		return
	}
	if p.st.Coord != coord { // Chief of Staff が替わったら、前の端末の turn の控えは捨てる
		p.st.Coord, p.st.NewsTurn = coord, nil
	}
	// 前の <news> の答え: turn が進んでいれば、送った turn に andon news ack があるかを見る（1回だけ。画面の答えは読まない）
	if p.st.NewsTurn != nil {
		if c.Turn <= *p.st.NewsTurn { // c: まだ答えている途中
			return
		}
		if acked(c, p.st.NewsTurn, "") {
			if label(coord, c.Turn) == nil {
				p.st.countReplied("ack")
				p.logf("%s の turn %d は ack。news.replied=%d を書いた", coord, c.Turn, c.Turn)
			} else {
				p.logf("%s に news.replied を書けなかった", coord)
			}
		}
		p.st.NewsTurn = nil
	}
	if len(p.st.Pending) == 0 || !inputEmpty(coord) { // b
		return
	}
	msg := Render(p.l.Slug, p.st.Pending)
	// d: 送る直前に 状態 → 入力欄 → 状態と turn の順で読み直す（入力欄を読む間に人が送ってターンが始まると、入力欄は空に見える）
	if c2, err := hcGet(coord); err != nil || c2.Status.State != status {
		return
	}
	if !inputEmpty(coord) {
		return
	}
	if c3, err := hcGet(coord); err != nil || c3.Status.State != status || c3.Turn != c.Turn {
		return
	}
	// done は --if idle に拒まれるので、確かめた直後に --if なしで送る。
	// ここから hc send が打つまでの間に人が打ち始めると、hc send は人の手が止まるまで待ってから、その書きかけに続けて送る
	// （待ちは取り消せない。dispatcher.md §4.4。dispatcher だけでは塞げない）。最後の hc read からここまでを短く保つ
	args := []string{"send", coord, msg}
	if status == "idle" {
		args = []string{"send", "--if", "idle", coord, msg}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hc", args...)
	cmd.Env = withoutTerminal()
	if err := cmd.Run(); err != nil {
		p.logf("%s への hc send に失敗（次の回に送り直す）: %v", coord, err)
		return
	}
	p.logf("%s に <news> を送った（%d 件）", coord, len(p.st.Pending))
	p.st.Pending = []Notification{}
	turn := c.Turn
	p.st.NewsTurn = &turn
}

func hcOut(args ...string) (string, error) {
	cmd := exec.Command("hc", args...)
	cmd.Env = withoutTerminal()
	out, err := cmd.Output()
	return string(out), err
}

// inputEmpty は入力欄が空か（dispatcher.md §4.2・§4.3）。文字の正は hc read。attach で組み直した入力欄（薄い字も残したもの）が
// hc read の入力欄と一字も違わないときだけ、薄い字を除いて決める。違う・取れない・読めないなら false（送らない）。
// hc read は observe（最大 3 秒）のあとに読む: 文字の確かめを hc send の直前に寄せる（observe の間に人が打ち始めれば、ここで食い違う。§4.4）。
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
