# tests/package/37-report-gaps.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  37. 知識の欠けの申告（ADR-0061 受け入れ条件 11 のうち担当と Chief Engineer の skill の分）: worker skill の振り返りの工程が申告を名指し、
#      §7 の報告の書式に ## Gaps の節があり、--data の gaps を引き継ぐ。レビュアーの節（§6）には申告を足していない。
#      Chief Engineer の skill の渡し方に申告の欄がある。
# === 37. 報告の申告の欄 gaps（worker skill §2・§6・§7、chief-engineer skill の渡し方） ===
# gaps_miss <worker SKILL.md> <chief-engineer SKILL.md>: 足りないものを1行で出す（空なら合っている）
gaps_miss() {
  local m='' review report
  grep -q '^| 振り返り |.*無かった、誤っていた、辿り着くのが遅れた.*`gaps`.*| .*`## Gaps`' "$1" || m="$m [振り返りの工程に申告が無い]"
  report=$(sed -n '/^## 7\. /,$p' "$1")
  grep -q '^## Gaps ' <<<"$report" || m="$m [§7 の報告の書式に ## Gaps が無い]"
  grep -q '^- `gaps`: 知識の欠けの申告。.*`missing`.*`wrong`.*`hard_to_find`' <<<"$report" || m="$m [§7 に gaps の3つの種類が無い]"
  grep -qF '`gaps`（`## Gaps` の写し）も `--data` に無ければ前の `report.json` から引き継ぐ' <<<"$report" || m="$m [§7 に gaps の引き継ぎが無い]"
  grep -qF '名指せる欠けは `gaps` に、名指せない学びは `remember` に書き' <<<"$report" || m="$m [§7 に申告と Remember の分け方が無い]"
  grep -qF '欠けが無ければ書かない' <<<"$report" || m="$m [§7 に欠けが無ければ書かないことが無い]"
  review=$(sed -n '/^## 6\. /,/^## 7\. /p' "$1")
  grep -qi 'gaps\|申告' <<<"$review" && m="$m [レビュアーの節（§6）に申告がある]"
  grep -q '^   \*\*申告の欄\*\*:.*`gaps\[\]`.*`## Gaps`' <(sed -n '/^## 渡し方/,/^## 取らないもの/p' "$2") || m="$m [Chief Engineer の渡し方に申告の欄が無い]"
  echo "$m"
}
miss=$(gaps_miss package/skills/worker/SKILL.md package/skills/chief-engineer/SKILL.md)
[ -z "$miss" ] && pass "worker skill の振り返りと報告の書式に申告 gaps があり、レビュアーの節には無く、Chief Engineer の渡し方に申告の欄がある" \
  || fail "申告の欄:$miss"
# 検査が当たる場合があること: 書式の ## Gaps を抜いた・引き継ぎを消した・レビュアーの節に足した・CE の渡し方から抜いた写しでは落ちる
sed '/^## Gaps /d' package/skills/worker/SKILL.md >"$tmp/gaps-w1.md"
sed 's/`gaps`（`## Gaps` の写し）も `--data` に無ければ前の `report.json` から引き継ぐ。//' package/skills/worker/SKILL.md >"$tmp/gaps-w2.md"
sed 's/^## 6\. レビュアーのとき$/&\nレビュアーも gaps を書く。/' package/skills/worker/SKILL.md >"$tmp/gaps-w3.md"
sed 's/^| 振り返り |.*$/| 振り返り | 次の課題への学び | `## Remember` |/' package/skills/worker/SKILL.md >"$tmp/gaps-w4.md"
sed '/^   \*\*申告の欄\*\*:/d' package/skills/chief-engineer/SKILL.md >"$tmp/gaps-ce.md"
gaps_caught=0
for w in "$tmp/gaps-w1.md" "$tmp/gaps-w2.md" "$tmp/gaps-w3.md" "$tmp/gaps-w4.md"; do
  if [ -n "$(gaps_miss "$w" package/skills/chief-engineer/SKILL.md)" ]; then gaps_caught=$((gaps_caught + 1)); fi
done
if [ -n "$(gaps_miss package/skills/worker/SKILL.md "$tmp/gaps-ce.md")" ]; then gaps_caught=$((gaps_caught + 1)); fi
[ "$gaps_caught" = 5 ] && pass "申告の欄の検査は、書式・引き継ぎ・レビュアーの節・振り返り・Chief Engineer の渡し方を崩した写しで落ちる" \
  || fail "申告の欄の検査が、崩した写し 5 つのうち $gaps_caught つでしか落ちない"
