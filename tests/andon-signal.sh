#!/usr/bin/env bash
# `andon signal collect`・`auth`（ADR-0011 受け入れ条件 2・5・12）。一時のセルの repo・偽の gh・一時の HOME で回す。
# 記録の当て方の細部（count・active・episodes・落ちた出所）は internal/signal/signal_test.go。ここはバイナリの入口・置き場・終了コードを見る。
# ANDON_EMIT_DIR があれば信号の記録を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
command -v jq >/dev/null || { echo "skip: jq が無い"; exit 0; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
die() { echo "FAIL $*" >&2; exit 1; }
ok() { echo "ok $*"; }
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }
unset HC_TERMINAL_ID HC_TERMINAL_SOCKET XDG_CONFIG_HOME
export HOME=$tmp/home XDG_STATE_HOME=$tmp/state ANDON_WORKSPACE=$tmp/plant
mkdir -p "$HOME" "$tmp/bin"

# 偽の gh: 打たれた引数を残し、CI は失敗1件、issue は open の2件を返す（FAIL があれば落ちる）
cat >"$tmp/bin/gh" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"$GH_LOG"
[ -z "${GH_FAIL:-}" ] || { echo "HTTP 502" >&2; exit 1; }
case "$1" in
api) cat <<'JSON'
{"workflow_runs": [{"path": ".github/workflows/ci.yml", "name": "ci", "event": "push", "conclusion": "failure", "html_url": "https://github.com/example/app/actions/runs/7", "head_sha": "9c4e1b0d7a3f5e2c8b6d4a1f0e9c7b5a3d2e1f08", "display_title": "Fix export"}]}
JSON
;;
issue) echo '[{"number": 1, "title": "One", "url": "https://github.com/example/app/issues/1"}, {"number": 2, "title": "Two", "url": "https://github.com/example/app/issues/2"}]' ;;
*) exit 9 ;;
esac
EOF
chmod +x "$tmp/bin/gh"
export PATH="$tmp/bin:$PATH" GH_LOG=$tmp/gh.log
: >"$GH_LOG"

# セルの repo（人のもの）: owner が commit した project.json
plant() {
  echo "$1" >"$tmp/plant/projects/demo/project.json"
  git -C "$tmp/plant" add -A
  GIT_AUTHOR_NAME=owner GIT_AUTHOR_EMAIL=owner@example.com GIT_COMMITTER_NAME=owner GIT_COMMITTER_EMAIL=owner@example.com \
    git -C "$tmp/plant" commit -qm plant
}
git init -q "$tmp/plant"; mkdir -p "$tmp/plant/projects/demo"
printf 'owner = "owner@example.com"\n' >"$tmp/plant/andon.toml"
rec=$XDG_STATE_HOME/andon/dispatcher/demo.signals.json

# 受け入れ条件 5: signals の無い project.json では何も読まず、何も書かない
plant '{"version": 1, "project": {"title": "Demo", "prefix": "S"}, "repos": ["github.com/example/app"]}'
"$andon" signal collect --project demo >"$tmp/out" || die "signals が無いのに落ちた"
grep -q 'signals が無い' "$tmp/out" || die "1行が無い: $(cat "$tmp/out")"
[ ! -e "$rec" ] && [ ! -s "$GH_LOG" ] || die 'signals が無いのに読んだ・書いた'
ok 'collect: signals が無ければ gh を打たず、記録も書かない'

# signals 入りの正しい project.json（受け入れ条件 12: delegate show が通る。filter はそのまま検索へ）
plant '{"version": 1, "project": {"title": "Demo", "prefix": "S"}, "repos": ["github.com/example/app"],
 "signals": [{"source": "github-ci", "repo": "github.com/example/app", "weight": "now"},
  {"source": "github-issues", "weight": "later", "every": 30, "filter": "label:bug -author:<owner>"},
  {"source": "sentry", "weight": "later", "org": "example", "project": "app"}]}'
"$andon" delegate show --project demo --json | jq -e '.reason | contains("delegation が無い")' >/dev/null ||
  die "signals 入りの project.json を delegate show が読めない: $("$andon" delegate show --project demo --json)"
ok 'delegate show: signals 入りの project.json を読む（形の誤りに倒れない）'

# 受け入れ条件 2（入口）: projects/<slug>/ で打てば slug が分かる。CI の失敗1件と issue 2件
(cd "$tmp/plant/projects/demo" && "$andon" signal collect) >"$tmp/out" || die "collect が落ちた: $(cat "$tmp/out")"
jq -e '[.signals[] | select(.active)] | length == 3' "$rec" >/dev/null || die "3件でない: $(cat "$rec")"
jq -e '.signals[] | select(.key == "github.com/example/app/ci.yml@main") | .weight == "now" and .count == 1 and .episodes == 1 and .delivered_episode == 0' "$rec" >/dev/null || die "CI の記録: $(cat "$rec")"
grep -qxF 'issue list --repo github.com/example/app --state open --limit 1000 --json number,title,url --search label:bug -author:<owner>' "$GH_LOG" ||
  die "filter がそのまま渡らない: $(cat "$GH_LOG")"
grep -q 'sentry: トークンが無いので飛ばす（andon signal auth sentry）' "$tmp/out" || die "sentry の1行が無い: $(cat "$tmp/out")"
emit "$rec" signals.json
"$andon" signal collect --project demo >/dev/null
jq -e '(.signals | length) == 3 and all(.signals[]; .count == 2)' "$rec" >/dev/null || die "2回目: $(cat "$rec")"
ok 'collect: CI の失敗1件と issue 2件を書き、2回目は count だけ増える。filter はそのまま。トークンの無い出所は1行で飛ばす'

# 受け入れ条件 5: API が落ちたら書かず、終了コード 1
jq -S 'del(.read_at)' "$rec" >"$tmp/before"
sleep 1.1 # read_at は失敗でも秒の精度の now に進む。秒をまたいでも信号の記録は変わらない
GH_FAIL=1 "$andon" signal collect --project demo >"$tmp/out" && die "API が落ちたのに 0"
jq -S 'del(.read_at)' "$rec" | cmp -s - "$tmp/before" || die 'API が落ちたのに信号の記録が変わった'
grep -q '読めないので信号の記録は変えずに飛ばす' "$tmp/out" || die "1行が無い: $(cat "$tmp/out")"
ok 'collect: API が落ちたら信号の記録を変えず（read_at は進める）、終了コード 1'

# 受け入れ条件 12: auth が置いたファイルが mode 600。GitHub は置かない
printf 'tok-123\n' | "$andon" signal auth sentry 2>"$tmp/err" || die "auth が落ちた: $(cat "$tmp/err")"
[ "$(stat -c %a "$HOME/.config/andon/signals/sentry.token")" = 600 ] || die "mode が 600 でない"
[ "$(stat -c %a "$HOME/.config/andon/signals")" = 700 ] || die "dir の mode が 700 でない"
echo x | "$andon" signal auth github-ci 2>/dev/null && die 'GitHub のトークンを置いた'
"$andon" signal collect --project demo >"$tmp/out"
grep -q 'sentry: 読む command がまだ無いので飛ばす' "$tmp/out" || die "トークンがあるときの1行: $(cat "$tmp/out")"
ok 'auth: ~/.config/andon/signals/<source>.token を mode 600 で置く。GitHub は gh の資格情報'
