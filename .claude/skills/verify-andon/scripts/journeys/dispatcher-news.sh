#!/usr/bin/env bash
# dispatcher の通知（features/dispatcher-news.md）。dispatcher の端末と、当てたあとに Chief of Staff へ届けた通知の記録を読む。読むだけ。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
state=${XDG_STATE_HOME:-$HOME/.local/state}/andon/dispatcher

# dn-live: dispatcher が当てた andon で、最後に当てたあとに起動して動いている（今の状態なので、検証する commit でなく最後に当てた時刻と比べる）
d=$(hc get dispatcher --json 2>/dev/null) || d=
echo "-- dn-live: hc get dispatcher --json"
[ -n "$d" ] && jq -c '{state, argv, created_ms, labels: (.labels | {role})}' <<<"$d" | sed 's/^/  | /'
if [ -n "$d" ] && [ "$(jq -r .state <<<"$d")" = live ] && [ "$(jq -r '.argv[0]' <<<"$d")" = "$pkg/bin/andon" ] \
  && [ "$(jq -r '.created_ms // 0' <<<"$d")" -ge "$now_ms" ]; then
  ok "dn-live: dispatcher は $pkg/bin/andon で、当てたあとに起動して live"
else
  fail "dn-live: dispatcher が無いか、live でないか、当てる前から動いている"
fi

# dn-delivered: 当てたあとに Chief of Staff へ届けた通知（<slug>.delivered.jsonl）と、記録だけの通知（<slug>.recorded.jsonl）
del=$(dsince "$state"/*.delivered.jsonl) rec=$(dsince "$state"/*.recorded.jsonl)
nd=$(grep -c . <<<"$del" || true) nr=$(grep -c . <<<"$rec" || true)
echo "-- dn-delivered: 当てたあとに届けた通知 $nd 件・記録だけの通知 $nr 件"
jq -c '{at, project, acked, news: [.news[] | "\(.issue // "-") \(.kind)"]}' <<<"$del" 2>/dev/null | tail -10 | sed 's/^/  | /'
jq -c '{at, issue, kind}' <<<"$rec" 2>/dev/null | tail -10 | sed 's/^/  | /'

# dn-idle: 当てたあとに枠の空きを見た控え（<slug>.json の idle）と、通知 idle。撃つかは枠の空き次第なので、通知が無くても壊れではない
echo "-- dn-idle: 控えの idle と、当てたあとの通知 idle"
looked=0
for f in "$state"/*.json; do
  i=$(jq -c --arg t "$d_at" --arg u "$d_until" 'select(.idle.at != null and .idle.at >= $t and ($u == "" or .idle.at < $u)) | {at: .idle.at, free: .idle.free, ready: (.idle.ready // [] | length), told: (.idle.told // [])}' "$f" 2>/dev/null) || continue
  [ -n "$i" ] && { echo "  | ${f##*/}: $i"; looked=$((looked + 1)); }
done
ni=$(jq -c 'select(any(.news[]?; .kind == "idle"))' <<<"$del" 2>/dev/null | grep -c . || true)
jq -c '.at as $at | .news[] | select(.kind == "idle") | {at: $at, detail}' <<<"$del" 2>/dev/null | tail -5 | sed 's/^/  | /'
if [ "$looked" -gt 0 ]; then
  ok "dn-idle: 当てたあとに枠の空きを見た控えが $looked 件。通知 idle を届けた回は $ni 回"
else
  echo "note dn-idle: 当てたあとに枠の空きを見た控えが無い（idle_minutes が 0 か、Chief of Staff の留守か、dispatcher がまだ見ていない）。dn-idle は未確認"
fi

# dn-corrections: 当てたあとに片付いた課題の手直しの記録（library/corrections.json。書き手は dispatcher だけ）。片付く課題が無ければ書かれないので、無くても壊れではない
echo "-- dn-corrections: 当てたあとに dispatcher が書いた手直しの記録"
nc=0
for f in "$ws"/projects/*/kanban/issues/*/library/corrections.json; do
  [ -f "$f" ] || continue
  cr=$(jq -c --arg t "$d_at" --arg u "$d_until" 'select(.at != null and .at >= $t and ($u == "" or .at < $u))
    | {at, kinds: ([.corrections[]?.kind] | group_by(.) | map({(.[0]): length}) | add // {}), human_commits: ([.corrections[]?.human_commits[]?] | length)}' "$f" 2>/dev/null) || continue
  [ -n "$cr" ] || continue
  i=${f%/library/corrections.json}
  echo "  | ${i##*/}: $cr"
  nc=$((nc + 1))
done
if [ "$nc" -gt 0 ]; then
  ok "dn-corrections: 当てたあとに手直しの記録を $nc 件書いた"
else
  echo "note dn-corrections: 当てたあとに書いた手直しの記録が無い（当てたあとに片付いた課題がまだ無い）。dn-corrections は未確認"
fi

# dn-signal: 当てたあとの通知 signal に release（失敗が始まった commit。記録にあるときだけ載る）が載ったか。言えるのは通知の detail だけ。
# 届けた記録（delivered.jsonl）は課題と kind だけで detail を持たないので数だけを読み、release は記録だけの通知（recorded.jsonl。dsince で
# 検証する間に絞ってある）の detail で読む。信号の記録（signals.json）は今の状態で、通知の中身でも検証する間の状態でもないので証拠にしない。title は外の人の文なので出さない
echo "-- dn-signal: 当てたあとの通知 signal と、通知に載った release"
nsd=$(jq -c '.news[]? | select(.kind == "signal")' <<<"$del" 2>/dev/null | grep -c . || true)
srec=$(jq -c 'select(.kind == "signal") | {at, key: .detail.key, url: .detail.url, release: .detail.release}' <<<"$rec" 2>/dev/null)
nsr=$(grep -c . <<<"$srec" || true)
echo "  | 届けた通知 signal $nsd 件・記録だけの通知 signal $nsr 件"
jq -c '.release //= "-"' <<<"$srec" 2>/dev/null | tail -5 | sed 's/^/  | /'
bad=$(jq -c 'select(.release != null and (.release | test("^[0-9a-f]{40}$") | not))' <<<"$srec" 2>/dev/null | grep -c . || true)
wr=$(jq -c 'select(.release != null)' <<<"$srec" 2>/dev/null | grep -c . || true)
if [ "$bad" != 0 ]; then
  fail "dn-signal: 通知 signal の release が 40 桁の commit でないものが $bad 件"
elif [ "$wr" != 0 ]; then
  ok "dn-signal: 記録だけの通知 signal $wr 件の detail に、release（40 桁の commit）が載った"
elif [ $((nsd + nsr)) = 0 ]; then
  echo "note dn-signal: 当てたあとの通知 signal が無い（信号の出所が無いか、新しい信号がまだ無い）。dn-signal は未確認"
else
  echo "note dn-signal: 当てたあとの通知 signal は $((nsd + nsr)) 件あるが、detail を読める通知に release が無い（届けた記録は detail を持たず、記録だけの通知は release を載せていない）。release が載ったかは未確認"
fi
# 記録だけの通知は Chief of Staff に届けていないので、届けた証拠にしない
[ "$fails" = 0 ] || exit 1
if [ "$nd" = 0 ]; then
  echo "note 当てたあとに Chief of Staff へ届けた通知がまだ無い（記録だけの通知 $nr 件）。dn-live は確かめたが、dn-delivered は未確認"
  exit 3
fi
ok "dn-delivered: 当てたあとに Chief of Staff へ通知を $nd 回届けた"
