package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// The schemas in schema/ledger/ are derived from docs/ledger.md. These tests check that
// what kanban writes fits them, with a small checker for the parts the schemas use (no library).

func loadSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "schema", "ledger", name))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// check returns the ways v doesn't fit schema s (type, const, enum, not, anyOf, pattern,
// minimum, required, properties, items, additionalProperties: false).
func check(s map[string]any, v any, at string) []string {
	var bad []string
	if m, ok := v.(map[string]any); ok {
		for _, r := range asList(s["required"]) {
			if _, ok := m[r.(string)]; !ok {
				bad = append(bad, fmt.Sprintf("%s: %s is required", at, r))
			}
		}
	}
	if alts := asList(s["anyOf"]); alts != nil && !slices.ContainsFunc(alts, func(x any) bool {
		return len(check(x.(map[string]any), v, at)) == 0
	}) {
		bad = append(bad, fmt.Sprintf("%s: fits none of anyOf", at))
	}
	switch s["type"] {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return []string{at + ": not an object"}
		}
		props, _ := s["properties"].(map[string]any)
		for k, x := range m {
			if p, ok := props[k].(map[string]any); ok {
				bad = append(bad, check(p, x, at+"."+k)...)
			} else if s["additionalProperties"] == false {
				bad = append(bad, fmt.Sprintf("%s: unknown key %s", at, k))
			}
		}
	case "array":
		list, ok := v.([]any)
		if !ok {
			return []string{at + ": not an array"}
		}
		if items, ok := s["items"].(map[string]any); ok {
			for i, x := range list {
				bad = append(bad, check(items, x, fmt.Sprintf("%s[%d]", at, i))...)
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			return []string{fmt.Sprintf("%s: %v is not a string", at, v)}
		}
		if p, ok := s["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(str) {
			bad = append(bad, fmt.Sprintf("%s: %q doesn't match %s", at, str, p))
		}
	case "integer":
		n, ok := v.(int)
		if !ok {
			return []string{fmt.Sprintf("%s: %v is not an integer", at, v)}
		}
		if min, ok := s["minimum"].(float64); ok && float64(n) < min {
			bad = append(bad, fmt.Sprintf("%s: %d is less than %v", at, n, min))
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return []string{fmt.Sprintf("%s: %v is not a boolean", at, v)}
		}
	}
	if c, ok := s["const"]; ok && fmt.Sprint(c) != fmt.Sprint(v) {
		bad = append(bad, fmt.Sprintf("%s: %v, want %v", at, v, c))
	}
	if e := asList(s["enum"]); e != nil && !slices.Contains(e, v) {
		bad = append(bad, fmt.Sprintf("%s: %v is not one of %v", at, v, e))
	}
	if n, ok := s["not"].(map[string]any); ok && len(check(n, v, at)) == 0 {
		bad = append(bad, fmt.Sprintf("%s: %v is not allowed", at, v))
	}
	return bad
}

func asList(v any) []any { l, _ := v.([]any); return l }

