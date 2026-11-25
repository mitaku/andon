// Package tools は案件の repo の mise.toml の宣言を満たす手（ADR-0063 決めたこと 6・7・8）と、
// 役の端末へ worktree の mise.toml をいちばん弱い既定として渡す葉 andon tools env（決めたこと 10）を持つ。
//
// 「宣言を満たす」は、repo の mise.toml が名指す道具とバージョンを、その repo の根で mise install を打って入れること。
// 起動の口（andon issue start・resume・andon chief-engineer・dispatcher がレビュアーを起動する口）は Satisfy を、
// gate の検査は Install を、hc run と検査の command の前に呼ぶ。
package tools

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Manifest は宣言のファイルの名前。正は mise.toml だけ（ADR-0063 決めたこと 2。.tool-versions などは見ない）。
const Manifest = "mise.toml"

// EnvKey は役の端末と gate の検査へ渡す環境変数。mise の設定の中でいちばん先に読まれ、あとの設定に負ける（決めたこと 10）。
const EnvKey = "MISE_SYSTEM_CONFIG_FILE"

// Limit は mise install を待つ上限（決めたこと 7）。過ぎても mise install は止めない。検査が短くする。
var Limit = 60 * time.Second

// ErrTimeout は Limit を過ぎても mise install が終わらなかったこと。mise install は止めずに手放してある。
var ErrTimeout = errors.New("mise install が上限の時間を過ぎても終わらない")

// allInstalled は mise install が何も入れなかったときに出す行（mise 2026.10.23 で実測。入れた回には出ない）。
const allInstalled = "all tools are installed"

// HasManifest は root の直下に mise.toml（ファイル）があるか。
func HasManifest(root string) bool {
	if root == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(root, Manifest))
	return err == nil && st.Mode().IsRegular()
}

// Install は root に mise.toml があり、mise が PATH にあれば、root を cwd にして mise install を打つ。
// 無ければ何も打たずに (false, nil)。installed は何かを入れたか。Limit を過ぎたら mise install を止めずに ErrTimeout を返す
// （止めると入れかけが残る。手放した mise install はそのまま入れ終え、同時に打たれた別の mise install は終わるのを待つ）。
// 出力は名前を消した一時のファイルに受ける。pipe に受けると、手放したあと呼び手が終わったときに書き先が閉じ、mise install が落ちる。
func Install(root string) (installed bool, err error) {
	if !HasManifest(root) {
		return false, nil
	}
	mise, err := exec.LookPath("mise")
	if err != nil {
		return false, nil
	}
	out, err := os.CreateTemp("", "andon-mise-install-")
	if err != nil {
		return false, err
	}
	defer out.Close()           //nolint:errcheck // 読み終えた一時のファイルを閉じるだけ
	defer os.Remove(out.Name()) //nolint:errcheck // 名前を消すだけ。手放した mise install は消えたファイルへ書き続ける
	c := exec.Command(mise, "install")
	c.Dir = root
	c.Stdout, c.Stderr = out, out
	// 新しいセッションにする: 呼び手の端末が閉じても、手放した mise install に信号が届かない
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return false, err
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }() // 手放したあとも Wait が子を回収する
	timer := time.NewTimer(Limit)
	defer timer.Stop()
	select {
	case werr := <-done:
		b, _ := os.ReadFile(out.Name())
		if werr != nil {
			return false, fmt.Errorf("mise install が失敗（%v）%s", werr, errorLine(b))
		}
		return !bytes.Contains(b, []byte(allInstalled)), nil
	case <-timer.C:
		return false, ErrTimeout
	}
}

// errorLine は mise の出力のうち、最初の ERROR の行（無ければ最後の行）を「: <行>」の形で返す。出力が空なら空。
func errorLine(b []byte) string {
	var last string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.Contains(l, "ERROR") {
			return ": " + l
		}
		last = l
	}
	if last == "" {
		return ""
	}
	return ": " + last
}

// Satisfy は起動の口が hc run の前に呼ぶ（決めたこと 6・7）。root で Install し、stderr に出す 1 行を返す（出すことが無ければ空）。
// 入れたときは入れたこと、満たせなかったときは警告と打ち直す command。どちらでも起動は止めない。
func Satisfy(root string) string {
	installed, err := Install(root)
	switch {
	case errors.Is(err, ErrTimeout):
		return fmt.Sprintf("%s の %s の道具が %s で入りきらない（mise install は止めずに続けさせ、起動へ進む）。道具が引けなければ、worktree の中で mise install を打つ（cd %s && mise install）", root, Manifest, Limit, root)
	case err != nil:
		return fmt.Sprintf("%s の %s の道具を入れられない（起動は続ける）: %v。道具が要るなら、worktree の中で mise install を打ち直す（cd %s && mise install）", root, Manifest, err, root)
	case installed:
		return fmt.Sprintf("%s の %s の道具を入れた（mise install）", root, Manifest)
	}
	return ""
}

// Warn は Satisfy の 1 行を prefix をつけて w に出す（空なら何も出さない）。
func Warn(w io.Writer, prefix, root string) {
	if line := Satisfy(root); line != "" {
		fmt.Fprintf(w, "%s: %s\n", prefix, line)
	}
}

// EnvFor は検査の command に足す環境変数（root に mise.toml があるときだけ。無ければ nil）。
func EnvFor(root string) []string {
	if !HasManifest(root) {
		return nil
	}
	return []string{EnvKey + "=" + filepath.Join(root, Manifest)}
}
