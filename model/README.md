# model/ — 課題の一生のモデルと規則

課題 1 つの一生（kanban に入ってから片付くまで）を TLA+ で写したモデルと、その一生が守る規則です。検査器 [tla-ts](https://github.com/aidenybai/tla-ts) が、届く全部の状態で規則が成り立つかを調べます。決めた経緯は [ADR-0025](../docs/adr/0025-formal-model-in-repo.md) にあります。

モデルが確かめるのは設計（規則と動き）です。コードの退行は、今までどおり `tests/` の筋書きのテストが確かめます。モデルとコードを機械で照らすことはしません。モデルは Go のコードを読んで手で写したもので、写したコードの場所を行動ごとに書いてあります。

## 回し方

```sh
model/check
```

手元も CI（`.github/workflows/model.yml` の job `model`）も、この 1 つを打ちます。要るのは Node と npm だけです（バージョンは repo の `mise.toml`）。検査器は `package-lock.json` のとおりに一時の場所へ入れ、install のスクリプトは走らせません。モデルの写しも一時の場所で回すので、repo にも HOME にも書きません。

`model/check` が確かめること:

1. 本物のモデル（`IssueRules.cfg`）が規則を全部守り、どの状態からも先がある。調べる規則は控え（`expected.txt` の `rule`）と同じ集合でなければならない（規則を設定から消すと落ちる）。規則には、検査器が調べるもの（`INVARIANT`）と、検査器が書いた状態の図を `reach.mjs` が読むもの（`IssueRules.cfg` の `\* REACH` の行）の 2 種類がある（下の「行ける」の形の規則）。
2. 状態の数が控え（`expected.txt` の `states`）と同じ。
3. 壊した見本（`broken/`）がどれも、控え（`expected.txt` の `broken`）に書いた規則の破れで落ちる。`REACH` の規則の見本は、`INVARIANT` を全部守ったうえで、その規則だけで落ちなければならない。
4. モデルと課題の状態の定義（`process/states/issue.yaml`）が食い違わない（下の「issue.yaml と照らす」）。
5. 下書きを調べる口（下の「下書きを調べる」）が動く。手を入れていない写しは通る。壊した見本を下書きとして渡すと、控えに書いた規則の名前を出して落ちる。状態の数が控えと違う写しでも、規則を守っていれば通り、飛ばした確かめの名前を書く。

2 と 3 は、検査器の誤りで「破れているのに通る」を見つけるための確かめです。

TLC で見比べるときは、TLC と同じ旗を受けるコマンドを `MODEL_CHECKER` に渡します（例: `MODEL_CHECKER="java -cp tla2tools.jar tlc2.TLC" model/check`）。モデルと設定のファイルは TLC と共通です。

### 下書きを調べる

```sh
model/check --draft <フォルダ> [<モジュール>]
```

設計の段で、このモデルの写しに案を書き足して規則を確かめるときの口です（ADR-0038 決めたこと 13）。フォルダの直下の `*.tla` と `*.cfg` を一時の場所に写し、上と同じバージョンの検査器と、repo の `reach.mjs` だけを回します。モジュールの既定は `IssueRules` で、設定は `<モジュール>.cfg` です。調べる規則は、その設定の `INVARIANT` と `\* REACH` の行です。

案を足した写しは、控えや対応表を直さない限り `model/check` のどこかで落ちます。そこでこの口は、控え（`expected.txt` の規則の集合と状態の数）・壊した見本（`broken/`）・状態の定義との照らし（`issue-map.txt` と `process/states/issue.yaml`）を飛ばし、飛ばしたことを出力の「飛ばす:」の行に書きます。規則を全部守り、どの状態からも先があれば終了コード 0 です。規則が破れたら、その名前と初めの状態から破れまでの並びを書いて 1 で終わります。渡したフォルダには書きません。設定に `CHECK_DEADLOCK` の行があると、先の無い状態の検査を切れてしまうので、値に関わらず断ります（引数なしのときも同じ）。

実装の PR では、下書きの口ではなく引数なしの `model/check` を通します。

## ファイル

| ファイル | 中身 |
|---|---|
| `IssueLife.tla` | 状態の変数と行動（コマンド・dispatcher・オーナーの動き・gate が決めたあとの変化）。行動ごとに写したコードの場所 |
| `IssueRules.tla` | 規則 R1〜R21（`IssueLife` を読む。R7・R18・R20 は文で書き、確かめるのは `reach.mjs`） |
| `IssueRules.cfg` | 検査器の設定（調べる規則の並び）と、`reach.mjs` が読む `\* REACH` の行 |
| `reach.mjs` | 状態の図を読んで「行ける」の形の規則を確かめる（`check` が打つ。Node の標準だけ） |
| `issue-map.txt` | 対応表: 行動とコマンドの対応と、「モデルの外」と名指しする `issue.yaml` の行 |
| `conform.mjs` | 状態の図と `issue.yaml` を対応表で照らす（`check` が打つ。Node の標準だけ） |
| `expected.txt` | 調べるはずの規則・状態の数の控えと、見本ごとの落ちるはずの規則 |
| `broken/` | 直す前の動きに戻した見本（A-0002・A-0003・A-0006 で直した動きと、ADR-0032 決めたこと 8 で直す review-request の守り）と、守りを外した見本（ADR-0034 決めたこと 3 の、マージの直前の読み直しと、閉じた PR の聞き直し。ADR-0042 決めたこと 19 の、ペインの答えの通知と、開いている問いだけに書く守り。ADR-0056 決めたこと 17 の、止める口が印を黄にする守りと、マージの直前に止めを読み直す守り。ADR-0067 決めたこと 16 の、前の課題が main に入ったあとの判定のやり直しと、積んだ PR のマージの守りと、前の課題が終わったことの知らせ）。積む前からある見本は、前の課題の無い課題（`prev = "none"`）だけを見る（積んだ課題の動きは見本が壊したものに関わらず、状態の図が大きくなりすぎるため） |
| `check` | 入口 |
| `package.json`・`package-lock.json` | 検査器のバージョン |

## issue.yaml と照らす

`issue.yaml` は手で書く定義で、`andon issue …` はそこに無い移り方を断ります。モデルから作るのではなく、`model/check` が 2 方向で照らします（ADR-0025 決めたこと 6）。検査器が書き出した状態の図（`-dump dot,actionlabels`。TLC も同じ旗）を、`conform.mjs` が対応表 `issue-map.txt` を介して `issue.yaml` と突き合わせます。

- **モデル → issue.yaml**: 行動が `status` を動かすなら、その `from`・`to`・コマンドの行が `issue.yaml` にある。コマンドを `-` とした行動（端末が落ちる・gate の答えなど）は `status` を動かしてはいけない。
- **issue.yaml → モデル**: `issue.yaml` の行（`command` の「andon issue a・b」は a と b を 1 つずつ）は、モデルで起きるか、対応表の `outside` に理由つきで名指しされている。モデルで起きる行を `outside` に書いても落ちる。

`issue.yaml` に行を足したら、モデルに行動を足すか、`issue-map.txt` に `outside` の行を足すかを決めます。モデルに行動を足したら、`issue-map.txt` に `action` の行を足します。

## 規則

| 名前 | 中身 |
|---|---|
| R1 `R1_NoWorkerAfterCancel` | やめると決めた課題で、担当がまた動き出すことはない |
| R2 `R2_FixReachesSomeone` | gate が直しを返したら、担当か Chief of Staff のどちらかが必ずそれを知る |
| R3 `R3_CleanupIsWhole` | worktree を片付けたら、課題は done になっている（片付けは全部か無し） |
| R4 `R4_DoneMeansMergedOrCancelled` | done の課題は、マージしたか、やめると決めたものだけ |
| R5 `R5_NoWorkerWhenDone` | done の課題に担当は残らない |
| R6 `R6_NoReviewWithOpenQuestion` | 担当の問いが開いたまま review にならない |
| R7 `R7_MergeWithoutCancel` | 取り消しを使わずに届くどの状態からも、取り消しを使わずにマージまで行ける（ADR-0032 決めたこと 9）。閉じた PR を開き直す道も含む |
| R8 `R8_AutoMergeOnlyApproved` | dispatcher が自動でマージするのは、gate が approve と判定した head で、その時点で任されているものだけ（ADR-0034 決めたこと 3） |
| R9 `R9_OnlyWrittenAnswersReachWorker` | 担当に届く答えは、だれかが書いた答えだけ。ペインの答えは、Chief of Staff が通知 `answered` を読んでから届く（ADR-0042 決めたこと 19） |
| R10 `R10_OneAnswerPerQuestion` | 1 つの問いに、答えは 1 つだけ（会話の答えとペインの答えが重ならない。ADR-0042 決めたこと 19） |
| R11 `R11_NoRelayWithoutAnswer` | 答えの無い問いは、担当に届いたことにならない（ADR-0042 決めたこと 19） |
| R12 `R12_NoMergeBeforeGate` | repo の ruleset が印 `andon/gate` を必須にしていれば、gate が渡していない head も、止めている間の PR も、マージされない（オーナーのマージも dispatcher のマージも。ADR-0056 決めたこと 17） |
| R13 `R13_GreenOnlyWhenReleased` | 開いている PR の今の head の印が緑なのは、gate がその head を approve か needs_human と判定していて、止めていないときだけ（ADR-0056 決めたこと 17） |
| R14 `R14_NoAutoMergeWhileHeld` | dispatcher は、止めている PR を自分でマージしない（ruleset が印を必須にしていなくても。ADR-0056 決めたこと 17） |
| R15 `R15_NoMergeWhileStacked` | 前の課題の branch に載ったままの PR は、マージされない（オーナーのマージも dispatcher のマージも。ADR-0067 決めたこと 16） |
| R16 `R16_BehindIsNotReleased` | 前の PR 待ちと判定した PR には、緑の印が付かず、マージを渡したことにもならず、オーナーにマージを頼まない |
| R17 `R17_ApproveOnlyOnMain` | gate が approve と判定するのは、既定の branch に載った PR だけ |
| R18 `R18_RestackWithoutOwner` | 前の課題がマージされずに終わる場合を除き、届くどの状態からも、オーナーの判断も、だれかの push も、Chief of Staff の止めも借りずに、既定の branch に載った形まで行ける（「行ける」の形） |
| R19 `R19_NoGreenWhileStacked` | 前の課題の branch に載ったままの head には、緑の印が付かない |
| R20 `R20_ContinueAfterPrevEnds` | 前の課題がマージされずに終わっても、やめる口も、だれかの push も、Chief of Staff の止めも借りずに、既定の branch に載った形まで行ける（「行ける」の形） |
| R21 `R21_ToldOnlyWhenGone` | 前の課題がマージされずに終わったと Chief of Staff に知らせるのは、終わったあとだけ |

### 「行ける」の形の規則

R7・R18・R20 のほかの規則は「この状態のときはこうなっている」の形で、検査器が状態 1 つずつ確かめます（`INVARIANT`）。R7 の「どの状態からも、マージまで行ける道がある」は状態 1 つの式では書けません。検査器の時相の性質（`PROPERTY`）は「どの道を進んでも」の形なので、これも「行ける道がある」は言えません。そこで、検査器が書いた状態の図を `reach.mjs` が読み、初めの状態から届く状態のそれぞれから、マージした状態まで道があるかを辿ります。

R7 の取り消しは、Chief of Staff のやめる（`ChiefOfStaffCancels`）と、gate を待たずにオーナーが先にマージすること（`HumanMergesEarly`）です。オーナーのマージは同じ動きを 2 つの行動に分けてあり、gate が決めたあと（approve で `merge: person`・needs_human の問いにオーナーが答えた）のマージ（`HumanMergesDecided`）は道に数えます。破れていると、`reach.mjs` は行けない状態のうち初めから一番近いものと、そこまでの行動の並びを書きます。

`IssueRules.cfg` の `\* REACH` の行は検査器にはコメントで、`check` が読んで `reach.mjs` に渡します。形は `\* REACH <規則> <変数> = <値> EXCEPT <除く行動…>` です。状態の図は同じ 2 つの状態の間の移り方を 1 本にまとめるので、除く行動には、ほかの行動と同じ移り方にならないものを選びます。

## 変えるとき

- 課題の一生の動き（finish・resume・gate の返し方など、行動が写しているコード）を変えたら、`IssueLife.tla` の行動とコードの場所を直します。状態の数が変わったら、`model/check` を打って `expected.txt` の `states` を直します。
- 規則を足すときは `IssueRules.tla` に書き、`IssueRules.cfg` の `INVARIANT`（「行ける」の形なら `\* REACH` の行）と `expected.txt` の `rule` に名前を足します。
- `model/` のモデルは整数の演算を読み込んでいないので、写しに足す変数は数でなく真偽で書きます。数で書くと検査器が構文で落ちます。
- 流れの順や状態の移り方を変えるときは、実装の前にこのモデルに書いて `model/check` で確かめます（ADR-0032 決めたこと 9）。

## gate が決めたあとの変化

gate が決めたあとに起きる 3 つの変化を、行動として入れてあります（ADR-0034 決めたこと 3）。

| 行動 | 中身 |
|---|---|
| `HeadMovesAfterGate` | gate が approve か needs_human を決めたあとに、PR の head が動く（担当かオーナーが push した）。PR が閉じている間の push も入る |
| `PRClosesOrReopens` | オーナーが PR をマージせずに閉じる・閉じた PR を開き直す。`andon issue settle` が聞き直して知る |
| `MergeSettingChanges` | issue.yaml の `merge` か委譲の `merge` が変わる。モデルは 2 つを「任されている」の 1 つ（`mergeAuto`）にまとめた |

gate の答えを捨てて回し直すのは、gate の 3 つの行動の始まりの 1 つ（`Regate`）にまとめました。コードでは、読み直しが通らない巡と、base が動いて `gh pr merge` が通らなかった巡は決めたことを書かずに終わり、次の巡で gate が決め直します。モデルはその 2 歩を 1 歩にしています。dispatcher が自分でマージする approve は、マージの直前に head と任されているかを読み直し、通ったときだけマージします（`DispatcherMerges`）。

## 印と止め

gate の印 `andon/gate`（`mark`）と、Chief of Staff の止め（`hold`。issue.yaml の予約キー）を入れてあります（ADR-0056 決めたこと 17）。`required` は repo の ruleset が印を必須にしているかで、初めにどちらかに決まり、途中で変わりません。`rel` は gate が今の head を渡した（approve か needs_human と判定した）か、`badMerge` は渡していない head か止めている間の PR がマージされたかです。

| 行動 | 中身 |
|---|---|
| `ChiefOfStaffHolds` | `andon issue hold`。hold を書き、今の head の緑を黄にしてから返る。gate が決めたあとなら判定を捨てる（コードでは次の巡の `holdDecision` が捨て、その間のマージは直前の読み直しが断る） |
| `ChiefOfStaffUnholds` | `andon issue unhold`。印は書かず、次の巡で gate が回り直して書く |
| `DispatcherAlignsMark` | dispatcher が PR を読むとき印を揃える。止めているのに緑なら黄に、doing に戻った課題の緑は赤に |

R8 は、dispatcher がマージした状態で、gate の答えと head と設定を確かめます。マージしたあとは、head は動かず、PR も閉じず、設定の変更も入れていないので、マージした時点の値がそのまま残ります。

## 積んだ課題

前の課題の PR の branch の上で起動する課題（ADR-0067 決めたこと 16）を入れてあります。モデルは課題 1 つの一生なので、前の課題はその PR の状態だけを変数で持ちます。`base` はこの課題の branch が載っている所（`main` か、前の課題の branch の `prev`）、`prev` は前の課題の PR（`none`・`waiting`・`merged`・`gone`）、`prevTold` は前の課題がマージされずに終わったことを Chief of Staff に知らせたかです。起動（`ChiefOfStaffStarts`）は、前の課題が main に入っていなければその branch の上に載せます。approve・オーナー待ち・dispatcher のマージ・オーナーのマージは、既定の branch に載った PR だけに起きます。

| 行動 | 中身 |
|---|---|
| `GateSaysBehind` | gate が前の PR 待ち（`behind`）と判定する。印は黄。CI の結果が無い head にも起きる |
| `WorkerRestacks` | `andon issue restack`。担当が既定の branch の先へ載せ直し、PR の向き先を既定の branch にする |
| `PrevMerges` | 前の課題の PR が main に入る。`Regate` が、前の PR 待ちの判定を捨てて回し直す |
| `PrevEnds` | 前の課題がマージされずに終わる |
| `GateTellsPrevGone` | 前の課題が終わったことを、dispatcher が Chief of Staff に知らせる |
| `ChiefOfStaffContinuesWithoutPrev` | 知らせたあと、オーナーの答えで Chief of Staff が担当へ返す |

待つ相手（`after`）と kanban の守り、3 つ以上積んだ列の 2 つ目から上、オーナーが draft を外して積んだ PR を前の課題の branch へマージすることは、モデルの外です（受け入れ条件の筋書きのテストで見る）。

## モデルが省いたもの

- 課題は 1 つ、PR も 1 つです。レビューの回（round・`review_rounds`・extra）、`queued`、PR の無い課題（調べる課題・Chief Engineer・トリアージ・オーナーの課題）、レビュアーの子課題そのものの一生は入れていません。
- commit は「いまの head」と「それより古いもの」の 2 つだけを見ます。古い commit が何本あっても 1 つにまとめ、動いているレビュアーは見ている commit の集まり（`reviewing`）で表します。CI は head の 1 つの状態（`ci`）で、job ごとの区別はありません。担当が push するのは直すことがあるとき（CI が落ちた・Must fix・gate の直し）だけです。
- レビューの結果は、Must fix 以上があるかどうかだけです（`result`）。レビュアーの指摘を担当が見送る道、gate が同じ head で 2 度目の Must fix をオーナーに回す道は、gate の答えを選ぶ自由さ（fix・needs_human）にまとめました。
- 二重に走らない守り（dispatcher の lock・控え）、gate の rate limit、GitHub に聞く間隔（1 分に 1 回）は入れていません。gate の wait は「CI が走っている間は approve しない」で表します。通知の配達は「Chief of Staff に届いた」の 1 ビットです。
- gate の問い（from: gate）の受け取りは入れていません。問いの変数 `q` は担当の問いだけを表します。`andon issue submit` は前の head について出た gate の問いを数えず、gate が今の head で決めたときに merge が閉じるので、担当が `doing` に戻って head を動かしたあとの `WorkerReady` は gate の問いで止まりません。
- resume は 1 つの死に 1 回までですが、モデルは毎回起こし直します。worktree に commit していない変更があると finish・settle が止まる道も省きました。
- 「いつか必ず片付く」の形の規則（公平さの仮定が要るもの）は置いていません。
