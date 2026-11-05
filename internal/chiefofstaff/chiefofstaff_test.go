package chiefofstaff

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 偽の kanban と hc を PATH の先頭に置く。打たれた引数は $LOG に1行ずつ。
// 振る舞いは環境変数で変える: FAKE_LS（hc ls）・FAKE_GET（hc get）・FAKE_TPL（hc templates）・FAKE_RUN（hc run）。
const fakeKanban = `#!/usr/bin/env bash
printf 'kanban %s | FLIGHTDECK_ROOT=%s\n' "$*" "${FLIGHTDECK_ROOT:-}" >>"$LOG"
case $1 in
  root) [ -n "${FAKE_ROOT:-}" ] && echo "$FAKE_ROOT" || { echo "no ledger" >&2; exit 1; } ;;
  config) sed -i "s/^coordinator:.*/coordinator: ${3#coordinator=}/" "$FLIGHTDECK_ROOT/.flightdeck/config.yaml" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const fakeHc = `#!/usr/bin/env bash
printf 'hc %s\n' "$*" >>"$LOG"
case $1 in
  ls) echo "$FAKE_LS" ;;
  get) echo "$FAKE_GET" ;;
  templates) echo "$FAKE_TPL" ;;
  run) echo "$FAKE_RUN" ;;
  attach) exit 0 ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

type fake struct {
	t         *testing.T
	root, log string
}

func newFake(t *testing.T) *fake {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"kanban": fakeKanban, "hc": fakeHc} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := &fake{t: t, root: filepath.Join(dir, "projects", "sample-site"), log: filepath.Join(dir, "log")}
	if err := os.MkdirAll(filepath.Join(f.root, ".flightdeck"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.setCoordinator("t1")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOG", f.log)
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("FLIGHTDECK_ROOT", "")
	t.Setenv("FAKE_LS", `{"terminals":[]}`)
	t.Setenv("FAKE_GET", `{}`)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"chief-of-staff"}]}`)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":true}}`)
	return f
}

func (f *fake) setCoordinator(addr string) {
	f.t.Helper()
	body := "version: 2\nprefix: S\ncolumns:\n  - todo\n  - done\nphases: []\nrepos: []\ncoordinator: " + addr + "\n"
	if err := os.WriteFile(filepath.Join(f.root, ".flightdeck", "config.yaml"), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fake) coordinator() string {
	b, _ := os.ReadFile(filepath.Join(f.root, ".flightdeck", "config.yaml"))
	for _, ln := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(ln, "coordinator: "); ok {
			return v
		}
	}
	return ""
}

