#!/usr/bin/env bash
# tests/andon-init.sh — andon init x でセルの repo を作り、偽のイメージ（一時ディレクトリ）で bin/bootstrap --dry-run を通す。
# 偽のイメージは package/ を <根>/opt/andon に写したもの（ANDON_IMAGE_ROOT）。PATH に docker を置かない
# （本物の docker・HOME・hc・/workspace に触らない）。docker compose が手元にあれば、取り出した compose.yaml の形も確かめる。
set -u
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

img=$tmp/image home=$tmp/home
mkdir -p "$img/opt" "$home" "$tmp/path"
cp -R "$andon_root/package" "$img/opt/andon"
for t in bash cat cmp cp dirname grep head install mkdir mktemp mv printf rm sed tail; do
  ln -s "$(command -v "$t")" "$tmp/path/$t"
done
# 偽の stat: bin/bootstrap が見るセルの repo の持ち主（stat -c %u）。既定は 1000、FAKE_OWNER で変える
printf '#!/bin/bash\necho "${FAKE_OWNER:-1000}"\n' >"$tmp/path/stat"
chmod 755 "$tmp/path/stat"
sandbox() { env -i PATH="$tmp/path" HOME="$home" ANDON_IMAGE_ROOT="$img" ANDON_WSL_PROBE="$tmp/no-wsl" "$@"; }

cd "$tmp" || exit 1
"$andon" init x --image example.com/andon:test --owner dev@example.com >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ ! -s "$tmp/err" ] && pass "andon init x は rc 0" || fail "init: rc=$r $(cat "$tmp/out" "$tmp/err")"
for f in andon.toml Dockerfile bin/bootstrap local/setup.sh local/aqua.yaml gate.local.json AGENTS.local.md wiki/index.md files/.gitkeep projects/.gitkeep .gitignore .dockerignore; do
  [ -f "x-cell/$f" ] || fail "x-cell/$f が無い"
done
grep -qx 'files/' x-cell/.gitignore && fail ".gitignore が files/ を除いている" || pass "files/ は追跡する（.gitignore に無い）"
grep -qx 'artifacts/' x-cell/.gitignore && fail ".gitignore に artifacts/ が残っている" || pass ".gitignore に artifacts/ が無い"
[ -x x-cell/bin/bootstrap ] && [ -x x-cell/local/setup.sh ] && pass "bin/bootstrap と local/setup.sh は実行できる" || fail "実行の権限"
[ "$(grep -c . x-cell/Dockerfile)" = 3 ] && head -n 1 x-cell/Dockerfile | grep -qx 'FROM example.com/andon:test' && pass "Dockerfile は FROM ＋ 2行" || fail "Dockerfile: $(cat x-cell/Dockerfile)"
grep -qx 'slug = "x"' x-cell/andon.toml && grep -qx 'owner = "dev@example.com"' x-cell/andon.toml && pass "andon.toml に slug と owner" || fail "andon.toml: $(cat x-cell/andon.toml)"
find x-cell -name '*.tmpl' | grep -q . && fail "*.tmpl が残っている" || pass "*.tmpl は埋めて外す"
[ -e x-cell/compose.yaml ] || [ -e x-cell/.andon ] && fail "セルの repo にパッケージのファイルがある" || pass "セルの repo に compose.yaml が無い"

"$andon" init x >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q '上書きしない' "$tmp/err" && pass "空でないディレクトリには書かない" || fail "2回目: rc=$r $(cat "$tmp/err")"

