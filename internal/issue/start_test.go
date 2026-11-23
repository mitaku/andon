package issue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"andon/internal/ledger/ledgertest"
	"andon/internal/spawn"
)

var sessionRe = regexp.MustCompile(`session=[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// start は偽の kanban の CLI と hc・本物の git（bare な origin とその clone）で見る（kanban を本物の kanban の CLI で回すのは tests/andon-issue.sh）。
// 偽の kanban の CLI は $FAKE_ROOT/show-<id>.json を返し、打たれたものを $LOG に書く。偽の hc は FAKE_LS・FAKE_TPL・FAKE_RUN（FAKE_RUN_RC）を返す。

const startKanban = `#!/usr/bin/env bash
printf 'kanban %s | HC_TERMINAL_ID=%s KANBAN_ROOT=%s\n' "$*" "${HC_TERMINAL_ID:-}" "${KANBAN_ROOT:-}" >>"$LOG"
case $1 in
  root) echo "$FAKE_ROOT" ;;
  show) cat "$FAKE_ROOT/show-$2.json" ;;
  set) echo "$2 updated" ;;
  ready) cat "$FAKE_ROOT/ready.json" ;;
  start) printf 'start-cwd %s\n' "$PWD" >>"$LOG"; echo "$2 doing" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const startHc = `#!/usr/bin/env bash
printf 'hc %s\n' "$*" >>"$LOG"
case $1 in
  ls) echo "$FAKE_LS" ;;
  templates) echo "$FAKE_TPL" ;;
  run) printf '%s\0' "${@:2}" >"$FAKE_ROOT/run-args"; [ "${FAKE_RUN_RC:-0}" = 0 ] || { echo "boom" >&2; exit 1; }; echo "$FAKE_RUN" ;;
  label) [ "${FAKE_LABEL_RC:-0}" = 0 ] || { echo "no label" >&2; exit 1; } ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

// 偽の claude・codex。起動の記録（launch.Gather）が打つ --version にだけバージョンを返す。検査が PATH の本物の harness を打たないように、
// newStart が一時の PATH の先頭に置く。--version のほかを打たれたら落ちる
const startHarness = `#!/bin/sh
[ "$1" = --version ] || { echo "unexpected $(basename "$0") $*" >&2; exit 9; }
echo "$(basename "$0") 0.0.0-fake"
`

type startFixture struct {
	t                        *testing.T
	root, log, repoDir, home string
	plant                    string
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
	for _, d := range []string{bin, filepath.Join(f.root, "kanban", "issues", "S-0007")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(bin, "kanban"), []byte(startKanban), 0o755)
	os.WriteFile(filepath.Join(bin, "hc"), []byte(startHc), 0o755)
	os.WriteFile(filepath.Join(bin, "claude"), []byte(startHarness), 0o755)
	os.WriteFile(filepath.Join(bin, "codex"), []byte(startHarness), 0o755)
	os.WriteFile(filepath.Join(f.root, "kanban", "issues", "S-0007", "brief.md"), []byte("# 題\n"), 0o644)
	os.WriteFile(filepath.Join(f.root, "kanban", "config.yaml"), []byte("version: 3\nrepos:\n  - github.com/example/sample-site\nchief_of_staff: t5\n"), 0o644)
	origin := filepath.Join(dir, "origin.git")
	gitT(t, "", "init", "-q", "--bare", "-b", "main", origin)
	gitT(t, "", "clone", "-q", origin, f.repoDir)
	gitT(t, f.repoDir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	gitT(t, f.repoDir, "push", "-q", "origin", "HEAD:main")
	// セルの repo（dir）: owner が commit した project.json が、書いてよい repo を持つ
	f.plant = dir
	gitT(t, dir, "init", "-q")
	os.WriteFile(filepath.Join(dir, "andon.toml"), []byte("owner = \"owner@example.com\"\n"), 0o644)
	f.project(`{"version":1,"project":{"title":"Sample site","prefix":"S"},"repos":["github.com/example/sample-site","github.com/example/none"]}`, "owner@example.com")
	f.task = map[string]any{"id": "S-0007", "title": "Add the button", "status": "todo", "profile": "claude", "repo": "github.com/example/sample-site"}
	// 予約キーは Chief of Staff（t5）が書いたもの（andon issue start は揃っていなければ断る）
	f.set("t5", map[string]any{"review": "on", "must_fix": "major", "reviewer": "codex-review", "review_rounds": "2", "merge": "person", "estimate": "medium"})
	f.ready(f.task)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", f.home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(f.home, ".local", "state")) // start の lock も一時の HOME の下に。受け継いだ置き場に書かない
	t.Setenv("GHQ_ROOT", filepath.Join(dir, "ghq"))
	// 信頼の設定（boot.TrustFromOS）は一時の HOME に書く。本物の CLAUDE_CONFIG_DIR・セルの repo に触らない
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	// 起動の記録（launch.Gather）が読む置き場は一時の HOME の下。受け継いだ本物の CODEX_HOME を読まない
	t.Setenv("CODEX_HOME", "")
	t.Setenv("ANDON_WORKSPACE", dir) // projects/sample-site/kanban を読む

	t.Setenv("LOG", f.log)
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("KANBAN_ROOT", "")
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

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func (f *startFixture) run(flags ...string) (int, string, string) {
	f.t.Helper()
	b := js(map[string]any{"issue": f.task})
	if err := os.WriteFile(filepath.Join(f.root, "show-S-0007.json"), []byte(b), 0o644); err != nil {
		f.t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := Main(append([]string{"start", "S-0007"}, flags...), strings.NewReader(""), &out, &errb)
	return rc, out.String(), errb.String()
}

// set は課題に予約キーを書き、端末 by が書いた行を kanban の log.jsonl に足す（kanban set の代わり。履歴の照合が読む）。
func (f *startFixture) set(by string, kv map[string]any) {
	f.t.Helper()
	for k, v := range kv {
		f.task[k] = v
	}
	b, _ := json.Marshal(map[string]any{"at": "2026-11-22T00:00:00Z", "by": "terminal:" + by, "id": "S-0007", "change": kv})
	lf, err := os.OpenFile(filepath.Join(f.root, "kanban", "log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		f.t.Fatal(err)
	}
	defer lf.Close()
	lf.Write(append(b, '\n'))
}

// unset は課題から予約キーを消す（書かなかった課題）。
func (f *startFixture) unset(keys ...string) {
	for _, k := range keys {
		delete(f.task, k)
	}
}

// project はセルの repo の projects/sample-site/project.json を書き、author を email にして commit する。
func (f *startFixture) project(body, email string) {
	f.t.Helper()
	os.WriteFile(filepath.Join(f.plant, "projects", "sample-site", "project.json"), []byte(body), 0o644)
	gitT(f.t, f.plant, "add", "andon.toml", "projects/sample-site/project.json")
	gitT(f.t, f.plant, "-c", "user.name=x", "-c", "user.email="+email, "commit", "-q", "-m", "project")
}

// addRepo は kanban の config.yaml の repos に足す。
// ready は偽の kanban の CLI に ready --json の返事を置く（kanban の CLI の書く形でなければテストを落とす）。
func (f *startFixture) ready(issues ...any) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, "ready.json"), []byte(ledgertest.Ready(f.t, issues...)), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *startFixture) addRepo(repo string) {
	p := filepath.Join(f.root, "kanban", "config.yaml")
	b, _ := os.ReadFile(p)
	s := strings.Replace(string(b), "repos:\n", "repos:\n  - "+repo+"\n", 1)
	os.WriteFile(p, []byte(s), 0o644)
}

func (f *startFixture) wt() string {
	return filepath.Join(f.plant, ".worktrees", "sample-site", "sample-site-S-0007")
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
	if !strings.Contains(out, "S-0007 を t42（sample-site-s-0007・claude）で起動した。cwd "+f.wt()+" branch sample-site/S-0007-add-the-button") {
		t.Errorf("out=%q", out)
	}
	if _, err := os.Stat(filepath.Join(f.wt(), ".git")); err != nil || !strings.Contains(f.branches(), "sample-site/S-0007-add-the-button") {
		t.Errorf("worktree か branch が無い: %v %q", err, f.branches())
	}
	// 置き場はセルの repo の .worktrees（ADR-0012 決めたこと 6）。中の .gitignore（*）でセルの repo の git status に出ない
	if b, err := os.ReadFile(filepath.Join(f.plant, ".worktrees", ".gitignore")); err != nil || string(b) != "*\n" {
		t.Errorf(".worktrees/.gitignore = %q, %v", b, err)
	}
	if st := gitOut(t, f.plant, "status", "--short"); strings.Contains(st, ".worktrees") {
		t.Errorf("セルの repo の git status に .worktrees が出る:\n%s", st)
	}
	brief := filepath.Join(f.root, "kanban", "issues", "S-0007", "brief.md")
	want := []string{
		"kanban root",
		"kanban show S-0007 --json",
		"kanban ready --json",
		"hc templates --json",
		"hc ls --json",
		"kanban set S-0007 branch=sample-site/S-0007-add-the-button worktree=" + f.wt(),
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-s-0007 --label project=sample-site --label issue=S-0007 --label title=Add the button --label herdr.space=andon-sample-site-S-0007 --label herdr.tab=andon-sample-site-S-0007 --label session=",
		"依頼書は `" + brief + "`。",
		"kanban start S-0007 | HC_TERMINAL_ID=t42 KANBAN_ROOT=" + f.root,
		"start-cwd " + f.wt(),
		"kanban set S-0007 session=",
	}
	calls := f.calls()
	// Claude 系は Andon が決めたセッションの id をラベルで渡し、同じ id を kanban の session に書く（ADR-0012 決めたこと 11）
	if len(calls) == len(want) {
		m := regexp.MustCompile(`--label session=([0-9a-f-]{36}) --cwd ` + regexp.QuoteMeta(f.wt()) + ` claude 課題`).FindStringSubmatch(calls[6])
		if m == nil || !strings.HasPrefix(calls[10], "kanban set S-0007 session="+m[1]+" ") {
			t.Errorf("session のラベルと kanban が合わない:\n%s\n%s", calls[6], calls[10])
		}
	}
	if len(calls) != len(want) {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
	for i := range want {
		if !strings.HasPrefix(calls[i], want[i]) {
			t.Errorf("call %d:\n got %q\nwant %q", i, calls[i], want[i])
		}
	}
	// 起動する前に案件の repo の根を信頼に足す（boot のあとに追加した repo でも信頼の画面を出さない）
	repo, _ := filepath.EvalSymlinks(f.repoDir)
	if cj := read(t, filepath.Join(f.home, ".claude.json")); !strings.Contains(cj, `"`+repo+`": {`) || !strings.Contains(cj, `"hasTrustDialogAccepted": true`) {
		t.Errorf(".claude.json に repo の根の信頼が無い:\n%s", cj)
	}
	if toml := read(t, filepath.Join(f.home, ".codex", "config.toml")); !strings.Contains(toml, `[projects."`+repo+`"]`+"\ntrust_level = \"trusted\"\n") {
		t.Errorf("config.toml に repo の根の信頼が無い:\n%s", toml)
	}
	// 起動の記録の harness のバージョンは、PATH の先頭に置いた偽の claude から読む（本物の harness を打たない）
	if l := read(t, filepath.Join(f.root, "kanban", "issues", "S-0007", "library", "launches.jsonl")); !strings.Contains(l, `"harness":{"name":"claude","version":"claude 0.0.0-fake"}`) {
		t.Errorf("起動の記録の harness が偽の claude でない:\n%s", l)
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
	cwd := filepath.Join(f.root, "kanban", "issues", "S-0007")
	if rc != 0 || errs != "" || !strings.Contains(out, "cwd "+cwd) || strings.Contains(out, "branch") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if f.has("kanban set S-0007 branch") || !f.has("start-cwd "+cwd) || !f.has("kanban set S-0007 session=") ||
		!f.has("hc run --json --bg --outlive-parent --no-notify --name sample-site-s-0007 --label project=sample-site --label issue=S-0007 --label title=Add the button --label herdr.space=andon-sample-site-S-0007 --label herdr.tab=andon-sample-site-S-0007 --label session=") {
		t.Errorf("calls=%q", f.calls())
	}
}

// レビュアーの課題として断るのは、parent があって profile が *-review の課題だけ。
// 子課題（観察など）と、parent の無い *-review の課題は起動する。
func TestStartNotReviewer(t *testing.T) {
	for _, c := range []struct {
		name, parent, profile string
	}{
		{"子課題", "S-0006", "codex"},
		{"parent の無い -review", "", "codex-review"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"codex"},{"name":"codex-review"}]}`)
			if c.parent != "" {
				f.task["parent"] = c.parent
			}
			f.task["profile"] = c.profile
			rc, out, errs := f.run()
			if rc != 0 || !f.has("kanban start S-0007") {
				t.Fatalf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
			}
		})
	}
}

