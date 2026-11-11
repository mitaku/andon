#!/usr/bin/env bash
# `andon gate` と `andon delegate`。本物の台帳の CLI（kanban）と git・一時のセルの repo・偽の gh で回す。
# PR の head は手元の origin から取る（git の insteadOf）。GitHub には出ない。台帳の CLI は tests/lib/kanban.sh が焼く。
# ANDON_EMIT_DIR があれば、ゲートの記録と委譲を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@example.com
export ANDON_HOME="$andon_root/package" ANDON_WORKSPACE="$tmp/plant" GHQ_ROOT="$tmp/ghq"
unset KANBAN_ROOT HC_TERMINAL_ID
mkdir -p "$HOME" "$tmp/bin" "$tmp/gh"
pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
as_owner() { GIT_AUTHOR_NAME=o GIT_AUTHOR_EMAIL=owner@example.com "$@"; }
as_agent() { GIT_AUTHOR_NAME=a GIT_AUTHOR_EMAIL=agent@example.com "$@"; }

# 案件 repo: origin（PR の head を持つ）と、ゲートが fetch する clone
git init -q "$tmp/origin"
as_agent git -C "$tmp/origin" commit -q --allow-empty -m base
git clone -q "$tmp/origin" "$tmp/ghq/github.com/example/app"
git config --global url."file://$tmp/origin".insteadOf https://github.com/example/app
change() { # change <path> <行>: origin に commit して head を返す
    mkdir -p "$tmp/origin/$(dirname "$1")"; echo "$2" >> "$tmp/origin/$1"
    git -C "$tmp/origin" add -A; as_agent git -C "$tmp/origin" commit -qm "$1"; git -C "$tmp/origin" rev-parse HEAD
}

# 偽の gh: PR の事実は $tmp/gh/ のファイル（GitHub の checks は rollup.json）。書き込み（status・comment）は $tmp/gh/posted に1行ずつ
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
d=$tmp/gh
case "\$*" in
  "pr view 7 --repo example/app --json headRefOid -q .headRefOid") jq -r .headRefOid \$d/view.json ;;
  "pr view 7 --repo example/app --json "*) cat \$d/view.json ;;
  "api graphql "*)  # statusCheckRollup.contexts を100件ずつ（after は前のページの件数）
    off=\$(printf '%s\n' "\$@" | sed -n 's/^after=//p'); echo "graphql \${off:-0}" >> \$d/pages
    jq --argjson off "\${off:-0}" --slurpfile v \$d/view.json '{data: {repository: {pullRequest: {headRefOid: \$v[0].headRefOid,
      commits: {nodes: [{commit: {oid: \$v[0].headRefOid, statusCheckRollup: (if length == 0 then null else
        {contexts: {pageInfo: {hasNextPage: (length > \$off + 100), endCursor: ((\$off + 100) | tostring)}, nodes: .[\$off:\$off + 100]}} end)}}]}}}}}' \$d/rollup.json ;;
  "pr view https://github.com/example/app/pull/7 --json headRefOid -q .headRefOid") jq -r .headRefOid \$d/view.json ;;
  "api --paginate --slurp repos/example/app/pulls/7/files") cat \$d/files.json ;;
  "pr diff 7 --repo example/app") cat \$d/diff ;;
  "api repos/example/app --jq .default_branch") echo main ;;
  "api user --jq .login") echo plant-bot ;;
  "api -X POST repos/example/app/statuses/"*) echo "status \$*" >> \$d/posted ;;
  "pr comment 7 --repo example/app --body "*) echo "comment" >> \$d/posted ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
