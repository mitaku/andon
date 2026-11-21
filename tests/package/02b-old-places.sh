# tests/package/02b-old-places.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  2b. 前の置き場の名前が残っていない（ADR-0004 受け入れ条件 4 の前半）。
# === 2b. 前の置き場の名前が残っていない（ADR-0004 受け入れ条件 4 の前半） ===
# kanban の置き場は kanban/（KANBAN_ROOT）、coordinator.md は AGENTS.md の Chief of Staff の節、委譲は project.json の delegation。
# 移す前の名前を skill・プロファイル・フック・command・環境メモに書かない（package/gate.json の保護の表は対象の外）。
left=$(grep -rnE '\.flightdeck|FLIGHTDECK_ROOT|coordinator\.md|delegation\.(json|md)' package/skills package/templates package/hooks package/bin package/lib package/environment.md || true)
[ -z "$left" ] && pass "skill・プロファイル・フック・command に .flightdeck・coordinator.md・delegation.json が残っていない" || fail "前の置き場の名前が残る: $(head -3 <<<"$left")"
