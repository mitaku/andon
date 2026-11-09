package boot

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testOwner = "12345+sample-owner@users.noreply.github.com"

// gitFixture は git を使える Env と、セルの repo・案件の repo・その worktree を作る。
func gitFixture(t *testing.T, owner string) (*fixture, Env, map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	f := newFixture(t)
	if owner != "" {
		put(t, filepath.Join(f.ws, "andon.toml"), "slug = \"sample\"\nowner = \""+owner+"\"\n")
	}
	e := f.env()
	e.GhqRoot = filepath.Join(f.ws, ".ghq")
	e.LookPath = func(n string) (string, error) {
		if n == "git" {
			return exec.LookPath("git")
		}
		return "", errors.New("無い")
	}
	product := filepath.Join(e.GhqRoot, "example.com", "o", "r")
	wt := filepath.Join(f.home, ".herdr", "worktrees", "r", "r-1")
	gitIn(t, f, "", "init", "-q", f.ws)
	gitIn(t, f, "", "init", "-q", product)
	gitIn(t, f, product, "-c", "user.name=x", "-c", "user.email=x@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, f, product, "worktree", "add", "-q", wt)
	return f, e, map[string]string{"cell": f.ws, "product": product, "worktree": wt}
}

// gitIn はセルの中と同じく、作者を環境変数で渡さずに git を打つ（HOME は偽の HOME。system の設定は読まない）。
func gitIn(t *testing.T, f *fixture, dir string, args ...string) string {
	t.Helper()
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	c := exec.Command("git", args...)
	c.Env = gitEnv(f)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitEnv(f *fixture) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "GIT_") || k == "HOME" || k == "XDG_CONFIG_HOME" || k == "EMAIL" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+f.home, "GIT_CONFIG_NOSYSTEM=1")
}

// 作者の無いセルで boot のあと、作者を指定せずに commit が通り、案件の repo（と worktree）は owner、セルの repo は owner でない。
func TestGitAuthorCommit(t *testing.T) {
	f, e, repos := gitFixture(t, testOwner)
	if err := genGitAuthor(e); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"product":  "sample-owner <" + testOwner + ">",
		"worktree": "sample-owner <" + testOwner + ">",
		"cell":     AgentName + " <" + AgentEmail + ">",
	} {
		dir := repos[name]
		gitIn(t, f, dir, "commit", "-q", "--allow-empty", "-m", "boot のあと")
		if got := gitIn(t, f, dir, "log", "-1", "--format=%an <%ae>"); got != want {
			t.Errorf("%s の作者: %q, want %q", name, got, want)
		}
	}
}

// boot を2回打っても ~/.gitconfig と include のファイルは同じ。gh の credential helper など、ほかの節は残る。
func TestGitAuthorIdempotent(t *testing.T) {
	f, e, _ := gitFixture(t, testOwner)
	gc := filepath.Join(f.home, ".gitconfig")
	put(t, gc, "[credential \"https://github.com\"]\n\thelper = !gh auth git-credential\n")
	if err := genGitAuthor(e); err != nil {
		t.Fatal(err)
	}
	first := snapshot(t, f.home)
	f.log.Reset()
	if err := genGitAuthor(e); err != nil {
		t.Fatal(err)
	}
	if second := snapshot(t, f.home); second != first {
		t.Errorf("2回目で変わった:\n%s\n---\n%s", first, second)
	}
	if f.log.Len() != 0 {
		t.Errorf("2回目のログ: %q", f.log.String())
	}
	if !strings.Contains(read(t, gc), "gh auth git-credential") {
		t.Errorf("credential helper が消えた:\n%s", read(t, gc))
	}
}

// 並びが違う（案件の repo の include が先）なら、セル→案件の順に直す。
func TestGitAuthorReorders(t *testing.T) {
	f, e, repos := gitFixture(t, testOwner)
	gc := filepath.Join(f.home, ".gitconfig")
	product := filepath.Join(f.home, ".config", "andon", "git", "product.gitconfig")
	put(t, gc, "[includeIf \"gitdir:"+e.GhqRoot+"/\"]\n\tpath = "+product+"\n")
	if err := genGitAuthor(e); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, f, repos["product"], "config", "user.email"); got != testOwner {
		t.Errorf("案件の repo の作者: %q\n%s", got, read(t, gc))
	}
	if n := strings.Count(read(t, gc), "product.gitconfig"); n != 1 {
		t.Errorf("product の include が %d 個:\n%s", n, read(t, gc))
	}
}

