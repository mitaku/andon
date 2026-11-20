// Package sessionlog はハーネスのセッションの記録（Claude Code のセッション・Codex の rollout）を数える。
//
// 読み手は andon measure（課題ごとのトークン。docs/decisions/2026-11-12-measure-tokens.md）と dispatcher の結果の記録
// （セッションごとのコスト。docs/contract.md §19）。トークンの数え方はここの1つだけで、両方が同じ数を出す。
// 読むだけで、記録には書かない。
package sessionlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
)

// Usage は入力・出力・キャッシュの読み書き（ドルに直さない）。
type Usage struct {
	Input      int64 `json:"input"`       // キャッシュを読み書きしなかった入力
	Output     int64 `json:"output"`      // 出力（Codex の推論の出力を含む）
	CacheRead  int64 `json:"cache_read"`  // キャッシュから読んだ入力
	CacheWrite int64 `json:"cache_write"` // キャッシュに書いた入力
}

// Add は v を足す。
func (u *Usage) Add(v Usage) {
	u.Input += v.Input
	u.Output += v.Output
	u.CacheRead += v.CacheRead
	u.CacheWrite += v.CacheWrite
}

// Log は1つのセッションの記録から数えたもの。
type Log struct {
	Models  []string // 使われたモデルの id（出た順に、重ねずに）
	Efforts []string // effort（出た順に、重ねずに）
	Turns   *int     // ターン数。数える印が記録に1つも無ければ nil
	Usage   Usage    // トークン
	Unread  bool     // 読み切れなかった記録のファイルがある（数えた数は足りない。読み手は未計測として扱える）
}

func (l *Log) model(m string) {
	if m != "" && m != "<synthetic>" && !slices.Contains(l.Models, m) {
		l.Models = append(l.Models, m)
	}
}

func (l *Log) effort(e string) {
	if e != "" && !slices.Contains(l.Efforts, e) {
		l.Efforts = append(l.Efforts, e)
	}
}

// Skip は読み飛ばした記録を知らせる（src は記録の置き場からの名前。line は 0 ならファイル全体）。nil なら知らせない。
type Skip func(src string, line int, reason string)

func (s Skip) call(src string, line int, reason string) {
	if s != nil {
		s(src, line, reason)
	}
}

// ClaudeDir は Claude Code の設定の置き場（CLAUDE_CONFIG_DIR。無ければ ~/.claude）。
func ClaudeDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.Getenv("HOME"), ".claude")
}

// CodexSessions は Codex の記録の置き場（CODEX_HOME。無ければ ~/.codex）/sessions。
func CodexSessions() string {
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		home = filepath.Join(os.Getenv("HOME"), ".codex")
	}
	return filepath.Join(home, "sessions")
}

