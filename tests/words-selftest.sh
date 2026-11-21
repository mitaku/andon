#!/usr/bin/env bash
# tests/words.sh が「今」の語で落ち、例外では落ちないこと（ADR-0013 受け入れ条件 3）。
# 役を指す「人」で落ち、役を指さない「人」では落ちないこと（ADR-0036 受け入れ条件 2）。
# 句読点（、。・）のすぐ後ろの空白と、英数字と句読点の間の空白で落ち、URL の前後・表の区切り・矢印の前では落ちないこと。
# どの段も、範囲がファイル1つの根でもファイル名つきで当たりを読んで落ち、ファイル名で読む例外が効くこと。
# 一時の根に小さな写しを作り、words.sh にその根を渡す。
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail=0
adr=docs/adr/0013-short-role-names-and-plain-japanese.md

# setup: ADR-0013 の写しと、バージョンの節のある CHANGELOG だけの根
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r/docs/adr"
  cp "$adr" "$tmp/r/$adr"
  printf '# CHANGELOG\n\n## 未リリース\n\n## v0.0.1\n- 費用を数えた\n' >"$tmp/r/CHANGELOG.md"
}
expect() { # expect ok|fail <説明>
  if bash tests/words.sh "$tmp/r" >/dev/null 2>&1; then got=ok; else got=fail; fi
  if [ "$got" = "$1" ]; then echo "ok   $2"; else echo "FAIL $2（$got）"; fail=1; fi
}

setup; expect ok '写し（ADR-0013 の引用・語の表・語を挙げた所、CHANGELOG のバージョンの節）は通る'
setup; printf '\n地の文で費用を数える。\n' >>"$tmp/r/$adr"; expect fail 'ADR-0013 の地の文の「費用」で落ちる'
setup; printf '\n- 人: 「費用もコストで」。直す語（持ち主→オーナー）。`起こした`\n   | 提供元 | プロバイダ | 理由 |\n' >>"$tmp/r/$adr"
expect ok 'ADR-0013 の引用・括弧・`→`・コード・語の表の「今」の列は通る'
for w in 'エージェントを起こす' 'レビュアーを起こさない' 'repo を迎えた' '文書の席だけ置く' '提供元' '起きたとき'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect fail "ほかの文書の「$w」で落ちる"
done
for w in '離席中' '問題を引き起こす' '何が起きたか'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect ok "「$w」は通る"
done
setup; sed -i 's/^## 未リリース$/&\n- 費用を数える/' "$tmp/r/CHANGELOG.md"; expect fail 'CHANGELOG の未リリースの「費用」で落ちる'
# repo の中の skill の置き場（ADR-0035 決めたこと 4）も見る
setup; mkdir -p "$tmp/r/.claude/skills/verify-andon"; echo '担当を起こす' >"$tmp/r/.claude/skills/verify-andon/SKILL.md"
expect fail '.claude/skills/ の skill の「起こす」で落ちる'

# 役を指す「人」（ADR-0036 受け入れ条件 2）: 替えた範囲（package/）に足すと落ち、役を指さない「人」は通る
for w in '人がマージする' '人の判断待ち' '人に聞く' 'gate は人へ回す' '人（2026-11-10）: 「残して」'; do
  setup; mkdir -p "$tmp/r/package"; echo "$w" >"$tmp/r/package/x.md"; expect fail "package/ の「$w」で落ちる"
done
for w in '外の人が読む' '1人にする' '人が読む写し' 'セルに 1 人（owner）' 'ほかの人の作業' '人向けの要約' '人と AI が読む' '人の名前を写さない' 'オーナーの判断待ち'; do
  setup; mkdir -p "$tmp/r/package"; echo "$w" >"$tmp/r/package/x.md"; expect ok "package/ の「$w」は通る"
done
setup; echo '人がマージする' >"$tmp/r/docs/x.md"; expect ok '替えていない範囲（docs/ の残り）の役の「人」はまだ見ない'
# docs/design.md は替えた範囲。引用（ビジョンの写し・freee の KPI 名・pstack の原則）だけは通る
for w in '人がマージする' '人（2026-11-05）: 「残して」' '| 人の接点 | 段に固定 |'; do
  setup; echo "$w" >"$tmp/r/docs/design.md"; expect fail "docs/design.md の「$w」で落ちる"
