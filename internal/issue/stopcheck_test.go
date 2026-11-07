package issue

import (
	"bytes"
	"strings"
	"testing"
)

// stop-check の判定（contract.md §9）。台帳は ready と同じ偽の kanban。log は doing に入った行から始める。
// 控え（XDG_STATE_HOME）は同じテストの中で引き継ぐ。
func (f *fixture) stopCheck(log ...map[string]any) string {
	if !f.state {
		f.t.Setenv("XDG_STATE_HOME", f.t.TempDir())
		f.state = true
	}
	f.write("show-T-0001.json", js(map[string]any{"issue": f.task}))
	list := []any{f.task}
	rows := js(map[string]any{"by": "terminal:t9", "id": "T-0001", "change": map[string]any{"status": "doing"}}) + "\n"
	if f.reviewer != nil {
		list = append(list, f.reviewer)
		rows += js(map[string]any{"by": f.by, "id": "T-0002", "change": map[string]any{"add": "review"}}) + "\n"
		if f.findings != "" {
			f.write("kanban/issues/T-0002/library/findings.json", `{"commit": "`+f.findings+`", "findings": []}`)
		}
	}
	for _, r := range log {
		rows += js(r) + "\n"
	}
	f.write("list.json", js(list))
	f.write("kanban/log.jsonl", rows)
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
	return map[string]any{"by": "terminal:t9", "id": "T-0001", "change": map[string]any{"question": n, "status": status}}
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
	// 前回のターンで問いを書いて通した後、答えを待つだけのターン（新しい行は無い）
	if out := f.stopCheck(q(1, "open")); out != "" {
		t.Fatalf("問いを書いたターンを止めた: %s", out)
	}
	var out, errb bytes.Buffer
	stopCheck([]string{"T-0001"}, strings.NewReader(`{}`), &out, &errb)
	if out.String() != "" {
		t.Fatalf("問いが開いたまま待つターンを止めた: %s", out.String())
	}
	// 答えが届いたのに受けずに終わるのは止める（answered の行は担当が返した印ではない）
	if !blocked(f.stopCheck(q(1, "open"), q(1, "answered"))) {
		t.Fatal("答えが届いたのに止めなかった")
	}
}

func TestStopCheckReviewRequested(t *testing.T) {
	f := newFixture(t)
	f.report = `{"commit": "` + f.head + `", "pr": "https://github.com/example/app/pull/7", "review_request": {"round": 1, "limit": 2, "commit": "` + f.head + `", "previous": null, "reviewer": "codex-review", "extra": false}}`
	// レビュアーがまだ起きていない・findings が無い・古い commit を見た: どれも待ち
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
	// 結果が届いたのに報告も ready もせずに終わるのは止める
	f.reviewer = map[string]any{"id": "T-0002", "parent": "T-0001", "status": "review", "profile": "codex-review", "updated": "2026-11-05T00:00:00Z"}
	f.findings = f.head
	if !blocked(f.stopCheck()) {
		t.Fatal("レビューが届いたのに止めなかった")
	}
}
