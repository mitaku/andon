package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // IANA time zones on machines without them (Windows)

	"go.yaml.in/yaml/v3"
)

// Routines (docs/ledger.md "Routines", docs/decisions.md §6): routines/<name>.yaml says
// what task to add and when. kanban tick fires the time triggers; the commands that move a
// task into a status fire the event triggers. kanban only reads routine files.

const timeLayout = "2006-01-02T15:04:05Z"

type routine struct {
	name   string
	d      *doc
	broken string // why the file can't be read
}

func routinesDir(root string) string { return filepath.Join(ledgerDir(root), "routines") }
func statePath(root string) string {
	return filepath.Join(ledgerDir(root), "state", "routines.json")
}

// readRoutines reads routines/*.yaml, by name. A routines/ that is a symbolic link is
// refused; a routine file that is a link, or can't be read, comes back broken.
func readRoutines(root string) ([]*routine, error) {
	dir := routinesDir(root)
	if err := checkNoLink(dir, false); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var out []*routine
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || e.IsDir() {
			continue
		}
		r := &routine{name: name}
		file := filepath.Join(dir, e.Name())
		if err := checkNoLink(file, true); err != nil {
			r.broken = "is a symbolic link or not a file; kanban reads only inside kanban/"
		} else if r.d, err = readDoc(file); err != nil {
			r.broken = firstLine(err.Error())
		}
		out = append(out, r)
	}
	return out, nil
}

func (r *routine) schedule() string { return r.d.str("schedule") }
func (r *routine) on() []string     { return r.d.list("on") }

// paused is the routine's paused key, a YAML boolean; default false.
func (r *routine) paused() (bool, error) {
	n := r.d.get("paused")
	if isNull(n) {
		return false, nil
	}
	var v bool
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!bool" || n.Decode(&v) != nil {
		return false, fail("paused must be true or false")
	}
	return v, nil
}

// open is how many of its tasks may be not done; default 1.
func (r *routine) open() (int, error) {
	n := r.d.get("open")
	if isNull(n) {
		return 1, nil
	}
	v, err := strconv.Atoi(n.Value)
	if n.Kind != yaml.ScalarNode || err != nil || v < 1 {
		return 0, fail("open must be a whole number of 1 or more")
	}
	return v, nil
}

// problems are what keep a routine from firing (kanban lint reports them).
func (r *routine) problems(cfg config) []string {
	if err := checkSegment("name", r.name); err != nil {
		return []string{err.Error()}
	}
	if r.broken != "" {
		return []string{r.broken}
	}
	var out []string
	if r.d.str("title") == "" {
		out = append(out, "no title")
	}
	if isNull(r.d.get("schedule")) && isNull(r.d.get("on")) {
		out = append(out, "needs schedule or on")
	}
	if s := r.d.get("schedule"); !isNull(s) {
		if _, err := parseCron(r.schedule()); err != nil {
			out = append(out, "schedule: "+err.Error())
		}
	}
	if repo := r.d.str("repo"); repo != "" && !slices.Contains(cfg.repos, repo) {
		out = append(out, fmt.Sprintf("repo %s is not in config.yaml repos", repo))
	}
	if _, err := r.open(); err != nil {
		out = append(out, err.Error())
	}
	if _, err := r.paused(); err != nil {
		out = append(out, err.Error())
	}
	return out
}

// loadLocation is config.yaml's timezone, else UTC.
func loadLocation(cfg config) (*time.Location, error) {
	if cfg.timezone == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(cfg.timezone)
	if err != nil || cfg.timezone == "Local" {
		return nil, fail("timezone %q is not an IANA time zone", cfg.timezone)
	}
	return loc, nil
}

// firing is one command's work on routines, under the lock: the routines, the state
// (routine name -> UTC time of its last firing, or of when kanban first saw it), and what fired.
type firing struct {
	root    string
	cfg     config
	at      time.Time
	all     []*routine
	state   map[string]any
	changed bool
	done    []*omap // {routine, fired|skipped}
}

// startFiring reads the routines and the state before anything is written, so a link
// or a broken state file stops the command with nothing half done.
func startFiring(root string, cfg config) (*firing, error) {
	f := &firing{root: root, cfg: cfg, at: clock(), state: map[string]any{}}
	var err error
	if f.all, err = readRoutines(root); err != nil {
		return nil, err
	}
	if err := checkNoLink(filepath.Dir(statePath(root)), false); err != nil {
		return nil, err
	}
	if err := checkNoLink(statePath(root), true); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(statePath(root))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &f.state); err != nil || f.state == nil {
			return nil, fail("state/routines.json is not a JSON object; fix or remove it")
		}
	}
	return f, nil
}