done
for w in '**Andon は「選択的 Dark Factory」。止められるものは機械、判断が要るものは LLM、残りだけ人。ラインは回り続け（AND ON）、人を呼ぶときだけ灯りがつく（アンドン）。**' \
  'freee の KPI「人の介在率」' 'pstack の「人を待たない」を出口に当てはめること' 'オーナーの介在回数・人間の介在回数'; do
  setup; echo "$w" >"$tmp/r/docs/design.md"; expect ok "docs/design.md の「${w:0:40}」は通る"
done
setup; echo 'freee の KPI「人の介在率」。人の答え 2026-11-07' >"$tmp/r/docs/design.md"
expect fail 'docs/design.md の引用と同じ行の地の文の「人の答え」で落ちる'
# 生きている文書（docs/ の4本と root の5本）も替えた範囲。CHANGELOG.md は未リリースの節だけ
for f in docs/contract.md docs/dispatcher.md docs/operate.md docs/image.md README.md AGENTS.md REVIEW.md SECURITY.md; do
  setup; echo '| マージは人がする。 | 無し。 |' >"$tmp/r/$f"; expect fail "$f の「人がする」で落ちる"
done
setup; sed -i 's/^## 未リリース$/&\n- 人の判断待ちの課題を数える/' "$tmp/r/CHANGELOG.md"; expect fail 'CHANGELOG の未リリースの「人の判断待ち」で落ちる'
setup; printf -- '- 人の判断待ちの課題を数えた\n' >>"$tmp/r/CHANGELOG.md"; expect ok 'CHANGELOG の版の節の「人の判断待ち」は通る'
# docs/contract.md が古い語も読むと書いた所と、README の ADR の一覧（ADR の題の写し）は通り、同じ行の地の文は当たる
for w in '`## オーナーに聞くこと`（古い見出し `## 人に聞くこと` も同じに読む）' '題の末尾が「（オーナーの判断待ち）」。古い末尾「（人の判断待ち）」も同じに読む' \
  '答え（前のバージョンの「人がマージした（…）」か、今の「オーナーがマージした（…）」）'; do
  setup; echo "$w" >"$tmp/r/docs/contract.md"; expect ok "docs/contract.md の「${w:0:40}」は通る"
done
setup; echo '古い見出し `## 人に聞くこと` も同じに読む。人が答える' >"$tmp/r/docs/contract.md"
expect fail 'docs/contract.md の古い見出しと同じ行の地の文の「人が答える」で落ちる'
setup; echo '`## 人に聞くこと` を拾う' >"$tmp/r/docs/contract.md"; expect fail 'docs/contract.md の「古い見出し」と書かない `## 人に聞くこと` で落ちる'
setup; echo '  - [0009 Chief of Staff を人の既定の入口にする](docs/adr/0009-cos-default-entry.md): 決定' >"$tmp/r/README.md"
expect fail 'README.md の ADR の一覧の行（ADR の題の写し）の「人の既定の入口」で落ちる（ADR の題も替えた）'
setup; echo '- [docs/contract.md](docs/contract.md) — 人の手直し' >"$tmp/r/README.md"; expect fail 'README.md の一覧でない行の「人の手直し」で落ちる'
# ADR と決定の記録（docs/adr/・docs/decisions/・docs/decisions.md）も替えた範囲。名指しした引用とバッククォートの中は外し、
# ほかの「」の中（引用でない言い回し・節の名前の写し）と地の文は当たる
for f in docs/adr/0001-x.md:人がマージする docs/decisions/2026-01-01-x.md:人の判断待ち 'docs/decisions.md:| 2026-11-05 | 人に聞く | 問い1 |' \
  'docs/adr/0003-x.md:人（2026-11-10）: 「残して」' 'docs/adr/0006-x.md:担当には「人に聞く」しか無い' \
  'docs/adr/0009-x.md:手引きの「人の入口」の節' 'docs/adr/0002-x.md:4. **2026-11-05 人が起動する 2 役**' \
  'docs/adr/0007-x.md:A5 人のマージとの重なり' 'docs/adr/0024-x.md:**報告書**（2026-11-12 人）: 別の案' \
  'docs/adr/0003-x.md:引用の前の名札（人 2026-11-10）: 「残して」'; do
  setup; mkdir -p "$tmp/r/$(dirname "${f%%:*}")"; echo "${f#*:}" >"$tmp/r/${f%%:*}"; expect fail "替えた範囲（${f%%:*}）の「${f#*:}」で落ちる"
