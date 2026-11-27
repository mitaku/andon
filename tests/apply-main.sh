#!/usr/bin/env bash
# tests/apply-main.sh — image/apply-main.sh（ADR-0027 の受け入れ条件 1〜7・10。S1）。
#   試しの repo（この repo の作業ツリーを、commit していない変更ごと写した commit。origin は一時の bare）と、偽のイメージ
#   （一時の opt/andon・kanban。kanban --version はイメージの commit を出す）に向けて当てる・状態を出す・戻すを回す。
#   go build は本物。andon boot と hc は偽物で、呼ばれた引数と、そのときの記録の commit を控える。sudo は使わない。
#   1 当てると中身がその commit の package/・schema/ に一致し、andon・kanban が置かれる  2 記録の3つの値
#   3 origin/main の外の commit を断る・作業ツリーの未 commit の変更が当たらない  4 go build が落ちても半端にならない
#   5 状態（当てる前・当てたあと・main が2つ進んだあと）  6 戻す（6a 2回当ててから・6b andon と kanban を壊してから）
#   7 置き換えのあとに boot と dispatcher の片付けが1回ずつ（7a）・鍵を取らない前のバージョンの dispatcher は止めない（7b）  10 道具が package/ の下に無い（以上 S1）
#   13 ハーネスの受け口（<根>/bin/andon otel serve）を boot の前に止める（当てる・戻す）。ほかの実行ファイルの受け口は止めない。動いていなければ何もしない
#   14 PATH に go が無く mise があるときは、mise which go の前に Andon の checkout で mise install を打つ（偽の mise。ADR-0063 受け入れ条件 7）
#   8 当てる前から動いている端末の一覧を出し、止めない（8a）・hc ls が失敗したら警告して当てる（8b）  9 image/ が変わっていない間は警告が無い（9a）・変わっていたら警告して当てる・状態にも出す（9b）（S2）
#   P0-1 当てたあと andon version が commit と食い違いを返す（ADR-0028）
#   11 試しの repo はこの repo の作業ツリーを写す（commit していない変更を見落とさない）
#   12 記録に前に当てた commit と日時を新しい順に残す（前のバージョンの記録・読めない記録からも当てる）
set -u
cd "$(dirname "$0")/.." || exit 2
here=$(pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/isolate.sh

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

script=${ANDON_APPLY_SCRIPT:-$here/image/apply-main.sh}
gobin=$(command -v go || mise which go 2>/dev/null || true)
if [ -z "$gobin" ]; then
  if [ "${CI:-false}" = true ]; then echo 'FAIL CI requires go' >&2; exit 1; fi
  echo "skip: go が無い"; exit 0
fi
# go の shim（mise）は道具を HOME の下から引く。HOME を一時に向ける前に実体を引き、一時の HOME に go を取り直させない
gobin=$("$gobin" env GOROOT)/bin/go

# go の cache は今の置き場のまま（HOME を一時に向けると module を取り直し、読むだけのファイルが後片付けで消せない）
GOMODCACHE=$("$gobin" env GOMODCACHE) GOCACHE=$("$gobin" env GOCACHE)
export GOMODCACHE GOCACHE
export HOME=$tmp/home
mkdir -p "$HOME"
git config --global user.name test
git config --global user.email test@example.com
git config --global init.defaultBranch main

# $1 の作業ツリーを $2 へ写す: 追跡しているファイルを commit していない変更ごと、無視していない追跡外のファイルも。
# 無視している追跡外の .go も写す（go build は無視の設定に関係なくそれを組む）。消したファイルと、追跡外の入れ子の repo
# （ls-files が dir/ で出す）は写さない。HEAD を写すと、commit していない変更が黙って試しから外れる。
# 一時ディレクトリの根（$tmp の親）が作業ツリーの中にあれば、その中の追跡外のファイルも写さない。並んで走るほかの検査の
# 書きかけで、作業ツリーの中身ではない（写すと tar が「file changed as we read it」で落ちる）
copy_worktree() (
  set -o pipefail
  temp_root=$(cd "$(dirname "$tmp")" && pwd -P) || exit 1
  cd "$1" || exit 1
  top=$(pwd -P) others=()
  case $temp_root in "$top"/*) others=(":(exclude,literal)${temp_root#"$top"/}") ;; esac
  { git ls-files -z --cached \
    && git ls-files -z --others --exclude-standard -- "${others[@]}" \
    && git ls-files -z --others --ignored --exclude-standard -- '*.go' "${others[@]}"; } \
    | while IFS= read -r -d '' f; do
        case $f in */) continue ;; esac
        if [ -e "$f" ] || [ -L "$f" ]; then printf '%s\0' "$f"; fi
      done \
    | tar --null -T - -cf - | tar -x -C "$2"
)
# 写した repo の中身を全部 commit する。無視の設定に当たる写したファイル（無視している .go）も -f で入れる
commit_all() { git -C "$1" add -A -f && git -C "$1" commit -qm "$2"; }

