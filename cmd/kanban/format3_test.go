package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Format 3 (docs/ledger.md, Changes from version 2; ADR-0008): init makes version 3 and issues/,
// add writes issues/<id>/issue.yaml, show --json says issue.
func TestInitAndAddWriteFormat3(t *testing.T) {
	root := ledger(t)
	cfg := readFile(t, filepath.Join(root, "kanban", "config.yaml"))
	if !strings.HasPrefix(cfg, "version: 3\n") {
		t.Fatalf("config.yaml: %q", cfg)
	}
	if !exists(filepath.Join(root, "kanban", "issues", "W-0001", "issue.yaml")) || exists(filepath.Join(root, "kanban", "tasks")) {
		t.Fatal("add did not write issues/W-0001/issue.yaml (or made tasks/)")
	}
	show := kanban(t, root, "show", "W-0001", "--json")
	if !strings.Contains(show, `"issue": {`) || strings.Contains(show, `"task"`) {
		t.Fatalf("show --json: %s", show)
	}
	if out := kanban(t, root, "lint"); out != "ok (1 issues)\n" {
		t.Fatalf("lint: %q", out)
	}
}

// v2Ledger is a format 2 ledger as a person has it before moving it by hand: tasks/, task.yaml,
// a question, and the files Andon wrote in library/ whose values name a task (report.json's
// next[].to, questions/<n>.json's from).
func v2Ledger(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "app")
	k := filepath.Join(root, "kanban")
	put(t, filepath.Join(k, "config.yaml"), "# by hand\nversion: 2\nprefix: A\nrepos: []\n")
	put(t, filepath.Join(k, ".gitignore"), "*\n")
	put(t, filepath.Join(k, "log.jsonl"), `{"at":"2026-11-01T00:00:00Z","by":"terminal:t5","id":"A-0001","change":{"add":"one"}}`+"\n")
	put(t, filepath.Join(k, "tasks", "A-0001", "task.yaml"), "id: A-0001\ntitle: one # kept\nstatus: done\n")
	put(t, filepath.Join(k, "tasks", "A-0001", "brief.md"), "解決すべき問題: x\n")
	put(t, filepath.Join(k, "tasks", "A-0001", "library", "report.json"), `{"next": [{"to": "task", "text": "x"}]}`+"\n")
	put(t, filepath.Join(k, "tasks", "A-0002", "task.yaml"), "id: A-0002\ntitle: two\nstatus: todo\nafter: [A-0001]\n")
	put(t, filepath.Join(k, "tasks", "A-0002", "questions.yaml"), "- n: 1\n  text: which?\n  status: open\n  asked: 2026-11-01T00:00:00Z\n")
	put(t, filepath.Join(k, "tasks", "A-0002", "library", "questions", "1.json"), `{"kind": "scope", "question": "which?", "options": [{"label": "a"}], "from": "task"}`+"\n")
	return root
}

// snapshot is every file under dir with its contents, and every folder.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := map[string]string{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			m[rel+"/"] = ""
			return nil
		}
		b, _ := os.ReadFile(p)
		m[rel] = string(b)
		return nil
	})
	return m
}

