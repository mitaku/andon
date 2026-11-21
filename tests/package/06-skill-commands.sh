# tests/package/06-skill-commands.sh — tests/package.sh が名前の順に source する節。単独では走らない（pass・fail・fm・tmp は入口が置き、前の節が置いた変数も使う）。
#   6. skill の機械向けの手順が command（andon issue …）を指し、手で JSON を書く手順が残っていない。例の問いと予約キーは schema に通す。
#  12. auto memory（ADR-0020 受け入れ条件 2・6。4 と 6 の節の中）: settings が入れる向き・環境メモの書き方・chief-of-staff の規約9・トリアージ・wiki の手引き。
#  15. Advisor（ADR-0022 受け入れ条件 1・2。6 の節の中）: --advisor fable は chief-of-staff の command だけ、CoS skill の「オーナーの入口」に相談する場面の1行。
# === 6. skill の機械向けの手順（command を指す。skill の中の ``` の塊の例は schema に通す） ===
# block <file> <1行目の頭> … 1行目が「<頭>」で始まる ``` の塊（字下げを除く）
block() { awk -v h="$2" '
  /^ *```/ { if (inb) { exit } ; fence = 1; next }
  fence && !inb { fence = 0; t = $0; sub(/^ +/, "", t); if (index(t, h) == 1) { inb = 1; ind = match($0, /[^ ]/) - 1 } else next }
  inb { print substr($0, ind + 1) }' "$1"; }
export ANDON_HOME=$PWD/package
chk() { "$andon" schema check "$1" "$2" >"$tmp/why" 2>&1; }

# 担当の機械向けの手順は command（andon issue …）。skill に JSON を手で書く手順・一致を手で確かめる手順が残っていない
w=package/skills/worker/SKILL.md
for c in 'andon issue report' 'andon issue review-request' 'andon issue ready' 'andon issue ask' 'andon issue show'; do
  grep -q "$c" "$w" && pass "worker skill は $c を使う" || fail "worker skill に $c が無い"
done
left=$(grep -nE 'schema_check|report\.json"? *$|> "\$lib/report\.json"|questions/\$\{n|一致を確かめる|headRefOid' "$w" || true)
[ -z "$left" ] && pass "worker skill に report.json・問いの JSON を手で書く手順と、一致を手で確かめる手順が無い" || fail "手の手順が残る: $left"
# skill の問いの例（andon issue ask の stdin）は question.schema.json に合う
q_blk=$(block "$w" 'andon issue ask "$id" -' | sed '1d;$d')
printf '%s\n' "$q_blk" >"$tmp/q.json"
[ -n "$q_blk" ] && chk question "$tmp/q.json" && pass "worker skill の問いの例は question.schema.json に合う" || fail "問いの例: $(head -1 "$tmp/why" 2>/dev/null) $q_blk"

# 工程の背骨（ADR-0006 決めたこと 9・11・12・13）: worker の工程表に6つの工程、報告の書式に ## 計画・探した範囲・計画との差。Chief of Staff の雛形に 横展開: と調べる課題の 終了条件:
w2=$(sed -n '/^## 2\. /,/^## 3\. /p' "$w")
miss=
for s in 理解 計画 作業 確認 完成 振り返り; do grep -q "^| $s |" <<<"$w2" || miss="$miss $s"; done
[ -z "$miss" ] && pass "worker skill §2 の工程表に 理解・計画・作業・確認・完成・振り返り がある" || fail "worker skill §2 の工程表に無い工程:$miss"
w7=$(block "$w" 'PR: <https://')
grep -q '^## 計画$' <<<"$w7" && grep -q '^探した範囲: ' <<<"$w7" && grep -q '^計画との差: ' <<<"$w7" \
  && pass "worker skill の報告の書式に ## 計画・探した範囲・計画との差がある" || fail "worker skill の報告の書式に ## 計画・探した範囲・計画との差が無い"
# 調べる課題の候補の例（andon issue candidates の stdin）は candidates.schema.json に合う。scope の問いは worker skill が案内する
c_blk=$(block "$w" 'andon issue candidates "$id" -' | sed '1d;$d')
printf '%s\n' "$c_blk" >"$tmp/c.json"
[ -n "$c_blk" ] && chk candidates "$tmp/c.json" && pass "worker skill の候補の例は candidates.schema.json に合う" || fail "候補の例: $(head -1 "$tmp/why" 2>/dev/null) $c_blk"
grep -q 'kind: scope' "$w" && grep -q 'andon issue candidates' "$w" && pass "worker skill は scope の問いと andon issue candidates を案内する" || fail "worker skill に scope の問いか andon issue candidates が無い"

# Chief of Staff の skill は原則（SKILL.md）と手引き（playbooks/*.md）に分かれる。検査は中身の在る手引きを見る
cos=package/skills/chief-of-staff/SKILL.md pb=package/skills/chief-of-staff/playbooks
cosall=$(cat "$cos" "$pb"/*.md)
block "$pb/brief.md" '# <id> <題>' | grep -q '^横展開: ' && block "$pb/brief.md" '# <id> 調べる: ' | grep -q '^終了条件: ' \
  && pass "Chief of Staff の依頼書の雛形に 横展開: の行、調べる課題の雛形に 終了条件: の行がある" || fail "chief-of-staff の雛形に 横展開: か 終了条件: が無い"

block "$pb/brief.md" '# <id> 調べる: ' | grep -q '^- Deliverable: candidates' \
  && pass "調べる課題の雛形の Settings に Deliverable: candidates がある" || fail "調べる課題の雛形に Deliverable: candidates が無い"

# ADR-0009 受け入れ条件 3: オーナーの入口・打つコマンド・依頼書の原則と範囲の外・差し戻し・wiki の節
grep -q '^1\. \*\*自分で作らない\*\*。.*自分で Chief Engineer を起動して経緯を渡す' "$cos" \
  && [ "$(grep -m1 '^## ' "$pb/cut.md")" = '## オーナーの入口' ] && block "$pb/cut.md" 'andon chief-engineer --brief - "<題>"' | grep -q '^経緯: ' \
  && ! grep -qE 'andon chief-engineer[^。]*を勧め|題としてオーナーに勧め|オーナーに題を勧め' <<<"$cosall" \
  && pass "Chief of Staff の原則1が自分で Chief Engineer を起動すると言い、切る手引きの先頭は「オーナーの入口」で andon chief-engineer --brief - を自分で打つ（オーナーに勧めない）" || fail "chief-of-staff の原則1か cut.md の先頭に「オーナーの入口」が無いか、andon chief-engineer をオーナーに勧める文が残る"
grep -q '^- 打つコマンドは .*`andon chief-engineer`' "$pb/wake.md" \
  && pass "Chief of Staff の打つコマンドの一覧に andon chief-engineer がある" || fail "chief-of-staff の打つコマンドの一覧に andon chief-engineer が無い"
sed -n '/^\*\*欄の読み方\*\*/,/^| 欄 |/p' "$pb/brief.md" | grep -q 'repo を読めば分かること（作法・構造・直し方）は書かず' \
  && grep -q '^| 守ること | 範囲の外（今回やらないこと）' "$pb/brief.md" \
  && pass "依頼書の欄の読み方の頭に原則の1行、守ることの説明に 範囲の外" || fail "brief.md に依頼書の原則か 範囲の外 が無い"
grep -q '^8\. \*\*2回通らなければ、手直しを重ねない\*\*' "$cos" \
  && grep -q '## 差し戻し（<日付>）' "$pb/gate.md" && grep -q '`## 差し戻し` が2つあってまだ通らない' "$pb/gate.md" && grep -q '依頼書を直した新しい課題を `kanban add`' "$pb/gate.md" \
  && pass "差し戻しは依頼書への追記で数え、2回で通らなければ新しい課題に切る" || fail "chief-of-staff に差し戻しの追記か切り直しの手順が無い"
cos11=$(cat "$pb/wiki.md")
# ADR-0016 受け入れ条件 1: 層・3つの操作・元資料の3種・移行があり、前の形（1ファイル1件・log.md は作らない）が無い
miss=""; for k in '何を' '層' '元資料を置くとき' 'いつ（ingest）' 'query' 'lint' '移行'; do grep -q "^- \*\*$k\*\*" <<<"$cos11" || miss="$miss $k"; done
for k in '`raw/`' '`sources/`' '`entities/`' '`topics/`' '`overview.md`' '`index.md`' '`log.md`'; do grep -q "^  - $k — " <<<"$cos11" || miss="$miss 層$k"; done
for k in 'オーナーの発言' '外の資料' '課題の記録' 'memory'; do grep -q "^  - $k: " <<<"$cos11" || miss="$miss 元資料:$k"; done
grep -qF '## [YYYY-MM-DD] ingest | <題>' <<<"$cos11" && grep -q 'ingest 約10件ごと' <<<"$cos11" && grep -q '`index.md` → ページの順' <<<"$cos11" || miss="$miss log の形・lint の時期・query の順"
grep -q 'commit --only -m "wiki: <要点>" -- wiki' <<<"$cos11" || miss="$miss commit（--only -- wiki）"
grep -q '書く前に秘密.*`<伏せた: 種類>`' <<<"$cos11" || miss="$miss raw/ に書く前に秘密を伏せる"
grep -qE '1ファイル1件|`log.md` は作らない' <<<"$cos11" && miss="$miss 前の形の行が残る"
[ -z "$miss" ] && pass "Chief of Staff の skill の wiki の節が LLM Wiki の形（層・ingest・query・lint・元資料・移行。ADR-0016）" || fail "chief-of-staff の wiki の節:$miss"
# ADR-0020 受け入れ条件 6: 規約9は memory を受け皿とし、トリアージが wiki 行きの memory を raw/memory/ に写し、wiki の raw/ は4種
grep -q '^| 9\. 記憶の規律 | 11 | `wiki.md` |$' "$cos" && grep -q '^11\. \*\*学びは層で分けて残す\*\*。.*memory は蒸留までの受け皿' "$cos" \
  && grep -q '^記憶の規律（規約9）: memory（auto memory。`/workspace/memory/`）は蒸留までの受け皿。残すものはトリアージを通して wiki と repo へ' "$pb/wiki.md" \
  && ! grep -q 'ハーネス内蔵のメモリは使わない' <<<"$cosall" \
  && grep -q '`close`「wiki へ: <題>」のものは、記録.*の本文を言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写して ingest' "$pb/triage.md" \
  && grep -q '^  - `raw/` — 元資料。書いたら変えない。4種: .*(4) トリアージで wiki 行きになった memory の写し（`raw/memory/' <<<"$cos11" \
  && pass "Chief of Staff の skill: 原則11・規約9・トリアージ・wiki の手引きが memory を蒸留までの受け皿として扱う（ADR-0020）" || fail "chief-of-staff の原則11・規約9・triage.md・wiki.md が ADR-0020 の形でない"
# ADR-0016 受け入れ条件 3: 読む側（Chief Engineer の「読む」と Chief of Staff の切る手引き）が index.md → ページの順で引く
grep -q '^1\. \*\*読む.*セルの wiki（`index.md` → ページの順で引く。使う事実は' package/skills/chief-engineer/SKILL.md \
  && grep -q 'wiki を引く（`wiki.md` の query。`index.md` → ページの順）' "$pb/cut.md" \
  && pass "Chief Engineer の「読む」と Chief of Staff の cut.md が wiki を index.md → ページの順で引く" || fail "読む側が index.md → ページの順を指していない"
# ADR-0009 受け入れ条件 4: Chief of Staff の template が wiki を道具の届く場所に足す
case " $(fm package/templates/chief-of-staff.md command) " in *" --add-dir /workspace/wiki "*) pass "Chief of Staff の template の command に --add-dir /workspace/wiki" ;; *) fail "Chief of Staff の template に --add-dir /workspace/wiki が無い" ;; esac
# ADR-0022 受け入れ条件 1・2: Advisor（Fable）は Chief of Staff だけ。skill の「オーナーの入口」に相談する場面の1行
case " $(fm package/templates/chief-of-staff.md command) " in *" --advisor fable "*) pass "Chief of Staff の template の command に --advisor fable" ;; *) fail "Chief of Staff の template に --advisor fable が無い" ;; esac
left=
for p in $profiles; do
  [ "$p" = chief-of-staff ] && continue
  for field in command resume; do
    case " $(fm "package/templates/$p.md" "$field") " in *--advisor*) left="$left $p.$field" ;; esac
  done
