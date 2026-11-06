#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# image/Dockerfile の段 bin（kanban・andon を焼く golang）の版は mise.toml の go と揃える（docker が無くても見る）
img_go=$(sed -n 's/^FROM golang:\([0-9.]*\)-.* AS bin$/\1/p' image/Dockerfile)
mise_go=$(sed -n 's/^go = "\(.*\)"$/\1/p' mise.toml)
if [ -z "$img_go" ] || [ "$img_go" != "$mise_go" ]; then
  echo "FAIL image: image/Dockerfile の golang（${img_go:-<無い>}）が mise.toml の go（$mise_go）と違う" >&2
  exit 1
fi

if ! command -v docker >/dev/null || ! docker info >/dev/null 2>&1; then
  if [ "${CI:-false}" = true ]; then
    echo 'FAIL image: CI requires Docker' >&2
    exit 1
  fi
  echo 'SKIP image: Docker is unavailable; build and runtime checks require CI'
  exit 0
fi
tmp=$(mktemp -d)
plant=$tmp/andon-imgtest
# 建屋の compose を片付ける。boot がコンテナの利用者で /workspace に書いたものは root で持ち主を戻してから消す
# compose の補間に要る値は bin/bootstrap と同じものを渡す
dc() { ANDON_IMAGE=andon-imgtest:local ANDON_SLUG=imgtest docker compose --project-name andon-imgtest --project-directory "$plant" -f "$plant/.andon/compose.yaml" "$@"; }
cleanup() {
  if [ -f "$plant/.andon/compose.yaml" ]; then
    dc down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  docker volume rm andon-imgtest_gh >/dev/null 2>&1 || true
  docker image rm andon-imgtest:local >/dev/null 2>&1 || true
  docker run --rm --user 0 -v "$tmp:/t" andon:test chown -R "$(id -u):$(id -g)" /t >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT
# 焼くのは image/build.sh（CI と建屋の bin/bootstrap rebuild --from も同じもの）。kanban・andon の go build も docker の中
bash image/build.sh . andon:test
rev=$(docker image inspect andon:test --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')
want=$(git rev-parse HEAD 2>/dev/null || echo unknown)
case "$rev" in
  "$want" | "$want-dirty") ;;
  *) echo "FAIL image: label org.opencontainers.image.revision が checkout の commit でない（$rev / $want）" >&2; exit 1 ;;
esac
docker run --rm --network none andon:test bash -euc '
  test "$(id -un):$(id -u):$(id -g)" = andon:1000:1000
  ! getent passwd ubuntu
  kanban --version
  test "$(kanban --help)" = "$(deck --help)"
  test "$(readlink /usr/local/bin/deck)" = kanban
  test -d /opt/andon
  andon help | grep -q "andon <sub-command>"
  test "$(readlink -f /usr/local/bin/andon)" = /opt/andon/bin/andon
  test -f /opt/andon/environment.md
  test -f /opt/andon/compose.yaml
  test -x /opt/andon/host/andon-up
  printf "{\"commit\": \"%040d\"}\n" 0 > /tmp/report.json
  python3 /opt/andon/lib/andon/schema_check.py validate /opt/andon/schema/report.schema.json /tmp/report.json
  test -d "$PLAYWRIGHT_BROWSERS_PATH"
  hc --version
  node --version
  python3 --version
  claude --version
  codex --version
  portless --version
  rg --version
  fzf --version
  jq --version
  usage --version
  # 対話の bash（bash -ic）に Tab 補完・履歴・プロンプト・fzf が入っていること。HISTFILE は volume（state）の下
  out=$(bash -ic "complete -p git gh hc andon deck just mise; echo HISTFILE=\$HISTFILE; echo PS1=\$PS1; (bind -X; bind -s) | grep -c fzf" 2>/dev/null)
  for c in git gh hc andon deck just mise; do grep -q " $c\$" <<<"$out" || { echo "no completion for $c" >&2; exit 1; }; done
  grep -qx "HISTFILE=/opt/andon-home/.local/state/bash/history" <<<"$out"
  grep -q "PS1=.*__git_ps1" <<<"$out"
  test "$(tail -1 <<<"$out")" -gt 0
  # ログインシェル（ssh が起こす形）でも ~/.profile → ~/.bashrc で同じものが読まれること
  login=$(bash -lic "complete -p hc; echo HISTFILE=\$HISTFILE" 2>/dev/null)
  grep -q " hc\$" <<<"$login"
  grep -qx "HISTFILE=/opt/andon-home/.local/state/bash/history" <<<"$login"
'
docker run --rm --network none \
  --mount "type=bind,source=$PWD/.github/workflows/tests.yml,target=/workspace/.github/workflows/tests.yml,readonly" \
  andon:test bash -euc '
    actionlint .github/workflows/tests.yml
    ghalint run
    pinact run --fix=false --no-api
  '

