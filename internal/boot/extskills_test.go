package boot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bundled(t *testing.T, e Env) string {
	t.Helper()
	d := filepath.Join(e.Tools, bundledSkills["playwright-cli"])
	put(t, filepath.Join(d, "SKILL.md"), "playwright-cli\n")
	return d
}

func symlinkT(t *testing.T, to, link string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(link))
	if err := os.Symlink(to, link); err != nil {
		t.Fatal(err)
	}
}

func TestBundledSkillLinks(t *testing.T) {
	f := newFixture(t)
	e := f.env()
	put(t, filepath.Join(f.pkg, "skills", "worker", "SKILL.md"), "worker\n")
	target := bundled(t, e)
	agents := filepath.Join(f.home, ".agents", "skills")
	claude := filepath.Join(f.claude, "skills")
	symlinkT(t, filepath.Join(e.Tools, "lib", "gone"), filepath.Join(agents, "gone-tool"))
	symlinkT(t, filepath.Join(f.root, "mine"), filepath.Join(agents, "mine"))
	symlinkT(t, filepath.Join(f.root, "mine"), filepath.Join(claude, "mine"))

	for range 2 {
		if err := Generate(e); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range []string{filepath.Join(agents, "playwright-cli"), filepath.Join(claude, "playwright-cli")} {
		if to, err := os.Readlink(link); err != nil || to != target {
			t.Errorf("%s → %q %v", link, to, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(agents, "gone-tool")); err == nil {
		t.Error("備品の置き場の下を指す古いリンクが残っている")
	}
	for _, p := range []string{filepath.Join(agents, "mine"), filepath.Join(claude, "mine")} {
		if to, _ := os.Readlink(p); to != filepath.Join(f.root, "mine") {
			t.Errorf("利用者の skill に触った: %s → %q", p, to)
		}
	}
	if to, _ := os.Readlink(filepath.Join(agents, "andon-worker")); to != filepath.Join(f.pkg, "skills", "worker") {
		t.Errorf("andon-worker → %q", to)
	}
	before := snapshot(t, f.home)
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, f.home) != before {
		t.Error("3回目に変わった（冪等でない）")
	}
}

func TestBundledSkillSameNameUserSkillIsKept(t *testing.T) {
	f := newFixture(t)
	e := f.env()
	bundled(t, e)
	agents := filepath.Join(f.home, ".agents", "skills")
	claude := filepath.Join(f.claude, "skills")
	put(t, filepath.Join(claude, "playwright-cli", "SKILL.md"), "利用者の\n")
	symlinkT(t, filepath.Join(f.root, "mine"), filepath.Join(agents, "playwright-cli"))
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(claude, "playwright-cli", "SKILL.md")); got != "利用者の\n" {
		t.Errorf("実ディレクトリに触った: %q", got)
	}
	if to, _ := os.Readlink(filepath.Join(agents, "playwright-cli")); to != filepath.Join(f.root, "mine") {
		t.Errorf("利用者のリンクを張り替えた: → %q", to)
	}
	for _, p := range []string{filepath.Join(agents, "playwright-cli"), filepath.Join(claude, "playwright-cli")} {
		if !strings.Contains(f.log.String(), "触らない: "+p) {
			t.Errorf("ログに %s が無い: %s", p, f.log.String())
		}
	}
}

func TestBundledSkillFollowsTools(t *testing.T) {
	f := newFixture(t)
	e := f.env()
	target := bundled(t, e)
	link := filepath.Join(f.home, ".agents", "skills", "playwright-cli")
	symlinkT(t, filepath.Join(DefaultTools, bundledSkills["playwright-cli"]), link)
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	if to, _ := os.Readlink(link); to != target {
		t.Errorf("張り替えていない: → %q", to)
	}
	if err := os.RemoveAll(e.Tools); err != nil {
		t.Fatal(err)
	}
	if err := Generate(e); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{link, filepath.Join(f.claude, "skills", "playwright-cli")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("備品に無い skill のリンクが残っている: %s", p)
		}
	}
}

