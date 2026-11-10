package issue

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHcGet は hc get の返事を FAKE_GET（終了コードは FAKE_GET_RC。stderr は FAKE_GET_ERR）にする偽の hc を PATH に置く。
func fakeHcGet(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	body := "#!/usr/bin/env bash\n[ \"$1\" = get ] || exit 9\necho \"${FAKE_GET_ERR:-}\" >&2\necho \"$FAKE_GET\"\nexit ${FAKE_GET_RC:-0}\n"
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// andon issue session は自分の端末のラベル session を出す。無い・形が違う・hc が落ちたら何も出さずに 0（template の行を落とさない）。
func TestSession(t *testing.T) {
	fakeHcGet(t)
	t.Setenv("HC_TERMINAL_ID", "t7")
	for _, c := range []struct{ get, rc, want string }{
		{`{"state":"live","labels":{"session":"0f9d2c4e-1b3a-4c5d-8e7f-a1b2c3d4e5f6"}}`, "0", "0f9d2c4e-1b3a-4c5d-8e7f-a1b2c3d4e5f6\n"},
		{`{"state":"live","labels":{}}`, "0", ""},
		{`{"state":"live","labels":{"session":"$(rm -rf /)"}}`, "0", ""},
		{``, "3", ""},
	} {
		t.Setenv("FAKE_GET", c.get)
		t.Setenv("FAKE_GET_RC", c.rc)
		var out, errb bytes.Buffer
		if rc := Main([]string{"session"}, strings.NewReader(""), &out, &errb); rc != 0 || out.String() != c.want {
			t.Errorf("%s: rc=%d out=%q err=%q", c.get, rc, out.String(), errb.String())
		}
	}
	t.Setenv("HC_TERMINAL_ID", "")
	var out, errb bytes.Buffer
	if rc := Main([]string{"session"}, strings.NewReader(""), &out, &errb); rc != 0 || out.Len() != 0 {
		t.Errorf("端末の外: rc=%d out=%q", rc, out.String())
	}
}

// 止まった端末の見分け: not_found・live でない・素のシェル（top-level も前に出ているものも）・それ以外は live。
func TestLookTerm(t *testing.T) {
	fakeHcGet(t)
	for _, c := range []struct{ get, rc, errOut, want string }{
		{``, "3", `{"error":{"kind":"not_found","message":"x"}}`, "not_found"},
		{`{"state":"finished","argv":["sh","-c","claude"]}`, "0", "", "finished"},
		{`{"state":"live","argv":["/bin/bash"],"foreground":{"argv":["/bin/bash"]}}`, "0", "", "bare_shell"},
		{`{"state":"live","argv":["/bin/bash"],"foreground":{"argv":["vim"]}}`, "0", "", "live"},
		{`{"state":"live","argv":["sh","-c","codex --no-daemon"],"foreground":{"argv":["sh","-c","codex --no-daemon"]}}`, "0", "", "live"},
	} {
		t.Setenv("FAKE_GET", c.get)
		t.Setenv("FAKE_GET_RC", c.rc)
		t.Setenv("FAKE_GET_ERR", c.errOut)
		got, err := lookTerm("t7")
		if err != nil || got.State != c.want {
			t.Errorf("%s: got %+v err=%v want %s", c.get, got, err, c.want)
		}
	}
	t.Setenv("FAKE_GET", "")
	t.Setenv("FAKE_GET_RC", "1")
	t.Setenv("FAKE_GET_ERR", "boom")
	if _, err := lookTerm("t7"); err == nil {
		t.Error("hc get が not_found でなく落ちたのに err が無い")
	}
}

// Codex の記録は先頭行だけを読み、cwd が同じもののうち最新の id。先頭行の外・形の違う行・別の cwd は数えない。
func TestCodexRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	cwd := t.TempDir()
	if id, n, err := codexRecord(cwd); id != "" || n != 0 || err != nil {
		t.Fatalf("記録の置き場が無い: %q %d %v", id, n, err)
	}
	dir := filepath.Join(home, "sessions", "2026", "11", "10")
	os.MkdirAll(dir, 0o755)
	put := func(name, body string) { os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644) }
	meta := func(id, cwd, at string) string {
		return `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"` + cwd + `","timestamp":"` + at + `"}}` + "\n"
	}
	put("rollout-1.jsonl", meta("11111111-1111-7111-8111-111111111111", cwd, "2026-11-10T01:00:00Z"))
	put("rollout-2.jsonl", meta("22222222-2222-7222-8222-222222222222", cwd, "2026-11-10T02:00:00Z"))
	put("rollout-3.jsonl", meta("33333333-3333-7333-8333-333333333333", "/elsewhere", "2026-11-10T03:00:00Z"))
	put("rollout-4.jsonl", `{"type":"x"}`+"\n"+meta("44444444-4444-7444-8444-444444444444", cwd, "2026-11-10T04:00:00Z"))
	put("rollout-5.jsonl", meta("not-an-id", cwd, "2026-11-10T05:00:00Z"))
	put("other.jsonl", meta("66666666-6666-7666-8666-666666666666", cwd, "2026-11-10T06:00:00Z"))
	id, n, err := codexRecord(cwd)
	if id != "22222222-2222-7222-8222-222222222222" || n != 5 || err != nil {
		t.Errorf("got %q %d %v", id, n, err)
	}
}
