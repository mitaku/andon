package schema

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	rc := Main(args, strings.NewReader(stdin), &out, &errOut)
	return rc, out.String(), errOut.String()
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCheck は andon schema check の終了コード（0 合う・1 合わない・2 検査できない）と、理由の出し先を見る。
func TestCheck(t *testing.T) {
	dir := t.TempDir()
	good := write(t, dir, "good.json", `{"commit": "`+strings.Repeat("a", 40)+`"}`)
	bad := write(t, dir, "bad.json", `{"commit": "x"}`)
	file := filepath.Join("..", "..", "schema", "report.schema.json")
	unknown := write(t, dir, "f.schema.json", `{"type": "string", "format": "date"}`)
	for _, c := range []struct {
		name    string
		stdin   string
		args    []string
		rc      int
		out     string // 標準出力に含む
		errText string // 標準エラーに含む
	}{
		{"埋め込んだ名前で合う", "", []string{"check", "report", good}, 0, "", ""},
		{"埋め込んだ名前で合わない", "", []string{"check", "report", bad}, 1, "schema/report.schema.json に合わない", ""},
		{"置き場で合う", "", []string{"check", file, good}, 0, "", ""},
		{"置き場で合わない", "", []string{"check", file, bad}, 1, "に合わない", ""},
		{"標準入力", `{"commit": "x"}`, []string{"check", "report", "-"}, 1, "に合わない", ""},
		{"知らない名前", "", []string{"check", "reports", good}, 2, "", "埋め込んだ schema に reports は無い"},
		{"知らないキーワード", "", []string{"check", unknown, good}, 2, "", "検査器の知らないキーワード"},
		{"JSON でない", "{", []string{"check", "report", "-"}, 2, "", "JSON でない"},
		{"無い JSON", "", []string{"check", "report", filepath.Join(dir, "none.json")}, 2, "", "JSON が読めない"},
		{"無い schema", "", []string{"check", filepath.Join(dir, "none.schema.json"), good}, 2, "", "schema が読めない"},
		{"引数が足りない", "", []string{"check", "report"}, 2, "", "usage: andon schema check"},
		{"知らない口", "", []string{"validate", "report", good}, 2, "", "usage: andon schema check"},
	} {
		rc, out, errOut := run(t, c.stdin, c.args...)
		if rc != c.rc || !strings.Contains(out, c.out) || !strings.Contains(errOut, c.errText) {
			t.Errorf("%s: rc=%d（want %d）\nstdout: %s\nstderr: %s", c.name, rc, c.rc, out, errOut)
		}
		if rc == 0 && out+errOut != "" {
			t.Errorf("%s: 合うのに何か出した: %s%s", c.name, out, errOut)
		}
	}
}

// TestBatch は andon schema batch が1行1件を check と同じ終了コードで、同じ順に返すかを見る。
func TestBatch(t *testing.T) {
	dir := t.TempDir()
	good := write(t, dir, "good.json", `{"commit": "`+strings.Repeat("a", 40)+`"}`)
	bad := write(t, dir, "bad.json", `{"commit": "x"}`)
	multi := write(t, dir, "multi.json", `{"commit": "x", "pr": 1}`)
	uniq := write(t, dir, "u.schema.json", `{"type": "array", "uniqueKeys": ["id"]}`)
	dup := write(t, dir, "dup.json", `[{"id": "c1"}, {"id": "c1"}]`)
	cases := strings.Join([]string{
		"report\t" + good,
		"report\t" + bad,
		uniq + "\t" + dup,
		"report\t" + multi,
		"",
		"report",
		"report\t-",
		"nosuch\t" + good,
	}, "\n") + "\n"
	rc, out, errOut := run(t, "", "batch", write(t, dir, "cases.tsv", cases))
	if rc != 0 || errOut != "" {
		t.Fatalf("batch: rc=%d stderr=%s", rc, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	want := []string{"0", "1", "1", "1", "2", "2", "2"}
	if len(lines) != len(want) {
		t.Fatalf("空の行を除く %d 件に %d 行:\n%s", len(want), len(lines), out)
	}
	for i, l := range lines {
		rc, why, ok := strings.Cut(l, "\t")
		if !ok || rc != want[i] || strings.Contains(why, "\t") {
			t.Errorf("%d 行目: %q（want 終了コード %s）", i+1, l, want[i])
		}
	}
	if !strings.Contains(lines[2], `"c1" が重なる`) {
		t.Errorf("uniqueKeys の重なりの理由が無い: %s", lines[2])
	}
	if rc, _, errOut := run(t, "", "batch", filepath.Join(dir, "none.tsv")); rc != 2 || !strings.Contains(errOut, "表が読めない") {
		t.Errorf("無い表: rc=%d stderr=%s", rc, errOut)
	}
}
