#!/usr/bin/env bash
# tests/adr-new.sh — tests/adr-new（ADR の番号を取る command。ADR-0055 決めたこと 6、受け入れ条件 5・6）。
#   一時の bare の repo（origin）と、その clone の本体の checkout と worktree 3 つで打つ。セルの本物の repo には触らない。
#   main の木は、ADR 2 本（0001・0002）と一覧・CHANGELOG・決定の記録を持つ小さな作り物で、tests/adr-index.sh を通る。
#   1 origin の枝の控えにだけある番号・worktree の無い手元の枝にだけある番号・追跡していない ADR だけがある別の worktree の番号を飛ばす
#   2 2 つの worktree で同時に打つと、違う番号の 2 つのファイルができる。鍵を持つ間は待つ
#   3 fetch できないと 0 でない終了で、ファイルを作らない
#   4 置いたファイルは 1 行目と状態行を持つ。名前の形と同じ名前を断る
#   5 置いたファイルに索引と「決めたこと」を書き足し、決定の記録を 1 件足すと、tests/adr-index.sh --root が通る
set -u
cd "$(dirname "$0")/.." || exit 2
here=$(pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/isolate.sh

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }
check() { if "${@:2}"; then pass "$1"; else fail "$1"; fi; }

export HOME=$tmp/home
mkdir -p "$HOME"
git config --global user.name test
git config --global user.email test@example.com
git config --global init.defaultBranch main
new=$here/tests/adr-new

# main の木: tests/adr-index.sh を通る小さな作り物
seed=$tmp/seed
mkdir -p "$seed/docs/adr" "$seed/docs/decisions"
adr() { # adr <番号> <名前> <木>: 索引と「決めたこと」を持つ ADR と、その決定の記録
  printf '# ADR-%s: 作り物 %s\n\n- 状態: **決定（2026-01-01）**\n\n## 決定の索引\n\n| # | 何を決めたか | 今の正 | 今 |\n|---|---|---|---|\n| 1 | 作り物 | この ADR | 生きている |\n\n## 決めたこと\n\n1. **作り物。**\n' \
    "$1" "$1" >"$3/docs/adr/$1-$2.md"
  printf '# ADR-%s の決定の記録\n' "$1" >"$3/docs/decisions/2026-01-01-adr-$1-$2.md"
}
adr 0001 first "$seed"
adr 0002 second "$seed"
printf '# 決定の記録\n\n## 14. 凍結した ADR の表\n\n| 日付 | 決定 | 出典 |\n|---|---|---|\n' >"$seed/docs/decisions.md"
printf '# 変更の記録\n\n## v0.0.1 — 2026-01-01\n\n- 作り物\n' >"$seed/CHANGELOG.md"
printf '# ADR\n\n## 一覧\n' >"$seed/docs/adr/README.md"
bash tests/adr-index.sh --root "$seed" --write-list || { echo "FAIL 作り物の一覧を書けない" >&2; exit 1; }
bash tests/adr-index.sh --root "$seed" >/dev/null || { echo "FAIL 作り物の main の木が tests/adr-index.sh を通らない" >&2; bash tests/adr-index.sh --root "$seed" >&2; exit 1; }

# scene <名前>: origin（bare）と clone（本体の checkout は main）と worktree w1・w2・w3 を作り、$s に置き場を入れる
scene() {
  s=$tmp/$1
  git init -q --bare "$s/origin.git"
  git clone -q "$s/origin.git" "$s/c" 2>/dev/null
  cp -r "$seed/." "$s/c/"
  git -C "$s/c" add -A && git -C "$s/c" commit -qm seed && git -C "$s/c" push -q origin main
  for w in w1 w2 w3; do git -C "$s/c" worktree add -q -b "$w" "$s/$w" main; done
}
# took <worktree> <番号>: そこで打ち、番号の名前のファイルを置いて 0 で終わったか
took() {
  local out
  out=$(cd "$1" && "$new" "$3" "番号を取る") || return 1
  [ "$out" = "$1/docs/adr/$2-$3.md" ] && [ -f "$out" ]
}
adrs() { find "$s" -path "$s/origin.git" -prune -o -name '[0-9][0-9][0-9][0-9]-*.md' -path '*/docs/adr/*' -print | sort; }

# 1 番号を集める所。どれも、それが無ければ 0003 を取る
scene remote
git clone -q "$s/origin.git" "$tmp/other" 2>/dev/null
adr 0005 remote-only "$tmp/other"
git -C "$tmp/other" switch -qc remote-only && git -C "$tmp/other" add -A && git -C "$tmp/other" commit -qm r && git -C "$tmp/other" push -q origin remote-only
rm -rf "$tmp/other"
check 'origin の枝の控えにだけある番号（打った時の fetch で来る）を飛ばす' took "$s/w1" 0006 next

scene local
git -C "$s/c" switch -qc local-only && adr 0005 local-only "$s/c" && git -C "$s/c" add -A && git -C "$s/c" commit -qm l && git -C "$s/c" switch -q main
check 'worktree の無い手元の枝にだけある番号を飛ばす' took "$s/w1" 0006 next

scene untracked
adr 0005 untracked "$s/w3"
check '追跡していない ADR だけがある別の worktree の番号を飛ばす' took "$s/w1" 0006 next
check '次に別の worktree で打つと、いま置いた追跡していない 0006 も飛ばして 0007 を取る' took "$s/w2" 0007 after

