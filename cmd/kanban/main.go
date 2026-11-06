// kanban changes the work ledger in kanban/ (docs/ledger.md; docs/design.md §3.5).
//
// Copied from herdr-flightdeck's cmd/deck at tag deck-v0.4.0 on 2026-11-05 and licensed
// under Apache-2.0 from that day (docs/decisions.md §7, §12). The behaviour is deck-v0.4.0's;
// only the command's name changed. deck is an alias of kanban for one version.
// One folder per piece of work (tasks/<id>/task.yaml, brief.md, report.md, library/),
// written under an OS file lock, atomically, and logged to log.jsonl.
// task.yaml is edited as a YAML node tree, so a person's comments and key order stay.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const help = `kanban: a project's work ledger in kanban/ (format 2; one folder per task)

  kanban init [--root dir] [--prefix W] [--repo host/owner/repo]…  make kanban/ (config.yaml, tasks/)
  kanban add "title" [--repo r] [--profile p] [--parent id] [--after id,…] [--phase id] [--brief file|-]
  kanban list [--status s] [--repo r] [--all] [--json]
  kanban ready [--repo r] [--json]          todo/queued with nothing left to wait for
  kanban show <id> [--json]
  kanban start <id> [--force] [--repo r]    doing; agent from $HC_TERMINAL_ID; branch and worktree from git
  kanban set <id> key=value …               empty value removes the key; after/links take a,b
  kanban review <id> | kanban done <id> [--force]  done refuses a task with questions not received
  kanban report <id> [--append] < file      write report.md
  kanban ask <id> "<text>"|- [--kind k]     ask the person a question (k: business, users, promise, irreversible, line)
  kanban questions [--open] [--json]        open and answered questions, oldest first
  kanban answer <id>#<n> "<answer>"|-       write the person's answer (open → answered)
  kanban received <id>#<n>                  the agent has the answer (answered → received)
  kanban lint [--json]                      tasks, questions, routines and config.yaml
  kanban tick [--json]                      add a task for each routine whose schedule came due
  kanban routine list [--json]              routines: trigger, last and next time, tasks not done
  kanban routine run <name>                 fire a routine now (open and paused still apply)
  kanban config set chief_of_staff=<address>   set config.yaml chief_of_staff (empty value removes it)
  kanban migrate [--root dir]               move a .flightdeck/ ledger to kanban/ (ids to <prefix>-0001; once)
  kanban root                               print the ledger's root
  kanban --version                          print kanban's version

