//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// holdOpen opens path the way a reader that doesn't allow deletes would (no FILE_SHARE_DELETE),
// which makes a rename over it fail with access denied until it is closed.
func holdOpen(t *testing.T, path string) windows.Handle {
	t.Helper()
	p, _ := windows.UTF16PtrFromString(path)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestRenameWaitsForAReaderToLetGo(t *testing.T) {
	file := filepath.Join(t.TempDir(), "task.yaml")
	os.WriteFile(file, []byte("title: old\n"), 0o644)
	h := holdOpen(t, file)
	go func() {
		time.Sleep(200 * time.Millisecond)
		windows.CloseHandle(h)
	}()
	if err := writeAtomic(file, []byte("title: new\n")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != "title: new\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRenameGivesUpAndCleansUp(t *testing.T) {
	old := renameWait
	renameWait = 150 * time.Millisecond
	defer func() { renameWait = old }()
	dir := t.TempDir()
	file := filepath.Join(dir, "task.yaml")
	os.WriteFile(file, []byte("title: old\n"), 0o644)
	h := holdOpen(t, file)
	defer windows.CloseHandle(h)
	err := writeAtomic(file, []byte("title: new\n"))
	if err == nil || !strings.Contains(err.Error(), "can't replace") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("got %v, want one line saying it can't replace the file", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary file left: %d files", len(entries))
	}
}
