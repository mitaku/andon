# ADR-0008: 台帳のカードを「課題（issue）」に改名するか（語は改める・識別子は改めない・依頼書の欄を「問題」に）（仕様・設計。2026-11-07。タスク Y-197）

- 状態: **提案（2026-11-07）**。人が PR をマージした日が決定の日（[decisions.md](../decisions.md) §14 の行の日付）。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff。Y-191・Y-194・Y-195 のマージのあと）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: [ADR-0006](0006-survey-fix-decide.md) 決めたこと 16 が別の ADR に回した改名。(a) 台帳のカードの日本語の語（タスク → 課題）、(b) 英語の語と識別子（`task`・`kanban/tasks/`・`task.yaml`・`andon task`・schema の `$id`・hc のラベル）を `issue` に替えるか、(c) 替えるなら移し方（形式の版・`kanban migrate`・別名の期間）、替えないなら理由、(d) glossary の「避ける: チケット・issue」と外部トラッカーの Issue との線、(e) 依頼書の欄「やること」が How に寄っていないか、(f) 変える範囲の見積り（task／タスクが人の目に出る所の実測）。
- 語は [design.md](../design.md) §0.1。この文書の「課題」は台帳のカード、「作業」は担当が計画で課題を分けた単位（ADR-0006 前提 10・決めたこと 9）。
- 入力: 依頼書（人の言葉 2026-11-07）、ADR-0006 の前提 9・10 と決めたこと 9・12・16、design.md §0.1（語彙）・§1.7（命名の方針。「task・triage・publish・boot・init は英語のまま。TPS に重なる語が無い」）・§3.5（台帳の CLI の名前。`deck` の別名は 1 版だけ）、glossary skill（「タスク（work item）。避ける: チケット、issue、job」）、[contract.md](../contract.md) §2・§8、[ledger.md](../ledger.md)（形式の版 2。「Moved without a version change」）、chief-of-staff skill §2（依頼書の雛形）、`internal/triage/apply.go`（トリアージが書く依頼書の欄）、andon main の実測（`grep`。前提 6）、Y-141（台帳の置き場の改名。PR #60 の差分の大きさ）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-07）: 「Task の主は How だが、解消したいのは Why/What。それを言葉にすれば issue」。

言い直すと: 台帳のカード 1 枚が持つべきものは「何が困っていて（Why）、何が出れば解けたか（What＝完了条件）」で、どう直すか（How）は担当が理解と計画で決める（ADR-0006 決めたこと 9・10・12）。ところが カードの名前が「タスク（task）」だと、名前が How を呼ぶ——Chief of Staff は依頼書の「やること」に手順を書き、担当は「言われた作業をした＝終わり」と読む。出典（曽根。ADR-0006 前提 9）でも「課題を整理して**作業（タスク）**に分解する」で、タスクは課題ではなく作業の側の語。ADR-0006 は語だけ分けて（課題＝カード、作業＝担当が切る単位）、識別子と glossary の改名をこの文書に回した。

決めるのは 3 つ。**人が読み書きする語**を改めるか、**機械が読む識別子**を改めるか、**依頼書の欄の名前**を改めるか。この 3 つは費用も効き目も別で、まとめて 1 つの「改名」にしない。

## 前提（番号。依頼書と現地から取ったもの）

