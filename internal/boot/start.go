package boot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"andon/internal/delegate"
	"andon/internal/dispatcher"
	"andon/internal/otel"
	"andon/internal/spawn"
	"andon/internal/tray"
)

const DefaultPortlessPort = "1355"

// Start は起動の手を順に打つ。道具が無ければ飛ばし、失敗は警告にして続ける。
func Start(e Env) {
	oldLive := false
	for _, step := range []struct {
		name string
		run  func(Env) error
	}{
		{"Claude Code の記録", fixClaudeInstall},
		{"git の資格情報", setupGitCredentials},
		{"portless", startPortless},
		{"ローカルの tray", startTray},
		{"ハーネスの受け口", startOtel},
		{"sshd", startSSHD},
		{"前のバージョンの係の端末", dropStaleRoles},
		{"dispatcher", startDispatcher},
		{"外の skill", installGhSkills},
	} {
		if step.name == "dispatcher" && oldLive {
			e.logf("前のバージョンの係の端末が残っているので dispatcher は起動しない（二重に回さない）。オーナーが hc kill <address> で片付けてから andon boot を打ち直す")
			continue
		}
		err := step.run(e)
		if errors.Is(err, errOldDispatcher) {
			oldLive = true
		}
		if err != nil {
			e.logf("%s の準備に失敗（続行する）: %v", step.name, err)
		}
	}
}

func (e Env) has(tool string) (string, bool) {
	p, err := e.LookPath(tool)
	return p, err == nil
}

func (e Env) dotenv(key, def string) string { return delegate.Dotenv(e.Workspace, key, def) }

func listening(port string) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 500*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close() //nolint:errcheck // 届くかを見るだけの接続
	return true
}

