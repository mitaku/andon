# 振り返り

振り返りの担当は、`andon retro collect` で集めた記録から繰り返し（パターン）を拾い、`andon retro patterns <id> -` で `library/retro.json` に書く。Chief of Staff は `andon retro apply <id>` で、`fix` のパターンを直す課題にする。直す先が Andon で、この kanban の repos に Andon が無ければ、パターンは「Andon へ伝えるか」のオーナーの課題になる。その課題は、決まった理由の印 `later`（「振り返りの直しの提案で、答えを待って止まる仕事が無い」）を持ち、`andon waiting list` で呼ばずに置いてある行になる。オーナーの課題を足す apply は Chief of Staff の端末でだけ通る。前のバージョンの apply が `who: person` だけ書いて落ちた課題は、打ち直しで足し直さずに印と問いを書く。

## 細かい機能

- `rt-patterns` 振り返りの担当の端末が `andon retro patterns` で、集め直した記録と照らしたパターンの一覧を `library/retro.json` に書く。一覧には集め直した期間の Chief of Staff のトークン（`chief_of_staff`）も入る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/patterns.go`、`internal/retro/apply.go`、`internal/retro/retro.go`、`internal/retro/typed.go`、`internal/retro/questions.go`、`internal/retro/github.go`
- `rt-collect` 振り返りの担当の端末が `andon retro collect` で集めた入力の全文の束に、結果の記録（`library/outcome.json`）の内訳 `breakdown` が wiki の読み（`knowledge`）ごと写り、期間の Chief of Staff のトークンが、ハーネスの役ごとの生の記録（`~/.local/state/andon/otel/<slug>/roles/chief-of-staff/<日付>.traces.jsonl`）から数えた日数と一緒に出る。形の違う内訳と読めない記録の行は飛ばした記録に出る。振り返りの課題が切った直しに、マージの前と後に片付いた担当の課題の数字（`fixes[].before_after`）が、どちらも 10 件あれば出て、足りなければ理由だけが出る。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/numbers.go`、`internal/retro/retro.go`、`internal/trace/check.go`、`internal/trace/roletokens.go`、`schema/retro.schema.json`
- `rt-later` Chief of Staff の端末の apply は、Andon の直しを「Andon へ伝えるか」のオーナーの課題にし、印 `later` と問いを書く。`andon waiting list --json` でその行が `call: false` になる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/apply.go`
- `rt-not-cos` オーナーの課題を足す apply を Chief of Staff でない端末で打つと、Chief of Staff の端末だけと言って断り、課題も当てた記録も増やさない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/apply.go`
- `rt-old-owner` 前のバージョンの apply が `who: person` だけ書いて落ちた課題（呼ぶ行）を、打ち直しで足し直さずに使い、印 `later` と問いを書いて置いてある行にする。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/apply.go`
- `rt-report` PR の無い振り返りの課題が `review` に入ると、dispatcher は Chief of Staff へ通知 `<id> report` を届け（`review` ではない）、Chief of Staff は手引きの 3 のとおり課題が `review` なのを読んで `andon retro apply <id> --dry-run` を打つ。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `package/skills/chief-of-staff/playbooks/retro.md`、`process/workstreams/retrospective.yaml`

## 辿り着き方（利用者から）

- dispatcher がルーチン retro を撃ち、振り返りの担当が `andon retro collect` で集め、`andon retro patterns <id> -` で一覧を書く（skill `retro`）。
- Chief of Staff が `andon retro apply <id> --dry-run` で読んでから、`andon retro apply <id>` で当てる（手引き `package/skills/chief-of-staff/playbooks/retro.md`）。
- オーナーと役が `andon waiting list` で、「Andon へ伝えるか」の課題を置いてある行として読む。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh retro <検証する commit>` を打つ。切り離した kanban（repos が無い）に、片付いた担当の課題を 2 つと、担当の端末 `tw` が動かす振り返りの課題を作る。Chief of Staff の端末は `tc`。
- **一覧を書く。** `rt-patterns` の行に、Andon の直しのパターン 2 つ（`fix`）と捨てるパターン 1 つ（`drop`）を書いた出力が出る。`ok   rt-patterns` が見えれば動いた。
- **Chief of Staff でない端末を断る。** `rt-not-cos` の行に、`tw` の apply の断り（「置いておく印を書くのは Chief of Staff の端末だけ」）が出る。`ok   rt-not-cos` が見えれば動いた。
- **前のバージョンの形を作る。** `tc` が、2つ目のパターンの「Andon へ伝えるか」の課題を振り返りの課題の下に足し、`who: person` だけを書く。`rt-old-owner` の行に、打ち直しの前の `andon waiting list --json` の行（`call: true`）が出る。
- **置いておく。** `rt-later`・`rt-old-owner` の行に、`tc` の apply の出力と、2つのオーナーの課題の `who`・`parent`・`later` と問い、`andon waiting list --json` の行（`call`・`later`）が出る。`ok   rt-later` が1行と `ok   rt-old-owner` が2行見えれば動いた。
- **入力を集める（`rt-collect`）。** 当てたあとに、片付いた担当の課題 1 の結果の記録に内訳（担当。入力のトークン 11。wiki の読み 1 ページ）を、課題 2 に形の違う内訳を置き、一時の HOME の `.local/state/andon/otel/project/roles/chief-of-staff/` に今日の日付の Chief of Staff の記録（入力 5・出力 6 の行と壊れた行）を置く。担当の課題をあと 8 件片付け、振り返りの課題を親に持つ直しを、結果の記録のマージの時刻つきで片付け、マージの後に担当の課題を 9 件片付けてから collect を打ち、1 件足してもう一度打つ。`rt-collect` の行に、課題 1 の束の内訳（役が `worker`・入力 11・読んだページ `topics/tool-quirks.md`）、課題 2 の内訳 `null` と飛ばした記録、Chief of Staff の 1 日分（入力 5・出力 6）と壊れた行の飛ばした記録、直しの前後（後が 9 件のときは `short` だけ、10 件で前の実働の中央値 3.5・トークンの中央値 11）が出て、`ok   rt-collect` が見えれば動いた。
- **片付く。** 最後に `ok   片付けた` が出る。
- **報告を受ける（`rt-report`）。** 本物の振り返りの課題が `review` に入ったあと、`~/.local/state/andon/dispatcher/<slug>.delivered.jsonl` にその課題の `report` の行があり、`review` の行が無いことを読む。`hc read <Chief of Staff の address>` で、その通知を `<news>` で読んだ turn が `andon retro apply <id> --dry-run` を打ったかを見る。Chief of Staff の address は kanban の `config.yaml` の `chief_of_staff`。

