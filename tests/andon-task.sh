#!/usr/bin/env bash
# `andon task show/report/review-request/ready/ask/start`。本物の台帳の CLI（kanban）・一時の台帳と git・偽の gh・偽の hc で回す。
# start は bare な origin とその clone（GHQ_ROOT）に worktree を切り、偽の hc で起こし、finish で片付くまで。本物の hc・台帳には触らない。
# ANDON_EMIT_DIR があれば、command が書いた JSON をそこに写す（tests/schema.sh が schema に通す）。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT   # Go が落とす toolchain は読むだけで置かれる
# helper の go build が本物の HOME の Go のキャッシュに書かないように、helper を読む前に一時の HOME とキャッシュへ向ける
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GOCACHE="$tmp/gocache" GOMODCACHE="$tmp/gomodcache" GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
mkdir -p "$HOME"
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
# helper が go build したなら、キャッシュは一時ディレクトリにある
if [ -z "${ANDON_BIN:-}" ] && [ -z "${KANBAN_BIN:-}" ]; then
  [ -n "$(ls -A "$tmp/gocache" 2>/dev/null)" ] || { echo "FAIL go のキャッシュが一時ディレクトリに無い" >&2; exit 1; }
  echo "ok   go build のキャッシュは一時ディレクトリ（$tmp/gocache）"
fi
export GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset KANBAN_ROOT
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
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t5 >/dev/null
as() { HC_TERMINAL_ID=$1 "${@:2}"; }   # 台帳の by を選ぶ（t5 が Chief of Staff、t7 が担当）
id=$(as t5 kanban add 'andon task を足す' --profile claude)
as t5 kanban set "$id" review=on must_fix=major "worktree=$tmp/wt" >/dev/null
lib=kanban/tasks/$id/library
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
grep -q '## Report' "kanban/tasks/$id/report.md"
emit "$lib/report.json" report-1.json
ok 'report: commit は HEAD、pr と --data を書き、台帳に報告する'
"$andon" task report "$id" <<<'## Report' >/dev/null
jq -e --arg pr "$pr" '.pr == $pr and (has("next") | not)' "$lib/report.json" >/dev/null
ok 'report: --pr を省くと前の pr を引き継ぎ、ほかは書き直す'

# review-request（1 回目）
"$andon" task review-request "$id" <<<'## Report' >/dev/null
jq -e --arg c "$c1" '.review_request == {round: 1, limit: 2, commit: $c, previous: null, reviewer: "codex-review", extra: false}' "$lib/report.json" >/dev/null
head -2 "kanban/tasks/$id/report.md" | grep -q "^1 回目／上限 2。commit $c1"
emit "$lib/report.json" report-review-request.json
ok 'review-request: 1 回目・前回 null・reviewer は実装と別ブランド・人用の行を先頭に'

# ready の負: レビューが無い
expect_fail 'レビューが missing' "$andon" task ready "$id"
ok 'ready: レビューが無ければ止まる'

# レビュアー（Chief of Staff が足す）。commit c1 を見た
rev=$(as t5 kanban add "$id のレビュー" --parent "$id" --profile codex-review)
mkdir -p "kanban/tasks/$rev/library"
jq -n --arg c "$c1" '{commit: $c, findings: [{severity: "minor", file: "f", line: 1}]}' > "kanban/tasks/$rev/library/findings.json"
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
mkdir -p "kanban/tasks/$fake/library"
jq -n --arg c "$c2" '{commit: $c, findings: []}' > "kanban/tasks/$fake/library/findings.json"
kanban review "$fake" >/dev/null
expect_fail 'レビューが stale' "$andon" task ready "$id"
ok 'ready: Chief of Staff が足していないレビューは数えない'

