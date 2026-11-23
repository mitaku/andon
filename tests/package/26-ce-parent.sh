# tests/package/26-ce-parent.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置き、pb は前の節が置く）。
#  26. Chief Engineer の文書から切った課題を parent で結ぶ（ADR-0042 受け入れ条件 13・決めたこと 15）: 手引き cut.md の、
#      文書の「依頼書に切る案」から切る段（「Chief Engineer の文書を受けたら」の 4）の kanban add に --parent <Chief Engineer の id> がある。
# === 26. 文書から切った課題を parent で結ぶ（ADR-0042 受け入れ条件 13） ===
ce_cut=$(sed -n '/^## Chief Engineer の文書を受けたら$/,/^## /p' "$pb/cut.md" | sed -n '/^4\. /,/^5\. /p')
grep -q '^4\. 「依頼書に切る案」を縦に薄く' <<<"$ce_cut" \
  && grep -q '`kanban add "<タイトル>" .*--parent <Chief Engineer の id>' <<<"$ce_cut" \
  && pass "cut.md の文書から課題を切る段（Chief Engineer の文書を受けたら 4）の kanban add に --parent <Chief Engineer の id> がある" \
  || fail "cut.md の文書から課題を切る段に --parent <Chief Engineer の id> が無い"