done
[ -z "$left" ] && pass "chief-of-staff のほかのプロファイルの command・resume に --advisor が無い" || fail "--advisor が付いている:$left"
sed -n '/^## オーナーの入口$/,/^## 切り方$/p' "$pb/cut.md" | grep 'advisor に相談してから決める' | grep -q '振り分けに迷うとき' \
  && sed -n '/^## オーナーの入口$/,/^## 切り方$/p' "$pb/cut.md" | grep 'advisor に相談してから決める' | grep -q '2回差し戻して切り直すとき' \
  && pass "Chief of Staff の skill の「オーナーの入口」に advisor に相談する場面（振り分けに迷うとき・2回差し戻して切り直すとき）" || fail "Chief of Staff の skill の「オーナーの入口」に advisor の1行が無い"

# Chief of Staff の issue.yaml の予約キー: 全部書き、issue-keys.schema.json に合う
set_line=$(block "$pb/brief.md" '# issue.yaml の予約キー' | grep '^kanban set ')
printf '%s\n' "$set_line" | tr ' ' '\n' | grep '=' | jq -R 'split("=") | {(.[0]): .[1]}' | jq -s add >"$tmp/keys.json"
if chk issue-keys "$tmp/keys.json" && jq -e 'keys == ["estimate","merge","must_fix","review","review_rounds","reviewer"]' "$tmp/keys.json" >/dev/null; then
  pass "Chief of Staff は予約キー6つ（review・must_fix・reviewer・review_rounds・merge・estimate）を issue.yaml に書く"
