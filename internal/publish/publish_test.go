package publish

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(t *testing.T) Env {
	t.Helper()
	return Env{Workspace: t.TempDir(), Hostname: "sample-site", Now: time.Date(2026, 1, 2, 23, 0, 0, 0, time.UTC)}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPublishCopiesWithDateAndSlug(t *testing.T) {
	e := env(t)
	src := filepath.Join(t.TempDir(), "review.html")
	write(t, src, "<p>v1</p>")
	cases := []struct{ slug, want string }{
		{"", "2026-01-02-review.html"},
		{"weekly", "2026-01-02-weekly.html"},
	}
	for _, c := range cases {
		url, err := Publish(e, src, c.slug)
		if err != nil {
			t.Fatal(err)
		}
		if want := "http://files.sample-site.localhost:" + DefaultPort + "/" + c.want; url != want {
			t.Errorf("slug %q: url = %q, want %q", c.slug, url, want)
		}
		got, err := os.ReadFile(filepath.Join(e.Workspace, "files", c.want))
		if err != nil || string(got) != "<p>v1</p>" {
			t.Errorf("slug %q: copy = %q, %v", c.slug, got, err)
		}
	}
	// 同じ日・同じ slug は上書き（再公開）。
	write(t, src, "<p>v2</p>")
	if _, err := Publish(e, src, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(e.Workspace, "files", "2026-01-02-review.html")); string(got) != "<p>v2</p>" {
		t.Errorf("republish = %q", got)
	}
}

func TestPublishPortFromEnvFile(t *testing.T) {
	cases := map[string]string{
		"":          DefaultPort,
		"OTHER=1\n": DefaultPort,
		"PORTLESS_PORT=2000\nPORTLESS_PORT=2001\n": "2001",
		"PORTLESS_PORT=\"2002\"\n":                 "2002",
		"PORTLESS_PORT=' 2003 '\n":                 "2003",
		"PORTLESS_PORT=\n":                         DefaultPort,
		"# PORTLESS_PORT=9\n":                      DefaultPort,
	}
	for body, want := range cases {
		e := env(t)
		write(t, filepath.Join(e.Workspace, ".env"), body)
		src := filepath.Join(t.TempDir(), "a.html")
		write(t, src, "x")
		url, err := Publish(e, src, "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(url, ".localhost:"+want+"/") {
			t.Errorf(".env %q: url = %q, want port %s", body, url, want)
		}
	}
}

func TestPublishRejectsNonFile(t *testing.T) {
	e := env(t)
	for _, p := range []string{filepath.Join(e.Workspace, "none.html"), e.Workspace} {
		if _, err := Publish(e, p, ""); err == nil || !strings.Contains(err.Error(), "not a file") {
			t.Errorf("%s: err = %v", p, err)
		}
	}
}

func TestMainUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Main(nil, nil, &out, &errb); rc != 2 || !strings.HasPrefix(errb.String(), "usage: andon publish") {
		t.Errorf("rc=%d err=%q", rc, errb.String())
	}
	errb.Reset()
	if rc := Main([]string{filepath.Join(t.TempDir(), "none")}, nil, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "not a file") {
		t.Errorf("rc=%d err=%q", rc, errb.String())
	}
}

// andon publish は ANDON_WORKSPACE のセルへ書く（ほかの command と同じ。検査が本物の /workspace/files に書かない）。
func TestMainPublishesIntoTheCellOfTheEnvironment(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	src := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(src, []byte("<p>x</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := Main([]string{src, "weekly"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	made, _ := filepath.Glob(filepath.Join(ws, "files", "*-weekly.html"))
	if len(made) != 1 {
		t.Fatalf("files/ に1つ書くはず: %v", made)
	}
	if got, _ := os.ReadFile(made[0]); string(got) != "<p>x</p>" {
		t.Errorf("%s: %q", made[0], got)
	}
	if !strings.HasSuffix(strings.TrimSpace(out.String()), "/"+filepath.Base(made[0])) {
		t.Errorf("URL: %q", out.String())
	}
}

func TestPublishSameFileKeepsContent(t *testing.T) {
	e := env(t)
	if _, err := Publish(e, writeTemp(t, "weekly.html", "<p>v1</p>"), ""); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(e.Workspace, "files", "2026-01-02-weekly.html")
	if _, err := Publish(e, dst, "weekly"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "<p>v1</p>" {
		t.Errorf("同じファイルの公開し直しで中身 = %q", got)
	}
	entries, _ := os.ReadDir(filepath.Join(e.Workspace, "files"))
	if len(entries) != 1 {
		t.Errorf("一時ファイルが残った: %v", entries)
	}
}

// 拡張子の無いファイルは拡張子を付けない（仮設の <日付>-report.report から意図して変えた）。
func TestPublishWithoutExtension(t *testing.T) {
	e := env(t)
	src := writeTemp(t, "report", "x")
	for slug, want := range map[string]string{"": "2026-01-02-report", "weekly": "2026-01-02-weekly"} {
		url, err := Publish(e, src, slug)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(url, "/"+want) {
			t.Errorf("slug %q: url = %q, want …/%s", slug, url, want)
		}
		if _, err := os.Stat(filepath.Join(e.Workspace, "files", want)); err != nil {
			t.Error(err)
		}
	}
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	write(t, p, body)
	return p
}

func TestPublishRejectsTooLarge(t *testing.T) {
	e := env(t)
	src := filepath.Join(t.TempDir(), "big.bin")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxSize + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Publish(e, src, ""); err == nil || !strings.Contains(err.Error(), "大きすぎる") {
		t.Fatalf("上限超えを拒まなかった: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.Workspace, "files")); err == nil {
		t.Error("拒んだのに files/ を作った")
	}
}
