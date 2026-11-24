package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"andon/internal/ledger"
)

func gluedProject(t *testing.T, log *strings.Builder) *project {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	l := ledger.At(t.TempDir())
	l.Slug = "demo"
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\nrepos: [github.com/example/app]\n"), 0o644)
	return &project{c: &Dispatcher{Log: log}, l: l, st: &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}}
}

func writeLib(t *testing.T, p *project, id, name, body string) string {
	t.Helper()
	path := filepath.Join(p.l.Library(id), name)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCorrectionsWritesOncePerDone(t *testing.T) {
	a, f := strings.Repeat("a", 40), strings.Repeat("f", 40)
	tools := newScripted(t, "kanban", "gh")
	var log strings.Builder
	p := gluedProject(t, &log)
	pr := `{"state":"MERGED","commits":[{"oid":"` + a + `","messageHeadline":"docs/a.md"}%s],` +
		`"comments":[{"author":{"login":"plant-bot"},"body":"andon gate: **approve**（commit x）","createdAt":"2026-11-09T00:00:00Z"}],"reviews":[]}`
	for _, c := range []struct{ id, n, extra string }{{"D-0001", "7", ""}, {"D-0002", "9", `,{"oid":"` + f + `","messageHeadline":"オーナーが直した"}`}} {
		writeLib(t, p, c.id, "report.json", `{"commit":"`+a+`","pr":"https://github.com/example/app/pull/`+c.n+`"}`)
		tools.reply("kanban show "+c.id+" --json", `{"issue":{"id":"`+c.id+`","status":"done","profile":"claude"},"questions":[]}`)
		tools.reply("gh pr view "+c.n+" --repo example/app --json state,commits,comments,reviews", strings.Replace(pr, "%s", c.extra, 1))
		tools.reply("gh api --paginate --slurp repos/example/app/pulls/"+c.n+"/comments", `[[]]`)
	}
	tools.reply("gh api repos/example/app/commits/"+f, `{"files":[{"filename":"docs/b.md"}]}`)
	tools.reply("kanban show D-0001-r1 --json", `{"issue":{"id":"D-0001-r1","status":"done","profile":"codex-review","parent":"D-0001"},"questions":[]}`)
	for _, id := range []string{"D-0001", "D-0001-r1", "D-0002"} {
		p.st.Corrections.Add(id)
	}
	p.corrections()
	if p.st.Corrections != nil {
		t.Errorf("書いた課題が控えに残った: %v", p.st.Corrections)
	}
	read := func(id string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), "corrections.json"))
		if err != nil {
			t.Fatalf("%s に corrections.json を書いていない: %v", id, err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	empty := read("D-0001")
	if at, _ := empty["at"].(string); !regexp.MustCompile(`^[0-9-]{10}T[0-9:]{8}Z$`).MatchString(at) || asJSON(t, empty["corrections"]) != `[]` {
		t.Errorf("手直しの無い課題に時刻つきの空の配列を書いていない（gate のコメントを数えた？）: %v", empty)
	}
	if got, want := asJSON(t, read("D-0002")["corrections"]), `[{"comments":[],"human_commits":[{"files":["docs/b.md"],"headline":"オーナーが直した","sha":"`+f+`"}],"kind":"pr"}]`; got != want {
		t.Errorf("担当の最後の報告より後の commit が、gh で読んだファイルつきの pr になっていない:\n got %s\nwant %s", got, want)
	}
	if _, err := os.Stat(filepath.Join(p.l.Library("D-0001-r1"), "corrections.json")); err == nil {
		t.Error("レビュアーの課題に corrections.json を書いた")
	}
	if !strings.Contains(log.String(), "D-0001 のオーナーの手直しを 0 件 corrections.json に書いた") {
		t.Errorf("書いたことがログに無い: %s", log.String())
	}
	before, _ := os.ReadFile(filepath.Join(p.l.Library("D-0002"), "corrections.json"))
	tools.calls()
	p.st.Corrections.Add("D-0002")
	p.corrections()
	after, _ := os.ReadFile(filepath.Join(p.l.Library("D-0002"), "corrections.json"))
	if string(before) != string(after) || p.st.Corrections != nil {
		t.Errorf("同じ done を2度拾って書き直した・控えに残した: %s", after)
	}
	if got := tools.calls(); len(got) != 0 {
		t.Errorf("もう書いた課題で kanban・gh を打った: %q", got)
	}
}