done
for w in 'オーナーの言葉（「人の他に、Sentry や Cloudflare、Linear、GitHub が Signal になるといい」）' \
  '43 件は「人がマージした」という答えだった' 'pstack の「人を待たない」' '当時の見出し `## 人に聞くこと` を拾う' \
  '外の人が読む' '2人いる' 'レビュアーは 1 人ずつ' '2026-11-05 2 人目が走る'; do
  setup; echo "$w" >"$tmp/r/docs/adr/0001-x.md"; expect ok "docs/adr/ の「$w」は通る"
done
setup; echo 'pstack の「人を待たない」。人が答える' >"$tmp/r/docs/adr/0001-x.md"; expect fail 'docs/adr/ の名指しした引用と同じ行の地の文の「人が答える」で落ちる'
# ADR-0036 とその決定のファイルの語の表の「今」の列と「X→」の形、ADR-0005・ADR-0011 の役でない句は、そのファイルでだけ通る
for f in docs/adr/0036-owner-not-hito.md docs/decisions/2026-11-20-role-name-replacement-reaches-frozen-records.md; do
  setup; mkdir -p "$tmp/r/$(dirname "$f")"; printf '   | 人の判断待ち | オーナーの判断待ち |\n# 置き換え（人→オーナー）\n' >"$tmp/r/$f"
  expect ok "$f の語の表の「今」の列と「人→オーナー」は通る"
done
setup; printf '   | 人の判断待ち | オーナーの判断待ち |\n' >"$tmp/r/docs/adr/0001-x.md"; expect fail 'ADR-0036 の外の語の表の「人の判断待ち」で落ちる'
setup; printf '   | オーナーの判断待ち | 人の判断待ち |\n' >"$tmp/r/docs/adr/0036-owner-not-hito.md"; expect fail 'ADR-0036 の語の表の「今」でない列の「人の判断待ち」で落ちる'
setup; echo 'kanban は、人と人が共有する issue の置き場ではない' >"$tmp/r/docs/adr/0005-cell-repo.md"; expect ok 'ADR-0005 の「人と人が共有する」は通る'
setup; echo 'open/closed はトラッカー側の人が決める' >"$tmp/r/docs/adr/0011-signals-and-value-loop.md"; expect ok 'ADR-0011 の「トラッカー側の人が決める」は通る'
setup; echo 'kanban は、人と人が共有する issue の置き場ではない' >"$tmp/r/docs/adr/0004-x.md"; expect fail 'ADR-0005 の外の「人と人が共有する」で落ちる'
# コマンドの出力とコードと検査（internal/・init/・cmd/・tests/）も見る
for d in internal init cmd tests; do
  setup; mkdir -p "$tmp/r/$d"; echo '"人の判断が要る"' >"$tmp/r/$d/x.go"; expect fail "$d/ の「人の判断が要る」で落ちる"
done
# 流れの定義と生成した文書とモデル（process/・internal/process/・tests/process/・docs/process/・model/）も見る
for f in process/states/x.yaml internal/process/x.go tests/process/process/workstreams/x.yaml docs/process/x.md model/X.tla; do
  setup; mkdir -p "$tmp/r/$(dirname "$f")"; echo '人がマージする' >"$tmp/r/$f"; expect fail "$f の「人がマージする」で落ちる"
