package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Questions (docs/ledger.md "Questions", docs/decisions.md §6): tasks/<id>/questions.yaml is
// a list of the questions a task's agent asked the person. Each goes open → answered →
// received; the text and the answer stay in the file, the log has only the number and status.

var (
	questionStatuses = []string{"open", "answered", "received"}
	questionKinds    = []string{"business", "users", "promise", "irreversible", "line"}
	// questionKeys are the keys kanban knows (the schema describes exactly these).
	questionKeys = []string{"n", "text", "kind", "asked", "askedBy", "status", "answer", "answered", "answeredBy", "received"}
)

// questionsFile is one tasks/<id>/questions.yaml kept as a node tree, so that a person's
// comments, key order and keys kanban doesn't know stay.
type questionsFile struct {
	id    string
	root  *yaml.Node // the document node
	seq   *yaml.Node // its top-level sequence
	items []*doc     // one per entry; nil for an entry that isn't a mapping
}

func questionsPath(root, id string) (string, error) {
	dir, err := taskPath(root, id)
	if err != nil {
		return "", err
	}
	file := filepath.Join(dir, "questions.yaml")
	if err := checkNoLink(file, true); err != nil {
		return "", err
	}
	return file, nil
}

// readQuestions reads a task's questions; a missing file is an empty list.
func readQuestions(root, id string) (*questionsFile, error) {
	file, err := questionsPath(root, id)
	if err != nil {
		return nil, err
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	q := &questionsFile{id: id, root: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{seq}}, seq: seq}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return q, nil
	} else if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var n yaml.Node
	if err := yaml.Unmarshal(data, &n); err != nil {
		return nil, fail("%s/questions.yaml: %s", id, firstLine(err.Error()))
	}
	if n.Kind == 0 { // empty file
		return q, nil
	}
	top := n.Content[0]
	if top.Kind == yaml.ScalarNode && top.ShortTag() == "!!null" {
		top.Kind, top.Tag, top.Value = yaml.SequenceNode, "!!seq", ""
	}
	if top.Kind != yaml.SequenceNode {
		return nil, fail("%s/questions.yaml is not a list", id)
	}
	q.root, q.seq = &n, top
	for _, c := range top.Content {
		if c = resolveAlias(c); c.Kind == yaml.MappingNode {
			q.items = append(q.items, &doc{m: c})
		} else {
			q.items = append(q.items, nil)
		}
	}
	return q, nil
}

func (q *questionsFile) write(root string) error {
	file, err := questionsPath(root, q.id)
	if err != nil {
		return err
	}
	return writeAtomic(file, (&doc{root: q.root}).encode())
}

// number is a question's n: a whole number of 1 or more, else 0.
func number(d *doc) int {
	n := d.get("n")
	if isNull(n) || n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" {
		return 0
	}
	v, err := strconv.Atoi(n.Value)
	if err != nil || v < 1 {
		return 0
	}
	return v
}

// usable refuses to write a file kanban can't read whole: an entry that isn't a mapping or has no n.
func (q *questionsFile) usable() error {
	for i, d := range q.items {
		if d == nil || number(d) == 0 {
			return fail("%s/questions.yaml entry %d has no n (kanban lint)", q.id, i+1)
		}
	}
	return nil
}

// find is the question numbered n, refusing one that is missing or in the file twice.
func (q *questionsFile) find(n int) (*doc, error) {
	var found *doc
	for _, d := range q.items {
		if d != nil && number(d) == n {
			if found != nil {
				return nil, fail("%s#%d is in questions.yaml twice (kanban lint)", q.id, n)
			}
			found = d
		}
	}
	if found == nil {
		return nil, fail("no question %s#%d", q.id, n)
	}
	return found, nil
}

// loggedNumber is the highest question number log.jsonl has for the task, so that a question
// removed from questions.yaml by hand still keeps its number.
func loggedNumber(root, id string) int {
	f, err := os.Open(filepath.Join(ledgerDir(root), "log.jsonl"))
	if err != nil {
		return 0
	}
	defer f.Close()
	top := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var l struct {
			ID     string         `json:"id"`
			Change map[string]any `json:"change"`
		}
		if json.Unmarshal(sc.Bytes(), &l) != nil || canonID(l.ID) != id {
			continue
		}
		if n, ok := l.Change["question"].(float64); ok {
			top = max(top, int(n))
		}
	}
	return top
}

func intNode(n int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(n)}
}

func logQuestion(root, id string, n int, status string) error {
	change := &omap{}
	change.set("question", n)
	change.set("status", status)
	return appendLog(root, id, change)
}

// questionJSON is a question as plain values, named and with its task's title.
func questionJSON(id, title string, d *doc) *omap {
	o := &omap{}
	o.set("name", fmt.Sprintf("%s#%d", id, number(d)))
	o.set("id", id)
	o.set("title", title)
	v := value(d.m).(*omap)
	for _, k := range v.keys {
		o.set(k, v.vals[k])
	}
	return o
}