else fail "Chief of Staff の kanban set: $set_line"; fi
# ADR-0014 受け入れ条件 7（E1）: 雛形の Settings に Estimate の行（段の名）、起き直しで打ち直す予約キーに estimate
block "$pb/brief.md" '# <id> <題>' | grep -qx -- '- Estimate: 中' \
  && block "$pb/brief.md" '# <id> 調べる: ' | grep -qx -- '- Estimate: 小' \
  && grep -q '同じ値で `kanban set <id> review=… .*`estimate`' "$pb/wake.md" \
  && pass "Chief of Staff の雛形に Estimate の行があり、起き直しで estimate も打ち直す" || fail "chief-of-staff に Estimate の行か起き直しの estimate が無い"
# ADR-0014 受け入れ条件 5・7（E4）: worker の報告の雛形の ## 計画 に見込みの反論の1行、--data に estimate（引き継ぐ）
grep -q '^- 見込み: <段>（依頼書は <段>）。<理由>' package/skills/worker/SKILL.md \
  && grep -q '`estimate`（§2 の見込みの反論）も `--data` に無ければ前の `report.json` から引き継ぐ' package/skills/worker/SKILL.md \
  && pass "worker の報告の ## 計画 に見込みの反論の1行があり、report.json の estimate は引き継ぐ" || fail "worker に見込みの反論の1行か estimate の引き継ぎが無い"
