#!/usr/bin/env bash
# tests/andon-init.sh — andon cell init x でセルの repo を作り、偽のイメージ（一時ディレクトリ）で bin/bootstrap --dry-run を通す。
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
# 偽の stat: bin/bootstrap が見るセルの repo のオーナー（stat -c %u）。既定は 1000、FAKE_OWNER で変える
printf '#!/bin/bash\necho "${FAKE_OWNER:-1000}"\n' >"$tmp/path/stat"
chmod 755 "$tmp/path/stat"
sandbox() { env -i PATH="$tmp/path" HOME="$home" GIT_CEILING_DIRECTORIES="$GIT_CEILING_DIRECTORIES" ANDON_IMAGE_ROOT="$img" ANDON_WSL_PROBE="$tmp/no-wsl" "$@"; }

cd "$tmp" || exit 1
"$andon" cell init x --image example.com/andon:test --owner dev@example.com >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ ! -s "$tmp/err" ] && pass "andon cell init x は rc 0" || fail "init: rc=$r $(cat "$tmp/out" "$tmp/err")"
for f in andon.toml Dockerfile bin/bootstrap local/setup.sh local/aqua.yaml gate.local.json AGENTS.local.md wiki/index.md wiki/log.md wiki/overview.md wiki/raw/.gitkeep wiki/sources/.gitkeep wiki/entities/.gitkeep wiki/topics/.gitkeep files/.gitkeep projects/.gitkeep .gitignore .dockerignore; do
  [ -f "x-cell/$f" ] || fail "x-cell/$f が無い"
done
for h in '## Sources' '## Entities' '## Topics'; do grep -qx "$h" x-cell/wiki/index.md || fail "wiki/index.md に $h の節が無い"; done
# 目録の節の順は Overview、Entities、Topics、Sources（ADR-0061 決めたこと 13。担当は Sources の手前まで読む）
order=$(grep '^## ' x-cell/wiki/index.md | tr '\n' '|')
[[ "$order" == "## Overview|## Entities|## Topics|## Sources|" ]] && pass "wiki/index.md の節は Overview、Entities、Topics、Sources の順（ADR-0061）" || fail "wiki/index.md の節の順: $order"
[ "$(grep -c '^## ' x-cell/wiki/log.md)" = 0 ] && grep -q '^追記だけの時系列。.*`## \[YYYY-MM-DD\] ingest | <タイトル>`' x-cell/wiki/log.md \
  && pass "wiki の雛形: index.md に種類ごとの節、log.md は空で先頭の形の説明だけ（ADR-0016）" || fail "wiki/log.md: $(cat x-cell/wiki/log.md)"
grep -qx 'files/' x-cell/.gitignore && fail ".gitignore が files/ を除いている" || pass "files/ は追跡する（.gitignore に無い）"
grep -qx 'artifacts/' x-cell/.gitignore && fail ".gitignore に artifacts/ が残っている" || pass ".gitignore に artifacts/ が無い"
grep -qx 'memory/' x-cell/.gitignore && pass ".gitignore が memory/（auto memory の置き場）を除く（ADR-0020）" || fail ".gitignore に memory/ が無い"
[ -x x-cell/bin/bootstrap ] && [ -x x-cell/local/setup.sh ] && pass "bin/bootstrap と local/setup.sh は実行できる" || fail "実行の権限"
[ "$(grep -c . x-cell/Dockerfile)" = 3 ] && head -n 1 x-cell/Dockerfile | grep -qx 'FROM example.com/andon:test' && pass "Dockerfile は FROM ＋ 2行" || fail "Dockerfile: $(cat x-cell/Dockerfile)"
grep -qx 'slug = "x"' x-cell/andon.toml && grep -qx 'owner = "dev@example.com"' x-cell/andon.toml && pass "andon.toml に slug と owner" || fail "andon.toml: $(cat x-cell/andon.toml)"
find x-cell -name '*.tmpl' | grep -q . && fail "*.tmpl が残っている" || pass "*.tmpl は埋めて外す"
[ -e x-cell/compose.yaml ] || [ -e x-cell/.andon ] && fail "セルの repo にパッケージのファイルがある" || pass "セルの repo に compose.yaml が無い"

"$andon" cell init x >/dev/null 2>"$tmp/err"; r=$?
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
! grep -q ' exec ' "$tmp/out" && pass "引数なしは起動だけ（herdr に入らない）" || fail "引数なしで入った: $(cat "$tmp/out")"
grep -q '^\[andon-x\] 開く: Herdr App で machine andon-x を選ぶ' "$tmp/out" && grep -q '^\[andon-x\] TUI で入るなら: bin/bootstrap herdr$' "$tmp/out" &&
  pass "引数なしは App での開き方と TUI の打ち方を出す" || fail "案内: $(cat "$tmp/out")"
for sub in herdr enter; do
  sandbox bash bin/bootstrap --dry-run $sub >"$tmp/out2" 2>"$tmp/err"; r=$?
  [ "$r" = 0 ] && grep -q '^+ docker compose --project-name andon-x .* up -d$' "$tmp/out2" && grep -q '^+ docker compose .* exec .* dev herdr$' "$tmp/out2" &&
    pass "$sub は起動して herdr に入る" || fail "$sub: rc=$r $(cat "$tmp/out2" "$tmp/err")"
