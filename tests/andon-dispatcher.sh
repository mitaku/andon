#!/usr/bin/env bash
# `andon dispatcher`（インナーループ。docs/dispatcher.md §5 の P6a）。本物の台帳の CLI（kanban）と git・一時のセルの repo・偽の hc・偽の gh で回す。
# Chief of Staff（t5）は台帳を用意したあと1つも打たず、review-request → レビュアーを起こす → レビュー → ready → ゲート →
# マージ → finish が通ることを見る。fix は担当へ hc send、needs_human は問い（from: gate）。GitHub と本物の hc には出ない。
# ANDON_EMIT_DIR があれば、通知・問い・ゲートの記録を写す（tests/schema.sh が schema に通す）。台帳の CLI は tests/lib/kanban.sh が焼く。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" XDG_STATE_HOME="$tmp/state" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@example.com GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=agent@example.com
export ANDON_HOME="$andon_root/package" ANDON_WORKSPACE="$tmp/plant" GHQ_ROOT="$tmp/ghq"
unset KANBAN_ROOT HC_TERMINAL_ID
mkdir -p "$HOME" "$tmp/bin" "$tmp/gh" "$tmp/hc"
pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
as_owner() { GIT_AUTHOR_NAME=o GIT_AUTHOR_EMAIL=owner@example.com "$@"; }

# 案件 repo: origin（PR の head と main を持つ）と、ghq の clone（担当とレビュアーの worktree の元）
git init -q -b main "$tmp/origin"
git -C "$tmp/origin" commit -q --allow-empty -m base
git -C "$tmp/origin" config receive.denyCurrentBranch ignore
app=$tmp/ghq/github.com/example/app
git clone -q "$tmp/origin" "$app"
git config --global url."file://$tmp/origin".insteadOf https://github.com/example/app

# 偽の gh: PR 7 の事実は $tmp/gh/ のファイル。マージは origin の main をその head に進める
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
d=$tmp/gh
case "\$*" in
  "pr view 7 --repo example/app --json headRefOid -q .headRefOid"|"pr view https://github.com/example/app/pull/7 --json headRefOid -q .headRefOid") jq -r .headRefOid \$d/view.json ;;
  "pr view 7 --repo example/app --json state,mergeCommit")   # 打った回数を states に（人のマージを見る間隔）。closed があれば閉じた PR
    echo x >> \$d/states
    if [ -f \$d/closed ]; then echo '{"state":"CLOSED","mergeCommit":null}'
    elif [ -f \$d/merged ]; then jq -n --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o}}'; else echo '{"state":"OPEN","mergeCommit":null}'; fi ;;
  "pr view 7 --repo example/app --json state,commits,comments,reviews")   # 人の手直し（prcfail があれば落ちる。prc.json があればその中身）
    [ ! -f \$d/prcfail ] || { echo "HTTP 502" >&2; exit 1; }
    if [ -f \$d/prc.json ]; then cat \$d/prc.json
    else jq --arg s "\$(if [ -f \$d/merged ]; then echo MERGED; else echo OPEN; fi)" '{state:\$s,commits:[{oid:.headRefOid,messageHeadline:"x"}],comments:[],reviews:[]}' \$d/view.json; fi ;;
  "api repos/example/app/commits/"*) echo '{"files":[{"filename":"docs/b.md"}]}' ;;
  "api --paginate --slurp repos/example/app/pulls/7/comments") if [ -f \$d/inline.json ]; then cat \$d/inline.json; else echo '[[]]'; fi ;;   # インラインのレビューコメント
  "pr view 7 --repo example/app --json "*) cat \$d/view.json ;;
  "pr view https://github.com/example/app/pull/7 --json state,mergeCommit,files")   # andon issue impact（impactfail があれば落ちる）
    [ ! -f \$d/impactfail ] || { echo "HTTP 502" >&2; exit 1; }
    if [ -f \$d/merged ]; then jq --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o},files:[.[0][] | {path: .filename}]}' \$d/files.json
    else echo '{"state":"OPEN","mergeCommit":null,"files":[]}'; fi ;;
  "api graphql "*)  # statusCheckRollup.contexts を100件ずつ（after は前のページの件数）
    off=\$(printf '%s\n' "\$@" | sed -n 's/^after=//p'); echo "graphql \${off:-0}" >> \$d/pages
    jq --argjson off "\${off:-0}" --slurpfile v \$d/view.json '{data: {repository: {pullRequest: {headRefOid: \$v[0].headRefOid,
      commits: {nodes: [{commit: {oid: \$v[0].headRefOid, statusCheckRollup: (if length == 0 then null else
        {contexts: {pageInfo: {hasNextPage: (length > \$off + 100), endCursor: ((\$off + 100) | tostring)}, nodes: .[\$off:\$off + 100]}} end)}}]}}}}}' \$d/rollup.json ;;
  "api --paginate --slurp repos/example/app/pulls/7/files") cat \$d/files.json ;;
  "pr diff 7 --repo example/app") cat \$d/diff ;;
  "api repos/example/app --jq .default_branch") echo main ;;
  "api user --jq .login") echo plant-bot ;;
  "api -X POST repos/example/app/statuses/"*) echo "status \$*" >> \$d/posted ;;
  "pr comment 7 --repo example/app --body "*) echo "comment" >> \$d/posted
    if [ -f \$d/hook ]; then sh \$d/hook; rm -f \$d/hook; fi ;;  # ゲートの最後の書き込み。マージの前に状態を変える
  "pr merge 7 --repo example/app "*)
    echo "\$*" >> \$d/merges
    h=\$(jq -r .headRefOid \$d/view.json)
    git -C $tmp/origin update-ref refs/heads/main "\$h" && echo "\$h" > \$d/merged ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
# GitHub の checks（statusCheckRollup）は $tmp/gh/rollup.json。既定は通っている
checks() { jq -n --arg s "$1" --arg c "${2:-}" '[{__typename: "CheckRun", name: "test", status: $s, conclusion: $c}]' > "$tmp/gh/rollup.json"; }
mkdir -p "$tmp/gh"; checks COMPLETED SUCCESS
# 偽の hc: 生きている端末は $tmp/hc/live（address issue の行）。run は t9, t10 … を返し、打ったものを $tmp/hc/log に。
# run の first_message は $tmp/hc/first の中身（無ければ届いた: sent・started が true）
# get --json は $tmp/hc/term-<addr>.json、read は screen-<addr>、attach --observe は ansi-<addr>（無ければ何も出さない）
cat > "$tmp/bin/hc" <<EOF
#!/bin/bash
d=$tmp/hc
echo "\$*" >> \$d/log
case \$1 in
  get) [ ! -f "\$d/getfail" ] || ! grep -qx "\$2" \$d/getfail || { echo '{"error":{"kind":"internal","message":"x"}}' >&2; exit 1; }  # 一時の失敗
       grep -q "^\$2 " \$d/live || { echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2; exit 3; }
       [ "\${3:-}" != --json ] || cat \$d/term-\$2.json 2>/dev/null ;;
  read) cat \$d/screen-\$2 2>/dev/null ;;                        # hc read <addr> [--lines n]: 文字だけの画面
  attach) [ "\$2" = --observe ] && cat \$d/ansi-\$3 2>/dev/null ;;  # hc attach --observe <addr>: ANSI の画面
  label) echo "\$2 \$3" >> \$d/labels ;;
  run) n=\$(( \$(cat \$d/next 2>/dev/null || echo 9) )); echo \$((n + 1)) > \$d/next
       task=\$(printf '%s\n' "\$@" | sed -n 's/^issue=//p')
       echo "t\$n \$task" >> \$d/live
       printf '{"address":"t%s","first_message":%s}\n' "\$n" "\$(cat \$d/first 2>/dev/null || echo '{"sent":true,"started":true}')" ;;
  send) [ "\$2" != --if ] || shift 2; [ ! -f "\$d/sendfail" ] || ! grep -qx "\$2" \$d/sendfail || exit 1
        printf '%s\t%s\n' "\$2" "\$3" >> \$d/sent ;;
  ls) task=\$(printf '%s\n' "\$@" | sed -n 's/^issue=//p')
      awk -v t="\$task" '\$2 == t { printf "%s{\"address\":\"%s\",\"state\":\"live\"}", (n++ ? "," : ""), \$1 }' \$d/live | { printf '{"terminals":['; cat; printf ']}\n'; } ;;
  kill|keep) ;;
  rm) sed -i "/^\$2 /d" \$d/live ;;
  *) exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH"
echo "t5 chief-of-staff" > "$tmp/hc/live"
# dispatcher の端末（Chief of Staff のフックの drain が中身を確かめる。terminal.go）。中身は andon dispatcher
echo "dispatcher role" >> "$tmp/hc/live"
fg_dispatcher() { echo '{"address":"dispatcher","state":"live","foreground":{"name":"andon","argv":["'"$andon"'","dispatcher"]}}' > "$tmp/hc/term-dispatcher.json"; }
fg_dispatcher

# セルの repo（人のもの）: 条件表（squash・検査）と委譲（merge B）を owner が commit する
git init -q "$tmp/plant"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
mkdir -p "$tmp/plant/projects/demo"
echo '{"version": 1, "project": {"title": "Demo", "prefix": "D"}, "repos": ["github.com/example/app"], "delegation": {"version": 1, "project": "demo", "preset": "work"}}' > "$tmp/plant/projects/demo/project.json"
as_owner git -C "$tmp/plant" add -A; as_owner git -C "$tmp/plant" commit -qm plant
cd "$tmp/plant/projects/demo"
kanban init --prefix D --repo github.com/example/app >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t5 >/dev/null
L=$PWD/kanban
dispatcher() { "$andon" dispatcher --once --project demo 2>>"$tmp/dispatcher.log"; }
state=$tmp/state/andon/dispatcher/demo.json
pending() { jq -c --arg t "$1" --arg k "$2" '[.pending[] | select(.issue == $t and .kind == $k)] | last' "$state"; }

# Chief of Staff が課題を切る（t5 が予約キーを書く）。担当 t7 が worktree で作業し、PR 7 を出す
newtask() { # newtask <題> <merge> → id
    local id; id=$(HC_TERMINAL_ID=t5 kanban add "$1" --profile claude --repo github.com/example/app)
    HC_TERMINAL_ID=t5 kanban set "$id" review=on must_fix=major merge="$2" >/dev/null
    git -C "$app" fetch -q origin
    git -C "$app" worktree add -q -b "demo/$id" "$HOME/.herdr/worktrees/app/demo-$id" origin/main
    HC_TERMINAL_ID=t5 kanban set "$id" "branch=demo/$id" "worktree=$HOME/.herdr/worktrees/app/demo-$id" >/dev/null  # andon issue start の代わり
    (cd "$HOME/.herdr/worktrees/app/demo-$id" && HC_TERMINAL_ID=t7 kanban start "$id" >/dev/null)
    echo "t7 $id" >> "$tmp/hc/live"
    echo "$id"
}
work() { # work <id> <path>: 担当が commit して push し、PR 7 の事実をその head にする
    local wt=$HOME/.herdr/worktrees/app/demo-$1
    mkdir -p "$wt/$(dirname "$2")"; echo "x$RANDOM" >> "$wt/$2"
    git -C "$wt" add -A; git -C "$wt" commit -qm "$2"; git -C "$wt" push -q origin "HEAD:demo/$1"
    head=$(git -C "$wt" rev-parse HEAD)
    jq -n --arg h "$head" --arg b "demo/$1" '{state:"OPEN",isDraft:false,headRefName:$b,headRefOid:$h,baseRefName:"main",
      mergeStateStatus:"CLEAN",author:{login:"plant-bot"},changedFiles:1,isCrossRepository:false}' > "$tmp/gh/view.json"
    jq -n --arg p "$2" '[[{filename:$p,additions:1,deletions:0}]]' > "$tmp/gh/files.json"
    printf '+++ b/%s\n+x\n' "$2" > "$tmp/gh/diff"
    rm -f "$tmp/gh/merged"
}
restart() { (cd "$HOME/.herdr/worktrees/app/demo-$1" && HC_TERMINAL_ID=t7 kanban start "$1" >/dev/null); }  # 直しが届いた担当
as_worker() { (cd "$HOME/.herdr/worktrees/app/demo-$1" && HC_TERMINAL_ID=t7 "$andon" issue "$2" "$1" "${@:3}" <<<'## Report' >/dev/null); }
review_by() { # review_by <レビュアーの id> <findings>: レビュアーが findings.json を書いて review にする
    mkdir -p "$L/issues/$1/library"
    jq -n --arg c "$head" --argjson f "$2" '{commit: $c, findings: $f}' > "$L/issues/$1/library/findings.json"
    HC_TERMINAL_ID=t9 kanban review "$1" >/dev/null
}