1. **人（2026-11-07）**: 語の向きは課題（issue）。ADR-0006 決めたこと 16 で決まり、改名はこの文書。
2. **ADR-0006 前提 10・決めたこと 9**: 課題＝台帳のカード（依頼書の「何が困っていて、何が出れば解けたか」と完了条件）。作業＝担当が計画で課題を分けた単位（台帳に乗らない。終了条件は担当が置く）。この 2 語は Y-191（S2）が glossary に足す。
3. **design.md §1.7 命名の方針 1・2**: 識別子の英語は、系譜の語と TPS の語が競えば系譜の語。`task` は「TPS に重なる語が無い」として英語のまま（§3.5 の表）。リーン開発・Kanban Method の系譜で台帳のカードは **work item**（glossary の今の括弧もこれ）で、**issue** はトラッカー（GitHub・Jira）の系譜の語。
4. **glossary skill**: 見出しの括弧は識別子に使う英語。今の見出しは「タスク（work item）」だが、識別子は `task` で、括弧と識別子が既に食い違っている（この文書で直す）。「避ける: チケット、issue、job」。台帳の「避ける: バックログ、トラッカー」。
5. **ADR-0006 決めたこと 14**: 判断の標準は 機械が確かめる形 → 文 → 人のレビュー の順に置く。名前が変えるのは**読む人と agent の判断の向き**で、機械の判定は変わらない。だから改名の効き目は文（skill・依頼書・docs）にあり、識別子（機械が読む所）にはほとんど無い。
6. **現地の実測（andon main `96f40f6`。`grep -c 'タスク'`・`grep -ci 'task'`。横展開＝task／タスクが人の目に出る所を数えた）**:

   | 場所 | 「タスク」（日本語の文） | `task`（識別子・英語） | 人の目に出るか |
   |---|---|---|---|
   | 識別子そのもの | — | `kanban/tasks/<id>/`・`task.yaml`・`andon task <8 つの sub command>`（`start`・`finish`・`ready`・`report`・`stop-check`・`ask`・`review-request`・`show`。`candidates` は ADR-0006 で足す）・`schema/task-keys.schema.json`（`$id` `…/task-keys/1`）・5 つの schema のキー `task`／`reviewer_task`／`triage_max_tasks`・hc のラベル `task=<id>`・`kanban` の sub command の引数 | 出る（command を打つ・`--help`・ボードがラベルを読む） |
   | skill（7 本） | 109 | 105 | 出る（agent が読む文。効き目が最も大きい所） |
   | プロファイルの template（最初のメッセージ。7 本） | 8 | 0 | 出る |
   | docs（生きている文書: design・contract・dispatcher・operate・README の日本語） | 144 | 170 | 出る |
   | ledger.md（形式の正。英語） | 0 | 101 | 出る（形式の読み手） |
   | decisions.md・ADR 0001〜0007 | 231 | 142 | 出るが**当時の言葉のまま**（decisions.md §16 の前例） |
   | Go（`cmd/kanban` 7 ファイル・`internal` 25 ファイル。テスト除く） | 日本語の文字列 39 種（command の出力・最初のメッセージ・通知の見出し） | 文字列 185 種・約 820 行 | 日本語の文は出る。識別子は出る（`--help`・JSON のキー） |
   | tests（13 ファイル・約 360 行） | — | — | 出ない（検査） |
   | ボードの mod（`package/host/kanban-board` ほか 10 ファイル） | 0 | 268（JS の変数名。台帳の path `kanban/tasks` が 6 か所） | 変数名は出ない。path は形式 |

   前例: Y-141（台帳の置き場 `.flightdeck/` → `kanban/`・`coordinator` → `chief_of_staff`・id を 4 桁）は 66 ファイル・+1,740／−726 行で、`kanban migrate` を 1 本足した。識別子 `task` の改名はそれより広い（Go・schema・skill・docs・tests・ボードの mod・hc のラベル・形式の版）。
