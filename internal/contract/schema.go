package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
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

// schema/ の直下の 22 本。
const (
	SchemaCandidates   Schema = "candidates"
	SchemaCorrections  Schema = "corrections"
	SchemaDelivered    Schema = "delivered"
	SchemaDigest       Schema = "digest"
	SchemaDigestInput  Schema = "digest-input"
	SchemaDigestSent   Schema = "digest-sent"
	SchemaFindings     Schema = "findings"
	SchemaGate         Schema = "gate"
	SchemaIssueBoard   Schema = "issue-board"
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
	SchemaWaiting      Schema = "waiting"
)

// Schemas は schema/ の直下の全部（テストが、埋め込んだ顔ぶれと突き合わせる）。
var Schemas = []Schema{SchemaCandidates, SchemaCorrections, SchemaDelivered, SchemaDigest, SchemaDigestInput, SchemaDigestSent, SchemaFindings, SchemaGate, SchemaIssueBoard, SchemaIssueKeys,
	SchemaLaunch, SchemaMeasure, SchemaNotification, SchemaOutcome, SchemaPatterns, SchemaProject, SchemaQuestion,
	SchemaReport, SchemaRetro, SchemaRun, SchemaSignal, SchemaWaiting}

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
	if unknown := unknownKeywords(raw, raw, "$", false, nil); len(unknown) > 0 {
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
// uniqueKeys は数を丸めずに読んだ値で比べる（float64 にすると 2^53 を超える別の整数が重なって見える）。
func (c *Checker) Check(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("%s と照らせない（JSON でない）: %v", c.name, err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var exact any
	if err := d.Decode(&exact); err != nil {
		return fmt.Errorf("%s と照らせない（JSON でない）: %v", c.name, err)
	}
	return c.check(v, exact)
}

// CheckValue は JSON を読んだ値 v（map[string]any・[]any・float64 など）がこの schema に合うかを確かめる。
func (c *Checker) CheckValue(v any) error { return c.check(v, v) }

// check は v を既製の検査器で、exact（数を json.Number で持つ同じ値でもよい）を uniqueKeys で照らす。
func (c *Checker) check(v, exact any) error {
	if err := c.resolved.Validate(v); err != nil {
		return &MismatchError{Schema: c.name, Reason: err.Error()}
	}
	if dup := c.uniqueKeys(c.raw, exact, "$", 0); len(dup) > 0 {
		return &MismatchError{Schema: c.name, Reason: strings.Join(dup, "; ")}
	}
	return nil
}

// unknownKeywords は schema の中の部分 schema を全部たどり、Keywords に無いキーワードを「場所: キーワード」で返す。
// uniqueKeys を読まない場所（anyOf・oneOf・not・if・then・else の中。branch が true）の uniqueKeys も返す。
// その場所の $ref が指す先（その先の $ref も）に uniqueKeys があるときも返す。
// 読まないのに置けると、一意を確かめたつもりで確かめない schema になるため。
// uniqueKeys の値が名前（文字列）の配列でないときも返す（読めない値を黙って空と読まない）。root は $ref を解く schema 全体。
func unknownKeywords(root map[string]any, s any, at string, branch bool, out []string) []string {
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
		case k == "uniqueKeys" && !names(m[k]):
			out = append(out, at+": uniqueKeys（値はキーの名前の配列にする）")
		case k == "$ref" && branch:
			if ref, _ := m[k].(string); reachesUniqueKeys(root, pointer(root, ref), map[string]bool{ref: true}) {
				out = append(out, at+": $ref "+ref+" の先の uniqueKeys（anyOf・oneOf・not・if・then・else の中では読まない）")
			}
		}
	}
	for _, k := range []string{"properties", "$defs"} {
		sub, _ := m[k].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(sub)) {
			// $defs は $ref から読むので、置き場が分岐の中でなければ読む（分岐の中の $ref からは上で見る）
			out = unknownKeywords(root, sub[name], at+"."+k+"."+name, branch, out)
		}
	}
	for _, k := range []string{"additionalProperties", "items", "not", "if", "then", "else"} {
		inBranch := branch || k == "not" || k == "if" || k == "then" || k == "else"
		out = unknownKeywords(root, m[k], at+"."+k, inBranch, out)
	}
	for _, k := range []string{"prefixItems", "oneOf", "anyOf", "allOf"} {
		list, _ := m[k].([]any)
		for i, sub := range list {
			inBranch := branch || k == "oneOf" || k == "anyOf"
			out = unknownKeywords(root, sub, fmt.Sprintf("%s.%s[%d]", at, k, i), inBranch, out)
		}
	}
	return out
}

// names は uniqueKeys の値がキーの名前（空でない文字列）の配列か。
func names(v any) bool {
	l, ok := v.([]any)
	if !ok {
		return false
	}
	for _, x := range l {
		if s, _ := x.(string); s == "" {
			return false
		}
	}
	return true
}

// reachesUniqueKeys は部分 schema s のどこか（$ref の先を含む。seen は辿った $ref）に uniqueKeys があるか。
func reachesUniqueKeys(root map[string]any, s any, seen map[string]bool) bool {
	switch s := s.(type) {
	case map[string]any:
		if _, ok := s["uniqueKeys"]; ok {
			return true
		}
		if ref, ok := s["$ref"].(string); ok && !seen[ref] {
			seen[ref] = true
			if reachesUniqueKeys(root, pointer(root, ref), seen) {
				return true
			}
		}
		for _, v := range s {
			if reachesUniqueKeys(root, v, seen) {
				return true
			}
		}
	case []any:
		for _, v := range s {
			if reachesUniqueKeys(root, v, seen) {
				return true
			}
		}
	}
	return false
}

// uniqueKeys は拡張のキーワード uniqueKeys（配列の object の要素で、名前を並べたキーの値が重ならない）を確かめ、
// 重なりを「場所: 値 が重なる」で返す。キーの無い要素は数えない。等しさは JSON の等しさ（1 と true は別）。
// たどるのは、値の形に沿って決まる部分 schema（$ref・allOf・properties・additionalProperties・items・prefixItems）だけ。
// anyOf・oneOf・not・if・then・else の中の uniqueKeys は読まない（Compile が置かせない。そこから $ref で届く先に置いたものも断る）。
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
				j := canonical(val)
				if seen[j] {
					shown, _ := json.Marshal(val)
					bad = append(bad, fmt.Sprintf("%s[%d].%s: %s が重なる", at, i, key, shown))
				}
				seen[j] = true
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

// canonical は JSON の値の等しさの鍵。数は値で比べ（1 と 1.0 は同じ。json.Number は丸めない）、object はキーの順によらない。
// 1 と true、"1" と 1 は別。
func canonical(v any) string {
	switch v := v.(type) {
	case json.Number:
		if r, ok := new(big.Rat).SetString(string(v)); ok {
			return "n" + r.RatString()
		}
		return "n" + string(v)
	case float64:
		if r := new(big.Rat).SetFloat64(v); r != nil {
			return "n" + r.RatString()
		}
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = canonical(x)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		parts := make([]string, 0, len(v))
		for _, k := range slices.Sorted(maps.Keys(v)) {
			kb, _ := json.Marshal(k)
			parts = append(parts, string(kb)+":"+canonical(v[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// pointer は同じ schema の中の $ref（#/$defs/x の形）が指す部分 schema。解けなければ nil（ライブラリが先に断る）。
func (c *Checker) pointer(ref string) map[string]any { return pointer(c.raw, ref) }

func pointer(root map[string]any, ref string) map[string]any {
	if ref == "#" {
		return root
	}
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	var node any = root
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
