// Package publish は生成したファイルを tray（セルが出した報告を集めて読む場所。ADR-0064）へ出し、オーナーに渡す URL を1行返す。
//
// 順はローカルの tray、プロジェクトが選んでいれば Cloudflare の tray（決めたこと 8）。どちらかに届いた物は files/ に写さない。
// ローカルの tray にも届かなかったときだけ、今までどおり <workspace>/files/ に写し、そのパスを返す。
// 同じ報告は、セルとプロジェクトと名前（<日付>-<slug>）の組で見分ける（決めたこと 9）。同じ日に同じ slug で出し直すと
// 新しいバージョン（中身が同じならバージョンは上がらない）。slug を省くと名前に時刻を足し、いつも別の報告にする。
// ローカルの tray のデータは volume の中にあり、セルの repo と一緒には動かない。動かしたい物は製品の repo か files/ に手で置く。
package publish

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/delegate"
	"andon/internal/ledger"
	"andon/internal/tray"
)

// MaxSize は出せる 1 ファイルの上限（バイト。HTML は参照を埋め込んだあとの大きさ）。files/ は git で追跡する（ADR-0005）ので、
// 大きいもの（動画など）は拒む。LFS は使わない。tray の上限も同じ。
const MaxSize = tray.MaxBytes

const Usage = "usage: andon file publish <file> [slug] [--title <タイトル>]\n" +
	"ファイルを tray へ出し、オーナーに渡す URL を1行出す（Cloudflare の tray に届けばその URL、届かなければローカルの tray の URL）。\n" +
	"名前は <日付>-<slug>。同じ日に同じ slug で出し直すと新しいバージョン（中身が同じなら上がらない）。slug を省くとファイルの名前に時刻を足し、いつも別の報告。\n" +
	"タイトルは --title、無ければ HTML の <title>・Markdown の最初の見出し・ファイルの名前の順。\n" +
	"HTML は相対で引いた画像・スタイル・スクリプト・フォントを埋め込む（http・https の参照は埋めず、数を stderr に出す）。\n" +
	"ローカルの tray にも届かなければ、セルの repo（ANDON_WORKSPACE。無ければ /workspace）の files/ に写してそのパスを出す。tray へ送れなくても終了コードは 0。\n"

type Env struct {
	Workspace string    // セルの repo（ANDON_WORKSPACE。無ければ /workspace）
	Project   string    // プロジェクトの slug（引けなければ tray.NoProject）
	Now       time.Time // 名前の日付と時刻
	Warn      io.Writer // 送れなかった理由・埋めなかった参照（stderr）
}

func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var pos []string
	title := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--title":
			if i+1 >= len(args) || args[i+1] == "" {
				fmt.Fprint(stderr, Usage)
				return 2
			}
			title = args[i+1]
			i++
		case strings.HasPrefix(a, "--"):
			fmt.Fprint(stderr, Usage)
			return 2
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 1 || len(pos) > 2 || pos[0] == "" {
		fmt.Fprint(stderr, Usage)
		return 2
	}
	slug := ""
	if len(pos) == 2 {
		slug = pos[1]
	}
	out, err := Publish(Env{Workspace: delegate.Workspace(), Project: Project(), Now: time.Now(), Warn: stderr}, pos[0], slug, title)
	if err != nil {
		fmt.Fprintf(stderr, "andon file publish: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, out)
	return 0
}

// Project はほかの葉と同じ引き方（--project・KANBAN_ROOT・今いるフォルダ・セルに1つだけ）で引いたプロジェクト。
// 引けなければ tray.NoProject（ローカルの tray にだけ送る。決めたこと 8）。
func Project() string {
	if root := os.Getenv("KANBAN_ROOT"); root != "" {
		return filepath.Base(root)
	}
	if l, err := ledger.Open(); err == nil {
		return l.Slug
	}
	return tray.NoProject
}

// Publish は file を tray へ出し、オーナーに渡す URL を返す。ローカルの tray にも届かなければ files/ に写し、そのパスを返す。
// slug が空なら file の名前から拡張子を除いたものに時刻を足す。
func Publish(env Env, file, slug, title string) (string, error) {
	st, err := os.Stat(file)
	if err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("not a file: %s", file)
	}
	if st.Size() > MaxSize {
		return "", fmt.Errorf("大きすぎる（%d バイト、上限 %d）。files/ は git で追跡するので、大きいものは置かない: %s", st.Size(), MaxSize, file)
	}
	if strings.ContainsRune(slug, '/') {
		return "", fmt.Errorf("slug に / は使えない: %s", slug)
	}
	base := filepath.Base(file)
	ext := filepath.Ext(base)
	name := Name(env.Now, slug)
	if slug == "" {
		name = Name(env.Now, strings.TrimSuffix(base, ext)) + "-" + env.Now.Format("150405.000")
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	ct := tray.ContentType(file)
	if strings.HasPrefix(ct, "text/html") {
		e, err := tray.Embed(file, body)
		if err != nil {
			return "", err
		}
		body = e.Body
		if e.Left > 0 {
			warn(env, "http・https の参照 %d 個は埋めずに残した（報告の面では表示されない）", e.Left)
		}
		if len(e.Missing) > 0 {
			warn(env, "埋められなかった手元の参照: %s", strings.Join(e.Missing, "、"))
		}
	}
	if title == "" {
		title = Title(ct, body, base)
	}
	project := env.Project
	if project == "" {
		project = tray.NoProject
	}
	remote, err := tray.RemoteFor(env.Workspace, project)
	if err != nil {
		warn(env, "Cloudflare の tray の設定を読めないので、ローカルの tray にだけ送る: %v", err)
		remote = nil
	}
	d := tray.Send(tray.Report{Project: project, Name: name, Title: title, Kind: tray.KindPublish, ContentType: ct, Body: body}, remote)
	for _, w := range d.Warnings() {
		warn(env, "%s", w)
	}
	if u := d.URL(); u != "" {
		return u, nil
	}
	// ローカルの tray にも届かなかった: files/ に写す（tray へ送れなくても出す手は止めない）
	dir := filepath.Join(env.Workspace, "files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, name+ext)
	if err := writeFile(dst, body); err != nil {
		return "", err
	}
	warn(env, "tray に届かなかったので files/ に写した")
	return dst, nil
}

// Name は報告の名前（<日付>-<slug>。日付は now の日）。
func Name(now time.Time, slug string) string {
	return now.Format("2006-01-02") + "-" + slug
}

// Title は --title の無いときのタイトル（HTML の <title>、Markdown の最初の見出し、どれも無ければファイルの名前）。
func Title(contentType string, body []byte, base string) string {
	var t string
	switch {
	case strings.HasPrefix(contentType, "text/html"):
		t = tray.HTMLTitle(body)
	case strings.HasPrefix(contentType, "text/markdown"):
		t = tray.MarkdownTitle(body)
	}
	if t == "" {
		return base
	}
	return t
}

func warn(env Env, format string, a ...any) {
	if env.Warn != nil {
		fmt.Fprintf(env.Warn, "andon file publish: "+format+"\n", a...)
	}
}

// writeFile は同じディレクトリの一時ファイルに書いてから rename する。
// dst が出す元と同じファイル（files/ に写したものを出し直す）でも中身を空にしない。
func writeFile(dst string, body []byte) error {
	out, err := os.CreateTemp(filepath.Dir(dst), ".publish-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name()) //nolint:errcheck // rename の後は無いので何もしない
	_, err = out.Write(body)
	if err == nil {
		err = out.Chmod(0o644)
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}
