#!/usr/bin/env bash
# tests/package.sh — package/ の skill・プロファイル・ゲートの条件表・managed settings・statusline（design.md §2.2・§2.4）。
#   1. skill 7つとプロファイル 8つがそろい、frontmatter の name がディレクトリ名・ファイル名と同じ。
#   2. plugin.json の name が andon。Claude Code 系のプロファイルは command で plugin と役の skill を運び（本文で読ませない）、Codex 系は本文で skill を指す。
#   3. skill とプロファイルに仮設の語が残っていない（用語集の「仮設の語」の節だけは対応表として持つ）。
#   4. gate.json が schema/gate.schema.json の条件表に合う。settings.json が JSON で、statusLine が package の statusline を指す。全フックの command が package に在るファイルだけを指す。
#      Codex の rules（package/codex.rules）の forbidden が settings の deny（コマンドの頭で引く線）と同じ。
#      settings の deny が人のファイル（ADR-0005 決めたこと 4 の (c)）の書き換えを Edit(//workspace/…) で拒む。
#      settings の deny が force-with-lease で main に当たる形を断り、自分の branch への形は allow のまま（当たる・当たらないの例）。
#   5. statusline が JSON を読んで1行を出す。
#   6. skill の機械向けの手順が command（andon issue …）を指し、手で JSON を書く手順が残っていない。例の問いと予約キーは schema に通す。
#   7. 課題（issue）への改名（ADR-0008 受け入れ条件 5）: glossary の項・欄名・skill と template に旧語が無い。
#   8. 役の略称と人への書き方（ADR-0013 受け入れ条件 1・2・7・8 の用語集の分・10）: 用語集の CE・作った語、package/writing.md の書き方の11行・但し書き・語の決まり3つ・用語集のパス、
#      用語集は書き方の節と語の決まりを持たず writing.md を指す、CE skill の問いの出し方、ask-one-by-one の読み手。
#   9. 画面の確かめ（ADR-0018 受け入れ条件 4）: managed だけの許可・worker skill §2・§3・§7・independent-review の観点・environment.md。
#  12. auto memory（ADR-0020 受け入れ条件 2・6）: settings が入れる向き・環境メモの書き方・chief-of-staff skill の規約9・§10・§11。
#  11. 価値のループ（ADR-0011 受け入れ条件 8）: CE skill の価値の仮説の 6 項目・CoS skill の空なら戻す・観察の課題・signal の受け方・worker skill の観察の課題（observation の例は schema に通す）。
#      CoS skill の規約7（write_back の出所への finish の1コメントは委譲の中。決めたこと 15）。
#  14. 受け入れ条件の証拠の質（ADR-0007 R5）: contract.md §4 の acceptance[] の行・CE skill・worker skill §7 に、証拠はテストの command と出力のパスで staged な出力と mock は証拠でない。
#  15. Advisor（ADR-0022 受け入れ条件 1・2）: --advisor fable は chief-of-staff の command だけ、CoS skill の「人の入口」に相談する場面の1行。
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
  for field in command resume; do
    value=$(fm "$f" "$field")
    for key in features.memories memories.generate_memories memories.use_memories; do
      case " $value " in
        *" -c $key=false "*) pass "プロファイル $p の $field が $key を無効化する" ;;
        *) fail "プロファイル $p の $field に -c $key=false が無い" ;;
      esac
    done
  done
  case " $(fm "$f" command) " in *--plugin-dir*|*--append-system-prompt-file*) fail "プロファイル $p（Codex）の command に Claude Code の口がある"; continue ;; esac
  grep -q '`/opt/andon/skills/worker/SKILL.md` を最初に読み' "$f" && [ -f package/skills/worker/SKILL.md ] \
    && pass "プロファイル $p（Codex）は本文で worker skill を指す" || fail "プロファイル $p（Codex）が本文で worker skill を指していない"
done
grep -q '§6' package/templates/codex-review.md && grep -q '§6' package/templates/claude-review.md \
  && pass "レビュアーのプロファイルは worker skill の §6 を指す" || fail "レビュアーのプロファイルが worker §6 を指していない"

# === 2b. 前の置き場の名前が残っていない（ADR-0004 受け入れ条件 4 の前半） ===
# 台帳は kanban/（KANBAN_ROOT）、coordinator.md は AGENTS.md の Chief of Staff の節、委譲は project.json の delegation。
# 移す前の名前を skill・プロファイル・フック・command・環境メモに書かない（package/gate.json の保護の表は対象の外）。
left=$(grep -rnE '\.flightdeck|FLIGHTDECK_ROOT|coordinator\.md|delegation\.(json|md)' package/skills package/templates package/hooks package/bin package/lib package/environment.md || true)
[ -z "$left" ] && pass "skill・プロファイル・フック・command に .flightdeck・coordinator.md・delegation.json が残っていない" || fail "前の置き場の名前が残る: $(head -3 <<<"$left")"

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
jq -e 'has("repos") | not' package/gate.json >/dev/null && pass "package/gate.json に repos が無い（セルの gate.local.json へ）" || fail "package/gate.json に repos がある"
for p in package/skills/ package/templates/ package/gate.json package/settings.json package/codex.rules package/writing.md; do
  jq -e --arg p "${p%/}" '.tier_c_paths | map(rtrimstr("/**")) | index($p)' package/gate.json >/dev/null \
    && pass "ラインの変更 $p はレベル C" || fail "$p が tier_c_paths に無い"
done
# 人だけが書くファイル（author 照合の対象）は、PR に入ればレベル C（contract.md §11・§7）
for p in 'projects/*/project.json' gate.local.json andon.toml; do
  jq -e --arg p "$p" '.tier_c_paths | index($p)' package/gate.json >/dev/null \
    && pass "人のファイル $p はレベル C" || fail "$p が tier_c_paths に無い"
done
if jq -e . package/settings.json >/dev/null 2>&1; then pass "package/settings.json は JSON"; else fail "package/settings.json が JSON でない"; fi
jq -e '.permissions.deny | index("Bash(gh pr merge:*)")' package/settings.json >/dev/null && pass "settings は gh pr merge を deny" || fail "settings の deny に gh pr merge が無い"
# ADR-0020 受け入れ条件 2: auto memory を入れ、置き場は /workspace/memory。env で止めない。
# 置き場の .md は Claude Code が自分で書き込みを許すので、allow に足さない（A-0144 の実測 (c)）
jq -e '.autoMemoryEnabled == true and .autoMemoryDirectory == "/workspace/memory" and ((.env // {}) | has("CLAUDE_CODE_DISABLE_AUTO_MEMORY") | not)' package/settings.json >/dev/null \
  && ! jq -r '.permissions.allow[]' package/settings.json | grep -q '/workspace/memory' \
  && pass "settings が auto memory を入れ、置き場は /workspace/memory（env で止めない・allow に足さない。ADR-0020）" || fail "settings の auto memory が ADR-0020 の形でない"