func TestStartRefuses(t *testing.T) {
	for _, c := range startRefusals(t) {
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
			if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-0007") {
				t.Errorf("worktree か branch を作った: %q", f.branches())
			}
		})
	}
}

type startRefusal struct {
	name, want string
	hc         bool // hc を呼ぶか作る直前の確かめ（--dry-run では確かめない）
	edit       func(f *startFixture)
}

func startRefusals(t *testing.T) []startRefusal {
	return []startRefusal{
		{"not todo", "S-0007 は doing（andon issue start は todo の課題にだけ打てる。process/states/issue.yaml）", false, func(f *startFixture) { f.task["status"] = "doing" }},
		{"queued", "S-0007 は queued", false, func(f *startFixture) { f.task["status"] = "queued" }},
		{"started", "S-0007 は起動済み", false, func(f *startFixture) { f.task["branch"] = "sample-site/S-0007" }},
		{"machine", "machine=remote1（別のマシンで起動する）", false, func(f *startFixture) { f.task["machine"] = "remote1" }},
		{"no profile", "S-0007 に profile が無い", false, func(f *startFixture) { delete(f.task, "profile") }},
		{"option profile", "profile が不正", false, func(f *startFixture) { f.task["profile"] = "--help" }},
		{"reviewer", "S-0007 は S-0006 のレビュアーの課題（profile codex-review）。起動するのは dispatcher だけ", false, func(f *startFixture) {
			f.task["parent"] = "S-0006"
			f.task["profile"] = "codex-review"
		}},
		{"no brief", "依頼書（brief.md）が無い", false, func(f *startFixture) {
			os.Remove(filepath.Join(f.root, "kanban", "issues", "S-0007", "brief.md"))
		}},
		{"no template", "プロファイル fable の template が無い", true, func(f *startFixture) { f.task["profile"] = "fable" }},
		{"live name", "端末 sample-site-s-0007 がもう動いている", true, func(f *startFixture) {
			t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","name":"sample-site-s-0007"}]}`)
		}},
		{"bad repo", "repo が不正", false, func(f *startFixture) { f.task["repo"] = "example/sample-site" }},
		{"no clone", "リポジトリが無い", false, func(f *startFixture) {
			f.task["repo"] = "github.com/example/none"
			f.addRepo("github.com/example/none")
		}},
		{"not in repos", "repo github.com/example/other が kanban の config.yaml の repos に無い", false, func(f *startFixture) {
			// clone はあるが、このプロジェクトの repo ではない
			gitT(t, "", "clone", "-q", f.repoDir, filepath.Join(filepath.Dir(f.repoDir), "other"))
			f.task["repo"] = "github.com/example/other"
		}},
		{"not in project repos", "repo github.com/example/other が projects/sample-site/project.json の repos に無い", false, func(f *startFixture) {
			// kanban の config.yaml にはあるが、オーナーが書いた project.json の repos に無い
			gitT(t, "", "clone", "-q", f.repoDir, filepath.Join(filepath.Dir(f.repoDir), "other"))
			f.task["repo"] = "github.com/example/other"
			f.addRepo("github.com/example/other")
		}},
		{"no project.json", "セルの repo の HEAD に projects/sample-site/project.json が無い", false, func(f *startFixture) {
			gitT(t, f.plant, "rm", "-q", "projects/sample-site/project.json")
			gitT(t, f.plant, "-c", "user.name=o", "-c", "user.email=owner@example.com", "commit", "-q", "-m", "rm")
		}},
		{"project.json not by owner", "author（x <agent@example.com>）が owner でない", false, func(f *startFixture) {
			f.project(`{"version":1,"project":{"title":"Sample site","prefix":"S"},"repos":["github.com/example/sample-site"]}`, "agent@example.com")
		}},
		{"project.json broken", "project.json の形が違う", false, func(f *startFixture) {
			f.project(`{"version":1,"project":{"title":"Sample site","prefix":"S"}}`, "owner@example.com")
		}},
		{"after", "S-0007 は after（S-0006）が済んでいない", false, func(f *startFixture) {
			f.task["after"] = []any{"S-0006"}
			f.ready(map[string]any{"id": "S-0008", "title": "Other", "status": "todo"})
		}},
		// 予約キー（contract.md §2）: 無い・知らない値・最後に書いたのが今の Chief of Staff の端末でないキーがあれば断り、直す kanban set の行を出す
		{"no review", "S-0007 の予約キーが揃っていないので起動しない（担当は予約キーを書けず、実装を終えてから review-request・ready・gate で止まる）。" +
			"無い・知らない値: review。Chief of Staff の端末で打つ（<…> は brief.md の表から値を選んで埋める）: kanban set S-0007 'review=<on|off>'\n", false,
			func(f *startFixture) { f.unset("review") }},
		{"no estimate", "無い・知らない値: estimate。Chief of Staff の端末で打つ（<…> は brief.md の表から値を選んで埋める）: kanban set S-0007 'estimate=<small|medium|large>'\n", false,
			func(f *startFixture) { f.unset("estimate") }},
		{"unknown value", "無い・知らない値: review_rounds。", false,
			func(f *startFixture) { f.set("t5", map[string]any{"review_rounds": "0"}) }},
		{"written by worker", "最後に書いたのが今の Chief of Staff の端末（t5）でない: merge。Chief of Staff の端末で打つ: kanban set S-0007 merge=auto\n", false,
			func(f *startFixture) { f.set("t42", map[string]any{"merge": "auto"}) }},
		{"chief of staff restarted", "最後に書いたのが今の Chief of Staff の端末（t6）でない: review・must_fix・reviewer・review_rounds・merge・estimate。" +
			"Chief of Staff の端末で打つ: kanban set S-0007 review=on must_fix=major reviewer=codex-review review_rounds=2 merge=person estimate=medium\n", false,
			func(f *startFixture) {
				os.WriteFile(filepath.Join(f.root, "kanban", "config.yaml"), []byte("version: 3\nrepos:\n  - github.com/example/sample-site\nchief_of_staff: t6\n"), 0o644)
			}},
		{"missing and untrusted", "無い・知らない値: must_fix・reviewer。最後に書いたのが今の Chief of Staff の端末（t5）でない: estimate。" +
			"Chief of Staff の端末で打つ（<…> は brief.md の表から値を選んで埋める）: kanban set S-0007 'must_fix=<critical|major|minor|trivial>' 'reviewer=<codex-review|claude-review>' estimate=large\n", false,
			func(f *startFixture) {
				f.unset("must_fix", "reviewer")
				f.set("t42", map[string]any{"estimate": "large"})
			}},
		// review が off でも、PR を出す課題は gate が must_fix と merge を読む
		{"review off needs must_fix", "無い・知らない値: must_fix。", false, func(f *startFixture) {
			f.set("t5", map[string]any{"review": "off"})
			f.unset("must_fix", "reviewer", "review_rounds")
		}},
		// review が off でも、差分がレベル B・C なら dispatcher はレビュアーを起動し、値のある reviewer・review_rounds の照合を求める
		{"review off with untrusted reviewer", "最後に書いたのが今の Chief of Staff の端末（t5）でない: reviewer。Chief of Staff の端末で打つ: kanban set S-0007 reviewer=claude-review\n", false,
			func(f *startFixture) {
				f.set("t5", map[string]any{"review": "off"})
				f.set("t42", map[string]any{"reviewer": "claude-review"})
			}},
		{"review off with unknown review_rounds", "無い・知らない値: review_rounds。", false, func(f *startFixture) {
			f.set("t5", map[string]any{"review": "off", "review_rounds": "0"})
		}},
		{"origin branch", "branch sample-site/S-0007-add-the-button が origin にもうある", true, func(f *startFixture) {
			// clone は fetch していないが、origin にだけ同名の branch がある
			gitT(t, f.repoDir, "push", "-q", "origin", "HEAD:refs/heads/sample-site/S-0007-add-the-button")
			gitT(t, f.repoDir, "update-ref", "-d", "refs/remotes/origin/sample-site/S-0007-add-the-button")
		}},
	}
}

