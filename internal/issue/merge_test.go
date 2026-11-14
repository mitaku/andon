package issue

import (
	"strings"
	"testing"

	"andon/internal/gate"
	"andon/internal/ledger"
)

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

// gh pr merge が失敗したあと、ゲートの判定した head のまま MERGED なら人が先にマージしたとして問いにしない。
// head が違う・MERGED でない・返事が読めないときは今までどおり問いにする（dispatcher.md §7.2 #5）。
func TestMergedBefore(t *testing.T) {
	head, other, merge := "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	for _, c := range []struct {
		name, view string
		want       bool
	}{
		{"同じ head で MERGED", `{"state":"MERGED","headRefOid":"` + head + `","mergeCommit":{"oid":"` + merge + `"}}`, true},
		{"違う head で MERGED", `{"state":"MERGED","headRefOid":"` + other + `","mergeCommit":{"oid":"` + merge + `"}}`, false},
		{"OPEN のまま", `{"state":"OPEN","headRefOid":"` + head + `","mergeCommit":null}`, false},
		{"CLOSED", `{"state":"CLOSED","headRefOid":"` + head + `","mergeCommit":null}`, false},
		{"マージの commit が無い", `{"state":"MERGED","headRefOid":"` + head + `","mergeCommit":null}`, false},
		{"head が無い", `{"state":"MERGED","mergeCommit":{"oid":"` + merge + `"}}`, false},
		{"読めない", `HTTP 502`, false},
	} {
		sha, ok := MergedBefore(c.view, head)
		if ok != c.want || (ok && sha != merge) {
			t.Errorf("%s: got (%q, %v), want ok=%v", c.name, sha, ok, c.want)
		}
	}
	if _, ok := MergedBefore(`{"state":"MERGED","headRefOid":"","mergeCommit":{"oid":"`+merge+`"}}`, ""); ok {
		t.Error("ゲートの head が空でも MERGED とした")
	}
}
