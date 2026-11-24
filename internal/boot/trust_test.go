package boot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func trustEnv(t *testing.T, f *fixture) (Env, []string) {
	t.Helper()
	e := f.env()
	e.GhqRoot = filepath.Join(f.ws, ".ghq")
	a := filepath.Join(e.GhqRoot, "github.com", "example", "sample-site")
	b := filepath.Join(e.GhqRoot, "github.com", "example", "tools")
	mustMkdir(t, filepath.Join(a, ".git"))
	put(t, filepath.Join(b, ".git"), "gitdir: elsewhere\n")
	mustMkdir(t, filepath.Join(e.GhqRoot, "github.com", "example", "not-a-repo"))
	mustMkdir(t, filepath.Join(e.GhqRoot, "github.com", "example", "unlisted", ".git"))
	put(t, filepath.Join(f.ws, "projects", "sample-site", "kanban", "config.yaml"),
		"version: 3\nrepos:\n  - github.com/example/sample-site\n  - github.com/example/not-a-repo\n  - github.com/example/missing\nchief_of_staff: t5\n")
	put(t, filepath.Join(f.ws, "projects", "tools", "kanban", "config.yaml"),
		"version: 3\nrepos:\n  - github.com/example/tools\n  - ../../../etc\n")
	mustMkdir(t, filepath.Join(f.home, ".herdr", "worktrees", "sample-site", "x-1"))
	roots := []string{f.ws, a, b}
	for i, r := range roots {
		p, err := filepath.EvalSymlinks(r)
		if err != nil {
			t.Fatal(err)
		}
		roots[i] = p
	}
	return e, roots
}