// 要らない予約キーは求めない: review が off なら reviewer・review_rounds、PR を出さない課題（repo が無い・調べる課題）なら
// must_fix・merge も。Chief Engineer の課題は予約キーを見ない。
func TestStartSettingsNotNeeded(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(f *startFixture)
	}{
		{"review off", func(f *startFixture) {
			f.set("t5", map[string]any{"review": "off"})
			f.unset("reviewer", "review_rounds")
		}},
		{"without repo", func(f *startFixture) {
			f.unset("repo", "must_fix", "reviewer", "review_rounds", "merge")
			f.set("t5", map[string]any{"review": "off", "estimate": "small"})
		}},
		{"candidates", func(f *startFixture) {
			f.unset("must_fix", "reviewer", "review_rounds", "merge")
			f.set("t5", map[string]any{"review": "off", "deliverable": "candidates", "estimate": "small"})
		}},
		{"chief engineer", func(f *startFixture) {
			t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"chief-engineer"}]}`)
			f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
			f.task["profile"] = "chief-engineer"
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			c.edit(f)
			rc, out, errs := f.run()
			if rc != 0 || !f.has("kanban start S-0007") {
				t.Fatalf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
			}
		})
	}
}

// ルーチンが足した課題（log.jsonl に by: routine:<名前>・fired の行がある）は、Chief of Staff の端末で打てば、無い予約キーを既定で
// 書いてから起動する（足すのは dispatcher の kanban tick で、Chief of Staff の書いた値を持てない）。書いてあるキーは変えない。
// --dry-run は書かない。欄だけの課題（kanban set で routine を書いた）と、ほかの端末で打ったときは今までどおり断る。
func TestStartFillsRoutineSettings(t *testing.T) {
	fired := func(f *startFixture, name, id string) {
		f.task["routine"] = name
		b, _ := json.Marshal(map[string]any{"at": "2026-11-23T00:00:00Z", "by": "routine:" + name, "routine": name, "fired": id})
		lf, err := os.OpenFile(filepath.Join(f.root, "kanban", "log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			f.t.Fatal(err)
		}
		defer lf.Close()
		lf.Write(append(b, '\n'))
	}
	const setLine = "kanban set S-0007 review=off must_fix=major merge=person estimate=medium | HC_TERMINAL_ID=t5"

	t.Run("fills", func(t *testing.T) {
		f := newStart(t)
		f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
		fired(f, "triage", "S-0007")
		rc, out, errs := f.run()
		if rc != 0 || errs != "" {
			t.Fatalf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
		}
		calls := f.calls()
		i := slices.IndexFunc(calls, func(c string) bool { return strings.HasPrefix(c, setLine) })
		j := slices.IndexFunc(calls, func(c string) bool { return strings.HasPrefix(c, "hc run ") })
		if i < 0 || j < 0 || i > j {
			t.Errorf("起動の前に既定を書かない:\n%s", strings.Join(calls, "\n"))
		}
	})
	t.Run("keeps written keys", func(t *testing.T) {
		f := newStart(t)
		f.unset("estimate", "merge")
		fired(f, "triage", "S-0007")
		if rc, out, errs := f.run(); rc != 0 {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		if !f.has("kanban set S-0007 merge=person estimate=medium | HC_TERMINAL_ID=t5") {
			t.Errorf("無いキーだけを書かない: %q", f.calls())
		}
	})
	t.Run("review on picks the other brand", func(t *testing.T) {
		f := newStart(t)
		f.task["profile"] = "codex"
		f.unset("reviewer", "review_rounds", "estimate")
		fired(f, "triage", "S-0007")
		if rc, out, errs := f.run(); rc != 0 {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		if !f.has("kanban set S-0007 reviewer=claude-review review_rounds=2 estimate=medium | HC_TERMINAL_ID=t5") {
			t.Errorf("review=on の課題で reviewer を別ブランドで埋めない: %q", f.calls())
		}
	})
	// 前の start の kanban set が issue.yaml を書いたあと log.jsonl に足す前に止まった（照合が外れた既定の値）なら、同じ値で書き直す。
	// 既定と違う値は誰かが選んだものなので、照合が外れていれば今までどおり断る
	// Chief of Staff が起き直してアドレスが変わった（前の端末 t5 が書いた行は照合が外れる）ときも同じ。
	t.Run("rewrites untrusted defaults", func(t *testing.T) {
		f := newStart(t)
		f.unset("reviewer", "review_rounds")
		f.set("t5", map[string]any{"review": "off", "must_fix": "major", "merge": "person", "estimate": "medium"})
		os.WriteFile(filepath.Join(f.root, "kanban", "config.yaml"), []byte("version: 3\nrepos:\n  - github.com/example/sample-site\nchief_of_staff: t6\n"), 0o644)
		t.Setenv("HC_TERMINAL_ID", "t6")
		fired(f, "triage", "S-0007")
		if rc, out, errs := f.run(); rc != 0 {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		if !f.has(strings.Replace(setLine, "HC_TERMINAL_ID=t5", "HC_TERMINAL_ID=t6", 1)) {
			t.Errorf("照合の外れた既定を書き直さない: %q", f.calls())
		}
	})
	t.Run("keeps refusing untrusted non-defaults", func(t *testing.T) {
		f := newStart(t)
		f.unset("estimate")
		f.set("t42", map[string]any{"merge": "auto"})
		fired(f, "triage", "S-0007")
		rc, _, errs := f.run()
		if rc != 1 || !strings.Contains(errs, "でない: merge。") || f.has("kanban set") {
			t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
		}
	})
	t.Run("dry run writes nothing", func(t *testing.T) {
		f := newStart(t)
		f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
		fired(f, "triage", "S-0007")
		rc, out, errs := f.run("--dry-run")
		if rc != 0 || !strings.Contains(out, "ルーチン triage が足した課題なので、無いキー（と照合の外れた既定のキー）を既定で書く: review=off must_fix=major merge=person estimate=medium") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		if f.has("kanban set") {
			t.Errorf("--dry-run が書いた: %q", f.calls())
		}
	})
	for _, c := range []struct {
		name string
		edit func(f *startFixture)
	}{
		{"routine key without a fired row", func(f *startFixture) { f.task["routine"] = "triage" }},
		{"fired as another issue", func(f *startFixture) { fired(f, "triage", "S-0008") }},
		{"not the chief of staff", func(f *startFixture) {
			fired(f, "triage", "S-0007")
			t.Setenv("HC_TERMINAL_ID", "t9")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
			c.edit(f)
			rc, _, errs := f.run()
			if rc != 1 || !strings.Contains(errs, "無い・知らない値: review・must_fix・reviewer・review_rounds・merge・estimate。") || f.has("kanban set") {
				t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
			}
		})
	}
}

// トリアージの apply が足した課題（記録の created_issue）は、apply が予約キーを書く前に止まっていても、Chief of Staff の端末で打てば
// 無いキーを既定で書いてから起動する。review は決定の値（束ならどれか1つが on で on）。記録に無い課題は今までどおり断る。
func TestStartFillsTriageSettings(t *testing.T) {
	record := func(f *startFixture, created string, reviews ...string) {
		var items []map[string]any
		for _, r := range reviews {
			items = append(items, map[string]any{"key": "k", "decision": "bundle", "created_issue": created, "issue": map[string]any{"review": r}})
		}
		items = append(items, map[string]any{"key": "p", "decision": "person", "created_issue": "S-0099"})
		dir := filepath.Join(f.root, "kanban", "library", "triage")
		os.MkdirAll(dir, 0o755)
		b, _ := json.Marshal(map[string]any{"items": items})
		os.WriteFile(filepath.Join(dir, "2026-11-23-S-0005.json"), b, 0o644)
	}
	for _, c := range []struct {
		name, created string
		reviews       []string
		want          string
	}{
		{"review on", "S-0007", []string{"off", "on"}, "kanban set S-0007 review=on must_fix=major reviewer=codex-review review_rounds=2 merge=person estimate=medium | HC_TERMINAL_ID=t5"},
		{"review off", "S-7", []string{"off"}, "kanban set S-0007 review=off must_fix=major merge=person estimate=medium | HC_TERMINAL_ID=t5"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
			record(f, c.created, c.reviews...)
			rc, out, errs := f.run("--dry-run")
			if rc != 0 || !strings.Contains(out, "トリアージ S-0005 が足した課題なので") || f.has("kanban set") {
				t.Fatalf("dry-run: rc=%d out=%q err=%q", rc, out, errs)
			}
			if rc, out, errs := f.run(); rc != 0 || !f.has(c.want) {
				t.Errorf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
			}
		})
	}
	// apply が kanban add のあと記録の前で止まった課題: 記録には足す前の印（adding）と題だけが残る。
	// 題が同じで、足したのが Chief of Staff の端末なら埋める。足した端末が違う・題が違うなら今までどおり断る。
	adding := func(f *startFixture, title, by string) {
		dir := filepath.Join(f.root, "kanban", "library", "triage")
		os.MkdirAll(dir, 0o755)
		b, _ := json.Marshal(map[string]any{"items": []map[string]any{
			{"key": "k", "decision": "fix", "adding": true, "issue": map[string]any{"title": title, "review": "on"}}}})
		os.WriteFile(filepath.Join(dir, "2026-11-23-S-0005.json"), b, 0o644)
		lf, _ := os.OpenFile(filepath.Join(f.root, "kanban", "log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		fmt.Fprintf(lf, `{"at":"2026-11-23T00:00:00Z","by":"%s","id":"S-0007","change":{"add":"%s"}}`+"\n", by, title)
		lf.Close()
	}
	t.Run("added before recorded", func(t *testing.T) {
		f := newStart(t)
		f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
		adding(f, "Add the button", "terminal:t5")
		want := "kanban set S-0007 review=on must_fix=major reviewer=codex-review review_rounds=2 merge=person estimate=medium | HC_TERMINAL_ID=t5"
		if rc, out, _ := f.run("--dry-run"); rc != 0 || !strings.Contains(out, "トリアージ S-0005 が足した課題なので") || f.has("kanban set") {
			t.Fatalf("dry-run: rc=%d out=%q", rc, out)
		}
		if rc, out, errs := f.run(); rc != 0 || !f.has(want) {
			t.Errorf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
		}
	})
	// kanban add が issue.yaml を書いたあと log.jsonl に足す前に止まった課題は、足した行が無い。印と題が合えば埋める。
	// log.jsonl が空なら（読めないのと分けられないので）今までどおり断る
	for _, c := range []struct {
		name string
		log  string
		ok   bool
	}{
		{"added without the add row", `{"at":"2026-11-23T00:00:00Z","by":"terminal:t5","config":{"chief_of_staff":"t5"}}` + "\n", true},
		{"added without any log row", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
			adding(f, "Add the button", "terminal:t5")
			if err := os.WriteFile(filepath.Join(f.root, "kanban", "log.jsonl"), []byte(c.log), 0o644); err != nil {
				t.Fatal(err)
			}
			want := "kanban set S-0007 review=on must_fix=major reviewer=codex-review review_rounds=2 merge=person estimate=medium | HC_TERMINAL_ID=t5"
			rc, out, errs := f.run()
			if c.ok && (rc != 0 || !f.has(want)) || !c.ok && (rc != 1 || !strings.Contains(errs, "無い・知らない値: review・") || f.has("kanban set")) {
				t.Errorf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
			}
		})
	}
	for _, c := range []struct{ name, title, by string }{
		{"added before recorded by another terminal", "Add the button", "terminal:t9"},
		{"added before recorded with another title", "Other", "terminal:t5"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
			adding(f, c.title, c.by)
			if rc, _, errs := f.run(); rc != 1 || !strings.Contains(errs, "無い・知らない値: review・") || f.has("kanban set") {
				t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
			}
		})
	}
	t.Run("not in the record", func(t *testing.T) {
		f := newStart(t)
		f.unset("review", "must_fix", "reviewer", "review_rounds", "merge", "estimate")
		record(f, "S-0008", "on")
		rc, _, errs := f.run()
		if rc != 1 || !strings.Contains(errs, "無い・知らない値: review・") || f.has("kanban set") {
			t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
		}
	})
}

func TestStartHcRunFails(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN_RC", "1")
	rc, _, errs := f.run()
	if rc != exitFail || !strings.Contains(errs, "hc run に失敗") || !strings.Contains(errs, "S-0007 は todo のまま") || f.has("kanban start") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	// やり直せるように worktree と branch を戻し、kanban の欄も消す
	if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-0007") || !f.has("kanban set S-0007 branch= worktree=") {
		t.Errorf("戻していない: branches=%q calls=%q", f.branches(), f.calls())
	}
}

func TestStartFirstMessageNotSent(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	// hc run は template の本文（役目の文）を課題の文の前に足して届ける。手で送る案内も同じ文にする
	tpl := filepath.Join(f.home, ".config", "herdr-core", "templates", "claude.md")
	if err := os.MkdirAll(filepath.Dir(tpl), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tpl, []byte("---\nname: claude\ndescription: d\ncommand: claude\n---\n役目の1文目。\n役目の2文目。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errs := f.run()
	if rc != exitFirst || out != "" {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	for _, want := range []string{
		"t42（sample-site-s-0007）に最初のメッセージが届いていない: trust prompt",
		"dispatcher が見回りで送り直す（画面が入力を受けられるようになれば。",
		"急ぐときだけ手で: hc send t42 '役目の1文目。\n役目の2文目。\n\n課題 `sample-site/S-0007`。",
		"kanban は doing・agent t42 のまま",
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("err に %q が無い: %q", want, errs)
		}
	}
	// 手で送る行は案内の最後（貼ったときに後ろの文を拾わない）
	if !strings.HasSuffix(errs, "`。'\n") {
		t.Errorf("手で送る行が最後でない: %q", errs)
	}
	// 送り直しは dispatcher に預ける（端末のラベル。contract.md §8）
	if !f.has("hc label t42 first_message.sent=false") {
		t.Errorf("ラベルを付けていない: calls=%q", f.calls())
	}
	// 起きた端末は kanban に結ぶ（worktree は残す。画面に答えて送り直せば続く）
	if !f.has("kanban start S-0007 | HC_TERMINAL_ID=t42") {
		t.Errorf("calls=%q", f.calls())
	}
	if _, err := os.Stat(f.wt()); err != nil {
		t.Errorf("worktree を消した: %v", err)
	}
}

