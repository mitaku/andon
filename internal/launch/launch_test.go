package launch

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/pkgversion"
)

func ptrBool(b bool) *bool { return &b }
func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

func TestDrift(t *testing.T) {
	sha := strings.Repeat("a", 40)
	clean := Checkout{Path: "/w", Commit: &sha, Dirty: ptrBool(false)}
	cases := []struct {
		name    string
		pkg     pkgversion.Info
		ws      Checkout
		want    *bool
		reasons int
	}{
		{"揃っている", pkgversion.Info{Drift: ptrBool(false)}, clean, ptrBool(false), 0},
		{"パッケージが食い違う", pkgversion.Info{Drift: ptrBool(true), Reasons: []string{"中身が違う"}}, clean, ptrBool(true), 1},
		{"セルの repo に変更", pkgversion.Info{Drift: ptrBool(false)}, Checkout{Path: "/w", Commit: &sha, Dirty: ptrBool(true)}, ptrBool(true), 1},
		{"パッケージを確かめられない", pkgversion.Info{Reasons: []string{"version.json が無い"}}, clean, nil, 1},
		{"セルの repo が git でない", pkgversion.Info{Drift: ptrBool(false)}, Checkout{Path: "/w"}, nil, 1},
		{"分からなくても食い違いが1つあれば true", pkgversion.Info{Drift: ptrBool(true), Reasons: []string{"x"}}, Checkout{Path: "/w"}, ptrBool(true), 2},
	}
	for _, c := range cases {
		got, reasons := drift(c.pkg, c.ws)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) || len(reasons) != c.reasons {
			t.Errorf("%s: drift=%v reasons=%q", c.name, got, reasons)
		}
	}
}

func TestSkillOf(t *testing.T) {
	b := []byte("---\nname: yomiyasu\nmetadata:\n    github-path: skills/yomiyasu\n    github-ref: refs/tags/v3.2.1\n    github-repo: https://github.com/example/yomiyasu\n    github-tree-sha: 8f7f66af3beb76e16de7e6a88a64fe9b682cd54c\n    version: 3.2.1\n---\n# body\n")
	x := skillOf("dir", "/s/dir", b)
	if x.Kind != "skill" || x.Name != "yomiyasu" || *x.Source != "https://github.com/example/yomiyasu" || *x.Ref != "refs/tags/v3.2.1" ||
		*x.Version != "3.2.1" || *x.Revision != "8f7f66af3beb76e16de7e6a88a64fe9b682cd54c" {
		t.Fatalf("%+v", x)
	}
	y := skillOf("plain", "/s/plain", []byte("# no frontmatter\n"))
	if y.Name != "plain" || y.Source != nil || y.Version != nil || y.Revision != nil {
		t.Fatalf("バージョンの無い skill: %+v", y)
	}
}

func TestExternals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	agents := filepath.Join(home, ".agents", "skills")
	must(t, os.MkdirAll(filepath.Join(agents, "yomi"), 0o755))
	must(t, os.WriteFile(filepath.Join(agents, "yomi", "SKILL.md"), []byte("---\nname: yomi\nmetadata:\n  version: 1.0.0\n---\n"), 0o644))
	must(t, os.MkdirAll(filepath.Join(agents, "nofile"), 0o755))                                 // SKILL.md の無いディレクトリは数えない
	must(t, os.Symlink(filepath.Join(home, "elsewhere"), filepath.Join(agents, "andon-worker"))) // symlink は数えない
	plug := filepath.Join(home, ".claude", "plugins")
	must(t, os.MkdirAll(plug, 0o755))
	must(t, os.WriteFile(filepath.Join(plug, "installed_plugins.json"), []byte(`{"version":2,"plugins":{"p@m":[{"installPath":"/x","version":"0.1.0","gitCommitSha":"35a90942a7f02e1ebbc2443643179815f2147954"}]}}`), 0o644))

	got, err := externals("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != "plugin" || got[0].Name != "p@m" || *got[0].Version != "0.1.0" ||
		got[1].Kind != "skill" || got[1].Name != "yomi" || *got[1].Version != "1.0.0" {
		b, _ := json.Marshal(got)
		t.Fatalf("claude: %s", b)
	}
	got, err = externals("codex")
	if err != nil || len(got) != 1 || got[0].Kind != "skill" {
		t.Fatalf("codex は plugin を読まない: %+v %v", got, err)
	}
	empty := t.TempDir()
	t.Setenv("HOME", empty)
	if got, err := externals("claude"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("置き場が無いのは空（null でない）: %+v %v", got, err)
	}
}

