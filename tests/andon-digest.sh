#!/usr/bin/env bash
# `andon digest`（ADR-0044 受け入れ条件 12）。一時の kanban（ファイルを直に置く）で collect・build・send を通し、files/ に HTML ができることを見る。
# 同じ日に別の課題で作っても、前の 1 枚が残ること。ここまでは kanban の CLI・git を使わない（偽を PATH に置き、呼ばれたら落とす。git で main を読むのは repo のある課題のときだけで、internal/digest/decided_test.go が見る）。gh は空の PR の一覧を返す。
# 最後に本物の kanban の CLI で、ルーチンの例（package/skills/digest/assets/daily-digest.yaml）を写して lint・routine run・tick を通す。
# 集め方・組み立て方の細部は internal/digest/digest_test.go。ここはバイナリの入口・終了コード・置き場を見る。
# ANDON_EMIT_DIR があれば input.json・digest.json・sent.json を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
command -v jq >/dev/null || { echo "skip: jq が無い"; exit 0; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET
mkdir -p "$tmp/bin"
for name in git kanban; do
  printf '#!/bin/sh\necho "%s $*" >>"%s/called"\nexit 99\n' "$name" "$tmp" >"$tmp/bin/$name"
  chmod +x "$tmp/bin/$name"
done
printf '#!/bin/sh\necho "gh $*" >>"%s/gh"\necho "[]"\n' "$tmp" >"$tmp/bin/gh"
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"

# collect は dispatcher の記録とセッションの記録も読む。セルの本物（HOME の下）を読ませない
own_state() { export XDG_STATE_HOME=$tmp/state CLAUDE_CONFIG_DIR=$tmp/claude CODEX_HOME=$tmp/codex; }
own_state
export ANDON_WORKSPACE=$tmp/cell
p=$ANDON_WORKSPACE/projects/sample-site
k=$p/kanban
export KANBAN_ROOT=$p
put() { mkdir -p "$(dirname "$1")"; cat >"$1"; }
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
hour_ago=$(date -u -d '-1 hour' +%Y-%m-%dT%H:%M:%SZ)
printf 'version: 3\nprefix: S\nchief_of_staff: t1\n' | put "$k/config.yaml"
# 期間に Chief of Staff が決めたこと（振り返りの記録と、範囲の問いの答え）
printf 'id: S-0002\ntitle: 振り返り\nstatus: review\ncreated: "2026-11-01T00:00:00Z"\n' | put "$k/issues/S-0002/issue.yaml"
printf '{"applied_at": "%s", "complete": true, "issues": [{"pattern": "p1"}], "dropped": []}\n' "$hour_ago" | put "$k/issues/S-0002/library/retro-applied.json"
printf -- '- n: 1\n  text: どれを直すか\n  kind: scope\n  asked: "%s"\n  status: received\n  answer: c1 だけ\n  answered: "%s"\n  answeredBy: terminal:t1\n' "$hour_ago" "$hour_ago" | put "$k/issues/S-0002/questions.yaml"
# 期間にマージされた課題（dispatcher がマージ）と、ダイジェストの課題 2 つ（同じルーチン）
printf 'id: S-0001\ntitle: 返事の下書きボタン\nstatus: done\ncreated: "2026-11-01T00:00:00Z"\n' | put "$k/issues/S-0001/issue.yaml"
printf '{"commit": "1111111111111111111111111111111111111111", "pr": "https://github.com/example/sample-site/pull/1"}\n' | put "$k/issues/S-0001/library/report.json"
printf '{"result": "merged", "pr": "https://github.com/example/sample-site/pull/1", "merged_at": "%s", "gates": [{"head": "1111111111111111111111111111111111111111", "at": "%s", "tier": "B", "decision": "approve", "reasons": [], "merged": true}], "worked_minutes": 20, "sessions": 1}\n' \
  "$hour_ago" "$hour_ago" | put "$k/issues/S-0001/library/outcome.json"
for id in S-0100 S-0101; do
  printf 'id: %s\ntitle: デイリーダイジェスト\nstatus: doing\nroutine: daily-digest\nprofile: claude\ncreated: "2026-11-01T00:00:00Z"\n' "$id" | put "$k/issues/$id/issue.yaml"
done
printf '{"at":"%s","by":"dispatcher","id":"S-0001","change":{"status":"done"}}\n' "$hour_ago" | put "$k/log.jsonl"
text='{"headline": ["返事の下書きボタンが入った。", "オーナーを待っているものは無い。"],
  "themes": [{"title": "返信を速くする", "body": ["下書きのボタンを足した。"], "issues": ["S-0001"],
    "figure": {"type": "before_after", "before": ["毎回一から書く"], "after": ["下書きを直して送る"]}}], "concerns": []}'

