# dispatcher の通知

dispatcher は kanban と端末を見回り、報告・問い・gate・マージなどを通知にして Chief of Staff へ届ける。届けるのは Chief of Staff のフック（drain）で、届けた通知は `~/.local/state/andon/dispatcher/<slug>.delivered.jsonl` に、記録だけと決めた通知は `<slug>.recorded.jsonl` に1行ずつ残る。

## 細かい機能

- `dn-live` dispatcher が当てた andon（`/opt/andon/bin/andon dispatcher run`）で、当てたあとに起動して live。見る道: 検証の担当だけ。
  - 見るファイル: `internal/dispatcher/dispatcher.go`、`internal/dispatcher/state.go`、`internal/dispatcher/terminal.go`、`internal/flock/flock.go`、`internal/dispatcher/otel.go`、`internal/otel/prune.go`、`internal/otel/otel.go`
- `dn-delivered` 当てたあとに Chief of Staff へ通知を届けた記録がある。見る道: 検証の担当だけ。
  - 見るファイル: `internal/dispatcher/pickup.go`、`internal/dispatcher/deliver.go`、`internal/dispatcher/delivered.go`、`internal/dispatcher/route.go`、`internal/dispatcher/abandoned.go`、`internal/dispatcher/estimate.go`、`internal/dispatcher/impact.go`、`internal/dispatcher/memory.go`、`internal/issue/impact.go`、`internal/contract/impact.go`、`schema/notification.schema.json`、`schema/delivered.schema.json`、`process/states/delivery.yaml`、`process/states/estimate.yaml`、`process/states/memory.yaml`
- `dn-idle` 当てたあとに dispatcher が枠の空きを見て、控え（`<slug>.json` の `idle`）に結果を残した。2回続けて枠が空いていれば、トリアージと振り返りのルーチンを線の上で、`idle_hours` を持つルーチンを時間の上で撃ち、通知 `idle` を届ける。見る道: 検証の担当だけ。
  - 見るファイル: `internal/dispatcher/idle.go`、`internal/dispatcher/retro.go`、`cmd/kanban/routines.go`、`process/states/idle.yaml`
- `dn-impact-stacked` 当てたあとに課題がマージされたとき、`andon issue impact` の出力で積んだ課題の行に `stacked_on` が出て、通知 `impact` は積んだ課題をぶつかり・重なり・検査の置き場の動きでは載せず、積んだ相手でない課題のマージが依頼書の指すファイルを変えたときだけ載せる（ADR-0067 決めたこと 11）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/impact.go`、`internal/contract/impact.go`、`internal/dispatcher/impact.go`、`package/skills/chief-of-staff/playbooks/finish.md`
- `dn-corrections` 当てたあとに片付いた課題の手直しの記録（`library/corrections.json`）を、当てた dispatcher が書いた。見る道: 検証の担当だけ。
  - 見るファイル: `internal/dispatcher/corrections.go`、`internal/dispatcher/outcome.go`、`process/states/aftermath.yaml`
- `dn-signal` 当てたあとの通知 signal に、記録にあるときだけ失敗が始まった commit（`release`、40 桁）が載った。見る道: 検証の担当だけ。
  - 見るファイル: `internal/dispatcher/signals.go`、`internal/signal/signal.go`、`internal/signal/cmd.go`、`internal/signal/echo_linux.go`、`process/states/signal.yaml`
- `dn-turn` 届いた通知を Chief of Staff が `<news>` として読み、オーナーへ返すか ack した。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/dispatcher/news.go`、`internal/dispatcher/ack.go`、`package/hooks/stop`、`package/skills/chief-of-staff/playbooks/watch.md`、`package/skills/chief-of-staff/playbooks/finish.md`
- `dn-ready-report` Chief of Staff が `kanban set <id> deliverable=report` を書いて起動した課題（当てたあとの検証・点検の課題）の依頼書の `## Settings` の写しが `Deliverable: report` のままで、その課題を待つ課題が引き取れるようになったとき、Chief of Staff に届く `<id> ready` の行に「待つ相手 … がやめになった」が付かない（ADR-0066 受け入れ条件 10）。見る道: オーナーが見る。
  - 見るファイル: `internal/dispatcher/abandoned.go`、`internal/dispatcher/dispatcher.go`、`internal/ledger/ledger.go`、`schema/issue-keys.schema.json`

