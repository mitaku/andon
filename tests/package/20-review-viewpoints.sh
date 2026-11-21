# tests/package/20-review-viewpoints.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  20. PR を出す前の3つの観点: worker skill §2 の観点（失敗の波及・前のバージョンの形・落ちたあとの起き直し）がレビュアーの依頼書（reviewerBrief・independent-review）の「正しさ」の語で書かれ、§2 の工程表・§4・§7 の書式にある。
#  20b. 検査を足すか変える PR の落ちるべき入力の変種: worker skill §2 に変種（引用符・空白の数・在りえない名前・範囲の外のファイル・除外した箇所）を確かめる項が3つの観点の外にあり、工程表・§4・§7 の書式から指されている。
# === 20. PR を出す前の3つの観点（worker skill §2・§4・§7 とレビュアーの依頼書） ===
# 担当が当てる観点は、レビュアーの依頼書（dispatcher の reviewerBrief と independent-review skill の型）の「正しさ」の語で書く。
# 3か所の「正しさ」の語の並びが同じで、担当の3つの観点の見出しと括弧の語が、その並びの中の語であること。
w=package/skills/worker/SKILL.md
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w"); w4=$(sed -n '/^## 4\. /,/^## 5\. /p' "$w"); w7=$(sed -n '/^## 7\. /,$p' "$w")
# 正しさの語の並び（「・」で区切った語を1行ずつ）
brief_ok=$(sed -n 's/.*"2\. 正しさ: \([^\\"]*\)\\n".*/\1/p' internal/dispatcher/loop.go)
ir_ok=$(sed -n 's/.*正しさ（\([^）]*\)）.*/\1/p' package/skills/independent-review/SKILL.md)
w_ok=$(sed -n 's/.*「正しさ: \([^」]*\)」.*/\1/p' <<<"$w2")
# 担当の観点: 「  - **<語>**」の見出しと、「（レビュアーの「<語>」の1つ）」の語
views=$(sed -n 's/^  - \*\*\([^*]*\)\*\*.*/\1/p' <<<"$(sed -n '/PR を出す前に、3つの観点を自分の差分に当てる/,/当てた結果は/p' <<<"$w2")")
parents=$(grep -o '（レビュアーの「[^」]*」の1つ）' <<<"$w2" | sed 's/（レビュアーの「\(.*\)」の1つ）/\1/')
# in_list <語> <並び>: 語が「・」区切りの並びの中にあるか
in_list() { sed 's/・/\n/g' <<<"$2" | grep -qxF -- "$1"; }
miss=
[ -n "$brief_ok" ] && [ "$brief_ok" = "$ir_ok" ] && [ "$brief_ok" = "$w_ok" ] \
  || miss="$miss [正しさの語が食い違う: reviewerBrief=$brief_ok independent-review=$ir_ok worker=$w_ok]"
