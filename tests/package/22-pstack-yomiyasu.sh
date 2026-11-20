# tests/package/22-pstack-yomiyasu.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#  22. pstack と yomiyasu の既定（ADR-0038 受け入れ条件 1・3 の一部）: settings の marketplace の登録と自動更新と pstack の有効化、gh-skills.txt の yomiyasu、「既定は空」の文、writing.md の推敲の節、環境メモの pstack の行。
# === 22. pstack と yomiyasu をパッケージの既定に入れる（ADR-0038 受け入れ条件 1・3 のうち writing.md と環境メモ。決めたこと 11・16〜19） ===
# 参照: managed settings に marketplace の登録（自動更新つき。版を固定する sha・ref は書かない）と pstack の有効化、外の skill の一覧に版なしの yomiyasu
refs_ok() {
  jq -e '.extraKnownMarketplaces["cursor-plugins"] as $m
    | $m.source == {"source": "github", "repo": "cursor/plugins"} and $m.autoUpdate == true
    and .enabledPlugins["pstack@cursor-plugins"] == true' "$1" >/dev/null || return 1
  grep -qx 'nanaism/yomiyasu yomiyasu' "$2" || return 1
  ! grep -vE '^[[:space:]]*(#|$)' "$2" | grep -q '@'
}
refs_ok package/settings.json package/gh-skills.txt \
  && pass "ADR-0038 A1: settings に marketplace の登録（autoUpdate つき・版の固定なし）と pstack の有効化、gh-skills.txt に版なしの yomiyasu がある" \
  || fail "ADR-0038 A1: settings の pstack の参照か gh-skills.txt の yomiyasu の行が無い（か版を固定している）"
# 検査が当たる場合があること: 参照を1つずつ消した・版を固定した写しでは落ちる
miss=
cp package/gh-skills.txt "$tmp/gh-skills.txt"
for q in 'del(.enabledPlugins)' 'del(.extraKnownMarketplaces["cursor-plugins"])' 'del(.extraKnownMarketplaces["cursor-plugins"].autoUpdate)' \
  '.extraKnownMarketplaces["cursor-plugins"].source.sha = "0000000000000000000000000000000000000000"'; do
  jq "$q" package/settings.json >"$tmp/settings.json"
  refs_ok "$tmp/settings.json" "$tmp/gh-skills.txt" && miss="$miss [$q]"
done
for s in '/^nanaism\/yomiyasu yomiyasu$/d' 's/^nanaism\/yomiyasu yomiyasu$/nanaism\/yomiyasu yomiyasu@v3.0.1/'; do
  sed "$s" package/gh-skills.txt >"$tmp/gh-skills.txt"
  refs_ok package/settings.json "$tmp/gh-skills.txt" && miss="$miss [gh-skills.txt: $s]"
done
[ -z "$miss" ] && pass "ADR-0038 A1: 参照を消した・版を固定した写しでは落ちる（検査が見ている）" || fail "ADR-0038 A1: 写しで落ちない:$miss"
# 「既定は空」の文が残っていない（決めたこと 18）。古い字面に当たることも確かめる
old_line='（既定は空）'
grep -qF "$old_line" <<<'`/opt/andon/gh-skills.txt`（既定は空）＋ セルの' || fail "ADR-0038: 「既定は空」の検査が何も見ていない"
! grep -qF '既定は空' package/gh-skills.txt && ! grep -qF "gh-skills.txt$old_line" internal/boot/extskills.go docs/design.md \
  && grep -qF 'plugins（pstack）の登録は `package/settings.json` の `extraKnownMarketplaces`' docs/design.md \
  && pass "ADR-0038 決めたこと 18: gh-skills.txt・extskills.go・design.md に「既定は空」が無く、design.md は plugin の登録の正を package/settings.json と書く" \
  || fail "ADR-0038 決めたこと 18: 「既定は空」が残るか、design.md の plugin の登録の正が古い"
# writing.md の推敲の行（決めたこと 16）と、環境メモの pstack の行（決めたこと 17）
pol=$(sed -n '/^## 推敲$/,/^## /p' package/writing.md)
grep -qF '依頼書・wiki のページ・課題の報告の3種類の文は、書いたあとに skill yomiyasu で推敲する。' <<<"$pol" \
  && grep -qF '会話の返事・`docs/` の文書・PR の説明文には通さない。' <<<"$pol" && grep -qF 'yomiyasu が入っていないセルでは飛ばす。' <<<"$pol" \
  && pass "ADR-0038 A3: writing.md の推敲の節に、通す3種類・通さない文・入っていないセルでは飛ばす、がある" || fail "ADR-0038 A3: writing.md の推敲の行が無い"
miss=
for k in 'pstack の skill は、役の skill か依頼書が名指したものだけを、`SKILL.md` を読んで使う。' '`setup-pstack` は走らせない。' '`pstack:` で始まる型のサブエージェントは使わない。'; do
  grep -qF -- "$k" package/environment.md || miss="$miss $k"
done
[ -z "$miss" ] && pass "ADR-0038 A3: 環境メモに pstack の行（名指したものだけ・setup-pstack は走らせない・pstack: の型は使わない）がある" || fail "ADR-0038 A3: 環境メモの pstack の行:$miss"
# 担当と Chief of Staff に pstack の skill を使う指示を足さない（やらないこと）
left=$(grep -ln 'pstack' package/skills/worker/SKILL.md package/skills/chief-of-staff/SKILL.md package/skills/chief-of-staff/playbooks/*.md || true)
[ -z "$left" ] && pass "ADR-0038: worker と CoS の skill に pstack の指示が無い" || fail "ADR-0038: pstack の指示がある: $left"
