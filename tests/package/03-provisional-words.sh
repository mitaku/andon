# tests/package/03-provisional-words.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   3. skill とプロファイルに仮設の語が残っていない（用語集の「仮設の語」の節だけは対応表として持つ）。
# === 3. 仮設の語 ===
# 用語集は「## 仮設の語」の節より前だけ、「_避ける_」の行を除いて見る。動詞の「知らせる」は語ではないので「の知らせ」だけを見る
for f in package/skills/*/SKILL.md package/skills/*/playbooks/*.md package/skills/*/assets/* package/templates/*.md; do
  case $f in package/skills/glossary/SKILL.md) sed '/^## 仮設の語/,$d; /^_避ける_/d' "$f" ;; *) cat "$f" ;; esac >"$tmp/body"
  hits=$(grep -nE -i '\byard\b|yard-|template-yard|コーディネーター|担当エージェント|の知らせ|推し|段 ?[ABC]\b|merge-gate|/workspace/\.claude' "$tmp/body" || true)
  [ -z "$hits" ] && pass "$f に仮設の語が無い" || fail "$f に仮設の語: $(echo "$hits" | head -3)"
done
grep -q '^## 仮設の語' package/skills/glossary/SKILL.md && pass "用語集は仮設の語の対応表を末尾に持つ" || fail "用語集に仮設の語の節が無い"
