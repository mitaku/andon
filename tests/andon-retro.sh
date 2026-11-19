#!/usr/bin/env bash
# `andon retro collect`。一時の台帳（ファイルを直に置く）・偽の gh・一時の Claude Code のセッションログで回す。台帳の CLI も hc の端末も使わない。
# 規則の細部は internal/retro/retro_test.go。ここはバイナリの入口・ADR-0028 の受け入れ条件 7〜10・18・19・読むだけ・終了コードを見る。
# ANDON_EMIT_DIR があれば出力を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
command -v jq >/dev/null || { echo "skip: jq が無い"; exit 0; }
date -u -d '1 day ago' +%s >/dev/null 2>&1 || { echo "skip: GNU date が無い"; exit 0; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET
ago() { date -u -d "$1 ago" +%Y-%m-%dT%H:%M:%SZ; }
die() { echo "FAIL $*" >&2; exit 1; }

# 偽の鍵（repo に鍵の形の文字列を置かない。ここで組み立てる）
key="ghp_$(printf 'Z9%.0s' $(seq 18))"
pr=https://github.com/example/sample-site/pull
sha=1111111111111111111111111111111111111111

# 偽の gh: マージされた PR の一覧（#1 は #3 に revert された・#7 と #8 は前の振り返りが切った直し）と、#1 のマージ commit の checks
mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
echo "\$*" >>"$tmp/gh.calls"
case "\$*" in
"pr list --repo example/sample-site --state merged --limit 1000 --json number,url,title,body,mergedAt,mergeCommit")
  cat <<'JSON'
[{"number": 1, "url": "$pr/1", "title": "one", "body": "", "mergedAt": "$(ago '2 days')", "mergeCommit": {"oid": "$sha"}},
 {"number": 3, "url": "$pr/3", "title": "Revert \"one\"", "body": "Reverts example/sample-site#1", "mergedAt": "$(ago '1 day')", "mergeCommit": null},
 {"number": 7, "url": "$pr/7", "title": "fix", "body": "", "mergedAt": "$(ago '3 days')", "mergeCommit": null},
 {"number": 8, "url": "$pr/8", "title": "fix", "body": "", "mergedAt": "$(ago '20 days')", "mergeCommit": null}]
JSON
  ;;
"api --paginate repos/example/sample-site/commits/$sha/check-runs?per_page=100")
  echo '{"check_runs": [{"name": "tests", "status": "completed", "conclusion": "failure"}, {"name": "lint", "status": "completed", "conclusion": "success"}]}' ;;
*) echo "unexpected: \$*" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"

# 台帳
root=$tmp/sample-site
k=$root/kanban
lib() { mkdir -p "$k/issues/$1/library"; echo "$k/issues/$1/library"; }
yaml() { local id=$1; shift; mkdir -p "$k/issues/$id"; printf 'id: %s\ntitle: %s の題\n%b' "$id" "$id" "$*" >"$k/issues/$id/issue.yaml"; }
mkdir -p "$k"
printf 'version: 3\nprefix: S\n' >"$k/config.yaml"
launch() { printf '{"at": "%s", "role": "worker", "launch": "start", "session": %s, "package": {"commit": "%s"}, "memory_index": "- 目録\\n"}\n' "$1" "$2" "$3"; }
# S-0001: 期間の内。PR はマージ後に revert され、マージ commit の checks が落ちた。レビュアーの子 S-0001-r1 がある
yaml S-0001 'status: done\nprofile: claude\nrepo: github.com/example/sample-site\n'
l=$(lib S-0001)
launch "$(ago '3 days')" '"a1a1a1a1-0000-4000-8000-000000000001"' "$sha" >"$l/launches.jsonl"
printf '{"pr": "%s/1", "remember": ["学び"]}\n' "$pr" >"$l/report.json"
printf '{"at": "x", "corrections": [{"kind": "pr", "comments": [{"author": "o", "at": "x", "body": "token = %s"}], "human_commits": []}]}\n' "$key" >"$l/corrections.json"
yaml S-0001-r1 'status: done\nprofile: codex-review\nparent: S-0001\n'
printf '{"commit": "%s", "findings": [{"severity": "major", "file": "a.go", "line": 3}]}\n' "$sha" >"$(lib S-0001-r1)/findings.json"
# S-0002: 期間の外。S-0003: 途中で当て直した（当て直しの記録の時刻が起動と片付けの間）
yaml S-0002 'status: done\nprofile: claude\n'
yaml S-0003 'status: done\nprofile: claude\n'
launch "$(ago '2 days')" null "$sha" >"$(lib S-0003)/launches.jsonl"
# S-0006: 前の振り返り（ルーチン retro が足した）。S-0007・S-0008 はそれが切った直し（マージは3日前と20日前）
yaml S-0006 'status: done\nprofile: claude\nroutine: retro\n'
yaml S-0007 'status: done\nprofile: claude\nparent: S-0006\n'
printf '{"pr": "%s/7"}\n' "$pr" >"$(lib S-0007)/report.json"
yaml S-0008 'status: done\nprofile: claude\nparent: S-0006\n'
printf '{"pr": "%s/8"}\n' "$pr" >"$(lib S-0008)/report.json"
for x in "S-0001 2 days" "S-0001-r1 2 days" "S-0002 9 days" "S-0003 1 day" "S-0006 21 days" "S-0007 3 days" "S-0008 20 days"; do
  set -- $x
  printf '{"at":"%s","by":"terminal:t1","id":"%s","change":{"status":"done"}}\n' "$(ago "$2 $3")" "$1"
