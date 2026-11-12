#!/usr/bin/env bash
# `andon chief-of-staff`（docs/design.md §1.8 の Chief of Staff を起動する）。本物の台帳の CLI（kanban）・一時の台帳・偽の hc で回す。
# 起動する・動いていれば起動しない（二重に起動しない）・起き直したら chief_of_staff が直る・2人なら止まる・最初のメッセージが届かないときの案内。
# 本物の hc には触らない。
set -euo pipefail
for c in jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT   # Go が落とす toolchain は読むだけで置かれる
# helper の go build が本物の HOME の Go のキャッシュに書かないように、helper を読む前に一時の HOME とキャッシュへ向ける
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GOCACHE="$tmp/gocache" GOMODCACHE="$tmp/gomodcache" GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
mkdir -p "$HOME"
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
# helper が go build したなら、キャッシュは一時ディレクトリにある
if [ -z "${ANDON_BIN:-}" ] && [ -z "${KANBAN_BIN:-}" ]; then
  [ -n "$(ls -A "$tmp/gocache" 2>/dev/null)" ] || { echo "FAIL go のキャッシュが一時ディレクトリに無い" >&2; exit 1; }
  echo "ok   go build のキャッシュは一時ディレクトリ（$tmp/gocache）"
fi
unset KANBAN_ROOT HC_TERMINAL_ID
mkdir -p "$HOME" "$tmp/bin" "$tmp/hc" "$tmp/projects/demo"

# 偽の hc: 生きている端末は $tmp/hc/live（1行に address name cwd role project）。run は t42, t43 … を返す。
# 打ったものは $tmp/hc/log に。$tmp/hc/first があれば run の first_message はその中身（false なら届かない）
cat > "$tmp/bin/hc" <<EOF
#!/usr/bin/env bash
d=$tmp/hc
printf '%s\n' "\$*" >>\$d/log
touch \$d/live
case \$1 in
  ls)  role= project=
       while [ \$# -gt 0 ]; do case \$1 in --label) case \$2 in role=*) role=\${2#role=};; project=*) project=\${2#project=};; esac; shift;; esac; shift; done
       awk -v r="\$role" -v p="\$project" '(r == "" || \$4 == r) && (p == "" || \$5 == p) {print \$1, \$2}' \$d/live \\
         | jq -Rn '{terminals: [inputs | split(" ") | {address: .[0], name: .[1]}]}' ;;
  get) fg='"argv":["sh","-c","claude"],"status":{"app":"claude-code"},"foreground":{"name":"sh","argv":["sh","-c","claude"]}'
       # \$d/shells にある address は作り直しで戻った端末（argv ごと素のシェル・agent 無し）
       if [ -f \$d/shells ] && grep -qx "\$2" \$d/shells; then fg='"argv":["/bin/bash"],"status":null,"foreground":{"name":"bash","argv":["/bin/bash"]}'; fi
       awk -v a="\$2" '\$1 == a {print \$1, \$2, \$3, \$6, \$7}' \$d/live \\
         | jq -Rn --argjson fg "{\$fg}" '[inputs | split(" ") | (\$fg + {address: .[0], name: .[1], cwd: .[2], labels: ({"herdr.space": .[3], "herdr.tab": .[4]} | with_entries(select(.value != null and .value != "")))})] | .[0] // error("no")' ;;
  keep|kill) ;;
  rm) awk -v a="\$2" '\$1 != a' \$d/live > \$d/live.new && mv \$d/live.new \$d/live ;;
  templates) echo '{"templates":[{"name":"claude"},{"name":"chief-of-staff"}]}' ;;
  run) n=\$(( \$(cat \$d/next 2>/dev/null || echo 42) )); echo \$((n + 1)) > \$d/next
       name= cwd= role= project= space= tab=
       while [ \$# -gt 0 ]; do case \$1 in
         --name) name=\$2; shift;; --cwd) cwd=\$2; shift;;
         --label) case \$2 in role=*) role=\${2#role=};; project=*) project=\${2#project=};; herdr.space=*) space=\${2#herdr.space=};; herdr.tab=*) tab=\${2#herdr.tab=};; esac; shift;;
       esac; shift; done
       echo "t\$n \$name \$cwd \$role \$project \$space \$tab" >> \$d/live
       sent=\$(cat \$d/first 2>/dev/null || echo true)
       echo "{\"address\":\"t\$n\",\"first_message\":{\"sent\":\$sent,\"reason\":\"trust prompt\"}}" ;;
  attach) echo "attach \$2" >> \$d/attached ;;  # 打てば印が残る（andon chief-of-staff は打たない。ADR-0002 決めたこと 14）
  *) echo "unexpected hc \$1" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/hc"
export PATH="$tmp/bin:$PATH"

pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }
runs() { cat "$tmp/hc/log" 2>/dev/null | grep -c '^run' || true; }
cos() { sed -n 's/^chief_of_staff: *//p' kanban/config.yaml; }

# 台帳の外で打つと止まる
cd "$tmp"
rc=0; "$andon" chief-of-staff >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '台帳が見つからない' "$tmp/err" && [ "$(runs)" = 0 ] || die "台帳の外で: rc=$rc $(cat "$tmp/err")"
ok '台帳の外では起動せずに止まる'

cd "$tmp/projects/demo"
kanban init --prefix D >/dev/null
root=$PWD

