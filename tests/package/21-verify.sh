# tests/package/21-verify.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  21. 当てたあとの検証（ADR-0035 受け入れ条件 13 の後半）: CoS の手引き verify.md に、差分が地図に当たるときだけ切る・同時に1つ・前の検証が見た commit から見る手順があり、名指す検証の skill と features.sh が在る。依頼書の雛形は、地図を直す PR がレベル B で Review: off でもレビューが要ることと、gate が読む Must fix を書く。
# === 21. 当てたあとの検証（ADR-0035 受け入れ条件 13 の後半・決めたこと 4・9） ===
# verify_miss <手引き>: CoS の手引きに、切る条件（地図に当たる・同時に1つ・前の検証が見た commit から）と、依頼書が名指す skill がそろうか
verify_miss() {
  local m='' k
  for k in '同時に動く検証の課題は1つ' 'select(.title | startswith("検証: "))' '前の検証が見た commit から見る' \
    'bash "$d/.claude/skills/verify-andon/scripts/features.sh" <始まり>..<当てた commit>' \
    '差分が地図の「変えたら動かすパス」に当たらない' '読む skill: .claude/skills/verify-andon/SKILL.md' \
    '前の検証が見た commit: <始まり>' 'kanban review <id> で返す' \
    '   レビュー: 地図を直す PR は .claude/skills/ の中なのでレベル B になり、Review: off でも gate が独立レビューを求める' '   - Review: off' '   - Must fix: major' '「当てたら」の 2 で始まりの commit を決め直す' '報告に見た範囲が無い（診断で止まった）ときも' 'それを当てた commit として「当てたら」の 3 から回す'; do
    grep -qF -- "$k" "$1" || m="$m [$k]"
  done
  echo "$m"
}
miss=$(verify_miss "$pb/verify.md")
# 手引きが名指すものが repo に在り、検証の skill が依頼書の同じ行を読む
for f in .claude/skills/verify-andon/SKILL.md .claude/skills/verify-andon/scripts/features.sh; do [ -f "$f" ] || miss="$miss [$f が無い]"; done
grep -qF '前の検証が見た commit' .claude/skills/verify-andon/SKILL.md || miss="$miss [検証の skill が「前の検証が見た commit」を読まない]"
grep -q '^| マージした変更をセルへ当てた.* | `verify.md` |$' "$cos" || miss="$miss [SKILL.md の目次に verify.md]"
[ -z "$miss" ] && pass "ADR-0035 A13 後半: CoS の手引き verify.md に、差分が地図に当たるときだけ・同時に1つ・前の検証が見た commit から検証の課題を切る手順がある" || fail "ADR-0035 A13 後半:$miss"
# 検査が当たる場合があること: 同時に1つの行を消した手引きでは落ちる
grep -vF '同時に動く検証の課題は1つ' "$pb/verify.md" >"$tmp/verify.md"
[ -n "$(verify_miss "$tmp/verify.md")" ] && pass "ADR-0035 A13 後半: 同時に1つの行が無い手引きは当たる（検査が見ている）" || fail "ADR-0035 A13 後半: 検査が何も見ていない"
# 雛形のレビューの行を雛形の外（行頭の字下げ無し）へ動かした手引きと、Review を on にした手引きも当たる
sed -e 's/^   レビュー: /レビュー: /' "$pb/verify.md" >"$tmp/verify.md"
[ -n "$(verify_miss "$tmp/verify.md")" ] && pass "ADR-0035 A13 後半: 雛形の外へ動かしたレビューの行は当たる（検査が見ている）" || fail "ADR-0035 A13 後半: 雛形の外のレビューの行で通る"
sed -e 's/^   - Review: off$/   - Review: on/' "$pb/verify.md" >"$tmp/verify.md"
[ -n "$(verify_miss "$tmp/verify.md")" ] && pass "ADR-0035 A13 後半: Review を on にした雛形は当たる（検査が見ている）" || fail "ADR-0035 A13 後半: Review を on にした雛形で通る"