// questionText is the text after <id> (joined), or stdin for "-"; trailing blank lines go.
func questionText(e env, words []string) (string, error) {
	s := strings.Join(words, " ")
	if len(words) == 1 && words[0] == "-" {
		var err error
		if s, err = readInput(e, "-"); err != nil {
			return "", err
		}
	}
	return strings.TrimRight(s, " \t\r\n"), nil
}

func cmdAsk(e env, a args) error {
	const usage = `usage: kanban ask <id> "<text>" [--kind business|users|promise|irreversible|line]  (text - reads stdin)`
	if len(a.rest) < 2 || a.rest[0] == "" {
		return fail(usage)
	}
	id := canonID(a.rest[0])
	kind := a.flags["kind"]
	if a.has("kind") && !slices.Contains(questionKinds, kind) {
		return fail("--kind must be one of %s", strings.Join(questionKinds, ", "))
	}
	text, err := questionText(e, a.rest[1:])
	if err != nil {
		return err
	}
	if text == "" {
		return fail("the question has no text")
	}
	var out *omap
	err = withLock(e.root, lockWait, func() error {
		t, err := readTask(e.root, id)
		if err != nil {
			return err
		}
		q, err := readQuestions(e.root, id)
		if err != nil {
			return err
		}
		if err := q.usable(); err != nil {
			return err
		}
		n := loggedNumber(e.root, id)
		for _, d := range q.items {
			n = max(n, number(d))
		}
		n++
		d := newDoc()
		d.set("n", intNode(n))
		d.setStr("text", text)
		if kind != "" {
			d.setStr("kind", kind)
		}
		d.setStr("asked", now())
		d.setStr("askedBy", who())
		d.setStr("status", "open")
		q.seq.Content = append(q.seq.Content, d.m)
		if err := q.write(e.root); err != nil {
			return err
		}
		out = questionJSON(id, t.d.str("title"), d)
		return logQuestion(e.root, id, n, "open")
	})
	if err != nil {
		return err
	}
	e.out(a, out, out.vals["name"].(string))
	return nil
}

