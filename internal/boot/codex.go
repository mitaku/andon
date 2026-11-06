package boot

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 建屋の Codex を権限の確認で止めない（建屋＝コンテナが信頼の境界。docs/image.md の「Codex の承認と sandbox」）。
// Claude Code の managed settings の deny と同じ線は、Codex の execpolicy の rules で引く（forbidden は承認の方針に
// かかわらず先に効く）。

// codexDefaults は ~/.codex/config.toml の top-level に無ければ足すキー（足す順）。
// sandbox を外すのは、担当が worktree の外（台帳の report・library）に書き、git push と hc の socket を使うため。
var codexDefaults = []struct{ key, line string }{
	{"approval_policy", `approval_policy = "never"`},
	{"sandbox_mode", `sandbox_mode = "danger-full-access"`},
}

// codexTopKeyRe は top-level の key = の行（bare key か引用した key）。
var codexTopKeyRe = regexp.MustCompile(`^\s*("([^"]*)"|'([^']*)'|[A-Za-z0-9_-]+)\s*=`)

// genCodexConfig は ~/.codex/config.toml の top-level に approval_policy・sandbox_mode が無ければ足す。
// 今ある値は何であれそのまま（人が決めた値を覆さない）。足すのはファイルの先頭（最初の表の見出しより前＝top-level）。
func genCodexConfig(e Env) error {
	path := filepath.Join(e.Home, ".codex", "config.toml")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	have := map[string]bool{}
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "[") {
			break // ここから下は表の中
		}
		if m := codexTopKeyRe.FindStringSubmatch(ln); m != nil {
			have[m[1]], have[m[2]], have[m[3]] = true, true, true
		}
	}
	var add, keys []string
	for _, d := range codexDefaults {
		if !have[d.key] {
			add = append(add, d.line)
			keys = append(keys, d.key)
		}
	}
	if len(add) == 0 {
		return nil
	}
	head := "# andon boot が足した（建屋が信頼の境界。値を変えれば boot は覆さない）\n" + strings.Join(add, "\n") + "\n"
	if strings.TrimSpace(text) != "" {
		head += "\n" + text
	}
	mode := fs.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := e.writeFile(path, []byte(head), mode); err != nil {
		return err
	}
	e.logf("Codex の既定を足した: %s", strings.Join(keys, " "))
	return nil
}

// genCodexRules は <pkg>/codex.rules を ~/.codex/rules/andon.rules に生成する（boot が持つファイル。
// 人が書き換えていれば .bak に退避してから書く）。人の rules は別のファイル（default.rules など）に書く。
// パッケージに無ければ飛ばす。
func genCodexRules(e Env) error {
	b, err := os.ReadFile(filepath.Join(e.Package, "codex.rules"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return e.generate(filepath.Join(e.Home, ".codex", "rules", "andon.rules"), b, 0o644)
}