# 2 同時に打つ。鍵を持つ間は待ち、放すと違う番号の 2 つのファイルができる
scene race
lock=$(git -C "$s/w1" rev-parse --path-format=absolute --git-common-dir)/andon-adr-new.lock
exec {held}>>"$lock"
flock "$held"
(exec {held}>&-; cd "$s/w1" && "$new" race-one "一つ目" >"$tmp/race1" 2>&1) & # 鍵を持つ fd を子に残さない
p1=$!
(exec {held}>&-; cd "$s/w2" && "$new" race-two "二つ目" >"$tmp/race2" 2>&1) &
p2=$!
sleep 1
check '鍵を持つ間は、どちらもファイルを置かない' test -z "$(adrs | grep -- '-race-')"
exec {held}>&-
rc=0; wait "$p1" || rc=1; wait "$p2" || rc=1
got=$(adrs | grep -o '[0-9]\{4\}-race-[a-z]*' | sort)
check '2 つの worktree で同時に打つと、どちらも 0 で終わる' test "$rc" -eq 0
check '2 つの worktree で同時に打つと、違う番号の 2 つのファイルができる（0003・0004）' \
  test "$(cut -c1-4 <<<"$got" | sort | tr '\n' ' ')" = '0003 0004 '

# 3 fetch できない
scene nofetch
git -C "$s/c" remote set-url origin "$tmp/no-such-origin.git"
before=$(adrs)
check 'fetch できないと 0 でない終了' bash -c '! (cd "$1" && "$2" broken "取れない") 2>"$3"' _ "$s/w1" "$new" "$tmp/nofetch.err"
check 'そのとき理由を 1 行出す' grep -q 'git fetch origin が通らない' "$tmp/nofetch.err"
check 'そのときファイルを作らない' test "$(adrs)" = "$before"

# 3b 番号を集める所が 1 つでも読めなければ、0 でない終了で、ファイルを作らない（読めた所だけで取ると、worktree の無い枝の 0005 と重なる）。
#    偽の git は、名指した subcommand だけ落ち、ほかは本物に渡す
realgit=$(command -v git)
mkdir -p "$tmp/fakegit"
printf '#!/usr/bin/env bash\n[ "${1:-}" = "$FAIL_GIT" ] && { echo "偽の git: $1 が落ちる" >&2; exit 1; }\nexec %q "$@"\n' "$realgit" >"$tmp/fakegit/git"
chmod +x "$tmp/fakegit/git"
for sub in for-each-ref ls-tree worktree; do
  scene "fail-$sub"
  git -C "$s/c" switch -qc local-only && adr 0005 local-only "$s/c" && git -C "$s/c" add -A && git -C "$s/c" commit -qm l && git -C "$s/c" switch -q main
  before=$(adrs)
  check "git $sub が落ちると 0 でない終了" bash -c '! (cd "$1" && PATH="$4:$PATH" FAIL_GIT="$5" "$2" broken "取れない") 2>"$3"' _ "$s/w1" "$new" "$tmp/fail.err" "$tmp/fakegit" "$sub"
  check "git $sub が落ちると理由を出す" grep -q '番号を取らない' "$tmp/fail.err"
  check "git $sub が落ちるとファイルを作らない" test "$(adrs)" = "$before"
done
check '偽の git は落とさない subcommand を本物に渡す（上の落ちる行が見ている）' bash -c 'cd "$1" && PATH="$2:$PATH" FAIL_GIT=none "$3" passes "通る" >/dev/null' _ "$s/w1" "$tmp/fakegit" "$new"
check 'そのとき worktree の無い枝の 0005 を飛ばして 0006 を取る' test -f "$s/w1/docs/adr/0006-passes.md"

# 4 置いたファイルと、断る名前
scene shape
check '名前を受けて置く' took "$s/w1" 0003 shape
f=$s/w1/docs/adr/0003-shape.md
check '1 行目は「# ADR-0003: 番号を取る」' test "$(sed -n 1p "$f")" = '# ADR-0003: 番号を取る'
check '状態行は「- 状態: **提案**」' grep -qx -- '- 状態: \*\*提案\*\*' "$f"
before=$(adrs)
for bad in Upper 'has space' 'under_score' '' ../escape; do
  check "名前「$bad」を断る" bash -c '! (cd "$1" && "$2" "$3" "題") 2>/dev/null' _ "$s/w1" "$new" "$bad"
done
check '同じ名前の ADR が在れば断る' bash -c '! (cd "$1" && "$2" shape "題") 2>/dev/null' _ "$s/w1" "$new"
check '断ったときファイルを作らない' test "$(adrs)" = "$before"

# 5 置いたファイルに本文を書き足すと、tests/adr-index.sh を通る（受け入れ条件 6）
printf '\n## 決定の索引\n\n| # | 何を決めたか | 今の正 | 今 |\n|---|---|---|---|\n| 1 | 番号を取る | この ADR | 生きている |\n\n## 決めたこと\n\n1. **番号を取る。**\n' >>"$f"
check '索引と「決めたこと」だけで、決定の記録が無ければ tests/adr-index.sh が落ちる' bash -c '! bash tests/adr-index.sh --root "$1" >/dev/null' _ "$s/w1"
printf '# ADR-0003 の決定の記録\n' >"$s/w1/docs/decisions/2026-01-02-adr-0003-shape.md"
check '決定の記録を 1 件足すと tests/adr-index.sh --root が通る' bash tests/adr-index.sh --root "$s/w1"

exit $((fails > 0))
