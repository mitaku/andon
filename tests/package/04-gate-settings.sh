# tests/package/04-gate-settings.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   4. gate.json が schema/gate.schema.json の条件表に合う。settings.json が JSON で、statusLine が package の statusline を指す。全フックの command が package に在るファイルだけを指す。
#      package/hooks/ の全部のフックの最初の実行行が「andon が無ければ通す」の行（ADR-0034 受け入れ条件 3）。
#      Codex の rules（package/codex.rules）の forbidden が settings の deny（コマンドの頭で引く線）と同じ。
#      settings の deny が人のファイル（ADR-0005 決めたこと 4 の (c)）の書き換えを Edit(//workspace/…) で拒む。
#      settings の deny が force-with-lease で main に当たる形を断り、自分の branch への形は allow のまま（当たる・当たらないの例）。
#  12. auto memory（ADR-0020 受け入れ条件 2・6。4 と 6 の節の中）: settings が入れる向き・環境メモの書き方・chief-of-staff の規約9・トリアージ・wiki の手引き。
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
