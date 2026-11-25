// Package toolstest は宣言を満たす手（tools.Install）を通る検査が使う偽の mise を置く。
// 検査は本物の mise install を打たない（ADR-0063 決めたこと 6 の細部）: HOME を替えた検査が本物の mise を呼ぶと、
// 替えた HOME の下へ道具を入れ直す。
package toolstest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeMise は呼ばれた引数と cwd を FAKE_MISE_LOG（と、あれば LOG）に 1 行足す。FAKE_MISE で振る舞いを変える:
// 空は何も入れない（all tools are installed）、installed は 1 つ入れた、fail は終了コード 1、slow は 2 秒かけて終わり、
// 終わった印 FAKE_MISE_DONE を作る（上限を過ぎても止められないことを見る）。FAKE_MISE_ENVLOG があれば、受けた MISE_SYSTEM_CONFIG_FILE を 1 行足す。
const fakeMise = `#!/bin/sh
line="mise $* | cwd=$PWD"
printf '%s\n' "$line" >>"$FAKE_MISE_LOG"
[ -z "${LOG:-}" ] || printf '%s\n' "$line" >>"$LOG"
[ -z "${FAKE_MISE_ENVLOG:-}" ] || printf '%s\n' "${MISE_SYSTEM_CONFIG_FILE:-}" >>"$FAKE_MISE_ENVLOG"
case "${FAKE_MISE:-}" in
  installed) echo "mise ✓ go@9.9.9" >&2; echo "mise 1/1 · installed 1 tool" >&2 ;;
  fail) echo "mise ERROR boom" >&2; exit 1 ;;
  slow) sleep 2; : >"$FAKE_MISE_DONE" ;;
  *) echo "mise all tools are installed" >&2 ;;
esac
`

// Fake は置いた偽の mise。
type Fake struct {
	log, done string
}

// New は偽の mise を PATH の先頭に置き、振る舞い mode（FAKE_MISE）を決める。
func New(t *testing.T, mode string) *Fake {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "mise"), []byte(fakeMise), 0o755); err != nil {
		t.Fatal(err)
	}
	f := &Fake{log: filepath.Join(dir, "calls"), done: filepath.Join(dir, "done")}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_MISE", mode)
	t.Setenv("FAKE_MISE_LOG", f.log)
	t.Setenv("FAKE_MISE_DONE", f.done)
	return f
}

// Calls は偽の mise が呼ばれた行（"mise <引数> | cwd=<cwd>"）。
func (f *Fake) Calls() []string {
	b, err := os.ReadFile(f.log)
	if err != nil || len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// Done は slow の偽の mise が終わった印があるか。
func (f *Fake) Done() bool {
	_, err := os.Stat(f.done)
	return err == nil
}

// WaitDone は slow の偽の mise が自分で終わるのを待つ（止められずに残ったことを見る）。
func (f *Fake) WaitDone(t *testing.T) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); !f.Done(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("手放した偽の mise が終わらない（止められたか、動いていない）")
		}
	}
}

// Manifest は dir に mise.toml を置く。
func Manifest(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\ngo = \"9.9.9\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Short は上限 *limit（tools.Limit）を d に短くし、検査の終わりに戻す。
func Short(t *testing.T, limit *time.Duration, d time.Duration) {
	t.Helper()
	old := *limit
	*limit = d
	t.Cleanup(func() { *limit = old })
}