# 偽の hc: HC_TERMINAL_ID の端末のラベル（$tmp/labels）
cat > "$tmp/bin/hc" <<EOF
#!/bin/sh
[ "\$1" = get ] && printf '{"address":"%s","labels":%s}\n' "\$2" "\$(cat $tmp/labels)"
EOF
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH"
# rollup <状態…>: GitHub の checks（CheckRun の status/conclusion か ctx:<state>）。ゲート自身の andon/gate の failure をいつも混ぜる
rollup() {
    printf '%s\n' "$@" | jq -R -s '[split("\n")[] | select(. != "") |
      if startswith("ctx:") then {__typename: "StatusContext", context: "ci/x", state: .[4:]}
      else (split("/") | {__typename: "CheckRun", name: "test", status: .[0], conclusion: (.[1] // "")}) end]
      + [{__typename: "StatusContext", context: "andon/gate", state: "FAILURE"}]' > "$tmp/gh/rollup.json"
}
rollup COMPLETED/SUCCESS
# pr <path> <行数>: PR の head を進め、view・files・diff を作る
pr() {
    head=$(change "$1" "${3:-x}")
    jq -n --arg h "$head" '{state:"OPEN",isDraft:false,headRefName:"feat",headRefOid:$h,baseRefName:"main",
      mergeStateStatus:"CLEAN",author:{login:"plant-bot"},changedFiles:1,isCrossRepository:false}' > "$tmp/gh/view.json"
    jq -n --arg p "$1" --argjson n "$2" '[[{filename:$p,additions:$n,deletions:0}]]' > "$tmp/gh/files.json"
    printf '+++ b/%s\n+%s\n' "$1" "${3:-x}" > "$tmp/gh/diff"
    git -C "$tmp/wt" fetch -q origin && git -C "$tmp/wt" checkout -q "$head"
}

# セルの repo（人のもの）と台帳
git init -q "$tmp/plant"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
as_owner git -C "$tmp/plant" add -A; as_owner git -C "$tmp/plant" commit -qm plant
mkdir -p "$tmp/plant/projects/demo"
cd "$tmp/plant/projects/demo"
kanban init --prefix D --repo github.com/example/app >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t5 >/dev/null
git clone -q "$tmp/origin" "$tmp/wt"
id=$(HC_TERMINAL_ID=t5 kanban add 'docs を直す' --profile claude)
HC_TERMINAL_ID=t5 kanban set "$id" review=on must_fix=major branch=feat "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
lib=kanban/issues/$id/library
gate() { local rc=0; "$andon" gate "$@" >"$tmp/out" 2>"$tmp/err" || rc=$?; echo "$rc"; }
expect() { # expect <rc> <decision> <理由に含む語>
    local rc=$1; shift
    [ "$got" = "$rc" ] && jq -e --arg d "$1" --arg w "${2:-}" '.decision == $d and ($w == "" or any(.reasons[]; contains($w)))' "$tmp/out" >/dev/null ||
        { cat "$tmp/out" "$tmp/err" >&2; die "want rc $rc $*, got rc $got"; }
}

# 1. report.json が無ければ記録も GitHub への書き込みもせず fix
pr docs/a.md 3
got=$(gate "$id"); expect 1 fix 'report.json'
[ ! -e "$lib/gate" ] && [ ! -e "$tmp/gh/posted" ] || die '信じられる PR が無いのに書いた'
ok '信じられる PR（report.json の pr）が無ければ記録せずに fix'

# 担当の手順: report → レビュアーの findings → ready
export HC_TERMINAL_ID=t7
"$andon" issue report "$id" --pr https://github.com/example/app/pull/7 <<<'## Report' >/dev/null
rev=$(HC_TERMINAL_ID=t5 kanban add "$id のレビュー" --parent "$id" --profile codex-review)
mkdir -p "kanban/issues/$rev/library"
findings() { jq -n --arg c "$1" --argjson f "${2:-[]}" '{commit: $c, findings: $f}' > "kanban/issues/$rev/library/findings.json"; }
findings "$head"
HC_TERMINAL_ID=t5 kanban review "$rev" >/dev/null
"$andon" issue ready "$id" >/dev/null
unset HC_TERMINAL_ID

# 2. 委譲が無ければ approve にならない（全部人）
got=$(gate "$id"); expect 4 needs_human 'delegation: merge=none < tier B'
rec="$lib/gate/$head.json"
jq -e '.tier == "B" and .delegation == {merge: "none", ok: false} and .review.state == "current" and (.review | has("issue") and (has("task") | not)) and .checks == [] and .merged == false' "$rec" >/dev/null
grep -q 'status .*state=failure' "$tmp/gh/posted" && grep -q comment "$tmp/gh/posted"
ok '委譲が無ければ needs_human（記録を書き、status と comment を付ける）'

# 3. andon delegate: 人が委譲を書く（project.json の delegation。project.json は先に人が書く）
rc=0; as_owner "$andon" delegate grant merge B --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'project.json が無い' "$tmp/err" && [ ! -e project.json ] || die 'project.json が無いのに書けた'
ok 'delegate: project.json が無ければ作らない（先に人が書く）'
# 移す前の delegation.json は、owner が commit していても読まない（無委譲）
echo '{"version": 1, "project": "demo", "preset": "work", "grants": [{"id": "d1", "scope": "merge", "value": "B", "by": "owner", "at": "2026-11-05", "evidence": "x"}]}' > delegation.json
as_owner git -C "$tmp/plant" add projects/demo/delegation.json; as_owner git -C "$tmp/plant" commit -qm 'legacy delegation.json'
"$andon" delegate show --json | jq -e '(.trusted | not) and .effective.merge == "none" and (.reason | contains("delegation.json は読まない"))' >/dev/null || die "delegation.json を読んだ: $("$andon" delegate show --json)"
got=$(gate "$id" --dry-run); expect 4 needs_human 'delegation: merge=none < tier B'
as_owner git -C "$tmp/plant" rm -q projects/demo/delegation.json; as_owner git -C "$tmp/plant" commit -qm 'legacy を消す'
ok 'delegate show・gate: 移す前の delegation.json だけのセルは無委譲（読まない）'
echo '{"version": 1, "project": {"title": "Demo", "prefix": "D"}, "repos": ["github.com/example/app"], "references": ["https://example.com/spec"]}' > project.json
as_owner git -C "$tmp/plant" add projects/demo/project.json; as_owner git -C "$tmp/plant" commit -qm 'project.json'
"$andon" delegate show --json | jq -e '(.trusted | not) and (.reason | contains("delegation が無い"))' >/dev/null || die 'delegation の無い project.json を委譲として読んだ'
echo '{"role":"chief-of-staff"}' > "$tmp/labels"
rc=0; HC_TERMINAL_ID=t5 "$andon" delegate grant merge B --evidence x --project demo >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'role=chief-of-staff' "$tmp/err" && ! jq -e 'has("delegation")' project.json >/dev/null || die 'Chief of Staff の端末で書けた'
ok 'delegate: Chief of Staff・担当の端末（role= / issue=）では打てない'
as_owner "$andon" delegate grant merge B --evidence '2026-11-05 の対話' >/dev/null
jq -e '.delegation | .version == 1 and .project == "demo" and .grants[0].id == "d1" and .grants[0].value == "B" and .grants[0].until == null' project.json >/dev/null
jq -e '.project == {title: "Demo", prefix: "D"} and .repos == ["github.com/example/app"] and .references == ["https://example.com/spec"]' project.json >/dev/null || die "人が書いたキーを変えた: $(cat project.json)"
[ ! -e delegation.md ] && [ "$(git -C "$tmp/plant" show --name-only --format= HEAD)" = projects/demo/project.json ] || die 'project.json のほかを書いた'
git -C "$tmp/plant" log -1 --format=%B | grep -q 'evidence: 2026-11-05 の対話'
emit project.json project.json
ok 'delegate grant: project.json の delegation だけを書き、根拠つきで commit する（cwd から slug。ほかのキーは人のまま）'
rc=0; as_owner "$andon" delegate narrow merge C --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '広げない' "$tmp/err" || die 'narrow で広げられた'
rc=0; as_owner "$andon" delegate grant irreversible delete-remote --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'delete-remote は委譲できない' "$tmp/err" || die 'delete-remote を委譲できた'
mkdir -p "$tmp/plant/projects/andon"
echo '{"version": 1, "project": {"title": "Andon", "prefix": "A"}, "repos": []}' > "$tmp/plant/projects/andon/project.json"
as_owner git -C "$tmp/plant" add projects/andon/project.json; as_owner git -C "$tmp/plant" commit -qm 'andon の project.json'
rc=0; as_owner "$andon" delegate grant merge C --evidence x --project andon >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'B まで' "$tmp/err" || die 'andon プロジェクトに merge C を委譲できた'
ok 'delegate: narrow は広げない・delete-remote と andon の merge C は断る'
# 作業ツリーの変更（エージェントが repos を足した、など）を人の commit に混ぜない。HEAD が owner の commit でなければ書かない
jq '.repos += ["github.com/example/other"]' project.json > "$tmp/p.json" && cp "$tmp/p.json" project.json
rc=0; as_owner "$andon" delegate grant profile true --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'commit していない変更がある' "$tmp/err" || die "未 commit の変更があるのに書けた: $(cat "$tmp/err")"
as_agent git -C "$tmp/plant" commit -qm 'agent が repos を足した' -- projects/demo/project.json
rc=0; as_owner "$andon" delegate grant profile true --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'owner でない' "$tmp/err" || die "owner でない HEAD の上に書けた: $(cat "$tmp/err")"
as_owner git -C "$tmp/plant" revert --no-edit HEAD >/dev/null
jq -e '.repos == ["github.com/example/app"] and (.delegation.grants | length) == 1' project.json >/dev/null || die "戻っていない: $(cat project.json)"
ok 'delegate: project.json に未 commit の変更がある・HEAD が owner の commit でなければ書かない（ほかの変更を人の commit に混ぜない）'
"$andon" delegate show --json | jq -e '.trusted and .preset == "work" and .effective.merge == "B" and .effective.deploy == "none"' >/dev/null
ok 'delegate show: HEAD の委譲を読み手と同じに読む'

# 4. 委譲の範囲内で揃えば approve
got=$(gate "$id"); expect 0 approve
jq -e '.delegation == {merge: "B", ok: true} and .checks == [{cmd: "test -f docs/a.md", ok: true}] and .reasons == []' "$rec" >/dev/null
jq -e '.github_checks == {state: "success", names: ["test"]}' "$rec" >/dev/null || die 'approve の記録に通した checks の名前が無い'
grep -q 'state=success' "$tmp/gh/posted"
emit "$rec" gate-record.json
ok 'approve: 予約キー・report.json の pr・findings.json・検査・委譲が揃う（記録に通した checks の名前）'

# 5. --dry-run は書かない
rm -f "$rec" "$tmp/gh/posted"
got=$(gate "$id" --dry-run); expect 0 approve
[ ! -e "$rec" ] && [ ! -e "$tmp/gh/posted" ] || die 'dry-run が書いた'
ok '--dry-run は記録も GitHub への書き込みもしない'

# 5b. GitHub の checks（PR の head の statusCheckRollup。ゲート自身の andon/gate は数えない）
rm -f "$rec" "$tmp/gh/posted"
rollup COMPLETED/SUCCESS IN_PROGRESS
got=$(gate "$id"); expect 5 wait 'まだ走っている: test'
jq -e '.github_checks == {state: "pending", names: ["test"]} and .checks == []' "$tmp/out" >/dev/null || die 'wait の出力の形が違う・検査を回した'
emit "$tmp/out" gate-record-wait.json
[ ! -e "$rec" ] && [ ! -e "$tmp/gh/posted" ] || die 'wait なのに記録・GitHub へ書いた'
ok 'GitHub の checks が走っていれば wait（終了コード 5。記録も GitHub への書き込みもしない・検査を回さない）'
rollup COMPLETED/SUCCESS COMPLETED/FAILURE
got=$(gate "$id" --dry-run); expect 1 fix 'GitHub の checks が落ちた: test'
jq -e '.github_checks.state == "failure" and .checks == []' "$tmp/out" >/dev/null || die 'failure の出力の形が違う'
rollup ctx:PENDING
got=$(gate "$id" --dry-run); expect 5 wait 'まだ走っている: ci/x'
rollup COMPLETED/ACTION_REQUIRED
got=$(gate "$id" --dry-run); expect 4 needs_human '決められない'
ok 'GitHub の checks が落ちれば fix・commit status の pending は wait・action_required は needs_human'
# 100 件を超える checks は全ページ読む（後のページだけの failure を見落とさない）
jq -n '[range(150) | {__typename: "CheckRun", name: "job\(.)", status: "COMPLETED", conclusion: (if . == 120 then "FAILURE" else "SUCCESS" end), databaseId: .}]' > "$tmp/gh/rollup.json"
rm -f "$tmp/gh/pages"
got=$(gate "$id" --dry-run); expect 1 fix 'GitHub の checks が落ちた: job120'
grep -qx 'graphql 100' "$tmp/gh/pages" || die '2ページ目を読んでいない'
# 同じ check の再実行は最新（databaseId が大きい）だけを数える。出てくる順に依らない
rerun() { jq -n --arg a "$1" --arg b "$2" '[{__typename: "CheckRun", name: "test", status: "COMPLETED", conclusion: $a, databaseId: 1,
  checkSuite: {app: {slug: "github-actions"}, workflowRun: {workflow: {id: "W_tests"}}}},
  {__typename: "CheckRun", name: "test", status: "COMPLETED", conclusion: $b, databaseId: 2,
  checkSuite: {app: {slug: "github-actions"}, workflowRun: {workflow: {id: "W_tests"}}}}] | if $ARGS.named.rev == "1" then reverse else . end' --arg rev "${3:-0}" > "$tmp/gh/rollup.json"; }
rerun FAILURE SUCCESS;   got=$(gate "$id" --dry-run); expect 0 approve
rerun FAILURE SUCCESS 1; got=$(gate "$id" --dry-run); expect 0 approve
rerun SUCCESS FAILURE;   got=$(gate "$id" --dry-run); expect 1 fix 'GitHub の checks が落ちた: test'
rerun SUCCESS FAILURE 1; got=$(gate "$id" --dry-run); expect 1 fix 'GitHub の checks が落ちた: test'
# 表示名が同じ別ファイルの workflow（id が違う）の同じ名前の check は束ねない
jq -n '[{__typename: "CheckRun", name: "test", status: "COMPLETED", conclusion: "FAILURE", databaseId: 1,
  checkSuite: {app: {slug: "github-actions"}, workflowRun: {workflow: {id: "W_a"}}}},
  {__typename: "CheckRun", name: "test", status: "COMPLETED", conclusion: "SUCCESS", databaseId: 2,
  checkSuite: {app: {slug: "github-actions"}, workflowRun: {workflow: {id: "W_b"}}}}]' > "$tmp/gh/rollup.json"
