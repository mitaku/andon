package signal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --project は動詞のあとならどこでも受ける（ADR-0053 決めたこと 4 の細部）。前に置いても後ろに置いても、同じに読む。
func TestDeliveredProjectAnywhere(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "projects", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDON_WORKSPACE", ws)
	t.Setenv("KANBAN_ROOT", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HC_TERMINAL_ID", "dispatcher")
	try := func(args ...string) (int, string) {
		var out, errb bytes.Buffer
		rc := DeliveredMain(args, nil, &out, &errb)
		return rc, errb.String()
	}
	front, ferr := try("--project", "a", "fp", "3")
	for _, args := range [][]string{{"fp", "--project", "a", "3"}, {"fp", "3", "--project", "a"}} {
		rc, errs := try(args...)
		if rc != front || errs != ferr || strings.Contains(errs, "usage:") {
			t.Errorf("%v: rc=%d err=%q（前に置いたとき rc=%d err=%q）", args, rc, errs, front, ferr)
		}
	}
	if !strings.Contains(ferr, "andon signal delivered:") {
		t.Errorf("前に置いたとき、記録に無い信号の誤りでない: %q", ferr)
	}
	// 2つ目の --project・値の無い --project は使い方の誤り
	for _, args := range [][]string{{"fp", "3", "--project", "a", "--project", "a"}, {"fp", "3", "--project"}} {
		if rc, errs := try(args...); rc != 2 || !strings.Contains(errs, "usage:") {
			t.Errorf("%v: rc=%d err=%q", args, rc, errs)
		}
	}
}
