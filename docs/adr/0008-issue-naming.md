# ADR-0008: 台帳のカードを「課題（issue）」に改名する（語も識別子も改める・課題 ⊃ 作業〈task〉・形式 v3〈手で移す〉・依頼書の欄を「解決すべき問題」に）（仕様・設計。2026-11-07。タスク Y-197）

- 状態: **提案（2026-11-07）**。人が PR をマージした日が決定の日（[decisions.md](../decisions.md) §14 の行の日付）。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff。Y-191〜Y-195 のマージのあと）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。**同日**: 1 回目の提案（語は改め、識別子は `task` のまま）に人が答え（前提 1・8）、識別子も `issue` に改める側で書き直した。1 回目の「識別子は残す」の理由は決めたこと 2 の捨てた案に残す。**同日（3 回目）**: 人が「後方互換は気にしなくてよい」と答え（前提 9）、`andon task` の別名・App の mod の `labels.task`・`kanban migrate` の v2 → v3 をやめた（決めたこと 7）。2・3・6 の文は上書きせず、覆した所は 7 を先に読む。対応表（2）・切る案（6）・受け入れ条件は 7 に合わせて直した。
- 主題: [ADR-0006](0006-survey-fix-decide.md) 決めたこと 16 が別の ADR に回した改名。(a) 台帳のカードの日本語の語（タスク → 課題）と英語の語（task → issue）、(b) 識別子（`kanban/tasks/`・`task.yaml`・`andon task`・schema の `$id` とキー・hc のラベル）を `issue` に改める対応表、(c) 移し方（台帳の形式 v3・既存の台帳を手で移す手順・互換を作らないこと）、(d) glossary の「避ける: チケット・issue」と外部トラッカーの Issue との書き分け、(e) 依頼書の欄「やること」の名前、(f) 変える範囲の見積り（実測）と切る順。
- 語は [design.md](../design.md) §0.1。この文書の「課題」は台帳のカード、「作業」は担当が計画で課題を分けた単位（ADR-0006 前提 10・決めたこと 9）。
- 入力: 依頼書（人の言葉 2026-11-07 と、同日の人の答え 2 つ。追記）、ADR-0006 の前提 9・10 と決めたこと 9・12・14・16、design.md §0.1（語彙）・§1.7（命名の方針。「task は英語のまま。TPS に重なる語が無い」）・§3.5（台帳の CLI の名前。`deck` の別名は 1 版だけ）、glossary skill（「タスク（work item）。避ける: チケット、issue、job」）、[contract.md](../contract.md) §2・§4〜§8・§10、[ledger.md](../ledger.md)（Compatibility の「フォルダ名・キーの改名は版を上げる」、Moved without a version change、Moving from `.flightdeck/`）、chief-of-staff skill §2（依頼書の雛形）、`internal/triage/apply.go`（トリアージが書く依頼書の欄）、`package/host/*`（ラベル `task` と path `kanban/tasks` を読む mod）、andon main の実測（`grep`。前提 6）、Y-141（台帳の置き場の改名。PR #60 の差分の大きさと `kanban migrate` の形）。

## 問題（人の言葉で。言い直しの段落）

人（2026-11-07）: 「Task の主は How だが、解消したいのは Why/What。それを言葉にすれば issue」。1 回目の提案（語だけ改め、識別子は `task` のまま）への答え: 「そこまで task⇒issue は大したことじゃないと思っている。issue の中に task が内包されるものだから、理解の阻害をしないほうがいいように思う」。欄の名前は「解決すべき問題 みたいな書き方でもいいかも」。2 回目の提案（別名 1 版・mod の例外・`kanban migrate`）への答え: 「まださほど動いていないから後方互換は気にしなくていいよ」。

言い直すと: 台帳のカード 1 枚が持つべきものは「何が困っていて（Why）、何が出れば解けたか（What＝完了条件）」で、どう直すか（How）は担当が理解と計画で決める（ADR-0006 決めたこと 9・10・12）。出典（曽根。ADR-0006 前提 9）でも「課題を整理して**作業（タスク）**に分解する」で、task は課題ではなく作業の側の語。だから名前は **課題（issue）が作業（task）を内包する**形にし、識別子もその形で読めるようにする——`andon issue start` で課題を起こし、担当がその中で task（作業）を計画する。1 回目の提案が識別子を残した理由（費用・外の Issue との衝突）は、人の判断で「理解を妨げないこと」より軽いとされた。費用は払い、衝突は書き分けで持つ。互換の仕掛け（別名・2 つのラベル・移す command）は、動いている台帳が少ない今は書かず、既存の台帳は手で移す。

## 前提（番号。依頼書と現地から取ったもの）

