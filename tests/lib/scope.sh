#!/usr/bin/env bash
# tests/lib/scope.sh < 変更されたパス（repo からの相対・1行1つ）
#
# 手元で「変えたものに関係する検査だけ」を走らせるとき（tests/run --changed）に、走らせる tests/*.sh を1行1つで出す。
# 安く全体を見るもの（public・public-selftest・smoke・ci）はいつも。tests/image.sh は CI と同じ
# tests/lib/image-scope.sh が true のときだけ。知らないパスは image.sh 以外を全部にする。表は docs/image.md の「CI の所要時間」。
set -euo pipefail
cd "$(dirname "$0")/../.."

declare -A run=([tests/public.sh]=1 [tests/public-selftest.sh]=1 [tests/smoke.sh]=1 [tests/ci.sh]=1)
all=false
paths=()
while IFS= read -r p; do
  [ -n "$p" ] || continue
  paths+=("$p")
  # root の Go のテスト（workflow_test.go など。image に入らない）
  case "$p" in */*) ;; *_test.go) continue ;; esac
  # kanban.sh は schema/ledger・docs/ledger.md・cmd/kanban が指す docs/ の文書があるかを見る。消した・動かした文書で足す
  case "$p" in docs/*) [ -e "$p" ] || run[tests/kanban.sh]=1 ;; esac
  case "$p" in
    # contract.md は例を schema と（schema.sh）、§4 の acceptance[] の行を skill と（package.sh）突き合わせる
    docs/contract.md) run[tests/schema.sh]=1 run[tests/package.sh]=1 ;;
    docs/ledger.md) run[tests/kanban.sh]=1 ;;
    docs/adr/*) run[tests/package.sh]=1 ;;
    tests/kanban/*) run[tests/kanban.sh]=1 ;;
    # 流れと状態の定義（ADR-0017）。dispatcher.md §7.2 は once: true の移り方と突き合わせる
    process/* | schema/process/* | docs/process/* | tests/process/* | docs/dispatcher.md) run[tests/process.sh]=1 ;;
    tests/lib/pty-probe.py | tests/image.sh) ;; # image-scope が見る
    tests/lib/*) all=true ;;
    tests/*.sh) [ -f "$p" ] && run[$p]=1 ;;
    # Go のテスト（go test ./... が見る）・人の文書は、いつもの分だけ
    cmd/*_test.go | internal/*_test.go | docs/* | tests/* | README.md | CHANGELOG.md | AGENTS.md | REVIEW.md | SECURITY.md | LICENSE) ;;
    *) all=true ;;
  esac
done
if [ "$all" = true ]; then
  for f in tests/*.sh; do run[$f]=1; done
fi
unset 'run[tests/image.sh]'
[ "$(printf '%s\n' "${paths[@]+"${paths[@]}"}" | bash tests/lib/image-scope.sh)" = true ] && run[tests/image.sh]=1
for f in tests/*.sh; do [ -n "${run[$f]:-}" ] && echo "$f"; done
exit 0
