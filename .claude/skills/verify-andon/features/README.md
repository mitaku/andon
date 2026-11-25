# Andon の機能の地図

この地図は、Andon の利用者から見た機能を、セルの本物で動かして確かめるための正。動かす前にこの目次を読み、当たる機能のファイルを手順として使う。手順の全体（当てる・診断・動かす・証拠・片付け）は `../SKILL.md`。

## 動かす前の決まり

- 動かすのは、セルに当てた本物（`/usr/local/bin/andon`・`/usr/local/bin/kanban`・`/opt/andon` のパッケージ）。repo の checkout から焼いたものでは動かさない。
- 先に `bash .claude/skills/verify-andon/scripts/doctor.sh` を打ち、終了コード 0 を確かめる。
- 1機能は `bash .claude/skills/verify-andon/scripts/drive.sh <機能> <検証する commit>` の1回の呼び出しで、作る・動かす・片付ける・跡の確かめまで済む。手順を Bash の呼び出しに分けない（呼び出しごとに環境が戻り、後の呼び出しがセルの本物の kanban を見る）。
- 証拠は `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh <機能> <検証する commit>` で撮る。出力がそのまま記録の `runs/<n>.log` になる。

## 動かし方の2つの形

- **切り離して動かす**: `scripts/sandbox.sh` を読む道。一時の HOME と一時の kanban を作り、本物の andon・`kanban`・`/opt/andon` を動かす。`hc` と `gh` は偽物で、本物の端末と GitHub には届かない。kanban を書く機能はこの形にする。
- **本物の記録を読む**: `scripts/cell.sh` を読む道。検証する commit を当てた時刻（`/opt/andon/applied.json`）から次の commit を当てた時刻までに、セルで本当に起きた出来事の記録を読む。検証する commit は `drive.sh` の2つ目の引数で、省けない（省くと使い方の誤りの終了コード 2）。dispatcher が書く記録（gate・通知・枠の空き）は、当てた時刻でなく dispatcher が切り替わった時刻（記録の `switched_at`。無ければ当てた時刻）で区切る。当ててから前の dispatcher が見回りを終えるまで、その記録は前のパッケージのコードが書くため。前に当てた commit と時刻は記録の `previous`（新しい順）にあり、そこに無い commit（前のバージョンの記録）は間が決まらないので終了コード 3（未確認）。何も書かず、何も起動しない。出来事がまだ無ければ終了コード 3（未確認）。今の状態を読む行（dispatcher の端末・boot の生成物）は、今当たっているパッケージを読む。

## 見る道（誰が見られるか）

検証の担当はエージェントを起動しない。本物のエージェントが要る機能は、細かい機能ごとに次の3つへ仕分けて、各ファイルの「## 細かい機能」に書く。

- **検証の担当だけ**: `drive.sh` か読むだけの command で、検証の担当が1人で証拠を取れる。
- **Chief of Staff が試しの課題を起動し、検証の担当が画面を読む**: 検証の担当が報告の `## Next` に「試しの課題を起動する」と書き、Chief of Staff が起動した端末を `hc read <address>` で読む。
- **オーナーが見る**: 画面の操作や、オーナーの離席が要る。報告の `acceptance[]` に `manual` と書き、見る場所を `evidence` に書く。

## 証拠と未確認の書き方

- 操作と結果の両方を残す。`drive.sh` は、打った command・終了コード・出力・結果の状態（kanban の JSON）を出す。
- kanban に書く機能は、書いたあとに別の口（`kanban show`・`log.jsonl`）で読み直したものを証拠にする。
- 機能と細かい機能の名前（`rr-ready` など）を、記録と報告の行に添える。
- 確かめられなかった道は、打った command と欠けた前提を書く。別の道で確かめたものを、その道の証拠にしない。
- 終了コード 3 の機能は「未確認」と書き、動いたとは書かない。
- 細かい機能ごとの結果は、`drive.sh` の最後の `結果` の行を写す。名指した細かい機能（commit の本文の `Verify-features:`）を3つ目の引数で渡すと、終了コードはその分だけで決まる。記録の期待と acceptance の行の書き方は SKILL.md の 3 の表。

## 機能のファイルの形

各ファイルは、H1 のタイトルと、利用者から見た振る舞いの1段落で始める。そのあとの H2 は次の6つで、この順に置く。