func trustedClaude(t *testing.T, path string) map[string]bool {
	t.Helper()
	var cfg struct {
		Projects map[string]struct {
			Trust *bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	if err := json.Unmarshal([]byte(read(t, path)), &cfg); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	got := map[string]bool{}
	for p, v := range cfg.Projects {
		got[p] = v.Trust != nil && *v.Trust
	}
	return got
}

func TestTrustFromNothing(t *testing.T) {
	f := newFixture(t)
	e, roots := trustEnv(t, f)
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	cj := filepath.Join(f.claude, ".claude.json")
	got := trustedClaude(t, cj)
	if len(got) != len(roots) {
		t.Errorf("Claude Code の信頼 = %v, want %v だけ", got, roots)
	}
	for _, r := range roots {
		if !got[r] {
			t.Errorf("Claude Code: %s を信頼していない", r)
		}
	}
	if to, _ := os.Readlink(filepath.Join(f.home, ".claude.json")); to != cj {
		t.Errorf("作った .claude.json に ~/.claude.json のリンクが無い: %q", to)
	}
	toml := withoutCodexDefaults(read(t, filepath.Join(f.home, ".codex", "config.toml")))
	want := ""
	for i, r := range roots {
		if i > 0 {
			want += "\n"
		}
		want += "[projects." + quoteTOML(r) + "]\ntrust_level = \"trusted\"\n"
	}
	if toml != want {
		t.Errorf("config.toml =\n%s\nwant\n%s", toml, want)
	}
	if st, _ := os.Stat(cj); st.Mode().Perm() != 0o600 {
		t.Errorf(".claude.json の権限 = %v", st.Mode().Perm())
	}

	first := snapshot(t, f.root)
	f.log.Reset()
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, f.root) != first || f.log.Len() != 0 {
		t.Errorf("2回目に書いた: %s", f.log.String())
	}
}

func TestTrustKeepsExisting(t *testing.T) {
	f := newFixture(t)
	e, roots := trustEnv(t, f)
	ws, repo := roots[0], roots[1]
	cj := filepath.Join(f.claude, ".claude.json")
	before := `{"numStartups": 12345678901234567890, "oauthAccount": {"email": "a<b>&c@example.com"},
 "projects": {"/elsewhere": {"hasTrustDialogAccepted": false, "allowedTools": ["x"]},
  "` + ws + `": {"hasTrustDialogAccepted": false, "allowedTools": ["Bash(ls)"], "lastCost": 0.10},
  "` + roots[2] + `": {"hasTrustDialogAccepted": true}}}`
	put(t, cj, before)
	codex := filepath.Join(f.home, ".codex", "config.toml")
	put(t, codex, `model = "m"  # 利用者のもの

[projects."`+ws+`"]
trust_level = "untrusted"

[projects.'`+repo+`']
# trust_level の無い表

[features]
hooks = true
memories = false`)
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	dec := json.NewDecoder(strings.NewReader(read(t, cj)))
	dec.UseNumber()
	if err := dec.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if n := got["numStartups"].(json.Number).String(); n != "12345678901234567890" {
		t.Errorf("数が変わった: %s", n)
	}
	if !strings.Contains(read(t, cj), `"a<b>&c@example.com"`) {
		t.Error("文字列を escape して書き換えた")
	}
	ps := got["projects"].(map[string]any)
	if p := ps["/elsewhere"].(map[string]any); p["hasTrustDialogAccepted"] != false {
		t.Error("ほかの project の信頼を変えた")
	}
	p := ps[ws].(map[string]any)
	if p["hasTrustDialogAccepted"] != true || p["lastCost"].(json.Number).String() != "0.10" || p["allowedTools"].([]any)[0] != "Bash(ls)" {
		t.Errorf("セルの repo の project = %v", p)
	}
	if !trustedClaude(t, cj)[repo] {
		t.Error("ghq の repo を足していない")
	}

	toml := withoutCodexDefaults(read(t, codex))
	if !strings.HasPrefix(toml, `model = "m"  # 利用者のもの`+"\n") || !strings.Contains(toml, "[features]\nhooks = true\nmemories = false\n") {
		t.Errorf("ほかの設定が変わった:\n%s", toml)
	}
	if !strings.Contains(toml, `[projects."`+ws+`"]`+"\ntrust_level = \"untrusted\"\n") || strings.Count(toml, ws+`"]`) != 1 {
		t.Errorf("オーナーが untrusted にした表を変えた:\n%s", toml)
	}
	if !strings.Contains(toml, `[projects.'`+repo+`']`+"\ntrust_level = \"trusted\"\n# trust_level の無い表\n") || strings.Count(toml, repo) != 1 {
		t.Errorf("trust_level の無い表に1行足していない:\n%s", toml)
	}
	if !strings.HasSuffix(toml, "memories = false\n\n[projects."+quoteTOML(roots[2])+"]\ntrust_level = \"trusted\"\n") {
		t.Errorf("無い表を末尾に足していない:\n%s", toml)
	}

	first := snapshot(t, f.root)
	f.log.Reset()
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, f.root) != first || f.log.Len() != 0 {
		t.Errorf("2回目に書いた: %s", f.log.String())
	}
}

func TestTrustLeavesUnreadable(t *testing.T) {
	for name, c := range map[string]struct{ claude, codex string }{
		"not-json":   {claude: "not json", codex: "[projects]\n\"/x\" = { trust_level = \"trusted\" }\n"},
		"not-object": {claude: `{"projects": []}`, codex: "projects = { }\n"},
		"bad-escape": {claude: `{"projects": {"/x": 1}}`, codex: "[projects.\"/a\\tb\"]\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			e, roots := trustEnv(t, f)
			cj := filepath.Join(f.claude, ".claude.json")
			codex := filepath.Join(f.home, ".codex", "config.toml")
			claude := c.claude
			if name == "bad-escape" {
				claude = `{"projects": {"` + roots[0] + `": 1}}`
			}
			put(t, cj, claude)
			put(t, codex, c.codex)
			if err := Generate(e); err != nil {
				t.Fatal(err)
			}
			if read(t, cj) != claude || withoutCodexDefaults(read(t, codex)) != c.codex {
				t.Error("読めない設定を書き換えた")
			}
			if n := strings.Count(f.log.String(), "信頼を足さない"); n != 2 {
				t.Errorf("ログに2行（Claude Code と Codex）無い: %s", f.log.String())
			}
		})
	}
}

