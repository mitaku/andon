package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// TriageMade は、課題 t を足したトリアージの apply の記録（kanban の library/triage/<日付>-<トリアージの id>.json の items。
// fix と bundle の決定）を探し、そのトリアージの id と、決定の review（束ならどれか1つが on で on）を返す。無ければ空。
// 記録の項目が t を指すのは、created_issue が t の id のときか、apply が足す前に残す印（adding）があって、題が t の題と同じで、
// t を足したのが今の Chief of Staff の端末（log.jsonl の最初の add の行。行が無ければ足す途中で止まったものとして数える）のとき。andon issue start が無い予約キーを埋めるかの判定に使い、
// トリアージの検査も、止まったあと足した課題を辿れるかを同じ判定で見る。kanban add が返ったあと記録の前で止まった課題も分かる。
func (l *Ledger) TriageMade(rows []Row, t Issue) (string, string) {
	dir, cos := l.Dir, l.ChiefOfStaff()
	id := CanonID(t.Str("id"))
	// 足した行が無い課題は、kanban add が issue.yaml を書いたあと log.jsonl に足す前に止まったもの（kanban add は行を書かずに
	// 終わらない）。足した端末は分からないが、ほかの端末が kanban add で足した課題には行が残るので、印と題が合えば辿る
	byCos := cos != ""
	for _, r := range rows {
		if _, ok := r.Change["add"]; ok && CanonID(r.ID) == id {
			byCos = cos != "" && r.By == "terminal:"+cos
			break
		}
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
		// 印のある項目は1つの決定（apply は決定を1つずつ当て、止まればそこで終わる）なので、review は印のある項目の全部で見る
		created, adding, onCreated, onAdding := false, false, false, false
		for _, it := range rec.Items {
			if it.Decision != "fix" && it.Decision != "bundle" {
				continue
			}
			switch {
			case it.Created != "" && CanonID(it.Created) == id:
				created, onCreated = true, onCreated || it.Issue.Review == "on"
			case it.Created == "" && it.Adding:
				adding, onAdding = adding || (byCos && it.Issue.Title == t.Str("title")), onAdding || it.Issue.Review == "on"
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
