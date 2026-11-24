package dispatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func reviewProject(t *testing.T, log *strings.Builder, children ...ledger.Issue) *project {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("GHQ_ROOT", t.TempDir())
	l := ledger.At(t.TempDir())
	l.Slug = "demo"
	os.MkdirAll(l.Dir, 0o755)
	os.WriteFile(filepath.Join(l.Dir, "config.yaml"), []byte("version: 3\nchief_of_staff: t5\nrepos: [github.com/example/app]\n"), 0o644)
	task := ledger.Issue{"id": "D-0001", "status": "doing", "profile": "claude", "repo": "github.com/example/app", "agent": "t7",
		"review": "on", "must_fix": "major", "merge": "person"}
	keys := map[string]any{"review": "on", "must_fix": "major", "merge": "person"}
	return &project{c: &Dispatcher{Log: log}, l: l, st: &State{Reviewers: map[string]Reviewer{}, Pending: []Notification{}},
		tasks: append([]ledger.Issue{task}, children...),
		rows:  []ledger.Row{{ID: "D-0001", By: "terminal:t5", Change: keys}, {ID: "D-0001", By: "terminal:t7", Change: map[string]any{"report": "write"}}}}
}

func writeBrief(t *testing.T, p *project, id, body string) {
	t.Helper()
	dir := filepath.Join(p.l.Dir, "issues", id)
	os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReviewRequestAdoptsWithoutState(t *testing.T) {
	const c = "1111111111111111111111111111111111111111"
	tools := newScripted(t, "hc", "gh")
	var log strings.Builder
	p := reviewProject(t, &log, ledger.Issue{"id": "D-0001-r1", "status": "doing", "profile": "codex-review", "parent": "D-0001", "agent": "t9"})
	writeBrief(t, p, "D-0001-r1", "対象: worktree `x`（detached）。commit "+c+"。差分は")
	rep := &contract.Report{PR: "https://github.com/example/app/pull/7", ReviewRequest: &contract.ReviewRequest{Round: 1, Limit: 2, Commit: c, Reviewer: "codex-review"}}
	p.reviewRequest(p.tasks[0], rep)
	r := p.st.Reviewers["D-0001@"+c]
	if !r.Started || r.ReviewerIssue != "D-0001-r1" || r.Address != "t9" {
		t.Errorf("もういるレビュアーを引き継いで控えていない: %+v", r)
	}
	if !strings.Contains(log.String(), "D-0001 のレビュー（commit 111111111111）は D-0001-r1（t9）がもう受けている") {
		t.Errorf("引き継いだことがログに無い: %s", log.String())
	}
	if got := tools.calls(); hasCall(got, "hc run") || hasCall(got, "gh ") {
		t.Errorf("引き継いだのに起動しに行った: %q", got)
	}
	if len(p.st.Pending) != 0 {
		t.Errorf("引き継いだだけで通知を積んだ: %+v", p.st.Pending)
	}
}

func TestReviewRequestRestartsAfterEmptyReturn(t *testing.T) {
	const c = "1111111111111111111111111111111111111111"
	tools := newScripted(t, "hc", "gh")
	var log strings.Builder
	p := reviewProject(t, &log, ledger.Issue{"id": "D-0001-r1", "status": "review", "profile": "codex-review", "parent": "D-0001", "agent": "t9"})
	writeBrief(t, p, "D-0001-r1", "対象: worktree `x`（detached）。commit "+c+"。差分は")
	key := "D-0001@" + c
	p.st.Reviewers[key] = Reviewer{Issue: "D-0001", Commit: c, Round: 1, ReviewerIssue: "D-0001-r1", Address: "t9", Started: true, Row: 1}
	p.rows = append(p.rows, ledger.Row{ID: "D-0001", By: "terminal:t7", Change: map[string]any{"report": "write"}})
	rep := &contract.Report{PR: "https://github.com/example/app/pull/7", ReviewRequest: &contract.ReviewRequest{Round: 2, Limit: 2, Commit: c, Reviewer: "codex-review"}}
	p.reviewRequest(p.tasks[0], rep)
	r := p.st.Reviewers[key]
	if r.Started || r.Round != 2 || r.ReviewerIssue != "" || r.Address != "" || r.Attempts != 1 || !strings.HasPrefix(r.Failed, "PR の head が読めない") || r.Row != 3 {
		t.Errorf("頼み直した回のレビュアーとして起動しに行っていない（前の子課題を持ったまま・回が古い）: %+v", r)
	}
	if !strings.Contains(log.String(), "D-0001 のレビュアー D-0001-r1 は commit 111111111111 の結果を残さずに返った。担当が頼み直したので、2 回目のレビュアーを起動し直す") {
		t.Errorf("起動し直す理由がログに無い: %s", log.String())
	}
	if got := tools.calls(); !hasCall(got, "gh pr view https://github.com/example/app/pull/7 --json headRefOid") {
		t.Errorf("起動しに行っていない（PR の head を読んでいない）: %q", got)
	}
}