got=$(gate "$id" --dry-run); expect 1 fix 'GitHub の checks が落ちた: test'
ok 'GitHub の checks は100件を超えても全ページ読み、同じ check の再実行は最新だけを数える（順に依らない。workflow は表示名でなく id で分ける）'
rollup
got=$(gate "$id" --dry-run); expect 5 wait 'まだ1つも無い'
jq -e '.github_checks == {state: "none", names: []}' "$tmp/out" >/dev/null || die 'none の形が違う（andon/gate を数えた）'
log=kanban/log.jsonl; cp "$log" "$tmp/log.bak"
jq -c --arg id "$id" 'if .id == $id and .change.status == "review" then .at = "2020-01-01T00:00:00Z" else . end' "$tmp/log.bak" > "$log"
got=$(gate "$id" --dry-run); expect 4 needs_human '1つも無いまま 30 分'
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"], "github_checks": "optional"}}}' > "$tmp/plant/gate.local.json"
as_owner git -C "$tmp/plant" commit -qam 'CI の無い repo'
got=$(gate "$id" --dry-run); expect 0 approve
rollup COMPLETED/FAILURE
got=$(gate "$id" --dry-run); expect 1 fix 'GitHub の checks が落ちた'
cp "$tmp/log.bak" "$log"
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
as_owner git -C "$tmp/plant" commit -qam 'checks を要るに戻す'
rollup COMPLETED/SUCCESS
got=$(gate "$id" --dry-run); expect 0 approve
ok 'checks が1つも無ければ wait、review から github_checks_wait_minutes を過ぎたら needs_human。optional の repo は無くても通し、在る checks は見る'

