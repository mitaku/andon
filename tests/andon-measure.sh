#!/usr/bin/env bash
# `andon measure`。一時の台帳（ファイルを直に置く）と偽の gh で回す。台帳の CLI も hc の端末も使わない。
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

# 課題 1 件（自動マージして revert された）と、壊れた log.jsonl の行
k=$tmp/sample-site/kanban
mkdir -p "$k/issues/S-0001/library/gate"
printf 'version: 3\nprefix: S\n' >"$k/config.yaml"
printf 'id: S-0001\ntitle: one\nstatus: done\ncreated: "2026-11-01T00:00:00Z"\n' >"$k/issues/S-0001/issue.yaml"
printf '{"pr": "%s/1"}\n' "$pr" >"$k/issues/S-0001/library/report.json"
printf '{"at": "2026-11-01T06:00:00Z", "id": "S-0001", "pr": "%s/1", "tier": "A", "decision": "approve", "merged": true}\n' "$pr" \
  >"$k/issues/S-0001/library/gate/aaaa.json"
cat >"$k/log.jsonl" <<'EOF'
{"at":"2026-11-01T00:00:00Z","by":"terminal:t1","id":"S-0001","change":{"add":"one"}}
{"at":"2026-11-01T01:00:00Z","by":"terminal:t2","id":"S-0001","change":{"status":"doing"}}
{"at":"2026-11-01T02:00:00Z","by":"someone","id":"S-0001","change":{"merge":"auto"}}
{"at":"broken
{"at":"2026-11-01T07:00:00Z","by":"terminal:t1","id":"S-0001","change":{"status":"done"}}
EOF
before=$(cd "$tmp/sample-site" && find . -type f -exec sha1sum {} + | sort)
export KANBAN_ROOT=$tmp/sample-site
"$andon" measure --since 2026-11-01 --until 2026-11-02 --json >"$tmp/out.json"
jq -e '.efficiency.merged_prs == 1 and .efficiency.auto_merge.rate == 1 and .efficiency.human_touches.total == 1
  and .counter.reverted == 1 and .counter.reopened.by_tier.A == 1 and .counter.reopened.corrections == "none" and .counter.corrected == {corrections: "none", by_tier: {A: 0, B: 0, C: 0, none: 0}}
  and .flow.lead_time.hours == 6 and .flow.closed.count == 1
  and (.skipped | length) == 1 and .skipped[0].line == 4' "$tmp/out.json" >/dev/null
emit "$tmp/out.json" measure.json
echo 'ok --json: 自動マージ・revert・飛ばした行'
"$andon" measure --since 2026-11-01 --until 2026-11-02 >"$tmp/table"
head -1 "$tmp/table" | grep -q '^価値: 未計測'
grep -q '^  人の手直しがあった課題  *A 0・B 0・C 0・レベル無し 0' "$tmp/table"
grep '自動マージ率' "$tmp/table" | grep -q '対抗: revert 1本'
grep -q 'kanban/log.jsonl:4' "$tmp/table"
echo 'ok 表: 価値: 未計測が先頭・自動マージ率の行に対抗の指標・飛ばした行'
after=$(cd "$tmp/sample-site" && find . -type f -exec sha1sum {} + | sort)
[ "$before" = "$after" ]
echo 'ok 台帳に何も書かない'

# 記録が1件も無いプロジェクト: 0 と「記録なし」で終了コード 0（gh は打たない）
mkdir -p "$tmp/empty/kanban"
printf 'version: 3\n' >"$tmp/empty/kanban/config.yaml"
printf '#!/bin/sh\necho forbidden-gh >&2\nexit 99\n' >"$tmp/bin/gh"
export KANBAN_ROOT=$tmp/empty
"$andon" measure --json >"$tmp/empty.json"
jq -e '.efficiency.merged_prs == 0 and .efficiency.auto_merge.rate == null and .flow.closed.count == 0
  and .prs == [] and .skipped == []' "$tmp/empty.json" >/dev/null
emit "$tmp/empty.json" measure-empty.json
"$andon" measure >"$tmp/empty.table"
grep '自動マージ率' "$tmp/empty.table" | grep -q '記録なし'
echo 'ok 空の台帳: 0・null・記録なし・終了コード 0'

set +e
"$andon" measure --since 2026-11-05 --until 2026-11-01 >/dev/null 2>&1; rc=$?
set -e
[[ $rc == 2 ]]
echo 'ok --since が --until より後なら終了コード 2'
