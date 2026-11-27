package tray

import (
	"os"
	"testing"

	"andon/internal/flock"
)

// 控えの pid が生きていても、serve の鍵を誰も持っていなければ動いていない（コンテナを作り直したあと、
// 古い控えの pid を別のプロセスが持つことがある）。鍵を持っている間だけ動いている。
func TestRunningNeedsLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(StateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteControl(&Control{List: "http://127.0.0.1:1", Upload: "http://127.0.0.1:2", PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	if _, ok := Running(); ok {
		t.Error("鍵を誰も持っていないのに、生きている pid だけで動いていると見た")
	}
	unlock, err := flock.Lock(LockPath())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, ok := Running(); !ok {
		t.Error("鍵を持っている serve の控えを、動いていないと見た")
	}
}
