#!/usr/bin/env bash
# andon の葉の表から作るものと、配る文の中の andon のコマンド（ADR-0053 受け入れ条件 A7・A8。決めたこと 11・16・21）。
#   A7 docs/commands.md と bash の補完（image/completions/andon）は、andon help --all --markdown・--bash で作り直しても変わらない。
#      作ったものを1行変えると、比べて落ちる。補完は bash として読め、名詞と動詞を補う。
#   A8 package/skills/ の文（`` で囲んだ語とコードの囲みの中）と package/hooks/・package/templates/ の全部の行の
#      andon <名詞> <動詞> は、全部が表にある（tests/lib/andon-refs.sh。読み方は検証の skill の検査と同じ）。
#      古い名前・綴りの誤り・表に無い動詞を書き戻すと落ちる（引用符・空白の数・入れ子の置き場・印の行を変種で見る）。
#      ホストの部品（package/host/*/main.js）が打つ引数は、waiting --json・waiting answer・issue board・issue history で、どれも表にある（ADR-0059 受け入れ条件 12）。
# 道具は andon（tests/lib/andon.sh）と perl。kanban も hc も起動しない。
set -uo pipefail
cd "$(dirname "$0")/.." || exit 2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/andon.sh
. tests/lib/andon-refs.sh

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

andon_leaves >"$tmp/leaves"
n=$(wc -l <"$tmp/leaves")
grep -qx 'issue start' "$tmp/leaves" && grep -qx 'open' "$tmp/leaves" && [ "$n" -ge 50 ] \
  && pass "andon help --all から葉 $n を読む" || { fail "andon help --all から葉が読めない（n=$n。下の確かめが何も見ない）"; exit 1; }

# === A7 生成物が表と揃う ===
"$andon" help --all --markdown >"$tmp/commands.md" && "$andon" help --all --bash >"$tmp/completion" \
  || fail "andon help --all --markdown・--bash が通らない"
for p in "docs/commands.md:--markdown:$tmp/commands.md" "image/completions/andon:--bash:$tmp/completion"; do
  IFS=: read -r f flag g <<<"$p"
  if cmp -s "$f" "$g"; then pass "$f は葉の表から作り直しても変わらない"
  else fail "$f が葉の表と揃わない（andon help --all $flag >$f で作り直す）: $(diff "$f" "$g" | sed -n 1,5p)"; fi
done
# 比べ方が何も見ていなくても通らないこと: 説明を1つ変えたものは揃わないと読む
sed '0,/^| `andon issue start` |/s//| `andon issue start` | 変えた説明 |/' "$tmp/commands.md" >"$tmp/commands.bad"
! cmp -s "$tmp/commands.md" "$tmp/commands.bad" && pass "説明を1つ変えた一覧は、作り直したものと揃わない" || fail "説明を変えても比べ方が気づかない"
while IFS= read -r l; do grep -qF "| \`andon $l\` |" "$tmp/commands.md" || fail "docs/commands.md に andon $l の行が無い"; done <"$tmp/leaves"
grep -q '^usage:\|--project\|<id>' "$tmp/commands.md" && fail "docs/commands.md に使い方やフラグが載る（決めたこと 16）" \
  || pass "docs/commands.md は葉ごとに打つ者と説明だけを載せる"

