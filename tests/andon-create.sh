#!/usr/bin/env bash
# `andon create`（ADR-0004 決めたこと 1 の予約名で projects/<slug>/ を立てる）。本物の台帳の CLI（kanban）・一時のセルの repo・偽の ghq と hc で回す。
# 作る中身（kanban lint が通る・project.json が schema を通る・AGENTS.md）・commit しない・既にあれば何も変えずに止まる・役の端末で拒む・
# 端末でなければ聞かずに slug が無いと止まる。対話の中身は go test（internal/create）が見る。本物の hc と ghq には触らない。
set -euo pipefail
for c in jq git python3; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" GOCACHE="$tmp/gocache" GOMODCACHE="$tmp/gomodcache" GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
mkdir -p "$HOME"
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
root=$(cd "$(dirname "$0")/.." && pwd)
unset KANBAN_ROOT HC_TERMINAL_ID

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

# 偽のセルの repo（owner の commit が1つ）
ws=$tmp/ws
mkdir -p "$ws/projects"
git -C "$ws" init -q
printf 'owner = "owner@example.com"\n' >"$ws/andon.toml"
git -C "$ws" add andon.toml
git -C "$ws" -c user.name=owner -c user.email=owner@example.com commit -q -m init
export ANDON_WORKSPACE=$ws
# 偽の ghq（get を log に書き、空の clone を作る）と hc（HC_TERMINAL_ID が role で始まれば Chief of Staff の端末）
export GHQ_ROOT=$tmp/ghq
mkdir -p "$tmp/bin"
git init -q "$GHQ_ROOT/example.com/o/have"
cat >"$tmp/bin/ghq" <<EOF
#!/usr/bin/env bash
echo "\$*" >>$tmp/ghq.log
git init -q "\$GHQ_ROOT/\$2"
EOF
cat >"$tmp/bin/hc" <<'EOF'
#!/usr/bin/env bash
case "$HC_TERMINAL_ID" in role*) echo '{"labels":{"role":"chief-of-staff"}}' ;; *) echo '{"labels":{}}' ;; esac
EOF
chmod +x "$tmp/bin/ghq" "$tmp/bin/hc"
export PATH="$tmp/bin:$PATH"
head0=$(git -C "$ws" rev-parse HEAD)

# === 全部渡す: 聞かずに作る ===
p=$ws/projects/sample-site
"$andon" create sample-site --repo example.com/o/have --repo example.com/o/new --prefix SS --title 'Sample site' </dev/null >"$tmp/out" 2>"$tmp/err" \
  && pass "引数を全部渡すと聞かずに作る" || fail "create: $(cat "$tmp/out" "$tmp/err")"
[ ! -s "$tmp/err" ] && pass "何も聞かない（stderr が空）" || fail "聞いた: $(cat "$tmp/err")"
(cd "$p" && kanban lint >/dev/null) && grep -qx 'prefix: SS' "$p/kanban/config.yaml" && grep -qx '  - example.com/o/new' "$p/kanban/config.yaml" \
  && pass "kanban/ は本物の台帳（lint が通る・prefix と repos）" || fail "kanban: $(cat "$p/kanban/config.yaml" 2>&1)"
python3 "$root/package/lib/andon/schema_check.py" validate "$root/schema/project.schema.json" "$p/project.json" >/dev/null 2>"$tmp/schema" \
  && pass "project.json の下書きは schema を通る" || fail "schema: $(cat "$tmp/schema")"
[ "$(jq -c '[.project, .repos, has("delegation")]' "$p/project.json")" = '[{"title":"Sample site","prefix":"SS"},["example.com/o/have","example.com/o/new"],false]' ] \
  && pass "project.json は題・prefix・repos で、delegation が無い（無委譲）" || fail "project.json: $(cat "$p/project.json")"
grep -qx '# Sample site' "$p/AGENTS.md" && grep -qx '## Chief of Staff' "$p/AGENTS.md" && [ "$(grep -cv '^\(#.*\)\{0,1\}$' "$p/AGENTS.md")" = 0 ] \
  && pass "AGENTS.md は見出しだけ" || fail "AGENTS.md: $(cat "$p/AGENTS.md")"
