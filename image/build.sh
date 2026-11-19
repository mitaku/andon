#!/usr/bin/env bash
# image/build.sh <andon の checkout> <tag> [<revision>]
#
# andon の checkout から base image を焼く。tests/image.sh（CI も）とセルの `bin/bootstrap rebuild --from` が同じものを打つ。
# ホストに要るのは docker と bash だけ（kanban・andon の go build は image/Dockerfile の段 bin で docker の中）。
# <revision> は焼いた元の commit（image の label org.opencontainers.image.revision）。省けば checkout の git から取る
# （汚れていれば末尾に -dirty。git が無ければ unknown）。kanban --version には先頭12文字（と -dirty）が出る。
# 環境変数 ANDON_APT_MIRROR（例 http://azure.archive.ubuntu.com/ubuntu/）があれば、build の中の apt の取得元をそこへ向ける。
set -euo pipefail

[ "$#" -ge 2 ] && [ "$#" -le 3 ] || { sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
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

# apt の取得元は CI の tests の job だけが向け直す（.github/workflows/tests.yml。手元・release・rebuild --from は渡さず、
# image/Dockerfile の既定の archive・security のまま）。渡さないときは build-arg を足さない（層の cache を変えない）
args=(--build-arg "ANDON_REVISION=$rev" --build-arg "ANDON_VERSION=$version")
if [ -n "${ANDON_APT_MIRROR:-}" ]; then
  args+=(--build-arg "APT_MIRROR=$ANDON_APT_MIRROR")
fi
docker build --file "$checkout/image/Dockerfile" "${args[@]}" --tag "$tag" "$checkout"
