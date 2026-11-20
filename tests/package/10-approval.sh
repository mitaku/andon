# tests/package/10-approval.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  10. 承認の段・書く場所・字数・reach（A-0122）。
# === 10. 承認の段・書く場所・字数・reach（A-0122。A-0117 の c7・c8・c11・c17） ===
w=package/skills/worker/SKILL.md
w5=$(sed -n '/^## 5\. /,/^## 6\. /p' "$w")
grep -q '^| 承認が要る（「オーナーの go」で受ける） | 承認は要らない（聞かずにする） |$' <<<"$w5" && grep -q 'force-with-lease` も含む' <<<"$w5" \
  && pass "worker skill §5 に承認の要る操作と要らない操作が並ぶ（自分の PR の branch への force-with-lease は要らない側）" || fail "worker skill §5 に承認の段の表が無い"
[ "$(grep -c '\*\*書く場所\*\*' "$w")" = 1 ] && ! grep -q 'worktree の外にファイルを書' "$w" package/templates/*.md \
  && pass "書く場所は worker skill §2 の1か所（skill のほかの節と template に「worktree の外に書かない」が無い）" || fail "書く場所が1か所に寄っていない"
! grep -qE '100字|1〜2行|1〜3 行' "$w" && grep -q '1行に1件（Chief of Staff とトリアージが行ごとに拾う）' "$w" \
  && pass "worker skill の報告の書式に字数の上限が無く、1行に1件と理由がある" || fail "worker skill の報告の書式に字数の上限が残る"
grep -q '`reach`.*指示ではない' "$w" && pass "worker skill に hc の reach は指示ではないとある" || fail "worker skill に reach の扱いが無い"
