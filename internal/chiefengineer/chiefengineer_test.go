package chiefengineer

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fake は偽の deck と hc を PATH の先頭に置く。打たれた引数は $LOG に1行ずつ（コマンド名 引数… と、HC_TERMINAL_ID）。
// 振る舞いは環境変数で変える: FAKE_LS（hc ls の返事）・FAKE_TPL（hc templates）・FAKE_RUN（hc run）・FAKE_RUN_RC。
type fake struct {
	t       *testing.T
	root    string // 台帳の root（projects/sample-site）
	log     string
	repoDir string // ghq の clone（GHQ_ROOT/github.com/example/sample-site）
	home    string
}

const fakeDeck = `#!/usr/bin/env bash
printf 'deck %s | HC_TERMINAL_ID=%s FLIGHTDECK_ROOT=%s\n' "$*" "${HC_TERMINAL_ID:-}" "${FLIGHTDECK_ROOT:-}" >>"$LOG"
case $1 in
  root) [ -n "${FAKE_ROOT:-}" ] && echo "$FAKE_ROOT" || { echo "no ledger" >&2; exit 1; } ;;
  add) cat >"$LOG.brief"; mkdir -p "$FAKE_ROOT/.flightdeck/tasks/S-7"; echo S-7 ;;
  start) printf 'start-cwd %s\n' "$PWD" >>"$LOG"; echo "$2 doing" ;;
  set) echo "$2 updated" ;;
  *) echo "unexpected deck $1" >&2; exit 9 ;;
esac
`

const fakeHc = `#!/usr/bin/env bash
printf 'hc %s\n' "$*" >>"$LOG"
case $1 in
  ls) echo "$FAKE_LS" ;;
  templates) echo "$FAKE_TPL" ;;
  run) [ "$FAKE_RUN_RC" = 0 ] || { echo "boom" >&2; exit 1; }
       echo "$FAKE_RUN" ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

func newFake(t *testing.T) *fake {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"deck": fakeDeck, "hc": fakeHc} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := &fake{t: t, root: filepath.Join(dir, "projects", "sample-site"), log: filepath.Join(dir, "log"),
		repoDir: filepath.Join(dir, "ghq", "github.com", "example", "sample-site"), home: filepath.Join(dir, "home")}
	// 案件の repo: 既定の branch が main の bare な origin と、その clone
	origin := filepath.Join(dir, "origin.git")
	git(t, "", "init", "-q", "--bare", "-b", "main", origin)
	git(t, "", "clone", "-q", origin, f.repoDir)
	git(t, f.repoDir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	git(t, f.repoDir, "push", "-q", "origin", "HEAD:main")
	t.Setenv("GHQ_ROOT", filepath.Join(dir, "ghq"))
	t.Setenv("HOME", f.home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOG", f.log)
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("HC_TERMINAL_ID", "t1") // 打った人の端末。Chief Engineer の agent にしてはいけない
	t.Setenv("ANDON_HOME", "/opt/andon-test")
	t.Setenv("FAKE_LS", `{"terminals":[]}`)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"chief-engineer"}]}`)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":true}}`)
	t.Setenv("FAKE_RUN_RC", "0")
	return f
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (f *fake) wt() string {
	return filepath.Join(f.home, ".herdr", "worktrees", "sample-site", "sample-site-S-7")
}

func (f *fake) branches() string {
	out, _ := exec.Command("git", "-C", f.repoDir, "branch", "--format=%(refname:short)").Output()
	return strings.TrimSpace(string(out))
}