cd x-cell || exit 1
# セルの repo の状態（種類・パス・中身）。--dry-run の前後で比べる
state() { find . -printf '%y %p %m\n' | sort; find . -type f -exec sha256sum {} + | sort; }
state >"$tmp/s0"
sandbox bash bin/bootstrap --dry-run >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && pass "bootstrap --dry-run は rc 0" || fail "dry-run: rc=$r $(cat "$tmp/out" "$tmp/err")"
state >"$tmp/s1"
cmp -s "$tmp/s0" "$tmp/s1" && pass "--dry-run はセルの repo に何も書かない（.andon/ を作らない）" || fail "dry-run で変わった: $(diff "$tmp/s0" "$tmp/s1")"
grep -q "^+ cp -R $img/opt/andon/compose.yaml .andon/compose.yaml$" "$tmp/out" && pass "compose.yaml をイメージから .andon/ に取り出す（と出す）" || fail "取り出し: $(cat "$tmp/out")"
mkdir .andon && echo '# 前の残り' >.andon/compose.yaml.new && echo '# 前の compose' >.andon/compose.yaml
state >"$tmp/s0"
sandbox bash bin/bootstrap --dry-run >/dev/null 2>&1
sandbox bash bin/bootstrap --dry-run rebuild >/dev/null 2>&1
sandbox bash bin/bootstrap --dry-run host-install >/dev/null 2>&1
state >"$tmp/s1"
cmp -s "$tmp/s0" "$tmp/s1" && pass "--dry-run は .andon/ の今あるファイルを消さず書き換えない" || fail "dry-run で変わった: $(diff "$tmp/s0" "$tmp/s1")"
rm -r .andon
grep -q '^+ docker build -t andon-x:local ' "$tmp/out" && pass "イメージを andon-x:local で焼く" || fail "build: $(cat "$tmp/out")"
grep -q '^+ docker compose --project-name andon-x --project-directory .* -f .andon/compose.yaml up -d$' "$tmp/out" && pass "compose の project は andon-x" || fail "up: $(cat "$tmp/out")"
grep -q '^+ docker compose .* exec .* dev herdr$' "$tmp/out" && pass "入口は herdr" || fail "enter: $(cat "$tmp/out")"

sandbox bash bin/bootstrap --dry-run host-install >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q "install -m 755 $home/.local/bin/andon-up" "$tmp/out" && [ ! -e "$home/.local" ] && [ ! -e "$home/.config" ] && pass "host-install --dry-run は HOME に書かない" || fail "host-install --dry-run: rc=$r $(cat "$tmp/out" "$tmp/err")"
# Use separate fake App homes so precedence and WSL cannot hit real settings.
sandbox HOME="$tmp/xdg-home" XDG_CONFIG_HOME="$tmp/xdg" bash bin/bootstrap --dry-run host-install >"$tmp/out" 2>&1
[ ! -e "$tmp/xdg" ] && grep -q "$tmp/xdg/herdr-app/mods/andon" "$tmp/out" \
  && pass "XDG の dry-run は置き場を示すだけ" || fail "XDG の dry-run"
sandbox HOME="$tmp/xdg-home" XDG_CONFIG_HOME="$tmp/xdg" bash bin/bootstrap host-install >"$tmp/out" 2>&1
cmp -s "$tmp/xdg/herdr-app/mods/andon/main.js" "$andon_root/package/host/andon/main.js" \
  && pass "XDG_CONFIG_HOME の App に置く" || fail "XDG の mod が無い"
sandbox HOME="$tmp/app-host-home" HERDR_APP_HOME="$tmp/app home" XDG_CONFIG_HOME="$tmp/unused-xdg" bash bin/bootstrap host-install >"$tmp/out" 2>&1
[ -f "$tmp/app home/mods/andon/main.js" ] && [ ! -e "$tmp/unused-xdg" ] \
  && pass "HERDR_APP_HOME が XDG より優先（空白を含むパス）" || fail "App home の優先順"
# WSL without an explicit App home stops before creating host configuration.
sandbox HOME="$tmp/wsl-home" WSL_DISTRO_NAME=sample WSL_INTEROP=sample XDG_CONFIG_HOME="$tmp/wsl-xdg" bash bin/bootstrap host-install >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && [ ! -e "$tmp/wsl-home" ] && [ ! -e "$tmp/wsl-xdg" ] && grep -q HERDR_APP_HOME "$tmp/err" \
  && pass "WSL は App home が未指定なら書き込む前に案内して止まる" || fail "WSL の未指定: rc=$r"
