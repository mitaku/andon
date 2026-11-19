package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLockIsExclusiveAndReleased(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "kanban"), 0o755)

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
	os.MkdirAll(filepath.Join(root, "kanban", ".lock"), 0o755)
	err := withLock(root, time.Second, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "is a folder") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteAtomicReplaces(t *testing.T) {
	file := filepath.Join(t.TempDir(), "issue.yaml")
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
		d.setStr("id", "W-0001")
		d.setStr("agent", "t3")
		d.setStr("custom", "x")
	})
	want := "# top of file\nid: W-0001\ntitle: Work # the title\nnotes: mine\n# about status\nstatus: doing # was todo\nagent: t3\nlinks: [a, b]\ncustom: x\n"
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
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "init", "--root", root)
	kanban(t, root, "add", "First")
	kanban(t, root, "add", "Second", "--after", "W-0001")
	file := taskFile(root, "W-0002")
	data, _ := os.ReadFile(file)
	os.WriteFile(file, []byte("# by hand\n"+strings.Replace(string(data), "status: queued", "status: queued # waits for W-0001", 1)), 0o644)

	kanban(t, root, "set", "W-0002", "phase=P1")
	kanban(t, root, "done", "W-0001")
	kanban(t, root, "start", "W-0002")
	kanban(t, root, "review", "W-0002")

	got, _ := os.ReadFile(file)
	for _, s := range []string{"# by hand\nid: W-0002\ntitle: Second\nstatus: review # waits for W-0001\nphase: P1\nafter: [W-0001]\nagent: t3\n"} {
		if !strings.HasPrefix(string(got), s) {
			t.Fatalf("got:\n%s\nwant prefix:\n%s", got, s)
		}
	}
}

// kanbanFails runs a command that must be refused: exit 1 and one "kanban: " line on stderr.
func kanbanFails(t *testing.T, cwd, stdin string, argv ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(argv, cwd, strings.NewReader(stdin), &out, &errb)
	msg := errb.String()
	if code != 1 || !strings.HasPrefix(msg, "kanban: ") || strings.Count(msg, "\n") != 1 {
		t.Fatalf("kanban %v: exit %d, stderr %q, stdout %q; want exit 1 and one kanban: line", argv, code, msg, out.String())
	}
	return msg
}

// taskFile is where an issue's issue.yaml is.
func taskFile(root, id string) string { return filepath.Join(taskDir(root, id), issueFileName) }

func newLedger(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	os.MkdirAll(root, 0o755)
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "")
	kanban(t, root, "init", "--root", root)
	kanban(t, root, "add", "First")
	return root
}

func TestPrefixThatLeavesTasksIsRefused(t *testing.T) {
	root := newLedger(t)
	outside := filepath.Join(filepath.Dir(root), "escape")
	for _, prefix := range []string{"../../../escape", `..\..\..\escape`, "a/b", "..", "C:x"} {
		os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 3\nprefix: '"+prefix+"'\n"), 0o644)
		kanbanFails(t, root, "", "add", "x")
	}
	matches, _ := filepath.Glob(outside + "*")
	entries, _ := os.ReadDir(issuesDir(root))
	if len(matches) != 0 || len(entries) != 1 {
		t.Fatalf("written outside %v, tasks %d", matches, len(entries))
	}
	kanbanFails(t, root, "", "init", "--root", filepath.Join(filepath.Dir(root), "other"), "--prefix", "../x")
}