7. **外部トラッカーとの線（chief-of-staff skill 規約 7・design.md §1.3 の 3）**: 外部のトラッカー（Backlog・GitHub Issues 等）で回る案件はそちらが正で、台帳のカードは `links` にキーを持つ写し。ADR-0006 決めたこと 6 の例 3 は「再現手順は issue #12」と GitHub の Issue を指す。design.md §1.9 は「外からの issue は無い」「信号の入口（CI・issue・監視）は最初の版の後」。つまり Andon の文では **issue はすでに外のものを指す語**で、将来の信号の入口でも外から来る。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-07 台帳のカードの日本語の語は「課題」。「タスク」は、台帳のカードの意味でも作業の意味でも使わない（作業は「作業」）。glossary の見出しは「課題（task）」で、Y-191 が足す「課題」の項と 1 つにする。**

   - glossary: 「タスク（work item）」の項を「**課題**（task）」に改める。定義は「台帳のカード 1 枚（`tasks/<id>/`）。何が困っていて、何が出れば解けたか（完了条件）を持つ。どう直すかは持たない（担当が計画で作業に分ける）。かんばん。プロジェクトの外では `<slug>/<id>`。人へ書くときは「番号（題）」」。_避ける_: タスク（作業の意味と混ざる。識別子 `task` はそのまま）、チケット、issue（外部トラッカーの Issue を指すときだけ「<トラッカー名> の Issue」と書く）、job。
   - 「作業」の項（Y-191）に「_避ける_: タスク」を足す。
   - 英語の文（README の英語・ledger.md）は **task のまま**。括弧の英語も `task`（識別子と揃える。「work item」は使わない——識別子に無い語を括弧に置かない、が glossary の決まり）。
   - 文の中で識別子を読むときは識別子のまま（「`andon task start` で課題を起こす」「`task.yaml`」）。日本語の語と識別子の英語が違うのは 建屋（plant）・台帳（ledger）・担当（worker）と同じ形で、新しい形ではない。
   - 仮設の語の表（glossary の末尾）に「タスク → 課題（識別子は `task`）」を 1 行足す。

   理由: 前提 1・2。出典の語（前提 9 の「作業（タスク）」）では タスク＝作業 で、ADR-0006 の背骨（課題の完了条件・作業の終了条件）が skill に入ると、「タスク」が課題と作業の両方を指して読めなくなる。改名の効き目は文にある（前提 5）ので、語を改めるのが最も安い 1 手。
   捨てた案: タスクと課題を同義で併用（glossary の「ここにある語だけを使う」が崩れ、作業との線が消える）／カードを「作業」と呼ぶ（How の側の語。人の指摘の逆）／英語の文も issue に（前提 7。README の読み手は GitHub の Issue を知っている人で、`andon task` と並ぶと別物に読める）。

2. **2026-11-07 識別子は `task` のまま。形式の版は 2 のまま、`kanban migrate` は触らず、別名の期間も無い。**

   - 変えないもの: `kanban/tasks/<id>/`・`task.yaml`・`andon task <sub>`・`kanban <sub> <id>` の語・`schema/task-keys.schema.json` と `$id`・通知／報告／問い／ゲートの schema のキー `task`・`reviewer_task`・`triage_max_tasks`・hc のラベル `task=<id>`・ledger.md の英語・ボードの mod が読む path・Go の変数名と JSON のキー。
   - 日本語の語と識別子が違うことは glossary の 1 行（「識別子は `task`」）で持つ。

   理由:
   - **効き目が無い所に費用を払わない**（前提 5。ADR-0006 決めたこと 14）。識別子を読むのは command・schema・dispatcher・ゲート・ボードで、どれも名前で判断を変えない。人と agent の判断の向きを変えるのは文で、1 で済む。
   - **`issue` は外のものの語**（前提 7）。`andon issue start` は `gh issue` と並び、`links` の先の Issue と台帳の issue が同じ文に出る（「この issue の links は GitHub の Issue #12」）。信号の入口（design.md §1.9）が入ると、外の Issue から台帳のカードを切る流れの両端が同じ名前になる。系譜でも work item が台帳の語で、issue はトラッカーの語（前提 3）。
   - **費用**（前提 6）: Y-141 より広く、形式の版を 3 に上げる（ledger.md の Compatibility: フォルダ名とファイル名の変更は版の変更）。`kanban migrate` に v2→v3 を足す。動いている端末のラベル `task=` は書き換えられないので、`doing`・`review` があれば止まる（今の migrate と同じ）。ボードの mod（Andon 同梱と、人の App にすでに置いたもの）が path を読み替える。`andon task` の別名を 1 版残す。skill・docs・tests の `task` を全部書き換える。ADR と decisions.md は当時の言葉のまま残る（前例）ので、読み手は 2 つの名前を往復する。
   - **`card`・`item`（かんばんのカードの語。衝突しない）も採らない**: 1 と同じ理由で効き目が無く、費用は同じ。

   捨てた案（変えるならこの形。開いている問い 1 の「変える」の答えはこれ）: 名前は `issue`（`kanban/issues/<id>/`・`issue.yaml`・`andon issue <sub>`・`schema/issue-keys.schema.json`・キー `issue`／`reviewer_issue`／`triage_max_issues`・ラベル `issue=<id>`）。形式の版 3。`kanban migrate` が v2 の台帳を 1 回で移す（フォルダとファイルの改名・`config.yaml` の `version: 3`・dispatcher の控え。`doing`・`review` があれば止まる）。`andon task` は 1 版だけ別名（§3.5 の `deck` と同じ）。ADR・decisions.md・`log.jsonl`・既存の依頼書と報告は書き換えない。Y-170（建屋 → セル）と同じく語の改めのタスクとして切り、先に Y-191〜Y-196 を済ませる。見積りは Y-141 の約 2 倍（前提 6 の表の Go・schema・tests・ボードの mod・docs・skill）。

