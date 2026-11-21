# ADR-0008: kanban のカードを「課題（issue）」に改名する（語も識別子も改める・課題 ⊃ 作業〈task〉・形式 v3〈手で移す〉・依頼書の欄を「解決すべき問題」に）（仕様・設計。2026-11-07。タスク Y-197）

- 状態: **決定（2026-11-07）**。オーナーが PR をマージした日が決定の日（[decisions.md](../decisions.md) §14 の行の日付）。実装は「依頼書に切る案」のとおり別タスク（切るのは Chief of Staff。Y-191〜Y-195 のマージのあと）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。**同日**: 1 回目の提案（語は改め、識別子は `task` のまま）にオーナーが答え（前提 1・8）、識別子も `issue` に改める側で書き直した。1 回目の「識別子は残す」の理由は決めたこと 2 の捨てた案に残す。**同日（3 回目）**: オーナーが「後方互換は気にしなくてよい」と答え（前提 9）、`andon task` の別名・App の mod の `labels.task`・`kanban migrate` の v2 → v3 をやめた（決めたこと 7）。2・3・6 の文は上書きせず、覆した所は 7 を先に読む。対応表（2）・切る案（6）・受け入れ条件は 7 に合わせて直した。
- 整理（2026-11-20。[ADR-0033](0033-adr-inventory-and-slimming.md) 決めたこと 1・2）: 「決めたこと」のうち、実装が済んだ決定の細部（表・図・手順）を、今の正の場所への参照に替えた。決定の文・理由・捨てた案は残した。整理前の全文は commit 7de0ec841bc25a00cf53cad5efbaf40899cce4dc のバージョンにある。
- 主題: [ADR-0006](0006-survey-fix-decide.md) 決めたこと 16 が別の ADR に回した改名。(a) kanban のカードの日本語の語（タスク → 課題）と英語の語（task → issue）、(b) 識別子（`kanban/tasks/`・`task.yaml`・`andon task`・schema の `$id` とキー・hc のラベル）を `issue` に改める対応表、(c) 移し方（kanban の形式 v3・既存の kanban を手で移す手順・互換を作らないこと）、(d) glossary の「避ける: チケット・issue」と外部トラッカーの Issue との書き分け、(e) 依頼書の欄「やること」の名前、(f) 変える範囲の見積り（実測）と切る順。
- 語は [design.md](../design.md) §0.1。この文書の「課題」は kanban のカード、「作業」は担当が計画で課題を分けた単位（ADR-0006 前提 10・決めたこと 9）。
- 入力: 依頼書（オーナーの言葉 2026-11-07 と、同日のオーナーの答え 2 つ。追記）、ADR-0006 の前提 9・10 と決めたこと 9・12・14・16、design.md §0.1（語彙）・§1.7（命名の方針。「task は英語のまま。TPS に重なる語が無い」）・§3.5（kanban の CLI の名前。`deck` の別名は 1 バージョンだけ）、glossary skill（「タスク（work item）。避ける: チケット、issue、job」）、[contract.md](../contract.md) §2・§4〜§8・§10、[ledger.md](../ledger.md)（Compatibility の「フォルダ名・キーの改名はバージョンを上げる」、Moved without a version change、Moving from `.flightdeck/`）、chief-of-staff skill §2（依頼書の雛形）、`internal/triage/apply.go`（トリアージが書く依頼書の欄）、`package/host/*`（ラベル `task` と path `kanban/tasks` を読む mod）、andon main の実測（`grep`。前提 6）、Y-141（kanban の置き場の改名。PR #60 の差分の大きさと `kanban migrate` の形）。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 語は 課題（issue）と 作業（task）にし、課題が作業を内包する。「タスク」は使わない | `package/skills/glossary/SKILL.md` | 生きている |
| 2 | 識別子を `task` から `issue` に改める。対応表を決める | `docs/ledger.md`・`schema/ledger/issue.v3.schema.json` | 生きている |
| 3 | kanban の形式を v3 にし、`kanban migrate` が v2 を 1 回で移す。別名は `andon task` だけ、1 バージョン | この ADR | 覆された → この ADR の 7（別名と migrate をやめた） |
| 4 | 依頼書の欄「やること」を「解決すべき問題」に改める | `package/skills/chief-of-staff/playbooks/brief.md` | 生きている |
| 5 | 外部トラッカーのものは「<トラッカー名> の Issue」と書き分け、kanban のカードは「課題」と書く | `package/skills/glossary/SKILL.md` | 生きている |
| 6 | 変える範囲と順は、形式・command・文と mod の順にする | この ADR | 役目を終えた |
| 7 | 後方互換は作らない。別名・読む側の互換・`kanban migrate` の v2 から v3 は作らず、既存の kanban はオーナーが手で移す | `docs/ledger.md` | 生きている |
| 8 | 手順 5 のファイル名を直し、`impact` の入れ子のキーも `issue` に揃える | `schema/notification.schema.json` | 生きている |

