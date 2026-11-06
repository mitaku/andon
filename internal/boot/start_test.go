package boot

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"andon/internal/dispatcher"
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

// fakeGH は呼ばれた引数を calls に1行ずつ足す gh。status が 0 以外を返すと未ログイン。
func fakeGH(t *testing.T, f *fixture, statusRC int) (e Env, calls string) {
	t.Helper()
	calls = filepath.Join(f.root, "gh-calls")
	gh := filepath.Join(f.root, "gh")
	put(t, gh, "#!/bin/sh\necho \"$*\" >> "+calls+"\n[ \"$*\" = \"auth status\" ] && exit "+strconv.Itoa(statusRC)+"\nexit 0\n")
	if err := os.Chmod(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	e = f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "gh" {
			return gh, nil
		}
		return "", errors.New("無い")
	}
	return e, calls
}

func TestSetupGitCredentialsLoggedIn(t *testing.T) {
	f := newFixture(t)
	e, calls := fakeGH(t, f, 0)
	for range 2 { // 毎起動打っても同じ
		if err := setupGitCredentials(e); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(calls)
	if want := "auth status\nauth setup-git\nauth status\nauth setup-git\n"; string(b) != want {
		t.Errorf("gh の呼び出し: %q", b)
	}
}

func TestSetupGitCredentialsNotLoggedIn(t *testing.T) {
	f := newFixture(t)
	e, calls := fakeGH(t, f, 1)
	if err := setupGitCredentials(e); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(calls)
	if string(b) != "auth status\n" {
		t.Errorf("未ログインで setup-git を打った: %q", b)
	}
	if !strings.Contains(f.log.String(), "gh はログインしていない") {
		t.Errorf("ログ: %q", f.log.String())
	}
}

func TestSetupGitCredentialsNoGH(t *testing.T) {
	f := newFixture(t)
	if err := setupGitCredentials(f.env()); err != nil || f.log.Len() != 0 {
		t.Errorf("gh が無いときは何もしない: %v %q", err, f.log.String())
	}
}

// dispatcher の端末を確かめられない（控えの置き場が作れない）ときは、警告の行に出す（「すでに動いている」と言わない）。
func TestStartDispatcherReportsLockError(t *testing.T) {
	f := newFixture(t)
	notDir := filepath.Join(f.root, "state-is-a-file")
	put(t, notDir, "")
	t.Setenv("XDG_STATE_HOME", notDir)
	e := f.env()
	e.LookPath = func(tool string) (string, error) { // hc だけある（ほかの起動は飛ばす）。hc は打つ前に止まる（無いパスなので本物の hc を打たない）
		if tool == "hc" {
			return filepath.Join(f.root, "no-hc"), nil
		}
		return "", os.ErrNotExist
	}
	Start(e)
	got := f.log.String()
	if !strings.Contains(got, "dispatcher の準備に失敗") || strings.Contains(got, "すでに動いている") {
		t.Errorf("起こせなかったことが見えない: %q", got)
	}
}

// 偽の hc（dispatcher の端末）: get が n 回目のとき $D/term.<n> があれば $D/term にしてから返す。打ったものは $D/log へ。
const dispatcherHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) n=$(( $(cat "$d/gets" 2>/dev/null || echo 0) + 1 )); echo $n > "$d/gets"
       [ ! -e "$d/term.$n" ] || mv "$d/term.$n" "$d/term"
       [ -e "$d/term" ] || { echo '{"error":{"kind":"not_found"}}' >&2; exit 3; }; cat "$d/term" ;;
  keep|kill|run) echo "$*" >>"$d/log" ;;
  rm) echo "$*" >>"$d/log"; rm -f "$d/term" ;;
  *) exit 9 ;;
