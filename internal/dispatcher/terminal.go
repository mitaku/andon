package dispatcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"andon/internal/flock"
	"andon/internal/spawn"
)

// dispatcher の端末の中身を戻す（dispatcher.md §5 の P3b「端末を起動する・中身を戻す」）。端末は --keep で起動するので、core が起き直すと（hc server restart・
// App の Add Machine や Update HC で serve が替わる）名前とラベルはそのままで中身がシェルに替わる。
// hc get dispatcher が通るだけでは「動いている」と言えないので、前に出ているもの（foreground.argv）で見る。
// 中身がシェルなら、そのシェルには打ち込まず、端末を片付けて起動し直す。
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
	Address    string            `json:"address"`
	State      string            `json:"state"`
	Labels     map[string]string `json:"labels"`
	Foreground *struct {
		Argv []string `json:"argv"`
		Name string   `json:"name"`
	} `json:"foreground"`
}

// IsDispatcher は前に出ているものが andon dispatcher（回り続けるもの）か。
func IsDispatcher(argv []string) bool {
	return len(argv) == 2 && filepath.Base(argv[0]) == "andon" && argv[1] == "dispatcher"
}

// Ensure は dispatcher の端末を確かめ、無ければ起動し、中身がシェルに替わっていれば端末を片付けて起動し直す。
// andon dispatcher が動いていても、置き場のラベル（herdr.space・herdr.tab）が今の Place と違えば（台帳の数が変わった）、
// 見回りの合間（PassLock が取れたとき）だけ同じ手で片付けて起動し直す。見回りの途中なら触らず、次に打たれたときに任せる。
// 何かしたらその1行を返す（動いていて何もしなければ空）。andon はパッケージの andon、cwd はセルの repo。
// 前に何も出ていない（作り直しで戻った直後・替わる途中）あいだは wait まで読み直す。取れないままなら ErrUnsure、
// ほかが確かめていれば ErrBusy を返す（どちらも「動いている」ではない）。
// シェルには打ち込まない（入力欄に人の打ちかけがあれば、それに足して実行してしまう）。dispatcher 専用の端末なので、
// keep を外して kill・rm し、同じ名前・ラベルで hc run し直す。
// シェルでも andon dispatcher でもないもの（人が何か動かしている）が出ているときは触らない。
func Ensure(andon, cwd string, wait time.Duration) (string, error) {
	return ensureTerm(andon, cwd, wait, nil)
}

// EnsureAfterCreate は andon create のあとの Ensure。置き直すときに見回りの途中なら、終わるまで待つ（期限は無い。
// 台帳を足したあと、フックが来なくても置き場を合わせる）。待ち始めるときに waiting を1回呼ぶ。
func EnsureAfterCreate(andon, cwd string, wait time.Duration, waiting func()) (string, error) {
	return ensureTerm(andon, cwd, wait, waiting)
}

// ErrOldDispatcher は、置き場が今の Place と違うが、動いている dispatcher が見回りの間 PassLock を持たない前の版なので
// 置き直さなかった（見回りの途中で止めうる）。端末が起き直す（core の起き直し・セルの作り直し）と今の版で合う。
var ErrOldDispatcher = errors.New("動いている dispatcher は見回りの間 lock を持たない前の版なので、置き直さない")

// ensureTerm の waiting が nil なら、見回りの途中のときは待たずに触らない。
func ensureTerm(andon, cwd string, wait time.Duration, waiting func()) (string, error) {
	unlock, err := ensureLock()
	if err != nil {
		return "", err // ErrBusy か、控えの置き場が作れない・開けない（起動できなかった理由を呼び手に見せる）
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
		return "dispatcher を起動した", nil
	case t.Foreground == nil:
		return "", fmt.Errorf("%w（%s は %v 待っても前に出ているものが取れない）", ErrUnsure, t.Address, wait)
	case IsDispatcher(t.Foreground.Argv):
		space, tab := Place(cwd)
		if t.Labels["herdr.space"] == space && t.Labels["herdr.tab"] == tab {
			return "", nil
		}
		if t.Address == "" {
			return "", fmt.Errorf("hc get %s に address が無い", Name)
		}
		// PassLock のファイルは、この版の dispatcher が見回るたびに作る。無ければ動いているのは前の版で、
		// lock が取れても見回りの合間とは限らない
		if _, err := os.Stat(PassLock()); errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w（%s の置き場は %s・%s、今の台帳の数では %s・%s）", ErrOldDispatcher, t.Address, t.Labels["herdr.space"], t.Labels["herdr.tab"], space, tab)
		}
		// 見回りの途中で止めない。lock は起動し直すまで持つ（新しい dispatcher は外れるのを待ってから回る）
		unpass, err := flock.TryLock(PassLock())
		if errors.Is(err, flock.ErrBusy) && waiting != nil {
			waiting()
			unpass, err = flock.Lock(PassLock())
		}
		if errors.Is(err, flock.ErrBusy) {
			return "", nil
		} else if err != nil {
			return "", err
		}
		defer unpass()
		if err := respawn(andon, cwd, t.Address); err != nil {
			return "", err
		}
		return fmt.Sprintf("dispatcher の端末（%s）の置き場が今の台帳の数に合っていなかったので、%s・%s に起動し直した", t.Address, space, tab), nil
	case !shells[t.Foreground.Name]:
		return "", fmt.Errorf("dispatcher の端末の前に %s が出ている（andon dispatcher でもシェルでもない。触らない）", t.Foreground.Name)
	case t.Address == "":
		return "", fmt.Errorf("hc get %s に address が無い", Name)
	}
	if err := respawn(andon, cwd, t.Address); err != nil {
		return "", err
	}
	return fmt.Sprintf("dispatcher の端末（%s）の中身がシェルに替わっていたので、片付けて起動し直した", t.Address), nil
}

// hostName はセルの名前（hostname）。セルの台帳が2つ以上のときの Space 名に使う。テストで差し替える。
var hostName = os.Hostname

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Place は dispatcher の置き場（herdr.space・herdr.tab のラベルの値）。ws はセルの repo。
// 台帳（ws/projects/*/kanban/config.yaml）が1つなら、その指揮側の Space andon-<slug>。
// それ以外（2つ以上。無いときも）はセルの名前の Space andon-<セルの slug>-plant。tab は andon-<slug またはセルの slug>-dispatcher。
// 名前は Andon の App の mod が付ける（ADR-0002 決めたこと 21）。
func Place(ws string) (space, tab string) {
	cfgs, _ := filepath.Glob(filepath.Join(ws, "projects", "*", "kanban", "config.yaml"))
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

// respawn は dispatcher の端末 addr を片付けて（keep を外して kill・rm。hc kill は終わるまで待つ）、起動し直す。
// 名前で引くと片付けた端末（finished）に当たりうるので、address で打つ。
func respawn(andon, cwd, addr string) error {
	for _, args := range [][]string{{"keep", addr, "--off"}, {"kill", addr}, {"rm", addr}} {
		if out, err := hcRun(args...); err != nil {
			return fmt.Errorf("hc %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(out))
		}
	}
	return runDispatcher(andon, cwd)
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