func detach(logPath, name string, args ...string) error {
	out, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck // 子の process は自分の写しで書くので、親の写しを閉じる失敗は出力に効かない
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// detachWatched は detach と同じく新しいセッションで手放して起動し、子が終わったら閉じる channel を返す（起動の途中で落ちたことを
// 待ちの長さを使い切らずに知る。boot が先に終われば、子はそのまま動き続ける）。
func detachWatched(logPath, name string, args ...string) (<-chan struct{}, error) {
	out, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	defer out.Close() //nolint:errcheck // 子の process は自分の写しで書くので、親の写しを閉じる失敗は出力に効かない
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return done, nil
}

// logTail はログの最後の空でない行（読めなければ空）。
func logTail(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func waitListening(port string, d time.Duration) bool {
	for end := time.Now().Add(d); time.Now().Before(end); time.Sleep(200 * time.Millisecond) {
		if listening(port) {
			return true
		}
	}
	return false
}

// setupGitCredentials は gh がログイン済みなら `gh auth setup-git` を打つ（git の credential helper を gh にする）。
// ~/.gitconfig は volume の外なので、作り直すと消える。毎起動打てば戻る（何度打っても同じ結果）。
// ログインしていなければ何もせず1行だけ残す。gh の出力は token を含みうるのでログに出さない。
func setupGitCredentials(e Env) error {
	gh, ok := e.has("gh")
	if !ok {
		return nil
	}
	run := func(args ...string) error {
		cmd := exec.Command(gh, args...)
		cmd.Env = append(os.Environ(), "HOME="+e.Home)
		return cmd.Run()
	}
	if run("auth", "status") != nil {
		e.logf("gh はログインしていない。git の資格情報は設定しない（gh auth login のあと、再起動か gh auth setup-git）")
		return nil
	}
	if err := run("auth", "setup-git"); err != nil {
		return fmt.Errorf("gh auth setup-git: %w", err)
	}
	e.logf("git の credential helper を gh にした")
	return nil
}

// fixClaudeInstall は .claude.json にネイティブのインストールの記録（"installMethod": "native"）が無ければ claude install latest で書き直す
// （無いと自己更新が disabled 扱いになる）。
func fixClaudeInstall(e Env) error {
	claude := filepath.Join(e.Home, ".local", "bin", "claude")
	if st, err := os.Stat(claude); err != nil || st.Mode()&0o111 == 0 {
		return nil
	}
	cj := filepath.Join(e.Home, ".claude.json")
	if e.ClaudeDir != "" {
		cj = filepath.Join(e.ClaudeDir, ".claude.json")
	}
	b, _ := os.ReadFile(cj)
	if regexp.MustCompile(`"installMethod":\s*"native"`).Match(b) {
		return nil
	}
	if out, err := exec.Command(claude, "install", "latest").CombinedOutput(); err != nil {
		return fmt.Errorf("claude install latest: %v: %s", err, strings.TrimSpace(string(out)))
	}
	e.logf("Claude Code: ネイティブのインストールの記録を書き直した")
	return nil
}

func startPortless(e Env) error {
	portless, ok := e.has("portless")
	if !ok {
		return nil
	}
	port := e.dotenv("PORTLESS_PORT", DefaultPortlessPort)
	if listening(port) {
		e.logf("portless はすでに動いている (:%s)", port)
		return nil
	}
	logPath := filepath.Join(os.TempDir(), "portless.log")
	if err := detach(logPath, portless, "proxy", "start", "--no-tls", "-p", port, "--tld", e.Hostname+".localhost"); err != nil {
		return err
	}
	if !waitListening(port, 5*time.Second) {
		return fmt.Errorf("待ち受けが上がらない（%s）", logPath)
	}
	e.logf("portless を起動した (http://<app>.%s.localhost:%s)", e.Hostname, port)
	return nil
}

// trayWait は andon tray serve が控えを書くまで待つ長さ（miniflare の起動と D1 の移行）。
var trayWait = 60 * time.Second

// startTray はローカルの tray（andon tray serve）を起動し、portless の名前 tray（一覧の面）と tray-view（報告の面）を付ける
// （ADR-0064 決めたこと 6・26。files サーバの代わり）。動いていれば起動しない。Node かビルドした tray が無い・自分が andon の
// バイナリでない（go test の偽の boot）ときは、理由を1行残して起動しない。
func startTray(e Env) error {
	if !otel.IsAndon(e.Self) {
		return nil
	}
	if _, err := tray.Ready(e.LookPath); err != nil {
		e.logf("ローカルの tray は起動しない（%v）。報告は files/ に写る", err)
		return nil
	}
	c, running := tray.Running()
	if !running {
		exited, err := detachWatched(tray.LogPath(), e.Self, "tray", "serve")
		if err != nil {
			return err
		}
		for end := time.Now().Add(trayWait); ; time.Sleep(200 * time.Millisecond) {
			if c, running = tray.Running(); running {
				break
			}
			select {
			case <-exited:
				return fmt.Errorf("andon tray serve が止まった: %s（%s）", logTail(tray.LogPath()), tray.LogPath())
			default:
			}
			if time.Now().After(end) {
				return fmt.Errorf("andon tray serve が動き始めない（%s）", tray.LogPath())
			}
		}
	}
	port := c.List[strings.LastIndex(c.List, ":")+1:]
	if portless, ok := e.has("portless"); ok {
		for _, name := range []string{"tray", "tray-view"} {
			if out, err := exec.Command(portless, "alias", name, port).CombinedOutput(); err != nil {
				return fmt.Errorf("portless alias %s %s: %v: %s", name, port, err, strings.TrimSpace(string(out)))
			}
		}
	}
	e.logf("ローカルの tray: %s/ （報告の一覧。データは %s）", c.ListOrigin, tray.StateDir())
	return nil
}

// startOtel はハーネスのトレースとログの受け口（andon otel serve）を、ほかの常駐と同じく手放して起動する（ADR-0060 決めたこと 4）。
// andon の受け口がもう listen していれば何もしない。役は hc の端末からしか起動しないので、hc が無ければ起動しない
// （送り手が無い。偽の PATH で boot を打つ検査が本物のポートに受け口を残さない）。起動し直す所は役の起動の口（andon otel env）にもある。
func startOtel(e Env) error {
	if _, ok := e.has("hc"); !ok || !otel.IsAndon(e.Self) {
		return nil
	}
	if why := otel.Ensure(e.Self); why != "" {
		return errors.New(why)
	}
	e.logf("ハーネスの受け口: http://%s （トレースとログを ~/.local/state/andon/otel/ に書く）", otel.Addr)
	return nil
}

// 設定は image に焼いた sshd_config。ホスト鍵は volume の ~/.ssh に置き、`-h` で sshd に渡す（渡さないと image の /etc/ssh の鍵が使われ、image を焼き直すたびに変わる）。
func startSSHD(e Env) error {
	keys := filepath.Join(e.Workspace, ".ssh", "authorized_keys")
	b, err := os.ReadFile(keys)
	if errors.Is(err, fs.ErrNotExist) {
		e.logf("ssh は無効（使うなら .ssh/authorized_keys に公開鍵を置いて再起動）")
		return nil
	} else if err != nil {
		e.logf("ssh は無効（.ssh/authorized_keys があるが読めない: %v。セルの repo のオーナーと andon の uid が同じか確かめる）", err)
		return nil
	}
	sudo, ok := e.has("sudo")
	keygen, ok2 := e.has("ssh-keygen")
	if !ok || !ok2 {
		return nil
	}
	ssh := filepath.Join(e.Home, ".ssh")
	if err := os.MkdirAll(ssh, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(ssh, 0o700); err != nil {
		return err
	}
	hostKey := filepath.Join(ssh, "ssh_host_ed25519_key")
	if _, err := os.Stat(hostKey); err != nil {
		if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-f", hostKey).CombinedOutput(); err != nil {
			return fmt.Errorf("ssh-keygen: %v: %s", err, out)
		}
		e.logf("ssh のホスト鍵を生成した")
	}
	if err := e.writeFile(filepath.Join(ssh, "authorized_keys"), b, 0o600); err != nil {
		return err
	}
	// /run/sshd は root 所有でないと sshd が起動を拒む。/run は起動ごとに作り直されることがある。
	for _, args := range [][]string{{"mkdir", "-p", "/run/sshd"}, {"chown", "root:root", "/run/sshd"}, {"chmod", "755", "/run/sshd"}} {
		if out, err := exec.Command(sudo, append([]string{"-n"}, args...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("sudo %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	if out, err := exec.Command(sudo, "-n", "/usr/sbin/sshd", "-h", hostKey).CombinedOutput(); err != nil {
		return fmt.Errorf("sshd: %v: %s", err, strings.TrimSpace(string(out)))
	}
	e.logf("sshd を起動した（コンテナの 22 番。ホスト側の番号は .env の SSH_PORT）")
	return nil
}

const oldDispatcher = "courier"

var errOldDispatcher = errors.New("前のバージョンの係の端末が残っている")

func dropStaleRoles(e Env) error {
	hc, ok := e.has("hc")
	if !ok {
		return nil
	}
	// 作り直しの直後は、戻った端末の前に出ているものがまだ取れない（foreground が無い）ことがある。
	end := time.Now().Add(dispatcherWait)
	var t spawn.Term
	for {
		got, err := spawn.Output(nil, "", hc, "get", oldDispatcher, "--json")
		if err != nil {
			return nil
		}
		t = spawn.Term{}
		if json.Unmarshal([]byte(got), &t) != nil || t.State != "live" || t.Address == "" ||
			t.Name != oldDispatcher || t.Labels["role"] != oldDispatcher {
			return nil
		}
		if t.Foreground != nil || !time.Now().Before(end) {
			break
		}
		time.Sleep(stalePoll)
	}
	if t.Foreground == nil || (!t.Shell() && !isOldDispatcher(t.Foreground.Argv)) {
		fg := "取れない"
		if t.Foreground != nil {
			fg = t.Foreground.Name
		}
		return fmt.Errorf("%w: %s の前に出ているものが %s なので触らない（hc kill %s で片付けてから andon boot）", errOldDispatcher, t.Address, fg, t.Address)
	}
	if err := spawn.DiscardWith(hc, t.Address); err != nil {
		return fmt.Errorf("%w: %v（hc kill %s で片付けてから andon boot）", errOldDispatcher, err, t.Address)
	}
	e.logf("前のバージョンの係の端末 %s（%s）を片付けた", t.Address, oldDispatcher)
	return nil
}

func isOldDispatcher(argv []string) bool {
	return len(argv) == 2 && filepath.Base(argv[0]) == "andon" && argv[1] == oldDispatcher
}

// --keep なので core が起き直すと中身がシェルに替わる。core の起き直しは boot を通らないので、Chief of Staff と担当のフックも同じ確かめを打つ。
func startDispatcher(e Env) error {
	if _, ok := e.has("hc"); !ok {
		return nil
	}
	did, err := dispatcher.Ensure(filepath.Join(e.Package, "bin", "andon"), e.Workspace, dispatcherWait)
	switch {
	case errors.Is(err, dispatcher.ErrBusy):
		did = "dispatcher の端末はほかが確かめている（boot は確かめなかった。Chief of Staff と担当のフックが続ける）"
	case errors.Is(err, dispatcher.ErrUnsure):
		did = fmt.Sprintf("%v。動いているとはみなさない（hc get dispatcher --json の foreground がシェルなら andon boot を打ち直す）", err)
	case err != nil:
		return err
	case did == "":
		did = "dispatcher はすでに動いている（前に andon dispatcher が出ている）"
	}
	e.logf("%s", did)
	return nil
}

var dispatcherWait = 5 * time.Second

var stalePoll = 200 * time.Millisecond
