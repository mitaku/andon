package dispatcher

import (
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/ledger"
)

// ADR-0026 A6: gate が今書く文面のコメント（判定ごと）を、オーナーの手直しに数えない。
// 同じ PR のオーナーのコメントは数える（検査が何も見ていないのではないことの確かめ）。
func TestCorrectionsSkipsGateComments(t *testing.T) {
	head := strings.Repeat("a", 40)
	for _, d := range []string{"approve", "fix", "needs_human"} {
		rec := &gate.Record{Head: head, At: "2026-11-19T00:00:00Z", Tier: "B", TierReasons: []string{"review: on"},
			GitHubChecks: gate.GitHubChecks{State: "success", Names: []string{"test"}}, Review: ledger.Review{State: "current"},
			Decision: d, Reasons: []string{"delegation: merge=none < tier B"}}
		body := gate.CommentBody(rec)
		if !strings.HasPrefix(body, gate.CommentPrefix) || !strings.HasPrefix(gate.StatusDescription(rec), gate.CommentPrefix) {
			t.Fatalf("%s: gate の文の頭が dispatcher の見分ける頭（%q）でない: %q", d, gate.CommentPrefix, body)
		}
		v := &PRView{State: "MERGED"}
		v.Commits = append(v.Commits, struct {
			Oid             string `json:"oid"`
			MessageHeadline string `json:"messageHeadline"`
		}{head, "msg"})
		for _, b := range []string{body, "ここは直して"} {
			v.Comments = append(v.Comments, struct {
				Author struct {
					Login string `json:"login"`
				} `json:"author"`
				Body      string `json:"body"`
				CreatedAt string `json:"createdAt"`
			}{Body: b, CreatedAt: "2026-11-19T01:00:00Z"})
		}
		var in InlineComment
		in.User.Login, in.User.Type, in.Body = "o", "User", body
		v.Inline = []InlineComment{in}
		got := asJSON(t, Corrections(nil, &contract.Report{Commit: head}, "major", v, nil))
		want := `[{"kind":"pr","comments":[{"author":"?","at":"2026-11-19T01:00:00Z","body":"ここは直して"}],"human_commits":[]}]`
		if got != want {
			t.Errorf("%s: gate のコメントをオーナーの手直しに数えた・オーナーのコメントを数えない:\n got %s\nwant %s", d, got, want)
		}
	}
}
