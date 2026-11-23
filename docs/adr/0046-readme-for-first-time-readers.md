# ADR-0046: README を、初めて来た読み手が README だけで判断できる形に組み直す（仕様・設計。2026-11-22）

- 状態: **決定（2026-11-22）**。オーナーが対話の中で「決めた」と言った（[decisions/2026-11-22-adr-0046-readme-for-first-time-readers.md](../decisions/2026-11-22-adr-0046-readme-for-first-time-readers.md)）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: repo の入口の文書（`README.md`）の読み手・役目・言語・構成・図・注記と、README から出すもの（ADR の一覧）、repo の description の古い語、2 つの言語の README がずれない守り方を決める。Andon のしくみは変えない。
- 語は [design.md](../design.md) §0.1。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は README の組み直しと、それに要る図・ADR の一覧の引っ越し・description の直しにする | この ADR | まだ実装していない |
| 2 | 主な読み手は日本語の開発者で、README の役目は数分で判断できるところまでにする | この ADR | まだ実装していない |
| 3 | `README.md` を英語にし、日本語は `README.ja.md` に置く | この ADR | まだ実装していない |
| 4 | 個人が学習と実験で作っている不安定なものだという注記を冒頭に置く | この ADR | まだ実装していない |
| 5 | README の節の順と、節ごとに言うことを決める | この ADR | まだ実装していない |
| 6 | 冒頭の文と注記の文言を決める | この ADR | まだ実装していない |
| 7 | 課題 1 本の流れと行燈の図を Mermaid で 1 枚、それぞれの README に置く | この ADR | まだ実装していない |
| 8 | Andon 自身の開発の流れの数は、README に載せない | この ADR | 生きている |
| 9 | ADR の一覧を `docs/adr/README.md` に移し、検査の読み先も移す | `docs/adr/README.md` | 覆された → ADR-0055（新しい ADR の PR が 1 行足すことと、検査が行と状態を見ること。置き場と読み方は生きている） |
| 10 | 2 つの README の構成がずれないことを検査 1 本で守る | この ADR | まだ実装していない |
| 11 | 英語の文では kanban・issue・owner と書く | この ADR | まだ実装していない |
| 12 | repo の description を、今の語と README の英語の語に合わせる | この ADR | まだ実装していない |
| 13 | README の本文の PR は、独立レビューが通れば自動でマージする | この ADR | 生きている |

## 問題（オーナーの言葉で。言い直しの段落）

オーナー（2026-11-21）: 「選択的DarkFactoryとしての骨子は固まってきたようにも思っています」「README自体をもう少しブラッシュアップをかけたいと思っています」。

言い直し: Andon は選択的 Dark Factory としての骨子が固まった。README は、骨子が固まる前から 58 回の変更で継ぎ足してきた文書である。初めて来た読み手が README だけを読んで、Andon が何で、どう回り、自分に合うかを判断できる形になっていない。固まった骨子が README だけで伝わるように組み直す。

読んで分かった事実（2026-11-22 の main `b890baa` と GitHub）:

- **読み手は外から来ている。** 2026-11-21 の 1 日で、repo の入口は 3,262 回、2,402 人に見られた。直近 14 日の来た元は、X（`t.co`）が 1,466 人、はてなブックマークが 683 人。スターは 11-21 と 11-22 の 2 日で 194 付いた。
- **ほとんどの読み手は README しか読まない。** 14 日で入口を見た 2,412 人に対し、`docs/design.md` は 27 人、`docs/vision.md` は 23 人、`docs/adr/0001-skills-distribution.md` は 11 人。3 つを足しても入口の約 2.5% である。
- **外の反応に、README への要望は無い。** はてなブックマークは 177 件で、コメントは 6 件。反応の中心は「人間は判断だけ」という立ち位置への感想（2 件）と、名前の由来（トヨタ生産方式のアンドン）への補足（1 件）。「どう動くか」を尋ねる声も、ほかの道具と比べる声も無かった。ブックマークのページのタイトルには repo の description がそのまま出ている。
- **README の形。** 154 行。英語の要約（34 行）が先で、日本語（112 行）が後。日本語の段のうち 45 行は ADR の一覧で、ADR を 1 本足すたびに 1 行増える。2026-11-22 の 1 日だけで、並んで走る 4 本の ADR がこの一覧の同じ所に行を足した。図も画面の写真も無い。課題が 1 本どう流れるかの例も、流れの数も無い。
- **古くなった所。** repo の description は「take tasks from a ledger」で、課題への改名（[ADR-0008](0008-issue-naming.md)）より前の語のまま。README のフックの一覧に PreToolUse が無い。文書の地図に `model/` が無い。
- **README を読む検査とコード。** `tests/adr-index.sh` は README の ADR の一覧を ADR の状態行と突き合わせる（[ADR-0033](0033-adr-inventory-and-slimming.md) 決めたこと 6）。`tests/words.sh` は 3 つの範囲の一覧に `README.md` を持つ。`tests/words-selftest.sh`・`tests/lib/docs-scope.sh`・`tests/lib/image-scope.sh`・`tests/lib/scope.sh`・`tests/ci.sh`・`tests/smoke.sh` が `README.md` を名指す。`internal/chiefengineer` の `TestDocumentedInvocations` は読む文書の一覧に `README.md` を持つが、今の README に `andon chief-engineer` の呼び出しは 1 つも無い。dispatcher が組み立てるレビュアーの依頼書は、仕様の正に「repo の README・docs/ のうち差分が触る節」を挙げる。README の相対リンクの先が在ることを見る検査は無い。
- **README を説明している文書。** [image.md](../image.md) の検査の表（`adr-index.sh` が読むものと、root の `README.md` を変えたら足す検査）、design.md §1.8（「ADR の状態行と README の ADR の一覧が揃い」）、`AGENTS.md` の ADR の行、`CHANGELOG.md` の冒頭（「README の Status」を指す）。
- **gate のレベル。** 条件表は `README*` をレベル A に、`**/*.sh` と `internal/**` をレベル B に置く。README と一緒に検査を直す PR はレベル B になり、独立レビューのあと、委譲の範囲なら自動でマージされる。
- **README の見出しへのリンク。** repo の中に、README の見出し（`README.md#…`）を指すリンクは README 自身の 2 か所だけ。
- **流れの数は出せる。** `andon measure` の閉じた 7 日（2026-11-15〜11-21。UTC）は、マージした PR が 205 本、依頼書からマージまでの中央値が 1.9 時間、自動マージ率が 42%（87 / 205）。期間が閉じる前に打つと、打つたびに数が変わる。「PR あたりのオーナーの介在回数」は 0.00 と出るが、数え方を改めると決めたところ（[ADR-0049](0049-owner-touches-by-kind.md)）で、今の数は使えない。

モデル: 調べなかった（文書と、文書を読む検査の変更で、状態を動かす流れを変えない）

## 前提（同意済み。番号）

オーナーが 2026-11-22 に 7 つとも同意した。

1. 困りごとは、README だけを読む外の読み手に骨子が伝わらないことである。オーナー自身やエージェントが読むための直しではない。
2. 主な読み手は、X とはてなブックマークから来る日本語の開発者で、コーディングエージェントを既に使っている。英語の読み手は二番目に置く。
3. README の役目は、何であるか・どう回るか・自分に合うかを数分で判断できるところまでである。立てる手順の本体は [operate.md](../operate.md) に置いたままにする。外の読み手の導入を支えることは、今回の目的にしない。
4. 何もしないと、次に話題になったときも、来た読み手の大半が英語の長い段落と ADR の一覧を見て離れる。
5. 中身の正は [design.md](../design.md) と ADR に既にある。README に足りないのは並べ方と見せ方で、新しく決める設計はほとんど無い。
6. 表示名「Andon — a one-person software factory」（オーナーの決定 2026-11-06）、「選択的 Dark Factory」の語、今の状態の書き方（実験的・1 人用・PR は受けない）は変えない。
7. X の投稿とはてなブックマークのコメントを Chief Engineer が読んでから案を作る。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-22 範囲は、README の組み直しと、それに要るものだけにする。（オーナーが決めた）** README を組み直し、課題の流れと行燈の図を 1 枚足し、ADR の一覧を README から出し、description の古い語を直す。
   理由: 読み手の大半は README しか読まないので、README の中の並べ替えだけでは「どう回るか」が文でしか伝わらない。画面の写真と動画は、セルの名前など固有の値が写るので、作り方から決める別の題になる。
   捨てた案: 広げる（画面の写真かデモの動画、social preview と topics の見直し、docs の入口のページの新設まで含める）。保つ（`README.md` の 1 ファイルの中だけを直し、ADR の一覧は残し、図は足さない）。減らす（古い所の直しと、ADR の一覧を畳むことだけ）。

2. **2026-11-22 主な読み手は日本語の開発者で、README の役目は「数分で判断できる」までにする。（オーナーが決めた。前提 1〜3 への同意）**
   理由: 「読んで分かった事実」の読み手の数。README に手順の本体まで入れると、判断に要る説明が埋もれる。

