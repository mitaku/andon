package main

import (
	"encoding/json"

	"andon/internal/delegate"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A new ledger's tasks are numbered from A-0001; A-1 and A-01 name the same task.
func TestIdsHaveFourDigitsAndShortIdsAreRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "")
	kanban(t, root, "init", "--root", root, "--prefix", "A")
	if out := kanban(t, root, "add", "First"); out != "A-0001\n" {
		t.Fatalf("add: %q", out)
	}
	if out := kanban(t, root, "add", "Second", "--after", "A-1", "--parent", "A-01"); out != "A-0002\n" {
		t.Fatalf("add: %q", out)
	}
	if s := readFile(t, taskFile(root, "A-0002")); !strings.Contains(s, "parent: A-0001\n") || !strings.Contains(s, "after: [A-0001]\n") {
		t.Fatalf("task.yaml: %q", s)
	}
	if a, b := kanban(t, root, "show", "A-2", "--json"), kanban(t, root, "show", "A-0002", "--json"); a != b {
		t.Fatalf("show A-2 and A-0002 differ:\n%s\n%s", a, b)
	}
	kanban(t, root, "set", "A-2", "after=A-1")
	kanban(t, root, "ask", "A-2", "which?")
	kanban(t, root, "answer", "A-2#1", "this")
	if s := readFile(t, filepath.Join(taskDir(root, "A-0002"), "questions.yaml")); !strings.Contains(s, "answer: this") {
		t.Fatalf("questions.yaml: %q", s)
	}
}

