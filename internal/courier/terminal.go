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
)

// クーリエの端末の中身を戻す（courier.md §5 の P3b「端末を起こす・中身を戻す」）。端末は --keep で起こすので、core が起き直すと（hc server restart・
// App の Add Machine や Update HC で serve が替わる）名前とラベルはそのままで中身がシェルに替わる。
// hc get courier が通るだけでは「動いている」と言えないので、前に出ているもの（foreground.argv）で見る。
// 中身がシェルなら、そのシェルには打ち込まず、端末を片付けて起こし直す。
// 打つのは boot（毎起動）と Chief of Staff のフック（andon courier drain。毎ターン）。core の起き直しは boot を通らないが、
// クーリエが回るのは Chief of Staff が生きている台帳だけで、Chief of Staff が動けばフックが走る。

// shells は、クーリエの端末で前に出ていれば「中身がシェルに替わった」とみなすもの（hc keep の替わりと、人が抜けたあと）。
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ash": true, "fish": true, "nu": true}

// courierTerm は hc get courier --json のうち、中身の見分けに読むもの。
type courierTerm struct {
	Address    string `json:"address"`
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

// Ensure はクーリエの端末を確かめ、無ければ起こし、中身がシェルに替わっていれば端末を片付けて起こし直す。
// 何かしたらその1行を返す（何もしなければ空）。andon はパッケージの andon、cwd は建屋の repo。
// シェルには打ち込まない（入力欄に人の打ちかけがあれば、それに足して実行してしまう）。クーリエ専用の端末なので、
// keep を外して kill・rm し、同じ名前・ラベルで hc run し直す。
// 前に何も出ていない（替わる途中）・シェルでも andon courier でもないもの（人が何か動かしている）が出ているときは触らない。
func Ensure(andon, cwd string) (string, error) {
	unlock, err := ensureLock()
	if errors.Is(err, errLocked) {
		return "", nil // ほかが確かめている（boot と複数の Chief of Staff のフックが重なった）
	} else if err != nil {
		return "", err // 控えの置き場が作れない・開けない（起こせなかった理由を呼び手に見せる）
	}
	defer unlock()
	t, gone, err := getCourier()
	switch {
	case err != nil:
		return "", err
	case gone:
		if err := runCourier(andon, cwd); err != nil {
			return "", err
		}
		return "クーリエを起こした", nil
	case t.Foreground == nil:
		return "", nil
	case IsCourier(t.Foreground.Argv):
		return "", nil
	case !shells[t.Foreground.Name]:
		return "", fmt.Errorf("クーリエの端末の前に %s が出ている（andon courier でもシェルでもない。触らない）", t.Foreground.Name)
	case t.Address == "":
		return "", fmt.Errorf("hc get %s に address が無い", Name)
	}
	// 名前で引くと片付けた端末（finished）に当たりうるので、address で打つ
	for _, args := range [][]string{{"keep", t.Address, "--off"}, {"kill", t.Address}, {"rm", t.Address}} {
		if out, err := hcRun(args...); err != nil {
			return "", fmt.Errorf("hc %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(out))
		}
	}
	if err := runCourier(andon, cwd); err != nil {
		return "", err
	}
	return fmt.Sprintf("クーリエの端末（%s）の中身がシェルに替わっていたので、片付けて起こし直した", t.Address), nil
}

func runCourier(andon, cwd string) error {
	out, err := hcRun("run", "--bg", "--no-parent", "--no-notify", "--keep",
		"--name", Name, "--label", "role="+Name, "--cwd", cwd, "--", andon, "courier")
	if err != nil {
		return fmt.Errorf("hc run: %v: %s", err, strings.TrimSpace(out))
	}
	return nil
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

// errLocked は、ほかが lock を持っている（Ensure を1つずつにする）。
var errLocked = errors.New("ほかがクーリエの端末を確かめている")

// ensureLock は Ensure を1つずつにする lock（ほかが持っていれば errLocked ですぐ諦める。ほかの失敗はそのまま返す）。
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
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errLocked
		}
		return nil, fmt.Errorf("lock が取れない: %v", err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
