package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLockIsExclusiveAndReleased(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".flightdeck"), 0o755)

	inside := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error)
	go func() {
		done <- withLock(root, time.Second, func() error {
			close(inside)
			<-release
			return nil
		})
	}()
	<-inside

	// A second open of the same file is a different holder (flock and LockFileEx both work per open file).
	err := withLock(root, 100*time.Millisecond, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "is held") {
		t.Fatalf("second lock while held: got %v, want 'is held'", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ran := false
	if err := withLock(root, time.Second, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("lock after release: ran=%v err=%v", ran, err)
	}
}

func TestLockRefusesOldNodeLockFolder(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".flightdeck", ".lock"), 0o755)
	err := withLock(root, time.Second, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "is a folder") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteAtomicReplaces(t *testing.T) {
	file := filepath.Join(t.TempDir(), "task.yaml")
	for _, s := range []string{"a: 1\n", "a: 2\n"} {
		if err := writeAtomic(file, []byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(file)
	entries, _ := os.ReadDir(filepath.Dir(file))
	if string(got) != "a: 2\n" || len(entries) != 1 {
		t.Fatalf("got %q with %d files", got, len(entries))
	}
}

func edit(t *testing.T, in string, f func(d *doc)) string {
	t.Helper()
	d, err := parseDoc([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	f(d)
	return string(d.encode())
}

func TestSetKeepsCommentsAndOrder(t *testing.T) {
	in := "# top of file\ntitle: Work   # the title\nnotes: mine\n# about status\nstatus: todo  # was todo\nlinks: [a]\n"
	got := edit(t, in, func(d *doc) {
		d.setStr("status", "doing")
		d.setList("links", []string{"a", "b"})
		d.setStr("id", "W-1")
		d.setStr("agent", "t3")
		d.setStr("custom", "x")
	})
	want := "# top of file\nid: W-1\ntitle: Work # the title\nnotes: mine\n# about status\nstatus: doing # was todo\nagent: t3\nlinks: [a, b]\ncustom: x\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDeleteKeepsNeighbourComments(t *testing.T) {
	in := "title: Work\n# about links\nlinks: [a]\nstatus: todo\n"
	got := edit(t, in, func(d *doc) { d.del("links") })
	if got != "title: Work\n# about links\nstatus: todo\n" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestStringsThatLookLikeOtherTypesAreQuoted(t *testing.T) {
	got := edit(t, "title: x\n", func(d *doc) {
		d.setStr("updated", "2026-11-02T10:00:00Z")
		d.setStr("phase", "1")
		d.setStr("title", "a: b")
	})
	d, err := parseDoc([]byte(got))
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"updated": "!!str", "phase": "!!str", "title": "!!str"} {
		if tag := d.get(k).ShortTag(); tag != want {
			t.Errorf("%s reads back as %s in:\n%s", k, tag, got)
		}
	}
}

func TestCRLFAndBOMAreRead(t *testing.T) {
	got := edit(t, "\xef\xbb\xbftitle: Work\r\n# kept\r\nstatus: todo\r\n", func(d *doc) { d.setStr("status", "done") })
	if strings.Contains(got, "\r") || !strings.Contains(got, "# kept") || !strings.Contains(got, "status: done") {
		t.Fatalf("got %q", got)
	}
}

func TestNotAMapping(t *testing.T) {
	if _, err := parseDoc([]byte("- a\n")); err == nil {
		t.Fatal("a list was accepted as a task")
	}
	if d, err := parseDoc(nil); err != nil || len(d.m.Content) != 0 {
		t.Fatalf("empty file: %v", err)
	}
}

// kanban runs a command in-process from cwd.
func kanban(t *testing.T, cwd string, argv ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(argv, cwd, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("kanban %v: exit %d: %s", argv, code, errb.String())
	}
	return out.String()
}

func TestCommandsKeepAPersonsComments(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FLIGHTDECK_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "init", "--root", root)
	kanban(t, root, "add", "First")
	kanban(t, root, "add", "Second", "--after", "W-1")
	file := taskFile(root, "W-2")
	data, _ := os.ReadFile(file)
	os.WriteFile(file, []byte("# by hand\n"+strings.Replace(string(data), "status: queued", "status: queued # waits for W-1", 1)), 0o644)

	kanban(t, root, "set", "W-2", "phase=P1")
	kanban(t, root, "done", "W-1")
	kanban(t, root, "start", "W-2")
	kanban(t, root, "review", "W-2")

	got, _ := os.ReadFile(file)
	for _, s := range []string{"# by hand\nid: W-2\ntitle: Second\nstatus: review # waits for W-1\nphase: P1\nafter: [W-1]\nagent: t3\n"} {
		if !strings.HasPrefix(string(got), s) {
			t.Fatalf("got:\n%s\nwant prefix:\n%s", got, s)
		}
	}
}

// deckFails runs a command that must be refused: exit 1 and one "kanban: " line on stderr.
func deckFails(t *testing.T, cwd, stdin string, argv ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(argv, cwd, strings.NewReader(stdin), &out, &errb)
	msg := errb.String()
	if code != 1 || !strings.HasPrefix(msg, "kanban: ") || strings.Count(msg, "\n") != 1 {
		t.Fatalf("kanban %v: exit %d, stderr %q, stdout %q; want exit 1 and one kanban: line", argv, code, msg, out.String())
	}
	return msg
}

func ledger(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	t.Setenv("FLIGHTDECK_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "")
	kanban(t, root, "init", "--root", root)
	kanban(t, root, "add", "First")
	return root
}

func TestPrefixThatLeavesTasksIsRefused(t *testing.T) {
	root := ledger(t)
	outside := filepath.Join(filepath.Dir(root), "escape")
	for _, prefix := range []string{"../../../escape", `..\..\..\escape`, "a/b", "..", "C:x"} {
		os.WriteFile(filepath.Join(root, ".flightdeck", "config.yaml"), []byte("version: 2\nprefix: '"+prefix+"'\n"), 0o644)
		deckFails(t, root, "", "add", "x")
	}
	matches, _ := filepath.Glob(outside + "*")
	entries, _ := os.ReadDir(tasksDir(root))
	if len(matches) != 0 || len(entries) != 1 {
		t.Fatalf("written outside %v, tasks %d", matches, len(entries))
	}
	deckFails(t, root, "", "init", "--root", filepath.Join(filepath.Dir(root), "other"), "--prefix", "../x")
}

func TestIDThatLeavesTasksIsRefused(t *testing.T) {
	root := ledger(t)
	// A task.yaml one level up from tasks/, which "../" ids would reach.
	sentinel := filepath.Join(root, ".flightdeck", "task.yaml")
	os.WriteFile(sentinel, []byte("title: sentinel\n"), 0o644)
	abs := filepath.Join(root, ".flightdeck")
	for _, id := range []string{"..", "../", "../W-1", `..\W-1`, `W-1\..\..`, abs, "/tmp", "C:W-1"} {
		deckFails(t, root, "", "show", id)
		deckFails(t, root, "", "set", id, "title=x")
		deckFails(t, root, "", "start", id)
		deckFails(t, root, "", "review", id)
		deckFails(t, root, "", "done", id)
		deckFails(t, root, "report", "report", id)
		deckFails(t, root, "report", "report", id, "--append")
	}
	if got, _ := os.ReadFile(sentinel); string(got) != "title: sentinel\n" {
		t.Fatalf("sentinel changed: %q", got)
	}
	if exists(filepath.Join(root, ".flightdeck", "report.md")) {
		t.Fatal("report written outside tasks/")
	}
}

// symlink makes link point at target, or skips where links need privileges (Windows).
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
}

func sentinelFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	os.WriteFile(p, []byte(content), 0o644)
	return p
}

func assertUnchanged(t *testing.T, path, want string) {
	t.Helper()
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("%s changed: %q", path, got)
	}
}

func TestTaskFolderLinkIsRefused(t *testing.T) {
	root := ledger(t)
	outDir := t.TempDir()
	os.WriteFile(filepath.Join(outDir, "task.yaml"), []byte("title: outside\n"), 0o644)
	symlink(t, outDir, filepath.Join(tasksDir(root), "escape"))
	deckFails(t, root, "", "set", "escape", "title=x")
	deckFails(t, root, "", "start", "escape")
	deckFails(t, root, "", "done", "escape")
	deckFails(t, root, "r", "report", "escape")
	assertUnchanged(t, filepath.Join(outDir, "task.yaml"), "title: outside\n")
	if exists(filepath.Join(outDir, "report.md")) {
		t.Fatal("report written through the link")
	}
}

func TestReportLinkIsRefused(t *testing.T) {
	root := ledger(t)
	out := sentinelFile(t, "outside.md", "outside\n")
	symlink(t, out, filepath.Join(taskDir(root, "W-1"), "report.md"))
	deckFails(t, root, "more\n", "report", "W-1", "--append")
	deckFails(t, root, "new\n", "report", "W-1")
	assertUnchanged(t, out, "outside\n")
}

func TestLogLinkIsRefused(t *testing.T) {
	root := ledger(t)
	out := sentinelFile(t, "outside.jsonl", "outside\n")
	logFile := filepath.Join(root, ".flightdeck", "log.jsonl")
	os.Remove(logFile)
	symlink(t, out, logFile)
	before, _ := os.ReadFile(taskFile(root, "W-1"))
	deckFails(t, root, "", "set", "W-1", "title=x")
	deckFails(t, root, "", "add", "y")
	assertUnchanged(t, out, "outside\n")
	assertUnchanged(t, taskFile(root, "W-1"), string(before))
}

func TestTaskFileLinkIsRefused(t *testing.T) {
	root := ledger(t)
	out := sentinelFile(t, "task.yaml", "title: outside\n")
	os.Remove(taskFile(root, "W-1"))
	symlink(t, out, taskFile(root, "W-1"))
	deckFails(t, root, "", "set", "W-1", "title=x")
	assertUnchanged(t, out, "title: outside\n")
}

func TestInitLeavesAnExistingGitignore(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FLIGHTDECK_ROOT", "")
	os.MkdirAll(filepath.Join(root, ".flightdeck"), 0o755)
	mine := filepath.Join(root, ".flightdeck", ".gitignore")
	os.WriteFile(mine, []byte("mine\n"), 0o644)
	kanban(t, root, "init", "--root", root)
	assertUnchanged(t, mine, "mine\n")
}

func TestInitWritesFormat2(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FLIGHTDECK_ROOT", "")
	kanban(t, root, "init", "--root", root, "--prefix", "FD", "--repo", "github.com/x/app", "--repo", "github.com/x/lib")
	cfg, err := readConfig(root)
	if err != nil || cfg.prefix != "FD" || strings.Join(cfg.repos, " ") != "github.com/x/app github.com/x/lib" {
		t.Fatalf("cfg %+v, err %v", cfg, err)
	}
	assertUnchanged(t, filepath.Join(root, ".flightdeck", ".gitignore"), "*\n")
	other := t.TempDir()
	kanban(t, other, "init")
	if cfg, err := readConfig(other); err != nil || len(cfg.repos) != 0 || cfg.prefix != "W" {
		t.Fatalf("init in the current folder: cfg %+v, err %v", cfg, err)
	}
}

func TestOtherFormatVersionsAreRefused(t *testing.T) {
	root := ledger(t)
	cfgFile := filepath.Join(root, ".flightdeck", "config.yaml")
	logFile := filepath.Join(root, ".flightdeck", "log.jsonl")
	before, _ := os.ReadFile(taskFile(root, "W-1"))
	logBefore, _ := os.ReadFile(logFile)
	for _, cfg := range []string{"prefix: W\nprojects:\n  app: projects/app\n", "version: 1\n", "version: 3\n", "version: [2]\n"} {
		os.WriteFile(cfgFile, []byte(cfg), 0o644)
		msg := deckFails(t, root, "", "add", "x")
		deckFails(t, root, "", "set", "W-1", "title=y")
		deckFails(t, root, "", "list")
		if !strings.Contains(msg, "format") {
			t.Errorf("%q: %s", cfg, msg)
		}
	}
	assertUnchanged(t, taskFile(root, "W-1"), string(before))
	assertUnchanged(t, logFile, string(logBefore))
	if entries, _ := os.ReadDir(tasksDir(root)); len(entries) != 1 {
		t.Fatalf("%d tasks", len(entries))
	}
}

func TestRepoMatches(t *testing.T) {
	for _, c := range []struct {
		checkout, repo string
		want           bool
	}{
		{"/w/.ghq/github.com/x/app", "github.com/x/app", true},
		{"/w/.ghq/github.com/x/app", "github.com/x/app/", false},
		{"/w/.ghq/github.com/x/app", "/github.com/x/app", false},
		{"/w/.ghq/github.com/x/app", "x/app", false},
		{"/w/.ghq/github.com/x/app", "a/github.com/x/app", false},
		{"/w/.ghq/github.com/x/app", "github.com//app", false},
		{"/w/.ghq/github.com/x/myapp", "github.com/x/app", false},
		{"/w/.ghq/github.com/x/app", "github.com/x/app2", false},
		{"/w/.ghq/github.com/x/app", "", false},
		{"/w/.ghq/github.com/x/App", "github.com/x/app", runtime.GOOS == "windows"},
	} {
		if got := repoMatches(filepath.FromSlash(c.checkout), c.repo); got != c.want {
			t.Errorf("repoMatches(%s, %s) = %v", c.checkout, c.repo, got)
		}
	}
}

func TestRepoAndProfile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("FLIGHTDECK_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, t.TempDir(), "init", "--root", root, "--repo", "github.com/x/app")
	deckFails(t, root, "", "add", "x", "--repo", "github.com/x/other")
	kanban(t, root, "add", "Work", "--repo", "github.com/x/app", "--profile", "opus", "--phase", "P1")
	kanban(t, root, "start", "W-1")
	got, _ := os.ReadFile(taskFile(root, "W-1"))
	want := "id: W-1\ntitle: Work\nstatus: doing\nrepo: github.com/x/app\nphase: P1\nprofile: opus\nagent: t3\n"
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("got:\n%s\nwant prefix:\n%s", got, want)
	}
	if out := kanban(t, root, "list", "--repo", "github.com/x/app"); !strings.Contains(out, "W-1") {
		t.Fatalf("list --repo: %q", out)
	}
	if out := kanban(t, root, "ready", "--repo", "github.com/x/app"); strings.Contains(out, "W-1") {
		t.Fatalf("ready --repo shows a doing task: %q", out)
	}
	kanban(t, root, "set", "W-1", "repo=github.com/x/gone", "profile=codex")
	var out, errb bytes.Buffer
	if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(out.String(), "W-1: repo github.com/x/gone is not in config.yaml repos") {
		t.Fatalf("lint: exit %d, %q %q", code, out.String(), errb.String())
	}
}

