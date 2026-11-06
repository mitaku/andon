package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const codexHead = "# andon boot が足した（建屋が信頼の境界。値を変えれば boot は覆さない）\n"

func TestCodexConfigAndRulesFromNothing(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(f.home, ".codex", "config.toml")
	want := codexHead + "approval_policy = \"never\"\nsandbox_mode = \"danger-full-access\"\n"
	if s := read(t, cfg); s != want {
		t.Errorf("config.toml = %q, want %q", s, want)
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
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	got := read(t, cfg)
	want := codexHead + "sandbox_mode = \"danger-full-access\"\n\n" + strings.ReplaceAll(user, "\r\n", "\n")
	if got != want {
		t.Errorf("config.toml =\n%s\nwant\n%s", got, want)
	}
	if st, _ := os.Stat(cfg); st.Mode().Perm() != 0o640 {
		t.Errorf("権限を変えた: %v", st.Mode())
	}
	if !strings.Contains(f.log.String(), "Codex の既定を足した: sandbox_mode") {
		t.Errorf("ログ: %s", f.log.String())
	}

	// 両方あれば触らない（値が何でも。引用した key も key として数える）
	both := "\"approval_policy\" = \"untrusted\"\nsandbox_mode='workspace-write'\n[sandbox_workspace_write]\nnetwork_access = true\n"
	put(t, cfg, both)
	f.log.Reset()
	if err := Generate(f.env()); err != nil {
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
