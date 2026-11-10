package dispatcher

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// record は通知を記録だけの置き場に1行足す（追記だけ。書き手は dispatcher だけ）。書けなければ控えの Unrecorded に残し、
// 次に書くとき（record・見回りの flush）に先に書く（記録から落とさない。順も保つ）。
func (p *project) record(n Notification) {
	p.st.Unrecorded = append(p.st.Unrecorded, n)
	p.flush()
}

// flush は控えの Unrecorded を記録だけの置き場にまとめて追記する。書けたら空にする。
func (p *project) flush() {
	if len(p.st.Unrecorded) == 0 {
		return
	}
	lines := make([][]byte, len(p.st.Unrecorded))
	for i, n := range p.st.Unrecorded {
		lines[i], _ = json.Marshal(n)
	}
	if err := appendLines(RecordedPath(p.l.Slug), lines); err != nil {
		p.logf("記録だけの通知を書けない（%d 件を控えに残し、次の見回りで書き直す）: %v", len(p.st.Unrecorded), err)
		return
	}
	p.st.Unrecorded = nil
}

// write はファイルへの書き込み（テストが一部だけ書いて失敗する書き手に替える）。
var write = func(f *os.File, b []byte) (int, error) { return f.Write(b) }

// appendLines は lines を1行ずつ path に追記する。
//   - 前に書けていた分は書かない: ファイルの末尾が lines の先頭の何行かと同じなら、その行を飛ばす
//     （追記のあと控えを保存する前に落ちた・前の回が書けたのに失敗を返した、を二重に書かない）。
//   - 一部だけ書いて失敗したら、書く前の大きさに切り詰める（半端な行を残さない）。
func appendLines(path string, lines [][]byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
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
	for k, n := len(lines), len(all); k > 0; k-- { // 書けていた先頭の k 行（長い順に照らす）
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

// lineStart は path の off の1つ前が改行か（off が行の頭か）。
func lineStart(f *os.File, off int64) bool {
	if off == 0 {
		return true
	}
	b := make([]byte, 1)
	_, err := f.ReadAt(b, off-1)
	return err == nil && b[0] == '\n'
}

// stuck は記録だけにした approve のうち、approveWait たっても merged も asked も無いものを1回だけ届ける。
// review にいない課題（人が戻した・片付けた）は捨てる。
func (p *project) stuck(now time.Time) {
	p.flush() // 前に書けなかった記録
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
