# 流れの数え

オーナーか役が `andon measure` を打つと、kanban の記録から期間の流れ（待ち時間・流量・WIP）と効率（介在回数・自動マージ率・gate の差し戻し率）と対抗の指標と、通知の素通りの数と、課題ごとのトークン数を数えて出す。読むだけで、何も書かない。

## 細かい機能

- `ms-flow` `--json` の流れの数（期間に片付いた数・期間の終わりの WIP）が、kanban の履歴どおりになる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/measure/measure.go`、`internal/measure/estimate.go`、`internal/ledger/worked.go`、`internal/ghpr/ghpr.go`、`schema/measure.schema.json`
- `ms-question-later` 問いが開いていた時間の平均（`flow.question_open`）は、置いておける課題（Chief of Staff が印 `later` を書いたオーナーの課題）の問いを入れない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/measure/measure.go`、`internal/waiting/waiting.go`
- `ms-tokens` 課題の担当のセッションの記録（Claude Code）から、トークンを入力・出力・キャッシュの読み書きに分けて数える。記録の無い課題は `null`。見る道: 検証の担当だけ。
  - 見るファイル: `internal/measure/tokens.go`、`internal/sessionlog/sessionlog.go`
- `ms-table` 人向けの表が「価値:」の行で始まる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/measure/measure.go`
- `ms-readonly` 打つ前と後で、kanban とセッションの記録が変わらない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/measure/measure.go`

## 辿り着き方（利用者から）

- オーナーか役が、プロジェクトの置き場（`projects/<slug>/`）か担当の worktree で `andon measure [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]` を打つ。
- `andon digest collect` が、同じ数え方で直近 7 日とその前の 7 日を数える（`features/digest.md`）。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh measure <検証する commit>` を打つ。切り離した kanban に、片付いた課題（todo → doing → review → done）と doing のまま残る課題を作り、片付いた課題の担当のセッションの記録を一時の HOME の `.claude/projects/` に置いてから打つ。
- **流れを読む。** `ms-flow` の行に `andon measure --json` の出力と、`closed`・`wip_end` の要約が出る。`closed.count` が 1、`wip_end` が 1 で、`ok   ms-flow` が見えれば動いた。
- **置いてある問いを外す。** 打つ前に、Chief of Staff の端末が呼ぶ判断（`andon issue ask --new --now`）と置いておく判断（`--later`）を 1 件ずつ足してある。どちらの問いも開いたまま。`ms-question-later` の行に `flow.question_open` が出る。`count` が 1 で、`ok   ms-question-later` が見えれば動いた。
- **トークンを読む。** `ms-tokens` の行に課題ごとの `tokens` が出る。片付いた課題が入力 11・出力 22・キャッシュの読み 33・書き 44、残った課題が `null` で、`ok   ms-tokens` が見えれば動いた。
- **表を読む。** `ms-table` の行に人向けの表が出る。`ok   ms-table` が見えれば動いた。
- **書かないことを確かめる。** `ok   ms-readonly` が見えれば、打つ前と後で kanban とセッションの記録の sha256 が同じ。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/measure/`
- `internal/ledger/`
- `internal/sessionlog/`
- `internal/ghpr/`
- `internal/waiting/waiting.go`
- `schema/measure.schema.json`

## 既に見ている検査

- `tests/andon-measure.sh`（偽の gh で、バイナリの入口・表・終了コードを見る）
- `go test ./internal/measure/`（数え方の細部を、手で数えた値と突き合わせる）

## 落とし穴

- 切り離した kanban の課題は PR を持たないので、`gh pr list` は呼ばれず、マージに関わる指標（リードタイム・自動マージ率・介在回数）は「記録なし」になる。これらの数え方は `go test ./internal/measure/` が見ている。`ms-flow` はリードタイムを見ない。
- 期間は UTC の日付で、既定は今日までの 7 日。日付が変わる前後に打つと、作った履歴が期間の外に落ちることがある。そのときは打ち直す。
- セッションの記録の置き場は `CLAUDE_CONFIG_DIR` が無ければ `~/.claude`。切り離した道は HOME を一時に向けるので、セルの本物のセッションの記録は読まない。
