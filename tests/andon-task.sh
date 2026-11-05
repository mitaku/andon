#!/usr/bin/env bash
# `andon task show/report/review-request/ready/ask`。本物の台帳の CLI（kanban）・一時の台帳と git・偽の gh で回す。
# ANDON_EMIT_DIR があれば、command が書いた JSON をそこに写す（tests/schema.sh が schema に通す）。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset FLIGHTDECK_ROOT
mkdir -p "$HOME" "$tmp/demo" "$tmp/bin"
# 偽の gh: pr view <url> --json headRefOid -q .headRefOid → $tmp/prhead
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
[ "\$1 \$2" = "pr view" ] && [ "\$4 \$5" = "--json headRefOid" ] || { echo "unexpected gh \$*" >&2; exit 99; }
cat "$tmp/prhead"
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"

pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }
# expect_fail <理由に含む語> <command…>: 終了コード 1 で、stderr に語がある
expect_fail() {
    local want=$1; shift
    local rc=0; "$@" >"$tmp/out" 2>"$tmp/err" || rc=$?
    [ "$rc" = 1 ] && grep -q -- "$want" "$tmp/err" || { cat "$tmp/err" >&2; die "want rc 1 with '$want', got rc $rc: $*"; }
}
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }

# 担当の worktree
git init -q "$tmp/wt"
commit() { echo "$1" > "$tmp/wt/f"; git -C "$tmp/wt" add f; git -C "$tmp/wt" commit -qm "$1"; git -C "$tmp/wt" rev-parse HEAD; }
c1=$(commit one)
echo "$c1" > "$tmp/prhead"

cd "$tmp/demo"
kanban init --prefix T >/dev/null
export FLIGHTDECK_ROOT=$PWD
kanban config set coordinator=t5 >/dev/null
as() { HC_TERMINAL_ID=$1 "${@:2}"; }   # 台帳の by を選ぶ（t5 が Chief of Staff、t7 が担当）
id=$(as t5 kanban add 'andon task を足す' --profile claude)
as t5 kanban set "$id" review=on must_fix=major "worktree=$tmp/wt" >/dev/null
lib=.flightdeck/tasks/$id/library
pr=https://github.com/example/app/pull/7
export HC_TERMINAL_ID=t7

# report
expect_fail 'Markdown' "$andon" task report "$id" --pr "$pr" </dev/null
ok 'report: 人用の報告が無ければ止まる'
expect_fail 'commit・review_request' "$andon" task report "$id" --data '{"commit":"x"}' <<<'## Report'
expect_fail '--data が読めない' "$andon" task report "$id" --data '{"title":"x"}' <<<'## Report'
expect_fail 'next は' "$andon" task report "$id" --data '{"next":[{"to":"human","text":"x"}]}' <<<'## Report'
[ ! -e "$lib/report.json" ] || die 'report.json が書かれた'
ok 'report: --data の知らないキー・command のキー・知らない宛先は書く前に止まる'
"$andon" task report "$id" --pr "$pr" --data '{"next":[{"to":"chief-of-staff","text":"マージ後に片付ける"}],"remember":["x"]}' <<<'## Report' >/dev/null
jq -e --arg c "$c1" --arg pr "$pr" '.commit == $c and .pr == $pr and .next[0].to == "chief-of-staff" and (has("review_request") | not)' "$lib/report.json" >/dev/null
grep -q '## Report' ".flightdeck/tasks/$id/report.md"
emit "$lib/report.json" report-1.json
ok 'report: commit は HEAD、pr と --data を書き、台帳に報告する'
"$andon" task report "$id" <<<'## Report' >/dev/null
jq -e --arg pr "$pr" '.pr == $pr and (has("next") | not)' "$lib/report.json" >/dev/null
ok 'report: --pr を省くと前の pr を引き継ぎ、ほかは書き直す'

