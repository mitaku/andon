package contract

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"andon"
)

// TestSchemasEmbedded は、埋め込んだ schema が repo の schema/ の直下と同じ顔ぶれ・同じ中身で、全部を検査器が読めるかを見る。
func TestSchemasEmbedded(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "schema", "*.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk, listed []string
	for _, f := range files {
		onDisk = append(onDisk, strings.TrimSuffix(filepath.Base(f), ".schema.json"))
	}
	for _, s := range Schemas {
		listed = append(listed, string(s))
	}
	slices.Sort(listed)
	if !slices.Equal(onDisk, listed) {
		t.Fatalf("schema/ の直下 %v と Schemas %v が違う", onDisk, listed)
	}
	for _, s := range Schemas {
		disk, _ := os.ReadFile(filepath.Join("..", "..", s.File()))
		emb, err := fs.ReadFile(andon.Schemas, s.File())
		if err != nil || string(disk) != string(emb) {
			t.Fatalf("%s の埋め込みが repo と違う: %v", s.File(), err)
		}
		if _, err := s.compile(); err != nil {
			t.Fatalf("検査器が読めない: %v", err)
		}
	}
}

// TestSchemaPatternsCompile は、schema/ の直下の全部の pattern が Go の正規表現（選んだ検査器が使う RE2）で読めるかを見る
// （ADR-0034 受け入れ条件 6。ECMA-262 と意味が割れる書き方は schema_check.py の lint が止める）。
func TestSchemaPatternsCompile(t *testing.T) {
	n := 0
	for _, s := range Schemas {
		b, err := fs.ReadFile(andon.Schemas, s.File())
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		var walk func(any, string)
		walk = func(x any, at string) {
			switch x := x.(type) {
			case map[string]any:
				for k, y := range x {
					if p, ok := y.(string); ok && k == "pattern" {
						n++
						if _, err := regexp.Compile(p); err != nil {
							t.Errorf("%s %s.pattern が Go の正規表現で読めない: %v", s.File(), at, err)
						}
						continue
					}
					walk(y, at+"."+k)
				}
			case []any:
				for _, y := range x {
					walk(y, at+"[]")
				}
			}
		}
		walk(v, "$")
	}
	if n == 0 {
		t.Fatal("pattern が1つも見つからない（歩き方の誤り）")
	}
	t.Logf("schema/ の直下 %d 本の pattern %d 個が、Go の正規表現で読めた", len(Schemas), n)
}

