# gate

課題が `review` に入ると、dispatcher が `andon gate <id>` を回し、PR の head・checks・レビュー・委譲・条件表（`/opt/andon/gate.json`）を照合して、判定（`approve`・`fix`・`needs_human`）とレベル（A・B・C）を記録に書く。`merge: auto` で委譲の範囲の中の `approve` は dispatcher がマージし、`fix` は担当へ、`needs_human` はオーナーへの問いになる。

## 細かい機能

- `gt-record` 当てたあとの gate の記録（`library/gate/<head>.json`）が、判定・レベル・head・理由の形を持つ。見る道: 検証の担当だけ。
- `gt-route` 判定ごとに課題が動いた（`approve` はマージかオーナーへ、`fix` は `doing`、`needs_human` は問い）。見る道: 検証の担当だけ。
- `gt-answer` dispatcher が閉じた問いは gate の問い（`from: gate`）だけで、dispatcher が書いた答えは印が `machine`、「決め直した」の答えが名指す gate の記録が在る。見る道: 検証の担当だけ。

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
- **閉じた問いを読む。** `gt-answer` の行に、検証する間に dispatcher が閉じた問い（kanban の `log.jsonl` で `by` が `terminal:dispatcher` の `received`）が1件ずつ（問いの `from`・答えた端末・印・答え）出る。`ok   gt-answer` が見えれば動いた。閉じた問いが無ければ `note gt-answer` が出て、gt-answer は未確認になる（終了コードは変えない）。
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

## 既に見ている検査

- `tests/andon-gate.sh`（偽の gh で判定とレベルの全部の枝を見る）
- `tests/andon-dispatcher.sh`（判定のあとの通知とマージ）
- `go test ./internal/gate/`

## 落とし穴

- `gate` は `andon gate` を本物の課題に打つと、その課題の記録と GitHub の status を書く。検証の担当は打たない。読むだけにする。
- dispatcher が問いを閉じるのは2つの場合だけ。1つは、gate がオーナーへ回した PR の head が動いたとき（答えは「PR の head が動いた（前 → 今）」で始まる）。もう1つは、オーナーが手でマージしたとき（答えは「オーナーがマージした」で始まる）。どちらも起きなければ `gt-answer` は未確認になる。オーナーが先に答えていた問いは、dispatcher が受け取るだけで答えを上書きしないので、答えた端末はオーナーのまま。
- 同じ head の gate は記録を上書きする。`at` が当てた時刻より後でも、判定は最後の1回のもの。
- `needs_human` の多くは条件表どおりのレベル C（オーナーがマージ）で、壊れではない。`tier_reasons` を読んで書き分ける。
- 検証の途中で次の commit が当たったら、`drive.sh gate <検証する commit>` と2つ目の引数を渡す。`/opt/andon/applied.json` の `previous` から、検証する commit の dispatcher が動いていた間（dispatcher が切り替わった時刻 `switched_at` から、次の commit で切り替わった時刻まで）の gate だけを読む。gate は dispatcher の中で回るので、次の commit を当ててから前の dispatcher が見回りを終えるまでの gate は、前の commit のコードの結果。引数を省くと、最後に当てた commit のあとだけを読む。
- 記録に検証する commit が無い（`previous` を持たない前のバージョンの記録か、古くて残っていない）と、`drive.sh` は終了コード 3 で終わる。そのときは `library/gate/*.json` の `at` を手で挟んで読み、読んだ command を `andon issue record` で撮る。下の端には、検証する commit の PR が GitHub でマージされた時刻（`gh api repos/<owner>/<repo>/commits/<検証する commit>/pulls --jq '.[0].merged_at'`）を使う。当てるのは main に入った commit なので、この時刻は当てた時刻より前になり、窓は少し広くなる。dispatcher の `<slug>.recorded.jsonl` の `merged` の行の `at` は使わない。dispatcher がマージに気づいた時刻で、オーナーが手でマージした PR では当てた時刻より後になることがあり、その間の記録を落とす。広がった分に入った記録は、報告に書き分ける。