1. **人（2026-11-07。答え 1）**: 語も識別子も `issue` に改める。台帳のカード＝issue、その中に担当が計画で切る task（作業）が入る、という包含が名前で読めるようにする。費用は大きな理由にしない。
2. **ADR-0006 前提 10・決めたこと 9**: 課題＝台帳のカード（「何が困っていて、何が出れば解けたか」と完了条件）。作業＝担当が計画で課題を分けた単位（台帳に乗らない。終了条件は担当が置く）。この 2 語は Y-191（S2）が glossary に足す。
3. **design.md §1.7 命名の方針 1・2**: 識別子の英語は、系譜の語と TPS の語が競えば系譜の語。`task` は「TPS に重なる語が無い」として英語のまま（§3.5 の表）。リーン開発・Kanban Method の系譜で台帳のカードは work item（glossary の今の括弧もこれ）、issue はトラッカー（GitHub・Jira）の系譜の語。→ この文書は §3.5 の表の `task` の行を覆す（`issue` は系譜の語ではないが、人が「包含が読める」ことを取った）。
4. **glossary skill**: 見出しの括弧は識別子に使う英語。今の見出しは「タスク（work item）」だが識別子は `task` で、括弧と識別子が食い違っている（この文書で直す）。「避ける: チケット、issue、job」。
5. **ADR-0006 決めたこと 14**: 判断の標準は 機械が確かめる形 → 文 → 人のレビュー の順に置く。名前が変えるのは読む人と agent の判断の向きで、機械の判定は変わらない。→ 改名の効き目は文にあり、識別子の改名は「名前で包含が読める」ための費用（前提 1）。
6. **現地の実測（andon main `96f40f6`。`grep -c 'タスク'`・`grep -ci 'task'`。横展開＝task／タスクが人の目に出る所を数えた）**:

   | 場所 | 「タスク」（日本語の文） | `task`（識別子・英語） | 人の目に出るか |
   |---|---|---|---|
   | 識別子そのもの | — | `kanban/tasks/<id>/`・`task.yaml`・`andon task <8 つの sub command>`（`start`・`finish`・`ready`・`report`・`stop-check`・`ask`・`review-request`・`show`。`candidates` は ADR-0006 で足す）・`kanban show --json` の `.task`・`.tasks`・`schema/task-keys.schema.json`（`$id` `…/task-keys/1`）・5 つの schema のキーと値 `task`／`reviewer_task`／`triage_max_tasks`・hc のラベル `task=<id>`・`andon triage apply --max-tasks`・dispatcher の控えのキー `task` | 出る（command を打つ・`--help`・ボードがラベルと path を読む） |
   | skill（7 本） | 109 | 105 | 出る（agent が読む文） |
   | プロファイルの template（最初のメッセージ。7 本） | 8 | 0 | 出る |
   | hooks（`package/hooks/stop`） | — | 3（`andon task stop-check`・ラベル `task=`） | 出ない（フック） |
   | docs（生きている文書: design・contract・dispatcher・operate・README の日本語） | 144 | 170 | 出る |
   | ledger.md（形式の正。英語） | 0 | 101 | 出る（形式の読み手） |
   | decisions.md・ADR 0001〜0007 | 231 | 142 | 出るが**当時の言葉のまま**（decisions.md §16 の前例） |
   | Go（`cmd/kanban` 7 ファイル・`internal` 25 ファイル。テスト除く。package `internal/task` を含む） | 日本語の文字列 39 種（command の出力・最初のメッセージ・通知の見出し） | 文字列 185 種・約 820 行 | 日本語の文は出る。識別子は出る（`--help`・JSON のキー） |
   | tests（13 ファイル・約 360 行） | — | — | 出ない（検査） |
   | ボードの mod（`package/host`）: kanban-board（path `kanban/tasks` 6 か所）・context-header／inbox／andon（ラベル `labels.task`） | 0 | 268（ほぼ JS の変数名） | path とラベルは形式。変数名は出ない |

   前例: Y-141（台帳の置き場 `.flightdeck/` → `kanban/`・`coordinator` → `chief_of_staff`・id を 4 桁）は 66 ファイル・+1,740／−726 行で、`kanban migrate` を 1 本足した（`doing`・`review` があれば止まる・dispatcher の控えも移す・`project.json` の案を書く）。この改名はそれより広い（上の表の全部）。見積りは Y-141 の約 2 倍。
