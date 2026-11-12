// Package flock は、ファイルの flock で処理を1つずつにする lock（dispatcher.md §7.2 の①）。
// lock は開いたファイルに付くので、プロセスが落ちれば OS が外す（消し忘れの lock は残らない）。
package flock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ErrBusy は、ほかが lock を持っているので取らなかった（TryLock）。
var ErrBusy = errors.New("ほかが lock を持っている")

// Lock は path の lock を取れるまで待って取る。置き場が無ければ作る。返した関数で外す。
func Lock(path string) (func(), error) {
	return lock(path, syscall.LOCK_EX)
}

// TryLock は path の lock を待たずに取る。ほかが持っていれば ErrBusy。
func TryLock(path string) (func(), error) {
	return lock(path, syscall.LOCK_EX|syscall.LOCK_NB)
}

func lock(path string, how int) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("lock の置き場を作れない: %v", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lock を開けない: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock が取れない: %v", err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
