#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
node --experimental-vm-modules tests/host/mod.mjs
node --experimental-vm-modules tests/host/screen.mjs
