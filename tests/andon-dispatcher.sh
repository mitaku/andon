#!/usr/bin/env bash
# `andon dispatcher`（インナーループ。docs/dispatcher.md §5 の P6a）。本物の kanban の CLI と git・一時のセルの repo・偽の hc・偽の gh で回す。
# Chief of Staff（t5）は kanban を用意したあと1つも打たず、review-request → レビュアーを起動する → レビュー → ready → gate →
# マージ → finish が通ることを見る。fix は担当へ hc send、needs_human は問い（from: gate）。GitHub と本物の hc には出ない。
# ANDON_EMIT_DIR があれば、通知・問い・gate の記録を写す（tests/schema.sh が schema に通す）。kanban の CLI は tests/lib/kanban.sh が焼く。
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
die() { # 落ちたら、dispatcher のログの末尾・控えのレビュアー・kanban の一覧も stderr に出す。一時領域は終わると消えるので、
    # CI でまれにしか落ちない揺れは、ここで出したものしか手がかりが残らない（dispatcher が起動を見送った理由はログにだけ書かれる）
    echo "FAIL $1" >&2
    local st=$tmp/state/andon/dispatcher/demo.json
    if [ -s "$tmp/dispatcher.log" ]; then echo "--- dispatcher.log の末尾 40 行" >&2; tail -n 40 "$tmp/dispatcher.log" >&2; fi
    if [ -f "$st" ]; then echo "--- 控えの reviewers" >&2; jq -c '.reviewers' "$st" >&2 || true; fi
    if [ -n "${KANBAN_ROOT:-}" ]; then
        echo "--- kanban の課題（id・status・parent）" >&2
        kanban list --all --json 2>&1 | jq -r '.[] | [.id, .status, (.parent // "")] | @tsv' >&2 || true
    fi
    exit 1
}
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
echo "\$*" >> \$d/calls   # 打ったもの（REST の回数を数える）
case "\$*" in   # ghfail があれば、gate の最初の pr view がその文を stderr に出して落ちる
  "pr view 7 --repo example/app --json state,isDraft,"*) [ ! -f \$d/ghfail ] || { cat \$d/ghfail >&2; exit 1; } ;;
esac
case "\$*" in
  "api rate_limit") cat \$d/rate_limit.json ;;
  "pr view 7 --repo example/app --json headRefOid -q .headRefOid"|"pr view https://github.com/example/app/pull/7 --json headRefOid -q .headRefOid") jq -r .headRefOid \$d/view.json ;;
  "pr view 7 --repo example/"*" --json state,mergeCommit")   # dispatcher がマージしたあとの読み直し。打った回数を states に
    echo x >> \$d/states
    if [ -f \$d/closed ]; then echo '{"state":"CLOSED","mergeCommit":null}'
    elif [ -f \$d/merged ]; then jq -n --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o}}'; else echo '{"state":"OPEN","mergeCommit":null}'; fi ;;
  "pr view 7 --repo example/"*" --json state,mergeCommit,headRefOid")   # andon issue settle と gh pr merge が失敗したあとの読み直し。
    echo x >> \$d/states                                                  # 打った回数を states に（オーナーのマージを見る間隔）。closed があれば閉じた PR
    if [ -f \$d/closed ]; then jq '{state:"CLOSED",mergeCommit:null,headRefOid}' \$d/view.json
    elif [ -f \$d/merged ]; then jq --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o},headRefOid}' \$d/view.json
    else jq '{state:"OPEN",mergeCommit:null,headRefOid}' \$d/view.json; fi ;;
  "pr view 7 --repo example/"*" --json state,mergeCommit,headRefOid,statusCheckRollup")   # andon issue settle。打った回数を states に（上と同じ）。
    echo x >> \$d/states                                                                    # statusCheckRollup は head に書いた gate の印（marks/<sha>。説明文は出さない）
    h=\$(jq -r .headRefOid \$d/view.json); m=\$(cat \$d/marks/\$h 2>/dev/null || :)
    r=\$(jq -n --arg m "\$m" 'if \$m == "" then [] else [{__typename: "StatusContext", context: "andon/gate", state: (\$m | ascii_upcase)}] end')
    if [ -f \$d/closed ]; then jq --argjson r "\$r" '{state:"CLOSED",mergeCommit:null,headRefOid,statusCheckRollup:\$r}' \$d/view.json
    elif [ -f \$d/merged ]; then jq --arg o "\$(cat \$d/merged)" --argjson r "\$r" '{state:"MERGED",mergeCommit:{oid:\$o},headRefOid,statusCheckRollup:\$r}' \$d/view.json
    else jq --argjson r "\$r" '{state:"OPEN",mergeCommit:null,headRefOid,statusCheckRollup:\$r}' \$d/view.json; fi ;;
  "pr view 7 --repo example/app --json state,commits,comments,reviews")   # オーナーの手直し（prcfail があれば落ちる。prc.json があればその中身）
    [ ! -f \$d/prcfail ] || { echo "HTTP 502" >&2; exit 1; }
    if [ -f \$d/prc.json ]; then cat \$d/prc.json
    else jq --arg s "\$(if [ -f \$d/merged ]; then echo MERGED; else echo OPEN; fi)" '{state:\$s,commits:[{oid:.headRefOid,messageHeadline:"x"}],comments:[],reviews:[]}' \$d/view.json; fi ;;
  "api repos/example/app/commits/"*)   # オーナーの commit ごとの親とファイル（commits/<sha>.json があればその中身）
    sha=\${2##*/}; if [ -f \$d/commits/\$sha.json ]; then cat \$d/commits/\$sha.json; else echo '{"files":[{"filename":"docs/b.md"}]}'; fi ;;
  "api --paginate --slurp repos/example/app/pulls/7/comments") if [ -f \$d/inline.json ]; then cat \$d/inline.json; else echo '[[]]'; fi ;;   # インラインのレビューコメント
  "pr view 8 --repo example/app --json state,mergeCommit,mergedAt") echo '{"state":"CLOSED","mergeCommit":null,"mergedAt":null}' ;;   # 結果の記録（閉じられた PR）
  "pr view 7 --repo example/app --json state,mergeCommit,mergedAt")   # 結果の記録（outcomefail があれば落ちる。closed・merged で PR の状態）
    [ ! -f \$d/outcomefail ] || { echo "HTTP 502" >&2; exit 1; }
    if [ -f \$d/closed ]; then echo '{"state":"CLOSED","mergeCommit":null,"mergedAt":null}'
    elif [ -f \$d/merged ]; then jq -n --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o},mergedAt:"2026-11-19T10:00:00Z"}'
    else echo '{"state":"OPEN","mergeCommit":null,"mergedAt":null}'; fi ;;
  "pr view 7 --repo example/app --json "*) cat \$d/view.json ;;
  "pr view https://github.com/example/app/pull/7 --json state,mergeCommit,files")   # andon issue impact（impactfail があれば落ちる）
    [ ! -f \$d/impactfail ] || { echo "HTTP 502" >&2; exit 1; }
    if [ -f \$d/merged ]; then jq --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o},files:[.[0][] | {path: .filename}]}' \$d/files.json
    else echo '{"state":"OPEN","mergeCommit":null,"files":[]}'; fi ;;
  "api graphql "*)  # statusCheckRollup.contexts を100件ずつ（after は前のページの件数）
    off=\$(printf '%s\n' "\$@" | sed -n 's/^after=//p'); echo "graphql \${off:-0}" >> \$d/pages
    jq --argjson off "\${off:-0}" --slurpfile v \$d/view.json '{data: {viewer: {login: "plant-bot"}, repository: {defaultBranchRef: {name: "main"}, pullRequest: {headRefOid: \$v[0].headRefOid,
      commits: {nodes: [{commit: {oid: \$v[0].headRefOid, statusCheckRollup: (if length == 0 then null else
        {contexts: {pageInfo: {hasNextPage: (length > \$off + 100), endCursor: ((\$off + 100) | tostring)}, nodes: .[\$off:\$off + 100]}} end)}}]}}}}}' \$d/rollup.json ;;
  "api --paginate --slurp repos/example/app/pulls/7/files") cat \$d/files.json ;;
  "pr diff 7 --repo example/app") cat \$d/diff ;;
  "api -X POST repos/example/app/statuses/"*) echo "status \$*" >> \$d/posted   # 書いた印は head ごとに marks/<sha>（settle の読みが返す）
    [ ! -f \$d/statusfail ] || { echo "HTTP 502" >&2; exit 1; }
    sha=\${4##*/}; mkdir -p \$d/marks; printf '%s\n' "\$@" | sed -n 's/^state=//p' > \$d/marks/\$sha ;;
  "pr comment 7 --repo example/app --body "*) echo "comment" >> \$d/posted
    if [ -f \$d/hook ]; then sh \$d/hook; rm -f \$d/hook; fi
    echo "https://github.com/example/app/pull/7#issuecomment-1" ;;  # gate の最後の書き込み。マージの前に状態を変える
  "pr merge 7 --repo example/app "*)   # race があれば、オーナーが直前に同じ head でマージしていて失敗する。mergefail があれば、その文を stderr に出してマージせずに失敗する
    echo "\$*" >> \$d/merges
    if [ -f \$d/mergefail ]; then cat \$d/mergefail >&2; exit 1; fi
    h=\$(jq -r .headRefOid \$d/view.json)
    git -C $tmp/origin update-ref refs/heads/main "\$h" && echo "\$h" > \$d/merged
    if [ -f \$d/race ]; then rm \$d/race; echo "Pull request example/app#7 was already merged" >&2; exit 1; fi ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
# GitHub の checks（statusCheckRollup）は $tmp/gh/rollup.json。既定は通っている
checks() { jq -n --arg s "$1" --arg c "${2:-}" '[{__typename: "CheckRun", name: "test", status: $s, conclusion: $c}]' > "$tmp/gh/rollup.json"; }
mkdir -p "$tmp/gh"; checks COMPLETED SUCCESS
# 偽の hc: 生きている端末は $tmp/hc/live（address issue の行）。run は t9, t10 … を返し、打ったものを $tmp/hc/log に。
# run の first_message は $tmp/hc/first の中身（無ければ届いた: sent・started が true）
# get --json は $tmp/hc/term-<addr>.json、read は screen-<addr>。attach は無い（打てば 99 で落ちる。dispatcher は入力欄を読まない）
cat > "$tmp/bin/hc" <<EOF
#!/bin/bash
d=$tmp/hc
echo "\$*" >> \$d/log
case \$1 in
  get) [ ! -f "\$d/getfail" ] || ! grep -qx "\$2" \$d/getfail || { echo '{"error":{"kind":"internal","message":"x"}}' >&2; exit 1; }  # 一時の失敗
       grep -q "^\$2 " \$d/live || { echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2; exit 3; }
       [ "\${3:-}" != --json ] || cat \$d/term-\$2.json 2>/dev/null ;;
  read) cat \$d/screen-\$2 2>/dev/null || : ;;                      # hc read <addr> [--lines n]: 文字だけの画面
  label) echo "\$2 \$3" >> \$d/labels ;;
  run) n=\$(( \$(cat \$d/next 2>/dev/null || echo 9) )); echo \$((n + 1)) > \$d/next
       task=\$(printf '%s\n' "\$@" | sed -n 's/^issue=//p')
       echo "t\$n \$task" >> \$d/live
       printf '{"address":"t%s","first_message":%s}\n' "\$n" "\$(cat \$d/first 2>/dev/null || echo '{"sent":true,"started":true}')" ;;
  send) [ "\$2" != --if ] || shift 2; [ ! -f "\$d/sendfail" ] || ! grep -qx "\$2" \$d/sendfail || exit 1
        printf '%s\t%s\n' "\$2" "\$3" >> \$d/sent ;;
  ls) task=\$(printf '%s\n' "\$@" | sed -n 's/^issue=//p')
      awk -v t="\$task" '\$2 == t { printf "%s{\"address\":\"%s\",\"state\":\"live\"}", (n++ ? "," : ""), \$1 }' \$d/live | { printf '{"terminals":['; cat; printf ']}\n'; } ;;
  templates) echo '{"templates":[{"name":"claude"},{"name":"claude-resume"}]}' ;;
  kill|keep) ;;
  rm) sed -i "/^\$2 /d" \$d/live ;;
  *) exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH"
echo "t5 chief-of-staff" > "$tmp/hc/live"
echo '{"address":"t5","state":"live","status":{"state":"working"},"labels":{"role":"chief-of-staff","project":"demo"}}' > "$tmp/hc/term-t5.json"   # Chief of Staff の端末（無ければ cos() が書き直す）
# dispatcher の端末（Chief of Staff のフックの drain が中身を確かめる。terminal.go）。中身は andon dispatcher
echo "dispatcher role" >> "$tmp/hc/live"
fg_dispatcher() { echo '{"address":"dispatcher","state":"live","labels":{"herdr.space":"andon-demo","herdr.tab":"andon-demo-dispatcher"},"foreground":{"name":"andon","argv":["'"$andon"'","dispatcher"]}}' > "$tmp/hc/term-dispatcher.json"; }
fg_dispatcher

# セルの repo（オーナーのもの）: 条件表（squash・検査）と委譲（merge B）を owner が commit する
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
dispatcher() { freeze; "$andon" dispatcher --once --project demo 2>>"$tmp/dispatcher.log"; }
state=$tmp/state/andon/dispatcher/demo.json
mergerec=$tmp/state/andon/dispatcher/demo.merge.json   # andon issue merge の記録（書き手は merge だけ）
settled=${state%.json}.settle.json   # andon issue settle の記録（書き手は settle だけ。ADR-0017 決めたこと 5）
# 見張りの時計を止める。merge の記録の polled・approved と settle の記録の at は、merge と settle が実際の時刻で書く。
# 1分（approved は10分）たつと、どの場面の課題でも PR を GitHub に聞き直し、gate を回し直すか知らせる。検査が遅いと（負荷が高い・
# 並べて回す）、前の場面に残った課題の聞き直しが後ろの場面に割り込み、gh の hook を先に使う・ログや数がずれる、で落ちる。
# そこで dispatcher・merge・settle を打つ前に、検査が戻した時刻（$due）のほかを 2999 年へ送る。聞き直させたい場面は、
# mpoll・watch_due でその課題だけ $due にする。初めて見る課題（時刻が無い）はこれまでどおりすぐ聞く。
due=2000-01-01T00:00:00Z
freeze() {
    if [ -f "$mergerec" ]; then
        jq --arg d "$due" 'map_values(with_entries(if (.key == "polled" or .key == "approved") and .value != $d then .value = "2999-01-01T00:00:00Z" else . end))' \
            "$mergerec" > "$mergerec.new" && mv "$mergerec.new" "$mergerec"
    fi
    if [ -f "$settled" ]; then
        jq --arg d "$due" 'map_values(if .at and .at != $d then .at = "2999-01-01T00:00:00Z" else . end)' "$settled" > "$settled.new" && mv "$settled.new" "$settled"
    fi
}
pending() { jq -c --arg t "$1" --arg k "$2" '[.pending[] | select(.issue == $t and .kind == $k)] | last' "$state"; }
recorded_log=$tmp/state/andon/dispatcher/demo.recorded.jsonl   # 記録だけの通知（ADR-0010 決めたこと 2）
recorded() { if [ -f "$recorded_log" ]; then jq -cs --arg t "$1" --arg k "$2" '[.[] | select(.issue == $t and .kind == $k)] | last' "$recorded_log"; else echo null; fi; }

# ほかの andon dispatcher が .dispatcher.lock を持っていれば、巡を回さずに 0 で終わる（dispatcher.md §7.2 #1）
mkdir -p "$tmp/state/andon/dispatcher"
r=0; flock "$tmp/state/andon/dispatcher/.dispatcher.lock" "$andon" dispatcher --once --project demo 2>"$tmp/g1" || r=$?
[ "$r" = 0 ] && grep -q 'ほかの andon dispatcher が動いている' "$tmp/g1" && [ ! -e "$state" ] || die "lock を持たれていても巡を回した: rc=$r $(cat "$tmp/g1")"
ok 'andon dispatcher は .dispatcher.lock をほかが持っていれば巡を回さずに終わる'

# Chief of Staff が課題を切る（t5 が予約キーを書く）。担当 t7 が worktree で作業し、PR 7 を出す
newtask() { # newtask <タイトル> <merge> → id。$(...) の中で呼ばれて set -e が効かないので、各行の失敗は die で止める（die の exit 1 が外の代入を落とす）
    local id; id=$(HC_TERMINAL_ID=t5 kanban add "$1" --profile claude --repo github.com/example/app) || die "newtask: kanban add が失敗した: $1"
    [ -n "$id" ] || die "newtask: kanban add が id を返さなかった: $1"
    HC_TERMINAL_ID=t5 kanban set "$id" review=on must_fix=major merge="$2" >/dev/null || die "newtask: kanban set（review・must_fix・merge）が失敗した: $id"
    git -C "$app" fetch -q origin || die "newtask: git fetch が失敗した: $id"
    git -C "$app" worktree add -q -b "demo/$id" "$ANDON_WORKSPACE/.worktrees/app/demo-$id" origin/main || die "newtask: git worktree add が失敗した: $id"
    HC_TERMINAL_ID=t5 kanban set "$id" "branch=demo/$id" "worktree=$ANDON_WORKSPACE/.worktrees/app/demo-$id" >/dev/null || die "newtask: kanban set（branch・worktree）が失敗した: $id"  # andon issue start の代わり
    (cd "$ANDON_WORKSPACE/.worktrees/app/demo-$id" && HC_TERMINAL_ID=t7 kanban start "$id" >/dev/null) || die "newtask: kanban start が失敗した: $id"
    echo "t7 $id" >> "$tmp/hc/live" || die "newtask: hc/live への追記が失敗した: $id"
    echo "$id"
}
work() { # work <id> <path>: 担当が commit して push し、PR 7 の事実をその head にする
    local wt=$ANDON_WORKSPACE/.worktrees/app/demo-$1
    mkdir -p "$wt/$(dirname "$2")"; echo "x$RANDOM" >> "$wt/$2"
    git -C "$wt" add -A; git -C "$wt" commit -qm "$2"; git -C "$wt" push -q origin "HEAD:demo/$1"
    head=$(git -C "$wt" rev-parse HEAD)
    jq -n --arg h "$head" --arg b "demo/$1" '{state:"OPEN",isDraft:false,headRefName:$b,headRefOid:$h,baseRefName:"main",
      mergeStateStatus:"CLEAN",author:{login:"plant-bot"},changedFiles:1,isCrossRepository:false}' > "$tmp/gh/view.json"
    jq -n --arg p "$2" '[[{filename:$p,additions:1,deletions:0}]]' > "$tmp/gh/files.json"
    printf '+++ b/%s\n+x\n' "$2" > "$tmp/gh/diff"
    rm -f "$tmp/gh/merged"
}
restart() { (cd "$ANDON_WORKSPACE/.worktrees/app/demo-$1" && HC_TERMINAL_ID=t7 kanban start "$1" >/dev/null); }  # 直しが届いた担当
as_worker() { (cd "$ANDON_WORKSPACE/.worktrees/app/demo-$1" && HC_TERMINAL_ID=t7 "$andon" issue "$2" "$1" "${@:3}" <<<'## Report' >/dev/null); }
review_by() { # review_by <レビュアーの id> <findings>: レビュアーが依頼書の示す置き場に findings.json を書いて review にする
    # 置き場は依頼書から読む（dispatcher の依頼書が kanban の外を指せば、gate は findings.json を読めず落ちる。A-0216）
    local f
    case $1 in ''|null) die "レビュアーの子課題が kanban に無い（直前の dispatcher の巡がレビュアーを起動しなかった）" ;; esac
    f=$(grep -o '`[^`]*/findings\.json`' "$L/issues/$1/brief.md" | tr -d '`' || true)
    [ "$(printf '%s\n' "$f" | grep -c .)" = 1 ] || die "レビュアーの依頼書に findings.json の置き場が1つでない: $f"
    mkdir -p "${f%/*}"
    jq -n --arg c "$head" --argjson f "$2" '{commit: $c, findings: $f}' > "$f"
    HC_TERMINAL_ID=t9 kanban review "$1" >/dev/null
}

