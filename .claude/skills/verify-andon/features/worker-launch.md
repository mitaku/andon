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

- `wl-next` 当てたあとに `andon issue start <id>` で起動すると、何をしたかの行（`<id> を <address>（…）で起動した。`）のあと、最後の `出た場所` の行の手前に、次の1手の行（`次: 報告と問いは dispatcher が届ける（止まれば dispatcher が1回起動し直す）。流れは andon issue trace <id> --project <slug> で見る`）を出す。`andon issue resume` と `andon design start` と `andon open` も次の1手の行を出す（ADR-0053 決めたこと 14）。次の1手は読むだけの葉で、Chief of Staff に `andon issue resume` を勧めない。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/issue/start.go`、`internal/issue/resume.go`、`internal/chiefengineer/chiefengineer.go`、`internal/chiefofstaff/chiefofstaff.go`

- `wl-wiki` セルの wiki の目録の Entities か Topics の節にページの行があるとき、当てたあとに起動した担当（`launch` が `start`）の最初のメッセージに、目録のパスを持つ1行があり、その担当が確認を求められずに目録を読める（ADR-0061 受け入れ条件 12）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/spawn/spawn.go`、`internal/issue/start.go`、`internal/issue/resume.go`、`package/skills/worker/`、`package/settings.json`

- `wl-rebuttal` 当てたあとに起動した担当が、1回目のレビューを頼む前の反論の段で、反論役のサブエージェントに repo の `REVIEW.md` のパスを渡し、その項目でも差分を壊させる。その項目のうち当てたあとの検証の道の疑いは、落ちる検査を書けなくても捨てず、地図に足すか、足さない理由を PR のトレードオフに書く。落ちる検査を書けて直した疑いは、同じ道を通る変種にも検査を書いて当て、捨てる疑いは、その入力で書いた検査が通るのを見てから捨てる（worker skill §4）。担当のセッションの記録の読み方は動かし方の `wl-rebuttal`。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `package/skills/worker/`

- `wl-tools` worktree の根に `mise.toml` がある課題で、当てたあとに起動した担当・レビュアー・Chief Engineer の端末が、誰も `mise install` を打たずに `mise.toml` のバージョンの道具を引く。端末の環境の `MISE_SYSTEM_CONFIG_FILE` が自分の worktree の `mise.toml` を指し、一時の置き場でも同じバージョンを引く（ADR-0063 決めたこと 6・10）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/tools/tools.go`、`internal/tools/env.go`、`internal/cli/leaves.go`、`internal/issue/start.go`、`internal/issue/resume.go`、`internal/dispatcher/loop.go`、`internal/dispatcher/resume.go`、`internal/chiefengineer/chiefengineer.go`、`package/templates/`

- `wl-settings` 当てたパッケージの `andon issue create` が、予約キーでないキーを課題を足さずに断り、`--dry-run` で予約キーから書いた依頼書の `## Settings` 欄と、足りないもの（profile）を書く次の1手を出す。当てたあとに足した起動前の課題の依頼書が、予約キーから書き直した欄と同じ（ADR-0053 受け入れ条件 A9・A10）。見る道: 検証の担当だけ。
  - 見るファイル: `internal/issue/create.go`、`internal/issue/start.go`、`internal/issue/resume.go`、`internal/ledger/ledger.go`、`internal/cli/leaves.go`、`package/skills/chief-of-staff/playbooks/brief.md`、`package/skills/chief-of-staff/playbooks/cut.md`、`package/skills/chief-of-staff/playbooks/start.md`、`package/skills/chief-of-staff/playbooks/verify.md`

- `wl-stacked` 当てたあとに `andon issue start <id>` を積める `queued` の課題に打つと、前の課題の PR の head から branch を切り、`stacked_on`・`stacked_base` を Chief of Staff の端末で書いて `doing` にし、最初のメッセージに前の課題の branch の上に積んだことと `gh pr create --draft --base <前の課題の branch>` を足す。積めない課題（前の課題が `doing`・PR が open でない・origin の branch の先が gate の通した commit と違う）は理由1行で断り、worktree も branch もキーも残さない（ADR-0067 決めたこと 4・5・7）。見る道: Chief of Staff が試しの課題を起動し、検証の担当が画面を読む。
  - 見るファイル: `internal/issue/start.go`、`internal/spawn/spawn.go`、`internal/waiting/stack.go`、`process/states/issue.yaml`、`package/skills/chief-of-staff/playbooks/start.md`、`package/skills/worker/`

## 辿り着き方（利用者から）

