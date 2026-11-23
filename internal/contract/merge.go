package contract

// Merging は andon issue merge の記録の1件（~/.local/state/andon/dispatcher/<slug>.merge.json の「課題 → これ」。書き手は merge だけ）。
// review に入った log.jsonl の行ごとに、gate を1回回して決めたこと（Decision）とマージ（Merged）を持つ。
// Decision が空なら、その行の gate はまだ決まっていない（checks を待つ・マージの直前に読み直して回し直す・base が動いたマージを打ち直す）。
type Merging struct {
	Row      int    `json:"row"`                // review に入った log.jsonl の行（1 から）
	Sha      string `json:"sha,omitempty"`      // gate が判定した head
	Decision string `json:"decision,omitempty"` // approve・fix・needs_human
	Merged   string `json:"merged,omitempty"`   // dispatcher の道でマージした commit
	Wait     string `json:"wait,omitempty"`     // gate が wait を返した head（その head の checks が揃うまで gate を回さない）
	Approved string `json:"approved,omitempty"` // merge: auto の approve でマージに進み始めた時刻（決まったら消す。10 分たてば dispatcher が届ける）
	Retries  int    `json:"retries,omitempty"`  // base が動いて通らなかったマージを打ち直した回数（決まったら消す）
	Polled   string `json:"polled,omitempty"`   // approve・needs_human を決めたあと、PR の head を最後に読んだ時刻（1分に1回だけ聞く。動いていれば gate を回し直す）
}

// Merge は andon issue merge の出力（docs/contract.md の「gate のあとのマージと片付け」）。Notify を待ちに積むのは dispatcher。
// State は skip（回さない）・wait（checks を待つ）・rate-limited（gh の上限）・regate（読み直して変わった・base が動いてマージが通らなかった。次に回し直す）・
// approve・fix・needs_human・merged・hold（Chief of Staff が止めている。gate を回さない。ADR-0056 決めたこと 9）。
type Merge struct {
	Issue  string   `json:"issue"`
	State  string   `json:"state"`
	Auto   bool     `json:"auto"` // 通知 gate の approve のあと、マージに進む（記録だけにして merged か asked を待つ）
	Log    []string `json:"log"`  // dispatcher のログに書く行
	Notify []Notice `json:"notify"`
}
