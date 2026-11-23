# 報告と ready

担当が走らせた記録を撮り（`andon issue record`）、報告を書き（`andon issue report`）、終わったと kanban に返す（`andon issue ready`）道。`ready` は報告・PR の head・問い・レビューを確かめてから課題を `review` に入れ、足りなければ理由1行で断って課題を `doing` のままにする。

## 細かい機能

- `rr-record` 走らせた記録 `runs/<n>.json` と出力 `runs/<n>.log` が残り、commit と commit していない変更の有無を持つ。見る道: 検証の担当だけ。
- `rr-ready-refuse` 報告の無い `ready` は断り、課題は `doing` のまま。見る道: 検証の担当だけ。
- `rr-report` 報告が `report.json`（commit は HEAD・`pr`・`acceptance[].run`）と `report.md` になる。見る道: 検証の担当だけ。
- `rr-ready` 揃った報告の `ready` で課題が `review` に入り、kanban の履歴に担当の端末の行が残る。見る道: 検証の担当だけ。

## 辿り着き方（利用者から）

- 担当が自分の worktree で `andon issue record <id> -- <command>` を打つ。
- 担当が `andon issue report <id> --pr <url> --data '<json>'` に報告の Markdown を標準入力で渡す。
- 担当が `andon issue ready <id>` を打つ。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 自分の課題が `doing`（記録を撮るため）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh report-ready` を打つ。切り離した kanban に課題 `V-0001` を作り、担当の端末 `tw` として4つの細かい機能を順に動かす。
- **記録を撮る。** `rr-record` の行に `andon issue record V-0001 -- git -C <worktree> log -1 --format=%H` が出る。`runs/1.json` の JSON（`exit_code` 0・`commit` が HEAD・`dirty` false）と `ok   rr-record` が見えれば動いた。
- **断られる。** `rr-ready-refuse` の行に `andon issue ready` の終了コード 1 と理由1行（`report.json が無い`）が出る。`ok   rr-ready-refuse` が見えれば動いた。
- **報告する。** `rr-report` の行に `report.json` の `commit`・`pr`・`acceptance` が出る。`ok   rr-report` が見えれば動いた。
- **ready する。** `rr-ready` の行のあとに `kanban show` の `status` が `review`、`log.jsonl` に `terminal:tw` の `{"status":"review"}` が出る。`ok   rr-ready` が見えれば動いた。`ready` はレビューが要るかを gate と同じ関数で決めるので、最後の `gh.calls` に PR の head・PR のファイル（`gh api … /pulls/7/files`）・差分（`gh pr diff`）の3行が出る。偽の `gh` は文書1行のレベル A の差分を返し、`review: off` のまま通る。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/issue/`
- `internal/contract/`
- `internal/ledger/`
- `schema/report.schema.json`
- `schema/run.schema.json`
- `process/states/issue.yaml`
- `cmd/kanban/`
- `cmd/andon/`
- `internal/cli/`
- `internal/gate/review.go`
- `internal/gate/gate.go`

## 既に見ている検査

- `tests/andon-issue.sh`（同じ道を checkout から焼いた andon で、断る場合も含めて見る）
- `tests/schema.sh`（`report.json` と `runs/<n>.json` の形）
- `go test ./internal/issue/`

## 落とし穴

- 偽の `gh` が知らない呼び出しを受けると終了コード 99 で落ちる。`ready` が新しく `gh` を呼ぶようになったら、`rr-ready` が落ち、出力の `gh.calls` に呼び出しが出る。地図と `scripts/sandbox.sh` を直す。
- `rr-ready` は `review: off`・`merge: person` の課題で通す。レビューの照合は `tests/andon-issue.sh` が見ており、ここでは見ない。
- 偽の `gh` が返す PR のファイルと差分は、`journeys/report-ready.sh` が置く文書1行（レベル A）。条件表（`/opt/andon/gate.json`）で `docs/**` がレベル A でなくなると、`ready` がレビューを求めて断り `rr-ready` が落ちる。そのときは偽の差分をレベル A のファイルに替える。
- 記録を撮る自分の課題の worktree に commit していない変更があると、自分の記録の `dirty` が true になる。証拠を撮る前に commit する。