func TestIDThatLeavesTasksIsRefused(t *testing.T) {
	root := newLedger(t)
	// An issue.yaml one level up from issues/, which "../" ids would reach.
	sentinel := filepath.Join(root, "kanban", "issue.yaml")
	os.WriteFile(sentinel, []byte("title: sentinel\n"), 0o644)
	abs := filepath.Join(root, "kanban")
	for _, id := range []string{"..", "../", "../W-0001", `..\W-0001`, `W-0001\..\..`, abs, "/tmp", "C:W-0001"} {
		kanbanFails(t, root, "", "show", id)
		kanbanFails(t, root, "", "set", id, "title=x")
		kanbanFails(t, root, "", "start", id)
		kanbanFails(t, root, "", "review", id)
		kanbanFails(t, root, "", "done", id)
		kanbanFails(t, root, "report", "report", id)
		kanbanFails(t, root, "report", "report", id, "--append")
	}
	if got, _ := os.ReadFile(sentinel); string(got) != "title: sentinel\n" {
		t.Fatalf("sentinel changed: %q", got)
	}
	if exists(filepath.Join(root, "kanban", "report.md")) {
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
	root := newLedger(t)
	outDir := t.TempDir()
	os.WriteFile(filepath.Join(outDir, "issue.yaml"), []byte("title: outside\n"), 0o644)
	symlink(t, outDir, filepath.Join(issuesDir(root), "escape"))
	kanbanFails(t, root, "", "set", "escape", "title=x")
	kanbanFails(t, root, "", "start", "escape")
	kanbanFails(t, root, "", "done", "escape")
	kanbanFails(t, root, "r", "report", "escape")
	assertUnchanged(t, filepath.Join(outDir, "issue.yaml"), "title: outside\n")
	if exists(filepath.Join(outDir, "report.md")) {
		t.Fatal("report written through the link")
	}
}

func TestReportLinkIsRefused(t *testing.T) {
	root := newLedger(t)
	out := sentinelFile(t, "outside.md", "outside\n")
	symlink(t, out, filepath.Join(taskDir(root, "W-0001"), "report.md"))
	kanbanFails(t, root, "more\n", "report", "W-0001", "--append")
	kanbanFails(t, root, "new\n", "report", "W-0001")
	assertUnchanged(t, out, "outside\n")
}

func TestLogLinkIsRefused(t *testing.T) {
	root := newLedger(t)
	out := sentinelFile(t, "outside.jsonl", "outside\n")
	logFile := filepath.Join(root, "kanban", "log.jsonl")
	os.Remove(logFile)
	symlink(t, out, logFile)
	before, _ := os.ReadFile(taskFile(root, "W-0001"))
	kanbanFails(t, root, "", "set", "W-0001", "title=x")
	kanbanFails(t, root, "", "add", "y")
	assertUnchanged(t, out, "outside\n")
	assertUnchanged(t, taskFile(root, "W-0001"), string(before))
}

func TestTaskFileLinkIsRefused(t *testing.T) {
	root := newLedger(t)
	out := sentinelFile(t, "issue.yaml", "title: outside\n")
	os.Remove(taskFile(root, "W-0001"))
	symlink(t, out, taskFile(root, "W-0001"))
	kanbanFails(t, root, "", "set", "W-0001", "title=x")
	assertUnchanged(t, out, "title: outside\n")
}

func TestInitLeavesAnExistingGitignore(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", "")
	os.MkdirAll(filepath.Join(root, "kanban"), 0o755)
	mine := filepath.Join(root, "kanban", ".gitignore")
	os.WriteFile(mine, []byte("mine\n"), 0o644)
	kanban(t, root, "init", "--root", root)
	assertUnchanged(t, mine, "mine\n")
}

func TestInitWritesFormat3(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", "")
	kanban(t, root, "init", "--root", root, "--prefix", "FD", "--repo", "github.com/x/app", "--repo", "github.com/x/lib")
	cfg, err := readConfig(root)
	if err != nil || cfg.prefix != "FD" || strings.Join(cfg.repos, " ") != "github.com/x/app github.com/x/lib" {
		t.Fatalf("cfg %+v, err %v", cfg, err)
	}
	assertUnchanged(t, filepath.Join(root, "kanban", ".gitignore"), "*\n")
	other := t.TempDir()
	kanban(t, other, "init")
	if cfg, err := readConfig(other); err != nil || len(cfg.repos) != 0 || cfg.prefix != "W" {
		t.Fatalf("init in the current folder: cfg %+v, err %v", cfg, err)
	}
}

func TestOtherFormatVersionsAreRefused(t *testing.T) {
	root := newLedger(t)
	cfgFile := filepath.Join(root, "kanban", "config.yaml")
	logFile := filepath.Join(root, "kanban", "log.jsonl")
	before, _ := os.ReadFile(taskFile(root, "W-0001"))
	logBefore, _ := os.ReadFile(logFile)
	for _, cfg := range []string{"prefix: W\nprojects:\n  app: projects/app\n", "version: 1\n", "version: 4\n", "version: [2]\n"} {
		os.WriteFile(cfgFile, []byte(cfg), 0o644)
		msg := kanbanFails(t, root, "", "add", "x")
		kanbanFails(t, root, "", "set", "W-0001", "title=y")
		kanbanFails(t, root, "", "list")
		if !strings.Contains(msg, "format") {
			t.Errorf("%q: %s", cfg, msg)
		}
	}
	assertUnchanged(t, taskFile(root, "W-0001"), string(before))
	assertUnchanged(t, logFile, string(logBefore))
	if entries, _ := os.ReadDir(issuesDir(root)); len(entries) != 1 {
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
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, t.TempDir(), "init", "--root", root, "--repo", "github.com/x/app")
	kanbanFails(t, root, "", "add", "x", "--repo", "github.com/x/other")
	kanban(t, root, "add", "Work", "--repo", "github.com/x/app", "--profile", "opus", "--phase", "P1")
	kanban(t, root, "start", "W-0001")
	got, _ := os.ReadFile(taskFile(root, "W-0001"))
	want := "id: W-0001\ntitle: Work\nstatus: doing\nrepo: github.com/x/app\nphase: P1\nprofile: opus\nagent: t3\n"
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("got:\n%s\nwant prefix:\n%s", got, want)
	}
	if out := kanban(t, root, "list", "--repo", "github.com/x/app"); !strings.Contains(out, "W-0001") {
		t.Fatalf("list --repo: %q", out)
	}
	if out := kanban(t, root, "ready", "--repo", "github.com/x/app"); strings.Contains(out, "W-0001") {
		t.Fatalf("ready --repo shows a doing task: %q", out)
	}
	kanban(t, root, "set", "W-0001", "repo=github.com/x/gone", "profile=codex")
	var out, errb bytes.Buffer
	if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(out.String(), "W-0001: repo github.com/x/gone is not in config.yaml repos") {
		t.Fatalf("lint: exit %d, %q %q", code, out.String(), errb.String())
	}
}