- Chief of Staff が `andon issue create "<タイトル>" … <予約キー>…` で担当の課題を足し、`andon issue start <id>` を打つ。
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

- **起動の出力を読む（`wl-next`）。** 検証の担当は自分で起動しない。当てたあとに Chief of Staff が `andon issue start` を打った画面を、Chief of Staff の端末（kanban の `config.yaml` の `chief_of_staff`）の `hc read <address>` で読む。`andon issue: <id> を <address>（…）で起動した。` の行のすぐ後に `andon issue: 次: 報告と問いは dispatcher が届ける（止まれば dispatcher が1回起動し直す）。流れは andon issue trace <id> --project <slug> で見る` の行があり、その次が最後の `andon issue: 出た場所 …` の行なら動いた。
  - 検証の担当自身の起動も、当てたあとに Chief of Staff が打った `andon issue start` にあたる。画面から流れて読めなければ未確認と書き、報告の `## Next` に「次に担当を起動したとき、出力の最後の3行を検証の課題に写す」と書く。
  - `andon design start` を当てたあとに Chief of Staff が打っていれば、同じ画面で `andon design start: 次: … 流れは andon issue trace <id> --project <slug> で見る` の行が最後の `出た場所` の行の手前にあるかを読む。打っていなければ未確認と書く。
  - `andon open` は、オーナーが手元の端末で打つ。当てたあとにオーナーが開き直していれば、報告の `acceptance[]` に `manual` と書き、`evidence` に「`andon open` の出力の最後の2行が `andon open: 次: … andon waiting list --project <slug> で出す` と `andon open: 出た場所 …`」と見る場所を書く。開き直していなければ未確認と書く。
  - `andon issue resume` は dispatcher が打ち、出力を捨てるので画面に出ない。Chief of Staff は自分では打たない。画面で確かめられないので未確認と書き、`tests/andon-issue.sh` の resume の段（最後の行が次の1手）を証拠の代わりにしない。報告の `## Next` に「オーナーの go で `andon issue resume --fresh` を打ったとき、出力の最後の行を検証の課題に写す」と書く。

- **wiki の目録を読む（`wl-wiki`）。** まず `/workspace/wiki/index.md` の Entities と Topics の節に `- ` で始まる行があるかを見る（`sed -n '/^## Entities$/,/^## Sources$/p' /workspace/wiki/index.md | grep -c '^- '`）。0 なら、担当の最初のメッセージに行が付かないのが正しいので、`wl-wiki` は未確認と書く。1 以上なら、`wl-first-turn` と同じく当てたあとに Chief of Staff が起動した対話の Claude Code の担当（プロファイル `claude`・`claude-medium`。`role` が `worker`、`launch` が `start`）を `hc read <address>` で読み、次の3つを確かめる。
  1. 最初のメッセージの依頼書の行の後ろに「セルの wiki の目録 `/workspace/wiki/index.md` の Entities と Topics の節を読み、課題に当たるページがあれば読む。」の1行がある。
  2. 担当が `/workspace/wiki/index.md` を読んだ。担当のセッションの記録（`issue.yaml` の `session` の id の `<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl`）に、`wiki/index.md` を読む道具の呼び出しと、その結果（中身）がある。
  3. その読みで、確認を求める画面が出ていない。記録の結果が中身で、拒まれた・許可を待った跡が無く、画面に許可を問う枠が残っていない。
  確認を求められたら壊れとして `fail` に書き、報告の `## Next` に「ADR-0061 決めたこと 12 の `--add-dir` を足さない判断を見直す」と書く（受け入れ条件 12）。レビュアーの最初のメッセージにこの行が無いことは `go test ./internal/dispatcher/` が見ているので、画面では見ない。
  - 検証の担当自身の最初のメッセージにも、条件を満たせばこの行がある。ほかに起動した担当が無ければ、自分の端末と自分のセッションの記録で確かめ、見たのが自分だと `evidence` に書く。

