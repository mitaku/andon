package dispatcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"andon/internal/contract"
)

// Route は通知の扱い（ADR-0010 決めたこと 1。docs/dispatcher.md §3.2 の表）。
type Route int

const (
	Deliver Route = iota
	Record
	Wait
)

type Facts struct {
	Gated     bool
	Reviewer  bool
	AutoMerge bool
	Engineer  bool
}

// RouteOf は通知の扱いの表（ADR-0010 決めたこと 1・7）。表に無い kind は届ける（安全側）。
func RouteOf(n Notification, f Facts) Route {
	switch n.Kind {
	case "review-requested", "resumed":
		return Record
	case "review":
		if f.Gated {
			return Record
		}
		return Deliver // Chief Engineer の課題・調べる課題など、gate を回さない課題（ADR-0010 決めたこと 7）
	case "gate":
		switch n.Detail["decision"] {
		case "fix":
			if u, ok := n.Detail["unsent"]; ok && u != nil && u != "" {
				return Deliver
			}
			if n.Detail["no_agent"] == true {
				return Deliver
			}
			return Record
		case "needs_human":
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
		if f.Engineer {
			return Deliver
		}
		return Record
	case "report":
		if f.Reviewer {
			return Record
		}
	case "answered":
		return Deliver // オーナーがペインで答えた。Chief of Staff が担当へ渡す（ADR-0042 決めたこと 19）
	case "stalled", "blocked", "ready":
		if f.Engineer {
			return Record // オーナーと話していてオーナーの返事を待つのが普通・起動済み。Chief of Staff は答えられない（ADR-0010 決めたこと 1）
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

func (p *project) facts(id string) Facts {
	for _, t := range p.tasks {
		if t.Str("id") == id {
			return Facts{Gated: p.gated(t), Reviewer: isReviewer(t), Engineer: isEngineer(t)}
		}
	}
	return Facts{}
}

func (p *project) put(n Notification, f Facts) {
	switch RouteOf(n, f) {
	case Deliver:
		p.st.Push(n)
	case Record, Wait:
		p.record(n)
	}
}

func (p *project) record(n Notification) {
	p.st.Unrecorded = append(p.st.Unrecorded, n)
	p.flush()
}

func (p *project) flush() {
	if len(p.st.Unrecorded) == 0 {
		return
	}
	lines := make([][]byte, len(p.st.Unrecorded))
	for i, n := range p.st.Unrecorded {
		b, err := contract.EncodeLine(contract.SchemaNotification, n)
		if err != nil {
			p.logf("記録だけの通知が schema に合わない（%d 件を書かずに控えに残す）: %v", len(p.st.Unrecorded), err)
			return
		}
		lines[i] = b
	}
	if err := appendLines(RecordedPath(p.l.Slug), lines); err != nil {
		p.logf("記録だけの通知を書けない（%d 件を控えに残し、次の見回りで書き直す）: %v", len(p.st.Unrecorded), err)
		return
	}
	p.st.Unrecorded = nil
}

var write = func(f *os.File, b []byte) (int, error) { return f.Write(b) }

func appendLines(path string, lines [][]byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // 書き切ったかは下の f.Sync で見る
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	var all []byte
	for _, l := range lines {
		all = append(append(all, l...), '\n')
	}
	tail := make([]byte, min(size, int64(len(all))))
	if _, err := f.ReadAt(tail, size-int64(len(tail))); err != nil {
		return err
	}
	done := 0
	for k, n := len(lines), len(all); k > 0; k-- {
		if bytes.HasSuffix(tail, all[:n]) && lineStart(f, size-int64(n)) {
			done = n
			break
		}
		n -= len(lines[k-1]) + 1
	}
	buf := all[done:]
	if len(buf) == 0 {
		return nil
	}
	if _, err := write(f, buf); err != nil {
		if terr := f.Truncate(size); terr != nil {
			return fmt.Errorf("%v（書く前の大きさに戻せない: %v）", err, terr)
		}
		return err
	}
	return f.Sync()
}

func lineStart(f *os.File, off int64) bool {
	if off == 0 {
		return true
	}
	b := make([]byte, 1)
	_, err := f.ReadAt(b, off-1)
	return err == nil && b[0] == '\n'
}

// stuck は andon issue merge の記録のうち、merge: auto の approve でマージに進み始めて（approved）approveWait たっても
// 決まらない（マージ・問い・別の答えのどれも無い）課題を、通知 `gate` として1回だけ届ける（配る側が merge の記録を読む。ADR-0017 決めたこと 6）。
// 同じ approved では二度届けない（控えの Stuck）。review にいない課題は見ない。
func (p *project) stuck(now time.Time) {
	p.flush()
	rec := map[string]contract.Merging{}
	if b, err := os.ReadFile(MergePath(p.l.Slug)); err == nil {
		if err := json.Unmarshal(b, &rec); err != nil {
			p.logf("andon issue merge の記録が読めない（approve のまま止まったかを見ない）: %v", err)
			return
		}
	}
	for id, at := range p.st.Stuck {
		if rec[id].Approved != at {
			delete(p.st.Stuck, id)
		}
	}
	for id, w := range rec {
		if w.Approved == "" || p.st.Stuck[id] == w.Approved || p.status(id) != "review" {
			continue
		}
		at, err := time.Parse("2006-01-02T15:04:05Z", w.Approved)
		if err == nil && now.Sub(at) < approveWait {
			continue
		}
		p.logf("%s は gate の approve のあと %d 分マージに進まない。Chief of Staff に知らせる", id, int(approveWait.Minutes()))
		record := filepath.Join(p.l.Library(id), "gate", w.Sha+".json")
		var g struct {
			Reasons []string `json:"reasons"`
		}
		if b, err := os.ReadFile(record); err == nil {
			json.Unmarshal(b, &g) //nolint:errcheck // 記録が読めなければ理由を空にして知らせる
		}
		if g.Reasons == nil {
			g.Reasons = []string{}
		}
		p.st.Push(Notification{At: w.Approved, Project: p.l.Slug, Issue: id, Kind: "gate",
			Detail: map[string]any{"decision": "approve", "reasons": g.Reasons, "record": record}})
		p.st.Stuck[id] = w.Approved
	}
}
