package dispatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 人が打ち始めるのを、偽の hc の attach --observe の中で起こす: attach が $D/flip-at 回目のとき、画面を出したあとで
// $D/<addr>.typed に替える（observe の最大 3 秒の間に人が打ち始めた）。read はそのときの画面を返す。
const typingHc = `#!/usr/bin/env bash
d=$FAKE_HC
case $1 in
  get) cat "$d/$2.json" ;;
  read) cat "$d/$2.screen" ;;
  attach)
    cat "$d/$3.screen"
    n=$(( $(cat "$d/attach-n" 2>/dev/null || echo 0) + 1 )); echo $n >"$d/attach-n"
    [ "$n" = "$(cat "$d/flip-at" 2>/dev/null)" ] && cp "$d/$3.typed" "$d/$3.screen"
    ;;
  send) shift; [ "$1" = --if ] && shift 2; printf 'send %s\n' "$1" >>"$d/log" ;;
  label) ;;
  *) echo "unexpected hc $1" >&2; exit 9 ;;
esac
`

func typingFixture(t *testing.T, flipAt string) *retryFixture {
	t.Helper()
	f := newRetry(t)
	f.p.tasks = nil
	if err := os.WriteFile(filepath.Join(f.d, "bin", "hc"), []byte(typingHc), 0o755); err != nil {
		t.Fatal(err)
	}
	f.screen("t1", "● 前の答え\n"+rule+"\n❯ \n"+rule+"\n")
	if err := os.WriteFile(filepath.Join(f.d, "t1.typed"), []byte("● 前の答え\n"+rule+"\n❯ はい、例のまま\n"+rule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.d, "flip-at"), []byte(flipAt+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *retryFixture) sent() string {
	b, _ := os.ReadFile(filepath.Join(f.d, "log"))
	return strings.TrimSpace(string(b))
}

// Chief of Staff への続きの送り直し（retryIfEmpty）は、observe の間に人が打ち始めたら打たない（A-0127。W5 で消すまで）。
func TestRetryChiefTypingDuringObserve(t *testing.T) {
	f := typingFixture(t, "1")
	f.term("t1", "error", cutMsg, 0, false)
	c, err := hcGet("t1")
	if err != nil {
		t.Fatal(err)
	}
	if f.p.retryIfEmpty("t1", c) {
		t.Fatal("打ち始めた入力欄に続きを打った")
	}
	if got := f.sent(); got != "" {
		t.Fatalf("打ち始めた入力欄に続きを打った: %q", got)
	}
}
