#!/usr/bin/env bash
# `andon issue show/report/review-request/ready/ask/start`。本物の台帳の CLI（kanban）・一時の台帳と git・偽の gh・偽の hc で回す。
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
id=$(as t5 kanban add 'andon issue を足す' --profile claude)
as t5 kanban set "$id" review=on must_fix=major "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
lib=kanban/issues/$id/library
pr=https://github.com/example/app/pull/7
export HC_TERMINAL_ID=t7

# report
expect_fail 'Markdown' "$andon" issue report "$id" --pr "$pr" </dev/null
ok 'report: 人用の報告が無ければ止まる'
expect_fail 'commit・review_request' "$andon" issue report "$id" --data '{"commit":"x"}' <<<'## Report'
expect_fail '--data が読めない' "$andon" issue report "$id" --data '{"title":"x"}' <<<'## Report'
expect_fail 'next は' "$andon" issue report "$id" --data '{"next":[{"to":"human","text":"x"}]}' <<<'## Report'
[ ! -e "$lib/report.json" ] || die 'report.json が書かれた'
ok 'report: --data の知らないキー・command のキー・知らない宛先は書く前に止まる'
"$andon" issue report "$id" --pr "$pr" --data '{"next":[{"to":"chief-of-staff","text":"マージ後に片付ける"}],"remember":["x"]}' <<<'## Report' >/dev/null
jq -e --arg c "$c1" --arg pr "$pr" '.commit == $c and .pr == $pr and .next[0].to == "chief-of-staff" and (has("review_request") | not)' "$lib/report.json" >/dev/null
grep -q '## Report' "kanban/issues/$id/report.md"
emit "$lib/report.json" report-1.json
ok 'report: commit は HEAD、pr と --data を書き、台帳に報告する'
"$andon" issue report "$id" <<<'## Report' >/dev/null
jq -e --arg pr "$pr" '.pr == $pr and (has("next") | not)' "$lib/report.json" >/dev/null
ok 'report: --pr を省くと前の pr を引き継ぎ、ほかは書き直す'

# review-request（1 回目）
"$andon" issue review-request "$id" <<<'## Report' >/dev/null
jq -e --arg c "$c1" '.review_request == {round: 1, limit: 2, commit: $c, previous: null, reviewer: "codex-review", extra: false}' "$lib/report.json" >/dev/null
head -2 "kanban/issues/$id/report.md" | grep -q "^1 回目／上限 2。commit $c1"
emit "$lib/report.json" report-review-request.json
ok 'review-request: 1 回目・前回 null・reviewer は実装と別ブランド・人用の行を先頭に'

# ready の負: レビューが無い
expect_fail 'レビューが missing' "$andon" issue ready "$id"
ok 'ready: レビューが無ければ止まる'

# レビュアー（Chief of Staff が足す）。commit c1 を見た
rev=$(as t5 kanban add "$id のレビュー" --parent "$id" --profile codex-review)
mkdir -p "kanban/issues/$rev/library"
jq -n --arg c "$c1" '{commit: $c, findings: [{severity: "minor", file: "f", line: 1}]}' > "kanban/issues/$rev/library/findings.json"
as t5 kanban review "$rev" >/dev/null

# HEAD を進める（直した）。report.json の commit は古い
c2=$(commit two)
expect_fail 'report.json の commit' "$andon" issue ready "$id"
ok 'ready: report.json の commit が HEAD と違えば止まる（HEAD 不一致）'
"$andon" issue report "$id" <<<'## Report' >/dev/null
expect_fail 'PR の head' "$andon" issue ready "$id"
ok 'ready: PR の head が HEAD と違えば止まる（push 前）'
echo "$c2" > "$tmp/prhead"
expect_fail 'レビューが stale' "$andon" issue ready "$id"
ok 'ready: レビューが古い commit を見ていれば止まる（stale）'

# review-request（2 回目）: 前回は c1
"$andon" issue review-request "$id" <<<'## Report' >/dev/null
jq -e --arg c "$c2" --arg p "$c1" '.review_request | .round == 2 and .commit == $c and .previous == $p and .extra == false' "$lib/report.json" >/dev/null
ok 'review-request: 2 回目・previous は前のレビューの commit'

# 担当が自分で足した子課題は数えない
fake=$(kanban add "$id の自作レビュー" --parent "$id" --profile codex-review)
mkdir -p "kanban/issues/$fake/library"
jq -n --arg c "$c2" '{commit: $c, findings: []}' > "kanban/issues/$fake/library/findings.json"
kanban review "$fake" >/dev/null
expect_fail 'レビューが stale' "$andon" issue ready "$id"
ok 'ready: Chief of Staff が足していないレビューは数えない'

# 形の壊れた findings.json は missing
rev2=$(as t5 kanban add "$id のレビュー 2" --parent "$id" --profile codex-review)
mkdir -p "kanban/issues/$rev2/library"
jq -n --arg c "$c2" '{commit: $c, findings: [{severity: "Major", file: "f", line: 1}]}' > "kanban/issues/$rev2/library/findings.json"
as t5 kanban review "$rev2" >/dev/null
expect_fail 'findings.json が無い・形が違う' "$andon" issue ready "$id"
ok 'ready: findings.json の形が違えば missing'
jq -n --arg c "$c2" '{commit: $c, findings: [{severity: "minor", file: "f", line: 2}]}' > "kanban/issues/$rev2/library/findings.json"

# HEAD のレビューがもうあれば頼まない
expect_fail 'レビューはもうある' "$andon" issue review-request "$id" <<<'## Report'
ok 'review-request: HEAD のレビューがあれば頼まない'

# 結果を残さず止まったレビュアーの課題があっても、次の依頼の回は被らない（id <id>-rN が空いている）
c3=$(commit three)
echo "$c3" > "$tmp/prhead"
dead=$(as t5 kanban add "$id のレビュー（結果なし）" --parent "$id" --profile codex-review --round 5)
"$andon" issue report "$id" <<<'## Report' >/dev/null
"$andon" issue review-request "$id" <<<'## Report' >/dev/null
jq -e --arg p "$c2" '.review_request | .round == 6 and .previous == $p' "$lib/report.json" >/dev/null
ok 'review-request: 結果の無いレビュアーの課題の次の回になる（previous は結果のある回のまま）'
# 後続の検査は c2 の状態で続ける
git -C "$tmp/wt" reset -q --hard "$c2"
echo "$c2" > "$tmp/prhead"
"$andon" issue report "$id" <<<'## Report' >/dev/null

# report.json の形が壊れていれば止まる
cp "$lib/report.json" "$tmp/good"
jq '. + {"x_unknown": 1}' "$tmp/good" > "$lib/report.json"
expect_fail 'report.json の形が違う' "$andon" issue ready "$id"
jq '.commit = "abc"' "$tmp/good" > "$lib/report.json"
expect_fail 'report.json の形が違う' "$andon" issue ready "$id"
jq 'del(.pr)' "$tmp/good" > "$lib/report.json"
expect_fail 'pr が無い' "$andon" issue ready "$id"
ok 'ready: report.json の知らないキー・壊れた commit・pr 無しで止まる'
cp "$tmp/good" "$lib/report.json"

