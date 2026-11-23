#!/usr/bin/env bash
# `andon issue show/report/review-request/ready/ask/start`。本物の kanban の CLI・一時の kanban と git・偽の gh・偽の hc で回す。
# start は bare な origin とその clone（GHQ_ROOT）に worktree を切り、偽の hc で起動し、finish で片付くまで。本物の hc・kanban には触らない。
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
src=$(cd "$(dirname "$0")/.." && pwd)   # この repo（起動の記録の検査がパッケージの置き場を写す元）
# helper が go build したなら、キャッシュは一時ディレクトリにある
if [ -z "${ANDON_BIN:-}" ] && [ -z "${KANBAN_BIN:-}" ]; then
  [ -n "$(ls -A "$tmp/gocache" 2>/dev/null)" ] || { echo "FAIL go のキャッシュが一時ディレクトリに無い" >&2; exit 1; }
  echo "ok   go build のキャッシュは一時ディレクトリ（$tmp/gocache）"
fi
export GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset KANBAN_ROOT
mkdir -p "$HOME" "$tmp/demo" "$tmp/bin"
# 偽の gh: pr view <url> --json headRefOid -q .headRefOid → $tmp/prhead、pr view <url> --json state -q .state → $tmp/prstate。
# issue comment <url> --body <本文> は $tmp/gh-issue に「comment <url>」の行、本文を $tmp/gh-body に足す（外へ出さない）。
# PR のファイル（api --paginate --slurp …/files）は $tmp/prfiles、差分（pr diff）は空（review: off の課題のレベルを決めるのに読む）
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
case "\$1 \$2 \$4 \$5" in
  "pr view --json headRefOid") cat "$tmp/prhead" ;;
  "pr view --json state") cat "$tmp/prstate" ;;
  "api --paginate "*) cat "$tmp/prfiles" ;;
  "pr diff --repo "*) ;;
  "issue comment --body "*) [ "\$4" = --body ] && [ \$# = 5 ] || { echo "unexpected gh \$*" >&2; exit 99; }
    echo "comment \$3" >> "$tmp/gh-issue"; printf '%s\n' "\$5" >> "$tmp/gh-body" ;;
  *) echo "\$1 \$2 \$3" >> "$tmp/gh-issue"; echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"
prfiles() { jq -n --arg p "$1" '[[{filename: $p, additions: 3, deletions: 0}]]' > "$tmp/prfiles"; }   # PR が変えたファイル
prfiles docs/a.md
# review: off の課題のレベルは repo の package/gate.json で決める（gate と同じ条件表。start の場面はあとで写しの置き場に向け直す）
export ANDON_HOME="$src/package"

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
as() { HC_TERMINAL_ID=$1 "${@:2}"; }   # kanban の by を選ぶ（t5 が Chief of Staff、t7 が担当）
id=$(as t5 kanban add 'andon issue を足す' --profile claude)
expect_fail "$id は todo（andon issue ready は doing の課題にだけ打てる" "$andon" issue ready "$id"
ok 'ready: 定義（process/states/issue.yaml）に無い移り方（todo から ready）は断る（ADR-0017 受け入れ条件 11）'
as t5 kanban set "$id" status=doing review=on must_fix=major "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
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
expect_fail 'estimate は' "$andon" issue report "$id" --data '{"estimate":{"size":"中","reason":"x"}}' <<<'## Report'
[ ! -e "$lib/report.json" ] || die 'report.json が書かれた'
ok 'report: estimate の段が small・medium・large でなければ書く前に止まる'
expect_fail 'plan' "$andon" issue report "$id" --data '{"plan":[{"id":"p1","text":"x"}]}' <<<'## Report'
[ ! -e "$lib/report.json" ] || die 'report.json が書かれた'
ok 'report: plan[] に done が無ければ書く前に止まる'
"$andon" issue report "$id" --pr "$pr" --data '{"estimate":{"size":"large","reason":"直す所が3つの package にまたがる"},"plan":[{"id":"p1","text":"schema に足す。終了条件: tests/schema.sh が通る","done":true},{"id":"p2","text":"引き継ぐ。終了条件: tests/andon-issue.sh が通る","done":false}],"next":[{"to":"chief-of-staff","text":"マージ後に片付ける"}],"remember":["x"]}' <<<'## Report' >/dev/null
jq -e --arg c "$c1" --arg pr "$pr" '.commit == $c and .pr == $pr and .estimate.size == "large" and .next[0].to == "chief-of-staff" and (has("review_request") | not)' "$lib/report.json" >/dev/null
grep -q '## Report' "kanban/issues/$id/report.md"
emit "$lib/report.json" report-1.json
ok 'report: commit は HEAD、pr と --data を書き、kanban に報告する'
"$andon" issue report "$id" <<<'## Report' >/dev/null
jq -e --arg pr "$pr" '.pr == $pr and .estimate.size == "large" and ([.plan[] | select(.done)] | length) == 1 and (has("next") | not)' "$lib/report.json" >/dev/null
ok 'report: --pr を省くと前の pr を、--data に estimate・plan が無ければ前のそれを引き継ぎ、ほかは書き直す（ADR-0014 決めたこと 11 の 3・ADR-0017 決めたこと 10）'
"$andon" issue report "$id" --data '{"plan":[{"id":"p1","text":"schema に足す","done":true},{"id":"p2","text":"引き継ぐ","done":true}]}' <<<'## Report' >/dev/null
jq -e '([.plan[] | select(.done)] | length) == 2 and .estimate.size == "large"' "$lib/report.json" >/dev/null
ok 'report: --data の plan は前の plan を置き換える（進み具合は done の数）'

# review-request（1 回目）
"$andon" issue review-request "$id" <<<'## Report' >/dev/null
jq -e --arg c "$c1" '.review_request == {round: 1, limit: 2, commit: $c, previous: null, reviewer: "codex-review", extra: false}' "$lib/report.json" >/dev/null
head -2 "kanban/issues/$id/report.md" | grep -q "^1 回目／上限 2。commit $c1"
emit "$lib/report.json" report-review-request.json
ok 'review-request: 1 回目・前回 null・reviewer は実装と別ブランド・人用の行を先頭に'
cp "$lib/report.json" "$tmp/rr1.json"
"$andon" issue report "$id" --data '{"remember":["推敲した"]}' <<<'## Report（推敲した）' >/dev/null
jq -e --slurpfile was "$tmp/rr1.json" '.review_request == $was[0].review_request and .remember == ["推敲した"]' "$lib/report.json" >/dev/null || die "HEAD が同じ報告の打ち直しで review_request が消えた: $(jq -c .review_request "$lib/report.json")"
ok 'report: HEAD が前の report.json の commit と同じなら、打ち直しても review_request を引き継ぐ'

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
jq -e --arg c "$c2" '.commit == $c and (has("review_request") | not)' "$lib/report.json" >/dev/null || die "HEAD が進んだのに review_request を引き継いだ: $(jq -c .review_request "$lib/report.json")"
ok 'report: HEAD が前の report.json の commit と違えば、review_request を引き継がない'
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

# 結果を残さずに返したレビュアーの課題があっても、次の依頼の回は被らない（id <id>-rN が空いている）。
# 動いている（todo・doing）レビュアーの課題があると review-request は断るので、返した形にする。足したのは Chief of Staff でない端末で、
# 照合には数えない（後続の show・ready が rev2 を読む）が、回の id は使えない
c3=$(commit three)
echo "$c3" > "$tmp/prhead"
dead=$(as t7 kanban add "$id のレビュー（結果なし）" --parent "$id" --profile codex-review --round 5)
as t9 kanban start "$dead" >/dev/null
as t9 kanban review "$dead" >/dev/null
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
as t5 kanban set "$id2" status=doing review=off "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
"$andon" issue report "$id2" --pr "$pr" <<<'## Report' >/dev/null
expect_fail 'review が off で、差分はレベル A。gate はレビューを求めないので頼めない' "$andon" issue review-request "$id2" <<<'## Report'
# 受け取られていない問い（open・answered）があれば、ready は断って doing のまま（finish と kanban done と同じ確かめ）
kanban ask "$id2" 'どちらか' >/dev/null
expect_fail "受け取られていない問いがあるので review にしない: $id2#1（open）" "$andon" issue ready "$id2"
kanban answer "$id2#1" A >/dev/null
expect_fail "$id2#1（answered）" "$andon" issue ready "$id2"
[ "$(kanban show "$id2" --json | jq -r .issue.status)" = doing ] || die 'ready: 問いで断ったのに status が変わった'
kanban received "$id2#1" >/dev/null
ok 'ready: 受け取られていない問いがあれば断り、課題は doing のまま'
"$andon" issue ready "$id2" >/dev/null
ok 'ready: review: off で差分がレベル A ならレビューを照合しない。問いを受け取ったあとは通る。review-request は断る'

