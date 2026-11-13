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

	"github.com/BurntSushi/toml"

	"andon/internal/ledger"
	"andon/internal/spawn"
)

// 信頼の画面をセルでは出さない（セル＝コンテナが信頼の境界。人の決定 2026-11-06）。
// boot が、エージェントが起きる場所の根を Claude Code と Codex の両方で信頼済みにしておく。足りない所だけ足し、
// 今ある設定は触らない（2回目は何も書かない）。
//
// 根は「セルの repo」と「台帳に載った案件の repo（セルの projects/*/kanban/config.yaml の repos にあり、
// <GhqRoot>/<host>/<owner>/<repo> に clone されたもの）」だけ。ghq の下の repo を全部は信頼しない（台帳に載せるのは
// Chief of Staff と人で、andon issue start が起動してよいのもその repo だけ）。担当・レビュアーの worktree は
// 置き場（<セルの repo>/.worktrees）ではなく元の repo の根で判定されるので、置き場は足さない（実測は docs/image.md）:
//   - Claude Code: git の外は cwd から親へ / まで遡る。git の中は repo の根で止まり、それより上の親は効かない。
//     worktree は元の repo の根の信頼が効く（worktree の置き場の親は効かない）。
//   - Codex: git の外では信頼の画面を出さない。git の中は repo の根で判定し、worktree は元の repo の根で判定する
//     （置き場の親は効かない。worktree そのものの根でも効く）。

// Trust は信頼の根を Claude Code と Codex の両方で信頼済みにする（足りない所だけ足す。boot の生成の2つと同じ手）。
func Trust(e Env) error {
	return errors.Join(trustClaude(e), trustCodex(e))
}

// TrustFromOS は環境変数の Env で Trust する。andon issue start・andon chief-engineer が起動する直前に打つ
// （boot のあとに ghq get で追加した repo の根も足すため。docs/operate.md の 7）。ログは log へ。
func TrustFromOS(log io.Writer) error {
	e := EnvFromOS(log)
	if e.Home == "" {
		return errors.New("HOME が無い")
	}
	return Trust(e)
}