# The /proc branch is also fake, including the systemd interop name.
mkdir "$tmp/wsl probe"
for marker in WSLInterop WSLInterop-late; do
  : >"$tmp/wsl probe/$marker"
  sandbox HOME="$tmp/probe-home" ANDON_WSL_PROBE="$tmp/wsl probe" bash bin/bootstrap host-install >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && [ ! -e "$tmp/probe-home" ] && grep -q HERDR_APP_HOME "$tmp/err" \
    && pass "偽の $marker で WSL を検出し、書く前に止まる" || fail "WSL probe $marker: rc=$r"
  rm "$tmp/wsl probe/$marker"
done
sandbox HOME="$tmp/wsl-host-home" WSL_DISTRO_NAME=sample HERDR_APP_HOME="$tmp/windows-app" bash bin/bootstrap host-install >"$tmp/out" 2>&1
cmp -s "$tmp/windows-app/mods/andon/main.js" "$andon_root/package/host/andon/main.js" \
  && pass "WSL は明示された App home に置く" || fail "WSL の App mod が無い"
echo '// personal' >"$tmp/windows-app/mods/andon/main.js"
sandbox HOME="$tmp/wsl-host-home" WSL_DISTRO_NAME=sample HERDR_APP_HOME="$tmp/windows-app" bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '// personal' "$tmp/windows-app/mods/andon/main.js" \
  && pass "明示先の既存 mod も保護する" || fail "明示先の mod を上書きした"
echo SSH_PORT=2299 >.env
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ -x "$home/.local/bin/andon-up" ] && [ -f "$home/.config/herdr/config.toml" ] && pass "host-install は andon-up と herdr の設定を置く（偽の HOME）" || fail "host-install: rc=$r $(cat "$tmp/out" "$tmp/err")"
mod=$home/.config/herdr-app/mods/andon
cmp -s "$mod/main.js" "$andon_root/package/host/andon/main.js" && cmp -s "$mod/mod.json" "$andon_root/package/host/andon/mod.json" \
  && pass "host-install は App mod を置く" || fail "App mod が違うか無い"
app=$home/.config/herdr-app
ok=1
for name in inbox kanban-board context-header files-plus; do
  diff -r "$app/mods/$name" "$andon_root/package/host/$name" >/dev/null || ok=0
done
cmp -s "$app/layouts/andon.json" "$andon_root/package/host/layouts/andon.json" && cmp -s "$app/layouts/andon.svg" "$andon_root/package/host/layouts/andon.svg" || ok=0
[ "$ok" = 1 ] && pass "host-install は inbox・kanban-board・context-header・files-plus とレイアウト andon を App の置き場に置く" || fail "画面の mod かレイアウトが違うか無い: $(ls -R "$app" 2>&1 | head -20)"
echo '{"personal": true}' >"$app/layouts/andon.json"
echo '// personal' >"$app/mods/inbox/main.js"
rm "$app/mods/kanban-board/mod.json"
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '{"personal": true}' "$app/layouts/andon.json" && grep -qx '// personal' "$app/mods/inbox/main.js" && [ ! -e "$app/mods/kanban-board/mod.json" ] \
  && grep -q "layouts/andon.json はすでにあり" "$tmp/out" && grep -q "mods/inbox はすでにある" "$tmp/out" \
  && pass "host-install は今あるレイアウトと画面の mod（編集済み・不完全でも）を上書きしない" || fail "レイアウトか画面の mod を上書きした: $(cat "$tmp/out")"
grep -q 'layout.switch --name andon' "$tmp/out" && pass "host-install はレイアウトを andon に切り替える1手を出す" || fail "切り替えの案内が無い: $(cat "$tmp/out")"
echo '// personal' >"$mod/main.js"
rm "$mod/mod.json"
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '// personal' "$mod/main.js" && [ ! -e "$mod/mod.json" ] \
  && pass "既存 mod は編集済み・不完全でもディレクトリごと触らない" || fail "既存 mod を変更した"
rm -r "$mod"
ln -s "$home/missing-mod" "$mod"
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
[ -L "$mod" ] && [ ! -e "$home/missing-mod" ] \
  && pass "リンク切れの mod も触らない" || fail "mod のリンクを変更した"
