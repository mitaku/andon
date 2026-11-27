package measure

import (
	"path/filepath"
	"testing"
)

// profileGate はプロファイルごとの節の gate の数の合計（記録・fix・needs_human）。
func profileGate(res *Result) [3]int {
	var n [3]int
	for _, p := range res.Profiles {
		n[0] += p.Gate.Records
		n[1] += p.Gate.Fix
		n[2] += p.Gate.NeedsHuman
	}
	return n
}

// TestMeasureSkipsBehindAndRestack は ADR-0067 受け入れ条件 15: 差し戻しの率は、前の PR 待ち（behind）の記録と、
// 載せ替えを頼む直し（restack: true の fix）の記録を、分母にも分子にも数えない。効率の節とプロファイルごとの節の両方で見る。
func TestMeasureSkipsBehindAndRestack(t *testing.T) {
	root := fakeLedger(t)
	before, err := Measure(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	if profileGate(before)[0] == 0 {
		t.Fatal("検査の前提: プロファイルごとの節に gate の記録が無い（下の比べが何も見ない）")
	}
	lib := filepath.Join(root, "kanban/issues/S-0002/library/gate")
	put(t, filepath.Join(lib, "dddd.json"),
		`{"at": "2026-11-02T09:00:00Z", "id": "S-0002", "pr": "`+repoURL+`2", "tier": "B", "decision": "behind", "merged": false}`)
	put(t, filepath.Join(lib, "eeee.json"),
		`{"at": "2026-11-02T09:30:00Z", "id": "S-0002", "pr": "`+repoURL+`2", "tier": "B", "decision": "fix", "merged": false, "restack": true}`)
	res, err := Measure(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	g := res.Efficiency.Gate
	if g.Records != 3 || g.Fix != 1 || g.NeedsHuman != 1 || val(g.Rate) != 0.67 {
		t.Errorf("差し戻しの数え = %+v（rate %v）want records 3・fix 1・needs_human 1・rate 0.67", g, val(g.Rate))
	}
	if got, want := profileGate(res), profileGate(before); got != want {
		t.Errorf("プロファイルごとの節の gate の数が behind と載せ替えの直しで動いた: %v want %v", got, want)
	}
	// 数えないのは restack の印の付いた fix だけ（印の無い fix は数える）
	put(t, filepath.Join(lib, "ffff.json"),
		`{"at": "2026-11-02T09:40:00Z", "id": "S-0002", "pr": "`+repoURL+`2", "tier": "B", "decision": "fix", "merged": false}`)
	if res, err = Measure(opts(root)); err != nil {
		t.Fatal(err)
	}
	if g := res.Efficiency.Gate; g.Records != 4 || g.Fix != 2 {
		t.Errorf("印の無い fix を数えない: %+v", g)
	}
	if got, want := profileGate(res), profileGate(before); got[0] != want[0]+1 || got[1] != want[1]+1 {
		t.Errorf("プロファイルごとの節が印の無い fix を数えない: %v（前 %v）", got, want)
	}
}
