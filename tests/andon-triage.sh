#!/usr/bin/env bash
# `andon triage`。本物の台帳の CLI（kanban）・一時の台帳と HOME で回す。hc の端末も GitHub も使わない。
# 台帳の CLI は tests/lib/kanban.sh が go build で焼く（go が無ければ skip）。
set -euo pipefail
for c in git jq sha1sum; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
script=$andon
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
# auto memory の置き場（<ANDON_WORKSPACE>/memory）は、memory の節まで作らない（無くても collect は落ちない）。
export ANDON_WORKSPACE="$tmp/ws"
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET KANBAN_ROOT
mkdir -p "$HOME" "$tmp/demo" "$tmp/bin"
# Optional terminal inventory is empty; gh is a tripwire.
printf '#!/bin/sh\necho '\''{"terminals":[]}'\''\n' > "$tmp/bin/hc"
printf '#!/bin/sh\necho forbidden-gh >&2\nexit 99\n' > "$tmp/bin/gh"
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH"
cd "$tmp/demo"
kanban init --prefix T --repo example.invalid/me/app >/dev/null
export KANBAN_ROOT=$PWD
for n in one two three; do
    issue=$(kanban add "$n")
    kanban report "$issue" <<REPORT
## Report
見送り（minor）:
- $n の修正
## Next
- $n の次
## Remember
- $n の学び
For AGENTS.md: $n の規則
## 人に聞くこと
- $n の問い
REPORT
    kanban done "$issue" >/dev/null
done
assigned=$(kanban add '担当済み')
kanban set "$assigned" agent=t99 >/dev/null
todo=$(kanban add '未担当')
person=$(kanban add '端末で確認（人の判断待ち）')
kanban set "$person" who=person >/dev/null
mkdir "$tmp/leftover"
kanban set T-1 "worktree=$tmp/leftover" >/dev/null
broken=$(kanban add broken); kanban report "$broken" <<<'見出しなし'; kanban done "$broken" >/dev/null
"$script" triage collect > "$tmp/items"
jq -e '([.[].kind] | unique) == ["agents","issue","leftover","next","person","question","remember","unreadable","waived"]' "$tmp/items" >/dev/null
echo 'ok collect: eight kinds and unreadable'
set +e
"$script" triage collect --limit 1 >/dev/null 2>&1; rc=$?
set -e
[[ $rc == 4 ]]
echo 'ok limit: exit 4'
triage=$(kanban add triage)
mkdir -p "kanban/issues/$triage/library"
file=kanban/issues/$triage/library/decisions.json
jq --arg todo "$todo" --arg person "$person" '
 def issue($title): {title:($title|ltrimstr("- ")),repo:"example.invalid/me/app",profile:"claude",review:"on",brief:"解決すべき問題: 修正\n完了条件: 検査を通す"};
 {goal:"テスト",collected:"2026-11-03T00:00:00Z",items:
  [(.[]|select(.from==$todo and .kind=="issue")|{key,decision:"close",reason:"不要"}),
   (.[]|select(.from==$person and .kind=="person")|{key,decision:"close",reason:"済み"}),
   (.[]|select(.kind=="next")|{key,decision:"fix",reason:"直せる",issue:issue(.text)}),
   (.[]|select(.kind=="agents")|{key,decision:"bundle",bundle:"docs",reason:"同じrepo",issue:issue(.text)}),
   (.[]|select(.kind=="question")|{key,decision:"person",reason:"人が必要",question:(.text|ltrimstr("- ")),kind:"promise",pick:"確認する"}),
   (.[]|select(.kind=="unreadable")|{key,decision:"keep",reason:"読む"})]}' "$tmp/items" > "$file"
# Put bundle first so max-issues 2 must create one bundle and one fix.
jq '.items |= sort_by(if .decision=="bundle" then 0 else 1 end)' "$file" > "$tmp/sorted"; cp "$tmp/sorted" "$file"
snapshot() { find kanban -type f -exec sha1sum {} + | sort; }
snapshot > "$tmp/before"
"$script" triage apply "$triage" --max-issues 2 --dry-run > "$tmp/plan"
snapshot > "$tmp/after"
cmp "$tmp/before" "$tmp/after"
jq -e '[.items[]|select(.decision=="keep")]|length==3' "$tmp/plan" >/dev/null
echo 'ok dry-run unchanged; cap defers two fixes'
cp "$file" "$tmp/good"
jq '.items[-1].reason=" "' "$tmp/good" > "$file"
set +e
"$script" triage apply "$triage" >/dev/null 2>&1; rc=$?
set -e
[[ $rc == 1 ]]
cp "$tmp/good" "$file"
snapshot > "$tmp/after"; cmp "$tmp/before" "$tmp/after"
echo 'ok invalid reason fails before mutation'
for filter in '.items[0].issue.title=""' '.items[0].issue.repo="example.invalid/other/repo"' '.items[0].key="other:T-1:issue"'; do
    jq "$filter" "$tmp/good" > "$file"
    set +e
    "$script" triage apply "$triage" >/dev/null 2>&1; rc=$?
    set -e
    [[ $rc == 1 ]]
