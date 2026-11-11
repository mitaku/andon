package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

const codexMemory = "memories = { generate_memories = false, use_memories = false }\nfeatures = { memories = false }\n"

const codexHead = "# andon boot が足した（セルが信頼の境界。承認・sandbox の既存値は保ち、記憶は止める）\n"

// withoutCodexDefaults は genCodexConfig が先頭に足した塊（見出しの行からその後の空行まで）を外す
// （信頼のテストが、同じファイルに足す既定に左右されないように）。
func withoutCodexDefaults(s string) string {
	if !strings.HasPrefix(s, codexHead) {
		return s
	}
	rest := strings.TrimPrefix(s, codexHead)
	for strings.HasPrefix(rest, "features = ") || strings.HasPrefix(rest, "memories = ") || strings.HasPrefix(rest, "approval_policy = ") || strings.HasPrefix(rest, "sandbox_mode = ") {
		rest = rest[strings.Index(rest, "\n")+1:]
	}
	return strings.TrimPrefix(rest, "\n")
}

func TestCodexConfigAndRulesFromNothing(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(f.home, ".codex", "config.toml")
	want := codexHead + codexMemory + "approval_policy = \"never\"\nsandbox_mode = \"danger-full-access\"\n"
	if s := read(t, cfg); !strings.HasPrefix(s, want) { // 後ろに信頼の表（trust.go）が続く
		t.Errorf("config.toml = %q, want %q で始まる", s, want)
	}
	if st, err := os.Stat(cfg); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("config.toml の権限: %v %v", st, err)
	}
	rules := filepath.Join(f.home, ".codex", "rules", "andon.rules")
	if s := read(t, rules); s != read(t, filepath.Join(f.pkg, "codex.rules")) {
		t.Errorf("andon.rules = %q", s)
	}

	// 2回目は何も書かない
	f.log.Reset()
	before := snapshot(t, f.root)
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, f.root); after != before || f.log.Len() != 0 {
		t.Errorf("2回目に書いた: %s", f.log.String())
	}
}

