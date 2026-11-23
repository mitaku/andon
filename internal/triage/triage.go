// Package triage は kanban の溜まったもの（報告の Next・見送り・Remember・For AGENTS.md・オーナーに聞くこと、
// 担当の無い todo、オーナーの判断待ち、片付けの残り）とセルの auto memory（<ANDON_WORKSPACE>/memory の1件1ファイル）と
// 外の信号の記録（<slug>.signals.json の、課題に切られていない active な信号）を集め（collect）、
// 仕分けの決定を kanban に当てる（apply）。
//
// collect は kanban を読むだけで、書くのは kanban の repo の checkout の refs/remotes/origin/<既定の branch>（最新の main を取り直す）だけ。
// apply は判断を持たず、decisions.json を検査し（close の閉じ方と項目の種類の組み合わせも）、見た commit を collect の記録（library/items.json）と照らし、
// 判断の閉じと課題を閉じる判定に Chief of Staff の判定（verdict が書く library/verdicts.json）が揃っているかを確かめてから kanban の CLI を打つ。
// kanban を書くのは kanban の CLI だけで、ここは record（library/triage/）を書き、仕分けの済んだ memory のファイルと目録の行を消す。
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

	"andon/internal/delegate"
	ledgercli "andon/internal/ledger"
	"andon/internal/signal"
	"andon/internal/spawn"
)

// 葉ごとの使い方（andon triage <動詞> --help の中身と、引数を誤ったときに出す文）。
const (
	where        = "projects/<slug>/ で打つ。\n"
	CollectUsage = "usage: andon triage collect [--limit N]\n" +
		"kanban の未処理と repo ごとの最新の main を JSON に出す（読むだけ。担当が打つ）。\n" + where
	ApplyUsage = "usage: andon triage apply <id> [--max-issues N] [--dry-run]\n" +
		"decisions.json を kanban に当てる（Chief of Staff が打つ）。--dry-run は Chief of Staff が読む判定の表 review も出す。\n" +
		"起動する上限は project.json の chief_of_staff.triage_max_issues（無ければ 3）。--max-issues はその回だけ上書きする。\n" + where
	VerdictUsage = "usage: andon triage verdict <id> <key> ok|overturn\n" +
		"Chief of Staff が、判断の閉じと課題を閉じる判定を1件ずつ「閉じてよい」か「覆す」と書く。\n" + where
)

var (
	idRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	slugRe   = spawn.SlugRe // schema の project と同じ
	numberRe = regexp.MustCompile(`^[0-9]{1,6}$`)
)

// 終了コード。
const (
	exitFail  = 1 // kanban・decisions.json・道具の誤り
	exitUsage = 2
	exitLimit = 4 // collect の項目が上限を超えた
)

// DefaultLimit は collect の項目の上限の既定（--limit が無いとき）。dispatcher が空きのトリアージを撃つ線も、この数を読む
// （ADR-0047 決めたこと 7。1 回分）。
const DefaultLimit = 50

// keepHold は、次回に回した（keep）項目を collect が出さない時間（ADR-0047 決めたこと 19）。
// 毎朝の回には全部が出るように、1 日より短い。
const keepHold = 20 * time.Hour

type failure struct{ msg string }

func (f failure) Error() string { return f.msg }

func fail(format string, a ...any) error { return failure{fmt.Sprintf(format, a...)} }

// Collect は andon triage collect。
func Collect(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("collect", args, stdout, stderr)
}

// Apply は andon triage apply。
func Apply(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("apply", args, stdout, stderr)
}

// Verdict は andon triage verdict。
func Verdict(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run("verdict", args, stdout, stderr)
}

func run(cmd string, args []string, stdout, stderr io.Writer) int {
	usage := func() int {
		fmt.Fprint(stderr, map[string]string{"collect": CollectUsage, "apply": ApplyUsage, "verdict": VerdictUsage}[cmd])
		return exitUsage
	}
	limit, maxIssues, maxSet, dry, id, key, value := DefaultLimit, 0, false, false, "", "", ""
	switch cmd {
	case "collect":
	case "verdict":
		if len(args) != 3 {
			return usage()
		}
		key, value = args[1], args[2]
		fallthrough
	case "apply":
		if len(args) == 0 {
			return usage()
		}
		id, args = ledgercli.CanonID(args[0]), args[1:]
		if cmd == "verdict" {
			args = nil
		}
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
				maxIssues, maxSet = n, true
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
	if err == nil && cmd == "apply" && !maxSet {
		maxIssues, err = projectMaxIssues(delegate.Workspace(), l.slug)
	}
	if err == nil {
		switch cmd {
		case "collect":
			err = runCollect(l, limit, stdout, stderr)
		case "verdict":
			err = runVerdict(l, id, key, value, time.Now().UTC(), stdout)
		default:
			err = runApply(l, id, maxIssues, dry, time.Now().UTC(), stdout)
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

// defaultMaxIssues は1回のトリアージで起動する課題の上限の既定（contract.md §11 の triage_max_issues）。
const defaultMaxIssues = 3

// projectMaxIssues は起動する上限を、セルの repo の HEAD の project.json の chief_of_staff.triage_max_issues から読む
// （contract.md §11。author 照合は delegate.ReadProject）。project.json か値が無ければ既定の 3。
// project.json があって信じられない・形が違うときは、上限を決められないので止まる（--max-issues で渡せば通る）。
func projectMaxIssues(ws, slug string) (int, error) {
	p, err := delegate.ReadProject(ws, slug)
	switch {
	case errors.Is(err, delegate.ErrAbsent):
		return defaultMaxIssues, nil
	case err != nil:
		return 0, fail("起動する上限を決められない（project.json を読めない: %v）。直すか --max-issues で渡す", err)
	case p.ChiefOfStaff == nil || p.ChiefOfStaff.TriageMaxIssues == nil:
		return defaultMaxIssues, nil
	}
	return *p.ChiefOfStaff.TriageMaxIssues, nil
}

type limitError struct{ limit int }

func (e limitError) Error() string { return fmt.Sprintf("over limit %d", e.limit) }

// ledger は1つの kanban。kanban（メソッド）は kanban の CLI をこの kanban に向けて打つ。
type ledger struct {
	cli     string // 打つ kanban の CLI
	root    string // プロジェクトの根（kanban の親）
	slug    string
	dir     string // <root>/kanban
	memory  string // auto memory の置き場（空なら拾わない）
	signals string // 信号の記録（<slug>.signals.json。空なら拾わない）
	tz      string // config.yaml の timezone（not_before を読む。空なら UTC）
	now     time.Time
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
		return nil, fail("kanban が見つからない")
	}
	return ledgerAt(cli, strings.TrimRight(string(out), "\n"), time.Now())
}

// ledgerAt はプロジェクトの根 root（kanban/ の親）の ledger。
func ledgerAt(cli, root string, now time.Time) (*ledger, error) {
	l := &ledger{cli: cli, root: root, slug: filepath.Base(root), dir: filepath.Join(root, "kanban"), memory: MemoryDir(), now: now}
	if !slugRe.MatchString(l.slug) {
		return nil, fail("slug が不正")
	}
	l.signals = signal.Path(l.slug)
	l.tz = ledgercli.At(root).Timezone()
	return l, nil
}

// kanban（メソッド）は kanban の CLI を打ち、stdout を返す（末尾の改行を除く）。stderr はそのまま流す。
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
