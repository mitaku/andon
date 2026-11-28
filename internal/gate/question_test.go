package gate

import "testing"

func TestQuestionHead(t *testing.T) {
	if RecordFact("x") != "gate の記録 x" {
		t.Fatalf("RecordFact が新しい語で書かない: %q", RecordFact("x"))
	}
	const head = "ed22b59ee771cac493e25bf63ddbc69352d3b3ab"
	record := "/p/kanban/issues/T-0001/library/gate/" + head + ".json"
	cases := []struct {
		name  string
		facts []string
		want  string
	}{
		{"記録の行から head を読む", []string{"delegation: merge=B < tier C", "PR https://github.com/example/app/pull/7", RecordFact(record)}, head},
		{"記録の行が無い", []string{"andon issue gate T-0001: 終了コード 3"}, ""},
		{"事実が無い", nil, ""},
		{"gate の置き場でない", []string{RecordFact("/p/kanban/issues/T-0001/library/" + head + ".json")}, ""},
		{"名前が sha でない", []string{RecordFact("/p/kanban/issues/T-0001/library/gate/abc.json")}, ""},
		{"行の頭が違う", []string{"記録 " + record}, ""},
		// ADR-0040 決めたこと 7・受け入れ条件 3: 行の頭は新しい語で書き、古い語の行も同じに読む
		{"新しい語の行の頭", []string{"gate の記録 " + record}, head},
		{"古い語の行の頭", []string{"ゲートの記録 " + record}, head}, // 古い語を読む（ADR-0040 決めたこと 7）
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := QuestionHead(c.facts); got != c.want {
				t.Fatalf("QuestionHead(%q) = %q, want %q", c.facts, got, c.want)
			}
		})
	}
}
