package courier

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 偽の hc: get courier は $D/term（無ければ not_found）を返す。run・send・keep・kill・rm は打たれたものを $D/log に1行ずつ。
// rm は $D/term を消す（片付けた端末は無くなる）。
// get が n 回目のとき $D/term.<n> があれば、それを $D/term にしてから返す（あとで前に出ているものが取れる・替わる）。
const ensureHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) n=$(( $(cat "$d/gets" 2>/dev/null || echo 0) + 1 )); echo $n > "$d/gets"
       [ ! -e "$d/term.$n" ] || mv "$d/term.$n" "$d/term"
       [ -e "$d/term" ] || { echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2; exit 3; }; cat "$d/term" ;;
  run|send|keep|kill) echo "$*" >>"$d/log"; exit 0 ;;
  rm) echo "$*" >>"$d/log"; rm -f "$d/term"; exit 0 ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

func newEnsure(t *testing.T) (d string, round func() (string, []string, error)) {
	return newEnsureWait(t, 0)
}

// newEnsureWait は round の Ensure に wait を渡す（前に出ているものを読み直す上限）。
func newEnsureWait(t *testing.T, wait time.Duration) (d string, round func() (string, []string, error)) {
	t.Helper()
	d = t.TempDir()
	bin := filepath.Join(d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(ensureHc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", d)
	hostName = func() (string, error) { return "TestPlant", nil }
	t.Cleanup(func() { hostName = os.Hostname })
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	ensurePoll = 10 * time.Millisecond
	t.Cleanup(func() { ensurePoll = 500 * time.Millisecond })
	return d, func() (string, []string, error) {
		os.Remove(filepath.Join(d, "log"))
		os.Remove(filepath.Join(d, "gets"))
		did, err := Ensure("/opt/andon/bin/andon", "/srv/plant", wait)
		b, _ := os.ReadFile(filepath.Join(d, "log"))
		var calls []string
		if s := strings.TrimSpace(string(b)); s != "" {
			calls = strings.Split(s, "\n")
		}
		return did, calls, err
	}
}

func putTerm(t *testing.T, path, fg string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(`{"address":"t1","state":"live","argv":["/bin/sh"],"foreground":`+fg+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	fgCourier = `{"name":"andon","argv":["/opt/andon/bin/andon","courier"]}`
	fgShell   = `{"name":"sh","argv":["/bin/sh"]}`
)

func TestEnsureCourier(t *testing.T) {
	d, round := newEnsure(t)
	term := filepath.Join(d, "term")

	// 端末が無い: 起こす
	did, calls, err := round()
	if err != nil || did == "" || len(calls) != 1 || calls[0] != "run --bg --no-parent --no-notify --keep --name courier --label role=courier --label herdr.space=andon-testplant-plant --label herdr.tab=andon-testplant-courier --cwd /srv/plant -- /opt/andon/bin/andon courier" {
		t.Fatalf("端末が無いとき: %q %q %v", did, calls, err)
	}

	// 中身が andon courier: 何もしない
	putTerm(t, term, fgCourier)
	if did, calls, err := round(); err != nil || did != "" || calls != nil {
		t.Fatalf("動いているとき: %q %q %v", did, calls, err)
	}

	// 端末はあるが中身がシェル（core が起き直して hc keep が替えた）: シェルには打ち込まず、片付けて同じ名前・ラベルで起こし直す
	putTerm(t, term, fgShell)
	did, calls, err = round()
	want := []string{"keep t1 --off", "kill t1", "rm t1", "run --bg --no-parent --no-notify --keep --name courier --label role=courier --label herdr.space=andon-testplant-plant --label herdr.tab=andon-testplant-courier --cwd /srv/plant -- /opt/andon/bin/andon courier"}
	if err != nil || !strings.Contains(did, "起こし直した") || strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("中身がシェルのとき: %q %q %v", did, calls, err)
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "send ") {
			t.Fatalf("シェルに打ち込んだ（打ちかけの文字に足してしまう）: %q", c)
		}
	}

	// 前に何も出ていない（替わる途中）: 触らないが、動いているとも言わない（ErrUnsure）
	putTerm(t, term, "null")
	if did, calls, err := round(); !errors.Is(err, ErrUnsure) || did != "" || calls != nil {
		t.Fatalf("前に何も出ていないとき: %q %q %v", did, calls, err)
	}
	// 別のものが出ている: 触らない
	putTerm(t, term, `{"name":"vim","argv":["vim"]}`)
	if did, calls, err := round(); err == nil || did != "" || calls != nil {
		t.Fatalf("別のものが出ているとき: %q %q %v", did, calls, err)
	}

	// 終わった端末（keep が外れて中身が抜けた）: 起こし直す
	if err := os.WriteFile(term, []byte(`{"state":"finished","foreground":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if did, calls, err := round(); err != nil || did == "" || len(calls) != 1 || !strings.HasPrefix(calls[0], "run ") {
		t.Fatalf("終わった端末: %q %q %v", did, calls, err)
	}
}

// 作り直しの直後: 戻った端末の前に出ているものが最初は取れず、読み直すと /bin/bash。動いているとはみなさず、片付けて起こし直す。
func TestEnsureSettlesThenRestarts(t *testing.T) {
	d, round := newEnsureWait(t, time.Second)
	putTerm(t, filepath.Join(d, "term"), "null")
	putTerm(t, filepath.Join(d, "term.3"), `{"name":"bash","argv":["/bin/bash"]}`)
	did, calls, err := round()
	if err != nil || !strings.Contains(did, "起こし直した") || len(calls) != 4 || calls[0] != "keep t1 --off" || !strings.HasPrefix(calls[3], "run ") {
		t.Fatalf("取れてからシェルだったとき: %q %q %v", did, calls, err)
	}
}

// 待っても前に出ているものが取れない: 何も打たず ErrUnsure（「すでに動いている」にしない）。
func TestEnsureUnsureAfterWait(t *testing.T) {
	d, round := newEnsureWait(t, 50*time.Millisecond)
	putTerm(t, filepath.Join(d, "term"), "null")
	did, calls, err := round()
	if !errors.Is(err, ErrUnsure) || did != "" || calls != nil || !strings.Contains(err.Error(), "t1") {
		t.Fatalf("取れないまま: %q %q %v", did, calls, err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "gets")); strings.TrimSpace(string(b)) == "1" {
		t.Error("読み直していない")
	}
}

// ほかが確かめている間（lock を持っている間）は何もしない（boot と複数の Chief of Staff のフックが重なっても二重に打たない）。
func TestEnsureLocked(t *testing.T) {
	_, round := newEnsure(t)
	unlock, err := ensureLock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if did, calls, err := round(); !errors.Is(err, ErrBusy) || did != "" || calls != nil {
		t.Fatalf("lock の間に打った: %q %q %v", did, calls, err)
	}
}

func TestIsCourier(t *testing.T) {
	for argv, want := range map[string]bool{
		"/opt/andon/bin/andon courier": true,
		"andon courier":                true,
		"andon courier --once":         false,
		"andon courier drain":          false,
		"/bin/sh":                      false,
		"andon gate":                   false,
	} {
		if got := IsCourier(strings.Fields(argv)); got != want {
			t.Errorf("%s: %v", argv, got)
		}
	}
}

// lock の置き場が作れない・開けないとき（ほかが確かめている、ではない）は、何もしなかったことにせずエラーを返す。
func TestEnsureLockError(t *testing.T) {
	d, round := newEnsure(t)
	notDir := filepath.Join(d, "state-is-a-file")
	if err := os.WriteFile(notDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", notDir) // この下に andon/courier を作れない
	did, calls, err := round()
	if err == nil || did != "" || calls != nil {
		t.Fatalf("lock の置き場が作れないのに黙った: %q %q %v", did, calls, err)
	}
}

// 偽の hc: ls はラベルで絞らずに3つ返す（別のプロジェクトの同じ id が先）。get は address ごとのラベルを返す。
const liveHc = `#!/usr/bin/env bash
case $1 in
  ls) echo "$*" >>"$FAKE_HC/log"
      echo '{"terminals":[{"address":"t20","state":"live","turn":3},{"address":"t22","state":"finished"},{"address":"t21","state":"live","turn":0}]}' ;;
  get) case $2 in
         t20) echo '{"state":"live","turn":3,"labels":{"project":"other-site","task":"D-3"}}' ;;
         t21) echo '{"state":"live","turn":0,"labels":{"project":"sample-site","task":"D-3"}}' ;;
         *) echo '{"error":{"kind":"not_found"}}' >&2; exit 3 ;;
       esac ;;
  *) exit 9 ;;
