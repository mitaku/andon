package task

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// start は偽の kanban と hc・本物の git（bare な origin とその clone）で見る（台帳を本物の kanban で回すのは tests/andon-task.sh）。
// 偽の kanban は $FAKE_ROOT/show-<id>.json を返し、打たれたものを $LOG に書く。偽の hc は FAKE_LS・FAKE_TPL・FAKE_RUN（FAKE_RUN_RC）を返す。

const startKanban = `#!/usr/bin/env bash
printf 'kanban %s | HC_TERMINAL_ID=%s FLIGHTDECK_ROOT=%s\n' "$*" "${HC_TERMINAL_ID:-}" "${FLIGHTDECK_ROOT:-}" >>"$LOG"
case $1 in
  root) echo "$FAKE_ROOT" ;;
  show) cat "$FAKE_ROOT/show-$2.json" ;;
  set) echo "$2 updated" ;;
  ready) cat "$FAKE_ROOT/ready.json" 2>/dev/null || echo '[{"id":"S-7"}]' ;;
  start) printf 'start-cwd %s\n' "$PWD" >>"$LOG"; echo "$2 doing" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const startHc = `#!/usr/bin/env bash
printf 'hc %s\n' "$*" >>"$LOG"
case $1 in
  ls) echo "$FAKE_LS" ;;
  templates) echo "$FAKE_TPL" ;;
  run) [ "${FAKE_RUN_RC:-0}" = 0 ] || { echo "boom" >&2; exit 1; }; echo "$FAKE_RUN" ;;
  label) [ "${FAKE_LABEL_RC:-0}" = 0 ] || { echo "no label" >&2; exit 1; } ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

type startFixture struct {
	t                        *testing.T
	root, log, repoDir, home string
	task                     map[string]any
}