# === 11. 試しの repo は作業ツリーを写す（commit していない変更・追跡外のファイル・無視している .go・消したファイル） ===
w11=$tmp/w11
git init -q "$w11"
mkdir -p "$w11/cmd/x" "$w11/internal/y"
echo 'package main // committed' >"$w11/cmd/x/main.go"
echo 'package y' >"$w11/internal/y/gone.go"
printf 'ignored_*\n' >"$w11/.gitignore"
git -C "$w11" add -A
git -C "$w11" commit -qm w11
echo 'package main // uncommitted' >"$w11/cmd/x/main.go"
echo 'package y // untracked' >"$w11/internal/y/new file.go"
rm "$w11/internal/y/gone.go"
echo 'package main // ignored' >"$w11/cmd/x/ignored_gen.go"
echo 'ignored' >"$w11/cmd/x/ignored_note.txt"
git init -q "$tmp/c11"
if copy_worktree "$w11" "$tmp/c11" && commit_all "$tmp/c11" c11 \
  && [ "$(git -C "$tmp/c11" ls-tree -r --name-only HEAD | tr '\n' ' ')" = ".gitignore cmd/x/ignored_gen.go cmd/x/main.go internal/y/new file.go " ] \
  && [ "$(git -C "$tmp/c11" show HEAD:cmd/x/main.go)" = 'package main // uncommitted' ]; then
  pass "11 試しの repo は作業ツリーを写して commit する（commit していない変更・追跡外のファイル・無視している .go を含み、消したファイルと無視している .go 以外を除く）"
else
  fail "11 作業ツリーの写し: $(git -C "$tmp/c11" ls-tree -r --name-only HEAD 2>&1 | tr '\n' ' ')"
fi
# 一時ディレクトリの根が作業ツリーの中（名前に空白と glob の字）: ほかの検査の追跡外のファイルと無視している .go は写さず、
# 追跡しているファイルと、根の外の似た名前のファイルは写す
t11="$w11/t [1]"
mkdir -p "$t11/mine" "$t11/other" "$w11/t 1"
echo tracked >"$t11/keep.txt"
git -C "$w11" add -- ":(literal)t [1]/keep.txt"
echo other >"$t11/other/half"
echo 'package x' >"$t11/other/ignored_half.go"
echo near >"$w11/t 1/near.txt"
c11t=$tmp/c11t
git init -q "$c11t"
if (tmp=$t11/mine && copy_worktree "$w11" "$c11t") && commit_all "$c11t" c11t \
  && [ "$(git -C "$tmp/c11t" ls-tree -r --name-only HEAD | tr '\n' ' ')" = ".gitignore cmd/x/ignored_gen.go cmd/x/main.go internal/y/new file.go t 1/near.txt t [1]/keep.txt " ]; then
  pass "11 一時ディレクトリの根が作業ツリーの中でも、ほかの検査の一時ファイルを写さない"
else
  fail "11 作業ツリーの中の一時ディレクトリ: $(git -C "$tmp/c11t" ls-tree -r --name-only HEAD 2>&1 | tr '\n' ' ')"
fi

# 試しの repo: c0（イメージの元。この repo の作業ツリー）→ c1（origin/main）。道具そのものは写さない（当てる commit のスクリプトに替わらない）
git init -q --bare "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$tmp/repo" 2>/dev/null
copy_worktree "$here" "$tmp/repo" || { echo "FAIL 作業ツリーを試しの repo へ写せない" >&2; exit 1; }
rm -f "$tmp/repo/image/apply-main.sh"
commit_all "$tmp/repo" c0 || { echo "FAIL 試しの repo の c0 を作れない" >&2; exit 1; }
c0=$(git -C "$tmp/repo" rev-parse HEAD)
echo "c1" >"$tmp/repo/c1.txt"
echo "# c1 で足した行" >>"$tmp/repo/package/writing.md"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm c1
c1=$(git -C "$tmp/repo" rev-parse HEAD)
git -C "$tmp/repo" push -q origin main

# 偽のイメージ: c0 の package/・schema/ と、偽の andon・`kanban`
img=$tmp/opt
mkdir -p "$img/andon" "$img/bin" "$tmp/fakebin"
git -C "$tmp/repo" archive "$c0" package schema | tar -x -C "$tmp"
cp -R "$tmp/package/." "$img/andon/"
cp -R "$tmp/schema" "$img/andon/schema"
rm -rf "$tmp/package" "$tmp/schema"
printf '#!/usr/bin/env bash\necho image-andon\n' >"$img/andon/bin/andon"
printf '#!/usr/bin/env bash\necho "kanban %s"\n' "${c0:0:12}" >"$img/bin/kanban"
chmod 755 "$img/andon/bin/andon" "$img/bin/kanban"
cp -a "$img" "$tmp/initial"

