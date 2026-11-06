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

// 偽の hc（端末の一覧）: ls は $D/t*.json の address を並べ、get <addr> はその中身を返す。keep・kill・rm は $D/log へ。
const staleHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  ls) printf '{"terminals":['; sep=; for f in "$d"/t*.json; do printf '%s{"address":"%s"}' "$sep" "$(basename "$f" .json)"; sep=,; done; echo ']}' ;;
  get) cat "$d/$2.json" ;;
  keep|kill|rm) echo "$*" >>"$d/log" ;;
  *) exit 9 ;;
esac
`

// 役のラベルの値が今の役のどれでもない端末（前の版の常駐の係）だけを片付ける。今の役・役の無い端末・終わった端末には触らない。
func TestDropStaleRoles(t *testing.T) {
	f, e, d := dispatcherFixture(t)
	put(t, filepath.Join(d, "bin", "hc"), staleHc)
	for addr, j := range map[string]string{
		"t1": `{"address":"t1","name":"old-relay","state":"live","labels":{"role":"old-relay"}}`,
		"t2": `{"address":"t2","name":"dispatcher","state":"live","labels":{"role":"dispatcher"}}`,
		"t3": `{"address":"t3","name":"cos","state":"live","labels":{"role":"chief-of-staff","project":"x"}}`,
		"t4": `{"address":"t4","name":"x-y-1","state":"live","labels":{"project":"x","task":"Y-1"}}`,
		"t5": `{"address":"t5","name":"ce","state":"live","labels":{"role":"chief-engineer"}}`,
		"t6": `{"address":"t6","name":"gone","state":"finished","labels":{"role":"old-relay"}}`,
	} {
		put(t, filepath.Join(d, addr+".json"), j)
	}
	if err := dropStaleRoles(e); err != nil {
		t.Fatal(err)
	}
	log, _ := os.ReadFile(filepath.Join(d, "log"))
	if want := "keep t1 --off\nkill t1\nrm t1\n"; string(log) != want {
		t.Errorf("hc=%q want %q", log, want)
	}
	if got := f.log.String(); !strings.Contains(got, "前の版の役の端末 t1（old-relay・role=old-relay）を片付けた") {
		t.Errorf("log=%q", got)
	}
}
