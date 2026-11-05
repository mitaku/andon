#!/usr/bin/env bash
# tests/package.sh — package/ の skill・プロファイル・ゲートの条件表・managed settings・statusline（design.md §2.2・§2.4）。
#   1. skill 7つとプロファイル 8つがそろい、frontmatter の name がディレクトリ名・ファイル名と同じ。
#   2. プロファイルの本文が指す skill が package/skills/ にある。
#   3. skill とプロファイルに仮設の語が残っていない（用語集の「仮設の語」の節だけは対応表として持つ）。
#   4. gate.json が schema/gate.schema.json の条件表に合う。settings.json が JSON で、statusLine が package の statusline を指す。
#   5. statusline が JSON を読んで1行を出す。
#   6. skill に書いた機械向けの手順（report.json・問いの JSON・task.yaml の予約キー）をそのまま回し、schema に通す。
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

# === 5. statusline ===
out=$(printf '%s' '{"model":{"display_name":"Sample"},"context_window":{"used_percentage":42},"cwd":"/tmp"}' | bash package/lib/andon/statusline 2>"$tmp/err")
case $out in *Sample*42%*) [ ! -s "$tmp/err" ] && pass "statusline は model と context% を出す" || fail "statusline の stderr: $(cat "$tmp/err")" ;;
  *) fail "statusline の出力: $out" ;; esac
out=$(printf '' | bash package/lib/andon/statusline)
[ "$out" = Claude ] && pass "statusline は空の入力で Claude" || fail "statusline（空）: $out"

