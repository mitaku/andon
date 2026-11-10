package issue

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"andon/internal/dispatcher"
	"andon/internal/ledger"
	"andon/internal/spawn"
)

// lockIssues は start・finish・resume が命令の間ずっと持つ lock（台帳1つに1つ。ADR-0012 決めたこと 13 の 4・14）。
// 取れるまで待つ。dispatcher の控えの lock（lockState）とは別のファイル: dispatcher は見回りの間 lockState を持ったまま
// andon issue finish を打つので、同じ lock だと finish が自分の呼び手を待って止まる。
func lockIssues(slug string) (func(), error) {
	path := filepath.Join(filepath.Dir(dispatcher.StatePath(slug)), slug+".issue.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fail("lock の置き場を作れない: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fail("lock を開けない: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fail("lock が取れない: %v", err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// withLockedIssue は lockIssues を持ってから課題を読む（読んだ status が命令の間変わらないように）。
func withLockedIssue(id string, f func(*ctx) error) error {
	l, err := ledger.Open()
	if err != nil {
		return err
	}
	unlock, err := lockIssues(l.Slug)
	if err != nil {
		return err
	}
	defer unlock()
	t, err := l.Show(id)
	if err != nil {
		return err
	}
	return f(&ctx{l: l, id: id, issue: t})
}

// session は andon issue session: 自分の端末（HC_TERMINAL_ID）のラベル session の値を出す（template の command・resume の行が読む）。
// 無い・読めない・形が違うときは何も出さずに 0（template の行を落とさない。読めなかった理由は stderr）。
func session(stdout, stderr io.Writer) int {
	addr := os.Getenv("HC_TERMINAL_ID")
	if addr == "" {
		return 0
	}
	t, err := lookTerm(addr)
	if err != nil {
		fmt.Fprintf(stderr, "andon issue session: %v\n", err)
		return 0
	}
	if s := t.Labels[spawn.SessionLabel]; spawn.SessionRe.MatchString(s) {
		fmt.Fprintln(stdout, s)
	}
	return 0
}