## 変えたら動かすパス

- `internal/retro/`
- `package/skills/chief-of-staff/playbooks/retro.md`
- `process/workstreams/retrospective.yaml`
- `schema/retro.schema.json`
- `internal/trace/check.go`
- `internal/trace/roletokens.go`

## 既に見ている検査

- `tests/andon-retro-apply.sh`（patterns の断り・apply の当て方・Chief of Staff でない端末の断り・「Andon へ伝えるか」の課題の印 `later` と `andon waiting list` の行）
- `tests/andon-retro.sh`（`andon retro collect` の入力）
- `go test ./internal/retro/`（打ち直しで印の無いオーナーの課題に印を書くのは `apply_test.go`。全文の束の内訳と、前の直しのマージの前後の数字と、Chief of Staff のトークンは `numbers_test.go`）

## 落とし穴

- `andon retro patterns` は、課題の `agent` の端末で、`doing` の課題にだけ書ける。道は `tw` で `kanban start` してから打つ。
- 根拠は、別々の 2 件の、前の振り返りより後に片付いた課題か、オーナーの文が要る。道は、その場で片付けた担当の課題 2 つを根拠にする。
- 直す先が Andon でも、kanban の repos に Andon があれば、パターンは担当の課題になり、この道の「Andon へ伝えるか」の課題は出ない。道の kanban は repos を持たない。
- 前のバージョンの形は、打ち直しの apply がタイトルと親で見つける課題で作る。タイトルは「Andon へ伝えるか: 」とパターンの `summary` で、親は振り返りの課題。
- dispatcher が振り返りのルーチンを撃つところ（`internal/retro/retro.go` の数え方）は、この道を通らない。`dispatcher-news` の `dn-idle` が見る。
- `andon version` の `commit` は `/opt/andon/applied.json` を読む。別のバイナリを `PATH` の先頭に置いて動かしても、記録の頭の `andon:` の行の `commit` は当てた commit のままで、どのバイナリで動かしたかはパスで見る。
