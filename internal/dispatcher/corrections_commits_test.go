package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0058 受け入れ条件 4・5: 担当の最後の報告より後の commit のうち、親が 2 つ以上の merge commit は
// human_commits に入れない（parents が無いか配列でなく、親が読めない commit は入れる）。報告より後が merge commit だけでコメントも無ければ pr を書かない。
// Chief Engineer の課題では、報告より後の親 1 つの commit も入れず、gh で commit を読まない。コメントは拾う。
// 担当の課題では、同じ親 1 つの commit が入る（外す条件が何も見ていないのではないことの確かめ）。
func TestCorrectionsDropsMergeAndEngineerCommits(t *testing.T) {
	sha := func(c byte) string { return strings.Repeat(string(c), 40) }
	a, m, b, u, x := sha('a'), sha('e'), sha('b'), sha('c'), sha('d')
	tools := newScripted(t, "kanban", "gh")
	var log strings.Builder
	p := gluedProject(t, &log)
	commits := func(oids ...string) string {
		var s []string
		for _, o := range oids {
			s = append(s, `{"oid":"`+o+`","messageHeadline":"h`+o[:1]+`"}`)
		}
		return `[` + strings.Join(s, ",") + `]`
	}
	note := `{"author":{"login":"o"},"body":"ここは直して","createdAt":"2026-11-23T01:00:00Z"}`
	for _, c := range []struct{ id, profile, n, commits, comments string }{
		{"W-0001", "claude", "1", commits(a, m, b, u, x), ``},
		{"W-0002", "claude", "2", commits(a, m), ``},
		{"E-0001", "chief-engineer", "3", commits(a, b), note},
	} {
		writeLib(t, p, c.id, "report.json", `{"commit":"`+a+`","pr":"https://github.com/example/app/pull/`+c.n+`"}`)
		tools.reply("kanban show "+c.id+" --json", `{"issue":{"id":"`+c.id+`","status":"done","profile":"`+c.profile+`"},"questions":[]}`)
		tools.reply("gh pr view "+c.n+" --repo example/app --json state,commits,comments,reviews",
			`{"state":"MERGED","commits":`+c.commits+`,"comments":[`+c.comments+`],"reviews":[]}`)
		tools.reply("gh api --paginate --slurp repos/example/app/pulls/"+c.n+"/comments", `[[]]`)
		p.st.Corrections.Add(c.id)
	}
	tools.reply("gh api repos/example/app/commits/"+m, `{"parents":[{"sha":"`+a+`"},{"sha":"`+sha('9')+`"}],"files":[{"filename":"docs/adr/README.md"}]}`)
	tools.reply("gh api repos/example/app/commits/"+b, `{"parents":[{"sha":"`+m+`"}],"files":[{"filename":"docs/b.md"}]}`)
	tools.reply("gh api repos/example/app/commits/"+u, `{"files":[{"filename":"docs/c.md"}]}`)
	tools.reply("gh api repos/example/app/commits/"+x, `{"parents":{"sha":"`+u+`"},"files":[{"filename":"docs/d.md"}]}`) // 親が配列でない
	p.corrections()
	if p.st.Corrections != nil {
		t.Fatalf("書いていない課題が控えに残った: %v\n%s", p.st.Corrections, log.String())
	}
	read := func(id string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), "corrections.json"))
		if err != nil {
			t.Fatalf("%s に corrections.json を書いていない: %v", id, err)
		}
		var v struct {
			Corrections any `json:"corrections"`
		}
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		return asJSON(t, v.Corrections)
	}
	if got, want := read("W-0001"), `[{"comments":[],"human_commits":[{"files":["docs/b.md"],"headline":"hb","sha":"`+b+`"},{"files":["docs/c.md"],"headline":"hc","sha":"`+u+`"},{"files":["docs/d.md"],"headline":"hd","sha":"`+x+`"}],"kind":"pr"}]`; got != want {
		t.Errorf("merge commit を外し、親 1 つと親の読めない commit を入れていない:\n got %s\nwant %s", got, want)
	}
	if got := read("W-0002"); got != `[]` {
		t.Errorf("報告より後が merge commit だけでコメントも無いのに pr を書いた: %s", got)
	}
	if got, want := read("E-0001"), `[{"comments":[{"at":"2026-11-23T01:00:00Z","author":"o","body":"ここは直して"}],"human_commits":[],"kind":"pr"}]`; got != want {
		t.Errorf("Chief Engineer の課題で commit を数えた・コメントを拾わない:\n got %s\nwant %s", got, want)
	}
	n := 0
	for _, c := range tools.calls() {
		if c == "gh api repos/example/app/commits/"+b {
			n++
		}
	}
	if n != 1 {
		t.Errorf("commit %s を %d 回読んだ（担当の課題の 1 回だけのはず。Chief Engineer の課題では読まない）", b[:7], n)
	}
}
