#!/usr/bin/env bash
# tests/apply-main.sh — image/apply-main.sh（ADR-0027 の受け入れ条件 1〜7・10。S1）。
#   試しの repo（この repo の追跡しているファイルを写した commit。origin は一時の bare）と、偽のイメージ
#   （一時の opt/andon・kanban。kanban --version はイメージの commit を出す）に向けて当てる・状態を出す・戻すを回す。
#   go build は本物。andon boot と hc は偽物で、呼ばれた引数と、そのときの記録の commit を控える。sudo は使わない。
#   1 当てると中身がその commit の package/・schema/ に一致し、andon・kanban が置かれる  2 記録の3つの値
#   3 origin/main の外の commit を断る・作業ツリーの未 commit の変更が当たらない  4 go build が落ちても半端にならない
#   5 状態（当てる前・当てたあと・main が2つ進んだあと）  6 戻す（2回当ててから・andon と kanban を壊してから）
#   7 置き換えのあとに boot と dispatcher の片付けが1回ずつ  10 道具が package/ の下に無い（以上 S1）
#   8 当てる前から動いている端末の一覧を出し、止めない  9 image/ が変わっていたら警告して当てる・状態にも出す（S2）
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

# go の cache は今の置き場のまま（HOME を一時に向けると module を取り直し、読むだけのファイルが後片付けで消せない）
GOMODCACHE=$("$gobin" env GOMODCACHE) GOCACHE=$("$gobin" env GOCACHE)
export GOMODCACHE GOCACHE
export HOME=$tmp/home
mkdir -p "$HOME"
git config --global user.name test
git config --global user.email test@example.com
git config --global init.defaultBranch main

# 試しの repo: c0（イメージの元）→ c1（origin/main）。道具そのものは写さない（当てる commit のスクリプトに替わらない）
git init -q --bare "$tmp/origin.git"
git clone -q "$tmp/origin.git" "$tmp/repo" 2>/dev/null
git -C "$here" archive HEAD | tar -x -C "$tmp/repo"
rm -f "$tmp/repo/image/apply-main.sh"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm c0
c0=$(git -C "$tmp/repo" rev-parse HEAD)
echo "c1" >"$tmp/repo/c1.txt"
echo "# c1 で足した行" >>"$tmp/repo/package/writing.md"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" commit -qm c1
c1=$(git -C "$tmp/repo" rev-parse HEAD)
git -C "$tmp/repo" push -q origin main

# 偽のイメージ: c0 の package/・schema/ と、偽の andon・kanban
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
# 見回りの鍵（dispatcher の PassLock）。見回りのたびに鍵を取る版の dispatcher が作るファイル
passlock=$HOME/.local/state/andon/dispatcher/.pass.lock
mkdir -p "$(dirname "$passlock")"
: >"$passlock"
# 偽の dispatcher の実体: 鍵を取る版（実体に鍵のファイル名がある）と前の版（無い）。sleep の写しで、後ろに足した字は実行に効かない
cp "$(command -v sleep)" "$tmp/old-dispatcher"
cp "$(command -v sleep)" "$tmp/new-dispatcher"
printf 'andon/dispatcher/.pass.lock' >>"$tmp/new-dispatcher"
grep -aq '\.pass\.lock' "$tmp/old-dispatcher" && fail "前の版の偽の dispatcher に鍵のファイル名がある"
"$tmp/new-dispatcher" 600 & newd=$!
"$tmp/old-dispatcher" 600 & oldd=$!
trap 'kill "$newd" "$oldd" 2>/dev/null; rm -rf "$tmp"' EXIT
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
if run status && grep -q "イメージの版: ${c0:0:12}" "$tmp/out" && grep -q "当てた commit: 当てていない" "$tmp/out"; then
  pass "5 当てる前は「当てていない」とイメージの版"
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

# === 1・2・3b・7. 当てる（作業ツリーを汚してから） ===
echo "dirty-marker-$$" >>"$tmp/repo/package/writing.md"
grep -q "dirty-marker-$$" "$tmp/repo/package/writing.md" || fail "3 作業ツリーを汚せていない"
: >"$CALLS"
if run apply; then pass "1 origin/main の先頭を当てると rc 0"; else fail "1 当てる: $(cat "$tmp/out")"; fi
cp "$tmp/out" "$tmp/out_apply"
# 9. image/ が変わっていない間は警告が無い
if ! grep -q '警告' "$tmp/out_apply" && ! grep -q 'image変更' "$tmp/out_apply"; then
  pass "9 image/ が変わっていないときは警告が無い"
