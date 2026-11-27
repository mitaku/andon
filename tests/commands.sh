#!/usr/bin/env bash
# andon の葉の表から作るものと、配る文の中の andon のコマンド（ADR-0053 受け入れ条件 A7・A8。決めたこと 11・16・21）。
#   A7 docs/commands.md と bash の補完（image/completions/andon）は、andon help --all --markdown・--bash で作り直しても変わらない。
#      作ったものを1行変えると、比べて落ちる。補完は bash として読め、名詞と動詞を補う。
#   A8 package/skills/ の文（`` で囲んだ語とコードの囲みの中）と package/hooks/・package/templates/ の全部の行の
#      andon <名詞> <動詞> は、全部が表にある（tests/lib/andon-refs.sh。読み方は検証の skill の検査と同じ）。
#      古い名前・綴りの誤り・表に無い動詞を書き戻すと落ちる（引用符・空白の数・入れ子の置き場・印の行を変種で見る）。
#      名詞だけの形が古い名前の名詞（andon waiting）は、名詞だけで書くと落ちる（tests/lib/andon-refs.sh の andon_retired_nouns）。
#      ホストの部品（package/host/*/main.js）が打つ引数は、waiting list --json・waiting answer・issue board --json・issue history --json で、
#      どれも表にある（ADR-0053 受け入れ条件 A8・ADR-0059 受け入れ条件 12）。
#   A5・A11 プロジェクトを読む葉は全部 --project を受け、決まらなければ終了コード 2。
#   A6 JSON を出せる葉（--help の「JSON: 」の行で選ぶ）は全部 --json を受け、標準出力を jq -e . で読める（決めたこと 13）。
#   A11 3 以上の終了コードは --help に数と意味が出る。フックが打つ dispatcher wait・rewake は知らない引数でも 0（決めたこと 15）。
# 道具は andon（tests/lib/andon.sh）・kanban・git・jq・perl と、偽の gh・hc。本物の hc と GitHub には出ない。
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
andon_retired_nouns "$tmp/leaves" >"$tmp/retired"
[ "$(cat "$tmp/retired")" = waiting ] && pass "名詞だけの形が古い名前の名詞は waiting だけ（andon waiting は andon waiting list と断られる）" \
  || fail "名詞だけの形が古い名前の名詞: [$(tr '\n' ' ' <"$tmp/retired")]"

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
bad=$(andon_bad_refs "$tmp/leaves" "$tmp/retired" <"$tmp/refs")
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
名前を変えた葉の古い名前 `andon init sample`・`andon delegate show --json`・`andon waiting --json`・`andon measure` を書き戻す。
```
andon  issue   submit S-0012
"andon" 'metrics' shw
/opt/andon/bin/andon issue stat S-0012
```
句読点のつく `andon issue sumbit,` と `andon issue revue.` も見る。
用語集の行 `andon task` <!-- andon の古い名前 -->
通る: `andon <名詞> <動詞>`・`andon open sample-site`・`andon gate S-0012`・`andon waiting list --json`・`andon waiting answer S-0012#1`・`andon issue`・`andon issue start <id>`
地の文の andon chief-engineer は見ない（`` の外）。
EOF
printf '%s\n' '| `andon chief-engineer "<タイトル>"` | 入れ子の置き場 |' >"$tmp/pkg/skills/x/playbooks/deep.md"
printf '%s\n' '#!/bin/sh' 'out=$(andon dispatcher drian 2>/dev/null)' >"$tmp/pkg/hooks/stop"
{ andon_refs md "$tmp/pkg/skills/x/SKILL.md" "$tmp/pkg/skills/x/playbooks/deep.md"; andon_refs code "$tmp/pkg/hooks/stop"; } >"$tmp/refs.bad"
got=$(andon_bad_refs "$tmp/leaves" "$tmp/retired" <"$tmp/refs.bad" | sed 's/^.*: andon /andon /' | LC_ALL=C sort | tr '\n' '|')
want='andon chief-engineer|andon delegate show|andon dispatcher drian|andon init sample|andon issue recrd|andon issue revue|andon issue stat|andon issue submit|andon issue sumbit|andon measure|andon metrics shw|andon task start|andon waiting|'
[ "$got" = "$want" ] && pass "書き戻した古い名前・誤った綴り・表に無い動詞は、引用符・空白・パス・句読点・入れ子・フックの中でも落ち、印の行と名指しの形は通る" \
  || fail "変種: got=[$got] want=[$want]"