[ "$(cat "$tmp/ghq.log")" = 'get example.com/o/new' ] && pass "ghq get は clone の無い repo だけ" || fail "ghq: $(cat "$tmp/ghq.log")"
[ "$(git -C "$ws" rev-parse HEAD)" = "$head0" ] && [ -z "$(git -C "$ws" diff --cached --name-only)" ] \
  && git -C "$ws" status --porcelain --untracked-files=all | grep -qx '?? projects/sample-site/project.json' \
  && ! git -C "$ws" status --porcelain --untracked-files=all | grep -q kanban/ \
  && pass "commit しない（project.json は未追跡、kanban/ は git の外）" || fail "git: $(git -C "$ws" status --porcelain --untracked-files=all)"
grep -q 'andon open sample-site' "$tmp/out" && grep -q 'git add projects/sample-site/project.json' "$tmp/out" \
  && pass "commit と andon open を案内する" || fail "案内: $(cat "$tmp/out")"
"$andon" delegate show --project sample-site 2>&1 | grep -q '無委譲' && pass "commit 前の andon delegate show は無委譲" || fail "delegate show"

# === 既にあれば何も変えずに止まる ===
sum() { (cd "$p" && find . -type f -print0 | sort -z | xargs -0 sha256sum); }
before=$(sum); : >"$tmp/ghq.log"
r=0; "$andon" create sample-site --repo example.com/o/other </dev/null >/dev/null 2>"$tmp/err" || r=$?
[ "$r" = 1 ] && grep -q '既にある' "$tmp/err" && [ "$(sum)" = "$before" ] && [ ! -s "$tmp/ghq.log" ] \
  && pass "既にあれば何も変えずに止まる（rc 1）" || fail "既にある: rc=$r $(cat "$tmp/err")"

# === 役の端末で拒む ===
r=0; HC_TERMINAL_ID=role-t1 "$andon" create sample-role </dev/null >/dev/null 2>"$tmp/err" || r=$?
[ "$r" = 1 ] && grep -q 'role=chief-of-staff' "$tmp/err" && [ ! -e "$ws/projects/sample-role" ] \
  && pass "役の端末では打てない" || fail "役の端末: rc=$r $(cat "$tmp/err")"

# === 端末でなければ聞かない: slug が無ければ止まり、slug だけなら既定 ===
r=0; "$andon" create --prefix X </dev/null >/dev/null 2>"$tmp/err" || r=$?
[ "$r" = 2 ] && grep -q 'slug が無い' "$tmp/err" && pass "端末でなく slug が無ければ止まる（rc 2）" || fail "slug 無し: rc=$r $(cat "$tmp/err")"
r=0; "$andon" create Bad_Slug </dev/null >/dev/null 2>"$tmp/err" || r=$?
[ "$r" = 2 ] && [ ! -e "$ws/projects/Bad_Slug" ] && pass "slug の形を確かめる" || fail "slug: rc=$r $(cat "$tmp/err")"
"$andon" create web-app </dev/null >/dev/null 2>"$tmp/err" \
  && [ "$(jq -c '[.project, .repos]' "$ws/projects/web-app/project.json")" = '[{"title":"web-app","prefix":"WA"},[]]' ] \
  && grep -qx 'prefix: WA' "$ws/projects/web-app/kanban/config.yaml" \
  && pass "slug だけなら既定（題は slug・prefix は頭文字・repos 無し）" || fail "既定: $(cat "$tmp/err" "$ws/projects/web-app/project.json" 2>&1)"

# === GHQ_ROOT の下に clone でないものがあれば止まる ===
mkdir -p "$GHQ_ROOT/example.com/o/empty"
r=0; "$andon" create sample-empty --repo example.com/o/empty </dev/null >/dev/null 2>"$tmp/err" || r=$?
[ "$r" = 1 ] && grep -q 'git の clone でない' "$tmp/err" && [ ! -e "$ws/projects/sample-empty" ] \
  && pass "clone でない置き場があれば何も作らずに止まる" || fail "clone でない: rc=$r $(cat "$tmp/err")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
