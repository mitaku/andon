package dispatcher

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/gate"
	"andon/internal/ledger"
)

func TestRetry(t *testing.T) {
	for _, c := range []struct {
		name  string
		prev  Reviewer
		seen  bool
		limit int
		row   int
		want  bool
	}{
		{"初めて", Reviewer{}, false, 2, 5, true},
		{"起こした（二重に起こさない）", Reviewer{Started: true, Row: 5}, true, 2, 5, false},
		{"起こした回を同じ commit で頼み直した（二重に起こさない）", Reviewer{Started: true, Row: 5}, true, 2, 9, false},
		{"起こしそこねた（続きから）", Reviewer{Failed: "fetch", ReviewerIssue: "D-2", Row: 5}, true, 2, 5, true},
		{"設定で起こさない", Reviewer{Skipped: "review が on でない", Row: 5}, true, 5, 5, false},
		{"起こさなかった回を同じ commit で頼み直した", Reviewer{Skipped: "review が on でない", Row: 5}, true, 5, 9, true},
		{"前の版の控え（初めて見た台帳で控えただけ）", Reviewer{Skipped: "初めて見た台帳"}, true, 2, 5, true},
		{"上限を超えた・上限そのまま", Reviewer{Skipped: "追加の回", OverLimit: 1, Row: 5}, true, 1, 5, false},
		{"上限を超えた・上限が上がった", Reviewer{Skipped: "追加の回", OverLimit: 1, Row: 5}, true, 2, 5, true},
	} {
		if got := Retry(c.prev, c.seen, c.limit, c.row); got != c.want {
			t.Errorf("%s: Retry = %v, want %v", c.name, got, c.want)
		}
	}
}

// 控えに無い review_request（初めて見た台帳）は、台帳にその commit のレビュアーの子課題が無ければ起こしに行き、
// あれば引き継ぐ（todo でなければもう受けている。todo なら端末を照らし、live なら address を引き継ぐ）。
func TestAdopt(t *testing.T) {
	const c1, c2 = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	briefs := map[string]string{
		"D-2": "対象: worktree `x`（detached）。commit " + c1 + "。差分は",
		"D-3": "対象: worktree `y`（detached）。commit " + c2 + "。差分は",
	}
	brief := func(rev string) string { return briefs[rev] }
	tasks := []ledger.Issue{
		{"id": "D-1", "status": "doing", "profile": "claude"},
		{"id": "D-2", "status": "done", "profile": "codex-review", "parent": "D-1", "agent": "t9"},
		{"id": "D-3", "status": "todo", "profile": "codex-review", "parent": "D-1"},
		{"id": "D-4", "status": "doing", "profile": "claude", "parent": "D-1"}, // レビュアーでない子
	}
	none := func(string) (string, int, bool) { return "", 0, false }
	if _, ok := Adopt(tasks, "D-1", "3333333333333333333333333333333333333333", brief, none); ok {
		t.Error("子課題の無い commit を引き継いだ（起こしに行かない）")
	}
	if r, ok := Adopt(tasks, "D-1", c1, brief, none); !ok || !r.Started || r.ReviewerIssue != "D-2" || r.Address != "t9" {
		t.Errorf("受けた子課題: %+v %v（もう受けているはず）", r, ok)
	}
	if r, ok := Adopt(tasks, "D-1", c2, brief, none); !ok || r.Started || r.ReviewerIssue != "D-3" || r.Address != "" {
		t.Errorf("todo で端末も無い子課題: %+v %v（起こすところから続けるはず）", r, ok)
	}
	// todo だが端末が live（hc run は通ったが kanban start の前に落ちた）: address を引き継ぎ、hc run し直さない
	asked := ""
	fresh := func(rev string) (string, int, bool) { asked = rev; return "t12", 0, true }
	if r, ok := Adopt(tasks, "D-1", c2, brief, fresh); !ok || r.Started || r.Address != "t12" || asked != "D-3" {
		t.Errorf("todo で端末が live（まだ何も受けていない）: %+v %v %q（address を引き継ぎ、最初のメッセージから）", r, ok, asked)
	}
	busy := func(string) (string, int, bool) { return "t12", 2, true }
	if r, ok := Adopt(tasks, "D-1", c2, brief, busy); !ok || !r.Started || r.Address != "t12" {
		t.Errorf("todo で端末が live（もう受けている）: %+v %v（二重に送らないはず）", r, ok)
	}
	if _, ok := Adopt(tasks, "D-9", c1, brief, none); ok {
		t.Error("ほかの課題の子を引き継いだ")
	}
}

func TestLastReport(t *testing.T) {
	rows := []ledger.Row{
		{ID: "D-1", Change: map[string]any{"report": "write"}},
		{ID: "D-2", Change: map[string]any{"report": "write"}},
		{ID: "D-1", Change: map[string]any{"status": "doing"}},
		{ID: "D-1", Change: map[string]any{"report": "write"}},
	}
	if got := lastReport(rows, "D-1"); got != 4 {
		t.Errorf("D-1: %d", got)
	}
	if got := lastReport(rows, "D-9"); got != 0 {
		t.Errorf("D-9: %d", got)
	}
}