done
setup; mkdir -p "$tmp/r/model/broken"; echo '\* レビュアーが 2 人走り' >"$tmp/r/model/broken/X.tla"; expect ok 'model/ の「2 人走り」（数え方）は通る'
setup; mkdir -p "$tmp/r/docs/process"; echo '# 定義（人が読む入口）' >"$tmp/r/docs/process/README.md"; expect ok 'docs/process/ の「人が読む入口」は通る'
# 古い語を読む所は通り、同じ語がほかのファイルにあれば落ちる
setup; mkdir -p "$tmp/r/internal/triage"; echo 'case "人に聞くこと", "オーナーに聞くこと":' >"$tmp/r/internal/triage/x.go"
expect ok 'internal/triage/ の古い見出しを読む所は通る'
setup; mkdir -p "$tmp/r/internal/issue"; echo 'case "人に聞くこと", "オーナーに聞くこと":' >"$tmp/r/internal/issue/x.go"
expect fail 'internal/triage/ の外の「人に聞くこと」で落ちる'

# 「台帳」「ゲート」「版」（ADR-0040 受け入れ条件 2）: 替えた範囲（package/）に足すと落ち、残す形と「」の中は通る
for w in '台帳に書く' '役・ゲート・約束' '自動マージゲート' '前の版に戻す' '版を上げる' '各版の節' '`<skill>@<版>`'; do
  setup; mkdir -p "$tmp/r/package"; echo "$w" >"$tmp/r/package/x.md"; expect fail "package/ の「$w」で落ちる"
done
for w in '最新版にする' '初版と比べる' '英語版を読む' '2006 版では' 'Go 版を使う' '用語集の「台帳」と「ゲート」は消す' '「前の版」は「前のバージョン」と書く'; do
  setup; mkdir -p "$tmp/r/package"; echo "$w" >"$tmp/r/package/x.md"; expect ok "package/ の「$w」は通る"
done
setup; mkdir -p "$tmp/r/package/skills/glossary"; echo '_避ける_: 台帳（2026-11-20 までの語）' >"$tmp/r/package/skills/glossary/SKILL.md"
expect ok '用語集の「_避ける_」の行の「台帳」は通る'
setup; mkdir -p "$tmp/r/package/skills/worker"; echo '_避ける_: 台帳' >"$tmp/r/package/skills/worker/SKILL.md"
expect fail '用語集でない skill の「_避ける_」の行の「台帳」で落ちる'
setup; mkdir -p "$tmp/r/.yomiyasu-words"; echo '前の版' >"$tmp/r/.yomiyasu-words/allow.txt"; expect fail '.yomiyasu-words/ の「前の版」で落ちる'
setup; echo '台帳に書く' >"$tmp/r/docs/x.md"; expect ok '替えていない範囲（docs/）の「台帳」はまだ見ない'
# 古い語を読む所（ADR-0040 決めたこと 7）は通り、同じ語がほかの行にあれば落ちる
setup; mkdir -p "$tmp/r/internal/issue"; echo 'var oldWords = [][2]string{{"台帳", "kanban"}, {"ゲート", "gate"}, {"版", "バージョン"}}' >"$tmp/r/internal/issue/settle.go"
expect ok 'internal/issue/settle.go の読み替えの表は通る'
setup; mkdir -p "$tmp/r/internal/issue"; echo 'return "ゲートの外の問いが開いている: "' >"$tmp/r/internal/issue/settle.go"
expect fail 'internal/issue/settle.go の読み替えの表でない行の「ゲート」で落ちる'
setup; mkdir -p "$tmp/r/internal/dispatcher"; echo 'var oldWords = [][2]string{{"台帳", "kanban"}}' >"$tmp/r/internal/dispatcher/x.go"
expect fail 'internal/issue/settle.go の外の読み替えの表で落ちる'
setup; mkdir -p "$tmp/r/tests"; echo 'jq -n --arg r "ゲートの記録 $L/x.json" \' >"$tmp/r/tests/andon-issue.sh"
expect ok 'tests/andon-issue.sh の古い語の事実の行で問いを置く行は通る'
setup; mkdir -p "$tmp/r/tests"; echo 'die "ゲートの記録が無い"' >"$tmp/r/tests/andon-dispatcher.sh"
expect fail 'tests/andon-dispatcher.sh のほかの行の「ゲート」で落ちる'
setup; mkdir -p "$tmp/r/internal/gate"; echo 'const oldRecordFact = "ゲートの記録 " // 古い語を読む（ADR-0040 決めたこと 7）' >"$tmp/r/internal/gate/gate.go"
expect ok 'internal/gate/ の、末尾に古い語を読む印のある行は通る'
setup; mkdir -p "$tmp/r/internal/gate"; echo 'return "ゲートの記録 " // 古い語を読む（ADR-0040 決めたこと 7）。あとに文' >"$tmp/r/internal/gate/gate.go"
expect fail 'internal/gate/ の、印が末尾でない行の「ゲート」で落ちる'
setup; mkdir -p "$tmp/r/internal/boot"; echo 'x := "ゲート" // 古い語を読む（ADR-0040 決めたこと 7）' >"$tmp/r/internal/boot/x.go"
expect fail 'internal/gate/ の外では、末尾に印があっても「ゲート」で落ちる'
for d in cmd/kanban init/bin internal/measure; do
  setup; mkdir -p "$tmp/r/$d"; echo '// 前の版の台帳' >"$tmp/r/$d/x"; expect fail "$d/ の「前の版の台帳」で落ちる"