done
# --help は先頭の注記の塊を出す（行番号で切らない）。注記を1行足した写しでも、最後の注記の行まで出て、コードの行は出さない
last_note='番号とタイムゾーンは .env に書く（SSH_PORT・PORTLESS_PORT・FILES_PORT・CELL_TZ）。セルに固有のサービスは compose.override.yaml に。'
help_ok() { [ "$(head -n 1 "$1")" = 'セルの入口。ホストに要るのは docker と bash だけ。' ] && [ "$(tail -n 1 "$1")" = "$last_note" ] && ! grep -q 'set -euo' "$1"; }
# help は本体（偽のイメージの opt/andon/host/bootstrap）が自分の先頭の注記を出す（ADR-0048 決めたこと 3）
body=$img/opt/andon/host/bootstrap
grep -qxF "# $last_note" "$body" && pass "本体の注記の最後の行は確かめる文と同じ" || fail "注記の最後の行が変わった: $(sed -n '2,30p' "$body")"
sandbox bash bin/bootstrap --help >"$tmp/out2" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && help_ok "$tmp/out2" && pass "--help は先頭の注記の塊を出す" || fail "--help: rc=$r $(cat "$tmp/out2" "$tmp/err")"
sed '3i # 足した注記の行' "$body" >"$tmp/body-more"
sandbox ANDON_ENTRY_BODY="$tmp/body-more" bash bin/bootstrap --help >"$tmp/out2" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && help_ok "$tmp/out2" && grep -qx '足した注記の行' "$tmp/out2" && pass "本体の注記の行が増えても --help は欠けない" || fail "注記を足した --help: rc=$r $(cat "$tmp/out2" "$tmp/err")"
(cd .. && sandbox bash x-cell/bin/bootstrap --help) >"$tmp/out2" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && help_ok "$tmp/out2" && pass "セルの repo の外から打っても --help を出す" || fail "外からの --help: rc=$r $(cat "$tmp/out2" "$tmp/err")"
sandbox bash bin/bootstrap no-such-sub >"$tmp/out2" 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && help_ok "$tmp/err" && pass "知らない sub-command は注記を stderr に出して rc 2" || fail "知らない sub-command: rc=$r $(cat "$tmp/out2" "$tmp/err")"
sandbox bash bin/bootstrap --dry-run up >"$tmp/out2" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q ' up -d$' "$tmp/out2" && ! grep -q ' exec \|開く:' "$tmp/out2" && pass "up は起動だけで案内も出さない" || fail "up: rc=$r $(cat "$tmp/out2" "$tmp/err")"

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
for name in inbox kanban-board context-header files-plus waiting; do
  diff -r "$app/mods/$name" "$andon_root/package/host/$name" >/dev/null || ok=0
done
cmp -s "$app/layouts/andon.json" "$andon_root/package/host/layouts/andon.json" && cmp -s "$app/layouts/andon.svg" "$andon_root/package/host/layouts/andon.svg" || ok=0
[ "$ok" = 1 ] && pass "host-install は inbox・kanban-board・context-header・files-plus・waiting とレイアウト andon を App の置き場に置く" || fail "画面の mod かレイアウトが違うか無い: $(ls -R "$app" 2>&1 | head -20)"
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
echo '# オーナーが書いた' >"$home/.config/herdr/config.toml"
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '# オーナーが書いた' "$home/.config/herdr/config.toml" && pass "host-install は今あるファイルを上書きしない" || fail "上書きした"
cmp -s .andon/cloud-init.yaml "$andon_root/package/host/cloud-init.yaml" && pass "host-install は cloud-init の雛形を .andon/ に置く" || fail "cloud-init.yaml が無い"
echo '# 埋めた' >.andon/cloud-init.yaml
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>&1
grep -qx '# 埋めた' .andon/cloud-init.yaml && pass "host-install は編集済みの cloud-init.yaml を残す" || fail "cloud-init.yaml を上書きした"
# Stream Deck の plugin（ADR-0045 の 12）。置き場は推測せず、STREAMDECK_PLUGINS にだけ置く
deck="$tmp/stream deck/Plugins"
lamp=$deck/andon.lamp.sdPlugin
sandbox HOME="$tmp/deck-home" STREAMDECK_PLUGINS="$deck" bash bin/bootstrap --dry-run host-install >"$tmp/out" 2>&1; r=$?
[ "$r" = 0 ] && [ ! -e "$deck" ] && grep -qxF "+ cp -R $img/opt/andon/host/streamdeck/andon.lamp.sdPlugin $lamp" "$tmp/out" \
  && pass "Stream Deck の dry-run は置き場を示すだけ" || fail "Stream Deck の dry-run: rc=$r $(cat "$tmp/out")"
sandbox HOME="$tmp/deck-home" STREAMDECK_PLUGINS="$deck/" bash bin/bootstrap host-install >"$tmp/out" 2>&1; r=$?
[ "$r" = 0 ] && diff -r "$lamp" "$andon_root/package/host/streamdeck/andon.lamp.sdPlugin" >/dev/null && [ "$(ls -A "$deck")" = "andon.lamp.sdPlugin" ] && grep -qxF "+ $lamp" "$tmp/out" \
  && pass "STREAMDECK_PLUGINS に andon.lamp.sdPlugin を置く（空白を含むパス・途中の残りなし）" || fail "Stream Deck の plugin: rc=$r $(cat "$tmp/out")"
echo '// personal' >"$lamp/plugin.js"
rm "$lamp/manifest.json"
sandbox HOME="$tmp/deck-home" STREAMDECK_PLUGINS="$deck" bash bin/bootstrap host-install >"$tmp/out" 2>&1; r=$?
[ "$r" = 0 ] && grep -qxF "! $lamp はすでにある（触らない）" "$tmp/out" && grep -qx '// personal' "$lamp/plugin.js" && [ ! -e "$lamp/manifest.json" ] \
  && pass "2回目は今ある plugin に触らない（編集済み・不完全でも）" || fail "plugin に触った: rc=$r $(cat "$tmp/out")"
