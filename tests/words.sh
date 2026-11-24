#!/usr/bin/env bash
# 置き換えた「今」の語が repo に戻っていないか（ADR-0013 決めたこと 4・7、受け入れ条件 3）。当たれば落ちる。
# 役を指す形の「人」が替えた範囲に戻っていないかも見る（ADR-0036 決めたこと 7、受け入れ条件 2。下の段）。
# 使い方: bash tests/words.sh [検査する根（既定: repo の根）]
# 語は句で持つ（「席」は「離席」に、「起こす」は「引き起こす」に当てない）。リストは秘密ではない。
#   費用→コスト・席→言い換え・持ち主→オーナー・逃げ口→フォールバック・文脈→コンテキスト・提供元→プロバイダ・
#   端末の土台→hc・迎える→追加する・起こす（エージェントを）→起動する・起きたとき→起動したとき
# 範囲の外: .github/（オーナーが見る範囲）。例外: このファイルと tests/words-selftest.sh・用語集の「_避ける_」の行・
#   docs/decisions.md の「語の改め」の段・tests/package/08-writing.sh の用語集の検査の行・
#   ADR-0022 のオーナーの言葉の引用（「」の中の「席」。オーナーがそう言った）・CHANGELOG.md のバージョンの節（## v…。バージョンを切ったときの記録）。
#   ADR-0013 とその決定のファイルでは、語そのものを挙げた所（「」・``・（）の中、「X→」の形、語の表の「今」の列）だけを
#   外す。地の文の語は当たる。
# tests/.task-tmp（tests/isolation が置く一時の置き場。selftest の写しが入る）は見ない。
# どの段も grep に -H を付け、範囲の空でないときだけ回す。範囲がファイル1つだと、-H が無い grep はファイル名を出さず、
#   ファイル名で読む例外と当たりの行（perl の「ファイル:行:」）が素通りする。範囲が空の grep -r は根の全体（.github/ も）を見る。
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "${1:-$here/..}" && pwd)"
cd "$root"

words='費用|持ち主|逃げ口|文脈|提供元|端末の土台|迎え(?:る|た|て|ず|な)|の席|(?<!離)(?<!着)(?<!出)席(?:だけ|を|が|の無)|(?<!引き)(?<!書き)起こ[さしすせそ]|で起こる|起きた(?:とき|時)'
paths=(docs package init image internal cmd tests process .claude README.md CHANGELOG.md AGENTS.md REVIEW.md SECURITY.md)
existing=()
for p in "${paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done

