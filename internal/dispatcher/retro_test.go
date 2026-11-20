package dispatcher

import (
	"os"
	"path/filepath"
	"testing"
)

// 件数のきっかけは、ルーチン retro がkanban にあり、止める印が無いときだけ数える（ADR-0039 決めたこと 8）。
// 読めない・paused が真偽でないファイルは、kanban の CLI も撃たないので無いと同じ。
func TestRetroOn(t *testing.T) {
	dir := t.TempDir()
	if retroOn(dir) {
		t.Error("ルーチン retro が無いのに数える")
	}
	for body, want := range map[string]bool{
		"title: 振り返り\nschedule: \"0 6 * * *\"\n": true,
		"title: 振り返り\npaused: false\n":           true,
		"title: 振り返り\npaused: true\n":            false,
		"title: 振り返り\npaused: \"yes\"\n":         false,
		"title: [振り返り\n":                         false,
		"":                                       false,
	} {
		if err := os.MkdirAll(filepath.Join(dir, "routines"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "routines", "retro.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := retroOn(dir); got != want {
			t.Errorf("%q → %v（want %v）", body, got, want)
		}
	}
}
