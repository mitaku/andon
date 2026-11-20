# tests/package/02-plugin.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   2. plugin.json の name が andon。Claude Code 系のプロファイルは command で plugin と役の skill を運び（本文で読ませない）、Codex 系は本文で skill を指す。
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
