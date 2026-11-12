# セルの wiki を LLM Wiki の手法に乗せる（仕様・設計。2026-11-10・Chief Engineer の課題 A-0084）

決定（2026-11-10）

## 問題（人の言葉で。言い直しの段落）

いまのセルの wiki は、Chief of Staff が「学んだと思ったこと」を思いついた時点で1件1ファイルで足すだけの置き場になっている。2026-11-10 の5件がその形で、うち3件は wiki ではなく repo（skill・用語集）に置くべき手順で、書く時期も skill §11 の決まりからずれていた。困りごとは「wiki が学びの貼り紙になっていて、知識として育たず、読まれもしない」こと。人の向きは Karpathy の LLM Wiki の手法に乗せること。つまり、元の資料（記録・報告・指摘・人の発言）の層と wiki の層を分け、LLM が資料を取り込んで（ingest）実体・概念ごとのページを育て、矛盾を lint で拾い、質問に答える（query）形にする。この文書で決めるのは「セルの wiki の形と運用（何が元資料か・ページの単位・誰がいつ取り込むか・lint・index と log）」で、実装は別の課題に切る（人が言い直しを認めた 2026-11-10）。

## 前提（同意済み。番号）

1. **何もしないとどうなるか（2026-11-10 同意）。** 今の skill §11 のまま続けると、1件1ファイルが日付順に積み上がり `index.md` が長くなるだけで、ページ同士の矛盾・重複は拾われず、Chief of Staff は「学び」と「repo に置くべき手順」を wiki に混ぜて捨て続け、Chief Engineer の「読む」も index を引くだけで、実質 wiki は使われない。
2. **元資料（raw sources）の層は wiki の中に集める（2026-11-10 同意。推奨案は覆された）。** 推奨案は「新しく作らず、台帳の記録（report・findings・corrections・ゲートの記録・問いと答え）と repo の ADR・decisions/ を出典として指す」だったが、人は原典どおり不変の元資料の層を wiki の横に持ち、記録・人の発言をそこへ写してから取り込む形を選んだ。何を・誰が・いつ写すかは「決めたこと」で決める。
3. **書き手はエージェントだけ（2026-11-10 同意）。** wiki の層を書くのはエージェントで、人は wiki のページを書かない（原典の "You never (or rarely) write the wiki yourself"）。人がするのは元資料を持ち込むこと・問うこと・ページを読んで直させること。どのエージェントが書くか（今は Chief of Staff だけ）は「決めたこと」で決める。
4. **境界は保つ（2026-11-10 同意）。** wiki に置くのは「セルを横断する運用の学び・確かめた事実・オーナーの思考と志向」。製品に効くもの（仕様・作法・テスト）は repo、決定は ADR と `decisions/` が正で、wiki はそれを指すだけで中身を写さない（decisions.md D12・問い21・skill §11）。LLM Wiki の形に乗せてもこの境界は変えない。
5. **配り方と規模（2026-11-10 同意）。** 届く先はパッケージ（Chief of Staff の skill §11 と `init/` の wiki の雛形）で、版上げで全セルへ広がる（横展）。ラインの変更なのでマージは人（レベル C）。規模は原典の「約100元資料・数百ページまでは index だけで足りる」より小さいと見て、検索ツール・embedding・Obsidian のような閲覧の道具は今回入れない。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-10 形は原典どおり（案 B）。** `wiki/raw/`（不変の元資料）＋ wiki の層を実体・概念ごとに育つページで持ち、`index.md`・`log.md` を置き、ingest・query・lint を Chief of Staff が回す。schema（原典の CLAUDE.md に当たるもの）は Chief of Staff の skill §11。
   理由: 人の向き「LLM Wiki の手法に乗っとる」を満たす最小で、書き手は規約9のまま。あとで書き手だけ担当に移せる（案 C）。
   捨てた案: A 最小（今の §11 に raw/・log.md・lint を足すだけ）——ページが実体・概念ごとに育たず前提1の困りごとが残る／C 横から（ingest・lint を wiki の担当がする）——セルの repo を相手にする担当の形・プロファイルが新しく要り、規模（前提5）に対して部品が多い。