// 同じ条件（gitdir:<ws>/・gitdir:<GHQ_ROOT>/）に人が置いた include は、並べ直すときも消さない。
func TestGitAuthorKeepsOtherIncludes(t *testing.T) {
	f, e, repos := gitFixture(t, testOwner)
	gc := filepath.Join(f.home, ".gitconfig")
	product := filepath.Join(f.home, ".config", "andon", "git", "product.gitconfig")
	human := filepath.Join(f.home, "human.gitconfig")
	put(t, human, "[alias]\n\tst = status\n")
	// 人の include（両方の条件）と、逆の並びの boot の include
	put(t, gc, "[includeIf \"gitdir:"+f.ws+"/\"]\n\tpath = "+human+"\n"+
		"[includeIf \"gitdir:"+e.GhqRoot+"/\"]\n\tpath = "+human+"\n"+
		"[includeIf \"gitdir:"+e.GhqRoot+"/\"]\n\tpath = "+product+"\n")
	for range 2 {
		if err := genGitAuthor(e); err != nil {
			t.Fatal(err)
		}
	}
	body := read(t, gc)
	if n := strings.Count(body, human); n != 2 {
		t.Errorf("人の include が %d 個（2 のはず）:\n%s", n, body)
	}
	if n := strings.Count(body, product); n != 1 {
		t.Errorf("product の include が %d 個:\n%s", n, body)
	}
	for _, r := range []string{"cell", "product"} {
		if got := gitIn(t, f, repos[r], "config", "alias.st"); got != "status" {
			t.Errorf("%s で人の include が効かない: %q", r, got)
		}
	}
	if got := gitIn(t, f, repos["product"], "config", "user.email"); got != testOwner {
		t.Errorf("案件の repo の作者: %q\n%s", got, body)
	}
	if got := gitIn(t, f, repos["cell"], "config", "user.email"); got != AgentEmail {
		t.Errorf("セルの repo の作者: %q\n%s", got, body)
	}
}

// 人が ~/.gitconfig に書いた user.* は上書きしない（2回打っても）。
func TestGitAuthorKeepsHuman(t *testing.T) {
	f, e, repos := gitFixture(t, testOwner)
	gc := filepath.Join(f.home, ".gitconfig")
	body := "[user]\n\tname = person\n\temail = person@example.com\n"
	put(t, gc, body)
	for range 2 {
		if err := genGitAuthor(e); err != nil {
			t.Fatal(err)
		}
	}
	if got := read(t, gc); got != body {
		t.Errorf("~/.gitconfig が変わった:\n%s", got)
	}
	if got := gitIn(t, f, repos["product"], "config", "user.email"); got != "person@example.com" {
		t.Errorf("作者: %q", got)
	}
	if !strings.Contains(f.log.String(), "触らない") {
		t.Errorf("ログ: %q", f.log.String())
	}
}

// owner が空（andon.toml が無い・owner の行が無い）なら、案件の repo もセルの作者で commit が通る（owner を名乗らない）。
func TestGitAuthorNoOwner(t *testing.T) {
	for name, toml := range map[string]string{"andon.toml が無い": "", "owner が無い": "slug = \"sample\"\n", "owner が空": "owner = \"\"\n"} {
		t.Run(name, func(t *testing.T) {
			f, e, repos := gitFixture(t, "")
			if toml != "" {
				put(t, filepath.Join(f.ws, "andon.toml"), toml)
			}
			if err := genGitAuthor(e); err != nil {
				t.Fatal(err)
			}
			gitIn(t, f, repos["product"], "commit", "-q", "--allow-empty", "-m", "owner なし")
			if got := gitIn(t, f, repos["product"], "log", "-1", "--format=%ae"); got != AgentEmail {
				t.Errorf("作者: %q", got)
			}
			if !strings.Contains(f.log.String(), "owner が無い") {
				t.Errorf("ログ: %q", f.log.String())
			}
		})
	}
}

func TestOwnerName(t *testing.T) {
	for in, want := range map[string]string{
		testOwner:                               "sample-owner",
		"sample-owner@users.noreply.github.com": "sample-owner",
		"someone@example.com":                   "someone",
	} {
		if got := ownerName(in); got != want {
			t.Errorf("ownerName(%q) = %q, want %q", in, got, want)
		}
	}
}
