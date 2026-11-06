package boot

import (
	"os"
	"path/filepath"
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
