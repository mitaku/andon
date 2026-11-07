//go:build !linux

package spawn

import "os"

// IsTerminal は f が端末か（linux 以外の近似: 文字デバイスで、/dev/null でない）。andon は linux の建屋で動く。
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(fi, null)
}
