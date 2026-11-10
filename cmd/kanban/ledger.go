package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

var defaultColumns = []string{"todo", "queued", "doing", "review", "done"}

const lockWait = 10 * time.Second

// deckError is a failure meant for the person: one line on stderr, exit code 1.
type deckError struct{ msg string }

func (e deckError) Error() string { return e.msg }

func fail(format string, a ...any) error { return deckError{fmt.Sprintf(format, a...)} }

// clock is the time now, or $DECK_NOW (RFC 3339) when set: tests set it so that routines
// fire without waiting for the real clock. dispatch refuses a DECK_NOW it can't read.
func clock() time.Time {
	if s := os.Getenv("DECK_NOW"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func checkClock() error {
	if s := os.Getenv("DECK_NOW"); s != "" {
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return fail("DECK_NOW %q is not an RFC 3339 time", s)
		}
	}
	return nil
}

func now() string { return clock().UTC().Format(timeLayout) }

func who() string {
	if id := os.Getenv("HC_TERMINAL_ID"); id != "" {
		return "terminal:" + id
	}
	for _, k := range []string{"USER", "USERNAME"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "unknown"
}

// git runs git without a shell and returns its trimmed output, or "" when it fails.
func git(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ledgerName is the ledger's folder in the project's root (docs/ledger.md, Layout).
const ledgerName = "kanban"

func ledgerDir(root string) string { return filepath.Join(root, ledgerName) }

// isLedger: root/kanban/config.yaml is a mapping whose version is a YAML integer (2, not "2").
// kanban/ is a plain name, so a kanban/ folder without that (in a product's repository, say) is not a ledger.
func isLedger(root string) bool {
	d, err := readDoc(filepath.Join(ledgerDir(root), "config.yaml"))
	if err != nil {
		return false
	}
	v := d.get("version")
	return isInt(v)
}

// isInt: n is a YAML integer scalar (its resolved tag is !!int; a quoted "2" is a string).
func isInt(n *yaml.Node) bool {
	if isNull(n) || n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" {
		return false
	}
	_, err := strconv.Atoi(n.Value)
	return err == nil
}

func findUp(dir string) string {
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if isLedger(d) {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// mainCheckout is the main checkout of the git repository that holds cwd (for a worktree,
// the repository it belongs to), or "" outside git.
func mainCheckout(cwd string) string {
	common := git(cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if common == "" {
		return ""
	}
	return filepath.Dir(filepath.FromSlash(common))
}

// validRepo: exactly <host>/<owner>/<repo>, three names with no empty part
// (so no leading or trailing /). Other entries match nothing.
func validRepo(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 3 && !slices.Contains(parts, "")
}

// repoMatches: the checkout's path ends with /<host>/<owner>/<repo> (docs/ledger.md, repos).
func repoMatches(checkout, repo string) bool {
	if !validRepo(repo) {
		return false
	}
	c, r := filepath.ToSlash(checkout), repo
	if runtime.GOOS == "windows" {
		c, r = strings.ToLower(c), strings.ToLower(r)
	}
	return strings.HasSuffix(c, "/"+r)
}

// repoOf is the entry of repos that the checkout matches, or "".
func repoOf(repos []string, checkout string) string {
	if checkout == "" {
		return ""
	}
	for _, r := range repos {
		if repoMatches(checkout, r) {
			return r
		}
	}
	return ""
}

// projectsFor walks up from the checkout and returns, at the first folder where any match,
// the projects/<slug> folders whose kanban/config.yaml lists the checkout in repos.
// It reads nothing but projects/*/kanban/config.yaml.
func projectsFor(checkout string) (string, []string) {
	for d := filepath.Clean(checkout); ; d = filepath.Dir(d) {
		configs, _ := filepath.Glob(filepath.Join(d, "projects", "*", ledgerName, "config.yaml"))
		var found []string
		for _, c := range configs {
			root := filepath.Dir(filepath.Dir(c))
			if doc, err := readDoc(c); err == nil && isLedger(root) && repoOf(doc.list("repos"), checkout) != "" {
				found = append(found, root)
			}
		}
		if len(found) > 0 {
			return d, found
		}
		if filepath.Dir(d) == d {
			return "", nil
		}
	}
}

// findRoot: KANBAN_ROOT, else up from here, else the project whose repos match
// this worktree's main checkout (docs/ledger.md, Finding the root).
func findRoot(cwd string) (string, error) {
	if env := os.Getenv("KANBAN_ROOT"); env != "" {
		r, _ := filepath.Abs(env)
		if !exists(filepath.Join(ledgerDir(r), "config.yaml")) {
			return "", fail("KANBAN_ROOT has no kanban/: %s (run kanban init there)", r)
		}
		return r, nil
	}
	if r := findUp(cwd); r != "" {
		return r, nil
	}
	if checkout := mainCheckout(cwd); checkout != "" {
		switch at, found := projectsFor(checkout); len(found) {
		case 0:
		case 1:
			return found[0], nil
		default:
			names := make([]string, len(found))
			for i, f := range found {
				names[i] = "projects/" + filepath.Base(f)
			}
			return "", fail("this repository is in more than one project under %s: %s (set KANBAN_ROOT)", at, strings.Join(names, ", "))
		}
	}
	return "", fail("no ledger: no kanban/ here or above, and no projects/*/kanban/ lists this repository (run kanban init, or set KANBAN_ROOT)")
}

// writeAtomic writes a temporary file beside the target and renames it over the target
// (a rename replaces an existing file on Windows too).
func writeAtomic(file string, data []byte) error {
	if err := checkNoLink(file, true); err != nil {
		return err
	}
	tmp := tmpName(file)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := renameRetrying(tmp, file, renameWait); err != nil {
		os.Remove(tmp)
		return fail("can't replace %s: %v", file, err)
	}
	return nil
}

// tmpName is the temporary file beside file.
func tmpName(file string) string {
	return fmt.Sprintf("%s.tmp-%d-%x", file, os.Getpid(), rand.Uint32())
}

// renameWait is how long a rename keeps trying while the target is busy (Windows only).
var renameWait = time.Second

func renameRetrying(from, to string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		err := os.Rename(from, to)
		if err == nil || !renameBusy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Duration(20+rand.IntN(30)) * time.Millisecond)
	}
}

// withLock runs fn while holding an OS lock on kanban/.lock: one writer at a time
// for ids and for task writes. The file stays (empty); the lock goes with the process.
func withLock(root string, wait time.Duration, fn func() error) error {
	return lockIn(ledgerDir(root), wait, fn)
}

// lockIn is withLock on the ledger folder dir (kanban migrate locks the old folder).
func lockIn(dir string, wait time.Duration, fn func() error) error {
	path := filepath.Join(dir, ".lock")
	if fi, err := os.Lstat(path); err == nil && fi.IsDir() {
		return fail("%s is a folder (left by the old node deck); remove it", path)
	}
	// Checked before anything is written, so a refused log doesn't leave a task half-changed.
	for _, p := range []string{path, filepath.Join(dir, "log.jsonl")} {
		if err := checkNoLink(p, true); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	deadline := time.Now().Add(wait)
	for {
		ok, err := tryLock(f)
		if err != nil {
			return err
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			return fail("%s is held (another kanban is writing)", path)
		}
		time.Sleep(time.Duration(20+rand.IntN(30)) * time.Millisecond)
	}
	defer unlock(f)
	return fn()
}

// formatVersion is the ledger format kanban reads and writes (docs/ledger.md).
const formatVersion = 3

// errFormat2 is the one line every command stops with on a format 2 ledger (tasks/, task.yaml):
// there is no migration and nothing reads it (docs/ledger.md, Changes from version 2).
const errFormat2 = "config.yaml is format 2; move it by hand (docs/operate.md)"

type config struct {
	prefix   string
	columns  []string
	repos    []string
	timezone string // IANA name for routines' schedule; "" is UTC
}

func (c config) done() string { return c.columns[len(c.columns)-1] }

func (c config) hasColumn(s string) bool { return slices.Contains(c.columns, s) }

func readDoc(file string) (*doc, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parseDoc(data)
}

// isFormat2: root's config.yaml says version 2.
func isFormat2(root string) bool {
	d, err := readDoc(filepath.Join(ledgerDir(root), "config.yaml"))
	if err != nil {
		return false
	}
	v := d.get("version")
	return isInt(v) && v.Value == "2"
}

// readConfig reads config.yaml and stops on any version but 3 (none means 1).
// title and chief_of_staff are kept in the file; kanban itself doesn't use them.
func readConfig(root string) (config, error) {
	d, err := readDoc(filepath.Join(ledgerDir(root), "config.yaml"))
	if err != nil {
		if err.Error() == "not a mapping" {
			return config{}, fail("config.yaml is not a mapping")
		}
		return config{}, err
	}
	switch v := d.get("version"); {
	case isNull(v):
		return config{}, fail("config.yaml has no version, so it is format 1; this kanban reads format %d only (docs/ledger.md: move the issues by hand)", formatVersion)
	case isInt(v) && v.Value == "2":
		return config{}, fail(errFormat2)
	case !isInt(v) || v.Value != strconv.Itoa(formatVersion):
		return config{}, fail("config.yaml is format version %s; this kanban reads format %d only", v.Value, formatVersion)
	}
	c := config{prefix: "W", columns: defaultColumns, repos: d.list("repos"), timezone: d.str("timezone")}
	if n := d.get("prefix"); !isNull(n) && n.Kind == yaml.ScalarNode {
		c.prefix = n.Value
	}
	if n := d.get("columns"); n != nil && n.Kind == yaml.SequenceNode && len(n.Content) > 0 {
		c.columns = d.list("columns")
	}
	return c, nil
}

// issueFileName is each issue's own file, issues/<id>/issue.yaml.
const issueFileName = "issue.yaml"

func issuesDir(root string) string    { return filepath.Join(ledgerDir(root), "issues") }
func taskDir(root, id string) string  { return filepath.Join(issuesDir(root), id) }
func taskFile(root, id string) string { return filepath.Join(taskDir(root, id), issueFileName) }

// numberOf is the number of a task's id: 5 for A-0005, and for a reviewer's A-0005-r1 too (its parent's).
func numberOf(id string) int {
	if base, _, ok := splitRound(id); ok {
		id = base
	}
	n, _ := strconv.Atoi(id[strings.LastIndex(id, "-")+1:])
	return n
}

// splitRound reads a reviewer's id, <parent>-r<k> (A-0005-r1): the parent's id and the round k (1 or more).
// ok is false for any other id.
func splitRound(id string) (parent string, k int, ok bool) {
	i := strings.LastIndex(id, "-r")
	if i <= 0 || i+2 == len(id) || strings.Trim(id[i+2:], "0123456789") != "" {
		return "", 0, false
	}
	k, err := strconv.Atoi(id[i+2:])
	if err != nil || k < 1 {
		return "", 0, false
	}
	return id[:i], k, true
}

// roundID is the id of a reviewer's task for the parent's round k: <parent>-r<k> (A-0005-r1).
func roundID(parent string, k int) string { return fmt.Sprintf("%s-r%d", parent, k) }

// isReviewProfile is whether a task with this profile is a reviewer's (the yard's *-review profiles).
func isReviewProfile(p string) bool { return strings.HasSuffix(p, "-review") }

// idDigits is how many digits a task's number has at least, so that ids sort by name
// (A-0005; past 9999 the number only grows, and names stop sorting in number order).
const idDigits = 4

// formatID is <prefix>-<n> with n zero-padded to idDigits.
func formatID(prefix string, n int) string { return fmt.Sprintf("%s-%0*d", prefix, idDigits, n) }

// canonID reads an id as typed: A-5, A-05 and A-0005 are the same task, A-0005; a reviewer's
// A-5-r1 is A-0005-r1 (the round without leading zeros). Any other id comes back as it is.
func canonID(id string) string {
	if base, k, ok := splitRound(id); ok {
		return roundID(canonNumber(base), k)
	}
	return canonNumber(id)
}

// canonNumber is canonID for <prefix>-<digits>.
func canonNumber(id string) string {
	i := strings.LastIndex(id, "-")
	if i <= 0 || i == len(id)-1 || strings.Trim(id[i+1:], "0123456789") != "" {
		return id
	}
	n, err := strconv.Atoi(id[i+1:])
	if err != nil || n < 1 {
		return id
	}
	return formatID(id[:i], n)
}

// canonIDs is canonID on each.
func canonIDs(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = canonID(id)
	}
	return out
}

func ids(root string) []string {
	entries, err := os.ReadDir(issuesDir(root))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := numberOf(out[i]), numberOf(out[j])
		if a != b {
			return a < b
		}
		_, ra, _ := splitRound(out[i])
		_, rb, _ := splitRound(out[j])
		if ra != rb {
			return ra < rb // a reviewer's issues right after their parent, in round order
		}
		return out[i] < out[j]
	})
	return out
}

// task is one issue, issues/<id>/issue.yaml. Its id is the folder's name.
type task struct {
	id     string
	d      *doc
	broken string
}

func (t *task) status() string  { return t.d.str("status") }
func (t *task) repo() string    { return t.d.str("repo") }
func (t *task) after() []string { return t.d.list("after") }

func readTask(root, id string) (*task, error) {
	dir, err := taskPath(root, id)
	if err != nil {
		return nil, err
	}
	file := filepath.Join(dir, issueFileName)
	if !exists(file) {
		return nil, fail("no issue %s", id)
	}
	d, err := readDoc(file)
	if err != nil {
		if err.Error() == "not a mapping" {
			return nil, fail("%s/%s is not a mapping", id, issueFileName)
		}
		return nil, err
	}
	return &task{id: id, d: d}, nil
}

func allTasks(root string) []*task {
	var out []*task
	for _, id := range ids(root) {
		t, err := readTask(root, id)
		if err != nil {
			t = &task{id: id, broken: firstLine(err.Error())}
		}
		out = append(out, t)
	}
	return out
}

func byID(tasks []*task) map[string]*task {
	m := map[string]*task{}
	for _, t := range tasks {
		m[t.id] = t
	}
	return m
}

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }

