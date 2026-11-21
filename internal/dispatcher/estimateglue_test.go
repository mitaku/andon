package dispatcher

import "testing"

// 見込みを超えた担当の課題は、Chief of Staff に estimate を積むだけで、担当の端末には何も打たない（hc を呼ばない）。
func TestEstimateDoesNotSendToWorker(t *testing.T) {
	tools := newScripted(t, "hc")
	f := newEstimate(t)
	for _, c := range []struct{ min, want int }{{31, 1}, {61, 2}} {
		if got := f.round(c.min); got != c.want {
			t.Fatalf("%d 分の見回り: 段 %d（want %d）", c.min, got, c.want)
		}
	}
	if err := f.p.st.Save(f.p.l.Slug); err != nil {
		t.Fatalf("積んだ通知が書く前の照合を通らない: %v", err)
	}
	if calls := tools.calls(); len(calls) != 0 {
		t.Fatalf("見込みを超えた担当に hc を打った: %q", calls)
	}
}
