// Package publish は生成したファイルを「公開」する: <workspace>/files/ に日付つきでコピーし、URL を返す。
//
// files/ は files サーバ（files/ を ROOT として読み取り公開する静的サーバ）の表玄関。
// 同じ日に同じ slug で公開し直すと上書きになる（再公開）。
package publish

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/delegate"
)

// MaxSize は公開できる 1 ファイルの上限（バイト）。files/ は git で追跡する（ADR-0005）ので、
// 大きいもの（動画など）は納品物の面に置かず拒む。LFS は使わない。
const MaxSize = 20 << 20

const usage = "usage: andon publish <file> [slug]\n"

// DefaultPort は .env に PORTLESS_PORT が無いときの番号。
const DefaultPort = "1355"

// Env は公開先を決める値。テストで差し替える。
type Env struct {
	Workspace string    // セルの repo（ANDON_WORKSPACE。無ければ /workspace）
	Hostname  string    // URL のホスト名に使うセルの名前
	Now       time.Time // 日付
}

// Main は andon publish。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 || args[0] == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	host, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(stderr, "andon publish: ホスト名を読めない: %v\n", err)
		return 1
	}
	slug := ""
	if len(args) == 2 {
		slug = args[1]
	}
	url, err := Publish(Env{Workspace: delegate.Workspace(), Hostname: host, Now: time.Now()}, args[0], slug)
	if err != nil {
		fmt.Fprintf(stderr, "andon publish: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, url)
	return 0
}

// Publish は file を <Workspace>/files/<日付>-<slug><拡張子> にコピーし、URL を返す。
// slug が空なら file の名前から拡張子を除いたもの。拡張子の無い file は拡張子を付けない
// （仮設は <日付>-report.report にしていた。意図して変えた）。
func Publish(env Env, file, slug string) (string, error) {
	st, err := os.Stat(file)
	if err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("not a file: %s", file)
	}
	if st.Size() > MaxSize {
		return "", fmt.Errorf("大きすぎる（%d バイト、上限 %d）。files/ は git で追跡するので、大きいものは置かない: %s", st.Size(), MaxSize, file)
	}
	base := filepath.Base(file)
	ext := filepath.Ext(base)
	if slug == "" {
		slug = strings.TrimSuffix(base, ext)
	}
	if strings.ContainsRune(slug, '/') {
		return "", fmt.Errorf("slug に / は使えない: %s", slug)
	}
	dir := filepath.Join(env.Workspace, "files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := env.Now.Format("2006-01-02") + "-" + slug + ext
	if err := copyFile(file, filepath.Join(dir, name)); err != nil {
		return "", err
	}
	return fmt.Sprintf("http://files.%s.localhost:%s/%s", env.Hostname, port(filepath.Join(env.Workspace, ".env")), name), nil
}

// copyFile は同じディレクトリの一時ファイルに書いてから rename する。
// src と dst が同じファイル（公開済みのものを公開し直す）でも中身を空にしない。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(dst), ".publish-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name()) // rename の後は無いので何もしない
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Chmod(0o644); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}

// port は .env の最後の PORTLESS_PORT= の値（引用符と空白を除く）。無ければ DefaultPort。
func port(envFile string) string {
	f, err := os.Open(envFile)
	if err != nil {
		return DefaultPort
	}
	defer f.Close()
	v := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "PORTLESS_PORT="); ok {
			v = rest
		}
	}
	// 仮設（cut -d= -f2）と同じく、2つ目の = より後は捨てる。
	v, _, _ = strings.Cut(v, "=")
	v = strings.Map(func(r rune) rune {
		if r == '"' || r == '\'' || r == ' ' {
			return -1
		}
		return r
	}, v)
	if v == "" {
		return DefaultPort
	}
	return v
}