7. **外部トラッカーとの線（chief-of-staff skill 規約 7・design.md §1.3 の 3）**: 外部のトラッカー（Backlog・GitHub Issues 等）で回る案件はそちらが正で、台帳のカードは `links` にキーを持つ写し。ADR-0006 決めたこと 6 の例 3 は「再現手順は issue #12」と GitHub の Issue を指す。design.md §1.9 は「外からの issue は無い」「信号の入口（CI・issue・監視）は最初の版の後」。→ 台帳の課題と外の Issue が同じ語になるので、書き分けを決めたこと 5 で持つ。
8. **人（2026-11-07。答え 2）**: 依頼書の最初の欄の名前は「解決すべき問題」（「問題」1 語より、課題として何を解くかが読める）。ほかの欄名・雛形・`andon triage apply`・tests の揃え先もこの語。
9. **人（2026-11-07。答え 3）**: 「まださほど動いていないから後方互換は気にしなくていい」。→ `andon task` の別名の期間・App の mod の `labels.task` の例外・読む側の互換・`kanban migrate` の v2 → v3 は作らない。既存の台帳は operate.md の短い手順（フォルダとキーの改名）で人が手で移す。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-07 語は 課題（issue）と 作業（task）。課題が作業を内包する。「タスク」は使わない（課題の意味でも作業の意味でも）。**

   | 語 | 識別子 | 何か | どこにあるか | 誰が置くか | 条件 |
   |---|---|---|---|---|---|
   | **課題**（issue） | `issue`（`kanban/issues/<id>/`・`issue.yaml`・`andon issue`） | 台帳のカード 1 枚。何が困っていて、何が出れば解けたか（解決すべき問題）と完了条件を持つ。どう直すかは持たない | 台帳 | Chief of Staff（依頼書） | **完了条件**（課題が解けたと言える状態。担当は書き換えない） |
   | **作業**（task） | 識別子は無い（台帳に乗らない） | 担当が計画で課題を分けた単位。1 つの課題に 1 つ以上 | 担当の報告の `## 計画`（1 行ずつ） | 担当 | **終了条件**（その作業をいつ終えるか。担当が置く） |

   ```
   課題（issue）A-0012 「ログインが落ちる」      ← 台帳のカード。依頼書: 解決すべき問題・完了条件
   ├─ 作業（task）1: 再現テストを足す。終了条件: 落ちるのを確かめた
   ├─ 作業（task）2: 原因の module を直す。終了条件: テスト green
   └─ 作業（task）3: 横展開で同じ呼び方を探す。終了条件: grep を読み切った
   ```

   - glossary（Y-191 が足す「課題」「作業」の項をこの形にし、「タスク（work item）」の項は消す）:
     - **課題**（issue）: 上の表。「かんばん。プロジェクトの外では `<slug>/<id>`。人へ書くときは「番号（題）」」は今の「タスク」の項から引き継ぐ。_避ける_: タスク（古い文書では課題の意味。作業と混ざる）、チケット、job、Issue（外部トラッカーのもの。指すときは「<トラッカー名> の Issue」。5）。
     - **作業**（task）: 上の表。_避ける_: タスク、サブタスク、ステップ。
     - 包含の図（上）を glossary の「仕事」の節の先頭に置く。
   - 英語の文（README・ledger.md・schema の description）は **issue** と **task**（作業）。README の英語「take tasks from a ledger」は「take issues from a ledger」。
   - 仮設の語の表（glossary の末尾）に「タスク（台帳のカード） → 課題（`issue`）」「task（識別子） → `issue`」を足す。
   - ADR 0001〜0007・decisions.md の過去の行は当時の言葉のまま（「タスク」「task」は課題の意味で読む）。

   理由: 前提 1・2。出典の語（前提 9 の「作業（タスク）」）では task＝作業で、人の答えもその包含。カードを task と呼び続けると、ADR-0006 の背骨（課題の完了条件・作業の終了条件）が skill に入ったとき「task」が課題と作業の両方を指して読めなくなる。語と識別子が同じ向きなら、glossary の 1 行で「識別子は `task` だが意味は課題」と持たなくてよい。
   捨てた案: 語だけ改めて識別子は `task` のまま（1 回目の提案。決めたこと 2 の捨てた案）／カードを「作業」と呼ぶ（How の側の語。人の指摘の逆）／英語の文は task のまま issue を使わない（識別子と文が食い違う）／作業も台帳に乗せる（ADR-0006 決めたこと 9 の捨てた案のまま。作業は報告の `## 計画` で足りる）。