# 1 枚目: collect → build → send
"$andon" digest collect S-0100 >"$tmp/out"
d=$k/issues/S-0100/library/digest
[[ "$(cat "$tmp/out")" == "$d/input.json" ]]
jq -e '.issue == "S-0100" and .previous == null and .truncated == false and ([.issues[].id] == ["S-0001"]) and .issues[0].merged_by == "dispatcher"' "$d/input.json" >/dev/null
emit "$d/input.json" digest-input.json
jq -e '.decided.decisions == [] and [.decided.retro[].issue] == ["S-0002"] and [.decided.answers[].answer] == ["c1 だけ"]' "$d/input.json" >/dev/null
printf '%s\n' "$text" | "$andon" digest build S-0100 - >"$tmp/out"
grep -q 'c1 だけ' "$d/digest.html"
grep -q '振り返りで 1 件を直すと決め、0 件を捨てた' "$d/digest.html"
[ "$(wc -l <"$tmp/out")" = 3 ] && [ -f "$d/digest.json" ] && [ -f "$d/digest.html" ] && [ -f "$d/message.txt" ]
emit "$d/digest.json" digest.json
"$andon" digest send S-0100 >"$tmp/url1"
url1=$(cat "$tmp/url1")
f1=$ANDON_WORKSPACE/files/${url1##*/}
[[ "$url1" == http://files.*.localhost:*/*-digest-sample-site-S-0100.html ]]
cmp -s "$f1" "$d/digest.html"
grep -q '<svg' "$f1" && ! grep -qi '<script' "$f1"
grep -qF "$url1" "$d/message.txt"
jq -e --arg u "$url1" '.destinations == [.destinations[0]] and .destinations[0].ok and .destinations[0].url == $u' "$d/sent.json" >/dev/null
emit "$d/sent.json" digest-sent.json
echo "ok collect・build・send で files/ に HTML ができ、URL を出して sent.json に書く"

# 2 枚目: 同じ日に別の課題で作っても、前の 1 枚が残る。期間は 1 枚目の終わりから
"$andon" digest collect S-0101 >/dev/null
d2=$k/issues/S-0101/library/digest
jq -e --arg u "$(jq -r .until "$d/input.json")" '.previous.issue == "S-0100" and .since == $u' "$d2/input.json" >/dev/null
emit "$d2/input.json" digest-input-previous.json
# 期間が 1 枚目の終わりからなので、S-0001 は入力に無く、指せば断る（終了コード 1・何も書かない）
set +e
printf '%s\n' "$text" | "$andon" digest build S-0101 - >/dev/null 2>"$tmp/err"; rc=$?
set -e
[ "$rc" = 1 ] && grep -q '入力に無い課題 S-0001' "$tmp/err"
[[ "$(ls "$d2")" == "input.json" ]]
printf '%s\n' '{"headline": ["動きは無かった。", "待っているものも無い。"], "themes": [], "concerns": []}' | "$andon" digest build S-0101 - >/dev/null
url2=$("$andon" digest send S-0101)
[[ "$url2" != "$url1" ]] && [ -f "$ANDON_WORKSPACE/files/${url2##*/}" ] && cmp -s "$f1" "$d/digest.html"
echo "ok 同じ日に別の課題で作っても前の 1 枚が残り、期間は前に送ったものの終わりから"

# 済んだ送り先には打ち直しても送らない
rm "$f1"
[[ "$("$andon" digest send S-0100)" == "$url1" ]] && [ ! -e "$f1" ]
echo "ok 済んだ送り先には打ち直しても送らない"

# 使い方の誤りは終了コード 2、無い課題は 1
set +e
"$andon" digest >/dev/null 2>&1; rc1=$?
"$andon" digest build S-0100 >/dev/null 2>&1; rc2=$?
"$andon" digest collect S-0999 >/dev/null 2>"$tmp/err"; rc3=$?
set -e
[ "$rc1" = 2 ] && [ "$rc2" = 2 ] && [ "$rc3" = 1 ] && grep -q 'S-0999 が kanban に無い' "$tmp/err"
echo "ok 使い方の誤りは 2、無い課題は 1"

[ ! -e "$tmp/called" ] || { echo "FAIL git・kanban を呼んだ: $(cat "$tmp/called")"; exit 1; }
# 上の確かめが当たる場合があること: 偽の git を呼べば印が残る
git probe >/dev/null 2>&1 || true
grep -q '^git probe$' "$tmp/called"
grep -q '^gh pr list' "$tmp/gh"
echo "ok repo の無い kanban では git も kanban の CLI も呼ばない（GitHub は gh pr list で読む）"

# ルーチンの例（受け入れ条件 12 のルーチンの部分）。ここからは本物の kanban の CLI を使うので、偽の git・kanban を外す。kanban は別の一時の置き場。
die() { echo "FAIL $*" >&2; exit 1; }
rm "$tmp/bin/git" "$tmp/bin/kanban"
hash -r
. "$(dirname "$0")/lib/kanban.sh"
own_state # kanban.sh が読む isolate.sh が置き場を外すので、もう一度向ける
skill=$andon_root/package/skills/digest
ex=$skill/assets/daily-digest.yaml
[ -f "$ex" ] || die "ルーチンの例が無い: package/skills/digest/assets/daily-digest.yaml"
export ANDON_WORKSPACE=$tmp/cell2
r=$ANDON_WORKSPACE/projects/sample-site
export KANBAN_ROOT=$r
mkdir -p "$r"
kanban init --root "$r" --prefix S >/dev/null
printf 'timezone: Asia/Tokyo\n' >>"$r/kanban/config.yaml"
mkdir -p "$r/kanban/routines"
cp "$ex" "$r/kanban/routines/daily-digest.yaml"
kanban lint >"$tmp/lint" 2>&1 || die "ルーチンの例が kanban lint に通らない: $(cat "$tmp/lint")"
# lint が当たる場合があること: schedule を壊した写しでは落ちる
sed 's/^schedule: .*/schedule: "61 7 * * *"/' "$ex" >"$r/kanban/routines/daily-digest.yaml"
if kanban lint >"$tmp/lint" 2>&1; then die "schedule を壊した写しが kanban lint に通る"; fi
cp "$ex" "$r/kanban/routines/daily-digest.yaml"
echo "ok ルーチンの例を kanban に写して kanban lint が通る"

added() { kanban list --all --json | jq -r '[.[] | select(.routine == "daily-digest") | .id] | join(" ")'; }
kanban routine run daily-digest >"$tmp/out" || die "kanban routine run daily-digest が落ちた: $(cat "$tmp/out")"
first=$(added)
[[ "$first" == "S-0001" ]] || die "課題が足されない: $first（$(cat "$tmp/out")）"
kanban show "$first" --json | jq -e '.issue | .title == "デイリーダイジェスト" and .routine == "daily-digest" and .status == "todo" and .profile == "claude" and ((.repo // "") == "")' >/dev/null \
  || die "足された課題のタイトル・ルーチン・プロファイル・repo: $(kanban show "$first" --json)"
named=$(grep -oE '/opt/andon/skills/[^ ]+' "$r/kanban/issues/$first/brief.md" || true)
[[ "$named" == "/opt/andon/skills/digest/SKILL.md" ]] && [ -f "$andon_root/package/${named#/opt/andon/}" ] || die "足された課題の依頼書が skill digest を指さない: $named"
echo "ok kanban routine run daily-digest で「デイリーダイジェスト」の課題が足される"

# skill の例の JSON で collect → build → send が通り、ルーチンのタイトルが見出しと短い文の 1 行目にそのまま出る
for title in 下書きボタン 一覧の並び 検索の速さ; do kanban add "$title" >/dev/null; done
for id in S-0002 S-0003 S-0004; do kanban done "$id" >/dev/null; done
"$andon" digest collect "$first" >/dev/null || die "ルーチンが足した課題で collect が落ちた"
awk '/^andon digest build .*<<.JSON.$/ {on = 1; next} /^JSON$/ {on = 0} on' "$skill/SKILL.md" >"$tmp/example.json"
[ -s "$tmp/example.json" ] || die "skill digest に andon digest build へ渡す JSON の例が無い"
"$andon" digest build "$first" "$tmp/example.json" >/dev/null 2>"$tmp/err" || die "skill の例の JSON が build に通らない: $(cat "$tmp/err")"
"$andon" digest send "$first" >/dev/null || die "ルーチンが足した課題で send が落ちた"
d3=$r/kanban/issues/$first/library/digest
grep -qF '<h1>デイリーダイジェスト' "$d3/digest.html" || die "HTML の見出しにルーチンのタイトルが出ない"
[[ "$(sed -n 1p "$d3/message.txt")" == "デイリーダイジェスト（sample-site）" ]] || die "短い文の 1 行目: $(sed -n 1p "$d3/message.txt")"
echo "ok skill の例の JSON で 1 枚ができ、ルーチンのタイトルが見出しと短い文の 1 行目に出る"

# 片付くまで次の課題は足されない（open: 1）。片付ければ足され、期間は前に送った 1 枚の終わりから
kanban routine run daily-digest >"$tmp/out"
[[ "$(added)" == "$first" ]] && grep -q 'skipped (open)' "$tmp/out" || die "片付いていないのに 2 件目が足された: $(added)"
kanban done "$first" >/dev/null
kanban routine run daily-digest >/dev/null
second=$(added); second=${second##* }
[[ "$second" == "S-0005" ]] || die "片付けたのに次の課題が足されない: $(added)"
"$andon" digest collect "$second" >/dev/null
jq -e --arg id "$first" --arg u "$(jq -r .until "$d3/input.json")" '.previous.issue == $id and .since == $u' "$r/kanban/issues/$second/library/digest/input.json" >/dev/null \
  || die "期間が前に送った 1 枚の終わりからでない"
echo "ok 片付くまで次の課題は足されず、片付ければ足され、期間は前に送った 1 枚の終わりから"

# 毎朝 7:37（kanban の timezone）に 1 件。7:36 には足さない。paused: true で止まる
kanban done "$second" >/dev/null
count() { added | wc -w | tr -d ' '; }
jq -n '{"daily-digest": "2030-01-01T00:00:00Z"}' >"$r/kanban/state/routines.json"
DECK_NOW=2030-01-01T22:36:00Z kanban tick >/dev/null
[ "$(count)" = 2 ] || die "7:36 に足された"
DECK_NOW=2030-01-01T22:37:00Z kanban tick >/dev/null
[ "$(count)" = 3 ] || die "7:37（Asia/Tokyo）に足されない"
third=$(added); third=${third##* }
kanban done "$third" >/dev/null
echo 'paused: true' >>"$r/kanban/routines/daily-digest.yaml"
DECK_NOW=2030-01-02T22:37:00Z kanban tick >/dev/null
kanban routine run daily-digest >"$tmp/out"
[ "$(count)" = 3 ] && grep -q 'skipped (paused)' "$tmp/out" || die "paused なのに足された: $(cat "$tmp/out")"
echo "ok 毎朝 7:37（kanban の timezone）に足され、paused: true で止まる"
