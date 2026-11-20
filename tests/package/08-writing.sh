# tests/package/08-writing.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   8. 役の略称とオーナーへの書き方（ADR-0013 受け入れ条件 1・2・7・8 の用語集の分・10）: 用語集の CE・作った語、package/writing.md の書き方の11行・但し書き・語の決まり3つ・用語集のパス、
#      用語集は書き方の節と語の決まりを持たず writing.md を指す、CE skill の問いの出し方、ask-one-by-one の読み手。
# === 8. 役の略称とオーナーへの書き方（ADR-0013 受け入れ条件 1・2） ===
g=package/skills/glossary/SKILL.md
grep -q '^\*\*Chief Engineer\*\*（chief-engineer。略して CE）:' "$g" && grep -q '^\*\*Chief of Staff\*\*（chief-of-staff。略して CoS）:' "$g" \
  && pass "用語集の Chief Engineer の項に「略して CE」がある" || fail "用語集に CE の略称が無い"
# 用語集（受け入れ条件 8 の用語集の分）: 書き方の節と語の決まり3つを持たず、writing.md を指す。モデルからは呼べないまま
grep -q '^- \*\*役と部品の語はここにある語だけを使う\*\*' "$g" && ! grep -q '^## オーナーへの書き方' "$g" && grep -q '^disable-model-invocation: true$' "$g" \
  && ! grep -qE 'ふだんカタカナで言う語|訳語を新しく作らない|作った訳語|文の中では略称で' "$g" \
  && grep -q '^- オーナーへの書き方と語の決まり.*は `/opt/andon/writing.md` に従う' "$g" \
  && pass "用語集は書き方の節と語の決まりを持たず writing.md を指す（disable-model-invocation は残る）" || fail "用語集に書き方の節か語の決まりが残る、か writing.md を指していない"
iss=$(sed -n '/^\*\*課題\*\*（issue）:/,/^_避ける_/p' "$g")
! grep -q '「番号（題）」' <<<"$iss" && grep -q '`/opt/andon/writing.md` の番号の決まりに従う' <<<"$iss" \
  && pass "用語集の課題の項は番号（題）の決まりを自分で言わず writing.md を指す" || fail "用語集の課題の項が writing.md を指していない"
