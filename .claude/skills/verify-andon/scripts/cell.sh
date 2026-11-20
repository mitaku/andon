# cell.sh: セルの本物の記録を読むだけの journeys/*.sh が source する。何も書かない・何も起動しない。
#   当てた記録（<パッケージ>/applied.json）から、当てた commit と時刻を読む。無ければ「当てたあと」が決まらないので 3（未確認）で終わる。
# パッケージの置き場は /opt/andon（VERIFY_PACKAGE は tests/verify-andon.sh が偽の置き場を指すときだけ使う）。
# 読んだあとに使えるもの: common.sh の分と $pkg（パッケージの置き場）・$ws（セルの repo）・$a_commit・$a_at（当てた時刻。UTC の ISO 8601）・
#   $a_ms（同じ時刻のミリ秒）・since <jsonl…>（.at が当てた時刻より後の行だけを出す）
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ws=${ANDON_WORKSPACE:-/workspace}
pkg=${VERIFY_PACKAGE:-/opt/andon}
applied=$pkg/applied.json
[ -f "$applied" ] || { echo "note 当てた記録 $applied が無い（イメージの版のまま）。当てたあとの出来事が決まらないので確かめない"; exit 3; }
a_commit=$(jq -r .commit "$applied") a_at=$(jq -r .applied_at "$applied")
a_ms=$(($(date -u -d "$a_at" +%s) * 1000))
echo "当てた commit $a_commit・当てた時刻 $a_at"
since() { cat "$@" 2>/dev/null | jq -c --arg t "$a_at" 'select(.at >= $t)'; }
