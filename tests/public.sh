#!/usr/bin/env bash
# public に耐えるかの検査（docs/design.md §2.6）。
#   秘密の形と、固有の値の禁止リストを、全ファイルに当てる。当たれば落ちる。
# 使い方: bash tests/public.sh [検査する根（既定: repo の根）]
# 禁止リストは repo の外から読む（名前を履歴に残さないため）。場所は
#   ${ANDON_PUBLIC_DENYLIST:-$HOME/.config/andon/public-denylist}
# 無ければ秘密の形だけ検査して通す。書き方は tests/README.md。
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "${1:-$here/..}" && pwd)"
denylist="${ANDON_PUBLIC_DENYLIST:-${HOME:-/nonexistent}/.config/andon/public-denylist}"

# 秘密の形（ERE）。行頭の # はコメント。
secret_patterns=(
  'AKIA[0-9A-Z]{16}'
  'gh[pousr]_[A-Za-z0-9]{36,}'
  'github_pat_[A-Za-z0-9_]{22,}'
  'sk-ant-[A-Za-z0-9_-]{20,}'
  'xox[abprs]-[A-Za-z0-9-]{10,}'
  '-----BEGIN [A-Z ]*PRIVATE KEY-----'
  '(password|passwd|secret|token|api_?key)[[:space:]]*[=:][[:space:]]*["'"'"']?[A-Za-z0-9/+_.-]{12,}'
)

files=()
while IFS= read -r -d '' f; do files+=("$f"); done < <(
  find "$root" -type f -not -name .git -not -path '*/.git/*' -not -path '*/node_modules/*' -print0
)

denylist_abs=""
if [ -f "$denylist" ]; then denylist_abs="$(cd "$(dirname "$denylist")" && pwd)/$(basename "$denylist")"; fi
fail=0

scan() { # scan <ラベル> <grep の引数...>
  local label=$1; shift
  local f
  for f in "${files[@]}"; do
    [ "$f" = "$denylist_abs" ] && continue          # 禁止リストそのものは検査しない
    if grep -InHE -e "$@" -- "$f" 2>/dev/null | sed "s|^$root/||; s|^|[$label] |"; then
      fail=1
    fi
  done
}

for p in "${secret_patterns[@]}"; do scan "secret" "$p"; done

if [ -f "$denylist" ]; then
  while IFS= read -r word || [ -n "$word" ]; do
    case "$word" in ''|'#'*) continue ;; esac
    for f in "${files[@]}"; do
      [ "$f" = "$denylist_abs" ] && continue
      if grep -InHiF -e "$word" -- "$f" 2>/dev/null | sed "s|^$root/||; s|^|[denylist] |"; then
        fail=1
      fi
    done
  done < "$denylist"
fi

if [ "$fail" -ne 0 ]; then
  echo "FAIL public: public に出せない値が見つかった" >&2
  exit 1
fi
if [ -f "$denylist" ]; then
  echo "ok   public: 秘密の形と禁止リストに当たるものは無い"
else
  echo "ok   public: 秘密の形に当たるものは無い（禁止リストが無いので固有の値は見ていない）"
fi