# 0. Chief of Staff の端末が生きていなくても（留守）kanban は回す（配る段だけ飛ばす。ADR-0017 決めたこと 8）。chief_of_staff が設定に無い kanban は飛ばす。
#    初めて見る kanban は通知の元を控えるだけだが、今の review_request は拾う
id=$(newtask '初めて見た kanban のレビュー依頼' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
kanban config set chief_of_staff= >/dev/null
dispatcher
[ ! -e "$state" ] && ! grep -qs '^run' "$tmp/hc/log" || die 'chief_of_staff が設定に無いのに回った'
kanban config set chief_of_staff=t5 >/dev/null
sed -i '/^t5 /d' "$tmp/hc/live"
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die "留守の初めて見た kanban で済んでいない review_request を拾わない: $(grep ^run "$tmp/hc/log" || :)"
jq -e --arg k "$id@$head" '(.reviewers | length) == 1 and .reviewers[$k].started == true and .pending == [] and (.away.since | length) > 0 and .away.count == 0' "$state" >/dev/null && recorded "$id" review-requested | jq -e . >/dev/null ||
    die "初めて見た kanban で控え・通知が違う（report・ready を流した・留守を控えない）: $(jq -c '{reviewers, pending, away}' "$state")"
grep -q '留守（宛先 "t5"' "$tmp/dispatcher.log" || die '留守の1行がログに無い'
echo "t5 chief-of-staff" >> "$tmp/hc/live"
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '同じ review_request で二度起動した'
jq -e 'has("away") | not' "$state" >/dev/null || die "何も積まなかった留守を戻ったあとも控えに残した: $(jq -c .away "$state")"
ok 'chief_of_staff が設定に無ければ飛ばし、Chief of Staff の留守でも回して初めて見る kanban の済んでいない review_request は拾う（過去の report・ready は流さない）'

# 0c. 前のバージョンの控え（初めて見た kanban で控えただけの回）と、起動しなかった回を同じ commit で頼み直したとき
id=$(newtask '止まっていた間のレビュー依頼' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
HC_TERMINAL_ID=t5 kanban set "$id" review=off >/dev/null
HC_TERMINAL_ID=t5 kanban set "$id" review=on >/dev/null
as_worker "$id" review-request
HC_TERMINAL_ID=t5 kanban set "$id" review=off >/dev/null
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die 'review が off なのに起動した'
jq -e --arg k "$id@$head" '.reviewers[$k].skipped | startswith("issue.yaml の review が off で、差分はレベル A")' "$state" >/dev/null || die "起動しなかった回を控えていない: $(jq -c --arg k "$id@$head" '.reviewers[$k]' "$state")"
HC_TERMINAL_ID=t5 kanban set "$id" review=on >/dev/null
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '頼み直していないのに起動し直した'
as_worker "$id" review-request   # 同じ commit で頼み直す
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '起動しなかった回を同じ commit で頼み直したのに拾わない'
rows=$(wc -l < "$L/log.jsonl")
! as_worker "$id" review-request 2>"$tmp/err" || die '起動した回のレビュアーが動いているのに、同じ commit の頼み直しを断らない'
grep -q "もう頼んである（$id-r1 が doing）" "$tmp/err" || die "断りの理由が違う: $(cat "$tmp/err")"
[ "$(wc -l < "$L/log.jsonl")" = "$rows" ] || die '断ったのに報告の行が増えた'
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '起動した回を同じ commit で頼み直したら二重に起動した'
ok '同じ commit の出し直しは、前回が起動せずに終わっていれば拾い、起動してレビュアーが動いていれば review-request が断る（二重にも起動しない）'

# 0d. Chief of Staff が起き直してアドレスが変わると、前の Chief of Staff が書いた予約キーは照合が外れる。その間のレビュー依頼は見送るが、
#     新しい Chief of Staff が予約キーを書き直せば、担当が頼み直さなくても次の見回りで拾い直す
id=$(newtask '起き直しをまたぐレビュー依頼' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
kanban config set chief_of_staff=t6 >/dev/null
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '照合が外れているのに起動した'
jq -e --arg k "$id@$head" '.reviewers[$k] | (.skipped | test("最後に書いたのが Chief of Staff でない")) and .untrusted == true' "$state" >/dev/null ||
    die "照合で見送った回を控えていない: $(jq -c --arg k "$id@$head" '.reviewers[$k]' "$state")"
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '照合が外れたままなのに起動し直した'
HC_TERMINAL_ID=t6 kanban set "$id" review=on must_fix=major merge=person >/dev/null   # 新しい Chief of Staff が同じ値で書き直す
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '予約キーを書き直したのに、照合で見送ったレビュー依頼を拾い直さない'
jq -e --arg k "$id@$head" '.reviewers[$k].started == true' "$state" >/dev/null || die '拾い直した回を起動したと控えていない'
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '拾い直した回を二重に起動した'
kanban config set chief_of_staff=t5 >/dev/null
HC_TERMINAL_ID=t5 kanban done "$id" >/dev/null   # あとの場面（1b のマージの影響）が数える動いている課題に残さない
ok 'Chief of Staff のアドレスが変わって照合で見送ったレビュー依頼は、予約キーを書き直せば担当の頼み直し無しに拾い直す'

# 0e. review: off で始めたコードの課題でも、差分がレベル B なら gate はレビューを求める。担当は Chief of Staff を待たずに頼め、
#     dispatcher は gate と同じ条件（差分のレベル）で決め直して起動する。review の値は Chief of Staff が書いた off のまま
id=$(newtask 'review: off のコードの課題' person)
HC_TERMINAL_ID=t5 kanban set "$id" review=off >/dev/null
work "$id" internal/issue/start_test.go
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request || die 'review: off でも差分がレベル B なのに review-request が断った'
grep -q 'review は off だが、差分がレベル B' "$L/issues/$id/report.md" || die "レビュー依頼の行に review: off で頼んだ理由が無い: $(sed -n 1,3p "$L/issues/$id/report.md")"
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die "review: off で差分がレベル B のレビュー依頼で、レビュアーを起動しない: $(jq -c --arg k "$id@$head" '.reviewers[$k]' "$state")"
jq -e --arg k "$id@$head" '.reviewers[$k].started == true' "$state" >/dev/null || die '起動したと控えていない'
[ "$(kanban show "$id" --json | jq -r .issue.review)" = off ] || die 'review の値を書き換えた'
HC_TERMINAL_ID=t5 kanban done "$id" >/dev/null   # あとの場面が数える動いている課題に残さない
ok 'review: off でも差分がレベル B なら、担当がレビューを頼め、dispatcher が gate と同じ条件で決めてレビュアーを起動する（review は off のまま）'

# 1. 一周: review-request → レビュアーを起動する → レビュー → ready → gate → マージ → finish。Chief of Staff は留守（端末が無い）でも回る（ADR-0017 決めたこと 8）
id=$(newtask 'docs を直す' auto)
sed -i '/^t5 /d' "$tmp/hc/live"
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
rwt=$ANDON_WORKSPACE/.worktrees/app/demo-$id-r1
[ "$(git -C "$rwt" rev-parse HEAD)" = "$head" ] && git -C "$app" worktree list --porcelain | grep -qx "worktree $rwt" || die 'detached worktree が head に無い'
# kanban（$tmp/plant/projects/demo）はセルの git の中にあるが、レビュアーの課題の作業場所は detached worktree（branch は無い）
kanban show "$rev" --json | jq -e --arg w "$rwt" '.issue.worktree == $w and (.issue.branch // "") == ""' >/dev/null ||
    die "レビュアーの課題の worktree・branch が detached worktree でない（セルの本体を書いた）: $(kanban show "$rev" --json | jq -c '.issue | {worktree, branch}')"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$(echo "$rev" | tr A-Z a-z) .*--cwd $L/issues/$rev codex-review" "$tmp/hc/log" || die "hc run の形が違う: $(grep ^run "$tmp/hc/log")"
grep -q "hc send t7 " "$L/issues/$rev/brief.md" && grep -q 'Chief of Staff・dispatcher へは送らない' "$L/issues/$rev/brief.md" || die 'レビュアーの依頼書に宛先（担当 t7）が無い'
grep -q "commit $head" "$L/issues/$rev/brief.md" && ! grep -q '## Report' "$L/issues/$rev/brief.md" || die 'レビュアーの依頼書に commit が無い・担当の報告が入った'
recorded "$id" review-requested | jq -e --arg r "$rev" '.detail == {round: 1, reviewer: "codex-review", reviewer_issue: $r}' >/dev/null || die '通知 review-requested を記録していない'
[ "$(pending "$id" review-requested)" = null ] || die '記録だけの review-requested を Chief of Staff への待ちに積んだ'
grep "^run " "$tmp/hc/log" | grep -q -- "--label issue=$rev " && ! grep "^run " "$tmp/hc/log" | grep -q -- '--label task=' ||
    die "レビュアーの端末のラベルが issue=<親>-r<回> でない（task= を書いた。ADR-0008）: $(grep ^run "$tmp/hc/log")"
recorded "$id" review-requested | jq -e 'has("issue") and (has("task") | not)' >/dev/null || die '通知のキーが issue でない'
jq -e --arg k "$id@$head" '.reviewers[$k] | has("issue") and has("reviewer_issue") and (has("task") | not)' "$state" >/dev/null || die "控えのキーが issue でない: $(jq -c --arg k "$id@$head" '.reviewers[$k]' "$state")"
emit <(recorded "$id" review-requested) notification-review-requested.json
grep "^run " "$tmp/hc/log" | grep -q -- "--label herdr.space=andon-demo-$id --label herdr.tab=andon-demo-$id --cwd " && ! grep "^run " "$tmp/hc/log" | grep -q 'where=' ||
    die "レビュアーに担当の herdr.space・herdr.tab が無い（where を写した）: $(grep ^run "$tmp/hc/log")"
grep -qx "get t7 --json" "$tmp/hc/log" || die '担当の端末を hc get --json で読んでいない'
rm "$tmp/hc/term-t7.json"
# 起動の記録（contract.md §18。ADR-0028 受け入れ条件 2）: レビュアーの課題に1件。repo は PR の head を読む detached worktree
lrec=$L/issues/$rev/library/launches.jsonl
[ "$(wc -l < "$lrec")" = 1 ] && jq -e --arg r "$rev" --arg a "$raddr" --arg h "$head" --arg w "$rwt" '.role == "reviewer" and .launch == "start" and .issue == $r
  and .profile == "codex-review" and .template == "codex-review" and .terminal == $a and .session == null and .repo == {name: "github.com/example/app", path: $w, commit: $h, dirty: false}' "$lrec" >/dev/null \
  || die "レビュアーの起動の記録: $(cat "$lrec" 2>/dev/null)"
emit "$lrec" launch-reviewer.json
ok "review_request でレビュアーを起動する（kanban add --parent・by terminal:dispatcher・detached worktree〈kanban の worktree もそれ・branch 無し〉・hc run --no-notify・担当と同じ herdr.space・herdr.tab・依頼書の宛先は担当・起動の記録）"
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '同じ review_request で二度起動した'
ok '同じ review_request（issue と commit）で二度起動しない'

review_by "$rev" '[{"severity":"minor","file":"docs/a.md","line":1}]'
as_worker "$id" report --data '{"waived":[{"severity":"minor","file":"docs/a.md","line":1,"reason":"起きる条件が無い"}]}'
as_worker "$id" ready
dispatcher
rec=$L/issues/$id/library/gate/$head.json
jq -e '.decision == "approve" and .merged == true and .tier == "B"' "$rec" >/dev/null || die "gate の記録が approve・merged でない: $(cat "$rec")"
grep -qx "pr merge 7 --repo example/app --squash --match-head-commit $head" "$tmp/gh/merges" || die 'マージの打ち方が違う'
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null || die '課題が done でない'
kanban show "$rev" --json | jq -e '.issue.status == "done"' >/dev/null || die 'レビュアーの課題が done でない'
[ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$id" ] && [ ! -e "$rwt" ] || die 'worktree が残った'
! git -C "$app" show-ref --verify --quiet "refs/heads/demo/$id" || die 'origin/main に入った branch が残った'
! grep -q " $id\$\| $rev\$" "$tmp/hc/live" || die '端末が残った'
recorded "$id" gate | jq -e --arg r "$rec" '.detail == {decision: "approve", reasons: [], record: $r}' >/dev/null || die '通知 `gate`（merge: auto の approve）を記録していない'
recorded "$id" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h}' >/dev/null || die '通知 merged を記録していない'
jq -e --arg t "$id" '[.pending[] | select(.issue == $t and (.kind == "gate" or .kind == "merged" or .kind == "review"))] == []' "$state" >/dev/null &&
    jq -e --arg t "$id" '(.[$t].approved // "") == ""' "$mergerec" >/dev/null ||
    die "記録だけの review・gate・merged を待ちに積んだ・マージしたのに approve の待ちを残した: $(jq -c .pending "$state") $(cat "$mergerec")"
recorded "$id-r1" report | jq -e . >/dev/null && [ "$(pending "$id-r1" report)" = null ] || die 'レビュアーの課題の report を記録だけにしていない'
emit <(recorded "$id" gate) notification-gate.json
emit <(recorded "$id" merged) notification-merged.json
emit "$rec" gate-record-merged.json
ok 'review に入ったら gate を回し、approve ＋ merge: auto ＋ 委譲 B でマージ（squash・head を固定）して finish（レビュアーと worktree も）'
emit "$L/log.jsonl" ledger-log.jsonl   # 本物の kanban の CLI が書いた行（tests/schema.sh が1行ずつ schema/ledger/log.v3 に通す）
tail -n +"$((mark + 1))" "$L/log.jsonl" | jq -e -s 'all(.by != "terminal:t5")' >/dev/null || die 'Chief of Staff が kanban に書いた'
[ ! -e "$tmp/hc/sent" ] || die "誰かに hc send した: $(cat "$tmp/hc/sent")"
ok 'その間 Chief of Staff（t5）は留守で、kanban に1行も書いていない・誰にも hc send していない'
echo "t5 chief-of-staff" >> "$tmp/hc/live"

# 1b. マージのあと（done の行）、同じ repo の動いている課題への影響を andon issue impact で1回だけ集めて知らせる
# 0・0c の課題は同じ docs/a.md を別の行で足した branch（doing）なので、ぶつかり・重なる。判断は Chief of Staff（通知だけ）
merged_id=$id
old0=$(kanban list --json | jq -r '.[] | select(.title == "初めて見た kanban のレビュー依頼") | .id')
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

# corrections.json の置き場と Chief of Staff の片付け（1d-1 からあとの節が使う）
cj() { echo "$L/issues/$1/library/corrections.json"; }
cos_done() { HC_TERMINAL_ID=t5 kanban done "$1" >/dev/null; }   # Chief of Staff が片付ける（done の行）
# 1d-1. 推奨と違う答え → answer
id=$(newtask '書き出しの形式を決める' person)
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$id" && HC_TERMINAL_ID=t7 "$andon" issue ask "$id" - >/dev/null <<'JSON'
{"kind": "users", "question": "書き出しは CSV だけか", "options": [{"label": "CSV だけ", "pick": true}, {"label": "CSV と Excel"}], "from": "issue"}
JSON
)
HC_TERMINAL_ID=t5 kanban answer "$id#1" "CSV と Excel" >/dev/null
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$id" && HC_TERMINAL_ID=t7 kanban received "$id#1" >/dev/null)
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
# 1d-2b. インラインのレビューコメントだけがある PR → pr（Bot のものは数えない。空の配列にしない）
id=$(newtask '用語を揃える' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
jq -n '[[{user: {login: "o", type: "User"}, body: "ここは kanban の語にする", created_at: "2026-11-09T01:00:00Z", path: "docs/a.md", line: 1},
        {user: {login: "ci", type: "Bot"}, body: "lint", created_at: "2026-11-09T01:01:00Z", path: "docs/a.md", line: 1}]]' > "$tmp/gh/inline.json"
cos_done "$id"; dispatcher
jq -e '.corrections == [{kind: "pr", comments: [{author: "o", at: "2026-11-09T01:00:00Z", body: "ここは kanban の語にする", file: "docs/a.md", line: 1}], human_commits: []}]' "$(cj "$id")" >/dev/null ||
    die "インラインのレビューコメントが pr の1件になっていない: $(cat "$(cj "$id")" 2>/dev/null)"
emit "$(cj "$id")" corrections-pr-inline.json
rm "$tmp/gh/inline.json"
ok 'インラインのレビューコメントだけの PR でも pr を1件書く（Bot のコメントは数えない）'
# 1d-2c. 報告より後の commit が、main を取り込んだ merge commit（親 2 つ）と親 1 つの 1 本ずつ → human_commits は親 1 つの 1 本だけ（ADR-0058 決めたこと 7）
id=$(newtask '見出しを揃える' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
mc=$(printf 'e%.0s' {1..40}); oc=$(printf 'b%.0s' {1..40})
jq --arg m "$mc" --arg o "$oc" '{state: "MERGED", commits: [{oid: .headRefOid, messageHeadline: "x"}, {oid: $m, messageHeadline: "Merge branch main"}, {oid: $o, messageHeadline: "オーナーが直した"}], comments: [], reviews: []}' \
    "$tmp/gh/view.json" > "$tmp/gh/prc.json"
mkdir -p "$tmp/gh/commits"
jq -n --arg h "$head" '{parents: [{sha: $h}, {sha: "0000000000000000000000000000000000000000"}], files: [{filename: "docs/adr/README.md"}]}' > "$tmp/gh/commits/$mc.json"
jq -n --arg m "$mc" '{parents: [{sha: $m}], files: [{filename: "docs/b.md"}]}' > "$tmp/gh/commits/$oc.json"
cos_done "$id"; dispatcher
jq -e --arg o "$oc" '.corrections == [{kind: "pr", comments: [], human_commits: [{sha: $o, headline: "オーナーが直した", files: ["docs/b.md"]}]}]' "$(cj "$id")" >/dev/null ||
    die "merge commit を外し、親 1 つの commit だけを human_commits にしていない: $(cat "$(cj "$id")" 2>/dev/null)"
rm -r "$tmp/gh/prc.json" "$tmp/gh/commits"
ok '報告より後の merge commit（親 2 つ）はオーナーの commit に数えず、親 1 つの commit だけを human_commits に書く'
# 1d-3. must_fix 以上の指摘を waive してマージした → override
id=$(newtask '例外を足す' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7 --data '{"waived":[{"severity":"major","file":"docs/a.md","line":1,"reason":"オーナーがこのままでよいと言った"}]}'
echo "$head" > "$tmp/gh/merged"
cos_done "$id"; dispatcher
jq -e '.corrections == [{kind: "override", finding: {severity: "major", file: "docs/a.md", line: 1}, reason: "オーナーがこのままでよいと言った"}]' "$(cj "$id")" >/dev/null ||
    die "オーナーが waive した指摘が override の1件になっていない: $(cat "$(cj "$id")" 2>/dev/null)"
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
grep -q "^t7	\[gate\] $id（説明を足す）: gate が fix を返した。理由: Must fix 以上が 1 件" "$tmp/hc/sent" || die "fix が担当に届いていない: $(cat "$tmp/hc/sent" 2>/dev/null)"
[ ! -e "$tmp/gh/merged" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'fix なのにマージした・片付けた'
recorded "$id" gate | jq -e '.detail.decision == "fix"' >/dev/null && [ "$(pending "$id" gate)" = null ] || die 'gate: fix を記録だけにしていない'
dispatcher
[ "$(grep -c "	\[gate\] $id（" "$tmp/hc/sent")" = 1 ] || die '同じ review 入りで二度 gate を回した'
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
kanban questions --json 2>/dev/null | jq -e --arg t "$id" 'any(.[]; .id == $t)' >/dev/null || kanban questions | grep -q "$id" || die 'kanban に問いが無い'
pending "$id" asked | jq -e '.detail.n == 1 and .detail.kind == "line"' >/dev/null || die '通知 asked が無い'
[ ! -e "$tmp/gh/merged" ] || die 'needs_human なのにマージした'
emit "$q" question-gate.json
emit <(pending "$id" asked) notification-asked.json
ok 'needs_human は問い（from: gate・kind line）を kanban に出し、マージしない'
ok 'レビュアーの課題の id は <親>-r<回>（1 回目 r1・2 回目 r2）で、通し番号を使わない'

# 3b. andon issue merge は2回打っても同じ結果（同じ review 入りでは gate も問いも通知も1回だけ）。
#     記録が無くなっても、その review 入りのあとの gate の記録を引き継ぎ、問いを出し直さない（ADR-0017 受け入れ条件 7）
mrun() { freeze; HC_TERMINAL_ID=dispatcher "$andon" issue merge "$1"; }
posted=$(grep -c '^status' "$tmp/gh/posted")
mrun "$id" | jq -e --arg t "$id" '.issue == $t and .state == "skip" and .notify == []' >/dev/null || die "2 回目の merge が何かをした: $(mrun "$id")"
jq -e --arg t "$id" '.[$t].decision == "needs_human" and .[$t].row > 0' "$mergerec" >/dev/null || die "merge の記録に決めたことが無い: $(cat "$mergerec")"
cp "$mergerec" "$tmp/merge.bak"
rm "$mergerec"
mrun "$id" | jq -e '.state == "skip" and .notify == []' >/dev/null || die '記録が無くなったら、gate の記録を引き継がずに回し直した'
jq -e --arg t "$id" '.[$t].decision == "needs_human"' "$mergerec" >/dev/null || die "gate の記録から決めたことを引き継いでいない: $(cat "$mergerec")"
[ ! -e "$L/issues/$id/library/questions/2.json" ] && [ "$(grep -c '^status' "$tmp/gh/posted")" = "$posted" ] || die '2 回目の merge で問い・PR への書き込みを出し直した'
cp "$tmp/merge.bak" "$mergerec"
ok 'andon issue merge を2回打っても、同じ review 入りでは gate・問い・通知は1回だけ。記録が無くなっても、gate の記録を引き継いで問いを出し直さない'

# 4. merge: person は approve でも止まる（通知 `gate` だけ）
id=$(newtask 'オーナーがマージする' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
review_by "$rev" '[]'
as_worker "$id" ready
dispatcher
pending "$id" gate | jq -e '.detail.decision == "approve"' >/dev/null || die '通知 `gate` が無い'
[ ! -e "$tmp/gh/merged" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'merge: person なのにマージした'
ok 'merge: person は approve でもマージせず、通知 `gate` で止まる（オーナーがマージする）'

# 4a. gate が approve か needs_human を決めたあと、review のうちに PR に push された（head が動いた）。merge は決めたあとも、
#     課題ごとに1分に1回だけ PR の head を聞き、判定した head と違えば前の head の gate の問いを閉じて、今の head で gate を回し直す。
#     report.json の commit は前の head のままなので、回し直した gate は fix を返して担当へ送る。gate の見ていない head はマージしない（dispatcher.md §5 の P6a）
mpoll() { # mpoll <id>: その課題だけ、次の merge で PR の head を聞かせる（ほかの課題の polled は先へ送り、聞かせない）
    jq --arg t "$1" --arg d "$due" 'with_entries(if .value.polled then .value.polled = (if .key == $t then $d else "2999-01-01T00:00:00Z" end) else . end)' "$mergerec" > "$mergerec.new" && mv "$mergerec.new" "$mergerec"
}
hv() { jq -r --arg t "$1" '.[$t] | "\(.decision) \(.sha)"' "$mergerec"; }
polls() { grep -c -e '^pr view 7 --repo example/app --json state,headRefOid$' "$tmp/gh/calls" || true; }   # merge が PR の head を聞いた回数
to_gate() { # to_gate <id> <path>: 担当が commit・PR・レビュー（指摘なし）・ready まで進め、dispatcher が gate を回す
    work "$1" "$2"
    as_worker "$1" report --pr https://github.com/example/app/pull/7
    as_worker "$1" review-request
    dispatcher
    review_by "$(kanban list --all --json | jq -r --arg p "$1" '[.[] | select(.parent == $p)] | last | .id')" '[]'
    as_worker "$1" ready
    dispatcher
}
# merge: person の approve（オーナーのマージを待つ間）
pa=$(newtask 'approve のあとに push される' person)
to_gate "$pa" docs/a.md; old=$head
[ "$(hv "$pa")" = "approve $old" ] || die "前提: gate が approve を決めていない: $(hv "$pa")"
jq -e --arg t "$pa" '.[$t].polled | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T")' "$mergerec" >/dev/null || die "決めたときに polled を書いていない: $(cat "$mergerec")"
work "$pa" docs/z.md   # 担当かオーナーが review のうちに push した（checks は通る）
n=$(polls)
mrun "$pa" | jq -e '.state == "skip" and .notify == []' >/dev/null || die '1分たたないうちに回し直した'
[ "$(polls)" = "$n" ] && [ "$(hv "$pa")" = "approve $old" ] || die "1分たたないうちに PR の head を GitHub に聞いた: $(( $(polls) - n )) 回"
mpoll "$pa"; : > "$tmp/dispatcher.log"; dispatcher
[ "$(polls)" = $((n + 1)) ] || die "PR の head を聞いた回数が1回でない: $(( $(polls) - n )) 回"
grep -q "$pa の PR https://github.com/example/app/pull/7 の head が gate の判定（approve）のあとに動いた（${old:0:12} → ${head:0:12}）。gate を回し直す" "$tmp/dispatcher.log" ||
    die "回し直しをログに書いていない: $(grep "$pa" "$tmp/dispatcher.log")"
[ "$(hv "$pa")" = "fix $head" ] && [ -e "$L/issues/$pa/library/gate/$head.json" ] || die "今の head で gate を回し直していない: $(hv "$pa")"
grep -q "^t7	\[gate\] $pa（.*）: gate が fix を返した。理由: .*report.json の commit が PR の head と違う" "$tmp/hc/sent" || die "回し直した gate の fix が担当に届いていない: $(grep "$pa" "$tmp/hc/sent")"
mpoll "$pa"; n=$(polls); dispatcher
[ "$(polls)" = "$n" ] || die 'fix を決めたあとも PR の head を聞いた（担当が ready し直す）'
ok 'merge: person の approve のあとに PR の head が動いたら、1分に1回だけ聞いて気づき、今の head で gate を回し直す（report.json と違うので fix を担当へ）'
# merge: auto の needs_human（gate の問いが開いている）
pn=$(newtask 'needs_human のあとに push される' auto)
to_gate "$pn" package/gate.json; old=$head
q=$L/issues/$pn/library/questions/1.json
[ "$(hv "$pn")" = "needs_human $old" ] && jq -e '.from == "gate"' "$q" >/dev/null || die "前提: needs_human の問いが無い: $(hv "$pn")"
merges=$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)
work "$pn" docs/z.md
mpoll "$pn"; : > "$tmp/dispatcher.log"; dispatcher
[ "$(hv "$pn")" = "fix $head" ] || die "needs_human のあとに head が動いても回し直していない: $(hv "$pn")"
kanban questions --json | jq -e --arg t "$pn" 'all(.[]; .id != $t)' >/dev/null || die "前の head の gate の問いが閉じていない: $(kanban questions --json | jq -c --arg t "$pn" 'map(select(.id == $t))')"
grep -q "PR の head が gate のあとに動いた（${old:0:12} → ${head:0:12}）。gate が今の head で回し直す" "$L/issues/$pn/questions.yaml" || die 'gate の問いに head が動いたことを答えていない'
[ ! -e "$L/issues/$pn/library/questions/2.json" ] || die '回し直した gate が fix なのに問いを出した'
[ "$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)" = "$merges" ] && [ ! -e "$tmp/gh/merged" ] || die 'merge: auto の課題を gate の見ていない head でマージした'
ok 'merge: auto の needs_human のあとに PR の head が動いたら、前の head の gate の問いを閉じて回し直し、見ていない head はマージしない'
# needs_human の問いが開いたまま、Chief of Staff が rebase を頼み、担当が doing に戻って新しい head を出した（review の外で head が動いた）。
# ready は前の head についての gate の問いを数えずに review にし、gate が今の head で決めたときにその問いを閉じる（答えに前と今の head）
pb=$(newtask 'needs_human のあとに rebase する' auto)
to_gate "$pb" package/gate.json; old=$head
[ "$(hv "$pb")" = "needs_human $old" ] && jq -e '.from == "gate"' "$L/issues/$pb/library/questions/1.json" >/dev/null || die "前提: needs_human の問いが無い: $(hv "$pb")"
restart "$pb"
work "$pb" package/gate.json
as_worker "$pb" report --pr https://github.com/example/app/pull/7
as_worker "$pb" review-request
dispatcher
review_by "$(kanban list --all --json | jq -r --arg p "$pb" '[.[] | select(.parent == $p)] | last | .id')" '[]'
as_worker "$pb" ready || die 'rebase のあとの ready が、前の head についての gate の問いで断った'
kanban show "$pb" --json | jq -e '.issue.status == "review"' >/dev/null || die 'rebase のあとの ready が review にしていない'
dispatcher
[ "$(hv "$pb")" = "needs_human $head" ] || die "今の head で gate を回していない: $(hv "$pb")"
kanban show "$pb" --json | jq -e '.questions[0].status == "received"' >/dev/null || die "前の head の gate の問いが閉じていない: $(kanban show "$pb" --json | jq -c '.questions')"
grep -q "PR の head が動いた（${old:0:12} → ${head:0:12}）。gate が今の head で決め直した" "$L/issues/$pb/questions.yaml" || die '前の head の gate の問いに、閉じた理由を答えていない'
# 事実の行の頭の語は andon gate が決める（RecordFact）。ここは今の head の記録を指す「gate の記録 <パス>」の行があることを見る
jq -e --arg p "$L/issues/$pb/library/gate/$head.json" '.from == "gate" and (.facts | any(startswith("gate の記録 ") and endswith(" " + $p)))' "$L/issues/$pb/library/questions/2.json" >/dev/null ||
    die '今の head の gate の問いを出していない'
kanban show "$pb" --json | jq -e '.questions[1].status == "open"' >/dev/null || die '今の head の gate の問いまで閉じた'
jq -se --arg t "$pb" '[.[] | select(.id == $t and .change.status == "review")] | last | .change.forced == [1]' "$L/log.jsonl" >/dev/null ||
    die 'ready が数えずに押し切った問いの番号を log.jsonl に残していない'
# gate が決めたときに閉じ損ねた前の head の問い（閉じる前にオーナーが答えた）は、merge が次に打たれたときに、オーナーの答えを残して受け取る
HC_TERMINAL_ID=dispatcher kanban ask "$pb" 'マージするか' --kind line >/dev/null
# 事実の行は 2026-11-20 までの語で書いた古い問いの形（読む側は古い語も受ける。ADR-0040 決めたこと 7）
jq -n --arg r "ゲートの記録 $L/issues/$pb/library/gate/$old.json" '{kind: "line", question: "マージするか", facts: [$r],
  options: [{label: "オーナーが PR を読んでマージする", pick: true}, {label: "担当に直させる"}], from: "gate"}' > "$L/issues/$pb/library/questions/3.json"
HC_TERMINAL_ID=t5 kanban answer "$pb#3" 'オーナーの答え' >/dev/null
mrun "$pb" >/dev/null
kanban show "$pb" --json | jq -e '.questions[2] | .status == "received" and .answer == "オーナーの答え"' >/dev/null ||
    die "閉じ損ねた前の head の問いを、次の merge で閉じ直していない: $(kanban show "$pb" --json | jq -c '.questions[2]')"
kanban show "$pb" --json | jq -e '.questions[1].status == "open"' >/dev/null || die '閉じ直すときに今の head の gate の問いまで閉じた'
# 後の検査の見回りがこの課題の PR の head を聞かないよう、fix で終える（needs_human のままだと1分ごとに聞き直す）
work "$pb" docs/z.md; mpoll "$pb"; dispatcher
[ "$(hv "$pb")" = "fix $head" ] || die "後始末: fix で終わっていない: $(hv "$pb")"
ok 'needs_human の問いが開いたまま rebase した担当は、前の head の問いを数えずに ready でき、gate が今の head で決めたときにその問いを閉じる'

# 4b. オーナーが GitHub でマージした（merge: person・needs_human）。dispatcher は review の課題の PR を1分に1回だけ見て、MERGED なら
#     gate の問い（from: gate。重なったものも）を「オーナーがマージした」と閉じ、andon issue finish し、通知 merged（by: person）を積む。
#     次の巡で影響（impact）とオーナーの手直し（corrections）も dispatcher がマージしたときと同じ道で拾う。dispatcher.md §2.7
watch_due() { # watch_due <id>: その課題だけ、次の見回りで GitHub に聞く（ほかの review の課題は聞かない）
    [ -f "$settled" ] || return 0
    jq --arg t "$1" --arg d "$due" 'with_entries(.value.at = (if .key == $t then $d else "2999-01-01T00:00:00Z" end))' "$settled" > "$settled.new" && mv "$settled.new" "$settled"
}
person_merges() { git -C "$tmp/origin" update-ref refs/heads/main "$head"; echo "$head" > "$tmp/gh/merged"; }
hm=$(newtask 'オーナーがマージする課題' person)
work "$hm" package/gate.json
as_worker "$hm" report --pr https://github.com/example/app/pull/7
as_worker "$hm" review-request
dispatcher
review_by "$(kanban list --all --json | jq -r --arg p "$hm" '[.[] | select(.parent == $p)] | last | .id')" '[]'
as_worker "$hm" ready
dispatcher
jq -e '.from == "gate"' "$L/issues/$hm/library/questions/1.json" >/dev/null || die '前提: needs_human の問いが無い'
HC_TERMINAL_ID=dispatcher "$andon" issue ask "$hm" - >/dev/null <<<'{"kind":"irreversible","question":"重なった gate の問い","options":[{"label":"オーナーが見る","pick":true},{"label":"x"}],"from":"gate"}'
HC_TERMINAL_ID=t5 kanban answer "$hm#2" 'オーナーが見る' >/dev/null   # answered のまま（received でない）
n=$(wc -l < "$tmp/gh/states")
watch_due "$hm"; dispatcher; dispatcher
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] || die "1分たたないうちに同じ PR を GitHub に聞き直した: $(( $(wc -l < "$tmp/gh/states") - n )) 回"
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die 'OPEN の PR の課題を片付けた'
merges=$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)
person_merges
sed -i '/^t5 /d' "$tmp/hc/live"   # オーナーのマージは Chief of Staff の留守でも拾う（ADR-0017 決めたこと 8）
watch_due "$hm"; dispatcher
echo "t5 chief-of-staff" >> "$tmp/hc/live"
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "留守の間にオーナーがマージしたのに片付けない: $(tail -3 "$tmp/dispatcher.log")"
kanban questions --json | jq -e --arg t "$hm" 'all(.[]; .id != $t)' >/dev/null || die "gate の問いが閉じていない: $(kanban questions --json | jq -c --arg t "$hm" 'map(select(.id == $t))')"
grep -q 'オーナーがマージした（https://github.com/example/app/pull/7）' "$L/issues/$hm/questions.yaml" || die 'gate の問いに「オーナーがマージした」と記録していない'
recorded "$hm" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}' >/dev/null && [ "$(pending "$hm" merged)" = null ] ||
    die "通知 merged（by: person）を記録だけにしていない: $(recorded "$hm" merged)"
[ "$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)" = "$merges" ] || die 'オーナーがマージする課題に gh pr merge を打った'
jq -e --arg t "$hm" 'has($t) | not' "$settled" >/dev/null || die '片付けたあとも settle の記録に残った'
jq -e 'has("watch") | not' "$state" >/dev/null || die 'dispatcher の控えにオーナーのマージの項目がある（settle の記録に持つ）'
emit <(recorded "$hm" merged) notification-merged-person.json
dispatcher
jq -e --arg t "$hm" '((.impact // {}) | has($t) | not) and ((.corrections // {}) | has($t) | not)' "$state" >/dev/null || die "オーナーのマージのあと impact・corrections を拾っていない: $(jq -c '{impact, corrections}' "$state")"
[ -e "$(cj "$hm")" ] || die 'オーナーのマージのあと corrections.json を書いていない'
ok 'オーナーが GitHub でマージしたら（Chief of Staff の留守でも）、次の見回りで gate の問い（重なったものも）を閉じて finish し、通知 merged（by: person）・impact・corrections を同じ道で回す。PR は1分に1回だけ見る'

# 4c. オーナーがマージしたが、gate の外の問い（担当の ask）が開いている → 片付けずに知らせる（1回）。問いが閉じたら次の巡で片付ける
hm=$(newtask '問いが残る課題' person)
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 "$andon" issue ask "$hm" - >/dev/null <<<'{"kind":"users","question":"どちらにする","options":[{"label":"A","pick":true},{"label":"B"}],"from":"issue"}')
# 入口の守り（ready・kanban review）を通らずに問いを残して review に入った課題として、--force で直に打つ
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 kanban review "$hm" --force >/dev/null)
HC_TERMINAL_ID=t5 kanban set "$hm" review=off >/dev/null
dispatcher
person_merges
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die 'gate の外の問いが開いているのに片付けた'
pending "$hm" merged | jq -e --arg q "$hm#1" '.detail.by == "person" and (.detail.held | contains($q))' >/dev/null || die "片付けない理由を知らせていない: $(pending "$hm" merged)"
emit <(pending "$hm" merged) notification-merged-held.json
jq --arg t "$hm" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
n=$(wc -l < "$tmp/gh/states"); dispatcher
[ "$(pending "$hm" merged)" = null ] || die '同じ理由で二度知らせた'
[ "$(wc -l < "$tmp/gh/states")" = "$n" ] || die 'MERGED と分かった PR を GitHub に聞き直した'
HC_TERMINAL_ID=t5 kanban answer "$hm#1" A >/dev/null; HC_TERMINAL_ID=t7 kanban received "$hm#1" >/dev/null
dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "問いが閉じたのに片付けない: $(tail -3 "$tmp/dispatcher.log")"
recorded "$hm" merged | jq -e '.detail.by == "person" and (.detail | has("held") | not)' >/dev/null || die "片付けた通知を記録していない: $(recorded "$hm" merged)"
ok 'オーナーがマージしても gate の外の問いが開いていれば片付けずに1回だけ知らせ、問いが閉じたら次の巡で片付ける（GitHub には聞き直さない）'