3. **2026-11-22 `README.md` を英語にし、日本語は `README.ja.md` に置く。（オーナーが決めた。Chief Engineer の推奨は、1 ファイルのまま日本語を先に置き、英語の要約を末尾に置く形だった）**
   2 つは同じ構成で、どちらも全部を書く。それぞれの冒頭（ロゴの直下）に、もう一方へのリンクを 1 行で置く。
   理由（オーナーの選択）: GitHub の慣習に合わせる。
   細部:
   - 文の正は `README.ja.md` に置く（Chief Engineer が決めた）。設計の文書が日本語で、語の正が design.md §0.1 にあるため。README を直すときは同じ PR で両方を直す。
   - 「文書は日本語から始め、必要になったら英語を足す」（オーナーの決定 2026-11-05）は覆さない。英語を足す所が、要約から README の全部に広がる。
   捨てた案: 日本語を先に置き英語の要約を末尾に置く 1 ファイル（2 つを揃える手間が無いが、最初の画面が日本語になる）。`README.md` を日本語にして英語を `README.en.md` に置く。今の並びのまま。

4. **2026-11-22 個人が学習と実験で作っているもので、不安定だという注記を置く。（オーナーが決めた。2026-11-22「個人が学習・実験で作っているものであり不安定なものであるというようなよくある注記をいれておきたい」）**
   細部（Chief Engineer が決めた）: 置き場は冒頭の説明のすぐ下で、図より前。形は GitHub の注記の書き方（`> [!NOTE]`）にする。今の「Status」の段が言っていること（実験的・1 人で使っている・約束は断りなく変わる・互換の約束が無い）はこの注記に寄せ、同じことを 2 か所に書かない。文言は決めたこと 6。

5. **2026-11-22 README の節の順と、節ごとに言うことを決める。（Chief Engineer が決めた）**
   読み手が上から読んで、何であるか → どう回るか → いつ呼ばれるか → 自分に合うか → 始め方、の順に判断の材料が出るように並べる。
   細部（2 つの README で同じ順。見出しの言葉は担当が決める）:

   | 順 | 節 | 言うこと | もとになる正 |
   |---|---|---|---|
   | 0 | ロゴ・言語の切り替え・表示名と冒頭の文 | 決めたこと 6 の文 | この ADR |
   | 1 | 注記 | 決めたこと 4・6 | この ADR |
   | 2 | どう回るか | 図（決めたこと 7）と、図の番号に合わせた 5〜6 行の説明。課題 1 本が、オーナーと Chief Engineer の対話から、Chief of Staff・担当・レビュアー・gate を通ってマージされるまで | design.md §1.3・§1.8、[process/](../process/README.md) |
   | 3 | オーナーが呼ばれるとき | 呼ばれる決断の 5 種の表。呼ばれないもの（ふだんの差分のレビュー・ふだんの PR のマージ・片付け・通知の仕分け）。誰が決断するかはプロジェクトごとに任せられ、何を作るかだけは任せられないこと。行燈は消灯・琥珀・赤の 3 つで、琥珀がオーナーが呼ばれている印であること | design.md §1.1・§1.3・§1.10 |
   | 4 | 選択的 Dark Factory と名前 | 止められるものは機械、判断が要るものは LLM、決断が要るものだけオーナー。名前はトヨタ生産方式のアンドンから。製造業のアンドン製品ではない | design.md §0・§1.1・§1.2 |
   | 5 | セル | 場所（セル）としくみ（Andon）の 2 つの面。今の「セルの考え方」の 4 行の表を、1 行 1〜2 文に縮めて置く | [ADR-0029](0029-factory-place-and-mechanism.md)・[ADR-0005](0005-cell-repo.md) |
   | 6 | 向いている読み手と今の状態 | 誰のためのものか。要るもの（Linux か WSL・Docker・bash・自分の Claude Code と Codex のアカウント）。向かない使い方（チームで使う・ホストされたサービスとして使う・互換の約束が要る）。今のバージョン。Andon は Andon のセルの中で開発しているという 1 文と、マージ済みの PR の一覧へのリンク（決めたこと 8） | 今の README の Who it is for・Status、[CHANGELOG.md](../../CHANGELOG.md) |
   | 7 | 始め方 | 今の「セルを立てる」の 3 行の command と、手順の本体（operate.md）へのリンク | operate.md |
   | 8 | 文書の地図 | 表 1 つ。今の英語の Map の表をもとに、`model/` と ADR の入口（`docs/adr/README.md`）を足す | 今の README |
   | 9 | 開発と履歴 | テストの打ち方の 2 行。前身の kanban の番号 `Y-<数>` の説明（オーナーの決定 2026-11-08 で README に書くと決めた 1 行） | 今の README |
   | 10 | 貢献・脆弱性・ライセンス | 今のまま | 今の README |

   - README から落とすもの: 「ディレクトリ」の段（`package/` の中身の列挙。正は design.md §2.2 と [image.md](../image.md) で、地図の表から指す）。ADR の一覧（決めたこと 9）。英語の「Why a cell」の 1 段落（表にする）。
   - 図（Mermaid の最初の行）は、それぞれの README の先頭から 40 行以内に置く。
   - 「Status」の段が注記に寄るので、`CHANGELOG.md` の冒頭の「README の Status」は「README の冒頭の注記」に直す。
   - 日本語の文体は、今の README と docs と同じ常体にする。
   理由: 今の README は、英語の長い段落と ADR の一覧が判断の材料より先に来る。外の反応の中心は「人間は判断だけ」という立ち位置だったので、オーナーが何を決め、何をしないかを表で具体的に見せる（順 3）。
   捨てた案:
   - 最小の案: 2 つのファイルに分け、今の段を並べ替え、ADR の一覧を出し、古い所を直す。図と「オーナーが呼ばれるとき」の表は足さない。大きさは小さく、新しく書く文も少ない。ただ「どう回るか」が文のままで、困りごと（前提 1）が残る。
   - 横からの案: README を 40 行ほどの入口にして、中身は docs の新しい概要のページに置く。2 つの言語を揃える量が減る。ただ読み手の約 97% は README の先へ進まないので、中身が読まれない。
   - ほかの道具との比べ（Warp など）を README に置く: 外の反応に比べる声が無く、相手の道具が変わると古びる。design.md §1.9 にあるので、README には置かない。