- **反論の段の渡すものを読む（`wl-rebuttal`）。** 当てたあとに Chief of Staff が起動した Claude Code の担当（`role` が `worker`、`launch` が `start`）のうち、`<kanban>/issues/<id>/report.md` に `反論: サブエージェントで` で始まる行があり、PR の repo に `REVIEW.md` があるものを選ぶ。その担当のセッションの記録（`issue.yaml` の `session` の id の `<CLAUDE_CONFIG_DIR>/projects/*/<session>.jsonl`）で、反論の段の呼び出しだけを選ぶ。サブエージェントを起動する道具の呼び出し（`name` が `Agent`）のうち、`input.prompt` に worker skill §4 の頼み方の文「この差分を壊せ」があるものが反論の段の呼び出しで、それ以外の Agent の呼び出し（調べもの・別のレビューなど）は数えない。反論の段の呼び出しごとに、同じ `input.prompt` に `REVIEW.md` のパスと、その項目を使う指示（「`REVIEW.md` の項目」の文字）の両方があるかを見る（`jq -c '.message.content[]? | select(.type == "tool_use" and .name == "Agent" and (.input.prompt | test("この差分を壊せ"))) | {path: (.input.prompt | test("REVIEW\\.md")), items: (.input.prompt | test("REVIEW\\.md の項目"))}'`）。反論の段の呼び出しが1つ以上あり、どれも両方が `true` なら動いた。どれかに片方でも `false` があれば、ほかの Agent の呼び出しに `REVIEW.md` があっても、壊れとして `fail` に書く。反論の段の呼び出しが無い担当は、この細かい機能の対象にしない。Codex などで `反論: 自分で` の行の担当は、その行に「`REVIEW.md` の項目から L 件」があれば動いた（項目を当てた数が報告に残る）。無ければ壊れとして `fail` に書く。
  - 検証の道の疑い: 同じ担当のセッションの記録で、反論の段の呼び出しの結果（その Agent の `tool_use_id` の `tool_result`）に、`REVIEW.md` の検証の道の項目（Andon の repo では項目 10。当てたあとの検証の道・地図・細かい機能の段が変えた振る舞いを通らない、という疑い）から出た疑いがあるかを見る。無ければこの見方の対象にしない。あれば、`反論:` の行に「検証の道の疑いは、地図に足した J 件（…）と、足さずにトレードオフに書いた G 件」があるかを見て、無ければ捨てない決まりに反したので壊れとして `fail` に書く（既にある細かい機能が通していて捨てた疑いだけなら、J と G が 0 の行がある）。J が 1 以上なら、行の括弧に疑いごとの細かい機能の名前が J 個あり、名前ごとに、PR の差分（`gh pr diff <PR>`）がその細かい機能の行か動かし方の手順か段（その repo の地図の置き場の下。Andon の repo では `.claude/skills/verify-andon/`）を変え、変えた中身が反論役の返した疑いの振る舞いを打つか、その見える証拠を挙げることを読む。地図の置き場の下が変わっていても、疑いと別の細かい機能だけなら合わない。G が 1 以上なら PR の本文の「トレードオフ」（`Tradeoffs`）の節に検証の道を足さない理由があることを見る。全部合えば動いた。合わなければ壊れとして `fail` に書く。
  - 変種と捨てた疑い: 同じ担当の `反論:` の行に「直した疑いの変種は V 件当て（…）、うち落ちて直した U 件」と「捨てた K 件（…）」があるかを見る。当てたあとの担当の行に無ければ、件数だけの前の形なので壊れとして `fail` に書く。M（直した疑い）も K（捨てた疑い）も 0 なら、足した手順を通っていないので、この見方は未確認と書く（行の形が合うだけで動いたにしない）。報告の `## Next` に「反論の段で疑いを直すか捨てた担当が出たあとで打ち直す」と書く。
    - 変種: V が 1 以上なら、括弧に変種ごとの検査の名前が V 個あり、名前ごとに PR の差分（`gh pr diff <PR>`）がその検査を足すか変えることを読む（通った変種の検査も残すので、U が 0 でも名前は差分にある）。名前が V 個に足りないか、差分に無い名前があれば壊れとして `fail` に書く。M が 1 以上で V が 0 なら、反論の段の呼び出しの結果にある直した疑いの入力を読み、同じ関数か同じ照合を通る別の形の入力（読めない入力に対する探索できない入力、見出しの照合に対するコードフェンスの中の行、件数の照合に対する件数だけ合う別の中身など）が書けるなら、変種を当てなかったので壊れとして `fail` に書く。
    - 捨てた疑い: K が 1 以上なら、括弧に捨てた疑いごとの名前か理由が K 個あるかを見る。検査の名前は、PR の差分にその検査があり、反論役の返した起きる条件の入力を使うことを読む。理由は、反論役の返した起きる条件がセルで作れない入力（セルに無い権限や外のサービスが要るなど）か、入力と結果で書けない読んで決まる疑い（文書の読み手・設計との食い違い・決定の記録の有無など）であることを読む。細かい機能の名前は、検証の道の疑いを地図で確かめて捨てたものとして上の見方で読む。反論役が具体的な入力と結果を返したのに、検査が無く理由だけで捨てた疑いがあれば、壊れとして `fail` に書く。
  - そうした担当が無い（反論の段がまだ走っていない・`REVIEW.md` の無い repo）なら未確認と書き、報告の `## Next` に「`Review: on` か差分がレベル B・C の課題の担当が1回目のレビューを頼んだあとで打ち直す」と書く。

