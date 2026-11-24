package dispatcher

import (
	"strings"
	"testing"

	"andon/internal/contract"
)

func TestImpactDetail(t *testing.T) {
	sha := strings.Repeat("a", 40)
	many := make([]string, 25)
	for i := range many {
		many[i] = "f" + string(rune('a'+i))
	}
	res := contract.Impact{Issue: "A-1", PR: "https://github.com/example/app/pull/1", Sha: sha, Issues: []contract.ImpactIssue{
		{Issue: "A-2", Contains: true, Conflict: true, Overlap: []string{"x"}}, // もう含む: 影響は済んでいる
		{Issue: "A-3", Overlap: []string{}, BriefRefs: []string{}},             // 何も重ならない
		{Issue: "A-4", Conflict: true, ConflictFiles: []string{"x"}, Overlap: many, BriefRefs: []string{"docs/a.md"}},
		{Issue: "A-5", Error: "branch demo/A-5 が読めない"},
		{Issue: "A-6", Status: "review", Overlap: []string{}, BriefRefs: []string{}, ChecksMoved: []string{".github/workflows/go.yml"}}, // 検査の置き場が動いた
		{Issue: "A-7", Status: "review", Contains: true, ChecksMoved: []string{".github/workflows/go.yml"}},
		{Issue: "A-8", Status: "doing", ChecksMoved: []string{".github/workflows/go.yml"}}, // doing は gate が ready のあと返す                             // もう含む
	}}
	d := ImpactDetail(res)
	if d == nil || d["pr"] != res.PR || d["sha"] != sha {
		t.Fatalf("detail = %v", d)
	}
	rows := d["issues"].([]any)
	if len(rows) != 3 {
		t.Fatalf("影響のある課題は A-4・A-5・A-6 の3件のはず: %v", rows)
	}
	a4, a5, a6 := rows[0].(map[string]any), rows[1].(map[string]any), rows[2].(map[string]any)
	if a4["issue"] != "A-4" || a4["conflict"] != true || len(a4["overlap"].([]any)) != 20 || len(a4["brief_refs"].([]any)) != 1 || a4["error"] != nil {
		t.Errorf("A-4 = %v", a4)
	}
	if a5["issue"] != "A-5" || a5["error"] == nil || len(a5["overlap"].([]any)) != 0 {
		t.Errorf("A-5 = %v", a5)
	}
	if c, _ := a6["checks_moved"].([]any); a6["issue"] != "A-6" || len(c) != 1 || c[0] != ".github/workflows/go.yml" {
		t.Errorf("A-6 = %v", a6)
	}
	if c, ok := a4["checks_moved"].([]any); !ok || len(c) != 0 {
		t.Errorf("A-4 の checks_moved が空の配列でない: %v", a4)
	}
	if ImpactDetail(contract.Impact{PR: res.PR, Sha: sha, Issues: res.Issues[:2]}) != nil {
		t.Error("影響のある課題が無いのに detail を作った")
	}

	got := line(Notification{Issue: "A-1", Kind: "impact", Detail: d})
	want := "A-1 impact: https://github.com/example/app/pull/1 のマージ（aaaaaaaaaaaa）で A-4（衝突・重なり 20・依頼書 1）; A-5（読めない）; A-6（検査の置き場 1）。andon issue impact A-1 で事実を読む"
	if got != want {
		t.Errorf("line =\n %s\nwant\n %s", got, want)
	}

	got = line(Notification{Issue: "A-1", Kind: "impact", Detail: map[string]any{"pr": res.PR, "error": "3 回打って失敗: exit status 1"}})
	want = "A-1 impact: https://github.com/example/app/pull/1 のマージの影響を集められなかった（3 回打って失敗: exit status 1）。andon issue impact A-1 を打って読む"
	if got != want {
		t.Errorf("line =\n %s\nwant\n %s", got, want)
	}
}
