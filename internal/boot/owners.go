package boot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// HomeVolumes は package/compose.yaml が HOME（/opt/andon-home）の下に重ねる volume の置き場（HOME からの相対）。
var HomeVolumes = []string{
	".claude", ".local/share/claude", ".codex", ".agents", ".config/gh", ".config/andon/signals", ".config/andon/digest", ".config/herdr",
	".config/herdr-core", ".ssh", ".portless", ".local/state",
	".local/share/mise",
}

func fixOwners(e Env) error {
	var errs []error
	for _, rel := range HomeVolumes {
		dir := filepath.Join(e.Home, rel)
		st, err := os.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			errs = append(errs, err)
			continue
		}
		sys, ok := st.Sys().(*syscall.Stat_t)
		if !ok || (int(sys.Uid) == e.Uid && int(sys.Gid) == e.Gid) {
			continue
		}
		sudo, err := e.LookPath("sudo")
		if err != nil {
			return fmt.Errorf("%s のオーナーが uid %d で、sudo が無い", dir, sys.Uid)
		}
		owner := fmt.Sprintf("%d:%d", e.Uid, e.Gid)
		if out, err := exec.Command(sudo, "-n", "chown", "-R", owner, dir).CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("sudo chown -R %s %s: %v: %s", owner, dir, err, strings.TrimSpace(string(out))))
			continue
		}
		e.logf("置き場のオーナーを直した: %s（uid %d → %d）", dir, sys.Uid, e.Uid)
	}
	return errors.Join(errs...)
}
