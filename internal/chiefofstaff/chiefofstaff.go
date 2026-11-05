// Package chiefofstaff は andon chief-of-staff: プロジェクトの Chief of Staff を起こす（動いていれば、そのまま使う）。
//
//	andon chief-of-staff              projects/<slug>/ で打つ。終わりに hc attach でその画面に乗る
//	andon chief-of-staff --no-attach  乗らずに終わる（Chief of Staff 自身が起きたときに打つ）
//
// することは4つ:
//  1. ラベル role=chief-of-staff project=<slug> の、動いている端末を探す。1つならそれを使う（名前と cwd が合うときだけ）。
//     2つ以上なら止まる（1プロジェクトに1人）。
//  2. 無ければプロファイル chief-of-staff で起こす（名前 <slug>-cos・cwd は台帳の root）。
//  3. その address を台帳の config.yaml の coordinator に書く（違うときだけ。kanban config set）。
//  4. --no-attach でなく、端末から打ったとき（stdin と stdout が TTY）は hc attach で画面に乗る。
//
// 打つのは人（プロジェクトで Chief of Staff と話すとき）か、起き直した Chief of Staff 自身（--no-attach。自分が見つかるので起こし直さない）。
package chiefofstaff

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"andon/internal/ledger"
	"andon/internal/spawn"
)

// Usage は andon chief-of-staff の使い方。
const Usage = "usage: andon chief-of-staff              projects/<slug>/ で打つ。終わりに hc attach でその画面に乗る\n" +
	"       andon chief-of-staff --no-attach  乗らずに終わる（Chief of Staff 自身が起きたときに打つ）\n" +
	"動いていればそれを使い、無ければプロファイル chief-of-staff で起こす（名前 <slug>-cos）。\n" +
	"台帳の config.yaml の coordinator をその address に合わせる。2人いれば止まる（1プロジェクトに1人）。\n" +
	"最初のメッセージが届かなかった（信頼の画面など）ときは終了コード 3 と、画面に答えたあとに打つ hc send を出す。\n"

// Profile は Chief of Staff のプロファイル（hc の template の名前）。
const Profile = "chief-of-staff"

const (
	exitFail  = 1
	exitUsage = 2
	exitFirst = 3 // 起きたが、最初のメッセージが届いていない
)

// isTTY は端末から打たれたか（テストでは差し替える）。
var isTTY = func(stdin io.Reader, stdout io.Writer) bool {
	tty := func(v any) bool {
		f, ok := v.(*os.File)
		if !ok {
			return false
		}
		fi, err := f.Stat()
		return err == nil && fi.Mode()&os.ModeCharDevice != 0
	}
	return tty(stdin) && tty(stdout)
}

