#!/usr/bin/env bash
# dispatcher の巡の外の信号の段（docs/dispatcher.md §2.1.1 の 4e。ADR-0011 受け入れ条件 3・9・10）。本物の台帳の CLI・一時のセルの repo・
# 偽の hc・偽の gh で `andon dispatcher --once` を回し、控えの待ち（pending）と信号の記録を見る。<news> の行の囲いは
# internal/dispatcher/signals_test.go、collect の当て方と every の数え方は internal/signal/signal_test.go。
# ANDON_EMIT_DIR があれば、積んだ通知 signal と信号の記録を写す（tests/schema.sh が schema に通す）。
set -euo pipefail
for c in git jq; do command -v "$c" >/dev/null || { echo "skip: $c が無い"; exit 0; }; done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"
. "$(dirname "$0")/lib/kanban.sh"
export HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/config" XDG_STATE_HOME="$tmp/state" GIT_CONFIG_GLOBAL="$tmp/gitconfig" GIT_CONFIG_NOSYSTEM=1
export GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@example.com GIT_AUTHOR_NAME=o GIT_AUTHOR_EMAIL=owner@example.com
export ANDON_HOME="$andon_root/package" ANDON_WORKSPACE="$tmp/plant"
unset KANBAN_ROOT HC_TERMINAL_ID
mkdir -p "$HOME" "$tmp/bin" "$tmp/gh" "$tmp/hc"
pass=0
ok() { echo "ok   $1"; pass=$((pass + 1)); }
die() { echo "FAIL $1" >&2; exit 1; }
emit() { [ -z "${ANDON_EMIT_DIR:-}" ] || cp "$1" "$ANDON_EMIT_DIR/$2"; }

# 偽の gh: 打った引数を calls に。main の CI は $tmp/gh/runs-<repo>.json（無ければ run なし）、issue は open の1件
cat >"$tmp/bin/gh" <<EOF
#!/bin/sh
d=$tmp/gh
echo "\$*" >> \$d/calls
case "\$*" in
  "api --hostname github.com repos/example/"*"/actions/runs?branch=main&status=completed&per_page=100")
    r=\$(echo "\$4" | cut -d/ -f3); cat \$d/runs-\$r.json 2>/dev/null || echo '{"workflow_runs": []}' ;;
  "issue list --repo github.com/example/app "*) echo '[{"number": 3, "title": "Later issue", "url": "https://github.com/example/app/issues/3"}]' ;;
  *) echo "unexpected gh \$*" >&2; exit 99 ;;
esac
EOF
# 偽の hc: 生きている端末は $tmp/hc/live。ほかは何もしない
cat >"$tmp/bin/hc" <<EOF
#!/bin/sh
d=$tmp/hc
echo "\$*" >> \$d/log
case \$1 in
  get) grep -q "^\$2\$" \$d/live || { echo '{"error":{"kind":"not_found","message":"no terminal"}}' >&2; exit 3; }
       [ "\${3:-}" != --json ] || echo "{\"address\":\"\$2\",\"state\":\"live\",\"status\":\"idle\"}" ;;
  ls) echo '{"terminals":[]}' ;;
  *) ;;
esac
EOF
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH"
printf 't5\ndispatcher\n' >"$tmp/hc/live"
: >"$tmp/gh/calls"

# main の CI の結果（workflow ごとに最新の1件）
runs() { # runs <repo> <workflow:conclusion>...
  local r=$1; shift
  printf '%s\n' "$@" | jq -R --arg r "$r" 'split(":") | {path: (".github/workflows/" + .[0]), name: (.[0] | rtrimstr(".yml")), event: "push",
    conclusion: .[1], html_url: ("https://github.com/example/" + $r + "/actions/runs/" + (.[0] | length | tostring)), display_title: "main に <b>入れた</b>"}' |
    jq -s '{workflow_runs: .}' >"$tmp/gh/runs-$r.json"
}

