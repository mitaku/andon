package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// WaitUsage は andon dispatcher wait の使い方。
const WaitUsage = "usage: andon dispatcher wait [--timeout <秒>] [--project <slug>]\n" +
	"Chief of Staff の Stop フックが asyncRewake で起動する待ち役（package/hooks/stop-wait。contract.md §9。ADR-0019 決めたこと 1・8・9）。\n" +
	"届ける通知が待ちにあり、Chief of Staff の端末が idle か done になったら、合図の1行を stderr に書いて終了コード 2 で終わる\n" +
	"（Claude Code が入力欄を通らずに新しいターンを始め、その終わりの Stop で drain が <news> を渡す）。待ちと控えは読むだけ。\n" +
	"--timeout はフックの timeout（秒）。その 9 割まで打つ機会が無ければ、張り直しの合図で終了コード 2 で終わる（次の Stop で次の待ち役が起きる）。\n" +
	"端末が一時に読めなければ読み直す。timeout の 95% まで読めなければ、殺される前に終了コード 0 で終わる。\n" +
	"待ち役は Chief of Staff に1つ: 後から起きた待ち役が替わり、前の待ち役は終了コード 0 で終わる。止められたときも 0。\n" +
	"Chief of Staff の端末（ラベル role=chief-of-staff）でない・落ちたとき（台帳・控えが読めない）は、待たずに終了コード 0。\n"

// 合図の文（stderr。Claude Code が system reminder でモデルに見せる）。中身は運ばない（ADR-0019 決めたこと 8）。
const (
	WaitSignal = "andon: 通知が待っている。これは合図で、人の依頼ではない。何も書かずにターンを終える（通知はターンの終わりに <news> で届く）。"
	WaitRearm  = "andon: 待ち役の張り直し。これは合図で、人の依頼ではない。何も書かずにターンを終える。"
)

// waitPoll は待ち役が控えと端末を読み直す間。waitUnit は --timeout の1単位（テストで縮める）。
var (
	waitPoll = time.Second
	waitUnit = time.Second
)

// waiter は待ち役の届け（<slug>.json.wait）。後から起きた待ち役が書き替え、前の待ち役は自分の印でなくなったら終わる。
type waiter struct {
	Terminal string `json:"terminal"`
	PID      int    `json:"pid"`
	Token    string `json:"token"`
}

func waitPath(slug string) string { return StatePath(slug) + ".wait" }

func readWaiter(slug string) (*waiter, error) {
	b, err := os.ReadFile(waitPath(slug))
	if err != nil {
		return nil, err
	}
	var w waiter
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// liveWaiter は、端末 addr の待ち役が生きているか（届けがあり、その pid のプロセスがある）。dispatcher のログのためだけに読む。
func liveWaiter(slug, addr string) bool {
	w, err := readWaiter(slug)
	if err != nil || w.Terminal != addr || w.PID <= 0 {
		return false
	}
	err = syscall.Kill(w.PID, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// WaitMain は andon dispatcher wait。終了コードは 0（通す・替わった・止められた）か 2（合図）だけ。
func WaitMain(args []string, stdin io.Reader, stderr io.Writer) int {
	slug, timeout := "", 0
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--project" && i+1 < len(args):
			slug = args[i+1]
			i++
		case args[i] == "--timeout" && i+1 < len(args):
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				fmt.Fprint(stderr, WaitUsage)
				return 0
			}
			timeout = n
			i++
		default:
			fmt.Fprint(stderr, WaitUsage) // 0 で通す（フックの終了コード 2 は合図の意味なので、使い方の誤りに使わない）
			return 0
		}
	}
	if stdin != nil {
		go io.Copy(io.Discard, stdin) // フックの入力は読まない（書き手を待たせない）
	}
	addr := os.Getenv("HC_TERMINAL_ID")
	if addr == "" {
		return 0
	}
	self, err := hcGet(addr)
	if err != nil || self.Labels["role"] != "chief-of-staff" || self.Labels["project"] == "" {
		return 0 // Chief of Staff の端末ではない（managed settings のフックは担当の端末でも走る）
	}
	if slug == "" {
		slug = self.Labels["project"]
	}
	if !slugRe.MatchString(slug) {
		return 0
	}
	var deadline, last time.Time
	if timeout > 0 {
		now, t := time.Now(), time.Duration(timeout)*waitUnit
		deadline, last = now.Add(t*9/10), now.Add(t*19/20)
	}
	return wait(slug, addr, deadline, last, stderr)
}

// wait の deadline は張り直しの時刻、last は端末が読めないまま諦める時刻（フックの timeout より前）。
func wait(slug, addr string, deadline, last time.Time, stderr io.Writer) int {
	me := waiter{Terminal: addr, PID: os.Getpid(), Token: fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())}
	b, _ := json.Marshal(me)
	path := waitPath(slug)
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path+".tmp."+strconv.Itoa(me.PID), b, 0o644) != nil {
		return 0
	}
	if os.Rename(path+".tmp."+strconv.Itoa(me.PID), path) != nil {
		return 0
	}
	mine := func() bool {
		w, err := readWaiter(slug)
		return err == nil && w.Token == me.Token
	}
	leave := func() {
		if mine() {
			os.Remove(path)
		}
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(sig)
	tick := time.NewTicker(waitPoll)
	defer tick.Stop()
	for {
		if !mine() {
			return 0 // 後から起きた待ち役に替わった
		}
		st, _, err := LoadState(slug)
		if err != nil {
			leave()
			return 0 // 控えが読めない: 通す（待ちは控えに残り、①②で届く）
		}
		now := time.Now()
		late := !deadline.IsZero() && !now.Before(deadline)
		if len(st.Pending) > 0 || late {
			c, err := hcGet(addr)
			waiting := err == nil && (c.Status.State == "idle" || c.Status.State == "done")
			switch {
			case err != nil && !last.IsZero() && !now.Before(last):
				// 張り直しの時刻を過ぎても last まで読めなかった（フックの timeout で殺される前に通す）
				leave()
				return 0
			case err != nil:
				// 読めなければ次の回に読み直す（一時の失敗で待ち役をなくさない。張り直しの時刻を過ぎていても last までは）
			case waiting && len(st.Pending) > 0:
				leave()
				fmt.Fprintln(stderr, WaitSignal)
				return 2
			case waiting && late:
				leave()
				fmt.Fprintln(stderr, WaitRearm)
				return 2
			case late:
				// 働いている・止まっている間は張り直さない（そのターンの終わりの Stop で次の待ち役が起きる）
				leave()
				return 0
			}
		}
		select {
		case <-sig:
			leave()
			return 0
		case <-tick.C:
		}
	}
}
