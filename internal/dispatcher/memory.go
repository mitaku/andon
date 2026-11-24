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
// kanban ごとの控えでなくセルに1つ持つ（kanban が2つ以上あっても、同じ超え方の memory は1通。ADR-0031 S2・B4）。
type Memory struct {
	// memory の件数が線を超えて memory を積んだ（線の下に戻ると false。ADR-0020 決めたこと 4）
	Over bool `json:"over"`
}

func MemoryPath() string {
	return filepath.Join(filepath.Dir(StatePath("x")), ".memory.json")
}

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
// 先に回った kanban が積めば、同じ巡のほかの kanban は積まない。記録の値はプロセスの中にも持ち（Dispatcher.memory）、
// 書き換えるのは積んだ kanban の控えを保存したあと（saveMemory）。ファイルに書けなかった記録は、この段で書き直す。
// 臨時のトリアージを足すかは Chief of Staff が決める（ここは届けるだけ）。
func (p *project) memory() {
	c := p.c
	if c.memory == nil {
		m, err := LoadMemory()
		if err != nil {
			p.logf("%v（次の巡で読み直す）", err)
			return
		}
		c.memory = &m
	}
	if c.memoryUnsaved {
		if err := c.memory.Save(); err != nil {
			p.logf("memory の量の線の記録を書けない（このプロセスの間は覚えておき、次の巡で書き直す）: %v", err)
		} else {
			c.memoryUnsaved = false
		}
	}
	files, err := triage.MemoryFiles(triage.MemoryDir())
	if err != nil {
		p.logf("memory の置き場を数えられない（次の巡で数え直す）: %v", err)
		return
	}
	over := len(files) > memoryLine
	if over == c.memory.Over {
		return
	}
	p.memoryOver = &over
	if over {
		p.notify("", "memory", map[string]any{"count": len(files), "line": memoryLine})
	}
}

func (p *project) saveMemory() {
	if p.memoryOver == nil {
		return
	}
	c := p.c
	c.memory = &Memory{Over: *p.memoryOver}
	if err := c.memory.Save(); err != nil {
		c.memoryUnsaved = true
		p.logf("memory の量の線の記録を書けない（このプロセスの間は覚えておき、次の巡で書き直す）: %v", err)
		return
	}
	c.memoryUnsaved = false
}
