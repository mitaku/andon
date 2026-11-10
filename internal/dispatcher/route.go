package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/ledger"
)

// Route は通知の扱い（ADR-0010 決めたこと 1。docs/dispatcher.md §3.2 の表）。
type Route int

const (
	Deliver Route = iota // 控えに積み、<news> で Chief of Staff に届ける
	Record               // 記録だけ（<slug>.recorded.jsonl に1行。届けない）
	Wait                 // 記録だけにして、approveWait たっても merged も asked も無ければ届ける
)

// Facts は扱いを決めるのに使う、kind のほかの事実（今あるキーから。新しいキーは足さない）。
type Facts struct {
	Gated     bool // dispatcher がゲートを回す課題（gated。Chief Engineer・レビュアー・repo の無い課題・調べる課題でない）
	Reviewer  bool // レビュアーの課題（isReviewer）
	AutoMerge bool // gate の approve のあと dispatcher がマージに進む（merge: auto を Chief of Staff が書いた・委譲の範囲内）
}

// RouteOf は通知の扱いの表（ADR-0010 決めたこと 1・7）。表に無い kind は届ける（安全側）。
func RouteOf(n Notification, f Facts) Route {
	switch n.Kind {
	case "review-requested":
		return Record
	case "review":
		if f.Gated {
			return Record
		}
		return Deliver // Chief Engineer の課題・調べる課題など、ゲートを回さない課題（ADR-0010 決めたこと 7）
	case "gate":
		switch n.Detail["decision"] {
		case "fix", "needs_human":
			return Record
		case "approve":
			if f.AutoMerge {
				return Wait
			}
		}
		return Deliver
	case "merged":
		if h, ok := n.Detail["held"]; ok && h != nil && h != "" {
			return Deliver // 片付けていない（ADR-0010 決めたこと 7）
		}
		return Record
	case "report":
		if f.Reviewer {
			return Record
		}
	}
	return Deliver
}

// approveWait は記録だけにした approve（merge: auto）から、届けるまでの間（仮の値。ADR-0010 決めたこと 1）。
var approveWait = 10 * time.Minute

// RecordedPath は記録だけの通知の置き場（控えの横。contract.md §1・§6）。
func RecordedPath(slug string) string {
	return strings.TrimSuffix(StatePath(slug), ".json") + ".recorded.jsonl"
}

// facts は課題 id の Facts（gate の AutoMerge は呼び手が足す）。
func (p *project) facts(id string) Facts {
	for _, t := range p.tasks {
		if t.Str("id") == id {
			return Facts{Gated: p.gated(t), Reviewer: isReviewer(t)}
		}
	}
	return Facts{}
}

// put は通知を表のとおりに積む・記録する。
func (p *project) put(n Notification, f Facts) {
	route := RouteOf(n, f)
	if route != Wait && (n.Kind == "merged" || n.Kind == "asked" || n.Kind == "gate") {
		delete(p.st.Approved, n.Issue) // approve のあとに進んだ（マージした・問いになった・ゲートが別の答えを出した）
	}
	switch route {
	case Deliver:
		p.st.Push(n)
	case Record:
		p.record(n)
	case Wait:
		p.record(n)
		if _, ok := p.st.Approved[n.Issue]; !ok { // 回し直したゲートの approve では測り直さない
			p.st.Approved[n.Issue] = n
		}
	}
}

// record は通知を記録だけの置き場に1行足す（追記だけ。書き手は dispatcher だけ）。
func (p *project) record(n Notification) {
	path := RecordedPath(p.l.Slug)
	b, _ := json.Marshal(n)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		var f *os.File
		if f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			_, err = f.Write(append(b, '\n'))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
	}
	if err != nil {
		p.logf("記録だけの通知を書けない（%s %s）: %v", n.Issue, n.Kind, err)
	}
}

// stuck は記録だけにした approve のうち、approveWait たっても merged も asked も無いものを1回だけ届ける。
// review にいない課題（人が戻した・片付けた）は捨てる。
func (p *project) stuck(now time.Time) {
	for id, n := range p.st.Approved {
		if p.status(id) != "review" {
			delete(p.st.Approved, id)
			continue
		}
		at, err := time.Parse("2006-01-02T15:04:05Z", n.At)
		if err != nil || now.Sub(at) >= approveWait {
			p.logf("%s はゲートの approve のあと %d 分マージに進まない。Chief of Staff に知らせる", id, int(approveWait.Minutes()))
			p.st.Push(n)
			delete(p.st.Approved, id)
		}
	}
}

// autoMerge は gate の approve のあと dispatcher がマージに進むか（approve と同じ条件。loop.go）。
func (p *project) autoMerge(t ledger.Issue, res gateResult) bool {
	return ledger.SettingsOf(t).Merge == "auto" && p.l.Trusted(p.rows, t, "merge") && res.Delegation.OK
}