// parseRef reads <id>#<n>.
func parseRef(s string) (string, int, error) {
	i := strings.LastIndex(s, "#")
	if i <= 0 {
		return "", 0, fail("%q is not <id>#<n>", s)
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil || n < 1 {
		return "", 0, fail("%q is not <id>#<n>", s)
	}
	return canonID(s[:i]), n, nil
}

// moveQuestion moves <id>#<n> from status from to status to under the lock, setting keys.
func moveQuestion(e env, a args, ref, from, to string, set func(d *doc)) error {
	id, n, err := parseRef(ref)
	if err != nil {
		return err
	}
	var out *omap
	err = withLock(e.root, lockWait, func() error {
		t, err := readTask(e.root, id)
		if err != nil {
			return err
		}
		q, err := readQuestions(e.root, id)
		if err != nil {
			return err
		}
		if err := q.usable(); err != nil {
			return err
		}
		d, err := q.find(n)
		if err != nil {
			return err
		}
		if s := d.str("status"); s != from {
			return fail("%s is %s, not %s", ref, map[bool]string{true: "without a status", false: s}[s == ""], from)
		}
		set(d)
		d.setStr("status", to)
		if err := q.write(e.root); err != nil {
			return err
		}
		out = questionJSON(id, t.d.str("title"), d)
		return logQuestion(e.root, id, n, to)
	})
	if err != nil {
		return err
	}
	e.out(a, out, ref+" "+to)
	return nil
}

func cmdAnswer(e env, a args) error {
	const usage = `usage: kanban answer <id>#<n> "<answer>"  (answer - reads stdin)`
	if len(a.rest) < 2 {
		return fail(usage)
	}
	answer, err := questionText(e, a.rest[1:])
	if err != nil {
		return err
	}
	if answer == "" {
		return fail("the answer has no text")
	}
	return moveQuestion(e, a, a.rest[0], "open", "answered", func(d *doc) {
		d.setStr("answer", answer)
		d.setStr("answered", now())
		d.setStr("answeredBy", who())
	})
}

func cmdReceived(e env, a args) error {
	if len(a.rest) != 1 {
		return fail("usage: kanban received <id>#<n>")
	}
	return moveQuestion(e, a, a.rest[0], "answered", "received", func(d *doc) {
		d.setStr("received", now())
	})
}

// age is how long ago a UTC time was, shortly: 5m, 3h, 2d ("?" when it can't be read).
func age(at string) string {
	t, err := time.Parse(timeLayout, at)
	if err != nil {
		return "?"
	}
	d := clock().Sub(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", max(0, int(d.Minutes())))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func cmdQuestions(e env, a args) error {
	type entry struct {
		id, title string
		d         *doc
	}
	var found []entry
	var broken []string
	for _, id := range ids(e.root) {
		q, err := readQuestions(e.root, id)
		if err != nil {
			broken = append(broken, fmt.Sprintf("%s: %s", id, firstLine(err.Error())))
			continue
		}
		title := ""
		if t, err := readTask(e.root, id); err == nil {
			title = t.d.str("title")
		}
		for _, d := range q.items {
			if d == nil {
				continue
			}
			if s := d.str("status"); s == "open" || (s == "answered" && !a.has("open")) {
				found = append(found, entry{id, title, d})
			}
		}
	}
	// Oldest first; ties in task order (ids are already sorted) and by n.
	sort.SliceStable(found, func(i, j int) bool {
		x, y := found[i], found[j]
		if ax, ay := x.d.str("asked"), y.d.str("asked"); ax != ay {
			return ax < ay
		}
		return x.id == y.id && number(x.d) < number(y.d)
	})
	values := []any{}
	lines := []string{}
	for _, f := range found {
		values = append(values, questionJSON(f.id, f.title, f.d))
		lines = append(lines, strings.Join([]string{
			padEnd(fmt.Sprintf("%s#%d", f.id, number(f.d)), 8), padEnd(f.d.str("status"), 8), padEnd(age(f.d.str("asked")), 4),
			f.title, "—", firstLine(f.d.str("text")),
		}, " "))
	}
	for _, b := range broken {
		o := &omap{}
		id, msg, _ := strings.Cut(b, ": ")
		o.set("id", id)
		o.set("broken", msg)
		values = append(values, o)
		lines = append(lines, b+"  (questions.yaml unreadable)")
	}
	text := strings.Join(lines, "\n")
	if text == "" {
		text = "(no questions)"
	}
	e.out(a, values, text)
	return nil
}

// notReceived are the questions that keep a task from done: open or answered.
func notReceived(q *questionsFile) (nums []int, names []string) {
	for _, d := range q.items {
		if d == nil {
			continue
		}
		if s := d.str("status"); s == "open" || s == "answered" {
			nums = append(nums, number(d))
			names = append(names, fmt.Sprintf("%s#%d (%s)", q.id, number(d), s))
		}
	}
	return nums, names
}

// questionCounts are open, answered and received, for kanban show.
func questionCounts(q *questionsFile) map[string]int {
	c := map[string]int{}
	for _, d := range q.items {
		if d != nil {
			c[d.str("status")]++
		}
	}
	return c
}

func lintQuestions(root string) []string {
	var out []string
	for _, id := range ids(root) {
		q, err := readQuestions(root, id)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %s", id, firstLine(err.Error())))
			continue
		}
		seen := map[int]bool{}
		for i, d := range q.items {
			at := fmt.Sprintf("%s: questions.yaml entry %d", id, i+1)
			if d == nil {
				out = append(out, at+" is not a mapping")
				continue
			}
			n := number(d)
			if n == 0 {
				out = append(out, at+": n is not a whole number of 1 or more")
			} else {
				at = fmt.Sprintf("%s#%d", id, n)
				if seen[n] {
					out = append(out, at+": n is used twice")
				}
				seen[n] = true
			}
			if strings.TrimSpace(d.str("text")) == "" {
				out = append(out, at+": no text")
			}
			status := d.str("status")
			if !slices.Contains(questionStatuses, status) {
				out = append(out, fmt.Sprintf("%s: status %q is not open, answered or received", at, status))
			}
			if k := d.get("kind"); !isNull(k) && !slices.Contains(questionKinds, d.str("kind")) {
				out = append(out, fmt.Sprintf("%s: kind %q is not %s", at, k.Value, strings.Join(questionKinds, ", ")))
			}
			for _, k := range []string{"asked", "answered", "received"} {
				if v := d.get(k); !isNull(v) {
					if _, err := time.Parse(timeLayout, d.str(k)); err != nil {
						out = append(out, fmt.Sprintf("%s: %s %q is not a UTC time", at, k, v.Value))
					}
				}
			}
			needs := map[string][]string{"open": {"asked"}, "answered": {"asked", "answer", "answered"}, "received": {"asked", "answer", "answered", "received"}}
			nots := map[string][]string{"open": {"answer", "answered", "received"}, "answered": {"received"}}
			for _, k := range needs[status] {
				if isNull(d.get(k)) {
					out = append(out, fmt.Sprintf("%s: %s but no %s", at, status, k))
				}
			}
			for _, k := range nots[status] {
				if !isNull(d.get(k)) {
					out = append(out, fmt.Sprintf("%s: %s but has %s", at, status, k))
				}
			}
		}
	}
	return out
}
