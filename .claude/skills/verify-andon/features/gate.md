# gate

課題が `review` に入ると、dispatcher が `andon gate <id>` を回し、PR の head・checks・レビュー・委譲・条件表（`/opt/andon/gate.json`）を照合して、判定（`approve`・`fix`・`needs_human`）とレベル（A・B・C）を記録に書く。`merge: auto` で委譲の範囲の中の `approve` は dispatcher がマージし、`fix` は担当へ、`needs_human` はオーナーへの問いになる。

## 細かい機能

- `gt-record` 当てたあとの gate の記録（`library/gate/<head>.json`）が、判定・レベル・head・理由の形を持つ。見る道: 検証の担当だけ。
  - 見るファイル: `internal/gate/gate.go`、`internal/gate/decide.go`、`internal/gate/comment.go`、`internal/gate/mark.go`、`internal/gate/review.go`、`package/gate.json`
- `gt-route` 判定ごとに課題が動いた（`approve` はマージかオーナーへ、`fix` は `doing`、`needs_human` は問い）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/merge.go`、`internal/issue/finish.go`、`internal/issue/writeback.go`、`internal/dispatcher/route.go`、`internal/contract/merge.go`、`internal/delegate/delegate.go`、`process/states/gate.yaml`
- `gt-answer` dispatcher が閉じた問いは gate の問い（`from: gate`）だけで、dispatcher が書いた答えは印が `machine`、「決め直した」の答えが名指す gate の記録が在る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/settle.go`、`internal/contract/settle.go`、`internal/dispatcher/humanmerge.go`、`process/states/human-merge.yaml`
- `gt-skip` 当てたあとに PR 無しで `review` に入った課題（検証・マージ後のレビュー）のうち、報告の commit が既定の branch に入っているもの（自分の commit が無い）に、gate が fix を返していない（kanban の履歴で、`review` に入った行の次の status の行が同じ担当の端末の `doing` でない。`<slug>.merge.json` にその行の分が残っていれば、`decision` が `fix` でない）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/merge.go`、`process/states/gate.yaml`

## 辿り着き方（利用者から）

- 担当が `andon issue ready <id>` で `review` に入れると、dispatcher の見回りが gate を回す。
- Chief of Staff かオーナーが `andon gate <id>` を打つ（記録を書く。マージはしない）。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、gate が1回以上回っている。無ければ `gt-record`・`gt-route` を読まずに note を出し、`gt-answer` だけを読んで終了コード 3（未確認）。dispatcher はオーナーのマージでも問いを閉じるので、gate の記録が無い間にも `gt-answer` の証拠は出る。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh gate <検証する commit>` を打つ。セルの本物の `projects/*/kanban/issues/*/library/gate/*.json` を読むだけ。
- **記録を読む。** `gt-record` の行に、当てたあとの gate が1行ずつ（課題・PR・レベル・判定・理由の数・checks・レビュー）出る。`ok   gt-record` が見えれば動いた。
- **行き先を読む。** `gt-route` の行に、課題ごとの判定と今の `status` が出る。`approve` で `merged` が true なら課題は `done` か片付けの途中、`fix` なら `doing`、`needs_human` なら `review` のままオーナーの問いがある。食い違えば、`kanban show <id>` と `log.jsonl` を読んで報告に書く。
- **閉じた問いを読む。** `gt-answer` の行に、検証する間に dispatcher が閉じた問い（kanban の `log.jsonl` で `by` が `terminal:dispatcher` の `received`）が1件ずつ（問いの `from`・答えた端末・印・答え）出る。`ok   gt-answer` が見えれば動いた。閉じた問いが無ければ `note gt-answer` が出て、gt-answer は未確認になる（終了コードは変えない）。gate の記録が無い回は、`ok   gt-answer` が見えても終了コードは 3 のままなので、報告には gt-answer だけを動いたと書き、gt-record・gt-route は未確認と書く。
- **PR 無しの課題を読む。** `gt-skip` の行に、当てたあとに PR 無しで `review` に入った、自分の commit の無い課題が1件ずつ（報告の commit・`review` にした端末・次の status の行・merge の記録の `decision`）出る。`ok   gt-skip` が見えれば動いた。該当する課題が無ければ `note gt-skip` が出て、gt-skip は未確認になる（終了コードは変えない）。検証の課題そのものが PR 無しで返るので、自分の課題が `review` に入ったあとに打ち直せば読める。
- **印を読む。** `drive.sh` は GitHub の印を読まない。`internal/gate/mark.go` を変えた差分では、記録の head ごとに `gh api repos/<owner>/<repo>/commits/<head>/statuses --jq '.[] | select(.context == "andon/gate") | [.created_at, .state, .description] | @tsv'` で印を読む。`approve` と `needs_human` は `success`（緑）、`fix` は `failure`（赤）で、説明文の見出しが判定に合えば動いた。