func ghSkillFake(t *testing.T, f *fixture, loggedIn bool) (Env, string) {
	t.Helper()
	calls := filepath.Join(f.root, "gh-calls")
	gh := filepath.Join(f.root, "bin", "gh")
	rc := "0"
	if !loggedIn {
		rc = "1"
	}
	put(t, gh, `#!/bin/sh
echo "$*" >> `+calls+`
[ "$*" = "auth status" ] && exit `+rc+`
if [ "$1 $2" = "skill install" ]; then
  case "$6" in
    claude-code) d="$CLAUDE_CONFIG_DIR/skills" ;;
    codex) d="$HOME/.agents/skills" ;;
    *) exit 2 ;;
  esac
  mkdir -p "$d/$(basename "$4")"
  printf -- '---\nmetadata:\n    github-repo: https://github.com/%s\n---\n' "$3" > "$d/$(basename "$4")/SKILL.md"
fi
exit 0
`)
	if err := os.Chmod(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	e := f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "gh" {
			return gh, nil
		}
		return "", errors.New("無い")
	}
	return e, calls
}

func ghCalls(t *testing.T, calls string) string {
	t.Helper()
	b, err := os.ReadFile(calls)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallGhSkills(t *testing.T) {
	f := newFixture(t)
	e, calls := ghSkillFake(t, f, true)
	put(t, filepath.Join(f.pkg, GhSkillsFile), "# 既定\nexample/skills alpha\n")
	put(t, filepath.Join(f.ws, "local", GhSkillsFile), "\nexample/skills alpha\nexample/more author/beta\nbad-line\n")
	user := "---\nmetadata:\n    github-repo: https://github.com/someone/beta\n---\n利用者の\n"
	put(t, filepath.Join(f.home, ".agents", "skills", "beta", "SKILL.md"), user)
	put(t, filepath.Join(f.home, ".claude", "skills", "mine", "SKILL.md"), "一覧の外\n")

	for range 2 {
		if err := installGhSkills(e); err != nil {
			t.Fatal(err)
		}
	}
	update := "skill update --all --dir " + filepath.Join(f.claude, "skills") + " alpha beta\n" +
		"skill update --all --dir " + filepath.Join(f.home, ".agents", "skills") + " alpha\n"
	want := "auth status\n" +
		"skill install example/skills alpha --agent claude-code --scope user\n" +
		"skill install example/skills alpha --agent codex --scope user\n" +
		"skill install example/more author/beta --agent claude-code --scope user\n" +
		update +
		"auth status\n" + update
	if got := ghCalls(t, calls); got != want {
		t.Errorf("gh の呼び出し:\n%s\nwant:\n%s", got, want)
	}
	if got := read(t, filepath.Join(f.home, ".agents", "skills", "beta", "SKILL.md")); got != user {
		t.Errorf("利用者の skill に触った: %q", got)
	}
	if !strings.Contains(f.log.String(), GhSkillsFile+":4 は「<owner>/<repo> <skill>」の形でない") {
		t.Errorf("形の違う行のログ: %s", f.log.String())
	}
}

func TestFromRepo(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		body string
		want bool
	}{
		{"---\nname: a\nmetadata:\n    github-repo: https://github.com/example/skills\n---\n", true},
		{"---\nmetadata:\n  github-repo: \"https://github.com/Example/Skills.git\"\n---\n", true},
		{"---\r\nmetadata:\r\n    github-repo: https://github.com/example/skills\r\n---\r\n", true},
		{"---\nmetadata:\n    github-repo: https://github.com/someone/skills\n---\n", false},
		{"---\nname: a\n---\ngithub-repo: https://github.com/example/skills\n", false},
		{"github-repo: https://github.com/example/skills\n", false},
		{"", false},
	} {
		p := filepath.Join(dir, "SKILL.md")
		put(t, p, c.body)
		if got := fromRepo(p, "example/skills"); got != c.want {
			t.Errorf("fromRepo(%q) = %v, want %v", c.body, got, c.want)
		}
	}
	if fromRepo(filepath.Join(dir, "none", "SKILL.md"), "example/skills") {
		t.Error("無いファイルで true")
	}
}