# ADR-0017 受け入れ条件 8（P4）: worker は ## 計画 を report.json の plan[] に写し、plan は引き継ぐ
grep -q '`plan`（`## 計画` の写し）も `--data` に無ければ前の `report.json` から引き継ぐ' package/skills/worker/SKILL.md \
  && grep -q '^- `plan`: `## 計画` の作業を1件ずつ `{"id": "p1", "text": ".*", "done": false}` で写す' package/skills/worker/SKILL.md \
  && pass "worker は ## 計画 を report.json の plan[] に写し、plan は引き継ぐ" || fail "worker に plan[] の写しか引き継ぎが無い"
grep -q '`auto` は\*\*委譲がこのプロジェクトのマージを任せているときだけ\*\*' "$pb/brief.md" \
  && pass "merge: auto は委譲の範囲内だけ" || fail "chief-of-staff に merge: auto の条件が無い"
# インナーループは dispatcher（P6a）: Chief of Staff の skill のレビューと gate の節はオーナーの判断が要るときだけ。レビュアーを手で起動する手順が残っていない
cos6=$(cat "$pb/gate.md")
grep -q '^3\. \*\*作った者に合否を決めさせない\*\*。.*指摘を中継しない・gate を手で回さない・マージしない' "$cos" \
  && grep -q '^# レビューと gate（オーナーの判断が要るときだけ）' <<<"$cos6" && grep -q '^## `needs_human` のとき' <<<"$cos6" \
  && ! grep -qE 'andon issue start "\$new"|worktree add --detach|andon gate <id>' <<<"$cosall" \
  && ! grep -qE 'andon issue start "\$new"|worktree add --detach|andon gate <id>' <<<"$cos6" \
  && ! grep -qE 'worktree add --detach|andon issue start "\$new"' package/skills/independent-review/SKILL.md \
  && pass "Chief of Staff の原則3と gate の手引きは needs_human のときだけ。レビュアーを起動するのと gate は dispatcher" || fail "chief-of-staff・independent-review にレビュアーを手で起動する手順・gate を手で回す手順が残る"
