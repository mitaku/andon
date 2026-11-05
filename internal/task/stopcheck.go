package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"andon/internal/ledger"
)

// maxBlocks は stop-check が続けて止める回数の上限。超えたら通して理由を1行出す（フックが続きのターンを回し続けない）。
const maxBlocks = 3

// stopState は stop-check の控え（~/.local/state/andon/stop-check/<slug>-<id>.json。台帳の外）。
type stopState struct {
	Pass   int `json:"pass"`   // 前回通したときの log.jsonl の行の数
	Blocks int `json:"blocks"` // 続けて止めた回数
}

// stopCheck は担当・レビュアーの Stop フック（contract.md §9）。台帳が doing で、前回の通過（無ければ doing に入った行）以後に
// 報告・review・問いのどれも log.jsonl に無ければ {"decision": "block"}。id を省くと自分の端末のラベル task= から引く。
// 担当の端末でない・台帳が読めないときは何も出さずに通す。
func stopCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 1 || (len(args) == 1 && !idRe.MatchString(args[0])) {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	var in struct {
		StopHookActive bool `json:"stop_hook_active"`
	}
	if b, _ := io.ReadAll(stdin); len(bytes.TrimSpace(b)) > 0 {
		json.Unmarshal(b, &in)
	}
	id := ""
	if len(args) == 1 {
		id = args[0]
	} else if id = ownTask(); id == "" {
		return 0 // 担当・レビュアーの端末ではない
	}
	pass := func(why string) int {
		if why != "" {
			fmt.Fprintf(stderr, "andon task stop-check: %s（通す）\n", why)
		}
		return 0
	}
	l, err := ledger.Open()
	if err != nil {
		return pass(err.Error())
	}
	t, err := l.Show(id)
	if err != nil {
		return pass(err.Error())
	}
	path := stopStatePath(l.Slug, id)
	var st stopState
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &st)
	}
	rows := l.Log()
	if st.Pass > len(rows) { // log.jsonl が短くなった: 控えを捨てる
		st = stopState{}
	}
	save := func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		b, _ := json.Marshal(st)
		return os.WriteFile(path, append(b, '\n'), 0o644)
	}
	if t.Str("status") != "doing" {
		return pass("")
	}
	base := st.Pass
	for i, r := range rows {
		if r.ID == id && r.Change["status"] == "doing" && i+1 > base {
			base = i + 1
		}
	}
	for _, r := range rows[base:] {
		if r.ID == id && returned(r.Change) {
			st = stopState{Pass: len(rows)}
			save()
			return pass("")
		}
	}
	if !in.StopHookActive {
		st.Blocks = 0
	}
	st.Blocks++
	if st.Blocks > maxBlocks {
		st.Blocks = 0
		save()
		return pass(fmt.Sprintf("%s は報告も問いも無いまま %d 回止めた", id, maxBlocks))
	}
	if err := save(); err != nil { // 止めた回数を数えられないなら止めない（止め続けない側に倒す）
		return pass("控えが書けない: " + err.Error())
	}
	reason := fmt.Sprintf("%s は doing のまま、報告も問いも台帳に無い。報告（andon task report。レビューを頼むなら review-request）か"+
		"問い（andon task ask）を書いてから止まる。終わったなら andon task ready まで。", id)
	b, _ := json.Marshal(map[string]string{"decision": "block", "reason": reason})
	fmt.Fprintln(stdout, string(b))
	return 0
}

// returned は log.jsonl の行が、担当が台帳に返した印（報告・review・問い）か。
func returned(c map[string]any) bool {
	if rep := c["report"]; rep == "write" || rep == "append" {
		return true
	}
	if c["status"] == "review" {
		return true
	}
	_, q := c["question"]
	return q && c["status"] == "open"
}

// ownTask は自分の端末（HC_TERMINAL_ID）のラベル task=。読めなければ空。
func ownTask() string {
	addr := os.Getenv("HC_TERMINAL_ID")
	if addr == "" {
		return ""
	}
	out, err := exec.Command("hc", "get", addr, "--json").Output()
	if err != nil {
		return ""
	}
	var t struct {
		Labels map[string]string `json:"labels"`
	}
	if json.Unmarshal(out, &t) != nil || !idRe.MatchString(t.Labels["task"]) {
		return ""
	}
	return t.Labels["task"]
}

func stopStatePath(slug, id string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "stop-check", strings.ReplaceAll(slug, "/", "_")+"-"+id+".json")
}