# 6. 委譲を最後に変えたのが owner でなければ無委譲
jq '.delegation = {"version": 1, "project": "demo", "preset": "hobby"}' project.json > "$tmp/p.json" && cp "$tmp/p.json" project.json
as_agent git -C "$tmp/plant" commit -qm 'agent が書いた' -- projects/demo/project.json
got=$(gate "$id" --dry-run); expect 4 needs_human 'merge=none'
as_owner git -C "$tmp/plant" revert --no-edit HEAD >/dev/null
ok '委譲の commit の author が owner でなければ無委譲'
as_owner "$andon" delegate revoke d1 --reason 'やめる' >/dev/null
got=$(gate "$id" --dry-run); expect 0 approve
"$andon" delegate show --json | jq -e '.effective.merge == "B" and .grants == []' >/dev/null
jq -e '.delegation.grants[0].revoked.reason == "やめる"' project.json >/dev/null
ok 'delegate revoke: 行を残して取り消す（型 work の merge B に戻る）'

# 7. 検査が落ちれば fix（gate.local.json はセルの repo の HEAD で、owner が commit したものだけ）
set_check() { echo "{\"repos\": {\"github.com/example/app\": {\"merge\": \"squash\", \"check\": [\"$1\"]}}}" > "$tmp/plant/gate.local.json"; }
set_check 'test -f nothing'
got=$(gate "$id" --dry-run); expect 0 approve
as_owner git -C "$tmp/plant" commit -qam 'check を変える'
got=$(gate "$id" --dry-run); expect 1 fix 'check: test -f nothing'
set_check 'true'; as_agent git -C "$tmp/plant" commit -qam 'agent が検査を緩めた'
got=$(gate "$id" --dry-run); expect 4 needs_human '条件表の repos'
grep -q 'gate.local.json を重ねない' "$tmp/err" || die '信じない理由が出ない'
set_check 'test -f docs/a.md'; as_owner git -C "$tmp/plant" commit -qam 'check を戻す'
got=$(gate "$id" --dry-run); expect 0 approve
ok 'gate.local.json は作業ツリーの変更を読まず、owner でない commit は重ねない'
ok '条件表の検査が PR の head で落ちれば fix'