Ids are <prefix>-<n> with n of four digits (W-0001); W-1 is read as W-0001.
The root: $KANBAN_ROOT, else the nearest folder above with kanban/config.yaml, else the projects/<slug>/
whose config.yaml lists this worktree's repository in repos (looked for above its main checkout).
repo is filled in from the current checkout when it matches one of repos.
deck is another name for kanban (for one version); both print this help.`

// version is set at build time (-ldflags "-X main.version=…"); a plain go build says dev.
var version = "dev"

var (
	listKeys  = map[string]bool{"after": true, "links": true}
	fixedKeys = map[string]bool{"id": true, "created": true}
	boolFlags = map[string]bool{"json": true, "append": true, "force": true, "all": true, "open": true}
)

type args struct {
	flags map[string]string   // the last value of each flag
	all   map[string][]string // every value of each flag, for flags that repeat (--repo in init)
	rest  []string
}

func (a args) has(k string) bool { _, ok := a.flags[k]; return ok }

func parseArgs(argv []string) (args, error) {
	a := args{flags: map[string]string{}, all: map[string][]string{}}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if !strings.HasPrefix(s, "--") {
			a.rest = append(a.rest, s)
			continue
		}
		k, v, ok := strings.Cut(s[2:], "=")
		switch {
		case ok:
		case boolFlags[k]:
			v = "true"
		case i+1 < len(argv):
			i++
			v = argv[i]
		default:
			return a, fail("--%s needs a value", k)
		}
		a.flags[k] = v
		a.all[k] = append(a.all[k], v)
	}
	return a, nil
}

// env is what a command runs with; tests replace stdin and stdout.
type env struct {
	root, cwd string
	stdin     io.Reader
	stdout    io.Writer
}

func (e env) println(s string) { fmt.Fprintln(e.stdout, s) }

func (e env) out(a args, v any, text string) {
	if a.has("json") {
		e.println(pretty(v))
	} else {
		e.println(text)
	}
}

type command func(e env, a args) error

var commands map[string]command

func init() {
	commands = map[string]command{
		"init": cmdInit, "add": cmdAdd, "list": cmdList, "ready": cmdReady, "show": cmdShow,
		"start": cmdStart, "set": cmdSet, "review": cmdReview, "done": cmdDone, "report": cmdReport,
		"lint": cmdLint, "tick": cmdTick, "routine": cmdRoutine, "config": cmdConfig,
		"ask": cmdAsk, "questions": cmdQuestions, "answer": cmdAnswer, "received": cmdReceived, "migrate": cmdMigrate, "root": func(e env, _ args) error { e.println(e.root); return nil },
	}
}

// errLint is a lint that found problems: its output is already printed, exit code 1.
var errLint = errors.New("lint found problems")

func run(argv []string, cwd string, stdin io.Reader, stdout, stderr io.Writer) int {
	err := dispatch(argv, cwd, stdin, stdout)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errLint):
		return 1
	}
	fmt.Fprintf(stderr, "kanban: %s\n", firstLine(err.Error()))
	return 1
}

func dispatch(argv []string, cwd string, stdin io.Reader, stdout io.Writer) error {
	if len(argv) == 0 || argv[0] == "help" || argv[0] == "--help" || argv[0] == "-h" {
		fmt.Fprintln(stdout, help)
		return nil
	}
	if argv[0] == "--version" {
		fmt.Fprintln(stdout, "kanban "+version)
		return nil
	}
	cmd, ok := commands[argv[0]]
	if !ok {
		return fail("unknown command %s (kanban help)", argv[0])
	}
	a, err := parseArgs(argv[1:])
	if err != nil {
		return err
	}
	if err := checkClock(); err != nil {
		return err
	}
	e := env{cwd: cwd, stdin: stdin, stdout: stdout}
	if argv[0] != "init" && argv[0] != "migrate" {
		if e.root, err = findRoot(cwd); err != nil {
			return err
		}
		// Every command but init (which checks for itself) and lint (which reports it as a
		// problem) stops on a format it doesn't know, before it reads or writes anything else.
		if argv[0] != "lint" {
			if _, err := readConfig(e.root); err != nil {
				return err
			}
		}
	}
	return cmd(e, a)
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kanban: %s\n", err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], cwd, os.Stdin, os.Stdout, os.Stderr))
}

// initialGitignore keeps the whole ledger out of git (docs/ledger.md, .gitignore).
const initialGitignore = "*\n"

const initialConfig = `version: 2
prefix: %s
columns:
  - todo
  - queued
  - doing
  - review
  - done
