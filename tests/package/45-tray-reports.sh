# tests/package/45-tray-reports.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  45. 報告の出し方（ADR-0064 決めたこと 13・受け入れ条件 17）: CoS の skill の前文が「行数では数えない」をやめて
#      「短くするために語を詰めない。3行を超えるなら tray へ出す」になり、原則 12 に、3行を超えるか表を含む報告を
#      `andon publish` で出して URL と判断だけを返すこと・問いは本文に残すこと・担当の報告は `andon issue report` が送ることがある。
#      CE の skill の骨格 3（入口）と 7（出口）の中に `andon publish` で出す決まりがある。worker skill の §7 に
#      「報告は `andon issue report` が tray に送る」がある。environment.md の「オーナーに見せる」の節に、tray へ出す
#      `andon publish` の文と3行を超える報告の決まりがあり、「`files/` へ出し」が無い。
#      どの検査も、その文を消した写しか、文を節の外へ動かした写しで当たることを確かめる。
# === 45. 報告の出し方（ADR-0064 受け入れ条件 17） ===
# block <file> <始まりの正規表現> <終わりの正規表現>: 始まりの行から、終わりの行の手前までを出す（始まりが無ければ空）
block() { awk -v a="$2" -v b="$3" 'on && $0 ~ b {exit} $0 ~ a {on=1} on' "$1"; }
# tray_miss <CoS SKILL.md> <CE SKILL.md> <worker SKILL.md> <environment.md>: 足りないものを1行で出す（空なら合っている）
tray_miss() {
  local m='' head p12 ce3 ce7 w7 env
  head=$(grep '^\*\*オーナーへの文は短く' "$1")
  grep -qF '短くするために語を詰めない' <<<"$head" && grep -qF '3行を超えるなら tray へ出す' <<<"$head" \
    || m="$m [CoS の前文に「語を詰めない。3行を超えるなら tray へ出す」が無い]"
  if grep -qF '行数では数えない' "$1"; then m="$m [CoS の skill に「行数では数えない」が残る]"; fi
  p12=$(grep '^12\. \*\*長い報告は tray へ出す\*\*' "$1")
  if [ -z "$p12" ]; then
    m="$m [CoS の原則 12 が無い]"
  else
    for w in '3行を超えるか表を含む' '`andon publish' 'URL' '問いは返事の本文に残す' '`andon issue report` が tray に送って'; do
      grep -qF "$w" <<<"$p12" || m="$m [CoS の原則 12 に「$w」が無い]"
    done
  fi
  ce3=$(block "$2" '^3\. \*\*入口: ' '^4\. \*\*')
  grep -F '3行を超えるか表を含む' <<<"$ce3" | grep -qF '`andon publish' || m="$m [CE の骨格 3（入口）に andon publish で出す決まりが無い]"
  ce7=$(block "$2" '^7\. \*\*出口: ' '^8\. \*\*')
  grep -qF '`andon publish`' <<<"$ce7" || m="$m [CE の骨格 7（出口）に andon publish で出す決まりが無い]"
  w7=$(block "$3" '^## 7\. ' '^## 8\. ')
  grep -qF '報告は `andon issue report` が tray に送る' <<<"$w7" || m="$m [worker skill の §7 に「報告は andon issue report が tray に送る」が無い]"
  env=$(block "$4" '^## オーナーに見せる$' '^## ')
  grep -F '`andon publish <file>`' <<<"$env" | grep -qF 'tray' || m="$m [environment.md の「オーナーに見せる」に tray へ出す andon publish の文が無い]"
  grep -F '3行を超えるか表を含む' <<<"$env" | grep -qF '`andon publish`' || m="$m [environment.md の「オーナーに見せる」に3行を超える報告の決まりが無い]"
  if grep -qF '`files/` へ出し' "$4"; then m="$m [environment.md に「files/ へ出し」が残る]"; fi
  echo "$m"
}
tc=package/skills/chief-of-staff/SKILL.md te=package/skills/chief-engineer/SKILL.md tw=package/skills/worker/SKILL.md tn=package/environment.md
miss=$(tray_miss "$tc" "$te" "$tw" "$tn")
[ -z "$miss" ] && pass "ADR-0064 受け入れ条件 17: CoS・CE・worker の skill と environment.md に報告の出し方が書いてある" \
  || fail "ADR-0064 受け入れ条件 17:$miss"