# Codex の rules（package/codex.rules）は、deny のうちコマンドの頭で引く線（Bash(<語…>:*)）と同じ一覧。
# それ以外の deny（Bash(git diff *--output*) など）は allow の穴を塞ぐもので、allow の無い Codex（承認は never）には要らない。
# その印として、頭の線でない deny は allow のどれかと同じコマンドで始まること（新しい種類の deny を足したら、ここで止まって決める）
deny_heads=$(jq -r '.permissions.deny[] | select(test("^Bash\\([^*]*:\\*\\)$")) | sub("^Bash\\("; "") | sub(":\\*\\)$"; "")' package/settings.json | sort)
# rules は頭の線ごとに、直接の形（["gh", "pr", "merge"]）と wrapper の形（[[wrapper…], "gh", "pr", "merge"]）の2本を持つ
rules_of() { # rules_of <direct|wrapped>: その形の forbidden の頭の線
  python3 - package/codex.rules "$1" <<'EOF'
import ast, re, sys
text = open(sys.argv[1], encoding="utf-8").read()
for m in re.finditer(r'prefix_rule\((.*?)\n\)', text, re.S):
    body = m.group(1)
    pat = re.search(r'pattern\s*=\s*(\[.*?\])\s*,\s*\n', body, re.S)
    dec = re.search(r'decision\s*=\s*"([a-z]+)"', body)
    if not (pat and dec and dec.group(1) == "forbidden"):
        continue
    p = ast.literal_eval(pat.group(1))
    wrapped = isinstance(p[0], list)
    rest = p[1:] if wrapped else p
    if all(isinstance(x, str) for x in rest) and wrapped == (sys.argv[2] == "wrapped"):
        print(" ".join(rest))
EOF
}
for kind in direct wrapped; do
  rule_heads=$(rules_of "$kind" | sort)
  [ -n "$deny_heads" ] && [ "$deny_heads" = "$rule_heads" ] \
    && pass "package/codex.rules の forbidden（$kind）が settings の deny（頭の線）と同じ（$(tr '\n' ',' <<<"$deny_heads" | sed 's/,$//')）" \
    || fail "package/codex.rules（$kind）と settings の deny がずれる（deny: $(tr '\n' ',' <<<"$deny_heads") / rules: $(tr '\n' ',' <<<"$rule_heads")）"
done
bad=
while IFS= read -r d; do
  [ -z "$d" ] && continue
  cmd=$(sed -E 's/^Bash\(([a-z-]+ [a-z-]+).*/\1/' <<<"$d")
  jq -e --arg c "$cmd" '.permissions.allow | any(startswith("Bash(" + $c + " ") or startswith("Bash(" + $c + ")") or startswith("Bash(" + $c + ":"))' package/settings.json >/dev/null || bad="$bad [$d]"
done < <(jq -r '.permissions.deny[] | select(startswith("Bash(")) | select(test("^Bash\\([^*]*:\\*\\)$") | not)' package/settings.json)
[ -z "$bad" ] && pass "頭の線でない deny は allow の穴を塞ぐものだけ（Codex の rules に要らない）" || fail "頭の線でも allow の穴塞ぎでもない deny（Codex の rules にどう引くか決める）:$bad"
# force-with-lease は自分の課題の branch だけに通し、main に当たる形は deny で断る（docs/decisions/2026-11-10-force-with-lease-own-branch-only.md）。
# Claude Code の当て方を写す: 規則は命令の全体に当て、* は空を含む任意の文字列。末尾の「 *」が唯一の * なら、空白の無い素の命令にも当たる（:* も同じ）
out=$(python3 - package/settings.json <<'EOF'
import json, re, sys
perm = json.load(open(sys.argv[1], encoding="utf-8"))["permissions"]
def rules(kind):
    return [r[5:-1] for r in perm[kind] if r.startswith("Bash(") and r.endswith(")")]
def hit(rule, cmd):
    if rule.endswith(":*"):
        rule = rule[:-2] + " *"
    if rule.endswith(" *") and rule.count("*") == 1 and cmd == rule[:-2]:
        return True
    return re.fullmatch(".*".join(re.escape(p) for p in rule.split("*")), cmd, re.S) is not None
def verdict(cmd):
    if any(hit(r, cmd) for r in rules("deny")):
        return "deny"
    return "allow" if any(hit(r, cmd) for r in rules("allow")) else "ask"
cases = {
    "allow": ["git push --force-with-lease", "git push --force-with-lease origin HEAD",
              "git push --force-with-lease origin andon/A-0001-fix-main-page",
              "git push --force-with-lease origin HEAD:andon/A-0001-maintenance"],
    "deny": ["git push --force-with-lease origin main", "git push origin main --force-with-lease",
             "git push --force-with-lease origin HEAD:main", "git push origin HEAD:main --force-with-lease",
             "git push --force-with-lease origin andon/A-0001-x:main", "git push --force-with-lease origin +HEAD:main",
             "git push --force-with-lease origin +main", "git push --force-with-lease origin refs/heads/main",
             "git push --force-with-lease origin HEAD:refs/heads/main", "git push origin HEAD:refs/heads/main --force-with-lease",
             "git push --force-with-lease=main:abc123 origin HEAD:main", "git push --force-with-lease origin main andon/A-0001-x",
             "git push --force-with-lease --all origin", "git push --all --force-with-lease", "git push --mirror --force-with-lease origin"],
}
for want, cmds in cases.items():
    for c in cmds:
        got = verdict(c)
        print(("ok   " if got == want else "FAIL ") + f"force-with-lease の許可ルール: {c} → {got}（期待 {want}）")
EOF
)
while IFS= read -r l; do
  case "$l" in "ok   "*) pass "${l#ok   }" ;; *) fail "${l#FAIL }" ;; esac
done <<<"$out"
# 人のファイル（ADR-0005 決めたこと 4 の (c)）への書き換えを deny する。今の Claude Code は Write・NotebookEdit・MultiEdit を Edit(path) の規則で引き、
# Write(path) の規則は受け付けても当たらない。絶対パスは //（単独の / は設定の置き場からの相対）。Codex には同じ線を引けない（docs/image.md）
for f in andon.toml AGENTS.md gate.local.json 'templates.local/**' 'local/**' 'bin/**' 'projects/*/project.json'; do
  jq -e --arg r "Edit(//workspace/$f)" '.permissions.deny | index($r)' package/settings.json >/dev/null \
    && pass "settings は人のファイル /workspace/$f の書き換えを deny" || fail "settings の deny に Edit(//workspace/$f) が無い"
