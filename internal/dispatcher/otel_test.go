package dispatcher

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// otelCell は、Chief of Staff が生きている kanban を 1 つ持つ偽のセルを作り、dispatcher を返す。
func otelCell(t *testing.T) (*Dispatcher, *bytes.Buffer, string) {
	t.Helper()
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"kanban": tickKanban, "hc": tickHc} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil { // Chief of Staff の端末は生きている
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_KANBAN", d)
	state := filepath.Join(d, "state")
	t.Setenv("XDG_STATE_HOME", state)
	ws := filepath.Join(d, "ws")
	t.Setenv("ANDON_WORKSPACE", ws)
	led := filepath.Join(ws, "projects", "sample-site", "kanban")
	if err := os.MkdirAll(led, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logb bytes.Buffer
	return &Dispatcher{WS: ws, Log: &logb}, &logb, state
}

// 片付けが失敗を返しても、1 巡は最後（控えの保存）まで進み、失敗はログに 1 行だけ出る（ADR-0060 受け入れ条件 6）。
func TestPassOtelPruneFailureKeepsGoing(t *testing.T) {
	c, logb, _ := otelCell(t)
	calls := 0
	c.OtelPrune = func(project string, _ time.Time) (bool, int, error) {
		calls++
		if project != "sample-site" {
			t.Errorf("片付けに渡したプロジェクトが %q", project)
		}
		return true, 0, errors.New("置き場が読めない")
	}
	c.Pass("")
	c.Pass("")
	if calls != 2 {
		t.Fatalf("片付けを 1 巡に 1 回呼ばない: %d 回", calls)
	}
	if _, first, err := LoadState("sample-site"); err != nil || first {
		t.Fatalf("片付けの失敗のあと控えを保存しない（巡が最後まで進まない）: first=%v err=%v", first, err)
	}
	if n := strings.Count(logb.String(), "ハーネスの生の記録を片付けられない"); n != 1 {
		t.Errorf("同じ失敗のログが %d 行（1 行のはず）:\n%s", n, logb.String())
	}
}

// 本物の片付け（otel.PruneDue）を巡が走らせ、30 日を過ぎたファイルを消し、同じ日の 2 度目の巡では走らせない。
func TestPassOtelPruneRemovesOld(t *testing.T) {
	c, logb, state := otelCell(t)
	dir := filepath.Join(state, "andon", "otel", "sample-site", "issues", "S-0001")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "traces.jsonl")
	if err := os.WriteFile(old, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(old, at, at); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("31 日前のファイルを巡が消さない: %v", err)
	}
	if !strings.Contains(logb.String(), "ハーネスの生の記録を 1 本消した") {
		t.Errorf("消したことをログに出さない:\n%s", logb.String())
	}
	// 同じ日の 2 度目の巡では走らせない（また 31 日前のファイルを置いても消えない）
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(old, at, at); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	if _, err := os.Stat(old); err != nil {
		t.Errorf("同じ日の 2 度目の巡で片付けを走らせた: %v", err)
	}
}
