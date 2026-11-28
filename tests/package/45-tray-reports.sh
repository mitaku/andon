# tests/package/45（報告の出し方） — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  45. 報告の出し方（ADR-0064 決めたこと 13・14・受け入れ条件 17）。報告を集めて読む場所の名前は、この節では書かない
#      （tests/ の下で名指してよいのは振り分けの道具だけ。workflow_test.go の TestWorkflowTrayOnlyInTrayJob）。文の形で照らす。
#      CoS の skill の前文（最初の節の前）が「行数では数えない」をやめて「短くするために語を詰めない。3行を超えるなら <場所> へ出す」になる。
#      原則の節の原則 12 に、3行を超えるか表を含む報告を `andon file publish` で出して URL と判断だけを返すこと・問いは本文に残すこと・
#      担当の報告は `andon issue report` が送ってあり一覧で指すこと・委譲の `publish` の値に依らないことがある。
#      CE の skill の骨格 3（入口）に同じ決まりと、ダイアログでは URL を問いの本文にも書くことがあり、表や代案を本文に出す古い文が無い。
#      骨格 7（出口）に `andon file publish` がある。worker skill の §7 に「報告は `andon issue report` が <場所> に送る」がある。
#      environment.md の「オーナーに見せる」の節に、報告を集めて読む場所へ出す `andon file publish` の文と3行を超える報告の決まりがあり、
#      「`files/` へ出し」が無い。決まりの行に打ち消し（出さず・使わず・ではなく など）があれば、語が残っていても落とす。
#      どの検査も、その文を消した写し・節の外へ動かした写し・語を残して意味を逆にした写しで当たることを確かめる。
# === 45. 報告の出し方（ADR-0064 受け入れ条件 17） ===
# block <file> <始まりの正規表現> <終わりの正規表現>: 始まりの行から、終わりの行の手前までを出す（始まりが無ければ空）
block() { awk -v a="$2" -v b="$3" 'on && $0 ~ b {exit} $0 ~ a {on=1} on' "$1"; }
# negated <文>: 決まりの行が打ち消しの形なら真
negated() { grep -qE '出さず|出さない|使わず|使わない|ではなく|書かず|書かない|やめる|しない' <<<"$1"; }
# tray_miss <CoS SKILL.md> <CE SKILL.md> <worker SKILL.md> <environment.md>: 足りないものを1行で出す（空なら合っている）
tray_miss() {
  local m='' head p12 ce3 ce3r ce7 w7 env envp envr w
  head=$(awk '/^## /{exit} {print}' "$1" | grep '^\*\*オーナーへの文は短く')
  if grep -qF '短くするために語を詰めない' <<<"$head" && grep -qE '3行を超えるなら [a-z]+ へ出す（原則 12）' <<<"$head"; then :; else
    m="$m [CoS の前文に「語を詰めない。3行を超えるなら出す」が無い]"
  fi
  if grep -qF '行数では数えない' "$1"; then m="$m [CoS の skill に「行数では数えない」が残る]"; fi
  p12=$(block "$1" '^## 原則' '^## 手引き' | grep -E '^12\. \*\*長い報告は [a-z]+ へ出す\*\*')
  if [ -z "$p12" ]; then
    m="$m [CoS の原則の節に原則 12 が無い]"
  else
    for w in '3行を超えるか表を含む' '`andon file publish' 'URL' '止まっていれば `files/` のパス' '問いは返事の本文に残す' '`andon issue report` が' '送ってあるので' 'の一覧に課題の報告がある' '委譲の `publish` の値に依らない'; do
      grep -qF "$w" <<<"$p12" || m="$m [CoS の原則 12 に「$w」が無い]"
    done
    if negated "$(sed 's/中身を書き写さず//' <<<"$p12")"; then m="$m [CoS の原則 12 が打ち消しの形]"; fi
  fi
  ce3=$(block "$2" '^3[.] [*][*]入口: ' '^4[.] [*][*]')
  ce3r=$(grep -F '3行を超えるか表を含む' <<<"$ce3" | grep -F '`andon file publish')
  if [ -z "$ce3r" ]; then
    m="$m [CE の骨格 3（入口）に andon file publish で出す決まりが無い]"
  else
    for w in '止まっていれば `files/` のパス' '問いは返事の本文かダイアログに残す' 'URL を問いの本文' '委譲の `publish` の値に依らない'; do
      grep -qF "$w" <<<"$ce3r" || m="$m [CE の骨格 3 の決まりに「$w」が無い]"
    done
    if negated "$ce3r"; then m="$m [CE の骨格 3 の決まりが打ち消しの形]"; fi
  fi
  if grep -qF '表・代案・自由記述の添削は本文に出し' "$2"; then m="$m [CE の skill に表や代案を本文に出す古い文が残る]"; fi
  ce7=$(block "$2" '^7[.] [*][*]出口: ' '^8[.] [*][*]' | grep -F '`andon file publish`')
  if [ -z "$ce7" ] || negated "$ce7"; then m="$m [CE の骨格 7（出口）に andon file publish で出す決まりが無い]"; fi
  w7=$(block "$3" '^## 7[.] ' '^## 8[.] ')
  grep -qE '^- 報告は `andon issue report` が [a-z]+ に送る' <<<"$w7" || m="$m [worker skill の §7 に「報告は andon issue report が送る」が無い]"
  env=$(block "$4" '^## オーナーに見せる$' '^## ')
  envp=$(grep -F '`andon file publish <file>`' <<<"$env" | grep -F 'セルが出した報告を集めて読む場所）へ出し')
  if [ -z "$envp" ] || negated "$envp"; then m="$m [environment.md の「オーナーに見せる」に報告の場所へ出す andon file publish の文が無い]"; fi
  envr=$(grep -F '3行を超えるか表を含む' <<<"$env" | grep -F '`andon file publish`')
  if [ -z "$envr" ] || negated "$envr"; then
    m="$m [environment.md の「オーナーに見せる」に3行を超える報告の決まりが無い]"
  else
    for w in '止まっていれば `files/` のパス' '問いは返事の本文に残す' '委譲の `publish` の値に依らない'; do
      grep -qF "$w" <<<"$envr" || m="$m [environment.md の決まりに「$w」が無い]"
    done
  fi
  if grep -qF '`files/` へ出し' "$4"; then m="$m [environment.md に「files/ へ出し」が残る]"; fi
  echo "$m"
}
tc=package/skills/chief-of-staff/SKILL.md te=package/skills/chief-engineer/SKILL.md tw=package/skills/worker/SKILL.md tn=package/environment.md
miss=$(tray_miss "$tc" "$te" "$tw" "$tn")
[ -z "$miss" ] && pass "ADR-0064 受け入れ条件 17: CoS・CE・worker の skill と environment.md に報告の出し方が書いてある" \
  || fail "ADR-0064 受け入れ条件 17:$miss"
