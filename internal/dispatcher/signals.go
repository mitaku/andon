package dispatcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"andon/internal/signal"
)

// signals は外の信号の段（dispatcher.md §2.1.1 の 4e。ADR-0011 決めたこと 5・6・8、ADR-0017 決めたこと 14）。
// andon signal collect --due を打ち（出所と repo の組ごとに signals[].every 分に1回だけ読む。読むかは collect が
// 記録の read_at で決める）、記録のうち届けていない now の信号（active で delivered_episode < episodes）を通知 signal で積む。
// 届けたことは控えではなく記録に持つ（andon signal delivered。書くのは andon signal の口）ので、初めて見る台帳・
// 控えを失った巡でも同じ規則で届く。積んだ待ちを控えに保存してから delivered を書く（落ちても届けそこねない。
// 保存のあと delivered の前に落ちたら、次の巡で同じ信号を積み直し、待ちの中では置き換わる）。
// gh の上限でゲートを止めている間（控えの GHReset）は collect を打たない（届けるのは続ける）。
// project.json に signals が無ければ何も打たない（読むかの正は collect。ここは巡ごとにコマンドを起こさないための下見）。
func (p *project) signals(at time.Time) {
	slug := p.l.Slug
	if !hasSignals(filepath.Join(p.l.Root, "project.json")) {
		return
	}
	if until, err := time.Parse(time.RFC3339, p.st.GHReset); err != nil || !at.Before(until) {
		if out, _, err := p.andon("", "signal", "collect", "--due", "--project", slug); err != nil {
			p.logf("andon signal collect が通らない（その組は every 分のあとに読み直す）: %v %s", err, oneLine(out))
		}
	}
	f, err := signal.Load(signal.Path(slug))
	if err != nil {
		p.logf("%v", err)
		return
	}
	todo := signal.Undelivered(f, slug)
	if len(todo) == 0 {
		return
	}
	for _, r := range todo {
		p.put(signalNotification(slug, r, at), Facts{})
	}
	if err := p.st.Save(slug); err != nil {
		p.logf("控えを書けないので、信号を届けたと書かない（次の巡で積み直す）: %v", err)
		return
	}
	for _, r := range todo {
		if _, _, err := p.andon("", "signal", "delivered", "--project", slug, r.Fingerprint, strconv.Itoa(r.Episodes)); err != nil {
			p.logf("信号 %s を届けたと書けない（次の巡で積み直す）: %v", r.Key, err)
		}
	}
}

// hasSignals は project.json に空でない signals があるか（形は確かめない。読めなければ無い）。
func hasSignals(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var v struct {
		Signals []json.RawMessage `json:"signals"`
	}
	return json.Unmarshal(b, &v) == nil && len(v.Signals) > 0
}

// signalNotification は記録1件の通知 signal（contract.md §6。detail は記録の写し）。
func signalNotification(slug string, r signal.Record, at time.Time) Notification {
	return Notification{At: at.UTC().Format("2006-01-02T15:04:05Z"), Project: slug, Kind: "signal", Detail: map[string]any{
		"source": r.Source, "key": r.Key, "url": r.URL, "title": r.Title, "weight": r.Weight}}
}

// oneLine は出力を1行にまとめる（ログ用）。
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