done
# 移す前の委譲のファイルは読まない（contract.md §11）ので deny しない（project.json の deny が受け持つ）
! jq -e '.permissions.deny | any(test("delegation\\.(json|md)"))' package/settings.json >/dev/null \
  && pass "settings の deny に delegation.json・delegation.md が残っていない" || fail "settings の deny に delegation.json・delegation.md が残る"
bad=$(jq -r '.permissions.deny[] | select(startswith("Bash(") or startswith("Edit(//workspace/") | not)' package/settings.json)
[ -z "$bad" ] && pass "deny は Bash(…) と Edit(//workspace/…) だけ（Write(…) は当たらないので書かない）" || fail "知らない形の deny: $(tr '\n' ' ' <<<"$bad")"
sl=$(jq -r '.statusLine.command' package/settings.json)
[ "$sl" = /opt/andon/lib/andon/statusline ] && [ -x package/lib/andon/statusline ] \
  && pass "statusLine は package/lib/andon/statusline を指す" || fail "statusLine: $sl"
for ev in Stop:stop UserPromptSubmit:user-prompt-submit; do
  jq -e --arg e "${ev%%:*}" --arg c "/opt/andon/hooks/${ev#*:}" '.hooks[$e] | any(.[].hooks[]; .command == $c)' package/settings.json >/dev/null \
    && [ -x "package/hooks/${ev#*:}" ] && sh -n "package/hooks/${ev#*:}" \
    && pass "${ev%%:*} フックは package/hooks/${ev#*:} を指す" || fail "${ev%%:*} フック: package/hooks/${ev#*:}"
done
# 待ち役（ADR-0019 決めたこと 1・9）: Stop に asyncRewake のフックが timeout つきで在り、待ち役に渡す --timeout がその timeout と同じ
wt=$(jq -r '[.hooks.Stop[].hooks[] | select(.command == "/opt/andon/hooks/stop-wait" and .asyncRewake == true and (.timeout | type) == "number")][0].timeout // empty' package/settings.json)
[ -n "$wt" ] && [ -x package/hooks/stop-wait ] && sh -n package/hooks/stop-wait && grep -q "andon dispatcher wait --timeout $wt\$" package/hooks/stop-wait \
  && pass "Stop の待ち役は asyncRewake のフック（timeout $wt 秒。--timeout も同じ）" || fail "Stop に asyncRewake の待ち役のフックが無い・timeout が待ち役の --timeout と違う"
# 提供元に切られた Chief of Staff を起こす（ADR-0019 決めたこと 10）: StopFailure に asyncRewake のフックが、最後の待ち（8 分）より長い timeout つきで在る
ft=$(jq -r '[.hooks.StopFailure[].hooks[] | select(.command == "/opt/andon/hooks/stop-failure" and .asyncRewake == true and (.timeout | type) == "number")][0].timeout // empty' package/settings.json)
[ -n "$ft" ] && [ "$ft" -gt 480 ] && [ -x package/hooks/stop-failure ] && sh -n package/hooks/stop-failure && grep -q '^exec andon dispatcher rewake$' package/hooks/stop-failure \
  && pass "StopFailure は asyncRewake のフック（timeout $ft 秒。8 分の待ちより長い）で andon dispatcher rewake を打つ" || fail "StopFailure に asyncRewake の rewake のフックが無い・timeout が 8 分以下"

# 全フックの command が、セルに在るファイルだけを指す（/opt/andon/ は package/ の写し。$HOME 配下は image に無い）
cmds=$(jq -r '[.hooks[][].hooks[].command] | .[]' package/settings.json)
bad=
while IFS= read -r c; do
  [ -z "$c" ] && continue
  case $c in
    /opt/andon/*) [ -e "package/${c#/opt/andon/}" ] || bad="$bad [$c]" ;;
    *) bad="$bad [$c]" ;;
  esac
done <<<"$cmds"
[ -z "$bad" ] && pass "settings のフックは package に在る /opt/andon/ のファイルだけを指す" || fail "settings のフックがセルに無いファイルを指す:$bad"

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

# 担当の機械向けの手順は command（andon issue …）。skill に JSON を手で書く手順・一致を手で確かめる手順が残っていない
w=package/skills/worker/SKILL.md
for c in 'andon issue report' 'andon issue review-request' 'andon issue ready' 'andon issue ask' 'andon issue show'; do
  grep -q "$c" "$w" && pass "worker skill は $c を使う" || fail "worker skill に $c が無い"
done
left=$(grep -nE 'schema_check|report\.json"? *$|> "\$lib/report\.json"|questions/\$\{n|一致を確かめる|headRefOid' "$w" || true)
[ -z "$left" ] && pass "worker skill に report.json・問いの JSON を手で書く手順と、一致を手で確かめる手順が無い" || fail "手の手順が残る: $left"
# skill の問いの例（andon issue ask の stdin）は question.schema.json に合う
q_blk=$(block "$w" 'andon issue ask "$id" -' | sed '1d;$d')
printf '%s\n' "$q_blk" >"$tmp/q.json"
[ -n "$q_blk" ] && chk question "$tmp/q.json" && pass "worker skill の問いの例は question.schema.json に合う" || fail "問いの例: $(head -1 "$tmp/why" 2>/dev/null) $q_blk"

# 工程の背骨（ADR-0006 決めたこと 9・11・12・13）: worker の工程表に6つの工程、報告の書式に ## 計画・探した範囲・計画との差。Chief of Staff の雛形に 横展開: と調べる課題の 終了条件:
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w")
miss=
for s in 理解 計画 作業 確認 完成 振り返り; do grep -q "^| $s |" <<<"$w2" || miss="$miss $s"; done
[ -z "$miss" ] && pass "worker skill §2 の工程表に 理解・計画・作業・確認・完成・振り返り がある" || fail "worker skill §2 の工程表に無い工程:$miss"
w7=$(block "$w" 'PR: <https://')
grep -q '^## 計画$' <<<"$w7" && grep -q '^探した範囲: ' <<<"$w7" && grep -q '^計画との差: ' <<<"$w7" \
  && pass "worker skill の報告の書式に ## 計画・探した範囲・計画との差がある" || fail "worker skill の報告の書式に ## 計画・探した範囲・計画との差が無い"# 調べる課題の候補の例（andon issue candidates の stdin）は candidates.schema.json に合う。scope の問いは worker skill が案内する
c_blk=$(block "$w" 'andon issue candidates "$id" -' | sed '1d;$d')
printf '%s\n' "$c_blk" >"$tmp/c.json"
[ -n "$c_blk" ] && chk candidates "$tmp/c.json" && pass "worker skill の候補の例は candidates.schema.json に合う" || fail "候補の例: $(head -1 "$tmp/why" 2>/dev/null) $c_blk"
grep -q 'kind: scope' "$w" && grep -q 'andon issue candidates' "$w" && pass "worker skill は scope の問いと andon issue candidates を案内する" || fail "worker skill に scope の問いか andon issue candidates が無い"

cos=package/skills/chief-of-staff/SKILL.md
block "$cos" '# <id> <題>' | grep -q '^横展開: ' && block "$cos" '# <id> 調べる: ' | grep -q '^終了条件: ' \
  && pass "Chief of Staff の依頼書の雛形に 横展開: の行、調べる課題の雛形に 終了条件: の行がある" || fail "chief-of-staff の雛形に 横展開: か 終了条件: が無い"

block "$cos" '# <id> 調べる: ' | grep -q '^- Deliverable: candidates' \
  && pass "調べる課題の雛形の Settings に Deliverable: candidates がある" || fail "調べる課題の雛形に Deliverable: candidates が無い"

# ADR-0009 受け入れ条件 3: 人の入口・打つコマンド・依頼書の原則と範囲の外・差し戻し・wiki の節
[ "$(grep -m1 '^## ' "$cos")" = '## 人の入口' ] && block "$cos" 'andon chief-engineer --brief - "<題>"' | grep -q '^経緯: ' \
  && ! grep -qE 'andon chief-engineer[^。]*を勧め|題として人に勧め|人に題を勧め' "$cos" \
  && pass "Chief of Staff の skill の先頭は「人の入口」で、andon chief-engineer --brief - を自分で打つ（人に勧めない）" || fail "chief-of-staff の先頭に「人の入口」が無いか、andon chief-engineer を人に勧める文が残る"
grep -q '^- 打つコマンドは .*`andon chief-engineer`' "$cos" \
  && pass "Chief of Staff の打つコマンドの一覧に andon chief-engineer がある" || fail "chief-of-staff の打つコマンドの一覧に andon chief-engineer が無い"
sed -n '/^\*\*欄の読み方\*\*/,/^| 欄 |/p' "$cos" | grep -q 'repo を読めば分かること（作法・構造・直し方）は書かず' \
  && grep -q '^| 守ること | 範囲の外（今回やらないこと）' "$cos" \
  && pass "依頼書の欄の読み方の頭に原則の1行、守ることの説明に 範囲の外" || fail "chief-of-staff §2 に依頼書の原則か 範囲の外 が無い"
