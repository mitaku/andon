package dispatcher

import (
	"strings"
	"testing"
)

// 留守の間は積んだ数を置き換えの前の数で数え、asked は問いの n ごとに別に積む（ADR-0017 決めたこと 8 の (b)(d)）。
func TestPushAwayCountsAndAskedPerN(t *testing.T) {
	st := &State{Pending: []Notification{}, Away: &Away{Since: "2026-11-12T00:00:00Z"}}
	st.Push(Notification{Issue: "A-0001", Kind: "asked", Detail: map[string]any{"n": 1}})
	st.Push(Notification{Issue: "A-0001", Kind: "asked", Detail: map[string]any{"n": 2}})
	st.Push(Notification{Issue: "A-0001", Kind: "asked", Detail: map[string]any{"n": float64(2)}}) // 控えを読み直した n（JSON の数）も同じ問い
	st.Push(Notification{Issue: "A-0001", Kind: "report", Detail: map[string]any{}})
	if len(st.Pending) != 3 {
		t.Errorf("待ちが %d 件（asked n=1・n=2・report の 3 件のはず）: %+v", len(st.Pending), st.Pending)
	}
	if st.Away.Count != 4 {
		t.Errorf("留守の数が %d（置き換えの前の 4 のはず）", st.Away.Count)
	}
	st.Away.Until = "2026-11-12T01:00:00Z"
	st.Push(Notification{Issue: "A-0002", Kind: "report", Detail: map[string]any{}})
	if st.Away.Count != 4 {
		t.Errorf("戻ったあとに積んだものを留守に数えた: %d", st.Away.Count)
	}
}

// 控えの留守の記録は、留守に入った巡で始まり、戻った巡で終わる。何も積まなかった留守は戻ったときに消し、
// 終わりのあと drain の前にまた留守になったら続きとして終わりを消す。
func TestMarkAway(t *testing.T) {
	p := &project{st: &State{}}
	p.away = true
	p.markAway("t1")
	if p.st.Away == nil || p.st.Away.Since != "t1" {
		t.Fatalf("留守の始まりを書かない: %+v", p.st.Away)
	}
	p.markAway("t2")
	if p.st.Away.Since != "t1" {
		t.Errorf("留守の続きで始まりを書き直した: %+v", p.st.Away)
	}
	p.away = false
	p.markAway("t3")
	if p.st.Away != nil {
		t.Errorf("何も積まなかった留守を戻ったあとも残した: %+v", p.st.Away)
	}
	p.away = true
	p.markAway("t4")
	p.st.Away.Count = 2
	p.away = false
	p.markAway("t5")
	if a := p.st.Away; a == nil || a.Since != "t4" || a.Until != "t5" {
		t.Fatalf("戻った巡で終わりを書かない: %+v", a)
	}
	p.away = true
	p.markAway("t6")
	if a := p.st.Away; a.Since != "t4" || a.Until != "" || a.Count != 2 {
		t.Errorf("drain の前にまた留守になったのを続きにしない: %+v", a)
	}
}

// 戻った最初の <news> の先頭（<news> の次の行）に留守の1行が付く。積まなかった留守では付けない。
func TestRenderAway(t *testing.T) {
	ns := []Notification{{Issue: "A-0001", Kind: "report", Detail: map[string]any{"report": "r.md"}}}
	got := RenderAway("demo", awayLine(&Away{Since: "2026-11-12T00:00:00Z", Until: "2026-11-12T00:10:00Z", Count: 3}), ns)
	if !strings.HasPrefix(got, "<news project=\"demo\">\n留守の間（2026-11-12T00:00:00Z〜2026-11-12T00:10:00Z）に 3 件\nA-0001 report: ") {
		t.Errorf("留守の1行が先頭に無い: %q", got)
	}
	if awayLine(&Away{Since: "x"}) != "" || awayLine(nil) != "" {
		t.Error("積まなかった留守に1行を付けた")
	}
	if Render("demo", ns) != RenderAway("demo", "", ns) {
		t.Error("留守の無い <news> が変わった")
	}
}
