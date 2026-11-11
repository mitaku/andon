# ADR-0020: auto memory を気づきの受け皿にして wiki へ蒸留する（仕様・設計。2026-11-11。Chief Engineer の課題 A-0143）

- 状態: **提案（2026-11-11）**。人が 2026-11-11 に「決めた」と言った（Chief Engineer の対話 A-0143）。人が PR をマージした日が決定の日（[decisions/2026-11-11-adr-0020-auto-memory-as-intake.md](../decisions/2026-11-11-adr-0020-auto-memory-as-intake.md)）。実装は「依頼書に切る案」のとおり別の課題。
- 主題: ハーネスの自動の記憶（Claude Code の auto memory・Codex の memories）を入れ直すか、入れるならどの役に・どこへ置き・誰がいつ蒸留するか、wiki との線引き。[ADR-0004](0004-project-layout.md) 決めたこと 2（memory は置かない）を覆すかどうかを含む。
- 語は [design.md](../design.md) §0.1。
- 入力: 依頼書 A-0143（人の発言 2026-11-11 と Chief of Staff の経緯）、ADR-0004 決めたこと 2、[ADR-0016](0016-cell-wiki-llm-wiki.md)、[ADR-0007](0007-interim-gap-inventory.md) C5・R9、`package/settings.json`、`package/skills/chief-of-staff/SKILL.md` 規約 9・§11、`package/compose.yaml`、セルの wiki。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-11）: 「Wiki と AutoMemory の棲み分けを少し考えていました。/workspace 配下に memory を置けば永続化もされるし、定期的に蒸留するならいいのかもな？と思っています」。

言い直し（人が同意 2026-11-11）。今は memory を置かないと決めてあり（ADR-0004 決めたこと 2）、auto memory は managed settings で止めてある。残るのは、担当が報告の `## Remember` に書いたものと、Chief of Staff が対話から手で wiki に写したものだけで、セッションの中でエージェントが気づいたことを拾う口が細い。案は、auto memory を入れ直して置き場を `/workspace` の下にし（セルの repo に入り、人から見え、永続化される）、定期的に蒸留して wiki・skill・`AGENTS.md` へ出し、memory 自体は痩せた状態に保つこと。これで ADR-0004 が挙げた2つの理由（見えない所で育つ・自動で載って肥大する）に答えられるかを詰める。決めるのは、入れ直すか・どの役に・置き場と git・蒸留を誰がいつどこへ・Codex を揃えるか。HOME の下の設定ファイルを誰が書くかは、memory の置き場に関わる分だけ扱う。

## 前提（同意済み。番号）

1. **何もしないとどうなるか（2026-11-11 同意）。** 今の拾い口（担当の報告の `## Remember` と、Chief of Staff が手で wiki に写すもの）だけでは、報告に書かれなかった担当の気づき・Chief Engineer とレビュアーの気づき・Chief of Staff が写しそびれたものがセッションと一緒に消える。取りこぼしは実際に起きていて、放っておけない。
2. **3つの家は変えない（2026-11-11 同意）。** ADR-0004 決めたこと 2 の分け方（続きは台帳と worktree、決定は ADR と `docs/decisions/`、学び・事実はアウターループの出口と wiki）は保つ。auto memory を入れ直しても、担うのは学び・事実の拾い口だけ。続きと決定を memory に持たせない。
3. **memory は受け皿で、最後の家ではない（2026-11-11 同意）。** 蒸留で wiki・skill・`AGENTS.md`・テストへ出すか捨て、出したものは memory から消して痩せた状態に戻す。長く残る知識の正は wiki と repo のまま。
4. **見える所に置けるときだけ入れ直す（2026-11-11 同意）。** 記憶は、人とほかのエージェントから見える所（`/workspace` の下）に置けるときだけ入れ直す。見えない置き場（HOME の下の volume）にしか置けない記憶は今までどおり切る。Codex にも同じ条件を当てる。
5. **配り方と重さ（2026-11-11 同意）。** 届く先はパッケージ（managed settings と skill）で、版上げで全セルへ広がる。ADR-0004 の「memory は置かない」を覆すので新しい ADR にし、実装のマージは人がする（レベル C）。まずこのセルで実測し、効かなければ切る設定に戻す。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-11 auto memory は全部の役に入れる（案 B。推奨案は覆された）。** Chief of Staff・Chief Engineer・担当・レビュアーのどの役のセッションでも auto memory を入れる。
   理由（人の選択）: 取りこぼし（前提 1）は担当・Chief Engineer・レビュアーの気づきにもあり、一部の役だけでは残る。
   捨てた案: A 最小（Chief of Staff だけ。Chief Engineer の推奨——蒸留が回るかを先に確かめてから広げる）／D 選んで広げる（人と話す役だけ）／C 横から（auto memory は切ったまま、Chief Engineer・レビュアーの報告にも `## Remember` を足す）——報告に書かれなかったものが消える根が残る。
   案 B が持ち込むもの（下の決めたことで備える）: 並んで走るセッションが同じ memory に書く・ある担当が書いた注意書きが次の担当に自動で載る・担当が自分の worktree の外に書く。
