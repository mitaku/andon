package gate

import (
	"strings"
	"testing"

	"andon/internal/contract"
	"andon/internal/ledger"
)

func TestReviewRequired(t *testing.T) {
	for _, c := range []struct {
		tier, review string
		want         bool
	}{
		{"A", "off", false},
		{"B", "off", true},
		{"C", "off", true},
		{"A", "on", true},
		{"B", "on", true},
		{"A", "", false},
	} {
		if got := ReviewRequired(c.tier, c.review); got != c.want {
			t.Errorf("ReviewRequired(%q, %q) = %v, want %v", c.tier, c.review, got, c.want)
		}
	}
}

// review: off で始めたコードの課題で、gate がレベル B・C としてレビューを求めた差分（Go のテスト・package/lib の検査器・
// モデルの対応表）は、本物の条件表でレビューが要る側に入る。docs だけの差分は要らない側のまま。
func TestReviewRequiredForReviewOffDiffs(t *testing.T) {
	tb := pkgTable(t)
	for _, c := range []struct {
		name  string
		files []File
		tier  string
	}{
		{"Go のテストだけ", []File{{Path: "internal/issue/start_test.go", Additions: 3}}, "B"},
		{"package/lib の検査器と tests", []File{{Path: "package/lib/andon/schema_check.py", Additions: 26}, {Path: "tests/schema.sh", Additions: 84, Deletions: 31}}, "B"},
		{"モデルの対応表", []File{{Path: "model/issue-map.txt", Additions: 42}, {Path: "docs/process/issue.md", Additions: 6}}, "C"},
		{"docs だけ", []File{{Path: "docs/a.md", Additions: 1}}, "A"},
	} {
		cl, err := tb.Classify(RepoTable{}, c.files, "", false)
		if err != nil {
			t.Fatal(err)
		}
		cl, _ = FromReport(cl, &contract.Report{})
		if cl.Tier != c.tier || ReviewRequired(cl.Tier, "off") != (c.tier != "A") {
			t.Errorf("%s: tier %s（%v）、want %s", c.name, cl.Tier, cl.TierReasons, c.tier)
		}
	}
}

func TestNeedsReviewWithoutPR(t *testing.T) {
	if need, _, err := NeedsReview(nil, nil, ledger.Issue{"repo": "github.com/example/app"}, "on", nil, ""); !need || err != nil {
		t.Errorf("on: need %v err %v", need, err)
	}
	if need, _, err := NeedsReview(nil, nil, ledger.Issue{"repo": "github.com/example/app"}, "", nil, ""); need || err != nil {
		t.Errorf("空: need %v err %v", need, err)
	}
	if _, _, err := NeedsReview(nil, nil, ledger.Issue{"repo": "github.com/example/app"}, "off", &contract.Report{}, ""); err == nil {
		t.Error("off で pr の無い報告を誤りにしない")
	}
}

func TestTierLine(t *testing.T) {
	if got := TierLine(Classified{Tier: "A", TierReasons: []string{}}); got != "レベル A" {
		t.Errorf("A: %q", got)
	}
	got := TierLine(Classified{Tier: "B", TierReasons: []string{"a", "b", "c", "d", "e"}})
	if got != "レベル B（a・b・c・ほか 2 件）" || strings.Contains(got, "d") {
		t.Errorf("B: %q", got)
	}
}
