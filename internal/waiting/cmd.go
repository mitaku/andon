package waiting

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/contract"
	"andon/internal/delegate"
	"andon/internal/ledger"
)

const Usage = "usage: andon waiting list [--json] [--project <slug>]\n" +
	"オーナー待ち（マージの依頼・判断・手作業）と並行の枠を出す。読むだけで、何も書かない。GitHub も git も読まない。\n" +
	"判断の問いにペインから選択肢で答える口は andon waiting answer。\n" +
	"kanban の履歴（log.jsonl）が読めなければ数えずに終了コード 1。\n" +
	"数えるのは今の head について決め終えたものだけ（docs/adr/0042-owner-waits-do-not-stop-the-line.md 決めたこと 8・9）。\n" +
	"kanban は --project <slug>（セルの repo の projects/<slug>/）か KANBAN_ROOT か、projects/<slug>/ か担当の worktree の kanban。\n" +
	"--json は docs/contract.md §25 の形。\n"

func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	asJSON, slug := false, ""
	for len(args) > 0 {
		switch {
		case args[0] == "--json":
			asJSON, args = true, args[1:]
		case args[0] == "--project" && len(args) > 1:
			slug, args = args[1], args[2:]
		default:
			fmt.Fprint(stderr, Usage)
			return 2
		}
	}
	root, err := projectRoot(slug)
	if err != nil {
		fmt.Fprintf(stderr, "andon waiting list: %v\n", err)
		return 1
	}
	res, warn, err := Read(root, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "andon waiting list: %v\n", err)
		return 1
	}
	for _, w := range warn {
		fmt.Fprintf(stderr, "andon waiting list: %s\n", w)
	}
	if asJSON {
		b, err := contract.Encode(contract.SchemaWaiting, res)
		if err != nil {
			fmt.Fprintf(stderr, "andon waiting list: 出力が schema/waiting.schema.json に合わない: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(b)
		return 0
	}
	fmt.Fprint(stdout, Text(res))
	return 0
}

// projectRoot は kanban の根（projects/<slug>/）: --project、KANBAN_ROOT、kanban の CLI（kanban root。根を聞くだけ）の順。
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

// Text は人が読む行（--json を付けないときの出力。中身は --json と同じ）。
func Text(r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s の枠: 上限 %d、動いている %d、オーナー待ち %d、空き %d\n",
		r.Project, r.Slots.Limit, r.Slots.Working, r.Slots.Waiting, r.Slots.Free)
	if len(r.Items) == 0 {
		b.WriteString("オーナー待ちは無い\n")
		return b.String()
	}
	// 呼んでいる行と置いてある行を分けて出す（ADR-0062 決めたこと 6）。行は呼んでいるものが先に並んでいる
	called, later := 0, 0
	for _, it := range r.Items {
		if it.Call {
			called++
		} else {
			later++
		}
	}
	if called > 0 {
		fmt.Fprintf(&b, "呼んでいるもの %d 件（行燈を灯す）\n", called)
	} else {
		b.WriteString("呼んでいるものは無い\n")
	}
	kind := map[string]string{"merge": "マージの依頼", "decision": "判断", "task": "手作業"}
	for i, it := range r.Items {
		if !it.Call && (i == 0 || r.Items[i-1].Call) {
			fmt.Fprintf(&b, "置いてあるもの %d 件（行燈を灯さない。いつでも答えられる）\n", later)
		}
		name := it.ID
		if it.Question != "" {
			name = it.Question
		}
		fmt.Fprintf(&b, "%s %s（%s）。%s から", kind[it.Kind], name, it.Title, it.Since)
		if len(it.Holds) > 0 {
			fmt.Fprintf(&b, "。後ろで %d 件が待つ（%s）", len(it.Holds), strings.Join(it.Holds, ", "))
		}
		if it.Ask != nil {
			fmt.Fprintf(&b, "。問い: %s", it.Ask.Question)
		} else if it.Reason != "" {
			fmt.Fprintf(&b, "。理由: %s", it.Reason)
		}
		if it.Kind == "decision" && it.Question == "" {
			// 問いの置かれていない判断の行は、直す 1 手を 2 つ並べる（ADR-0057 決めたこと 4）
			fmt.Fprintf(&b, "。直す: 判断なら andon issue ask %s -、手作業なら kanban set %s only_owner=\"<理由>\"", it.ID, it.ID)
		}
		if it.PR != "" {
			fmt.Fprintf(&b, "。PR %s", it.PR)
		}
		if it.Steps != "" {
			b.WriteString("。手順は依頼書にある（--json の steps）")
		}
		if len(it.Links) > 0 {
			fmt.Fprintf(&b, "。リンク: %s", strings.Join(it.Links, " "))
		}
		if !it.Call && it.Later != "" {
			fmt.Fprintf(&b, "。置いた理由: %s", it.Later)
			if it.LaterUntil != "" {
				fmt.Fprintf(&b, "。%s に呼ぶ", it.LaterUntil)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
