#!/usr/bin/env bash
# PR を止める（features/hold.md）。切り離した kanban で、担当が報告した PR を Chief of Staff が止め（andon issue hold）、
# 止めている間 gate が待つこと、担当が hold を消しても止まったままのこと、Chief of Staff が外す（andon issue unhold）ことを通す。
# 偽の gh は止める口が打つ2つの呼び出しだけを知るものに替える（sandbox.sh の偽の gh はこの2つを知らない）:
#   pr view <n> --repo <owner>/<repo> --json state,mergeCommit,headRefOid,statusCheckRollup は $tmp/gh/view、
#   api -X POST repos/<owner>/<repo>/statuses/<sha> … は記録して 0。ほかは記録して 99。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
echo "gh \$*" >>"$tmp/gh.calls"
case "\$1 \$2" in
  "pr view") [ "\$7" = state,mergeCommit,headRefOid,statusCheckRollup ] && exec cat "$tmp/gh/view" ;;
  "api -X") [ "\$3" = POST ] && exit 0 ;;
esac
echo "偽の gh: 知らない呼び出し \$*" >&2
exit 99
EOF
chmod +x "$tmp/bin/gh"

# 担当の worktree と PR。PR の head の印 andon/gate は緑（gate が今の commit を渡した）
git init -q "$tmp/wt" && echo one >"$tmp/wt/f" && git -C "$tmp/wt" add f && git -C "$tmp/wt" commit -qm one
head=$(git -C "$tmp/wt" rev-parse HEAD)
pr=https://github.com/example/sample-site/pull/7
jq -n --arg h "$head" '{state: "OPEN", headRefOid: $h, mergeCommit: null,
  statusCheckRollup: [{__typename: "StatusContext", context: "andon/gate", state: "SUCCESS", description: "andon gate: approve"}]}' >"$tmp/gh/view"

ledger_init V
id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の止める PR" --profile claude)
why=$(HC_TERMINAL_ID=tc "$kanban" add "$run のオーナーの答えを待つ問い")
gone=$(HC_TERMINAL_ID=tc "$kanban" add "$run の片付いた問い")
HC_TERMINAL_ID=tc "$kanban" set "$id" review=off must_fix=major merge=person \
  "worktree=$tmp/wt" repo=github.com/example/sample-site >/dev/null
# 担当の端末 tw が受け持つ（kanban start が agent に tw を書く。止めの読み方は agent に書かれた端末を担当と読む）
HC_TERMINAL_ID=tw "$kanban" start "$id" >/dev/null || fail "担当が kanban start で受け持てない（前提）"
HC_TERMINAL_ID=tc "$kanban" done "$gone" >/dev/null
lib=$tmp/project/kanban/issues/$id/library
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
echo "{\"labels\":{\"issue\":\"$id\"}}" >"$tmp/hc/tw.json"
hold_of() { "$kanban" show "$id" --json | jq -r '.issue.hold // ""'; }

# 担当が報告し（PR の URL が report.json に入る）、Chief of Staff が review に入れた状態にする
HC_TERMINAL_ID=tw "$andon" issue report "$id" --pr "$pr" <<<"## Report
$run の止める PR" >/dev/null || fail "担当の報告が書けない（前提）"
HC_TERMINAL_ID=tc "$kanban" set "$id" status=review >/dev/null
export HC_TERMINAL_ID=tc

# hd-refuse: 理由の課題が自分自身・片付いた課題なら、何も書かずに断る
step "hd-refuse: 理由の課題に $id 自身を渡す" "$andon" issue hold "$id" "$id" \
  && fail "hd-refuse: 自分自身を理由にした hold が通った" \
  || { [ -z "$(hold_of)" ] && ok "hd-refuse: 自分自身を理由にすると断り、hold を書かない" || fail "hd-refuse: 断ったのに hold を書いた"; }
step "hd-refuse: 片付いた課題 $gone を理由に渡す" "$andon" issue hold "$id" "$gone" \
  && fail "hd-refuse: 片付いた課題を理由にした hold が通った" \
  || { [ -z "$(hold_of)" ] && ok "hd-refuse: 片付いた課題を理由にすると断り、hold を書かない" || fail "hd-refuse: 断ったのに hold を書いた"; }