# 補完: bash として読め、1語目に名詞、2語目に動詞、open に slug を補う
mkdir -p "$tmp/ws/projects/sample-site/kanban" && : >"$tmp/ws/projects/sample-site/kanban/config.yaml"
got=$(ANDON_WORKSPACE="$tmp/ws" bash -c '
  complete() { :; }
  . "$1"
  c() { COMP_WORDS=("$@"); COMP_CWORD=$(($# - 1)); COMPREPLY=(); _andon; echo "${COMPREPLY[*]}"; }
  c andon iss; c andon issue rec; c andon triage ""; c andon open sam; c andon issue start ""' _ image/completions/andon 2>&1)
[ "$got" = "$(printf 'issue\nrecord\ncollect apply verdict\nsample-site\n')" ] \
  && pass "bash の補完は名詞・動詞・open の slug を補い、3語目からは補わない" || fail "補完: $got"

# === A8 配る文の中の andon のコマンドが表にある ===
skills=$(find package/skills -name '*.md' | sort)
# shellcheck disable=SC2086 # find の結果は空白を含まないパス
{ andon_refs md $skills; andon_refs code package/hooks/* package/templates/*; } >"$tmp/refs"
bad=$(andon_bad_refs "$tmp/leaves" <"$tmp/refs")
m=$(wc -l <"$tmp/refs")
for want in 'package/hooks/stop:.*	dispatcher	drain' 'package/templates/claude.md:.*	issue	session' 'package/skills/worker/SKILL.md:.*	issue	ready'; do
  grep -q "$want" "$tmp/refs" || fail "andon のコマンドの読み方が壊れている（$want を拾わない）"
done
[ -z "$bad" ] && [ "$m" -gt 100 ] && pass "package/ の skill・フック・テンプレートの andon のコマンド $m 件が、全部表にある" \
  || fail "表に無い andon のコマンド（$m 件を見た）:"$'\n'"$bad"

# 落ちるべき入力の変種: 古い名前・綴りの誤り・表に無い動詞・引用符・空白の数・パスつきの andon・後ろにつく句読点・入れ子の置き場・フックの中の呼び出し。
# 印の行・名指しの形（<名詞>）・1語の葉の引数・名詞だけは通る
mkdir -p "$tmp/pkg/skills/x/playbooks" "$tmp/pkg/hooks"
cat >"$tmp/pkg/skills/x/SKILL.md" <<'EOF'
古い名前は `andon task start S-0012` だった。
`andon issue recrd S-0012` で残す。
```
andon  issue   submit S-0012
"andon" 'waiting' list
/opt/andon/bin/andon issue stat S-0012
```
句読点のつく `andon issue sumbit,` と `andon issue revue.` も見る。
用語集の行 `andon task` <!-- andon の古い名前 -->
通る: `andon <名詞> <動詞>`・`andon open sample-site`・`andon gate S-0012`・`andon waiting --json`・`andon issue`・`andon issue start <id>`
地の文の andon design start は見ない（`` の外）。
EOF
printf '%s\n' '| `andon design start "<タイトル>"` | 入れ子の置き場 |' >"$tmp/pkg/skills/x/playbooks/deep.md"
printf '%s\n' '#!/bin/sh' 'out=$(andon dispatcher drian 2>/dev/null)' >"$tmp/pkg/hooks/stop"
{ andon_refs md "$tmp/pkg/skills/x/SKILL.md" "$tmp/pkg/skills/x/playbooks/deep.md"; andon_refs code "$tmp/pkg/hooks/stop"; } >"$tmp/refs.bad"
got=$(andon_bad_refs "$tmp/leaves" <"$tmp/refs.bad" | sed 's/^.*: andon /andon /' | sort | tr '\n' '|')
want='andon design start|andon dispatcher drian|andon issue recrd|andon issue revue|andon issue stat|andon issue submit|andon issue sumbit|andon task start|andon waiting list|'
[ "$got" = "$want" ] && pass "書き戻した古い名前・誤った綴り・表に無い動詞は、引用符・空白・パス・句読点・入れ子・フックの中でも落ち、印の行と名指しの形は通る" \
  || fail "変種: got=[$got] want=[$want]"

# ホストの部品が打つ引数（引数を配列で渡すので、上の読み方では見えない。決めたこと 21）
host=$(perl -ne 'while (/exec\("andon", \[\s*"([^"]+)"(?:,\s*"([^"]+)")?/g) { print "$1" . (defined $2 ? " $2" : "") . "\n" }' package/host/*/main.js | LC_ALL=C sort -u)
if [ "$host" = "$(printf 'issue board\nissue history\nwaiting --json\nwaiting answer')" ] && grep -qx 'waiting' "$tmp/leaves" && grep -qx 'waiting answer' "$tmp/leaves" \
  && grep -qx 'issue board' "$tmp/leaves" && grep -qx 'issue history' "$tmp/leaves"; then
  pass "ホストの部品が打つのは andon waiting --json・waiting answer・issue board・issue history で、どれも表にある"
else fail "ホストの部品の引数: [$host]"; fi
# 引数の拾い方が何も見ていなくても通らないこと: 表に無い葉や書く葉を足した写しは、一覧が変わる
extra=$(printf '%s\n' 'exec("andon", ["issue", "done", id], { cwd });' 'exec("andon", ["issue", "board", "--json"], { cwd });' \
  | perl -ne 'while (/exec\("andon", \[\s*"([^"]+)"(?:,\s*"([^"]+)")?/g) { print "$1" . (defined $2 ? " $2" : "") . "\n" }' | LC_ALL=C sort -u | tr '\n' '|')
[ "$extra" = 'issue board|issue done|' ] && pass "ホストの部品の引数を拾う式は、足した呼び出しを拾う" || fail "ホストの部品の引数を拾う式: [$extra]"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