func readYAML(t *testing.T, file string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return m
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestWhatKanbanWritesFitsTheSchemas(t *testing.T) {
	cfgSchema, taskSchema := loadSchema(t, "config.v3.schema.json"), loadSchema(t, "issue.v3.schema.json")
	root := filepath.Join(t.TempDir(), "root")
	t.Setenv("KANBAN_ROOT", root)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, t.TempDir(), "init", "--root", root, "--prefix", "FD", "--repo", "github.com/x/app")
	kanban(t, root, "add", "Epic")
	kanban(t, root, "add", "Work", "--parent", "FD-0001", "--phase", "P1", "--profile", "opus", "--repo", "github.com/x/app")
	kanban(t, root, "add", "Later", "--after", "FD-0002")
	kanban(t, root, "set", "FD-0002", "links=https://a.example,docs/x.md", "branch=work/2", "worktree=~/wt/2")
	kanban(t, root, "start", "FD-0002")
	kanban(t, root, "set", "FD-0003", "who=person")
	kanban(t, root, "config", "set", "chief_of_staff=t2")
	os.MkdirAll(filepath.Join(root, "kanban", "routines"), 0o755)
	os.WriteFile(filepath.Join(root, "kanban", "routines", "after.yaml"), []byte("title: After\nbrief: Look.\nprofile: sonnet\nrepo: github.com/x/app\non: [done]\n"), 0o644)
	kanban(t, root, "done", "FD-0002") // the routine adds FD-0004

	cfg := readYAML(t, filepath.Join(root, "kanban", "config.yaml"))
	for _, p := range check(cfgSchema, cfg, "config.yaml") {
		t.Error(p)
	}
	// kanban writes only keys the schema describes.
	cfgProps, _ := cfgSchema["properties"].(map[string]any)
	for _, k := range keysOf(cfg) {
		if cfgProps[k] == nil {
			t.Errorf("config.yaml: kanban wrote %s, which the schema doesn't describe", k)
		}
	}
	taskProps, _ := taskSchema["properties"].(map[string]any)
	if !exists(taskFile(root, "FD-0004")) {
		t.Fatal("the routine added no task")
	}
	for _, id := range []string{"FD-0001", "FD-0002", "FD-0003", "FD-0004"} {
		task := readYAML(t, taskFile(root, id))
		for _, p := range check(taskSchema, task, id) {
			t.Error(p)
		}
		for _, k := range keysOf(task) {
			if taskProps[k] == nil {
				t.Errorf("%s: kanban wrote %s, which the schema doesn't describe", id, k)
			}
		}
	}
	// The task schema describes exactly the keys kanban knows (keyOrder), and nothing is required
	// but config.yaml's version.
	if got, want := keysOf(taskProps), slices.Sorted(slices.Values(keyOrder)); !slices.Equal(got, want) {
		t.Errorf("task schema keys %v, kanban's keys %v", got, want)
	}
	if r := asList(cfgSchema["required"]); len(r) != 1 || r[0] != "version" || taskSchema["required"] != nil {
		t.Errorf("required: config %v, task %v", r, taskSchema["required"])
	}
}

func TestSchemasRefuseWhatTheFormatRefuses(t *testing.T) {
	cfgSchema, taskSchema := loadSchema(t, "config.v3.schema.json"), loadSchema(t, "issue.v3.schema.json")
	routineSchema := loadSchema(t, "routine.v3.schema.json")
	for _, c := range []struct {
		schema map[string]any
		yaml   string
		ok     bool
	}{
		{cfgSchema, "version: 3\nunknown: kept\n", true},
		{cfgSchema, "prefix: W\nprojects: {app: projects/app}\n", false},
		{cfgSchema, "version: 1\n", false},
		{cfgSchema, "version: '3'\n", false},
		{cfgSchema, "version: 3\nprefix: a/b\n", false},
		{cfgSchema, "version: 3\nprefix: '..'\n", false},
		{cfgSchema, "version: 3\nrepos: github.com/x/app\n", false},
		{cfgSchema, "version: 3\nrepos: [github.com/x/app]\n", true},
		{cfgSchema, "version: 3\nrepos: [github.com/x/app/]\n", false},
		{cfgSchema, "version: 3\nrepos: [/github.com/x/app]\n", false},
		{cfgSchema, "version: 3\nrepos: [x/app]\n", false},
		{taskSchema, "title: x\nnotes: kept\n", true},
		{taskSchema, "id: ../W-0001\n", false},
		{taskSchema, "after: [W-0001, '.']\n", false},
		{taskSchema, "created: 2026-11-02\n", false},
		{taskSchema, "created: '2026-11-02T10:00:00Z'\n", true},
		{taskSchema, "routine: triage\n", true},
		{taskSchema, "routine: ../x\n", false},
		{taskSchema, "who: person\n", true},
		{taskSchema, "who: agent\n", false},
		{taskSchema, "not_before: '2026-12-01'\n", true},
		{taskSchema, "not_before: soon\n", false},
		{taskSchema, "not_before: '2026-12-01T00:00:00Z'\n", false},
		{taskSchema, "session: 0f9d2c4e-1b3a-4c5d-8e7f-a1b2c3d4e5f6\n", true},
		{taskSchema, "session: 0F9D2C4E-1B3A-4C5D-8E7F-A1B2C3D4E5F6\n", false},
		{taskSchema, "session: latest\n", false},
		{cfgSchema, "version: 3\ntimezone: Asia/Tokyo\n", true},
		{routineSchema, "title: T\nbrief: B\nprofile: p\nrepo: github.com/x/app\nschedule: '0 9 * * 1-5'\non: [done]\nopen: 1\npaused: false\nmine: kept\n", true},
		{routineSchema, "title: T\non: [done]\n", true},
		{routineSchema, "title: T\n", false},
		{routineSchema, "schedule: '* * * * *'\n", false},
		{routineSchema, "title: T\nschedule: '* * * *'\n", false},
		{routineSchema, "title: T\non: [done]\nopen: 0\n", false},
		{routineSchema, "title: T\non: [done]\npaused: 'yes'\n", false},
		{routineSchema, "title: T\non: [done]\nrepo: x/app\n", false},
	} {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(c.yaml), &m); err != nil {
			t.Fatal(err)
		}
		if bad := check(c.schema, m, "doc"); (len(bad) == 0) != c.ok {
			t.Errorf("%q: ok=%v, got %s", c.yaml, c.ok, strings.Join(bad, "; "))
		}
	}
}