esac
`

func dispatcherFixture(t *testing.T) (*fixture, Env, string) {
	t.Helper()
	f := newFixture(t)
	d := filepath.Join(f.root, "hc")
	mustMkdir(t, filepath.Join(d, "bin"))
	hc := filepath.Join(d, "bin", "hc")
	put(t, hc, dispatcherHc)
	if err := os.Chmod(hc, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(d, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.root, "state"))
	e := f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "hc" {
			return hc, nil
		}
		return "", errors.New("無い")
	}
	return f, e, d
}

func term(fg string) string {
	return `{"address":"t1","state":"live","argv":["/bin/bash"],"foreground":` + fg + `}`
}

// 作り直しの直後: 戻った端末の前に出ているものが最初は取れず、読み直すと /bin/bash。「すでに動いている」と書かずに起こし直す。
func TestStartDispatcherRestoredShell(t *testing.T) {
	f, e, d := dispatcherFixture(t)
	old := dispatcherWait
	dispatcherWait = 2 * time.Second
	t.Cleanup(func() { dispatcherWait = old })
	put(t, filepath.Join(d, "term"), term("null"))
	put(t, filepath.Join(d, "term.2"), term(`{"name":"bash","argv":["/bin/bash"]}`))
	if err := startDispatcher(e); err != nil {
		t.Fatal(err)
	}
	got := f.log.String()
	log, _ := os.ReadFile(filepath.Join(d, "log"))
	if strings.Contains(got, "すでに動いている") || !strings.Contains(got, "起こし直した") || !strings.Contains(string(log), "run ") {
		t.Errorf("log=%q hc=%q", got, log)
	}
}

// 待っても前に出ているものが取れない: 「確かめられなかった」と書き、「すでに動いている」とは書かない。何も打たない。
func TestStartDispatcherUnsure(t *testing.T) {
	f, e, d := dispatcherFixture(t)
	old := dispatcherWait
	dispatcherWait = 100 * time.Millisecond
	t.Cleanup(func() { dispatcherWait = old })
	put(t, filepath.Join(d, "term"), term("null"))
	if err := startDispatcher(e); err != nil {
		t.Fatal(err)
	}
	got := f.log.String()
	if strings.Contains(got, "すでに動いている") || !strings.Contains(got, "確かめられなかった") || strings.Contains(got, "準備に失敗") {
		t.Errorf("log=%q", got)
	}
	if _, err := os.Stat(filepath.Join(d, "log")); err == nil {
		t.Error("確かめられないのに hc を打った")
	}
}

// 前に andon dispatcher が出ていれば「すでに動いている」。
func TestStartDispatcherRunning(t *testing.T) {
	f, e, d := dispatcherFixture(t)
	put(t, filepath.Join(d, "term"), term(`{"name":"andon","argv":["/opt/andon/bin/andon","dispatcher"]}`))
	if err := startDispatcher(e); err != nil {
		t.Fatal(err)
	}
	if got := f.log.String(); !strings.Contains(got, "dispatcher はすでに動いている") {
		t.Errorf("log=%q", got)
	}
}

// ほかが確かめている（lock を持っている）: 「ほかが確かめている」と書き、「すでに動いている」とは書かない。
func TestStartDispatcherBusy(t *testing.T) {
	f, e, d := dispatcherFixture(t)
	put(t, filepath.Join(d, "term"), term("null"))
	lock := filepath.Join(filepath.Dir(dispatcher.StatePath("x")), ".terminal.lock")
	mustMkdir(t, filepath.Dir(lock))
	fh, err := os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if err := syscall.Flock(int(fh.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if err := startDispatcher(e); err != nil {
		t.Fatal(err)
	}
	if got := f.log.String(); strings.Contains(got, "すでに動いている") || !strings.Contains(got, "ほかが確かめている") {
		t.Errorf("log=%q", got)
	}
}

// 偽の hc: get <name> は $D/<name>.json（無ければ not_found）。keep・kill・rm は $D/log へ。
const staleHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) [ -e "$d/$2.json" ] || { echo '{"error":{"kind":"not_found"}}' >&2; exit 3; }; cat "$d/$2.json" ;;
  keep|kill|rm) [ ! -e "$d/fail-$1" ] || { echo "$1 failed" >&2; exit 1; }; echo "$*" >>"$d/log" ;;
  run) echo "$*" >>"$d/log" ;;
  *) exit 9 ;;
esac
`

// 前の版の係（名前もラベルも courier）だけを、前に出ているものが前の係かシェルのときに片付ける。
// ほかの役・知らない役・人が中で何か動かしている端末・前に出ているものが取れない端末には触らない。
func TestDropStaleRoles(t *testing.T) {
	old := func(fg string) string {
		return `{"address":"t1","name":"courier","state":"live","argv":["/bin/bash"],"labels":{"role":"courier"},"foreground":` + fg + `}`
	}
	for _, c := range []struct {
		name, term string
		dropped    bool
		log        string
	}{
		{"前の係が動いている", old(`{"name":"andon","argv":["/opt/andon/bin/andon","courier"]}`), true, "前の版の係の端末 t1（courier）を片付けた"},
		{"作り直しで戻ったシェル", old(`{"name":"bash","argv":["/bin/bash"]}`), true, "片付けた"},
		{"名前が courier でもラベルが違う", `{"address":"t1","name":"courier","state":"live","labels":{"role":"worker"},"foreground":{"name":"bash","argv":["/bin/bash"]}}`, false, ""},
		{"終わった端末", `{"address":"t1","name":"courier","state":"finished","labels":{"role":"courier"}}`, false, ""},
		{"端末が無い", "", false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, e, d := dispatcherFixture(t)
			put(t, filepath.Join(d, "bin", "hc"), staleHc)
			if c.term != "" {
				put(t, filepath.Join(d, "courier.json"), c.term)
			}
			if err := dropStaleRoles(e); err != nil {
				t.Fatal(err)
			}
			log, _ := os.ReadFile(filepath.Join(d, "log"))
			if want := map[bool]string{true: "keep t1 --off\nkill t1\nrm t1\n", false: ""}[c.dropped]; string(log) != want {
				t.Errorf("hc=%q want %q", log, want)
			}
			if got := f.log.String(); (c.log == "") != (got == "") || !strings.Contains(got, c.log) {
				t.Errorf("log=%q want %q", got, c.log)
			}
		})
	}
}

