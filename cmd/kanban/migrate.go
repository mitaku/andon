package main

import (
	"encoding/json"

	"andon/internal/delegate"
	"andon/internal/ledger"
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
// Chief of Staff and the dispatcher stopped. The ledger stays format 2 (tasks/, task.yaml):
// the person then moves it to format 3 by hand (docs/operate.md); kanban migrate doesn't.

const oldLedgerName = ".flightdeck"

const oldFormat = 2

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
	if isFormat2(root) {
		return fail(errFormat2)
	}
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
	for _, p := range []string{filepath.Join(old, "coordinator.md"), filepath.Join(root, "AGENTS.md")} {
		if err := checkNoLink(p, true); err != nil {
			return err
		}
	}
	slug := filepath.Base(root)
	if _, _, err := readDispatcherState(slug); err != nil {
		return err
	}
	draft, signals, err := projectDraft(root, old)
	if err != nil {
		return err
	}
	var moved map[string]string
	err = lockIn(old, lockWait, func() error {
		var err error
		moved, err = migrateIn(root, old)
		return err
	})
	if err != nil {
		return err
	}
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
	state, err := migrateDispatcherState(slug, moved)
	if err != nil {
		return err
	}
	project := ""
	if draft != nil {
		project = filepath.Join(root, "project.json")
		if err := writeAtomic(project, draft); err != nil {
			return err
		}
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

	lines := []string{fmt.Sprintf("moved %s to %s (%d ids renamed)", old, dir, len(moved)),
		fmt.Sprintf("it is format %d: move it to format %d by hand (docs/operate.md)", oldFormat, formatVersion)}
	if agents != "" {
		lines = append(lines, "coordinator.md is now the Chief of Staff section of "+agents)
	}
	if state != "" {
		lines = append(lines, "dispatcher state renamed ids: "+state)
	}
	if project != "" {
		lines = append(lines, "wrote a draft of "+project+" (not committed): check it, commit it as yours, then andon delegate show")
		if signals {
			lines = append(lines, "the draft's signals read main's CI failures and tell the Chief of Staff now; remove them before committing if the project doesn't want that")
		}
	}
	o := &omap{}
	o.set("root", root)
	o.set("ids", ids)
	o.set("agents", agents)
	o.set("dispatcherState", state)
	o.set("projectDraft", project)
	e.out(a, o, strings.Join(lines, "\n"))
	return nil
}

func migrateIn(root, old string) (map[string]string, error) {
	cfgFile := filepath.Join(old, "config.yaml")
	cfg, err := readDoc(cfgFile)
	if err != nil {
		return nil, fail("%s: %v", cfgFile, err)
	}
	if v := cfg.get("version"); !isInt(v) || v.Value != fmt.Sprint(oldFormat) {
		return nil, fail("%s is not format %d; kanban migrate moves format %d only", cfgFile, oldFormat, oldFormat)
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
		// A link is refused before IsDir, which is false for a link to a folder.
		if err := checkNoLink(filepath.Join(tasksAt, en.Name()), false); err != nil {
			return nil, err
		}
		if !en.IsDir() {
			continue
		}
		file := filepath.Join(tasksAt, en.Name(), "task.yaml")
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
		n := ledger.CanonID(it.id)
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
	for _, it := range items {
		before := string(it.d.encode())
		if v := it.d.str("id"); v != "" {
			it.d.setStr("id", ledger.CanonID(v))
		}
		if v := it.d.str("parent"); v != "" {
			it.d.setStr("parent", ledger.CanonID(v))
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

// projectDraft is the project.json to write when the project has none (nil when it has one):
// version 1, project (config.yaml title, else the folder's name; prefix), repos (config.yaml),
// delegation (delegation.json as it is) and, when there is a repo, signals with one line that
// reads main's CI failures now, as andon create's draft has (ADR-0051, 4); signals says it has
// that line. The shape is Andon's schema/project.schema.json;
// andon delegate show checks it once the person has committed it.
func projectDraft(root, old string) ([]byte, bool, error) {
	to := filepath.Join(root, "project.json")
	if err := checkNoLink(to, true); err != nil {
		return nil, false, err
	}
	if exists(to) {
		return nil, false, nil
	}
	cfg, err := readDoc(filepath.Join(old, "config.yaml"))
	if err != nil {
		return nil, false, err
	}
	title := cfg.str("title")
	if title == "" {
		title = filepath.Base(root)
	}
	prefix := cfg.str("prefix")
	if prefix == "" {
		prefix = "W"
	}
	repos := cfg.list("repos")
	if repos == nil {
		repos = []string{}
	}
	var delegation json.RawMessage
	from := filepath.Join(root, "delegation.json")
	if err := checkNoLink(from, true); err != nil {
		return nil, false, err
	}
	switch b, err := os.ReadFile(from); {
	case os.IsNotExist(err):
	case err != nil:
		return nil, false, err
	default:
		if !json.Valid(b) {
			return nil, false, fail("not moved: %s is not JSON; fix it first (it becomes project.json's delegation)", from)
		}
		delegation = json.RawMessage(b)
	}
	var signals []delegate.Signal
	if len(repos) > 0 {
		signals = []delegate.Signal{{Source: "github-ci", Weight: "now"}}
	}
	type info struct {
		Title  string `json:"title"`
		Prefix string `json:"prefix"`
	}
	out, err := json.MarshalIndent(struct {
		Version    int               `json:"version"`
		Project    info              `json:"project"`
		Repos      []string          `json:"repos"`
		Delegation json.RawMessage   `json:"delegation,omitempty"`
		Signals    []delegate.Signal `json:"signals,omitempty"`
	}{1, info{title, prefix}, repos, delegation, signals}, "", "  ")
	if err != nil {
		return nil, false, err
	}
	if _, err := delegate.ParseProject(out, filepath.Base(root)); err != nil {
		return nil, false, fail("not moved: the project.json drafted from config.yaml and delegation.json can't be read by Andon (%v); fix them first, or write project.json yourself", err)
	}
	return append(out, '\n'), signals != nil, nil
}

func readDispatcherState(slug string) ([]byte, any, error) {
	file := dispatcherStatePath(slug)
	if err := checkNoLink(file, true); err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, nil, nil
	} else if err != nil {
		return nil, nil, err
	}
	var v any
	if json.Unmarshal(data, &v) != nil || json.Unmarshal(data, new(dispatcherState)) != nil {
		return nil, nil, fail("not moved: %s is not the dispatcher's state; fix or remove it first", file)
	}
	return data, v, nil
}

func dispatcherStatePath(slug string) string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "andon", "dispatcher", slug+".json")
}

func migrateDispatcherState(slug string, moved map[string]string) (string, error) {
	file := dispatcherStatePath(slug)
	if len(moved) == 0 {
		return "", nil
	}
	data, v, err := readDispatcherState(slug)
	if err != nil || data == nil {
		return "", err
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
	slices.SortFunc(keys, func(a, b string) int { return ledger.NumberOf(a) - ledger.NumberOf(b) })
	return keys
}
