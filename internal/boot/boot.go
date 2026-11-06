// Package boot は建屋の起動（docs/design.md §2.3・§3.1）。仮設の起動スクリプト3本を1本にしたもの。
//
// 毎起動、パッケージ（/opt/andon）から建屋へ配布物を生成する。symlink の網は張らない:
//
//	<pkg>/settings.json                       → managed settings（/etc/claude-code/managed-settings.json）
//	<pkg>/templates/*.md ＋ <ws>/templates.local/*.md → ~/.config/herdr-core/templates/<name>.md（本文の /opt/andon は <pkg> に）
//	<pkg>/environment.md ＋ <ws>/AGENTS.local.md → <claude>/CLAUDE.md（@ の2行）・~/.codex/AGENTS.md（連結）
//	<pkg>/skills/<name>/                      → ~/.agents/skills/andon-<name>（symlink。Codex など .agents/skills を読むハーネス向け）
//	<pkg>/host/herdr/*.toml                   → ~/.config/herdr/…
//
// そのあと、Claude Code の起動リンクと .claude.json の救出、建屋の repo と台帳に載った repo の根を Claude Code・Codex で
// 信頼済みにする（trust.go。足りない所だけ足す）、portless・files サーバ・sshd・クーリエの起動。
// 起動は道具が無ければ飛ばし、失敗しても続ける（建屋は立ち上がりきることを優先する）。
// 生成は何度走らせても同じ結果になる（中身が同じなら書かない）。
package boot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Usage は andon boot --help の中身。
const Usage = "usage: andon boot\n" +
	"       andon boot serve-files <dir> <port>   files サーバ（boot が自分で起こす。<dir> の中の実体だけを 127.0.0.1 で返す）\n" +
	"パッケージ（ANDON_HOME。既定 /opt/andon）から建屋へ配布物を生成し、portless・files・sshd・クーリエを起こす。\n" +
	"毎起動、compose の command から打つ（andon boot; exec sleep infinity）。何度打っても同じ結果。\n" +
	"生成: managed settings・プロファイル（~/.config/herdr-core/templates）・CLAUDE.md の2行・Codex の AGENTS.md・skill の symlink（~/.agents/skills）・herdr の設定。\n" +
	"信頼: 建屋の repo と台帳（projects/*/.flightdeck/config.yaml の repos）に載った repo の根を .claude.json と ~/.codex/config.toml で信頼済みにする（足りない所だけ足す）。\n" +
	"生成に失敗すると rc 1（起動は続ける）。起動の失敗は警告だけ。\n"

// DefaultPackage はパッケージの既定の置き場。プロファイルの本文はこのパスで書く。
const DefaultPackage = "/opt/andon"

// Env は boot が読み書きする場所。テストで差し替える。
type Env struct {
	Package         string // パッケージ（ANDON_HOME。既定 /opt/andon）
	Workspace       string // 建屋の repo（ANDON_WORKSPACE。既定 /workspace）
	GhqRoot         string // 案件の repo の置き場（GHQ_ROOT）。空なら信頼の根に案件の repo を足さない
	Home            string // HOME
	ClaudeDir       string // CLAUDE_CONFIG_DIR。空なら <Home>/.claude
	ManagedSettings string // ANDON_MANAGED_SETTINGS。既定 /etc/claude-code/managed-settings.json
	Hostname        string
	// LookPath は道具を探す。見つからなければ、その起動は飛ばす。
	LookPath func(string) (string, error)
	Self     string // andon の実行ファイル（files サーバとして自分を起こす）
	Log      io.Writer
	Uid, Gid int // boot を打つ利用者。HOME の volume の持ち主をこれに揃える（fixOwners）
}