3. **2026-11-07 依頼書の欄「やること」を「問題」に改める。書くことは ADR-0006 決めたこと 12 のまま（何が困っていて、何が出れば解けたか。手順ではない）。**

   - 雛形（chief-of-staff skill §2）の 1 行目を `問題: <何が困っていて、何が出れば解けたか（1〜3 行）。手順ではない。場所の手がかりは書いてよい>` に。欄の並びは 問題・なぜ・完了条件・横展開・守ること・Settings。
   - 読む側（worker skill §2「依頼書の『やること』『完了条件』『守ること』だけをする」）は「問題」を読み、古い依頼書の「やること」も同じ欄として読む（1 行の括弧）。既存の依頼書は書き換えない。
   - `andon triage apply` が書く依頼書の欄（`internal/triage/apply.go` の `やること:`）と triage skill の説明、`tests/andon-triage.sh`・`tests/package.sh` の雛形の検査を揃える。
   - ADR-0006 決めたこと 12 の表の「やること」の行は、この文書で「問題」と読み替える（ADR-0006 は書き換えない）。

   理由: 人の指摘そのもの。「やること」は字のとおり How（する事）の欄名で、決めたこと 12 が「手順ではない」と但し書きで押さえているのは、名前が中身と逆だから。Chief Engineer の文書の最初の節が「問題（人の言葉で）」で、依頼書の最初の欄も「問題」なら、文書 → 依頼書の写しが 1 対 1 になる（決めたこと 12 の理由「完了条件が状態で書けていれば担当は理解と計画ができる」）。
   捨てた案: 「困りごと」（口語で読みやすいが、Chief Engineer の文書の「問題」と 2 語になる）／「課題:」（カードの名と欄の名が同じで、課題の中の課題になる）／「やること」のまま但し書きで持つ（今の形。名前が読み手を How へ引く）／欄を 5W1H（Why・What・Where）に作り直す（ADR-0006 決めたこと 12 の捨てた案と同じ。既存の依頼書と skill のテストが全部変わる）。

4. **2026-11-07 外部トラッカーとの線。台帳のカードは「課題」、トラッカーのものは「<トラッカー名> の Issue」（GitHub の Issue #12・Backlog の課題 PROJ-12）。`links` は課題からトラッカーのものへの参照で、中身は写さない（規約 7 のまま）。**

   - skill と docs の文で外のものを指すときはトラッカー名を添える。ADR-0006 決めたこと 6 の例 3 は Y-195 が skill に写すとき「GitHub の Issue #12」と書く（ADR-0006 は書き換えない）。
   - Backlog の「課題」（Backlog の日本語 UI の語）と台帳の課題が同じ文に出るときは「Backlog の課題 <キー>」と書く。

   理由: 前提 7。課題と Issue を 1 語にしないことで、信号の入口（design.md §1.9。外の Issue → トリアージ → 台帳の課題）が来ても両端の名前が違う。
   捨てた案: 台帳のカードも issue と呼び、トラッカーのものを「外部 Issue」と呼び分ける（既定の語が外のもので、台帳の語に修飾が要るのは逆）。

