package retro

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBeforeAfterSkipsBehindAndRestack は ADR-0067 受け入れ条件 15: 前の PR 待ち（behind）の記録が残った課題を含む期間で
// andon retro collect が通り（出力が schema に合う）、差し戻しの率は behind の記録と、載せ替えを頼む直し（restack: true の fix）の
// 記録を数えない。
func TestBeforeAfterSkipsBehindAndRestack(t *testing.T) {
	root, _ := baKanban(t, 12, 10)
	gates := filepath.Join(root, "kanban", "issues", "A-0401", "library", "gate")
	write(t, filepath.Join(gates, strings.Repeat("b", 40)+".json"),
		`{"head":"`+strings.Repeat("b", 40)+`","at":"2026-01-01T00:00:00Z","tier":"B","decision":"behind","reasons":["前の課題 A-0400 の PR がまだ main に入っていない"],"merged":false}`)
	write(t, filepath.Join(gates, strings.Repeat("c", 40)+".json"),
		`{"head":"`+strings.Repeat("c", 40)+`","at":"2026-01-01T00:01:00Z","tier":"B","decision":"fix","reasons":["x"],"merged":false,"restack":true}`)
	_, f := collectFix(t, root)
	if g := f.BeforeAfter.After.Numbers.Gate; g.Records != 10 || g.Fix != 0 || g.Rate == nil || *g.Rate != 0 {
		t.Errorf("後の差し戻しの数え = %+v want records 10・fix 0・rate 0", g)
	}
}