- **道具を読む（`wl-tools`）。** 案件の repo の根に `mise.toml` がある課題で、当てたあとに Chief of Staff が起動した担当（`wl-record` に出た端末）か、dispatcher が起動したそのレビュアーの端末か、Chief of Staff が `andon design start "<タイトル>"` で起動した Chief Engineer の端末（worktree は案件の repo の Chief Engineer の worktree）で、次の3つを打ってもらうか、検証の担当が自分の端末で打つ。
  1. `printenv MISE_SYSTEM_CONFIG_FILE` が、その課題の kanban の `worktree` の根の `mise.toml` を指す（レビュアーは cwd が `issues/<課題>/` でも、PR の head の worktree を指す）。
  2. worktree の中の `go version` と、`cd "$(mktemp -d)" && go version` の両方が、`mise.toml` の go のバージョンを出す。
  3. 起動を打った Chief of Staff の出力か dispatcher の端末のログ（`hc read dispatcher`）に `mise install` の失敗の警告が無い。道具が消えたあとの最初の起動なら「道具を入れた」の1行がある。
  見たのが自分の端末なら、そう `evidence` に書く。
  道具がもう入っているセルでは、1 と 2 は入れる手を通らなくても ok になる。入れる手（「道具を入れた」の1行と、入れられないときの警告）を通すには、試しの課題の branch の `mise.toml` に、セルにまだ入っていない小さな道具のバージョンを足してから起動してもらい、起動を打った Chief of Staff の出力か dispatcher の端末のログに「道具を入れた」の1行が出ることを見る。dispatcher が `andon issue resume` で起動し直した回の行は、dispatcher の端末のログに「<課題> の resume: 」を頭に付けて出る。行が無ければ、入れる手は未確認と書き、1 と 2 だけを ok にする。

- **依頼書の欄を読む（`wl-settings`）。** 同じ `drive.sh` の出力の `wl-settings` の行を読む。道は、当てたパッケージの `andon` で、`andon issue create "verify wl-settings" who=person --dry-run` が終了コード 1 で断ることと、`--brief - review=off estimate=small --dry-run` が `- Review: off`・`- Estimate: 小` の欄と `kanban set <足した id> 'profile=<名前>'` の次の1手を出すことを見る（どちらも何も書かない）。続けて、当てたあとに足した（`log.jsonl` の `change.add` の行）課題のうち、`todo` で、予約キーを書いた行が1行だけ（足した直後の分。あとから `kanban set` で変えた値は次の起動まで欄に写らないのが正しいので比べない）のものを選び、依頼書が `andon issue start <id> --dry-run --json` の `.brief`（予約キーから書き直した欄）と同じで `## Settings` の見出しを持つかを見る。起動の確かめで断る課題は比べない。`ok   wl-settings` が見えれば動いた。`ok` は、`--dry-run` の確かめと依頼書の比べの両方を通したときだけ出る。セルにプロジェクトが無いか、比べられる課題が無ければ `note wl-settings` だけが出て未確認になる。そのときは報告の `## Next` に「Chief of Staff が `andon issue create` で課題を足したあとで打ち直す」と書く。

