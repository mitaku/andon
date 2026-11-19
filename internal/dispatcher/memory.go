package dispatcher

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"andon/internal/triage"
)

// memoryLine は memory の件数の線（ADR-0020 決めたこと 4。仮の数で、実測で直す。開いている問い 1）。
const memoryLine = 30

// Memory は memory の量の線の記録（控えの置き場の .memory.json）。セルの memory の置き場は1つなので、
// 台帳ごとの控えでなくセルに1つ持つ（台帳が2つ以上あっても、同じ超え方の memory は1通。ADR-0031 S2・B4）。
type Memory struct {
	// memory の件数が線を超えて memory を積んだ（線の下に戻ると false。ADR-0020 決めたこと 4）
	Over bool `json:"over"`
}

// MemoryPath は memory の量の線の記録の置き場（控えの横。台帳の名前と重ならないよう . で始める）。
func MemoryPath() string {
	return filepath.Join(filepath.Dir(StatePath("x")), ".memory.json")
}

// LoadMemory は memory の量の線の記録を読む。無ければ線の中。
func LoadMemory() (Memory, error) {
	var m Memory
	b, err := os.ReadFile(MemoryPath())
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, errors.New("memory の量の線の記録が読めない: " + MemoryPath())
	}
	return m, nil
}

// Save は memory の量の線の記録を書く（書いてから置き換える）。
func (m Memory) Save() error {
	path := MemoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// memory は auto memory の量の段（docs/dispatcher.md §2.11。ADR-0020 決めたこと 4）: セルの memory の置き場の
// 件数（目録 MEMORY.md を除く。数え方はトリアージが拾うものと同じ triage.MemoryFiles）が線を超えたら、
// Chief of Staff に memory を1度積む。記録の Over（セルに1つ）で、線の下に戻るまで同じ超え方を繰り返さない。
// 先に回った台帳が積めば、同じ巡のほかの台帳は積まない。記録を書くのは、積んだ台帳の控えを保存したあと（saveMemory）。
// 臨時のトリアージを足すかは Chief of Staff が決める（ここは届けるだけ）。
func (p *project) memory() {
	m, err := LoadMemory()
	if err != nil {
		p.logf("%v（次の巡で読み直す）", err)
		return
	}
	files, err := triage.MemoryFiles(triage.MemoryDir())
	if err != nil {
		p.logf("memory の置き場を数えられない（次の巡で数え直す）: %v", err)
		return
	}
	over := len(files) > memoryLine
	if over == m.Over {
		return
	}
	p.memoryOver = &over
	if over {
		p.notify("", "memory", map[string]any{"count": len(files), "line": memoryLine})
	}
}

// saveMemory は memory の段が変えた記録を書く（pass が控えを保存したあとに呼ぶ）。控えの保存の前に落ちたら
// 記録は前のままで、次の巡でもう一度積む（届くのは1度か2度で、落ちることは無い）。書けなければログに1行。
func (p *project) saveMemory() {
	if p.memoryOver == nil {
		return
	}
	if err := (Memory{Over: *p.memoryOver}).Save(); err != nil {
		p.logf("memory の量の線の記録を書けない（次の巡で積み直しうる）: %v", err)
	}
}
