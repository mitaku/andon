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
# 片付いた課題の結果の記録の内訳（dispatcher が書く形。ここでは手で置く）と、Chief of Staff の役ごとの生の記録 1 日分（今日の日付。ms-breakdown）
mkdir -p "$tmp/project/kanban/issues/$done_id/library"
cat >"$tmp/project/kanban/issues/$done_id/library/outcome.json" <<EOF
{"issue": "$done_id", "breakdown": {"role": "worker", "coverage": {"sessions": 1, "traced": 1}, "interactions": 1, "llm_ms": 100, "tool_ms": 50, "blocked_ms": 0,
 "tokens": {"input": 1, "output": 2, "cache_read": 3, "cache_write": 4}, "errors": {"llm": 0, "tool": 0},
 "tools": [{"name": "Bash", "argv0": "go", "count": 1, "ms": 50, "errors": 0}, {"name": "Read", "argv0": null, "count": 3, "ms": 20, "errors": 0}],
 "waits": {"question_ms": 1000, "review_ms": 2000, "ci_ms": 3000, "gone_ms": 4000},
 "knowledge": {"index_reads": 1, "pages": [{"path": "topics/shell.md", "reads": 2}]}}}
EOF
# 知識の節（ms-knowledge）: 片付いた課題の依頼書に wiki のページ、報告に同じ先への申告 2 件。wiki の置き場は一時のセルの repo に向ける
# （セルの本物の wiki を読まない）。読まれたページ 1 枚と、読まれなかった Topics と Entities のページ 1 枚ずつ
export ANDON_WORKSPACE=$tmp/workspace
mkdir -p "$ANDON_WORKSPACE/wiki/topics" "$ANDON_WORKSPACE/wiki/entities"
for p in index.md topics/shell.md topics/ci.md entities/owner.md; do echo '# x' >"$ANDON_WORKSPACE/wiki/$p"; done
echo '先に wiki/topics/shell.md を読む。' >>"$tmp/project/kanban/issues/$done_id/brief.md"
echo '{"gaps": [{"kind": "missing", "where": "AGENTS.md", "text": "x"}, {"kind": "wrong", "where": "AGENTS.md", "text": "y"}]}' \
  >"$tmp/project/kanban/issues/$done_id/library/report.json"
# 片付いた課題のレビュアーの課題（結果の記録が無い）。流れにもプロファイルにも数えないが、内訳の無い課題に数え、親の周回になる
rev_id=$(HC_TERMINAL_ID=tc "$kanban" add "$run のレビュー" --profile codex-review --parent "$done_id" --round 1)
for s in doing review done; do HC_TERMINAL_ID=tc "$kanban" set "$rev_id" "status=$s" >/dev/null; done
otel=$HOME/.local/state/andon/otel/project/roles/chief-of-staff
mkdir -p "$otel"
echo '{"resourceSpans":[{"resource":{"attributes":[]},"scopeSpans":[{"spans":[{"traceId":"v","spanId":"1","name":"claude_code.llm_request","startTimeUnixNano":"1","endTimeUnixNano":"2","attributes":[{"key":"input_tokens","value":{"intValue":"5"}},{"key":"output_tokens","value":{"intValue":"6"}},{"key":"cache_read_tokens","value":{"intValue":"7"}},{"key":"cache_creation_tokens","value":{"intValue":"8"}}]}]}]}]}' \
  >"$otel/$(date -u +%F).traces.jsonl"
sums() { find "$tmp/project" "$HOME/.claude" "$HOME/.local/state/andon/otel" -type f -exec sha256sum {} + | sort; }
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

