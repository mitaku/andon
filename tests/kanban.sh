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
[ "$("$kanban" --help)" = "$("$tmp/deck" --help)" ] || die "kanban --help と deck --help が違う"
"$kanban" --help | head -1 | grep -q '^kanban: ' || die "kanban --help の1行目"
"$kanban" --help | grep -q '^deck is another name for kanban' || die "kanban --help に deck の別名が無い"
ok "kanban --help と deck --help が同じ"

# 3. --version（K4 で足す）。go build のままなら dev、CI は -X main.version で版を入れる
"$kanban" --version | grep -qE '^kanban [^ ]+$' || die "kanban --version: $("$kanban" --version)"
ok "kanban --version（$("$kanban" --version)）"

echo "kanban: $pass ok"
