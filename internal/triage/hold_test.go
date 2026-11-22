package triage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// 次回に回した項目は、それを回したいちばん新しい記録の集めた時刻から 20 時間たつまで collect に出ない
// （ADR-0047 決めたこと 19、受け入れ条件 12）。
func TestCollectHoldsKeptItemsFor20Hours(t *testing.T) {
	now := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }
	key := itemKey("demo", "T-1", "issue", "課題 1")
	// kept は項目を 1 件持つ記録。collected は JSON の値のまま渡す。
	kept := func(collected any, decision string) map[string]any {
		return map[string]any{"collected": collected, "items": []any{map[string]any{"key": key, "decision": decision}}}
	}
	other := map[string]any{"collected": ago(time.Hour), "items": []any{}}

	for _, c := range []struct {
		name    string
		records []map[string]any // ファイルの名前の順
		shown   bool
	}{
		{"記録が無い", nil, true},
		{"集めた直後", []map[string]any{kept(ago(0), "keep")}, false},
		{"20 時間に 1 秒足りない", []map[string]any{kept(ago(20*time.Hour-time.Second), "keep")}, false},
		{"20 時間ちょうど", []map[string]any{kept(ago(20*time.Hour), "keep")}, true},
		{"20 時間を超えた", []map[string]any{kept(ago(30*time.Hour), "keep")}, true},
		{"新しい記録から数える", []map[string]any{kept(ago(30*time.Hour), "keep"), kept(ago(2*time.Hour), "keep")}, false},
		{"新しい記録は名前の順でなく時刻で選ぶ", []map[string]any{kept(ago(2*time.Hour), "keep"), kept(ago(30*time.Hour), "keep")}, false},
		{"あとの回の記録にその項目が無くても出さない", []map[string]any{kept(ago(2*time.Hour), "keep"), other}, false},
		{"時差つきの時刻", []map[string]any{kept(now.Add(-2*time.Hour).In(time.FixedZone("", 9*60*60)).Format(time.RFC3339), "keep")}, false},
		{"時刻を読めなければ出す", []map[string]any{kept("今朝", "keep")}, true},
		{"時刻が文字列でなければ出す", []map[string]any{kept(1, "keep")}, true},
		{"時刻の欄が無ければ出す", []map[string]any{kept(nil, "keep")}, true},
		{"先の時刻なら出す", []map[string]any{kept(ago(-time.Hour), "keep")}, true},
		{"閉じたものは 20 時間たっても出ない", []map[string]any{kept(ago(30*time.Hour), "close")}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := collectTitles(t, now, 1, defaultLimit, c.records...)
			if err != nil {
				t.Fatal(err)
			}
			if shown := slices.Contains(got, "課題 1"); shown != c.shown {
				t.Errorf("出た=%v、期待=%v（%v）", shown, c.shown, got)
			}
		})
	}

	// 次回に回した項目が 50 件あっても、上限に数えず、新しい項目の場所を取らない。
	var fifty []any
	for i := 1; i <= 50; i++ {
		fifty = append(fifty, map[string]any{"key": itemKey("demo", fmt.Sprintf("T-%d", i), "issue", fmt.Sprintf("課題 %d", i)), "decision": "keep"})
	}
	full := map[string]any{"collected": ago(time.Hour), "items": fifty}
	got, err := collectTitles(t, now, 51, defaultLimit, full)
	if err != nil || !slices.Equal(got, []string{"課題 51"}) {
		t.Errorf("回した 50 件と新しい 1 件: got %v, %v", got, err)
	}
	if got, err := collectTitles(t, now.Add(19*time.Hour), 51, 100, full); err != nil || len(got) != 51 {
		t.Errorf("集めてから 20 時間たつと 51 件: got %d 件, %v", len(got), err)
	}
}

// collectTitles は、担当の無い todo が n 件と records のある偽の kanban で andon triage collect の本体を回し、
// 出た項目の本文を並びのまま返す。
func collectTitles(t *testing.T, now time.Time, n, limit int, records ...map[string]any) ([]string, error) {
	t.Helper()
	dir := t.TempDir()
	// hc を見つけない（端末一覧をセルの本物から読まない）。
	t.Setenv("PATH", filepath.Join(dir, "nobin"))
	var list []string
	for i := 1; i <= n; i++ {
		list = append(list, fmt.Sprintf(`{"id": "T-%d", "title": "課題 %d", "status": "todo", "created": "%s"}`,
			i, i, now.Add(-time.Duration(400-i)*24*time.Hour).Format("2006-01-02T15:04:05Z")))
	}
	write := func(path, body string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	listPath := filepath.Join(dir, "list.json")
	write(listPath, "["+strings.Join(list, ",")+"]\n", 0o644)
	cli := filepath.Join(dir, "cli")
	write(cli, "#!/bin/sh\nwhile IFS= read -r ln; do printf '%s\\n' \"$ln\"; done < '"+listPath+"'\n", 0o755)
	for i, r := range records {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(dir, "kanban", "library", "triage", fmt.Sprintf("2026-11-22-T-%03d.json", 900+i)), string(b), 0o644)
	}
	l := &ledger{cli: cli, root: dir, slug: "demo", dir: filepath.Join(dir, "kanban"), now: now}
	var out bytes.Buffer
	if err := runCollect(l, limit, &out, nil); err != nil {
		return nil, err
	}
	var got Collected
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		return nil, err
	}
	var titles []string
	for _, it := range got.Items {
		titles = append(titles, it.Text)
	}
	return titles, nil
}
