#!/usr/bin/env bash
# gate（features/gate.md）。当てたあとに dispatcher が回した gate の記録（library/gate/<head>.json）を読む。読むだけ。
# 当てたあとに gate が1回も回っていなければ 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"

recs=$(for f in "$ws"/projects/*/kanban/issues/*/library/gate/*.json; do
  [ -f "$f" ] && jq -c --arg f "$f" '. + {file: $f}' "$f" 2>/dev/null
done | jq -c --arg t "$a_at" 'select(.at >= $t)')
n=$(grep -c . <<<"$recs" || true)
echo "-- gt-record: 当てたあとの gate の記録 $n 件"
[ "$n" -gt 0 ] || { echo "note 当てたあとに gate が回っていない。次の課題が review に入ったあとで打ち直す"; exit 3; }
jq -c '{at, id, pr, head: .head[0:12], tier, decision, merged, reasons: (.reasons | length), checks: .github_checks.state, review: .review.state}' <<<"$recs" | sed 's/^/  | /'
bad=$(jq -c 'select((.decision | IN("approve", "fix", "needs_human") | not) or (.tier | IN("A", "B", "C") | not) or (.head | test("^[0-9a-f]{40}$") | not) or (.reasons | type) != "array")' <<<"$recs")
[ -z "$bad" ] && ok "gt-record: 当てたあとの gate $n 回は、全部が判定・レベル・head・理由の形を持つ" \
  || { fail "gt-record: 形の違う gate の記録がある"; sed 's/^/  | /' <<<"$bad"; }

# gt-route: 判定が kanban に届いた（fix は担当へ、needs_human は問い、approve はマージかオーナーへ）。課題の今の status を並べる
while IFS=$'\t' read -r id decision merged; do
  st=$(cd "$ws" && kanban show "$id" --json 2>/dev/null | jq -r .issue.status) || st='?'
  echo "  | $id: 判定 $decision・merged $merged・今の status $st"
done < <(jq -r '[.id, .decision, (.merged | tostring)] | @tsv' <<<"$recs")
ok "gt-route: 判定ごとの課題の今の status を並べた（読み方は features/gate.md）"
[ "$fails" = 0 ]
