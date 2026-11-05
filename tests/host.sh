#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
node --experimental-vm-modules tests/host/mod.mjs