## 問題（オーナーの言葉で。言い直しの段落）

オーナー（2026-11-07）: 「Task の主は How だが、解消したいのは Why/What。それを言葉にすれば issue」。1 回目の提案（語だけ改め、識別子は `task` のまま）への答え: 「そこまで task⇒issue は大したことじゃないと思っている。issue の中に task が内包されるものだから、理解の阻害をしないほうがいいように思う」。欄の名前は「解決すべき問題 みたいな書き方でもいいかも」。2 回目の提案（別名 1 バージョン・mod の例外・`kanban migrate`）への答え: 「まださほど動いていないから後方互換は気にしなくていいよ」。

言い直すと: kanban のカード 1 枚が持つべきものは「何が困っていて（Why）、何が出れば解けたか（What＝完了条件）」で、どう直すか（How）は担当が理解と計画で決める（ADR-0006 決めたこと 9・10・12）。出典（曽根。ADR-0006 前提 9）でも「課題を整理して**作業（タスク）**に分解する」で、task は課題ではなく作業の側の語。だから名前は **課題（issue）が作業（task）を内包する**形にし、識別子もその形で読めるようにする——`andon issue start` で課題を起動し、担当がその中で task（作業）を計画する。1 回目の提案が識別子を残した理由（コスト・外の Issue との衝突）は、オーナーの判断で「理解を妨げないこと」より軽いとされた。コストは払い、衝突は書き分けで持つ。互換の仕掛け（別名・2 つのラベル・移す command）は、動いている kanban が少ない今は書かず、既存の kanban は手で移す。

## 前提（番号。依頼書と現地から取ったもの）

1. **オーナー（2026-11-07。答え 1）**: 語も識別子も `issue` に改める。kanban のカード＝issue、その中に担当が計画で切る task（作業）が入る、という包含が名前で読めるようにする。コストは大きな理由にしない。
2. **ADR-0006 前提 10・決めたこと 9**: 課題＝kanban のカード（「何が困っていて、何が出れば解けたか」と完了条件）。作業＝担当が計画で課題を分けた単位（kanban に乗らない。終了条件は担当が置く）。この 2 語は Y-191（S2）が glossary に足す。
3. **design.md §1.7 命名の方針 1・2**: 識別子の英語は、系譜の語と TPS の語が競えば系譜の語。`task` は「TPS に重なる語が無い」として英語のまま（§3.5 の表）。リーン開発・Kanban Method の系譜で kanban のカードは work item（glossary の今の括弧もこれ）、issue はトラッカー（GitHub・Jira）の系譜の語。→ この文書は §3.5 の表の `task` の行を覆す（`issue` は系譜の語ではないが、オーナーが「包含が読める」ことを取った）。
4. **glossary skill**: 見出しの括弧は識別子に使う英語。今の見出しは「タスク（work item）」だが識別子は `task` で、括弧と識別子が食い違っている（この文書で直す）。「避ける: チケット、issue、job」。
5. **ADR-0006 決めたこと 14**: 判断の標準は 機械が確かめる形 → 文 → オーナーのレビュー の順に置く。名前が変えるのは読む人と agent の判断の向きで、機械の判定は変わらない。→ 改名の効き目は文にあり、識別子の改名は「名前で包含が読める」ためのコスト（前提 1）。
6. **現地の実測（andon main `96f40f6`。`grep -c 'タスク'`・`grep -ci 'task'`。横展開＝task／タスクがオーナーの目に出る所を数えた）**:

   | 場所 | 「タスク」（日本語の文） | `task`（識別子・英語） | オーナーの目に出るか |
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

   前例: Y-141（kanban の置き場 `.flightdeck/` → `kanban/`・`coordinator` → `chief_of_staff`・id を 4 桁）は 66 ファイル・+1,740／−726 行で、`kanban migrate` を 1 本足した（`doing`・`review` があれば止まる・dispatcher の控えも移す・`project.json` の案を書く）。この改名はそれより広い（上の表の全部）。見積りは Y-141 の約 2 倍。
