# tests/package/46（積む起動と載せ替え） — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置き、前の節が置いた pb も使う）。
#  46. 積む起動と載せ替えの手引きと skill（ADR-0067 決めたこと 17）。worker skill の §3 が積んだ課題を kanban のキーで見分け、PR の前と ready の前に
#      restack を打ち、PR を draft で前の課題の branch へ出して draft も向き先も変えず、差分を分かれ目から読み、積んでいる間 rebase しないと書く。
#      §7 が載せ替えを andon issue restack で済ませ、rebase を打ち直さないと書き、「頼まれていない rebase はしない」の行に前の課題の branch へも
#      rebase しないと足す。finish.md の影響の表は、積んだ課題の 2 行が `conflict: true` の行より上にあり、2 行目の判断が rebase を頼まない。
#      片付けの表に既定の branch でない base の行、やめる課題に積んだ課題を言う文と doing の担当を待たせる文がある。watch.md の gate の behind の
#      2 行は、前の PR 待ちならオーナーに何も言わず、マージされずに終わったならやめるか続けるかの 2 つを聞く。
#      どの検査も、文を消した写しと、意味を逆にした写し（行の順を入れ替える・判断を rebase を頼むにする・draft を外す・選択肢を替える）で落ちる。
# === 46. 積む起動と載せ替え（ADR-0067 決めたこと 17） ===
# stack_miss <worker SKILL.md> <finish.md> <watch.md>: 足りないものを1行で出す（空なら合っている）
stack_miss() {
  local m='' w3 w3s w7 r1 r2 rc n1 n2 nc wb wg
  w3=$(sed -n '/^## 3\. 検査・commit・PR$/,/^## 4\./p' "$1")
  w3s=$(sed -n '/^   \*\*積んだ課題\*\*/,/^4\. /p' <<<"$w3")
  w7=$(sed -n '/^## 7\. 報告して返す$/,$p' "$1")
  grep -qF '`kanban show <id>` に `stacked_on`・`stacked_base` がある' <<<"$w3s" || m="$m [worker §3 の見分け方]"
  grep -qF '行の有無では決めない' <<<"$w3s" || m="$m [worker §3 の resume のあとの見分け]"
  grep -qF 'PR を出す前に `andon issue restack <id>` を打つ' <<<"$w3s" || m="$m [worker §3 の PR の前の restack]"
  grep -qF '`gh pr create --draft --base <前の課題の branch>`' <<<"$w3s" || m="$m [worker §3 の draft の出し方]"
  grep -qF 'draft を外さず、向き先も変えない' <<<"$w3s" || m="$m [worker §3 の draft を外さない]"
  if grep -qE 'gh pr ready|gh pr edit --base' <<<"$w3s"; then m="$m [worker §3 が積んでいる間に draft か向き先を変える]"; fi
  grep -qF '`git diff <stacked_base>..HEAD`' <<<"$w3s" || m="$m [worker §3 の分かれ目からの差分]"
  grep -qF '前の課題の branch へも main へも rebase しない' <<<"$w3s" || m="$m [worker §3 の rebase しない]"
  grep -qF '`andon issue ready` の前にも `andon issue restack <id>` を打つ' <<<"$w3s" || m="$m [worker §3 の ready の前の restack]"
  grep -qF '答えが届くまで `andon issue restack`・PR・`andon issue ready` を打たない' <<<"$w3s" || m="$m [worker §3 のやめるときの待ち]"
  grep -qF '**載せ替え**（積んだ課題。' <<<"$w7" || m="$m [worker §7 の載せ替え]"
  grep -qF 'rebase は打ち直さない' <<<"$w7" || m="$m [worker §7 の打ち直さない]"
  grep -q '^- どちらも、頼まれていない rebase や向きの変更はしない.*積んでいる間は、前の課題の branch へも rebase しない' <<<"$w7" || m="$m [worker §7 の頼まれていない rebase]"
  # 影響の表は上から見て最初に当たった行で決めるので、積んだ課題の 2 行は conflict の行より上に要る
  n1=$(grep -n '^| `stacked_on` が、マージした課題そのもの' "$2" | cut -d: -f1)
  n2=$(grep -n '^| `stacked_on` があり、マージしたのがほかの課題' "$2" | cut -d: -f1)
  nc=$(grep -n '^| `conflict: true` |' "$2" | cut -d: -f1)
  if [[ -z "$n1" || -z "$n2" || -z "$nc" ]] || (( n1 > nc || n2 > nc )); then m="$m [finish.md の積んだ課題の 2 行が conflict の行より上に無い]"; fi
  r1=$(grep '^| `stacked_on` が、マージした課題そのもの' "$2"); r2=$(grep '^| `stacked_on` があり、マージしたのがほかの課題' "$2")
  [ "$(cut -d'|' -f3 <<<"$r1")" = ' 影響なし（gate が載せ替えを頼む） ' ] || m="$m [finish.md の積んだ相手の行の判断]"
  [ "$(cut -d'|' -f3 <<<"$r2")" = ' rebase を頼まない ' ] && grep -qF 'があっても rebase を頼まない' <<<"$r2" && grep -qF '（rebase は頼まない）' <<<"$r2" || m="$m [finish.md のほかの課題の行の判断]"
  grep -qF '片付けていない: PR が既定の branch でない base' "$2" && grep -qF '`kanban show <前の課題>` を見て、`done` になっていたら' "$2" || m="$m [finish.md の既定でない base の行]"
  grep -qF '積んだ課題の PR が open なら、dispatcher が gate を回し直し' "$2" || m="$m [finish.md の積んだ相手の行の open の条件]"
  grep -qF '「既定の branch でない base」の行で手で片付けた課題なら、聞かずに' "$3" || m="$m [watch.md の手で片付けた前の課題の但し書き]"
  grep -qF 'その課題の上に積んだ課題' "$2" || m="$m [finish.md のやめる課題]"
  grep -qF '答えを送るまで andon issue restack・PR・ready を打たずに待って' "$2" || m="$m [finish.md の doing の担当を待たせる]"
  wb=$(grep -F '`<id> gate: behind（前の課題 <id> の PR がまだ main に入っていない…）`' "$3")
  wg=$(grep -F '`<id> gate: behind（前の課題 <id> がマージされずに終わった…）`' "$3")
  [ -n "$wb" ] && grep -qF 'オーナーには何も言わない' <<<"$wb" && grep -qF '`start.md` の 1 で枠を数え直す' <<<"$wb" || m="$m [watch.md の前の PR 待ち]"
  [ -n "$wg" ] && grep -qF '選択肢は「この課題もやめる」と「前の課題の変更なしで続ける」の2つ' <<<"$wg" && grep -qF 'andon issue restack' <<<"$wg" || m="$m [watch.md のマージされずに終わった]"
  printf '%s' "$m"
}
w=package/skills/worker/SKILL.md
miss=$(stack_miss "$w" "$pb/finish.md" "$pb/watch.md")
[ -z "$miss" ] && pass "ADR-0067 決めたこと 17: worker skill と finish.md・watch.md が積んだ PR の出し方と載せ替えを書く" || fail "ADR-0067 決めたこと 17:$miss"
# 当たる場合があること（文を消した写し）: 載せ替えの段・影響の表の 1 行・behind の行を消すと落ちる
sed '/^\*\*載せ替え\*\*（積んだ課題。/d' "$w" >"$tmp/w46.md"
grep -v '^| `stacked_on` があり' "$pb/finish.md" >"$tmp/f46.md"
grep -v 'がマージされずに終わった…）`' "$pb/watch.md" >"$tmp/x46.md"
got=$(stack_miss "$tmp/w46.md" "$tmp/f46.md" "$tmp/x46.md")
if [[ "$got" == *'[worker §7 の載せ替え]'* && "$got" == *'[finish.md の積んだ課題の 2 行が conflict の行より上に無い]'* && "$got" == *'[watch.md のマージされずに終わった]'* ]]; then
  pass "ADR-0067 決めたこと 17: 文を消した写しで検査が落ちる（検査が見ている）"