# セルの repo（人のもの）: owner が commit した project.json（main の CI は now、issue は later）
git init -q "$tmp/plant"
printf 'owner = "owner@example.com"\n' >"$tmp/plant/andon.toml"
mkdir -p "$tmp/plant/projects/demo"
cat >"$tmp/plant/projects/demo/project.json" <<'EOF'
{"version": 1, "project": {"title": "Demo", "prefix": "D"}, "repos": ["github.com/example/app", "github.com/example/web"],
 "signals": [{"source": "github-ci", "weight": "now"},
             {"source": "github-issues", "repo": "github.com/example/app", "weight": "later"}]}
EOF
git -C "$tmp/plant" add -A; git -C "$tmp/plant" commit -qm plant
cd "$tmp/plant/projects/demo"
kanban init --prefix D --repo github.com/example/app >/dev/null
export KANBAN_ROOT=$PWD
kanban config set chief_of_staff=t5 >/dev/null

dispatcher() { "$andon" dispatcher --once --project demo 2>>"$tmp/dispatcher.log" || die "dispatcher が落ちた: $(tail -3 "$tmp/dispatcher.log")"; }
state=$tmp/state/andon/dispatcher/demo.json
rec=$tmp/state/andon/dispatcher/demo.signals.json
signals() { jq -c '[.pending[] | select(.kind == "signal") | .detail.key]' "$state"; }
drained() { jq '.pending = []' "$state" >"$tmp/s" && mv "$tmp/s" "$state"; }   # Chief of Staff のフックが取り出した
later() { # every 分たったことにする（read_at を 11 分前に）
  jq '.read_at |= map_values((fromdateiso8601 - 660) | todateiso8601)' "$rec" >"$tmp/r" && mv "$tmp/r" "$rec"
}
calls() { wc -l <"$tmp/gh/calls" | tr -d ' '; }
ep() { jq -c --arg k "$1" '.signals[] | select(.key == $k) | [.active, .episodes, .delivered_episode, .count]' "$rec"; }
app=github.com/example/app/ci.yml@main

# 受け入れ条件 3・10: 初めて見る台帳（控えが無い）の巡でも、届けていない now の信号を積む。later は積まない
runs app ci.yml:failure
dispatcher
[ "$(signals)" = "[\"$app\"]" ] || die "初めての巡に now の信号が積まれない: $(jq -c .pending "$state")"
jq -e --arg k "$app" '.pending[] | select(.kind == "signal") | .issue == null and .detail == {source: "github-ci", key: $k,
  url: "https://github.com/example/app/actions/runs/6", title: "ci: main に <b>入れた</b>", weight: "now"}' "$state" >/dev/null ||
  die "通知の形: $(jq -c .pending "$state")"
[ "$(ep "$app")" = '[true,1,1,1]' ] || die "delivered_episode を書いていない: $(ep "$app")"
[ "$(ep github.com/example/app#3)" = '[true,1,0,1]' ] || die "later を届けた: $(ep github.com/example/app#3)"
[ "$(calls)" = 3 ] || die "1巡目の gh: $(cat "$tmp/gh/calls")"   # CI は app・web、issue は app
jq '.pending[] | select(.kind == "signal")' "$state" >"$tmp/n.json"; emit "$tmp/n.json" notification-signal.json
ok '初めて見る台帳の巡でも、届けていない now の信号を通知 signal で積み（detail は記録の写し）、delivered_episode を書く。later は積まない'

# 受け入れ条件 9: every より短い巡では読みに行かない。届けた出来事は積み直さない
drained
dispatcher; dispatcher
[ "$(calls)" = 3 ] || die "every の前に読んだ: $(cat "$tmp/gh/calls")"
[ "$(signals)" = '[]' ] || die "届けた信号を積み直した: $(signals)"
ok 'every（既定 10 分）より短い巡では gh を打たず、届けた信号を積み直さない'

