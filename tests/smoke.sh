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
  [[ $base =~ ^([0-9]{4}-[0-9]{2}-[0-9]{2})-[a-z0-9]+(-[a-z0-9]+)*\.md$ ]] || { echo "  名前が YYYY-MM-DD-<英字の短いタイトル>.md でない: $f" >&2; return 1; }
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

# ADR-0041 受け入れ条件 2: command はプロジェクトの slug を andon と比べて委譲の上限を決めない。型 andon の名前と値は残す
cap_re='AndonProject\|andonCap'
check "上限の名前の検査が古い定義に当たる" grep -q "$cap_re" <<<'const AndonProject = "andon"'
no_cap() { ! grep -rn "$cap_re" --include='*.go' . >&2; }
check "Go のコードに AndonProject・andonCap が無い" no_cap
check "委譲の型（Presets）に andon が在る" grep -q '^	"andon": {"merge": "B"' internal/delegate/delegate.go
# 枠の空きを見る dispatcher はオーナー待ちの数では止めない（ADR-0065 受け入れ条件 6）
waits_idle='オーナー待ちが上限に達している間は'
check "オーナー待ちで空きを見ない文の検査が直す前の文に当たる" grep -q "$waits_idle" <<<'前の回の結果を消す。オーナー待ちが上限に達している間は `slots.free` が 0 なので空きにならない。'
no_waits_idle() { ! grep -n "$waits_idle" docs/dispatcher.md >&2; }
check "dispatcher.md にオーナー待ちが上限の間は空きを見ない文が無い" no_waits_idle

exit "$fail"