# 建屋を立てる道（docs/operate.md の 2・3・5）: ホストへ andon を取り出す → andon init → bin/bootstrap up → boot の生成物
docker run --rm --network none andon:test cat /opt/andon/bin/andon >"$tmp/andon"
chmod 755 "$tmp/andon"
"$tmp/andon" init imgtest --dir "$plant" --image andon:test --owner owner@example.com
free_port() { python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])'; }
printf 'SSH_PORT=%s\nPORTLESS_PORT=%s\nFILES_PORT=%s\n' "$(free_port)" "$(free_port)" "$(free_port)" >"$plant/.env"
# 建屋の repo は uid 1000 の持ち物（WSL・cloud-init の既定の利用者。runner は 1000 とは限らないので root で渡す）。
# 公開鍵は docs/operate.md の SSH の節のとおり 700 の .ssh・600 の authorized_keys。.andon/ は bootstrap（runner）が書く
mkdir "$plant/.andon"
docker run --rm --network none --user 0 -v "$plant:/p" andon:test bash -euc '
  mkdir -m 700 /p/.ssh
  ssh-keygen -q -t ed25519 -N "" -C imgtest -f /p/.ssh/id_test
  cp /p/.ssh/id_test.pub /p/.ssh/authorized_keys
  chmod 600 /p/.ssh/authorized_keys
  chown -R 1000:1000 /p
  chmod 777 /p/.andon
'
# andon が uid 1001 だった版の image で作った volume（中身は 1001 の持ち物）。boot が作り直さずに andon へ渡すこと
docker volume create --label com.docker.compose.project=andon-imgtest --label com.docker.compose.volume=gh andon-imgtest_gh >/dev/null
docker run --rm --network none --user 0 -v andon-imgtest_gh:/v andon:test bash -euc 'echo old >/v/hosts.yml; chown -R 1001:1001 /v'
# 建屋の image は FROM andon:test。docker-container の builder（CI の setup-buildx）は手元の image を見ないので、docker の builder で焼く
BUILDX_BUILDER=default "$plant/bin/bootstrap" up
# ログは1度取ってから grep する（dc logs | grep -q だと grep の早抜けで SIGPIPE になり、pipefail で偽になる）
boot_log=$(dc logs dev 2>&1) || true
if ! grep -q '\[andon boot\] ready' <<<"$boot_log" || ! grep -q 'sshd を起動した' <<<"$boot_log" ||
  ! grep -qF '持ち主を直した: /opt/andon-home/.config/gh（uid 1001 → 1000）' <<<"$boot_log"; then
  printf '%s\n' "$boot_log" >&2
  echo 'FAIL image: compose の起動で andon boot が ready まで走らない（sshd・volume の持ち主の行も見る）' >&2
  exit 1
fi
dc exec -T dev bash -euc '
  andon help >/dev/null
  python3 -c "import json,sys; a,b=(json.load(open(p)) for p in sys.argv[1:]); sys.exit(a!=b)" \
    /opt/andon/settings.json /etc/claude-code/managed-settings.json
  for t in claude sonnet fable codex codex-review claude-review chief-of-staff chief-engineer; do
    test -s "$HOME/.config/herdr-core/templates/$t.md"
  done
  test "$(cat "$CLAUDE_CONFIG_DIR/CLAUDE.md")" = "$(printf "@/opt/andon/environment.md\n@/workspace/AGENTS.local.md")"
  test -s /opt/andon/environment.md
  test -s /workspace/AGENTS.local.md
  test "$(readlink "$HOME/.agents/skills/andon-worker")" = /opt/andon/skills/worker
  test "$(stat -c %u:%g "$HOME/.config/gh" "$HOME/.config/gh/hosts.yml" | sort -u)" = 1000:1000
  test "$(cat "$HOME/.config/gh/hosts.yml")" = old
  test -d /workspace/artifacts
  test "$(ssh -i /workspace/.ssh/id_test -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    -o LogLevel=ERROR andon@127.0.0.1 hostname)" = imgtest
'
# (a) ssh のセッション（非対話）に image の環境が渡ること。Herdr App の herdr-core serve もこの形で起きる
ssh_dev() { dc exec -T dev ssh -i /workspace/.ssh/id_test -o BatchMode=yes -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR andon@127.0.0.1 "$@"; }
ssh_tools=$(ssh_dev 'command -v claude hc gh aqua') || {
  printf '%s\n' "$ssh_tools" >&2
  echo 'FAIL image: ssh のセッションに claude・hc・gh・aqua が見えない' >&2
  exit 1
}
if [ "$(sed -n 2p <<<"$ssh_tools")" != /opt/andon-home/.local/bin/hc ]; then
  printf '%s\n' "$ssh_tools" >&2
  echo 'FAIL image: ssh のセッションの hc が ~/.local/bin のものでない' >&2
  exit 1
