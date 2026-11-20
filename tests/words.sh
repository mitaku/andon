#!/usr/bin/env bash
# 置き換えた「今」の語が repo に戻っていないか（ADR-0013 決めたこと 4・7、受け入れ条件 3）。当たれば落ちる。
# 役を指す形の「人」が替えた範囲に戻っていないかも見る（ADR-0036 決めたこと 7、受け入れ条件 2。下の段）。
# 使い方: bash tests/words.sh [検査する根（既定: repo の根）]
# 語は句で持つ（「席」は「離席」に、「起こす」は「引き起こす」に当てない）。リストは秘密ではない。
#   費用→コスト・席→言い換え・持ち主→オーナー・逃げ口→フォールバック・文脈→コンテキスト・提供元→プロバイダ・
#   端末の土台→hc・迎える→追加する・起こす（エージェントを）→起動する・起きたとき→起動したとき
# 範囲の外: .github/（人が見る範囲）。例外: このファイルと tests/words-selftest.sh・用語集の「_避ける_」の行・
#   docs/decisions.md の「語の改め」の段・tests/package/08-writing.sh の用語集の検査の行・
#   ADR-0022 の人の言葉の引用（「」の中の「席」。人がそう言った）・CHANGELOG.md の版の節（## v…。版を切ったときの記録）。
#   ADR-0013 とその決定のファイルでは、語そのものを挙げた所（「」・``・（）の中、「X→」の形、語の表の「今」の列）だけを
#   外す。地の文の語は当たる。
# tests/.task-tmp（tests/isolation が置く一時の置き場。selftest の写しが入る）は見ない。
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "${1:-$here/..}" && pwd)"
cd "$root"

words='費用|持ち主|逃げ口|文脈|提供元|端末の土台|迎え(?:る|た|て|ず|な)|の席|(?<!離)(?<!着)(?<!出)席(?:だけ|を|が|の無)|(?<!引き)(?<!書き)起こ[さしすせそ]|で起こる|起きた(?:とき|時)'
paths=(docs package init image internal cmd tests process .claude README.md CHANGELOG.md AGENTS.md REVIEW.md SECURITY.md)
existing=()
for p in "${paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done

changelog_v=$(grep -n -m1 '^## v' CHANGELOG.md 2>/dev/null | cut -d: -f1 || true)
hits=$(grep -rnIP "$words" "${existing[@]}" \
  --exclude-dir=.git --exclude-dir=.task-tmp \
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

# 役を指す形の「人」（ADR-0036 決めたこと 7、受け入れ条件 2）。「人」の後ろに助詞か日付の括弧が続き、
# 前に数字も「外の」「読む」のような語も無い形を禁じる。役を指さない「人」（外の人・読む人・1人・個人・人が読む・人向け・
# 人と AI が読む・人の名前）は通る。範囲は替え終えた場所だけで、替えるたびに role_paths に足す。
role='(?<![0-9０-９一二三四五六七八九十何個他本外数うくすつぬむるいたほ])(?<![0-9] )(?<!外の)(?<!ほかの)(?<!他の)人(?!間|用|向|数|物|手|が読|と ?AI|の名前)(?:[がにへのはをともや]|（[0-9]|から|だけ)'
role_paths=(package)
existing=()
for p in "${role_paths[@]}"; do [ -e "$p" ] && existing+=("$p"); done
hits=""
[ "${#existing[@]}" -gt 0 ] && hits=$(grep -rnIP "$role" "${existing[@]}" --exclude-dir=.git --exclude-dir=.task-tmp || true)
if [ -n "$hits" ]; then
  echo "FAIL words: 役を指す「人」が残る（ADR-0036 決めたこと 1。「オーナー」と書く）" >&2
  printf '%s\n' "$hits" | cut -c1-200 >&2
  exit 1
fi
echo "ok   words: 役を指す「人」が替えた範囲（${role_paths[*]}）に出ない"
