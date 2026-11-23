// Package publish は生成したファイルを「公開」する: <workspace>/files/ に日付つきでコピーし、URL を返す。
//
// files/ は files サーバ（files/ を ROOT として読み取り公開する静的サーバ）の表玄関。
// 同じ日に同じ slug で公開し直すと上書きになる（再公開）。
package publish

import (
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

// Usage は andon publish の使い方。
const Usage = "usage: andon publish <file> [slug]\n" +
	"セルの repo（ANDON_WORKSPACE。無ければ /workspace）の files/ に日付つきでコピーし、URL を返す。同じ日に同じ slug で打つと上書き。\n"

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
		fmt.Fprint(stderr, Usage)
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
	name := Name(env, slug, ext)
	if err := copyFile(file, filepath.Join(dir, name)); err != nil {
		return "", err
	}
	return URL(env, name), nil
}

// Name は files/ に置く名前（<日付>-<slug><拡張子>。日付は env.Now の日）。
func Name(env Env, slug, ext string) string {
	return env.Now.Format("2006-01-02") + "-" + slug + ext
}

// URL は files/ の name を開く URL（files サーバ。番号はセルの .env の PORTLESS_PORT）。
func URL(env Env, name string) string {
	return fmt.Sprintf("http://files.%s.localhost:%s/%s", env.Hostname, delegate.Dotenv(env.Workspace, "PORTLESS_PORT", DefaultPort), name)
}

// copyFile は同じディレクトリの一時ファイルに書いてから rename する。
// src と dst が同じファイル（公開済みのものを公開し直す）でも中身を空にしない。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // 読むだけのファイルで、閉じる失敗は読んだ中身に効かない
	out, err := os.CreateTemp(filepath.Dir(dst), ".publish-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name()) //nolint:errcheck // rename の後は無いので何もしない
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck // 先の失敗を返すので、片付けの失敗は見ない
		return err
	}
	if err := out.Chmod(0o644); err != nil {
		out.Close() //nolint:errcheck // 先の失敗を返すので、片付けの失敗は見ない
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}