grep -q '元の課題（`issue.yaml` の `parent`）の担当だけ' package/skills/worker/SKILL.md && grep -q '結果の宛先は元の課題の担当だけ' package/skills/independent-review/SKILL.md \
  && pass "レビュアーの結果の宛先は parent の課題の担当だけ（Chief of Staff へ送らない）" || fail "レビュアーの宛先が parent の課題の担当に固定されていない"

# Chief Engineer: worktree で文書の PR と report.json を同じ課題から返す。問いは問いの JSON
ce=package/skills/chief-engineer/SKILL.md
grep -q 'cwd は案件の repo の自分の worktree' "$ce" && grep -q '`pr`＝文書の PR' "$ce" && ! grep -q 'issues/<id>/library/<題' "$ce" \
  && pass "Chief Engineer は worktree で文書の PR と report.json を返す" || fail "chief-engineer の渡し方"
grep -q 'worker skill §5 の1の手順で1問ずつ問いの JSON' "$ce" && grep -q 'andon issue ask "$id" -' package/skills/ask-one-by-one/SKILL.md \
  && pass "Chief Engineer と Chief of Staff の問いは問いの JSON の手順を使う" || fail "問いの JSON の手順を指していない"
# ADR-0009 受け入れ条件 5: WIP の行が無い・読むに価値の仮説の結果・文書の形に価値の仮説の見出し・言い直すで決断が無ければ next で返す
! grep -qE '1プロジェクトに1題|前の題が終わらないまま' "$ce" package/skills/glossary/SKILL.md \
  && pass "Chief Engineer の skill と用語集に WIP の行（1プロジェクトに1題・前の題が終わらないまま）が無い" || fail "chief-engineer か用語集に WIP の行が残る"
ce1=$(sed -n '/^1\. \*\*読む/,/^2\. /p' "$ce"); ce2=$(sed -n '/^2\. \*\*入口: 言い直しと前提/,/^3\. /p' "$ce")
grep -q '前の価値の仮説の結果' <<<"$ce1" && block "$ce" '# <題>（仕様・設計' | grep -q '^## 価値の仮説' \
  && pass "Chief Engineer の「読む」に前の価値の仮説の結果、文書の形に ## 価値の仮説 の見出し" || fail "chief-engineer に価値の仮説の見出しが無い"
grep -q '決断が無ければ文書を書かずに返す' <<<"$ce2" && grep -q '"to": "chief-of-staff", "text": "直す課題に切る: ' <<<"$ce2" && grep -q 'kanban review <id>' <<<"$ce2" \
  && pass "Chief Engineer は入口の「言い直しと前提」で決断が無ければ文書を書かず next で返す" || fail "chief-engineer の入口の「言い直しと前提」に文書なしで返す段が無い"
# CE の推奨案はオーナーに迎合させない（docs/decisions/2026-11-20-ce-recommend-own-best.md）。4点は役の段落に残り、
# 「決めるのはオーナー」は線の中の決定のこと。作り方の決定は CE が決め、古い「決めない」の文は無い（ADR-0037 受け入れ条件 1）
ce0=$(sed -n '/^\*\*あなたは Chief Engineer/,/^- 課題は/p' "$ce")
old_rule='推奨案は出すが決めない|自分で答えて先へ進んだら骨格の違反'
grep -qF '自分のプロダクトだと思って最適と考える案にする' <<<"$ce0" && grep -qF 'オーナーの好みに合わせて推奨を変えない' <<<"$ce0" \
  && grep -qF '事実か理由で推奨の前提を崩したときだけ' <<<"$ce0" && grep -qF '線の中の決定は、推奨を変えなくても決めるのはオーナー' <<<"$ce0" \
  && grep -qF '作り方の決定はあなたが決め、下の「オーナーに聞く線」の中の決定はオーナーが決める' <<<"$ce0" \
  && ! grep -qE "$old_rule" "$ce" \
  && pass "ADR-0037 A1: CE の役の段落は作り方の決定を CE が、線の中の決定をオーナーが決めると言い、推奨の立て方の4点が残り、「推奨案は出すが決めない」が無い" \
  || fail "ADR-0037 A1: chief-engineer の役の段落に線の分け方か推奨の立て方の4点が無いか、「推奨案は出すが決めない」「骨格の違反」の文が残る"