# 形の壊れた findings.json は missing
rev2=$(as t5 kanban add "$id のレビュー 2" --parent "$id" --profile codex-review)
mkdir -p "kanban/tasks/$rev2/library"
jq -n --arg c "$c2" '{commit: $c, findings: [{severity: "Major", file: "f", line: 1}]}' > "kanban/tasks/$rev2/library/findings.json"
as t5 kanban review "$rev2" >/dev/null
expect_fail 'findings.json が無い・形が違う' "$andon" task ready "$id"
ok 'ready: findings.json の形が違えば missing'
jq -n --arg c "$c2" '{commit: $c, findings: [{severity: "minor", file: "f", line: 2}]}' > "kanban/tasks/$rev2/library/findings.json"

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
jq -e --arg rev "$rev2" --arg c "$c2" '.settings == {review: "on", must_fix: "major", reviewer: "codex-review", review_rounds: "2", merge: "person", deliverable: "pr"}
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
mkdir -p "kanban/tasks/$r3/library"; jq -n --arg c "$c1" '{commit: $c, findings: []}' > "kanban/tasks/$r3/library/findings.json"
"$andon" task review-request "$id3" <<<'## Report' >/dev/null
jq -e '.review_request | .round == 2 and .limit == 1 and .extra == true' "kanban/tasks/$id3/library/report.json" >/dev/null
head -2 "kanban/tasks/$id3/report.md" | grep -q '（追加）'
ok 'review-request: 上限を超えた回は extra'

# ask
q='{"kind":"users","question":"既定を変えるか","facts":["docs/design.md §1.10"],"options":[{"label":"変える","pick":true},{"label":"変えない"}],"remaining":1,"from":"task"}'
"$andon" task ask "$id" - <<<"$q" | grep -q "$id#1"
jq -e '.kind == "users" and .options[0].pick == true' "$lib/questions/1.json" >/dev/null
kanban show "$id" --json | jq -e '.questions[0] | .kind == "users" and (.text | contains("推奨案: 変える")) and (.text | contains("あと 1 問"))' >/dev/null
emit "$lib/questions/1.json" question.json
ok 'ask: 問いを台帳に置き、JSON を library/questions/<n>.json に'
"$andon" task ask "$id" - <<<"$(jq -c '.kind = "line"' <<<"$q")" >/dev/null
kanban show "$id" --json | jq -e '.questions[1].kind == "line"' >/dev/null
jq -e '.kind == "line"' "$lib/questions/2.json" >/dev/null
ok 'ask: kind line も台帳に kind を付ける（kanban ask --kind line）'
for bad in '.options[1].pick = true' '.options = [.options[0]]' '.question = "a\nb"' '. + {"x": 1}' '.kind = "other"'; do
    expect_fail '問いの JSON の形が違う' "$andon" task ask "$id" - <<<"$(jq -c "$bad" <<<"$q")"
done
[ "$(kanban show "$id" --json | jq '.questions | length')" = 2 ]
ok 'ask: 形の違う問いは台帳に書く前に止まる'

# 候補・範囲の問い・調べるタスクの ready（ADR-0006 決めたこと 2・3・4・7。受け入れ条件 2）。worktree は別に切る
git init -q "$tmp/wt2"
commit2() { echo "$1" > "$tmp/wt2/f"; git -C "$tmp/wt2" add f; git -C "$tmp/wt2" commit -qm "$1"; git -C "$tmp/wt2" rev-parse HEAD; }
s1=$(commit2 one)
# 直すタスクが範囲で止まる: candidates → ask（scope）→ stop-check は通す
fx=$(as t5 kanban add '誤字を1つ直す' --profile claude)
as t5 kanban set "$fx" review=off must_fix=major "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
kanban start "$fx" >/dev/null
lfx=kanban/tasks/$fx/library
cands='{"candidates":[{"id":"c1","text":"docs/a.md:4 「建家」→「建屋」","file":"docs/a.md","line":4,"in_scope":true},{"id":"c2","text":"README の段を書き直す","in_scope":false}]}'
for bad in '.candidates[1].id = "c1"|重なっている' 'del(.candidates[0].in_scope)|in_scope が無い' '.candidates[0] += {"tier": "A"}|知らないキー' \
           '.candidates[0].text = "a\nb"|2行以上' '.candidates[0].file = "/etc/x"|相対パス' '.candidates[0].line = 0|line が' '.candidates[0].id = "x1"|c1, c2' \
           '. + {"commit": "'"$s1"'"}|commit は書かない' 'del(.candidates)|candidates が無い'; do
    expect_fail "${bad#*|}" "$andon" task candidates "$fx" - <<<"$(jq -c "${bad%|*}" <<<"$cands")"
    [ "$(wc -l < "$tmp/err")" = 1 ] || die "candidates: 理由が1行でない: $(cat "$tmp/err")"