2. **2026-11-07 識別子を `issue` に改める。対応表はこれ。読む側に古い名前の互換は作らず、`kanban migrate` が 1 回で移す（3）。**

   | 場所 | 今 | 後 | 備考 |
   |---|---|---|---|
   | 台帳の置き場 | `kanban/tasks/<id>/` | `kanban/issues/<id>/` | フォルダ名の変更＝形式の版を上げる（ledger.md Compatibility）。`kanban/`・`routines/`・`library/`・`log.jsonl` は変えない |
   | 課題の状態のファイル | `tasks/<id>/task.yaml` | `issues/<id>/issue.yaml` | 中のキー（`id`・`title`・`status`・`repo`・`profile`・`parent`・予約キー）は変えない |
   | 形式の版 | `config.yaml` `version: 2` | `version: 3` | 版 2 の台帳は「format 2; move it by hand (docs/operate.md)」の 1 行で止まる（読まない。7） |
   | 台帳の CLI | `kanban add`・`show`・`start`・… | 同じ | sub command の名前と引数は変えない。英語の文の task → issue（「ok (3 issues)」）。`kanban show --json` の `.task` → `.issue`、`list --json` の `.tasks` → `.issues` |
   | 台帳の `log.jsonl` | `{"id": …, "change": {"add": "<題>"}}` | 同じ | 行の形にキー `task` は無い。前の行はそのまま |
   | ルーチン | 「fires: adds a task」 | adds an issue | ledger.md の文だけ |
   | 担当の command | `andon task <sub>`（8 つ＋`candidates`） | `andon issue <sub>` | 別名は無い（7）。`--help` と文は `issue` |
   | トリアージ | `andon triage apply --max-tasks` | `--max-issues` | |
   | 予約キーの schema | `schema/task-keys.schema.json`（`$id` `…/task-keys/1`） | `schema/issue-keys.schema.json`（`$id` `…/issue-keys/1`） | ファイルの改名。中のキーは同じ（`deliverable` を含む） |
   | 通知の schema | `task`・`reviewer_task` | `issue`・`reviewer_issue` | `$id` を `notification/2` に |
   | 報告の schema | `next[].to` の値 `task` | `issue` | `$id` を `report/2` に |
   | 問いの schema | `from` の値 `task` | `issue` | `$id` を `question/2` に |
   | ゲートの記録の schema | `review.task` | `review.issue` | `$id` を `gate/2` に |
   | プロジェクトの schema | `chief_of_staff.triage_max_tasks` | `triage_max_issues` | `$id` を `project/2` に。`project.json` は repo の中の人のファイルで、人が手で改めて commit する（7 の手順） |
   | hc のラベル | `task=<id>` | `issue=<id>` | 書くのは `andon issue start`・dispatcher（レビュアー）・`andon chief-engineer`。読むのは dispatcher・boot・hooks・App の mod。`project=`・`role=` は変えない |
   | Space・tab の id と名前 | `andon-<slug>-<id>`・`<id> <題>` | 同じ | id に task は無い（ADR-0003） |
   | worktree・branch の名前 | `<slug>-<id>`・`<prefix>/<id>-<slug>` | 同じ | |
   | dispatcher の控え（`~/.local/state/andon/dispatcher/`） | キー `task` | `issue` | 移さず消す（7 の手順。`doing`・`review` が無ければ作り直される） |
   | フック | `package/hooks/stop`: `andon task stop-check`・ラベル `task=` | `andon issue stop-check`・`issue=` | |
   | Go | package `internal/task`・型 `Task`・JSON タグ `task` | `internal/issue`・`Issue`・`issue` | 人の目に出ないが差分に入る |
   | App の mod（`package/host`） | kanban-board の path `kanban/tasks`、context-header／inbox／andon のラベル `labels.task` | `kanban/issues`、`labels.issue`（`labels.task` は読まない。7） | 人の App に置いた写しは `host-install` が上書きしないので、人が置き直す（operate.md に 1 行） |
   | 仮設（yard の `yard-task`・`deck`・`.flightdeck/`） | — | 触らない | 仮設は止まるまで当時の言葉のまま |

   理由: 前提 1。包含を名前で読むには、人と agent が打つ `andon issue start`・読む `issues/<id>/issue.yaml`・ボードが読むラベルまで同じ語でなければ、「`task` は課題」の注が残る。読む側に互換を作らないのは Y-141 と同じ（2 つの名前を読む code は、どちらが正か分からなくなる。decisions.md §16）。App の mod の `labels.task` だけは例外: App は 1 つで、仮設の端末（`task=`）も同じ App に出ているので、仮設が止まるまでは名前が付かなくなる。
   捨てた案（1 回目の提案。識別子は `task` のまま）: 効き目は文にあり（前提 5）、`issue` は外のものの語（前提 7）、費用は Y-141 の約 2 倍で形式の版が上がる——人が「費用は大きな理由にしない」「理解を妨げないほうが大事」と答えた（前提 1）。／`card`・`item`（かんばんの語。衝突しない）: 包含（課題 ⊃ 作業）が読めない。／schema の `$id` を上げず、キーだけ改める: ledger.md と contract.md §10 の決まり（キーの改名は版を上げる）に反する。／App の mod に `labels.task` を残さない: 仮設の端末の名前が消える（人が毎日見る面）。

3. **2026-11-07 移し方。台帳の形式を v3 にし、`kanban migrate` が v2 の台帳を 1 回で移す。別名は `andon task` だけ、1 版。**

   - **7 で覆した**: `kanban migrate` の v2 → v3・`andon task` の別名・ラベルの例外は作らない。この項で生きているのは「読む側は v3 だけ読み、v2 は 1 行で止まる」「順（Y-191〜Y-195 のあと・1 つのパッケージの版）」だけで、移す手順は 7。
   - **`kanban migrate`（v2 → v3）**。止まって何もしないのは: `config.yaml` の `version` が 2 でない／`issues/` がもうある／`task.yaml` が読めない・課題のフォルダが symbolic link／dispatcher の控えが読めない／**`doing`・`review` の課題がある**（端末がラベル `task=` を持って動いていて書き換えられない。worktree・branch・Space の id は変わらないので理由はラベルだけ。先に `finish` する）。
     するのは順に: (1) `tasks/` → `issues/`、各 `task.yaml` → `issue.yaml`（中身は変えない）、(2) 各課題の `library/report.json` の `next[].to` と `questions.yaml` の `from` の `task` → `issue`（`done` のものも。読む互換を作らないため。`report.md`・`brief.md`・`log.jsonl` は触らない）、(3) dispatcher の控えのキー `task` → `issue`、(4) `<root>/project.json` の `triage_max_tasks` → `triage_max_issues`（人のファイル。書き換えて「commit して」と出す。無ければ書かない）、(5) `config.yaml` を `version: 3`、(6) `log.jsonl` に `{"migrate": {"from": "tasks", "version": 3}}`。
     `.flightdeck/` → `kanban/` の移行（v2 のまま）は残す（今の `migrate`）。両方要る台帳は 2 回打つ（v1 の読み込みは今までどおり断る）。
   - **読む側**: `kanban`・`andon`・dispatcher・ゲート・App の mod は v3 だけ読む。v2 の台帳に当たったら「format 2; run `kanban migrate`」の 1 行で止まる（ledger.md Compatibility の「知らない版は止まる」）。
   - **別名の期間**: `andon task <sub>` は `andon issue <sub>` と同じことをする（`--help` には出さない）。残すのは改名を入れたパッケージの版だけで、次の版で消す（design.md §3.5 の `deck` の別名と同じ）。`kanban` に別名は無い（sub command の名前が変わらない）。ラベル `task=` は書かない（App の mod が読むのは仮設のため。2）。
   - **順**: 改名は 1 つのパッケージの版にまとめて入れる（セルは版上げ 1 回で移る）。Y-191〜Y-195（ADR-0006 の実装。`andon task candidates`・`task-keys` の `deliverable` を書く）のマージのあとに始める。先にすると 3 本のタスクが rebase になる。
   - **セルの手順**（operate.md に写す）: パッケージの版を上げる → `kanban migrate`（プロジェクトごと） → `project.json` を commit → App の mod を置き直す → `andon boot`。

   理由: ledger.md の決まり（フォルダ名の変更は版を上げて告げる。知らない版は止まる）。Y-141 で「互換は作らず migrate が 1 回で移す」「`doing`・`review` があれば止まる」の形が動いたので同じ形。`report.json`・`questions.yaml` の値を書き換えるのは、トリアージと振り返りが `done` の課題の報告も読むため（残すと読む側に `task` の互換が要る）。
   捨てた案: 読む側が v2 も読む（2 つの形を読む code）／`task=` と `issue=` の両方のラベルを書く（二重の名前が残る）／`andon task` の別名を残さない（skill・人の手が一斉に変わる日に打ち間違いで止まる。1 版だけ）／`project.json` は人に手で直してもらう（migrate が全部の案件を 1 回で揃えるほうが確か。commit は人）。

