package boot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trustEnv は建屋の repo と、台帳に載った ghq の repo を2つ（片方は .git が file の worktree 型）置いた Env。
// 台帳に載っていない repo・clone されていない repo・形の違う repo の名前は根にしない。
func trustEnv(t *testing.T, f *fixture) (Env, []string) {
	t.Helper()
	e := f.env()
	e.GhqRoot = filepath.Join(f.ws, ".ghq")
	a := filepath.Join(e.GhqRoot, "github.com", "example", "sample-site")
	b := filepath.Join(e.GhqRoot, "github.com", "example", "tools")
	mustMkdir(t, filepath.Join(a, ".git"))
	put(t, filepath.Join(b, ".git"), "gitdir: elsewhere\n")
	mustMkdir(t, filepath.Join(e.GhqRoot, "github.com", "example", "not-a-repo"))
	mustMkdir(t, filepath.Join(e.GhqRoot, "github.com", "example", "unlisted", ".git")) // 台帳に無い repo は信頼しない
	put(t, filepath.Join(f.ws, "projects", "sample-site", ".flightdeck", "config.yaml"),
		"version: 2\nrepos:\n  - github.com/example/sample-site\n  - github.com/example/not-a-repo\n  - github.com/example/missing\ncoordinator: t5\n")
	put(t, filepath.Join(f.ws, "projects", "tools", ".flightdeck", "config.yaml"),
		"version: 2\nrepos:\n  - github.com/example/tools\n  - ../../../etc\n")
	mustMkdir(t, filepath.Join(f.home, ".herdr", "worktrees", "sample-site", "x-1")) // 置き場は足さない
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
	toml := read(t, filepath.Join(f.home, ".codex", "config.toml"))
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

	// 2回目は何も書かない
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
	// 利用者の設定: 大きな数・<>& を含む文字列・ほかの project・信頼前の project（false と、ほかのキー）・信頼済みの project
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
hooks = true`)
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
		t.Errorf("建屋の repo の project = %v", p)
	}
	if !trustedClaude(t, cj)[repo] {
		t.Error("ghq の repo を足していない")
	}

	toml := read(t, codex)
	if !strings.HasPrefix(toml, `model = "m"  # 利用者のもの`+"\n") || !strings.Contains(toml, "[features]\nhooks = true\n") {
		t.Errorf("ほかの設定が変わった:\n%s", toml)
	}
	if !strings.Contains(toml, `[projects."`+ws+`"]`+"\ntrust_level = \"untrusted\"\n") || strings.Count(toml, ws+`"]`) != 1 {
		t.Errorf("人が untrusted にした表を変えた:\n%s", toml)
	}
	if !strings.Contains(toml, `[projects.'`+repo+`']`+"\ntrust_level = \"trusted\"\n# trust_level の無い表\n") || strings.Count(toml, repo) != 1 {
		t.Errorf("trust_level の無い表に1行足していない:\n%s", toml)
	}
	if !strings.HasSuffix(toml, "hooks = true\n\n[projects."+quoteTOML(roots[2])+"]\ntrust_level = \"trusted\"\n") {
		t.Errorf("無い表を末尾に足していない:\n%s", toml)
	}

	// 2回目は何も書かない
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
				claude = `{"projects": {"` + roots[0] + `": 1}}` // 根の project が object でない
			}
			put(t, cj, claude)
			put(t, codex, c.codex)
			if err := Generate(e); err != nil {
				t.Fatal(err)
			}
			if read(t, cj) != claude || read(t, codex) != c.codex {
				t.Error("読めない設定を書き換えた")
			}
			if n := strings.Count(f.log.String(), "信頼を足さない"); n != 2 {
				t.Errorf("ログに2行（Claude Code と Codex）無い: %s", f.log.String())
			}
		})
	}
}