conf=$home/.ssh/config.d/andon-x.conf
grep -qx 'Host andon-x' "$conf" && grep -qx '  Port 2299' "$conf" && ! grep -qE '<(slug|ssh-port|portless-port)>' "$conf" && pass "ssh config にセルの名前と .env の番号" || fail "ssh config: $(cat "$conf" 2>&1)"
echo '# 人が書いた' >"$home/.config/herdr/config.toml"
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '# 人が書いた' "$home/.config/herdr/config.toml" && pass "host-install は今あるファイルを上書きしない" || fail "上書きした"
cmp -s .andon/cloud-init.yaml "$andon_root/package/host/cloud-init.yaml" && pass "host-install は cloud-init の雛形を .andon/ に置く" || fail "cloud-init.yaml が無い"
echo '# 埋めた' >.andon/cloud-init.yaml
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '# 埋めた' .andon/cloud-init.yaml && pass "host-install は編集済みの cloud-init.yaml を残す" || fail "cloud-init.yaml を上書きした"
echo 'SSH_PORT=22/;s/x/y' >.env
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q '数字だけ' "$tmp/err" && pass "host-install は数字でない番号で止まる" || fail "番号の検査: rc=$r $(cat "$tmp/out" "$tmp/err")"
rm .env

# セルの repo の持ち主が uid 1000 でなければ、起こす前に止まって回り道を出す（down・status・host-install は止めない）
for sub in "" up sh rebuild; do
  sandbox FAKE_OWNER=1001 bash bin/bootstrap --dry-run $sub >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && grep -q 'uid 1001' "$tmp/err" && grep -q '回り道' "$tmp/err" && ! grep -q 'docker' "$tmp/out" \
    && pass "持ち主が 1001 なら '${sub:-enter}' は起こす前に止まる" || fail "持ち主 1001 で ${sub:-enter}: rc=$r $(cat "$tmp/out" "$tmp/err")"
done
sandbox FAKE_OWNER=1001 bash bin/bootstrap --dry-run down >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && pass "持ち主が 1001 でも down は通る" || fail "持ち主 1001 で down: rc=$r $(cat "$tmp/err")"

# rebuild --from: andon の checkout から image/build.sh で base image を焼き、FROM をそれにして build・up する（偽の docker）
co=$tmp/co
mkdir -p "$co/image" "$tmp/dpath"
cp "$andon_root/image/build.sh" "$andon_root/image/Dockerfile" "$co/image/"
git -C "$co" init -q && git -C "$co" add . && git -C "$co" -c user.name=t -c user.email=t@example.com commit -qm init
sha=$(git -C "$co" rev-parse HEAD)
cp -P "$tmp/path/"* "$tmp/dpath/"
for t in git env; do ln -s "$(command -v "$t")" "$tmp/dpath/$t"; done
# 偽の docker: 1回の呼び出しを1行に、argv を1つずつ [ ] で囲んで記録する（空白を含む引数の境界も比べる）。git は FAKE_SHA を答え、image inspect は FAKE_REVISION を答える
cat >"$tmp/dpath/docker" <<'SH'
#!/bin/bash
{ printf '[%s]' "$@"; echo; } >>"$FAKE_DOCKER_LOG"
case "$*" in
  *"rev-parse HEAD") echo "$FAKE_SHA" ;;
  "image inspect"*) [ -n "${FAKE_REVISION:-}" ] && echo "$FAKE_REVISION" || exit 1 ;;
esac
exit 0
SH
chmod 755 "$tmp/dpath/docker"
log=$tmp/docker.log
argv() { printf '[%s]' "$@"; }
dsandbox() { : >"$log"; env -i PATH="$tmp/dpath" HOME="$home" ANDON_IMAGE_ROOT="$img" FAKE_DOCKER_LOG="$log" FAKE_SHA="$sha" "$@"; }
cp Dockerfile "$tmp/Dockerfile.orig"
tag=andon:local-${sha:0:12}

dsandbox bash bin/bootstrap --dry-run rebuild --from "$co" >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -qxF "+ bash $co/image/build.sh $co $tag $sha" "$tmp/out" && cmp -s Dockerfile "$tmp/Dockerfile.orig" && [ ! -s "$log" ] \
  && pass "rebuild --from --dry-run は image/build.sh を出すだけ（FROM も docker も触らない）" || fail "rebuild --from --dry-run: rc=$r $(cat "$tmp/out" "$tmp/err" "$log")"