# 8. 予約キーを書いたのが Chief of Staff でなければ needs_human
HC_TERMINAL_ID=t7 kanban set "$id" must_fix=trivial >/dev/null
got=$(gate "$id" --dry-run); expect 4 needs_human 'Chief of Staff'
HC_TERMINAL_ID=t5 kanban set "$id" must_fix=major >/dev/null
ok '予約キーを最後に書いたのが Chief of Staff でなければ needs_human'

# 9. Must fix が残れば fix、同じ head で2度目は needs_human
findings "$head" '[{"severity":"major","file":"docs/a.md","line":1},{"severity":"minor","file":"docs/a.md","line":2}]'
got=$(gate "$id"); expect 1 fix 'Must fix 以上が 1 件'
jq -e '.review.must_fix_open == 1' "$rec" >/dev/null
got=$(gate "$id"); expect 4 needs_human '2度目'
ok 'Must fix が残れば fix、同じ head で2度目は needs_human'

# 9b. preexisting: true の指摘は Must fix に数えない。ただし PR の差分で足した行に当たれば数える（申告でゲートを緩めない）
printf 'diff --git a/docs/a.md b/docs/a.md\n--- a/docs/a.md\n+++ b/docs/a.md\n@@ -1,0 +2 @@\n+x\n' > "$tmp/gh/diff"
findings "$head" '[{"severity":"major","file":"docs/a.md","line":1,"preexisting":true},{"severity":"critical","file":"docs/old.md","line":3,"preexisting":true}]'
got=$(gate "$id" --dry-run); expect 0 approve
jq -e '.review.must_fix_open == 0' "$tmp/out" >/dev/null || die 'preexisting を数えた'
findings "$head" '[{"severity":"major","file":"docs/a.md","line":2,"preexisting":true}]'
got=$(gate "$id" --dry-run); expect 1 fix 'Must fix 以上が 1 件'
printf '+++ b/docs/a.md\n+x\n' > "$tmp/gh/diff"
ok 'preexisting: true の Must fix 以上だけなら fix にしない（PR が足した行の指摘は preexisting でも数える）'

