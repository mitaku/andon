package retro

import (
	"testing"
	"time"

	"andon/internal/ledger"
)

func TestDoneTimesSkipsRedone(t *testing.T) {
	row := func(at, status string) ledger.Row {
		return ledger.Row{At: at, By: "terminal:t1", ID: "S-0001", Change: map[string]any{"status": status}}
	}
	first, again := "2026-11-01T00:00:00Z", "2026-11-05T00:00:00Z"
	for name, c := range map[string]struct {
		rows []ledger.Row
		want string
	}{
		"打ち直し":     {[]ledger.Row{row(first, "done"), row(again, "done")}, first},
		"戻してから片付け": {[]ledger.Row{row(first, "done"), row("2026-11-03T00:00:00Z", "todo"), row(again, "done")}, again},
	} {
		t.Run(name, func(t *testing.T) {
			want, _ := time.Parse(time.RFC3339, c.want)
			if got := doneTimes(c.rows)["S-0001"]; !got.Equal(want) {
				t.Errorf("doneTimes: got %v, want %v", got, want)
			}
		})
	}
}