func TestAuthorized(t *testing.T) {
	auto := ledger.Settings{Review: "on", MustFix: "major", Reviewer: "codex-review", ReviewRounds: "2", Merge: "auto"}
	person := auto
	person.Merge = "person"
	minor := auto
	minor.MustFix = "minor"
	for _, c := range []struct {
		name                string
		before, now         ledger.Settings
		status              string
		trusted             bool
		gateMerge, nowMerge string
		tier                string
		want                bool
		why                 string
	}{
		{"変わっていない", auto, auto, "review", true, "B", "B", "B", true, ""},
		{"merge: person に変わった", auto, person, "review", true, "B", "B", "B", false, "予約キー"},
		{"must_fix が変わった", auto, minor, "review", true, "B", "B", "B", false, "予約キー"},
		{"Chief of Staff でない者が書いた", auto, auto, "review", false, "B", "B", "B", false, "Chief of Staff"},
		{"review でなくなった", auto, auto, "doing", true, "B", "B", "B", false, "review でなくなった"},
		{"委譲を取り消した", auto, auto, "review", true, "B", "none", "B", false, "B → none"},
		{"委譲を広げた（判定とずれる）", auto, auto, "review", true, "A", "B", "A", false, "A → B"},
		{"委譲の外", auto, auto, "review", true, "A", "A", "B", false, "merge=A < tier B"},
	} {
		got, why := Authorized(c.before, c.now, c.status, c.trusted, c.gateMerge, c.nowMerge, c.tier)
		if got != c.want || !strings.Contains(why, c.why) {
			t.Errorf("%s: Authorized = %v %q, want %v（%q を含む）", c.name, got, why, c.want, c.why)
		}
	}
}

func TestChecksStill(t *testing.T) {
	h1, h2 := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ok := gate.GitHubChecks{State: "success", Names: []string{}}
	for _, c := range []struct {
		name     string
		now      string
		checks   gate.GitHubChecks
		optional bool
		want     bool
		why      string
	}{
		{"変わっていない", h1, ok, false, true, ""},
		{"ゲートのあとに push された", h2, ok, false, false, "head がゲートのあとに動いた"},
		{"checks が走り直した", h1, gate.GitHubChecks{State: "pending", Names: []string{"test"}}, false, false, "pending: test"},
		{"checks が落ちた", h1, gate.GitHubChecks{State: "failure", Names: []string{"image"}}, false, false, "failure: image"},
		{"checks が消えた（要る）", h1, gate.GitHubChecks{State: "none"}, false, false, "none"},
		{"checks が無い（要らない repo）", h1, gate.GitHubChecks{State: "none"}, true, true, ""},
	} {
		got, why := ChecksStill(h1, c.now, c.checks, c.optional)
		if got != c.want || !strings.Contains(why, c.why) {
			t.Errorf("%s: ChecksStill = %v %q, want %v（%q を含む）", c.name, got, why, c.want, c.why)
		}
	}
}

func TestStatePushReplacesSameEvent(t *testing.T) {
	s := &State{Pending: []Notification{}}
	s.Push(Notification{Issue: "D-1", Kind: "gate", Detail: map[string]any{"decision": "fix"}})
	s.Push(Notification{Issue: "D-2", Kind: "gate"})
	s.Push(Notification{Issue: "D-1", Kind: "review-requested"})
	s.Push(Notification{Issue: "D-1", Kind: "gate", Detail: map[string]any{"decision": "approve"}})
	if len(s.Pending) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(s.Pending), s.Pending)
	}
	last := s.Pending[2]
	if last.Issue != "D-1" || last.Kind != "gate" || last.Detail["decision"] != "approve" {
		t.Errorf("同じ task と kind が新しい1件に置き換わっていない: %+v", s.Pending)
	}
}

func TestStateLoadSave(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st, first, err := LoadState("demo")
	if err != nil || !first || st.Reviewers == nil || st.Gates == nil || st.Pending == nil {
		t.Fatalf("初めて: first=%v err=%v st=%+v", first, err, st)
	}
	st.Reviewers["D-1@abc"] = Reviewer{Issue: "D-1", Commit: "abc", Failed: "fetch", Attempts: 1, ReviewerIssue: "D-2", Address: "t9"}
	st.Gates["D-1#3"] = Gate{Issue: "D-1", Decision: "approve"}
	if err := st.Save("demo"); err != nil {
		t.Fatal(err)
	}
	back, first, err := LoadState("demo")
	if err != nil || first {
		t.Fatalf("2回目: first=%v err=%v", first, err)
	}
	if r := back.Reviewers["D-1@abc"]; r.Failed != "fetch" || r.Attempts != 1 || r.Address != "t9" || r.Started {
		t.Errorf("控えが戻らない: %+v", r)
	}
	if _, err := os.Stat(StatePath("demo") + ".tmp"); err == nil {
		t.Error("書きかけの .tmp が残った")
	}
	os.WriteFile(StatePath("demo"), []byte("{"), 0o644)
	if _, _, err := LoadState("demo"); err == nil {
		t.Error("壊れた控えを読めたことにした")
	}
}

