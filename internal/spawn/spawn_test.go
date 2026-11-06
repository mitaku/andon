package spawn

import (
	"fmt"
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
