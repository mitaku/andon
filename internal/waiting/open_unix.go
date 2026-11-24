//go:build unix

package waiting

import (
	"os"
	"syscall"
)

// openNoWait は path を読むために開く。FIFO でも書き手を待たずに返るよう O_NONBLOCK で開く
// （普通のファイルの読みには効かない）。開いたものが普通のファイルかは呼び手が見る。
func openNoWait(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