7. **外部トラッカーとの線（chief-of-staff skill 規約 7・design.md §1.3 の 3）**: 外部のトラッカー（Backlog・GitHub Issues 等）で回る案件はそちらが正で、kanban のカードは `links` にキーを持つ写し。ADR-0006 決めたこと 6 の例 3 は「再現手順は issue #12」と GitHub の Issue を指す。design.md §1.9 は「外からの issue は無い」「信号の入口（CI・issue・監視）は最初のバージョンの後」。→ kanban の課題と外の Issue が同じ語になるので、書き分けを決めたこと 5 で持つ。
8. **オーナー（2026-11-07。答え 2）**: 依頼書の最初の欄の名前は「解決すべき問題」（「問題」1 語より、課題として何を解くかが読める）。ほかの欄名・雛形・`andon triage apply`・tests の揃え先もこの語。
9. **オーナー（2026-11-07。答え 3）**: 「まださほど動いていないから後方互換は気にしなくていい」。→ `andon task` の別名の期間・App の mod の `labels.task` の例外・読む側の互換・`kanban migrate` の v2 → v3 は作らない。既存の kanban は operate.md の短い手順（フォルダとキーの改名）でオーナーが手で移す。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-07 語は 課題（issue）と 作業（task）。課題が作業を内包する。「タスク」は使わない（課題の意味でも作業の意味でも）。**

   課題（issue）は kanban のカード 1 枚で、何が困っていて何が出れば解けたか（解決すべき問題）と完了条件を持ち、どう直すかは持たない。置くのは Chief of Staff（依頼書）で、完了条件は担当が書き換えない。作業（task）は担当が計画で課題を分けた単位で、1 つの課題に 1 つ以上ある。kanban に乗らず、識別子は無い。担当の報告の `## 計画` に、担当が置く終了条件つきで 1 行ずつ書く。

   - 細部（2 語の表、包含の図、用語集の「課題」「作業」の項と「避ける」の行、「タスク（work item）」の項を消すこと、仮設の語の表に足す 2 行）は、今は `package/skills/glossary/SKILL.md` の「課題」「作業」の項・包含の図・「仮設の語（2026-11 以前）」の表と、[design.md](../design.md) §0.1 の表が正。
   - 英語の文（README・ledger.md・schema の description）は **issue** と **task**（作業）。今の正は README の冒頭の段と [ledger.md](../ledger.md) の「Changes from version 2」。
   - ADR 0001〜0007・decisions.md の過去の行は当時の言葉のまま（「タスク」「task」は課題の意味で読む）。

   理由: 前提 1・2。出典の語（前提 9 の「作業（タスク）」）では task＝作業で、オーナーの答えもその包含。カードを task と呼び続けると、ADR-0006 の背骨（課題の完了条件・作業の終了条件）が skill に入ったとき「task」が課題と作業の両方を指して読めなくなる。語と識別子が同じ向きなら、glossary の 1 行で「識別子は `task` だが意味は課題」と持たなくてよい。
   捨てた案: 語だけ改めて識別子は `task` のまま（1 回目の提案。決めたこと 2 の捨てた案）／カードを「作業」と呼ぶ（How の側の語。オーナーの指摘の逆）／英語の文は task のまま issue を使わない（識別子と文が食い違う）／作業も kanban に乗せる（ADR-0006 決めたこと 9 の捨てた案のまま。作業は報告の `## 計画` で足りる）。

2. **2026-11-07 識別子を `issue` に改める。対応表はこれ。読む側に古い名前の互換は作らず、`kanban migrate` が 1 回で移す（3）。**

   改めるのは、kanban の置き場とファイル（`kanban/issues/<id>/issue.yaml`）・形式のバージョン・`kanban show --json` と `list --json` のキー・英語の文・担当の command（`andon issue <sub>`）・トリアージの `--max-issues`・予約キーの schema の名前（`issue-keys`）・通知／報告／問い／gate／プロジェクトの schema のキーと値と `$id`・hc のラベル `issue=`・フック・Go の package（`internal/issue`）・dispatcher の控えのキー・App の mod の path とラベル。変えないのは、`issue.yaml` の中のキー・sub command の名前と引数・`log.jsonl` の形・id の形・Space と tab の id と名前・worktree と branch の名前・仮設の語と識別子。
   細部（場所ごとの今・後・備考の表）は、今は [ledger.md](../ledger.md) の「Changes from version 2」（置き場・ファイル・バージョン・JSON・文）、[contract.md](../contract.md) §10 の「バージョン 2」の行（schema のキーと値・`$id`・`issue-keys`・ラベル）と §8 のラベル、[operate.md](../operate.md) の「kanban を形式 3 に手で移す」（dispatcher の控え・`project.json`・App の mod）が正。7 と 8 が指す「2 の表」の行は、整理前のバージョンにある（冒頭の「整理」の行）。

   理由: 前提 1。包含を名前で読むには、オーナーと agent が打つ `andon issue start`・読む `issues/<id>/issue.yaml`・ボードが読むラベルまで同じ語でなければ、「`task` は課題」の注が残る。読む側に互換を作らないのは Y-141 と同じ（2 つの名前を読む code は、どちらが正か分からなくなる。decisions.md §16）。App の mod の `labels.task` だけは例外: App は 1 つで、仮設の端末（`task=`）も同じ App に出ているので、仮設が止まるまでは名前が付かなくなる。
   捨てた案（1 回目の提案。識別子は `task` のまま）: 効き目は文にあり（前提 5）、`issue` は外のものの語（前提 7）、コストは Y-141 の約 2 倍で形式のバージョンが上がる——オーナーが「コストは大きな理由にしない」「理解を妨げないほうが大事」と答えた（前提 1）。／`card`・`item`（かんばんの語。衝突しない）: 包含（課題 ⊃ 作業）が読めない。／schema の `$id` を上げず、キーだけ改める: ledger.md と contract.md §10 の決まり（キーの改名はバージョンを上げる）に反する。／App の mod に `labels.task` を残さない: 仮設の端末の名前が消える（オーナーが毎日見る面）。

