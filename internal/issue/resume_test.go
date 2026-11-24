package issue

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeHcGet(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	body := "#!/usr/bin/env bash\n[ \"$1\" = get ] || exit 9\necho \"${FAKE_GET_ERR:-}\" >&2\necho \"$FAKE_GET\"\nexit ${FAKE_GET_RC:-0}\n"
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

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

func TestFreshMessageGuide(t *testing.T) {
	with := FreshMessage("sample-site", "S-1", "/b.md", "/r.md", "\nオーナーの指針 `/g` も読む。")
	if !strings.Contains(with, "オーナーの指針") {
		t.Errorf("指針の行が入らない: %q", with)
	}
	if without := FreshMessage("sample-site", "S-1", "/b.md", "/r.md", ""); strings.Contains(without, "オーナーの指針") {
		t.Errorf("指針の行が入った: %q", without)
	}
}

func TestResumeRunsHcAndMovesAgent(t *testing.T) {
	f := newStart(t)
	sid := "11111111-2222-4333-8444-555555555555"
	cfg := filepath.Join(f.home, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	os.MkdirAll(filepath.Join(cfg, "projects", "x"), 0o755)
	os.WriteFile(filepath.Join(cfg, "projects", "x", sid+".jsonl"), nil, 0o644)
	hc := strings.Replace(startHc, "case $1 in\n", "case $1 in\n  get) echo '{\"error\":{\"kind\":\"not_found\"}}' >&2; exit 3 ;;\n", 1)
	os.WriteFile(filepath.Join(filepath.Dir(f.log), "bin", "hc"), []byte(hc), 0o755)
	t.Setenv("FAKE_TPL", `{"templates":[{"name":"claude"},{"name":"claude-resume","command":"claude --resume \"$s\""}]}`)
	t.Setenv("FAKE_RUN", `{"address":"t43","first_message":{"sent":true}}`)
	f.task = map[string]any{"id": "S-0007", "title": "Add the button", "status": "doing", "profile": "claude", "agent": "t50", "session": sid}
	os.WriteFile(filepath.Join(f.root, "show-S-0007.json"), []byte(js(map[string]any{"issue": f.task})), 0o644)
	var out, errb bytes.Buffer
	if rc := Main([]string{"resume", "S-0007"}, strings.NewReader(""), &out, &errb); rc != 0 {
		t.Fatalf("rc=%d out=%q err=%q calls=%q", rc, out.String(), errb.String(), f.calls())
	}
	args := strings.Join(f.runArgs(), " ")
	for _, want := range []string{"--name sample-site-s-0007", "--label issue=S-0007", "--label session=" + sid, "claude-resume"} {
		if !strings.Contains(args, want) {
			t.Errorf("hc run の引数に %q が無い: %q", want, args)
		}
	}
	if !f.has("kanban set S-0007 agent=t43") {
		t.Errorf("kanban の agent を起動した端末に替えない: %q", f.calls())
	}
}
