// Package board は andon issue board と andon issue history（ADR-0059 決めたこと 6〜10）。
// ボード（mod kanban-board）が kanban を読み直すたびに呼ぶ、読むだけの 2 つの command で、kanban の issues/<id>/issue.yaml を
// 自分で読み、要る課題だけを返す。kanban のファイルも履歴も書かず、kanban の CLI も呼ばない（根を聞く kanban root を除く）。
// 読む前に symlink と大きさを確かめ、YAML の alias を開かない（決めたこと 10）。守りに当たらない課題の中身は、
// kanban list --json --all が出す課題と同じにする（受け入れ条件 5）。
package board

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"andon/internal/ledger"

	"go.yaml.in/yaml/v3"
)

// Version は出力の version（docs/contract.md §29・schema/issue-board.schema.json）。
const Version = 1

// MaxFile は読む issue.yaml の大きさの上限（mod の今の読み方と同じ 256 KiB）。
const MaxFile = 256 * 1024

// History の件数の既定と上限（決めたこと 8）。
const (
	DefaultLimit = 200
	MaxLimit     = 2000
)

const timeLayout = "2006-01-02T15:04:05Z"

// Result は 2 つの command の --json の出力。board は Since と Older を持ち、history は More を持つ。
type Result struct {
	Version int              `json:"version"`
	At      string           `json:"at"`
	Project string           `json:"project"`
	Done    string           `json:"done"`
	Since   string           `json:"since,omitempty"`
	Issues  []map[string]any `json:"issues"`
	Older   *int             `json:"older,omitempty"`
	More    *int             `json:"more,omitempty"`
}

type issue struct {
	id      string // フォルダの名前
	key     string // ledger.CanonID(id)。親と after を引く鍵
	broken  string
	value   map[string]any // 出力する中身（kanban list --json と同じ。id を含む）
	status  string
	parent  string   // CanonID
	after   []string // CanonID
	updated time.Time
	hasTime bool // updated が時刻として読めた
	// ownBroken は読めた課題が自分の broken のキーに空でない値（JavaScript で真）を持つこと
	ownBroken bool
}

type kanban struct {
	project string
	done    string
	issues  []*issue // 番号の順
	byKey   map[string]*issue
}

// isDone は片付いた課題か。読めない課題と、自分の broken のキーに空でない値を持つ課題（読み手は読めない課題として扱う。
// contract.md §29）は片付いた課題に数えず、範囲のアに入れる。
func (t *issue) isDone(done string) bool { return t.broken == "" && !t.ownBroken && t.status == done }

// read は kanban の根 root（projects/<slug>/。kanban/ の親）を読む。
func read(root string) (*kanban, error) {
	dir := filepath.Join(root, "kanban")
	cfg, err := ledger.ReadConfig(dir)
	if err != nil {
		return nil, fmt.Errorf("kanban が読めない（%s）: %v", filepath.Join(dir, "config.yaml"), err)
	}
	if !cfg.VersionIs(3) {
		return nil, fmt.Errorf("kanban の形式が 3 でない（%s の version: %q）", filepath.Join(dir, "config.yaml"), cfg.Version)
	}
	k := &kanban{project: filepath.Base(filepath.Clean(root)), done: "done", byKey: map[string]*issue{}}
	if len(cfg.Columns) > 0 {
		k.done = cfg.Columns[len(cfg.Columns)-1]
	}
	issuesDir := filepath.Join(dir, "issues")
	if fi, err := os.Lstat(issuesDir); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s が symlink（読まない）", issuesDir)
	}
	ents, err := os.ReadDir(issuesDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s が読めない: %v", issuesDir, err)
	}
	for _, e := range ents {
		t := readIssue(issuesDir, e)
		if t == nil {
			continue
		}
		k.issues = append(k.issues, t)
	}
	sort.SliceStable(k.issues, func(i, j int) bool { return ledger.IDLess(k.issues[i].id, k.issues[j].id) })
	for _, t := range k.issues {
		if _, dup := k.byKey[t.key]; !dup {
			k.byKey[t.key] = t
		}
	}
	return k, nil
}

