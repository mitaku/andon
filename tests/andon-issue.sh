#!/usr/bin/env bash
# `andon issue show/report/review-request/ready/ask/start`。本物の kanban の CLI・一時の kanban と git・偽の gh・偽の hc で回す。
# start は bare な origin とその clone（GHQ_ROOT）に worktree を切り、偽の hc で起動し、finish で片付くまで。本物の hc・kanban には触らない。
# ANDON_EMIT_DIR があれば、command が書いた JSON をそこに写す（tests/schema.sh が schema に通す）。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT   # Go が落とす toolchain は読むだけで置かれる
# go の shim（mise）は道具を HOME の下から引く。HOME を一時に向ける前に go の実体を PATH の先に置き、一時の HOME に go を取り直させない
if goroot=$(cd "$(dirname "$0")/.." && { go env GOROOT || mise exec -- go env GOROOT; } 2>/dev/null); then PATH="$goroot/bin:$PATH"; fi
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
  [ ! -e "$HOME/.local/share/mise" ] || { echo "FAIL 一時の HOME に mise が道具を入れた: $(ls "$HOME/.local/share/mise/installs" 2>&1)" >&2; exit 1; }
  echo "ok   go の道具を一時の HOME に取り直さない"
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
"$andon" issue report "$id" --data '{"remember":["推敲した"]}' <<<$'## Report\n推敲した' >/dev/null
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
# 報告の課題（deliverable: report を Chief of Staff が書いた）で pr が無ければ、断って kanban review で返すと案内し、--pr を案内しない
# （ADR-0066 決めたこと 8・受け入れ条件 6）。担当が書いた report は PR の課題として --pr を案内し、外れた理由を添える
as t5 kanban set "$id" deliverable=report >/dev/null
expect_fail "deliverable: report の課題で PR が無いときは kanban review $id で返す" "$andon" issue ready "$id"
if grep -q -- '--pr' "$tmp/err"; then die "PR の無い報告の課題の ready が --pr を案内した: $(cat "$tmp/err")"; fi
as t7 kanban set "$id" deliverable=report >/dev/null
expect_fail 'deliverable: report を書いたのが Chief of Staff でない' "$andon" issue ready "$id"
grep -q -- '--pr' "$tmp/err" || die "照合が外れた report の ready が --pr を案内しない: $(cat "$tmp/err")"
as t5 kanban set "$id" deliverable=pr >/dev/null
ok 'ready: 報告の課題で pr が無ければ kanban review で返すと案内し（--pr を出さない）、担当が書いた report は PR の課題として --pr を案内する'
cp "$tmp/good" "$lib/report.json"

# show
"$andon" issue show "$id" > "$tmp/show"
jq -e --arg rev "$rev2" --arg c "$c2" '.settings == {review: "on", must_fix: "major", reviewer: "codex-review", review_rounds: "2", merge: "person", deliverable: "pr"}
  and .review == {state: "current", issue: $rev, commit: $c, must_fix_open: 0, waived: 0} and .report.commit == $c' "$tmp/show" >/dev/null
jq .settings "$tmp/show" > "$tmp/settings.json"; emit "$tmp/settings.json" issue-keys.json
ok 'show: 予約キー（既定で埋める）・report.json・レビューの照合'

# ready の正。pr のある報告の課題は PR の課題と同じ確かめを通る（ADR-0066 受け入れ条件 6）
as t5 kanban set "$id" deliverable=report >/dev/null
"$andon" issue ready "$id" >/dev/null
[ "$(kanban show "$id" --json | jq -r .issue.status)" = review ] || die 'pr のある報告の課題を ready が review にしない'
as t5 kanban set "$id" deliverable=pr >/dev/null
ok 'ready: HEAD ＝ report.json ＝ PR の head ＝ レビューの commit で review にする（pr のある報告の課題も同じ）'

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
out=$(as t5 "$andon" issue ask --new --now - <<<"$nq")
own=${out%#*}
[[ "$out" == "$own#1" ]] || die "ask --new: 出力が <id>#<n> でない: $out"
[ "$(count)" = $((n0 + 1)) ] || die 'ask --new: 課題が 1 つ増えていない'
kanban show "$own" --json | jq -e '.issue | .who == "person" and .status == "todo" and .title == "外のメンテナーに約束の日を伝えるか（オーナーの判断待ち）" and (has("profile") | not)' >/dev/null
kanban show "$own" --json | jq -e '.questions | length == 1 and .[0].status == "open" and .[0].kind == "promise"' >/dev/null
jq -e --argjson q "$nq" '. == $q' "kanban/issues/$own/library/questions/1.json" >/dev/null || die 'ask --new: questions/1.json が渡した JSON でない'
grep -qx 'kind: promise' "kanban/issues/$own/brief.md" && grep -qx '推奨案: 伝える' "kanban/issues/$own/brief.md" \
  && grep -q '^出どころ: T-0001（andon issue を足す） / docs/a.md$' "kanban/issues/$own/brief.md" || die "ask --new: 依頼書に種類・推奨案・出どころが無い: $(cat "kanban/issues/$own/brief.md")"
"$andon" waiting list --json | jq -e --arg id "$own" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].question == ($id + "#1") and .[0].answerable == true' >/dev/null \
  || die "ask --new: 判断の行に出ない: $("$andon" waiting list --json)"
ok 'ask --new: 課題を 1 つ足し、who: person・タイトル・依頼書（種類・推奨案）・開いた問い・questions/1.json を置き、andon waiting list の判断の行に答えられる形で出る（受け入れ条件 7）'
for bad in '.options = [.options[0]]|options が2つ未満' '.kind = "other"|kind が' '.kind = "scope"|scope の問いはオーナーの課題に置かない'; do
    expect_fail "${bad#*|}" as t5 "$andon" issue ask --new --now - <<<"$(jq -c "${bad%|*}" <<<"$nq")"