func TestReviewerBriefAddressesTheWorker(t *testing.T) {
	l := ledger.At(filepath.Join(t.TempDir(), "demo"))
	prev := strings.Repeat("a", 40)
	rep := &contract.Report{PR: "https://github.com/example/app/pull/7",
		ReviewRequest: &contract.ReviewRequest{Round: 2, Limit: 2, Commit: strings.Repeat("b", 40), Previous: &prev, Reviewer: "codex-review"}}
	task := ledger.Issue{"id": "D-1", "agent": "t7"}
	b := reviewerBrief(l, task, rep, Reviewer{Issue: "D-1", Round: 2, Worktree: "/w/demo-D-1-r2"}, "example/app", "7")
	for _, want := range []string{
		"hc send t7 ", "Chief of Staff・dispatcher へは送らない", "宛先はこの1つだけ",
		"commit " + strings.Repeat("b", 40), "git diff " + prev + ".." + strings.Repeat("b", 40), "/w/demo-D-1-r2",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("依頼書に %q が無い:\n%s", want, b)
		}
	}
	if b2 := reviewerBrief(l, ledger.Issue{"id": "D-1"}, rep, Reviewer{Issue: "D-1", Round: 2}, "example/app", "7"); strings.Contains(b2, "hc send") || !strings.Contains(b2, "（まだ無い）") {
		t.Errorf("担当がいないのに宛先を書いた:\n%s", b2)
	}
}

// 担当の端末のラベル herdr.space・herdr.tab をレビュアーに写す（ADR-0002 決めたこと 12・受け入れ条件 3。ADR-0003 で Space も課題ごと）。読めなければ付けない。
func TestWorkerPlace(t *testing.T) {
	bin := t.TempDir()
	hc := "#!/bin/sh\n[ \"$1 $3\" = 'get --json' ] || exit 9\ncase $2 in\n" +
		"  t7) echo '{\"address\":\"t7\",\"labels\":{\"task\":\"D-1\",\"herdr.space\":\"andon-demo-D-1\",\"herdr.tab\":\"andon-demo-D-1\",\"where\":\"x / y\"}}' ;;\n" +
		"  t8) echo '{\"address\":\"t8\",\"labels\":{\"task\":\"D-2\"}}' ;;\n" +
		"  t9) echo '{\"address\":\"t9\"' ;;\n" +
		"  *) echo '{\"error\":{\"kind\":\"not_found\"}}' >&2; exit 3 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "hc"), []byte(hc), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, c := range []struct {
		agent string
		want  []string
	}{
		{"t7", []string{"herdr.space=andon-demo-D-1", "herdr.tab=andon-demo-D-1"}}, // where は写さない
		{"t8", nil},  // ラベルが無い
		{"t9", nil},  // 返事が読めない
		{"t10", nil}, // 端末が無い
		{"", nil},    // agent が無い
	} {
		if got := workerPlace(c.agent); !slices.Equal(got, c.want) {
			t.Errorf("%q: got %q want %q", c.agent, got, c.want)
		}
	}
}

// 引き継ぐ detached worktree は、その repo の worktree list に detached で載り HEAD が commit のものだけ（置き場にあるだけでは信じない）。
func TestDetachedAt(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GHQ_ROOT", root)
	dir := filepath.Join(root, "github.com", "example", "app")
	git := func(args ...string) string {
		t.Helper()
		out, _, err := run("", []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=x@example.com", "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=x@example.com"}, "", "git", args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	git("init", "-q", dir)
	git("-C", dir, "commit", "-q", "--allow-empty", "-m", "a")
	c1 := git("-C", dir, "rev-parse", "HEAD")
	git("-C", dir, "commit", "-q", "--allow-empty", "-m", "b")
	wt := filepath.Join(root, "wt", "sample-D-1-r1")
	git("-C", dir, "worktree", "add", "-q", "--detach", wt, c1)
	if !detachedAt("github.com/example/app", wt, c1) {
		t.Error("その commit の detached worktree を引き継がない")
	}
	if detachedAt("github.com/example/app", wt, git("-C", dir, "rev-parse", "HEAD")) {
		t.Error("HEAD が違う worktree を引き継いだ")
	}
	plain := filepath.Join(root, "wt", "sample-D-1-r2")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if detachedAt("github.com/example/app", plain, c1) {
		t.Error("worktree でない置き場を引き継いだ")
	}
	link := filepath.Join(root, "wt", "sample-D-1-r3")
	if err := os.Symlink(wt, link); err != nil {
		t.Fatal(err)
	}
	if detachedAt("github.com/example/app", link, c1) {
		t.Error("symlink を引き継いだ")
	}
}