# 受け入れ条件 3: count が増えた巡では積まない。消えて（CI が通って）また出たら（episodes が 1 増えて）積む
later; dispatcher
[ "$(calls)" = 6 ] && [ "$(ep "$app")" = '[true,1,1,2]' ] && [ "$(signals)" = '[]' ] || die "count が増えた巡: $(ep "$app") $(signals)"
runs app ci.yml:success; later; dispatcher
[ "$(ep "$app")" = '[false,1,1,2]' ] && [ "$(signals)" = '[]' ] || die "CI が通った巡: $(ep "$app") $(signals)"
runs app ci.yml:failure; later; dispatcher
[ "$(ep "$app")" = '[true,2,2,3]' ] && [ "$(signals)" = "[\"$app\"]" ] || die "また失敗した巡: $(ep "$app") $(signals)"
ok 'count が増えただけの巡では積まず、CI が通ってまた失敗した巡（episodes が 1 増えた）で積み直す'

# 受け入れ条件 10: 同じ巡に now の信号が2件（別の workflow・別の repo）出たら2件とも積む
drained
runs app ci.yml:failure lint.yml:failure
runs web ci.yml:failure
later; dispatcher
[ "$(signals)" = '["github.com/example/app/lint.yml@main","github.com/example/web/ci.yml@main"]' ] || die "2件とも積まない: $(signals)"
ok '同じ巡に出た別の workflow・別の repo の now の信号を、2件とも積む'

# 受け入れ条件 10: 控えを失った巡でも、届けていない now の信号を届ける（届けたかは記録で見る）
rm -f "$state"
runs web ci.yml:failure lint.yml:failure
later; dispatcher
[ "$(signals)" = '["github.com/example/web/lint.yml@main"]' ] || die "控えを失った巡: $(signals)"
emit "$rec" signals-delivered.json
ok '控えを失った巡でも、届けていない now の信号だけを積む（届けた信号は積み直さない）'

# andon signal delivered を打つのは dispatcher だけ（HC_TERMINAL_ID=dispatcher）。担当の端末・未設定なら断り、記録を変えない
web=github.com/example/web/lint.yml@main
jq --arg k "$web" '(.signals[] | select(.key == $k) | .delivered_episode) = 0' "$rec" >"$tmp/r" && mv "$tmp/r" "$rec"
fp=$(jq -r --arg k "$web" '.signals[] | select(.key == $k) | .fingerprint' "$rec")
cp "$rec" "$tmp/before"
for who in t7 ''; do
  r=0; HC_TERMINAL_ID=$who "$andon" signal delivered --project demo "$fp" 1 2>"$tmp/err" || r=$?
  [ "$r" = 2 ] && grep -q '打つのは dispatcher だけ' "$tmp/err" && cmp -s "$rec" "$tmp/before" || die "dispatcher でない（$who）のに通った: rc=$r $(cat "$tmp/err")"
done
HC_TERMINAL_ID=dispatcher "$andon" signal delivered --project demo "$fp" 1 || die 'dispatcher が打てない'
[ "$(ep "$web")" = '[true,1,1,1]' ] || die "dispatcher が打っても書かない: $(ep "$web")"
ok 'andon signal delivered は dispatcher（HC_TERMINAL_ID=dispatcher）以外から打つと断り、記録を変えない'

# gh の上限でゲートを止めている間（控えの gh_reset）は collect を打たない
drained
jq --arg t "$(date -u -d '+30 min' +%Y-%m-%dT%H:%M:%SZ)" '.gh_reset = $t' "$state" >"$tmp/s" && mv "$tmp/s" "$state"
n=$(calls); later; dispatcher
[ "$(calls)" = "$n" ] || die "gh の上限の間に読んだ: $(tail -2 "$tmp/gh/calls")"
ok 'gh の上限が戻るまで（gh_reset）は collect を打たない'

# signals の無い project.json では gh を打たず、記録も書かない
jq 'del(.signals)' "$tmp/plant/projects/demo/project.json" >"$tmp/p" && cp "$tmp/p" "$tmp/plant/projects/demo/project.json"
git -C "$tmp/plant" commit -qam 'no signals'
rm -f "$rec" "$state"; n=$(calls); dispatcher
[ "$(calls)" = "$n" ] && [ ! -e "$rec" ] || die "signals が無いのに読んだ・書いた"
ok 'signals の無い project.json では gh を打たず、信号の記録も書かない'

echo "andon-dispatcher-signal: $pass ok"