rm -r "$lamp"
ln -s "$tmp/missing-plugin" "$lamp"
sandbox HOME="$tmp/deck-home" STREAMDECK_PLUGINS="$deck" bash bin/bootstrap host-install >"$tmp/out" 2>&1; r=$?
[ "$r" = 0 ] && [ -L "$lamp" ] && [ ! -e "$tmp/missing-plugin" ] && grep -qF "はすでにある（触らない）" "$tmp/out" \
  && pass "リンク切れの plugin も触らない" || fail "plugin のリンクを変更した: rc=$r"
n=0
for unset_as in STREAMDECK_UNUSED= STREAMDECK_PLUGINS=; do
  n=$((n + 1))
  h="$tmp/nodeck-$n"
  sandbox HOME="$h" "$unset_as" bash bin/bootstrap host-install >"$tmp/out" 2>&1; r=$?
  [ "$r" = 0 ] && [ "$(grep -c STREAMDECK_PLUGINS "$tmp/out")" = 1 ] && [ -f "$h/.config/herdr-app/mods/inbox/main.js" ] && [ -x "$h/.local/bin/andon-up" ] && [ -f "$h/.config/herdr-app/layouts/andon.json" ] \
    && ! grep -q 'andon.lamp.sdPlugin' "$tmp/out" \
    && pass "STREAMDECK_PLUGINS が無い（$unset_as）: 置かずに案内1行、ほかは従来どおり" || fail "未指定の host-install（$unset_as）: rc=$r $(cat "$tmp/out")"
done
# 相対パス（wslpath に通していない Windows のパスも）は、セルの repo の中に置いてしまう。書く前に止める
for relative in 'C:\Users\sample\Plugins' Plugins; do
  sandbox HOME="$tmp/relative-home" STREAMDECK_PLUGINS="$relative" bash bin/bootstrap host-install >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && [ ! -e "$relative" ] && [ ! -e "$tmp/relative-home" ] && grep -q '絶対パス' "$tmp/err" \
    && pass "STREAMDECK_PLUGINS が絶対パスでなければ書く前に止まる（$relative）" || fail "相対の置き場（$relative）: rc=$r"
done
# 新しい bootstrap と、plugin の無い前のイメージ
mkdir -p "$tmp/old-image/opt"
cp -R "$img/opt/andon" "$tmp/old-image/opt/andon"
rm -rf "$tmp/old-image/opt/andon/host/streamdeck"
sandbox HOME="$tmp/old-home" ANDON_IMAGE_ROOT="$tmp/old-image" STREAMDECK_PLUGINS="$tmp/old-deck" bash bin/bootstrap host-install >"$tmp/out" 2>&1; r=$?
[ "$r" = 0 ] && [ ! -e "$tmp/old-deck" ] && grep -q 'イメージに Stream Deck の plugin が無い' "$tmp/out" && [ -f "$tmp/old-home/.config/herdr-app/mods/inbox/main.js" ] \
  && pass "plugin の無いイメージでは置かずに知らせ、ほかは置く" || fail "前のイメージ: rc=$r $(cat "$tmp/out")"
# 前に作ったセルは bin/bootstrap を写して置き換える: 雛形はセルごとの値を持たない
cmp -s bin/bootstrap "$andon_root/init/bin/bootstrap" \
  && pass "bin/bootstrap は雛形と同じ（そのまま置き換えられる）" || fail "bin/bootstrap にセルごとの値が入った"
echo 'SSH_PORT=22/;s/x/y' >.env
sandbox bash bin/bootstrap host-install >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q '数字だけ' "$tmp/err" && pass "host-install は数字でない番号で止まる" || fail "番号の検査: rc=$r $(cat "$tmp/out" "$tmp/err")"
rm .env

# セルの repo のオーナーが uid 1000 でなければ、起動する前に止まって回り道を出す（down・status・host-install は止めない）
for sub in "" up herdr sh rebuild; do
  sandbox FAKE_OWNER=1001 bash bin/bootstrap --dry-run $sub >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && grep -q 'uid 1001' "$tmp/err" && grep -q '回り道' "$tmp/err" && ! grep -q 'docker' "$tmp/out" \
    && pass "repo のオーナーが 1001 なら '${sub:-enter}' は起動する前に止まる" || fail "repo のオーナー 1001 で ${sub:-enter}: rc=$r $(cat "$tmp/out" "$tmp/err")"
done
sandbox FAKE_OWNER=1001 bash bin/bootstrap --dry-run down >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && pass "repo のオーナーが 1001 でも down は通る" || fail "repo のオーナー 1001 で down: rc=$r $(cat "$tmp/err")"

