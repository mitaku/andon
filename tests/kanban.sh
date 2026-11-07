#!/usr/bin/env bash
# 台帳の CLI（kanban。cmd/kanban）の振る舞い。herdr-flightdeck の deck-v0.4.0 の振る舞いテスト
# （tests/kanban/test-deck.mjs。写しのまま）を、ここで焼いた kanban に当てる。加えて deck の別名と --version。
# node が要る（CI では無ければ失敗、手元では skip）。
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. tests/lib/kanban.sh
if ! command -v node >/dev/null; then
  if [ "${CI:-false}" = true ]; then echo 'FAIL kanban: CI requires node' >&2; exit 1; fi
  echo 'skip: node が無い'; exit 0
fi
pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }

# 1. deck-v0.4.0 の振る舞いテスト（K4）
out=$(DECK_BIN="$kanban" node tests/kanban/test-deck.mjs) || { echo "$out"; die "test-deck.mjs"; }
echo "$out" | tail -1 | grep -qE ': [0-9]+ passed, 0 failed$' || { echo "$out"; die "test-deck.mjs の結果の行"; }
ok "test-deck.mjs（$(echo "$out" | tail -1 | sed 's/.*: //')）"

# 2. deck は kanban の別名（1版だけ。K5）: 同じバイナリを deck の名前で打っても help は同じ
ln -s "$kanban" "$tmp/deck"
help_out=$("$kanban" --help)
[ "$help_out" = "$("$tmp/deck" --help)" ] || die "kanban --help と deck --help が違う"
head -n 1 <<<"$help_out" | grep -q '^kanban: ' || die "kanban --help の1行目"
grep -q '^deck is another name for kanban' <<<"$help_out" || die "kanban --help に deck の別名が無い"
ok "kanban --help と deck --help が同じ"

# 3. --version（K4 で足す）。go build のままなら dev、CI は -X main.version で版を入れる
"$kanban" --version | grep -qE '^kanban [^ ]+$' || die "kanban --version: $("$kanban" --version)"
ok "kanban --version（$("$kanban" --version)）"

# 4. 形式の文書と schema が指す文書が repo にある（写す前の repo の concept.md・mod-design.md などを指したままにしない）
#    brief.md・report.md は台帳の中のファイルの名前、coordinator.md は前の版の台帳のファイルの名前（ledger.md の履歴）、docs/ledger-format.md は写す元（herdr-flightdeck）の文書なので除く
refs=$(grep -ohE '(docs/)?[A-Za-z0-9_-]+\.md' schema/ledger/*.json docs/ledger.md $(ls cmd/kanban/*.go | grep -v _test.go) | grep -vxE 'brief\.md|report\.md|coordinator\.md|docs\/ledger-format\.md' | sort -u)
for r in $refs; do
  [ -f "$r" ] || [ -f "docs/$r" ] || [ -f "docs/adr/$r" ] || die "指す先が無い文書: $r（schema/ledger・docs/ledger.md・cmd/kanban）"
done
ok "schema/ledger・docs/ledger.md・cmd/kanban が指す文書は全部ある"

# 5. 範囲の問い（ADR-0006 受け入れ条件 5）: kanban ask --kind scope が通り、questions.yaml に kind: scope が残り、lint が受ける
(
  cd "$tmp" && mkdir scope && cd scope
  "$kanban" init --prefix S >/dev/null
  id=$("$kanban" add '誤字を1つ直す')
  [ "$("$kanban" ask "$id" '誤字は1つではなく3つ。どれを直すか' --kind scope)" = "$id#1" ]
  grep -qx '  kind: scope' "kanban/tasks/$id/questions.yaml"
  "$kanban" show "$id" --json | grep -q '"kind": *"scope"'
  "$kanban" lint >/dev/null
  ! "$kanban" ask "$id" x --kind money >/dev/null 2>&1
) || die "kanban ask --kind scope"
ok "kanban ask --kind scope が通り、questions.yaml に kind: scope が残る（lint も受ける）"

echo "kanban: $pass ok"
