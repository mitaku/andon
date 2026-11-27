# 報告と ready

担当が走らせた記録を撮り（`andon issue record`）、報告を書き（`andon issue report`）、終わったと kanban に返す（`andon issue ready`）道。`ready` は報告・PR の head・問い・レビューを確かめてから課題を `review` に入れ、足りなければ理由1行で断って課題を `doing` のままにする。

## 細かい機能

- `rr-record` 走らせた記録 `runs/<n>.json` と出力 `runs/<n>.log` が残り、commit と commit していない変更の有無を持つ。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/record.go`、`internal/contract/runs.go`、`schema/run.schema.json`
- `rr-ready-refuse` 報告の無い `ready` は断り、課題は `doing` のまま。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/transition.go`、`internal/process/process.go`、`process/states/issue.yaml`
- `rr-report` 報告が `report.json`（commit は HEAD・`pr`・`acceptance[].run`）と `report.md` になる。PR の branch が課題の `branch` と違えば（ほかの課題の PR）、`report --pr` は書かずに断り、前の報告から引き継ぐ pr なら外す。人が読む報告に見出し「## Report」の行が無ければ（本文が消えて PR の行だけが残ったもの）、`report` も、前の報告を引き継ぐ `review-request` も、書かずに断る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/issue.go`、`internal/contract/contract.go`、`internal/contract/schema.go`、`schema/report.schema.json`
- `rr-ready-report` 報告の課題（Chief of Staff が `deliverable=report` を書いた課題）で `pr` の無い報告の `ready` は、`kanban review <id>` で返すと案内して断り（`--pr` を案内しない）、課題は `doing` のまま。手で打った `andon gate` も回さずに終了コード 3 で、gate の記録を書かない（ADR-0066 決めたこと 8）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/issue.go`、`internal/ledger/ledger.go`、`internal/gate/gate.go`
- `rr-acceptance` 課題の予約キー `acceptance` が名指す受け入れ条件の番号のうち、報告に行の無い番号だけを名指して `ready` が断る。番号は数字だけの形（`1`）と A を付けた形（`A1`）を同じ番号と読む。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/issue.go`、`internal/contract/runs.go`
- `rr-stacked` Chief of Staff の端末が `stacked_on`・`stacked_base` を書いた課題（積んだ課題）は、`after` の前の課題が `review` のままでも `kanban start` で `doing` に入り、`after` は残る。担当の端末が自分で書いたキーは信じず、前の課題を待つ相手に数えて断る（ADR-0067 決めたこと 5）。見る道: 検証の担当だけ。
  - 見るファイル: `cmd/kanban/ledger.go`、`internal/ledger/stacked.go`、`internal/ledger/ledger.go`、`schema/issue-keys.schema.json`
- `rr-restack` 積んだ課題の前の課題が `done` になると、`ready` は「先に andon issue restack で載せ替える」と断る。担当が `andon issue restack <id>` を打つと、自分の commit だけが既定の branch の先に載り、PR の向き先の付け替えが push より先に打たれ、draft が外れ、`stacked_on`・`stacked_base`・`restack_onto` が消える。前の課題がまだ `done` でなければ何も変えずに 0（ADR-0067 決めたこと 12・13）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/issue/restack.go`、`internal/issue/issue.go`、`internal/issue/leaves.go`、`internal/cli/leaves.go`
- `rr-ready` 揃った報告の `ready` で課題が `review` に入り、kanban の履歴に担当の端末の行が残る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/issue.go`、`internal/issue/leaves.go`、`internal/issue/transition.go`、`internal/ledger/ledger.go`、`internal/ledger/id.go`、`internal/ledger/config.go`、`internal/ledger/review.go`、`internal/gate/review.go`、`internal/gate/gate.go`、`cmd/andon/main.go`、`internal/cli/cli.go`、`internal/cli/leaves.go`、`cmd/kanban/ledger.go`、`cmd/kanban/lock_unix.go`、`cmd/kanban/main.go`、`cmd/kanban/paths.go`、`cmd/kanban/rename_other.go`、`cmd/kanban/yamldoc.go`