# 9c. 2回目以降のレビューは直しの確認だけ: 前の回の findings.json に同じ file の指摘が無い critical でない指摘は数えない
findings "$(git -C "$tmp/origin" rev-parse HEAD~1)" '[{"severity":"major","file":"docs/a.md","line":1}]'
rev1=$rev
rev=$(HC_TERMINAL_ID=t5 kanban add "$id のレビュー（2 回目）" --parent "$id" --profile codex-review)
[ "$rev" != "$rev1" ] || die '2回目のレビュアーの課題が足せない'
mkdir -p "kanban/issues/$rev/library"
findings "$head" '[{"severity":"major","file":"docs/new.md","line":3}]'
HC_TERMINAL_ID=t5 kanban review "$rev" >/dev/null
got=$(gate "$id" --dry-run); expect 0 approve
jq -e --arg r "$rev" '.review.issue == $r and .review.must_fix_open == 0' "$tmp/out" >/dev/null || die "2回目の新しい major を数えた: $(cat "$tmp/out")"
findings "$head" '[{"severity":"critical","file":"docs/new.md","line":3}]'
got=$(gate "$id" --dry-run); expect 1 fix 'Must fix 以上が 1 件'
findings "$head" '[{"severity":"major","file":"docs/a.md","line":9}]'
got=$(gate "$id" --dry-run); expect 1 fix 'Must fix 以上が 1 件'
findings "$head" '[]'
ok '2回目のレビューで新しい major だけなら fix にしない（critical と、前の回と同じ file の指摘は数える）'