else
  fail "9 余計な警告: $(cat "$tmp/out_apply")"
fi
mkdir -p "$tmp/x1"
git -C "$tmp/repo" archive "$c1" package schema | tar -x -C "$tmp/x1"
if diff -r -x bin -x applied.json -x schema "$tmp/x1/package" "$img/andon" && diff -r "$tmp/x1/schema" "$img/andon/schema"; then
  pass "1 中身が c1 の package/・schema/ に一致（bin/・記録を除く）"
else
  fail "1 中身が c1 と違う"
fi
if "$img/andon/bin/andon" help >"$tmp/help" 2>&1 && ! grep -q image-andon "$tmp/help"; then
  pass "1 <根>/bin/andon help が rc 0（焼いた andon。イメージの偽物でない）"
else
  fail "1 <根>/bin/andon help: $(head -3 "$tmp/help")"
fi
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

# === 8. 動いている端末の一覧を出す。止めない ===
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
  && grep -q "イメージの版: ${c0:0:12}" "$tmp/out"; then
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

# === 8. hc ls が失敗したら、警告を出して当てる ===
: >"$HC_LS_FAIL"
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

# === 9. image/ が変わったら警告して当てる ===
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

# === 7. 鍵を取らない前の版の dispatcher は、鍵のファイルが残っていても止めない ===
[ -e "$passlock" ] || fail "7 鍵のファイルが残っていない（前の版を見分ける検査が何も見ていない）"
echo "$oldd" >"$DISPATCHER_PID"
: >"$CALLS"
if run apply && [ "$(grep -c '^hc \(keep\|kill\|rm\) ' "$CALLS")" = 0 ] && [ "$(grep -c '^andon boot ' "$CALLS")" = 1 ] \
  && grep -q '^hc get dispatcher' "$CALLS" && grep -q '確かめられないので、止めずに残した' "$tmp/out"; then
  pass "7 前の版の dispatcher は、鍵のファイルが残っていても止めず、boot だけ打ち直す"
else
  fail "7 前の版の dispatcher: $(cat "$CALLS") / $(cat "$tmp/out")"
fi
echo "$newd" >"$DISPATCHER_PID"

# === 6. 2回当ててから戻す ===
if run apply && [ "$(jq -r .image_commit "$rec")" = "${c0:0:12}" ]; then
  pass "6 2回目を当てても記録のイメージの版は最初のまま"
else
  fail "6 2回目を当てる: $(cat "$tmp/out")"
fi
: >"$CALLS"
# 見回りの途中（鍵を持っている）なら、外れるまで dispatcher を止めない
flock "$passlock" sh -c 'sleep 2; echo released >>"$CALLS"' &
holder=$!
sleep 0.5
if run revert && same_as "$tmp/initial/andon" "$tmp/initial/bin/kanban" && [ ! -e "$rec" ] && [ ! -e "$img/andon-image" ]; then
  pass "6 戻すと <根> と kanban が最初の中身に一致し、記録が無い（rc 0）"
else
  fail "6 戻す: $(cat "$tmp/out")"
fi
wait "$holder"
if [ "$(grep -n '^released' "$CALLS" | cut -d: -f1)" -lt "$(grep -n '^hc kill' "$CALLS" | cut -d: -f1)" ]; then
  pass "7 見回りの鍵が外れてから dispatcher を止める"
else
  fail "7 見回りの途中で止めた: $(cat "$CALLS")"
fi
if [ "$(grep -c '^andon boot .*applied=none$' "$CALLS")" = 1 ] && [ "$(grep -c '^hc kill t9 applied=none$' "$CALLS")" = 1 ]; then
  pass "7 戻したあとにも dispatcher の片付けと andon boot が1回ずつ"
else
  fail "7 戻したあとの呼び出し: $(cat "$CALLS")"
fi
if run status && grep -q "当てた commit: 当てていない" "$tmp/out"; then pass "6 戻したあとの状態は「当てていない」"; else fail "6 戻したあとの状態: $(cat "$tmp/out")"; fi

# === 6. andon と kanban を壊してから戻す ===
run apply || fail "6 当て直す: $(cat "$tmp/out")"
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

[ "$fails" = 0 ] || { echo "$fails 件失敗" >&2; exit 1; }
echo "all ok"
