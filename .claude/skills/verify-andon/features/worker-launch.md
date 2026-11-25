# 担当の起動

Chief of Staff が `andon issue start <id>` で担当を起動する道（レビュアーは dispatcher が同じ口で起動する）。起動すると、課題の worktree と branch が切られ、プロファイルのテンプレートで端末が立ち、課題の `library/launches.jsonl` に起動の記録（どのパッケージの commit で・どの command で起動したか）が1行足される。

## 細かい機能

- `wl-record` 当てたあとの起動の記録が、全部当てた commit のパッケージ（`/opt/andon`・食い違い無し）を指す。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/start.go`、`internal/issue/resume.go`、`internal/issue/lock.go`、`internal/dispatcher/loop.go`、`internal/dispatcher/fallback.go`、`internal/dispatcher/resume.go`、`internal/launch/launch.go`、`internal/pkgversion/pkgversion.go`、`internal/sessionlog/sessionlog.go`、`internal/ledger/triagemade.go`、`process/states/reviewer.yaml`
- `wl-terminal` 起動した端末のうち動いているものが、課題のラベル `issue=` を持つ。見る道: 検証の担当だけ。
  - 見るファイル: `internal/spawn/spawn.go`、`internal/spawn/term.go`、`process/states/worker.yaml`
- `wl-resume` 当てたあとに `andon issue resume` で起動し直した端末（`launch` が `resume`）の最初のメッセージの最後の段落が、送り手の印 `[resume]` で始まる（オーナーの文と読まれない）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/resume.go`、`internal/spawn/spawn.go`
- `wl-review-brief` 当てたあとに dispatcher が起動したレビュアー（`role` が `reviewer`、`launch` が `start`）の課題の依頼書が、対象の repo の `REVIEW.md` を先に読む指示を持つ。見る道: 検証の担当だけ。
  - 見るファイル: `internal/dispatcher/loop.go`
- `wl-first-turn` 起動した担当が、最初の turn で依頼書を読んで動き出し、課題に書く（テンプレート・skill・`--append-system-prompt-file` の効き）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `package/templates/`、`package/skills/worker/`、`package/.claude-plugin/`、`package/settings.json`

