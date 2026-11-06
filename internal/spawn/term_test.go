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

func TestKind(t *testing.T) {
	fg := func(argv ...string) *struct {
		Argv []string `json:"argv"`
		Name string   `json:"name"`
	} {
		return &struct {
			Argv []string `json:"argv"`
			Name string   `json:"name"`
		}{Argv: argv}
	}
	claude := &struct {
		App string `json:"app"`
	}{App: AgentApp}
	agentCmd := []string{"sh", "-c", "claude"}
	for name, c := range map[string]struct {
		term Term
		want Kind
	}{
		"戻った端末（argv ごとシェル・前もシェル）":                {Term{Argv: []string{"/bin/bash"}, Foreground: fg("/bin/bash")}, Restored},
		"戻った端末で人が何か動かしている":                       {Term{Argv: []string{"/bin/bash"}, Foreground: fg("vim")}, Unsure},
		"戻った端末で前に出ているものが取れない":                    {Term{Argv: []string{"/bin/bash"}}, Unsure},
		"Claude Code が動いている":                     {Term{Argv: agentCmd, Status: claude, Foreground: fg(agentCmd...)}, Running},
		"エージェントの command で前にシェル（人が中で使う）":         {Term{Argv: agentCmd, Foreground: fg("/bin/bash")}, Unsure},
		"エージェントの command で agent の情報が無い":         {Term{Argv: agentCmd, Foreground: fg("/usr/bin/vim")}, Unsure},
		"argv がシェルなのに agent が claude-code（古い報告）": {Term{Argv: []string{"/bin/bash"}, Status: claude, Foreground: fg("vim")}, Unsure},
	} {
		if got := c.term.Kind(); got != c.want {
			t.Errorf("%s: %v", name, got)
		}
	}
}
