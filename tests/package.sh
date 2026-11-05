#!/usr/bin/env bash
# tests/package.sh — package/ の skill・プロファイル・ゲートの条件表・managed settings・statusline（design.md §2.2・§2.4）。
#   1. skill 7つとプロファイル 8つがそろい、frontmatter の name がディレクトリ名・ファイル名と同じ。
#   2. プロファイルの本文が指す skill が package/skills/ にある。
#   3. skill とプロファイルに仮設の語が残っていない（用語集の「仮設の語」の節だけは対応表として持つ）。
#   4. gate.json が schema/gate.schema.json の条件表に合う。settings.json が JSON で、statusLine が package の statusline を指す。
#   5. statusline が JSON を読んで1行を出す。
# 道具は bash・jq・python3（tests/lib/schema_check.py）。
set -u
cd "$(dirname "$0")/.." || exit 2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

skills="ask-one-by-one chief-engineer chief-of-staff glossary independent-review triage worker"
profiles="chief-engineer chief-of-staff claude claude-review codex codex-review fable sonnet"

# frontmatter の key の値（最初の --- から次の --- まで）
fm() { awk -v k="$2" 'NR==1 && $0!="---" {exit} NR>1 && $0=="---" {exit} NR>1 && index($0, k": ")==1 {print substr($0, length(k)+3); exit}' "$1"; }

# === 1. 顔ぶれと name ===
got=$(cd package/skills && ls -1 | sort | tr '\n' ' ' | sed 's/ $//')
[ "$got" = "$skills" ] && pass "package/skills は7つ（$skills）" || fail "package/skills の顔ぶれ: $got"
got=$(cd package/templates && ls -1 *.md | sed 's/\.md$//' | sort | tr '\n' ' ' | sed 's/ $//')
[ "$got" = "$profiles" ] && pass "package/templates は8つ（$profiles）" || fail "package/templates の顔ぶれ: $got"
for s in $skills; do
  f=package/skills/$s/SKILL.md
  [ "$(fm "$f" name)" = "$s" ] && [ -n "$(fm "$f" description)" ] \
    && pass "skill $s の frontmatter" || fail "skill $s の frontmatter（name=$(fm "$f" name)）"
done
for p in $profiles; do
  f=package/templates/$p.md
  [ "$(fm "$f" name)" = "$p" ] && [ -n "$(fm "$f" description)" ] && [ -n "$(fm "$f" command)" ] \
    && pass "プロファイル $p の frontmatter" || fail "プロファイル $p の frontmatter"
done

# === 2. プロファイルが指す skill ===
for p in $profiles; do
  refs=$(grep -o '/opt/andon/skills/[a-z-]*/SKILL\.md' "package/templates/$p.md" | sort -u)
  [ -n "$refs" ] || { fail "プロファイル $p が skill を指していない"; continue; }
  ok=1
  for r in $refs; do [ -f "package/${r#/opt/andon/}" ] || ok=0; done
  [ "$ok" = 1 ] && pass "プロファイル $p の skill がある（$(echo $refs | tr '\n' ' ')）" || fail "プロファイル $p が無い skill を指す: $refs"
done
grep -q '/skills/worker/SKILL.md' package/templates/codex-review.md && grep -q '§6' package/templates/codex-review.md \
  && pass "レビュアーのプロファイルは worker skill の §6 を指す" || fail "codex-review が worker §6 を指していない"

# === 3. 仮設の語 ===
# 用語集は「## 仮設の語」の節より前だけ、「_避ける_」の行を除いて見る。動詞の「知らせる」は語ではないので「の知らせ」だけを見る
for f in package/skills/*/SKILL.md package/skills/*/assets/* package/templates/*.md; do
  case $f in package/skills/glossary/SKILL.md) sed '/^## 仮設の語/,$d; /^_避ける_/d' "$f" ;; *) cat "$f" ;; esac >"$tmp/body"
  hits=$(grep -nE -i '\byard\b|yard-|template-yard|コーディネーター|担当エージェント|の知らせ|推し|段 ?[ABC]\b|merge-gate|/workspace/\.claude' "$tmp/body" || true)
  [ -z "$hits" ] && pass "$f に仮設の語が無い" || fail "$f に仮設の語: $(echo "$hits" | head -3)"
done
grep -q '^## 仮設の語' package/skills/glossary/SKILL.md && pass "用語集は仮設の語の対応表を末尾に持つ" || fail "用語集に仮設の語の節が無い"

# === 4. gate.json・settings.json ===
if python3 tests/lib/schema_check.py validate schema/gate.schema.json package/gate.json >"$tmp/why" 2>&1; then
  pass "package/gate.json は gate.schema.json に合う"
else fail "package/gate.json: $(head -1 "$tmp/why")"; fi
jq -e 'has("repos") | not' package/gate.json >/dev/null && pass "package/gate.json に repos が無い（建屋の gate.local.json へ）" || fail "package/gate.json に repos がある"
for p in package/skills/ package/templates/ package/gate.json package/settings.json; do
  jq -e --arg p "${p%/}" '.tier_c_paths | map(rtrimstr("/**")) | index($p)' package/gate.json >/dev/null \
    && pass "ラインの変更 $p はレベル C" || fail "$p が tier_c_paths に無い"
done
if jq -e . package/settings.json >/dev/null 2>&1; then pass "package/settings.json は JSON"; else fail "package/settings.json が JSON でない"; fi
jq -e '.permissions.deny | index("Bash(gh pr merge:*)")' package/settings.json >/dev/null && pass "settings は gh pr merge を deny" || fail "settings の deny に gh pr merge が無い"
sl=$(jq -r '.statusLine.command' package/settings.json)
[ "$sl" = /opt/andon/lib/andon/statusline ] && [ -x package/lib/andon/statusline ] \
  && pass "statusLine は package/lib/andon/statusline を指す" || fail "statusLine: $sl"

# === 5. statusline ===
out=$(printf '%s' '{"model":{"display_name":"Sample"},"context_window":{"used_percentage":42},"cwd":"/tmp"}' | bash package/lib/andon/statusline 2>"$tmp/err")
case $out in *Sample*42%*) [ ! -s "$tmp/err" ] && pass "statusline は model と context% を出す" || fail "statusline の stderr: $(cat "$tmp/err")" ;;
  *) fail "statusline の出力: $out" ;; esac
out=$(printf '' | bash package/lib/andon/statusline)
[ "$out" = Claude ] && pass "statusline は空の入力で Claude" || fail "statusline（空）: $out"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
