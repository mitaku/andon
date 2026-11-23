#!/usr/bin/env bash
# gate（features/gate.md）。当てたあとに dispatcher が回した gate の記録（library/gate/<head>.json）を読む。読むだけ。
# 当てたあと（検証する commit が当たっていた間）に gate が1回も回っていなければ 3（未確認）。
set -uo pipefail
. "$(dirname "$0")/../cell.sh"

recs=$(for f in "$ws"/projects/*/kanban/issues/*/library/gate/*.json; do
  [ -f "$f" ] && jq -c --arg f "$f" '. + {file: $f}' "$f" 2>/dev/null
done | dsince)
n=$(grep -c . <<<"$recs" || true)
echo "-- gt-record: 当てたあとの gate の記録 $n 件"
[ "$n" -gt 0 ] || { echo "note 当てたあとに gate が回っていない。次の課題が review に入ったあとで打ち直す"; exit 3; }
jq -c '{at, id, pr, head: .head[0:12], tier, decision, merged, reasons: (.reasons | length), checks: .github_checks.state, review: .review.state}' <<<"$recs" | sed 's/^/  | /'
bad=$(jq -c 'select((.decision | IN("approve", "fix", "needs_human") | not) or (.tier | IN("A", "B", "C") | not) or (.head | test("^[0-9a-f]{40}$") | not) or (.reasons | type) != "array")' <<<"$recs")
[ -z "$bad" ] && ok "gt-record: 当てたあとの gate $n 回は、全部が判定・レベル・head・理由の形を持つ" \
  || { fail "gt-record: 形の違う gate の記録がある"; sed 's/^/  | /' <<<"$bad"; }

# gt-route: 判定が kanban に届いた（fix は担当へ、needs_human は問い、approve はマージかオーナーへ）。課題の今の status を並べる。
# $ws を cwd にしても kanban は見つからない（projects/*/kanban が複数あり得る）ので、記録のパスから kanban の親を KANBAN_ROOT に渡す
unread=0
while IFS=$'\t' read -r id decision merged file; do
  st=$(KANBAN_ROOT="${file%%/kanban/issues/*}" kanban show "$id" --json 2>/dev/null | jq -r '.issue.status // empty')
  [ -n "$st" ] || { st='?'; unread=$((unread + 1)); }
  echo "  | $id: 判定 $decision・merged $merged・今の status $st"
done < <(jq -r '[.id, .decision, (.merged | tostring), .file] | @tsv' <<<"$recs")
[ "$unread" = 0 ] && ok "gt-route: 判定ごとの課題の今の status を並べた（読み方は features/gate.md）" \
  || fail "gt-route: 課題 $unread 件の今の status を kanban show で読めない"
[ "$fails" = 0 ]
