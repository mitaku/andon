package signal

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// noEcho は stdin が端末なら打った文字を画面に出さないようにし、元に戻す関数を返す（トークンを画面に残さない）。
func noEcho(r io.Reader) func() {
	f, ok := r.(*os.File)
	if !ok {
		return func() {}
	}
	fd := int(f.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return func() {}
	}
	t := *old
	t.Lflag &^= unix.ECHO
	if unix.IoctlSetTermios(fd, unix.TCSETS, &t) != nil {
		return func() {}
	}
	return func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, old) }
}
