package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"andon/internal/ledger/ledgertest"
)

// log.jsonl の行と kanban ready --json の出力の形は schema/ledger/ の log.v3・ready.v3（ADR-0034 決めたこと 7）。
// 本物の kanban の CLI に全部の形の行を書かせ、全部の行と ready の出力をその schema に通す。照らすのは、Go のテストが
// 手で書く kanban を通す補助（internal/ledger/ledgertest）と同じ既製の検査器。

// lineShape は log.jsonl の1行の形の名前（どの形の行を書かせたかを数える）。
func lineShape(t *testing.T, line string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("log.jsonl の行が JSON でない: %s", line)
	}
	switch {
	case m["migrate"] != nil:
		return "migrate"
	case m["config"] != nil:
		return "config"
	case m["routine"] != nil:
		if s, ok := m["skipped"].(string); ok {
			return "routine skipped " + s
		}
		return "routine fired"
	}
	change, _ := m["change"].(map[string]any)
	keys := make([]string, 0, len(change))
	for k, v := range change {
		if v == nil {
			k += "=null"
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "change " + strings.Join(keys, " ")
}

func TestLogAndReadyFitTheLedgerSchemas(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "t3")
	t.Setenv("DECK_NOW", "2026-11-02T08:00:00Z")
	kanban(t, t.TempDir(), "init", "--root", root, "--prefix", "W", "--repo", "github.com/x/app")
	kanban(t, root, "add", "Epic")                                                                            // W-0001
	kanban(t, root, "add", "Work", "--parent", "W-0001", "--profile", "claude", "--repo", "github.com/x/app") // W-0002
	kanban(t, root, "add", "Later", "--after", "W-0002")                                                      // W-0003, queued
	kanban(t, root, "add", "Rewired", "--after", "W-0002")                                                    // W-0004, queued
	kanban(t, root, "add", "Decide", "--status", "todo")                                                      // W-0005
	kanban(t, root, "add", "Review of work", "--parent", "W-0002", "--profile", "codex-review")               // W-0002-r1
	kanban(t, root, "set", "W-0002", "links=https://a.example,docs/x.md", "branch=work/2", "worktree=~/wt/2", "phase=P1")
	kanban(t, root, "set", "W-0002", "phase=")
	kanban(t, root, "set", "W-0004", "after=")
	kanban(t, root, "set", "W-0005", "who=person", "not_before=2026-11-01", "session=0f9d2c4e-1b3a-4c5d-8e7f-a1b2c3d4e5f6")
	kanban(t, root, "start", "W-0002")
	kanbanIn(t, root, "done so far\n", "report", "W-0002")
	kanbanIn(t, root, "and more\n", "report", "W-0002", "--append")
	kanban(t, root, "ask", "W-0002", "one", "--kind", "irreversible")
	kanban(t, root, "ask", "W-0002", "two")
	kanban(t, root, "answer", "W-0002#1", "yes")
	kanban(t, root, "received", "W-0002#1")
	kanban(t, root, "review", "W-0002", "--force") // #2 is not received
	kanban(t, root, "config", "set", "chief_of_staff=t2")
	kanban(t, root, "config", "set", "chief_of_staff=")
	kanban(t, root, "config", "set", "chief_of_staff=t2")
	dir := filepath.Join(root, "kanban", "routines")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "after.yaml"), []byte("title: After\nbrief: Look.\nprofile: sonnet\nrepo: github.com/x/app\non: [done]\nopen: 1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "stopped.yaml"), []byte("title: Stopped\non: [done]\npaused: true\n"), 0o644)
	kanban(t, root, "done", "W-0002", "--force") // frees W-0003; after adds W-0006, stopped is skipped
	kanban(t, root, "done", "W-0001")            // after is skipped: W-0006 is not done

	lines := strings.Split(strings.TrimSuffix(readFile(t, filepath.Join(root, "kanban", "log.jsonl")), "\n"), "\n")
	lines = append(lines, migratedLog(t)...)
	if err := ledgertest.CheckLog([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
		t.Errorf("kanban が書いた行:\n%v", err)
	}
	seen := map[string]bool{}
	for _, l := range lines {
		seen[lineShape(t, l)] = true
	}
	// 書かせた形（docs/ledger.md の log.jsonl の節の全部）。足りなければ、この検査が見ていない形がある。
	for _, want := range []string{
		"change add",
		"change after=null status",
		"change branch links phase worktree",
		"change phase=null",
		"change not_before session who",
		"change agent branch status",
		"change report",
		"change question status",
		"change forced status",
		"change status",
		"change freedBy status",
		"config",
		"routine fired",
		"routine skipped open",
		"routine skipped paused",
		"migrate",
	} {
		if !seen[want] {
			t.Errorf("kanban に %q の行を書かせていない（書いた形: %v）", want, slices.Sorted(func(yield func(string) bool) {
				for k := range seen {
					if !yield(k) {
						return
					}
				}
			}))
		}
	}

	// ready: オーナーの課題・レビュアーの子課題・ルーチンが足した課題・queued から出た課題が並ぶ。
	out := kanban(t, root, "ready", "--json")
	if err := ledgertest.Check(ledgertest.ReadySchema, []byte(out)); err != nil {
		t.Errorf("kanban ready --json: %v\n%s", err, out)
	}
	var ready []map[string]any
	if err := json.Unmarshal([]byte(out), &ready); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range ready {
		ids = append(ids, r["id"].(string))
	}
	if got := strings.Join(ids, " "); got != "W-0002-r1 W-0003 W-0004 W-0005 W-0006" {
		t.Errorf("ready: %s", got)
	}
	// 何も無いときは空の並び。
	empty := filepath.Join(t.TempDir(), "empty")
	kanban(t, t.TempDir(), "init", "--root", empty)
	if out := kanban(t, empty, "ready", "--json", "--root", empty); ledgertest.Check(ledgertest.ReadySchema, []byte(out)) != nil || strings.TrimSpace(out) != "[]" {
		t.Errorf("空の kanban の ready: %q", out)
	}
}

// kanban set は log の予約キー（add・report・question・forced・freedBy）を断り、行も issue.yaml も書かない。
// 断らなければ、change に question だけ（status 無し）などの schema/ledger/log.v3 に合わない行ができる。
func TestSetRefusesLogKeys(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("DECK_NOW", "2026-11-02T08:00:00Z")
	kanban(t, t.TempDir(), "init", "--root", root, "--prefix", "W")
	kanban(t, root, "add", "Work")
	log := filepath.Join(root, "kanban", "log.jsonl")
	before, yaml := readFile(t, log), readFile(t, taskFile(root, "W-0001"))
	for _, k := range []string{"add", "report", "question", "forced", "freedBy"} {
		for _, v := range []string{"x", ""} {
			msg := kanbanFails(t, root, "", "set", "W-0001", "title=Other", k+"="+v)
			if !strings.Contains(msg, k+" is a log.jsonl key") {
				t.Errorf("set %s=%s: %s", k, v, msg)
			}
		}
	}
	if readFile(t, log) != before || readFile(t, taskFile(root, "W-0001")) != yaml {
		t.Errorf("断った set が kanban を書いた:\n%s", readFile(t, log))
	}
	// 予約キーの外は今までどおり書け、その行は schema に合う。
	kanban(t, root, "set", "W-0001", "review=on", "added=x")
	if err := ledgertest.CheckLog([]byte(readFile(t, log))); err != nil {
		t.Error(err)
	}
}

// migratedLog は .flightdeck から移した kanban の log.jsonl の行（kanban migrate の行を含む）。
func migratedLog(t *testing.T) []string {
	t.Helper()
	ws := t.TempDir()
	root := filepath.Join(ws, "projects", "app")
	old := filepath.Join(root, ".flightdeck")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("KANBAN_ROOT", "")
	gitInit(t, ws)
	put(t, filepath.Join(old, "config.yaml"), "version: 2\nprefix: A\ncoordinator: t5\nrepos: []\n")
	put(t, filepath.Join(old, "tasks", "A-5", "task.yaml"), "id: A-5\ntitle: five\nstatus: todo\n")
	put(t, filepath.Join(old, "log.jsonl"), `{"at":"2026-11-01T00:00:00Z","by":"terminal:t5","id":"A-5","change":{"add":"five"}}`+"\n")
	kanban(t, root, "migrate")
	return strings.Split(strings.TrimSuffix(readFile(t, filepath.Join(root, "kanban", "log.jsonl")), "\n"), "\n")
}

// ready.v3 の課題の項目は issue.v3 の項目の写し（status だけ todo・queued に絞る）。issue.v3 を変えたら ready.v3 も直す。
func TestReadySchemaCopiesTheIssueSchema(t *testing.T) {
	read := func(name string) map[string]any {
		var m map[string]any
		if err := json.Unmarshal([]byte(readFile(t, filepath.Join(ledgertest.Dir(), name))), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	issue := read("issue.v3.schema.json")["properties"].(map[string]any)
	ready := read(ledgertest.ReadySchema)["$defs"].(map[string]any)["issue"].(map[string]any)["properties"].(map[string]any)
	status := ready["status"].(map[string]any)
	if !reflect.DeepEqual(status["enum"], []any{"todo", "queued"}) || status["type"] != "string" {
		t.Errorf("ready の status: %v", status)
	}
	delete(issue, "status")
	delete(ready, "status")
	if !reflect.DeepEqual(issue, ready) {
		for k := range issue {
			if !reflect.DeepEqual(issue[k], ready[k]) {
				t.Errorf("ready.v3 の %s が issue.v3 と違う:\n issue %v\n ready %v", k, issue[k], ready[k])
			}
		}
		for k := range ready {
			if issue[k] == nil {
				t.Errorf("ready.v3 に issue.v3 に無い %s がある", k)
			}
		}
	}
}
