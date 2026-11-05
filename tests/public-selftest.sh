#!/usr/bin/env bash
# tests/public.sh が、きれいな木では通り、禁止リストと秘密の形に当たると落ちることを確かめる。
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fail=0
expect() { # expect <期待: pass|fail> <説明> <根> <禁止リスト>
  local want=$1 name=$2 dir=$3 list=$4 got=pass
  ANDON_PUBLIC_DENYLIST="$list" bash "$here/public.sh" "$dir" >/dev/null 2>&1 || got=fail
  if [ "$got" = "$want" ]; then echo "ok   $name"; else echo "FAIL $name（期待 $want、実際 $got）"; fail=1; fi
}

printf 'forbidden-example\n' > "$tmp/list.txt"

mkdir "$tmp/clean";   echo "hello example.com" > "$tmp/clean/a.md"
mkdir "$tmp/deny";    echo "see forbidden-example here" > "$tmp/deny/a.md"
mkdir "$tmp/denyci";  echo "see FORBIDDEN-Example here" > "$tmp/denyci/a.md"
mkdir "$tmp/secret";  echo "key AKIA""ABCDEFGHIJKLMNOP" > "$tmp/secret/a.md"
mkdir "$tmp/pass";    echo 'pass''word = "hunter2hunter2hunter2"' > "$tmp/pass/a.md"
mkdir "$tmp/pem";     echo "-----BEGIN RSA PRIV""ATE KEY-----" > "$tmp/pem/a.md"

expect pass "きれいな木は通る"                 "$tmp/clean"  "$tmp/list.txt"
expect fail "禁止リストの語に当たると落ちる"   "$tmp/deny"   "$tmp/list.txt"
expect fail "禁止リストは大文字小文字を区別しない" "$tmp/denyci" "$tmp/list.txt"
expect fail "アクセスキーの形に当たると落ちる" "$tmp/secret" "$tmp/list.txt"
expect fail "password= の形に当たると落ちる"   "$tmp/pass"   "$tmp/list.txt"
expect fail "秘密鍵の見出しに当たると落ちる"   "$tmp/pem"    "$tmp/list.txt"
# リストが無いときは秘密の形だけ見て通る（固有の値は見ない）
expect pass "リストが無ければ固有の値は見ずに通る" "$tmp/deny" "$tmp/none.txt"
expect fail "リストが無くても秘密の形には落ちる" "$tmp/secret" "$tmp/none.txt"
out="$(ANDON_PUBLIC_DENYLIST="$tmp/none.txt" bash "$here/public.sh" "$tmp/deny" 2>&1)"
case "$out" in *"固有の値は見ていない"*) echo "ok   リストが無い旨を1行出す" ;; *) echo "FAIL リストが無い旨が出ない"; fail=1 ;; esac

exit "$fail"
