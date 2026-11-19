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
