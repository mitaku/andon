# tests/package/33-owner-issue-new.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置く）。
#  33. オーナーの課題の作り方（ADR-0057 決めたこと 7・8。受け入れ条件 15 のうち ask-one-by-one と cut.md の分）。
#      ask-one-by-one §3 は判断を `andon issue ask --new` で、手作業を `only_owner` の印で作ると書き、
#      判断の課題を kanban add と kanban set who=person で作る古い手順と「判断がつかない」で貯める文が無い。
#      cut.md も同じ形を指す。無くなった文の検査は、古い文を置いた写しで当たることも確かめる。
# === 33. オーナーの課題の作り方（ADR-0057） ===
aobo3() { sed -n '/^## 3\. /,/^## 4\. /p' "$1"; }
s3=$(aobo3 package/skills/ask-one-by-one/SKILL.md)
grep -qF 'andon issue ask --new --now -' <<<"$s3" && grep -qF 'only_owner="<' <<<"$s3" \
  && pass "ADR-0057 A15: ask-one-by-one §3 は判断を andon issue ask --new、手作業を only_owner の印で作ると書く" \
  || fail "ADR-0057 A15: ask-one-by-one §3 に andon issue ask --new か only_owner の手順が無い"
# 古い手順: kanban add の結果を id に受けて、そのあと kanban set "$id" who=person を打つ。「判断がつかない」で貯める文
old_steps='^kanban set "\$id" who=person$|kind: 判断がつかない'
left=$(grep -nE "$old_steps" <<<"$s3" || true)
[ -z "$left" ] && pass "ADR-0057 A15: ask-one-by-one §3 に判断の課題を kanban set \"\$id\" who=person で作る古い手順と「判断がつかない」が無い" \
  || fail "ADR-0057 A15: ask-one-by-one §3 に古い手順が残る: $left"
# 検査が当たる場合があること: 古い §3 の行を置いた写しで、同じ grep が 2 行とも拾う
mkdir -p "$tmp/aobo"
printf '%s\n' '## 3. 問い' 'kanban set "$id" who=person' '`kind: 判断がつかない` だけで貯める' '## 4. 次' > "$tmp/aobo/SKILL.md"
[ "$(aobo3 "$tmp/aobo/SKILL.md" | grep -cE "$old_steps")" = 2 ] \
  && pass "ADR-0057 A15: 古い手順の検査は、古い §3 の写しで 2 行とも当たる" || fail "ADR-0057 A15: 古い手順の検査が古い §3 の写しに当たらない"
cutmd=package/skills/chief-of-staff/playbooks/cut.md
grep -qF 'andon issue ask --new --now -' "$cutmd" && grep -qF 'andon issue ask --new --later "<' "$cutmd" && grep -qF 'only_owner="<' "$cutmd" && grep -qF 'ask-one-by-one skill §3' "$cutmd" \
  && pass "ADR-0057 A15: cut.md はオーナーの課題を ask-one-by-one §3 と同じ形（andon issue ask --new・only_owner）で作ると書く" \
  || fail "ADR-0057 A15: cut.md がオーナーの課題の作り方（andon issue ask --new・only_owner）を指していない"
