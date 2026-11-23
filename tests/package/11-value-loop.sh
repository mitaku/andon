# tests/package/11-value-loop.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  11. 価値のループ（ADR-0011 受け入れ条件 8）: CE skill の価値の仮説の 6 項目・CoS skill の空なら戻す・観察の課題・signal の受け方・worker skill の観察の課題（observation の例は schema に通す）。
#      CoS の規約7（write_back の出所への finish の1コメントは委譲の中。決めたこと 15）。
# === 11. 価値のループ（ADR-0011 受け入れ条件 8・決めたこと 4・7・12・13） ===
ce=package/skills/chief-engineer/SKILL.md
ce_form=$(block "$ce" '# <タイトル>（仕様・設計。')
miss=
grep -q '^## 価値の仮説（下の 6 項目。利用者に見えないテーマは「無し: <理由>」の1行）$' <<<"$ce_form" || miss="$miss 文書の形の見出し"
i=0
for k in '誰の何が変わるか' '見る信号（' '見る日（絶対の日付' '誰が要るか' '効いたとどう分かるか' '何があれば止めるか'; do
  i=$((i + 1)); grep -q "^$i\. $k" "$ce" || miss="$miss ($i)$k"
done
grep -q '「無し: <理由>」' "$ce" && grep -q '観察の結果が「外れた」「分からない」でも、ラインは何もしない' "$ce" || miss="$miss 無し・ラインは何もしない"
grep -q 'P6 で形を決める\|P6 で形が決まるまで' "$ce" && miss="$miss P6 の仮の行が残る"
[ -z "$miss" ] && pass "chief-engineer skill の文書の形に価値の仮説の 6 項目と「無し: <理由>」がある" || fail "chief-engineer skill の価値の仮説:$miss"
cos1=$(cat "$pb/cut.md")
miss=
grep -q '「価値の仮説」の節が見出しだけ（空）なら、\*\*切らずに Chief Engineer へ戻す\*\*' <<<"$cos1" || miss="$miss 空なら戻す"
grep -q '`kanban add "観察: <タイトル>の価値の仮説" --parent <Chief Engineer の id>' <<<"$cos1" && grep -q '`kanban set <新しい id> not_before=<(3) の日付>`' <<<"$cos1" || miss="$miss 観察の課題を --parent と not_before で足す"
grep -q '^## `signal` の通知を受けたら$' <<<"$cos1" && grep -q '^2\. \*\*直す課題に切る\*\*.*`links` に信号の `url` と `key`' <<<"$cos1" || miss="$miss signal を直す課題に切る"
# ADR-0051 決めたこと 8・受け入れ条件 9: 切る前に release から壊した変更の候補の PR を引き、依頼書と links に書く
grep -q '^1\. \*\*`release` があれば、切る前に壊した変更の候補を引く。\*\* .*`gh api --hostname <host> repos/<owner>/<repo>/commits/<release>/pulls' <<<"$cos1" &&
  grep -q '依頼書の「解決すべき問題」に、失敗が始まった変更の候補として、PR の URL' <<<"$cos1" &&
  grep -q '1 で引いた PR（か commit）の URL も足す' <<<"$cos1" || miss="$miss signal の release から壊した変更の候補を引く"
grep -q '^## 観察の課題の報告を受けたら$' <<<"$cos1" && grep -q '\*\*ラインは何もしない\*\*' <<<"$cos1" || miss="$miss 観察の報告をオーナーへ返すだけ"
block "$pb/brief.md" '# <id> 観察: ' | grep -q '^完了条件: report.json の observation' || miss="$miss 観察の課題の雛形"
grep -q '^| `signal` の行' "$pb/watch.md" || miss="$miss news の表の signal の行"
[ -z "$miss" ] && pass "chief-of-staff skill に、空なら戻す・観察の課題を --parent と not_before で足す・signal を直す課題に切る、の行がある" || fail "chief-of-staff skill の価値のループ:$miss"
grep -q '^| 7\. 課題は kanban が既定 | 4・6 | `cut.md` |$' "$cos" \
  && grep -q '^  `write_back: true` の出所への `andon issue finish` の1コメントは委譲の中。それ以外のトラッカーへの書き込み（状態の変更・途中のコメント）はオーナーの go' <<<"$cos1" \
  && grep -q '`write_back: true` の出所（今は `github-issues`）の issue は、`links` に URL' <<<"$cos1" \
  && pass "chief-of-staff の規約7（cut.md）が write_back の1コメントを委譲の中に、ほかの書き込みをオーナーの go に分ける（ADR-0011 決めたこと 15）" \
  || fail "chief-of-staff skill の規約7・links の URL（write_back）"
w=package/skills/worker/SKILL.md
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w")
o_blk=$(block "$w" 'andon issue report "$id" --data '"'"'{"observation"' | head -1 | sed "s/^[^']*'//; s/'.*\$//")
printf '%s\n' "$o_blk" >"$tmp/o.json"
grep -q '^### 観察の課題のとき$' <<<"$w2" && grep -q '\*\*直さない・課題を切らない・出所へ書き戻さない\*\*' <<<"$w2" && grep -q '^2\. `kanban review <id>`' <<<"$w2" \
  && [ -n "$o_blk" ] && jq -e '.observation' "$tmp/o.json" >/dev/null 2>&1 && chk report "$tmp/o.json" \
  && pass "worker skill に観察の課題の節があり、observation の例は report.schema.json に合う" || fail "worker skill の観察の課題: $(head -1 "$tmp/why" 2>/dev/null) $o_blk"