// Every sub command stops on a format 2 ledger with the same one line, exit 1, and writes nothing.
func TestFormat2StopsEveryCommand(t *testing.T) {
	root := v2Ledger(t)
	argv := map[string][]string{
		"init": {"--root", root}, "add": {"x"}, "list": {"--all"}, "ready": nil, "show": {"A-0001"},
		"start": {"A-0002"}, "set": {"A-0002", "title=y"}, "review": {"A-0002"}, "done": {"A-0002"},
		"report": {"A-0002"}, "lint": nil, "tick": nil, "routine": {"list"}, "config": {"set", "chief_of_staff=t9"},
		"ask": {"A-0002", "why?"}, "questions": nil, "answer": {"A-0002#1", "this"}, "received": {"A-0002#1"},
		"migrate": {"--root", root}, "root": nil,
	}
	for name := range commands {
		if _, ok := argv[name]; !ok {
			t.Fatalf("no case for kanban %s: add it here", name)
		}
	}
	names := make([]string, 0, len(argv))
	for name := range argv {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("HC_TERMINAL_ID", "t3")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	before := snapshot(t, root)
	for _, name := range names {
		for _, cwd := range []string{root, filepath.Join(root, "kanban", "tasks", "A-0002")} {
			var out, errb bytes.Buffer
			code := run(append([]string{name}, argv[name]...), cwd, strings.NewReader("text\n"), &out, &errb)
			if want := "kanban: " + errFormat2 + "\n"; code != 1 || errb.String() != want || out.Len() != 0 {
				t.Errorf("kanban %s in %s: exit %d, stderr %q, stdout %q; want exit 1 and %q", name, cwd, code, errb.String(), out.String(), want)
			}
		}
	}
	after := snapshot(t, root)
	for k, v := range after {
		if b, ok := before[k]; !ok || b != v {
			t.Errorf("written: %s", k)
		}
	}
	if len(after) != len(before) {
		t.Errorf("%d entries before, %d after", len(before), len(after))
	}
}

// moveByHand is the hand move of docs/ledger.md (Changes from version 2), steps 3 to 5 of
// ADR-0008 decision 7: tasks/ → issues/, each task.yaml → issue.yaml, version 3, and the
// value task → issue in library/report.json's next[].to and library/questions/*.json's from.
func moveByHand(t *testing.T, root string) {
	t.Helper()
	k := filepath.Join(root, "kanban")
	if err := os.Rename(filepath.Join(k, "tasks"), filepath.Join(k, "issues")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(k, "issues"))
	for _, e := range entries {
		d := filepath.Join(k, "issues", e.Name())
		if err := os.Rename(filepath.Join(d, "task.yaml"), filepath.Join(d, "issue.yaml")); err != nil {
			t.Fatal(err)
		}
		sub := func(file, re, to string) {
			if b, err := os.ReadFile(file); err == nil {
				os.WriteFile(file, regexp.MustCompile(re).ReplaceAll(b, []byte(to)), 0o644)
			}
		}
		sub(filepath.Join(d, "library", "report.json"), `"to": *"task"`, `"to": "issue"`)
		qs, _ := filepath.Glob(filepath.Join(d, "library", "questions", "*.json"))
		for _, q := range qs {
			sub(q, `"from": *"task"`, `"from": "issue"`)
		}
	}
	cfg := filepath.Join(k, "config.yaml")
	b := readFile(t, cfg)
	if !strings.Contains(b, "\nversion: 2\n") && !strings.HasPrefix(b, "version: 2\n") {
		t.Fatalf("config.yaml is not version 2: %q", b)
	}
	put(t, cfg, regexp.MustCompile(`(?m)^version: 2$`).ReplaceAllString(b, "version: 3"))
}

// A copy of a format 2 ledger, moved by hand, passes lint and reads: list, show, questions.
// The log, brief and the person's comments stay as they were.
func TestFormat2MovedByHandPassesLint(t *testing.T) {
	root := v2Ledger(t)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("HC_TERMINAL_ID", "")
	log := readFile(t, filepath.Join(root, "kanban", "log.jsonl"))
	moveByHand(t, root)
	if out := kanban(t, root, "lint"); out != "ok (2 issues)\n" {
		t.Fatalf("lint: %q", out)
	}
	if out := kanban(t, root, "list", "--all"); !strings.Contains(out, "A-0001") || !strings.Contains(out, "A-0002") {
		t.Fatalf("list: %q", out)
	}
	if out := kanban(t, root, "show", "A-0002", "--json"); !strings.Contains(out, `"issue": {`) || !strings.Contains(out, `"text": "which?"`) {
		t.Fatalf("show: %s", out)
	}
	if out := kanban(t, root, "ready"); !strings.Contains(out, "A-0002") {
		t.Fatalf("ready: %q", out)
	}
	assertUnchanged(t, filepath.Join(root, "kanban", "config.yaml"), "# by hand\nversion: 3\nprefix: A\nrepos: []\n")
	assertUnchanged(t, filepath.Join(root, "kanban", "issues", "A-0001", "issue.yaml"), "id: A-0001\ntitle: one # kept\nstatus: done\n")
	assertUnchanged(t, filepath.Join(root, "kanban", "issues", "A-0001", "library", "report.json"), `{"next": [{"to": "issue", "text": "x"}]}`+"\n")
	assertUnchanged(t, filepath.Join(root, "kanban", "issues", "A-0002", "library", "questions", "1.json"), `{"kind": "scope", "question": "which?", "options": [{"label": "a"}], "from": "issue"}`+"\n")
	if out := kanban(t, root, "add", "three"); out != "A-0003\n" {
		t.Fatalf("add after the move: %q", out)
	}
	if got := readFile(t, filepath.Join(root, "kanban", "log.jsonl")); !strings.HasPrefix(got, log) {
		t.Fatalf("log.jsonl's old lines changed:\n%s", got)
	}
}
