package dispatcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"andon/internal/ledger"
)

// 件数のきっかけは、ルーチン retro がkanban にあり、止める印が無いときだけ数える（ADR-0039 決めたこと 8）。
// 読めない・paused が真偽でないファイルは、kanban の CLI も撃たないので無いと同じ。
func TestRetroOn(t *testing.T) {
	dir := t.TempDir()
	if routineOn(dir, retroRoutine) {
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
		if got := routineOn(dir, retroRoutine); got != want {
			t.Errorf("%q → %v（want %v）", body, got, want)
		}
	}
}

func TestRetroDueLogsUnreadableAgainAfterReadable(t *testing.T) {
	ws := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", ws}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	rel := filepath.Join("projects", "demo", "project.json")
	os.MkdirAll(filepath.Join(ws, "projects", "demo"), 0o755)
	os.WriteFile(filepath.Join(ws, rel), []byte(`{"version":1,"project":{"title":"Demo","prefix":"S"},"repos":["github.com/example/app"],"chief_of_staff":{"retro_every":5}}`), 0o644)
	git("init", "-q")
	git("add", rel)
	git("-c", "user.name=o", "-c", "user.email=owner@example.com", "commit", "-q", "-m", "project")
	toml := filepath.Join(ws, "andon.toml")

	var log strings.Builder
	p := gluedProject(t, &log)
	p.c.WS = ws
	os.MkdirAll(filepath.Join(p.l.Dir, "routines"), 0o755)
	os.WriteFile(filepath.Join(p.l.Dir, "routines", "retro.yaml"), []byte("title: 振り返り\n"), 0o644)
	p.tasks = []ledger.Issue{{"id": "S-0001", "title": "直す", "status": "done", "profile": "claude"}}
	p.rows = []ledger.Row{{At: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), ID: "S-0001", Change: map[string]any{"status": "done"}}}

	unreadable := func() int { return strings.Count(log.String(), "retro_every を読めない") }
	p.retroDue()
	if n := unreadable(); n != 1 {
		t.Fatalf("1回目の読めなさのログが %d 行:\n%s", n, log.String())
	}
	os.WriteFile(toml, []byte("owner = \"owner@example.com\"\n"), 0o644)
	p.retroDue()
	if n := unreadable(); n != 1 {
		t.Fatalf("読めたのに読めないとログした（%d 行）:\n%s", n, log.String())
	}
	os.Remove(toml)
	p.retroDue()
	if n := unreadable(); n != 2 {
		t.Fatalf("また読めなくなったのにログが %d 行（2 行のはず）:\n%s", n, log.String())
	}
}

func TestReReviewRangeReadsBack(t *testing.T) {
	prev, cur, mb := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	for _, c := range []struct {
		name    string
		sp      span
		rebased bool
	}{
		{"祖先", span{}, false},
		{"確かめられない", span{Unknown: true}, false},
		{"rebase（base を読めた）", span{Rebased: true, Old: mb, New: mb}, true},
		{"rebase（base を読めない）", span{Rebased: true}, true},
	} {
		got, rebased := ledger.BriefReReview(reReviewRange(prev, cur, "https://github.com/example/app/pull/7", c.sp))
		if got != prev || rebased != c.rebased {
			t.Errorf("%s: 前の回 %q・rebase %v（want %q・%v）", c.name, got, rebased, prev, c.rebased)
		}
	}
	if got, _ := ledger.BriefReReview("対象: worktree `/w`（detached）。commit " + cur + "。\n"); got != "" {
		t.Errorf("1回目の依頼書から前の回の commit を読んだ: %q", got)
	}
}
