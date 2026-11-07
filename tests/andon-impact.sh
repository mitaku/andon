#!/usr/bin/env bash
# `andon issue impact`（マージのあとの影響の事実。docs/contract.md §6・docs/dispatcher.md §2.5）。本物の台帳の CLI（kanban）と git、偽の gh で回す。
# origin とその clone（GHQ_ROOT）に、マージした PR と、同じ repo で動いている課題の branch を作り、衝突・重なり・依頼書の指し・もう含む・
# 無関係・読めない branch を見分けることを見る。台帳にも worktree にも書かない（読むだけ）。GitHub と本物の hc には出ない。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GOCACHE="$tmp/gocache" GOMODCACHE="$tmp/gomodcache" GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
mkdir -p "$HOME"
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
export GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1 GHQ_ROOT="$tmp/ghq"
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset KANBAN_ROOT HC_TERMINAL_ID
mkdir -p "$tmp/bin" "$tmp/gh"
pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }

# 案件 repo: origin と ghq の clone
git init -q -b main "$tmp/origin"
printf 'a\nb\nc\nd\ne\nf\ng\n' > "$tmp/origin/doc.md"; echo x > "$tmp/origin/other.md"
git -C "$tmp/origin" add -A; git -C "$tmp/origin" commit -qm base
git -C "$tmp/origin" config receive.denyCurrentBranch ignore
app=$tmp/ghq/github.com/example/app
git clone -q "$tmp/origin" "$app"

# 偽の gh: PR 1 の状態・マージの commit・ファイルは $tmp/gh/view.json
cat > "$tmp/bin/gh" <<EOF
#!/bin/sh
[ "\$*" = "pr view https://github.com/example/app/pull/1 --json state,mergeCommit,files" ] || { echo "unexpected gh \$*" >&2; exit 99; }
cat $tmp/gh/view.json
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH"

mkdir -p "$tmp/demo"; cd "$tmp/demo"
kanban init --prefix D --repo github.com/example/app >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t5 >/dev/null
L=$PWD/kanban
# task <題> <status> [brief]: branch demo/<id> を origin/main（その時点）から切り、worktree を置く
task() {
    local id wt; id=$(HC_TERMINAL_ID=t5 kanban add "$1" --profile claude --repo github.com/example/app --brief - <<<"${3:-やること: $1}")
    wt=$HOME/.herdr/worktrees/app/demo-$id
    git -C "$app" worktree add -q -b "demo/$id" "$wt" origin/main
    HC_TERMINAL_ID=t5 kanban set "$id" "branch=demo/$id" "worktree=$wt" >/dev/null
    (cd "$wt" && HC_TERMINAL_ID=t7 kanban start "$id" >/dev/null)
    [ "$2" = doing ] || (cd "$wt" && HC_TERMINAL_ID=t7 kanban review "$id" >/dev/null)
    echo "$id"
}
edit() { # edit <id> <file> <sed の式> [commit]
    local wt=$HOME/.herdr/worktrees/app/demo-$1
    sed -i "$3" "$wt/$2"
    [ "${4:-}" = nocommit ] || { git -C "$wt" add -A; git -C "$wt" commit -qm "$2"; }
}

m=$(task 'マージする' doing)
conf=$(task 'ぶつかる' review)
clean=$(task '同じファイルの別の所' doing)
dirty=$(task 'commit していない' doing)
brief=$(task '依頼書が指す' doing '入力（正）: doc.md の段')
none=$(task '関係ない' doing)
fresh=$(task 'まだ git add していない新しいファイル' doing)
gone=$(task 'worktree が読めない' doing)
locked=$(task 'worktree を確かめられない' doing)
edit "$m" doc.md 's/^a$/A merged/'
echo merged > "$HOME/.herdr/worktrees/app/demo-$m/new.md"; git -C "$HOME/.herdr/worktrees/app/demo-$m" add new.md; git -C "$HOME/.herdr/worktrees/app/demo-$m" commit -qm new
edit "$conf" doc.md 's/^a$/A mine/'
edit "$clean" doc.md 's/^g$/G mine/'
edit "$dirty" doc.md 's/^f$/F wip/' nocommit
edit "$brief" other.md 's/^x$/y/'
edit "$none" other.md 's/^x$/z/'
echo new > "$HOME/.herdr/worktrees/app/demo-$fresh/new.md"   # 未追跡（マージした PR も new.md を足す）
# レビュアー（親が m・profile *-review）は見ない
HC_TERMINAL_ID=t5 kanban add 'レビュー' --profile codex-review --parent "$conf" --round 1 >/dev/null
# m の PR をマージする（squash の代わりに m の commit を origin の main に入れる）
git -C "$HOME/.herdr/worktrees/app/demo-$m" push -q origin "HEAD:main"
sha=$(git -C "$HOME/.herdr/worktrees/app/demo-$m" rev-parse HEAD)
mkdir -p "$L/issues/$m/library"
echo '{"commit": "'"$sha"'", "pr": "https://github.com/example/app/pull/1"}' > "$L/issues/$m/library/report.json"
mkdir -p "$L/issues/$conf/library"
jq -n --arg c "$(git -C "$app" rev-parse "demo/$conf")" '{commit: $c, pr: "https://github.com/example/app/pull/9"}' > "$L/issues/$conf/library/report.json"

