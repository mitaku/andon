package issue

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"

	"andon/internal/contract"
)

// record は担当が証拠に走らせた command の記録（contract.md §24。ADR-0035 決めたこと 2・8・11）。
// 課題の worktree で command を shell を通さずに走らせ、出力を端末へ流しながら library/runs/<n>.log に残し、
// 終了コード・始めたときの commit と commit していない変更の有無・時刻を library/runs/<n>.json に書く。
// 記録は終了コードが期待と違っても書く（走らせた事実を残す）。期待（pass は 0、fail は 0 以外）と違えば終了コード 1。
// command を起動できなければ記録を書かない（打ち間違いを「落ちた」記録にしない）。
// 記録は書き足すだけで書き直さない。番号は runs/<n>.log を排他で作って取る（同じ課題で並べて打っても重ならない）。

// recordLimit は残す出力の上限（バイト）。超えたら終わりの側を残す（検査の結果は終わりに出る）。
const recordLimit = 1 << 20

func (c *ctx) record(stdout io.Writer, expect string, argv []string) error {
	dir := c.issue.Path("worktree")
	if dir == "" {
		dir = "."
	}
	var commit string
	var dirty *bool
	if c.hasRepo() {
		head, err := c.head()
		if err != nil {
			return err
		}
		d, err := worktreeDirty(dir)
		if err != nil {
			return err
		}
		commit, dirty = head, &d
	}
	runs := filepath.Join(c.l.Library(c.id), "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		return err
	}
	n, logf, err := reserve(runs)
	if err != nil {
		return err
	}
	tail := &tailBuffer{limit: recordLimit}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = io.MultiWriter(stdout, tail)
	cmd.Stderr = cmd.Stdout
	started := time.Now().UTC()
	runErr := cmd.Run()
	finished := time.Now().UTC()
	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			logf.Close()
			os.Remove(logf.Name())
			return fail("command を起動できない（記録は書かない）: %v", runErr)
		}
		code = ee.ExitCode() // signal で終わったら -1
	}
	if _, err := logf.Write(tail.bytes()); err != nil {
		logf.Close()
		return err
	}
	if err := logf.Close(); err != nil {
		return err
	}
	r := &contract.Run{ID: n, Command: argv, Expect: expect, ExitCode: code, Commit: commit, Dirty: dirty,
		StartedAt: started.Format(time.RFC3339), FinishedAt: finished.Format(time.RFC3339),
		Output: contract.RunOutput(n), OutputBytes: tail.total, Truncated: tail.cut() > 0}
	if err := r.Check(); err != nil {
		return fail("記録の形が違う: %v", err)
	}
	path := filepath.Join(runs, strconv.Itoa(n)+".json")
	if err := contract.Write(path, contract.SchemaRun, r); err != nil {
		return err
	}
	line := fmt.Sprintf("%s/library/runs/%d.json written（期待 %s・終了コード %d", c.id, n, expect, code)
	if commit != "" {
		line += "・commit " + commit[:12]
		if *dirty {
			line += "・commit していない変更あり"
		}
	}
	fmt.Fprintln(stdout, line+"）")
	if expect == "pass" && dirty != nil && *dirty {
		fmt.Fprintln(stdout, "commit していない変更のある木で撮った。pass を期待する記録は証拠に数えられない。commit してから撮り直す")
	}
	if !r.Met() {
		if expect == "pass" {
			return fail("pass を期待したが終了コード %d（記録 runs/%d.json は書いた）", code, n)
		}
		return fail("fail を期待したが終了コード 0（記録 runs/%d.json は書いた）", n)
	}
	return nil
}

// worktreeDirty は commit していない変更（追跡していないファイルを含む。無視するファイルは除く）があるか。
func worktreeDirty(dir string) (bool, error) {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return false, fail("commit していない変更を確かめられない（worktree: %s）", dir)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

// reserve は runs/ の次の番号の出力のファイルを排他で作る。番号は今ある記録・出力の最大 ＋ 1 から。
func reserve(runs string) (int, *os.File, error) {
	entries, err := os.ReadDir(runs)
	if err != nil {
		return 0, nil, err
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if k, err := strconv.Atoi(name[:len(name)-len(ext)]); err == nil && (ext == ".json" || ext == ".log") && k > n {
			n = k
		}
	}
	for {
		n++
		f, err := os.OpenFile(filepath.Join(runs, strconv.Itoa(n)+".log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return n, f, err
	}
}

// tailBuffer は書かれたものの終わりの limit バイトだけを持ち、全体のバイト数を数える。
type tailBuffer struct {
	limit int
	total int64
	buf   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.total += int64(len(p))
	t.buf = append(t.buf, p...)
	// 上限の2倍まで溜めてから詰める（書くたびに詰めると、細かい書き込みのたびに上限の分を動かす）
	if over := len(t.buf) - t.limit; over > t.limit {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

// kept は残す終わりの limit バイト。
func (t *tailBuffer) kept() []byte {
	if over := len(t.buf) - t.limit; over > 0 {
		return t.buf[over:]
	}
	return t.buf
}

// cut は切った先頭のバイト数。
func (t *tailBuffer) cut() int64 { return t.total - int64(len(t.kept())) }

// bytes は残す中身。切ったなら、先頭に切った量の1行を置き、文字の途中から始めない。
func (t *tailBuffer) bytes() []byte {
	b := t.kept()
	if t.cut() == 0 {
		return b
	}
	for len(b) > 0 && !utf8.RuneStart(b[0]) {
		b = b[1:]
	}
	head := fmt.Sprintf("（出力が %d バイトで上限 %d バイトを超えたので、先頭の %d バイトを切った）\n", t.total, t.limit, t.total-int64(len(b)))
	return append([]byte(head), b...)
}
