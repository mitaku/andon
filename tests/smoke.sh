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

exit "$fail"