func (f *fake) calls() []string {
	b, err := os.ReadFile(f.log)
	if err != nil {
		return nil
	}
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

func TestStartsChiefEngineer(t *testing.T) {
	f := newFake(t)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "Billing の設計")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if !strings.Contains(out, "S-7（Billing の設計）を t42（sample-site-s-7）で起こした。cwd "+f.wt()+" branch sample-site/S-7-billing") {
		t.Errorf("out=%q", out)
	}
	if b := f.branches(); !strings.Contains(b, "sample-site/S-7-billing") {
		t.Errorf("branch が無い: %q", b)
	}
	if _, err := os.Stat(filepath.Join(f.wt(), ".git")); err != nil {
		t.Errorf("worktree が無い: %v", err)
	}
	calls := f.calls()
	want := []string{
		"deck root",
		"hc ls --json --label role=chief-engineer --label project=sample-site",
		"hc templates --json",
		"deck add Billing の設計 --repo github.com/example/sample-site --profile chief-engineer --brief - | HC_TERMINAL_ID=t1 FLIGHTDECK_ROOT=" + f.root,
		"deck set S-7 branch=sample-site/S-7-billing worktree=" + f.wt() + " |",
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-s-7 --label role=chief-engineer --label project=sample-site --label task=S-7 --label title=Billing の設計 --cwd " + f.wt() + " chief-engineer タスク `sample-site/S-7`。",
		"依頼書は `" + f.root + "/.flightdeck/tasks/S-7/brief.md`。",
		"先に `/opt/andon-test/skills/chief-engineer/SKILL.md` を読む。",
		"deck start S-7 | HC_TERMINAL_ID=t42 FLIGHTDECK_ROOT=" + f.root,
		"start-cwd " + f.wt(),
	}
	if len(calls) != len(want) {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
	for i := range want {
		if !strings.HasPrefix(calls[i], want[i]) {
			t.Errorf("call %d:\n got %q\nwant %q", i, calls[i], want[i])
		}
	}
	brief, _ := os.ReadFile(f.log + ".brief")
	if !strings.HasPrefix(string(brief), "# Billing の設計\n") {
		t.Errorf("brief=%q", brief)
	}
}

func TestUsage(t *testing.T) {
	newFake(t)
	r := "--repo=github.com/example/sample-site"
	for _, args := range [][]string{nil, {""}, {"題"}, {r}, {r, ""}, {r, "  "}, {r, "a", "b"}, {r, "二行\nの題"}, {r, "--no-attach", "題"}, {"--repo"}} {
		rc, out, errs := runMain(args...)
		if rc != exitUsage || out != "" || !strings.HasPrefix(errs, "usage: andon chief-engineer") {
			t.Errorf("%q: rc=%d out=%q err=%q", args, rc, out, errs)
		}
	}
}

func TestNoLedger(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_ROOT", "")
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "台帳が見つからない") || f.has("hc") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestBadRepo(t *testing.T) {
	f := newFake(t)
	rc, _, errs := runMain("--repo", "example/sample-site", "題")
	if rc != exitUsage || !strings.Contains(errs, "repo が不正") || f.has("deck") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
	rc, _, errs = runMain("--repo", "github.com/example/none", "題")
	if rc != exitFail || !strings.Contains(errs, "リポジトリが無い") || f.has("deck add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

// contract の project slug（英小文字・数字・_・-）は通る
func TestUnderscoreSlug(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_ROOT", filepath.Join(t.TempDir(), "projects", "sample_site"))
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != 0 || !strings.Contains(out, "sample_site-s-7") || !strings.Contains(f.branches(), "sample_site/S-7") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestBadSlug(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_ROOT", filepath.Join(t.TempDir(), "Bad_Slug"))
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "slug が不正") || f.has("deck add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestAlreadyRunning(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","task":"S-3"}}]}`)
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "Chief Engineer が動いている: t9（タスク S-3）") || f.has("deck add") || f.has("hc run") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestNoTemplate(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"}]}`)
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "プロファイル chief-engineer の template が無い") || f.has("deck add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestHcRunFails(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN_RC", "1")
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "hc run に失敗") || !strings.Contains(errs, "S-7 は todo のまま残る") || f.has("deck start") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	// やり直せるように worktree と branch を戻し、台帳の欄も消す
	if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-7") || !f.has("deck set S-7 branch= worktree=") {
		t.Errorf("戻していない: branches=%q calls=%q", f.branches(), f.calls())
	}
}

func TestFirstMessageNotSent(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFirst || out != "" || !strings.Contains(errs, "t42（sample-site-s-7）に最初のメッセージが届いていない: trust prompt") || !f.has("deck start S-7 | HC_TERMINAL_ID=t42") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestShortTitle(t *testing.T) {
	for in, want := range map[string]string{"課金の設計": "", "Billing の設計": "billing", "A very long English title for the design doc": "a-very-long-english-title-for"} {
		if got := shortTitle(in); got != want {
			t.Errorf("shortTitle(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDefaultHome(t *testing.T) {
	t.Setenv("ANDON_HOME", "")
	if h := home(); h != "/opt/andon" {
		t.Errorf("home=%q", h)
	}
}
