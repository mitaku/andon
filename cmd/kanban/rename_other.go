//go:build !windows

package main

// renameBusy: a rename over an open file doesn't fail outside Windows.
func renameBusy(error) bool { return false }
