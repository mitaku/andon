#!/usr/bin/env bash
# tests/package.sh — package/ の skill・プロファイル・ゲートの条件表・managed settings・statusline（design.md §2.2・§2.4）。
#   1. skill 8つとプロファイル 8つがそろい、frontmatter の name がディレクトリ名・ファイル名と同じ。
#   2. plugin.json の name が andon。Claude Code 系のプロファイルは command で plugin と役の skill を運び（本文で読ませない）、Codex 系は本文で skill を指す。
#  2b. 前の置き場の名前が残っていない（ADR-0004 受け入れ条件 4 の前半）。
#   3. skill とプロファイルに仮設の語が残っていない（用語集の「仮設の語」の節だけは対応表として持つ）。
#   4. gate.json が schema/gate.schema.json の条件表に合う。settings.json が JSON で、statusLine が package の statusline を指す。全フックの command が package に在るファイルだけを指す。
#      package/hooks/ の全部のフックの最初の実行行が「andon が無ければ通す」の行（ADR-0034 受け入れ条件 3）。
#      Codex の rules（package/codex.rules）の forbidden が settings の deny（コマンドの頭で引く線）と同じ。
#      settings の deny が人のファイル（ADR-0005 決めたこと 4 の (c)）の書き換えを Edit(//workspace/…) で拒む。
#      settings の deny が force-with-lease で main に当たる形を断り、自分の branch への形は allow のまま（当たる・当たらないの例）。
#   5. statusline が JSON を読んで1行を出す。
#   6. skill の機械向けの手順が command（andon issue …）を指し、手で JSON を書く手順が残っていない。例の問いと予約キーは schema に通す。
#   7. 課題（issue）への改名（ADR-0008 受け入れ条件 5）: glossary の項・欄名・skill と template に旧語が無い。
#   8. 役の略称と人への書き方（ADR-0013 受け入れ条件 1・2・7・8 の用語集の分・10）: 用語集の CE・作った語、package/writing.md の書き方の11行・但し書き・語の決まり3つ・用語集のパス、
#      用語集は書き方の節と語の決まりを持たず writing.md を指す、CE skill の問いの出し方、ask-one-by-one の読み手。
#   9. 画面の確かめ（ADR-0018 受け入れ条件 4）: managed だけの許可・worker skill §2・§3・§7・independent-review の観点・environment.md。
#  10. 承認の段・書く場所・字数・reach（A-0122）。10b. Chief of Staff の行数・承認の段・起動の go・続きの文（A-0123）。
#  12. auto memory（ADR-0020 受け入れ条件 2・6。4 と 6 の節の中）: settings が入れる向き・環境メモの書き方・chief-of-staff の規約9・トリアージ・wiki の手引き。
#  11. 価値のループ（ADR-0011 受け入れ条件 8）: CE skill の価値の仮説の 6 項目・CoS skill の空なら戻す・観察の課題・signal の受け方・worker skill の観察の課題（observation の例は schema に通す）。
#      CoS の規約7（write_back の出所への finish の1コメントは委譲の中。決めたこと 15）。
#  17. CoS の原則と手引き（A-0195）: SKILL.md は役・原則・手引きの目次・手引きの扱い・規約の行き先だけで 10KB 以下、目次の手引きが全部在り、手引きの先頭に「読む時」と「消す条件」。
#  13. herdr notification show の文言（ADR-0007 R3）。
#  14. 受け入れ条件の証拠の質（ADR-0007 R5）: contract.md §4 の acceptance[] の行・CE skill・worker skill §7 に、証拠はテストの command と出力のパスで staged な出力と mock は証拠でない。
#  15. Advisor（ADR-0022 受け入れ条件 1・2。6 の節の中）: --advisor fable は chief-of-staff の command だけ、CoS skill の「人の入口」に相談する場面の1行。
#  16. 日報（ADR-0024 受け入れ条件 A1・A2）: worker skill の日報の節と工程表の振り返り、CoS skill の差し戻しの1行、用語集の「日報」。
#  18. ADR の索引と書き分け（ADR-0033 受け入れ条件 9）: CE skill の文書の形と design.md §1.8 の題の次に ## 決定の索引、索引の列と語・細部の書き分け・実装の PR が索引の行を直すこと、worker §3 の1行。
#  19. 振り返りの手順（ADR-0035 受け入れ条件 12）: retro skill に AGENTS.md の表を読む手順と、検査を足す直しの「誤りを戻すと落ちる」の受け入れ条件を入れる手順。
#  20. PR を出す前の3つの観点: worker skill §2 の観点（失敗の波及・前の版の形・落ちたあとの起き直し）がレビュアーの依頼書（reviewerBrief・independent-review）の「正しさ」の語で書かれ、§2 の工程表・§4・§7 の書式にある。
#  21. 当てたあとの検証（ADR-0035 受け入れ条件 13 の後半）: CoS の手引き verify.md に、差分が地図に当たるときだけ切る・同時に1つ・前の検証が見た commit から見る手順があり、名指す検証の skill と features.sh が在る。
#  22. pstack と yomiyasu の既定（ADR-0038 受け入れ条件 1・3 の一部）: settings の marketplace の登録と自動更新と pstack の有効化、gh-skills.txt の yomiyasu、「既定は空」の文、writing.md の推敲の節、環境メモの pstack の行。
#  23. yomiyasu に観点の語を言い換えさせない: .yomiyasu-words/allow.txt の「正から写す語」の節が、レビュアーの依頼書（reviewerBrief の「見てほしいこと」）と worker skill §2 の観点の見出しの語と同じで、報告の言い方（外向きの操作・前の版）を含み、用語集の語を含まない。
#  24. 役の名前（ADR-0036 受け入れ条件 1・3）: 用語集の役の項が「セルのオーナー」、「**人**:」の項と「オーナー（人）」が package/ に無い、CoS の規約 10 が持ち場の CoS を指す。
# 道具は bash・jq・python3（package/lib/andon/schema_check.py）。
set -u
cd "$(dirname "$0")/.." || exit 2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fails=0
pass() { echo "ok   $1"; }
fail() { echo "FAIL $1" >&2; fails=$((fails + 1)); }

skills="ask-one-by-one chief-engineer chief-of-staff glossary independent-review retro triage worker"
profiles="chief-engineer chief-of-staff claude claude-review codex codex-review fable sonnet"

# frontmatter の key の値（最初の --- から次の --- まで）
fm() { awk -v k="$2" 'NR==1 && $0!="---" {exit} NR>1 && $0=="---" {exit} NR>1 && index($0, k": ")==1 {print substr($0, length(k)+3); exit}' "$1"; }

# === 1. 顔ぶれと name ===
got=$(cd package/skills && ls -1 | sort | tr '\n' ' ' | sed 's/ $//')
[ "$got" = "$skills" ] && pass "package/skills は8つ（$skills）" || fail "package/skills の顔ぶれ: $got"
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
for f in package/skills/*/SKILL.md package/skills/*/playbooks/*.md package/skills/*/assets/* package/templates/*.md; do
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
for p in 'projects/*/project.json' gate.local.json andon.toml '**/AGENTS.local.md' '**/AGENTS.worker.md'; do
  jq -e --arg p "$p" '.tier_c_paths | index($p)' package/gate.json >/dev/null \
    && pass "人のファイル $p はレベル C" || fail "$p が tier_c_paths に無い"
done
if jq -e . package/settings.json >/dev/null 2>&1; then pass "package/settings.json は JSON"; else fail "package/settings.json が JSON でない"; fi
jq -e '.permissions.deny | index("Bash(gh pr merge:*)")' package/settings.json >/dev/null && pass "settings は gh pr merge を deny" || fail "settings の deny に gh pr merge が無い"
# commit と PR の帰属を消すのは attribution（includeCoAuthoredBy は v2.0.62 から非推奨。false は v2.1.281 から。A-0204）
jq -e '.attribution == false and (has("includeCoAuthoredBy") | not)' package/settings.json >/dev/null \
  && pass "settings は attribution: false で帰属を消す（includeCoAuthoredBy を使わない）" || fail "settings の帰属の消し方が attribution: false でない"
# ENABLE_TOOL_SEARCH は managed settings の env の1か所（Claude Code の起動のしかたによらず届く。A-0204）
jq -e '.env.ENABLE_TOOL_SEARCH == "auto"' package/settings.json >/dev/null && ! grep -q ENABLE_TOOL_SEARCH image/Dockerfile image/sshd_config.d/andon.conf \
  && pass "ENABLE_TOOL_SEARCH は settings の env だけに置く" || fail "ENABLE_TOOL_SEARCH が settings の env に無いか、image の ENV・sshd の SetEnv に残る"
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
# 末尾の :* は「 *」と読まれるので、refspec の「:<ref>」で終わる形は末尾を :** と書く（* が2つ続いても任意の文字列）
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
              "git push --force-with-lease origin HEAD:andon/A-0001-maintenance",
              "git push --force-with-lease origin andon/A-0001-delete-tags"],
    "deny": ["git push --force-with-lease origin main", "git push origin main --force-with-lease",
             "git push --force-with-lease origin HEAD:main", "git push origin HEAD:main --force-with-lease",
             "git push --force-with-lease origin andon/A-0001-x:main", "git push --force-with-lease origin +HEAD:main",
             "git push --force-with-lease origin +main", "git push --force-with-lease origin refs/heads/main",
             "git push --force-with-lease origin HEAD:refs/heads/main", "git push origin HEAD:refs/heads/main --force-with-lease",
             "git push --force-with-lease=main:abc123 origin HEAD:main", "git push --force-with-lease origin main andon/A-0001-x",
             "git push --force-with-lease --all origin", "git push --all --force-with-lease", "git push --mirror --force-with-lease origin",
             # --force は許さない・消す形と tag の形も通さない（A-0200）
             "git push --force-with-lease --force", "git push --force-with-lease --force origin HEAD", "git push --force-with-lease origin HEAD --force",
             "git push --force-with-lease -f origin HEAD", "git push --force-with-lease origin HEAD -f",
             "git push --force-with-lease --delete origin andon/A-0001-x", "git push --force-with-lease origin --delete andon/A-0001-x",
             "git push --force-with-lease -d origin andon/A-0001-x", "git push --force-with-lease origin :andon/A-0001-x",
             "git push --force-with-lease origin +:andon/A-0001-x", "git push --force-with-lease --prune origin HEAD",
             "git push --force-with-lease --tags", "git push --force-with-lease origin --tags", "git push --force-with-lease --follow-tags origin HEAD",
             "git push --force-with-lease origin tag v1.0.0", "git push --force-with-lease origin refs/tags/v1.0.0",
             "git push --force-with-lease origin HEAD:refs/tags/v1.0.0",
             # allow は「git push --force-with-lease」と「git push --force-with-lease origin <1語>」だけ。1語は旗でも + でもない（A-0200 のレビュー）
             "git push --force-with-lease origin -fu",
             "git push --force-with-lease origin -uf HEAD", "git push --force-with-lease origin HEAD -vf",
             "git push --force-with-lease origin +v1.0", "git push --force-with-lease origin +HEAD:andon/A-0001-x",
             "git push --force-with-lease origin HEAD andon/A-0001-x"],
    # origin より前に旗を置く形は allow に当たらず、人に聞く（-fu も --force-if-includes も）
    "ask": ["git push --force-with-lease upstream HEAD", "git push --force-with-lease -u origin HEAD",
            "git push --force-with-lease -fu origin HEAD:refs/heads/andon/review", "git push --force-with-lease --force-if-includes origin HEAD"],
}
for want, cmds in cases.items():
    for c in cmds:
        got = verdict(c)
        print(("ok   " if got == want else "FAIL ") + f"force-with-lease の許可ルール: {c} → {got}（期待 {want}）")
# * の無い規則が、同じ側の別の規則（末尾の「 *」は素の命令にも当たる）に含まれていない（A-0204）
for kind in ("allow", "deny"):
    rs = rules(kind)
    dup = [f"{r} ⊂ {s}" for r in rs if "*" not in r for s in rs if s != r and hit(s, r)]
    print(("FAIL " if dup else "ok   ") + f"settings の {kind} に別の規則で済む行が無い" + (f": {dup}" if dup else ""))