4. **2026-11-07 依頼書の欄「やること」を「解決すべき問題」に改める。書くことは ADR-0006 決めたこと 12 のまま（何が困っていて、何が出れば解けたか。手順ではない）。**

   - 雛形（chief-of-staff skill §2）の 1 行目を `解決すべき問題: <何が困っていて、何が出れば解けたか（1〜3 行）。手順ではない。場所の手がかりは書いてよい>` に。欄の並びは 解決すべき問題・なぜ・完了条件・横展開・守ること・Settings。
   - 読む側（worker skill §2「依頼書の『やること』『完了条件』『守ること』だけをする」）は「解決すべき問題」を読み、古い依頼書の「やること」も同じ欄として読む（1 行の括弧）。既存の依頼書は書き換えない。
   - `andon triage apply` が書く依頼書の欄（`internal/triage/apply.go` の `やること:`）と triage skill の説明、`tests/andon-triage.sh`・`tests/package.sh` の雛形の検査を揃える。
   - ADR-0006 決めたこと 12 の表の「やること」の行は、この文書で「解決すべき問題」と読み替える（ADR-0006 は書き換えない）。

   理由: 前提 8。「やること」は字のとおり How（する事）の欄名で、決めたこと 12 が「手順ではない」と但し書きで押さえているのは、名前が中身と逆だから。「解決すべき問題」は課題（issue）の定義そのもので、Chief Engineer の文書の最初の節「問題（人の言葉で）」とも向きが同じ。
   捨てた案: 「問題」（1 回目の提案。人が「解決すべき問題」のほうが何を解くかが読めると答えた）／「困りごと」（口語で読みやすいが、Chief Engineer の文書の「問題」と 2 語になる）／「課題:」（カードの名と欄の名が同じで、課題の中の課題になる）／「やること」のまま但し書きで持つ（名前が読み手を How へ引く）／欄を 5W1H に作り直す（ADR-0006 決めたこと 12 の捨てた案のまま）。

5. **2026-11-07 外部トラッカーとの書き分け。台帳のカードは「課題」、トラッカーのものは「<トラッカー名> の Issue」（GitHub の Issue #12・Backlog の課題 PROJ-12）。`links` は課題からトラッカーのものへの参照で、中身は写さない（規約 7 のまま）。**

   - skill と docs の文で外のものを指すときはトラッカー名を添える。ADR-0006 決めたこと 6 の例 3 は Y-195 が skill に写すとき「GitHub の Issue #12」と書く（ADR-0006 は書き換えない）。
   - command と schema で外のものを持つときは `links`（今の予約キー）だけ。`issue` の語を外のものの識別子に使わない（信号の入口（design.md §1.9）が入っても、外の Issue は `signal` の中身で、台帳の課題は `issue`）。
   - Backlog の「課題」（Backlog の日本語 UI の語）と台帳の課題が同じ文に出るときは「Backlog の課題 <キー>」と書く。

   理由: 前提 7。語を 1 つにした代わりに、外のものには必ず持ち主の名前を添える。識別子では `issue` が台帳のもの、外のものは `links` と `signal` で、名前が重ならない。
   捨てた案: 台帳のカードを別の語にして衝突を避ける（1 回目の提案。前提 1）／外のものを「外部 Issue」と呼ぶ（持ち主の名前のほうが読める）。

