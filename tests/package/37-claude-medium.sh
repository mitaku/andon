# tests/package/37-claude-medium.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  37. プロファイル claude-medium は claude と effort だけが違う（name・description を除き、command・resume の --effort high を medium にした写し）。
#      CoS の cut.md のプロファイルの表が claude-medium を見込みの小・中で選び、claude を見込みの大と effort high を要する課題で選ぶ。
# === 37. claude-medium と claude ===
# medium_miss <claude のテンプレート> <claude-medium のテンプレート>: 足りないものを1行で出す（空なら合っている）
medium_miss() {
  local m=''
  grep -q -- '--effort high ' "$1" || m="$m [claude の command に --effort high が無い]"
  [ "$(grep -c -- '--effort medium ' "$2")" = 2 ] || m="$m [claude-medium の command と resume の両方に --effort medium が無い]"
  diff <(grep -vE '^(name|description): ' "$1" | sed 's/--effort high /--effort medium /g') <(grep -vE '^(name|description): ' "$2") >/dev/null \
    || m="$m [claude-medium が claude と effort のほかでも違う]"
  echo "$m"
}
miss=$(medium_miss package/templates/claude.md package/templates/claude-medium.md)
[ -z "$miss" ] && pass "プロファイル claude-medium は claude と effort（high → medium）だけが違う" || fail "プロファイル claude-medium:$miss"
# 検査が当たる場合があること: effort を high に戻した・本文を変えた・resume だけ high に残した写しでは落ちる
miss=
sed 's/--effort medium /--effort high /g' package/templates/claude-medium.md >"$tmp/medium.md"
[ -n "$(medium_miss package/templates/claude.md "$tmp/medium.md")" ] || miss="$miss [effort を high に戻した]"
sed 's/裏で走らせ続けない。/裏で走らせない。/' package/templates/claude-medium.md >"$tmp/medium.md"
[ -n "$(medium_miss package/templates/claude.md "$tmp/medium.md")" ] || miss="$miss [本文を変えた]"
sed '/^resume: /s/--effort medium /--effort high /' package/templates/claude-medium.md >"$tmp/medium.md"
[ -n "$(medium_miss package/templates/claude.md "$tmp/medium.md")" ] || miss="$miss [resume だけ high に残した]"
sed 's/--model opus /--model sonnet /' package/templates/claude-medium.md >"$tmp/medium.md"
[ -n "$(medium_miss package/templates/claude.md "$tmp/medium.md")" ] || miss="$miss [モデルを変えた]"
[ -z "$miss" ] && pass "claude-medium の検査は、effort の戻し・本文やモデルの違い・resume の取り残しで落ちる（検査が見ている）" || fail "claude-medium の検査が落ちない:$miss"
# CoS の選び方は cut.md のプロファイルの表の1か所だけ
table=$(sed -n '/^- プロファイルの選び方/,/^- 切るときに/p' "$pb/cut.md")
if grep -q '^  | `claude-medium` | .*見込みが小・中' <<<"$table" && grep -q '^  | `claude` | .*見込みが大.*effort high' <<<"$table"; then
  pass "cut.md のプロファイルの表が claude-medium を見込みの小・中、claude を見込みの大と effort high で選ぶ"
else
  fail "cut.md のプロファイルの表に見込みでの選び方が無い"
fi
left=$(grep -rln 'claude-medium.*見込み\|見込み.*claude-medium' package/skills/chief-of-staff | grep -vx "$pb/cut.md")
[ -z "$left" ] && pass "見込みでの claude-medium の選び方は cut.md の1か所だけ" || fail "見込みでの claude-medium の選び方が cut.md のほかにもある: $left"