# 10. head が進んでレビューが古ければ stale で fix
pr docs/a.md 4 y
export HC_TERMINAL_ID=t7
"$andon" issue report "$id" <<<'## Report' >/dev/null
unset HC_TERMINAL_ID
got=$(gate "$id"); expect 1 fix 'review stale'
jq -e --arg c "$(jq -r .commit "kanban/issues/$rev/library/findings.json")" '.review.state == "stale" and .review.commit == $c' "$lib/gate/$head.json" >/dev/null
emit "$lib/gate/$head.json" gate-record-stale.json
ok 'レビューが古い commit を見ていれば stale で fix（記録の形）'

# 11. レベル C の変更は委譲 merge B の外
pr package/gate.json 1
export HC_TERMINAL_ID=t7
"$andon" issue report "$id" <<<'## Report' >/dev/null
unset HC_TERMINAL_ID
findings "$head"
got=$(gate "$id" --dry-run); expect 4 needs_human 'delegation: merge=B < tier C'
jq -e '.tier == "C" and (.tier_reasons | any(contains("package/gate.json")))' "$tmp/out" >/dev/null
ok 'レベル C の変更は委譲 merge B の外で needs_human'

# 11b. screen_paths（ADR-0018 受け入れ条件 3）: 当たる差分の PR に GitHub の添付が無ければ fix。本文かコメントに添付があれば通る
echo '{"screen_paths": ["web/**"], "repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
as_owner git -C "$tmp/plant" commit -qam 'screen_paths で画面を指す'
pr web/a.css 1
export HC_TERMINAL_ID=t7
"$andon" issue report "$id" <<<'## Report' >/dev/null
unset HC_TERMINAL_ID
findings "$head"
got=$(gate "$id" --dry-run); expect 1 fix 'screen_paths: web/a.css に当たるが画面の証拠（添付）が無い'
jq -e '.tier == "B" and .github_checks.state == "success"' "$tmp/out" >/dev/null || die "添付の無い fix の記録の形が違う: $(cat "$tmp/out")"
jq '.body = "![x](./a.png)" | .comments = [{body: "andon gate: **fix**"}]' "$tmp/gh/view.json" > "$tmp/v.json" && cp "$tmp/v.json" "$tmp/gh/view.json"
got=$(gate "$id" --dry-run); expect 1 fix 'screen_paths: web/a.css'
jq '.comments += [{body: "![after](https://github.com/user-attachments/assets/0a1b2c3d-4e5f-6789-abcd-ef0123456789)"}]' "$tmp/gh/view.json" > "$tmp/v.json" && cp "$tmp/v.json" "$tmp/gh/view.json"
got=$(gate "$id" --dry-run); expect 0 approve
jq '.comments = [] | .body = "## Summary\n\n![after](https://github.com/user-attachments/assets/0a1b2c3d-4e5f-6789-abcd-ef0123456789)"' "$tmp/gh/view.json" > "$tmp/v.json" && cp "$tmp/v.json" "$tmp/gh/view.json"
got=$(gate "$id" --dry-run); expect 0 approve
pr docs/b.md 1
export HC_TERMINAL_ID=t7
"$andon" issue report "$id" <<<'## Report' >/dev/null
unset HC_TERMINAL_ID
findings "$head"
got=$(gate "$id" --dry-run); expect 0 approve
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
as_owner git -C "$tmp/plant" commit -qam 'screen_paths を戻す'
ok 'screen_paths に当たる差分は PR の本文かコメントに GitHub の添付（user-attachments）が無ければ fix、あれば通す。当たらない差分は要らない'

