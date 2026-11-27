package dispatcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/flock"
)

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
	putTermAt(t, path, fg, "andon-testplant-plant", "andon-testplant-dispatcher")
}

func putTermAt(t *testing.T, path, fg, space, tab string) {
	t.Helper()
	labels := `{"role":"dispatcher","herdr.space":"` + space + `","herdr.tab":"` + tab + `"}`
	if err := os.WriteFile(path, []byte(`{"address":"t1","state":"live","argv":["/bin/sh"],"labels":`+labels+`,"foreground":`+fg+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	fgDispatcher = `{"name":"andon","argv":["/opt/andon/bin/andon","dispatcher","run"]}`
	// 名前を変える前のバージョンが起動した dispatcher（ADR-0053 受け入れ条件 A13）
	fgOldDispatcher = `{"name":"andon","argv":["/opt/andon/bin/andon","dispatcher"]}`
	fgShell         = `{"name":"sh","argv":["/bin/sh"]}`
)

func TestEnsureDispatcher(t *testing.T) {
	d, round := newEnsure(t)
	term := filepath.Join(d, "term")

	did, calls, err := round()
	if err != nil || did == "" || len(calls) != 1 || calls[0] != "run --bg --no-parent --no-notify --keep --name dispatcher --label role=dispatcher --label herdr.space=andon-testplant-plant --label herdr.tab=andon-testplant-dispatcher --cwd /srv/plant -- /opt/andon/bin/andon dispatcher run" {
		t.Fatalf("端末が無いとき: %q %q %v", did, calls, err)
	}

	for _, fg := range []string{fgDispatcher, fgOldDispatcher} {
		putTerm(t, term, fg)
		if did, calls, err := round(); err != nil || did != "" || calls != nil {
			t.Fatalf("動いているとき（%s）: %q %q %v", fg, did, calls, err)
		}
	}
	// 常駐でない形と、別の葉は dispatcher と読まず、シェルでもないので触らない
	for _, fg := range []string{`{"name":"andon","argv":["/opt/andon/bin/andon","dispatcher","run","--once"]}`, `{"name":"andon","argv":["/opt/andon/bin/andon","dispatcher","drain"]}`} {
		putTerm(t, term, fg)
		if did, calls, err := round(); err == nil || did != "" || calls != nil {
			t.Fatalf("dispatcher でないものが出ているとき（%s）: %q %q %v", fg, did, calls, err)
		}
	}

	putTerm(t, term, fgShell)
	did, calls, err = round()
	want := []string{"keep t1 --off", "kill t1", "rm t1", "run --bg --no-parent --no-notify --keep --name dispatcher --label role=dispatcher --label herdr.space=andon-testplant-plant --label herdr.tab=andon-testplant-dispatcher --cwd /srv/plant -- /opt/andon/bin/andon dispatcher run"}
	if err != nil || !strings.Contains(did, "起動し直した") || strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("中身がシェルのとき: %q %q %v", did, calls, err)
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "send ") {
			t.Fatalf("シェルに打ち込んだ（打ちかけの文字に足してしまう）: %q", c)
		}
	}

	putTerm(t, term, "null")
	if did, calls, err := round(); !errors.Is(err, ErrUnsure) || did != "" || calls != nil {
		t.Fatalf("前に何も出ていないとき: %q %q %v", did, calls, err)
	}
	putTerm(t, term, `{"name":"vim","argv":["vim"]}`)
	if did, calls, err := round(); err == nil || did != "" || calls != nil {
		t.Fatalf("別のものが出ているとき: %q %q %v", did, calls, err)
	}

	if err := os.WriteFile(term, []byte(`{"state":"finished","foreground":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if did, calls, err := round(); err != nil || did == "" || len(calls) != 1 || !strings.HasPrefix(calls[0], "run ") {
		t.Fatalf("終わった端末: %q %q %v", did, calls, err)
	}
}

func TestEnsureSettlesThenRestarts(t *testing.T) {
	d, round := newEnsureWait(t, time.Second)
	putTerm(t, filepath.Join(d, "term"), "null")
	putTerm(t, filepath.Join(d, "term.3"), `{"name":"bash","argv":["/bin/bash"]}`)
	did, calls, err := round()
	if err != nil || !strings.Contains(did, "起動し直した") || len(calls) != 4 || calls[0] != "keep t1 --off" || !strings.HasPrefix(calls[3], "run ") {
		t.Fatalf("取れてからシェルだったとき: %q %q %v", did, calls, err)
	}
}

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

func TestIsDispatcher(t *testing.T) {
	for argv, want := range map[string]bool{
		"/opt/andon/bin/andon dispatcher run": true,
		"andon dispatcher run":                true,
		"/opt/andon/bin/andon dispatcher":     true, // 名前を変える前の形（ADR-0053 決めたこと 9）
		"andon dispatcher":                    true,
		"andon dispatcher run --once":         false,
		"andon dispatcher --once":             false,
		"andon dispatcher run x":              false,
		"andon dispatcher drain":              false,
		"andon dispatcher runx":               false,
		"andon run":                           false,
		"/bin/andonx dispatcher run":          false,
		"/bin/sh":                             false,
		"andon gate":                          false,
	} {
		if got := IsDispatcher(strings.Fields(argv)); got != want {
			t.Errorf("%s: %v", argv, got)
		}
	}
}

func TestEnsureLockError(t *testing.T) {
	d, round := newEnsure(t)
	notDir := filepath.Join(d, "state-is-a-file")
	if err := os.WriteFile(notDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", notDir)
	did, calls, err := round()
	if err == nil || did != "" || calls != nil {
		t.Fatalf("lock の置き場が作れないのに黙った: %q %q %v", did, calls, err)
	}
}

const liveHc = `#!/usr/bin/env bash
case $1 in
  ls) echo "$*" >>"$FAKE_HC/log"
      echo '{"terminals":[{"address":"t20","state":"live","turn":3},{"address":"t22","state":"finished"},{"address":"t21","state":"live","turn":0}]}' ;;
  get) case $2 in
         t20) echo '{"state":"live","turn":3,"labels":{"project":"other-site","issue":"D-3"}}' ;;
         t21) echo '{"state":"live","turn":0,"labels":{"project":"sample-site","issue":"D-3"}}' ;;
         *) echo '{"error":{"kind":"not_found"}}' >&2; exit 3 ;;
       esac ;;
  *) exit 9 ;;