2. **2026-11-10 元資料（`wiki/raw/`）は3種。** (1) 人の発言の写し（Chief of Staff が対話から、言葉のまま・日付つき）、(2) 人が持ち込む外の資料（記事・資料）、(3) 課題を finish したときの記録の写し（人向けの報告 `report.md` と人の手直し `corrections.json`。finish 後は不変）。指摘（findings）・ゲートの記録は写さず、必要なら台帳を指す。元資料は書いたら変えない（原典の immutable）。→ ADR-0020（4種目: トリアージで wiki 行きになった memory の写し `raw/memory/`。[ADR-0020](0020-auto-memory-as-intake.md) 決めたこと 5）
   理由: 今は記録に残っていない人の発言が残り、台帳がセルの寿命（ADR-0005 決めたこと 5）でも wiki がセルの repo だけで完結する。
   捨てた案: 発言と外の資料だけ（台帳を指すだけ）——セルを建て直すと出典が消える／library/ を丸ごと写す——raw/ がラインのノイズで太る。
3. **2026-11-10 wiki の層のページは4種をフォルダで分ける。** `sources/`（元資料に1対1の要約）・`entities/`（実体: 役・部品・案件・オーナー。例 `entities/owner.md`・`entities/dispatcher.md`）・`topics/`（概念・運用の題。例「rebase のあとのレビューの頼み直し」）・`overview.md`（まとめ1枚）。原典の comparisons・synthesis は topics と overview に吸う。
   理由: 実体と概念が育つ場所が決まり、オーナーの志向は `entities/owner.md` に集まる。
   捨てた案: 原典の6種そのまま——小さい wiki に空の種類ができる／フォルダで分けず front matter の kind だけ——ファイル一覧で形が見えない。
4. **2026-11-10 ingest は、元資料を `raw/` に置いたその場で1件ずつ（原典どおり）。** 置く時＝人の発言を写したとき・人が資料を置いて「取り込んで」と言ったとき・finish で記録を写したとき。取り込み＝`sources/` に要約ページを書き、関わる `entities/`・`topics/` を直し、`index.md` と `log.md` に足す。
   理由: 元資料と wiki がずれない。
   捨てた案: 溜めてまとめて（人が頼んだときか週1）——未取り込みが溜まり index が古くなる／finish のときだけ（今の §11）——人の発言と外の資料が次の finish まで待つ。
5. **2026-11-10 `index.md` と `log.md` の2つを置く（§11 の「log.md は作らない」を覆す）。** `index.md` は中身の目録（ページごとに1行: リンク・1行の要約。種類ごとの節。ingest のたびに更新。query は最初にこれを読む）。`log.md` は時系列の追記だけの記録（ingest・query・lint の1件に1項。先頭を `## [YYYY-MM-DD] ingest | <題>` の形に揃え、`grep` で引ける）。
   理由: 案 B（決めたこと 1）に含む。git の履歴は commit の粒で、ingest・query・lint の種類と触ったページが見えない。log.md は lint の回数（決めたこと 6）を数える元にもなる。
   捨てた案: log.md は作らず git の履歴で代える（今の §11）。
6. **2026-11-10 lint は Chief of Staff が ingest を約10件するごとと、人が頼んだとき。** 見るもの: ページ間の矛盾・新しい元資料に覆された古い主張・孤立ページ（入るリンクが無い）・名前は出るのにページが無い重要な概念・リンク漏れ。矛盾は古いページを上書きせず「→ 覆された（<新しいページ>）」を足す（今の §11 の lint の行を保つ）。定期の課題は起動しない（§11 の線を保つ）。振り返りのルーチンができたらそこへ移す。
   捨てた案: 定期の課題（ルーチン）——ルーチンの形がまだ無い／人が頼んだときだけ——頼まなければ矛盾が残る。
7. **2026-11-10 query は3つの口。良い答えは `topics/` に戻す。** 引くのは Chief Engineer の「読む」・Chief of Staff が課題を切るとき・人が Chief of Staff に聞いたとき。読み方は `index.md` → ページの順で、出典（元資料かページ）つきで答える。人が Chief of Staff に聞いた答えのうち次も使えるものは、Chief of Staff が `topics/` のページにして `log.md` に `query` として足す（戻すかは Chief of Staff の判断）。Chief Engineer の答えは ADR に残るので戻さない。
   理由: 原典の "good answers can be filed back into the wiki"。会話に消える答えが残る。
   捨てた案: 引くだけ——答えが会話に消える／全部戻す——小さい答えでページが増える。