1. `細かい機能` は、短い名前・1行の説明・見る道を1行ずつ並べる。
   各行の下に `  - 見るファイル: ` の行を置き、その細かい機能が動かすファイルを `` で囲んで「、」で並べる。見るファイルは同じ機能の「変えたら動かすパス」の下にあるものだけ。Go のファイルは1つずつ書き、`<dir>/` は Go のファイルを持たない置き場（skill・テンプレートなど）だけに使う。
2. `辿り着き方（利用者から）` は、利用者（担当・Chief of Staff・オーナー）がその機能に触る入口を並べる。
3. `動かし方` は `前提:` で始め、操作ごとに command と、見えれば動いた証拠になるものを対にして書く。
4. `変えたら動かすパス` は、差分がこのパスに当たれば検証の課題でこの機能を動かす、という行を1行1つ `` で囲んで書く（`scripts/features.sh` が読む）。書き方は `tests/lib/scope.sh` と同じで、`<dir>/` はその下の全部。
5. `既に見ている検査` は、同じ振る舞いを CI で見ている `tests/*.sh` と Go のテストを並べる。
6. `落とし穴` は、検証を無駄にするか、証拠を無効にする罠を並べる。

`tests/verify-andon.sh` は地図について3つを見る。1つ目は、「変えたら動かすパス」の下のテストでないファイル（`*_test.go`・`testdata/`・親の package の名前に test を付けたテスト用の package〈`internal/ledger/ledgertest/`〉を除く）が、どれかの細かい機能の見るファイルに入ること。2つ目は、`internal/`・`cmd/`・`package/` の下のテストでないファイルが、どれかの機能の「変えたら動かすパス」か、下の「地図が覆わないもの」に入ること。製品の変更がファイルや package を足したら、同じ変更で地図を直す。3つ目は、下の「変更ごとの名指し」。

実装の細部は地図に書かない。書くのは、利用者の道・決まった名前・要る状態・command・見える証拠だけ。

## 変更ごとの名指し

見るファイルに載っているだけでは、変えた振る舞いを通る細かい機能があるとは限らない。そこで、細かい機能の見るファイルを変えた変更は、変えた振る舞いを通る細かい機能の名前を、commit の本文の1行で名指す。

```
Verify-features: wl-resume, wl-record
```

- 変えたテストでないファイルが細かい機能の見るファイルに入るなら、そのファイルを見る細かい機能のうち1つ以上を名指す。見るファイルは、変更の前と後の両方の地図から引く。同じ変更で見るファイルから外したファイルや、消したファイルも名指しが要る。
- Go のファイルでコメントだけを変えたもの（`scripts/gocode.go` が go/scanner で読んだ字句の並びが、前後で同じもの）は、名指しが要らない。`scripts/features.sh` も、そのファイルで機能を出さない。振る舞いを変えるコメント（`//go:` の指令・`//line`・`// +build`・`//export`・`//extern`）は字句に数え、`import "C"` を持つファイルはコメントを全部数える（cgo の前置きがコメントに入るため）。
- 名指す名前は、地図の細かい機能に在るもの。同じ変更で足した細かい機能も、同じ変更で消した細かい機能も名指せる。
- 名指せる細かい機能が、変えた振る舞いを通らないなら、同じ変更で細かい機能を足して（か、既にある細かい機能の手順を直して）名指す。見るファイルに載っているだけの細かい機能を名指さない。名指した細かい機能が、当てたあとの検証の課題が動かす道になる。
- 行は、変更の commit のどれかの本文にあればよい。後から足すなら、行だけの commit を積む（`git commit --allow-empty`）。
- `tests/verify-andon.sh` が、比べる base から HEAD までの差分と commit の本文で確かめる。base は、PR の CI では tests の job が `ANDON_VERIFY_BASE` で渡し、手元では `origin/main` との分かれ目を使う。

## 機能