done
cp "$tmp/good" "$file"
snapshot > "$tmp/after"; cmp "$tmp/before" "$tmp/after"
echo 'ok missing title, mixed-repo bundle and foreign key rejected before mutation'
assigned_hash=$(printf '%s' '担当済み' | sha1sum); assigned_hash=${assigned_hash%% *}
for bad_key in "demo:$assigned:issue" "demo:$assigned:issue:$assigned_hash" "demo:T-1:next:0000000000000000000000000000000000000000"; do
    jq --arg key "$bad_key" '.items += [{key:$key,decision:"close",reason:"typo"}]' "$tmp/good" > "$file"
    set +e
    "$script" triage apply "$triage" >/dev/null 2>&1; rc=$?
    set -e
    [[ $rc == 1 ]]
done
cp "$tmp/good" "$file"
snapshot > "$tmp/after"; cmp "$tmp/before" "$tmp/after"
echo 'ok absent hash, assigned issue and uncollected key rejected without writes'
"$script" triage apply "$triage" --max-issues 2 > "$tmp/summary"
[[ $(kanban show "$todo" --json | jq -r .issue.status) == done ]]
tail -1 "kanban/issues/$todo/report.md" | grep -q '不要（'
record=$(echo kanban/library/triage/*.json)
jq -e '[.items[]|select(.decision=="bundle")|.created_issue]|unique|length==1' "$record" >/dev/null
while read -r issue; do
    [[ $(kanban show "$issue" --json | jq -r .issue.who) == person ]]
done < <(jq -r '.items[]|select(.decision=="person")|.created_issue' "$record")
grep -q '閉じた 2 ・ 直す 2（うち束ね 1） ・ 人へ 3 ・ 次回 3 ・ 読めない 1' "$tmp/summary"
echo 'ok apply: close, bundle, person, summary'
"$script" triage collect --limit 100 > "$tmp/again"
jq -e --slurpfile r "$record" '
 [.[].key] as $keys | all($r[0].items[]|select(.decision!="keep"); .key as $k | ($keys|index($k))==null)
' "$tmp/again" >/dev/null
jq -e --slurpfile r "$record" '
 [.[].key] as $keys | all($r[0].items[]|select(.decision=="keep"); .key as $k | ($keys|index($k))!=null)
' "$tmp/again" >/dev/null
echo 'ok recorded keys excluded, keep returns'
set +e
"$script" triage apply "$triage" >/dev/null 2>&1; rc=$?
set -e
[[ $rc == 1 ]]
echo 'ok repeated apply refused'
# Recorded keys can be revisited, but text still comes from the original source.
revisit=$(kanban add revisit)
mkdir -p "kanban/issues/$revisit/library"
jq '{goal:"戻す",collected,items:[(.items[]|select(.decision=="bundle"))][0:1]} | .items[0] += {decision:"keep",reason:"再検討",text:"spoofed text"}' "$record" > "kanban/issues/$revisit/library/decisions.json"
"$script" triage apply "$revisit" --dry-run > "$tmp/revisit"
jq -e '.items[0].text != "spoofed text"' "$tmp/revisit" >/dev/null
echo 'ok recorded key can be revisited; decisions text is not trusted'
# kind: memory（ADR-0020）。置き場の直下の .md が1件1項目。目録（MEMORY.md）は項目にしない。
mem=$ANDON_WORKSPACE/memory
mkdir -p "$mem"
"$script" triage collect --limit 100 > "$tmp/mem0"
jq -e 'all(.[]; .kind != "memory")' "$tmp/mem0" >/dev/null
echo 'ok memory: empty folder gives no items'
printf -- '---\nname: a\n---\nテストは go test で回す。\n' > "$mem/feedback_a.md"
printf -- 'b の注意。理由: 古い\n' > "$mem/project_b.md"
printf -- 'c は次回\n' > "$mem/note c.md"
printf -- '- [A](feedback_a.md) — a\n- [C](note c.md) — c\n- [X](other.md) — x\n' > "$mem/MEMORY.md"
"$script" triage collect --limit 100 > "$tmp/mem1"
jq -e '[.[]|select(.kind=="memory")] | length==3 and (map(.key)|unique|length)==3 and all(.text|length>0) and all(.repo=="")' "$tmp/mem1" >/dev/null
jq -e '[.[]|select(.kind=="memory").from]|sort == ["feedback_a","note-c","project_b"]' "$tmp/mem1" >/dev/null
jq -e --arg t "$(cat "$mem/feedback_a.md")" '[.[]|select(.from=="feedback_a")][0].text == ($t+"\n")' "$tmp/mem1" >/dev/null
jq -e 'all(.[]; .text|contains("[A](feedback_a.md)")|not)' "$tmp/mem1" >/dev/null
echo 'ok memory: one item per file, text is the body, index is not an item'
distill=$(kanban add distill)
mkdir -p "kanban/issues/$distill/library"
jq '{goal:"memory",collected:"2026-11-11T00:00:00Z",items:
  [(.[]|select(.from=="feedback_a")|{key,decision:"close",reason:"済み: AGENTS.md にある"}),
   (.[]|select(.from=="project_b")|{key,decision:"close",reason:"理由が当てはまらない"}),
   (.[]|select(.from=="note-c")|{key,decision:"keep",reason:"待つ: T-1"})]}' "$tmp/mem1" > "kanban/issues/$distill/library/decisions.json"
"$script" triage apply "$distill" --dry-run >/dev/null
[[ -f $mem/feedback_a.md && -f $mem/project_b.md ]]
echo 'ok memory: dry-run keeps files'
"$script" triage apply "$distill" >/dev/null
[[ ! -e $mem/feedback_a.md && ! -e $mem/project_b.md && -f "$mem/note c.md" ]]
[[ $(cat "$mem/MEMORY.md") == $'- [C](note c.md) — c\n- [X](other.md) — x' ]]
rec=$(ls kanban/library/triage/*-"$distill".json)
jq -e '[.items[]|select(.decision=="close")] | length==2 and all(.text|length>0) and all(.reason|length>0)' "$rec" >/dev/null
jq -e '[.items[]|select(.decision=="close").text]|any(contains("テストは go test で回す。"))' "$rec" >/dev/null
"$script" triage collect --limit 100 > "$tmp/mem2"
jq -e '[.[]|select(.kind=="memory").from] == ["note-c"]' "$tmp/mem2" >/dev/null
echo 'ok memory: apply removes sorted files and index lines (missing line is fine), keep stays, record has body and reason'
# 収集のあと消す前に同じファイルが書き換わったら消さない（A-0155）。apply の途中（kanban add のとき）に別のセッションが追記する。
printf -- 'race の学び\n' > "$mem/race.md"
printf -- 'gone の学び\n' > "$mem/gone.md"
printf -- '- [R](race.md) — r\n- [G](gone.md) — g\n' >> "$mem/MEMORY.md"
"$script" triage collect --limit 100 > "$tmp/mem3"
race=$(kanban add race)
mkdir -p "kanban/issues/$race/library"
jq '{goal:"memory",collected:"2026-11-11T00:00:00Z",items:
  [(.[]|select(.from=="race")|{key,decision:"fix",reason:"直せる",issue:{title:"race",repo:"example.invalid/me/app",profile:"claude",review:"off",brief:"解決すべき問題: race"}}),
   (.[]|select(.from=="gone")|{key,decision:"close",reason:"要らない"})]}' "$tmp/mem3" > "kanban/issues/$race/library/decisions.json"
mkdir -p "$tmp/race-bin"
printf '#!/bin/sh
[ "$1" = add ] && echo "書きかけの新しい学び" >> "%s/race.md"
exec "%s" "$@"
' "$mem" "$kanban" > "$tmp/race-bin/kanban"
chmod +x "$tmp/race-bin/kanban"
PATH="$tmp/race-bin:$PATH" "$script" triage apply "$race" > "$tmp/race-md"
[[ $(cat "$mem/race.md") == $'race の学び\n書きかけの新しい学び' && ! -e $mem/gone.md ]]
grep -q '(race.md)' "$mem/MEMORY.md" && ! grep -q '(gone.md)' "$mem/MEMORY.md"
rec=$(ls kanban/library/triage/*-"$race".json)
jq -e '[.items[]|select(.memory_kept)] | length==1 and .[0].decision=="fix" and (.[0].created_issue|length>0)' "$rec" >/dev/null
jq -e '[.items[]|select(.decision=="close")][0] | has("memory_kept")|not' "$rec" >/dev/null
grep -q '消さずに残した memory' "$tmp/race-md"
echo 'ok memory: a file rewritten after collect is kept with its index line, reason in the record'
echo 'all passed'
