package flock

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTryLockBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "x.lock")
	unlock, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TryLock(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("2本目は ErrBusy のはず: %v", err)
	}
	unlock()
	again, err := TryLock(path)
	if err != nil {
		t.Fatalf("外したあとは取れるはず: %v", err)
	}
	again()
}

func TestLockWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{})
	go func() {
		u, err := Lock(path)
		if err == nil {
			u()
		}
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("持っている間は待つはず")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("外したら取れるはず")
	}
}
