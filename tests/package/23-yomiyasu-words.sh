# tests/package/23-yomiyasu-words.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  23. yomiyasu に観点の語を言い換えさせない: .yomiyasu-words/allow.txt の「正から写す語」の節が、レビュアーの依頼書（reviewerBrief の「見てほしいこと」）と worker skill §2 の観点の見出しの語と同じで、報告の言い方（外向きの操作・前の版）を含み、用語集の語を含まない。
# === 23. yomiyasu に観点の語を言い換えさせない（.yomiyasu-words/allow.txt） ===
# yomiyasu は git の最上位の .yomiyasu-words/allow.txt の語を直さない。一覧の「正から写す語」の節は、レビュアーの依頼書の
# 「見てほしいこと」の 1 の括弧の語・2 の正しさの語・3・5 の見出しと、worker skill §2 の観点の見出しを並べたものと同じ。
# 4 は文（テストが主張を本当に検証しているか）で語ではないので写さない。項目が 1〜5 から変わったら、この検査と一覧を直す。
# allow_miss <loop.go> <worker SKILL.md> <allow.txt> <用語集 SKILL.md>: 食い違いを1行で出す（空なら合っている）
allow_miss() {
  local m='' src got all n x words
  n=$(grep -cE '^[[:space:]]*"[0-9]+\. ' "$1")
  [ "$n" = 5 ] || m="$m [reviewerBrief の見てほしいことが 1〜5 でない（$n 項目）。検査 23 と allow.txt を直す]"
  src=$( {
    sed -n 's/.*"1\. [^（"]*（\([^）]*\)）\\n".*/\1/p' "$1"
    sed -n 's/.*"2\. 正しさ: \([^\\"]*\)\\n".*/\1/p' "$1"
    sed -n 's/.*"3\. \([^\\"]*\)\\n".*/\1/p' "$1"
    sed -n 's/.*"5\. \([^:"]*\): .*/\1/p' "$1"
    sed -n '/^## 2\. /,/^## 3\. /p' "$2" | sed -n '/PR を出す前に、3つの観点を自分の差分に当てる/,/当てた結果は/p' | sed -n 's/^  - \*\*\([^*]*\)\*\*.*/\1/p'
  } | sed 's/・/\n/g' | sed '/^$/d' | sort -u)
  [ "$(wc -l <<<"$src")" -ge 9 ] || m="$m [正の語を読めない: $(tr '\n' ' ' <<<"$src")]"
  got=$(sed -n '/^# begin: 正から写す語$/,/^# end: 正から写す語$/p' "$3" | grep -vE '^[[:space:]]*(#|$)' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sort -u)
  for x in $(comm -23 <(echo "$src") <(echo "$got")); do m="$m [正の語「$x」が allow.txt に無い]"; done
  for x in $(comm -13 <(echo "$src") <(echo "$got")); do m="$m [allow.txt の「$x」が正に無い]"; done
  all=$(grep -vE '^[[:space:]]*(#|$)' "$3" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
  for x in 失敗の波及 境界 外向きの操作 前の版; do grep -qxF -- "$x" <<<"$all" || m="$m [allow.txt に「$x」が無い]"; done
  words=$(sed -n 's/^\*\*\([^*]*\)\*\*.*/\1/p' "$4" | sed 's/／/\n/g' | sed '/^$/d')
  [ "$(wc -l <<<"$words")" -ge 20 ] || m="$m [用語集の語を読めない]"
  for x in $(grep -xF -f <(echo "$words") <<<"$all"); do m="$m [用語集の語「$x」が allow.txt にある]"; done
  echo "$m"
}
w=package/skills/worker/SKILL.md d=internal/dispatcher/loop.go a=.yomiyasu-words/allow.txt g=package/skills/glossary/SKILL.md
miss=$(allow_miss "$d" "$w" "$a" "$g")
[ -z "$miss" ] && pass "yomiyasu の allow.txt の正から写す語が、レビュアーの依頼書と worker §2 の観点の語と同じで、外向きの操作・前の版を含み、用語集の語を含まない" \
  || fail "yomiyasu の allow.txt:$miss"
# 検査が当たる場合があること: 正の語を変えた・一覧から語を抜いた・用語集の語を足した写しでは落ちる
miss=
sed 's/正しさ: 並行性・失敗の波及・境界/正しさ: 並行性・失敗の波及・境界・後始末/' "$d" >"$tmp/loop.go"
[ -n "$(allow_miss "$tmp/loop.go" "$w" "$a" "$g")" ] || miss="$miss [依頼書に語を足した]"
sed 's/^  - \*\*前の版の形\*\*/  - **古い形**/' "$w" >"$tmp/worker.md"
[ -n "$(allow_miss "$d" "$tmp/worker.md" "$a" "$g")" ] || miss="$miss [worker の見出しを変えた]"
for x in 境界 外向きの操作; do
  grep -vxF -- "$x" "$a" >"$tmp/allow.txt"
  [ -n "$(allow_miss "$d" "$w" "$tmp/allow.txt" "$g")" ] || miss="$miss [allow.txt から「$x」を抜いた]"
done
{ cat "$a"; echo 台帳; } >"$tmp/allow.txt"
[ -n "$(allow_miss "$d" "$w" "$tmp/allow.txt" "$g")" ] || miss="$miss [allow.txt に台帳を足した]"
[ -z "$miss" ] && pass "yomiyasu の allow.txt: 正を変えた・語を抜いた・用語集の語を足した写しでは落ちる（検査が見ている）" || fail "yomiyasu の allow.txt: 写しで落ちない:$miss"
