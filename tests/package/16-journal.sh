# tests/package/16-journal.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  16. 日報（ADR-0024 受け入れ条件 A1・A2）: worker skill の日報の節と工程表の振り返り、CoS skill の差し戻しの1行、用語集の「日報」。
# === 16. 日報（ADR-0024 A1・A2） ===
miss=
j=$(sed -n '/^### 日報（`library\/journal\.md`）/,/^### /p' "$w")
for k in 'issues/<id>/library/journal.md' 'その場で' '報告を書く前に' '## 感じたこと' '## 改善できそうなこと' '判定に使われない' '差し戻されない' '秘密・資格情報は書かない'; do
  grep -qF -- "$k" <<<"$j" || miss="$miss [$k]"
done
grep '^| 振り返り |' "$w" | grep -q '日報' || miss="$miss [工程表の振り返り]"
grep -q '日報.*検収の判定に使わない。日報の中身.*を理由に差し戻さない' "$pb/gate.md" || miss="$miss [CoS skill]"
grep -q '^\*\*日報\*\*' package/skills/glossary/SKILL.md || miss="$miss [用語集]"
[ -z "$miss" ] && pass "worker skill に日報の節（置き場・時機・2つの見出し・判定に使わない・秘密）と工程表の振り返り、CoS skill に差し戻さない1行、用語集に日報がある" || fail "日報:$miss"
