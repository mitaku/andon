# tests/package/36-worker-start-text.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  36. 担当の起動の文と worker skill の線: 担当のプロファイル（claude・codex・sonnet・fable）の本文が、ほかの担当・レビュアー・プロファイルと
#      claude・codex の command の起動を断り、見るだけでその turn のうちに結果を返して終わるサブエージェントを許す。
#      worker skill の表と冒頭も同じ線で書き、§4 の反論の段が起動の文に断られない。
# === 36. 担当の起動の文と worker skill の表（worker skill 冒頭・§4） ===
# 起動の文が「ほかのエージェントを起動しない」だけだと、許可の判定が反論の段のサブエージェントを断る。
# 許す範囲を広く書くと、担当が長く走るエージェントや claude・codex の command を起動しうる。どちらに戻っても落とす。
start_deny='ほかの担当・レビュアー・プロファイルは起動しない（`hc run` でも、`claude`・`codex` を command で打つのでも起動しない）。'
start_allow='見るだけで書き換えず、その turn のうちに結果を返して終わるサブエージェントは起動してよい'
# start_miss <テンプレート> <worker SKILL.md>: 足りないものを1行で出す（空なら合っている）
start_miss() {
  local m='' body head
  body=$(awk 'n >= 2; /^---$/ && n < 2 {n++}' "$1")
  grep -qF "$start_deny" <<<"$body" || m="$m [起動の文に断る線（担当・レビュアー・プロファイル、hc run、claude・codex の command）]"
  grep -qF "$start_allow" <<<"$body" || m="$m [起動の文に許す線（見るだけで、その turn のうちに結果を返して終わるサブエージェント）]"
  grep -qF 'ほかのエージェントを起動しない' <<<"$body" && m="$m [起動の文に前の「ほかのエージェントを起動しない」が残る]"
  head=$(sed -n '/^| する | しない |$/,/^## 1\. /p' "$2")
  grep -q '^| 読むだけでその turn のうちに返るサブエージェントを起動する.*| ほかの担当・レビュアー・プロファイルを起動する.*`claude`・`codex` を command で起動する.*turn を越えて走るサブエージェント' <<<"$head" \
    || m="$m [worker の表の「する」「しない」が起動の文と同じ線でない]"
  grep -qF "「$start_allow」" <<<"$head" || m="$m [worker の冒頭が起動の文の許す線を引いていない]"
  grep -q '^- \*\*起動してよい\*\*:.*「ほかの担当・レビュアー・プロファイルは起動しない」に当たらない' "$2" || m="$m [worker §4 の反論の段に、起動の文に当たらないことが無い]"
  echo "$m"
}
w=package/skills/worker/SKILL.md
for p in claude codex sonnet fable; do
  miss=$(start_miss "package/templates/$p.md" "$w")
  [ -z "$miss" ] && pass "プロファイル $p の起動の文が、ほかのエージェントと claude・codex の command を断り、見るだけでその turn のうちに返るサブエージェントを許し、worker skill の表と冒頭・§4 と同じ線で書く" \
    || fail "プロファイル $p の起動の文:$miss"
done
# 検査が当たる場合があること: 前の文に戻した・断る線を抜いた・許す線を広げた・skill の表や冒頭を戻した写しでは落ちる
miss=
t=package/templates/claude.md
sed "s/書く場所は skill の決まりに従う。.*/書く場所は skill の決まりに従い、ほかのエージェントを起動しない。/" "$t" >"$tmp/start.md"
[ -n "$(start_miss "$tmp/start.md" "$w")" ] || miss="$miss [前の文に戻した]"
sed 's/、`claude`・`codex` を command で打つのでも起動しない//' "$t" >"$tmp/start.md"
[ -n "$(start_miss "$tmp/start.md" "$w")" ] || miss="$miss [claude・codex の command を抜いた]"
sed 's/見るだけで書き換えず、その turn のうちに結果を返して終わる/ファイルを書かない/' "$t" >"$tmp/start.md"
[ -n "$(start_miss "$tmp/start.md" "$w")" ] || miss="$miss [許す線を広げた]"
sed 's/・turn を越えて走るサブエージェント//' "$w" >"$tmp/worker.md"
[ -n "$(start_miss "$t" "$tmp/worker.md")" ] || miss="$miss [表から turn を越えて走るサブエージェントを抜いた]"
grep -vF "「$start_allow」" "$w" >"$tmp/worker.md"
[ -n "$(start_miss "$t" "$tmp/worker.md")" ] || miss="$miss [冒頭から起動の文の引用を消した]"
[ -z "$miss" ] && pass "起動の文の検査は、前の文・断る線の抜け・広い許し・skill の表や冒頭の戻しで落ちる（検査が見ている）" || fail "起動の文の検査が落ちない:$miss"
