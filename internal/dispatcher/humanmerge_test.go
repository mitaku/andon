package dispatcher

import "testing"

// 人のマージの通知は「<id> merged: <PR>」で始まり、人がマージしたと分かる文に描く（dispatcher.md §2.7）。
func TestHumanMergeLines(t *testing.T) {
	pr, sha := "https://github.com/example/app/pull/7", "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08"
	for _, c := range []struct {
		kind   string
		detail map[string]any
		want   string
	}{
		{"merged", map[string]any{"pr": pr, "sha": sha}, "A-1 merged: " + pr + " をマージして片付けた（9c4e1b0d7a3f）"},
		{"merged", map[string]any{"pr": pr, "sha": sha, "by": "person"}, "A-1 merged: " + pr + " は人がマージした（9c4e1b0d7a3f）。ゲートの問いを閉じて片付けた"},
		{"merged", map[string]any{"pr": pr, "sha": sha, "by": "person", "held": "ゲートの外の問いが開いている: A-1#1"},
			"A-1 merged: " + pr + " は人がマージした（9c4e1b0d7a3f）が、片付けていない: ゲートの外の問いが開いている: A-1#1"},
		{"closed", map[string]any{"pr": pr}, "A-1 closed: " + pr + " はマージされずに閉じた。片付けていない"},
	} {
		if got := line(Notification{Issue: "A-1", Kind: c.kind, Detail: c.detail}); got != c.want {
			t.Errorf("%s %v:\n got %q\nwant %q", c.kind, c.detail, got, c.want)
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
