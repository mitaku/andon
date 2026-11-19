package spawn

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShortTitle(t *testing.T) {
	for in, want := range map[string]string{"課金の設計": "", "Billing の設計": "billing", "A very long English title for the design doc": "a-very-long-english-title-for"} {
		if got := ShortTitle(in); got != want {
			t.Errorf("ShortTitle(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDefaultBranchErrorAuthHint(t *testing.T) {
	auth := fmt.Errorf("exit status 128: fatal: could not read Username for 'https://github.com': No such device or address")
	got := defaultBranchError("/repo", auth).Error()
	for _, want := range []string{"/repo", "could not read Username", "gh auth status", "gh auth setup-git"} {
		if !strings.Contains(got, want) {
			t.Errorf("認証の失敗の文言に %q が無い: %s", want, got)
		}
	}
	other := defaultBranchError("/repo", fmt.Errorf("exit status 128: fatal: unable to access: Could not resolve host")).Error()
	if strings.Contains(other, "gh auth") || !strings.Contains(other, "Could not resolve host") {
		t.Errorf("認証でない失敗に案内を出さず、git の stderr は添える: %s", other)
	}
	if got := defaultBranchError("/repo", nil).Error(); strings.Contains(got, "gh auth") {
		t.Errorf("原因が無いときに案内を出さない: %s", got)

	}
}

// 最初のメッセージは、送れて動き出さなかった（started=false）のでもないときだけ届いたとみる。
// started が無い（状態を報告しない program）は誰にも分からないので届いたとみる。
func TestFirstDelivered(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name string
		f    First
		want bool
	}{
		{"送れなかった", First{Sent: false, Reason: "t9 is still starting: its output has not settled"}, false},
		{"送ったが動き出さない", First{Sent: true, Started: &no}, false},
		{"送って動き出した", First{Sent: true, Started: &yes}, true},
		{"状態を報告しない", First{Sent: true}, true},
	} {
		if got := c.f.Delivered(); got != c.want {
			t.Errorf("%s: Delivered()=%v", c.name, got)
		}
	}
	s := Started{First: First{Sent: true, Started: &no}}
	if r := s.Reason(); r != "送ったが動き出さなかった（起動の途中で入力を捨てられたとみる）" {
		t.Errorf("Reason()=%q", r)
	}
}

func TestNewSession(t *testing.T) {
	a, b := NewSession(), NewSession()
	if !SessionRe.MatchString(a) || a == b || a[14] != '4' {
		t.Errorf("NewSession: %q %q", a, b)
	}
}

func TestGuide(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	if g := Guide(); g != "" {
		t.Errorf("ファイルが無いのに入った: %q", g)
	}
	if err := os.WriteFile(filepath.Join(ws, GuideFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "\n人の指針 `" + filepath.Join(ws, GuideFile) + "` も読む。"
	if g := Guide(); g != want {
		t.Errorf("got %q want %q", g, want)
	}
}

// 案件の repo の置き場は GHQ_ROOT、無ければ ghq root。どちらも無ければ誤り。
func TestGhqRoot(t *testing.T) {
	t.Setenv("GHQ_ROOT", "/w/.ghq")
	if got, err := GhqRoot(); err != nil || got != "/w/.ghq" {
		t.Errorf("GHQ_ROOT: %q %v", got, err)
	}
	if got, err := RepoPath("github.com/example/app"); err != nil || got != filepath.Join("/w/.ghq", "github.com/example/app") {
		t.Errorf("RepoPath: %q %v", got, err)
	}
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "ghq"), []byte("#!/bin/sh\necho /from/ghq\n"), 0o755)
	t.Setenv("PATH", bin)
	t.Setenv("GHQ_ROOT", "")
	if got, err := GhqRoot(); err != nil || got != "/from/ghq" {
		t.Errorf("ghq root: %q %v", got, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := GhqRoot(); err == nil {
		t.Error("ghq が無いのに誤りにならない")
	}
}