// trustRoots は信頼を置く根（実在するものを、symlink を解いた絶対パスで）。順はセルの repo、台帳の名前順と repos の順。
func (e Env) trustRoots() []string {
	cands := []string{e.Workspace}
	ghq, err := filepath.EvalSymlinks(e.GhqRoot)
	if e.GhqRoot != "" && e.Workspace != "" && err == nil {
		cfgs, _ := filepath.Glob(filepath.Join(e.Workspace, "projects", "*", "kanban", "config.yaml"))
		sort.Strings(cfgs)
		for _, cfg := range cfgs {
			for _, repo := range ledger.At(filepath.Dir(filepath.Dir(cfg))).Repos() {
				if !spawn.RepoRe.MatchString(repo) {
					continue
				}
				// 実体のパスで、実体の GHQ_ROOT の内側にある clone だけ（symlink で外へ逃げる repo は信頼しない）
				dir, err := filepath.EvalSymlinks(filepath.Join(ghq, repo))
				if err != nil || !strings.HasPrefix(dir, ghq+string(filepath.Separator)) {
					if err == nil {
						e.logf("信頼を足さない: %s が GHQ_ROOT の外（%s）を指す", repo, dir)
					}
					continue
				}
				if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
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
//
// 動いている Claude Code も同じファイルを書くので、古い中身で上書きしないように: 足すものが無ければ書かない。
// 書く直前に読み直し、変わっていれば読み直した中身からやり直す（claudeTries 回で諦めてログに1行）。
// 読み直しから rename までの短い隙間に Claude Code が書いたものは負けうる（受け入れる。docs/decisions.md）。
func trustClaude(e Env) error {
	path := e.claudeJSON()
	roots := e.trustRoots()
	for try := 0; try < claudeTries; try++ {
		b, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		out, added, skip := addClaudeTrust(b, roots)
		if skip != "" {
			return e.skipTrust("%s %s（書かない）", path, skip)
		}
		if len(added) == 0 {
			return nil
		}
		claudeRace(path)
		now, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if !bytes.Equal(now, b) {
			continue // 読んだあとに書き換わった: 新しい中身からやり直す
		}
		if err := e.writeFile(path, out, modeOr(path, 0o600)); err != nil {
			return err
		}
		e.logf("信頼を足した（Claude Code）: %s", strings.Join(added, " "))
		if len(b) == 0 && e.ClaudeDir != "" {
			return e.symlink(path, filepath.Join(e.Home, ".claude.json")) // 作ったときは rescueClaudeJSON と同じリンクを張る
		}
		return nil
	}
	return e.skipTrust("%s が読むたびに書き換わる（%d 回。次の起動か andon issue start で足す）", path, claudeTries)
}

// claudeTries は .claude.json が読むたびに書き換わるとき、やり直す回数。
const claudeTries = 5

// claudeRace は書く直前の読み直しの前に呼ぶ（テストが並行の書き手を差し込む）。
var claudeRace = func(string) {}

// addClaudeTrust は .claude.json の中身 b に roots の信頼を足した中身を返す。足すものが無ければ added が空。
// 形が読めなければ skip に理由。
func addClaudeTrust(b []byte, roots []string) (out []byte, added []string, skip string) {
	top := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(b)) > 0 {
		if err := json.Unmarshal(b, &top); err != nil || top == nil {
			return nil, nil, "が JSON の object でない"
		}
	}
	projects := map[string]json.RawMessage{}
	if raw, ok := top["projects"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &projects); err != nil || projects == nil {
			return nil, nil, "の projects が object でない"
		}
	}
	for _, root := range roots {
		entry := map[string]json.RawMessage{}
		if raw, ok := projects[root]; ok && string(raw) != "null" {
			if err := json.Unmarshal(raw, &entry); err != nil || entry == nil {
				return nil, nil, fmt.Sprintf("の projects[%q] が object でない", root)
			}
		}
		if string(entry["hasTrustDialogAccepted"]) == "true" {
			continue
		}
		entry["hasTrustDialogAccepted"] = json.RawMessage("true")
		raw, err := marshal(entry, "")
		if err != nil {
			return nil, nil, err.Error()
		}
		projects[root] = raw
		added = append(added, root)
	}
	if len(added) == 0 {
		return nil, nil, ""
	}
	raw, err := marshal(projects, "")
	if err != nil {
		return nil, nil, err.Error()
	}
	top["projects"] = raw
	if out, err = marshal(top, "  "); err != nil {
		return nil, nil, err.Error()
	}
	return append(out, '\n'), added, ""
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
	// codexOtherRe は projects をこの関数が読まない形で書いたもの（[projects] の表・top-level の projects = {…}・
	// projects."<path>".… の dotted key）。
	codexOtherRe = regexp.MustCompile(`^\s*(\[\s*projects\s*\]|projects\s*(=|\.))`)
)

// trustCodex は ~/.codex/config.toml に [projects."<根>"] trust_level = "trusted" を足す。
// 今ある表には触らない（trust_level があれば値が何でもそのまま。人が untrusted にしたものを覆さない）。
// 表があって trust_level だけが無ければ、見出しの直後に1行足す。projects を別の形で書いていれば書かない。
// trust_level の有無は TOML のパーサで読む（読めない TOML には書かない）。見出しの行を探すときは複数行の文字列の中を
// 飛ばす。書く前に出力を読み直し、足した根の trust_level がパーサから見えなければ書かない。
func trustCodex(e Env) error {
	path := filepath.Join(e.Home, ".codex", "config.toml")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	trusted, err := codexTrusted(text)
	if err != nil {
		return e.skipTrust("%s が TOML として読めない（書かない）: %v", path, err)
	}
	lines := strings.Split(text, "\n")
	if text == "" {
		lines = nil
	}
	inString := multilineStringLines(lines)
	type table struct {
		at    int // 見出しの行
		trust bool
	}
	tables := map[string]*table{}
	var cur *table
	top := true // まだ表の見出しの前（top-level の key）
	for i, ln := range lines {
		if inString[i] {
			continue // 複数行の文字列の中（見出しでもキーでもない）
		}
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
	}
	for key, t := range tables {
		_, t.trust = trusted[key]
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
	data := strings.Join(out, "\n") + "\n"
	after, err := codexTrusted(data)
	for _, root := range added {
		if _, ok := after[root]; err != nil || !ok {
			return e.skipTrust("%s に足した %s の trust_level が読み直すと見えない（書かない）", path, root)
		}
	}
	if err := e.writeFile(path, []byte(data), modeOr(path, 0o600)); err != nil {
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

// codexTrusted は config.toml の中身から、trust_level を持つ projects の根を返す（値は問わない）。
func codexTrusted(text string) (map[string]bool, error) {
	var cfg struct {
		Projects map[string]map[string]any `toml:"projects"`
	}
	if _, err := toml.Decode(strings.TrimPrefix(text, "\ufeff"), &cfg); err != nil {
		return nil, err
	}
	got := map[string]bool{}
	for root, p := range cfg.Projects {
		if _, ok := p["trust_level"]; ok {
			got[root] = true
		}
	}
	return got, nil
}

// multilineStringLines は、行ごとに「行の頭が複数行の文字列（"""…""" か ”'…”'）の中にあるか」を返す。
// 1行の中の文字列・コメントの中の引用符は見分けない（取り違えたときは、書く前の読み直しで止まる）。
func multilineStringLines(lines []string) []bool {
	in := make([]bool, len(lines))
	delim := ""
	for i, ln := range lines {
		in[i] = delim != ""
		for rest := ln; ; {
			if delim != "" {
				j := strings.Index(rest, delim)
				if j < 0 {
					break
				}
				rest, delim = rest[j+3:], ""
				continue
			}
			j, d := strings.Index(rest, `"""`), `"""`
			if k := strings.Index(rest, "'''"); k >= 0 && (j < 0 || k < j) {
				j, d = k, "'''"
			}
			if j < 0 {
				break
			}
			rest, delim = rest[j+3:], d
		}
	}
	return in
}
