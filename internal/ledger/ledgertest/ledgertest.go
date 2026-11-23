// Package ledgertest は、Go のテストが手で書く kanban を、kanban の CLI が書く形（schema/ledger/）に通す補助。
// 本物の kanban の CLI を呼ばないテストが手で書いた log.jsonl の行や kanban ready --json の返事が、kanban の書く形からずれたら、
// そのテストを落とす（ADR-0034 決めたこと 7）。cmd/kanban のテストも、本物の kanban の CLI の出力を同じ照合に通す。
// テストだけが使う（配る binary には入らない）。
package ledgertest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"andon/internal/contract"
)

// schema/ledger/ の名前。
const (
	LogSchema   = "log.v3.schema.json"   // log.jsonl の1行
	ReadySchema = "ready.v3.schema.json" // kanban ready --json の出力
)

// Dir は repo の schema/ledger/（このファイルの置き場から辿る。テストの cwd に依らない）。
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schema", "ledger")
}

// Check は JSON の b が schema/ledger/<name> に合うかを確かめる。合わなければ、どこがどう合わないかの誤り。
// 検査器は andon が約束の JSON を書く前に照らすのと同じもの（internal/contract）。
func Check(name string, b []byte) error {
	c, err := contract.CompileFile(filepath.Join(Dir(), name))
	if err != nil {
		return err
	}
	var mismatch *contract.MismatchError
	if err := c.Check(b); errors.As(err, &mismatch) {
		return fmt.Errorf("schema/ledger/%s に合わない: %s", name, mismatch.Reason)
	} else if err != nil {
		return err
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
// どの行も kanban の CLI が書く形（schema/ledger/log.v3.schema.json）に合わなければ、テストを落とす。
// 読めない行の扱いを確かめるテストは、この補助を通さずに書く。
func Log(t testing.TB, lines ...string) string {
	t.Helper()
	if len(lines) == 0 {
		return ""
	}
	data := strings.Join(lines, "\n") + "\n"
	if err := CheckLog([]byte(data)); err != nil {
		t.Fatalf("手で書いた log.jsonl の行が、kanban の CLI の書く形でない:\n%v", err)
	}
	return data
}

// Ready は kanban ready --json の返事（課題の並び）を JSON にする。kanban の CLI が書く形
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
		t.Fatalf("手で書いた kanban ready --json の返事が、kanban の CLI の書く形でない: %v\n%s", err, b)
	}
	return string(b)
}

// Kanban は repo の cmd/kanban を一時のフォルダに build し、そのフォルダを返す。PATH の先頭に置くと、コードが打つ
// kanban の CLI が本物になる（答えの印のように、kanban が書いた結果そのものを見たいテストが使う）。go が無ければ飛ばす。
func Kanban(t testing.TB) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go が無い")
	}
	// go test は build した kanban の元を入力と見ないので、元を読んで入力に数えさせる（元を変えたのに前の結果が
	// キャッシュから返らないように）。
	repo := filepath.Join(Dir(), "..", "..")
	srcs, _ := filepath.Glob(filepath.Join(repo, "cmd", "kanban", "*.go"))
	for _, f := range srcs {
		if _, err := os.ReadFile(f); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	name := "kanban"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	c := exec.Command(gobin, "build", "-o", filepath.Join(dir, name), "./cmd/kanban")
	c.Dir = repo
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("kanban の CLI を build できない: %v\n%s", err, out)
	}
	return dir
}