// v1 and unknown versions are refused by every command, before anything is read or made.
func TestOtherVersionsAreRefusedByEveryCommand(t *testing.T) {
	for _, cfg := range []string{"prefix: W\nprojects: {}\n", "version: 1\n", "version: 3\n"} {
		root := t.TempDir()
		t.Setenv("FLIGHTDECK_ROOT", root)
		fd := filepath.Join(root, ".flightdeck")
		os.MkdirAll(filepath.Join(fd, "tasks", "W-1"), 0o755)
		os.WriteFile(filepath.Join(fd, "config.yaml"), []byte(cfg), 0o644)
		os.WriteFile(filepath.Join(fd, "tasks", "W-1", "task.yaml"), []byte("id: W-1\ntitle: old\nstatus: todo\n"), 0o644)
		deckFails(t, root, "text\n", "report", "W-1")
		deckFails(t, root, "text\n", "report", "W-1", "--append")
		deckFails(t, root, "", "root")
		deckFails(t, root, "", "show", "W-1")
		deckFails(t, root, "", "init", "--root", root)
		deckFails(t, root, "", "config", "set", "coordinator=t9")
		assertUnchanged(t, filepath.Join(fd, "config.yaml"), cfg)
		for _, made := range []string{"log.jsonl", ".lock", ".gitignore", filepath.Join("tasks", "W-1", "report.md")} {
			if exists(filepath.Join(fd, made)) {
				t.Errorf("%q: %s was made", cfg, made)
			}
		}
		var out, errb bytes.Buffer
		if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(out.String(), "config.yaml: ") || !strings.Contains(out.String(), "format") {
			t.Errorf("%q: lint exit %d, %q %q", cfg, code, out.String(), errb.String())
		}

		// init on a folder holding only such a config.yaml makes neither tasks/ nor .gitignore.
		bare := t.TempDir()
		os.MkdirAll(filepath.Join(bare, ".flightdeck"), 0o755)
		os.WriteFile(filepath.Join(bare, ".flightdeck", "config.yaml"), []byte(cfg), 0o644)
		deckFails(t, bare, "", "init", "--root", bare)
		if entries, _ := os.ReadDir(filepath.Join(bare, ".flightdeck")); len(entries) != 1 {
			t.Errorf("%q: init made %d entries", cfg, len(entries))
		}
	}
}

