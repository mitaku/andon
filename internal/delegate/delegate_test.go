package delegate

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"andon/internal/contract"
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
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte("# セル\nowner = \"someone@example.com\" # 人\n[plugins]\nowner = \"other\"\n"), 0o644)
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
	p := filepath.Join(ws, ProjectRel("sample-site"))
	os.WriteFile(p, []byte(projectOf(sample)), 0o644)
	run(owner, "add", "-A")
	run(owner, "commit", "-qm", "d")
	if r := Read(ws, "sample-site", "2026-11-10"); !r.Trusted || r.Merge() != "B" || len(r.Grants) != 2 {
		t.Errorf("owner の commit: %+v", r)
	}
	// 作業ツリーの変更は読まない
	os.WriteFile(p, []byte(projectOf(strings.Replace(sample, `"hobby"`, `"work"`, 1))), 0o644)
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
	// 移す前の delegation.json は、owner が commit していても読まない（無委譲。理由に移すことを書く）
	os.MkdirAll(filepath.Join(ws, "projects", "legacy"), 0o755)
	os.WriteFile(filepath.Join(ws, LegacyRel("legacy")), []byte(strings.Replace(sample, "sample-site", "legacy", 1)), 0o644)
	run(owner, "add", "-A")
	run(owner, "commit", "-qm", "legacy")
	if r := Read(ws, "legacy", "2026-11-10"); r.Trusted || r.Merge() != "none" || !strings.Contains(r.Reason, "delegation.json は読まない") {
		t.Errorf("delegation.json を読んだ: %+v", r)
	}
	// project.json に delegation が無ければ無委譲
	os.WriteFile(filepath.Join(ws, ProjectRel("legacy")), []byte(`{"version":1,"project":{"title":"L","prefix":"L"},"repos":[]}`), 0o644)
	run(owner, "add", "-A")
	run(owner, "commit", "-qm", "project")
	if r := Read(ws, "legacy", "2026-11-10"); r.Trusted || !strings.Contains(r.Reason, "delegation が無い") {
		t.Errorf("delegation の無い project.json: %+v", r)
	}
	if p, err := ReadProject(ws, "legacy"); err != nil || len(p.Repos) != 0 {
		t.Errorf("ReadProject: %+v %v", p, err)
	}
}

// projectOf は委譲 d を delegation に持つ project.json。
func projectOf(d string) string {
	return `{"version": 1, "project": {"title": "Sample site", "prefix": "S"}, "repos": ["github.com/example/sample-site"], "delegation": ` + d + `}`
}

// signalsOf は signals に出所 sig を1つ持つ project.json（references に読むだけの repo と URL）。
func signalsOf(sig string) string {
	return `{"version": 1, "project": {"title": "Sample site", "prefix": "S"}, "repos": ["github.com/example/sample-site"],
 "references": ["github.com/example/design-notes", "https://example.com/spec"], "signals": [` + sig + `]}`
}

// ADR-0011 受け入れ条件 1: signals の正しい形は通り、書き直しても（andon delegate grant）そのまま残る。
func TestParseProjectSignals(t *testing.T) {
	j := `{"version": 1, "project": {"title": "Sample site", "prefix": "S"}, "repos": ["github.com/example/sample-site"],
 "references": ["github.com/example/design-notes"],
 "signals": [{"source": "github-ci", "repo": "github.com/example/sample-site", "weight": "now"},
  {"source": "github-issues", "repo": "github.com/example/design-notes", "weight": "later", "every": 30, "filter": "label:bug", "write_back": false},
  {"source": "sentry", "weight": "later", "org": "example", "project": "sample-site"}]}`
	p, err := ParseProject([]byte(j), "sample-site")
	if err != nil || len(p.Signals) != 3 || *p.Signals[1].Every != 30 || *p.Signals[1].Filter != "label:bug" || *p.Signals[2].Org != "example" {
		t.Fatalf("%+v %v", p, err)
	}
	var back, want any
	_ = json.Unmarshal(contract.Marshal(p), &back)
	_ = json.Unmarshal([]byte(j), &want)
	if !reflect.DeepEqual(back.(map[string]any)["signals"], want.(map[string]any)["signals"]) {
		t.Errorf("書き直すと signals が変わる: %s", contract.Marshal(p))
	}
	if p, err := ParseProject([]byte(signalsOf(`{"source":"github-ci","weight":"now"}`)), "sample-site"); err != nil || p.Signals[0].Repo != nil {
		t.Errorf("repo の無い出所: %+v %v", p, err)
	}
}

