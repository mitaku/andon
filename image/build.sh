#!/usr/bin/env bash
# image/build.sh <andon の checkout> <tag> [<revision>]
#
# andon の checkout から base image を焼く。tests/image.sh（CI も）とセルの `bin/bootstrap rebuild --from` が同じものを打つ。
# ホストに要るのは docker と bash だけ（kanban・andon の go build は image/Dockerfile の段 bin で docker の中）。
# <revision> は焼いた元の commit（image の label org.opencontainers.image.revision）。省けば checkout の git から取る
# （汚れていれば末尾に -dirty。git が無ければ unknown）。kanban --version には先頭12文字（と -dirty）が出る。
set -euo pipefail

[ "$#" -ge 2 ] && [ "$#" -le 3 ] || { sed -n '2,7p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
checkout=$1 tag=$2 rev=${3:-}
[ -f "$checkout/image/Dockerfile" ] || { echo "✗ $checkout/image/Dockerfile が無い（andon の checkout を渡す）" >&2; exit 1; }

if [ -z "$rev" ]; then
  if command -v git >/dev/null && rev=$(git -C "$checkout" rev-parse HEAD 2>/dev/null); then
    [ -z "$(git -C "$checkout" status --porcelain 2>/dev/null)" ] || rev=$rev-dirty
  else
    rev=unknown
  fi
fi
case "$rev" in
  *-dirty) version=${rev:0:12}-dirty ;;
  unknown) version=dev ;;
  *) version=${rev:0:12} ;;
esac

docker build --file "$checkout/image/Dockerfile" \
  --build-arg "ANDON_REVISION=$rev" --build-arg "ANDON_VERSION=$version" \
  --tag "$tag" "$checkout"
