#!/usr/bin/env bash
# features.sh <commit の範囲>
#
# 範囲（git diff に渡す形。例 8c32aac..origin/main）の差分が当たる機能の名前を、1行1つで出す。当たらなければ何も出さず 0。
# 機能と「変えたら動かすパス」は features/<機能>.md の「## 変えたら動かすパス」の節（1行1つ、`` で囲んだパス）が正。
# パスの書き方は tests/lib/scope.sh と同じ: 「<dir>/」はその下の全部、「*」は同じ段の中だけ、それ以外はそのパス。
# 目次（features/README.md）の「地図が覆わないもの」の行の先頭に名指したパスは、機能のパスの下にあっても数えない（どの機能の道も通らないもの）。
# テスト（*_test.go・testdata/・internal/ の下で親の package の名前に test を付けたテスト用の package。internal/ledger/ledgertest/ など）は動きを変えないので数えない。
# Go のファイルでコメントだけを変えたもの（gocode.go で字句の並びが前後で同じ。//go: の指令と cgo の前置きは字句に数える）も数えない。git の repo の中（Andon の checkout）で打つ。
set -euo pipefail
[ $# = 1 ] && [ -n "$1" ] || { sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
map=$(cd "$(dirname "$0")/../features" && pwd)
changed=$(git -c core.quotepath=off diff --name-only --no-renames "$1") # ASCII でないパスを引用符で包ませない
# Go のファイルでコメントだけを変えたもの（gocode.go で字句の並びが前後で同じ）は数えない。範囲が <前>..<後> か <前>...<後> のときだけ見る
here=$(cd "$(dirname "$0")" && pwd)
if [[ "$1" == *..* ]]; then
  before=${1%%..*} after=${1#*..}
  if [[ "$after" == .* ]]; then after=${after#.}; before=$(git merge-base "${before:-HEAD}" "${after:-HEAD}"); fi
  gofiles=$(grep -E '\.go$' <<<"$changed" | grep -vE '_test\.go$' || true)
  if [ -n "$gofiles" ]; then
    repo=$PWD
    code=$(cd "$here" && go run gocode.go "$repo" "${before:-HEAD}" "${after:-HEAD}" <<<"$gofiles")
    changed=$( { grep -vE '\.go$' <<<"$changed" || true; grep -E '_test\.go$' <<<"$changed" || true; echo "$code"; } | grep . || true)
  fi
fi

# hits <部分> <パス>: パスが部分に入るか
hits() {
  case "$1" in
    */) [[ "$2" == "$1"* ]] ;;
    *'*'*) [[ "$2" == $1 && "${2%/*}" == "${1%/*}" ]] ;;
    *) [ "$2" = "$1" ] ;;
  esac
}
mapfile -t outside < <(sed -n '/^## 地図が覆わないもの$/,/^## /p' "$map/README.md" | grep -oE '^- (`[^`]+`・)*`[^`]+`' | grep -oE '`[^`]+`' | tr -d '`')
[ "${#outside[@]}" -gt 0 ] || { echo "features.sh: $map/README.md の「## 地図が覆わないもの」に置き場の行が無い" >&2; exit 1; }

for f in "$map"/*.md; do
  [ "$(basename "$f")" = README.md ] && continue
  name=$(basename "$f" .md)
  mapfile -t parts < <(sed -n '/^## 変えたら動かすパス$/,/^## /p' "$f" | sed -n 's/^- `\([^`]*\)`.*/\1/p')
  [ "${#parts[@]}" -gt 0 ] || { echo "features.sh: $f に「## 変えたら動かすパス」の行が無い" >&2; exit 1; }
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    case "$p" in *_test.go) continue ;; esac
    if [[ "$p" =~ (^|/)testdata/ ]] || grep -qE '^internal/([^/]+/)*([a-z]+)/\2test/' <<<"$p"; then continue; fi
    for o in "${outside[@]}"; do
      if hits "$o" "$p"; then continue 2; fi
    done
    for part in "${parts[@]}"; do
      if hits "$part" "$p"; then echo "$name"; continue 3; fi
    done
  done <<<"$changed"
done
exit 0