// TestRelPathMatchesSchema は、Go の RelPath と schema の相対パスの pattern（findings の file）が同じものを通すかを見る。
func TestRelPathMatchesSchema(t *testing.T) {
	b, _ := fs.ReadFile(andon.Schemas, SchemaFindings.File())
	var v struct {
		Defs struct {
			Finding struct {
				Properties struct {
					File struct {
						Pattern string `json:"pattern"`
					} `json:"file"`
				} `json:"properties"`
			} `json:"finding"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(b, &v); err != nil || v.Defs.Finding.Properties.File.Pattern == "" {
		t.Fatalf("findings の file の pattern が読めない: %v", err)
	}
	re := regexp.MustCompile(v.Defs.Finding.Properties.File.Pattern)
	alphabet := []string{"a", ".", "/", "\x01", "é"}
	var cases []string
	var gen func(string, int)
	gen = func(s string, n int) {
		cases = append(cases, s)
		if n == 0 {
			return
		}
		for _, c := range alphabet {
			gen(s+c, n-1)
		}
	}
	gen("", 5)
	for _, s := range cases {
		if re.MatchString(s) != RelPath(s) {
			t.Fatalf("%q: schema の pattern は %v、RelPath は %v", s, re.MatchString(s), RelPath(s))
		}
	}
	t.Logf("%d 個の文字列で、schema の pattern と RelPath が同じ答え", len(cases))
}

// TestWriteChecksSchemaBeforeWriting は、約束の JSON を書く口が schema に合わない値を書かずに誤りを返すかを見る
// （ADR-0034 受け入れ条件 6）。前の中身は残り、合う値は書ける。
func TestWriteChecksSchemaBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	good := Report{Commit: strings.Repeat("a", 40), PR: "https://github.com/example/app/pull/7"}
	if err := Write(path, SchemaReport, good); err != nil {
		t.Fatalf("schema に合う report.json を書けない: %v", err)
	}
	before, _ := os.ReadFile(path)
	bad := []struct {
		name string
		s    Schema
		v    any
	}{
		{"pr が PR の URL でない", SchemaReport, Report{Commit: strings.Repeat("a", 40), PR: "https://example.com/x"}},
		{"知らないキー", SchemaReport, map[string]any{"commit": strings.Repeat("a", 40), "unknown": 1}},
		{"next の宛先が無い値", SchemaReport, Report{Next: []Next{{To: "someone", Text: "x"}}}},
		{"plan の id が空", SchemaReport, Report{Plan: []PlanItem{{ID: "", Text: "x", Done: new(bool)}}}},
	}
	for _, c := range bad {
		err := Write(path, c.s, c.v)
		if err == nil || !strings.Contains(err.Error(), "schema/report.schema.json に合わない") {
			t.Fatalf("%s: schema に合わないのに書いた・誤りの文が違う: %v", c.name, err)
		}
		if after, _ := os.ReadFile(path); string(after) != string(before) {
			t.Fatalf("%s: 照らして落ちたのに前の中身を書き換えた", c.name)
		}
		if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s: 一時ファイルを残した", c.name)
		}
	}
	// 無い置き場には何も作らない
	fresh := filepath.Join(dir, "candidates.json")
	if err := Write(fresh, SchemaCandidates, map[string]any{"candidates": "x"}); err == nil {
		t.Fatal("schema に合わない candidates.json を書いた")
	}
	if _, err := os.Stat(fresh); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("照らして落ちたのにファイルを作った")
	}
	// 1行の口と、中身を返すだけの口も同じく照らす
	if _, err := EncodeLine(SchemaNotification, map[string]any{"kind": "report"}); err == nil {
		t.Fatal("schema に合わない通知を1行にした")
	}
	if _, err := Encode(SchemaQuestion, Question{Kind: "line", Question: "q"}); err == nil {
		t.Fatal("選択肢の無い問いを JSON にした")
	}
	line, err := EncodeLine(SchemaNotification, map[string]any{"at": "2026-11-20T00:00:00Z", "project": "demo", "issue": "A-1",
		"kind": "report", "detail": map[string]any{"report": "/w/kanban/issues/A-1/report.md"}})
	if err != nil || strings.Contains(string(line), "\n") {
		t.Fatalf("schema に合う通知を1行にできない: %q %v", line, err)
	}
}

// TestCompileRefusesWhatItDoesNotRead は、検査器の知らないキーワードと、読まない場所（分岐の中）の uniqueKeys がある schema を、
// 照らす前に断るかを見る（検査が効いていないのに通るのを防ぐ）。断るのは合わないこと（*MismatchError）でなく、検査できないこと。
func TestCompileRefusesWhatItDoesNotRead(t *testing.T) {
	for _, c := range []struct {
		name, schema string
	}{
		{"知らないキーワード format", `{"type": "string", "format": "date"}`},
		{"入れ子の知らないキーワード", `{"type": "object", "properties": {"a": {"type": "integer", "maximum": 3}}}`},
		{"$defs の中の知らないキーワード", `{"$defs": {"x": {"uniqueItems": true}}}`},
		{"anyOf の中の uniqueKeys", `{"anyOf": [{"type": "array", "uniqueKeys": ["id"]}]}`},
		{"if の中の uniqueKeys", `{"if": {"type": "array"}, "then": {"uniqueKeys": ["id"]}}`},
		{"not の下の properties の uniqueKeys", `{"not": {"properties": {"a": {"uniqueKeys": ["id"]}}}}`},
	} {
		if _, err := Compile("x", []byte(c.schema)); err == nil || !strings.Contains(err.Error(), "検査器の知らないキーワード") {
			t.Errorf("%s: 断らない・誤りの文が違う: %v", c.name, err)
		}
	}
	for _, ok := range []string{
		`{"type": "array", "uniqueKeys": ["id"], "items": {"$ref": "#/$defs/x"}, "$defs": {"x": {"type": "object"}}}`,
		`{"allOf": [{"properties": {"a": {"uniqueKeys": ["id"]}}}]}`,
		`{"$defs": {"x": {"type": "array", "uniqueKeys": ["id"]}}, "properties": {"a": {"$ref": "#/$defs/x"}}}`,
	} {
		if _, err := Compile("x", []byte(ok)); err != nil {
			t.Errorf("読める schema を断った: %s: %v", ok, err)
		}
	}
}

// TestCheckerUniqueKeys は、拡張のキーワード uniqueKeys を、値の形に沿ってたどった先（properties・items・prefixItems・
// additionalProperties・$ref・allOf）で確かめるかを見る。
func TestCheckerUniqueKeys(t *testing.T) {
	c, err := Compile("u", []byte(`{
	  "$defs": {"list": {"type": "array", "uniqueKeys": ["id"], "items": {"$ref": "#/$defs/item"}},
	            "item": {"properties": {"sub": {"$ref": "#/$defs/list"}}}},
	  "type": "object",
	  "properties": {"a": {"$ref": "#/$defs/list"}, "t": {"type": "array", "prefixItems": [{"$ref": "#/$defs/list"}]}},
	  "additionalProperties": {"allOf": [{"$ref": "#/$defs/list"}]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		json string
		ok   bool
	}{
		{`{"a": [{"id": "c1"}, {"id": "c2"}, {"x": 1}, 3]}`, true},
		{`{"a": [{"id": 1}, {"id": true}, {"id": "1"}]}`, true},
		{`{"a": [{"id": "c1"}, {"id": "c1"}]}`, false},
		{`{"a": [{"id": {"k": 1, "j": 2}}, {"id": {"j": 2, "k": 1}}]}`, false},
		{`{"a": [{"id": "c1", "sub": [{"id": "d"}, {"id": "d"}]}]}`, false},
		{`{"t": [[{"id": 1}, {"id": 1.0}]]}`, false},
		{`{"z": [{"id": "c1"}, {"id": "c1"}]}`, false},
		{`{"a": [{"id": "c1"}], "t": [[{"id": "c1"}]], "z": [{"id": "c1"}]}`, true},
	} {
		err := c.Check([]byte(x.json))
		var mismatch *MismatchError
		if x.ok != (err == nil) || (err != nil && !errors.As(err, &mismatch)) {
			t.Errorf("%s: ok=%v, got %v", x.json, x.ok, err)
		}
	}
	if err := c.Check([]byte(`{`)); err == nil || errors.As(err, new(*MismatchError)) {
		t.Errorf("JSON でないものを、合わないとして返した（検査できないとして返す）: %v", err)
	}
}

// TestCompileFile は、置き場の schema を読んで照らし、読めない置き場は検査できない誤りにするかを見る。
func TestCompileFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.schema.json")
	if err := os.WriteFile(p, []byte(`{"type": "integer", "minimum": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := CompileFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Check([]byte("1.0")); err != nil {
		t.Errorf("integer が 1.0 を受けない（2020-12）: %v", err)
	}
	if err := c.Check([]byte("0")); err == nil {
		t.Error("minimum の外を通した")
	}
	if _, err := CompileFile(filepath.Join(dir, "none.schema.json")); err == nil {
		t.Error("無い置き場の schema を読めたことにした")
	}
}

// TestKeywordsMatchLint は、検査器が読むキーワード（Keywords）と、schema の決まりの lint（package/lib/andon/schema_check.py の
// ANNOTATIONS と KEYWORDS）が同じ顔ぶれかを見る。ずれると、lint が通した schema を検査器が断るか、その逆になる。
func TestKeywordsMatchLint(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "package", "lib", "andon", "schema_check.py"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "ANNOTATIONS = {")
	end := strings.Index(src[max(start, 0):], "\n}\n")
	if start < 0 || end < 0 {
		t.Fatal("schema_check.py に ANNOTATIONS と KEYWORDS の並びが無い")
	}
	var lint []string
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(src[start:start+end], -1) {
		lint = append(lint, m[1])
	}
	slices.Sort(lint)
	want := slices.Sorted(slices.Values(Keywords))
	if !slices.Equal(lint, want) {
		t.Fatalf("schema_check.py の lint %v と検査器の Keywords %v が違う", lint, want)
	}
}