# ホストの部品が打つ引数（引数を配列で渡すので、上の読み方では見えない。決めたこと 21）
# 拾うのは配列の頭の文字列の引数を3つまで（変数が来たらそこで止める）。
host_args() { perl -ne 'while (/exec\("andon", \[\s*"([^"]+)"(?:,\s*"([^"]+)")?(?:,\s*"([^"]+)")?/g) { print join(" ", grep { defined } $1, $2, $3) . "\n" }' "$@" | LC_ALL=C sort -u; }
host=$(host_args package/host/*/main.js)
if [ "$host" = "$(printf 'issue board --json\nissue history --json\nwaiting answer\nwaiting list --json')" ] && grep -qx 'waiting list' "$tmp/leaves" && grep -qx 'waiting answer' "$tmp/leaves" \
  && grep -qx 'issue board' "$tmp/leaves" && grep -qx 'issue history' "$tmp/leaves"; then
  pass "ホストの部品が打つのは andon waiting list --json・waiting answer・issue board --json・issue history --json で、どれも表にある"
else fail "ホストの部品の引数: [$host]"; fi
# 引数の拾い方が何も見ていなくても通らないこと: 表に無い葉や書く葉、古い名前の引数を足した写しは、一覧が変わる
printf '%s\n' 'exec("andon", ["issue", "done", id], { cwd });' 'exec("andon", ["issue", "board", "--json"], { cwd });' 'exec("andon", ["waiting", "--json", "--project", slug], { on });' >"$tmp/host.js"
extra=$(host_args "$tmp/host.js" | tr '\n' '|')
[ "$extra" = 'issue board --json|issue done|waiting --json --project|' ] && pass "ホストの部品の引数を拾う式は、足した呼び出しと古い名前の引数を拾う" || fail "ホストの部品の引数を拾う式: [$extra]"

# === A5・A11 どこで打っても動く（決めたこと 3・4・15・20） ===
# 一時のセルに a と b を作り、焼いた andon と kanban を打つ。読む葉は waiting list（--project を葉が自分で読む）と
# issue show（振り分けが外して KANBAN_ROOT に渡す）。どちらも kanban を読むだけ
. tests/lib/kanban.sh
export HOME="$tmp/home"
mkdir -p "$HOME"
mkcell() { # mkcell <セル> <slug>…: 各プロジェクトに課題を1件足す（タイトルは「<slug> の課題」）
  local ws=$1 s
  shift
  for s in "$@"; do
    mkdir -p "$ws/projects/$s"
    "$kanban" init --root "$ws/projects/$s" --prefix "$(printf '%s' "$s" | tr a-z A-Z)" >/dev/null \
      && KANBAN_ROOT="$ws/projects/$s" "$kanban" add "$s の課題" >/dev/null || fail "一時のセルの $s を作れない"
  done
}
two=$tmp/cell2 one=$tmp/cell1
mkcell "$two" a b
mkcell "$one" solo
mkdir -p "$two/notes" "$one/notes"

out=$(cd "$two" && ANDON_WORKSPACE=$two "$andon" waiting list --project a 2>&1)
[[ "$out" == "a の枠:"* ]] && pass "セルの根で --project a を付けると a の kanban を読む（waiting list）" || fail "根で --project a（waiting list）: $out"
out=$(cd "$two" && ANDON_WORKSPACE=$two "$andon" issue show A-0001 --project a 2>&1)
jq -e '.id == "A-0001"' >/dev/null 2>&1 <<<"$out" && pass "セルの根で --project a を付けると a の kanban を読む（issue show）" || fail "根で --project a（issue show）: $out"
out=$(cd "$two" && ANDON_WORKSPACE=$two "$andon" issue show B-0001 --project a 2>&1)
jq -e '.id' >/dev/null 2>&1 <<<"$out" && fail "--project a で b の課題が読めた: $out" || pass "--project a は b の kanban を読まない"
out=$(cd "$two/projects/a" && ANDON_WORKSPACE=$two "$andon" waiting list 2>&1)
[[ "$out" == "a の枠:"* ]] && pass "projects/a/ では --project を省いても a を読む" || fail "projects/a/ で省く: $out"
for at in "$one" "$one/notes"; do
  out=$(cd "$at" && ANDON_WORKSPACE=$one "$andon" issue show SOLO-0001 2>&1)
  jq -e '.id == "SOLO-0001"' >/dev/null 2>&1 <<<"$out" && pass "プロジェクトが1つのセルでは ${at#"$tmp"/} で省いても読む" || fail "1つのセルの ${at#"$tmp"/} で省く: $out"
done
out=$(cd "$one" && "$kanban" list 2>&1)
[[ "$out" == *"solo の課題"* ]] && pass "プロジェクトが1つのセルの根で、kanban list も同じ kanban を読む" || fail "1つのセルの根で kanban list: $out"

# 表の欄で選ぶ: プロジェクトを読む葉は --help に「プロジェクト: --project <slug>」の行を持つ（go test ./internal/cli が欄と突き合わせる）。
# dispatcher の葉は打たない（省くと全部を回る常駐と、フック。フックは決まらなくても 2 を返さないことを go test ./internal/cli が見る）
: >"$tmp/project-leaves"
while IFS= read -r l; do
  # shellcheck disable=SC2086 # 葉の名前は空白で区切った語
  h=$("$andon" $l --help)
  [[ "$h" == *$'\nプロジェクト: --project <slug>'* ]] && echo "$l" >>"$tmp/project-leaves"
done <"$tmp/leaves"
np=$(wc -l <"$tmp/project-leaves")
grep -qx 'issue show' "$tmp/project-leaves" && grep -qx 'waiting list' "$tmp/project-leaves" && ! grep -qx 'schema check' "$tmp/project-leaves" && [ "$np" -ge 30 ] \
  && pass "プロジェクトを読む葉 $np を --help の行から選ぶ" || fail "プロジェクトを読む葉が選べない（$np）"
bad=
while IFS= read -r l; do
  case "$l" in dispatcher | dispatcher\ *) continue ;; esac
  # shellcheck disable=SC2086 # 葉の名前は空白で区切った語
  err=$(cd "$two" && ANDON_WORKSPACE=$two "$andon" $l </dev/null 2>&1 >/dev/null); rc=$?
  # shellcheck disable=SC2086
  err2=$(cd "$two" && ANDON_WORKSPACE=$two "$andon" $l --project nosuch </dev/null 2>&1 >/dev/null); rc2=$?
  if [ "$l" = "issue stop-check" ]; then # Stop フックが打つ葉は 2 を返さない（2 はフックの合図）
    [ "$rc" = 0 ] && [ "$rc2" = 0 ] || bad+="$l: フックなのに $rc・$rc2"$'\n'
    continue
  fi
  [ "$rc" = 2 ] && [[ "$err" == *"プロジェクトが決まらない"* && "$err" == *"セルのプロジェクト: a・b"* && "$err" == *"andon $l --project <slug>"* ]] \
    || bad+="$l: 2つのセルの根で省いて $rc: ${err%%$'\n'*}"$'\n'
  [ "$rc2" = 2 ] && [[ "$err2" == *"プロジェクト nosuch は無い"* ]] || bad+="$l --project nosuch: $rc2: ${err2%%$'\n'*}"$'\n'
done <"$tmp/project-leaves"
[ -z "$bad" ] && pass "プロジェクトを読む葉は全部 --project を受け、2つのセルの根で省くと終了コード 2 と2つの slug を出す" \
  || fail "プロジェクトを読む葉:"$'\n'"$bad"

# === A6 JSON を出せる葉は全部 --json を受け、標準出力が JSON として読める（決めたこと 13） ===
# 表の欄で選ぶ: JSON を出せる葉は --help に「JSON: 」の行を持つ（go test ./internal/cli が欄と突き合わせる）。
# 一時のセルに、PR の課題（report.json と、マージした PR を返す偽の gh）・起動する課題（予約キーを Chief of Staff の端末が書いた）・
# 起動し直す課題（doing で端末が素のシェル、セッションの記録あり。偽の hc）を作り、葉ごとに --json を付けて打つ。
: >"$tmp/json-leaves"
while IFS= read -r l; do
  # shellcheck disable=SC2086 # 葉の名前は空白で区切った語
  h=$("$andon" $l --help)
  [[ "$h" == *$'\nJSON: '* ]] && echo "$l" >>"$tmp/json-leaves"
done <"$tmp/leaves"
nj=$(wc -l <"$tmp/json-leaves")
grep -qx 'gate' "$tmp/json-leaves" && grep -qx 'issue start' "$tmp/json-leaves" && grep -qx 'waiting list' "$tmp/json-leaves" \
  && ! grep -qx 'issue finish' "$tmp/json-leaves" && [ "$nj" -ge 17 ] \
  && pass "JSON を出せる葉 $nj を --help の行から選ぶ" || fail "JSON を出せる葉が選べない（$nj）"

jc=$tmp/cell3 jp=$tmp/cell3/projects/j
mkdir -p "$tmp/jbin" "$tmp/ghq" "$jp" "$HOME/.claude/projects/-x"
export GHQ_ROOT=$tmp/ghq GIT_CONFIG_GLOBAL=$tmp/gitconfig GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
git init -q -b main "$tmp/origin" && git -C "$tmp/origin" commit -q --allow-empty -m base && echo a >"$tmp/origin/a.md" \
  && git -C "$tmp/origin" add -A && git -C "$tmp/origin" commit -qm pr && git clone -q "$tmp/origin" "$GHQ_ROOT/github.com/example/app" \
  || fail "案件 repo の origin と clone を作れない"
sha=$(git -C "$tmp/origin" rev-parse HEAD)
cat >"$tmp/jbin/gh" <<EOF
#!/bin/sh
case "\$*" in
  "pr view https://github.com/example/app/pull/1 --json state,mergeCommit,files") echo '{"state":"MERGED","mergeCommit":{"oid":"$sha"},"files":[{"path":"a.md"}]}' ;;
  "api --paginate --slurp repos/example/app/pulls/1/files") echo '[[{"filename":"a.md"}]]' ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
cat >"$tmp/jbin/hc" <<'EOF'
#!/bin/sh
case $1 in
  templates) echo '{"templates":[{"name":"claude"},{"name":"claude-resume"}]}' ;;
  get) echo '{"state":"live","argv":["/bin/bash"],"foreground":{"argv":["/bin/bash"]},"labels":{}}' ;;
  ls) echo '{"terminals":[]}' ;;
  *) echo "unexpected hc $*" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/jbin/gh" "$tmp/jbin/hc"
sess=11111111-2222-4333-8444-555555555555
jk() { KANBAN_ROOT=$jp HC_TERMINAL_ID=t1 "$kanban" "$@" >/dev/null; } # Chief of Staff（t1）の端末で打つ
"$kanban" init --root "$jp" --prefix J --repo github.com/example/app >/dev/null && jk config set chief_of_staff=t1 \
  && jk add "PR の課題" --repo github.com/example/app && mkdir -p "$jp/kanban/issues/J-0001/library" \
  && printf '{"commit":"%s","pr":"https://github.com/example/app/pull/1"}\n' "$sha" >"$jp/kanban/issues/J-0001/library/report.json" \
  && jk add "起動する課題" --profile claude --brief - <<<'# 起動する課題' && jk set J-0002 review=off estimate=small must_fix=major merge=person \
  && jk add "起動し直す課題" --profile claude --brief - <<<'# 起動し直す課題' && jk set J-0003 status=doing agent=t9 "session=$sess" \
  && : >"$HOME/.claude/projects/-x/$sess.jsonl" || fail "JSON の葉を打つ一時のセルを作れない"
bad= seen=0
while IFS= read -r l; do
  case "$l" in # 葉ごとの引数（--json と --project は下で足す）。JSON の葉を足したら、ここに引数を足す
    version | waiting\ list | delegation\ show | metrics\ show | triage\ collect | retro\ collect | issue\ history) a=() ;;
    issue\ show | issue\ trace | issue\ impact | issue\ merge | issue\ settle) a=(J-0001) ;;
    gate) a=(J-0001 --dry-run) ;;
    issue\ board) a=(--since 2026-01-01T00:00:00Z) ;;
    issue\ start) a=(J-0002 --dry-run) ;;
    issue\ create) a=("A6 の課題" review=off estimate=small --dry-run) ;;
    issue\ resume) a=(J-0003 --dry-run) ;;
    process\ json) a=("$PWD/process/states/issue.yaml") ;;
    *) bad+="$l: 打つ引数を tests/commands.sh の A6 に足していない"$'\n'; continue ;;
  esac
  # shellcheck disable=SC2086 # 葉の名前は空白で区切った語
  h=$("$andon" $l --help)
  [[ "$h" == *$'\nプロジェクト: '* ]] && a+=(--project j)
  # shellcheck disable=SC2086
  # 条件表（gate.json）はセルの /opt/andon でなく repo の package から読む（CI と切り離した検査にはセルのパッケージが無い）
  out=$(cd "$jc" && PATH="$tmp/jbin:$PATH" ANDON_HOME="$andon_root/package" ANDON_WORKSPACE=$jc CLAUDE_CONFIG_DIR= "$andon" $l "${a[@]}" --json </dev/null 2>"$tmp/json-err")
  rc=$?
  if [[ "$out" == [\{\[]* ]] && jq -e . >/dev/null 2>&1 <<<"$out"; then seen=$((seen + 1))
  else bad+="$l --json: 終了コード $rc、標準出力が JSON でない: $(sed -n 1p "$tmp/json-err")"$'\n'; fi
  # --dry-run と組む葉は、--dry-run の無い --json を使い方の誤りとして断る
  if [[ "$h" == *$'\nJSON: --dry-run --json'* ]]; then
    # shellcheck disable=SC2086
    (cd "$jc" && PATH="$tmp/jbin:$PATH" ANDON_HOME="$andon_root/package" ANDON_WORKSPACE=$jc "$andon" $l "${a[0]}" --project j --json </dev/null >/dev/null 2>&1)
    rc=$?
    [ "$rc" = 2 ] || bad+="$l --json（--dry-run 無し）: 終了コード $rc"$'\n'
  fi
done <"$tmp/json-leaves"
[ -z "$bad" ] && [ "$seen" = "$nj" ] && pass "JSON を出せる葉 $seen が全部 --json を受け、標準出力を jq -e . で読める" \
  || fail "JSON を出せる葉（$seen/$nj）:"$'\n'"$bad"
# 読み方が何も見ていなくても通らないこと: JSON でない出力と、JSON を出せない葉の --json は落ちる
out=$(cd "$jc" && ANDON_WORKSPACE=$jc "$andon" issue finish J-0001 --project j --json 2>&1); rc=$?
[ "$rc" = 2 ] && [[ "$out" == usage:* ]] && ! jq -e . >/dev/null 2>&1 <<<'andon issue: 起動した' \
  && pass "JSON でない出力は jq -e . で読めない。JSON を出せない葉の --json は使い方の誤りで断られる" || fail "JSON の読み方が何も見ていない: $rc $out"

# === A11 3 以上の終了コードは --help に出る。フックの wait と rewake は知らない引数でも 0（決めたこと 15） ===
# 焼いた andon の --help に出る 3 以上の数を、葉ごとに控えと比べる。コードの定数（exit… = N）と表の突き合わせは go test の TestExitsMatchCode
got=
while IFS= read -r l; do
  # shellcheck disable=SC2086 # 葉の名前は空白で区切った語
  line=$("$andon" $l --help | sed -n 's/^終了コード: //p')
  codes=$(sed 's/・/\n/g' <<<"$line" | sed -n 's/^\([3-9]\|[1-9][0-9][0-9]*\) ..*/\1/p' | tr '\n' ',')
  if [ -n "$codes" ]; then got+="$l:$codes|"; fi
done <"$tmp/leaves"
want='open:3,|design start:3,|issue start:3,|issue impact:4,|gate:3,4,5,6,|triage collect:4,|'
[ "$got" = "$want" ] && pass "3 以上の終了コードを使う葉（open・design start・issue start・issue impact・gate・triage collect）は --help にその数と意味を出す" \
  || fail "--help の 3 以上の終了コード: got=[$got] want=[$want]"
h=$("$andon" schema check --help)
grep -qx '終了コード: 0 成功・1 合わない（理由は標準出力）・2 検査できない（使い方の誤りも）' <<<"$h" \
  && pass "schema check の 2（検査できない）は --help に意味が出る" || fail "schema check の --help の終了コード"
for v in wait rewake; do
  (cd "$two" && ANDON_WORKSPACE=$two HC_TERMINAL_ID= "$andon" dispatcher "$v" --bogus </dev/null >/dev/null 2>&1)
  rc=$?
  [ "$rc" = 0 ] && pass "dispatcher $v は知らない引数でも 0（終了コード 2 はフックの合図）" || fail "dispatcher $v --bogus: $rc"
done

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
