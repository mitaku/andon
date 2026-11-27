# tests/package/46（積む起動と載せ替え） — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置き、前の節が置いた pb も使う）。
#  46. 積む起動と載せ替えの手引きと skill（ADR-0067 決めたこと 17）。worker skill の §3 が積んだ課題の PR を draft で前の課題の branch へ出し、
#      差分を分かれ目から読み、積んでいる間 rebase しないと書く。§7 が載せ替えを andon issue restack で済ませ、rebase を打ち直さないと書き、
#      「頼まれていない rebase はしない」の行に前の課題の branch へも rebase しないと足す。finish.md の影響の表に積んだ課題の行が 2 つ、
#      片付けの表に既定の branch でない base の行、やめる課題に積んだ課題を言う文がある。watch.md に gate の behind の 2 行がある。
#      どの検査も、その文を消した写しで当たることを確かめる。
# === 46. 積む起動と載せ替え（ADR-0067 決めたこと 17） ===
# stack_miss <worker SKILL.md> <finish.md> <watch.md>: 足りないものを1行で出す（空なら合っている）
stack_miss() {
  local m='' w3 w7
  w3=$(sed -n '/^## 3\. 検査・commit・PR$/,/^## 4\./p' "$1")
  w7=$(sed -n '/^## 7\. 報告して返す$/,$p' "$1")
  grep -qF '`gh pr create --draft --base <前の課題の branch>`' <<<"$w3" || m="$m [worker §3 の draft の出し方]"
  grep -qF '`git diff <stacked_base>..HEAD`' <<<"$w3" || m="$m [worker §3 の分かれ目からの差分]"
  grep -qF '前の課題の branch へも main へも rebase しない' <<<"$w3" || m="$m [worker §3 の rebase しない]"
  grep -qF '**載せ替え**（積んだ課題。' <<<"$w7" || m="$m [worker §7 の載せ替え]"
  grep -qF 'rebase は打ち直さない' <<<"$w7" || m="$m [worker §7 の打ち直さない]"
  grep -q '^- どちらも、頼まれていない rebase や向きの変更はしない.*積んでいる間は、前の課題の branch へも rebase しない' <<<"$w7" || m="$m [worker §7 の頼まれていない rebase]"
  [ "$(grep -c '^| `stacked_on` ' "$2")" = 2 ] || m="$m [finish.md の影響の表の積んだ課題の 2 行]"
  grep -qF '片付けていない: PR が既定の branch でない base' "$2" || m="$m [finish.md の既定でない base の行]"
  grep -qF 'その課題の上に積んだ課題' "$2" || m="$m [finish.md のやめる課題]"
  grep -qF '`<id> gate: behind（前の課題 <id> の PR がまだ main に入っていない…）`' "$3" || m="$m [watch.md の前の PR 待ち]"
  grep -qF '`<id> gate: behind（前の課題 <id> がマージされずに終わった…）`' "$3" || m="$m [watch.md のマージされずに終わった]"
  printf '%s' "$m"
}
w=package/skills/worker/SKILL.md
miss=$(stack_miss "$w" "$pb/finish.md" "$pb/watch.md")
[ -z "$miss" ] && pass "ADR-0067 決めたこと 17: worker skill と finish.md・watch.md が積んだ PR の出し方と載せ替えを書く" || fail "ADR-0067 決めたこと 17:$miss"
# 当たる場合があること: 載せ替えの段を消した写し・影響の表の 1 行を消した写し・behind の行を消した写しでは落ちる
sed '/^\*\*載せ替え\*\*（積んだ課題。/d' "$w" >"$tmp/w46.md"
grep -v '^| `stacked_on` があり' "$pb/finish.md" >"$tmp/f46.md"
grep -v 'がマージされずに終わった…）`' "$pb/watch.md" >"$tmp/x46.md"
got=$(stack_miss "$tmp/w46.md" "$tmp/f46.md" "$tmp/x46.md")
if [[ "$got" == *'[worker §7 の載せ替え]'* && "$got" == *'[finish.md の影響の表の積んだ課題の 2 行]'* && "$got" == *'[watch.md のマージされずに終わった]'* ]]; then
  pass "ADR-0067 決めたこと 17: 文を消した写しで検査が落ちる（検査が見ている）"
else fail "ADR-0067 決めたこと 17: 文を消した写しで検査が落ちない: $got"; fi
