#!/usr/bin/env bash
# tests/lib/run-tests.sh [<tests/x.sh>...]
#
# tests/*.sh を並べて走らせる。出力は1本ごとに一時ファイルへ受け、終わった順に1本ずつまとめて出す（混ざらない）。
# 最後に1本ごとの所要時間の表を出す（GITHUB_STEP_SUMMARY があればそこにも）。1本でも落ちれば 1 で終わる。
#   引数が無ければ tests/*.sh 全部。ANDON_TEST_SKIP（空白区切りのパス）は走らせず、表に skip と出す。
#   並べる数は ANDON_TEST_JOBS（既定は nproc）。長いもの（image.sh・schema.sh）を先に起動する。
# tests/*.sh は互いに独立（一時ディレクトリだけに書く。docker を使うのは image.sh だけ）であることを前提にする。
#   ただし mise の道具の置き場は共有なので、go を mise で使うときは並べる前に mise install を一度打つ。
set -uo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -gt 0 ]; then all=("$@"); else all=(tests/*.sh); fi
skip=" ${ANDON_TEST_SKIP:-} "
jobs_max=${ANDON_TEST_JOBS:-$(nproc 2>/dev/null || echo 2)}
gha=${GITHUB_ACTIONS:-false}

queue=()
for slow in tests/image.sh tests/schema.sh; do
  for f in "${all[@]}"; do [ "$f" = "$slow" ] && queue+=("$f"); done
done
for f in "${all[@]}"; do
  case " tests/image.sh tests/schema.sh " in *" $f "*) ;; *) queue+=("$f") ;; esac
done

# go が PATH に無く mise で焼くとき（tests/lib/andon.sh・kanban.sh）は、並べる前に一度だけ mise install する。
# 入っていない道具を並べた mise exec が同時に入れると、入れ終わる前の go を exec して「go couldn't exec」で落ちる。
if [ -z "${ANDON_BIN:-}${KANBAN_BIN:-}" ] && ! command -v go >/dev/null && command -v mise >/dev/null; then
  mise install >&2 || echo "mise install に失敗（このまま回す）" >&2
fi

logs=$(mktemp -d)
trap 'rm -rf "$logs"' EXIT
declare -A name_of=() start_of=() result=() secs=()
running=0 failed=0

reap() {
  local pid st f s
  wait -n -p pid
  st=$?
  f=${name_of[$pid]}
  s=$(awk -v a="${start_of[$pid]}" -v b="$EPOCHREALTIME" 'BEGIN { printf "%.0f", b - a }')
  secs[$f]=$s
  running=$((running - 1))
  if [ "$st" -eq 0 ]; then
    result[$f]=ok
    if [ "$gha" = true ]; then echo "::group::== $f（ok ${s}s）"; else echo "== $f（ok ${s}s）"; fi
    cat "$logs/${f//\//_}.log"
    [ "$gha" = true ] && echo "::endgroup::"
  else
    result[$f]="FAIL($st)"
    failed=1
    echo "== $f（FAIL exit $st, ${s}s）"
    cat "$logs/${f//\//_}.log"
    [ "$gha" = true ] && echo "::error title=$f::$f が exit $st で落ちた"
  fi
  return 0
}

for f in "${queue[@]}"; do
  case "$skip" in *" $f "*) result[$f]=skip; secs[$f]=-; continue ;; esac
  while [ "$running" -ge "$jobs_max" ]; do reap; done
  start=$EPOCHREALTIME
  bash "$f" </dev/null >"$logs/${f//\//_}.log" 2>&1 &
  name_of[$!]=$f
  start_of[$!]=$start
  running=$((running + 1))
done
while [ "$running" -gt 0 ]; do reap; done

table=$(
  echo "| test | 結果 | 秒 |"
  echo "|---|---|---:|"
  for f in "${all[@]}"; do echo "| $f | ${result[$f]} | ${secs[$f]} |"; done
)
echo "$table"
[ -n "${GITHUB_STEP_SUMMARY:-}" ] && printf '### tests/*.sh（並べる数 %s）\n\n%s\n' "$jobs_max" "$table" >>"$GITHUB_STEP_SUMMARY"
exit "$failed"
