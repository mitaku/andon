#!/usr/bin/env bash
# `andon waiting list`。一時の kanban（ファイルを直に置く）で回す。kanban の CLI も hc の端末も使わない。偽の gh・git を PATH に置き、呼ばれたら落とす。
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

"$andon" waiting list --json --project sample-site >"$tmp/out.json"
jq -e '.version == 1 and .project == "sample-site"
  and .slots == {limit: 3, working: 0, waiting: 2, free: 3}
  and ([.items[] | [.kind, .id, (.question // "")]] == [["merge", "S-0001", "S-0001#1"], ["decision", "S-0003", "S-0003#1"], ["task", "S-0004", ""], ["decision", "S-0005", ""]])
  and .items[0].holds == ["S-0002"] and .items[0].pr == $pr
  and .items[1].ask.question == "CSV だけにするか" and .items[1].answerable == true
  and .items[2].reason == "ホストで image を焼き直す"
  and (.items[3] | has("ask") | not) and .items[3].answerable == false and (.items[3].reason | startswith("オーナーに聞く問いが置かれていない"))' --arg pr "$pr" "$tmp/out.json" >/dev/null
emit "$tmp/out.json" waiting.json
echo 'ok --json --project: マージの依頼・判断・手作業・問いの番号の無い判断と、枠・後ろで待つ課題・並び'

(cd "$p" && KANBAN_ROOT=$p "$andon" waiting list) >"$tmp/text"
grep -q '^sample-site の枠: 上限 3、動いている 0、オーナー待ち 2、空き 3$' "$tmp/text"
grep -q '^マージの依頼 S-0001#1（名指しの口を足す）。2026-11-21T04:43:35Z から。後ろで 1 件が待つ（S-0002）。理由: マージするか。PR https://github.com/example/sample-site/pull/41$' "$tmp/text"
grep -q '^判断 S-0003#1（書き出しの形を決める）。.*問い: CSV だけにするか$' "$tmp/text"
grep -q '^手作業 S-0004（image を焼き直す）。.*理由: ホストで image を焼き直す$' "$tmp/text"
grep -q '^判断 S-0005（公開の範囲を決める）。.*直す: 判断なら andon issue ask S-0005 -、手作業なら kanban set S-0005 only_owner="<理由>"$' "$tmp/text"
echo 'ok 人が読む行: 枠の行と、行ごとの種類・問い・後ろで待つ数・PR'

# 置いてあるもの（ADR-0062 受け入れ条件 8）: Chief of Staff（t1）が印 later を書いたオーナーの課題の判断は、
# 置いてある行として呼んでいる行の後に分かれ、印の理由が出る
lp=$tmp/later
lk=$lp/kanban
mkdir -p "$lk"
printf 'version: 3\nprefix: S\nchief_of_staff: t1\n' | put "$lk/config.yaml"
printf 'id: S-0001\ntitle: 公開の範囲を決める\nstatus: todo\nwho: person\nlater: 急がない。次のリリースで決めればよい\ncreated: "2026-11-21T01:00:00Z"\n' | put "$lk/issues/S-0001/issue.yaml"
printf -- '- n: 1\n  text: 公開するか\n  kind: users\n  status: open\n  asked: "2026-11-21T02:00:00Z"\n' | put "$lk/issues/S-0001/questions.yaml"
printf 'id: S-0002\ntitle: 書き出しの形を決める\nstatus: todo\nwho: person\ncreated: "2026-11-21T03:00:00Z"\n' | put "$lk/issues/S-0002/issue.yaml"
printf -- '- n: 1\n  text: CSV だけにするか\n  kind: users\n  status: open\n  asked: "2026-11-21T04:00:00Z"\n' | put "$lk/issues/S-0002/questions.yaml"
printf '{"at":"2026-11-21T05:00:00Z","by":"terminal:t1","id":"S-0001","change":{"later":"急がない。次のリリースで決めればよい"}}\n' | put "$lk/log.jsonl"
KANBAN_ROOT=$lp "$andon" waiting list --json >"$tmp/later.json"
jq -e '[.items[] | [.question, .call, (.later // "")]] == [["S-0002#1", true, ""], ["S-0001#1", false, "急がない。次のリリースで決めればよい"]]' "$tmp/later.json" >/dev/null \
  || die "置いてある行の call・later・並びが違う: $(cat "$tmp/later.json")"
emit "$tmp/later.json" waiting-later.json
KANBAN_ROOT=$lp "$andon" waiting list >"$tmp/later.text"
sed -n 2,5p "$tmp/later.text" >"$tmp/later.body"
printf '%s\n' '呼んでいるもの 1 件（行燈を灯す）' '^判断 S-0002#1（書き出しの形を決める）' \
  '^置いてあるもの 1 件（行燈を灯さない。いつでも答えられる）$' '^判断 S-0001#1（公開の範囲を決める）。.*。置いた理由: 急がない。次のリリースで決めればよい$' >"$tmp/later.want"
paste -d '\t' "$tmp/later.want" "$tmp/later.body" | while IFS=$'\t' read -r re line; do
  grep -q -- "$re" <<<"$line" || die "人が読む行が違う（$re）: $line"
done
[[ $(wc -l <"$tmp/later.text") == 5 ]] || die "人が読む行の数が違う: $(cat "$tmp/later.text")"
# 当たる場合があること: 印を担当の端末（t7）が書き直すと、置いてある行は無くなる
printf '{"at":"2026-11-21T06:00:00Z","by":"terminal:t7","id":"S-0001","change":{"later":"急がない。次のリリースで決めればよい"}}\n' >>"$lk/log.jsonl"
KANBAN_ROOT=$lp "$andon" waiting list >"$tmp/later2.text"
if grep -q '置いてあるもの' "$tmp/later2.text"; then die "担当の端末が書いた印を読んだ: $(cat "$tmp/later2.text")"; fi
grep -q '^呼んでいるもの 2 件' "$tmp/later2.text" || die "担当の端末が書いた印で呼ぶ行が 2 件にならない: $(cat "$tmp/later2.text")"
echo 'ok 置いてあるもの: 呼んでいる行の後に分かれ、印の理由が出る。担当の端末が書いた印は読まない'

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
KANBAN_ROOT=$tmp/empty "$andon" waiting list --json >"$tmp/empty.json"
jq -e '.items == [] and .slots == {limit: 2, working: 0, waiting: 0, free: 2}' "$tmp/empty.json" >/dev/null
emit "$tmp/empty.json" waiting-empty.json
KANBAN_ROOT=$tmp/empty "$andon" waiting list | grep -q '^オーナー待ちは無い$'
echo 'ok 空の kanban: 上限 2・空の並び・終了コード 0'

# オーナー待ちが上限以上でも空きは減らない（ADR-0065 受け入れ条件 2）: 上限 2・判断を待つ担当 2・動いている担当 1
fp=$tmp/full
fk=$fp/kanban
printf 'version: 3\nprefix: S\nchief_of_staff: t1\n' | put "$fk/config.yaml"
printf '{"version": 1, "project": {"title": "Sample", "prefix": "S"}, "repos": [], "chief_of_staff": {"parallel": 2}}\n' | put "$fp/project.json"
for id in S-0001 S-0002; do
  printf 'id: %s\ntitle: 判断を待つ\nstatus: doing\nprofile: claude\ncreated: "2026-11-21T01:00:00Z"\n' "$id" | put "$fk/issues/$id/issue.yaml"
  printf -- '- n: 1\n  text: CSV だけにするか\n  kind: users\n  status: open\n  asked: "2026-11-21T02:00:00Z"\n' | put "$fk/issues/$id/questions.yaml"
done
printf 'id: S-0003\ntitle: 動いている\nstatus: doing\nprofile: claude\ncreated: "2026-11-21T01:00:00Z"\n' | put "$fk/issues/S-0003/issue.yaml"
: | put "$fk/log.jsonl"
KANBAN_ROOT=$fp "$andon" waiting list --json >"$tmp/full.json"
jq -e '.slots == {limit: 2, working: 1, waiting: 2, free: 1}' "$tmp/full.json" >/dev/null \
  || die "待ちが上限以上の slots が違う（キーは limit・working・waiting・free の 4 つ）: $(jq -c .slots "$tmp/full.json")"
KANBAN_ROOT=$fp "$andon" waiting list >"$tmp/full.text"
grep -q '^full の枠: 上限 2、動いている 1、オーナー待ち 2、空き 1$' "$tmp/full.text" \
  || die "待ちが上限以上の枠の行が違う: $(cat "$tmp/full.text")"
if grep -q '起動しない' "$tmp/full.text"; then die "人が読む出力に起動しないの文がある: $(cat "$tmp/full.text")"; fi
# 当たる場合があること: 直す前の枠の行には上の否定の grep が当たる
grep -q '起動しない' <<<'full の枠: 上限 2、動いている 1、オーナー待ち 2、空き 0。オーナー待ちが上限に達したので新しい課題を起動しない' \
  || die "起動しないの文を見る grep が直す前の文に当たらない"
echo 'ok 待ちが上限以上: 空きは上限から動いている数を引いた数で、waiting_full も起動しないの文も無い'

set +e
"$andon" waiting list --project nothing >/dev/null 2>"$tmp/err"; rc=$?
"$andon" waiting list --bogus >/dev/null 2>&1; rc2=$?
set -e
[[ $rc == 1 ]] && grep -q 'プロジェクト nothing の kanban が無い' "$tmp/err" || die "無いプロジェクトを終了コード 1 で断らない（rc=$rc）"
[[ $rc2 == 2 ]]
echo 'ok 無いプロジェクトは終了コード 1、知らない引数は 2'
