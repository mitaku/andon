package measure

import (
	"path/filepath"
	"time"

	"andon/internal/ledger"
	"andon/internal/sessionlog"
)

// Tokens は課題1件のトークン数（docs/decisions/2026-11-12-measure-tokens.md）。
type Tokens struct {
	Issue  string      `json:"issue"`
	Source string      `json:"source"` // 引いた記録: claude（Claude Code のセッション）か codex（Codex の rollout）
	Usage  *TokenUsage `json:"usage"`  // 記録が無ければ null（0 と分ける）
}

// TokenUsage は入力・出力・キャッシュの読み書き（ドルに直さない。数え方は sessionlog と同じ）。
type TokenUsage = sessionlog.Usage

// ClaudeDir は Claude Code の設定の置き場（CLAUDE_CONFIG_DIR。無ければ ~/.claude）。
func ClaudeDir() string { return sessionlog.ClaudeDir() }

// CodexSessions は Codex の記録の置き場（CODEX_HOME。無ければ ~/.codex）/sessions。
func CodexSessions() string { return sessionlog.CodexSessions() }

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

// claudeUsage は Claude Code のセッションの記録のトークン（sessionlog.Claude）。記録が1つも無ければ nil。
func (r *reader) claudeUsage(dir, session string) *TokenUsage {
	l := sessionlog.Claude(dir, session, r.skipped1)
	if l == nil {
		return nil
	}
	return &l.Usage
}

// codexByCwd は Codex の記録を、先頭行（session_meta）の cwd ごとに分ける（sessionlog.CodexByCwd）。
func (r *reader) codexByCwd(root string) map[string][]string {
	return sessionlog.CodexByCwd(root, r.skipped1)
}

// codexUsage は rollout ごとに最後の積み上がった数を足す（sessionlog.Codex）。記録が無ければ nil。
func (r *reader) codexUsage(files []string) *TokenUsage {
	return sessionlog.Codex(files, r.skipped1)
}

// skipped1 は sessionlog が読み飛ばした記録を、飛ばした記録に足す。
func (r *reader) skipped1(src string, line int, reason string) {
	r.skip(src, line, "%s", reason)
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