6. **2026-11-22 冒頭の文と注記の文言。（オーナーが決めた。Chief Engineer の推奨のとおり）**
   細部:
   - 英語の冒頭: 「**Andon — a one-person software factory.** AI coding agents take issues from a kanban through implementation, review and merge. The owner is called only for the decisions that only they can make.」続けて kanban の 1 文の説明を置く: 「The kanban is a set of files per project that holds issues, questions, routines and history; the `kanban` command reads and writes it.」（kanban が初めて出る公開の文書では 1 文で説明を添える、という ADR-0040 決めたこと 2 の決まり）
   - 日本語の冒頭: 「**Andon — a one-person software factory.** 一人で使うソフトウェア工場。エージェントが kanban の課題を実装・レビュー・マージまで流し、オーナーは自分にしかできない決断のときだけ呼ばれる。」続けて、今の README にある kanban の 1 文の説明（プロジェクトごとの課題・問い・ルーチン・履歴を置くファイル群で、`kanban` コマンドで読み書きする）を置く。
   - 英語の注記: 「Andon is a personal project. Its author builds it to learn and experiment, and uses it for their own work. It is unstable: commands, file formats and documents change without notice, and things break. There is no support and no compatibility promise, and pull requests are not accepted for now.」
   - 日本語の注記: 「Andon は、作者が学習と実験のために個人で作り、自分の仕事で使っているもの。不安定で、コマンド・形式・文書は断りなく変わり、壊れることもある。サポートと互換の約束は無く、今は PR も受け付けていない。」
   理由: 表示名の行と冒頭の文は、今の README の文をほぼそのまま使う（オーナーの決定 2026-11-06 の表示名と、はてなブックマークの概要に出ている文）。変えるのは英語の語（決めたこと 11）だけ。

7. **2026-11-22 課題 1 本の流れと行燈の図を、Mermaid で 1 枚、それぞれの README に置く。（Chief Engineer が決めた）**
   理由: Mermaid は GitHub がそのまま描き、文字なので差分が読め、言語ごとにラベルを書ける。repo の docs は既に Mermaid を 16 ファイルで使っている（design.md §2.7 の道具の選定）。
   細部（英語の README の図。日本語の README は同じノードと矢印で、ラベルだけ日本語にする。ラベルの語は design.md §0.1 に合わせる）:

   ```mermaid
   flowchart LR
     owner(["Owner"])
     ce["Chief Engineer<br/>spec and design with the owner"]
     cos["Chief of Staff<br/>cuts issues, starts workers"]
     worker["Worker<br/>one issue, one worktree, one PR"]
     checks["CI checks<br/>independent review"]
     gate{"gate"}
     merge["Merge and clean up"]
     lamp(("andon light"))
     retro["Retro"]

     owner -- "what to build" --> ce
     ce -- "design document" --> cos
     cos -- "brief" --> worker
     worker -- "pull request" --> checks
     checks --> gate
     gate -- "fix" --> worker
     gate -- "approve, within delegation" --> merge
     gate -- "line change or irreversible" --> lamp
     worker -. "question" .-> cos
     cos -. "only the owner can answer" .-> lamp
     lamp --> owner
     owner -- "merges" --> merge
     merge -. "records" .-> retro
     retro -. "fixes to skills, tests, rules" .-> cos

     classDef light fill:#F0A202,stroke:#1f2328,color:#1f2328
     class lamp light
   ```

   - 灯りのノードだけ琥珀で塗り、字の色も決める（GitHub の暗い配色でも読めるように）。琥珀はロゴと画面の行燈と同じ `#F0A202`（[ADR-0023](0023-sidebar-andon-look.md)）。ほかのノードは色を決めず、GitHub の配色に任せる。
   - 図は、問いへのオーナーの答えが担当へ戻る矢印を持たない（線が増えて読めなくなる）。図の下の説明で、答えは kanban を通って担当へ戻ると書く。
   - 図の下の説明で、図に入れなかった細部を言う: レベル A はテストが通れば自動でマージし、レベル B は別ブランドのモデルの独立レビューつきで自動でマージし、レベル C はオーナーがマージする。マージを打つのは dispatcher。
   捨てた案: 手で描いた SVG（[vision.svg](../assets/vision.svg) と同じ作り。見た目は整うが、言語ごとに 1 枚要り、直すたびに座標を触る）。[vision.svg](../assets/vision.svg) をそのまま載せる（2026-11-07 の時点の図で、その後の変更を反映していないと vision.md の冒頭に書いてある）。画面の写真（決めたこと 1 の範囲の外）。

