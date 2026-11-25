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
	worked, _ := WorkedWaits(rows, issues, id, cos, gone, now)
	return worked
}

// Waits は Worked が引いた待ちを種類ごとに分けたもの（どれも doing の区間の中だけ。結果の記録の内訳 breakdown の waits。
// docs/contract.md §19）。種類の間で重なった区間は、それぞれの種類に数える（Worked は重なりを1回だけ引く）。
type Waits struct {
	Question time.Duration // 自分の問いが開いていた間
	Review   time.Duration // レビュアーの課題を待っていた間
	Gone     time.Duration // 端末が死んでいた間
}

// WorkedWaits は Worked と、Worked が引いた待ちの種類ごとの内訳を返す。数え方は Worked の注釈のとおり。
func WorkedWaits(rows []Row, issues []Issue, id, cos string, gone []time.Time, now time.Time) (time.Duration, Waits) {
	children := map[string]bool{}
	for _, t := range issues {
		if t.Str("parent") == id && strings.HasSuffix(t.Str("profile"), "-review") {
			children[t.Str("id")] = true
		}
	}
	var (
		doing  []span
		asks   []span
		revs   []span
		deads  []span
		in     *time.Time
		asked  = map[string]time.Time{}
		review = map[string]time.Time{}
		agents []time.Time
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
				revs = append(revs, span{from, at})
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
				asks = append(asks, span{from, at})
				delete(asked, n)
			} else if status == "open" && r.By != DispatcherBy && (cos == "" || r.By != "terminal:"+cos) {
				asked[n] = at
			}
			continue
		}
		switch status {
		case "":
		case "doing":
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
		asks = append(asks, span{from, now})
	}
	for _, from := range review {
		revs = append(revs, span{from, now})
	}
	for _, g := range gone {
		to := now
		for _, a := range agents {
			if !a.Before(g) {
				to = a
				break
			}
		}
		deads = append(deads, span{g, to})
	}
	var total time.Duration
	for _, d := range doing {
		total += d.to.Sub(d.from)
	}
	all := append(append(append([]span{}, asks...), revs...), deads...)
	w := Waits{Question: within(doing, asks), Review: within(doing, revs), Gone: within(doing, deads)}
	return total - within(doing, all), w
}

// within は waits を重なりを除いて合わせ、doing の区間に入る長さの合計を返す。
func within(doing, waits []span) time.Duration {
	waits = union(waits)
	var total time.Duration
	for _, d := range doing {
		for _, w := range waits {
			from, to := later(d.from, w.from), earlier(d.to, w.to)
			if to.After(from) {
				total += to.Sub(from)
			}
		}
	}
	return total
}

// LastStatus は log.jsonl の課題 id の change.status が status の最後の行の位置と時刻（問いの行と時刻の読めない行は飛ばす）。
// 閉じたときまでの実働は rows[:i+1] で数える（時刻で打ち切ると、同じ秒にあとから書かれた todo などの行を混ぜる）。
func LastStatus(rows []Row, id, status string) (int, time.Time, bool) {
	return LastStatusUntil(rows, id, status, time.Time{})
}

// LastStatusUntil は LastStatus のうち、時刻が until まで（含む）の最後の行。until が零なら限らない。
// 閉じた時刻を別に読んだ履歴で決めた読み手（andon measure）が、そのあとに足された done の行まで伸ばさないために使う。
func LastStatusUntil(rows []Row, id, status string, until time.Time) (int, time.Time, bool) {
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if CanonID(r.ID) != id || r.Change == nil || r.Change["question"] != nil || r.Change["status"] != status {
			continue
		}
		if t, err := time.Parse(time.RFC3339, r.At); err == nil && (until.IsZero() || !t.After(until)) {
			return i, t, true
		}
	}
	return 0, time.Time{}, false
}

// MovedTo は rows の行ごとに、課題をほかの列から status の列へ動かした行か（rows と同じ長さ）。前の列も status の行
// （片付いた課題への kanban done の打ち直し）は動いたうちに入れない。問いの行（question を持つ）は課題の列に数えない。
// 履歴に前の列が無い課題の最初の行は動いたうちに入れる。
func MovedTo(rows []Row, status string) []bool {
	out := make([]bool, len(rows))
	column := map[string]any{}
	for i, r := range rows {
		s, ok := r.Change["status"]
		if !ok || r.Change["question"] != nil {
			continue
		}
		id := CanonID(r.ID)
		out[i] = s == status && column[id] != status
		column[id] = s
	}
	return out
}

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
