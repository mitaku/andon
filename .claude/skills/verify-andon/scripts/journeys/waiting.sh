#!/usr/bin/env bash
# オーナー待ち（features/waiting.md）。切り離した kanban で、Chief of Staff がオーナーの課題を足し（andon issue ask --new --now）、
# オーナーと同じく andon waiting list で判断の行として読む。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

ledger_init V
p1=$(HC_TERMINAL_ID=tc "$kanban" add "$run の親の課題 1" --profile claude)
p2=$(HC_TERMINAL_ID=tc "$kanban" add "$run の親の課題 2" --profile claude)
export HC_TERMINAL_ID=tc
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
q="{\"kind\": \"users\", \"question\": \"$run の書き出しは CSV だけにするか\", \"options\": [{\"label\": \"CSV だけ\", \"pick\": true}, {\"label\": \"Excel も\"}], \"from\": \"chief-of-staff\"}"
owner_of() { "$kanban" show "$1" --json | jq -c '.issue | {id, title, parent, who, status}'; }

# ow-ask-new: オーナーの課題が足され、who: person と問いを持つ
step "ow-ask-new: 親 $p1 の下にオーナーの課題を足す" "$andon" issue ask --new --now --parent "$p1" - <<<"$q"
a=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
echo "  | ${a:-無い}: $( [ -n "$a" ] && owner_of "$a")"
[ -n "$a" ] && "$kanban" show "$a" --json | jq -e --arg p "$p1" '.issue.who == "person" and .issue.parent == $p and (.issue.title | endswith("（オーナーの判断待ち）"))' >/dev/null \
  && jq -e '.kind == "users" and .from == "chief-of-staff"' "$tmp/project/kanban/issues/$a/library/questions/1.json" >/dev/null \
  && ok "ow-ask-new: オーナーの課題 $a が who: person・親 $p1 で足され、問い 1 が library/questions/1.json に置かれる" \
  || fail "ow-ask-new: オーナーの課題か問いが無い・形が違う"

# ow-same-title: 同じ問いでも親が違えば別のオーナーの課題になる。同じ親で打ち直すと断り、課題を増やさない
step "ow-same-title: 同じ問いを親 $p2 の下に足す" "$andon" issue ask --new --now --parent "$p2" - <<<"$q"
b=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
echo "  | ${b:-無い}: $( [ -n "$b" ] && owner_of "$b")"
[ -n "$b" ] && [ "$b" != "$a" ] && [ "$("$kanban" show "$b" --json | jq -r .issue.parent)" = "$p2" ] \
  && ok "ow-same-title: 親の違う同じタイトルは取り違えず、別のオーナーの課題 $b になる" \
  || fail "ow-same-title: 親の違う同じ問いが別の課題にならない"
n0=$("$kanban" list --json | jq length)
step "ow-same-title: 同じ問いを親 $p1 の下に打ち直す" "$andon" issue ask --new --now --parent "$p1" - <<<"$q" \
  && fail "ow-same-title: 問いを持つ同じオーナーの課題があるのに通った" \
  || { grep -qF "同じ問いのオーナーの課題 $a がもうある" "$tmp/step.out" && [ "$("$kanban" list --json | jq length)" = "$n0" ] \
    && ok "ow-same-title: 同じ親の同じ問いは、同じ問いの課題 $a があると言って断り、課題を増やさない" \
    || fail "ow-same-title: 断った理由が同じ問いの課題でないか、断ったのに課題が増えた"; }

# ow-waiting: オーナー待ちに、2つの課題が判断の行として出る（答えられる問い）
step "ow-waiting: andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
jq -c '{slots, items: [.items[] | {kind, id, question, answerable}]}' "$tmp/step.out" 2>/dev/null | sed 's/^/  waiting: /'
jq -e --arg a "$a" --arg b "$b" '[.items[] | select(.kind == "decision" and .answerable == true) | .id] | sort == ([$a, $b] | sort)' "$tmp/step.out" >/dev/null \
  && ok "ow-waiting: オーナー待ちに $a と $b が答えられる判断の行として出る" \
  || fail "ow-waiting: 判断の行が足した2つの課題と違う"