# hd-hold: hold に理由の課題を書き、緑の印の commit に黄（pending）を書いてから返る。別の口（kanban show・log.jsonl）で読み直す
: >"$tmp/gh.calls"
step "hd-hold: Chief of Staff が $id を止める（理由の課題 $why）" "$andon" issue hold "$id" "$why" \
  && [ "$(hold_of)" = "$why" ] \
  && grep -qF "\"by\":\"terminal:tc\",\"id\":\"$id\",\"change\":{\"hold\":\"$why\"}" "$tmp/project/kanban/log.jsonl" \
  && grep -qxF "gh api -X POST repos/example/sample-site/statuses/$head -f context=andon/gate -f state=pending -f description=andon gate: メンテナーの確認を待っています" "$tmp/gh.calls" \
  && ok "hd-hold: hold=$why が Chief of Staff の端末の行で残り、PR の head に黄の印を書いた" \
  || fail "hd-hold: hold が無いか、黄の印を書いていない"
echo "  kanban show: $("$kanban" show "$id" --json | jq -c '.issue | {id, status, agent, hold}')"
sed 's/^/  gh.calls: /' "$tmp/gh.calls"

# hd-gate-wait: 止めている間、gate は GitHub を読まずに wait を返し、記録を書かない
: >"$tmp/gh.calls"
step "hd-gate-wait: 止めている課題に andon gate を打つ" "$andon" gate "$id"
rc=$?
[ "$rc" = 5 ] && jq -e --arg w "$why" '.decision == "wait" and (.reasons[0] | contains($w))' "$tmp/step.out" >/dev/null \
  && [ ! -s "$tmp/gh.calls" ] && ! compgen -G "$lib/gate/*.json" >/dev/null \
  && ok "hd-gate-wait: gate は終了コード 5 の wait で理由の課題 $why を名指し、GitHub を読まず記録も書かない" \
  || fail "hd-gate-wait: 止めているのに gate が wait でないか、GitHub を読んだか、記録を書いた"

# hd-worker: 担当（agent の端末 tw）が hold を消しても止めたまま（gate は wait のまま）
HC_TERMINAL_ID=tw "$kanban" set "$id" hold= >/dev/null
echo "  担当 tw が kanban set $id hold= を打った。今の hold: '$(hold_of)'"
: >"$tmp/gh.calls"
step "hd-worker: 担当が消したあとに andon gate を打つ" "$andon" gate "$id"
rc=$?
[ "$rc" = 5 ] && jq -e --arg w "$why" '.decision == "wait" and (.reasons[0] | contains($w))' "$tmp/step.out" >/dev/null && [ ! -s "$tmp/gh.calls" ] \
  && ok "hd-worker: 担当が消した hold は止めたままと読み、gate は wait のまま" \
  || fail "hd-worker: 担当が hold を消したら gate が止めを外した"

# hd-unhold: Chief of Staff が外す。担当が消した止めも外せて、印は書かない。2回目は止めていないと返す
: >"$tmp/gh.calls"
step "hd-unhold: Chief of Staff が止めを外す" "$andon" issue unhold "$id" \
  && grep -qF "止めを外した（理由の課題 $why）" "$tmp/step.out" && [ ! -s "$tmp/gh.calls" ] \
  && jq -se --arg id "$id" '[.[] | select(.id == $id and (.change | has("hold")))] | last | .by == "terminal:tc" and (.change.hold // "") == ""' \
    "$tmp/project/kanban/log.jsonl" >/dev/null \
  && ok "hd-unhold: 担当が消した止めも理由の課題 $why ごと外し、印は書かない" \
  || fail "hd-unhold: 止めを外せないか、印を書いた"
step "hd-unhold: もう一度外す" "$andon" issue unhold "$id" \
  && grep -qF "止めていない" "$tmp/step.out" \
  && ok "hd-unhold: 外したあとは止めていないと返す" || fail "hd-unhold: 外したあとも止めていると読む"
jq -c --arg id "$id" 'select(.id == $id and (.change | has("hold"))) | {by, change}' "$tmp/project/kanban/log.jsonl" | sed 's/^/  log.jsonl: /'

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件"
sed 's/^/  /' "$tmp/hc.calls"
[ "$fails" = 0 ]