// Gather と Append: 取れない値は null、取れる値は入る。2回目の Append は1行目を変えない（受け入れ条件 1・2 の土台）。
func TestGatherAppend(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmp, "home"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	bin := filepath.Join(tmp, "bin") // git だけ。hc・claude を引けない（command・harness.version は null）
	must(t, os.MkdirAll(bin, 0o755))
	must(t, os.Symlink(gitPath(t), filepath.Join(bin, "git")))
	t.Setenv("PATH", bin)
	pkg := filepath.Join(tmp, "pkg")
	must(t, os.MkdirAll(filepath.Join(pkg, "skills", "worker"), 0o755))
	must(t, os.WriteFile(filepath.Join(pkg, "skills", "worker", "SKILL.md"), []byte("# worker\n"), 0o644))
	sha := strings.Repeat("b", 40)
	t.Setenv("ANDON_HOME", pkg)
	must(t, pkgversion.Stamp(pkg, sha))
	ws := filepath.Join(tmp, "ws")
	gitInit(t, ws)
	must(t, os.WriteFile(filepath.Join(ws, ".git", "info", "exclude"), []byte("memory/\n"), 0o644)) // セルの repo は memory を git で追わない
	must(t, os.MkdirAll(filepath.Join(ws, "memory"), 0o755))
	must(t, os.WriteFile(filepath.Join(ws, "memory", "MEMORY.md"), []byte("- [a](a.md) — x\n"), 0o644))
	t.Setenv("ANDON_WORKSPACE", ws)
	wt := filepath.Join(tmp, "wt")
	gitInit(t, wt)

	r := Gather(Spec{Role: RoleWorker, Launch: LaunchStart, Issue: "S-0001", Profile: "claude", Template: "claude",
		Session: "0f8fad5b-d9cb-469f-a165-70867728950e", Repo: "github.com/example/app", RepoPath: wt, Brand: "claude"})
	r.Terminal = "t42"
	if r.Command != nil || r.Harness.Version != nil {
		t.Fatalf("取れない値が null でない: command=%v version=%v", r.Command, r.Harness.Version)
	}
	if r.Package.Commit == nil || *r.Package.Commit != sha || r.Package.Drift == nil || *r.Package.Drift {
		t.Fatalf("package: %+v", r.Package)
	}
	if r.Workspace.Commit == nil || r.Repo == nil || r.Repo.Commit == nil || r.Drift == nil || *r.Drift || len(r.DriftReasons) != 0 {
		t.Fatalf("checkout と drift: %+v %+v drift=%v %q", r.Workspace, r.Repo, r.Drift, r.DriftReasons)
	}
	if r.MemoryIndex == nil || *r.MemoryIndex != "- [a](a.md) — x\n" || r.External == nil || len(*r.External) != 0 {
		t.Fatalf("memory_index・external: %v %v", r.MemoryIndex, r.External)
	}
	path := IssuePath(filepath.Join(tmp, "kanban"), "S-0001")
	must(t, Append(path, r))
	first, _ := os.ReadFile(path)

	// パッケージの置き場を書き換えると印が立つ（受け入れ条件 4 の土台）。Codex は memory を読まない
	must(t, os.WriteFile(filepath.Join(pkg, "skills", "worker", "SKILL.md"), []byte("# worker (hand edit)\n"), 0o644))
	r2 := Gather(Spec{Role: RoleWorker, Launch: LaunchResume, Issue: "S-0001", Profile: "codex", Template: "codex-resume", Brand: "codex"})
	r2.Terminal = "t43"
	if r2.Drift == nil || !*r2.Drift || r2.MemoryIndex != nil || r2.Session != nil || r2.Repo != nil {
		t.Fatalf("2件目: drift=%v memory=%v session=%v repo=%v", r2.Drift, r2.MemoryIndex, r2.Session, r2.Repo)
	}
	must(t, Append(path, r2))
	all, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(all), string(first)) {
		t.Fatal("2件目を足したら1件目が変わった")
	}
	sc := bufio.NewScanner(strings.NewReader(string(all)))
	n := 0
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("行 %d が JSON でない: %v", n+1, err)
		}
		for _, k := range []string{"command", "session", "repo", "memory_index", "external"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("行 %d にキー %s が無い（null でも書く）", n+1, k)
			}
		}
		n++
	}
	if n != 2 {
		t.Fatalf("行が %d", n)
	}
	if got := ChiefOfStaffPath("/l/kanban"); got != "/l/kanban/chief-of-staff/launches.jsonl" {
		t.Fatal(got)
	}
}

func gitPath(t *testing.T) string {
	p, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git が無い")
	}
	return p
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}