done >"$k/log.jsonl"
export ANDON_HOME=$tmp/pkg
mkdir -p "$ANDON_HOME"
printf '{"commit": "%s", "applied_at": "%s", "image_commit": "x"}\n' "$sha" "$(ago '36 hours')" >"$ANDON_HOME/applied.json"

# セッションログ: Chief of Staff（cwd がプロジェクトの根）。どの行にも人の印を付ける
export CLAUDE_CONFIG_DIR=$tmp/claude CODEX_HOME=$tmp/codex
mkdir -p "$CLAUDE_CONFIG_DIR/projects/-sample-site"
hum() { jq -cn --arg u "$1" --arg at "$2" --arg cwd "$root" --arg text "$3" \
  '{type: "user", uuid: $u, timestamp: $at, cwd: $cwd, origin: {kind: "human"}, promptSource: "typed", message: {role: "user", content: $text}}'; }
{
  hum u0 "$(ago '3 days')" 'プロジェクト sample-site の Chief of Staff。'
  hum u1 "$(ago '2 days')" "PR の題に課題番号を書かないで。鍵は $key"
  hum u2 "$(ago '2 days')" '<hc from="t9 x">ほかの端末から届いた文</hc>'
  hum u3 "$(ago '2 days')" $'<news project="sample-site">\nS-0001 review\n</news>'
  hum u4 "$(ago '2 days')" '一時のエラーで止まった。やり直しに承認は要らない。止まる前の作業を続けて（dispatcher から）'
  jq -cn --arg at "$(ago '2 days')" '{type: "user", uuid: "u5", timestamp: $at, origin: {kind: "human"}, message: {content: [{type: "tool_result", content: "道具の結果"}]}}'
} >"$CLAUDE_CONFIG_DIR/projects/-sample-site/c0c0c0c0-0000-4000-8000-000000000001.jsonl"

before=$( (cd "$tmp" && find sample-site claude pkg -type f -exec sha1sum {} +) | sort)
grep -q "$key" "$CLAUDE_CONFIG_DIR/projects/-sample-site/"*.jsonl || die "検査の前提: セッションログに偽の鍵がある"
KANBAN_ROOT=$root "$andon" retro collect >"$tmp/out.json"
emit "$tmp/out.json" retro.json

# 7: 秘密が残らない（同じ grep が入力の側では当たることを上で確かめた）
! grep -q "$key" "$tmp/out.json" || die "出力に鍵が残っている"
jq -e '.redacted == 2' "$tmp/out.json" >/dev/null || die "伏せた数が違う: $(jq .redacted "$tmp/out.json")"
echo 'ok 7: 秘密の形を伏せる（人の文と手直しの本文）'

# 8: 期間に片付いた担当の課題だけ（片付いた順）。子の指摘は親に入る
jq -e '[.issues[].id] == ["S-0007", "S-0001", "S-0003"]' "$tmp/out.json" >/dev/null || die "束ねた課題: $(jq -c '[.issues[].id]' "$tmp/out.json")"
jq -e '.issues[] | select(.id == "S-0001") | .reviews == [{id: "S-0001-r1", profile: "codex-review",
  findings: {commit: "1111111111111111111111111111111111111111", findings: [{severity: "major", file: "a.go", line: 3}]}, outcome: null}]
  and .remember == ["学び"] and (.launches[0] | has("memory_index") | not)' "$tmp/out.json" >/dev/null || die "S-0001 の束: $(jq -c '.issues[1]' "$tmp/out.json")"