# === 6. skill の機械向けの手順（skill の中の ``` の塊をそのまま抜いて回し、schema に通す） ===
# block <file> <1行目の頭> … 1行目が「<頭>」で始まる ``` の塊（字下げを除く）
block() { awk -v h="$2" '
  /^ *```/ { if (inb) { exit } ; fence = 1; next }
  fence && !inb { fence = 0; t = $0; sub(/^ +/, "", t); if (index(t, h) == 1) { inb = 1; ind = match($0, /[^ ]/) - 1 } else next }
  inb { print substr($0, ind + 1) }' "$1"; }
export ANDON_HOME=$PWD/package ANDON_SCHEMA=$PWD/schema
chk() { python3 package/lib/andon/schema_check.py validate "schema/$1.schema.json" "$2" >"$tmp/why" 2>&1; }

# 偽の deck: root は $tmp/proj、ask は stdin と引数を控えて <id>#1
mkdir -p "$tmp/bin" "$tmp/proj" "$tmp/git"
cat >"$tmp/bin/deck" <<'DECK'
#!/usr/bin/env bash
case $1 in
  root) echo "$FAKE_ROOT" ;;
  ask) printf '%s\n' "$*" >"$FAKE_ROOT/ask.args"; cat >"$FAKE_ROOT/ask.text"; echo "$2#1" ;;
  *) exit 9 ;;
esac
DECK
chmod +x "$tmp/bin/deck"
git -C "$tmp/git" init -q && git -C "$tmp/git" -c user.name=t -c user.email=t@example.com commit -q --allow-empty -m x
head_sha=$(git -C "$tmp/git" rev-parse HEAD)

rr_blk=$(block package/skills/worker/SKILL.md '# review_request')
rep_blk=$(block package/skills/worker/SKILL.md '# report.json')
q_blk=$(block package/skills/worker/SKILL.md '# 問い')
[ -n "$rr_blk" ] && [ -n "$rep_blk" ] && [ -n "$q_blk" ] && pass "worker skill に review_request・report.json・問いの手順がある" || fail "worker skill の手順の塊が見つからない"

runblk() { # runblk <説明> <前置き> <塊…> — $tmp/git で、偽の deck を先に置いて回す
  local desc=$1 pre=$2; shift 2
  (cd "$tmp/git" && PATH="$tmp/bin:$PATH" FAKE_ROOT="$tmp/proj" bash -euc "$pre"$'\n'"$(printf '%s\n' "$@")") >"$tmp/run" 2>&1 \
    || { fail "$desc（回らない: $(head -3 "$tmp/run")）"; return 1; }
}
lib=$tmp/proj/.flightdeck/tasks/S-1/library
if runblk "report.json（PR・レビュー依頼・見送り・次）" 'id=S-1 pr=https://github.com/example/sample-site/pull/3
waived='"'"'[{"severity":"minor","file":"a.go","line":2,"reason":"起きない"}]'"'"' next='"'"'[{"to":"chief-of-staff","text":"マージ後に finish"}]'"'"'' "$rr_blk" "$rep_blk"; then
  if chk report "$lib/report.json" && jq -e --arg h "$head_sha" '.commit == $h and .pr != null and .review_request.round == 1 and .review_request.limit == 2 and .review_request.previous == null and .review_request.commit == $h and .review_request.extra == false and (.waived | length) == 1 and (.next[0].to == "chief-of-staff") and (has("remember") | not)' "$lib/report.json" >/dev/null
  then pass "worker の手順の report.json（PR・review_request・waived・next）が report.schema.json に合う"
  else fail "report.json: $(head -1 "$tmp/why") $(cat "$lib/report.json")"; fi
fi
if runblk "report.json（追加の回）" 'id=S-1 pr=https://github.com/example/sample-site/pull/3' "$(printf '%s\n' "$rr_blk" | sed "s/^round=1 limit=2 reviewer=codex-review previous=null\$/round=3 limit=2 reviewer=claude-review previous='\"$head_sha\"'/")" "$rep_blk"; then
  chk report "$lib/report.json" && jq -e --arg h "$head_sha" '.review_request.extra == true and .review_request.previous == $h and .review_request.reviewer == "claude-review"' "$lib/report.json" >/dev/null \
    && pass "追加の回（round > limit）は extra: true・previous つきで schema に合う" || fail "追加の回: $(cat "$lib/report.json")"
fi
if runblk "report.json（PR 前・頼まない）" 'id=S-1' "$rep_blk"; then
  chk report "$lib/report.json" && jq -e 'keys == ["commit"]' "$lib/report.json" >/dev/null \
    && pass "何も無いときの report.json は commit だけで schema に合う" || fail "空の report.json: $(cat "$lib/report.json")"
fi
if runblk "問い" 'id=S-1' "$q_blk"; then
  if chk question "$lib/questions/1.json" && grep -qx 'ask S-1 - --kind users' "$tmp/proj/ask.args" && grep -q '^推奨案: <推奨案>$' "$tmp/proj/ask.text"
  then pass "worker の手順の問いは questions/<n>.json（question.schema.json）と台帳の text・kind になる"
  else fail "問い: $(head -1 "$tmp/why") $(cat "$tmp/proj/ask.args" 2>/dev/null)"; fi
fi
if runblk "問い（line）" 'id=S-1' "$(printf '%s\n' "$q_blk" | sed 's/--arg kind users/--arg kind line/')"; then
  grep -qx 'ask S-1 -' "$tmp/proj/ask.args" && chk question "$lib/questions/1.json" \
    && pass "kind: line の問いは JSON にだけ書き、台帳には kind を渡さない" || fail "問い（line）: $(cat "$tmp/proj/ask.args")"
fi
rm -f "$tmp/proj/ask.args" "$lib/questions/1.json"
if (cd "$tmp/git" && PATH="$tmp/bin:$PATH" FAKE_ROOT="$tmp/proj" bash -euc "id=S-1"$'\n'"$(printf '%s\n' "$q_blk" | sed 's/, {label: \$other}//')") >"$tmp/run" 2>&1; then
  fail "選択肢が1つの問いが通った"
elif [ ! -e "$tmp/proj/ask.args" ] && [ ! -e "$lib/questions/1.json" ]; then
  pass "形の壊れた問い（選択肢が1つ）は schema で止まり、台帳に置かない"
else fail "形の壊れた問いが台帳に置かれた"; fi

# Chief of Staff の task.yaml の予約キー: 全部書き、task-keys.schema.json に合う
set_line=$(block package/skills/chief-of-staff/SKILL.md '# task.yaml の予約キー' | grep '^deck set ')
printf '%s\n' "$set_line" | tr ' ' '\n' | grep '=' | jq -R 'split("=") | {(.[0]): .[1]}' | jq -s add >"$tmp/keys.json"
if chk task-keys "$tmp/keys.json" && jq -e 'keys == ["merge","must_fix","review","review_rounds","reviewer"]' "$tmp/keys.json" >/dev/null; then
  pass "Chief of Staff は予約キー5つ（review・must_fix・reviewer・review_rounds・merge）を task.yaml に書く"
else fail "Chief of Staff の deck set: $set_line"; fi
grep -q '`auto` は\*\*委譲がこのプロジェクトのマージを任せているときだけ\*\*' package/skills/chief-of-staff/SKILL.md \
  && pass "merge: auto は委譲の範囲内だけ" || fail "chief-of-staff に merge: auto の条件が無い"
grep -q 'jq .review_request tasks/<id>/library/report.json' package/skills/chief-of-staff/SKILL.md \
  && pass "Chief of Staff はレビュー依頼を report.json から読む" || fail "chief-of-staff がレビュー依頼を report.json から読まない"

# Chief Engineer: worktree で文書の PR と report.json を同じタスクから返す。問いは問いの JSON
ce=package/skills/chief-engineer/SKILL.md
grep -q 'cwd は案件の repo の自分の worktree' "$ce" && grep -q '`pr`＝文書の PR' "$ce" && ! grep -q 'tasks/<id>/library/<題' "$ce" \
  && pass "Chief Engineer は worktree で文書の PR と report.json を返す" || fail "chief-engineer の渡し方"
grep -q 'worker skill §5 の1の手順で1問ずつ問いの JSON' "$ce" && grep -q 'worker skill §5 の1の手順で、問いの JSON' package/skills/ask-one-by-one/SKILL.md \
  && pass "Chief Engineer と Chief of Staff の問いは問いの JSON の手順を使う" || fail "問いの JSON の手順を指していない"
left=$(grep -nE 'deck ask [^-]*"<|deck set <id> pr=' package/skills/*/SKILL.md || true)
[ -z "$left" ] && pass "skill に JSON を通らない deck ask と task.yaml の pr が残っていない" || fail "JSON を通らない書き方が残る: $left"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
