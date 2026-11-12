package dispatcher

import (
	"encoding/json"
	"fmt"
	"os"
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
		err = appendLines(DeliveredPath(slug), [][]byte{b})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "andon dispatcher drain: 届けた通知の記録を書けない（数えから落ちる）: %v\n", err)
	}
}