- `wl-wiki` セルの wiki の目録の Entities か Topics の節にページの行があるとき、当てたあとに起動した担当（`launch` が `start`）の最初のメッセージに、目録のパスを持つ1行があり、その担当が確認を求められずに目録を読める（ADR-0061 受け入れ条件 12）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/spawn/spawn.go`、`internal/issue/start.go`、`internal/issue/resume.go`、`package/skills/worker/`、`package/settings.json`

- `wl-rebuttal` 当てたあとに起動した担当が、1回目のレビューを頼む前の反論の段で、反論役のサブエージェントに repo の `REVIEW.md` のパスを渡し、その項目でも差分を壊させる（worker skill §4）。担当のセッションの記録の読み方は動かし方の `wl-rebuttal`。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `package/skills/worker/`

## 辿り着き方（利用者から）

- Chief of Staff が `andon issue start <id>` を打つ。
- dispatcher がレビューの頼みを見てレビュアーを起動する（`role` が `reviewer`、`launch` が `start`。使用量の上限で別のプロファイルに起動し直すと `fallback`）。
- 起き直した担当を Chief of Staff が `andon issue resume <id>` で起動し直す（`launch` が `resume`。`--fresh` を付けると `fresh`）。

## 動かし方

前提:

- `scripts/doctor.sh` が終了コード 0。
- 当てた時刻より後に、Chief of Staff か dispatcher が1つ以上の端末を起動している。無ければ終了コード 3（未確認）。

- **通しで動かす。** `andon issue record <自分の課題> -- bash .claude/skills/verify-andon/scripts/drive.sh worker-launch <検証する commit>` を打つ。セルの本物の `projects/*/kanban/issues/*/library/launches.jsonl` を読むだけで、何も起動しない。
- **記録を読む。** `wl-record` の行に、当てたあとの起動が1行ずつ（課題・役・`package.commit`・`package.drift`）出る。全部の `package.commit` が当てた commit で、`ok   wl-record` が見えれば動いた。
- **端末を読む。** `wl-terminal` の行に、動いている端末の `hc get` のラベルが出る。`ok   wl-terminal` が見えれば動いた。
- **レビュアーの依頼書を読む。** `wl-review-brief` の行に、当てたあとに起動したレビュアーの課題ごとに、依頼書に `REVIEW.md` を読む指示があるかが出る。`ok   wl-review-brief` が見えれば動いた。当てたあとにレビュアーが起動していなければ `note wl-review-brief` が出て、この細かい機能は未確認（ほかの行は確かめてある）。報告の `## Next` に「`Review: on` の課題のレビューが起動したあとで打ち直す」と書く。
- **resume の最初のメッセージを読む（`wl-resume`）。** 同じ `drive.sh` の出力の `wl-resume` の行を読む。道は、当てたあとの `launch` が `resume` の起動ごとに最初のメッセージを読む。Claude Code の担当は、起動の記録の `session` のセッションの記録（`<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl`）から、起動の `at` から同じ課題の次の起動か次に当てた時刻の前までの、最初のオーナーの印（`origin.kind` が `human`）の user の文を読む。貼った文の包みを外した文の頭か最後の段落の頭が `[resume] ` なら印つき（製品の `spawn.Marked` と振り返りの見分け方と同じ）。セッションの記録が読めないとき（Codex など）は、`hc read <address> --lines 5000` の画面の、その課題の「課題 `<slug>/<id>` の続き」の行が全部印つきかを参考に出すだけで、その起動は未確認にする。`ok   wl-resume` が見えれば動いた。`FAIL wl-resume` はセッションの記録の最初のメッセージに印が無いもので、壊れ。当てたあとの resume の起動が無いか、どれも確かめられなければ `note wl-resume` を出して終了コード 3（未確認）で終わる（`wl-record` と `wl-terminal` の行は確かめてある）。報告の `## Next` に「担当を `andon issue resume` で起動し直したあとで打ち直す」と書く。
- **最初の turn を読む（`wl-first-turn`）。** 検証の担当は自分で起動しない。当てたあとに Chief of Staff が起動した担当（`wl-record` に出た端末）のうち `role` が `worker`、`launch` が `start` のものを `hc read <address>` で読み、依頼書のパスの入った最初のメッセージのあとに turn が進んだことを確かめる。担当が課題に書いたことは kanban の履歴で読む。`jq -c 'select(.id == "<id>" and .by == "terminal:<address>" and .at >= "<起動の at>" and .change.status != "doing")' <kanban>/log.jsonl` に行（`report`・`ask`・`status: review` など）が1行以上あり、`<kanban>/issues/<id>/library/report.json` の `plan` が1件以上あれば、担当が skill に従って依頼書を読んで動いた。`plan` は worker skill の計画の写しで、Stop フック（`andon issue stop-check`）は報告を促すが `plan` は促さない。報告の行だけで `plan` が無ければ、skill が効いた証拠にならないので未確認と書く。`doing` の行は数えない（start が担当の端末の名前で打った行で、担当は `doing` を見て `kanban start` を打たない。worker skill §1 の 4）。1行も無ければ、まだ最初の報告の前なので未確認と書き、壊れに数えない。起動したばかりの端末が無ければ、報告の `## Next` に「試しの課題を1件起動する」と書く。
  - 検証の担当自身も、当てたあとに Chief of Staff が起動した担当にあたる（`wl-record` に自分の課題が出る）。ほかに起動した担当が無ければ、自分の端末を `hc read <自分の address>` で読む。依頼書のパスの入った最初のメッセージのあとに turn が進んだことと、最初のメッセージの起動の文が `/opt/andon/templates/<プロファイル>.md` の今の文と同じことを確かめる。見たのが自分の端末だということを `evidence` に書く。課題に書いた行は、自分の `andon issue report` の行で足りる（ふだんの手順で報告を書いてから確かめる）。証拠を作るために `kanban start` を打たない。

- **wiki の目録を読む（`wl-wiki`）。** まず `/workspace/wiki/index.md` の Entities と Topics の節に `- ` で始まる行があるかを見る（`sed -n '/^## Entities$/,/^## Sources$/p' /workspace/wiki/index.md | grep -c '^- '`）。0 なら、担当の最初のメッセージに行が付かないのが正しいので、`wl-wiki` は未確認と書く。1 以上なら、`wl-first-turn` と同じく当てたあとに Chief of Staff が起動した対話の Claude Code の担当（プロファイル `claude`・`claude-medium`。`role` が `worker`、`launch` が `start`）を `hc read <address>` で読み、次の3つを確かめる。
  1. 最初のメッセージの依頼書の行の後ろに「セルの wiki の目録 `/workspace/wiki/index.md` の Entities と Topics の節を読み、課題に当たるページがあれば読む。」の1行がある。
  2. 担当が `/workspace/wiki/index.md` を読んだ。担当のセッションの記録（`issue.yaml` の `session` の id の `<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl`）に、`wiki/index.md` を読む道具の呼び出しと、その結果（中身）がある。
  3. その読みで、確認を求める画面が出ていない。記録の結果が中身で、拒まれた・許可を待った跡が無く、画面に許可を問う枠が残っていない。
  確認を求められたら壊れとして `fail` に書き、報告の `## Next` に「ADR-0061 決めたこと 12 の `--add-dir` を足さない判断を見直す」と書く（受け入れ条件 12）。レビュアーの最初のメッセージにこの行が無いことは `go test ./internal/dispatcher/` が見ているので、画面では見ない。
  - 検証の担当自身の最初のメッセージにも、条件を満たせばこの行がある。ほかに起動した担当が無ければ、自分の端末と自分のセッションの記録で確かめ、見たのが自分だと `evidence` に書く。

