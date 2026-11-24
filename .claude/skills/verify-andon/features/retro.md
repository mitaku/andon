# 振り返り

振り返りの担当は、`andon retro collect` で集めた記録から繰り返し（パターン）を拾い、`andon retro patterns <id> -` で `library/retro.json` に書く。Chief of Staff は `andon retro apply <id>` で、`fix` のパターンを直す課題にする。直す先が Andon で、この kanban の repos に Andon が無ければ、パターンは「Andon へ伝えるか」のオーナーの課題になる。その課題は、決まった理由の印 `later`（「振り返りの直しの提案で、答えを待って止まる仕事が無い」）を持ち、`andon waiting` で呼ばずに置いてある行になる。オーナーの課題を足す apply は Chief of Staff の端末でだけ通る。前のバージョンの apply が `who: person` だけ書いて落ちた課題は、打ち直しで足し直さずに印と問いを書く。

## 細かい機能

- `rt-patterns` 振り返りの担当の端末が `andon retro patterns` で、集め直した記録と照らしたパターンの一覧を `library/retro.json` に書く。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/patterns.go`、`internal/retro/retro.go`、`internal/retro/typed.go`、`internal/retro/questions.go`、`internal/retro/github.go`
- `rt-later` Chief of Staff の端末の apply は、Andon の直しを「Andon へ伝えるか」のオーナーの課題にし、印 `later` と問いを書く。`andon waiting --json` でその行が `call: false` になる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/apply.go`
- `rt-not-cos` オーナーの課題を足す apply を Chief of Staff でない端末で打つと、Chief of Staff の端末だけと言って断り、課題も当てた記録も増やさない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/apply.go`
- `rt-old-owner` 前のバージョンの apply が `who: person` だけ書いて落ちた課題（呼ぶ行）を、打ち直しで足し直さずに使い、印 `later` と問いを書いて置いてある行にする。見る道: 検証の担当だけ。
  - 見るファイル: `internal/retro/apply.go`

## 辿り着き方（利用者から）

- dispatcher がルーチン retro を撃ち、振り返りの担当が `andon retro collect` で集め、`andon retro patterns <id> -` で一覧を書く（skill `retro`）。
- Chief of Staff が `andon retro apply <id> --dry-run` で読んでから、`andon retro apply <id>` で当てる（手引き `package/skills/chief-of-staff/playbooks/retro.md`）。
- オーナーと役が `andon waiting` で、「Andon へ伝えるか」の課題を置いてある行として読む。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh retro <検証する commit>` を打つ。切り離した kanban（repos が無い）に、片付いた担当の課題を 2 つと、担当の端末 `tw` が動かす振り返りの課題を作る。Chief of Staff の端末は `tc`。
- **一覧を書く。** `rt-patterns` の行に、Andon の直しのパターン 2 つ（`fix`）と捨てるパターン 1 つ（`drop`）を書いた出力が出る。`ok   rt-patterns` が見えれば動いた。
- **Chief of Staff でない端末を断る。** `rt-not-cos` の行に、`tw` の apply の断り（「置いておく印を書くのは Chief of Staff の端末だけ」）が出る。`ok   rt-not-cos` が見えれば動いた。
- **前のバージョンの形を作る。** `tc` が、2つ目のパターンの「Andon へ伝えるか」の課題を振り返りの課題の下に足し、`who: person` だけを書く。`rt-old-owner` の行に、打ち直しの前の `andon waiting --json` の行（`call: true`）が出る。
- **置いておく。** `rt-later`・`rt-old-owner` の行に、`tc` の apply の出力と、2つのオーナーの課題の `who`・`parent`・`later` と問い、`andon waiting --json` の行（`call`・`later`）が出る。`ok   rt-later` が1行と `ok   rt-old-owner` が2行見えれば動いた。
- **片付く。** 最後に `ok   片付けた` が出る。

## 変えたら動かすパス

- `internal/retro/`

## 既に見ている検査

- `tests/andon-retro-apply.sh`（patterns の断り・apply の当て方・Chief of Staff でない端末の断り・「Andon へ伝えるか」の課題の印 `later` と `andon waiting` の行）
- `tests/andon-retro.sh`（`andon retro collect` の入力）
- `go test ./internal/retro/`（打ち直しで印の無いオーナーの課題に印を書くのは `apply_test.go`）

## 落とし穴

- `andon retro patterns` は、課題の `agent` の端末で、`doing` の課題にだけ書ける。道は `tw` で `kanban start` してから打つ。
- 根拠は、別々の 2 件の、前の振り返りより後に片付いた課題か、オーナーの文が要る。道は、その場で片付けた担当の課題 2 つを根拠にする。
- 直す先が Andon でも、kanban の repos に Andon があれば、パターンは担当の課題になり、この道の「Andon へ伝えるか」の課題は出ない。道の kanban は repos を持たない。
- 前のバージョンの形は、打ち直しの apply がタイトルと親で見つける課題で作る。タイトルは「Andon へ伝えるか: 」とパターンの `summary` で、親は振り返りの課題。
- dispatcher が振り返りのルーチンを撃つところ（`internal/retro/retro.go` の数え方）は、この道を通らない。`dispatcher-news` の `dn-idle` が見る。
- `andon version` の `commit` は `/opt/andon/applied.json` を読む。別のバイナリを `PATH` の先頭に置いて動かしても、記録の頭の `andon:` の行の `commit` は当てた commit のままで、どのバイナリで動かしたかはパスで見る。
