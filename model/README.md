# model/ — 課題の一生のモデルと規則

課題 1 つの一生（台帳に入ってから片付くまで）を TLA+ で写したモデルと、その一生が守る規則です。検査器 [tla-ts](https://github.com/aidenybai/tla-ts) が、届く全部の状態で規則が成り立つかを調べます。決めた経緯は [ADR-0025](../docs/adr/0025-formal-model-in-repo.md) にあります。

モデルが確かめるのは設計（規則と動き）です。コードの退行は、今までどおり `tests/` の筋書きのテストが確かめます。モデルとコードを機械で照らすことはしません。モデルは Go のコードを読んで手で写したもので、写したコードの場所を行動ごとに書いてあります。

## 回し方

```sh
model/check
```

手元も CI（`.github/workflows/model.yml` の job `model`）も、この 1 つを打ちます。要るのは Node と npm だけです（版は repo の `mise.toml`）。検査器は `package-lock.json` のとおりに一時の場所へ入れ、install のスクリプトは走らせません。モデルの写しも一時の場所で回すので、repo にも HOME にも書きません。

`model/check` が確かめること:

1. 本物のモデル（`IssueRules.cfg`）が規則を全部守り、どの状態からも先がある。調べる規則は控え（`expected.txt` の `rule`）と同じ集合でなければならない（規則を設定から消すと落ちる）。
2. 状態の数が控え（`expected.txt` の `states`）と同じ。
3. 壊した見本（`broken/`）がどれも、控え（`expected.txt` の `broken`）に書いた規則の破れで落ちる。

2 と 3 は、検査器の誤りで「破れているのに通る」を見つけるための確かめです。

TLC で見比べるときは、TLC と同じ旗を受けるコマンドを `MODEL_CHECKER` に渡します（例: `MODEL_CHECKER="java -cp tla2tools.jar tlc2.TLC" model/check`）。モデルと設定のファイルは TLC と共通です。

## ファイル

| ファイル | 中身 |
|---|---|
| `IssueLife.tla` | 状態の変数と行動（コマンド・dispatcher・人の動き）。行動ごとに写したコードの場所 |
| `IssueRules.tla` | 規則 R1〜R6（`IssueLife` を読む） |
| `IssueRules.cfg` | 検査器の設定（調べる規則の並び） |
| `expected.txt` | 調べるはずの規則・状態の数の控えと、見本ごとの落ちるはずの規則 |
| `broken/` | 直す前の動きに戻した見本（A-0002・A-0003・A-0006 で直した動き） |
| `check` | 入口 |
| `package.json`・`package-lock.json` | 検査器の版 |

## 規則

| 名前 | 中身 |
|---|---|
| R1 `R1_NoWorkerAfterCancel` | やめると決めた課題で、担当がまた動き出すことはない |
| R2 `R2_FixReachesSomeone` | ゲートが直しを返したら、担当か Chief of Staff のどちらかが必ずそれを知る |
| R3 `R3_CleanupIsWhole` | worktree を片付けたら、課題は done になっている（片付けは全部か無し） |
| R4 `R4_DoneMeansMergedOrCancelled` | done の課題は、マージしたか、やめると決めたものだけ |
| R5 `R5_NoWorkerWhenDone` | done の課題に担当は残らない |
| R6 `R6_NoReviewWithOpenQuestion` | 担当の問いが開いたまま review にならない |

## 変えるとき

- 課題の一生の動き（finish・resume・ゲートの返し方など、行動が写しているコード）を変えたら、`IssueLife.tla` の行動とコードの場所を直します。状態の数が変わったら、`model/check` を打って `expected.txt` の `states` を直します。
- 規則を足すときは `IssueRules.tla` に書き、`IssueRules.cfg` の `INVARIANT` と `expected.txt` の `rule` に名前を足します。

## モデルが省いたもの

- 課題は 1 つ、PR も 1 つです。レビューの回（round・`review_rounds`・extra）、`queued`、PR の無い課題（調べる課題・Chief Engineer・トリアージ・人の課題）、レビュアーの子課題そのものの一生は入れていません。
- 二重に走らない守り（dispatcher の lock・控え）、ゲートの wait・rate limit・checks 待ち・head が動いたときの回し直しは入れていません。通知の配達は「Chief of Staff に届いた」の 1 ビットです。
- ゲートの問い（from: gate）の受け取りは入れていません。問いの変数 `q` は担当の問いだけを表します。
- resume は 1 つの死に 1 回までですが、モデルは毎回起こし直します。worktree に commit していない変更があると finish・settle が止まる道、settle の CLOSED も省きました。
- 「いつか必ず片付く」の形の規則（公平さの仮定が要るもの）は置いていません。
