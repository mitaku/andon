#!/usr/bin/env bash
# tests/package.sh — package/ の skill・プロファイル・ゲートの条件表・managed settings・statusline（design.md §2.2・§2.4）。
#   1. skill 7つとプロファイル 8つがそろい、frontmatter の name がディレクトリ名・ファイル名と同じ。
#   2. plugin.json の name が andon。Claude Code 系のプロファイルは command で plugin と役の skill を運び（本文で読ませない）、Codex 系は本文で skill を指す。
#   3. skill とプロファイルに仮設の語が残っていない（用語集の「仮設の語」の節だけは対応表として持つ）。
#   4. gate.json が schema/gate.schema.json の条件表に合う。settings.json が JSON で、statusLine が package の statusline を指す。
#   5. statusline が JSON を読んで1行を出す。
#   6. skill の機械向けの手順が command（andon task …）を指し、手で JSON を書く手順が残っていない。例の問いと予約キーは schema に通す。
# 道具は bash・jq・python3（package/lib/andon/schema_check.py）。
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

# === 2. plugin とプロファイルが指す skill（docs/adr/0001-skills-distribution.md 決定 2・3） ===
jq -e '.name == "andon"' package/.claude-plugin/plugin.json >/dev/null 2>&1 \
  && pass "package/.claude-plugin/plugin.json の name は andon" || fail "package/.claude-plugin/plugin.json が無いか name が andon でない"
# Claude Code 系: command が plugin と役の skill を運び、本文は skill を読ませない。役の skill は package/skills/ にある
role() { case $1 in chief-engineer|chief-of-staff) echo "$1" ;; *) echo worker ;; esac; }
for p in chief-engineer chief-of-staff claude sonnet fable claude-review; do
  f=package/templates/$p.md r=$(role "$p")
  cmd=" $(fm "$f" command) "
  body=$(awk 'n >= 2; /^---$/ && n < 2 {n++}' "$f")
  case $cmd in *" --plugin-dir /opt/andon "*) ;; *) fail "プロファイル $p の command に --plugin-dir /opt/andon が無い"; continue ;; esac
  case $cmd in *" --append-system-prompt-file /opt/andon/skills/$r/SKILL.md "*) ;; *) fail "プロファイル $p の command に --append-system-prompt-file /opt/andon/skills/$r/SKILL.md が無い"; continue ;; esac
  [ -f "package/skills/$r/SKILL.md" ] || { fail "プロファイル $p の skill $r が package/skills に無い"; continue; }
  if grep -qE 'SKILL\.md|読む|読み' <<<"$body"; then fail "プロファイル $p の本文に「先に読む」の行が残る: $(grep -E 'SKILL\.md|読む|読み' <<<"$body" | head -1)"
  else pass "プロファイル $p は command で plugin と skill $r を運ぶ（本文で読ませない）"; fi
done
# Codex 系: 本文で役の skill を絶対パスで指す（今のまま）
for p in codex codex-review; do
  f=package/templates/$p.md
  case " $(fm "$f" command) " in *--plugin-dir*|*--append-system-prompt-file*) fail "プロファイル $p（Codex）の command に Claude Code の口がある"; continue ;; esac
  grep -q '`/opt/andon/skills/worker/SKILL.md` を最初に読み' "$f" && [ -f package/skills/worker/SKILL.md ] \
    && pass "プロファイル $p（Codex）は本文で worker skill を指す" || fail "プロファイル $p（Codex）が本文で worker skill を指していない"
done
grep -q '§6' package/templates/codex-review.md && grep -q '§6' package/templates/claude-review.md \
  && pass "レビュアーのプロファイルは worker skill の §6 を指す" || fail "レビュアーのプロファイルが worker §6 を指していない"