- **反論の段の渡すものを読む（`wl-rebuttal`）。** 当てたあとに Chief of Staff が起動した Claude Code の担当（`role` が `worker`、`launch` が `start`）のうち、`<kanban>/issues/<id>/report.md` に `反論: サブエージェントで` で始まる行があり、PR の repo に `REVIEW.md` があるものを選ぶ。その担当のセッションの記録（`issue.yaml` の `session` の id の `<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl`）で、サブエージェントを起動する道具の呼び出し（`name` が `Agent`）の `input.prompt` に `REVIEW.md` の文字があれば動いた（`jq -c 'select(.message.content[]? | .type == "tool_use" and .name == "Agent" and (.input.prompt | test("REVIEW\\.md")))'`）。サブエージェントの呼び出しがあるのに `REVIEW.md` が無ければ壊れとして `fail` に書く。Codex などで `反論: 自分で` の行の担当は、その行に「`REVIEW.md` の項目から L 件」があれば動いた（項目を当てた数が報告に残る）。無ければ壊れとして `fail` に書く。そうした担当が無い（反論の段がまだ走っていない・`REVIEW.md` の無い repo）なら未確認と書き、報告の `## Next` に「`Review: on` か差分がレベル B・C の課題の担当が1回目のレビューを頼んだあとで打ち直す」と書く。

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
- `internal/issue/lock.go`
- `internal/dispatcher/resume.go`
- `internal/ledger/triagemade.go`
- `process/states/worker.yaml`
- `process/states/reviewer.yaml`

## 既に見ている検査

- `go test ./internal/spawn/ ./internal/issue/ ./internal/dispatcher/ ./internal/chiefengineer/`（wiki の目録の行が、担当の起動・前の会話を続けない起動し直し・dispatcher の送り直しにだけ付き、レビュアーと Chief Engineer と会話を続ける起動し直しには付かない）
- `tests/andon-issue.sh`（start を偽の hc で起動し、finish で片付くまで）
- `tests/andon-chief-of-staff.sh`
- `go test ./internal/issue/ ./internal/launch/`

## 落とし穴

- `wl-wiki` の行は、目録の Entities と Topics の節にページの行があるときだけ付く。`andon init` の雛形のままのセルや、Sources にしか行が無いセルで行が無いのは正しい。行の有無は最初のメッセージを組んだときの目録で決まる。起動のあとに目録を直しても、届いた担当の最初のメッセージは変わらない（dispatcher が送り直すときは、送るときの目録で決まる）。
- 当てる前から動いている担当は前のパッケージで動いている。`wl-record` は当てた時刻より後の行だけを見る。前の行を証拠にしない。
- `package.drift` と、記録の一番外の `drift` は別のもの。外の `drift` はセルの repo の commit していない変更でも true になるので見ない。
- `andon issue start` は起動した担当の端末の名前で `kanban start` を1回打つ。`doing` の行は `by` が担当の端末でも start が打ったもので、担当が動いた証拠にしない。担当は `doing` を見て打ち直さないので、`doing` の行の数も見ない。
- Claude Code の画面は打った command を畳む（`Ran 5 shell commands` など）。画面に `kanban start` が見えないことを、打たなかった証拠にしない。
- `wl-resume` は `launch` が `resume` の起動だけを数える。`andon issue resume --fresh`（`launch` が `fresh`）の最初のメッセージは start と同じ形で印を付けないので、印が無いのは正しい。
- Claude Code の画面は長い貼り付けを `[Pasted text #1 …]` に畳み、古い行は履歴から流れる。続きから起動すると、前の起動の印つきの文も画面に映る。画面の行は今回の起動の文と結び付けられないので、ok の証拠にも壊れの証拠にもしない。Codex の担当の `wl-resume` は未確認のまま残る。壊れと言えるのは、セッションの記録の最初のメッセージに印が無いときだけ。
- 起動の直後のセッションの記録には、道具の結果と裏の作業の知らせ（`<task-notification>`）が user の行として最初のメッセージより先に入る。オーナーの印（`origin.kind` が `human`）の無い行を最初のメッセージと読まない。
- 片付いた端末は `hc get` で取れない。`wl-terminal` は片付いたことを行に出すだけで、落ちとしない。
