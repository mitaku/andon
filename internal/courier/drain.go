package courier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"andon/internal/delegate"
)

// DrainUsage は andon courier drain の使い方。
const DrainUsage = "usage: andon courier drain [--context] [--project <slug>] < フックの入力の JSON\n" +
	"Chief of Staff の Stop・UserPromptSubmit フックが打つ（package/hooks/。contract.md §9）。待ちの通知を全部取り出して <news> に描く。\n" +
	"  既定（Stop）: 待ちがあれば {\"decision\": \"block\", \"reason\": \"<news>…</news>\"}。前にフックで渡した <news> の答えが <no-reply> だけなら news.replied を書く\n" +
	"  --context（UserPromptSubmit）: 待ちと委譲の一覧（andon delegate show --json）を additionalContext に\n" +
	"--project を省くと、自分の端末（HC_TERMINAL_ID）のラベル role=chief-of-staff・project=<slug> から引く。Chief of Staff でなければ何も出さない。\n" +
	"Chief of Staff なら、クーリエの端末（courier）の中身がシェルに替わっていれば片付けて起こし直す（core が起き直したあと）。\n" +
	"落ちたとき（台帳・控えが読めない・lock が取れない）は何も出さずに通す（待ちは次の口で届く）。\n"

// lockWait は drain が控えの lock を待つ長さ。クーリエの見回りが持っていれば、待ちは次の口で届く。
var lockWait = 10 * time.Second

// lockState は控えの lock を取る（クーリエの見回りと drain が控えを読み書きする間）。wait が 0 なら取れるまで待つ。
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

// terminal は hc get --json のうち、クーリエとフックが読むもの。
type terminal struct {
	State  string `json:"state"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	StatusMs int64             `json:"status_ms"`
	Turn     int               `json:"turn"`
	Labels   map[string]string `json:"labels"`
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

// Drain は andon courier drain。
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
		fmt.Fprintf(stderr, "andon courier drain: %v（通す）\n", err)
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
	SessionID            string `json:"session_id"`
	PromptID             string `json:"prompt_id"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

// sameTurn は、Stop の入力 in が、フックで <news> を渡した turn（h）の終わりか。
// prompt_id（と session_id）があれば照合する。Stop の block で渡したものは続きの turn（stop_hook_active）の終わりだけ。
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
	ensure() // core が起き直してクーリエの中身がシェルに替わっていれば戻す（boot を通らない起き直しの分。terminal.go）
	unlock, err := lockState(slug, lockWait)
	if err != nil {
		return nil, err
	}
	defer unlock()
	st, _, err := LoadState(slug)
	if err != nil {
		return nil, err
	}
	changed := false
	// 前にフックで渡した <news>。渡した turn が Stop で終わったら確かめ済み（答えが <no-reply> だけなら news.replied を書く）。
	// 終わりを見ないまま別の turn が来た（人が中断した・フックの出力が捨てられた）なら、まだ届けていない1回目の分だけ届け直す
	var again []Notification
	if h := st.Hook; h != nil {
		if !context && sameTurn(h, addr, in) {
			if strings.TrimSpace(in.LastAssistantMessage) == "<no-reply>" && self != nil {
				label(addr, self.Turn)
			}
		} else {
			again = h.News // 届け直しは1回だけ（h.Again は捨てる）
		}
		st.Hook, changed = nil, true
	}
	ns := append([]Notification{}, again...)
	retry := map[string]bool{}
	for _, n := range again {
		retry[n.Task+"\x00"+n.Kind] = true
	}
	for _, n := range st.Pending {
		delete(retry, n.Task+"\x00"+n.Kind) // 新しい1件に置き換わったものは1回目として数える
		tmp := State{Pending: ns}
		tmp.Push(n)
		ns = tmp.Pending
	}
	var news string
	if len(ns) > 0 {
		news = Render(slug, ns)
		h := &HookNews{Terminal: addr, Via: "stop", SessionID: in.SessionID, PromptID: in.PromptID}
		if context {
			h.Via = "prompt"
		}
		for _, n := range ns {
			if retry[n.Task+"\x00"+n.Kind] {
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
		return map[string]any{"decision": "block", "reason": news}, nil
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

// ensure はクーリエの端末を確かめる（courier.Ensure）。落ちても drain は通す（stderr に1行。フックは捨てる）。
func ensure() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	if did, err := Ensure(self, delegate.Workspace()); err != nil {
		fmt.Fprintf(os.Stderr, "andon courier drain: クーリエの端末を確かめられない: %v\n", err)
	} else if did != "" {
		fmt.Fprintf(os.Stderr, "andon courier drain: %s\n", did)
	}
}

// label は Chief of Staff の端末に news.replied=<turn> を書く（ボードとの約束。courier.md §3.1）。
func label(addr string, turn int) error {
	cmd := exec.Command("hc", "label", addr, "news.replied="+strconv.Itoa(turn))
	cmd.Env = withoutTerminal()
	return cmd.Run()
}
