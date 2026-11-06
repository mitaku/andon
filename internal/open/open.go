// Package open は andon open [slug]: プロジェクトを選んで Chief of Staff を開く。
//
//	andon open <slug>   projects/<slug>/ で andon chief-of-staff を打つのと同じ（cd はしない）
//	andon open          台帳のあるプロジェクトの一覧から fzf で選んで開く
//
// slug が無く、fzf が無いか端末でないときは、一覧（Chief of Staff が動いているかつき）を出して終了コード 2。
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
	"andon/internal/spawn"
)

// Usage は andon open の使い方。
const Usage = "usage: andon open [slug]\n" +
	"slug があれば projects/<slug>/ の Chief of Staff を開く（andon chief-of-staff と同じ。cd は要らない）。\n" +
	"slug が無ければ、台帳のあるプロジェクトの一覧から fzf で選ぶ（横に Chief of Staff が動いているかを出す）。\n" +
	"fzf が無いか端末でないときは、一覧を出して終了コード 2。\n"

const (
	exitFail  = 1
	exitUsage = 2
)

// 差し替えられる外部とのつなぎ（テスト用）。
var (
	start      = chiefofstaff.Start
	isTerminal = func(f *os.File) bool {
		fi, err := f.Stat()
		return err == nil && fi.Mode()&os.ModeCharDevice != 0
	}
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

// running は slug の Chief of Staff が hc で動いているか（ラベルで分かる範囲。hc が無い・失敗なら false）。
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

// Main は andon open を走らせ、終了コードを返す。
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprint(stderr, Usage)
		return exitUsage
	}
	ws := delegate.Workspace()
	slugs := Slugs(ws)
	if len(args) == 1 {
		return openSlug(ws, args[0], slugs, stdout, stderr)
	}
	if len(slugs) == 0 {
		fmt.Fprintf(stderr, "andon open: %s/projects/ に台帳のあるプロジェクトが無い\n", ws)
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
