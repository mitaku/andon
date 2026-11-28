# tests/package/08-writing.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   8. 役の略称とオーナーへの書き方（ADR-0013 受け入れ条件 1・2・7・8 の用語集の分・10）: 用語集の CE・作った語、package/writing.md の書き方の11行・但し書き・語の決まり3つ・用語集のパス、
#      用語集は書き方の節と語の決まりを持たず writing.md を指す、CE skill の問いの出し方、ask-one-by-one の読み手。
#      ADR-0052 受け入れ条件 1: writing.md のタイトルとテーマの決まりの 1 行と例・括弧の決まり、用語集の 1人1テーマ、文書の形の 1 行目。
#      オーナーへはふだんの言葉で言う語: drain.go の写しの行が用語集の表にあり、writing.md が表を指す。
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
! grep -q '「番号（タイトル）」' <<<"$iss" && grep -q '`/opt/andon/writing.md` の番号の決まりに従う' <<<"$iss" \
  && pass "用語集の課題の項は番号（タイトル）の決まりを自分で言わず writing.md を指す" || fail "用語集の課題の項が writing.md を指していない"
# skill 4本と environment.md（受け入れ条件 8 の残り）: 「glossary skill に従う」が無く、writing.md か用語集のパスを指す。番号（タイトル）の決まりは writing.md を指す。報告の書式に「（推奨案:」が無い
left=""
for f in package/environment.md package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/chief-engineer/SKILL.md package/skills/triage/SKILL.md; do
  grep -qE 'glossary skill に従う|/andon:glossary' "$f" && left="$left [$f:旧い言い方]"
  [ "$f" = "$cos" ] && grep -qE 'glossary skill に従う|/andon:glossary' "$pb"/*.md && left="$left [$pb:旧い言い方]"
  grep -q '/opt/andon/writing.md' "$f" && grep -q '/opt/andon/skills/glossary/SKILL.md' "$f" || left="$left [$f:パスが無い]"
done
[ -z "$left" ] && pass "worker・CoS・CE・triage の skill と environment.md は writing.md と用語集のパスを指す" || fail "「glossary skill に従う」が残るか、パスが無い:$left"
left=""
for f in package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/ask-one-by-one/SKILL.md; do
  grep -q '「番号（タイトル）」の形' "$f" && left="$left [$f:番号（タイトル）の決まり]"
  [ "$f" = "$cos" ] && grep -q '「番号（タイトル）」の形' "$pb"/*.md && left="$left [$pb:番号（タイトル）の決まり]"
  grep -q '/opt/andon/writing.md` の番号の決まりに従う' "$f" || left="$left [$f:writing.md を指していない]"
done
[ -z "$left" ] && pass "CoS・worker・ask-one-by-one の skill は番号（タイトル）の決まりを自分で言わず writing.md を指す" || fail "skill が番号の決まりを自分で言うか、writing.md を指していない:$left"
! grep -q '（推奨案:' package/skills/worker/SKILL.md && pass "worker skill の報告の書式に「（推奨案:」が無い" || fail "worker skill の報告の書式に「（推奨案:」が残る"
# writing.md（受け入れ条件 7）: 読み手をオーナーに返す文に絞り、箇条書きは11行、但し書き・語の決まり3つ・用語集のパス
wr=package/writing.md
how=$(sed -n '1,/^## /p' "$wr")
n=$(grep -c '^- ' <<<"$how")
[ "$n" = 11 ] && grep -q '^CoS・CE・担当がオーナーに返す文' <<<"$how" \
  && grep -q '^- 結論を先に書く（BLUF）' <<<"$how" && grep -q '状況・経緯・見立て・推奨の順に書く（SBAR）' <<<"$how" \
  && grep -q '「・」で語を詰めない' <<<"$how" && grep -q '1文に1つのこと' <<<"$how" \
  && grep -q "英語の用語を直訳した語は使わない" <<<"$how" && grep -q '灯り → オーナーの判断待ち、レベル C → オーナーがマージ' <<<"$how" \
  && grep -q '^- 括弧に説明を入れない。説明は文にする。括弧に入れるのは識別子と、番号に添えるタイトルだけ。' <<<"$how" \
  && grep -q '^- 番号だけで指さない。.*`S-0004（返事の下書きボタン）`.*同じ返事の2度目からは番号だけでよい。' <<<"$how" \
  && grep -q '^- 読む人は道具の呼び出しも、読んだファイルも、途中の考えも見ていない。' <<<"$how" \
  && grep -q '^- つながる考えは段落の文で書く。箇条書きは別々の項目を並べるときに使う。' <<<"$how" \
  && grep -q '^- 作業の途中で自分が付けた呼び名や略称を使わない。' <<<"$how" \
  && grep -q '^コードと、オーナーが別の形を求めたときは、この決まりから除く' <<<"$how" \
  && pass "writing.md はオーナーに返す文に絞り、書き方の11行と但し書きがある" || fail "writing.md の書き方の節（行の数 $n）"
! grep -qE '括弧には識別子だけ|番号で参照しない' "$wr" && pass "writing.md に古い字面（括弧には識別子だけ・番号で参照しない）が無い" || fail "writing.md に古い字面が残る"
grep -q 'Chief of Staff を CoS、Chief Engineer を CE と書く' "$wr" \
  && grep -q 'ふだんカタカナで言う語（コスト・プロファイル・テンプレート・コンテキスト・オーナー・ログ・パス・バージョン・タイトル・テーマ）はカタカナのまま' "$wr" \
  && grep -q '訳語を新しく作らない。迷ったら同僚に口で言うときの言い方にする' "$wr" && grep -q '`/opt/andon/skills/glossary/SKILL.md`' "$wr" \
  && pass "writing.md に語の決まり3つと用語集のパスがある" || fail "writing.md に語の決まりか用語集のパスが無い"
# 作った語は「避ける」の行と仮設の語の表にしか出ない。残す語（検査・継ぎ目・条件表・控え）は消さない
left=$(sed '/^## 仮設の語/,$d' "$g" | grep -nE '席|端末の土台|持ち主|提供元' | grep -v '^[0-9]*:_避ける_' || true)
[ -z "$left" ] && pass "用語集に作った語（避ける語）が正式な語として出ない" || fail "用語集に作った語が残る: $left"
grep -q '^\*\*検査\*\*（checks）:' "$g" && grep -q '条件表' "$g" \
  && pass "用語集の残す語（検査・条件表）は残る" || fail "用語集から残す語が消えた"
# ADR-0040 A1: 用語集の 2 つの項は kanban と gate で、「台帳」「ゲート」の項は無い。writing.md の語の決まりの例に「バージョン」があり「台帳」が無い
# old_terms <用語集> <writing.md>: 食い違いを1行で出す（空なら合っている）
old_terms() {
  local m=''
  grep -q '^\*\*kanban\*\*（コードと schema の名前は `ledger`）:' "$1" || m="$m [用語集に kanban の項が無い]"
  grep -q '^\*\*gate\*\*（`andon issue gate`）:' "$1" || m="$m [用語集に gate の項が無い]"
  if grep -qE '^\*\*(台帳|ゲート)\*\*' "$1"; then m="$m [用語集に「台帳」か「ゲート」の項がある]"; fi  # 古い語を見る（ADR-0040 決めたこと 10）
  grep -q '^_避ける_: 台帳（2026-11-20 までの語）' "$1" || m="$m [kanban の項の避けるに「台帳」が無い]"  # 古い語を見る（ADR-0040 決めたこと 10）
  grep -q '^_避ける_: ゲート（2026-11-20 までの語）' "$1" || m="$m [gate の項の避けるに「ゲート」が無い]"  # 古い語を見る（ADR-0040 決めたこと 10）
  grep -q 'ふだんカタカナで言う語（[^）]*バージョン[^）]*）' "$2" || m="$m [writing.md のカタカナの例にバージョンが無い]"
  if grep -q 'もともと日本語で言う語（[^）]*台帳' "$2"; then m="$m [writing.md の日本語の例に「台帳」がある]"; fi  # 古い語を見る（ADR-0040 決めたこと 10）
  echo "$m"
}
miss=$(old_terms "$g" "$wr")
[ -z "$miss" ] && pass "ADR-0040 A1: 用語集に kanban と gate の項があり「台帳」と「ゲート」の項が無い。writing.md の例にバージョンがあり「台帳」が無い" || fail "ADR-0040 A1:$miss"
# 否定の検査が当たる場合があること: 古い項と古い例を戻した写しでは落ちる
miss=
sed 's/^\*\*kanban\*\*（コードと schema の名前は `ledger`）:/**台帳**（ledger）:/' "$g" >"$tmp/glossary.md"  # 古い語を見る（ADR-0040 決めたこと 10）
[ -n "$(old_terms "$tmp/glossary.md" "$wr")" ] || miss="$miss [「台帳」の項を戻した]"
sed 's/^\*\*gate\*\*（`andon issue gate`）:/**ゲート**（gate）:/' "$g" >"$tmp/glossary.md"  # 古い語を見る（ADR-0040 決めたこと 10）
[ -n "$(old_terms "$tmp/glossary.md" "$wr")" ] || miss="$miss [「ゲート」の項を戻した]"
sed 's/もともと日本語で言う語（課題/もともと日本語で言う語（台帳・課題/' "$wr" >"$tmp/writing.md"  # 古い語を見る（ADR-0040 決めたこと 10）
[ -n "$(old_terms "$g" "$tmp/writing.md")" ] || miss="$miss [writing.md の例に「台帳」を戻した]"
[ -z "$miss" ] && pass "ADR-0040 A1: 古い項と古い例を戻した写しでは落ちる（検査が見ている）" || fail "ADR-0040 A1: 写しで落ちない:$miss"
# ADR-0052 A1: writing.md の語の決まりの 1 行とカタカナの例、括弧の決まり、用語集の Chief Engineer の項、
# Chief Engineer の skill と design.md §1.8 の文書の形の 1 行目
# title_terms <writing.md> <用語集> <CE skill> <design.md>: 食い違いを1行で出す（空なら合っている）
title_terms() {
  local m='' ceg
  grep -q '^- 課題や PR の名前は「タイトル」、Chief Engineer が受け持つ検討の単位は「テーマ」と書き、漢字 1 字の「[^」]」では書かない。ダイジェストのテーマや App の色のテーマと紛れる所では、何のテーマかを付ける。$' "$1" \
    || m="$m [writing.md に語の決まりの 1 行が無い]"
  grep -q 'ふだんカタカナで言う語（[^）]*・タイトル・テーマ）' "$1" || m="$m [writing.md のカタカナの例にタイトルとテーマが無い]"
  grep -q '^- 括弧に説明を入れない。.*括弧に入れるのは識別子と、番号に添えるタイトルだけ。$' "$1" || m="$m [writing.md の括弧の決まりが「番号に添えるタイトルだけ」でない]"
  if grep -q '番号に添える題' "$1"; then m="$m [writing.md の括弧の決まりに古い語が残る]"; fi  # 古い語を見る（ADR-0052 決めたこと 10）
  ceg=$(sed -n '/^\*\*Chief Engineer\*\*（chief-engineer。略して CE）:/,/^_避ける_/p' "$2")
  grep -q '受け持つ検討の単位をテーマと呼び、1人1テーマで' <<<"$ceg" || m="$m [用語集の Chief Engineer の項に 1人1テーマ が無い]"
  if grep -q '1人1題' <<<"$ceg"; then m="$m [用語集の Chief Engineer の項に古い語が残る]"; fi  # 古い語を見る（ADR-0052 決めたこと 10）
  grep -qx '# <タイトル>（仕様・設計。日付）' "$3" || m="$m [Chief Engineer の skill の文書の形の 1 行目が # <タイトル>（仕様・設計。日付） でない]"
  grep -qx '# <タイトル>（仕様・設計。日付）' "$4" || m="$m [design.md §1.8 の文書の形の 1 行目が # <タイトル>（仕様・設計。日付） でない]"
  if grep -q '^# <題>' "$3" "$4"; then m="$m [文書の形の 1 行目に古い置き場が残る]"; fi  # 古い語を見る（ADR-0052 決めたこと 10）
  echo "$m"
}
miss=$(title_terms "$wr" "$g" "$ce" docs/design.md)
[ -z "$miss" ] && pass "ADR-0052 A1: writing.md の語の決まりの 1 行と例、括弧の決まり、用語集の 1人1テーマ、文書の形の 1 行目がタイトルとテーマで書いてある" || fail "ADR-0052 A1:$miss"
# 否定の検査が当たる場合があること: 古い語を戻した写しでは落ちる
miss=
sed 's/番号に添えるタイトルだけ/番号に添える題だけ/' "$wr" >"$tmp/writing.md"  # 古い語を見る（ADR-0052 決めたこと 10）
[ -n "$(title_terms "$tmp/writing.md" "$g" "$ce" docs/design.md)" ] || miss="$miss [括弧の決まりに古い語を戻した]"
sed '/^- 課題や PR の名前は「タイトル」/d' "$wr" >"$tmp/writing.md"
[ -n "$(title_terms "$tmp/writing.md" "$g" "$ce" docs/design.md)" ] || miss="$miss [語の決まりの 1 行を消した]"
sed 's/・タイトル・テーマ）/）/' "$wr" >"$tmp/writing.md"
[ -n "$(title_terms "$tmp/writing.md" "$g" "$ce" docs/design.md)" ] || miss="$miss [カタカナの例からタイトルとテーマを消した]"
sed 's/受け持つ検討の単位をテーマと呼び、1人1テーマで/1人1題で/' "$g" >"$tmp/glossary.md"  # 古い語を見る（ADR-0052 決めたこと 10）
[ -n "$(title_terms "$wr" "$tmp/glossary.md" "$ce" docs/design.md)" ] || miss="$miss [用語集に古い語を戻した]"
sed 's/^# <タイトル>（仕様・設計。日付）$/# <題>（仕様・設計。日付）/' "$ce" >"$tmp/ce.md"  # 古い語を見る（ADR-0052 決めたこと 10）
[ -n "$(title_terms "$wr" "$g" "$tmp/ce.md" docs/design.md)" ] || miss="$miss [Chief Engineer の skill の文書の形に古い語を戻した]"
sed 's/^# <タイトル>（仕様・設計。日付）$/# <題>（仕様・設計。日付）/' docs/design.md >"$tmp/design.md"  # 古い語を見る（ADR-0052 決めたこと 10）
[ -n "$(title_terms "$wr" "$g" "$ce" "$tmp/design.md")" ] || miss="$miss [design.md の文書の形に古い語を戻した]"
[ -z "$miss" ] && pass "ADR-0052 A1: 古い語を戻した写しと、決まりの 1 行を消した写しでは落ちる（検査が見ている）" || fail "ADR-0052 A1: 写しで落ちない:$miss"
# オーナーへはふだんの言葉で言う語: drain.go の写しの行が全部、用語集の表にある（逆向きは internal/dispatcher/drain_words_test.go）。
# writing.md の内輪の語の行が表を指す
# owner_words <用語集> <drain.go> <writing.md>: 食い違いを1行で出す（空なら合っている）
owner_words() {
  local m='' table rows=0 word say
  table=$(sed -n '/^## オーナーへはふだんの言葉で言う語$/,/^## /p' "$1")
  while IFS=$'\t' read -r word say; do
    rows=$((rows + 1))
    grep -qxF "| $word | $say |" <<<"$table" || m="$m [用語集の表に「$word」→「$say」の行が無い]"
  done < <(sed -n '/^var ownerWords = /,/^}/p' "$2" | sed -n 's/^[[:space:]]*{"\([^"]*\)", "\([^"]*\)"},$/\1\t\2/p')
  [ "$rows" -gt 0 ] || m="$m [drain.go の ownerWords に行が無い]"
  grep -q '^- 内輪の語は.*言い換えの一覧は用語集の「オーナーへはふだんの言葉で言う語」の表にあり' "$3" || m="$m [writing.md の内輪の語の行が表を指していない]"
  echo "$m"
}
dw=internal/dispatcher/drain.go
miss=$(owner_words "$g" "$dw" "$wr")
[ -z "$miss" ] && pass "drain.go の ownerWords の行が全部、用語集の「オーナーへはふだんの言葉で言う語」の表にあり、writing.md が表を指す" || fail "オーナーへはふだんの言葉で言う語:$miss"
# 検査が当たる場合があること: 用語集の行を消した写し・drain.go に行を足した写し・writing.md の指す文を消した写しでは落ちる
miss=
sed '/^| 地図 | /d' "$g" >"$tmp/glossary.md"
[ -n "$(owner_words "$tmp/glossary.md" "$dw" "$wr")" ] || miss="$miss [用語集の行を消した]"
sed 's/^\(\t{"灯り", "オーナーの判断待ち"},\)$/\1\n\t{"見張り", "確かめ"},/' "$dw" >"$tmp/drain.go"
[ -n "$(owner_words "$g" "$tmp/drain.go" "$wr")" ] || miss="$miss [drain.go に用語集に無い行を足した]"
sed 's/言い換えの一覧は用語集の.*$//' "$wr" >"$tmp/writing.md"
[ -n "$(owner_words "$g" "$dw" "$tmp/writing.md")" ] || miss="$miss [writing.md の指す文を消した]"
[ -z "$miss" ] && pass "オーナーへはふだんの言葉で言う語: 写しで落ちる（検査が見ている）" || fail "オーナーへはふだんの言葉で言う語: 写しで落ちない:$miss"
! grep -q '一度に出すのは互いに依らない問いの全部' "$ce" && grep -q 'ask-one-by-one skill §2 に従う（1回の返事で1問）' "$ce" \
  && pass "CE skill の問いの出し方は ask-one-by-one §2 を指す（束で出す行が無い）" || fail "CE skill の問いの出し方が ask-one-by-one §2 を指していない"
fm package/skills/ask-one-by-one/SKILL.md description | grep -q 'Chief Engineer（CE）' \
  && pass "ask-one-by-one の description の読み手に CE がある" || fail "ask-one-by-one の description に CE が無い"