// Main は andon chief-of-staff を走らせ、終了コードを返す。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	attach := true
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--no-attach":
		attach = false
	default:
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	attach = attach && isTTY(stdin, stdout)
	die := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "andon chief-of-staff: "+format+"\n", a...)
		return exitFail
	}

	cli := ledger.CLI
	if err := ledger.Check(); err != nil {
		return die("%v", err)
	}
	if _, err := exec.LookPath("hc"); err != nil {
		return die("hc が無い")
	}
	root, err := spawn.Output(nil, "", cli, "root")
	if err != nil || root == "" {
		return die("台帳が見つからない（projects/<slug>/ で打つ）")
	}
	l := ledger.At(root)
	slug := l.Slug
	name := slug + "-cos"
	if !spawn.SlugRe.MatchString(slug) || !spawn.NameRe.MatchString(name) {
		return die("slug が不正: '%s'（台帳の root のフォルダ名。英小文字・数字・_・-、28字まで）", slug)
	}
	config := filepath.Join(l.Dir, "config.yaml")
	if fi, err := os.Lstat(config); err != nil || !fi.Mode().IsRegular() {
		return die("%s が無いか symlink", config)
	}

	// 1プロジェクトに1人
	ls, err := spawn.Output(nil, "", "hc", "ls", "--json", "--label", "role="+Profile, "--label", "project="+slug)
	if err != nil {
		return die("hc ls に失敗: %v", err)
	}
	var live struct {
		Terminals []struct {
			Address string `json:"address"`
		} `json:"terminals"`
	}
	if err := json.Unmarshal([]byte(ls), &live); err != nil {
		return die("hc ls の返事が読めない: %v", err)
	}
	var addrs []string
	for _, t := range live.Terminals {
		addrs = append(addrs, t.Address)
	}
	if len(addrs) > 1 {
		return die("%s の Chief of Staff が %d 人いる: %s（1人に減らす）", slug, len(addrs), strings.Join(addrs, " "))
	}

	msg := fmt.Sprintf("プロジェクト `%s` の Chief of Staff。\n台帳は `%s`。", slug, l.Dir)
	var addr, how string
	var run *spawn.Started
	if len(addrs) == 1 {
		addr, how = addrs[0], "動いているものを使う"
		// ラベルだけでは決めない: 名前と cwd が合うものだけを使う（ほかのプロジェクトの端末を config.yaml に書かない）
		got, err := spawn.Output(nil, "", "hc", "get", addr, "--json")
		if err != nil {
			return die("hc get %s に失敗: %v", addr, err)
		}
		var t struct {
			Name string `json:"name"`
			Cwd  string `json:"cwd"`
		}
		if err := json.Unmarshal([]byte(got), &t); err != nil {
			return die("hc get %s の返事が読めない: %v", addr, err)
		}
		if t.Name != name || t.Cwd == "" || !sameDir(t.Cwd, root) {
			return die("%s はラベルが %s の Chief of Staff だが、名前（%s）か cwd（%s）が合わない（名前 %s・cwd %s のはず）。hc ls で確かめる", addr, slug, t.Name, t.Cwd, name, root)
		}
	} else {
		found, err := spawn.HasTemplate(Profile)
		if err != nil {
			return die("%v", err)
		}
		if !found {
			return die("プロファイル %s の template が無い（hc templates で確かめる。作るのは人）", Profile)
		}
		// skill chief-of-staff はプロファイルの command が system prompt に足すので、最初のメッセージは読ませる行を持たない（ADR-0001 決定 3）
		run, err = spawn.Run(name, []string{"role=" + Profile, "project=" + slug}, root, Profile, msg)
		if err != nil {
			return die("%v", err)
		}
		addr, how = run.Address, "起こした"
	}

	if l.Coordinator() != addr {
		if _, err := spawn.Output([]string{"FLIGHTDECK_ROOT=" + root}, "", cli, "config", "set", "coordinator="+addr); err != nil {
			return die("%s config set coordinator=%s に失敗: %v", cli, addr, err)
		}
	}

	switch {
	case run != nil && !run.First.Sent && attach:
		// 乗れば画面に答えられるので、案内を出して乗る
		fmt.Fprintf(stderr, "andon chief-of-staff: %s（%s）に最初のメッセージが届いていない: %s。画面で答えてから、次を送る: hc send %s %q\n",
			addr, name, run.Reason(), addr, msg)
	case run != nil && !run.First.Sent:
		fmt.Fprintf(stderr, "andon chief-of-staff: %s（%s）に最初のメッセージが届いていない: %s。hc attach %s で画面に答えてから、次を送る: hc send %s %q\n",
			addr, name, run.Reason(), addr, addr, msg)
		return exitFirst
	default:
		fmt.Fprintf(stdout, "andon chief-of-staff: %s の Chief of Staff は %s（%s・%s）。config.yaml の coordinator: %s\n", slug, addr, name, how, addr)
	}
	if !attach {
		fmt.Fprintf(stdout, "andon chief-of-staff: 話すには hc attach %s\n", addr)
		return 0
	}
	fmt.Fprintf(stdout, "andon chief-of-staff: %s に乗る。抜けるのは ctrl+\\\n", addr)
	c := exec.Command("hc", "attach", addr)
	c.Stdin, c.Stdout, c.Stderr = stdin, stdout, stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return die("hc attach %s に失敗: %v", addr, err)
	}
	return 0
}

// sameDir は a と b が同じディレクトリか（symlink を解いて比べる）。
func sameDir(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}