func newStart(t *testing.T) *startFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	f := &startFixture{t: t, root: filepath.Join(dir, "projects", "sample-site"), log: filepath.Join(dir, "log"),
		repoDir: filepath.Join(dir, "ghq", "github.com", "example", "sample-site"), home: filepath.Join(dir, "home")}
	for _, d := range []string{bin, filepath.Join(f.root, ".flightdeck", "tasks", "S-7")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(bin, "kanban"), []byte(startKanban), 0o755)
	os.WriteFile(filepath.Join(bin, "hc"), []byte(startHc), 0o755)
	os.WriteFile(filepath.Join(f.root, ".flightdeck", "tasks", "S-7", "brief.md"), []byte("# 題\n"), 0o644)
	os.WriteFile(filepath.Join(f.root, ".flightdeck", "config.yaml"), []byte("version: 2\nrepos:\n  - github.com/example/sample-site\ncoordinator: t5\n"), 0o644)
	origin := filepath.Join(dir, "origin.git")
	gitT(t, "", "init", "-q", "--bare", "-b", "main", origin)
	gitT(t, "", "clone", "-q", origin, f.repoDir)
	gitT(t, f.repoDir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	gitT(t, f.repoDir, "push", "-q", "origin", "HEAD:main")
	f.task = map[string]any{"id": "S-7", "title": "Add the button", "status": "todo", "profile": "claude", "repo": "github.com/example/sample-site"}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", f.home)
	t.Setenv("GHQ_ROOT", filepath.Join(dir, "ghq"))
	// 信頼の設定（boot.TrustFromOS）は一時の HOME に書く。本物の CLAUDE_CONFIG_DIR・建屋の repo に触らない
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("ANDON_WORKSPACE", dir) // projects/sample-site/.flightdeck の台帳を読む

	t.Setenv("LOG", f.log)
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("FLIGHTDECK_ROOT", "")
	t.Setenv("HC_TERMINAL_ID", "t5") // 打った Chief of Staff。担当の agent にしてはいけない
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t5","name":"sample-site-cos"}]}`)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"codex"}]}`)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":true}}`)
	t.Setenv("FAKE_RUN_RC", "0")
	return f
}

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (f *startFixture) run() (int, string, string) {
	f.t.Helper()
	b := js(map[string]any{"task": f.task})
	if err := os.WriteFile(filepath.Join(f.root, "show-S-7.json"), []byte(b), 0o644); err != nil {
		f.t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := Main([]string{"start", "S-7"}, strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

// addRepo は台帳の config.yaml の repos に足す。
func (f *startFixture) addRepo(repo string) {
	p := filepath.Join(f.root, ".flightdeck", "config.yaml")
	b, _ := os.ReadFile(p)
	s := strings.Replace(string(b), "repos:\n", "repos:\n  - "+repo+"\n", 1)
	os.WriteFile(p, []byte(s), 0o644)
}

func (f *startFixture) wt() string {
	return filepath.Join(f.home, ".herdr", "worktrees", "sample-site", "sample-site-S-7")
}

func (f *startFixture) calls() []string {
	b, _ := os.ReadFile(f.log)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func (f *startFixture) has(prefix string) bool {
	for _, c := range f.calls() {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func (f *startFixture) branches() string {
	out, _ := exec.Command("git", "-C", f.repoDir, "branch", "--format=%(refname:short)").Output()
	return string(out)
}

func TestStart(t *testing.T) {
	f := newStart(t)
	rc, out, errs := f.run()
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	if !strings.Contains(out, "S-7 を t42（sample-site-s-7・claude）で起こした。cwd "+f.wt()+" branch sample-site/S-7-add-the-button") {
		t.Errorf("out=%q", out)
	}
	if _, err := os.Stat(filepath.Join(f.wt(), ".git")); err != nil || !strings.Contains(f.branches(), "sample-site/S-7-add-the-button") {
		t.Errorf("worktree か branch が無い: %v %q", err, f.branches())
	}
	brief := filepath.Join(f.root, ".flightdeck", "tasks", "S-7", "brief.md")
	want := []string{
		"kanban root",
		"kanban show S-7 --json",
		"kanban ready --json",
		"hc templates --json",
		"hc ls --json",
		"kanban set S-7 branch=sample-site/S-7-add-the-button worktree=" + f.wt(),
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-s-7 --label project=sample-site --label task=S-7 --label title=Add the button --label herdr.space=andon-sample-site-S-7 --label herdr.tab=andon-sample-site-S-7 --cwd " + f.wt() + " claude タスク `sample-site/S-7`。",
		"依頼書は `" + brief + "`。",
		"kanban start S-7 | HC_TERMINAL_ID=t42 FLIGHTDECK_ROOT=" + f.root,
		"start-cwd " + f.wt(),
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
	// 起こす前に案件の repo の根を信頼に足す（boot のあとに迎えた repo でも信頼の画面を出さない）
	repo, _ := filepath.EvalSymlinks(f.repoDir)
	if cj := read(t, filepath.Join(f.home, ".claude.json")); !strings.Contains(cj, `"`+repo+`": {`) || !strings.Contains(cj, `"hasTrustDialogAccepted": true`) {
		t.Errorf(".claude.json に repo の根の信頼が無い:\n%s", cj)
	}
	if toml := read(t, filepath.Join(f.home, ".codex", "config.toml")); !strings.Contains(toml, `[projects."`+repo+`"]`+"\ntrust_level = \"trusted\"\n") {
		t.Errorf("config.toml に repo の根の信頼が無い:\n%s", toml)
	}
	// skill はプロファイルの command が system prompt に足す（ADR-0001 決定 3）。最初のメッセージで読ませない
	if all := strings.Join(calls, "\n"); strings.Contains(all, "SKILL.md") || strings.Contains(all, "先に") {
		t.Errorf("最初のメッセージに「先に読む」の行が残る:\n%s", all)
	}
}

func TestStartWithoutRepo(t *testing.T) {
	f := newStart(t)
	delete(f.task, "repo")
	rc, out, errs := f.run()
	cwd := filepath.Join(f.root, ".flightdeck", "tasks", "S-7")
	if rc != 0 || errs != "" || !strings.Contains(out, "cwd "+cwd) || strings.Contains(out, "branch") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if f.has("kanban set") || !f.has("start-cwd "+cwd) || !f.has("hc run --json --bg --outlive-parent --no-notify --name sample-site-s-7 --label project=sample-site --label task=S-7 --label title=Add the button --label herdr.space=andon-sample-site-S-7 --label herdr.tab=andon-sample-site-S-7 --cwd "+cwd+" claude") {
		t.Errorf("calls=%q", f.calls())
	}
}

func TestStartRefuses(t *testing.T) {
	for _, c := range []struct {
		name, want string
		edit       func(f *startFixture)
	}{
		{"not todo", "S-7 は doing（起こせるのは todo だけ）", func(f *startFixture) { f.task["status"] = "doing" }},
		{"queued", "S-7 は queued", func(f *startFixture) { f.task["status"] = "queued" }},
		{"started", "S-7 は起こし済み", func(f *startFixture) { f.task["branch"] = "sample-site/S-7" }},
		{"machine", "machine=remote1（別のマシンで起こす）", func(f *startFixture) { f.task["machine"] = "remote1" }},
		{"no profile", "S-7 に profile が無い", func(f *startFixture) { delete(f.task, "profile") }},
		{"option profile", "profile が不正", func(f *startFixture) { f.task["profile"] = "--help" }},
		{"no brief", "依頼書（brief.md）が無い", func(f *startFixture) {
			os.Remove(filepath.Join(f.root, ".flightdeck", "tasks", "S-7", "brief.md"))
		}},
		{"no template", "プロファイル fable の template が無い", func(f *startFixture) { f.task["profile"] = "fable" }},
		{"live name", "端末 sample-site-s-7 がもう動いている", func(f *startFixture) {
			t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","name":"sample-site-s-7"}]}`)
		}},
		{"bad repo", "repo が不正", func(f *startFixture) { f.task["repo"] = "example/sample-site" }},
		{"no clone", "リポジトリが無い", func(f *startFixture) {
			f.task["repo"] = "github.com/example/none"
			f.addRepo("github.com/example/none")
		}},
		{"not in repos", "repo github.com/example/other が台帳の config.yaml の repos に無い", func(f *startFixture) {
			// clone はあるが、このプロジェクトの repo ではない
			gitT(t, "", "clone", "-q", f.repoDir, filepath.Join(filepath.Dir(f.repoDir), "other"))
			f.task["repo"] = "github.com/example/other"
		}},
		{"after", "S-7 は after（S-6）が済んでいない", func(f *startFixture) {
			f.task["after"] = []any{"S-6"}
			os.WriteFile(filepath.Join(f.root, "ready.json"), []byte(`[{"id":"S-8"}]`), 0o644)
		}},
		{"origin branch", "branch sample-site/S-7-add-the-button が origin にもうある", func(f *startFixture) {
			// clone は fetch していないが、origin にだけ同名の branch がある
			gitT(t, f.repoDir, "push", "-q", "origin", "HEAD:refs/heads/sample-site/S-7-add-the-button")
			gitT(t, f.repoDir, "update-ref", "-d", "refs/remotes/origin/sample-site/S-7-add-the-button")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			c.edit(f)
			rc, out, errs := f.run()
			if rc != exitFail || out != "" || !strings.Contains(errs, c.want) {
				t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
			}
			if f.has("hc run") || f.has("kanban set") || f.has("kanban start") {
				t.Errorf("止まる前に書いた: %q", f.calls())
			}
			if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-7") {
				t.Errorf("worktree か branch を作った: %q", f.branches())
			}
		})
	}
}

