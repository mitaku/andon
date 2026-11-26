package chiefengineer

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"andon/internal/spawn"
)

// fake は偽の kanban の CLI と hc を PATH の先頭に置く。打たれた引数は $LOG に1行ずつ（コマンド名 引数… と、HC_TERMINAL_ID）。
// 振る舞いは環境変数で変える: FAKE_LS（hc ls の返事）・FAKE_GET（hc get。無ければ Claude Code が動いている）・FAKE_TPL（hc templates）・FAKE_RUN（hc run）・FAKE_RUN_RC。
type fake struct {
	t       *testing.T
	root    string // kanban の root（projects/sample-site）
	log     string
	repoDir string // ghq の clone（GHQ_ROOT/github.com/example/sample-site）
	home    string
}

const fakeDeck = `#!/usr/bin/env bash
printf 'kanban %s | HC_TERMINAL_ID=%s KANBAN_ROOT=%s\n' "$*" "${HC_TERMINAL_ID:-}" "${KANBAN_ROOT:-}" >>"$LOG"
case $1 in
  root) [ -n "${FAKE_ROOT:-}" ] && echo "$FAKE_ROOT" || { echo "no ledger" >&2; exit 1; } ;;
  add) cat >"$LOG.brief"; mkdir -p "$FAKE_ROOT/kanban/issues/S-7"; echo S-7 ;;
  start) printf 'start-cwd %s\n' "$PWD" >>"$LOG"; echo "$2 doing" ;;
  set) echo "$2 updated" ;;
  *) echo "unexpected kanban $1" >&2; exit 9 ;;
esac
`

const fakeHc = `#!/usr/bin/env bash
printf 'hc %s\n' "$*" >>"$LOG"
case $1 in
  ls) echo "$FAKE_LS" ;;
  get) n=$(( $(cat "$LOG.gets" 2>/dev/null || echo 0) + 1 )); echo $n > "$LOG.gets"; s="FAKE_GET$n"  # FAKE_GET<n> があれば n 回目の get はそれ
       if [ -n "${!s:-}" ]; then echo "${!s}"
       elif [ -n "${FAKE_GET:-}" ]; then echo "$FAKE_GET"
       else echo '{"argv":["sh","-c","claude"],"status":{"app":"claude-code"},"foreground":{"name":"sh","argv":["sh","-c","claude"]}}'; fi ;;
  keep|kill|rm) ;;
  templates) echo "$FAKE_TPL" ;;
  run) [ "$FAKE_RUN_RC" = 0 ] || { echo "boom" >&2; exit 1; }
       echo "$FAKE_RUN" ;;
  label) [ "${FAKE_LABEL_RC:-0}" = 0 ] || { echo "no label" >&2; exit 1; } ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

// 偽の claude・codex。起動の記録（launch.Gather）が打つ --version にだけバージョンを返す。検査が PATH の本物の harness を打たないように、
// newFake が一時の PATH の先頭に置く。--version のほかを打たれたら落ちる
const fakeHarness = `#!/bin/sh
[ "$1" = --version ] || { echo "unexpected $(basename "$0") $*" >&2; exit 9; }
echo "$(basename "$0") 0.0.0-fake"
`

func newFake(t *testing.T) *fake {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"kanban": fakeDeck, "hc": fakeHc, "claude": fakeHarness, "codex": fakeHarness} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f := &fake{t: t, root: filepath.Join(dir, "projects", "sample-site"), log: filepath.Join(dir, "log"),
		repoDir: filepath.Join(dir, "ghq", "github.com", "example", "sample-site"), home: filepath.Join(dir, "home")}
	origin := filepath.Join(dir, "origin.git")
	git(t, "", "init", "-q", "--bare", "-b", "main", origin)
	git(t, "", "clone", "-q", origin, f.repoDir)
	git(t, f.repoDir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	git(t, f.repoDir, "push", "-q", "origin", "HEAD:main")
	t.Setenv("GHQ_ROOT", filepath.Join(dir, "ghq"))
	f.config("repos:\n  - github.com/example/sample-site\n")
	t.Setenv("HOME", f.home)
	// 信頼の設定（boot.TrustFromOS）は一時の HOME に書く。本物の CLAUDE_CONFIG_DIR・セルの repo に触らない
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")       // 起動の記録が読む置き場は一時の HOME の下。受け継いだ本物の CODEX_HOME を読まない
	t.Setenv("ANDON_WORKSPACE", dir) // projects/sample-site/kanban/ を読む
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOG", f.log)
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("HC_TERMINAL_ID", "t1") // 打った人の端末。Chief Engineer の agent にしてはいけない
	t.Setenv("ANDON_HOME", "/opt/andon-test")
	t.Setenv("FAKE_LS", `{"terminals":[]}`)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"chief-engineer"}]}`)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":true}}`)
	t.Setenv("FAKE_RUN_RC", "0")
	spawn.SettleWait = time.Second
	t.Cleanup(func() { spawn.SettleWait = 5 * time.Second })
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