func TestTrustRefusesSymlinkOutsideGhq(t *testing.T) {
	f := newFixture(t)
	e, roots := trustEnv(t, f)
	outside := filepath.Join(f.root, "outside", "repo")
	mustMkdir(t, filepath.Join(outside, ".git"))
	if err := os.Symlink(outside, filepath.Join(e.GhqRoot, "github.com", "example", "escape")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.ws, "projects", "escape", "kanban", "config.yaml"), "version: 3\nrepos:\n  - github.com/example/escape\n")
	if err := Trust(e); err != nil {
		t.Fatal(err)
	}
	got := trustedClaude(t, filepath.Join(f.claude, ".claude.json"))
	real, _ := filepath.EvalSymlinks(outside)
	if got[real] || got[outside] || len(got) != len(roots) {
		t.Errorf("GHQ_ROOT の外を信頼した: %v", got)
	}
	if toml := read(t, filepath.Join(f.home, ".codex", "config.toml")); strings.Contains(toml, "outside") {
		t.Errorf("Codex で GHQ_ROOT の外を信頼した:\n%s", toml)
	}
	if !strings.Contains(f.log.String(), "github.com/example/escape が GHQ_ROOT の外") {
		t.Errorf("ログに無い: %s", f.log.String())
	}
}

func TestTrustClaudeConcurrentWriter(t *testing.T) {
	f := newFixture(t)
	e, roots := trustEnv(t, f)
	cj := filepath.Join(f.claude, ".claude.json")
	put(t, cj, `{"numStartups": 1}`)
	calls := 0
	claudeRace = func(path string) {
		calls++
		if calls == 1 {
			put(t, path, `{"numStartups": 2, "oauthAccount": {"emailAddress": "a@example.com"}}`)
		}
	}
	defer func() { claudeRace = func(string) {} }()
	if err := trustClaude(e); err != nil {
		t.Fatal(err)
	}
	s := read(t, cj)
	if calls != 2 || !strings.Contains(s, `"numStartups": 2`) || !strings.Contains(s, `"a@example.com"`) {
		t.Errorf("Claude Code の書き込みを失った（calls=%d）:\n%s", calls, s)
	}
	if got := trustedClaude(t, cj); len(got) != len(roots) {
		t.Errorf("信頼を足していない: %v", got)
	}

	put(t, cj, `{"numStartups": 1}`)
	n := 0
	claudeRace = func(path string) { n++; put(t, path, `{"numStartups": `+strings.Repeat("9", n)+`}`) }
	f.log.Reset()
	if err := trustClaude(e); err != nil {
		t.Fatal(err)
	}
	if s := read(t, cj); n != claudeTries || s != `{"numStartups": `+strings.Repeat("9", n)+`}` {
		t.Errorf("書き換わり続けるのに書いた（n=%d）: %s", n, s)
	}
	if !strings.Contains(f.log.String(), "読むたびに書き換わる") {
		t.Errorf("ログに無い: %s", f.log.String())
	}
}

func TestTrustCodexIgnoresTablesInMultilineStrings(t *testing.T) {
	f := newFixture(t)
	e, roots := trustEnv(t, f)
	codex := filepath.Join(f.home, ".codex", "config.toml")
	user := "developer_instructions = \"\"\"\n[projects." + quoteTOML(roots[0]) + "]\ntrust_level = \"trusted\"\n\"\"\"\nnote = '''\n[projects." + quoteTOML(roots[1]) + "]\n'''\n"
	put(t, codex, user)
	if err := trustCodex(e); err != nil {
		t.Fatal(err)
	}
	got := read(t, codex)
	if !strings.HasPrefix(got, user) {
		t.Errorf("文字列の中を書き換えた:\n%s", got)
	}
	trusted, err := codexTrusted(got)
	if err != nil {
		t.Fatalf("書いたものが TOML として読めない: %v\n%s", err, got)
	}
	for _, r := range roots {
		if !trusted[r] {
			t.Errorf("%s の本物の表が無い:\n%s", r, got)
		}
	}
	f.log.Reset()
	if err := trustCodex(e); err != nil {
		t.Fatal(err)
	}
	if read(t, codex) != got || f.log.Len() != 0 {
		t.Errorf("2回目に書いた: %s", f.log.String())
	}
}

