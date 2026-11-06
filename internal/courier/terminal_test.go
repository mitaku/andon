package courier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 偽の hc: get courier は $D/term（無ければ not_found）を返す。run・send は打たれたものを $D/log に1行ずつ。
// send は $D/after があれば、それを $D/term に替える（打ち込んだシェルが andon courier に替わる）。
const ensureHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) [ -e "$d/term" ] || { echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2; exit 3; }; cat "$d/term" ;;
  run|send) echo "$*" >>"$d/log"; [ "$1" = send ] && [ -e "$d/after" ] && cp "$d/after" "$d/term"; exit 0 ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

func newEnsure(t *testing.T) (d string, round func() (string, []string, error)) {
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
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	old := ensureWait
	ensureWait = 300 * time.Millisecond
	t.Cleanup(func() { ensureWait = old })
	return d, func() (string, []string, error) {
		os.Remove(filepath.Join(d, "log"))
		did, err := Ensure("/opt/andon/bin/andon", "/srv/plant")
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
	if err := os.WriteFile(path, []byte(`{"state":"live","argv":["/bin/sh"],"foreground":`+fg+`}`), 0o644); err != nil {
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
	if err != nil || did == "" || len(calls) != 1 || calls[0] != "run --bg --no-parent --no-notify --keep --name courier --label role=courier --cwd /srv/plant -- /opt/andon/bin/andon courier" {
		t.Fatalf("端末が無いとき: %q %q %v", did, calls, err)
	}

	// 中身が andon courier: 何もしない
	putTerm(t, term, fgCourier)
	if did, calls, err := round(); err != nil || did != "" || calls != nil {
		t.Fatalf("動いているとき: %q %q %v", did, calls, err)
	}

	// 端末はあるが中身がシェル（core が起き直して hc keep が替えた）: andon courier を打ち直す
	putTerm(t, term, fgShell)
	putTerm(t, filepath.Join(d, "after"), fgCourier)
	did, calls, err = round()
	if err != nil || !strings.Contains(did, "打ち直した") || len(calls) != 1 || calls[0] != "send courier '/opt/andon/bin/andon' courier" {
		t.Fatalf("中身がシェルのとき: %q %q %v", did, calls, err)
	}
	if did, calls, _ := round(); did != "" || calls != nil {
		t.Fatalf("打ち直したあとにまた打った: %q %q", did, calls)
	}

	// 前に何も出ていない（替わる途中）・別のものが出ている: 触らない
	putTerm(t, term, "null")
	if did, calls, err := round(); err != nil || did != "" || calls != nil {
		t.Fatalf("前に何も出ていないとき: %q %q %v", did, calls, err)
	}
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

// ほかが確かめている間（lock を持っている間）は何もしない（boot と複数の Chief of Staff のフックが重なっても二重に打たない）。
func TestEnsureLocked(t *testing.T) {
	_, round := newEnsure(t)
	unlock, err := ensureLock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if did, calls, err := round(); err != nil || did != "" || calls != nil {
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