8. **2026-11-22 Andon 自身の開発の流れの数は、README に載せない。（オーナーが決めた。Chief Engineer の推奨は、マージした PR の数・依頼書からマージまでの中央値・オーナーを待たずにマージした割合の 3 つを、期間の日付つきで載せる形だった）**
   オーナー（2026-11-22）: 「流れは乗せてもいいと思うけれども、数は別にいらないと思った」。課題の流れは図と説明（決めたこと 5 の順 2・決めたこと 7）で見せる。
   細部（Chief Engineer が決めた）: 順 6 の節に、「Andon は Andon のセルの中で開発している」の 1 文と、マージ済みの PR の一覧へのリンクを置く。数は書かない。数を取り直す手順も足さない。
   捨てた案: 3 つの数を載せ、バージョンを切るときに取り直す（Chief Engineer の推奨。オーナーが選ばなかった）。PR の数と時間だけ載せる。バッジで自動に出す（数を公開の場所に定期に出す仕組みが要り、範囲を越える）。

9. **2026-11-22 ADR の一覧を `docs/adr/README.md` に移し、`tests/adr-index.sh` の読み先も移す。（オーナーが決めた。決めたこと 1 の範囲に含めて選んだ）** ADR-0033 決めたこと 6 の「README の ADR の一覧」の置き場を改める。検査が見ること（一覧の行が ADR ごとにあり、状態の語が ADR の状態行と同じ）は変えない。
   理由: 一覧は README の日本語の段の 4 割を占め、ADR を足すたびに増える。読み手の判断の材料ではない。
   細部（Chief Engineer が決めた）:
   - `docs/adr/README.md` は日本語で、冒頭に ADR の読み方（番号・状態行・決定の索引・覆し方。design.md §1.8 を指す）を 3〜4 行、続けて一覧を置く。一覧の行の形は今と同じで、リンクの先だけ同じフォルダからの相対パスにする。
   - 2 つの README は、文書の地図の 1 行で `docs/adr/README.md` を指す。
   - 一緒に直す所: `tests/adr-index.sh`（読み先・冒頭の説明・自己確認の写し・出力の文）。`tests/words-selftest.sh` の、一覧の行を `README.md` に書いて試す例。`tests/lib/scope.sh` の対応の表（`README.md` を `adr-index.sh` の行から外し、人用の文書の行へ移す。外すだけだと、知らないパスとして全部の検査が選ばれる）。`tests/ci.sh` の `README.md` で `adr-index.sh` が選ばれるという確かめ。image.md の検査の表の 2 行。design.md §1.8 の 1 文。`AGENTS.md` の ADR の行の「守らせるもの」。ADR-0033 の決めたこと 6 の下に足す改めの 1 行。
   - 新しい ADR の PR は、`docs/adr/README.md` に 1 行足す。README には足さない。
   - 払うもの: 一覧が 1 ファイルである限り、並んで走る ADR の PR は同じ所に行を足してぶつかる。これは移す前と変わらない。
   捨てた案: 一覧を README に残して `<details>` で畳む（行は減らないまま、ADR のたびに 2 つの README を触ることになる）。一覧を持たず、ADR のフォルダのファイルの並びに任せる（並んで走る PR がぶつからなくなるが、ファイルの名前は英語の短い形で、日本語の題と状態は 1 本ずつ開かないと分からない）。
   → 2026-11-22 に、[ADR-0055](0055-adr-prs-without-conflicts.md) 決めたこと 2・5 が、「新しい ADR の PR は `docs/adr/README.md` に 1 行足す」と「検査が見ることは変えない」を覆した。一覧は command が書き、バージョンを切る PR が作り直す。検査は ADR ごとの行と状態の語を見ない。置き場と、冒頭の読み方と、2 つの README から指すことは変わらない（オーナーが決めた）。受け入れ条件 1 の「状態を変えた写しが落ちる」と切る案 1 は、ADR-0055 の受け入れ条件と切る案 1 に置き換える。

