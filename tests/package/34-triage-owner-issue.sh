# tests/package/34-triage-owner-issue.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置く）。
#  34. トリアージが足すオーナーの課題の形（ADR-0057 決めたこと 9・10。受け入れ条件 15 のうちトリアージの分）。
#      triage skill の判定表の 5 は手作業を only_owner で、6 は判断を kind（line を含む）と options で、10 は
#      いちばん近い kind に unsure を付けて出し、decisions.json の節も同じ形を書く。前の形（kind 無しの person・pick・
#      「判断がつかない」）の文が無い。CoS の手引き triage.md は ask-one-by-one §3 と同じ形と、unsure で数える
#      「線引きを見直す」を指す。無くなった文の検査は、古い文を置いた写しで当たることも確かめる。
# === 34. トリアージが足すオーナーの課題（ADR-0057） ===
tskill=package/skills/triage/SKILL.md
trow() { grep -E "^\| $1 \|" "$2"; }
trow 5 "$tskill" | grep -qF '`only_owner`' \
  && trow 6 "$tskill" | grep -qF '`kind: business\|users\|promise\|irreversible\|line`' && trow 6 "$tskill" | grep -qF '`options`' \
  && trow 10 "$tskill" | grep -qF '`"unsure": true`' \
  && pass "ADR-0057 A15: triage skill の判定表の 5 は only_owner、6 は line を含む kind と options、10 は unsure で出す" \
  || fail "ADR-0057 A15: triage skill の判定表の 5・6・10 が決めたこと 9・10 の形でない"
# decisions.json の節（## 4.）: 判断の例は options、手作業の例は only_owner を持ち、apply が断る形と unsure の書き方がある。
tsec4() { sed -n '/^## 4\. /,/^## 5\. /p' "$1"; }
s4=$(tsec4 "$tskill")
grep -qF '"options": [' <<<"$s4" && grep -qF '"only_owner": "' <<<"$s4" && grep -qF '"unsure": true' <<<"$s4" \
  && grep -qF '`kind` も `only_owner` も無い・両方ある' <<<"$s4" \
  && pass "ADR-0057 A15: triage skill の decisions.json の節は判断（options）と手作業（only_owner）と unsure の形を書く" \
  || fail "ADR-0057 A15: triage skill の decisions.json の節に判断・手作業・unsure の形が無い"
# 前の形: 種類の無い person、推奨案の pick、「判断がつかない」と「kind 無し」で数える文。
old_person='"pick": "|`kind` を付けない|`kind` 無し|判断がつかない|`pick`（推奨案）'
left=$(grep -nE "$old_person" "$tskill" || true)
[ -z "$left" ] && pass "ADR-0057 A15: triage skill に前の person の形（kind 無し・pick・判断がつかない）が無い" \
  || fail "ADR-0057 A15: triage skill に前の person の形が残る: $left"
# 検査が当たる場合があること: 前の判定表と decisions.json の行を置いた写しで、同じ grep が 4 行とも拾う
mkdir -p "$tmp/tskill"
printf '%s\n' '| 5 | 実機 | `person`（`kind` を付けない。`question` に期限の目安） |' \
  '| 10 | それ以外（判断がつかない） | `person` |' \
  '     "question": "q", "pick": "よい"},' \
  '- `person` は `question` が必須。`kind` があれば `pick`（推奨案）が必須。' > "$tmp/tskill/SKILL.md"
[ "$(grep -cE "$old_person" "$tmp/tskill/SKILL.md")" = 4 ] \
  && pass "ADR-0057 A15: 前の person の形の検査は、前の skill の写しで 4 行とも当たる" || fail "ADR-0057 A15: 前の person の形の検査が前の skill の写しに当たらない"
ttri=package/skills/chief-of-staff/playbooks/triage.md
grep -qF 'ask-one-by-one skill §3 と同じ形' "$ttri" && grep -qF '`andon issue ask --new`' "$ttri" && grep -qF '`only_owner`' "$ttri" \
  && grep -qF '種類を決めきれなかった判断〈`unsure`〉が 3 を超えた' "$ttri" \
  && pass "ADR-0057 A15: CoS の手引き triage.md は ask-one-by-one §3 と同じ形（andon issue ask --new・only_owner）と unsure の線引きを指す" \
  || fail "ADR-0057 A15: CoS の手引き triage.md がオーナーの課題の形か unsure の線引きを指していない"
