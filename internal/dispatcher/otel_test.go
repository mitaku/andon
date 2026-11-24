package dispatcher

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/otel"
)

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
	if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil {
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
	c.OtelPrune = func(project string, _ time.Time, _ func() otel.Limits) (bool, int, error) {
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

func putOwnerProject(t *testing.T, ws, body, author string) {
	t.Helper()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", ws}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	rel := filepath.Join("projects", "sample-site", "project.json")
	if err := os.WriteFile(filepath.Join(ws, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "andon.toml"), []byte("owner = \"owner@example.com\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("init", "-q")
	git("add", rel)
	git("-c", "user.name=o", "-c", "user.email="+author, "commit", "-q", "-m", "project")
}

func putOld(t *testing.T, state, id string, days int) string {
	t.Helper()
	dir := filepath.Join(state, "andon", "otel", "sample-site", "issues", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "traces.jsonl")
	if err := os.WriteFile(f, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	if err := os.Chtimes(f, at, at); err != nil {
		t.Fatal(err)
	}
	return f
}

// 片付けは寿命を project.json の otel.days から読む（ADR-0060 決めたこと 11）。7 日なら 8 日前のファイルを消し、6 日前は残す。
func TestPassOtelPruneReadsProjectLimits(t *testing.T) {
	c, logb, state := otelCell(t)
	putOwnerProject(t, c.WS, `{"version":1,"project":{"title":"Sample","prefix":"S"},"repos":[],"otel":{"enabled":false,"days":7,"max_gb":1}}`, "owner@example.com")
	old, fresh := putOld(t, state, "S-0001", 8), putOld(t, state, "S-0002", 6)
	c.Pass("")
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("otel.days が 7 なのに 8 日前のファイルを消さない: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("6 日前のファイルを消した: %v", err)
	}
	if !strings.Contains(logb.String(), "寿命 7 日・上限 1 GB") || strings.Contains(logb.String(), "otel が読めない") {
		t.Errorf("project.json の値で片付けたことがログに出ない:\n%s", logb.String())
	}
}

func TestPassOtelPruneUnreadableProjectUsesDefault(t *testing.T) {
	c, logb, state := otelCell(t)
	putOwnerProject(t, c.WS, `{"version":1,"project":{"title":"Sample","prefix":"S"},"repos":[],"otel":{"days":7}}`, "someone@example.com")
	old, kept := putOld(t, state, "S-0001", 31), putOld(t, state, "S-0002", 8)
	c.Pass("")
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("31 日前のファイルを消さない: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("読めない project.json の days を使った（8 日前を消した）: %v", err)
	}
	if n := strings.Count(logb.String(), "project.json の otel が読めない"); n != 1 {
		t.Errorf("読めないことのログが %d 行:\n%s", n, logb.String())
	}
}
