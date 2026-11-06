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
)

// kanban migrate moves a format 2 ledger from its old folder, .flightdeck/, to kanban/
// (docs/ledger.md, Moving from .flightdeck/; docs/operate.md). Once, by a person, with the
// Chief of Staff and the dispatcher stopped:
//
//   - .flightdeck/ becomes kanban/ (its .gitignore, *, keeps it out of git as before);
//   - config.yaml's coordinator becomes chief_of_staff;
//   - task ids get four digits (A-5 → A-0005): the folders, and id, parent and after in task.yaml;
//   - .flightdeck/coordinator.md becomes the Chief of Staff's section of the project's AGENTS.md;
//   - the dispatcher's state for the project (~/.local/state/andon/dispatcher/<slug>.json)
//     names the tasks by their new ids.
//
// log.jsonl, reports and briefs keep the ids they were written with: kanban and Andon read
// A-5 as A-0005. A task in doing or review stops the move, before anything changes: its
// worktree, branch and terminal carry its id.

const oldLedgerName = ".flightdeck"

func cmdMigrate(e env, a args) error {
	root := a.flags["root"]
	if root == "" {
		root = os.Getenv("KANBAN_ROOT")
	}
	if root == "" {
		root = e.cwd
	}
	root, _ = filepath.Abs(root)
	old, dir := filepath.Join(root, oldLedgerName), ledgerDir(root)
	if !exists(filepath.Join(old, "config.yaml")) {
		return fail("nothing to move: no %s/config.yaml in %s", oldLedgerName, root)
	}
	if exists(dir) {
		return fail("%s is already there; move by hand or remove it first", dir)
	}
	for _, p := range []string{old, filepath.Join(old, "tasks")} {
		if err := checkNoLink(p, false); err != nil {
			return err
		}
	}
	var moved map[string]string
	err := lockIn(old, lockWait, func() error {
		var err error
		moved, err = migrateIn(root, old)
		return err
	})
	if err != nil {
		return err
	}
	// The lock goes with the process; the file moved with the folder.
	if err := os.Rename(old, dir); err != nil {
		return fail("can't move %s to %s: %v", old, dir, err)
	}
	if ignore := filepath.Join(dir, ".gitignore"); !exists(ignore) {
		if err := writeAtomic(ignore, []byte(initialGitignore)); err != nil {
			return err
		}
	}
	agents, err := moveCoordinatorMD(root)
	if err != nil {
		return err
	}
	state, err := migrateDispatcherState(filepath.Base(root), moved)
	if err != nil {
		return err
	}
	entry := &omap{}
	entry.set("at", now())
	entry.set("by", who())
	m := &omap{}
	m.set("from", oldLedgerName)
	ids := &omap{}
	for _, k := range sortedKeys(moved) {
		ids.set(k, moved[k])
	}
	m.set("ids", ids)
	entry.set("migrate", m)
	if err := appendEntry(root, entry); err != nil {
		return err
	}

	lines := []string{fmt.Sprintf("moved %s to %s (%d ids renamed)", old, dir, len(moved))}
	if agents != "" {
		lines = append(lines, "coordinator.md is now the Chief of Staff section of "+agents)
	}
	if state != "" {
		lines = append(lines, "dispatcher state renamed ids: "+state)
	}
	o := &omap{}
	o.set("root", root)
	o.set("ids", ids)
	o.set("agents", agents)
	o.set("dispatcherState", state)
	e.out(a, o, strings.Join(lines, "\n"))
	return nil
}

