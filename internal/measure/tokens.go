package measure

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"andon/internal/ledger"
)

// Tokens は課題1件のトークン数（docs/decisions/2026-11-12-measure-tokens.md）。
type Tokens struct {
	Issue  string      `json:"issue"`
	Source string      `json:"source"` // 引いた記録: claude（Claude Code のセッション）か codex（Codex の rollout）
	Usage  *TokenUsage `json:"usage"`  // 記録が無ければ null（0 と分ける）
}

// TokenUsage は入力・出力・キャッシュの読み書き（ドルに直さない）。
type TokenUsage struct {
	Input      int64 `json:"input"`       // キャッシュを読み書きしなかった入力
	Output     int64 `json:"output"`      // 出力（Codex の推論の出力を含む）
	CacheRead  int64 `json:"cache_read"`  // キャッシュから読んだ入力
	CacheWrite int64 `json:"cache_write"` // キャッシュに書いた入力
}

func (u *TokenUsage) add(v TokenUsage) {
	u.Input += v.Input
	u.Output += v.Output
	u.CacheRead += v.CacheRead
	u.CacheWrite += v.CacheWrite
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

// tokens は期間に log.jsonl の行がある課題（id の順）のトークン数を、課題の記録の全体から数える。
// Claude は issue.yaml の session の記録（とそのサブエージェントの記録）、Codex は cwd が課題の cwd の rollout 全部。
func (r *reader) tokens(o Options, ids []string, byID map[string]*issue, active map[string]bool) []Tokens {
	out := []Tokens{}
	var codex map[string][]string
	for _, id := range ids {
		if !active[id] {
			continue
		}
		t := byID[id]
		x := Tokens{Issue: id, Source: ledger.Brand(t.profile)}
		if x.Source == "codex" {
			if codex == nil {
				codex = r.codexByCwd(o.CodexSessions)
			}
			var files []string
			for _, d := range t.cwds(r.dir) {
				files = append(files, codex[d]...)
			}
			x.Usage = r.codexUsage(files)
		} else {
			x.Usage = r.claudeUsage(o.ClaudeDir, t.session)
		}
		out = append(out, x)
	}
	return out
}

// cwds は課題の担当が起きうる所: 台帳の issues/<id>/（レビュアー・repo の無い課題）と worktree（repo のある担当）。
// どちらもその課題だけの場所なので、ほかの課題の記録は混ざらない。
func (t *issue) cwds(dir string) []string {
	if a, err := filepath.Abs(dir); err == nil {
		dir = a
	}
	out := []string{filepath.Join(dir, "issues", t.id)}
	if t.worktree != "" && filepath.Clean(t.worktree) != out[0] {
		out = append(out, filepath.Clean(t.worktree))
	}
	return out
}

// claudeUsage は Claude Code のセッションの記録（<claude>/projects/*/<session>.jsonl と <session>/subagents/*.jsonl）の
// assistant の行の message.usage を足す。content block ごとに同じ usage の行が並ぶので、requestId（無ければ message.id）
// ごとに1回だけ数える。記録が1つも無ければ nil。
func (r *reader) claudeUsage(dir, session string) *TokenUsage {
	if dir == "" || session == "" || strings.ContainsAny(session, `/\*?[`) {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(dir, "projects", "*", session+".jsonl"))
	if len(files) == 0 {
		return nil
	}
	for _, f := range append([]string{}, files...) {
		sub, _ := filepath.Glob(filepath.Join(strings.TrimSuffix(f, ".jsonl"), "subagents", "*.jsonl"))
		files = append(files, sub...)
	}
	sort.Strings(files)
	total := &TokenUsage{}
	seen := map[string]bool{}
	for _, f := range files {
		src := "claude/" + session + "/" + filepath.Base(f)
		if filepath.Base(f) == session+".jsonl" {
			src = "claude/" + session + ".jsonl"
		}
		r.lines(f, src, func(line int, b []byte) {
			if !strings.Contains(string(b), `"usage"`) {
				return
			}
			var v struct {
				Type      string `json:"type"`
				RequestID string `json:"requestId"`
				Message   struct {
					ID    string `json:"id"`
					Usage *struct {
						Input      int64 `json:"input_tokens"`
						Output     int64 `json:"output_tokens"`
						CacheRead  int64 `json:"cache_read_input_tokens"`
						CacheWrite int64 `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(b, &v) != nil {
				r.skip(src, line, "JSON の object でない")
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
			total.add(TokenUsage{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite})
		})
	}
	return total
}

// codexByCwd は Codex の記録（sessions/**/rollout-*.jsonl）を、先頭行（session_meta）の cwd ごとに分ける。
func (r *reader) codexByCwd(root string) map[string][]string {
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
			r.skip("codex/"+d.Name(), 0, "読めない")
			return nil
		}
		line, _ := bufio.NewReader(f).ReadBytes('\n')
		f.Close()
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
		r.skip("codex", 0, "記録の置き場を読めない（Codex の課題のトークン数は数えていない）")
	}
	return out
}

// codexUsage は rollout ごとに最後の token_count の total_token_usage（積み上がった数）を取り、rollout をまたいで足す。
// Codex の input_tokens はキャッシュの読み書きを含むので、それを引いた残りを入力にする。記録が無ければ nil。
func (r *reader) codexUsage(files []string) *TokenUsage {
	if len(files) == 0 {
		return nil
	}
	sort.Strings(files)
	total := &TokenUsage{}
	for _, f := range files {
		src := "codex/" + filepath.Base(f)
		var last TokenUsage
		r.lines(f, src, func(line int, b []byte) {
			if !strings.Contains(string(b), `"token_count"`) {
				return
			}
			var v struct {
				Type    string `json:"type"`
				Payload struct {
					Type string `json:"type"`
					Info *struct {
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
				r.skip(src, line, "JSON の object でない")
				return
			}
			if v.Type != "event_msg" || v.Payload.Type != "token_count" || v.Payload.Info == nil || v.Payload.Info.Total == nil {
				return
			}
			u := v.Payload.Info.Total
			in := u.Input - u.CacheRead - u.CacheWrite
			if in < 0 {
				in = 0
			}
			last = TokenUsage{Input: in, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite}
		})
		total.add(last)
	}
	return total
}

// lines は記録のファイルを1行ずつ f に渡す（空の行は飛ばす。長い行も読む）。読めなければ飛ばした記録にする。
func (r *reader) lines(path, src string, f func(line int, b []byte)) {
	fh, err := os.Open(path)
	if err != nil {
		r.skip(src, 0, "読めない")
		return
	}
	defer fh.Close()
	br := bufio.NewReader(fh)
	for n := 1; ; n++ {
		b, err := br.ReadBytes('\n')
		if len(strings.TrimSpace(string(b))) > 0 {
			f(n, b)
		}
		if err != nil {
			return
		}
	}
}

// active は期間に log.jsonl の行がある課題。
func active(rows []logRow, in func(time.Time) bool) map[string]bool {
	m := map[string]bool{}
	for _, row := range rows {
		if in(row.at) {
			m[row.id] = true
		}
	}
	return m
}