# review-request（1 回目）
"$andon" task review-request "$id" <<<'## Report' >/dev/null
jq -e --arg c "$c1" '.review_request == {round: 1, limit: 2, commit: $c, previous: null, reviewer: "codex-review", extra: false}' "$lib/report.json" >/dev/null
head -2 ".flightdeck/tasks/$id/report.md" | grep -q "^1 回目／上限 2。commit $c1"
emit "$lib/report.json" report-review-request.json
ok 'review-request: 1 回目・前回 null・reviewer は実装と別ブランド・人用の行を先頭に'

# ready の負: レビューが無い
expect_fail 'レビューが missing' "$andon" task ready "$id"
ok 'ready: レビューが無ければ止まる'

# レビュアー（Chief of Staff が足す）。commit c1 を見た
rev=$(as t5 kanban add "$id のレビュー" --parent "$id" --profile codex-review)
mkdir -p ".flightdeck/tasks/$rev/library"
jq -n --arg c "$c1" '{commit: $c, findings: [{severity: "minor", file: "f", line: 1}]}' > ".flightdeck/tasks/$rev/library/findings.json"
as t5 kanban review "$rev" >/dev/null

# HEAD を進める（直した）。report.json の commit は古い
c2=$(commit two)
expect_fail 'report.json の commit' "$andon" task ready "$id"
ok 'ready: report.json の commit が HEAD と違えば止まる（HEAD 不一致）'
"$andon" task report "$id" <<<'## Report' >/dev/null
expect_fail 'PR の head' "$andon" task ready "$id"
ok 'ready: PR の head が HEAD と違えば止まる（push 前）'
echo "$c2" > "$tmp/prhead"
expect_fail 'レビューが stale' "$andon" task ready "$id"
ok 'ready: レビューが古い commit を見ていれば止まる（stale）'

# review-request（2 回目）: 前回は c1
"$andon" task review-request "$id" <<<'## Report' >/dev/null
jq -e --arg c "$c2" --arg p "$c1" '.review_request | .round == 2 and .commit == $c and .previous == $p and .extra == false' "$lib/report.json" >/dev/null
ok 'review-request: 2 回目・previous は前のレビューの commit'

# 担当が自分で足した子タスクは数えない
fake=$(kanban add "$id の自作レビュー" --parent "$id" --profile codex-review)
mkdir -p ".flightdeck/tasks/$fake/library"
jq -n --arg c "$c2" '{commit: $c, findings: []}' > ".flightdeck/tasks/$fake/library/findings.json"
kanban review "$fake" >/dev/null
expect_fail 'レビューが stale' "$andon" task ready "$id"
ok 'ready: Chief of Staff が足していないレビューは数えない'

# 形の壊れた findings.json は missing
rev2=$(as t5 kanban add "$id のレビュー 2" --parent "$id" --profile codex-review)
mkdir -p ".flightdeck/tasks/$rev2/library"
jq -n --arg c "$c2" '{commit: $c, findings: [{severity: "Major", file: "f", line: 1}]}' > ".flightdeck/tasks/$rev2/library/findings.json"
as t5 kanban review "$rev2" >/dev/null
expect_fail 'findings.json が無い・形が違う' "$andon" task ready "$id"
ok 'ready: findings.json の形が違えば missing'
jq -n --arg c "$c2" '{commit: $c, findings: [{severity: "minor", file: "f", line: 2}]}' > ".flightdeck/tasks/$rev2/library/findings.json"

# HEAD のレビューがもうあれば頼まない
expect_fail 'レビューはもうある' "$andon" task review-request "$id" <<<'## Report'
ok 'review-request: HEAD のレビューがあれば頼まない'

# report.json の形が壊れていれば止まる
cp "$lib/report.json" "$tmp/good"
jq '. + {"x_unknown": 1}' "$tmp/good" > "$lib/report.json"
expect_fail 'report.json の形が違う' "$andon" task ready "$id"
jq '.commit = "abc"' "$tmp/good" > "$lib/report.json"
expect_fail 'report.json の形が違う' "$andon" task ready "$id"
jq 'del(.pr)' "$tmp/good" > "$lib/report.json"
expect_fail 'pr が無い' "$andon" task ready "$id"
ok 'ready: report.json の知らないキー・壊れた commit・pr 無しで止まる'
cp "$tmp/good" "$lib/report.json"

