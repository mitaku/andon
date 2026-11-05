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
docker build --file image/Dockerfile --tag andon:test .
docker run --rm --network none andon:test bash -euc '
  test "$(id -u)" -ne 0
  test -d /opt/andon
  test -d "$PLAYWRIGHT_BROWSERS_PATH"
  hc --version
  node --version
  python3 --version
  claude --version
  codex --version
  portless --version
'
docker run --rm --network none \
  --mount "type=bind,source=$PWD,target=/src,readonly" --workdir /src \
  andon:test bash -euc '
    actionlint
    ghalint run
    pinact run --fix=false --no-api
  '
echo 'ok   image: build, offline runtime and workflow policy checks'
