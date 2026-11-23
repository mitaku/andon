# ダイジェスト

ダイジェストの担当が、期間の記録を集め（`andon digest collect`）、要旨とテーマの文を照らして1枚の HTML とメッセージに組み立て（`andon digest build`）、files/ と送り先へ出す（`andon digest send`）道。送り先はオーナーが `project.json` の `digest` に書いたチャンネルだけで、無ければ files/ だけに出す。

## 細かい機能

- `dg-collect` 期間の記録が `library/digest/input.json` に集まり、期間に片付いた課題が入る。見る道: 検証の担当だけ。
- `dg-build-refuse` 入力に無い課題を指す文は理由1行で断り、何も書かない。見る道: 検証の担当だけ。
- `dg-build` 照らした文から `digest.json`・図を持つ `digest.html`・`message.txt` が書かれる。見る道: 検証の担当だけ。
- `dg-send` 送り先の無い `project.json` では `digest.html` を files/ にそのまま出し、URL を返し、`sent.json` の送り先は files だけ。見る道: 検証の担当だけ。
- `dg-check-refuse` 送り先が無ければ、`andon digest check` は何も送らずに断る。見る道: 検証の担当だけ。
- `dg-slack` 送り先の Slack のチャンネルに、`digest.html` と `message.txt` の文が届く。見る道: オーナーが見る。

## 辿り着き方（利用者から）

- ルーチンが足したダイジェストの課題を担当が受け持ち、`andon digest collect <id>`・`andon digest build <id> -`・`andon digest send <id>` を順に打つ。
- オーナーが送り先を書いたあと、`andon digest check` で送り先へ確かめのファイルを送る。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh digest` を打つ。切り離した kanban に片付いた課題とダイジェストの課題を作り、送り先を書かないセルの repo（一時の git の repo）で、担当として5つの細かい機能を順に動かす。Slack へは何も送らない。
- **集める。** `dg-collect` の行に `input.json` の `since`・`until`・`issues`・`skipped` が出る。`ok   dg-collect` が見えれば動いた。
- **断られる。** `dg-build-refuse` の行に `andon digest build` の終了コード 1 と、入力に無い課題を指したという理由1行が出る。`ok   dg-build-refuse` が見えれば動いた。
- **組み立てる。** `dg-build` の行に3つのファイルのパスと `ls -l` が出る。`ok   dg-build` が見えれば動いた。
- **送る。** `dg-send` の行に files/ の URL と `sent.json` が出る。`sent.json` の `destinations` が `files` の1件だけで、`ok   dg-send` が見えれば動いた。
- **確かめを断る。** `dg-check-refuse` の行に「送り先の設定が無いので何も送らない」が出る。`ok   dg-check-refuse` が見えれば動いた。
- **Slack を見る（`dg-slack`）。** オーナーが送り先を書いたプロジェクトで、次のダイジェストのあとにチャンネルを見る。報告の `acceptance[]` に `manual` と書き、見る場所（チャンネルとダイジェストの課題）を `evidence` に書く。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/digest/`
- `internal/publish/`
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
- `dg-send` の URL のホスト名とポートは、セルの名前と `.env` の値から決まる。URL がオーナーの手元で開けるかは見ない。
- ダイジェストの期間は前に送ったダイジェストの `sent.json` から決まる。切り離した kanban には前のダイジェストが無いので、期間は 24 時間前から今まで。