grep -q '## 差し戻し（<日付>）' "$cos" && grep -q '`## 差し戻し` が2つあってまだ通らない' "$cos" && grep -q '依頼書を直した新しい課題を `kanban add`' "$cos" \
  && pass "差し戻しは依頼書への追記で数え、2回で通らなければ新しい課題に切る" || fail "chief-of-staff に差し戻しの追記か切り直しの手順が無い"
cos11=$(sed -n '/^## 11\. wiki$/,/^## ルール$/p' "$cos")
# ADR-0016 受け入れ条件 1: 層・3つの操作・元資料の3種・移行があり、前の形（1ファイル1件・log.md は作らない）が無い
miss=""; for k in '何を' '層' '元資料を置くとき' 'いつ（ingest）' 'query' 'lint' '移行'; do grep -q "^- \*\*$k\*\*" <<<"$cos11" || miss="$miss $k"; done
for k in '`raw/`' '`sources/`' '`entities/`' '`topics/`' '`overview.md`' '`index.md`' '`log.md`'; do grep -q "^  - $k — " <<<"$cos11" || miss="$miss 層$k"; done
for k in '人の発言' '外の資料' '課題の記録' 'memory'; do grep -q "^  - $k: " <<<"$cos11" || miss="$miss 元資料:$k"; done
grep -qF '## [YYYY-MM-DD] ingest | <題>' <<<"$cos11" && grep -q 'ingest 約10件ごと' <<<"$cos11" && grep -q '`index.md` → ページの順' <<<"$cos11" || miss="$miss log の形・lint の時期・query の順"
grep -q 'commit --only -m "wiki: <要点>" -- wiki' <<<"$cos11" || miss="$miss commit（--only -- wiki）"
grep -q '書く前に秘密.*`<伏せた: 種類>`' <<<"$cos11" || miss="$miss raw/ に書く前に秘密を伏せる"
grep -qE '1ファイル1件|`log.md` は作らない' <<<"$cos11" && miss="$miss 前の形の行が残る"
[ -z "$miss" ] && pass "Chief of Staff の skill の wiki の節が LLM Wiki の形（層・ingest・query・lint・元資料・移行。ADR-0016）" || fail "chief-of-staff の wiki の節:$miss"
# ADR-0020 受け入れ条件 6: 規約9は memory を受け皿とし、§10 が wiki 行きの memory を raw/memory/ に写し、§11 の raw/ は4種
grep -q '^| 9\. 記憶の規律 | memory（auto memory。`/workspace/memory/`）は蒸留までの受け皿。残すものはトリアージを通して wiki と repo へ' "$cos" \
  && ! grep -q 'ハーネス内蔵のメモリは使わない' "$cos" \
  && sed -n '/^## 10\. トリアージ$/,/^## 11\. wiki$/p' "$cos" | grep -q '`close`「wiki へ: <題>」のものは、記録.*の本文を言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写して ingest' \
  && grep -q '^  - `raw/` — 元資料。書いたら変えない。4種: .*(4) トリアージで wiki 行きになった memory の写し（`raw/memory/' <<<"$cos11" \
  && pass "Chief of Staff の skill: 規約9・§10・§11 が memory を蒸留までの受け皿として扱う（ADR-0020）" || fail "chief-of-staff の規約9・§10・§11 が ADR-0020 の形でない"
# ADR-0016 受け入れ条件 3: 読む側（Chief Engineer の「読む」と Chief of Staff §1）が index.md → ページの順で引く
grep -q '^1\. \*\*読む.*セルの wiki（`index.md` → ページの順で引く。使った事実は出典' package/skills/chief-engineer/SKILL.md \
  && sed -n '/^## 1\. /,/^## 2\. /p' "$cos" | grep -q 'wiki を引く（§11 の query。`index.md` → ページの順）' \
  && pass "Chief Engineer の「読む」と Chief of Staff §1 が wiki を index.md → ページの順で引く" || fail "読む側が index.md → ページの順を指していない"