10. **2026-11-22 2 つの README の構成がずれないことを、検査 1 本（`tests/readme.sh`）で守る。（Chief Engineer が決めた）**
    見ること:
    - 2 つのファイルがあり、冒頭 10 行以内に互いへのリンクがある。
    - 見出しのレベルの並びが同じ。
    - 相対リンクの先の集合が同じで（互いへのリンクは除く）、先が全部 repo に在る。ページの中の見出しへのリンク（`#…`）は、見出しが言語で違うので集合の比べから外し、数が同じことだけ見る。
    - コードブロック（Mermaid は除く）は、コメント（`#` から行末）と置き換えの印（`<…>` の中）を除いた command の行が同じ。
    - Mermaid のブロックは数が同じで、引用符の中（ラベル）を除いた行の並びが同じ（決めたこと 7 の「同じノードと矢印」）。最初のブロックが先頭から 40 行以内に始まる。
    - 表示名の行と注記（`> [!NOTE]`）が、どちらにもある。
    - ADR の一覧の行（`- [NNNN ` で始まる行）が無い。
    理由: 言語を 2 つのファイルに分けると、片方だけ直す変更が起きる（決めたこと 3 の代わりに払うもの）。構成・リンク・command は機械で比べられる。文の意味のずれは機械で見られないので、同じ PR で両方を直す決まり（決めたこと 3）とレビューに任せる。
    細部:
    - 検査は壊した写しで落ちることを自分で確かめる（`tests/adr-index.sh` と同じ作り）。
    - `tests/lib/scope.sh` では、どの PR でも走る「いつもの分」に入れる。README を変えた PR でだけ選ぶと、README が指す文書を消す・動かす PR では走らず、main に入ってから落ちる。`tests/ci.sh` のいつもの分の確かめと、image.md の検査の表も直す。
    捨てた案: 1 つの元から 2 つを生成する（元の形と生成の道具が要り、README 2 枚には重い。repo に同じ役の生成は `andon process render` の 1 つで、対象は YAML）。既製のリンクの検査器（lychee など）を入れる（見られるのはリンクの先だけで、2 つの構成の比べには使えない。新しい道具の導入にもなる）。何も足さない（ずれが気づかれない）。repo にある同じ役の自作の検査は `tests/adr-index.sh`（索引と本文）と `tests/process.sh`（生成物と元）の 2 本で、README の 2 枚を比べるものは無い。

11. **2026-11-22 英語の文では kanban・issue・owner と書く。（Chief Engineer が決めた）** 今の英語の段の「ledger」は kanban に、「the human」「a person」は the owner に替える。`ledger` は、コードと schema の名前として文書の地図にだけ出す。
    理由: command もフォルダも `kanban` で、日本語の文は kanban と書く（[ADR-0040](0040-kanban-gate-version-words.md)）。2 つの README で同じものを同じ語で呼ぶ。役の名前は日本語で「オーナー」に改めた（[ADR-0036](0036-owner-not-hito.md)）。
    細部: 識別子と、`docs/ledger.md` などほかの英語の文書は変えない。

12. **2026-11-22 repo の description を、今の語と README の英語の語に合わせる。（オーナーが決めた。Chief Engineer の推奨のとおり、3 語とも直す）** 「A one-person software factory: AI coding agents take issues from a kanban through review to merge, and call the owner only for decisions only they can make.」
    今の文（オーナーの決定 2026-11-06）から変えるのは 3 語で、性質が 2 つに分かれる。
    - tasks → issues は、古くなった語の直し。課題への改名（ADR-0008）が、英語の文では issue と書くと決めている。
    - a ledger → a kanban と the human → the owner は、英語の文の語を新しく決めること（決めたこと 11）。ADR-0040 と ADR-0036 が替えたのは日本語の文だけで、今の英語の「ledger」「the human」はどの決定にも反していない。
    ADR-0029 のやらないことの「description を変えること」と、2026-11-06 の description の決定を、この 3 語について改める。social preview の画像と topics は触らない。
    理由: description は、はてなブックマークのページのタイトルと検索の結果にそのまま出る。README の冒頭の文と語が違うと、同じものが 2 つの名前で読まれる。
    細部: 打つのは `gh repo edit --description`。repo の設定なので PR にならない。打つのは README の PR がマージされた日にする。

13. **2026-11-22 README の本文を書き直す PR は、独立レビューが通れば自動でマージする。（オーナーが決めた。Chief Engineer の推奨は、オーナーが読んでからマージする形だった）**
    README の本文の PR は、検査も一緒に直すのでレベル B になる。ふだんどおり、独立レビューのあと、委譲の範囲で自動でマージする。オーナーが読むのはマージのあとで、直したい所は別の PR で直す。
    細部: 冒頭の文と注記の文言はオーナーが決めた文（決めたこと 6）を使う。ほかの節の文は担当が書き、レビュアーが受け入れ条件 7 の 5 問で見る。

## やらないこと（範囲の外。理由）

