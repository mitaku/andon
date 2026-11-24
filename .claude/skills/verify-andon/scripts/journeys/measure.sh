#!/usr/bin/env bash
# 流れの数え（features/measure.md）。切り離した kanban に数の分かっている履歴を作り、オーナーと同じく andon measure を打つ。
# 読むだけの command なので、打つ前と後で kanban とセッションの記録が変わらないことも確かめる。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

ledger_init V
# 片付いた課題 1 件（todo → doing → review → done）と、doing のまま残る課題 1 件
done_id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の片付いた課題" --profile claude)
for s in doing review done; do HC_TERMINAL_ID=tc "$kanban" set "$done_id" "status=$s" >/dev/null; done
wip_id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の途中の課題" --profile claude)
HC_TERMINAL_ID=tc "$kanban" set "$wip_id" status=doing >/dev/null
# 片付いた課題の担当のセッションの記録（Claude Code の置き場。HOME は一時）。トークンは入力 11・出力 22・キャッシュの読み 33・書き 44
session=$(cat /proc/sys/kernel/random/uuid)
HC_TERMINAL_ID=tc "$kanban" set "$done_id" "session=$session" >/dev/null
mkdir -p "$HOME/.claude/projects/verify"
echo '{"type":"assistant","requestId":"req_1","message":{"id":"msg_1","model":"claude-verify","usage":{"input_tokens":11,"output_tokens":22,"cache_read_input_tokens":33,"cache_creation_input_tokens":44}}}' \
  >"$HOME/.claude/projects/verify/$session.jsonl"
# オーナーの判断 2 件: Chief of Staff（tc）が呼ぶもの（--now）と、置いておくもの（--later）を足す。問いはどちらも開いたまま（ms-question-later）
echo '{"labels":{"role":"chief-of-staff"}}' >"$tmp/hc/tc.json"
q="{\"kind\": \"users\", \"question\": \"$run の書き出しは CSV だけにするか\", \"options\": [{\"label\": \"CSV だけ\", \"pick\": true}, {\"label\": \"Excel も\"}], \"from\": \"chief-of-staff\"}"
called=$(HC_TERMINAL_ID=tc "$andon" issue ask --new --now - <<<"$q" | sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p')
placed=$(HC_TERMINAL_ID=tc "$andon" issue ask --new --later "$run 急がない" - <<<"${q/CSV だけにするか/並びを変えるか}" | sed -n 's/^\([A-Z]*-[0-9]*\)#1$/\1/p')
echo "  | 呼ぶ判断: ${called:-無い}。置いておく判断: ${placed:-無い}"
sums() { find "$tmp/project" "$HOME/.claude" -type f -exec sha256sum {} + | sort; }
before=$(sums)

# ms-flow: 期間の流れが履歴どおり数えられる（片付いた 1・残った WIP 1）
step "ms-flow: andon measure --json を打つ" "$andon" measure --json
jq -c '{project, since, until, closed: .flow.closed, wip_end: .flow.wip_end}' "$tmp/step.out" 2>/dev/null | sed 's/^/  measure: /'
jq -e '.flow.closed.count == 1 and .flow.wip_end == 1' "$tmp/step.out" >/dev/null \
  && ok "ms-flow: 片付いた課題 1・期間の終わりの WIP 1 を数える" \
  || fail "ms-flow: 流れの数が履歴と違うか、JSON が読めない"

# ms-question-later: 問いが開いていた時間の平均は、置いておける課題の問いを入れない（開いている問い 2 つのうち 1 つだけ数える）
jq -c '.flow.question_open' "$tmp/step.out" 2>/dev/null | sed 's/^/  question_open: /'
if [ -n "$called" ] && [ -n "$placed" ] && jq -e '.flow.question_open.count == 1' "$tmp/step.out" >/dev/null; then
  ok "ms-question-later: 呼ぶ $called の問いだけを数え、置いた $placed の問いを入れない"
else
  fail "ms-question-later: 判断を足せなかったか、置いた判断の問いを数えた"
fi

# ms-tokens: 片付いた課題の担当のセッションの記録から、トークンを数える。記録の無い課題は null
jq -c '.tokens[]' "$tmp/step.out" 2>/dev/null | sed 's/^/  tokens: /'
jq -e --arg d "$done_id" --arg w "$wip_id" \
  'any(.tokens[]; .issue == $d and .usage == {"input": 11, "output": 22, "cache_read": 33, "cache_write": 44}) and any(.tokens[]; .issue == $w and .usage == null)' \
  "$tmp/step.out" >/dev/null \
  && ok "ms-tokens: $done_id のトークンをセッションの記録のとおり数え、記録の無い $wip_id は null" \
  || fail "ms-tokens: トークンの数が記録と違うか、記録の無い課題が null でない"

# ms-table: 人向けの表は価値の行で始まる
step "ms-table: andon measure を打つ" "$andon" measure \
  && sed -n 1p "$tmp/step.out" | grep -q '^価値: ' \
  && ok "ms-table: 人向けの表が「価値:」の行で始まる" || fail "ms-table: 表が出ないか、先頭が価値の行でない"

# ms-readonly: 打つ前と後で、kanban とセッションの記録が1バイトも変わらない
[ "$before" = "$(sums)" ] && ok "ms-readonly: measure の前後で kanban とセッションの記録が変わらない" \
  || fail "ms-readonly: measure が kanban かセッションの記録を書き換えた"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
