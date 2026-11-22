# tests/package/27-cos-slots.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置き、前の節が置いた pb も使う）。
#  27. 並行の枠はオーナー待ちを数えない（ADR-0042 受け入れ条件 8 の後半。決めたこと 10・11）: 手引き start.md の枠の数え方が andon waiting を指し、
#      上限で起動を止める手引きが同じ枠を指す。古い数え方（`doing` と `review` の担当の数）が残らない。ask-one-by-one §3 の出す順が andon waiting の並びで読む。
# === 27. 並行の枠はオーナー待ちを数えない（ADR-0042） ===
start1=$(sed -n '/^1\. \*\*枠を確かめる\*\*/,/^2\. /p' "$pb/start.md")
miss=
for want in '`andon waiting --json`' '`slots.free` が 1 以上のときだけ' '`slots.waiting_full` が `true`' '`project.json` の `chief_of_staff.parallel`' '前の課題がオーナーのマージを待っていても先に起動しない' '`items[].holds`'; do
  grep -qF -- "$want" <<<"$start1" || miss="$miss [$want]"
done
[ -z "$miss" ] && pass "ADR-0042 A8: start.md の 1 が枠を andon waiting で数え、オーナー待ちが上限なら止め、マージ待ちの後ろの課題を先に起動しない" || fail "ADR-0042 A8: start.md の 1 に無い:$miss"

# 起動に触れる手引きが同じ枠（slots.free）を指す
miss=
for f in triage retro digest verify watch; do grep -qF '`andon waiting` の `slots.free`' "$pb/$f.md" || miss="$miss [$f.md]"; done
grep -qF '`start.md` の 1 で枠を数え直し' "$pb/gate.md" || miss="$miss [gate.md の数え直し]"
grep -qF '`start.md` の 1 で枠を数え直し' "$pb/finish.md" || miss="$miss [finish.md の数え直し]"
[ -z "$miss" ] && pass "ADR-0042 A8: 起動に触れる手引きが andon waiting の枠を指し、gate.md と finish.md が枠を数え直す" || fail "ADR-0042 A8: 枠を指していない手引き:$miss"

# 古い数え方: 並行の上限を `doing` と `review` の担当の数で数える文
old_slots='`doing`[^。]*`review`[^。]*担当の数|並行の上限(の中|の範囲|まで|とぶつかり|を確かめる)'
left=$(grep -nE "$old_slots" package/skills/chief-of-staff/SKILL.md "$pb"/*.md package/skills/ask-one-by-one/SKILL.md || true)
[ -z "$left" ] && pass "ADR-0042 A8: chief-of-staff と ask-one-by-one に古い枠の数え方が無い" || fail "ADR-0042 A8: 古い枠の数え方が残る: $left"
printf '%s\n' '1. 並行の上限を確かめる（`doing` と `review` の担当の数）。' | grep -qE "$old_slots" \
  && printf '%s\n' '依頼書を確かめ、並行の上限の中で `andon issue start`' | grep -qE "$old_slots" \
  && printf '%s\n' '並行の上限（`doing`・`review`の担当の数）' | grep -qE "$old_slots" \
  && ! printf '%s\n' '`doing` と `review` の担当の課題のうち、オーナー待ちでないもの' | grep -qE "$old_slots" \
  && pass "ADR-0042 A8: 古い数え方の検査が古い文に当たり、新しい文に当たらない（検査が見ている）" || fail "ADR-0042 A8: 古い数え方の検査が何も見ていない"

# ask-one-by-one §3 の出す順（決めたこと 11）
aobo3=$(sed -n '/^## 3\. /,/^## 4\. /p' package/skills/ask-one-by-one/SKILL.md)
grep -q '^- \*\*出す順\*\*: 止めている作業が多いもの.*`andon waiting --json` の `items` の並び' <<<"$aobo3" \
  && pass "ADR-0042 A8: ask-one-by-one §3 の出す順が、止めている作業の多さを andon waiting の並びで読む" || fail "ADR-0042 A8: ask-one-by-one §3 の出す順が andon waiting の並びを指さない"
