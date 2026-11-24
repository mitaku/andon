package issue

import (
	"bytes"
	"strings"
	"testing"

	"andon/internal/ledger/ledgertest"
)

// stop-check の判定（contract.md §9）。kanban の CLI は ready と同じ偽物。log は doing に入った行から始める。
// 控え（XDG_STATE_HOME）は同じテストの中で引き継ぐ。
func (f *fixture) stopCheck(log ...map[string]any) string {
	if !f.state {
		f.t.Setenv("XDG_STATE_HOME", f.t.TempDir())
		f.state = true
	}
	f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
	list := []any{f.task}
	rows := []string{js(map[string]any{"at": "2026-11-20T00:00:00Z", "by": "terminal:t9", "id": "T-0001", "change": map[string]any{"status": "doing"}})}
	if f.reviewer != nil {
		list = append(list, f.reviewer)
		rows = append(rows, js(map[string]any{"at": "2026-11-20T00:00:00Z", "by": f.by, "id": "T-0002", "change": map[string]any{"add": "review"}}))
		if f.findings != "" {
			f.write("kanban/issues/T-0002/library/findings.json", `{"commit": "`+f.findings+`", "findings": []}`)
		}
	}
	for _, r := range log {
		rows = append(rows, js(r))
	}
	f.write("list.json", js(list))
	f.write("kanban/log.jsonl", ledgertest.Log(f.t, rows...))
	if f.report != "" {
		f.write("kanban/issues/T-0001/library/report.json", f.report)
	}
	var out, errb bytes.Buffer
	if rc := stopCheck([]string{"T-0001"}, strings.NewReader(`{"stop_hook_active":false}`), &out, &errb); rc != 0 {
		f.t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	return out.String()
}

func blocked(out string) bool { return strings.Contains(out, `"decision":"block"`) }

func q(n int, status string) map[string]any {
	return map[string]any{"at": "2026-11-20T00:00:00Z", "by": "terminal:t9", "id": "T-0001", "change": map[string]any{"question": n, "status": status}}
}

func TestStopCheckBlocksDoingWithoutReport(t *testing.T) {
	f := newFixture(t)
	f.reviewer, f.report = nil, ""
	if !blocked(f.stopCheck()) {
		t.Fatal("doing で報告も問いも無いのに止めなかった")
	}
}

func TestStopCheckPassesReview(t *testing.T) {
	f := newFixture(t)
	f.task["status"] = "review"
	f.reviewer, f.report = nil, ""
	if out := f.stopCheck(); out != "" {
		t.Fatalf("status が review なのに止めた: %s", out)
	}
}

func TestStopCheckOpenQuestion(t *testing.T) {
	f := newFixture(t)
	f.reviewer, f.report = nil, ""
	if out := f.stopCheck(q(1, "open")); out != "" {
		t.Fatalf("問いを書いたターンを止めた: %s", out)
	}
	var out, errb bytes.Buffer
	stopCheck([]string{"T-0001"}, strings.NewReader(`{}`), &out, &errb)
	if out.String() != "" {
		t.Fatalf("問いが開いたまま待つターンを止めた: %s", out.String())
	}
	if !blocked(f.stopCheck(q(1, "open"), q(1, "answered"))) {
		t.Fatal("答えが届いたのに止めなかった")
	}
}

func TestStopCheckReviewRequested(t *testing.T) {
	f := newFixture(t)
	f.report = `{"commit": "` + f.head + `", "pr": "https://github.com/example/app/pull/7", "review_request": {"round": 1, "limit": 2, "commit": "` + f.head + `", "previous": null, "reviewer": "codex-review", "extra": false}}`
	for name, set := range map[string]func(){
		"レビュアー無し":     func() { f.reviewer = nil },
		"findings 無し": func() { f.findings = "" },
		"古い commit":   func() { f.findings = other },
	} {
		f.reviewer = map[string]any{"id": "T-0002", "parent": "T-0001", "status": "review", "profile": "codex-review", "updated": "2026-11-05T00:00:00Z"}
		f.findings = f.head
		set()
		if out := f.stopCheck(); out != "" {
			t.Errorf("%s: レビュー待ちのターンを止めた: %s", name, out)
		}
	}
	f.reviewer = map[string]any{"id": "T-0002", "parent": "T-0001", "status": "review", "profile": "codex-review", "updated": "2026-11-05T00:00:00Z"}
	f.findings = f.head
	if !blocked(f.stopCheck()) {
		t.Fatal("レビューが届いたのに止めなかった")
	}
}

// Chief Engineer の課題（profile chief-engineer）は、報告も問いも無いターンでも止めない（ADR-0009 決めたこと 10）。ほかの profile は今のまま。
func TestStopCheckPassesChiefEngineer(t *testing.T) {
	f := newFixture(t)
	f.reviewer, f.report = nil, ""
	f.task["profile"] = "chief-engineer"
	for i := 0; i < 2; i++ {
		if out := f.stopCheck(); out != "" {
			t.Fatalf("Chief Engineer の課題を止めた（%d 回目）: %s", i+1, out)
		}
	}
	for _, p := range []string{"claude", "codex", "codex-review"} {
		f.task["profile"] = p
		if !blocked(f.stopCheck()) {
			t.Errorf("profile %s の doing の課題を止めなかった", p)
		}
	}
}
