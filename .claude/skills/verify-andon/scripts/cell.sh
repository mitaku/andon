# cell.sh: セルの本物の記録を読むだけの journeys/*.sh が source する。何も書かない・何も起動しない。
#   当てた記録（<パッケージ>/applied.json）から、検証する commit を当てた時刻と、次に当てた時刻を読む。この間だけを読む。
#   検証する commit は VERIFY_COMMIT（drive.sh の2つ目の引数。先頭の数文字でよい）。無いか今当たっている commit なら、当てた時刻から今まで。
#   前に当てた commit は記録の previous（新しい順）から探す。記録が無い・previous に無い（前のバージョンの記録）なら、間が決まらないので 3（未確認）で終わる。
# パッケージの置き場は /opt/andon（VERIFY_PACKAGE は tests/verify-andon.sh が偽の置き場を指すときだけ使う）。
# 読んだあとに使えるもの: common.sh の分と $pkg（パッケージの置き場）・$ws（セルの repo）・$a_commit（検証する commit）・$a_at（それを当てた時刻。UTC の ISO 8601）・
#   $a_ms（同じ時刻のミリ秒）・$a_until（次に当てた時刻。今当たっていれば空）・since <jsonl…>（.at が当てた時刻から次に当てた時刻までの行だけを出す。引数が無ければ標準入力）
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ws=${ANDON_WORKSPACE:-/workspace}
pkg=${VERIFY_PACKAGE:-/opt/andon}
applied=$pkg/applied.json
[ -f "$applied" ] || { echo "note 当てた記録 $applied が無い（イメージのバージョンのまま）。当てたあとの出来事が決まらないので確かめない"; exit 3; }
a_commit=$(jq -r .commit "$applied") a_at=$(jq -r .applied_at "$applied") a_until=
want=${VERIFY_COMMIT:-}
if [ -n "$want" ] && [[ "$a_commit" != "$want"* ]]; then
  # 今と前に当てたもの（新しい順）から、want で始まる一番新しいものと、その1つ新しいもの（次に当てたもの）の時刻
  hit=$(jq -r --arg w "$want" '([{commit, applied_at}] + (.previous // [])) as $all
    | first(range(1; $all | length) | select($all[.].commit | startswith($w))) // empty
    | "\($all[.].commit) \($all[.].applied_at) \($all[. - 1].applied_at)"' "$applied" 2>/dev/null)
  [ -n "$hit" ] || { echo "note 当てた記録 $applied に $want を前に当てた記録が無い（前のバージョンの記録か、古くて残っていない）。当たっていた間が決まらないので確かめない"; exit 3; }
  read -r a_commit a_at a_until <<<"$hit"
fi
a_ms=$(($(date -u -d "$a_at" +%s) * 1000))
echo "検証する commit $a_commit・当てた時刻 $a_at・次に当てた時刻 ${a_until:-無し（今も当たっている）}"
since() { cat "$@" 2>/dev/null | jq -c --arg t "$a_at" --arg u "$a_until" 'select(.at >= $t and ($u == "" or .at < $u))'; }
