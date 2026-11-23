package triage

import (
	"time"

	ledgercli "andon/internal/ledger"
)

// Unsorted は、仕分けていない項目の数（ADR-0047 決めたこと 7）。dispatcher が、空きのトリアージを撃つかをこの数で決める。
// 数えるのは andon triage collect が出す項目（上限で切らず、伏せる時間の中の keep も含めた全部）のうち、前までの記録で
// 次回に回した（keep）項目を除いたもの。root はプロジェクトの根（kanban/ の親）。読むだけで、kanban にもリポジトリにも書かない。
func Unsorted(root string, now time.Time) (int, error) {
	l, err := ledgerAt(ledgercli.CLI, root, now)
	if err != nil {
		return 0, err
	}
	return l.unsorted()
}

func (l *ledger) unsorted() (int, error) {
	records, err := l.records()
	if err != nil {
		return 0, err
	}
	signals, err := l.loadSignals()
	if err != nil {
		return 0, err
	}
	items, err := l.collectFrom(records, signals)
	if err != nil {
		return 0, err
	}
	kept := map[string]bool{}
	for _, r := range records {
		for _, it := range r.items {
			if it.decision() == "keep" {
				kept[it.key()] = true
			}
		}
	}
	n := 0
	for _, it := range items {
		if !kept[it.Key] {
			n++
		}
	}
	return n, nil
}
