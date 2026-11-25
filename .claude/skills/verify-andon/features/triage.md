# トリアージ

トリアージの担当は、kanban に溜まったもの（報告の Next・Remember・オーナーに聞くことなど）を `andon triage collect` で集め、項目ごとの判定を `library/decisions.json` に書く。Chief of Staff は `andon triage apply <id>` で判定を kanban に当てる。オーナーへ回す判定（`person`）は、急がないなら置いておく理由の 1 行を、呼ぶなら `false` を、キー `later` に必ず持つ。理由のある判定は、オーナーの課題に印 `later`（と `later_until`）を書いて足し、`andon waiting` で呼ばずに置いてある行になる。`false` の判定は印を持たず、呼ぶ行になる。印を書く apply は Chief of Staff の端末でだけ通り、`later` の無い前の形の `person` は何も書かずに断る。

## 細かい機能

- `tr-collect` `andon triage collect` が、片付いた担当の報告の「オーナーに聞くこと」を `question` の項目として出す。見る道: 検証の担当だけ。
  - 見るファイル: `internal/triage/triage.go`、`internal/triage/collect.go`、`internal/triage/memory.go`、`internal/triage/signals.go`
- `tr-later` Chief of Staff の端末の apply は、`later` に理由のある判断と手作業の `person` を、印 `later`（と `later_until`）を持つオーナーの課題にし、`andon waiting --json` でその行が `call: false` になる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/triage/apply.go`、`internal/triage/decisions.go`、`internal/triage/verdict.go`
- `tr-call` `later` が `false` の判断は、印を持たないオーナーの課題になり、`andon waiting --json` でその行が `call: true` になる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/triage/apply.go`、`internal/triage/decisions.go`
- `tr-not-cos` 印を書く判定を Chief of Staff でない端末で当てると、Chief of Staff の端末だけと言って断り、課題も記録も増やさない。見る道: 検証の担当だけ。
  - 見るファイル: `internal/triage/apply.go`
- `tr-old-shape` `later` の無い前の形の `person` は、Chief of Staff の端末でも `later` が無いと言って断り、課題も記録も増やさない。`person` の無い前の形の判定（`keep` だけ）は、印を書かないので担当の端末でも当たる。見る道: 検証の担当だけ。
  - 見るファイル: `internal/triage/decisions.go`、`internal/triage/apply.go`
- `tr-report` PR の無いトリアージの課題が `review` に入ると、dispatcher は Chief of Staff へ通知 `<id> report` を届け（`review` ではない）、Chief of Staff は手引きの 3 のとおり課題が `review` なのを読んで `andon triage apply <id> --dry-run` を打つ。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `package/skills/chief-of-staff/playbooks/triage.md`、`process/workstreams/triage.yaml`

## 辿り着き方（利用者から）

- dispatcher がルーチン triage を撃ち、トリアージの担当が `andon triage collect` で集めて `library/items.json` に置き、判定を `library/decisions.json` に書く（skill `triage`）。
- Chief of Staff が `andon triage apply <id> --dry-run` で読んでから、`andon triage apply <id>` で当てる（手引き `package/skills/chief-of-staff/playbooks/triage.md`）。
- オーナーと役が `andon waiting` で、足したオーナーの課題を、呼んでいる行か置いてある行として読む。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh triage <検証する commit>` を打つ。切り離した kanban に、「オーナーに聞くこと」を持つ報告の片付いた担当の課題を 3 つと、トリアージの課題を 2 つ作る。Chief of Staff の端末は `tc`、担当の端末は `tw`。
- **集める。** `tr-collect` の行に、`andon triage collect` の出力と 3 つの `question` の項目が出る。`ok   tr-collect` が見えれば動いた。
- **前の形を読む。** `tr-old-shape` の行に、`later` の無い `person` の断り（「person に later が無い」）と、`keep` だけの判定を `tw` で当てた記録の表が出る。`ok   tr-old-shape` が2行見えれば動いた。
- **Chief of Staff でない端末を断る。** `tr-not-cos` の行に、`tw` の apply の断り（「later で置いておく印を書くのは Chief of Staff の端末だけ」）が出る。`ok   tr-not-cos` が見えれば動いた。
- **置いておく・呼ぶ。** `tr-later`・`tr-call` の行に、`tc` の apply の記録の表と、足したオーナーの課題の `who`・`only_owner`・`later`・`later_until` と、`andon waiting --json` の行（`kind`・`call`・`later`）が出る。`ok   tr-later` が2行と `ok   tr-call` が1行見えれば動いた。
- **片付く。** 最後に `ok   片付けた` が出る。
- **報告を受ける（`tr-report`）。** 本物のトリアージの課題が `review` に入ったあと、`~/.local/state/andon/dispatcher/<slug>.delivered.jsonl` にその課題の `report` の行があり、`review` の行が無いことを読む。`hc read <Chief of Staff の address>` で、その通知を `<news>` で読んだ turn が `andon triage apply <id> --dry-run` を打ったかを見る。Chief of Staff の address は kanban の `config.yaml` の `chief_of_staff`。

## 変えたら動かすパス

- `internal/triage/`
- `package/skills/chief-of-staff/playbooks/triage.md`
- `process/workstreams/triage.yaml`

## 既に見ている検査

- `tests/andon-triage.sh`（collect の8種類の項目・apply の判定ごとの当て方・Chief of Staff でない端末の印を断る・`later` の印と `andon waiting` の行）
- `go test ./internal/triage/`（`later` の形の断りは `person_test.go`、1 件目の閉じは `gap_test.go`）
- `bash tests/package.sh` の `tests/package/41-triage-later.sh`（skill `triage` の置いておく基準の文）

## 落とし穴

- `andon triage collect` は端末の一覧（`hc ls`）を読む。偽の `hc` が一覧を返さないと「端末一覧を読めない」で止まる。道は、偽の `hc` の `ls` に空の一覧を返させる。
- apply は同じトリアージの課題を同じ日に 2 回当てられない（「このトリアージは記録済み」）。前の形と今の形を、別のトリアージの課題で当てる。
- apply の記録は `kanban/library/triage/` に `.json` と `.md` を 1 つずつ置く。記録の数は `.json` で数える。
- `later` の無い前の形の `person` が断られるのは、ADR-0062 決めたこと 10 のとおりの振る舞いで、壊れではない。前の形のうち `person` を持たない判定だけが、そのまま当たる。
- `andon version` の `commit` は `/opt/andon/applied.json` を読む。別のバイナリを `PATH` の先頭に置いて動かしても、記録の頭の `andon:` の行の `commit` は当てた commit のままで、どのバイナリで動かしたかはパスで見る。
- dispatcher がトリアージのルーチンを撃つところ（`internal/triage/unsorted.go` で数える）は、この道を通らない。`dispatcher-news` の `dn-idle` が見る。