# 4d. PR がマージされずに閉じた → 片付けず、通知 closed を閉じるたびに1回。閉じた PR も1分に1回聞き直し、
#     オーナーが開き直してマージしたら片付ける（dispatcher.md §2.7）
hm=$(newtask '閉じた PR の課題' person)
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 kanban review "$hm" >/dev/null)
HC_TERMINAL_ID=t5 kanban set "$hm" review=off >/dev/null
dispatcher
touch "$tmp/gh/closed"
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die '閉じた PR の課題を片付けた'
pending "$hm" closed | jq -e '.detail == {pr: "https://github.com/example/app/pull/7"}' >/dev/null || die "通知 closed が無い: $(pending "$hm" closed)"
emit <(pending "$hm" closed) notification-closed.json
jq --arg t "$hm" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
n=$(wc -l < "$tmp/gh/states"); dispatcher; dispatcher
[ "$(wc -l < "$tmp/gh/states")" = "$n" ] || die "1分たたないうちに閉じた PR を聞き直した: $(( $(wc -l < "$tmp/gh/states") - n )) 回"
watch_due "$hm"; dispatcher
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] || die "1分たっても閉じた PR を聞き直さない: $(( $(wc -l < "$tmp/gh/states") - n )) 回"
[ "$(pending "$hm" closed)" = null ] || die '閉じたままの PR を二度知らせた'
rm "$tmp/gh/closed"   # オーナーが開き直した
watch_due "$hm"; dispatcher
jq -e --arg t "$hm" '.[$t].state // "" | . == ""' "$settled" >/dev/null || die "開き直した PR を CLOSED のまま記録している: $(cat "$settled")"
touch "$tmp/gh/closed"; watch_due "$hm"; dispatcher   # また閉じた
pending "$hm" closed | jq -e '.detail == {pr: "https://github.com/example/app/pull/7"}' >/dev/null || die "開き直したあと閉じたのに知らせない: $(pending "$hm" closed)"
jq --arg t "$hm" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
rm "$tmp/gh/closed"; person_merges   # 開き直してマージした
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "閉じた PR を開き直してマージしたのに片付けない: $(grep "$hm" "$tmp/dispatcher.log" | tail -3)"
recorded "$hm" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}' >/dev/null || die "通知 merged（by: person）を記録していない: $(recorded "$hm" merged)"
jq -e --arg t "$hm" 'has($t) | not' "$settled" >/dev/null || die "片付けたあとも settle の記録に残った: $(cat "$settled")"
rm -f "$tmp/gh/merged"   # 後の節は PR 7 を OPEN から使う
ok 'PR がマージされずに閉じたら片付けず、通知 closed を閉じるたびに1回積む。閉じた PR も1分に1回聞き直し、開き直してマージしたら片付ける'

# 4d-2. merge: auto の課題を、dispatcher の gh pr merge の直前にオーナーが同じ head でマージした → gh pr merge は失敗するが、
#     読み直して MERGED・head が gate と同じなら問い（irreversible の誤報）を出さず、オーナーがマージしたときの道（§2.7）で片付ける
hm=$(newtask 'オーナーが先にマージする' auto)
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
as_worker "$hm" review-request
dispatcher
review_by "$hm-r1" '[]'
as_worker "$hm" ready
touch "$tmp/gh/race"; watch_due "$hm"   # doing のうちに聞いた分は待たせない（ほかの課題は聞かない）
merges=$(wc -l < "$tmp/gh/merges"); n=$(wc -l < "$tmp/gh/states")
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = $((merges + 1)) ] || die '前提: gh pr merge を打っていない'
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "オーナーが先にマージしたのに片付けない: $(tail -3 "$tmp/dispatcher.log")"
! grep -q " $hm をマージしない" "$tmp/dispatcher.log" && [ "$(pending "$hm" asked)" = null ] && [ "$(recorded "$hm" asked)" = null ] || die "オーナーが先にマージしただけで問いを出した: $(grep "$hm" "$tmp/dispatcher.log" | tail -3)"
jq -e '.decision == "approve" and (.merged // false) == false' "$L/issues/$hm/library/gate/$head.json" >/dev/null || die "オーナーのマージなのに gate の記録に merged を書いた: $(cat "$L/issues/$hm/library/gate/$head.json")"
recorded "$hm" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}' >/dev/null || die "通知 merged（by: person）を記録していない: $(recorded "$hm" merged)"
jq -e --arg t "$hm" '[.pending[] | select(.issue == $t and (.kind == "asked" or .kind == "merged"))] == []' "$state" >/dev/null && jq -e --arg t "$hm" '(.[$t].approved // "") == ""' "$mergerec" >/dev/null ||
    die "問い・merged を待ちに積んだ・approve の待ちを残した: $(jq -c .pending "$state") $(cat "$mergerec")"
