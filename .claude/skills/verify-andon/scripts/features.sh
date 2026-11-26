#!/usr/bin/env bash
# features.sh <commit の範囲>
#
# 範囲（git diff に渡す形。例 8c32aac..origin/main）の差分が当たる機能の名前を、1行1つで出す。当たらなければ何も出さず 0。
# 機能と「変えたら動かすパス」は features/<機能>.md の「## 変えたら動かすパス」の節（1行1つ、`` で囲んだパス）が正。
# パスの書き方は tests/lib/scope.sh と同じ: 「<dir>/」はその下の全部、「*」は同じ段の中だけ、それ以外はそのパス。
# 目次（features/README.md）の「地図が覆わないもの」の行の先頭に名指したパスは、機能のパスの下にあっても数えない（どの機能の道も通らないもの）。
# テスト（*_test.go・testdata/・internal/ の下で親の package の名前に test を付けたテスト用の package。internal/ledger/ledgertest/ など）は動きを変えないので数えない。
# Go のファイルでコメントだけを変えたもの（gocode.go で字句の並びが前後で同じ。//go: の指令と cgo の前置きは字句に数える）も数えない。git の repo の中（Andon の checkout）で打つ。
# Go のファイルは package の単位でも読む（gopkgs.go）: パスが名指す Go のファイルと同じ package か、パスの package が repo の中で推移的に import する package のファイルなら、その機能を出す。
set -euo pipefail
[ $# = 1 ] && [ -n "$1" ] || { sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
map=$(cd "$(dirname "$0")/../features" && pwd)
changed=$(git -c core.quotepath=off diff --name-only --no-renames "$1") # ASCII でないパスを引用符で包ませない
# Go のファイルでコメントだけを変えたもの（gocode.go で字句の並びが前後で同じ）は数えない。範囲が <前>..<後> か <前>...<後> のときだけ見る
here=$(cd "$(dirname "$0")" && pwd)
revs=("$1" "") # import を読む木。範囲でなければ、渡した commit と作業ツリー（git diff <commit> と同じ）
if [[ "$1" == *..* ]]; then
  before=${1%%..*} after=${1#*..}
  if [[ "$after" == .* ]]; then after=${after#.}; before=$(git merge-base "${before:-HEAD}" "${after:-HEAD}"); fi
  revs=("${before:-HEAD}" "${after:-HEAD}")
  gofiles=$(grep -E '\.go$' <<<"$changed" | grep -vE '_test\.go$' || true)
  if [ -n "$gofiles" ]; then
    # go は打った所（Andon の checkout。mise.toml が go を決める）で打つ。skill は repo の外へ展開して打たれることがある（verify.md）
    code=$(go run "$here/gocode.go" "$PWD" "${before:-HEAD}" "${after:-HEAD}" <<<"$gofiles")
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

# istest <パス>: テスト（*_test.go・testdata/・テスト用の package）か
istest() {
  case "$1" in *_test.go) return 0 ;; esac
  [[ "$1" =~ (^|/)testdata/ ]] || grep -qE '^internal/([^/]+/)*([a-z]+)/\2test/' <<<"$1"
}
# outsider <パス>: 「地図が覆わないもの」に入るか
outsider() {
  local o
  for o in "${outside[@]}"; do
    if hits "$o" "$1"; then return 0; fi
  done
  return 1
}

names=() partlist="" declare -A partsof=()
for f in "$map"/*.md; do
  [ "$(basename "$f")" = README.md ] && continue
  name=$(basename "$f" .md)
  mapfile -t parts < <(sed -n '/^## 変えたら動かすパス$/,/^## /p' "$f" | sed -n 's/^- `\([^`]*\)`.*/\1/p')
  [ "${#parts[@]}" -gt 0 ] || { echo "features.sh: $f に「## 変えたら動かすパス」の行が無い" >&2; exit 1; }
  names+=("$name")
  partsof[$name]=$(printf '%s\n' "${parts[@]}")
  for part in "${parts[@]}"; do partlist+="$name"$'\t'"$part"$'\n'; done
done

# 数える Go のファイル（テストでも地図が覆わないものでもない）があるときだけ、機能ごとの package の置き場を引く（gopkgs.go）
declare -A reach=()
gopaths=0
while IFS= read -r p; do
  if [[ "$p" == *.go ]] && ! istest "$p" && ! outsider "$p"; then gopaths=1; fi
done <<<"$changed"
if [ "$gopaths" = 1 ]; then
  dirs=$(go run "$here/gopkgs.go" "$PWD" "${revs[@]}" <<<"$partlist") # 落ちたら set -e で止まる（黙って package を読まずに抜けない）
  while IFS= read -r l; do if [ -n "$l" ]; then reach["$l"]=1; fi; done <<<"$dirs"
fi

for name in "${names[@]}"; do
  mapfile -t parts <<<"${partsof[$name]}"
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    if istest "$p" || outsider "$p"; then continue; fi
    for part in "${parts[@]}"; do
      if hits "$part" "$p"; then echo "$name"; continue 3; fi
    done
    # 同じ package か、パスの package が推移的に import する package の Go のファイル
    if [[ "$p" == *.go ]] && [ -n "${reach["$name"$'\t'"${p%/*}"]:-}" ]; then echo "$name"; continue 2; fi
  done <<<"$changed"
done
exit 0