else fail "ADR-0067 決めたこと 17: 文を消した写しで検査が落ちない: $got"; fi
# 当たる場合があること（意味を逆にした写し）
#   (a) 積んだ課題の 2 行を conflict の行より下へ動かす (b) 2 行目の判断を rebase を頼むにする
#   (c) worker §3 で PR を出したら draft を外して向き先を main にする (e) watch.md の選択肢を替え、前の PR 待ちでオーナーに聞く
awk '/^[|] `stacked_on` /{h=h $0 "\n"; next} {print} /^[|] `conflict: true` [|]/{printf "%s", h; h=""}' "$pb/finish.md" >"$tmp/fa46.md"
sed 's/^\(| `stacked_on` があり、マージしたのがほかの課題 | \)rebase を頼まない |/\1**rebase を頼む** |/; s/があっても rebase を頼まない/があれば rebase を頼む/' "$pb/finish.md" >"$tmp/fb46.md"
sed 's/draft を外さず、向き先も変えない/PR を出したらすぐ gh pr ready で draft を外し、gh pr edit --base main で向き先を変える/' "$w" >"$tmp/wc46.md"
sed 's/選択肢は「この課題もやめる」と「前の課題の変更なしで続ける」の2つ/選択肢は「マージする」/; s/オーナーには何も言わない/オーナーに1問聞く/' "$pb/watch.md" >"$tmp/xe46.md"
ga=$(stack_miss "$w" "$tmp/fa46.md" "$pb/watch.md"); gb=$(stack_miss "$w" "$tmp/fb46.md" "$pb/watch.md")
gc=$(stack_miss "$tmp/wc46.md" "$pb/finish.md" "$pb/watch.md"); ge=$(stack_miss "$w" "$pb/finish.md" "$tmp/xe46.md")
if [[ "$ga" == *'[finish.md の積んだ課題の 2 行が conflict の行より上に無い]'* && "$gb" == *'[finish.md のほかの課題の行の判断]'* \
   && "$gc" == *'[worker §3 の draft を外さない]'* && "$gc" == *'[worker §3 が積んでいる間に draft か向き先を変える]'* \
   && "$ge" == *'[watch.md の前の PR 待ち]'* && "$ge" == *'[watch.md のマージされずに終わった]'* ]]; then
  pass "ADR-0067 決めたこと 17: 意味を逆にした写し（行の順・判断・draft・選択肢）で検査が落ちる（検査が見ている）"
else fail "ADR-0067 決めたこと 17: 意味を逆にした写しで検査が落ちない: a=$ga b=$gb c=$gc e=$ge"; fi