# 末尾の「 *」が素の命令にも当たる例（この検査が重なりを見ていることの確かめ）
print(("ok   " if verdict("git diff") == "allow" and verdict("git status") == "allow" else "FAIL ") + "素の git diff・git status は「 *」の規則で allow")
EOF
)
while IFS= read -r l; do
  case "$l" in "ok   "*) pass "${l#ok   }" ;; *) fail "${l#FAIL }" ;; esac
done <<<"$out"
# 人のファイル（ADR-0005 決めたこと 4 の (c)）への書き換えを deny する。今の Claude Code は Write・NotebookEdit・MultiEdit を Edit(path) の規則で引き、
# Write(path) の規則は受け付けても当たらない。絶対パスは //（単独の / は設定の置き場からの相対）。Codex には同じ線を引けない（docs/image.md）
for f in andon.toml AGENTS.md AGENTS.worker.md gate.local.json 'templates.local/**' 'local/**' 'bin/**' 'projects/*/project.json'; do
  jq -e --arg r "Edit(//workspace/$f)" '.permissions.deny | index($r)' package/settings.json >/dev/null \
    && pass "settings は人のファイル /workspace/$f の書き換えを deny" || fail "settings の deny に Edit(//workspace/$f) が無い"
done
# 根の CLAUDE.md・.claude/CLAUDE.md は .worktrees の下の担当にも届き、repo の AGENTS.md が読まれなくなる。書き込みを deny する
for f in CLAUDE.md .claude/CLAUDE.md; do
  jq -e --arg r "Edit(//workspace/$f)" '.permissions.deny | index($r)' package/settings.json >/dev/null \
    && pass "settings は根の /workspace/$f の書き込みを deny" || fail "settings の deny に Edit(//workspace/$f) が無い"
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
# プロバイダに切られた Chief of Staff を起動する（ADR-0019 決めたこと 10）: StopFailure に asyncRewake のフックが、最後の待ち（8 分）より長い timeout つきで在る
ft=$(jq -r '[.hooks.StopFailure[].hooks[] | select(.command == "/opt/andon/hooks/stop-failure" and .asyncRewake == true and (.timeout | type) == "number")][0].timeout // empty' package/settings.json)
[ -n "$ft" ] && [ "$ft" -gt 480 ] && [ -x package/hooks/stop-failure ] && sh -n package/hooks/stop-failure && grep -q '^exec andon dispatcher rewake$' package/hooks/stop-failure \
  && pass "StopFailure は asyncRewake のフック（timeout $ft 秒。8 分の待ちより長い）で andon dispatcher rewake を打つ" || fail "StopFailure に asyncRewake の rewake のフックが無い・timeout が 8 分以下"
# フックの 1 行目（ADR-0034 決めたこと 8・受け入れ条件 3）: package/hooks/ の全部のフックで、最初の実行行が「andon が無ければ通す」の行。
# どこかにあることではなく、最初の実行行であること。1 行目の #!・コメントの行・空行は飛ばす。フックの名前は書かない（足したフックも見る）
guard='command -v andon >/dev/null 2>&1 || exit 0'
first_exec() { awk 'NR == 1 && /^#!/ {next} /^[[:space:]]*(#|$)/ {next} {print; exit}' "$1"; }
n=0
for h in package/hooks/*; do
  [ -f "$h" ] || continue
  n=$((n + 1))
  got=$(first_exec "$h")
  [ "$got" = "$guard" ] && pass "$h の最初の実行行は「$guard」" || fail "$h の最初の実行行が「$guard」でない: $got"
done
[ "$n" -gt 0 ] && pass "package/hooks/ のフック $n 本の最初の実行行を見た" || fail "package/hooks/ にフックが無い"
# 検査が見ていることの確かめ: 守りの行の前に別の実行行を足した写しは当たらず、守りの行が後ろにあるだけでは通らない
awk -v g="$guard" '$0 == g {print "in=$(cat)"} {print}' package/hooks/stop > "$tmp/hook-moved"
grep -qxF "$guard" "$tmp/hook-moved" && [ "$(first_exec "$tmp/hook-moved")" != "$guard" ] \
  && pass "守りの行の前に実行行を足したフックは、最初の実行行の検査に当たらない（検査が見ている）" || fail "フックの最初の実行行の検査が、前に足した実行行を見ていない"

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
out=$(printf '%s' '{"model":{"display_name":"Sample"},"context_window":{"used_percentage":42},"cwd":"/tmp","cost":{"total_duration_ms":3725000},"effort":{"level":"high"}}' | HOME=$tmp bash package/lib/andon/statusline)
case $out in *1h2m*high*) pass "statusline は入力の cost.total_duration_ms と effort.level を出す" ;; *) fail "statusline の時間と effort: $out" ;; esac
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
  && pass "worker skill の報告の書式に ## 計画・探した範囲・計画との差がある" || fail "worker skill の報告の書式に ## 計画・探した範囲・計画との差が無い"
# 調べる課題の候補の例（andon issue candidates の stdin）は candidates.schema.json に合う。scope の問いは worker skill が案内する
c_blk=$(block "$w" 'andon issue candidates "$id" -' | sed '1d;$d')
printf '%s\n' "$c_blk" >"$tmp/c.json"
[ -n "$c_blk" ] && chk candidates "$tmp/c.json" && pass "worker skill の候補の例は candidates.schema.json に合う" || fail "候補の例: $(head -1 "$tmp/why" 2>/dev/null) $c_blk"
grep -q 'kind: scope' "$w" && grep -q 'andon issue candidates' "$w" && pass "worker skill は scope の問いと andon issue candidates を案内する" || fail "worker skill に scope の問いか andon issue candidates が無い"

# Chief of Staff の skill は原則（SKILL.md）と手引き（playbooks/*.md）に分かれる。検査は中身の在る手引きを見る
cos=package/skills/chief-of-staff/SKILL.md pb=package/skills/chief-of-staff/playbooks
cosall=$(cat "$cos" "$pb"/*.md)
block "$pb/brief.md" '# <id> <題>' | grep -q '^横展開: ' && block "$pb/brief.md" '# <id> 調べる: ' | grep -q '^終了条件: ' \
  && pass "Chief of Staff の依頼書の雛形に 横展開: の行、調べる課題の雛形に 終了条件: の行がある" || fail "chief-of-staff の雛形に 横展開: か 終了条件: が無い"

block "$pb/brief.md" '# <id> 調べる: ' | grep -q '^- Deliverable: candidates' \
  && pass "調べる課題の雛形の Settings に Deliverable: candidates がある" || fail "調べる課題の雛形に Deliverable: candidates が無い"

# ADR-0009 受け入れ条件 3: オーナーの入口・打つコマンド・依頼書の原則と範囲の外・差し戻し・wiki の節
grep -q '^1\. \*\*自分で作らない\*\*。.*自分で Chief Engineer を起動して経緯を渡す' "$cos" \
  && [ "$(grep -m1 '^## ' "$pb/cut.md")" = '## オーナーの入口' ] && block "$pb/cut.md" 'andon chief-engineer --brief - "<題>"' | grep -q '^経緯: ' \
  && ! grep -qE 'andon chief-engineer[^。]*を勧め|題としてオーナーに勧め|オーナーに題を勧め' <<<"$cosall" \
  && pass "Chief of Staff の原則1が自分で Chief Engineer を起動すると言い、切る手引きの先頭は「オーナーの入口」で andon chief-engineer --brief - を自分で打つ（オーナーに勧めない）" || fail "chief-of-staff の原則1か cut.md の先頭に「オーナーの入口」が無いか、andon chief-engineer をオーナーに勧める文が残る"
grep -q '^- 打つコマンドは .*`andon chief-engineer`' "$pb/wake.md" \
  && pass "Chief of Staff の打つコマンドの一覧に andon chief-engineer がある" || fail "chief-of-staff の打つコマンドの一覧に andon chief-engineer が無い"
sed -n '/^\*\*欄の読み方\*\*/,/^| 欄 |/p' "$pb/brief.md" | grep -q 'repo を読めば分かること（作法・構造・直し方）は書かず' \
  && grep -q '^| 守ること | 範囲の外（今回やらないこと）' "$pb/brief.md" \
  && pass "依頼書の欄の読み方の頭に原則の1行、守ることの説明に 範囲の外" || fail "brief.md に依頼書の原則か 範囲の外 が無い"
grep -q '^8\. \*\*2回通らなければ、手直しを重ねない\*\*' "$cos" \
  && grep -q '## 差し戻し（<日付>）' "$pb/gate.md" && grep -q '`## 差し戻し` が2つあってまだ通らない' "$pb/gate.md" && grep -q '依頼書を直した新しい課題を `kanban add`' "$pb/gate.md" \
  && pass "差し戻しは依頼書への追記で数え、2回で通らなければ新しい課題に切る" || fail "chief-of-staff に差し戻しの追記か切り直しの手順が無い"
cos11=$(cat "$pb/wiki.md")
# ADR-0016 受け入れ条件 1: 層・3つの操作・元資料の3種・移行があり、前の形（1ファイル1件・log.md は作らない）が無い
miss=""; for k in '何を' '層' '元資料を置くとき' 'いつ（ingest）' 'query' 'lint' '移行'; do grep -q "^- \*\*$k\*\*" <<<"$cos11" || miss="$miss $k"; done
for k in '`raw/`' '`sources/`' '`entities/`' '`topics/`' '`overview.md`' '`index.md`' '`log.md`'; do grep -q "^  - $k — " <<<"$cos11" || miss="$miss 層$k"; done
for k in 'オーナーの発言' '外の資料' '課題の記録' 'memory'; do grep -q "^  - $k: " <<<"$cos11" || miss="$miss 元資料:$k"; done
grep -qF '## [YYYY-MM-DD] ingest | <題>' <<<"$cos11" && grep -q 'ingest 約10件ごと' <<<"$cos11" && grep -q '`index.md` → ページの順' <<<"$cos11" || miss="$miss log の形・lint の時期・query の順"
grep -q 'commit --only -m "wiki: <要点>" -- wiki' <<<"$cos11" || miss="$miss commit（--only -- wiki）"
grep -q '書く前に秘密.*`<伏せた: 種類>`' <<<"$cos11" || miss="$miss raw/ に書く前に秘密を伏せる"
grep -qE '1ファイル1件|`log.md` は作らない' <<<"$cos11" && miss="$miss 前の形の行が残る"
[ -z "$miss" ] && pass "Chief of Staff の skill の wiki の節が LLM Wiki の形（層・ingest・query・lint・元資料・移行。ADR-0016）" || fail "chief-of-staff の wiki の節:$miss"
# ADR-0020 受け入れ条件 6: 規約9は memory を受け皿とし、トリアージが wiki 行きの memory を raw/memory/ に写し、wiki の raw/ は4種
grep -q '^| 9\. 記憶の規律 | 11 | `wiki.md` |$' "$cos" && grep -q '^11\. \*\*学びは層で分けて残す\*\*。.*memory は蒸留までの受け皿' "$cos" \
  && grep -q '^記憶の規律（規約9）: memory（auto memory。`/workspace/memory/`）は蒸留までの受け皿。残すものはトリアージを通して wiki と repo へ' "$pb/wiki.md" \
  && ! grep -q 'ハーネス内蔵のメモリは使わない' <<<"$cosall" \
  && grep -q '`close`「wiki へ: <題>」のものは、記録.*の本文を言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写して ingest' "$pb/triage.md" \
  && grep -q '^  - `raw/` — 元資料。書いたら変えない。4種: .*(4) トリアージで wiki 行きになった memory の写し（`raw/memory/' <<<"$cos11" \
  && pass "Chief of Staff の skill: 原則11・規約9・トリアージ・wiki の手引きが memory を蒸留までの受け皿として扱う（ADR-0020）" || fail "chief-of-staff の原則11・規約9・triage.md・wiki.md が ADR-0020 の形でない"
# ADR-0016 受け入れ条件 3: 読む側（Chief Engineer の「読む」と Chief of Staff の切る手引き）が index.md → ページの順で引く
grep -q '^1\. \*\*読む.*セルの wiki（`index.md` → ページの順で引く。使う事実は' package/skills/chief-engineer/SKILL.md \
  && grep -q 'wiki を引く（`wiki.md` の query。`index.md` → ページの順）' "$pb/cut.md" \
  && pass "Chief Engineer の「読む」と Chief of Staff の cut.md が wiki を index.md → ページの順で引く" || fail "読む側が index.md → ページの順を指していない"
