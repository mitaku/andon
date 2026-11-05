#!/usr/bin/env bash
# `andon courier`（インナーループ。docs/courier.md §5 の P6a）。本物の台帳の CLI（kanban）と git・一時の建屋の repo・偽の hc・偽の gh で回す。
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
unset FLIGHTDECK_ROOT HC_TERMINAL_ID
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
  "pr view 7 --repo example/app --json state,mergeCommit")
    if [ -f \$d/merged ]; then jq -n --arg o "\$(cat \$d/merged)" '{state:"MERGED",mergeCommit:{oid:\$o}}'; else echo '{"state":"OPEN","mergeCommit":null}'; fi ;;
  "pr view 7 --repo example/app --json "*) cat \$d/view.json ;;
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
# 偽の hc: 生きている端末は $tmp/hc/live（address task の行）。run は t9, t10 … を返し、打ったものを $tmp/hc/log に。
# get --json は $tmp/hc/term-<addr>.json、read は screen-<addr>、attach --observe は ansi-<addr>（無ければ何も出さない）
cat > "$tmp/bin/hc" <<EOF
#!/bin/bash
d=$tmp/hc
echo "\$*" >> \$d/log
case \$1 in
  get) grep -q "^\$2 " \$d/live || exit 1; [ "\${3:-}" != --json ] || cat \$d/term-\$2.json 2>/dev/null ;;
  read) cat \$d/screen-\$2 2>/dev/null ;;                        # hc read <addr> [--lines n]: 文字だけの画面
  attach) [ "\$2" = --observe ] && cat \$d/ansi-\$3 2>/dev/null ;;  # hc attach --observe <addr>: ANSI の画面
  label) echo "\$2 \$3" >> \$d/labels ;;
  run) n=\$(( \$(cat \$d/next 2>/dev/null || echo 9) )); echo \$((n + 1)) > \$d/next
       task=\$(printf '%s\n' "\$@" | sed -n 's/^task=//p')
       echo "t\$n \$task" >> \$d/live
       printf '{"address":"t%s","first_message":{"sent":%s}}\n' "\$n" "\$(cat \$d/first 2>/dev/null || echo true)" ;;
  send) [ "\$2" != --if ] || shift 2; printf '%s\t%s\n' "\$2" "\$3" >> \$d/sent ;;
  ls) task=\$(printf '%s\n' "\$@" | sed -n 's/^task=//p')
      awk -v t="\$task" '\$2 == t { printf "%s{\"address\":\"%s\",\"state\":\"live\"}", (n++ ? "," : ""), \$1 }' \$d/live | { printf '{"terminals":['; cat; printf ']}\n'; } ;;
  kill) ;;
  rm) sed -i "/^\$2 /d" \$d/live ;;
  *) exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH"
echo "t5 chief-of-staff" > "$tmp/hc/live"

# 建屋の repo（人のもの）: 条件表（squash・検査）と委譲（merge B）を owner が commit する
git init -q "$tmp/plant"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
mkdir -p "$tmp/plant/projects/demo"
echo '{"version": 1, "project": "demo", "preset": "work"}' > "$tmp/plant/projects/demo/delegation.json"
as_owner git -C "$tmp/plant" add -A; as_owner git -C "$tmp/plant" commit -qm plant
cd "$tmp/plant/projects/demo"
kanban init --prefix D --repo github.com/example/app >/dev/null
export FLIGHTDECK_ROOT=$PWD
kanban config set coordinator=t5 >/dev/null
L=$PWD/.flightdeck
courier() { "$andon" courier --once --project demo 2>>"$tmp/courier.log"; }
state=$tmp/state/andon/courier/demo.json
pending() { jq -c --arg t "$1" --arg k "$2" '[.pending[] | select(.task == $t and .kind == $k)] | last' "$state"; }

