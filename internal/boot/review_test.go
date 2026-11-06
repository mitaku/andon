package boot

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 人が書いた CLAUDE.md・Codex の AGENTS.md・herdr の設定は黙って消さず .bak に退避する。
// boot が前に書いたものは、パッケージが変わっても退避せずに書き直す。
func TestBacksUpHandWrittenFiles(t *testing.T) {
	f := newFixture(t)
	f.fullPackage(t)
	hand := map[string]string{
		filepath.Join(f.claude, "CLAUDE.md"):                                     "人の指示\n",
		filepath.Join(f.home, ".codex", "AGENTS.md"):                             "人の Codex\n",
		filepath.Join(f.home, ".config", "herdr", "config.toml"):                 "[mine]\n",
		filepath.Join(f.home, ".config", "herdr-core", "templates", "claude.md"): "人のプロファイル\n",
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

	// パッケージが変わった: boot が書いたものなので退避しない
	put(t, filepath.Join(f.pkg, "templates", "claude.md"), "新しい claude\n")
	put(t, filepath.Join(f.pkg, "host", "herdr", "config.toml"), "[ui2]\n")
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

func TestFilesHandlerServesOnlyInsideRoot(t *testing.T) {
	r := t.TempDir()
	root := filepath.Join(r, "files")
	put(t, filepath.Join(root, "report.html"), "納品物")
	put(t, filepath.Join(r, "secret.json"), "秘密")
	mustMkdir(t, filepath.Join(r, "outside"))
	put(t, filepath.Join(r, "outside", "x.txt"), "外")
	for link, to := range map[string]string{
		"leak.json": filepath.Join(r, "secret.json"),
		"inner":     "report.html", // 中を指す相対リンクは返す（絶対パスのリンクは os.Root が開かない）
		"dir":       filepath.Join(r, "outside"),
	} {
		if err := os.Symlink(to, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(t, filepath.Join(root, "sub"))
	if err := os.Symlink(filepath.Join(r, "secret.json"), filepath.Join(root, "sub", "index.html")); err != nil {
		t.Fatal(err)
	}
	h, err := FilesHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) (int, string) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = path
		h.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	// 中の実体は返す。外を指すものは返さない（os.Root が開かない。コードは 404 か 500、ディレクトリは一覧に落ちる）
	for path, want := range map[string]string{"/report.html": "納品物", "/inner": "納品物"} {
		if code, body := get(path); code != 200 || body != want {
			t.Errorf("%s: code=%d body=%q", path, code, body)
		}
	}
	for _, path := range []string{"/leak.json", "/sub/", "/sub/index.html", "/dir/x.txt", "/../secret.json", "/none"} {
		code, body := get(path)
		if strings.Contains(body, "秘密") || strings.Contains(body, "外") || (code == 200 && path != "/sub/") {
			t.Errorf("%s: code=%d body=%q", path, code, body)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/report.html", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", w.Code)
	}

	linked := filepath.Join(r, "linked")
	if err := os.Symlink(filepath.Join(r, "outside"), linked); err != nil {
		t.Fatal(err)
	}
	if _, err := FilesHandler(linked); err == nil {
		t.Error("files 自体が symlink なのに配った")
	}
}

// portless alias が失敗したら、成功の行を出さずに誤りを返す（Start が警告にする）。
func TestFilesAliasFailureIsReported(t *testing.T) {
	f := newFixture(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	put(t, filepath.Join(f.ws, ".env"), "FILES_PORT="+port+"\n")
	portless := filepath.Join(f.root, "bin", "portless")
	put(t, portless, "#!/bin/sh\necho alias に失敗 >&2\nexit 3\n")
	if err := os.Chmod(portless, 0o755); err != nil {
		t.Fatal(err)
	}
	e := f.env()
	e.Self = "/nonexistent/andon" // 待ち受けがあるので起こさない
	e.LookPath = func(n string) (string, error) {
		if n == "portless" {
			return portless, nil
		}
		return "", os.ErrNotExist
	}
	err = startFiles(e)
	if err == nil || !strings.Contains(err.Error(), "alias に失敗") {
		t.Errorf("err = %v", err)
	}
	if strings.Contains(f.log.String(), "files: http://") {
		t.Errorf("失敗したのに成功の行を出した: %s", f.log.String())
	}
}