func writeTask(root string, t *task) error {
	dir, err := taskPath(root, t.id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if t.d.str("id") != t.id {
		t.d.setStr("id", t.id)
	}
	return writeAtomic(filepath.Join(dir, issueFileName), t.d.encode())
}

// json is the task as plain values (file order), with id set to the folder's name.
func (t *task) json() any {
	if t.broken != "" {
		o := &omap{}
		o.set("id", t.id)
		o.set("broken", t.broken)
		return o
	}
	o := value(t.d.m).(*omap)
	o.set("id", t.id)
	return o
}

func appendLog(root, id string, change *omap) error {
	entry := &omap{}
	entry.set("at", now())
	entry.set("by", who())
	entry.set("id", id)
	entry.set("change", change)
	return appendEntry(root, entry)
}

func appendEntry(root string, entry *omap) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return appendFile(filepath.Join(ledgerDir(root), "log.jsonl"), append(line, '\n'))
}

func waitingOn(t *task, all map[string]*task, cfg config) []string {
	out := []string{}
	for _, a := range t.after() {
		if o := all[a]; o == nil || o.broken != "" || o.status() != cfg.done() {
			out = append(out, a)
		}
	}
	return out
}

func isReady(t *task, all map[string]*task, cfg config) bool {
	s := t.status()
	return (s == "todo" || s == "queued") && len(waitingOn(t, all, cfg)) == 0 && !notYet(t, cfg)
}