## 辿り着き方（利用者から）

- 担当が自分の worktree で `andon issue record <id> -- <command>` を打つ。
- 担当が `andon issue report <id> --pr <url> --data '<json>'` に報告の Markdown を標準入力で渡す。
- 担当が `andon issue ready <id>` を打つ。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 自分の課題が `doing`（記録を撮るため）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh report-ready <検証する commit>` を打つ。切り離した kanban に課題 `V-0001` を作り、担当の端末 `tw` として7つの細かい機能を順に動かす。
- **記録を撮る。** `rr-record` の行に `andon issue record V-0001 -- git -C <worktree> log -1 --format=%H` が出る。`runs/1.json` の JSON（`exit_code` 0・`commit` が HEAD・`dirty` false）と `ok   rr-record` が見えれば動いた。
- **断られる。** `rr-ready-refuse` の行に `andon issue ready` の終了コード 1 と理由1行（`report.json が無い`）が出る。`ok   rr-ready-refuse` が見えれば動いた。
- **報告する。** 課題には `branch` を書いてある。偽の `gh` が初めにほかの branch（`other/branch`）を返すので、1回目の `rr-report` の行に `report --pr` の終了コード 1 と理由1行（`… この課題の PR でない …`）が出て、`report.json` は書かれない。続く `rr-report` の行で、ほかの課題の PR を入れた `report.json` を `--pr` を付けずに書き直し、「この課題の PR でないので引き継がない」の1行と、`pr` が空の `report.json` が出る。偽の `gh` を課題の branch に替えたあと、PR の行だけの報告を渡す `rr-report` の行に、終了コード 1 と理由1行（`… 見出し「## Report」の行が無い …`）が出て、`report.json` は変わらない。続けて `report.md` を PR の行だけに書き換えて `review-request` を stdin なしで打つ `rr-report` の行にも、同じ理由1行が出て、`report.json` は変わらない。最後の `rr-report` の行に、`report.json` の `commit`・`pr`・`acceptance` が出る。どちらも `ok   rr-report` が見えれば動いた。
- **報告の課題で断られる。** `pr` の無い報告を書いたあと、`kanban set V-0001 deliverable=report`（Chief of Staff の端末 `tc` として）のあと `andon issue ready` と `andon gate` を打つ。`rr-ready-report` の行に、`ready` の終了コード 1 と理由1行（`deliverable: report の課題で PR が無いときは kanban review V-0001 で返す`）、`gate` の終了コード 3 と理由1行（`deliverable: report の課題で PR が無い。gate を回すものが無い`）が出る。続けて担当の端末 `tw` が `deliverable=report` を書き直すと、`ready` も `gate` も PR の課題として `report.json に pr が無い` 側の理由に「`deliverable: report` を書いたのが Chief of Staff でない」を添えて断る（`gate` は記録を書かない）。`ok   rr-ready-report` が見えれば動いた。そのあと Chief of Staff の端末 `tc` が `deliverable=report` を書き直し、`pr` のある報告で `rr-ready` まで続ける（`pr` があれば PR の課題と同じ確かめ）。
- **受け入れ条件の番号で断られる。** 報告の行は `A1` だけのまま、`kanban set V-0001 acceptance=1,2`（Chief of Staff の端末 `tc` として）のあと `andon issue ready` を打つ。`rr-acceptance` の行に終了コード 1 と、足りない番号として `2` だけを名指す理由1行（`… review にしない: 2。…`）が出る。`ok   rr-acceptance` が見えれば動いた。そのあと予約キーを `acceptance=1` に書き直す。
- **ready する。** `rr-ready` の行のあとに `kanban show` の `status` が `review`・`acceptance` が `1`、`log.jsonl` に `terminal:tw` の `{"status":"review"}` が出る。`ok   rr-ready` が見えれば動いた。`ready` はレビューが要るかを gate と同じ関数で決めるので、最後の `gh.calls` に PR の head・PR のファイル（`gh api … /pulls/7/files`）・差分（`gh pr diff`）の3行が出る。偽の `gh` は文書1行のレベル A の差分を返し、`review: off` のまま通る。
- **積んだ課題を起動する。** `rr-ready` のあと、前の課題を `review` に置き、それを `after` に持つ課題を2つ足す。1つは担当の端末 `tw` が `stacked_on`・`stacked_base` を書き、`rr-stacked` の行に `kanban start` の終了コード 1 と理由1行（`… waits for <前の課題> …`）が出て、課題は `queued` のまま。もう1つは Chief of Staff の端末 `tc` が書き、`kanban start` が通って `status` が `doing`、`after` が前の課題のまま。`log.jsonl` の `terminal:tc` の `stacked_on` の行と `terminal:tw` の `{"status":"doing"}` の行が出る。両方の `ok   rr-stacked` が見えれば動いた。
- **片付く。** 最後に `ok   片付けた` が出る。
- **載せ替える（`rr-restack`）。** 通しの道は打たない。本物のセルで、Chief of Staff が積んだ課題を試しに起動し、前の課題が main に入って載せ替えの直しが届いたら、担当が restack の前に打った `andon issue ready <id>` が「先に andon issue restack <id> で既定の branch の先へ載せ替えて」の 1 行で断り（検証の担当が担当に頼んで打ってもらう）、`andon issue restack <id>` が「ふつうの課題になった」の 1 行で終わり、`kanban show <id>` に `stacked_on`・`stacked_base`・`restack_onto` が無く、`gh pr view <PR> --json baseRefName,isDraft` が既定の branch と `false` を返せば動いた。

