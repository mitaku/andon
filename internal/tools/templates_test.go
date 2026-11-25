package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// launchLines はプロファイル（package/templates/*.md）の command と resume の行を "<ファイル>: <行>" で返す。
func launchLines(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "package", "templates", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("template が無い: %v", err)
	}
	var lines []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "command:") || strings.HasPrefix(l, "resume:") {
				lines = append(lines, filepath.Base(f)+": "+l)
			}
		}
	}
	return lines
}

// passesToolsEnv は行が andon tools env を、役のプログラムを起動する env に渡すか（andon otel env の隣。ADR-0063 決めたこと 10）。
// 注釈や別の command の env に書いても効かないので、env の語の並びの中にあることを見る。
func passesToolsEnv(l string) bool {
	return strings.Contains(l, "env $(andon otel env) $(andon tools env) CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1 claude ") ||
		strings.Contains(l, "env $(andon otel env --codex-env) $(andon tools env) codex ")
}

// 受け入れ条件 6: package/templates/ の 17 行が、どれも $(andon tools env) を env に渡している。
func TestTemplatesPassToolsEnv(t *testing.T) {
	lines := launchLines(t)
	if len(lines) != 17 {
		t.Errorf("command・resume の行が %d 行（ADR-0063 は 17 行。足したなら同じ句を渡して数を直す）", len(lines))
	}
	for _, l := range lines {
		if !passesToolsEnv(l) {
			t.Errorf("andon tools env を env に渡していない: %s", l)
		}
	}
	// 検査が当たる場合があること: 渡さない行・env の外に書いた行を作ると落ちる
	for _, l := range lines {
		for _, broken := range []string{
			strings.Replace(l, " $(andon tools env)", "", 1),
			strings.Replace(l, " $(andon tools env)", "", 1) + " # $(andon tools env)",
		} {
			if passesToolsEnv(broken) {
				t.Errorf("渡さない行を通した: %s", broken)
			}
		}
	}
}
