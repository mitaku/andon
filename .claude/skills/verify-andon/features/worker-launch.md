# 担当の起動

Chief of Staff が `andon issue start <id>` で担当を起動する道（レビュアーは dispatcher が同じ口で起動する）。起動すると、課題の worktree と branch が切られ、プロファイルのテンプレートで端末が立ち、課題の `library/launches.jsonl` に起動の記録（どのパッケージの commit で・どの command で起動したか）が1行足される。

## 細かい機能

- `wl-record` 当てたあとの起動の記録が、全部当てた commit のパッケージ（`/opt/andon`・食い違い無し）を指す。見る道: 検証の担当だけ。
- `wl-terminal` 起動した端末のうち動いているものが、課題のラベル `issue=` を持つ。見る道: 検証の担当だけ。
- `wl-first-turn` 起動した担当が、最初の turn で依頼書を読んで動き出し、課題に書く（テンプレート・skill・`--append-system-prompt-file` の効き）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。

## 辿り着き方（利用者から）

- Chief of Staff が `andon issue start <id>` を打つ。
- dispatcher がレビューの頼みを見てレビュアーを起動する（`role` が `reviewer`、`launch` が `start`。使用量の上限で別のプロファイルに起動し直すと `fallback`）。
- 起き直した担当を Chief of Staff が `andon issue resume <id>` で起動し直す（`launch` が `resume`。`--fresh` を付けると `fresh`）。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff か dispatcher が1つ以上の端末を起動している。無ければ終了コード 3（未確認）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh worker-launch` を打つ。セルの本物の `projects/*/kanban/issues/*/library/launches.jsonl` を読むだけで、何も起動しない。
- **記録を読む。** `wl-record` の行に、当てたあとの起動が1行ずつ（課題・役・`package.commit`・`package.drift`）出る。全部の `package.commit` が当てた commit で、`ok   wl-record` が見えれば動いた。
- **端末を読む。** `wl-terminal` の行に、動いている端末の `hc get` のラベルが出る。`ok   wl-terminal` が見えれば動いた。
- **最初の turn を読む（`wl-first-turn`）。** 検証の担当は自分で起動しない。当てたあとに Chief of Staff が起動した担当（`wl-record` に出た端末）のうち `role` が `worker`、`launch` が `start` のものを `hc read <address>` で読み、依頼書のパスの入った最初のメッセージのあとに turn が進んだことを確かめる。担当が課題に書いたことは kanban の履歴で読む。`jq -c 'select(.id == "<id>" and .by == "terminal:<address>" and .at >= "<起動の at>" and .change.status != "doing")' <kanban>/log.jsonl` に行（`report`・`ask`・`status: review` など）が1行以上あれば、担当が依頼書を読んで動いた。`doing` の行は数えない（start が担当の端末の名前で打った行で、担当は `doing` を見て `kanban start` を打たない。worker skill §1 の 4）。1行も無ければ、まだ最初の報告の前なので未確認と書き、壊れに数えない。起動したばかりの端末が無ければ、報告の `## Next` に「試しの課題を1件起動する」と書く。
  - 検証の担当自身も、当てたあとに Chief of Staff が起動した担当にあたる（`wl-record` に自分の課題が出る）。ほかに起動した担当が無ければ、自分の端末を `hc read <自分の address>` で読む。依頼書のパスの入った最初のメッセージのあとに turn が進んだことと、最初のメッセージの起動の文が `/opt/andon/templates/<プロファイル>.md` の今の文と同じことを確かめる。見たのが自分の端末だということを `evidence` に書く。課題に書いた行は、自分の `andon issue report` の行で足りる（ふだんの手順で報告を書いてから確かめる）。証拠を作るために `kanban start` を打たない。

## 変えたら動かすパス

- `internal/issue/start.go`
- `internal/issue/resume.go`
- `internal/dispatcher/loop.go`
- `internal/dispatcher/fallback.go`
- `internal/launch/`
- `internal/spawn/`
- `internal/sessionlog/`
- `internal/pkgversion/`
- `package/templates/`
- `package/skills/worker/`
- `package/.claude-plugin/`
- `package/settings.json`

## 既に見ている検査

- `tests/andon-issue.sh`（start を偽の hc で起動し、finish で片付くまで）
- `tests/andon-chief-of-staff.sh`
- `go test ./internal/issue/ ./internal/launch/`

## 落とし穴

- 当てる前から動いている担当は前のパッケージで動いている。`wl-record` は当てた時刻より後の行だけを見る。前の行を証拠にしない。
- `package.drift` と、記録の一番外の `drift` は別のもの。外の `drift` はセルの repo の commit していない変更でも true になるので見ない。
- `andon issue start` は起動した担当の端末の名前で `kanban start` を1回打つ。`doing` の行は `by` が担当の端末でも start が打ったもので、担当が動いた証拠にしない。担当は `doing` を見て打ち直さないので、`doing` の行の数も見ない。
- Claude Code の画面は打った command を畳む（`Ran 5 shell commands` など）。画面に `kanban start` が見えないことを、打たなかった証拠にしない。
- 片付いた端末は `hc get` で取れない。`wl-terminal` は片付いたことを行に出すだけで、落ちとしない。
