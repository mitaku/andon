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
		got, err := os.ReadFile(filepath.Join(e.Workspace, "artifacts", c.want))
		if err != nil || string(got) != "<p>v1</p>" {
			t.Errorf("slug %q: copy = %q, %v", c.slug, got, err)
		}
	}
	// 同じ日・同じ slug は上書き（再公開）。
	write(t, src, "<p>v2</p>")
	if _, err := Publish(e, src, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(e.Workspace, "artifacts", "2026-01-02-review.html")); string(got) != "<p>v2</p>" {
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