# show
"$andon" issue show "$id" > "$tmp/show"
jq -e --arg rev "$rev2" --arg c "$c2" '.settings == {review: "on", must_fix: "major", reviewer: "codex-review", review_rounds: "2", merge: "person", deliverable: "pr"}
  and .review == {state: "current", issue: $rev, commit: $c, must_fix_open: 0, waived: 0} and .report.commit == $c' "$tmp/show" >/dev/null
jq .settings "$tmp/show" > "$tmp/settings.json"; emit "$tmp/settings.json" issue-keys.json
ok 'show: 予約キー（既定で埋める）・report.json・レビューの照合'

# ready の正
"$andon" issue ready "$id" >/dev/null
[ "$(kanban show "$id" --json | jq -r .issue.status)" = review ]
ok 'ready: HEAD ＝ report.json ＝ PR の head ＝ レビューの commit で review にする'

# review: off なら照合しない。review が無ければ止まる
id2=$(as t5 kanban add 'レビュー無し' --profile codex)
as t5 kanban set "$id2" review=off "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
"$andon" issue report "$id2" --pr "$pr" <<<'## Report' >/dev/null
expect_fail 'review が on でない' "$andon" issue review-request "$id2" <<<'## Report'
"$andon" issue ready "$id2" >/dev/null
ok 'ready: review: off ならレビューを照合しない。review-request は断る'
id3=$(as t5 kanban add 'Settings 無し')
as t5 kanban set "$id3" "worktree=$tmp/wt" repo=github.com/example/app review_rounds=1 >/dev/null
"$andon" issue report "$id3" --pr "$pr" <<<'## Report' >/dev/null
expect_fail 'review が on/off でない' "$andon" issue ready "$id3"
ok 'ready: issue.yaml の review が無ければ止まる'
as t5 kanban set "$id3" review=on >/dev/null
r3=$(as t5 kanban add "$id3 のレビュー" --parent "$id3" --profile codex-review)
mkdir -p "kanban/issues/$r3/library"; jq -n --arg c "$c1" '{commit: $c, findings: []}' > "kanban/issues/$r3/library/findings.json"
"$andon" issue review-request "$id3" <<<'## Report' >/dev/null
jq -e '.review_request | .round == 2 and .limit == 1 and .extra == true' "kanban/issues/$id3/library/report.json" >/dev/null
head -2 "kanban/issues/$id3/report.md" | grep -q '（追加）'
ok 'review-request: 上限を超えた回は extra'

# ask
q='{"kind":"users","question":"既定を変えるか","facts":["docs/design.md §1.10"],"options":[{"label":"変える","pick":true},{"label":"変えない"}],"remaining":1,"from":"issue"}'
"$andon" issue ask "$id" - <<<"$q" | grep -q "$id#1"
jq -e '.kind == "users" and .options[0].pick == true' "$lib/questions/1.json" >/dev/null
kanban show "$id" --json | jq -e '.questions[0] | .kind == "users" and (.text | contains("推奨案: 変える")) and (.text | contains("あと 1 問"))' >/dev/null
emit "$lib/questions/1.json" question.json
ok 'ask: 問いを台帳に置き、JSON を library/questions/<n>.json に'
"$andon" issue ask "$id" - <<<"$(jq -c '.kind = "line"' <<<"$q")" >/dev/null
kanban show "$id" --json | jq -e '.questions[1].kind == "line"' >/dev/null
jq -e '.kind == "line"' "$lib/questions/2.json" >/dev/null
ok 'ask: kind line も台帳に kind を付ける（kanban ask --kind line）'
for bad in '.options[1].pick = true' '.options = [.options[0]]' '.question = "a\nb"' '. + {"x": 1}' '.kind = "other"'; do
    expect_fail '問いの JSON の形が違う' "$andon" issue ask "$id" - <<<"$(jq -c "$bad" <<<"$q")"
done
[ "$(kanban show "$id" --json | jq '.questions | length')" = 2 ]
ok 'ask: 形の違う問いは台帳に書く前に止まる'

# 候補・範囲の問い・調べる課題の ready（ADR-0006 決めたこと 2・3・4・7。受け入れ条件 2）。worktree は別に切る
git init -q "$tmp/wt2"
commit2() { echo "$1" > "$tmp/wt2/f"; git -C "$tmp/wt2" add f; git -C "$tmp/wt2" commit -qm "$1"; git -C "$tmp/wt2" rev-parse HEAD; }
s1=$(commit2 one)
field0() { kanban show "$1" --json | jq -r --arg k "$2" '.issue[$k] // ""'; }
# 直す課題が範囲で止まる: candidates → ask（scope）→ stop-check は通す
fx=$(as t5 kanban add '誤字を1つ直す' --profile claude)
as t5 kanban set "$fx" review=off must_fix=major "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
kanban start "$fx" >/dev/null
lfx=kanban/issues/$fx/library
cands='{"candidates":[{"id":"c1","text":"docs/a.md:4 「台長」→「台帳」","file":"docs/a.md","line":4,"in_scope":true},{"id":"c2","text":"README の段を書き直す","in_scope":false}]}'
for bad in '.candidates[1].id = "c1"|重なっている' 'del(.candidates[0].in_scope)|in_scope が無い' '.candidates[0] += {"tier": "A"}|知らないキー' \
           '.candidates[0].text = "a\nb"|2行以上' '.candidates[0].file = "/etc/x"|相対パス' '.candidates[0].line = 0|line が' '.candidates[0].id = "x1"|c1, c2' \
           '. + {"commit": "'"$s1"'"}|commit は書かない' 'del(.candidates)|candidates が無い'; do
    expect_fail "${bad#*|}" "$andon" issue candidates "$fx" - <<<"$(jq -c "${bad%|*}" <<<"$cands")"
    [ "$(wc -l < "$tmp/err")" = 1 ] || die "candidates: 理由が1行でない: $(cat "$tmp/err")"
done
[ ! -e "$lfx/candidates.json" ] || die 'candidates: 形の違う一覧を書いた'
ok 'candidates: 形が違えば書かずに終了コード 1 と理由1行（id の重複・in_scope 無し・知らないキー・commit を書く）'
"$andon" issue candidates "$fx" - <<<"$cands" | grep -q '候補 2 件、範囲の中 1 件'
jq -e --arg c "$s1" '.commit == $c and (.candidates | length) == 2 and .candidates[0].file == "docs/a.md" and .candidates[1].in_scope == false and (.candidates[1] | has("file") or has("line") | not)' "$lfx/candidates.json" >/dev/null
emit "$lfx/candidates.json" candidates.json
printf '%s' "$cands" > "$tmp/cands.json"; "$andon" issue candidates "$fx" "$tmp/cands.json" >/dev/null
ok 'candidates: 形を確かめて library/candidates.json に書く（commit は HEAD。ファイルからも）'
expect_fail '形が違う' "$andon" issue candidates "$fx" - <<<'{"candidates":[{"id":"c1","text":"x","in_scope":true},{"id":"c1","text":"y","in_scope":true}]}'
jq -e '(.candidates | length) == 2 and .candidates[1].id == "c2"' "$lfx/candidates.json" >/dev/null || die 'candidates: 形の違う一覧で前の一覧を壊した'
ok 'candidates: 形が違えば前の candidates.json はそのまま'
sq=$(jq -nc --arg f "$lfx/candidates.json" '{kind: "scope", question: "誤字は1つではなく2つ。どれを直すか", facts: [$f, "依頼書: 誤字を1つ直す"],
  options: [{label: "c1 と c2 を全部この PR で", pick: true}, {label: "c1 だけ。c2 は別の課題"}], from: "issue"}')
