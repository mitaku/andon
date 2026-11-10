//go:build !linux

package signal

import "io"

// noEcho は linux の外では何もしない（andon はセル〈linux〉で動く）。
func noEcho(io.Reader) func() { return func() {} }
