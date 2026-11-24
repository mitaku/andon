#!/usr/bin/env bash
# トリアージ（features/triage.md）。切り離した kanban で、担当の報告の「オーナーに聞くこと」を andon triage collect で集め、
# Chief of Staff の端末で andon triage apply を打つ。person の項目が、later の理由なら置いてある行に、false なら呼ぶ行になるかを
# andon waiting --json で読む。Chief of Staff でない端末の印と、later の無い前の形の person は、何も書かずに断るかを見る。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

ledger_init V
export HC_TERMINAL_ID=tc
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
# collect は端末の一覧（残った worktree を見る）を読む。偽の hc の ls は空の一覧を返す
sed -i 's|^exit 0$|[ "$1" = ls ] \&\& echo '"'"'{"terminals":[]}'"'"'\nexit 0|' "$tmp/bin/hc"
# 担当の課題 3 つ。報告の「オーナーに聞くこと」が、トリアージの項目（question）になる
for n in 1 2 3; do
  w=$("$kanban" add "$run の担当の課題 $n" --profile claude)
  printf '## Report\n%s の作業\n## オーナーに聞くこと\n- %s の問い %s\n' "$run" "$run" "$n" | "$kanban" report "$w" >/dev/null
  "$kanban" done "$w" >/dev/null
done
lib() { echo "$tmp/project/kanban/issues/$1/library"; }
# トリアージの課題 2 つ。t1 は今の形の判定、t2 は前の形（person の later が無い・person の無い）の判定を当てる
t1=$("$kanban" add "$run のトリアージ" --profile claude) t2=$("$kanban" add "$run の前の形のトリアージ" --profile claude)
step "tr-collect: andon triage collect を打つ" "$andon" triage collect
jq -c '[.items[] | {key, kind, text}]' "$tmp/step.out" 2>/dev/null | sed 's/^/  items: /'
mkdir -p "$(lib "$t1")" "$(lib "$t2")"
cp "$tmp/step.out" "$(lib "$t1")/items.json" && cp "$tmp/step.out" "$(lib "$t2")/items.json"
jq -e --arg r "$run" '[.items[] | select(.kind == "question" and (.text | contains($r)))] | length == 3' "$tmp/step.out" >/dev/null \
  && ok "tr-collect: 3 つの報告の「オーナーに聞くこと」が、question の項目として出る" \
  || fail "tr-collect: question の項目が 3 つ出ない"
key() { jq -r --arg t "$run の問い $1" '.items[] | select(.kind == "question" and (.text | endswith($t))) | .key' "$(lib "$t1")/items.json"; }
k1=$(key 1) k2=$(key 2) k3=$(key 3)
why="$run 急がない。答えを待つ仕事が無い"
# 今の形: k1 は置いておく判断（later に理由と later_until）、k2 は呼ぶ判断（later: false）、k3 は置いておく手作業（only_owner と later）
jq -n --arg k1 "$k1" --arg k2 "$k2" --arg k3 "$k3" --arg w "$why" --arg r "$run" '{goal: "検証", collected: "2026-11-25T00:00:00Z", items: [
  {key: $k1, decision: "person", reason: "オーナーが要る", kind: "promise", question: "\($r) の公開の約束を結ぶか", options: ["結ぶ", "結ばない"], later: $w, later_until: "2999-12-31"},
  {key: $k2, decision: "person", reason: "オーナーが要る", kind: "promise", question: "\($r) の外の窓口を決めるか", options: ["決める", "決めない"], later: false},
  {key: $k3, decision: "person", reason: "オーナーの手がいる", question: "\($r) の外のサービスの画面で設定を変える", only_owner: "画面はオーナーのアカウントにだけある", later: $w}]}' >"$(lib "$t1")/decisions.json"
# 前の形: person に later が無い。kanban に何も書かずに断る
jq 'del(.items[].later, .items[].later_until)' "$(lib "$t1")/decisions.json" >"$(lib "$t2")/decisions.json"
count() { "$kanban" list --all --json | jq length; }
recs() { find "$tmp/project/kanban/library/triage" -name '*.json' 2>/dev/null | wc -l; } # 1 回の apply は記録の .json と .md を 1 つずつ置く

