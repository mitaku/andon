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
# 秘密の語は大文字でも当たる（A-0200）
mkdir "$tmp/passuc";  echo 'PASS''WORD = "hunter2hunter2hunter2"' > "$tmp/passuc/a.md"
expect fail "PASSWORD= の形（大文字）に当たると落ちる" "$tmp/passuc" "$tmp/list.txt"
# tests/.task-tmp（tests/isolation の一時置き場）は見ない
mkdir -p "$tmp/tasktmp/tests/.task-tmp/x"; echo "see forbidden-example here" > "$tmp/tasktmp/tests/.task-tmp/x/a.md"
expect pass "tests/.task-tmp の中は見ない" "$tmp/tasktmp" "$tmp/list.txt"
# 禁止リストの空行と # の行は語ではない（空の語で全部に当たらない）
printf '# コメント\n\nforbidden-example\n' > "$tmp/list2.txt"
expect pass "禁止リストの空行と # の行は語として当てない" "$tmp/clean" "$tmp/list2.txt"
expect fail "空行と # の行のあとの語は当てる"           "$tmp/deny"  "$tmp/list2.txt"
# リストが無いときは秘密の形だけ見て通る（固有の値は見ない）
expect pass "リストが無ければ固有の値は見ずに通る" "$tmp/deny" "$tmp/none.txt"
expect fail "リストが無くても秘密の形には落ちる" "$tmp/secret" "$tmp/none.txt"
out="$(ANDON_PUBLIC_DENYLIST="$tmp/none.txt" bash "$here/public.sh" "$tmp/deny" 2>&1)"
case "$out" in *"固有の値は見ていない"*) echo "ok   リストが無い旨を1行出す" ;; *) echo "FAIL リストが無い旨が出ない"; fail=1 ;; esac

# 当たったとき、行の中身と語を出力に出さない（public のログに値が残らないように）。出すのは種類・ファイル名・行番号だけ。
mkdir "$tmp/leak"
printf 'line one\nsee forbidden-example and AKIA%s here\n' "ABCDEFGHIJKLMNOP" > "$tmp/leak/a.md"
out="$(ANDON_PUBLIC_DENYLIST="$tmp/list.txt" bash "$here/public.sh" "$tmp/leak" 2>&1 || true)"
leak=""
case "$out" in *forbidden-example*|*ABCDEFGHIJKLMNOP*|*"see "*) leak=1 ;; esac
case "$out" in *"[secret] a.md:2"*"[denylist] a.md:2"*) ;; *) leak="${leak}形" ;; esac
# ファイル名に : があっても、行番号を切り違えない
mkdir "$tmp/colon"; printf 'x\nsee forbidden-example\n' > "$tmp/colon/a:1.md"
out="$(ANDON_PUBLIC_DENYLIST="$tmp/list.txt" bash "$here/public.sh" "$tmp/colon" 2>&1 || true)"
case "$out" in *"[denylist] a:1.md:2"*) echo "ok   ファイル名に : があっても行番号を出す" ;; *) echo "FAIL ファイル名に : があるときの当たり: $out"; fail=1 ;; esac
if [ -z "$leak" ]; then echo "ok   当たりは種類とファイル名:行番号だけを出し、中身と語を出さない"; else echo "FAIL 当たりの出力に中身か語が出ている、または形が違う: $out"; fail=1; fi

exit "$fail"
