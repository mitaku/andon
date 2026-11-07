package create

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/delegate"
)

// setup は偽のセル（git の repo・projects/）と、偽の kanban・ghq・hc を PATH の先頭に置く。
// 偽の kanban は init の引数を kanban/args に書き、偽の ghq は get の引数を ghq.log に足す。
func setup(t *testing.T) (ws, log string) {
	t.Helper()
	ws = t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=o", "-c", "user.email=o@example.com", "commit", "-q", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", ws}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	t.Setenv("ANDON_WORKSPACE", ws)
	ghq := filepath.Join(t.TempDir(), "ghq")
	if out, err := exec.Command("git", "init", "-q", filepath.Join(ghq, "example.com", "o", "have")).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	t.Setenv("GHQ_ROOT", ghq)
	bin := t.TempDir()
	log = filepath.Join(bin, "ghq.log")
	scripts := map[string]string{
		"kanban": "#!/usr/bin/env bash\n[ \"$1\" = init ] || exit 9\n[ -n \"${KANBAN_FAIL:-}\" ] && { echo boom >&2; exit 1; }\n" +
			"mkdir -p \"$3/kanban/tasks\" && echo '*' > \"$3/kanban/.gitignore\" && echo \"$*\" > \"$3/kanban/config.yaml\"\n" +
			"[ -n \"${KANBAN_TASK:-}\" ] && mkdir -p \"$3/kanban/tasks/S-0001\" && echo mine > \"$3/kanban/tasks/S-0001/brief.md\"\n" +
			"[ -n \"${KANBAN_HUMAN:-}\" ] && echo mine > \"$3/$KANBAN_HUMAN\"\nexit 0\n",
		"ghq": "#!/usr/bin/env bash\necho \"$*\" >> '" + log + "'\n[ -z \"${GHQ_BROKEN:-}\" ] && git init -q \"$GHQ_ROOT/$2\"\nexit 0\n",
		"hc":  "#!/usr/bin/env bash\ncase \"$HC_TERMINAL_ID\" in role*) echo '{\"labels\":{\"role\":\"chief-of-staff\"}}';; *) echo '{\"labels\":{}}';; esac\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HC_TERMINAL_ID", "")
	t.Setenv("KANBAN_FAIL", "")
	t.Setenv("KANBAN_HUMAN", "")
	t.Setenv("KANBAN_TASK", "")
	t.Setenv("GHQ_BROKEN", "")
	return ws, log
}