func TestParseProject(t *testing.T) {
	full := `{"version": 1, "project": {"title": "Sample site", "prefix": "S"},
 "repos": ["github.com/example/sample-site"], "references": ["github.com/example/design-notes", "https://example.com/spec"],
 "delegation": ` + sample + `, "chief_of_staff": {"parallel": 2, "review_when": ["外向きの面"], "triage_max_issues": 3}}`
	p, err := ParseProject([]byte(full), "sample-site")
	if err != nil || p.Project.Prefix != "S" || len(p.Repos) != 1 || *p.ChiefOfStaff.Parallel != 2 {
		t.Fatalf("%+v %v", p, err)
	}
	for name, j := range map[string]string{
		"repos が無い":            `{"version":1,"project":{"title":"x","prefix":"S"}}`,
		"project が無い":          `{"version":1,"repos":[]}`,
		"prefix が無い":           `{"version":1,"project":{"title":"x"},"repos":[]}`,
		"prefix が小文字":          `{"version":1,"project":{"title":"x","prefix":"s"},"repos":[]}`,
		"title が空":             `{"version":1,"project":{"title":"","prefix":"S"},"repos":[]}`,
		"知らないキー":               `{"version":1,"project":{"title":"x","prefix":"S"},"repos":[],"x":1}`,
		"project に知らないキー":      `{"version":1,"project":{"title":"x","prefix":"S","slug":"a"},"repos":[]}`,
		"知らない version":         `{"version":2,"project":{"title":"x","prefix":"S"},"repos":[]}`,
		"repo の形":              `{"version":1,"project":{"title":"x","prefix":"S"},"repos":["example/app"]}`,
		"repo が重なる":            `{"version":1,"project":{"title":"x","prefix":"S"},"repos":["github.com/a/b","github.com/a/b"]}`,
		"references が URL でない": `{"version":1,"project":{"title":"x","prefix":"S"},"repos":[],"references":["ftp://x"]}`,
		"delegation が null":    `{"version":1,"project":{"title":"x","prefix":"S"},"repos":[],"delegation":null}`,
		"delegation の形":        `{"version":1,"project":{"title":"x","prefix":"S"},"repos":[],"delegation":{"version":1,"project":"other"}}`,
		"parallel が 0":         `{"version":1,"project":{"title":"x","prefix":"S"},"repos":[],"chief_of_staff":{"parallel":0}}`,
		"cos に知らないキー":          `{"version":1,"project":{"title":"x","prefix":"S"},"repos":[],"chief_of_staff":{"goal":"x"}}`,
		"signals の weight":     signalsOf(`{"source":"github-ci","repo":"github.com/example/sample-site","weight":"soon"}`),
		"signals の weight が無い": signalsOf(`{"source":"github-ci","repo":"github.com/example/sample-site"}`),
		"signals の source":     signalsOf(`{"source":"datadog","weight":"later"}`),
		"signals の source が無い": signalsOf(`{"weight":"later"}`),
		"signals の repo が外":    signalsOf(`{"source":"github-issues","repo":"github.com/example/other","weight":"later"}`),
		"signals の repo が URL": signalsOf(`{"source":"github-issues","repo":"https://example.com/spec","weight":"later"}`),
		"signals の repo が空":    signalsOf(`{"source":"github-issues","repo":"","weight":"later"}`),
		"signals の every が 0":  signalsOf(`{"source":"github-ci","weight":"now","every":0}`),
		"signals の filter が空":  signalsOf(`{"source":"github-issues","weight":"later","filter":""}`),
		"signals の write_back": signalsOf(`{"source":"linear","weight":"later","write_back":"yes"}`),
		"signals に知らないキー":      signalsOf(`{"source":"sentry","weight":"later","token":"x"}`),
		"signals の null":       signalsOf(`{"source":"sentry","weight":"later","org":null}`),
	} {
		if _, err := ParseProject([]byte(j), "sample-site"); err == nil {
			t.Errorf("%s: 通ってしまう", name)
		}
	}
}

