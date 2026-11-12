package dispatcher

import "andon/internal/triage"

// memoryLine は memory の件数の線（ADR-0020 決めたこと 4。仮の数で、実測で直す。開いている問い 1）。
const memoryLine = 30

// memory は auto memory の量の段（docs/dispatcher.md §2.11。ADR-0020 決めたこと 4）: セルの memory の置き場の
// 件数（目録 MEMORY.md を除く。数え方はトリアージが拾うものと同じ triage.MemoryFiles）が線を超えたら、
// Chief of Staff に memory を1度積む。控えの MemoryOver で、線の下に戻るまで同じ超え方を繰り返さない。
// 臨時のトリアージを足すかは Chief of Staff が決める（ここは届けるだけ）。
func (p *project) memory() {
	files, err := triage.MemoryFiles(triage.MemoryDir())
	if err != nil {
		p.logf("memory の置き場を数えられない（次の巡で数え直す）: %v", err)
		return
	}
	if len(files) <= memoryLine {
		p.st.MemoryOver = false
		return
	}
	if p.st.MemoryOver {
		return
	}
	p.st.MemoryOver = true
	p.notify("", "memory", map[string]any{"count": len(files), "line": memoryLine})
}