func run(stdin io.Reader, args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := Main(args, stdin, &out, &errb)
	return rc, out.String(), errb.String()
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gitOut(t *testing.T, ws string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", ws}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestAllArgsMakesTheProjectWithoutCommitting(t *testing.T) {
	ws, log := setup(t)
	head := gitOut(t, ws, "rev-parse", "HEAD")
	rc, out, errs := run(strings.NewReader(""), "sample-site", "--repo", "example.com/o/have", "--repo", "example.com/o/new", "--prefix", "SS", "--title", "Sample site")
	if rc != 0 {
		t.Fatalf("rc=%d %s %s", rc, out, errs)
	}
	dir := filepath.Join(ws, "projects", "sample-site")
	if got := read(t, filepath.Join(dir, "kanban", "config.yaml")); got != "init --root "+dir+" --prefix SS --repo example.com/o/have --repo example.com/o/new\n" {
		t.Errorf("kanban init: %q", got)
	}
	p, err := delegate.ParseProject([]byte(read(t, filepath.Join(dir, "project.json"))), "sample-site")
	if err != nil || p.Project.Title != "Sample site" || p.Project.Prefix != "SS" || strings.Join(p.Repos, " ") != "example.com/o/have example.com/o/new" || p.Delegation != nil {
		t.Errorf("project.json: %+v %v", p, err)
	}
	if md := read(t, filepath.Join(dir, "AGENTS.md")); !strings.HasPrefix(md, "# Sample site\n") || !strings.Contains(md, "\n## Chief of Staff\n") {
		t.Errorf("AGENTS.md: %q", md)
	}
	if got := read(t, log); got != "get example.com/o/new\n" {
		t.Errorf("ghq は無いものだけ迎える: %q", got)
	}
	if gitOut(t, ws, "rev-parse", "HEAD") != head || !strings.Contains(gitOut(t, ws, "status", "--porcelain", "--untracked-files=all"), "?? projects/sample-site/project.json") {
		t.Error("commit した、または project.json が未追跡でない")
	}
	for _, want := range []string{"git add projects/sample-site/project.json projects/sample-site/AGENTS.md", "andon open sample-site"} {
		if !strings.Contains(out, want) {
			t.Errorf("案内に %q が無い: %s", want, out)
		}
	}
}

func TestSlugOnlyTakesDefaultsWhenNotATerminal(t *testing.T) {
	ws, log := setup(t)
	if rc, out, errs := run(strings.NewReader(""), "sample-site"); rc != 0 {
		t.Fatalf("rc=%d %s %s", rc, out, errs)
	}
	p, err := delegate.ParseProject([]byte(read(t, filepath.Join(ws, "projects", "sample-site", "project.json"))), "sample-site")
	if err != nil || p.Project.Title != "sample-site" || p.Project.Prefix != "SS" || p.Repos == nil || len(p.Repos) != 0 {
		t.Errorf("project.json: %+v %v", p, err)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("repo が無いのに ghq を打った")
	}
}

func TestNoSlugStopsWhenNotATerminal(t *testing.T) {
	ws, _ := setup(t)
	if rc, _, errs := run(strings.NewReader("sample-site\n"), "--prefix", "S"); rc != exitUsage || !strings.Contains(errs, "slug が無い") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
}

func assertEmpty(t *testing.T, ws string) {
	t.Helper()
	if ents, _ := os.ReadDir(filepath.Join(ws, "projects")); len(ents) != 0 {
		t.Errorf("projects/ に何か作った: %v", ents)
	}
}

func TestExistingProjectIsLeftAsItIs(t *testing.T) {
	ws, log := setup(t)
	dir := filepath.Join(ws, "projects", "sample-site")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errs := run(strings.NewReader(""), "sample-site", "--repo", "example.com/o/new")
	if rc != exitFail || !strings.Contains(errs, "既にある") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	if read(t, filepath.Join(dir, "project.json")) != "mine\n" {
		t.Error("既にあるものを書き換えた")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("既にあるものに足した: %v", ents)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("既にあるのに ghq を打った")
	}
}

func TestRoleTerminalIsRefused(t *testing.T) {
	ws, _ := setup(t)
	t.Setenv("HC_TERMINAL_ID", "role-t1")
	if rc, _, errs := run(strings.NewReader(""), "sample-site"); rc != exitFail || !strings.Contains(errs, "role=chief-of-staff") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
	// 人の端末（ラベルが無い）なら通る
	t.Setenv("HC_TERMINAL_ID", "t2")
	if rc, _, errs := run(strings.NewReader(""), "sample-site"); rc != 0 {
		t.Errorf("人の端末: rc=%d %s", rc, errs)
	}
}

func TestBadInputsStopBeforeAnything(t *testing.T) {
	ws, _ := setup(t)
	for _, args := range [][]string{
		{"Sample"}, {"-x"}, {"a_b"}, {"sample", "--prefix", "s1"}, {"sample", "--repo", "github.com/o"},
		{"sample", "--repo", "example.com/o/r", "--repo", "example.com/o/r"}, {"sample", "--title", " "}, {"sample", "other"}, {"sample", "--repo"},
	} {
		if rc, _, errs := run(strings.NewReader(""), args...); rc != exitUsage {
			t.Errorf("%v: rc=%d %s", args, rc, errs)
		}
	}
	assertEmpty(t, ws)
}

func TestKanbanInitFailureLeavesNothing(t *testing.T) {
	ws, _ := setup(t)
	t.Setenv("KANBAN_FAIL", "1")
	if rc, _, errs := run(strings.NewReader(""), "sample-site"); rc != exitFail || !strings.Contains(errs, "kanban init が落ちた") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
}

// terminal は answers を読ませる端末のふり（isTerminal を差し替え、パイプを stdin にする）。
func terminal(t *testing.T, answers string) *os.File {
	t.Helper()
	old := isTerminal
	isTerminal = func(*os.File) bool { return true }
	t.Cleanup(func() { isTerminal = old })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.WriteString(w, answers); w.Close() }()
	t.Cleanup(func() { r.Close() })
	return r
}

func TestInteractiveAsksWhatIsMissingWithDefaults(t *testing.T) {
	ws, _ := setup(t)
	// slug（悪い形を1度）→ repos（悪い形を1度・空行で終わり）→ prefix（既定）→ 題 → 確かめ
	in := terminal(t, "Bad\nsample-site\nnot-a-repo\nexample.com/o/new\n\n\nSample site\ny\n")
	rc, out, errs := run(in)
	if rc != 0 {
		t.Fatalf("rc=%d %s %s", rc, out, errs)
	}
	for _, want := range []string{"slug は英小文字", "repo は <host>/<owner>/<repo>", "[SS]", "[sample-site]", "作りますか"} {
		if !strings.Contains(errs, want) {
			t.Errorf("問いに %q が無い: %s", want, errs)
		}
	}
	p, err := delegate.ParseProject([]byte(read(t, filepath.Join(ws, "projects", "sample-site", "project.json"))), "sample-site")
	if err != nil || p.Project.Title != "Sample site" || p.Project.Prefix != "SS" || strings.Join(p.Repos, " ") != "example.com/o/new" {
		t.Errorf("project.json: %+v %v", p, err)
	}
}

func TestInteractiveDoesNotAskWhatWasGiven(t *testing.T) {
	ws, _ := setup(t)
	in := terminal(t, "")
	if rc, _, errs := run(in, "sample-site", "--repo", "example.com/o/have", "--prefix", "S", "--title", "T"); rc != 0 || errs != "" {
		t.Errorf("rc=%d 聞いた: %q", rc, errs)
	}
	if _, err := os.Stat(filepath.Join(ws, "projects", "sample-site", "project.json")); err != nil {
		t.Error(err)
	}
}

