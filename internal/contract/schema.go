package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"andon"

	"github.com/google/jsonschema-go/jsonschema"
)

// Schema は約束の JSON の schema の名前（schema/<名前>.schema.json。docs/contract.md の冒頭の表）。
// 約束の JSON は Write・Encode・EncodeLine で、書く前にこの schema と照らす（ADR-0034 決めたこと 6）。
type Schema string

// schema/ の直下の 17 本。
const (
	SchemaCandidates   Schema = "candidates"
	SchemaCorrections  Schema = "corrections"
	SchemaDelivered    Schema = "delivered"
	SchemaFindings     Schema = "findings"
	SchemaGate         Schema = "gate"
	SchemaIssueKeys    Schema = "issue-keys"
	SchemaLaunch       Schema = "launch"
	SchemaMeasure      Schema = "measure"
	SchemaNotification Schema = "notification"
	SchemaOutcome      Schema = "outcome"
	SchemaPatterns     Schema = "patterns"
	SchemaProject      Schema = "project"
	SchemaQuestion     Schema = "question"
	SchemaReport       Schema = "report"
	SchemaRetro        Schema = "retro"
	SchemaRun          Schema = "run"
	SchemaSignal       Schema = "signal"
)

// Schemas は schema/ の直下の全部（テストが、埋め込んだ顔ぶれと突き合わせる）。
var Schemas = []Schema{SchemaCandidates, SchemaCorrections, SchemaDelivered, SchemaFindings, SchemaGate, SchemaIssueKeys,
	SchemaLaunch, SchemaMeasure, SchemaNotification, SchemaOutcome, SchemaPatterns, SchemaProject, SchemaQuestion,
	SchemaReport, SchemaRetro, SchemaRun, SchemaSignal}

var (
	resolvedMu sync.Mutex
	resolved   = map[Schema]*Checker{}
	filesMu    sync.Mutex
	files      = map[string]*Checker{}
)

// File は埋め込んだ schema の置き場（repo からの相対）。
func (s Schema) File() string { return "schema/" + string(s) + ".schema.json" }

// compile は埋め込んだ schema を読んで解く（1度だけ。読めなければ毎回誤りを返す）。
func (s Schema) compile() (*Checker, error) {
	resolvedMu.Lock()
	defer resolvedMu.Unlock()
	if c := resolved[s]; c != nil {
		return c, nil
	}
	b, err := andon.Schemas.ReadFile(s.File())
	if err != nil {
		return nil, fmt.Errorf("%s が埋め込まれていない: %v", s.File(), err)
	}
	c, err := Compile(s.File(), b)
	if err != nil {
		return nil, err
	}
	resolved[s] = c
	return c, nil
}

// Check は JSON の b が schema に合うかを確かめる。合わなければ、どこがどう合わないかの誤り（*MismatchError）。
func (s Schema) Check(b []byte) error {
	c, err := s.compile()
	if err != nil {
		return err
	}
	return c.Check(b)
}

// ParseSchema は名前（schema/ の直下の report など）を埋め込んだ schema にする。知らない名前は false。
func ParseSchema(name string) (Schema, bool) {
	for _, s := range Schemas {
		if string(s) == name {
			return s, true
		}
	}
	return "", false
}

// Keywords は検査器が読む JSON Schema（draft 2020-12）のキーワード。schema/ が使うものだけで、ほかのキーワードが
// schema にあれば、黙って飛ばさずに検査できないとして断る（検査が効いていないのに通るのを防ぐ）。
// uniqueKeys は標準に無い拡張で、ライブラリは読み飛ばすので Checker が自分で確かめる。
// package/lib/andon/schema_check.py の lint の KEYWORDS と同じ顔ぶれにする。
var Keywords = []string{"$schema", "$id", "title", "description",
	"$defs", "$ref", "type", "properties", "required", "additionalProperties",
	"items", "prefixItems", "enum", "const", "pattern", "minimum", "minLength",
	"minItems", "oneOf", "anyOf", "allOf", "if", "then", "else", "not",
	"uniqueKeys"}

