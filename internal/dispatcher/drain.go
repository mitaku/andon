package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	"andon/internal/delegate"
)

// DrainUsage は andon dispatcher drain の使い方。
const DrainUsage = "usage: andon dispatcher drain [--context] [--project <slug>] < フックの入力の JSON\n" +
	"Chief of Staff の Stop・UserPromptSubmit フックが打つ（package/hooks/。contract.md §9）。待ちの通知を全部取り出して <news> に描く。\n" +
	"  既定（Stop）: 待ちがあれば {\"hookSpecificOutput\": {\"hookEventName\": \"Stop\", \"additionalContext\": \"<news>…</news>\"}}（続きのターンになる）。前にフックで渡した <news> の turn に andon news ack があれば news.replied を書く（答えの文は読まない）\n" +
	"  --context（UserPromptSubmit）: 待ちと委譲の一覧（andon delegate show --json）を additionalContext に\n" +
	"--project を省くと、自分の端末（HC_TERMINAL_ID）のラベル role=chief-of-staff・project=<slug> から引く。Chief of Staff でなければ何も出さない。\n" +
	"Chief of Staff なら、dispatcher の端末（dispatcher）の中身がシェルに替わっていれば片付けて起こし直す（core が起き直したあと）。\n" +
	"落ちたとき（台帳・控えが読めない・lock が取れない）は何も出さずに通す（待ちは次の口で届く）。\n"

// lockWait は drain が控えの lock を待つ長さ。dispatcher の見回りが持っていれば、待ちは次の口で届く。
var lockWait = 10 * time.Second

// lockState は控えの lock を取る（dispatcher の見回りと drain が控えを読み書きする間）。wait が 0 なら取れるまで待つ。
func lockState(slug string, wait time.Duration) (func(), error) {
	path := StatePath(slug) + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		how := syscall.LOCK_EX
		if wait > 0 {
			how |= syscall.LOCK_NB
		}
		err := syscall.Flock(int(f.Fd()), how)
		if err == nil {
			return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
		}
		if wait == 0 || !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("控えの lock が取れない: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// terminal は hc get --json のうち、dispatcher とフックが読むもの。
type terminal struct {
	State  string `json:"state"`
	Status struct {
		State string `json:"state"`
		Msg   string `json:"msg"` // 状態に添えた文（Claude Code の error の API のエラー。提供元に切られたかの見分けにだけ読む）
	} `json:"status"`
	StatusMs int64             `json:"status_ms"`
	Turn     int               `json:"turn"`
	Labels   map[string]string `json:"labels"`
	// Argv と Foreground は top-level の command と前に出ているもの（作り直しで中身がシェルに替わった端末の見分けに読む）
	Argv       []string `json:"argv"`
	Foreground *struct {
		Argv []string `json:"argv"`
	} `json:"foreground"`
}

func hcGet(addr string) (*terminal, error) {
	cmd := exec.Command("hc", "get", addr, "--json")
	cmd.Env = withoutTerminal()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("hc get %s: %v", addr, err)
	}
	var t terminal
	if err := json.Unmarshal(out, &t); err != nil {
		return nil, fmt.Errorf("hc get %s の形が読めない", addr)
	}
	return &t, nil
}

// withoutTerminal は HC_TERMINAL_ID を除いた環境（hc の子の扱いにしない）。
func withoutTerminal() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "HC_TERMINAL_ID=") {
			env = append(env, e)
		}
	}
	return env
}

// Drain は andon dispatcher drain。
func Drain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	context, slug := false, ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--context":
			context = true
		case args[i] == "--project" && i+1 < len(args):
			slug = args[i+1]
			i++
		default:
			fmt.Fprint(stderr, DrainUsage)
			return exitUsage
		}
	}
	var in hookInput
	if b, _ := io.ReadAll(stdin); len(bytes.TrimSpace(b)) > 0 {
		json.Unmarshal(b, &in) // 読めなければ空のまま（turn の照合が緩くなり、news.replied を書かないだけ）
	}
	out, err := drain(slug, context, in)
	if err != nil {
		fmt.Fprintf(stderr, "andon dispatcher drain: %v（通す）\n", err)
		return 0
	}
	if out != nil {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false) // <news> を \u003c にしない（人が debug のログで読む）
		enc.Encode(out)
	}
	return 0
}