// migrateIn checks the old ledger and changes it in place: config.yaml and the tasks.
// It returns old id → new id for the ids that change. Nothing is written when a check fails.
func migrateIn(root, old string) (map[string]string, error) {
	cfgFile := filepath.Join(old, "config.yaml")
	cfg, err := readDoc(cfgFile)
	if err != nil {
		return nil, fail("%s: %v", cfgFile, err)
	}
	if v := cfg.get("version"); isNull(v) || v.Value != fmt.Sprint(formatVersion) {
		return nil, fail("%s is not format %d; kanban migrate moves format %d only", cfgFile, formatVersion, formatVersion)
	}
	if !isNull(cfg.get("chief_of_staff")) {
		return nil, fail("%s already has chief_of_staff", cfgFile)
	}
	tasksAt := filepath.Join(old, "tasks")
	entries, err := os.ReadDir(tasksAt)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	type item struct {
		id   string
		file string
		d    *doc
	}
	var items []item
	var busy, broken []string
	for _, en := range entries {
		if !en.IsDir() {
			continue
		}
		file := filepath.Join(tasksAt, en.Name(), "task.yaml")
		if err := checkNoLink(filepath.Join(tasksAt, en.Name()), false); err != nil {
			return nil, err
		}
		if err := checkNoLink(file, true); err != nil {
			return nil, err
		}
		d, err := readDoc(file)
		if err != nil {
			broken = append(broken, en.Name())
			continue
		}
		if s := d.str("status"); s == "doing" || s == "review" {
			busy = append(busy, en.Name()+" ("+s+")")
		}
		items = append(items, item{en.Name(), file, d})
	}
	if len(busy) > 0 {
		sort.Strings(busy)
		return nil, fail("not moved: %s still in doing or review; their worktree, branch and terminal carry the id. Finish them first", strings.Join(busy, ", "))
	}
	if len(broken) > 0 {
		sort.Strings(broken)
		return nil, fail("not moved: task.yaml unreadable in %s (kanban lint)", strings.Join(broken, ", "))
	}
	moved := map[string]string{}
	taken := map[string]bool{}
	for _, it := range items {
		n := canonID(it.id)
		if taken[n] {
			return nil, fail("not moved: two tasks become %s", n)
		}
		taken[n] = true
		if n != it.id {
			if err := checkSegment("task id", n); err != nil {
				return nil, err
			}
			moved[it.id] = n
		}
	}
	// Write: task.yaml first (in their old folders), then the folders, then config.yaml.
	for _, it := range items {
		before := string(it.d.encode())
		if v := it.d.str("id"); v != "" {
			it.d.setStr("id", canonID(v))
		}
		if v := it.d.str("parent"); v != "" {
			it.d.setStr("parent", canonID(v))
		}
		if n := it.d.get("after"); n != nil && !isNull(n) {
			it.d.setList("after", canonIDs(it.d.list("after")))
		}
		if string(it.d.encode()) != before {
			if err := writeAtomic(it.file, it.d.encode()); err != nil {
				return nil, err
			}
		}
	}
	for _, from := range sortedKeys(moved) {
		if err := os.Rename(filepath.Join(tasksAt, from), filepath.Join(tasksAt, moved[from])); err != nil {
			return nil, fail("can't rename tasks/%s: %v", from, err)
		}
	}
	if i := cfg.index("coordinator"); i >= 0 {
		cfg.m.Content[i].Value = "chief_of_staff"
		if err := writeAtomic(cfgFile, cfg.encode()); err != nil {
			return nil, err
		}
	}
	return moved, nil
}

// moveCoordinatorMD appends the old ledger's coordinator.md to the project's AGENTS.md
// under "## Chief of Staff" (ADR-0004, 5.3) and removes it.
// It returns AGENTS.md's path, or "" when there was no coordinator.md.
func moveCoordinatorMD(root string) (string, error) {
	from := filepath.Join(ledgerDir(root), "coordinator.md")
	if err := checkNoLink(from, true); err != nil {
		return "", err
	}
	body, err := os.ReadFile(from)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	to := filepath.Join(root, "AGENTS.md")
	if err := checkNoLink(to, true); err != nil {
		return "", err
	}
	text, err := os.ReadFile(to)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var b strings.Builder
	if len(text) == 0 {
		b.WriteString("# " + filepath.Base(root) + "\n")
	} else {
		b.Write(text)
		if !strings.HasSuffix(string(text), "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Chief of Staff\n\n")
	b.WriteString(strings.TrimRight(string(body), " \t\r\n") + "\n")
	if err := writeAtomic(to, []byte(b.String())); err != nil {
		return "", err
	}
	return to, os.Remove(from)
}

// dispatcherStatePath is Andon's dispatcher's state for the project slug (internal/dispatcher, StatePath).
func dispatcherStatePath(slug string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "dispatcher", slug+".json")
}

// migrateDispatcherState renames the moved ids in the dispatcher's state: any key or string
// that is an id, or an id followed by #, @ or | (the state's keys are <task>#<row>,
// <task>@<commit> and <task>|<agent>|…). It returns the file's path when it changed it.
func migrateDispatcherState(slug string, moved map[string]string) (string, error) {
	file := dispatcherStatePath(slug)
	if len(moved) == 0 {
		return "", nil
	}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return "", fail("%s is not JSON; fix or remove it, then start the dispatcher again", file)
	}
	re := regexp.MustCompile(`^([^#@|]+)([#@|].*)?$`)
	rename := func(s string) string {
		m := re.FindStringSubmatch(s)
		if m == nil {
			return s
		}
		if to, ok := moved[m[1]]; ok {
			return to + m[2]
		}
		return s
	}
	var walk func(any) any
	walk = func(x any) any {
		switch t := x.(type) {
		case string:
			return rename(t)
		case []any:
			for i := range t {
				t[i] = walk(t[i])
			}
			return t
		case map[string]any:
			out := make(map[string]any, len(t))
			for k, val := range t {
				out[rename(k)] = walk(val)
			}
			return out
		}
		return x
	}
	out, err := json.MarshalIndent(walk(v), "", "  ")
	if err != nil {
		return "", err
	}
	out = append(out, '\n')
	if string(out) == string(data) {
		return "", nil
	}
	return file, writeAtomic(file, out)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int { return numberOf(a) - numberOf(b) })
	return keys
}