// hc が送ったと返しても、動き出さなければ（started=false。起動の途中で入力を捨てられた）届いていない扱いにする。
func TestStartFirstMessageNotStarted(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":true,"started":false,"turn":1}}`)
	rc, _, errs := f.run()
	if rc != exitFirst || !strings.Contains(errs, "に最初のメッセージが届いていない: 送ったが動き出さなかった") {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if !f.has("hc label t42 first_message.sent=false") {
		t.Errorf("ラベルを付けていない: calls=%q", f.calls())
	}
}

// ラベルを付けられなければ、dispatcher は送り直さないので手で送る案内だけにする。
func TestStartFirstMessageNoLabel(t *testing.T) {
	f := newStart(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	t.Setenv("FAKE_LABEL_RC", "1")
	rc, _, errs := f.run()
	if rc != exitFirst || !strings.Contains(errs, "ラベル first_message.sent を付けられず") ||
		!strings.Contains(errs, "dispatcher は送り直さない。hc read t42 で画面を読み、答えてから手で送る: hc send t42 ") ||
		strings.Contains(errs, "dispatcher が見回りで送り直す") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

// 担当の置き場のラベルは slug・id・題だけで決まる（machine を引数に取らない。ADR-0002 決めたこと 6・前提 7、ADR-0003）。
func TestWorkLabels(t *testing.T) {
	want := []string{"project=sample-site", "issue=S-0007", "title=Add the button",
		"herdr.space=andon-sample-site-S-0007", "herdr.tab=andon-sample-site-S-0007"}
	if got := workLabels("sample-site", "S-0007", "Add the button"); !slices.Equal(got, want) {
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

// noHc は呼ばれたら落ちる偽の hc にする（--dry-run は hc を1回も呼ばない）。
func (f *startFixture) noHc() {
	f.t.Helper()
	bin := filepath.Join(filepath.Dir(f.log), "bin", "hc")
	if err := os.WriteFile(bin, []byte("#!/usr/bin/env bash\nprintf 'hc %s\\n' \"$*\" >>\"$LOG\"\necho 'hc は呼ばない' >&2\nexit 99\n"), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *startFixture) dryJSON() startPlan {
	f.t.Helper()
	rc, out, errs := f.run("--dry-run", "--json")
	if rc != 0 || errs != "" {
		f.t.Fatalf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	var p startPlan
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		f.t.Fatalf("JSON でない: %v\n%s", err, out)
	}
	return p
}

// runArgs は偽の hc が書いた hc run の引数（run の後ろ）。
func (f *startFixture) runArgs() []string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.root, "run-args"))
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
}

func TestStartDryRunCallsNoHc(t *testing.T) {
	for _, c := range []struct {
		name  string
		flags []string
		edit  func(f *startFixture)
	}{
		{"json", []string{"--dry-run", "--json"}, func(*startFixture) {}},
		{"human", []string{"--dry-run"}, func(*startFixture) {}},
		{"without repo", []string{"--dry-run"}, func(f *startFixture) { delete(f.task, "repo") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			f.noHc()
			c.edit(f)
			rc, out, errs := f.run(c.flags...)
			if rc != 0 || errs != "" || out == "" {
				t.Fatalf("rc=%d out=%q err=%q calls=%q", rc, out, errs, f.calls())
			}
			if f.has("hc ") || f.has("kanban set") || f.has("kanban start") {
				t.Errorf("hc を呼んだか kanban を書いた: %q", f.calls())
			}
			if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-0007") {
				t.Errorf("worktree か branch を作った: %q", f.branches())
			}
			// 信頼の設定（boot.TrustFromOS）も書かない
			for _, p := range []string{filepath.Join(f.home, ".claude.json"), filepath.Join(f.home, ".codex")} {
				if _, err := os.Stat(p); err == nil {
					t.Errorf("%s を書いた", p)
				}
			}
		})
	}
}

func TestStartDryRunHuman(t *testing.T) {
	f := newStart(t)
	f.noHc()
	os.WriteFile(filepath.Join(f.root, "kanban", "issues", "S-0007", "brief.md"), []byte("# 題\n完了条件: 押せる\n"), 0o644)
	rc, out, _ := f.run("--dry-run")
	brief := filepath.Join(f.root, "kanban", "issues", "S-0007", "brief.md")
	for _, want := range []string{
		"乾いた実行", "sample-site-s-0007", "claude", f.wt(), "sample-site/S-0007-add-the-button",
		"herdr.space=andon-sample-site-S-0007",
		"ok      delegation", "skipped template",
		"最初のメッセージ（hc run に渡す文）:\n課題 `sample-site/S-0007`。\n依頼書は `" + brief + "`。\n",
		"依頼書（" + brief + "）:\n# 題\n完了条件: 押せる\n",
	} {
		if rc != 0 || !strings.Contains(out, want) {
			t.Errorf("rc=%d %q が無い:\n%s", rc, want, out)
		}
	}
}

// --dry-run が出すものは、本番で hc run に渡すものと1文字も違わない（最初のメッセージ・名前・ラベル・cwd・プロファイル）。
func TestStartDryRunMatchesRun(t *testing.T) {
	for _, c := range []struct {
		name string
		edit func(f *startFixture)
	}{
		{"repo", func(*startFixture) {}},
		{"without repo", func(f *startFixture) { delete(f.task, "repo") }},
		{"japanese title", func(f *startFixture) { f.task["title"] = "返事の下書き「ボタン」\t" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			c.edit(f)
			p := f.dryJSON()
			if rc, _, errs := f.run(); rc != 0 {
				t.Fatalf("本番 rc=%d err=%q", rc, errs)
			}
			want := []string{"--json", "--bg", "--outlive-parent", "--no-notify", "--name", p.Terminal}
			for _, l := range p.Labels {
				want = append(want, "--label", l)
			}
			want = append(want, "--cwd", p.Cwd, p.Profile, p.FirstMessage)
			// session の値だけは起動するたびに新しい（Claude 系の --session-id。ADR-0012 決めたこと 11）
			norm := func(a []string) []string {
				out := slices.Clone(a)
				for i := range out {
					out[i] = sessionRe.ReplaceAllString(out[i], "session=<id>")
				}
				return out
			}
			if got := f.runArgs(); !slices.Equal(norm(got), norm(want)) || !slices.Contains(norm(want), "session=<id>") {
				t.Errorf("hc run の引数が違う:\n got %q\nwant %q", got, want)
			}
			if got := f.runArgs(); got[len(got)-1] != p.FirstMessage {
				t.Errorf("最初のメッセージが違う:\n got %q\nwant %q", got[len(got)-1], p.FirstMessage)
			}
			if b, _ := os.ReadFile(p.BriefPath); p.Brief != string(b) || p.Brief == "" {
				t.Errorf("依頼書の全文が違う: %q", p.Brief)
			}
			if _, has := f.task["repo"]; has && !f.has("kanban set S-0007 branch="+p.Branch+" worktree="+p.Worktree) {
				t.Errorf("branch・worktree が本番と違う: %+v calls=%q", p, f.calls())
			}
		})
	}
}

// 止まる課題（書けない repo など）では、--dry-run も本番と同じ終了コードと理由1行を返す。
func TestStartDryRunRefusesLikeRun(t *testing.T) {
	for _, c := range startRefusals(t) {
		if c.hc {
			continue // hc を呼ぶ・作る直前の確かめは --dry-run ではしない
		}
		t.Run(c.name, func(t *testing.T) {
			f := newStart(t)
			c.edit(f)
			rc, out, errs := f.run()
			f.noHc()
			drc, dout, derrs := f.run("--dry-run")
			jrc, jout, jerrs := f.run("--dry-run", "--json")
			if rc != exitFail || drc != rc || jrc != rc || derrs != errs || jerrs != errs || out != "" || dout != "" || jout != "" {
				t.Errorf("本番 rc=%d err=%q / dry rc=%d err=%q / json rc=%d err=%q", rc, errs, drc, derrs, jrc, jerrs)
			}
		})
	}
}

func TestStartUsage(t *testing.T) {
	for _, flags := range [][]string{{"--json"}, {"--dry-run", "--dry-run"}, {"--dry"}, {"--json", "--json", "--dry-run"}} {
		var out, errb bytes.Buffer
		if rc := Main(append([]string{"start", "S-0007"}, flags...), strings.NewReader(""), &out, &errb); rc != exitUsage || !strings.HasPrefix(errb.String(), "usage:") {
			t.Errorf("%q: rc=%d err=%q", flags, rc, errb.String())
		}
	}
}

func TestStartFirstMessageGuide(t *testing.T) {
	f := newStart(t)
	ws := os.Getenv("ANDON_WORKSPACE") // newStart が置いたセルの repo
	if m := f.dryJSON().FirstMessage; strings.Contains(m, spawn.GuideFile) {
		t.Errorf("ファイルが無いのに指している: %q", m)
	}
	os.WriteFile(filepath.Join(ws, spawn.GuideFile), []byte("x"), 0o644)
	if m := f.dryJSON().FirstMessage; !strings.Contains(m, filepath.Join(ws, spawn.GuideFile)) {
		t.Errorf("ファイルがあるのに指していない: %q", m)
	}
}
