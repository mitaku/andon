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

// fake は偽の kanban と hc を PATH の先頭に置く。打たれた引数は $LOG に1行ずつ（コマンド名 引数… と、HC_TERMINAL_ID）。
// 振る舞いは環境変数で変える: FAKE_LS（hc ls の返事）・FAKE_GET（hc get。無ければ Claude Code が動いている）・FAKE_TPL（hc templates）・FAKE_RUN（hc run）・FAKE_RUN_RC。
type fake struct {
	t       *testing.T
	root    string // 台帳の root（projects/sample-site）
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

func newFake(t *testing.T) *fake {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"kanban": fakeDeck, "hc": fakeHc} {
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
	f.config("repos:\n  - github.com/example/sample-site\n")
	t.Setenv("HOME", f.home)
	// 信頼の設定（boot.TrustFromOS）は一時の HOME に書く。本物の CLAUDE_CONFIG_DIR・セルの repo に触らない
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("ANDON_WORKSPACE", dir) // projects/sample-site/kanban の台帳を読む
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

// config は台帳の config.yaml に、台帳の CLI が書く形で repos を置く。
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
	if !strings.Contains(out, "S-7（Billing の設計）を t42（sample-site-s-7）で起こした。cwd "+f.wt()+" branch sample-site/S-7-billing") {
		t.Errorf("out=%q", out)
	}
	// 最後に出た場所を「<Space 名> / <tab 名>」で出す。hc attach は打たない（ADR-0002 決めたこと 14・16・17）
	if !strings.HasSuffix(out, "andon chief-engineer: 出た場所 sample-site / CE S-7 Billing の設計（t42）\n") || strings.Contains(out, "hc attach") || f.has("hc attach") {
		t.Errorf("出た場所が最後に無い・attach が残る: out=%q", out)
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
		"hc ls --json --label role=chief-engineer --label project=sample-site",
		"hc templates --json",
		"kanban add Billing の設計 --repo github.com/example/sample-site --profile chief-engineer --brief - | HC_TERMINAL_ID=t1 KANBAN_ROOT=" + f.root,
		"kanban set S-7 branch=sample-site/S-7-billing worktree=" + f.wt() + " |",
		"hc run --json --bg --outlive-parent --no-notify --name sample-site-s-7 --label role=chief-engineer --label project=sample-site --label issue=S-7 --label title=Billing の設計 --label herdr.space=andon-sample-site --label herdr.tab=andon-sample-site-S-7 --cwd " + f.wt() + " chief-engineer 課題 `sample-site/S-7`。",
		"依頼書は `" + f.root + "/kanban/issues/S-7/brief.md`。",
		"kanban start S-7 | HC_TERMINAL_ID=t42 KANBAN_ROOT=" + f.root,
		"start-cwd " + f.wt(),
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
}

func TestUsage(t *testing.T) {
	newFake(t)
	r := "--repo=github.com/example/sample-site"
	for _, args := range [][]string{nil, {""}, {r}, {r, ""}, {r, "  "}, {r, "a", "b"}, {r, "二行\nの題"}, {r, "--no-attach", "題"}, {"--repo"}} {
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

// 文書（design.md・skill・README）に書いた andon chief-engineer の呼び出しを、そのまま打って通る
func TestDocumentedInvocations(t *testing.T) {
	re := regexp.MustCompile("andon chief-engineer((?: (?:\\[--repo <[^>]+>\\]|--repo <[^>]+>))?)( --brief -)? \"<題>\"")
	files, _ := filepath.Glob("../../package/skills/*/SKILL.md")
	files = append(files, "../../docs/design.md", "../../README.md")
	found := 0
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			found++
			args := []string{"題"}
			if strings.Contains(m[1], "--repo") && !strings.Contains(m[1], "[") {
				args = []string{"--repo", "github.com/example/sample-site", "題"}
			}
			in := ""
			if m[2] != "" {
				args = append([]string{"--brief", "-"}, args...)
				in = "経緯: 人の言葉の写し\n"
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
		t.Errorf("文書に andon chief-engineer の呼び出しが %d 個しか見つからない", found)
	}
	// 呼び出しの形が上の正規表現に合わない書き方が文書に残っていない
	loose := regexp.MustCompile("andon chief-engineer [^`）]*")
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
		{"repos:\n  - github.com/example/a\n  - github.com/example/b\n", exitUsage, "台帳の repos が 2 個（github.com/example/a・github.com/example/b）。--repo"},
		{"repos: []\n", exitUsage, "台帳の repos が 0 個"},
		{"", exitUsage, "repos が読めない"},
		{"repos: [github.com/example/sample-site]\n", exitUsage, "repos が読めない"},
	} {
		f := newFake(t)
		f.config(c.repos)
		rc, _, errs := runMain("題")
		if rc != c.rc || !strings.Contains(errs, c.err) || f.has("kanban add") {
			t.Errorf("%q: rc=%d err=%q", c.repos, rc, errs)
		}
	}
	// 1つなら --repo 無しで通り、--repo があれば repos より --repo
	f := newFake(t)
	f.config("repos:\n  - \"github.com/example/sample-site\"\n")
	if rc, _, errs := runMain("題"); rc != 0 || !f.has("kanban add 題 --repo github.com/example/sample-site ") {
		t.Errorf("1つ: rc=%d err=%q", rc, errs)
	}
	f = newFake(t)
	f.config("repos:\n  - github.com/example/a\n  - github.com/example/b\n")
	if rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題"); rc != 0 {
		t.Errorf("--repo があれば repos が2つでも通る: rc=%d err=%q", rc, errs)
	}
}

func TestBadRepo(t *testing.T) {
	f := newFake(t)
	rc, _, errs := runMain("--repo", "example/sample-site", "題")
	if rc != exitUsage || !strings.Contains(errs, "repo が不正") || f.has("kanban") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
	rc, _, errs = runMain("--repo", "github.com/example/none", "題")
	if rc != exitFail || !strings.Contains(errs, "リポジトリが無い") || f.has("kanban add") {
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
	if rc != exitFail || !strings.Contains(errs, "slug が不正") || f.has("kanban add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

// Chief Engineer は1プロジェクトに同時に複数いてよい（ADR-0009 決めたこと 3）。動いている端末があっても起こし、その端末には触らない。
func TestStartsWhileAnotherRunning(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-3","title":"Billing の設計"}}]}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != 0 || errs != "" || !strings.Contains(out, "S-7（題）を t42") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if !f.has("kanban add") || !f.has("hc run") || f.has("hc keep") || f.has("hc kill") || f.has("hc rm") {
		t.Errorf("起こしていない・動いている端末に触った: %q", f.calls())
	}
}

// 作り直しで中身が素のシェルに替わって戻った Chief Engineer の端末は片付けてから起こす。
func TestShellChiefEngineerIsReplaced(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-3","title":"Billing の設計"}}]}`)
	t.Setenv("FAKE_GET", `{"address":"t9","argv":["/bin/bash"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != 0 || errs != "" || !strings.Contains(out, "sample-site / CE S-3 Billing の設計（t9）は中身がシェルに替わっていた") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	for _, c := range []string{"hc keep t9 --off", "hc kill t9", "hc rm t9", "kanban add", "hc run"} {
		if !f.has(c) {
			t.Errorf("%s を打っていない: %q", c, f.calls())
		}
	}
}

// 動いている端末と戻った端末が並んでいれば、戻った端末だけ片付けて起こす。
func TestRunningKeptShellReplaced(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_LS", `{"terminals":[{"address":"t8","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-2","title":"A"}},{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-3","title":"B"}}]}`)
	t.Setenv("FAKE_GET2", `{"address":"t9","argv":["/bin/bash"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != 0 || errs != "" || !strings.Contains(out, "CE S-3 B（t9）は中身がシェルに替わっていた") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
	if !f.has("hc kill t9") || f.has("hc kill t8") || f.has("hc keep t8") || !f.has("hc run") {
		t.Errorf("calls=%q", f.calls())
	}
}

// 人がシェルで使っている・agent が Claude Code でない端末は、片付けない（起こすのは止めない）。
func TestUnsureChiefEngineerKept(t *testing.T) {
	for _, get := range []string{
		`{"argv":["sh","-c","claude"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}}`,
		`{"argv":["/bin/bash"],"status":null,"foreground":{"name":"vim","argv":["vim"]}}`,
		`{"argv":["sh","-c","claude"],"status":null,"foreground":{"name":"vim","argv":["/usr/bin/vim"]}}`,
	} {
		f := newFake(t)
		t.Setenv("FAKE_LS", `{"terminals":[{"address":"t9","labels":{"role":"chief-engineer","project":"sample-site","issue":"S-3","title":"Billing の設計"}}]}`)
		t.Setenv("FAKE_GET", get)
		rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
		if rc != 0 || errs != "" {
			t.Errorf("%s: rc=%d err=%q", get, rc, errs)
		}
		if f.has("hc keep") || f.has("hc kill") || f.has("hc rm") || !f.has("kanban add") {
			t.Errorf("%s: 人の端末に触った・起こしていない: %q", get, f.calls())
		}
	}
}

// --brief - は stdin の文（経緯）を依頼書の既定の行の下に足す。省けば既定の行だけ。
func TestBriefFromStdin(t *testing.T) {
	head := "# 題\n\nChief Engineer の課題。人と直接話して仕様・設計を詰め、文書にして Chief of Staff へ渡す。\n" +
		"題は人の1行。手順は skill chief-engineer。\n"
	for _, args := range [][]string{{"--brief", "-", "題"}, {"題", "--brief=-"}} {
		f := newFake(t)
		var out, errb bytes.Buffer
		in := "## 経緯\r\n人: 「請求を月ごとにしたい」\n出典: docs/billing.md\n\n"
		rc := Main(append([]string{"--repo", "github.com/example/sample-site"}, args...), strings.NewReader(in), &out, &errb)
		if rc != 0 {
			t.Fatalf("%q: rc=%d err=%q", args, rc, errb.String())
		}
		brief, _ := os.ReadFile(f.log + ".brief")
		want := head + "\n## 経緯\n人: 「請求を月ごとにしたい」\n出典: docs/billing.md\n"
		if string(brief) != want {
			t.Errorf("%q: brief=%q\nwant %q", args, brief, want)
		}
	}
	f := newFake(t)
	var out, errb bytes.Buffer
	if rc := Main([]string{"--repo", "github.com/example/sample-site", "題"}, strings.NewReader("読まない"), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d err=%q", rc, errb.String())
	}
	if brief, _ := os.ReadFile(f.log + ".brief"); string(brief) != head {
		t.Errorf("--brief 無しで依頼書が変わった: %q", brief)
	}
}

// --brief - の stdin が空なら、何もせずに usage の終了コード。stdin のほかの値は受けない。
func TestBriefBad(t *testing.T) {
	f := newFake(t)
	var out, errb bytes.Buffer
	rc := Main([]string{"--repo", "github.com/example/sample-site", "--brief", "-", "題"}, strings.NewReader(" \n"), &out, &errb)
	if rc != exitUsage || !strings.Contains(errb.String(), "--brief - の stdin が空") || f.has("hc") || f.has("kanban") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errb.String(), f.calls())
	}
	r := "--repo=github.com/example/sample-site"
	for _, args := range [][]string{{r, "--brief", "題"}, {r, "--brief", "x.md", "題"}, {r, "--brief=x", "題"}, {r, "--brief", "-", "--brief", "-", "題"}} {
		if rc, _, errs := runMain(args...); rc != exitUsage || !strings.HasPrefix(errs, "usage: andon chief-engineer") {
			t.Errorf("%q: rc=%d err=%q", args, rc, errs)
		}
	}
}

func TestNoTemplate(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"}]}`)
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "プロファイル chief-engineer の template が無い") || f.has("kanban add") {
		t.Errorf("rc=%d err=%q", rc, errs)
	}
}

func TestHcRunFails(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN_RC", "1")
	rc, _, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFail || !strings.Contains(errs, "hc run に失敗") || !strings.Contains(errs, "S-7 は todo のまま残る") || f.has("kanban start") {
		t.Errorf("rc=%d err=%q calls=%q", rc, errs, f.calls())
	}
	// やり直せるように worktree と branch を戻し、台帳の欄も消す
	if _, err := os.Stat(f.wt()); err == nil || strings.Contains(f.branches(), "S-7") || !f.has("kanban set S-7 branch= worktree=") {
		t.Errorf("戻していない: branches=%q calls=%q", f.branches(), f.calls())
	}
}

func TestFirstMessageNotSent(t *testing.T) {
	f := newFake(t)
	t.Setenv("FAKE_RUN", `{"address":"t42","first_message":{"sent":false,"reason":"trust prompt"}}`)
	rc, out, errs := runMain("--repo", "github.com/example/sample-site", "題")
	if rc != exitFirst || out != "andon chief-engineer: 出た場所 sample-site / CE S-7 題（t42）\n" ||
		!strings.Contains(errs, "t42（sample-site-s-7）に最初のメッセージが届いていない: trust prompt。dispatcher が見回りで送り直す") ||
		!strings.Contains(errs, "急ぐときだけ手で: hc send t42 \"課題 `sample-site/S-7`。") || !strings.Contains(errs, "（App では sample-site / CE S-7 題）") ||
		!f.has("hc label t42 first_message.sent=false") ||
		!f.has("kanban start S-7 | HC_TERMINAL_ID=t42") || f.has("hc attach") {
		t.Errorf("rc=%d out=%q err=%q", rc, out, errs)
	}
}