fi
ssh_env=$(ssh_dev env)
exec_env=$(dc exec -T dev env)
for k in $(sed -n 's/^SetEnv //p' image/sshd_config.d/andon.conf | tr ' ' '\n' | cut -d= -f1); do
  a=$(grep "^$k=" <<<"$ssh_env" || true)
  b=$(grep "^$k=" <<<"$exec_env" || true)
  if [ -z "$b" ] || [ "$a" != "$b" ]; then
    echo "FAIL image: ssh のセッションの $k がコンテナの環境と違う（ssh: ${a:-<無い>} / exec: ${b:-<無い>}）" >&2
    exit 1
  fi
done
# (a2) ssh の対話ログインシェルでも補完が出る（sshd → ~/.profile → ~/.bashrc）。履歴の置き場は volume
ssh_login=$(ssh_dev 'bash -lic "complete -p gh hc; echo HISTFILE=\$HISTFILE" 2>/dev/null') || true
if ! grep -q ' hc$' <<<"$ssh_login" || ! grep -q ' gh$' <<<"$ssh_login" ||
  ! grep -qx 'HISTFILE=/opt/andon-home/.local/state/bash/history' <<<"$ssh_login"; then
  printf '%s\n' "$ssh_login" >&2
  echo 'FAIL image: ssh の対話ログインシェルに補完・HISTFILE が効いていない' >&2
  exit 1
fi
# (b) ~/.local/bin/herdr-core が実ファイル（Herdr App が置いたもの）なら、boot し直しても触らず、hc も core もそれを使う
dc exec -T dev bash -euc '
  seed=$(readlink -f "$HOME/.local/bin/herdr-core")
  test "$seed" != "$HOME/.local/bin/herdr-core"
  test "$(readlink "$HOME/.local/bin/hc")" = herdr-core
  rm "$HOME/.local/bin/herdr-core"
  cp "$seed" "$HOME/.local/bin/herdr-core"
'
app_sum=$(dc exec -T dev sha256sum /opt/andon-home/.local/bin/herdr-core)
dc restart dev >/dev/null
for _ in $(seq 90); do
  boot_log=$(dc logs dev 2>&1) || true
  [ "$(grep -c '\[andon boot\] ready' <<<"$boot_log" || true)" -ge 2 ] && break
  sleep 1
done
if [ "$(grep -c '\[andon boot\] ready' <<<"$boot_log" || true)" -lt 2 ]; then
  printf '%s\n' "$boot_log" >&2
  echo 'FAIL image: restart のあと andon boot が ready まで走らない' >&2
  exit 1
fi
if [ "$(dc exec -T dev sha256sum /opt/andon-home/.local/bin/herdr-core)" != "$app_sum" ] ||
  ! dc exec -T dev bash -euc '
    core="$HOME/.local/bin/herdr-core"
    test -f "$core" && ! test -L "$core"
    test "$(readlink "$HOME/.local/bin/hc")" = herdr-core
    test "$(command -v hc)" = "$HOME/.local/bin/hc"
    st=$(hc server status --json)
    pids=$(jq -r ".pid, .holders[].pid" <<<"$st")
    test -n "$pids"
    for p in $pids; do
      test "$(readlink "/proc/$p/exe")" = "$core" || { echo "pid $p: $(readlink "/proc/$p/exe")" >&2; exit 1; }
    done
  '; then
  echo 'FAIL image: ~/.local/bin/herdr-core（実ファイル）を boot が触った、または hc・core がそれを使っていない' >&2
  exit 1
fi
# ホスト鍵は volume に残り、container を作り直しても sshd が同じ鍵を出すこと（手元の known_hosts が壊れない）
hostkey_fp() { dc exec -T dev bash -euc 'ssh-keyscan -t ed25519 127.0.0.1 2>/dev/null | ssh-keygen -lf - | cut -d" " -f2'; }
fp1=$(hostkey_fp)
vol_fp=$(dc exec -T dev ssh-keygen -lf /opt/andon-home/.ssh/ssh_host_ed25519_key.pub | cut -d' ' -f2)
dc down --remove-orphans >/dev/null
BUILDX_BUILDER=default "$plant/bin/bootstrap" up
fp2=$(hostkey_fp)
if [ -z "$fp1" ] || [ "$fp1" != "$vol_fp" ] || [ "$fp1" != "$fp2" ]; then
  echo "FAIL image: sshd のホスト鍵が volume のものでない、または作り直しで変わった（$fp1 / $vol_fp / $fp2）" >&2
  exit 1
fi
echo 'ok   image: build, offline runtime, workflow policy checks and plant boot (init → bootstrap up → boot → ssh env → herdr-core in ~/.local/bin)'