func (f *firing) find(name string) *routine {
	for _, r := range f.all {
		if r.name == name {
			return r
		}
	}
	return nil
}

// since is the time a routine's schedule counts from. The first time kanban sees a routine,
// it notes now and returns false: that time doesn't fire.
func (f *firing) since(r *routine) (time.Time, bool) {
	if s, ok := f.state[r.name].(string); ok {
		if t, err := time.Parse(timeLayout, s); err == nil {
			return t, true
		}
	}
	f.note(r)
	return f.at, false
}

func (f *firing) note(r *routine) {
	f.state[r.name] = f.at.UTC().Format(timeLayout)
	f.changed = true
}

// notDone counts the tasks a routine added that aren't done.
func notDone(tasks []*task, cfg config, name string) int {
	n := 0
	for _, t := range tasks {
		if t.broken == "" && t.d.str("routine") == name && t.status() != cfg.done() {
			n++
		}
	}
	return n
}

// fire is one firing: skipped when paused or when open of its tasks are not done,
// else a task added as kanban add would. Either way it is logged and its time noted.
func (f *firing) fire(r *routine) error {
	result := &omap{}
	result.set("routine", r.name)
	open, _ := r.open()
	paused, _ := r.paused()
	switch {
	case paused:
		result.set("skipped", "paused")
	case notDone(allTasks(f.root), f.cfg, r.name) >= open:
		result.set("skipped", "open")
	default:
		fields := map[string]string{"profile": r.d.str("profile"), "repo": r.d.str("repo"), "routine": r.name}
		var brief *string
		if b := r.d.get("brief"); !isNull(b) {
			s := b.Value
			brief = &s
		}
		t, err := newTask(f.root, f.cfg, "", r.d.str("title"), fields, nil, f.cfg.columns[0], brief)
		if err != nil {
			return err
		}
		result.set("fired", t.id)
	}
	entry := &omap{}
	entry.set("at", f.at.UTC().Format(timeLayout))
	entry.set("by", "routine:"+r.name)
	for _, k := range result.keys {
		entry.set(k, result.vals[k])
	}
	if err := appendEntry(f.root, entry); err != nil {
		return err
	}
	f.note(r)
	f.done = append(f.done, result)
	return nil
}

// entered fires the routines whose on lists the status a task just moved into,
// except routines that added that task. Routines with problems don't fire.
func (f *firing) entered(t *task, status string) error {
	for _, r := range f.all {
		if r.d == nil || !slices.Contains(r.on(), status) || t.d.str("routine") == r.name || len(r.problems(f.cfg)) > 0 {
			continue
		}
		if err := f.fire(r); err != nil {
			return err
		}
	}
	return nil
}

