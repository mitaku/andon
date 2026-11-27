package ledger

import "andon/internal/contract"

// Stacked は、積んだ課題 t の予約キー stacked_on（前の課題の id）と stacked_base（分かれ目の commit）を、信じられるときだけ返す
// （純粋関数。ADR-0067 決めたこと 5）。2 つとも、最後に変えた log.jsonl の行の書き手がその行の時点の Chief of Staff の端末で、
// 値が今の値と同じとき（TrustedThen）だけ信じる。stacked_base は 40 桁の小文字の SHA、stacked_on は自分でない id に限る。
// 片方でも外れれば、どちらも無いものとして読む（担当が自分で書いたキーで、積んだ課題として扱わせない）。
// kanban の待つ相手の数え（cmd/kanban の waitingOn）と、gate・照合・dispatcher の差分の起点が、この1つで読む。
func Stacked(rows []Row, t Issue, cos string) (on, base string, ok bool) {
	on, base = CanonID(t.Str("stacked_on")), t.Str("stacked_base")
	switch {
	case on == "" || base == "" || on == CanonID(t.Str("id")) || !contract.IsSHA(base):
		return "", "", false
	case !TrustedThen(rows, t, "stacked_on", cos) || !TrustedThen(rows, t, "stacked_base", cos):
		return "", "", false
	}
	return on, base, true
}

// ReviewBase は、レビュアーの課題 rev の予約キー review_base（レビューが見た差分の起点の commit）を、信じられるときだけ返す
// （純粋関数。ADR-0067 決めたこと 9）。レビュアーの課題を作るのは dispatcher だけなので、最後に変えた log.jsonl の行の書き手が
// dispatcher の端末（DispatcherBy）で、値が今の値と同じ 40 桁の SHA のときだけ信じる。信じられなければ空（照合は今までどおり
// 既定の branch との merge-base から数える）。
func ReviewBase(rows []Row, rev Issue) string {
	id, v := CanonID(rev.Str("id")), rev.Str("review_base")
	if !contract.IsSHA(v) {
		return ""
	}
	var last *Row
	for i := range rows {
		if rows[i].ID != id || rows[i].Change == nil {
			continue
		}
		if _, ok := rows[i].Change["review_base"]; ok {
			last = &rows[i]
		}
	}
	if last == nil || last.By != DispatcherBy {
		return ""
	}
	if s, ok := last.Change["review_base"].(string); !ok || s != v {
		return ""
	}
	return v
}