done
[ ! -e "$lfx/candidates.json" ] || die 'candidates: 形の違う一覧を書いた'
ok 'candidates: 形が違えば書かずに終了コード 1 と理由1行（id の重複・in_scope 無し・知らないキー・commit を書く）'
"$andon" task candidates "$fx" - <<<"$cands" | grep -q '候補 2 件、範囲の中 1 件'
jq -e --arg c "$s1" '.commit == $c and (.candidates | length) == 2 and .candidates[0].file == "docs/a.md" and .candidates[1].in_scope == false and (.candidates[1] | has("file") or has("line") | not)' "$lfx/candidates.json" >/dev/null
emit "$lfx/candidates.json" candidates.json
printf '%s' "$cands" > "$tmp/cands.json"; "$andon" task candidates "$fx" "$tmp/cands.json" >/dev/null
ok 'candidates: 形を確かめて library/candidates.json に書く（commit は HEAD。ファイルからも）'
expect_fail '形が違う' "$andon" task candidates "$fx" - <<<'{"candidates":[{"id":"c1","text":"x","in_scope":true},{"id":"c1","text":"y","in_scope":true}]}'
jq -e '(.candidates | length) == 2 and .candidates[1].id == "c2"' "$lfx/candidates.json" >/dev/null || die 'candidates: 形の違う一覧で前の一覧を壊した'
ok 'candidates: 形が違えば前の candidates.json はそのまま'
sq=$(jq -nc --arg f "$lfx/candidates.json" '{kind: "scope", question: "誤字は1つではなく2つ。どれを直すか", facts: [$f, "依頼書: 誤字を1つ直す"],
  options: [{label: "c1 と c2 を全部この PR で", pick: true}, {label: "c1 だけ。c2 は別のタスク"}], from: "task"}')
"$andon" task ask "$fx" - <<<"$sq" | grep -q "$fx#1"
kanban show "$fx" --json | jq -e '.questions[0] | .kind == "scope" and .status == "open" and (.text | contains("（scope）"))' >/dev/null
jq -e '.kind == "scope"' "$lfx/questions/1.json" >/dev/null
emit "$lfx/questions/1.json" question-scope.json
ok 'ask: kind scope を受けて kanban ask --kind scope を打つ'
out=$("$andon" task stop-check "$fx" <<<'{"stop_hook_active":false}' 2>/dev/null)
[ -z "$out" ] || die "stop-check: scope の問いが開いたままのターンを止めた: $out"
out=$("$andon" task stop-check "$fx" <<<'{"stop_hook_active":false}' 2>/dev/null)
[ -z "$out" ] || die "stop-check: scope の問いの答えを待つターンを止めた: $out"
ok 'stop-check: scope の問いが開いたままのターンを通す'

