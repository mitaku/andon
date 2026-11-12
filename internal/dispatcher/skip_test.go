package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Chief of Staff の端末が live でない台帳を飛ばし始めたら、ログに1行（台帳・宛先・理由）と herdr の通知を1つ出す。
// 同じ理由が続く間は繰り返さず、生き返ったあとにまた飛ばしたら出し直す。
func TestPassSkipIsNotSilent(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	herdr := "#!/bin/sh\necho \"$*\" >>\"$FAKE_KANBAN/herdr\"\n"
	for name, body := range map[string]string{"kanban": tickKanban, "hc": tickHc, "herdr": herdr} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_KANBAN", d)
	t.Setenv("XDG_STATE_HOME", filepath.Join(d, "state"))
	ws := filepath.Join(d, "ws")
	led := filepath.Join(ws, "projects", "sample-site", "kanban")
	if err := os.MkdirAll(led, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logb bytes.Buffer
	c := &Dispatcher{WS: ws, Log: &logb}
	notes := func() int {
		b, _ := os.ReadFile(filepath.Join(d, "herdr"))
		return strings.Count(string(b), "notification show")
	}

	c.Pass("")
	c.Pass("")
	out := logb.String()
	if strings.Count(out, "台帳を飛ばす") != 1 || !strings.Contains(out, "sample-site") || !strings.Contains(out, "t42") || !strings.Contains(out, "live でない") {
		t.Errorf("飛ばした 1 行（台帳・宛先・理由）が 1 回だけ出ていない: %q", out)
	}
	if n := notes(); n != 1 {
		t.Errorf("herdr の通知が %d 回（1 回のはず）", n)
	}

	if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil { // 生き返る
		t.Fatal(err)
	}
	c.Pass("")
	if err := os.Remove(filepath.Join(d, "cos")); err != nil { // また死ぬ
		t.Fatal(err)
	}
	c.Pass("")
	if strings.Count(logb.String(), "台帳を飛ばす") != 2 || notes() != 2 {
		t.Errorf("生き返ったあとの飛ばし直しで出し直していない: %q / %d", logb.String(), notes())
	}
}