# ADR-0009 受け入れ条件 4: Chief of Staff の template が wiki を道具の届く場所に足す
case " $(fm package/templates/chief-of-staff.md command) " in *" --add-dir /workspace/wiki "*) pass "Chief of Staff の template の command に --add-dir /workspace/wiki" ;; *) fail "Chief of Staff の template に --add-dir /workspace/wiki が無い" ;; esac
# ADR-0022 受け入れ条件 1・2: Advisor（Fable）は Chief of Staff だけ。skill の「オーナーの入口」に相談する場面の1行
case " $(fm package/templates/chief-of-staff.md command) " in *" --advisor fable "*) pass "Chief of Staff の template の command に --advisor fable" ;; *) fail "Chief of Staff の template に --advisor fable が無い" ;; esac
left=
for p in $profiles; do
  [ "$p" = chief-of-staff ] && continue
  for field in command resume; do
    case " $(fm "package/templates/$p.md" "$field") " in *--advisor*) left="$left $p.$field" ;; esac
  done
done
[ -z "$left" ] && pass "chief-of-staff のほかのプロファイルの command・resume に --advisor が無い" || fail "--advisor が付いている:$left"
sed -n '/^## オーナーの入口$/,/^## 切り方$/p' "$pb/cut.md" | grep 'advisor に相談してから決める' | grep -q '振り分けに迷うとき' \
  && sed -n '/^## オーナーの入口$/,/^## 切り方$/p' "$pb/cut.md" | grep 'advisor に相談してから決める' | grep -q '2回差し戻して切り直すとき' \
  && pass "Chief of Staff の skill の「オーナーの入口」に advisor に相談する場面（振り分けに迷うとき・2回差し戻して切り直すとき）" || fail "Chief of Staff の skill の「オーナーの入口」に advisor の1行が無い"

# Chief of Staff の issue.yaml の予約キー: 全部書き、issue-keys.schema.json に合う
set_line=$(block "$pb/brief.md" '# issue.yaml の予約キー' | grep '^kanban set ')
printf '%s\n' "$set_line" | tr ' ' '\n' | grep '=' | jq -R 'split("=") | {(.[0]): .[1]}' | jq -s add >"$tmp/keys.json"
if chk issue-keys "$tmp/keys.json" && jq -e 'keys == ["estimate","merge","must_fix","review","review_rounds","reviewer"]' "$tmp/keys.json" >/dev/null; then
  pass "Chief of Staff は予約キー6つ（review・must_fix・reviewer・review_rounds・merge・estimate）を issue.yaml に書く"
else fail "Chief of Staff の kanban set: $set_line"; fi
# ADR-0014 受け入れ条件 7（E1）: 雛形の Settings に Estimate の行（段の名）、起き直しで打ち直す予約キーに estimate
block "$pb/brief.md" '# <id> <題>' | grep -qx -- '- Estimate: 中' \
  && block "$pb/brief.md" '# <id> 調べる: ' | grep -qx -- '- Estimate: 小' \
  && grep -q '同じ値で `kanban set <id> review=… .*`estimate`' "$pb/wake.md" \
  && pass "Chief of Staff の雛形に Estimate の行があり、起き直しで estimate も打ち直す" || fail "chief-of-staff に Estimate の行か起き直しの estimate が無い"
# ADR-0014 受け入れ条件 5・7（E4）: worker の報告の雛形の ## 計画 に見込みの反論の1行、--data に estimate（引き継ぐ）
grep -q '^- 見込み: <段>（依頼書は <段>）。<理由>' package/skills/worker/SKILL.md \
  && grep -q '`estimate`（§2 の見込みの反論）も `--data` に無ければ前の `report.json` から引き継ぐ' package/skills/worker/SKILL.md \
  && pass "worker の報告の ## 計画 に見込みの反論の1行があり、report.json の estimate は引き継ぐ" || fail "worker に見込みの反論の1行か estimate の引き継ぎが無い"
# ADR-0017 受け入れ条件 8（P4）: worker は ## 計画 を report.json の plan[] に写し、plan は引き継ぐ
grep -q '`plan`（`## 計画` の写し）も `--data` に無ければ前の `report.json` から引き継ぐ' package/skills/worker/SKILL.md \
  && grep -q '^- `plan`: `## 計画` の作業を1件ずつ `{"id": "p1", "text": ".*", "done": false}` で写す' package/skills/worker/SKILL.md \
  && pass "worker は ## 計画 を report.json の plan[] に写し、plan は引き継ぐ" || fail "worker に plan[] の写しか引き継ぎが無い"
grep -q '`auto` は\*\*委譲がこのプロジェクトのマージを任せているときだけ\*\*' "$pb/brief.md" \
  && pass "merge: auto は委譲の範囲内だけ" || fail "chief-of-staff に merge: auto の条件が無い"
# インナーループは dispatcher（P6a）: Chief of Staff の skill のレビューとゲートの節はオーナーの判断が要るときだけ。レビュアーを手で起動する手順が残っていない
cos6=$(cat "$pb/gate.md")
grep -q '^3\. \*\*作った者に合否を決めさせない\*\*。.*指摘を中継しない・ゲートを手で回さない・マージしない' "$cos" \
  && grep -q '^# レビューとゲート（オーナーの判断が要るときだけ）' <<<"$cos6" && grep -q '^## `needs_human` のとき' <<<"$cos6" \
  && ! grep -qE 'andon issue start "\$new"|worktree add --detach|andon gate <id>' <<<"$cosall" \
  && ! grep -qE 'andon issue start "\$new"|worktree add --detach|andon gate <id>' <<<"$cos6" \
  && ! grep -qE 'worktree add --detach|andon issue start "\$new"' package/skills/independent-review/SKILL.md \
  && pass "Chief of Staff の原則3とゲートの手引きは needs_human のときだけ。レビュアーを起動するのとゲートは dispatcher" || fail "chief-of-staff・independent-review にレビュアーを手で起動する手順・ゲートを手で回す手順が残る"
grep -q '元の課題（`issue.yaml` の `parent`）の担当だけ' package/skills/worker/SKILL.md && grep -q '結果の宛先は元の課題の担当だけ' package/skills/independent-review/SKILL.md \
  && pass "レビュアーの結果の宛先は parent の課題の担当だけ（Chief of Staff へ送らない）" || fail "レビュアーの宛先が parent の課題の担当に固定されていない"

# Chief Engineer: worktree で文書の PR と report.json を同じ課題から返す。問いは問いの JSON
ce=package/skills/chief-engineer/SKILL.md
grep -q 'cwd は案件の repo の自分の worktree' "$ce" && grep -q '`pr`＝文書の PR' "$ce" && ! grep -q 'issues/<id>/library/<題' "$ce" \
  && pass "Chief Engineer は worktree で文書の PR と report.json を返す" || fail "chief-engineer の渡し方"
grep -q 'worker skill §5 の1の手順で1問ずつ問いの JSON' "$ce" && grep -q 'andon issue ask "$id" -' package/skills/ask-one-by-one/SKILL.md \
  && pass "Chief Engineer と Chief of Staff の問いは問いの JSON の手順を使う" || fail "問いの JSON の手順を指していない"
# ADR-0009 受け入れ条件 5: WIP の行が無い・読むに価値の仮説の結果・文書の形に価値の仮説の見出し・言い直すで決断が無ければ next で返す
! grep -qE '1プロジェクトに1題|前の題が終わらないまま' "$ce" package/skills/glossary/SKILL.md \
  && pass "Chief Engineer の skill と用語集に WIP の行（1プロジェクトに1題・前の題が終わらないまま）が無い" || fail "chief-engineer か用語集に WIP の行が残る"
ce1=$(sed -n '/^1\. \*\*読む/,/^2\. /p' "$ce"); ce2=$(sed -n '/^2\. \*\*入口: 言い直しと前提/,/^3\. /p' "$ce")
grep -q '前の価値の仮説の結果' <<<"$ce1" && block "$ce" '# <題>（仕様・設計' | grep -q '^## 価値の仮説' \
  && pass "Chief Engineer の「読む」に前の価値の仮説の結果、文書の形に ## 価値の仮説 の見出し" || fail "chief-engineer に価値の仮説の見出しが無い"
grep -q '決断が無ければ文書を書かずに返す' <<<"$ce2" && grep -q '"to": "chief-of-staff", "text": "直す課題に切る: ' <<<"$ce2" && grep -q 'kanban review <id>' <<<"$ce2" \
  && pass "Chief Engineer は入口の「言い直しと前提」で決断が無ければ文書を書かず next で返す" || fail "chief-engineer の入口の「言い直しと前提」に文書なしで返す段が無い"
# CE の推奨案はオーナーに迎合させない（docs/decisions/2026-11-20-ce-recommend-own-best.md）。4点は役の段落に残り、
# 「決めるのはオーナー」は線の中の決定のこと。作り方の決定は CE が決め、古い「決めない」の文は無い（ADR-0037 受け入れ条件 1）
ce0=$(sed -n '/^\*\*あなたは Chief Engineer/,/^- 課題は/p' "$ce")
old_rule='推奨案は出すが決めない|自分で答えて先へ進んだら骨格の違反'
grep -qF '自分のプロダクトだと思って最適と考える案にする' <<<"$ce0" && grep -qF 'オーナーの好みに合わせて推奨を変えない' <<<"$ce0" \
  && grep -qF '事実か理由で推奨の前提を崩したときだけ' <<<"$ce0" && grep -qF '線の中の決定は、推奨を変えなくても決めるのはオーナー' <<<"$ce0" \
  && grep -qF '作り方の決定はあなたが決め、下の「オーナーに聞く線」の中の決定はオーナーが決める' <<<"$ce0" \
  && ! grep -qE "$old_rule" "$ce" \
  && pass "ADR-0037 A1: CE の役の段落は作り方の決定を CE が、線の中の決定をオーナーが決めると言い、推奨の立て方の4点が残り、「推奨案は出すが決めない」が無い" \
  || fail "ADR-0037 A1: chief-engineer の役の段落に線の分け方か推奨の立て方の4点が無いか、「推奨案は出すが決めない」「骨格の違反」の文が残る"
# 否定の検査が当たる場合があること: 古い skill の行には同じ grep が当たる
grep -qE "$old_rule" <<<'   - **推奨案は出すが決めない**。自分で答えて先へ進んだら骨格の違反。' \
  && pass "ADR-0037 A1: 古い「決めない」の行は否定の grep に当たる（検査が見ている）" || fail "ADR-0037 A1: 否定の grep が何も見ていない"
# ADR-0037 受け入れ条件 2: 線の表（a〜g）と「どちらとも読める決定は Chief Engineer が決める」
cel=$(sed -n '/^## オーナーに聞く線$/,/^## 骨格/p' "$ce")
miss=
for r in 'a business' 'b users' 'c promise' 'd irreversible' 'e line'; do
  grep -q "^| ${r% *} | .* | \`${r#* }\` |\$" <<<"$cel" || miss="$miss [${r% *}]"
done
grep -q '^| f | オーナーが前に決めたことを覆すこと | ' <<<"$cel" || miss="$miss [f]"
grep -q '^| g | 事実で決まらず、答えで設計の大半が分かれる不明点 | ' <<<"$cel" || miss="$miss [g]"
grep -qF '**どちらとも読める決定は Chief Engineer が決める。**' <<<"$cel" || miss="$miss [どちらとも読める]"
grep -qF '印の無い過去の決定は、オーナーが決めたものとして読む' <<<"$cel" || miss="$miss [印の無い過去の決定]"
[ -z "$miss" ] && pass "ADR-0037 A2: CE skill に線の表（a〜g）と「どちらとも読める決定は Chief Engineer が決める」がある" || fail "ADR-0037 A2: CE skill の線:$miss"
# ADR-0037 受け入れ条件 3: 骨格は入口（言い直しと前提を1問、続けて範囲）・真ん中（聞くのは g だけ）・出口（要点・線の中の決定・最後の確かめ）
miss=
for k in '^## 骨格（この順で。オーナーと話すのは 2・3 の入口と 7 の出口）$' \
  '^2\. \*\*入口: 言い直しと前提を1問で確かめる。\*\*' '^3\. \*\*入口: 続けて範囲を聞く。\*\*' \
  '^4\. \*\*真ん中: 決定の一覧の全部に答えを入れる（本体）。\*\*' '^5\. \*\*代案を2つ以上作り、自分で選ぶ。\*\*' '^6\. \*\*反論を走らせる' \
  '^7\. \*\*出口: 要点を見せ、線の中の決定を1問ずつ聞き、最後の確かめを1問で取る。\*\*' '^8\. \*\*文書にして渡す。\*\*' \
  '真ん中でオーナーに聞いてよいのは、線の g の不明点だけ' '\*\*真ん中では、問いを出すとき以外は止まらない。\*\*' \
  '価値の仮説は独立の問いにせず、要点に載せて最後の確かめで一緒に通す'; do
  grep -q "$k" "$ce" || miss="$miss [$k]"
