package ledger

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// ErrNotMapping は config.yaml の一番上が mapping でないこと。
var ErrNotMapping = errors.New("not a mapping")

// Config は台帳の config.yaml（docs/ledger.md）。読み手はこれ1つで、版を確かめるのは呼び手。
type Config struct {
	HasVersion   bool   // version があり null でない
	Version      string // version の字面
	IntVersion   bool   // version が YAML の整数（2。"2" は文字列）
	HasPrefix    bool   // prefix が null でないスカラー
	Prefix       string
	Columns      []string // 空でない並びのときだけ（中のスカラーだけ）。無ければ nil
	Repos        []string // スカラーの並び（1つのスカラーは1つの並び）
	Timezone     string
	ChiefOfStaff string
}

// VersionIs は version が整数の n か。
func (c Config) VersionIs(n int) bool { return c.IntVersion && c.Version == strconv.Itoa(n) }

// ReadConfig は台帳（kanban/）の dir の config.yaml を読む。読めない・YAML でなければその誤り、一番上が mapping でなければ ErrNotMapping。
func ReadConfig(dir string) (Config, error) {
	b, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return Config{}, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &root); err != nil {
		return Config{}, err
	}
	m := &yaml.Node{Kind: yaml.MappingNode}
	if root.Kind != 0 {
		m = root.Content[0]
	}
	if isNull(m) {
		m = &yaml.Node{Kind: yaml.MappingNode}
	}
	if m.Kind != yaml.MappingNode {
		return Config{}, ErrNotMapping
	}
	get := func(key string) *yaml.Node {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				return alias(m.Content[i+1])
			}
		}
		return nil
	}
	str := func(key string) string {
		if n := get(key); !isNull(n) && n.Kind == yaml.ScalarNode {
			return n.Value
		}
		return ""
	}
	var c Config
	if v := get("version"); !isNull(v) {
		c.HasVersion, c.Version = true, v.Value
		if v.Kind == yaml.ScalarNode && v.ShortTag() == "!!int" {
			_, err := strconv.Atoi(v.Value)
			c.IntVersion = err == nil
		}
	}
	if n := get("prefix"); !isNull(n) && n.Kind == yaml.ScalarNode {
		c.HasPrefix, c.Prefix = true, n.Value
	}
	if n := get("columns"); n != nil && n.Kind == yaml.SequenceNode && len(n.Content) > 0 {
		c.Columns = scalars(n)
	}
	c.Repos = scalars(get("repos"))
	c.Timezone = str("timezone")
	c.ChiefOfStaff = str("chief_of_staff")
	return c, nil
}

// Config は台帳の config.yaml。
func (l *Ledger) Config() (Config, error) { return ReadConfig(l.Dir) }

func alias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func isNull(n *yaml.Node) bool {
	return n == nil || (n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null")
}

// scalars はスカラーの並び（1つのスカラーは1つの並び。null でないスカラーだけ）。
func scalars(n *yaml.Node) []string {
	switch {
	case isNull(n):
		return nil
	case n.Kind == yaml.ScalarNode:
		return []string{n.Value}
	case n.Kind == yaml.SequenceNode:
		out := []string{}
		for _, c := range n.Content {
			if c = alias(c); c.Kind == yaml.ScalarNode && !isNull(c) {
				out = append(out, c.Value)
			}
		}
		return out
	}
	return nil
}