"$andon" issue ask "$fx" - <<<"$sq" | grep -q "$fx#1"
kanban show "$fx" --json | jq -e '.questions[0] | .kind == "scope" and .status == "open" and (.text | contains("（scope）"))' >/dev/null
jq -e '.kind == "scope"' "$lfx/questions/1.json" >/dev/null
emit "$lfx/questions/1.json" question-scope.json
ok 'ask: kind scope を受けて kanban ask --kind scope を打つ'
out=$("$andon" issue stop-check "$fx" <<<'{"stop_hook_active":false}' 2>/dev/null)
[ -z "$out" ] || die "stop-check: scope の問いが開いたままのターンを止めた: $out"
out=$("$andon" issue stop-check "$fx" <<<'{"stop_hook_active":false}' 2>/dev/null)
[ -z "$out" ] || die "stop-check: scope の問いの答えを待つターンを止めた: $out"
ok 'stop-check: scope の問いが開いたままのターンを通す'

# 調べる課題（deliverable: candidates）の ready: PR 無しで candidates.json を要る
sv=$(as t5 kanban add '調べる: 古い command 名' --profile claude)
as t5 kanban set "$sv" deliverable=candidates review=off "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
kanban start "$sv" >/dev/null
lsv=kanban/issues/$sv/library
"$andon" issue report "$sv" --data '{"next":[{"to":"chief-of-staff","text":"candidates.json から切る"}]}' <<<'## Report' >/dev/null
expect_fail 'candidates.json が無い' "$andon" issue ready "$sv"
ok 'ready（candidates）: candidates.json が無ければ止まる'
echo '{"candidates":[' > "$lsv/candidates.json"
expect_fail 'candidates.json の形が違う' "$andon" issue ready "$sv"
ok 'ready（candidates）: 手で壊した candidates.json で止まる'
"$andon" issue candidates "$sv" - <<<'{"candidates":[]}' >/dev/null
s2=$(commit2 two)
"$andon" issue report "$sv" <<<'## Report' >/dev/null
expect_fail 'candidates.json の commit' "$andon" issue ready "$sv"
ok 'ready（candidates）: candidates.json の commit が HEAD と違えば止まる'
"$andon" issue candidates "$sv" - <<<'{"candidates":[]}' >/dev/null
"$andon" issue show "$sv" > "$tmp/show-sv"
jq -e '.settings.deliverable == "candidates"' "$tmp/show-sv" >/dev/null
jq .settings "$tmp/show-sv" > "$tmp/settings-sv.json"; emit "$tmp/settings-sv.json" issue-keys-candidates.json
"$andon" issue ready "$sv" >/dev/null
[ "$(kanban show "$sv" --json | jq -r .issue.status)" = review ] || die 'ready（candidates）: review にならない'
ok 'ready（candidates）: PR 無しで、候補 0 件の candidates.json と HEAD が揃えば review にする（show の settings に deliverable）'
# Chief of Staff でない書き手の deliverable は信じず pr として読む（PR を要る）
sv2=$(as t5 kanban add '調べる: 担当が自分で型を変える' --profile claude)
as t5 kanban set "$sv2" review=off "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
as t7 kanban set "$sv2" deliverable=candidates >/dev/null
kanban start "$sv2" >/dev/null
"$andon" issue candidates "$sv2" - <<<'{"candidates":[]}' >/dev/null
"$andon" issue report "$sv2" <<<'## Report' >/dev/null
expect_fail 'pr が無い' "$andon" issue ready "$sv2"
ok 'ready: deliverable を最後に書いたのが Chief of Staff でなければ pr として PR を要る'
# repo の無い調べる課題: commit を書かず、HEAD を照合しない。cwd は git の外（start と同じ issues/<id>/）で、candidates → report → ready まで通る。
# kanban start を git の checkout の中で打つと worktree が入るが（台帳が checkout の中にあるとき）、repo が無ければ repo の無い課題として読む
nr=$(as t5 kanban add '調べる: repo 無し' --profile claude)
as t5 kanban set "$nr" deliverable=candidates review=off >/dev/null
(cd "$tmp/wt2" && kanban start "$nr" >/dev/null)
[ -n "$(field0 "$nr" worktree)" ] || die 'repo 無し: kanban start が worktree を書かない（前提が崩れた）'
pushd "kanban/issues/$nr" >/dev/null
! git rev-parse HEAD >/dev/null 2>&1 || die 'repo 無し: cwd が git の中'
"$andon" issue candidates "$nr" - <<<'{"candidates":[{"id":"c1","text":"x","in_scope":false}]}' >/dev/null
jq -e 'has("commit") | not' library/candidates.json >/dev/null
commit2 three >/dev/null   # worktree に入った checkout の HEAD が進んでも止まらない
"$andon" issue report "$nr" --data '{"next":[{"to":"chief-of-staff","text":"candidates.json から切る"}]}' <<<'## Report' >/dev/null
jq -e 'has("commit") | not' library/report.json >/dev/null
emit library/report.json report-norepo.json
"$andon" issue ready "$nr" >/dev/null
popd >/dev/null
emit "kanban/issues/$nr/library/candidates.json" candidates-norepo.json
[ "$(kanban show "$nr" --json | jq -r .issue.status)" = review ] || die 'repo 無し: ready で review にならない'
ok 'repo の無い調べる課題: commit を書かずに candidates → report → ready で review になる'
# ほかの端末からは、その課題の候補を書けない（書き換えない）
cp "$lfx/candidates.json" "$tmp/fx-before.json"
expect_fail '担当' env HC_TERMINAL_ID=t9 "$andon" issue candidates "$fx" - <<<'{"candidates":[]}'
expect_fail '担当' env -u HC_TERMINAL_ID "$andon" issue candidates "$fx" - <<<'{"candidates":[]}'
cmp -s "$tmp/fx-before.json" "$lfx/candidates.json" || die 'candidates: ほかの端末の書き込みで一覧が変わった'
ok 'candidates: その課題の担当（agent）でない端末からは書かずに終了コード 1'

# start: Chief of Staff（t5）が担当を起こす。案件の repo は bare な origin とその clone
export GHQ_ROOT="$tmp/ghq"
# start は起こす前に信頼の設定を足す（boot と同じ手）。一時の HOME に書かせ、本物の CLAUDE_CONFIG_DIR・セルの repo に触らない
unset CLAUDE_CONFIG_DIR
export ANDON_WORKSPACE="$tmp/plant"
# セルの repo（人のもの）: owner が commit した project.json が、書いてよい repo（app だけ）を持つ
git init -q "$tmp/plant"; mkdir -p "$tmp/plant/projects/demo"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
# signals 入りでも start が通る（ADR-0011 受け入れ条件 12。signals を知らない版は project.json ごと弾いて起こせなくなる）
echo '{"version": 1, "project": {"title": "Demo", "prefix": "S"}, "repos": ["github.com/example/app"],
 "signals": [{"source": "github-ci", "repo": "github.com/example/app", "weight": "now"}, {"source": "github-issues", "weight": "later", "filter": "label:bug"}]}' > "$tmp/plant/projects/demo/project.json"