// 前の版の係の端末を片付けられなければ（片付けに失敗した・触らずに残した）、その起動では dispatcher を起こさない。
// 片付けられれば起こす。
func TestStartSkipsDispatcherWhileOldRemains(t *testing.T) {
	oldWait, oldPoll := dispatcherWait, stalePoll
	dispatcherWait, stalePoll = 100*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { dispatcherWait, stalePoll = oldWait, oldPoll })
	old := func(fg string) string {
		return `{"address":"t1","name":"courier","state":"live","argv":["/bin/bash"],"labels":{"role":"courier"},"foreground":` + fg + `}`
	}
	running := old(`{"name":"andon","argv":["/opt/andon/bin/andon","courier"]}`)
	for _, c := range []struct {
		name, term, fail string
		started          bool
		log              string
	}{
		{"kill に失敗", running, "kill", false, "kill failed"},
		{"keep --off に失敗", running, "keep", false, "keep failed"},
		{"人が中で何か動かしている", old(`{"name":"vim","argv":["vim"]}`), "", false, "前に出ているものが vim なので触らない"},
		{"前に出ているものが取れない", old("null"), "", false, "取れない なので触らない"},
		{"片付けた", running, "", true, "前の版の係の端末 t1（courier）を片付けた"},
		{"前の係の端末が無い", "", "", true, "dispatcher を起こした"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, e, d := dispatcherFixture(t)
			put(t, filepath.Join(d, "bin", "hc"), staleHc)
			if c.term != "" {
				put(t, filepath.Join(d, "courier.json"), c.term)
			}
			if c.fail != "" {
				put(t, filepath.Join(d, "fail-"+c.fail), "")
			}
			Start(e)
			log, _ := os.ReadFile(filepath.Join(d, "log"))
			if got := strings.Contains(string(log), "run "); got != c.started {
				t.Errorf("dispatcher の hc run=%v want %v: %q", got, c.started, log)
			}
			got := f.log.String()
			if !strings.Contains(got, c.log) {
				t.Errorf("log=%q want %q", got, c.log)
			}
			if !c.started && !strings.Contains(got, "dispatcher は起こさない") {
				t.Errorf("起こさない理由が無い: %q", got)
			}
		})
	}
}

// 作り直しの直後は前に出ているものが最初は取れない。取れるまで読み直し、シェルだと分かったら片付ける。
func TestDropStaleRolesRereadsForeground(t *testing.T) {
	oldWait, oldPoll := dispatcherWait, stalePoll
	dispatcherWait, stalePoll = 2*time.Second, 10*time.Millisecond
	t.Cleanup(func() { dispatcherWait, stalePoll = oldWait, oldPoll })
	f, e, d := dispatcherFixture(t)
	// 最初の2回は foreground が null、3回目から /bin/bash を返す偽の hc。
	put(t, filepath.Join(d, "bin", "hc"), `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) n=$(cat "$d/n" 2>/dev/null || echo 0); n=$((n+1)); echo $n >"$d/n"
       if [ $n -le 2 ]; then fg=null; else fg='{"name":"bash","argv":["/bin/bash"]}'; fi
       echo '{"address":"t1","name":"courier","state":"live","argv":["/bin/bash"],"labels":{"role":"courier"},"foreground":'"$fg"'}' ;;
  keep|kill|rm) echo "$*" >>"$d/log" ;;
  *) exit 9 ;;
esac
`)
	if err := dropStaleRoles(e); err != nil {
		t.Fatal(err)
	}
	log, _ := os.ReadFile(filepath.Join(d, "log"))
	if string(log) != "keep t1 --off\nkill t1\nrm t1\n" {
		t.Errorf("hc=%q", log)
	}
	if !strings.Contains(f.log.String(), "片付けた") {
		t.Errorf("log=%q", f.log.String())
	}
}
