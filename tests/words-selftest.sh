#!/usr/bin/env bash
# tests/words.sh が「今」の語で落ち、例外では落ちないこと（ADR-0013 受け入れ条件 3）。
# 役を指す「人」で落ち、役を指さない「人」では落ちないこと（ADR-0036 受け入れ条件 2）。
# 一時の根に小さな写しを作り、words.sh にその根を渡す。
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail=0
adr=docs/adr/0013-short-role-names-and-plain-japanese.md

# setup: ADR-0013 の写しと、版の節のある CHANGELOG だけの根
setup() {
  rm -rf "$tmp/r"; mkdir -p "$tmp/r/docs/adr"
  cp "$adr" "$tmp/r/$adr"
  printf '# CHANGELOG\n\n## 未リリース\n\n## v0.0.1\n- 費用を数えた\n' >"$tmp/r/CHANGELOG.md"
}
expect() { # expect ok|fail <説明>
  if bash tests/words.sh "$tmp/r" >/dev/null 2>&1; then got=ok; else got=fail; fi
  if [ "$got" = "$1" ]; then echo "ok   $2"; else echo "FAIL $2（$got）"; fail=1; fi
}

setup; expect ok '写し（ADR-0013 の引用・語の表・語を挙げた所、CHANGELOG の版の節）は通る'
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
setup; echo '人がマージする' >"$tmp/r/docs/x.md"; expect ok '替えていない範囲（docs/）の役の「人」はまだ見ない'
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
expect ok 'README.md の ADR の一覧の行（ADR の題の写し）は通る'
setup; echo '- [docs/contract.md](docs/contract.md) — 人の手直し' >"$tmp/r/README.md"; expect fail 'README.md の一覧でない行の「人の手直し」で落ちる'
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
setup; mkdir -p "$tmp/r/internal/process"; echo '台帳の CLI' >"$tmp/r/internal/process/x.go"; expect ok '替えていない範囲（internal/process/）の「台帳」はまだ見ない'
setup; echo 'gate が kanban の課題を読む。版を上げる' >"$tmp/r/docs/design.md"; expect fail '替えた範囲（docs/design.md）の「版」で落ちる'
setup; echo '日本語の呼び名を「台帳」のままにした。2006 版では' >"$tmp/r/docs/design.md"; expect ok 'docs/design.md の「」の中と「2006 版」は通る'

exit "$fail"
