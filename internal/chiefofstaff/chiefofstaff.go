// Package chiefofstaff はプロジェクトの Chief of Staff を起動する（動いていれば、そのまま使う）。打つ口は andon open（internal/open）。
// 最後に出た場所を「<Space 名> / <tab 名>」で出す。
//
// することは4つ（1〜3 は <slug>.cos.lock を持って1つずつ）:
//  1. ラベル role=chief-of-staff project=<slug> の、動いている端末を探す。1つならそれを使う（名前と cwd が合うときだけ）。
//     2つ以上なら止まる（1プロジェクトに1人）。数えるのは hc の agent が Claude Code の端末だけ。
//     作り直しで戻った端末（top-level の argv ごと素のシェル）は数えず片付ける。どちらとも言えない端末があれば止まる。
//  2. 無ければプロファイル chief-of-staff で起動する（名前 <slug>-cos・cwd は kanban の root）。
//     ラベル herdr.space=andon-<slug>・herdr.tab=andon-<slug>-cos でオーナーの Herdr App の指揮側の Space に出す（ADR-0002 決めたこと 6・10）。
//  3. その address を kanban の config.yaml の chief_of_staff に書く（違うときだけ。kanban config set）。
//  4. 出た場所を「<Space 名> / <tab 名>」（<slug> / CoS）で出す。hc attach は打たない（ADR-0002 決めたこと 14・17）。
//
// 打つのはオーナー（プロジェクトで Chief of Staff と話すとき）か、起き直した Chief of Staff 自身（自分が見つかるので起動し直さない）。
package chiefofstaff

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"andon/internal/dispatcher"
	"andon/internal/flock"
	"andon/internal/launch"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// Profile は Chief of Staff のプロファイル（hc の template の名前）。
const Profile = "chief-of-staff"

const (
	exitFail  = 1
	exitFirst = 3 // 起きたが、最初のメッセージが届いていない
)

// Start は dir（空なら今のディレクトリ）から kanban を探して、そのプロジェクトの Chief of Staff を起動する／使う。
// andon open <slug> が projects/<slug> を指して呼び、andon open が slug なしで今いるプロジェクトを開くときは空で呼ぶ（cd はしない）。
func Start(dir string, stdout, stderr io.Writer) int {
	die := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "andon open: "+format+"\n", a...)
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
		return die("kanban が見つからない（andon open <slug> でプロジェクトを指す）")
	}
	l := ledger.At(root)
	slug := l.Slug
	name := slug + "-cos"
	if !spawn.SlugRe.MatchString(slug) || !spawn.NameRe.MatchString(name) {
		return die("slug が不正: '%s'（kanban の root のフォルダ名。英小文字・数字・_・-、28字まで）", slug)
	}
	config := filepath.Join(l.Dir, "config.yaml")
	if fi, err := os.Lstat(config); err != nil || !fi.Mode().IsRegular() {
		return die("%s が無いか symlink", config)
	}

	// 1プロジェクトに1人。数えてから起動して config.yaml に書くまで lock を持つ（同時に2本打っても2人にしない。G5 を塞いだ。
	// dispatcher.md §7.2 #10）。後の方は待ち、先の方が起動した端末を数えて使う
	unlock, err := flock.Lock(Lock(slug))
	if err != nil {
		return die("%v", err)
	}
	defer unlock()
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
	// 動いているのは hc の agent が Claude Code のときだけ。どちらとも言えなければ、片付けも使いもせず止まる
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
			return die("%s（%s の Chief of Staff のラベル）で Claude Code が動いているか確かめられないので止まる（%s）。オーナーが使っていれば済ませてから、戻った端末なら hc kill %s してから打ち直す", lt.Address, slug, t.Describe(), lt.Address)
		}
	}
	if len(addrs) > 1 {
		return die("%s の Chief of Staff が %d 人いる: %s（1人に減らす）", slug, len(addrs), strings.Join(addrs, " "))
	}
	for _, a := range shells {
		if err := spawn.Discard(a); err != nil {
			return die("%s（Chief of Staff のラベルで中身がシェル）を片付けられない: %v", a, err)
		}
		fmt.Fprintf(stdout, "andon open: %s は Chief of Staff のラベルのまま中身がシェルに替わっていた（作り直しで戻った端末）ので片付けた\n", a)
	}

	space, tab := "andon-"+slug, "andon-"+slug+"-cos"
	where := slug + " / CoS" // オーナーに端末を指す言い方（ADR-0002 決めたこと 16・17）。名前は App の mod が付ける
	msg := fmt.Sprintf("プロジェクト `%s` の Chief of Staff。\nkanban は `%s`。", slug, l.Dir)
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
			// ラベルを付ける前に起きた端末。付け直さない（置き場は起動するときのラベルだけで決める）
			where = "App の Space の外（herdr.space・herdr.tab のラベルより前に起きた端末）"
		}
	} else {
		command, found, err := spawn.TemplateCommand(Profile)
		if err != nil {
			return die("%v", err)
		}
		if !found {
			return die("プロファイル %s の template が無い（hc templates で確かめる。作るのはオーナー）", Profile)
		}
		// skill chief-of-staff はプロファイルの command が system prompt に足すので、最初のメッセージは読ませる行を持たない（ADR-0001 決定 3）
		rec := launch.Gather(launch.Spec{Role: launch.RoleChiefOfStaff, Launch: launch.LaunchStart, Profile: Profile, Template: Profile,
			Command: &command, Brand: ledger.Brand(Profile)})
		run, err = spawn.Run(name, []string{"role=" + Profile, "project=" + slug, "herdr.space=" + space, "herdr.tab=" + tab}, root, Profile, msg)
		if err != nil {
			return die("%v", err)
		}
		rec.Terminal = run.Address
		if err := launch.Append(launch.ChiefOfStaffPath(l.Dir), rec); err != nil {
			fmt.Fprintf(stderr, "andon open: 起動の記録を書けない（起動は続ける）: %v\n", err)
		}
		addr, how = run.Address, "起動した"
	}

	if l.ChiefOfStaff() != addr {
		if _, err := spawn.Output([]string{"KANBAN_ROOT=" + root}, "", cli, "config", "set", "chief_of_staff="+addr); err != nil {
			return die("%s config set chief_of_staff=%s に失敗: %v", cli, addr, err)
		}
	}

	if run != nil && !run.First.Delivered() {
		fmt.Fprintf(stderr, "andon open: %s（%s）に最初のメッセージが届いていない: %s。App の %s で答えてから、次を送る: %s\n",
			addr, name, run.Reason(), where, spawn.SendCommand(addr, Profile, msg))
		fmt.Fprintf(stdout, "andon open: 出た場所 %s（%s）\n", where, addr)
		return exitFirst
	}
	fmt.Fprintf(stdout, "andon open: %s の Chief of Staff は %s（%s・%s）。config.yaml の chief_of_staff: %s\n", slug, addr, name, how, addr)
	fmt.Fprintf(stdout, "andon open: 次: 出た場所の端末で Chief of Staff に話す。オーナー待ちは andon waiting list --project %s で出す\n", slug)
	fmt.Fprintf(stdout, "andon open: 出た場所 %s（%s）\n", where, addr)
	return 0
}

// Lock は andon open が「数える→起動する→config.yaml に書く」の間持つ lock のパス（控えの置き場の <slug>.cos.lock）。
func Lock(slug string) string {
	return filepath.Join(filepath.Dir(dispatcher.StatePath(slug)), slug+".cos.lock")
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