3. **2026-11-07 移し方。kanban の形式を v3 にし、`kanban migrate` が v2 の kanban を 1 回で移す。別名は `andon task` だけ、1 バージョン。**

   - **7 で覆した**: `kanban migrate` の v2 → v3・`andon task` の別名・ラベルの例外は作らない。この項で生きているのは「読む側は v3 だけ読み、v2 は 1 行で止まる」「順（Y-191〜Y-195 のあと・1 つのパッケージのバージョン）」だけで、移す手順は 7。
   - **`kanban migrate`（v2 → v3）**。止まって何もしないのは: `config.yaml` の `version` が 2 でない／`issues/` がもうある／`task.yaml` が読めない・課題のフォルダが symbolic link／dispatcher の控えが読めない／**`doing`・`review` の課題がある**（端末がラベル `task=` を持って動いていて書き換えられない。worktree・branch・Space の id は変わらないので理由はラベルだけ。先に `finish` する）。
     するのは順に: (1) `tasks/` → `issues/`、各 `task.yaml` → `issue.yaml`（中身は変えない）、(2) 各課題の `library/report.json` の `next[].to` と `questions.yaml` の `from` の `task` → `issue`（`done` のものも。読む互換を作らないため。`report.md`・`brief.md`・`log.jsonl` は触らない）、(3) dispatcher の控えのキー `task` → `issue`、(4) `<root>/project.json` の `triage_max_tasks` → `triage_max_issues`（オーナーのファイル。書き換えて「commit して」と出す。無ければ書かない）、(5) `config.yaml` を `version: 3`、(6) `log.jsonl` に `{"migrate": {"from": "tasks", "version": 3}}`。
     `.flightdeck/` → `kanban/` の移行（v2 のまま）は残す（今の `migrate`）。両方要る kanban は 2 回打つ（v1 の読み込みは今までどおり断る）。
   - **読む側**: `kanban`・`andon`・dispatcher・gate・App の mod は v3 だけ読む。v2 の kanban に当たったら「format 2; run `kanban migrate`」の 1 行で止まる（ledger.md Compatibility の「知らないバージョンは止まる」）。
   - **別名の期間**: `andon task <sub>` は `andon issue <sub>` と同じことをする（`--help` には出さない）。残すのは改名を入れたパッケージのバージョンだけで、次のバージョンで消す（design.md §3.5 の `deck` の別名と同じ）。`kanban` に別名は無い（sub command の名前が変わらない）。ラベル `task=` は書かない（App の mod が読むのは仮設のため。2）。
   - **順**: 改名は 1 つのパッケージのバージョンにまとめて入れる（セルはバージョンアップ 1 回で移る）。Y-191〜Y-195（ADR-0006 の実装。`andon task candidates`・`task-keys` の `deliverable` を書く）のマージのあとに始める。先にすると 3 本のタスクが rebase になる。
   - **セルの手順**（operate.md に写す）: パッケージのバージョンを上げる → `kanban migrate`（プロジェクトごと） → `project.json` を commit → App の mod を置き直す → `andon boot`。

   理由: ledger.md の決まり（フォルダ名の変更はバージョンを上げて告げる。知らないバージョンは止まる）。Y-141 で「互換は作らず migrate が 1 回で移す」「`doing`・`review` があれば止まる」の形が動いたので同じ形。`report.json`・`questions.yaml` の値を書き換えるのは、トリアージと振り返りが `done` の課題の報告も読むため（残すと読む側に `task` の互換が要る）。
   捨てた案: 読む側が v2 も読む（2 つの形を読む code）／`task=` と `issue=` の両方のラベルを書く（二重の名前が残る）／`andon task` の別名を残さない（skill・オーナーの手が一斉に変わる日に打ち間違いで止まる。1 バージョンだけ）／`project.json` はオーナーに手で直してもらう（migrate が全部の案件を 1 回で揃えるほうが確か。commit はオーナー）。

