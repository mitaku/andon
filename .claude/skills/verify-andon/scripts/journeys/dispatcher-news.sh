#!/usr/bin/env bash
# dispatcher の通知（features/dispatcher-news.md）。dispatcher の端末と、当てたあとに Chief of Staff へ届けた通知の記録を読む。読むだけ。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
state=${XDG_STATE_HOME:-$HOME/.local/state}/andon/dispatcher

# dn-live: dispatcher が当てた andon で、当てたあとに起動して動いている
d=$(hc get dispatcher --json 2>/dev/null) || d=
echo "-- dn-live: hc get dispatcher --json"
[ -n "$d" ] && jq -c '{state, argv, created_ms, labels: (.labels | {role})}' <<<"$d" | sed 's/^/  | /'
if [ -n "$d" ] && [ "$(jq -r .state <<<"$d")" = live ] && [ "$(jq -r '.argv[0]' <<<"$d")" = /opt/andon/bin/andon ] \
  && [ "$(jq -r '.created_ms // 0' <<<"$d")" -ge "$a_ms" ]; then
  ok "dn-live: dispatcher は /opt/andon/bin/andon で、当てたあとに起動して live"
else
  fail "dn-live: dispatcher が無いか、live でないか、当てる前から動いている"
fi

# dn-delivered: 当てたあとに Chief of Staff へ届けた通知（<slug>.delivered.jsonl）と、記録だけの通知（<slug>.recorded.jsonl）
del=$(since "$state"/*.delivered.jsonl) rec=$(since "$state"/*.recorded.jsonl)
nd=$(grep -c . <<<"$del" || true) nr=$(grep -c . <<<"$rec" || true)
echo "-- dn-delivered: 当てたあとに届けた通知 $nd 件・記録だけの通知 $nr 件"
jq -c '{at, project, acked, news: [.news[] | "\(.issue) \(.kind)"]}' <<<"$del" 2>/dev/null | tail -10 | sed 's/^/  | /'
jq -c '{at, issue, kind}' <<<"$rec" 2>/dev/null | tail -10 | sed 's/^/  | /'
if [ $((nd + nr)) = 0 ]; then
  echo "note 当てたあとの通知がまだ無い。dn-live は確かめたが、dn-delivered は未確認"
  [ "$fails" = 0 ] && exit 3
fi
[ "$nd" -gt 0 ] && ok "dn-delivered: 当てたあとに Chief of Staff へ通知を $nd 回届けた"
[ "$fails" = 0 ]
