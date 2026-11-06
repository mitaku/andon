package courier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// クーリエの端末の中身を戻す（courier.md §5 の P3b「端末を起こす・中身を戻す」）。端末は --keep で起こすので、core が起き直すと（hc server restart・
// App の Add Machine や Update HC で serve が替わる）名前とラベルはそのままで中身がシェルに替わる。
// hc get courier が通るだけでは「動いている」と言えないので、前に出ているもの（foreground.argv）で見る。
// 打つのは boot（毎起動）と Chief of Staff のフック（andon courier drain。毎ターン）。core の起き直しは boot を通らないが、
// クーリエが回るのは Chief of Staff が生きている台帳だけで、Chief of Staff が動けばフックが走る。

// ensureWait は、シェルに打ち込んだあと中身がクーリエに替わるのを待つ長さ（その間 lock を持ち、二重に打たない）。
var ensureWait = 3 * time.Second

// shells は、クーリエの端末で前に出ていれば「中身がシェルに替わった」とみなすもの（hc keep の替わりと、人が抜けたあと）。
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ash": true, "fish": true, "nu": true}

// courierTerm は hc get courier --json のうち、中身の見分けに読むもの。
type courierTerm struct {
	State      string `json:"state"`
	Foreground *struct {
		Argv []string `json:"argv"`
		Name string   `json:"name"`
	} `json:"foreground"`
}

// IsCourier は前に出ているものが andon courier（回り続けるもの）か。
func IsCourier(argv []string) bool {
	return len(argv) == 2 && filepath.Base(argv[0]) == "andon" && argv[1] == "courier"
}

// Ensure はクーリエの端末を確かめ、無ければ起こし、中身がシェルに替わっていれば andon courier を打ち直す。
// 何かしたらその1行を返す（何もしなければ空）。andon はパッケージの andon、cwd は建屋の repo。
// 前に何も出ていない（替わる途中）・シェルでも andon courier でもないものが出ているときは触らない（迷ったら動かさない）。
func Ensure(andon, cwd string) (string, error) {
	unlock, err := ensureLock()
	if err != nil {
		return "", nil // ほかが確かめている（boot と複数の Chief of Staff のフックが重なった）
	}
	defer unlock()
	t, gone, err := getCourier()
	switch {
	case err != nil:
		return "", err
	case gone:
		out, err := hcRun("run", "--bg", "--no-parent", "--no-notify", "--keep",
			"--name", Name, "--label", "role="+Name, "--cwd", cwd, "--", andon, "courier")
		if err != nil {
			return "", fmt.Errorf("hc run: %v: %s", err, strings.TrimSpace(out))
		}
		return "クーリエを起こした", nil
	case t.Foreground == nil:
		return "", nil
	case IsCourier(t.Foreground.Argv):
		return "", nil
	case !shells[t.Foreground.Name]:
		return "", fmt.Errorf("クーリエの端末の前に %s が出ている（andon courier でもシェルでもない。触らない）", t.Foreground.Name)
	}
	if out, err := hcRun("send", Name, shellQuote(andon)+" courier"); err != nil {
		return "", fmt.Errorf("hc send %s: %v: %s", Name, err, strings.TrimSpace(out))
	}
	for end := time.Now().Add(ensureWait); time.Now().Before(end); time.Sleep(200 * time.Millisecond) {
		if t, _, err := getCourier(); err == nil && t != nil && t.Foreground != nil && IsCourier(t.Foreground.Argv) {
			break
		}
	}
	return "クーリエの端末がシェルに替わっていたので andon courier を打ち直した", nil
}

// getCourier は hc get courier。端末が無い・終わっていれば gone。
func getCourier() (*courierTerm, bool, error) {
	cmd := exec.Command("hc", "get", Name, "--json")
	cmd.Env = withoutTerminal()
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		var e struct {
			Error struct {
				Kind string `json:"kind"`
			} `json:"error"`
		}
		if json.Unmarshal(bytes.TrimSpace(errb.Bytes()), &e) == nil && e.Error.Kind == "not_found" {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("hc get %s: %v %s", Name, err, strings.TrimSpace(errb.String()))
	}
	var t courierTerm
	if json.Unmarshal(out, &t) != nil || t.State == "" {
		return nil, false, fmt.Errorf("hc get %s の形が読めない", Name)
	}
	if t.State != "live" {
		return nil, true, nil
	}
	return &t, false, nil
}

func hcRun(args ...string) (string, error) {
	cmd := exec.Command("hc", args...)
	cmd.Env = withoutTerminal()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ensureLock は Ensure を1つずつにする lock（取れなければすぐ諦める）。
func ensureLock() (func(), error) {
	path := filepath.Join(filepath.Dir(StatePath("x")), ".terminal.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("lock が取れない")
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