4. **2026-11-07 依頼書の欄「やること」を「解決すべき問題」に改める。書くことは ADR-0006 決めたこと 12 のまま（何が困っていて、何が出れば解けたか。手順ではない）。**

   - 雛形の 1 行目と欄の並びの細部は、今は `package/skills/chief-of-staff/playbooks/brief.md` の雛形と欄の表が正。
   - 読む側（worker skill §2「依頼書の『やること』『完了条件』『守ること』だけをする」）は「解決すべき問題」を読み、古い依頼書の「やること」も同じ欄として読む（1 行の括弧）。既存の依頼書は書き換えない。
   - `andon triage apply` が書く依頼書の欄と triage skill の説明も揃える。今の正は `internal/triage/apply.go` と `package/skills/triage/SKILL.md`、雛形の検査は `tests/package.sh` の 7。
   - ADR-0006 決めたこと 12 の表の「やること」の行は、この文書で「解決すべき問題」と読み替える（ADR-0006 は書き換えない）。

   理由: 前提 8。「やること」は字のとおり How（する事）の欄名で、決めたこと 12 が「手順ではない」と但し書きで押さえているのは、名前が中身と逆だから。「解決すべき問題」は課題（issue）の定義そのもので、Chief Engineer の文書の最初の節「問題（オーナーの言葉で）」とも向きが同じ。
   捨てた案: 「問題」（1 回目の提案。オーナーが「解決すべき問題」のほうが何を解くかが読めると答えた）／「困りごと」（口語で読みやすいが、Chief Engineer の文書の「問題」と 2 語になる）／「課題:」（カードの名と欄の名が同じで、課題の中の課題になる）／「やること」のまま但し書きで持つ（名前が読み手を How へ引く）／欄を 5W1H に作り直す（ADR-0006 決めたこと 12 の捨てた案のまま）。

5. **2026-11-07 外部トラッカーとの書き分け。kanban のカードは「課題」、トラッカーのものは「<トラッカー名> の Issue」（GitHub の Issue #12・Backlog の課題 PROJ-12）。`links` は課題からトラッカーのものへの参照で、中身は写さない（規約 7 のまま）。**

   - skill と docs の文で外のものを指すときはトラッカー名を添える。ADR-0006 決めたこと 6 の例 3 は Y-195 が skill に写すとき「GitHub の Issue #12」と書く（ADR-0006 は書き換えない）。
   - command と schema で外のものを持つときは `links`（今の予約キー）だけ。`issue` の語を外のものの識別子に使わない（信号の入口（design.md §1.9）が入っても、外の Issue は `signal` の中身で、kanban の課題は `issue`）。
   - Backlog の「課題」（Backlog の日本語 UI の語）と kanban の課題が同じ文に出るときは「Backlog の課題 <キー>」と書く。

   理由: 前提 7。語を 1 つにした代わりに、外のものには必ずオーナーの名前を添える。識別子では `issue` が kanban のもの、外のものは `links` と `signal` で、名前が重ならない。
   捨てた案: kanban のカードを別の語にして衝突を避ける（1 回目の提案。前提 1）／外のものを「外部 Issue」と呼ぶ（オーナーの名前のほうが読める）。

6. **2026-11-07 変える範囲と順（実装のタスクが直す。この文書の PR では変えない）。Y-191〜Y-195 のマージのあと、形式 → command → 文・mod の順。**

   変える所と変えない所の一覧（`cmd/kanban`・`internal/ledger`・ledger.md・`internal/issue` ほかの Go・`schema/`・contract.md・`package/hooks/stop`・glossary・skill と template・docs・`package/host`・tests の、どこを変え、どこを変えないか）は、実装の課題が直し終えて役目を終えた。表は整理前のバージョンにある（冒頭の「整理」の行）。それぞれの今の姿は、その場所のファイルが正。

   順: Y-191・Y-194・Y-195 のマージ → S2（形式 v3）→ S3（`andon` の command・schema・contract.md・hooks）→ S4（glossary・skill・template・依頼書の欄・Go の日本語の文字列）・S5（docs）・S6（App の mod）は独立 → S7（セルで実測）。全部を 1 つのパッケージのバージョンに入れる。

   理由: 形式が先でないと command のテストが書けない。同じ skill を Y-194・Y-195 と同時に触ると衝突する（chief-of-staff skill §3 の 1「ぶつからないか」）。語の改めは Y-170（建屋 → セル）と同じ型の仕事。
   捨てた案: 全部を 1 本（Y-141 の 2 倍の差分。レビューできない）／文書を触るたびに直す（glossary の「ここにある語だけ」が長く破られ、2 つの名前が同居する）／形式と command を別のバージョンで出す（セルが 2 回移る）。