phases: []
repos:%s
`

func cmdInit(e env, a args) error {
	root := a.flags["root"]
	if root == "" {
		root = os.Getenv("KANBAN_ROOT")
	}
	if root == "" {
		root = e.cwd
	}
	repos := " []"
	if rs := a.all["repo"]; len(rs) > 0 {
		repos = ""
		for _, r := range rs {
			if !validRepo(r) {
				return fail("--repo %q must be host/owner/repo", r)
			}
			repos += "\n  - " + strNodeText(r)
		}
	}
	prefix := a.flags["prefix"]
	if prefix == "" {
		prefix = "W"
	}
	if err := checkSegment("prefix", prefix); err != nil {
		return err
	}
	root, _ = filepath.Abs(root)
	dir := ledgerDir(root)
	file := filepath.Join(dir, "config.yaml")
	// An existing ledger is checked before anything is made, so a format kanban doesn't know stays as it is.
	already := exists(file)
	if already {
		if _, err := readConfig(root); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		return err
	}
	// The ledger stays out of git (ADR-0005, 5).
	if ignore := filepath.Join(dir, ".gitignore"); !exists(ignore) {
		if err := writeAtomic(ignore, []byte(initialGitignore)); err != nil {
			return err
		}
	}
	if already {
		e.println("already a ledger: " + dir)
		return nil
	}
	if err := writeAtomic(file, fmt.Appendf(nil, initialConfig, strNodeText(prefix), repos)); err != nil {
		return err
	}
	e.println("created " + dir)
	return nil
}

// strNodeText is a string as a YAML scalar (quoted when it would read as something else).
func strNodeText(s string) string {
	d := newDoc()
	d.setStr("x", s)
	return strings.TrimSuffix(strings.TrimPrefix(string(d.encode()), "x: "), "\n")
}

func readInput(e env, name string) (string, error) {
	if name == "-" {
		b, err := io.ReadAll(e.stdin)
		return string(b), err
	}
	b, err := os.ReadFile(name)
	return string(b), err
}

func cmdAdd(e env, a args) error {
	title := strings.TrimSpace(strings.Join(a.rest, " "))
	if title == "" {
		return fail(`usage: kanban add "title" [--repo r] [--profile p] [--parent id] [--after id,…] [--phase id] [--brief file|-]`)
	}
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	if err := checkSegment("prefix in config.yaml", cfg.prefix); err != nil {
		return err
	}
	repo, err := taskRepo(e, a, cfg)
	if err != nil {
		return err
	}
	var brief *string
	if a.has("brief") {
		b, err := readInput(e, a.flags["brief"])
		if err != nil {
			return err
		}
		brief = &b
	}
	var t *task
	err = withLock(e.root, lockWait, func() error {
		tasks := allTasks(e.root)
		all := byID(tasks)
		var after []string
		if a.has("after") {
			after = canonIDs(splitList(a.flags["after"]))
		}
		parent := canonID(a.flags["parent"])
		for _, ref := range append([]string{parent}, after...) {
			if ref != "" && all[ref] == nil {
				return fail("no task %s", ref)
			}
		}
		status := a.flags["status"]
		if !a.has("status") {
			status = cfg.columns[0]
			if len(waitingOn(&task{d: listDoc("after", after)}, all, cfg)) > 0 {
				status = "queued"
			}
		}
		if !cfg.hasColumn(status) {
			return fail("status must be one of %s", strings.Join(cfg.columns, ", "))
		}
		fields := map[string]string{"repo": repo, "parent": parent, "phase": a.flags["phase"], "profile": a.flags["profile"]}
		var err error
		if t, err = newTask(e.root, cfg, title, fields, after, status, brief); err != nil {
			return err
		}
		change := &omap{}
		change.set("add", title)
		return appendLog(e.root, t.id, change)
	})
	if err != nil {
		return err
	}
	e.out(a, t.json(), t.id)
	return nil
}

func listDoc(key string, items []string) *doc {
	d := newDoc()
	d.setList(key, items)
	return d
}

// newTask adds a task under the lock, as kanban add does: the next id, title, the non-empty
// fields, after, status, created and updated, and brief.md when brief isn't nil. The caller logs it.
func newTask(root string, cfg config, title string, fields map[string]string, after []string, status string, brief *string) (*task, error) {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(cfg.prefix) + `-(\d+)$`)
	n := 0
	for _, x := range allTasks(root) {
		if re.MatchString(x.id) {
			n = max(n, numberOf(x.id))
		}
	}
	at := now()
	t := &task{id: formatID(cfg.prefix, n+1), d: newDoc()}
	t.d.setStr("id", t.id)
	t.d.setStr("title", title)
	t.d.setStr("status", status)
	for k, v := range fields {
		if v != "" {
			t.d.setStr(k, v)
		}
	}
	t.d.setList("after", after)
	t.d.setStr("created", at)
	t.d.setStr("updated", at)
	if err := writeTask(root, t); err != nil {
		return nil, err
	}
	if brief != nil {
		if err := writeAtomic(filepath.Join(taskDir(root, t.id), "brief.md"), []byte(*brief)); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// taskRepo is the repo for a new or started task: --repo, which must be one of repos,
// else the entry of repos that the current folder's main checkout matches, else "".
func taskRepo(e env, a args, cfg config) (string, error) {
	if a.has("repo") {
		r := a.flags["repo"]
		if !validRepo(r) {
			return "", fail("--repo %q must be host/owner/repo", r)
		}
		if !slices.Contains(cfg.repos, r) {
			return "", fail("repo %s is not in config.yaml repos", r)
		}
		return r, nil
	}
	return repoOf(cfg.repos, mainCheckout(e.cwd)), nil
}

// repoName is the last part of a repo (github.com/x/app -> app), for short lines.
func repoName(r string) string { return r[strings.LastIndex(r, "/")+1:] }

func padEnd(s string, n int) string {
	if c := utf8.RuneCountInString(s); c < n {
		return s + strings.Repeat(" ", n-c)
	}
	return s
}

func line(t *task, cfg config, all map[string]*task) string {
	if t.broken != "" {
		return fmt.Sprintf("%s  (unreadable: %s)", t.id, t.broken)
	}
	wait, agent := "", ""
	if w := waitingOn(t, all, cfg); len(w) > 0 {
		wait = "  [after " + strings.Join(w, ", ") + "]"
	}
	if s := t.d.str("agent"); s != "" {
		agent = "  @" + s
	}
	parts := []string{padEnd(t.id, 6), padEnd(t.status(), 7), padEnd(repoName(t.repo()), 10), t.d.str("title"), wait, agent}
	return strings.TrimRight(strings.Join(parts, " "), " \t")
}

func render(e env, a args, shown []*task, cfg config, all map[string]*task, none string) {
	values := make([]any, 0, len(shown))
	lines := make([]string, 0, len(shown))
	for _, t := range shown {
		values = append(values, t.json())
		lines = append(lines, line(t, cfg, all))
	}
	text := strings.Join(lines, "\n")
	if text == "" {
		text = none
	}
	e.out(a, values, text)
}

func cmdList(e env, a args) error {
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	tasks := allTasks(e.root)
	all := byID(tasks)
	var shown []*task
	for _, t := range tasks {
		st := ""
		if t.broken == "" {
			st = t.status()
		}
		if a.has("status") && st != a.flags["status"] {
			continue
		}
		if a.has("repo") && (t.broken != "" || t.repo() != a.flags["repo"]) {
			continue
		}
		if !a.has("all") && !a.has("status") && t.broken == "" && st == cfg.done() {
			continue
		}
		shown = append(shown, t)
	}
	render(e, a, shown, cfg, all, "(nothing)")
	return nil
}

func cmdReady(e env, a args) error {
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	tasks := allTasks(e.root)
	all := byID(tasks)
	var shown []*task
	for _, t := range tasks {
		if t.broken == "" && isReady(t, all, cfg) && (!a.has("repo") || t.repo() == a.flags["repo"]) {
			shown = append(shown, t)
		}
	}
	render(e, a, shown, cfg, all, "(nothing ready)")
	return nil
}

func need(a args, usage string) (string, error) {
	if len(a.rest) == 0 || a.rest[0] == "" {
		return "", fail("usage: %s", usage)
	}
	return canonID(a.rest[0]), nil
}

func cmdShow(e env, a args) error {
	id, err := need(a, "kanban show <id>")
	if err != nil {
		return err
	}
	t, err := readTask(e.root, id)
	if err != nil {
		return err
	}
	read := func(name string) any {
		b, err := os.ReadFile(filepath.Join(taskDir(e.root, id), name))
		if err != nil {
			return nil
		}
		return string(b)
	}
	brief, report := read("brief.md"), read("report.md")
	o := &omap{}
	o.set("task", t.json())
	o.set("brief", brief)
	o.set("report", report)
	q, qerr := readQuestions(e.root, id)
	if qerr == nil {
		all := []any{}
		for _, d := range q.items {
			if d != nil {
				all = append(all, value(d.m))
			}
		}
		o.set("questions", all)
	} else {
		o.set("questions", nil)
		o.set("questionsBroken", firstLine(qerr.Error()))
	}
	if t.d.str("id") != id {
		t.d.setStr("id", id)
	}
	parts := []string{strings.TrimRight(string(t.d.encode()), "\n \t")}
	head := func(s string) string {
		l := strings.Split(s, "\n")
		return strings.Join(l[:min(40, len(l))], "\n")
	}
	if s, ok := brief.(string); ok && s != "" {
		parts = append(parts, "\n--- brief.md\n"+head(s))
	}
	if s, ok := report.(string); ok && s != "" {
		parts = append(parts, "\n--- report.md\n"+head(s))
	}
	if qerr != nil {
		parts = append(parts, "\n--- questions: unreadable ("+firstLine(qerr.Error())+")")
	} else if len(q.items) > 0 {
		c := questionCounts(q)
		parts = append(parts, fmt.Sprintf("\n--- questions: %d open, %d answered, %d received", c["open"], c["answered"], c["received"]))
	}
	e.out(a, o, strings.Join(parts, "\n"))
	return nil
}

func cmdStart(e env, a args) error {
	id, err := need(a, "kanban start <id>")
	if err != nil {
		return err
	}
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	repo, err := taskRepo(e, a, cfg)
	if err != nil {
		return err
	}
	var t *task
	f, err := withEvents(e, cfg, func(f *firing) error {
		all := byID(allTasks(e.root))
		if t, err = readTask(e.root, id); err != nil {
			return err
		}
		was := t.status()
		if w := waitingOn(t, all, cfg); len(w) > 0 && !a.has("force") {
			return fail("%s waits for %s (--force to start anyway)", id, strings.Join(w, ", "))
		}
		branch := git(e.cwd, "rev-parse", "--abbrev-ref", "HEAD")
		top := git(e.cwd, "rev-parse", "--show-toplevel")
		t.d.setStr("status", "doing")
		t.d.setStr("updated", now())
		if agent := os.Getenv("HC_TERMINAL_ID"); agent != "" {
			t.d.setStr("agent", agent)
		}
		if branch != "" && branch != "HEAD" {
			t.d.setStr("branch", branch)
		}
		if top != "" {
			t.d.setStr("worktree", tilde(filepath.FromSlash(top)))
		}
		if repo != "" && (a.has("repo") || isNull(t.d.get("repo"))) {
			t.d.setStr("repo", repo)
		}
		if err := writeTask(e.root, t); err != nil {
			return err
		}
		change := &omap{}
		change.set("status", "doing")
		for _, k := range []string{"agent", "branch"} {
			if v := t.d.str(k); v != "" {
				change.set(k, v)
			}
		}
		if err := appendLog(e.root, id, change); err != nil {
			return err
		}
		if was != "doing" {
			return f.entered(t, "doing")
		}
		return nil
	})
	if err != nil {
		return err
	}
	text := id + " doing"
	if s := t.d.str("agent"); s != "" {
		text += " @" + s
	}
	if s := t.d.str("branch"); s != "" {
		text += " on " + s
	}
	e.out(a, t.json(), text)
	printFired(e, a, f)
	return nil
}

func cmdSet(e env, a args) error {
	if len(a.rest) < 2 || a.rest[0] == "" {
		return fail("usage: kanban set <id> key=value …  (empty value removes the key)")
	}
	id, pairs := canonID(a.rest[0]), a.rest[1:]
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	var t *task
	f, err := withEvents(e, cfg, func(f *firing) error {
		if t, err = readTask(e.root, id); err != nil {
			return err
		}
		was := t.status()
		change := &omap{}
		for _, pair := range pairs {
			k, v, _ := strings.Cut(pair, "=")
			if !strings.Contains(pair, "=") || k == "" {
				return fail("not key=value: %s", pair)
			}
			if fixedKeys[k] {
				return fail("%s can't be changed", k)
			}
			if k == "status" && !cfg.hasColumn(v) {
				return fail("status must be one of %s", strings.Join(cfg.columns, ", "))
			}
			switch {
			case v == "":
				t.d.del(k)
				change.set(k, nil)
			case listKeys[k]:
				items := splitList(v)
				if k == "after" {
					items = canonIDs(items)
				}
				t.d.setList(k, items)
				change.set(k, items)
			default:
				if k == "parent" {
					v = canonID(v)
				}
				t.d.setStr(k, v)
				change.set(k, v)
			}
		}
		t.d.setStr("updated", now())
		if err := writeTask(e.root, t); err != nil {
			return err
		}
		if err := appendLog(e.root, id, change); err != nil {
			return err
		}
		if _, set := change.vals["status"]; set && t.status() != was && t.status() != "" {
			return f.entered(t, t.status())
		}
		return nil
	})
	if err != nil {
		return err
	}
	e.out(a, t.json(), id+" updated")
	printFired(e, a, f)
	return nil
}

func cmdReview(e env, a args) error {
	id, err := need(a, "kanban review <id>")
	if err != nil {
		return err
	}
	return cmdSet(e, args{flags: a.flags, rest: []string{id, "status=review"}})
}

// cmdDone marks a task done; then any queued task with nothing left to wait for moves to the first column.
func cmdDone(e env, a args) error {
	id, err := need(a, "kanban done <id>")
	if err != nil {
		return err
	}
	cfg, err := readConfig(e.root)
	if err != nil {
		return err
	}
	freed := []string{}
	f, err := withEvents(e, cfg, func(f *firing) error {
		t, err := readTask(e.root, id)
		if err != nil {
			return err
		}
		was := t.status()
		// A question not received keeps the task open (docs/ledger.md, Questions); --force closes it and logs which.
		var forced []int
		q, err := readQuestions(e.root, id)
		if err != nil && !a.has("force") {
			return fail("%s (--force to close anyway)", firstLine(err.Error()))
		}
		if err == nil {
			nums, names := notReceived(q)
			if len(nums) > 0 && !a.has("force") {
				return fail("%s has questions not received: %s (--force to close anyway)", id, strings.Join(names, ", "))
			}
			forced = nums
		}
		t.d.setStr("status", cfg.done())
		t.d.setStr("updated", now())
		if err := writeTask(e.root, t); err != nil {
			return err
		}
		change := &omap{}
		change.set("status", cfg.done())
		if len(forced) > 0 {
			change.set("forced", forced)
		}
		if err := appendLog(e.root, id, change); err != nil {
			return err
		}
		if was != cfg.done() {
			if err := f.entered(t, cfg.done()); err != nil {
				return err
			}
		}
		tasks := allTasks(e.root)
		all := byID(tasks)
		for _, o := range tasks {
			if o.broken != "" || o.status() != "queued" || !contains(o.after(), id) || len(waitingOn(o, all, cfg)) > 0 {
				continue
			}
			o.d.setStr("status", cfg.columns[0])
			o.d.setStr("updated", now())
			if err := writeTask(e.root, o); err != nil {
				return err
			}
			change := &omap{}
			change.set("status", cfg.columns[0])
			change.set("freedBy", id)
			if err := appendLog(e.root, o.id, change); err != nil {
				return err
			}
			if err := f.entered(o, cfg.columns[0]); err != nil {
				return err
			}
			freed = append(freed, o.id)
		}
		return nil
	})
	if err != nil {
		return err
	}
	o := &omap{}
	o.set("id", id)
	o.set("ready", freed)
	text := id + " done"
	if len(freed) > 0 {
		text += "; now ready: " + strings.Join(freed, ", ")
	}
	e.out(a, o, text)
	printFired(e, a, f)
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func cmdReport(e env, a args) error {
	id, err := need(a, "kanban report <id> [--append] < file")
	if err != nil {
		return err
	}
	if _, err := readTask(e.root, id); err != nil {
		return err
	}
	text, err := readInput(e, "-")
	if err != nil {
		return err
	}
	appending := a.has("append")
	err = withLock(e.root, lockWait, func() error {
		dir, err := taskPath(e.root, id)
		if err != nil {
			return err
		}
		file := filepath.Join(dir, "report.md")
		if err := checkNoLink(file, true); err != nil {
			return err
		}
		if appending && exists(file) {
			if !strings.HasPrefix(text, "\n") {
				text = "\n" + text
			}
			if err := appendFile(file, []byte(text)); err != nil {
				return err
			}
		} else if err := writeAtomic(file, []byte(text)); err != nil {
			return err
		}
		change := &omap{}
		change.set("report", map[bool]string{true: "append", false: "write"}[appending])
		return appendLog(e.root, id, change)
	})
	if err != nil {
		return err
	}
	e.println(fmt.Sprintf("%s/report.md %s", id, map[bool]string{true: "appended", false: "written"}[appending]))
	return nil
}

func cmdLint(e env, a args) error {
	problems := []string{}
	cfg, err := readConfig(e.root)
	cfgRead := err == nil // repos can be checked only against a config.yaml that was read
	if err != nil {
		problems = append(problems, "config.yaml: "+firstLine(err.Error()))
		cfg = config{prefix: "W", columns: defaultColumns}
	} else if err := checkSegment("prefix", cfg.prefix); err != nil {
		problems = append(problems, "config.yaml: "+err.Error())
	}
	for _, r := range cfg.repos {
		if !validRepo(r) {
			problems = append(problems, fmt.Sprintf("config.yaml: repos entry %q is not host/owner/repo", r))
		}
	}
	tasks := allTasks(e.root)
	all := byID(tasks)
	for _, t := range tasks {
		if t.broken != "" {
			problems = append(problems, fmt.Sprintf("%s: %s", t.id, t.broken))
			continue
		}
		if own := t.d.get("id"); !isNull(own) && own.Value != t.id {
			problems = append(problems, fmt.Sprintf("%s: id says %s", t.id, own.Value))
		}
		if t.d.str("title") == "" {
			problems = append(problems, t.id+": no title")
		}
		if !cfg.hasColumn(t.status()) {
			problems = append(problems, fmt.Sprintf("%s: status %q is not a column", t.id, t.status()))
		}
		for _, x := range t.after() {
			if all[x] == nil {
				problems = append(problems, fmt.Sprintf("%s: after %s, which doesn't exist", t.id, x))
			}
		}
		if p := t.d.str("parent"); p != "" && all[p] == nil {
			problems = append(problems, fmt.Sprintf("%s: parent %s, which doesn't exist", t.id, p))
		}
		if r := t.repo(); r != "" && cfgRead && !slices.Contains(cfg.repos, r) {
			problems = append(problems, fmt.Sprintf("%s: repo %s is not in config.yaml repos", t.id, r))
		}
	}
	problems = append(problems, lintQuestions(e.root)...)
	problems = append(problems, lintRoutines(e.root, cfg, cfgRead)...)
	o := &omap{}
	o.set("ok", len(problems) == 0)
	o.set("problems", problems)
	text := fmt.Sprintf("ok (%d tasks)", len(tasks))
	if len(problems) > 0 {
		text = strings.Join(problems, "\n")
	}
	e.out(a, o, text)
	if len(problems) > 0 {
		return errLint
	}
	return nil
}

// cmdConfig changes config.yaml under the lock. Only chief_of_staff: the Chief of Staff's address changes
// each time it is started again, and it sets it here rather than by hand, outside the lock.
// config.yaml is edited as a YAML tree, so comments and key order stay.
func cmdConfig(e env, a args) error {
	const usage = "usage: kanban config set chief_of_staff=<address>  (empty value removes it)"
	if len(a.rest) != 2 || a.rest[0] != "set" || !strings.Contains(a.rest[1], "=") {
		return fail(usage)
	}
	k, v, _ := strings.Cut(a.rest[1], "=")
	if k != "chief_of_staff" {
		return fail("kanban config set changes chief_of_staff only, not %q", k)
	}
	file := filepath.Join(ledgerDir(e.root), "config.yaml")
	err := withLock(e.root, lockWait, func() error {
		if err := checkNoLink(file, true); err != nil {
			return err
		}
		// Read again under the lock: a format kanban doesn't know stops here, before anything is written.
		if _, err := readConfig(e.root); err != nil {
			return err
		}
		d, err := readDoc(file)
		if err != nil {
			return err
		}
		change := &omap{}
		if v == "" {
			d.del(k)
			change.set(k, nil)
		} else {
			d.setStr(k, v)
			change.set(k, v)
		}
		if err := writeAtomic(file, d.encode()); err != nil {
			return err
		}
		entry := &omap{}
		entry.set("at", now())
		entry.set("by", who())
		entry.set("config", change)
		return appendEntry(e.root, entry)
	})
	if err != nil {
		return err
	}
	var shown any
	text := "chief_of_staff removed"
	if v != "" {
		shown, text = v, "chief_of_staff: "+v
	}
	e.out(a, map[string]any{k: shown}, text)
	return nil
}
