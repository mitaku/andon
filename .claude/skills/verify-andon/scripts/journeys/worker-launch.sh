#!/usr/bin/env bash
# 担当の起動（features/worker-launch.md）。当てたあとにセルで起動した担当・レビュアーの起動の記録（library/launches.jsonl）を読み、
# 当てたパッケージで起動したかを見る。読むだけ。当てたあとの起動がまだ無ければ 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"

lines=$(since "$ws"/projects/*/kanban/issues/*/library/launches.jsonl)
n=$(grep -c . <<<"$lines" || true)
echo "-- wl-record: 当てたあとの起動の記録 $n 件"
[ "$n" -gt 0 ] || { echo "note 当てたあとに担当もレビュアーも起動していない。Chief of Staff が次に課題を起動したあとで打ち直す"; exit 3; }
jq -c '{at, issue, role, launch, profile, terminal, package: .package | {commit, source, drift}}' <<<"$lines" | sed 's/^/  | /'
bad=$(jq -c --arg c "$a_commit" --arg p "$pkg" 'select(.package.commit != $c or .package.drift != false or .package.root != $p)' <<<"$lines")
[ -z "$bad" ] && ok "wl-record: 当てたあとの起動 $n 件は、全部が当てた commit のパッケージ（$pkg・食い違い無し）で起動した" \
  || { fail "wl-record: 当てた commit と違うパッケージか、食い違いのあるパッケージで起動した"; sed 's/^/  | /' <<<"$bad"; }

# wl-terminal: 起動した端末のうち今も動いているものが、記録の template と課題のラベルで立っている（読むだけ）
while IFS=$'\t' read -r addr issue; do
  [ -n "$addr" ] || continue
  t=$(hc get "$addr" --json 2>/dev/null) || { echo "  | $addr（$issue）は片付いている"; continue; }
  echo "  | $addr（$issue）: $(jq -c '{state, labels: (.labels | {issue, role})}' <<<"$t")"
  [ "$(jq -r '.labels.issue // empty' <<<"$t")" = "$issue" ] || fail "wl-terminal: $addr のラベル issue= が $issue でない"
done < <(jq -r '[.terminal, .issue] | @tsv' <<<"$lines" | sort -u)
[ "$fails" = 0 ] && ok "wl-terminal: 動いている端末は、起動した課題のラベルを持つ"
[ "$fails" = 0 ]
