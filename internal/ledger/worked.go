package ledger

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// estimateMinutes は予約キー estimate の段→分（contract.md §2 の表。待ちを除いた実働の目安で、規則ではない）。
// Go の定数はここだけ。dispatcher（ズレの通知）と andon measure（記録）が EstimateMinutes で読む。ADR-0014 決めたこと 3・11 の 7。
var estimateMinutes = map[string]int{"small": 30, "medium": 60, "large": 120}

// EstimateMinutes は見込みの段 v の分。知らない値・空は 0（見込み無し）。
func EstimateMinutes(v string) int {
	return estimateMinutes[v]
}

// span は時刻の区間 [from, to)。
type span struct{ from, to time.Time }

// Worked は課題 id の実働（待ちを除いた時間。ADR-0014 決めたこと 5・11 の 1・2）。
//
//   - 課題の change.status が doing の区間の総和。入りは「doing でない status の後の最初の doing」（andon issue start と
//     担当の kanban start で doing の行が2本ある）。問いの行（change.question がある）の status は見ない。
//     status が todo か queued に戻ったら（done → todo → doing の起動し直し）、それまでの区間を捨てて数え直す。
//   - そこから次の待ちを引く（重なりは1回だけ）:
//     自分の問いが open の区間（open の行の by が dispatcher・terminal:<cos> でない問い。answered などで閉じる）、
//     レビュー待ち（parent が id で profile が *-review の子課題の add の行から、その子の最初の review か done の行まで）、
//     端末が死んでいた区間（gone は dispatcher が Gone を見た時刻。そこから課題の次の agent の行まで）。
//
// 終わっていない区間は now まで。rows は log.jsonl の順（時刻の順）。時刻の読めない行は飛ばす。
func Worked(rows []Row, issues []Issue, id, cos string, gone []time.Time, now time.Time) time.Duration {
	children := map[string]bool{}
	for _, t := range issues {
		if t.Str("parent") == id && strings.HasSuffix(t.Str("profile"), "-review") {
			children[t.Str("id")] = true
		}
	}
	var (
		doing  []span
		waits  []span
		in     *time.Time
		asked  = map[string]time.Time{} // 開いている自分の問い（番号 → open の時刻）
		review = map[string]time.Time{} // 結果を待っているレビュアーの子課題（id → add の時刻）
		agents []time.Time              // 課題の agent の行（死んでいた区間の終わり）
	)
	for _, r := range rows {
		at, err := time.Parse(time.RFC3339, r.At)
		if err != nil || r.Change == nil {
			continue
		}
		status, _ := r.Change["status"].(string)
		if children[r.ID] {
			if _, ok := r.Change["add"]; ok {
				review[r.ID] = at
			} else if from, ok := review[r.ID]; ok && (status == "review" || status == "done") {
				waits = append(waits, span{from, at})
				delete(review, r.ID)
			}
			continue
		}
		if r.ID != id {
			continue
		}
		if _, ok := r.Change["agent"]; ok {
			agents = append(agents, at)
		}
		if q, ok := r.Change["question"]; ok {
			n := fmt.Sprint(q)
			if from, ok := asked[n]; ok && status != "open" {
				waits = append(waits, span{from, at})
				delete(asked, n)
			} else if status == "open" && r.By != DispatcherBy && (cos == "" || r.By != "terminal:"+cos) {
				asked[n] = at
			}
			continue
		}
		switch {
		case status == "":
		case status == "doing":
			if in == nil {
				in = &at
			}
		default:
			if in != nil {
				doing = append(doing, span{*in, at})
				in = nil
			}
			if status == "todo" || status == "queued" {
				doing = nil
			}
		}
	}
	if in != nil {
		doing = append(doing, span{*in, now})
	}
	for _, from := range asked {
		waits = append(waits, span{from, now})
	}
	for _, from := range review {
		waits = append(waits, span{from, now})
	}
	for _, g := range gone {
		to := now
		for _, a := range agents {
			if !a.Before(g) {
				to = a
				break
			}
		}
		waits = append(waits, span{g, to})
	}
	waits = union(waits)
	var total time.Duration
	for _, d := range doing {
		total += d.to.Sub(d.from)
		for _, w := range waits {
			from, to := later(d.from, w.from), earlier(d.to, w.to)
			if to.After(from) {
				total -= to.Sub(from)
			}
		}
	}
	return total
}

// union は区間を時刻の順に並べ、重なるものをつなぐ。
func union(in []span) []span {
	sort.Slice(in, func(i, j int) bool { return in[i].from.Before(in[j].from) })
	var out []span
	for _, s := range in {
		if !s.to.After(s.from) {
			continue
		}
		if n := len(out); n > 0 && !s.from.After(out[n-1].to) {
			out[n-1].to = later(out[n-1].to, s.to)
			continue
		}
		out = append(out, s)
	}
	return out
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
