# tests/package/32-answer-decided-by.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置く）。
#  32. skill が `kanban answer` を打たせる行は、全部 --decided-by を持つ（ADR-0049 決めたこと 6）。
#      当たる行が 6 行以上あること、blocked.md の「オーナーに聞くこと」の節に answer の行があることも見る。
# === 32. kanban answer の印（ADR-0049） ===
# answer_lines <フォルダ>: `kanban answer` を含む行を「ファイル:行」つきで出す
answer_lines() { grep -rnF 'kanban answer' "$1" --include='*.md'; }
total=$(answer_lines package/skills | wc -l)
bare=$(answer_lines package/skills | grep -vcF -e '--decided-by')
if [ "$total" -ge 6 ] && [ "$bare" = 0 ]; then
  pass "kanban answer の行 $total 行が全部 --decided-by を持つ"
else
  fail "kanban answer の行: 全 $total 行のうち --decided-by の無い行 $bare（6 行以上で全部に要る）: $(answer_lines package/skills | grep -vF -e '--decided-by' | cut -c1-80 | tr '\n' ' ')"
fi
# 検査が当たる場合があること: 印の無い行を足した写しで、数え方が落とす
mkdir -p "$tmp/ans"
printf '%s\n' '`kanban answer <id>#<n> "<答え>"`' > "$tmp/ans/x.md"
[ "$(answer_lines "$tmp/ans" | grep -vcF -e '--decided-by')" = 1 ] \
  && pass "印の無い kanban answer の行を数えられる" || fail "印の無い行を数えられない"
sec=$(awk '/^## /{on=($0=="## オーナーに聞くこと")} on' package/skills/chief-of-staff/playbooks/blocked.md)
grep -qF 'kanban answer' <<<"$sec" \
  && pass "blocked.md の「オーナーに聞くこと」に kanban answer の行がある" \
  || fail "blocked.md の「オーナーに聞くこと」に kanban answer の行が無い"
