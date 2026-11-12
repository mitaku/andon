#!/usr/bin/env bash
# tests/lib/docs-scope.sh < 変更されたパス（repo からの相対・1行1つ）
#
# PR の変更が文書だけかを true / false で出す。CI の go と tests の job が打つ。true のとき、go の job は
# 文書を読む Go のテストだけ、tests の job は tests/lib/scope.sh が選ぶ文書を読む tests/*.sh だけを走らせる。
# true と言えるのは、変更が1つ以上あり、全部が docs/ の下か root の人の文書のときだけ。表は docs/image.md の「CI の所要時間」。
set -euo pipefail

# 入力は終わりまで読む（途中で抜けると、pipefail の CI で前の git diff が SIGPIPE で落ちる）
seen=false other=false
while IFS= read -r p; do
  case "$p" in
    '') ;;
    docs/* | README.md | CHANGELOG.md | AGENTS.md | REVIEW.md | SECURITY.md | LICENSE) seen=true ;;
    *) other=true ;;
  esac
done
[ "$seen" = true ] && [ "$other" = false ] && echo true || echo false