# 否定の検査が当たる場合があること: 古い skill の行には同じ grep が当たる
grep -qE "$old_rule" <<<'   - **推奨案は出すが決めない**。自分で答えて先へ進んだら骨格の違反。' \
  && pass "ADR-0037 A1: 古い「決めない」の行は否定の grep に当たる（検査が見ている）" || fail "ADR-0037 A1: 否定の grep が何も見ていない"
# ADR-0037 受け入れ条件 2: 線の表（a〜g）と「どちらとも読める決定は Chief Engineer が決める」
cel=$(sed -n '/^## オーナーに聞く線$/,/^## 骨格/p' "$ce")
miss=
for r in 'a business' 'b users' 'c promise' 'd irreversible' 'e line'; do
  grep -q "^| ${r% *} | .* | \`${r#* }\` |\$" <<<"$cel" || miss="$miss [${r% *}]"
done
grep -q '^| f | オーナーが前に決めたことを覆すこと | ' <<<"$cel" || miss="$miss [f]"
grep -q '^| g | 事実で決まらず、答えで設計の大半が分かれる不明点 | ' <<<"$cel" || miss="$miss [g]"
grep -qF '**どちらとも読める決定は Chief Engineer が決める。**' <<<"$cel" || miss="$miss [どちらとも読める]"
grep -qF '印の無い過去の決定は、オーナーが決めたものとして読む' <<<"$cel" || miss="$miss [印の無い過去の決定]"
[ -z "$miss" ] && pass "ADR-0037 A2: CE skill に線の表（a〜g）と「どちらとも読める決定は Chief Engineer が決める」がある" || fail "ADR-0037 A2: CE skill の線:$miss"
# ADR-0037 受け入れ条件 3: 骨格は入口（言い直しと前提を1問、続けて範囲）・真ん中（聞くのは g だけ）・出口（要点・線の中の決定・最後の確かめ）
miss=
for k in '^## 骨格（この順で。オーナーと話すのは 2・3 の入口と 7 の出口）$' \
  '^2\. \*\*入口: 言い直しと前提を1問で確かめる。\*\*' '^3\. \*\*入口: 続けて範囲を聞く。\*\*' \
  '^4\. \*\*真ん中: 決定の一覧の全部に答えを入れる（本体）。\*\*' '^5\. \*\*代案を2つ以上作り、自分で選ぶ。\*\*' '^6\. \*\*反論を走らせる' \
  '^7\. \*\*出口: 要点を見せ、線の中の決定を1問ずつ聞き、最後の確かめを1問で取る。\*\*' '^8\. \*\*文書にして渡す。\*\*' \
  '真ん中でオーナーに聞いてよいのは、線の g の不明点だけ' '\*\*真ん中では、問いを出すとき以外は止まらない。\*\*' \
  '価値の仮説は独立の問いにせず、要点に載せて最後の確かめで一緒に通す'; do
  grep -q "$k" "$ce" || miss="$miss [$k]"
done
grep -qF '待つことが本業' "$ce" && miss="$miss [待つことが本業が残る]"
[ -z "$miss" ] && pass "ADR-0037 A3: CE skill の骨格は入口・真ん中・出口の形" || fail "ADR-0037 A3: CE skill の骨格:$miss"
# ADR-0037 受け入れ条件 4: 文書の形の「決めたこと」に誰が決めたか
grep -q '^## 決めたこと（番号・日付・誰が決めたか・' <<<"$(block "$ce" '# <題>（仕様・設計。')" \
  && grep -qF '**「決めたこと」の各行には、誰が決めたかを書く。** 書き方は、決定の1文の末尾の「（オーナーが決めた）」か「（Chief Engineer が決めた）」。' "$ce" \
  && grep -qF '「仮」とは書かない' "$ce" \
  && pass "ADR-0037 A4: CE skill の文書の形は「決めたこと」の各行に誰が決めたかを書く" || fail "ADR-0037 A4: CE skill の文書の形に誰が決めたかが無い"