# 1. MERGED でなければ終了コード 4（何も出さない）
echo '{"state":"OPEN","mergeCommit":null,"files":[]}' > "$tmp/gh/view.json"
rc=0; "$andon" issue impact "$m" >"$tmp/out" 2>"$tmp/err" || rc=$?
[ "$rc" = 4 ] && [ ! -s "$tmp/out" ] && grep -q 'MERGED でない（OPEN）' "$tmp/err" || die "MERGED でないのに rc=$rc: $(cat "$tmp/err")"
ok 'PR がまだ MERGED でなければ終了コード 4 で何も出さない'

# 2. マージしたあと: 衝突・重なり・依頼書・関係なし
jq -n --arg o "$sha" '{state: "MERGED", mergeCommit: {oid: $o}, files: [{path: "doc.md"}, {path: "new.md"}]}' > "$tmp/gh/view.json"
# worktree はあるが git として読めない（.git の指す先が消えた）
echo "gitdir: $tmp/nowhere" > "$HOME/.herdr/worktrees/app/demo-$gone/.git"
# worktree の上の段に入れない（stat が EACCES）。無い（ENOENT）とは分けて error にする
mkdir -p "$tmp/locked"; git -C "$app" worktree move "$HOME/.herdr/worktrees/app/demo-$locked" "$tmp/locked/wt"
HC_TERMINAL_ID=t5 kanban set "$locked" "worktree=$tmp/locked/wt" >/dev/null
chmod 000 "$tmp/locked"
log0=$(wc -l < "$L/log.jsonl")
st0=$(git -C "$HOME/.herdr/worktrees/app/demo-$dirty" status --porcelain)
"$andon" issue impact "$m" > "$tmp/impact.json" || die "andon issue impact が落ちた: $(cat "$tmp/impact.json")"
I=$tmp/impact.json
jq -e --arg m "$m" --arg s "$sha" '.issue == $m and .sha == $s and .pr == "https://github.com/example/app/pull/1" and .files == ["doc.md", "new.md"]' "$I" >/dev/null || die "頭の事実が違う: $(jq -c 'del(.issues)' "$I")"
row() { jq -c --arg t "$1" '.issues[] | select(.issue == $t)' "$I"; }
[ "$(jq '.issues | length' "$I")" = 8 ] || die "見るのは同じ repo の doing／review の担当の8件（自分・レビュアーを除く）: $(jq -c '[.issues[].issue]' "$I")"
row "$conf" | jq -e '.status == "review" and .conflict == true and .conflict_files == ["doc.md"] and .overlap == ["doc.md"] and .contains == false and .pr == "https://github.com/example/app/pull/9"' >/dev/null || die "衝突: $(row "$conf")"
row "$clean" | jq -e '.conflict == false and .overlap == ["doc.md"]' >/dev/null || die "衝突しない重なり: $(row "$clean")"
row "$dirty" | jq -e '.conflict == false and .overlap == ["doc.md"]' >/dev/null || die "commit していない変更の重なり: $(row "$dirty")"
row "$brief" | jq -e '.conflict == false and .overlap == [] and .brief_refs == ["doc.md"]' >/dev/null || die "依頼書が指す: $(row "$brief")"
row "$none" | jq -e '.conflict == false and .overlap == [] and .brief_refs == [] and (has("error") | not)' >/dev/null || die "関係ない: $(row "$none")"
row "$fresh" | jq -e '.overlap == ["new.md"] and (has("error") | not)' >/dev/null || die "まだ git add していない新しいファイルの重なり: $(row "$fresh")"
chmod 755 "$tmp/locked"
if [ "$(id -u)" != 0 ]; then   # root は権限で止まらない
  row "$locked" | jq -e '.error | contains("worktree を確かめられない")' >/dev/null || die "worktree を stat できないのに error でない: $(row "$locked")"
fi
row "$gone" | jq -e '.error | contains("commit していない変更が読めない")' >/dev/null || die "worktree が読めないのに error でない: $(row "$gone")"
[ "$(wc -l < "$L/log.jsonl")" = "$log0" ] || die '台帳に書いた'
[ "$(git -C "$HOME/.herdr/worktrees/app/demo-$dirty" status --porcelain)" = "$st0" ] || die 'worktree を変えた'
ok '同じ repo の動いている課題ごとに、衝突（git merge-tree）・重なり（commit していない変更と未追跡のファイルも）・依頼書の指し・関係なし を出し、worktree が読めない・確かめられなければ error（台帳と worktree に書かない）'

# 3. もう含む（rebase 済み）・branch が読めない
git -C "$HOME/.herdr/worktrees/app/demo-$clean" -c core.editor=true rebase -q "$sha"
git -C "$app" worktree remove --force "$HOME/.herdr/worktrees/app/demo-$none"; git -C "$app" branch -q -D "demo/$none"
"$andon" issue impact "$m" > "$I"
row "$clean" | jq -e '.contains == true and .overlap == [] and .conflict == false' >/dev/null || die "もう含む: $(row "$clean")"
row "$none" | jq -e '.error | contains("demo/'"$none"'")' >/dev/null || die "branch が読めない: $(row "$none")"
ok 'branch がもうマージの commit を含めば contains、branch が読めなければ error で続ける'

# 4. repo・pr が無い課題は理由1行で止まる
x=$(HC_TERMINAL_ID=t5 kanban add 'repo の無い' --profile claude)
expect=0; "$andon" issue impact "$x" >/dev/null 2>"$tmp/err" || expect=$?
[ "$expect" = 1 ] && grep -q 'repo が無い' "$tmp/err" || die "repo の無い課題で rc=$expect: $(cat "$tmp/err")"
ok 'repo の無い課題は終了コード 1 と理由1行'

echo "all passed ($pass)"
