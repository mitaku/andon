package dispatcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Delivered は届けた通知の記録の1行（<slug>.delivered.jsonl。contract.md §17。schema/delivered.schema.json）。
// フックで <news> を渡したターンの終わりを drain が見たときに1行。acked はそのターンに andon news ack があったか。
type Delivered struct {
	At      string          `json:"at"`
	Project string          `json:"project"`
	Acked   bool            `json:"acked"`
	News    []DeliveredNews `json:"news"`
}

// DeliveredNews はそのターンで渡した通知1件（課題と kind だけ）。
type DeliveredNews struct {
	Issue string `json:"issue,omitempty"`
	Kind  string `json:"kind"`
}

// DeliveredPath は届けた通知の記録の置き場（控えの横。contract.md §1・§17）。
func DeliveredPath(slug string) string {
	return strings.TrimSuffix(StatePath(slug), ".json") + ".delivered.jsonl"
}

// logDelivered は、フックで渡した <news>（h）のターンの終わりを見たとき、届けた通知の記録に1行足す。
// 数えのための記録なので、書けなくても届けは止めない（stderr に1行。andon measure が素通りの数を数える）。
func logDelivered(slug string, h *HookNews, acked bool) {
	d := Delivered{At: now(), Project: slug, Acked: acked}
	for _, n := range append(append([]Notification{}, h.News...), h.Again...) {
		d.News = append(d.News, DeliveredNews{Issue: n.Issue, Kind: n.Kind})
	}
	if len(d.News) == 0 {
		return
	}
	b, err := json.Marshal(d)
	if err == nil {
		err = appendLine(DeliveredPath(slug), b)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "andon dispatcher drain: 届けた通知の記録を書けない（数えから落ちる）: %v\n", err)
	}
}

// appendLine は1行を追記する。appendLines と違い、末尾に同じ行があっても飛ばさない
// （同じ秒に同じ通知を渡した別のターンは別の行。書き直しはしないので二重に書くことも無い）。
func appendLine(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
