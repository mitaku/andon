package dispatcher

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"andon/internal/ledger"
)

// ログの行の頭に秒までの時刻が付く。台帳に timezone があればそれで、無ければ UTC。行の中身は変えない。
func TestLogLineHasTimestamp(t *testing.T) {
	now := time.Date(2026, 11, 12, 7, 5, 9, 0, time.UTC)
	var b bytes.Buffer
	c := &Dispatcher{Log: &b, Now: func() time.Time { return now }}

	c.logf("hello %d", 1)
	if got, want := b.String(), "2026-11-12T07:05:09Z andon dispatcher: hello 1\n"; got != want {
		t.Errorf("UTC: got %q want %q", got, want)
	}

	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "kanban"), 0o755)
	os.WriteFile(filepath.Join(root, "kanban", "config.yaml"), []byte("version: 3\ntimezone: Asia/Tokyo\n"), 0o644)
	p := &project{c: c, l: ledger.At(root)}
	b.Reset()
	p.logf("x")
	if got, want := b.String(), "2026-11-12T16:05:09+09:00 andon dispatcher: "+p.l.Slug+": x\n"; got != want {
		t.Errorf("timezone: got %q want %q", got, want)
	}

	p = &project{c: c, l: ledger.At(t.TempDir())}
	b.Reset()
	p.logf("x")
	if got, want := b.String(), "2026-11-12T07:05:09Z andon dispatcher: "+p.l.Slug+": x\n"; got != want {
		t.Errorf("no config: got %q want %q", got, want)
	}
}