esac
`

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
	addr, turn, ok := liveIssue("sample-site", "D-3")
	if !ok || addr != "t21" || turn != 0 {
		t.Fatalf("別のプロジェクトの端末を引いた・自分の端末を引かない: %q %d %v", addr, turn, ok)
	}
	b, _ := os.ReadFile(filepath.Join(d, "log"))
	if got := strings.TrimSpace(string(b)); got != "ls --json --label project=sample-site --label issue=D-3" {
		t.Errorf("hc ls を project と task で絞っていない: %q", got)
	}
	if _, _, ok := liveIssue("third-site", "D-3"); ok {
		t.Error("ラベルの合わない端末を引いた")
	}
}

func TestEnsureMovesToPlace(t *testing.T) {
	d, _ := newEnsure(t)
	ws := t.TempDir()
	putTermAt(t, filepath.Join(d, "term"), fgDispatcher, "andon-testplant-plant", "andon-testplant-dispatcher")
	cfg := filepath.Join(ws, "projects", "sample-site", "kanban", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	round := func() (string, string, error) {
		os.Remove(filepath.Join(d, "log"))
		did, err := Ensure("/opt/andon/bin/andon", ws, 0)
		b, _ := os.ReadFile(filepath.Join(d, "log"))
		return did, strings.TrimSpace(string(b)), err
	}

	if did, calls, err := round(); !errors.Is(err, ErrOldDispatcher) || did != "" || calls != "" {
		t.Fatalf("前のバージョンの dispatcher を止めた・黙った: %q %q %v", did, calls, err)
	}

	unpass, err := flock.TryLock(PassLock())
	if err != nil {
		t.Fatal(err)
	}
	if did, calls, err := round(); err != nil || did != "" || calls != "" {
		t.Fatalf("見回りの途中で止めた: %q %q %v", did, calls, err)
	}
	unpass()

	did, calls, err := round()
	want := "keep t1 --off\nkill t1\nrm t1\nrun --bg --no-parent --no-notify --keep --name dispatcher --label role=dispatcher --label herdr.space=andon-sample-site --label herdr.tab=andon-sample-site-dispatcher --cwd " + ws + " -- /opt/andon/bin/andon dispatcher run"
	if err != nil || !strings.Contains(did, "andon-sample-site") || calls != want {
		t.Fatalf("置き場が違うとき: %q\n%s\n%v", did, calls, err)
	}
	if u, err := flock.TryLock(PassLock()); err != nil {
		t.Fatalf("起動し直したあとも PassLock を持っている: %v", err)
	} else {
		u()
	}

	putTermAt(t, filepath.Join(d, "term"), fgDispatcher, "andon-sample-site", "andon-sample-site-dispatcher")
	if did, calls, err := round(); err != nil || did != "" || calls != "" {
		t.Fatalf("合っているのに打った: %q %q %v", did, calls, err)
	}
}

func TestPlace(t *testing.T) {
	hostName = func() (string, error) { return "Sample_Plant.local", nil }
	t.Cleanup(func() { hostName = os.Hostname })
	ws := t.TempDir()
	ledger := func(slug string) {
		dir := filepath.Join(ws, "projects", slug, "kanban")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "config.yaml"), nil, 0o644)
	}
	check := func(label, wantSpace, wantTab string) {
		t.Helper()
		if sp, tb := Place(ws); sp != wantSpace || tb != wantTab {
			t.Errorf("%s: %q %q", label, sp, tb)
		}
	}
	check("kanban なし", "andon-sample-plant-local-plant", "andon-sample-plant-local-dispatcher")
	ledger("alpha")
	check("kanban 1つ", "andon-alpha", "andon-alpha-dispatcher")
	ledger("beta")
	check("kanban 2つ", "andon-sample-plant-local-plant", "andon-sample-plant-local-dispatcher")
}
