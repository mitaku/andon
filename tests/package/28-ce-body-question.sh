# ADR-0043 受け入れ条件 10: Chief Engineer は本文の問いで待つ前に、その問いを kanban に置く（決めたこと 11）
ce=package/skills/chief-engineer/SKILL.md
in3=$(grep -n '^3\. \*\*入口: 続けて範囲' "$ce" | cut -d: -f1)
in4=$(grep -n '^4\. \*\*真ん中' "$ce" | cut -d: -f1)
out7=$(grep -n '^7\. \*\*出口' "$ce" | cut -d: -f1)
out8=$(awk -v s="$out7" 'NR>s && /^[0-9]+\. \*\*/ {print NR; exit}' "$ce")
phrase='本文の問い.*答えを待ってターンを終える前に.*kanban に置く'
sed -n "${in3},$((in4 - 1))p" "$ce" | grep -q "$phrase" \
  && pass "ADR-0043 A10: chief-engineer の入口の段に、本文の問いを kanban に置く文がある" || fail "ADR-0043 A10: chief-engineer の入口の段に文が無い"
sed -n "${out7},$((${out8:-$((out7 + 10))} - 1))p" "$ce" | grep -q "$phrase" \
  && pass "ADR-0043 A10: chief-engineer の出口の段に、本文の問いを kanban に置く文がある" || fail "ADR-0043 A10: chief-engineer の出口の段に文が無い"
grep -q "$phrase" <(sed -n '/^### 1\.8 /,/^### 1\.9 /p' docs/design.md) \
  && pass "ADR-0043 A10: design.md §1.8 に同じ決まりがある" || fail "ADR-0043 A10: design.md §1.8 に決まりが無い"
printf '%s\n' '本文の問いで答えを待ってターンを終える前に、その問いを kanban に置く' | grep -q "$phrase" \
  && ! printf '%s\n' 'ダイアログの問いは今までどおり' | grep -q "$phrase" \
  && pass "ADR-0043 A10: 検査の語が当たる文に当たり、当たらない文に当たらない（検査が見ている）" || fail "ADR-0043 A10: 検査の語が何も見ていない"
