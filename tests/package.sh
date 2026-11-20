#!/usr/bin/env bash
# tests/package.sh — package/ の skill・プロファイル・ゲートの条件表・managed settings・statusline（design.md §2.2・§2.4）。
# 節は tests/package/ に1節1ファイルで置き、この入口が名前の順（LC_ALL=C）に同じシェルで source する。
# 節の一覧は tests/package/ のファイル名で、各節が何を見るかはそのファイルの冒頭に書く。この入口は節の一覧を持たない。
# 節を足すときは tests/package/<番号>-<名前>.sh を新しく置き、この入口とほかの節は書き換えない。番号は今ある最大の次。
# 並べて動く PR が同じ番号を選んでも、名前が違えば両方が名前の順に回る。
# 節は同じシェルで続けて回るので、前の節が置いた変数（cos・pb など）を後ろの節が使う。
# 道具は bash・jq・python3・andon（tests/lib/andon.sh。JSON は andon schema check で schema と照らす）。
set -u
cd "$(dirname "$0")/.." || exit 2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/andon.sh

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

skills="ask-one-by-one chief-engineer chief-of-staff glossary independent-review retro triage worker"
profiles="chief-engineer chief-of-staff claude claude-review codex codex-review fable sonnet"

# frontmatter の key の値（最初の --- から次の --- まで）
fm() { awk -v k="$2" 'NR==1 && $0!="---" {exit} NR>1 && $0=="---" {exit} NR>1 && index($0, k": ")==1 {print substr($0, length(k)+3); exit}' "$1"; }

mapfile -t sections < <(printf '%s\n' tests/package/*.sh | LC_ALL=C sort)
[ -f "${sections[0]}" ] || { echo "FAIL tests/package/ に節が無い" >&2; exit 1; }
for section in "${sections[@]}"; do
  # source は文法の誤りでも残りを飛ばして先へ進むので、先に読めるかを見て、読めない節は落とす
  bash -n "$section" 2>"$tmp/syntax" || { fail "$section が読めない: $(head -1 "$tmp/syntax")"; continue; }
  # shellcheck source=/dev/null
  . "$section"
done

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
