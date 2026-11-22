# tests/package/31-digest.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置き、pb は前の節が置く）。
#  31. ダイジェストの片付けが止まらず、出来事の落ちを黙らない（ADR-0044 決めたこと 4・5）: 手引き digest.md に、片付け済みの課題へ
#      もう一度届いた report・報告のあと doing で止まった担当・前に送れた 1 枚が無いときの期間・片付けたあとの枠の数え直しがある。
#      CoS は送り直しを頼まない（回数を残す場所が無く、2 つ目の report と競る。送り直しは担当が 1 回する）。
# === 31. ダイジェストの片付け（ADR-0044） ===
# digest_gaps <手引き> <skill>: 欠けているものを出す（空なら揃っている）
digest_gaps() {
  local m=''
  grep -qF '`done` なら何もしない' "$1" || m="$m [2 つ目の report]"
  grep -qF '`kanban review <id>` を打つよう' "$1" || m="$m [doing で止まった担当]"
  grep -qF '前に送れた1枚が無ければ' "$1" || m="$m [前の 1 枚が無いときの期間]"
  grep -qF '`start.md` の 1 で枠を数え直し' "$1" || m="$m [枠の数え直し]"
  if grep -qE 'hc send[^。]*andon +digest +send' "$1"; then m="$m [CoS が送り直しを頼む]"; fi
  if grep -qF '送り直しが届いたら' "$2"; then m="$m [担当が送り直しを待つ]"; fi
  echo "$m"
}
miss=$(digest_gaps "$pb/digest.md" package/skills/digest/SKILL.md)
[ -z "$miss" ] && pass "ADR-0044: ダイジェストの手引きが、止まらずに片付き、落ちる期間を黙らない" || fail "ADR-0044: ダイジェストの手引き:$miss"
# 否定の検査が当たる場合があること: 送り直しを頼む文を戻した写しでは落ちる（空白の数が違っても）
miss=
{ cat "$pb/digest.md"; echo '担当へ `hc send`「`andon  digest send <id>` を打ち直して」'; } >"$tmp/digest-playbook.md"
digest_gaps "$tmp/digest-playbook.md" package/skills/digest/SKILL.md | grep -qF '[CoS が送り直しを頼む]' || miss="$miss [手引き]"
{ cat package/skills/digest/SKILL.md; echo '- 送り直しが届いたら send を打つ'; } >"$tmp/digest-skill.md"
digest_gaps "$pb/digest.md" "$tmp/digest-skill.md" | grep -qF '[担当が送り直しを待つ]' || miss="$miss [skill]"
[ -z "$miss" ] && pass "ADR-0044: 送り直しを戻した写しでは落ちる（検査が見ている）" || fail "ADR-0044: 写しで落ちない:$miss"