7. **2026-11-07 後方互換は作らない（オーナーの答え。2 の表・3・6 の一部を覆す）。`andon task` の別名・App の mod の `labels.task`・読む側の互換・`kanban migrate` の v2 → v3 は作らない。既存の kanban はオーナーが手で移す。手順は operate.md に写す。**

   - **覆すもの**: 2 の表の「担当の command」「形式のバージョン」「プロジェクトの schema」「dispatcher の控え」「App の mod」の行（表は直した）、3 の `kanban migrate`・別名・ラベルの例外、6 の S2・S5・S6 の中身（表は直した）、やらないことの「例外」。
   - **残るもの**: 形式のバージョンは 3 に上げる（ledger.md の決まり。フォルダ名の変更はバージョンを上げる）。読む側（`kanban`・`andon`・dispatcher・gate・App の mod）は v3 だけ読み、v2 の kanban には「format 2; move it by hand (docs/operate.md)」の 1 行で止まる（v1 と同じ扱い。ledger.md「There is no migration」の形）。`kanban migrate` は今の `.flightdeck/` → `kanban/`（v2 のまま）だけを持ち、v3 への行は足さない。
   - **手で移す手順**（kanban ごと。9 行。各手順の command と確かめ方の細部は、今は [operate.md](../operate.md) の「kanban を形式 3 に手で移す」と [ledger.md](../ledger.md) の「Changes from version 2」が正）:
     1. `doing`・`review` の課題を片付ける。ラベル `task=` を持つ担当・レビュアー・Chief Engineer の端末が居なくなる。
     2. パッケージのバージョンを上げる（`andon`・`kanban` が v3 を読む）。
     3. `mv kanban/tasks kanban/issues`、各 `issues/<id>/task.yaml` を `issue.yaml` に。
     4. `kanban/config.yaml` の `version: 2` を `version: 3` に。
     5. `issues/*/library/report.json` の `"to": "task"` を `"to": "issue"` に、`issues/*/questions.yaml` の `from: task` を `from: issue` に。
     6. dispatcher の控えを消す。控えが守るのはレビュアーの二重起動と gate の重複で、動いている課題が無ければ要らず、次の見回りで作り直される。
     7. `projects/<slug>/project.json` の `triage_max_tasks` を `triage_max_issues` に直して commit（無ければ何もしない）。
     8. オーナーの App の mod を置き直す。
     9. `andon boot`。`kanban lint` が通れば済み。
     `log.jsonl` の行・`brief.md`・`report.md` は触らない（過去の行は当時の言葉のまま）。
   - 動いている課題がある kanban で手順 3 以降をすると、端末のラベル `task=` と kanban が食い違い、dispatcher が担当を見失う。だから 1 を先に、`kanban lint` を最後に。

   理由: 前提 9。動いている kanban は少なく（セルフホストの前）、互換の仕掛け（別名・2 つのラベル・migrate とそのテスト）は書く量とテストを増やすのに使うのは数回。Y-141 の migrate が持っていた守り（`doing`・`review` で止まる）は手順の 1 に置き、確かめは `kanban lint` に任せる。バージョンを 3 に上げるのは残す: 読む側が v2 の kanban を「壊れた」ではなく「古い。手順はここ」と言えるため。
   捨てた案: 2・3 の形（別名 1 バージョン・`labels.task` の例外・`kanban migrate` の v2 → v3。オーナーが「後方互換は気にしなくてよい」と答えた）／形式のバージョンを上げない（フォルダ名の変更はバージョンを上げる決まり。v2 を読む code が無いのにバージョンが同じだと、止まる理由が言えない）／`kanban lint --fix` で移す（migrate を名前を変えて作るのと同じ）／dispatcher の控えのキーを手で書き換える（消すほうが短く、守りは要らない）。

8. **2026-11-07 S3（Y-209）の実装で足したこと。7 の手順 5 のファイル名を直し、`impact` の入れ子のキーも `issue` に揃える。**

   - **7 の手順 5 のファイル名の誤り**: 問いの JSON は `issues/*/questions.yaml` ではなく `issues/*/library/questions/<n>.json`（contract.md §5。`questions.yaml` は kanban の CLI のファイルで `from` を持たない）。手順 5 は「`issues/*/library/report.json` の `"to": "task"` を `"to": "issue"` に、`issues/*/library/questions/*.json` の `"from": "task"` を `"from": "issue"` に」と読む（ledger.md の手順と `tests/kanban.sh` はこの形）。3 の (2) の `questions.yaml` も同じ読み替え。
   - **`impact` の入れ子**: 2 の表に無かった通知 `impact` の `detail.tasks[]`・その1件の `task`、`andon issue impact` の出力の `task`・`tasks[]` も `issues[]`・`issue` にした（`notification/2` の中。キーに task を残すと、表の「通知の schema」の行と食い違う）。
   - **変えなかったもの**: トリアージの `decisions.json` の `task`（起動する課題の中身）・記録の `created_task`・`andon triage collect` の項目の kind `task` は、triage skill が形を持つので S4 に回した（`andon` が書く schema の約束ではない）。`cmd/kanban` の Go の識別子（型 `task` など）は S2 の範囲で、オーナーの目に出ない。

   理由: 7 の手順は S2 が ledger.md と `tests/kanban.sh` に写したとき正しい置き場で書いたが、この文書の本文が古い名前のままだった。`impact` は表から漏れていた（受け入れ条件 3 の「`task` を持つ例が落ちる」に当たる）。