# rebuild --from: andon の checkout から image/build.sh で base image を焼き、FROM をそれにして build・up する（偽の docker）
co=$tmp/co
mkdir -p "$co/image" "$tmp/dpath"
cp "$andon_root/image/build.sh" "$andon_root/image/Dockerfile" "$co/image/"
git -C "$co" init -q && git -C "$co" add . && git -C "$co" -c user.name=t -c user.email=t@example.com commit -qm init
sha=$(git -C "$co" rev-parse HEAD)
cp -P "$tmp/path/"* "$tmp/dpath/"
for t in git env seq; do ln -s "$(command -v "$t")" "$tmp/dpath/$t"; done
# 偽の docker: 1回の呼び出しを1行に、argv を1つずつ [ ] で囲んで記録する（空白を含む引数の境界も比べる）。git は FAKE_SHA を答え、image inspect は FAKE_REVISION を答える
cat >"$tmp/dpath/docker" <<'SH'
#!/bin/bash
{ printf '[%s]' "$@"; echo; } >>"$FAKE_DOCKER_LOG"
case "$*" in
  *"rev-parse HEAD") echo "$FAKE_SHA" ;;
  *"compose "*" logs dev") echo '[andon boot] ready' ;;
  "history "*) # 履歴（新しい段が上）: andon-x:local は FAKE_IMAGE_HISTORY、ほか（FROM）は FAKE_FROM_HISTORY。空なら手元に無い
    case "${*: -1}" in andon-x:local) h=${FAKE_IMAGE_HISTORY:-} ;; *) h=${FAKE_FROM_HISTORY:-} ;; esac
    [ -n "$h" ] && printf '%s\n' "$h" || exit 1 ;;
  # セルの image（andon-x:local）は、FAKE_HAS_IMAGE か FAKE_REVISION があるか、build で焼いた印（$FAKE_DOCKER_LOG.image）があれば在る
  "image inspect andon-x:local") [ -n "${FAKE_HAS_IMAGE:-}${FAKE_REVISION:-}" ] || [ -e "$FAKE_DOCKER_LOG.image" ] || exit 1 ;;
  "image inspect"*) [ -n "${FAKE_REVISION:-}" ] && echo "$FAKE_REVISION" || exit 1 ;;
  # セルの image を焼く（FAKE_BUILD_FAIL なら落ちる）。焼いたら印を置く
  "build "*"-t andon-x:local "*) [ -z "${FAKE_BUILD_FAIL:-}" ] || exit 1; : >"$FAKE_DOCKER_LOG.image" ;;
  # 取り出し: image の中身は FAKE_IMAGE の根（無いパスは落ちる。本物の docker cp と同じに、今あるファイルは上書きする）
  "create "*) echo fakecid ;;
  "cp fakecid:"*) src=${FAKE_IMAGE:-/nonexistent}${2#fakecid:}; [ -e "$src" ] || exit 1; cp -R "$src" "$3" ;;
esac
exit 0
SH
chmod 755 "$tmp/dpath/docker"
log=$tmp/docker.log
argv() { printf '[%s]' "$@"; }
dsandbox() { : >"$log"; rm -f "$log.image"; env -i PATH="$tmp/dpath" HOME="$home" GIT_CEILING_DIRECTORIES="$GIT_CEILING_DIRECTORIES" ANDON_IMAGE_ROOT="$img" FAKE_DOCKER_LOG="$log" FAKE_SHA="$sha" "$@"; }
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
# 作り直しは入口から入り直した本体の _recreate が打つ（calls[3] はその docker info。ADR-0048 決めたこと 8）
[ "${calls[4]:-}" = "$(argv compose --project-name andon-x --project-directory "$PWD" -f .andon/compose.yaml up -d --force-recreate)" ] \
  && pass "そのあと compose で作り直す" || fail "up: $(cat "$log")"
grep -q "commit: $sha" "$tmp/out" && grep -q 'GHCR のバージョンへ戻すなら' "$tmp/out" \
  && pass "元の commit と、GHCR のバージョンへ戻す道を出す" || fail "出力: $(cat "$tmp/out")"

dsandbox FAKE_REVISION="$sha" bash bin/bootstrap status >"$tmp/out" 2>&1
grep -qxF "FROM: $tag（commit: $sha）" "$tmp/out" && pass "status は FROM と元の commit を出す" || fail "status: $(cat "$tmp/out")"
dsandbox bash bin/bootstrap status >"$tmp/out" 2>&1
grep -qxF "FROM: $tag（commit: 手元に無い）" "$tmp/out" && pass "status は image が手元に無いと言う" || fail "status（無い）: $(cat "$tmp/out")"

# up: 手元の image が FROM の上に焼かれていないと分かれば、起動せずに rebuild を打てと止まる（#96）
base=$'2026-11-02T00:00:00Z ENV HOME=/opt/andon-home\n2026-11-01T00:00:00Z COPY . /opt/andon'
cell=$'2026-11-03T00:00:00Z RUN local/setup.sh\n'"$base"
up_ok="$(argv compose --project-name andon-x --project-directory "$PWD" -f .andon/compose.yaml up -d)"
dsandbox FAKE_REVISION=x FAKE_FROM_HISTORY="$base" FAKE_IMAGE_HISTORY=$'2026-10-03T00:00:00Z RUN local/setup.sh\n2026-10-01T00:00:00Z COPY . /opt/andon' bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q "FROM（$tag）から焼いたものでない" "$tmp/err" && grep -q 'bin/bootstrap rebuild' "$tmp/err" && ! grep -qE '^\[build\]|\[(up|rm|rmi)\]' "$log" \
  && pass "up は前の FROM で焼いた古い image なら起動せず rebuild を打てと止まる（docker を消さない・焼かない）" || fail "up（古い）: rc=$r $(cat "$tmp/err" "$log")"
# 層を作らない段（ENV）だけが違う FROM でも止まる（A-0177 のレビュー）
dsandbox FAKE_REVISION=x FAKE_FROM_HISTORY="$base" FAKE_IMAGE_HISTORY=$'2026-11-03T00:00:00Z RUN local/setup.sh\n2026-10-20T00:00:00Z ENV HOME=/home/andon\n2026-11-01T00:00:00Z COPY . /opt/andon' bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'bin/bootstrap rebuild' "$tmp/err" && ! grep -qF "$up_ok" "$log" \
  && pass "up は FROM と ENV の段だけ違う image でも止まる" || fail "up（ENV 違い）: rc=$r $(cat "$tmp/err" "$log")"
dsandbox FAKE_REVISION=x FAKE_FROM_HISTORY="$base" FAKE_IMAGE_HISTORY="$cell" bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -qF "$up_ok" "$log" && ! grep -q '^\[build\]' "$log" \
  && pass "up は FROM の上に焼いた image ならそのまま起動する" || fail "up（合う）: rc=$r $(cat "$tmp/err" "$log")"
dsandbox FAKE_REVISION=x FAKE_IMAGE_HISTORY="$cell" bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -qF "$up_ok" "$log" \
  && pass "up は FROM の image が手元に無ければ（照らせないので）そのまま起動する" || fail "up（FROM 無し）: rc=$r $(cat "$tmp/err" "$log")"

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

# 薄い入口と本体（ADR-0048）。偽の image の根を渡さず、偽の docker の create・cp が FAKE_IMAGE の根から取り出す
xsandbox() { : >"$log"; rm -f "$log.image"; env -i PATH="$tmp/dpath" HOME="$home" GIT_CEILING_DIRECTORIES="$GIT_CEILING_DIRECTORIES" ANDON_WSL_PROBE="$tmp/no-wsl" FAKE_DOCKER_LOG="$log" FAKE_SHA="$sha" FAKE_IMAGE="$img" "$@"; }
# at <記録の行の頭>: 偽の docker の記録で、その頭の行が初めて出る行番号（無ければ空）
at() { grep -nF -- "$1" "$log" | sed -n 1p | cut -d: -f1; }
take_body=$(argv cp fakecid:/opt/andon/host/bootstrap)
leftover() { [ -d .andon ] && find .andon -name 'bootstrap.*' -o -name 'compose.yaml.?*' | grep -q .; }
# 受け入れ条件 1: 薄い入口は host-install・rebuild・mod の名前・install_streamdeck を持たない（本体には当たる）
entry_words='host-install|rebuild|install_streamdeck|inbox|kanban-board|context-header|files-plus|waiting'
grep -qE "$entry_words" "$body" && ! grep -qE "$entry_words" bin/bootstrap \
  && pass "薄い入口は host-install・rebuild・mod の名前・install_streamdeck を持たない（同じ語で本体には当たる）" || fail "薄い入口に本体の語がある: $(grep -nE "$entry_words" bin/bootstrap)"
cmp -s "$body" "$andon_root/package/host/bootstrap" && grep -q '^host_install()' "$body" && grep -q '^install_streamdeck()' "$body" \
  && pass "偽の image の根の opt/andon/host/bootstrap が本体（host-install と Stream Deck の置き先を持つ）" || fail "偽の image の本体"
# 受け入れ条件 2: 打つたびに create・cp（.andon/ の一時の名前へ）・rm の順で本体を取り出し、.andon/bootstrap に置く
for n in 1 2; do
  xsandbox FAKE_HAS_IMAGE=1 bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
  c=$(at "$(argv create andon-x:local)") p=$(at "$take_body") d=$(at "$(argv rm fakecid)") u=$(at "[compose]")
  [ "$r" = 0 ] && [ -n "$c" ] && [ -n "$p" ] && [ -n "$d" ] && [ -n "$u" ] && [ "$c" -lt "$p" ] && [ "$p" -lt "$d" ] && [ "$d" -lt "$u" ] \
    && grep -qE '^\[cp\]\[fakecid:/opt/andon/host/bootstrap\]\[\.andon/bootstrap\.[A-Za-z0-9]{6}\]$' "$log" && [ "$(grep -cF "$take_body" "$log")" = 1 ] \
    && cmp -s .andon/bootstrap "$body" && ! leftover && ! grep -q '^\[build\]' "$log" \
    && pass "up（$n 回目）は create・cp（.andon/ の一時の名前へ）・rm の順で本体を取り出し、.andon/bootstrap に置いてから走らせる（一時のファイルは残らない）" \
    || fail "本体の取り出し（$n 回目）: rc=$r $(cat "$tmp/err" "$log"; ls -A .andon)"
done
# 受け入れ条件 3: 書き換えた .andon/bootstrap は走らず、image の本体に戻る。--dry-run でも走らない
hack() { printf '#!/bin/bash\n: >"%s"\n' "$tmp/hacked" >.andon/bootstrap; }
hack
xsandbox FAKE_HAS_IMAGE=1 bash bin/bootstrap status >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ ! -e "$tmp/hacked" ] && cmp -s .andon/bootstrap "$body" && grep -q '^\[compose\].*\[ps\]$' "$log" \
  && pass "書き換えた .andon/bootstrap は走らせず、image の本体を取り出し直して戻す" || fail "書き換えた本体: rc=$r $(cat "$tmp/err" "$log")"
