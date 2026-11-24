# tests/package/41-triage-later.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・tmp は入口が置く）。
#  41. トリアージと振り返りが急がないオーナー待ちを置いておく（ADR-0062 決めたこと 10・11・13・14。受け入れ条件 18 のうち skill triage と手引きと約束の分）。
#      skill triage は置いておく基準の 3 つを skill ask-one-by-one と同じ文で書き、person の later を必須と書く。
#      判定表の 6 と 8 と「レベル C に当たる変更は fix にしない」の文が Andon の skill とパッケージの直しを除き、その直しの表（a1〜a4）があり、
#      remember (b) と memory (c) がその表を指し、閉じ方の first が remember・memory・next にも使える。
#      Chief of Staff のトリアージと振り返りの手引きが印を読み、contract.md の予約キー later の書き手に apply がある。
#      無くなった文（Andon の直しを 6 の person にする文・first を gap だけに使う文・判断はどれも灯す文）の検査は、古い文を置いた写しで当たることも確かめる。
# === 41. トリアージと振り返りの置いておく印（ADR-0062） ===
tsk=package/skills/triage/SKILL.md
aobo=package/skills/ask-one-by-one/SKILL.md
# 置いておく基準の 3 つは、2 つの skill で同じ文（決めたこと 11）
crit() { sed -n '/^\*\*置いておくのは、次の3つを全部満たすときだけにし、迷ったら呼ぶ\*\*/,/^3\. /p' "$1" | grep -E '^[123]\. '; }
t_crit=$(crit "$tsk"); a_crit=$(crit "$aobo")
[ "$(grep -c . <<<"$t_crit")" = 3 ] && [ "$t_crit" = "$a_crit" ] \
  && grep -qF '**置いておくのは、次の3つを全部満たすときだけにし、迷ったら呼ぶ**（skill `ask-one-by-one` にも同じ文がある）' "$tsk" \
  && pass "ADR-0062 A18: skill triage は置いておく基準の 3 つと「迷ったら呼ぶ」を、skill ask-one-by-one と同じ文で書く" \
  || fail "ADR-0062 A18: skill triage の置いておく基準が ask-one-by-one と違うか無い"
# person の later（決めたこと 10）
grep -qF '判断も手作業も、`later` を必ず書く（§3 の置いておく基準）。置いておくなら理由の1行、呼ぶなら `false`（`true` は使わない）' "$tsk" \
  && grep -qF '`later` が無い・形が違う項目があれば、`apply` は kanban に手を入れずに断る' "$tsk" \
  && grep -qF '"later": false}' "$tsk" && grep -qF '"later": "今週中に済めばよく、待つ課題が無い"' "$tsk" \
  && pass "ADR-0062 A18: skill triage は person の later を必須と書き、decisions.json の例が later を持つ" \
  || fail "ADR-0062 A18: skill triage に person の later の決まりか例が無い"
# 判定表（決めたこと 13・14）
tlater_miss() {
  local m=''
  grep -q '^| 6 | \*\*灯りがつく決断\*\*.*Andon の skill とパッケージの直しは除く' "$1" || m="$m [表の 6 が Andon の直しを除かない]"
  grep -q '^| 8 | \*\*直せる\*\*.*Andon の skill とパッケージの直しは、レベル C でも下の表で `fix`' "$1" || m="$m [表の 8 が Andon の直しを入れない]"
  grep -q '^- \*\*Andon の skill とパッケージの直し\*\*（`remember`・`next`・`memory` の項目のうち' "$1" || m="$m [Andon の直しの決まりが無い]"
  grep -q '^  | a1 | オーナーの決定.*権限か秘密か信頼の境界を弱める | `person` の判断' "$1" || m="$m [a1 が無い]"
  grep -q '^  | a2 | 書いてあることと実物の食い違いを、自分で確かめた | `fix`（レベル C に当たる変更でも' "$1" || m="$m [a2 が無い]"
  grep -q '^  | a3 | 同じ直しの根拠が、同じ回に集めた別々の課題の項目に 2 件ある | `fix`' "$1" || m="$m [a3 が無い]"
  grep -q '^  | a4 | 根拠が 1 件 | `close`・`closing: first`' "$1" || m="$m [a4 が無い]"
  grep -qF '(b) Andon の skill・パッケージの直し → 下の「Andon の skill とパッケージの直し」の表に当てる（オーナーへの問いにしない）' "$1" || m="$m [remember (b) が表を指さない]"
  grep -qF 'Andon の skill・パッケージの直しは、上の「Andon の skill とパッケージの直し」の表に当てる（`remember` (b) と同じ）' "$1" || m="$m [memory (c) が表を指さない]"
  grep -qF '| 判断の閉じ | `first` | 1 件目 | `gap`・`remember`・`memory`・`next` だけ |' "$1" || m="$m [閉じ方の表の first]"
  grep -qF 'レベル C に当たる変更は `fix` にしない（表の 6 で `person`）。ただし Andon の skill とパッケージの直しは、レベル C でも' "$1" || m="$m [レベル C の文が Andon の直しを除かない]"
  echo "$m"
}
miss=$(tlater_miss "$tsk")
[ -z "$miss" ] && pass "ADR-0062 A18: skill triage の判定表が決めたこと 13・14 のとおりで、レベル C の文が Andon の skill とパッケージの直しを除く" \
  || fail "ADR-0062 A18: skill triage の判定表:$miss"