# 0. Chief of Staff の端末が生きていなければ台帳を飛ばす。初めて見る台帳は通知の元を控えるだけだが、今の review_request は拾う
id=$(newtask '初めて見た台帳のレビュー依頼' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
sed -i '/^t5 /d' "$tmp/hc/live"
dispatcher
[ ! -e "$state" ] || die 'Chief of Staff がいないのに回った'
echo "t5 chief-of-staff" >> "$tmp/hc/live"
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die "初めて見た台帳の済んでいない review_request を拾わない: $(grep ^run "$tmp/hc/log")"
jq -e --arg k "$id@$head" '(.reviewers | length) == 1 and .reviewers[$k].started == true and ([.pending[].kind] == ["review-requested"])' "$state" >/dev/null ||
    die "初めて見た台帳で控え・通知が違う（report・ready を流した）: $(jq -c '{reviewers, pending}' "$state")"
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '同じ review_request で二度起こした'
ok 'Chief of Staff の端末が無ければ飛ばし、初めて見る台帳でも済んでいない review_request は拾う（過去の report・ready は流さない）'

# 0b. 控えを無くしても（初めて見た台帳と同じ）、台帳にその commit のレビュアーがもういれば二重に起こさない
cp "$state" "$tmp/state.bak"; rm "$state"
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '控えを無くしたあと、もういるレビュアーを二重に起こした'
grep -q "$id のレビュー（commit ${head:0:12}）は .*がもう受けている" "$tmp/dispatcher.log" || die '引き継いだことがログに無い'
cp "$tmp/state.bak" "$state"
ok '控えに無い review_request でも、台帳にその commit のレビュアーの子課題があれば引き継ぐ（二重に起こさない）'

# 0c. 前の版の控え（初めて見た台帳で控えただけの回）と、起こさなかった回を同じ commit で頼み直したとき
id=$(newtask '止まっていた間のレビュー依頼' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
HC_TERMINAL_ID=t5 kanban set "$id" review=off >/dev/null
HC_TERMINAL_ID=t5 kanban set "$id" review=on >/dev/null
as_worker "$id" review-request
HC_TERMINAL_ID=t5 kanban set "$id" review=off >/dev/null
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die 'review が off なのに起こした'
jq -e --arg k "$id@$head" '.reviewers[$k].skipped | startswith("issue.yaml の review が on でない")' "$state" >/dev/null || die '起こさなかった回を控えていない'
HC_TERMINAL_ID=t5 kanban set "$id" review=on >/dev/null
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '頼み直していないのに起こし直した'
as_worker "$id" review-request   # 同じ commit で頼み直す
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '起こさなかった回を同じ commit で頼み直したのに拾わない'
as_worker "$id" review-request
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '起こした回を同じ commit で頼み直したら二重に起こした'
ok '同じ commit の出し直しは、前回が起こさずに終わっていれば拾い、起こしていれば二重に起こさない'

# 1. 一周: review-request → レビュアーを起こす → レビュー → ready → ゲート → マージ → finish
id=$(newtask 'docs を直す' auto)
mark=$(wc -l < "$L/log.jsonl")
# 担当 t7 は andon issue start のラベルで App の課題の Space の tab にいる（ADR-0003）。レビュアーは同じ値を付ける
echo '{"address":"t7","state":"live","labels":{"issue":"'"$id"'","herdr.space":"andon-demo-'"$id"'","herdr.tab":"andon-demo-'"$id"'","where":"x / y"}}' > "$tmp/hc/term-t7.json"
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
[ -n "$rev" ] || die 'レビュアーの課題が無い'
[ "$rev" = "$id-r1" ] || die "レビュアーの課題の id が <親>-r<回> でない: $rev"
raddr=$(jq -r --arg k "$id@$head" '.reviewers[$k].address' "$state")
kanban show "$rev" --json | jq -e --arg p "$id" --arg a "$raddr" '.issue.profile == "codex-review" and .issue.status == "doing" and .issue.agent == $a and ($a | startswith("t")) and .issue.parent == $p' >/dev/null ||
    die "レビュアーの課題の項目が違う: $(kanban show "$rev" --json)"
jq -e --arg r "$rev" 'select(.id == $r and has("change") and .change.add != null) | .by == "terminal:dispatcher"' "$L/log.jsonl" >/dev/null || die 'レビュアーの課題の作成が dispatcher でない'
rwt=$HOME/.herdr/worktrees/app/demo-$id-r1
[ "$(git -C "$rwt" rev-parse HEAD)" = "$head" ] && git -C "$app" worktree list --porcelain | grep -qx "worktree $rwt" || die 'detached worktree が head に無い'
# 台帳（$tmp/plant/projects/demo）はセルの git の中にあるが、レビュアーの課題の作業場所は detached worktree（branch は無い）
kanban show "$rev" --json | jq -e '.issue.worktree == "~/.herdr/worktrees/app/demo-'"$id"'-r1" and (.issue.branch // "") == ""' >/dev/null ||
    die "レビュアーの課題の worktree・branch が detached worktree でない（セルの本体を書いた）: $(kanban show "$rev" --json | jq -c '.issue | {worktree, branch}')"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$(echo "$rev" | tr A-Z a-z) .*--cwd $L/issues/$rev codex-review" "$tmp/hc/log" || die "hc run の形が違う: $(grep ^run "$tmp/hc/log")"
grep -q "hc send t7 " "$L/issues/$rev/brief.md" && grep -q 'Chief of Staff・dispatcher へは送らない' "$L/issues/$rev/brief.md" || die 'レビュアーの依頼書に宛先（担当 t7）が無い'
grep -q "commit $head" "$L/issues/$rev/brief.md" && ! grep -q '## Report' "$L/issues/$rev/brief.md" || die 'レビュアーの依頼書に commit が無い・担当の報告が入った'
pending "$id" review-requested | jq -e --arg r "$rev" '.detail == {round: 1, reviewer: "codex-review", reviewer_issue: $r}' >/dev/null || die '通知 review-requested が無い'
grep "^run " "$tmp/hc/log" | grep -q -- "--label issue=$rev " && ! grep "^run " "$tmp/hc/log" | grep -q -- '--label task=' ||
    die "レビュアーの端末のラベルが issue=<親>-r<回> でない（task= を書いた。ADR-0008）: $(grep ^run "$tmp/hc/log")"
pending "$id" review-requested | jq -e 'has("issue") and (has("task") | not)' >/dev/null || die '通知のキーが issue でない'
jq -e --arg k "$id@$head" '.reviewers[$k] | has("issue") and has("reviewer_issue") and (has("task") | not)' "$state" >/dev/null || die "控えのキーが issue でない: $(jq -c --arg k "$id@$head" '.reviewers[$k]' "$state")"
emit <(pending "$id" review-requested) notification-review-requested.json
grep "^run " "$tmp/hc/log" | grep -q -- "--label herdr.space=andon-demo-$id --label herdr.tab=andon-demo-$id --cwd " && ! grep "^run " "$tmp/hc/log" | grep -q 'where=' ||
    die "レビュアーに担当の herdr.space・herdr.tab が無い（where を写した）: $(grep ^run "$tmp/hc/log")"
grep -qx "get t7 --json" "$tmp/hc/log" || die '担当の端末を hc get --json で読んでいない'
rm "$tmp/hc/term-t7.json"
ok "review_request でレビュアーを起こす（kanban add --parent・by terminal:dispatcher・detached worktree〈台帳の worktree もそれ・branch 無し〉・hc run --no-notify・担当と同じ herdr.space・herdr.tab・依頼書の宛先は担当）"
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '同じ review_request で二度起こした'
ok '同じ review_request（issue と commit）で二度起こさない'

review_by "$rev" '[{"severity":"minor","file":"docs/a.md","line":1}]'
as_worker "$id" report --data '{"waived":[{"severity":"minor","file":"docs/a.md","line":1,"reason":"起きる条件が無い"}]}'
as_worker "$id" ready
dispatcher
rec=$L/issues/$id/library/gate/$head.json
jq -e '.decision == "approve" and .merged == true and .tier == "B"' "$rec" >/dev/null || die "ゲートの記録が approve・merged でない: $(cat "$rec")"
grep -qx "pr merge 7 --repo example/app --squash --match-head-commit $head" "$tmp/gh/merges" || die 'マージの打ち方が違う'
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null || die '課題が done でない'
kanban show "$rev" --json | jq -e '.issue.status == "done"' >/dev/null || die 'レビュアーの課題が done でない'
[ ! -e "$HOME/.herdr/worktrees/app/demo-$id" ] && [ ! -e "$rwt" ] || die 'worktree が残った'
! git -C "$app" show-ref --verify --quiet "refs/heads/demo/$id" || die 'origin/main に入った branch が残った'
! grep -q " $id\$\| $rev\$" "$tmp/hc/live" || die '端末が残った'
pending "$id" gate | jq -e --arg r "$rec" '.detail == {decision: "approve", reasons: [], record: $r}' >/dev/null || die '通知 gate が無い'
pending "$id" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h}' >/dev/null || die '通知 merged が無い'
emit <(pending "$id" gate) notification-gate.json
emit <(pending "$id" merged) notification-merged.json
emit "$rec" gate-record-merged.json
ok 'review に入ったらゲートを回し、approve ＋ merge: auto ＋ 委譲 B でマージ（squash・head を固定）して finish（レビュアーと worktree も）'
tail -n +"$((mark + 1))" "$L/log.jsonl" | jq -e -s 'all(.by != "terminal:t5")' >/dev/null || die 'Chief of Staff が台帳に書いた'
[ ! -e "$tmp/hc/sent" ] || die "誰かに hc send した: $(cat "$tmp/hc/sent")"
ok 'その間 Chief of Staff（t5）は台帳に1行も書いていない・誰にも hc send していない'

# 1b. マージのあと（done の行）、同じ repo の動いている課題への影響を andon issue impact で1回だけ集めて知らせる
# 0・0c の課題は同じ docs/a.md を別の行で足した branch（doing）なので、ぶつかり・重なる。判断は Chief of Staff（通知だけ）
merged_id=$id
old0=$(kanban list --json | jq -r '.[] | select(.title == "初めて見た台帳のレビュー依頼") | .id')
echo "入力: docs/a.md を読む" >> "$L/issues/$old0/brief.md"
nsent() { if [ -e "$tmp/hc/sent" ]; then wc -l < "$tmp/hc/sent"; else echo 0; fi; }
sent0=$(nsent)
dispatcher
pending "$merged_id" impact | jq -e --arg h "$head" --arg o "$old0" '.detail.pr == "https://github.com/example/app/pull/7" and .detail.sha == $h
  and (.detail.issues | length) == 2 and all(.detail.issues[]; .conflict == true and .overlap == ["docs/a.md"])
  and ([.detail.issues[] | select(.issue == $o) | .brief_refs] == [["docs/a.md"]])' >/dev/null ||
    die "通知 impact が無い・形が違う: $(pending "$merged_id" impact)"
emit <(pending "$merged_id" impact) notification-impact.json
jq -e '(.impact // {}) | length == 0' "$state" >/dev/null || die "済んだ impact が控えに残った: $(jq -c .impact "$state")"
[ "$(nsent)" = "$sent0" ] || die '影響を見て dispatcher が担当に hc send した（判断と依頼は Chief of Staff）'
n0=$(jq '[.pending[] | select(.kind == "impact")] | length' "$state")
dispatcher
[ "$(jq '[.pending[] | select(.kind == "impact")] | length' "$state")" = "$n0" ] || die '同じマージの影響を二度知らせた'
ok 'マージで done になったら andon issue impact を1回打ち、ぶつかる・重なる・依頼書が指す動いている課題を通知 impact にする（担当には送らない）'
# 1c. 影響を集められない（gh が落ちる）ときは次の見回りで打ち直し、3 回で諦めて Chief of Staff に error の impact を積む
jq --arg m "$merged_id" '.impact = {($m): 0}' "$state" > "$state.new" && mv "$state.new" "$state"
touch "$tmp/gh/impactfail"
dispatcher; dispatcher
jq -e --arg m "$merged_id" '.impact[$m] == 2' "$state" >/dev/null || die "失敗の回数を控えていない: $(jq -c .impact "$state")"
pending "$merged_id" impact | jq -e 'has("detail") and (.detail | has("error") | not)' >/dev/null || die '3 回の前に諦めた'
dispatcher
pending "$merged_id" impact | jq -e '.detail.pr == "https://github.com/example/app/pull/7" and (.detail.error | contains("3 回打って失敗")) and (.detail | has("issues") | not)' >/dev/null ||
    die "諦めたのに Chief of Staff に知らせない: $(pending "$merged_id" impact)"
jq -e '(.impact // {}) | length == 0' "$state" >/dev/null || die '諦めたあとも控えに残った'
emit <(pending "$merged_id" impact) notification-impact-error.json
rm "$tmp/gh/impactfail"
ok '影響を集められなければ次の見回りで打ち直し、3 回で諦めて Chief of Staff に error の impact を積む（ログだけにしない）'

# 1d. 人の手直し: done の行ごとに1回だけ library/corrections.json を書く（書き手は dispatcher。contract.md §14）
# 1 の課題は推奨どおり・人の commit もコメントも無く、waived は must_fix（major）より軽い → 空の配列で書いてある
cj() { echo "$L/issues/$1/library/corrections.json"; }
jq -e '.corrections == [] and (.at | test("^[0-9-]{10}T[0-9:]{8}Z$"))' "$(cj "$merged_id")" >/dev/null || die "手直しの無い課題に空の配列を書いていない: $(cat "$(cj "$merged_id")" 2>/dev/null)"
[ ! -e "$(cj "$merged_id-r1")" ] || die 'レビュアーの課題に corrections.json を書いた'
jq -e '(.corrections // {}) | length == 0' "$state" >/dev/null || die "済んだ corrections が控えに残った: $(jq -c .corrections "$state")"
emit "$(cj "$merged_id")" corrections-empty.json
ok '手直しの無い課題は finish（done）のあと corrections.json に空の配列を書く（レビュアーの課題には書かない）'
cos_done() { HC_TERMINAL_ID=t5 kanban done "$1" >/dev/null; }   # Chief of Staff が片付ける（done の行）
# 1d-1. 推奨と違う答え → answer
id=$(newtask '書き出しの形式を決める' person)
(cd "$HOME/.herdr/worktrees/app/demo-$id" && HC_TERMINAL_ID=t7 "$andon" issue ask "$id" - >/dev/null <<'JSON'
{"kind": "users", "question": "書き出しは CSV だけか", "options": [{"label": "CSV だけ", "pick": true}, {"label": "CSV と Excel"}], "from": "issue"}
JSON
)
HC_TERMINAL_ID=t5 kanban answer "$id#1" "CSV と Excel" >/dev/null
(cd "$HOME/.herdr/worktrees/app/demo-$id" && HC_TERMINAL_ID=t7 kanban received "$id#1" >/dev/null)
cos_done "$id"; dispatcher
jq -e '.corrections == [{kind: "answer", question: 1, decision_kind: "users", recommended: "CSV だけ", picked: "CSV と Excel"}]' "$(cj "$id")" >/dev/null ||
    die "推奨と違う答えが answer の1件になっていない: $(cat "$(cj "$id")" 2>/dev/null)"
emit "$(cj "$id")" corrections-answer.json
# 同じ finish を2度拾っても書き直さない（控えに戻しても、在る corrections.json はそのまま）
before=$(cat "$(cj "$id")")
jq --arg m "$id" '.corrections = {($m): 0}' "$state" > "$state.new" && mv "$state.new" "$state"
sleep 1; dispatcher
[ "$(cat "$(cj "$id")")" = "$before" ] || die '同じ finish を2度拾って corrections.json を書き直した'
jq -e '(.corrections // {}) | length == 0' "$state" >/dev/null || die '書いてある課題が控えに残った'
ok '問いの答えが推奨案と違えば answer を1件書く。同じ finish を2度拾っても書き直さない'
# 1d-2. 担当の最後の報告より後に人が積んだ commit → pr
id=$(newtask '見出しを直す' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
human=$(printf 'f%.0s' $(seq 40))
jq -n --arg h "$head" --arg x "$human" '{state: "MERGED", commits: [{oid: $h, messageHeadline: "docs/a.md"}, {oid: $x, messageHeadline: "人が直した"}],
  comments: [{author: {login: "plant-bot"}, body: "andon gate: **approve**（commit x）", createdAt: "2026-11-09T00:00:00Z"}]}' > "$tmp/gh/prc.json"
cos_done "$id"; dispatcher
jq -e --arg x "$human" '.corrections == [{kind: "pr", comments: [], human_commits: [{sha: $x, headline: "人が直した", files: ["docs/b.md"]}]}]' "$(cj "$id")" >/dev/null ||
    die "人の commit が pr の1件になっていない（ゲートのコメントを数えた？）: $(cat "$(cj "$id")" 2>/dev/null)"
emit "$(cj "$id")" corrections-pr.json
rm "$tmp/gh/prc.json"
ok 'PR に担当の最後の報告より後の commit があれば pr を1件書く（ゲートのコメントは数えない）'
# 1d-2b. インラインのレビューコメントだけがある PR → pr（Bot のものは数えない。空の配列にしない）
id=$(newtask '用語を揃える' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
jq -n '[[{user: {login: "o", type: "User"}, body: "ここは台帳の語にする", created_at: "2026-11-09T01:00:00Z", path: "docs/a.md", line: 1},
        {user: {login: "ci", type: "Bot"}, body: "lint", created_at: "2026-11-09T01:01:00Z", path: "docs/a.md", line: 1}]]' > "$tmp/gh/inline.json"