changelog_v=$(grep -n -m1 '^## v' CHANGELOG.md 2>/dev/null | cut -d: -f1 || true)
hits=""
[ "${#existing[@]}" -gt 0 ] && hits=$(grep -rnHIP "$words" "${existing[@]}" \
  --exclude-dir=.git --exclude-dir=.task-tmp --exclude-dir=node_modules \
  | grep -v -e '^tests/words\.sh:' -e '^tests/words-selftest\.sh:' \
    -e '^package/skills/glossary/SKILL\.md:[0-9]*:_避ける_' \
    -e '^docs/decisions\.md:[0-9]*:語の改め（2026-11-10 から）' \
    -e '^tests/package/08-writing\.sh:[0-9]*:.*席|端末の土台|持ち主|提供元' \
    -e '^docs/adr/0022-advisor-and-chief-engineer\.md:[0-9]*:.*「[^」]*席' \
  | awk -F: -v v="${changelog_v:-0}" '!($1 == "CHANGELOG.md" && v > 0 && $2 >= v)' \
  | perl -CSDA -Mutf8 -ne '
      BEGIN { $w = shift }
      my ($f, $n, $t) = /^([^:]*):(\d+):(.*)$/s or next;
      if ($f =~ m{^docs/(adr/0013-|decisions/2026-11-10-adr-0013-)}) {
        $t =~ s/^\s*\|[^|]*\|//;                      # 語の表の「今」の列
        1 while $t =~ s/「[^「」]*」//g;                 # 引用と、語を挙げた「」（入れ子は内から）
        1 while $t =~ s/（[^（）]*）//g;
        $t =~ s/`[^`]*`//g;
        $t =~ s/[^\s・（「]+→//g;                        # 「費用→コスト」の形
        $t =~ /$w/ or next;
      }
      print' "$words" || true)

if [ -n "$hits" ]; then
  echo "FAIL words: ADR-0013 決めたこと 4 の「今」の語が残る（直したあとの語に置き換える）" >&2
  printf '%s\n' "$hits" | cut -c1-200 >&2
  exit 1
fi
echo "ok   words: ADR-0013 決めたこと 4 の「今」の語が repo に出ない"

# 役を指す形の「人」（ADR-0036 決めたこと 7、受け入れ条件 2）。「人」の後ろに助詞か日付（括弧の中か空白のあと）が続き、
# 前に数字も「外の」「読む」のような語も無い形を禁じる。前の数字が日付（2026-11-05 人が）か番号（A5 人の）の終わりなら当たり、名札の「（2026-11-12 人）」も当たる。役を指さない「人」（外の人・読む人・1人・個人・人が読む・人向け・
# 人と AI が読む・人の名前）は通る。範囲は替え終えた場所だけで、替えるたびに role_paths に足す。
# 例外: このファイルと tests/words-selftest.sh（落ちる例）・
#   古い語を読む所（internal/triage と tests/andon-triage.sh の「人に聞くこと」「（人の判断待ち）」、internal/retro と
#   tests/andon-retro.sh の前のバージョンの答え「人がマージした」。ADR-0036 決めたこと 6）・
#   古い文に当たることを確かめる行（tests/andon-gate.sh の役の語の検査・tests/package/06-skill-commands.sh の古い文）・
#   docs/design.md の引用（§0 のビジョンの写し・freee の KPI「人の介在率」・pstack の「人を待たない」。ADR-0036 決めたこと 5）・
#   docs/contract.md が古い語も読むと書いた所（古い見出し `## 人に聞くこと`・古い末尾「（人の判断待ち）」・前のバージョンの答え
#   「人がマージした（…）」。ADR-0036 決めたこと 6）・
#   CHANGELOG.md の版の節（## v…。版を切ったときの記録。ADR-0036 決めたこと 4）。
#   ADR と決定の記録（docs/adr/・docs/decisions/・docs/decisions.md）では、バッククォートの中（当時の識別子）と、
#   名指しした「」だけを外す（ADR-0036 決めたこと 4）。名指しするのは、オーナーの発言・外の文の写し（pstack・LLM wiki・
#   ビジョン）・数えたデータの値（前のバージョンの答え「人がマージした」）・「手で書く」の意味の「人が書く YAML」。
#   節の名前や当時の文の写しの「」（「人の入口」など）は名指しせず、当たる。ほかに外すのは、役でない 2 つの句
#   （ADR-0005 の「人と人が共有する」・ADR-0011 の「トラッカー側の人が決める」）と、ADR-0036 とその決定のファイルの
#   「」の中・語の表の「今」の列・「X→」の形（語そのものを挙げた所）。名指しした引用だけを外し、同じ行のほかの「人」は当たる。
role='(?<![0-9０-９一二三四五六七八九十何個他本外数うくすつぬむるいたほ])(?:(?<![0-9] )|(?<=-[0-9]{2} )|(?<=[A-Za-z][0-9] ))(?<!外の)(?<!ほかの)(?<!他の)人(?!間|用|向|数|物|手|が読|と ?AI|の名前)(?:[がにへのはをともや]|（[0-9]|から|だけ| [0-9]{4}-|(?<=-[0-9]{2} 人)[）)])'
role_paths=(package internal init cmd tests docs/design.md process docs/process model docs/contract.md docs/dispatcher.md
  docs/operate.md docs/image.md README.md AGENTS.md REVIEW.md SECURITY.md CHANGELOG.md docs/adr docs/decisions docs/decisions.md
  .claude/skills/verify-andon)
existing=()
for p in "${role_paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done
hits=""
[ "${#existing[@]}" -gt 0 ] && hits=$(grep -rnHIP "$role" "${existing[@]}" --exclude-dir=.git --exclude-dir=.task-tmp --exclude-dir=node_modules \
  | grep -v -e '^tests/words\.sh:' -e '^tests/words-selftest\.sh:' \
    -e '^internal/triage/[^:]*:[0-9]*:.*\(人に聞くこと\|（人の判断待ち）\)' \
    -e '^tests/andon-triage\.sh:[0-9]*:.*\(人に聞くこと\|（人の判断待ち）\)' \
    -e '^internal/retro/[^:]*:[0-9]*:.*人がマージした' -e '^tests/andon-retro\.sh:[0-9]*:.*人がマージした' \
    -e '^tests/andon-gate\.sh:[0-9]*:.*古い見出しに当たらない' \
    -e "^tests/package/06-skill-commands\.sh:[0-9]*:.*\(old_cos=\|old_docs=\|<<<'\)" \
  | awk -F: -v v="${changelog_v:-0}" '!($1 == "CHANGELOG.md" && v > 0 && $2 >= v)' \
  | perl -CSDA -Mutf8 -ne '
      BEGIN { $r = shift }
      my ($f, $t) = /^([^:]*):\d+:(.*)$/s or next;
      if ($f =~ m{^docs/(?:adr/|decisions/|decisions\.md$)}) {
        if ($f =~ m{^docs/(?:adr/0036-|decisions/2026-11-20-(?:adr-0036-|role-name-replacement-))}) {
          $t =~ s/^\s*\|[^|]*\|//;                    # 語の表の「今」の列
          $t =~ s/[^\s・（「]+→//g;                    # 「人→オーナー」の形
          1 while $t =~ s/「[^「」]*」//g;               # 語を挙げた「」（入れ子は内から）
        }
        for my $q (                                    # 名指しした引用（オーナーの発言・外の文・データの値・手で書く）
          "人の他に、Sentry や Cloudflare、Linear、GitHub が Signal になるといい",
          "人を待たない", "人と LLM が一緒に育てる", "人が直すたびに回す",
          "人は、自分にしかできない判断だけをする。ラインは人が見ていない間も回り、灯りがつくのはその判断のときだけ",
          "人が PR を読んでマージする", "人がマージした（PR の URL）", "人がマージした",
          "`issue.yaml` は人が書くまま、モデルと照らす", "人が書いた YAML", "人が書く YAML（残り 9 本）") {
          $t =~ s/\Q「$q」\E//g;
        }
        $t =~ s/`[^`]*`//g;
        $t =~ s/人と人が共有する//g if $f =~ m{^docs/adr/0005-};
        $t =~ s/トラッカー側の人が決める//g if $f =~ m{^docs/adr/0011-};
        $t =~ /$r/ or next;
      }
      if ($f eq "docs/design.md") {
        $t =~ s/止められるものは機械、判断が要るものは LLM、残りだけ人。ラインは回り続け（AND ON）、人を呼ぶときだけ灯りがつく（アンドン）//g;
        $t =~ s/「人の介在率」|「人を待たない」//g;
        $t =~ /$r/ or next;
      }
      if ($f eq "docs/contract.md") {
        $t =~ s/古い見出し `## 人に聞くこと`|古い末尾「（人の判断待ち）」|前のバージョンの「人がマージした（…）」//g;
        $t =~ /$r/ or next;
      }
      print' "$role" || true)
if [ -n "$hits" ]; then
  echo "FAIL words: 役を指す「人」が残る（ADR-0036 決めたこと 1。「オーナー」と書く）" >&2
  printf '%s\n' "$hits" | cut -c1-200 >&2
  exit 1
fi
echo "ok   words: 役を指す「人」が替えた範囲（${role_paths[*]}）に出ない"

# 「台帳」「ゲート」「版」（ADR-0040 決めたこと 10、受け入れ条件 2）。「台帳」と「ゲート」はどの形も、「版」は残す形
# （最新版・初版・英語版・更新版・出版・年に続く「2006 版」・製品の名前に続く形「Go 版」）のほかを禁じる。
# 「」の中（引用と、語そのものを挙げた所）と用語集の「_避ける_」の行は外す。バッククォートの中は外さない。
# 古い語を読むための行も外す（ADR-0040 決めたこと 7）: internal/issue/settle.go の読み替えの表（var oldWords）と、その検査の表の行
#   （internal/issue/settle_test.go の {" で始まる行）、古い語の事実の行で問いを置く検査（tests/andon-dispatcher.sh・
#   tests/andon-issue.sh の --arg r "ゲートの記録 …）、internal/gate/ の Go のうち行の末尾に「// 古い語を読む（ADR-0040 決めたこと 7）」
#   と書いた行（gate の理由の文の正規表現、gate の問いの事実の行の古い頭、その検査）。
# 古い語を見る検査の行も外す: tests/ の .sh のうち行の末尾に「# 古い語を見る（ADR-0040 決めたこと 10）」と書いた行（古い語が無いことを
#   見る grep と、古い語を戻した写し。まだ古い語で書く出力を照らす行）。このファイルと tests/words-selftest.sh も外す。
# 生きている文書のうち、CHANGELOG.md のバージョンごとの節（## v…。バージョンを切ったときの記録。ADR-0040 決めたこと 6）は外す。
# 決定の記録は docs/decisions/ と docs/decisions.md の全部（凍結した表も）を見る。
# ADR は全部を見る。オーナーの発言を「」に入れずに写した ADR-0018 の 1 行（「オーナーの期待（2026-11-11）: 検証の…」）と、
#   ADR-0040 の「決めたこと」の表の「今」の列（行の頭が「   | 」の表の最初の欄。語そのものを挙げた所）は外す。
# 範囲は替え終えた場所だけで、替えるたびに old_paths に足す（.github/ は足さない）。
old_paths=(package .yomiyasu-words internal cmd init tests .claude/skills/verify-andon docs/design.md
  docs/contract.md docs/dispatcher.md docs/operate.md docs/image.md docs/ledger.md README.md REVIEW.md SECURITY.md CHANGELOG.md
  docs/decisions.md process docs/process model image AGENTS.md .golangci.yml workflow_test.go
  docs/adr docs/decisions)
existing=()
for p in "${old_paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done
hits=""
[ "${#existing[@]}" -gt 0 ] && hits=$(grep -rnHI -e '台帳' -e 'ゲート' -e '版' "${existing[@]}" --exclude-dir=.git --exclude-dir=.task-tmp \
  | grep -v -e '^package/skills/glossary/SKILL\.md:[0-9]*:_避ける_' \
    -e '^tests/words\.sh:' -e '^tests/words-selftest\.sh:' \
    -e '^tests/[^:]*\.sh:[0-9]*:.*# 古い語を見る（ADR-0040 決めたこと 10）$' \
    -e '^internal/gate/[a-z_]*\.go:[0-9]*:.*// 古い語を読む（ADR-0040 決めたこと 7）$' \
    -e '^internal/issue/settle\.go:[0-9]*:var oldWords = ' \
    -e '^internal/issue/settle_test\.go:[0-9]*:[[:space:]]*{"' \
    -e '^tests/andon-\(dispatcher\|issue\)\.sh:[0-9]*:jq -n --arg r "ゲートの記録 ' \
    -e '^docs/adr/0018-screen-check-tooling\.md:[0-9]*:オーナーの期待（2026-11-11）: 検証の' \
  | awk -F: -v v="${changelog_v:-0}" '!($1 == "CHANGELOG.md" && v > 0 && $2 >= v)' \
  | perl -CSDA -Mutf8 -ne '
      my ($f, $n, $t) = /^([^:]*):(\d+):(.*)$/s or next;
      $t =~ s/^   \| [^|]*\|// if $f eq "docs/adr/0040-kanban-gate-version-words.md";   # 決めたこと の表の「今」の列
      1 while $t =~ s/「[^「」]*」//g;                     # 引用と、語を挙げた「」（入れ子は内から）
      $t =~ s/最新版|初版|英語版|更新版|出版|[0-9]{4} ?版|[A-Za-z][A-Za-z0-9.]* ?版//g;   # 残す「版」
      $t =~ /台帳|ゲート|版/ or next;
      print' || true)
if [ -n "$hits" ]; then
  echo "FAIL words: 「台帳」「ゲート」「版」が残る（ADR-0040。kanban・gate・バージョンと書く）" >&2
  printf '%s\n' "$hits" | cut -c1-200 >&2
  exit 1
fi
echo "ok   words: 「台帳」「ゲート」「版」が替えた範囲（${old_paths[*]}）に出ない"

# 漢字 1 字の「題」（ADR-0052 決めたこと 10、受け入れ条件 2）。前後が熟語（課題・問題・主題・話題・議題・本題・題名・題材）の字で
# ない「題」を禁じる。名前は「タイトル」、Chief Engineer が受け持つ検討の単位は「テーマ」と書く。
# バッククォートの中も見る（置き場の形 `<…>` はバッククォートの中にある）。「」の中も丸ごとは外さない。
# 例外は名指しした句だけを外し、同じ行のほかの字は見る: このファイルと tests/words-selftest.sh・package/writing.md の語の決まりの行の
#   「漢字 1 字の「…」では書かない」（かぎ括弧で囲んだ 1 字の形で語を挙げた所）・古い語が無いことを見る検査の行（tests/ の .sh のうち
#   行の末尾に「# 古い語を見る（ADR-0052 決めたこと 10）」と書いた行）の単引用符の中（grep や sed に渡す古い形の文字列）。
#   印のある行も、単引用符の外の字は見る。
# 範囲は替え終えた場所だけで、替えるたびに dai_paths に足す（docs/vision.md と .github/ は足さない）。
dai='(?<![課問主話議本])題(?![名材])'
dai_paths=(package init tests docs/design.md internal cmd .claude/skills/verify-andon)
existing=()
for p in "${dai_paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done
hits=""
[ "${#existing[@]}" -gt 0 ] && hits=$(grep -rnHIP "$dai" "${existing[@]}" --exclude-dir=.git --exclude-dir=.task-tmp --exclude-dir=node_modules \
  | grep -v -e '^tests/words\.sh:' -e '^tests/words-selftest\.sh:' \
  | perl -CSDA -Mutf8 -ne '
      BEGIN { $d = shift }
      my ($f, $n, $t) = /^([^:]*):(\d+):(.*)$/s or next;
      $t =~ s/漢字 1 字の「題」では書かない//g if $f eq "package/writing.md" && $t =~ /^- 課題や PR の名前は「タイトル」/;
      if ($f =~ m{^tests/.*\.sh$} && $t =~ s/# 古い語を見る（ADR-0052 決めたこと 10）$//) {
        $t =~ s/\x27[^\x27]*\x27//g;                     # 印のある行は、単引用符の中（古い形の文字列）だけを外す
      }
      $t =~ /$d/ or next;
      print' "$dai" || true)
if [ -n "$hits" ]; then
  echo "FAIL words: 漢字 1 字の「題」が残る（ADR-0052。名前は「タイトル」、Chief Engineer が受け持つ検討の単位は「テーマ」と書く）" >&2
  printf '%s\n' "$hits" | cut -c1-200 >&2
  exit 1
fi
echo "ok   words: 漢字 1 字の「題」が替えた範囲（${dai_paths[*]}）に出ない"

# 句読点（「、」「。」「・」）の前後の余分な半角の空白。語を perl で一括して置き換えるとき、\p{Han}・\p{Katakana} は
# Script_Extensions で句読点も含むので、和字と英字の間に空白を足す規則が「、 kanban」「。 gate」「kanban ・」を作る。
# 置き換えの誤りを差分を読む前に拾う。見るのは、句読点のすぐ後ろの空白と、英数字と句読点の間の空白（「kanban ・」）。
# 通るもの: 後ろの空白は、後ろが URL（URL の前後には空白を置く決まり）・表の区切りの「|」・「→」・閉じる「``」（空白を挟む
#   コードの書き方）・行の末尾なら通る。空白が 2 つ以上続けば、後ろが何でも落ちる（行の末尾を除く）。前の空白は、前が URL なら通る。
# 例外: このファイルと tests/words-selftest.sh（落ちる例）・triage の要約の行（「閉じた N ・ 直す N」。空白を挟む「・」で項目を区切る形）・
#   docs/decisions/2026-11-13-cos-principles-playbooks.md の表の、元の文を写した行（改行を空白にした写し）。
punct='(?:、|。|・) (?!https?://|[|→]|``|\s*$)|(?<=[A-Za-z0-9]) (?:、|。|・)'
punct_paths=("${paths[@]}" model)
existing=()
for p in "${punct_paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done
hits=""
[ "${#existing[@]}" -gt 0 ] && hits=$(grep -rnHIP ' (?:、|。|・)|(?:、|。|・) ' "${existing[@]}" --exclude-dir=.git --exclude-dir=.task-tmp --exclude-dir=node_modules \
  | grep -v -e '^tests/words\.sh:' -e '^tests/words-selftest\.sh:' \
    -e '^\(internal/triage/[a-z_]*\.go\|package/skills/triage/SKILL\.md\|tests/andon-triage\.sh\):[0-9]*:.*閉じ[たる] .* ・ 直す ' \
    -e '^docs/decisions/2026-11-13-cos-principles-playbooks\.md:[0-9]*:| [0-9]* | `' \
  | perl -CSDA -Mutf8 -ne '
      BEGIN { $p = shift }
      my ($f, $n, $t) = /^([^:]*):(\d+):(.*)$/s or next;
      $t =~ s{https?://\S+ }{https://}g;                # URL とその後ろの空白（決まりの形）を外す
      $t =~ /$p/ or next;
      print' "$punct" || true)
if [ -n "$hits" ]; then
  echo "FAIL words: 句読点（、。・）の前後に余分な半角の空白がある（置き換えの規則が \\p{Han}・\\p{Katakana} で句読点にも当たった。空白を消す）" >&2
  printf '%s\n' "$hits" | cut -c1-200 >&2
  exit 1
fi
echo "ok   words: 句読点（、。・）の前後に余分な半角の空白が無い"