// hookInput は Claude Code のフックの入力のうち drain が読むもの。
type hookInput struct {
	SessionID      string `json:"session_id"`
	PromptID       string `json:"prompt_id"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// sameTurn は、Stop の入力 in が、フックで <news> を渡した turn（h）の終わりか。
// prompt_id（と session_id）があれば照合する。Stop で渡したもの（additionalContext の続き）は続きの turn（stop_hook_active）の終わりだけ。
func sameTurn(h *HookNews, addr string, in hookInput) bool {
	if h.Terminal != addr {
		return false
	}
	if h.Via == "stop" && !in.StopHookActive {
		return false
	}
	if h.SessionID != "" && in.SessionID != "" && h.SessionID != in.SessionID {
		return false
	}
	if h.PromptID != "" && in.PromptID != "" {
		return h.PromptID == in.PromptID
	}
	return true
}

func drain(slug string, context bool, in hookInput) (any, error) {
	addr := os.Getenv("HC_TERMINAL_ID")
	var self *terminal
	if addr != "" {
		self, _ = hcGet(addr)
	}
	if slug == "" {
		if self == nil || self.Labels["role"] != "chief-of-staff" || self.Labels["project"] == "" {
			return nil, nil // Chief of Staff の端末ではない
		}
		slug = self.Labels["project"]
	}
	if !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("プロジェクトの名前が不正: %q", slug)
	}
	if !context {
		ResetRewake(slug) // ターンが通った: 提供元に切られた回数を数え直す（rewake.go）
	}
	ensure() // core が起き直して dispatcher の中身がシェルに替わっていれば戻す（boot を通らない起き直しの分。terminal.go）
	unlock, err := lockState(slug, lockWait)
	if err != nil {
		if !context {
			noteStop(slug, addr, self, in) // 渡した turn の終わりは控えの外に残す（lock を待たない。次の drain が確かめ済みにする）
		}
		return nil, err
	}
	defer unlock()
	st, _, err := LoadState(slug)
	if err != nil {
		return nil, err
	}
	changed := false
	// lock が取れなかった Stop で見た、渡した turn の終わり（noteStop）。同じ印にだけ当てる
	if n, ok := takeStop(slug); ok {
		if h := st.Hook; h != nil && reflect.DeepEqual(n.Hook, *h) {
			if n.Acked {
				st.countReplied("ack")
			}
			logDelivered(slug, h, n.Acked)
			st.Hook, changed = nil, true
		}
	}
	// 前にフックで渡した <news>。渡した turn が Stop で終わったら確かめ済み（ラベル news.ack が渡したときの turn と同じなら news.replied を書く。
	// 答えの文は読まない。ADR-0010 決めたこと 5・6）。
	// 終わりを見ないまま別の turn が来た（人が中断した・フックの出力が捨てられた）なら、まだ届けていない1回目の分だけ届け直す
	var again []Notification
	if h := st.Hook; h != nil {
		if !context && sameTurn(h, addr, in) {
			ok := acked(self, h.Turn, h.AckBefore) && label(addr, self.Turn) == nil
			if ok {
				st.countReplied("ack")
			}
			logDelivered(slug, h, ok)
		} else {
			again = h.News // 届け直しは1回だけ（h.Again は捨てる）
		}
		st.Hook, changed = nil, true
	}
	ns := append([]Notification{}, again...)
	retry := map[string]bool{}
	for _, n := range again {
		retry[n.Issue+"\x00"+n.Kind] = true
	}
	for _, n := range st.Pending {
		delete(retry, n.Issue+"\x00"+n.Kind) // 新しい1件に置き換わったものは1回目として数える
		tmp := State{Pending: ns}
		tmp.Push(n)
		ns = tmp.Pending
	}
	var news string
	if len(ns) > 0 {
		news = Render(slug, ns)
		h := &HookNews{Terminal: addr, Via: "stop", SessionID: in.SessionID, PromptID: in.PromptID}
		if self != nil {
			turn := self.Turn
			h.Turn, h.AckBefore = &turn, self.Labels[AckLabel]
		}
		if context {
			h.Via = "prompt"
		}
		for _, n := range ns {
			if retry[n.Issue+"\x00"+n.Kind] {
				h.Again = append(h.Again, n)
			} else {
				h.News = append(h.News, n)
			}
		}
		st.Pending, st.Hook, changed = []Notification{}, h, true
	}
	if changed {
		// 書けてから渡す（書けなければ通す。待ちは控えに残り、次の口で届く）
		if err := st.Save(slug); err != nil {
			return nil, err
		}
	}
	if !context {
		if news == "" {
			return nil, nil
		}
		// block でなく additionalContext で返す（続きのターンになるのは同じで、画面が「Stop hook error」にならない。ADR-0019 決めたこと 6）
		return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "Stop", "additionalContext": news}}, nil
	}
	ctx := news
	var d, dc bytes.Buffer
	if delegate.Main([]string{"show", "--json", "--project", slug}, nil, &d, io.Discard) == 0 && json.Compact(&dc, d.Bytes()) == nil {
		if ctx != "" {
			ctx += "\n"
		}
		ctx += fmt.Sprintf("<delegation project=\"%s\">\n%s\n</delegation>", slug, dc.String())
	}
	if ctx == "" {
		return nil, nil
	}
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "UserPromptSubmit", "additionalContext": ctx}}, nil
}

// stopNote は、控えの lock が取れなかった Stop が、フックで渡した <news>（Hook）の turn の終わりだったことの控え。
// 見回りは lock を巡の終わりまで持つので、続きのターンの Stop が lock を諦めると、渡した印が残って
// 次の送信で届け直していた（ADR-0019 決めたこと 7）。Hook を書くのは drain だけなので、lock の外で読んでよい。
type stopNote struct {
	Hook  HookNews `json:"hook"`  // 終わりを見た印（次の drain の Hook と同じときだけ確かめ済みにする）
	Acked bool     `json:"acked"` // その turn に andon news ack があった（news.replied はもう書いた。数えるのは次の drain）
}

func stopPath(slug string) string { return StatePath(slug) + ".stop" }

// noteStop は lock の外で控えを読み、Stop の入力 in が渡した turn の終わりなら控えに残す。落ちたら何もしない（届け直しになるだけ）。
func noteStop(slug, addr string, self *terminal, in hookInput) {
	st, _, err := LoadState(slug)
	if err != nil || st.Hook == nil || !sameTurn(st.Hook, addr, in) {
		return
	}
	n := stopNote{Hook: *st.Hook}
	if acked(self, st.Hook.Turn, st.Hook.AckBefore) && label(addr, self.Turn) == nil {
		n.Acked = true
	}
	b, err := json.Marshal(n)
	if err != nil {
		return
	}
	path := stopPath(slug)
	if os.WriteFile(path+".tmp", b, 0o644) == nil {
		os.Rename(path+".tmp", path)
	}
}

// takeStop は noteStop の控えを取り出す（先に名前を替えるので、取り出したあとに書かれた控えは次の drain に残る）。
func takeStop(slug string) (stopNote, bool) {
	var n stopNote
	path := stopPath(slug)
	if os.Rename(path, path+".taken") != nil {
		return n, false
	}
	b, err := os.ReadFile(path + ".taken")
	os.Remove(path + ".taken")
	if err != nil {
		return n, false
	}
	return n, json.Unmarshal(b, &n) == nil
}

// ensure は dispatcher の端末を確かめる（dispatcher.Ensure）。落ちても drain は通す（stderr に1行。フックは捨てる）。
func ensure() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	// フックは人の入力ごとに走るので待たない（前に何も出ていなければ ErrUnsure で次のターンに任せる）
	if did, err := Ensure(self, delegate.Workspace(), 0); errors.Is(err, ErrBusy) {
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "andon dispatcher drain: dispatcher の端末を確かめられない: %v\n", err)
	} else if did != "" {
		fmt.Fprintf(os.Stderr, "andon dispatcher drain: %s\n", did)
	}
}

// label は Chief of Staff の端末に news.replied=<turn> を書く（ボードとの約束。dispatcher.md §3.1）。
func label(addr string, turn int) error {
	cmd := exec.Command("hc", "label", addr, "news.replied="+strconv.Itoa(turn))
	cmd.Env = withoutTerminal()
	return cmd.Run()
}