# review: off で始めたコードの課題でも、差分がレベル B・C なら gate はレビューを求める。ready は review に入る前に断り、
# review-request は Chief of Staff が review を書き直すのを待たずに受ける（review の値は off のまま）
id6=$(as t5 kanban add 'review: off のコードの課題' --profile claude)
as t5 kanban set "$id6" status=doing review=off must_fix=major "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
"$andon" issue report "$id6" --pr "$pr" <<<'## Report' >/dev/null
prfiles internal/issue/start_test.go
expect_fail 'review は off だが、差分がレベル B（実行面 internal/issue/start_test.go' "$andon" issue ready "$id6"
[ "$(kanban show "$id6" --json | jq -r .issue.status)" = doing ] || die 'ready: review: off で差分がレベル B なのに review にした'
"$andon" issue review-request "$id6" <<<'## Report' >/dev/null || die 'review-request: review: off で差分がレベル B なのに断った'
jq -e --arg c "$(git -C "$tmp/wt" rev-parse HEAD)" '.review_request.round == 1 and .review_request.commit == $c' "kanban/issues/$id6/library/report.json" >/dev/null || die 'review-request: review_request を書いていない'
grep -q 'review は off だが、差分がレベル B' "kanban/issues/$id6/report.md" || die 'review-request: レビュー依頼の行に review: off で頼んだ理由が無い'
[ "$(kanban show "$id6" --json | jq -r .issue.review)" = off ] || die 'review-request: review の値を書き換えた'
prfiles model/issue-map.txt
expect_fail '差分がレベル C' "$andon" issue ready "$id6"
prfiles docs/a.md
as t5 kanban done "$id6" >/dev/null
ok 'review: off でも差分がレベル B・C なら、ready はレビューを求めて断り、review-request は受ける（review は off のまま）'
# ready は前の head についての gate の問いを数えずに review にするが、越えるのはその問いだけ。ready が問いを確かめてから
# kanban review を打つまでの間に kanban ask で足された問いがあれば、review にせず doing のまま残す。
# 間に問いを足すのは、kanban review の手前で kanban ask を打つ包み（PATH の先頭の kanban）で写す
id5=$(as t5 kanban add '前の head の gate の問いと、あとから足された問い' --profile codex)
as t5 kanban set "$id5" status=doing review=off "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
"$andon" issue report "$id5" --pr "$pr" <<<'## Report' >/dev/null
HC_TERMINAL_ID=dispatcher kanban ask "$id5" 'マージするか' --kind line >/dev/null
mkdir -p "kanban/issues/$id5/library/questions"
# 事実の行は 2026-11-20 までの語で書いた古い問いの形（読む側は古い語も受ける。ADR-0040 決めたこと 7）
jq -n --arg r "ゲートの記録 $PWD/kanban/issues/$id5/library/gate/0000000000000000000000000000000000000001.json" \
    '{kind: "line", question: "マージするか", facts: [$r], options: [{label: "オーナーが PR を読んでマージする", pick: true}, {label: "担当に直させる"}], from: "gate"}' \
    > "kanban/issues/$id5/library/questions/1.json"
mkdir -p "$tmp/late-bin"
cat > "$tmp/late-bin/kanban" <<EOF
#!/bin/sh
[ "\$1" = review ] && "$kanban" ask "\$2" 'あとから足された問い' >/dev/null
exec "$kanban" "\$@"
EOF
chmod +x "$tmp/late-bin/kanban"
rc=0; PATH="$tmp/late-bin:$PATH" "$andon" issue ready "$id5" >"$tmp/out" 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q "$id5#2 (open)" "$tmp/err" || { cat "$tmp/err" >&2; die "ready: 確かめのあとに足された問いを越えた（rc $rc）"; }
grep -q "$id5#1" "$tmp/err" && die 'ready: 前の head の gate の問いで断った'
[ "$(kanban show "$id5" --json | jq -r .issue.status)" = doing ] || die 'ready: 確かめのあとに足された問いがあるのに review にした'
ok 'ready: 前の head の gate の問いだけを越え、確かめのあとに足された問いがあれば review にせず doing のまま'
kanban answer "$id5#2" A >/dev/null; kanban received "$id5#2" >/dev/null
"$andon" issue ready "$id5" >/dev/null
[ "$(kanban show "$id5" --json | jq -r .issue.status)" = review ] || die 'ready: 前の head の gate の問いだけが残るのに review にしない'
jq -se --arg t "$id5" '[.[] | select(.id == $t and .change.status == "review")] | last | .change.forced == [1]' kanban/log.jsonl >/dev/null ||
    die 'ready: 越えた前の head の gate の問いの番号を log.jsonl に残していない'
ok 'ready: 足された問いを受け取ったあとは、前の head の gate の問いだけを越えて review にし、越えた番号を log に残す'
expect_fail "$id2 は review（andon issue review-request は doing の課題にだけ打てる" "$andon" issue review-request "$id2" <<<'## Report'
[ "$(kanban show "$id2" --json | jq -r .issue.status)" = review ] || die '断ったのに status が変わった'
ok 'review-request: review の課題には打てない（定義に無い移り方）'
id3=$(as t5 kanban add 'Settings 無し')
as t5 kanban set "$id3" status=doing "worktree=$tmp/wt" repo=github.com/example/app review_rounds=1 >/dev/null
"$andon" issue report "$id3" --pr "$pr" <<<'## Report' >/dev/null
expect_fail 'review が on/off でない' "$andon" issue ready "$id3"
ok 'ready: issue.yaml の review が無ければ止まる'
as t5 kanban set "$id3" review=on >/dev/null
r3=$(as t5 kanban add "$id3 のレビュー" --parent "$id3" --profile codex-review)
mkdir -p "kanban/issues/$r3/library"; jq -n --arg c "$c1" '{commit: $c, findings: []}' > "kanban/issues/$r3/library/findings.json"
as t9 kanban start "$r3" >/dev/null; as t9 kanban review "$r3" >/dev/null
"$andon" issue review-request "$id3" <<<'## Report' >/dev/null
jq -e '.review_request | .round == 2 and .limit == 1 and .extra == true' "kanban/issues/$id3/library/report.json" >/dev/null
head -2 "kanban/issues/$id3/report.md" | grep -q '（追加）'
ok 'review-request: 上限を超えた回は extra'

# その commit を見ているレビュアーの課題がまだ動いている（todo・doing）間は、同じ commit で頼み直さない（回だけが進み、dispatcher は二重に起動しないので捨てる）
id4=$(as t5 kanban add '同じ commit の頼み直し')
as t5 kanban set "$id4" status=doing review=on "worktree=$tmp/wt" repo=github.com/example/app >/dev/null
"$andon" issue report "$id4" --pr "$pr" <<<'## Report' >/dev/null
"$andon" issue review-request "$id4" <<<'## Report' >/dev/null
head4=$(git -C "$tmp/wt" rev-parse HEAD)
r4=$(as t5 kanban add "$id4 のレビュー（1 回目）" --parent "$id4" --profile codex-review --round 1 --brief - <<<"対象: worktree \`x\`（detached）。commit $head4。差分は gh pr diff。")
cp "kanban/issues/$id4/report.md" "$tmp/report4.md"
expect_fail "commit ${head4:0:12} のレビューはもう頼んである（$r4 が todo）" "$andon" issue review-request "$id4" <<<'## Report'
as t9 kanban start "$r4" >/dev/null
expect_fail "もう頼んである（$r4 が doing）" "$andon" issue review-request "$id4" <<<'## Report'
jq -e '.review_request.round == 1' "kanban/issues/$id4/library/report.json" >/dev/null || die '断ったのに回が進んだ'
cmp -s "$tmp/report4.md" "kanban/issues/$id4/report.md" || die '断ったのに報告が書き直された'
ok 'review-request: その commit のレビュアーの課題が todo・doing の間は「もう頼んである」と断り、回も報告も変えない'
# 別の commit でも、レビュアーが動いている間は頼み直さない（レビュアーが 2 人走ると、照合が古いほうを読んで進めなくなる。ADR-0032 決めたこと 8）
c4=$(commit four)
echo "$c4" > "$tmp/prhead"
cp "kanban/issues/$id4/library/report.json" "$tmp/report4.json"
expect_fail "レビュアー $r4 がまだ動いている（doing。commit ${head4:0:12} を見ている）" "$andon" issue review-request "$id4" <<<'## Report'
cmp -s "$tmp/report4.json" "kanban/issues/$id4/library/report.json" || die '断ったのに report.json が書き直された'
cmp -s "$tmp/report4.md" "kanban/issues/$id4/report.md" || die '断ったのに報告が書き直された'
ok 'review-request: 古い commit のレビュアーの課題が動いている間は、新しい commit でも断り、report.json も報告も変えない'
# レビュアーが結果を残さずに返したあとの頼み直しは、新しい commit で今までどおり次の回になる
as t9 kanban review "$r4" >/dev/null
"$andon" issue review-request "$id4" <<<'## Report' >/dev/null
jq -e --arg c "$c4" '.review_request | .round == 2 and .commit == $c' "kanban/issues/$id4/library/report.json" >/dev/null
ok 'review-request: レビュアーが返したあと（結果なし）の頼み直しは断らない'
# 後続の検査は c2 の状態で続ける
git -C "$tmp/wt" reset -q --hard "$head4"
echo "$head4" > "$tmp/prhead"

