#!/usr/bin/env bash
# 報告と ready（features/report-ready.md）。切り離した kanban で、担当が打つ道（record → report → ready）をそのまま通す。
set -uo pipefail
. "$(dirname "$0")/../sandbox.sh"

# 担当の worktree（repo の課題の形）と、PR の head
git init -q "$tmp/wt" && echo one >"$tmp/wt/f" && git -C "$tmp/wt" add f && git -C "$tmp/wt" commit -qm one
head=$(git -C "$tmp/wt" rev-parse HEAD)
pr=https://github.com/example/sample-site/pull/7
echo "$head" >"$tmp/gh/head"
# PR の branch（report --pr が課題の branch と照らす）。初めはほかの課題の branch にして、rr-report が断られることを見る
branch=verify/$run
echo other/branch >"$tmp/gh/branch"
# PR のファイルと差分（ready が gate と同じ関数でレビューが要るかを決めるのに読む）。文書1行だけのレベル A にして、review: off で通す
echo '[[{"filename":"docs/verify.md","additions":1,"deletions":0}]]' >"$tmp/gh/files"
printf '%s\n' 'diff --git a/docs/verify.md b/docs/verify.md' 'new file mode 100644' '--- /dev/null' '+++ b/docs/verify.md' '@@ -0,0 +1 @@' '+verify' >"$tmp/gh/diff"

ledger_init V
id=$(HC_TERMINAL_ID=tc "$kanban" add "$run の報告と ready" --profile claude)
HC_TERMINAL_ID=tc "$kanban" set "$id" status=doing review=off must_fix=major merge=person \
  "worktree=$tmp/wt" repo=github.com/example/sample-site "branch=$branch" >/dev/null
lib=$tmp/project/kanban/issues/$id/library
export HC_TERMINAL_ID=tw
echo "{\"labels\":{\"issue\":\"$id\"}}" >"$tmp/hc/tw.json"

# rr-record: 走らせた記録が残る
step "rr-record: 証拠の command を記録で撮る" "$andon" issue record "$id" -- git -C "$tmp/wt" log -1 --format=%H \
  && jq -e --arg c "$head" '.id == 1 and .exit_code == 0 and .expect == "pass" and .commit == $c and .dirty == false' "$lib/runs/1.json" >/dev/null \
  && grep -qx "$head" "$lib/runs/1.log" \
  && ok "rr-record: runs/1.json（終了コード 0・commit が HEAD・dirty false）と runs/1.log（出力）が残る" \
  || fail "rr-record: 記録か出力が無い・形が違う"
jq -c . "$lib/runs/1.json" 2>/dev/null | sed 's/^/  runs\/1.json: /'

# rr-ready-refuse: 報告の前の ready は断られ、kanban は動かない
step "rr-ready-refuse: 報告の前に ready を打つ" "$andon" issue ready "$id" \
  && fail "rr-ready-refuse: 報告が無いのに ready が通った" \
  || { [ "$("$kanban" show "$id" --json | jq -r .issue.status)" = doing ] \
    && ok "rr-ready-refuse: 報告が無ければ断り、課題は doing のまま" || fail "rr-ready-refuse: 断ったのに status が動いた"; }

# rr-report: PR の branch が課題の branch と違えば（ほかの課題の PR）、report --pr は書かずに断る。課題の branch の PR なら書く
step "rr-report: ほかの branch の PR を報告に書く" "$andon" issue report "$id" --pr "$pr" <<<"## Report" \
  && fail "rr-report: PR の branch（other/branch）が課題の branch（$branch）と違うのに report --pr が通った" \
  || { grep -q 'この課題の PR でない' "$tmp/step.out" && [ ! -e "$lib/report.json" ] \
    && ok "rr-report: PR の branch が課題の branch と違えば、report.json を書かずに断る" \
    || fail "rr-report: 断る理由が「この課題の PR でない」でないか、report.json を書いた"; }
echo "$branch" >"$tmp/gh/branch"
# rr-report: 報告が report.json と report.md になる
data='{"acceptance":[{"id":"A1","result":"pass","evidence":"git log -1（記録 runs/1.json）","run":1}],"next":[{"to":"chief-of-staff","text":"検証の跡を片付ける"}]}'
step "rr-report: 報告を書く" "$andon" issue report "$id" --pr "$pr" --data "$data" <<<"## Report
$run の検証の報告" \
  && jq -e --arg c "$head" --arg pr "$pr" '.commit == $c and .pr == $pr and .acceptance[0].run == 1' "$lib/report.json" >/dev/null \
  && grep -q "$run の検証の報告" "$tmp/project/kanban/issues/$id/report.md" \
  && ok "rr-report: report.json（commit は HEAD・pr・acceptance の run）と report.md が書かれる" \
  || fail "rr-report: report.json か report.md が無い・形が違う"
jq -c '{commit, pr, acceptance}' "$lib/report.json" 2>/dev/null | sed 's/^/  report.json: /'

# rr-acceptance: 予約キー acceptance は数字だけで書く（Chief Engineer の文書の振り方）。報告の行は A1 のまま。
# 行の無い番号（2）だけを名指して断り、A1 の行は番号 1 に数える。断ったあとキーを 1 にすると、rr-ready が A1 の行で通す
HC_TERMINAL_ID=tc "$kanban" set "$id" acceptance=1,2 >/dev/null
step "rr-acceptance: 予約キー acceptance=1,2 で ready を打つ（報告の行は A1 だけ）" "$andon" issue ready "$id" \
  && fail "rr-acceptance: 番号 2 の行が無いのに ready が通った" \
  || { grep -q '無いので review にしない: 2。' "$tmp/step.out" \
    && [ "$("$kanban" show "$id" --json | jq -r .issue.status)" = doing ] \
    && ok "rr-acceptance: 行の無い番号 2 だけを名指して断り（A1 の行は番号 1 に数える）、課題は doing のまま" \
    || fail "rr-acceptance: 断る理由が番号 2 だけでないか、status が動いた"; }
HC_TERMINAL_ID=tc "$kanban" set "$id" acceptance=1 >/dev/null

# rr-ready: ready で review へ進む
step "rr-ready: ready を打つ" "$andon" issue ready "$id" \
  && [ "$("$kanban" show "$id" --json | jq -r .issue.status)" = review ] \
  && ok "rr-ready: ready が通り、課題が review に入る" \
  || fail "rr-ready: ready が通らないか、status が review でない"
"$kanban" show "$id" --json | jq -c '.issue | {id, status, review, merge, acceptance}' | sed 's/^/  kanban show: /'
jq -c --arg id "$id" 'select(.id == $id and .change.status) | {by, change}' "$tmp/project/kanban/log.jsonl" | sed 's/^/  log.jsonl: /'

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