2. **2026-11-11 置き場は `/workspace/memory/`。git では追わない（推奨案は覆された）。** セルの repo の直下に置き、セルの `.gitignore` に入れる。人とほかのエージェントからは見える（前提 4）。
   理由（人の選択）: memory は蒸留されるまでの受け皿（前提 3）で、長く残すものは蒸留で wiki と repo へ出る。
   捨てた案: 同じ場所で git で追う（Chief Engineer の推奨——セルを建て直しても残り、蒸留で消したものが履歴に残る）／`/workspace/wiki/inbox/`——wiki を書くのは Chief of Staff だけという線（ADR-0016 前提 3・規約 9）が崩れる。
   受け入れること: セルを建て直すと蒸留前の memory は消える。蒸留で消したものの履歴は git に残らない。

3. **2026-11-11 蒸留はトリアージに乗せる。** `andon triage collect` が `/workspace/memory/` の1件を項目（`kind: memory`）として拾い、トリアージの担当が実物で確かめて仕分け、Chief of Staff が反映する（wiki 行きは Chief of Staff が ingest、repo 行きは直す課題、要らないものは捨てる）。仕分けの済んだ memory のファイルは反映のときに消す（前提 3）。wiki を書くのは Chief of Staff のまま（ADR-0016・規約 9）。
   理由: 報告の `## Remember`（`kind: remember`）と同じ道で、新しい役が要らない。確かめてから仕分ける役が、理由の当てはまらなくなった注意書きを落とせる。
   捨てた案: Chief of Staff が自分で（wiki の lint と同じ所で読む）——全部の役が書くので量が多く、Chief of Staff の「深い調査はしない」に当たる／蒸留の担当を新しく作る——wiki の書き手の線を変える（ADR-0016 が捨てた案 C と同じ形）。

4. **2026-11-11 蒸留の周期は週1のトリアージ＋量の線。** 既定は今のルーチン `triage`（週1）に乗る。加えて、`/workspace/memory/` の件数が線（仮に 30 件。実測で直す）を超えたら dispatcher が Chief of Staff に届け、Chief of Staff が臨時のトリアージを足す（同時に1つの決まりは保つ）。
   理由: 普段は今ある周期に乗るだけで、溜まった週だけ早く回る。蒸留までの間、書かれた注意書きは全部の役に自動で載り続けるので、量で上を切る。
   捨てた案: 週1だけ——活発な週に最長1週間載り続ける／毎日——空振りが多い／課題の finish のたび——担当が1つ余計に起き、誰が書いた memory かを見分ける仕組みが要る。

5. **2026-11-11 wiki へ出す memory だけ `wiki/raw/memory/` に写す。** wiki 行きは、言葉のまま `wiki/raw/memory/<日付>-<短い名>.md` に写してから ingest する（`raw/` の4種目。ADR-0016 決めたこと 2 に足す）。repo 行きは直す課題の依頼書に引用する。捨てたものはトリアージの記録（項目の本文と仕分けの理由）に残る。そのうえで memory のファイルを消す。
   理由: wiki の出典が wiki の中だけで完結し（ADR-0016 決めたこと 2 の理由）、捨てたものの跡はすでにあるトリアージの記録で足りる。
   捨てた案: 全部を `raw/` に写す——`raw/` が要らない注意書きで太る／`raw/` に写さずトリアージの記録を指す——台帳はセルの寿命なので、建て直すと wiki の出典が切れる。

