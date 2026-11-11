package boot

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// セルの Codex を権限の確認で止めない（セル＝コンテナが信頼の境界。docs/image.md の「Codex の承認と sandbox」）。
// Claude Code の managed settings の deny のうち頭で引く線を、Codex の execpolicy の rules で引く（forbidden は承認の方針に
// かかわらず先に効く。拒めるのは直接の形とよくある wrapper だけ）。

// codexDefaults は ~/.codex/config.toml の top-level に無ければ足すキー（足す順）。
// sandbox を外すのは、担当が worktree の外（台帳の report・library）に書き、git push と hc の socket を使うため。
var codexDefaults = []struct{ key, line string }{
	{"approval_policy", `approval_policy = "never"`},
	{"sandbox_mode", `sandbox_mode = "danger-full-access"`},
}

// genCodexConfig は ~/.codex/config.toml の top-level に approval_policy・sandbox_mode が無ければ足す。
// 承認・sandbox の今ある値はそのまま。記憶の生成と利用は常に止める（ADR-0007 C5）。キーの有無は TOML のパーサで読み、足すのはファイルの先頭
// （BOM があればその後ろ。最初の表の見出しより前＝top-level）。TOML として読めなければ書かない（人の設定を壊さない）。
func genCodexConfig(e Env) error {
	path := filepath.Join(e.Home, ".codex", "config.toml")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	const bom = "\ufeff"
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	lead := ""
	if strings.HasPrefix(text, bom) {
		lead, text = bom, strings.TrimPrefix(text, bom)
	}
	have := map[string]any{}
	if _, err := toml.Decode(text, &have); err != nil {
		e.logf("Codex の既定を足さない: %s が TOML として読めない（書かない）: %v", path, err)
		return nil
	}
	// memories・features の表が無ければ inline table を先頭に足し、ほかの行とコメントは保持する。
	// 既存の表を直す場合だけ TOML を再生成する。引用キー・dotted key・inline table も同じ意味で扱う。
	var add, keys []string
	rewrite := false
	for _, policy := range []struct {
		table, line string
		fields      []string
	}{
		{"memories", "memories = { generate_memories = false, use_memories = false }", []string{"generate_memories", "use_memories"}},
		{"features", "features = { memories = false }", []string{"memories"}},
	} {
		if value, exists := have[policy.table]; !exists {
			add = append(add, policy.line)
			keys = append(keys, policy.table)
		} else {
			table, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("Codex の %s が表ではない: %s", policy.table, path)
			}
			for _, field := range policy.fields {
				if table[field] != false {
					table[field] = false
					rewrite = true
				}
			}
		}
	}
	if rewrite {
		var out bytes.Buffer
		encoder := toml.NewEncoder(&out)
		encoder.Indent = ""
		if err := encoder.Encode(have); err != nil {
			return err
		}
		// trustCodex が読む形は [projects."<path>"]。encoder が出す空の親見出しは省く。
		text = strings.Replace(out.String(), "[projects]\n", "", 1)
		keys = append(keys, "記憶を無効化")
	}
	for _, d := range codexDefaults {
		if _, ok := have[d.key]; !ok {
			add = append(add, d.line)
			keys = append(keys, d.key)
		}
	}
	if len(add) == 0 && !rewrite {
		return nil
	}
	head := lead + "# andon boot が足した（セルが信頼の境界。承認・sandbox の既存値は保ち、記憶は止める）\n" + strings.Join(add, "\n") + "\n"
	if strings.TrimSpace(text) != "" {
		head += "\n" + text
	}
	mode := fs.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	// 再生成するとコメントと書式が失われるため、元の設定は通常の生成物と同じく退避する。
	write := e.writeFile
	if rewrite {
		write = e.generate
	}
	if err := write(path, []byte(head), mode); err != nil {
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