done
grep -qF '待つことが本業' "$ce" && miss="$miss [待つことが本業が残る]"
[ -z "$miss" ] && pass "ADR-0037 A3: CE skill の骨格は入口・真ん中・出口の形" || fail "ADR-0037 A3: CE skill の骨格:$miss"
# ADR-0037 受け入れ条件 4: 文書の形の「決めたこと」に誰が決めたか
grep -q '^## 決めたこと（番号・日付・誰が決めたか・' <<<"$(block "$ce" '# <題>（仕様・設計。')" \
  && grep -qF '**「決めたこと」の各行には、誰が決めたかを書く。** 書き方は、決定の1文の末尾の「（オーナーが決めた）」か「（Chief Engineer が決めた）」。' "$ce" \
  && grep -qF '「仮」とは書かない' "$ce" \
  && pass "ADR-0037 A4: CE skill の文書の形は「決めたこと」の各行に誰が決めたかを書く" || fail "ADR-0037 A4: CE skill の文書の形に誰が決めたかが無い"
# ADR-0037 受け入れ条件 5: CoS の手引き（決めたこと 8）と、問いの出し方の skill の1行（決めたこと 9）
cosr=$(sed -n '/^## Chief Engineer の文書を受けたら$/,/^## 観察の課題/p' "$pb/cut.md")
old_cos='文書が人の言っていないことを言っている・'
grep -qF '「オーナーに聞く線」（a〜g）に当たる決定が「（Chief Engineer が決めた）」の印で書かれていないかを見る。見つけたら、切らずにオーナーへ知らせる。' <<<"$cosr" \
  && grep -qF '（検収は、オーナーが Chief Engineer の出口の最後の確かめでした）' <<<"$cosr" && ! grep -qF "$old_cos" <<<"$cosr" \
  && grep -qF -- '- Chief Engineer が自分の設計の決定をオーナーに聞くかどうかは、この節でなく chief-engineer skill の「オーナーに聞く線」に従う。' package/skills/ask-one-by-one/SKILL.md \
  && pass "ADR-0037 A5: CoS の cut.md は線に当たる CE の印の決定を見て知らせ、ask-one-by-one に CE は自分の線に従う1行がある" \
  || fail "ADR-0037 A5: cut.md の CE の文書を受ける手順か、ask-one-by-one の1行が古い"
grep -qF "$old_cos" <<<'6. 文書が人の言っていないことを言っている・受け入れ条件がテストできない形なら、切らずに人へ知らせる' \
  && pass "ADR-0037 A5: 古い cut.md の行は否定の grep に当たる（検査が見ている）" || fail "ADR-0037 A5: 否定の grep が何も見ていない"
# 線の確かめは、空の価値の仮説で戻すこと・切ること・観察の課題を足すことより先（切ってからでは担当が先に動く）
line_of() { grep -n -m1 -F -- "$2" <<<"$1" | cut -d: -f1; }
ce_order() { local a b c d; a=$(line_of "$1" '「オーナーに聞く線」（a〜g）に当たる決定'); b=$(line_of "$1" '「価値の仮説」の節が見出しだけ')
  c=$(line_of "$1" '「依頼書に切る案」を縦に薄く'); d=$(line_of "$1" '**観察の課題を1件**足す')
  [ -n "$a" ] && [ -n "$b" ] && [ -n "$c" ] && [ -n "$d" ] && [ "$a" -lt "$b" ] && [ "$a" -lt "$c" ] && [ "$a" -lt "$d" ]; }
ce_order "$cosr" && pass "ADR-0037 A5: cut.md は線の確かめを、価値の仮説で戻す・切る・観察の課題を足すより先に置く" || fail "ADR-0037 A5: cut.md の線の確かめが、切る手順より後にある"
# ADR-0038 受け入れ条件 3（Chief Engineer の決まりの文）: statechart の段（当てはまる設計の線・案を見せる前・反論に渡す・
# 文書の 1 行・資料として渡す）と、モデルのファイルを添える例外（CE の skill の渡し方と cut.md の 2 か所）
ces=$(sed -n '/^## 状態を動かす設計をモデルで調べる$/,/^## 文書の形$/p' "$ce")
miss=
for k in '**当てはまる設計の線。**' '状態を動かす流れそのものを変える設計' 'モデルは反論の代わりにしない' \
  'pstack の statechart' '`~/.claude/plugins/installed_plugins.json`' '`skills/statechart/SKILL.md`' '「本物のコードで再現する」はしない' \
  '`issues/<id>/library/` の下' 'repo のモデルには書かない' \
  '**案を見せる前に調べる。**' '推奨案は必ずモデルにして調べる' 'どの案を選ぶかは骨格 5 のとおりあなたが決める' \
  '**反論にモデルも渡す。**' '反論（骨格 6）は調べたあとに走らせる' \
  '**文書に1行書く。**' '`モデル: 調べた（規則 N 本・見つけた破れ M 件）`' '`モデル: 調べなかった（<理由>）`' \
  '**資料として渡す。**'; do
  grep -qF -- "$k" <<<"$ces" || miss="$miss [段: $k]"
done
grep -qF '状態を動かす流れを変えるか（下の「状態を動かす設計をモデルで調べる」の線）も決める' <<<"$ce2" || miss="$miss [入口で当てはまるかを決める]"
model_ex='例外として、検討で調べたモデルのファイルは、資料として添えてよい。依頼書はそのパスを指してよい'
sed -n '/^## 渡し方$/,/^## 取らないもの$/p' "$ce" | grep -qF "$model_ex" || miss="$miss [CE の渡し方の例外]"
grep -qF '"最初の実装の課題の依頼書に <モデルのパス> を添える"' "$ce" || miss="$miss [CE の next にモデルを添える行]"
grep -qF "（渡されるのは文書だけ。$model_ex）" <<<"$cosr" || miss="$miss [cut.md の例外]"
grep -qF '`next` に「最初の実装の課題の依頼書に <モデルのパス> を添える」があれば' <<<"$cosr" || miss="$miss [cut.md がモデルのパスを依頼書に書く]"
# repo へ入れるのはモデルが既にある流れだけ。モデルが無い流れのモデルは repo に置かない（ADR-0038 決めたこと 5）
model_bound='repo にその流れのモデルがあれば、実装の PR でそこへ入れる。モデルが無い流れのモデルは資料のまま使い切り、repo に新しく置かない'
unbound='実装の PR で repo のモデル[^（あ]*へ入れる'
cesd=$(sed -n '/^\*\*状態を動かす設計をモデルで調べる\*\*/,/^\*\*成果物の形\*\*/p' docs/design.md)
for t in "$ces" "$cosr" "$cesd"; do
  grep -qF "$model_bound" <<<"$t" || { miss="$miss [モデルを repo へ入れるのはモデルがある流れだけ]"; break; }
done
grep -qE "$unbound" "$ce" "$pb/cut.md" docs/design.md && miss="$miss [モデルの無い流れにも repo へ入れると読める文が残る]"
# 置き場のパスは版ごとに変わるので skill に書かない（ADR-0038 決めたこと 12）
pstack_path='plugins/cache/[^ ]*pstack/[0-9]'
grep -qE "$pstack_path" "$ce" && miss="$miss [版の入った pstack の置き場のパスが skill にある]"
[ -z "$miss" ] && pass "ADR-0038 A3: CE skill に statechart の段（線・案を見せる前・反論に渡す・文書の1行・資料として渡す）、渡し方の例外が CE skill と cut.md の2か所にあり、repo へ入れるのはモデルがある流れだけ" \
  || fail "ADR-0038 A3: CE の決まりの文:$miss"
grep -qE "$pstack_path" <<<'/opt/x/.claude/plugins/cache/cursor-plugins/pstack/0.15.43/skills/statechart/SKILL.md' \
  && grep -qE "$unbound" <<<'担当はそれを土台に実装し、実装の PR で repo のモデルへ入れる。' \
  && pass "ADR-0038 A3: 版の入った pstack の置き場のパスと、限らずに repo へ入れる文は否定の grep に当たる（検査が見ている）" || fail "ADR-0038 A3: 否定の grep が何も見ていない"
! ce_order "$(printf '%s\n' '3. 「依頼書に切る案」を縦に薄く' '4. **観察の課題を1件**足す' '2. 「価値の仮説」の節が見出しだけ' '6. 切る前に、「オーナーに聞く線」（a〜g）に当たる決定')" \
  && pass "ADR-0037 A5: 線の確かめが後ろにある並びは順の検査に当たる（検査が見ている）" || fail "ADR-0037 A5: 順の検査が何も見ていない"
# ADR-0037 受け入れ条件 6: design.md §1.8・§1.9 と dispatcher.md に古い文が残らない
old_docs='推奨案は出すが決めない|待つことが Chief Engineer の本業|docs がレベル A／B なら自動|検収は人が対話の中でした|人と Chief Engineer の対話で決める'
left=$(grep -nE "$old_docs" docs/design.md docs/dispatcher.md || true)
[ -z "$left" ] && grep -qF '作り方の決定と、どちらとも読める決定。オーナーに聞く線（7 種）の中の決定はオーナーが決める' docs/design.md \
  && grep -qF 'Chief Engineer の課題の文書の PR は、どの案件でもいつもオーナーがマージする' docs/design.md \
  && pass "ADR-0037 A6: design.md の役の表と骨格と dispatcher.md が新しい手順で、古い文が無い" || fail "ADR-0037 A6: design.md か dispatcher.md に古い文が残るか、新しい文が無い:$left"
grep -qE "$old_docs" <<<'Chief of Staff は Chief Engineer の課題を**検収しない**（検収は人が対話の中でした）' \
  && pass "ADR-0037 A6: 古い design.md の行は否定の grep に当たる（検査が見ている）" || fail "ADR-0037 A6: 否定の grep が何も見ていない"
