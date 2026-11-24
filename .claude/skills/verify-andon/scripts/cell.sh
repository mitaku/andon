# cell.sh: セルの本物の記録を読むだけの journeys/*.sh が source する。何も書かない・何も起動しない。
#   当てた記録（<パッケージ>/applied.json）から、検証する commit を当てた時刻と、次に当てた時刻を読む。この間だけを読む。
#   検証する commit は VERIFY_COMMIT（drive.sh の2つ目の引数。drive.sh は省けず、先頭の 7 文字以上を必ず渡す）。無い（道を直に呼ぶ検査だけ）なら今当たっている commit。今当たっていれば次に当てた時刻は無く、今まで。
#   前に当てた commit は記録の previous（新しい順）から探す。同じ commit を続けて当てたら、最初に当てた時刻から。
#   記録が無い・previous に無い（前のバージョンの記録）なら、間が決まらないので 3（未確認）で終わる。
# パッケージの置き場は /opt/andon（VERIFY_PACKAGE は tests/verify-andon.sh が偽の置き場を指すときだけ使う）。
# 読んだあとに使えるもの: common.sh の分と $pkg（パッケージの置き場）・$ws（セルの repo）・$a_commit（検証する commit）・$a_at（それを当てた時刻。UTC の ISO 8601）・
#   $now_ms（最後に当てた時刻のミリ秒。今の状態を読む行が比べる）・$a_until（次に当てた時刻。今当たっていれば空）・since <jsonl…>（.at が当てた時刻から次に当てた時刻までの行だけを出す。引数が無ければ標準入力）・
#   $d_at・$d_until・dsince <jsonl…>（同じことを dispatcher が切り替わった時刻 switched_at で。dispatcher が書く記録〈gate・通知・枠の空き〉はこちらで読む。
#   当ててから前の dispatcher が見回りを終えるまでの間、その記録は前のパッケージのコードが書くため）
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"
ws=${ANDON_WORKSPACE:-/workspace}
pkg=${VERIFY_PACKAGE:-/opt/andon}
applied=$pkg/applied.json
[ -f "$applied" ] || { echo "note 当てた記録 $applied が無い（イメージのバージョンのまま）。当てたあとの出来事が決まらないので確かめない"; exit 3; }
# 今と前に当てたもの（新しい順）から、検証する commit（無ければ今のもの）で始まる一番新しい回 i と、同じ commit を続けて当てた一番古い回 j。
# 当たっていた間は j を当てた時刻から、i の1つ新しい回（次に当てたもの）を当てた時刻まで
# dispatcher が書く記録は、当てた時刻でなく dispatcher が切り替わった時刻（switched_at。無ければ当てた時刻）で区切る
hit=$(jq -r --arg w "${VERIFY_COMMIT:-}" '([{commit, applied_at, switched_at}] + (.previous // [])) as $all
  | def sw(k): $all[k].switched_at // $all[k].applied_at;
  first(range(0; $all | length) | select($all[.].commit | startswith($w))) as $i
  | last(range($i; $all | length) | select(all(range($i; . + 1); $all[.].commit == $all[$i].commit))) as $j
  | [$all[$i].commit, $all[$j].applied_at, sw($j), (if $i == 0 then "-" else $all[$i - 1].applied_at end), (if $i == 0 then "-" else sw($i - 1) end)] | join(" ")' "$applied" 2>/dev/null)
[ -n "$hit" ] || { echo "note 当てた記録 $applied に ${VERIFY_COMMIT:-今の commit} を当てた記録が無い（前のバージョンの記録か、古くて残っていないか、読めない）。当たっていた間が決まらないので確かめない"; exit 3; }
read -r a_commit a_at d_at a_until d_until <<<"$hit"
[ "$a_until" != - ] || a_until=
[ "$d_until" != - ] || d_until=
# 今の状態（dispatcher の端末・boot の生成物）を読む行は、最後に当てた時刻と比べる
now_ms=$(($(date -u -d "$(jq -r .applied_at "$applied")" +%s) * 1000))
echo "検証する commit $a_commit・当てた時刻 $a_at・次に当てた時刻 ${a_until:-無し（今も当たっている）}"
echo "dispatcher の記録を読む間: $d_at から ${d_until:-今}まで（dispatcher が切り替わった時刻。記録に無ければ当てた時刻）"
since() { cat "$@" 2>/dev/null | jq -c --arg t "$a_at" --arg u "$a_until" 'select(.at >= $t and ($u == "" or .at < $u))'; }
dsince() { cat "$@" 2>/dev/null | jq -c --arg t "$d_at" --arg u "$d_until" 'select(.at >= $t and ($u == "" or .at < $u))'; }
