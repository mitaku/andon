package board

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/ledger"
)

const BoardUsage = "usage: andon issue board --since <時刻> [--json] [--project <slug>]\n" +
	"ボードが読む課題だけを出す。読むだけで、kanban のファイルも履歴も書かない（docs/adr/0059-board-leaves-old-done-to-history.md 決めたこと 6・7）。\n" +
	"出すのは、片付いていない課題と読めない課題、updated が --since 以降の片付いた課題、その 2 つの親をたどった先、\n" +
	"片付いていない課題の親をたどった先を親に持つ課題、その 2 つの after の相手。older は updated が --since より前の片付いた課題の数。\n" +
	"--since は省けない（RFC 3339 の時刻。2026-11-23T13:00:00Z）。\n" +
	"issue.yaml かそのフォルダが symlink・256 KiB を超える・YAML の alias を含む課題は、中身を出さずに読めない課題（broken）として出す。\n" +
	"kanban は --project <slug>（セルの repo の projects/<slug>/）か KANBAN_ROOT か、projects/<slug>/ か担当の worktree の kanban。\n" +
	"--json は docs/contract.md §29 の形。\n"

const HistoryUsage = "usage: andon issue history [--json] [--limit <件数>] [--match <文字>] [--project <slug>]\n" +
	"片付いた課題を updated の新しい順に出す（ボードの表示 History が読む）。読むだけで、kanban のファイルも履歴も書かない（docs/adr/0059-board-leaves-old-done-to-history.md 決めたこと 8）。\n" +
	"--limit は件数（既定 200、上限 2000）。more は条件に合うのに出さなかった数。\n" +
	"--match は、番号・タイトル・repo・プロファイル・端末の名前（agent）のどれかにその文字を含む課題だけを残す（大文字と小文字を分けない）。\n" +
	"読み方の守りと kanban の探し方は andon issue board と同じ。--json は docs/contract.md §29 の形。\n"

func BoardMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	asJSON, slug, sinceArg, hasSince := false, "", "", false
	for len(args) > 0 {
		switch {
		case args[0] == "--json":
			asJSON, args = true, args[1:]
		case args[0] == "--project" && len(args) > 1:
			slug, args = args[1], args[2:]
		case args[0] == "--since" && len(args) > 1:
			sinceArg, hasSince, args = args[1], true, args[2:]
		default:
			fmt.Fprint(stderr, BoardUsage)
			return 2
		}
	}
	if !hasSince {
		fmt.Fprint(stderr, "andon issue board: --since は省けない\n"+BoardUsage)
		return 2
	}
	since, err := time.Parse(time.RFC3339, sinceArg)
	if err != nil {
		fmt.Fprintf(stderr, "andon issue board: --since が RFC 3339 の時刻でない（%q）\n%s", sinceArg, BoardUsage)
		return 2
	}
	root, err := projectRoot(slug)
	if err != nil {
		fmt.Fprintf(stderr, "andon issue board: %v\n", err)
		return 1
	}
	res, err := Board(root, since, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "andon issue board: %v\n", err)
		return 1
	}
	return emit("andon issue board", res, asJSON, stdout, stderr)
}

func HistoryMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	asJSON, slug, match, limit := false, "", "", DefaultLimit
	for len(args) > 0 {
		switch {
		case args[0] == "--json":
			asJSON, args = true, args[1:]
		case args[0] == "--project" && len(args) > 1:
			slug, args = args[1], args[2:]
		case args[0] == "--match" && len(args) > 1:
			match, args = args[1], args[2:]
		case args[0] == "--limit" && len(args) > 1:
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 || n > MaxLimit {
				fmt.Fprintf(stderr, "andon issue history: --limit は 1 から %d までの数（%q）\n%s", MaxLimit, args[1], HistoryUsage)
				return 2
			}
			limit, args = n, args[2:]
		default:
			fmt.Fprint(stderr, HistoryUsage)
			return 2
		}
	}
	root, err := projectRoot(slug)
	if err != nil {
		fmt.Fprintf(stderr, "andon issue history: %v\n", err)
		return 1
	}
	res, err := History(root, limit, match, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "andon issue history: %v\n", err)
		return 1
	}
	return emit("andon issue history", res, asJSON, stdout, stderr)
}

func emit(name string, res Result, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		b, err := contract.Encode(contract.SchemaIssueBoard, res)
		if err != nil {
			fmt.Fprintf(stderr, "%s: 出力が schema/issue-board.schema.json に合わない: %v\n", name, err)
			return 1
		}
		_, _ = stdout.Write(b)
		return 0
	}
	fmt.Fprint(stdout, Text(res))
	return 0
}

// projectRoot は kanban の根（projects/<slug>/）: --project、KANBAN_ROOT、kanban の CLI（kanban root。根を聞くだけ）の順。
// andon waiting list と同じ探し方。
func projectRoot(slug string) (string, error) {
	if slug != "" {
		if _, err := delegate.ProjectSlug(delegate.Workspace(), slug); err != nil {
			return "", err
		}
		root := filepath.Join(delegate.Workspace(), "projects", slug)
		if _, err := os.Stat(filepath.Join(root, "kanban", "config.yaml")); err != nil {
			return "", fmt.Errorf("プロジェクト %s の kanban が無い（%s）", slug, filepath.Join(root, "kanban"))
		}
		return root, nil
	}
	if root := os.Getenv("KANBAN_ROOT"); root != "" {
		return root, nil
	}
	l, err := ledger.Open()
	if err != nil {
		return "", err
	}
	return l.Root, nil
}

// Text は人が読む行（--json を付けないときの出力。1 課題 1 行で、中身は --json と同じ）。
func Text(r Result) string {
	var b strings.Builder
	for _, it := range r.Issues {
		id, _ := it["id"].(string)
		if why, ok := it["broken"].(string); ok {
			fmt.Fprintf(&b, "%s 読めない: %s\n", id, why)
			continue
		}
		fmt.Fprintf(&b, "%s %s %s\n", id, str(it["status"]), str(it["title"]))
	}
	if len(r.Issues) == 0 {
		b.WriteString("課題は無い\n")
	}
	if r.Older != nil {
		fmt.Fprintf(&b, "片付いて %s より前の課題: %d 件\n", r.Since, *r.Older)
	}
	if r.More != nil && *r.More > 0 {
		fmt.Fprintf(&b, "ほかに %d 件（--limit で増やす）\n", *r.More)
	}
	return b.String()
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
