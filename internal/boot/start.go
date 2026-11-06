package boot

import (
	"bufio"
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

	"andon/internal/dispatcher"
	"andon/internal/spawn"
)

// 番号の既定。建屋の .env（PORTLESS_PORT・FILES_PORT）で上書きする。
const (
	DefaultPortlessPort = "1355"
	DefaultFilesPort    = "8907"
)

// Start は起動の手を順に打つ。道具が無ければ飛ばし、失敗は警告にして続ける。
func Start(e Env) {
	oldLive := false // 前の版の係の端末を片付けられなかった（dispatcher を起こさない）
	for _, step := range []struct {
		name string
		run  func(Env) error
	}{
		{"Claude Code の記録", fixClaudeInstall},
		{"git の資格情報", setupGitCredentials},
		{"portless", startPortless},
		{"files サーバ", startFiles},
		{"sshd", startSSHD},
		{"前の版の係の端末", dropStaleRoles},
		{"dispatcher", startDispatcher},
	} {
		if step.name == "dispatcher" && oldLive {
			e.logf("前の版の係の端末が残っているので dispatcher は起こさない（二重に回さない）。人が hc kill <address> で片付けてから andon boot を打ち直す")
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

// dotenv は <ws>/.env の key の値（最後の行）。無ければ def。
func (e Env) dotenv(key, def string) string {
	f, err := os.Open(filepath.Join(e.Workspace, ".env"))
	if err != nil {
		return def
	}
	defer f.Close()
	v := def
	s := bufio.NewScanner(f)
	for s.Scan() {
		if k, val, ok := strings.Cut(s.Text(), "="); ok && strings.TrimSpace(k) == key {
			if val = strings.Trim(strings.TrimSpace(val), `"'`); val != "" {
				v = val
			}
		}
	}
	return v
}

func listening(port string) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 500*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// detach は name を起こして手放す（boot が終わっても残る）。出力は logPath へ。
func detach(logPath, name string, args ...string) error {
	out, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
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

// fixClaudeInstall は .claude.json に「ネイティブ版で入っている」記録が無ければ claude install latest で書き直す
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
	e.logf("Claude Code: ネイティブ版の記録を書き直した")
	return nil
}

// startPortless はアプリを名前で当てるプロキシを起こす（http://<app>.<建屋名>.localhost:<番号>）。
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

// startFiles は納品物 artifacts/ を読み取りで公開する files サーバ（andon boot serve-files）を起こし、
// portless に files で登録する。返すのは実体が artifacts/ の中にあるものだけ（FilesHandler）。
func startFiles(e Env) error {
	portless, ok := e.has("portless")
	if !ok || e.Self == "" {
		return nil
	}
	dir := filepath.Join(e.Workspace, "artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	port := e.dotenv("FILES_PORT", DefaultFilesPort)
	if !listening(port) {
		logPath := filepath.Join(os.TempDir(), "files-server.log")
		if err := detach(logPath, e.Self, "boot", "serve-files", dir, port); err != nil {
			return err
		}
		if !waitListening(port, 3*time.Second) {
			return fmt.Errorf("待ち受けが上がらない（%s）", logPath)
		}
	}
	// 登録は volume に残るが、冪等なので毎回打つ。失敗すれば納品物の URL が届かないので知らせる
	if out, err := exec.Command(portless, "alias", "files", port).CombinedOutput(); err != nil {
		return fmt.Errorf("portless alias files %s: %v: %s", port, err, strings.TrimSpace(string(out)))
	}
	e.logf("files: http://files.%s.localhost:%s/ （artifacts/ を読み取り公開）", e.Hostname, e.dotenv("PORTLESS_PORT", DefaultPortlessPort))
	return nil
}

// startSSHD は外の端末から入る口を開ける。建屋の .ssh/authorized_keys が無いか読めなければ無効のまま。
// 設定は image に焼いた sshd_config。ホスト鍵は volume の ~/.ssh に置き、`-h` で sshd に渡す（渡さないと image の /etc/ssh の鍵が使われ、image を焼き直すたびに変わる）。
func startSSHD(e Env) error {
	keys := filepath.Join(e.Workspace, ".ssh", "authorized_keys")
	b, err := os.ReadFile(keys)
	if errors.Is(err, fs.ErrNotExist) {
		e.logf("ssh は無効（使うなら .ssh/authorized_keys に公開鍵を置いて再起動）")
		return nil
	} else if err != nil {
		e.logf("ssh は無効（.ssh/authorized_keys があるが読めない: %v。建屋の repo の持ち主と andon の uid が同じか確かめる）", err)
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

// oldDispatcher は名前を改める前の常駐の係の名前（端末の名前と role= の値。2026-11-06 に dispatcher へ改めた）。
const oldDispatcher = "courier"

// errOldDispatcher は前の版の係の端末が live のまま残った（片付けなかった・片付けに失敗した）。
// Start はその起動で dispatcher を起こさない（前の係が動いていれば二重に回る）。
var errOldDispatcher = errors.New("前の版の係の端末が残っている")

// dropStaleRoles は、名前を改める前の常駐の係の端末（名前もラベル role= も oldDispatcher の live な端末）を片付ける
// （keep を外して kill・rm）。建屋の upgrade のあとに残り、前の版の係が動いたままなら今の dispatcher と二重に回る
// （同じ PR をマージしにいく）ので、起こす前に片付ける。前に出ているものが前の係（andon courier）か引数の無いシェル
// （作り直しで戻った中身）のときだけ片付ける。それ以外（人が中で何か動かしている・前に出ているものが取れない）と
// 片付けに失敗したときは errOldDispatcher を返す（Start はその起動で dispatcher を起こさない）。
func dropStaleRoles(e Env) error {
	hc, ok := e.has("hc")
	if !ok {
		return nil
	}
	got, err := spawn.Output(nil, "", hc, "get", oldDispatcher, "--json")
	if err != nil {
		return nil // 前の係の端末は無い（not_found）。hc 自体の不調は dispatcher の段が出す
	}
	var t spawn.Term
	if json.Unmarshal([]byte(got), &t) != nil || t.State != "live" || t.Address == "" ||
		t.Name != oldDispatcher || t.Labels["role"] != oldDispatcher {
		return nil
	}
	if t.Foreground == nil || !(t.Shell() || isOldDispatcher(t.Foreground.Argv)) {
		fg := "取れない"
		if t.Foreground != nil {
			fg = t.Foreground.Name
		}
		return fmt.Errorf("%w: %s の前に出ているものが %s なので触らない（hc kill %s で片付けてから andon boot）", errOldDispatcher, t.Address, fg, t.Address)
	}
	for _, args := range [][]string{{"keep", t.Address, "--off"}, {"kill", t.Address}, {"rm", t.Address}} {
		if o, err := exec.Command(hc, args...).CombinedOutput(); err != nil {
			return fmt.Errorf("%w: hc %s: %v: %s（hc kill %s で片付けてから andon boot）", errOldDispatcher, strings.Join(args, " "), err, strings.TrimSpace(string(o)), t.Address)
		}
	}
	e.logf("前の版の係の端末 %s（%s）を片付けた", t.Address, oldDispatcher)
	return nil
}

// isOldDispatcher は前に出ているものが前の版の係（andon courier）か。
func isOldDispatcher(argv []string) bool {
	return len(argv) == 2 && filepath.Base(argv[0]) == "andon" && argv[1] == oldDispatcher
}

// startDispatcher は dispatcher の端末を確かめる（dispatcher.Ensure。無ければ起こし、中身がシェルに替わっていれば片付けて起こし直す）。
// --keep なので core が起き直すと中身がシェルに替わる。core の起き直しは boot を通らないので、Chief of Staff のフックも同じ確かめを打つ。
// 作り直しの直後は、戻った端末の前に出ているものがまだ取れないことがあるので、dispatcherWait まで読み直す。
// 何もしなかったときは理由ごとに書き分ける（動いている・確かめられなかった・ほかが確かめている）。
func startDispatcher(e Env) error {
	if _, ok := e.has("hc"); !ok {
		return nil
	}
	did, err := dispatcher.Ensure(filepath.Join(e.Package, "bin", "andon"), e.Workspace, dispatcherWait)
	switch {
	case errors.Is(err, dispatcher.ErrBusy):
		did = "dispatcher の端末はほかが確かめている（boot は確かめなかった。Chief of Staff のフックが続ける）"
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

// dispatcherWait は、dispatcher の端末の前に出ているものが取れるまで boot が待つ上限。テストで差し替える。
var dispatcherWait = 5 * time.Second