done
setup; mkdir -p "$tmp/r/tests"; echo '# 本物の台帳で回す' >"$tmp/r/tests/andon-gate.sh"; expect fail 'tests/andon-gate.sh の「台帳」で落ちる'
setup; mkdir -p "$tmp/r/internal/process"; echo '台帳の CLI' >"$tmp/r/internal/process/x.go"; expect fail 'internal/process/ の「台帳の CLI」で落ちる'
for d in tests tests/lib tests/package .claude/skills/verify-andon/scripts; do
  setup; mkdir -p "$tmp/r/$d"; echo '# 前の版の台帳で回す' >"$tmp/r/$d/x.sh"; expect fail "$d/ の「前の版の台帳」で落ちる"
done
setup; mkdir -p "$tmp/r/tests/process"; echo '台帳（review）' >"$tmp/r/tests/process/x.yaml"; expect fail 'tests/process/ の「台帳（review）」で落ちる'
# 流れの定義・生成した文書・モデル・image・repo の AGENTS.md と CI の設定（ADR-0040 依頼書に切る案 3）
for f in process/states/x.yaml process/workstreams/x.yaml docs/process/x.md model/x.tla model/README.md image/x.sh AGENTS.md .golangci.yml workflow_test.go; do
  for w in 'kanban（review）でなく台帳（review）' 'ゲートを回す' '前の版に戻す'; do
    setup; mkdir -p "$(dirname "$tmp/r/$f")"; echo "$w" >"$tmp/r/$f"; expect fail "$f の「$w」で落ちる"
  done
done
setup; mkdir -p "$tmp/r/image"; echo '# 最新版の Node と初版の手順。「ゲート」という語は使わない' >"$tmp/r/image/x.sh"; expect ok 'image/ の「最新版」「初版」と「」の中は通る'
setup; mkdir -p "$tmp/r/.github/workflows"; echo '# ゲートは checks を待つ。golangci-lint の版' >"$tmp/r/.github/workflows/x.yml"; expect ok '範囲の外（.github/）の「ゲート」「版」は見ない'
# 古い語を見る検査の行（行の末尾の印）は通り、印が末尾でない行と、tests/ の外の印は落ちる
m='# 古い語を見る（ADR-0040 決めたこと 10）'
setup; mkdir -p "$tmp/r/tests/package"; echo "grep -q '^_避ける_: 台帳' \"\$g\"  $m" >"$tmp/r/tests/package/08-x.sh"
expect ok 'tests/ の .sh の、末尾に印のある行の「台帳」は通る'
setup; mkdir -p "$tmp/r/tests/package"; echo "grep -q '台帳' \"\$g\"  $m。あとに文" >"$tmp/r/tests/package/08-x.sh"
expect fail 'tests/ の、印が末尾でない行の「台帳」で落ちる'
setup; mkdir -p "$tmp/r/tests"; echo "x 台帳  $m" >"$tmp/r/tests/x.md"; expect fail 'tests/ の .sh でないファイルでは、末尾に印があっても「台帳」で落ちる'
setup; mkdir -p "$tmp/r/.claude/skills/verify-andon/scripts"; echo "x 台帳  $m" >"$tmp/r/.claude/skills/verify-andon/scripts/x.sh"
expect fail 'tests/ の外では、末尾に印があっても「台帳」で落ちる'
setup; echo 'gate が kanban の課題を読む。版を上げる' >"$tmp/r/docs/design.md"; expect fail '替えた範囲（docs/design.md）の「版」で落ちる'
setup; echo '日本語の呼び名を「台帳」のままにした。2006 版では' >"$tmp/r/docs/design.md"; expect ok 'docs/design.md の「」の中と「2006 版」は通る'
# 生きている文書（docs/ の5本と root の4本）も替えた範囲。CHANGELOG.md は未リリースの節だけ、README.md は ADR の一覧の行を外す
for f in docs/contract.md:台帳に書く docs/dispatcher.md:ゲートを回す docs/operate.md:版を上げるとき docs/image.md:各版の節 docs/ledger.md:台帳の形式 \
  README.md:役・ゲート・約束 REVIEW.md:前の版に戻す SECURITY.md:台帳の外; do
  setup; echo "${f#*:}" >"$tmp/r/${f%%:*}"; expect fail "替えた範囲（${f%%:*}）の「${f#*:}」で落ちる"
