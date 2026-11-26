// Package open は andon open [slug]: プロジェクトの Chief of Staff を開く（動いていれば、そのまま使う）と、
// andon project list: セルのプロジェクトを並べる。
//
//	andon open <slug>   projects/<slug>/ の Chief of Staff を開く（cd はしない）
//	andon open          今いるプロジェクト（kanban の CLI が見つける kanban）の Chief of Staff を開く。
//	                    見つからなければ、kanban のあるプロジェクトの一覧から fzf で選んで開く
//
// 選ぶときに fzf が無いか端末でなければ、一覧（Chief of Staff が動いているかつき）を出して終了コード 2。
package open

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"andon/internal/chiefofstaff"
	"andon/internal/delegate"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

const Usage = "usage: andon open [slug]\n" +
	"プロジェクトの Chief of Staff を開く。オーナーの Herdr App の <slug> / CoS に出る。cd は要らない。\n" +
	"動いていればそれを使い、無ければプロファイル chief-of-staff で起動する（名前 <slug>-cos）。\n" +
	"kanban の config.yaml の chief_of_staff をその address に合わせる。2人いれば止まる（1プロジェクトに1人）。\n" +
	"slug を省くと、今いるプロジェクト（KANBAN_ROOT か、上のフォルダの kanban）を開く。起き直した Chief of Staff は自分の端末でこれを打つ。\n" +
	"今いるプロジェクトが無ければ、kanban のあるプロジェクトの一覧から fzf で選ぶ（横に Chief of Staff が動いているかを出す）。\n" +
	"fzf が無いか端末でないときは、一覧を出して終了コード 2。\n" +
	"最初のメッセージが届かなかった（信頼の画面など）ときは終了コード 3 と、App のその tab で答えたあとに打つ hc send を出す。\n"

const ListUsage = "usage: andon project list\n" +
	"セルのプロジェクト（kanban のある projects/<slug>/）の slug を1行ずつ出す。読むだけ。\n"

const (
	exitFail  = 1
	exitUsage = 2
)

var (
	start = chiefofstaff.Start
	// here は今いるフォルダから kanban の CLI が kanban を見つけるか（andon open を slug なしで打ったとき）。
	here = func() bool {
		root, err := spawn.Output(nil, "", ledger.CLI, "root")
		return err == nil && root != ""
	}
	isTerminal = spawn.IsTerminal
	// pick は fzf に lines を渡し、選ばれた行を返す。
	pick = func(lines []string, stderr io.Writer) (string, error) {
		c := exec.Command("fzf", "--prompt", "andon open> ", "--height", "40%", "--reverse")
		c.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
		c.Stderr = stderr
		out, err := c.Output()
		return strings.TrimSpace(string(out)), err
	}
)

// Slugs は ws の projects/*/kanban/config.yaml のあるプロジェクトの slug（昇順）。
func Slugs(ws string) []string {
	cfgs, _ := filepath.Glob(filepath.Join(ws, "projects", "*", "kanban", "config.yaml"))
	var out []string
	for _, c := range cfgs {
		out = append(out, filepath.Base(filepath.Dir(filepath.Dir(c))))
	}
	sort.Strings(out)
	return out
}

func running(slug string) bool {
	out, err := spawn.Output(nil, "", "hc", "ls", "--json", "--label", "role="+chiefofstaff.Profile, "--label", "project="+slug)
	return err == nil && strings.Contains(out, `"address"`)
}

func line(slug string) string {
	if running(slug) {
		return slug + "\t(CoS 稼働中)"
	}
	return slug
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	ws := delegate.Workspace()
	slugs := Slugs(ws)
	if len(args) == 1 {
		return openSlug(ws, args[0], slugs, stdout, stderr)
	}
	if here() {
		return start("", stdout, stderr)
	}
	if len(slugs) == 0 {
		fmt.Fprintf(stderr, "andon open: %s/projects/ に kanban のあるプロジェクトが無い\n", ws)
		return exitFail
	}
	lines := make([]string, len(slugs))
	for i, s := range slugs {
		lines[i] = line(s)
	}
	_, isFile := stdin.(*os.File)
	f, _ := stdin.(*os.File)
	_, fzfErr := exec.LookPath("fzf")
	if !isFile || !isTerminal(f) || fzfErr != nil {
		fmt.Fprintln(stdout, strings.Join(lines, "\n"))
		fmt.Fprint(stderr, "andon open: 選べない（端末でないか fzf が無い）。上の一覧から選んで andon open <slug> と打つ\n")
		return exitUsage
	}
	sel, err := pick(lines, stderr)
	if err != nil || sel == "" {
		return exitFail // 取り消し
	}
	slug, _, _ := strings.Cut(sel, "\t")
	return openSlug(ws, slug, slugs, stdout, stderr)
}

func openSlug(ws, slug string, slugs []string, stdout, stderr io.Writer) int {
	for _, s := range slugs {
		if s == slug {
			return start(filepath.Join(ws, "projects", slug), stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "andon open: プロジェクト '%s' は無い。あるのは:\n", slug)
	for _, s := range slugs {
		fmt.Fprintf(stderr, "  %s\n", s)
	}
	return exitUsage
}

// ListMain は andon project list。
func ListMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprint(stderr, ListUsage)
		return exitUsage
	}
	ws := delegate.Workspace()
	slugs := Slugs(ws)
	if len(slugs) == 0 {
		fmt.Fprintf(stderr, "andon project list: %s/projects/ に kanban のあるプロジェクトが無い（立てるなら andon project create <slug>）\n", ws)
		return 0
	}
	for _, s := range slugs {
		fmt.Fprintln(stdout, s)
	}
	return 0
}