func (f *firing) save() error {
	if !f.changed {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(statePath(f.root)), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(statePath(f.root), append(data, '\n'))
}

// lines are the firings for people: "<routine>: added W-5" or "<routine>: skipped (open)".
func (f *firing) lines() []string {
	var out []string
	for _, d := range f.done {
		if id, ok := d.vals["fired"]; ok {
			out = append(out, fmt.Sprintf("%s: added %s", d.vals["routine"], id))
		} else {
			out = append(out, fmt.Sprintf("%s: skipped (%s)", d.vals["routine"], d.vals["skipped"]))
		}
	}
	return out
}

// withEvents runs fn under the lock with a firing for the statuses it reports, then saves
// the state. A command that moves tasks calls entered for each move.
func withEvents(e env, cfg config, fn func(f *firing) error) (*firing, error) {
	var f *firing
	err := withLock(e.root, lockWait, func() error {
		var err error
		if f, err = startFiring(e.root, cfg); err != nil {
			return err
		}
		if err := fn(f); err != nil {
			return err
		}
		return f.save()
	})
	return f, err
}

// printFired tells a person which routines a move fired (not in --json, whose shape stays).
func printFired(e env, a args, f *firing) {
	if f != nil && !a.has("json") {
		for _, l := range f.lines() {
			e.println("routine " + l)
		}
	}
}

func cmdTick(e env, a args) error {
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	loc, err := loadLocation(cfg)
	if err != nil {
		return err
	}
	var bad []string
	f, err := withEvents(e, cfg, func(f *firing) error {
		for _, r := range f.all {
			if len(r.problems(cfg)) > 0 {
				bad = append(bad, r.name)
				continue
			}
			since, seen := f.since(r)
			if !seen || r.schedule() == "" {
				continue
			}
			s, _ := parseCron(r.schedule())
			if t, ok := s.next(since, loc); ok && !t.After(f.at) {
				if err := f.fire(r); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	text := strings.Join(f.lines(), "\n")
	if text == "" {
		text = "(nothing due)"
	}
	e.out(a, f.done, text)
	if len(bad) > 0 {
		return fail("routines not read: %s (kanban lint)", strings.Join(bad, ", "))
	}
	return nil
}

func cmdRoutine(e env, a args) error {
	sub := ""
	if len(a.rest) > 0 {
		sub = a.rest[0]
	}
	switch sub {
	case "list":
		return routineList(e, a)
	case "run":
		return routineRun(e, a)
	}
	return fail("usage: kanban routine list [--json] | kanban routine run <name>")
}

func routineList(e env, a args) error {
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	loc, err := loadLocation(cfg)
	if err != nil {
		return err
	}
	var values []any
	var lines []string
	_, err = withEvents(e, cfg, func(f *firing) error {
		tasks := allTasks(e.root)
		for _, r := range f.all {
			o := &omap{}
			o.set("name", r.name)
			if p := r.problems(cfg); len(p) > 0 {
				o.set("broken", strings.Join(p, "; "))
				values = append(values, o)
				lines = append(lines, fmt.Sprintf("%s  (not read: %s)", r.name, strings.Join(p, "; ")))
				continue
			}
			since, _ := f.since(r)
			open, _ := r.open()
			paused, _ := r.paused()
			o.set("title", r.d.str("title"))
			parts := []string{padEnd(r.name, 12)}
			var next any
			if s := r.schedule(); s != "" {
				o.set("schedule", s)
				parts = append(parts, fmt.Sprintf("schedule %q", s))
				if c, _ := parseCron(s); c != nil && !paused {
					if t, ok := c.next(since, loc); ok {
						next = t.UTC().Format(timeLayout)
					}
				}
			}
			if on := r.on(); len(on) > 0 {
				o.set("on", on)
				parts = append(parts, "on "+strings.Join(on, ","))
			}
			last := f.state[r.name]
			o.set("last", last)
			o.set("next", next)
			n := notDone(tasks, cfg, r.name)
			o.set("notDone", n)
			o.set("open", open)
			o.set("paused", paused)
			parts = append(parts, fmt.Sprintf("last %v", last))
			if next != nil {
				parts = append(parts, fmt.Sprintf("next %v", next))
			}
			parts = append(parts, fmt.Sprintf("open %d/%d", n, open))
			if paused {
				parts = append(parts, "paused")
			}
			values = append(values, o)
			lines = append(lines, strings.Join(parts, "  "))
		}
		return nil
	})
	if err != nil {
		return err
	}
	text := strings.Join(lines, "\n")
	if text == "" {
		text = "(no routines)"
	}
	if values == nil {
		values = []any{}
	}
	e.out(a, values, text)
	return nil
}

func routineRun(e env, a args) error {
	if len(a.rest) < 2 || a.rest[1] == "" {
		return fail("usage: kanban routine run <name>")
	}
	name := a.rest[1]
	if err := checkSegment("routine name", name); err != nil {
		return err
	}
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	f, err := withEvents(e, cfg, func(f *firing) error {
		r := f.find(name)
		if r == nil {
			return fail("no routine %s", name)
		}
		if p := r.problems(cfg); len(p) > 0 {
			return fail("routine %s: %s (kanban lint)", name, p[0])
		}
		return f.fire(r)
	})
	if err != nil {
		return err
	}
	e.out(a, f.done[0], f.lines()[0])
	return nil
}

// lintRoutines is kanban lint's part for routines and the time zone.
func lintRoutines(root string, cfg config, cfgRead bool) []string {
	var out []string
	if cfgRead {
		if _, err := loadLocation(cfg); err != nil {
			out = append(out, "config.yaml: "+err.Error())
		}
	}
	rs, err := readRoutines(root)
	if err != nil {
		return append(out, "routines/: "+firstLine(err.Error()))
	}
	for _, r := range rs {
		for _, p := range r.problems(cfg) {
			if !cfgRead && strings.HasPrefix(p, "repo ") {
				continue
			}
			out = append(out, fmt.Sprintf("routines/%s.yaml: %s", r.name, p))
		}
	}
	return out
}
