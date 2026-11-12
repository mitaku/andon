#!/usr/bin/env bash
# tests/words.sh が「今」の語で落ち、例外では落ちないこと（ADR-0013 受け入れ条件 3）。
# 一時の根に小さな写しを作り、words.sh にその根を渡す。
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail=0
adr=docs/adr/0013-short-role-names-and-plain-japanese.md

# setup: ADR-0013 の写しと、版の節のある CHANGELOG だけの根
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r/docs/adr"
  cp "$adr" "$tmp/r/$adr"
  printf '# CHANGELOG\n\n## 未リリース\n\n## v0.0.1\n- 費用を数えた\n' >"$tmp/r/CHANGELOG.md"
}
expect() { # expect ok|fail <説明>
  if bash tests/words.sh "$tmp/r" >/dev/null 2>&1; then got=ok; else got=fail; fi
  if [ "$got" = "$1" ]; then echo "ok   $2"; else echo "FAIL $2（$got）"; fail=1; fi
}

setup; expect ok '写し（ADR-0013 の引用・語の表・語を挙げた所、CHANGELOG の版の節）は通る'
setup; printf '\n地の文で費用を数える。\n' >>"$tmp/r/$adr"; expect fail 'ADR-0013 の地の文の「費用」で落ちる'
setup; printf '\n- 人: 「費用もコストで」。直す語（持ち主→オーナー）。`起こした`\n   | 提供元 | プロバイダ | 理由 |\n' >>"$tmp/r/$adr"
expect ok 'ADR-0013 の引用・括弧・`→`・コード・語の表の「今」の列は通る'
for w in 'エージェントを起こす' 'レビュアーを起こさない' 'repo を迎えた' '文書の席だけ置く' '提供元' '起きたとき'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect fail "ほかの文書の「$w」で落ちる"
done
for w in '離席中' '問題を引き起こす' '何が起きたか'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect ok "「$w」は通る"
done
setup; sed -i 's/^## 未リリース$/&\n- 費用を数える/' "$tmp/r/CHANGELOG.md"; expect fail 'CHANGELOG の未リリースの「費用」で落ちる'

exit "$fail"