// notBeforeLayout is the form of an issue's not_before: a date, YYYY-MM-DD.
const notBeforeLayout = "2006-01-02"

// notYet is true when the issue has a not_before that is still in the future: its date, read in
// config.yaml's timezone (UTC when there is none or it can't be read), is later than today there.
// A not_before that isn't a date hides nothing (lint reports it).
func notYet(t *task, cfg config) bool {
	v := t.d.str("not_before")
	if v == "" {
		return false
	}
	day, err := time.Parse(notBeforeLayout, v)
	if err != nil {
		return false
	}
	loc, err := loadLocation(cfg)
	if err != nil {
		loc = time.UTC
	}
	return clock().In(loc).Format(notBeforeLayout) < day.Format(notBeforeLayout)
}

func tilde(p string) string {
	home, err := os.UserHomeDir()
	if p == "" || err != nil || !strings.HasPrefix(p, home) {
		return p
	}
	return "~" + strings.ReplaceAll(p[len(home):], `\`, "/")
}

func splitList(v string) []string {
	out := []string{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// omap is a JSON object that keeps its keys in insertion order.
type omap struct {
	keys []string
	vals map[string]any
}

func (o *omap) set(k string, v any) {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *omap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := marshal(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// pretty is JSON.stringify(v, null, 2).
func pretty(v any) string {
	raw, err := marshal(v)
	if err != nil {
		panic(err)
	}
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		panic(err)
	}
	return b.String()
}
