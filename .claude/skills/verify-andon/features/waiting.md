# オーナー待ち

Chief of Staff がオーナーの判断を要る問いを `andon issue ask --new --now`（呼ぶ）か `--later "<理由>"`（置いておく）で出すと、オーナーの課題（`who: person`）が足され、問いが置かれる。同じタイトルの片付いていない課題でも、親が違えば取り違えない。オーナーと役は `andon waiting list` で、マージの依頼・判断・手作業のオーナー待ちと並行の枠を読む。Chief of Staff が予約キー `later`（急がない理由）を書いたオーナー待ちは、呼ばずに置いてある行として後ろに分かれ、`later_until` の日が来るとまた呼ぶ。

## 細かい機能

- `ow-ask-new` `andon issue ask --new --now --parent <id>` で、オーナーの課題が `who: person`・その親・「（オーナーの判断待ち）」のタイトルで足され、問いが `library/questions/1.json` に置かれる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/ledger/ask.go`、`internal/issue/leaves.go`、`cmd/kanban/questions.go`、`schema/question.schema.json`
- `ow-same-title` 同じ問いでも親が違えば別のオーナーの課題になる。同じ親で打ち直すと、同じ問いの課題があると言って断り、課題を増やさない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/ledger/ask.go`
- `ow-waiting` `andon waiting list --json` と人が読む出力に、足したオーナーの課題が答えられる判断の行として出る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/waiting/waiting.go`、`internal/waiting/cmd.go`、`internal/waiting/open_unix.go`、`internal/waiting/open_other.go`、`schema/waiting.schema.json`
- `ow-old-name` 名前を変える前の `andon waiting --json`（古いホストの部品が打つ形）は、`andon waiting list` に名前が変わったと1行で言って終了コード 2 で終わり、オーナー待ちを出さない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/cli/cli.go`、`internal/cli/leaves.go`
- `ow-slots` オーナー待ちの担当の課題が上限以上でも、動いている課題が上限より少なければ、`andon waiting list --json` の `slots.free` は上限から動いている課題の数を引いた正の数で、`slots` に `waiting_full` は無い。人が読む出力の枠の行にも、新しい課題を起動しないの文が無い。見る道: 検証の担当だけ。
  - 見るファイル: `internal/waiting/waiting.go`、`internal/waiting/cmd.go`、`schema/waiting.schema.json`
- `ow-later` Chief of Staff が `kanban set <id> later=<理由>` を書いたオーナーの課題は、`andon waiting list --json` で `call: false` と理由を持ち、呼んでいる行の後に並ぶ。人が読む出力は「置いてあるもの N 件」の見出しの下に置いた理由つきで出す。`later_until` の日が先なら置いたまま、来たらまた呼ぶ。担当の端末が書き直した印は信じない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/waiting/waiting.go`、`internal/waiting/cmd.go`、`internal/ledger/ledger.go`、`schema/issue-keys.schema.json`
- `ow-ask-later` `andon issue ask --new` は、`--now` か `--later "<理由>"` のどちらも無ければ理由を言って断る。Chief of Staff の端末の `--later --until <日>` は課題に `later` と `later_until` を書き、`andon waiting list --json` でその行が `call: false` になる。担当の端末の `--later` は断る。どの断りでも課題は増えない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/leaves.go`、`internal/ledger/ask.go`
- `ow-pane` Herdr App のオーナー待ちのパネル（`package/host/waiting/`）に、オーナーの課題が判断の行として出て、選択肢で答えられる。枠の行は、オーナー待ちが上限の数以上でも「Slots <working>/<limit> working · <waiting> waiting on you」の形のままで、「full」を出さず、色を変えない。見る道: オーナーが見る。
  - 見るファイル: `internal/waiting/answer.go`

## 辿り着き方（利用者から）