6. **2026-11-11 memory への書き方は環境メモに短く書く。** 全部の役が毎セッション読むパッケージの環境メモ（`package/environment.md`）に数行: memory は蒸留までの受け皿で、週1で仕分けられて消える／書くのは、確かめた事実と、起きた条件と理由のついた学び／続き・決定・秘密は書かない（続きは台帳、決定は文書の PR）／memory に書いても、報告の `## Remember` は今までどおり書く。
   理由: 一か所で全役に届く。注意書きに条件と理由があれば、読む側が自分の場面に当てはまるかを判断でき、トリアージの担当も確かめやすい（慎重さの積み重なりへの備えを、書く側と仕分ける側の両方に置く）。
   捨てた案: 何も指示しない（ハーネスの既定に任せ、質は蒸留で落とす）——理由の無い注意書きが載り続ける／役ごとの skill に書く——直す所が4つに増える。

7. **2026-11-11 Codex の記憶は切ったまま。** ADR-0007 R9（A-0072。boot で `features.memories`・`memories.generate_memories`・`memories.use_memories` を無効にする）はそのまま進める。Codex で動く役の気づきは報告の `## Remember` で拾う。
   理由: 前提 4。置き場は `$CODEX_HOME` の下で、そこだけを `/workspace` へ移す設定が docs に無く、中身の形も不明で、トリアージが1件ずつ拾って消す形に乗る見込みが立たない。
   捨てた案: 調べてから決める（形と置き場をセルで実測する課題を先に）／`~/.codex` の下のまま入れる——前提 4 を Codex だけ外すことになる。
   考え直す条件: Codex が記憶の置き場を変えられるようになったとき。

8. **2026-11-11 入れる前にセルで実測し、許可は要るときだけ足す。** 調べる課題を1つ先に回す（本物の HOME に書かない形で）。確かめるのは docs に無い3つ: (a) `autoMemoryDirectory` に `/workspace/memory` を書いたとき、repo ごとのサブフォルダが切られるか、全セッションが同じフォルダを共有するか／(b) 並んで走るセッションが同じ `MEMORY.md` に同時に書いたとき何が起きるか／(c) cwd が worktree の担当が `/workspace/memory/` に書くとき、許可の確認が出るか。
   - (c) で確認が出ると分かったときだけ、managed settings の `allow` に `/workspace/memory/**` への書き込みを足す。人のファイルを守る今の `deny` は変えない。
   - (a)・(b) はどちらの結果でも設計を変えない（memory は1件1ファイルで、トリアージはフォルダのファイルを直接拾う。目録の行が欠けても蒸留には効かない）。ただし memory のファイル自体が壊れる・失われると分かったら、入れる前に人へ戻す。
   理由: 権限を広げるのは要ると確かめたときだけにする。
   捨てた案: 実測せずに許可を足して入れる——要らないかもしれない許可を広げ、壊れ方を本番の担当で知る／許可は足さず、確認が出る役は切る——決めたこと 1 が設定の都合で崩れる。
9. **2026-11-11 設定の形と、覆す文（問わずに推奨案として書き、最後の確かめで人が認めた）。**
   - managed settings（`package/settings.json`）: `autoMemoryEnabled: true`、`autoMemoryDirectory: "/workspace/memory"`、`env` の `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` を外す。memory の設定を書くのはこの1か所だけで、セルの HOME の下の設定ファイル（`~/.claude/settings.json`・`~/.claude/CLAUDE.md`）には誰も書き足さない。
   - セルの repo: `/workspace/memory/` を `.gitignore` に入れる（`andon init` の雛形と、今あるセルは版上げの手順で）。
   - ADR-0004 決めたこと 2 の「memory は置かない」「ハーネスの auto memory は managed で OFF」と前提 7 に「→ 覆された（ADR-0020）」を足す（上書きしない）。3つの家の表は保つ（前提 2）。`projects/<slug>/` の下に `memory/` を作らない線も保つ（置き場はセルの直下）。
   - chief-of-staff skill の規約 9「ハーネス内蔵のメモリは使わない」を、「memory は蒸留までの受け皿。残すものはトリアージを通して wiki と repo へ」に書き直す。