6. **2026-11-07 変える範囲と順（実装のタスクが直す。この文書の PR では変えない）。Y-191〜Y-195 のマージのあと、形式 → command → 文・mod の順。**

   | 場所 | 変える所 | 変えない所 |
   |---|---|---|
   | `cmd/kanban`・`internal/ledger`・tests/kanban | 形式 v3（`issues/`・`issue.yaml`・`version: 3`。v2 は 1 行で止まる。`kanban migrate` は足さない。7）、JSON の `.issue`／`.issues`、英語の文（2） | sub command の名前・引数・`log.jsonl` の形・id の形 |
   | ledger.md | 版 3 の節（Changes from version 2 の表。移すのは手で、operate.md を指す）、Layout、文の task → issue | 版 1・2 の節（当時の言葉） |
   | `internal/task` → `internal/issue`・`cmd/andon`・`internal/dispatcher`・`internal/gate`・`internal/triage`・`internal/spawn`・`internal/boot`・`internal/chiefengineer` | `andon issue <sub>`（別名なし）、ラベル `issue=`、`--max-issues`、JSON のキー、Go の日本語の文字列（39 種。「タスク」→「課題」。例「課題 `<slug>/<id>`。依頼書は …」）、`andon triage apply` の欄名「解決すべき問題:」 | 変数名の英語以外の文 |
   | `schema/` | `issue-keys.schema.json`（改名）、notification／report／question／gate／project の `$id` を 2 に上げてキーと値を改める | `findings`・`candidates`（`task` を持たない） |
   | contract.md | §1 の木、§2 の見出しと例、§4〜§7 のキー、§8 のラベル、§10 に版の行 | §0・§3 |
   | `package/hooks/stop` | `andon issue stop-check`・ラベル `issue=` | |
   | glossary skill | 課題（issue）・作業（task）の項と包含の図、「タスク（work item）」の項を消す、仮設の語の表（1） | |
   | skill 6 本と template 7 本 | 文の「タスク」→「課題」（作業の意味なら「作業」）、`andon task` → `andon issue`、依頼書の雛形の「やること」→「解決すべき問題」（4）、worker skill の読み方の 1 行、issue へのトラッカー名（5） | skill の名前 `worker` |
   | docs（design・dispatcher・operate・README） | 文の「タスク」→「課題」、design.md §0.1 の表の行（「課題 \| issue \| 台帳のカード 1 枚…」）と「作業 \| task」の行、§1.7 のかんばんの当て（「タスク」→「課題」）、§3.5 の表の `task` の行に「`issue` に改めた。ADR-0008」、operate.md に手で移す手順（7）、README の英語と日本語 | decisions.md の過去の行・ADR 0001〜0007・README の由来の行 |
   | `package/host` | kanban-board の path、context-header／inbox／andon のラベル `issue`、`tests/host/*.mjs` | レイアウト・Space の id |
   | tests | `tests/andon-task.sh` → `tests/andon-issue.sh` ほか、変えた文字列と形の検査 | `tests/public.sh`（語の禁止リストは repo の外） |
   | 台帳の中身（既存の依頼書・報告の Markdown・`log.jsonl`） | 変えない（手で移すのは 7 の手順の 3〜7 だけ） | 全部 |

   順: Y-191・Y-194・Y-195 のマージ → S2（形式 v3）→ S3（`andon` の command・schema・contract.md・hooks）→ S4（glossary・skill・template・依頼書の欄・Go の日本語の文字列）・S5（docs）・S6（App の mod）は独立 → S7（セルで実測）。全部を 1 つのパッケージの版に入れる。

   理由: 形式が先でないと command のテストが書けない。同じ skill を Y-194・Y-195 と同時に触ると衝突する（chief-of-staff skill §3 の 1「ぶつからないか」）。語の改めは Y-170（建屋 → セル）と同じ型の仕事。
   捨てた案: 全部を 1 本（Y-141 の 2 倍の差分。レビューできない）／文書を触るたびに直す（glossary の「ここにある語だけ」が長く破られ、2 つの名前が同居する）／形式と command を別の版で出す（セルが 2 回移る）。

7. **2026-11-07 後方互換は作らない（人の答え。2 の表・3・6 の一部を覆す）。`andon task` の別名・App の mod の `labels.task`・読む側の互換・`kanban migrate` の v2 → v3 は作らない。既存の台帳は人が手で移す。手順は operate.md に写す。**

   - **覆すもの**: 2 の表の「担当の command」「形式の版」「プロジェクトの schema」「dispatcher の控え」「App の mod」の行（表は直した）、3 の `kanban migrate`・別名・ラベルの例外、6 の S2・S5・S6 の中身（表は直した）、やらないことの「例外」。
   - **残るもの**: 形式の版は 3 に上げる（ledger.md の決まり。フォルダ名の変更は版を上げる）。読む側（`kanban`・`andon`・dispatcher・ゲート・App の mod）は v3 だけ読み、v2 の台帳には「format 2; move it by hand (docs/operate.md)」の 1 行で止まる（v1 と同じ扱い。ledger.md「There is no migration」の形）。`kanban migrate` は今の `.flightdeck/` → `kanban/`（v2 のまま）だけを持ち、v3 への行は足さない。
   - **手で移す手順**（operate.md の「前の置き場から移す」の次に節を足す。台帳ごと。9 行）:
     1. `doing`・`review` の課題を片付ける（マージして `andon task finish`、または `todo` に戻す）。ラベル `task=` を持つ担当・レビュアー・Chief Engineer の端末が居なくなる。確かめ方: `grep -lE '^status: (doing|review)' projects/<slug>/kanban/tasks/*/task.yaml` が何も出ない。
     2. パッケージの版を上げる（`andon`・`kanban` が v3 を読む）。
     3. `mv kanban/tasks kanban/issues`、各 `issues/<id>/task.yaml` を `issue.yaml` に。
     4. `kanban/config.yaml` の `version: 2` を `version: 3` に。
     5. `issues/*/library/report.json` の `"to": "task"` を `"to": "issue"` に、`issues/*/questions.yaml` の `from: task` を `from: issue` に（`sed`。無ければ飛ばす）。
     6. dispatcher の控え `~/.local/state/andon/dispatcher/<slug>.json`（`XDG_STATE_HOME` があればその下）を消す。控えが守るのはレビュアーの二重起こしとゲートの重複で、動いている課題が無ければ要らず、次の見回りで作り直される。
     7. `projects/<slug>/project.json` の `triage_max_tasks` を `triage_max_issues` に直して commit（無ければ何もしない）。
     8. 人の App の mod を置き直す（`host-install` は今あるものを上書きしないので、Andon の mod を消してから）。
     9. `andon boot`。`kanban lint` が通れば済み。
     `log.jsonl` の行・`brief.md`・`report.md` は触らない（過去の行は当時の言葉のまま）。
   - 動いている課題がある台帳で手順 3 以降をすると、端末のラベル `task=` と台帳が食い違い、dispatcher が担当を見失う。だから 1 を先に、`kanban lint` を最後に。

   理由: 前提 9。動いている台帳は少なく（セルフホストの前）、互換の仕掛け（別名・2 つのラベル・migrate とそのテスト）は書く量とテストを増やすのに使うのは数回。Y-141 の migrate が持っていた守り（`doing`・`review` で止まる）は手順の 1 に置き、確かめは `kanban lint` に任せる。版を 3 に上げるのは残す: 読む側が v2 の台帳を「壊れた」ではなく「古い。手順はここ」と言えるため。
   捨てた案: 2・3 の形（別名 1 版・`labels.task` の例外・`kanban migrate` の v2 → v3。人が「後方互換は気にしなくてよい」と答えた）／形式の版を上げない（フォルダ名の変更は版を上げる決まり。v2 を読む code が無いのに版が同じだと、止まる理由が言えない）／`kanban lint --fix` で移す（migrate を名前を変えて作るのと同じ）／dispatcher の控えのキーを手で書き換える（消すほうが短く、守りは要らない）。