# ADR-0037 受け入れ条件 5: CoS の手引き（決めたこと 8）と、問いの出し方の skill の1行（決めたこと 9）
cosr=$(sed -n '/^## Chief Engineer の文書を受けたら$/,/^## 観察の課題/p' "$pb/cut.md")
old_cos='文書が人の言っていないことを言っている・'
grep -qF '「オーナーに聞く線」（a〜g）に当たる決定が「（Chief Engineer が決めた）」の印で書かれていないかを見る。見つけたら、切らずにオーナーへ知らせる。' <<<"$cosr" \
  && grep -qF '（検収は、オーナーが Chief Engineer の出口の最後の確かめでした）' <<<"$cosr" && ! grep -qF "$old_cos" <<<"$cosr" \
  && grep -qF -- '- Chief Engineer が自分の設計の決定をオーナーに聞くかどうかは、この節でなく chief-engineer skill の「オーナーに聞く線」に従う。' package/skills/ask-one-by-one/SKILL.md \
  && pass "ADR-0037 A5: CoS の cut.md は線に当たる CE の印の決定を見て知らせ、ask-one-by-one に CE は自分の線に従う1行がある" \
  || fail "ADR-0037 A5: cut.md の CE の文書を受ける手順か、ask-one-by-one の1行が古い"
grep -qF "$old_cos" <<<'6. 文書が人の言っていないことを言っている・受け入れ条件がテストできない形なら、切らずに人へ知らせる' \
  && pass "ADR-0037 A5: 古い cut.md の行は否定の grep に当たる（検査が見ている）" || fail "ADR-0037 A5: 否定の grep が何も見ていない"
# 線の確かめは、空の価値の仮説で戻すこと・切ること・観察の課題を足すことより先（切ってからでは担当が先に動く）
line_of() { grep -n -m1 -F -- "$2" <<<"$1" | cut -d: -f1; }
ce_order() { local a b c d; a=$(line_of "$1" '「オーナーに聞く線」（a〜g）に当たる決定'); b=$(line_of "$1" '「価値の仮説」の節が見出しだけ')
  c=$(line_of "$1" '「依頼書に切る案」を縦に薄く'); d=$(line_of "$1" '**観察の課題を1件**足す')
  [ -n "$a" ] && [ -n "$b" ] && [ -n "$c" ] && [ -n "$d" ] && [ "$a" -lt "$b" ] && [ "$a" -lt "$c" ] && [ "$a" -lt "$d" ]; }
ce_order "$cosr" && pass "ADR-0037 A5: cut.md は線の確かめを、価値の仮説で戻す・切る・観察の課題を足すより先に置く" || fail "ADR-0037 A5: cut.md の線の確かめが、切る手順より後にある"
# ADR-0038 受け入れ条件 3（Chief Engineer の決まりの文）: statechart の段（当てはまる設計の線・案を見せる前・反論に渡す・
# 文書の 1 行・資料として渡す）と、モデルのファイルを添える例外（CE の skill の渡し方と cut.md の 2 か所）
ces=$(sed -n '/^## 状態を動かす設計をモデルで調べる$/,/^## 文書の形$/p' "$ce")
miss=
for k in '**当てはまる設計の線。**' '状態を動かす流れそのものを変える設計' 'モデルは反論の代わりにしない' \
  'pstack の statechart' '`~/.claude/plugins/installed_plugins.json`' '`skills/statechart/SKILL.md`' '「本物のコードで再現する」はしない' \
  '`issues/<id>/library/` の下' 'repo のモデルには書かない' \
  '**案を見せる前に調べる。**' '推奨案は必ずモデルにして調べる' 'どの案を選ぶかは骨格 5 のとおりあなたが決める' \
  '**反論にモデルも渡す。**' '反論（骨格 6）は調べたあとに走らせる' \
  '**文書に1行書く。**' '`モデル: 調べた（規則 N 本・見つけた破れ M 件）`' '`モデル: 調べなかった（<理由>）`' \
  '**資料として渡す。**'; do
  grep -qF -- "$k" <<<"$ces" || miss="$miss [段: $k]"
