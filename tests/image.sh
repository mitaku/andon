#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

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
# 台帳の CLI と andon は同じ repo から焼いて build context `kanban`・`andon` で渡す（image/Dockerfile）
mkdir "$tmp/kanban" "$tmp/andon-bin"
arch=$(docker version --format '{{.Server.Arch}}')
build_bin() { # build_bin <go コマンド…>: kanban と andon を linux の静的バイナリに焼く
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$@" build -trimpath -ldflags "-s -w -X main.version=$(git rev-parse --short=12 HEAD 2>/dev/null || echo dev)" -o "$tmp/kanban/kanban" ./cmd/kanban
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$@" build -trimpath -ldflags "-s -w" -o "$tmp/andon-bin/andon" ./cmd/andon
}
if command -v go >/dev/null; then build_bin go
elif command -v mise >/dev/null; then build_bin mise exec -- go
else echo 'FAIL image: go is required to build kanban and andon' >&2; exit 1; fi
docker build --file image/Dockerfile --build-context "kanban=$tmp/kanban" --build-context "andon=$tmp/andon-bin" --tag andon:test .
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
echo 'ok   image: build, offline runtime, workflow policy checks and plant boot (init → bootstrap up → boot → ssh)'
