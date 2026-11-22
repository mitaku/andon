#!/usr/bin/env bash
# `andon digest`（ADR-0044 受け入れ条件 12 のうちルーチンを除く部分）。一時の kanban（ファイルを直に置く）で collect・build・send を通し、files/ に HTML ができることを見る。
# 同じ日に別の課題で作っても、前の 1 枚が残ること。kanban の CLI・git は使わない（偽を PATH に置き、呼ばれたら落とす）。gh は空の PR の一覧を返す。
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

export ANDON_WORKSPACE=$tmp/cell
p=$ANDON_WORKSPACE/projects/sample-site
k=$p/kanban
export KANBAN_ROOT=$p
put() { mkdir -p "$(dirname "$1")"; cat >"$1"; }
now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
hour_ago=$(date -u -d '-1 hour' +%Y-%m-%dT%H:%M:%SZ)
printf 'version: 3\nprefix: S\nchief_of_staff: t1\n' | put "$k/config.yaml"
# 期間にマージされた課題（dispatcher がマージ）と、ダイジェストの課題 2 つ（同じルーチン）
printf 'id: S-0001\ntitle: 返事の下書きボタン\nstatus: done\ncreated: "2026-11-01T00:00:00Z"\n' | put "$k/issues/S-0001/issue.yaml"
printf '{"commit": "1111111111111111111111111111111111111111", "pr": "https://github.com/example/sample-site/pull/1"}\n' | put "$k/issues/S-0001/library/report.json"
printf '{"result": "merged", "pr": "https://github.com/example/sample-site/pull/1", "merged_at": "%s", "gates": [{"head": "1111111111111111111111111111111111111111", "at": "%s", "tier": "B", "decision": "approve", "reasons": [], "merged": true}], "worked_minutes": 20, "sessions": 1}\n' \
  "$hour_ago" "$hour_ago" | put "$k/issues/S-0001/library/outcome.json"
for id in S-0100 S-0101; do
  printf 'id: %s\ntitle: 日次のダイジェスト\nstatus: doing\nroutine: digest\nprofile: claude\ncreated: "2026-11-01T00:00:00Z"\n' "$id" | put "$k/issues/$id/issue.yaml"
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
printf '%s\n' "$text" | "$andon" digest build S-0100 - >"$tmp/out"
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
echo "ok git も kanban の CLI も呼ばない（GitHub は gh pr list で読む）"