# skill 4本と environment.md（受け入れ条件 8 の残り）: 「glossary skill に従う」が無く、writing.md か用語集のパスを指す。番号（題）の決まりは writing.md を指す。報告の書式に「（推奨案:」が無い
left=""
for f in package/environment.md package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/chief-engineer/SKILL.md package/skills/triage/SKILL.md; do
  grep -qE 'glossary skill に従う|/andon:glossary' "$f" && left="$left [$f:旧い言い方]"
  [ "$f" = "$cos" ] && grep -qE 'glossary skill に従う|/andon:glossary' "$pb"/*.md && left="$left [$pb:旧い言い方]"
  grep -q '/opt/andon/writing.md' "$f" && grep -q '/opt/andon/skills/glossary/SKILL.md' "$f" || left="$left [$f:パスが無い]"
done
[ -z "$left" ] && pass "worker・CoS・CE・triage の skill と environment.md は writing.md と用語集のパスを指す" || fail "「glossary skill に従う」が残るか、パスが無い:$left"
left=""
for f in package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/ask-one-by-one/SKILL.md; do
  grep -q '「番号（題）」の形' "$f" && left="$left [$f:番号（題）の決まり]"
  [ "$f" = "$cos" ] && grep -q '「番号（題）」の形' "$pb"/*.md && left="$left [$pb:番号（題）の決まり]"
  grep -q '/opt/andon/writing.md` の番号の決まりに従う' "$f" || left="$left [$f:writing.md を指していない]"
done
[ -z "$left" ] && pass "CoS・worker・ask-one-by-one の skill は番号（題）の決まりを自分で言わず writing.md を指す" || fail "skill が番号の決まりを自分で言うか、writing.md を指していない:$left"
! grep -q '（推奨案:' package/skills/worker/SKILL.md && pass "worker skill の報告の書式に「（推奨案:」が無い" || fail "worker skill の報告の書式に「（推奨案:」が残る"
# writing.md（受け入れ条件 7）: 読み手をオーナーに返す文に絞り、箇条書きは11行、但し書き・語の決まり3つ・用語集のパス
wr=package/writing.md
how=$(sed -n '1,/^## /p' "$wr")
n=$(grep -c '^- ' <<<"$how")
[ "$n" = 11 ] && grep -q '^CoS・CE・担当がオーナーに返す文' <<<"$how" \
  && grep -q '^- 結論を先に書く（BLUF）' <<<"$how" && grep -q '状況・経緯・見立て・推奨の順に書く（SBAR）' <<<"$how" \
  && grep -q '「・」で語を詰めない' <<<"$how" && grep -q '1文に1つのこと' <<<"$how" \
  && grep -q "英語の用語を直訳した語は使わない" <<<"$how" && grep -q '灯り → オーナーの判断待ち、レベル C → オーナーがマージ' <<<"$how" \
  && grep -q '^- 括弧に説明を入れない。説明は文にする。括弧に入れるのは識別子と、番号に添える題だけ。' <<<"$how" \
  && grep -q '^- 番号だけで指さない。.*`S-0004（返事の下書きボタン）`.*同じ返事の2度目からは番号だけでよい。' <<<"$how" \
  && grep -q '^- 読む人は道具の呼び出しも、読んだファイルも、途中の考えも見ていない。' <<<"$how" \
  && grep -q '^- つながる考えは段落の文で書く。箇条書きは別々の項目を並べるときに使う。' <<<"$how" \
  && grep -q '^- 作業の途中で自分が付けた呼び名や略称を使わない。' <<<"$how" \
  && grep -q '^コードと、オーナーが別の形を求めたときは、この決まりから除く' <<<"$how" \
  && pass "writing.md はオーナーに返す文に絞り、書き方の11行と但し書きがある" || fail "writing.md の書き方の節（行の数 $n）"
! grep -qE '括弧には識別子だけ|番号で参照しない' "$wr" && pass "writing.md に古い字面（括弧には識別子だけ・番号で参照しない）が無い" || fail "writing.md に古い字面が残る"
grep -q 'Chief of Staff を CoS、Chief Engineer を CE と書く' "$wr" \
  && grep -q 'ふだんカタカナで言う語（コスト・プロファイル・テンプレート・コンテキスト・オーナー・ログ・パス）はカタカナのまま' "$wr" \
  && grep -q '訳語を新しく作らない。迷ったら同僚に口で言うときの言い方にする' "$wr" && grep -q '`/opt/andon/skills/glossary/SKILL.md`' "$wr" \
  && pass "writing.md に語の決まり3つと用語集のパスがある" || fail "writing.md に語の決まりか用語集のパスが無い"
# 作った語は「避ける」の行と仮設の語の表にしか出ない。残す語（検査・継ぎ目・条件表・控え）は消さない
left=$(sed '/^## 仮設の語/,$d' "$g" | grep -nE '席|端末の土台|持ち主|提供元' | grep -v '^[0-9]*:_避ける_' || true)
[ -z "$left" ] && pass "用語集に作った語（避ける語）が正式な語として出ない" || fail "用語集に作った語が残る: $left"
grep -q '^\*\*検査\*\*（checks）:' "$g" && grep -q '条件表' "$g" \
  && pass "用語集の残す語（検査・条件表）は残る" || fail "用語集から残す語が消えた"
! grep -q '一度に出すのは互いに依らない問いの全部' "$ce" && grep -q 'ask-one-by-one skill §2 に従う（1回の返事で1問）' "$ce" \
  && pass "CE skill の問いの出し方は ask-one-by-one §2 を指す（束で出す行が無い）" || fail "CE skill の問いの出し方が ask-one-by-one §2 を指していない"
fm package/skills/ask-one-by-one/SKILL.md description | grep -q 'Chief Engineer（CE）' \
  && pass "ask-one-by-one の description の読み手に CE がある" || fail "ask-one-by-one の description に CE が無い"
