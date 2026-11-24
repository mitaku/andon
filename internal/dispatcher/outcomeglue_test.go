package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/ledger"
)

func TestOutcomeWritesOncePerDone(t *testing.T) {
	head, merge := strings.Repeat("a", 40), strings.Repeat("c", 40)
	tools := newScripted(t, "gh")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ids := []string{"D-0001", "D-0001-r1", "D-0002", "D-0003", "D-0004", "D-0005", "D-0006"}
	var rows []string
	for _, id := range ids {
		rows = append(rows, statusRow(id, "2026-11-19T09:00:00Z", "doing"), statusRow(id, "2026-11-19T09:10:00Z", "done"))
	}
	p := outcomeProject(t, rows...)
	var log strings.Builder
	p.c.Log = &log
	p.l.Slug = "demo"
	p.st = &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}}
	p.tasks = []ledger.Issue{{"id": "D-0001", "profile": "claude"}, {"id": "D-0001-r1", "profile": "codex-review", "parent": "D-0001"},
		{"id": "D-0002", "profile": "claude"}, {"id": "D-0003", "profile": "claude"}, {"id": "D-0004", "profile": "claude"},
		{"id": "D-0005", "who": "person"}, {"id": "D-0006", "profile": "claude"}}
	for id, n := range map[string]string{"D-0001": "7", "D-0003": "9", "D-0004": "8", "D-0006": "6"} {
		writeFile(t, filepath.Join(p.l.Library(id), "report.json"), `{"commit":"`+head+`","pr":"https://github.com/example/app/pull/`+n+`"}`)
	}
	tools.reply("gh pr view 7 --repo example/app --json state,mergeCommit,mergedAt", `{"state":"MERGED","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-11-19T10:00:00Z"}`)
	tools.reply("gh pr view 9 --repo example/app --json state,mergeCommit,mergedAt", `{"state":"OPEN","mergeCommit":null,"mergedAt":null}`)
	tools.reply("gh pr view 8 --repo example/app --json state,mergeCommit,mergedAt", `{"state":"CLOSED","mergeCommit":null,"mergedAt":null}`)
	writeFile(t, filepath.Join(p.l.Library("D-0001"), "gate", head+".json"), `{"head":"`+head+`","at":"2026-11-19T09:30:00Z","tier":"B","decision":"approve","reasons":[],"merged":true}`)
	const s1 = "11111111-0000-4000-8000-000000000001"
	writeFile(t, filepath.Join(p.l.Library("D-0001-r1"), "launches.jsonl"), `{"session":"`+s1+`","harness":{"name":"claude"},"repo":null}`+"\n")
	writeFile(t, filepath.Join(home, ".claude", "projects", "-w", s1+".jsonl"), `{"type":"user","promptId":"p1"}`+"\n"+
		`{"type":"assistant","requestId":"r1","effort":"high","message":{"model":"claude-opus-5-5","usage":{"input_tokens":5,"output_tokens":50}}}`+"\n")
	for _, id := range ids {
		p.st.Outcomes.Add(id)
	}
	p.outcomes()
	read := func(id string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(p.l.Library(id), outcomeFile))
		if err != nil {
			t.Fatalf("%s に outcome.json を書いていない: %v", id, err)
		}
		var v map[string]any
		json.Unmarshal(b, &v)
		return v
	}
	if o := read("D-0001"); o["issue"] != "D-0001" || o["result"] != "merged" || o["pr"] != "https://github.com/example/app/pull/7" || o["merge_commit"] != merge ||
		o["merged_at"] != "2026-11-19T10:00:00Z" || asJSON(t, o["gates"]) != `[{"at":"2026-11-19T09:30:00Z","decision":"approve","head":"`+head+`","merged":true,"reasons":[],"tier":"B"}]` ||
		o["reviewers"] != 1.0 || o["worked_minutes"] != 10.0 || o["sessions"] != nil {
		t.Errorf("マージした課題の outcome.json が違う: %v", o)
	}
	if o := read("D-0001-r1"); o["result"] != "none" || o["pr"] != nil || o["reviewers"] != 0.0 ||
		asJSON(t, o["sessions"]) != `[{"cost":{"efforts":["high"],"models":["claude-opus-5-5"],"tokens":{"cache_read":0,"cache_write":0,"input":5,"output":50},"turns":1},"harness":"claude","session":"`+s1+`"}]` {
		t.Errorf("レビュアーの課題の outcome.json（起動の記録と HOME の下のセッションログから）が違う: %v", o)
	}
	if o := read("D-0002"); o["result"] != "none" || o["pr"] != nil || o["merge_commit"] != nil || o["merged_at"] != nil || asJSON(t, o["gates"]) != `[]` || o["sessions"] != nil {
		t.Errorf("PR の無い課題が none でない: %v", o)
	}
	if o := read("D-0003"); o["result"] != "open" || o["merge_commit"] != nil || o["merged_at"] != nil {
		t.Errorf("開いたままの PR が open でない: %v", o)
	}
	if o := read("D-0004"); o["result"] != "closed" || o["pr"] != "https://github.com/example/app/pull/8" || o["merge_commit"] != nil {
		t.Errorf("閉じられた PR が closed でない: %v", o)
	}
	if _, err := os.Stat(filepath.Join(p.l.Library("D-0005"), outcomeFile)); err == nil {
		t.Error("オーナーの課題に outcome.json を書いた")
	}
	if _, err := os.Stat(filepath.Join(p.l.Library("D-0006"), outcomeFile)); err == nil || asJSON(t, p.st.Outcomes) != `{"D-0006":1}` {
		t.Errorf("gh が落ちたのに書いた・回数を控えていない: %v", p.st.Outcomes)
	}
	before, _ := os.ReadFile(filepath.Join(p.l.Library("D-0003"), outcomeFile))
	p.st.Outcomes.Add("D-0003")
	p.outcomes()
	if after, _ := os.ReadFile(filepath.Join(p.l.Library("D-0003"), outcomeFile)); string(after) != string(before) {
		t.Errorf("同じ done を2度拾って outcome.json を書き直した: %s", after)
	}
	if _, err := os.Stat(filepath.Join(p.l.Library("D-0006"), outcomeFile)); err == nil || p.st.Outcomes != nil {
		t.Errorf("2 回とも落ちたのに書いた・控えに残した: %v", p.st.Outcomes)
	}
	if !strings.Contains(log.String(), "D-0006 の結果を集められない（2 回打って失敗") {
		t.Errorf("諦めたことがログに無い: %s", log.String())
	}
	p.st.Outcomes.Add("D-0006")
	p.st.Outcomes.Fail("D-0006", outcomeTries)
	tools.reply("gh pr view 6 --repo example/app --json state,mergeCommit,mergedAt", `{"state":"OPEN","mergeCommit":null,"mergedAt":null}`)
	p.outcomes()
	if o := read("D-0006"); o["result"] != "open" || p.st.Outcomes != nil {
		t.Errorf("次の見回りで打ち直していない: %v %v", o, p.st.Outcomes)
	}
}