jq -e --arg t "$hm" 'has($t) | not' "$settled" >/dev/null || die "片付けたあとも settle の記録に残った: $(cat "$settled")"
[ "$(wc -l < "$tmp/gh/states")" = $((n + 2)) ] || die "読み直しと settle のほかに同じ PR を聞いた: $(( $(wc -l < "$tmp/gh/states") - n )) 回"
ok 'merge: auto でもオーナーが先に同じ head でマージしていれば、gh pr merge の失敗を問いにせず §2.7 の道で片付ける（gate の記録の merged は書かない）'

# merge: auto でマージしたあと、受け取られていない担当の問いで finish が断られても、片付けかけ（review のまま worktree・端末が消える）を残さない
# （A-0002・A-0001 の repro-b3）。finish は何も消さずに止まり、dispatcher は「片付けに失敗した」を問いにする
qa=$(newtask '問いを残したまま通る' auto)
qwt=$ANDON_WORKSPACE/.worktrees/app/demo-$qa
work "$qa" docs/a.md
(cd "$qwt" && HC_TERMINAL_ID=t7 "$andon" issue ask "$qa" - <<<'{"kind":"users","question":"どちらにするか","options":[{"label":"A","pick":true},{"label":"B"}],"from":"issue"}' >/dev/null)
as_worker "$qa" report --pr https://github.com/example/app/pull/7
as_worker "$qa" review-request
dispatcher
review_by "$qa-r1" '[]'
# andon issue ready は受け取られていない問いがあれば断る（入口の守り）。ここでは ready を通らずに review へ入った課題として、
# finish の守り（奥の守り）を確かめる。kanban review も問いを確かめて断るので、--force で直に打つ
! as_worker "$qa" ready 2>/dev/null || die 'ready が受け取られていない問いを残したまま通った'
kanban show "$qa" --json | jq -e '.issue.status == "doing"' >/dev/null || die "ready が断ったのに課題が doing でない: $(kanban show "$qa" --json | jq -c .issue)"
! (cd "$qwt" && HC_TERMINAL_ID=t7 kanban review "$qa" >/dev/null 2>&1) || die 'kanban review が受け取られていない問いを残したまま通った'
(cd "$qwt" && HC_TERMINAL_ID=t7 kanban review "$qa" --force >/dev/null)
dispatcher
[ -e "$tmp/gh/merged" ] || die "前提: マージしていない: $(grep "$qa" "$tmp/dispatcher.log" | tail -3)"
kanban show "$qa" --json | jq -e '.issue.status == "review"' >/dev/null || die "問いが残るのに課題が review でない: $(kanban show "$qa" --json | jq -c .issue)"
[ -d "$qwt" ] && git -C "$app" show-ref --verify --quiet "refs/heads/demo/$qa" && grep -q "^t7 $qa\$" "$tmp/hc/live" \
  && [ -d "$ANDON_WORKSPACE/.worktrees/app/demo-$qa-r1" ] && kanban show "$qa-r1" --json | jq -e '.issue.status != "done"' >/dev/null \
  || die "断られた finish が片付けかけを残した（worktree・branch・端末・レビュアー）: $(grep "$qa" "$tmp/dispatcher.log" | tail -3)"
jq -e --arg q "$qa#1（open）" '.from == "gate" and .kind == "irreversible" and any(.facts[]; contains($q))' "$L/issues/$qa/library/questions/2.json" >/dev/null \
  && pending "$qa" asked | jq -e '.detail.n == 2' >/dev/null || die "片付けに失敗したことを問いにしていない: $(cat "$L/issues/$qa/library/questions/2.json" 2>&1)"
# 問いを受け取れば Chief of Staff の finish で片付く
for q in $(kanban questions --json | jq -r --arg t "$qa" '.[] | select(.id == $t) | "\(.id)#\(.n) \(.status)"' | tr ' ' :); do
    [ "${q#*:}" = answered ] || kanban answer "${q%%:*}" x >/dev/null; kanban received "${q%%:*}" >/dev/null
done
HC_TERMINAL_ID=t5 "$andon" issue finish "$qa" >/dev/null || die '問いを受け取ったあとの finish が通らない'
[ ! -e "$qwt" ] && kanban show "$qa" --json | jq -e '.issue.status == "done"' >/dev/null || die '受け取ったあとに片付かない'
ok 'merge: auto のマージのあと担当の問いで finish が断られても、review のまま worktree・branch・端末・レビュアーを消さずに止まり、片付けの失敗を問いにする'

# 4e. report.json の pr が issue.yaml の repo と違う repo の PR なら、それがマージされていても見ない・片付けない
hm=$(newtask '別の repo の PR' person)
work "$hm" docs/a.md
# andon issue report は違う repo の PR を断る（ADR-0034 決めたこと 11）ので、report.json は手で書く
mkdir -p "$L/issues/$hm/library"
jq -n --arg h "$head" '{commit: $h, pr: "https://github.com/example/other/pull/7"}' > "$L/issues/$hm/library/report.json"
r=0; as_worker "$hm" report --pr https://github.com/example/other/pull/7 2>/dev/null || r=$?
[ "$r" = 1 ] && jq -e '.pr == "https://github.com/example/other/pull/7"' "$L/issues/$hm/library/report.json" >/dev/null || die "andon issue report が違う repo の PR を断らない（終了コード $r）"
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 kanban report "$hm" <<<'## Report' >/dev/null && HC_TERMINAL_ID=t7 kanban review "$hm" >/dev/null)
HC_TERMINAL_ID=t5 kanban set "$hm" review=off >/dev/null
person_merges
n=$(wc -l < "$tmp/gh/states"); dispatcher; watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die '別の repo の PR がマージされたのを見て片付けた'
[ "$(wc -l < "$tmp/gh/states")" = "$n" ] && [ "$(pending "$hm" merged)" = null ] && [ "$(recorded "$hm" merged)" = null ] || die '別の repo の PR を GitHub に聞いた・知らせた'
rm -f "$tmp/gh/merged"
HC_TERMINAL_ID=t5 "$andon" issue finish "$hm" >/dev/null
ok 'issue.yaml の repo と違う repo の PR は、マージされていても見ず、片付けない'

# 4f. andon issue settle は 2 回打っても同じ結果（GitHub に聞くのは 1 分に 1 回・通知は 1 回・finish は 1 回）。dispatcher が
#     自分でマージした PR（gate の記録 library/gate/<head>.json の merged が true）は、dispatcher の控えを読まずに見分けて片付けない
#     （同じ巡で finish が失敗してもオーナーのマージと誤報しない）。ADR-0017 決めたこと 5 の (d)・受け入れ条件 7
settle() { freeze; HC_TERMINAL_ID=dispatcher "$andon" issue settle "$1"; }
hm=$(newtask 'dispatcher がマージした課題' person)
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$hm" && HC_TERMINAL_ID=t7 kanban review "$hm" >/dev/null)
HC_TERMINAL_ID=t5 kanban set "$hm" review=off >/dev/null
person_merges
mkdir -p "$L/issues/$hm/library/gate"
jq -n --arg h "$head" '{head: $h, decision: "approve", merged: true}' > "$L/issues/$hm/library/gate/$head.json"
watch_due "$hm"; n=$(wc -l < "$tmp/gh/states")
a=$(settle "$hm"); b=$(settle "$hm")
[ "$a" = "$b" ] && jq -e '.state == "auto" and .notify == []' <<<"$a" >/dev/null || die "dispatcher がマージした PR をオーナーのマージとした・2 回で結果が違う: $a / $b"
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] || die 'MERGED と分かった PR を GitHub に聞き直した'
kanban show "$hm" --json | jq -e '.issue.status == "review"' >/dev/null || die 'dispatcher がマージした課題を settle が片付けた'
rm "$L/issues/$hm/library/gate/$head.json"   # オーナーがマージした（gate の記録に merged が無い）
a=$(settle "$hm"); b=$(settle "$hm")
jq -e --arg h "$head" '.state == "finished" and .notify == [{kind: "merged", detail: {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}}]' <<<"$a" >/dev/null ||
    die "オーナーのマージを片付けて通知を返さない: $a"
jq -e '.state == "skip" and .notify == []' <<<"$b" >/dev/null || die "2 回目にまた片付けた・知らせた: $b"
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die 'オーナーのマージを settle が片付けない'
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] || die '2 回目に GitHub に聞き直した'
rm -f "$tmp/gh/merged"
ok 'andon issue settle は 2 回打っても同じ（聞く・知らせる・片付けるのは 1 回）。gate の記録の merged が true なら dispatcher のマージとして片付けない'

# 4g. 担当が doing のうちにオーナーが GitHub で PR をマージした → review のときと同じ道で片付ける（done・端末・worktree）。
#     受け取られていない問い・commit していない変更があれば片付けず、理由つきの通知 merged（held）を理由ごとに1回だけ積む。dispatcher.md §2.7
hm=$(newtask 'doing のうちにマージされる' person)
dwt=$ANDON_WORKSPACE/.worktrees/app/demo-$hm
work "$hm" docs/a.md
as_worker "$hm" report --pr https://github.com/example/app/pull/7
(cd "$dwt" && HC_TERMINAL_ID=t7 "$andon" issue ask "$hm" - >/dev/null <<<'{"kind":"users","question":"どちらにする","options":[{"label":"A","pick":true},{"label":"B"}],"from":"issue"}')
n=$(wc -l < "$tmp/gh/states")
watch_due "$hm"; dispatcher
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] || die "doing の課題の PR を聞いていない・聞きすぎた: $(( $(wc -l < "$tmp/gh/states") - n )) 回"
kanban show "$hm" --json | jq -e '.issue.status == "doing"' >/dev/null || die 'OPEN の PR の doing の課題を片付けた'
person_merges
watch_due "$hm"; dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "doing"' >/dev/null && [ -d "$dwt" ] && grep -q "^t7 $hm\$" "$tmp/hc/live" || die '問いが開いているのに doing の課題を片付けた'
pending "$hm" merged | jq -e --arg q "$hm#1" '.detail.by == "person" and (.detail.held | contains($q))' >/dev/null || die "片付けない理由（問い）を知らせていない: $(pending "$hm" merged)"
jq --arg t "$hm" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
n=$(wc -l < "$tmp/gh/states"); dispatcher
[ "$(pending "$hm" merged)" = null ] && [ "$(wc -l < "$tmp/gh/states")" = "$n" ] || die 'doing の課題で同じ理由を二度知らせた・MERGED の PR を聞き直した'
HC_TERMINAL_ID=t5 kanban answer "$hm#1" A >/dev/null; HC_TERMINAL_ID=t7 kanban received "$hm#1" >/dev/null
echo wip >> "$dwt/docs/a.md"   # 担当が commit していない変更
dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "doing"' >/dev/null && [ -d "$dwt" ] && grep -q "^t7 $hm\$" "$tmp/hc/live" || die 'commit していない変更があるのに doing の課題を片付けた'
pending "$hm" merged | jq -e '.detail.by == "person" and (.detail.held | contains("commit していない変更"))' >/dev/null || die "片付けない理由（変更）を知らせていない: $(pending "$hm" merged)"
git -C "$dwt" checkout -q -- docs/a.md
dispatcher
kanban show "$hm" --json | jq -e '.issue.status == "done"' >/dev/null || die "doing のうちにオーナーがマージした課題を片付けない: $(tail -3 "$tmp/dispatcher.log")"
[ ! -e "$dwt" ] && ! grep -q "^t7 $hm\$" "$tmp/hc/live" || die 'doing の課題を done にしたが worktree・端末が残った'
recorded "$hm" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}' >/dev/null || die "通知 merged（by: person）を記録していない: $(recorded "$hm" merged)"
jq -e --arg t "$hm" 'has($t) | not' "$settled" >/dev/null || die "片付けたあとも settle の記録に残った: $(cat "$settled")"
rm -f "$tmp/gh/merged"
ok 'doing のうちにオーナーがマージした PR も、review のときと同じ道で片付ける（done・端末・worktree）。問い・commit していない変更があれば片付けず、理由ごとに1回だけ知らせる'

# 4h. Chief Engineer の課題（profile chief-engineer）の文書の PR をオーナーがマージした → gate は回さず、review にいれば settle で片付け、
#     通知 merged（by: person）を届ける（CE の課題に impact は回らない）。doing の間（オーナーと話している）は PR を読んで印を揃えるだけで、
#     片付けない（マージされたことは記録し、review に入ったら GitHub に聞き直さずに片付ける）。dispatcher.md §2.7・ADR-0056 決めたこと 6
ce=$(HC_TERMINAL_ID=t5 kanban add '設計を文書にする' --profile chief-engineer --repo github.com/example/app)
cwt=$ANDON_WORKSPACE/.worktrees/app/demo-$ce
git -C "$app" fetch -q origin
git -C "$app" worktree add -q -b "demo/$ce" "$cwt" origin/main
HC_TERMINAL_ID=t5 kanban set "$ce" "branch=demo/$ce" "worktree=$cwt" >/dev/null   # andon chief-engineer の代わり
(cd "$cwt" && HC_TERMINAL_ID=t7 kanban start "$ce" >/dev/null); echo "t7 $ce" >> "$tmp/hc/live"
work "$ce" docs/adr/0001-design.md
as_worker "$ce" report --pr https://github.com/example/app/pull/7
person_merges
n=$(wc -l < "$tmp/gh/states"); watch_due "$ce"; dispatcher
kanban show "$ce" --json | jq -e '.issue.status == "doing"' >/dev/null && [ "$(pending "$ce" merged)" = null ] || die 'doing の Chief Engineer の課題の PR を片付けた・知らせた'
[ "$(wc -l < "$tmp/gh/states")" = $((n + 1)) ] && jq -e --arg t "$ce" '.[$t].state == "MERGED"' "$settled" >/dev/null || die "doing の Chief Engineer の課題の PR を読まない・マージを記録しない: $(jq -c --arg t "$ce" '.[$t]' "$settled" 2>/dev/null)"
(cd "$cwt" && HC_TERMINAL_ID=t7 kanban review "$ce" >/dev/null)
merges=$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)
dispatcher
kanban show "$ce" --json | jq -e '.issue.status == "done"' >/dev/null || die "オーナーがマージした Chief Engineer の文書の PR を片付けない: $(grep "$ce" "$tmp/dispatcher.log" | tail -3)"
[ ! -e "$cwt" ] && ! grep -q "^t7 $ce\$" "$tmp/hc/live" || die 'Chief Engineer の課題を done にしたが worktree・端末が残った'
pending "$ce" merged | jq -e --arg h "$head" '.detail == {pr: "https://github.com/example/app/pull/7", sha: $h, by: "person"}' >/dev/null ||
    die "通知 merged（by: person）を Chief of Staff に届けない: $(pending "$ce" merged) $(recorded "$ce" merged)"
[ "$(recorded "$ce" merged)" = null ] || die 'Chief Engineer の merged を記録だけにした'
[ ! -e "$L/issues/$ce/library/gate" ] && [ "$(cat "$tmp/gh/merges" 2>/dev/null | wc -l)" = "$merges" ] || die 'Chief Engineer の課題に gate を回した・gh pr merge を打った'
jq --arg t "$ce" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"
dispatcher
[ "$(pending "$ce" merged)" = null ] || die '片付けた Chief Engineer の課題をまた知らせた'
rm -f "$tmp/gh/merged"
ok 'Chief Engineer の文書の PR をオーナーがマージしたら、gate を回さずに review の課題を片付け、通知 merged（by: person）を届ける。doing の間は片付けない'

# 4i. ADR-0056 A6（揃える場面）: dispatcher は PR を読むたびに（settle の 1 分に 1 回の読み）、開いている PR の head の印を課題の状態に揃える。
#     オーナー待ちに残った前のバージョンの赤は緑に、判定のあと doing に戻った課題の緑は赤に、Chief Engineer の文書の PR は緑に。
#     判定の無い head・閉じた PR には書かない。揃っていれば書かない（ADR-0056 決めたこと 3・5・6）
marks() { grep -c "^status .*/statuses/$1 " "$tmp/gh/posted" || true; }   # その head に書いた印の数
lastmark() { grep "^status .*/statuses/$1 " "$tmp/gh/posted" | tail -1; }
al=$(newtask '印を揃える課題' person)
work "$al" package/gate.json
as_worker "$al" report --pr https://github.com/example/app/pull/7
as_worker "$al" review-request
dispatcher
review_by "$(kanban list --all --json | jq -r --arg p "$al" '[.[] | select(.parent == $p)] | last | .id')" '[]'
as_worker "$al" ready
dispatcher
jq -e '.from == "gate"' "$L/issues/$al/library/questions/1.json" >/dev/null || die '前提: needs_human の問いが無い'
lastmark "$head" | grep -q 'state=success .*description=andon gate: メンテナーの判断が要る変更です（自動でマージしてよい範囲を超える変更） -f target_url=https://' ||
    die "gate がオーナー待ちに緑の印（コメントへのリンクつき）を書かない: $(lastmark "$head")"
# 前のバージョンが書いた赤（オーナー待ちなのに failure）が残っている → 次に読んだとき緑に直す
echo failure > "$tmp/gh/marks/$head"; n=$(marks "$head"); : > "$tmp/dispatcher.log"
watch_due "$al"; dispatcher
[ "$(marks "$head")" = $((n + 1)) ] && lastmark "$head" | grep -q 'context=andon/gate -f state=success -f description=andon gate: メンテナーの判断が要る変更です（自動でマージしてよい範囲を超える変更）$' ||
    die "オーナー待ちの赤を緑に直さない: $(lastmark "$head")"
grep -q "$al の PR の印を success に揃えた" "$tmp/dispatcher.log" || die "揃えたことをログに書かない: $(cat "$tmp/dispatcher.log")"
n=$(marks "$head"); watch_due "$al"; dispatcher
[ "$(marks "$head")" = "$n" ] || die '揃っている印を書き直した'
ok 'オーナー待ちの PR に残った赤は、次に PR を読んだとき緑（理由の括弧つき）に直し、揃っていれば書き直さない（ADR-0056 A6）'
# 判定のあと doing に戻った（オーナーが直させると答えた）→ 緑を赤に書き換える。赤になったら書き直さない
HC_TERMINAL_ID=t5 kanban answer "$al#1" '直させる' >/dev/null; HC_TERMINAL_ID=t7 kanban received "$al#1" >/dev/null
restart "$al"; n=$(marks "$head")
watch_due "$al"; dispatcher
[ "$(marks "$head")" = $((n + 1)) ] && lastmark "$head" | grep -q 'state=failure -f description=andon gate: 作った側が直しています$' ||
    die "doing に戻った課題の緑を赤にしない: $(lastmark "$head")"
