package spawn

import (
	"os"

	"golang.org/x/sys/unix"
)

// IsTerminal は f が端末（TTY）か。文字デバイス（/dev/null・/dev/zero）は端末でない。
func IsTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}