// Claude は Claude Code のセッションの記録（<claude>/projects/*/<session>.jsonl と <session>/subagents/*.jsonl）を数える。
//   - トークン: assistant の行の message.usage を足す。content block ごとに同じ usage の行が並ぶので、requestId（無ければ message.id）
//     ごとに1回だけ数える。どちらも無い行は数えない。
//   - モデルと effort: 数えた assistant の行の message.model と effort（サブエージェントのものも。<synthetic> は数えない）。
//   - ターン数: 本線（サブエージェントでない記録の、isSidechain でない user の行）の promptId の数。
//
// 記録が1つも無ければ nil（0 と分ける）。読み切れなかったファイルか、一覧できなかったサブエージェントの置き場があれば Unread。
func Claude(dir, session string, skip Skip) *Log {
	if dir == "" || session == "" || strings.ContainsAny(session, `/\*?[`) {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(dir, "projects", "*", session+".jsonl"))
	if len(files) == 0 {
		return nil
	}
	out := &Log{}
	for _, f := range append([]string{}, files...) {
		sub, ok := subagents(filepath.Join(strings.TrimSuffix(f, ".jsonl"), "subagents"), "claude/"+session+"/subagents", skip)
		files = append(files, sub...)
		if !ok {
			out.Unread = true
		}
	}
	sort.Strings(files)
	seen := map[string]bool{}
	prompts := map[string]bool{}
	for _, f := range files {
		src := "claude/" + session + "/" + filepath.Base(f)
		main := filepath.Base(f) == session+".jsonl"
		if main {
			src = "claude/" + session + ".jsonl"
		}
		ok := lines(f, src, skip, func(line int, b []byte) {
			s := string(b)
			if !strings.Contains(s, `"usage"`) && (!main || !strings.Contains(s, `"promptId"`)) {
				return
			}
			var v struct {
				Type        string `json:"type"`
				RequestID   string `json:"requestId"`
				Effort      string `json:"effort"`
				PromptID    string `json:"promptId"`
				IsSidechain bool   `json:"isSidechain"`
				Message     struct {
					ID    string `json:"id"`
					Model string `json:"model"`
					Usage *struct {
						Input      int64 `json:"input_tokens"`
						Output     int64 `json:"output_tokens"`
						CacheRead  int64 `json:"cache_read_input_tokens"`
						CacheWrite int64 `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(b, &v) != nil {
				skip.call(src, line, "JSON の object でない")
				return
			}
			if v.Type == "user" && main && !v.IsSidechain && v.PromptID != "" {
				prompts[v.PromptID] = true
				return
			}
			u := v.Message.Usage
			if v.Type != "assistant" || u == nil {
				return
			}
			key := v.RequestID
			if key == "" {
				key = v.Message.ID
			}
			if key == "" || seen[key] {
				return // 同じ要求の続きの行（か、要求に紐づかない行）
			}
			seen[key] = true
			out.model(v.Message.Model)
			out.effort(v.Effort)
			out.Usage.Add(Usage{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite})
		})
		if !ok {
			out.Unread = true
		}
	}
	if len(prompts) > 0 {
		n := len(prompts)
		out.Turns = &n
	}
	return out
}

// subagents はサブエージェントの記録の置き場 dir の *.jsonl。置き場が無ければ空。一覧できなければ飛ばした記録にして false
// （filepath.Glob は一覧のエラーを捨てるので、読めない置き場の記録を数えずに済ませてしまう）。
func subagents(dir, src string, skip Skip) ([]string, bool) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, true
	}
	if err != nil {
		skip.call(src, 0, "読めない")
		return nil, false
	}
	var out []string
	for _, e := range entries {
		if ok, _ := filepath.Match("*.jsonl", e.Name()); ok {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out, true
}

// CodexByCwd は Codex の記録（<root>/**/rollout-*.jsonl）を、先頭行（session_meta）の cwd ごとに分ける。
func CodexByCwd(root string, skip Skip) map[string][]string {
	out := map[string][]string{}
	if root == "" {
		return out
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			skip.call("codex/"+d.Name(), 0, "読めない")
			return nil
		}
		line, _ := bufio.NewReader(f).ReadBytes('\n')
		f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
		var meta struct {
			Type    string `json:"type"`
			Payload struct {
				Cwd string `json:"cwd"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &meta) != nil || meta.Type != "session_meta" || meta.Payload.Cwd == "" {
			return nil
		}
		cwd := filepath.Clean(meta.Payload.Cwd)
		out[cwd] = append(out[cwd], path)
		return nil
	})
	if err != nil {
		skip.call("codex", 0, "記録の置き場を読めない（Codex の課題のトークン数は数えていない）")
	}
	return out
}

// Codex は rollout ごとに最後の token_count の total_token_usage（積み上がった数）を取り、rollout をまたいで足す。
// Codex の input_tokens はキャッシュの読み書きを含むので、それを引いた残りを入力にする。記録が無ければ nil。
func Codex(files []string, skip Skip) *Usage {
	if len(files) == 0 {
		return nil
	}
	files = append([]string{}, files...)
	sort.Strings(files)
	total := &Usage{}
	for _, f := range files {
		_, r := rollout(f, skip)
		total.Add(r.Usage)
	}
	return total
}

// CodexLogs は rollout をセッション（session_meta の id）ごとに数える（ids は id の順）。トークンは Codex と同じく
// rollout ごとの最後の積み上がった数を足す。モデルと effort は turn_context の model と effort、ターン数は turn_context の turn_id の数。
// id の読めない rollout は id を空にして1つにまとめる。
func CodexLogs(files []string, skip Skip) (ids []string, logs map[string]*Log) {
	logs = map[string]*Log{}
	turns := map[string]map[string]bool{}
	files = append([]string{}, files...)
	sort.Strings(files)
	for _, f := range files {
		id, r := rollout(f, skip)
		x := logs[id]
		if x == nil {
			x = &Log{}
			logs[id] = x
			turns[id] = map[string]bool{}
			ids = append(ids, id)
		}
		for _, m := range r.Models {
			x.model(m)
		}
		for _, e := range r.Efforts {
			x.effort(e)
		}
		for _, t := range r.turnIDs {
			turns[id][t] = true
		}
		x.Usage.Add(r.Usage)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if n := len(turns[id]); n > 0 {
			logs[id].Turns = &n
		}
	}
	return ids, logs
}

// one は rollout 1本を数えたもの（turnIDs はターンの id）。
type one struct {
	Log
	turnIDs []string
}

// rollout は rollout 1本の session_meta の id と、最後の token_count・turn_context のモデルと effort とターンの id。
func rollout(f string, skip Skip) (string, one) {
	src := "codex/" + filepath.Base(f)
	var out one
	id := ""
	lines(f, src, skip, func(line int, b []byte) {
		s := string(b)
		if !strings.Contains(s, `"token_count"`) && !strings.Contains(s, `"turn_context"`) && !strings.Contains(s, `"session_meta"`) {
			return
		}
		var v struct {
			Type    string `json:"type"`
			Payload struct {
				Type   string `json:"type"`
				ID     string `json:"id"`
				Model  string `json:"model"`
				Effort string `json:"effort"`
				TurnID string `json:"turn_id"`
				Info   *struct {
					Total *struct {
						Input      int64 `json:"input_tokens"`
						CacheRead  int64 `json:"cached_input_tokens"`
						CacheWrite int64 `json:"cache_write_input_tokens"`
						Output     int64 `json:"output_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if json.Unmarshal(b, &v) != nil {
			skip.call(src, line, "JSON の object でない")
			return
		}
		switch {
		case v.Type == "session_meta":
			if id == "" {
				id = v.Payload.ID
			}
		case v.Type == "turn_context":
			out.model(v.Payload.Model)
			out.effort(v.Payload.Effort)
			if v.Payload.TurnID != "" && !slices.Contains(out.turnIDs, v.Payload.TurnID) {
				out.turnIDs = append(out.turnIDs, v.Payload.TurnID)
			}
		case v.Type == "event_msg" && v.Payload.Type == "token_count" && v.Payload.Info != nil && v.Payload.Info.Total != nil:
			u := v.Payload.Info.Total
			in := u.Input - u.CacheRead - u.CacheWrite
			if in < 0 {
				in = 0
			}
			out.Usage = Usage{Input: in, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite}
		}
	})
	return id, out
}

// lines は記録のファイルを1行ずつ f に渡す（空の行は飛ばす。長い行も読む）。開けない・読み切れなければ飛ばした記録にして false。
func lines(path, src string, skip Skip, f func(line int, b []byte)) bool {
	fh, err := os.Open(path)
	if err != nil {
		skip.call(src, 0, "読めない")
		return false
	}
	defer fh.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	br := bufio.NewReader(fh)
	for n := 1; ; n++ {
		b, err := br.ReadBytes('\n')
		if len(strings.TrimSpace(string(b))) > 0 {
			f(n, b)
		}
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil {
			skip.call(src, 0, "読めない")
			return false
		}
	}
}
