# gate

課題が `review` に入ると、dispatcher が `andon gate <id>` を回し、PR の head・checks・レビュー・委譲・条件表（`/opt/andon/gate.json`）を照合して、判定（`approve`・`fix`・`needs_human`）とレベル（A・B・C）を記録に書く。`merge: auto` で委譲の範囲の中の `approve` は dispatcher がマージし、`fix` は担当へ、`needs_human` はオーナーへの問いになる。

## 細かい機能

- `gt-record` 当てたあとの gate の記録（`library/gate/<head>.json`）が、判定・レベル・head・理由の形を持つ。見る道: 検証の担当だけ。
- `gt-route` 判定ごとに課題が動いた（`approve` はマージかオーナーへ、`fix` は `doing`、`needs_human` は問い）。見る道: 検証の担当だけ。

## 辿り着き方（利用者から）

- 担当が `andon issue ready <id>` で `review` に入れると、dispatcher の見回りが gate を回す。
- Chief of Staff かオーナーが `andon gate <id>` を打つ（記録を書く。マージはしない）。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、gate が1回以上回っている。無ければ終了コード 3（未確認）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh gate` を打つ。セルの本物の `projects/*/kanban/issues/*/library/gate/*.json` を読むだけ。
- **記録を読む。** `gt-record` の行に、当てたあとの gate が1行ずつ（課題・PR・レベル・判定・理由の数・checks・レビュー）出る。`ok   gt-record` が見えれば動いた。
- **行き先を読む。** `gt-route` の行に、課題ごとの判定と今の `status` が出る。`approve` で `merged` が true なら課題は `done` か片付けの途中、`fix` なら `doing`、`needs_human` なら `review` のままオーナーの問いがある。食い違えば、`kanban show <id>` と `log.jsonl` を読んで報告に書く。

## 変えたら動かすパス

- `internal/gate/`
- `internal/delegate/`
- `package/gate.json`
- `internal/issue/merge.go`
- `internal/issue/settle.go`
- `internal/dispatcher/route.go`

## 既に見ている検査

- `tests/andon-gate.sh`（偽の gh で判定とレベルの全部の枝を見る）
- `tests/andon-dispatcher.sh`（判定のあとの通知とマージ）
- `go test ./internal/gate/`

## 落とし穴

- `gate` は `andon gate` を本物の課題に打つと、その課題の記録と GitHub の status を書く。検証の担当は打たない。読むだけにする。
- 同じ head の gate は記録を上書きする。`at` が当てた時刻より後でも、判定は最後の1回のもの。
- `needs_human` の多くは条件表どおりのレベル C（オーナーがマージ）で、壊れではない。`tier_reasons` を読んで書き分ける。
