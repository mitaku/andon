package tray

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"andon/internal/delegate"
	"andon/internal/flock"
	"andon/internal/pkgversion"
)

const ServeUsage = "usage: andon tray serve\n" +
	"ローカルの tray を起動する常駐（andon boot が起動する。ADR-0064 決めたこと 6・26）。Node で tray/scripts/serve.mjs（miniflare）を動かす。\n" +
	"一覧の面と報告の面は 127.0.0.1:<TRAY_PORT>（セルの .env。既定 " + DefaultPort + "）に開き、portless の名前 tray と tray-view で見る。\n" +
	"アップロードの口は 127.0.0.1 の空いた番号に開く。宛先は状態の置き場の serve.json に書き、止まると消す（送る側はそこから引く）。\n" +
	"データは ~/.local/state/andon/tray/data に置く（volume state の下）。もう動いていれば何もせずに 0。\n"

// DefaultPort は一覧の面の番号の既定（前の files サーバの番号を引き継ぐ。セルの .env の TRAY_PORT で替える）。
const DefaultPort = "8907"

// defaultPortlessPort は portless の番号の既定（boot と同じ）。
const defaultPortlessPort = "1355"

// readyWait は serve.mjs が1行の JSON を出すまで待つ長さ（miniflare の起動と D1 の移行）。
var readyWait = 60 * time.Second

// Dir は tray をビルドした成果物の置き場（パッケージの置き場の tray/）。
func Dir() string { return filepath.Join(pkgversion.Home(), "tray") }

// LogPath は boot が起動した serve の出力の置き場。
func LogPath() string { return filepath.Join(os.TempDir(), "andon-tray.log") }

// Ready は tray を起動できるか（Node と、ビルドした成果物があるか）。できなければ理由。
func Ready(lookPath func(string) (string, error)) (node string, err error) {
	node, err = lookPath("node")
	if err != nil {
		return "", errors.New("node が無い")
	}
	for _, rel := range []string{"scripts/serve.mjs", "dist/workers/local.js", "dist/workers/local-upload.js"} {
		if _, err := os.Stat(filepath.Join(Dir(), rel)); err != nil {
			return "", fmt.Errorf("%s が無い（イメージの tray をビルドしていない）", filepath.Join(Dir(), rel))
		}
	}
	return node, nil
}

func ServeMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprint(stderr, ServeUsage)
		return 2
	}
	if err := serve(stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "andon tray serve: %v\n", err)
		return 1
	}
	return 0
}

func serve(stdout, stderr io.Writer) error {
	node, err := Ready(exec.LookPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		return err
	}
	// 1つだけ動く（鍵は動いている間ずっと持つ）
	unlock, err := flock.TryLock(LockPath())
	if err != nil {
		if c, rerr := ReadControl(); rerr == nil {
			fmt.Fprintf(stdout, "もう動いている: %s\n", c.ListOrigin)
			return nil
		}
		return fmt.Errorf("ほかの andon tray serve が鍵を持っている: %v", err)
	}
	defer unlock()
	_ = os.Remove(ControlPath()) // 前に落ちた serve の控え（宛先はもう無い）

	cell, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("ホスト名を読めない: %v", err)
	}
	ws := delegate.Workspace()
	cmd := exec.Command(node, filepath.Join(Dir(), "scripts", "serve.mjs"),
		"--cell", cell, "--data", filepath.Join(StateDir(), "data"),
		"--port", delegate.Dotenv(ws, "TRAY_PORT", DefaultPort), "--upload-port", "0",
		"--public-port", delegate.Dotenv(ws, "PORTLESS_PORT", defaultPortlessPort))
	cmd.Dir = Dir()
	cmd.Stderr = stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(sig)

	line := make(chan string, 1)
	r := bufio.NewReader(out)
	go func() {
		l, _ := r.ReadString('\n')
		line <- l
		_, _ = io.Copy(stderr, r) // 残りの出力は stderr（boot のログ）へ
	}()
	stop := func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}
	var c Control
	select {
	case l := <-line:
		if err := json.Unmarshal([]byte(strings.TrimSpace(l)), &c); err != nil || c.Upload == "" || c.List == "" {
			stop()
			return fmt.Errorf("serve.mjs の1行目が読めない: %q", strings.TrimSpace(l))
		}
	case s := <-sig:
		stop()
		return fmt.Errorf("起動の途中で %v を受けた", s)
	case <-time.After(readyWait):
		stop()
		return fmt.Errorf("serve.mjs が %v のうちに動き始めない", readyWait)
	}
	c.PID = os.Getpid()
	if err := WriteControl(&c); err != nil {
		stop()
		return err
	}
	defer removeOwnControl()
	fmt.Fprintf(stdout, "ローカルの tray: %s （アップロードの口 %s）\n", c.ListOrigin, c.Upload)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return fmt.Errorf("serve.mjs が止まった: %v", err)
	case <-sig:
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done: //nolint:errcheck // 止めた子の終わり方は問わない
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done //nolint:errcheck // 止めた子の終わり方は問わない
		}
		return nil
	}
}

// removeOwnControl は控えが自分の書いたものなら消す（止まったあとに古い宛先へ送らせない）。
func removeOwnControl() {
	if c, err := ReadControl(); err == nil && c.PID == os.Getpid() {
		_ = os.Remove(ControlPath())
	}
}

// LockPath は serve が動いている間ずっと持つ鍵。
func LockPath() string { return filepath.Join(StateDir(), "serve.lock") }

// Running は控えがあり、書いた serve がまだ動いているか。pid が生きているだけでは決めない（状態の置き場は volume で、
// コンテナを作り直すと止まりそこねた控えが残り、その pid を別のプロセスが持つことがある）。serve の鍵を誰かが持っていれば動いている。
func Running() (*Control, bool) {
	c, err := ReadControl()
	if err != nil {
		return nil, false
	}
	if c.PID <= 0 || syscall.Kill(c.PID, 0) != nil {
		return c, false
	}
	if unlock, err := flock.TryLock(LockPath()); err == nil {
		unlock()
		return c, false
	}
	return c, true
}
