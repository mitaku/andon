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
: "${DECK_REPOSITORY:?set DECK_REPOSITORY to the private release repository (<owner>/herdr-flightdeck)}"
# worktree の中で取得し、認証情報は Docker に渡さない。
build_dir=$(mktemp -d .image-build.XXXXXX)
trap 'rm -rf "$build_dir"' EXIT
release=deck-v0.4.0
case "$(docker version --format '{{.Server.Arch}}')" in
  amd64) arch=amd64; checksum=f24b08c040815244d952adf609bcb348b88565fd66cd1ed825161283d14da4d1 ;;
  arm64) arch=arm64; checksum=a94f2b07797886428fb2ff4a485717061c050d337de93bab4325b10cd745009a ;;
  *) echo 'FAIL image: supported architectures are amd64 and arm64' >&2; exit 1 ;;
esac
gh release download "$release" --repo "$DECK_REPOSITORY" \
  --pattern "deck-linux-$arch" --dir "$build_dir"
printf '%s  %s\n' "$checksum" "$build_dir/deck-linux-$arch" | sha256sum -c -
mv "$build_dir/deck-linux-$arch" "$build_dir/deck"
docker build --build-context "deck=$build_dir" --file image/Dockerfile --tag andon:test .
docker run --rm --network none andon:test bash -euc '
  test "$(id -u)" -ne 0
  test -d /opt/andon
  test -d "$PLAYWRIGHT_BROWSERS_PATH"
  deck help >/dev/null
  hc --version
  node --version
  python3 --version
  claude --version
  codex --version
  portless --version
'
echo 'ok   image: build and offline runtime smoke checks'