func (f *fake) config(repos string) {
	f.t.Helper()
	p := filepath.Join(f.root, "kanban", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	body := "version: 3\nprefix: S\ncolumns:\n  - todo\n  - done\nphases: []\n" + repos + "chief_of_staff: t1\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fake) wt() string {
	return filepath.Join(filepath.Dir(filepath.Dir(f.root)), ".worktrees", "sample-site", "sample-site-S-7") // <ANDON_WORKSPACE>/.worktrees
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
	if !strings.Contains(out, "S-7（Billing の設計）を t42（sample-site-s-7）で起動した。cwd "+f.wt()+" branch sample-site/S-7-billing") {
		t.Errorf("out=%q", out)
	}
	// 最後に出た場所を「<Space 名> / <tab 名>」で出す。hc attach は打たない（ADR-0002 決めたこと 14・16・17）
	if !strings.HasSuffix(out, "andon design start: 出た場所 sample-site / CE S-7 Billing の設計（t42）\n") || strings.Contains(out, "hc attach") || f.has("hc attach") {
		t.Errorf("出た場所が最後に無い・attach が残る: out=%q", out)
	}
	// 出た場所の手前に次の1手の行（ADR-0053 決めたこと 14・受け入れ条件 A12）
	if !strings.HasSuffix(out, "andon design start: 次: 出た場所の端末で Chief Engineer と話す。止まったら andon issue resume S-7 --project sample-site\nandon design start: 出た場所 sample-site / CE S-7 Billing の設計（t42）\n") {
		t.Errorf("次の1手（andon issue resume）が出た場所の手前に無い: out=%q", out)
	}
	if b := f.branches(); !strings.Contains(b, "sample-site/S-7-billing") {
		t.Errorf("branch が無い: %q", b)
	}
	if _, err := os.Stat(filepath.Join(f.wt(), ".git")); err != nil {
		t.Errorf("worktree が無い: %v", err)
	}
	calls := f.calls()
	want := []string{
		"kanban root",
		"hc templates --json",
		"kanban add Billing の設計 --repo github.com/example/sample-site --profile chief-engineer --brief - | HC_TERMINAL_ID=t1 KANBAN_ROOT=" + f.root,
		"kanban set S-7 branch=sample-site/S-7-billing worktree=" + f.wt() + " |",
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-s-7 --label role=chief-engineer --label project=sample-site --label issue=S-7 --label title=Billing の設計 --label herdr.space=andon-sample-site --label herdr.tab=andon-sample-site-S-7 --label session=",
		"依頼書は `" + f.root + "/kanban/issues/S-7/brief.md`。",
		"kanban start S-7 | HC_TERMINAL_ID=t42 KANBAN_ROOT=" + f.root,
		"start-cwd " + f.wt(),
		"kanban set S-7 session=",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
	// skill はプロファイルの command が system prompt に足す（ADR-0001 決定 3）。最初のメッセージで読ませない
	if all := strings.Join(calls, "\n"); strings.Contains(all, "SKILL.md") || strings.Contains(all, "先に") {
		t.Errorf("最初のメッセージに「先に読む」の行が残る:\n%s", all)
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
	// 渡したセッションの id（ラベル）と kanban に書いた id が同じ（ADR-0012 決めたこと 11）
	m := regexp.MustCompile(`--label session=([0-9a-f-]{36}) --cwd ` + regexp.QuoteMeta(f.wt()) + ` chief-engineer `).FindStringSubmatch(calls[4])
	if m == nil || !spawn.SessionRe.MatchString(m[1]) || !strings.HasPrefix(calls[len(calls)-1], "kanban set S-7 session="+m[1]+" ") {
		t.Errorf("session のラベルと kanban が合わない:\n%s\n%s", calls[4], calls[len(calls)-1])
	}
}

func TestUsage(t *testing.T) {
	newFake(t)
	r := "--repo=github.com/example/sample-site"
	for _, args := range [][]string{nil, {""}, {r}, {r, ""}, {r, "  "}, {r, "a", "b"}, {r, "二行\nのタイトル"}, {r, "--no-attach", "タイトル"}, {"--repo"}} {
		rc, out, errs := runMain(args...)
		if rc != exitUsage || out != "" || !strings.HasPrefix(errs, "usage: andon design start") {
			t.Errorf("%q: rc=%d out=%q err=%q", args, rc, out, errs)
		}
	}
}

func TestNoLedger(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_ROOT", "")
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != exitFail || !strings.Contains(errs, "kanban が見つからない") || f.has("hc") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
}

func TestDocumentedInvocations(t *testing.T) {
	re := regexp.MustCompile("andon design start((?: (?:\\[--repo <[^>]+>\\]|--repo <[^>]+>))?)( --brief -)? \"<タイトル>\"")
	files, _ := filepath.Glob("../../package/skills/*/SKILL.md")
	files = append(files, "../../docs/design.md", "../../README.md", "../../README.ja.md")
	found := 0
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			found++
			args := []string{"タイトル"}
			if strings.Contains(m[1], "--repo") && !strings.Contains(m[1], "[") {
				args = []string{"--repo", "github.com/example/sample-site", "タイトル"}
			}
			in := ""
			if m[2] != "" {
				args = append([]string{"--brief", "-"}, args...)
				in = "経緯: オーナーの言葉の写し\n"
			}
			t.Run(file+":"+m[0], func(t *testing.T) {
				newFake(t)
				var out, errb bytes.Buffer
				if rc := Main(args, strings.NewReader(in), &out, &errb); rc != 0 {
					t.Errorf("%s: rc=%d err=%q", m[0], rc, errb.String())
				}
			})
		}
	}
	if found < 3 {
		t.Errorf("文書に andon design start の呼び出しが %d 個しか見つからない", found)
	}
	// 呼び出しの形が上の正規表現に合わない書き方が文書に残っていない
	loose := regexp.MustCompile("andon design start [^`）]*")
	for _, file := range files {
		b, _ := os.ReadFile(file)
		for _, m := range loose.FindAllString(string(b), -1) {
			if !re.MatchString(m) && strings.Contains(m, "<") {
				t.Errorf("%s: 確かめられない呼び出しの書き方: %q", file, m)
			}
		}
	}
}

func TestLedgerRepos(t *testing.T) {
	for _, c := range []struct {
		repos string
		rc    int
		err   string
	}{
		{"repos:\n  - github.com/example/a\n  - github.com/example/b\n", exitUsage, "kanban の repos が 2 個（github.com/example/a・github.com/example/b）。--repo"},
		{"repos: []\n", exitUsage, "kanban の repos が 0 個"},
		{"", exitUsage, "kanban の repos が 0 個"},
		{"repos: [github.com/example/a, github.com/example/b]\n", exitUsage, "kanban の repos が 2 個"},
		{"- github.com/example/sample-site\n", exitUsage, "repos が読めない"},
	} {
		f := newFake(t)
		f.config(c.repos)
		rc, _, errs := runMain("タイトル")
		if rc != c.rc || !strings.Contains(errs, c.err) || f.has("kanban add") {
			t.Errorf("%q: rc=%d err=%q", c.repos, rc, errs)
		}
	}
	f := newFake(t)
	f.config("repos:\n  - \"github.com/example/sample-site\"\n")
	if rc, _, errs := runMain("タイトル"); rc != 0 || !f.has("kanban add タイトル --repo github.com/example/sample-site ") {
		t.Errorf("1つ: rc=%d err=%q", rc, errs)
	}
	f = newFake(t)
	f.config("repos: [github.com/example/sample-site]\n")
	if rc, _, errs := runMain("タイトル"); rc != 0 || !f.has("kanban add タイトル --repo github.com/example/sample-site ") {
		t.Errorf("flow の並び: rc=%d err=%q", rc, errs)
	}
	f = newFake(t)
	f.config("repos:\n  - github.com/example/a\n  - github.com/example/b\n")
	if rc, _, errs := runMain("--repo", "github.com/example/sample-site", "タイトル"); rc != 0 {
		t.Errorf("--repo があれば repos が2つでも通る: rc=%d err=%q", rc, errs)
	}
}

func TestBadRepo(t *testing.T) {
	f := newFake(t)
	rc, _, errs := runMain("--repo", "example/sample-site", "タイトル")
	if rc != exitUsage || !strings.Contains(errs, "repo が不正") || f.has("kanban") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
	rc, _, errs = runMain("--repo", "github.com/example/none", "タイトル")
	if rc != exitFail || !strings.Contains(errs, "リポジトリが無い") || f.has("kanban add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestUnderscoreSlug(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_ROOT", filepath.Join(t.TempDir(), "projects", "sample_site"))
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != 0 || !strings.Contains(out, "sample_site-s-7") || !strings.Contains(f.branches(), "sample_site/S-7") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestBadSlug(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_ROOT", filepath.Join(t.TempDir(), "Bad_Slug"))
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != exitFail || !strings.Contains(errs, "slug が不正") || f.has("kanban add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

// Chief Engineer は1プロジェクトに同時に複数いてよい（ADR-0009 決めたこと 3）。動いている端末があっても起動し、その端末には触らない。
func TestStartsWhileAnotherRunning(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-3","title":"Billing の設計"}}]}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != 0 || errs != "" || !strings.Contains(out, "S-7（タイトル）を t42") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if !f.has("kanban add") || !f.has("hc run") || f.has("hc keep") || f.has("hc kill") || f.has("hc rm") {
		t.Errorf("起動していない・動いている端末に触った: %q", f.calls())
	}
}

// 作り直しで素のシェルに替わった Chief Engineer の端末も片付けない（その課題は andon issue resume が続きから起動する。ADR-0012 決めたこと 12）。
// hc ls・hc get も打たない（ほかの Chief Engineer の端末を見ない）。
func TestShellChiefEngineerIsLeft(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-3","title":"Billing の設計"}}]}`)
	t.Setenv("FAKE_GET", `{"address":"t9","argv":["/bin/bash"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != 0 || errs != "" || !strings.Contains(out, "S-7（タイトル）を t42") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	for _, c := range []string{"hc ls", "hc get", "hc keep", "hc kill", "hc rm"} {
		if f.has(c) {
			t.Errorf("%s を打った: %q", c, f.calls())
		}
	}
}

func TestBriefFromStdin(t *testing.T) {
	head := "# タイトル\n\nChief Engineer の課題。オーナーと直接話して仕様・設計を詰め、文書にして Chief of Staff へ渡す。\n" +
		"タイトルはオーナーの1行。手順は skill chief-engineer。\n"
	for _, args := range [][]string{{"--brief", "-", "タイトル"}, {"タイトル", "--brief=-"}} {
		f := newFake(t)
		var out, errb bytes.Buffer
		in := "## 経緯\r\nオーナー: 「請求を月ごとにしたい」\n出典: docs/billing.md\n\n"
		rc := Main(append([]string{"--repo", "github.com/example/sample-site"}, args...), strings.NewReader(in), &out, &errb)
		if rc != 0 {
			t.Fatalf("%q: rc=%d err=%q", args, rc, errb.String())
		}
		brief, _ := os.ReadFile(f.log + ".brief")
		want := head + "\n## 経緯\nオーナー: 「請求を月ごとにしたい」\n出典: docs/billing.md\n"
		if string(brief) != want {
			t.Errorf("%q: brief=%q\nwant %q", args, brief, want)
		}
	}
	f := newFake(t)
	var out, errb bytes.Buffer
	if rc := Main([]string{"--repo", "github.com/example/sample-site", "タイトル"}, strings.NewReader("読まない"), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	if brief, _ := os.ReadFile(f.log + ".brief"); string(brief) != head {
		t.Errorf("--brief 無しで依頼書が変わった: %q", brief)
	}
}

func TestBriefBad(t *testing.T) {
	f := newFake(t)
	var out, errb bytes.Buffer
	rc := Main([]string{"--repo", "github.com/example/sample-site", "--brief", "-", "タイトル"}, strings.NewReader(" \n"), &out, &errb)
	if rc != exitUsage || !strings.Contains(errb.String(), "--brief - の stdin が空") || f.has("hc") || f.has("kanban") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errb.String(), f.calls())
	}
	r := "--repo=github.com/example/sample-site"
	for _, args := range [][]string{{r, "--brief", "タイトル"}, {r, "--brief", "x.md", "タイトル"}, {r, "--brief=x", "タイトル"}, {r, "--brief", "-", "--brief", "-", "タイトル"}} {
		if rc, _, errs := runMain(args...); rc != exitUsage || !strings.HasPrefix(errs, "usage: andon design start") {
			t.Errorf("%q: rc=%d err=%q", args, rc, errs)
		}
	}
}

func TestNoTemplate(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"}]}`)
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != exitFail || !strings.Contains(errs, "プロファイル chief-engineer の template が無い") || f.has("kanban add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestHcRunFails(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN_RC", "1")
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != exitFail || !strings.Contains(errs, "hc run に失敗") || !strings.Contains(errs, "S-7 は todo のまま残る") || f.has("kanban start") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	// やり直せるように worktree と branch を戻し、kanban の欄も消す
	if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-7") || !f.has("kanban set S-7 branch= worktree=") {
		t.Errorf("戻していない: branches=%q calls=%q", f.branches(), f.calls())
	}
}

func TestFirstMessageNotSent(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	// hc run は template の本文（役目の文）を課題の文の前に足して届ける。手で送る案内も同じ文にする
	tpl := filepath.Join(f.home, ".config", "herdr-core", "templates", "chief-engineer.md")
	if err := os.MkdirAll(filepath.Dir(tpl), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tpl, []byte("---\nname: chief-engineer\ndescription: d\ncommand: claude\n---\n役目の1文目。\n役目の2文目。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "タイトル")
	if rc != exitFirst || out != "andon design start: 出た場所 sample-site / CE S-7 タイトル（t42）\n" ||
		!strings.Contains(errs, "t42（sample-site-s-7）に最初のメッセージが届いていない: trust prompt（App では sample-site / CE S-7 タイトル）。dispatcher が見回りで送り直す") ||
		!strings.Contains(errs, "急ぐときだけ手で: hc send t42 '役目の1文目。\n役目の2文目。\n\n課題 `sample-site/S-7`。") || !strings.Contains(errs, "（App では sample-site / CE S-7 タイトル）") ||
		!strings.HasSuffix(errs, "`。'\n") || // 手で送る行は案内の最後（貼ったときに後ろの文を拾わない）
		!f.has("hc label t42 first_message.sent=false") ||
		!f.has("kanban start S-7 | HC_TERMINAL_ID=t42") || f.has("hc attach") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

// Chief Engineer の最初のメッセージには、目録の Topics にページがあっても wiki の目録の行を足さない（ADR-0061 決めたこと 12・受け入れ条件 7）。
// Chief Engineer の skill がもう wiki を読むと書いている。
func TestFirstMessageHasNoWiki(t *testing.T) {
	f := newFake(t)
	idx := filepath.Join(os.Getenv("ANDON_WORKSPACE"), spawn.WikiIndex)
	if err := os.MkdirAll(filepath.Dir(idx), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idx, []byte("# wiki\n\n## Entities\n\n## Topics\n\n- [topics/a.md](topics/a.md) — a\n\n## Sources\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if spawn.Wiki() == "" {
		t.Fatal("目録の Topics にページがあるのに、担当なら足す行が空（検査が何も見ていない）")
	}
	if rc, _, errs := runMain("--repo", "github.com/example/sample-site", "Billing の設計"); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	all := strings.Join(f.calls(), "\n")
	if !strings.Contains(all, "依頼書は `") {
		t.Fatalf("最初のメッセージが見えない:\n%s", all)
	}
	if strings.Contains(all, idx) {
		t.Errorf("Chief Engineer の最初のメッセージに目録の行がある:\n%s", all)
	}
}
