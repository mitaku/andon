# tests/package/07-issue-rename.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   7. 課題（issue）への改名（ADR-0008 受け入れ条件 5）: glossary の項・欄名・skill と template に旧語が無い。
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
sed -n '/^# <id> <タイトル>$/,/^```$/p' "$pb/brief.md" | sed -n '2p' | grep -q '^解決すべき問題:' \
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
code=$(grep -oE '"(wont-happen|one-off|not-applicable|read|first|done|gone|wiki|delegated)"' internal/triage/decisions.go | tr -d '"' | sort -u | tr '\n' ' ')
table=$(grep -oE '^\| (根拠|判断)の閉じ \| `[a-z-]+`' "$tsk" | grep -oE '`[a-z-]+`' | tr -d '`' | sort -u | tr '\n' ' ')
[ -n "$table" ] && [ "$code" = "$table" ] || miss="$miss 表の閉じ方（$table）がコード（$code）と違う"
grep -q '^   \*\*判断の閉じと課題の閉じを1件ずつ読む\*\*: `--dry-run` の出力の `review`' "$pb/triage.md" \
  && grep -q 'andon triage verdict <id> <key> overturn' "$pb/triage.md" && grep -q 'andon triage verdict <id> <key> ok' "$pb/triage.md" \
  || miss="$miss Chief of Staff の手引きに review を読んで verdict を書く手"
[ -z "$miss" ] && pass "triage skill に閉じ方の欄・見た commit・組み合わせの表、Chief of Staff の手引きに判断の閉じを読む手（ADR-0030）" || fail "トリアージの閉じ方の網:$miss"
grep -q 'andon issue stop-check' package/hooks/stop \
  && pass "package/hooks/stop が andon issue stop-check を打つ" || fail "package/hooks/stop が andon issue stop-check を打たない"