# 検査が当たる場合があること: 表の 6・a4・閉じ方の行を崩した写しでは落ちる
sed 's/。Andon の skill とパッケージの直しは除く（/（/' "$tsk" >"$tmp/tl1.md"
sed '/^  | a4 | /d' "$tsk" >"$tmp/tl2.md"
sed 's/| 1 件目 | `gap`・`remember`・`memory`・`next` だけ |/| 1 件目 | `gap` だけ |/' "$tsk" >"$tmp/tl3.md"
tl_caught=0
for f in "$tmp/tl1.md" "$tmp/tl2.md" "$tmp/tl3.md"; do
  if [ -n "$(tlater_miss "$f")" ]; then tl_caught=$((tl_caught + 1)); fi
done
[ "$tl_caught" = 3 ] && pass "ADR-0062 A18: 判定表の検査は、表の 6・a4・閉じ方の行を崩した写しで落ちる" \
  || fail "ADR-0062 A18: 判定表の検査が、崩した写し 3 つのうち $tl_caught つでしか落ちない"
# 手引きと約束
tpb=package/skills/chief-of-staff/playbooks/triage.md
rpb=package/skills/chief-of-staff/playbooks/retro.md
grep -qF '`person` の `later`（置いておく理由か、呼ぶ `false`）が置いておく基準' "$tpb" \
  && grep -qF '`later` が `false` なら呼んでいる行になって行燈を灯し、`later` に理由があれば印 `later` を持つ置いてある行になって灯らない' "$tpb" \
  && grep -qF '「1 件目」（`first`）は、根拠が 1 件の、Andon の skill とパッケージの直し（申告〈`gap`〉と、`remember`・`next`・`memory` の項目）' "$tpb" \
  && grep -qF '`apply` が決まった理由の印 `later` を書くので、置いてある行になって行燈を灯さない' "$rpb" \
  && pass "ADR-0062 A18: Chief of Staff のトリアージと振り返りの手引きが印 later と、広げた 1 件目を読む" \
  || fail "ADR-0062 A18: Chief of Staff の triage.md か retro.md が印 later を読まない"
grep -E '^\| `later` \|' docs/contract.md | grep -qF '`andon triage apply`（`later` が理由の `person`）・`andon retro apply`（「Andon へ伝えるか」の課題。決まった理由）' \
  && grep -qF 'どちらの形も、呼ぶか置いておくかを言い切るキー `later` を必ず持つ' docs/contract.md \
  && pass "ADR-0062 A18: contract.md の予約キー later の書き手に 2 つの apply があり、トリアージの節が person の later を書く" \
  || fail "ADR-0062 A18: contract.md に apply の印の書き手か、person の later が無い"
# 無くなった文
old_lines='ラインの変更なので 6 の `person`（`kind: line`）|`first` は種類 `gap` の項目にだけ使える|判断はどれも、`apply` が問いにしてオーナーの課題に置き、行燈を灯す|ask-one-by-one で1問ずつ'
left=$(grep -nE "$old_lines" "$tsk" "$tpb" "$rpb" || true)
[ -z "$left" ] && pass "ADR-0062 A18: skill triage と手引きに、Andon の直しを問いにする古い文・first を gap だけに使う古い文・判断を全部灯す古い文が無い" \
  || fail "ADR-0062 A18: 古い文が残る: $left"
# 検査が当たる場合があること: 古い文を置いた写しで、同じ grep が 4 行とも拾う
printf '%s\n' '(b) Andon の skill・パッケージの直し → ラインの変更なので 6 の `person`（`kind: line`）。' \
  '  - `first` は種類 `gap` の項目にだけ使える判断の閉じで' '本当に種類を決めきれないときだけ使う。判断はどれも、`apply` が問いにしてオーナーの課題に置き、行燈を灯す。' \
  '   オーナーの課題（タイトルが「Andon へ伝えるか: …」）は ask-one-by-one で1問ずつ。' >"$tmp/tl-old.md"
[ "$(grep -cE "$old_lines" "$tmp/tl-old.md")" = 4 ] \
  && pass "ADR-0062 A18: 古い文の検査は、古い文の写しで 4 行とも当たる" || fail "ADR-0062 A18: 古い文の検査が古い文の写しに当たらない"
