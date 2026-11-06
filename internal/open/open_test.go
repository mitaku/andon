package open

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T, slugs ...string) string {
	t.Helper()
	ws := t.TempDir()
	for _, s := range slugs {
		d := filepath.Join(ws, "projects", s, "kanban")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "config.yaml"), []byte("version: 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ANDON_WORKSPACE", ws)
	// 偽の hc: sample-a だけ Chief of Staff が動いている
	bin := t.TempDir()
	hc := "#!/usr/bin/env bash\ncase \"$*\" in *project=sample-a*) echo '{\"terminals\":[{\"address\":\"t1\"}]}';; *) echo '{\"terminals\":[]}';; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(hc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return ws
}

func fakeStart(t *testing.T, got *string) {
	t.Helper()
	old := start
	start = func(dir string, _, _ io.Writer) int { *got = dir; return 0 }
	t.Cleanup(func() { start = old })
}

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := Main(args, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

func TestSlugOpensThatProject(t *testing.T) {
	ws := setup(t, "sample-a", "sample-b")
	var got string
	fakeStart(t, &got)
	if rc, _, _ := run("sample-b"); rc != 0 || got != filepath.Join(ws, "projects", "sample-b") {
		t.Errorf("rc=%d dir=%q", rc, got)
	}
}

func TestUnknownSlug(t *testing.T) {
	setup(t, "sample-a", "sample-b")
	var got string
	fakeStart(t, &got)
	rc, _, errs := run("nope")
	if rc != 2 || got != "" || !strings.Contains(errs, "'nope' は無い") || !strings.Contains(errs, "  sample-a\n") || !strings.Contains(errs, "  sample-b\n") {
		t.Errorf("rc=%d dir=%q err=%q", rc, got, errs)
	}
}

func TestNoSlugNotTerminalListsAndExits2(t *testing.T) {
	setup(t, "sample-a", "sample-b")
	var got string
	fakeStart(t, &got)
	rc, out, errs := run()
	if rc != 2 || got != "" || out != "sample-a\t(CoS 稼働中)\nsample-b\n" || !strings.Contains(errs, "andon open <slug>") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestTooManyArgs(t *testing.T) {
	setup(t, "sample-a")
	if rc, _, errs := run("a", "b"); rc != 2 || !strings.HasPrefix(errs, "usage: andon open") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestNoProjects(t *testing.T) {
	setup(t)
	if rc, _, _ := run(); rc != 1 {
		t.Errorf("rc=%d", rc)
	}
}