# ask
q='{"kind":"users","question":"既定を変えるか","facts":["docs/design.md §1.10"],"options":[{"label":"変える","pick":true},{"label":"変えない"}],"remaining":1,"from":"issue"}'
"$andon" issue ask "$id" - <<<"$q" | grep -q "$id#1"
jq -e '.kind == "users" and .options[0].pick == true' "$lib/questions/1.json" >/dev/null
kanban show "$id" --json | jq -e '.questions[0] | .kind == "users" and (.text | contains("推奨案: 変える")) and (.text | contains("あと 1 問"))' >/dev/null
emit "$lib/questions/1.json" question.json
ok 'ask: 問いを kanban に置き、JSON を library/questions/<n>.json に'
"$andon" issue ask "$id" - <<<"$(jq -c '.kind = "line"' <<<"$q")" >/dev/null
kanban show "$id" --json | jq -e '.questions[1].kind == "line"' >/dev/null
jq -e '.kind == "line"' "$lib/questions/2.json" >/dev/null
ok 'ask: kind line も kanban に kind を付ける（`kanban ask --kind line`）'
for bad in '.options[1].pick = true' '.options = [.options[0]]' '.question = "a\nb"' '. + {"x": 1}' '.kind = "other"'; do
    expect_fail '問いの JSON の形が違う' "$andon" issue ask "$id" - <<<"$(jq -c "$bad" <<<"$q")"
done
[ "$(kanban show "$id" --json | jq '.questions | length')" = 2 ]
ok 'ask: 形の違う問いは kanban に書く前に止まる'

# ask --new: オーナーの課題と問いを 1 回で置く（ADR-0057 決めたこと 7。受け入れ条件 7・8・9）。Chief of Staff（t5）が打つ
count() { kanban list --all --json | jq length; }
nq=$(jq -nc '{kind: "promise", question: "外のメンテナーに約束の日を伝えるか", facts: ["T-0001（andon issue を足す）", "docs/a.md"],
  reason: "約束は外に出ると戻せない", options: [{label: "伝える", pick: true}, {label: "伝えない"}], from: "chief-of-staff"}')
n0=$(count)
out=$(as t5 "$andon" issue ask --new - <<<"$nq")
own=${out%#*}
[[ "$out" == "$own#1" ]] || die "ask --new: 出力が <id>#<n> でない: $out"
[ "$(count)" = $((n0 + 1)) ] || die 'ask --new: 課題が 1 つ増えていない'
kanban show "$own" --json | jq -e '.issue | .who == "person" and .status == "todo" and .title == "外のメンテナーに約束の日を伝えるか（オーナーの判断待ち）" and (has("profile") | not)' >/dev/null
kanban show "$own" --json | jq -e '.questions | length == 1 and .[0].status == "open" and .[0].kind == "promise"' >/dev/null
jq -e --argjson q "$nq" '. == $q' "kanban/issues/$own/library/questions/1.json" >/dev/null || die 'ask --new: questions/1.json が渡した JSON でない'
grep -qx 'kind: promise' "kanban/issues/$own/brief.md" && grep -qx '推奨案: 伝える' "kanban/issues/$own/brief.md" \
  && grep -q '^出どころ: T-0001（andon issue を足す） / docs/a.md$' "kanban/issues/$own/brief.md" || die "ask --new: 依頼書に種類・推奨案・出どころが無い: $(cat "kanban/issues/$own/brief.md")"
"$andon" waiting --json | jq -e --arg id "$own" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].question == ($id + "#1") and .[0].answerable == true' >/dev/null \
  || die "ask --new: 判断の行に出ない: $("$andon" waiting --json)"
ok 'ask --new: 課題を 1 つ足し、who: person・タイトル・依頼書（種類・推奨案）・開いた問い・questions/1.json を置き、andon waiting の判断の行に答えられる形で出る（受け入れ条件 7）'
for bad in '.options = [.options[0]]|options が2つ未満' '.kind = "other"|kind が'; do
    expect_fail "${bad#*|}" as t5 "$andon" issue ask --new - <<<"$(jq -c "${bad%|*}" <<<"$nq")"
done
expect_fail '問いの JSON の形が違う' as t5 "$andon" issue ask --new - <<<'オーナーに聞きたいことがある'
[ "$(count)" = $((n0 + 1)) ] || die 'ask --new: 形の違う問いで課題が増えた'
ok 'ask --new: 選択肢が 1 つ・知らない種類・JSON でない文は、終了コード 1 で課題を足さない（受け入れ条件 8）'
pre=$(as t5 kanban add '前の課題' --profile claude)
out=$(as t5 "$andon" issue ask --new --after "$pre" --parent "$id" - <<<"$(jq -c '.question = "前の課題のあとで go を出すか" | .kind = "line"' <<<"$nq")")
own2=${out%#*}
kanban show "$own2" --json | jq -e --arg pre "$pre" --arg p "$id" '(.issue | .status == "queued" and .after == [$pre] and .parent == $p and .who == "person") and .questions[0].status == "open"' >/dev/null \
  || die "ask --new --after: queued・after・parent・開いた問いでない: $(kanban show "$own2" --json)"
"$andon" waiting --json | jq -e --arg id "$own2" '[.items[] | select(.id == $id)] | length == 0' >/dev/null || die 'ask --new --after: 前の課題が片付く前に andon waiting に出た'
as t5 kanban done "$pre" >/dev/null
"$andon" waiting --json | jq -e --arg id "$own2" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].question == ($id + "#1")' >/dev/null \
  || die 'ask --new --after: 前の課題が片付いても判断の行に出ない'
ok 'ask --new --after --parent: queued で after と parent を持ち、問いを置き、前の課題が片付くまで andon waiting に出ず、片付くと判断の行に出る（受け入れ条件 9）'
pre2=$(as t5 kanban add '前の課題 2' --profile claude)
wq=$(as t5 kanban add '前を待つ担当の課題' --profile claude --after "$pre2")
[ "$(kanban show "$wq" --json | jq -r .issue.status)" = queued ] || die '担当の課題が queued で作られていない'
expect_fail "$wq は queued（queued の課題に問いを置けるのはオーナーの課題" "$andon" issue ask "$wq" - <<<"$q"
[ "$(kanban show "$wq" --json | jq '.questions | length')" = 0 ] || die 'queued の担当の課題に問いが置かれた'
as t5 kanban done "$pre2" >/dev/null
as t5 kanban done "$wq" >/dev/null
ok 'ask: queued の担当の課題には今までどおり問いを置かない（受け入れ条件 9）'

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
cands='{"candidates":[{"id":"c1","text":"docs/a.md:4 「依頼所」→「依頼書」","file":"docs/a.md","line":4,"in_scope":true},{"id":"c2","text":"README の段を書き直す","in_scope":false}]}'
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
# Chief of Staff が起き直してアドレスが変わり、前の Chief of Staff が書いた deliverable の照合が外れても、書き直す前に PR を求めない
sv2=$(as t5 kanban add '調べる: Chief of Staff の起き直しをまたぐ' --profile claude)
as t5 kanban set "$sv2" review=off deliverable=candidates "worktree=$tmp/wt2" repo=github.com/example/app >/dev/null
kanban start "$sv2" >/dev/null
kanban config set chief_of_staff=t6 >/dev/null
"$andon" issue report "$sv2" <<<'## Report' >/dev/null
expect_fail 'candidates.json が無い' "$andon" issue ready "$sv2"
"$andon" issue candidates "$sv2" - <<<'{"candidates":[]}' >/dev/null
"$andon" issue report "$sv2" <<<'## Report' >/dev/null
"$andon" issue ready "$sv2" >/dev/null
[ "$(kanban show "$sv2" --json | jq -r .issue.status)" = review ] || die 'ready（照合が外れた candidates）: review にならない'
kanban config set chief_of_staff=t5 >/dev/null
ok 'ready: deliverable の照合が外れても（Chief of Staff の起き直しのあと）PR を求めず、candidates.json を確かめて review にする'
# repo の無い調べる課題: commit を書かず、HEAD を照合しない。cwd は git の外（start と同じ issues/<id>/）で、candidates → report → ready まで通る。
# `kanban start` を git の checkout の中で打つと worktree が入るが（kanban が checkout の中にあるとき）、repo が無ければ repo の無い課題として読む
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