| 機能 | ファイル | 動かし方の形 | 見る道 |
|---|---|---|---|
| 担当の起動 | [worker-launch.md](./worker-launch.md) | 本物の記録を読む | 記録は検証の担当だけ。最初の turn は Chief of Staff の試しの課題 |
| 報告と ready | [report-ready.md](./report-ready.md) | 切り離して動かす | 検証の担当だけ |
| gate | [gate.md](./gate.md) | 本物の記録を読む | 検証の担当だけ |
| dispatcher の通知 | [dispatcher-news.md](./dispatcher-news.md) | 本物の記録を読む | 記録は検証の担当だけ。届いた turn は Chief of Staff の試しの課題 |
| hook | [hooks.md](./hooks.md) | 切り離して動かす | 振る舞いは検証の担当だけ。待ち役と Esc はオーナーが見る |
| boot | [boot.md](./boot.md) | 本物の記録を読む | 検証の担当だけ |
| ダイジェスト | [digest.md](./digest.md) | 切り離して動かす | 検証の担当だけ。Slack に届いたかはオーナーが見る |
| 流れの数え | [measure.md](./measure.md) | 切り離して動かす | 検証の担当だけ |
| プロジェクトを立てる | [create.md](./create.md) | 切り離して動かす | 検証の担当だけ。端末で聞く道はオーナーが見る |
| オーナー待ち | [waiting.md](./waiting.md) | 切り離して動かす | 検証の担当だけ。Herdr App のパネルはオーナーが見る |
| PR を止める | [hold.md](./hold.md) | 切り離して動かす | 検証の担当だけ |
| トリアージ | [triage.md](./triage.md) | 切り離して動かす | 検証の担当だけ |
| 振り返り | [retro.md](./retro.md) | 切り離して動かす | 検証の担当だけ |
| ハーネスの受け口 | [otel.md](./otel.md) | 本物の記録を読む | 記録は検証の担当だけ。受け口を止めて起動する道は Chief of Staff の試しの課題 |

## 地図が覆わないもの

`internal/`・`cmd/`・`package/` の下のうち、次のものはどの機能の道も通らない。機能の「変えたら動かすパス」の下にあっても、変えたときに検証の課題は切られない（`scripts/features.sh` が機能を返さない）。細かい機能の見るファイルには書かない。機能のパスを丸ごと外す置き場も書かない。1行に置き場と理由を書く。行の先頭に `` で囲んで「・」で並べたパスを `tests/verify-andon.sh` が読む。

- `package/lib/`（statusline と schema の検査器）。利用者が動かす道が無い。statusline は Claude Code が画面に出すだけで、検査器は `tests/schema.sh` と `go test ./internal/contract/` が CI で見ている。セルの本物で動かして増える証拠が無い。
- `package/host/`（ホスト用の雛形、Herdr App の mod、Stream Deck の plugin）。セルの外のオーナーの手元で動き、セルの中の検証の担当は動かせない。画面は、変えた PR でオーナーが見る。
- `package/compose.yaml`。セルを起動し直す口で、動いているセルの中からは打ち直せない。`tests/image.sh` が docker のある CI で見ている。
- `package/bin/`。焼いた andon を置く場所で、repo には空の印しか無い。
- `internal/triage/unsorted.go`（トリアージの溜まった数）。dispatcher がトリアージのルーチンを撃つかを決める数え方で、`triage` の道は通らない。撃つところは `dispatcher-news` の `dn-idle` が見る。`go test ./internal/triage/` が見ている。
- `internal/board/`・`internal/chiefofstaff/`・`internal/chiefengineer/`・`internal/open/`（ボードと、Chief of Staff・Chief Engineer を起動して開く口）。オーナーか Chief of Staff が端末で開く道で、機能のファイルはまだ無い。
- `internal/plantinit/`（`andon init`）。新しいセルの repo を作る口で、動いているセルの検証の道には出てこない。`tests/andon-init.sh` が見ている。
- `internal/schema/`（`andon schema check`）。schema の検査器を包む口で、`tests/schema.sh` が見ている。
- `cmd/kanban/migrate.go`・`cmd/kanban/dispatcherstate.go`（`kanban migrate`）。前の形の kanban を移す口で、どの機能の道も打たない。`go test ./cmd/kanban/` と `tests/kanban.sh` が見ている。
- `internal/delegate/cmd.go`（`andon delegate grant` など）。オーナーが委譲を変える口で、どの機能の道も打たない。gate が委譲を読むところは `gt-route` が見る。`go test ./internal/delegate/` と `tests/andon-gate.sh` が見ている。
- `cmd/kanban/lock_windows.go`・`cmd/kanban/rename_windows.go`・`internal/spawn/tty_other.go`・`internal/signal/echo_other.go`。linux でない OS でだけ build されるファイルで、セルの andon と kanban には入らない。

`package/gh-skills.txt` は覆う。`boot.md` の「変えたら動かすパス」に載せた。
