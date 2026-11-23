#!/usr/bin/env bash
# `andon waiting`。一時の kanban（ファイルを直に置く）で回す。kanban の CLI も hc の端末も使わない。偽の gh・git を PATH に置き、呼ばれたら落とす。
# 数え方の細部は internal/waiting/waiting_test.go。ここはバイナリの入口・人が読む行・終了コード・何も書かないことを見る。
# ANDON_EMIT_DIR があれば --json の出力を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
command -v jq >/dev/null || { echo "skip: jq が無い"; exit 0; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
# set -e は A && B の A が偽でも止まらないので、並べた確かめには || die を付ける
die() { echo "FAIL $*" >&2; exit 1; }
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET
mkdir -p "$tmp/bin"
for name in gh git kanban; do
  printf '#!/bin/sh\necho "%s $*" >>"%s/called"\nexit 99\n' "$name" "$tmp" >"$tmp/bin/$name"
  chmod +x "$tmp/bin/$name"
done
export PATH="$tmp/bin:$PATH"

head=1111111111111111111111111111111111111111
pr=https://github.com/example/sample-site/pull/41
export ANDON_WORKSPACE=$tmp/cell
p=$ANDON_WORKSPACE/projects/sample-site
k=$p/kanban
put() { mkdir -p "$(dirname "$1")"; cat >"$1"; }
printf 'version: 3\nprefix: S\nchief_of_staff: t1\n' | put "$k/config.yaml"
printf '{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": [], "chief_of_staff": {"parallel": 3}}\n' | put "$p/project.json"
# マージの依頼（gate の問いが今の head について開いている。後ろで S-0002 が待つ）
printf 'id: S-0001\ntitle: 名指しの口を足す\nstatus: review\nprofile: claude\ncreated: "2026-11-21T01:00:00Z"\n' | put "$k/issues/S-0001/issue.yaml"
printf -- '- n: 1\n  text: マージするか\n  kind: line\n  status: open\n  asked: "2026-11-21T04:43:35Z"\n' | put "$k/issues/S-0001/questions.yaml"
printf '{"commit": "%s", "pr": "%s"}\n' "$head" "$pr" | put "$k/issues/S-0001/library/report.json"
printf '{"kind": "line", "question": "マージするか", "facts": ["gate の記録 %s"], "options": [{"label": "マージする", "pick": true}, {"label": "直させる"}], "from": "gate"}\n' \
  "$k/issues/S-0001/library/gate/$head.json" | put "$k/issues/S-0001/library/questions/1.json"
printf 'id: S-0002\ntitle: 後ろで待つ\nstatus: todo\nafter: [S-0001]\ncreated: "2026-11-21T01:00:00Z"\n' | put "$k/issues/S-0002/issue.yaml"
# 判断（担当の問い）と手作業（印 only_owner のあるオーナーの課題）と、問いも印も無いオーナーの課題（問いの番号の無い判断。ADR-0057）
printf 'id: S-0003\ntitle: 書き出しの形を決める\nstatus: doing\nprofile: claude\ncreated: "2026-11-21T01:00:00Z"\n' | put "$k/issues/S-0003/issue.yaml"
printf -- '- n: 1\n  text: CSV だけにするか\n  kind: users\n  status: open\n  asked: "2026-11-21T02:12:13Z"\n' | put "$k/issues/S-0003/questions.yaml"
printf '{"kind": "users", "question": "CSV だけにするか", "options": [{"label": "CSV だけ", "pick": true}, {"label": "Excel も"}], "from": "issue"}\n' \
  | put "$k/issues/S-0003/library/questions/1.json"
printf 'id: S-0004\ntitle: image を焼き直す\nstatus: todo\nwho: person\nonly_owner: ホストで image を焼き直す\ncreated: "2026-11-21T08:55:43Z"\n' | put "$k/issues/S-0004/issue.yaml"
printf 'id: S-0005\ntitle: 公開の範囲を決める\nstatus: todo\nwho: person\ncreated: "2026-11-21T09:10:00Z"\n' | put "$k/issues/S-0005/issue.yaml"
printf '{"at":"2026-11-21T04:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"review"}}\n' | put "$k/log.jsonl"
before=$(cd "$p" && find . -type f -exec sha1sum {} + | sort)

"$andon" waiting --json --project sample-site >"$tmp/out.json"
jq -e '.version == 1 and .project == "sample-site"
  and .slots == {limit: 3, working: 0, waiting: 2, free: 3, waiting_full: false}
  and ([.items[] | [.kind, .id, (.question // "")]] == [["merge", "S-0001", "S-0001#1"], ["decision", "S-0003", "S-0003#1"], ["task", "S-0004", ""], ["decision", "S-0005", ""]])
  and .items[0].holds == ["S-0002"] and .items[0].pr == $pr
  and .items[1].ask.question == "CSV だけにするか" and .items[1].answerable == true
  and .items[2].reason == "ホストで image を焼き直す"
  and (.items[3] | has("ask") | not) and .items[3].answerable == false and (.items[3].reason | startswith("オーナーに聞く問いが置かれていない"))' --arg pr "$pr" "$tmp/out.json" >/dev/null
emit "$tmp/out.json" waiting.json
echo 'ok --json --project: マージの依頼・判断・手作業・問いの番号の無い判断と、枠・後ろで待つ課題・並び'

(cd "$p" && KANBAN_ROOT=$p "$andon" waiting) >"$tmp/text"
grep -q '^sample-site の枠: 上限 3、動いている 0、オーナー待ち 2、空き 3$' "$tmp/text"
grep -q '^マージの依頼 S-0001#1（名指しの口を足す）。2026-11-21T04:43:35Z から。後ろで 1 件が待つ（S-0002）。理由: マージするか。PR https://github.com/example/sample-site/pull/41$' "$tmp/text"
grep -q '^判断 S-0003#1（書き出しの形を決める）。.*問い: CSV だけにするか$' "$tmp/text"
grep -q '^手作業 S-0004（image を焼き直す）。.*理由: ホストで image を焼き直す$' "$tmp/text"
grep -q '^判断 S-0005（公開の範囲を決める）。.*直す: 判断なら andon issue ask S-0005 -、手作業なら kanban set S-0005 only_owner="<理由>"$' "$tmp/text"
echo 'ok 人が読む行: 枠の行と、行ごとの種類・問い・後ろで待つ数・PR'

after=$(cd "$p" && find . -type f -exec sha1sum {} + | sort)
[[ "$before" == "$after" ]]
[ ! -e "$tmp/called" ] || { echo "FAIL gh・git・kanban を呼んだ: $(cat "$tmp/called")"; exit 1; }
# 上の確かめが当たる場合があること: 偽の gh を呼べば印が残る
gh probe >/dev/null 2>&1 || true
grep -q '^gh probe$' "$tmp/called"
rm -f "$tmp/called"
echo 'ok kanban に何も書かず、gh も git も kanban の CLI も呼ばない'

# オーナー待ちが無い kanban（project.json も無い）: 上限 2・空の並び・終了コード 0
mkdir -p "$tmp/empty/kanban"
printf 'version: 3\n' >"$tmp/empty/kanban/config.yaml"
KANBAN_ROOT=$tmp/empty "$andon" waiting --json >"$tmp/empty.json"
jq -e '.items == [] and .slots == {limit: 2, working: 0, waiting: 0, free: 2, waiting_full: false}' "$tmp/empty.json" >/dev/null
emit "$tmp/empty.json" waiting-empty.json
KANBAN_ROOT=$tmp/empty "$andon" waiting | grep -q '^オーナー待ちは無い$'
echo 'ok 空の kanban: 上限 2・空の並び・終了コード 0'

set +e
"$andon" waiting --project nothing >/dev/null 2>"$tmp/err"; rc=$?
"$andon" waiting --bogus >/dev/null 2>&1; rc2=$?
set -e
[[ $rc == 1 ]] && grep -q 'プロジェクト nothing の kanban が無い' "$tmp/err" || die "無いプロジェクトを終了コード 1 で断らない（rc=$rc）"
[[ $rc2 == 2 ]]
echo 'ok 無いプロジェクトは終了コード 1、知らない引数は 2'
