# tests/package/44-merge-request-holds.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置く）。
#  44. マージの依頼に後ろで待つ課題の数を書き足す（ADR-0042 決めたこと 16）: ask-one-by-one skill の §2 の 1 と、手引き gate.md の
#      「## `needs_human` のとき」の 3 に、`holds` が1件以上のときだけ「この後ろで N 件が待っている」を書き足す文がある。
#      §2 の文は、ほかの返事にオーナー待ちの件数を添える行を足さないとも言う。
#      どの検査も、その文を消した写しで当たることを確かめる。
# === 44. マージの依頼に後ろで待つ課題の数（ADR-0042 決めたこと 16） ===
# holds_miss <ask-one-by-one SKILL.md> <gate.md>: 足りないものを1行で出す（空なら合っている）
holds_miss() {
  local m='' aobo2 gate3
  aobo2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$1")
  gate3=$(sed -n '/^## `needs_human` のとき/,/^## /p' "$2" | grep '^3\. ' || true)
  grep -qF '`holds`' <<<"$aobo2" && grep -qF '「この後ろで N 件が待っている」' <<<"$aobo2" \
    || m="$m [ask-one-by-one §2 にマージの依頼へ後ろで待つ数を書き足す文が無い]"
  grep -qF 'ほかの返事にオーナー待ちの件数を添える行は足さない' <<<"$aobo2" \
    || m="$m [ask-one-by-one §2 にほかの返事へ件数を添えない文が無い]"
  grep -qF '`holds`' <<<"$gate3" && grep -qF '「この後ろで N 件が待っている」' <<<"$gate3" \
    || m="$m [gate.md の 3 にマージの依頼へ後ろで待つ数を書き足す文が無い]"
  printf '%s' "$m"
}
miss=$(holds_miss package/skills/ask-one-by-one/SKILL.md package/skills/chief-of-staff/playbooks/gate.md)
[ -z "$miss" ] && pass "ADR-0042 決めたこと 16: マージの依頼に「この後ろで N 件が待っている」を書き足す文が ask-one-by-one §2 と gate.md の 3 にある" \
  || fail "ADR-0042 決めたこと 16:$miss"
# 検査が当たる場合があること: 文を消した写しでは、どちらのファイルでも当たる
cut44=$(mktemp -d)
grep -vF '「この後ろで N 件が待っている」' package/skills/ask-one-by-one/SKILL.md >"$cut44/aobo.md" || true
grep -vF '「この後ろで N 件が待っている」' package/skills/chief-of-staff/playbooks/gate.md >"$cut44/gate.md" || true
miss_a=$(holds_miss "$cut44/aobo.md" package/skills/chief-of-staff/playbooks/gate.md)
miss_g=$(holds_miss package/skills/ask-one-by-one/SKILL.md "$cut44/gate.md")
rm -rf "$cut44"
if [[ "$miss_a" == *'ask-one-by-one §2 にマージの依頼'* ]] && [[ "$miss_g" == *'gate.md の 3'* ]]; then
  pass "ADR-0042 決めたこと 16: 文を消した写しは検査に当たる（検査が見ている）"
else
  fail "ADR-0042 決めたこと 16: 文を消した写しでも検査が通る（a=[$miss_a] g=[$miss_g]）"
fi