step "ow-waiting: andon waiting list を打つ（人が読む出力）" "$andon" waiting list \
  && grep -qF "$a" "$tmp/step.out" && ok "ow-waiting: 人が読む出力にも $a が出る" || fail "ow-waiting: 人が読む出力に $a が無い"

# ow-later: Chief of Staff が印 later を書いたオーナーの課題は、呼ばずに置いてある行になる。日（later_until）が来るか、
# 印を担当の端末が書き直すと、また呼ぶ
row_of() { jq -c --arg id "$1" '.items[] | select(.id == $id) | {id, call, later, later_until}' "$tmp/step.out" 2>/dev/null; }
why="$run 急がない。次のリリースで決めればよい"
step "ow-later: Chief of Staff（tc）が $a に印 later を書く" "$kanban" set "$a" later="$why"
step "ow-later: 印のあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a") $(row_of "$b")"
jq -e --arg a "$a" --arg b "$b" --arg w "$why" '[.items[] | select(.id == $a or .id == $b) | [.id, .call, (.later // "")]] == [[$b, true, ""], [$a, false, $w]]' "$tmp/step.out" >/dev/null \
  && ok "ow-later: $a は call: false で印の理由を持ち、呼んでいる $b の後に並ぶ" \
  || fail "ow-later: 印を書いた $a が置いてある行にならない・並びが違う"
step "ow-later: 印のあとに andon waiting list を打つ（人が読む出力）" "$andon" waiting list
grep -q '^置いてあるもの 1 件' "$tmp/step.out" && grep -qF "置いた理由: $why" "$tmp/step.out" \
  && ok "ow-later: 人が読む出力に「置いてあるもの 1 件」と置いた理由が出る" \
  || fail "ow-later: 人が読む出力に置いてあるものの見出しか理由が無い"
step "ow-later: Chief of Staff が $a に先の日 later_until=2999-12-31 を書く" "$kanban" set "$a" later_until=2999-12-31
step "ow-later: 先の日のあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a")"
jq -e --arg a "$a" '.items[] | select(.id == $a) | .call == false and .later_until == "2999-12-31"' "$tmp/step.out" >/dev/null \
  && ok "ow-later: 日が先なら $a は置いたままで、呼ぶ日 2999-12-31 が出る" \
  || fail "ow-later: 日が先なのに $a を呼んだか、呼ぶ日が出ない"
step "ow-later: Chief of Staff が $a に来た日 later_until=2000-01-01 を書く" "$kanban" set "$a" later_until=2000-01-01
step "ow-later: 日が来たあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a")"
jq -e --arg a "$a" '.items[] | select(.id == $a) | .call == true' "$tmp/step.out" >/dev/null \
  && ok "ow-later: 日が来た $a はまた呼ぶ（call: true）" \
  || fail "ow-later: 日が来たのに $a を呼ばない"
step "ow-later: Chief of Staff が later_until を消す" "$kanban" set "$a" later_until=
step "ow-later: 担当の端末 tw が $a の印 later を書き直す" env HC_TERMINAL_ID=tw "$kanban" set "$a" later="$why"
step "ow-later: 担当が書いたあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $(row_of "$a")"
jq -e --arg a "$a" '.items[] | select(.id == $a) | .call == true and (has("later") | not)' "$tmp/step.out" >/dev/null \
  && ok "ow-later: 担当の端末が書いた印は信じず、$a をまた呼ぶ" \
  || fail "ow-later: 担当の端末が書いた印で $a を置いたままにした"