# === 3. 仮設の語 ===
# 用語集は「## 仮設の語」の節より前だけ、「_避ける_」の行を除いて見る。動詞の「知らせる」は語ではないので「の知らせ」だけを見る
for f in package/skills/*/SKILL.md package/skills/*/assets/* package/templates/*.md; do
  case $f in package/skills/glossary/SKILL.md) sed '/^## 仮設の語/,$d; /^_避ける_/d' "$f" ;; *) cat "$f" ;; esac >"$tmp/body"
  hits=$(grep -nE -i '\byard\b|yard-|template-yard|コーディネーター|担当エージェント|の知らせ|推し|段 ?[ABC]\b|merge-gate|/workspace/\.claude' "$tmp/body" || true)
  [ -z "$hits" ] && pass "$f に仮設の語が無い" || fail "$f に仮設の語: $(echo "$hits" | head -3)"
done
grep -q '^## 仮設の語' package/skills/glossary/SKILL.md && pass "用語集は仮設の語の対応表を末尾に持つ" || fail "用語集に仮設の語の節が無い"

# === 4. gate.json・settings.json ===
if python3 package/lib/andon/schema_check.py validate schema/gate.schema.json package/gate.json >"$tmp/why" 2>&1; then
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
for ev in Stop:stop UserPromptSubmit:user-prompt-submit; do
  jq -e --arg e "${ev%%:*}" --arg c "/opt/andon/hooks/${ev#*:}" '.hooks[$e] | any(.[].hooks[]; .command == $c)' package/settings.json >/dev/null \
    && [ -x "package/hooks/${ev#*:}" ] && sh -n "package/hooks/${ev#*:}" \
    && pass "${ev%%:*} フックは package/hooks/${ev#*:} を指す" || fail "${ev%%:*} フック: package/hooks/${ev#*:}"
done

# === 5. statusline ===
out=$(printf '%s' '{"model":{"display_name":"Sample"},"context_window":{"used_percentage":42},"cwd":"/tmp"}' | bash package/lib/andon/statusline 2>"$tmp/err")
case $out in *Sample*42%*) [ ! -s "$tmp/err" ] && pass "statusline は model と context% を出す" || fail "statusline の stderr: $(cat "$tmp/err")" ;;
  *) fail "statusline の出力: $out" ;; esac
out=$(printf '' | bash package/lib/andon/statusline)
[ "$out" = Claude ] && pass "statusline は空の入力で Claude" || fail "statusline（空）: $out"

# === 6. skill の機械向けの手順（command を指す。skill の中の ``` の塊の例は schema に通す） ===
# block <file> <1行目の頭> … 1行目が「<頭>」で始まる ``` の塊（字下げを除く）
block() { awk -v h="$2" '
  /^ *```/ { if (inb) { exit } ; fence = 1; next }
  fence && !inb { fence = 0; t = $0; sub(/^ +/, "", t); if (index(t, h) == 1) { inb = 1; ind = match($0, /[^ ]/) - 1 } else next }
  inb { print substr($0, ind + 1) }' "$1"; }
export ANDON_HOME=$PWD/package ANDON_SCHEMA=$PWD/schema
chk() { python3 package/lib/andon/schema_check.py validate "schema/$1.schema.json" "$2" >"$tmp/why" 2>&1; }

# 担当の機械向けの手順は command（andon task …）。skill に JSON を手で書く手順・一致を手で確かめる手順が残っていない
w=package/skills/worker/SKILL.md
for c in 'andon task report' 'andon task review-request' 'andon task ready' 'andon task ask' 'andon task show'; do
  grep -q "$c" "$w" && pass "worker skill は $c を使う" || fail "worker skill に $c が無い"
done
left=$(grep -nE 'schema_check|report\.json"? *$|> "\$lib/report\.json"|questions/\$\{n|一致を確かめる|headRefOid' "$w" || true)
[ -z "$left" ] && pass "worker skill に report.json・問いの JSON を手で書く手順と、一致を手で確かめる手順が無い" || fail "手の手順が残る: $left"
# skill の問いの例（andon task ask の stdin）は question.schema.json に合う
q_blk=$(block "$w" 'andon task ask "$id" -' | sed '1d;$d')
printf '%s\n' "$q_blk" >"$tmp/q.json"
[ -n "$q_blk" ] && chk question "$tmp/q.json" && pass "worker skill の問いの例は question.schema.json に合う" || fail "問いの例: $(head -1 "$tmp/why" 2>/dev/null) $q_blk"

# Chief of Staff の task.yaml の予約キー: 全部書き、task-keys.schema.json に合う
set_line=$(block package/skills/chief-of-staff/SKILL.md '# task.yaml の予約キー' | grep '^deck set ')
printf '%s\n' "$set_line" | tr ' ' '\n' | grep '=' | jq -R 'split("=") | {(.[0]): .[1]}' | jq -s add >"$tmp/keys.json"
if chk task-keys "$tmp/keys.json" && jq -e 'keys == ["merge","must_fix","review","review_rounds","reviewer"]' "$tmp/keys.json" >/dev/null; then
  pass "Chief of Staff は予約キー5つ（review・must_fix・reviewer・review_rounds・merge）を task.yaml に書く"
else fail "Chief of Staff の deck set: $set_line"; fi
grep -q '`auto` は\*\*委譲がこのプロジェクトのマージを任せているときだけ\*\*' package/skills/chief-of-staff/SKILL.md \
  && pass "merge: auto は委譲の範囲内だけ" || fail "chief-of-staff に merge: auto の条件が無い"
# インナーループはクーリエ（P6a）: Chief of Staff の skill のレビューとゲートの節は人の判断が要るときだけ。レビュアーを手で起こす手順が残っていない
cos6=$(sed -n '/^## 6\. /,/^## 7\. /p' package/skills/chief-of-staff/SKILL.md)
grep -q '^## 6\. レビューとゲート（人の判断が要るときだけ）' <<<"$cos6" && grep -q '^### `needs_human` のとき' <<<"$cos6" \
  && ! grep -qE 'andon task start "\$new"|worktree add --detach|andon gate <id>' <<<"$cos6" \
  && ! grep -qE 'worktree add --detach|andon task start "\$new"' package/skills/independent-review/SKILL.md \
  && pass "Chief of Staff の §6 は needs_human のときだけ。レビュアーを起こすのとゲートはクーリエ" || fail "chief-of-staff・independent-review にレビュアーを手で起こす手順・ゲートを手で回す手順が残る"
grep -q '元のタスク（`task.yaml` の `parent`）の担当だけ' package/skills/worker/SKILL.md && grep -q '結果の宛先は元のタスクの担当だけ' package/skills/independent-review/SKILL.md \
  && pass "レビュアーの結果の宛先は parent のタスクの担当だけ（Chief of Staff へ送らない）" || fail "レビュアーの宛先が parent のタスクの担当に固定されていない"

# Chief Engineer: worktree で文書の PR と report.json を同じタスクから返す。問いは問いの JSON
ce=package/skills/chief-engineer/SKILL.md
grep -q 'cwd は案件の repo の自分の worktree' "$ce" && grep -q '`pr`＝文書の PR' "$ce" && ! grep -q 'tasks/<id>/library/<題' "$ce" \
  && pass "Chief Engineer は worktree で文書の PR と report.json を返す" || fail "chief-engineer の渡し方"
grep -q 'worker skill §5 の1の手順で1問ずつ問いの JSON' "$ce" && grep -q 'andon task ask "$id" -' package/skills/ask-one-by-one/SKILL.md \
  && pass "Chief Engineer と Chief of Staff の問いは問いの JSON の手順を使う" || fail "問いの JSON の手順を指していない"
left=$(grep -nE 'deck ask [^-]*"<|deck set <id> pr=' package/skills/*/SKILL.md || true)
[ -z "$left" ] && pass "skill に JSON を通らない deck ask と task.yaml の pr が残っていない" || fail "JSON を通らない書き方が残る: $left"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