5. **2026-11-07 変える範囲と順（実装のタスクが直す。この文書の PR では変えない）。先に Y-191（glossary）・Y-194・Y-195（skill と設計書の文）をマージし、同じファイルを 2 本のタスクで触らない。**

   | 場所 | 変える所 | 変えない所 |
   |---|---|---|
   | glossary skill | 「タスク」の項 → 「課題（task）」（1）。「作業」に避ける語。仮設の語の表に 1 行 | — |
   | skill 6 本（worker・chief-of-staff・chief-engineer・ask-one-by-one・independent-review・triage）と template 7 本 | 文の「タスク」→「課題」（作業の意味なら「作業」）。依頼書の雛形の「やること」→「問題」（3）。「issue」にはトラッカー名（4） | 識別子（`andon task`・`tasks/<id>/`・`task.yaml`・`task=`）。skill の名前 `worker` |
   | docs（design・contract・dispatcher・operate・README の日本語） | 文の「タスク」→「課題」。design.md §0.1 の表の行を「課題 \| task \| 台帳のカード 1 枚…」に。§3.5 の表の `task` の行に「日本語の語は課題。ADR-0008」。contract.md §2 の見出し「台帳の項目（`task.yaml` の…）」はそのまま | ledger.md（英語の形式の正）・decisions.md の過去の行・ADR 0001〜0007・README の英語 |
   | Go の日本語の文字列（39 種。command の出力・最初のメッセージ・通知の見出し・`--help` の日本語） | 「タスク」→「課題」。例: 「タスク `<slug>/<id>`。依頼書は …」→「課題 `<slug>/<id>`。依頼書は …」（template 7 本の 1 行目と揃う）。`andon triage apply` の「やること:」→「問題:」 | 識別子・JSON のキー・変数名・英語の文字列 |
   | tests | 変えた文字列を見る検査（`tests/package.sh`・`tests/andon-triage.sh`・`tests/andon-task.sh`・`tests/andon.sh`） | 形式・schema の検査（変わらない） |
   | ボードの mod | 変えない（人の目に「タスク」は出ていない。変数名は識別子） | 全部 |
   | 台帳の中身（既存の依頼書・報告・`log.jsonl`） | 変えない | 全部 |

   順: Y-191・Y-194・Y-195 のマージ → S2（glossary・skill・template・依頼書の欄。1 本）→ S3（docs）と S4（Go の日本語の文字列と tests）は独立。

   理由: 同じ skill を Y-194・Y-195 と同時に触ると衝突する（chief-of-staff skill §3 の 1「ぶつからないか」）。語の改めは Y-170（建屋 → セル）と同じ型の仕事で、プロファイルは `sonnet`。
   捨てた案: 文書を触るたびに直す（glossary の「ここにある語だけ」が長く破られる）／全部を 1 本（約 300 か所・20 ファイル超。レビューしにくい。3 本に分ける）。

## やらないこと（範囲の外。理由）

- **識別子の改名**（`issue`・`card`・`item`）: 決めたこと 2。人が開いている問い 1 で「変える」と答えたら、2 の捨てた案の形で別のタスク。
- **形式の版 3・`kanban migrate` の版上げ・`andon task` の別名**: 識別子を変えないので要らない。
- **ADR 0001〜0007・decisions.md の過去の行・ledger.md の英語・`log.jsonl`・既存の依頼書と報告の書き換え**: 当時の言葉のまま（decisions.md §16 の前例）。
- **README の英語を issue に**: 決めたこと 1 の捨てた案。
- **「作業」を台帳に乗せる**（作業の一覧の形・`library/plan.json`）: ADR-0006 決めたこと 9 の捨てた案のまま。
- **ボードの mod の変数名**: 人の目に出ない。
- **仮設の工場（yard・deck）の語**: 仮設は止まるまで当時の言葉のまま（glossary の仮設の語の表に 1 行足すだけ）。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