hack
state >"$tmp/s0"
xsandbox bash bin/bootstrap --dry-run up >"$tmp/out" 2>"$tmp/err"; r=$?
state >"$tmp/s1"
want3=$(printf '+ docker build -t andon-x:local %s\n+ docker cp \\<docker\\ create\\ andon-x:local\\>:/opt/andon/host/bootstrap .andon/bootstrap\n+ bash .andon/bootstrap --dry-run up' "$PWD")
[ "$r" = 0 ] && [ "$(cat "$tmp/out")" = "$want3" ] && [ ! -s "$log" ] && [ ! -e "$tmp/hacked" ] && cmp -s "$tmp/s0" "$tmp/s1" && grep -q ANDON_ENTRY_BODY "$tmp/err" \
  && pass "--dry-run は docker を打たず、何も書かず、前に取り出した本体も走らせず、焼く・取り出す・走らせるの 3 行を出す" \
  || fail "--dry-run（根も名指しも無い）: rc=$r $(cat "$tmp/out" "$tmp/err" "$log"; diff "$tmp/s0" "$tmp/s1")"
rm -rf .andon
# 受け入れ条件 4: 本体は ANDON_ENTRY=1 で起動される（取り出した本体でも、名指しの本体でも）
mkdir -p "$tmp/probe-image/opt/andon/host"
printf '#!/bin/bash\necho "ANDON_ENTRY=$ANDON_ENTRY ARGS=$* PWD=$PWD"\n' >"$tmp/probe-image/opt/andon/host/bootstrap"
xsandbox FAKE_HAS_IMAGE=1 FAKE_IMAGE="$tmp/probe-image" bash bin/bootstrap status 'a b' >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ "$(cat "$tmp/out")" = "ANDON_ENTRY=1 ARGS=status a b PWD=$PWD" ] \
  && pass "取り出した本体は ANDON_ENTRY=1・引数そのまま・作業フォルダはセルの repo の根で起動される" || fail "本体の起動: rc=$r $(cat "$tmp/out" "$tmp/err")"