// v1 and unknown versions are refused by every command, before anything is read or made.
func TestOtherVersionsAreRefusedByEveryCommand(t *testing.T) {
	for _, cfg := range []string{"prefix: W\nprojects: {}\n", "version: 1\n", "version: 4\n"} {
		root := t.TempDir()
		t.Setenv("KANBAN_ROOT", root)
		fd := filepath.Join(root, "kanban")
		os.MkdirAll(filepath.Join(fd, "issues", "W-0001"), 0o755)
		os.WriteFile(filepath.Join(fd, "config.yaml"), []byte(cfg), 0o644)
		os.WriteFile(filepath.Join(fd, "issues", "W-0001", "issue.yaml"), []byte("id: W-0001\ntitle: old\nstatus: todo\n"), 0o644)
		kanbanFails(t, root, "text\n", "report", "W-0001")
		kanbanFails(t, root, "text\n", "report", "W-0001", "--append")
		kanbanFails(t, root, "", "root")
		kanbanFails(t, root, "", "show", "W-0001")
		kanbanFails(t, root, "", "init", "--root", root)
		kanbanFails(t, root, "", "config", "set", "chief_of_staff=t9")
		assertUnchanged(t, filepath.Join(fd, "config.yaml"), cfg)
		for _, made := range []string{"log.jsonl", ".lock", ".gitignore", filepath.Join("issues", "W-0001", "report.md")} {
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
		os.MkdirAll(filepath.Join(bare, "kanban"), 0o755)
		os.WriteFile(filepath.Join(bare, "kanban", "config.yaml"), []byte(cfg), 0o644)
		kanbanFails(t, bare, "", "init", "--root", bare)
		if entries, _ := os.ReadDir(filepath.Join(bare, "kanban")); len(entries) != 1 {
			t.Errorf("%q: init made %d entries", cfg, len(entries))
		}
	}
}

// repos entries must be exactly host/owner/repo: init and --repo refuse others, lint reports them.
func TestReposMustBeHostOwnerRepo(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	for _, bad := range []string{"github.com/x/app/", "/github.com/x/app", "x/app", "github.com//app"} {
		dir := filepath.Join(t.TempDir(), "p")
		kanbanFails(t, t.TempDir(), "", "init", "--root", dir, "--repo", bad)
		if exists(dir) {
			t.Errorf("init --repo %q made %s", bad, dir)
		}
	}
	root := filepath.Join(t.TempDir(), "root")
	kanban(t, t.TempDir(), "init", "--root", root, "--repo", "github.com/x/app")
	t.Setenv("KANBAN_ROOT", root)
	os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 3\nrepos: [github.com/x/app, github.com/x/lib/, x/app]\n"), 0o644)
	kanbanFails(t, root, "", "add", "x", "--repo", "github.com/x/lib/")
	kanbanFails(t, root, "", "add", "x", "--repo", "x/app")
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
	root := newLedger(t)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "set", "W-0001", "profile=opus")
	kanban(t, root, "start", "W-0001")
	kanban(t, root, "set", "W-0001", "who=person")
	got, _ := os.ReadFile(taskFile(root, "W-0001"))
	if !strings.Contains(string(got), "profile: opus\nwho: person\nagent: t3\n") {
		t.Fatalf("got:\n%s", got)
	}
	kanban(t, root, "set", "W-0001", "who=")
	if got, _ := os.ReadFile(taskFile(root, "W-0001")); strings.Contains(string(got), "who") {
		t.Fatalf("who= left:\n%s", got)
	}
}

