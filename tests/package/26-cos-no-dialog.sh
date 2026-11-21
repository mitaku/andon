# tests/package/26-cos-no-dialog.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm は入口が置く）。
#  26. Chief of Staff はダイアログで問わない（ADR-0042 受け入れ条件 7・8）: 起動行がダイアログの道具を外し、Chief Engineer の起動行は外さない。
#      ask-one-by-one・chief-of-staff の skill と手引きに「AskUserQuestion を既定」と読める文が無く、Chief of Staff は本文、Chief Engineer はダイアログと書き分ける。
# === 26. Chief of Staff はダイアログで問わない（ADR-0042） ===
nodialog=' --disallowedTools AskUserQuestion '
has_nodialog() { case " $1 " in *"$nodialog"*) return 0 ;; *) return 1 ;; esac; }
has_nodialog "$(fm package/templates/chief-of-staff.md command)" \
  && pass "ADR-0042 A7: Chief of Staff の起動行がダイアログの道具を外す" || fail "ADR-0042 A7: Chief of Staff の起動行に$nodialog が無い"
miss=
for field in command resume; do
  v=$(fm package/templates/chief-engineer.md "$field")
  [ -n "$v" ] || miss="$miss [$field が読めない]"
  # 書き方の変種（--disallowed-tools・=・引用符・ほかの道具と並べる）も拾うよう、道具の名前が出るだけで落とす
  case $v in *AskUserQuestion*) miss="$miss [$field が AskUserQuestion に触れている]" ;; esac
done
[ -z "$miss" ] && pass "ADR-0042 A7: Chief Engineer の起動行はダイアログの道具を外さない（決めたこと 7）" || fail "ADR-0042 A7: Chief Engineer の起動行:$miss"
# 検査が当たる場合があること: 指定を入れた行には当たり、入れない行には当たらない
has_nodialog 'claude --model opus --disallowedTools AskUserQuestion --plugin-dir /opt/andon' && ! has_nodialog 'claude --model opus --plugin-dir /opt/andon' \
  && pass "ADR-0042 A7: 起動行の検査が指定の有無を見分ける（検査が見ている）" || fail "ADR-0042 A7: 起動行の検査が何も見ていない"

# 「AskUserQuestion（ダイアログ）を既定」と読める文
dialog_default='(AskUserQuestion|ダイアログ)[^。]*既定|既定[^。]*(AskUserQuestion|ダイアログ)'
left=$(grep -nE "$dialog_default" package/skills/ask-one-by-one/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/chief-of-staff/playbooks/*.md || true)
[ -z "$left" ] && pass "ADR-0042 A8: ask-one-by-one と chief-of-staff の skill・手引きに AskUserQuestion を既定と読める文が無い" || fail "ADR-0042 A8: AskUserQuestion を既定と読める文が残る: $left"
printf '%s\n' '1. 選択肢で答えられる問いは **AskUserQuestion を既定**にする。' | grep -qE "$dialog_default" \
  && printf '%s\n' '既定はダイアログで聞く。' | grep -qE "$dialog_default" \
  && pass "ADR-0042 A8: 古い文は否定の検査に当たる（検査が見ている）" || fail "ADR-0042 A8: 否定の検査が何も見ていない"
aobo2=$(sed -n '/^## 2\. /,/^## 3\. /p' package/skills/ask-one-by-one/SKILL.md)
grep -qF '**Chief of Staff は、どの問いも返事の本文に書き、答えを待たずにターンを終える**' <<<"$aobo2" \
  && grep -qF '**Chief Engineer は、選択肢で答えられる問いをダイアログで出す**' <<<"$aobo2" \
  && grep -qF '**答えは、待っている問いのどれからでも受ける**' <<<"$aobo2" \
  && pass "ADR-0042 A8: ask-one-by-one §2 が Chief of Staff は本文、Chief Engineer はダイアログと書き分け、答えをどの問いからでも受ける" || fail "ADR-0042 A8: ask-one-by-one §2 の書き分けが無い"
grep -qF '聞く前に kanban のオーナーの課題にして置く' package/skills/ask-one-by-one/SKILL.md \
  && grep -q '^7\. \*\*オーナーの時間を守る\*\*。.*問いはどれも返事の本文の最後に書き、答えを待たずにターンを終える.*聞く問いは、聞く前に kanban に置く。' package/skills/chief-of-staff/SKILL.md \
  && pass "ADR-0042 A8: 問いは聞く前に kanban に置き、Chief of Staff の原則 7 が本文に書いてターンを終えると言う" || fail "ADR-0042 A8: ask-one-by-one §3 か chief-of-staff の原則 7 が古い"
