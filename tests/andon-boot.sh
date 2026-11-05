#!/usr/bin/env bash
# tests/andon-boot.sh — andon boot を偽の HOME・偽の建屋・偽のパッケージで2回打つ。
# 生成（managed settings・プロファイル・CLAUDE.md の2行）が揃い、2回目で何も変わらないこと。
# PATH には andon の置き場だけを置く（hc・portless・sshd を起こさない。本物の HOME と /workspace に触らない）。
set -u
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
. "$(dirname "$0")/lib/andon.sh"

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

pkg=$tmp/opt/andon ws=$tmp/workspace home=$tmp/home managed=$tmp/etc/claude-code/managed-settings.json
mkdir -p "$pkg/templates" "$pkg/skills/worker" "$ws/templates.local" "$home" "$tmp/path"
echo '{"permissions":{"deny":["Bash(gh pr merge:*)"]}}' >"$pkg/settings.json"
printf -- '---\nname: claude\n---\nパッケージ\n' >"$pkg/templates/claude.md"
printf -- '---\nname: claude\n---\n建屋\n' >"$ws/templates.local/claude.md"
echo worker >"$pkg/skills/worker/SKILL.md"
cp "$andon" "$tmp/path/andon"

boot() {
  env -i PATH="$tmp/path" HOME="$home" CLAUDE_CONFIG_DIR="$home/.claude" \
    ANDON_HOME="$pkg" ANDON_WORKSPACE="$ws" ANDON_MANAGED_SETTINGS="$managed" \
    andon boot >"$tmp/out" 2>"$tmp/err"
}
snap() { (cd "$tmp" && find etc home -printf '%y %p %l %m %T@\n' | sort && find etc home -type f -exec cat {} +); }

boot; r=$?
[ "$r" = 0 ] && grep -q 'ready' "$tmp/out" && [ ! -s "$tmp/err" ] && pass "andon boot は rc 0 で ready" || fail "boot: rc=$r $(cat "$tmp/out" "$tmp/err")"
grep -q 'gh pr merge' "$managed" && pass "managed settings を生成する" || fail "managed settings: $(cat "$managed" 2>&1)"
grep -q '建屋' "$home/.config/herdr-core/templates/claude.md" && pass "プロファイルを生成する（建屋の templates.local が勝つ）" || fail "プロファイル"
[ "$(cat "$home/.claude/CLAUDE.md")" = "@$pkg/environment.md
@$ws/AGENTS.local.md" ] && pass "CLAUDE.md は2行" || fail "CLAUDE.md: $(cat "$home/.claude/CLAUDE.md")"
[ "$(readlink "$home/.claude/skills/andon-worker")" = "$pkg/skills/worker" ] && pass "skill の symlink" || fail "skill"

snap >"$tmp/snap1"
boot; r=$?
snap >"$tmp/snap2"
[ "$r" = 0 ] && cmp -s "$tmp/snap1" "$tmp/snap2" && pass "2回打っても同じ（冪等）" || fail "2回目: rc=$r $(diff "$tmp/snap1" "$tmp/snap2")"
grep -q '生成:' "$tmp/out" && fail "2回目に書いた: $(cat "$tmp/out")" || pass "2回目は何も書かない"

"$andon" boot extra >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon boot' "$tmp/err" && pass "引数を取らない（usage で rc 2）" || fail "boot extra: rc=$r"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
