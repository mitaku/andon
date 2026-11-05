#!/usr/bin/env bash
# `andon gate` と `andon delegate`。本物の台帳の CLI（deck）と git・一時の建屋の repo・偽の gh で回す。
# PR の head は手元の origin から取る（git の insteadOf）。GitHub には出ない。台帳の CLI が無ければ skip。
# ANDON_EMIT_DIR があれば、ゲートの記録と委譲を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
for c in deck git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@example.com
export ANDON_HOME="$andon_root/package" ANDON_WORKSPACE="$tmp/plant" GHQ_ROOT="$tmp/ghq"
unset FLIGHTDECK_ROOT HC_TERMINAL_ID
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

# 偽の gh: PR の事実は $tmp/gh/ のファイル。書き込み（status・comment）は $tmp/gh/posted に1行ずつ
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
d=$tmp/gh
case "\$*" in
  "pr view 7 --repo example/app --json headRefOid -q .headRefOid") jq -r .headRefOid \$d/view.json ;;
  "pr view 7 --repo example/app --json "*) cat \$d/view.json ;;
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
# pr <path> <行数>: PR の head を進め、view・files・diff を作る
pr() {
    head=$(change "$1" "${3:-x}")
    jq -n --arg h "$head" '{state:"OPEN",isDraft:false,headRefName:"feat",headRefOid:$h,baseRefName:"main",
      mergeStateStatus:"CLEAN",author:{login:"plant-bot"},changedFiles:1,isCrossRepository:false}' > "$tmp/gh/view.json"
    jq -n --arg p "$1" --argjson n "$2" '[[{filename:$p,additions:$n,deletions:0}]]' > "$tmp/gh/files.json"
    printf '+++ b/%s\n+%s\n' "$1" "${3:-x}" > "$tmp/gh/diff"
    git -C "$tmp/wt" fetch -q origin && git -C "$tmp/wt" checkout -q "$head"
}

# 建屋の repo（人のもの）と台帳
git init -q "$tmp/plant"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
echo '{"repos": {"github.com/example/app": {"merge": "squash", "check": ["test -f docs/a.md"]}}}' > "$tmp/plant/gate.local.json"
as_owner git -C "$tmp/plant" add -A; as_owner git -C "$tmp/plant" commit -qm plant
mkdir -p "$tmp/plant/projects/demo"
cd "$tmp/plant/projects/demo"
deck init --prefix D --repo github.com/example/app >/dev/null
export FLIGHTDECK_ROOT=$PWD
deck config set coordinator=t5 >/dev/null
git clone -q "$tmp/origin" "$tmp/wt"
id=$(HC_TERMINAL_ID=t5 deck add 'docs を直す' --profile claude)
HC_TERMINAL_ID=t5 deck set "$id" review=on must_fix=major branch=feat "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
lib=.flightdeck/tasks/$id/library
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
"$andon" task report "$id" --pr https://github.com/example/app/pull/7 <<<'## Report' >/dev/null
rev=$(HC_TERMINAL_ID=t5 deck add "$id のレビュー" --parent "$id" --profile codex-review)
mkdir -p ".flightdeck/tasks/$rev/library"
findings() { jq -n --arg c "$1" --argjson f "${2:-[]}" '{commit: $c, findings: $f}' > ".flightdeck/tasks/$rev/library/findings.json"; }
findings "$head"
HC_TERMINAL_ID=t5 deck review "$rev" >/dev/null
"$andon" task ready "$id" >/dev/null
unset HC_TERMINAL_ID

# 2. 委譲が無ければ approve にならない（全部人）
got=$(gate "$id"); expect 4 needs_human 'delegation: merge=none < tier B'
rec="$lib/gate/$head.json"
jq -e '.tier == "B" and .delegation == {merge: "none", ok: false} and .review.state == "current" and .checks == [] and .merged == false' "$rec" >/dev/null
grep -q 'status .*state=failure' "$tmp/gh/posted" && grep -q comment "$tmp/gh/posted"
ok '委譲が無ければ needs_human（記録を書き、status と comment を付ける）'

