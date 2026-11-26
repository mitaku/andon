# tests/package/43-deliverable-report.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp・pb は入口と前の節が置く）。
#  43. 成果が報告の課題の印（ADR-0066 受け入れ条件 8）: 検証の雛形の Settings に `- Deliverable: report` があり、雛形の下
#      （雛形の囲みを閉じたあと）に `kanban set <id> deliverable=report` がある。点検の課題の起動の手引き（idle.md）の、点検の課題を
#      起動する文にも同じ一手がある。
#      CoS の手引き brief.md の表と worker skill の表の deliverable の行に `report` がある。
#      どの検査も、その行を消した写しか、行を雛形の外へ動かした写しで当たることを確かめる。
# === 43. 成果が報告の課題の印（ADR-0066） ===
# report_miss <verify.md> <idle.md> <brief.md> <worker SKILL.md>: 足りないものを1行で出す（空なら合っている）
report_miss() {
  local m='' at='' close set n=0 inset=0 ln
  # 行は雛形の「   ## Settings」の直後に続く「   - 」の行の並びの中だけで探す（ファイルのどこかにあるだけでは足りない）
  while IFS= read -r ln; do
    n=$((n + 1))
    if [[ "$ln" == '   ## Settings' ]]; then inset=1; continue; fi
    if [ "$inset" = 1 ]; then
      if [[ "$ln" != '   - '* ]]; then inset=0
      elif [[ "$ln" == '   - Deliverable: report' && -z "$at" ]]; then at=$n
      fi
    fi
  done <"$1"
  if [ -z "$at" ]; then
    m="$m [検証の雛形の Settings に - Deliverable: report が無い]"
  else
    close=$(tail -n +"$at" "$1" | grep -nxF '   ```' | sed -n 1p | cut -d: -f1)
    set=$(tail -n +"$at" "$1" | grep -nF 'kanban set <id> deliverable=report' | sed -n 1p | cut -d: -f1)
    if [ -z "$close" ] || [ -z "$set" ] || [ "$set" -le "$close" ]; then
      m="$m [検証の雛形の下に kanban set <id> deliverable=report が無い]"
    fi
  fi
  # 点検の課題を起動する文（「点検の課題」と言う行）の中に、印を打つ一手がある（ファイルのどこかにあるだけでは足りない）
  grep -qF '`kanban set <id> deliverable=report`' <<<"$(grep -F '点検の課題' "$2")" || m="$m [点検の課題の起動に kanban set <id> deliverable=report が無い]"
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
# 雛形の Settings から外し、手順 3 の git archive の囲みへ動かした写しでも当たる（ファイルのどこかにあるだけでは足りない）
grep -vxF '   - Deliverable: report' "$rv" | sed 's/^   git archive <当てた commit> .*$/&\n   - Deliverable: report/' >"$tmp/r-verify.md"
grep -qxF '   - Deliverable: report' "$tmp/r-verify.md" || fail "ADR-0066 受け入れ条件 8: 動かした写しを作れない（手順 3 の git archive の行が無い）"
miss=$(report_miss "$tmp/r-verify.md" "$ri" "$rb" "$rw")
[[ "$miss" == *"[検証の雛形の Settings に - Deliverable: report が無い]"* ]] && pass "ADR-0066 受け入れ条件 8: 雛形の Settings の外へ動かした Deliverable の行で当たる" \
  || fail "ADR-0066 受け入れ条件 8: 雛形の Settings の外へ動かした Deliverable の行で当たらない: [$miss]"
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
# 点検の起動の文から外し、同じ文字列をファイルの末尾（点検の課題を言わない行）へ動かした写しでも当たる
{ sed 's/、起動の前に自分の端末で `kanban set <id> deliverable=report` を打ってから/、/' "$ri"; echo '注: `kanban set <id> deliverable=report`'; } >"$tmp/r-idle.md"
miss=$(report_miss "$rv" "$tmp/r-idle.md" "$rb" "$rw")
[[ "$miss" == *"[点検の課題の起動に kanban set <id> deliverable=report が無い]"* ]] && pass "ADR-0066 受け入れ条件 8: 点検の起動の文の外へ動かした kanban set の行で当たる" \
  || fail "ADR-0066 受け入れ条件 8: 点検の起動の文の外へ動かした kanban set の行で当たらない: [$miss]"
