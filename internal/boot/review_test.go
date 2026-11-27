package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/flock"
	"andon/internal/tray"
)

func TestBacksUpHandWrittenFiles(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	hand := map[string]string{
		filepath.Join(f.claude, "CLAUDE.md"):                                     "オーナーの指示\n",
		filepath.Join(f.home, ".codex", "AGENTS.md"):                             "オーナーの Codex\n",
		filepath.Join(f.home, ".config", "herdr-core", "templates", "claude.md"): "オーナーのプロファイル\n",
	}
	for p, body := range hand {
		put(t, p, body)
	}
	put(t, filepath.Join(f.claude, "CLAUDE.md.bak"), "前の退避\n")
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	for p, body := range hand {
		bak := p + ".bak"
		if strings.HasSuffix(p, "CLAUDE.md") {
			m, _ := filepath.Glob(p + ".bak.*")
			if len(m) != 1 {
				t.Fatalf("CLAUDE.md の退避が .bak.<時刻> に無い: %v", m)
			}
			bak = m[0]
			if read(t, p+".bak") != "前の退避\n" {
				t.Error("前の .bak を上書きした")
			}
		}
		if read(t, bak) != body {
			t.Errorf("%s を退避していない", p)
		}
		if read(t, p) == body {
			t.Errorf("%s を生成していない", p)
		}
		if !strings.Contains(f.log.String(), "退避: "+p+" → ") {
			t.Errorf("%s の退避を1行出していない:\n%s", p, f.log.String())
		}
	}

	put(t, filepath.Join(f.pkg, "templates", "claude.md"), "新しい claude\n")
	f.log.Reset()
	if err := Generate(f.env()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.log.String(), "退避") {
		t.Errorf("boot が書いたものを退避した:\n%s", f.log.String())
	}
	if read(t, filepath.Join(f.home, ".config", "herdr-core", "templates", "claude.md")) != "新しい claude\n" {
		t.Error("パッケージの更新が届いていない")
	}
}

func TestNullSettingsKeepsManaged(t *testing.T) {
	f := newFixture(t)
	put(t, f.managed, "{\"keep\": true}\n")
	put(t, filepath.Join(f.pkg, "settings.json"), "null")
	if err := Generate(f.env()); err == nil || !strings.Contains(err.Error(), "managed settings") {
		t.Fatalf("err = %v", err)
	}
	if read(t, f.managed) != "{\"keep\": true}\n" {
		t.Error("null で managed settings を書き換えた")
	}
}

// trayFixture は、ビルドした tray の置き場（ANDON_HOME/tray）と、動いている serve の控え（pid は自分）と鍵を一時の置き場に作る。
func trayFixture(t *testing.T, f *fixture, portless string) Env {
	t.Helper()
	home := t.TempDir()
	for _, rel := range []string{"scripts/serve.mjs", "dist/workers/local.js", "dist/workers/local-upload.js"} {
		put(t, filepath.Join(home, "tray", rel), "")
	}
	t.Setenv("ANDON_HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := tray.WriteControl(&tray.Control{List: "http://127.0.0.1:45678", Upload: "http://127.0.0.1:45679",
		ListOrigin: "http://tray.cell.localhost:1355", PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	unlock, err := flock.Lock(tray.LockPath()) // 動いている serve は鍵を持つ
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unlock)
	e := f.env()
	e.Self = "/nonexistent/andon"
	e.LookPath = func(n string) (string, error) {
		switch {
		case n == "node":
			return "/bin/sh", nil
		case n == "portless" && portless != "":
			return portless, nil
		}
		return "", os.ErrNotExist
	}
	return e
}

// ADR-0064 決めたこと 6: 動いているローカルの tray に、portless の名前 tray（一覧の面）と tray-view（報告の面）を付ける。
func TestTrayAliases(t *testing.T) {
	f := newFixture(t)
	portless := filepath.Join(f.root, "bin", "portless")
	calls := filepath.Join(f.root, "calls")
	put(t, portless, "#!/bin/sh\necho \"$*\" >>"+calls+"\n")
	if err := os.Chmod(portless, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := startTray(trayFixture(t, f, portless)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, calls); got != "alias tray 45678\nalias tray-view 45678\n" {
		t.Errorf("portless の呼び出し: %q", got)
	}
	if !strings.Contains(f.log.String(), "ローカルの tray: http://tray.cell.localhost:1355/") {
		t.Errorf("ログ: %s", f.log.String())
	}
}

func TestTrayAliasFailureIsReported(t *testing.T) {
	f := newFixture(t)
	portless := filepath.Join(f.root, "bin", "portless")
	put(t, portless, "#!/bin/sh\necho alias に失敗 >&2\nexit 3\n")
	if err := os.Chmod(portless, 0o755); err != nil {
		t.Fatal(err)
	}
	err := startTray(trayFixture(t, f, portless))
	if err == nil || !strings.Contains(err.Error(), "alias に失敗") {
		t.Errorf("err = %v", err)
	}
	if strings.Contains(f.log.String(), "ローカルの tray: http://") {
		t.Errorf("失敗したのに成功の行を出した: %s", f.log.String())
	}
}

// Node が無ければ起動せず、理由を1行残す（報告は files/ に写る）。自分が andon のバイナリでなければ何もしない。
func TestTraySkipped(t *testing.T) {
	f := newFixture(t)
	e := trayFixture(t, f, "")
	if err := os.Remove(tray.ControlPath()); err != nil {
		t.Fatal(err)
	}
	e.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if err := startTray(e); err != nil || !strings.Contains(f.log.String(), "ローカルの tray は起動しない（node が無い）") {
		t.Errorf("err=%v log=%s", err, f.log.String())
	}
	f.log.Reset()
	e.Self = "/tmp/boot.test"
	if err := startTray(e); err != nil || f.log.Len() != 0 {
		t.Errorf("andon のバイナリでないのに: err=%v log=%s", err, f.log.String())
	}
}