# 検査が当たる場合があること: 文を消した写し・節の外へ動かした写し・古い文を戻した写し・語を残して意味を逆にした写し
# t17 <説明> <期待する印> <CoS> <CE> <worker> <env>
t17() {
  local got
  got=$(tray_miss "$3" "$4" "$5" "$6")
  if [[ "$got" == *"$2"* ]]; then pass "ADR-0064 受け入れ条件 17: $1で当たる"; else fail "ADR-0064 受け入れ条件 17: $1で当たらない: [$got]"; fi
}
# mv_after <file> <動かす行> <この正規表現の行の後ろへ>: 行を消して、印の行の後ろへ置いた写しを出す
mv_after() { grep -vxF -e "$2" "$1" | awk -v l="$2" -v at="$3" '{print} $0 ~ at {print l}'; }
# mv_end <file> <動かす行>: 行を消して、ファイルの末尾へ置いた写しを出す
mv_end() { grep -vxF -e "$2" "$1"; printf '%s\n' "$2"; }
# moved <写し> <行>: 動かした写しを作れたかを見る（作れないと、空の写しで当たったことになる）
moved() { grep -qxF -e "$2" "$1" || fail "ADR-0064 受け入れ条件 17: 動かした写しを作れない（$1）"; }
sed -E 's/短くするために語を詰めない（語を「・」で詰めると、その返事だけで通じなくなる）。3行を超えるなら [a-z]+ へ出す（原則 12）。/行数では数えない（行数を守ると語を「・」で詰めた文になり、その返事だけで通じなくなる）。/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の前文を古い文に戻した写し" '[CoS の前文に「語を詰めない。3行を超えるなら出す」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
t17 "CoS の前文を古い文に戻した写し" '[CoS の skill に「行数では数えない」が残る]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
hl=$(grep '^\*\*オーナーへの文は短く' "$tc" | sed -n 1p)
mv_end "$tc" "$hl" >"$tmp/t-cos.md"; moved "$tmp/t-cos.md" "$hl"
t17 "CoS の前文をファイルの末尾へ動かした写し" '[CoS の前文に「語を詰めない。3行を超えるなら出す」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
pl=$(grep '^12\. \*\*長い報告は ' "$tc" | sed -n 1p)
grep -vxF -e "$pl" "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 を消した写し" '[CoS の原則の節に原則 12 が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
mv_end "$tc" "$pl" >"$tmp/t-cos.md"; moved "$tmp/t-cos.md" "$pl"
t17 "CoS の原則 12 を原則の節の外（末尾）へ動かした写し" '[CoS の原則の節に原則 12 が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
mv_after "$tc" "$pl" '^## 手引きの扱い' >"$tmp/t-cos.md"; moved "$tmp/t-cos.md" "$pl"
t17 "CoS の原則 12 を手引きの扱いの節へ動かした写し" '[CoS の原則の節に原則 12 が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
sed '/^12\. \*\*長い報告は /s/問いは返事の本文に残す/問いも出す/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 から問いを本文に残す文を消した写し" '[CoS の原則 12 に「問いは返事の本文に残す」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
sed '/^12\. \*\*長い報告は /s/の一覧に課題の報告がある/に課題の報告がある/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 から一覧で指す文を消した写し" '[CoS の原則 12 に「の一覧に課題の報告がある」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
sed '/^12\. \*\*長い報告は /s/委譲の `publish` の値に依らない/委譲に従う/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 から委譲に依らない文を消した写し" '[CoS の原則 12 に「委譲の `publish` の値に依らない」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
sed '/^12\. \*\*長い報告は /s/`andon file publish <file> --title <タイトル>` で出し/`andon file publish` では出さず本文に全部書き/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 を語を残して打ち消した写し" '[CoS の原則 12 が打ち消しの形]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
# CE: 入口の決まりを骨格 1（読む）の下へ動かした写し（ファイルのどこかにあるだけでは足りない）
cl=$(grep -F '3行を超えるか表を含む' "$te" | sed -n 1p)
mv_after "$te" "$cl" '^1[.] [*][*]読む' >"$tmp/t-ce.md"; moved "$tmp/t-ce.md" "$cl"
t17 "CE の入口の決まりを骨格 1 へ動かした写し" '[CE の骨格 3（入口）に andon file publish で出す決まりが無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed 's/問いは返事の本文かダイアログに残す/問いもファイルに書く/' "$te" >"$tmp/t-ce.md"
t17 "CE の入口から問いを本文に残す文を消した写し" '[CE の骨格 3 の決まりに「問いは返事の本文かダイアログに残す」が無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed 's/URL を問いの本文/URL を返事/' "$te" >"$tmp/t-ce.md"
t17 "CE の入口からダイアログの問いに URL を書く文を消した写し" '[CE の骨格 3 の決まりに「URL を問いの本文」が無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed "s/\`andon file publish <file> --title <タイトル>\` で出し、返事には/\`andon file publish\` は使わず本文に出し、返事には/" "$te" >"$tmp/t-ce.md"
t17 "CE の入口を語を残して打ち消した写し" '[CE の骨格 3 の決まりが打ち消しの形]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
{ cat "$te"; echo '   表・代案・自由記述の添削は本文に出し、決断そのものだけをダイアログで1問ずつ取る。'; } >"$tmp/t-ce.md"
t17 "CE に表や代案を本文に出す古い文を戻した写し" '[CE の skill に表や代案を本文に出す古い文が残る]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed '/^7\. \*\*出口: /s/`andon file publish`/別の口/' "$te" >"$tmp/t-ce.md"
t17 "CE の出口から andon file publish を消した写し" '[CE の骨格 7（出口）に andon file publish で出す決まりが無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed '/^7\. \*\*出口: /s/`andon file publish` で出し/`andon file publish` では出さず/' "$te" >"$tmp/t-ce.md"
t17 "CE の出口を語を残して打ち消した写し" '[CE の骨格 7（出口）に andon file publish で出す決まりが無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
# worker skill の文を §7 の外（§1 の冒頭）へ動かした写し
wl=$(grep -E '^- 報告は `andon issue report` が [a-z]+ に送る' "$tw" | sed -n 1p)
mv_after "$tw" "$wl" '^## 1[.] ' >"$tmp/t-worker.md"; moved "$tmp/t-worker.md" "$wl"
t17 "worker skill の文を §7 の外へ動かした写し" '[worker skill の §7 に「報告は andon issue report が送る」が無い]' "$tc" "$te" "$tmp/t-worker.md" "$tn"
# environment.md: 決まりを「気をつけること」の節（末尾）へ動かした写し・古い文に戻した写し・語を残して打ち消した写し
el=$(grep -F '3行を超えるか表を含む' "$tn" | sed -n 1p)
mv_end "$tn" "$el" >"$tmp/t-env.md"; moved "$tmp/t-env.md" "$el"
t17 "environment.md の決まりを節の外へ動かした写し" '[environment.md の「オーナーに見せる」に3行を超える報告の決まりが無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
sed 's/^- 生成した HTML などは `andon file publish <file>` で .*$/- 生成した HTML などは `andon file publish <file>` で `files\/` へ出し、返った URL を伝える。/' "$tn" >"$tmp/t-env.md"
t17 "environment.md の publish の文を古い文に戻した写し" '[environment.md の「オーナーに見せる」に報告の場所へ出す andon file publish の文が無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
t17 "environment.md の publish の文を古い文に戻した写し" '[environment.md に「files/ へ出し」が残る]' "$tc" "$te" "$tw" "$tmp/t-env.md"
sed '/^- 生成した HTML などは /s/読む場所）へ出し、/読む場所）へ出し、ではなく files\/ に写し、/' "$tn" >"$tmp/t-env.md"
t17 "environment.md の publish の文を「ではなく」と打ち消した写し" '[environment.md の「オーナーに見せる」に報告の場所へ出す andon file publish の文が無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
sed '/3行を超えるか表を含む/s/`andon file publish` で出し/`andon file publish` では出さず返事に全部書き/' "$tn" >"$tmp/t-env.md"
t17 "environment.md の決まりを語を残して打ち消した写し" '[environment.md の「オーナーに見せる」に3行を超える報告の決まりが無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
sed '/3行を超えるか表を含む/s/問いは返事の本文に残す。//' "$tn" >"$tmp/t-env.md"
t17 "environment.md の決まりから問いを本文に残す文を消した写し" '[environment.md の決まりに「問いは返事の本文に残す」が無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
# 止まっていて files/ のパスが返るときの書き方（返事に URL だけを求めると、その場合に届ける道が無い）を消した写し
sed 's/止まっていれば `files\/` のパス/止まっていれば何も/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 から files/ のパスの場合を消した写し" '[CoS の原則 12 に「止まっていれば `files/` のパス」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
sed 's/止まっていれば `files\/` のパス/止まっていれば何も/' "$te" >"$tmp/t-ce.md"
t17 "CE の入口から files/ のパスの場合を消した写し" '[CE の骨格 3 の決まりに「止まっていれば `files/` のパス」が無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed '/3行を超えるか表を含む/s/止まっていれば `files\/` のパス/止まっていれば何も/' "$tn" >"$tmp/t-env.md"
t17 "environment.md の決まりから files/ のパスの場合を消した写し" '[environment.md の決まりに「止まっていれば `files/` のパス」が無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
