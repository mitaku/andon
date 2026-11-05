#!/usr/bin/env bash
# この建屋だけの追加。イメージのビルドで andon の利用者として1回走る（Dockerfile の RUN）。
# CLI は同じ場所の aqua.yaml に書く（足したら `aqua update-checksum -a`）。ここには aqua で入らないもの
# （apt・公式のインストーラ）だけを足す。使えるもの: sudo（パスワード不要）・aqua で入れた道具。
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
AQUA_GLOBAL_CONFIG="$here/aqua.yaml:${AQUA_GLOBAL_CONFIG:-}" aqua install --all
