package spawn

import (
	"strings"
	"testing"
)

func TestBareShell(t *testing.T) {
	for argv, want := range map[string]bool{
		"/bin/bash":            true,
		"bash":                 true,
		"-zsh":                 true,
		"/bin/sh":              true,
		"sh -c claude":         false, // エージェントの起動（hc の template）
		"bash /srv/news":       false, // スクリプトを回している
		"claude":               false,
		"/opt/andon/bin/andon": false,
	} {
		if got := BareShell(strings.Fields(argv)); got != want {
			t.Errorf("%s: %v", argv, got)
		}
	}
	if BareShell(nil) {
		t.Error("空を シェルとみた")
	}
}
