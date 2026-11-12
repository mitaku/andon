package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chief_of_staff が設定に無い台帳を飛ばし始めたら、ログに1行（台帳・理由）と herdr の通知を1つ出す。
// Chief of Staff の端末が live でない（留守）なら飛ばさず、ログに1行だけ（宛先・理由。herdr の通知は出さない。ADR-0017 決めたこと 8）。
// 同じ理由が続く間は繰り返さず、生き返ったあとにまた留守になったら出し直す。
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
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("timezone: UTC\n"), 0o644); err != nil {
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
	if strings.Count(out, "台帳を飛ばす") != 1 || !strings.Contains(out, "sample-site") || !strings.Contains(out, "chief_of_staff が空") {
		t.Errorf("飛ばした 1 行（台帳・理由）が 1 回だけ出ていない: %q", out)
	}
	if n := notes(); n != 1 {
		t.Errorf("herdr の通知が %d 回（1 回のはず）", n)
	}

	logb.Reset()
	if err := os.WriteFile(filepath.Join(led, "config.yaml"), []byte("chief_of_staff: t42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.Pass("")
	c.Pass("")
	out = logb.String()
	if strings.Count(out, "留守") != 1 || strings.Contains(out, "台帳を飛ばす") || !strings.Contains(out, "t42") || !strings.Contains(out, "が無い") {
		t.Errorf("留守の 1 行（台帳・宛先・理由）が 1 回だけ出ていない: %q", out)
	}
	if n := notes(); n != 1 {
		t.Errorf("留守で herdr の通知を出した（%d 回）", n-1)
	}

	if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil { // 生き返る
		t.Fatal(err)
	}
	c.Pass("")
	if err := os.Remove(filepath.Join(d, "cos")); err != nil { // また死ぬ
		t.Fatal(err)
	}
	c.Pass("")
	if strings.Count(logb.String(), "留守") != 2 {
		t.Errorf("生き返ったあとの留守で出し直していない: %q", logb.String())
	}

	// hc get が一時に落ちた（not_found でない）巡は、留守にしない（控えの留守の記録を作らない）。ログは1行
	if err := os.WriteFile(filepath.Join(d, "cos"), nil, 0o644); err != nil { // 生き返る（留守の記録は戻った巡で消える）
		t.Fatal(err)
	}
	c.Pass("")
	os.Remove(filepath.Join(d, "cos"))
	if err := os.WriteFile(filepath.Join(d, "hcfail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	logb.Reset()
	c.Pass("")
	c.Pass("")
	if st, _, err := LoadState("sample-site"); err != nil || st.Away != nil {
		t.Errorf("hc get の一時の失敗を留守にした: %+v %v", st.Away, err)
	}
	if out := logb.String(); strings.Count(out, "確かめられない") != 1 || strings.Contains(out, "留守（") {
		t.Errorf("確かめられない巡のログが1行でない・留守と書いた: %q", out)
	}
}
