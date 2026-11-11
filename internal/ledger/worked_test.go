package ledger

import (
	"testing"
	"time"
)

// 段→分は small 30・medium 60・large 120 で、知らない値は 0（contract.md §2 の表。ADR-0014 決めたこと 3）。
func TestEstimateMinutes(t *testing.T) {
	for v, want := range map[string]int{"small": 30, "medium": 60, "large": 120, "": 0, "中": 0, "Medium": 0} {
		if got := EstimateMinutes(v); got != want {
			t.Errorf("EstimateMinutes(%q) = %d, want %d", v, got, want)
		}
	}
}

// 実働は doing の区間から、自分の問い・レビュー待ち・死んでいた区間を引いた分（ADR-0014 受け入れ条件 2）。
func TestWorked(t *testing.T) {
	base := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return base.Add(time.Duration(m) * time.Minute) }
	row := func(m int, by, id string, change map[string]any) Row {
		return Row{At: at(m).Format(time.RFC3339), By: "terminal:" + by, ID: id, Change: change}
	}
	st := func(m int, by, s string) Row { return row(m, by, "A-0001", map[string]any{"status": s}) }
	q := func(m int, by string, n float64, s string) Row {
		return row(m, by, "A-0001", map[string]any{"question": n, "status": s})
	}
	issues := []Issue{
		{"id": "A-0001", "profile": "claude"},
		{"id": "A-0001-r1", "parent": "A-0001", "profile": "codex-review"},
		{"id": "A-0002", "parent": "A-0001", "profile": "claude"}, // レビュアーでない子は待ちに数えない
	}
	cases := []struct {
		name string
		rows []Row
		gone []int
		now  int
		want int
	}{
		{"待ち無し", []Row{
			row(0, "cos", "A-0001", map[string]any{"add": "題"}),
			st(5, "w", "doing"), st(45, "w", "review"),
		}, nil, 100, 40},
		{"doing のまま今まで", []Row{st(0, "w", "doing")}, nil, 25, 25},
		{"自分の問いが open のまま", []Row{
			st(0, "w", "doing"), q(10, "w", 1, "open"),
		}, nil, 50, 10},
		{"自分の問いが答えられて閉じた", []Row{
			st(0, "w", "doing"), q(10, "w", 1, "open"), q(30, "cos", 1, "answered"), q(30, "cos", 1, "received"),
			st(60, "w", "review"),
		}, nil, 100, 40},
		{"Chief of Staff の問いは待ちに数えない", []Row{
			st(0, "w", "doing"), q(10, "cos", 1, "open"), st(40, "w", "review"),
		}, nil, 100, 40},
		{"ゲートの問いが open のまま doing で働いた（A-0005 の形）", []Row{
			st(0, "w", "doing"), st(20, "w", "review"),
			q(21, "dispatcher", 1, "open"),
			st(60, "w", "doing"), st(90, "w", "review"),
			q(200, "cos", 1, "answered"),
		}, nil, 300, 50},
		{"レビュアーの子課題が add から review まで", []Row{
			st(0, "w", "doing"),
			row(20, "dispatcher", "A-0001-r1", map[string]any{"add": "A-0001 のレビュー（1 回目）"}),
			row(21, "r", "A-0001-r1", map[string]any{"status": "doing", "agent": "r"}),
			row(35, "r", "A-0001-r1", map[string]any{"status": "review"}),
			row(36, "dispatcher", "A-0002", map[string]any{"add": "別の子"}),
			row(37, "x", "A-0002", map[string]any{"status": "review"}),
			st(50, "w", "review"),
		}, nil, 100, 35},
		{"レビュアーの結果を待ったまま", []Row{
			st(0, "w", "doing"),
			row(20, "dispatcher", "A-0001-r1", map[string]any{"add": "A-0001 のレビュー（1 回目）"}),
		}, nil, 60, 20},
		{"死んでいた区間（Gone から resume の agent の行まで）", []Row{
			st(0, "w", "doing"),
			row(90, "cos", "A-0001", map[string]any{"agent": "w2"}),
			st(120, "w2", "review"),
		}, []int{30}, 200, 60},
		{"死んだまま", []Row{st(0, "w", "doing")}, []int{30}, 100, 30},
		{"待ちが重なっても1回だけ引く", []Row{
			st(0, "w", "doing"), q(10, "w", 1, "open"),
			row(20, "dispatcher", "A-0001-r1", map[string]any{"add": "A-0001 のレビュー（1 回目）"}),
			q(30, "cos", 1, "answered"),
			row(40, "r", "A-0001-r1", map[string]any{"status": "review"}),
			st(60, "w", "review"),
		}, nil, 100, 30},
		{"doing の行が2本（入りは最初の1本）", []Row{
			row(0, "cos", "A-0001", map[string]any{"status": "doing", "agent": "w"}),
			row(1, "w", "A-0001", map[string]any{"status": "doing", "agent": "w"}),
			st(30, "w", "review"),
		}, nil, 100, 30},
		{"done → todo → doing は数え直し", []Row{
			st(0, "w", "doing"), st(30, "w", "review"), st(40, "cos", "done"),
			st(100, "cos", "todo"), st(110, "w", "doing"), st(130, "w", "review"),
		}, nil, 200, 20},
		{"review → doing の直しは足す", []Row{
			st(0, "w", "doing"), st(30, "w", "review"), st(50, "w", "doing"), st(60, "w", "review"),
		}, nil, 200, 40},
	}
	for _, c := range cases {
		var gone []time.Time
		for _, g := range c.gone {
			gone = append(gone, at(g))
		}
		if got := Worked(c.rows, issues, "A-0001", "cos", gone, at(c.now)); got != time.Duration(c.want)*time.Minute {
			t.Errorf("%s: Worked = %v, want %dm", c.name, got, c.want)
		}
	}
}