# 3. andon delegate: 人が委譲を書く
echo '{"role":"chief-of-staff"}' > "$tmp/labels"
rc=0; HC_TERMINAL_ID=t5 "$andon" delegate grant merge B --evidence x --project demo >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'role=chief-of-staff' "$tmp/err" && [ ! -e delegation.json ] || die 'Chief of Staff の端末で書けた'
ok 'delegate: Chief of Staff・担当の端末（role= / task=）では打てない'
as_owner "$andon" delegate grant merge B --evidence '2026-11-05 の対話' >/dev/null
jq -e '.version == 1 and .project == "demo" and .grants[0].id == "d1" and .grants[0].value == "B" and .grants[0].until == null' delegation.json >/dev/null
grep -q '| `merge` | B | d1 |' delegation.md
git -C "$tmp/plant" log -1 --format=%B | grep -q 'evidence: 2026-11-05 の対話'
emit delegation.json delegation.json
ok 'delegate grant: JSON を書き、delegation.md を生成し、根拠つきで commit する（cwd から slug）'
rc=0; as_owner "$andon" delegate narrow merge C --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '広げない' "$tmp/err" || die 'narrow で広げられた'
rc=0; as_owner "$andon" delegate grant irreversible delete-remote --evidence x >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'delete-remote は委譲できない' "$tmp/err" || die 'delete-remote を委譲できた'
mkdir -p "$tmp/plant/projects/andon"
rc=0; as_owner "$andon" delegate grant merge C --evidence x --project andon >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'B まで' "$tmp/err" || die 'andon プロジェクトに merge C を委譲できた'
ok 'delegate: narrow は広げない・delete-remote と andon の merge C は断る'
"$andon" delegate show --json | jq -e '.trusted and .preset == "work" and .effective.merge == "B" and .effective.deploy == "none"' >/dev/null
ok 'delegate show: HEAD の委譲を読み手と同じに読む'

# 4. 委譲の範囲内で揃えば approve
got=$(gate "$id"); expect 0 approve
jq -e '.delegation == {merge: "B", ok: true} and .checks == [{cmd: "test -f docs/a.md", ok: true}] and .reasons == []' "$rec" >/dev/null
grep -q 'state=success' "$tmp/gh/posted"
emit "$rec" gate-record.json
ok 'approve: 予約キー・report.json の pr・findings.json・検査・委譲が揃う'

# 5. --dry-run は書かない
rm -f "$rec" "$tmp/gh/posted"
got=$(gate "$id" --dry-run); expect 0 approve
[ ! -e "$rec" ] && [ ! -e "$tmp/gh/posted" ] || die 'dry-run が書いた'
ok '--dry-run は記録も GitHub への書き込みもしない'

# 6. 委譲を最後に変えたのが owner でなければ無委譲
echo '{"version": 1, "project": "demo", "preset": "hobby"}' > delegation.json
as_agent git -C "$tmp/plant" commit -qam 'agent が書いた'
got=$(gate "$id" --dry-run); expect 4 needs_human 'merge=none'
as_owner git -C "$tmp/plant" revert --no-edit HEAD >/dev/null
ok '委譲の commit の author が owner でなければ無委譲'
as_owner "$andon" delegate revoke d1 --reason 'やめる' >/dev/null
got=$(gate "$id" --dry-run); expect 0 approve
"$andon" delegate show --json | jq -e '.effective.merge == "B" and .grants == []' >/dev/null
jq -e '.grants[0].revoked.reason == "やめる"' delegation.json >/dev/null
ok 'delegate revoke: 行を残して取り消す（型 work の merge B に戻る）'

# 7. 検査が落ちれば fix（gate.local.json は建屋の repo の HEAD で、owner が commit したものだけ）
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
HC_TERMINAL_ID=t7 deck set "$id" must_fix=trivial >/dev/null
got=$(gate "$id" --dry-run); expect 4 needs_human 'Chief of Staff'
HC_TERMINAL_ID=t5 deck set "$id" must_fix=major >/dev/null
ok '予約キーを最後に書いたのが Chief of Staff でなければ needs_human'

# 9. Must fix が残れば fix、同じ head で2度目は needs_human
findings "$head" '[{"severity":"major","file":"docs/a.md","line":1},{"severity":"minor","file":"docs/a.md","line":2}]'
got=$(gate "$id"); expect 1 fix 'Must fix 以上が 1 件'
jq -e '.review.must_fix_open == 1' "$rec" >/dev/null
got=$(gate "$id"); expect 4 needs_human '2度目'
ok 'Must fix が残れば fix、同じ head で2度目は needs_human'

# 10. head が進んでレビューが古ければ stale で fix
pr docs/a.md 4 y
export HC_TERMINAL_ID=t7
"$andon" task report "$id" <<<'## Report' >/dev/null
unset HC_TERMINAL_ID
got=$(gate "$id"); expect 1 fix 'review stale'
jq -e --arg c "$(jq -r .commit ".flightdeck/tasks/$rev/library/findings.json")" '.review.state == "stale" and .review.commit == $c' "$lib/gate/$head.json" >/dev/null
emit "$lib/gate/$head.json" gate-record-stale.json
ok 'レビューが古い commit を見ていれば stale で fix（記録の形）'

# 11. レベル C の変更は委譲 merge B の外
pr package/gate.json 1
export HC_TERMINAL_ID=t7
"$andon" task report "$id" <<<'## Report' >/dev/null
unset HC_TERMINAL_ID
findings "$head"
got=$(gate "$id" --dry-run); expect 4 needs_human 'delegation: merge=B < tier C'
jq -e '.tier == "C" and (.tier_reasons | any(contains("package/gate.json")))' "$tmp/out" >/dev/null
ok 'レベル C の変更は委譲 merge B の外で needs_human'

echo "all passed ($pass)"