## やらないこと（範囲の外。理由）

- **読む側の互換と移す command**（v2 の kanban を読む・`task=` のラベル・`next[].to: task`・`andon task` の別名・App の mod の `labels.task`・`kanban migrate` の v2 → v3）: 決めたこと 7。既存の kanban は手で移す（仮設の端末に名前が付かなくなるのは仮設が止まるまでの間だけ）。
- **kanban の CLI の名前 `kanban`・sub command の名前・id の形・Space／tab／worktree／branch の名前**: 変えない（`task` を含まない）。
- **ADR 0001〜0007・decisions.md の過去の行・`log.jsonl`・既存の依頼書と報告の Markdown の書き換え**: 当時の言葉のまま（decisions.md §16 の前例）。
- **「作業」を kanban に乗せる**（作業の一覧の形・`library/plan.json`・作業の id）: ADR-0006 決めたこと 9 の捨てた案のまま。作業は報告の `## 計画`。
- **仮設の工場（yard・`yard-task`・`deck`・`.flightdeck/`）の語と識別子**: 仮設は止まるまで当時の言葉のまま。
- **外の Issue を kanban に取り込む口**（信号の入口）: design.md §1.9 のまま最初のバージョンの後。この文書は識別子 `issue` を kanban に取ることだけ決めた。
- **ボードの mod の変数名**: オーナーの目に出ない。path とラベルだけ変える。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

1. **形式（`tests/kanban.sh`・`tests/kanban/test-deck.mjs`・`go test ./cmd/kanban/... ./internal/ledger/...`）**: `kanban init` が `version: 3` と `issues/` を作る。`kanban add` が `issues/<id>/issue.yaml` を書く。`kanban show --json` が `.issue`、`list --json` が `.issues`。`version: 2` の kanban は全部の sub command（`migrate` を含む）が「format 2; move it by hand (docs/operate.md)」の 1 行で止まる（終了コード 1。何も書かない）。v2 の kanban の写しに operate.md の手順（7 の 3〜7）を当てると `kanban lint` が通り、`list`・`show` が読める。
2. **command（`tests/andon-issue.sh`・`tests/andon.sh`・`go test ./...`）**: `andon issue start|finish|ready|report|stop-check|ask|review-request|show|candidates` が今の `andon task` と同じ振る舞い。`andon task <sub>` は知らない command として止まり、`andon --help`・`andon issue --help` に `task` が出ない。起動した端末のラベルが `issue=<id>`（`task=` は無い）。担当の最初のメッセージが「課題 `<slug>/<id>`。依頼書は …」。`cmd/`・`internal/` の日本語の文字列に「タスク」が無い（`grep -rn 'タスク' --include='*.go' cmd internal` が空）。`andon triage apply --max-issues` が効き、書く依頼書の最初の欄が `解決すべき問題:`。
3. **schema（`tests/schema.sh`）**: `issue-keys.schema.json` があり `task-keys.schema.json` が無い。notification／report／question／gate／project の `$id` が `/2`。contract.md の例が全部通る。`task`・`reviewer_task`・`triage_max_tasks`・`to: task`・`from: task` を持つ例が落ちる。
4. **dispatcher と gate（`tests/andon-dispatcher.sh`・`tests/andon-gate.sh`）**: 通知が `issue`・`reviewer_issue` で届く。gate の記録が `review.issue`。dispatcher の控えのキーが `issue`。レビュアーの端末のラベルが `issue=<親>-r<回>`。
5. **パッケージ（`tests/package.sh`）**: glossary に「課題（issue）」「作業（task）」の項と包含の図があり、「タスク（work item）」の項が無く、「タスク」は「避ける」の行と仮設の語の表にしか無い。skill 6 本と template 7 本に「タスク」と `andon task` が無い。chief-of-staff skill §2 の雛形の最初の欄が `解決すべき問題:` で `やること:` が無い。worker skill が「解決すべき問題」と古い「やること」の両方を読むと書いてある。`package/hooks/stop` が `andon issue stop-check` を打つ。
6. **App の mod（`tests/host/mod.mjs`・`tests/host/screen.mjs`）**: kanban-board が `kanban/issues` を読み、`kanban/tasks` は読まない。context-header／inbox／andon が `labels.issue` で名付け、`labels.task` は読まない。
7. **文書**: design.md §0.1 の表に「課題 \| issue」「作業 \| task」、§3.5 の表の `task` の行に ADR-0008 への参照。ledger.md が `version` 3 と `issues/` を書き、バージョン 2 からの変更の表があり、移すのは手で（operate.md）と書いてある。operate.md に 7 の手順がある。docs の日本語の文に「タスク」が無い（decisions.md・ADR を除く。`grep -c 'タスク' docs/design.md docs/contract.md docs/dispatcher.md docs/operate.md README.md` が全部 0）。`tests/public.sh` が通る。
8. **セルで実測（P4 以降の決まり。報告に書く）**: パッケージのバージョンを上げたセルで、`doing`・`review` の無い kanban を operate.md の手順（7）で移すと `kanban lint` が通り、dispatcher が控え無しから見回りを続け、App のボードが課題を出す（mod を置き直したあと）。「解決すべき問題:」で書いた依頼書を 1 本流し、担当の報告の `## Report` の冒頭の言い直しがその欄と完了条件から書かれ、依頼書に手順が無くても担当が計画して PR が出る（オーナーの turn がゼロ）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

