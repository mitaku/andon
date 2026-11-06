package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"andon/internal/spawn"
)

// dispatcher の端末の中身を戻す（dispatcher.md §5 の P3b「端末を起こす・中身を戻す」）。端末は --keep で起こすので、core が起き直すと（hc server restart・
// App の Add Machine や Update HC で serve が替わる）名前とラベルはそのままで中身がシェルに替わる。
// hc get dispatcher が通るだけでは「動いている」と言えないので、前に出ているもの（foreground.argv）で見る。
// 中身がシェルなら、そのシェルには打ち込まず、端末を片付けて起こし直す。
// 打つのは boot（毎起動）と Chief of Staff のフック（andon dispatcher drain。毎ターン）。core の起き直しは boot を通らないが、
// dispatcher が回るのは Chief of Staff が生きている台帳だけで、Chief of Staff が動けばフックが走る。

// shells は、dispatcher の端末で前に出ていれば「中身がシェルに替わった」とみなすもの（hc keep の替わりと、人が抜けたあと）。
var shells = spawn.Shells

// ErrBusy は、ほかが確かめている（boot と複数の Chief of Staff のフックが重なった）ので何もしなかった。
var ErrBusy = errors.New("ほかが dispatcher の端末を確かめている")

// ErrUnsure は、待っても前に出ているものが取れず、動いているかを確かめられなかった（作り直しで戻った直後など）。
// 「動いている」とはみなさない。
var ErrUnsure = errors.New("dispatcher の端末を確かめられなかった")

// ensurePoll は前に出ているものを読み直す間隔。
var ensurePoll = 500 * time.Millisecond

// dispatcherTerm は hc get dispatcher --json のうち、中身の見分けに読むもの。
type dispatcherTerm struct {
	Address    string `json:"address"`
	State      string `json:"state"`
	Foreground *struct {
		Argv []string `json:"argv"`
		Name string   `json:"name"`
	} `json:"foreground"`
}

// IsDispatcher は前に出ているものが andon dispatcher（回り続けるもの）か。
func IsDispatcher(argv []string) bool {
	return len(argv) == 2 && filepath.Base(argv[0]) == "andon" && argv[1] == "dispatcher"
}

// Ensure は dispatcher の端末を確かめ、無ければ起こし、中身がシェルに替わっていれば端末を片付けて起こし直す。
// 何かしたらその1行を返す（動いていて何もしなければ空）。andon はパッケージの andon、cwd は建屋の repo。
// 前に何も出ていない（作り直しで戻った直後・替わる途中）あいだは wait まで読み直す。取れないままなら ErrUnsure、
// ほかが確かめていれば ErrBusy を返す（どちらも「動いている」ではない）。
// シェルには打ち込まない（入力欄に人の打ちかけがあれば、それに足して実行してしまう）。dispatcher 専用の端末なので、
// keep を外して kill・rm し、同じ名前・ラベルで hc run し直す。
// シェルでも andon dispatcher でもないもの（人が何か動かしている）が出ているときは触らない。
func Ensure(andon, cwd string, wait time.Duration) (string, error) {
	unlock, err := ensureLock()
	if err != nil {
		return "", err // ErrBusy か、控えの置き場が作れない・開けない（起こせなかった理由を呼び手に見せる）
	}
	defer unlock()
	t, gone, err := getDispatcher()
	for end := time.Now().Add(wait); err == nil && !gone && t.Foreground == nil && time.Now().Before(end); {
		time.Sleep(ensurePoll)
		t, gone, err = getDispatcher()
	}
	switch {
	case err != nil:
		return "", err
	case gone:
		if err := runDispatcher(andon, cwd); err != nil {
			return "", err
		}
		return "dispatcher を起こした", nil
	case t.Foreground == nil:
		return "", fmt.Errorf("%w（%s は %v 待っても前に出ているものが取れない）", ErrUnsure, t.Address, wait)
	case IsDispatcher(t.Foreground.Argv):
		return "", nil
	case !shells[t.Foreground.Name]:
		return "", fmt.Errorf("dispatcher の端末の前に %s が出ている（andon dispatcher でもシェルでもない。触らない）", t.Foreground.Name)
	case t.Address == "":
		return "", fmt.Errorf("hc get %s に address が無い", Name)
	}
	// 名前で引くと片付けた端末（finished）に当たりうるので、address で打つ
	for _, args := range [][]string{{"keep", t.Address, "--off"}, {"kill", t.Address}, {"rm", t.Address}} {
		if out, err := hcRun(args...); err != nil {
			return "", fmt.Errorf("hc %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(out))
		}
	}
	if err := runDispatcher(andon, cwd); err != nil {
		return "", err
	}
	return fmt.Sprintf("dispatcher の端末（%s）の中身がシェルに替わっていたので、片付けて起こし直した", t.Address), nil
}

// hostName は建屋の名前（hostname）。建屋の台帳が2つ以上のときの Space 名に使う。テストで差し替える。
var hostName = os.Hostname

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Place は dispatcher の置き場（herdr.space・herdr.tab のラベルの値）。ws は建屋の repo。
// 台帳（ws/projects/*/.flightdeck/config.yaml）が1つなら、その指揮側の Space andon-<slug>。
// それ以外（2つ以上。無いときも）は建屋の名前の Space andon-<建屋の slug>-plant。tab は andon-<slug または建屋の slug>-dispatcher。
// 名前は Andon の App の mod が付ける（ADR-0002 決めたこと 21）。
func Place(ws string) (space, tab string) {
	cfgs, _ := filepath.Glob(filepath.Join(ws, "projects", "*", ".flightdeck", "config.yaml"))
	if len(cfgs) == 1 {
		slug := filepath.Base(filepath.Dir(filepath.Dir(cfgs[0])))
		return "andon-" + slug, "andon-" + slug + "-dispatcher"
	}
	h, _ := hostName()
	plant := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(h), "-"), "-")
	if plant == "" {
		plant = "plant"
	}
	return "andon-" + plant + "-plant", "andon-" + plant + "-dispatcher"
}

func runDispatcher(andon, cwd string) error {
	space, tab := Place(cwd)
	out, err := hcRun("run", "--bg", "--no-parent", "--no-notify", "--keep",
		"--name", Name, "--label", "role="+Name, "--label", "herdr.space="+space, "--label", "herdr.tab="+tab,
		"--cwd", cwd, "--", andon, "dispatcher")
	if err != nil {
		return fmt.Errorf("hc run: %v: %s", err, strings.TrimSpace(out))
	}
	return nil
}

// getDispatcher は hc get dispatcher。端末が無い・終わっていれば gone。
func getDispatcher() (*dispatcherTerm, bool, error) {
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
	var t dispatcherTerm
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

// ensureLock は Ensure を1つずつにする lock（ほかが持っていれば ErrBusy ですぐ諦める。ほかの失敗はそのまま返す）。
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
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock が取れない: %v", err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
