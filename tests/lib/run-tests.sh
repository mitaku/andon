#!/usr/bin/env bash
# tests/lib/run-tests.sh [<tests/x.sh>...]
#
# tests/*.sh を並べて走らせる。出力は1本ごとに一時ファイルへ受け、終わった順に1本ずつまとめて出す（混ざらない）。
# 最後に1本ごとの所要時間の表を出す（GITHUB_STEP_SUMMARY があればそこにも）。1本でも落ちれば 1 で終わる。
#   引数が無ければ tests/*.sh 全部。ANDON_TEST_SKIP（空白区切りのパス）は走らせず、表に skip と出す。
#   並べる数は ANDON_TEST_JOBS（既定は nproc）。長いもの（image.sh・andon-dispatcher.sh）を先に起動する。
#   command の出力を読む検査（ANDON_TEST_EMIT_READER。既定は tests/schema.sh）は、`<読み手> --emitters` が出す検査を
#   回し直さない。同じ回に渡された emitters には ANDON_EMIT_DIR=<置き場>/out を渡し、終わったら <置き場>/<名前>.status に
#   終了コード、<名前>.log に出力を置く。読み手は最後に、その emitters が全部終わってから ANDON_EMITTED=<置き場> で起動する。
#   ANDON_TEST_STREAM（空白区切りのパス。既定は tests/image.sh）は溜めずに、1行ごとに「[パス] 」を付けて流す。
#   長いものが job の上限で打ち切られても、どこまで進んだかがログに残る。終わったときは見出しだけ出す（本文は流した分）。
# tests/run が渡す鍵（fd 9。同時に走る tests/run を1本にする）はこの runner が持ち、検査には渡さない（9>&-）。
#   検査が裏に残したプロセスが鍵を持ち続けて、次の tests/run が止まらないように。
# tests/*.sh は互いに独立（一時ディレクトリだけに書く。docker を使うのは image.sh だけ）であることを前提にする。
#   ただし mise の道具の置き場は共有なので、go を mise で使うときは並べる前に mise install を一度打つ。
set -uo pipefail
cd "$(dirname "$0")/../.."

if [ "$#" -gt 0 ]; then all=("$@"); else all=(tests/*.sh); fi
skip=" ${ANDON_TEST_SKIP:-} "
jobs_max=${ANDON_TEST_JOBS:-$(nproc 2>/dev/null || echo 2)}
gha=${GITHUB_ACTIONS:-false}
stream=" ${ANDON_TEST_STREAM-tests/image.sh} "

reader=${ANDON_TEST_EMIT_READER:-tests/schema.sh}
declare -A emitter=()   # 読み手と同じ回に走る emitters（skip は除く）
has_reader=false
for f in "${all[@]}"; do [ "$f" = "$reader" ] && [[ "$skip" != *" $f "* ]] && has_reader=true; done
if [ "$has_reader" = true ]; then
  while IFS= read -r e; do
    for f in "${all[@]}"; do [ "$f" = "$e" ] && [[ "$skip" != *" $f "* ]] && emitter[$f]=1; done
  done < <(bash "$reader" --emitters)
fi

# 長いものを先に、読み手は emitters を待つので最後に
queue=()
for slow in tests/image.sh tests/andon-dispatcher.sh; do
  for f in "${all[@]}"; do [ "$f" = "$slow" ] && [ "$f" != "$reader" ] && queue+=("$f"); done
done
for f in "${all[@]}"; do
  case " tests/image.sh tests/andon-dispatcher.sh $reader " in *" $f "*) ;; *) queue+=("$f") ;; esac
done
for f in "${all[@]}"; do [ "$f" = "$reader" ] && queue+=("$f"); done

# go が PATH に無く mise で焼くとき（tests/lib/andon.sh・kanban.sh）は、並べる前に一度だけ mise install する。
# 入っていない道具を並べた mise exec が同時に入れると、入れ終わる前の go を exec して「go couldn't exec」で落ちる。
if [ -z "${ANDON_BIN:-}${KANBAN_BIN:-}" ] && ! command -v go >/dev/null && command -v mise >/dev/null; then
  mise install >&2 || echo "mise install に失敗（このまま回す）" >&2
fi

logs=$(mktemp -d)
trap 'rm -rf "$logs"' EXIT
emitted="$logs/emitted"
[ "${#emitter[@]}" -eq 0 ] || mkdir -p "$emitted/out"
declare -A name_of=() start_of=() result=() secs=()
running=0 failed=0 emitting=0
log_of() { echo "$logs/${1//\//_}.log"; }

reap() {
  local pid st f s
  wait -n -p pid
  st=$?
  f=${name_of[$pid]}
  s=$(awk -v a="${start_of[$pid]}" -v b="$EPOCHREALTIME" 'BEGIN { printf "%.0f", b - a }')
  secs[$f]=$s
  running=$((running - 1))
  if [ -n "${emitter[$f]:-}" ]; then
    local b; b=$(basename "$f" .sh)
    cp "$(log_of "$f")" "$emitted/$b.log"
    echo "$st" >"$emitted/$b.status"
    emitting=$((emitting - 1))
  fi
  if [ "$st" -eq 0 ]; then
    result[$f]=ok
    if [[ "$stream" == *" $f "* ]]; then
      echo "== $f（ok ${s}s。出力は上に流した）"
    else
      if [ "$gha" = true ]; then echo "::group::== $f（ok ${s}s）"; else echo "== $f（ok ${s}s）"; fi
      cat "$(log_of "$f")"
      [ "$gha" = true ] && echo "::endgroup::"
    fi
  else
    result[$f]="FAIL($st)"
    failed=1
    echo "== $f（FAIL exit $st, ${s}s）"
    [[ "$stream" == *" $f "* ]] || cat "$(log_of "$f")"
    [ "$gha" = true ] && echo "::error title=$f::$f が exit $st で落ちた"
  fi
  return 0
}

for f in "${queue[@]}"; do
  case "$skip" in *" $f "*) result[$f]=skip; secs[$f]=-; continue ;; esac
  # 読み手は、同じ回の emitters が全部終わってから
  if [ "$f" = "$reader" ]; then while [ "$emitting" -gt 0 ]; do reap; done; fi
  while [ "$running" -ge "$jobs_max" ]; do reap; done
  env=()
  if [ -n "${emitter[$f]:-}" ]; then env=(ANDON_EMIT_DIR="$emitted/out"); emitting=$((emitting + 1)); fi
  if [ "$f" = "$reader" ] && [ "${#emitter[@]}" -gt 0 ]; then env=(ANDON_EMITTED="$emitted"); fi
  start=$EPOCHREALTIME
  if [[ "$stream" == *" $f "* ]]; then
    # 流すものも出力を控える（emitters の結果として読み手が skip の行を読む）
    { env "${env[@]}" bash "$f" </dev/null 2>&1 | tee "$(log_of "$f")" | sed -u "s|^|[$f] |"; exit "${PIPESTATUS[0]}"; } 9>&- &
  else
    env "${env[@]}" bash "$f" </dev/null >"$(log_of "$f")" 2>&1 9>&- &
  fi
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