- Chief of Staff が `andon issue ask --new (--now | --later "<理由>" [--until <YYYY-MM-DD>]) [--parent <id>] [--after <id>,…] -` に問いの JSON を渡す。振り返りとトリアージの apply は同じ関数でオーナーの課題を足す。
- Chief of Staff が `kanban set <id> later=<理由>`（と `later_until=<YYYY-MM-DD>`）で、急がないオーナー待ちを置いておく。
- オーナーか役が `andon waiting list [--json]` を打つ。Herdr App のオーナー待ちのパネルは同じ出力を描く。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh waiting <検証する commit>` を打つ。切り離した kanban に親の課題を2つ作り、Chief of Staff の端末 `tc` として、同じ問いを親を変えて出す。
- **オーナーの課題を足す。** `ow-ask-new` の行に `<id>#1` と、足した課題の `who`・`parent`・タイトルが出る。`ok   ow-ask-new` が見えれば動いた。
- **取り違えない。** `ow-same-title` の行に、親の違う2つ目の課題の id と、同じ親で打ち直したときの「同じ問いのオーナーの課題 … がもうある」が出る。`ok   ow-same-title` が2行見えれば動いた。
- **呼ぶか置いておくかを言い切らせる。** `ow-ask-later` の行に、旗なしの断り（「呼ぶか置いておくかを言い切る」）、担当の端末 `tw` の `--later` の断り（「Chief of Staff の端末だけ」）、`--later --until` で足した課題の `andon waiting list --json` の行が出る。`ok   ow-ask-later` が3行見えれば動いた。
- **オーナー待ちを読む。** `ow-waiting` の行に、`andon waiting list --json` の枠と判断の行、人が読む出力が出る。`ok   ow-waiting` が2行見えれば動いた。
- **古い名前を断る。** `ow-old-name` の行に、`andon waiting --json` の終了コード 2 と「`andon waiting list` に名前が変わった」の1行が出る。`ok   ow-old-name` が見えれば動いた。
- **枠の空きを読む。** `ow-slots` の行に、担当の課題 2 つを担当の端末 `tw1`・`tw2` で `doing` にして判断の問いを出したあとの `andon waiting list --json` の `slots` が出る。オーナー待ちが上限（既定の 2）以上で、`free` が上限から動いている数を引いた正の数で、続く `andon waiting list` の枠の行に新しい課題を起動しないの文が無ければ、`ok   ow-slots` が 2 行見えれば動いた。
- **置いておく。** `ow-later` の行に、印を書いたあとの `andon waiting list --json` の行（`call`・`later`・`later_until`）が出る。印を書いた課題が置いてある行になり、先の日では置いたまま、来た日ではまた呼び、担当の端末 `tw` が書き直した印では呼ぶ。`ok   ow-later` が5行見えれば動いた。
- **パネルを見る（`ow-pane`）。** 本物のオーナーの課題がオーナー待ちにある間に、オーナーが Herdr App のパネルで見る。`package/host/waiting/` を変えたときは、mod を置き直したあと、オーナー待ちの数が `andon waiting list --json` の `slots.limit` 以上ある project でパネルを開き、枠の行に「full」が無く、待ちが上限より少ない project と同じ色であることも見る。報告の `acceptance[]` に `manual` と書き、見る場所（Herdr App のオーナー待ちのパネル）と、見た枠の行の文を `evidence` に書く。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/waiting/`
- `internal/ledger/ask.go`
- `internal/issue/leaves.go`
- `cmd/kanban/questions.go`
- `schema/waiting.schema.json`
- `schema/question.schema.json`
- `schema/issue-keys.schema.json`
- `internal/ledger/ledger.go`

## 既に見ている検査

- `tests/andon-waiting.sh`（バイナリの入口・人が読む行・何も書かないことを見る）
- `tests/andon-issue.sh`（`ask --new` の出力・`who: person`・打ち直し・`--now`・`--later`・`--until` と断る打ち方）
- `tests/andon-waiting.sh` の置いてあるものの段（印 later を Chief of Staff が書いた行と担当が書き直した行）
- `go test ./internal/waiting/`（マージの依頼・判断・手作業の数え方と枠。置いておく印と呼ぶ日は `later_test.go`）と `go test ./internal/ledger/`

## 落とし穴

- 判断の行に出るのは灯りの種類（`business`・`users`・`promise`・`irreversible`・`line`）の問いだけ。`scope` の問いは `ask --new` が断る。
- `ask --new --later` を打てるのは Chief of Staff の端末だけ。道は `HC_TERMINAL_ID=tc` で打つ。`--now` はだれが打っても通る。
- オーナー待ちの数では起動を止めない。`slots.free` はオーナー待ちがいくつあっても減らない。`slots.waiting_full` のキーはもう無い。
- `andon waiting list` の枠の「オーナー待ち」（`slots.waiting`）は、`doing` か `review` の担当の課題のうちオーナー待ちの行を持つものの数で、オーナーの課題は数えない。判断の行が2つあっても 0 のことがある。
- 置いておけるのは、オーナーの課題か、状態が `todo` か `queued` の課題だけ。マージの依頼と、`doing`・`review` の担当の課題の行は、印があっても呼ぶ。
- 印 `later` は、その行の時点の Chief of Staff の端末が最後に書いたときだけ信じる。道は `HC_TERMINAL_ID=tc` で書き、担当は `tw` で書く。
- トリアージと振り返りの apply が足すオーナーの課題は、この道を通らない。`triage` の `tr-later` と `retro` の `rt-later` が見る。
