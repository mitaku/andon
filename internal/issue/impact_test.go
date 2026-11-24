package issue

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func TestBriefRefs(t *testing.T) {
	brief := "入力（正）: docs/dispatcher.md の §7 と ADR-0006。skill は chief-of-staff/SKILL.md"
	files := []string{"docs/dispatcher.md", "package/skills/chief-of-staff/SKILL.md", "package/skills/worker/SKILL.md", "internal/issue/issue.go"}
	want := []string{"docs/dispatcher.md", "package/skills/chief-of-staff/SKILL.md", "package/skills/worker/SKILL.md"}
	if got := BriefRefs(brief, files); !reflect.DeepEqual(got, want) {
		t.Errorf("BriefRefs = %v, want %v", got, want)
	}
	if got := BriefRefs("", files); len(got) != 0 || got == nil {
		t.Errorf("空の依頼書で %v", got)
	}
}

func TestIntersect(t *testing.T) {
	got := Intersect([]string{"b", "a", "b", "c"}, []string{"c", "b", "x"})
	if !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Errorf("Intersect = %v", got)
	}
}

// TestImpactChecksMoved は、検査の置き場を変えた PR のマージのあと、それを含まない review の課題だけが影響ありになることを、本物の git で見る。
func TestImpactChecksMoved(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("ANDON_WORKSPACE", tmp)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "a")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "a@example.com")
	}
	repo := filepath.Join(tmp, "repo")
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "init", "-q", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// 条件表: パッケージの gate.json に、strip_prefix の付いた repo を足す（検査の置き場の判定は gate.CheckPlaceFiles）
	pkg := filepath.Join(tmp, "pkg")
	b, err := os.ReadFile(filepath.Join("..", "..", "package", "gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table map[string]any
	if err := json.Unmarshal(b, &table); err != nil {
		t.Fatal(err)
	}
	table["repos"] = map[string]any{"github.com/example/mono": map[string]any{"strip_prefix": "app/"}}
	b, _ = json.Marshal(table)
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "gate.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDON_HOME", pkg)

	const app = "github.com/example/app"
	write("tests/x.sh", "a\n")
	write("tests/y.sh", "a\n")
	write("app/tests/z.sh", "a\n")
	write("doc.md", "a\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "old")
	write("mine.md", "old\n")
	run("add", "-A")
	run("commit", "-q", "-m", "old")
	run("checkout", "-q", "main")
	commit := func(msg string, f func()) string {
		f()
		run("add", "-A")
		run("commit", "-q", "-m", msg)
		return run("rev-parse", "HEAD")
	}
	sha := commit("検査の置き場を変える", func() { write("tests/x.sh", "b\n"); write("doc.md", "b\n") })
	plain := commit("検査の置き場を変えない", func() { write("doc.md", "c\n") })
	moved := commit("検査を置き場の外へ移す", func() { run("mv", "tests/y.sh", "y.sh") })
	mono := commit("strip_prefix の下の検査", func() { write("app/tests/z.sh", "b\n") })
	run("checkout", "-q", "-b", "fresh", sha)
	write("fresh.md", "fresh\n")
	run("add", "-A")
	run("commit", "-q", "-m", "fresh")
	run("checkout", "-q", "--detach")

	places := func(repoName, at string, gh ...string) []string {
		t.Helper()
		got, err := mergedCheckPlaces(repo, repoName, at, gh)
		if err != nil {
			t.Fatalf("mergedCheckPlaces(%s): %v", at, err)
		}
		return got
	}
	checks := places(app, sha, "doc.md", "tests/x.sh")
	for _, x := range []struct {
		name string
		got  []string
		want []string
	}{
		{"検査の置き場を変えた", checks, []string{"tests/x.sh"}},
		{"gh の一覧が切れた", places(app, sha), []string{"tests/x.sh"}},
		{"検査の置き場を変えない", places(app, plain, "doc.md"), []string{}},
		{"rename の元（gh は新しいパスだけ）", places(app, moved, "y.sh"), []string{"tests/y.sh"}},
		{"strip_prefix を外す", places("github.com/example/mono", mono, "app/tests/z.sh"), []string{"tests/z.sh"}},
		{"strip_prefix の無い repo", places(app, mono, "app/tests/z.sh"), []string{}},
	} {
		if x.got == nil || !reflect.DeepEqual(x.got, x.want) {
			t.Errorf("%s: %v, want %v", x.name, x.got, x.want)
		}
	}
	if _, err := mergedCheckPlaces(repo, app, strings.Repeat("0", 40), nil); err == nil {
		t.Error("読めないマージの commit で黙って空を返した")
	}

	c := &ctx{l: &ledger.Ledger{Dir: filepath.Join(tmp, "kanban")}}
	issue := func(id, status, branch string) ledger.Issue {
		return ledger.Issue{"id": id, "title": id, "status": status, "branch": branch}
	}
	merged := []string{"doc.md", "tests/x.sh"}

	old := c.impactOf(issue("A-2", "review", "old"), repo, sha, merged, checks)
	if old.Error != "" || old.Contains || !reflect.DeepEqual(old.ChecksMoved, []string{"tests/x.sh"}) || !old.Affected() {
		t.Errorf("含まない review の課題が検査の置き場の動きで影響ありにならない: %+v", old)
	}
	if len(old.Overlap) != 0 || old.Conflict || len(old.BriefRefs) != 0 {
		t.Errorf("検査の置き場のほかの事実で影響ありになっている（見たいものを見ていない）: %+v", old)
	}
	doing := c.impactOf(issue("A-4", "doing", "old"), repo, sha, merged, checks)
	if doing.Error != "" || !reflect.DeepEqual(doing.ChecksMoved, []string{"tests/x.sh"}) || doing.Affected() {
		t.Errorf("doing の課題は事実を出すが、検査の置き場の動きだけで影響ありにしない（gate が ready のあと返す）: %+v", doing)
	}
	fresh := c.impactOf(issue("A-3", "review", "fresh"), repo, sha, merged, checks)
	if fresh.Error != "" || !fresh.Contains || len(fresh.ChecksMoved) != 0 || fresh.Affected() {
		t.Errorf("マージを含む課題が影響ありになった: %+v", fresh)
	}
	other := c.impactOf(issue("A-2", "review", "old"), repo, plain, []string{"doc.md"}, places(app, plain, "doc.md"))
	if other.Error != "" || other.ChecksMoved == nil || len(other.ChecksMoved) != 0 || other.Affected() {
		t.Errorf("検査の置き場を変えない PR のマージで影響ありになった: %+v", other)
	}
	for _, x := range []struct {
		it   contract.ImpactIssue
		want []any
	}{{old, []any{"tests/x.sh"}}, {other, []any{}}} {
		var m map[string]any
		if err := json.Unmarshal(contract.Marshal(x.it), &m); err != nil || !reflect.DeepEqual(m["checks_moved"], x.want) {
			t.Errorf("出力の checks_moved = %v（%v）, want %v", m["checks_moved"], err, x.want)
		}
	}
}
