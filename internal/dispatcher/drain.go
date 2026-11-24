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
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"andon/internal/delegate"
)

const DrainUsage = "usage: andon dispatcher drain [--context] [--project <slug>] < フックの入力の JSON\n" +
	"Chief of Staff の Stop・UserPromptSubmit フックが打つ（package/hooks/。contract.md §9）。待ちの通知を全部取り出して <news> に描く。\n" +
	"  既定（Stop）: 待ちがあれば {\"hookSpecificOutput\": {\"hookEventName\": \"Stop\", \"additionalContext\": \"<news>…</news>\"}}（続きのターンになる）。前にフックで渡した <news> の turn に andon news ack があれば news.replied を書く（答えの文は読まない）\n" +
	"    最後の返事（last_assistant_message）にオーナーへはふだんの言葉で言う語（用語集の表）があれば、同じ additionalContext に <owner-words> を添えて言い直させる（1つの turn に1回だけ）\n" +
	"  --context（UserPromptSubmit）: 待ちと委譲の一覧（andon delegate show --json）を additionalContext に\n" +
	"--project を省くと、自分の端末（HC_TERMINAL_ID）のラベル role=chief-of-staff・project=<slug> から引く。Chief of Staff でなければ何も出さない。\n" +
	"Chief of Staff なら、dispatcher の端末（dispatcher）の中身がシェルに替わっていれば片付けて起動し直す（core が起き直したあと）。\n" +
	"落ちたとき（kanban・控えが読めない・lock が取れない）は何も出さずに通す（待ちは次の口で届く）。\n"

var lockWait = 10 * time.Second

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
			return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil //nolint:errcheck // lock は Close でも外れる
		}
		if wait == 0 || !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			f.Close() //nolint:errcheck // 先の失敗を返すので、片付けの失敗は見ない
			return nil, fmt.Errorf("控えの lock が取れない: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type terminal struct {
	State  string `json:"state"`
	Status struct {
		State string `json:"state"`
		Msg   string `json:"msg"` // 状態に添えた文（Claude Code の error の API のエラー。プロバイダに切られたかの見分けにだけ読む）
	} `json:"status"`
	StatusMs   int64             `json:"status_ms"`
	Turn       int               `json:"turn"`
	Labels     map[string]string `json:"labels"`
	Argv       []string          `json:"argv"`
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
		json.Unmarshal(b, &in) //nolint:errcheck // 読めなければ空のまま（turn の照合が緩くなり、news.replied を書かないだけ）
	}
	out, err := drain(slug, context, in)
	if err != nil {
		fmt.Fprintf(stderr, "andon dispatcher drain: %v（通す）\n", err)
	}
	if out != nil {
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.Encode(out) //nolint:errcheck // 出力への書き込み（fmt.Fprintf を設定で除くのと同じ扱い）
	}
	return 0
}