| # | 手 | 完了条件 | 依存 | プロファイル |
|---|---|---|---|---|
| S1 | この文書の PR（decisions.md §14 に 1 行） | `tests/run`・`tests/public.sh` ok（CI は予算で止まっているので手元で） | — | — |
| S2 | kanban の形式 v3（v2 は 1 行で止まる。`kanban migrate` は足さない）、`cmd/kanban` の JSON と文、ledger.md のバージョン 3 の節、`tests/kanban` | 受け入れ条件 1 | S1、Y-191・Y-194・Y-195 のマージ | `claude` |
| S3 | `andon issue`（`internal/task` → `internal/issue`・別名なし・ラベル `issue=`・`--max-issues`・Go の日本語の文字列）、schema の改名と `$id`、contract.md §1・§2・§4〜§8・§10、`package/hooks/stop`、dispatcher・gate のキー、tests | 受け入れ条件 2・3・4 | S2 | `claude`（Review: on。約 500 行超） |
| S4 | glossary（課題・作業・包含の図・仮設の語の表）、skill 6 本と template 7 本の語と `andon issue`、依頼書の雛形の「解決すべき問題」と worker skill の読み方、`andon triage apply` の欄名 | 受け入れ条件 5 と 2 の欄名 | S3 | `sonnet` |
| S5 | docs（design §0.1・§1.7・§3.5・dispatcher・operate〈手で移す手順。7〉・README） | 受け入れ条件 7 | S3。S4 と独立 | `sonnet` |
| S6 | App の mod（kanban-board の path、ラベル `issue`）、`tests/host` | 受け入れ条件 6 | S2。S3〜S5 と独立 | `claude` |
| S7 | セルで実測（受け入れ条件 8）。食い違えば実測が正で、この文書の「決めたこと」に下から足す | 報告 | S2〜S6 を 1 つのパッケージのバージョンに入れたあと。Y-196 と一緒でよい | `claude` |

## 開いている問い（オーナーに聞く。1 問ずつ。推奨案つき）

無し。1 回目の提案の 2 問（識別子を残すか・欄の名前）はオーナーが同日に答え、前提 1・8 に写した。3 回目の答え（後方互換は気にしない）は前提 9 と決めたこと 7。実装で決める細部（止まる文言・手順の sed の形）は推奨どおり進め、担当が報告に書く。

## 出典（読んだもの・捨てた案の出所）

- 依頼書（オーナーの言葉 2026-11-07。追記の答え 1・2・3）、[ADR-0006](0006-survey-fix-decide.md) 前提 9・10、決めたこと 6・9・12・14・16、開いている問い 2。
- [design.md](../design.md) §0.1（語彙）・§1.7（系譜と命名の方針・かんばんの当て）・§1.9（外からの issue は無い。信号の入口）・§3.5（CLI の名前。`deck` の別名 1 バージョン。`task` は英語のまま、の行）。
- glossary skill（`package/skills/glossary/SKILL.md`）、chief-of-staff skill §2・§3・§9 規約 7、worker skill §2、triage skill、`internal/triage/apply.go`、`package/hooks/stop`、`package/host/{kanban-board,context-header,inbox,andon}/main.js`。
- [contract.md](../contract.md) §2（`task.yaml` の予約キー）・§4〜§7（報告・問い・通知・gate のキー）・§8（hc のラベル）・§10（互換とバージョン）、`schema/*.json`、[ledger.md](../ledger.md)（Compatibility・Layout・Moved without a version change・Moving from `.flightdeck/`・Changes from version 1）。
- [decisions.md](../decisions.md) §16（Y-141 の行。「前の名前を読む互換は作らず、`kanban migrate` が 1 回で移す」「前の行は当時の言葉のまま」）、PR #60 の差分（66 ファイル）、`cmd/kanban/migrate.go`（止まる条件・dispatcher の控え・`project.json` の案）。
- 実測: andon main `96f40f6` の `grep`（前提 6）。
