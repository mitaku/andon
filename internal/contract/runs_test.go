package contract

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const otherSHA = "0000000000000000000000000000000000000001"

// putRun は library/runs/<n>.json と <n>.log を置く（andon issue record が書く形）。commit が空なら repo の無い課題の記録。
func putRun(t *testing.T, library string, n int, expect string, code int, commit string, dirty bool) {
	t.Helper()
	r := Run{ID: n, Command: []string{"go", "test", "./..."}, Expect: expect, ExitCode: code, Commit: commit,
		StartedAt: "2026-11-20T09:00:00Z", FinishedAt: "2026-11-20T09:00:10Z", Output: RunOutput(n), OutputBytes: 3}
	if commit != "" {
		r.Dirty = &dirty
	}
	if err := os.MkdirAll(filepath.Join(library, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(filepath.Join(library, "runs", strconv.Itoa(n)+".json"), SchemaRun, &r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, r.Output), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func row(id string, run int) Acceptance {
	if run == 0 {
		return Acceptance{ID: id, Result: "pass", Evidence: "CI の go"}
	}
	return Acceptance{ID: id, Result: "pass", Evidence: "go test ./...", Run: &run}
}

// ADR-0035 受け入れ条件 3: 記録を指す行だけを見る。無い・終了コードが期待と違う・pass を期待する記録が dirty の3通りで断る理由を返す
func TestCheckRuns(t *testing.T) {
	cases := []struct {
		name  string
		put   func(t *testing.T, lib string)
		rows  []Acceptance
		want  string // 空なら理由が無い
		stale []StaleRun
	}{
		{"記録を指さない行だけなら見ない", func(*testing.T, string) {}, []Acceptance{row("A1", 0), {ID: "A2", Result: "manual", Evidence: "人が見る"}}, "", nil},
		{"pass を期待して通った記録", func(t *testing.T, l string) { putRun(t, l, 1, "pass", 0, sha, false) }, []Acceptance{row("A1", 1)}, "", nil},
		{"fail を期待して落ちた記録は変更のある木でよい", func(t *testing.T, l string) { putRun(t, l, 1, "fail", 1, sha, true) }, []Acceptance{row("A1", 1)}, "", nil},
		{"repo の無い課題の記録", func(t *testing.T, l string) { putRun(t, l, 1, "pass", 0, "", false) }, []Acceptance{row("A1", 1)}, "", nil},
		{"記録が無い", func(*testing.T, string) {}, []Acceptance{row("A1", 4)}, "受け入れ条件 A1 の記録 runs/4.json が無い", nil},
		{"出力が無い", func(t *testing.T, l string) {
			putRun(t, l, 1, "pass", 0, sha, false)
			os.Remove(filepath.Join(l, "runs", "1.log"))
		}, []Acceptance{row("A1", 1)}, "受け入れ条件 A1 の記録 runs/1.json の出力 runs/1.log が無い", nil},
		{"記録の形が違う", func(t *testing.T, l string) {
			os.MkdirAll(filepath.Join(l, "runs"), 0o755)
			os.WriteFile(filepath.Join(l, "runs", "1.json"), []byte(`{"id": 1}`), 0o644)
		}, []Acceptance{row("A1", 1)}, "受け入れ条件 A1 の記録 runs/1.json が読めない", nil},
		{"pass を期待して落ちた", func(t *testing.T, l string) { putRun(t, l, 2, "pass", 1, sha, false) }, []Acceptance{row("A3", 2)},
			"受け入れ条件 A3 の記録 runs/2.json の終了コード 1 が期待（pass）と違う", nil},
		{"fail を期待して通った", func(t *testing.T, l string) { putRun(t, l, 2, "fail", 0, sha, true) }, []Acceptance{row("A3", 2)},
			"受け入れ条件 A3 の記録 runs/2.json の終了コード 0 が期待（fail）と違う", nil},
		{"pass を期待する記録が変更のある木", func(t *testing.T, l string) { putRun(t, l, 1, "pass", 0, sha, true) }, []Acceptance{row("A1", 1)},
			"受け入れ条件 A1 の記録 runs/1.json は commit していない変更のある木で撮った", nil},
		{"head と違う commit の記録は断らず古いと返す", func(t *testing.T, l string) {
			putRun(t, l, 1, "pass", 0, otherSHA, false)
			putRun(t, l, 2, "pass", 0, sha, false)
		}, []Acceptance{row("A1", 1), row("A2", 2), row("A3", 0)}, "", []StaleRun{{ID: "A1", Run: 1, Commit: otherSHA}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lib := t.TempDir()
			c.put(t, lib)
			problems, stale := CheckRuns(lib, c.rows, sha)
			switch {
			case c.want == "" && len(problems) > 0:
				t.Fatalf("理由が出た: %v", problems)
			case c.want != "" && (len(problems) != 1 || !strings.HasPrefix(problems[0], c.want)):
				t.Fatalf("want %q, got %v", c.want, problems)
			case !reflect.DeepEqual(stale, c.stale):
				t.Fatalf("stale: want %v, got %v", c.stale, stale)
			}
		})
	}
	// head が空（andon issue ready）なら古さは見ない
	lib := t.TempDir()
	putRun(t, lib, 1, "pass", 0, otherSHA, false)
	if p, s := CheckRuns(lib, []Acceptance{row("A1", 1)}, ""); len(p) > 0 || len(s) > 0 {
		t.Fatalf("head が空で理由か古さが出た: %v %v", p, s)
	}
}
