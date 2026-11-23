# dispatcher の通知

dispatcher は kanban と端末を見回り、報告・問い・gate・マージなどを通知にして Chief of Staff へ届ける。届けるのは Chief of Staff のフック（drain）で、届けた通知は `~/.local/state/andon/dispatcher/<slug>.delivered.jsonl` に、記録だけと決めた通知は `<slug>.recorded.jsonl` に1行ずつ残る。

## 細かい機能

- `dn-live` dispatcher が当てた andon（`/opt/andon/bin/andon dispatcher`）で、当てたあとに起動して live。見る道: 検証の担当だけ。
- `dn-delivered` 当てたあとに Chief of Staff へ通知を届けた記録がある。見る道: 検証の担当だけ。
- `dn-idle` 当てたあとに dispatcher が枠の空きを見て、控え（`<slug>.json` の `idle`）に結果を残した。2回続けて枠が空いていれば、トリアージと振り返りのルーチンを撃ち、通知 `idle` を届ける。見る道: 検証の担当だけ。
- `dn-corrections` 当てたあとに片付いた課題の手直しの記録（`library/corrections.json`）を、当てた dispatcher が書いた。見る道: 検証の担当だけ。
- `dn-turn` 届いた通知を Chief of Staff が `<news>` として読み、オーナーへ返すか ack した。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。

## 辿り着き方（利用者から）

- 担当が `andon issue report`・`ask`・`ready` を打つと、次の見回りで通知になる。
- Chief of Staff の turn の終わり（Stop）とオーナーの送信（UserPromptSubmit）で、待ちの通知が届く。
- 担当の課題が片付く（`andon issue finish`・Chief of Staff の `kanban done`）と、次の見回りで dispatcher がその課題の手直しの記録を1回だけ書く。
- 枠が空いたまま `idle_minutes`（既定 15 分）を2回またぐと、dispatcher がルーチンを撃ち、通知 `idle` を積む。誰も打たない。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff へ届けた通知が1件以上ある。無ければ `dn-live` だけを確かめて終了コード 3（`dn-delivered` は未確認）。記録だけの通知は届けた証拠にしない。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh dispatcher-news` を打つ。`hc get dispatcher --json` と控えの置き場を読むだけ。
- **常駐を読む。** `dn-live` の行に dispatcher の `state`・`argv`・`created_ms` が出る。`ok   dn-live` が見えれば動いた。
- **届けたものを読む。** `dn-delivered` の行に、当てたあとに届けた通知（課題と種類）と記録だけの通知が出る。`ok   dn-delivered` が見えれば動いた。
- **枠の空きを見たかを読む。** `dn-idle` の行に、控えの `idle`（見た時刻・枠が空いていたか・起動できる課題の数・知らせた id）と、当てたあとに届けた通知 `idle` の `detail` が出る。`ok   dn-idle` が見えれば、当てたあとに枠の空きを見た。撃ったルーチンは `kanban routine list` の `last` でも読める。控えが無ければ `note dn-idle` が出て、dn-idle は未確認になる（終了コードは変えない）。
- **手直しの記録を読む。** `dn-corrections` の行に、当てたあとに書いた `corrections.json` ごとの課題・時刻・種類ごとの数・`human_commits` の数が出る。`ok   dn-corrections` が見えれば、当てた dispatcher が手直しの記録を書いた。当てたあとに片付いた課題が無ければ `note dn-corrections` が出て、dn-corrections は未確認になる（終了コードは変えない）。拾う規則（merge commit を外すなど）が効いたかは、記録の `human_commits` と PR の commit を見比べて読む。
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
- `acked` が false の行は、Chief of Staff がオーナーへ何かを言ったことを示し、届かなかったことではない。
- 控えの置き場は `XDG_STATE_HOME` があればその下。`drive.sh` は呼んだ端末の環境で読む。
- dispatcher は起動して最初の回では枠を数えて控えるだけで、撃たない。ルーチンと通知 `idle` を見るには、当てたあと `idle_minutes` の2倍ほど待ってから打ち直す。枠が埋まっていれば撃たないので、通知 `idle` が無いことは壊れではない。
- 手直しの記録は、当てたあとに片付いた課題にだけ書かれる。当ててすぐ検証すると、片付いた課題がまだ無く dn-corrections は未確認になる。当てる前の見回りで書いた記録（時刻が切り替わった時刻より前）は、前のパッケージのコードが書いたので証拠にしない。
- 通知 `idle` は課題を持たない。`dn-delivered` の行では課題が `-` になる。
- 次の commit が当たると dispatcher は起動し直し、枠の空きの見回りは最初の回からやり直しになる。当てる間が `idle_minutes` の2倍より短い間は、ルーチンを撃つところは見られない。dn-idle の撃つところは未確認と書く。