(cd bin && xsandbox ANDON_ENTRY_BODY=../../probe-image/opt/andon/host/bootstrap bash ./bootstrap up) >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && [ "$(cat "$tmp/out")" = "ANDON_ENTRY=1 ARGS=up PWD=$PWD" ] && [ ! -s "$log" ] \
  && pass "名指しの本体（打った場所からの相対パス）も ANDON_ENTRY=1 で、セルの repo の根で起動される" || fail "名指しの本体の起動: rc=$r $(cat "$tmp/out" "$tmp/err")"
sandbox bash "$body" up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q '薄い入口）から打つ' "$tmp/err" && [ ! -s "$tmp/out" ] \
  && pass "ANDON_ENTRY を渡さずに本体を直に打つと、入口から打つよう言って止まる" || fail "本体を直に打った: rc=$r $(cat "$tmp/out" "$tmp/err")"
sed 's/^need_entry=1$/need_entry=2/' "$body" >"$tmp/body-need2"
paste_line='docker run --rm andon-x:local andon cell init --print bin/bootstrap > bin/bootstrap.new && install -m 755 bin/bootstrap.new bin/bootstrap && rm bin/bootstrap.new'
sandbox ANDON_ENTRY_BODY="$tmp/body-need2" bash bin/bootstrap --dry-run up >"$tmp/out" 2>"$tmp/err"; r=$?
grep -qx 'need_entry=2' "$tmp/body-need2" && [ "$r" = 1 ] && grep -qxF "$paste_line" "$tmp/err" && grep -q '入口 bin/bootstrap が古い' "$tmp/err" && [ ! -s "$tmp/out" ] \
  && pass "要る番号を 2 にした本体は、番号 1 の入口から打つと止まり、入口を置き換える 1 行を出す" || fail "番号 2 の本体: rc=$r $(cat "$tmp/out" "$tmp/err")"
mkdir -p "$tmp/outside"
cp "$body" "$tmp/outside/bootstrap"
sandbox ANDON_ENTRY=1 bash "$tmp/outside/bootstrap" --dry-run up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q '^+ docker compose --project-name andon-x .* up -d$' "$tmp/out" \
  && pass "セルの repo の外に置いた本体は、作業フォルダのセルの andon.toml を読む" || fail "外の本体: rc=$r $(cat "$tmp/out" "$tmp/err")"
# 受け入れ条件 5: image が無ければ 1 回焼いてから取り出す。本体の無い image は 1 回だけ焼き直し、それでも無ければ案内して止まる
xsandbox bash bin/bootstrap status >"$tmp/out" 2>"$tmp/err"; r=$?
b=$(at "$(argv build -t andon-x:local "$PWD")") c=$(at "$(argv create andon-x:local)")
[ "$r" = 0 ] && [ "$(grep -c '^\[build\]' "$log")" = 1 ] && [ -n "$b" ] && [ -n "$c" ] && [ "$b" -lt "$c" ] && cmp -s .andon/bootstrap "$body" && grep -q 'image が無い' "$tmp/err" \
  && pass "image が無ければ、何のために焼くかを言って 1 回焼いてから本体を取り出す" || fail "image 無し: rc=$r $(cat "$tmp/err" "$log")"
mkdir -p "$tmp/nobody-image/opt"
cp -R "$img/opt/andon" "$tmp/nobody-image/opt/andon"
rm "$tmp/nobody-image/opt/andon/host/bootstrap"
for case_ in 'FAKE_HAS_IMAGE=1:image はあるが本体が無い' 'FAKE_UNUSED=1:image も本体も無い' 'FAKE_BUILD_FAIL=1:image が無く焼けない'; do
  rm -rf .andon
  xsandbox FAKE_IMAGE="$tmp/nobody-image" "${case_%%:*}" bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
  [ "$r" = 1 ] && [ "$(grep -c '^\[build\]' "$log")" = 1 ] && grep -q 'FROM を' "$tmp/err" && grep -q 'ANDON_ENTRY_BODY=' "$tmp/err" && ! grep -q '^\[compose\]' "$log" && ! leftover \
    && pass "${case_#*:}: build は 1 回だけで、FROM を上げる手と ANDON_ENTRY_BODY を言って止まる" || fail "${case_#*:}: rc=$r $(cat "$tmp/err" "$log")"
