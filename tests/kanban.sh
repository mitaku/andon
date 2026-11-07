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
  grep -qx '  kind: scope' "kanban/issues/$id/questions.yaml"
  "$kanban" show "$id" --json | grep -q '"kind": *"scope"'
  "$kanban" lint >/dev/null
  ! "$kanban" ask "$id" x --kind money >/dev/null 2>&1
) || die "kanban ask --kind scope"
ok "kanban ask --kind scope が通り、questions.yaml に kind: scope が残る（lint も受ける）"

# 6. 形式 v3（ADR-0008 受け入れ条件 1）: init が version 3 と issues/ を作り、add が issues/<id>/issue.yaml を書く
(
  cd "$tmp" && mkdir v3 && cd v3
  "$kanban" init --prefix A >/dev/null
  head -n 1 kanban/config.yaml | grep -qx 'version: 3'
  [ -d kanban/issues ] && [ ! -e kanban/tasks ]
  id=$("$kanban" add 'one')
  [ -f "kanban/issues/$id/issue.yaml" ]
  "$kanban" show "$id" --json | grep -q '"issue": {'
  [ "$("$kanban" lint)" = 'ok (1 issues)' ]
) || die "kanban init・add が形式 v3 を書く"
ok "kanban init が version 3 と issues/ を作り、add が issues/<id>/issue.yaml を書く"

# 7. v2 の台帳は全部の sub command が同じ1行で止まり（終了コード 1）、何も書かない。手で移す手順を当てた写しは lint を通る
v2=$tmp/v2/app
mkdir -p "$v2/kanban/tasks/A-0001/library" "$v2/kanban/tasks/A-0002"
printf '# by hand\nversion: 2\nprefix: A\nrepos: []\n' >"$v2/kanban/config.yaml"
printf 'id: A-0001\ntitle: one\nstatus: done\n' >"$v2/kanban/tasks/A-0001/task.yaml"
printf '{"next": [{"to": "task", "text": "x"}]}\n' >"$v2/kanban/tasks/A-0001/library/report.json"
printf 'id: A-0002\ntitle: two\nstatus: todo\nafter: [A-0001]\n' >"$v2/kanban/tasks/A-0002/task.yaml"
printf -- '- n: 1\n  text: which?\n  status: open\n  asked: 2026-11-01T00:00:00Z\n' >"$v2/kanban/tasks/A-0002/questions.yaml"
mkdir -p "$v2/kanban/tasks/A-0002/library/questions"
printf '{"kind": "scope", "question": "which?", "options": [{"label": "a"}], "from": "task"}\n' >"$v2/kanban/tasks/A-0002/library/questions/1.json"
want='kanban: config.yaml is format 2; move it by hand (docs/operate.md)'
before=$(cd "$v2" && find . -print0 | sort -z | xargs -0 ls -ld --time-style=+%s.%N 2>/dev/null; find . -type f -print0 | sort -z | xargs -0 cat)
for c in 'init --root .' 'add x' list ready 'show A-0001' 'start A-0002' 'set A-0002 title=y' 'review A-0002' 'done A-0002' \
         'report A-0002' lint tick 'routine list' 'config set chief_of_staff=t9' 'ask A-0002 why' questions \
         'answer A-0002#1 this' 'received A-0002#1' 'migrate --root .' root; do
  # shellcheck disable=SC2086
  rc=0; (cd "$v2" && KANBAN_ROOT= "$kanban" $c </dev/null >"$tmp/v2.out" 2>"$tmp/v2.err") || rc=$?
  [ "$rc" = 1 ] && [ "$(cat "$tmp/v2.err")" = "$want" ] && [ ! -s "$tmp/v2.out" ] || die "v2 の台帳で kanban $c: 終了コード $rc・$(cat "$tmp/v2.err" "$tmp/v2.out")"
done
after=$(cd "$v2" && find . -print0 | sort -z | xargs -0 ls -ld --time-style=+%s.%N 2>/dev/null; find . -type f -print0 | sort -z | xargs -0 cat)
[ "$before" = "$after" ] || die "v2 の台帳に何か書いた"
ok "v2 の台帳は全部の sub command が「$want」の1行で止まり、何も書かない"
(
  # ADR-0008 決めたこと 7 の手順 3〜5（docs/ledger.md「Changes from version 2」）を写しに当てる
  cd "$v2/kanban"
  mv tasks issues
  for d in issues/*/; do mv "$d/task.yaml" "$d/issue.yaml"; done
  sed -i 's/^version: 2$/version: 3/' config.yaml
  for f in issues/*/library/report.json; do [ -e "$f" ] && sed -i 's/"to": *"task"/"to": "issue"/g' "$f"; done
  for f in issues/*/library/questions/*.json; do [ -e "$f" ] && sed -i 's/"from": *"task"/"from": "issue"/g' "$f"; done
  cd ..
  [ "$(KANBAN_ROOT= "$kanban" lint)" = 'ok (2 issues)' ]
  KANBAN_ROOT= "$kanban" list --all | grep -q '^A-0002 '
  KANBAN_ROOT= "$kanban" show A-0002 --json | grep -q '"text": *"which?"'
  grep -q '"from": "issue"' kanban/issues/A-0002/library/questions/1.json
  grep -q '"to": "issue"' kanban/issues/A-0001/library/report.json
  head -n 1 kanban/config.yaml | grep -qx '# by hand'
) || die "手で移した v2 の写しが lint を通らない・読めない"
ok "v2 の台帳の写しに手で移す手順を当てると kanban lint が通り、list・show が読める"

echo "kanban: $pass ok"