## やらないこと（範囲の外。理由）

- memory を長く残す知識の家にすること（前提 3）。毎セッション自動で載せたい事実が出たら、それは `AGENTS.md`・環境メモ・skill の題として別に扱う。
- memory を git で追うこと・セルをまたいで持ち運ぶこと（決めたこと 2）。
- Codex の記憶を入れること（決めたこと 7）。
- 蒸留の専用の役・memory のための command（`andon memory …`）。拾うのは `andon triage collect`、反映は `andon triage apply`（決めたこと 3）。
- 役ごとに入れる／切るを分ける仕組み（決めたこと 1 で全部の役にしたので要らない）。
- サブエージェントの memory（agent の frontmatter の `memory:`）。メインの会話の memory はサブエージェントに載らず、サブエージェントは調べものだけに使う（オーナーの志向。wiki `entities/owner.md`）。
- 報告の `## Remember` をやめること（決めたこと 6。Codex の役と、検収で読む報告のために残す）。
- 続き・決定を memory に持たせること（前提 2）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に。既存の継ぎ目を優先し、できるだけ高く、できるだけ少なく）

継ぎ目は `tests/package.sh`（settings と skill・環境メモの本文）・`andon triage collect`／`apply` の今あるテスト・`tests/andon-init.sh`（雛形）、それにセルでの実測。

1. **実測（決めたこと 8）**: 調べる課題の報告に (a)・(b)・(c) の結果が、打った手順と一緒に書いてある。本物の HOME（セルの `~/.claude`・`~/.codex`）に書いていない。
2. **settings**: `package/settings.json` が `autoMemoryEnabled == true`・`autoMemoryDirectory == "/workspace/memory"` で、`env` に `CLAUDE_CODE_DISABLE_AUTO_MEMORY` が無い。(c) で要ると分かったときだけ `allow` に `/workspace/memory/**` の書き込みがある。`tests/package.sh` の auto memory の検査を新しい形に合わせる。
3. **collect**: `/workspace/memory/` に memory のファイルが2件あるとき、`andon triage collect` が `kind: memory` の項目を2つ返す（`text` に本文、`key` はファイルごと）。`MEMORY.md`（目録）は項目にしない。フォルダが無い・空のときは項目を返さず落ちない。
4. **apply**: `kind: memory` の項目を仕分けた decisions を `andon triage apply` すると、仕分けの済んだ memory のファイルが消え、目録の行も消える。`keep` のものは残る。記録（`library/triage/`）に本文と理由が残る。
5. **量の線**: memory の件数が線を超えたとき、dispatcher が Chief of Staff に1度届ける（同じ超え方で繰り返し届けない）。
6. **文書と指示**: 環境メモに決めたこと 6 の数行がある。chief-of-staff skill の規約 9・§10（memory の項目の反映: wiki 行きは `raw/memory/` に写して ingest）・§11（`raw/` の4種目）と triage skill（`kind: memory` の確かめ方）が直っている。ADR-0004・ADR-0016 に「→ ADR-0020」がある。
7. **雛形**: `andon init` の `.gitignore` に `memory/` がある。
8. **セルでの実測**（マージ後。Chief of Staff）: このセルで担当を1つ起こし、`/workspace/memory/` に書かれること・次のセッションに載ること・トリアージで拾われて消えることを1周見る。

## 価値の仮説

人が同意（2026-11-11）。数（5 件・8 割・2 件・4 週間）は Chief Engineer の見立てで、観察のときに実数と一緒に見直す。

