package plantinit

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon"
)

func TestInitWritesEveryTemplateFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x-cell")
	if err := Init(Options{Slug: "x", Dir: dir, Image: "example.com/andon:test", Owner: "dev@example.com"}); err != nil {
		t.Fatal(err)
	}
	src, _ := fs.Sub(andon.Init, "init")
	n := 0
	fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		n++
		dst := filepath.Join(dir, strings.TrimSuffix(p, ".tmpl"))
		st, err := os.Stat(dst)
		if err != nil {
			t.Errorf("%s が無い: %v", dst, err)
			return nil
		}
		exec := st.Mode()&0o100 != 0
		if want := strings.HasPrefix(p, "bin/") || strings.HasSuffix(p, ".sh"); exec != want {
			t.Errorf("%s: 実行の権限 %v（want %v）", p, exec, want)
		}
		b, _ := os.ReadFile(dst)
		if strings.Contains(string(b), "{{") {
			t.Errorf("%s に埋めていない所がある", p)
		}
		return nil
	})
	if n < 9 {
		t.Errorf("雛形のファイルが %d しか無い", n)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if !bytes.HasPrefix(b, []byte("FROM example.com/andon:test\n")) || bytes.Count(b, []byte("\n")) != 3 {
		t.Errorf("Dockerfile:\n%s", b)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "andon.toml"))
	if !bytes.Contains(b, []byte("\nslug = \"x\"\n")) || !bytes.Contains(b, []byte("\nowner = \"dev@example.com\"\n")) {
		t.Errorf("andon.toml:\n%s", b)
	}
}

func TestInitRefuses(t *testing.T) {
	full := t.TempDir()
	os.WriteFile(filepath.Join(full, "keep"), []byte("人のもの"), 0o644)
	for name, o := range map[string]Options{
		"slug が大文字":     {Slug: "X", Dir: filepath.Join(t.TempDir(), "a"), Image: "i"},
		"slug が - で始まる": {Slug: "-x", Dir: filepath.Join(t.TempDir(), "a"), Image: "i"},
		"空でないディレクトリ":    {Slug: "x", Dir: full, Image: "i"},
		"image に空白":     {Slug: "x", Dir: filepath.Join(t.TempDir(), "a"), Image: "a b"},
		"owner に引用符":    {Slug: "x", Dir: filepath.Join(t.TempDir(), "a"), Image: "i", Owner: `a"b`},
	} {
		if err := Init(o); err == nil {
			t.Errorf("%s: 通った", name)
		}
		if name != "空でないディレクトリ" {
			if _, err := os.Stat(o.Dir); err == nil {
				t.Errorf("%s: %s を作った", name, o.Dir)
			}
		}
	}
	if ents, _ := os.ReadDir(full); len(ents) != 1 {
		t.Errorf("空でないディレクトリに書いた: %v", ents)
	}
}

func TestMainArgs(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errb bytes.Buffer
	if rc := Main([]string{"--image", "example.com/andon:1", "y"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errb.String())
	}
	if _, err := os.Stat("y-cell/bin/bootstrap"); err != nil {
		t.Error(err)
	}
	if rc := Main([]string{"z"}, nil, &out, &errb); rc != 0 || !strings.Contains(out.String(), "まだ埋まっていない") {
		t.Errorf("既定の image: rc=%d out=%s", rc, out.String())
	}
	for _, args := range [][]string{nil, {"a", "b"}, {"--bogus", "a"}} {
		errb.Reset()
		if rc := Main(args, nil, &out, &errb); rc != 2 || !strings.Contains(errb.String(), "usage: andon init") {
			t.Errorf("%v: rc=%d err=%s", args, rc, errb.String())
		}
	}
}