# record: 走らせた記録（contract.md §24。ADR-0035 決めたこと 2・8・11）。fx は doing・repo あり・worktree は wt2
expect_fail 'pass を期待したが終了コード 3（記録 runs/1.json は書いた）' "$andon" issue record "$fx" -- sh -c 'echo 落ちた; exit 3'
jq -e '.id == 1 and .exit_code == 3 and .expect == "pass" and .dirty == false and .output == "runs/1.log"' "$lfx/runs/1.json" >/dev/null || die 'record: 落ちた記録が違う'
grep -qx '落ちた' "$lfx/runs/1.log" || die 'record: 出力を残さない'
"$andon" issue record "$fx" -- git rev-parse HEAD >"$tmp/out"
grep -q "$fx/library/runs/2.json written（期待 pass・終了コード 0・commit " "$tmp/out" || die "record: 書いた行が無い: $(cat "$tmp/out")"
jq -e --arg c "$(git -C "$tmp/wt2" rev-parse HEAD)" '.id == 2 and .exit_code == 0 and .commit == $c and .dirty == false and .command == ["git", "rev-parse", "HEAD"]' "$lfx/runs/2.json" >/dev/null || die 'record: 通った記録が違う'
grep -qx "$(git -C "$tmp/wt2" rev-parse HEAD)" "$lfx/runs/2.log" || die 'record: worktree で走らせていない'
emit "$lfx/runs/2.json" run.json
ok 'record: worktree で走らせ、通っても落ちても記録と出力を書く（落ちたら終了コード 1）'
echo mistake > "$tmp/wt2/mistake"
"$andon" issue record "$fx" --expect fail -- test ! -e mistake >/dev/null
jq -e '.id == 3 and .expect == "fail" and .exit_code == 1 and .dirty == true' "$lfx/runs/3.json" >/dev/null || die 'record: 落ちることを期待した記録が違う'
emit "$lfx/runs/3.json" run-fail.json
expect_fail 'fail を期待したが終了コード 0' "$andon" issue record "$fx" --expect fail -- test -e mistake
rm "$tmp/wt2/mistake"
expect_fail '起動できない' "$andon" issue record "$fx" -- no-such-command-andon
[ ! -e "$lfx/runs/5.json" ] && [ ! -e "$lfx/runs/5.log" ] || die 'record: 起動できない command の記録を書いた'
ok 'record: --expect fail は落ちて 0・通って 1。起動できない command は記録しない'
expect_fail "$sv は review（andon issue record は doing の課題にだけ打てる" "$andon" issue record "$sv" -- true
ok 'record: doing でない課題には打てない'
nrr=$(as t5 kanban add '記録: repo 無し' --profile claude)
kanban start "$nrr" >/dev/null
"$andon" issue record "$nrr" -- true >/dev/null
jq -e 'has("commit") or has("dirty") | not' "kanban/issues/$nrr/library/runs/1.json" >/dev/null || die 'record: repo の無い課題に commit・dirty を書いた'
emit "kanban/issues/$nrr/library/runs/1.json" run-norepo.json
ok 'record: repo の無い課題の記録は commit・dirty を持たない'