# Chief of Staff がタスクを切る（t5 が予約キーを書く）。担当 t7 が worktree で作業し、PR 7 を出す
newtask() { # newtask <題> <merge> → id
    local id; id=$(HC_TERMINAL_ID=t5 kanban add "$1" --profile claude --repo github.com/example/app)
    HC_TERMINAL_ID=t5 kanban set "$id" review=on must_fix=major merge="$2" >/dev/null
    git -C "$app" fetch -q origin
    git -C "$app" worktree add -q -b "demo/$id" "$HOME/.herdr/worktrees/app/demo-$id" origin/main
    HC_TERMINAL_ID=t5 kanban set "$id" "branch=demo/$id" "worktree=$HOME/.herdr/worktrees/app/demo-$id" >/dev/null  # andon task start の代わり
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
as_worker() { (cd "$HOME/.herdr/worktrees/app/demo-$1" && HC_TERMINAL_ID=t7 "$andon" task "$2" "$1" "${@:3}" <<<'## Report' >/dev/null); }
review_by() { # review_by <レビュアーの id> <findings>: レビュアーが findings.json を書いて review にする
    mkdir -p "$L/tasks/$1/library"
    jq -n --arg c "$head" --argjson f "$2" '{commit: $c, findings: $f}' > "$L/tasks/$1/library/findings.json"
    HC_TERMINAL_ID=t9 kanban review "$1" >/dev/null
}

# 0. Chief of Staff の端末が生きていなければ台帳を飛ばす。初めて見る台帳は控えるだけ
id=$(newtask 'docs を直す' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
sed -i '/^t5 /d' "$tmp/hc/live"
courier
[ ! -e "$state" ] || die 'Chief of Staff がいないのに回った'
echo "t5 chief-of-staff" >> "$tmp/hc/live"
courier
[ -e "$state" ] && ! grep -q '^run' "$tmp/hc/log" || die '初めて見る台帳で動いた'
jq -e '.pending == [] and (.reviewers | length) == 1' "$state" >/dev/null || die '初めて見た review_request を控えていない'
ok 'Chief of Staff の端末が無ければ飛ばし、初めて見る台帳は控えるだけ'

# 1. 一周: review-request → レビュアーを起こす → レビュー → ready → ゲート → マージ → finish
mark=$(wc -l < "$L/log.jsonl")
work "$id" docs/a.md
as_worker "$id" report
as_worker "$id" review-request
courier
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
[ -n "$rev" ] || die 'レビュアーのタスクが無い'
kanban show "$rev" --json | jq -e --arg p "$id" '.task.profile == "codex-review" and .task.status == "doing" and .task.agent == "t9" and .task.parent == $p' >/dev/null ||
    die "レビュアーのタスクの項目が違う: $(kanban show "$rev" --json)"
jq -e --arg r "$rev" 'select(.id == $r and has("change") and .change.add != null) | .by == "terminal:courier"' "$L/log.jsonl" >/dev/null || die 'レビュアーのタスクの作成が courier でない'
rwt=$HOME/.herdr/worktrees/app/demo-$id-r1
[ "$(git -C "$rwt" rev-parse HEAD)" = "$head" ] && git -C "$app" worktree list --porcelain | grep -qx "worktree $rwt" || die 'detached worktree が head に無い'
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$(echo "$rev" | tr A-Z a-z) .*--cwd $L/tasks/$rev codex-review" "$tmp/hc/log" || die "hc run の形が違う: $(grep ^run "$tmp/hc/log")"
grep -q "hc send t7 " "$L/tasks/$rev/brief.md" && grep -q 'Chief of Staff・クーリエへは送らない' "$L/tasks/$rev/brief.md" || die 'レビュアーの依頼書に宛先（担当 t7）が無い'
grep -q "commit $head" "$L/tasks/$rev/brief.md" && ! grep -q '## Report' "$L/tasks/$rev/brief.md" || die 'レビュアーの依頼書に commit が無い・担当の報告が入った'
pending "$id" review-requested | jq -e --arg r "$rev" '.detail == {round: 1, reviewer: "codex-review", reviewer_task: $r}' >/dev/null || die '通知 review-requested が無い'
emit <(pending "$id" review-requested) notification-review-requested.json
ok "review_request でレビュアーを起こす（kanban add --parent・by terminal:courier・detached worktree・hc run --no-notify・依頼書の宛先は担当）"
courier
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '同じ review_request で二度起こした'
ok '同じ review_request（task と commit）で二度起こさない'

review_by "$rev" '[{"severity":"minor","file":"docs/a.md","line":1}]'
as_worker "$id" report --data '{"waived":[{"severity":"minor","file":"docs/a.md","line":1,"reason":"起きる条件が無い"}]}'
as_worker "$id" ready
courier
rec=$L/tasks/$id/library/gate/$head.json
jq -e '.decision == "approve" and .merged == true and .tier == "B"' "$rec" >/dev/null || die "ゲートの記録が approve・merged でない: $(cat "$rec")"
grep -qx "pr merge 7 --repo example/app --squash --match-head-commit $head" "$tmp/gh/merges" || die 'マージの打ち方が違う'
kanban show "$id" --json | jq -e '.task.status == "done"' >/dev/null || die 'タスクが done でない'
kanban show "$rev" --json | jq -e '.task.status == "done"' >/dev/null || die 'レビュアーのタスクが done でない'
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

# 2. fix は担当へ hc send（Must fix が残る）
id=$(newtask '説明を足す' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
courier
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
review_by "$rev" '[{"severity":"major","file":"docs/a.md","line":1}]'
as_worker "$id" ready
courier
grep -q "^t7	$id（説明を足す）: ゲートが fix を返した。理由: Must fix 以上が 1 件" "$tmp/hc/sent" || die "fix が担当に届いていない: $(cat "$tmp/hc/sent" 2>/dev/null)"
[ ! -e "$tmp/gh/merged" ] && kanban show "$id" --json | jq -e '.task.status == "review"' >/dev/null || die 'fix なのにマージした・片付けた'
courier
[ "$(grep -c "	$id（" "$tmp/hc/sent")" = 1 ] || die '同じ review 入りで二度ゲートを回した'
ok 'fix は理由を担当（task.yaml の agent）へ hc send し、同じ review 入りでは二度回さない'

# 3. needs_human は問い（from: gate）。レベル C は委譲 merge B の外
restart "$id"
work "$id" package/gate.json
as_worker "$id" report
as_worker "$id" review-request
courier
rev=$(kanban list --all --json | jq -r --arg p "$id" '[.[] | select(.parent == $p)] | last | .id')
review_by "$rev" '[]'
as_worker "$id" ready
courier
q=$L/tasks/$id/library/questions/1.json
jq -e '.from == "gate" and .kind == "line" and (.facts | any(contains("delegation: merge=B < tier C"))) and .options[0].pick == true' "$q" >/dev/null || die "問いが無い・形が違う: $(cat "$q" 2>/dev/null)"
kanban questions --json 2>/dev/null | jq -e --arg t "$id" 'any(.[]; .task == $t)' >/dev/null || kanban questions | grep -q "$id" || die '台帳に問いが無い'
pending "$id" asked | jq -e '.detail.n == 1 and .detail.kind == "line"' >/dev/null || die '通知 asked が無い'
[ ! -e "$tmp/gh/merged" ] || die 'needs_human なのにマージした'
emit "$q" question-gate.json
emit <(pending "$id" asked) notification-asked.json
ok 'needs_human は問い（from: gate・kind line）を台帳に出し、マージしない'

# 4. merge: person は approve でも止まる（通知 gate だけ）
id=$(newtask '人がマージする' person)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
courier
rev=$(kanban list --all --json | jq -r --arg p "$id" '.[] | select(.parent == $p) | .id')
review_by "$rev" '[]'
as_worker "$id" ready
courier
pending "$id" gate | jq -e '.detail.decision == "approve"' >/dev/null || die '通知 gate が無い'
[ ! -e "$tmp/gh/merged" ] && kanban show "$id" --json | jq -e '.task.status == "review"' >/dev/null || die 'merge: person なのにマージした'
ok 'merge: person は approve でもマージせず、通知 gate で止まる（人がマージする）'

# 5. 上限を超えた回（extra）は起こさない（Chief of Staff が決める）
HC_TERMINAL_ID=t5 kanban set "$id" review_rounds=1 >/dev/null
restart "$id"
work "$id" docs/a.md
as_worker "$id" report
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
courier
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '上限を超えた回でレビュアーを起こした'
pending "$id" report | jq -e '.detail.report | endswith("/report.md")' >/dev/null || die '通知 report が無い'
grep -q '追加の回（2 回目／上限 1）' "$tmp/courier.log" || die '起こさない理由がログに無い'
courier
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] || die '上限を超えた回を二度目の見回りで起こした'
HC_TERMINAL_ID=t5 kanban set "$id" review_rounds=2 >/dev/null
courier
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die 'Chief of Staff が review_rounds を上げたのに起こさない'
ok '上限を超えた回（extra）は起こさず通知 report。Chief of Staff が review_rounds を上げたら次の見回りで起こす'

# 6. andon task finish（人がマージしたあと Chief of Staff が打つ）はレビュアーの子タスクと detached worktree も片付ける
revs=$(kanban list --all --json | jq -r --arg p "$id" '[.[] | select(.parent == $p and .status != "done")] | length')
[ "$revs" -ge 2 ] && ls -d "$HOME/.herdr/worktrees/app/demo-$id-r"* >/dev/null || die 'レビュアーの子タスク・detached worktree が無い（前提）'
HC_TERMINAL_ID=t5 "$andon" task finish "$id" >/dev/null
kanban list --all --json | jq -e --arg p "$id" 'all(.[] | select(.id == $p or .parent == $p); .status == "done")' >/dev/null || die '子タスクが done でない'
! ls -d "$HOME/.herdr/worktrees/app/demo-$id"* >/dev/null 2>&1 || die 'worktree が残った'
git -C "$app" show-ref --verify --quiet "refs/heads/demo/$id" || die 'マージしていない branch を消した'
ok 'andon task finish はレビュアーの子タスクと detached worktree も片付け、origin に入っていない branch は残す'

# 7. 最初のメッセージが届かなければ起こしたことにしない。次の見回りで同じ端末へ送り直す（二重に起こさない）
id=$(newtask '最初の文が届かない' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
echo false > "$tmp/hc/first"
courier
rm "$tmp/hc/first"
key="$id@$head"
jq -e --arg k "$key" '.reviewers[$k] | (.started | not) and .failed != null and .reviewer_task != null and .address != null' "$state" >/dev/null || die "届いていないのに起こしたことにした: $(jq -c --arg k "$key" '.reviewers[$k]' "$state")"
[ "$(pending "$id" review-requested)" = null ] || die '届いていないのに通知 review-requested を積んだ'
addr=$(jq -r --arg k "$key" '.reviewers[$k].address' "$state")
courier
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] || die '二重に起こした'
grep -q "^$addr	タスク \`demo/" "$tmp/hc/sent" || die '同じ端末へ最初のメッセージを送り直していない'
jq -e --arg k "$key" '.reviewers[$k].started == true' "$state" >/dev/null && [ "$(pending "$id" review-requested)" != null ] || die '送り直したのに起こしたことにならない'
[ "$(kanban list --all --json | jq --arg p "$id" '[.[] | select(.parent == $p)] | length')" = 1 ] || die 'レビュアーのタスクを二つ足した'
ok '最初のメッセージが届くまでは起こしたことにせず、次の見回りで同じ端末へ送り直す（タスクも端末も二つにしない）'

# 8. fetch の一時の失敗は次の見回りでやり直す
id=$(newtask 'fetch が一度落ちる' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
runs=$(grep -c '^run' "$tmp/hc/log")
insteadof() { git config --global --remove-section "url.file://$2" 2>/dev/null || true; git config --global url."file://$1".insteadOf https://github.com/example/app; }
insteadof "$tmp/nowhere" "$tmp/origin"
courier
insteadof "$tmp/origin" "$tmp/nowhere"
[ "$(grep -c '^run' "$tmp/hc/log")" = "$runs" ] && jq -e --arg k "$id@$head" '.reviewers[$k].failed | contains("fetch")' "$state" >/dev/null || die 'fetch に失敗したのに起こした・控えに失敗が無い'
courier
[ "$(grep -c '^run' "$tmp/hc/log")" = "$((runs + 1))" ] && jq -e --arg k "$id@$head" '.reviewers[$k].started == true' "$state" >/dev/null || die 'fetch が戻ったのに起こさない'
ok 'fetch の一時の失敗は控えで成功と分け、次の見回りで起こす'

# 9. ゲートのあと Chief of Staff が merge: person に変えたら、マージせず次の見回りでゲートから回し直す
gate_through() { # gate_through <id>: レビュアーが指摘なしで返し、担当が ready まで
    local rev; rev=$(kanban list --all --json | jq -r --arg p "$1" '[.[] | select(.parent == $p)] | last | .id')
    review_by "$rev" '[]'
    as_worker "$1" ready
}
gate_through "$id"
merges=$(wc -l < "$tmp/gh/merges")
echo "cd $PWD && HC_TERMINAL_ID=t5 kanban set $id merge=person >/dev/null" > "$tmp/gh/hook"
courier
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'ゲートのあと merge: person に変えたのにマージした'
grep -q "$id をいまはマージしない.*予約キーがゲートのあとに変わった" "$tmp/courier.log" || die '止めた理由がログに無い'
courier
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] && kanban show "$id" --json | jq -e '.task.status == "review"' >/dev/null || die '回し直しでマージした'
[ "$(grep -c "^status .*$head" "$tmp/gh/posted")" = 2 ] || die 'ゲートを回し直していない'
ok 'ゲートのあと merge: person に変わったら gh pr merge を打たず、次の見回りでゲートから回し直す（approve でも person なので止まる）'

# 10. ゲートのあとオーナーが委譲の merge を取り消したら、マージしない（回し直したゲートは needs_human）
id=$(newtask '委譲が取り消される' auto)
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
as_worker "$id" review-request
courier
gate_through "$id"
echo "cd $tmp/plant/projects/demo && env -u HC_TERMINAL_ID GIT_AUTHOR_NAME=o GIT_AUTHOR_EMAIL=owner@example.com $andon delegate narrow merge none --evidence x >$tmp/hook.out 2>&1" > "$tmp/gh/hook"
courier
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'ゲートのあと委譲を取り消したのにマージした'
grep -q "$id をいまはマージしない.*委譲の merge がゲートのあとに変わった（B → none）" "$tmp/courier.log" || die '止めた理由がログに無い'
courier
jq -e '.from == "gate" and (.facts | any(contains("delegation: merge=none")))' "$L/tasks/$id/library/questions/1.json" >/dev/null || die '回し直したゲートの needs_human が問いになっていない'
[ "$(wc -l < "$tmp/gh/merges")" = "$merges" ] || die 'マージした'
ok 'ゲートのあと委譲の merge が取り消されたら gh pr merge を打たず、回し直したゲートの needs_human を問いにする'

# === フックで届ける・止める（P6b。courier.md §3・§4、contract.md §9）===
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
event() { # event <題>: クーリエが通知 review-requested を積む出来事（担当がレビューを頼む）。id を返す
    local id; id=$(newtask "$1" person)
    work "$id" docs/a.md
    as_worker "$id" report --pr https://github.com/example/app/pull/7
    as_worker "$id" review-request
    courier
    pending "$id" review-requested | jq -e . >/dev/null || die "$1: 通知が積まれていない"
    echo "$id"
}
sent() { cat "$tmp/hc/sent" 2>/dev/null | grep -c '^t5	'; }

# 11. turn 中の通知が Stop で届く（Chief of Staff が working の間、クーリエは hc send しない）
cos working 3
screen '❯' '❯'
id=$(event 'ターンの中の通知')
[ "$(sent)" = 0 ] || die 'working の Chief of Staff に hc send した'
out=$(HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"見た"}')
jq -e --arg id "$id" '.decision == "block" and (.reason | startswith("<news project=\"demo\">\n") and contains("\n\($id) review-requested: 1 回目のレビュアー ") and endswith("「<no-reply>」とだけ返す。\n</news>"))' <<<"$out" >/dev/null ||
    die "Stop が <news> で block しない: $out"
jq -e '.pending == [] and .hook == "t5"' "$state" >/dev/null || die '取り出した待ちが控えに残った'
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
out=$(HC_TERMINAL_ID=t5 sh "$hooks/user-prompt-submit" <<<'{"prompt":"もう一つ"}')
jq -e '.hookSpecificOutput.additionalContext | (contains("<news") | not) and contains("<delegation")' <<<"$out" >/dev/null || die "待ちが無いときに委譲だけを添えない: $out"
out=$(HC_TERMINAL_ID=t5 PATH=/usr/bin:/bin sh "$hooks/user-prompt-submit" <<<'{}'); rc=$?
[ "$rc" = 0 ] && [ -z "$out" ] || die 'andon が無いときに通さない'
HC_TERMINAL_ID=t5 sh "$hooks/stop" <<<'{"last_assistant_message":"了解"}' >/dev/null
! grep -q 'news.replied=3$' <(tail -n +2 "$tmp/hc/labels") || die '人への答えに news.replied を書いた'
ok '人が送ったとき、待ちと委譲の一覧を UserPromptSubmit の additionalContext に添える（andon が無ければ通す）'

# 13. 離席時だけ hc send（状態 idle・入力欄が空・前の <news> の turn が進んでいる）
id=$(event '離席の通知')
cos idle 4
screen '❯ これを直して' '❯ これを直して'
courier
[ "$(sent)" = 0 ] || die '人の書きかけがあるのに hc send した'
screen '❯' '❯ 前の文'
courier
[ "$(sent)" = 0 ] || die 'hc read と組み直した画面が違うのに hc send した'
screen '❯ Try "fix lint"' $'❯ \e[7m\e[2mTry "fix lint"\e[0m\e[?7h'
courier
[ "$(sent)" = 0 ] || die '許可の一覧に無い列があるのに hc send した'
screen '❯ Try "fix lint"' $'❯ \e[2mTry "fix lint"\e[22m'
cos working 4
courier
[ "$(sent)" = 0 ] || die 'working の Chief of Staff に hc send した'
cos idle 4
courier
[ "$(sent)" = 1 ] && grep -q "^send --if idle t5 <news project=\"demo\">" "$tmp/hc/log" || die "薄い字のサジェストだけの入力欄に hc send --if idle しない: $(grep '^send' "$tmp/hc/log" | tail -1)"
grep -q "^t5	<news project=\"demo\">" "$tmp/hc/sent" && grep -q "$id review-requested:" "$tmp/hc/sent" || die '<news> の文が違う'
jq -e '.pending == [] and .news_turn == 4' "$state" >/dev/null || die '送った turn を控えていない'
id2=$(event '答えている途中の通知')
courier
[ "$(sent)" = 1 ] || die '前の <news> の turn が進んでいないのに送った'
cos done 5
printf '❯ <news project="demo">\n  %s review-requested: x\n  </news>\n\n● <no-reply>\n\n%s\n❯\n%s\n' "$id" "$rule" "$rule" > "$tmp/hc/screen-t5"
courier
grep -qx 't5 news.replied=5' "$tmp/hc/labels" || die '③で送った <news> の <no-reply> に news.replied を書かない'
screen '❯' '❯'
courier
[ "$(sent)" = 2 ] && grep -q "^send t5 <news" "$tmp/hc/log" && grep -q "$id2 review-requested:" "$tmp/hc/sent" || die 'done の Chief of Staff に --if なしで送らない'
ok '離席時だけ hc send（idle か done・入力欄が空・薄い字は空・hc read と一致・許可の一覧の列だけ・前の turn が進んでから。idle は --if idle）'

# 14. doing で報告無しの Stop が block（担当・レビュアーの Stop フック。stop-check）
id=$(newtask '止まる前に報告する' person)
wt=$HOME/.herdr/worktrees/app/demo-$id
term t7 idle 1 project=demo "task=$id"
stop() { (cd "$wt" && HC_TERMINAL_ID=t7 sh "$hooks/stop" <<<"{\"stop_hook_active\":${1:-false}}"); }
out=$(stop)
jq -e --arg id "$id" '.decision == "block" and (.reason | startswith("\($id) は doing のまま"))' <<<"$out" >/dev/null || die "報告の無い doing の Stop が block しない: $out"
work "$id" docs/a.md
as_worker "$id" report --pr https://github.com/example/app/pull/7
[ -z "$(stop)" ] || die '報告を書いたのに block した'
[ -n "$(stop)" ] || die '前回の通過のあと報告も問いも無い（次のターン）のに通した'
restart "$id"
[ -n "$(stop)" ] || die '直しで doing に戻ったのに、報告の前の Stop を通した'
(cd "$wt" && HC_TERMINAL_ID=t7 "$andon" task ask "$id" - <<<'{"kind":"line","question":"どちらにする？","facts":["x"],"options":[{"label":"A","pick":true},{"label":"B"}]}' >/dev/null)
[ -z "$(stop)" ] || die '問いを書いたのに block した'
restart "$id"
stop >/dev/null; stop true >/dev/null; stop true >/dev/null
[ -z "$(stop true)" ] || die '続けて止める回数の上限を超えても block した'
term t7 idle 1
[ -z "$(stop)" ] || die 'ラベル task= の無い端末で block した'
ok 'doing で報告・問いの無い Stop は block（stop-check。前回の通過以後に報告か問いがあれば通す。続けて止めるのは3回まで）'

echo "all passed ($pass)"
