# tests/package/40-later-owner-waits.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail は入口が置く）。
#  40. オーナー待ちを置いておく作り方（ADR-0062 決めたこと 10・11・12。受け入れ条件 18 のうち ask-one-by-one と Chief of Staff の手引きの分）。
#      ask-one-by-one は、作る口の旗（--now・--later・--until）と置いておく基準の 3 つと「迷ったら呼ぶ」、
#      置いてあるものを返事で聞かないこと、「あと何問」を呼んでいるものだけで数えること、「あとで」「今見る」の印の付け外しを書く。
#      start.md は呼んでいる行から問いを出し、watch.md は並びを呼んでいる行が先と書く。
#      無くなった文（旗の無い ask --new の例・並びの先頭から問いを出す文）の検査は、古い文を置いた写しで当たることも確かめる。
# === 40. オーナー待ちを置いておく作り方（ADR-0062） ===
aobo=package/skills/ask-one-by-one/SKILL.md
sec() { sed -n "/^## $1\\. /,/^## $(($1 + 1))\\. /p" "$2"; }
l2=$(sec 2 "$aobo"); l3=$(sec 3 "$aobo"); l5=$(sed -n '/^## 5\. /,$p' "$aobo")
grep -qF -- '`--now`（今呼ぶ）か `--later "<置いておく理由>"`（置いておく）のどちらか1つを必ず付ける' <<<"$l3" \
  && grep -qF -- '`--until <YYYY-MM-DD>` に、答えが要る日の前の日を書く' <<<"$l3" \
  && grep -qF -- '`--later` を打てるのは Chief of Staff の端末だけ' <<<"$l3" \
  && pass "ADR-0062 A18: ask-one-by-one §3 は ask --new の --now・--later・--until と、--later を打てる端末を書く" \
  || fail "ADR-0062 A18: ask-one-by-one §3 に作る口の旗の手順が無い"
grep -qF '**置いておくのは、次の3つを全部満たすときだけにし、迷ったら呼ぶ**' <<<"$l3" \
  && grep -qxF '1. 答えが出なくても、動いている課題が止まらない。' <<<"$l3" \
  && grep -qxF '2. 後ろで待つ課題が無いか、あっても続きの手順や観察で、それも急がない。' <<<"$l3" \
  && grep -qxF '3. 期限が無いか、先にある。期限が先にあるものは、`later_until` に答えが要る日の前の日を書く。' <<<"$l3" \
  && pass "ADR-0062 A18: ask-one-by-one §3 は置いておく基準の 3 つと「迷ったら呼ぶ」を書く" \
  || fail "ADR-0062 A18: ask-one-by-one §3 に置いておく基準が無い"
grep -qF '置いておくなら同じ `kanban set` に `later="<理由>"` を足す（印の無い手作業は呼ぶ）' <<<"$l3" \
  && pass "ADR-0062 A18: ask-one-by-one §3 は手作業を置いておくときの印 later を書く" \
  || fail "ADR-0062 A18: ask-one-by-one §3 に手作業の印 later が無い"
grep -qF '置いてあるオーナー待ち（`call` が偽）は返事で聞かない' <<<"$l2" && grep -qF '呼んでいるものだけを数える' <<<"$l2" \
  && grep -qF '出すのは呼んでいる行（`call` が真）だけ' <<<"$l3" \
  && pass "ADR-0062 A18: ask-one-by-one §2・§3 は呼んでいるものだけを返事で聞き、「あと何問」も呼んでいるものだけで数える" \
  || fail "ADR-0062 A18: ask-one-by-one §2・§3 が置いてあるものを返事で聞かないと書いていない"
grep -qF '返事で1問ずつ聞かない' <<<"$l5" \
  && grep -qF 'オーナーが「あとで」と言ったオーナー待ちには、`kanban set <id> later="<オーナーの言葉>（オーナー <YYYY-MM-DD>）"` を書く' <<<"$l5" \
  && grep -qF 'オーナーが「今見る」と言えば、`kanban set <id> later= later_until=` で消し' <<<"$l5" \
  && pass "ADR-0062 A18: ask-one-by-one §5 は置いてあるものを聞かず、「あとで」「今見る」で印を付け外す" \
  || fail "ADR-0062 A18: ask-one-by-one §5 に「あとで」「今見る」の扱いが無い"
startmd=package/skills/chief-of-staff/playbooks/start.md
grep -qF '`items` のうち呼んでいる行（`call` が真）の並びの先頭から問いを出す' "$startmd" \
  && grep -qF '呼んでいる行〈`call` が真〉が先、置いてある行が後' package/skills/chief-of-staff/playbooks/watch.md \
  && pass "ADR-0062 A18: start.md は呼んでいる行から問いを出し、watch.md は並びを呼んでいる行が先と書く" \
  || fail "ADR-0062 A18: start.md か watch.md が呼んでいる行を指していない"
# 無くなった文: 旗の無い ask --new の例と、並びの先頭から（呼んでいるかを見ずに）問いを出す文
old_lines='andon issue ask --new - |andon issue ask --new -`|`items` の並びの先頭から問いを出す'
left=$(grep -nE "$old_lines" "$aobo" "$startmd" package/skills/chief-of-staff/playbooks/cut.md || true)
[ -z "$left" ] && pass "ADR-0062 A18: ask-one-by-one・start.md・cut.md に旗の無い ask --new と、並びの先頭から問いを出す古い文が無い" \
  || fail "ADR-0062 A18: 古い文が残る: $left"
# 検査が当たる場合があること: 古い文を置いた写しで、同じ grep が 3 行とも拾う
mkdir -p "$tmp/later"
printf '%s\n' "andon issue ask --new - <<'EOF'" '問いの JSON を `andon issue ask --new -` に渡し' '`items` の並びの先頭から問いを出す（出す順）' > "$tmp/later/old.md"
[ "$(grep -cE "$old_lines" "$tmp/later/old.md")" = 3 ] \
  && pass "ADR-0062 A18: 古い文の検査は、古い文の写しで 3 行とも当たる" || fail "ADR-0062 A18: 古い文の検査が古い文の写しに当たらない"
