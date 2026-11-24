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

func TestChecksMoved(t *testing.T) {
	files := []string{"docs/a.md", "tests/run", ".github/workflows/go.yml", ".claude/skills/verify-andon/SKILL.md", "internal/x/x_test.go", "internal/x/x.go"}
	want := []string{"tests/run", ".github/workflows/go.yml", ".claude/skills/verify-andon/SKILL.md", "internal/x/x_test.go"}
	if got := ChecksMoved(files); !reflect.DeepEqual(got, want) {
		t.Errorf("ChecksMoved = %v, want %v", got, want)
	}
	if got := ChecksMoved([]string{"docs/a.md"}); got == nil || len(got) != 0 {
		t.Errorf("検査の置き場の無いファイルで %v", got)
	}
}

// TestImpactChecksMoved は、検査の置き場を変えた PR のマージのあと、それを含まない動いている課題だけが影響ありになることを、本物の git で見る。
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
	write("tests/x.sh", "a\n")
	write("doc.md", "a\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	run("checkout", "-q", "-b", "old")
	write("mine.md", "old\n")
	run("add", "-A")
	run("commit", "-q", "-m", "old")
	run("checkout", "-q", "main")
	write("tests/x.sh", "b\n")
	write("doc.md", "b\n")
	run("add", "-A")
	run("commit", "-q", "-m", "merged")
	sha := run("rev-parse", "HEAD")
	run("checkout", "-q", "-b", "fresh")
	write("fresh.md", "fresh\n")
	run("add", "-A")
	run("commit", "-q", "-m", "fresh")
	run("checkout", "-q", "--detach")

	c := &ctx{l: &ledger.Ledger{Dir: filepath.Join(tmp, "kanban")}}
	issue := func(id, branch string) ledger.Issue {
		return ledger.Issue{"id": id, "title": id, "status": "review", "branch": branch}
	}
	moved := []string{"doc.md", "tests/x.sh"}

	old := c.impactOf(issue("A-2", "old"), repo, sha, moved)
	if old.Error != "" || old.Contains || !reflect.DeepEqual(old.ChecksMoved, []string{"tests/x.sh"}) || !old.Affected() {
		t.Errorf("含まない課題が検査の置き場の動きで影響ありにならない: %+v", old)
	}
	if len(old.Overlap) != 0 || old.Conflict || len(old.BriefRefs) != 0 {
		t.Errorf("検査の置き場のほかの事実で影響ありになっている（見たいものを見ていない）: %+v", old)
	}
	fresh := c.impactOf(issue("A-3", "fresh"), repo, sha, moved)
	if fresh.Error != "" || !fresh.Contains || len(fresh.ChecksMoved) != 0 || fresh.Affected() {
		t.Errorf("マージを含む課題が影響ありになった: %+v", fresh)
	}
	other := c.impactOf(issue("A-2", "old"), repo, sha, []string{"doc.md"})
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