# 1. 起動する
"$andon" chief-of-staff >"$tmp/out"
[ "$(runs)" = 1 ] || die "hc run の回数: $(runs)"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-cos --label role=chief-of-staff --label project=demo --label herdr.space=andon-demo --label herdr.tab=andon-demo-cos --cwd $root chief-of-staff プロジェクト \`demo\` の Chief of Staff。" "$tmp/hc/log" \
  || die "hc run の形が違う: $(cat "$tmp/hc/log")"
! grep -q 'SKILL.md\|先に' "$tmp/hc/log" || die '最初のメッセージに skill を読ませる行が残る（ADR-0001）'
[ "$(cos)" = t42 ] || die "chief_of_staff: $(cos)"
jq -e 'select(.config) | .config.chief_of_staff == "t42"' kanban/log.jsonl >/dev/null || die 'log.jsonl に config の行が無い'
grep -q 'demo の Chief of Staff は t42（demo-cos・起動した）' "$tmp/out" && [ ! -e "$tmp/hc/attached" ] || die "out: $(cat "$tmp/out")"
[ "$(tail -n 1 "$tmp/out")" = 'andon chief-of-staff: 出た場所 demo / CoS（t42）' ] || die "出た場所が最後に無い: $(cat "$tmp/out")"
ok '起動する: hc run（ラベル role=chief-of-staff project=demo herdr.space=andon-demo herdr.tab=andon-demo-cos・cwd は台帳の root）・config.yaml の chief_of_staff・hc attach を打たず出た場所「demo / CoS」を出す'

# 2. 動いていれば起動しない（Chief of Staff 自身が起動したときに打つ形）
lines=$(wc -l < kanban/log.jsonl)
"$andon" chief-of-staff >"$tmp/out"
[ "$(runs)" = 1 ] || die '二重に起動した'
[ "$(wc -l < kanban/log.jsonl)" = "$lines" ] || die 'chief_of_staff が合っているのに台帳を書いた'
grep -q 't42（demo-cos・動いているものを使う）' "$tmp/out" && grep -q '出た場所 demo / CoS（t42）' "$tmp/out" || die "out: $(cat "$tmp/out")"
ok '動いていれば起動しない（二重に起動しない）。chief_of_staff が合っていれば台帳を書かない'

# 3. 起き直した（端末が変わった）: chief_of_staff が直る
sed -i 's/^t42 /t50 /' "$tmp/hc/live"
"$andon" chief-of-staff >/dev/null
[ "$(runs)" = 1 ] && [ "$(cos)" = t50 ] || die "chief_of_staff が直らない: $(cos) runs=$(runs)"
ok '起き直して address が変わったら chief_of_staff を直す'

# 4. 名前か cwd が合わない端末は使わない
cp "$tmp/hc/live" "$tmp/live.bak"
sed -i "s| $root | /elsewhere |" "$tmp/hc/live"
rc=0; "$andon" chief-of-staff >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'cwd（/elsewhere）が合わない' "$tmp/err" && [ "$(cos)" = t50 ] || die "合わない端末: rc=$rc $(cat "$tmp/err")"
cp "$tmp/live.bak" "$tmp/hc/live"
ok 'ラベルが合っても名前か cwd が違う端末は使わない（台帳を書かない）'

# 5. 2人なら止まる
echo "t51 demo-cos2 $root chief-of-staff demo" >> "$tmp/hc/live"
rc=0; "$andon" chief-of-staff >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'Chief of Staff が 2 人いる: t50 t51' "$tmp/err" && [ "$(runs)" = 1 ] || die "2人: rc=$rc $(cat "$tmp/err")"
cp "$tmp/live.bak" "$tmp/hc/live"
ok '2人いれば止まる（1プロジェクトに1人）'

# 6. 最初のメッセージが届かない（信頼の画面など）
: > "$tmp/hc/live"
echo false > "$tmp/hc/first"
rc=0; "$andon" chief-of-staff >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 3 ] || die "rc=$rc $(cat "$tmp/err")"
grep -q '最初のメッセージが届いていない: trust prompt。App の demo / CoS で答えてから、次を送る: hc send t43 "プロジェクト `demo` の Chief of Staff。' "$tmp/err" || die "案内: $(cat "$tmp/err")"
[ "$(cos)" = t43 ] || die "届かなくても台帳に載せる: $(cos)"
rm "$tmp/hc/first"
[ ! -e "$tmp/hc/attached" ] || die "hc attach を打った: $(cat "$tmp/hc/attached")"
ok '最初のメッセージが届かなければ終了コード 3・送り直す hc send を案内（台帳には載せる）'

# 7. 作り直しで戻った端末（ラベルと名前はそのまま、中身が素のシェル）: 使わずに片付けて起動し直し、chief_of_staff を合わせる
echo t43 > "$tmp/hc/shells"
"$andon" chief-of-staff >"$tmp/out"
[ "$(runs)" = 3 ] && [ "$(cos)" = t44 ] || die "戻った端末: runs=$(runs) chief_of_staff=$(cos) $(cat "$tmp/out")"
grep -qx 'keep t43 --off' "$tmp/hc/log" && grep -qx 'kill t43' "$tmp/hc/log" && grep -qx 'rm t43' "$tmp/hc/log" || die "片付けていない: $(cat "$tmp/hc/log")"
grep -q 't43 は Chief of Staff のラベルのまま中身がシェルに替わっていた' "$tmp/out" && grep -q 't44（demo-cos・起動した）' "$tmp/out" || die "out: $(cat "$tmp/out")"
rm "$tmp/hc/shells"
ok '中身がシェルに替わって戻った端末は使わず、keep を外して kill・rm し、起動し直して chief_of_staff を合わせる'

# 8. --no-attach は消した（ADR-0002 決めたこと 14）
rc=0; "$andon" chief-of-staff --no-attach >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 2 ] && grep -q '^usage: andon chief-of-staff' "$tmp/err" || die "--no-attach: rc=$rc $(cat "$tmp/err")"
ok '--no-attach は使い方の誤り（attach をやめたので要らない）'

echo "all passed ($pass)"