func TestTrustCodexLeavesBrokenTOML(t *testing.T) {
	f := newFixture(t)
	e, _ := trustEnv(t, f)
	codex := filepath.Join(f.home, ".codex", "config.toml")
	put(t, codex, "developer_instructions = \"\"\"\n[projects.\"/x\"]\n")
	if err := trustCodex(e); err != nil {
		t.Fatal(err)
	}
	if read(t, codex) != "developer_instructions = \"\"\"\n[projects.\"/x\"]\n" || !strings.Contains(f.log.String(), "TOML として読めない") {
		t.Errorf("読めない TOML に書いた: %s", f.log.String())
	}
}

func TestTrustAddDirsOfPackageTemplates(t *testing.T) {
	f := newFixture(t)
	e, roots := trustEnv(t, f)
	src, err := filepath.Glob(filepath.Join("..", "..", "package", "templates", "*.md"))
	if err != nil || len(src) == 0 {
		t.Fatalf("package/templates が読めない: %v", err)
	}
	var dirs []string
	for _, s := range src {
		body := read(t, s)
		put(t, filepath.Join(f.pkg, "templates", filepath.Base(s)), body)
		args := strings.Fields(strings.ReplaceAll(body, DefaultPackage+"/", f.pkg+"/"))
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--add-dir" {
				dirs = append(dirs, args[i+1])
			}
		}
	}
	pb := filepath.Join(f.pkg, "skills", "chief-of-staff", "playbooks")
	if !containsString(dirs, pb) {
		t.Fatalf("chief-of-staff の --add-dir に %s が無い（テストの前提）: %v", pb, dirs)
	}
	for _, d := range dirs {
		if strings.HasPrefix(d, f.pkg+"/") {
			mustMkdir(t, d)
		}
	}
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	got := trustedClaude(t, filepath.Join(f.claude, ".claude.json"))
	for _, d := range dirs {
		real, err := filepath.EvalSymlinks(d)
		if err != nil {
			continue
		}
		if !got[real] {
			t.Errorf("--add-dir %s を信頼していない: %v", d, got)
		}
	}
	for _, d := range []string{filepath.Dir(pb), filepath.Join(f.pkg, "skills"), f.pkg} {
		real, _ := filepath.EvalSymlinks(d)
		if got[real] {
			t.Errorf("--add-dir の親 %s を信頼した", d)
		}
	}
	if toml := read(t, filepath.Join(f.home, ".codex", "config.toml")); strings.Contains(toml, "playbooks") {
		t.Errorf("Codex に --add-dir のフォルダを足した:\n%s", toml)
	}
	for _, r := range roots {
		if !got[r] {
			t.Errorf("根 %s を信頼していない", r)
		}
	}
	first := snapshot(t, f.root)
	f.log.Reset()
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, f.root) != first || f.log.Len() != 0 {
		t.Errorf("2回目に書いた: %s", f.log.String())
	}
}

func TestTrustAddDirsReadsOnlyCommandLines(t *testing.T) {
	f := newFixture(t)
	e := f.env()
	d := func(name string) string {
		p := filepath.Join(f.root, "dirs", name)
		mustMkdir(t, p)
		real, _ := filepath.EvalSymlinks(p)
		return real
	}
	cmd, resume, overridden, desc, body := d("cmd"), d("resume"), d("overridden"), d("desc"), d("body")
	put(t, filepath.Join(f.pkg, "templates", "a.md"), "---\nname: a\ndescription: x --add-dir "+desc+"\ncommand: claude --add-dir "+cmd+
		" --add-dir rel --add-dir "+filepath.Join(f.root, "dirs", "missing")+"\nresume: claude --resume --add-dir "+resume+"\n---\n--add-dir "+body+"\n")
	put(t, filepath.Join(f.pkg, "templates", "b.md"), "---\nname: b\ncommand: claude --add-dir "+overridden+"\n---\n")
	put(t, filepath.Join(f.ws, "templates.local", "b.md"), "---\nname: b\ncommand: claude\n---\n")
	got := e.addDirs()
	if len(got) != 2 || got[0] != cmd || got[1] != resume {
		t.Errorf("addDirs = %v, want [%s %s]", got, cmd, resume)
	}
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
