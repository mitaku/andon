# dispatcher の通知

dispatcher は kanban と端末を見回り、報告・問い・gate・マージなどを通知にして Chief of Staff へ届ける。届けるのは Chief of Staff のフック（drain）で、届けた通知は `~/.local/state/andon/dispatcher/<slug>.delivered.jsonl` に、記録だけと決めた通知は `<slug>.recorded.jsonl` に1行ずつ残る。

## 細かい機能

- `dn-live` dispatcher が当てた andon（`/opt/andon/bin/andon dispatcher`）で、当てたあとに起動して live。見る道: 検証の担当だけ。
- `dn-delivered` 当てたあとに Chief of Staff へ通知を届けた記録がある。見る道: 検証の担当だけ。
- `dn-turn` 届いた通知を Chief of Staff が `<news>` として読み、人へ返すか ack した。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。

## 辿り着き方（利用者から）

- 担当が `andon issue report`・`ask`・`ready` を打つと、次の見回りで通知になる。
- Chief of Staff の turn の終わり（Stop）と人の送信（UserPromptSubmit）で、待ちの通知が届く。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff へ届けた通知が1件以上ある。無ければ `dn-live` だけを確かめて終了コード 3（`dn-delivered` は未確認）。記録だけの通知は届けた証拠にしない。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh dispatcher-news` を打つ。`hc get dispatcher --json` と控えの置き場を読むだけ。
- **常駐を読む。** `dn-live` の行に dispatcher の `state`・`argv`・`created_ms` が出る。`ok   dn-live` が見えれば動いた。
- **届けたものを読む。** `dn-delivered` の行に、当てたあとに届けた通知（課題と種類）と記録だけの通知が出る。`ok   dn-delivered` が見えれば動いた。
- **読んだ turn を見る（`dn-turn`）。** `hc read <Chief of Staff の address>` で、`dn-delivered` に出た通知の時刻のあとに `<news>` を読んだ turn があるかを見る。Chief of Staff の address は kanban の `config.yaml` の `chief_of_staff`。

## 変えたら動かすパス

- `internal/dispatcher/`
- `schema/notification.schema.json`
- `process/states/`
- `package/hooks/`

## 既に見ている検査

- `tests/andon-dispatcher.sh`（偽の端末と kanban で、見回り・通知・届け・ack を見る）
- `tests/andon-dispatcher-signal.sh`
- `go test ./internal/dispatcher/`

## 落とし穴

- dispatcher に `hc send` や `andon dispatcher drain` を手で打たない。待ちの通知を本物の Chief of Staff から抜き取ってしまう。
- `acked` が false の行は、Chief of Staff が人へ何かを言ったことを示し、届かなかったことではない。
- 控えの置き場は `XDG_STATE_HOME` があればその下。`drive.sh` は呼んだ端末の環境で読む。