- **画面の写真とデモの動画。** セルの名前など固有の値が写るので、作り方（見本のセルで撮るか・描くか）から決める別の題（決めたこと 1）。
- **social preview の画像と topics の見直し、表示名の変更。** ADR-0029 のやらないことのまま。
- **[vision.md](../vision.md) と図（vision.svg）の描き直し。** 冒頭に「2026-11-07 の時点のもの」と断ってあり、今の姿は design.md が正。
- **design.md・operate.md・image.md の中身の書き直し。** README は指すだけ。直すのは、README の ADR の一覧や検査の範囲を説明している文（design.md §1.8 の 1 文と image.md の検査の表。決めたこと 9・10）だけ。
- **`.github/workflows/go.yml` の注釈の「README.md」を直すこと。** workflow に触れるとレベル C になる。注釈は今のままでも嘘にならない。
- **外の読み手の導入を支えること**（導入の手引きの拡充・互換の約束・issue や PR の受け付け）。前提 3。
- **ほかの道具との比べを README に置くこと。** 決めたこと 5 の捨てた案。
- **過去の ADR・決定の記録・CHANGELOG の「README の ADR の一覧」という文を書き換えること。** 当時の事実なので残す。
- **`docs/ledger.md` など、ほかの英語の文書の語を替えること。** 決めたこと 11。
- **gate の条件表と、README の PR のマージの仕方を変えること。** 決めたこと 13。
- **流れの数を README に載せること。** 決めたこと 8。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は 3 つ。文書を読む検査（`tests/*.sh`。既にある継ぎ目に 1 本足す）、本物の GitHub の描画（図）、読み手（独立レビューのレビュアー）。証拠は、走らせた command と出力のパスを報告に書く。

1. **ADR の一覧が `docs/adr/README.md` にあり、検査がそこを読む。** `bash tests/adr-index.sh` が通る。README に一覧の行が無いことは `bash -c '[ "$(grep -c "^ *- \[[0-9]\{4\} " README.md)" = 0 ]'` で見る（`grep -c` は 0 件のとき終了コードが 1 なので、数を比べる形にする。この grep が当たりうることは、直す前の main で 45 以上当たることで確かめる）。検査の自己確認で、`docs/adr/README.md` の状態を変えた写しが落ちる。
2. **2 つの README の構成が揃っている。** `bash tests/readme.sh` が通る。自己確認で、次の写しがそれぞれ落ちる: 片方の見出しを 1 つ消した写し、相対リンクを在りえないパスにした写し、片方のコードブロックの command を変えた写し、片方の図の矢印を 1 本消した写し、互いへのリンクを消した写し、片方の注記を消した写し、片方の表示名の行を消した写し、ADR の一覧の行を足した写し、図を 41 行目より後ろへ動かした写し。
3. **`tests/readme.sh` がどの PR でも走る。** `tests/ci.sh` が、README に触れない文書だけの変更でも `tests/readme.sh` が選ばれることを確かめる。
4. **語の検査が `README.ja.md` を見る。** `bash tests/words.sh` と `bash tests/words-selftest.sh` が通り、`README.ja.md` に禁じる語を入れた写しで落ちる。
5. **ほかの検査が通る。** `tests/run --changed` と `go test ./...`。`tests/ci.sh` が、`README.ja.md` だけの変更を文書だけの変更として扱うことを確かめる。
6. **図が GitHub で描かれる。** PR のブランチの `README.md` と `README.ja.md` を github.com で開き、明るい配色と暗い配色で画面を撮る。図が描かれ、灯りのノードの字が読める。証拠は 4 枚の画像のパス。
7. **README だけで 5 問に答えられる。** 課題の依頼書の完了条件に、次の 5 問を書く。独立レビューのレビュアーは依頼書の完了条件を仕様の正として読むので、英語と日本語のそれぞれで、答えが README の中にあり、design.md と食い違わないことを見る。(1) Andon は何か。(2) 課題 1 本は、誰がどの順で扱ってマージまで行くか。(3) オーナーが呼ばれるのはどんなときで、呼ばれないのは何か。(4) 使うには何が要り、向かないのはどんな使い方か。(5) 今どの程度安定していて、PR は送れるか。証拠はレビューの指摘の記録（5 問についての指摘が残っていないこと）。
8. **description。** `gh repo view --json description -q .description` が決めたこと 12 の文と同じ。

## 価値の仮説

