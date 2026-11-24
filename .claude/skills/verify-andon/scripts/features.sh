#!/usr/bin/env bash
# features.sh <commit の範囲>
#
# 範囲（git diff に渡す形。例 8c32aac..origin/main）の差分が当たる機能の名前を、1行1つで出す。当たらなければ何も出さず 0。
# 機能と「変えたら動かすパス」は features/<機能>.md の「## 変えたら動かすパス」の節（1行1つ、`` で囲んだパス）が正。
# パスの書き方は tests/lib/scope.sh と同じ: 「<dir>/」はその下の全部、「*」は同じ段の中だけ、それ以外はそのパス。
# テスト（*_test.go・testdata/・internal/ の下で親の package の名前に test を付けたテスト用の package。internal/ledger/ledgertest/ など）は動きを変えないので数えない。git の repo の中（Andon の checkout）で打つ。
set -euo pipefail
[ $# = 1 ] && [ -n "$1" ] || { sed -n '2,7p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
map=$(cd "$(dirname "$0")/../features" && pwd)
changed=$(git diff --name-only --no-renames "$1")

# hits <部分> <パス>: パスが部分に入るか
hits() {
  case "$1" in
    */) [[ "$2" == "$1"* ]] ;;
    *'*'*) [[ "$2" == $1 && "${2%/*}" == "${1%/*}" ]] ;;
    *) [ "$2" = "$1" ] ;;
  esac
}

for f in "$map"/*.md; do
  [ "$(basename "$f")" = README.md ] && continue
  name=$(basename "$f" .md)
  mapfile -t parts < <(sed -n '/^## 変えたら動かすパス$/,/^## /p' "$f" | sed -n 's/^- `\([^`]*\)`.*/\1/p')
  [ "${#parts[@]}" -gt 0 ] || { echo "features.sh: $f に「## 変えたら動かすパス」の行が無い" >&2; exit 1; }
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    case "$p" in *_test.go) continue ;; esac
    if [[ "$p" =~ (^|/)testdata/ ]] || grep -qE '^internal/([^/]+/)*([a-z]+)/\2test/' <<<"$p"; then continue; fi
    for part in "${parts[@]}"; do
      if hits "$part" "$p"; then echo "$name"; continue 3; fi
    done
  done <<<"$changed"
done
exit 0
