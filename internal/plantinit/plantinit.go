// Package plantinit は andon init <slug>（docs/design.md §2.2・§5.2 の P4）。
// 埋め込んだ雛形（repo の init/）をセルの repo（既定 ./<slug>-cell）に写す。生成後は追従しない。
// 名前が .tmpl のものは text/template で埋めて拡張子を外す。bin/ の下と *.sh は実行できる形で置く。
package plantinit

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"andon"
)

// Usage は andon init --help の中身。
const Usage = "usage: andon init <slug> [--dir <path>] [--image <ref>] [--owner <email>]\n" +
	"セルの repo の雛形を <path>（既定 ./<slug>-cell）に作る。<path> がすでにあって空でなければ何もしない。\n" +
	"  --image  Dockerfile の FROM（既定は埋める場所だけを置く。bin/bootstrap は埋めるまで動かない）\n" +
	"  --owner  andon.toml の owner（委譲のファイルを信じる commit の author の email）\n" +
	"できたら: cd <path> → Dockerfile の FROM を確かめる → bin/bootstrap\n"

// PlaceholderImage は --image が無いときの FROM。bin/bootstrap は < > が残っていれば止まる。
const PlaceholderImage = "ghcr.io/<owner>/andon:<版>"

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Options は雛形を埋める値。
type Options struct {
	Slug  string
	Dir   string
	Image string
	Owner string
}

// Main は andon init。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("andon init", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	var o Options
	fl.StringVar(&o.Dir, "dir", "", "")
	fl.StringVar(&o.Image, "image", PlaceholderImage, "")
	fl.StringVar(&o.Owner, "owner", "", "")
	// slug を先にも後にも書けるように、flag でない引数を1つだけ拾う
	var pos []string
	for rest := args; ; {
		if err := fl.Parse(rest); err != nil {
			fmt.Fprintf(stderr, "andon init: %v\n%s", err, Usage)
			return 2
		}
		if fl.NArg() == 0 {
			break
		}
		pos = append(pos, fl.Arg(0))
		rest = fl.Args()[1:]
	}
	if len(pos) != 1 {
		fmt.Fprint(stderr, Usage)
		return 2
	}
	o.Slug = pos[0]
	if o.Dir == "" {
		o.Dir = o.Slug + "-cell"
	}
	if err := Init(o); err != nil {
		fmt.Fprintf(stderr, "andon init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "セルの repo を作った: %s\n次: cd %s → Dockerfile の FROM を確かめる → bin/bootstrap\n", o.Dir, o.Dir)
	if o.Image == PlaceholderImage {
		fmt.Fprintf(stdout, "Dockerfile の FROM はまだ埋まっていない（%s）\n", PlaceholderImage)
	}
	return 0
}

// Init は o.Dir に雛形を写す。o.Dir がすでにあり空でなければ、何も書かずに誤りを返す。
func Init(o Options) error {
	if !slugRe.MatchString(o.Slug) {
		return fmt.Errorf("slug は英小文字・数字・- で、英小文字か数字で始める: %q", o.Slug)
	}
	if strings.ContainsAny(o.Image, " \t\r\n") || o.Image == "" {
		return fmt.Errorf("--image が空か、空白を含む: %q", o.Image)
	}
	if strings.ContainsAny(o.Owner, "\"\\\r\n") {
		return fmt.Errorf("--owner に \" \\ 改行を入れない: %q", o.Owner)
	}
	if ents, err := os.ReadDir(o.Dir); err == nil && len(ents) > 0 {
		return fmt.Errorf("%s はすでにあり、空でない（上書きしない）", o.Dir)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	src, err := fs.Sub(andon.Init, "init")
	if err != nil {
		return err
	}
	return fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(o.Dir, filepath.FromSlash(strings.TrimSuffix(p, ".tmpl")))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if strings.HasSuffix(p, ".tmpl") {
			t, err := template.New(p).Option("missingkey=error").Parse(string(b))
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if err := t.Execute(&buf, o); err != nil {
				return err
			}
			b = buf.Bytes()
		}
		mode := fs.FileMode(0o644)
		if strings.HasPrefix(p, "bin/") || path.Ext(p) == ".sh" {
			mode = 0o755
		}
		return os.WriteFile(dst, b, mode)
	})
}