done
expect_fail '問いの JSON の形が違う' as t5 "$andon" issue ask --new --now - <<<'オーナーに聞きたいことがある'
[ "$(count)" = $((n0 + 1)) ] || die 'ask --new: 形の違う問いで課題が増えた'
ok 'ask --new: 選択肢が 1 つ・知らない種類・scope・JSON でない文は、終了コード 1 で課題を足さない（受け入れ条件 8）'
# 途中で落ちたあとの打ち直し: kanban add のあと who: person を書く前に止まった課題（同じタイトルで、who も問いも無い）は、
# 同じ command を打ち直すと足し直さずに続きから置く。でき上がった課題に同じ問いをもう一度渡すと、2 つ目を足さずに断る
hq=$(jq -c '.question = "途中で止まった判断"' <<<"$nq")
half=$(as t5 kanban add '途中で止まった判断（オーナーの判断待ち）')
n1=$(count)
out=$(as t5 "$andon" issue ask --new --now - <<<"$hq")
[[ "$out" == "$half#1" ]] || die "ask --new: 途中で止まった課題 $half に続きから置いていない: $out"
[ "$(count)" = "$n1" ] || die 'ask --new: 打ち直しで課題が増えた'
kanban show "$half" --json | jq -e '.issue.who == "person" and (.questions | length == 1 and .[0].status == "open")' >/dev/null
expect_fail "同じ問いのオーナーの課題 $half がもうある" as t5 "$andon" issue ask --new --now - <<<"$hq"
[ "$(count)" = "$n1" ] && [ "$(kanban show "$half" --json | jq '.questions | length')" = 1 ] || die 'ask --new: 同じ問いで課題か問いが増えた'
ok 'ask --new: who を書く前に止まった課題には打ち直しで続きから置き、でき上がった課題には同じ問いを足さない'
pre=$(as t5 kanban add '前の課題' --profile claude)
out=$(as t5 "$andon" issue ask --new --now --after "$pre" --parent "$id" - <<<"$(jq -c '.question = "前の課題のあとで go を出すか" | .kind = "line"' <<<"$nq")")
own2=${out%#*}
kanban show "$own2" --json | jq -e --arg pre "$pre" --arg p "$id" '(.issue | .status == "queued" and .after == [$pre] and .parent == $p and .who == "person") and .questions[0].status == "open"' >/dev/null \
  || die "ask --new --after: queued・after・parent・開いた問いでない: $(kanban show "$own2" --json)"
"$andon" waiting list --json | jq -e --arg id "$own2" '[.items[] | select(.id == $id)] | length == 0' >/dev/null || die 'ask --new --after: 前の課題が片付く前に andon waiting list に出た'
as t5 kanban done "$pre" >/dev/null
"$andon" waiting list --json | jq -e --arg id "$own2" '[.items[] | select(.id == $id)] | length == 1 and .[0].kind == "decision" and .[0].question == ($id + "#1")' >/dev/null \
  || die 'ask --new --after: 前の課題が片付いても判断の行に出ない'
ok 'ask --new --after --parent: queued で after と parent を持ち、問いを置き、前の課題が片付くまで andon waiting list に出ず、片付くと判断の行に出る（受け入れ条件 9）'
# ask --new は呼ぶか置いておくかを言い切らせる（ADR-0062 決めたこと 10。受け入れ条件 9）
lq=$(jq -c '.question = "置いておける判断"' <<<"$nq")
out=$(as t5 "$andon" issue ask --new --later '急がない。答えを待つ仕事が無い' --until 2099-12-31 - <<<"$lq")
own3=${out%#*}
kanban show "$own3" --json | jq -e '.issue | .who == "person" and .later == "急がない。答えを待つ仕事が無い" and .later_until == "2099-12-31"' >/dev/null \
  || die "ask --new --later --until: later と later_until を書いていない: $(kanban show "$own3" --json)"
"$andon" waiting list --json | jq -e --arg id "$own3" '[.items[] | select(.id == $id)] | length == 1 and .[0].call == false and .[0].later == "急がない。答えを待つ仕事が無い"' >/dev/null \
  || die "ask --new --later: andon waiting list で置いてある行（call が false）にならない: $("$andon" waiting list --json)"
"$andon" waiting list --json | jq -e --arg id "$own" '[.items[] | select(.id == $id)] | length == 1 and .[0].call == true' >/dev/null \
  || die 'ask --new --now: 呼んでいる行（call が true）にならない'
kanban show "$own" --json | jq -e '.issue | (has("later") or has("later_until")) | not' >/dev/null || die 'ask --new --now: 課題に印がある'
ok 'ask --new: Chief of Staff の --later --until は later と later_until を書いて置いてある行になり、--now は印を書かず呼ぶ行になる（ADR-0062 受け入れ条件 9）'
refuse() {
    local want=$1; shift
    local rc=0; "$@" >"$tmp/out" 2>"$tmp/err" || rc=$?
    [ "$rc" != 0 ] && grep -q -- "$want" "$tmp/err" || { cat "$tmp/err" >&2; die "want rc != 0 with '$want', got rc $rc: $*"; }
}
n2=$(count)
rq=$(jq -c '.question = "断られる判断"' <<<"$nq")
refuse '呼ぶか置いておくかを言い切る' as t5 "$andon" issue ask --new - <<<"$rq"
refuse '呼ぶか置いておくかを言い切る' as t5 "$andon" issue ask --new --now --later '急がない' - <<<"$rq"
refuse '--until は --later と一緒のときだけ' as t5 "$andon" issue ask --new --until 2099-12-31 - <<<"$rq"
refuse '--until の日付は YYYY-MM-DD' as t5 "$andon" issue ask --new --later '急がない' --until 2099-13-01 - <<<"$rq"
refuse '--later で印を書くのは Chief of Staff の端末だけ' as t7 "$andon" issue ask --new --later '急がない' - <<<"$rq"
refuse '--later の値が無い（--now が続いた）' as t5 "$andon" issue ask --new --later --now - <<<"$rq"
[ "$(count)" = "$n2" ] || die "ask --new: 断った打ち方で課題が増えた（$n2 → $(count)）"
kanban list --all --json | jq -e '[.[] | select(.title | startswith("断られる判断"))] | length == 0' >/dev/null || die 'ask --new: 断った問いの課題がある'
ok 'ask --new: 旗が無い・両方ある・--until だけ・日付の形が違う・Chief of Staff でない端末の --later・値の無い --later は、0 でない終了コードで課題を足さない（ADR-0062 受け入れ条件 9）'
# 打ち直しで旗が前の回と違えば、前の回の印を残さない（問いを置く前に落ちた --later --until の回を、--now か --later だけで打ち直す）
for again in '--now|' '--later|今は急がない'; do
    flag=${again%|*} val=${again#*|}
    tq=$(jq -c --arg f "$flag" '.question = ("旗を変えて打ち直す判断 " + $f)' <<<"$nq")
    th=$(as t5 kanban add "$(jq -r .question <<<"$tq")（オーナーの判断待ち）")
    as t5 kanban set "$th" later='前の回の理由' later_until=2099-12-31 >/dev/null
    if [ -n "$val" ]; then out=$(as t5 "$andon" issue ask --new "$flag" "$val" - <<<"$tq"); else out=$(as t5 "$andon" issue ask --new "$flag" - <<<"$tq"); fi
    [[ "$out" == "$th#1" ]] || die "ask --new $flag: 途中で止まった課題 $th に続きから置いていない: $out"
    want=$(jq -nc --arg v "$val" 'if $v == "" then {} else {later: $v} end')
    kanban show "$th" --json | jq -e --argjson w "$want" '.issue | {later, later_until} | with_entries(select(.value != null)) == $w' >/dev/null \
      || die "ask --new $flag: 打ち直しで前の回の印が残る: $(kanban show "$th" --json | jq -c '.issue')"
done
"$andon" waiting list --json | jq -e '[.items[] | select(.title | startswith("旗を変えて打ち直す判断 --now")) | .call] == [true]' >/dev/null \
  || die "ask --new --now の打ち直し: 呼ぶ行にならない: $("$andon" waiting list --json)"
ok 'ask --new: 前の回と旗の違う打ち直しは、前の回の later・later_until を消し、--now なら呼ぶ行になる'
# kanban ask のあと控えを書く前に落ちた --later --until の回（問いはあり、questions/1.json が無い）を --now で打ち直しても、
# 控えを書き直す道で前の回の印を消し、呼ぶ行になる
cq=$(jq -c '.question = "控えを書く前に落ちた判断"' <<<"$nq")
out=$(as t5 "$andon" issue ask --new --later '前の回の理由' --until 2099-12-31 - <<<"$cq")
ch=${out%#*}
rm "kanban/issues/$ch/library/questions/1.json"
out=$(as t5 "$andon" issue ask --new --now - <<<"$cq")
[[ "$out" == "$ch#1" ]] || die "ask --new --now: 控えの無い課題 $ch に続きから置いていない: $out"
[ -f "kanban/issues/$ch/library/questions/1.json" ] || die 'ask --new --now: 控えを書き直していない'
kanban show "$ch" --json | jq -e '.issue | (has("later") or has("later_until")) | not' >/dev/null \
  || die "ask --new --now: 控えを書き直す打ち直しで前の回の印が残る: $(kanban show "$ch" --json | jq -c '.issue')"
"$andon" waiting list --json | jq -e --arg id "$ch" '[.items[] | select(.id == $id) | .call] == [true]' >/dev/null \
  || die 'ask --new --now: 控えを書き直す打ち直しのあと、呼ぶ行にならない'
ok 'ask --new: 控えを書く前に落ちた回を旗を変えて打ち直すと、控えを書き直し、前の回の印を消して呼ぶ行になる'
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
# 何をしたかの1行と、出た場所の手前に次の1手の行（ADR-0053 決めたこと 14・受け入れ条件 A12）
# 次の1手は読むだけの葉。止まった担当を起動し直すのは dispatcher で、Chief of Staff には resume を勧めない（docs/dispatcher.md §2.9）
[ "$(tail -n 2 "$tmp/out" | sed -n 1p)" = "andon issue: 次: 報告と問いは dispatcher が届ける（止まれば dispatcher が1回起動し直す）。流れは andon issue trace $s1 --project demo で見る" ] \
  || die "start が次の1手（andon issue trace）を出さない: $(cat "$tmp/out")"
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

# 成果が報告の課題（deliverable: report。ADR-0066 受け入れ条件 7）: 求める予約キーは PR の課題と同じで、--dry-run が出す
# 依頼書の写しは Deliverable: report のまま（値 report を知らない読み方では Deliverable: pr に書き直す）
s14=$(kanban add 'Report only' --repo github.com/example/app --profile claude --brief - <<<$'# Report only\n\n## Settings\n- Review: off\n- Deliverable: report\n- Estimate: 小')
HC_TERMINAL_ID=t5 kanban set "$s14" review=off deliverable=report estimate=small >/dev/null
expect_fail '無い・知らない値: must_fix・merge。' "$andon" issue start "$s14" --dry-run
HC_TERMINAL_ID=t5 kanban set "$s14" must_fix=major merge=person >/dev/null
"$andon" issue start "$s14" --dry-run --json > "$tmp/dry-report" || die "deliverable=report の課題の dry-run が落ちた: $(cat "$tmp/err" 2>/dev/null)"
jq -e '(.brief | contains("\n- Deliverable: report ★\n")) and (.brief | contains("Deliverable: pr") | not)' "$tmp/dry-report" >/dev/null \
  || die "依頼書の写しが Deliverable: report でない: $(jq -r .brief "$tmp/dry-report")"
ok 'start: deliverable=report の課題は PR の課題と同じ予約キーを求め、--dry-run の依頼書の写しが Deliverable: report になる'

# andon issue create（ADR-0053 受け入れ条件 A9・A10）: 課題を足して予約キーを書き（書き手は打った端末）、依頼書の Settings 欄を
# 予約キーから書き、足した id と次に打つ andon issue start <id> を出す。予約キーでないキー・知らない値は、何も足さずに終了コード 1
n0=$(kanban list --all --json | jq length); l0=$(wc -l < kanban/log.jsonl)
expect_fail 'review=maybe' "$andon" issue create 'Bad value' --repo github.com/example/app --brief - review=maybe estimate=small <<<'# x'
expect_fail 'who は andon issue create が受ける予約キーでない' "$andon" issue create 'Bad key' --repo github.com/example/app review=off who=person
expect_fail 'estimate=huge' "$andon" issue create 'Bad estimate' review=off estimate=huge
expect_fail 'タイトルが無い（最初の語 review=maybe' "$andon" issue create review=maybe --repo github.com/example/app
expect_fail '依頼書（--brief -）が空' "$andon" issue create 'Empty brief' --repo github.com/example/app --profile claude --brief - review=off estimate=small </dev/null
expect_fail '渡していない予約キーの値がある（- Acceptance: 9,10・- Deliverable: candidates ★調べる）' "$andon" issue create 'Only in brief' \
  --repo github.com/example/app --profile claude --brief - review=off estimate=small <<<$'# x\n\n## Settings\n- Review: off\n- Acceptance: 9,10\n- Deliverable: candidates ★調べる\n- Estimate: 小'
"$andon" issue create 'Dry' --repo github.com/example/app --brief - review=on estimate=small --dry-run --json <<<'# Dry' > "$tmp/dry-create" || die 'create --dry-run が落ちた'
[ "$(kanban list --all --json | jq length)" = "$n0" ] && [ "$(wc -l < kanban/log.jsonl)" = "$l0" ] || die "断ったか --dry-run なのに課題を足した: $(kanban list --all --json | jq -c '.[-1]')"
jq -e '.dry_run and .keys == ["review=on", "estimate=small"] and (.brief | endswith("\n## Settings\n- Review: on ★\n- Estimate: 小\n"))' "$tmp/dry-create" >/dev/null \
  || die "create --dry-run の中身: $(cat "$tmp/dry-create")"
ok 'create: 知らない値（review=maybe・estimate=huge）・予約キーでないキー（who=person）・タイトルの抜け・空の依頼書・欄にだけある値は課題を足さずに終了コード 1。--dry-run は何も書かずに書く依頼書を出す（A9）'
"$andon" issue create 'Create in one' --repo github.com/example/app --profile claude --brief - review=off estimate=small must_fix=major merge=person \
  <<<$'# Create in one\n完了条件: 押せる\n' > "$tmp/out" 2>"$tmp/err" || die "create が落ちた: $(cat "$tmp/err")"
cr1=$(sed -n 's/^andon issue create: \([A-Z]*-[0-9]*\)（Create in one）を足した.*/\1/p' "$tmp/out")
[ -n "$cr1" ] && [ "$(tail -n 1 "$tmp/out")" = "andon issue create: 次: andon issue start $cr1 --project demo" ] || die "足した id と次の1手が出ない: $(cat "$tmp/out")"
[ "$(field "$cr1" review)$(field "$cr1" estimate)$(field "$cr1" must_fix)$(field "$cr1" merge)" = offsmallmajorperson ] \
  && jq -e --arg id "$cr1" 'select(.id == $id and .change.review == "off") | .by == "terminal:t5"' kanban/log.jsonl >/dev/null \
  || die "予約キーを打った端末で書いていない: $(kanban show "$cr1" --json | jq -c .issue)"
[ "$(cat "kanban/issues/$cr1/brief.md")" = $'# Create in one\n完了条件: 押せる\n\n## Settings\n- Review: off\n- Must fix: major\n- Merge: person\n- Estimate: 小' ] \
  || die "Settings 欄が予約キーの写しでない: $(cat "kanban/issues/$cr1/brief.md")"
"$andon" issue start "$cr1" --dry-run --json | jq -e '[.checks[] | select(.name == "settings" and .result == "ok")] | length == 1' >/dev/null \
  || die "create のあとの start --dry-run が予約キーの理由で断る: $("$andon" issue start "$cr1" --dry-run 2>&1)"
ok 'create: 課題を1件足し、予約キーを打った端末で書き、依頼書に Settings 欄を足し、id と andon issue start <id> を出す。続く start --dry-run は予約キーで断らない（A9）'
"$andon" issue create 'Settings copy' --repo github.com/example/app --profile claude --brief - --json \
  review=on reviewer=codex-review must_fix=major review_rounds=2 merge=auto estimate=medium --reason merge=委譲の範囲 \
  <<<$'# Settings copy\n\n## Settings\n- Review: off\n- Reviewer: codex-review\n- Must fix: major\n- Review rounds: 2\n- Merge: person\n- Estimate: 中\n\n頼まれていない稀なケースのために設計しない。' > "$tmp/create.json" \
  || die "create --json が落ちた: $(cat "$tmp/create.json")"
cr2=$(jq -r .id "$tmp/create.json")
jq -e --arg id "$cr2" '.next == "andon issue start \($id) --project demo" and .missing == []' "$tmp/create.json" >/dev/null || die "create --json: $(cat "$tmp/create.json")"
want=$'# Settings copy\n\n## Settings\n- Review: on ★\n- Reviewer: codex-review\n- Must fix: major\n- Review rounds: 2\n- Merge: auto ★委譲の範囲\n- Estimate: 中\n\n頼まれていない稀なケースのために設計しない。'
[ "$(cat "kanban/issues/$cr2/brief.md")" = "$want" ] || die "雛形の書き方の写しでない: $(cat "kanban/issues/$cr2/brief.md")"
HC_TERMINAL_ID=t5 kanban set "$cr2" estimate=large merge=person >/dev/null
"$andon" issue start "$cr2" >/dev/null || die "create した課題を起動できない: $(cat "$tmp/err" 2>/dev/null)"
[ "$(cat "kanban/issues/$cr2/brief.md")" = "${want/$'Merge: auto ★委譲の範囲\n- Estimate: 中'/$'Merge: person\n- Estimate: 大'}" ] \
  || die "kanban set のあとの start が欄を新しい値にしない: $(cat "kanban/issues/$cr2/brief.md")"
ok 'create: 依頼書の Settings 欄を雛形の書き方で写し（estimate=medium は Estimate: 中、既定から変えた行に ★ と理由）、kanban set のあとの start が新しい値に書き直す（A10）'
"$andon" issue create 'Needs keys' --repo github.com/example/app --profile claude --brief - review=on <<<'# x' > "$tmp/out" || die 'create が落ちた'
cr3=$(sed -n 's/^andon issue create: \([A-Z]*-[0-9]*\)（Needs keys）を足した.*/\1/p' "$tmp/out")
[ "$(tail -n 1 "$tmp/out")" = "andon issue create: 次: kanban set $cr3 'must_fix=<critical|major|minor|trivial>' 'reviewer=<codex-review|claude-review>' 'review_rounds=<1 以上の数>' 'merge=<auto|person>' 'estimate=<small|medium|large>' を打ってから andon issue start $cr3 --project demo" ] \
  || die "足りない予約キーを書く行が出ない: $(cat "$tmp/out")"
"$andon" issue create 'No profile' --brief - review=off estimate=small <<<'# x' > "$tmp/out" || die 'profile の無い create が落ちた'
cr4=$(sed -n 's/^andon issue create: \([A-Z]*-[0-9]*\)（No profile）を足した.*/\1/p' "$tmp/out")
[ "$(tail -n 1 "$tmp/out")" = "andon issue create: 次: kanban set $cr4 'profile=<名前>' を打ってから andon issue start $cr4 --project demo" ] || die "profile を書く行が出ない: $(cat "$tmp/out")"
ok 'create: andon issue start が求める予約キーか profile が足りなければ、次の1手にそれを書く kanban set の行を出す'
ce0=$'# CE brief\n完了条件: 文書\n'
ce2=$("$andon" issue create 'CE brief' --repo github.com/example/app --profile chief-engineer --brief - --json review=on estimate=small <<<"$ce0" | jq -r .id)
printf '%s\n' "$ce0" > "$tmp/ce-brief"
"$andon" issue start "$ce2" --dry-run --json > "$tmp/ce-dry" || die "Chief Engineer の課題の start --dry-run が落ちた: $(cat "$tmp/ce-dry")"
cmp -s "$tmp/ce-brief" "kanban/issues/$ce2/brief.md" && [ "$(jq -r .brief "$tmp/ce-dry")" = "$(cat "$tmp/ce-brief")" ] \
  || die "Chief Engineer の課題の依頼書を変えた: $(cat "kanban/issues/$ce2/brief.md")"
ok 'create・start: Chief Engineer の課題の依頼書は変えない（A10）'

# ルーチンが足した課題（log.jsonl に by: routine:<名前>・fired の行）は、Chief of Staff の端末で打てば、無い予約キーを既定で書いてから起動する。
# --dry-run は書かない。ほかの端末で打てば今までどおり断る
mkdir -p kanban/routines
printf 'title: "Upkeep"\nprofile: claude\nbrief: |\n  # Upkeep\nschedule: "0 0 1 1 *"\n' > kanban/routines/upkeep.yaml
s13=$(kanban routine run upkeep | sed -n 's/^upkeep: added //p')
[ -n "$s13" ] && [ "$(field "$s13" routine)" = upkeep ] && [ -z "$(field "$s13" review)$(field "$s13" estimate)" ] || die "ルーチンが課題を足さない: $s13"
before=$(wc -l < kanban/log.jsonl)
"$andon" issue start "$s13" --dry-run --json | jq -e '.fill == ["review=off", "estimate=medium"]' >/dev/null \
  || die "dry-run が埋めるキー: $("$andon" issue start "$s13" --dry-run 2>&1)"
[ "$(wc -l < kanban/log.jsonl)" = "$before" ] || die 'dry-run が kanban を書いた'
HC_TERMINAL_ID=t7 expect_fail '無い・知らない値: review・estimate。' "$andon" issue start "$s13"
"$andon" issue start "$s13" >/dev/null || die "ルーチンが足した課題を起動できない: $(cat "$tmp/err" 2>/dev/null)"
[ "$(field "$s13" status)" = doing ] && [ "$(field "$s13" review)" = off ] && [ "$(field "$s13" estimate)" = medium ] \
  && jq -e --arg id "$s13" 'select(.id == $id and .change.review == "off" and .change.estimate == "medium") | .by == "terminal:t5"' kanban/log.jsonl >/dev/null \
  || die "既定を Chief of Staff の端末で書いていない: $(kanban show "$s13" --json | jq -c .issue)"
ok 'start: ルーチンが足した課題は、Chief of Staff の端末で打てば無い予約キーを既定で書いてから起動する（--dry-run は書かない。ほかの端末では断る）'

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
checked() { # checked <タイトル>: finish が断り、端末・worktree・branch・kanban がそのまま
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
wbissue() {  # wbissue <タイトル> <links…>: links と report.json の pr を持つ、repo の無い課題
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
  and (.first_message | test("^\\[resume\\] 課題 `[^`]*` の続き。セルが起き直した。worktree は同じ場所に戻した。`kanban list` と git で今の状態を確かめ、続きから。"))' "$tmp/dry" >/dev/null \
  || die "resume --dry-run の計画: $(cat "$tmp/dry")"
ok 'resume --dry-run: doing で素のシェルの課題は、worktree のパス・branch・起動する template・cwd・session・最初のメッセージの計画を出す（hc は読むだけ）'

: > "$tmp/hc/log"
HC_TERMINAL_ID=t5 kanban set "$r1" estimate=large >/dev/null   # 起動のあとに変えた値は、次に起動する resume が依頼書の欄に写す（ADR-0053 決めたこと 19）
"$andon" issue resume "$r1" > "$tmp/out" || die "resume が落ちた: $(cat "$tmp/out")"
grep -qx -- '- Estimate: 大' "kanban/issues/$r1/brief.md" && ! grep -q 'Estimate: 小' "kanban/issues/$r1/brief.md" \
  || die "resume が依頼書の Settings 欄を予約キーから書き直していない: $(cat "kanban/issues/$r1/brief.md")"
ok 'resume: 起き直す前に、依頼書の Settings 欄を予約キーから書き直す（ADR-0053 決めたこと 19）'
a2=$(field "$r1" agent)
[ -d "$rwt" ] && [ "$(git -C "$rwt" rev-parse --abbrev-ref HEAD)" = "$rbr" ] || die "worktree が戻っていない: $(cat "$tmp/out")"
[ "$a2" != "$a1" ] && [ "$(field "$r1" status)" = doing ] && [ "$(field "$r1" session)" = "$sess" ] && [ "$(field "$r1" worktree)" = "$rwt" ] \
  || die "kanban: $(kanban show "$r1" --json | jq -c .issue)"
order=$(grep -n "^keep $a1 --off\|^kill $a1\|^rm $a1\|^run " "$tmp/hc/log" | cut -d: -f2 | cut -d' ' -f1 | tr '\n' ' ')
[ "$order" = "keep kill rm run " ] || die "素のシェルを片付けてから起動していない: $(cat "$tmp/hc/log")"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-$(tr A-Z a-z <<<"$r1") --label project=demo --label issue=$r1 --label title=Resume me --label herdr.space=andon-demo-$r1 --label herdr.tab=andon-demo-$r1 --label session=$sess --cwd $rwt claude-resume \[resume\] 課題 \`demo/$r1\` の続き。" "$tmp/hc/log" \
  || die "hc run の形: $(grep ^run "$tmp/hc/log")"
jq -e --arg id "$r1" --arg a "$a2" 'select(.id == $id and .change.agent == $a) | .by == "terminal:t5"' kanban/log.jsonl >/dev/null || die 'kanban の agent を新しい端末にしていない'
# 何をしたかの1行のあとに次の1手の行（ADR-0053 決めたこと 14・受け入れ条件 A12）
grep -q "^andon issue: $r1 を $a2（.*）で続きから起動した。" "$tmp/out" \
  && [ "$(tail -n 1 "$tmp/out")" = "andon issue: 次: 報告と問いは dispatcher が届ける（止まれば dispatcher が1回起動し直す）。流れは andon issue trace $r1 --project demo で見る" ] \
  || die "resume が何をしたかと次の1手（andon issue trace）を出さない: $(cat "$tmp/out")"
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
[ "$(field "$s4" session)" = "$new" ] && grep -q -- "--label session=$new --cwd $cwt codex-resume \[resume\] 課題 \`demo/$s4\` の続き。" "$tmp/hc/log" \
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
HC_TERMINAL_ID=t5 kanban set "$rv" estimate=small >/dev/null # 予約キーがあれば欄を書くので、書かないことが見える
cp "kanban/issues/$rv/brief.md" "$tmp/rv-brief"
"$andon" issue resume "$rv" > "$tmp/out" || die "レビュアーの resume が落ちた: $(cat "$tmp/out")"
cmp -s "$tmp/rv-brief" "kanban/issues/$rv/brief.md" || die "resume がレビュアーの課題の依頼書を変えた: $(cat "kanban/issues/$rv/brief.md")"
[ "$(git -C "$rvwt" rev-parse HEAD)" = "$rc" ] && ! git -C "$rvwt" symbolic-ref -q HEAD >/dev/null || die "detached worktree が commit $rc に戻っていない: $(cat "$tmp/out")"
grep -qF -- "--label session=$rvs --cwd $rvcwd codex-review-resume [resume] 課題 \`demo/$rv\` の続き。" "$tmp/hc/log" || die "レビュアーの hc run: $(grep ^run "$tmp/hc/log")"
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

# andon design start（ADR-0028 受け入れ条件 2）: Chief Engineer の課題に起動の記録が1件
"$andon" design start --repo github.com/example/app 'Design topic' > "$tmp/out" || die "andon design start が落ちた: $(cat "$tmp/out")"
ce=$(kanban list --json | jq -r '.[] | select(.title == "Design topic") | .id')
[ -n "$ce" ] && [ "$(nlaunch "$ce")" = 1 ] && jq -e --arg id "$ce" --arg s "$(field "$ce" session)" --arg wt "$(field "$ce" worktree)" \
  '.role == "chief-engineer" and .launch == "start" and .issue == $id and .profile == "chief-engineer" and .session == $s and .repo.path == $wt and .repo.commit != null' "$(launches "$ce")" >/dev/null \
  || die "andon design start の起動の記録: $(cat "$(launches "$ce")" 2>/dev/null) $(cat "$tmp/out")"
emit "$(launches "$ce")" launch-chief-engineer.json
ok 'andon design start: Chief Engineer の課題に起動の記録が1件（役 chief-engineer・session・worktree の commit。ADR-0028 受け入れ条件 2）'

# 止める口（ADR-0056 決めたこと 7・8。受け入れ条件 A5）: andon issue hold・unhold。偽の gh を止めの場面のものに替える:
# pr view 7 --repo … --json state,mergeCommit,headRefOid,statusCheckRollup,headRefName,isCrossRepository,baseRefName は $tmp/hold-view を返し、
# api -X POST …/statuses/<sha> は引数を $tmp/hold-gh に1行で足す（$tmp/hold-fail があれば落ちる）
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
case "\$1 \$2" in
  "pr view") [ "\$7" = state,mergeCommit,headRefOid,statusCheckRollup,headRefName,isCrossRepository,baseRefName ] || { echo "unexpected gh \$*" >&2; exit 99; }
    cat "$tmp/hold-view" ;;
  "api -X") echo "\$*" >> "$tmp/hold-gh"; [ ! -e "$tmp/hold-fail" ] || { echo "HTTP 502" >&2; exit 1; } ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
hh=$(git -C "$tmp/wt" rev-parse HEAD)
view() { jq -n --arg h "$hh" --arg s "$1" --arg st "${2:-OPEN}" '{state: $st, headRefOid: $h, mergeCommit: null,
  statusCheckRollup: (if $s == "" then [] else [{__typename: "StatusContext", context: "andon/gate", state: $s}] end)}' > "$tmp/hold-view"; }
held=$(as t5 kanban add '止める PR' --repo github.com/example/app --profile claude)
why=$(as t5 kanban add 'オーナーの答えを待つ問い' --repo github.com/example/app)
as t5 kanban set "$held" status=review >/dev/null
mkdir -p "kanban/issues/$held/library"
jq -n --arg c "$hh" --arg pr "$pr" '{commit: $c, pr: $pr}' > "kanban/issues/$held/library/report.json"
: > "$tmp/hold-gh"
view SUCCESS
as t5 "$andon" issue hold "$held" "$why" > "$tmp/out" || die "hold が落ちた: $(cat "$tmp/out")"
[ "$(field "$held" hold)" = "$why" ] && grep -q "\"by\":\"terminal:t5\",\"id\":\"$held\",\"change\":{\"hold\":\"$why\"}" kanban/log.jsonl \
  && grep -q "^api -X POST repos/example/app/statuses/$hh -f context=andon/gate -f state=pending -f description=andon gate: メンテナーの確認を待っています$" "$tmp/hold-gh" \
  || die "hold が hold を書き、渡している commit に黄を書いていない: $(cat "$tmp/hold-gh") $(field "$held" hold)"
ok 'hold: 予約キー hold に理由の課題を書き、印が緑（渡している）の commit に state=pending を書いて 0 で終わる'
: > "$tmp/hold-gh"
as t5 "$andon" issue unhold "$held" > "$tmp/out" || die "unhold が落ちた: $(cat "$tmp/out")"
[ "$(field "$held" hold)" = "" ] && [ ! -s "$tmp/hold-gh" ] || die "unhold が hold を消さない・印を書いた: $(cat "$tmp/hold-gh")"
ok 'unhold: hold を消し、印は書かない（gh を呼ばない）'
touch "$tmp/hold-fail"
expect_fail '黄にできない' as t5 "$andon" issue hold "$held" "$why"
[ "$(field "$held" hold)" = "$why" ] || die '印を書けなかった hold で hold が残っていない'
rm "$tmp/hold-fail"; as t5 "$andon" issue unhold "$held" >/dev/null
ok 'hold: 印の書き込みに失敗すると 0 以外で終わり、hold は書いたまま（dispatcher が揃える）'
: > "$tmp/hold-gh"
view ''
as t5 "$andon" issue hold "$held" "$why" > "$tmp/out" || die "渡していない commit の hold が落ちた: $(cat "$tmp/out")"
[ ! -s "$tmp/hold-gh" ] && grep -q 'まだ渡していない' "$tmp/out" || die "渡していない commit に印を書いた: $(cat "$tmp/hold-gh")"
as t5 "$andon" issue unhold "$held" >/dev/null
ok 'hold: 渡していない（印が無く、gate の判定も無い）commit には印を書かない'
expect_fail 'T-9999 が無い' as t5 "$andon" issue hold "$held" T-9999
[ "$(field "$held" hold)" = "" ] || die '無い課題を理由にした hold が hold を書いた'
ok 'hold: 無い課題を理由に渡すと何も書かずに断る'
cp "kanban/issues/$held/library/report.json" "$tmp/hold-report"; echo '{' > "kanban/issues/$held/library/report.json"
view SUCCESS; : > "$tmp/hold-gh"
expect_fail 'report.json' as t5 "$andon" issue hold "$held" "$why"
[ "$(field "$held" hold)" = "$why" ] && [ ! -s "$tmp/hold-gh" ] || die '壊れた report.json で hold を書かないか、印を書いた'
cp "$tmp/hold-report" "kanban/issues/$held/library/report.json"; as t5 "$andon" issue unhold "$held" >/dev/null
ok 'hold: report.json を読めないと、PR が無いのと分けて 0 以外で終わる（印が緑のまま成功と言わない）'
view SUCCESS MERGED; : > "$tmp/hold-gh"
expect_fail 'MERGED' as t5 "$andon" issue hold "$held" "$why"
[ ! -s "$tmp/hold-gh" ] || die 'マージ済みの PR に印を書いた'
as t5 "$andon" issue unhold "$held" >/dev/null
ok 'hold: PR がもうマージされていれば 0 以外で終わる（止められていないのに成功と言わない）'
view SUCCESS; : > "$tmp/hold-gh"
flock "$lk" sleep 2 & lp=$!
sleep 0.3; t0=$(date +%s%N)
as t5 "$andon" issue hold "$held" "$why" >/dev/null || die 'lock のあと hold が落ちた'
wait "$lp"; [ $(( ($(date +%s%N) - t0) / 1000000 )) -ge 1200 ] || die 'hold がマージの鍵（issue lock）を待たなかった'
flock "kanban/issues/$held/library/.gate.lock" sleep 2 & lp=$!
sleep 0.3; t0=$(date +%s%N)
as t5 "$andon" issue unhold "$held" >/dev/null || die 'gate の lock のあと unhold が落ちた'
wait "$lp"; [ $(( ($(date +%s%N) - t0) / 1000000 )) -ge 1200 ] || die 'unhold が gate の鍵を待たなかった'
ok 'hold・unhold: 別のプロセスがマージの鍵か gate の鍵を持っている間は、放されるまで待つ'

# andon issue board・history（ADR-0059 受け入れ条件 5）: 切り離した kanban に本物の kanban でいろいろなキーを持つ課題を作り、
# 全部の課題より前の --since の board の issues が kanban list --json --all と同じであることを見る（2 つを jq -S で比べる）
bk="$tmp/board"; mkdir -p "$bk"
bkan() { (cd "$bk" && KANBAN_ROOT="$bk" HC_TERMINAL_ID=t5 kanban "$@"); }
bkan init --prefix B --repo github.com/example/app >/dev/null
b1=$(bkan add '親の課題' --repo github.com/example/app --profile claude)
b2=$(bkan add '子の課題' --repo github.com/example/app --profile codex --parent "$b1")
b3=$(bkan add '後ろで待つ課題' --repo github.com/example/app --after "$b2" --status queued)
b4=$(bkan add 'レビュアーの子課題' --repo github.com/example/app --profile codex-review --parent "$b2" --round 1)
bkan set "$b1" review=on must_fix=major review_rounds=2 merge=auto estimate=large links=https://example.com/a,https://example.com/b >/dev/null
bkan set "$b2" status=doing agent=t9 branch=andon/b-2 not_before=2026-12-01 >/dev/null
bkan set "$b4" status=done >/dev/null
bkan set "$b1" status=done --force >/dev/null
bkan list --json --all > "$tmp/board-list.json"
[ "$(jq length "$tmp/board-list.json")" = 4 ] || die "board の見本の課題が 4 件でない: $(cat "$tmp/board-list.json")"
(cd "$bk" && KANBAN_ROOT="$bk" "$andon" issue board --json --since 2000-01-01T00:00:00Z) > "$tmp/board-out.json" || die 'andon issue board が落ちた'
jq -S '.issues | sort_by(.id)' "$tmp/board-out.json" > "$tmp/board-a.json"
jq -S 'sort_by(.id)' "$tmp/board-list.json" > "$tmp/board-b.json"
cmp -s "$tmp/board-a.json" "$tmp/board-b.json" || { diff "$tmp/board-a.json" "$tmp/board-b.json" >&2; die 'andon issue board の issues が kanban list --json --all と違う'; }
[ "$(jq -c '[.version, .older, .done, .project]' "$tmp/board-out.json")" = '[1,0,"done","board"]' ] || die "board の頭が違う: $(cat "$tmp/board-out.json")"
ok 'issue board: 全部の課題より前の --since で、issues が kanban list --json --all と同じ（ADR-0059 受け入れ条件 5）'
emit "$tmp/board-out.json" issue-board.json
(cd "$bk" && KANBAN_ROOT="$bk" "$andon" issue history --json --limit 1) > "$tmp/history-out.json" || die 'andon issue history が落ちた'
[ "$(jq -c '[.issues[].status, .more]' "$tmp/history-out.json")" = '["done",1]' ] || die "history の出力が違う: $(cat "$tmp/history-out.json")"
ok 'issue history: 片付いた課題を limit 件、more に残りの数'
emit "$tmp/history-out.json" issue-history.json

# 積んだ課題のレビューの引き継ぎ（ADR-0067 決めたこと 9・受け入れ条件 10）: 前の課題の PR の上に積んでいた間のレビュー（起点は分かれ目。
# dispatcher がレビュアーの課題に控えた review_base）は、前の課題が main に入ったあとの衝突の無い載せ替えで、新しい head に引き継がれ、
# review-request が頼み直しを求めない。squash で入ったときと、マージ commit で入ったときの両方で見る。担当の端末が書いた review_base は信じない
# （載せ替えの口 andon issue restack はまだ無いので、git rebase --onto で載せ替え、担当の端末でキーを消す）
# 偽の gh は始めの形に戻す（PR の head は $tmp/prhead。hold の場面が作り直している）
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
case "\$1 \$2 \$4 \$5" in
  "pr view --json headRefOid") cat "$tmp/prhead" ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
restacked() { # restacked <squash|merge> <review_base を書く端末>: 積んで、レビューして、載せ替えた課題の id を返す
    local how=$1 by=$2 g="$tmp/stk-$1-$2" fork s prv sid rv
    git init -q -b main "$g"
    printf 'func A() {\n\tx\n\tx\n\tx\n\tx\n\ty\n}\n' > "$g/a.txt"; echo p > "$g/p.txt"
    git -C "$g" add -A; git -C "$g" commit -qm base
    git -C "$g" switch -qc prev; echo P >> "$g/p.txt"; git -C "$g" commit -qam 'prev change'; fork=$(git -C "$g" rev-parse HEAD)
    git -C "$g" switch -qc stk; sed -i 's/^\ty$/\tY/' "$g/a.txt"; git -C "$g" commit -qam 'stacked change'; s=$(git -C "$g" rev-parse HEAD)
    prv=$(as t5 kanban add '前の課題' --profile claude)
    sid=$(as t5 kanban add '積んだ課題' --profile claude --after "$prv")
    as t5 kanban set "$sid" status=doing review=on must_fix=major reviewer=codex-review review_rounds=2 "worktree=$g" repo=github.com/example/app \
        stacked_on="$prv" stacked_base="$fork" >/dev/null
    echo "$s" > "$tmp/prhead"
    as t7 "$andon" issue report "$sid" --pr "$pr" <<<'## Report' >/dev/null
    rv=$(as dispatcher kanban add "$sid のレビュー（1 回目）" --parent "$sid" --profile codex-review --round 1)
    as "$by" kanban set "$rv" review_base="$fork" >/dev/null
    mkdir -p "kanban/issues/$rv/library"
    jq -n --arg c "$s" '{commit: $c, findings: []}' > "kanban/issues/$rv/library/findings.json"
    as dispatcher kanban start "$rv" >/dev/null; as dispatcher kanban review "$rv" >/dev/null
    git -C "$g" switch -q main
    if [ "$how" = squash ]; then git -C "$g" merge -q --squash prev >/dev/null; git -C "$g" commit -qm 'prev (squash)'; else git -C "$g" merge -q --no-ff -m 'prev (merge)' prev >/dev/null; fi
    git -C "$g" update-ref refs/remotes/origin/main main; git -C "$g" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main
    git -C "$g" switch -q stk; git -C "$g" rebase -q --onto origin/main "$fork"
    [ "$(git -C "$g" rev-parse HEAD)" != "$s" ] || die "載せ替えで head が変わっていない（$how）"
    as t7 kanban set "$sid" stacked_on= stacked_base= >/dev/null
    git -C "$g" rev-parse HEAD > "$tmp/prhead"
    as t7 "$andon" issue report "$sid" <<<'## Report' >/dev/null
    echo "$sid"
}
for how in squash merge; do
    sid=$(restacked "$how" dispatcher)
    expect_fail 'そのレビューを引き継ぐので頼まない' as t7 "$andon" issue review-request "$sid" <<<'## Report'
done
ok 'review-request: 積んでいた間のレビュー（review_base から数えた差分）は、前の課題が squash・マージ commit のどちらで入っても、衝突の無い載せ替えのあとの head に引き継がれ、頼み直しを求めない'
sid=$(restacked squash t7)
as t7 "$andon" issue review-request "$sid" <<<'## Report' >/dev/null || { cat "$tmp/err" >&2; die '担当の端末が書いた review_base のレビューを引き継いだ'; }
jq -e '.review_request.round == 2' "kanban/issues/$sid/library/report.json" >/dev/null || die "担当の端末が書いた review_base で 2 回目を頼まない: $(jq -c .review_request "kanban/issues/$sid/library/report.json")"
ok 'review-request: 担当の端末が書いた review_base は信じず、既定の branch との merge-base から数えて、載せ替えのあとに頼み直す'

# 載せ替えの口 andon issue restack（ADR-0067 決めたこと 13・受け入れ条件 9）。bare な origin と担当の clone で、前の課題の branch の上に
# 積んだ branch を作り、前の課題を main に入れてから打つ。偽の gh は呼び出しを $tmp/rs-log に 1 行ずつ足し（pr view は $tmp/rs-view を返す。
# pr edit --base と pr ready は $tmp/rs-view を書き換える）、origin の pre-receive は push を同じ記録に足す（$tmp/rs-pushfail があれば 1 回落ちる）
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
echo "gh \$*" >> "$tmp/rs-log"
case "\$1 \$2 \$4 \$5" in
  "pr view --json state,baseRefName,isDraft") cat "$tmp/rs-view" ;;
  "pr edit --base "*) jq --arg b "\$5" '.baseRefName = \$b' "$tmp/rs-view" > "$tmp/rs-view.new" && mv "$tmp/rs-view.new" "$tmp/rs-view" ;;
  "pr ready  ") jq '.isDraft = false' "$tmp/rs-view" > "$tmp/rs-view.new" && mv "$tmp/rs-view.new" "$tmp/rs-view" ;;
  "pr list example/app --head") cat "$tmp/rs-list" ;;   # branch から引いた PR の URL（--jq で 1 つ目の url を出した形。無ければ空）
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
stacked() { # stacked <名前> <squash|merge>: 積んだ課題を作り、id を $rs、worktree を $rg、分かれ目を $rf、積んだ commit を $rc に置く。前の課題 $rp は todo
    local o="$tmp/rs-$1.git"
    rg="$tmp/rs-$1"
    git init -q --bare -b main "$o"
    printf '#!/bin/sh\nif [ -e "%s" ]; then rm -f "%s"; echo "偽の push の失敗" >&2; exit 1; fi\necho push >> "%s"\n' "$tmp/rs-pushfail" "$tmp/rs-pushfail" "$tmp/rs-log" > "$o/hooks/pre-receive"
    chmod +x "$o/hooks/pre-receive"
    git clone -q "$o" "$rg" 2>/dev/null; git -C "$rg" symbolic-ref HEAD refs/heads/main
    printf 'a1\na2\na3\na4\na5\n' > "$rg/a.txt"; printf 'p1\n' > "$rg/p.txt"; printf 'm1\n' > "$rg/m.txt"
    git -C "$rg" add -A; git -C "$rg" commit -qm base; git -C "$rg" push -q origin main 2>/dev/null || die "$1: 見本の origin へ push できない"
    git -C "$rg" switch -qc prev; echo p2 >> "$rg/p.txt"; git -C "$rg" commit -qam 'prev change'; rf=$(git -C "$rg" rev-parse HEAD)
    git -C "$rg" push -q origin prev 2>/dev/null
    git -C "$rg" switch -qc stk; sed -i 's/^a5$/A5/' "$rg/a.txt"; git -C "$rg" commit -qam 'stacked change'; rc=$(git -C "$rg" rev-parse HEAD)
    git -C "$rg" push -q origin stk 2>/dev/null
    rp=$(as t5 kanban add "前の課題（$1）" --profile claude)
    rs=$(as t5 kanban add "積んだ課題（$1）" --profile claude --after "$rp")
    as t5 kanban set "$rs" status=doing review=on must_fix=major "worktree=$rg" branch=stk repo=github.com/example/app stacked_on="$rp" stacked_base="$rf" >/dev/null
    mkdir -p "kanban/issues/$rs/library"
    jq -n --arg c "$rc" --arg pr "$pr" '{commit: $c, pr: $pr}' > "kanban/issues/$rs/library/report.json"
    echo '{"state": "OPEN", "baseRefName": "prev", "isDraft": true}' > "$tmp/rs-view"
    # 前の課題を origin の main に入れる（別の clone から。担当の clone の origin/main は古いまま）
    git clone -q "$o" "$rg-m" 2>/dev/null; git -C "$rg-m" fetch -q origin prev
    if [ "$2" = squash ]; then git -C "$rg-m" merge -q --squash origin/prev >/dev/null; git -C "$rg-m" commit -qm 'prev (squash)'
    else git -C "$rg-m" merge -q --no-ff -m 'prev (merge)' origin/prev >/dev/null; fi
    git -C "$rg-m" push -q origin main 2>/dev/null || die "$1: 前の課題を origin の main へ入れられない"
    : > "$tmp/rs-log"
}
rkey() { kanban show "$rs" --json | jq -r '[.issue.stacked_on // "", .issue.stacked_base // "", .issue.restack_onto // ""] | join(",")'; }
restacked_ok() { # restacked_ok <場面>: 載せ替えが済んだ形（main の先・差分は自分の変更だけ・付け替えが push より先・draft が外れた・キーが消えた）
    git -C "$rg" fetch -q origin
    [ "$(git -C "$rg" merge-base HEAD origin/main)" = "$(git -C "$rg" rev-parse origin/main)" ] || die "$1: 既定の branch の先に載っていない: $(git -C "$rg" log --oneline -5)"
    [ "$(git -C "$rg" diff --name-only origin/main HEAD)" = a.txt ] || die "$1: 既定の branch との差分が自分の変更だけでない: $(git -C "$rg" diff --name-only origin/main HEAD)"
    [ "$(git -C "$rg" rev-parse origin/stk)" = "$(git -C "$rg" rev-parse HEAD)" ] || die "$1: 載せ替えた branch を push していない"
    grep -n -e '^gh pr edit' -e '^push$' -e '^gh pr ready' "$tmp/rs-log" | tail -3 | cut -d: -f2- | tr '\n' '|' > "$tmp/rs-order"
    [ "$(cat "$tmp/rs-order")" = "gh pr edit $pr --base main|push|gh pr ready $pr|" ] || die "$1: 付け替え・push・draft の順が違う: $(cat "$tmp/rs-log")"
    jq -e '.baseRefName == "main" and .isDraft == false' "$tmp/rs-view" >/dev/null || die "$1: 向き先が main でない・draft のまま: $(cat "$tmp/rs-view")"
    [ "$(rkey)" = ,, ] || die "$1: stacked_on・stacked_base・restack_onto が残った: $(rkey)"
}
stacked squash squash
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "前の課題が done でないのに restack が 0 で終わらない: $(cat "$tmp/out")"
grep -q 'まだ done でない' "$tmp/out" && [ "$(git -C "$rg" rev-parse HEAD)" = "$rc" ] && [ ! -s "$tmp/rs-log" ] && [ "$(rkey)" = "$rp,$rf," ] ||
    die "前の課題が done でないのに何か変えた: $(cat "$tmp/out") $(cat "$tmp/rs-log") $(rkey)"
ok 'restack: 前の課題がまだ done でなければ、何も変えずに 1 行出して 0 で終わる'
as t5 kanban set "$rp" status=done --force >/dev/null
expect_fail "前の課題 $rp が done になった。先に andon issue restack $rs" as t7 "$andon" issue ready "$rs"
ok 'ready: 積んだ課題の前の課題が done になったら、載せ替えるまで review にしない'
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "squash の前の課題のあとの restack が落ちた: $(cat "$tmp/out")"
restacked_ok squash
grep -q 'ふつうの課題になった' "$tmp/out" || die "載せ替えた 1 行を出さない: $(cat "$tmp/out")"
stacked merge merge
as t5 kanban set "$rp" status=done --force >/dev/null
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "マージ commit の前の課題のあとの restack が落ちた: $(cat "$tmp/out")"
restacked_ok merge
ok 'restack: 前の課題が squash でもマージ commit でも入ったあと、既定の branch の先へ載せ、差分は自分の変更だけ、付け替えを push より先に打ち、draft を外し、キーを消す'
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "済んだあとの restack が落ちた: $(cat "$tmp/out")"
grep -q '積んでいない' "$tmp/out" || die "済んだあとの restack が積んでいないと言わない: $(cat "$tmp/out")"
# PR を出したが報告に pr をまだ書いていない: branch から PR を引いて、付け替えと draft を外すことを飛ばさない
stacked nopr squash
jq -n --arg c "$rc" '{commit: $c}' > "kanban/issues/$rs/library/report.json"
echo "$pr" > "$tmp/rs-list"
as t5 kanban set "$rp" status=done --force >/dev/null
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "報告に pr の無い restack が落ちた: $(cat "$tmp/out")"
grep -q '^gh pr list --repo example/app --head stk ' "$tmp/rs-log" || die "報告に pr が無いのに branch から PR を引かない: $(cat "$tmp/rs-log")"
restacked_ok nopr
rm -f "$tmp/rs-list"
ok 'restack: 報告に pr がまだ無くても、branch から PR を引いて向き先の付け替えと draft を外すことを飛ばさない'
# 変種: branch から引いても PR が無い（まだ出していない）: push とキーを消すことだけをし、PR には触らない
stacked nopr-none squash
jq -n --arg c "$rc" '{commit: $c}' > "kanban/issues/$rs/library/report.json"
: > "$tmp/rs-list"
as t5 kanban set "$rp" status=done --force >/dev/null
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "PR の無い restack が落ちた: $(cat "$tmp/out")"
git -C "$rg" fetch -q origin
[ "$(git -C "$rg" diff --name-only origin/main HEAD)" = a.txt ] && [ "$(git -C "$rg" rev-parse origin/stk)" = "$(git -C "$rg" rev-parse HEAD)" ] && [ "$(rkey)" = ,, ] ||
    die "PR の無い restack が載せ替え・push・キーを消すことをしない: $(rkey) $(cat "$tmp/rs-log")"
if grep -q -e '^gh pr edit' -e '^gh pr ready' -e '^gh pr view' "$tmp/rs-log"; then die "PR が無いのに PR に触った: $(cat "$tmp/rs-log")"; fi
grep -q '^gh pr list --repo example/app --head stk ' "$tmp/rs-log" || die '前提: branch から PR を引いていない'
grep -q -e '^gh pr edit' <<<"gh pr edit $pr --base main" || die '上の否定の grep が、記録の付け替えの行に当たらない'
rm -f "$tmp/rs-list"
ok 'restack: branch から引いても PR が無ければ、載せ替えて push しキーを消すだけで、PR には触らない'

# 衝突: 前の課題のあとの main が、積んだ課題と同じ行を変えている。1 で止まり、解いて git rebase --continue のあと打ち直すと済む
stacked conflict squash
sed -i 's/^a5$/B5/' "$rg-m/a.txt"; git -C "$rg-m" commit -qam 'main changes a5'; git -C "$rg-m" push -q origin main 2>/dev/null; : > "$tmp/rs-log"
as t5 kanban set "$rp" status=done --force >/dev/null
expect_fail '衝突を解いて git rebase --continue' as t7 "$andon" issue restack "$rs"
[ "$(kanban show "$rs" --json | jq -r '.issue.stacked_on')" = "$rp" ] && ! grep -q '^push$' "$tmp/rs-log" || die "衝突したのにキーを消した・push した: $(rkey) $(cat "$tmp/rs-log")"
expect_fail 'rebase の途中' as t7 "$andon" issue restack "$rs"
sed -i 's/^<<<<<<<.*$//; s/^=======$//; s/^>>>>>>>.*$//; s/^B5$//' "$rg/a.txt"; sed -i '/^$/d' "$rg/a.txt"
git -C "$rg" add a.txt; GIT_EDITOR=true git -C "$rg" rebase --continue >/dev/null 2>&1 || die "衝突を解いた rebase が続かない: $(git -C "$rg" status --short)"
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "衝突を解いたあとの restack が落ちた: $(cat "$tmp/out") $(cat "$tmp/err" 2>/dev/null)"
restacked_ok conflict
ok 'restack: 衝突したら rebase の途中で 1 で止まり（途中で打ち直しても止まる）、解いて git rebase --continue のあと打ち直すと済む'

# push の失敗のあとの打ち直し: 分かれ目より後の main に、同じ行を 2 回変えた commit の並びが在る。済んだ rebase を打ち直すと main の
# commit を積み直して偽の衝突で止まるので、打ち直しは rebase をせずに続きから進む（事実 17）
stacked retry squash   # 場面の名前は置き場の名前になる（$tmp/rs-pushfail は push を落とす印なので使わない）
sed -i 's/^m1$/m2/' "$rg-m/m.txt"; git -C "$rg-m" commit -qam 'm to m2'
sed -i 's/^m2$/m3/' "$rg-m/m.txt"; git -C "$rg-m" commit -qam 'm to m3'; git -C "$rg-m" push -q origin main 2>/dev/null; : > "$tmp/rs-log"
as t5 kanban set "$rp" status=done --force >/dev/null
touch "$tmp/rs-pushfail"
expect_fail 'push できない' as t7 "$andon" issue restack "$rs"
[ "$(kanban show "$rs" --json | jq -r '.issue.restack_onto')" = "$(git -C "$rg" rev-parse origin/main)" ] || die "push の失敗のあと restack_onto が移す先でない: $(rkey)"
mid=$(git -C "$rg" rev-parse HEAD)
as t7 "$andon" issue restack "$rs" > "$tmp/out" || die "push の失敗のあとの打ち直しが落ちた: $(cat "$tmp/out")"
[ "$(git -C "$rg" rev-parse HEAD)" = "$mid" ] || die '打ち直しで rebase をやり直した（HEAD が変わった）'
restacked_ok retry
# 対照: 済んだ rebase を同じ分かれ目から打ち直すと、この origin では済まない（打ち直しが rebase をしないことに意味がある）
git -C "$rg" switch -q --detach "$mid"
if git -C "$rg" rebase -q --onto "$(git -C "$rg" rev-parse origin/main)" "$rf" >/dev/null 2>&1; then
    die '対照: 済んだ rebase を同じ分かれ目から打ち直しても衝突しない（この origin が事実 17 の形でない）'
fi
git -C "$rg" rebase --abort
ok 'restack: rebase のあとの push を 1 回失敗させてから打ち直すと、rebase をやり直さずに続きから済む（main に同じ行を 2 回変えた commit の並びがあっても）'

# 積んで起動する（ADR-0067 決めたこと 4・5・受け入れ条件 1・2）: 前の課題が review にいて、その PR の今の head に今の review の回の
# gate の記録（approve）があるとき、after でそれを待つ queued の課題に andon issue start を打つと、前の課題の PR の head から branch を切り、
# stacked_on・stacked_base を Chief of Staff の端末で書いて doing にする。条件を 1 つずつ欠くと、理由を 1 行出して断り、何も残さない。
# 偽の gh は前の課題の PR を $tmp/st-view で返す。案件の repo は start の場面の bare な origin とその clone（$appdir）
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
case "\$1 \$2 \$4 \$5" in
  "pr view --json state,headRefOid,headRefName") cat "$tmp/st-view" ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
git -C "$appdir" fetch -q origin main; git -C "$appdir" switch -qc stack-prev origin/main
echo prev > "$appdir/prev.txt"; git -C "$appdir" add prev.txt; git -C "$appdir" commit -qm 'prev change'
sp=$(git -C "$appdir" rev-parse HEAD); git -C "$appdir" push -q origin stack-prev 2>/dev/null; git -C "$appdir" switch -q --detach origin/main
pv() { jq -n --arg h "${2:-$sp}" --arg s "${1:-OPEN}" '{state: $s, headRefOid: $h, headRefName: "stack-prev"}' > "$tmp/st-view"; }
pv
gateat() { jq -n --arg h "$2" --arg at "$3" --arg d "${4:-approve}" '{at: $at, decision: $d, head: $h}' > "kanban/issues/$1/library/gate/$2.json"; }
prevof() { # prevof <タイトル>: review にいる前の課題（PR の head は $sp、今の回の gate の記録は approve）を作り、id を返す
    local p
    p=$(kanban add "$1" --repo github.com/example/app --profile claude --brief - <<<'# prev')
    keys "$p"; kanban start "$p" >/dev/null; kanban review "$p" >/dev/null
    mkdir -p "kanban/issues/$p/library/gate"
    jq -n --arg c "$sp" --arg pr "$pr" '{commit: $c, pr: $pr}' > "kanban/issues/$p/library/report.json"
    gateat "$p" "$sp" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "$p"
}
nextof() { # nextof <タイトル> <after…>: after で待つ queued の課題を作り、id を返す
    local n t=$1; shift
    n=$(kanban add "$t" --repo github.com/example/app --profile claude --after "$(IFS=,; echo "$*")" --brief - <<<'# next')
    keys "$n"; echo "$n"
}
sp1=$(prevof 'Stack prev')
sn1=$(nextof 'Stack next' "$sp1")
[ "$(field "$sn1" status)" = queued ] || die "前提: 後ろの課題が queued でない: $(field "$sn1" status)"
"$andon" waiting list --json > "$tmp/st-wait" || die "andon waiting list が落ちた"
jq -e --arg n "$sn1" --arg p "$sp1" '.stackable == [{id: $n, on: $p}]' "$tmp/st-wait" >/dev/null || die "stackable に出ない: $(jq -c .stackable "$tmp/st-wait")"
"$andon" issue start "$sn1" --dry-run --json > "$tmp/st-dry" || die "積む start の dry-run が落ちた: $(cat "$tmp/st-dry")"
jq -e --arg p "$sp1" --arg s "$sp" '.stacked_on == $p and .stacked_base == $s and .stack_branch == "stack-prev"
  and ([.checks[] | select(.name == "after" and .result == "ok")] | length) == 1' "$tmp/st-dry" >/dev/null || die "dry-run が積むことを出さない: $(cat "$tmp/st-dry")"
[ "$(field "$sn1" status)" = queued ] && [ -z "$(field "$sn1" stacked_on)" ] || die 'dry-run が kanban を書いた'
"$andon" issue start "$sn1" > "$tmp/out" || die "積む start が落ちた: $(cat "$tmp/err" 2>/dev/null) $(cat "$tmp/out")"
swt=$ANDON_WORKSPACE/.worktrees/app/demo-$sn1
[ "$(git -C "$swt" merge-base HEAD "$sp")" = "$sp" ] && [ "$(git -C "$swt" rev-parse HEAD)" = "$sp" ] || die "branch の根元が前の課題の PR の head でない: $(git -C "$swt" log --oneline -3)"
[ "$(git -C "$swt" rev-parse --abbrev-ref HEAD)" = "demo/$sn1-stack-next" ] || die "branch の名前: $(git -C "$swt" rev-parse --abbrev-ref HEAD)"
[ "$(field "$sn1" status)" = doing ] && [ "$(field "$sn1" stacked_on)" = "$sp1" ] && [ "$(field "$sn1" stacked_base)" = "$sp" ] || die "kanban: $(kanban show "$sn1" --json | jq -c .issue)"
jq -se --arg n "$sn1" '[.[] | select(.id == $n and (.change | has("stacked_on")))] | length == 1 and .[0].by == "terminal:t5"' kanban/log.jsonl >/dev/null || die '積んだ印を書いたのが Chief of Staff の端末でない'
grep -q "前の課題 \`$sp1\` の PR の branch \`stack-prev\`" "$tmp/hc/log" && grep -q -- "gh pr create --draft --base stack-prev" "$tmp/hc/log" || die "最初のメッセージに積んだことと PR の出し方が無い: $(tail -5 "$tmp/hc/log")"
ok 'start: 積める queued の課題は、前の課題の PR の head から branch を切り、stacked_on・stacked_base を Chief of Staff の端末で書いて doing にし、最初のメッセージに draft の PR の出し方を足す（受け入れ条件 1）'
"$andon" waiting list --json > "$tmp/st-wait"
jq -e '.stackable == []' "$tmp/st-wait" >/dev/null || die "起動した課題が stackable に残る: $(jq -c .stackable "$tmp/st-wait")"
ok 'waiting list: 起動した積んだ課題は stackable から外れる'

# 受け入れ条件 2: 条件を 1 つずつ欠いた課題。断ったあと、worktree も branch もキーも残さない
nothing_left() { # nothing_left <id> <タイトルの英字の短い形>
    [ ! -e "$ANDON_WORKSPACE/.worktrees/app/demo-$1" ] && [ -z "$(git -C "$appdir" branch --list "demo/$1-$2")" ] \
      && [ -z "$(git -C "$appdir" ls-remote --heads origin "demo/$1-$2")" ] && [ "$(field "$1" status)" = queued ] \
      && [ -z "$(field "$1" stacked_on)$(field "$1" stacked_base)$(field "$1" branch)$(field "$1" worktree)" ] || die "$1: 断ったのに何か残った: $(kanban show "$1" --json | jq -c .issue)"
}
sp2=$(prevof 'Stack prev two')
sq2=$(kanban add 'Other todo' --repo github.com/example/app --profile claude --brief - <<<'# x')
sn2=$(nextof 'Stack two' "$sp2" "$sq2")
expect_fail "の待つ相手が 2 つ残る" "$andon" issue start "$sn2"
nothing_left "$sn2" stack-two
sp3=$(kanban add 'Stack prev doing' --repo github.com/example/app --profile claude --brief - <<<'# x'); keys "$sp3"; kanban start "$sp3" >/dev/null
sn3=$(nextof 'Stack doing' "$sp3")
expect_fail "前の課題 $sp3 が review でない（doing）" "$andon" issue start "$sn3"
nothing_left "$sn3" stack-doing
sp4=$(prevof 'Stack prev held')
sn4=$(nextof 'Stack held' "$sp4")
kanban set "$sp4" "hold=$sq2" >/dev/null
expect_fail "前の課題 $sp4 が止められている" "$andon" issue start "$sn4"
nothing_left "$sn4" stack-held
kanban set "$sp4" hold= >/dev/null
sp5=$(prevof 'Stack prev nogate')
sn5=$(nextof 'Stack nogate' "$sp5")
rm "kanban/issues/$sp5/library/gate/$sp.json"
expect_fail "前の課題 $sp5 の今の head（${sp:0:12}）に、今の review の回の gate の記録が無い" "$andon" issue start "$sn5"
nothing_left "$sn5" stack-nogate
gateat "$sp5" "$sp" "2000-01-01T00:00:00Z"   # 前の review の回の記録は数えない
expect_fail "今の review の回の gate の記録が無い" "$andon" issue start "$sn5"
nothing_left "$sn5" stack-nogate
sp6=$(prevof 'Stack prev lib')
sn6=$(kanban add 'Stack lib' --repo github.com/example/lib --profile claude --after "$sp6" --brief - <<<'# x'); keys "$sn6"
expect_fail "前の課題 $sp6 の repo（github.com/example/app）が $sn6 の repo（github.com/example/lib）と違う" "$andon" issue start "$sn6"
[ -z "$(field "$sn6" stacked_on)$(field "$sn6" worktree)" ] && [ "$(field "$sn6" status)" = queued ] || die "repo の違う課題で何か残った"
ok 'start: 待つ相手が 2 つ残る・前の課題が doing・前の課題が止められている・前の課題の今の head に今の回の gate の記録が無い・repo が違う課題は、理由を 1 行出して断り、worktree も branch もキーも残さない（受け入れ条件 2）'
# GitHub と origin で確かめる所: PR が open でない・PR の今の head が gate の通した commit と違う・origin の branch の先が違う
sp7=$(prevof 'Stack prev gh')
sn7=$(nextof 'Stack gh' "$sp7")
pv MERGED
expect_fail "前の課題 $sp7 の PR が open でない（MERGED）" "$andon" issue start "$sn7"
nothing_left "$sn7" stack-gh
pv OPEN "$c1"
expect_fail "前の課題 $sp7 の PR の今の head（${c1:0:12}）が gate の通した commit（${sp:0:12}）と違う" "$andon" issue start "$sn7"
nothing_left "$sn7" stack-gh
pv
git -C "$appdir" switch -q stack-prev; echo more >> "$appdir/prev.txt"; git -C "$appdir" commit -qam 'prev moved'; git -C "$appdir" push -q origin stack-prev 2>/dev/null
git -C "$appdir" switch -q --detach origin/main; git -C "$appdir" update-ref refs/remotes/origin/stack-prev "$sp"
expect_fail "前の課題の branch origin/stack-prev の先（" "$andon" issue start "$sn7"
nothing_left "$sn7" stack-gh
ok 'start: 前の課題の PR が open でない・PR の今の head が gate の通した commit と違う・origin の branch の先が違う（取り直して見る）ときも断り、何も残さない'

echo "all passed ($pass)"