## 変えたら動かすパス

- `internal/gate/`
- `internal/delegate/`
- `package/gate.json`
- `internal/issue/merge.go`
- `internal/issue/settle.go`
- `internal/dispatcher/route.go`
- `internal/contract/merge.go`
- `process/states/gate.yaml`
- `internal/issue/finish.go`
- `internal/issue/writeback.go`
- `internal/contract/settle.go`
- `internal/dispatcher/humanmerge.go`
- `process/states/human-merge.yaml`

## 既に見ている検査

- `tests/andon-gate.sh`（偽の gh で判定とレベルの全部の枝を見る）
- `tests/andon-dispatcher.sh`（判定のあとの通知とマージ）
- `go test ./internal/gate/`

## 落とし穴

- `gate` は `andon gate` を本物の課題に打つと、その課題の記録と GitHub の status を書く。検証の担当は打たない。読むだけにする。
- gate が回っても記録（`library/gate/<head>.json`）を書かない回がある。`report.json` に `pr` が無い回と、PR の branch が課題の `branch` と違う回で、gate は理由の `fix` を担当へ送るが、記録も PR の印も書かない。PR が無く、報告の `commit` が既定の branch に入っている課題（検証・マージ後のレビューのように PR を出さない課題）には、gate を回さない（merge の記録 `<slug>.merge.json` にも残らない）。記録が無いことを、gate が回っていない証拠にしない。kanban の `log.jsonl`（`review` に入った行のあとの `doing` の行）、dispatcher のログ、担当の画面（`[gate]` の印の付いた fix）も読み、報告には「gate は回ったが記録が無い」と書き分ける。
- `gt-skip` の「担当が `doing` に戻した」は、gate の fix を受けた形のほかに、Chief of Staff が直しを頼んで担当が戻した場合にも出る。落ちたら担当の画面で、`[gate]` の印の付いた fix が届いていたかを読んで書き分ける。merge の記録は課題が片付くと消えるので、片付いた課題の `decision` は「無し」と出る。
- dispatcher が問いを閉じるのは2つの場合だけ。1つは、gate がオーナーへ回した PR の head が動いたとき（答えは「PR の head が動いた（前 → 今）」で始まる）。もう1つは、オーナーが手でマージしたとき（答えは「オーナーがマージした」で始まる）。どちらも起きなければ `gt-answer` は未確認になる。オーナーが先に答えていた問いは、dispatcher が受け取るだけで答えを上書きしないので、答えた端末はオーナーのまま。
- 同じ head の gate は記録を上書きする。`at` が当てた時刻より後でも、判定は最後の1回のもの。
- `needs_human` の多くは条件表どおりのレベル C（オーナーがマージ）で、壊れではない。`tier_reasons` を読んで書き分ける。
- 検証の途中で次の commit が当たっても、`drive.sh gate <検証する commit>` の2つ目の引数は検証の課題の commit のまま変えない。`/opt/andon/applied.json` の `previous` から、検証する commit の dispatcher が動いていた間（dispatcher が切り替わった時刻 `switched_at` から、次の commit で切り替わった時刻まで）の gate だけを読む。gate は dispatcher の中で回るので、次の commit を当ててから前の dispatcher が見回りを終えるまでの gate は、前の commit のコードの結果。
- 記録に検証する commit が無い（`previous` を持たない前のバージョンの記録か、古くて残っていない）と、`drive.sh` は終了コード 3 で終わる。そのときは `library/gate/*.json` の `at` を手で挟んで読み、読んだ command を `andon issue record` で撮る。下の端には、検証する commit の PR が GitHub でマージされた時刻（`gh api repos/<owner>/<repo>/commits/<検証する commit>/pulls --jq '.[0].merged_at'`）を使う。当てるのは main に入った commit なので、この時刻は当てた時刻より前になり、窓は少し広くなる。dispatcher の `<slug>.recorded.jsonl` の `merged` の行の `at` は使わない。dispatcher がマージに気づいた時刻で、オーナーが手でマージした PR では当てた時刻より後になることがあり、その間の記録を落とす。広がった分に入った記録は、報告に書き分ける。