# show
"$andon" task show "$id" > "$tmp/show"
jq -e --arg rev "$rev2" --arg c "$c2" '.settings == {review: "on", must_fix: "major", reviewer: "codex-review", review_rounds: "2", merge: "person"}
  and .review == {state: "current", task: $rev, commit: $c, must_fix_open: 0, waived: 0} and .report.commit == $c' "$tmp/show" >/dev/null
jq .settings "$tmp/show" > "$tmp/settings.json"; emit "$tmp/settings.json" task-keys.json
ok 'show: 予約キー（既定で埋める）・report.json・レビューの照合'

# ready の正
"$andon" task ready "$id" >/dev/null
[ "$(kanban show "$id" --json | jq -r .task.status)" = review ]
ok 'ready: HEAD ＝ report.json ＝ PR の head ＝ レビューの commit で review にする'

# review: off なら照合しない。review が無ければ止まる
id2=$(as t5 kanban add 'レビュー無し' --profile codex)
as t5 kanban set "$id2" review=off "worktree=$tmp/wt" >/dev/null
"$andon" task report "$id2" --pr "$pr" <<<'## Report' >/dev/null
expect_fail 'review が on でない' "$andon" task review-request "$id2" <<<'## Report'
"$andon" task ready "$id2" >/dev/null
ok 'ready: review: off ならレビューを照合しない。review-request は断る'
id3=$(as t5 kanban add 'Settings 無し')
as t5 kanban set "$id3" "worktree=$tmp/wt" review_rounds=1 >/dev/null
"$andon" task report "$id3" --pr "$pr" <<<'## Report' >/dev/null
expect_fail 'review が on/off でない' "$andon" task ready "$id3"
ok 'ready: task.yaml の review が無ければ止まる'
as t5 kanban set "$id3" review=on >/dev/null
r3=$(as t5 kanban add "$id3 のレビュー" --parent "$id3" --profile codex-review)
mkdir -p ".flightdeck/tasks/$r3/library"; jq -n --arg c "$c1" '{commit: $c, findings: []}' > ".flightdeck/tasks/$r3/library/findings.json"
"$andon" task review-request "$id3" <<<'## Report' >/dev/null
jq -e '.review_request | .round == 2 and .limit == 1 and .extra == true' ".flightdeck/tasks/$id3/library/report.json" >/dev/null
head -2 ".flightdeck/tasks/$id3/report.md" | grep -q '（追加）'
ok 'review-request: 上限を超えた回は extra'

# ask
q='{"kind":"users","question":"既定を変えるか","facts":["docs/design.md §1.10"],"options":[{"label":"変える","pick":true},{"label":"変えない"}],"remaining":1,"from":"task"}'
"$andon" task ask "$id" - <<<"$q" | grep -q "$id#1"
jq -e '.kind == "users" and .options[0].pick == true' "$lib/questions/1.json" >/dev/null
kanban show "$id" --json | jq -e '.questions[0] | .kind == "users" and (.text | contains("推奨案: 変える")) and (.text | contains("あと 1 問"))' >/dev/null
emit "$lib/questions/1.json" question.json
ok 'ask: 問いを台帳に置き、JSON を library/questions/<n>.json に'
"$andon" task ask "$id" - <<<"$(jq -c '.kind = "line"' <<<"$q")" >/dev/null
kanban show "$id" --json | jq -e '.questions[1] | has("kind") | not' >/dev/null
jq -e '.kind == "line"' "$lib/questions/2.json" >/dev/null
ok 'ask: kind line は台帳に kind を付けず、JSON にだけ書く'
for bad in '.options[1].pick = true' '.options = [.options[0]]' '.question = "a\nb"' '. + {"x": 1}' '.kind = "other"'; do
    expect_fail '問いの JSON の形が違う' "$andon" task ask "$id" - <<<"$(jq -c "$bad" <<<"$q")"
done
[ "$(kanban show "$id" --json | jq '.questions | length')" = 2 ]
ok 'ask: 形の違う問いは台帳に書く前に止まる'

echo "all passed ($pass)"
