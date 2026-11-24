package ledger

import (
	"reflect"
	"testing"
)

func TestMovedTo(t *testing.T) {
	row := func(id string, change map[string]any) Row {
		return Row{At: "2026-11-01T00:00:00Z", By: "terminal:t1", ID: id, Change: change}
	}
	rows := []Row{
		row("S-0001", map[string]any{"status": "done"}),
		row("S-0001", map[string]any{"status": "done"}),
		row("S-0001", map[string]any{"question": 1.0, "status": "open"}),
		row("S-0001", map[string]any{"status": "done"}),
		row("S-0001", map[string]any{"status": "todo"}),
		row("S-0001", map[string]any{"status": "done"}),
		row("S-1", map[string]any{"status": "done"}),
		row("S-0002", map[string]any{"agent": "t2"}),
		row("S-0002", map[string]any{"status": "done"}),
	}
	want := []bool{true, false, false, false, false, true, false, false, true}
	if got := MovedTo(rows, "done"); !reflect.DeepEqual(got, want) {
		t.Errorf("MovedTo: got %v, want %v", got, want)
	}
}
