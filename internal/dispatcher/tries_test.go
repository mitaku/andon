package dispatcher

import (
	"encoding/json"
	"reflect"
	"testing"
)

// 失敗を数えて諦める控え（影響の集め・オーナーの手直しの集め・結果の記録が共有する型。ADR-0034 決めたこと 9）。
// 同じ done の行を2度読んでも数え直さず、済んだ課題を外し、上限の回で諦め、控えの JSON の形は課題 → 数のまま。
func TestTries(t *testing.T) {
	var tr Tries
	tr.Add("D-0002")
	tr.Add("D-0001")
	if tr.Fail("D-0001", 3) {
		t.Fatal("1 回目の失敗で諦めた")
	}
	tr.Add("D-0001") // 同じ done の行をもう一度読んだ
	if tr["D-0001"] != 1 {
		t.Fatalf("控え直しで失敗の回数が戻った: %v", tr)
	}
	if tr.Fail("D-0001", 3) || !tr.Fail("D-0001", 3) {
		t.Fatalf("3 回目の失敗で諦めない（2 回目で諦めた）: %v", tr)
	}
	var seen []string
	tr.Each(func(id string) bool { seen = append(seen, id); return id == "D-0002" })
	if !reflect.DeepEqual(seen, []string{"D-0001", "D-0002"}) {
		t.Fatalf("id の順に渡さない: %v", seen)
	}
	if _, ok := tr["D-0002"]; ok || tr["D-0001"] != 3 {
		t.Fatalf("済んだ課題を外さない・済んでいない課題の回数を変えた: %v", tr)
	}
	b, err := json.Marshal(struct {
		T Tries `json:"t,omitempty"`
	}{tr})
	if err != nil || string(b) != `{"t":{"D-0001":3}}` {
		t.Fatalf("控えの形が課題 → 数でない: %s %v", b, err)
	}
	tr.Each(func(string) bool { return true })
	if tr != nil {
		t.Fatalf("空になった控えを nil にしない（控えに空の欄が残る）: %#v", tr)
	}
}
