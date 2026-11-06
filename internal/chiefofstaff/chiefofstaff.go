// Package chiefofstaff は andon chief-of-staff: プロジェクトの Chief of Staff を起こす（動いていれば、そのまま使う）。
//
//	andon chief-of-staff   projects/<slug>/ で打つ。最後に出た場所を「<Space 名> / <tab 名>」で出す
//
// することは4つ:
//  1. ラベル role=chief-of-staff project=<slug> の、動いている端末を探す。1つならそれを使う（名前と cwd が合うときだけ）。
//     2つ以上なら止まる（1プロジェクトに1人）。数えるのは hc の agent が Claude Code の端末だけ。
//     作り直しで戻った端末（top-level の argv ごと素のシェル）は数えず片付ける。どちらとも言えない端末があれば止まる。
//  2. 無ければプロファイル chief-of-staff で起こす（名前 <slug>-cos・cwd は台帳の root）。
//     ラベル herdr.space=andon-<slug>・herdr.tab=andon-<slug>-cos で人の Herdr App の指揮側の Space に出す（ADR-0002 決めたこと 6・10）。
//  3. その address を台帳の config.yaml の chief_of_staff に書く（違うときだけ。kanban config set）。
//  4. 出た場所を「<Space 名> / <tab 名>」（<slug> / CoS）で出す。hc attach は打たない（ADR-0002 決めたこと 14・17）。
//
// 打つのは人（プロジェクトで Chief of Staff と話すとき）か、起き直した Chief of Staff 自身（自分が見つかるので起こし直さない）。
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
const Usage = "usage: andon chief-of-staff   projects/<slug>/ で打つ。人の Herdr App の <slug> / CoS に出る\n" +
	"動いていればそれを使い、無ければプロファイル chief-of-staff で起こす（名前 <slug>-cos）。\n" +
	"台帳の config.yaml の chief_of_staff をその address に合わせる。2人いれば止まる（1プロジェクトに1人）。\n" +
	"最初のメッセージが届かなかった（信頼の画面など）ときは終了コード 3 と、App のその tab で答えたあとに打つ hc send を出す。\n"

// Profile は Chief of Staff のプロファイル（hc の template の名前）。
const Profile = "chief-of-staff"

const (
	exitFail  = 1
	exitUsage = 2
	exitFirst = 3 // 起きたが、最初のメッセージが届いていない
)

// Main は andon chief-of-staff を走らせ、終了コードを返す。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	return Start("", stdout, stderr)
}

// Start は dir（空なら今のディレクトリ）から台帳を探して、そのプロジェクトの Chief of Staff を起こす／使う。
// andon open <slug> が projects/<slug> を指して呼ぶ（cd はしない）。
func Start(dir string, stdout, stderr io.Writer) int {
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
	root, err := spawn.OutputIn(dir, nil, "", cli, "root")
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
	// ラベルと live だけでは決めない: 作り直しで戻った端末は、ラベルはそのままで top-level の argv ごと素のシェル。
	// 動いているは hc の agent が Claude Code のときだけ。どちらとも言えなければ、片付けも使いもせず止まる
	var addrs, shells []string
	var cur *spawn.Term
	for _, lt := range live.Terminals {
		t, err := spawn.Look(lt.Address)
		if err != nil {
			return die("%v", err)
		}
		switch t.Kind() {
		case spawn.Restored:
			shells = append(shells, lt.Address)
		case spawn.Running:
			addrs, cur = append(addrs, lt.Address), t
		default:
			return die("%s（%s の Chief of Staff のラベル）で Claude Code が動いているか確かめられないので止まる（%s）。人が使っていれば済ませてから、戻った端末なら hc kill %s してから打ち直す", lt.Address, slug, t.Describe(), lt.Address)
		}
	}
	if len(addrs) > 1 {
		return die("%s の Chief of Staff が %d 人いる: %s（1人に減らす）", slug, len(addrs), strings.Join(addrs, " "))
	}
	for _, a := range shells {
		if err := spawn.Discard(a); err != nil {
			return die("%s（Chief of Staff のラベルで中身がシェル）を片付けられない: %v", a, err)
		}
		fmt.Fprintf(stdout, "andon chief-of-staff: %s は Chief of Staff のラベルのまま中身がシェルに替わっていた（作り直しで戻った端末）ので片付けた\n", a)
	}

	space, tab := "andon-"+slug, "andon-"+slug+"-cos"
	where := slug + " / CoS" // 人に端末を指す言い方（ADR-0002 決めたこと 16・17）。名前は App の mod が付ける
	msg := fmt.Sprintf("プロジェクト `%s` の Chief of Staff。\n台帳は `%s`。", slug, l.Dir)
	var addr, how string
	var run *spawn.Started
	if len(addrs) == 1 {
		addr, how = addrs[0], "動いているものを使う"
		// ラベルだけでは決めない: 名前と cwd が合うものだけを使う（ほかのプロジェクトの端末を config.yaml に書かない）
		t := cur
		if t.Name != name || t.Cwd == "" || !sameDir(t.Cwd, root) {
			return die("%s はラベルが %s の Chief of Staff だが、名前（%s）か cwd（%s）が合わない（名前 %s・cwd %s のはず）。hc ls で確かめる", addr, slug, t.Name, t.Cwd, name, root)
		}
		if t.Labels["herdr.space"] != space || t.Labels["herdr.tab"] != tab {
			// ラベルを付ける前に起きた端末。付け直さない（置き場は起こすときのラベルだけで決める）
			where = "App の Space の外（herdr.space・herdr.tab のラベルより前に起きた端末）"
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
		run, err = spawn.Run(name, []string{"role=" + Profile, "project=" + slug, "herdr.space=" + space, "herdr.tab=" + tab}, root, Profile, msg)
		if err != nil {
			return die("%v", err)
		}
		addr, how = run.Address, "起こした"
	}

	if l.ChiefOfStaff() != addr {
		if _, err := spawn.Output([]string{"KANBAN_ROOT=" + root}, "", cli, "config", "set", "chief_of_staff="+addr); err != nil {
			return die("%s config set chief_of_staff=%s に失敗: %v", cli, addr, err)
		}
	}

	if run != nil && !run.First.Delivered() {
		fmt.Fprintf(stderr, "andon chief-of-staff: %s（%s）に最初のメッセージが届いていない: %s。App の %s で答えてから、次を送る: hc send %s %q\n",
			addr, name, run.Reason(), where, addr, msg)
		fmt.Fprintf(stdout, "andon chief-of-staff: 出た場所 %s（%s）\n", where, addr)
		return exitFirst
	}
	fmt.Fprintf(stdout, "andon chief-of-staff: %s の Chief of Staff は %s（%s・%s）。config.yaml の chief_of_staff: %s\n", slug, addr, name, how, addr)
	fmt.Fprintf(stdout, "andon chief-of-staff: 出た場所 %s（%s）\n", where, addr)
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
