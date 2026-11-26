#!/usr/bin/env bash
# tests/lib/cpus.sh
#
# 検査を並べる数の既定を1行で出す（tests/lib/run-tests.sh と tests/schema.sh が使う）。
# cgroup v2 の cpu.max（ANDON_CPU_MAX。既定は /sys/fs/cgroup/cpu.max）が「<quota> <period>」なら、quota ÷ period を
# 切り上げた数（1 以上）。docker の --cpus はここに出るが、nproc は見ない。cpu.max が無い・読めない・「max」・数でない
# ときは nproc（それも打てなければ 2）。cgroup v1 は読まない。
set -uo pipefail

max=${ANDON_CPU_MAX:-/sys/fs/cgroup/cpu.max}
quota='' period=''
[ -r "$max" ] && read -r quota period <"$max" 2>/dev/null
if [[ "$quota" =~ ^[0-9]+$ && "$period" =~ ^[0-9]+$ ]] && [ "$period" -gt 0 ]; then
  n=$(((quota + period - 1) / period))
  [ "$n" -ge 1 ] || n=1
  echo "$n"
else
  nproc 2>/dev/null || echo 2
fi