dsandbox bash bin/bootstrap rebuild --from "$co" >"$tmp/out" 2>"$tmp/err"; r=$?
want_build=$(argv build --file "$co/image/Dockerfile" --build-arg "ANDON_REVISION=$sha" --build-arg "ANDON_VERSION=${sha:0:12}" --tag "$tag" "$co")
mapfile -t calls <"$log"
[ "$r" = 0 ] && [ "${calls[1]:-}" = "$want_build" ] \
  && pass "rebuild --from は image/build.sh で checkout から $tag を焼く" || fail "rebuild --from の base: rc=$r $(cat "$tmp/err" "$log")"
[ "$(head -n 1 Dockerfile)" = "FROM $tag" ] && [ "$(tail -n +2 Dockerfile)" = "$(tail -n +2 "$tmp/Dockerfile.orig")" ] \
  && pass "rebuild --from は Dockerfile の FROM だけを $tag にする" || fail "FROM: $(cat Dockerfile)"
[ "${calls[2]:-}" = "$(argv build -t andon-x:local "$PWD")" ] \
  && pass "続けて FROM $tag でセルの image を焼く（--pull を付けない）" || fail "セルの build: $(cat "$log")"
[ "${calls[3]:-}" = "$(argv compose --project-name andon-x --project-directory "$PWD" -f .andon/compose.yaml up -d --force-recreate)" ] \
  && pass "そのあと compose で作り直す" || fail "up: $(cat "$log")"
grep -q "commit: $sha" "$tmp/out" && grep -q 'GHCR の版へ戻すなら' "$tmp/out" \
  && pass "元の commit と、GHCR の版へ戻す道を出す" || fail "出力: $(cat "$tmp/out")"

dsandbox FAKE_REVISION="$sha" bash bin/bootstrap status >"$tmp/out" 2>&1
grep -qxF "FROM: $tag（commit: $sha）" "$tmp/out" && pass "status は FROM と元の commit を出す" || fail "status: $(cat "$tmp/out")"
dsandbox bash bin/bootstrap status >"$tmp/out" 2>&1
grep -qxF "FROM: $tag（commit: 手元に無い）" "$tmp/out" && pass "status は image が手元に無いと言う" || fail "status（無い）: $(cat "$tmp/out")"

echo x >"$co/new-file"
dsandbox bash bin/bootstrap rebuild --from "$co" >"$tmp/out" 2>&1
grep -qF -- "$(argv --build-arg "ANDON_REVISION=$sha-dirty" --build-arg "ANDON_VERSION=${sha:0:12}-dirty" --tag "$tag-dirty" "$co")" "$log" && [ "$(head -n 1 Dockerfile)" = "FROM $tag-dirty" ] \
  && pass "汚れた checkout は -dirty の tag で焼く" || fail "dirty: $(cat "$tmp/out" "$log")"
rm "$co/new-file"

# ホストに git が無ければ、今の FROM の image の git で読む
rm "$tmp/dpath/git"
cp "$tmp/Dockerfile.orig" Dockerfile
dsandbox bash bin/bootstrap rebuild --from "$co" >"$tmp/out" 2>&1; r=$?
grep -qF "$(argv run --rm --network none -v "$co:/src:ro" example.com/andon:test git -c 'safe.directory=*' -C /src rev-parse HEAD)" "$log" && [ "$(head -n 1 Dockerfile)" = "FROM $tag" ] \
  && pass "ホストに git が無ければ docker の中の git で commit を読む" || fail "git 無し: rc=$r $(cat "$tmp/out" "$log")"

