package courier

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func TestRetry(t *testing.T) {
	for _, c := range []struct {
		name  string
		prev  Reviewer
		seen  bool
		limit int
		want  bool
	}{
		{"初めて", Reviewer{}, false, 2, true},
		{"起こした（二重に起こさない）", Reviewer{Started: true, Failed: ""}, true, 2, false},
		{"起こしそこねた（続きから）", Reviewer{Failed: "fetch", ReviewerTask: "D-2"}, true, 2, true},
		{"初めて見た台帳", Reviewer{Skipped: "初めて見た台帳"}, true, 2, false},
		{"設定で起こさない", Reviewer{Skipped: "review が on でない"}, true, 5, false},
		{"上限を超えた・上限そのまま", Reviewer{Skipped: "追加の回", OverLimit: 1}, true, 1, false},
		{"上限を超えた・上限が上がった", Reviewer{Skipped: "追加の回", OverLimit: 1}, true, 2, true},
	} {
		if got := Retry(c.prev, c.seen, c.limit); got != c.want {
			t.Errorf("%s: Retry = %v, want %v", c.name, got, c.want)
		}
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

func TestStatePushReplacesSameEvent(t *testing.T) {
	s := &State{Pending: []Notification{}}
	s.Push(Notification{Task: "D-1", Kind: "gate", Detail: map[string]any{"decision": "fix"}})
	s.Push(Notification{Task: "D-2", Kind: "gate"})
	s.Push(Notification{Task: "D-1", Kind: "review-requested"})
	s.Push(Notification{Task: "D-1", Kind: "gate", Detail: map[string]any{"decision": "approve"}})
	if len(s.Pending) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(s.Pending), s.Pending)
	}
	last := s.Pending[2]
	if last.Task != "D-1" || last.Kind != "gate" || last.Detail["decision"] != "approve" {
		t.Errorf("同じ task と kind が新しい1件に置き換わっていない: %+v", s.Pending)
	}
}

func TestStateLoadSave(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st, first, err := LoadState("demo")
	if err != nil || !first || st.Reviewers == nil || st.Gates == nil || st.Pending == nil {
		t.Fatalf("初めて: first=%v err=%v st=%+v", first, err, st)
	}
	st.Reviewers["D-1@abc"] = Reviewer{Task: "D-1", Commit: "abc", Failed: "fetch", Attempts: 1, ReviewerTask: "D-2", Address: "t9"}
	st.Gates["D-1#3"] = Gate{Task: "D-1", Decision: "approve"}
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
	task := ledger.Task{"id": "D-1", "agent": "t7"}
	b := reviewerBrief(l, task, rep, Reviewer{Task: "D-1", Round: 2, Worktree: "/w/demo-D-1-r2"}, "example/app", "7")
	for _, want := range []string{
		"hc send t7 ", "Chief of Staff・クーリエへは送らない", "宛先はこの1つだけ",
		"commit " + strings.Repeat("b", 40), "git diff " + prev + ".." + strings.Repeat("b", 40), "/w/demo-D-1-r2",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("依頼書に %q が無い:\n%s", want, b)
		}
	}
	if b2 := reviewerBrief(l, ledger.Task{"id": "D-1"}, rep, Reviewer{Task: "D-1", Round: 2}, "example/app", "7"); strings.Contains(b2, "hc send") || !strings.Contains(b2, "（まだ無い）") {
		t.Errorf("担当がいないのに宛先を書いた:\n%s", b2)
	}
}

// 担当の端末のラベル herdr.space・herdr.tab をレビュアーに写す（ADR-0002 決めたこと 12・受け入れ条件 3）。読めなければ付けない。
func TestWorkerPlace(t *testing.T) {
	bin := t.TempDir()
	hc := "#!/bin/sh\n[ \"$1 $3\" = 'get --json' ] || exit 9\ncase $2 in\n" +
		"  t7) echo '{\"address\":\"t7\",\"labels\":{\"task\":\"D-1\",\"herdr.space\":\"andon-demo-work\",\"herdr.tab\":\"andon-demo-D-1\",\"where\":\"x / y\"}}' ;;\n" +
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
		{"t7", []string{"herdr.space=andon-demo-work", "herdr.tab=andon-demo-D-1"}}, // where は写さない
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
