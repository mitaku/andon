package chiefofstaff

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/spawn"
)

// 偽の kanban と hc を PATH の先頭に置く。打たれた引数は $LOG に1行ずつ。
// 振る舞いは環境変数で変える: FAKE_LS（hc ls）・FAKE_GET（hc get。FAKE_GET_<address> があればその address はそれ）・FAKE_TPL（hc templates）・FAKE_RUN（hc run）。
const fakeKanban = `#!/usr/bin/env bash
printf 'kanban %s | KANBAN_ROOT=%s\n' "$*" "${KANBAN_ROOT:-}" >>"$LOG"
case $1 in
  root) [ -n "${FAKE_ROOT:-}" ] && echo "$FAKE_ROOT" || { echo "no ledger" >&2; exit 1; } ;;
  config) sed -i "s/^chief_of_staff:.*/chief_of_staff: ${3#chief_of_staff=}/" "$KANBAN_ROOT/kanban/config.yaml" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const fakeHc = `#!/usr/bin/env bash
printf 'hc %s\n' "$*" >>"$LOG"
case $1 in
  ls) echo "$FAKE_LS" ;;
  get) n=$(( $(cat "$LOG.gets" 2>/dev/null || echo 0) + 1 )); echo $n > "$LOG.gets"
       s="FAKE_GET$n"; v="FAKE_GET_$2"  # FAKE_GET<n> があれば n 回目の get はそれ
       if [ -n "${!s:-}" ]; then echo "${!s}"; else echo "${!v:-$FAKE_GET}"; fi ;;
  keep|kill|rm) ;;
  templates) echo "$FAKE_TPL" ;;
  run) echo "$FAKE_RUN" ;;
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
	if err := os.MkdirAll(filepath.Join(f.root, "kanban"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.setCoordinator("t1")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOG", f.log)
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("FAKE_LS", `{"terminals":[]}`)
	t.Setenv("FAKE_GET", `{}`)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"chief-of-staff"}]}`)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":true}}`)
	spawn.SettleWait = 50 * time.Millisecond
	t.Cleanup(func() { spawn.SettleWait = 5 * time.Second })
	return f
}