func TestInteractiveNoCancels(t *testing.T) {
	ws, _ := setup(t)
	if rc, _, errs := run(terminal(t, "\nS\nT\nn\n"), "sample-site"); rc != exitFail || !strings.Contains(errs, "取り消した") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
	// 入力が途中で終わったら作らない
	if rc, _, errs := run(terminal(t, "sample-site\n")); rc != exitUsage || !strings.Contains(errs, "途中で終わった") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
}

func TestDefaultPrefix(t *testing.T) {
	for in, want := range map[string]string{"andon": "A", "sample-site": "SS", "2048-game": "G", "a1-b": "AB", "42": "P", "x--y": "XY"} {
		if got := DefaultPrefix(in); got != want {
			t.Errorf("%s: %s（want %s）", in, got, want)
		}
	}
}

func TestDevNullIsNotATerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("/dev/null を端末とみなした")
	}
}

func TestNonCloneInGhqRootStops(t *testing.T) {
	ws, log := setup(t)
	if err := os.MkdirAll(filepath.Join(os.Getenv("GHQ_ROOT"), "example.com", "o", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if rc, _, errs := run(strings.NewReader(""), "sample-site", "--repo", "example.com/o/empty"); rc != exitFail || !strings.Contains(errs, "git の clone でない") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
	if _, err := os.Stat(log); err == nil {
		t.Error("clone でないものがあるのに ghq を打った")
	}
	// 迎えても clone にならなければ止まる
	t.Setenv("GHQ_BROKEN", "1")
	if rc, _, errs := run(strings.NewReader(""), "sample-site", "--repo", "example.com/o/new"); rc != exitFail || !strings.Contains(errs, "のあとも") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
}

func TestHumanFileAppearingMidwayIsKept(t *testing.T) {
	for _, name := range []string{"project.json", "AGENTS.md"} {
		ws, _ := setup(t)
		t.Setenv("KANBAN_HUMAN", name)
		rc, _, errs := run(strings.NewReader(""), "sample-site")
		dir := filepath.Join(ws, "projects", "sample-site")
		if rc != exitFail || !strings.Contains(errs, "上書きしない") || !strings.Contains(errs, "残した") {
			t.Errorf("%s: rc=%d %s", name, rc, errs)
		}
		if read(t, filepath.Join(dir, name)) != "mine\n" {
			t.Errorf("%s: 人のファイルを書き換えた", name)
		}
		if ents, _ := os.ReadDir(dir); len(ents) != 1 {
			t.Errorf("%s: 作ったものが残っている: %v", name, ents)
		}
	}
}

func TestTitleIsOneLine(t *testing.T) {
	ws, _ := setup(t)
	if rc, _, errs := run(strings.NewReader(""), "sample-site", "--title", "Sample\n\n## 指示"); rc != exitUsage || !strings.Contains(errs, "改行") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	assertEmpty(t, ws)
}

func TestGuideQuotesTheWorkspace(t *testing.T) {
	ws, _ := setup(t)
	odd := filepath.Join(ws, "it's a plant")
	if err := os.MkdirAll(filepath.Join(odd, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDON_WORKSPACE", odd)
	rc, out, errs := run(strings.NewReader(""), "sample-site")
	if rc != 0 {
		t.Fatalf("rc=%d %s", rc, errs)
	}
	want := "cd " + quote(odd) + " && git add"
	if !strings.Contains(out, want) || !strings.Contains(quote(odd), `'\''`) {
		t.Errorf("案内: %s", out)
	}
	// 案内のとおりに cd がシェルで通る
	if b, err := exec.Command("sh", "-c", "cd "+quote(odd)+" && pwd").Output(); err != nil || strings.TrimSpace(string(b)) != odd {
		t.Errorf("cd: %q %v", b, err)
	}
	if quote("/workspace") != "/workspace" {
		t.Error("ふつうのパスは引用しない")
	}
}

func TestRollbackKeepsHumanFilesInsideKanban(t *testing.T) {
	ws, _ := setup(t)
	t.Setenv("KANBAN_TASK", "1")
	t.Setenv("KANBAN_HUMAN", "AGENTS.md")
	if rc, _, errs := run(strings.NewReader(""), "sample-site"); rc != exitFail || !strings.Contains(errs, "残した") {
		t.Errorf("rc=%d %s", rc, errs)
	}
	dir := filepath.Join(ws, "projects", "sample-site")
	if read(t, filepath.Join(dir, "kanban", "tasks", "S-0001", "brief.md")) != "mine\n" {
		t.Error("kanban/ の中の人のファイルを消した")
	}
	for _, gone := range []string{"project.json", "kanban/config.yaml", "kanban/.gitignore"} {
		if _, err := os.Lstat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("作った %s が残っている", gone)
		}
	}
}