[ "$(tr '\n' '|' <<<"$views")" = '失敗の波及|前のバージョンの形|落ちたあとの起き直し|' ] || miss="$miss [worker §2 の観点の見出し: $(tr '\n' ' ' <<<"$views")]"
[ "$(wc -l <<<"$parents")" = 2 ] || miss="$miss [worker §2 の観点に「レビュアーの「…」の1つ」が2つ無い]"
for v in 失敗の波及 $parents; do in_list "$v" "$brief_ok" || miss="$miss [観点の語「$v」がレビュアーの正しさ（$brief_ok）に無い]"; done
grep -q '^| 確認 |.*PR を出す前の3つの観点' <<<"$w2" || miss="$miss [worker §2 の工程表の確認に3つの観点]"
grep -q '3つの観点〈§2〉を当てた行は、PR を出す前に報告に書いておく' <<<"$w4" || miss="$miss [worker §4 の 1 に3つの観点]"
for v in $views; do grep -q "^$v: <" <<<"$w7" || miss="$miss [worker §7 の書式に「$v:」の行]"; done
[ -z "$miss" ] && pass "worker skill の PR を出す前の3つの観点が、レビュアーの依頼書（reviewerBrief・independent-review）の正しさの語（$brief_ok）で書かれ、§2 の工程表・§4・§7 の書式にある" || fail "3つの観点:$miss"
# 検査が当たる場合があること: 依頼書の正しさの語が変わると、担当の並びと食い違う
[ "$(sed -n 's/.*"2\. 正しさ: \([^\\"]*\)\\n".*/\1/p' <<<'		"2. 正しさ: 並行性・境界\n" +')" != "$w_ok" ] && ! in_list 失敗の波及 '並行性・境界' \
  && pass "3つの観点: 依頼書の正しさの語が変わった形は食い違いとして当たる（検査が見ている）" || fail "3つの観点: 検査が何も見ていない"
# === 20b. 検査を足すか変える PR の、落ちるべき入力の変種（worker skill §2・§4・§7） ===
# 担当が足した検査が、狙いの1つの入力では落ちても、同じ誤りの変種（引用符・空白の数・在りえない名前・範囲の外のファイル・除外した箇所）を
# 素通りさせて、1回目のレビューで major になることが重なった。worker skill §2 に変種を確かめる項があり、3つの観点の並び（検査 20）の外に置かれ、
# 工程表の確認・§4 の 1・§7 の書式から指されていること。
# variants_miss <worker SKILL.md>: 足りないものを1行で出す（空なら合っている）
variants_miss() {
  local m='' s2 s4 s7 item v
  s2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$1"); s4=$(sed -n '/^## 4\. /,/^## 5\. /p' "$1"); s7=$(sed -n '/^## 7\. /,$p' "$1")
  item=$(sed -n '/^- \*\*検査を足すか変える PR では、落ちるべき入力の変種で落ちることを確かめる\*\*/,/^- \*\*/p' <<<"$s2" | sed '$d')
  [ -n "$item" ] || m="$m [worker §2 に「検査を足すか変える PR では、落ちるべき入力の変種で落ちることを確かめる」の項]"
  for v in 引用符 空白の数 在りえない名前 範囲の外のファイル 除外した箇所; do grep -qF -- "$v" <<<"$item" || m="$m [worker §2 の変種の項に「$v」]"; done
  grep -qF '検査が当たる場合があること' <<<"$item" || m="$m [worker §2 の変種の項に、落ちない変種を自己確認に足すこと]"
  sed -n '/PR を出す前に、3つの観点を自分の差分に当てる/,/当てた結果は/p' <<<"$s2" | grep -qF '落ちるべき入力' && m="$m [変種の項が3つの観点の並びの中にある]"
  grep -q '^| 確認 |.*落ちるべき入力の変種' <<<"$s2" || m="$m [worker §2 の工程表の確認に落ちるべき入力の変種]"
  grep -qF '落ちるべき入力〈§2〉の行も' <<<"$s4" || m="$m [worker §4 の 1 に落ちるべき入力の行]"
  grep -q '^落ちるべき入力: <.*検査を足すか変える PR のとき' <<<"$s7" || m="$m [worker §7 の書式に「落ちるべき入力:」の行]"
  echo "$m"
}
miss=$(variants_miss "$w")
[ -z "$miss" ] && pass "worker skill §2 に、検査を足すか変える PR で落ちるべき入力の変種を確かめる項があり、工程表・§4・§7 の書式から指されている" || fail "落ちるべき入力の変種:$miss"
# 検査が当たる場合があること: 項を消した・変種の語を1つ抜いた・3つの観点の並びに入れた・工程表か §4 か §7 から消した写しでは落ちる
miss=
grep -vF '検査を足すか変える PR では、落ちるべき入力の変種で落ちることを確かめる' "$w" >"$tmp/worker.md"
[ -n "$(variants_miss "$tmp/worker.md")" ] || miss="$miss [項を消した]"
for v in 引用符 空白の数 在りえない名前 範囲の外のファイル 除外した箇所; do
  sed "/^- \*\*検査を足すか変える PR では/,/^- \*\*/s/$v/X/g" "$w" >"$tmp/worker.md"
  [ -n "$(variants_miss "$tmp/worker.md")" ] || miss="$miss [変種「$v」を抜いた]"
done
sed 's/^  - 当てた結果は、/  - 落ちるべき入力も見る。当てた結果は、/' "$w" >"$tmp/worker.md"
[ -n "$(variants_miss "$tmp/worker.md")" ] || miss="$miss [3つの観点の並びに入れた]"
grep -v '^落ちるべき入力: <' "$w" >"$tmp/worker.md"
[ -n "$(variants_miss "$tmp/worker.md")" ] || miss="$miss [§7 の行を消した]"
sed 's/・検査を足すか変える PR は落ちるべき入力の変種（下）//' "$w" >"$tmp/worker.md"
[ -n "$(variants_miss "$tmp/worker.md")" ] || miss="$miss [工程表の確認から消した]"
sed 's/。検査を足すか変える PR は、落ちるべき入力〈§2〉の行も）/）/' "$w" >"$tmp/worker.md"
[ -n "$(variants_miss "$tmp/worker.md")" ] || miss="$miss [§4 の 1 から消した]"
[ -z "$miss" ] && pass "落ちるべき入力の変種: 項・語・置き場・工程表と §4 と §7 の行を崩した写しでは落ちる（検査が見ている）" || fail "落ちるべき入力の変種: 写しで落ちない:$miss"