left=$(grep -nE 'kanban ask [^-]*"<|kanban set <id> pr=' package/skills/*/SKILL.md package/skills/*/playbooks/*.md || true)
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
sed -n '/^# <id> <題>$/,/^```$/p' "$pb/brief.md" | sed -n '2p' | grep -q '^解決すべき問題:' \
  && ! grep -q '^やること:' <<<"$cosall" && pass "chief-of-staff の brief.md の雛形の最初の欄が 解決すべき問題: で、やること: が無い" || fail "chief-of-staff の雛形の最初の欄が 解決すべき問題: でない"
grep -q '解決すべき問題' package/skills/triage/SKILL.md && ! grep -q 'やること' package/skills/triage/SKILL.md package/skills/triage/assets/triage.yaml \
  && pass "triage skill とルーチンの依頼書の欄名が 解決すべき問題" || fail "triage の欄名が 解決すべき問題 でない"
# ADR-0030 受け入れ条件 14: triage skill に閉じ方の欄・見た commit・組み合わせの表があり、Chief of Staff の手引きに判断の閉じを読む手がある。
tsk=package/skills/triage/SKILL.md
miss=""
for row in '| 判断の閉じ | `wont-happen` | 起きない | `waived` だけ |' '| 判断の閉じ | `one-off` | その場限り | `remember`・`memory` だけ |' \
    '| 判断の閉じ | `not-applicable` | 当てはまらない | `memory` だけ |' '| 判断の閉じ | `read` | 読んだ: 拾うものなし | `unreadable` だけ |' \
    '| 根拠の閉じ | `done` | 済み | どの種類にも |' '| 根拠の閉じ | `gone` | 対象が無い | どの種類にも |'; do
  grep -qF -- "$row" "$tsk" || miss="$miss 組み合わせの行:$row"
done
grep -q '"decision": "close", "closing": "gone"' "$tsk" || miss="$miss decisions.json の例に closing"
grep -q '"commit": "collect が返した commit"' "$tsk" || miss="$miss decisions.json の例に commit"
grep -q '`close` は `closing`.*が必須' "$tsk" || miss="$miss closing が必須"
grep -q '`andon triage verdict` を打つ（どちらも Chief of Staff）' "$tsk" || miss="$miss 担当は verdict を打たない"
# 判断の閉じの値は apply のコードと同じ（表に無い値・コードに無い値が無い）。
code=$(grep -oE '"(wont-happen|one-off|not-applicable|read|done|gone|wiki|delegated)"' internal/triage/decisions.go | tr -d '"' | sort -u | tr '\n' ' ')
table=$(grep -oE '^\| (根拠|判断)の閉じ \| `[a-z-]+`' "$tsk" | grep -oE '`[a-z-]+`' | tr -d '`' | sort -u | tr '\n' ' ')
[ -n "$table" ] && [ "$code" = "$table" ] || miss="$miss 表の閉じ方（$table）がコード（$code）と違う"
grep -q '^   \*\*判断の閉じと課題の閉じを1件ずつ読む\*\*: `--dry-run` の出力の `review`' "$pb/triage.md" \
  && grep -q 'andon triage verdict <id> <key> overturn' "$pb/triage.md" && grep -q 'andon triage verdict <id> <key> ok' "$pb/triage.md" \
  || miss="$miss Chief of Staff の手引きに review を読んで verdict を書く手"
[ -z "$miss" ] && pass "triage skill に閉じ方の欄・見た commit・組み合わせの表、Chief of Staff の手引きに判断の閉じを読む手（ADR-0030）" || fail "トリアージの閉じ方の網:$miss"
grep -q 'andon issue stop-check' package/hooks/stop \
  && pass "package/hooks/stop が andon issue stop-check を打つ" || fail "package/hooks/stop が andon issue stop-check を打たない"

# === 8. 役の略称とオーナーへの書き方（ADR-0013 受け入れ条件 1・2） ===
g=package/skills/glossary/SKILL.md
grep -q '^\*\*Chief Engineer\*\*（chief-engineer。略して CE）:' "$g" && grep -q '^\*\*Chief of Staff\*\*（chief-of-staff。略して CoS）:' "$g" \
  && pass "用語集の Chief Engineer の項に「略して CE」がある" || fail "用語集に CE の略称が無い"
# 用語集（受け入れ条件 8 の用語集の分）: 書き方の節と語の決まり3つを持たず、writing.md を指す。モデルからは呼べないまま
grep -q '^- \*\*役と部品の語はここにある語だけを使う\*\*' "$g" && ! grep -q '^## オーナーへの書き方' "$g" && grep -q '^disable-model-invocation: true$' "$g" \
  && ! grep -qE 'ふだんカタカナで言う語|訳語を新しく作らない|作った訳語|文の中では略称で' "$g" \
  && grep -q '^- オーナーへの書き方と語の決まり.*は `/opt/andon/writing.md` に従う' "$g" \
  && pass "用語集は書き方の節と語の決まりを持たず writing.md を指す（disable-model-invocation は残る）" || fail "用語集に書き方の節か語の決まりが残る、か writing.md を指していない"
iss=$(sed -n '/^\*\*課題\*\*（issue）:/,/^_避ける_/p' "$g")
! grep -q '「番号（題）」' <<<"$iss" && grep -q '`/opt/andon/writing.md` の番号の決まりに従う' <<<"$iss" \
  && pass "用語集の課題の項は番号（題）の決まりを自分で言わず writing.md を指す" || fail "用語集の課題の項が writing.md を指していない"
# skill 4本と environment.md（受け入れ条件 8 の残り）: 「glossary skill に従う」が無く、writing.md か用語集のパスを指す。番号（題）の決まりは writing.md を指す。報告の書式に「（推奨案:」が無い
left=""
for f in package/environment.md package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/chief-engineer/SKILL.md package/skills/triage/SKILL.md; do
  grep -qE 'glossary skill に従う|/andon:glossary' "$f" && left="$left [$f:旧い言い方]"
  [ "$f" = "$cos" ] && grep -qE 'glossary skill に従う|/andon:glossary' "$pb"/*.md && left="$left [$pb:旧い言い方]"
  grep -q '/opt/andon/writing.md' "$f" && grep -q '/opt/andon/skills/glossary/SKILL.md' "$f" || left="$left [$f:パスが無い]"
done
[ -z "$left" ] && pass "worker・CoS・CE・triage の skill と environment.md は writing.md と用語集のパスを指す" || fail "「glossary skill に従う」が残るか、パスが無い:$left"
left=""
for f in package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/ask-one-by-one/SKILL.md; do
  grep -q '「番号（題）」の形' "$f" && left="$left [$f:番号（題）の決まり]"
  [ "$f" = "$cos" ] && grep -q '「番号（題）」の形' "$pb"/*.md && left="$left [$pb:番号（題）の決まり]"
  grep -q '/opt/andon/writing.md` の番号の決まりに従う' "$f" || left="$left [$f:writing.md を指していない]"
done
[ -z "$left" ] && pass "CoS・worker・ask-one-by-one の skill は番号（題）の決まりを自分で言わず writing.md を指す" || fail "skill が番号の決まりを自分で言うか、writing.md を指していない:$left"
! grep -q '（推奨案:' package/skills/worker/SKILL.md && pass "worker skill の報告の書式に「（推奨案:」が無い" || fail "worker skill の報告の書式に「（推奨案:」が残る"
# writing.md（受け入れ条件 7）: 読み手をオーナーに返す文に絞り、箇条書きは11行、但し書き・語の決まり3つ・用語集のパス
wr=package/writing.md
how=$(sed -n '1,/^## /p' "$wr")
n=$(grep -c '^- ' <<<"$how")
[ "$n" = 11 ] && grep -q '^CoS・CE・担当がオーナーに返す文' <<<"$how" \
  && grep -q '^- 結論を先に書く（BLUF）' <<<"$how" && grep -q '状況・経緯・見立て・推奨の順に書く（SBAR）' <<<"$how" \
  && grep -q '「・」で語を詰めない' <<<"$how" && grep -q '1文に1つのこと' <<<"$how" \
  && grep -q "英語の用語を直訳した語は使わない" <<<"$how" && grep -q '灯り → オーナーの判断待ち、レベル C → オーナーがマージ' <<<"$how" \
  && grep -q '^- 括弧に説明を入れない。説明は文にする。括弧に入れるのは識別子と、番号に添える題だけ。' <<<"$how" \
  && grep -q '^- 番号だけで指さない。.*`S-0004（返事の下書きボタン）`.*同じ返事の2度目からは番号だけでよい。' <<<"$how" \
  && grep -q '^- 読む人は道具の呼び出しも、読んだファイルも、途中の考えも見ていない。' <<<"$how" \
  && grep -q '^- つながる考えは段落の文で書く。箇条書きは別々の項目を並べるときに使う。' <<<"$how" \
  && grep -q '^- 作業の途中で自分が付けた呼び名や略称を使わない。' <<<"$how" \
  && grep -q '^コードと、オーナーが別の形を求めたときは、この決まりから除く' <<<"$how" \
  && pass "writing.md はオーナーに返す文に絞り、書き方の11行と但し書きがある" || fail "writing.md の書き方の節（行の数 $n）"
! grep -qE '括弧には識別子だけ|番号で参照しない' "$wr" && pass "writing.md に古い字面（括弧には識別子だけ・番号で参照しない）が無い" || fail "writing.md に古い字面が残る"
grep -q 'Chief of Staff を CoS、Chief Engineer を CE と書く' "$wr" \
  && grep -q 'ふだんカタカナで言う語（コスト・プロファイル・テンプレート・コンテキスト・オーナー・ログ・パス）はカタカナのまま' "$wr" \
  && grep -q '訳語を新しく作らない。迷ったら同僚に口で言うときの言い方にする' "$wr" && grep -q '`/opt/andon/skills/glossary/SKILL.md`' "$wr" \
  && pass "writing.md に語の決まり3つと用語集のパスがある" || fail "writing.md に語の決まりか用語集のパスが無い"
# 作った語は「避ける」の行と仮設の語の表にしか出ない。残す語（検査・継ぎ目・条件表・控え）は消さない
left=$(sed '/^## 仮設の語/,$d' "$g" | grep -nE '席|端末の土台|持ち主|提供元' | grep -v '^[0-9]*:_避ける_' || true)
[ -z "$left" ] && pass "用語集に作った語（避ける語）が正式な語として出ない" || fail "用語集に作った語が残る: $left"
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
grep -q 'オーナーが見るブラウザは無い' package/environment.md && ! grep -qE '(^|[^見る])ブラウザは無い' package/environment.md \
  && grep -q '担当が画面を見て確かめるには `playwright-cli`' package/environment.md \
  && pass "environment.md は「オーナーが見るブラウザは無い」と playwright-cli を案内する" || fail "environment.md の「ブラウザは無い」が直っていない"
# ADR-0020 受け入れ条件 6（決めたこと 6・A-0144 の c2・c3）: 環境メモに memory への書き方
envmem=$(sed -n '/^## 気づきを残す（auto memory）$/,/^## /p' package/environment.md)
miss=""
for k in '`/workspace/memory/`' '蒸留までの受け皿' '週1のトリアージ' '1件1ファイルの `.md` を、フォルダの直下に書く' '確かめた事実と、起きた条件と理由のついた学び' '本文に課題番号（無ければ repo）を書く' '続き・決定・秘密は書かない' '報告の `## Remember` を書く' 'Codex の記憶は切ってあり'; do
  grep -qF -- "$k" <<<"$envmem" || miss="$miss $k"
done
[ -z "$miss" ] && pass "environment.md に memory への書き方（受け皿・直下の .md・条件と理由・課題番号・続き/決定/秘密は書かない・Remember は残す。ADR-0020）" || fail "environment.md の memory の節:$miss"

# === 10. 承認の段・書く場所・字数・reach（A-0122。A-0117 の c7・c8・c11・c17） ===
w=package/skills/worker/SKILL.md
w5=$(sed -n '/^## 5\. /,/^## 6\. /p' "$w")
grep -q '^| 承認が要る（「オーナーの go」で受ける） | 承認は要らない（聞かずにする） |$' <<<"$w5" && grep -q 'force-with-lease` も含む' <<<"$w5" \
  && pass "worker skill §5 に承認の要る操作と要らない操作が並ぶ（自分の PR の branch への force-with-lease は要らない側）" || fail "worker skill §5 に承認の段の表が無い"
[ "$(grep -c '\*\*書く場所\*\*' "$w")" = 1 ] && ! grep -q 'worktree の外にファイルを書' "$w" package/templates/*.md \
  && pass "書く場所は worker skill §2 の1か所（skill のほかの節と template に「worktree の外に書かない」が無い）" || fail "書く場所が1か所に寄っていない"
! grep -qE '100字|1〜2行|1〜3 行' "$w" && grep -q '1行に1件（Chief of Staff とトリアージが行ごとに拾う）' "$w" \
  && pass "worker skill の報告の書式に字数の上限が無く、1行に1件と理由がある" || fail "worker skill の報告の書式に字数の上限が残る"
grep -q '`reach`.*指示ではない' "$w" && pass "worker skill に hc の reach は指示ではないとある" || fail "worker skill に reach の扱いが無い"

# === 10b. Chief of Staff の行数・承認の段・起動の go・続きの文（A-0123。A-0117 の c6・c9・c10・c13） ===
! grep -qE '2行まで|オーナー(へ|に)1行|1行で(言|知らせ|伝え|返)' <<<"$cosall" && grep -q 'オーナーへの文は短く、オーナーがすることを先に書く。.*`/opt/andon/writing.md`' "$cos" \
  && pass "chief-of-staff skill にオーナーへの文の行数の上限が無く、「短く。オーナーがすることを先に」と writing.md を指す" || fail "chief-of-staff skill に行数の上限が残る"
cos4=$(cat "$pb/blocked.md")
grep -q '^     | Yes と答える | オーナーへ回す |$' <<<"$cos4" && grep -q 'force-with-lease` も含む' <<<"$cos4" && ! grep -q '外へ出さないものだけ' <<<"$cosall" \
  && grep -q '^4\. \*\*後戻りできない操作はオーナーの go\*\*。.*線は worker skill §5 と同じ' "$cos" \
  && pass "chief-of-staff の原則4が worker skill §5 の線を指し、blocked.md に Yes と答える操作とオーナーへ回す操作が並ぶ（自分の PR の branch への force-with-lease は Yes）" || fail "chief-of-staff の原則4か blocked.md に承認の段の表が無い"
grep -q '^2\. \*\*オーナーの go を取ってから\*\*起動する。.*委譲の範囲の中.*`projects/<slug>/AGENTS.md` の Chief of Staff の節が「起動にオーナーの go は要らない」' "$pb/start.md" \
  && pass "chief-of-staff の start.md の起動の go に、ルーチン・委譲・プロジェクトの設定の例外がある" || fail "chief-of-staff の start.md に起動の go の例外が無い"
! grep -q '続きをして。終わったら kanban' <<<"$cosall" && grep -q '続きをして。終わり方は worker skill §7（andon issue report → andon issue ready）' "$pb/watch.md" \
  && pass "chief-of-staff skill の担当への続きの文が worker skill §7 を指す" || fail "chief-of-staff skill の続きの文が古い"

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
cos1=$(cat "$pb/cut.md")
miss=
grep -q '「価値の仮説」の節が見出しだけ（空）なら、\*\*切らずに Chief Engineer へ戻す\*\*' <<<"$cos1" || miss="$miss 空なら戻す"
grep -q '`kanban add "観察: <題>の価値の仮説" --parent <Chief Engineer の id>' <<<"$cos1" && grep -q '`kanban set <新しい id> not_before=<(3) の日付>`' <<<"$cos1" || miss="$miss 観察の課題を --parent と not_before で足す"
grep -q '^## `signal` の通知を受けたら$' <<<"$cos1" && grep -q '^1\. \*\*直す課題に切る\*\*.*`links` に信号の `url` と `key`' <<<"$cos1" || miss="$miss signal を直す課題に切る"
grep -q '^## 観察の課題の報告を受けたら$' <<<"$cos1" && grep -q '\*\*ラインは何もしない\*\*' <<<"$cos1" || miss="$miss 観察の報告をオーナーへ返すだけ"
block "$pb/brief.md" '# <id> 観察: ' | grep -q '^完了条件: report.json の observation' || miss="$miss 観察の課題の雛形"
grep -q '^| `signal` の行' "$pb/watch.md" || miss="$miss news の表の signal の行"
[ -z "$miss" ] && pass "chief-of-staff skill に、空なら戻す・観察の課題を --parent と not_before で足す・signal を直す課題に切る、の行がある" || fail "chief-of-staff skill の価値のループ:$miss"
grep -q '^| 7\. 課題は台帳が既定 | 4・6 | `cut.md` |$' "$cos" \
  && grep -q '^  `write_back: true` の出所への `andon issue finish` の1コメントは委譲の中。それ以外のトラッカーへの書き込み（状態の変更・途中のコメント）はオーナーの go' <<<"$cos1" \
  && grep -q '`write_back: true` の出所（今は `github-issues`）の issue は、`links` に URL' <<<"$cos1" \
  && pass "chief-of-staff の規約7（cut.md）が write_back の1コメントを委譲の中に、ほかの書き込みをオーナーの go に分ける（ADR-0011 決めたこと 15）" \
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
for f in package/environment.md "$pb/watch.md" "$pb/triage.md"; do
  grep -n 'herdr notification show' "$f" | grep -q 'TUI' || miss="$miss $f"
  grep -q '台帳の問いとボードで足りる' "$f" || miss="$miss $f(足りる)"
done
left=$(grep -n 'herdr notification show' "$cos" "$pb"/*.md | grep -v 'TUI' || true); [ -n "$left" ] && miss="$miss TUI の無い行:$left"
[ -z "$miss" ] && pass "environment.md と chief-of-staff の手引き（watch.md・triage.md）が herdr notification show を「TUI が動いているときだけ」と書く" || fail "herdr notification show の文言:$miss"

# hc の手引き（A-0192）: 全役が読む environment.md に「hc --skill を読む」の1行がある
grep -q 'skill に書いていない `hc` の操作が要るときだけ、先に `hc --skill`' package/environment.md \
  && sed -n '/^## 起動したら1回$/,/^## 毎ターン/p' "$pb/wake.md" | grep -q '`hc --skill` を1回読む' \
  && pass "environment.md は要るときだけ、chief-of-staff の wake.md は起動したときに hc --skill を読む" || fail "hc --skill を読む行（environment.md か chief-of-staff の wake.md）が無い"

# === 14. 受け入れ条件の証拠の質（ADR-0007 R5・A4） ===
miss=
c4=$(sed -n '/^## 4\. /,/^## 5\. /p' docs/contract.md | grep '^| `acceptance\[\]` |')
for f in "$c4" "$(grep '^\*\*受け入れ条件の節\*\*' "$ce")" "$(grep '^- `acceptance`:' "$w")"; do
  grep -q 'テストの command と' <<<"$f" && grep -q '出力' <<<"$f" && grep -q 'staged な）出力' <<<"$f" && grep -q 'mock に置き換えたテスト' <<<"$f" && grep -q '証拠でない' <<<"$f" \
    || miss="$miss [$(cut -c1-30 <<<"$f")]"
done
[ -z "$miss" ] && pass "contract.md §4 の acceptance[] の行・CE skill の受け入れ条件の節・worker skill §7 に、証拠はテストの command と出力のパスで staged な出力と mock は証拠でない、とある" || fail "受け入れ条件の証拠の質:$miss"

# === 16. 日報（ADR-0024 A1・A2） ===
miss=
j=$(sed -n '/^### 日報（`library\/journal\.md`）/,/^### /p' "$w")
for k in 'issues/<id>/library/journal.md' 'その場で' '報告を書く前に' '## 感じたこと' '## 改善できそうなこと' '判定に使われない' '差し戻されない' '秘密・資格情報は書かない'; do
  grep -qF -- "$k" <<<"$j" || miss="$miss [$k]"
done
grep '^| 振り返り |' "$w" | grep -q '日報' || miss="$miss [工程表の振り返り]"
grep -q '日報.*検収の判定に使わない。日報の中身.*を理由に差し戻さない' "$pb/gate.md" || miss="$miss [CoS skill]"
grep -q '^\*\*日報\*\*' package/skills/glossary/SKILL.md || miss="$miss [用語集]"
[ -z "$miss" ] && pass "worker skill に日報の節（置き場・時機・2つの見出し・判定に使わない・秘密）と工程表の振り返り、CoS skill に差し戻さない1行、用語集に日報がある" || fail "日報:$miss"

# === 17. Chief of Staff の原則と手引き（A-0195） ===
# SKILL.md（毎ターン system prompt に載る）は役・原則・手引きの目次・手引きの扱い・規約の行き先だけで 10KB 以下。
# 目次の手引きは全部在り、在る手引きは全部目次に載り、どれも先頭に「読む時」と「消す条件」を持つ。template は SKILL.md を載せたまま、手引きの置き場を --add-dir で足す。
miss=
[ "$(wc -c <"$cos")" -le 10240 ] || miss="$miss [SKILL.md が $(wc -c <"$cos") バイト]"
got=$(grep '^## ' "$cos" | tr '\n' '|')
[ "$got" = '## 原則（モデルや command が替わっても守る）|## 手引き（状況が来たら読む）|## 手引きの扱い|## 規約1〜10 の行き先|' ] || miss="$miss [SKILL.md の節: $got]"
for i in 1 2 3 4 5 6 7 8 9 10 11; do grep -q "^$i\. \*\*" "$cos" || miss="$miss [原則 $i]"; done
grep -q '`/opt/andon/skills/chief-of-staff/playbooks/`' "$cos" || miss="$miss [手引きの置き場]"
listed=$(sed -n '/^## 手引き（状況が来たら読む）$/,/^## 手引きの扱い$/p' "$cos" | grep -o '| `[a-z-]*\.md` |$' | tr -d '|` ' | sort | tr '\n' ' ')
have=$(cd "$pb" && ls -1 *.md | sort | tr '\n' ' ')
[ -n "$have" ] && [ "$listed" = "$have" ] || miss="$miss [目次と手引きが食い違う: 目次 $listed／在る $have]"
for f in "$pb"/*.md; do
  [ "$(sed -n 1p "$f")" = '---' ] && [ -n "$(fm "$f" 読む時)" ] && [ -n "$(fm "$f" 消す条件)" ] || miss="$miss [$f の先頭に 読む時・消す条件 が無い]"
done
for n in $(seq 1 10); do grep -q "^| $n\. .* | \`[a-z]*\.md\` |$" "$cos" || miss="$miss [規約 $n の行き先]"; done
case " $(fm package/templates/chief-of-staff.md command) " in *" --append-system-prompt-file /opt/andon/skills/chief-of-staff/SKILL.md "*) ;; *) miss="$miss [template が SKILL.md を載せていない]" ;; esac
# 手引きは Read で読む。/opt/andon は作業の場の外で、--add-dir が無いと Read が許可の確認で止まる（A-0195 の実測）
case " $(fm package/templates/chief-of-staff.md command) " in *" --add-dir /opt/andon/skills/chief-of-staff/playbooks "*) ;; *) miss="$miss [template に手引きの --add-dir が無い]" ;; esac
[ -z "$miss" ] && pass "chief-of-staff の SKILL.md は原則と目次だけで 10KB 以下、目次の手引きが全部在り、どれも 読む時・消す条件 を持つ" || fail "chief-of-staff の原則と手引き:$miss"

# ADR-0026 受け入れ条件 A1〜A4（案 1）: PR と文書の書き方の決まりは、セルの外の読み手に通じるように
w=package/skills/worker/SKILL.md; ce=package/skills/chief-engineer/SKILL.md
sec3=$(sed -n '/^## 3\. 検査・commit・PR$/,/^## 4\./p' "$w")
chk3() { grep -qF -- "$2" <<<"$sec3" && pass "ADR-0026 A1: worker §3 に $1" || fail "ADR-0026 A1: worker §3 に $1 が無い"; }
chk3 "タイトル・commit に課題番号を書かない" 'PR のタイトル・commit のタイトル・commit の本文に入れない'
chk3 "メモの行の形" '`Memo: 内部の課題番号 <id>`（repo が英語なら `Memo: internal issue <id>`）'
chk3 "6つの見出しと必須の3節" '必須は「変更理由」「やったこと」「検証結果と確かめ方」の3節'
for h in 変更範囲 トレードオフ 影響範囲; do grep -q "^     | $h | あれば |" <<<"$sec3" || fail "ADR-0026 A1: worker §3 の表に $h が無い"; done
chk3 "長い物だけ details に畳む" '長い物だけ `<details>`／`<summary>` に畳む'
chk3 "状態の図" '変更前と変更後の状態の図（Mermaid）'
chk3 "ほかの PR は URL か owner/repo#n" '`owner/repo#n`'
chk3 "セルの物を指さず案件の repo は Andon の用語を書かない" '案件の repo では Andon の用語'
# ADR-0026 A2
grep -qF 'PR 本文の「トレードオフ」と報告の「見送り」にも写す' "$w" && pass "ADR-0026 A2: worker §4 の見送りの写し先は トレードオフ" || fail "ADR-0026 A2: worker §4 の見送りの写し先が トレードオフ でない"
# ADR-0026 A3
d3=$(sed -n '/^# <題>（仕様・設計。日付）$/p' "$ce" docs/design.md | wc -l)
[ "$d3" = 2 ] && ! grep -q 'Chief Engineer の課題 id）' "$ce" docs/design.md \
  && grep -qF 'オーナーの発言の日付だけ' "$ce" && grep -qF 'オーナーの発言の日付だけ' docs/design.md && grep -qF 'オーナーの発言の日付だけ' docs/decisions.md \
  && ! grep -qF '`sources/` のページか `raw/` の元資料' "$ce" \
  && pass "ADR-0026 A3: CE skill・design.md §1.8・decisions.md の出典は題に課題 id を求めず、セルの wiki と台帳を指さない" || fail "ADR-0026 A3: 文書の形か出典の書き方が古い"
# 否定の検査が当たる場合があること: 古い形を持つ文字列では上の検査が落ちる
! printf '%s\n' '# <題>（仕様・設計。日付・Chief Engineer の課題 id）' | grep -q '^# <題>（仕様・設計。日付）$' && pass "ADR-0026 A3: 古い題の行は新しい形に当たらない（検査が見ている）" || fail "ADR-0026 A3: 検査が何も見ていない"
# ADR-0026 A4
grep -q '^- 番号だけで指さない。この決まりはセルの中のオーナーへの文に効く。PR のタイトルや本文・commit・repo のファイルの文は' package/writing.md \
  && pass "ADR-0026 A4: writing.md の番号の決まりに範囲（セルの中のオーナーへの文。PR と repo の文は除く）がある" || fail "ADR-0026 A4: writing.md の番号の決まりに範囲が無い"

# === 18. ADR の索引と書き分け（ADR-0033 受け入れ条件 9・決めたこと 3・4・5） ===
# 文書の形の見出しで、題の次の行が `## 決定の索引`（tests/adr-index.sh が見る見出しと同じ。括弧の説明を付けない）
idx_next() { block "$1" '# <題>（仕様・設計。' | sed -n 2p; }
miss=
for f in "$ce" docs/design.md; do
  [ "$(idx_next "$f")" = '## 決定の索引' ] || miss="$miss [$f の文書の形に題の次の ## 決定の索引]"
  grep -q '| # | 何を決めたか | 今の正 | 今 |' "$f" || miss="$miss [$f に索引の 4 列]"
  grep -q '生きている・覆された.*役目を終えた・まだ実装していない' "$f" || miss="$miss [$f に「今」の 4 つの語]"
  grep -q '「細部:」と断って分ける' "$f" || miss="$miss [$f に決定と細部の書き分け]"
  grep -q '実装の PR が.*「今の正」と「今」' "$f" && grep -q '本文は畳まない' "$f" || miss="$miss [$f に実装の PR が索引の行を直す]"
done
grep -q '^   - \*\*ADR の決定を実装する PR\*\*: その ADR の冒頭に `## 決定の索引` があれば、実装した決定の行の「今の正」（repo の中のパス）と「今」を同じ PR で直す。' "$w" || miss="$miss [worker §3 に索引の行を直す]"
[ -z "$miss" ] && pass "ADR-0033 A9: CE skill の文書の形と design.md §1.8 に冒頭の索引・決定と細部の書き分け・実装の PR が索引の行を直すことがあり、worker §3 に索引の行を直す1行がある" || fail "ADR-0033 A9:$miss"
# 検査が当たる場合があること: 索引の見出しが無い形では題の次の行が別の見出しになる
[ "$(printf '%s\n' '```' '# <題>（仕様・設計。日付）' '## 問題（オーナーの言葉で。言い直しの段落）' '```' >"$tmp/form.md"; idx_next "$tmp/form.md")" != '## 決定の索引' ] \
  && pass "ADR-0033 A9: 索引の見出しの無い文書の形は当たらない（検査が見ている）" || fail "ADR-0033 A9: 検査が何も見ていない"

# === 19. 振り返りの手順（ADR-0035 受け入れ条件 12・決めたこと 6・8） ===
r=package/skills/retro/SKILL.md
miss=
t=$(sed -n '/^### `AGENTS.md` の表を読む$/,/^## 3\. /p' "$r")
[ -n "$t" ] || miss="$miss [§2 に AGENTS.md の表を読む節]"
for k in 'show origin/HEAD:AGENTS.md' '「決まり｜守らせるもの」の対' '表の行は根拠に数えない' '守らせるものが「無し。」の行' '文を足さず、検査か設計へ移す' '守らせるものが名指されている行' '誤りが起きようのなくなった行'; do
  grep -qF -- "$k" <<<"$t" || miss="$miss [表の節に $k]"
done
s3=$(sed -n '/^## 3\. /,/^## 4\. /p' "$r")
for k in '**検査を足すか変える直し**' '受け入れ条件: 根拠の誤りを戻した木で、足した検査が落ちる' 'andon issue record <id> --expect fail --' '「期待する効果:」の行になる'; do
  grep -qF -- "$k" <<<"$s3" || miss="$miss [§3 に $k]"
done
# §6 の例の test の段のパターンも、effect に受け入れ条件を持つ
block "$r" 'andon retro patterns "$id"' | sed '1d;$d' >"$tmp/retro-in.json"
jq -e '[.patterns[] | select(.level == "test" or .level == "check") | .effect | contains("受け入れ条件: 根拠の誤りを戻した木で、足した検査が落ちる")] | length > 0 and all' "$tmp/retro-in.json" >/dev/null 2>&1 \
  || miss="$miss [§6 の例の test の段の effect に受け入れ条件]"
grep -qF '「誤りを戻すと落ちる」の受け入れ条件が載っているか' "$r" || miss="$miss [§6 の dry-run で受け入れ条件を見る]"
grep -qF '`AGENTS.md` の表: <読んだ repo>' "$r" || miss="$miss [§7 の報告に表の行]"
grep -q '^description: .*`AGENTS.md` の表の読み方.*「誤りを戻すと落ちる」の受け入れ条件' "$r" || miss="$miss [description]"
[ -z "$miss" ] && pass "ADR-0035 A12: retro skill に AGENTS.md の表を読む手順と、検査を足す直しの依頼書に「誤りを戻すと落ちる」の受け入れ条件を入れる手順がある" || fail "ADR-0035 A12:$miss"

# === 20. PR を出す前の3つの観点（worker skill §2・§4・§7 とレビュアーの依頼書） ===
# 担当が当てる観点は、レビュアーの依頼書（dispatcher の reviewerBrief と independent-review skill の型）の「正しさ」の語で書く。
# 3か所の「正しさ」の語の並びが同じで、担当の3つの観点の見出しと括弧の語が、その並びの中の語であること。
w=package/skills/worker/SKILL.md
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w"); w4=$(sed -n '/^## 4\. /,/^## 5\. /p' "$w"); w7=$(sed -n '/^## 7\. /,$p' "$w")
# 正しさの語の並び（「・」で区切った語を1行ずつ）
brief_ok=$(sed -n 's/.*"2\. 正しさ: \([^\\"]*\)\\n".*/\1/p' internal/dispatcher/loop.go)
ir_ok=$(sed -n 's/.*正しさ（\([^）]*\)）.*/\1/p' package/skills/independent-review/SKILL.md)
w_ok=$(sed -n 's/.*「正しさ: \([^」]*\)」.*/\1/p' <<<"$w2")
# 担当の観点: 「  - **<語>**」の見出しと、「（レビュアーの「<語>」の1つ）」の語
views=$(sed -n 's/^  - \*\*\([^*]*\)\*\*.*/\1/p' <<<"$(sed -n '/PR を出す前に、3つの観点を自分の差分に当てる/,/当てた結果は/p' <<<"$w2")")
parents=$(grep -o '（レビュアーの「[^」]*」の1つ）' <<<"$w2" | sed 's/（レビュアーの「\(.*\)」の1つ）/\1/')
# in_list <語> <並び>: 語が「・」区切りの並びの中にあるか
in_list() { sed 's/・/\n/g' <<<"$2" | grep -qxF -- "$1"; }
miss=
[ -n "$brief_ok" ] && [ "$brief_ok" = "$ir_ok" ] && [ "$brief_ok" = "$w_ok" ] \
  || miss="$miss [正しさの語が食い違う: reviewerBrief=$brief_ok independent-review=$ir_ok worker=$w_ok]"
[ "$(tr '\n' '|' <<<"$views")" = '失敗の波及|前の版の形|落ちたあとの起き直し|' ] || miss="$miss [worker §2 の観点の見出し: $(tr '\n' ' ' <<<"$views")]"
[ "$(wc -l <<<"$parents")" = 2 ] || miss="$miss [worker §2 の観点に「レビュアーの「…」の1つ」が2つ無い]"
for v in 失敗の波及 $parents; do in_list "$v" "$brief_ok" || miss="$miss [観点の語「$v」がレビュアーの正しさ（$brief_ok）に無い]"; done
grep -q '^| 確認 |.*PR を出す前の3つの観点' <<<"$w2" || miss="$miss [worker §2 の工程表の確認に3つの観点]"
grep -q '3つの観点〈§2〉を当てた行は、PR を出す前に報告に書いておく' <<<"$w4" || miss="$miss [worker §4 の 1 に3つの観点]"
for v in $views; do grep -q "^$v: <" <<<"$w7" || miss="$miss [worker §7 の書式に「$v:」の行]"; done
[ -z "$miss" ] && pass "worker skill の PR を出す前の3つの観点が、レビュアーの依頼書（reviewerBrief・independent-review）の正しさの語（$brief_ok）で書かれ、§2 の工程表・§4・§7 の書式にある" || fail "3つの観点:$miss"
# 検査が当たる場合があること: 依頼書の正しさの語が変わると、担当の並びと食い違う
[ "$(sed -n 's/.*"2\. 正しさ: \([^\\"]*\)\\n".*/\1/p' <<<'		"2. 正しさ: 並行性・境界\n" +')" != "$w_ok" ] && ! in_list 失敗の波及 '並行性・境界' \
  && pass "3つの観点: 依頼書の正しさの語が変わった形は食い違いとして当たる（検査が見ている）" || fail "3つの観点: 検査が何も見ていない"

# === 21. 当てたあとの検証（ADR-0035 受け入れ条件 13 の後半・決めたこと 4・9） ===
# verify_miss <手引き>: CoS の手引きに、切る条件（地図に当たる・同時に1つ・前の検証が見た commit から）と、依頼書が名指す skill がそろうか
verify_miss() {
  local m='' k
  for k in '同時に動く検証の課題は1つ' 'select(.title | startswith("検証: "))' '前の検証が見た commit から見る' \
    'bash "$d/.claude/skills/verify-andon/scripts/features.sh" <始まり>..<当てた commit>' \
    '差分が地図の「変えたら動かすパス」に当たらない' '読む skill: .claude/skills/verify-andon/SKILL.md' \
    '前の検証が見た commit: <始まり>' 'kanban review <id> で返す' '「当てたら」の 2 で始まりの commit を決め直す' '報告に見た範囲が無い（診断で止まった）ときも' 'それを当てた commit として「当てたら」の 3 から回す'; do
    grep -qF -- "$k" "$1" || m="$m [$k]"
  done
  echo "$m"
}
miss=$(verify_miss "$pb/verify.md")
# 手引きが名指すものが repo に在り、検証の skill が依頼書の同じ行を読む
for f in .claude/skills/verify-andon/SKILL.md .claude/skills/verify-andon/scripts/features.sh; do [ -f "$f" ] || miss="$miss [$f が無い]"; done
grep -qF '前の検証が見た commit' .claude/skills/verify-andon/SKILL.md || miss="$miss [検証の skill が「前の検証が見た commit」を読まない]"
grep -q '^| マージした変更をセルへ当てた.* | `verify.md` |$' "$cos" || miss="$miss [SKILL.md の目次に verify.md]"
[ -z "$miss" ] && pass "ADR-0035 A13 後半: CoS の手引き verify.md に、差分が地図に当たるときだけ・同時に1つ・前の検証が見た commit から検証の課題を切る手順がある" || fail "ADR-0035 A13 後半:$miss"
# 検査が当たる場合があること: 同時に1つの行を消した手引きでは落ちる
grep -vF '同時に動く検証の課題は1つ' "$pb/verify.md" >"$tmp/verify.md"
[ -n "$(verify_miss "$tmp/verify.md")" ] && pass "ADR-0035 A13 後半: 同時に1つの行が無い手引きは当たる（検査が見ている）" || fail "ADR-0035 A13 後半: 検査が何も見ていない"

# === 22. pstack と yomiyasu をパッケージの既定に入れる（ADR-0038 受け入れ条件 1・3 のうち writing.md と環境メモ。決めたこと 11・16〜19） ===
# 参照: managed settings に marketplace の登録（自動更新つき。版を固定する sha・ref は書かない）と pstack の有効化、外の skill の一覧に版なしの yomiyasu
refs_ok() {
  jq -e '.extraKnownMarketplaces["cursor-plugins"] as $m
    | $m.source == {"source": "github", "repo": "cursor/plugins"} and $m.autoUpdate == true
    and .enabledPlugins["pstack@cursor-plugins"] == true' "$1" >/dev/null || return 1
  grep -qx 'nanaism/yomiyasu yomiyasu' "$2" || return 1
  ! grep -vE '^[[:space:]]*(#|$)' "$2" | grep -q '@'
}
refs_ok package/settings.json package/gh-skills.txt \
  && pass "ADR-0038 A1: settings に marketplace の登録（autoUpdate つき・版の固定なし）と pstack の有効化、gh-skills.txt に版なしの yomiyasu がある" \
  || fail "ADR-0038 A1: settings の pstack の参照か gh-skills.txt の yomiyasu の行が無い（か版を固定している）"
# 検査が当たる場合があること: 参照を1つずつ消した・版を固定した写しでは落ちる
miss=
cp package/gh-skills.txt "$tmp/gh-skills.txt"
for q in 'del(.enabledPlugins)' 'del(.extraKnownMarketplaces["cursor-plugins"])' 'del(.extraKnownMarketplaces["cursor-plugins"].autoUpdate)' \
  '.extraKnownMarketplaces["cursor-plugins"].source.sha = "0000000000000000000000000000000000000000"'; do
  jq "$q" package/settings.json >"$tmp/settings.json"
  refs_ok "$tmp/settings.json" "$tmp/gh-skills.txt" && miss="$miss [$q]"
done
for s in '/^nanaism\/yomiyasu yomiyasu$/d' 's/^nanaism\/yomiyasu yomiyasu$/nanaism\/yomiyasu yomiyasu@v3.0.1/'; do
  sed "$s" package/gh-skills.txt >"$tmp/gh-skills.txt"
  refs_ok package/settings.json "$tmp/gh-skills.txt" && miss="$miss [gh-skills.txt: $s]"
done
[ -z "$miss" ] && pass "ADR-0038 A1: 参照を消した・版を固定した写しでは落ちる（検査が見ている）" || fail "ADR-0038 A1: 写しで落ちない:$miss"
# 「既定は空」の文が残っていない（決めたこと 18）。古い字面に当たることも確かめる
old_line='（既定は空）'
grep -qF "$old_line" <<<'`/opt/andon/gh-skills.txt`（既定は空）＋ セルの' || fail "ADR-0038: 「既定は空」の検査が何も見ていない"
! grep -qF '既定は空' package/gh-skills.txt && ! grep -qF "gh-skills.txt$old_line" internal/boot/extskills.go docs/design.md \
  && grep -qF 'plugins（pstack）の登録は `package/settings.json` の `extraKnownMarketplaces`' docs/design.md \
  && pass "ADR-0038 決めたこと 18: gh-skills.txt・extskills.go・design.md に「既定は空」が無く、design.md は plugin の登録の正を package/settings.json と書く" \
  || fail "ADR-0038 決めたこと 18: 「既定は空」が残るか、design.md の plugin の登録の正が古い"
# writing.md の推敲の行（決めたこと 16）と、環境メモの pstack の行（決めたこと 17）
pol=$(sed -n '/^## 推敲$/,/^## /p' package/writing.md)
grep -qF '依頼書・wiki のページ・課題の報告の3種類の文は、書いたあとに skill yomiyasu で推敲する。' <<<"$pol" \
  && grep -qF '会話の返事・`docs/` の文書・PR の説明文には通さない。' <<<"$pol" && grep -qF 'yomiyasu が入っていないセルでは飛ばす。' <<<"$pol" \
  && pass "ADR-0038 A3: writing.md の推敲の節に、通す3種類・通さない文・入っていないセルでは飛ばす、がある" || fail "ADR-0038 A3: writing.md の推敲の行が無い"
miss=
for k in 'pstack の skill は、役の skill か依頼書が名指したものだけを、`SKILL.md` を読んで使う。' '`setup-pstack` は走らせない。' '`pstack:` で始まる型のサブエージェントは使わない。'; do
  grep -qF -- "$k" package/environment.md || miss="$miss $k"
done
[ -z "$miss" ] && pass "ADR-0038 A3: 環境メモに pstack の行（名指したものだけ・setup-pstack は走らせない・pstack: の型は使わない）がある" || fail "ADR-0038 A3: 環境メモの pstack の行:$miss"
# 担当と Chief of Staff に pstack の skill を使う指示を足さない（やらないこと）
left=$(grep -ln 'pstack' package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/chief-of-staff/playbooks/*.md || true)
[ -z "$left" ] && pass "ADR-0038: worker と CoS の skill に pstack の指示が無い" || fail "ADR-0038: pstack の指示がある: $left"


# === 23. yomiyasu に観点の語を言い換えさせない（.yomiyasu-words/allow.txt） ===
# yomiyasu は git の最上位の .yomiyasu-words/allow.txt の語を直さない。一覧の「正から写す語」の節は、レビュアーの依頼書の
# 「見てほしいこと」の 1 の括弧の語・2 の正しさの語・3・5 の見出しと、worker skill §2 の観点の見出しを並べたものと同じ。
# 4 は文（テストが主張を本当に検証しているか）で語ではないので写さない。項目が 1〜5 から変わったら、この検査と一覧を直す。
# allow_miss <loop.go> <worker SKILL.md> <allow.txt> <用語集 SKILL.md>: 食い違いを1行で出す（空なら合っている）
allow_miss() {
  local m='' src got all n x words
  n=$(grep -cE '^[[:space:]]*"[0-9]+\. ' "$1")
  [ "$n" = 5 ] || m="$m [reviewerBrief の見てほしいことが 1〜5 でない（$n 項目）。検査 23 と allow.txt を直す]"
  src=$( {
    sed -n 's/.*"1\. [^（"]*（\([^）]*\)）\\n".*/\1/p' "$1"
    sed -n 's/.*"2\. 正しさ: \([^\\"]*\)\\n".*/\1/p' "$1"
    sed -n 's/.*"3\. \([^\\"]*\)\\n".*/\1/p' "$1"
    sed -n 's/.*"5\. \([^:"]*\): .*/\1/p' "$1"
    sed -n '/^## 2\. /,/^## 3\. /p' "$2" | sed -n '/PR を出す前に、3つの観点を自分の差分に当てる/,/当てた結果は/p' | sed -n 's/^  - \*\*\([^*]*\)\*\*.*/\1/p'
  } | sed 's/・/\n/g' | sed '/^$/d' | sort -u)
  [ "$(wc -l <<<"$src")" -ge 9 ] || m="$m [正の語を読めない: $(tr '\n' ' ' <<<"$src")]"
  got=$(sed -n '/^# begin: 正から写す語$/,/^# end: 正から写す語$/p' "$3" | grep -vE '^[[:space:]]*(#|$)' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sort -u)
  for x in $(comm -23 <(echo "$src") <(echo "$got")); do m="$m [正の語「$x」が allow.txt に無い]"; done
  for x in $(comm -13 <(echo "$src") <(echo "$got")); do m="$m [allow.txt の「$x」が正に無い]"; done
  all=$(grep -vE '^[[:space:]]*(#|$)' "$3" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
  for x in 失敗の波及 境界 外向きの操作 前の版; do grep -qxF -- "$x" <<<"$all" || m="$m [allow.txt に「$x」が無い]"; done
  words=$(sed -n 's/^\*\*\([^*]*\)\*\*.*/\1/p' "$4" | sed 's/／/\n/g' | sed '/^$/d')
  [ "$(wc -l <<<"$words")" -ge 20 ] || m="$m [用語集の語を読めない]"
  for x in $(grep -xF -f <(echo "$words") <<<"$all"); do m="$m [用語集の語「$x」が allow.txt にある]"; done
  echo "$m"
}
w=package/skills/worker/SKILL.md d=internal/dispatcher/loop.go a=.yomiyasu-words/allow.txt g=package/skills/glossary/SKILL.md
miss=$(allow_miss "$d" "$w" "$a" "$g")
[ -z "$miss" ] && pass "yomiyasu の allow.txt の正から写す語が、レビュアーの依頼書と worker §2 の観点の語と同じで、外向きの操作・前の版を含み、用語集の語を含まない" \
  || fail "yomiyasu の allow.txt:$miss"
# 検査が当たる場合があること: 正の語を変えた・一覧から語を抜いた・用語集の語を足した写しでは落ちる
miss=
sed 's/正しさ: 並行性・失敗の波及・境界/正しさ: 並行性・失敗の波及・境界・後始末/' "$d" >"$tmp/loop.go"
[ -n "$(allow_miss "$tmp/loop.go" "$w" "$a" "$g")" ] || miss="$miss [依頼書に語を足した]"
sed 's/^  - \*\*前の版の形\*\*/  - **古い形**/' "$w" >"$tmp/worker.md"
[ -n "$(allow_miss "$d" "$tmp/worker.md" "$a" "$g")" ] || miss="$miss [worker の見出しを変えた]"
for x in 境界 外向きの操作; do
  grep -vxF -- "$x" "$a" >"$tmp/allow.txt"
  [ -n "$(allow_miss "$d" "$w" "$tmp/allow.txt" "$g")" ] || miss="$miss [allow.txt から「$x」を抜いた]"
done
{ cat "$a"; echo 台帳; } >"$tmp/allow.txt"
[ -n "$(allow_miss "$d" "$w" "$tmp/allow.txt" "$g")" ] || miss="$miss [allow.txt に台帳を足した]"
[ -z "$miss" ] && pass "yomiyasu の allow.txt: 正を変えた・語を抜いた・用語集の語を足した写しでは落ちる（検査が見ている）" || fail "yomiyasu の allow.txt: 写しで落ちない:$miss"

# === 24. 役の名前（ADR-0036 受け入れ条件 1・3） ===
# 用語集の役の項が「セルのオーナー」で、「**人**:」の項と「オーナー（人）」が package/ に無い。規約 10 は持ち場の CoS を指す
g=package/skills/glossary/SKILL.md
role_head='^\*\*セルのオーナー\*\*（略してオーナー）:$'
grep -q "$role_head" "$g" && ! grep -q '^\*\*人\*\*:' "$g" && ! grep -rqF 'オーナー（人）' package/ \
  && sed -n '/^\*\*セルのオーナー\*\*/,/^_避ける_/p' "$g" | grep -q '^_避ける_: 人（この役を指すとき）' \
  && sed -n '/^\*\*セルのオーナー\*\*/,/^_避ける_/p' "$g" | grep -q 'メンテナー' \
  && pass "ADR-0036 A1: 用語集の役の項は「セルのオーナー（略してオーナー）」で、「**人**:」の項と「オーナー（人）」が package/ に無い" \
  || fail "ADR-0036 A1: 用語集の役の項が替わっていないか、「**人**:」か「オーナー（人）」が残る"
# 否定の検査が当たる場合があること: 古い項と「オーナー（人）」の形には当たる
printf '%s\n' '**人**:' '仕事を頼み、決断が要るところだけ決める人。' >"$tmp/g-old.md"; printf '%s\n' 'オーナー（人）が任せた範囲' >"$tmp/o-old.md"
grep -q '^\*\*人\*\*:' "$tmp/g-old.md" && grep -qF 'オーナー（人）' "$tmp/o-old.md" && ! grep -q "$role_head" "$tmp/g-old.md" \
  && pass "ADR-0036 A1: 古い役の項と「オーナー（人）」には当たる（検査が見ている）" || fail "ADR-0036 A1: 検査が何も見ていない"
grep -q '^10\. \*\*持ち場の外は、その持ち場の CoS へ\*\*。' "$cos" && grep -q '^| 10\. 持ち場の外は、その持ち場の CoS へ |' "$cos" \
  && grep -q '^## ほかのプロジェクトへの委譲（規約10: 持ち場の外は、その持ち場の CoS へ）$' "$pb/triage.md" \
  && ! grep -rq '持ち場の外はオーナーへ' package/ \
  && pass "ADR-0036 A3: CoS の規約 10 と手引きの見出しは「持ち場の外は、その持ち場の CoS へ」" || fail "ADR-0036 A3: CoS の規約 10 が「持ち場の外はオーナーへ」のまま"

[ "$fails" = 0 ] && echo "all passed" || { echo "$fails failed" >&2; exit 1; }