done
setup; sed -i 's/^## 未リリース$/&\n- 前の版の台帳を読む/' "$tmp/r/CHANGELOG.md"; expect fail 'CHANGELOG の未リリースの「版」「台帳」で落ちる'
setup; printf -- '- 前の版の台帳を読む。ゲートを回す\n' >>"$tmp/r/CHANGELOG.md"; expect ok 'CHANGELOG のバージョンの節の「版」「台帳」「ゲート」は通る'
setup; echo '  - [0040 「台帳」「ゲート」「版」をやめ、kanban・gate・バージョンと書く](docs/adr/0040-x.md): 決定' >"$tmp/r/README.md"
expect ok 'README.md の ADR の一覧の行の「」の中の 3 語は通る'
setup; echo '  - [0040 「台帳」をやめる。ゲートを回す](docs/adr/0040-x.md): 決定' >"$tmp/r/README.md"
expect fail 'README.md の ADR の一覧の、0025 から後の行の「ゲート」で落ちる（ADR を全部替えた）'
setup; echo '  - [0008 台帳のカードを「課題（issue）」に改名する](docs/adr/0008-issue-naming.md): 決定' >"$tmp/r/README.md"
expect fail 'README.md の ADR の一覧の 0001〜0024 の行の「台帳」で落ちる'
setup; echo '- 台帳のカードを「課題（issue）」に改名する' >"$tmp/r/README.md"; expect fail 'README.md の ADR の一覧でない行の「台帳」で落ちる'
# ADR は全部を見る。ADR-0018 のオーナーの期待の 1 行と、ADR-0040 の決めたことの表の「今」の列は外す
for f in docs/adr/0001-x.md docs/adr/0013-short-role-names-and-plain-japanese.md docs/adr/0024-x.md docs/adr/0025-x.md docs/adr/0039-x.md docs/adr/0041-x.md; do
  for w in '台帳に書く' 'ゲートを回す' '前の版に戻す'; do
    setup; echo "$w" >>"$tmp/r/$f"; expect fail "$f の「$w」で落ちる"
  done
