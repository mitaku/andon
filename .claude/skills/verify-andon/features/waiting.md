# オーナー待ち

Chief of Staff がオーナーの判断を要る問いを `andon issue ask --new` で出すと、オーナーの課題（`who: person`）が足され、問いが置かれる。同じタイトルの片付いていない課題でも、親が違えば取り違えない。オーナーと役は `andon waiting` で、マージの依頼・判断・手作業のオーナー待ちと並行の枠を読む。

## 細かい機能

- `ow-ask-new` `andon issue ask --new --parent <id>` で、オーナーの課題が `who: person`・その親・「（オーナーの判断待ち）」のタイトルで足され、問いが `library/questions/1.json` に置かれる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/ledger/ask.go`、`cmd/kanban/questions.go`、`schema/question.schema.json`
- `ow-same-title` 同じ問いでも親が違えば別のオーナーの課題になる。同じ親で打ち直すと断り、課題を増やさない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/ledger/ask.go`
- `ow-waiting` `andon waiting --json` と人が読む出力に、足したオーナーの課題が答えられる判断の行として出る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/waiting/waiting.go`、`internal/waiting/cmd.go`、`internal/waiting/open_unix.go`、`internal/waiting/open_other.go`、`schema/waiting.schema.json`
- `ow-pane` Herdr App のオーナー待ちのパネル（`package/host/waiting/`）に、オーナーの課題が判断の行として出て、選択肢で答えられる。見る道: オーナーが見る。
  - 見るファイル: `internal/waiting/answer.go`

## 辿り着き方（利用者から）

- Chief of Staff か振り返り・トリアージの担当が `andon issue ask --new [--parent <id>] [--after <id>,…] -` に問いの JSON を渡す。
- オーナーか役が `andon waiting [--json]` を打つ。Herdr App のオーナー待ちのパネルは同じ出力を描く。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh waiting <検証する commit>` を打つ。切り離した kanban に親の課題を2つ作り、Chief of Staff の端末 `tc` として、同じ問いを親を変えて出す。
- **オーナーの課題を足す。** `ow-ask-new` の行に `<id>#1` と、足した課題の `who`・`parent`・タイトルが出る。`ok   ow-ask-new` が見えれば動いた。
- **取り違えない。** `ow-same-title` の行に、親の違う2つ目の課題の id と、同じ親で打ち直したときの「同じ問いのオーナーの課題 … がもうある」が出る。`ok   ow-same-title` が2行見えれば動いた。
- **オーナー待ちを読む。** `ow-waiting` の行に、`andon waiting --json` の枠と判断の行、人が読む出力が出る。`ok   ow-waiting` が2行見えれば動いた。
- **パネルを見る（`ow-pane`）。** 本物のオーナーの課題がオーナー待ちにある間に、オーナーが Herdr App のパネルで見る。報告の `acceptance[]` に `manual` と書き、見る場所（Herdr App のオーナー待ちのパネル）を `evidence` に書く。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/waiting/`
- `internal/ledger/ask.go`
- `cmd/kanban/questions.go`
- `schema/waiting.schema.json`
- `schema/question.schema.json`

## 既に見ている検査

- `tests/andon-waiting.sh`（バイナリの入口・人が読む行・何も書かないことを見る）
- `tests/andon-issue.sh`（`ask --new` の出力・`who: person`・打ち直し）
- `go test ./internal/waiting/`（マージの依頼・判断・手作業の数え方と枠）と `go test ./internal/ledger/`

## 落とし穴

- 判断の行に出るのは灯りの種類（`business`・`users`・`promise`・`irreversible`・`line`）の問いだけ。`scope` の問いは `ask --new` が断る。
- `andon waiting` の枠の「オーナー待ち」（`slots.waiting`）は、`doing` か `review` の担当の課題のうちオーナー待ちの行を持つものの数で、オーナーの課題は数えない。判断の行が2つあっても 0 のことがある。
- 振り返りの apply が足すオーナーの課題（`internal/retro/`）は、この道を通らない。
