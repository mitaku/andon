#!/usr/bin/env bash
# ホストの Herdr に入れる package/host/ の plugin（andon・kanban-board・files-plus ほか）の振る舞い。node が要る（CI では無ければ失敗、手元では skip。tests/kanban.sh と同じ）。
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v node >/dev/null; then
  if [ "${CI:-false}" = true ]; then echo 'FAIL host: CI requires node' >&2; exit 1; fi
  echo 'skip: node が無い'; exit 0
fi
node --experimental-vm-modules tests/host/mod.mjs
node --experimental-vm-modules tests/host/screen.mjs
