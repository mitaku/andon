package ledger

import (
	"reflect"
	"testing"
)

// done の打ち直し（前の列も done）と問いの行は、done へ動いた行に入れない。戻してから片付けた行と、履歴の最初の行は入れる。
func TestMovedTo(t *testing.T) {
	row := func(id string, change map[string]any) Row {
		return Row{At: "2026-11-01T00:00:00Z", By: "terminal:t1", ID: id, Change: change}
	}
	rows := []Row{
		row("S-0001", map[string]any{"status": "done"}),                  // 最初の行
		row("S-0001", map[string]any{"status": "done"}),                  // 打ち直し
		row("S-0001", map[string]any{"question": 1.0, "status": "open"}), // 問いの行は列を変えない
		row("S-0001", map[string]any{"status": "done"}),                  // まだ打ち直し
		row("S-0001", map[string]any{"status": "todo"}),
		row("S-0001", map[string]any{"status": "done"}), // 戻してから片付け
		row("S-1", map[string]any{"status": "done"}),    // 移す前の id の形でも同じ課題
		row("S-0002", map[string]any{"agent": "t2"}),
		row("S-0002", map[string]any{"status": "done"}),
	}
	want := []bool{true, false, false, false, false, true, false, false, true}
	if got := MovedTo(rows, "done"); !reflect.DeepEqual(got, want) {
		t.Errorf("MovedTo: got %v, want %v", got, want)
	}
}