## 辿り着き方（利用者から）

- 担当が `andon issue report`・`ask`・`ready` を打つと、次の見回りで通知になる。
- Chief of Staff の turn の終わり（Stop）とオーナーの送信（UserPromptSubmit）で、待ちの通知が届く。
- 担当の課題が片付く（`andon issue finish`・Chief of Staff の `kanban done`）と、次の見回りで dispatcher がその課題の手直しの記録を1回だけ書く。
- 枠が空いたまま `idle_minutes`（既定 15 分）を2回またぐと、dispatcher がルーチンを撃ち、通知 `idle` を積む。誰も打たない。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff へ届けた通知が1件以上ある。無ければ `dn-live` だけを確かめて終了コード 3（`dn-delivered` は未確認）。記録だけの通知は届けた証拠にしない。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh dispatcher-news <検証する commit>` を打つ。`hc get dispatcher --json` と控えの置き場を読むだけ。
- **常駐を読む。** `dn-live` の行に dispatcher の `state`・`argv`・`created_ms` が出る。`argv` が当てた andon の `dispatcher run` で、`ok   dn-live` が見えれば動いた。`argv` が名前を変える前の `dispatcher` なら、当てる前のバージョンが起動した dispatcher が残っている。
- **届けたものを読む。** `dn-delivered` の行に、当てたあとに届けた通知（課題と種類）と記録だけの通知が出る。`ok   dn-delivered` が見えれば動いた。
- **枠の空きを見たかを読む。** `dn-idle` の行に、控えの `idle`（見た時刻・枠が空いていたか・起動できる課題の数・知らせた id）と、当てたあとに届けた通知 `idle` の `detail` が出る。`ok   dn-idle` が見えれば、当てたあとに枠の空きを見た。撃ったルーチンは `kanban routine list` の `last` でも読める。控えが無ければ `note dn-idle` が出て、dn-idle は未確認になる（終了コードは変えない）。続く行に、空きで撃てるルーチン（トリアージ・振り返りと `idle_hours` を持つもの）ごとに `idle_hours`・`paused`・片付いていない課題・検証する間に足した課題が出る。足した課題のほかは今の状態で、前の commit を検証するときも今を読む。`idle_hours`（1 以上の整数）を持つルーチンが無いか、撃てるルーチンがどれも `paused` か片付いていない課題を持てば、`note dn-idle` が出て、空きで撃つところは未確認になる（終了コードは変えない）。
- **手直しの記録を読む。** `dn-corrections` の行に、当てたあとに書いた `corrections.json` ごとの課題・時刻・種類ごとの数・`human_commits` の数が出る。`ok   dn-corrections` が見えれば、当てた dispatcher が手直しの記録を書いた。当てたあとに片付いた課題が無ければ `note dn-corrections` が出て、dn-corrections は未確認になる（終了コードは変えない）。拾う規則（merge commit を外すなど）が効いたかは、記録の `human_commits` と PR の commit を見比べて読む。
- **通知 signal を読む。** `dn-signal` の行に、当てたあとの通知 signal の数（届けたものと記録だけのもの）と、記録だけの通知 signal の key・URL・`release` が出る。`ok   dn-signal` が見えれば、検証する間の通知 signal の detail に、40 桁の commit の `release` が載った。届けた記録（`delivered.jsonl`）は課題と kind だけで detail を持たないので、届けた通知 signal に `release` が載ったかはこの行では言えない。detail を読める通知に `release` が無いか、通知 signal が無ければ `note dn-signal` が出て、dn-signal は未確認になる（終了コードは変えない）。`release` の commit から PR を引く手（Chief of Staff の手引き）は、この行では見ない。
- **報告の課題を待つ課題の ready を見る（`dn-ready-report`）。** 検証の担当は動かせない。報告の `acceptance[]` に `manual` と書き、オーナーが見る次の証拠を `evidence` に書く: 当てたあとに起動した検証の課題の `issues/<id>/brief.md` の `## Settings` に `- Deliverable: report` があり、`kanban show <id> --json` の `.issue.deliverable` が `report`。その課題を `after` に持つ課題があれば、その課題の `<id> ready` の行に「がやめになった」が無い。待つ課題が無ければ、付かなかったことは見られないので未確認と書く。
- **読んだ turn を見る（`dn-turn`）。** `hc read <Chief of Staff の address>` で、`dn-delivered` に出た通知の時刻のあとに `<news>` を読んだ turn があるかを見る。Chief of Staff の address は kanban の `config.yaml` の `chief_of_staff`。積んだ課題の通知があれば、その turn の扱いも読む。`gate` の `behind`（前の課題の PR がまだ main に入っていない）は、オーナーに何も言わずに枠を数え直したか（`watch.md`）。`behind` の「前の課題 <id> がマージされずに終わった」と、`merged` の `held` が「PR が既定の branch でない base」で始まる通知は、オーナーに1問出したか（`watch.md`・`finish.md`）。`behind` の「main に入ったかを gate が判定できない」は、続く理由を読んで直すかオーナーへ知らせたか（`watch.md`）。積んだ課題を上に持つ課題をやめる go を取った返事に積んだ課題のことがあり、積んだ課題が `doing` なら finish の前に担当へ待てと送り、finish のあとに問いを出したか（`finish.md` のやめる課題）。
- **積んだ課題の影響を読む（`dn-impact-stacked`）。** 当てたあとに、積んだ課題（`stacked_on` のある `doing`・`review` の課題）があるうちに同じ repo の別の課題がマージされたときだけ確かめられる。そのマージした課題に `andon issue impact <id>` を打ち、積んだ課題の行に `stacked_on` が出ることを読む。同じ時刻のあとに届けた通知 `impact` の `detail` に、積んだ課題がぶつかり・重なり・検査の置き場の動きだけでは載っていないことを読む（依頼書の指すファイルを変えたときは載る）。Chief of Staff の画面で、積んだ課題の担当に rebase を頼んでいないことも読む（`finish.md` の影響の表の積んだ課題の 2 行）。積んだ課題が無ければ未確認と書く。
- **積める課題を知らせたかを読む（`dn-idle` の積める課題）。** 控えの `idle` の `ready` に、そのときの `andon waiting list --json` の `stackable` の id が入っていて、2回続けて起動されずに残った積める課題が通知 `idle` の `unstarted` に出ることを読む。当てたあとに積める課題が無ければ、この段は未確認と書く。