type hookInput struct {
	SessionID            string `json:"session_id"`
	PromptID             string `json:"prompt_id"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

// sameTurn は、Stop の入力 in が、フックで <news> を渡した turn（h）の終わりか。
// prompt_id（と session_id）があれば照合する。Stop で渡したもの（additionalContext の続き）は続きの turn（stop_hook_active）の終わりだけ。
//
// 中断した turn を見分けるのは prompt_id。Claude Code 2.1.330 で測ったところ（-p と対話の両方）:
//   - Stop と UserPromptSubmit の入力のどちらにも prompt_id が来て、同じ turn では同じ値
//   - additionalContext の続きの turn の Stop（stop_hook_active が true）は、元の turn と同じ prompt_id
//   - オーナーが中断した turn には Stop が来ない。次の入力の UserPromptSubmit と Stop は新しい prompt_id
//   - asyncRewake（exit 2）で始まった turn も UserPromptSubmit が走り、新しい prompt_id。その Stop は stop_hook_active が true
//
// 片方にしか prompt_id が無ければ、合わないものとして1回だけ届け直す（取りこぼすより2度届くほうを選ぶ）。
// 両方に無いとき（prompt_id を渡さないバージョン）は端末と経路だけで照らし、中断した turn の通知も確かめ済みにする
// （合わないとすると、フックで渡した通知が毎回2度届く）。
// 測った形は drain_test.go の TestSameTurnClaudeCodeInputs が守る。
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
	if h.PromptID != "" || in.PromptID != "" {
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
			return nil, nil
		}
		slug = self.Labels["project"]
	}
	if !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("プロジェクトの名前が不正: %q", slug)
	}
	note := ""
	if !context {
		ResetRewake(slug)
		note = ownerWordsNote(slug, in)
	}
	EnsureFromHook("andon dispatcher drain")
	unlock, err := lockState(slug, lockWait)
	if err != nil {
		if !context {
			noteStop(slug, addr, self, in)
		}
		return stopOutput(note, ""), err
	}
	defer unlock()
	st, _, err := LoadState(slug)
	if err != nil {
		return stopOutput(note, ""), err
	}
	changed := false
	cleared := false
	if n, ok := takeStop(slug); ok {
		if h := st.Hook; h != nil && reflect.DeepEqual(n.Hook, *h) {
			if n.Replied {
				st.countReplied("ack")
			}
			logDelivered(slug, h, n.Acked)
			st.Hook, changed = nil, true
			cleared = clearAck(addr, self)
		}
	}
	// 前にフックで渡した <news>。渡した turn が Stop で終わったら確かめ済み（ラベル news.ack が渡したときの turn と同じなら news.replied を書く。
	// 答えの文は読まない。ADR-0010 決めたこと 5・6）。
	// 終わりを見ないまま別の turn が来た（オーナーが中断した・フックの出力が捨てられた）なら、まだ届けていない1回目の分だけ届け直す
	var again []Notification
	if h := st.Hook; h != nil {
		if !context && sameTurn(h, addr, in) {
			ok := acked(self, h.Turn, h.AckBefore)
			if ok && label(addr, self.Turn) == nil {
				st.countReplied("ack")
			}
			logDelivered(slug, h, ok)
			cleared = clearAck(addr, self)
		} else {
			again = h.News
		}
		st.Hook, changed = nil, true
	}
	ns := append([]Notification{}, again...)
	retry := map[string]bool{}
	for _, n := range again {
		retry[eventKey(n)] = true
	}
	for _, n := range st.Pending {
		delete(retry, eventKey(n))
		tmp := State{Pending: ns}
		tmp.Push(n)
		ns = tmp.Pending
	}
	var news string
	if len(ns) > 0 {
		news = RenderAway(slug, awayLine(st.Away), ns) // 戻った最初の <news> に留守の1行（ADR-0017 決めたこと 8 の (c)）
		st.Away = nil
		h := &HookNews{Terminal: addr, Via: "stop", SessionID: in.SessionID, PromptID: in.PromptID}
		if self != nil {
			turn := self.Turn
			h.Turn = &turn
			if !cleared {
				h.AckBefore = self.Labels[AckLabel]
			}
		}
		if context {
			h.Via = "prompt"
		}
		for _, n := range ns {
			if retry[eventKey(n)] {
				h.Again = append(h.Again, n)
			} else {
				h.News = append(h.News, n)
			}
		}
		st.Pending, st.Hook, changed = []Notification{}, h, true
	}
	if changed {
		if err := st.Save(slug); err != nil {
			return stopOutput(note, ""), err
		}
	}
	if !context {
		return stopOutput(note, news), nil
	}
	ctx := news
	var d, dc bytes.Buffer
	if delegate.ShowMain([]string{"--json", "--project", slug}, nil, &d, io.Discard) == 0 && json.Compact(&dc, d.Bytes()) == nil {
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

// stopOutput は Stop フックの出力。<owner-words> と <news> を1つの additionalContext にまとめ、どちらも無ければ nil。
// block でなく additionalContext で返す（続きのターンになるのは同じで、画面が「Stop hook error」にならない。ADR-0019 決めたこと 6）
func stopOutput(note, news string) any {
	ctx := note
	if news != "" {
		if ctx != "" {
			ctx += "\n"
		}
		ctx += news
	}
	if ctx == "" {
		return nil
	}
	return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "Stop", "additionalContext": ctx}}
}

// ownerWords は、オーナーへの文ではふだんの言葉で言う語と、その言い方。
// 正は用語集（package/skills/glossary/SKILL.md）の「オーナーへはふだんの言葉で言う語」の表で、ここはその写し。
// 用語集の行がここにあるかは drain_words_test.go が、ここの行が用語集にあるかは tests/package/08-writing.sh が見る。
// 語の中の半角の空白は、空白が無い・幾つかある書き方にも当てる。英字で終わる語は、英字が続けば当てない（レベル AA は別の語）。
var ownerWords = []struct{ Word, Say string }{
	{"灯り", "オーナーの判断待ち"},
	{"レベル A", "テストが通れば自動でマージ"},
	{"レベル B", "独立レビューを通せば自動でマージ"},
	{"レベル C", "オーナーがマージ"},
	{"地図", "当てたあとに確かめる機能の一覧"},
}

var codeSpan = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")

// ownerWordsNote は Stop の入力の最後の返事から ownerWords の語を探し、あれば言い直させる <owner-words> の文を返す。
// 文を読むのは contract.md §0 の原則 1 の例外（語を探して促すだけで、控えや通知の判定には使わない）。
// 1つの turn（session_id と prompt_id）に1回だけ: 言い直した返事にまだ語が残っても、もう止めない。
// prompt_id が無い入力では、続きの turn（stop_hook_active）で探さない。turn を控えに書けなければ探さない（止め続けないため）。
func ownerWordsNote(slug string, in hookInput) string {
	if in.LastAssistantMessage == "" {
		return ""
	}
	if in.PromptID == "" && in.StopHookActive {
		return ""
	}
	text := codeSpan.ReplaceAllString(in.LastAssistantMessage, " ")
	var found []string
	for _, w := range ownerWords {
		pat := strings.ReplaceAll(regexp.QuoteMeta(w.Word), " ", `[ \t　]*`)
		if c := w.Word[len(w.Word)-1]; c < 0x80 && (c|0x20) >= 'a' && (c|0x20) <= 'z' {
			pat += `([^A-Za-z]|$)`
		}
		re := regexp.MustCompile(pat)
		if re.MatchString(text) {
			found = append(found, fmt.Sprintf("「%s」→「%s」", w.Word, w.Say))
		}
	}
	if len(found) == 0 {
		return ""
	}
	if in.PromptID != "" {
		key := in.SessionID + " " + in.PromptID
		path := StatePath(slug) + ".words"
		if b, err := os.ReadFile(path); err == nil && string(b) == key {
			return ""
		}
		if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path+".tmp", []byte(key), 0o644) != nil || os.Rename(path+".tmp", path) != nil {
			return ""
		}
	}
	return "<owner-words>\n直前の返事に、オーナーへの文ではふだんの言葉で言う語があった: " + strings.Join(found, "、") + "。\n" +
		"オーナーが読む部分を、この言い方で短く言い直す（/opt/andon/writing.md の内輪の語の決まり）。識別子・オーナーの言葉の引用・案件の語（地図アプリの地図など）として使っているだけなら、言い直さずに続ける。\n" +
		"</owner-words>"
}

// stopNote は、控えの lock が取れなかった Stop が、フックで渡した <news>（Hook）の turn の終わりだったことの控え。
// 見回りは lock を巡の終わりまで持つので、続きのターンの Stop が lock を諦めると、渡した印が残って
// 次の送信で届け直していた（ADR-0019 決めたこと 7）。Hook を書くのは drain だけなので、lock の外で読んでよい。
type stopNote struct {
	Hook    HookNews `json:"hook"`
	Acked   bool     `json:"acked"`
	Replied bool     `json:"replied,omitempty"`
}

func stopPath(slug string) string { return StatePath(slug) + ".stop" }

func noteStop(slug, addr string, self *terminal, in hookInput) {
	st, _, err := LoadState(slug)
	if err != nil || st.Hook == nil || !sameTurn(st.Hook, addr, in) {
		return
	}
	n := stopNote{Hook: *st.Hook}
	if acked(self, st.Hook.Turn, st.Hook.AckBefore) {
		n.Acked = true
		n.Replied = label(addr, self.Turn) == nil
	}
	b, err := json.Marshal(n)
	if err != nil {
		return
	}
	path := stopPath(slug)
	if os.WriteFile(path+".tmp", b, 0o644) == nil {
		os.Rename(path+".tmp", path) //nolint:errcheck // 控えは試すだけで、無ければ drain は控え無しで進む
	}
}

func takeStop(slug string) (stopNote, bool) {
	var n stopNote
	path := stopPath(slug)
	if os.Rename(path, path+".taken") != nil {
		return n, false
	}
	b, err := os.ReadFile(path + ".taken")
	os.Remove(path + ".taken") //nolint:errcheck // 取り出したあとの片付けは試すだけ
	if err != nil {
		return n, false
	}
	return n, json.Unmarshal(b, &n) == nil
}

// EnsureFromHook は dispatcher の端末を確かめる（dispatcher.Ensure）。打つのは Chief of Staff のフック（drain）と、
// 担当・レビュアーの Stop フック（andon issue stop-check）。Chief of Staff の留守にも core の起き直しから戻すため
// （ADR-0017 決めたこと 8 の (e)。dispatcher.md §7.2 #1）。落ちてもフックは通す（stderr に1行。who はその頭）。
func EnsureFromHook(who string) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	if did, err := Ensure(self, delegate.Workspace(), 0); errors.Is(err, ErrBusy) {
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "%s: dispatcher の端末を確かめられない: %v\n", who, err)
	} else if did != "" {
		fmt.Fprintf(os.Stderr, "%s: %s\n", who, did)
	}
}

// label は Chief of Staff の端末に news.replied=<turn> を書く（ボードとの約束。dispatcher.md §3.1）。
// clearAck は、確かめ済みの news.ack を消す。消さないと、同じ turn の続きで次の <news> を渡したとき、
// 2回目の ack が前の ack と同じ値になり、数えられない（A-0305）。渡したあとに ack が無ければラベルが無いので数えない。
// 消せたら true（次の ack_before は空）。消せなければ false で、前の値を ack_before に控えて数えない側に倒す。
func clearAck(addr string, self *terminal) bool {
	if self == nil {
		return false
	}
	if _, ok := self.Labels[AckLabel]; !ok {
		return true
	}
	cmd := exec.Command("hc", "label", addr, AckLabel+"=")
	cmd.Env = withoutTerminal()
	return cmd.Run() == nil
}

func label(addr string, turn int) error {
	cmd := exec.Command("hc", "label", addr, "news.replied="+strconv.Itoa(turn))
	cmd.Env = withoutTerminal()
	return cmd.Run()
}