func TestInstallGhSkillsNoUpdateWhenNothingInstalled(t *testing.T) {
	f := newFixture(t)
	calls := filepath.Join(f.root, "gh-calls")
	gh := filepath.Join(f.root, "bin", "gh")
	put(t, gh, "#!/bin/sh\necho \"$*\" >> "+calls+"\n[ \"$*\" = \"auth status\" ] && exit 0\nexit 1\n")
	if err := os.Chmod(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	e := f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "gh" {
			return gh, nil
		}
		return "", errors.New("無い")
	}
	put(t, filepath.Join(f.pkg, GhSkillsFile), "example/skills alpha\n")
	if err := installGhSkills(e); err == nil {
		t.Error("install の失敗を返さない")
	}
	if got := ghCalls(t, calls); strings.Contains(got, "skill update") {
		t.Errorf("入っていないのに update を打った: %q", got)
	}
}

func TestInstallGhSkillsEmptyList(t *testing.T) {
	f := newFixture(t)
	e, calls := ghSkillFake(t, f, true)
	if err := installGhSkills(e); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(f.pkg, GhSkillsFile), "# 既定は空\n\n")
	if err := installGhSkills(e); err != nil {
		t.Fatal(err)
	}
	if got := ghCalls(t, calls); got != "" || f.log.Len() != 0 {
		t.Errorf("一覧が空なのに打った: %q %q", got, f.log.String())
	}
}

func TestInstallGhSkillsNotLoggedIn(t *testing.T) {
	f := newFixture(t)
	e, calls := ghSkillFake(t, f, false)
	put(t, filepath.Join(f.pkg, GhSkillsFile), "example/skills alpha\n")
	if err := installGhSkills(e); err != nil {
		t.Fatal(err)
	}
	if got := ghCalls(t, calls); got != "auth status\n" {
		t.Errorf("未ログインで打った: %q", got)
	}
	if !strings.Contains(f.log.String(), "gh はログインしていない。外の skill（1 件）は入れない") {
		t.Errorf("ログ: %q", f.log.String())
	}
	f.log.Reset()
	if err := installGhSkills(f.env()); err != nil || !strings.Contains(f.log.String(), "gh が無い") {
		t.Errorf("gh が無いとき: %v %q", err, f.log.String())
	}
}

func TestInstallGhSkillsSharedDeadline(t *testing.T) {
	f := newFixture(t)
	calls := filepath.Join(f.root, "gh-calls")
	gh := filepath.Join(f.root, "bin", "gh")
	put(t, gh, "#!/bin/sh\necho \"$*\" >> "+calls+"\n[ \"$*\" = \"auth status\" ] && exit 0\nexec sleep 5\n")
	if err := os.Chmod(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	e := f.env()
	e.LookPath = func(n string) (string, error) {
		if n == "gh" {
			return gh, nil
		}
		return "", errors.New("無い")
	}
	put(t, filepath.Join(f.pkg, GhSkillsFile), "example/skills alpha\n")
	old := ghTimeout
	ghTimeout = 300 * time.Millisecond
	defer func() { ghTimeout = old }()

	start := time.Now()
	err := installGhSkills(e)
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("全体の期限で戻らない: %v", d)
	}
	if err == nil || !strings.Contains(err.Error(), "残りを打たない") {
		t.Errorf("期限切れを返さない: %v", err)
	}
	if got := ghCalls(t, calls); got != "auth status\nskill install example/skills alpha --agent claude-code --scope user\n" {
		t.Errorf("期限のあとも打った: %q", got)
	}
}