// repos entries must be exactly host/owner/repo: init and --repo refuse others, lint reports them.
func TestReposMustBeHostOwnerRepo(t *testing.T) {
	t.Setenv("FLIGHTDECK_ROOT", "")
	for _, bad := range []string{"github.com/x/app/", "/github.com/x/app", "x/app", "github.com//app"} {
		dir := filepath.Join(t.TempDir(), "p")
		deckFails(t, t.TempDir(), "", "init", "--root", dir, "--repo", bad)
		if exists(dir) {
			t.Errorf("init --repo %q made %s", bad, dir)
		}
	}
	root := filepath.Join(t.TempDir(), "root")
	kanban(t, t.TempDir(), "init", "--root", root, "--repo", "github.com/x/app")
	t.Setenv("FLIGHTDECK_ROOT", root)
	os.WriteFile(filepath.Join(root, ".flightdeck", "config.yaml"), []byte("version: 2\nrepos: [github.com/x/app, github.com/x/lib/, x/app]\n"), 0o644)
	deckFails(t, root, "", "add", "x", "--repo", "github.com/x/lib/")
	deckFails(t, root, "", "add", "x", "--repo", "x/app")
	kanban(t, root, "add", "ok", "--repo", "github.com/x/app")
	var out, errb bytes.Buffer
	run([]string{"lint"}, root, strings.NewReader(""), &out, &errb)
	for _, want := range []string{`repos entry "github.com/x/lib/"`, `repos entry "x/app"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("lint doesn't report %s: %q", want, out.String())
		}
	}
	if strings.Contains(out.String(), `"github.com/x/app"`) {
		t.Errorf("lint reports a good entry: %q", out.String())
	}
}

// who goes between profile and agent (keyOrder), wherever the other keys are.
func TestWhoGoesBetweenProfileAndAgent(t *testing.T) {
	root := ledger(t)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "set", "W-1", "profile=opus")
	kanban(t, root, "start", "W-1")
	kanban(t, root, "set", "W-1", "who=person")
	got, _ := os.ReadFile(taskFile(root, "W-1"))
	if !strings.Contains(string(got), "profile: opus\nwho: person\nagent: t3\n") {
		t.Fatalf("got:\n%s", got)
	}
	kanban(t, root, "set", "W-1", "who=")
	if got, _ := os.ReadFile(taskFile(root, "W-1")); strings.Contains(string(got), "who") {
		t.Fatalf("who= left:\n%s", got)
	}
}

// kanban config set coordinator= rewrites config.yaml under the lock, keeping comments and order, and logs it.
func TestConfigSetCoordinator(t *testing.T) {
	root := ledger(t)
	file := filepath.Join(root, ".flightdeck", "config.yaml")
	os.WriteFile(file, []byte("# by hand\nversion: 2\ntitle: App # shown\ncoordinator: t1 # the lead\nprefix: W\nmine: kept\n"), 0o644)
	if out := kanban(t, root, "config", "set", "coordinator=t7"); out != "coordinator: t7\n" {
		t.Fatalf("out %q", out)
	}
	assertUnchanged(t, file, "# by hand\nversion: 2\ntitle: App # shown\ncoordinator: t7 # the lead\nprefix: W\nmine: kept\n")
	kanban(t, root, "config", "set", "coordinator=")
	assertUnchanged(t, file, "# by hand\nversion: 2\ntitle: App # shown\nprefix: W\nmine: kept\n")
	if out := kanban(t, root, "config", "set", "coordinator=t8", "--json"); !strings.Contains(out, `"coordinator": "t8"`) {
		t.Fatalf("json %q", out)
	}
	assertUnchanged(t, file, "# by hand\nversion: 2\ntitle: App # shown\nprefix: W\nmine: kept\ncoordinator: t8\n")
	if cfg, err := readConfig(root); err != nil || cfg.prefix != "W" {
		t.Fatalf("cfg %+v, err %v", cfg, err)
	}
	log, _ := os.ReadFile(filepath.Join(root, ".flightdeck", "log.jsonl"))
	for _, want := range []string{`"config":{"coordinator":"t7"}`, `"config":{"coordinator":null}`, `"config":{"coordinator":"t8"}`} {
		if !strings.Contains(string(log), want) {
			t.Errorf("log has no %s:\n%s", want, log)
		}
	}
	for _, bad := range [][]string{{"config"}, {"config", "set"}, {"config", "set", "prefix=X"}, {"config", "set", "coordinator"}, {"config", "get", "coordinator=t1"}} {
		deckFails(t, root, "", bad...)
	}
	assertUnchanged(t, file, "# by hand\nversion: 2\ntitle: App # shown\nprefix: W\nmine: kept\ncoordinator: t8\n")
}

func TestConfigLinkIsRefused(t *testing.T) {
	root := ledger(t)
	file := filepath.Join(root, ".flightdeck", "config.yaml")
	real := sentinelFile(t, "config.yaml", "version: 2\n")
	os.Remove(file)
	symlink(t, real, file)
	deckFails(t, root, "", "config", "set", "coordinator=t7")
	assertUnchanged(t, real, "version: 2\n")
}
