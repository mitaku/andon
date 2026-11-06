#!/usr/bin/env bash
# tests/andon-init.sh — andon init x で建屋の repo を作り、偽のイメージ（一時ディレクトリ）で bin/bootstrap --dry-run を通す。
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
# 偽の stat: bin/bootstrap が見る建屋の repo の持ち主（stat -c %u）。既定は 1000、FAKE_OWNER で変える
printf '#!/bin/bash\necho "${FAKE_OWNER:-1000}"\n' >"$tmp/path/stat"
chmod 755 "$tmp/path/stat"
sandbox() { env -i PATH="$tmp/path" HOME="$home" ANDON_IMAGE_ROOT="$img" ANDON_WSL_PROBE="$tmp/no-wsl" "$@"; }

cd "$tmp" || exit 1
"$andon" init x --image example.com/andon:test --owner dev@example.com >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ ! -s "$tmp/err" ] && pass "andon init x は rc 0" || fail "init: rc=$r $(cat "$tmp/out" "$tmp/err")"
for f in andon.toml Dockerfile bin/bootstrap local/setup.sh local/aqua.yaml gate.local.json AGENTS.local.md wiki/index.md projects/.gitkeep .gitignore .dockerignore; do
  [ -f "andon-x/$f" ] || fail "andon-x/$f が無い"
done
[ -x andon-x/bin/bootstrap ] && [ -x andon-x/local/setup.sh ] && pass "bin/bootstrap と local/setup.sh は実行できる" || fail "実行の権限"
[ "$(grep -c . andon-x/Dockerfile)" = 3 ] && head -n 1 andon-x/Dockerfile | grep -qx 'FROM example.com/andon:test' && pass "Dockerfile は FROM ＋ 2行" || fail "Dockerfile: $(cat andon-x/Dockerfile)"
grep -qx 'slug = "x"' andon-x/andon.toml && grep -qx 'owner = "dev@example.com"' andon-x/andon.toml && pass "andon.toml に slug と owner" || fail "andon.toml: $(cat andon-x/andon.toml)"
find andon-x -name '*.tmpl' | grep -q . && fail "*.tmpl が残っている" || pass "*.tmpl は埋めて外す"
[ -e andon-x/compose.yaml ] || [ -e andon-x/.andon ] && fail "建屋の repo にパッケージのファイルがある" || pass "建屋の repo に compose.yaml が無い"

"$andon" init x >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q '上書きしない' "$tmp/err" && pass "空でないディレクトリには書かない" || fail "2回目: rc=$r $(cat "$tmp/err")"

cd andon-x || exit 1
# 建屋の repo の状態（種類・パス・中身）。--dry-run の前後で比べる
state() { find . -printf '%y %p %m\n' | sort; find . -type f -exec sha256sum {} + | sort; }
state >"$tmp/s0"
sandbox bash bin/bootstrap --dry-run >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && pass "bootstrap --dry-run は rc 0" || fail "dry-run: rc=$r $(cat "$tmp/out" "$tmp/err")"
state >"$tmp/s1"
cmp -s "$tmp/s0" "$tmp/s1" && pass "--dry-run は建屋の repo に何も書かない（.andon/ を作らない）" || fail "dry-run で変わった: $(diff "$tmp/s0" "$tmp/s1")"
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
grep -qx 'Host andon-x' "$conf" && grep -qx '  Port 2299' "$conf" && ! grep -qE '<(slug|ssh-port|portless-port)>' "$conf" && pass "ssh config に建屋の名前と .env の番号" || fail "ssh config: $(cat "$conf" 2>&1)"
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

# 建屋の repo の持ち主が uid 1000 でなければ、起こす前に止まって回り道を出す（down・status・host-install は止めない）
for sub in "" up sh rebuild; do
  sandbox FAKE_OWNER=1001 bash bin/bootstrap --dry-run $sub >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && grep -q 'uid 1001' "$tmp/err" && grep -q '回り道' "$tmp/err" && ! grep -q 'docker' "$tmp/out" \
    && pass "持ち主が 1001 なら '${sub:-enter}' は起こす前に止まる" || fail "持ち主 1001 で ${sub:-enter}: rc=$r $(cat "$tmp/out" "$tmp/err")"
done
sandbox FAKE_OWNER=1001 bash bin/bootstrap --dry-run down >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && pass "持ち主が 1001 でも down は通る" || fail "持ち主 1001 で down: rc=$r $(cat "$tmp/err")"

sed -i 's|^FROM .*|FROM ghcr.io/<owner>/andon:<版>|' Dockerfile
sandbox bash bin/bootstrap --dry-run >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'FROM を埋める' "$tmp/err" && pass "FROM が埋まっていなければ止まる" || fail "placeholder: rc=$r $(cat "$tmp/err")"

if command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  ANDON_IMAGE=andon-x:local ANDON_SLUG=x docker compose --project-directory "$PWD" -f "$andon_root/package/compose.yaml" config -q 2>"$tmp/err" &&
    pass "compose.yaml は docker compose が読める" || fail "docker compose config: $(cat "$tmp/err")"
else
  echo "skip docker compose config（docker compose が無い）"
fi

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