esac
`

// 端末は project と task の両方で照らす（task の id はプロジェクトをまたいで同じことがある）。
func TestLiveTask(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(liveHc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_HC", d)
	hostName = func() (string, error) { return "TestPlant", nil }
	t.Cleanup(func() { hostName = os.Hostname })
	addr, turn, ok := liveTask("sample-site", "D-3")
	if !ok || addr != "t21" || turn != 0 {
		t.Fatalf("別のプロジェクトの端末を引いた・自分の端末を引かない: %q %d %v", addr, turn, ok)
	}
	b, _ := os.ReadFile(filepath.Join(d, "log"))
	if got := strings.TrimSpace(string(b)); got != "ls --json --label project=sample-site --label task=D-3" {
		t.Errorf("hc ls を project と task で絞っていない: %q", got)
	}
	if _, _, ok := liveTask("third-site", "D-3"); ok {
		t.Error("ラベルの合わない端末を引いた")
	}
}

// 置き場: 台帳が1つなら指揮側の Space に、2つ以上（無いときも）なら建屋の名前の Space に出す。
func TestPlace(t *testing.T) {
	hostName = func() (string, error) { return "Sample_Plant.local", nil }
	t.Cleanup(func() { hostName = os.Hostname })
	ws := t.TempDir()
	ledger := func(slug string) {
		dir := filepath.Join(ws, "projects", slug, ".flightdeck")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "config.yaml"), nil, 0o644)
	}
	check := func(label, wantSpace, wantTab string) {
		t.Helper()
		if sp, tb := Place(ws); sp != wantSpace || tb != wantTab {
			t.Errorf("%s: %q %q", label, sp, tb)
		}
	}
	check("台帳なし", "andon-sample-plant-local-plant", "andon-sample-plant-local-courier")
	ledger("alpha")
	check("台帳1つ", "andon-alpha", "andon-alpha-courier")
	ledger("beta")
	check("台帳2つ", "andon-sample-plant-local-plant", "andon-sample-plant-local-courier")
}