jq -e '.previous_retro.issue == "S-0006"' "$tmp/out.json" >/dev/null || die "前の振り返り"
echo 'ok 8: 期間の内の担当の課題だけ。レビュアーの子の指摘は親の1件に入る'

# 9: 人が打った文だけ
jq -e '[.typed[] | [.id, .role, .issue]] == [["h-u1", "chief-of-staff", null]]' "$tmp/out.json" >/dev/null || die "人の文: $(jq -c '.typed' "$tmp/out.json")"
echo 'ok 9: 人の文だけ（起動の最初のメッセージ・hc send・通知・dispatcher の再開の文・道具の結果は入らない）'

# 10: マージ後のこと
jq -e '.issues[] | select(.id == "S-0001") | .after_merge == {merge_commit: "1111111111111111111111111111111111111111", merged_at: .after_merge.merged_at,
  checks: {state: "failure", failed: ["tests"]}, reverted_by: [{pr: "https://github.com/example/sample-site/pull/3", merged_at: .after_merge.reverted_by[0].merged_at}]}' \
  "$tmp/out.json" >/dev/null || die "マージ後: $(jq -c '.issues[1].after_merge' "$tmp/out.json")"
echo 'ok 10: マージ commit の checks の失敗と revert が元の課題に付く'

# 18: 前の直し（3日前のものだけ。マージのあとに片付いた担当の課題は S-0001 と S-0003）
jq -e '.fixes == [{issue: "S-0007", title: "S-0007 の題", retro: "S-0006", pr: "https://github.com/example/sample-site/pull/7", merged_at: .fixes[0].merged_at, issues_after: 2}]' \
  "$tmp/out.json" >/dev/null || die "前の直し: $(jq -c '.fixes' "$tmp/out.json")"
echo 'ok 18: 前の直しはマージから 14 日以内のものだけ、マージのあとの課題の数つき'

# 19: 当て直しの印
jq -e '[.issues[] | {(.id): .reapplied}] | add == {"S-0007": null, "S-0001": false, "S-0003": true}' "$tmp/out.json" >/dev/null || die "当て直し: $(jq -c '[.issues[] | [.id, .reapplied]]' "$tmp/out.json")"
echo 'ok 19: 起動と片付けの間に当て直した課題に印（起動の記録の無い課題は null）'

after=$( (cd "$tmp" && find sample-site claude pkg -type f -exec sha1sum {} +) | sort)
[ "$before" = "$after" ] || die "台帳かセッションログが書き換わった"
! grep -qv '^pr list \|^api --paginate repos/' "$tmp/gh.calls" || die "gh で読む以外を打った: $(cat "$tmp/gh.calls")"
echo 'ok 読むだけ（台帳・セッションログ・パッケージの置き場を書き換えない。gh は読むだけ）'

# GitHub が読めないときも集め、マージ後のことは null・skipped に1行
printf '#!/bin/sh\necho "HTTP 502" >&2\nexit 1\n' >"$tmp/bin/gh"
KANBAN_ROOT=$root "$andon" retro collect >"$tmp/out2.json"
jq -e '(.issues[] | select(.id == "S-0001") | .after_merge) == null and .fixes == [] and (.skipped | length) == 1 and (.skipped[0].reason | test("HTTP 502"))' \
  "$tmp/out2.json" >/dev/null || die "gh が落ちたとき: $(jq -c '.skipped' "$tmp/out2.json")"
emit "$tmp/out2.json" retro-nogh.json
echo 'ok GitHub が読めなくても集める（マージ後は null・skipped に1行）'

# 終了コード
set +e
"$andon" retro >/dev/null 2>&1; rc_usage=$?
KANBAN_ROOT=$root "$andon" retro collect --days 0 >/dev/null 2>&1; rc_days=$?
KANBAN_ROOT=$tmp/nowhere "$andon" retro collect >/dev/null 2>&1; rc_none=$?
set -e
[ "$rc_usage" = 2 ] && [ "$rc_days" = 2 ] && [ "$rc_none" = 1 ] || die "終了コード: usage=$rc_usage days=$rc_days none=$rc_none"
echo 'ok 終了コード: 使い方 2・--days の外 2・台帳が無い 1'
