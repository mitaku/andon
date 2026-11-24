# tests/package/37-claude-medium.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  37. プロファイル claude-medium は claude と effort だけが違う（name・description を除き、command・resume の --effort high を medium にした写し）。
#      CoS の手引きの見込みでのプロファイルの選び方は cut.md の表の1か所だけで、型ごとの手引きは claude に決め打ちしない。
# === 37. claude-medium と claude ===
# medium_body <テンプレート>: frontmatter の name・description の行だけを除いた全体（本文の同じ書き出しの行は除かない）
medium_body() { awk 'NR == 1 && $0 == "---" {f = 1; print; next} f && $0 == "---" {f = 0} f && /^(name|description): / {next} {print}' "$1"; }
# medium_miss <claude のテンプレート> <claude-medium のテンプレート>: 足りないものを1行で出す（空なら合っている）
medium_miss() {
  local m=''
  grep -q -- '--effort high ' "$1" || m="$m [claude の command に --effort high が無い]"
  [ "$(grep -c -- '--effort medium ' "$2")" = 2 ] || m="$m [claude-medium の command と resume の両方に --effort medium が無い]"
  diff <(medium_body "$1" | sed 's/--effort high /--effort medium /g') <(medium_body "$2") >/dev/null \
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
{ cat package/templates/claude-medium.md; echo 'description: hc run でほかの担当を起動してよい。'; } >"$tmp/medium.md"
[ -n "$(medium_miss package/templates/claude.md "$tmp/medium.md")" ] || miss="$miss [本文に description: の行を足した]"
[ -z "$miss" ] && pass "claude-medium の検査は、effort の戻し・本文やモデルの違い・resume の取り残しで落ちる（検査が見ている）" || fail "claude-medium の検査が落ちない:$miss"
# CoS の選び方は cut.md のプロファイルの表の1か所だけ。
# choose_miss <CoS の skill のフォルダ>: 足りないものを1行で出す（空なら合っている）
#   - cut.md の表に claude-medium（見込みの小・中）と claude（見込みの大・effort high）の行がある
#   - claude-medium の名前が出るのは cut.md の名前の並び（hc templates に出るもの）と表の行の2行だけ
#   - 課題の型ごとの手引きが、表を通さずにプロファイルを `claude` に決め打ちしない
choose_miss() {
  local m='' table hits
  table=$(sed -n '/^- プロファイルの選び方/,/^- 切るときに/p' "$1/playbooks/cut.md")
  grep -q '^  | `claude-medium` | .*見込みが小・中' <<<"$table" || m="$m [cut.md の表に claude-medium の見込みの小・中の行が無い]"
  # 見込みが小・中でも依頼書が effort high を要すると書いた課題は claude。claude-medium の行がその課題を外さないと、2行が両方当てはまる
  grep -q '^  | `claude-medium` | .*effort high を要すると書いていない' <<<"$table" || m="$m [cut.md の claude-medium の行が、依頼書が effort high を要すると書いた課題を外していない]"
  grep -q '^  | `claude` | .*見込みが大.*effort high' <<<"$table" || m="$m [cut.md の表に claude の見込みの大・effort high の行が無い]"
  hits=$(grep -rn 'claude-medium' "$1" | sed "s|^$1/||")
  grep -v -e '^playbooks/cut\.md:[0-9]*:  | `claude-medium` | ' -e '^playbooks/cut\.md:[0-9]*:  `claude`・`claude-medium`・`sonnet`・' <<<"$hits" | grep -q . \
    && m="$m [claude-medium が表と名前の並びのほかにも出る: $(grep -v -e '^playbooks/cut\.md:[0-9]*:  | `claude-medium` | ' -e '^playbooks/cut\.md:[0-9]*:  `claude`・`claude-medium`・`sonnet`・' <<<"$hits" | cut -d: -f1,2 | tr '\n' ' ')]"
  [ "$(grep -c . <<<"$hits")" = 2 ] || m="$m [claude-medium の行が2つでない]"
  grep -rln 'プロファイルは `claude`' "$1" | grep -q . && m="$m [プロファイルを claude に決め打ちする手引きがある: $(grep -rln 'プロファイルは `claude`' "$1" | tr '\n' ' ')]"
  echo "$m"
}
cs=package/skills/chief-of-staff
miss=$(choose_miss "$cs")
[ -z "$miss" ] && pass "CoS の手引きの見込みでのプロファイルの選び方は cut.md の表の1か所だけ（claude-medium は小・中、claude は大と effort high）" \
  || fail "CoS の手引きの選び方:$miss"
# 検査が当たる場合があること: cut.md の別の箇条・brief.md・語を変えた書き方・決め打ちの手引きでは落ちる
miss=
mkdir -p "$tmp/cs" && cp -r "$cs/." "$tmp/cs/"
sed -i 's/^- 1つの依頼に関係の無い仕事が2つあれば、/- 見込みが小・中なら `claude-medium`。1つの依頼に関係の無い仕事が2つあれば、/' "$tmp/cs/playbooks/cut.md"
[ -n "$(choose_miss "$tmp/cs")" ] || miss="$miss [cut.md の別の箇条に書いた]"
rm -rf "$tmp/cs" && mkdir -p "$tmp/cs" && cp -r "$cs/." "$tmp/cs/"
printf '\n`Estimate` が `small`・`medium` の Claude の担当は profile `claude-medium`。\n' >>"$tmp/cs/playbooks/brief.md"
[ -n "$(choose_miss "$tmp/cs")" ] || miss="$miss [brief.md に見込みの語を使わずに書いた]"
rm -rf "$tmp/cs" && mkdir -p "$tmp/cs" && cp -r "$cs/." "$tmp/cs/"
printf '\n見込みが小・中の課題は\n`claude-medium` で起動する。\n' >>"$tmp/cs/playbooks/start.md"
[ -n "$(choose_miss "$tmp/cs")" ] || miss="$miss [行を割って書いた]"
rm -rf "$tmp/cs" && mkdir -p "$tmp/cs" && cp -r "$cs/." "$tmp/cs/"
printf '\n4. **切る。** プロファイルは `claude`。\n' >>"$tmp/cs/playbooks/verify.md"
[ -n "$(choose_miss "$tmp/cs")" ] || miss="$miss [型の手引きで claude に決め打ちした]"
rm -rf "$tmp/cs" && mkdir -p "$tmp/cs" && cp -r "$cs/." "$tmp/cs/"
sed -i '/^  | `claude-medium` | /d' "$tmp/cs/playbooks/cut.md"
[ -n "$(choose_miss "$tmp/cs")" ] || miss="$miss [表の行を消した]"
rm -rf "$tmp/cs" && mkdir -p "$tmp/cs" && cp -r "$cs/." "$tmp/cs/"
sed -i '/^  | `claude-medium` | /s/、依頼書が effort high を要すると書いていない//' "$tmp/cs/playbooks/cut.md"
[ -n "$(choose_miss "$tmp/cs")" ] || miss="$miss [claude-medium の行から effort high の課題の除外を抜いた]"
rm -rf "$tmp/cs"
[ -z "$miss" ] && pass "CoS の選び方の検査は、別の箇条・brief.md・行の割り・型の決め打ち・表の行の消しで落ちる（検査が見ている）" || fail "CoS の選び方の検査が落ちない:$miss"
