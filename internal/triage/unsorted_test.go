package triage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 仕分けていない項目の数は、collect が出す項目のうち、次回に回した項目を、伏せる 20 時間をすぎたものも含めて除いた数
// （ADR-0047 決めたこと 7）。閉じた項目は collect も出さない。上限（50）では切らない。
func TestUnsortedSkipsKeptItems(t *testing.T) {
	now := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	t.Setenv("PATH", filepath.Join(dir, "nobin")) // hc を見つけない
	write := func(path, body string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	var list []string
	for i := 1; i <= 60; i++ {
		list = append(list, fmt.Sprintf(`{"id": "T-%d", "title": "課題 %d", "status": "todo", "created": "2026-11-01T00:00:00Z"}`, i, i))
	}
	listPath := filepath.Join(dir, "list.json")
	write(listPath, "["+strings.Join(list, ",")+"]\n", 0o644)
	cli := filepath.Join(dir, "cli")
	write(cli, "#!/bin/sh\nwhile IFS= read -r ln; do printf '%s\\n' \"$ln\"; done < '"+listPath+"'\n", 0o755)
	item := func(i int, decision string) string {
		return fmt.Sprintf(`{"key": %q, "decision": %q}`, itemKey("demo", fmt.Sprintf("T-%d", i), "issue", fmt.Sprintf("課題 %d", i)), decision)
	}
	// 古い記録（30 時間前）で T-1〜T-4 を次回に回し、T-5〜T-7 を閉じた。新しい記録（1 時間前）で T-8〜T-10 を次回に回した。
	var old, recent []string
	for i := 1; i <= 4; i++ {
		old = append(old, item(i, "keep"))
	}
	for i := 5; i <= 7; i++ {
		old = append(old, item(i, "close"))
	}
	for i := 8; i <= 10; i++ {
		recent = append(recent, item(i, "keep"))
	}
	rec := func(name string, at time.Duration, items []string) {
		write(filepath.Join(dir, "kanban", "library", "triage", name), fmt.Sprintf(`{"collected": %q, "items": [%s]}`,
			now.Add(-at).Format(time.RFC3339), strings.Join(items, ",")), 0o644)
	}
	rec("2026-11-21-T-900.json", 30*time.Hour, old)
	rec("2026-11-22-T-901.json", time.Hour, recent)
	l := &ledger{cli: cli, root: dir, slug: "demo", dir: filepath.Join(dir, "kanban"), now: now}
	n, err := l.unsorted()
	if err != nil {
		t.Fatal(err)
	}
	if n != 50 {
		t.Errorf("仕分けていない項目 %d 件（60 − 次回に回した 7 − 閉じた 3 = 50 のはず）", n)
	}
}