func (f *fake) setCoordinator(addr string) {
	f.t.Helper()
	body := "version: 3\nprefix: S\ncolumns:\n  - todo\n  - done\nphases: []\nrepos: []\nchief_of_staff: " + addr + "\n"
	if err := os.WriteFile(filepath.Join(f.root, "kanban", "config.yaml"), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fake) chief_of_staff() string {
	b, _ := os.ReadFile(filepath.Join(f.root, "kanban", "config.yaml"))
	for _, ln := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(ln, "chief_of_staff: "); ok {
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

// 端末の様子: エージェント（hc の template は sh -c '… claude …' で起こし、agent は claude-code）と、作り直しで戻った端末（argv ごと素のシェル・agent 無し）
const (
	fgAgent = `"argv":["sh","-c","claude"],"status":{"app":"claude-code"},"foreground":{"name":"sh","argv":["sh","-c","claude"]}`
	fgShell = `"argv":["/bin/bash"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}`
)

func TestStartsChiefOfStaff(t *testing.T) {
	f := newFake(t)
	rc, out, errs := runMain()
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if !strings.Contains(out, "sample-site の Chief of Staff は t42（sample-site-cos・起こした）") {
		t.Errorf("out=%q", out)
	}
	// 最後に出た場所を「<Space 名> / <tab 名>」で出す（ADR-0002 決めたこと 14・17）。address は末尾に添えるだけ
	if !strings.HasSuffix(out, "andon chief-of-staff: 出た場所 sample-site / CoS（t42）\n") {
		t.Errorf("出た場所が最後に無い: out=%q", out)
	}
	want := []string{
		"kanban root",
		"hc ls --json --label role=chief-of-staff --label project=sample-site",
		"hc templates --json",
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-cos --label role=chief-of-staff --label project=sample-site --label herdr.space=andon-sample-site --label herdr.tab=andon-sample-site-cos --cwd " + f.root + " chief-of-staff プロジェクト `sample-site` の Chief of Staff。",
		"台帳は `" + f.root + "/kanban`。",
		"kanban config set chief_of_staff=t42 | KANBAN_ROOT=" + f.root,
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
	if f.chief_of_staff() != "t42" {
		t.Errorf("chief_of_staff=%q", f.chief_of_staff())
	}
}

func TestUsesRunningOne(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+fgAgent+`,"labels":{"herdr.space":"andon-sample-site","herdr.tab":"andon-sample-site-cos"}}`)
	rc, out, errs := runMain()
	if rc != 0 || errs != "" || !strings.Contains(out, "t1（sample-site-cos・動いているものを使う）") || !strings.HasSuffix(out, "出た場所 sample-site / CoS（t1）\n") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	// 二重に起こさない。chief_of_staff が合っていれば書かない
	if f.has("hc run") || f.has("hc templates") || f.has("kanban config") {
		t.Errorf("calls=%q", f.calls())
	}
}

func TestFixesCoordinator(t *testing.T) {
	f := newFake(t)
	f.setCoordinator("t3") // 起き直す前の address
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t8"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t8","name":"sample-site-cos","cwd":"`+f.root+`",`+fgAgent+`}`)
	if rc, _, errs := runMain(); rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if f.has("hc run") || f.chief_of_staff() != "t8" {
		t.Errorf("chief_of_staff=%q calls=%q", f.chief_of_staff(), f.calls())
	}
}

func TestTwoRunning(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"},{"address":"t2"}]}`)
	t.Setenv("FAKE_GET", `{"name":"sample-site-cos","cwd":"`+f.root+`",`+fgAgent+`}`)
	rc, _, errs := runMain()
	if rc != exitFail || !strings.Contains(errs, "Chief of Staff が 2 人いる: t1 t2") || f.has("hc run") || f.has("kanban config") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestRunningOneDoesNotMatch(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t9","name":"sample-site-cos","cwd":"/elsewhere",`+fgAgent+`}`)
	rc, _, errs := runMain()
	if rc != exitFail || !strings.Contains(errs, "名前（sample-site-cos）か cwd（/elsewhere）が合わない") || f.has("kanban config") || f.chief_of_staff() != "t1" {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestNoTemplate(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"}]}`)
	rc, _, errs := runMain()
	if rc != exitFail || !strings.Contains(errs, "プロファイル chief-of-staff の template が無い") || f.has("hc run") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestNoLedger(t *testing.T) {
	newFake(t)
	t.Setenv("FAKE_ROOT", "")
	if rc, _, errs := runMain(); rc != exitFail || !strings.Contains(errs, "台帳が見つからない") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestFirstMessageNotSent(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	rc, _, errs := runMain()
	if rc != exitFirst || !strings.Contains(errs, "t42（sample-site-cos）に最初のメッセージが届いていない: trust prompt") ||
		!strings.Contains(errs, "App の sample-site / CoS で答えてから、次を送る: hc send t42 \"プロジェクト `sample-site` の Chief of Staff。") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
	// 起きた端末は台帳に載せる（起き直さずに画面に答えればよい）
	if f.chief_of_staff() != "t42" {
		t.Errorf("chief_of_staff=%q", f.chief_of_staff())
	}
}

func TestRunningOneWithoutLabels(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+fgAgent+`}`)
	rc, out, _ := runMain()
	// ラベルより前に起きた端末: 付け直さず、App の Space に無いと言う（嘘の場所を出さない）
	if rc != 0 || strings.Contains(out, "sample-site / CoS") || !strings.Contains(out, "出た場所 App の Space の外") || f.has("hc label") {
		t.Errorf("rc=%d out=%q calls=%q", rc, out, f.calls())
	}
}

func TestNeverAttaches(t *testing.T) {
	for _, run := range []string{`{"address":"t42","first_message":{"sent":true}}`, `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`} {
		f := newFake(t)
		t.Setenv("FAKE_RUN", run)
		runMain()
		if f.has("hc attach") {
			t.Errorf("hc attach を打った: %q", f.calls())
		}
	}
}

func TestUsage(t *testing.T) {
	// --no-attach は消した（ADR-0002 決めたこと 14）
	for _, args := range [][]string{{"x"}, {"--no-attach"}, {"--attach"}} {
		if rc, _, errs := runMain(args...); rc != exitUsage || !strings.HasPrefix(errs, "usage: andon chief-of-staff") {
			t.Errorf("%v: rc=%d err=%q", args, rc, errs)
		}
	}
}

// 作り直しで戻った端末（ラベル・名前・cwd はそのまま、中身が素のシェル）: 使わずに片付け、起こし直して chief_of_staff を合わせる。
func TestShellChiefOfStaffIsReplaced(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+fgShell+`}`)
	rc, out, errs := runMain()
	if rc != 0 || errs != "" || !strings.Contains(out, "t1 は Chief of Staff のラベルのまま中身がシェルに替わっていた") || !strings.Contains(out, "t42（sample-site-cos・起こした）") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	calls := strings.Join(f.calls(), "\n")
	if !strings.Contains(calls, "hc keep t1 --off\nhc kill t1\nhc rm t1\n") || !f.has("hc run") || f.has("hc send") || f.chief_of_staff() != "t42" {
		t.Errorf("chief_of_staff=%q calls:\n%s", f.chief_of_staff(), calls)
	}
}

// 中身がシェルの端末は2人と数えない（動いている1人を使い、シェルの端末は片付ける）。
func TestShellChiefOfStaffNotCounted(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"},{"address":"t2"}]}`)
	t.Setenv("FAKE_GET_t1", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+fgShell+`}`)
	t.Setenv("FAKE_GET_t2", `{"address":"t2","name":"sample-site-cos","cwd":"`+f.root+`",`+fgAgent+`}`)
	rc, out, errs := runMain()
	if rc != 0 || errs != "" || !strings.Contains(out, "t2（sample-site-cos・動いているものを使う）") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if !f.has("hc kill t1") || f.has("hc kill t2") || f.has("hc run") || f.chief_of_staff() != "t2" {
		t.Errorf("chief_of_staff=%q calls=%q", f.chief_of_staff(), f.calls())
	}
}

// 待っても前に出ているものが取れない: 使わず・起こさず・片付けず止まる（動いているかを確かめられない）。
func TestChiefOfStaffForegroundUnknown(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
	t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`","argv":["/bin/bash"],"foreground":null}`)
	rc, _, errs := runMain()
	if rc != exitFail || !strings.Contains(errs, "確かめられないので止まる") || f.has("hc run") || f.has("hc kill") || f.has("kanban config") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	n := 0
	for _, c := range f.calls() {
		if strings.HasPrefix(c, "hc get t1") {
			n++
		}
	}
	if n < 2 {
		t.Errorf("読み直していない: hc get %d 回", n)
	}
}

// 人がシェルで使っている端末（top-level はエージェントの command で前にシェル・top-level がシェルで前に別のもの）は片付けない。
// 使いも数えもせず、確かめられないとして止まる（台帳を書かない）。
func TestHumanShellChiefOfStaffKept(t *testing.T) {
	for _, term := range []string{
		`"argv":["sh","-c","claude"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}`,
		`"argv":["/bin/bash"],"status":null,"foreground":{"name":"vim","argv":["vim","notes.md"]}`,
	} {
		f := newFake(t)
		t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
		t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+term+`}`)
		rc, _, errs := runMain()
		if rc != exitFail || !strings.Contains(errs, "Claude Code が動いているか確かめられないので止まる") {
			t.Errorf("%s: rc=%d err=%q", term, rc, errs)
		}
		if f.has("hc keep") || f.has("hc kill") || f.has("hc rm") || f.has("hc run") || f.has("kanban config") || f.chief_of_staff() != "t1" {
			t.Errorf("%s: 人の端末に触った・台帳を書いた: %q", term, f.calls())
		}
	}
}

// agent が Claude Code でない端末（前に別のもの・agent の情報が無い）は動いているとして使わない。
// agent が claude-code の sh -c claude は動いているとして使う（TestUsesRunningOne）。
func TestNonAgentChiefOfStaffNotUsed(t *testing.T) {
	for _, term := range []string{
		`"argv":["sh","-c","claude"],"status":null,"foreground":{"name":"vim","argv":["/usr/bin/vim"]}`,
		`"argv":["sh","-c","claude"],"status":{"app":"codex"},"foreground":{"name":"sh","argv":["sh","-c","claude"]}`,
	} {
		f := newFake(t)
		f.setCoordinator("t3")
		t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
		t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+term+`}`)
		rc, out, errs := runMain()
		if rc != exitFail || strings.Contains(out, "動いているものを使う") || f.chief_of_staff() != "t3" || f.has("hc kill") || f.has("hc rm") || f.has("hc run") || f.has("kanban config") {
			t.Errorf("%s: rc=%d out=%q err=%q calls=%q", term, rc, out, errs, f.calls())
		}
	}
}

// 起動の直後: 最初の hc get には agent の情報がまだ無く、読み直すと status.app=claude-code。動いているとして使う（打ち直しを求めない）。
func TestChiefOfStaffAgentAppearsLater(t *testing.T) {
	f := newFake(t)
	spawn.SettleWait = 2 * time.Second
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t1"}]}`)
	t.Setenv("FAKE_GET1", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`","argv":["sh","-c","claude"],"status":null,"foreground":{"name":"sh","argv":["sh","-c","claude"]}}`)
	t.Setenv("FAKE_GET", `{"address":"t1","name":"sample-site-cos","cwd":"`+f.root+`",`+fgAgent+`}`)
	rc, out, errs := runMain()
	if rc != 0 || errs != "" || !strings.Contains(out, "t1（sample-site-cos・動いているものを使う）") || f.has("hc run") || f.has("hc kill") {
		t.Errorf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
	}
}
