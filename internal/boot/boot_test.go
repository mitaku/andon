package boot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixture は偽の HOME・偽の建屋・偽のパッケージを一時ディレクトリに作る。
type fixture struct {
	root, pkg, ws, home, claude, managed string
	log                                  bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	r := t.TempDir()
	f := &fixture{
		root:    r,
		pkg:     filepath.Join(r, "opt", "andon"),
		ws:      filepath.Join(r, "workspace"),
		home:    filepath.Join(r, "home"),
		managed: filepath.Join(r, "etc", "claude-code", "managed-settings.json"),
	}
	f.claude = filepath.Join(f.home, ".claude")
	for _, d := range []string{f.pkg, f.ws, f.home} {
		mustMkdir(t, d)
	}
	return f
}

func (f *fixture) env() Env {
	return Env{
		Package: f.pkg, Workspace: f.ws, Home: f.home, ClaudeDir: f.claude,
		ManagedSettings: f.managed, Hostname: "example-plant",
		LookPath: func(string) (string, error) { return "", errors.New("無い") },
		Log:      &f.log,
	}
}

func mustMkdir(t *testing.T, d string) {
	t.Helper()
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
}

func put(t *testing.T, path, body string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// snapshot は root の下の全部（種類・中身・リンク先・mode・更新時刻）を1つの文字列にする。
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		st, err := os.Lstat(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case st.Mode()&fs.ModeSymlink != 0:
			to, _ := os.Readlink(p)
			b.WriteString("L " + rel + " -> " + to + "\n")
		case st.IsDir():
			b.WriteString("D " + rel + "\n")
		default:
			c, _ := os.ReadFile(p)
			b.WriteString("F " + rel + " " + st.Mode().String() + " " + st.ModTime().Format(time.RFC3339Nano) + "\n" + string(c) + "\n")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// fullPackage はパッケージと建屋に、生成の元になるものを一通り置く。
func (f *fixture) fullPackage(t *testing.T) {
	put(t, filepath.Join(f.pkg, "settings.json"), `{"permissions":{"deny":["Bash(gh pr merge:*)"]},"hooks":{}}`)
	put(t, filepath.Join(f.pkg, "templates", "claude.md"), "---\nname: claude\ncommand: claude --plugin-dir /opt/andon\n---\nパッケージの claude。`/opt/andon/skills/worker/SKILL.md`\n")
	put(t, filepath.Join(f.pkg, "templates", "sonnet.md"), "---\nname: sonnet\n---\nパッケージの sonnet\n")
	put(t, filepath.Join(f.pkg, "templates", ".gitkeep"), "")
	put(t, filepath.Join(f.pkg, "environment.md"), "# 建屋の環境\n")
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	put(t, filepath.Join(f.pkg, "skills", "glossary", "SKILL.md"), "glossary\n")
	put(t, filepath.Join(f.pkg, "host", "herdr", "config.toml"), "[ui]\n")
	put(t, filepath.Join(f.ws, "templates.local", "sonnet.md"), "---\nname: sonnet\n---\n建屋の sonnet\n")
	put(t, filepath.Join(f.ws, "templates.local", "local-only.md"), "建屋だけ\n")
	put(t, filepath.Join(f.ws, "AGENTS.local.md"), "# 案件の部分")
}

func TestGenerateWritesEverything(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(read(t, f.managed)), &got); err != nil {
		t.Fatalf("managed settings が JSON でない: %v", err)
	}
	if deny := got["permissions"].(map[string]any)["deny"].([]any); len(deny) != 1 || deny[0] != "Bash(gh pr merge:*)" {
		t.Errorf("managed settings: %v", got)
	}

	tpl := filepath.Join(f.home, ".config", "herdr-core", "templates")
	for name, want := range map[string]string{
		"claude.md":     "パッケージの claude",
		"sonnet.md":     "建屋の sonnet", // 建屋の templates.local が勝つ
		"local-only.md": "建屋だけ",
	} {
		if s := read(t, filepath.Join(tpl, name)); !strings.Contains(s, want) {
			t.Errorf("%s: %q に %q が無い", name, s, want)
		}
	}
	if _, err := os.Stat(filepath.Join(tpl, ".gitkeep")); err == nil {
		t.Error(".gitkeep をプロファイルにしている")
	}

	want := "@" + filepath.Join(f.pkg, "environment.md") + "\n@" + filepath.Join(f.ws, "AGENTS.local.md") + "\n"
	if s := read(t, filepath.Join(f.claude, "CLAUDE.md")); s != want {
		t.Errorf("CLAUDE.md = %q, want %q", s, want)
	}
	if s := read(t, filepath.Join(f.home, ".codex", "AGENTS.md")); s != "# 建屋の環境\n\n# 案件の部分\n" {
		t.Errorf("Codex の AGENTS.md = %q", s)
	}
	for _, name := range []string{"worker", "glossary"} {
		if to, err := os.Readlink(filepath.Join(f.home, ".agents", "skills", "andon-"+name)); err != nil || to != filepath.Join(f.pkg, "skills", name) {
			t.Errorf("skill %s: %q %v", name, to, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(f.claude, "skills")); err == nil {
		t.Error("<claude>/skills を作った（Claude Code は plugin で読む）")
	}
	if s := read(t, filepath.Join(f.home, ".config", "herdr", "config.toml")); s != "[ui]\n" {
		t.Errorf("herdr の設定 = %q", s)
	}
}

func TestGenerateIsIdempotent(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	first := snapshot(t, f.root)
	f.log.Reset()
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if second := snapshot(t, f.root); second != first {
		t.Errorf("2回目で変わった:\n--- 1回目\n%s\n--- 2回目\n%s", first, second)
	}
	if f.log.Len() != 0 {
		t.Errorf("2回目に何か書いた: %s", f.log.String())
	}
}

func TestGenerateWithEmptyPackage(t *testing.T) {
	f := newFixture(t)
	for _, d := range []string{"skills", "templates", "hooks", "host", "bin"} {
		put(t, filepath.Join(f.pkg, d, ".gitkeep"), "")
	}
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if s := read(t, f.managed); s != "{}\n" {
		t.Errorf("managed settings = %q", s)
	}
	if s := read(t, filepath.Join(f.claude, "CLAUDE.md")); strings.Count(s, "\n") != 2 {
		t.Errorf("CLAUDE.md = %q", s)
	}
	first := snapshot(t, f.root)
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, f.root) != first {
		t.Error("空のパッケージで2回目に変わった")
	}
}

// 仮設の symlink の網から移るとき: リンクを外して実ファイルを生成し、リンク先には書かない。
func TestReplacesLegacySymlinks(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	repo := filepath.Join(f.root, "old-repo")
	put(t, filepath.Join(repo, "AGENTS.md"), "仮設の環境メモ\n")
	put(t, filepath.Join(repo, "coordinator.md"), "仮設のプロファイル\n")
	mustMkdir(t, f.claude)
	if err := os.Symlink(filepath.Join(repo, "AGENTS.md"), filepath.Join(f.claude, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	tpl := filepath.Join(f.home, ".config", "herdr-core", "templates")
	mustMkdir(t, tpl)
	if err := os.Symlink(filepath.Join(repo, "coordinator.md"), filepath.Join(tpl, "claude.md")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(tpl, "mine.md"), "人が入れたもの\n")

	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(f.claude, "CLAUDE.md"), filepath.Join(tpl, "claude.md")} {
		if st, err := os.Lstat(p); err != nil || !st.Mode().IsRegular() {
			t.Errorf("%s が実ファイルになっていない", p)
		}
	}
	if read(t, filepath.Join(repo, "AGENTS.md")) != "仮設の環境メモ\n" || read(t, filepath.Join(repo, "coordinator.md")) != "仮設のプロファイル\n" {
		t.Error("リンク先に書き込んだ")
	}
	if read(t, filepath.Join(tpl, "mine.md")) != "人が入れたもの\n" {
		t.Error("パッケージに無いプロファイルに触った")
	}
}

func TestBrokenSettingsKeepsManaged(t *testing.T) {
	f := newFixture(t)
	put(t, f.managed, "{\"keep\": true}\n")
	put(t, filepath.Join(f.pkg, "settings.json"), "{壊れている")
	err := Generate(f.env())
	if err == nil || !strings.Contains(err.Error(), "managed settings") {
		t.Fatalf("err = %v", err)
	}
	if read(t, f.managed) != "{\"keep\": true}\n" {
		t.Error("壊れた settings.json で managed settings を書き換えた")
	}
	if _, err := os.Stat(filepath.Join(f.claude, "CLAUDE.md")); err != nil {
		t.Error("1つ失敗したら残りを止めた")
	}
}

func TestSkillLinks(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	agents := filepath.Join(f.home, ".agents", "skills")
	old := filepath.Join(f.claude, "skills")
	for _, d := range []string{agents, old} {
		mustMkdir(t, d)
		for name, to := range map[string]string{
			"andon-gone": filepath.Join(f.pkg, "skills", "gone"),
			"my-skill":   filepath.Join(f.root, "my-skill"),
		} {
			if err := os.Symlink(to, filepath.Join(d, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 前の置き場の andon-worker（仮設の形から移るときに残っているもの）
	if err := os.Symlink(filepath.Join(f.pkg, "skills", "worker"), filepath.Join(old, "andon-worker")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(agents, "andon-real", "SKILL.md"), "実ディレクトリ\n")
	// 利用者が andon- の名前で置いた skill（パッケージの外を指す）: 消えた名前でも同じ名前でも触らない
	if err := os.Symlink(filepath.Join(f.root, "my-skill"), filepath.Join(agents, "andon-personal")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.root, "my-skill"), filepath.Join(old, "andon-personal")); err != nil {
		t.Fatal(err)
	}

	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(agents, "andon-gone"), filepath.Join(old, "andon-gone"), filepath.Join(old, "andon-worker")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s が残っている", p)
		}
	}
	for _, p := range []string{filepath.Join(agents, "my-skill"), filepath.Join(old, "my-skill"), filepath.Join(agents, "andon-real", "SKILL.md"),
		filepath.Join(agents, "andon-personal"), filepath.Join(old, "andon-personal")} {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("%s を外した（symlink でない・andon- でないものには触らない）", p)
		}
	}
	if to, err := os.Readlink(filepath.Join(agents, "andon-worker")); err != nil || to != filepath.Join(f.pkg, "skills", "worker") {
		t.Errorf("andon-worker → %q %v", to, err)
	}
}

// 同じ名前でも、利用者のリンク（パッケージの外を指す）は張り替えない。ログに1行出す。
func TestSkillLinkSameNameUserLinkIsKept(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	agents := filepath.Join(f.home, ".agents", "skills")
	mustMkdir(t, agents)
	mine := filepath.Join(f.root, "my-worker")
	if err := os.Symlink(mine, filepath.Join(agents, "andon-worker")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if to, _ := os.Readlink(filepath.Join(agents, "andon-worker")); to != mine {
		t.Errorf("利用者の andon-worker を張り替えた: → %q", to)
	}
	if !strings.Contains(f.log.String(), "触らない: "+filepath.Join(agents, "andon-worker")) {
		t.Errorf("ログに出していない: %s", f.log.String())
	}
}

// ANDON_HOME を替えて打ち直すと、既定の /opt/andon の skills を指す前のリンクは boot のものとして張り替える。
func TestSkillLinkFollowsPackageChange(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	agents := filepath.Join(f.home, ".agents", "skills")
	mustMkdir(t, agents)
	if err := os.Symlink(filepath.Join(DefaultPackage, "skills", "worker"), filepath.Join(agents, "andon-worker")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(DefaultPackage, "skills", "gone"), filepath.Join(agents, "andon-gone")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if to, _ := os.Readlink(filepath.Join(agents, "andon-worker")); to != filepath.Join(f.pkg, "skills", "worker") {
		t.Errorf("andon-worker → %q", to)
	}
	if _, err := os.Lstat(filepath.Join(agents, "andon-gone")); err == nil {
		t.Error("andon-gone が残っている")
	}
}

func TestAgentsSkillsDirSymlinkStops(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	repo := filepath.Join(f.root, "skills-repo")
	mustMkdir(t, repo)
	mustMkdir(t, filepath.Join(f.home, ".agents"))
	if err := os.Symlink(repo, filepath.Join(f.home, ".agents", "skills")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(f.env()); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Errorf("~/.agents/skills が symlink なのに黙って通した: %v", err)
	}
	if ents, _ := os.ReadDir(repo); len(ents) != 0 {
		t.Error("リンク先の repo に書き込んだ")
	}
}

// 前の置き場 <claude>/skills が symlink（仮設の形）なら、その先には触らない（andon-* も外さない）。
func TestOldClaudeSkillsDirSymlinkIsLeftAlone(t *testing.T) {
	f := newFixture(t)
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	repo := filepath.Join(f.root, "skills-repo")
	mustMkdir(t, repo)
	if err := os.Symlink(filepath.Join(f.pkg, "skills", "worker"), filepath.Join(repo, "andon-worker")); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, f.claude)
	if err := os.Symlink(repo, filepath.Join(f.claude, "skills")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(repo, "andon-worker")); err != nil {
		t.Error("リンク先の repo の中を外した")
	}
	if _, err := os.Readlink(filepath.Join(f.home, ".agents", "skills", "andon-worker")); err != nil {
		t.Error("~/.agents/skills/andon-worker が無い")
	}
}

// プロファイルの本文の /opt/andon は ANDON_HOME に置き換わる。既定のままなら変えない。
func TestTemplatesUsePackagePath(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	put(t, filepath.Join(f.ws, "templates.local", "mine.md"), "`/opt/andon/skills/worker/SKILL.md`\n")
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	tpl := filepath.Join(f.home, ".config", "herdr-core", "templates")
	if s := read(t, filepath.Join(tpl, "claude.md")); strings.Contains(s, " /opt/andon") || strings.Contains(s, "`/opt/andon") ||
		!strings.Contains(s, "--plugin-dir "+f.pkg+"\n") || !strings.Contains(s, "`"+f.pkg+"/skills/worker/SKILL.md`") {
		t.Errorf("claude.md:\n%s", s)
	}
	if s := read(t, filepath.Join(tpl, "mine.md")); s != "`"+f.pkg+"/skills/worker/SKILL.md`\n" {
		t.Errorf("mine.md = %q", s)
	}

	in := []byte("--plugin-dir /opt/andon `/opt/andon/skills/worker/SKILL.md`")
	for pkg, want := range map[string]string{
		"/opt/andon":          string(in),
		"/opt/andon/":         string(in),
		"/src/andon/package/": "--plugin-dir /src/andon/package `/src/andon/package/skills/worker/SKILL.md`",
		"/src/andon/package":  "--plugin-dir /src/andon/package `/src/andon/package/skills/worker/SKILL.md`",
	} {
		if got, err := Template(in, pkg); err != nil || string(got) != want {
			t.Errorf("Template(%q) = %q %v, want %q", pkg, got, err, want)
		}
	}
	// パスの要素の境界だけ置き換える
	for in, want := range map[string]string{
		"--plugin-dir /opt/andon-snapshot":     "--plugin-dir /opt/andon-snapshot",
		"--plugin-dir /opt/andonx/skills":      "--plugin-dir /opt/andonx/skills",
		"/srv/opt/andon/skills":                "/srv/opt/andon/skills",
		"/opt/andon.bak":                       "/opt/andon.bak",
		"/opt/andon":                           "/p",
		"x=/opt/andon/skills\n'/opt/andon' ":   "x=/p/skills\n'/p' ",
		"--plugin-dir=/opt/andon,a:/opt/andon": "--plugin-dir=/p,a:/p",
	} {
		if got, err := Template([]byte(in), "/p"); err != nil || string(got) != want {
			t.Errorf("Template(%q) = %q %v, want %q", in, got, err, want)
		}
	}
	// command で割れる文字・相対パスはエラー（quote を作らない）
	for _, pkg := range []string{"/tmp/andon checkout/package", "/tmp/a\tb", "/tmp/a'b", "/tmp/$x", "rel/package"} {
		if _, err := Template(in, pkg); err == nil {
			t.Errorf("Template(%q) がエラーにならない", pkg)
		}
	}
}

// ANDON_HOME が command で割れるなら、プロファイルを1枚も書かずにエラー（今あるものを残す）。
func TestTemplatesUnsafePackageStops(t *testing.T) {
	f := newFixture(t)
	f.pkg = filepath.Join(f.root, "andon checkout", "package")
	f.fullPackage(t)
	tpl := filepath.Join(f.home, ".config", "herdr-core", "templates")
	put(t, filepath.Join(tpl, "claude.md"), "前のプロファイル\n")
	err := Generate(f.env())
	if err == nil || !strings.Contains(err.Error(), "プロファイル") {
		t.Fatalf("err = %v", err)
	}
	if read(t, filepath.Join(tpl, "claude.md")) != "前のプロファイル\n" {
		t.Error("プロファイルを書き換えた")
	}
	if _, err := os.Stat(filepath.Join(tpl, "sonnet.md")); err == nil {
		t.Error("ほかのプロファイルを書いた")
	}
	if _, err := os.Stat(filepath.Join(f.claude, "CLAUDE.md")); err != nil {
		t.Error("1つ失敗したら残りを止めた")
	}
}

func TestClaudeLauncherAndRescue(t *testing.T) {
	f := newFixture(t)
	versions := filepath.Join(f.home, ".local", "share", "claude", "versions")
	for _, v := range []string{"1.9.3", "1.10.0", "1.10.0.tmp-download", "2.0"} {
		put(t, filepath.Join(versions, v), "bin")
	}
	put(t, filepath.Join(f.claude, "backups", ".claude.json.backup.1"), strings.Repeat("x", 2000))
	put(t, filepath.Join(f.claude, "backups", ".claude.json.backup.2"), "{}") // 空に近いものは飛ばす
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if to, _ := os.Readlink(filepath.Join(f.home, ".local", "bin", "claude")); to != filepath.Join(versions, "2.0") {
		t.Errorf("claude → %q", to)
	}
	cj := filepath.Join(f.claude, ".claude.json")
	if read(t, cj) != strings.Repeat("x", 2000) {
		t.Error(".claude.json をバックアップから戻していない")
	}
	if to, _ := os.Readlink(filepath.Join(f.home, ".claude.json")); to != cj {
		t.Errorf("~/.claude.json → %q", to)
	}
}

func TestVersionLess(t *testing.T) {
	for _, c := range [][2]string{{"1.9.3", "1.10.0"}, {"1.2", "1.2.1"}, {"0.9", "2.0"}} {
		if !versionLess(c[0], c[1]) || versionLess(c[1], c[0]) {
			t.Errorf("%s < %s が成り立たない", c[0], c[1])
		}
	}
}

// Main を環境変数だけで走らせる: 偽の HOME・偽の建屋、PATH には記録するだけの偽の hc。
// 本物の hc・HOME・/workspace に触らない。2回目はクーリエを起こし直さない。
func TestMainWithFakeHome(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	bin := filepath.Join(f.root, "bin")
	calls := filepath.Join(f.root, "hc-calls")
	put(t, filepath.Join(bin, "hc"), "#!/bin/sh\nPATH=/usr/bin:/bin\necho \"$*\" >> '"+calls+"'\n"+
		"[ \"$1\" = get ] && { grep -q '^run ' '"+calls+"' && exit 0 || exit 1; }\nexit 0\n")
	if err := os.Chmod(filepath.Join(bin, "hc"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", f.home)
	t.Setenv("CLAUDE_CONFIG_DIR", f.claude)
	t.Setenv("ANDON_HOME", f.pkg)
	t.Setenv("ANDON_WORKSPACE", f.ws)
	t.Setenv("ANDON_MANAGED_SETTINGS", f.managed)

	for i := 1; i <= 2; i++ {
		var out, errb bytes.Buffer
		if rc := Main(nil, nil, &out, &errb); rc != 0 || errb.Len() != 0 || !strings.Contains(out.String(), "ready") {
			t.Fatalf("%d 回目: rc=%d out=%q err=%q", i, rc, out.String(), errb.String())
		}
	}
	got := read(t, calls)
	if strings.Count(got, "run ") != 1 {
		t.Errorf("クーリエを起こした回数が1でない:\n%s", got)
	}
	for _, want := range []string{"--name courier", "--label role=courier", "--cwd " + f.ws, filepath.Join(f.pkg, "bin", "andon") + " courier"} {
		if !strings.Contains(got, want) {
			t.Errorf("hc run に %q が無い:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(f.claude, "CLAUDE.md")); err != nil {
		t.Error("偽の HOME に CLAUDE.md が無い")
	}
}

func TestMainRejectsArgs(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Main([]string{"x"}, nil, &out, &errb); rc != 2 || !strings.HasPrefix(errb.String(), "usage: andon boot") {
		t.Errorf("rc=%d err=%q", rc, errb.String())
	}
}

// herdr-core は ~/.local/bin が正。無いか symlink なら aqua の種へ繋ぎ、実ファイル（Herdr App が置いたもの）は触らない
func TestHerdrCoreLink(t *testing.T) {
	f := newFixture(t)
	seed := filepath.Join(f.root, "aqua", "pkgs", "herdr-core")
	put(t, seed, "seed")
	aqua := filepath.Join(f.root, "bin", "aqua")
	put(t, aqua, "#!/bin/sh\necho "+seed+"\n")
	if err := os.Chmod(aqua, 0o755); err != nil {
		t.Fatal(err)
	}
	e := f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "aqua" {
			return aqua, nil
		}
		return "", errors.New("無い")
	}
	bin := filepath.Join(f.home, ".local", "bin")
	core, hc := filepath.Join(bin, "herdr-core"), filepath.Join(bin, "hc")

	// 無い → 種へ。hc は herdr-core への相対 symlink
	if err := linkHerdrCore(e); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(core); got != seed {
		t.Fatalf("herdr-core → %q", got)
	}
	if got, _ := os.Readlink(hc); got != "herdr-core" {
		t.Fatalf("hc → %q", got)
	}

	// 古い種を指す symlink → 今の種へ張り直す
	if err := os.Remove(core); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.root, "old"), core); err != nil {
		t.Fatal(err)
	}
	if err := linkHerdrCore(e); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(core); got != seed {
		t.Fatalf("herdr-core → %q（張り直していない）", got)
	}

	// 実ファイル（App が置いた）→ 触らない。hc もそのまま
	if err := os.Remove(core); err != nil {
		t.Fatal(err)
	}
	put(t, core, "from-app")
	if err := linkHerdrCore(e); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(core); string(b) != "from-app" {
		t.Fatalf("App の herdr-core を書き換えた: %q", b)
	}
	if got, _ := os.Readlink(hc); got != "herdr-core" {
		t.Fatalf("hc → %q", got)
	}

	// hc が実ファイルなら触らない
	if err := os.Remove(hc); err != nil {
		t.Fatal(err)
	}
	put(t, hc, "own-hc")
	if err := linkHerdrCore(e); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(hc); string(b) != "own-hc" {
		t.Fatalf("hc を書き換えた: %q", b)
	}
}