## 変えたら動かすパス

- `internal/issue/`
- `internal/contract/`
- `internal/ledger/`
- `schema/report.schema.json`
- `schema/run.schema.json`
- `schema/issue-keys.schema.json`
- `process/states/issue.yaml`
- `cmd/kanban/`
- `cmd/andon/`
- `internal/cli/`
- `internal/gate/review.go`
- `internal/gate/gate.go`
- `internal/process/`

## 既に見ている検査

- `tests/andon-issue.sh`（同じ道を checkout から焼いた andon で、断る場合も含めて見る）
- `tests/schema.sh`（`report.json` と `runs/<n>.json` の形）
- `go test ./internal/issue/`

## 落とし穴

- `rr-report` の断りは課題に `branch` があるときだけ効く。`branch` の無い課題（repo の無い課題）では照らさないので、段から `branch` を外すと断りの行が落ちる。
- 偽の `gh` が知らない呼び出しを受けると終了コード 99 で落ちる。`ready` が新しく `gh` を呼ぶようになったら、`rr-ready` が落ち、出力の `gh.calls` に呼び出しが出る。地図と `scripts/sandbox.sh` を直す。
- `rr-acceptance` と `rr-ready` は、予約キーの番号を数字だけ（`1`）、報告の行を A を付けた形（`A1`）で書く。両方を同じ形で書くと、番号の読み方が変わっても落ちず、確かめにならない。
- `rr-ready` は `review: off`・`merge: person` の課題で通す。レビューの照合は `tests/andon-issue.sh` が見ており、ここでは見ない。
- 偽の `gh` が返す PR のファイルと差分は、`journeys/report-ready.sh` が置く文書1行（レベル A）。条件表（`/opt/andon/gate.json`）で `docs/**` がレベル A でなくなると、`ready` がレビューを求めて断り `rr-ready` が落ちる。そのときは偽の差分をレベル A のファイルに替える。
- 記録を撮る自分の課題の worktree に commit していない変更があると、自分の記録の `dirty` が true になる。証拠を撮る前に commit する。