# tr-old-shape: 前の形の person（later が無い）は、どの端末でも何も書かずに断る。person の無い前の形は、そのまま当たる
n0=$(count) r0=$(recs)
step "tr-old-shape: later の無い前の形の decisions.json を当てる" "$andon" triage apply "$t2" \
  && fail "tr-old-shape: later の無い person を当てた" \
  || { grep -qF 'person に later が無い' "$tmp/step.out" && [ "$(count)" = "$n0" ] && [ "$(recs)" = "$r0" ] \
    && ok "tr-old-shape: later の無い前の形の person は、later が無いと言って断り、課題も記録も増やさない" \
    || fail "tr-old-shape: 断った理由が違うか、断ったのに課題か記録が増えた"; }
jq '{goal, collected, items: [.items[] | {key, decision: "keep", reason: "次回に見る"}]}' "$(lib "$t2")/decisions.json" >"$tmp/keep.json" && cp "$tmp/keep.json" "$(lib "$t2")/decisions.json"
step "tr-old-shape: person の無い前の形（keep だけ）を担当の端末 tw で当てる" env HC_TERMINAL_ID=tw "$andon" triage apply "$t2" \
  && [ "$(count)" = "$n0" ] && [ "$(recs)" = $((r0 + 1)) ] \
  && ok "tr-old-shape: later を持たない前の形の keep だけの判定は、印を書かないので担当の端末でも当たり、記録を 1 つ置く" \
  || fail "tr-old-shape: person の無い前の形の判定が当たらない"

# tr-not-cos: 置いておく印を書く apply は、Chief of Staff でない端末では何も書かずに断る
n0=$(count) r0=$(recs)
step "tr-not-cos: 担当の端末 tw が、印を書く判定を当てる" env HC_TERMINAL_ID=tw "$andon" triage apply "$t1" \
  && fail "tr-not-cos: 担当の端末の apply が印を書いた" \
  || { grep -qF 'later で置いておく印を書くのは Chief of Staff の端末だけ' "$tmp/step.out" && [ "$(count)" = "$n0" ] && [ "$(recs)" = "$r0" ] \
    && ok "tr-not-cos: 担当の端末の apply は、Chief of Staff の端末だけと言って断り、課題も記録も増やさない" \
    || fail "tr-not-cos: 断った理由が違うか、断ったのに課題か記録が増えた"; }

# tr-later・tr-call: Chief of Staff の端末で当てる。理由のある判断と手作業は置いてある行、false の判断は呼ぶ行になる
step "tr-later: Chief of Staff（tc）が当てる" "$andon" triage apply "$t1"
rec=$(ls "$tmp/project/kanban/library/triage/"*-"$t1".json 2>/dev/null | sed -n 1p)
made() { [ -n "$rec" ] && jq -r --arg k "$1" '.items[] | select(.key == $k) | .created_issue // empty' "$rec"; }
i1=$(made "$k1") i2=$(made "$k2") i3=$(made "$k3")
for i in $i1 $i2 $i3; do echo "  | $i: $("$kanban" show "$i" --json | jq -c '.issue | {id, title, who, only_owner, later, later_until}')"; done
step "tr-later: 当てたあとに andon waiting --json を打つ" "$andon" waiting --json >/dev/null
row() { jq -c --arg id "$1" '.items[] | select(.id == $id) | {id, kind, call, later, later_until}' "$tmp/step.out" 2>/dev/null; }
echo "  waiting: $(row "$i1") $(row "$i2") $(row "$i3")"
[ -n "$i1" ] && jq -e --arg id "$i1" --arg w "$why" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].call == false and .[0].later == $w and .[0].later_until == "2999-12-31"' "$tmp/step.out" >/dev/null \
  && ok "tr-later: later に理由のある判断は、オーナーの課題 $i1 が置いてある行（call: false）になり、理由と呼ぶ日を持つ" \
  || fail "tr-later: later に理由のある判断が置いてある行にならない"
[ -n "$i3" ] && jq -e --arg id "$i3" --arg w "$why" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "task" and .[0].call == false and .[0].later == $w' "$tmp/step.out" >/dev/null \
  && ok "tr-later: later に理由のある手作業は、オーナーの課題 $i3 が置いてある行（call: false）になる" \
  || fail "tr-later: later に理由のある手作業が置いてある行にならない"
[ -n "$i2" ] && jq -e --arg id "$i2" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].call == true and (.[0] | has("later") | not)' "$tmp/step.out" >/dev/null \
  && ! "$kanban" show "$i2" --json | jq -e '.issue | has("later") or has("later_until")' >/dev/null \
  && ok "tr-call: later が false の判断は、オーナーの課題 $i2 が印を持たず、呼ぶ行（call: true）になる" \
  || fail "tr-call: later が false の判断が呼ぶ行にならないか、印を持つ"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
