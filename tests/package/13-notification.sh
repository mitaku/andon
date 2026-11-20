# tests/package/13-notification.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  13. herdr notification show の文言（ADR-0007 R3）。
# === 13. herdr notification show の文言（ADR-0007 R3・B7）===
miss=
for f in package/environment.md "$pb/watch.md" "$pb/triage.md"; do
  grep -n 'herdr notification show' "$f" | grep -q 'TUI' || miss="$miss $f"
  grep -q 'kanban の問いとボードで足りる' "$f" || miss="$miss $f(足りる)"
done
left=$(grep -n 'herdr notification show' "$cos" "$pb"/*.md | grep -v 'TUI' || true); [ -n "$left" ] && miss="$miss TUI の無い行:$left"
[ -z "$miss" ] && pass "environment.md と chief-of-staff の手引き（watch.md・triage.md）が herdr notification show を「TUI が動いているときだけ」と書く" || fail "herdr notification show の文言:$miss"

# hc の手引き（A-0192）: 全役が読む environment.md に「hc --skill を読む」の1行がある
grep -q 'skill に書いていない `hc` の操作が要るときだけ、先に `hc --skill`' package/environment.md \
  && sed -n '/^## 起動したら1回$/,/^## 毎ターン/p' "$pb/wake.md" | grep -q '`hc --skill` を1回読む' \
  && pass "environment.md は要るときだけ、chief-of-staff の wake.md は起動したときに hc --skill を読む" || fail "hc --skill を読む行（environment.md か chief-of-staff の wake.md）が無い"