func TestStartHcRunFails(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN_RC", "1")
	rc, _, errs := f.run()
	if rc != exitFail || !strings.Contains(errs, "hc run に失敗") || !strings.Contains(errs, "S-7 は todo のまま") || f.has("kanban start") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	// やり直せるように worktree と branch を戻し、台帳の欄も消す
	if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-7") || !f.has("kanban set S-7 branch= worktree=") {
		t.Errorf("戻していない: branches=%q calls=%q", f.branches(), f.calls())
	}
}

func TestStartFirstMessageNotSent(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	rc, out, errs := f.run()
	if rc != exitFirst || out != "" {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	for _, want := range []string{
		"t42（sample-site-s-7）に最初のメッセージが届いていない: trust prompt",
		"クーリエが見回りで送り直す（画面が入力を受けられるようになれば。",
		"急ぐときだけ手で: hc send t42 \"タスク `sample-site/S-7`。",
		"台帳は doing・agent t42 のまま",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("err に %q が無い: %q", want, errs)
		}
	}
	// 送り直しはクーリエに預ける（端末のラベル。contract.md §8）
	if !f.has("hc label t42 first_message.sent=false") {
		t.Errorf("ラベルを付けていない: calls=%q", f.calls())
	}
	// 起きた端末は台帳に結ぶ（worktree は残す。画面に答えて送り直せば続く）
	if !f.has("kanban start S-7 | HC_TERMINAL_ID=t42") {
		t.Errorf("calls=%q", f.calls())
	}
	if _, err := os.Stat(f.wt()); err != nil {
		t.Errorf("worktree を消した: %v", err)
	}
}

// ラベルを付けられなければ、クーリエは送り直さないので手で送る案内だけにする。
func TestStartFirstMessageNoLabel(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	t.Setenv("FAKE_LABEL_RC", "1")
	rc, _, errs := f.run()
	if rc != exitFirst || !strings.Contains(errs, "ラベル first_message.sent を付けられず") ||
		!strings.Contains(errs, "クーリエは送り直さない。hc read t42 で画面を読み、答えてから手で送る: hc send t42 ") ||
		strings.Contains(errs, "クーリエが見回りで送り直す") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

// 担当の置き場のラベルは slug・id・題だけで決まる（machine を引数に取らない。ADR-0002 決めたこと 6・前提 7、ADR-0003）。
func TestWorkLabels(t *testing.T) {
	want := []string{"project=sample-site", "task=S-7", "title=Add the button",
		"herdr.space=andon-sample-site-S-7", "herdr.tab=andon-sample-site-S-7"}
	if got := workLabels("sample-site", "S-7", "Add the button"); !slices.Equal(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