# checkout の path: 空白を含む・相対・git でない
ln -s "$(command -v git)" "$tmp/dpath/git"
sp="$tmp/co dir"
cp -R "$co" "$sp"
cp "$tmp/Dockerfile.orig" Dockerfile
dsandbox bash bin/bootstrap rebuild --from "$sp" >"$tmp/out" 2>"$tmp/err"; r=$?
mapfile -t calls <"$log"
[ "$r" = 0 ] && [ "${calls[1]:-}" = "$(argv build --file "$sp/image/Dockerfile" --build-arg "ANDON_REVISION=$sha" --build-arg "ANDON_VERSION=${sha:0:12}" --tag "$tag" "$sp")" ] && [ "$(head -n 1 Dockerfile)" = "FROM $tag" ] \
  && pass "空白を含む checkout の path でも焼ける" || fail "空白の path: rc=$r $(cat "$tmp/err" "$log")"
cp "$tmp/Dockerfile.orig" Dockerfile
dsandbox bash bin/bootstrap rebuild --from ../co >"$tmp/out" 2>"$tmp/err"; r=$?
mapfile -t calls <"$log"
[ "$r" = 0 ] && [ "${calls[1]:-}" = "$want_build" ] \
  && pass "相対の checkout の path は絶対にして渡す" || fail "相対の path: rc=$r $(cat "$tmp/err" "$log")"
mkdir -p "$tmp/nogit/image" && cp "$co/image/build.sh" "$co/image/Dockerfile" "$tmp/nogit/image/"
cp "$tmp/Dockerfile.orig" Dockerfile
dsandbox bash bin/bootstrap rebuild --from "$tmp/nogit" >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'commit が読めない' "$tmp/err" && ! grep -q "^\[build\]" "$log" && cmp -s Dockerfile "$tmp/Dockerfile.orig" \
  && pass "git でない checkout は焼く前に止まる（FROM も触らない）" || fail "git でない: rc=$r $(cat "$tmp/err" "$log")"

dsandbox bash bin/bootstrap rebuild --from "$tmp/nowhere" >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q "image/build.sh が無い" "$tmp/err" && ! grep -q "^\[build\]" "$log" && pass "andon の checkout でなければ止まる" || fail "--from 無効: rc=$r $(cat "$tmp/err")"
dsandbox bash bin/bootstrap rebuild --bogus >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && ! grep -q "^\[build\]" "$log" && pass "rebuild の知らない引数で止まる" || fail "rebuild --bogus: rc=$r"
cp "$tmp/Dockerfile.orig" Dockerfile
rm -rf .andon

sed -i 's|^FROM .*|FROM ghcr.io/<owner>/andon:<版>|' Dockerfile
sandbox bash bin/bootstrap --dry-run >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'FROM を埋める' "$tmp/err" && pass "FROM が埋まっていなければ止まる" || fail "placeholder: rc=$r $(cat "$tmp/err")"

if command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  ANDON_IMAGE=andon-x:local ANDON_SLUG=x docker compose --project-directory "$PWD" -f "$andon_root/package/compose.yaml" config -q 2>"$tmp/err" &&
    pass "compose.yaml は docker compose が読める" || fail "docker compose config: $(cat "$tmp/err")"
else
  echo "skip docker compose config（docker compose が無い）"
fi

# andon-up: <slug>-cell と前の形の andon-<slug> を1回ずつ起こす（偽の docker と HOME。本物の docker に触らない）
up=$tmp/up && mkdir -p "$up/src" "$up/home" "$up/path"
printf '#!/bin/sh\nexit 0\n' >"$up/path/docker" && chmod 755 "$up/path/docker"
for d in a-cell andon-b andon-c-cell d; do
  mkdir -p "$up/src/$d/bin"
  printf '#!/bin/sh\necho %s >>"%s"\n' "$d" "$up/started" >"$up/src/$d/bin/bootstrap"
  chmod 755 "$up/src/$d/bin/bootstrap"
done
env -i PATH="$up/path:/usr/bin:/bin" HOME="$up/home" ANDON_PLANTS="$up/src" sh "$andon_root/package/host/andon-up" >"$tmp/out" 2>&1; r=$?
got=$(sort "$up/started" 2>/dev/null | tr '\n' ' ')
[ "$r" = 0 ] && [ "$got" = "a-cell andon-b andon-c-cell " ] && pass "andon-up は *-cell と andon-* を1回ずつ起こす" || fail "andon-up: rc=$r 起こした: $got $(cat "$tmp/out")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
