package triage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"andon/internal/signal"
)

// 項目が上限の既定（50）を超えるとき、collect は memory と signal を先頭に寄せる（その中と残りは古い順のまま）。
// 超えないときは古い順のまま。
func TestCollectPutsMemoryAndSignalFirstOverLimit(t *testing.T) {
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	daysAgo := func(n int) time.Time { return now.Add(-time.Duration(n) * 24 * time.Hour) }
	collect := func(issues int) []string {
		t.Helper()
		dir := t.TempDir()
		// hc を見つけない（端末一覧をセルの本物から読まない）。
		t.Setenv("PATH", filepath.Join(dir, "nobin"))
		var list []string
		for i := 1; i <= issues; i++ { // memory と signal より古い、担当の無い todo
			list = append(list, fmt.Sprintf(`{"id": "T-%d", "title": "課題 %d", "status": "todo", "created": "%s"}`,
				i, i, daysAgo(400-i).Format("2006-01-02T15:04:05Z")))
		}
		listPath := filepath.Join(dir, "list.json")
		if err := os.WriteFile(listPath, []byte("["+strings.Join(list, ",")+"]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cli := filepath.Join(dir, "kanban")
		script := "#!/bin/sh\nwhile IFS= read -r ln; do printf '%s\\n' \"$ln\"; done < '" + listPath + "'\n"
		if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		mem := filepath.Join(dir, "memory")
		if err := os.Mkdir(mem, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, age := range map[string]int{"older.md": 5, "newer.md": 2} {
			path := filepath.Join(mem, name)
			if err := os.WriteFile(path, []byte(name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, daysAgo(age), daysAgo(age)); err != nil {
				t.Fatal(err)
			}
		}
		sig := func(key string, age int) signal.Record {
			return signal.Record{Source: "github-issues", Project: "demo", Key: key, URL: "https://example.com/" + key, Title: key,
				Weight: "later", Fingerprint: signal.Fingerprint("github-issues", key), FirstSeen: daysAgo(age).Format("2006-01-02T15:04:05Z"),
				LastSeen: now.Format("2006-01-02T15:04:05Z"), Count: 1, Active: true, Episodes: 1}
		}
		l := &ledger{cli: cli, root: dir, slug: "demo", dir: filepath.Join(dir, "kanban.d"), memory: mem, now: now}
		items, err := l.listFrom(nil, []signal.Record{sig("github.com/example/app#1", 10), sig("github.com/example/app#2", 3)})
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, it := range items {
			name := it.From
			if it.Kind == "signal" {
				name = it.Text
			}
			got = append(got, it.Kind+":"+name)
		}
		return got
	}
	first := []string{"signal:github.com/example/app#1", "memory:older", "signal:github.com/example/app#2", "memory:newer"}
	todo := func(n int) []string {
		var got []string
		for i := 1; i <= n; i++ {
			got = append(got, fmt.Sprintf("issue:T-%d", i))
		}
		return got
	}

	// 51 件: 上限を超える。memory と signal が先頭（その中は古い順）、残りの課題も古い順。
	if got, want := collect(47), slices.Concat(first, todo(47)); !slices.Equal(got, want) {
		t.Errorf("上限を超える日の並び:\n got  %v\n want %v", got, want)
	}
	// 50 件: 上限を超えない。古い順のまま（memory と signal は新しいので後ろ）。
	if got, want := collect(46), slices.Concat(todo(46), first); !slices.Equal(got, want) {
		t.Errorf("上限を超えない日の並び:\n got  %v\n want %v", got, want)
	}
}

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
