#!/usr/bin/env bash
# image/build.sh <andon の checkout> <tag> [<revision>]
#
# andon の checkout から base image を焼く。tests/image.sh（CI も）と建屋の `bin/bootstrap rebuild --from` が同じものを打つ。
# ホストに要るのは docker と bash だけ（kanban・andon の go build は image/Dockerfile の段 bin で docker の中）。
# <revision> は焼いた元の commit（image の label org.opencontainers.image.revision）。省けば checkout の git から取る
# （汚れていれば末尾に -dirty。git が無ければ unknown）。kanban --version には先頭12文字（と -dirty）が出る。
#
# CI だけが使う env: ANDON_BUILD_CACHE_FROM・ANDON_BUILD_CACHE_TO（docker buildx の --cache-from・--cache-to の値。
# 片方でもあれば docker buildx build --load で焼く）。どちらも無ければ（建屋・release）今までどおり cache を読み書きしない。
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

build=(docker build)
if [ -n "${ANDON_BUILD_CACHE_FROM:-}${ANDON_BUILD_CACHE_TO:-}" ]; then
  build=(docker buildx build --load)
  [ -z "${ANDON_BUILD_CACHE_FROM:-}" ] || build+=(--cache-from "$ANDON_BUILD_CACHE_FROM")
  [ -z "${ANDON_BUILD_CACHE_TO:-}" ] || build+=(--cache-to "$ANDON_BUILD_CACHE_TO")
fi
"${build[@]}" --file "$checkout/image/Dockerfile" \
  --build-arg "ANDON_REVISION=$rev" --build-arg "ANDON_VERSION=$version" \
  --tag "$tag" "$checkout"
