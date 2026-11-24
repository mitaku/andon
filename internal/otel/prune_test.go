package otel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func put(t *testing.T, path string, size int, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// 受け入れ条件 6（片付けの側）: 最後に書いた時刻が 31 日前のファイルは消え、29 日前のファイルは残る。
func TestPruneAge(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "sample-site")
	old := filepath.Join(dir, "issues", "S-0001", "traces.jsonl")
	young := filepath.Join(dir, "issues", "S-0002", "traces.jsonl")
	oldRole := filepath.Join(dir, "roles", "chief-of-staff", "2026-10-24.logs.jsonl")
	other := filepath.Join(dir, "issues", "S-0003", "note.txt") // 生の記録でないものは消さない
	put(t, old, 10, now.Add(-31*24*time.Hour))
	put(t, young, 10, now.Add(-29*24*time.Hour))
	put(t, oldRole, 10, now.Add(-31*24*time.Hour))
	put(t, other, 10, now.Add(-90*24*time.Hour))
	n, err := Prune(dir, now, Default)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || exists(old) || exists(oldRole) {
		t.Errorf("31 日前のファイルを消さない（消した数 %d）", n)
	}
	if !exists(young) {
		t.Error("29 日前のファイルを消した")
	}
	if !exists(other) {
		t.Error(".jsonl でないファイルを消した")
	}
	if exists(filepath.Dir(old)) || exists(filepath.Dir(oldRole)) {
		t.Error("中身の無くなったフォルダが残っている")
	}
	if !exists(dir) {
		t.Error("プロジェクトの置き場そのものを消した")
	}
	// 2 度走っても結果は同じ
	if n, err := Prune(dir, now, Default); err != nil || n != 0 || !exists(young) {
		t.Errorf("2 度目の片付け: 消した数 %d・err %v", n, err)
	}
}

// 受け入れ条件 6（上限）: 合計が上限を超えるフォルダでは、古いファイルから消えて上限に収まる。
func TestPruneMaxBytes(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "sample-site")
	f1 := filepath.Join(dir, "issues", "S-0001", "traces.jsonl")
	f2 := filepath.Join(dir, "issues", "S-0001", "logs.jsonl")
	f3 := filepath.Join(dir, "roles", "worker", "2026-11-24.traces.jsonl")
	f4 := filepath.Join(dir, "issues", "S-0002", "traces.jsonl")
	put(t, f1, 400, now.Add(-4*time.Hour)) // 一番古い
	put(t, f2, 300, now.Add(-3*time.Hour))
	put(t, f3, 200, now.Add(-2*time.Hour))
	put(t, f4, 100, now.Add(-1*time.Hour))
	lim := Limits{Days: 30, MaxBytes: 350}
	n, err := Prune(dir, now, lim)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || exists(f1) || exists(f2) {
		t.Errorf("古い 2 本を消さない（消した数 %d）", n)
	}
	if !exists(f3) || !exists(f4) {
		t.Error("新しいファイルを消した")
	}
	var total int64
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, _ := d.Info()
			total += info.Size()
		}
		return nil
	})
	if total > lim.MaxBytes {
		t.Errorf("片付けのあとの合計 %d が上限 %d を超えている", total, lim.MaxBytes)
	}
}

// 置き場が無ければ何もしない。置き場の中の symlink は辿らず消さない。
func TestPruneMissingAndSymlink(t *testing.T) {
	now := time.Now()
	if n, err := Prune(filepath.Join(t.TempDir(), "none"), now, Default); n != 0 || err != nil {
		t.Errorf("無い置き場で %d・%v", n, err)
	}
	top := t.TempDir()
	target := filepath.Join(top, "outside.jsonl")
	put(t, target, 10, now.Add(-90*24*time.Hour))
	dir := filepath.Join(top, "sample-site")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Prune(dir, now, Limits{Days: 30, MaxBytes: 0}); err != nil {
		t.Fatal(err)
	}
	if !exists(target) || !exists(link) {
		t.Error("symlink かその先を消した")
	}
}

// PruneDue は 1 日に 1 回だけ走らせる。置き場の無いプロジェクトは何もしない。形の違う名前は断る。
func TestPruneDue(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	now := time.Now()
	if ran, _, err := PruneDue("sample-site", now); ran || err != nil {
		t.Errorf("置き場の無いプロジェクトで走らせた: %v・%v", ran, err)
	}
	if exists(filepath.Join(Root(), "sample-site")) {
		t.Error("置き場の無いプロジェクトに置き場を作った")
	}
	dir := filepath.Join(state, "andon", "otel", "sample-site")
	old := filepath.Join(dir, "issues", "S-0001", "traces.jsonl")
	put(t, old, 10, now.Add(-31*24*time.Hour))
	ran, n, err := PruneDue("sample-site", now)
	if !ran || n != 1 || err != nil || exists(old) {
		t.Fatalf("1 度目: ran=%v 消した数 %d err %v", ran, n, err)
	}
	put(t, old, 10, now.Add(-31*24*time.Hour))
	if ran, _, _ := PruneDue("sample-site", now.Add(23*time.Hour)); ran || !exists(old) {
		t.Error("1 日たたないうちに走らせた")
	}
	if ran, n, _ := PruneDue("sample-site", now.Add(25*time.Hour)); !ran || n != 1 {
		t.Errorf("1 日たったのに走らせない: ran=%v 消した数 %d", ran, n)
	}
	if _, _, err := PruneDue("..", now); err == nil {
		t.Error("形の違うプロジェクトの名前を断らない")
	}
}

// 片付けの一部が失敗しても、残りは片付けて失敗を返す。
func TestPruneReportsErrorsAndContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root では権限で失敗させられない")
	}
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "sample-site")
	locked := filepath.Join(dir, "issues", "S-0001", "traces.jsonl")
	free := filepath.Join(dir, "issues", "S-0002", "traces.jsonl")
	put(t, locked, 10, now.Add(-31*24*time.Hour))
	put(t, free, 10, now.Add(-31*24*time.Hour))
	if err := os.Chmod(filepath.Dir(locked), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(locked), 0o700) })
	n, err := Prune(dir, now, Default)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		t.Errorf("消せなかったことを返さない: %v", err)
	}
	if n != 1 || exists(free) {
		t.Errorf("失敗のあとの残りを片付けない（消した数 %d）", n)
	}
}