func TestCodexConfigKeepsUserValues(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	cfg := filepath.Join(f.home, ".codex", "config.toml")
	// 人が決めた approval_policy は残す。表の中の sandbox_mode は top-level ではないので足す
	user := "model = \"gpt-x\"\r\napproval_policy = \"on-request\" # 人の値\r\n\r\n[profiles.ro]\r\nsandbox_mode = \"read-only\"\r\n\r\n[projects.\"/workspace\"]\r\ntrust_level = \"trusted\"\r\n"
	put(t, cfg, user)
	if err := os.Chmod(cfg, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := genCodexConfig(f.env()); err != nil {
		t.Fatal(err)
	}
	got := read(t, cfg)
	want := codexHead + codexMemory + "sandbox_mode = \"danger-full-access\"\n\n" + strings.ReplaceAll(user, "\r\n", "\n")
	if got != want {
		t.Errorf("config.toml =\n%s\nwant\n%s", got, want)
	}
	if st, _ := os.Stat(cfg); st.Mode().Perm() != 0o640 {
		t.Errorf("権限を変えた: %v", st.Mode())
	}
	if !strings.Contains(f.log.String(), "Codex の既定を足した: memories features sandbox_mode") {
		t.Errorf("ログ: %s", f.log.String())
	}

	// 両方あれば触らない（値が何でも。引用した key も key として数える）
	both := codexMemory + "\"approval_policy\" = \"untrusted\"\nsandbox_mode='workspace-write'\n[sandbox_workspace_write]\nnetwork_access = true\n"
	put(t, cfg, both)
	f.log.Reset()
	if err := genCodexConfig(f.env()); err != nil {
		t.Fatal(err)
	}
	if s := read(t, cfg); s != both || strings.Contains(f.log.String(), "config.toml") {
		t.Errorf("今ある値を書き換えた: %q %s", s, f.log.String())
	}
}

func TestCodexRulesSkippedWithoutPackageFile(t *testing.T) {
	f := newFixture(t)
	if err := genCodexRules(f.env()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(f.home, ".codex", "rules")); err == nil {
		t.Error("パッケージに無いのに rules を作った")
	}
}

func TestCodexConfigKeepsBOMFirst(t *testing.T) {
	f := newFixture(t)
	cfg := filepath.Join(f.home, ".codex", "config.toml")
	put(t, cfg, "\ufeffapproval_policy = \"on-request\"\n")
	if err := genCodexConfig(f.env()); err != nil {
		t.Fatal(err)
	}
	want := "\ufeff" + codexHead + codexMemory + "sandbox_mode = \"danger-full-access\"\n\napproval_policy = \"on-request\"\n"
	if s := read(t, cfg); s != want {
		t.Errorf("config.toml = %q, want %q", s, want)
	}
}

func TestCodexConfigReadsMultilineStrings(t *testing.T) {
	f := newFixture(t)
	cfg := filepath.Join(f.home, ".codex", "config.toml")
	// 複数行の文字列の中の [example] は表ではない。その後ろの top-level のキーも数える
	user := codexMemory + "developer_instructions = \"\"\"\n[example]\n\"\"\"\napproval_policy = \"on-request\"\nsandbox_mode = \"read-only\"\n"
	put(t, cfg, user)
	if err := genCodexConfig(f.env()); err != nil {
		t.Fatal(err)
	}
	if s := read(t, cfg); s != user {
		t.Errorf("今あるキーを重ねて足した: %q", s)
	}
}

func TestCodexConfigLeavesBrokenTOML(t *testing.T) {
	f := newFixture(t)
	cfg := filepath.Join(f.home, ".codex", "config.toml")
	put(t, cfg, "approval_policy = \n")
	if err := genCodexConfig(f.env()); err != nil {
		t.Fatal(err)
	}
	if s := read(t, cfg); s != "approval_policy = \n" || !strings.Contains(f.log.String(), "TOML として読めない") {
		t.Errorf("読めない設定に書いた: %q %s", s, f.log.String())
	}
}

func TestCodexMemoryDisabled(t *testing.T) {
	for name, user := range map[string]string{
		"empty":    "",
		"enabled":  "[features]\nmemories = true\napps = true\n[memories]\ngenerate_memories = true\nuse_memories = true\nmax_rollouts_per_startup = 8\n",
		"inline":   "memories = { use_memories = true }\n",
		"projects": "[memories]\nuse_memories = true\n[projects.\"/sample\"]\ntrust_level = \"trusted\"\n",
		"dotted":   "memories.generate_memories = true\n",
		"disabled": "[memories]\ngenerate_memories = false\nuse_memories = false\n",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			cfg := filepath.Join(f.home, ".codex", "config.toml")
			put(t, cfg, "model = \"gpt-x\"\napproval_policy = \"on-request\"\n"+user)
			if err := os.Chmod(cfg, 0o640); err != nil {
				t.Fatal(err)
			}
			if err := genCodexConfig(f.env()); err != nil {
				t.Fatal(err)
			}
			got := read(t, cfg)
			if st, err := os.Stat(cfg); err != nil || st.Mode().Perm() != 0o640 {
				t.Fatalf("権限を変えた: %v %v", st, err)
			}
			if _, err := codexTrusted(got); err != nil {
				t.Fatalf("信頼設定が読めない: %v", err)
			}
			var parsed map[string]any
			if _, err := toml.Decode(got, &parsed); err != nil {
				t.Fatal(err)
			}
			features, ok := parsed["features"].(map[string]any)
			if !ok || features["memories"] != false {
				t.Fatalf("記憶機能が止まっていない: %s", got)
			}
			if name == "enabled" && features["apps"] != true {
				t.Fatalf("ほかの機能を変えた: %s", got)
			}
			memory, ok := parsed["memories"].(map[string]any)
			if !ok || memory["generate_memories"] != false || memory["use_memories"] != false {
				t.Fatalf("記憶が止まっていない: %s", got)
			}
			if parsed["model"] != "gpt-x" || parsed["approval_policy"] != "on-request" {
				t.Fatalf("人の値を変えた: %s", got)
			}
			if name == "enabled" && memory["max_rollouts_per_startup"] != int64(8) {
				t.Fatalf("ほかの記憶設定を変えた: %s", got)
			}
			if name == "enabled" || name == "inline" || name == "dotted" || name == "projects" {
				if backup := read(t, cfg+".bak"); backup != "model = \"gpt-x\"\napproval_policy = \"on-request\"\n"+user {
					t.Fatalf("元の設定を退避していない: %s", backup)
				}
			}
			f.log.Reset()
			if err := genCodexConfig(f.env()); err != nil {
				t.Fatal(err)
			}
			if read(t, cfg) != got || f.log.Len() != 0 {
				t.Fatalf("2回目に書いた: %s", f.log.String())
			}
		})
	}
}
