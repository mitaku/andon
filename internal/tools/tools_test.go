package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/tools/toolstest"
)

func TestInstallCallsMiseInRoot(t *testing.T) {
	f := toolstest.New(t, "")
	root := t.TempDir()
	toolstest.Manifest(t, root)
	installed, err := Install(root)
	if err != nil || installed {
		t.Fatalf("installed=%v err=%v", installed, err)
	}
	if got := f.Calls(); len(got) != 1 || got[0] != "mise install | cwd="+root {
		t.Errorf("calls = %q", got)
	}
	if line := Satisfy(root); line != "" {
		t.Errorf("入っているときに 1 行出した: %q", line)
	}
}

func TestInstallReportsInstalled(t *testing.T) {
	toolstest.New(t, "installed")
	root := t.TempDir()
	toolstest.Manifest(t, root)
	if installed, err := Install(root); err != nil || !installed {
		t.Fatalf("installed=%v err=%v", installed, err)
	}
	if line := Satisfy(root); !strings.Contains(line, "道具を入れた") {
		t.Errorf("入れたときの 1 行 = %q", line)
	}
}

func TestInstallSkipsWithoutManifest(t *testing.T) {
	f := toolstest.New(t, "")
	root := t.TempDir()
	// 根の直下でなければ見ない（下のフォルダの mise.toml・フォルダの mise.toml）
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	toolstest.Manifest(t, filepath.Join(root, "sub"))
	os.Mkdir(filepath.Join(root, "mise.toml"), 0o755)
	for _, r := range []string{root, ""} {
		if installed, err := Install(r); err != nil || installed {
			t.Errorf("%q: installed=%v err=%v", r, installed, err)
		}
	}
	if got := f.Calls(); len(got) != 0 {
		t.Errorf("mise.toml が無いのに呼んだ: %q", got)
	}
	if env := EnvFor(root); env != nil {
		t.Errorf("EnvFor = %q", env)
	}
}

func TestInstallSkipsWithoutMise(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	toolstest.Manifest(t, root)
	if installed, err := Install(root); err != nil || installed {
		t.Fatalf("installed=%v err=%v", installed, err)
	}
}

func TestInstallFails(t *testing.T) {
	toolstest.New(t, "fail")
	root := t.TempDir()
	toolstest.Manifest(t, root)
	_, err := Install(root)
	if err == nil || !strings.Contains(err.Error(), "mise ERROR boom") {
		t.Fatalf("err = %v", err)
	}
	line := Satisfy(root)
	if !strings.Contains(line, "起動は続ける") || !strings.Contains(line, "cd "+root+" && mise install") {
		t.Errorf("警告 = %q", line)
	}
}

func TestInstallTimeoutLeavesMiseRunning(t *testing.T) {
	f := toolstest.New(t, "slow")
	toolstest.Short(t, &Limit, 200*time.Millisecond)
	root := t.TempDir()
	toolstest.Manifest(t, root)
	start := time.Now()
	_, err := Install(root)
	if err != ErrTimeout {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Errorf("上限で返らない: %v", d)
	}
	if f.Done() {
		t.Fatal("上限の前に偽の mise が終わっている（検査にならない）")
	}
	f.WaitDone(t)
}

func TestEnvFor(t *testing.T) {
	root := t.TempDir()
	toolstest.Manifest(t, root)
	if got := EnvFor(root); len(got) != 1 || got[0] != "MISE_SYSTEM_CONFIG_FILE="+filepath.Join(root, "mise.toml") {
		t.Errorf("EnvFor = %q", got)
	}
}
