package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// keyOrder is where kanban puts a key it adds. Keys a person put elsewhere stay where they are.
var keyOrder = []string{"id", "title", "status", "repo", "parent", "phase", "after", "profile", "who", "agent", "branch", "worktree", "links", "routine", "not_before", "created", "updated"}

// doc is one YAML file kept as a node tree, so that changing a value keeps the
// comments and the order of everything else.
type doc struct {
	root *yaml.Node // the document node
	m    *yaml.Node // its top-level mapping
}

func parseDoc(data []byte) (*doc, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root.Kind == 0 { // empty file
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	m := root.Content[0]
	if m.Kind == yaml.ScalarNode && m.ShortTag() == "!!null" {
		m.Kind, m.Tag, m.Value = yaml.MappingNode, "!!map", ""
	}
	if m.Kind != yaml.MappingNode {
		return nil, errors.New("not a mapping")
	}
	return &doc{root: &root, m: m}, nil
}

func newDoc() *doc {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	return &doc{root: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}}, m: m}
}

func (d *doc) encode() []byte {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(d.root); err != nil {
		panic(err) // a tree built from a parse or from strings always encodes
	}
	enc.Close()
	return b.Bytes()
}

func (d *doc) index(key string) int {
	for i := 0; i+1 < len(d.m.Content); i += 2 {
		if d.m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func (d *doc) get(key string) *yaml.Node {
	if i := d.index(key); i >= 0 {
		return resolveAlias(d.m.Content[i+1])
	}
	return nil
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func isNull(n *yaml.Node) bool {
	return n == nil || (n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null")
}

// str is a scalar's text, or "" when the key is missing, null or not a scalar.
func (d *doc) str(key string) string {
	n := d.get(key)
	if isNull(n) || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// list reads a sequence of scalars; a single scalar counts as a list of one.
func (d *doc) list(key string) []string {
	n := d.get(key)
	switch {
	case isNull(n):
		return nil
	case n.Kind == yaml.ScalarNode:
		return []string{n.Value}
	case n.Kind == yaml.SequenceNode:
		out := []string{}
		for _, c := range n.Content {
			if c = resolveAlias(c); c.Kind == yaml.ScalarNode && !isNull(c) {
				out = append(out, c.Value)
			}
		}
		return out
	}
	return nil
}

func strNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func listNode(items []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, s := range items {
		n.Content = append(n.Content, strNode(s))
	}
	return n
}

func (d *doc) setStr(key, value string) { d.set(key, strNode(value)) }

func (d *doc) setList(key string, items []string) {
	if len(items) == 0 {
		d.del(key)
		return
	}
	d.set(key, listNode(items))
}

// set replaces a value in place (keeping the comments around it), or adds the key
// next to its neighbours in keyOrder.
func (d *doc) set(key string, value *yaml.Node) {
	if i := d.index(key); i >= 0 {
		old := d.m.Content[i+1]
		value.HeadComment, value.LineComment, value.FootComment = old.HeadComment, old.LineComment, old.FootComment
		if old.Kind == value.Kind && value.Kind == yaml.SequenceNode {
			value.Style = old.Style
		}
		d.m.Content[i+1] = value
		return
	}
	at := d.insertAt(key)
	k := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	if at == 0 && len(d.m.Content) > 0 {
		// The comment at the top of the file stays at the top.
		k.HeadComment, d.m.Content[0].HeadComment = d.m.Content[0].HeadComment, ""
	}
	d.m.Content = slices.Insert(d.m.Content, at, k, value)
}

func (d *doc) insertAt(key string) int {
	rank := slices.Index(keyOrder, key)
	if rank < 0 {
		return len(d.m.Content)
	}
	after, before := -1, -1
	for i := 0; i+1 < len(d.m.Content); i += 2 {
		r := slices.Index(keyOrder, d.m.Content[i].Value)
		if r < 0 {
			continue
		}
		if r < rank {
			after = i
		} else if before < 0 {
			before = i
		}
	}
	switch {
	case after >= 0:
		return after + 2
	case before >= 0:
		return before
	}
	return len(d.m.Content)
}

func (d *doc) del(key string) {
	if i := d.index(key); i >= 0 {
		// The value goes; a person's comments around it stay, on the next key.
		k, v := d.m.Content[i], d.m.Content[i+1]
		kept := joinComments(joinComments(k.HeadComment, k.FootComment), v.FootComment)
		d.m.Content = slices.Delete(d.m.Content, i, i+2)
		switch {
		case kept == "":
		case i < len(d.m.Content):
			d.m.Content[i].HeadComment = joinComments(kept, d.m.Content[i].HeadComment)
		case i > 0:
			d.m.Content[i-2].FootComment = joinComments(d.m.Content[i-2].FootComment, kept)
		default:
			d.m.FootComment = joinComments(d.m.FootComment, kept)
		}
	}
}

func joinComments(a, b string) string {
	return strings.Trim(a+"\n"+b, "\n")
}

// value turns a node into plain values for JSON: mappings keep their order.
func value(n *yaml.Node) any {
	n = resolveAlias(n)
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil
		}
		return value(n.Content[0])
	case yaml.MappingNode:
		o := &omap{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			o.set(n.Content[i].Value, value(n.Content[i+1]))
		}
		return o
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			out = append(out, value(c))
		}
		return out
	}
	switch n.ShortTag() {
	case "!!null":
		return nil
	case "!!int", "!!float", "!!bool":
		var v any
		if n.Decode(&v) == nil {
			return v
		}
	}
	return n.Value // strings, and timestamps as written
}