export ANDON_APPLY_REPO=$tmp/repo ANDON_APPLY_ROOT=$img/andon ANDON_APPLY_KANBAN=$img/bin/kanban ANDON_APPLY_STASH=$img/andon-image
export ANDON_APPLY_ANDON=$tmp/fakebin/andon CALLS=$tmp/calls.log
# 偽の andon（boot だけ）と hc: 呼ばれた引数と、そのときの記録の commit を控える
cat >"$tmp/fakebin/andon" <<'EOF'
#!/usr/bin/env bash
c=$(jq -r .commit "$ANDON_APPLY_ROOT/applied.json" 2>/dev/null || echo none)
echo "andon $* home=$ANDON_HOME applied=$c" >>"$CALLS"
# boot を打たれたときに、偽の受け口が動いていたか（boot は受け口が listen していなければ起動する）
p=$(cat "$OTEL_PID" 2>/dev/null) || p=
if [ -n "$p" ]; then
  s=$(cat "/proc/$p/stat" 2>/dev/null) || s=
  s=${s##*) }
  if [ -n "$s" ] && [ "${s%% *}" != Z ]; then echo "otel alive $p" >>"$CALLS"; else echo "otel gone $p" >>"$CALLS"; fi
fi
EOF
cat >"$tmp/fakebin/hc" <<'EOF'
#!/usr/bin/env bash
c=$(jq -r .commit "$ANDON_APPLY_ROOT/applied.json" 2>/dev/null || echo none)
echo "hc $* applied=$c" >>"$CALLS"
[ "$1" = ls ] && [ -e "$HC_LS_FAIL" ] && { echo "herdr: 接続できない" >&2; exit 1; }
[ "$1" = ls ] && [ -e "$HC_LS_EMPTY" ] && exit 0
[ "$1" = ls ] && { cat "$HC_LS" 2>/dev/null; exit 0; }
[ "$1 $2" = "get dispatcher" ] && echo '{"address":"t9","state":"live","foreground":{"name":"andon","pid":'"$(cat "$DISPATCHER_PID")"'}}'
exit 0
EOF
chmod 755 "$tmp/fakebin/andon" "$tmp/fakebin/hc"
export PATH=$tmp/fakebin:$(dirname "$gobin"):$PATH
# 見回りの鍵（dispatcher の PassLock）。見回りのたびに鍵を取るバージョンの dispatcher が作るファイル
passlock=$HOME/.local/state/andon/dispatcher/.pass.lock
mkdir -p "$(dirname "$passlock")"
: >"$passlock"
# 偽の dispatcher の実体: 鍵を取るバージョン（実体に鍵のファイル名がある）と前のバージョン（無い）。sleep の写しで、後ろに足した字は実行に効かない
cp "$(command -v sleep)" "$tmp/old-dispatcher"
cp "$(command -v sleep)" "$tmp/new-dispatcher"
printf 'andon/dispatcher/.pass.lock' >>"$tmp/new-dispatcher"
grep -aq '\.pass\.lock' "$tmp/old-dispatcher" && fail "前のバージョンの偽の dispatcher に鍵のファイル名がある"
"$tmp/new-dispatcher" 600 & newd=$!
"$tmp/old-dispatcher" 600 & oldd=$!
# 偽の受け口: argv が「<実行ファイル> otel serve」の、眠るだけのプロセス（argv[0] を exec -a で差し替える）。
# <根>/bin/andon のもの（当てる道具が止める）と、ほかの実行ファイルのもの（セルの本物に当たる。止めない）
mkdir -p "$tmp/rcvsrc"
printf 'package main\n\nimport "time"\n\nfunc main() { time.Sleep(10 * time.Minute) }\n' >"$tmp/rcvsrc/main.go"
(cd "$tmp/rcvsrc" && GOFLAGS= "$gobin" build -o "$tmp/rcv" main.go) || fail "13 偽の受け口を組めない"
export OTEL_PID=$tmp/otel.pid
rcvs=()
# 受け口を起動し、argv が差し替わるまで待つ（上限 10 秒）。差し替わる前に当てる道具が走ると、道具は受け口を見つけない。
# 待ちは時刻でなく argv で決める（負荷の下では exec が遅れる）
start_rcv() {
  local p argv
  (exec -a "$1" "$tmp/rcv" otel serve) &
  p=$!
  rcvs+=("$p")
  for _ in $(seq 100); do
    mapfile -d '' -t argv 2>/dev/null <"/proc/$p/cmdline" || argv=()
    [ "${argv[*]:-}" = "$1 otel serve" ] && return 0
    sleep 0.1
  done
  fail "13 偽の受け口（pid $p）の argv が 10 秒で差し替わらない"
}
# 見回りの鍵を持つ側を裏で起動し（pid は holder）、鍵を取った印が置かれるまで待つ（上限 30 秒）。
# 鍵を持つ側は、鍵を取ってから印を置き、$@ が返るまで鍵を持つ。待ちは時刻でなく印で決める（負荷の下では鍵を取るのが遅れる）
hold_passlock() {
  local held=$tmp/pass.held
  rm -f "$held"
  flock "$passlock" bash -c ': >"$1"; shift; "$@"' _ "$held" "$@" &
  holder=$!
  for _ in $(seq 300); do
    [ -e "$held" ] && return 0
    sleep 0.1
  done
  fail "鍵を持つ側が 30 秒で見回りの鍵を取らない"
}
# 当てる道具が見回りの鍵を待ち始める（鍵のファイルを fd 9 に開いた flock が /proc に現れる）まで待つ（上限 $1 秒）。
# 鍵を持つ側がこれを待ってから放すと、道具が鍵を見る前に鍵が外れる道が無くなる（負荷の下でも、鍵を待たない道具を見逃さない）
cat >"$tmp/lock-waiter" <<'EOF'
#!/usr/bin/env bash
lock=$(readlink -f "$PASSLOCK")
for _ in $(seq $(($1 * 10))); do
  for p in /proc/[0-9]*; do
    [ "$(readlink "$p/fd/9" 2>/dev/null)" = "$lock" ] || continue
    mapfile -d '' -t argv 2>/dev/null <"$p/cmdline" || continue
    [ "${argv[0]##*/}" = flock ] && exit 0
  done
  sleep 0.1
done
exit 1
EOF
chmod 755 "$tmp/lock-waiter"
export PASSLOCK=$passlock
alive() { local s; s=$(cat "/proc/$1/stat" 2>/dev/null) || return 1; s=${s##*) }; [ "${s%% *}" != Z ]; }
start_rcv "$tmp/other/bin/andon"
other_rcv=$!
trap 'kill "$newd" "$oldd" "${rcvs[@]}" 2>/dev/null; rm -rf "$tmp"' EXIT
export DISPATCHER_PID=$tmp/dispatcher.pid HC_LS=$tmp/hc-ls.json HC_LS_FAIL=$tmp/hc-ls-fail HC_LS_EMPTY=$tmp/hc-ls-empty
cat >"$HC_LS" <<'JSON'
{"terminals":[
 {"address":"t3","name":"dispatcher","state":"live"},
 {"address":"t7","name":"worker-alpha","state":"live","status":"working"},
 {"address":"t8","name":"worker-beta","state":"live","status":"done"},
 {"address":"t9x","name":"gone-gamma","state":"dead"}]}
JSON
echo "$newd" >"$DISPATCHER_PID"

run() { bash "$script" "$@" >"$tmp/out" 2>&1; }
same_as() { diff -r "$1" "$img/andon" >/dev/null && cmp -s "$2" "$img/bin/kanban"; }

# === 10. 道具は package/ の下に無い（イメージに入らない） ===
if [ -f image/apply-main.sh ] && [ -z "$(find package -name 'apply-main*')" ]; then
  pass "10 道具は image/apply-main.sh にあり、package/ の下に無い"
else
  fail "10 道具が無いか package/ の下にある"
fi

# === 5a. 当てる前 ===
if run status && grep -q "イメージのバージョン: ${c0:0:12}" "$tmp/out" && grep -q "当てた commit: 当てていない" "$tmp/out"; then
  pass "5 当てる前は「当てていない」とイメージのバージョン"
else
  fail "5 当てる前の状態: $(cat "$tmp/out")"
fi

# === 3a. origin/main の外の commit は断る ===
git -C "$tmp/repo" switch -qc side
echo side >"$tmp/repo/side.txt"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm side
side=$(git -C "$tmp/repo" rev-parse HEAD)
git -C "$tmp/repo" switch -q main
if ! run apply "$side" && same_as "$tmp/initial/andon" "$tmp/initial/bin/kanban" && [ ! -e "$img/andon-image" ] \
  && grep -q "origin/main" "$tmp/out"; then
  pass "3 origin/main の外の commit は断り、何も変えない"
else
  fail "3 origin/main の外の commit: rc を見る・$(cat "$tmp/out")"
fi

# === 1・2・3b・7a. 当てる（作業ツリーを汚してから） ===
echo "dirty-marker-$$" >>"$tmp/repo/package/writing.md"
grep -q "dirty-marker-$$" "$tmp/repo/package/writing.md" || fail "3 作業ツリーを汚せていない"
: >"$CALLS"
start_rcv "$img/andon/bin/andon"
rcv=$!
echo "$rcv" >"$OTEL_PID"
alive "$rcv" || fail "13 偽の受け口が動いていない"
if run apply; then pass "1 origin/main の先頭を当てると rc 0"; else fail "1 当てる: $(cat "$tmp/out")"; fi
cp "$tmp/out" "$tmp/out_apply"
# 13. 受け口は boot の前に止まり、boot が当てたパッケージで起動し直す。ほかの実行ファイルの受け口は止めない
if ! alive "$rcv" && grep -q "^otel gone $rcv\$" "$CALLS" && grep -q "受け口（pid $rcv）を止めた" "$tmp/out_apply"; then
  pass "13 当てると <根>/bin/andon の受け口を boot の前に止める"
else
  fail "13 当てたあとの受け口: $(cat "$CALLS") / $(cat "$tmp/out_apply")"
fi
if alive "$other_rcv"; then pass "13 ほかの実行ファイルの受け口は止めない"; else fail "13 ほかの実行ファイルの受け口を止めた"; fi
: >"$OTEL_PID"
# 9a. image/ が変わっていない間は警告が無い
if ! grep -q '警告' "$tmp/out_apply" && ! grep -q 'image変更' "$tmp/out_apply"; then
  pass "9 image/ が変わっていないときは警告が無い"
else
  fail "9 余計な警告: $(cat "$tmp/out_apply")"
fi
mkdir -p "$tmp/x1"
git -C "$tmp/repo" archive "$c1" package schema | tar -x -C "$tmp/x1"
if diff -r -x bin -x applied.json -x version.json -x schema "$tmp/x1/package" "$img/andon" && diff -r "$tmp/x1/schema" "$img/andon/schema"; then
  pass "1 中身が c1 の package/・schema/ に一致（bin/・記録を除く）"
else
  fail "1 中身が c1 と違う"
fi
if "$img/andon/bin/andon" help >"$tmp/help" 2>&1 && ! grep -q image-andon "$tmp/help"; then
  pass "1 <根>/bin/andon help が rc 0（焼いた andon。イメージの偽物でない）"
else
  fail "1 <根>/bin/andon help: $(head -3 "$tmp/help")"
fi
# ADR-0028 P0-1: andon version が当てた commit と食い違いを返す（受け入れ条件 3・4 の土台）
ver() { ANDON_HOME=$img/andon "$img/andon/bin/andon" version 2>&1; }
v=$(ver)
if [ "$(jq -r .commit <<<"$v")" = "$c1" ] && [ "$(jq -r .source <<<"$v")" = applied ] && [ "$(jq -r .drift <<<"$v")" = false ] \
  && [ -f "$img/andon/version.json" ]; then
  pass "P0-1 当てたあと andon version が c1・applied・食い違い無しを返す"
else
  fail "P0-1 当てたあとの andon version: $v"
fi
w=$img/andon/skills/worker/SKILL.md
if git -C "$tmp/repo" show "$c1:package/skills/worker/SKILL.md" | diff - "$w" >/dev/null; then
  pass "P0-1 記録の commit から当時の SKILL.md を復元すると、置き場のものと同じ"
else
  fail "P0-1 c1 の SKILL.md と置き場のものが違う"
fi
cp "$w" "$tmp/worker.bak"
echo "# 手で足した行" >>"$w"
v=$(ver)
if [ "$(jq -r .drift <<<"$v")" = true ] && [ "$(jq -r .commit <<<"$v")" = "$c1" ] \
  && ! git -C "$tmp/repo" show "$c1:package/skills/worker/SKILL.md" | diff - "$w" >/dev/null; then
  pass "P0-1 置き場の1ファイルを書き換えると食い違いが立つ（復元したものとも違う）"
else
  fail "P0-1 書き換えたあとの andon version: $v"
fi
cp "$tmp/worker.bak" "$w"
[ "$(ver | jq -r .drift)" = false ] || fail "P0-1 書き戻しても食い違いが消えない: $(ver)"
[ "$("$img/bin/kanban" --version)" = "kanban ${c1:0:12}" ] && pass "1 kanban --version が c1 の先頭12文字" \
  || fail "1 kanban --version: $("$img/bin/kanban" --version)"
rec=$img/andon/applied.json
if [ "$(jq -r .commit "$rec")" = "$c1" ] && [ "$(jq -r .image_commit "$rec")" = "${c0:0:12}" ] \
  && jq -r .applied_at "$rec" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z$'; then
  pass "2 記録に当てた commit・日時・イメージの元の commit（最初の kanban --version）"
else
  fail "2 記録: $(cat "$rec" 2>/dev/null)"
fi
if ! grep -rq "dirty-marker-$$" "$img/andon"; then pass "3 作業ツリーの未 commit の変更は当たらない"; else fail "3 汚した中身が当たった"; fi
git -C "$tmp/repo" checkout -q -- package/writing.md
if [ "$(grep -c "^andon boot home=$img/andon applied=$c1\$" "$CALLS")" = 1 ] \
  && [ "$(grep -c "^hc kill t9 applied=$c1\$" "$CALLS")" = 1 ] && [ "$(grep -c "^hc rm t9 applied=$c1\$" "$CALLS")" = 1 ] \
  && [ "$(grep -v '^hc ls ' "$CALLS" | grep -c 'applied=none')" = 0 ] \
  && [ "$(grep -n '^hc rm t9' "$CALLS" | cut -d: -f1)" -lt "$(grep -n '^andon boot' "$CALLS" | cut -d: -f1)" ]; then
  pass "7 置き換えのあとに dispatcher の片付けと andon boot が1回ずつ（片付けが先）"
else
  fail "7 当てたあとの呼び出し: $(cat "$CALLS")"
fi

# === 8a. 動いている端末の一覧を出す。止めない ===
if grep -q 'worker-alpha' "$tmp/out_apply" && grep -q 'worker-beta' "$tmp/out_apply" \
  && ! grep -q 'gone-gamma' "$tmp/out_apply" && ! grep -Eq '^  t[0-9a-z]+ dispatcher ' "$tmp/out_apply"; then
  pass "8 当てる前から動いている端末（dispatcher・動いていないものを除く）を出す"
else
  fail "8 端末の一覧: $(cat "$tmp/out_apply")"
fi
if ! grep -Eq '^hc (kill|keep|rm) (t7|t8)' "$CALLS" && grep -q '^hc kill t9 ' "$CALLS"; then
  pass "8 一覧の端末を止める呼び出しが無い（dispatcher を止める呼び出しはある）"
else
  fail "8 端末を止めた: $(cat "$CALLS")"
fi

# === 5b・5c. 当てたあと・main が2つ進んだあと ===
if run status && grep -q "当てた commit: $c1" "$tmp/out" && grep -q "origin/main からの遅れ: 0" "$tmp/out" \
  && grep -q "イメージのバージョン: ${c0:0:12}" "$tmp/out"; then
  pass "5 当てたあとは当てた commit と遅れ 0"
else
  fail "5 当てたあとの状態: $(cat "$tmp/out")"
fi
for n in 2 3; do
  echo "c$n" >"$tmp/repo/c$n.txt"
  git -C "$tmp/repo" add -A
  git -C "$tmp/repo" commit -qm "c$n"
done
git -C "$tmp/repo" push -q origin main
git -C "$tmp/repo" reset -q --hard "$c1" # 手元の origin/main は fetch で進む
if run status && grep -q "origin/main からの遅れ: 2" "$tmp/out"; then
  pass "5 main が2つ進むと遅れ 2"
else
  fail "5 進んだあとの状態: $(cat "$tmp/out")"
fi
git -C "$tmp/repo" reset -q --hard origin/main

# === 8b. hc ls が失敗したら、警告を出して当てる ===
: >"$HC_LS_FAIL"
at1=$(jq -r .applied_at "$rec")
if run apply && grep -q '警告: hc ls に失敗' "$tmp/out"; then
  pass "8 hc ls が失敗すると、一覧を出せない警告を出して当てる"
else
  fail "8 hc ls の失敗: $(cat "$tmp/out")"
fi
rm -f "$HC_LS_FAIL"
: >"$HC_LS_EMPTY"
if run apply && grep -q '警告: hc ls に失敗' "$tmp/out"; then
  pass "8 hc ls が rc 0 で空を返しても、一覧を出せない警告を出して当てる"
else
  fail "8 hc ls の空応答: $(cat "$tmp/out")"
fi
rm -f "$HC_LS_EMPTY"

# === 12. 記録に前に当てた commit と日時（新しい順）を残す。前のバージョンの記録・読めない記録からも当てる ===
# ここまでに c1 → origin/main の先頭（c3）→ c3 と3回当てた
c3=$(git -C "$tmp/repo" rev-parse origin/main)
# dispatcher を片付けて boot を打ち直した回は、切り替わった時刻（当てた時刻より後）を書き、次の回の previous に写す
if [ "$(jq -c '[.previous[].commit]' "$rec")" = "[\"$c3\",\"$c1\"]" ] && [ "$(jq -r '.previous[1].applied_at' "$rec")" = "$at1" ] \
  && [ "$(jq -r .commit "$rec")" = "$c3" ] \
  && [ "$(jq -r '[., .previous[]] | all(.switched_at | type == "string" and test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z$"))' "$rec")" = true ] \
  && [ "$(jq -r '.switched_at >= .applied_at' "$rec")" = true ] \
  && [ "$(sed -n 's/.*"commit": *"\([0-9a-f]*\)".*/\1/p' "$rec" | head -1)" = "$c3" ]; then
  pass "12 記録の previous に前に当てた commit と日時が新しい順にある（1行目だけを読む前のバージョンの読み方でも当てた commit を読む）"
else
  fail "12 記録の previous: $(cat "$rec")"
fi
echo "{\"commit\": \"$c1\", \"applied_at\": \"2026-01-01T00:00:00Z\", \"image_commit\": \"${c0:0:12}\"}" >"$rec"
if run apply && [ "$(jq -c .previous "$rec")" = "[{\"commit\":\"$c1\",\"applied_at\":\"2026-01-01T00:00:00Z\"}]" ]; then
  pass "12 previous の無い前のバージョンの記録から当てると、その1件を previous に写す"
else
  fail "12 前のバージョンの記録から当てる: $(cat "$rec") $(cat "$tmp/out")"
fi
echo '{"commit": ' >"$rec"
if run apply && [ "$(jq -c .previous "$rec")" = '[]' ] && [ "$(jq -r .commit "$rec")" = "$c3" ]; then
  pass "12 読めない記録から当てても落ちず、previous は空"
else
  fail "12 読めない記録から当てる: $(cat "$rec") $(cat "$tmp/out")"
fi

# === 9b. image/ が変わったら警告して当てる ===
printf 'x\n' >"$tmp/repo/image/drift-marker.txt"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm drift
git -C "$tmp/repo" push -q origin main
if run status && grep -q '警告' "$tmp/out" && grep -q 'image/drift-marker.txt' "$tmp/out"; then
  pass "9 状態を出す口にも image/ の変更の警告が出る"
else
  fail "9 状態の警告: $(cat "$tmp/out")"
fi
if run apply && grep -q '警告' "$tmp/out" && grep -q 'image/drift-marker.txt' "$tmp/out" \
  && [ "$(jq -r .commit "$rec")" = "$(git -C "$tmp/repo" rev-parse origin/main)" ]; then
  pass "9 警告を出して当てる（rc 0・記録は当てた commit）"
else
  fail "9 警告つきで当てる: $(cat "$tmp/out")"
fi

# === 4. go build が落ちる commit を当てても半端にならない ===
cp -a "$img" "$tmp/before4"
printf 'package main\nfunc broken( {\n' >"$tmp/repo/cmd/andon/broken.go"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm broken
git -C "$tmp/repo" push -q origin main
if ! run apply && diff -r "$tmp/before4/andon" "$img/andon" >/dev/null && cmp -s "$tmp/before4/bin/kanban" "$img/bin/kanban"; then
  pass "4 go build が落ちると rc が 0 でなく、<根>・記録・kanban は当てる前のまま"
else
  fail "4 落ちる commit: $(tail -3 "$tmp/out")"
fi
git -C "$tmp/repo" rm -q cmd/andon/broken.go
git -C "$tmp/repo" commit -qm fix
git -C "$tmp/repo" push -q origin main

# === 7b. 鍵を取らない前のバージョンの dispatcher は、鍵のファイルが残っていても止めない ===
[ -e "$passlock" ] || fail "7 鍵のファイルが残っていない（前のバージョンを見分ける検査が何も見ていない）"
echo "$oldd" >"$DISPATCHER_PID"
: >"$CALLS"
if run apply && [ "$(grep -c '^hc \(keep\|kill\|rm\) ' "$CALLS")" = 0 ] && [ "$(grep -c '^andon boot ' "$CALLS")" = 1 ] \
  && grep -q '^hc get dispatcher' "$CALLS" && grep -q '確かめられないので、止めずに残した' "$tmp/out" \
  && [ "$(jq -r '.switched_at // "none"' "$rec")" = none ]; then
  pass "7 前のバージョンの dispatcher は、鍵のファイルが残っていても止めず、boot だけ打ち直す（記録に切り替わった時刻を書かない）"
else
  fail "7 前のバージョンの dispatcher: $(cat "$CALLS") / $(cat "$tmp/out")"
fi
echo "$newd" >"$DISPATCHER_PID"

# === 13. dispatcher の見回りが終わらず boot を打たない道でも、受け口は止めて当てたパッケージで起動し直す（dispatcher は止めない） ===
: >"$CALLS"
start_rcv "$img/andon/bin/andon"
rcv=$!
echo "$rcv" >"$OTEL_PID"
# 鍵は当てる道具が返るまで持つ（放す合図のファイルが置かれるまで。上限 300 秒で放し、当てる道具が返らなくても場面を終える）
release=$tmp/pass.release
rm -f "$release"
hold_passlock bash -c 'for _ in $(seq 3000); do [ -e "$1" ] && exit 0; sleep 0.1; done; exit 1' _ "$release"
ANDON_APPLY_PASS_WAIT=1 run apply
: >"$release"
wait "$holder" || fail "13 鍵を持つ側が、当てる道具が返る前に上限の 300 秒で鍵を放した"
for _ in $(seq 30); do grep -q '^andon otel serve ' "$CALLS" && break; sleep 0.1; done
if ! alive "$rcv" && grep -q "受け口（pid $rcv）を止めた" "$tmp/out" && grep -q '15分終わらない' "$tmp/out" \
  && [ "$(grep -c '^andon otel serve ' "$CALLS")" = 1 ] && ! grep -q '^andon boot ' "$CALLS" && ! grep -q '^hc kill ' "$CALLS"; then
  pass "13 見回りが終わらず boot を打たない道でも、受け口を止めて当てたパッケージで起動し直す（dispatcher は止めない）"
else
  fail "13 見回りが終わらない道の受け口: $(cat "$CALLS") / $(cat "$tmp/out")"
fi
: >"$OTEL_PID"

# === 6a. 2回当ててから戻す ===
if run apply && [ "$(jq -r .image_commit "$rec")" = "${c0:0:12}" ]; then
  pass "6 2回目を当てても記録のイメージのバージョンは最初のまま"
else
  fail "6 2回目を当てる: $(cat "$tmp/out")"
fi
: >"$CALLS"
# 見回りの途中（鍵を持っている）なら、外れるまで dispatcher を止めない。鍵を持つ側は、戻す道具が鍵を待ち始めてから放す
# （上限 60 秒。待ち始めなければ、その印を残して放す）
hold_passlock sh -c '"$1" 60 || echo "lock-waiter-missing" >>"$CALLS"; echo released >>"$CALLS"' _ "$tmp/lock-waiter"
start_rcv "$img/andon/bin/andon"
rcv=$!
echo "$rcv" >"$OTEL_PID"
if run revert && same_as "$tmp/initial/andon" "$tmp/initial/bin/kanban" && [ ! -e "$rec" ] && [ ! -e "$img/andon-image" ]; then
  pass "6 戻すと <根> と kanban が最初の中身に一致し、記録が無い（rc 0）"
else
  fail "6 戻す: $(cat "$tmp/out")"
fi
wait "$holder"
if grep -q '^lock-waiter-missing' "$CALLS"; then
  fail "7 戻す道具が見回りの鍵を待たなかった（鍵を fd 9 に開いた flock が 60 秒現れない）: $(cat "$CALLS")"
elif [ "$(grep -n '^released' "$CALLS" | cut -d: -f1)" -lt "$(grep -n '^hc kill' "$CALLS" | cut -d: -f1)" ]; then
  pass "7 見回りの鍵が外れてから dispatcher を止める"
else
  fail "7 見回りの途中で止めた: $(cat "$CALLS")"
fi
if [ "$(grep -c '^andon boot .*applied=none$' "$CALLS")" = 1 ] && [ "$(grep -c '^hc kill t9 applied=none$' "$CALLS")" = 1 ]; then
  pass "7 戻したあとにも dispatcher の片付けと andon boot が1回ずつ"
else
  fail "7 戻したあとの呼び出し: $(cat "$CALLS")"
fi
if ! alive "$rcv" && grep -q "^otel gone $rcv\$" "$CALLS" && grep -q "受け口（pid $rcv）を止めた" "$tmp/out"; then
  pass "13 戻すときも <根>/bin/andon の受け口を boot の前に止める"
else
  fail "13 戻したあとの受け口: $(cat "$CALLS") / $(cat "$tmp/out")"
fi
: >"$OTEL_PID"
if run status && grep -q "当てた commit: 当てていない" "$tmp/out"; then pass "6 戻したあとの状態は「当てていない」"; else fail "6 戻したあとの状態: $(cat "$tmp/out")"; fi

# === 6b. andon と kanban を壊してから戻す ===
# 13. 受け口が動いていなければ、当てる道具は受け口に触らずに当てる（受け口を起動するかは boot が決める。hc があれば起動する〈ADR-0060 決めたこと 4〉）
: >"$CALLS"
if run apply && ! grep -q '受け口' "$tmp/out" && [ "$(grep -c '^andon boot ' "$CALLS")" = 1 ] && ! grep -q '^otel ' "$CALLS"; then
  pass "13 受け口が動いていなければ、当てる道具は受け口に触らずに当てる（rc 0）"
else
  fail "13 受け口の無いときの当て直し: $(cat "$CALLS") / $(cat "$tmp/out")"
fi
echo broken >"$img/andon/bin/andon"
chmod 644 "$img/andon/bin/andon"
echo broken >"$img/bin/kanban"
if run revert && same_as "$tmp/initial/andon" "$tmp/initial/bin/kanban" && [ ! -e "$rec" ]; then
  pass "6 andon と kanban が壊れていても戻せる（rc 0）"
else
  fail "6 壊れてから戻す: $(cat "$tmp/out")"
fi
run revert && grep -q "当てていない" "$tmp/out" && pass "6 当てていないときの revert は rc 0 で何もしない" || fail "6 二度目の revert: $(cat "$tmp/out")"

# === 当てる commit のスクリプトで当てる（ADR-0027 決めたこと 5） ===
mkdir -p "$tmp/repo/image"
sed 's/^say() { echo "\[apply-main\] \$\*"; }$/say() { echo "[apply-main-committed] $*"; }/' "$script" >"$tmp/repo/image/apply-main.sh"
grep -q 'apply-main-committed' "$tmp/repo/image/apply-main.sh" || fail "決めたこと 5 の印を入れられていない"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm script
git -C "$tmp/repo" push -q origin main
if run apply && grep -q '^\[apply-main-committed\] .* を当てた' "$tmp/out"; then
  pass "決めたこと 5: 当てる commit の image/apply-main.sh で当てる"
else
  fail "決めたこと 5: $(cat "$tmp/out")"
fi
run revert || fail "決めたこと 5 のあとの revert: $(cat "$tmp/out")"

# === 14. PATH に go が無く mise があるときは、mise which go の前に Andon の checkout で mise install を打つ（ADR-0063 決めたこと 8・受け入れ条件 7） ===
# 偽の mise は呼ばれた引数と cwd を控え、which go に本物の go の場所を答える（本物の mise install は打たない）。
# PATH は偽の mise を先頭に置き、go を持つフォルダ（mise の shims・本物の go の置き場）を外す
nogo=$tmp/nogo
mkdir -p "$nogo"
printf '#!/usr/bin/env bash\necho "mise $* | cwd=$PWD" >>"$CALLS"\n[ "$1 $2" != "which go" ] || echo %q\n' "$gobin" >"$nogo/mise"
chmod 755 "$nogo/mise"
nogo_path=$nogo
IFS=: read -ra path_dirs <<<"$PATH"
# go のあるフォルダは丸ごと外さず、go だけを除いた写し（symlink）に替える（CI では go が bash と同じ /usr/bin にある）
n=0
for d in "${path_dirs[@]}"; do
  if [ -x "$d/go" ]; then
    n=$((n + 1))
    mkdir -p "$tmp/nogo-$n"
    for f in "$d"/*; do
      if [ "${f##*/}" != go ]; then ln -s "$f" "$tmp/nogo-$n/"; fi
    done
    nogo_path=$nogo_path:$tmp/nogo-$n
  else
    nogo_path=$nogo_path:$d
  fi
done
# go がある PATH では mise を呼ばない（検査が当たる場合があること: 次の場面と PATH の違いだけで分かれる）
: >"$CALLS"
if PATH=$nogo:$PATH run apply && grep -q 'を当てた' "$tmp/out" && ! grep -q '^mise ' "$CALLS"; then
  pass "14 PATH に go があれば mise を呼ばない"
else
  fail "14 go があるのに mise を呼んだ: $(grep '^mise ' "$CALLS" | tr '\n' ' ') / $(cat "$tmp/out")"
fi
: >"$CALLS"
if ! PATH=$nogo_path bash -c 'command -v go' >/dev/null && PATH=$nogo_path run apply && grep -q 'を当てた' "$tmp/out" \
  && [ "$(grep '^mise ' "$CALLS")" = "$(printf 'mise install | cwd=%s\nmise which go | cwd=%s' "$ANDON_APPLY_REPO" "$ANDON_APPLY_REPO")" ]; then
  pass "14 PATH に go が無いときは、mise which go の前に Andon の checkout で mise install を打つ"
else
  fail "14 mise install と mise which go の順: $(grep '^mise ' "$CALLS" | tr '\n' ' ') / $(cat "$tmp/out")"
fi

# === go の道具を一時の HOME に取り直さない（mise の shim が HOME の下に go を入れ直すと、遅くなり、網が無いと落ちる） ===
if [ ! -e "$HOME/.local/share/mise" ]; then
  pass "go の道具を一時の HOME（$HOME）に取り直さない"
else
  fail "一時の HOME に mise が道具を入れた: $(ls "$HOME/.local/share/mise/installs" 2>&1 | tr '\n' ' ')"
fi

[ "$fails" = 0 ] || { echo "$fails 件失敗" >&2; exit 1; }
echo "all ok"