func (f *fake) calls() []string {
	b, _ := os.ReadFile(f.log)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func (f *fake) has(prefix string) bool {
	for _, c := range f.calls() {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func runMain(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := Main(args, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

func TestStartsChiefOfStaff(t *testing.T) {
	f := newFake(t)
	rc, out, errs := runMain("--no-attach")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if !strings.Contains(out, "sample-site の Chief of Staff は t42（sample-site-cos・起こした）") {
		t.Errorf("out=%q", out)
	}
	want := []string{
		"kanban root",
		"hc ls --json --label role=chief-of-staff --label project=sample-site",
		"hc templates --json",
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-cos --label role=chief-of-staff --label project=sample-site --cwd " + f.root + " chief-of-staff プロジェクト `sample-site` の Chief of Staff。",
		"台帳は `" + f.root + "/.flightdeck`。",
		"kanban config set coordinator=t42 | FLIGHTDECK_ROOT=" + f.root,
	}
	calls := f.calls()
	if len(calls) != len(want) {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
	for i := range want {
		if !strings.HasPrefix(calls[i], want[i]) {
			t.Errorf("call %d:\n got %q\nwant %q", i, calls[i], want[i])
		}
	}
	// skill はプロファイルの command が system prompt に足す（ADR-0001 決定 3）
	if all := strings.Join(calls, "\n"); strings.Contains(all, "SKILL.md") || strings.Contains(all, "先に") {
		t.Errorf("最初のメッセージに「先に読む」の行が残る:\n%s", all)
	}
	if f.coordinator() != "t42" {
		t.Errorf("coordinator=%q", f.coordinator())
	}
}

func TestUsesRunningOne(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`"}`)
	rc, out, errs := runMain("--no-attach")
	if rc != 0 || errs != "" || !strings.Contains(out, "t1（sample-site-cos・動いているものを使う）") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	// 二重に起こさない。coordinator が合っていれば書かない
	if f.has("hc run") || f.has("hc templates") || f.has("kanban config") {
		t.Errorf("calls=%q", f.calls())
	}
}

func TestFixesCoordinator(t *testing.T) {
	f := newFake(t)
	f.setCoordinator("t3") // 起き直す前の address
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t8"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t8","name":"sample-site-cos","cwd":"`+f.root+`"}`)
	if rc, _, errs := runMain("--no-attach"); rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if f.has("hc run") || f.coordinator() != "t8" {
		t.Errorf("coordinator=%q calls=%q", f.coordinator(), f.calls())
	}
}

func TestTwoRunning(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"},{"address":"t2"}]}`)
	rc, _, errs := runMain("--no-attach")
	if rc != exitFail || !strings.Contains(errs, "Chief of Staff が 2 人いる: t1 t2") || f.has("hc run") || f.has("kanban config") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestRunningOneDoesNotMatch(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t9","name":"sample-site-cos","cwd":"/elsewhere"}`)
	rc, _, errs := runMain("--no-attach")
	if rc != exitFail || !strings.Contains(errs, "名前（sample-site-cos）か cwd（/elsewhere）が合わない") || f.has("kanban config") || f.coordinator() != "t1" {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestNoTemplate(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"}]}`)
	rc, _, errs := runMain("--no-attach")
	if rc != exitFail || !strings.Contains(errs, "プロファイル chief-of-staff の template が無い") || f.has("hc run") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestNoLedger(t *testing.T) {
	newFake(t)
	t.Setenv("FAKE_ROOT", "")
	if rc, _, errs := runMain("--no-attach"); rc != exitFail || !strings.Contains(errs, "台帳が見つからない") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestFirstMessageNotSent(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	rc, _, errs := runMain("--no-attach")
	if rc != exitFirst || !strings.Contains(errs, "t42（sample-site-cos）に最初のメッセージが届いていない: trust prompt") ||
		!strings.Contains(errs, "hc attach t42 で画面に答えてから、次を送る: hc send t42 \"プロジェクト `sample-site` の Chief of Staff。") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
	// 起きた端末は台帳に載せる（起き直さずに画面に答えればよい）
	if f.coordinator() != "t42" {
		t.Errorf("coordinator=%q", f.coordinator())
	}
}

func TestAttach(t *testing.T) {
	f := newFake(t)
	old := isTTY
	isTTY = func(io.Reader, io.Writer) bool { return true }
	t.Cleanup(func() { isTTY = old })
	if rc, _, errs := runMain(); rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if c := f.calls(); c[len(c)-1] != "hc attach t42" {
		t.Errorf("calls=%q", c)
	}
	// --no-attach なら端末からでも乗らない
	os.Remove(f.log)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t42"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t42","name":"sample-site-cos","cwd":"`+f.root+`"}`)
	if rc, _, _ := runMain("--no-attach"); rc != 0 || f.has("hc attach") {
		t.Errorf("rc=%d calls=%q", rc, f.calls())
	}
}

func TestFirstMessageNotSentAttaches(t *testing.T) {
	f := newFake(t)
	old := isTTY
	isTTY = func(io.Reader, io.Writer) bool { return true }
	t.Cleanup(func() { isTTY = old })
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	rc, _, errs := runMain()
	if rc != 0 || !strings.Contains(errs, "画面で答えてから、次を送る: hc send t42") || !f.has("hc attach t42") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{{"x"}, {"--no-attach", "x"}, {"--attach"}} {
		if rc, _, errs := runMain(args...); rc != exitUsage || !strings.HasPrefix(errs, "usage: andon chief-of-staff") {
			t.Errorf("%v: rc=%d err=%q", args, rc, errs)
		}
	}
}
