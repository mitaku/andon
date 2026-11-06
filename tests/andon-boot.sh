#!/usr/bin/env bash
# tests/andon-boot.sh — andon boot を偽の HOME・偽の建屋・偽のパッケージで2回打つ。
# 生成（managed settings・プロファイル・CLAUDE.md の2行・~/.agents/skills の symlink・信頼）が揃い、2回目で何も変わらないこと。
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
cp "$(dirname "$0")/../package/codex.rules" "$pkg/codex.rules"
printf -- '---\nname: claude\n---\nパッケージ\n' >"$pkg/templates/claude.md"
printf -- '---\nname: codex\n---\n`/opt/andon/skills/worker/SKILL.md` を読む\n' >"$pkg/templates/codex.md"
printf -- '---\nname: claude\n---\n建屋\n' >"$ws/templates.local/claude.md"
echo worker >"$pkg/skills/worker/SKILL.md"
mkdir -p "$home/.claude/skills"
ln -s "$pkg/skills/worker" "$home/.claude/skills/andon-worker" # 前の置き場（外れること）
ln -s "$tmp/mine" "$home/.claude/skills/mine"                   # 利用者の skill（残ること）
mkdir -p "$home/.agents/skills"
ln -s "$tmp/mine" "$home/.agents/skills/andon-mine"             # 利用者が andon- の名前で置いた skill（残ること）
mkdir -p "$ws/.ghq/github.com/example/sample-site/.git" "$ws/.ghq/github.com/example/unlisted/.git" "$ws/projects/demo/kanban"
printf 'version: 2\nrepos:\n  - github.com/example/sample-site\n' >"$ws/projects/demo/kanban/config.yaml" # 台帳に載った repo だけが信頼の根
cp "$andon" "$tmp/path/andon"

boot() {
  env -i PATH="$tmp/path" HOME="$home" CLAUDE_CONFIG_DIR="$home/.claude" \
    ANDON_HOME="$pkg" ANDON_WORKSPACE="$ws" ANDON_MANAGED_SETTINGS="$managed" GHQ_ROOT="$ws/.ghq" \
    andon boot >"$tmp/out" 2>"$tmp/err"
}
snap() { (cd "$tmp" && find etc home -printf '%y %p %l %m %T@\n' | sort && find etc home -type f -exec cat {} +); }

boot; r=$?
[ "$r" = 0 ] && grep -q 'ready' "$tmp/out" && [ ! -s "$tmp/err" ] && pass "andon boot は rc 0 で ready" || fail "boot: rc=$r $(cat "$tmp/out" "$tmp/err")"
grep -q 'gh pr merge' "$managed" && pass "managed settings を生成する" || fail "managed settings: $(cat "$managed" 2>&1)"
grep -q '建屋' "$home/.config/herdr-core/templates/claude.md" && pass "プロファイルを生成する（建屋の templates.local が勝つ）" || fail "プロファイル"
[ "$(cat "$home/.claude/CLAUDE.md")" = "@$pkg/environment.md
@$ws/AGENTS.local.md" ] && pass "CLAUDE.md は2行" || fail "CLAUDE.md: $(cat "$home/.claude/CLAUDE.md")"
[ "$(readlink "$home/.agents/skills/andon-worker")" = "$pkg/skills/worker" ] && pass "skill の symlink（~/.agents/skills）" || fail "skill"
[ ! -L "$home/.claude/skills/andon-worker" ] && [ -L "$home/.claude/skills/mine" ] && [ "$(readlink "$home/.agents/skills/andon-mine")" = "$tmp/mine" ] && pass "boot が張った andon-* だけ外す" || fail "~/.claude/skills: $(ls -l "$home/.claude/skills")"
grep -qF "\`$pkg/skills/worker/SKILL.md\`" "$home/.config/herdr-core/templates/codex.md" && pass "プロファイルの /opt/andon を ANDON_HOME に置き換える" || fail "置換: $(cat "$home/.config/herdr-core/templates/codex.md")"
# Codex の承認・sandbox の既定は config.toml の先頭の塊（最初の空行まで）。信頼の表はその後ろに続く
[ "$(sed -n '2,3p' "$home/.codex/config.toml")" = 'approval_policy = "never"
sandbox_mode = "danger-full-access"' ] && cmp -s "$pkg/codex.rules" "$home/.codex/rules/andon.rules" \
  && pass "Codex の承認・sandbox の既定と rules を置く" || fail "Codex の既定: $(cat "$home/.codex/config.toml")"
repo=$ws/.ghq/github.com/example/sample-site
jq -e --arg w "$ws" --arg r "$repo" '.projects[$w].hasTrustDialogAccepted == true and .projects[$r].hasTrustDialogAccepted == true and (.projects | length) == 2' "$home/.claude/.claude.json" >/dev/null \
  && [ "$(sed '1,/^$/d' "$home/.codex/config.toml")" = "[projects.\"$ws\"]
trust_level = \"trusted\"

[projects.\"$repo\"]
trust_level = \"trusted\"" ] && pass "建屋の repo と ghq の repo を Claude Code・Codex で信頼済みにする" || fail "信頼: $(cat "$home/.claude/.claude.json" "$home/.codex/config.toml" 2>&1)"

snap >"$tmp/snap1"
boot; r=$?
snap >"$tmp/snap2"
[ "$r" = 0 ] && cmp -s "$tmp/snap1" "$tmp/snap2" && pass "2回打っても同じ（冪等）" || fail "2回目: rc=$r $(diff "$tmp/snap1" "$tmp/snap2")"
grep -qE '生成:|リンク:|外した:|信頼を足した' "$tmp/out" && fail "2回目に書いた: $(cat "$tmp/out")" || pass "2回目は何も書かない"

"$andon" boot extra >/dev/null 2>"$tmp/err"; r=$?
[ "$r" = 2 ] && grep -q '^usage: andon boot' "$tmp/err" && pass "引数を取らない（usage で rc 2）" || fail "boot extra: rc=$r"

mkdir -p "$tmp/sp ace" && cp -r "$pkg" "$tmp/sp ace/"
env -i PATH="$tmp/path" HOME="$home" CLAUDE_CONFIG_DIR="$home/.claude" \
  ANDON_HOME="$tmp/sp ace/andon" ANDON_WORKSPACE="$ws" ANDON_MANAGED_SETTINGS="$managed" \
  andon boot >"$tmp/out" 2>"$tmp/err"; r=$?
[ "$r" = 1 ] && grep -q 'command で割れる' "$tmp/err" && grep -qF "\`$pkg/skills/worker/SKILL.md\`" "$home/.config/herdr-core/templates/codex.md" \
  && pass "空白を含む ANDON_HOME は置換せず rc 1（プロファイルを残す）" || fail "空白: rc=$r $(cat "$tmp/err")"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
