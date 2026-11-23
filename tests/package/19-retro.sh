# tests/package/19-retro.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  19. 振り返りの手順（ADR-0035 受け入れ条件 12）: retro skill に AGENTS.md の表を読む手順と、検査を足す直しの「誤りを戻すと落ちる」の受け入れ条件を入れる手順。
#      ADR-0058 決めたこと 11: 手直しの記録の PR のコメントのうちエージェントの文面を、決まりが破られた跡として読む行。
# === 19. 振り返りの手順（ADR-0035 受け入れ条件 12・決めたこと 6・8） ===
r=package/skills/retro/SKILL.md
miss=
t=$(sed -n '/^### `AGENTS.md` の表を読む$/,/^## 3\. /p' "$r")
[ -n "$t" ] || miss="$miss [§2 に AGENTS.md の表を読む節]"
for k in 'show origin/HEAD:AGENTS.md' '「決まり｜守らせるもの」の対' '表の行は根拠に数えない' '守らせるものが「無し。」の行' '文を足さず、検査か設計へ移す' '守らせるものが名指されている行' '誤りが起きようのなくなった行'; do
  grep -qF -- "$k" <<<"$t" || miss="$miss [表の節に $k]"
done
s3=$(sed -n '/^## 3\. /,/^## 4\. /p' "$r")
for k in '**検査を足すか変える直し**' '受け入れ条件: 根拠の誤りを戻した木で、足した検査が落ちる' 'andon issue record <id> --expect fail --' '「期待する効果:」の行になる'; do
  grep -qF -- "$k" <<<"$s3" || miss="$miss [§3 に $k]"
done
# §6 の例の test の段のパターンも、effect に受け入れ条件を持つ
block "$r" 'andon retro patterns "$id"' | sed '1d;$d' >"$tmp/retro-in.json"
jq -e '[.patterns[] | select(.level == "test" or .level == "check") | .effect | contains("受け入れ条件: 根拠の誤りを戻した木で、足した検査が落ちる")] | length > 0 and all' "$tmp/retro-in.json" >/dev/null 2>&1 \
  || miss="$miss [§6 の例の test の段の effect に受け入れ条件]"
grep -qF '「誤りを戻すと落ちる」の受け入れ条件が載っているか' "$r" || miss="$miss [§6 の dry-run で受け入れ条件を見る]"
grep -qF '`AGENTS.md` の表: <読んだ repo>' "$r" || miss="$miss [§7 の報告に表の行]"
grep -q '^description: .*`AGENTS.md` の表の読み方.*「誤りを戻すと落ちる」の受け入れ条件' "$r" || miss="$miss [description]"
[ -z "$miss" ] && pass "ADR-0035 A12: retro skill に AGENTS.md の表を読む手順と、検査を足す直しの依頼書に「誤りを戻すと落ちる」の受け入れ条件を入れる手順がある" || fail "ADR-0035 A12:$miss"
# ADR-0058 決めたこと 11・受け入れ条件 3: 手直しの記録の pr のコメントがエージェントの文面なら、オーナーの手直しと読まず、決まりが破られた跡として拾う
grep -qF '| `corrections` の `pr` の `comments` のうち、エージェントの文面のもの' "$r" && grep -qF 'エージェントは PR の会話欄に書かない決まりなので、オーナーの手直しとして読まない。決まりが破られた跡として拾う' "$r" \
  && pass "ADR-0058 決めたこと 11: retro skill が、手直しの記録の PR のコメントのうちエージェントの文面を、決まりが破られた跡として読む" || fail "ADR-0058 決めたこと 11: retro skill に PR のコメントのうちエージェントの文面の読み方が無い"