1. **誰の何が変わるか**: repo の入口に来た外の読み手が、README だけで、Andon が何か・どう回るか・自分に合うかを判断できる。日本語の読み手は、冒頭のリンクから日本語の README を読める。
2. **見る信号**: オーナーの目視。並べて見る数として、GitHub の traffic（`gh api repos/<owner>/<repo>/traffic/popular/paths`）で、入口の unique に対する `README.ja.md` の unique の割合と、スターの増えた数を取る。信号の記録（contract.md §15）に当たる出所は無いので、観察の課題が command を打って数える。この command は上位 10 件しか返さないので、`README.ja.md` が 10 件に入らなければ、割合は「10 位の unique より少ない」としか言えない。
3. **見る日**: 2026-12-09（README の PR のマージ見込み 2026-11-25 から 14 日後。GitHub の traffic は 14 日分しか残らないので、14 日目に見る）。
4. **誰が要るか**: オーナー（読んで言う）。数のほうは、外の読み手が来ること。入口の unique が 14 日で 100 に届かなければ、数は「分からない」とする。
5. **効いたとどう分かるか**: オーナーが、マージから 14 日のあいだに README を読み返して「骨子が伝わる」と言う。`README.ja.md` の割合は当たりの条件にしない。割合が低いとき、英語の README で足りたのか、最初の画面で離れたのかを、この数は分けられない。
6. **何があれば止めるか**: `README.ja.md` の unique が入口の unique の 50% 以上なら、日本語の README まで進む読み手が多数で、最初の画面が合っていない。言語の並べ方（決めたこと 3）を見直す題を、オーナーと決める。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

この文書の PR に入れるもの（切らない）: この ADR、決定の記録 1 件、今の README の ADR の一覧の 1 行。

1. **ADR の一覧を `docs/adr/README.md` に移す。** 決めたこと 9。受け入れ条件 1・5。依存なし。文書と検査だけ。並んで走る ADR の PR は README の一覧に 1 行足しているので、この課題がマージされたあとは `docs/adr/README.md` に足し直す。先に入れる。
2. **`README.md`（英語）と `README.ja.md`（日本語）を書き、`tests/readme.sh` を足す。** 決めたこと 3〜8・10・11。受け入れ条件 2〜7。依存: 1。依頼書の完了条件に受け入れ条件 7 の 5 問を書く。マージはふだんどおり（決めたこと 13）。一緒に直す所: `README.md` を名指す一覧に `README.ja.md` を足す（`tests/words.sh` の 3 か所・`tests/words-selftest.sh`・`tests/lib/docs-scope.sh`・`tests/lib/image-scope.sh`・`tests/lib/scope.sh`・`tests/ci.sh`・`internal/chiefengineer` の `TestDocumentedInvocations` が読む一覧・image.md の文書だけの PR の説明と表）。`CHANGELOG.md` の冒頭の「README の Status」。README に `andon chief-engineer` の呼び出しを書くなら、`TestDocumentedInvocations` が打てる形（題を `"<題>"` と書く形）にする。
3. **description を直す。** 決めたこと 12。受け入れ条件 8。依存: 2 のマージ（README と同じ日に替える）。repo の設定を変える操作で、PR にならない。
4. **観察。** 価値の仮説。見る日 2026-12-09。依存: 2・3。

## 開いている問い（残った問い。決断の種類つき）

無し。出口の 4 つ（冒頭と注記の文言・流れの数・description・README の PR のマージ）は、オーナーが 2026-11-22 に決めた。

## 出典

- オーナーの発言 2026-11-21（v0.0.13 のあとにやりたいこと）と 2026-11-22（この題の対話。前提への同意・範囲・言語の並べ方・注記・冒頭と注記の文言・流れの数・description・README の PR のマージ）。
- repo の中: `README.md`（main `b890baa`）、[design.md](../design.md) §0・§0.1・§1.1〜§1.3・§1.10・§2.1・§2.6・§2.7、[vision.md](../vision.md)、[decisions.md](../decisions.md) の 2026-11-05（文書の言語）と 2026-11-06（description・表示名）の行、[decisions/2026-11-08-predecessor-ids-and-public-logs.md](../decisions/2026-11-08-predecessor-ids-and-public-logs.md)、[ADR-0008](0008-issue-naming.md)、[ADR-0029](0029-factory-place-and-mechanism.md)、[ADR-0033](0033-adr-inventory-and-slimming.md)、[ADR-0036](0036-owner-not-hito.md)、[ADR-0040](0040-kanban-gate-version-words.md)、[ADR-0023](0023-sidebar-andon-look.md)、[image.md](../image.md)、[contract.md](../contract.md) §2、`tests/adr-index.sh`・`tests/words.sh`・`tests/words-selftest.sh`・`tests/ci.sh`・`tests/lib/scope.sh`、`internal/dispatcher/loop.go`（レビュアーの依頼書）、`internal/chiefengineer` の検査、`package/gate.json`。
- GitHub の数（2026-11-22 に `gh api` で読んだ）: repo の traffic（views・popular/paths・popular/referrers）、スターの日付、repo の description と topics。
- 流れの数: `andon measure --since 2026-11-15 --until 2026-11-21`（2026-11-22 に打った）。
- 外の反応: はてなブックマークの、この repo の URL のページ（2026-11-22 に読んだ。177 件、コメント 6 件）。
