#!/usr/bin/env bash
# 型を示す1本の偽テスト。実テストはここに倣って tests/ に足す。
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
check() { # check <説明> <コマンド...>
  local name=$1; shift
  if "$@"; then echo "ok   $name"; else echo "FAIL $name"; fail=1; fi
}

check "README.md がある" test -f README.md
check "LICENSE がある" test -f LICENSE
check "docs/decisions.md がある" test -f docs/decisions.md

# 決定の記録は docs/decisions/ に1件1ファイル（書き方は docs/decisions.md の冒頭）。名前と、日付・出典の行を見る。
decision_ok() { # decision_ok <file>
  local f=$1 base date
  base=$(basename "$f")
  [[ $base =~ ^([0-9]{4}-[0-9]{2}-[0-9]{2})-[a-z0-9]+(-[a-z0-9]+)*\.md$ ]] || { echo "  名前が YYYY-MM-DD-<英字の短い題>.md でない: $f" >&2; return 1; }
  date=${BASH_REMATCH[1]}
  head -1 "$f" | grep -q '^# ' || { echo "  1行目が見出しでない: $f" >&2; return 1; }
  grep -qx -- "- 日付: $date" "$f" || { echo "  「- 日付: $date」の行が無い（名前の日付と揃える）: $f" >&2; return 1; }
  grep -q -- '^- 出典: .' "$f" || { echo "  「- 出典:」の行が無い: $f" >&2; return 1; }
}
# docs/decisions.md の表は 2026-11-07 で凍結した。それより後の日付の行は docs/decisions/ へ。
frozen_ok() { ! awk -F'|' '/^\| 20[0-9][0-9]-/ { d=$2; gsub(/ /, "", d); if (d > "2026-11-07") { print "  凍結した表に行が足された: " $0; bad=1 } } END { exit !bad }' docs/decisions.md >&2; }
check "docs/decisions.md の表に 2026-11-07 より後の行が無い" frozen_ok
for f in docs/decisions/*; do
  [ -e "$f" ] || continue
  check "決定の記録の形: $f" decision_ok "$f"
done

exit "$fail"