// kanban config set chief_of_staff= rewrites config.yaml under the lock, keeping comments and order, and logs it.
func TestConfigSetChiefOfStaff(t *testing.T) {
	root := newLedger(t)
	file := filepath.Join(root, "kanban", "config.yaml")
	os.WriteFile(file, []byte("# by hand\nversion: 3\ntitle: App # shown\nchief_of_staff: t1 # the lead\nprefix: W\nmine: kept\n"), 0o644)
	if out := kanban(t, root, "config", "set", "chief_of_staff=t7"); out != "chief_of_staff: t7\n" {
		t.Fatalf("out %q", out)
	}
	assertUnchanged(t, file, "# by hand\nversion: 3\ntitle: App # shown\nchief_of_staff: t7 # the lead\nprefix: W\nmine: kept\n")
	kanban(t, root, "config", "set", "chief_of_staff=")
	assertUnchanged(t, file, "# by hand\nversion: 3\ntitle: App # shown\nprefix: W\nmine: kept\n")
	if out := kanban(t, root, "config", "set", "chief_of_staff=t8", "--json"); !strings.Contains(out, `"chief_of_staff": "t8"`) {
		t.Fatalf("json %q", out)
	}
	assertUnchanged(t, file, "# by hand\nversion: 3\ntitle: App # shown\nprefix: W\nmine: kept\nchief_of_staff: t8\n")
	if cfg, err := readConfig(root); err != nil || cfg.prefix != "W" {
		t.Fatalf("cfg %+v, err %v", cfg, err)
	}
	log, _ := os.ReadFile(filepath.Join(root, "kanban", "log.jsonl"))
	for _, want := range []string{`"config":{"chief_of_staff":"t7"}`, `"config":{"chief_of_staff":null}`, `"config":{"chief_of_staff":"t8"}`} {
		if !strings.Contains(string(log), want) {
			t.Errorf("log has no %s:\n%s", want, log)
		}
	}
	for _, bad := range [][]string{{"config"}, {"config", "set"}, {"config", "set", "prefix=X"}, {"config", "set", "chief_of_staff"}, {"config", "get", "chief_of_staff=t1"}} {
		kanbanFails(t, root, "", bad...)
	}
	assertUnchanged(t, file, "# by hand\nversion: 3\ntitle: App # shown\nprefix: W\nmine: kept\nchief_of_staff: t8\n")
}

func TestConfigLinkIsRefused(t *testing.T) {
	root := newLedger(t)
	file := filepath.Join(root, "kanban", "config.yaml")
	real := sentinelFile(t, "config.yaml", "version: 3\n")
	os.Remove(file)
	symlink(t, real, file)
	kanbanFails(t, root, "", "config", "set", "chief_of_staff=t7")
	assertUnchanged(t, real, "version: 3\n")
}