# ADR-0009 受け入れ条件 4: Chief of Staff の template が wiki を道具の届く場所に足す
case " $(fm package/templates/chief-of-staff.md command) " in *" --add-dir /workspace/wiki "*) pass "Chief of Staff の template の command に --add-dir /workspace/wiki" ;; *) fail "Chief of Staff の template に --add-dir /workspace/wiki が無い" ;; esac
# ADR-0022 受け入れ条件 1・2: Advisor（Fable）は Chief of Staff だけ。skill の「人の入口」に相談する場面の1行
case " $(fm package/templates/chief-of-staff.md command) " in *" --advisor fable "*) pass "Chief of Staff の template の command に --advisor fable" ;; *) fail "Chief of Staff の template に --advisor fable が無い" ;; esac
left=
for p in $profiles; do
  [ "$p" = chief-of-staff ] && continue
  for field in command resume; do
    case " $(fm "package/templates/$p.md" "$field") " in *--advisor*) left="$left $p.$field" ;; esac
  done
done
[ -z "$left" ] && pass "chief-of-staff のほかのプロファイルの command・resume に --advisor が無い" || fail "--advisor が付いている:$left"
sed -n '/^## 人の入口$/,/^## 0\. /p' "$cos" | grep 'advisor に相談してから決める' | grep -q '振り分けに迷うとき' \
  && sed -n '/^## 人の入口$/,/^## 0\. /p' "$cos" | grep 'advisor に相談してから決める' | grep -q '2回差し戻して切り直すとき' \
  && pass "Chief of Staff の skill の「人の入口」に advisor に相談する場面（振り分けに迷うとき・2回差し戻して切り直すとき）" || fail "Chief of Staff の skill の「人の入口」に advisor の1行が無い"

# Chief of Staff の issue.yaml の予約キー: 全部書き、issue-keys.schema.json に合う
set_line=$(block package/skills/chief-of-staff/SKILL.md '# issue.yaml の予約キー' | grep '^kanban set ')
printf '%s\n' "$set_line" | tr ' ' '\n' | grep '=' | jq -R 'split("=") | {(.[0]): .[1]}' | jq -s add >"$tmp/keys.json"
if chk issue-keys "$tmp/keys.json" && jq -e 'keys == ["estimate","merge","must_fix","review","review_rounds","reviewer"]' "$tmp/keys.json" >/dev/null; then
  pass "Chief of Staff は予約キー6つ（review・must_fix・reviewer・review_rounds・merge・estimate）を issue.yaml に書く"
else fail "Chief of Staff の kanban set: $set_line"; fi
# ADR-0014 受け入れ条件 7（E1）: 雛形の Settings に Estimate の行（段の名）、起き直しで打ち直す予約キーに estimate
block package/skills/chief-of-staff/SKILL.md '# <id> <題>' | grep -qx -- '- Estimate: 中' \
  && block package/skills/chief-of-staff/SKILL.md '# <id> 調べる: ' | grep -qx -- '- Estimate: 小' \
  && grep -q '同じ値で `kanban set <id> review=… .*`estimate`' package/skills/chief-of-staff/SKILL.md \
  && pass "Chief of Staff の雛形に Estimate の行があり、起き直しで estimate も打ち直す" || fail "chief-of-staff に Estimate の行か起き直しの estimate が無い"
grep -q '`auto` は\*\*委譲がこのプロジェクトのマージを任せているときだけ\*\*' package/skills/chief-of-staff/SKILL.md \
  && pass "merge: auto は委譲の範囲内だけ" || fail "chief-of-staff に merge: auto の条件が無い"
# インナーループは dispatcher（P6a）: Chief of Staff の skill のレビューとゲートの節は人の判断が要るときだけ。レビュアーを手で起こす手順が残っていない
cos6=$(sed -n '/^## 6\. /,/^## 7\. /p' package/skills/chief-of-staff/SKILL.md)
grep -q '^## 6\. レビューとゲート（人の判断が要るときだけ）' <<<"$cos6" && grep -q '^### `needs_human` のとき' <<<"$cos6" \
  && ! grep -qE 'andon issue start "\$new"|worktree add --detach|andon gate <id>' <<<"$cos6" \
  && ! grep -qE 'worktree add --detach|andon issue start "\$new"' package/skills/independent-review/SKILL.md \
  && pass "Chief of Staff の §6 は needs_human のときだけ。レビュアーを起こすのとゲートは dispatcher" || fail "chief-of-staff・independent-review にレビュアーを手で起こす手順・ゲートを手で回す手順が残る"
grep -q '元の課題（`issue.yaml` の `parent`）の担当だけ' package/skills/worker/SKILL.md && grep -q '結果の宛先は元の課題の担当だけ' package/skills/independent-review/SKILL.md \
  && pass "レビュアーの結果の宛先は parent の課題の担当だけ（Chief of Staff へ送らない）" || fail "レビュアーの宛先が parent の課題の担当に固定されていない"

# Chief Engineer: worktree で文書の PR と report.json を同じ課題から返す。問いは問いの JSON
ce=package/skills/chief-engineer/SKILL.md
grep -q 'cwd は案件の repo の自分の worktree' "$ce" && grep -q '`pr`＝文書の PR' "$ce" && ! grep -q 'issues/<id>/library/<題' "$ce" \
  && pass "Chief Engineer は worktree で文書の PR と report.json を返す" || fail "chief-engineer の渡し方"
grep -q 'worker skill §5 の1の手順で1問ずつ問いの JSON' "$ce" && grep -q 'andon issue ask "$id" -' package/skills/ask-one-by-one/SKILL.md \
  && pass "Chief Engineer と Chief of Staff の問いは問いの JSON の手順を使う" || fail "問いの JSON の手順を指していない"
# ADR-0009 受け入れ条件 5: WIP の行が無い・読むに価値の仮説の結果・文書の形に価値の仮説の見出し・言い直すで決断が無ければ next で返す
! grep -qE '1プロジェクトに1題|前の題が終わらないまま' "$ce" \
  && pass "Chief Engineer の skill に WIP の行（1プロジェクトに1題・前の題が終わらないまま）が無い" || fail "chief-engineer に WIP の行が残る"
ce1=$(sed -n '/^1\. \*\*読む/,/^2\. /p' "$ce"); ce2=$(sed -n '/^2\. \*\*言い直す/,/^3\. /p' "$ce")
grep -q '前の価値の仮説の結果' <<<"$ce1" && block "$ce" '# <題>（仕様・設計' | grep -q '^## 価値の仮説' \
  && pass "Chief Engineer の「読む」に前の価値の仮説の結果、文書の形に ## 価値の仮説 の見出し" || fail "chief-engineer に価値の仮説の席が無い"
grep -q '決断が無ければ文書を書かずに返す' <<<"$ce2" && grep -q '"to": "chief-of-staff", "text": "直す課題に切る: ' <<<"$ce2" && grep -q 'kanban review <id>' <<<"$ce2" \
  && pass "Chief Engineer は「言い直す」で決断が無ければ文書を書かず next で返す" || fail "chief-engineer の「言い直す」に文書なしで返す段が無い"
