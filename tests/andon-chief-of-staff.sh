#!/usr/bin/env bash
# `andon chief-of-staff`（docs/design.md §1.8 の Chief of Staff を起こす）。本物の台帳の CLI（kanban）・一時の台帳・偽の hc で回す。
# 起こす・動いていれば起こさない（二重に起こさない）・起き直したら coordinator が直る・2人なら止まる・最初のメッセージが届かないときの案内。
# 本物の hc には触らない。
set -euo pipefail
for c in jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config"
unset FLIGHTDECK_ROOT HC_TERMINAL_ID
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
  get) awk -v a="\$2" '\$1 == a {print \$1, \$2, \$3}' \$d/live | jq -Rn '[inputs | split(" ") | {address: .[0], name: .[1], cwd: .[2]}] | .[0] // error("no")' ;;
  templates) echo '{"templates":[{"name":"claude"},{"name":"chief-of-staff"}]}' ;;
  run) n=\$(( \$(cat \$d/next 2>/dev/null || echo 42) )); echo \$((n + 1)) > \$d/next
       name= cwd= role= project=
       while [ \$# -gt 0 ]; do case \$1 in
         --name) name=\$2; shift;; --cwd) cwd=\$2; shift;;
         --label) case \$2 in role=*) role=\${2#role=};; project=*) project=\${2#project=};; esac; shift;;
       esac; shift; done
       echo "t\$n \$name \$cwd \$role \$project" >> \$d/live
       sent=\$(cat \$d/first 2>/dev/null || echo true)
       echo "{\"address\":\"t\$n\",\"first_message\":{\"sent\":\$sent,\"reason\":\"trust prompt\"}}" ;;
  attach) echo "attach \$2" >> \$d/attached ;;
  *) echo "unexpected hc \$1" >&2; exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/hc"
export PATH="$tmp/bin:$PATH"

pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }
runs() { cat "$tmp/hc/log" 2>/dev/null | grep -c '^run' || true; }
coordinator() { sed -n 's/^coordinator: *//p' .flightdeck/config.yaml; }

# 台帳の外で打つと止まる
cd "$tmp"
rc=0; "$andon" chief-of-staff --no-attach >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q '台帳が見つからない' "$tmp/err" && [ "$(runs)" = 0 ] || die "台帳の外で: rc=$rc $(cat "$tmp/err")"
ok '台帳の外では起こさずに止まる'

cd "$tmp/projects/demo"
kanban init --prefix D >/dev/null
root=$PWD

# 1. 起こす
"$andon" chief-of-staff --no-attach >"$tmp/out"
[ "$(runs)" = 1 ] || die "hc run の回数: $(runs)"
grep -q "^run --json --bg --outlive-parent --no-notify --name demo-cos --label role=chief-of-staff --label project=demo --cwd $root chief-of-staff プロジェクト \`demo\` の Chief of Staff。" "$tmp/hc/log" \
  || die "hc run の形が違う: $(cat "$tmp/hc/log")"
! grep -q 'SKILL.md\|先に' "$tmp/hc/log" || die '最初のメッセージに skill を読ませる行が残る（ADR-0001）'
[ "$(coordinator)" = t42 ] || die "coordinator: $(coordinator)"
jq -e 'select(.config) | .config.coordinator == "t42"' .flightdeck/log.jsonl >/dev/null || die 'log.jsonl に config の行が無い'
grep -q 'demo の Chief of Staff は t42（demo-cos・起こした）' "$tmp/out" && [ ! -e "$tmp/hc/attached" ] || die "out: $(cat "$tmp/out")"
ok '起こす: hc run（ラベル role=chief-of-staff project=demo・cwd は台帳の root）・config.yaml の coordinator・--no-attach なら乗らない'

# 2. 動いていれば起こさない（Chief of Staff 自身が起きたときに打つ形）
lines=$(wc -l < .flightdeck/log.jsonl)
"$andon" chief-of-staff --no-attach >"$tmp/out"
[ "$(runs)" = 1 ] || die '二重に起こした'
[ "$(wc -l < .flightdeck/log.jsonl)" = "$lines" ] || die 'coordinator が合っているのに台帳を書いた'
grep -q 't42（demo-cos・動いているものを使う）' "$tmp/out" || die "out: $(cat "$tmp/out")"
ok '動いていれば起こさない（二重に起こさない）。coordinator が合っていれば台帳を書かない'

# 3. 起き直した（端末が変わった）: coordinator が直る
sed -i 's/^t42 /t50 /' "$tmp/hc/live"
"$andon" chief-of-staff --no-attach >/dev/null
[ "$(runs)" = 1 ] && [ "$(coordinator)" = t50 ] || die "coordinator が直らない: $(coordinator) runs=$(runs)"
ok '起き直して address が変わったら coordinator を直す'

# 4. 名前か cwd が合わない端末は使わない
cp "$tmp/hc/live" "$tmp/live.bak"
sed -i "s| $root | /elsewhere |" "$tmp/hc/live"
rc=0; "$andon" chief-of-staff --no-attach >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'cwd（/elsewhere）が合わない' "$tmp/err" && [ "$(coordinator)" = t50 ] || die "合わない端末: rc=$rc $(cat "$tmp/err")"
cp "$tmp/live.bak" "$tmp/hc/live"
ok 'ラベルが合っても名前か cwd が違う端末は使わない（台帳を書かない）'

# 5. 2人なら止まる
echo "t51 demo-cos2 $root chief-of-staff demo" >> "$tmp/hc/live"
rc=0; "$andon" chief-of-staff --no-attach >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 1 ] && grep -q 'Chief of Staff が 2 人いる: t50 t51' "$tmp/err" && [ "$(runs)" = 1 ] || die "2人: rc=$rc $(cat "$tmp/err")"
cp "$tmp/live.bak" "$tmp/hc/live"
ok '2人いれば止まる（1プロジェクトに1人）'

# 6. 最初のメッセージが届かない（信頼の画面など）
: > "$tmp/hc/live"
echo false > "$tmp/hc/first"
rc=0; "$andon" chief-of-staff --no-attach >/dev/null 2>"$tmp/err" || rc=$?
[ "$rc" = 3 ] || die "rc=$rc $(cat "$tmp/err")"
grep -q '最初のメッセージが届いていない: trust prompt。hc attach t43 で画面に答えてから、次を送る: hc send t43 "プロジェクト `demo` の Chief of Staff。' "$tmp/err" || die "案内: $(cat "$tmp/err")"
[ "$(coordinator)" = t43 ] || die "届かなくても台帳に載せる: $(coordinator)"
rm "$tmp/hc/first"
ok '最初のメッセージが届かなければ終了コード 3・送り直す hc send を案内（台帳には載せる）'

echo "all passed ($pass)"