// show は --project と --json だけを受ける。grant・revoke の option は、projects/<slug>/ で打っても使い方の誤りになる。
func TestShowRefusesOtherCommandsOptions(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	dir := filepath.Join(ws, "projects", "sample-site")
	os.MkdirAll(dir, 0o755)
	t.Chdir(dir)
	var out, errb bytes.Buffer
	if rc := Main([]string{"show"}, nil, &out, &errb); rc != 0 || !strings.Contains(out.String(), "sample-site: 無委譲") {
		t.Fatalf("show: rc=%d out=%q err=%q", rc, out.String(), errb.String())
	}
	for _, o := range []string{"--reason", "--evidence", "--until"} {
		out.Reset()
		errb.Reset()
		if rc := Main([]string{"show", o, "x"}, nil, &out, &errb); rc != exitUsage || out.Len() != 0 || !strings.HasPrefix(errb.String(), "usage: andon delegate") {
			t.Errorf("show %s x: rc=%d out=%q err=%q", o, rc, out.String(), errb.String())
		}
	}
}

// grant は端末の git の user が owner と違っても、andon.toml の owner を author と committer にして commit する（読み手が信じる）。
// owner が andon.toml に無ければ commit せずに止まる。
func TestGrantCommitsAsOwner(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	ws := t.TempDir()
	t.Setenv("ANDON_WORKSPACE", ws)
	t.Setenv("HC_TERMINAL_ID", "")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", ws}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("config", "user.name", "Andon agent")
	run("config", "user.email", "agent@example.com")
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte(`owner = "owner@example.com"`+"\n"), 0o644)
	os.MkdirAll(filepath.Join(ws, "projects", "sample-site"), 0o755)
	os.WriteFile(filepath.Join(ws, ProjectRel("sample-site")), []byte(projectOf(sample)), 0o644)
	run("add", "-A")
	run("-c", "user.name=Owner", "-c", "user.email=owner@example.com", "commit", "-qm", "init")

	var out, errb strings.Builder
	if rc := Main([]string{"grant", "profile", "true", "--evidence", "x", "--project", "sample-site"}, nil, &out, &errb); rc != 0 {
		t.Fatalf("grant: %d %s", rc, errb.String())
	}
	if got := run("log", "-1", "--format=%an <%ae> / %cn <%ce>"); got != "Owner <owner@example.com> / Owner <owner@example.com>" {
		t.Errorf("author / committer: %s", got)
	}
	if r := Read(ws, "sample-site", "2026-11-10"); !r.Trusted || r.Effective["profile"] != true {
		t.Errorf("grant の commit を信じない: %+v", r)
	}

	// owner が andon.toml に無ければ commit しない
	os.WriteFile(filepath.Join(ws, "andon.toml"), []byte("# owner が無い\n"), 0o644)
	head := run("rev-parse", "HEAD")
	errb.Reset()
	if rc := Main([]string{"grant", "merge", "A", "--evidence", "x", "--project", "sample-site"}, nil, &out, &errb); rc != 1 || !strings.Contains(errb.String(), "owner が無い") {
		t.Errorf("owner が無いのに: %d %s", rc, errb.String())
	}
	if run("rev-parse", "HEAD") != head || run("status", "--porcelain", "--", ProjectRel("sample-site")) != "" {
		t.Error("owner が無いのに書いた")
	}
}

// .env は最後の空でない行の値（前後の空白と引用符を除く）。無ければ既定。boot と andon publish が読む。
func TestDotenv(t *testing.T) {
	for body, want := range map[string]string{
		"":          "def",
		"OTHER=1\n": "def",
		"PORTLESS_PORT=2000\nPORTLESS_PORT=2001\n": "2001",
		"PORTLESS_PORT=\"2002\"\n":                 "2002",
		"PORTLESS_PORT=' 2003 '\n":                 "2003",
		" PORTLESS_PORT = 2004 \n":                 "2004",
		"PORTLESS_PORT=2005\nPORTLESS_PORT=\n":     "2005",
		"# PORTLESS_PORT=9\n":                      "def",
	} {
		ws := t.TempDir()
		os.WriteFile(filepath.Join(ws, ".env"), []byte(body), 0o644)
		if got := Dotenv(ws, "PORTLESS_PORT", "def"); got != want {
			t.Errorf("%q: %q, want %q", body, got, want)
		}
	}
	if got := Dotenv(t.TempDir(), "PORTLESS_PORT", "def"); got != "def" {
		t.Errorf("no .env: %q", got)
	}
}