// readIssue は issues/ の 1 つの項目を読む。課題でない項目（フォルダでないファイル）は nil。
func readIssue(issuesDir string, e fs.DirEntry) *issue {
	id := e.Name()
	t := &issue{id: id, key: ledger.CanonID(id)}
	folder := filepath.Join(issuesDir, id)
	if e.Type()&fs.ModeSymlink != 0 {
		// 先がフォルダの symlink だけを課題に数える（先の中身は読まない。Stat は先の種類を見るだけ）
		if fi, err := os.Stat(folder); err != nil || !fi.IsDir() {
			return nil
		}
		t.broken = "symbolic link"
		return t
	}
	if !e.IsDir() {
		return nil
	}
	file := filepath.Join(folder, "issue.yaml")
	fi, err := os.Lstat(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		t.broken = "no issue " + id // kanban の CLI と同じ（フォルダはあるが issue.yaml が無い）
		return t
	case err != nil:
		t.broken = firstLine(err.Error())
		return t
	case fi.Mode()&fs.ModeSymlink != 0:
		t.broken = "symbolic link"
		return t
	case !fi.Mode().IsRegular():
		t.broken = "not a file"
		return t
	case fi.Size() > MaxFile:
		t.broken = fmt.Sprintf("larger than %d bytes", MaxFile)
		return t
	}
	b, err := readLimited(file, fi)
	if err != nil {
		t.broken = firstLine(err.Error())
		return t
	}
	m, err := parse(b)
	if err != nil {
		if err == errNotMapping {
			t.broken = id + "/issue.yaml is not a mapping"
		} else {
			t.broken = firstLine(err.Error())
		}
		return t
	}
	t.fill(m)
	return t
}

// readLimited は file を読む。Lstat（fi）のあとに symlink や別のファイルに替わっていれば読まず、大きくなっていても MaxFile を超えて読まない。
func readLimited(file string, fi fs.FileInfo) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // 読むだけのファイルで、閉じる誤りに打つ手が無い
	if now, err := f.Stat(); err != nil {
		return nil, err
	} else if !os.SameFile(fi, now) {
		return nil, errors.New("symbolic link")
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxFile {
		return nil, fmt.Errorf("larger than %d bytes", MaxFile)
	}
	return b, nil
}

var (
	errNotMapping = errors.New("not a mapping")
	errAlias      = errors.New("has a YAML alias")
)

// parse は issue.yaml を node の木に読み、alias があれば開かずに断る。空のファイルと null は空の mapping（kanban の CLI と同じ）。
func parse(b []byte) (*yaml.Node, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	if root.Kind == 0 {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	if hasAlias(&root) {
		return nil, errAlias
	}
	m := root.Content[0]
	if m.Kind == yaml.ScalarNode && m.ShortTag() == "!!null" {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	if m.Kind != yaml.MappingNode {
		return nil, errNotMapping
	}
	return m, nil
}

func hasAlias(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode {
		return true
	}
	for _, c := range n.Content {
		if hasAlias(c) {
			return true
		}
	}
	return false
}

func (t *issue) fill(m *yaml.Node) {
	t.value = map[string]any{}
	var status, parent, updated *yaml.Node
	var after *yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i].Value, m.Content[i+1]
		t.value[k] = value(v)
		switch k {
		case "status":
			status = v
		case "parent":
			parent = v
		case "after":
			after = v
		case "updated":
			updated = v
		}
	}
	t.value["id"] = t.id
	t.ownBroken = truthy(t.value["broken"])
	t.status = scalar(status)
	if p := scalar(parent); p != "" {
		t.parent = ledger.CanonID(p)
	}
	t.after = list(after)
	if s := scalar(updated); s != "" {
		if at, err := time.Parse(time.RFC3339, s); err == nil {
			t.updated, t.hasTime = at, true
		}
	}
}

// truthy は JSON の値が JavaScript で真か（読み手の mod は、broken が真の課題を読めない課題として描く）。
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int:
		return x != 0
	case int64:
		return x != 0
	case uint64:
		return x != 0
	case float64:
		return x != 0 && !math.IsNaN(x)
	}
	return true
}

// scalar はスカラーの字面。無い・null・スカラーでなければ ""（kanban の CLI の str と同じ）。
func scalar(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.ShortTag() == "!!null" {
		return ""
	}
	return n.Value
}

// list はスカラーの並び。1 つのスカラーは 1 つの並び（kanban の CLI の list と同じ）。CanonID にして返す。
func list(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	switch n.Kind {
	case yaml.ScalarNode:
		if s := scalar(n); s != "" {
			out = append(out, ledger.CanonID(s))
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if s := scalar(c); s != "" {
				out = append(out, ledger.CanonID(s))
			}
		}
	}
	return out
}

// value は node を JSON の値にする（kanban の CLI の value と同じ。alias は parse で断ってある）。
// 有限でない数（.inf・.nan）は JSON に書けないので字面のまま出す。
func value(n *yaml.Node) any {
	switch n.Kind {
	case yaml.MappingNode:
		o := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			o[n.Content[i].Value] = value(n.Content[i+1])
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
			if f, ok := v.(float64); ok && (math.IsInf(f, 0) || math.IsNaN(f)) {
				return n.Value
			}
			return v
		}
	}
	return n.Value
}

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }

// out は課題の出力の中身（読めない課題は id と broken だけ）。
func (t *issue) out() map[string]any {
	if t.broken != "" {
		return map[string]any{"id": t.id, "broken": t.broken}
	}
	return t.value
}

// Rule は範囲の組の名前（決めたこと 7 のア〜オ）。検査が 1 つを抜いて落ちることを確かめるのに使う。
type Rule string

