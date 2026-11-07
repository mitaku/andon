package contract

// Impact は andon task impact の出力（docs/contract.md §6 の「マージのあとの影響」。dispatcher が読んで通知 impact にする）。事実だけで、判断は Chief of Staff。
type Impact struct {
	Task  string       `json:"task"`
	PR    string       `json:"pr"`
	Sha   string       `json:"sha"`   // マージの commit
	Files []string     `json:"files"` // マージした PR が触ったファイル
	Tasks []ImpactTask `json:"tasks"` // 同じ repo の doing／review の担当のタスク
}

// ImpactTask は、マージで影響を受けうる1つのタスクの事実。
type ImpactTask struct {
	Task          string   `json:"task"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Agent         string   `json:"agent,omitempty"`
	Branch        string   `json:"branch"`
	Head          string   `json:"head,omitempty"`
	PR            string   `json:"pr,omitempty"`
	Contains      bool     `json:"contains"`       // branch がもうマージの commit を含む（影響は済んでいる）
	Conflict      bool     `json:"conflict"`       // マージの commit と git merge-tree でぶつかる
	ConflictFiles []string `json:"conflict_files"` // ぶつかるファイル
	Overlap       []string `json:"overlap"`        // マージした PR と、このタスクの branch（＋worktree の commit していない変更）の両方が触ったファイル
	BriefRefs     []string `json:"brief_refs"`     // マージした PR が触ったファイルのうち、このタスクの依頼書に path か名前が出てくるもの
	Error         string   `json:"error,omitempty"`
}

// Affected は Chief of Staff が見るべきか（ぶつかる・重なる・依頼書が指す・事実を読めなかった）。
func (t ImpactTask) Affected() bool {
	if t.Error != "" {
		return true
	}
	return !t.Contains && (t.Conflict || len(t.Overlap) > 0 || len(t.BriefRefs) > 0)
}