done
# .andon/ に書けないときは、本体が無いと取り違えて焼かず、書けないことを言って止まる
rm -rf .andon
: >.andon
xsandbox FAKE_HAS_IMAGE=1 bash bin/bootstrap status >"$tmp/out" 2>"$tmp/err"; r=$?
rm -f .andon
[ "$r" = 1 ] && grep -q '\.andon/ に書けない' "$tmp/err" && ! grep -qE '^\[(build|create)\]' "$log" \
  && pass ".andon/ に書けないときは、焼かず、取り出しもせず、書けないことを言って止まる" || fail ".andon/ に書けない: rc=$r $(cat "$tmp/err" "$log")"
# 名指しの本体に薄い入口を渡すと、入口が自分を走らせ続けずに止まる
timeout 10 env -i PATH="$tmp/path" HOME="$home" ANDON_ENTRY_BODY=bin/bootstrap bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q '薄い入口で、本体でない' "$tmp/err" && [ "$(grep -c '名指しの .* で動く' "$tmp/err")" = 0 ] \
  && pass "名指しの本体に薄い入口を渡すと、本体でないと言って 1 回で止まる" || fail "薄い入口の名指し: rc=$r $(sed -n 1,3p "$tmp/err")"
# 受け入れ条件 6: 名指しの本体は、本体のための docker の create・cp・build を打たずに走り、名指しで動くことを 1 行出す。
# image が無くても焼かない（本体の status が compose.yaml を取り出す create・cp は打つ）。無いファイルは止まる
xsandbox ANDON_ENTRY_BODY="$body" bash bin/bootstrap status >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && ! grep -qF "$take_body" "$log" && ! grep -q '^\[build\]' "$log" \
  && [ "$(grep -c "名指しの $body で動く" "$tmp/err")" = 1 ] && grep -q '^\[compose\].*\[ps\]$' "$log" && [ ! -e .andon/bootstrap ] \
  && pass "名指しの本体は本体のための create・cp・build を打たずに走り、名指しで動くことを標準エラーに 1 行出す" || fail "名指しの本体: rc=$r $(cat "$tmp/err" "$log")"
xsandbox ANDON_ENTRY_BODY="$tmp/no-such-body" bash bin/bootstrap up >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q "$tmp/no-such-body が無い" "$tmp/err" && [ ! -s "$log" ] \
  && pass "名指しの本体が無ければ、理由を言って止まる" || fail "無い名指し: rc=$r $(cat "$tmp/err" "$log")"
# 受け入れ条件 6・8: image が無くても、名指しの本体で rebuild --from が base の image を焼くところまで進み、作り直しは焼いた image の本体で行う
rm -rf .andon
xsandbox ANDON_ENTRY_BODY="$body" bash bin/bootstrap rebuild --from "$co" >"$tmp/out" 2>"$tmp/err"; r=$?
base_at=$(at "$(argv build --file "$co/image/Dockerfile")") cell_at=$(at "$(argv build -t andon-x:local "$PWD")") p=$(at "$take_body") u=$(at "$(argv compose --project-name andon-x --project-directory "$PWD" -f .andon/compose.yaml up -d --force-recreate)")
first=$(grep -nE '^\[(create|cp|build)\]' "$log" | sed -n 1p | cut -d: -f1)
[ "$r" = 0 ] && [ -n "$base_at" ] && [ "$first" = "$base_at" ] && [ -n "$cell_at" ] && [ -n "$p" ] && [ -n "$u" ] && [ "$base_at" -lt "$cell_at" ] && [ "$cell_at" -lt "$p" ] && [ "$p" -lt "$u" ] \
  && [ "$(grep -c '名指しの' "$tmp/err")" = 1 ] && cmp -s .andon/bootstrap "$body" && [ "$(head -n 1 Dockerfile)" = "FROM $tag" ] \
  && pass "image が無くても名指しの本体で rebuild --from が base を焼き、作り直しは焼いた image から取り出した本体で行う" || fail "名指しの rebuild --from: rc=$r $(cat "$tmp/err" "$log")"
cp "$tmp/Dockerfile.orig" Dockerfile
# 受け入れ条件 8: rebuild は焼いたあと入口から入り直し、もう一度取り出した本体で compose … up -d --force-recreate を打つ
for args in rebuild "rebuild --from $co"; do
  rm -rf .andon
  # shellcheck disable=SC2086 # args は空白で割る（checkout の path に空白は無い）
  xsandbox FAKE_HAS_IMAGE=1 bash bin/bootstrap $args >"$tmp/out" 2>"$tmp/err"; r=$?
  b=$(grep -n '^\[build\].*\[-t\]\[andon-x:local\]' "$log" | sed -n 1p | cut -d: -f1)
  ps_=$(grep -nF "$take_body" "$log" | sed -n 2p | cut -d: -f1)
  u=$(at "$(argv compose --project-name andon-x --project-directory "$PWD" -f .andon/compose.yaml up -d --force-recreate)")
  [ "$r" = 0 ] && [ "$(grep -cF "$take_body" "$log")" = 2 ] && [ -n "$b" ] && [ -n "$ps_" ] && [ -n "$u" ] && [ "$b" -lt "$ps_" ] && [ "$ps_" -lt "$u" ] && ! leftover \
    && pass "$args は焼いたあと本体をもう一度取り出し、そのあとで up -d --force-recreate を打つ" || fail "$args の順: rc=$r $(cat "$tmp/err" "$log")"
  cp "$tmp/Dockerfile.orig" Dockerfile