1. **パッケージ（`tests/package.sh`）**: glossary に見出し「課題（task）」があり、「タスク」は glossary の「避ける」の行と仮設の語の表にしか無い。skill 6 本と template 7 本に「タスク」が無い（識別子 `task` は数えない）。chief-of-staff skill §2 の雛形の最初の欄が `問題:` で、`やること:` が無い。worker skill が「問題」と古い「やること」の両方を読むと書いてある。
2. **トリアージ（`tests/andon-triage.sh`）**: `andon triage apply` が書く依頼書の最初の欄が `問題:`。
3. **command（`tests/andon-task.sh`・`tests/andon.sh`・`go test ./...`）**: `cmd/`・`internal/` の日本語の文字列に「タスク」が無い（`grep -rn 'タスク' --include='*.go' cmd internal` が空）。担当の最初のメッセージが「課題 `<slug>/<id>`。依頼書は …」。
4. **形式と schema（`tests/kanban.sh`・`tests/schema.sh`）**: 変わらない。`config.yaml` の `version` は 2、`$id` は今のまま、`kanban migrate` の振る舞いは Y-141 のまま。
5. **文書**: design.md §0.1 の表に「課題 \| task」、§3.5 の表の `task` の行に ADR-0008 への参照。docs の日本語の文に「タスク」が無い（decisions.md・ADR・ledger.md を除く。`grep -c 'タスク' docs/design.md docs/contract.md docs/dispatcher.md docs/operate.md README.md` が全部 0）。`tests/public.sh` が通る。
6. **建屋で実測（Y-196 と一緒に）**: 「問題:」で書いた依頼書を 1 本流し、担当の報告の `## Report` の冒頭の言い直しが「問題」と「完了条件」から書かれ、依頼書に手順が無くても担当が計画して PR が出る（人の turn がゼロ）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 |
|---|---|---|---|
| S1 | この文書の PR（decisions.md §14 に 1 行） | CI green（`tests/public.sh` を含む） | — |
| S2 | glossary（課題（task）・作業の避ける語・仮設の語の表）、skill 6 本と template 7 本の「タスク」→「課題」／「作業」、依頼書の雛形の「やること」→「問題」、worker skill の読み方の 1 行、issue へのトラッカー名（1・3・4） | 受け入れ条件 1 | S1、Y-191・Y-194・Y-195 のマージ |
| S3 | docs の日本語（design §0.1・§3.5 を含む・contract・dispatcher・operate・README）の「タスク」→「課題」 | 受け入れ条件 5 | S2（glossary が先） |
| S4 | Go の日本語の文字列（39 種）と `andon triage apply` の欄名、tests の文字列 | 受け入れ条件 2・3 | S2。S3 と独立 |
| S5 | 建屋で実測（受け入れ条件 6）。Y-196 の (a) と同じ依頼で、依頼書を「問題:」で書く | 報告 | S2・S4、Y-196 |

## 開いている問い（人に聞く。1 問ずつ。推奨案つき）

1. **識別子 `task` を残すか**（判断の種類: ライン〈台帳の形式・command の名前・hc のラベル〉。人の見立ては issue だった）。推奨案: **残す**（決めたこと 2。効き目は文にあり、`issue` は外のものの語、費用は Y-141 の約 2 倍で形式の版が上がる）。「変える」なら決めたこと 2 の捨てた案の形（`issue`・形式 3・`kanban migrate`・別名 1 版）で、Y-191〜Y-196 のあとに別のタスク。
2. **依頼書の最初の欄の名前は「問題」でよいか**（判断の種類: ライン〈依頼書の雛形〉）。推奨案: **問題**（決めたこと 3。Chief Engineer の文書の「問題」と 1 対 1）。代案は「困りごと」。

## 出典（読んだもの・捨てた案の出所）

- 依頼書（人の言葉 2026-11-07）、[ADR-0006](0006-survey-fix-decide.md) 前提 9・10、決めたこと 6・9・12・14・16、開いている問い 2。
- [design.md](../design.md) §0.1（語彙）・§1.7（系譜と命名の方針）・§1.9（外からの issue は無い。信号の入口）・§3.5（CLI の名前。`deck` の別名 1 版）。
- glossary skill（`package/skills/glossary/SKILL.md`）、chief-of-staff skill §2・§9 規約 7、worker skill §2、triage skill、`internal/triage/apply.go`。
- [contract.md](../contract.md) §2（`task.yaml` の予約キー）・§8（hc のラベル）、[ledger.md](../ledger.md)（Compatibility・Moved without a version change・Moving from `.flightdeck/`）。
- [decisions.md](../decisions.md) §16（Y-141 の行。「前の名前を読む互換は作らず、`kanban migrate` が 1 回で移す」「前の行は当時の言葉のまま」）、PR #60 の差分（66 ファイル）。
- 実測: andon main `96f40f6` の `grep`（前提 6）。
