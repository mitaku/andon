package boot

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// setLocaltime は /etc/localtime をセルの TZ（compose が .env の TZ から渡す。既定 Asia/Tokyo）の zoneinfo に向ける。
// 環境変数の TZ は compose から起動したもの（boot・docker exec・その子）にしか届かない。sshd は環境を引き継がず、
// SetEnv（image/sshd_config.d/andon.conf）は焼いた値しか持てないので、ssh のセッション（Herdr App の端末も）は
// /etc/localtime を読む。これを TZ に揃えて、どの入り口でも同じ時刻を出す。台帳の timezone は読まない（別のもの）。
func setLocaltime(e Env) error {
	tz := e.TZ
	if tz == "" {
		return nil
	}
	// IANA の名前（Asia/Tokyo・UTC）だけを受ける。POSIX の書き方（JST-9）や ":" で始まる値・パスは zoneinfo に無い
	if strings.HasPrefix(tz, ":") || filepath.IsAbs(tz) || filepath.Clean(tz) != tz || strings.HasPrefix(tz, "..") {
		return fmt.Errorf("TZ=%q は IANA の名前でない（.env の TZ を Asia/Tokyo の形に直す）", tz)
	}
	zone := filepath.Join(e.Zoneinfo, tz)
	if fi, err := os.Stat(zone); err != nil || !fi.Mode().IsRegular() {
		return fmt.Errorf("TZ=%q の %s が無い（名前の誤りか、image に tzdata が無い）", tz, zone)
	}
	if cur, err := os.Readlink(e.Localtime); err == nil && cur == zone {
		return nil
	}
	sudo, ok := e.has("sudo")
	if !ok {
		return errors.New("sudo が無い")
	}
	if out, err := exec.Command(sudo, "-n", "ln", "-sfn", zone, e.Localtime).CombinedOutput(); err != nil {
		return fmt.Errorf("sudo ln -sfn %s %s: %v: %s", zone, e.Localtime, err, strings.TrimSpace(string(out)))
	}
	e.logf("タイムゾーン: %s → %s（.env の TZ）", e.Localtime, zone)
	return nil
}
