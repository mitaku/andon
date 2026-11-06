package spawn

import "testing"

func TestShortTitle(t *testing.T) {
	for in, want := range map[string]string{"課金の設計": "", "Billing の設計": "billing", "A very long English title for the design doc": "a-very-long-english-title-for"} {
		if got := ShortTitle(in); got != want {
			t.Errorf("ShortTitle(%q)=%q want %q", in, got, want)
		}
	}
}

// 最初のメッセージは、送れて動き出さなかった（started=false）のでもないときだけ届いたとみる。
// started が無い（状態を報告しない program）は誰にも分からないので届いたとみる。
func TestFirstDelivered(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name string
		f    First
		want bool
	}{
		{"送れなかった", First{Sent: false, Reason: "t9 is still starting: its output has not settled"}, false},
		{"送ったが動き出さない", First{Sent: true, Started: &no}, false},
		{"送って動き出した", First{Sent: true, Started: &yes}, true},
		{"状態を報告しない", First{Sent: true}, true},
	} {
		if got := c.f.Delivered(); got != c.want {
			t.Errorf("%s: Delivered()=%v", c.name, got)
		}
	}
	s := Started{First: First{Sent: true, Started: &no}}
	if r := s.Reason(); r != "送ったが動き出さなかった（起動の途中で入力を捨てられたとみる）" {
		t.Errorf("Reason()=%q", r)
	}
}