# 検査が当たる場合があること: 文を消した写し・節の外へ動かした写し・古い文を戻した写し
# t17 <説明> <期待する印> <CoS> <CE> <worker> <env>
t17() {
  local got
  got=$(tray_miss "$3" "$4" "$5" "$6")
  if [[ "$got" == *"$2"* ]]; then pass "ADR-0064 受け入れ条件 17: $1で当たる"; else fail "ADR-0064 受け入れ条件 17: $1で当たらない: [$got]"; fi
}
sed 's/短くするために語を詰めない（語を「・」で詰めると、その返事だけで通じなくなる）。3行を超えるなら tray へ出す（原則 12）。/行数では数えない（行数を守ると語を「・」で詰めた文になり、その返事だけで通じなくなる）。/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の前文を古い文に戻した写し" '[CoS の前文に「語を詰めない。3行を超えるなら tray へ出す」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
t17 "CoS の前文を古い文に戻した写し" '[CoS の skill に「行数では数えない」が残る]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
grep -v '^12\. \*\*長い報告は tray へ出す\*\*' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 を消した写し" '[CoS の原則 12 が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
sed '/^12\. \*\*長い報告は tray へ出す\*\*/s/問いは返事の本文に残す/問いも出す/' "$tc" >"$tmp/t-cos.md"
t17 "CoS の原則 12 から問いを本文に残す文を消した写し" '[CoS の原則 12 に「問いは返事の本文に残す」が無い]' "$tmp/t-cos.md" "$te" "$tw" "$tn"
# CE の入口の決まりを、骨格 1（読む）の下へ動かした写し（ファイルのどこかにあるだけでは足りない）
line=$(grep -F '3行を超えるか表を含む' "$te" | sed -n 1p)
grep -vxF -e "$line" "$te" | awk -v l="$line" '{print} /^1\. \*\*読む/ {print l}' >"$tmp/t-ce.md"
grep -qxF -e "$line" "$tmp/t-ce.md" || fail "ADR-0064 受け入れ条件 17: 動かした写しを作れない（CE の骨格 1 の行が無い）"
t17 "CE の入口の決まりを骨格 1 へ動かした写し" '[CE の骨格 3（入口）に andon publish で出す決まりが無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
sed '/^7\. \*\*出口: /s/`andon publish`/tray/' "$te" >"$tmp/t-ce.md"
t17 "CE の出口から andon publish を消した写し" '[CE の骨格 7（出口）に andon publish で出す決まりが無い]' "$tc" "$tmp/t-ce.md" "$tw" "$tn"
# worker skill の文を §7 の外（§1 の冒頭）へ動かした写し
wl=$(grep -F '報告は `andon issue report` が tray に送る' "$tw" | sed -n 1p)
grep -vxF -e "$wl" "$tw" | awk -v l="$wl" '{print} /^## 1\. / {print l}' >"$tmp/t-worker.md"
grep -qxF -e "$wl" "$tmp/t-worker.md" || fail "ADR-0064 受け入れ条件 17: 動かした写しを作れない（worker skill の §1 が無い）"
t17 "worker skill の文を §7 の外へ動かした写し" '[worker skill の §7 に「報告は andon issue report が tray に送る」が無い]' "$tc" "$te" "$tmp/t-worker.md" "$tn"
# environment.md の決まりを「気をつけること」の節へ動かした写し・古い文に戻した写し
el=$(grep -F '3行を超えるか表を含む' "$tn" | sed -n 1p)
grep -vxF -e "$el" "$tn" >"$tmp/t-env.md"
echo "$el" >>"$tmp/t-env.md"
t17 "environment.md の決まりを節の外へ動かした写し" '[environment.md の「オーナーに見せる」に3行を超える報告の決まりが無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
sed 's/^- 生成した HTML などは `andon publish <file>` で tray.*$/- 生成した HTML などは `andon publish <file>` で `files\/` へ出し、返った URL を伝える。/' "$tn" >"$tmp/t-env.md"
t17 "environment.md の publish の文を古い文に戻した写し" '[environment.md の「オーナーに見せる」に tray へ出す andon publish の文が無い]' "$tc" "$te" "$tw" "$tmp/t-env.md"
t17 "environment.md の publish の文を古い文に戻した写し" '[environment.md に「files/ へ出し」が残る]' "$tc" "$te" "$tw" "$tmp/t-env.md"
