package dispatcher

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedBody は偽の道具（kanban・gh・hc・andon）。打たれた引数を calls に「道具 引数」で1行ずつ書き、
// 引数の md5 の名前の答え（<名前>.out）を標準出力に返す。答えが無ければ 1 で落ちる。
const scriptedBody = `#!/bin/sh
d=$FAKE_TOOLS
tool=${0##*/}
echo "$tool $*" >>"$d/calls"
k=$(printf '%s %s' "$tool" "$*" | md5sum | cut -c1-32)
[ -e "$d/$k.out" ] || exit 1
cat "$d/$k.out"
`

type scripted struct {
	t *testing.T
	d string
}

// newScripted は names の偽の道具を PATH の先頭に置く。
func newScripted(t *testing.T, names ...string) *scripted {
	t.Helper()
	s := &scripted{t: t, d: t.TempDir()}
	bin := filepath.Join(s.d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(bin, n), []byte(scriptedBody), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_TOOLS", s.d)
	return s
}

func (s *scripted) key(cmd string) string {
	sum := md5.Sum([]byte(cmd))
	return filepath.Join(s.d, hex.EncodeToString(sum[:]))
}

// reply は cmd（「道具 引数…」を空白1つで）に out を答える。
func (s *scripted) reply(cmd, out string) {
	s.t.Helper()
	if err := os.WriteFile(s.key(cmd)+".out", []byte(out), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

// calls は打たれたものを1行ずつ返し、控えを空にする。
func (s *scripted) calls() []string {
	b, _ := os.ReadFile(filepath.Join(s.d, "calls"))
	os.Remove(filepath.Join(s.d, "calls"))
	if t := strings.TrimSpace(string(b)); t != "" {
		return strings.Split(t, "\n")
	}
	return nil
}

func hasCall(calls []string, prefix string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}
