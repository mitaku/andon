# tests/package/39-retro-gaps.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  39. 振り返りが申告と問いを読む（ADR-0061 受け入れ条件 11 のうち skill retro の分）: skill retro の読む表に束の gaps があり、
#      申告だけではパターンにしない（裏づけとして読む）と書いてあり、問いの繰り返しを自分の報告の gaps[] に missing で書く節がある。
#      問いと申告を根拠に指せないことも書いてある。
# === 39. 振り返りの申告と問い（skill retro） ===
# rgaps_miss <retro SKILL.md>: 足りないものを1行で出す（空なら合っている）
rgaps_miss() {
  local m=''
  grep -q '^| `gaps`（全文の束は本文まで、要約の束は種類と先） | .*裏づけとして読む |$' "$1" || m="$m [読む表に申告の行が無い]"
  grep -q '^- \*\*申告だけではパターンにしない。\*\* 申告は、ほかの跡.*裏づけとして読む。' "$1" || m="$m [申告を裏づけとして読む決まりが無い]"
  grep -q '^### 問いの繰り返しを申告に書く$' "$1" || m="$m [問いの繰り返しの節が無い]"
  grep -q '^- 当たったら、パターンには書かず、自分の報告の `gaps\[\]` に `missing` として1件書く' "$1" || m="$m [問いの繰り返しを missing の申告に書く行が無い]"
  grep -q '問いと申告は根拠に指せない' "$1" || m="$m [問いと申告を根拠に指せないと書いていない]"
  grep -q '`questions\[\]` は期間に聞かれた問い' "$1" || m="$m [集める節に questions[] が無い]"
  echo "$m"
}
rsk=package/skills/retro/SKILL.md
miss=$(rgaps_miss "$rsk")
[ -z "$miss" ] && pass "skill retro が申告を裏づけとして読み、問いの繰り返しを自分の報告の missing の申告に書き、問いと申告を根拠に指さない" \
  || fail "振り返りの申告と問い:$miss"
# 検査が当たる場合があること: 読む表の行・裏づけの決まり・節の見出し・missing の行・根拠の行・集める節を崩した写しでは落ちる
sed '/^| `gaps`（全文の束は本文まで/d' "$rsk" >"$tmp/rg1.md"
sed 's/^- \*\*申告だけではパターンにしない。\*\*/- 申告もパターンにする。/' "$rsk" >"$tmp/rg2.md"
sed '/^### 問いの繰り返しを申告に書く$/d' "$rsk" >"$tmp/rg3.md"
sed 's/として1件書く/として書く/' "$rsk" | sed 's/`gaps\[\]` に `missing`/`remember` に/' >"$tmp/rg4.md"
sed 's/問いと申告は根拠に指せない/問いも根拠に指せる/' "$rsk" >"$tmp/rg5.md"
sed 's/`questions\[\]` は期間に聞かれた問い/問い/' "$rsk" >"$tmp/rg6.md"
rg_caught=0
for c in rg1 rg2 rg3 rg4 rg5 rg6; do
  if [ -n "$(rgaps_miss "$tmp/$c.md")" ]; then rg_caught=$((rg_caught + 1)); fi
done
[ "$rg_caught" = 6 ] && pass "振り返りの申告と問いの検査は、表の行・裏づけ・節・missing の行・根拠・集める節を崩した写しで落ちる" \
  || fail "振り返りの申告と問いの検査が、崩した写し 6 つのうち $rg_caught つでしか落ちない"
