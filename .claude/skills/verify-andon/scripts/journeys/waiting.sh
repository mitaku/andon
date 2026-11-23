#!/usr/bin/env bash
# オーナー待ち（features/waiting.md）。切り離した kanban で、Chief of Staff がオーナーの課題を足し（andon issue ask --new）、
# オーナーと同じく andon waiting で判断の行として読む。
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
step "ow-ask-new: 親 $p1 の下にオーナーの課題を足す" "$andon" issue ask --new --parent "$p1" - <<<"$q"
a=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
echo "  | ${a:-無い}: $( [ -n "$a" ] && owner_of "$a")"
[ -n "$a" ] && "$kanban" show "$a" --json | jq -e --arg p "$p1" '.issue.who == "person" and .issue.parent == $p and (.issue.title | endswith("（オーナーの判断待ち）"))' >/dev/null \
  && jq -e '.kind == "users" and .from == "chief-of-staff"' "$tmp/project/kanban/issues/$a/library/questions/1.json" >/dev/null \
  && ok "ow-ask-new: オーナーの課題 $a が who: person・親 $p1 で足され、問い 1 が library/questions/1.json に置かれる" \
  || fail "ow-ask-new: オーナーの課題か問いが無い・形が違う"

# ow-same-title: 同じ問いでも親が違えば別のオーナーの課題になる。同じ親で打ち直すと断り、課題を増やさない
step "ow-same-title: 同じ問いを親 $p2 の下に足す" "$andon" issue ask --new --parent "$p2" - <<<"$q"
b=$(sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p' "$tmp/step.out")
echo "  | ${b:-無い}: $( [ -n "$b" ] && owner_of "$b")"
[ -n "$b" ] && [ "$b" != "$a" ] && [ "$("$kanban" show "$b" --json | jq -r .issue.parent)" = "$p2" ] \
  && ok "ow-same-title: 親の違う同じタイトルは取り違えず、別のオーナーの課題 $b になる" \
  || fail "ow-same-title: 親の違う同じ問いが別の課題にならない"
n0=$("$kanban" list --json | jq length)
step "ow-same-title: 同じ問いを親 $p1 の下に打ち直す" "$andon" issue ask --new --parent "$p1" - <<<"$q" \
  && fail "ow-same-title: 問いを持つ同じオーナーの課題があるのに通った" \
  || { [ "$("$kanban" list --json | jq length)" = "$n0" ] \
    && ok "ow-same-title: 同じ親の同じ問いは断り、課題を増やさない" || fail "ow-same-title: 断ったのに課題が増えた"; }

# ow-waiting: オーナー待ちに、2つの課題が判断の行として出る（答えられる問い）
step "ow-waiting: andon waiting --json を打つ" "$andon" waiting --json >/dev/null
jq -c '{slots, items: [.items[] | {kind, id, question, answerable}]}' "$tmp/step.out" 2>/dev/null | sed 's/^/  waiting: /'
jq -e --arg a "$a" --arg b "$b" '[.items[] | select(.kind == "decision" and .answerable == true) | .id] | sort == ([$a, $b] | sort)' "$tmp/step.out" >/dev/null \
  && ok "ow-waiting: オーナー待ちに $a と $b が答えられる判断の行として出る" \
  || fail "ow-waiting: 判断の行が足した2つの課題と違う"
step "ow-waiting: andon waiting を打つ（人が読む出力）" "$andon" waiting \
  && grep -qF "$a" "$tmp/step.out" && ok "ow-waiting: 人が読む出力にも $a が出る" || fail "ow-waiting: 人が読む出力に $a が無い"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