const (
	RuleOpen      Rule = "a" // ア: 片付いていない課題と読めない課題
	RuleRecent    Rule = "i" // イ: updated が since 以降の片付いた課題
	RuleAncestors Rule = "u" // ウ: アとイの親をたどった先
	RuleSiblings  Rule = "e" // エ: アの親をたどった先のどれかを親に持つ課題
	RuleAfter     Rule = "o" // オ: アとイの after の相手
)

var AllRules = []Rule{RuleOpen, RuleRecent, RuleAncestors, RuleSiblings, RuleAfter}

// Board は andon issue board の中身: 範囲の課題（番号の順）と older。
func Board(root string, since, now time.Time) (Result, error) {
	return board(root, since, now, AllRules)
}

func board(root string, since, now time.Time, rules []Rule) (Result, error) {
	k, err := read(root)
	if err != nil {
		return Result{}, err
	}
	on := map[Rule]bool{}
	for _, r := range rules {
		on[r] = true
	}
	// アとイ（どちらも 1 つの課題について決まる）と older
	var open, recent []*issue
	older := 0
	for _, t := range k.issues {
		switch {
		case !t.isDone(k.done):
			open = append(open, t)
		case !t.hasTime || !t.updated.Before(since):
			// updated が無い・読めない片付いた課題は線の内側に入れる（mod が自分の線で並べ直す。欠けるより多い方がよい）
			recent = append(recent, t)
		default:
			older++
		}
	}
	in := map[string]bool{}
	add := func(t *issue) {
		if t != nil {
			in[t.id] = true
		}
	}
	// ancestors は t の親をたどった先の全部（循環で止まる）
	ancestors := func(t *issue) []*issue {
		var out []*issue
		seen := map[string]bool{t.id: true}
		for p := k.byKey[t.parent]; p != nil && !seen[p.id]; p = k.byKey[p.parent] {
			seen[p.id] = true
			out = append(out, p)
		}
		return out
	}
	if on[RuleOpen] {
		for _, t := range open {
			add(t)
		}
	}
	if on[RuleRecent] {
		for _, t := range recent {
			add(t)
		}
	}
	both := append(append([]*issue{}, open...), recent...)
	if on[RuleAncestors] {
		for _, t := range both {
			for _, a := range ancestors(t) {
				add(a)
			}
		}
	}
	if on[RuleSiblings] {
		openAnc := map[string]bool{}
		for _, t := range open {
			for _, a := range ancestors(t) {
				openAnc[a.key] = true
			}
		}
		for _, t := range k.issues {
			if t.parent != "" && openAnc[t.parent] {
				add(t)
			}
		}
	}
	if on[RuleAfter] {
		for _, t := range both {
			for _, a := range t.after {
				add(k.byKey[a])
			}
		}
	}
	res := Result{Version: Version, At: now.UTC().Format(timeLayout), Project: k.project, Done: k.done,
		Since: since.UTC().Format(time.RFC3339Nano), Issues: []map[string]any{}, Older: &older}
	for _, t := range k.issues {
		if in[t.id] {
			res.Issues = append(res.Issues, t.out())
		}
	}
	return res, nil
}

// History は andon issue history の中身: 片付いた課題を updated の新しい順に limit 件まで。match は番号・タイトル・repo・
// プロファイル・端末の名前のどれかに含む課題だけを残す（大文字と小文字を分けない）。more は条件に合うのに出さなかった数。
func History(root string, limit int, match string, now time.Time) (Result, error) {
	k, err := read(root)
	if err != nil {
		return Result{}, err
	}
	needle := strings.ToLower(match)
	var hit []*issue
	for _, t := range k.issues {
		if !t.isDone(k.done) {
			continue
		}
		if needle != "" && !t.matches(needle) {
			continue
		}
		hit = append(hit, t)
	}
	sort.SliceStable(hit, func(i, j int) bool {
		a, b := hit[i], hit[j]
		if a.hasTime != b.hasTime {
			return a.hasTime // 時刻の読めない課題は最後
		}
		if !a.updated.Equal(b.updated) {
			return a.updated.After(b.updated)
		}
		return ledger.IDLess(b.id, a.id) // 同じ時刻は番号の新しい順
	})
	more := 0
	if len(hit) > limit {
		more = len(hit) - limit
		hit = hit[:limit]
	}
	res := Result{Version: Version, At: now.UTC().Format(timeLayout), Project: k.project, Done: k.done,
		Issues: make([]map[string]any, 0, len(hit)), More: &more}
	for _, t := range hit {
		res.Issues = append(res.Issues, t.out())
	}
	return res, nil
}

func (t *issue) matches(needle string) bool {
	fields := []string{t.id}
	for _, k := range []string{"title", "repo", "profile", "agent"} {
		if s, ok := t.value[k].(string); ok {
			fields = append(fields, s)
		} else if v, ok := t.value[k]; ok && v != nil {
			fields = append(fields, fmt.Sprint(v))
		}
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), needle) {
			return true
		}
	}
	return false
}