done
sandbox bash bin/bootstrap --help >"$tmp/out2" 2>&1
! grep -q _recreate "$tmp/out2" && grep -q '_recreate)' "$body" && pass "_recreate は help に出ない" || fail "_recreate が help に出る"
# 受け入れ条件 7: image が無い・docker に繋がらないときの --help は、焼かずに短い案内を出して 0 で終わる
rm -rf .andon
xsandbox bash bin/bootstrap --help >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q '本体を読めない: andon-x:local が無い' "$tmp/out" && ! grep -qE '^\[(build|create|cp)\]' "$log" && [ ! -e .andon/bootstrap ] \
  && pass "image が無いときの --help は焼かずに案内を出して rc 0" || fail "image 無しの --help: rc=$r $(cat "$tmp/out" "$tmp/err" "$log")"
xsandbox FAKE_HAS_IMAGE=1 FAKE_IMAGE="$tmp/nobody-image" bash bin/bootstrap --help >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q '本体を読めない: andon-x:local に本体が無い' "$tmp/out" && ! grep -q '^\[build\]' "$log" \
  && pass "本体の無い image の --help は焼かずに案内を出して rc 0" || fail "本体無しの --help: rc=$r $(cat "$tmp/out" "$tmp/err" "$log")"
env -i PATH="$tmp/path" HOME="$home" bash bin/bootstrap --help >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 0 ] && grep -q '本体を読めない: docker に繋がらない' "$tmp/out" \
  && pass "docker の無いホストの --help は案内を出して rc 0" || fail "docker 無しの --help: rc=$r $(cat "$tmp/out" "$tmp/err")"
rm -rf .andon

# 受け入れ条件 12: 薄い入口の中身の控え（ハッシュ）と番号。中身が変わって控えが前のままなら落ちる（ADR-0048 決めたこと 12）
entry_num=1
entry_sum=8fbc0b5cab97952ff5bfc58f1dd4e589fe7787cf78a2ef34ee56750424d71c8c
# check_entry <薄い入口のファイル>: 控えと合えば 0。合わなければ直し方を出して 1
check_entry() {
  local num sum
  num=$(sed -n 's/^entry=\([0-9]*\)$/\1/p' "$1")
  sum=$(sha256sum "$1" | cut -d' ' -f1)
  if [ "$num" != "$entry_num" ] || [ "$sum" != "$entry_sum" ]; then
    echo "薄い入口 init/bin/bootstrap が控えと違う（番号 $num・控え $entry_num、ハッシュ $sum）。動きが変わるなら entry の番号を上げ（package/host/bootstrap の need_entry も）、tests/andon-init.sh の entry_num と entry_sum を直す。動きが変わらないなら entry_sum だけを直す"
    return 1
  fi
}
need=$(sed -n 's/^need_entry=\([0-9]*\)$/\1/p' "$andon_root/package/host/bootstrap")
check_entry "$andon_root/init/bin/bootstrap" >"$tmp/out" && [ "$need" = "$entry_num" ] \
  && pass "薄い入口の中身は控えのハッシュと番号 $entry_num に合い、本体が要る番号も $entry_num" || fail "$(cat "$tmp/out") 本体の need_entry=$need"
sed '2s/$/x/' "$andon_root/init/bin/bootstrap" >"$tmp/entry-broken"
if check_entry "$tmp/entry-broken" >"$tmp/out"; then
  fail "薄い入口を 1 字変えた写しでも控えの検査が通った"
else
  grep -q '番号を上げ' "$tmp/out" && grep -q 'entry_sum だけを直す' "$tmp/out" \
    && pass "薄い入口を 1 字変えた写しは控えの検査で落ち、番号を上げるか控えを直すよう言う" || fail "壊した写しの言い方: $(cat "$tmp/out")"
fi

sed -i 's|^FROM .*|FROM ghcr.io/<owner>/andon:<バージョン>|' Dockerfile
sandbox bash bin/bootstrap --dry-run >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'FROM を埋める' "$tmp/err" && pass "FROM が埋まっていなければ止まる" || fail "placeholder: rc=$r $(cat "$tmp/err")"

if command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  ANDON_IMAGE=andon-x:local ANDON_SLUG=x ANDON_ENTRY=1 docker compose --project-directory "$PWD" -f "$andon_root/package/compose.yaml" config -q 2>"$tmp/err" &&
    pass "compose.yaml は docker compose が読める" || fail "docker compose config: $(cat "$tmp/err")"
else
  echo "skip docker compose config（docker compose が無い）"
fi

# andon-up: <slug>-cell と前の形の andon-<slug> を1回ずつ起動する（偽の docker と HOME。本物の docker に触らない）
up=$tmp/up && mkdir -p "$up/src" "$up/home" "$up/path"
printf '#!/bin/sh\nexit 0\n' >"$up/path/docker" && chmod 755 "$up/path/docker"
for d in a-cell andon-b andon-c-cell d; do
  mkdir -p "$up/src/$d/bin"
  printf '#!/bin/sh\necho %s >>"%s"\n' "$d" "$up/started" >"$up/src/$d/bin/bootstrap"
  chmod 755 "$up/src/$d/bin/bootstrap"
done
env -i PATH="$up/path:/usr/bin:/bin" HOME="$up/home" ANDON_PLANTS="$up/src" sh "$andon_root/package/host/andon-up" >"$tmp/out" 2>&1; r=$?
got=$(sort "$up/started" 2>/dev/null | tr '\n' ' ')
[ "$r" = 0 ] && [ "$got" = "a-cell andon-b andon-c-cell " ] && pass "andon-up は *-cell と andon-* を1回ずつ起動する" || fail "andon-up: rc=$r 起動した: $got $(cat "$tmp/out")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
