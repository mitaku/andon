// Package triage は台帳の溜まったもの（報告の Next・見送り・Remember・For AGENTS.md・人に聞くこと、
// 担当の無い todo、人の判断待ち、片付けの残り）を集め（collect）、仕分けの決定を台帳に当てる（apply）。
//
// collect は読むだけ。apply は判断を持たず、decisions.json を検査してから台帳の CLI を打つ。
// 台帳を書くのは台帳の CLI（kanban）だけで、ここは record（library/triage/）だけを書く。
package triage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	ledgercli "andon/internal/ledger"
)

// Usage は andon triage の使い方。
const Usage = "usage: andon triage collect [--limit N]                    台帳の未処理を JSON に集める（読むだけ）\n" +
	"       andon triage apply <id> [--max-issues N] [--dry-run] decisions.json を台帳に当てる\n"

var (
	idRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	slugRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	numberRe = regexp.MustCompile(`^[0-9]{1,6}$`)
)

// 終了コード。
const (
	exitFail  = 1 // 台帳・decisions.json・道具の誤り
	exitUsage = 2
	exitLimit = 4 // collect の項目が上限を超えた
)

type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func fail(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

// Main は andon triage。
func Main(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	usage := func() int { fmt.Fprint(stderr, Usage); return exitUsage }
	if len(args) == 0 {
		return usage()
	}
	cmd, args := args[0], args[1:]
	limit, maxTasks, dry, id := 50, 3, false, ""
	switch cmd {
	case "collect":
	case "apply":
		if len(args) == 0 {
			return usage()
		}
		id, args = ledgercli.CanonID(args[0]), args[1:]
		if !idRe.MatchString(id) {
			fmt.Fprintln(stderr, "andon triage: id が不正")
			return exitFail
		}
	default:
		return usage()
	}
	for len(args) > 0 {
		switch {
		case (cmd == "collect" && args[0] == "--limit") || (cmd == "apply" && args[0] == "--max-issues"):
			if len(args) < 2 || !numberRe.MatchString(args[1]) {
				return usage()
			}
			n, _ := strconv.Atoi(args[1])
			if cmd == "collect" {
				limit = n
			} else {
				maxTasks = n
			}
			args = args[2:]
		case cmd == "apply" && args[0] == "--dry-run":
			dry = true
			args = args[1:]
		default:
			return usage()
		}
	}
	l, err := openLedger()
	if err == nil {
		if cmd == "collect" {
			err = runCollect(l, limit, stdout, stderr)
		} else {
			err = runApply(l, id, maxTasks, dry, time.Now().UTC(), stdout)
		}
	}
	var f failure
	var le limitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &le):
		fmt.Fprintf(stderr, "andon triage: 項目が上限 %d を超えた（--limit を増やして収集する）\n", le.limit)
		return exitLimit
	case errors.As(err, &f):
		fmt.Fprintf(stderr, "andon triage: %s\n", f.msg)
	default:
		fmt.Fprintf(stderr, "andon triage: %v\n", err)
	}
	return exitFail
}

type limitError struct{ limit int }

func (e limitError) Error() string { return fmt.Sprintf("over limit %d", e.limit) }

// ledger は1つの台帳。kanban（メソッド）は台帳の CLI をこの台帳に向けて打つ。
type ledger struct {
	cli  string // 打つ台帳の CLI
	root string // プロジェクトの根（kanban の親）
	slug string
	dir  string // <root>/kanban
	now  time.Time
}

func openLedger() (*ledger, error) {
	if err := ledgercli.Check(); err != nil {
		return nil, fail("%v", err)
	}
	cli := ledgercli.CLI
	for _, c := range []string{"git"} {
		if _, err := exec.LookPath(c); err != nil {
			return nil, fail("%s が無い", c)
		}
	}
	out, err := exec.Command(cli, "root").Output()
	if err != nil {
		return nil, fail("台帳が見つからない")
	}
	root := strings.TrimRight(string(out), "\n")
	l := &ledger{cli: cli, root: root, slug: filepath.Base(root), dir: filepath.Join(root, "kanban"), now: time.Now()}
	if !slugRe.MatchString(l.slug) {
		return nil, fail("slug が不正")
	}
	return l, nil
}

// kanban は台帳の CLI（kanban）を打ち、stdout を返す（末尾の改行を除く）。stderr はそのまま流す。
func (l *ledger) kanban(stdin string, args ...string) (string, error) {
	c := exec.Command(l.cli, args...)
	c.Env = append(os.Environ(), "KANBAN_ROOT="+l.root)
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	c.Stderr = os.Stderr
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", l.cli, strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// writeJSON は jq と同じく2字下げで、HTML の文字を逃がさずに書く。
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func marshalJSON(v any) []byte {
	var b bytes.Buffer
	_ = writeJSON(&b, v)
	return b.Bytes()
}