// A reviewer's task (--parent and a *-review profile) is <parent>-r<k> and takes no number from the sequence.
func TestReviewerIDs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", root)
	kanban(t, root, "init", "--root", root, "--prefix", "A")
	for range 5 {
		kanban(t, root, "add", "work")
	}
	if got := kanban(t, root, "add", "A-0005 のレビュー（1 回目）", "--parent", "A-5", "--profile", "codex-review", "--round", "1"); got != "A-0005-r1\n" {
		t.Fatalf("round 1: %q", got)
	}
	if got := kanban(t, root, "add", "A-0005 のレビュー（2 回目）", "--parent", "A-0005", "--profile", "codex-review"); got != "A-0005-r2\n" {
		t.Fatalf("round 2 (counted): %q", got)
	}
	if got := kanban(t, root, "add", "next"); got != "A-0006\n" {
		t.Fatalf("the sequence moved: %q", got)
	}
	if got := kanban(t, root, "add", "not a reviewer", "--parent", "A-0005", "--profile", "claude"); got != "A-0007\n" {
		t.Fatalf("a child that isn't a reviewer: %q", got)
	}
	kanbanFails(t, root, "", "add", "again", "--parent", "A-0005", "--profile", "codex-review", "--round", "2")
	kanbanFails(t, root, "", "add", "x", "--parent", "A-0005", "--profile", "codex-review", "--round", "0")
	kanbanFails(t, root, "", "add", "x", "--parent", "A-0005", "--round", "3")
	kanbanFails(t, root, "", "add", "x", "--parent", "A-0099", "--profile", "codex-review")

	// read as typed, in every place an id is taken
	kanban(t, root, "start", "A-5-r01")
	kanban(t, root, "set", "A-0006", "after=A-5-r1")
	got, _ := os.ReadFile(taskFile(root, "A-0006"))
	if !strings.Contains(string(got), "after: [A-0005-r1]") {
		t.Fatalf("after not read as A-0005-r1:\n%s", got)
	}
	if out := kanban(t, root, "ask", "A-5-r2", "which?"); out != "A-0005-r2#1\n" {
		t.Fatalf("ask: %q", out)
	}
	kanban(t, root, "answer", "A-5-r2#1", "this")
	if out := kanban(t, root, "show", "A-0005-r1"); !strings.Contains(out, "A-0005 のレビュー（1 回目）") {
		t.Fatalf("show: %q", out)
	}
	// listed right after their parent, in round order
	out := kanban(t, root, "list", "--all")
	if a, b, c, d := strings.Index(out, "A-0005 "), strings.Index(out, "A-0005-r1"), strings.Index(out, "A-0005-r2"), strings.Index(out, "A-0006"); !(a < b && b < c && c < d) {
		t.Fatalf("order:\n%s", out)
	}
}

// Older reviewers' tasks took a number (A-0003 reviews A-0002); they still read, and count toward the round.
func TestOldReviewerTasksAreRead(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", root)
	kanban(t, root, "init", "--root", root, "--prefix", "A")
	kanban(t, root, "add", "work")
	put(t, taskFile(root, "A-0002"), "id: A-0002\ntitle: A-0001 のレビュー（1 回目）\nstatus: done\nparent: A-0001\nprofile: codex-review\n")
	if out := kanban(t, root, "show", "A-2"); !strings.Contains(out, "A-0001 のレビュー（1 回目）") {
		t.Fatalf("show: %q", out)
	}
	if got := kanban(t, root, "add", "A-0001 のレビュー（2 回目）", "--parent", "A-1", "--profile", "codex-review"); got != "A-0001-r2\n" {
		t.Fatalf("round after an old reviewer: %q", got)
	}
	if got := kanban(t, root, "add", "next"); got != "A-0003\n" {
		t.Fatalf("next: %q", got)
	}
	kanban(t, root, "lint")
}

func TestLintChecksReviewerIDs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", root)
	kanban(t, root, "init", "--root", root, "--prefix", "A")
	kanban(t, root, "add", "work")
	kanban(t, root, "add", "other")
	put(t, taskFile(root, "A-0001-r1"), "id: A-0001-r1\ntitle: x\nstatus: todo\nparent: A-0002\nprofile: claude\n")
	var out, errb bytes.Buffer
	if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 ||
		!strings.Contains(out.String(), `A-0001-r1: a reviewer's id, but parent is "A-0002", not A-0001`) ||
		!strings.Contains(out.String(), `A-0001-r1: a reviewer's id, but profile "claude" doesn't end in -review`) {
		t.Fatalf("lint: exit %d, %q %q", code, out.String(), errb.String())
	}
}

// A folder named the way an id is typed (A-5, A-0005-r01) is read by no command; lint says so.
func TestLintChecksIDsAreWrittenCanonically(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", root)
	kanban(t, root, "init", "--root", root, "--prefix", "A")
	kanban(t, root, "add", "work")
	put(t, taskFile(root, "A-0001-r01"), "id: A-0001-r01\ntitle: x\nstatus: todo\nparent: A-0001\nprofile: codex-review\n")
	put(t, taskFile(root, "A-2"), "id: A-2\ntitle: y\nstatus: todo\n")
	var out, errb bytes.Buffer
	if code := run([]string{"lint"}, root, strings.NewReader(""), &out, &errb); code != 1 ||
		!strings.Contains(out.String(), "A-0001-r01: not written as an id is (A-0001-r1)") ||
		!strings.Contains(out.String(), "A-2: not written as an id is (A-0002)") {
		t.Fatalf("lint: exit %d, %q %q", code, out.String(), errb.String())
	}
}

