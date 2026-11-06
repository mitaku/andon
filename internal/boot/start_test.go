package boot

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// authorized_keys が無いときと、あるが読めないときを別の行で出す（どちらも sshd は起こさない）。
func TestStartSSHDNoKeysAndUnreadableKeys(t *testing.T) {
	f := newFixture(t)
	if err := startSSHD(f.env()); err != nil {
		t.Fatal(err)
	}
	if got := f.log.String(); !strings.Contains(got, "ssh は無効（使うなら .ssh/authorized_keys に公開鍵を置いて再起動）") {
		t.Errorf("鍵が無いとき: %q", got)
	}

	if os.Getuid() == 0 {
		t.Skip("root は mode 000 のファイルも読める")
	}
	keys := filepath.Join(f.ws, ".ssh", "authorized_keys")
	put(t, keys, "ssh-ed25519 AAAA example\n")
	if err := os.Chmod(keys, 0); err != nil {
		t.Fatal(err)
	}
	f.log.Reset()
	if err := startSSHD(f.env()); err != nil {
		t.Fatal(err)
	}
	got := f.log.String()
	if !strings.Contains(got, "ssh は無効（.ssh/authorized_keys があるが読めない: ") || !strings.Contains(got, keys) ||
		!strings.Contains(got, "permission denied") || strings.Contains(got, "公開鍵を置いて再起動") {
		t.Errorf("読めないとき: %q", got)
	}
}

// fakeGH は呼ばれた引数を calls に1行ずつ足す gh。status が 0 以外を返すと未ログイン。
func fakeGH(t *testing.T, f *fixture, statusRC int) (e Env, calls string) {
	t.Helper()
	calls = filepath.Join(f.root, "gh-calls")
	gh := filepath.Join(f.root, "gh")
	put(t, gh, "#!/bin/sh\necho \"$*\" >> "+calls+"\n[ \"$*\" = \"auth status\" ] && exit "+strconv.Itoa(statusRC)+"\nexit 0\n")
	if err := os.Chmod(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	e = f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "gh" {
			return gh, nil
		}
		return "", errors.New("無い")
	}
	return e, calls
}

func TestSetupGitCredentialsLoggedIn(t *testing.T) {
	f := newFixture(t)
	e, calls := fakeGH(t, f, 0)
	for range 2 { // 毎起動打っても同じ
		if err := setupGitCredentials(e); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(calls)
	if want := "auth status\nauth setup-git\nauth status\nauth setup-git\n"; string(b) != want {
		t.Errorf("gh の呼び出し: %q", b)
	}
}

func TestSetupGitCredentialsNotLoggedIn(t *testing.T) {
	f := newFixture(t)
	e, calls := fakeGH(t, f, 1)
	if err := setupGitCredentials(e); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(calls)
	if string(b) != "auth status\n" {
		t.Errorf("未ログインで setup-git を打った: %q", b)
	}
	if !strings.Contains(f.log.String(), "gh はログインしていない") {
		t.Errorf("ログ: %q", f.log.String())
	}
}

func TestSetupGitCredentialsNoGH(t *testing.T) {
	f := newFixture(t)
	if err := setupGitCredentials(f.env()); err != nil || f.log.Len() != 0 {
		t.Errorf("gh が無いときは何もしない: %v %q", err, f.log.String())
	}
}
