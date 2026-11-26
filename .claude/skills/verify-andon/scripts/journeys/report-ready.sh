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
# rr-report: 前の report.json の pr がほかの課題の PR（この直しより前に書かれたもの）なら、--pr を付けずに書き直すと引き継がずに外す
step "rr-report: 報告を書く（pr 無し）" "$andon" issue report "$id" <<<"## Report" >/dev/null
# rr-ready-report: 報告の課題（Chief of Staff が deliverable=report を書いた）で pr の無い報告の ready は、kanban review で返すと案内して
# 断り（--pr を案内しない）、手で打った gate も回さずに終了コード 3 で記録を書かない（ADR-0066 決めたこと 8）。PR の課題に戻して続ける
HC_TERMINAL_ID=tc "$kanban" set "$id" deliverable=report >/dev/null
step "rr-ready-report: 報告の課題で pr の無い報告の ready を打つ" "$andon" issue ready "$id" \
  && fail "rr-ready-report: 報告の課題で pr が無いのに ready が通った" \
  || { grep -q "deliverable: report の課題で PR が無いときは kanban review $id で返す" "$tmp/step.out" \
    && ! grep -q -- '--pr' "$tmp/step.out" \
    && [ "$("$kanban" show "$id" --json | jq -r .issue.status)" = doing ] \
    && ok "rr-ready-report: 報告の課題で pr が無ければ、ready は kanban review で返すと案内して断り（--pr を案内しない）、課題は doing のまま" \
    || fail "rr-ready-report: 断る理由が kanban review の案内でないか、--pr を案内したか、status が動いた"; }
grc=0; step "rr-ready-report: 報告の課題で pr の無い報告に gate を打つ" "$andon" gate "$id" || grc=$?
[ "$grc" = 3 ] && grep -q 'deliverable: report の課題で PR が無い。gate を回すものが無い' "$tmp/step.out" && [ ! -e "$lib/gate" ] \
  && ok "rr-ready-report: 報告の課題で pr が無ければ、手で打った gate は回さずに終了コード 3 で、記録を書かない" \
  || fail "rr-ready-report: 報告の課題で pr が無いのに、gate が終了コード 3 でないか、理由が違うか、記録を書いた"
# 上の「--pr を案内しない」の確かめが何も見ない形でないこと: 担当が書いた report は PR の課題として --pr を案内する
"$kanban" set "$id" deliverable=report >/dev/null
step "rr-ready-report: 担当が書いた report で pr の無い報告の ready を打つ" "$andon" issue ready "$id" \
  && fail "rr-ready-report: 担当が report を書いた課題で pr が無いのに ready が通った" \
  || { grep -q -- '--pr' "$tmp/step.out" && grep -q 'Chief of Staff でない' "$tmp/step.out" \
    && ok "rr-ready-report: 担当が書いた report は PR の課題として --pr を案内し、外れた理由を添える" \
    || fail "rr-ready-report: 担当が書いた report の ready が --pr を案内しないか、外れた理由を添えない"; }
step "rr-ready-report: 担当が書いた report で pr の無い報告に gate を打つ" "$andon" gate "$id" \
  && fail "rr-ready-report: 担当が report を書いた課題で pr が無いのに gate が通った" \
  || { grep -q 'report.json に pr が無い' "$tmp/step.out" && grep -q 'Chief of Staff でない' "$tmp/step.out" && [ ! -e "$lib/gate" ] \
    && ok "rr-ready-report: 担当が書いた report は、手で打った gate も PR の課題として pr を求める fix を返し、外れた理由を添える（記録は書かない）" \
    || fail "rr-ready-report: 担当が書いた report の gate が pr を求めないか、外れた理由を添えないか、記録を書いた"; }
# この先は Chief of Staff が書いた報告の課題のまま、pr のある報告で rr-ready を通す（pr があれば PR の課題と同じ確かめ）
HC_TERMINAL_ID=tc "$kanban" set "$id" deliverable=report >/dev/null
jq --arg pr "$pr" '.pr = $pr' "$lib/report.json" >"$tmp/rep.new" && mv "$tmp/rep.new" "$lib/report.json"
step "rr-report: ほかの課題の PR が入った報告を、--pr を付けずに書き直す" "$andon" issue report "$id" <<<"## Report" \
  && grep -q 'この課題の PR でないので引き継がない' "$tmp/step.out" && jq -e '(.pr // "") == ""' "$lib/report.json" >/dev/null \
  && ok "rr-report: 前の報告のほかの課題の PR は、書き直すと引き継がずに外す" \
  || fail "rr-report: 前の報告のほかの課題の PR を引き継いだか、外したことを書かない"
echo "$branch" >"$tmp/gh/branch"
# rr-report: 見出し「## Report」の行の無い報告（本文が消えて PR の行だけが残ったもの）は、書かずに断る
cp "$lib/report.json" "$tmp/rep.before"
step "rr-report: 見出しの無い報告を書く" "$andon" issue report "$id" --pr "$pr" <<<"PR: $pr" \
  && fail "rr-report: 見出し「## Report」の行が無いのに report が通った" \
  || { grep -q '見出し「## Report」の行が無い' "$tmp/step.out" && cmp -s "$tmp/rep.before" "$lib/report.json" \
    && ok "rr-report: 見出し「## Report」の行が無ければ、report.json を書かずに断る" \
    || fail "rr-report: 断る理由が「見出しの行が無い」でないか、report.json を書いた"; }
# rr-report: review-request が stdin なしで引き継ぐ前の報告（report.md）に見出しが無くても、書かずに断る
printf 'PR: %s\n' "$pr" >"$tmp/project/kanban/issues/$id/report.md"
step "rr-report: 見出しの無い前の報告で review-request を打つ" "$andon" issue review-request "$id" </dev/null \
  && fail "rr-report: 前の報告に見出し「## Report」の行が無いのに review-request が通った" \
  || { grep -q '見出し「## Report」の行が無い' "$tmp/step.out" && cmp -s "$tmp/rep.before" "$lib/report.json" \
    && ok "rr-report: 前の報告に見出し「## Report」の行が無ければ、review-request は書かずに断る" \
    || fail "rr-report: review-request の断る理由が「見出しの行が無い」でないか、report.json を書いた"; }
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

# rr-ready: ready で review へ進む。課題は Chief of Staff が deliverable=report を書いた報告の課題で、pr があるので PR の課題と同じ確かめを通る
step "rr-ready: ready を打つ" "$andon" issue ready "$id" \
  && [ "$("$kanban" show "$id" --json | jq -r .issue.status)" = review ] \
  && ok "rr-ready: ready が通り、課題が review に入る" \
  || fail "rr-ready: ready が通らないか、status が review でない"
"$kanban" show "$id" --json | jq -c '.issue | {id, status, review, merge, acceptance, deliverable}' | sed 's/^/  kanban show: /'
jq -c --arg id "$id" 'select(.id == $id and .change.status) | {by, change}' "$tmp/project/kanban/log.jsonl" | sed 's/^/  log.jsonl: /'

touch "$tmp/hc.calls" "$tmp/gh.calls"
echo "偽の hc への呼び出し: $(wc -l <"$tmp/hc.calls") 件。偽の gh への呼び出し: $(wc -l <"$tmp/gh.calls") 件"
sed 's/^/  /' "$tmp/hc.calls" "$tmp/gh.calls"
[ "$fails" = 0 ]
