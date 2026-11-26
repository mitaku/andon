#!/usr/bin/env bash
# `andon metrics show`。一時の kanban（ファイルを直に置く）と偽の gh で回す。kanban の CLI も hc の端末も使わない。
# 数え方の細部は internal/measure/measure_test.go（手で数えた値と突き合わせる）。ここはバイナリの入口・表・終了コードを見る。
# ANDON_EMIT_DIR があれば --json の出力を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
command -v jq >/dev/null || { echo "skip: jq が無い"; exit 0; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET
mkdir -p "$tmp/bin"
pr=https://github.com/example/sample-site/pull
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
[ "\$*" = "pr list --repo example/sample-site --state merged --limit 1000 --json number,url,title,body,mergedAt" ] || { echo "unexpected: \$*" >&2; exit 9; }
cat <<'JSON'
[{"number": 1, "url": "$pr/1", "title": "one", "body": "", "mergedAt": "2026-11-01T06:00:00Z"},
 {"number": 3, "url": "$pr/3", "title": "Revert \"one\"", "body": "Reverts example/sample-site#1", "mergedAt": "2026-11-02T00:00:00Z"}]
JSON
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"

# 担当の課題 1 件（自動マージして revert された。owner の答え 2・Chief of Staff の答え 1・印の無い答え 1）と、
# 問いを持たずに片付いたオーナーの課題 1 件と、壊れた log.jsonl の行
k=$tmp/sample-site/kanban
mkdir -p "$k/issues/S-0001/library/gate" "$k/issues/S-0002"
printf 'id: S-0002\ntitle: two\nstatus: done\nwho: person\ncreated: "2026-11-01T00:00:00Z"\n' >"$k/issues/S-0002/issue.yaml"
n=0
for mark in owner owner chief-of-staff ''; do
  n=$((n + 1))
  printf -- '- n: %d\n  text: x\n  status: received\n  asked: "2026-11-01T01:30:00Z"\n  answered: "2026-11-01T01:40:00Z"\n' "$n"
  if [[ -n "$mark" ]]; then printf '  decidedBy: %s\n' "$mark"; fi
done >"$k/issues/S-0001/questions.yaml"
printf 'version: 3\nprefix: S\n' >"$k/config.yaml"
printf 'id: S-0001\ntitle: one\nstatus: done\ncreated: "2026-11-01T00:00:00Z"\nsession: 11111111-0000-4000-8000-000000000001\n' >"$k/issues/S-0001/issue.yaml"
printf '{"pr": "%s/1"}\n' "$pr" >"$k/issues/S-0001/library/report.json"
printf '{"at": "2026-11-01T06:00:00Z", "id": "S-0001", "pr": "%s/1", "tier": "A", "decision": "approve", "merged": true}\n' "$pr" \
  >"$k/issues/S-0001/library/gate/aaaa.json"
cat >"$k/log.jsonl" <<'EOF'
{"at":"2026-11-01T00:00:00Z","by":"terminal:t1","id":"S-0001","change":{"add":"one"}}
{"at":"2026-11-01T01:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"doing"}}
{"at":"2026-11-01T02:00:00Z","by":"someone","id":"S-0001","change":{"merge":"auto"}}
{"at":"broken
{"at":"2026-11-01T07:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"done"}}
{"at":"2026-11-01T08:00:00Z","by":"terminal:t1","id":"S-0002","change":{"status":"done"}}
EOF
# dispatcher の置き場（記録だけの通知・届けた通知の記録）は一時の XDG_STATE_HOME に置く: report を2回届けて2回とも ack、merged を1回記録だけ
export XDG_STATE_HOME=$tmp/state
d=$XDG_STATE_HOME/andon/dispatcher
mkdir -p "$d"
cat >"$d/sample-site.delivered.jsonl" <<'EOF'
{"at":"2026-11-01T03:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0001","kind":"report"}]}
{"at":"2026-11-01T05:00:00Z","project":"sample-site","acked":true,"news":[{"issue":"S-0001","kind":"report"}]}
EOF
printf '{"at":"2026-11-01T06:00:00Z","project":"sample-site","issue":"S-0001","kind":"merged","detail":{"pr":"%s/1"}}\n' "$pr" >"$d/sample-site.recorded.jsonl"
# セッションの記録（一時の置き場）: S-0001 の要求 req_1 が content block ごとに2行（同じ usage）→ 1回だけ数える。Codex の記録は無い
export CLAUDE_CONFIG_DIR=$tmp/claude CODEX_HOME=$tmp/codex
mkdir -p "$CLAUDE_CONFIG_DIR/projects/-sample"
u='{"type":"assistant","requestId":"req_1","message":{"id":"msg_1","usage":{"input_tokens":3,"output_tokens":40,"cache_read_input_tokens":500,"cache_creation_input_tokens":60}}}'
printf '%s\n%s\n' "$u" "$u" >"$CLAUDE_CONFIG_DIR/projects/-sample/11111111-0000-4000-8000-000000000001.jsonl"
# 内訳（ADR-0060 決めたこと 16）: S-0001 の outcome.json の breakdown（担当）と、Chief of Staff の役ごとの生の記録 2 日分
cat >"$k/issues/S-0001/library/outcome.json" <<'EOF'
{"issue": "S-0001", "breakdown": {"role": "worker", "coverage": {"sessions": 1, "traced": 1}, "interactions": 2, "llm_ms": 900, "tool_ms": 300, "blocked_ms": 0,
 "tokens": {"input": 10, "output": 20, "cache_read": 60, "cache_write": 10}, "errors": {"llm": 0, "tool": 1},
 "tools": [{"name": "Bash", "argv0": "go", "count": 3, "ms": 300, "errors": 1}],
 "waits": {"question_ms": 600000, "review_ms": 0, "ci_ms": 120000, "gone_ms": 0},
 "knowledge": {"index_reads": 1, "pages": [{"path": "topics/shell.md", "reads": 2}]}}}
EOF
# 知識の節（ADR-0061 決めたこと 17）: 依頼書に wiki のページ、申告 2 件（同じ先）、wiki の置き場（切り離したセルの repo の wiki/）に
# 読まれたページ 1 枚と読まれなかったページ 2 枚
printf '先に wiki/topics/shell.md を読む。\n' >"$k/issues/S-0001/brief.md"
printf '{"pr": "%s/1", "gaps": [{"kind": "missing", "where": "AGENTS.md", "text": "x"}, {"kind": "wrong", "where": "AGENTS.md", "text": "y"}]}\n' "$pr" \
  >"$k/issues/S-0001/library/report.json"
mkdir -p "$ANDON_WORKSPACE/wiki/topics" "$ANDON_WORKSPACE/wiki/entities"
for p in topics/shell.md topics/ci.md entities/owner.md index.md; do printf '# x\n' >"$ANDON_WORKSPACE/wiki/$p"; done
o=$XDG_STATE_HOME/andon/otel/sample-site/roles/chief-of-staff
mkdir -p "$o"
span() { # 1 日分の claude_code.llm_request の span（入力・出力・キャッシュ読み・キャッシュ書き が 1・2・3・4）
  printf '{"resourceSpans":[{"resource":{"attributes":[]},"scopeSpans":[{"spans":[{"traceId":"t","spanId":"%s","name":"claude_code.llm_request","startTimeUnixNano":"1","endTimeUnixNano":"2","attributes":[{"key":"input_tokens","value":{"intValue":"1"}},{"key":"output_tokens","value":{"intValue":"2"}},{"key":"cache_read_tokens","value":{"intValue":"3"}},{"key":"cache_creation_tokens","value":{"intValue":"4"}}]}]}]}]}\n' "$1"
}
span s1 >"$o/2026-11-01.traces.jsonl"
span s2 >"$o/2026-11-02.traces.jsonl"
records=$(find "$tmp/claude" "$XDG_STATE_HOME/andon/otel" -type f -exec sha1sum {} + | sort)
before=$(cd "$tmp/sample-site" && find . -type f -exec sha1sum {} + | sort)
export KANBAN_ROOT=$tmp/sample-site
"$andon" metrics show --since 2026-11-01 --until 2026-11-02 --json >"$tmp/out.json"
jq -e '.value == {observed: {count: 0, hit: 0, miss: 0, unknown: 0}, days_to_observe: {count: 0, days: null}, no_hypothesis: 0}
  and .efficiency.merged_prs == 1 and .efficiency.auto_merge.rate == 1
  and .efficiency.human_touches == {total: 3, per_pr: 3, merge: 0, decision: 2, task: 1} and .prs[0].human_touches == 2
  and .efficiency.answers == {owner: 2, chief_of_staff: 1, machine: 0, unmarked: 1}
  and .counter.reverted == 1 and .counter.reopened.by_tier.A == 1 and .counter.reopened.corrections == "none" and .counter.corrected == {corrections: "none", by_tier: {A: 0, B: 0, C: 0, none: 0}}
  and .flow.lead_time.hours == 6 and .flow.closed.count == 2
  and (.skipped | length) == 1 and .skipped[0].line == 4
  and .news.kinds == [{kind: "merged", delivered: 0, acked: 0, rate: null, recorded: 1}, {kind: "report", delivered: 2, acked: 2, rate: 1, recorded: 0}]
  and .news.candidates == ["report"]
  and .tokens == [{issue: "S-0001", source: "claude", usage: {input: 3, output: 40, cache_read: 500, cache_write: 60}}, {issue: "S-0002", source: "claude", usage: null}]
  and .estimate == {count: 0, within: 0, rate: null, none: 1} and (.issues | map(.issue)) == ["S-0001"] and .issues[0].estimate_minutes == null
  and .breakdown.issues == 1 and .breakdown.none == 0
  and .breakdown.waits == {issues: 1, question_ms: 600000, review_ms: 0, ci_ms: 120000, gone_ms: 0}
  and .breakdown.roles == [{role: "worker", issues: 1, token_issues: 1, tokens: {input: 10, output: 20, cache_read: 60, cache_write: 10}, cache_read_rate: 0.75, errors: {llm: 0, tool: 1}}]
  and .breakdown.chief_of_staff == {days: 2, tokens: {input: 2, output: 4, cache_read: 6, cache_write: 8}, share: 0.17}
  and .profiles == [{profile: null, issues: 1, worked_minutes: {count: 1, median: 360}, tokens: {count: 1, median: 100},
    gate: {records: 1, fix: 0, needs_human: 0, rate: 0}, review_rounds: {count: 1, median: 0}}]
  and .knowledge == {issues: 1, briefed: 1, read: {issues: 1, index_only: 0, no_record: 0},
    pages: [{path: "topics/shell.md", reads: 2, issues: 1}], unread: ["entities/owner.md", "topics/ci.md"],
    gaps: {count: 2, issues: 1, by_kind: {missing: 1, wrong: 1, hard_to_find: 0},
      wheres: [{where: "AGENTS.md", count: 2, issues: ["S-0001"]}], repeated: [{where: "AGENTS.md", count: 2, issues: ["S-0001"]}]}}' "$tmp/out.json" >/dev/null
emit "$tmp/out.json" measure.json
echo 'ok --json: 自動マージ・revert・飛ばした行・通知の素通り・トークン（同じ requestId は1回）・内訳・Chief of Staff の 2 日分・プロファイル'
"$andon" metrics show --since 2026-11-01 --until 2026-11-02 >"$tmp/table"
head -1 "$tmp/table" | grep -q '^価値: 仮説を確かめた数 記録なし'
grep -q '^  オーナーの手直しがあった課題  *A 0・B 0・C 0・レベル無し 0' "$tmp/table"
grep '自動マージ率' "$tmp/table" | grep -q '対抗: revert 1本'
grep -q 'kanban/log.jsonl:4' "$tmp/table"
grep -q '^  記録だけへ移す候補.*  report$' "$tmp/table"
grep -q '^  S-0001  *claude 入力 3・出力 40・キャッシュ読み 500・キャッシュ書き 60$' "$tmp/table"
grep -q '^  PR あたりのオーナーの介在回数  *3\.00（3回 / 1本。マージ 0・判断 2・オーナーの課題 1）$' "$tmp/table"
grep -q '^  答えを決めた者  *オーナー 2・Chief of Staff 1・command 0・印なし 1$' "$tmp/table"
grep -q '^  chief-of-staff 2日分 入力 2・出力 4・キャッシュ読み 6・キャッシュ書き 8（全体の 17%）$' "$tmp/table"
grep -q '^  プロファイル無し 1件・実働 360（1件）' "$tmp/table"
grep -q '^  依頼書に wiki のページを載せた課題 1件 / 1件$' "$tmp/table"
grep -q '^  1 回も読まれなかった Topics と Entities のページ  2枚$' "$tmp/table"
grep -q '^    AGENTS.md 2件（繰り返し） S-0001$' "$tmp/table"
echo 'ok 表: 介在の内訳と、答えを決めた者の行と、知識の節'
echo 'ok 表: 価値の行が先頭・自動マージ率の行に対抗の指標・飛ばした行・記録だけへ移す候補'
after=$(cd "$tmp/sample-site" && find . -type f -exec sha1sum {} + | sort)
[ "$before" = "$after" ]
[ "$records" = "$(find "$tmp/claude" "$XDG_STATE_HOME/andon/otel" -type f -exec sha1sum {} + | sort)" ]
echo 'ok kanban にもセッションの記録にもハーネスの記録にも何も書かない'

# 記録が1件も無いプロジェクト: 0 と「記録なし」で終了コード 0（gh は打たない）
mkdir -p "$tmp/empty/kanban"
printf 'version: 3\n' >"$tmp/empty/kanban/config.yaml"
printf '#!/bin/sh\necho forbidden-gh >&2\nexit 99\n' >"$tmp/bin/gh"
export KANBAN_ROOT=$tmp/empty
"$andon" metrics show --json >"$tmp/empty.json"
jq -e '.efficiency.merged_prs == 0 and .efficiency.auto_merge.rate == null and .flow.closed.count == 0
  and .efficiency.human_touches == {total: 0, per_pr: null, merge: 0, decision: 0, task: 0}
  and .efficiency.answers == {owner: 0, chief_of_staff: 0, machine: 0, unmarked: 0}
  and .prs == [] and .issues == [] and .estimate == {count: 0, within: 0, rate: null, none: 0} and .tokens == [] and .skipped == [] and .news == {threshold: 0.8, kinds: [], candidates: []}
  and .breakdown == {issues: 0, none: 0, waits: {issues: 0, question_ms: 0, review_ms: 0, ci_ms: 0, gone_ms: 0}, roles: [], tools: [], chief_of_staff: {days: 0, tokens: null, share: null}}
  and .profiles == []
  and .knowledge == {issues: 0, briefed: 0, read: {issues: 0, index_only: 0, no_record: 0}, pages: [], unread: null,
    gaps: {count: 0, issues: 0, by_kind: {missing: 0, wrong: 0, hard_to_find: 0}, wheres: [], repeated: []}}' "$tmp/empty.json" >/dev/null
emit "$tmp/empty.json" measure-empty.json
"$andon" metrics show >"$tmp/empty.table"
grep '自動マージ率' "$tmp/empty.table" | grep -q '記録なし'
echo 'ok 空の kanban: 0・null・記録なし・終了コード 0'

set +e
"$andon" metrics show --since 2026-11-05 --until 2026-11-01 >/dev/null 2>&1; rc=$?
set -e
[[ $rc == 2 ]]
echo 'ok --since が --until より後なら終了コード 2'