done
grep -qF '状態を動かす流れを変えるか（下の「状態を動かす設計をモデルで調べる」の線）も決める' <<<"$ce2" || miss="$miss [入口で当てはまるかを決める]"
model_ex='例外として、検討で調べたモデルのファイルは、資料として添えてよい。依頼書はそのパスを指してよい'
sed -n '/^## 渡し方$/,/^## 取らないもの$/p' "$ce" | grep -qF "$model_ex" || miss="$miss [CE の渡し方の例外]"
grep -qF '"最初の実装の課題の依頼書に <モデルのパス> を添える"' "$ce" || miss="$miss [CE の next にモデルを添える行]"
grep -qF "（渡されるのは文書だけ。$model_ex）" <<<"$cosr" || miss="$miss [cut.md の例外]"
grep -qF '`next` に「最初の実装の課題の依頼書に <モデルのパス> を添える」があれば' <<<"$cosr" || miss="$miss [cut.md がモデルのパスを依頼書に書く]"
# repo へ入れるのはモデルが既にある流れだけ。モデルが無い流れのモデルは repo に置かない（ADR-0038 決めたこと 5）
model_bound='repo にその流れのモデルがあれば、実装の PR でそこへ入れる。モデルが無い流れのモデルは資料のまま使い切り、repo に新しく置かない'
unbound='実装の PR で repo のモデル[^（あ]*へ入れる'
cesd=$(sed -n '/^\*\*状態を動かす設計をモデルで調べる\*\*/,/^\*\*成果物の形\*\*/p' docs/design.md)
for t in "$ces" "$cosr" "$cesd"; do
  grep -qF "$model_bound" <<<"$t" || { miss="$miss [モデルを repo へ入れるのはモデルがある流れだけ]"; break; }
done
grep -qE "$unbound" "$ce" "$pb/cut.md" docs/design.md && miss="$miss [モデルの無い流れにも repo へ入れると読める文が残る]"
# 置き場のパスはバージョンごとに変わるので skill に書かない（ADR-0038 決めたこと 12）
pstack_path='plugins/cache/[^ ]*pstack/[0-9]'
grep -qE "$pstack_path" "$ce" && miss="$miss [バージョンの入った pstack の置き場のパスが skill にある]"
[ -z "$miss" ] && pass "ADR-0038 A3: CE skill に statechart の段（線・案を見せる前・反論に渡す・文書の1行・資料として渡す）、渡し方の例外が CE skill と cut.md の2か所にあり、repo へ入れるのはモデルがある流れだけ" \
  || fail "ADR-0038 A3: CE の決まりの文:$miss"
grep -qE "$pstack_path" <<<'/opt/x/.claude/plugins/cache/cursor-plugins/pstack/0.15.43/skills/statechart/SKILL.md' \
  && grep -qE "$unbound" <<<'担当はそれを土台に実装し、実装の PR で repo のモデルへ入れる。' \
  && pass "ADR-0038 A3: バージョンの入った pstack の置き場のパスと、限らずに repo へ入れる文は否定の grep に当たる（検査が見ている）" || fail "ADR-0038 A3: 否定の grep が何も見ていない"
! ce_order "$(printf '%s\n' '3. 「依頼書に切る案」を縦に薄く' '4. **観察の課題を1件**足す' '2. 「価値の仮説」の節が見出しだけ' '6. 切る前に、「オーナーに聞く線」（a〜g）に当たる決定')" \
  && pass "ADR-0037 A5: 線の確かめが後ろにある並びは順の検査に当たる（検査が見ている）" || fail "ADR-0037 A5: 順の検査が何も見ていない"
# ADR-0037 受け入れ条件 6: design.md §1.8・§1.9 と dispatcher.md に古い文が残らない
old_docs='推奨案は出すが決めない|待つことが Chief Engineer の本業|docs がレベル A／B なら自動|検収は人が対話の中でした|人と Chief Engineer の対話で決める'
left=$(grep -nE "$old_docs" docs/design.md docs/dispatcher.md || true)
[ -z "$left" ] && grep -qF '作り方の決定と、どちらとも読める決定。オーナーに聞く線（7 種）の中の決定はオーナーが決める' docs/design.md \
  && grep -qF 'Chief Engineer の課題の文書の PR は、どの案件でもいつもオーナーがマージする' docs/design.md \
  && pass "ADR-0037 A6: design.md の役の表と骨格と dispatcher.md が新しい手順で、古い文が無い" || fail "ADR-0037 A6: design.md か dispatcher.md に古い文が残るか、新しい文が無い:$left"
grep -qE "$old_docs" <<<'Chief of Staff は Chief Engineer の課題を**検収しない**（検収は人が対話の中でした）' \
  && pass "ADR-0037 A6: 古い design.md の行は否定の grep に当たる（検査が見ている）" || fail "ADR-0037 A6: 否定の grep が何も見ていない"
left=$(grep -nE 'kanban ask [^-]*"<|kanban set <id> pr=' package/skills/*/SKILL.md package/skills/*/playbooks/*.md || true)
[ -z "$left" ] && pass "skill に JSON を通らない kanban ask と issue.yaml の pr が残っていない" || fail "JSON を通らない書き方が残る: $left"