// Checker は1本の schema を解いたもの。照合は既製の検査器（google/jsonschema-go）と、拡張の uniqueKeys。
type Checker struct {
	name     string // 誤りに出す schema の名前（schema/report.schema.json やパス）
	resolved *jsonschema.Resolved
	raw      map[string]any // uniqueKeys を読む生の schema
}

// MismatchError は、検査できたうえで JSON が schema に合わなかったこと。ほかの誤りは検査できなかったこと
// （schema が読めない・知らないキーワード・JSON でない）。
type MismatchError struct {
	Schema string // schema の名前
	Reason string // どこがどう合わないか
}

func (e *MismatchError) Error() string { return e.Schema + " に合わない: " + e.Reason }

// Compile は schema の中身 b を解く。name は誤りに出す名前。知らないキーワードがあれば断る。
func Compile(name string, b []byte) (*Checker, error) {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", name, err)
	}
	if unknown := unknownKeywords(raw, "$", false, nil); len(unknown) > 0 {
		return nil, fmt.Errorf("%s に検査器の知らないキーワード: %s", name, strings.Join(unknown, ", "))
	}
	var js jsonschema.Schema
	if err := json.Unmarshal(b, &js); err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", name, err)
	}
	r, err := js.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("%s が読めない: %v", name, err)
	}
	return &Checker{name: name, resolved: r, raw: raw}, nil
}

// CompileFile は置き場 path の schema を読んで解く（同じ path は1度だけ）。
func CompileFile(path string) (*Checker, error) {
	filesMu.Lock()
	defer filesMu.Unlock()
	if c := files[path]; c != nil {
		return c, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("schema が読めない: %v", err)
	}
	c, err := Compile(path, b)
	if err != nil {
		return nil, err
	}
	files[path] = c
	return c, nil
}

// Check は JSON の b がこの schema に合うかを確かめる。合わなければ *MismatchError、JSON でなければほかの誤り。
func (c *Checker) Check(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("%s と照らせない（JSON でない）: %v", c.name, err)
	}
	return c.CheckValue(v)
}

// CheckValue は JSON を読んだ値 v（map[string]any・[]any・float64 など）がこの schema に合うかを確かめる。
func (c *Checker) CheckValue(v any) error {
	if err := c.resolved.Validate(v); err != nil {
		return &MismatchError{Schema: c.name, Reason: err.Error()}
	}
	if dup := c.uniqueKeys(c.raw, v, "$", 0); len(dup) > 0 {
		return &MismatchError{Schema: c.name, Reason: strings.Join(dup, "; ")}
	}
	return nil
}

// unknownKeywords は schema の中の部分 schema を全部たどり、Keywords に無いキーワードを「場所: キーワード」で返す。
// uniqueKeys を読まない場所（anyOf・oneOf・not・if・then・else の中。branch が true）の uniqueKeys も返す。
// 読まないのに置けると、一意を確かめたつもりで確かめない schema になるため。
func unknownKeywords(s any, at string, branch bool, out []string) []string {
	m, ok := s.(map[string]any)
	if !ok {
		return out
	}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		switch {
		case !slices.Contains(Keywords, k):
			out = append(out, at+": "+k)
		case k == "uniqueKeys" && branch:
			out = append(out, at+": uniqueKeys（anyOf・oneOf・not・if・then・else の中では読まない）")
		}
	}
	for _, k := range []string{"properties", "$defs"} {
		sub, _ := m[k].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(sub)) {
			// $defs は $ref から読むので、置き場が分岐の中でなければ読む
			out = unknownKeywords(sub[name], at+"."+k+"."+name, branch, out)
		}
	}
	for _, k := range []string{"additionalProperties", "items", "not", "if", "then", "else"} {
		inBranch := branch || k == "not" || k == "if" || k == "then" || k == "else"
		out = unknownKeywords(m[k], at+"."+k, inBranch, out)
	}
	for _, k := range []string{"prefixItems", "oneOf", "anyOf", "allOf"} {
		list, _ := m[k].([]any)
		for i, sub := range list {
			inBranch := branch || k == "oneOf" || k == "anyOf"
			out = unknownKeywords(sub, fmt.Sprintf("%s.%s[%d]", at, k, i), inBranch, out)
		}
	}
	return out
}

