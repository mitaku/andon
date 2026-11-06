package boot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"andon/internal/ledger"
	"andon/internal/spawn"
)

// 信頼の画面を建屋では出さない（建屋＝コンテナが信頼の境界。人の決定 2026-11-06）。
// boot が、エージェントが起きる場所の根を Claude Code と Codex の両方で信頼済みにしておく。足りない所だけ足し、
// 今ある設定は触らない（2回目は何も書かない）。
//
// 根は「建屋の repo」と「台帳に載った案件の repo（建屋の projects/*/.flightdeck/config.yaml の repos にあり、
// <GhqRoot>/<host>/<owner>/<repo> に clone されたもの）」だけ。ghq の下の repo を全部は信頼しない（台帳に載せるのは
// Chief of Staff と人で、andon task start が起こしてよいのもその repo だけ）。担当・レビュアーの worktree は
// 置き場（~/.herdr/worktrees）ではなく元の repo の根で判定されるので、置き場は足さない（実測は docs/image.md）:
//   - Claude Code 2.1.318: git の外は cwd から親へ / まで遡る。git の中は repo の根で止まり、それより上の親は効かない。
//     worktree は元の repo の根の信頼が効く（worktree の置き場の親は効かない）。
//   - Codex 0.166.0: git の外では信頼の画面を出さない。git の中は repo の根で判定し、worktree は元の repo の根で判定する
//     （置き場の親は効かない。worktree そのものの根でも効く）。

// Trust は信頼の根を Claude Code と Codex の両方で信頼済みにする（足りない所だけ足す。boot の生成の2つと同じ手）。
func Trust(e Env) error {
	return errors.Join(trustClaude(e), trustCodex(e))
}

// TrustFromOS は環境変数の Env で Trust する。andon task start・andon chief-engineer が起こす直前に打つ
// （boot のあとに ghq get で迎えた repo の根も足すため。docs/operate.md の 7）。ログは log へ。
func TrustFromOS(log io.Writer) error {
	e := EnvFromOS(log)
	if e.Home == "" {
		return errors.New("HOME が無い")
	}
	return Trust(e)
}

// trustRoots は信頼を置く根（実在するものを、symlink を解いた絶対パスで）。順は建屋の repo、台帳の名前順と repos の順。
func (e Env) trustRoots() []string {
	cands := []string{e.Workspace}
	if e.GhqRoot != "" && e.Workspace != "" {
		cfgs, _ := filepath.Glob(filepath.Join(e.Workspace, "projects", "*", ".flightdeck", "config.yaml"))
		sort.Strings(cfgs)
		for _, cfg := range cfgs {
			for _, repo := range ledger.At(filepath.Dir(filepath.Dir(cfg))).Repos() {
				dir := filepath.Join(e.GhqRoot, repo)
				if _, err := os.Lstat(filepath.Join(dir, ".git")); spawn.RepoRe.MatchString(repo) && err == nil {
					cands = append(cands, dir)
				}
			}
		}
	}
	var roots []string
	seen := map[string]bool{}
	for _, c := range cands {
		if c == "" {
			continue
		}
		p, err := filepath.EvalSymlinks(c)
		if err != nil {
			continue
		}
		if st, err := os.Stat(p); err != nil || !st.IsDir() || seen[p] {
			continue
		}
		seen[p] = true
		roots = append(roots, p)
	}
	return roots
}

// claudeJSON は Claude Code の .claude.json（CLAUDE_CONFIG_DIR があればその下、無ければ ~/.claude.json）。
func (e Env) claudeJSON() string {
	if e.ClaudeDir != "" {
		return filepath.Join(e.ClaudeDir, ".claude.json")
	}
	return filepath.Join(e.Home, ".claude.json")
}

// trustClaude は .claude.json の projects["<根>"].hasTrustDialogAccepted を true にする。ほかのキー・値は
// そのまま残す（読んだ JSON の値を組み直さずに書き戻す）。無ければ作る（Claude Code は足りないキーを既定で補う）。
// JSON の object でなければ書かない。
func trustClaude(e Env) error {
	path := e.claudeJSON()
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	top := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(b)) > 0 {
		if err := json.Unmarshal(b, &top); err != nil || top == nil {
			return e.skipTrust("%s が JSON の object でない（書かない）", path)
		}
	}
	projects := map[string]json.RawMessage{}
	if raw, ok := top["projects"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &projects); err != nil || projects == nil {
			return e.skipTrust("%s の projects が object でない（書かない）", path)
		}
	}
	var added []string
	for _, root := range e.trustRoots() {
		entry := map[string]json.RawMessage{}
		if raw, ok := projects[root]; ok && string(raw) != "null" {
			if err := json.Unmarshal(raw, &entry); err != nil || entry == nil {
				return e.skipTrust("%s の projects[%q] が object でない（書かない）", path, root)
			}
		}
		if string(entry["hasTrustDialogAccepted"]) == "true" {
			continue
		}
		entry["hasTrustDialogAccepted"] = json.RawMessage("true")
		raw, err := marshal(entry, "")
		if err != nil {
			return err
		}
		projects[root] = raw
		added = append(added, root)
	}
	if len(added) == 0 {
		return nil
	}
	raw, err := marshal(projects, "")
	if err != nil {
		return err
	}
	top["projects"] = raw
	out, err := marshal(top, "  ")
	if err != nil {
		return err
	}
	if err := e.writeFile(path, append(out, '\n'), modeOr(path, 0o600)); err != nil {
		return err
	}
	e.logf("信頼を足した（Claude Code）: %s", strings.Join(added, " "))
	if len(b) == 0 && e.ClaudeDir != "" {
		return e.symlink(path, filepath.Join(e.Home, ".claude.json")) // 作ったときは rescueClaudeJSON と同じリンクを張る
	}
	return nil
}

