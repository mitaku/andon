// Package ledgertest は、Go のテストが手で書く台帳を、台帳の CLI が書く形（schema/ledger/）に通す補助。
// 本物の kanban を呼ばないテストが手で書いた log.jsonl の行や kanban ready --json の返事が、kanban の書く形からずれたら、
// そのテストを落とす（ADR-0034 決めたこと 7）。cmd/kanban のテストも、本物の kanban の出力を同じ照合に通す。
// テストだけが使う（配る binary には入らない）。
package ledgertest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// schema/ledger/ の名前。
const (
	LogSchema   = "log.v3.schema.json"   // log.jsonl の1行
	ReadySchema = "ready.v3.schema.json" // kanban ready --json の出力
)

var (
	mu       sync.Mutex
	resolved = map[string]*jsonschema.Resolved{}
)

// Dir は repo の schema/ledger/（このファイルの置き場から辿る。テストの cwd に依らない）。
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schema", "ledger")
}

func compile(name string) (*jsonschema.Resolved, error) {
	mu.Lock()
	defer mu.Unlock()
	if r := resolved[name]; r != nil {
		return r, nil
	}
	b, err := os.ReadFile(filepath.Join(Dir(), name))
	if err != nil {
		return nil, err
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("schema/ledger/%s が読めない: %v", name, err)
	}
	r, err := s.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("schema/ledger/%s が読めない: %v", name, err)
	}
	resolved[name] = r
	return r, nil
}

// Check は JSON の b が schema/ledger/<name> に合うかを確かめる。合わなければ、どこがどう合わないかの誤り。
func Check(name string, b []byte) error {
	r, err := compile(name)
	if err != nil {
		return err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("JSON でない: %v", err)
	}
	if err := r.Validate(v); err != nil {
		return fmt.Errorf("schema/ledger/%s に合わない: %v", name, err)
	}
	return nil
}

// CheckLog は log.jsonl の中身を1行ずつ schema/ledger/log.v3.schema.json に通す。合わない行があれば、その行の番号つきの誤り。
func CheckLog(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var bad []string
	for i, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		if err := Check(LogSchema, line); err != nil {
			bad = append(bad, fmt.Sprintf("log.jsonl:%d: %v: %s", i+1, err, line))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s", strings.Join(bad, "\n"))
	}
	return nil
}

// Log は log.jsonl の行（1行1件の JSON）をつないで、ファイルの中身（末尾に改行）にする。
// どの行も台帳の CLI が書く形（schema/ledger/log.v3.schema.json）に合わなければ、テストを落とす。
// 読めない行の扱いを確かめるテストは、この補助を通さずに書く。
func Log(t testing.TB, lines ...string) string {
	t.Helper()
	if len(lines) == 0 {
		return ""
	}
	data := strings.Join(lines, "\n") + "\n"
	if err := CheckLog([]byte(data)); err != nil {
		t.Fatalf("手で書いた log.jsonl の行が、台帳の CLI の書く形でない:\n%v", err)
	}
	return data
}

// Ready は kanban ready --json の返事（課題の並び）を JSON にする。台帳の CLI が書く形
// （schema/ledger/ready.v3.schema.json）に合わなければ、テストを落とす。
func Ready(t testing.TB, issues ...any) string {
	t.Helper()
	if issues == nil {
		issues = []any{}
	}
	b, err := json.Marshal(issues)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(ReadySchema, b); err != nil {
		t.Fatalf("手で書いた kanban ready --json の返事が、台帳の CLI の書く形でない: %v\n%s", err, b)
	}
	return string(b)
}
