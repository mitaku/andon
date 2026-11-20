package triage

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 起動する上限は project.json の chief_of_staff.triage_max_issues（contract.md §11）。無ければ 3、信じられなければ止まる。
func TestProjectMaxIssues(t *testing.T) {
	ws := t.TempDir()
	run := func(env []string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = ws
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		c.Env = append(c.Env, env...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	owner := []string{"GIT_AUTHOR_NAME=o", "GIT_AUTHOR_EMAIL=owner@example.com", "GIT_COMMITTER_NAME=o", "GIT_COMMITTER_EMAIL=owner@example.com"}
	other := []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=other@example.com", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=other@example.com"}
	rel := filepath.Join("projects", "demo", "project.json")
	commit := func(env []string, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(ws, "projects", "demo"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		run(env, "add", "-A")
		run(env, "commit", "-q", "-m", "project")
	}
	got := func() (int, error) { return projectMaxIssues(ws, "demo") }

	// セルの repo が無い・project.json が無い → 既定の 3。
	if n, err := got(); err != nil || n != 3 {
		t.Fatalf("repo 無し: %d %v", n, err)
	}
	run(nil, "init", "-q")
	if err := os.WriteFile(filepath.Join(ws, "andon.toml"), []byte("owner = \"owner@example.com\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(owner, "add", "andon.toml")
	run(owner, "commit", "-q", "-m", "toml")
	if n, err := got(); err != nil || n != 3 {
		t.Fatalf("project.json 無し: %d %v", n, err)
	}
	base := `{"version": 1, "project": {"title": "Demo", "prefix": "T"}, "repos": []`
	commit(owner, base+`}`)
	if n, err := got(); err != nil || n != 3 {
		t.Fatalf("chief_of_staff 無し: %d %v", n, err)
	}
	commit(owner, base+`, "chief_of_staff": {"parallel": 2}}`)
	if n, err := got(); err != nil || n != 3 {
		t.Fatalf("triage_max_issues 無し: %d %v", n, err)
	}
	commit(owner, base+`, "chief_of_staff": {"triage_max_issues": 2}}`)
	if n, err := got(); err != nil || n != 2 {
		t.Fatalf("triage_max_issues 2: %d %v", n, err)
	}
	// 形が違う・owner でない人が最後に変えた project.json は信じない（既定にも倒さない）。
	commit(owner, base+`, "chief_of_staff": {"triage_max_issues": 0}}`)
	if _, err := got(); err == nil || !strings.Contains(err.Error(), "--max-issues で渡す") {
		t.Fatalf("triage_max_issues 0 を通した: %v", err)
	}
	commit(other, base+`, "chief_of_staff": {"triage_max_issues": 5}}`)
	if _, err := got(); err == nil || !strings.Contains(err.Error(), "owner でない") {
		t.Fatalf("owner でない author の値を通した: %v", err)
	}
}