watch_due "$al"; dispatcher
[ "$(marks "$head")" = $((n + 1)) ] || die '赤くした印をまた書いた'
ok 'gate が緑を付けたあと課題が doing に戻ったら、次に PR を読んだとき赤（作った側が直しています）に書き換える（ADR-0056 A6）'
# 同じ head のまま review に入り直し、今の回の gate がまだ決めていない → 前の回の判定（緑）を書き戻さない
# （前の回の判定・doing・review が同じ秒に並んでも見分ける。時刻でなく merge の記録の行で照らす）
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$al" && HC_TERMINAL_ID=t7 kanban review "$al" >/dev/null)
watch_due "$al"; HC_TERMINAL_ID=dispatcher "$andon" issue settle "$al" >/dev/null
[ "$(marks "$head")" = $((n + 1)) ] || die "review に入り直した回で、前の回の判定から印を書き戻した: $(lastmark "$head")"
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$al" && HC_TERMINAL_ID=t7 kanban start "$al" >/dev/null)
ok '同じ head のまま review に入り直しても、今の回の gate が決めるまで前の回の判定から印を書き戻さない'
# 担当が push した（gate の判定の無い head）→ 書かない。印の無い head は doing でも review でも書かない
old=$head; work "$al" package/gate.json; n=$(wc -l < "$tmp/gh/posted")
watch_due "$al"; dispatcher
as_worker "$al" report; (cd "$ANDON_WORKSPACE/.worktrees/app/demo-$al" && HC_TERMINAL_ID=t7 kanban review "$al" >/dev/null)
freeze; HC_TERMINAL_ID=dispatcher "$andon" issue settle "$al" >/dev/null   # review でも、判定の無い head（gate の前）
[ "$(wc -l < "$tmp/gh/posted")" = "$n" ] && [ "$(marks "$head")" = 0 ] || die "判定の無い head に印を書いた: $(tail -2 "$tmp/gh/posted")"
ok '判定の無い head（gate の前）には doing でも review でも印を書かない'
HC_TERMINAL_ID=t5 kanban done "$al" >/dev/null; sed -i "/^t7 $al\$/d" "$tmp/hc/live"   # あとの場面が数える動いている課題に残さない
# Chief Engineer の文書の PR（gate を通らない）は review に入ったら緑。閉じた PR には書かない
ce2=$(HC_TERMINAL_ID=t5 kanban add '設計の文書の印' --profile chief-engineer --repo github.com/example/app)
cwt2=$ANDON_WORKSPACE/.worktrees/app/demo-$ce2
git -C "$app" fetch -q origin
git -C "$app" worktree add -q -b "demo/$ce2" "$cwt2" origin/main
HC_TERMINAL_ID=t5 kanban set "$ce2" "branch=demo/$ce2" "worktree=$cwt2" >/dev/null
(cd "$cwt2" && HC_TERMINAL_ID=t7 kanban start "$ce2" >/dev/null); echo "t7 $ce2" >> "$tmp/hc/live"
work "$ce2" docs/adr/0002-design.md
as_worker "$ce2" report --pr https://github.com/example/app/pull/7
(cd "$cwt2" && HC_TERMINAL_ID=t7 kanban review "$ce2" >/dev/null)
touch "$tmp/gh/closed"; watch_due "$ce2"; dispatcher; rm "$tmp/gh/closed"
[ "$(marks "$head")" = 0 ] || die "閉じた PR に印を書いた: $(lastmark "$head")"
jq --arg t "$ce2" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"   # 閉じた通知は見ない
watch_due "$ce2"; dispatcher
[ "$(marks "$head")" = 1 ] && lastmark "$head" | grep -q 'state=success -f description=andon gate: 仕様・設計の文書です。メンテナーがマージします$' ||
    die "Chief Engineer の文書の PR に緑を書かない: $(lastmark "$head")"
[ ! -e "$L/issues/$ce2/library/gate" ] || die 'Chief Engineer の課題に gate を回した'
(cd "$cwt2" && HC_TERMINAL_ID=t7 kanban start "$ce2" >/dev/null)   # オーナーと話し直す（doing に戻った）
watch_due "$ce2"; dispatcher
[ "$(marks "$head")" = 2 ] && lastmark "$head" | grep -q 'state=failure -f description=andon gate: 作った側が直しています$' && kanban show "$ce2" --json | jq -e '.issue.status == "doing"' >/dev/null ||
    die "doing に戻った Chief Engineer の文書の PR の緑を赤にしない: $(lastmark "$head")"
ok 'Chief Engineer の文書の PR は、review に入って PR を読んだとき緑（仕様・設計の文書です）、doing に戻ったら赤。閉じた PR には書かない（ADR-0056 A6）'
HC_TERMINAL_ID=t5 kanban done "$ce2" >/dev/null
sed -i "/^t7 $ce2\$/d" "$tmp/hc/live"

# 4j. ADR-0056 A6（止める場面）: Chief of Staff が止めた課題（予約キー hold）は gate を回さない。判定のあとに止めたら、判定を捨てて
#     gate の問いを閉じ、緑を黄（メンテナーの確認を待っています）に揃える。外したら次の巡で gate から回し直す。理由の課題が done に
#     なったら、止めが残っている間だけ1回知らせる（決めたこと 7・9）
hd=$(newtask '止める課題' person)
hwhy=$(HC_TERMINAL_ID=t5 kanban add '止めた理由の問い' --profile claude --repo github.com/example/app)
work "$hd" package/gate.json
as_worker "$hd" report --pr https://github.com/example/app/pull/7
as_worker "$hd" review-request
dispatcher
review_by "$(kanban list --all --json | jq -r --arg p "$hd" '[.[] | select(.parent == $p)] | last | .id')" '[]'
as_worker "$hd" ready
dispatcher
kanban show "$hd" --json | jq -e '.questions[0].status == "open"' >/dev/null && lastmark "$head" | grep -q 'state=success' || die '前提: gate の問いと緑が無い'
hrec=$L/issues/$hd/library/gate/$head.json; hat=$(jq -r .at "$hrec")   # gate の記録は head ごとに1つ（回し直すと上書き）
HC_TERMINAL_ID=t5 kanban set "$hd" "hold=$hwhy" >/dev/null   # 口（andon issue hold）を通さずに書いても、dispatcher が揃える
n=$(marks "$head"); : > "$tmp/dispatcher.log"
watch_due "$hd"; dispatcher
[ "$(jq -r .at "$hrec")" = "$hat" ] || die '止めた課題に gate を回した'   # 秒の時刻なので、回し直しの見張りは問いで見る
kanban show "$hd" --json | jq -e '.questions[0].status == "received"' >/dev/null || die "止めたのに gate の問いを閉じない: $(kanban show "$hd" --json | jq -c '.questions')"
jq -e --arg t "$hd" '.[$t] | has("decision") | not' "$mergerec" >/dev/null || die "止めたのに判定を捨てない: $(jq -c --arg t "$hd" '.[$t]' "$mergerec")"
[ "$(marks "$head")" = $((n + 1)) ] && lastmark "$head" | grep -q 'state=pending -f description=andon gate: メンテナーの確認を待っています$' ||
    die "止めた課題の緑を黄にしない: $(lastmark "$head")"
grep -q "$hd は gate の判定（needs_human）のあとに止められた" "$tmp/dispatcher.log" || die "判定を捨てたことをログに書かない: $(cat "$tmp/dispatcher.log")"
n=$(marks "$head"); watch_due "$hd"; dispatcher
[ "$(marks "$head")" = "$n" ] && [ "$(jq -r .at "$hrec")" = "$hat" ] || die '止めている間に gate を回したか、黄を書き直した'
ok '判定のあとに止めた課題は、次の巡で判定を捨てて gate の問いを閉じ、緑を黄にする。止めている間は gate を回さない（ADR-0056 A6）'
HC_TERMINAL_ID=t5 "$andon" issue unhold "$hd" >/dev/null
watch_due "$hd"; dispatcher
kanban show "$hd" --json | jq -e '.questions[1] | .status == "open"' >/dev/null ||
    die "外したのに gate を回し直さない: $(kanban show "$hd" --json | jq -c '.questions')"
lastmark "$head" | grep -q 'state=success' || die "外したあと gate が印を書かない: $(lastmark "$head")"
ok '止めを外したら、次の巡で gate を今の head で回し直す（ADR-0056 A6）'
# 止めてから外すまでに dispatcher が一度も見なかったとき: 止める前の判定から緑に戻さず、判定を捨てて今の head で gate から回す
lastmark "$head" | grep -q 'state=success' || die '前提: 回し直した gate の緑が無い'
HC_TERMINAL_ID=t5 "$andon" issue hold "$hd" "$hwhy" >/dev/null || die '止める口が落ちた'
lastmark "$head" | grep -q 'state=pending' || die "止める口が黄を書かない: $(lastmark "$head")"
HC_TERMINAL_ID=t5 "$andon" issue unhold "$hd" >/dev/null
: > "$tmp/dispatcher.log"; watch_due "$hd"; dispatcher
kanban show "$hd" --json | jq -e '.questions[1].status == "received" and .questions[2].status == "open"' >/dev/null ||
    die "止めてすぐ外したのに、止める前の判定を捨てて gate を回し直さない: $(kanban show "$hd" --json | jq -c '.questions')"
grep -q "$hd は gate の判定（needs_human）のあとに止められた" "$tmp/dispatcher.log" || die "判定を捨てたことをログに書かない: $(cat "$tmp/dispatcher.log")"
ok '止めてから外すまでに dispatcher が見なかったときも、止める前の判定を捨てて gate を今の head で回し直す（ADR-0056 A6）'
HC_TERMINAL_ID=t5 "$andon" issue hold "$hd" "$hwhy" >/dev/null
HC_TERMINAL_ID=t5 kanban done "$hwhy" >/dev/null
dispatcher
pending "$hd" hold | jq -e --arg r "$hwhy" '.detail == {reason: $r}' >/dev/null || die "理由の課題が done になったのに知らせない: $(pending "$hd" hold)"
holds() { jq --arg t "$hd" '[.pending[] | select(.issue == $t and .kind == "hold")] | length' "$state"; }
[ "$(holds)" = 1 ] || die "知らせが1件でない: $(holds)"
dispatcher
[ "$(holds)" = 1 ] || die '理由の課題の done を2度知らせた'
ok '止めた理由の課題が done になっても止めは外れず、止めが残っていれば Chief of Staff に1回だけ知らせる（ADR-0056 A6）'
HC_TERMINAL_ID=t5 "$andon" issue unhold "$hd" >/dev/null; HC_TERMINAL_ID=t5 kanban done "$hd" >/dev/null; sed -i "/^t7 $hd\$/d" "$tmp/hc/live"
jq --arg t "$hd" '.pending |= map(select(.issue != $t))' "$state" > "$state.new" && mv "$state.new" "$state"

# 5. 上限を超えた回（extra）は起動しない（Chief of Staff が決める）
HC_TERMINAL_ID=t5 kanban set "$id" review_rounds=1 >/dev/null
restart "$id"
work "$id" docs/a.md
as_worker "$id" report
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '上限を超えた回でレビュアーを起動した'
pending "$id" report | jq -e '.detail.report | endswith("/report.md")' >/dev/null || die '通知 report が無い'
grep -q '追加の回（2 回目／上限 1）' "$tmp/dispatcher.log" || die '起動しない理由がログに無い'
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '上限を超えた回を二度目の見回りで起動した'
HC_TERMINAL_ID=t5 kanban set "$id" review_rounds=2 >/dev/null
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die 'Chief of Staff が review_rounds を上げたのに起動しない'
ok '上限を超えた回（extra）は起動せず通知 report。Chief of Staff が review_rounds を上げたら次の見回りで起動する'

# 5b. deliverable: candidates（調べる課題。Chief of Staff が書いた）はインナーループを通らない: review_request があってもレビュアーを起動せず、
#     review に入っても andon gate を回さない（通知は届ける）。ADR-0006 受け入れ条件 3
sid=$(newtask '誤りの候補を調べる' person)
HC_TERMINAL_ID=t5 kanban set "$sid" deliverable=candidates >/dev/null
mkdir -p "$L/issues/$sid/library"
jq '.review_request' -e "$L/issues/$id/library/report.json" >/dev/null || die '写す review_request が無い'
cp "$L/issues/$id/library/report.json" "$L/issues/$sid/library/report.json"   # 形の合う review_request つきの報告
runs=$(grep -c '^run' "$tmp/hc/log"); posted=$(grep -c '^status' "$tmp/gh/posted")
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die 'candidates の課題の review_request でレビュアーを起動した'
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$sid" && HC_TERMINAL_ID=t7 kanban review "$sid" >/dev/null)
dispatcher
[ "$(grep -c '^status' "$tmp/gh/posted")" = "$posted" ] && [ ! -e "$L/issues/$sid/library/gate" ] || die 'candidates の課題に gate を回した'
jq -e --arg t "$sid" 'has($t) | not' "$mergerec" >/dev/null || die "candidates の課題に andon issue merge を打った: $(cat "$mergerec")"
[ "$(pending "$sid" review)" != null ] || [ "$(pending "$sid" report)" != null ] || die "candidates の課題の review の通知が無い: $(jq -c .pending "$state")"
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die 'candidates の課題でレビュアーを起動した'
ok 'deliverable: candidates の課題は review_request でレビュアーを起動せず、review に入っても gate を回さない（通知は届ける）'

# 5c. Chief of Staff が起き直してアドレスが変わり、deliverable の照合が外れても（書き直す前でも）、調べる課題は PR の課題として扱わない:
#     gate を回さず（担当に「report.json に pr が無い」の fix を送らない）、返ったことを Chief of Staff に届ける（PR が無いので通知は report）
sid=$(newtask '起き直しをまたぐ調べる課題' person)
HC_TERMINAL_ID=t5 kanban set "$sid" deliverable=candidates >/dev/null
kanban config set chief_of_staff=t6 >/dev/null
sends=$(nsent); posted=$(grep -c '^status' "$tmp/gh/posted")
(cd "$ANDON_WORKSPACE/.worktrees/app/demo-$sid" && HC_TERMINAL_ID=t7 kanban review "$sid" >/dev/null)
dispatcher
[ ! -e "$L/issues/$sid/library/gate" ] && [ "$(grep -c '^status' "$tmp/gh/posted")" = "$posted" ] || die '照合が外れた調べる課題に gate を回した'
jq -e --arg t "$sid" 'has($t) | not' "$mergerec" >/dev/null || die "照合が外れた調べる課題に andon issue merge を打った: $(cat "$mergerec")"
[ "$(nsent)" = "$sends" ] || die "照合が外れた調べる課題の担当に何か送った: $(tail -n 1 "$tmp/hc/sent")"
[ "$(pending "$sid" report)" != null ] || die "照合が外れた調べる課題が返ったことが Chief of Staff に届かない: $(jq -c .pending "$state")"
kanban config set chief_of_staff=t5 >/dev/null
ok 'deliverable: candidates の照合が外れていても（Chief of Staff の起き直しのあと）、gate を回さず返ったことを Chief of Staff に届ける'

# 6. andon issue finish（オーナーがマージしたあと Chief of Staff が打つ）はレビュアーの子課題と detached worktree も片付ける
revs=$(kanban list --all --json | jq -r --arg p "$id" '[.[] | select(.parent == $p and .status != "done")] | length')
[ "$revs" -ge 2 ] && ls -d "$ANDON_WORKSPACE/.worktrees/app/demo-$id-r"* >/dev/null || die 'レビュアーの子課題・detached worktree が無い（前提）'
HC_TERMINAL_ID=t5 "$andon" issue finish "$id" >/dev/null
kanban list --all --json | jq -e --arg p "$id" 'all(.[] | select(.id == $p or .parent == $p); .status == "done")' >/dev/null || die '子課題が done でない'
! ls -d "$ANDON_WORKSPACE/.worktrees/app/demo-$id"* >/dev/null 2>&1 || die 'worktree が残った'
git -C "$app" show-ref --verify --quiet "refs/heads/demo/$id" || die 'マージしていない branch を消した'
ok 'andon issue finish はレビュアーの子課題と detached worktree も片付け、origin に入っていない branch は残す'
dispatcher
grep -q "$id の PR はマージされていないので影響を見ない" "$tmp/dispatcher.log" || die 'Chief of Staff が打った finish（done）を拾って andon issue impact を打っていない'
ok 'Chief of Staff が打った finish（done）も拾い、PR が MERGED でなければ影響を知らせない'

# 6b. kanban の worktree が repo の本体の checkout（git worktree list の先頭）を指していたら、finish は何も消さずに止まる
id=$(HC_TERMINAL_ID=t5 kanban add '本体を指す' --profile claude --repo github.com/example/app)
HC_TERMINAL_ID=t5 kanban set "$id" "worktree=$app" branch=main >/dev/null
if out=$(HC_TERMINAL_ID=t5 "$andon" issue finish "$id" 2>&1); then die "本体の checkout を指す課題を finish が片付けた: $out"; fi
grep -q 'repo の本体の checkout' <<<"$out" || die "止まった理由が本体の checkout でない: $out"
[ -d "$app/.git" ] && [ "$(git -C "$app" worktree list --porcelain | sed -n 1p)" = "worktree $app" ] || die '本体の checkout が消えた'
kanban show "$id" --json | jq -e '.issue.status != "done"' >/dev/null || die '止まったのに done にした'
ok 'andon issue finish は kanban の worktree が repo の本体の checkout なら、消さずに止まる'

