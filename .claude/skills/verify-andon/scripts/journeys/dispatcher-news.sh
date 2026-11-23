#!/usr/bin/env bash
# dispatcher の通知（features/dispatcher-news.md）。dispatcher の端末と、当てたあとに Chief of Staff へ届けた通知の記録を読む。読むだけ。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
state=${XDG_STATE_HOME:-$HOME/.local/state}/andon/dispatcher

# dn-live: dispatcher が当てた andon で、当てたあとに起動して動いている
d=$(hc get dispatcher --json 2>/dev/null) || d=
echo "-- dn-live: hc get dispatcher --json"
[ -n "$d" ] && jq -c '{state, argv, created_ms, labels: (.labels | {role})}' <<<"$d" | sed 's/^/  | /'
if [ -n "$d" ] && [ "$(jq -r .state <<<"$d")" = live ] && [ "$(jq -r '.argv[0]' <<<"$d")" = "$pkg/bin/andon" ] \
  && [ "$(jq -r '.created_ms // 0' <<<"$d")" -ge "$a_ms" ]; then
  ok "dn-live: dispatcher は $pkg/bin/andon で、当てたあとに起動して live"
else
  fail "dn-live: dispatcher が無いか、live でないか、当てる前から動いている"
fi

# dn-delivered: 当てたあとに Chief of Staff へ届けた通知（<slug>.delivered.jsonl）と、記録だけの通知（<slug>.recorded.jsonl）
del=$(since "$state"/*.delivered.jsonl) rec=$(since "$state"/*.recorded.jsonl)
nd=$(grep -c . <<<"$del" || true) nr=$(grep -c . <<<"$rec" || true)
echo "-- dn-delivered: 当てたあとに届けた通知 $nd 件・記録だけの通知 $nr 件"
jq -c '{at, project, acked, news: [.news[] | "\(.issue // "-") \(.kind)"]}' <<<"$del" 2>/dev/null | tail -10 | sed 's/^/  | /'
jq -c '{at, issue, kind}' <<<"$rec" 2>/dev/null | tail -10 | sed 's/^/  | /'

# dn-idle: 当てたあとに枠の空きを見た控え（<slug>.json の idle）と、通知 idle。撃つかは枠の空き次第なので、通知が無くても壊れではない
echo "-- dn-idle: 控えの idle と、当てたあとの通知 idle"
looked=0
for f in "$state"/*.json; do
  i=$(jq -c --arg t "$a_at" 'select(.idle.at != null and .idle.at >= $t) | {at: .idle.at, free: .idle.free, ready: (.idle.ready // [] | length), told: (.idle.told // [])}' "$f" 2>/dev/null) || continue
  [ -n "$i" ] && { echo "  | ${f##*/}: $i"; looked=$((looked + 1)); }
done
ni=$(jq -c 'select(any(.news[]?; .kind == "idle"))' <<<"$del" 2>/dev/null | grep -c . || true)
jq -c '.at as $at | .news[] | select(.kind == "idle") | {at: $at, detail}' <<<"$del" 2>/dev/null | tail -5 | sed 's/^/  | /'
if [ "$looked" -gt 0 ]; then
  ok "dn-idle: 当てたあとに枠の空きを見た控えが $looked 件。通知 idle を届けた回は $ni 回"
else
  echo "note dn-idle: 当てたあとに枠の空きを見た控えが無い（idle_minutes が 0 か、Chief of Staff の留守か、dispatcher がまだ見ていない）。dn-idle は未確認"
fi
# 記録だけの通知は Chief of Staff に届けていないので、届けた証拠にしない
[ "$fails" = 0 ] || exit 1
if [ "$nd" = 0 ]; then
  echo "note 当てたあとに Chief of Staff へ届けた通知がまだ無い（記録だけの通知 $nr 件）。dn-live は確かめたが、dn-delivered は未確認"
  exit 3
fi
ok "dn-delivered: 当てたあとに Chief of Staff へ通知を $nd 回届けた"
