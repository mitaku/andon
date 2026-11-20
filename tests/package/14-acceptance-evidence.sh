# tests/package/14-acceptance-evidence.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  14. 受け入れ条件の証拠の質（ADR-0007 R5）: contract.md §4 の acceptance[] の行・CE skill・worker skill §7 に、証拠はテストの command と出力のパスで staged な出力と mock は証拠でない。
# === 14. 受け入れ条件の証拠の質（ADR-0007 R5・A4） ===
miss=
c4=$(sed -n '/^## 4\. /,/^## 5\. /p' docs/contract.md | grep '^| `acceptance\[\]` |')
for f in "$c4" "$(grep '^\*\*受け入れ条件の節\*\*' "$ce")" "$(grep '^- `acceptance`:' "$w")"; do
  grep -q 'テストの command と' <<<"$f" && grep -q '出力' <<<"$f" && grep -q 'staged な）出力' <<<"$f" && grep -q 'mock に置き換えたテスト' <<<"$f" && grep -q '証拠でない' <<<"$f" \
    || miss="$miss [$(cut -c1-30 <<<"$f")]"
done
[ -z "$miss" ] && pass "contract.md §4 の acceptance[] の行・CE skill の受け入れ条件の節・worker skill §7 に、証拠はテストの command と出力のパスで staged な出力と mock は証拠でない、とある" || fail "受け入れ条件の証拠の質:$miss"
