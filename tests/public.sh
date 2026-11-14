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
)
# 大文字小文字を区別せずに当てる形（PASSWORD=・Api_Key: も）
secret_patterns_i=(
  '(password|passwd|secret|token|api_?key)[[:space:]]*[=:][[:space:]]*["'"'"']?[A-Za-z0-9/+_.-]{12,}'
)

files=()
while IFS= read -r -d '' f; do files+=("$f"); done < <(
  # tests/.task-tmp は tests/isolation の一時置き場（検査が書いた写しで、repo のファイルではない）
  find "$root" -type f -not -name .git -not -path '*/.git/*' -not -path '*/node_modules/*' -not -path "$root/tests/.task-tmp/*" -print0
)

denylist_abs=""
if [ -f "$denylist" ]; then denylist_abs="$(cd "$(dirname "$denylist")" && pwd)/$(basename "$denylist")"; fi
targets=()                                          # 禁止リストそのものは検査しない
for f in "${files[@]}"; do [ "$f" = "$denylist_abs" ] || targets+=("$f"); done
fail=0

# 当たりは「[種類] ファイル名:行番号」だけを出す。行の中身と当たった語は出さない
# （public の repo では Actions のログも公開される。秘密や固有の値がログに残らないように）。
report() { # report <ラベル> <ファイル> <行番号>
  echo "[$1] ${2#"$root"/}:$3"
}

# scan <ラベル> <grep の引数...>: 全ファイルに grep を1回（xargs が引数の長さで分けるときだけ数回）当てる。
# -Z でファイル名の後ろを NUL にし、行番号だけを読む（ファイル名に : があっても切り違えない）。
scan() {
  local label=$1; shift
  local f rest
  [ "${#targets[@]}" -gt 0 ] || return 0
  while IFS= read -r -d '' f && IFS= read -r rest; do
    report "$label" "$f" "${rest%%:*}"; fail=1
  done < <(printf '%s\0' "${targets[@]}" | xargs -0 grep -HInZ "$@" -- 2>/dev/null || true)
}

args=(); for p in "${secret_patterns[@]}"; do args+=(-e "$p"); done
scan "secret" -E "${args[@]}"
args=(); for p in "${secret_patterns_i[@]}"; do args+=(-e "$p"); done
scan "secret" -iE "${args[@]}"

if [ -f "$denylist" ]; then
  words=$(mktemp); trap 'rm -f "$words"' EXIT
  grep -v -e '^$' -e '^#' -- "$denylist" >"$words" || true   # 空行と # の行は語ではない（空の語は全行に当たる）
  [ -s "$words" ] && scan "denylist" -iF -f "$words"
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