# start: Chief of Staff（t5）が担当を起動する。案件の repo は bare な origin とその clone
export GHQ_ROOT="$tmp/ghq"
# start は起動する前に信頼の設定を足す（boot と同じ手）。一時の HOME に書かせ、本物の CLAUDE_CONFIG_DIR・セルの repo に触らない
unset CLAUDE_CONFIG_DIR
export ANDON_WORKSPACE="$tmp/plant"
# セルの repo（オーナーのもの）: owner が commit した project.json が、書いてよい repo（app だけ）を持つ
git init -q "$tmp/plant"; mkdir -p "$tmp/plant/projects/demo"
printf 'owner = "owner@example.com"\n' > "$tmp/plant/andon.toml"
# signals 入りでも start が通る（ADR-0011 受け入れ条件 12。signals を知らないバージョンは project.json ごと弾いて起動できなくなる）
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
  templates) echo '{"templates":[{"name":"claude","command":"claude --model opus --effort high"},{"name":"codex"},{"name":"claude-resume","command":"claude --model opus --effort high --resume"},{"name":"codex-resume"},{"name":"chief-engineer"},{"name":"chief-engineer-resume"},{"name":"codex-review-resume"},{"name":"claude-review-resume"}]}' ;;
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
# 起動の記録（contract.md §18。ADR-0028 受け入れ条件 1〜4）の土台: ハーネスはバージョンを返し、パッケージの置き場は
# この repo の package/ を HEAD から写して、その commit で焼いた印（andon version stamp）を付ける。auto memory の目録はセルの repo の memory/（git の外）
printf '#!/bin/sh\necho "9.9.9 (Claude Code)"\n' > "$tmp/bin/claude"; printf '#!/bin/sh\necho "codex-cli 9.9.9"\n' > "$tmp/bin/codex"; chmod +x "$tmp/bin/claude" "$tmp/bin/codex"
pkgc=$(git -C "$src" rev-parse HEAD)
mkdir -p "$tmp/pkg" && git -C "$src" archive HEAD package | tar -x -C "$tmp/pkg" --strip-components=1
export ANDON_HOME=$tmp/pkg
"$andon" version stamp "$pkgc" >/dev/null
echo 'memory/' >> "$tmp/plant/.git/info/exclude"; mkdir -p "$tmp/plant/memory"; echo '- [a](a.md) — 目録の1行' > "$tmp/plant/memory/MEMORY.md"
launches() { echo "kanban/issues/$1/library/launches.jsonl"; }
nlaunch() { if [ -e "$(launches "$1")" ]; then wc -l < "$(launches "$1")"; else echo 0; fi; }
# kanban は repos に案件の repo を持つ別の kanban（slug は demo）
mkdir -p "$tmp/start/demo" && cd "$tmp/start/demo"
KANBAN_ROOT= kanban init --prefix S --repo github.com/example/app --repo github.com/example/lib >/dev/null
export KANBAN_ROOT=$PWD HC_TERMINAL_ID=t5
kanban config set chief_of_staff=t5 >/dev/null
# 予約キーは Chief of Staff（t5）が書く。andon issue start は揃っていない課題を起動しない（担当が review-request・ready で止まる）
keys() { HC_TERMINAL_ID=t5 kanban set "$1" review=off must_fix=major merge=person estimate=small >/dev/null; }
s1=$(kanban add 'Add the button' --repo github.com/example/app --profile claude --brief - <<<'# Add the button')
keys "$s1"
# 乾いた実行: 本物の kanban の CLI で、hc を呼ばず、何も作らず kanban も書かずに、起動するときに渡すものを出す
hcn=$(cat "$tmp/hc/log" 2>/dev/null | wc -l || true); before=$(wc -l < kanban/log.jsonl)
"$andon" issue start "$s1" --dry-run --json > "$tmp/dry" || die "dry-run が落ちた"
[ "$(cat "$tmp/hc/log" 2>/dev/null | wc -l || true)" = "$hcn" ] && [ "$(wc -l < kanban/log.jsonl)" = "$before" ] && [ "$(field "$s1" status)" = todo ] && [ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s1" ] \
  || die "dry-run が hc を呼んだか何かを作った: $(cat "$tmp/hc/log")"
jq -e --arg id "$s1" --arg wt "$ANDON_WORKSPACE/.worktrees/app/demo-$s1" '.dry_run and .terminal == ("demo-" + ($id | ascii_downcase)) and .worktree == $wt and .cwd == $wt
  and .branch == "demo/\($id)-add-the-button" and .profile == "claude" and (.brief | startswith("# Add the button"))
  and ([.checks[] | select(.result == "ok") | .name] | index("delegation") != null)' "$tmp/dry" >/dev/null || die "dry-run の中身: $(cat "$tmp/dry")"
ok 'start --dry-run: hc を呼ばず worktree・kanban に触れずに、端末の名前・worktree・branch・プロファイル・依頼書・確かめの結果を出す'
"$andon" issue start "$s1" > "$tmp/out"
[ "$(jq -r .first_message "$tmp/dry")" = "$(printf '課題 `demo/%s`。\n依頼書は `%s`。' "$s1" "$(jq -r .brief_path "$tmp/dry")")" ] || die "dry-run の最初のメッセージの形: $(jq .first_message "$tmp/dry")"
lc=$(tr A-Z a-z <<<"$s1")
wt=$ANDON_WORKSPACE/.worktrees/app/demo-$s1
br=demo/$s1-add-the-button
[ -d "$wt" ] && [ "$(git -C "$wt" rev-parse --abbrev-ref HEAD)" = "$br" ] || die "worktree か branch が無い: $(cat "$tmp/out")"
[ -z "$(git -C "$wt" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null)" ] || die 'branch に upstream がある'
[ "$(field "$s1" status)" = doing ] && [ "$(field "$s1" agent)" = t42 ] && [ "$(field "$s1" branch)" = "$br" ] && [ "$(field "$s1" worktree)" = "$wt" ] \
  || die "kanban: $(kanban show "$s1" --json | jq -c .issue)"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$lc --label project=demo --label issue=$s1 --label title=Add the button --label herdr.space=andon-demo-$s1 --label herdr.tab=andon-demo-$s1 --label session=$(field "$s1" session) --cwd $wt claude 課題 \`demo/$s1\`。" "$tmp/hc/log" \
  || die "hc run の形が違う: $(grep ^run "$tmp/hc/log")"
! grep -q 'SKILL.md\|先に' "$tmp/hc/log" || die '最初のメッセージに skill を読ませる行が残る（ADR-0001）'
grep -q "$s1 を t42（demo-$lc・claude）で起動した。cwd $wt branch $br" "$tmp/out" || die "out: $(cat "$tmp/out")"
[ "$(tail -n 1 "$tmp/out")" = "andon issue: 出た場所 $s1 Add the button / $s1（t42）" ] || die "出た場所が最後に無い: $(cat "$tmp/out")"
jq -e --arg id "$s1" 'select(.id == $id and .change.status == "doing") | .by == "terminal:t42"' kanban/log.jsonl >/dev/null || die 'doing にしたのが担当の端末でない'
[ "$(cat "$ANDON_WORKSPACE/.worktrees/.gitignore")" = '*' ] && ! git -C "$ANDON_WORKSPACE" status --short | grep -q worktrees \
  || die "置き場の .gitignore が * でないか、セルの repo の git status に出る: $(git -C "$ANDON_WORKSPACE" status --short)"
git -C "$ANDON_WORKSPACE" diff --quiet HEAD -- .gitignore && [ ! -e "$ANDON_WORKSPACE/.gitignore" ] || die 'セルの追跡される .gitignore に触った'
ok 'start: 置き場はセルの repo の .worktrees/<repo 名>/<slug>-<id>。中の .gitignore（*）で git status に出ない（ADR-0012 決めたこと 6）'
ok 'start: worktree と branch（upstream 無し）・hc run（名前・ラベル・herdr.space と herdr.tab・cwd・最初のメッセージ）・kanban start で doing と agent・出た場所'

# 起動の記録（ADR-0028 受け入れ条件 1）: 1件増え、欄が全部ある
rec=$(launches "$s1")
[ "$(nlaunch "$s1")" = 1 ] || die "起動の記録が1件でない: $(cat "$rec" 2>/dev/null)"
jq -e --arg id "$s1" --arg pc "$pkgc" --arg pk "$ANDON_HOME" --arg wc "$(git -C "$tmp/plant" rev-parse HEAD)" --arg ws "$tmp/plant" --arg ac "$(git -C "$appdir" rev-parse origin/main)" \
  --arg s "$(field "$s1" session)" --arg wt "$wt" '.role == "worker" and .launch == "start" and .issue == $id and .profile == "claude" and .template == "claude"
  and .command == "claude --model opus --effort high" and .terminal == "t42" and .session == $s and .harness == {name: "claude", version: "9.9.9 (Claude Code)"}
  and .package == {root: $pk, commit: $pc, source: "image", drift: false, reasons: []} and .workspace == {path: $ws, commit: $wc, dirty: false}
  and .repo == {name: "github.com/example/app", path: $wt, commit: $ac, dirty: false}
  and .external == [] and .drift == false and .drift_reasons == [] and .memory_index == "- [a](a.md) — 目録の1行\n"
  and (.at | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"))' "$rec" >/dev/null || die "起動の記録の中身: $(cat "$rec")"
emit "$rec" launch-start.json
ok 'start: 起動の記録（library/launches.jsonl）が1件増える。起動行・ハーネスのバージョン・パッケージ・セルの repo・案件の repo の commit・memory の目録の写しが入る（ADR-0028 受け入れ条件 1）'
# バージョンから当時の文を復元できる（受け入れ条件 3）: 記録の commit の skill と、プロファイルが読む置き場の skill が同じ
diff <(git -C "$src" show "$(jq -r .package.commit "$rec"):package/skills/worker/SKILL.md") "$(jq -r .package.root "$rec")/skills/worker/SKILL.md" >/dev/null \
  || die "記録の commit の worker skill と置き場の worker skill が違う（食い違いの印は $(jq .drift "$rec")）"
ok 'start: 食い違いの印が無いとき、記録のパッケージの commit の git show と置き場の skill の diff が空（ADR-0028 受け入れ条件 3）'


expect_fail "は doing" "$andon" issue start "$s1"
[ "$(grep -c '^run' "$tmp/hc/log")" = 1 ] || die '二度起動した'
ok 'start: 起動済み（doing）は起動しない'

s2=$(kanban add '依頼書の無い課題' --repo github.com/example/app --profile claude)
expect_fail '依頼書（brief.md）が無い' "$andon" issue start "$s2"
s3=$(kanban add 'プロファイルの無いもの' --repo github.com/example/app --profile fable --brief - <<<'# x')
expect_fail 'プロファイル fable の template が無い' "$andon" issue start "$s3"
[ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s2" ] && [ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s3" ] || die '止まる前に worktree を作った'
ok 'start: 依頼書が無い・template が無いなら何も作らずに止まる'

# kanban の repos に無い repo（clone はある）・未完了の after・origin にだけある同名の branch は、何も作らずに止まる
git clone -q "$tmp/origin.git" "$GHQ_ROOT/github.com/example/other" 2>/dev/null
s6=$(kanban add 'Other repo' --profile claude --brief - <<<'# x')
kanban set "$s6" repo=github.com/example/other >/dev/null
expect_fail 'kanban の config.yaml の repos に無い' "$andon" issue start "$s6"
s7=$(kanban add 'Later' --repo github.com/example/app --profile claude --brief - <<<'# x')
kanban set "$s7" "after=$s2" >/dev/null
[ "$(field "$s7" status)" = todo ] || die "after を足しても todo のはず: $(field "$s7" status)"
expect_fail "after（$s2）が済んでいない" "$andon" issue start "$s7"
s8=$(kanban add 'Taken name' --repo github.com/example/app --profile claude --brief - <<<'# x')
keys "$s8"   # origin の branch は予約キーの確かめのあと（worktree を作る直前）に見る
git -C "$appdir" push -q origin "HEAD:refs/heads/demo/$s8-taken-name"
git -C "$appdir" update-ref -d "refs/remotes/origin/demo/$s8-taken-name"
expect_fail "branch demo/$s8-taken-name が origin にもうある" "$andon" issue start "$s8"
# kanban の config.yaml の repos にはあるが、project.json の repos に無い repo（このプロジェクトが書いてよい repo でない）
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
ok 'start: kanban か project.json（owner の commit）の repos に無い repo・未完了の after・origin にある同名の branch では、worktree・branch・kanban の欄・hc run を作らずに止まる'

# 予約キーが揃っていない課題（無いキー・担当の端末が書いたキー）は何も作らずに止まり、直す kanban set の行を最後に出す。
# 値はコマンドが決めない（無いキーは選ぶ場所を残す）。Chief of Staff がその行を埋めて打てば起動できる
s12=$(kanban add 'No keys' --repo github.com/example/app --profile claude --brief - <<<'# x')
HC_TERMINAL_ID=t7 kanban set "$s12" review=off must_fix=major merge=person >/dev/null
expect_fail '最後に書いたのが今の Chief of Staff の端末（t5）でない: review・must_fix・merge。' "$andon" issue start "$s12"
fix=$(sed -n 's/.*Chief of Staff の端末で打つ（<…> は brief.md の表から値を選んで埋める）: //p' "$tmp/err")
[ "$fix" = "kanban set $s12 review=off must_fix=major merge=person 'estimate=<small|medium|large>'" ] && grep -q '無い・知らない値: estimate。' "$tmp/err" \
  || die "直す行: $(cat "$tmp/err")"
[ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s12" ] && [ "$(field "$s12" status)" = todo ] && [ -z "$(field "$s12" estimate)" ] && ! grep -q "issue=$s12 " "$tmp/hc/log" \
  || die "予約キーが揃っていないのに作ったか、キーを書いた: $(kanban show "$s12" --json | jq -c .issue)"
eval "HC_TERMINAL_ID=t5 ${fix/<small|medium|large>/small}" >/dev/null
"$andon" issue start "$s12" --dry-run --json | jq -e '[.checks[] | select(.name == "settings" and .result == "ok")] | length == 1' >/dev/null \
  || die "直す行を打っても起動できない: $("$andon" issue start "$s12" --dry-run 2>&1)"
ok 'start: 予約キーが無いか最後に書いたのが今の Chief of Staff の端末でなければ、何も作らず値も書かずに止まり、直す kanban set の行を出す。埋めて打てば起動できる'

# hc run が落ちたら、作った worktree と branch を戻す
s4=$(kanban add 'Run fails' --repo github.com/example/app --profile codex --brief - <<<'# x')
keys "$s4"
echo 1 > "$tmp/hc/run-rc"
expect_fail 'hc run に失敗' "$andon" issue start "$s4"
rm "$tmp/hc/run-rc"
[ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$s4" ] && ! git -C "$appdir" show-ref --quiet "refs/heads/demo/$s4-run-fails" \
  && [ "$(field "$s4" status)" = todo ] && [ -z "$(field "$s4" branch)$(field "$s4" worktree)" ] || die "戻していない: $(kanban show "$s4" --json | jq -c .issue)"
[ "$(nlaunch "$s4")" = 0 ] || die "起動できなかったのに起動の記録を書いた: $(cat "$(launches "$s4")")"
# パッケージの置き場の1ファイルを手で書き換えてから起動する（受け入れ条件 4）
echo '手で足した行' >> "$ANDON_HOME/skills/worker/SKILL.md"
"$andon" issue start "$s4" >/dev/null || die 'やり直せない'
git -C "$src" show "$pkgc:package/skills/worker/SKILL.md" > "$ANDON_HOME/skills/worker/SKILL.md"
[ "$(nlaunch "$s4")" = 1 ] || die "やり直した start の起動の記録が1件でない"
jq -e '.drift == true and .package.drift == true and (.drift_reasons | index("パッケージ: 中身が焼いたときと違う（手で書き換えた・足した・消した）") != null)
  and .harness == {name: "codex", version: "codex-cli 9.9.9"} and .session == null and .memory_index == null and .command == null' "$(launches "$s4")" >/dev/null \
  || die "置き場を書き換えた起動の記録: $(cat "$(launches "$s4")")"
emit "$(launches "$s4")" launch-drift.json
ok 'start: 起動できなかったら起動の記録を書かない。パッケージの置き場の1ファイルを書き換えて起動すると食い違いの印が立つ（ADR-0028 受け入れ条件 4）。Codex は session・memory の目録が null、読めない起動行も null'
grep -q "issue=$s4 " "$tmp/hc/log" || die 'やり直した start が hc run を打っていない'
! grep "issue=$s4 " "$tmp/hc/log" | grep -q 'session=' || die 'Codex 系の start がセッションの id を渡した（Codex は決める口が無い）'
ok 'start: hc run が落ちたら worktree・branch・kanban の欄を戻す（やり直せる）'

# 最初のメッセージが届かない（信頼の画面など）
s5=$(kanban add 'Trust prompt' --profile claude --brief - <<<'# x')   # repo の無い課題: issues/<id>/ で起動する
HC_TERMINAL_ID=t5 kanban set "$s5" review=off estimate=small >/dev/null   # PR を出さない課題は must_fix・merge を求めない
echo false > "$tmp/hc/first"
rc=0; "$andon" issue start "$s5" >"$tmp/out" 2>"$tmp/err" || rc=$?
rm "$tmp/hc/first"
[ "$rc" = 3 ] || die "rc=$rc $(cat "$tmp/err")"
addr=$(field "$s5" agent)
grep -q "に最初のメッセージが届いていない: trust prompt。kanban は doing・agent $addr のまま.*dispatcher が見回りで送り直す（.*急ぐときだけ手で: hc send $addr '課題 \`demo/$s5\`。" "$tmp/err" \
  && [ "$(field "$s5" status)" = doing ] || die "案内: $(cat "$tmp/err")"
grep -q "^label $addr first_message.sent=false\$" "$tmp/hc/log" || die 'dispatcher に預けるラベルを付けていない'
grep -q "^run .*--cwd $PWD/kanban/issues/$s5 claude " "$tmp/hc/log" || die 'repo の無い課題の cwd が issues/<id>/ でない'
ok 'start: 最初のメッセージが届かなければ終了コード 3・端末にラベル first_message.sent=false（dispatcher が送り直す）と急ぐとき用の hc send（kanban は doing・agent はその端末）。repo の無い課題は issues/<id>/ で'
jq -e --arg a "$addr" '.repo == null and .terminal == $a and .role == "worker"' "$(launches "$s5")" >/dev/null || die "repo の無い課題の起動の記録: $(cat "$(launches "$s5")")"
emit "$(launches "$s5")" launch-norepo.json
ok 'start: 最初のメッセージが届かなくても起きた端末の起動の記録は書く。repo の無い課題は repo が null'

# 受け取られていない問いがあれば、finish は何も消さずに止まる（片付けは全部か無し。A-0002・A-0001 の repro-b1）。
# 消してから kanban done に断られると、doing のまま端末が死に、dispatcher の resume がやめた課題で担当を起動し直す
s11=$(kanban add 'Cancel me' --repo github.com/example/app --profile claude --brief - <<<'# x')
keys "$s11"
"$andon" issue start "$s11" >/dev/null
wt11=$(field "$s11" worktree); br11=$(field "$s11" branch); a11=$(field "$s11" agent)
HC_TERMINAL_ID=$a11 "$andon" issue ask "$s11" - <<<'{"kind":"users","question":"どちらにするか","options":[{"label":"A","pick":true},{"label":"B"}],"from":"issue"}' >/dev/null
checked() { # checked <題>: finish が断り、端末・worktree・branch・kanban がそのまま
    : > "$tmp/hc/log"
    expect_fail "$1" "$andon" issue finish "$s11"
    ! grep -q '^kill \|^rm ' "$tmp/hc/log" || die "断る finish が端末を消した: $(cat "$tmp/hc/log")"
    [ -d "$wt11" ] && git -C "$appdir" show-ref --quiet "refs/heads/$br11" && grep -q "^$a11 " "$tmp/hc/live" \
      && [ "$(field "$s11" status)" = doing ] && [ "$(field "$s11" agent)" = "$a11" ] && [ "$(field "$s11" worktree)" = "$wt11" ] \
      || die "断る finish が片付けかけを残した: $(kanban show "$s11" --json | jq -c .issue)"
}
checked "$s11#1（open）"
kanban answer "$s11#1" A >/dev/null
checked "$s11#1（answered）"
# `kanban done`（closing）も同じ問いで断る（確かめの形が食い違わない）
expect_fail 'questions not received' kanban done "$s11"
kanban received "$s11#1" >/dev/null
# レビュアーの子課題の問いでも、親の finish は何も消さない（子を片付けてから親で止まる、をしない）
r11=$(kanban add 'Review' --parent "$s11" --profile codex-review --brief - <<<'# x')
kanban ask "$r11" 'どちらか' >/dev/null
checked "$r11#1（open）"
[ "$(field "$r11" status)" != done ] || die '子課題を done にした'
kanban answer "$r11#1" x >/dev/null; kanban received "$r11#1" >/dev/null
"$andon" issue finish "$s11" >/dev/null || die '問いを受け取ったあとの finish が通らない'
[ ! -e "$wt11" ] && [ "$(field "$s11" status)" = done ] && [ "$(field "$r11" status)" = done ] && ! grep -q "^$a11 " "$tmp/hc/live" \
  || die "受け取ったあとに片付かない: $(kanban show "$s11" --json | jq -c .issue)"
ok 'finish: 課題かレビュアーの子課題に受け取られていない問い（open・answered）があれば、端末・worktree・branch・kanban に触らずに止まる（`kanban done` と同じ確かめ。受け取れば片付く）'

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

# finish の書き戻し（ADR-0011 決めたこと 15・19、ADR-0026 決めたこと 9）: write_back: true の出所（github-issues。repo が無ければ repos の全部）の
# issue の URL を links に持つ課題の finish で、その issue に1コメント（結果と PR の URL。課題 id は書かない）を1回だけ書く。状態は変えない（gh は issue comment だけ）
setsignals() {
  jq --argjson s "$1" '.signals = $s' "$tmp/plant/projects/demo/project.json" > "$tmp/p.json" && cp "$tmp/p.json" "$tmp/plant/projects/demo/project.json"
  GIT_AUTHOR_EMAIL=owner@example.com git -C "$tmp/plant" commit -qam "signals: $1"
}
wbissue() {  # wbissue <題> <links…>: links と report.json の pr を持つ、repo の無い課題
  local x; x=$(kanban add "$1" --profile claude); shift
  kanban set "$x" "links=$(IFS=,; echo "$*")" >/dev/null
  mkdir -p "kanban/issues/$x/library"; jq -n --arg pr "$pr" '{pr: $pr}' > "kanban/issues/$x/library/report.json"
  echo "$x"
}
rm -f "$tmp/gh-issue" "$tmp/gh-body"
setsignals '[{"source": "github-issues", "weight": "later", "write_back": true}]'
iss=https://github.com/example/app/issues/3
w1=$(wbissue 'Write back' "$iss" "$iss" https://github.com/example/other/issues/4 https://example.com/x)
echo MERGED > "$tmp/prstate"
"$andon" issue finish "$w1" > "$tmp/out" || die "write_back の課題を finish できない"
[ "$(field "$w1" status)" = done ] && [ "$(cat "$tmp/gh-issue")" = "comment $iss" ] || die "1コメントでない・repos に無い repo にも書いた・状態に触れた: $(cat "$tmp/gh-issue")"
[ "$(cat "$tmp/gh-body")" = "$(printf 'この Issue に対応する変更がマージされた。\nPR: %s' "$pr")" ] || die "マージの本文が違う: $(cat "$tmp/gh-body")"
grep -q "$w1" "$tmp/out" && ! grep -qE '[A-Z]-[0-9]{4}|課題' "$tmp/gh-body" || die "外の Issue への本文に課題番号: $(cat "$tmp/gh-body")"
grep -q "$w1 の結果を $iss に書いた" "$tmp/out" || die "書いたことを出していない: $(cat "$tmp/out")"
expect_fail "$w1 は done" "$andon" issue finish "$w1"
[ "$(wc -l < "$tmp/gh-issue")" = 1 ] || die '2回目の finish で書いた'
ok 'finish: write_back: true の出所の issue に1コメント（マージされた・PR の URL。課題番号は書かない）を1回だけ書く（状態は変えない。repos に無い repo・トラッカーでない links には書かない。ADR-0011 受け入れ条件 13・ADR-0026 A7）'

: > "$tmp/gh-issue"; : > "$tmp/gh-body"
echo CLOSED > "$tmp/prstate"
setsignals '[{"source": "github-issues", "repo": "github.com/example/app", "weight": "later", "write_back": true}]'
w2=$(wbissue 'Closed' "$iss")
"$andon" issue finish "$w2" >/dev/null
[ "$(cat "$tmp/gh-issue")" = "comment $iss" ] && [ "$(cat "$tmp/gh-body")" = "$(printf 'この Issue に対応する変更は、マージされずに閉じられた。\nPR: %s' "$pr")" ] || die "閉じた PR の結果: $(cat "$tmp/gh-body")"
! grep -qE '[A-Z]-[0-9]{4}|課題' "$tmp/gh-body" || die "外の Issue への本文に課題番号: $(cat "$tmp/gh-body")"
ok 'finish: マージされずに閉じた PR は「閉じられた」と書く（出所の repo を書いたときはその repo の issue に。課題番号は書かない）'

: > "$tmp/gh-issue"; : > "$tmp/gh-body"
w6=$(wbissue 'No PR' "$iss")
echo '{}' > "kanban/issues/$w6/library/report.json"
"$andon" issue finish "$w6" >/dev/null
[ "$(cat "$tmp/gh-issue")" = "comment $iss" ] && [ "$(cat "$tmp/gh-body")" = 'この Issue に対応する作業は、変更（PR）を作らずに終えた。' ] || die "PR の無い課題の本文: $(cat "$tmp/gh-body")"
grep -q 'この Issue' "$tmp/gh-body" && ! grep -qE "$w6|課題" "$tmp/gh-body" || die "外の Issue への本文に課題番号: $(cat "$tmp/gh-body")"
ok 'finish: PR の無い課題は「変更を作らずに終えた」と書く（課題番号は書かない。ADR-0026 A7）'

for sig in '[{"source": "github-issues", "weight": "later", "write_back": false}]' \
           '[{"source": "github-issues", "weight": "later"}]' \
           '[{"source": "github-ci", "weight": "now", "write_back": true}]'; do
  : > "$tmp/gh-issue"
  setsignals "$sig"
  w3=$(wbissue 'No write back' "$iss")
  "$andon" issue finish "$w3" >/dev/null
  [ "$(field "$w3" status)" = done ] && [ ! -s "$tmp/gh-issue" ] || die "write_back が true の github-issues でないのに書いた（$sig）: $(cat "$tmp/gh-issue")"
done
ok 'finish: write_back が無い・false・github-issues でない出所なら書かない'

: > "$tmp/gh-issue"
setsignals '[{"source": "github-issues", "weight": "later", "write_back": true}]'
w5=$(wbissue 'PR still open' "$iss")
echo OPEN > "$tmp/prstate"
"$andon" issue finish "$w5" > "$tmp/out" 2>/dev/null || die 'PR が OPEN だと finish が落ちた'
[ "$(field "$w5" status)" = done ] && [ ! -s "$tmp/gh-issue" ] && grep -q "トラッカーへ書けなかった（PR が MERGED でも CLOSED でもない" "$tmp/out" || die "OPEN の PR で書いた・言わない: $(cat "$tmp/out")"
ok 'finish: PR が OPEN（MERGED でも CLOSED でもない）なら「閉じ」と書かずに1行出す（レビュー A-0030-r1）'
w4=$(wbissue 'PR unreadable' "$iss")
rm "$tmp/prstate"
"$andon" issue finish "$w4" > "$tmp/out" 2>/dev/null || die 'PR が読めないと finish が落ちた'
[ "$(field "$w4" status)" = done ] && [ ! -s "$tmp/gh-issue" ] && grep -q "トラッカーへ書けなかった（PR を読めない" "$tmp/out" || die "PR が読めないのに書いた・言わない: $(cat "$tmp/out")"
ok 'finish: PR の状態が読めなければ書かずに1行出す（結果を書き違えない。片付けは戻さない）'

# resume: 止まった担当を同じ worktree でセッションの続きから起動する（ADR-0012 決めたこと 3・7・11・13。受け入れ条件 1・2・3・5）
r1=$(kanban add 'Resume me' --repo github.com/example/app --profile claude --brief - <<<'# Resume me')
keys "$r1"
expect_fail "は todo" "$andon" issue resume "$r1" --dry-run
"$andon" issue start "$r1" >/dev/null
sess=$(field "$r1" session); a1=$(field "$r1" agent); rwt=$ANDON_WORKSPACE/.worktrees/app/demo-$r1; rbr=demo/$r1-resume-me
[[ $sess =~ ^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$ ]] && grep "issue=$r1 " "$tmp/hc/log" | grep -q -- "--label session=$sess --cwd $rwt claude " \
  || die "start が Claude 系にセッションの id を渡して kanban に書いていない: session=$sess $(grep "issue=$r1 " "$tmp/hc/log")"
ok 'start: Claude 系はセッションの id を決めてラベル session で渡し、kanban の session に書く（Codex 系は渡さない）'
r1first=$(head -n 1 "$(launches "$r1")")
expect_fail "端末 $a1 がもう動いている" "$andon" issue resume "$r1" --dry-run
ok 'resume --dry-run: todo・端末が動いている課題では理由を言って止まる'

# 起き直し: 端末は素のシェルで戻り、worktree は消えた（.git/worktrees の登録は残る）
echo '{"state":"live","argv":["/bin/bash"],"foreground":{"argv":["/bin/bash"]},"labels":{}}' > "$tmp/hc/get-$a1"
rm -rf "$rwt"
expect_fail "セッション $sess の記録が無い" "$andon" issue resume "$r1" --dry-run
expect_fail "の記録が無い" "$andon" issue resume "$r1"
[ "$(field "$r1" agent)" = "$a1" ] && [ ! -e "$rwt" ] && ! grep -q "^run .*issue=$r1 .*claude-resume" "$tmp/hc/log" || die '記録が無いのに起動したか kanban・worktree を変えた'
ok 'resume: Claude のセッションの記録が無ければ、起動せず kanban も worktree も変えずに非 0'
mkdir -p "$HOME/.claude/projects/-x" && : > "$HOME/.claude/projects/-x/$sess.jsonl"
hcn=$(wc -l < "$tmp/hc/log"); before=$(wc -l < kanban/log.jsonl)
"$andon" issue resume "$r1" --dry-run --json > "$tmp/dry" || die "resume --dry-run が落ちた"
! tail -n +"$((hcn + 1))" "$tmp/hc/log" | grep -qv '^get \|^ls ' && [ "$(wc -l < kanban/log.jsonl)" = "$before" ] && [ ! -e "$rwt" ] \
  || die "resume --dry-run が読むほかに hc を打ったか何かを作った: $(tail -n +"$((hcn + 1))" "$tmp/hc/log")"
jq -e --arg wt "$rwt" --arg br "$rbr" --arg s "$sess" --arg a "$a1" '.dry_run and .template == "claude-resume" and .agent == $a and .agent_state == "bare_shell"
  and .worktree == $wt and .cwd == $wt and .branch == $br and .restore == "add" and .session == $s and (.labels | index("session=" + $s) != null)
  and (.first_message | test("の続き。セルが起き直した。worktree は同じ場所に戻した。`kanban list` と git で今の状態を確かめ、続きから。"))' "$tmp/dry" >/dev/null \
  || die "resume --dry-run の計画: $(cat "$tmp/dry")"
ok 'resume --dry-run: doing で素のシェルの課題は、worktree のパス・branch・起動する template・cwd・session・最初のメッセージの計画を出す（hc は読むだけ）'

: > "$tmp/hc/log"
"$andon" issue resume "$r1" > "$tmp/out" || die "resume が落ちた: $(cat "$tmp/out")"
a2=$(field "$r1" agent)
[ -d "$rwt" ] && [ "$(git -C "$rwt" rev-parse --abbrev-ref HEAD)" = "$rbr" ] || die "worktree が戻っていない: $(cat "$tmp/out")"
[ "$a2" != "$a1" ] && [ "$(field "$r1" status)" = doing ] && [ "$(field "$r1" session)" = "$sess" ] && [ "$(field "$r1" worktree)" = "$rwt" ] \
  || die "kanban: $(kanban show "$r1" --json | jq -c .issue)"
order=$(grep -n "^keep $a1 --off\|^kill $a1\|^rm $a1\|^run " "$tmp/hc/log" | cut -d: -f2 | cut -d' ' -f1 | tr '\n' ' ')
[ "$order" = "keep kill rm run " ] || die "素のシェルを片付けてから起動していない: $(cat "$tmp/hc/log")"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$(tr A-Z a-z <<<"$r1") --label project=demo --label issue=$r1 --label title=Resume me --label herdr.space=andon-demo-$r1 --label herdr.tab=andon-demo-$r1 --label session=$sess --cwd $rwt claude-resume 課題 \`demo/$r1\` の続き。" "$tmp/hc/log" \
  || die "hc run の形: $(grep ^run "$tmp/hc/log")"
jq -e --arg id "$r1" --arg a "$a2" 'select(.id == $id and .change.agent == $a) | .by == "terminal:t5"' kanban/log.jsonl >/dev/null || die 'kanban の agent を新しい端末にしていない'
ok 'resume: worktree を同じパスに branch で戻し、素のシェルを片付け、同じ名前・ラベルで <profile>-resume を起動し、届いてから kanban の agent を書く（doing のまま・session は同じ）'
r1rec=$(launches "$r1")
[ "$(nlaunch "$r1")" = 2 ] && [ "$(head -n 1 "$r1rec")" = "$r1first" ] || die "起動し直しで起動の記録が2件にならないか、1件目が変わった: $(cat "$r1rec")"
tail -n 1 "$r1rec" | jq -e --arg a "$a2" --arg s "$sess" '.launch == "resume" and .role == "worker" and .template == "claude-resume" and .command == "claude --model opus --effort high --resume"
  and .terminal == $a and .session == $s' >/dev/null || die "起動し直しの記録: $(tail -n 1 "$r1rec")"
tail -n 1 "$r1rec" > "$tmp/l.json"; emit "$tmp/l.json" launch-resume.json
ok 'resume: 起動の記録がもう1件増え（launch resume・template claude-resume）、前の1件は変わらない（ADR-0028 受け入れ条件 2）'

# 端末がまた消えた。worktree はあり、commit していない変更を置いた: 触らずに起動する
rm -f "$tmp/hc/get-$a1"; sed -i "/^$a2 /d" "$tmp/hc/live"
echo wip > "$rwt/wip.txt"
"$andon" issue resume "$r1" --dry-run --json | jq -e '.restore == "keep" and .agent_state == "not_found"' >/dev/null || die 'あれば keep の計画でない'
"$andon" issue resume "$r1" >/dev/null || die 'worktree があると resume できない'
[ "$(cat "$rwt/wip.txt")" = wip ] && [ "$(git -C "$rwt" status --porcelain)" = '?? wip.txt' ] || die 'commit していない変更に触った'
ok 'resume: worktree があれば（commit していない変更があっても）触らずに起動する。端末が無い（not_found）でも起動する'

# 最初のメッセージが届かない: 起動した端末を片付け、kanban を変えずに非 0
a3=$(field "$r1" agent); sed -i "/^$a3 /d" "$tmp/hc/live"
echo false > "$tmp/hc/first"
expect_fail "最初のメッセージが届かない" "$andon" issue resume "$r1"
rm "$tmp/hc/first"
bad=t$(( $(cat "$tmp/hc/next") - 1 ))   # 最後に起動した端末
[ "$(field "$r1" agent)" = "$a3" ] && grep -q "^kill $bad\$" "$tmp/hc/log" && grep -q "^rm $bad\$" "$tmp/hc/log" \
  || die "届かなかった端末を片付けていないか kanban を変えた: agent=$(field "$r1" agent) $(tail -5 "$tmp/hc/log")"
[ "$(nlaunch "$r1")" = 3 ] || die "片付けた起動（最初のメッセージが届かない）を記録した: $(nlaunch "$r1") 件"
ok 'resume: 最初のメッセージが届かなければ、起動した端末を片付けて kanban を変えずに非 0（起動の記録も足さない）'

# --fresh: 同じ worktree で新しいセッション。最初のメッセージに前の担当の報告を読む1行
: > "$tmp/hc/log"
"$andon" issue resume "$r1" --fresh > "$tmp/out" || die "--fresh が落ちた: $(cat "$tmp/out")"
s2=$(field "$r1" session)
[ "$s2" != "$sess" ] && [[ $s2 =~ ^[0-9a-f-]{36}$ ]] && grep -q -- "--label session=$s2 --cwd $rwt claude 課題 \`demo/$r1\`。" "$tmp/hc/log" \
  && grep -q "前の担当の報告 \`$PWD/kanban/issues/$r1/report.md\`（あれば）を読み" "$tmp/hc/log" && [ "$(cat "$rwt/wip.txt")" = wip ] \
  || die "--fresh: session=$s2 $(cat "$tmp/hc/log")"
tail -n 1 "$(launches "$r1")" | jq -e --arg s "$s2" '.launch == "fresh" and .template == "claude" and .session == $s' >/dev/null || die "--fresh の起動の記録: $(tail -n 1 "$(launches "$r1")")"
ok 'resume --fresh: template の command（resume でない）・新しい session を kanban に・最初のメッセージに report.md を読む1行。起動の記録は launch fresh'

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
tail -n 1 "$(launches "$s4")" | jq -e --arg s "$new" '.launch == "resume" and .template == "codex-resume" and .session == $s' >/dev/null || die "Codex の起動し直しの記録: $(tail -n 1 "$(launches "$s4")")"
ok 'resume: Codex 系は記録の先頭行から cwd が同じ最新の id を引き、codex-resume で起動し、引けた id を kanban の session に書く。無ければ非 0。起動の記録の session は引けた id'

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
ok 'resume: 旧の置き場（~/.herdr/worktrees/…）の値でも、kanban の値のまま同じパスに戻す'

# 置き場の外の worktree・machine のある課題・レビュアーは止まる
r3=$(kanban add 'Elsewhere' --repo github.com/example/app --profile claude --brief - <<<'# x')
kanban set "$r3" status=doing "worktree=$tmp/elsewhere" "branch=demo/$r3" >/dev/null
expect_fail '置き場' "$andon" issue resume "$r3" --dry-run
kanban set "$r3" machine=vps >/dev/null
expect_fail 'machine=vps' "$andon" issue resume "$r3" --dry-run
[ ! -e "$tmp/elsewhere" ] || die '置き場の外に worktree を作った'
ok 'resume: 置き場の外の worktree の値・machine のある課題は理由を言って止まる'

# レビュアー: detached worktree を依頼書の commit で同じパスに戻し、cwd issues/<rev>/ で <profile>-resume を起動する（ADR-0012 決めたこと 8・13 の 5）
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
  || die "レビュアーの kanban: $(kanban show "$rv" --json | jq -c .issue)"
jq -e --arg c "$rc" --arg wt "$rvwt" '.role == "reviewer" and .launch == "resume" and .repo.commit == $c and .repo.path == $wt' "$(launches "$rv")" >/dev/null \
  || die "レビュアーの起動し直しの記録: $(cat "$(launches "$rv")")"
ok 'resume: レビュアーは detached worktree を依頼書の対象の commit（前の回の commit でなく）で同じパスに戻し、cwd issues/<rev>/ で codex-review-resume を起動する。起動の記録は役 reviewer・repo の commit は対象の commit'

# レビュアーの worktree があれば触らない。claude-review は kanban の session で claude-review-resume
sed -i "/^$(field "$rv" agent) /d" "$tmp/hc/live"; echo note > "$rvwt/note.txt"
rcs=66666666-6666-4666-8666-666666666666; : > "$HOME/.claude/projects/-x/$rcs.jsonl"
kanban set "$rv" profile=claude-review "session=$rcs" >/dev/null
"$andon" issue resume "$rv" --dry-run --json | jq -e --arg s "$rcs" '.restore == "keep" and .template == "claude-review-resume" and .session == $s' >/dev/null \
  || die "claude-review の resume の計画: $("$andon" issue resume "$rv" --dry-run --json)"
"$andon" issue resume "$rv" >/dev/null && [ "$(cat "$rvwt/note.txt")" = note ] || die 'レビュアーの worktree があると resume できないか、触った'
ok 'resume: レビュアーの worktree があれば触らずに起動する。claude-review は kanban の session で claude-review-resume'

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
"$andon" issue resume "$r5" >/dev/null || die 'Chief Engineer の resume が落ちた'
[ "$(nlaunch "$r5")" = 2 ] && jq -s -e '[.[].role] == ["chief-engineer", "chief-engineer"] and [.[].launch] == ["start", "resume"]' "$(launches "$r5")" >/dev/null \
  || die "Chief Engineer の起動の記録: $(cat "$(launches "$r5")")"
ok 'resume: Chief Engineer は chief-engineer-resume で、指揮側の Space のラベル（role=chief-engineer）で起動する。起動の記録の役は chief-engineer'

# lock: start・finish・resume は同じ lock を待つ（dispatcher の控えの lock とは別）
lk=$HOME/.local/state/andon/dispatcher/demo.issue.lock
sed -i "/^$(field "$r1" agent) /d" "$tmp/hc/live"; : > "$HOME/.claude/projects/-x/$(field "$r1" session).jsonl"
flock "$lk" sleep 2 & lp=$!
sleep 0.3; t0=$(date +%s%N)
"$andon" issue resume "$r1" >/dev/null || die 'lock のあと resume が落ちた'
wait "$lp"; [ $(( ($(date +%s%N) - t0) / 1000000 )) -ge 1200 ] || die 'resume が lock を待たなかった'
flock -n "$HOME/.local/state/andon/dispatcher/demo.json.lock" true || die 'dispatcher の控えの lock を持ったまま'
ok 'resume: kanban の issue lock を命令の間持つ（finish・start と同じ。dispatcher の控えの lock は取らない）'

# andon chief-engineer（ADR-0028 受け入れ条件 2）: Chief Engineer の課題に起動の記録が1件
"$andon" chief-engineer --repo github.com/example/app 'Design topic' > "$tmp/out" || die "andon chief-engineer が落ちた: $(cat "$tmp/out")"
ce=$(kanban list --json | jq -r '.[] | select(.title == "Design topic") | .id')
[ -n "$ce" ] && [ "$(nlaunch "$ce")" = 1 ] && jq -e --arg id "$ce" --arg s "$(field "$ce" session)" --arg wt "$(field "$ce" worktree)" \
  '.role == "chief-engineer" and .launch == "start" and .issue == $id and .profile == "chief-engineer" and .session == $s and .repo.path == $wt and .repo.commit != null' "$(launches "$ce")" >/dev/null \
  || die "andon chief-engineer の起動の記録: $(cat "$(launches "$ce")" 2>/dev/null) $(cat "$tmp/out")"
emit "$(launches "$ce")" launch-chief-engineer.json
ok 'andon chief-engineer: Chief Engineer の課題に起動の記録が1件（役 chief-engineer・session・worktree の commit。ADR-0028 受け入れ条件 2）'

echo "all passed ($pass)"
