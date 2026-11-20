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
// （ADR-0034 受け入れ条件 6。ECMA-262・Python と意味が割れる書き方は schema_check.py の lint が止める）。
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
