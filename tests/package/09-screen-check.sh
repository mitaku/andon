# tests/package/09-screen-check.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   9. 画面の確かめ（ADR-0018 受け入れ条件 4）: managed だけの許可・worker skill §2・§3・§7・independent-review の観点・environment.md。
#      PR の会話欄に書かない（ADR-0058 受け入れ条件 3）: §3 の gh pr edit --attach・package/ に gh pr comment／review を打たせる行が無い・環境メモの行。
# === 9. 画面の確かめ（ADR-0018 受け入れ条件 4） ===
jq -e '.allowManagedPermissionRulesOnly == true' package/settings.json >/dev/null \
  && pass "settings は許可の規則を managed だけにする（allowManagedPermissionRulesOnly。skill の allowed-tools を効かせない）" || fail "settings に allowManagedPermissionRulesOnly: true が無い"
w=package/skills/worker/SKILL.md
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w"); w3=$(sed -n '/^## 3\. /,/^## 4\. /p' "$w"); w7=$(sed -n '/^## 7\. /,$p' "$w")
grep -q '^| 確認 |.*画面' <<<"$w2" && grep -q '画面に触れる変更は、確認の工程で画面を見て確かめる' <<<"$w2" \
  && grep -q '`127\.0\.0\.1`' <<<"$w2" && grep -q 'playwright-cli open http://127\.0\.0\.1:' <<<"$w2" && grep -q '→ `screenshot`' <<<"$w2" \
  && pass "worker skill §2 の確認の工程に、127.0.0.1 の dev サーバを playwright-cli で確かめスクリーンショットを撮る段がある" || fail "worker skill §2 に画面の確かめの段が無い"
grep -q 'gh pr create --attach' <<<"$w3" && grep -q 'gh pr edit <n> --body-file <本文> --attach' <<<"$w3" \
  && grep -qF '`![説明](./<画像の名前>)`' <<<"$w3" \
  && pass "worker skill §3 に gh pr create --attach と、出したあとの gh pr edit --attach（本文に足す。ADR-0058 決めたこと 6）がある" || fail "worker skill §3 に gh pr create --attach か gh pr edit --attach が無い"
# ADR-0058 受け入れ条件 3: package/ の下に、PR の会話欄に書く command（gh pr comment・gh pr review に引数が続く形）を打たせる行が無い。
# 名前を挙げるだけの行（`gh pr comment` と …）は当たらない
talk='gh[[:space:]]+pr[[:space:]]+(comment|review)([[:space:]]|$)'
hits=$(grep -rnE "$talk" package/ || true)
[ -z "$hits" ] && pass "package/ に gh pr comment・gh pr review を打たせる行が無い（ADR-0058）" || fail "package/ に PR の会話欄に書く command の行がある: $hits"
# 検査が見ていることの確かめ: 直す前の行と、引数の形を変えた行には当たり、名前を挙げるだけの行には当たらない
miss=
for l in '（PR を出したあとなら `gh pr comment <n> --attach <画像>`。1枚 10 MB まで）' 'gh pr comment --body x' 'gh  pr review 12 --comment -b x' 'env gh pr comment 3'; do
  grep -qE "$talk" <<<"$l" || miss="$miss [$l]"
done
grep -qE "$talk" <<<'`gh pr comment` と `gh pr review` は設定で断られる' && miss="$miss [名前だけの行に当たる]"
[ -z "$miss" ] && pass "会話欄に書く command の検査は、直す前の行と変種に当たり、名前だけの行に当たらない" || fail "会話欄に書く command の検査が見ていない:$miss"
grep -qF '**PR の会話欄（会話のコメント・レビュー）に書かない。**' <<<"$w3" && ! grep -qF 'PR 本文・報告・コメントに書いた' <<<"$w3" \
  && grep -qF 'PR のコメントには書かない（§3）' "$w" \
  && pass "worker skill §3 に PR の会話欄に書かない決まりがあり、読み返す行と §4 の見送りもコメントを前提にしない（ADR-0058 決めたこと 5）" || fail "worker skill に PR の会話欄に書かない決まりが無いか、コメントを前提にした行が残る"
grep -qF 'PR の会話欄（会話のコメント・レビュー）には、どの役も書かない。' package/environment.md && grep -qF 'オーナーの手直しとして集められる' package/environment.md \
  && grep -qF 'オーナーにコメントを頼まれたら、文案を返してオーナーに書いてもらう' package/environment.md \
  && pass "environment.md に PR の会話欄に書かない決まりの行がある（ADR-0058 決めたこと 5）" || fail "environment.md に PR の会話欄に書かない決まりの行が無い"
grep -q '`evidence` には、PR の添付の URL.*撮った command' <<<"$w7" \
  && pass "worker skill §7 の acceptance[].evidence に添付の URL と command" || fail "worker skill §7 に添付の URL と command の段が無い"
grep -q 'screen_paths.*画面の証拠.*major' package/skills/independent-review/SKILL.md \
  && grep -q 'screen_paths.*添付が無い.*（major）' internal/dispatcher/loop.go \
  && pass "independent-review の依頼書の型（skill と dispatcher の reviewerBrief）に画面の証拠の観点（major）がある" || fail "independent-review の型に画面の証拠の観点が無い"
grep -q 'オーナーが見るブラウザは無い' package/environment.md && ! grep -qE '(^|[^見る])ブラウザは無い' package/environment.md \
  && grep -q '担当が画面を見て確かめるには `playwright-cli`' package/environment.md \
  && pass "environment.md は「オーナーが見るブラウザは無い」と playwright-cli を案内する" || fail "environment.md の「ブラウザは無い」が直っていない"
# ADR-0020 受け入れ条件 6（決めたこと 6・A-0144 の c2・c3）: 環境メモに memory への書き方
envmem=$(sed -n '/^## 気づきを残す（auto memory）$/,/^## /p' package/environment.md)
miss=""
for k in '`/workspace/memory/`' '蒸留までの受け皿' '週1のトリアージ' '1件1ファイルの `.md` を、フォルダの直下に書く' '確かめた事実と、起きた条件と理由のついた学び' '本文に課題番号（無ければ repo）を書く' '続き・決定・秘密は書かない' '報告の `## Remember` を書く' 'Codex の記憶は切ってあり'; do
  grep -qF -- "$k" <<<"$envmem" || miss="$miss $k"
done
[ -z "$miss" ] && pass "environment.md に memory への書き方（受け皿・直下の .md・条件と理由・課題番号・続き/決定/秘密は書かない・Remember は残す。ADR-0020）" || fail "environment.md の memory の節:$miss"
