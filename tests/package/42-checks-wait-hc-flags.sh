# tests/package/42-checks-wait-hc-flags.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  42. worker skill §3 の 4 の checks の待ちの起動の例が、hc の旗（--json・--bg・--no-notify・--name）を全部 -- の前に書き、
#      -- の後ろ（gh の引数）に hc の旗を書かない。gh pr checks を待つ例を全部見る。
#      --json を -- の後ろに書いた写しと、2つ目の誤った例を足した写しで当たることも確かめる。
# === 42. checks の待ちの hc run の旗の位置（worker skill §3 の 4） ===
wsk=package/skills/worker/SKILL.md
# ck_miss <worker SKILL.md>: 足りないものを1行で出す（空なら合っている）
ck_miss() {
  local m='' cmds cmd pre post f
  # gh pr checks を待つ例（バッククォートで囲んだ hc run の command）を全部見る
  cmds=$(grep -F 'gh pr checks' "$1" | grep -oE '`hc run [^`]*`' | tr -d '`' | grep -F -- ' -- gh pr checks ')
  [ -n "$cmds" ] || { echo " [checks の待ちの hc run の例が無い]"; return; }
  while IFS= read -r cmd; do
    pre=${cmd%% -- *}; post=${cmd#* -- }
    for f in --json --bg --no-notify --name; do
      [[ " $pre " == *" $f "* || " $pre " == *" $f="* ]] || m="$m [$f が -- の前に無い]"
    done
    for f in --json --bg --no-notify --name --rm --keep --cwd; do
      if [[ " $post " == *" $f "* || " $post " == *" $f="* ]]; then m="$m [$f が -- の後ろにある]"; fi
    done
  done <<<"$cmds"
  grep -qF '返った JSON の `address` を控える' "$1" || m="$m [JSON の address を控える文が無い]"
  echo "$m"
}
miss=$(ck_miss "$wsk")
[ -z "$miss" ] && pass "worker skill §3 の 4: checks の待ちの hc run が hc の旗を全部 -- の前に書く" \
  || fail "worker skill §3 の 4: checks の待ちの hc run の旗の位置が違う:$miss"
# 根拠の出来事の形（--json を -- の後ろに書いて gh に渡す）に当たることを確かめる
ckbad=$tmp/ck-bad-skill.md
sed 's/`hc run --json --bg --no-notify --name \([^`]*\) -- gh pr checks <n> --watch`/`hc run --bg --no-notify --name \1 -- gh pr checks <n> --watch --json`/' "$wsk" >"$ckbad"
miss=$(ck_miss "$ckbad")
[[ "$miss" == *"[--json が -- の前に無い]"* && "$miss" == *"[--json が -- の後ろにある]"* ]] \
  && pass "worker skill §3 の 4 の検査は、--json を -- の後ろに書いた例に当たる" \
  || fail "worker skill §3 の 4 の検査が --json を -- の後ろに書いた例に当たらない: [$miss]"
# 正しい例の後ろに、--json= を -- の後ろに書いた2つ目の例を足した写しにも当たる
ckbad2=$tmp/ck-bad2-skill.md
{ cat "$wsk"; echo '別の例: `hc run --json --bg --no-notify --name x -- gh pr checks 12 --watch --json=state`'; } >"$ckbad2"
miss=$(ck_miss "$ckbad2")
[[ "$miss" == *"[--json が -- の後ろにある]"* ]] \
  && pass "worker skill §3 の 4 の検査は、2つ目の例の -- の後ろの --json= にも当たる" \
  || fail "worker skill §3 の 4 の検査が2つ目の例の -- の後ろの --json= に当たらない: [$miss]"