git -C "$tmp/plant" add -A; GIT_AUTHOR_EMAIL=owner@example.com git -C "$tmp/plant" commit -qm plant
git init -q --bare -b main "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/app" 2>/dev/null
appdir=$GHQ_ROOT/github.com/example/app
git -C "$appdir" commit -q --allow-empty -m init
git -C "$appdir" push -q origin HEAD:main
# 偽の hc: 生きている端末は $tmp/hc/live（1行に address name issue）。run は t42, t43 … を返し、打ったものを $tmp/hc/log に。
# $tmp/hc/run-rc があれば run はその終了コードで落ちる。$tmp/hc/first があれば first_message.sent はその中身。
# get は $tmp/hc/get-<address> があればその中身、live にあれば動いている Claude Code、無ければ not_found
mkdir -p "$tmp/hc"; : > "$tmp/hc/live"
cat > "$tmp/bin/hc" <<EOF
#!/usr/bin/env bash
d=$tmp/hc
printf '%s\n' "\$*" >>\$d/log
case \$1 in
  ls) issue=; for a in "\$@"; do case \$a in issue=*) issue=\${a#issue=};; esac; done
      awk -v t="\$issue" 't == "" || \$3 == t {print \$1, \$2}' \$d/live | jq -Rn '{terminals: [inputs | split(" ") | {address: .[0], name: .[1], state: "live"}]}' ;;
  templates) echo '{"templates":[{"name":"claude"},{"name":"codex"},{"name":"claude-resume"},{"name":"codex-resume"},{"name":"chief-engineer"},{"name":"chief-engineer-resume"},{"name":"codex-review-resume"},{"name":"claude-review-resume"}]}' ;;
  get) if [ -e \$d/get-\$2 ]; then cat \$d/get-\$2
       elif grep -q "^\$2 " \$d/live; then echo '{"state":"live","argv":["sh","-c","claude"],"foreground":{"argv":["sh","-c","claude"]},"labels":{}}'
       else echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2; exit 3; fi ;;
  keep) ;;
  run) [ -e \$d/run-rc ] && { echo boom >&2; exit "\$(cat \$d/run-rc)"; }
       n=\$(cat \$d/next 2>/dev/null || echo 42); echo \$((n + 1)) > \$d/next
       name= issue=; while [ \$# -gt 0 ]; do case \$1 in --name) name=\$2; shift;; --label) case \$2 in issue=*) issue=\${2#issue=};; esac; shift;; esac; shift; done
       echo "t\$n \$name \$issue" >> \$d/live
       echo "{\"address\":\"t\$n\",\"first_message\":{\"sent\":\$(cat \$d/first 2>/dev/null || echo true),\"reason\":\"trust prompt\"}}" ;;
  kill) ;;
  label) ;;
  rm) sed -i "/^\$2 /d" \$d/live ;;
  *) echo "unexpected hc \$1" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/hc"
