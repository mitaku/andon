package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TriageMade は、課題 t を足したトリアージの apply の記録（kanban の library/triage/<日付>-<トリアージの id>.json の items。
// fix と bundle の決定）を探し、そのトリアージの id と、決定の review（束ならどれか1つが on で on）を返す。無ければ空。
// 記録の項目が t を指すのは、created_issue が t の id のときか、apply が足す前に残す印（adding）があって、タイトルが t のタイトルと同じで、
// t を足したのが今の Chief of Staff の端末（log.jsonl の最初の add の行。行が無ければ、t が印を残した時刻〈adding_at〉より後に作られた）のとき。andon issue start が無い予約キーを埋めるかの判定に使い、
// トリアージの検査も、止まったあと足した課題を辿れるかを同じ判定で見る。kanban add が返ったあと記録の前で止まった課題も分かる。
func (l *Ledger) TriageMade(rows []Row, t Issue) (string, string) {
	dir, cos := l.Dir, l.ChiefOfStaff()
	id := CanonID(t.Str("id"))
	hasRow, byCos := false, false
	for _, r := range rows {
		if _, ok := r.Change["add"]; ok && CanonID(r.ID) == id {
			hasRow, byCos = true, cos != "" && r.By == "terminal:"+cos
			break
		}
	}
	marked := func(addingAt string) bool {
		if hasRow {
			return byCos
		}
		mark, err1 := time.Parse(time.RFC3339, addingAt)
		made, err2 := time.Parse(time.RFC3339, t.Str("created"))
		return cos != "" && len(rows) > 0 && err1 == nil && err2 == nil && !made.Before(mark.Truncate(time.Second))
	}
	files, _ := filepath.Glob(filepath.Join(dir, "library", "triage", "*.json"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var rec struct {
			Items []struct {
				Created  string `json:"created_issue"`
				Adding   bool   `json:"adding"`
				AddingAt string `json:"adding_at"`
				Decision string `json:"decision"`
				Issue    struct {
					Title  string `json:"title"`
					Review string `json:"review"`
				} `json:"issue"`
			} `json:"items"`
		}
		if json.Unmarshal(b, &rec) != nil {
			continue
		}
		created, adding, onCreated, onAdding := false, false, false, false
		for _, it := range rec.Items {
			if it.Decision != "fix" && it.Decision != "bundle" {
				continue
			}
			switch {
			case it.Created != "" && CanonID(it.Created) == id:
				created, onCreated = true, onCreated || it.Issue.Review == "on"
			case it.Created == "" && it.Adding:
				adding, onAdding = adding || (marked(it.AddingAt) && it.Issue.Title == t.Str("title")), onAdding || it.Issue.Review == "on"
			}
		}
		if !created && !adding {
			continue
		}
		review := "off"
		if created && onCreated || !created && onAdding {
			review = "on"
		}
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if len(name) > len("2006-01-02-") {
			name = name[len("2006-01-02-"):]
		}
		return name, review
	}
	return "", ""
}