# 12. deliverable: candidates（調べる課題）にはゲートが無い: 理由1行と終了コード 3、記録も GitHub への書き込みもしない（ADR-0006 受け入れ条件 4）
sid=$(HC_TERMINAL_ID=t5 kanban add 'docs の誤りを調べる' --profile claude)
HC_TERMINAL_ID=t5 kanban set "$sid" deliverable=candidates branch=survey "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
rm -f "$tmp/gh/posted"
got=$(gate "$sid")
[ "$got" = 3 ] && [ ! -s "$tmp/out" ] && [ "$(wc -l < "$tmp/err")" = 1 ] && grep -q 'deliverable: candidates' "$tmp/err" || { cat "$tmp/out" "$tmp/err" >&2; die "candidates に rc $got"; }
[ ! -e "kanban/issues/$sid/library/gate" ] && [ ! -e "$tmp/gh/posted" ] || die 'candidates なのに記録・GitHub に書いた'
# 最後に書いたのが Chief of Staff でなければ pr として読む（今までどおり report.json を見に行く）
HC_TERMINAL_ID=t7 kanban set "$sid" deliverable=candidates >/dev/null
got=$(gate "$sid"); expect 1 fix 'report.json'
ok 'deliverable: candidates は回さず終了コード 3（記録しない）。Chief of Staff が書いたものでなければ pr として読む'

echo "all passed ($pass)"