done
a40=docs/adr/0040-kanban-gate-version-words.md
setup; printf '   | 台帳に書く、ゲートを回す、前の版、各版、推奨版 | kanban に書く、gate を回す |\n' >"$tmp/r/$a40"
expect ok 'ADR-0040 の決めたことの表の「今」の列の 3 語は通る'
setup; printf '   | 台帳に書く | ゲートを回す |\n' >"$tmp/r/$a40"; expect fail 'ADR-0040 の表の「今」でない列の「ゲート」で落ちる'
setup; printf '台帳に書く\n' >"$tmp/r/$a40"; expect fail 'ADR-0040 の表でない行の「台帳」で落ちる'
setup; printf '| 1 | 台帳に書く | この ADR |\n' >"$tmp/r/$a40"; expect fail 'ADR-0040 の決定の索引（頭が「| 」）の「台帳」で落ちる'
setup; printf '   | 台帳に書く | kanban に書く |\n' >"$tmp/r/docs/adr/0036-x.md"; expect fail 'ADR-0040 の外の表の「今」の列の「台帳」で落ちる'
setup; echo '初版と比べる。「台帳」と「ゲート」をやめる' >"$tmp/r/docs/adr/0001-x.md"; expect ok 'ADR 0001 の「初版」と「」の中は通る'
o='オーナーの期待（2026-11-11）: 検証のゲートを手厚くする。PR に証憑を正しく載せる'
setup; echo "$o" >"$tmp/r/docs/adr/0018-screen-check-tooling.md"; expect ok 'ADR-0018 のオーナーの期待の行の「ゲート」は通る'
setup; printf '%s\nゲートを回す\n' "$o" >"$tmp/r/docs/adr/0018-screen-check-tooling.md"; expect fail 'ADR-0018 のほかの行の「ゲート」で落ちる'
setup; echo "$o" >"$tmp/r/docs/adr/0017-x.md"; expect fail 'ADR-0018 の外では、オーナーの期待の行でも「ゲート」で落ちる'
# docs/decisions.md は冒頭（最初の「## 1.」より前）だけを見て、凍結した表は外す
setup; printf '# 決定の記録\n\n- セルの台帳や wiki は指さない\n\n## 1. 設計書 §6 のオーナーの答え\n\n| 日付 | 決定 |\n' >"$tmp/r/docs/decisions.md"
expect fail 'docs/decisions.md の冒頭の「台帳」で落ちる'
setup; printf '# 決定の記録\n\n- セルの kanban や wiki は指さない\n\n## 1. 設計書 §6 のオーナーの答え\n\n| 2026-11-05 | ゲートの記録の前の版 |\n' >"$tmp/r/docs/decisions.md"
expect ok 'docs/decisions.md の凍結した表（「## 1.」から後ろ）の「ゲート」「版」は通る'


# 句読点のすぐ後ろの半角の空白（\p{Han} が句読点も含むため、和字と英字の間に空白を足す置き換えが作る形）: 落ち、決まりの形は通る
for f in docs/x.md package/x.md internal/x.go tests/x.sh model/X.tla README.md; do
  setup; mkdir -p "$tmp/r/$(dirname "$f")"; echo '課題の kanban、 gate、 バージョンを読む' >"$tmp/r/$f"; expect fail "$f の「、 gate」で落ちる"
done
for w in '止める。 gate が読む' 'kanban・ gate の語' '本文。  gate が読む' 'kanban・  gate の語' '"課題を、 kanban に書く"'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect fail "「$w」で落ちる"
done
for w in 'kanban、gate。バージョン・PR' '開く。 https://example.com/x を読む' '| 1 | 終わり。 | 次 |' '決める。 → 2026-11-20 に決めた' '本文が無ければ `` 課題 `<id>`。 ``' '行の末尾の空白。 ' '行の末尾の空白が 2 つ。  '; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect ok "「$w」は通る"
done
# 句読点の前の空白（\p{Katakana} も句読点を含むため、英字とカタカナの間に空白を足す置き換えが作る「kanban ・」）
for w in 'kanban ・バージョン' 'バージョン・gate ・PR' 'gate 、kanban' '番号は 2 。次へ' 'uses kanban 、gate'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect fail "「$w」で落ちる"
done
for w in '仕様 https://example.com/x 、実装 https://example.com/y 。' '表が 4 列（# ・何を決めたか）' '頭は andon gate: 。記録'; do
  setup; echo "$w" >"$tmp/r/docs/x.md"; expect ok "「$w」は通る"
done
setup; mkdir -p "$tmp/r/internal/triage"; echo 'fmt.Fprintf(&b, "閉じた %d ・ 直す %d ・ 次回 %d\n",' >"$tmp/r/internal/triage/apply.go"
expect ok 'triage の要約の行（空白を挟む「・」）は通る'
setup; echo '閉じた 1 ・ 直す 1 ・ 次回 1' >"$tmp/r/docs/x.md"; expect fail 'triage の外の空白を挟む「・」で落ちる'
setup; mkdir -p "$tmp/r/docs"; echo 'PATH にあるか。 WSL で打つ' >"$tmp/r/docs/x.md"; expect fail '例外の文でも、例外のファイルの外では落ちる'