// EnvFromOS は環境変数から Env を作る。
func EnvFromOS(log io.Writer) Env {
	host, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	self, _ := os.Executable()
	return Env{
		Package:         getenv("ANDON_HOME", DefaultPackage),
		Workspace:       getenv("ANDON_WORKSPACE", "/workspace"),
		GhqRoot:         os.Getenv("GHQ_ROOT"),
		Home:            home,
		ClaudeDir:       os.Getenv("CLAUDE_CONFIG_DIR"),
		ManagedSettings: getenv("ANDON_MANAGED_SETTINGS", "/etc/claude-code/managed-settings.json"),
		Hostname:        host,
		LookPath:        exec.LookPath,
		Self:            self,
		Log:             log,
		Uid:             os.Getuid(),
		Gid:             os.Getgid(),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// Main は andon boot。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 3 && args[0] == "serve-files" {
		if err := serveFiles(args[1], args[2]); err != nil {
			fmt.Fprintf(stderr, "andon boot serve-files: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) > 0 {
		fmt.Fprint(stderr, Usage)
		return 2
	}
	env := EnvFromOS(stdout)
	if env.Home == "" {
		fmt.Fprintln(stderr, "andon boot: HOME が無い")
		return 1
	}
	if err := fixOwners(env); err != nil {
		env.logf("volume の持ち主の修正に失敗（続行する）: %v", err)
	}
	if err := Generate(env); err != nil {
		fmt.Fprintf(stderr, "andon boot: %v\n", err)
		Start(env)
		return 1
	}
	Start(env)
	fmt.Fprintln(stdout, "[andon boot] ready")
	return 0
}

func (e Env) claudeDir() string {
	if e.ClaudeDir != "" {
		return e.ClaudeDir
	}
	return filepath.Join(e.Home, ".claude")
}

func (e Env) logf(format string, a ...any) {
	if e.Log != nil {
		fmt.Fprintf(e.Log, "[andon boot] "+format+"\n", a...)
	}
}

// Generate はパッケージから配布物を生成する。1つ失敗しても残りは続け、失敗をまとめて返す。
func Generate(e Env) error {
	var errs []error
	for _, step := range []struct {
		name string
		run  func(Env) error
	}{
		{"managed settings", genManagedSettings},
		{"プロファイル", genTemplates},
		{"CLAUDE.md", genClaudeMD},
		{"Codex の AGENTS.md", genCodexAgents},
		{"skill", genSkills},
		{"herdr の設定", genHerdr},
		{"Claude Code の起動リンク", linkClaude},
		{"herdr-core の起動リンク", linkHerdrCore},
		{".claude.json", rescueClaudeJSON},
		{"信頼（Claude Code）", trustClaude}, // .claude.json を戻してから足す
		{"信頼（Codex）", trustCodex},
	} {
		if err := step.run(e); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", step.name, err))
		}
	}
	return errors.Join(errs...)
}

// --- 生成 ---

// writeFile は path を data にする。中身が同じなら書かない。path が symlink なら外して実ファイルにする
// （仮設の symlink の網から移るとき、リンク先の repo に書き込まない）。
func (e Env) writeFile(path string, data []byte, mode fs.FileMode) error {
	if st, err := os.Lstat(path); err == nil {
		if st.Mode().IsRegular() {
			if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, data) {
				return nil
			}
		} else if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".andon-boot-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	e.logf("生成: %s", path)
	return nil
}

// generate は boot が持つ配布物を書く。今ある実ファイルが、boot が前に書いたもの（控えの sha256）でも
// これから書くものでもなければ、人が書いたものとして <path>.bak に退避してから書く（黙って消さない）。
// 控えは ~/.local/state/andon/boot.json。読めなければ空として扱う（退避が増えるだけで、消えはしない）。
func (e Env) generate(path string, data []byte, mode fs.FileMode) error {
	state := e.loadState()
	if st, err := os.Lstat(path); err == nil && st.Mode().IsRegular() {
		cur, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(cur, data) && state[path] != sum(cur) {
			bak := path + ".bak"
			if _, err := os.Lstat(bak); err == nil {
				bak += "." + time.Now().Format("20060102-150405")
			}
			if err := os.Rename(path, bak); err != nil {
				return err
			}
			e.logf("退避: %s → %s（boot が書いたものでないため）", path, bak)
		}
	}
	if err := e.writeFile(path, data, mode); err != nil {
		return err
	}
	if state[path] == sum(data) {
		return nil
	}
	state[path] = sum(data)
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return e.writeFile(e.statePath(), append(b, '\n'), 0o644)
}

func (e Env) statePath() string {
	return filepath.Join(e.Home, ".local", "state", "andon", "boot.json")
}

func (e Env) loadState() map[string]string {
	state := map[string]string{}
	if b, err := os.ReadFile(e.statePath()); err == nil {
		if json.Unmarshal(b, &state) != nil || state == nil {
			state = map[string]string{}
		}
	}
	return state
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// genManagedSettings は <pkg>/settings.json を managed settings に置く。無ければ {}。
// JSON でなければ書かない（今あるものを残す）。置き場に書けなければ sudo で置く（root の場所）。
func genManagedSettings(e Env) error {
	src := filepath.Join(e.Package, "settings.json")
	settings := map[string]any{}
	if b, err := os.ReadFile(src); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil || settings == nil {
			return fmt.Errorf("%s が JSON のオブジェクトでない（書かずに残す）: %v", src, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if cur, err := os.ReadFile(e.ManagedSettings); err == nil && bytes.Equal(cur, data) {
		return nil
	}
	err = e.writeFile(e.ManagedSettings, data, 0o644)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	return e.sudoInstall(e.ManagedSettings, data)
}

func (e Env) sudoInstall(dst string, data []byte) error {
	sudo, err := e.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("%s に書けず sudo も無い", dst)
	}
	tmp, err := os.CreateTemp("", "andon-boot-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	if out, err := exec.Command(sudo, "-n", "install", "-D", "-m", "0644", tmp.Name(), dst).CombinedOutput(); err != nil {
		return fmt.Errorf("sudo install %s: %v: %s", dst, err, bytes.TrimSpace(out))
	}
	e.logf("生成: %s（sudo）", dst)
	return nil
}

// genTemplates はプロファイルを生成する。同じ名前なら建屋の templates.local が勝つ。
// 本文の /opt/andon はパッケージの置き場（ANDON_HOME）に置き換える（checkout の plugin と skill で起こせるように。ADR-0001 決定 5）。
// 置き場にある、どちらにも無い名前のファイルには触らない（人が hc に入れたもの）。
func genTemplates(e Env) error {
	if _, err := Template(nil, e.Package); err != nil {
		return err // 1枚も書かない（今あるプロファイルを残す）
	}
	files := map[string]string{}
	for _, dir := range []string{
		filepath.Join(e.Package, "templates"),
		filepath.Join(e.Workspace, "templates.local"),
	} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			return err
		}
		for _, m := range matches {
			files[filepath.Base(m)] = m
		}
	}
	dst := filepath.Join(e.Home, ".config", "herdr-core", "templates")
	var errs []error
	for _, name := range sortedKeys(files) {
		b, err := os.ReadFile(files[name])
		if err == nil {
			b, err = Template(b, e.Package)
		}
		if err == nil {
			err = e.generate(filepath.Join(dst, name), b, 0o644)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// safePath は command の引数に quote なしで置いても1つの引数のままでいられるパス。
var safePath = regexp.MustCompile(`^/[A-Za-z0-9._/@+:,=-]*$`)

// Template はプロファイルの本文の /opt/andon を pkg に置き換える。pkg が既定なら何もしない。
// 置き換えるのはパスの要素の境界にあるものだけ（前が「/」でもパスの文字でもなく、後ろが「/」かパスの文字でない）。
// /opt/andon-snapshot や /x/opt/andon は変えない。pkg が絶対パスでないか、空白など command で割れる文字を含めばエラー
// （quote を作り込まない）。
func Template(b []byte, pkg string) ([]byte, error) {
	pkg = filepath.Clean(pkg)
	if pkg == DefaultPackage {
		return b, nil
	}
	if !safePath.MatchString(pkg) {
		return nil, fmt.Errorf("ANDON_HOME %q は絶対パスでないか、command で割れる文字を含む（使えるのは英数字と ._/@+:,=-）。プロファイルを書かずに残す", pkg)
	}
	old := []byte(DefaultPackage)
	var out []byte
	for {
		i := bytes.Index(b, old)
		if i < 0 {
			return append(out, b...), nil
		}
		end := i + len(old)
		before := i == 0 || !isPathByte(b[i-1]) && b[i-1] != '/'
		after := end == len(b) || b[end] == '/' || !isPathByte(b[end])
		out = append(out, b[:i]...)
		if before && after {
			out = append(out, pkg...)
		} else {
			out = append(out, old...)
		}
		b = b[end:]
	}
}

// isPathByte はパスの要素の中に現れうる文字（「/」を除く）。「=」「:」「,」は区切りとして扱う（--plugin-dir=/opt/andon・a:/opt/andon）。
func isPathByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("._@+-", c) >= 0 || c >= 0x80
}

// ClaudeMD は <claude>/CLAUDE.md の中身（2行）。
func ClaudeMD(pkg, ws string) string {
	return "@" + filepath.Join(pkg, "environment.md") + "\n@" + filepath.Join(ws, "AGENTS.local.md") + "\n"
}

func genClaudeMD(e Env) error {
	return e.generate(filepath.Join(e.claudeDir(), "CLAUDE.md"), []byte(ClaudeMD(e.Package, e.Workspace)), 0o644)
}

// genCodexAgents は Codex の AGENTS.md を連結で生成する（Codex は @ で読まない）。無いほうは飛ばす。
func genCodexAgents(e Env) error {
	var parts [][]byte
	for _, p := range []string{filepath.Join(e.Package, "environment.md"), filepath.Join(e.Workspace, "AGENTS.local.md")} {
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if len(b) > 0 && b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
		parts = append(parts, b)
	}
	return e.generate(filepath.Join(e.Home, ".codex", "AGENTS.md"), bytes.Join(parts, []byte("\n")), 0o644)
}

// genSkills は ~/.agents/skills/andon-<name> → <pkg>/skills/<name> を張る（Codex など .agents/skills を読むハーネス向け。
// Claude Code は plugin として --plugin-dir で読む。ADR-0001 決定 2）。
// 外す・張り替えるのは boot が張ったリンク（andon-* で、パッケージの skills の直下を指す symlink）だけ。
// それ以外の andon-*（利用者が置いたもの・実ディレクトリ）には触らず、ログに1行出す。
// パッケージから消えた skill のリンクは外す。前の置き場 <claude>/skills の boot のリンクも外す（二重に読まれるのを避ける）。
func genSkills(e Env) error {
	dir := filepath.Join(e.Home, ".agents", "skills")
	if isSymlink(dir) {
		return fmt.Errorf("%s が symlink（仮設の形）。外してから打ち直す", dir)
	}
	want := map[string]string{}
	entries, err := os.ReadDir(filepath.Join(e.Package, "skills"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, ent := range entries {
		if ent.IsDir() {
			want["andon-"+ent.Name()] = filepath.Join(e.Package, "skills", ent.Name())
		}
	}
	errs := []error{e.unlinkAndon(dir, want)}
	for _, name := range sortedKeys(want) {
		link := filepath.Join(dir, name)
		if _, err := os.Lstat(link); err == nil && !e.ownedLink(link) {
			e.logf("触らない: %s（boot が張ったリンクでない）", link)
			continue
		}
		errs = append(errs, e.symlink(want[name], link))
	}
	if old := filepath.Join(e.claudeDir(), "skills"); isSymlink(old) {
		e.logf("%s が symlink（仮設の形）なので andon-* を外さない", old)
	} else {
		errs = append(errs, e.unlinkAndon(old, nil))
	}
	return errors.Join(errs...)
}

// ownedLink は link が boot の張ったリンク（パッケージの skills の直下を指す symlink）か。
// パッケージは今の ANDON_HOME と既定の /opt/andon の両方を見る（ANDON_HOME を替えて打ち直したとき張り替えられるように）。
func (e Env) ownedLink(link string) bool {
	to, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(to) {
		to = filepath.Join(filepath.Dir(link), to)
	}
	parent := filepath.Dir(filepath.Clean(to))
	for _, pkg := range []string{e.Package, DefaultPackage} {
		if parent == filepath.Join(filepath.Clean(pkg), "skills") {
			return true
		}
	}
	return false
}

// unlinkAndon は dir の中の、boot が張った andon-* のリンクのうち keep に無いものを外す。dir が無ければ何もしない。
func (e Env) unlinkAndon(dir string, keep map[string]string) error {
	have, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	var errs []error
	for _, ent := range have {
		if _, ok := keep[ent.Name()]; ok || !strings.HasPrefix(ent.Name(), "andon-") {
			continue
		}
		p := filepath.Join(dir, ent.Name())
		if !e.ownedLink(p) {
			e.logf("触らない: %s（boot が張ったリンクでない）", p)
			continue
		}
		if err := os.Remove(p); err != nil {
			errs = append(errs, err)
			continue
		}
		e.logf("外した: %s", p)
	}
	return errors.Join(errs...)
}

func isSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&fs.ModeSymlink != 0
}

// genHerdr はパッケージの herdr の設定を volume へ写す。パッケージに無ければ何もしない。
func genHerdr(e Env) error {
	cfg := filepath.Join(e.Home, ".config", "herdr")
	var errs []error
	for src, dst := range map[string]string{
		"config.toml":      filepath.Join(cfg, "config.toml"),
		"file-viewer.toml": filepath.Join(cfg, "plugins", "config", "herdr-file-viewer", "config.toml"),
	} {
		b, err := os.ReadFile(filepath.Join(e.Package, "host", "herdr", src))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil {
			err = e.generate(dst, b, 0o644)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// symlink は link → target にする。すでにそうなら何もしない。実ディレクトリがあれば触らない。
func (e Env) symlink(target, link string) error {
	if cur, err := os.Readlink(link); err == nil {
		if cur == target {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if _, err := os.Lstat(link); err == nil {
		st, _ := os.Stat(link)
		if st != nil && st.IsDir() {
			return fmt.Errorf("%s が実ディレクトリなので張らない", link)
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(target, link); err != nil {
		return err
	}
	e.logf("リンク: %s → %s", link, target)
	return nil
}

// --- Claude Code の起動リンクと設定の救出 ---

var versionRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)+$`)

// linkClaude は ~/.local/bin/claude を volume の中の最新版へ張り直す（自己更新で版が増え、
// ~/.local/bin はコンテナ層なので作り直すと古い種を指したままになる）。版が無ければ何もしない。
func linkClaude(e Env) error {
	dir := filepath.Join(e.Home, ".local", "share", "claude", "versions")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	newest := ""
	for _, ent := range entries {
		if versionRe.MatchString(ent.Name()) && (newest == "" || versionLess(newest, ent.Name())) {
			newest = ent.Name()
		}
	}
	if newest == "" {
		return nil
	}
	return e.symlink(filepath.Join(dir, newest), filepath.Join(e.Home, ".local", "bin", "claude"))
}

// linkHerdrCore は herdr-core（hc）を ~/.local/bin に揃える。正は ~/.local/bin/herdr-core で、Herdr App が
// Add Machine・Update HC でここに置き、差し替える（版は App が決める）。無いか symlink なら image の種（aqua）へ繋ぐ。
// 実ファイルは App の持ち物なので触らない。hc は無いときだけ herdr-core への相対 symlink にする（差し替えに付いていく）。
// ~/.local/bin は PATH で aqua より先なので、クーリエを起こす hc も、それが起こす core も同じ herdr-core になる。
func linkHerdrCore(e Env) error {
	bin := filepath.Join(e.Home, ".local", "bin")
	core := filepath.Join(bin, "herdr-core")
	if st, err := os.Lstat(core); err != nil || st.Mode()&fs.ModeSymlink != 0 {
		aqua, ok := e.has("aqua")
		if !ok {
			return nil
		}
		out, err := exec.Command(aqua, "which", "herdr-core").Output()
		if err != nil {
			return fmt.Errorf("aqua which herdr-core: %v", err)
		}
		seed := strings.TrimSpace(string(out))
		if seed == "" || seed == core {
			return nil
		}
		if err := e.symlink(seed, core); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(bin, "hc")); err != nil {
		return e.symlink("herdr-core", filepath.Join(bin, "hc"))
	}
	return nil
}

func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, _ := strconv.Atoi(as[i])
		y, _ := strconv.Atoi(bs[i])
		if x != y {
			return x < y
		}
	}
	return len(as) < len(bs)
}

// rescueClaudeJSON は CLAUDE_CONFIG_DIR を使うとき、<claude>/.claude.json（ログイン・信頼・installMethod）を守る。
// 無いか空なら ~/.claude.json（実ファイル）か、バックアップの最新（1KB 超）から戻す。
// あれば ~/.claude.json に読み取り用の symlink を張る（既定の場所を見る道具向け）。
func rescueClaudeJSON(e Env) error {
	if e.ClaudeDir == "" {
		return nil // Claude Code が ~/.claude.json を直に使う
	}
	cj := filepath.Join(e.ClaudeDir, ".claude.json")
	legacy := filepath.Join(e.Home, ".claude.json")
	if st, err := os.Stat(cj); err != nil || st.Size() == 0 {
		src := ""
		if st, err := os.Lstat(legacy); err == nil && st.Mode().IsRegular() {
			src = legacy
		} else {
			src = newestBackup(filepath.Join(e.ClaudeDir, "backups"))
		}
		if src != "" {
			b, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := e.writeFile(cj, b, 0o600); err != nil {
				return err
			}
			e.logf(".claude.json を %s から戻した", src)
		}
	}
	if _, err := os.Stat(cj); err != nil {
		return nil
	}
	return e.symlink(cj, legacy)
}

func newestBackup(dir string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, ".claude.json.backup.*"))
	best, bestT := "", int64(0)
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil || st.Size() <= 1024 {
			continue
		}
		if t := st.ModTime().UnixNano(); best == "" || t > bestT {
			best, bestT = m, t
		}
	}
	return best
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
