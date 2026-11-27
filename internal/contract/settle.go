package contract

// Settled は andon issue settle の記録の1件（~/.local/state/andon/dispatcher/<slug>.settle.json の「課題 → これ」。書き手は settle だけ）。
// State は空（見ている）・MERGED・CLOSED。MERGED と分かったら GitHub に聞き直さない。CLOSED は開き直されうるので聞き直す。
type Settled struct {
	PR     string `json:"pr"`
	Commit string `json:"commit,omitempty"`  // 見始めたときの report.json の commit（替われば見直す）
	At     string `json:"at,omitempty"`      // 最後に gh pr view を打った時刻
	State  string `json:"state,omitempty"`   // MERGED・CLOSED
	Head   string `json:"head,omitempty"`    // マージされた PR の head（gate の記録 library/gate/<head>.json を引く）
	Sha    string `json:"sha,omitempty"`     // マージの commit
	Held   string `json:"held,omitempty"`    // 片付けずに知らせた理由（同じ理由では二度知らせない）
	Base   string `json:"base,omitempty"`    // 既定の branch でない base にマージされたときの base（片付けない。ADR-0067 決めたこと 15）
	BaseOK bool   `json:"base_ok,omitempty"` // MERGED の PR の向き先を既定の branch と比べた。無い MERGED の記録（前のバージョンが書いた）は比べ直す
}

// Settle は andon issue settle の出力（docs/contract.md の「オーナーのマージの片付け」）。Notify を待ちに積むのは dispatcher。
// State は skip（見ない）・watching（まだ OPEN）・auto（dispatcher がマージした）・closed・held・finished。
// Log は dispatcher が自分のログに写す行（PR の印を揃えた・揃えられなかった。ADR-0056 決めたこと 5）。
type Settle struct {
	Issue  string   `json:"issue"`
	State  string   `json:"state"`
	Notify []Notice `json:"notify"`
	Log    []string `json:"log,omitempty"`
}

// Notice は Settle が返す通知の種類と detail（通知の形は docs/contract.md §6）。
type Notice struct {
	Kind   string         `json:"kind"`
	Detail map[string]any `json:"detail"`
}