8. **2026-11-10 人の発言を `raw/` に写す時は、人の答えが推奨案と違ったとき・志向や理由が見えたとき（今の §11 の時期）と、人が「残して」と言ったとき。** 写すのは言葉そのまま・日付つき（要約しない。要約は `sources/` の役）。決断そのものは問いの答えと ADR に残るので写さない。
   理由: 今の時期を保ち、人が自分で指示できる口を足す。
   捨てた案: 毎回全部写す——raw/ が会話の記録になり ingest が毎ターン起きる／「残して」のときだけ——言わなければ志向が残らない。
9. **2026-11-10 schema の置き場（決めたこと 1 の補足。問わずに推奨案として書き、最後の確かめで人が認めた）。** 骨（層・ページの種類・操作・index と log の形）は Chief of Staff の skill §11（パッケージ。前提 5）。セルに固有の慣習（そのセルで増えた実体の名・書き方の癖）は、原典の「人と LLM が一緒に育てる」に当たり、セルの `AGENTS.local.md`（環境メモが「セルに固有のこと」と指す場所）に書く。wiki の中に AI 向けの規約のファイルは置かない（1ファイル1読み手）。
10. **2026-11-10 今の5件（2026-11-10 に書いたもの）の移行。** 人の言葉の部分を `raw/` に発言の写しとして置き直し、新しい形で ingest し直す（`entities/owner.md` と `topics/` ができる）。repo に置くべき3件（Chief of Staff の起き直しの予約キーの書き直し・rebase のあとのレビューの頼み直し・英語の用語を直訳しない）は Chief of Staff が直す課題に切り、wiki のページは skill・ADR を指すだけにする。古い `<日付>-<短い名>.md` は消す（git に残る）。
   理由: 新しい形の最初の実測になる（`package/` の変更はセルで実測する決まり）。
   捨てた案: 5件を `topics/` に動かすだけ——raw/ が空で始まり手順の3件が wiki に残る／消して空から——今日の志向が消える。

## やらないこと（範囲の外。理由）

- 検索ツール（qmd など）・embedding・Obsidian などの閲覧の道具・画像の取り込み・Marp などの出力の形（前提 5。規模が小さく、原典も任意としている）。
- wiki の担当（案 C）とセルの repo を相手にする担当の形（決めたこと 1。書き手は Chief of Staff のまま）。
- lint の定期の課題（ルーチン）（決めたこと 6。ルーチンの形がまだ無い）。
- 決定・仕様・作法を wiki に写すこと（前提 4）。指摘・ゲートの記録を `raw/` に写すこと（決めたこと 2）。
- プロジェクトごとの wiki（ADR-0005 の決めたこと 1 のまま。固有のものは front matter の `project`）。
- wiki のための command（`andon wiki …`）。手順は skill §11 の本文（今の §11 の「command は無い」を保つ）。finish のときの記録の写しを `andon issue finish` が自動でするかは開いている問い 1。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に。既存の継ぎ目を優先し、できるだけ高く、できるだけ少なく）

継ぎ目は `tests/package.sh`（skill の本文の検査。ADR-0009 の受け入れ条件 3 の形）と `tests/andon-init.sh`（雛形）、それにセルでの実測。

