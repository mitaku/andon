#!/usr/bin/env bash
# drive.sh <機能> <検証する commit> [<名指した細かい機能>]
#
# 機能の地図（features/<機能>.md）の1機能を、scripts/journeys/<機能>.sh で動かす。証拠は標準出力に出る（andon issue record で撮る）。
# 1回の呼び出しの中で、一時の置き場を作り・動かし・片付け・跡が残っていないことを確かめるまでを済ませる
# （エージェントの Bash は呼び出しごとに環境が戻るので、手順を呼び出しに分けない）。
# 検証する commit（依頼書の「当てた commit:」の行。先頭の 7 文字以上の16進数）は省けない。本物の記録を読む道は、その commit が当たっていた間
# （次に当てた時刻まで）だけを読む。今当たっているものを読むときも andon version の commit を渡す。切り離して動かす道は今当たっている本物を動かす。
# 名指した細かい機能（commit の本文の Verify-features: の名前。「,」か空白で区切って1つの引数で渡す）は、地図のどれかの機能に在る名前。
# 別の機能の名前は数えずに捨てる。最後に細かい機能ごとの結果（ok・未確認・FAIL）を1行ずつ出し、名指しがあれば終了コードは名指した分だけで決める。
# 終了コード: 0 は動いた証拠が取れた（名指しがあれば名指した細かい機能が全部 ok）、1 は FAIL の行がある（壊れ。名指さない細かい機能の FAIL も、道が 3 で終わったときも）か跡が残った、
# 2 は使い方の誤り（検証する commit が無い・形が違う・知らない機能・地図に無い細かい機能）、3 は前提が無く確かめられなかった（未確認。動いたと書かない）。
set -uo pipefail
usage() { sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
# 検証する commit を省くと、検証の途中で次の commit が当たったとき、新しい commit の間を読んで未確認や誤りになる。既定は持たない
[ $# = 2 ] || [ $# = 3 ] || { echo "drive.sh: 機能と検証する commit の2つ（と名指した細かい機能）を渡す（今は $# 個）" >&2; usage; }
[[ "$2" =~ ^[0-9a-f]{7,40}$ ]] || { echo "drive.sh: 検証する commit「$2」が先頭の 7 文字以上の16進数（小文字）でない" >&2; usage; }
scripts=$(cd "$(dirname "$0")" && pwd)
journey=$scripts/journeys/$1.sh
[ -f "$journey" ] || { echo "drive.sh: 知らない機能 $1（features/README.md の一覧から選ぶ）" >&2; exit 2; }
# subs <機能>: 地図の「## 細かい機能」の節の名前を1行1つで出す
subs() { sed -n '/^## 細かい機能$/,/^## /p' "$scripts/../features/$1.md" 2>/dev/null | sed -n 's/^- `\([^`]*\)`.*/\1/p'; }
mine=$(subs "$1")
[ -n "$mine" ] || { echo "drive.sh: features/$1.md の細かい機能が読めない" >&2; exit 2; }
# 見る道が「検証の担当だけ」でない細かい機能（Chief of Staff の試し・オーナーが見る）は drive.sh では証拠が取れない。名指しても終了コードに数えない
inside=$(sed -n '/^## 細かい機能$/,/^## /p' "$scripts/../features/$1.md" | grep '見る道: 検証の担当だけ。$' | sed -n 's/^- `\([^`]*\)`.*/\1/p')
named=() outside=() other=()
if [ $# = 3 ]; then
  all=$(for f in "$scripts"/../features/*.md; do n=${f##*/}; [ "$n" = README.md ] || subs "${n%.md}"; done)
  for s in ${3//,/ }; do
    grep -qxF -- "$s" <<<"$all" || { echo "drive.sh: 名指した細かい機能「$s」が地図に無い（features/*.md の「## 細かい機能」の名前を渡す）" >&2; usage; }
    if grep -qxF -- "$s" <<<"$inside"; then named+=("$s")
    elif grep -qxF -- "$s" <<<"$mine"; then outside+=("$s")
    else other+=("$s"); fi
  done
  [ $((${#named[@]} + ${#outside[@]} + ${#other[@]})) -gt 0 ] || { echo "drive.sh: 名指した細かい機能が空" >&2; usage; }
fi

# 跡の印。切り離した場所で作る課題のタイトル・一時の置き場の名前に入れ、片付けたあとセルの本物に残っていないかを探す
run=verify-$1-$(date -u +%Y%m%dT%H%M%SZ)-$$
ws=${ANDON_WORKSPACE:-/workspace}
tmp=$(mktemp -d "${TMPDIR:-/tmp}/$run.XXXXXX")
out=$(mktemp "${TMPDIR:-/tmp}/$run-out.XXXXXX") # 道の出力の写し。下の集計が読む。一時の置き場（$tmp）は道が使うので分ける
cleanup() { chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp" "$out"; }
trap cleanup EXIT

echo "== $run"
echo "機能: $1（$journey）"
echo "andon: $(command -v andon) $(andon version 2>/dev/null | jq -c '{commit, source, drift}')"
echo "当てた記録: $(cat "${VERIFY_PACKAGE:-/opt/andon}/applied.json" 2>/dev/null || echo 無し)"
echo "検証する commit: $2（本物の記録を読む行が読む間を決める。切り離して動かす道と今の状態を読む行は、今当たっているパッケージを見る）"
echo "名指した細かい機能: ${named[*]:-無し}$([ ${#outside[@]} = 0 ] || echo "（drive.sh の外で見る ${outside[*]} は数えない）")$([ ${#other[@]} = 0 ] || echo "（別の機能の ${other[*]} は数えない）")"
echo "始めた: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
VERIFY_COMMIT=$2 VERIFY_RUN=$run VERIFY_TMP=$tmp VERIFY_REPO=$(cd "$scripts/../../../.." && pwd) bash "$journey" | tee "$out"
rc=${PIPESTATUS[0]}
echo "== 動かした結果: 終了コード $rc"

# 細かい機能ごとの結果。道の行の「ok   <名前>」「FAIL <名前>」を読む。どちらも無ければ未確認で、理由は名前を含む note の行。
# 名指した細かい機能は「名指し」、ほかは「補い」と添える（補いの未確認は終了コードを変えない。補いの FAIL は道が 1 で終わるので 1 のまま）
echo "== 細かい機能ごとの結果"
word() { grep -E -- "(^|[^a-z0-9-])$1([^a-z0-9-]|\$)"; }
unconfirmed=() failed=()
while read -r s; do
  tag=補い
  for n in "${named[@]}" "${outside[@]}"; do [ "$n" = "$s" ] && tag=名指し; done
  if ! grep -qxF -- "$s" <<<"$inside"; then
    echo "結果 $s 外で見る（$tag）: $(sed -n '/^## 細かい機能$/,/^## /p' "$scripts/../features/$1.md" | grep "^- \`$s\` " | grep -o '見る道: .*')"
    continue
  fi
  # ok の行には、その名前で始まる note（ok の但し書き）だけを添える。ほかの細かい機能の note が「<名前> は確かめた」と名前を含むため
  notes=$(grep -F "note $s:" "$out" | grep -E "^note $s:" || true)
  if grep -E '^FAIL ' "$out" | word "$s" >/dev/null; then
    st=FAIL; failed+=("$s")
  elif grep -E '^ok +' "$out" | word "$s" >/dev/null; then
    st=ok
  else
    st=未確認; [ "$tag" = 名指し ] && unconfirmed+=("$s")
    notes=$(grep '^note ' "$out" | word "$s" || true)
  fi
  echo "結果 $s $st（$tag）$([ -z "$notes" ] || echo ": $(sed -n 1p <<<"$notes")")"
  [ "$(grep -c . <<<"$notes")" -le 1 ] || sed 1d <<<"$notes" | sed 's/^/  | /'
done <<<"$mine"
loose=$(grep '^note ' "$out" | while IFS= read -r l; do
  hit=false
  while read -r s; do if word "$s" >/dev/null <<<"$l"; then hit=true; break; fi; done <<<"$mine"
  [ "$hit" = true ] || echo "$l"
done)
[ -z "$loose" ] || { echo "細かい機能を名指さない note:"; sed 's/^/  | /' <<<"$loose"; }
anyfail=false; grep -q '^FAIL ' "$out" && anyfail=true # 写しは下の片付けで消えるので、先に読む

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
# FAIL の行が1つでもあれば、名指しによらず 1（道が FAIL を出したあと 3 で終わっても、壊れを未確認にしない）
if [ "$anyfail" = true ]; then
  echo "== 終了コード 1（FAIL の行がある。道の終了コードは $rc）"
  exit 1
fi
# 名指し（3つ目の引数）が無ければ道の終了コードのまま。あれば、道が 0 か 3 のときだけ名指した分で決め直す（2 などはそのまま）。
# 名指しのうち drive.sh で確かめるものがこの機能に無い（外で見るものか別の機能のものだけ）なら、決める細かい機能が無いので 0。補いの未確認は終了コードを変えない
if [ $# = 2 ]; then
  echo "== 終了コード $rc（名指しが無いので、道の終了コードのまま）"
  exit "$rc"
fi
case $rc in
  0 | 3)
    if [ ${#unconfirmed[@]} -gt 0 ]; then final=3; else final=0; fi ;;
  *) final=$rc ;;
esac
if [ ${#named[@]} = 0 ]; then
  echo "== 終了コード $final（名指した細かい機能のうち drive.sh で確かめるものがこの機能に無い。道の終了コードは $rc。結果の行は補いだけ）"
else
  echo "== 終了コード $final（名指した ${named[*]} で決めた。道の終了コードは $rc$([ ${#unconfirmed[@]} = 0 ] || echo "。名指したうち未確認: ${unconfirmed[*]}")）"
fi
exit "$final"
