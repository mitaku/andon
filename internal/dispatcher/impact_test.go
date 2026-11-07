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
	res := contract.Impact{Task: "A-1", PR: "https://github.com/example/app/pull/1", Sha: sha, Tasks: []contract.ImpactTask{
		{Task: "A-2", Contains: true, Conflict: true, Overlap: []string{"x"}}, // もう含む: 影響は済んでいる
		{Task: "A-3", Overlap: []string{}, BriefRefs: []string{}},             // 何も重ならない
		{Task: "A-4", Conflict: true, ConflictFiles: []string{"x"}, Overlap: many, BriefRefs: []string{"docs/a.md"}},
		{Task: "A-5", Error: "branch demo/A-5 が読めない"},
	}}
	d := ImpactDetail(res)
	if d == nil || d["pr"] != res.PR || d["sha"] != sha {
		t.Fatalf("detail = %v", d)
	}
	rows := d["tasks"].([]any)
	if len(rows) != 2 {
		t.Fatalf("影響のあるタスクは A-4・A-5 の2件のはず: %v", rows)
	}
	a4, a5 := rows[0].(map[string]any), rows[1].(map[string]any)
	if a4["task"] != "A-4" || a4["conflict"] != true || len(a4["overlap"].([]any)) != 20 || len(a4["brief_refs"].([]any)) != 1 || a4["error"] != nil {
		t.Errorf("A-4 = %v", a4)
	}
	if a5["task"] != "A-5" || a5["error"] == nil || len(a5["overlap"].([]any)) != 0 {
		t.Errorf("A-5 = %v", a5)
	}
	if ImpactDetail(contract.Impact{PR: res.PR, Sha: sha, Tasks: res.Tasks[:2]}) != nil {
		t.Error("影響のあるタスクが無いのに detail を作った")
	}

	got := line(Notification{Task: "A-1", Kind: "impact", Detail: d})
	want := "A-1 impact: https://github.com/example/app/pull/1 のマージ（aaaaaaaaaaaa）で A-4（衝突・重なり 20・依頼書 1）; A-5（読めない）。andon task impact A-1 で事実を読む"
	if got != want {
		t.Errorf("line =\n %s\nwant\n %s", got, want)
	}

	got = line(Notification{Task: "A-1", Kind: "impact", Detail: map[string]any{"pr": res.PR, "error": "3 回打って失敗: exit status 1"}})
	want = "A-1 impact: https://github.com/example/app/pull/1 のマージの影響を集められなかった（3 回打って失敗: exit status 1）。andon task impact A-1 を打って読む"
	if got != want {
		t.Errorf("line =\n %s\nwant\n %s", got, want)
	}
}