# 調べるタスク（deliverable: candidates）の ready: PR 無しで candidates.json を要る
sv=$(as t5 kanban add '調べる: 古い command 名' --profile claude)
as t5 kanban set "$sv" deliverable=candidates review=off "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
kanban start "$sv" >/dev/null
lsv=kanban/tasks/$sv/library
"$andon" task report "$sv" --data '{"next":[{"to":"chief-of-staff","text":"candidates.json から切る"}]}' <<<'## Report' >/dev/null
expect_fail 'candidates.json が無い' "$andon" task ready "$sv"
ok 'ready（candidates）: candidates.json が無ければ止まる'
echo '{"candidates":[' > "$lsv/candidates.json"
expect_fail 'candidates.json の形が違う' "$andon" task ready "$sv"
ok 'ready（candidates）: 手で壊した candidates.json で止まる'
"$andon" task candidates "$sv" - <<<'{"candidates":[]}' >/dev/null
s2=$(commit2 two)
"$andon" task report "$sv" <<<'## Report' >/dev/null
expect_fail 'candidates.json の commit' "$andon" task ready "$sv"
ok 'ready（candidates）: candidates.json の commit が HEAD と違えば止まる'
"$andon" task candidates "$sv" - <<<'{"candidates":[]}' >/dev/null
"$andon" task show "$sv" > "$tmp/show-sv"
jq -e '.settings.deliverable == "candidates"' "$tmp/show-sv" >/dev/null
jq .settings "$tmp/show-sv" > "$tmp/settings-sv.json"; emit "$tmp/settings-sv.json" task-keys-candidates.json
"$andon" task ready "$sv" >/dev/null
[ "$(kanban show "$sv" --json | jq -r .task.status)" = review ] || die 'ready（candidates）: review にならない'
ok 'ready（candidates）: PR 無しで、候補 0 件の candidates.json と HEAD が揃えば review にする（show の settings に deliverable）'
# Chief of Staff でない書き手の deliverable は信じず pr として読む（PR を要る）
sv2=$(as t5 kanban add '調べる: 担当が自分で型を変える' --profile claude)
as t5 kanban set "$sv2" review=off "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
as t7 kanban set "$sv2" deliverable=candidates >/dev/null
kanban start "$sv2" >/dev/null
"$andon" task candidates "$sv2" - <<<'{"candidates":[]}' >/dev/null
"$andon" task report "$sv2" <<<'## Report' >/dev/null
expect_fail 'pr が無い' "$andon" task ready "$sv2"
ok 'ready: deliverable を最後に書いたのが Chief of Staff でなければ pr として PR を要る'
# repo の無い調べるタスク: commit を書かず、HEAD を照合しない。cwd は git の外（start と同じ tasks/<id>/）で、candidates → report → ready まで通る
nr=$(as t5 kanban add '調べる: repo 無し' --profile claude)
as t5 kanban set "$nr" deliverable=candidates review=off >/dev/null
kanban start "$nr" >/dev/null
pushd "kanban/tasks/$nr" >/dev/null
! git rev-parse HEAD >/dev/null 2>&1 || die 'repo 無し: cwd が git の中'
"$andon" task candidates "$nr" - <<<'{"candidates":[{"id":"c1","text":"x","in_scope":false}]}' >/dev/null
jq -e 'has("commit") | not' library/candidates.json >/dev/null
"$andon" task report "$nr" --data '{"next":[{"to":"chief-of-staff","text":"candidates.json から切る"}]}' <<<'## Report' >/dev/null
jq -e 'has("commit") | not' library/report.json >/dev/null
emit library/report.json report-norepo.json
"$andon" task ready "$nr" >/dev/null
popd >/dev/null
emit "kanban/tasks/$nr/library/candidates.json" candidates-norepo.json
[ "$(kanban show "$nr" --json | jq -r .task.status)" = review ] || die 'repo 無し: ready で review にならない'
ok 'repo の無い調べるタスク: commit を書かずに candidates → report → ready で review になる'
# ほかの端末からは、そのタスクの候補を書けない（書き換えない）
cp "$lfx/candidates.json" "$tmp/fx-before.json"
expect_fail '担当' env HC_TERMINAL_ID=t9 "$andon" task candidates "$fx" - <<<'{"candidates":[]}'
expect_fail '担当' env -u HC_TERMINAL_ID "$andon" task candidates "$fx" - <<<'{"candidates":[]}'
cmp -s "$tmp/fx-before.json" "$lfx/candidates.json" || die 'candidates: ほかの端末の書き込みで一覧が変わった'
ok 'candidates: そのタスクの担当（agent）でない端末からは書かずに終了コード 1'