# ms-breakdown: 内訳の節が片付いた課題の outcome.json の breakdown を役ごとに合わせ、Chief of Staff の分を役ごとの生の記録から数える。
# プロファイルごとの節に片付いた担当の課題が出る
jq -c '{breakdown, profiles}' "$tmp/step.out" 2>/dev/null | sed 's/^/  breakdown: /'
jq -e '.breakdown.issues == 2 and .breakdown.none == 1
  and .breakdown.waits == {"issues": 1, "question_ms": 1000, "review_ms": 2000, "ci_ms": 3000, "gone_ms": 4000}
  and .breakdown.roles == [{"role": "worker", "issues": 1, "token_issues": 1, "tokens": {"input": 1, "output": 2, "cache_read": 3, "cache_write": 4}, "cache_read_rate": 0.38, "errors": {"llm": 0, "tool": 0}}]
  and (.breakdown.tools | map(.name)) == ["Bash", "Read"]
  and .breakdown.chief_of_staff == {"days": 1, "tokens": {"input": 5, "output": 6, "cache_read": 7, "cache_write": 8}, "share": 0.72}
  and (.profiles | map({profile, issues, tokens, review_rounds})) == [{"profile": "claude", "issues": 1, "tokens": {"count": 1, "median": 10}, "review_rounds": {"count": 1, "median": 1}}]' \
  "$tmp/step.out" >/dev/null \
  && ok "ms-breakdown: $done_id の内訳を担当の行と待ちとツールに合わせ、結果の記録の無い $rev_id を内訳の無い課題に数え、Chief of Staff の 1 日分を数え、プロファイル claude に 1 件（周回 1）を出す" \
  || fail "ms-breakdown: 内訳の節かプロファイルごとの節の数が置いた記録と違う"

# ms-knowledge: 知識の節が、片付いた担当の課題の依頼書・内訳の読み・申告を数え、読まれなかったページを wiki の置き場から並べる
jq -c '.knowledge' "$tmp/step.out" 2>/dev/null | sed 's/^/  knowledge: /'
jq -e --arg d "$done_id" '.knowledge == {"issues": 1, "briefed": 1, "read": {"issues": 1, "index_only": 0, "no_record": 0},
    "pages": [{"path": "topics/shell.md", "reads": 2, "issues": 1}], "unread": ["entities/owner.md", "topics/ci.md"],
    "gaps": {"count": 2, "issues": 1, "by_kind": {"missing": 1, "wrong": 1, "hard_to_find": 0},
      "wheres": [{"where": "AGENTS.md", "count": 2, "issues": [$d]}], "repeated": [{"where": "AGENTS.md", "count": 2, "issues": [$d]}]}}' \
  "$tmp/step.out" >/dev/null \
  && ok "ms-knowledge: $done_id を依頼書に載せた課題と読んだ課題に数え、topics/shell.md の 2 回、読まれなかった 2 枚、AGENTS.md への申告の繰り返しを出す" \
  || fail "ms-knowledge: 知識の節の数が置いた記録と違う"

# ms-table: 人向けの表は価値の行で始まる
step "ms-table: andon measure を打つ" "$andon" measure \
  && sed -n 1p "$tmp/step.out" | grep -q '^価値: ' \
  && grep -q '^内訳（' "$tmp/step.out" && grep -q '^  片付いた課題  *2件・内訳の無い課題 1件$' "$tmp/step.out" \
  && grep -q '^  chief-of-staff 1日分 ' "$tmp/step.out" && grep -q '^プロファイル（' "$tmp/step.out" && grep -q '^  claude  *1件・実働 ' "$tmp/step.out" \
  && grep -q '^知識（' "$tmp/step.out" && grep -q '^    AGENTS.md 2件（繰り返し） ' "$tmp/step.out" \
  && ok "ms-table: 人向けの表が「価値:」の行で始まり、内訳の節とプロファイルの節と知識の節が出る" || fail "ms-table: 表が出ないか、先頭が価値の行でないか、内訳かプロファイルか知識の節が無い"

# ms-readonly: 打つ前と後で、kanban とセッションの記録が1バイトも変わらない
[ "$before" = "$(sums)" ] && ok "ms-readonly: measure の前後で kanban とセッションの記録が変わらない" \
  || fail "ms-readonly: measure が kanban かセッションの記録を書き換えた"

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