// uniqueKeys は拡張のキーワード uniqueKeys（配列の object の要素で、名前を並べたキーの値が重ならない）を確かめ、
// 重なりを「場所: 値 が重なる」で返す。キーの無い要素は数えない。等しさは JSON の等しさ（1 と true は別）。
// たどるのは、値の形に沿って決まる部分 schema（$ref・allOf・properties・additionalProperties・items・prefixItems）だけ。
// anyOf・oneOf・not・if・then・else の中の uniqueKeys は読まない（schema_check.py の lint が置かせない）。
func (c *Checker) uniqueKeys(s map[string]any, v any, at string, depth int) []string {
	if s == nil || depth > 256 { // $ref の輪で止まらない schema への守り
		return nil
	}
	var bad []string
	if ref, ok := s["$ref"].(string); ok {
		bad = append(bad, c.uniqueKeys(c.pointer(ref), v, at, depth+1)...)
	}
	for _, sub := range asList(s["allOf"]) {
		m, _ := sub.(map[string]any)
		bad = append(bad, c.uniqueKeys(m, v, at, depth+1)...)
	}
	switch v := v.(type) {
	case []any:
		for _, k := range asList(s["uniqueKeys"]) {
			key, _ := k.(string)
			seen := map[string]bool{}
			for i, x := range v {
				m, ok := x.(map[string]any)
				if !ok {
					continue
				}
				val, ok := m[key]
				if !ok {
					continue
				}
				j, _ := json.Marshal(val)
				if seen[string(j)] {
					bad = append(bad, fmt.Sprintf("%s[%d].%s: %s が重なる", at, i, key, j))
				}
				seen[string(j)] = true
			}
		}
		prefix := asList(s["prefixItems"])
		items, _ := s["items"].(map[string]any)
		for i, x := range v {
			sub := items
			if i < len(prefix) {
				sub, _ = prefix[i].(map[string]any)
			}
			bad = append(bad, c.uniqueKeys(sub, x, fmt.Sprintf("%s[%d]", at, i), depth+1)...)
		}
	case map[string]any:
		props, _ := s["properties"].(map[string]any)
		extra, _ := s["additionalProperties"].(map[string]any)
		for _, k := range slices.Sorted(maps.Keys(v)) {
			sub := extra
			if p, in := props[k]; in {
				sub, _ = p.(map[string]any)
			}
			bad = append(bad, c.uniqueKeys(sub, v[k], at+"."+k, depth+1)...)
		}
	}
	return bad
}

// pointer は同じ schema の中の $ref（#/$defs/x の形）が指す部分 schema。解けなければ nil（ライブラリが先に断る）。
func (c *Checker) pointer(ref string) map[string]any {
	if ref == "#" {
		return c.raw
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	var node any = c.raw
	for _, part := range strings.Split(ref[2:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[part]
	}
	m, _ := node.(map[string]any)
	return m
}

func asList(v any) []any { l, _ := v.([]any); return l }

// Encode は v を約束の JSON（2字下げ・HTML の文字を逃がさない・末尾に改行）にし、schema と照らす。合わなければ誤りで、中身を返さない。
func Encode(s Schema, v any) ([]byte, error) {
	b := Marshal(v)
	if err := s.Check(b); err != nil {
		return nil, err
	}
	return b, nil
}

// EncodeLine は v を1行の約束の JSON（HTML の文字を逃がさない・末尾の改行は無し）にし、schema と照らす。
// 1行1件で足していく置き場（*.jsonl）に使う。
func EncodeLine(s Schema, v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	line := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	if err := s.Check(line); err != nil {
		return nil, err
	}
	return line, nil
}

// Write は v を約束の JSON にして schema と照らし、合えば path に書く（一時ファイルに書いてから置き換える）。
// 合わなければ書かずに誤りを返す（前の中身は残る）。
func Write(path string, s Schema, v any) error {
	b, err := Encode(s, v)
	if err != nil {
		return err
	}
	return writeFile(path, b)
}

// WriteState は約束でない JSON（schema の無い控え。dispatcher の merge の記録など）を書く。照らさない。
// 呼ぶ所は、ソースを読むテスト（writers_test.go）の表に理由と載せる。
func WriteState(path string, v any) error {
	return writeFile(path, Marshal(v))
}

func writeFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
