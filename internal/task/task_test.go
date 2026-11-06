package task

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ready の照合は偽の kanban と gh・本物の git で見る（振る舞いの全体は tests/andon-task.sh）。
// 偽の kanban は $FAKE_ROOT の show-<id>.json・list.json を返し、review は reviewed-<id> を置くだけ。

const fakeDeck = `#!/bin/sh
case "$1" in
  root) echo "$FAKE_ROOT" ;;
  show) cat "$FAKE_ROOT/show-$2.json" ;;
  list) cat "$FAKE_ROOT/list.json" ;;
  review) touch "$FAKE_ROOT/reviewed-$2"; echo "$2 updated" ;;
  *) exit 9 ;;
esac
`

const fakeGH = `#!/bin/sh
cat "$FAKE_ROOT/prhead"
`

type fixture struct {
	t        *testing.T
	root     string
	wt       string
	head     string
	task     map[string]any
	reviewer map[string]any // nil ならレビュアーのタスクは無い
	by       string         // レビュアーのタスクを足した端末
	report   string         // report.json の中身（空なら置かない）
	findings string         // レビュアーの findings.json の commit（空なら置かない）
	prHead   string
	state    bool // stop-check の控えの置き場を作ったか
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	for _, c := range []string{"git", "sh"} {
		if _, err := exec.LookPath(c); err != nil {
			t.Skipf("%s が無い", c)
		}
	}
	dir := t.TempDir()
	f := &fixture{t: t, root: filepath.Join(dir, "proj"), wt: filepath.Join(dir, "wt"), by: "terminal:t5"}
	bin := filepath.Join(dir, "bin")
	for _, d := range []string{bin, filepath.Join(f.root, "kanban"), f.wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(bin, "kanban"), []byte(fakeDeck), 0o755)
	os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755)
	os.WriteFile(filepath.Join(f.root, "kanban", "config.yaml"), []byte("version: 2\nchief_of_staff: t5\n"), 0o644)
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", f.wt}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "x")
	f.head = git("rev-parse", "HEAD")
	f.prHead = f.head
	f.task = map[string]any{"id": "T-0001", "status": "doing", "profile": "claude", "review": "on", "must_fix": "major", "worktree": f.wt}
	f.reviewer = map[string]any{"id": "T-0002", "parent": "T-0001", "status": "review", "profile": "codex-review", "updated": "2026-11-05T00:00:00Z"}
	f.report = `{"commit": "` + f.head + `", "pr": "https://github.com/example/app/pull/7"}`
	f.findings = f.head
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ROOT", f.root)
	t.Setenv("KANBAN_ROOT", "")
	return f
}

func (f *fixture) write(rel, s string) {
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func js(v any) string { b, _ := json.Marshal(v); return string(b) }

// ready を打ち、終了コードと stderr と review にしたかを返す。
func (f *fixture) ready() (int, string, bool) {
	f.write("show-T-0001.json", js(map[string]any{"task": f.task}))
	list := []any{f.task}
	log := ""
	if f.reviewer != nil {
		list = append(list, f.reviewer)
		log = js(map[string]any{"by": f.by, "id": "T-0002", "change": map[string]any{"add": "review"}}) + "\n"
		if f.findings != "" {
			f.write("kanban/tasks/T-0002/library/findings.json", `{"commit": "`+f.findings+`", "findings": []}`)
		}
	}
	f.write("list.json", js(list))
	f.write("kanban/log.jsonl", log)
	if f.report != "" {
		f.write("kanban/tasks/T-0001/library/report.json", f.report)
	}
	f.write("prhead", f.prHead+"\n")
	var out, errb bytes.Buffer
	rc := Main([]string{"ready", "T-0001"}, strings.NewReader(""), &out, &errb)
	_, err := os.Stat(filepath.Join(f.root, "reviewed-T-0001"))
	return rc, errb.String(), err == nil
}

const other = "0000000000000000000000000000000000000001"

func TestReady(t *testing.T) {
	cases := []struct {
		name string
		mod  func(*fixture)
		want string // 空なら通る
	}{
		{"揃えば review にする", func(*fixture) {}, ""},
		{"report.json が無い", func(f *fixture) { f.report = "" }, "report.json が無い"},
		{"report.json に知らないキー", func(f *fixture) { f.report = strings.Replace(f.report, `"pr"`, `"x": 1, "pr"`, 1) }, "形が違う"},
		{"report.json の pr が null", func(f *fixture) { f.report = `{"commit": "` + f.head + `", "pr": null}` }, "形が違う"},
		{"report.json に pr が無い", func(f *fixture) { f.report = `{"commit": "` + f.head + `"}` }, "pr が無い"},
		{"HEAD 不一致", func(f *fixture) { f.report = strings.Replace(f.report, f.head, other, 1) }, "HEAD"},
		{"PR の head が違う", func(f *fixture) { f.prHead = other }, "PR の head"},
		{"レビューが古い commit（stale）", func(f *fixture) { f.findings = other }, "stale"},
		{"レビューが無い（missing）", func(f *fixture) { f.reviewer = nil }, "missing"},
		{"findings.json が無い", func(f *fixture) { f.findings = "" }, "missing"},
		{"Chief of Staff が足していないレビュー", func(f *fixture) { f.by = "terminal:t7" }, "数えない"},
		{"同じブランドのレビュアー", func(f *fixture) { f.reviewer["profile"] = "claude-review" }, "missing"},
		{"レビュー中（review でも done でもない）", func(f *fixture) { f.reviewer["status"] = "doing" }, "missing"},
		{"review: off ならレビューを見ない", func(f *fixture) { f.task["review"] = "off"; f.reviewer = nil }, ""},
		{"review が無ければ止まる", func(f *fixture) { delete(f.task, "review") }, "on/off でない"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			c.mod(f)
			rc, errs, reviewed := f.ready()
			if c.want == "" {
				if rc != 0 || !reviewed {
					t.Fatalf("rc=%d reviewed=%v err=%q", rc, reviewed, errs)
				}
				return
			}
			if rc != 1 || reviewed || !strings.Contains(errs, c.want) {
				t.Fatalf("want rc 1 with %q, got rc=%d reviewed=%v err=%q", c.want, rc, reviewed, errs)
			}
		})
	}
}
