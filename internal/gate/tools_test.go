package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/tools"
	"andon/internal/tools/toolstest"
)

// checksRepo は bare な origin と、その clone を GHQ_ROOT の下に作る。runChecks が打つ https://github.com/example/app の fetch は、
// 一時の git の設定（GIT_CONFIG_GLOBAL）の insteadOf で origin へ向ける。返すのは mise.toml のある commit と無い commit。
func checksRepo(t *testing.T) (with, without string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	dir := t.TempDir()
	origin := filepath.Join(dir, "origin.git")
	clone := filepath.Join(dir, "ghq", "github.com", "example", "app")
	cfg := filepath.Join(dir, "gitconfig")
	os.WriteFile(cfg, []byte("[url \"file://"+origin+"\"]\n\tinsteadOf = https://github.com/example/app\n[user]\n\tname = t\n\temail = t@example.com\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GHQ_ROOT", filepath.Join(dir, "ghq"))
	g := func(wd string, args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = wd
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	g("", "init", "-q", "--bare", "-b", "main", origin)
	g("", "clone", "-q", origin, clone)
	g(clone, "commit", "-q", "--allow-empty", "-m", "init")
	without = g(clone, "rev-parse", "HEAD")
	toolstest.Manifest(t, clone)
	g(clone, "add", "mise.toml")
	g(clone, "commit", "-q", "-m", "mise")
	with = g(clone, "rev-parse", "HEAD")
	g(clone, "push", "-q", "origin", "HEAD:main")
	return with, without
}

// 検査の command は記録（$LOG）に打った印と、受け取った MISE_SYSTEM_CONFIG_FILE と cwd を書く。
const checkCmd = `printf 'check env=%s | cwd=%s\n' "${MISE_SYSTEM_CONFIG_FILE:-}" "$PWD" >>"$LOG"`

func checkLog(t *testing.T) []string {
	t.Helper()
	b, _ := os.ReadFile(os.Getenv("LOG"))
	if s := strings.TrimSpace(string(b)); s != "" {
		return strings.Split(s, "\n")
	}
	return nil
}

// runChecks は一時の worktree の根に mise.toml があれば、最初の検査の command の前に偽の mise を install で呼び、
// 検査の command に MISE_SYSTEM_CONFIG_FILE=<一時の worktree>/mise.toml を渡す（ADR-0063 受け入れ条件 3）。
func TestRunChecksSatisfiesDeclaration(t *testing.T) {
	with, _ := checksRepo(t)
	toolstest.New(t, "")
	t.Setenv("LOG", filepath.Join(t.TempDir(), "log"))
	checks, err := runChecks("github.com/example/app", with, []string{checkCmd, checkCmd})
	if err != nil || len(checks) != 2 || !checks[0].OK || !checks[1].OK {
		t.Fatalf("checks=%+v err=%v", checks, err)
	}
	got := checkLog(t)
	if len(got) != 3 {
		t.Fatalf("記録: %q", got)
	}
	wt, ok := strings.CutPrefix(got[0], "mise install | cwd=")
	if !ok || filepath.Base(wt) != "worktree" {
		t.Fatalf("最初が一時の worktree の根での mise install でない: %q", got)
	}
	for _, l := range got[1:] {
		if l != "check env="+filepath.Join(wt, "mise.toml")+" | cwd="+wt {
			t.Errorf("検査の command の環境: %q", l)
		}
	}
}

// mise.toml の無い commit では呼ばず、MISE_SYSTEM_CONFIG_FILE も渡さない（受け継いだ値はそのまま）。
func TestRunChecksWithoutManifest(t *testing.T) {
	_, without := checksRepo(t)
	m := toolstest.New(t, "")
	t.Setenv("LOG", filepath.Join(t.TempDir(), "log"))
	t.Setenv("MISE_SYSTEM_CONFIG_FILE", "")
	checks, err := runChecks("github.com/example/app", without, []string{checkCmd})
	if err != nil || len(checks) != 1 || !checks[0].OK {
		t.Fatalf("checks=%+v err=%v", checks, err)
	}
	if got := m.Calls(); len(got) != 0 {
		t.Errorf("mise.toml が無いのに呼んだ: %q", got)
	}
	if got := checkLog(t); len(got) != 1 || !strings.HasPrefix(got[0], "check env= | cwd=") {
		t.Errorf("記録: %q", got)
	}
}

// 偽の mise が失敗するか上限を過ぎると、検査の command は 1 つも打たれず、runChecks は失敗を返す（判定しない。受け入れ条件 3）。
// mise.toml があるのに mise が PATH に無いときも、検査の command を打たずに判定しない
// （道具の無い環境の command not found を、PR の欠陥として返さない）
func TestRunChecksRefusesWithoutMise(t *testing.T) {
	with, _ := checksRepo(t)
	bin := t.TempDir()
	for _, n := range []string{"git", "bash", "sh", "printf"} {
		if p, err := exec.LookPath(n); err == nil {
			os.Symlink(p, filepath.Join(bin, n))
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("LOG", filepath.Join(t.TempDir(), "log"))
	checks, err := runChecks("github.com/example/app", with, []string{checkCmd})
	if err == nil || checks != nil || !strings.Contains(err.Error(), "mise が PATH に無い") {
		t.Fatalf("checks=%+v err=%v", checks, err)
	}
	for _, l := range checkLog(t) {
		if strings.HasPrefix(l, "check ") {
			t.Errorf("mise が無いのに検査の command を打った: %q", l)
		}
	}
}

func TestRunChecksRefusesWhenDeclarationFails(t *testing.T) {
	for _, mode := range []string{"fail", "slow"} {
		t.Run(mode, func(t *testing.T) {
			with, _ := checksRepo(t)
			m := toolstest.New(t, mode)
			toolstest.Short(t, &tools.Limit, 200*time.Millisecond)
			t.Setenv("LOG", filepath.Join(t.TempDir(), "log"))
			checks, err := runChecks("github.com/example/app", with, []string{checkCmd})
			if err == nil || checks != nil || !strings.Contains(err.Error(), "検査の道具を入れられない（mise.toml の mise install。判定しない）") {
				t.Fatalf("checks=%+v err=%v", checks, err)
			}
			for _, l := range checkLog(t) {
				if strings.HasPrefix(l, "check ") {
					t.Errorf("満たせないのに検査の command を打った: %q", l)
				}
			}
			if mode == "slow" {
				m.WaitDone(t)
			}
		})
	}
}