# start: Chief of Staff（t5）が担当を起こす。案件の repo は bare な origin とその clone
export GHQ_ROOT="$tmp/ghq"
# start は起こす前に信頼の設定を足す（boot と同じ手）。一時の HOME に書かせ、本物の CLAUDE_CONFIG_DIR・建屋の repo に触らない
unset CLAUDE_CONFIG_DIR
export ANDON_WORKSPACE="$tmp/plant"
# 建屋の repo（人のもの）: owner が commit した project.json が、書いてよい repo（app だけ）を持つ
git init -q "$tmp/plant"; mkdir -p "$tmp/plant/projects/demo"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
echo '{"version": 1, "project": {"title": "Demo", "prefix": "S"}, "repos": ["github.com/example/app"]}' > "$tmp/plant/projects/demo/project.json"
git -C "$tmp/plant" add -A; GIT_AUTHOR_EMAIL=owner@example.com git -C "$tmp/plant" commit -qm plant
git init -q --bare -b main "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/app" 2>/dev/null
appdir=$GHQ_ROOT/github.com/example/app
git -C "$appdir" commit -q --allow-empty -m init
git -C "$appdir" push -q origin HEAD:main
# 偽の hc: 生きている端末は $tmp/hc/live（1行に address name task）。run は t42, t43 … を返し、打ったものを $tmp/hc/log に。
# $tmp/hc/run-rc があれば run はその終了コードで落ちる。$tmp/hc/first があれば first_message.sent はその中身
mkdir -p "$tmp/hc"; : > "$tmp/hc/live"
cat > "$tmp/bin/hc" <<EOF
#!/usr/bin/env bash
d=$tmp/hc
printf '%s\n' "\$*" >>\$d/log
case \$1 in
  ls) task=; for a in "\$@"; do case \$a in task=*) task=\${a#task=};; esac; done
      awk -v t="\$task" 't == "" || \$3 == t {print \$1, \$2}' \$d/live | jq -Rn '{terminals: [inputs | split(" ") | {address: .[0], name: .[1], state: "live"}]}' ;;
  templates) echo '{"templates":[{"name":"claude"},{"name":"codex"}]}' ;;
  run) [ -e \$d/run-rc ] && { echo boom >&2; exit "\$(cat \$d/run-rc)"; }
       n=\$(cat \$d/next 2>/dev/null || echo 42); echo \$((n + 1)) > \$d/next
       name= task=; while [ \$# -gt 0 ]; do case \$1 in --name) name=\$2; shift;; --label) case \$2 in task=*) task=\${2#task=};; esac; shift;; esac; shift; done
       echo "t\$n \$name \$task" >> \$d/live
       echo "{\"address\":\"t\$n\",\"first_message\":{\"sent\":\$(cat \$d/first 2>/dev/null || echo true),\"reason\":\"trust prompt\"}}" ;;
  kill) ;;
  label) ;;
  rm) sed -i "/^\$2 /d" \$d/live ;;
  *) echo "unexpected hc \$1" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/hc"