// skipTrust は設定の形が読めないとき: 書かずにログに1行出して続ける（人の設定を壊さない。boot は止めない）。
func (e Env) skipTrust(format string, a ...any) error {
	e.logf("信頼を足さない: "+format, a...)
	return nil
}

// marshal は HTML の escape をせずに JSON にする（読んだ値の文字をそのまま残す）。indent が空なら1行。
func marshal(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// modeOr は path の今の権限。無ければ def。
func modeOr(path string, def fs.FileMode) fs.FileMode {
	if st, err := os.Stat(path); err == nil {
		return st.Mode().Perm()
	}
	return def
}

var (
	// codexProjectRe は [projects."<path>"] か [projects.'<path>'] の表の見出し。
	codexProjectRe = regexp.MustCompile(`^\s*\[\s*projects\s*\.\s*("((?:[^"\\]|\\.)*)"|'([^']*)')\s*\]\s*(#.*)?$`)
	codexTableRe   = regexp.MustCompile(`^\s*\[`)
	codexTrustRe   = regexp.MustCompile(`^\s*trust_level\s*=`)
	// codexOtherRe は projects をこの関数が読まない形で書いたもの（[projects] の表・top-level の projects = {…}・
	// projects."<path>".… の dotted key）。
	codexOtherRe = regexp.MustCompile(`^\s*(\[\s*projects\s*\]|projects\s*(=|\.))`)
)

// trustCodex は ~/.codex/config.toml に [projects."<根>"] trust_level = "trusted" を足す。
// 今ある表には触らない（trust_level があれば値が何でもそのまま。人が untrusted にしたものを覆さない）。
// 表があって trust_level だけが無ければ、見出しの直後に1行足す。projects を別の形で書いていれば書かない。
func trustCodex(e Env) error {
	path := filepath.Join(e.Home, ".codex", "config.toml")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if text == "" {
		lines = nil
	}
	type table struct {
		at    int // 見出しの行
		trust bool
	}
	tables := map[string]*table{}
	var cur *table
	top := true // まだ表の見出しの前（top-level の key）
	for i, ln := range lines {
		if m := codexProjectRe.FindStringSubmatch(ln); m != nil {
			key, ok := m[3], true
			if strings.HasPrefix(m[1], `"`) {
				key, ok = unescapeTOML(m[2])
			}
			if !ok {
				return e.skipTrust("%s の見出しの escape が読めない（書かない）: %s", path, strings.TrimSpace(ln))
			}
			cur, top = &table{at: i}, false
			if _, dup := tables[key]; !dup {
				tables[key] = cur
			}
			continue
		}
		if codexOtherRe.MatchString(ln) && (top || codexTableRe.MatchString(ln)) {
			return e.skipTrust("%s の projects がこの形で読めない（書かない）: %s", path, strings.TrimSpace(ln))
		}
		if codexTableRe.MatchString(ln) {
			cur, top = nil, false
			continue
		}
		if cur != nil && codexTrustRe.MatchString(ln) {
			cur.trust = true
		}
	}
	var added []string
	insert := map[int]bool{}
	var tail []string
	for _, root := range e.trustRoots() {
		t, ok := tables[root]
		switch {
		case ok && t.trust:
			continue
		case ok:
			insert[t.at] = true
		default:
			tail = append(tail, "", fmt.Sprintf("[projects.%s]", quoteTOML(root)), `trust_level = "trusted"`)
		}
		added = append(added, root)
	}
	if len(added) == 0 {
		return nil
	}
	var out []string
	for i, ln := range lines {
		out = append(out, ln)
		if insert[i] {
			out = append(out, `trust_level = "trusted"`)
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 && len(tail) > 0 {
		tail = tail[1:] // 新しいファイルは空行で始めない
	}
	out = append(out, tail...)
	if err := e.writeFile(path, []byte(strings.Join(out, "\n")+"\n"), modeOr(path, 0o600)); err != nil {
		return err
	}
	e.logf("信頼を足した（Codex）: %s", strings.Join(added, " "))
	return nil
}

// quoteTOML は TOML の basic string（"…"）にする。
func quoteTOML(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// unescapeTOML は basic string の中身の \" と \\ を戻す。ほかの escape があれば ok=false（同じパスの表を
// 二つにすると TOML として壊れるので、読めないものは書かない側に倒す）。
func unescapeTOML(s string) (string, bool) {
	if strings.Contains(strings.NewReplacer(`\"`, "", `\\`, "").Replace(s), `\`) {
		return "", false
	}
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(s), true
}