1. 誰の何が変わるか: オーナーと Chief of Staff。エージェントの気づきが、報告に書かれなくても拾われ、仕分けを通って wiki か repo に出る。
2. 見る信号: オーナーの目視と、台帳の数（トリアージの記録の `kind: memory` の項目数と仕分けの内訳、`wiki/log.md` の `raw/memory/` 由来の ingest の数）。
3. 見る日: 2026-12-09。
4. 誰が要るか: Chief of Staff（トリアージを回す）とオーナー（要約を見る）。
5. 効いたとどう分かるか: 見る日までに、memory 由来の項目のうち wiki か repo へ出たものが 5 件以上あり、週1の蒸留のあと memory の件数が線（30 件）の下に戻っている。
6. 何があれば止めるか: 仕分けの 8 割以上が「捨てる」／memory の注意書きが元で、担当が要らない確認や控えをした例が 2 件出た／蒸留が 2 週続けて回らず線を超えたまま。どれかに当たれば、切る設定に戻すかを人が決める。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| 案 | 中身 | 受け入れ条件 | 依存 |
|---|---|---|---|
| M0 | A-0072（PR #183。Codex の記憶を切る）を止めから外して進める（決めたこと 7） | — | — |
| M1 | 調べる課題: auto memory の3つの実測（決めたこと 8。本物の HOME に書かない。結果は報告だけ） | 1 | — |
| M2 | `andon triage collect` に `kind: memory`、`andon triage apply` で仕分け済みの memory を消す＋ triage skill | 3・4 | M1（フォルダの形） |
| M3 | settings を入れる向きに（＋要るときだけ `allow`）・環境メモの数行・chief-of-staff skill 規約 9・§10・§11・`andon init` の `.gitignore`・ADR-0004 と ADR-0016 の指し直し・`tests/package.sh`（レベル C） | 2・6・7 | M1・M2（蒸留の道が先） |
| M4 | 量の線: dispatcher が memory の件数を見て Chief of Staff に届ける | 5 | M3 |
| — | セルでの実測1周は課題にせず、Chief of Staff が M3 のマージと版上げのあとにする | 8 | M3 |
| M5 | 観察: 見る日に価値の仮説を見る | — | M3 のマージ |

## 開いている問い（残った問い。決断の種類つき）

1. 量の線の数（仮に 30 件）を実測のあとでいくつにするか（`line`）。
2. 担当へ載る注意書きが元の控えが実際に出たとき、役ごとに切る口を作るか（決めたこと 1 を選び直す。`line`。価値の仮説の撤退の条件で拾う）。

## 出典（読んだもの・捨てた案の出所）

- 人が「決めた」と言った日: 2026-11-11（最後の確かめ「共有できたか」への答え）。反論のサブエージェントは走らせていない（外向きの面が無く、切る設定に戻せば元に戻る）。
- 人の発言 2026-11-11（Chief of Staff との対話。依頼書 A-0143 の経緯）・2026-11-06（「memory は不要。アウターループで wiki などに出す」。ADR-0004 の問題の段）。
- Claude Code の公式ドキュメント（memory・settings・env-vars・sub-agents。2026-11-11 に確認）: 既定の置き場 `~/.claude/projects/<project>/memory/`（同じ repo の worktree は共有）、`autoMemoryDirectory`（絶対パスか `~/` 始まり。policy の層にも書ける）、`MEMORY.md` の先頭 200 行か 25KB が毎セッション載る、書くのはモデル自身、メインの会話の memory はサブエージェントに載らない。 https://code.claude.com/docs/en/memory ・ https://code.claude.com/docs/en/settings ・ https://code.claude.com/docs/en/env-vars ・ https://code.claude.com/docs/en/sub-agents
- Codex の memories のドキュメント（2026-11-11 に確認）: 置き場は `$CODEX_HOME` の下（`memories_v2/`）、会話が静かになってから背景で書く。 https://learn.chatgpt.com/docs/customization/memories
- Fable 5.2 の手引き「慎重さが積み重なるのを防ぐ」「蓄積したルール、メモ、記憶」（2026-11-11 に確認）: 注意書きは memory・メモ・指示ファイルに溜まり、不要な承認や控えを増やす。理由が当てはまらなくなったものを消す。 https://platform.claude.com/docs/ja/build-with-claude/prompt-engineering/prompting-claude-fable-5-2
- `package/compose.yaml`: `~/.claude`・`~/.codex` は named volume、`/workspace` はセルの repo の bind mount。
