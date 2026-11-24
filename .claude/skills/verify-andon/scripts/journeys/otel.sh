#!/usr/bin/env bash
# ハーネスの受け口（features/otel.md）。受け口が listen しているかと、当てたあとに起動した Claude Code の担当・レビュアーの
# 課題の置き場（~/.local/state/andon/otel/<slug>/issues/<id>/traces.jsonl）を読む。読むだけ。何も起動しない・止めない。
# 当てたあとに Claude Code で起動した課題がまだ無いか、どの課題もまだ記録を持たなければ 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"
root=${XDG_STATE_HOME:-$HOME/.local/state}/andon/otel

# ot-listen: 127.0.0.1:4318 が andon の受け口の名前を返す（今の状態）
name=$(curl -s -m 2 http://127.0.0.1:4318/ 2>/dev/null | tr -d '\r\n') || name=
echo "-- ot-listen: GET http://127.0.0.1:4318/ → ${name:-返らない}"
[ "$name" = andon-otel ] && ok "ot-listen: 受け口が listen し、名前 andon-otel を返す" || fail "ot-listen: 受け口が listen していないか、ほかのプロセスがポートを取っている"

# ot-codex: 当てたあとに Codex のプロファイルで起動した課題ごとに、置き場の traces.jsonl と logs.jsonl を読む（ADR-0060 受け入れ条件 17）。
# 記録が無いことは報告に書く条件なので、note にして道の終了コードを変えない。個人の属性かツールの出力が残っていれば fail。
cx=0
while IFS=$'\t' read -r kanban id; do
  [ -n "$id" ] || continue
  slug=$(basename "$(dirname "$kanban")")
  for kind in traces logs; do
    f=$root/$slug/issues/$id/$kind.jsonl
    [ -f "$f" ] || { echo "  | $slug/$id: Codex の $kind.jsonl が無い"; continue; }
    tied=$(jq -c '(.resourceSpans // .resourceLogs)[0].resource.attributes | map(.key) | select(index("andon.issue") != null and index("andon.role") != null)' "$f" 2>/dev/null | grep -c . || true)
    email=$(grep -c '"user.email"' "$f" || true)
    output=$(jq -c '.. | objects | select(.attributes? and ((.attributes | map(select(.key == "event.name")) | .[0].value.stringValue) == "codex.tool_result" or .eventName? == "codex.tool_result")) | .attributes[] | select(.key == "output")' "$f" 2>/dev/null | grep -c . || true)
    echo "  | $slug/$id: Codex の $kind.jsonl の andon.issue・andon.role を持つ行 $tied・user.email を持つ行 $email・codex.tool_result の output $output"
    [ "$email" = 0 ] || fail "ot-codex: $f に user.email が残っている"
    [ "$output" = 0 ] || fail "ot-codex: $f に codex.tool_result の output が残っている"
    [ "$tied" -gt 0 ] && cx=$((cx + 1))
  done
done < <(for f in "$ws"/projects/*/kanban/issues/*/library/launches.jsonl; do
  since "$f" | jq -r --arg k "${f%/issues/*}" 'select(.profile | test("codex")) | [$k, .issue] | @tsv'; done | sort -u)
if [ "$cx" -gt 0 ]; then
  ok "ot-codex: 当てたあとに Codex で起動した課題の記録 $cx 本が課題に結ばれ、user.email と codex.tool_result の output が無い"
else
  echo "note ot-codex: 当てたあとに Codex で起動した課題の記録が無い（Codex の起動が無いか、送っていない・結べていない）。受け入れ条件 17 のとおり、その事実を報告に書く"
fi

# ot-records: 当てたあとに Claude Code のプロファイル（codex でない）で起動した課題ごとに、置き場の traces.jsonl を読む
lines=$(since "$ws"/projects/*/kanban/issues/*/library/launches.jsonl | jq -c 'select(.profile | test("codex") | not)')
n=$(grep -c . <<<"$lines" || true)
echo "-- ot-records: 当てたあとに Claude Code で起動した課題の起動 $n 件"
[ "$n" -gt 0 ] || { echo "note 当てたあとに Claude Code の担当もレビュアーも起動していない。Chief of Staff が次に課題を起動したあとで打ち直す"; exit 3; }
seen=0
while IFS=$'\t' read -r kanban id; do
  [ -n "$id" ] || continue
  slug=$(basename "$(dirname "$kanban")")
  f=$root/$slug/issues/$id/traces.jsonl
  [ -f "$f" ] || { echo "  | $slug/$id: 記録が無い（まだターンが無いか、受け口が止まっていた）"; continue; }
  llm=$(jq -c 'select(.resourceSpans[0] | (.resource.attributes | map(.key) | (index("andon.issue") != null and index("andon.role") != null))
    and ([.scopeSpans[].spans[].name] | index("claude_code.llm_request") != null))' "$f" 2>/dev/null | grep -c . || true)
  email=$(grep -c '"user.email"' "$f" || true)
  echo "  | $slug/$id: claude_code.llm_request と andon.issue・andon.role を持つ行 $llm・user.email を持つ行 $email"
  [ "$email" = 0 ] || fail "ot-records: $f に user.email が残っている"
  [ "$llm" -gt 0 ] && seen=$((seen + 1))
done < <(for f in "$ws"/projects/*/kanban/issues/*/library/launches.jsonl; do
  since "$f" | jq -r --arg k "${f%/issues/*}" 'select(.profile | test("codex") | not) | [$k, .issue] | @tsv'; done | sort -u)
[ "$fails" = 0 ] || exit 1
[ "$seen" -gt 0 ] || { echo "note 当てたあとに起動した課題のどれにも、まだ claude_code.llm_request の記録が無い。担当が 1 ターン以上動いたあとで打ち直す"; exit 3; }
ok "ot-records: 当てたあとに起動した $seen 件の課題の記録に claude_code.llm_request と andon.issue・andon.role があり、user.email が無い"
echo "note ot-restart（受け口を止めてから担当を起動する）はこの道では動かさない。features/otel.md の手順で、Chief of Staff の試しの課題で確かめる"
