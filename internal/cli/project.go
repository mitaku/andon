package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"andon/internal/delegate"
	"andon/internal/ledger"
	"andon/internal/open"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// kanbanRoot は今いるフォルダから kanban の CLI が見つける kanban の根（KANBAN_ROOT・上のフォルダ・repo の載ったプロジェクト・
// 上の projects/ に1つだけ。docs/ledger.md の Finding the root）。見つからなければ空。kanban の CLI が無ければ cli が false。
// 検査が差し替える。
var kanbanRoot = func() (root string, cli bool) {
	if _, err := exec.LookPath(ledger.CLI); err != nil {
		return "", false
	}
	out, err := exec.Command(ledger.CLI, "root").Output()
	if err != nil {
		return "", true
	}
	return strings.TrimSpace(string(out)), true
}

// project はプロジェクトを読む葉のプロジェクトを決める（ADR-0053 決めたこと 3・4・15）。順は --project <slug>、KANBAN_ROOT、
// 今いるフォルダ、セルに1つだけ（後ろの3つは kanban の CLI が決める。決めたこと 20）。--project は動詞のあとのどこでも、-- の前で受け、
// 葉が自分で読まないものは外して KANBAN_ROOT に渡す。決まらなければ、セルのプロジェクトの一覧と --project を付ける1行を出して
// 終了コード 2（ok は false）。フックが打つ葉は 2 を返さず、決まらなくても葉に渡す（葉が自分で通す）。
// 省くと全部のプロジェクトを回る葉（dispatcher）は、省いたときに決めない。
func project(l leaf, args []string, stderr io.Writer) ([]string, int, bool) {
	at := -1
	for i, a := range args {
		if a == "--" {
			break
		}
		if a == "--project" {
			at = i
			break
		}
	}
	ws := delegate.Workspace()
	drop := func() []string {
		if at < 0 || l.ownFlag {
			return args
		}
		return append(append([]string{}, args[:at]...), args[min(at+2, len(args)):]...)
	}
	refuse := func(format string, a ...any) ([]string, int, bool) {
		if l.hook {
			return drop(), 0, true
		}
		fmt.Fprintf(stderr, "andon %s: %s\n", l.name, fmt.Sprintf(format, a...))
		fmt.Fprintf(stderr, "%s\n", projectsLine(ws))
		fmt.Fprintf(stderr, "andon %s --project <slug> … のように --project を付けて打ち直す\n", l.name)
		return nil, 2, false
	}
	if at >= 0 {
		if at+1 >= len(args) || args[at+1] == "--" {
			return refuse("--project に slug が要る")
		}
		slug := args[at+1]
		root := filepath.Join(ws, "projects", slug)
		// 在るかはフォルダで見る。委譲と信号の葉は、kanban の無い projects/<slug>/project.json だけでも読む
		if st, err := os.Stat(root); !slugRe.MatchString(slug) || err != nil || !st.IsDir() {
			return refuse("プロジェクト %s は無い（%s が無い）", slug, root)
		}
		if l.ownFlag {
			return args, 0, true // 葉が自分で --project を読む
		}
		os.Setenv("KANBAN_ROOT", root) //nolint:errcheck // 名前も値も空でないので失敗しない
		return drop(), 0, true
	}
	if l.every {
		return args, 0, true
	}
	if root := os.Getenv("KANBAN_ROOT"); root != "" {
		if !hasKanban(root) {
			return refuse("KANBAN_ROOT（%s）に kanban が無い", root)
		}
		return args, 0, true
	}
	if inProject(ws) {
		return args, 0, true // 葉が今いるフォルダから決める（kanban の無い projects/<slug>/ でも project.json を読む葉がある）
	}
	root, cli := kanbanRoot()
	if !cli {
		return args, 0, true // kanban の CLI が無いことは、葉が言う（ledger.Missing）
	}
	if root == "" {
		return refuse("プロジェクトが決まらない（今いるフォルダからは kanban が見つからない）")
	}
	return args, 0, true
}

// inProject は今いるフォルダがセルの projects/<slug>/ の下か。
func inProject(ws string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	base := filepath.Join(ws, "projects")
	if b, err := filepath.EvalSymlinks(base); err == nil {
		base = b
	}
	if c, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = c
	}
	rel, err := filepath.Rel(base, cwd)
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(rel, string(filepath.Separator))
	return first != "." && first != ".." && slugRe.MatchString(first)
}

// projectText は andon <葉> --help のプロジェクトの行。
func projectText(l leaf) string {
	if l.every {
		return "--project <slug> で1つに絞る。省くとセルの全部のプロジェクトを回る"
	}
	return "--project <slug> で指す。省くと KANBAN_ROOT、今いるフォルダ、セルに1つだけのプロジェクトの順で決める"
}

func hasKanban(root string) bool {
	_, err := os.Stat(filepath.Join(root, "kanban", "config.yaml"))
	return err == nil
}

// projectsLine はセルのプロジェクトの一覧の1行。
func projectsLine(ws string) string {
	slugs := open.Slugs(ws)
	if len(slugs) == 0 {
		return fmt.Sprintf("セル（%s）にプロジェクトが無い。andon project create <slug> で立てる", ws)
	}
	return "セルのプロジェクト: " + strings.Join(slugs, "・")
}