field() { kanban show "$1" --json | jq -r --arg k "$2" '.task[$k] // ""'; }
# 台帳は repos に案件の repo を持つ別の台帳（slug は demo）
mkdir -p "$tmp/start/demo" && cd "$tmp/start/demo"
KANBAN_ROOT= kanban init --prefix S --repo github.com/example/app --repo github.com/example/lib >/dev/null
export KANBAN_ROOT=$PWD HC_TERMINAL_ID=t5
kanban config set chief_of_staff=t5 >/dev/null
s1=$(kanban add 'Add the button' --repo github.com/example/app --profile claude --brief - <<<'# Add the button')
"$andon" task start "$s1" > "$tmp/out"
lc=$(tr A-Z a-z <<<"$s1")
wt=$HOME/.herdr/worktrees/app/demo-$s1
br=demo/$s1-add-the-button
[ -d "$wt" ] && [ "$(git -C "$wt" rev-parse --abbrev-ref HEAD)" = "$br" ] || die "worktree か branch が無い: $(cat "$tmp/out")"
[ -z "$(git -C "$wt" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null)" ] || die 'branch に upstream がある'
[ "$(field "$s1" status)" = doing ] && [ "$(field "$s1" agent)" = t42 ] && [ "$(field "$s1" branch)" = "$br" ] && [ "$(field "$s1" worktree)" = "~/.herdr/worktrees/app/demo-$s1" ] \
  || die "台帳: $(kanban show "$s1" --json | jq -c .task)"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$lc --label project=demo --label task=$s1 --label title=Add the button --label herdr.space=andon-demo-$s1 --label herdr.tab=andon-demo-$s1 --cwd $wt claude タスク \`demo/$s1\`。" "$tmp/hc/log" \
  || die "hc run の形が違う: $(grep ^run "$tmp/hc/log")"
! grep -q 'SKILL.md\|先に' "$tmp/hc/log" || die '最初のメッセージに skill を読ませる行が残る（ADR-0001）'
grep -q "$s1 を t42（demo-$lc・claude）で起こした。cwd $wt branch $br" "$tmp/out" || die "out: $(cat "$tmp/out")"
[ "$(tail -n 1 "$tmp/out")" = "andon task: 出た場所 $s1 Add the button / $s1（t42）" ] || die "出た場所が最後に無い: $(cat "$tmp/out")"
jq -e --arg id "$s1" 'select(.id == $id and .change.status == "doing") | .by == "terminal:t42"' kanban/log.jsonl >/dev/null || die 'doing にしたのが担当の端末でない'
ok 'start: worktree と branch（upstream 無し）・hc run（名前・ラベル・herdr.space と herdr.tab・cwd・最初のメッセージ）・kanban start で doing と agent・出た場所'


expect_fail "は doing" "$andon" task start "$s1"
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '二度起こした'
ok 'start: 起こし済み（doing）は起こさない'

s2=$(kanban add '依頼書の無いタスク' --repo github.com/example/app --profile claude)
expect_fail '依頼書（brief.md）が無い' "$andon" task start "$s2"
s3=$(kanban add 'プロファイルの無いもの' --repo github.com/example/app --profile fable --brief - <<<'# x')
expect_fail 'プロファイル fable の template が無い' "$andon" task start "$s3"
[ ! -e "$HOME/.herdr/worktrees/app/demo-$s2" ] && [ ! -e "$HOME/.herdr/worktrees/app/demo-$s3" ] || die '止まる前に worktree を作った'
ok 'start: 依頼書が無い・template が無いなら何も作らずに止まる'

# 台帳の repos に無い repo（clone はある）・未完了の after・origin にだけある同名の branch は、何も作らずに止まる
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/other" 2>/dev/null
s6=$(kanban add 'Other repo' --profile claude --brief - <<<'# x')
kanban set "$s6" repo=github.com/example/other >/dev/null
expect_fail '台帳の config.yaml の repos に無い' "$andon" task start "$s6"
s7=$(kanban add 'Later' --repo github.com/example/app --profile claude --brief - <<<'# x')
kanban set "$s7" "after=$s2" >/dev/null
[ "$(field "$s7" status)" = todo ] || die "after を足しても todo のはず: $(field "$s7" status)"
expect_fail "after（$s2）が済んでいない" "$andon" task start "$s7"
s8=$(kanban add 'Taken name' --repo github.com/example/app --profile claude --brief - <<<'# x')
git -C "$appdir" push -q origin "HEAD:refs/heads/demo/$s8-taken-name"
git -C "$appdir" update-ref -d "refs/remotes/origin/demo/$s8-taken-name"
expect_fail "branch demo/$s8-taken-name が origin にもうある" "$andon" task start "$s8"
# 台帳の config.yaml の repos にはあるが、project.json の repos に無い repo（このプロジェクトが書いてよい repo でない）
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/lib" 2>/dev/null
s9=$(kanban add 'Lib' --repo github.com/example/lib --profile claude --brief - <<<'# x')
expect_fail 'repo github.com/example/lib が projects/demo/project.json の repos に無い' "$andon" task start "$s9"
! git -C "$GHQ_ROOT/github.com/example/lib" show-ref --quiet "refs/heads/demo/$s9-lib" || die 'project.json の repos に無いのに branch を作った'
# project.json を最後に変えたのが owner でなければ、書いてよい repo は無い（安全側）
jq '.repos += ["github.com/example/lib"]' "$tmp/plant/projects/demo/project.json" > "$tmp/p.json" && cp "$tmp/p.json" "$tmp/plant/projects/demo/project.json"
GIT_AUTHOR_EMAIL=agent@example.com git -C "$tmp/plant" commit -qam 'agent が repos を足した'
expect_fail 'author（t <agent@example.com>）が owner でない' "$andon" task start "$s9"
git -C "$tmp/plant" reset -q --hard HEAD~1
for x in "$s6" "$s7" "$s8" "$s9"; do
  [ ! -e "$HOME/.herdr/worktrees/app/demo-$x" ] && [ ! -e "$HOME/.herdr/worktrees/other/demo-$x" ] && [ "$(field "$x" status)" = todo ] && [ -z "$(field "$x" branch)$(field "$x" worktree)" ] \
    || die "$x: 止まる前に作った: $(kanban show "$x" --json | jq -c .task)"
  ! grep -q "task=$x " "$tmp/hc/log" || die "$x: hc run を打った"
done
! git -C "$GHQ_ROOT/github.com/example/other" show-ref --quiet "refs/heads/demo/$s6-other-repo" && ! git -C "$appdir" show-ref --quiet "refs/heads/demo/$s7-later" || die '止まる前に branch を作った'
ok 'start: 台帳か project.json（owner の commit）の repos に無い repo・未完了の after・origin にある同名の branch では、worktree・branch・台帳の欄・hc run を作らずに止まる'

# hc run が落ちたら、作った worktree と branch を戻す
s4=$(kanban add 'Run fails' --repo github.com/example/app --profile codex --brief - <<<'# x')
echo 1 > "$tmp/hc/run-rc"
expect_fail 'hc run に失敗' "$andon" task start "$s4"
rm "$tmp/hc/run-rc"
[ ! -e "$HOME/.herdr/worktrees/app/demo-$s4" ] && ! git -C "$appdir" show-ref --quiet "refs/heads/demo/$s4-run-fails" \
  && [ "$(field "$s4" status)" = todo ] && [ -z "$(field "$s4" branch)$(field "$s4" worktree)" ] || die "戻していない: $(kanban show "$s4" --json | jq -c .task)"
"$andon" task start "$s4" >/dev/null || die 'やり直せない'
ok 'start: hc run が落ちたら worktree・branch・台帳の欄を戻す（やり直せる）'

# 最初のメッセージが届かない（信頼の画面など）
s5=$(kanban add 'Trust prompt' --profile claude --brief - <<<'# x')   # repo の無いタスク: tasks/<id>/ で起こる
echo false > "$tmp/hc/first"
rc=0; "$andon" task start "$s5" >"$tmp/out" 2>"$tmp/err" || rc=$?
rm "$tmp/hc/first"
[ "$rc" = 3 ] || die "rc=$rc $(cat "$tmp/err")"
addr=$(field "$s5" agent)
grep -q "に最初のメッセージが届いていない: trust prompt。dispatcher が見回りで送り直す（.*急ぐときだけ手で: hc send $addr \"タスク \`demo/$s5\`。" "$tmp/err" \
  && [ "$(field "$s5" status)" = doing ] || die "案内: $(cat "$tmp/err")"
grep -q "^label $addr first_message.sent=false\$" "$tmp/hc/log" || die 'dispatcher に預けるラベルを付けていない'
grep -q "^run .*--cwd $PWD/kanban/tasks/$s5 claude " "$tmp/hc/log" || die 'repo の無いタスクの cwd が tasks/<id>/ でない'
ok 'start: 最初のメッセージが届かなければ終了コード 3・端末にラベル first_message.sent=false（dispatcher が送り直す）と急ぐとき用の hc send（台帳は doing・agent はその端末）。repo の無いタスクは tasks/<id>/ で'

# start が作ったものを finish が片付ける
: > "$tmp/hc/log"
"$andon" task finish "$s1" >/dev/null
[ -z "$(grep -v '^ls \|^kill \|^rm ' "$tmp/hc/log")" ] || die "finish が kill・rm（と ls）のほかに hc を打った: $(cat "$tmp/hc/log")"
[ ! -e "$wt" ] && ! git -C "$appdir" show-ref --quiet "refs/heads/$br" && [ "$(field "$s1" status)" = done ] && ! grep -q " $s1\$" "$tmp/hc/live" \
  || die "finish で片付かない: $(kanban show "$s1" --json | jq -c .task)"
ok 'start が作った worktree・branch・端末を finish が片付ける（ラベルに触らない。hc は ls・kill・rm だけ）'

echo "all passed ($pass)"