cos_done "$id"; dispatcher
jq -e '.corrections == [{kind: "pr", comments: [{author: "o", at: "2026-11-09T01:00:00Z", body: "ここは台帳の語にする", file: "docs/a.md", line: 1}], human_commits: []}]' "$(cj "$id")" >/dev/null ||
    die "インラインのレビューコメントが pr の1件になっていない: $(cat "$(cj "$id")" 2>/dev/null)"
emit "$(cj "$id")" corrections-pr-inline.json
rm "$tmp/gh/inline.json"
ok 'インラインのレビューコメントだけの PR でも pr を1件書く（Bot のコメントは数えない）'
# 1d-3. must_fix 以上の指摘を waive してマージした → override
id=$(newtask '例外を足す' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7 --data '{"waived":[{"severity":"major","file":"docs/a.md","line":1,"reason":"人がこのままでよいと言った"}]}'
echo "$head" > "$tmp/gh/merged"
cos_done "$id"; dispatcher
jq -e '.corrections == [{kind: "override", finding: {severity: "major", file: "docs/a.md", line: 1}, reason: "人がこのままでよいと言った"}]' "$(cj "$id")" >/dev/null ||
    die "人が waive した指摘が override の1件になっていない: $(cat "$(cj "$id")" 2>/dev/null)"
emit "$(cj "$id")" corrections-override.json
ok 'must_fix 以上の waived がマージされたら override を1件書く'
# 1d-4. gh が落ちたら書かず、次の見回りで1回だけ打ち直す。それも落ちたら書かずに通知 corrections を積む
id=$(newtask '表を足す' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
touch "$tmp/gh/prcfail"
cos_done "$id"; dispatcher
[ ! -e "$(cj "$id")" ] || die 'gh が落ちたのに corrections.json を書いた（集めていないのに「無かった」にした）'
jq -e --arg m "$id" '.corrections[$m] == 1' "$state" >/dev/null || die "失敗の回数を控えていない: $(jq -c .corrections "$state")"
rm "$tmp/gh/prcfail"; dispatcher
jq -e '.corrections == []' "$(cj "$id")" >/dev/null || die '次の見回りで打ち直していない'
rm "$(cj "$id")"
jq --arg m "$id" '.corrections = {($m): 0}' "$state" > "$state.new" && mv "$state.new" "$state"
touch "$tmp/gh/prcfail"; dispatcher; dispatcher
[ ! -e "$(cj "$id")" ] || die '2 回とも落ちたのに書いた'
pending "$id" corrections | jq -e '.detail.error | contains("2 回打って失敗")' >/dev/null || die "諦めたのに Chief of Staff に知らせない: $(pending "$id" corrections)"
jq -e '(.corrections // {}) | length == 0' "$state" >/dev/null || die '諦めたあとも控えに残った'
emit <(pending "$id" corrections) notification-corrections.json
rm "$tmp/gh/prcfail"
ok 'gh が落ちたら書かずに次の見回りで1回だけ打ち直し、それも落ちたら書かずに Chief of Staff に通知 corrections を積む'

# 2. fix は担当へ hc send（Must fix が残る）
id=$(newtask '説明を足す' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
review_by "$rev" '[{"severity":"major","file":"docs/a.md","line":1}]'
as_worker "$id" ready
dispatcher
grep -q "^t7	$id（説明を足す）: ゲートが fix を返した。理由: Must fix 以上が 1 件" "$tmp/hc/sent" || die "fix が担当に届いていない: $(cat "$tmp/hc/sent" 2>/dev/null)"
[ ! -e "$tmp/gh/merged" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'fix なのにマージした・片付けた'
dispatcher
[ "$(grep -c "	$id（" "$tmp/hc/sent")" = 1 ] || die '同じ review 入りで二度ゲートを回した'
ok 'fix は理由を担当（issue.yaml の agent）へ hc send し、同じ review 入りでは二度回さない'

# 3. needs_human は問い（from: gate）。レベル C は委譲 merge B の外
restart "$id"
work "$id" package/gate.json
as_worker "$id" report
as_worker "$id" review-request
dispatcher
rev=$(kanban list --all --json | jq -r --arg p "$id" '[.[] | select(.parent == $p)] | last | .id')
[ "$rev" = "$id-r2" ] || die "2 回目のレビュアーの課題の id が $id-r2 でない: $rev"
kanban list --all --json | jq -e '[.[].id | select(test("^D-[0-9]+$")) | ltrimstr("D-") | tonumber] | sort == [range(1; length + 1)]' >/dev/null &&
  kanban list --all --json | jq -e 'all(.[]; (.id | test("^D-[0-9]{4}(-r[1-9][0-9]*)?$")) and ((.id | test("-r")) == (.profile == "codex-review")))' >/dev/null ||
    die "レビュアーの課題が通し番号を使った・id の形が違う: $(kanban list --all --json | jq -c 'map([.id, .profile])')"
review_by "$rev" '[]'
as_worker "$id" ready
dispatcher
q=$L/issues/$id/library/questions/1.json
jq -e '.from == "gate" and .kind == "line" and (.facts | any(contains("delegation: merge=B < tier C"))) and .options[0].pick == true' "$q" >/dev/null || die "問いが無い・形が違う: $(cat "$q" 2>/dev/null)"
kanban questions --json 2>/dev/null | jq -e --arg t "$id" 'any(.[]; .id == $t)' >/dev/null || kanban questions | grep -q "$id" || die '台帳に問いが無い'
pending "$id" asked | jq -e '.detail.n == 1 and .detail.kind == "line"' >/dev/null || die '通知 asked が無い'
[ ! -e "$tmp/gh/merged" ] || die 'needs_human なのにマージした'
emit "$q" question-gate.json
emit <(pending "$id" asked) notification-asked.json
ok 'needs_human は問い（from: gate・kind line）を台帳に出し、マージしない'
ok 'レビュアーの課題の id は <親>-r<回>（1 回目 r1・2 回目 r2）で、通し番号を使わない'

# 4. merge: person は approve でも止まる（通知 gate だけ）
id=$(newtask '人がマージする' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
review_by "$rev" '[]'
as_worker "$id" ready
dispatcher
pending "$id" gate | jq -e '.detail.decision == "approve"' >/dev/null || die '通知 gate が無い'
[ ! -e "$tmp/gh/merged" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'merge: person なのにマージした'
ok 'merge: person は approve でもマージせず、通知 gate で止まる（人がマージする）'

# 4b. 人が GitHub でマージした（merge: person・needs_human）。dispatcher は review の課題の PR を1分に1回だけ見て、MERGED なら
#     ゲートの問い（from: gate。重なったものも）を「人がマージした」と閉じ、andon issue finish し、通知 merged（by: person）を積む。
#     次の巡で影響（impact）と人の手直し（corrections）も dispatcher がマージしたときと同じ道で拾う。dispatcher.md §2.7
watch_due() { # watch_due <id>: その課題だけ、次の見回りで GitHub に聞く（ほかの review の課題は聞かない）
    jq --arg t "$1" '.watch |= with_entries(.value.at = (if .key == $t then "2000-01-01T00:00:00Z" else "2999-01-01T00:00:00Z" end))' "$state" > "$state.new" && mv "$state.new" "$state"
}
person_merges() { git -C "$tmp/origin" update-ref refs/heads/main "$head"; echo "$head" > "$tmp/gh/merged"; }
hm=$(newtask '人がマージする課題' person)
work "$hm" package/gate.json
as_worker "$hm" report --pr https://github.com/example/app/pull/7
as_worker "$hm" review-request
dispatcher
review_by "$(kanban list --all --json | jq -r --arg p "$hm" '[.[] | select(.parent == $p)] | last | .id')" '[]'
as_worker "$hm" ready
dispatcher
jq -e '.from == "gate"' "$L/issues/$hm/library/questions/1.json" >/dev/null || die '前提: needs_human の問いが無い'
HC_TERMINAL_ID=dispatcher "$andon" issue ask "$hm" - >/dev/null <<<'{"kind":"irreversible","question":"重なったゲートの問い","options":[{"label":"人が見る","pick":true},{"label":"x"}],"from":"gate"}'
HC_TERMINAL_ID=t5 kanban answer "$hm#2" '人が見る' >/dev/null   # answered のまま（received でない）
n=$(wc -l < "$tmp/gh/states")
watch_due "$hm"; dispatcher; dispatcher
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] || die "1分たたないうちに同じ PR を GitHub に聞き直した: $(( $(wc -l < "$tmp/gh/states") - n )) 回"
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die 'OPEN の PR の課題を片付けた'
merges=$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)
person_merges
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "人がマージしたのに片付けない: $(tail -3 "$tmp/dispatcher.log")"
kanban questions --json | jq -e --arg t "$hm" 'all(.[]; .id != $t)' >/dev/null || die "ゲートの問いが閉じていない: $(kanban questions --json | jq -c --arg t "$hm" 'map(select(.id == $t))')"
grep -q '人がマージした（https://github.com/example/app/pull/7）' "$L/issues/$hm/questions.yaml" || die 'ゲートの問いに「人がマージした」と記録していない'
pending "$hm" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}' >/dev/null || die "通知 merged（by: person）が無い: $(pending "$hm" merged)"
[ "$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)" = "$merges" ] || die '人がマージする課題に gh pr merge を打った'
jq -e --arg t "$hm" '(.watch // {}) | has($t) | not' "$state" >/dev/null || die '片付けたあとも控えに残った'
emit <(pending "$hm" merged) notification-merged-person.json
dispatcher
jq -e --arg t "$hm" '((.impact // {}) | has($t) | not) and ((.corrections // {}) | has($t) | not)' "$state" >/dev/null || die "人のマージのあと impact・corrections を拾っていない: $(jq -c '{impact, corrections}' "$state")"
[ -e "$(cj "$hm")" ] || die '人のマージのあと corrections.json を書いていない'
ok '人が GitHub でマージしたら、次の見回りでゲートの問い（重なったものも）を閉じて finish し、通知 merged（by: person）・impact・corrections を同じ道で回す。PR は1分に1回だけ見る'

# 4c. 人がマージしたが、ゲートの外の問い（担当の ask）が開いている → 片付けずに知らせる（1回）。問いが閉じたら次の巡で片付ける
hm=$(newtask '問いが残る課題' person)
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
(cd "$HOME/.herdr/worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 "$andon" issue ask "$hm" - >/dev/null <<<'{"kind":"users","question":"どちらにする","options":[{"label":"A","pick":true},{"label":"B"}],"from":"issue"}')
(cd "$HOME/.herdr/worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 kanban review "$hm" >/dev/null)
HC_TERMINAL_ID=t5 kanban set "$hm" review=off >/dev/null
dispatcher
person_merges
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die 'ゲートの外の問いが開いているのに片付けた'
pending "$hm" merged | jq -e --arg q "$hm#1" '.detail.by == "person" and (.detail.held | contains($q))' >/dev/null || die "片付けない理由を知らせていない: $(pending "$hm" merged)"
emit <(pending "$hm" merged) notification-merged-held.json
jq --arg t "$hm" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
n=$(wc -l < "$tmp/gh/states"); dispatcher
[ "$(pending "$hm" merged)" = null ] || die '同じ理由で二度知らせた'
[ "$(wc -l < "$tmp/gh/states")" = "$n" ] || die 'MERGED と分かった PR を GitHub に聞き直した'
HC_TERMINAL_ID=t5 kanban answer "$hm#1" A >/dev/null; HC_TERMINAL_ID=t7 kanban received "$hm#1" >/dev/null
dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "問いが閉じたのに片付けない: $(tail -3 "$tmp/dispatcher.log")"
pending "$hm" merged | jq -e '.detail.by == "person" and (.detail | has("held") | not)' >/dev/null || die "片付けた通知が無い: $(pending "$hm" merged)"
ok '人がマージしてもゲートの外の問いが開いていれば片付けずに1回だけ知らせ、問いが閉じたら次の巡で片付ける（GitHub には聞き直さない）'

# 4d. PR がマージされずに閉じた → 片付けず、通知 closed を1回だけ
hm=$(newtask '閉じた PR の課題' person)
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
(cd "$HOME/.herdr/worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 kanban review "$hm" >/dev/null)
HC_TERMINAL_ID=t5 kanban set "$hm" review=off >/dev/null
dispatcher
touch "$tmp/gh/closed"
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die '閉じた PR の課題を片付けた'
pending "$hm" closed | jq -e '.detail == {pr: "https://github.com/example/app/pull/7"}' >/dev/null || die "通知 closed が無い: $(pending "$hm" closed)"
emit <(pending "$hm" closed) notification-closed.json
jq --arg t "$hm" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
n=$(wc -l < "$tmp/gh/states"); watch_due "$hm"; dispatcher
[ "$(pending "$hm" closed)" = null ] && [ "$(wc -l < "$tmp/gh/states")" = "$n" ] || die '閉じた PR を二度知らせた・聞き直した'
rm "$tmp/gh/closed"
HC_TERMINAL_ID=t5 "$andon" issue finish "$hm" >/dev/null   # 前の節の課題と同じ PR 7 を使うので、ここで片付ける
ok 'PR がマージされずに閉じたら片付けず、通知 closed を1回だけ積む（もう聞き直さない）'

# 5. 上限を超えた回（extra）は起こさない（Chief of Staff が決める）
HC_TERMINAL_ID=t5 kanban set "$id" review_rounds=1 >/dev/null
restart "$id"
work "$id" docs/a.md
as_worker "$id" report
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '上限を超えた回でレビュアーを起こした'
pending "$id" report | jq -e '.detail.report | endswith("/report.md")' >/dev/null || die '通知 report が無い'
grep -q '追加の回（2 回目／上限 1）' "$tmp/dispatcher.log" || die '起こさない理由がログに無い'
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '上限を超えた回を二度目の見回りで起こした'
HC_TERMINAL_ID=t5 kanban set "$id" review_rounds=2 >/dev/null
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die 'Chief of Staff が review_rounds を上げたのに起こさない'
ok '上限を超えた回（extra）は起こさず通知 report。Chief of Staff が review_rounds を上げたら次の見回りで起こす'

# 5b. deliverable: candidates（調べる課題。Chief of Staff が書いた）はインナーループを通らない: review_request があってもレビュアーを起こさず、
#     review に入っても andon gate を回さない（通知は届ける）。ADR-0006 受け入れ条件 3
sid=$(newtask '誤りの候補を調べる' person)
HC_TERMINAL_ID=t5 kanban set "$sid" deliverable=candidates >/dev/null
mkdir -p "$L/issues/$sid/library"
jq '.review_request' -e "$L/issues/$id/library/report.json" >/dev/null || die '写す review_request が無い'
cp "$L/issues/$id/library/report.json" "$L/issues/$sid/library/report.json"   # 形の合う review_request つきの報告
runs=$(grep -c '^run' "$tmp/hc/log"); posted=$(grep -c '^status' "$tmp/gh/posted")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die 'candidates の課題の review_request でレビュアーを起こした'
(cd "$HOME/.herdr/worktrees/app/demo-$sid" && HC_TERMINAL_ID=t7 kanban review "$sid" >/dev/null)
dispatcher
[ "$(grep -c '^status' "$tmp/gh/posted")" = "$posted" ] && [ ! -e "$L/issues/$sid/library/gate" ] || die 'candidates の課題にゲートを回した'
jq -e --arg t "$sid" '[.gates | to_entries[] | select(.value.issue == $t)] == []' "$state" >/dev/null || die "candidates の課題のゲートを控えた: $(jq -c .gates "$state")"
[ "$(pending "$sid" review)" != null ] || [ "$(pending "$sid" report)" != null ] || die "candidates の課題の review の通知が無い: $(jq -c .pending "$state")"
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die 'candidates の課題でレビュアーを起こした'
ok 'deliverable: candidates の課題は review_request でレビュアーを起こさず、review に入ってもゲートを回さない（通知は届ける）'

# 6. andon issue finish（人がマージしたあと Chief of Staff が打つ）はレビュアーの子課題と detached worktree も片付ける
revs=$(kanban list --all --json | jq -r --arg p "$id" '[.[] | select(.parent == $p and .status != "done")] | length')
[ "$revs" -ge 2 ] && ls -d "$HOME/.herdr/worktrees/app/demo-$id-r"* >/dev/null || die 'レビュアーの子課題・detached worktree が無い（前提）'
HC_TERMINAL_ID=t5 "$andon" issue finish "$id" >/dev/null
kanban list --all --json | jq -e --arg p "$id" 'all(.[] | select(.id == $p or .parent == $p); .status == "done")' >/dev/null || die '子課題が done でない'
! ls -d "$HOME/.herdr/worktrees/app/demo-$id"* >/dev/null 2>&1 || die 'worktree が残った'
git -C "$app" show-ref --verify --quiet "refs/heads/demo/$id" || die 'マージしていない branch を消した'
ok 'andon issue finish はレビュアーの子課題と detached worktree も片付け、origin に入っていない branch は残す'
dispatcher
grep -q "$id の PR はマージされていないので影響を見ない" "$tmp/dispatcher.log" || die 'Chief of Staff が打った finish（done）を拾って andon issue impact を打っていない'
ok 'Chief of Staff が打った finish（done）も拾い、PR が MERGED でなければ影響を知らせない'

# 6b. 台帳の worktree が repo の本体の checkout（git worktree list の先頭）を指していたら、finish は何も消さずに止まる
id=$(HC_TERMINAL_ID=t5 kanban add '本体を指す' --profile claude --repo github.com/example/app)
HC_TERMINAL_ID=t5 kanban set "$id" "worktree=$app" branch=main >/dev/null
if out=$(HC_TERMINAL_ID=t5 "$andon" issue finish "$id" 2>&1); then die "本体の checkout を指す課題を finish が片付けた: $out"; fi
grep -q 'repo の本体の checkout' <<<"$out" || die "止まった理由が本体の checkout でない: $out"
[ -d "$app/.git" ] && git -C "$app" worktree list --porcelain | head -1 | grep -qx "worktree $app" || die '本体の checkout が消えた'
kanban show "$id" --json | jq -e '.issue.status != "done"' >/dev/null || die '止まったのに done にした'
ok 'andon issue finish は台帳の worktree が repo の本体の checkout なら、消さずに止まる'

# 7. 最初のメッセージが届かなければ（送れなかった・送ったが動き出さなかった）、端末にラベル first_message.sent=false を付けて
#    起こしたことにし、次の見回りで1回だけ送り直す（担当と同じ。dispatcher.md §5）。課題も端末も二つにしない
for first in '{"sent":false,"reason":"t9 is still starting: its output has not settled"}' '{"sent":true,"started":false,"turn":1}'; do
  id=$(newtask "最初の文が届かない $(jq -r .sent <<<"$first")" auto)
  work "$id" docs/a.md
  as_worker "$id" report --pr https://github.com/example/app/pull/7
  as_worker "$id" review-request
  runs=$(grep -c '^run' "$tmp/hc/log")
  echo "$first" > "$tmp/hc/first"
  dispatcher
  rm "$tmp/hc/first"
  key="$id@$head"
  jq -e --arg k "$key" '.reviewers[$k] | .started == true and .reviewer_issue != null and .address != null' "$state" >/dev/null || die "ラベルを付けたのに起こしたことにならない（$first）: $(jq -c --arg k "$key" '.reviewers[$k]' "$state")"
  addr=$(jq -r --arg k "$key" '.reviewers[$k].address' "$state")
  rev=$(jq -r --arg k "$key" '.reviewers[$k].reviewer_issue' "$state")
  grep -qx "$addr first_message.sent=false" "$tmp/hc/labels" || die "届いていない端末にラベルを付けていない（$first）"
  ! grep -q "^$addr	" "$tmp/hc/sent" 2>/dev/null || die "起こした回に送り直した（$first）"
  # 次の見回り: レビュアーは idle のまま（入力欄が空）。同じ端末へレビュアーの最初のメッセージを1回だけ送り、ラベルを外す
  printf '{"address":"%s","state":"live","status":{"state":"idle"},"status_ms":%s,"turn":1,"labels":{"first_message.sent":"false"}}\n' "$addr" "$(date +%s%3N)" > "$tmp/hc/term-$addr.json"
  dispatcher
  [ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die "二重に起こした（$first）"
  [ "$(grep -c "^$addr	課題 \`demo/$rev\`。" "$tmp/hc/sent")" = 1 ] || die "同じ端末へレビュアーの最初のメッセージを送り直していない（$first）"
  grep -A2 "^$addr	課題" "$tmp/hc/sent" | grep -q "^先に \`.*/skills/worker/SKILL.md\` を読む。" || die "送り直した文がレビュアーの最初のメッセージでない（$first）"
  grep -qx "$addr first_message.sent=" "$tmp/hc/labels" || die "送り直したのにラベルを外していない（$first）"
  # ラベルが外れたので、idle のままでももう送らない（外しそこねたときは控えの first が守る。first_test.go）
  printf '{"address":"%s","state":"live","status":{"state":"idle"},"status_ms":%s,"turn":1,"labels":{}}\n' "$addr" "$(date +%s%3N)" > "$tmp/hc/term-$addr.json"
  dispatcher
  [ "$(grep -c "^$addr	" "$tmp/hc/sent")" = 1 ] || die "二度送った（$first）"
  rm "$tmp/hc/term-$addr.json"
  [ "$(kanban list --all --json | jq --arg p "$id" '[.[] | select(.parent == $p)] | length')" = 1 ] || die "レビュアーの課題を二つ足した（$first）"
done
rm "$tmp/hc/labels" # 後ろの見出し（12）は labels の行を先頭から数える
ok '最初のメッセージが届かなければ（sent=false・sent=true でも動き出さない）ラベルを付け、次の見回りで idle のレビュアーへ1回だけ送り直す（課題も端末も二つにしない）'

# 8. fetch の一時の失敗は次の見回りでやり直す
id=$(newtask 'fetch が一度落ちる' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
insteadof() { git config --global --remove-section "url.file://$2" 2>/dev/null || true; git config --global url."file://$1".insteadOf https://github.com/example/app; }
insteadof "$tmp/nowhere" "$tmp/origin"
dispatcher
insteadof "$tmp/origin" "$tmp/nowhere"
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] && jq -e --arg k "$id@$head" '.reviewers[$k].failed | contains("fetch")' "$state" >/dev/null || die 'fetch に失敗したのに起こした・控えに失敗が無い'
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] && jq -e --arg k "$id@$head" '.reviewers[$k].started == true' "$state" >/dev/null || die 'fetch が戻ったのに起こさない'
! grep '^run ' "$tmp/hc/log" | tail -n 1 | grep -q 'herdr\.' || die "担当の端末のラベルが読めないのに herdr.space・herdr.tab を付けた: $(grep ^run "$tmp/hc/log" | tail -n 1)"
ok 'fetch の一時の失敗は控えで成功と分け、次の見回りで起こす（担当の端末のラベルが読めなければ herdr.space・herdr.tab を付けない）'

# 9. ゲートのあと Chief of Staff が merge: person に変えたら、マージせず次の見回りでゲートから回し直す
gate_through() { # gate_through <id>: レビュアーが指摘なしで返し、担当が ready まで
    local rev; rev=$(kanban list --all --json | jq -r --arg p "$1" '[.[] | select(.parent == $p)] | last | .id')
    review_by "$rev" '[]'
    as_worker "$1" ready
}
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
echo "cd $PWD && HC_TERMINAL_ID=t5 kanban set $id merge=person >/dev/null" > "$tmp/gh/hook"
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'ゲートのあと merge: person に変えたのにマージした'
grep -q "$id をいまはマージしない.*予約キーがゲートのあとに変わった" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die '回し直しでマージした'
[ "$(grep -c "^status .*$head" "$tmp/gh/posted")" = 2 ] || die 'ゲートを回し直していない'
ok 'ゲートのあと merge: person に変わったら gh pr merge を打たず、次の見回りでゲートから回し直す（approve でも person なので止まる）'

# 9b. GitHub の checks が走っている間は待ち（マージしない・人に回さない・控えに書かない）。通れば次の見回りでマージ
id=$(newtask 'CI を待つ' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
checks IN_PROGRESS
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
dispatcher
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'checks が走っているのにマージした'
[ "$(grep -c "$id のゲートは GitHub の checks を待つ: GitHub の checks がまだ走っている: test" "$tmp/dispatcher.log")" = 2 ] || die '見回りのたびにゲートを回し直していない'
jq -e --arg t "$id" '([.gates[] | select(.issue == $t)] | length) == 0 and ([.pending[] | select(.issue == $t and (.kind == "gate" or .kind == "asked"))] | length) == 0' "$state" >/dev/null ||
    die 'wait を控えに書いた・通知 gate か問いを出した'
[ ! -e "$L/issues/$id/library/questions" ] && [ ! -e "$L/issues/$id/library/gate" ] || die 'wait で問い・ゲートの記録を書いた'
checks COMPLETED SUCCESS
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$((merges + 1))" ] && kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null || die 'checks が通ったのにマージしない'
ok 'GitHub の checks が走っている間はゲートが wait。dispatcher は控えに書かず問いも出さず、見回りのたびに回し直し、通ればマージする'

# 9c. ゲートのあと checks が変わった・push されたら、マージの直前に読み直して打たない
id=$(newtask 'CI が走り直す' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
checks IN_PROGRESS; mv "$tmp/gh/rollup.json" "$tmp/gh/rollup-pending.json"; checks COMPLETED SUCCESS
echo "mv $tmp/gh/rollup-pending.json $tmp/gh/rollup.json" > "$tmp/gh/hook"  # ゲートが approve を書いたあとに走り直す
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'ゲートのあと checks が走り直したのにマージした'
grep -q "$id をいまはマージしない.*GitHub の checks がゲートのあとに変わった（pending: test）" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
checks COMPLETED SUCCESS
old=$head
echo "jq --arg h $(printf 'f%.0s' $(seq 40)) '.headRefOid = \$h' $tmp/gh/view.json > $tmp/gh/v && mv $tmp/gh/v $tmp/gh/view.json" > "$tmp/gh/hook"
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'ゲートのあと push されたのにマージした'
grep -q "$id をいまはマージしない.*PR の head がゲートのあとに動いた（${old:0:12} → ffffffffffff）" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
dispatcher
grep -q "^t7	$id（CI が走り直す）: ゲートが fix を返した。理由: .*report.json の commit が PR の head と違う" "$tmp/hc/sent" || die "回し直したゲートが fix を担当へ返していない: $(tail -n 2 "$tmp/hc/sent" 2>/dev/null)"
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die '回し直しでマージした'
ok 'マージの直前に PR の head と GitHub の checks を読み直し、checks が変わった・head が動いたら打たない（回し直したゲートが担当へ fix）'

# 10. ゲートのあとオーナーが委譲の merge を取り消したら、マージしない（回し直したゲートは needs_human）
id=$(newtask '委譲が取り消される' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
gate_through "$id"
echo "cd $tmp/plant/projects/demo && env -u HC_TERMINAL_ID GIT_AUTHOR_NAME=o GIT_AUTHOR_EMAIL=owner@example.com $andon delegate narrow merge none --evidence x >$tmp/hook.out 2>&1" > "$tmp/gh/hook"
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'ゲートのあと委譲を取り消したのにマージした'
grep -q "$id をいまはマージしない.*委譲の merge がゲートのあとに変わった（B → none）" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
dispatcher
jq -e '.from == "gate" and (.facts | any(contains("delegation: merge=none")))' "$L/issues/$id/library/questions/1.json" >/dev/null || die '回し直したゲートの needs_human が問いになっていない'
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'マージした'
ok 'ゲートのあと委譲の merge が取り消されたら gh pr merge を打たず、回し直したゲートの needs_human を問いにする'

# === フックで届ける・止める（P6b。dispatcher.md §3・§4、contract.md §9）===
# フックは package/hooks/ を Claude Code と同じに打つ（stdin にフックの入力の JSON、HC_TERMINAL_ID は自分の端末）
ln -sf "$andon" "$tmp/bin/andon"
hooks=$andon_root/package/hooks
rule=$(printf '─%.0s' $(seq 30))
term() { # term <addr> <状態> <turn> [<k=v>…]: 偽の hc get --json
    local a=$1 s=$2 n=$3; shift 3
    printf '%s\n' "$@" | jq -R 'select(. != "") | split("=") | {(.[0]): .[1]}' | jq -s --arg a "$a" --arg s "$s" --argjson n "$n" \
        '{address: $a, state: "live", status: {state: $s}, turn: $n, labels: (add // {})}' > "$tmp/hc/term-$a.json"
}
cos() { term t5 "$1" "$2" role=chief-of-staff project=demo; }
screen() { # screen <入力欄の文字> <入力欄の ANSI>: Chief of Staff の画面（hc read と attach --observe）
    printf '● 前の答え\n%s\n%s\n%s\n  model │ 12%%\n' "$rule" "$1" "$rule" > "$tmp/hc/screen-t5"
    printf '\033[!p\033[?1049l\033[?1049h\033[2J\033[H● 前の答え\r\n%s\r\n%s\033[0m\r\n%s\r\n  model │ 12%%' "$rule" "$2" "$rule" > "$tmp/hc/ansi-t5"
}
event() { # event <題>: dispatcher が通知 review-requested を積む出来事（担当がレビューを頼む）。id を返す
    local id; id=$(newtask "$1" person)
    work "$id" docs/a.md
    as_worker "$id" report --pr https://github.com/example/app/pull/7
    as_worker "$id" review-request
    dispatcher
    pending "$id" review-requested | jq -e . >/dev/null || die "$1: 通知が積まれていない"
    echo "$id"
}
sent() { cat "$tmp/hc/sent" 2>/dev/null | grep -c '^t5	'; }

# 11. turn 中の通知が Stop で届く（Chief of Staff が working の間、dispatcher は hc send しない）
cos working 3
screen '❯' '❯'
id=$(event 'ターンの中の通知')
[ "$(sent)" = 0 ] || die 'working の Chief of Staff に hc send した'
out=$(HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"見た"}')
jq -e --arg id "$id" '.decision == "block" and (.reason | startswith("<news project=\"demo\">\n") and contains("\n\($id) review-requested: 1 回目のレビュアー ") and endswith("「<no-reply>」とだけ返す。\n</news>"))' <<<"$out" >/dev/null ||
    die "Stop が <news> で block しない: $out"
jq -e '.pending == [] and .hook.terminal == "t5" and .hook.via == "stop"' "$state" >/dev/null || die '取り出した待ちが控えに残った・渡した turn を控えていない'
out=$(HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"stop_hook_active":true,"last_assistant_message":"<no-reply>"}')
[ -z "$out" ] || die "待ちが無いのに block した: $out"
grep -qx 't5 news.replied=3' "$tmp/hc/labels" && jq -e 'has("hook") | not' "$state" >/dev/null || die '<no-reply> だけの答えに news.replied を書かない'
out=$(HC_TERMINAL_ID=t7 sh "$hooks/stop" <<<'{}')
[ -z "$out" ] || die "Chief of Staff でも担当でもない端末で何か出した: $out"
ok 'turn 中の通知は Chief of Staff の Stop フック（drain）が block の reason で渡し、答えが <no-reply> だけなら news.replied を書く'

# 12. 送信時に添えられる（UserPromptSubmit。委譲の一覧も同じ口で）
id=$(event '送信時の通知')
out=$(HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"hook_event_name":"UserPromptSubmit","prompt":"次は？"}')
jq -e --arg id "$id" '.hookSpecificOutput.hookEventName == "UserPromptSubmit" and (.hookSpecificOutput.additionalContext
    | contains("\($id) review-requested:") and contains("<delegation project=\"demo\">\n{\"project\":\"demo\""))' <<<"$out" >/dev/null ||
    die "additionalContext に待ちと委譲が無い: $out"
jq -e '.pending == []' "$state" >/dev/null || die '添えた待ちが残った'
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"last_assistant_message":"了解"}' >/dev/null
! grep -q 'news.replied=3$' <(tail -n +2 "$tmp/hc/labels") || die '人への答えに news.replied を書いた'
out=$(HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"prompt":"もう一つ"}')
jq -e '.hookSpecificOutput.additionalContext | (contains("<news") | not) and contains("<delegation")' <<<"$out" >/dev/null || die "待ちが無いときに委譲だけを添えない: $out"
out=$(HC_TERMINAL_ID=t5 PATH=/usr/bin:/bin sh "$hooks/user-prompt-submit" <<<'{}'); rc=$?
[ "$rc" = 0 ] && [ -z "$out" ] || die 'andon が無いときに通さない'
ok '人が送ったとき、待ちと委譲の一覧を UserPromptSubmit の additionalContext に添える（andon が無ければ通す）'

# 12a. core が起き直して dispatcher の端末の中身がシェルに替わっていたら、Chief of Staff のフックが片付けて起こし直す（シェルには打ち込まない）
echo '{"address":"t3","state":"live","argv":["/bin/sh"],"foreground":{"name":"sh","argv":["/bin/sh"]}}' > "$tmp/hc/term-dispatcher.json"
logs=$(wc -l < "$tmp/hc/log")
HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"prompt":"x"}' >/dev/null
got=$(tail -n +"$((logs + 1))" "$tmp/hc/log" | grep -v '^get \|^ls ')
want="keep t3 --off
kill t3
rm t3
run --bg --no-parent --no-notify --keep --name dispatcher --label role=dispatcher --label herdr.space=andon-demo --label herdr.tab=andon-demo-dispatcher --cwd $tmp/plant -- $andon dispatcher"
[ "$got" = "$want" ] || die "シェルに替わった dispatcher を片付けて起こし直さない: $got"
! grep -q '^dispatcher	' "$tmp/hc/sent" || die 'dispatcher のシェルに打ち込んだ'
fg_dispatcher
logs=$(wc -l < "$tmp/hc/log")
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"last_assistant_message":"x"}' >/dev/null
! tail -n +"$((logs + 1))" "$tmp/hc/log" | grep -q '^run \|^kill \|^rm ' || die '中身が andon dispatcher の dispatcher を起こし直した'
ok 'dispatcher の端末の中身がシェルに替わっていたら、Chief of Staff のフック（drain）が片付けて同じ名前・ラベルで起こし直す（シェルに打ち込まない・動いていれば触らない）'

# 12b. UserPromptSubmit で渡したあと turn が中断された（Stop が来ない）ら、次の送信で1回だけ届け直す。news.replied は渡した turn の答えにだけ
ups() { HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<"{\"session_id\":\"s1\",\"prompt_id\":\"$1\",\"prompt\":\"x\"}" | jq -r '.hookSpecificOutput.additionalContext'; }
cstop() { HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<"{\"session_id\":\"s1\",\"prompt_id\":\"$1\",\"stop_hook_active\":${3:-false},\"last_assistant_message\":\"$2\"}"; }
id=$(event '中断される送信')
labels=$(wc -l < "$tmp/hc/labels")
ups p1 | grep -q "$id review-requested:" || die '1回目の送信で渡さない'
jq -e '.hook.via == "prompt" and .hook.prompt_id == "p1" and (.hook.news | any(.kind == "review-requested"))' "$state" >/dev/null || die '渡した <news> を turn の終わりまで控えに残さない'
# p1 は中断（Stop が来ない）。次の送信で届け直す
ups p2 | grep -q "$id review-requested:" || die '中断された turn の <news> を次の送信で届け直さない'
jq -e '(.hook.news // []) == [] and (.hook.again | any(.kind == "review-requested"))' "$state" >/dev/null || die '届け直した分を控えていない'
# p2 も中断。届け直しは1回だけ
ups p3 | grep -q "review-requested" && die '届け直しを2回した'
cstop p3 '<no-reply>' >/dev/null
[ "$(wc -l < "$tmp/hc/labels")" = "$labels" ] || die '<news> を渡していない turn の <no-reply> に news.replied を書いた'
# 渡した turn と違う turn の Stop では書かない・渡した turn の Stop では書く
id=$(event '照合する送信')
ups p4 >/dev/null
cos working 6
out=$(cstop p0 '<no-reply>')
[ "$(wc -l < "$tmp/hc/labels")" = "$labels" ] || die '別の turn（prompt_id 違い）の Stop で news.replied を書いた'
jq -e --arg id "$id" '.decision == "block" and (.reason | contains("\($id) review-requested:"))' <<<"$out" >/dev/null || die "確かめていない <news> をその Stop で届け直さない: $out"
cstop p0 '<no-reply>' true >/dev/null
grep -qx 't5 news.replied=6' "$tmp/hc/labels" && jq -e 'has("hook") | not' "$state" >/dev/null || die '届け直した turn の <no-reply> に news.replied を書かない'
cstop p0 '<no-reply>' true >/dev/null
[ "$(grep -c 'news.replied' "$tmp/hc/labels")" = 2 ] || die '確かめ済みの <news> に二度 news.replied を書いた'
ok '送信で渡した <news> は turn の Stop まで控えに残し、中断されたら次の送信で1回だけ届け直す。news.replied は prompt_id で照合した turn だけ（違う turn の Stop では書かず届け直す）'

# 13. 離席時だけ hc send（状態 idle・入力欄が空・前の <news> の turn が進んでいる）
id=$(event '離席の通知')
cos idle 4
screen '❯ これを直して' '❯ これを直して'
dispatcher
[ "$(sent)" = 0 ] || die '人の書きかけがあるのに hc send した'
screen '❯' '❯ 前の文'
dispatcher
[ "$(sent)" = 0 ] || die 'hc read と組み直した画面が違うのに hc send した'
screen '❯ Try "fix lint"' $'❯ \e[7m\e[2mTry "fix lint"\e[0m\e[?7h'
dispatcher
[ "$(sent)" = 0 ] || die '許可の一覧に無い列があるのに hc send した'
screen '❯ Try "fix lint"' $'❯ \e[2mTry "fix lint"\e[22m'
cos working 4
dispatcher
[ "$(sent)" = 0 ] || die 'working の Chief of Staff に hc send した'
cos idle 4
dispatcher
[ "$(sent)" = 1 ] && grep -q "^send --if idle t5 <news project=\"demo\">" "$tmp/hc/log" || die "薄い字のサジェストだけの入力欄に hc send --if idle しない: $(grep '^send' "$tmp/hc/log" | tail -1)"
grep -q "^t5	<news project=\"demo\">" "$tmp/hc/sent" && grep -q "$id review-requested:" "$tmp/hc/sent" || die '<news> の文が違う'
jq -e '.pending == [] and .news_turn == 4' "$state" >/dev/null || die '送った turn を控えていない'
id2=$(event '答えている途中の通知')
dispatcher
[ "$(sent)" = 1 ] || die '前の <news> の turn が進んでいないのに送った'
cos done 5
printf '❯ <news project="demo">\n  %s review-requested: x\n  </news>\n\n● <no-reply>\n\n%s\n❯\n%s\n' "$id" "$rule" "$rule" > "$tmp/hc/screen-t5"
dispatcher
grep -qx 't5 news.replied=5' "$tmp/hc/labels" || die '③で送った <news> の <no-reply> に news.replied を書かない'
screen '❯' '❯'
dispatcher
[ "$(sent)" = 2 ] && grep -q "^send t5 <news" "$tmp/hc/log" && grep -q "$id2 review-requested:" "$tmp/hc/sent" || die 'done の Chief of Staff に --if なしで送らない'
ok '離席時だけ hc send（idle か done・入力欄が空・薄い字は空・hc read と一致・許可の一覧の列だけ・前の turn が進んでから。idle は --if idle）'

# 14. doing で報告無しの Stop が block（担当・レビュアーの Stop フック。stop-check）
id=$(newtask '止まる前に報告する' person)
wt=$HOME/.herdr/worktrees/app/demo-$id
term t7 idle 1 project=demo "issue=$id"
stop() { (cd "$wt" && HC_TERMINAL_ID=t7 sh "$hooks/stop" <<<"{\"stop_hook_active\":${1:-false}}"); }
out=$(stop)
jq -e --arg id "$id" '.decision == "block" and (.reason | startswith("\($id) は doing のまま"))' <<<"$out" >/dev/null || die "報告の無い doing の Stop が block しない: $out"
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
[ -z "$(stop)" ] || die '報告を書いたのに block した'
[ -n "$(stop)" ] || die '前回の通過のあと報告も問いも無い（次のターン）のに通した'
restart "$id"
[ -n "$(stop)" ] || die '直しで doing に戻ったのに、報告の前の Stop を通した'
(cd "$wt" && HC_TERMINAL_ID=t7 "$andon" issue ask "$id" - <<<'{"kind":"line","question":"どちらにする？","facts":["x"],"options":[{"label":"A","pick":true},{"label":"B"}]}' >/dev/null)
[ -z "$(stop)" ] || die '問いを書いたのに block した'
restart "$id"
stop >/dev/null; stop true >/dev/null; stop true >/dev/null
[ -z "$(stop true)" ] || die '続けて止める回数の上限を超えても block した'
# 控えが書けなければ止めない（止めた回数を数えられないまま止め続けない）
restart "$id"
: > "$tmp/notadir"
for a in false true true true true; do
    [ -z "$(cd "$wt" && XDG_STATE_HOME=$tmp/notadir HC_TERMINAL_ID=t7 sh "$hooks/stop" <<<"{\"stop_hook_active\":$a}")" ] || die '控えが書けないのに block した'
done
term t7 idle 1
[ -z "$(stop)" ] || die 'ラベル issue= の無い端末で block した'
ok 'doing で報告・問いの無い Stop は block（stop-check。前回の通過以後に報告か問いがあれば通す。続けて止めるのは3回まで・控えが書けなければ通す）'

# === 通知の元（dispatcher.md §2.2・§2.3。contract.md §6）===
# 別の台帳 pick（Chief of Staff t6。偽の hc get --json は無いので経路③では送らない）で、拾う元ごとに正負を見る
mkdir -p "$tmp/plant/projects/pick"
cd "$tmp/plant/projects/pick"
env -u KANBAN_ROOT kanban init --prefix P >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t6 >/dev/null
echo "t6 chief-of-staff" >> "$tmp/hc/live"
P=$PWD/kanban
pk() { "$andon" dispatcher --once --project pick 2>>"$tmp/dispatcher.log"; }
pst=$tmp/state/andon/dispatcher/pick.json
pp() { jq -c --arg t "$1" --arg k "$2" '[.pending[] | select(.issue == $t and .kind == $k)]' "$pst"; }  # その issue と kind の待ち（配列）
clear_pending() { jq '.pending = []' "$pst" > "$pst.tmp" && mv "$pst.tmp" "$pst"; }
as_cos() { HC_TERMINAL_ID=t6 "$@"; }
as_w() { HC_TERMINAL_ID=t8 "$@"; }
now_ms() { echo $(( $(date +%s) * 1000 )); }
termp() { # termp <addr> <status> <status_ms>: 生きている担当の偽の hc get --json
    mkdir -p "$(dirname "$tmp/hc/term-$1.json")"
    jq -n --arg a "$1" --arg s "$2" --argjson ms "$3" '{address: $a, state: "live", status: {state: $s}, status_ms: $ms, turn: 1, labels: {}}' > "$tmp/hc/term-$1.json"
    grep -q "^$1 " "$tmp/hc/live" || echo "$1 agent" >> "$tmp/hc/live"
}
qjson='{"kind":"business","question":"どちらの名前にする？","facts":["x"],"options":[{"label":"A","pick":true},{"label":"B"}]}'

# 15. 初めて見る台帳は ready と log.jsonl の位置を控えるだけ
p1=$(as_cos kanban add '前からある課題')
as_w kanban report "$p1" <<<'## Report' >/dev/null
pk
jq -e --arg p "$p1" '.pending == [] and .ready == [$p] and .log_pos == ('"$(stat -c %s "$P/log.jsonl")"')' "$pst" >/dev/null || die "初めて見る台帳で積んだ・控えていない: $(jq -c '{pending, ready, log_pos}' "$pst")"
ok '初めて見る台帳は、今の ready と log.jsonl の位置を控えるだけで積まない'

# 16. ready は差で拾う（人の課題は除く・ルーチンは detail.routine）
p2=$(as_cos kanban add '新しく引き取れる')
p3=$(as_cos kanban add '人がする'); as_cos kanban set "$p3" who=person >/dev/null
p4=$(as_cos kanban add 'ルーチンが足した'); as_cos kanban set "$p4" routine=nightly >/dev/null
pk
pp "$p2" ready | jq -e '.[0].detail == {title: "新しく引き取れる"}' >/dev/null || die "増えた ready を拾わない: $(pp "$p2" ready)"
pp "$p4" ready | jq -e '.[0].detail == {title: "ルーチンが足した", routine: "nightly"}' >/dev/null || die "ルーチンの ready に routine が無い: $(pp "$p4" ready)"
[ "$(pp "$p1" ready)" = '[]' ] || die '前回もあった ready を拾った'
[ "$(pp "$p3" ready)" = '[]' ] || die '人の課題の ready を拾った'
emit <(pp "$p4" ready | jq '.[0]') notification-ready.json
clear_pending; pk
[ "$(jq '[.pending[] | select(.kind == "ready")] | length' "$pst")" = 0 ] || die '同じ ready を二度拾った'
ok 'ready は前回との差で拾い（人の課題は除く・routine を添える）、同じものを二度拾わない'

# 17. log.jsonl: report・問い・review。Chief of Staff 自身の行は拾わない。同じ課題の同じ出来事は新しい1件に置き換える
as_w kanban start "$p2" >/dev/null
as_w kanban report "$p2" <<<'## Report' >/dev/null
as_cos kanban report "$p4" <<<'## Report' >/dev/null
pk
pp "$p2" report | jq -e --arg r "$P/issues/$p2/report.md" '.[0].detail == {report: $r}' >/dev/null || die "報告を拾わない: $(pp "$p2" report)"
[ "$(pp "$p4" report)" = '[]' ] || die 'Chief of Staff 自身の報告を拾った'
emit <(pp "$p2" report | jq '.[0]') notification-report.json
first_at=$(pp "$p2" report | jq -r '.[0].at')
sleep 1
as_w kanban report "$p2" --append <<<'足した' >/dev/null
pk
pp "$p2" report | jq -e --arg a "$first_at" 'length == 1 and .[0].at > $a' >/dev/null || die "同じ出来事を新しい1件に置き換えない: $(pp "$p2" report)"
ok 'log.jsonl の報告を前回の続きから拾い、Chief of Staff（by terminal:t6）の行は拾わず、同じ課題の同じ出来事は新しい1件に置き換える'

(HC_TERMINAL_ID=t8 "$andon" issue ask "$p2" - <<<"$qjson" >/dev/null)
(HC_TERMINAL_ID=t6 "$andon" issue ask "$p4" - <<<"$qjson" >/dev/null)
pk
pp "$p2" asked | jq -e '.[0].detail == {n: 1, kind: "business", question: "どちらの名前にする？"}' >/dev/null || die "問いを拾わない: $(pp "$p2" asked)"
[ "$(pp "$p4" asked)" = '[]' ] || die 'Chief of Staff 自身の問いを拾った'
# 問いの JSON が台帳の行より遅れて置かれたら、1回だけ待って拾う。JSON の無い問い（台帳の CLI を直に打った）は待ったあと飛ばす
as_w kanban ask "$p2" '直に打った問い' >/dev/null
pk
[ "$(pp "$p2" asked | jq -r '.[0].detail.n')" = 1 ] && jq -e '.log_hold != null' "$pst" >/dev/null || die '問いの JSON を待たない'
mkdir -p "$P/issues/$p2/library/questions"; echo "$qjson" > "$P/issues/$p2/library/questions/2.json"
pk
pp "$p2" asked | jq -e 'length == 1 and .[0].detail.n == 2' >/dev/null && jq -e 'has("log_hold") | not' "$pst" >/dev/null || die "遅れて置かれた問いの JSON を拾わない: $(pp "$p2" asked)"
as_w kanban ask "$p2" 'JSON の無い問い' >/dev/null
as_w kanban report "$p2" --append <<<'その後' >/dev/null
clear_pending; pk; pk
[ "$(pp "$p2" asked)" = '[]' ] && [ "$(pp "$p2" report | jq length)" = 1 ] || die "JSON の無い問いで止まり続けた・あとの行を拾わない: $(jq -c .pending "$pst")"
grep -q "$p2 の問い 3 の JSON が読めない" "$tmp/dispatcher.log" || die '飛ばした問いがログに無い'
ok 'log.jsonl の問いを問いの JSON から拾い（Chief of Staff の問いは拾わない）、JSON が遅れたら1回だけ待ち、無ければ飛ばして続きを読む'

mkdir -p "$P/issues/$p2/library"
jq -n '{commit: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", pr: "https://github.com/example/app/pull/7"}' > "$P/issues/$p2/library/report.json"
as_w kanban review "$p2" >/dev/null
p5=$(as_cos kanban add 'PR の無い課題'); as_w kanban start "$p5" >/dev/null; as_w kanban review "$p5" >/dev/null
p6=$(as_cos kanban add 'Chief of Staff がレビューに入れる'); as_cos kanban review "$p6" >/dev/null
clear_pending; pk
pp "$p2" review | jq -e --arg r "$P/issues/$p2/report.md" '.[0].detail == {report: $r, pr: "https://github.com/example/app/pull/7"}' >/dev/null || die "review を拾わない: $(pp "$p2" review)"
pp "$p5" report | jq -e 'length == 1' >/dev/null && [ "$(pp "$p5" review)" = '[]' ] || die "PR の無い review を report で知らせない: $(jq -c .pending "$pst")"
[ "$(jq --arg t "$p6" '[.pending[] | select(.issue == $t)] | length' "$pst")" = 0 ] || die 'Chief of Staff 自身の review を拾った'
emit <(pp "$p2" review | jq '.[0]') notification-review.json
ok 'log.jsonl の review を report.json の pr つきで拾い（PR が無ければ report）、Chief of Staff の review は拾わない'

# 18. 止まった担当（doing の担当の端末。同じ状態・同じ status_ms につき1回）
st_task() { local id; id=$(as_cos kanban add "$1"); HC_TERMINAL_ID=$2 kanban start "$id" >/dev/null; echo "$id"; }
old_idle=$(( $(now_ms) - 16 * 60 * 1000 )); old_work=$(( $(now_ms) - 21 * 60 * 1000 )); fresh=$(( $(now_ms) - 60 * 1000 ))
g=$(st_task '端末が無い' w21)
b=$(st_task 'blocked' w22); termp w22 blocked "$fresh"
e=$(st_task 'error（認証）' w24); termp w24 error "$fresh"; printf 'API Error: 401 unauthorized · Please run /login\n' > "$tmp/hc/screen-w24"
i=$(st_task 'idle が長い' w25); termp w25 idle "$old_idle"
r=$(st_task 'レビュー待ち' w26); termp w26 done "$old_idle"; as_cos kanban add 'レビュアー' --parent "$r" --profile codex-review >/dev/null
w=$(st_task 'working が長い' w27); termp w27 working "$old_work"
q=$(st_task '静か（短い）' w28); termp w28 idle "$fresh"
k=$(st_task 'working（短い）' w29); termp w29 working "$fresh"
pk
pp "$g" finished | jq -e '.[0].detail == {agent: "w21", status_ms: 0}' >/dev/null || die "端末の無い担当を finished で知らせない: $(pp "$g" finished)"
pp "$b" blocked | jq -e --argjson ms "$fresh" '.[0].detail == {agent: "w22", status_ms: $ms}' >/dev/null || die "blocked を知らせない: $(pp "$b" blocked)"
pp "$e" error | jq -e 'length == 1' >/dev/null || die '提供元でない error を知らせない'
pp "$i" stalled | jq -e --argjson ms "$old_idle" '.[0].detail == {agent: "w25", status_ms: $ms}' >/dev/null || die "idle のまま15分を stalled で知らせない: $(pp "$i" stalled)"
pp "$w" stalled | jq -e 'length == 1' >/dev/null || die 'working のまま20分を stalled で知らせない'
[ "$(pp "$r" stalled)" = '[]' ] || die '子のレビュアーの課題を待つ担当を stalled で知らせた'
[ "$(jq --arg q "$q" --arg k "$k" '[.pending[] | select(.issue == $q or .issue == $k)] | length' "$pst")" = 0 ] || die '短い idle・working を知らせた'
! grep -q "^t2[0-9]	" "$tmp/hc/sent" 2>/dev/null || die '提供元でない error に続きを打った'
for f in "$g finished" "$b blocked" "$e error" "$i stalled"; do set -- $f; emit <(pp "$1" "$2" | jq '.[0]') "notification-$2.json"; done
clear_pending; pk
[ "$(jq '.pending | length' "$pst")" = 0 ] || die "同じ状態で二度知らせた: $(jq -c .pending "$pst")"
termp w22 blocked "$(now_ms)"
pk
pp "$b" blocked | jq -e 'length == 1' >/dev/null || die '新しい status_ms の blocked を知らせない'
ok '止まった担当: 端末が無い（finished）・blocked・提供元でない error・idle 15分・working 20分を知らせ、レビュー待ち・短いものは知らせず、同じ status_ms につき1回'

# 19. 提供元に切られたら自分で続きを打つ（切られた 1〜4 回目は 1・2・4・8 分あと）。5 回目に切られたら知らせる（dispatcher.md §2.4）
v=$(st_task 'error（混雑）' w23); termp w23 error "$fresh"; printf '› hello\n■ We’re currently experiencing high demand, which may cause temporary errors.\n' > "$tmp/hc/screen-w23"
clear_pending; pk
! grep -q '^w23	' "$tmp/hc/sent" 2>/dev/null && jq -e --arg k "$v|w23" '.retry[$k].since > 0 and .retry[$k].n == 0' "$pst" >/dev/null || die '初めて見た切断にすぐ打った・見た時刻を控えない'
jq --arg k "$v|w23" '.retry[$k].since = 0' "$pst" > "$pst.tmp" && mv "$pst.tmp" "$pst"   # 見てから 1 分たった
pk
[ "$(grep -c "^w23	提供元（モデルの API）に切られて止まった。止まる前の続きを" "$tmp/hc/sent")" = 1 ] && [ "$(pp "$v" error)" = '[]' ] || die '提供元に切られた担当に続きを打たない・知らせた'
pk
[ "$(grep -c '^w23	' "$tmp/hc/sent")" = 1 ] || die '2分を待たずに続きを打った'
jq --arg k "$v|w23" '.retry[$k] = {n: 4, at: 0}' "$pst" > "$pst.tmp" && mv "$pst.tmp" "$pst"
pk
pp "$v" error | jq -e '.[0].detail.agent == "w23"' >/dev/null && [ "$(grep -c '^w23	' "$tmp/hc/sent")" = 1 ] || die "4回続きを打ってまた切られた担当を error で知らせない: $(pp "$v" error)"
ok '提供元に切られたら自分で続きを打ち（間を空けて4回まで）、5回目に切られたら error で知らせる'

# 20. log.jsonl が書き直されて前の位置より長くなっても、先頭の行が替われば先頭から読む
old=$(stat -c %s "$P/log.jsonl")
p7=$(as_cos kanban add '書き直しのあと')
{ echo "{\"at\":\"2026-01-01T00:00:00Z\",\"by\":\"terminal:t6\",\"id\":\"$p7\",\"change\":{\"title\":\"書き直した\"}}"
  echo "{\"at\":\"2026-01-01T00:00:01Z\",\"by\":\"terminal:t8\",\"id\":\"$p7\",\"change\":{\"report\":\"write\"}}"
  while [ "$(stat -c %s "$P/log.jsonl.new" 2>/dev/null || echo 0)" -le "$old" ]; do
    echo "{\"at\":\"2026-01-01T00:00:02Z\",\"by\":\"terminal:t6\",\"id\":\"$p7\",\"change\":{\"title\":\"埋め草\"}}" >> "$P/log.jsonl.new"
  done; } >> "$P/log.jsonl.new.head"
cat "$P/log.jsonl.new.head" "$P/log.jsonl.new" > "$P/log.jsonl"; rm "$P/log.jsonl.new.head" "$P/log.jsonl.new"
[ "$(stat -c %s "$P/log.jsonl")" -gt "$old" ] || die '前提: 書き直した log が前の位置より長くない'
clear_pending; pk
pp "$p7" report | jq -e 'length == 1' >/dev/null || die "書き直した log の先頭の報告を拾わない: $(jq -c .pending "$pst")"
grep -q 'log.jsonl の先頭の行が替わった' "$tmp/dispatcher.log" || die '読み直した理由がログに無い'
clear_pending; pk
[ "$(pp "$p7" report)" = '[]' ] || die '読み直したあと同じ行をもう一度拾った'
ok 'log.jsonl が書き直されて前の位置より長くなっても、先頭の行が替われば先頭から読み直す（追記では読み直さない）'

# 21. 別のマシンの担当（agent: <machine>/<address>）も見る
m=$(as_cos kanban add '別のマシン'); HC_TERMINAL_ID=m1/w30 kanban start "$m" >/dev/null
kanban show "$m" --json | jq -e '.issue.agent == "m1/w30"' >/dev/null || die '前提: agent が m1/w30 でない'
termp m1/w30 blocked "$(now_ms)"
clear_pending; pk
pp "$m" blocked | jq -e '.[0].detail.agent == "m1/w30"' >/dev/null || die "別のマシンの担当の blocked を知らせない: $(pp "$m" blocked)"
sed -i '/^m1\/w30 /d' "$tmp/hc/live"
pk
pp "$m" finished | jq -e '.[0].detail.agent == "m1/w30"' >/dev/null || die '別のマシンの担当の端末が無いのを知らせない'
ok '別のマシンの担当（<machine>/<address>）の blocked・finished も知らせる'

# 22. hc get の一時の失敗は端末が無いことと分け、取れない間は前の状態を保つ
n=$(st_task '一時の失敗' w31); termp w31 blocked "$fresh"
clear_pending; pk
pp "$n" blocked | jq -e 'length == 1' >/dev/null || die '前提: blocked を知らせない'
echo w31 > "$tmp/hc/getfail"
clear_pending; pk
[ "$(jq --arg t "$n" '[.pending[] | select(.issue == $t)] | length' "$pst")" = 0 ] || die "hc get の失敗で何か積んだ（finished など）: $(jq -c .pending "$pst")"
jq -e --arg k "$n|w31|blocked|$fresh" '.sent[$k]' "$pst" >/dev/null || die '取れない間に知らせた状態を忘れた'
rm "$tmp/hc/getfail"
pk
[ "$(jq --arg t "$n" '[.pending[] | select(.issue == $t)] | length' "$pst")" = 0 ] || die '取れるようになったら同じ状態をもう一度知らせた'
ok 'hc get の一時の失敗では finished にせず、取れない間は知らせた状態を保つ（戻っても同じ状態は知らせない）'

# 23. 続きを送れなかった試行も回数と時刻に数える
u=$(st_task '送れない' w32); termp w32 error "$old_work"; printf '■ Selected model is at capacity. Please try a different model.\n' > "$tmp/hc/screen-w32"
echo w32 > "$tmp/hc/sendfail"
clear_pending; pk
jq --arg k "$u|w32" '.retry[$k].since = 0' "$pst" > "$pst.tmp" && mv "$pst.tmp" "$pst"   # 見てから 1 分たった
pk; pk
jq -e --arg k "$u|w32" '.retry[$k].n == 1 and .retry[$k].at > 0' "$pst" >/dev/null || die "送れなかった試行を数えない・間を空けずに打ち直した: $(jq -c --arg k "$u|w32" '.retry[$k]' "$pst")"
[ "$(grep -c '^send w32 ' "$tmp/hc/log")" = 1 ] || die '送れないまま見回りごとに打った'
jq --arg k "$u|w32" '.retry[$k].at = 0 | .retry[$k].n = 3' "$pst" > "$pst.tmp" && mv "$pst.tmp" "$pst"
pk
jq -e --arg k "$u|w32" '.retry[$k].n == 4' "$pst" >/dev/null || die '送れなかった4回目を数えない'
jq --arg k "$u|w32" '.retry[$k].at = 0' "$pst" > "$pst.tmp" && mv "$pst.tmp" "$pst"
pk
pp "$u" error | jq -e 'length == 1' >/dev/null || die '送れないまま4回を超えても error で知らせない'
rm "$tmp/hc/sendfail"
ok '提供元のエラーで続きを送れなかった試行も回数と時刻に数え、上限を超えたら error で知らせる'

echo "all passed ($pass)"
