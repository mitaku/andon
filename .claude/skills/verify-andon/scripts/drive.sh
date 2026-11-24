#!/usr/bin/env bash
# drive.sh <機能> <検証する commit>
#
# 機能の地図（features/<機能>.md）の1機能を、scripts/journeys/<機能>.sh で動かす。証拠は標準出力に出る（andon issue record で撮る）。
# 1回の呼び出しの中で、一時の置き場を作り・動かし・片付け・跡が残っていないことを確かめるまでを済ませる
# （エージェントの Bash は呼び出しごとに環境が戻るので、手順を呼び出しに分けない）。
# 検証する commit（依頼書の「当てた commit:」の行。先頭の 7 文字以上の16進数）は省けない。本物の記録を読む道は、その commit が当たっていた間
# （次に当てた時刻まで）だけを読む。今当たっているものを読むときも andon version の commit を渡す。切り離して動かす道は今当たっている本物を動かす。
# 終了コード: 0 は動いた証拠が取れた、1 は動かなかった（壊れ）か跡が残った、2 は使い方の誤り（検証する commit が無い・形が違う・知らない機能）、
# 3 は前提が無く確かめられなかった（未確認。動いたと書かない）。
set -uo pipefail
usage() { sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
# 検証する commit を省くと、検証の途中で次の commit が当たったとき、新しい commit の間を読んで未確認や誤りになる。既定は持たない
[ $# = 2 ] || { echo "drive.sh: 機能と検証する commit の2つを渡す（今は $# 個）" >&2; usage; }
[[ "$2" =~ ^[0-9a-f]{7,40}$ ]] || { echo "drive.sh: 検証する commit「$2」が先頭の 7 文字以上の16進数（小文字）でない" >&2; usage; }
scripts=$(cd "$(dirname "$0")" && pwd)
journey=$scripts/journeys/$1.sh
[ -f "$journey" ] || { echo "drive.sh: 知らない機能 $1（features/README.md の一覧から選ぶ）" >&2; exit 2; }

# 跡の印。切り離した場所で作る課題のタイトル・一時の置き場の名前に入れ、片付けたあとセルの本物に残っていないかを探す
run=verify-$1-$(date -u +%Y%m%dT%H%M%SZ)-$$
ws=${ANDON_WORKSPACE:-/workspace}
tmp=$(mktemp -d "${TMPDIR:-/tmp}/$run.XXXXXX")
cleanup() { chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"; }
trap cleanup EXIT

echo "== $run"
echo "機能: $1（$journey）"
echo "andon: $(command -v andon) $(andon version 2>/dev/null | jq -c '{commit, source, drift}')"
echo "当てた記録: $(cat "${VERIFY_PACKAGE:-/opt/andon}/applied.json" 2>/dev/null || echo 無し)"
echo "検証する commit: $2（本物の記録を読む行が読む間を決める。切り離して動かす道と今の状態を読む行は、今当たっているパッケージを見る）"
echo "始めた: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
VERIFY_COMMIT=$2 VERIFY_RUN=$run VERIFY_TMP=$tmp VERIFY_REPO=$(cd "$scripts/../../../.." && pwd) bash "$journey"
rc=$?
echo "== 動かした結果: 終了コード $rc"

# 片付けと、跡が残っていないことの確かめ（証拠の記録 library/runs/*.log は除く。この出力そのものが入る）
cleanup
trace=0
[ ! -e "$tmp" ] || { echo "FAIL 一時の置き場が残った: $tmp"; trace=1; }
left=$(grep -rlF --exclude='*.log' -- "$run" "$ws"/projects/*/kanban 2>/dev/null | grep -v '/library/runs/' || true)
[ -z "$left" ] || { echo "FAIL セルの本物の kanban に跡が残った:"; echo "$left"; trace=1; }
if hc ls >/dev/null 2>&1; then
  ! hc ls 2>/dev/null | grep -qF -- "$run" || { echo "FAIL 印 $run の端末が残った"; trace=1; }
fi
[ "$trace" = 0 ] && echo "ok   片付けた: 一時の置き場が無く、セルの本物の kanban と端末に印 $run が無い"
echo "終えた: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
[ "$trace" = 0 ] || exit 1
exit "$rc"
