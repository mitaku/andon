#!/usr/bin/env bash
# sshd の SetEnv（image/sshd_config.d/andon.conf）が image/Dockerfile の ENV（HOME 以外）と揃っているか。
# compose（package/compose.yaml）が environment で上書きするものは compose の値と比べる。
# 実機の比較（ssh のセッションと docker exec の env）は tests/image.sh がする。
set -euo pipefail
cd "$(dirname "$0")/.."

conf=image/sshd_config.d/andon.conf
# ubuntu の base image の PATH（Dockerfile の ${PATH}）
base_path=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

setenv_lines=$(grep -c '^SetEnv ' "$conf" || true)
if [ "$setenv_lines" != 1 ]; then
  echo "FAIL sshd-env: $conf の SetEnv は1行に（今 $setenv_lines 行。2行目以降は sshd が黙って無視する）" >&2
  exit 1
fi
declare -A got
for kv in $(sed -n 's/^SetEnv //p' "$conf"); do got[${kv%%=*}]=${kv#*=}; done

declare -A want
while IFS= read -r kv; do
  k=${kv%%=*} v=${kv#*=}
  [ "$k" = HOME ] && continue
  want[$k]=${v//\$\{PATH\}/$base_path}
done < <(sed -n 's/^ENV //p' image/Dockerfile)
# compose の environment（`      KEY: value` の行）が上書きするもの
while IFS= read -r line; do
  k=${line%%:*} v=${line#*: }
  [ -n "${want[$k]+x}" ] && want[$k]=$v
done < <(sed -n '/^    environment:/,/^    [a-z]/{s/^      \([A-Z_][A-Z0-9_]*\): \(.*\)$/\1: \2/p}' package/compose.yaml)

fail=0
for k in "${!want[@]}"; do
  if [ "${got[$k]-<無い>}" != "${want[$k]}" ]; then
    echo "FAIL sshd-env: $k が揃っていない（SetEnv: ${got[$k]-<無い>} / image: ${want[$k]}）" >&2
    fail=1
  fi
done
for k in "${!got[@]}"; do
  if [ -z "${want[$k]+x}" ]; then
    echo "FAIL sshd-env: SetEnv の $k は image の ENV に無い" >&2
    fail=1
  fi
done
[ "$fail" = 0 ] || exit 1
echo "ok   sshd-env: SetEnv matches image ENV (${#want[@]} keys)"