## やらないこと（範囲の外。理由）

- **読む側の互換と移す command**（v2 の台帳を読む・`task=` のラベル・`next[].to: task`・`andon task` の別名・App の mod の `labels.task`・`kanban migrate` の v2 → v3）: 決めたこと 7。既存の台帳は手で移す（仮設の端末に名前が付かなくなるのは仮設が止まるまでの間だけ）。
- **台帳の CLI の名前 `kanban`・sub command の名前・id の形・Space／tab／worktree／branch の名前**: 変えない（`task` を含まない）。
- **ADR 0001〜0007・decisions.md の過去の行・`log.jsonl`・既存の依頼書と報告の Markdown の書き換え**: 当時の言葉のまま（decisions.md §16 の前例）。
- **「作業」を台帳に乗せる**（作業の一覧の形・`library/plan.json`・作業の id）: ADR-0006 決めたこと 9 の捨てた案のまま。作業は報告の `## 計画`。
- **仮設の工場（yard・`yard-task`・`deck`・`.flightdeck/`）の語と識別子**: 仮設は止まるまで当時の言葉のまま。
- **外の Issue を台帳に取り込む口**（信号の入口）: design.md §1.9 のまま最初の版の後。この文書は識別子 `issue` を台帳に取ることだけ決めた。
- **ボードの mod の変数名**: 人の目に出ない。path とラベルだけ変える。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

