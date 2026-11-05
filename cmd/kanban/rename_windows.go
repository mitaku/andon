//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// renameBusy: the target is open in another program (the board reading it, a virus scanner),
// which Windows reports as access denied or a sharing violation. Worth trying again shortly.
func renameBusy(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
