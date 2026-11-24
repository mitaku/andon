# tests/package/38-wiki-index.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  38. 担当が wiki の目録を引く口（ADR-0061 受け入れ条件 11 のうち担当の skill の理解の工程と Chief of Staff の wiki の手引きの分）:
#      worker skill の理解の工程が目録を引き、読んだページの誤りを申告に書く。レビュアーの節（§6）には足していない。
#      Chief of Staff の wiki の手引きに、4 つ目の口（担当の理解の工程）と、目録の節の順（Overview・Entities・Topics・Sources）がある。
# === 38. wiki の目録を引く口（worker skill §2・§6、chief-of-staff playbooks/wiki.md） ===
# wiki_miss <worker SKILL.md> <wiki.md>: 足りないものを1行で出す（空なら合っている）
wiki_miss() {
  local m=''
  grep -q '^| 理解 |.*wiki の目録を引く（下）' "$1" || m="$m [理解の工程に目録を引くことが無い]"
  grep -q '^- \*\*wiki の目録を引く\*\*（理解の工程）。最初のメッセージに wiki の目録の行があれば、目録の Entities と Topics の節を読み' "$1" \
    || m="$m [理解の工程の目録を引く行が無い]"
  grep -q '^- 読んだ wiki のページが誤っていた.*申告.*`gaps`' "$1" || m="$m [読んだページの誤りを申告に書く行が無い]"
  sed -n '/^## 6\. /,/^## 7\. /p' "$1" | grep -q 'wiki' && m="$m [レビュアーの節（§6）に wiki がある]"
  grep -q '^- \*\*query\*\*: 引く口は4つ — .*・担当の理解の工程。' "$2" || m="$m [wiki の手引きに 4 つ目の口が無い]"
  grep -q '^  - `index.md` — .*節は Overview・Entities・Topics・Sources の順' "$2" || m="$m [wiki の手引きに目録の節の順が無い]"
  echo "$m"
}
pb_wiki=package/skills/chief-of-staff/playbooks/wiki.md
miss=$(wiki_miss package/skills/worker/SKILL.md "$pb_wiki")
[ -z "$miss" ] && pass "worker skill の理解の工程が wiki の目録を引き、レビュアーの節には無く、Chief of Staff の wiki の手引きに 4 つ目の口と目録の節の順がある" \
  || fail "wiki の目録を引く口:$miss"
# 検査が当たる場合があること: 理解の工程・目録の行・誤りの行を抜いた・レビュアーの節に足した・手引きの口を3つに戻した・順を消した写しでは落ちる
sed 's/wiki の目録を引く（下）。//' package/skills/worker/SKILL.md >"$tmp/wiki-w1.md"
sed '/^- \*\*wiki の目録を引く\*\*/d' package/skills/worker/SKILL.md >"$tmp/wiki-w2.md"
sed '/^- 読んだ wiki のページが誤っていた/d' package/skills/worker/SKILL.md >"$tmp/wiki-w3.md"
sed 's/^## 6\. レビュアーのとき$/&\nレビュアーも wiki の目録を引く。/' package/skills/worker/SKILL.md >"$tmp/wiki-w4.md"
sed 's/引く口は4つ/引く口は3つ/' "$pb_wiki" >"$tmp/wiki-p1.md"
sed 's/節は Overview・Entities・Topics・Sources の順に並べる/節は Sources・Entities・Topics/' "$pb_wiki" >"$tmp/wiki-p2.md"
wiki_caught=0
for w in "$tmp/wiki-w1.md" "$tmp/wiki-w2.md" "$tmp/wiki-w3.md" "$tmp/wiki-w4.md"; do
  if [ -n "$(wiki_miss "$w" "$pb_wiki")" ]; then wiki_caught=$((wiki_caught + 1)); fi
done
for p in "$tmp/wiki-p1.md" "$tmp/wiki-p2.md"; do
  if [ -n "$(wiki_miss package/skills/worker/SKILL.md "$p")" ]; then wiki_caught=$((wiki_caught + 1)); fi
done
[ "$wiki_caught" = 6 ] && pass "wiki の目録を引く口の検査は、理解の工程・目録の行・誤りの行・レビュアーの節・手引きの口・目録の順を崩した写しで落ちる" \
  || fail "wiki の目録を引く口の検査が、崩した写し 6 つのうち $wiki_caught つでしか落ちない"