- **積んで起動したかを読む（`wl-stacked`）。** 検証の担当は自分で起動しない。当てたあとに Chief of Staff が、`andon waiting list --json` の `stackable` に出た課題へ `andon issue start` を打ったときだけ確かめられる（積んだ列を作るのは ADR-0067 受け入れ条件 14 の課題）。打った課題ごとに次を読む。
  1. `kanban show <id> --json` の `status` が `doing` で、`stacked_on` が `stackable` の `on`、`stacked_base` が前の課題の `report.json` の `commit` と同じ。`log.jsonl` の `stacked_on` を書いた行の `by` が Chief of Staff の端末。
  2. 課題の worktree で `git merge-base HEAD <stacked_base>` が `stacked_base` を返す（branch の根元が前の課題の PR の head）。
  3. 担当の最初のメッセージ（`hc read <担当の address>` か、起動を打った Chief of Staff の画面）に「前の課題 `<on>` の PR の branch `<branch>` の上に積んで起動した」と `gh pr create --draft --base <branch>` の行がある。
  4. 積んだ担当が PR を出したら、`gh pr view <PR> --json isDraft,baseRefName` が `true` と前の課題の branch を返す（worker skill §3 の積んだ課題の出し方）。Chief of Staff の画面で、同じ優先の課題があったときに積まずに起動できる課題を先に打ち、積む課題と前の課題の PR の重なりで起動を止めず、起動を知らせる返事に前の課題の上に積んだことを言い添えたかも読む（`start.md` の 1 と 4）。担当の画面で、反論の段に渡した差分が `git diff <stacked_base>..HEAD` で、積んでいる間に rebase を打たず、PR の前と `andon issue submit` の前に `andon issue restack` を打ったかも読む（worker skill §3）。
  積めない課題に打った画面があれば、理由の1行（「積んでも起動できない: …」「Chief of Staff の端末（…）だけ」など）が出て、`kanban show` の `branch`・`worktree`・`stacked_on` が空のままかも読む。当てたあとに積む起動が1つも無ければ未確認と書き、報告の `## Next` に「次に積む起動を打ったとき wl-stacked を読む」と書く。

## 変えたら動かすパス

- `internal/tools/`
- `internal/cli/leaves.go`
- `internal/chiefengineer/chiefengineer.go`
- `internal/chiefofstaff/chiefofstaff.go`
- `internal/issue/start.go`
- `internal/issue/resume.go`
- `internal/issue/create.go`
- `internal/ledger/ledger.go`
- `package/skills/chief-of-staff/playbooks/brief.md`
- `package/skills/chief-of-staff/playbooks/cut.md`
- `package/skills/chief-of-staff/playbooks/start.md`
- `package/skills/chief-of-staff/playbooks/verify.md`
- `internal/dispatcher/loop.go`
- `internal/dispatcher/fallback.go`
- `internal/launch/`
- `internal/spawn/`
- `internal/waiting/stack.go`
- `process/states/issue.yaml`
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
- `tests/andon-issue.sh`（start を偽の hc で起動し、finish で片付くまで。start と resume が出た場所の手前に出す次の1手の行）
- `go test ./internal/chiefengineer/`（design start が出た場所の手前に出す次の1手の行）
- `tests/andon-chief-of-staff.sh`
- `go test ./internal/issue/ ./internal/launch/`

## 落とし穴


- `wl-wiki` の行は、目録の Entities と Topics の節にページの行があるときだけ付く。`andon cell init` の雛形のままのセルや、Sources にしか行が無いセルで行が無いのは正しい。行の有無は最初のメッセージを組んだときの目録で決まる。起動のあとに目録を直しても、届いた担当の最初のメッセージは変わらない（dispatcher が送り直すときは、送るときの目録で決まる）。
- 当てる前から動いている担当は前のパッケージで動いている。`wl-record` は当てた時刻より後の行だけを見る。前の行を証拠にしない。
- `package.drift` と、記録の一番外の `drift` は別のもの。外の `drift` はセルの repo の commit していない変更でも true になるので見ない。
- `andon issue start` は起動した担当の端末の名前で `kanban start` を1回打つ。`doing` の行は `by` が担当の端末でも start が打ったもので、担当が動いた証拠にしない。担当は `doing` を見て打ち直さないので、`doing` の行の数も見ない。
- Claude Code の画面は打った command を畳む（`Ran 5 shell commands` など）。画面に `kanban start` が見えないことを、打たなかった証拠にしない。
- `wl-resume` は `launch` が `resume` の起動だけを数える。`andon issue resume --fresh`（`launch` が `fresh`）の最初のメッセージは start と同じ形で印を付けないので、印が無いのは正しい。
- Claude Code の画面は長い貼り付けを `[Pasted text #1 …]` に畳み、古い行は履歴から流れる。続きから起動すると、前の起動の印つきの文も画面に映る。画面の行は今回の起動の文と結び付けられないので、ok の証拠にも壊れの証拠にもしない。Codex の担当の `wl-resume` は未確認のまま残る。壊れと言えるのは、セッションの記録の最初のメッセージに印が無いときだけ。
- 起動の直後のセッションの記録には、道具の結果と裏の作業の知らせ（`<task-notification>`）が user の行として最初のメッセージより先に入る。オーナーの印（`origin.kind` が `human`）の無い行を最初のメッセージと読まない。
- 片付いた端末は `hc get` で取れない。`wl-terminal` は片付いたことを行に出すだけで、落ちとしない。
