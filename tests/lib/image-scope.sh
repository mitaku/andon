#!/usr/bin/env bash
# tests/lib/image-scope.sh < 変更されたパス（repo からの相対・1行1つ）
#
# PR の変更に image の build と runtime の検査（tests/image.sh）が要るかを true / false で出す。CI の tests の job が打つ。
# 要らない（false）と言えるのは、変更が全部「image に入らず、tests/image.sh も読まないもの」のときだけ。
# 知らないパスは要る（true）に倒す。表と理由は docs/image.md の「CI の所要時間」。
set -euo pipefail

need=false
while IFS= read -r p; do
  # root の Go のテスト（workflow_test.go など。image に入らない）
  case "$p" in */*) ;; *_test.go) continue ;; esac
  case "$p" in
    '') ;;
    # image の検査そのもの・それを走らせる道具
    tests/image.sh | tests/lib/pty-probe.py | tests/lib/run-tests.sh | tests/lib/image-scope.sh) need=true ;;
    # image に入らない場所の Go のテスト（go build は読まない。package/ の下は image に丸ごと入るので含めない）・
    # 人用の文書・ほかの tests
    cmd/*_test.go | internal/*_test.go | docs/* | tests/*) ;;
    README.md | README.ja.md | CHANGELOG.md | AGENTS.md | REVIEW.md | SECURITY.md | LICENSE) ;;
    # セルに main の commit を当てる道具（ADR-0027）。image/ の下だが Dockerfile は COPY しない
    image/apply-main.sh) ;;
    # repo の中の skill の置き場（ADR-0035 決めたこと 4。image に入らず、tests/verify-andon.sh・words.sh が見る）
    .claude/skills/* | .agents/skills/*) ;;
    # yomiyasu の許可の一覧（image に入らず、tests/package.sh・words.sh が見る）
    .yomiyasu-words/*) ;;
    # 課題の一生のモデルと検査器のバージョン（ADR-0025。image に入らず、job model が回す）
    model/*) ;;
    # それ以外（image/・package/・schema/・cmd/・internal/・init/・go.mod・go.sum・embed.go・.dockerignore・
    # mise.toml・.github/ と、まだ無いパス）は要る
    *) need=true ;;
  esac
done
echo "$need"