field() { kanban show "$1" --json | jq -r --arg k "$2" '.issue[$k] // ""'; }
# 台帳は repos に案件の repo を持つ別の台帳（slug は demo）
mkdir -p "$tmp/start/demo" && cd "$tmp/start/demo"
KANBAN_ROOT= kanban init --prefix S --repo github.com/example/app --repo github.com/example/lib >/dev/null
export KANBAN_ROOT=$PWD HC_TERMINAL_ID=t5
kanban config set chief_of_staff=t5 >/dev/null
s1=$(kanban add 'Add the button' --repo github.com/example/app --profile claude --brief - <<<'# Add the button')
# 乾いた実行: 本物の台帳で、hc を呼ばず、何も作らず台帳も書かずに、起こすときに渡すものを出す
hcn=$(cat "$tmp/hc/log" 2>/dev/null | wc -l || true); before=$(wc -l < kanban/log.jsonl)
"$andon" issue start "$s1" --dry-run --json > "$tmp/dry" || die "dry-run が落ちた"
[ "$(cat "$tmp/hc/log" 2>/dev/null | wc -l || true)" = "$hcn" ] && [ "$(wc -l < kanban/log.jsonl)" = "$before" ] && [ "$(field "$s1" status)" = todo ] && [ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s1" ] \
  || die "dry-run が hc を呼んだか何かを作った: $(cat "$tmp/hc/log")"
jq -e --arg id "$s1" --arg wt "$ANDON_WORKSPACE/.worktrees/app/demo-$s1" '.dry_run and .terminal == ("demo-" + ($id | ascii_downcase)) and .worktree == $wt and .cwd == $wt
  and .branch == "demo/\($id)-add-the-button" and .profile == "claude" and (.brief | startswith("# Add the button"))
  and ([.checks[] | select(.result == "ok") | .name] | index("delegation") != null)' "$tmp/dry" >/dev/null || die "dry-run の中身: $(cat "$tmp/dry")"
ok 'start --dry-run: hc を呼ばず worktree・台帳に触れずに、端末の名前・worktree・branch・プロファイル・依頼書・確かめの結果を出す'
"$andon" issue start "$s1" > "$tmp/out"
[ "$(jq -r .first_message "$tmp/dry")" = "$(printf '課題 `demo/%s`。\n依頼書は `%s`。' "$s1" "$(jq -r .brief_path "$tmp/dry")")" ] || die "dry-run の最初のメッセージの形: $(jq .first_message "$tmp/dry")"
lc=$(tr A-Z a-z <<<"$s1")
wt=$ANDON_WORKSPACE/.worktrees/app/demo-$s1
br=demo/$s1-add-the-button
[ -d "$wt" ] && [ "$(git -C "$wt" rev-parse --abbrev-ref HEAD)" = "$br" ] || die "worktree か branch が無い: $(cat "$tmp/out")"
[ -z "$(git -C "$wt" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null)" ] || die 'branch に upstream がある'
[ "$(field "$s1" status)" = doing ] && [ "$(field "$s1" agent)" = t42 ] && [ "$(field "$s1" branch)" = "$br" ] && [ "$(field "$s1" worktree)" = "$wt" ] \
  || die "台帳: $(kanban show "$s1" --json | jq -c .issue)"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$lc --label project=demo --label issue=$s1 --label title=Add the button --label herdr.space=andon-demo-$s1 --label herdr.tab=andon-demo-$s1 --label session=$(field "$s1" session) --cwd $wt claude 課題 \`demo/$s1\`。" "$tmp/hc/log" \
  || die "hc run の形が違う: $(grep ^run "$tmp/hc/log")"
! grep -q 'SKILL.md\|先に' "$tmp/hc/log" || die '最初のメッセージに skill を読ませる行が残る（ADR-0001）'
grep -q "$s1 を t42（demo-$lc・claude）で起こした。cwd $wt branch $br" "$tmp/out" || die "out: $(cat "$tmp/out")"
[ "$(tail -n 1 "$tmp/out")" = "andon issue: 出た場所 $s1 Add the button / $s1（t42）" ] || die "出た場所が最後に無い: $(cat "$tmp/out")"
jq -e --arg id "$s1" 'select(.id == $id and .change.status == "doing") | .by == "terminal:t42"' kanban/log.jsonl >/dev/null || die 'doing にしたのが担当の端末でない'
[ "$(cat "$ANDON_WORKSPACE/.worktrees/.gitignore")" = '*' ] && ! git -C "$ANDON_WORKSPACE" status --short | grep -q worktrees \
  || die "置き場の .gitignore が * でないか、セルの repo の git status に出る: $(git -C "$ANDON_WORKSPACE" status --short)"
git -C "$ANDON_WORKSPACE" diff --quiet HEAD -- .gitignore && [ ! -e "$ANDON_WORKSPACE/.gitignore" ] || die 'セルの追跡される .gitignore に触った'
ok 'start: 置き場はセルの repo の .worktrees/<repo 名>/<slug>-<id>。中の .gitignore（*）で git status に出ない（ADR-0012 決めたこと 6）'
ok 'start: worktree と branch（upstream 無し）・hc run（名前・ラベル・herdr.space と herdr.tab・cwd・最初のメッセージ）・kanban start で doing と agent・出た場所'


expect_fail "は doing" "$andon" issue start "$s1"
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '二度起こした'
ok 'start: 起こし済み（doing）は起こさない'

s2=$(kanban add '依頼書の無い課題' --repo github.com/example/app --profile claude)
expect_fail '依頼書（brief.md）が無い' "$andon" issue start "$s2"
s3=$(kanban add 'プロファイルの無いもの' --repo github.com/example/app --profile fable --brief - <<<'# x')
expect_fail 'プロファイル fable の template が無い' "$andon" issue start "$s3"
[ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s2" ] && [ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s3" ] || die '止まる前に worktree を作った'
ok 'start: 依頼書が無い・template が無いなら何も作らずに止まる'

# 台帳の repos に無い repo（clone はある）・未完了の after・origin にだけある同名の branch は、何も作らずに止まる
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/other" 2>/dev/null
s6=$(kanban add 'Other repo' --profile claude --brief - <<<'# x')
kanban set "$s6" repo=github.com/example/other >/dev/null
expect_fail '台帳の config.yaml の repos に無い' "$andon" issue start "$s6"
s7=$(kanban add 'Later' --repo github.com/example/app --profile claude --brief - <<<'# x')
kanban set "$s7" "after=$s2" >/dev/null
[ "$(field "$s7" status)" = todo ] || die "after を足しても todo のはず: $(field "$s7" status)"
expect_fail "after（$s2）が済んでいない" "$andon" issue start "$s7"
s8=$(kanban add 'Taken name' --repo github.com/example/app --profile claude --brief - <<<'# x')
git -C "$appdir" push -q origin "HEAD:refs/heads/demo/$s8-taken-name"
git -C "$appdir" update-ref -d "refs/remotes/origin/demo/$s8-taken-name"
expect_fail "branch demo/$s8-taken-name が origin にもうある" "$andon" issue start "$s8"
# 台帳の config.yaml の repos にはあるが、project.json の repos に無い repo（このプロジェクトが書いてよい repo でない）
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/lib" 2>/dev/null
s9=$(kanban add 'Lib' --repo github.com/example/lib --profile claude --brief - <<<'# x')
expect_fail 'repo github.com/example/lib が projects/demo/project.json の repos に無い' "$andon" issue start "$s9"
! git -C "$GHQ_ROOT/github.com/example/lib" show-ref --quiet "refs/heads/demo/$s9-lib" || die 'project.json の repos に無いのに branch を作った'
# project.json を最後に変えたのが owner でなければ、書いてよい repo は無い（安全側）
jq '.repos += ["github.com/example/lib"]' "$tmp/plant/projects/demo/project.json" > "$tmp/p.json" && cp "$tmp/p.json" "$tmp/plant/projects/demo/project.json"
GIT_AUTHOR_EMAIL=agent@example.com git -C "$tmp/plant" commit -qam 'agent が repos を足した'
expect_fail 'author（t <agent@example.com>）が owner でない' "$andon" issue start "$s9"
git -C "$tmp/plant" reset -q --hard HEAD~1
for x in "$s6" "$s7" "$s8" "$s9"; do
  [ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$x" ] && [ ! -e "$ANDON_WORKSPACE/.worktrees/other/demo-$x" ] && [ "$(field "$x" status)" = todo ] && [ -z "$(field "$x" branch)$(field "$x" worktree)" ] \
    || die "$x: 止まる前に作った: $(kanban show "$x" --json | jq -c .issue)"
  ! grep -q "issue=$x " "$tmp/hc/log" || die "$x: hc run を打った"
done
! git -C "$GHQ_ROOT/github.com/example/other" show-ref --quiet "refs/heads/demo/$s6-other-repo" && ! git -C "$appdir" show-ref --quiet "refs/heads/demo/$s7-later" || die '止まる前に branch を作った'
ok 'start: 台帳か project.json（owner の commit）の repos に無い repo・未完了の after・origin にある同名の branch では、worktree・branch・台帳の欄・hc run を作らずに止まる'

# hc run が落ちたら、作った worktree と branch を戻す
s4=$(kanban add 'Run fails' --repo github.com/example/app --profile codex --brief - <<<'# x')
echo 1 > "$tmp/hc/run-rc"
expect_fail 'hc run に失敗' "$andon" issue start "$s4"
rm "$tmp/hc/run-rc"
[ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s4" ] && ! git -C "$appdir" show-ref --quiet "refs/heads/demo/$s4-run-fails" \
  && [ "$(field "$s4" status)" = todo ] && [ -z "$(field "$s4" branch)$(field "$s4" worktree)" ] || die "戻していない: $(kanban show "$s4" --json | jq -c .issue)"
"$andon" issue start "$s4" >/dev/null || die 'やり直せない'
ok 'start: hc run が落ちたら worktree・branch・台帳の欄を戻す（やり直せる）'

# 最初のメッセージが届かない（信頼の画面など）
s5=$(kanban add 'Trust prompt' --profile claude --brief - <<<'# x')   # repo の無い課題: issues/<id>/ で起こる
echo false > "$tmp/hc/first"
rc=0; "$andon" issue start "$s5" >"$tmp/out" 2>"$tmp/err" || rc=$?
rm "$tmp/hc/first"
[ "$rc" = 3 ] || die "rc=$rc $(cat "$tmp/err")"
addr=$(field "$s5" agent)
grep -q "に最初のメッセージが届いていない: trust prompt。dispatcher が見回りで送り直す（.*急ぐときだけ手で: hc send $addr \"課題 \`demo/$s5\`。" "$tmp/err" \
  && [ "$(field "$s5" status)" = doing ] || die "案内: $(cat "$tmp/err")"
grep -q "^label $addr first_message.sent=false\$" "$tmp/hc/log" || die 'dispatcher に預けるラベルを付けていない'
grep -q "^run .*--cwd $PWD/kanban/issues/$s5 claude " "$tmp/hc/log" || die 'repo の無い課題の cwd が issues/<id>/ でない'
ok 'start: 最初のメッセージが届かなければ終了コード 3・端末にラベル first_message.sent=false（dispatcher が送り直す）と急ぐとき用の hc send（台帳は doing・agent はその端末）。repo の無い課題は issues/<id>/ で'

# start が作ったものを finish が片付ける
: > "$tmp/hc/log"
"$andon" issue finish "$s1" >/dev/null
[ -z "$(grep -v '^ls \|^kill \|^rm ' "$tmp/hc/log")" ] || die "finish が kill・rm（と ls）のほかに hc を打った: $(cat "$tmp/hc/log")"
[ ! -e "$wt" ] && ! git -C "$appdir" show-ref --quiet "refs/heads/$br" && [ "$(field "$s1" status)" = done ] && ! grep -q " $s1\$" "$tmp/hc/live" \
  || die "finish で片付かない: $(kanban show "$s1" --json | jq -c .issue)"
ok 'start が作った worktree・branch・端末を finish が片付ける（ラベルに触らない。hc は ls・kill・rm だけ）'

# 旧の置き場（~/.herdr/worktrees。ADR-0012 の前に起きた課題）の値の課題も finish が片付ける
s10=$(kanban add 'Old home' --repo github.com/example/app --profile claude --brief - <<<'# x')
owt=$HOME/.herdr/worktrees/app/demo-$s10
git -C "$appdir" worktree add -q -b "demo/$s10-old-home" "$owt" origin/main
kanban set "$s10" "branch=demo/$s10-old-home" "worktree=$owt" >/dev/null
(cd "$owt" && HC_TERMINAL_ID=t77 kanban start "$s10" >/dev/null)
"$andon" issue finish "$s10" >/dev/null || die "旧の置き場の課題を finish できない"
[ ! -e "$owt" ] && ! git -C "$appdir" show-ref --quiet "refs/heads/demo/$s10-old-home" && [ "$(field "$s10" status)" = done ] \
  || die "旧の置き場の worktree が片付かない: $(kanban show "$s10" --json | jq -c .issue)"
ok 'finish: 旧の置き場（~/.herdr/worktrees/<repo 名>/<slug>-<id>）の値の課題も片付ける'

# resume: 止まった担当を同じ worktree でセッションの続きから起こす（ADR-0012 決めたこと 3・7・11・13。受け入れ条件 1・2・3・5）
grep "issue=$s4 " "$tmp/hc/log" | grep -q 'session=' && die 'Codex 系の start がセッションの id を渡した（Codex は決める口が無い）'
r1=$(kanban add 'Resume me' --repo github.com/example/app --profile claude --brief - <<<'# Resume me')
expect_fail "は todo" "$andon" issue resume "$r1" --dry-run
"$andon" issue start "$r1" >/dev/null
sess=$(field "$r1" session); a1=$(field "$r1" agent); rwt=$ANDON_WORKSPACE/.worktrees/app/demo-$r1; rbr=demo/$r1-resume-me
[[ $sess =~ ^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$ ]] && grep "issue=$r1 " "$tmp/hc/log" | grep -q -- "--label session=$sess --cwd $rwt claude " \
  || die "start が Claude 系にセッションの id を渡して台帳に書いていない: session=$sess $(grep "issue=$r1 " "$tmp/hc/log")"
ok 'start: Claude 系はセッションの id を決めてラベル session で渡し、台帳の session に書く（Codex 系は渡さない）'
expect_fail "端末 $a1 がもう動いている" "$andon" issue resume "$r1" --dry-run
ok 'resume --dry-run: todo・端末が動いている課題では理由を言って止まる'

# 起き直し: 端末は素のシェルで戻り、worktree は消えた（.git/worktrees の登録は残る）
echo '{"state":"live","argv":["/bin/bash"],"foreground":{"argv":["/bin/bash"]},"labels":{}}' > "$tmp/hc/get-$a1"
rm -rf "$rwt"
expect_fail "セッション $sess の記録が無い" "$andon" issue resume "$r1" --dry-run
expect_fail "の記録が無い" "$andon" issue resume "$r1"
[ "$(field "$r1" agent)" = "$a1" ] && [ ! -e "$rwt" ] && ! grep -q "^run .*issue=$r1 .*claude-resume" "$tmp/hc/log" || die '記録が無いのに起こしたか台帳・worktree を変えた'
ok 'resume: Claude のセッションの記録が無ければ、起こさず台帳も worktree も変えずに非 0'
mkdir -p "$HOME/.claude/projects/-x" && : > "$HOME/.claude/projects/-x/$sess.jsonl"
hcn=$(wc -l < "$tmp/hc/log"); before=$(wc -l < kanban/log.jsonl)
"$andon" issue resume "$r1" --dry-run --json > "$tmp/dry" || die "resume --dry-run が落ちた"
! tail -n +"$((hcn + 1))" "$tmp/hc/log" | grep -qv '^get \|^ls ' && [ "$(wc -l < kanban/log.jsonl)" = "$before" ] && [ ! -e "$rwt" ] \
  || die "resume --dry-run が読むほかに hc を打ったか何かを作った: $(tail -n +"$((hcn + 1))" "$tmp/hc/log")"
jq -e --arg wt "$rwt" --arg br "$rbr" --arg s "$sess" --arg a "$a1" '.dry_run and .template == "claude-resume" and .agent == $a and .agent_state == "bare_shell"
  and .worktree == $wt and .cwd == $wt and .branch == $br and .restore == "add" and .session == $s and (.labels | index("session=" + $s) != null)
  and (.first_message | test("の続き。セルが起き直した。worktree は同じ場所に戻した。台帳（`deck list`）と git の状態を確かめ、続きから。"))' "$tmp/dry" >/dev/null \
  || die "resume --dry-run の計画: $(cat "$tmp/dry")"
ok 'resume --dry-run: doing で素のシェルの課題は、worktree のパス・branch・起こす template・cwd・session・最初のメッセージの計画を出す（hc は読むだけ）'

: > "$tmp/hc/log"
"$andon" issue resume "$r1" > "$tmp/out" || die "resume が落ちた: $(cat "$tmp/out")"
a2=$(field "$r1" agent)
[ -d "$rwt" ] && [ "$(git -C "$rwt" rev-parse --abbrev-ref HEAD)" = "$rbr" ] || die "worktree が戻っていない: $(cat "$tmp/out")"
[ "$a2" != "$a1" ] && [ "$(field "$r1" status)" = doing ] && [ "$(field "$r1" session)" = "$sess" ] && [ "$(field "$r1" worktree)" = "$rwt" ] \
  || die "台帳: $(kanban show "$r1" --json | jq -c .issue)"
order=$(grep -n "^keep $a1 --off\|^kill $a1\|^rm $a1\|^run " "$tmp/hc/log" | cut -d: -f2 | cut -d' ' -f1 | tr '\n' ' ')
[ "$order" = "keep kill rm run " ] || die "素のシェルを片付けてから起こしていない: $(cat "$tmp/hc/log")"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$(tr A-Z a-z <<<"$r1") --label project=demo --label issue=$r1 --label title=Resume me --label herdr.space=andon-demo-$r1 --label herdr.tab=andon-demo-$r1 --label session=$sess --cwd $rwt claude-resume 課題 \`demo/$r1\` の続き。" "$tmp/hc/log" \
  || die "hc run の形: $(grep ^run "$tmp/hc/log")"
jq -e --arg id "$r1" --arg a "$a2" 'select(.id == $id and .change.agent == $a) | .by == "terminal:t5"' kanban/log.jsonl >/dev/null || die '台帳の agent を新しい端末にしていない'
ok 'resume: worktree を同じパスに branch で戻し、素のシェルを片付け、同じ名前・ラベルで <profile>-resume を起こし、届いてから台帳の agent を書く（doing のまま・session は同じ）'

# 端末がまた消えた。worktree はあり、commit していない変更を置いた: 触らずに起こす
rm -f "$tmp/hc/get-$a1"; sed -i "/^$a2 /d" "$tmp/hc/live"
echo wip > "$rwt/wip.txt"
"$andon" issue resume "$r1" --dry-run --json | jq -e '.restore == "keep" and .agent_state == "not_found"' >/dev/null || die 'あれば keep の計画でない'
"$andon" issue resume "$r1" >/dev/null || die 'worktree があると resume できない'
[ "$(cat "$rwt/wip.txt")" = wip ] && [ "$(git -C "$rwt" status --porcelain)" = '?? wip.txt' ] || die 'commit していない変更に触った'
ok 'resume: worktree があれば（commit していない変更があっても）触らずに起こす。端末が無い（not_found）でも起こす'

# 最初のメッセージが届かない: 起こした端末を片付け、台帳を変えずに非 0
a3=$(field "$r1" agent); sed -i "/^$a3 /d" "$tmp/hc/live"
echo false > "$tmp/hc/first"
expect_fail "最初のメッセージが届かない" "$andon" issue resume "$r1"
rm "$tmp/hc/first"
bad=t$(( $(cat "$tmp/hc/next") - 1 ))   # 最後に起こした端末
[ "$(field "$r1" agent)" = "$a3" ] && grep -q "^kill $bad\$" "$tmp/hc/log" && grep -q "^rm $bad\$" "$tmp/hc/log" \
  || die "届かなかった端末を片付けていないか台帳を変えた: agent=$(field "$r1" agent) $(tail -5 "$tmp/hc/log")"
ok 'resume: 最初のメッセージが届かなければ、起こした端末を片付けて台帳を変えずに非 0'

# --fresh: 同じ worktree で新しいセッション。最初のメッセージに前の担当の報告を読む1行
: > "$tmp/hc/log"
"$andon" issue resume "$r1" --fresh > "$tmp/out" || die "--fresh が落ちた: $(cat "$tmp/out")"
s2=$(field "$r1" session)
[ "$s2" != "$sess" ] && [[ $s2 =~ ^[0-9a-f-]{36}$ ]] && grep -q -- "--label session=$s2 --cwd $rwt claude 課題 \`demo/$r1\`。" "$tmp/hc/log" \
  && grep -q "前の担当の報告 \`$PWD/kanban/issues/$r1/report.md\`（あれば）を読み" "$tmp/hc/log" && [ "$(cat "$rwt/wip.txt")" = wip ] \
  || die "--fresh: session=$s2 $(cat "$tmp/hc/log")"
ok 'resume --fresh: template の command（resume でない）・新しい session を台帳に・最初のメッセージに report.md を読む1行'

# Codex: 記録（sessions/**/rollout-*.jsonl の先頭行 session_meta）から cwd が同じ最新の id を引く
a4=$(field "$s4" agent); cwt=$(field "$s4" worktree); sed -i "/^$a4 /d" "$tmp/hc/live"
expect_fail "の Codex の記録が無い" "$andon" issue resume "$s4"
cs=$HOME/.codex/sessions/2026/11/10; mkdir -p "$cs"
meta() { jq -cn --arg id "$1" --arg cwd "$2" --arg at "$3" '{type: "session_meta", payload: {id: $id, cwd: $cwd, timestamp: $at, base_instructions: "x"}}'; }
old=11111111-1111-7111-8111-111111111111 new=22222222-2222-7222-8222-222222222222 other=33333333-3333-7333-8333-333333333333
{ meta "$old" "$cwt" 2026-11-10T01:00:00.000Z; echo '{"type":"x"}'; } > "$cs/rollout-a.jsonl"
meta "$new" "$cwt" 2026-11-10T02:00:00.000Z > "$cs/rollout-b.jsonl"
meta "$other" /elsewhere 2026-11-10T03:00:00.000Z > "$cs/rollout-c.jsonl"
: > "$tmp/hc/log"
"$andon" issue resume "$s4" >/dev/null || die 'Codex の resume が落ちた'
[ "$(field "$s4" session)" = "$new" ] && grep -q -- "--label session=$new --cwd $cwt codex-resume 課題 \`demo/$s4\` の続き。" "$tmp/hc/log" \
  || die "Codex の resume: session=$(field "$s4" session) $(cat "$tmp/hc/log")"
ok 'resume: Codex 系は記録の先頭行から cwd が同じ最新の id を引き、codex-resume で起こし、引けた id を台帳の session に書く。無ければ非 0'

# 旧の置き場（~/.herdr/worktrees）の値でも、その値のまま戻す
r2=$(kanban add 'Old place' --repo github.com/example/app --profile claude --brief - <<<'# x')
owt=$HOME/.herdr/worktrees/app/demo-$r2; s3=44444444-4444-4444-8444-444444444444
git -C "$appdir" worktree add -q -b "demo/$r2-old-place" "$owt" origin/main
(cd "$owt" && HC_TERMINAL_ID=t88 kanban start "$r2" >/dev/null)
kanban set "$r2" "session=$s3" >/dev/null; : > "$HOME/.claude/projects/-x/$s3.jsonl"
rm -rf "$owt"
"$andon" issue resume "$r2" >/dev/null || die '旧の置き場の値の課題を resume できない'
[ "$(git -C "$owt" rev-parse --abbrev-ref HEAD)" = "demo/$r2-old-place" ] && [ "$(field "$r2" worktree)" = "~/.herdr/worktrees/app/demo-$r2" ] \
  || die "旧の置き場に戻っていない: $(kanban show "$r2" --json | jq -c .issue)"
ok 'resume: 旧の置き場（~/.herdr/worktrees/…）の値でも、台帳の値のまま同じパスに戻す'

# 置き場の外の worktree・machine のある課題・レビュアーは止まる
r3=$(kanban add 'Elsewhere' --repo github.com/example/app --profile claude --brief - <<<'# x')
kanban set "$r3" status=doing "worktree=$tmp/elsewhere" "branch=demo/$r3" >/dev/null
expect_fail '置き場' "$andon" issue resume "$r3" --dry-run
kanban set "$r3" machine=vps >/dev/null
expect_fail 'machine=vps' "$andon" issue resume "$r3" --dry-run
[ ! -e "$tmp/elsewhere" ] || die '置き場の外に worktree を作った'
ok 'resume: 置き場の外の worktree の値・machine のある課題は理由を言って止まる'

# レビュアー: detached worktree を依頼書の commit で同じパスに戻し、cwd issues/<rev>/ で <profile>-resume を起こす（ADR-0012 決めたこと 8・13 の 5）
git -C "$appdir" commit -q --allow-empty -m reviewed; rc=$(git -C "$appdir" rev-parse HEAD); prev=$(git -C "$appdir" rev-parse HEAD~1)
rv=$(kanban add "$r1 のレビュー（2 回目）" --repo github.com/example/app --parent "$r1" --profile codex-review --round 2 --brief - <<EOF
# $r1 のレビュー（2 回目）
再レビュー: 前の回の commit $prev。見るのは \`git diff $prev..$rc\` だけ。
対象: worktree \`x\`（detached）。commit $rc。差分は \`gh pr diff x\`。
EOF
)
rvwt=$ANDON_WORKSPACE/.worktrees/app/demo-$rv; rvcwd=$PWD/kanban/issues/$rv
git -C "$appdir" worktree add -q --detach "$rvwt" "$rc"
(cd "$rvwt" && HC_TERMINAL_ID=t90 kanban start "$rv" >/dev/null)
rm -rf "$rvwt"
expect_fail "cwd $rvcwd の Codex の記録が無い" "$andon" issue resume "$rv" --dry-run
rvs=55555555-5555-7555-8555-555555555555
meta "$rvs" "$rvcwd" 2026-11-10T05:00:00.000Z > "$cs/rollout-r.jsonl"
"$andon" issue resume "$rv" --dry-run --json > "$tmp/dry" || die "レビュアーの resume --dry-run が落ちた"
jq -e --arg wt "$rvwt" --arg cwd "$rvcwd" --arg c "$rc" --arg s "$rvs" --arg id "$rv" --arg p "$r1" '.reviewer and .template == "codex-review-resume"
  and .worktree == $wt and .cwd == $cwd and .commit == $c and .restore == "add" and .branch == null and .session == $s
  and .labels == ["project=demo", "issue=\($id)", "title=\($p) のレビュー（2 回目）", "herdr.space=andon-demo-\($p)", "herdr.tab=andon-demo-\($p)", "session=\($s)"]' "$tmp/dry" >/dev/null \
  || die "レビュアーの resume の計画: $(cat "$tmp/dry")"
[ ! -e "$rvwt" ] || die 'dry-run が worktree を作った'
: > "$tmp/hc/log"
"$andon" issue resume "$rv" > "$tmp/out" || die "レビュアーの resume が落ちた: $(cat "$tmp/out")"
[ "$(git -C "$rvwt" rev-parse HEAD)" = "$rc" ] && ! git -C "$rvwt" symbolic-ref -q HEAD >/dev/null || die "detached worktree が commit $rc に戻っていない: $(cat "$tmp/out")"
grep -qF -- "--label session=$rvs --cwd $rvcwd codex-review-resume 課題 \`demo/$rv\` の続き。" "$tmp/hc/log" || die "レビュアーの hc run: $(grep ^run "$tmp/hc/log")"
[ "$(field "$rv" agent)" != t90 ] && [ "$(field "$rv" status)" = doing ] && [ "$(field "$rv" session)" = "$rvs" ] && [ "$(field "$rv" worktree)" = "$rvwt" ] && [ -z "$(field "$rv" branch)" ] \
  || die "レビュアーの台帳: $(kanban show "$rv" --json | jq -c .issue)"
ok 'resume: レビュアーは detached worktree を依頼書の対象の commit（前の回の commit でなく）で同じパスに戻し、cwd issues/<rev>/ で codex-review-resume を起こす'

# レビュアーの worktree があれば触らない。claude-review は台帳の session で claude-review-resume
sed -i "/^$(field "$rv" agent) /d" "$tmp/hc/live"; echo note > "$rvwt/note.txt"
rcs=66666666-6666-4666-8666-666666666666; : > "$HOME/.claude/projects/-x/$rcs.jsonl"
kanban set "$rv" profile=claude-review "session=$rcs" >/dev/null
"$andon" issue resume "$rv" --dry-run --json | jq -e --arg s "$rcs" '.restore == "keep" and .template == "claude-review-resume" and .session == $s' >/dev/null \
  || die "claude-review の resume の計画: $("$andon" issue resume "$rv" --dry-run --json)"
"$andon" issue resume "$rv" >/dev/null && [ "$(cat "$rvwt/note.txt")" = note ] || die 'レビュアーの worktree があると resume できないか、触った'
ok 'resume: レビュアーの worktree があれば触らずに起こす。claude-review は台帳の session で claude-review-resume'

# 依頼書に対象の commit が無い・parent が無いレビュアーは止まる
r4=$(kanban add "$r1 のレビュー" --repo github.com/example/app --parent "$r1" --profile codex-review --brief - <<<'# x')
kanban set "$r4" status=doing "worktree=$ANDON_WORKSPACE/.worktrees/app/demo-$r4" >/dev/null
expect_fail '対象の commit' "$andon" issue resume "$r4" --dry-run
r6=$(kanban add 'orphan' --repo github.com/example/app --profile codex-review --brief - <<<'# x')
kanban set "$r6" status=doing >/dev/null
expect_fail 'parent が無い' "$andon" issue resume "$r6" --dry-run
ok 'resume: 依頼書に対象の commit が無い・parent の無いレビュアーは理由を言って止まる'

# Chief Engineer: ラベルは指揮側の Space（role=chief-engineer・herdr.space=andon-<slug>）
r5=$(kanban add 'CE topic' --repo github.com/example/app --profile chief-engineer --brief - <<<'# x')
"$andon" issue start "$r5" >/dev/null
sed -i "/^$(field "$r5" agent) /d" "$tmp/hc/live"; : > "$HOME/.claude/projects/-x/$(field "$r5" session).jsonl"
"$andon" issue resume "$r5" --dry-run --json | jq -e --arg id "$r5" '.template == "chief-engineer-resume"
  and .labels[0:6] == ["role=chief-engineer", "project=demo", "issue=\($id)", "title=CE topic", "herdr.space=andon-demo", "herdr.tab=andon-demo-\($id)"]' >/dev/null \
  || die "Chief Engineer の resume の計画: $("$andon" issue resume "$r5" --dry-run --json)"
ok 'resume: Chief Engineer は chief-engineer-resume で、指揮側の Space のラベル（role=chief-engineer）で起こす'

# lock: start・finish・resume は同じ lock を待つ（dispatcher の控えの lock とは別）
lk=$HOME/.local/state/andon/dispatcher/demo.issue.lock
sed -i "/^$(field "$r1" agent) /d" "$tmp/hc/live"; : > "$HOME/.claude/projects/-x/$(field "$r1" session).jsonl"
flock "$lk" sleep 2 & lp=$!
sleep 0.3; t0=$(date +%s%N)
"$andon" issue resume "$r1" >/dev/null || die 'lock のあと resume が落ちた'
wait "$lp"; [ $(( ($(date +%s%N) - t0) / 1000000 )) -ge 1200 ] || die 'resume が lock を待たなかった'
flock -n "$HOME/.local/state/andon/dispatcher/demo.json.lock" true || die 'dispatcher の控えの lock を持ったまま'
ok 'resume: 台帳の issue lock を命令の間持つ（finish・start と同じ。dispatcher の控えの lock は取らない）'

echo "all passed ($pass)"
