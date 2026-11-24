# tests/package/38-triage-gaps.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  38. トリアージが知識の欠けの申告を仕分ける（ADR-0061 受け入れ条件 11 のうち skill triage の分）: skill triage の項目の種類に gap があり、
#      判定表に申告の行（skill とパッケージの誤りと 2 件の直し・1 件目）と、閉じ方の表に first（gap と、ADR-0062 で広げた remember・memory・next）がある。
#      Chief of Staff のトリアージの手引きが 1 件目を読み、申告の写しを raw/gaps/ に置き、wiki の手引きの元資料が 5 種になっている。
# === 38. トリアージの申告の仕分け（skill triage・Chief of Staff の triage.md と wiki.md） ===
# tgaps_miss <triage SKILL.md> <triage.md> <wiki.md>: 足りないものを1行で出す（空なら合っている）
tgaps_miss() {
  local m=''
  grep -q '^  `gap`（知識の欠けの申告。.*`report.json` の `gaps`' "$1" || m="$m [§2 の種類に gap が無い]"
  grep -q '^- \*\*`gap`\*\*（知識の欠けの申告。' "$1" || m="$m [種類ごとの決まりに gap が無い]"
  grep -q '^  | g5 | Andon の skill とパッケージで、種類が `wrong` で.*| `fix`' "$1" || m="$m [判定表に確かめた誤りの直しが無い]"
  grep -q '^  | g6 | Andon の skill とパッケージで、種類が `missing` か `hard_to_find` で、同じ欠けの根拠が別々に 2 件ある | `fix`' "$1" || m="$m [判定表に根拠 2 件の直しが無い]"
  grep -q '^  | g7 | .*根拠が 1 件 | `close`・`closing: first`' "$1" || m="$m [判定表に 1 件目が無い]"
  grep -q '^  | g4 | wiki のページ | `close`・`closing: wiki`' "$1" || m="$m [判定表に wiki の行が無い]"
  grep -qF '| 判断の閉じ | `first` | 1 件目 | `gap`・`remember`・`memory`・`next` だけ |' "$1" || m="$m [閉じ方の表に first が無い]"
  grep -q '1 件目（`first`）\|「1 件目」（`first`）' "$2" || m="$m [Chief of Staff のトリアージの手引きが 1 件目を読まない]"
  grep -qF 'wiki/raw/gaps/' "$2" || m="$m [Chief of Staff のトリアージの手引きに申告の写しが無い]"
  grep -q '^  - `raw/` — 元資料。書いたら変えない。5種: .*(5) トリアージで wiki 行きになった知識の欠けの申告の写し（`raw/gaps/' "$3" || m="$m [wiki の手引きの元資料に申告の写しが無い]"
  echo "$m"
}
tsk=package/skills/triage/SKILL.md
tpb=package/skills/chief-of-staff/playbooks/triage.md
wpb=package/skills/chief-of-staff/playbooks/wiki.md
miss=$(tgaps_miss "$tsk" "$tpb" "$wpb")
[ -z "$miss" ] && pass "skill triage に種類 gap・申告の判定表・閉じ方 first があり、Chief of Staff の手引きが 1 件目を読み、申告を raw/gaps/ に写す" \
  || fail "トリアージの申告の仕分け:$miss"
# 検査が当たる場合があること: 種類・判定表の行・閉じ方・手引きの1件目・元資料の種を崩した写しでは落ちる
sed '/^  `gap`（知識の欠けの申告。/d' "$tsk" >"$tmp/tg1.md"
sed '/^  | g7 | /d' "$tsk" >"$tmp/tg2.md"
sed '/^| 判断の閉じ | `first` |/d' "$tsk" >"$tmp/tg3.md"
sed 's/「1 件目」（`first`）/1件目/' "$tpb" >"$tmp/tg4.md"
sed 's/元資料。書いたら変えない。5種: /元資料。書いたら変えない。4種: /' "$wpb" >"$tmp/tg5.md"
tg_caught=0
for c in "$tmp/tg1.md|$tpb|$wpb" "$tmp/tg2.md|$tpb|$wpb" "$tmp/tg3.md|$tpb|$wpb" "$tsk|$tmp/tg4.md|$wpb" "$tsk|$tpb|$tmp/tg5.md"; do
  IFS='|' read -r a b c3 <<<"$c"
  if [ -n "$(tgaps_miss "$a" "$b" "$c3")" ]; then tg_caught=$((tg_caught + 1)); fi
done
[ "$tg_caught" = 5 ] && pass "申告の仕分けの検査は、種類・判定表・閉じ方・手引きの 1 件目・元資料の種を崩した写しで落ちる" \
  || fail "申告の仕分けの検査が、崩した写し 5 つのうち $tg_caught つでしか落ちない"