# 7. 最初のメッセージが届かなければ（送れなかった・送ったが動き出さなかった）、端末にラベル first_message.sent=false を付けて
#    起動したことにし、次の見回りで1回だけ送り直す（担当と同じ。dispatcher.md §5）。課題も端末も二つにしない
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
  jq -e --arg k "$key" '.reviewers[$k] | .started == true and .reviewer_issue != null and .address != null' "$state" >/dev/null || die "ラベルを付けたのに起動したことにならない（$first）: $(jq -c --arg k "$key" '.reviewers[$k]' "$state")"
  addr=$(jq -r --arg k "$key" '.reviewers[$k].address' "$state")
  rev=$(jq -r --arg k "$key" '.reviewers[$k].reviewer_issue' "$state")
  grep -qx "$addr first_message.sent=false" "$tmp/hc/labels" || die "届いていない端末にラベルを付けていない（$first）"
  ! grep -q "^$addr	" "$tmp/hc/sent" 2>/dev/null || die "起動した回に送り直した（$first）"
  # 次の見回り: レビュアーは idle のまま（入力欄が空）。同じ端末へレビュアーの最初のメッセージを1回だけ送り、ラベルを外す
  printf '{"address":"%s","state":"live","status":{"state":"idle"},"status_ms":%s,"turn":1,"labels":{"first_message.sent":"false"}}\n' "$addr" "$(date +%s%3N)" > "$tmp/hc/term-$addr.json"
  dispatcher
  [ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die "二重に起動した（$first）"
  [ "$(grep -c "^$addr	\[dispatcher\] 課題 \`demo/$rev\`。" "$tmp/hc/sent")" = 1 ] || die "同じ端末へレビュアーの最初のメッセージを送り直していない（$first）"
  grep -A2 "^$addr	\[dispatcher\] 課題" "$tmp/hc/sent" | grep -q "^先に \`.*/skills/worker/SKILL.md\` を読む。" || die "送り直した文がレビュアーの最初のメッセージでない（$first）"
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
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] && jq -e --arg k "$id@$head" '.reviewers[$k].failed | contains("fetch")' "$state" >/dev/null || die 'fetch に失敗したのに起動した・控えに失敗が無い'
dispatcher
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] && jq -e --arg k "$id@$head" '.reviewers[$k].started == true' "$state" >/dev/null || die 'fetch が戻ったのに起動しない'
! grep '^run ' "$tmp/hc/log" | tail -n 1 | grep -q 'herdr\.' || die "担当の端末のラベルが読めないのに herdr.space・herdr.tab を付けた: $(grep ^run "$tmp/hc/log" | tail -n 1)"
ok 'fetch の一時の失敗は控えで成功と分け、次の見回りで起動する（担当の端末のラベルが読めなければ herdr.space・herdr.tab を付けない）'

# 9. gate のあと Chief of Staff が merge: person に変えたら、マージせず次の見回りで gate から回し直す
gate_through() { # gate_through <id>: レビュアーが指摘なしで返し、担当が ready まで
    local rev; rev=$(kanban list --all --json | jq -r --arg p "$1" '[.[] | select(.parent == $p)] | last | .id')
    review_by "$rev" '[]'
    as_worker "$1" ready
}
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
echo "cd $PWD && HC_TERMINAL_ID=t5 kanban set $id merge=person >/dev/null" > "$tmp/gh/hook"
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'gate のあと merge: person に変えたのにマージした'
grep -q "$id をいまはマージしない.*予約キーが gate のあとに変わった" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die '回し直しでマージした'
[ "$(grep -c "^status .*$head" "$tmp/gh/posted")" = 2 ] || die 'gate を回し直していない'
ok 'gate のあと merge: person に変わったら gh pr merge を打たず、次の見回りで gate から回し直す（approve でも person なので止まる）'

# 9b. GitHub の checks が走っている間は待ち（マージしない・オーナーに回さない・控えに書かない）。通れば次の見回りでマージ
id=$(newtask 'CI を待つ' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
checks IN_PROGRESS
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
rest() { grep -cE '^api --paginate --slurp repos/[^ ]+/pulls/[0-9]+/files$|^pr diff |^api user|^api repos/[^/ ]+/[^/ ]+ ' "$tmp/gh/calls" || true; }   # gate が読む REST
: > "$tmp/gh/calls"
dispatcher
[ "$(rest)" = 2 ] || die "gate の1回で読む REST が files・diff の2回でない: $(cat "$tmp/gh/calls")"
: > "$tmp/gh/calls"
dispatcher
dispatcher
[ "$(rest)" = 0 ] && grep -q '^api graphql ' "$tmp/gh/calls" || die "checks が走っている間の見回りで REST を読んだ・checks を読んでいない: $(cat "$tmp/gh/calls")"
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'checks が走っているのにマージした'
[ "$(grep -c "$id の gate は GitHub の checks を待つ: GitHub の checks がまだ走っている: test" "$tmp/dispatcher.log")" = 1 ] &&
    [ "$(grep -c "$id は GitHub の checks を待つ（gate は回さない）" "$tmp/dispatcher.log")" = 2 ] || die 'wait のあと checks が走っている間も gate を回し直した'

[ ! -e "$L/issues/$id/library/questions" ] && [ ! -e "$L/issues/$id/library/gate" ] || die 'wait で問い・gate の記録を書いた'
checks COMPLETED SUCCESS
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$((merges + 1))" ] && kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null || die 'checks が通ったのにマージしない'
ok 'GitHub の checks が走っている間は gate が wait。merge の記録に決めたことを書かず問いも出さず、同じ head の checks が走っている間は gate を回さず（REST 0 回）、通ればマージする'

# 9c. checks が1つも無い（none）間も、review から github_checks_wait_minutes を過ぎるまでは gate を回さない（REST 0 回）。過ぎたら gate の needs_human
id=$(newtask 'CI がまだ付かない' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
echo '[]' > "$tmp/gh/rollup.json"
gate_through "$id"
: > "$tmp/gh/calls"
dispatcher
[ "$(rest)" = 2 ] && grep -q "$id の gate は GitHub の checks を待つ: GitHub の checks がまだ1つも無い" "$tmp/dispatcher.log" || die "none の wait で gate を回していない: $(cat "$tmp/gh/calls")"
: > "$tmp/gh/calls"
dispatcher
dispatcher
[ "$(rest)" = 0 ] && [ "$(grep -c "$id は GitHub の checks を待つ（gate は回さない）" "$tmp/dispatcher.log")" = 2 ] || die "checks が1つも無い間の見回りで REST を読んだ: $(cat "$tmp/gh/calls")"
cp "$L/log.jsonl" "$tmp/log.bak"   # review に入った時刻を昔にする（同じ長さの時刻なので読んだ位置はずれない）
jq -c --arg id "$id" 'if .id == $id and .change.status == "review" then .at = "2020-01-01T00:00:00Z" else . end' "$tmp/log.bak" > "$L/log.jsonl"
dispatcher
q=$L/issues/$id/library/questions/1.json
jq -e '.from == "gate" and (.facts | any(contains("1つも無いまま")))' "$q" >/dev/null || die "待ち時間を過ぎた none で gate の needs_human の問いが出ない: $(cat "$q" 2>/dev/null)"
checks COMPLETED SUCCESS
ok 'GitHub の checks が1つも無い間も、github_checks_wait_minutes を過ぎるまでは gate を回さず（REST 0 回）、過ぎたら gate が needs_human を返して問いになる'

# 9d. gate が gh の上限（rate limit）で落ちたら、問いにせず merge の記録に決めたことも書かず、gh api rate_limit の reset まで gate を回さない
id=$(newtask 'gh の上限' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
gate_through "$id"
reset=$(( $(date +%s) + 3600 ))
jq -n --argjson r "$reset" '{resources: {core: {limit: 5000, remaining: 4000, reset: ($r + 600)}, graphql: {limit: 5000, remaining: 0, reset: $r}}}' > "$tmp/gh/rate_limit.json"
printf 'GraphQL: API rate limit exceeded for user ID 1.\n' > "$tmp/gh/ghfail"
merges=$(wc -l < "$tmp/gh/merges")
dispatcher
jq -e --arg t "$id" --arg r "$(date -u -d "@$reset" +%Y-%m-%dT%H:%M:%SZ)" '.gh_reset == $r and
    ([.pending[] | select(.issue == $t and (.kind == "gate" or .kind == "asked"))] | length) == 0' "$state" >/dev/null &&
    jq -e --arg t "$id" '(.[$t].decision // "") == ""' "$mergerec" >/dev/null ||
    die "rate limit で控えに書いた・問いを出した・reset を graphql の値にしていない: $(jq -c '{gh_reset, pending: [.pending[] | select(.issue == "'"$id"'")]}' "$state")"
[ ! -e "$L/issues/$id/library/questions" ] && grep -q "$id の gate は gh の上限（rate limit）で落ちた" "$tmp/dispatcher.log" || die 'rate limit で問いを書いた・ログに無い'
rm "$tmp/gh/ghfail"
: > "$tmp/gh/calls"
dispatcher
[ ! -s "$tmp/gh/calls" ] || ! grep -qE '^pr view 7 --repo example/app --json state,isDraft|^api graphql ' "$tmp/gh/calls" || die "reset の前に gate・checks を読んだ: $(cat "$tmp/gh/calls")"
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] && kanban show "$id" --json | jq -e '.issue.status == "review"' >/dev/null || die 'reset の前にマージした'
cp "$state" "$tmp/state.bak"; jq '.gh_reset = "2020-01-01T00:00:00Z"' "$tmp/state.bak" > "$state"   # reset を過ぎた
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$((merges + 1))" ] && kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null || die 'reset を過ぎても gate を回し直さない'
jq -e '(.gh_wait // 0) == 0' "$state" >/dev/null || die "上限で落ちずに gate が返ったのに gh_wait が残る: $(jq -c .gh_wait "$state")"
ok 'gate が gh の上限で落ちたら、問いにも merge の記録の決めたことにもせず、gh api rate_limit の reset まで回さない。過ぎたら回し直してマージする'

# 9d2. secondary rate limit（rate_limit の残りがある。Retry-After は gh の出力に出ない）は1分から待ち、続けて落ちるたびに倍にする
id=$(newtask 'gh の secondary 上限' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
gate_through "$id"
jq -n '{resources: {core: {remaining: 4000, reset: 0}, graphql: {remaining: 4000, reset: 0}}}' > "$tmp/gh/rate_limit.json"
printf 'gh: You have exceeded a secondary rate limit. Please wait a few minutes before you try again. (HTTP 403)\n' > "$tmp/gh/ghfail"
waited() { # waited <分>: gh_wait がその分で、gh_reset が今からおよそその分後
    jq -e --argjson m "$1" --argjson now "$(date +%s)" '.gh_wait == $m and ((.gh_reset | fromdate) - $now - $m * 60 | fabs) < 30' "$state" >/dev/null ||
        die "secondary rate limit の待ちが $1 分でない: $(jq -c '{gh_wait, gh_reset}' "$state")"
}
dispatcher; waited 1
cp "$state" "$tmp/state.bak"; jq '.gh_reset = "2020-01-01T00:00:00Z"' "$tmp/state.bak" > "$state"
dispatcher; waited 2
cp "$state" "$tmp/state.bak"; jq '.gh_reset = "2020-01-01T00:00:00Z"' "$tmp/state.bak" > "$state"
dispatcher; waited 4
[ ! -e "$L/issues/$id/library/questions" ] || die 'secondary rate limit で問いを書いた'
rm "$tmp/gh/ghfail"
cp "$state" "$tmp/state.bak"; jq '.gh_reset = "2020-01-01T00:00:00Z"' "$tmp/state.bak" > "$state"
dispatcher
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null && jq -e '(.gh_wait // 0) == 0' "$state" >/dev/null || die 'secondary rate limit が明けてもマージしない・gh_wait が残る'
ok 'secondary rate limit（Retry-After は読めない）は1分から待ち、続けて上限で落ちるたびに2分・4分と延ばし、明ければ回し直して待ちを戻す'

# 9e. rate limit 以外で gate が落ちた（終了コード 3）なら、今どおり needs_human の問い
id=$(newtask 'gh が落ちる' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
gate_through "$id"
printf 'HTTP 502: Bad Gateway\n' > "$tmp/gh/ghfail"
dispatcher
rm "$tmp/gh/ghfail"
jq -e '.from == "gate" and (.question | contains("判定できなかった"))' "$L/issues/$id/library/questions/1.json" >/dev/null &&
    jq -e '(.gh_reset // "") == ""' "$state" >/dev/null || die "rate limit でない終了コード 3 が needs_human の問いにならない: $(cat "$L/issues/$id/library/questions/1.json" 2>/dev/null)"
# ADR-0056 A7: 印が無いとオーナーもマージできないので、先頭（推奨）は「原因を直して回し直す」
jq -e '.options[0].pick == true and (.options[0].label | contains("原因を直してから回し直す")) and (.options[1].label | contains("オーナーが PR を読んで"))' "$L/issues/$id/library/questions/1.json" >/dev/null ||
    die "gate が判定できない問いの先頭が原因を直して回し直すでない: $(jq -c .options "$L/issues/$id/library/questions/1.json")"
ok 'rate limit でない gh の失敗で gate が落ちたら、今どおり needs_human の問いにする（上限の控えは付けない。先頭は原因を直して回し直す）'

# 9e-2. ほかの PR のマージと重なって gh pr merge が「Base branch was modified」で失敗した → 問いにせず、次の巡で gate から照合し直して打ち直す
basemod='GraphQL: Base branch was modified. Review and try the merge again. (mergePullRequest)'
tried() { grep -c -- "--match-head-commit $head\$" "$tmp/gh/merges" || true; }   # 今の head（$head）に gh pr merge を打った回数
no_question() { [ ! -e "$L/issues/$id/library/questions" ]; }
base_moved() { # base_moved <タイトル>: gate の approve のあと、1回目の gh pr merge が base の動きで失敗した所まで。id と head を置く
    id=$(newtask "$1" auto)
    printf '%s\n' "$basemod" > "$tmp/gh/mergefail"
    to_gate "$id" docs/a.md
    [ "$(tried)" = 1 ] || die "前提: gh pr merge を1回打っていない（$1）: $(tried)"
    no_question || die "base が動いただけの失敗を問いにした（$1）: $(cat "$L/issues/$id/library/questions/1.json")"
    jq -e --arg t "$id" '.[$t] | (.decision // "") == "" and .retries == 1 and .approved != ""' "$mergerec" >/dev/null ||
        die "打ち直しの回数を記録していない・決めたことにした（$1）: $(cat "$mergerec")"
    grep -q "$id をいまはマージしない.*base が動いて gh pr merge が通らなかった" "$tmp/dispatcher.log" || die "打ち直す理由がログに無い（$1）"
}
drop() { # drop: 場面の課題を片付ける（あとの場面の数に入れない）
    rm -f "$tmp/gh/mergefail"
    checks COMPLETED SUCCESS
    HC_TERMINAL_ID=t5 kanban done "$id" --force >/dev/null
}

base_moved 'マージが重なる'
rm "$tmp/gh/mergefail"   # base が落ち着いた
dispatcher
[ "$(tried)" = 2 ] || die "同じ head で打ち直していない: $(tried)"
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null && no_question || die "打ち直したのに片付かない・問いにした: $(tail -3 "$tmp/dispatcher.log")"
jq -e '.merged == true' "$L/issues/$id/library/gate/$head.json" >/dev/null || die '打ち直したマージを gate の記録に書いていない'
ok 'base が動いて gh pr merge が失敗したら問いにせず、次の巡で gate から照合し直し、同じ head でマージを打ち直して片付ける'

# 打ち直しを待つ間に merge の記録が消えた → gate の記録の approve を決め終えたと引き継がず、gate から回し直して打ち直す
lost_merge_record() { # lost_merge_record: merge の記録を消して場面の課題に andon issue merge を打ち、ほかの課題の記録は戻す（あとの場面の数に入れない）
    # dispatcher の巡では、記録を作り直すのは最初に打つ課題なので、場面の課題に直に打つ（3b と同じ）
    cp "$mergerec" "$tmp/merge.bak"
    rm "$mergerec"
    freeze; HC_TERMINAL_ID=dispatcher "$andon" issue merge "$id" > "$tmp/merge.out"
    jq -s --arg t "$id" '(.[0] | del(.[$t])) + .[1]' "$tmp/merge.bak" "$mergerec" > "$tmp/merge.new" && mv "$tmp/merge.new" "$mergerec"
}
base_moved '待つ間に merge の記録が消える'
rm "$tmp/gh/mergefail"
lost_merge_record
[ "$(tried)" = 2 ] || die "記録が消えたら approve を引き継いで止まり、打ち直さない: $(tried) $(cat "$tmp/merge.out")"
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null && no_question || die "記録が消えたあと打ち直して片付けない・問いにした: $(cat "$tmp/merge.out")"
ok '打ち直しを待つ間に merge の記録が消えても、gate の approve を決め終えたと読まず、gate から回し直して打ち直す'

# ADR-0056 A7（決めたこと 12）: 印を書いた直後で必須の check がまだ揃わず「the base branch policy prohibits the merge」で断られた → 同じく打ち直す
id=$(newtask '必須の check が揃う前' auto)
printf 'X Pull request example/app#7 is not mergeable: the base branch policy prohibits the merge.\n' > "$tmp/gh/mergefail"
to_gate "$id" docs/a.md
no_question && jq -e --arg t "$id" '.[$t].retries == 1' "$mergerec" >/dev/null && grep -q "$id をいまはマージしない.*必須の check がまだ揃わず gh pr merge が通らなかった" "$tmp/dispatcher.log" ||
    die "必須の check で断られたマージを打ち直しにしない: $(cat "$mergerec") $(grep "$id" "$tmp/dispatcher.log" | tail -2)"
rm "$tmp/gh/mergefail"
dispatcher
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null && no_question && [ "$(tried)" = 2 ] || die "必須の check が揃ったあと打ち直して片付けない: $(tail -3 "$tmp/dispatcher.log")"
ok '必須の check で断られた gh pr merge（the base branch policy prohibits the merge）も問いにせず、次の巡で gate から照合し直して打ち直す（ADR-0056 A7）'

base_moved '打ち直す前に push される'
jq --arg h "$(printf 'e%.0s' $(seq 40))" '.headRefOid = $h' "$tmp/gh/view.json" > "$tmp/gh/v" && mv "$tmp/gh/v" "$tmp/gh/view.json"
merges=$(wc -l < "$tmp/gh/merges")
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'head が動いたのに打ち直した'
grep -q "^t7	\[gate\] $id（.*report.json の commit が PR の head と違う" "$tmp/hc/sent" || die '動いた head を担当へ返していない'
drop

base_moved '打ち直す前に衝突する'
jq '.mergeStateStatus = "DIRTY"' "$tmp/gh/view.json" > "$tmp/gh/v" && mv "$tmp/gh/v" "$tmp/gh/view.json"
dispatcher
[ "$(tried)" = 1 ] || die '衝突したのに打ち直した'
jq -e '.from == "gate" and (.facts | index("conflict"))' "$L/issues/$id/library/questions/1.json" >/dev/null || die '衝突が gate の問いになっていない'
drop

base_moved '打ち直す前に判定が変わる'
checks COMPLETED FAILURE
dispatcher
[ "$(tried)" = 1 ] || die 'gate の判定が fix に変わったのに打ち直した'
jq -e --arg t "$id" '.[$t] | .decision == "fix" and (.retries // 0) == 0' "$mergerec" >/dev/null || die "変わった判定を記録していない・回数を残した: $(cat "$mergerec")"
drop

base_moved '打ち直す前に委譲の外になる'
HC_TERMINAL_ID=t5 kanban set "$id" merge=person >/dev/null
dispatcher
[ "$(tried)" = 1 ] || die 'merge: person に変わったのに打ち直した'
drop
ok 'base が動いたあと head が動く・衝突する・gate の判定が変わる・merge: person になると、マージを打ち直さない'

# 打ち直しを待つ間にオーナーが同じ head でマージした → gate を回さず（fix を送らず）、settle が片付ける
base_moved '待つ間にオーナーがマージする'
rm "$tmp/gh/mergefail"
git -C "$tmp/origin" update-ref refs/heads/main "$head"; echo "$head" > "$tmp/gh/merged"
jq '.state = "MERGED"' "$tmp/gh/view.json" > "$tmp/gh/v" && mv "$tmp/gh/v" "$tmp/gh/view.json"
watch_due "$id"
dispatcher
[ "$(tried)" = 1 ] || die 'オーナーがマージしたのに打ち直した'
! grep -q "^t7	\[gate\] $id（" "$tmp/hc/sent" || die "オーナーがマージした課題の担当へ fix を送った: $(grep "\[gate\] $id（" "$tmp/hc/sent")"
jq -e '.decision == "approve" and (.merged // false) == false' "$L/issues/$id/library/gate/$head.json" >/dev/null || die 'オーナーのマージで gate の記録を書き換えた'
kanban show "$id" --json | jq -e '.issue.status == "done"' >/dev/null && no_question || die '待つ間のオーナーのマージを片付けない・問いにした'
ok '打ち直しを待つ間にオーナーが同じ head でマージしたら、fix も問いも出さずオーナーのマージとして片付ける'

base_moved '待つ間に gate が落ちる'
printf 'HTTP 502: Bad Gateway\n' > "$tmp/gh/ghfail"
dispatcher
rm "$tmp/gh/ghfail"
jq -e '.question | contains("判定できなかった")' "$L/issues/$id/library/questions/1.json" >/dev/null || die '落ちた gate が問いにならない'
jq -e --arg t "$id" '.[$t] | (.approved // "") == "" and (.retries // 0) == 0' "$mergerec" >/dev/null || die "問いにしたのに approve の待ちを残した（10 分で止まったと届く）: $(cat "$mergerec")"
drop
ok '打ち直しを待つ間に gate が落ちて問いにしたら、approve の待ちと回数を残さない'

base_moved '重なり続ける'
checks IN_PROGRESS   # checks を待つ巡は打たず、回数も忘れない
dispatcher
[ "$(tried)" = 1 ] || die 'checks を待つ間に打ち直した'
checks COMPLETED SUCCESS
dispatcher; dispatcher
[ "$(tried)" = 3 ] && no_question || die "上限の内で問いにした・打ち直していない: $(tried)"
dispatcher
[ "$(tried)" = 4 ] || die "上限（打ち直し 3 回）まで打っていない: $(tried)"
jq -e '.from == "gate" and .kind == "irreversible" and (.question | contains("dispatcher がマージできなかった")) and (.facts[0] | contains("Base branch was modified"))' \
    "$L/issues/$id/library/questions/1.json" >/dev/null || die "上限を超えた失敗がオーナーへの問いにならない: $(cat "$L/issues/$id/library/questions/1.json" 2>/dev/null)"
dispatcher
[ "$(tried)" = 4 ] || die '上限を超えても打ち直した'
jq -e --arg t "$id" '.[$t] | .decision == "approve" and (.retries // 0) == 0 and (.approved // "") == ""' "$mergerec" >/dev/null || die "決めたあとに待ちを残した: $(cat "$mergerec")"
lost_merge_record   # 問いを出したあとに記録が消えても、gate の記録を引き継いで打ち直さず、問いを出し直さない
[ "$(tried)" = 4 ] && [ ! -e "$L/issues/$id/library/questions/2.json" ] || die "問いを出した approve の記録が消えたら、打ち直した・問いを出し直した: $(tried) $(cat "$tmp/merge.out")"
drop
ok 'base が動き続けたら打ち直しは 3 回までで、checks を待っても回数を忘れず、超えたら今までどおりオーナーへの問いにする'

# 似た文の失敗は今までどおり1回で問いにする
for why in 'GraphQL: Pull Request is not mergeable (mergePullRequest)' \
    'GraphQL: Head branch was modified. Review and try the merge again. (mergePullRequest)' 'HTTP 502: Bad Gateway'; do
    id=$(newtask 'ほかの失敗' auto)
    printf '%s\n' "$why" > "$tmp/gh/mergefail"
    to_gate "$id" docs/a.md
    dispatcher
    [ "$(tried)" = 1 ] || die "ほかの失敗で打ち直した: $why"
    jq -e --arg w "$why" '.from == "gate" and (.facts[0] | contains($w))' "$L/issues/$id/library/questions/1.json" >/dev/null || die "ほかの失敗が問いにならない: $why"
    drop
done
ok 'base の動きでない gh pr merge の失敗（衝突・head の動き・502）は、打ち直さず今までどおり問いにする'

# 9f.gate のあと checks が変わった・push されたら、マージの直前に読み直して打たない
# 遅い機械を写す: 前の場面までの課題の見張りの時刻を、実際の2分前にする（負荷が高いと、9e から 9f までに1分を超え、dispatcher はこう読む）。
# 時計が止まっていなければ、approve・needs_human のまま残った前の場面の課題の PR を聞き直し、head が動いたので gate を回し直す。
# その gate の書き込みが 9f の hook を先に使うと、9f の gate は checks の走り直しを見ずに終わる（揺れの元）
ago=$(date -u -d '-2 min' +%Y-%m-%dT%H:%M:%SZ)
jq --arg a "$ago" 'map_values(if .polled then .polled = $a else . end)' "$mergerec" > "$mergerec.new" && mv "$mergerec.new" "$mergerec"
jq --arg a "$ago" 'map_values(if .at then .at = $a else . end)' "$settled" > "$settled.new" && mv "$settled.new" "$settled"
id=$(newtask 'CI が走り直す' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
n=$(polls) s=$(wc -l < "$tmp/gh/states")
dispatcher
[ "$(polls)" = "$n" ] && [ "$(wc -l < "$tmp/gh/states")" = $((s + 1)) ] ||
    die "前の場面の課題の PR を、実際に1分過ぎたので聞き直した（見張りの時計が止まっていない）: head $(( $(polls) - n )) 回・状態 $(( $(wc -l < "$tmp/gh/states") - s )) 回（9f の課題の初めの1回だけのはず）"
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
checks IN_PROGRESS; mv "$tmp/gh/rollup.json" "$tmp/gh/rollup-pending.json"; checks COMPLETED SUCCESS
echo "mv $tmp/gh/rollup-pending.json $tmp/gh/rollup.json" > "$tmp/gh/hook"  # gate が approve を書いたあとに走り直す
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'gate のあと checks が走り直したのにマージした'
grep -q "$id をいまはマージしない.*GitHub の checks が gate のあとに変わった（pending: test）" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
checks COMPLETED SUCCESS
old=$head
echo "jq --arg h $(printf 'f%.0s' $(seq 40)) '.headRefOid = \$h' $tmp/gh/view.json > $tmp/gh/v && mv $tmp/gh/v $tmp/gh/view.json" > "$tmp/gh/hook"
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'gate のあと push されたのにマージした'
grep -q "$id をいまはマージしない.*PR の head が gate のあとに動いた（${old:0:12} → ffffffffffff）" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
dispatcher
grep -q "^t7	\[gate\] $id（CI が走り直す）: gate が fix を返した。理由: .*report.json の commit が PR の head と違う" "$tmp/hc/sent" || die "回し直した gate が fix を担当へ返していない: $(tail -n 2 "$tmp/hc/sent" 2>/dev/null)"
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die '回し直しでマージした'
ok 'マージの直前に PR の head と GitHub の checks を読み直し、checks が変わった・head が動いたら打たない（回し直した gate が担当へ fix）'

# 10. gate のあとオーナーが委譲の merge を取り消したら、マージしない（回し直した gate は needs_human）
id=$(newtask '委譲が取り消される' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
dispatcher
gate_through "$id"
echo "cd $tmp/plant/projects/demo && env -u HC_TERMINAL_ID GIT_AUTHOR_NAME=o GIT_AUTHOR_EMAIL=owner@example.com $andon delegate narrow merge none --evidence x >$tmp/hook.out 2>&1" > "$tmp/gh/hook"
dispatcher
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'gate のあと委譲を取り消したのにマージした'
grep -q "$id をいまはマージしない.*委譲の merge が gate のあとに変わった（B → none）" "$tmp/dispatcher.log" || die '止めた理由がログに無い'
dispatcher
jq -e '.from == "gate" and (.facts | any(contains("delegation: merge=none")))' "$L/issues/$id/library/questions/1.json" >/dev/null || die '回し直した gate の needs_human が問いになっていない'
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'マージした'
ok 'gate のあと委譲の merge が取り消されたら gh pr merge を打たず、回し直した gate の needs_human を問いにする'

# === フックで届ける・止める（P6b。dispatcher.md §3・§4、contract.md §9）===
# フックは package/hooks/ を Claude Code と同じに打つ（stdin にフックの入力の JSON、HC_TERMINAL_ID は自分の端末）
ln -sf "$andon" "$tmp/bin/andon"
hooks=$andon_root/package/hooks
term() { # term <addr> <状態> <turn> [<k=v>…]: 偽の hc get --json
    local a=$1 s=$2 n=$3; shift 3
    printf '%s\n' "$@" | jq -R 'select(. != "") | split("=") | {(.[0]): .[1]}' | jq -s --arg a "$a" --arg s "$s" --argjson n "$n" \
        '{address: $a, state: "live", status: {state: $s}, turn: $n, labels: (add // {})}' > "$tmp/hc/term-$a.json"
}
cos() { term t5 "$1" "$2" role=chief-of-staff project=demo; }
event() { # event <タイトル>: dispatcher が通知 report を積む出来事（担当が報告を書く。届ける kind）。id を返す
    local id; id=$(newtask "$1" person)
    work "$id" docs/a.md
    as_worker "$id" report --pr https://github.com/example/app/pull/7
    dispatcher
    pending "$id" report | jq -e . >/dev/null || die "$1: 通知が積まれていない"
    echo "$id"
}
sent() { cat "$tmp/hc/sent" 2>/dev/null | grep -c '^t5	'; }

# 11. turn 中の通知が Stop で届く（dispatcher は Chief of Staff に hc send しない）
cos working 3
id=$(event 'ターンの中の通知')
[ "$(sent)" = 0 ] || die 'working の Chief of Staff に hc send した'
out=$(HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"見た"}')
jq -e --arg id "$id" '(has("decision") | not) and .hookSpecificOutput.hookEventName == "Stop" and (.hookSpecificOutput.additionalContext | startswith("<news project=\"demo\">\n") and contains("\n\($id) report: 報告 ") and endswith("オーナーに言うことが無ければ `andon news ack` を打ち、文を書かずにターンを終える。\n</news>"))' <<<"$out" >/dev/null ||
    die "Stop が <news> を additionalContext で渡さない: $out"
jq -e '.pending == [] and .hook.terminal == "t5" and .hook.via == "stop"' "$state" >/dev/null || die '取り出した待ちが控えに残った・渡した turn を控えていない'
: >> "$tmp/hc/labels"
out=$(HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"stop_hook_active":true,"last_assistant_message":"<no-reply>"}')
[ -z "$out" ] || die "待ちが無いのに block した: $out"
! grep -qx 't5 news.replied=3' "$tmp/hc/labels" && jq -e 'has("hook") | not' "$state" >/dev/null || die '<no-reply> だけの答え（ack なし）に news.replied を書いた・確かめ済みにしない'
out=$(HC_TERMINAL_ID=t7 sh "$hooks/stop" <<<'{}')
[ -z "$out" ] || die "Chief of Staff でも担当でもない端末で何か出した: $out"
ok 'turn 中の通知は Chief of Staff の Stop フック（drain）が additionalContext で渡し（末尾は andon news ack の1行）、答えが <no-reply> だけでは news.replied を書かない'

# 12. 送信時に添えられる（UserPromptSubmit。委譲の一覧も同じ口で）
id=$(event '送信時の通知')
out=$(HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"hook_event_name":"UserPromptSubmit","prompt":"次は？"}')
jq -e --arg id "$id" '.hookSpecificOutput.hookEventName == "UserPromptSubmit" and (.hookSpecificOutput.additionalContext
    | contains("\($id) report:") and contains("<delegation project=\"demo\">\n{\"project\":\"demo\""))' <<<"$out" >/dev/null ||
    die "additionalContext に待ちと委譲が無い: $out"
jq -e '.pending == []' "$state" >/dev/null || die '添えた待ちが残った'
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"last_assistant_message":"了解"}' >/dev/null
! grep -q 'news.replied=3$' "$tmp/hc/labels" || die 'オーナーへの答えに news.replied を書いた'
out=$(HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"prompt":"もう一つ"}')
jq -e '.hookSpecificOutput.additionalContext | (contains("<news") | not) and contains("<delegation")' <<<"$out" >/dev/null || die "待ちが無いときに委譲だけを添えない: $out"
out=$(HC_TERMINAL_ID=t5 PATH=/usr/bin:/bin sh "$hooks/user-prompt-submit" <<<'{}'); rc=$?
[ "$rc" = 0 ] && [ -z "$out" ] || die 'andon が無いときに通さない'
ok 'オーナーが送ったとき、待ちと委譲の一覧を UserPromptSubmit の additionalContext に添える（andon が無ければ通す）'

# 12a. core が起き直して dispatcher の端末の中身がシェルに替わっていたら、Chief of Staff のフックが片付けて起動し直す（シェルには打ち込まない）
sw=$(event 'シェルに替わった dispatcher の場面')   # 担当の Stop フックの場面が使う、報告済みの doing の課題。積んだ通知は Chief of Staff の Stop で渡しきる
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"stop_hook_active":false,"last_assistant_message":"y"}' >/dev/null
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"stop_hook_active":true,"last_assistant_message":"y"}' >/dev/null
echo '{"address":"t3","state":"live","argv":["/bin/sh"],"foreground":{"name":"sh","argv":["/bin/sh"]}}' > "$tmp/hc/term-dispatcher.json"
logs=$(wc -l < "$tmp/hc/log")
HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"prompt":"x"}' >/dev/null
got=$(tail -n +"$((logs + 1))" "$tmp/hc/log" | grep -v '^get \|^ls ')
want="keep t3 --off
kill t3
rm t3
run --bg --no-parent --no-notify --keep --name dispatcher --label role=dispatcher --label herdr.space=andon-demo --label herdr.tab=andon-demo-dispatcher --cwd $tmp/plant -- $andon dispatcher"
[ "$got" = "$want" ] || die "シェルに替わった dispatcher を片付けて起動し直さない: $got"
! grep -q '^dispatcher	' "$tmp/hc/sent" || die 'dispatcher のシェルに打ち込んだ'
fg_dispatcher
logs=$(wc -l < "$tmp/hc/log")
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"last_assistant_message":"x"}' >/dev/null
! tail -n +"$((logs + 1))" "$tmp/hc/log" | grep -q '^run \|^kill \|^rm ' || die '中身が andon dispatcher の dispatcher を起動し直した'
# 担当の Stop フック（stop-check）も同じ確かめを打つ（Chief of Staff の留守にも戻す。ADR-0017 決めたこと 8 の (e)）
sed -i '/^t5 /d' "$tmp/hc/live"
echo '{"address":"t3","state":"live","argv":["/bin/sh"],"foreground":{"name":"sh","argv":["/bin/sh"]}}' > "$tmp/hc/term-dispatcher.json"
echo '{"address":"t7","state":"live","labels":{"issue":"'"$sw"'"}}' > "$tmp/hc/term-t7.json"
logs=$(wc -l < "$tmp/hc/log")
HC_TERMINAL_ID=t7 sh "$hooks/stop" <<<'{}' >/dev/null
got=$(tail -n +"$((logs + 1))" "$tmp/hc/log" | grep -v '^get \|^ls ')
[ "$got" = "$want" ] || die "担当の Stop フックがシェルに替わった dispatcher を起動し直さない: $got"
rm "$tmp/hc/term-t7.json"; fg_dispatcher
echo "t5 chief-of-staff" >> "$tmp/hc/live"
ok 'dispatcher の端末の中身がシェルに替わっていたら、Chief of Staff のフック（drain）と担当の Stop フック（stop-check）が片付けて同じ名前・ラベルで起動し直す（シェルに打ち込まない・動いていれば触らない）'

# 12b. UserPromptSubmit で渡したあと turn が中断された（Stop が来ない）ら、次の送信で1回だけ届け直す。news.replied は渡した turn の答えにだけ
ups() { HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<"{\"session_id\":\"s1\",\"prompt_id\":\"$1\",\"prompt\":\"x\"}" | jq -r '.hookSpecificOutput.additionalContext'; }
cstop() { HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<"{\"session_id\":\"s1\",\"prompt_id\":\"$1\",\"stop_hook_active\":${3:-false},\"last_assistant_message\":\"$2\"}"; }
id=$(event '中断される送信')
labels=$(wc -l < "$tmp/hc/labels")
ups p1 | grep -q "$id report:" || die '1回目の送信で渡さない'
jq -e '.hook.via == "prompt" and .hook.prompt_id == "p1" and (.hook.news | any(.kind == "report"))' "$state" >/dev/null || die '渡した <news> を turn の終わりまで控えに残さない'
# p1 は中断（Stop が来ない）。次の送信で届け直す
ups p2 | grep -q "$id report:" || die '中断された turn の <news> を次の送信で届け直さない'
jq -e '(.hook.news // []) == [] and (.hook.again | any(.kind == "report"))' "$state" >/dev/null || die '届け直した分を控えていない'
# p2 も中断。届け直しは1回だけ
ups p3 | grep -q "$id report:" && die '届け直しを2回した'
cstop p3 '<no-reply>' >/dev/null
[ "$(wc -l < "$tmp/hc/labels")" = "$labels" ] || die '<news> を渡していない turn の <no-reply> に news.replied を書いた'
# 渡した turn と違う turn の Stop では書かない・渡した turn の Stop では書く
id=$(event '照合する送信')
ups p4 >/dev/null
cos working 6
out=$(cstop p0 'x')
[ "$(wc -l < "$tmp/hc/labels")" = "$labels" ] || die '別の turn（prompt_id 違い）の Stop で news.replied を書いた'
jq -e --arg id "$id" '(has("decision") | not) and (.hookSpecificOutput.additionalContext | contains("\($id) report:"))' <<<"$out" >/dev/null || die "確かめていない <news> をその Stop で届け直さない: $out"
term t5 working 6 role=chief-of-staff project=demo news.ack=6
r=$(grep -c 'news.replied' "$tmp/hc/labels" || true)
cstop p0 'x' true >/dev/null
grep -qx 't5 news.replied=6' "$tmp/hc/labels" && jq -e 'has("hook") | not' "$state" >/dev/null || die '届け直した turn の ack に news.replied を書かない'
cstop p0 'x' true >/dev/null
[ "$(grep -c 'news.replied' "$tmp/hc/labels")" = $((r + 1)) ] || die '確かめ済みの <news> に二度 news.replied を書いた'
ok '送信で渡した <news> は turn の Stop まで控えに残し、中断されたら次の送信で1回だけ届け直す。news.replied は prompt_id で照合した turn だけ（違う turn の Stop では書かず届け直す）'

# 12c. andon news ack（ADR-0010 決めたこと 4〜6）: Chief of Staff の端末だけに news.ack=<turn> を書き、
# 渡した turn に ack があれば答えの文に関わらず news.replied を書く（<no-reply> だけの答えでは書かない。ADR-0010 決めたこと 6・受け入れ条件 6）
replied() { grep -c 'news.replied' "$tmp/hc/labels"; }
cos working 7
out=$(HC_TERMINAL_ID=t5 "$andon" news ack) && tail -1 "$tmp/hc/labels" | grep -qx 't5 news.ack=7' || die "Chief of Staff の端末に news.ack=<turn> を書かない: $out"
term t7 working 1 project=demo issue=A-0001
labels=$(wc -l < "$tmp/hc/labels")
HC_TERMINAL_ID=t7 "$andon" news ack >/dev/null 2>&1 && die 'Chief of Staff でない端末の ack が 0 で終わった'
[ "$(wc -l < "$tmp/hc/labels")" = "$labels" ] || die 'Chief of Staff でない端末に news.ack を書いた'
# 渡し中が無い（控えに hook が無い）ときの ack は news.replied を書かせない
n=$(replied) a=$(jq '.replied.ack // 0' "$state")
term t5 working 7 role=chief-of-staff project=demo news.ack=7
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"stop_hook_active":false,"last_assistant_message":"了解"}' >/dev/null
[ "$(replied)" = "$n" ] || die '渡し中が無いのに ack で news.replied を書いた'
# Stop で渡し（turn 8）、続きのターンで ack して人向けの文も返した: 文に関わらず書く。2回 ack しても1回だけ
id=$(event 'ack する通知')
term t5 working 8 role=chief-of-staff project=demo news.ack=7
cstop p8 '見た' | jq -e --arg id "$id" '.hookSpecificOutput.additionalContext | contains("\($id) report:")' >/dev/null || die 'Stop で <news> を渡さない'
jq -e '.hook.turn == 8 and .hook.ack_before == "7"' "$state" >/dev/null || die '渡したときの turn と news.ack を控えていない'
HC_TERMINAL_ID=t5 "$andon" news ack >/dev/null && HC_TERMINAL_ID=t5 "$andon" news ack >/dev/null || die 'ack が通らない'
term t5 working 8 role=chief-of-staff project=demo news.ack=8
cstop p8 '片付けた。オーナーへ言うことは無い' true >/dev/null
cstop p8 '片付けた。オーナーへ言うことは無い' true >/dev/null
[ "$(replied)" = $((n + 1)) ] && grep -qx 't5 news.replied=8' "$tmp/hc/labels" || die 'ack したターンに news.replied を1回だけ書かない'
# 確かめ済みの news.ack は消す（同じ turn の続きで次の <news> を渡したとき、2回目の ack も数えるため。A-0305）
tail -1 "$tmp/hc/labels" | grep -qx 't5 news.ack=' || die '確かめ済みの news.ack を消さない'
jq -e --argjson a "$a" '.replied.ack == $a + 1' "$state" >/dev/null || die 'ack で書いたことを控えに数えない'
# ack の無いターン（前のターンの値のまま）は書かない
id=$(event 'ack しない通知')
term t5 working 9 role=chief-of-staff project=demo news.ack=8
cstop p9 'x' >/dev/null
cstop p9 'オーナーへの答え' true >/dev/null
[ "$(replied)" = $((n + 1)) ] || die 'ack の無いターンに news.replied を書いた'
# <no-reply> の受け口は外した: ack が無ければ、答えが <no-reply> だけでも書かない・数えない
id=$(event '<no-reply> で答える通知')
cstop p9 'x' >/dev/null
cstop p9 '<no-reply>' true >/dev/null
[ "$(replied)" = $((n + 1)) ] && jq -e --argjson a "$a" '(.replied | has("no-reply") | not) and .replied.ack == $a + 1 and (has("hook") | not)' "$state" >/dev/null || die 'ack の無い <no-reply> だけの答えに news.replied を書いた・数えた'
# 同じ turn（Stop の block の続きは turn が進まない）で、渡す前にもう ack していたら、その ack は数えない
id=$(event '前の ack が残る通知')
term t5 working 9 role=chief-of-staff project=demo news.ack=9
cstop p9 'x' >/dev/null
cstop p9 'オーナーへの答え' true >/dev/null
[ "$(replied)" = $((n + 1)) ] || die '渡す前の同じ turn の ack で news.replied を書いた'
ok 'andon news ack は Chief of Staff の端末だけに news.ack=<turn> を書き、渡した turn に ack があれば文に関わらず news.replied を1回だけ書く（渡し中が無い・前の ack は数えない。<no-reply> だけの答えでは書かない）'

# 待ち役の道具（13a が使う）
waitf=$tmp/state/andon/dispatcher/demo.json.wait
wstart() { # wstart <名前> [<andon dispatcher wait の引数>…]: 待ち役を背景で起動する（引数が無ければフックの stop-wait そのもの）。終了コードは $tmp/<名前>.rc
    local n=$1; shift
    rm -f "$tmp/$n.rc" "$tmp/$n.err"
    if [ $# = 0 ]; then set -- sh "$hooks/stop-wait"; else set -- "$andon" dispatcher wait "$@"; fi
    (rc=0; HC_TERMINAL_ID=t5 "$@" </dev/null 2>"$tmp/$n.err" || rc=$?; echo $rc > "$tmp/$n.rc~"; mv "$tmp/$n.rc~" "$tmp/$n.rc") &
    local was; was=$(jq -r .token "$waitf" 2>/dev/null || true)
    for _ in $(seq 50); do   # 起きて届けを書いた（か、もう終わった）まで待つ
        [ -e "$tmp/$n.rc" ] && break
        t=$(jq -r .token "$waitf" 2>/dev/null || true); [ -n "$t" ] && [ "$t" != "$was" ] && break
        sleep 0.1
    done
}
wend() { # wend <名前> <秒>: 終わるまで待って終了コードを出す（終わらなければ空）
    for _ in $(seq $(( $2 * 10 ))); do [ -e "$tmp/$1.rc" ] && { cat "$tmp/$1.rc"; return; }; sleep 0.1; done
}
# 13a. 待ちが無い間・working の間は終わらない。次の待ち役が起きたら前のは 0 で終わる。止められたら 0。担当の端末では待たずに 0
cos done 6
wstart w2
sleep 1.5
[ -z "$(wend w2 0)" ] || die "待ちが無いのに待ち役が終わった: $(cat "$tmp/w2.rc")"
cos working 6
id=$(event '働いている間の通知')
sleep 1.5
[ -z "$(wend w2 0)" ] || die 'Chief of Staff が working の間に待ち役が終わった'
logs=$(wc -l < "$tmp/dispatcher.log")
dispatcher
! tail -n +"$((logs + 1))" "$tmp/dispatcher.log" | grep -q '待ち役がいない' || die '待ち役が生きているのに、いないとログに書いた'
wstart w3
[ "$(wend w2 5)" = 0 ] && [ ! -s "$tmp/w2.err" ] || die "次の待ち役が起きても、前の待ち役が静かに 0 で終わらない: $(cat "$tmp/w2.rc" "$tmp/w2.err" 2>/dev/null)"
[ -z "$(wend w3 1)" ] || die '新しい待ち役が残らない'
kill -TERM "$(jq -r .pid "$waitf")"
[ "$(wend w3 5)" = 0 ] && [ ! -e "$waitf" ] || die "止められた待ち役が 0 で終わらない・届けを残した: $(cat "$tmp/w3.rc" 2>/dev/null)"
term t7 idle 1 project=demo issue=A-0001
rc=0; HC_TERMINAL_ID=t7 timeout 5 sh "$hooks/stop-wait" </dev/null || rc=$?
[ "$rc" = 0 ] || die "担当の端末で待ち役が待った・0 で通さない: $rc"
rc=0; HC_TERMINAL_ID=t5 PATH=/usr/bin:/bin sh "$hooks/stop-wait" </dev/null || rc=$?
[ "$rc" = 0 ] || die 'andon が無いときに通さない'
ok '待ち役は待ちが無い間・working の間は終わらず、次の待ち役が起きたら前のは静かに 0、止められたら 0。Chief of Staff でない端末・andon が無ければ待たずに 0'

# 13c. プロバイダに切られた Chief of Staff を起動する StopFailure のフック（andon dispatcher rewake。ADR-0019 決めたこと 10）。
# 待って合図する筋と、待ち役と同時に合図しても <news> が1回だけなのは Go のテスト（rewake_test.go。待ちが分の単位）。ここはすぐ 0 で通す筋と、Stop で数え直す筋
rwf=$tmp/state/andon/dispatcher/demo.json.rewake
rw() { # rw <端末> <入力>: フックそのものを打って終了コードを出す（5 秒で切る。待ったら 124）
    local rc=0; HC_TERMINAL_ID=$1 timeout 5 sh "$hooks/stop-failure" <<<"$2" 2>"$tmp/rw.err" || rc=$?; echo "$rc"
}
cos error 8
for in in '{"error":"authentication_failed"}' '{"error":"billing_error"}' '{}'; do
    [ "$(rw t5 "$in")" = 0 ] && [ ! -s "$tmp/rw.err" ] || die "ほかの error で起動した・待った: $in"
done
term t7 error 1 project=demo issue=A-0001
[ "$(rw t7 '{"error":"server_error"}')" = 0 ] || die '担当の端末で起動した・待った'
[ ! -e "$rwf" ] || die '起動しない入力で回数を数えた'
rc=0; HC_TERMINAL_ID=t5 PATH=/usr/bin:/bin sh "$hooks/stop-failure" <<<'{"error":"server_error"}' || rc=$?
[ "$rc" = 0 ] || die 'andon が無いときに通さない'
echo '{"count":4}' > "$rwf"
[ "$(rw t5 '{"error":"rate_limit"}')" = 0 ] && [ ! -s "$tmp/rw.err" ] || die '5 回目に起動した・待った'
grep -q 't5 はプロバイダに切られて 5 回目（rate_limit）。起動しない' "$rwf.log" || die "5 回目のログが無い: $(cat "$rwf.log" 2>/dev/null)"
cos done 8
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"stop_hook_active":false}' >/dev/null
[ ! -e "$rwf" ] || die 'Chief of Staff の Stop（ターンが通った）で回数を数え直さない'
ok 'StopFailure のフックは、ほかの error・担当の端末・andon が無いときは待たずに 0、5 回目は 0 とログ1行。Chief of Staff の Stop で回数を数え直す'

# 14. doing で報告無しの Stop が block（担当・レビュアーの Stop フック。stop-check）
id=$(newtask '止まる前に報告する' person)
wt=$ANDON_WORKSPACE/.worktrees/app/demo-$id
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
# 別の kanban pick（Chief of Staff t6。working なので待ち役の見張りはログも書かない）で、拾う元ごとに正負を見る
mkdir -p "$tmp/plant/projects/pick"
cd "$tmp/plant/projects/pick"
env -u KANBAN_ROOT kanban init --prefix P >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t6 >/dev/null
echo "t6 chief-of-staff" >> "$tmp/hc/live"
echo '{"address":"t6","state":"live","status":{"state":"working"},"labels":{}}' > "$tmp/hc/term-t6.json"
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

# 15. 初めて見る kanban は ready と log.jsonl の位置を控えるだけ
p1=$(as_cos kanban add '前からある課題')
as_w kanban report "$p1" <<<'## Report' >/dev/null
pk
jq -e --arg p "$p1" '.pending == [] and .ready == [$p] and .log_pos == ('"$(stat -c %s "$P/log.jsonl")"')' "$pst" >/dev/null || die "初めて見る kanban で積んだ・控えていない: $(jq -c '{pending, ready, log_pos}' "$pst")"
ok '初めて見る kanban は、今の ready と log.jsonl の位置を控えるだけで積まない'

# 16. ready は差で拾う（オーナーの課題とレビュアーの子課題は除く・ルーチンは detail.routine）
p2=$(as_cos kanban add '新しく引き取れる')
p3=$(as_cos kanban add 'オーナーがする'); as_cos kanban set "$p3" who=person >/dev/null
p4=$(as_cos kanban add 'ルーチンが足した'); as_cos kanban set "$p4" routine=nightly >/dev/null
# レビュアーを起動しそこねて todo に残った子課題。起動するのは dispatcher だけなので、引き取れるとは知らせない
p5=$(as_cos kanban add --parent "$p2" "$p2 のレビュー"); as_cos kanban set "$p5" profile=codex-review >/dev/null
# 同じ親の子課題でも、レビュアーでない（観察など）ものは知らせる
p6=$(as_cos kanban add --parent "$p2" '観察'); as_cos kanban set "$p6" profile=sonnet >/dev/null
kanban ready --json | jq -e --arg p "$p5" 'any(.[]; .id == $p and .profile == "codex-review" and .parent != null)' >/dev/null ||
    die "レビュアーの子課題が kanban ready に出ない（検査が何も見ていない）: $(kanban ready --json | jq -c .)"
kanban ready --json >"$tmp/ready.json"; emit "$tmp/ready.json" ledger-ready.json
pk
pp "$p2" ready | jq -e '.[0].detail == {title: "新しく引き取れる"}' >/dev/null || die "増えた ready を拾わない: $(pp "$p2" ready)"
pp "$p4" ready | jq -e '.[0].detail == {title: "ルーチンが足した", routine: "nightly"}' >/dev/null || die "ルーチンの ready に routine が無い: $(pp "$p4" ready)"
[ "$(pp "$p1" ready)" = '[]' ] || die '前回もあった ready を拾った'
[ "$(pp "$p3" ready)" = '[]' ] || die 'オーナーの課題の ready を拾った'
[ "$(pp "$p5" ready)" = '[]' ] || die "レビュアーの子課題の ready を拾った: $(pp "$p5" ready)"
pp "$p6" ready | jq -e '.[0].detail == {title: "観察"}' >/dev/null || die "レビュアーでない子課題の ready を拾わない: $(pp "$p6" ready)"
jq -e --arg p "$p5" '.ready | index($p) == null' "$pst" >/dev/null || die "レビュアーの子課題を ready の控えに入れた: $(jq -c .ready "$pst")"
emit <(pp "$p4" ready | jq '.[0]') notification-ready.json
clear_pending; pk
[ "$(jq '[.pending[] | select(.kind == "ready")] | length' "$pst")" = 0 ] || die '同じ ready を二度拾った'
ok 'ready は前回との差で拾い（オーナーの課題とレビュアーの子課題は除く・routine を添える）、同じものを二度拾わない'

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
# 問いの JSON が kanban の行より遅れて置かれたら、1回だけ待って拾う。JSON の無い問い（kanban の CLI を直に打った）は待ったあと飛ばす
as_w kanban ask "$p2" '直に打った問い' >/dev/null
pk
[ "$(pp "$p2" asked | jq -r '.[0].detail.n')" = 1 ] && jq -e '.log_hold != null' "$pst" >/dev/null || die '問いの JSON を待たない'
mkdir -p "$P/issues/$p2/library/questions"; echo "$qjson" > "$P/issues/$p2/library/questions/2.json"
pk
pp "$p2" asked | jq -e '[.[].detail.n] == [1, 2]' >/dev/null && jq -e 'has("log_hold") | not' "$pst" >/dev/null || die "遅れて置かれた問いの JSON を拾わない（2つ目の問いが1つ目を消した）: $(pp "$p2" asked)"
as_w kanban ask "$p2" 'JSON の無い問い' >/dev/null
as_w kanban report "$p2" --append <<<'その後' >/dev/null
clear_pending; pk; pk
[ "$(pp "$p2" asked)" = '[]' ] && [ "$(pp "$p2" report | jq length)" = 1 ] || die "JSON の無い問いで止まり続けた・あとの行を拾わない: $(jq -c .pending "$pst")"
grep -q "$p2 の問い 3 の JSON が読めない" "$tmp/dispatcher.log" || die '飛ばした問いがログに無い'
ok 'log.jsonl の問いを問いの JSON から拾い（Chief of Staff の問いは拾わない）、JSON が遅れたら1回だけ待ち、無ければ飛ばして続きを読む'

mkdir -p "$P/issues/$p2/library"
jq -n '{commit: "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", pr: "https://github.com/example/app/pull/7"}' > "$P/issues/$p2/library/report.json"
as_w kanban review "$p2" --force >/dev/null   # 上の問い（open のまま）を残して review に入れる
p5=$(as_cos kanban add 'PR の無い課題'); as_w kanban start "$p5" >/dev/null; as_w kanban review "$p5" >/dev/null
p6=$(as_cos kanban add 'Chief of Staff がレビューに入れる'); as_cos kanban review "$p6" >/dev/null
clear_pending; pk
pp "$p2" review | jq -e --arg r "$P/issues/$p2/report.md" '.[0].detail == {report: $r, pr: "https://github.com/example/app/pull/7"}' >/dev/null || die "review を拾わない: $(pp "$p2" review)"
pp "$p5" report | jq -e 'length == 1' >/dev/null && [ "$(pp "$p5" review)" = '[]' ] || die "PR の無い review を report で知らせない: $(jq -c .pending "$pst")"
[ "$(jq --arg t "$p6" '[.pending[] | select(.issue == $t)] | length' "$pst")" = 0 ] || die 'Chief of Staff 自身の review を拾った'
emit <(pp "$p2" review | jq '.[0]') notification-review.json
ok 'log.jsonl の review を report.json の pr つきで拾い（PR が無ければ report）、Chief of Staff の review は拾わない'

# 担当の課題と時刻の道具（18c・22・23・25 が使う）
st_task() { local id; id=$(as_cos kanban add "$1"); HC_TERMINAL_ID=$2 kanban start "$id" >/dev/null; echo "$id"; }
old_idle=$(( $(now_ms) - 16 * 60 * 1000 )); old_work=$(( $(now_ms) - 21 * 60 * 1000 )); fresh=$(( $(now_ms) - 60 * 1000 ))
# 18c. 担当が自分の問いの答えを待つ間（受け取られていない問いが open・answered）は stalled を積まず、received したあとは積む（A-0221）
a=$(st_task '問いの答えを待つ' w32); termp w32 idle "$old_idle"
mkdir -p "$P/issues/$a/library/questions"; echo "$qjson" > "$P/issues/$a/library/questions/1.json"
HC_TERMINAL_ID=w32 kanban ask "$a" 'どちらの名前にする？' >/dev/null
clear_pending; pk
pp "$a" asked | jq -e 'length == 1' >/dev/null && [ "$(pp "$a" stalled)" = '[]' ] || die "問いの答えを待つ担当を stalled で知らせた・asked を積まない: $(jq -c .pending "$pst")"
HC_TERMINAL_ID=t5 kanban answer "$a#1" 'A' >/dev/null
pk
[ "$(pp "$a" stalled)" = '[]' ] || die "答えを受け取る前（answered）の担当を stalled で知らせた: $(pp "$a" stalled)"
HC_TERMINAL_ID=w32 kanban received "$a#1" >/dev/null
pk
pp "$a" stalled | jq -e --argjson ms "$old_idle" '.[0].detail == {agent: "w32", status_ms: $ms}' >/dev/null || die "問いを受け取ったあと idle のまま15分を stalled で知らせない: $(pp "$a" stalled)"
ok '担当が自分の問いの答えを待つ間（open・answered）は stalled を積まず、received したあとは積む'

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
ok 'プロバイダのエラーで続きを送れなかった試行も回数と時刻に数え、上限を超えたら error で知らせる'

# 27. 件数のきっかけ（ADR-0039 決めたこと 7・8。受け入れ条件 10・11。dispatcher.md §2.1.1 の 3a）。別の kanban rt で、
#     project.json の chief_of_staff.retro_every を 3 にする（オーナーの commit）。ルーチン retro の schedule は 1 月 1 日だけ（検査の間は撃たない）
mkdir -p "$tmp/plant/projects/rt"
cd "$tmp/plant/projects/rt"
env -u KANBAN_ROOT kanban init --prefix R >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t6 >/dev/null
R=$PWD/kanban
echo '{"version": 1, "project": {"title": "Retro", "prefix": "R"}, "repos": [], "chief_of_staff": {"retro_every": 3}}' > project.json
as_owner git -C "$tmp/plant" add projects/rt/project.json; as_owner git -C "$tmp/plant" commit -qm 'rt の project.json'
mkdir -p "$R/routines"
routine() { printf 'title: 振り返り\nbrief: 振り返り\nprofile: claude\nschedule: "0 0 1 1 *"\n%s' "${1:-}" > "$R/routines/retro.yaml"; }
routine
rk() { "$andon" dispatcher --once --project rt 2>>"$tmp/dispatcher.log"; }
retros() { kanban list --all --json | jq -r '[.[] | select(.routine == "retro")] | length'; }   # ルーチン retro が足した課題の数
open_retro() { kanban list --all --json | jq -r '[.[] | select(.routine == "retro" and .status != "done")][0].id // empty'; }
routine_rows() { grep -c '"by":"routine:retro"' "$R/log.jsonl" || true; }   # ルーチンが撃った・撃ち損ねた行
workers() { local i id; for i in $(seq 1 "$1"); do id=$(as_cos kanban add "担当の課題 $i" --profile claude); as_cos kanban done "$id" >/dev/null; done; }
close_retro() { # close_retro <id>: 振り返りが集めて（retro.json の until）、1 秒あとに片付く。境目は until
    local u; u=$(date -u +%Y-%m-%dT%H:%M:%SZ); sleep 1
    mkdir -p "$R/issues/$1/library"
    jq -n --arg r "$1" --arg u "$u" '{retro: $r, at: $u, since: $u, until: $u, days: 7, previous_retro: null, patterns: [], fixes: []}' > "$R/issues/$1/library/retro.json"
    as_cos kanban done "$1" >/dev/null; sleep 1
}
workers 2                                   # 前の振り返りより前に片付いた担当の課題（数えない）
as_cos kanban routine run retro >/dev/null  # 前の振り返り
close_retro "$(open_retro)"
rk
[ "$(retros)" = 1 ] || die "前の振り返りより前の課題を数えて撃った: $(retros)"
workers 2; rk
[ "$(retros)" = 1 ] || die "件数（3）に足りない 2 件で撃った: $(retros)"
workers 1; rk
[ "$(retros)" = 2 ] && [ -n "$(open_retro)" ] || die "件数に達した巡で振り返りの課題を足さない: $(retros)"
grep -q 'rt: 片付いた担当の課題が 3 件（設定 3 件）たまったので、振り返りのルーチンを撃った' "$tmp/dispatcher.log" || die "撃ったことがログに無い: $(tail -3 "$tmp/dispatcher.log")"
rk
[ "$(retros)" = 2 ] || die "次の巡でもう一度足した: $(retros)"
ok '前の振り返りが集めたあとに片付いた担当の課題が設定の件数に達した巡で、振り返りの課題を 1 件足し、次の巡では足さない'

close_retro "$(open_retro)"
for i in 1 2 3; do p=$(as_cos kanban add "親 $i" --profile claude); c=$(as_cos kanban add "レビュー $i" --profile codex-review --parent "$p"); as_cos kanban done "$c" >/dev/null; done
rk
[ "$(retros)" = 2 ] || die "レビュアーの子の課題を数えて撃った: $(retros)"
routine 'paused: true
'
workers 3; rk
[ "$(retros)" = 2 ] || die "paused のルーチンを撃った: $(retros)"
routine; rk
[ "$(retros)" = 3 ] || die "paused を外した巡で撃たない（数え方が違う）: $(retros)"
workers 3
rows=$(routine_rows)
rk; rk; rk
[ "$(retros)" = 3 ] && [ "$(routine_rows)" = "$rows" ] || die "振り返りが残っている間に撃った・撃ち損ねの行を書いた: $(retros) 件・行 $rows → $(routine_rows)"
ok '数えるのは担当の課題だけで、paused のルーチンは撃たず、振り返りが残っている間は 3 巡回しても kanban の履歴にルーチンの行を足さない'

# 28. 空き（ADR-0047 決めたこと 4・11・12。受け入れ条件 6・7。dispatcher.md §2.13）。別の kanban ie（Chief of Staff は t4。生きている）。
#     project.json は無い（idle_minutes は既定の 15 分）。並行の上限は既定の 2 で、動いている課題が無いので枠は 2 つ空いている。
#     --once は起動し直しに数えない。見る時刻は、控えの idle.at を 16 分前に戻して来させる
mkdir -p "$tmp/plant/projects/ie"
cd "$tmp/plant/projects/ie"
env -u KANBAN_ROOT kanban init --prefix I >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t4 >/dev/null
echo "t4 chief-of-staff" >> "$tmp/hc/live"
echo '{"address":"t4","state":"live","status":{"state":"working"},"labels":{}}' > "$tmp/hc/term-t4.json"
I=$PWD/kanban
ist=$tmp/state/andon/dispatcher/ie.json
ik() { "$andon" dispatcher --once --project ie 2>>"$tmp/dispatcher.log"; }
later() { jq '.idle.at = (now - 16 * 60 | strftime("%Y-%m-%dT%H:%M:%SZ"))' "$ist" > "$ist.tmp" && mv "$ist.tmp" "$ist"; }   # 前に見てから 16 分たった
pidle() { jq -c '[.pending[] | select(.kind == "idle")]' "$ist"; }
w1=$(HC_TERMINAL_ID=t4 kanban add "起動できる担当の課題" --profile claude)
ow=$(HC_TERMINAL_ID=t4 kanban add "オーナーの課題（オーナーの判断待ち）"); HC_TERMINAL_ID=t4 kanban set "$ow" who=person >/dev/null
rp=$(HC_TERMINAL_ID=t4 kanban add "レビューの親" --profile claude --status doing)
rv=$(HC_TERMINAL_ID=t4 kanban add "レビュー" --profile codex-review --parent "$rp" --round 1)
kanban ready --json | jq -e --arg o "$ow" --arg r "$rv" 'map(.id) | index($o) and index($r)' >/dev/null || die "前提: オーナーの課題とレビュアーの子が ready に出ない: $(kanban ready --json | jq -c 'map(.id)')"
ik
[ "$(pidle)" = '[]' ] || die "初めて見る kanban で idle を積んだ: $(pidle)"
later; ik
pidle | jq -e --arg w "$w1" 'length == 1 and (.[0] | has("issue") | not) and .[0].detail == {free: 1, unstarted: [$w], fired: [], lint: 0}' >/dev/null \
  || die "2 回続けて ready に出た担当の課題で idle を積まない・形が違う（オーナーの課題とレビュアーの子は入らない）: $(pidle)"
emit <(pidle | jq '.[0]') notification-idle.json
later; ik
pidle | jq -e --arg w "$w1" 'length == 1 and .[0].detail.unstarted == [$w]' >/dev/null || die "同じ課題でもう一度 idle を積んだ: $(pidle)"
w2=$(HC_TERMINAL_ID=t4 kanban add "あとから起動できる担当の課題" --profile claude)
later; ik; later; ik; later; ik
pidle | jq -e --arg a "$w1" --arg b "$w2" 'length == 1 and .[0].detail.unstarted == [$a, $b]' >/dev/null \
  || die "届く前に積んだ次の idle が、前の unstarted を残さない: $(pidle)"
jq '.pending = []' "$ist" > "$ist.tmp" && mv "$ist.tmp" "$ist"
later; ik
[ "$(pidle)" = '[]' ] || die "知らせた課題で、届いたあとにもう一度 idle を積んだ: $(pidle)"
ok '前の回にも今の回にも ready に出ている担当の課題を idle で 1 回だけ知らせ、届く前の次の idle に前の unstarted を残す（オーナーの課題とレビュアーの子は入れない）'

d1=$(HC_TERMINAL_ID=t4 kanban add "片付いた課題" --profile claude); HC_TERMINAL_ID=t4 kanban done "$d1" >/dev/null
echo "parent: I-9999" >> "$I/issues/$d1/issue.yaml"   # kanban lint の problem（無い親）
! kanban lint >/dev/null 2>&1 || die '前提: kanban lint が problem を出さない'
later; ik
pidle | jq -e 'length == 1 and .[0].detail.lint == 1 and .[0].detail.unstarted == []' >/dev/null || die "kanban lint の problem の件数が idle に入らない: $(pidle)"
jq '.pending = []' "$ist" > "$ist.tmp" && mv "$ist.tmp" "$ist"
later; ik
[ "$(pidle)" = '[]' ] || die "同じ problem のままの次の回に idle を積んだ: $(pidle)"
ok 'kanban lint が problem を出す回に idle の lint へ件数を入れ、同じ problem のままの次の回には積まない'

echo "all passed ($pass)"
