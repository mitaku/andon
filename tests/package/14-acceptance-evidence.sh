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

# ADR-0035 受け入れ条件 13 の前半（決めたこと 3）: Chief of Staff が課題を切るとき受け入れ条件の番号を予約キー acceptance で持たせる手順があり、
# brief.md の Settings の表・contract.md §2・worker skill §1 の表が同じキーを持つ（andon issue ready が行の無い報告を断る）
miss=
grep -q '^4\. 「依頼書に切る案」.*完了条件に受け入れ条件の番号を書く。同じ番号を .*`kanban set <id> acceptance=A4,A13`' "$pb/cut.md" || miss="$miss [cut.md の4に番号を持たせる手順]"
grep -q '^| `acceptance`（Acceptance） | .*`kanban set <id> acceptance=A4,A13`' "$pb/brief.md" || miss="$miss [brief.md の Settings の表]"
sed -n '/^## 2\. /,/^## 3\. /p' docs/contract.md | grep -q '^| `acceptance` | .*`andon issue ready`' || miss="$miss [contract.md §2]"
grep -q '^   | `acceptance`（Acceptance） |' "$w" && grep -q '番号ごとに同じ `id` の行が要る' "$w" || miss="$miss [worker skill §1・§7]"
[ -z "$miss" ] && pass "ADR-0035 A13: Chief of Staff の cut.md と brief.md に受け入れ条件の番号を予約キー acceptance で持たせる手順があり、contract.md §2 と worker skill が同じキーを持つ" || fail "ADR-0035 A13 の前半:$miss"