## 変えたら動かすパス

- `internal/dispatcher/`
- `schema/notification.schema.json`
- `schema/delivered.schema.json`
- `process/states/`
- `package/hooks/`
- `internal/flock/`
- `internal/signal/`
- `internal/otel/otel.go`
- `internal/otel/prune.go`
- `internal/contract/impact.go`
- `internal/issue/impact.go`
- `cmd/kanban/routines.go`
- `package/skills/chief-of-staff/playbooks/watch.md`
- `package/skills/chief-of-staff/playbooks/finish.md`
- `internal/ledger/ledger.go`
- `schema/issue-keys.schema.json`

## 既に見ている検査

- `tests/andon-dispatcher.sh`（偽の端末と kanban で、見回り・通知・届け・ack を見る）
- `tests/andon-dispatcher-signal.sh`
- `go test ./internal/dispatcher/`
- `go test ./internal/otel/`（ハーネスの生の記録の片付け）

## 落とし穴

- dispatcher に `hc send` や `andon dispatcher drain` を手で打たない。待ちの通知を本物の Chief of Staff から抜き取ってしまう。
- `acked` が false の行は、Chief of Staff がオーナーへ何かを言ったことを示し、届かなかったことではない。
- 控えの置き場は `XDG_STATE_HOME` があればその下。`drive.sh` は呼んだ端末の環境で読む。
- dispatcher は起動して最初の回では枠を数えて控えるだけで、撃たない。ルーチンと通知 `idle` を見るには、当てたあと `idle_minutes` の2倍ほど待ってから打ち直す。枠が埋まっていれば撃たないので、通知 `idle` が無いことは壊れではない。
- 手直しの記録は、当てたあとに片付いた課題にだけ書かれる。当ててすぐ検証すると、片付いた課題がまだ無く dn-corrections は未確認になる。当てる前の見回りで書いた記録（時刻が切り替わった時刻より前）は、前のパッケージのコードが書いたので証拠にしない。
- 通知 signal は、信号の出所（`andon signal auth` で入れる）が無いセルでは出ない。出ないことは壊れではない。`release` は出所が github-ci で、記録が失敗の始まった run の commit を持つときだけ載る。
- 通知 signal の `title` は外の人の文。`dn-signal` の行には出さない。
- `dn-signal` に偽の信号の出所を置く道は持たない。偽の出所で dispatcher を切り離して回しても、セルの本物の dispatcher が当てたあとに通知を作った証拠にならない。届けた記録は detail を持たないので、`release` が載ったかは、信号の出所（`project.json` の `signals`）を持つプロジェクトのあるセルで、記録だけの通知の detail を読んで見る。
- 信号の記録（`<slug>.signals.json`）の `release` を、通知に載った証拠にしない。信号の記録は今の状態で、通知の中身でも、検証する commit が当たっていた間の状態でもない。届けた通知 signal に `release` が載ることは `tests/andon-dispatcher-signal.sh` と `go test ./internal/dispatcher/` が見ている。
- 空きで撃つのは、`paused` でなく、片付いていない課題を持たないルーチンだけ。dispatcher は `paused` が無いか真偽値の false のときだけ撃ち、`true`・引用符の文字列・空の値では撃たない。道も YAML と同じく注釈（前に空白のある `#` から後）を外して読む。`idle_hours` はルーチンのファイルに Andon が読む欄で、1 以上の整数でない値は欄が無いものとして扱う。書いたルーチンが kanban に無ければ、`idle_hours` で撃つところは見られない。履歴の `fired` の行は、空きで撃ったか `schedule` で撃ったかを区別しない。どちらで撃ったかは dispatcher の端末のログ（`hc read dispatcher`）の行で読む。
- dispatcher は巡ごとにハーネスの生の記録の片付け（`internal/dispatcher/otel.go`・`internal/otel/prune.go`）を呼ぶ。片付けは置き場（`~/.local/state/andon/otel/<slug>/`）が無ければ何もせず、失敗してもログに1行書くだけで巡を止めない。`dn-live` が見るのは、当てた dispatcher が片付けを含む巡を回し続けていることだけで、記録を消したことは見ない。消したかは dispatcher の端末のログ（`hc read dispatcher`）の「ハーネスの生の記録を N 本消した」の行と、置き場の `.pruned` の時刻で読む。受け口（`andon otel serve`。boot と役の起動が起動する。機能の地図の `otel`）がまだ何も書いていないセルでは置き場ができず、その行も `.pruned` も現れない。そのときは片付けを未確認と書く。寿命と上限は `project.json` の `otel` から読むので、消えた本数は既定の 30 日と 10 GB と違うことがある。`internal/otel/otel.go` の受け口の本体（属性を落とす・秘密を伏せる・書く）も `dn-live` では動かないので、その部分を変えた差分で `dn-live` が ok でも、受け口を確かめたとは書かない。消し方と受け口は `go test ./internal/otel/` が見ている。
- 通知 `idle` は課題を持たない。`dn-delivered` の行では課題が `-` になる。
- 通知 `answered`・`hold` を渡したターンも届けた記録に1行残り、`dn-delivered` の行に `<課題> answered`・`<課題> hold` と出る。記録が持てる種類は `schema/delivered.schema.json` の一覧で、通知の一覧とのずれは `go test ./internal/contract/` が見ている。当てたあとにこの2つを渡したターンが無ければ、行に出ないことは壊れではない。
- 次の commit が当たると dispatcher は起動し直し、枠の空きの見回りは最初の回からやり直しになる。当てる間が `idle_minutes` の2倍より短い間は、ルーチンを撃つところは見られない。dn-idle の撃つところは未確認と書く。
