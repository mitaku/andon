# プロジェクトを立てる

オーナーがセルの repo で `andon create <slug>` を打つと、`projects/<slug>/` に kanban・`project.json` の下書き（委譲は無し）・`AGENTS.md` の雛形を作り、repo の clone が無ければ `ghq get` で足す。作ったあと、dispatcher の置き場が kanban の数に合わなければ起動し直す。役の端末では打てず、在る slug には何も変えずに断る。

## 細かい機能

- `cr-role-refuse` 役の端末（`hc` のラベル `role=` か `issue=`）では断り、`projects/<slug>` を作らない。見る道: 検証の担当だけ。
- `cr-create` オーナーの端末で、kanban・`project.json` の下書き（接頭字・repos・main の CI の信号の行。委譲は無し）・`AGENTS.md` ができ、作った kanban に接頭字の番号の課題を足せる。dispatcher が無ければ `hc run` で起動しようとする。見る道: 検証の担当だけ。
- `cr-exists-refuse` 在る slug は断り、`projects/<slug>` の中身を変えない。見る道: 検証の担当だけ。
- `cr-ask` 端末で引数を省くと、slug・repos・接頭字・題を既定値つきで1つずつ聞く。見る道: オーナーが見る。

## 辿り着き方（利用者から）

- オーナーがセルの repo で `andon create <slug> --repo <host>/<owner>/<repo> [--prefix X] [--title <題>]` を打つ。引数を省けば端末で聞かれる。
- 作ったあと、オーナーが `project.json` と `AGENTS.md` を見て commit し、`andon open <slug>` で Chief of Staff を開く。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh create` を打つ。一時のセルの repo（`ANDON_WORKSPACE`）と一時の clone の根（`GHQ_ROOT`。repo の clone を先に置く）で、端末でなく（標準入力を `/dev/null` にして）打つ。`hc` は偽物で、偽の `hc get dispatcher` は終わった端末を返す。
- **役の端末で断られる。** `cr-role-refuse` の行に `andon create` の終了コード 1 と「この端末（role=chief-of-staff）では打てない」が出る。`ok   cr-role-refuse` が見えれば動いた。
- **立てる。** `cr-create` の行に作ったファイルの一覧と `project.json` が出る。そのあとに `kanban add` が返した `VC-0001` と、偽の `hc` に届いた `hc run --bg … --name dispatcher … dispatcher` が出る。`ok   cr-create` が3行見えれば動いた。
- **在る slug で断られる。** `cr-exists-refuse` の行に「projects/<slug> が既にある」が出る。`ok   cr-exists-refuse` が見えれば動いた。
- **聞かれる（`cr-ask`）。** オーナーが次にプロジェクトを立てるとき、端末で引数を省いて打ち、聞かれる順と既定値を見る。報告の `acceptance[]` に `manual` と書き、見る場所（オーナーの端末）を `evidence` に書く。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/create/`
- `internal/dispatcher/terminal.go`
- `schema/project.schema.json`

## 既に見ている検査

- `tests/andon-create.sh`（偽の ghq と hc で、作る中身・commit しないこと・在る slug と役の端末で断ること・端末でなければ聞かないことを見る）
- `go test ./internal/create/`（聞く道の中身）

## 落とし穴

- `andon create` をセルの本物の repo で打つと、`projects/<slug>/` を作り、本物の dispatcher を起動し直す。検証の担当は `drive.sh create` の切り離した道だけで打つ。
- repo の clone が一時の `GHQ_ROOT` に無いと `ghq get` がネットワークへ出る。`journeys/create.sh` は clone を先に置く。clone の判定（git の作業ツリーの一番上か）が変わると `cr-create` が `ghq get` で落ちる。そのときは置き方を直す。
- 一時のセルの repo は git の repo でない。`project.json` を commit する手と、`andon delegate show` が HEAD の `project.json` を読む道は見ない。