# 範囲がファイル1つの根（-H の無い grep はファイル名を出さず、当たりと例外が素通りする）: 段ごとに落ち、例外は効く
solo() { # solo <根からのパス> <中身>。根にそのファイル1つだけを置く
  rm -rf "$tmp/r"; mkdir -p "$tmp/r/$(dirname "$1")"; printf '%s\n' "$2" >"$tmp/r/$1"
}
solo README.md '費用を数える'; expect fail 'README.md だけの根の「費用」で落ちる（語の段）'
solo CHANGELOG.md "$(printf '# CHANGELOG\n\n## 未リリース\n- 費用を数える\n\n## v0.0.1\n- x')"; expect fail 'CHANGELOG.md だけの根の未リリースの「費用」で落ちる（語の段）'
solo CHANGELOG.md "$(printf '# CHANGELOG\n\n## 未リリース\n\n## v0.0.1\n- 費用を数えた')"; expect ok 'CHANGELOG.md だけの根のバージョンの節の「費用」は通る（語の段）'
solo README.md '人がマージする'; expect fail 'README.md だけの根の「人がマージする」で落ちる（役の「人」の段）'
solo docs/adr/0001-x.md '人がマージする'; expect fail 'docs/adr/ のファイル1つだけの根の「人がマージする」で落ちる（役の「人」の段）'
solo docs/adr/0036-owner-not-hito.md '   | 人の判断待ち | オーナーの判断待ち |'; expect ok 'ADR-0036 だけの根の語の表の「今」の列は通る（役の「人」の段）'
solo docs/design.md '人がマージする'; expect fail 'docs/design.md だけの根の「人がマージする」で落ちる（役の「人」の段）'
solo docs/design.md 'freee の KPI「人の介在率」'; expect ok 'docs/design.md だけの根の引用は通る（役の「人」の段）'
solo docs/design.md 'gate が読む。版を上げる'; expect fail 'docs/design.md だけの根の「版」で落ちる（「台帳」「ゲート」「版」の段）'
solo docs/design.md '日本語の呼び名を「台帳」のままにした。2006 版では'; expect ok 'docs/design.md だけの根の「」の中と「2006 版」は通る（「台帳」「ゲート」「版」の段）'
solo docs/adr/0001-x.md '台帳に書く'; expect fail 'docs/adr/0001 のファイル1つだけの根の「台帳」で落ちる（「台帳」「ゲート」「版」の段）'
solo README.md '課題の kanban、 gate を読む'; expect fail 'README.md だけの根の「、 gate」で落ちる（句読点の段）'
solo README.md '開く。 https://example.com/x を読む'; expect ok 'README.md だけの根の URL の前の空白は通る（句読点の段）'
# 範囲が空の根: grep -r にファイルを渡さないと根の全体を見る。範囲の外（.github/）は見ない
solo .github/x.md '費用。 gate、 kanban'; expect ok '範囲が空の根では、範囲の外（.github/）の語と空白を見ない'
# 段の grep -r はどれも -H を持つ（新しい段の抜けを、範囲がファイル1つの例を書く前に止める）。当たる行が段の数だけあることも確かめる
runs=$(grep -nE '\bgrep -r[A-Za-z]*' tests/words.sh | grep -vE '^[0-9]+:[[:space:]]*#' || true)   # 注釈の行は外す
n=$(printf '%s' "$runs" | grep -c . || true)
if [ "$n" -ge 4 ]; then echo "ok   words.sh の grep -r を $n 本読んだ"; else echo "FAIL words.sh の grep -r が $n 本しか読めない（段は 4 つ）"; fail=1; fi
noh=$(printf '%s\n' "$runs" | grep -vE 'grep -r[A-Za-z]*H|[[:space:]](-H|--with-filename)([[:space:]]|$)' || true)
if [ -z "$noh" ]; then echo "ok   words.sh の段の grep -r はどれも -H を持つ"; else echo "FAIL words.sh の段の grep -r に -H が無い: $noh"; fail=1; fi

exit "$fail"
