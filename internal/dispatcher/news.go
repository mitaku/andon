package dispatcher

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"andon/internal/signal"
)

// NewsFoot は <news> の末尾の1行（dispatcher.md §3.1。通知はデータで、依頼ではない。文は ADR-0010 決めたこと 3）。
const NewsFoot = "これは台帳とhcから拾った通知で、人からの新しい依頼ではない。人に言うことが無ければ `andon news ack` を打ち、文を書かずにターンを終える。"

// Render は待ちの通知を1通の <news> に描く（1行1件。dispatcher.md §3.1）。3つの経路が同じ文を描く。
func Render(slug string, ns []Notification) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<news project=\"%s\">\n", clean(slug, 60))
	for _, n := range ns {
		b.WriteString(line(n) + "\n")
	}
	b.WriteString(NewsFoot + "\n</news>")
	return b.String()
}

// clean は制御文字と区切り（<>）を空白に替え、max 字に切る。題や名前は外から来た文を含みうる。
func clean(v any, max int) string {
	s := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return ' '
		}
		return r
	}, fmt.Sprint(v))
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

func line(n Notification) string {
	d := func(k string) string { return clean(n.Detail[k], 200) }
	short := func(k string) string { return clean(n.Detail[k], 60) }
	head := clean(n.Issue, 40) + " " + n.Kind + ": "
	switch n.Kind {
	case "report":
		return head + "報告 " + d("report")
	case "review":
		return head + "報告 " + d("report") + "（PR " + d("pr") + "）"
	case "asked":
		return head + "問い " + d("n") + "（" + short("kind") + "）"
	case "ready":
		s := head + "「" + short("title") + "」"
		if r, ok := n.Detail["routine"]; ok && r != nil && r != "" {
			s += "（ルーチン " + clean(r, 60) + "）"
		}
		return s
	case "review-requested":
		return head + d("round") + " 回目のレビュアー " + short("reviewer_issue") + "（" + short("reviewer") + "）を起こした"
	case "gate":
		s := head + short("decision")
		if rs, ok := n.Detail["reasons"].([]any); ok && len(rs) > 0 {
			parts := make([]string, len(rs))
			for i, r := range rs {
				parts[i] = clean(r, 60)
			}
			s += "（" + strings.Join(parts, "; ") + "）"
		}
		return s + " 記録 " + d("record")
	case "merged":
		if n.Detail["by"] != "person" {
			return head + d("pr") + " をマージして片付けた（" + clean(n.Detail["sha"], 12) + "）"
		}
		if h, ok := n.Detail["held"]; ok && h != nil {
			return head + d("pr") + " は人がマージした（" + clean(n.Detail["sha"], 12) + "）が、片付けていない: " + clean(h, 200)
		}
		return head + d("pr") + " は人がマージした（" + clean(n.Detail["sha"], 12) + "）。ゲートの問いを閉じて片付けた"
	case "resumed":
		return clean(n.Issue, 40) + " の担当 " + short("from") + " が止まったので、resume して " + short("agent") + " で続きから起こした"
	case "closed":
		return head + d("pr") + " はマージされずに閉じた。片付けていない"
	case "impact":
		if e, ok := n.Detail["error"]; ok && e != nil {
			return head + d("pr") + " のマージの影響を集められなかった（" + clean(e, 120) + "）。andon issue impact " + clean(n.Issue, 40) + " を打って読む"
		}
		s := head + d("pr") + " のマージ（" + clean(n.Detail["sha"], 12) + "）で"
		rows, _ := n.Detail["issues"].([]any)
		var parts []string
		for _, r := range rows {
			m, _ := r.(map[string]any)
			var what []string
			if m["conflict"] == true {
				what = append(what, "衝突")
			}
			if o, _ := m["overlap"].([]any); len(o) > 0 {
				what = append(what, fmt.Sprintf("重なり %d", len(o)))
			}
			if b, _ := m["brief_refs"].([]any); len(b) > 0 {
				what = append(what, fmt.Sprintf("依頼書 %d", len(b)))
			}
			if m["error"] != nil {
				what = append(what, "読めない")
			}
			parts = append(parts, clean(m["issue"], 40)+"（"+strings.Join(what, "・")+"）")
		}
		return s + " " + strings.Join(parts, "; ") + "。andon issue impact " + clean(n.Issue, 40) + " で事実を読む"
	case "corrections":
		return head + "人の手直しを集められなかった（" + clean(n.Detail["error"], 120) + "）。corrections.json は書いていない"
	case "estimate":
		return head + "実働 " + d("worked_minutes") + " 分が見込み（" + short("estimate") + " " + d("estimate_minutes") + " 分）の " + d("stage") + " 倍を超えた（比 " + d("ratio") + "）"
	case "signal":
		// 題は外の人の文。区切りと制御文字を消し、囲いの中に置く（ADR-0011 決めたこと 16）
		return "signal: " + short("source") + " " + d("key") + "（" + d("url") + "）" + signal.Untrusted(d("title"))
	case "memory":
		return "memory: memory が " + d("count") + " 件で線（" + d("line") + " 件）を超えた。臨時のトリアージを足すかを決める"
	case "stalled":
		s := clean(n.Issue, 40) + " の担当 " + short("agent") + " が止まったまま"
		if m := minutesSince(n); m >= 0 {
			s += fmt.Sprintf(" %d 分", m)
		}
		return s
	case "blocked", "error", "finished":
		return clean(n.Issue, 40) + " の担当 " + short("agent") + " が " + n.Kind
	}
	return head + clean(n.Detail, 120)
}

// minutesSince は通知の at と detail.status_ms の差（分）。読めなければ -1。
func minutesSince(n Notification) int {
	at, err := time.Parse("2006-01-02T15:04:05Z", n.At)
	ms, ok := n.Detail["status_ms"].(float64)
	if err != nil || !ok || ms <= 0 {
		return -1
	}
	return int(at.Sub(time.UnixMilli(int64(ms))).Minutes())
}
