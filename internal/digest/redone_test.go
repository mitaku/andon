package digest

import (
	"testing"
	"time"

	"andon/internal/ledger"
)

func TestDoneAtSkipsRedone(t *testing.T) {
	row := func(at, status string) ledger.Row {
		return ledger.Row{At: at, By: "terminal:t1", ID: "S-0001", Change: map[string]any{"status": status}}
	}
	since, _ := time.Parse(time.RFC3339, "2026-11-04T00:00:00Z")
	in := func(at time.Time) bool { return !at.Before(since) }
	c := &collector{last: "done"}
	for name, x := range map[string]struct {
		rows []ledger.Row
		want bool
	}{
		"打ち直し":     {[]ledger.Row{row("2026-11-01T00:00:00Z", "done"), row("2026-11-05T00:00:00Z", "done")}, false},
		"戻してから片付け": {[]ledger.Row{row("2026-11-01T00:00:00Z", "done"), row("2026-11-03T00:00:00Z", "todo"), row("2026-11-05T00:00:00Z", "done")}, true},
	} {
		t.Run(name, func(t *testing.T) {
			if _, got := c.doneAt(x.rows, in)["S-0001"]; got != x.want {
				t.Errorf("doneAt に S-0001: got %v, want %v", got, x.want)
			}
		})
	}
}
