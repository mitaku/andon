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
trap 'rm -rf "$tmp"' EXIT
# 台帳の CLI は同じ repo から焼いて build context `kanban` で渡す（image/Dockerfile）
mkdir "$tmp/kanban"
arch=$(docker version --format '{{.Server.Arch}}')
build_kanban() { CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$@" build -trimpath -ldflags "-s -w -X main.version=$(git rev-parse --short=12 HEAD 2>/dev/null || echo dev)" -o "$tmp/kanban/kanban" ./cmd/kanban; }
if command -v go >/dev/null; then build_kanban go
elif command -v mise >/dev/null; then build_kanban mise exec -- go
else echo 'FAIL image: go is required to build kanban' >&2; exit 1; fi
docker build --file image/Dockerfile --build-context "kanban=$tmp/kanban" --tag andon:test .
docker run --rm --network none andon:test bash -euc '
  test "$(id -u)" -ne 0
  kanban --version
  test "$(kanban --help)" = "$(deck --help)"
  test "$(readlink /usr/local/bin/deck)" = kanban
  test -d /opt/andon
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
echo 'ok   image: build, offline runtime and workflow policy checks'
