# ダイジェスト

ダイジェストの担当が、期間の記録を集め（`andon digest collect`）、要旨とテーマの文を照らして1枚の HTML とメッセージに組み立て（`andon digest build`）、files/ と送り先へ出す（`andon digest send`）道。送り先はオーナーが `project.json` の `digest` に書いたチャンネルだけで、無ければ files/ だけに出す。

## 細かい機能

- `dg-collect` 期間の記録が `library/digest/input.json` に集まり、期間に片付いた課題が入る。`andon digest collect --help` は `--project` の行を持ち、どこで打つかを言わない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/collect.go`、`internal/digest/decided.go`、`internal/digest/digest.go`、`internal/redact/redact.go`、`internal/ledger/cron.go`、`internal/ledger/config.go`、`internal/measure/measure.go`、`internal/waiting/waiting.go`、`schema/digest-input.schema.json`
- `dg-build-refuse` 入力に無い課題を指す文は理由1行で断り、何も書かない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/build.go`
- `dg-build` 照らした文から `digest.json`・図を持つ `digest.html`・`message.txt` が書かれる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/build.go`、`internal/digest/render.go`、`internal/digest/digest.html.tmpl`、`schema/digest.schema.json`
- `dg-later` 冒頭の「オーナーが呼ばれている」の数と琥珀は呼んでいるオーナー待ちだけで、節 2 は「呼んでいるもの」と「置いてあるもの」に分かれ、置いてある行に置いた理由が出る。`message.txt` は呼んでいるものと置いてあるものの 2 つの数を書く。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/build.go`、`internal/digest/render.go`、`internal/digest/digest.html.tmpl`、`internal/digest/collect.go`、`internal/digest/digest.go`、`internal/waiting/waiting.go`
- `dg-send` 送り先の無い `project.json` で、ローカルの tray に届かなければ、`digest.html` を files/ にそのまま出し、置いたパスを返し、`sent.json` の送り先は `path` を持つ files だけ。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/send.go`、`internal/publish/publish.go`、`internal/tray/tray.go`、`internal/tray/html.go`、`schema/digest-sent.schema.json`
- `dg-tray` 切り離した置き場で当てたパッケージのローカルの tray（`andon tray serve`）を起動し、送りを入れて打ち直すと、files/ に倒れた files の行がローカルの tray の URL に置き直る。`andon publish` は tray の URL を返して files/ に写さず、`andon tray status` が上がった 2 枚を数える。パッケージに tray が無ければ note で未確認。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/send.go`、`internal/publish/publish.go`、`internal/tray/tray.go`、`internal/tray/html.go`
- `dg-check-refuse` 送り先が無ければ、`andon digest check` は何も送らずに断る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/digest/send.go`
- `dg-slack` 送り先の Slack のチャンネルに、`digest.html` と `message.txt` の文が届く。見る道: オーナーが見る。
  - 見るファイル: `internal/digest/slack.go`

## 辿り着き方（利用者から）

- ルーチンが足したダイジェストの課題を担当が受け持ち、`andon digest collect <id>`・`andon digest build <id> -`・`andon digest send <id>` を順に打つ。
- オーナーが送り先を書いたあと、`andon digest check` で送り先へ確かめのファイルを送る。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh digest <検証する commit>` を打つ。切り離した kanban に片付いた課題とダイジェストの課題を作り、送り先を書かないセルの repo（一時の git の repo）で、担当として5つの細かい機能を順に動かす。Slack へは何も送らない。
- **集める。** `dg-collect` の行に `input.json` の `since`・`until`・`issues`・`skipped` が出る。続く `dg-collect` の行に `andon digest collect --help` が出る。`ok   dg-collect` が2行見えれば動いた。
- **断られる。** `dg-build-refuse` の行に `andon digest build` の終了コード 1 と、入力に無い課題を指したという理由1行が出る。`ok   dg-build-refuse` が見えれば動いた。
- **組み立てる。** `dg-build` の行に3つのファイルのパスと `ls -l` が出る。`ok   dg-build` が見えれば動いた。
- **分けて描く。** 集める前に、Chief of Staff の端末が呼ぶ判断（`andon issue ask --new --now`）と置いておく判断（`--later`）を 1 件ずつ足してある。`dg-later` の行に節 2 の 2 つの見出しと置いた理由、`message.txt` の 2 つの数の行が出る。`ok   dg-later` が見えれば動いた。
- **送る。** `dg-send` の行に files/ に置いたパスと `sent.json` が出る。`sent.json` の `destinations` が `path` を持つ `files` の1件だけで、`ok   dg-send` が見えれば動いた。
- **tray へ送る。** `dg-tray` の行に切り離した tray の控え、打ち直した send の URL と `sent.json`、`andon publish` の URL、`andon tray status` の出力が出る。`ok   dg-tray` が見えれば、当てたパッケージで tray へ送る道が動いた。`note dg-tray` なら、パッケージに tray が無い。
- **確かめを断る。** `dg-check-refuse` の行に「送り先の設定が無いので何も送らない」が出る。`ok   dg-check-refuse` が見えれば動いた。
- **Slack を見る（`dg-slack`）。** オーナーが送り先を書いたプロジェクトで、次のダイジェストのあとにチャンネルを見る。報告の `acceptance[]` に `manual` と書き、見る場所（チャンネルとダイジェストの課題）を `evidence` に書く。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/digest/`
- `internal/ledger/`
- `internal/publish/`
- `internal/tray/tray.go`
- `internal/tray/html.go`
- `internal/redact/`
- `internal/measure/`
- `internal/waiting/`
- `schema/digest-input.schema.json`
- `schema/digest.schema.json`
- `schema/digest-sent.schema.json`

## 既に見ている検査

- `tests/andon-digest.sh`（collect・build・send で files/ に HTML ができること、送り先の設定が無ければ送らないこと）
- `go test ./internal/digest/`（送る中身と順は `slack_test.go` が偽のサーバで見る）

## 落とし穴

- 本物の andon は Slack の API の根を差し替えられないので、Slack へ送る道は切り離して動かせない。切り離した HOME にはトークン（`~/.config/andon/digest/slack.token`）が無く、送り先を書いても送りそこねるだけで、`dg-slack` の証拠にならない。`dg-slack` はオーナーが見る。
- `andon digest check` は、送り先があれば本物のチャンネルへ確かめのファイルを送る。検証の担当は、送り先のあるプロジェクトで打たない。
- 切り離した道は、検査と同じく報告をローカルの tray へ送らない（`tests/lib/isolate.sh` が送りを切る）。`dg-send` が見るのは、tray に届かないときに `files/` へ倒れる道。tray へ送る道は `dg-tray` が、切り離した置き場に別のローカルの tray を起動して見る。セルの本物の tray（オーナーの一覧）には送らない。
- `dg-tray` の一覧の面は空いている番号で起動する（切り離したセルの repo の `.env` に `TRAY_PORT=0`）。セルの本物の tray が既定の番号を持っているため。
- `dg-tray` の URL のホスト名とポートは、セルの名前と `.env` の `PORTLESS_PORT` から決まる。URL がオーナーの手元で開けるかは見ない。
- ダイジェストの期間は前に送ったダイジェストの `sent.json` から決まる。切り離した kanban には前のダイジェストが無いので、期間は 24 時間前から今まで。
