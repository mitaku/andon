package delegate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `{"version": 1, "project": "sample-site", "preset": "hobby",
 "grants": [
   {"id": "d1", "scope": "publish", "value": "none", "by": "owner", "at": "2026-11-05", "until": null, "evidence": "x", "revoked": null},
   {"id": "d2", "scope": "deploy", "value": "none", "by": "owner", "at": "2026-11-05", "until": "2026-12-31", "evidence": "y",
    "revoked": {"at": "2026-11-20", "by": "owner", "reason": "z"}},
   {"id": "d3", "scope": "merge", "value": "B", "by": "owner", "at": "2026-11-06", "until": "2026-11-30", "evidence": "w"}]}`

func TestParseAndEffective(t *testing.T) {
	f, err := Parse([]byte(sample), "sample-site")
	if err != nil {
		t.Fatal(err)
	}
	e := f.Effective("2026-11-10")
	if e["publish"] != "none" || e["deploy"] != "production" || e["merge"] != "B" || e["line-change"] != "plant" {
		t.Errorf("型 hobby に grant を重ねた値: %v", e)
	}
	if f.Effective("2026-12-01")["merge"] != "C" {
		t.Error("期限の過ぎた grant が効いている")
	}
	if e := (&File{Version: 1, Project: "x"}).Effective("2026-11-10"); e["merge"] != "B" || e["publish"] != "none" {
		t.Errorf("型が無ければ work: %v", e)
	}
}

func TestParseRejects(t *testing.T) {
	for name, c := range map[string]struct{ json, slug string }{
		"置き場と違う project":      {`{"version":1,"project":"a"}`, "b"},
		"知らない version":        {`{"version":2,"project":"a"}`, "a"},
		"知らないキー":              {`{"version":1,"project":"a","x":1}`, "a"},
		"andon の型が andon でない": {`{"version":1,"project":"andon","preset":"hobby"}`, "andon"},
		"andon の merge C":     {`{"version":1,"project":"andon","preset":"andon","grants":[{"id":"d1","scope":"merge","value":"C","by":"owner","at":"2026-11-05","evidence":"x"}]}`, "andon"},
		"id が重なる":             {`{"version":1,"project":"a","grants":[{"id":"d1","scope":"profile","value":true,"by":"owner","at":"2026-11-05","evidence":"x"},{"id":"d1","scope":"profile","value":false,"by":"owner","at":"2026-11-05","evidence":"x"}]}`, "a"},
		"delete-remote":       {`{"version":1,"project":"a","grants":[{"id":"d1","scope":"irreversible","value":["delete-remote"],"by":"owner","at":"2026-11-05","evidence":"x"}]}`, "a"},
		"暦に無い日":               {`{"version":1,"project":"a","grants":[{"id":"d1","scope":"profile","value":true,"by":"owner","at":"2026-02-30","evidence":"x"}]}`, "a"},
		"scope と値が合わない":       {`{"version":1,"project":"a","grants":[{"id":"d1","scope":"merge","value":"public","by":"owner","at":"2026-11-05","evidence":"x"}]}`, "a"},
		"evidence が無い":        {`{"version":1,"project":"a","grants":[{"id":"d1","scope":"profile","value":true,"by":"owner","at":"2026-11-05"}]}`, "a"},
	} {
		if _, err := Parse([]byte(c.json), c.slug); err == nil {
			t.Errorf("%s: 通ってしまう", name)
		}
	}
}

func TestNarrower(t *testing.T) {
	if !Narrower("merge", "A", "B") || Narrower("merge", "C", "B") || !Narrower("profile", false, true) || Narrower("profile", true, false) {
		t.Error("順序")
	}
	if !Narrower("irreversible", []any{"force-push"}, []string{"delete-branch", "force-push"}) || Narrower("irreversible", []any{"rewrite-history"}, []string{}) {
		t.Error("irreversible は部分集合")
	}
}

func TestOwner(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte("# 建屋\nowner = \"someone@example.com\" # 人\n[plugins]\nowner = \"other\"\n"), 0o644)
	if o := Owner(ws); o != "someone@example.com" {
		t.Errorf("owner: %q", o)
	}
}

// Read は HEAD だけを読み、author が owner でなければ無委譲。
func TestReadTrustsOnlyOwnerAtHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	ws := t.TempDir()
	run := func(env []string, args ...string) {
		c := exec.Command("git", append([]string{"-C", ws}, args...)...)
		c.Env = append(os.Environ(), append([]string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_COMMITTER_NAME=c", "GIT_COMMITTER_EMAIL=c@example.com"}, env...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	owner := []string{"GIT_AUTHOR_NAME=o", "GIT_AUTHOR_EMAIL=owner@example.com"}
	other := []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=agent@example.com"}
	run(nil, "init", "-q")
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte(`owner = "owner@example.com"`+"\n"), 0o644)
	os.MkdirAll(filepath.Join(ws, "projects", "sample-site"), 0o755)
	p := filepath.Join(ws, Rel("sample-site"))
	os.WriteFile(p, []byte(sample), 0o644)
	run(owner, "add", "-A")
	run(owner, "commit", "-qm", "d")
	if r := Read(ws, "sample-site", "2026-11-10"); !r.Trusted || r.Merge() != "B" || len(r.Grants) != 2 {
		t.Errorf("owner の commit: %+v", r)
	}
	// 作業ツリーの変更は読まない
	os.WriteFile(p, []byte(strings.Replace(sample, `"hobby"`, `"work"`, 1)), 0o644)
	if r := Read(ws, "sample-site", "2026-11-10"); r.Preset != "hobby" {
		t.Errorf("作業ツリーを読んだ: %+v", r)
	}
	run(other, "commit", "-qam", "x")
	if r := Read(ws, "sample-site", "2026-11-10"); r.Trusted || r.Merge() != "none" || !strings.Contains(r.Reason, "owner でない") {
		t.Errorf("owner でない commit を信じた: %+v", r)
	}
	if r := Read(ws, "nothing", "2026-11-10"); r.Trusted || r.Merge() != "none" {
		t.Errorf("無いのに委譲がある: %+v", r)
	}
}
