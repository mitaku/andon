package gate

import "testing"

// ゲートの問いがどの head について出たかは、事実のゲートの記録の行（RecordFact）から読む。
func TestQuestionHead(t *testing.T) {
	const head = "ed22b59ee771cac493e25bf63ddbc69352d3b3ab"
	record := "/p/kanban/issues/T-0001/library/gate/" + head + ".json"
	cases := []struct {
		name  string
		facts []string
		want  string
	}{
		{"記録の行から head を読む", []string{"delegation: merge=B < tier C", "PR https://github.com/example/app/pull/7", RecordFact(record)}, head},
		{"記録の行が無い", []string{"andon gate T-0001: 終了コード 3"}, ""},
		{"事実が無い", nil, ""},
		{"gate の置き場でない", []string{RecordFact("/p/kanban/issues/T-0001/library/" + head + ".json")}, ""},
		{"名前が sha でない", []string{RecordFact("/p/kanban/issues/T-0001/library/gate/abc.json")}, ""},
		{"行の頭が違う", []string{"記録 " + record}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := QuestionHead(c.facts); got != c.want {
				t.Fatalf("QuestionHead(%q) = %q, want %q", c.facts, got, c.want)
			}
		})
	}
}
