//go:build !unix

package waiting

import "os"

// openNoWait は path を読むために開く（unix でない所には、開くと書き手を待つ FIFO が無い）。
func openNoWait(path string) (*os.File, error) {
	return os.Open(path)
}
