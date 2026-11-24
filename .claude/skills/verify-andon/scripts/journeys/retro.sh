#!/usr/bin/env bash
# 振り返り（features/retro.md）。切り離した kanban で、振り返りの担当の端末 tw が andon retro patterns で一覧を書き、
# Chief of Staff の端末 tc が andon retro apply を打つ。この kanban の repos に Andon が無いので、Andon の直しは
# 「Andon へ伝えるか」のオーナーの課題になる。その課題が印 later を持ち、andon waiting --json で置いてある行になるかを読む。
# 前のバージョンの apply が who: person だけ書いて落ちた課題に、打ち直しで印を書くかと、Chief of Staff でない端末を断るかも見る。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

ledger_init V
export HC_TERMINAL_ID=tc
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
# 根拠になる、片付いた担当の課題 2 つ
e1=$("$kanban" add "$run の担当の課題 1" --profile claude) e2=$("$kanban" add "$run の担当の課題 2" --profile claude)
"$kanban" done "$e1" >/dev/null && "$kanban" done "$e2" >/dev/null
# 振り返りの課題。担当の端末 tw が動かしている
r=$("$kanban" add "$run の振り返り" --profile claude)
"$kanban" set "$r" routine=retro >/dev/null
HC_TERMINAL_ID=tw "$kanban" start "$r" >/dev/null
lib=$tmp/project/kanban/issues/$r/library
s1="$run 見送りの理由が報告に無く、検収で戻した" s2="$run 依頼書の完了条件が二通りに読めた"
why='振り返りの直しの提案で、答えを待って止まる仕事が無い'
pat() { # pat <id> <summary> <decision>: Andon の直しのパターン1つ
  jq -cn --arg id "$1" --arg s "$2" --arg d "$3" --arg e1 "$e1" --arg e2 "$e2" '{id: $id, summary: $s, evidence: [$e1, $e2], level: "test", action: "add",
    target: {kind: "andon", paths: ["tests/report-shape.sh"]}, effect: "同じ抜けが CI で落ちる", risk: "誤検知", decision: $d, reason: "検査で止められる"}'
}
jq -n --argjson p1 "$(pat p1 "$s1" fix)" --argjson p2 "$(pat p2 "$s2" fix)" --argjson p3 "$(pat p3 "$run 見込みより長くかかった" drop)" \
  '{patterns: [$p1, $p2, $p3], fixes: []}' >"$tmp/patterns.json"

# rt-patterns: 振り返りの担当が一覧を書く
step "rt-patterns: 担当の端末 tw が andon retro patterns を打つ" env HC_TERMINAL_ID=tw "$andon" retro patterns "$r" - <"$tmp/patterns.json"
[ -f "$lib/retro.json" ] && jq -e --arg r "$r" '.retro == $r and ([.patterns[] | [.id, .decision]] == [["p1", "fix"], ["p2", "fix"], ["p3", "drop"]])' "$lib/retro.json" >/dev/null \
  && ok "rt-patterns: $r/library/retro.json に、fix 2 件と drop 1 件のパターンの一覧が置かれる" \
  || fail "rt-patterns: retro.json が無いか、形が違う"

# 読めなければ毎回違う値を返し、「課題の数が変わらない」の比べが素通りしない
count() { "$kanban" list --all --json | jq -e length 2>/dev/null || echo "読めない-$RANDOM"; }
# rt-not-cos: オーナーの課題を足す apply は、Chief of Staff でない端末では何も書かずに断る
n0=$(count)
step "rt-not-cos: 担当の端末 tw が andon retro apply を打つ" env HC_TERMINAL_ID=tw "$andon" retro apply "$r" \
  && fail "rt-not-cos: 担当の端末の apply が通った" \
  || { grep -qF '置いておく印を書くのは Chief of Staff の端末だけ' "$tmp/step.out" && [ "$(count)" = "$n0" ] && [ ! -e "$lib/retro-applied.json" ] \
    && ok "rt-not-cos: 担当の端末の apply は、Chief of Staff の端末だけと言って断り、課題も当てた記録も増やさない" \
    || fail "rt-not-cos: 断った理由が違うか、断ったのに課題か当てた記録が増えた"; }

# 前のバージョンの apply が p2 の課題を足し、who: person だけ書いて、問いを置く前に落ちた形を作る
old=$("$kanban" add "Andon へ伝えるか: $s2" --parent "$r")
"$kanban" set "$old" who=person >/dev/null
step "rt-old-owner: 打ち直しの前に andon waiting --json を打つ" "$andon" waiting --json >/dev/null
row() { jq -c --arg id "$1" '.items[] | select(.id == $id) | {id, kind, call, later}' "$tmp/step.out" 2>/dev/null; }
echo "  waiting: $(row "$old")"
jq -e --arg id "$old" '[.items[] | select(.id == $id)] | length == 1 and .[0].call == true' "$tmp/step.out" >/dev/null \
  && ok "rt-old-owner: 打ち直しの前、前のバージョンが who: person だけ書いた課題 $old は呼ぶ行（call: true）" \
  || fail "rt-old-owner: 打ち直しの前の課題 $old が呼ぶ行でない（前のバージョンの形を作れていない）"

# rt-later・rt-old-owner: Chief of Staff の端末で当てる
n0=$(count)
step "rt-later: Chief of Staff（tc）が andon retro apply を打つ" "$andon" retro apply "$r"
made() { jq -r --arg p "$1" '.issues[] | select(.pattern == $p) | .issue' "$lib/retro-applied.json" 2>/dev/null; }
i1=$(made p1) i2=$(made p2)
for i in $i1 $i2; do echo "  | $i: $("$kanban" show "$i" --json | jq -c '{issue: (.issue | {id, title, parent, who, later}), questions: [.questions[] | {status, kind}]}')"; done
step "rt-later: 当てたあとに andon waiting --json を打つ" "$andon" waiting --json >/dev/null
echo "  waiting: $(row "$i1") $(row "$i2")"
[ -n "$i1" ] && [ "$i1" != "$old" ] && "$kanban" show "$i1" --json | jq -e --arg r "$r" --arg w "$why" '.issue.who == "person" and .issue.parent == $r and .issue.later == $w and (.questions | length == 1)' >/dev/null \
  && jq -e --arg id "$i1" --arg w "$why" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].call == false and .[0].later == $w' "$tmp/step.out" >/dev/null \
  && ok "rt-later: p1 の「Andon へ伝えるか」の課題 $i1 は、決まった理由の印 later と問いを持ち、置いてある行（call: false）になる" \
  || fail "rt-later: p1 のオーナーの課題が印を持たないか、置いてある行にならない"
[ "$i2" = "$old" ] && [ "$(count)" = $((n0 + 1)) ] && "$kanban" show "$old" --json | jq -e --arg w "$why" '.issue.who == "person" and .issue.later == $w and (.questions | length == 1)' >/dev/null \
  && jq -e --arg id "$old" --arg w "$why" '[.items[] | select(.id == $id)] | length == 1 and .[0].call == false and .[0].later == $w' "$tmp/step.out" >/dev/null \
  && ok "rt-old-owner: 前のバージョンが who: person だけ書いた課題 $old を足し直さずに使い、印 later と問いを書いて置いてある行にする" \
  || fail "rt-old-owner: 前のバージョンの課題に印が無いか、課題を足し直した（p2: ${i2:-無い}・前の課題: $old）"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