1. **skill §11**: `package/skills/chief-of-staff/SKILL.md` §11 に、層（`raw/`・`sources/`・`entities/`・`topics/`・`overview.md`・`index.md`・`log.md`）・3つの操作（ingest・query・lint）とその時期（決めたこと 4・6・7・8）・元資料の3種（決めたこと 2）・移行の手順（決めたこと 10）がある。「1ファイル1件（`wiki/<日付>-<短い名>.md`）」「`log.md` は作らない」の行が無い。`tests/package.sh` の wiki の節の検査を新しい項に合わせる（`commit --only -- wiki` の行は保つ）。
2. **雛形**: `init/wiki/` に `index.md`（種類ごとの節）・`log.md`（空。先頭の形の説明1行）・`raw/`・`sources/`・`entities/`・`topics/` がある（空のフォルダは `.gitkeep`）。`tests/andon-init.sh` の一覧に足す。
3. **読む側**: `package/skills/chief-engineer/SKILL.md` の「読む」と `chief-of-staff/SKILL.md` §1 の query が「`index.md` → ページ」の順で読み、出典つきで答える形を指す。
4. **文書**: `docs/design.md`（§1.5 の表の wiki・§3 のセルの repo の表）と ADR-0005 の表の `wiki/` の行（「1 ファイル 1 件・日付・出典・痩せた `index.md`」）が ADR-0016 を指す（上書きせず「→ ADR-0016」を足す）。
5. **セルでの実測**（`package/` の変更はセルで実測する決まり。マージ後、Chief of Staff がする）: このセルの `wiki/` が新しい形になり、`raw/` に 2026-11-10 の発言の写し、`entities/owner.md`、`index.md` に種類ごとの節、`log.md` に `ingest` の項が5つ以上ある。古い `<日付>-<短い名>.md` が無い。repo へ出す3件の課題が台帳にある。

## 価値の仮説（P6 で形を決める）

ページが実体・概念ごとに育てば、Chief Engineer の「読む」と Chief of Staff が課題を切るときに wiki が引かれ、同じつまずき（例: rebase のあとのレビューの頼み直し）が2度目から起きなくなる。見るもの: `log.md` の `query` の項の数（月ごと）と、lint で見つかった矛盾の数。効かなければ（query が月に0のまま）、wiki に書く範囲（前提 4）か書き手（案 C）を疑う。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| 案 | 中身 | 受け入れ条件 | 依存 |
|---|---|---|---|
| K1 | skill §11 の書き直し＋ `init/wiki/` の雛形＋ `tests/package.sh`・`tests/andon-init.sh`＋ chief-engineer skill の「読む」＋ design.md・ADR-0005 の表の指し直し（1本の PR。レベル C） | 1〜4 | — |
| K2a | Chief of Staff の起き直しの予約キーの書き直しを chief-of-staff skill へ | （K1 と別。今の wiki の1件目を repo へ） | — |
| K2b | rebase のあとのレビューの頼み直しを chief-of-staff skill（か worker skill）へ | 同上（4件目） | — |
| K2c | 英語の用語を直訳しない、を ADR-0013 の線で glossary か chief-of-staff skill の人への書き方へ | 同上（5件目） | — |
| — | このセルの wiki の移行（決めたこと 10）は課題にせず、Chief of Staff が K1 のマージ後に新しい §11 の手順で自分でする（受け入れ条件 5） | 5 | K1 のマージ |
| K3 | 観察: 1か月後に `log.md` の `query`・`lint` の数を見る（価値の仮説。P6 の形が決まるまでは人の課題） | — | K1 のマージ＋移行 |

## 開いている問い（残った問い。決断の種類つき）

1. finish のときの `report.md`・`corrections.json` の `raw/` への写しを、`andon issue finish` が自動でするか（今は Chief of Staff の手。`line`。実測で手が重ければ）。
2. 振り返りのルーチンができたとき、lint をそこへ移すか（決めたこと 6 に書いた向き。`line`）。
3. 複数のセルで同じ `topics/` が育ったとき、セルをまたぐ横展をどうするか（design.md §1.7 の yokoten。今は無し。`line`）。

## 出典（読んだもの・捨てた案の出所）

- 人が「決めた」と言った日: 2026-11-10（最後の確かめ「共有できたか」への答え）。

- 人の発言 2026-11-10（Chief of Staff との対話。「Wiki の使い方間違えていそう」「まず Cell の Wiki は LLM Wiki の手法に乗っとるべきです」）。
- Karpathy "LLM Wiki"（https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f 。raw を 2026-11-10 に直接読んだ）。
- `package/skills/chief-of-staff/SKILL.md` §11・§9 規約 8・9、`/workspace/wiki/`（index と 2026-11-10 の5件）、design.md §1.4・§1.5・§4、ADR-0004 決めたこと 2・ADR-0005 の表。