// kanban/ is a plain name: a kanban/ folder without a ledger's config.yaml is not a ledger.
func TestAKanbanFolderInARepositoryIsNotALedger(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	ws := t.TempDir()
	project := filepath.Join(ws, "projects", "app")
	repo := filepath.Join(ws, ".ghq", "github.com", "x", "app")
	os.MkdirAll(project, 0o755)
	kanban(t, project, "init", "--root", project, "--repo", "github.com/x/app")
	gitInit(t, repo)
	for name, body := range map[string]string{"": "", "config.yaml": "version: '2'\ntheme: dark\n", "tasks/W-1/task.yaml": "id: W-1\n"} {
		if name == "" {
			os.MkdirAll(filepath.Join(repo, "kanban"), 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(filepath.Join(repo, "kanban", name)), 0o755)
		os.WriteFile(filepath.Join(repo, "kanban", name), []byte(body), 0o644)
	}
	if out := strings.TrimSpace(kanban(t, repo, "root")); !sameFile(out, project) {
		t.Fatalf("root from the repository: %q, want %q", out, project)
	}
	if out := strings.TrimSpace(kanban(t, filepath.Join(repo, "kanban"), "root")); !sameFile(out, project) {
		t.Fatalf("root from its kanban/: %q, want %q", out, project)
	}
	// A quoted version is a string, not a format: kanban writes to the project's ledger, not the repository's.
	if out := kanban(t, repo, "add", "x"); out != "W-0001\n" || !exists(taskFile(project, "W-0001")) || exists(filepath.Join(repo, "kanban", "tasks", "W-0001")) {
		t.Fatalf("add from the repository: %q", out)
	}
	t.Setenv("KANBAN_ROOT", repo)
	deckFails(t, repo, "", "list")
}

// kanban migrate moves .flightdeck/ to kanban/: the folder (still out of git), chief_of_staff, the ids,
// coordinator.md and the dispatcher's state. log.jsonl keeps the old ids, read as the new.
func TestMigrateMovesTheOldLedger(t *testing.T) {
	ws := t.TempDir()
	root := filepath.Join(ws, "projects", "app")
	old := filepath.Join(root, ".flightdeck")
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("HC_TERMINAL_ID", "")
	gitInit(t, ws)
	put(t, filepath.Join(old, ".gitignore"), "*\n")
	put(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\n# the lead\ncoordinator: t5 # now\nrepos: []\n")
	put(t, filepath.Join(old, "coordinator.md"), "並行の上限: 3\n")
	put(t, filepath.Join(root, "AGENTS.md"), "# app\n\nthe map\n")
	put(t, filepath.Join(root, "delegation.json"), `{"version": 1, "project": "app", "preset": "hobby", "grants": [{"id": "d1", "scope": "merge", "value": "B", "by": "owner", "at": "2026-11-06", "until": null, "evidence": "x"}]}`)
	put(t, filepath.Join(old, "tasks", "A-3", "task.yaml"), "id: A-3\ntitle: three\nstatus: done\n")
	put(t, filepath.Join(old, "tasks", "A-5", "task.yaml"), "id: A-5\ntitle: five # kept\nstatus: todo\nparent: A-3\nafter: [A-3]\n")
	put(t, filepath.Join(old, "tasks", "A-5", "questions.yaml"), "- n: 1\n  text: which?\n  status: open\n")
	put(t, filepath.Join(old, "tasks", "A-5", "library", "x.json"), "{}\n")
	put(t, filepath.Join(old, "tasks", "A-12", "task.yaml"), "id: A-12\ntitle: twelve\nstatus: todo\n")
	put(t, filepath.Join(old, "log.jsonl"), `{"at":"2026-11-01T00:00:00Z","by":"terminal:t5","id":"A-5","change":{"question":1,"status":"open"}}`+"\n")
	put(t, filepath.Join(old, "state", "routines.json"), "{}\n")
	dstate := filepath.Join(state, "andon", "dispatcher", "app.json")
	put(t, dstate, `{"reviewers":{"A-5@abc":{"task":"A-5","reviewer_task":"A-12"}},"gates":{"A-5#3":{"task":"A-5"}},"pending":[{"task":"A-3","kind":"review"}],"ready":["A-5"],"sent":{"A-5|t9|done|1":true},"coord":"t5","worktree":"/w/yard-A-5"}`)

	kanban(t, root, "migrate")

	dir := filepath.Join(root, "kanban")
	if exists(old) {
		t.Fatal(".flightdeck/ is still there")
	}
	assertUnchanged(t, filepath.Join(dir, ".gitignore"), "*\n")
	assertUnchanged(t, filepath.Join(dir, "config.yaml"), "version: 2\nprefix: A\n# the lead\nchief_of_staff: t5 # now\nrepos: []\n")
	got, _ := os.ReadDir(filepath.Join(dir, "tasks"))
	names := []string{}
	for _, e := range got {
		names = append(names, e.Name())
	}
	if strings.Join(names, " ") != "A-0003 A-0005 A-0012" {
		t.Fatalf("tasks: %v", names)
	}
	assertUnchanged(t, filepath.Join(dir, "tasks", "A-0005", "task.yaml"), "id: A-0005\ntitle: five # kept\nstatus: todo\nparent: A-0003\nafter: [A-0003]\n")
	if !exists(filepath.Join(dir, "tasks", "A-0005", "library", "x.json")) {
		t.Fatal("library/ did not move with its task")
	}
	assertUnchanged(t, filepath.Join(root, "AGENTS.md"), "# app\n\nthe map\n\n## Chief of Staff\n\n並行の上限: 3\n")
	if exists(filepath.Join(dir, "coordinator.md")) {
		t.Fatal("coordinator.md is still in the ledger")
	}
	// kanban/ stays out of git (ADR-0005, 5); nothing is git added.
	if out := git(ws, "status", "--porcelain", "--untracked-files=all"); strings.Contains(out, "kanban/") || git(ws, "diff", "--cached", "--name-only") != "" {
		t.Fatalf("git status after migrate:\n%s", out)
	}
	// A draft of project.json from config.yaml and delegation.json, which Andon reads with the same delegation.
	p, err := delegate.ParseProject([]byte(readFile(t, filepath.Join(root, "project.json"))), "app")
	if err != nil || p.Project.Title != "app" || p.Project.Prefix != "A" || len(p.Repos) != 0 {
		t.Fatalf("project.json draft: %+v %v", p, err)
	}
	if f, err := delegate.Parse(p.Delegation, "app"); err != nil || f.Effective("2026-11-07")["merge"] != "B" {
		t.Fatalf("project.json delegation: %v", err)
	}
	if !exists(filepath.Join(root, "delegation.json")) {
		t.Fatal("delegation.json was removed")
	}
	var st map[string]any
	json.Unmarshal([]byte(readFile(t, dstate)), &st)
	b, _ := json.Marshal(st)
	if want := `{"coord":"t5","gates":{"A-0005#3":{"task":"A-0005"}},"pending":[{"kind":"review","task":"A-0003"}],"ready":["A-0005"],"reviewers":{"A-0005@abc":{"reviewer_task":"A-0012","task":"A-0005"}},"sent":{"A-0005|t9|done|1":true},"worktree":"/w/yard-A-5"}`; string(b) != want {
		t.Fatalf("dispatcher state:\n%s\nwant\n%s", b, want)
	}
	// A-5#1 still answers: the question kept its number (log.jsonl says A-5).
	t.Setenv("KANBAN_ROOT", root)
	kanban(t, root, "answer", "A-5#1", "this")
	if out := kanban(t, root, "ask", "A-0005", "and?"); out != "A-0005#2\n" {
		t.Fatalf("ask after migrate: %q", out)
	}
	if out := kanban(t, root, "add", "next"); out != "A-0013\n" {
		t.Fatalf("add after migrate: %q", out)
	}
}

// A task in doing or review stops the move, and nothing changes.
func TestMigrateStopsOnDoingOrReview(t *testing.T) {
	for _, status := range []string{"doing", "review"} {
		root := t.TempDir()
		t.Setenv("KANBAN_ROOT", "")
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		old := filepath.Join(root, ".flightdeck")
		put(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\ncoordinator: t5\n")
		put(t, filepath.Join(old, "tasks", "A-1", "task.yaml"), "id: A-1\ntitle: one\nstatus: done\n")
		put(t, filepath.Join(old, "tasks", "A-2", "task.yaml"), "id: A-2\ntitle: two\nstatus: "+status+"\n")
		msg := deckFails(t, root, "", "migrate")
		if !strings.Contains(msg, "A-2 ("+status+")") {
			t.Fatalf("%s: %q", status, msg)
		}
		if exists(filepath.Join(root, "kanban")) || !exists(filepath.Join(old, "tasks", "A-1")) {
			t.Fatalf("%s: something moved", status)
		}
		assertUnchanged(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\ncoordinator: t5\n")
		assertUnchanged(t, filepath.Join(old, "tasks", "A-1", "task.yaml"), "id: A-1\ntitle: one\nstatus: done\n")
	}
}

// migrate refuses when there is nothing to move, or kanban/ is already there.
func TestMigrateRefusesWithoutOldOrWithNew(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	root := t.TempDir()
	deckFails(t, root, "", "migrate")
	put(t, filepath.Join(root, ".flightdeck", "config.yaml"), "version: 2\n")
	os.MkdirAll(filepath.Join(root, "kanban"), 0o755)
	deckFails(t, root, "", "migrate")
	assertUnchanged(t, filepath.Join(root, ".flightdeck", "config.yaml"), "version: 2\n")
}

func put(t *testing.T, file, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, file string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
}

func sameFile(a, b string) bool {
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(fa, fb)
}

// A task folder that is a symbolic link stops the move before anything changes.
func TestMigrateRefusesALinkedTaskFolder(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	old := filepath.Join(root, ".flightdeck")
	put(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\n")
	put(t, filepath.Join(old, "tasks", "A-1", "task.yaml"), "id: A-1\ntitle: one\nstatus: todo\n")
	elsewhere := filepath.Join(t.TempDir(), "A-5")
	put(t, filepath.Join(elsewhere, "task.yaml"), "id: A-5\ntitle: five\nstatus: todo\n")
	symlink(t, elsewhere, filepath.Join(old, "tasks", "A-5"))
	deckFails(t, root, "", "migrate")
	if exists(filepath.Join(root, "kanban")) || !exists(filepath.Join(old, "tasks", "A-1")) {
		t.Fatal("something moved")
	}
	assertUnchanged(t, filepath.Join(old, "tasks", "A-1", "task.yaml"), "id: A-1\ntitle: one\nstatus: todo\n")
}

// A dispatcher state that isn't JSON stops the move before anything changes; once it is fixed, migrate runs.
func TestMigrateChecksTheDispatcherStateFirst(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	root := filepath.Join(t.TempDir(), "app")
	old := filepath.Join(root, ".flightdeck")
	put(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\ncoordinator: t5\n")
	put(t, filepath.Join(old, "tasks", "A-5", "task.yaml"), "id: A-5\ntitle: five\nstatus: todo\n")
	file := filepath.Join(state, "andon", "dispatcher", "app.json")
	for _, bad := range []string{"{not json", "[]", `{"reviewers": []}`} {
		put(t, file, bad)
		deckFails(t, root, "", "migrate")
		if exists(filepath.Join(root, "kanban")) || !exists(filepath.Join(old, "tasks", "A-5")) {
			t.Fatalf("%s: something moved", bad)
		}
		assertUnchanged(t, file, bad)
	}
	assertUnchanged(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\ncoordinator: t5\n")
	put(t, file, `{"ready":["A-5"]}`)
	kanban(t, root, "migrate")
	if !exists(taskFile(root, "A-0005")) || !strings.Contains(readFile(t, file), `"A-0005"`) {
		t.Fatalf("migrate after the fix: %s", readFile(t, file))
	}
}

// project.json is drafted only when there is none, and a delegation.json that isn't a JSON object stops the move.
func TestMigrateProjectDraft(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "app")
	put(t, filepath.Join(root, ".flightdeck", "config.yaml"), "version: 2\nprefix: A\ntitle: App\nrepos:\n  - github.com/x/app\n")
	for _, bad := range []string{"[1]", "{}", "{not json", `{"version": 1, "project": "app", "preset": "hobby", "grants": [{"id": "x"}]}`} {
		put(t, filepath.Join(root, "delegation.json"), bad)
		deckFails(t, root, "", "migrate")
		if exists(filepath.Join(root, "kanban")) || !exists(filepath.Join(root, ".flightdeck", "config.yaml")) || exists(filepath.Join(root, "project.json")) {
			t.Fatalf("%s: something moved", bad)
		}
		assertUnchanged(t, filepath.Join(root, "delegation.json"), bad)
	}
	put(t, filepath.Join(root, "project.json"), "mine\n")
	kanban(t, root, "migrate")
	assertUnchanged(t, filepath.Join(root, "project.json"), "mine\n")

	other := filepath.Join(t.TempDir(), "web")
	put(t, filepath.Join(other, ".flightdeck", "config.yaml"), "version: 2\nprefix: W\ntitle: Web\nrepos:\n  - github.com/x/web\n")
	kanban(t, other, "migrate")
	assertUnchanged(t, filepath.Join(other, "project.json"), "{\n  \"version\": 1,\n  \"project\": {\n    \"title\": \"Web\",\n    \"prefix\": \"W\"\n  },\n  \"repos\": [\n    \"github.com/x/web\"\n  ]\n}\n")
}

// A quoted version is not format 2: nothing moves; an integer one does.
func TestMigrateNeedsAnIntegerVersion(t *testing.T) {
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "app")
	cfg := filepath.Join(root, ".flightdeck", "config.yaml")
	put(t, cfg, "version: '2'\nprefix: A\n")
	put(t, filepath.Join(root, ".flightdeck", "tasks", "A-1", "task.yaml"), "id: A-1\ntitle: one\nstatus: todo\n")
	deckFails(t, root, "", "migrate")
	assertUnchanged(t, cfg, "version: '2'\nprefix: A\n")
	if exists(filepath.Join(root, "kanban")) || exists(filepath.Join(root, "project.json")) || !exists(filepath.Join(root, ".flightdeck", "tasks", "A-1")) {
		t.Fatal("something moved")
	}
	put(t, cfg, "version: 2\nprefix: A\n")
	kanban(t, root, "migrate")
	if out := strings.TrimSpace(kanban(t, root, "root")); !sameFile(out, root) || !exists(taskFile(root, "A-0001")) {
		t.Fatalf("after migrate: root %q", out)
	}
}