1. **形式（`tests/kanban.sh`・`tests/kanban/test-deck.mjs`・`go test ./cmd/kanban/... ./internal/ledger/...`）**: `kanban init` が `version: 3` と `issues/` を作る。`kanban add` が `issues/<id>/issue.yaml` を書く。`kanban show --json` が `.issue`、`list --json` が `.issues`。`version: 2` の台帳は全部の sub command（`migrate` を含む）が「format 2; move it by hand (docs/operate.md)」の 1 行で止まる（終了コード 1。何も書かない）。v2 の台帳の写しに operate.md の手順（7 の 3〜7）を当てると `kanban lint` が通り、`list`・`show` が読める。
2. **command（`tests/andon-issue.sh`・`tests/andon.sh`・`go test ./...`）**: `andon issue start|finish|ready|report|stop-check|ask|review-request|show|candidates` が今の `andon task` と同じ振る舞い。`andon task <sub>` は知らない command として止まり、`andon --help`・`andon issue --help` に `task` が出ない。起こした端末のラベルが `issue=<id>`（`task=` は無い）。担当の最初のメッセージが「課題 `<slug>/<id>`。依頼書は …」。`cmd/`・`internal/` の日本語の文字列に「タスク」が無い（`grep -rn 'タスク' --include='*.go' cmd internal` が空）。`andon triage apply --max-issues` が効き、書く依頼書の最初の欄が `解決すべき問題:`。
3. **schema（`tests/schema.sh`）**: `issue-keys.schema.json` があり `task-keys.schema.json` が無い。notification／report／question／gate／project の `$id` が `/2`。contract.md の例が全部通る。`task`・`reviewer_task`・`triage_max_tasks`・`to: task`・`from: task` を持つ例が落ちる。
4. **dispatcher とゲート（`tests/andon-dispatcher.sh`・`tests/andon-gate.sh`）**: 通知が `issue`・`reviewer_issue` で届く。ゲートの記録が `review.issue`。dispatcher の控えのキーが `issue`。レビュアーの端末のラベルが `issue=<親>-r<回>`。
5. **パッケージ（`tests/package.sh`）**: glossary に「課題（issue）」「作業（task）」の項と包含の図があり、「タスク（work item）」の項が無く、「タスク」は「避ける」の行と仮設の語の表にしか無い。skill 6 本と template 7 本に「タスク」と `andon task` が無い。chief-of-staff skill §2 の雛形の最初の欄が `解決すべき問題:` で `やること:` が無い。worker skill が「解決すべき問題」と古い「やること」の両方を読むと書いてある。`package/hooks/stop` が `andon issue stop-check` を打つ。
6. **App の mod（`tests/host/mod.mjs`・`tests/host/screen.mjs`）**: kanban-board が `kanban/issues` を読み、`kanban/tasks` は読まない。context-header／inbox／andon が `labels.issue` で名付け、`labels.task` は読まない。
7. **文書**: design.md §0.1 の表に「課題 \| issue」「作業 \| task」、§3.5 の表の `task` の行に ADR-0008 への参照。ledger.md が `version` 3 と `issues/` を書き、版 2 からの変更の表があり、移すのは手で（operate.md）と書いてある。operate.md に 7 の手順がある。docs の日本語の文に「タスク」が無い（decisions.md・ADR を除く。`grep -c 'タスク' docs/design.md docs/contract.md docs/dispatcher.md docs/operate.md README.md` が全部 0）。`tests/public.sh` が通る。
8. **セルで実測（P4 以降の決まり。報告に書く）**: パッケージの版を上げたセルで、`doing`・`review` の無い台帳を operate.md の手順（7）で移すと `kanban lint` が通り、dispatcher が控え無しから見回りを続け、App のボードが課題を出す（mod を置き直したあと）。「解決すべき問題:」で書いた依頼書を 1 本流し、担当の報告の `## Report` の冒頭の言い直しがその欄と完了条件から書かれ、依頼書に手順が無くても担当が計画して PR が出る（人の turn がゼロ）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 | プロファイル |
|---|---|---|---|---|
| S1 | この文書の PR（decisions.md §14 に 1 行） | `tests/run`・`tests/public.sh` ok（CI は予算で止まっているので手元で） | — | — |
| S2 | 台帳の形式 v3（v2 は 1 行で止まる。`kanban migrate` は足さない）、`cmd/kanban` の JSON と文、ledger.md の版 3 の節、`tests/kanban` | 受け入れ条件 1 | S1、Y-191・Y-194・Y-195 のマージ | `claude` |
| S3 | `andon issue`（`internal/task` → `internal/issue`・別名なし・ラベル `issue=`・`--max-issues`・Go の日本語の文字列）、schema の改名と `$id`、contract.md §1・§2・§4〜§8・§10、`package/hooks/stop`、dispatcher・ゲートのキー、tests | 受け入れ条件 2・3・4 | S2 | `claude`（Review: on。約 500 行超） |
| S4 | glossary（課題・作業・包含の図・仮設の語の表）、skill 6 本と template 7 本の語と `andon issue`、依頼書の雛形の「解決すべき問題」と worker skill の読み方、`andon triage apply` の欄名 | 受け入れ条件 5 と 2 の欄名 | S3 | `sonnet` |
| S5 | docs（design §0.1・§1.7・§3.5・dispatcher・operate〈手で移す手順。7〉・README） | 受け入れ条件 7 | S3。S4 と独立 | `sonnet` |
| S6 | App の mod（kanban-board の path、ラベル `issue`）、`tests/host` | 受け入れ条件 6 | S2。S3〜S5 と独立 | `claude` |
| S7 | セルで実測（受け入れ条件 8）。食い違えば実測が正で、この文書の「決めたこと」に下から足す | 報告 | S2〜S6 を 1 つのパッケージの版に入れたあと。Y-196 と一緒でよい | `claude` |

## 開いている問い（人に聞く。1 問ずつ。推奨案つき）

無し。1 回目の提案の 2 問（識別子を残すか・欄の名前）は人が同日に答え、前提 1・8 に写した。3 回目の答え（後方互換は気にしない）は前提 9 と決めたこと 7。実装で決める細部（止まる文言・手順の sed の形）は推奨どおり進め、担当が報告に書く。

## 出典（読んだもの・捨てた案の出所）

- 依頼書（人の言葉 2026-11-07。追記の答え 1・2・3）、[ADR-0006](0006-survey-fix-decide.md) 前提 9・10、決めたこと 6・9・12・14・16、開いている問い 2。
- [design.md](../design.md) §0.1（語彙）・§1.7（系譜と命名の方針・かんばんの当て）・§1.9（外からの issue は無い。信号の入口）・§3.5（CLI の名前。`deck` の別名 1 版。`task` は英語のまま、の行）。
- glossary skill（`package/skills/glossary/SKILL.md`）、chief-of-staff skill §2・§3・§9 規約 7、worker skill §2、triage skill、`internal/triage/apply.go`、`package/hooks/stop`、`package/host/{kanban-board,context-header,inbox,andon}/main.js`。
- [contract.md](../contract.md) §2（`task.yaml` の予約キー）・§4〜§7（報告・問い・通知・ゲートのキー）・§8（hc のラベル）・§10（互換と版）、`schema/*.json`、[ledger.md](../ledger.md)（Compatibility・Layout・Moved without a version change・Moving from `.flightdeck/`・Changes from version 1）。
- [decisions.md](../decisions.md) §16（Y-141 の行。「前の名前を読む互換は作らず、`kanban migrate` が 1 回で移す」「前の行は当時の言葉のまま」）、PR #60 の差分（66 ファイル）、`cmd/kanban/migrate.go`（止まる条件・dispatcher の控え・`project.json` の案）。
- 実測: andon main `96f40f6` の `grep`（前提 6）。