func TestQuestionsFitTheSchema(t *testing.T) {
	schema := loadSchema(t, "questions.v3.schema.json")
	root := ledger(t)
	t.Setenv("HC_TERMINAL_ID", "t3")
	kanban(t, root, "ask", "W-0001", "one", "--kind", "irreversible")
	kanban(t, root, "ask", "W-0001", "two")
	kanban(t, root, "ask", "W-0001", "three", "--kind", "line")
	kanban(t, root, "answer", "W-0001#2", "yes")
	kanban(t, root, "answer", "W-0001#3", "no")
	kanban(t, root, "received", "W-0001#3")
	data, _ := os.ReadFile(filepath.Join(taskDir(root, "W-0001"), "questions.yaml"))
	var written any
	if err := yaml.Unmarshal(data, &written); err != nil {
		t.Fatal(err)
	}
	for _, p := range check(schema, written, "questions.yaml") {
		t.Error(p)
	}
	// The schema describes exactly the keys kanban knows.
	props := schema["items"].(map[string]any)["properties"].(map[string]any)
	if got, want := keysOf(props), slices.Sorted(slices.Values(questionKeys)); !slices.Equal(got, want) {
		t.Errorf("questions schema keys %v, kanban's keys %v", got, want)
	}
	for _, c := range []struct {
		yaml string
		ok   bool
	}{
		{"- n: 1\n  text: x\n  status: open\n  asked: '2026-11-03T01:20:00Z'\n  mine: kept\n", true},
		{"[]\n", true},
		{"n: 1\ntext: x\n", false},
		{"- text: x\n", false},
		{"- n: 1\n", false},
		{"- n: 0\n  text: x\n", false},
		{"- n: '1'\n  text: x\n", false},
		{"- n: 1\n  text: x\n  status: closed\n", false},
		{"- n: 1\n  text: x\n  kind: money\n", false},
		{"- n: 1\n  text: x\n  asked: 2026-11-03\n", false},
	} {
		var v any
		if err := yaml.Unmarshal([]byte(c.yaml), &v); err != nil {
			t.Fatal(err)
		}
		if bad := check(schema, v, "doc"); (len(bad) == 0) != c.ok {
			t.Errorf("%q: ok=%v, got %s", c.yaml, c.ok, strings.Join(bad, "; "))
		}
	}
}

// kanban lint and the routine schema say the same about a routine: a key written with another
// type than the schema's (null included) is a problem, and a routine with a problem doesn't fire.
func TestLintAgreesWithTheRoutineSchema(t *testing.T) {
	schema := loadSchema(t, "routine.v3.schema.json")
	cfg, err := readConfig(routineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		"title: T\nbrief: B\nprofile: p\nrepo: github.com/x/app\nschedule: '0 9 * * 1-5'\non: [done]\nopen: 2\npaused: false\nmine: kept\n",
		"title: T\non: [done, review]\n",
		"title: T\non: [done]\npaused: true\n",
		"title: T\non: [done]\npaused: null\n",
		"title: T\non: [done]\npaused:\n",
		"title: T\non: [done]\npaused: 'true'\n",
		"title: T\non: done\n",
		"title: T\non: [done, [review]]\n",
		"title: T\nschedule: '0 9 * * *'\non: null\n",
		"title: T\non: [done]\nopen: '2'\n",
		"title: T\non: [done]\nopen: null\n",
		"title: T\non: [done]\nopen: 1.5\n",
		"title: T\non: [done]\nschedule: null\n",
		"title: T\non: [done]\nbrief: [a]\n",
		"title: T\non: [done]\nbrief: null\n",
		"title: T\non: [done]\nprofile: 3\n",
		"title: 2026\non: [done]\n",
	} {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(body), &m); err != nil {
			t.Fatal(err)
		}
		d, err := parseDoc([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		bad := check(schema, m, "doc")
		problems := (&routine{name: "r", d: d}).problems(cfg)
		if (len(bad) == 0) != (len(problems) == 0) {
			t.Errorf("%q: schema %v, lint %v", body, bad, problems)
		}
	}
}
