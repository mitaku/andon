# tests/package/43-deliverable-report.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp・pb は入口と前の節が置く）。
#  43. 成果が報告の課題の印（ADR-0066 受け入れ条件 8）: 検証の雛形の Settings に `- Deliverable: report` があり、雛形の下
#      （雛形の囲みを閉じたあと）に `kanban set <id> deliverable=report` がある。点検の課題の起動の手引き（idle.md）にも同じ行がある。
#      CoS の手引き brief.md の表と worker skill の表の deliverable の行に `report` がある。
#      どの検査も、その行を消した写しか、行を雛形の外へ動かした写しで当たることを確かめる。
# === 43. 成果が報告の課題の印（ADR-0066） ===
# report_miss <verify.md> <idle.md> <brief.md> <worker SKILL.md>: 足りないものを1行で出す（空なら合っている）
report_miss() {
  local m='' at close set
  at=$(grep -nxF '   - Deliverable: report' "$1" | sed -n 1p | cut -d: -f1)
  if [ -z "$at" ]; then
    m="$m [検証の雛形の Settings に - Deliverable: report が無い]"
  else
    close=$(tail -n +"$at" "$1" | grep -nxF '   ```' | sed -n 1p | cut -d: -f1)
    set=$(tail -n +"$at" "$1" | grep -nF 'kanban set <id> deliverable=report' | sed -n 1p | cut -d: -f1)
    if [ -z "$close" ] || [ -z "$set" ] || [ "$set" -le "$close" ]; then
      m="$m [検証の雛形の下に kanban set <id> deliverable=report が無い]"
    fi
  fi
  grep -qF '`kanban set <id> deliverable=report`' "$2" || m="$m [点検の課題の起動に kanban set <id> deliverable=report が無い]"
  grep -qE '^\| `deliverable`（Deliverable） \| `pr` / `candidates` / `report` \|.*`kanban set <id> deliverable=report`' "$3" \
    || m="$m [brief.md の表の deliverable の行に report が無い]"
  grep -qE '^   \| `deliverable`（Deliverable） \| `pr` / `candidates` / `report` \|.*`report` は' "$4" \
    || m="$m [worker skill の表の deliverable の行に report が無い]"
  echo "$m"
}
rv=$pb/verify.md ri=$pb/idle.md rb=$pb/brief.md rw=package/skills/worker/SKILL.md
miss=$(report_miss "$rv" "$ri" "$rb" "$rw")
[ -z "$miss" ] && pass "ADR-0066 受け入れ条件 8: 検証の雛形と点検の起動に deliverable=report の印があり、brief.md と worker skill の表に report の行がある" \
  || fail "ADR-0066 受け入れ条件 8:$miss"
# 検査が当たる場合があること: 雛形の行を消した写し・kanban set の行を雛形の中へ動かした写し・表の report を消した写し
grep -vxF '   - Deliverable: report' "$rv" >"$tmp/r-verify.md"
miss=$(report_miss "$tmp/r-verify.md" "$ri" "$rb" "$rw")
[[ "$miss" == *"[検証の雛形の Settings に - Deliverable: report が無い]"* ]] && pass "ADR-0066 受け入れ条件 8: 雛形の Deliverable の行を消した写しで当たる" \
  || fail "ADR-0066 受け入れ条件 8: 雛形の Deliverable の行を消した写しで当たらない: [$miss]"
grep -vF 'kanban set <id> deliverable=report' "$rv" | sed 's/^   - Deliverable: report$/   - Deliverable: report\n   kanban set <id> deliverable=report/' >"$tmp/r-verify.md"
miss=$(report_miss "$tmp/r-verify.md" "$ri" "$rb" "$rw")
[[ "$miss" == *"[検証の雛形の下に kanban set <id> deliverable=report が無い]"* ]] && pass "ADR-0066 受け入れ条件 8: kanban set の行を雛形の中へ動かした写しで当たる" \
  || fail "ADR-0066 受け入れ条件 8: kanban set の行を雛形の中へ動かした写しで当たらない: [$miss]"
sed 's/`pr` \/ `candidates` \/ `report`/`pr` \/ `candidates`/' "$rb" >"$tmp/r-brief.md"
sed 's/`pr` \/ `candidates` \/ `report`/`pr` \/ `candidates`/' "$rw" >"$tmp/r-worker.md"
miss=$(report_miss "$rv" "$ri" "$tmp/r-brief.md" "$tmp/r-worker.md")
[[ "$miss" == *"[brief.md の表の deliverable の行に report が無い]"* && "$miss" == *"[worker skill の表の deliverable の行に report が無い]"* ]] \
  && pass "ADR-0066 受け入れ条件 8: 表の値から report を消した写しで当たる" \
  || fail "ADR-0066 受け入れ条件 8: 表の値から report を消した写しで当たらない: [$miss]"
grep -vF 'deliverable=report' "$ri" >"$tmp/r-idle.md"
miss=$(report_miss "$rv" "$tmp/r-idle.md" "$rb" "$rw")
[[ "$miss" == *"[点検の課題の起動に kanban set <id> deliverable=report が無い]"* ]] && pass "ADR-0066 受け入れ条件 8: 点検の起動の行を消した写しで当たる" \
  || fail "ADR-0066 受け入れ条件 8: 点検の起動の行を消した写しで当たらない: [$miss]"