left=$(grep -nE 'kanban ask [^-]*"<|kanban set <id> pr=' package/skills/*/SKILL.md || true)
[ -z "$left" ] && pass "skill に JSON を通らない kanban ask と issue.yaml の pr が残っていない" || fail "JSON を通らない書き方が残る: $left"

# === 7. 課題（issue）／作業（task）への改名（ADR-0008 受け入れ条件 5・4・2 の欄名） ===
g=package/skills/glossary/SKILL.md
grep -q '^\*\*課題\*\*（issue）:' "$g" && grep -q '^\*\*作業\*\*（task）:' "$g" && grep -q '^課題（issue）A-0012' "$g" && grep -q '作業（task）1:' "$g" \
  && pass "glossary に 課題（issue）・作業（task）の項と包含の図がある" || fail "glossary の課題・作業の項か包含の図が無い"
! grep -q '（work item）' "$g" && pass "glossary に「タスク（work item）」の項が無い" || fail "glossary に work item の項が残る"
# 「タスク」は「避ける」の行と仮設の語の表の行にしか無い
left=$(grep -n 'タスク' "$g" | grep -v '^[0-9]*:_避ける_' | grep -v '^[0-9]*:| タスク' || true)
[ -z "$left" ] && pass "glossary の「タスク」は「避ける」の行と仮設の語の表にしか無い" || fail "glossary に「タスク」が残る: $left"
# skill 7本と template 8本に「タスク」と andon task・旧欄名・旧キーが無い（glossary は上で見た）
left=$(grep -rnE 'タスク|andon task|to: task|from: task|max-tasks|created_task|task\.yaml|kanban/tasks' package/skills package/templates --exclude-dir=glossary || true)
[ -z "$left" ] && pass "skill と template に「タスク」・andon task・旧キーが無い" || fail "旧語が残る: $left"
sed -n '/^# <id> <題>$/,/^```$/p' "$cos" | sed -n '2p' | grep -q '^解決すべき問題:' \
  && ! grep -q '^やること:' "$cos" && pass "chief-of-staff §2 の雛形の最初の欄が 解決すべき問題: で、やること: が無い" || fail "chief-of-staff の雛形の最初の欄が 解決すべき問題: でない"
grep -q '古い依頼書の「やること」欄は「解決すべき問題」と同じ欄として読む' package/skills/worker/SKILL.md \
  && pass "worker skill は 解決すべき問題 と古い やること の両方を読む" || fail "worker skill に古い やること 欄の読み替えが無い"
grep -q '解決すべき問題' package/skills/triage/SKILL.md && ! grep -q 'やること' package/skills/triage/SKILL.md package/skills/triage/assets/triage.yaml \
  && pass "triage skill とルーチンの依頼書の欄名が 解決すべき問題" || fail "triage の欄名が 解決すべき問題 でない"
grep -q 'andon issue stop-check' package/hooks/stop \
  && pass "package/hooks/stop が andon issue stop-check を打つ" || fail "package/hooks/stop が andon issue stop-check を打たない"

# === 8. 役の略称と人への書き方（ADR-0013 受け入れ条件 1・2） ===
g=package/skills/glossary/SKILL.md
grep -q '^\*\*Chief Engineer\*\*（chief-engineer。略して CE）:' "$g" && grep -q '^\*\*Chief of Staff\*\*（chief-of-staff。略して CoS）:' "$g" \
  && pass "用語集の Chief Engineer の項に「略して CE」がある" || fail "用語集に CE の略称が無い"
# 用語集（受け入れ条件 8 の用語集の分）: 書き方の節と語の決まり3つを持たず、writing.md を指す。モデルからは呼べないまま
grep -q '^- \*\*役と部品の語はここにある語だけを使う\*\*' "$g" && ! grep -q '^## 人への書き方' "$g" && grep -q '^disable-model-invocation: true$' "$g" \
  && ! grep -qE 'ふだんカタカナで言う語|訳語を新しく作らない|作った訳語|文の中では略称で' "$g" \
  && grep -q '^- 人への書き方と語の決まり.*は `/opt/andon/writing.md` に従う' "$g" \
  && pass "用語集は書き方の節と語の決まりを持たず writing.md を指す（disable-model-invocation は残る）" || fail "用語集に書き方の節か語の決まりが残る、か writing.md を指していない"
iss=$(sed -n '/^\*\*課題\*\*（issue）:/,/^_避ける_/p' "$g")
! grep -q '「番号（題）」' <<<"$iss" && grep -q '`/opt/andon/writing.md` の番号の決まりに従う' <<<"$iss" \
  && pass "用語集の課題の項は番号（題）の決まりを自分で言わず writing.md を指す" || fail "用語集の課題の項が writing.md を指していない"
# writing.md（受け入れ条件 7）: 読み手を人に返す文に絞り、箇条書きは11行、但し書き・語の決まり3つ・用語集のパス
wr=package/writing.md
how=$(sed -n '1,/^## /p' "$wr")
n=$(grep -c '^- ' <<<"$how")
[ "$n" = 11 ] && grep -q '^CoS・CE・担当が人に返す文' <<<"$how" \
  && grep -q '^- 結論を先に書く（BLUF）' <<<"$how" && grep -q '状況・経緯・見立て・推奨の順に書く（SBAR）' <<<"$how" \
  && grep -q '「・」で語を詰めない' <<<"$how" && grep -q '1文に1つのこと' <<<"$how" \
  && grep -q "英語の用語を直訳した語は使わない" <<<"$how" && grep -q '灯り → 人の判断待ち、レベル C → 人がマージ' <<<"$how" \
  && grep -q '^- 括弧に説明を入れない。説明は文にする。括弧に入れるのは識別子と、番号に添える題だけ。' <<<"$how" \
  && grep -q '^- 番号だけで指さない。.*`S-0004（返事の下書きボタン）`.*同じ返事の2度目からは番号だけでよい。' <<<"$how" \
  && grep -q '^- 読む人は道具の呼び出しも、読んだファイルも、途中の考えも見ていない。' <<<"$how" \
  && grep -q '^- つながる考えは段落の文で書く。箇条書きは別々の項目を並べるときに使う。' <<<"$how" \
  && grep -q '^- 作業の途中で自分が付けた呼び名や略称を使わない。' <<<"$how" \
  && grep -q '^コードと、人が別の形を求めたときは、この決まりから除く' <<<"$how" \
  && pass "writing.md は人に返す文に絞り、書き方の11行と但し書きがある" || fail "writing.md の書き方の節（行の数 $n）"
! grep -qE '括弧には識別子だけ|番号で参照しない' "$wr" && pass "writing.md に古い字面（括弧には識別子だけ・番号で参照しない）が無い" || fail "writing.md に古い字面が残る"
grep -q 'Chief of Staff を CoS、Chief Engineer を CE と書く' "$wr" \
  && grep -q 'ふだんカタカナで言う語（コスト・プロファイル・テンプレート・コンテキスト・オーナー・ログ・パス）はカタカナのまま' "$wr" \
  && grep -q '訳語を新しく作らない。迷ったら同僚に口で言うときの言い方にする' "$wr" && grep -q '`/opt/andon/skills/glossary/SKILL.md`' "$wr" \
  && pass "writing.md に語の決まり3つと用語集のパスがある" || fail "writing.md に語の決まりか用語集のパスが無い"
# 作った語は「避ける」の行と仮設の語の表にしか出ない。残す語（検査・継ぎ目・条件表・控え）は消さない
left=$(sed '/^## 仮設の語/,$d' "$g" | grep -nE '席|端末の土台|持ち主|提供元' | grep -v '^[0-9]*:_避ける_' || true)
[ -z "$left" ] && pass "用語集に「席」「端末の土台」「持ち主」「提供元」が正式な語として出ない" || fail "用語集に作った語が残る: $left"
grep -q '^\*\*検査\*\*（checks）:' "$g" && grep -q '条件表' "$g" \
  && pass "用語集の残す語（検査・条件表）は残る" || fail "用語集から残す語が消えた"
! grep -q '一度に出すのは互いに依らない問いの全部' "$ce" && grep -q 'ask-one-by-one skill §2 に従う（1回の返事で1問）' "$ce" \
  && pass "CE skill の問いの出し方は ask-one-by-one §2 を指す（束で出す行が無い）" || fail "CE skill の問いの出し方が ask-one-by-one §2 を指していない"
fm package/skills/ask-one-by-one/SKILL.md description | grep -q 'Chief Engineer（CE）' \
  && pass "ask-one-by-one の description の読み手に CE がある" || fail "ask-one-by-one の description に CE が無い"

# === 9. 画面の確かめ（ADR-0018 受け入れ条件 4） ===
jq -e '.allowManagedPermissionRulesOnly == true' package/settings.json >/dev/null \
  && pass "settings は許可の規則を managed だけにする（allowManagedPermissionRulesOnly。skill の allowed-tools を効かせない）" || fail "settings に allowManagedPermissionRulesOnly: true が無い"
w=package/skills/worker/SKILL.md
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w"); w3=$(sed -n '/^## 3\. /,/^## 4\. /p' "$w"); w7=$(sed -n '/^## 7\. /,$p' "$w")
grep -q '^| 確認 |.*画面' <<<"$w2" && grep -q '画面に触れる変更は、確認の工程で画面を見て確かめる' <<<"$w2" \
  && grep -q '`127\.0\.0\.1`' <<<"$w2" && grep -q 'playwright-cli open http://127\.0\.0\.1:' <<<"$w2" && grep -q '→ `screenshot`' <<<"$w2" \
  && pass "worker skill §2 の確認の工程に、127.0.0.1 の dev サーバを playwright-cli で確かめスクリーンショットを撮る段がある" || fail "worker skill §2 に画面の確かめの段が無い"
grep -q 'gh pr create --attach' <<<"$w3" && grep -q 'gh pr comment <n> --attach' <<<"$w3" \
  && pass "worker skill §3 に gh pr create／comment --attach がある" || fail "worker skill §3 に --attach が無い"
grep -q '`evidence` には、PR の添付の URL.*撮った command' <<<"$w7" \
  && pass "worker skill §7 の acceptance[].evidence に添付の URL と command" || fail "worker skill §7 に添付の URL と command の段が無い"
grep -q 'screen_paths.*画面の証拠.*major' package/skills/independent-review/SKILL.md \
  && grep -q 'screen_paths.*添付が無い.*（major）' internal/dispatcher/loop.go \
  && pass "independent-review の依頼書の型（skill と dispatcher の reviewerBrief）に画面の証拠の観点（major）がある" || fail "independent-review の型に画面の証拠の観点が無い"
grep -q '人が見るブラウザは無い' package/environment.md && ! grep -qE '(^|[^見る])ブラウザは無い' package/environment.md \
  && grep -q '担当が画面を見て確かめるには `playwright-cli`' package/environment.md \
  && pass "environment.md は「人が見るブラウザは無い」と playwright-cli を案内する" || fail "environment.md の「ブラウザは無い」が直っていない"
# ADR-0020 受け入れ条件 6（決めたこと 6・A-0144 の c2・c3）: 環境メモに memory への書き方
envmem=$(sed -n '/^## 気づきを残す（auto memory）$/,/^## /p' package/environment.md)
miss=""
for k in '`/workspace/memory/`' '蒸留までの受け皿' '週1のトリアージ' '1件1ファイルの `.md` を、フォルダの直下に書く' '確かめた事実と、起きた条件と理由のついた学び' '本文に課題番号（無ければ repo）を書く' '続き・決定・秘密は書かない' '報告の `## Remember` は今までどおり書く' 'Codex の記憶は切ってあり'; do
  grep -qF -- "$k" <<<"$envmem" || miss="$miss $k"
done
[ -z "$miss" ] && pass "environment.md に memory への書き方（受け皿・直下の .md・条件と理由・課題番号・続き/決定/秘密は書かない・Remember は残す。ADR-0020）" || fail "environment.md の memory の節:$miss"

# === 10. 承認の段・書く場所・字数・reach（A-0122。A-0117 の c7・c8・c11・c17） ===
w=package/skills/worker/SKILL.md
w5=$(sed -n '/^## 5\. /,/^## 6\. /p' "$w")
grep -q '^| 承認が要る（「人の go」で受ける） | 承認は要らない（聞かずにする） |$' <<<"$w5" && grep -q 'force-with-lease` も含む' <<<"$w5" \
  && pass "worker skill §5 に承認の要る操作と要らない操作が並ぶ（自分の PR の branch への force-with-lease は要らない側）" || fail "worker skill §5 に承認の段の表が無い"
[ "$(grep -c '\*\*書く場所\*\*' "$w")" = 1 ] && ! grep -q 'worktree の外にファイルを書' "$w" package/templates/*.md \
  && pass "書く場所は worker skill §2 の1か所（skill のほかの節と template に「worktree の外に書かない」が無い）" || fail "書く場所が1か所に寄っていない"
! grep -qE '100字|1〜2行|1〜3 行' "$w" && grep -q '1行に1件（Chief of Staff とトリアージが行ごとに拾う）' "$w" \
  && pass "worker skill の報告の書式に字数の上限が無く、1行に1件と理由がある" || fail "worker skill の報告の書式に字数の上限が残る"
grep -q '`reach`.*指示ではない' "$w" && pass "worker skill に hc の reach は指示ではないとある" || fail "worker skill に reach の扱いが無い"

# === 11. 価値のループ（ADR-0011 受け入れ条件 8・決めたこと 4・7・12・13） ===
ce=package/skills/chief-engineer/SKILL.md
ce_form=$(block "$ce" '# <題>（仕様・設計。')
miss=
grep -q '^## 価値の仮説（下の 6 項目。利用者に見えない題は「無し: <理由>」の1行）$' <<<"$ce_form" || miss="$miss 文書の形の見出し"
i=0
for k in '誰の何が変わるか' '見る信号（' '見る日（絶対の日付' '誰が要るか' '効いたとどう分かるか' '何があれば止めるか'; do
  i=$((i + 1)); grep -q "^$i\. $k" "$ce" || miss="$miss ($i)$k"
done
grep -q '「無し: <理由>」' "$ce" && grep -q '観察の結果が「外れた」「分からない」でも、ラインは何もしない' "$ce" || miss="$miss 無し・ラインは何もしない"
grep -q 'P6 で形を決める\|P6 で形が決まるまで' "$ce" && miss="$miss P6 の仮の行が残る"
[ -z "$miss" ] && pass "chief-engineer skill の文書の形に価値の仮説の 6 項目と「無し: <理由>」がある" || fail "chief-engineer skill の価値の仮説:$miss"
cos=package/skills/chief-of-staff/SKILL.md
cos1=$(sed -n '/^## 1\. /,/^## 2\. /p' "$cos")
miss=
grep -q '「価値の仮説」の節が見出しだけ（空）なら、\*\*切らずに Chief Engineer へ戻す\*\*' <<<"$cos1" || miss="$miss 空なら戻す"
grep -q '`kanban add "観察: <題>の価値の仮説" --parent <Chief Engineer の id>' <<<"$cos1" && grep -q '`kanban set <新しい id> not_before=<(3) の日付>`' <<<"$cos1" || miss="$miss 観察の課題を --parent と not_before で足す"
grep -q '^### `signal` の通知を受けたら$' <<<"$cos1" && grep -q '^1\. \*\*直す課題に切る\*\*.*`links` に信号の `url` と `key`' <<<"$cos1" || miss="$miss signal を直す課題に切る"
grep -q '^### 観察の課題の報告を受けたら$' <<<"$cos1" && grep -q '\*\*ラインは何もしない\*\*' <<<"$cos1" || miss="$miss 観察の報告を人へ返すだけ"
block "$cos" '# <id> 観察: ' | grep -q '^完了条件: report.json の observation' || miss="$miss 観察の課題の雛形"
grep -q '^| `signal` の行' "$cos" || miss="$miss news の表の signal の行"
[ -z "$miss" ] && pass "chief-of-staff skill に、空なら戻す・観察の課題を --parent と not_before で足す・signal を直す課題に切る、の行がある" || fail "chief-of-staff skill の価値のループ:$miss"
grep -q '^| 7\. 課題は台帳が既定 | .*`write_back: true` の出所への `andon issue finish` の1コメントは委譲の中。それ以外のトラッカーへの書き込み（状態の変更・途中のコメント）は人の go' "$cos" \
  && grep -q '`write_back: true` の出所（今は `github-issues`）の issue は、`links` に URL' <<<"$cos1" \
  && pass "chief-of-staff skill の規約7が write_back の1コメントを委譲の中に、ほかの書き込みを人の go に分ける（ADR-0011 決めたこと 15）" \
  || fail "chief-of-staff skill の規約7・links の URL（write_back）"
w=package/skills/worker/SKILL.md
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w")
o_blk=$(block "$w" 'andon issue report "$id" --data '"'"'{"observation"' | head -1 | sed "s/^[^']*'//; s/'.*\$//")
printf '%s\n' "$o_blk" >"$tmp/o.json"
grep -q '^### 観察の課題のとき$' <<<"$w2" && grep -q '\*\*直さない・課題を切らない・出所へ書き戻さない\*\*' <<<"$w2" && grep -q '^2\. `kanban review <id>`' <<<"$w2" \
  && [ -n "$o_blk" ] && jq -e '.observation' "$tmp/o.json" >/dev/null 2>&1 && chk report "$tmp/o.json" \
  && pass "worker skill に観察の課題の節があり、observation の例は report.schema.json に合う" || fail "worker skill の観察の課題: $(head -1 "$tmp/why" 2>/dev/null) $o_blk"

# === 13. herdr notification show の文言（ADR-0007 R3・B7）===
miss=
for f in package/environment.md package/skills/chief-of-staff/SKILL.md; do
  grep -n 'herdr notification show' "$f" | grep -q 'TUI' || miss="$miss $f"
  grep -q '台帳の問いとボードで足りる' "$f" || miss="$miss $f(足りる)"
done
[ -z "$miss" ] && pass "environment.md と chief-of-staff skill が herdr notification show を「TUI が動いているときだけ」と書く" || fail "herdr notification show の文言:$miss"

# hc の手引き（A-0192）: 全役が読む environment.md に「hc --skill を読む」の1行がある
grep -q 'skill に書いていない `hc` の操作が要るときだけ、先に `hc --skill`' package/environment.md \
  && sed -n '/^## 0\. 起きたとき/,/^毎ターン/p' package/skills/chief-of-staff/SKILL.md | grep -q '`hc --skill` を1回読む' \
  && pass "environment.md は要るときだけ、chief-of-staff skill §0 は起きたときに hc --skill を読む" || fail "hc --skill を読む行（environment.md か chief-of-staff §0）が無い"

# === 14. 受け入れ条件の証拠の質（ADR-0007 R5・A4） ===
miss=
c4=$(sed -n '/^## 4\. /,/^## 5\. /p' docs/contract.md | grep '^| `acceptance\[\]` |')
for f in "$c4" "$(grep '^\*\*受け入れ条件の節\*\*' "$ce")" "$(grep '^- `acceptance`:' "$w")"; do
  grep -q 'テストの command と' <<<"$f" && grep -q '出力' <<<"$f" && grep -q 'staged な）出力' <<<"$f" && grep -q 'mock に置き換えたテスト' <<<"$f" && grep -q '証拠でない' <<<"$f" \
    || miss="$miss [$(cut -c1-30 <<<"$f")]"
done
[ -z "$miss" ] && pass "contract.md §4 の acceptance[] の行・CE skill の受け入れ条件の節・worker skill §7 に、証拠はテストの command と出力のパスで staged な出力と mock は証拠でない、とある" || fail "受け入れ条件の証拠の質:$miss"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