// An issue.yaml that can't be read stops a reviewer's add without --round (it can't be counted), with a line, not a panic.
func TestReviewerAddWithABrokenTask(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KANBAN_ROOT", root)
	kanban(t, root, "init", "--root", root, "--prefix", "A")
	kanban(t, root, "add", "work")
	kanban(t, root, "add", "other")
	put(t, taskFile(root, "A-0002"), "- not a mapping\n")
	if msg := kanbanFails(t, root, "", "add", "review", "--parent", "A-0001", "--profile", "codex-review"); !strings.Contains(msg, "issue A-0002 can't be read") {
		t.Fatalf("message: %q", msg)
	}
	if got := kanban(t, root, "add", "review", "--parent", "A-0001", "--profile", "codex-review", "--round", "1"); got != "A-0001-r1\n" {
		t.Fatalf("with --round: %q", got)
	}
}

// start writes a worktree under HOME as ~/…, and one whose path only begins with HOME's text (a sibling
// directory) in full: readers open ~/ and nothing else.
func TestStartShortensOnlyWorktreesUnderHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("HOME is not what os.UserHomeDir reads on Windows")
	}
	root := newLedger(t) // W-0001
	kanban(t, root, "add", "second")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "h"))
	for id, dir := range map[string]string{"W-0001": filepath.Join(base, "h", "src", "repo"), "W-0002": filepath.Join(base, "h2", "repo")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", dir, "init", "-b", "main").CombinedOutput(); err != nil {
			t.Skipf("git: %v %s", err, out)
		}
		kanban(t, dir, "start", id)
	}
	if got, _ := os.ReadFile(taskFile(root, "W-0001")); !strings.Contains(string(got), "worktree: ~/src/repo\n") {
		t.Errorf("a worktree under HOME should be written as ~/src/repo:\n%s", got)
	}
	if got, _ := os.ReadFile(taskFile(root, "W-0002")); !strings.Contains(string(got), "worktree: "+filepath.Join(base, "h2", "repo")+"\n") {
		t.Errorf("a worktree beside HOME should be written in full:\n%s", got)
	}
}

// A reviewer's issue keeps the worktree and branch it was given, from any cwd; a worker's issue still takes them from git.
func TestStartKeepsAReviewersWorktreeAndBranch(t *testing.T) {
	root := newLedger(t) // W-0001
	kanban(t, root, "add", "review", "--parent", "W-0001", "--profile", "codex-review")
	kanban(t, root, "set", "W-0001-r1", "worktree=~/wt/sample-r1")

	repo := t.TempDir()
	for _, c := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-m", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, c...)...).CombinedOutput(); err != nil {
			t.Skipf("git: %v %s", err, out)
		}
	}
	for _, cwd := range []string{repo, root} {
		kanban(t, cwd, "start", "W-0001-r1")
		got, _ := os.ReadFile(taskFile(root, "W-0001-r1"))
		if !strings.Contains(string(got), "worktree: ~/wt/sample-r1") || strings.Contains(string(got), "branch: main") {
			t.Fatalf("reviewer's worktree/branch changed from %s:\n%s", cwd, got)
		}
	}
	// the first start (the dispatcher's, from the detached worktree) sets it when it is still empty
	kanban(t, root, "add", "review 2", "--parent", "W-0001", "--profile", "claude-review")
	kanban(t, repo, "start", "W-0001-r2")
	if got, _ := os.ReadFile(taskFile(root, "W-0001-r2")); !strings.Contains(string(got), "worktree: ") {
		t.Fatalf("the first start should set the worktree:\n%s", got)
	}
	kanban(t, repo, "start", "W-0001")
	got, _ := os.ReadFile(taskFile(root, "W-0001"))
	if !strings.Contains(string(got), "branch: main") {
		t.Fatalf("a worker's issue should still take its branch from git:\n%s", got)
	}
}

// --help after a command prints the help; after --, every word is a word, not a flag.
func TestHelpFlagAndDoubleDash(t *testing.T) {
	root := newLedger(t)
	for _, argv := range [][]string{{"add", "--help"}, {"list", "--help"}, {"set", "W-0001", "--help"}} {
		if out := kanban(t, root, argv...); !strings.Contains(out, "kanban add \"title\"") {
			t.Fatalf("kanban %v: %q", argv, out)
		}
	}
	if n := len(allTasks(root)); n != 1 {
		t.Fatalf("--help added an issue: %d", n)
	}
	kanban(t, root, "add", "--profile", "p", "--", "--help is not a flag")
	if got := readYAML(t, taskFile(root, "W-0002")); got["title"] != "--help is not a flag" || got["profile"] != "p" {
		t.Fatalf("W-0002: %v", got)
	}
}