# ow-ask-later: 作る口は呼ぶか置いておくかを言い切らせる。旗が無ければ理由を言って断り、Chief of Staff の --later は印を書いて
# 置いてある行にし、担当の端末の --later は断る。どの断りでも課題は増えない
q2="{\"kind\": \"users\", \"question\": \"$run の一覧の既定の並びを変えるか\", \"options\": [{\"label\": \"変えない\", \"pick\": true}, {\"label\": \"新しい順\"}], \"from\": \"chief-of-staff\"}"
why2="$run 急がない。答えを待つ仕事が無い"
n1=$("$kanban" list --json | jq length)
step "ow-ask-later: 旗なしで ask --new を打つ" "$andon" issue ask --new --parent "$p2" - <<<"$q2" \
  && fail "ow-ask-later: 旗なしの ask --new が通った" \
  || { grep -qF '呼ぶか置いておくかを言い切る' "$tmp/step.out" && [ "$("$kanban" list --json | jq length)" = "$n1" ] \
    && ok "ow-ask-later: 旗なしの ask --new は、呼ぶか置いておくかを言い切れと言って断り、課題を増やさない" \
    || fail "ow-ask-later: 旗なしの断りの理由が違うか、課題が増えた"; }
step "ow-ask-later: 担当の端末 tw が --later を打つ" env HC_TERMINAL_ID=tw "$andon" issue ask --new --later "$why2" --parent "$p2" - <<<"$q2" \
  && fail "ow-ask-later: 担当の端末の --later が通った" \
  || { grep -qF -- '--later で印を書くのは Chief of Staff の端末だけ' "$tmp/step.out" && [ "$("$kanban" list --json | jq length)" = "$n1" ] \
    && ok "ow-ask-later: 担当の端末の --later は、Chief of Staff の端末だけと言って断り、課題を増やさない" \
    || fail "ow-ask-later: 担当の端末の --later の断りの理由が違うか、課題が増えた"; }
step "ow-ask-later: Chief of Staff（tc）が --later --until で足す" "$andon" issue ask --new --later "$why2" --until 2999-12-31 --parent "$p2" - <<<"$q2"
c=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
step "ow-ask-later: 足したあとに andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  waiting: $( [ -n "$c" ] && row_of "$c")"
[ -n "$c" ] && jq -e --arg c "$c" --arg w "$why2" '.items[] | select(.id == $c) | .call == false and .later == $w and .later_until == "2999-12-31"' "$tmp/step.out" >/dev/null \
  && ok "ow-ask-later: --later --until で足した $c は、印の理由と呼ぶ日を持つ置いてある行（call: false）になる" \
  || fail "ow-ask-later: --later --until で足した課題が置いてある行にならない"

# ow-slots: オーナー待ちの担当の課題が上限（既定の 2）以上でも、動いている課題が上限より少なければ空きは正になる
for i in 1 2; do
  w=$("$kanban" add "$run の判断を待つ担当の課題 $i" --profile claude)
  step "ow-slots: 担当の端末 tw$i が $w を doing にする" env HC_TERMINAL_ID="tw$i" "$kanban" start "$w"
  step "ow-slots: $w が判断の問いを出す" env HC_TERMINAL_ID="tw$i" "$kanban" ask "$w" "$run の担当 $i の問い" --kind users
done
step "ow-slots: andon waiting list --json を打つ" "$andon" waiting list --json >/dev/null
echo "  slots: $(jq -c .slots "$tmp/step.out" 2>/dev/null)"
jq -e '.slots.waiting >= .slots.limit and .slots.working < .slots.limit and .slots.free == .slots.limit - .slots.working and .slots.free > 0 and (.slots | has("waiting_full") | not)' "$tmp/step.out" >/dev/null \
  && ok "ow-slots: オーナー待ちが上限以上でも、空き（slots.free）は上限から動いている数を引いた正の数で、waiting_full は無い" \
  || fail "ow-slots: オーナー待ちが上限以上で空きが 0 か、waiting_full がある"
step "ow-slots: andon waiting list を打つ（人が読む出力）" "$andon" waiting list
if grep -q '起動しない' "$tmp/step.out"; then fail "ow-slots: 人が読む出力に、オーナー待ちで新しい課題を起動しないの文がある"
elif grep -qE 'の枠: 上限 2、動いている 0、オーナー待ち 2、空き 2$' "$tmp/step.out"; then ok "ow-slots: 人が読む枠の行は空き 2 で、新しい課題を起動しないの文が無い"
else fail "ow-slots: 人が読む枠の行が違う"; fi

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
