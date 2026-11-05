package gate

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"andon/internal/ledger"
)

func pkgTable(t *testing.T) *Table {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	tb, _, err := LoadTable(filepath.Join(root, "package"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestGlob(t *testing.T) {
	for _, c := range []struct {
		g, p string
		want bool
	}{
		{"docs/**", "docs/a/b.md", true},
		{"**/AGENTS.md", "AGENTS.md", true},
		{"**/AGENTS.md", "a/b/AGENTS.md", true},
		{"README*", "README.md", true},
		{"README*", "docs/README.md", false},
		{"compose*.y*ml", "compose.yaml", true},
		{"projects/*/delegation.json", "projects/x/delegation.json", true},
		{"projects/*/delegation.json", "projects/x/y/delegation.json", false},
		{"a.b", "axb", false},
	} {
		if got := Glob(c.g, c.p); got != c.want {
			t.Errorf("Glob(%q, %q) = %v", c.g, c.p, got)
		}
	}
}

func TestClassify(t *testing.T) {
	tb := pkgTable(t)
	for _, c := range []struct {
		name     string
		files    []string
		diff     string
		review   bool
		override map[string]string
		want     string
	}{
		{"docs だけは A", []string{"docs/x.md"}, "", false, nil, "A"},
		{"internal は B", []string{"internal/x.go"}, "", false, nil, "B"},
		{"review: on は B", []string{"docs/x.md"}, "", true, nil, "B"},
		{"skills は C", []string{"package/skills/worker/SKILL.md"}, "", false, nil, "C"},
		{".claude/skills は C の例外（B）", []string{"x/.claude/skills/a.md"}, "", false, nil, "B"},
		{".claude/settings は C", []string{"x/.claude/settings.json"}, "", false, nil, "C"},
		{"追加行 sudo は C", []string{"docs/x.md"}, "+run sudo x\n", false, nil, "C"},
		{"削除行の sudo は数えない", []string{"docs/x.md"}, "-run sudo x\n", false, nil, "A"},
		{"tier_overrides で AGENTS.md を B", []string{"AGENTS.md"}, "", false, map[string]string{"AGENTS.md": "B"}, "B"},
		{"tier_overrides で docs を C", []string{"docs/x.md"}, "", false, map[string]string{"docs/**": "C"}, "C"},
	} {
		var files []File
		for _, f := range c.files {
			files = append(files, File{Path: f, Additions: 1})
		}
		got, err := tb.Classify(RepoTable{TierOverrides: c.override}, files, c.diff, c.review)
		if err != nil || got.Tier != c.want {
			t.Errorf("%s: tier %s（%v）want %s", c.name, got.Tier, got.TierReasons, c.want)
		}
	}
	got, _ := tb.Classify(RepoTable{}, []File{{Path: "docs/x.md", Additions: 600}, {Path: "tests/a.sh", Additions: 5000}}, "", false)
	if got.Tier != "B" || got.ChangedLines != 600 {
		t.Errorf("行数は tests を除いて数え、size_review_lines 超は B: %+v", got)
	}
	got, _ = tb.Classify(RepoTable{}, []File{{Path: "docs/x.md", Additions: 1001}}, "", false)
	if got.Tier != "C" {
		t.Errorf("size_human_lines 超が C でない: %+v", got)
	}
}

func TestWithinAndRaise(t *testing.T) {
	if Within("A", "none") || !Within("B", "B") || Within("C", "B") || !Within("C", "C") {
		t.Error("Within")
	}
	if Raise("A") != "B" || Raise("B") != "C" || Raise("C") != "C" {
		t.Error("Raise")
	}
}

func TestDecide(t *testing.T) {
	good := Facts{PROpen: true, RepoKnown: true, Tier: "B", MergeLevel: "B", ReviewRequired: true,
		Checks: []Check{{"bash tests/*.sh", true}}, Review: ledger.Review{State: "current"}}
	cases := []struct {
		name string
		mod  func(*Facts)
		want string
	}{
		{"揃えば approve", func(*Facts) {}, "approve"},
		{"台帳と PR が合わない", func(f *Facts) { f.LedgerReasons = []string{"x"} }, "fix"},
		{"draft", func(f *Facts) { f.PROpen = false }, "fix"},
		{"fork", func(f *Facts) { f.Blockers = []string{"fork の PR"} }, "needs_human"},
		{"予約キーを信じられない", func(f *Facts) { f.SettingsReason = "x" }, "needs_human"},
		{"条件表に repo が無い", func(f *Facts) { f.RepoKnown = false }, "needs_human"},
		{"委譲の外", func(f *Facts) { f.Tier = "C" }, "needs_human"},
		{"無委譲", func(f *Facts) { f.MergeLevel = "none" }, "needs_human"},
		{"検査が落ちた", func(f *Facts) { f.Checks[0].OK = false }, "fix"},
		{"レビュー missing", func(f *Facts) { f.Review.State = "missing" }, "fix"},
		{"レビュー stale", func(f *Facts) { f.Review.State = "stale" }, "fix"},
		{"Must fix が残る", func(f *Facts) { f.Review.MustFixOpen = 1 }, "fix"},
		{"同じ head で2度目", func(f *Facts) { f.Review.MustFixOpen = 1; f.PreviousMustFix = true }, "needs_human"},
		{"Must fix を見送った", func(f *Facts) { f.Review.WaivedHeavy = 1 }, "needs_human"},
		{"受け入れ条件の fail", func(f *Facts) { f.AcceptanceFail = 1 }, "fix"},
		{"レビュー不要なら missing でも通る", func(f *Facts) { f.ReviewRequired = false; f.Review.State = "off" }, "approve"},
	}
	for _, c := range cases {
		f := good
		f.Checks = append([]Check{}, good.Checks...)
		c.mod(&f)
		d, reasons := Decide(f)
		if d != c.want || (d == "approve") != (len(reasons) == 0) {
			t.Errorf("%s: %s %v want %s", c.name, d, reasons, c.want)
		}
	}
	if _, r := Decide(func() Facts { f := good; f.Tier = "C"; return f }()); !reflect.DeepEqual(r, []string{"delegation: merge=B < tier C"}) {
		t.Errorf("委譲の理由の形: %v", r)
	}
}
