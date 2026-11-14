# ADR-0012: 止まった担当を resume で起動し直す・worktree の置き場（仕様・設計。2026-11-10。課題 A-0034）

- 状態: **決定（2026-11-10）**。人が 2026-11-10 に「決めた」と言った（Chief Engineer の対話 A-0034）。人が PR をマージした日が決定の日。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を `覆された → ADR-NNNN` にする。小さな改めは「決めたこと」に下から足す。
- 主題: (a) セルが起き直したあと、`doing` の担当とレビュアーをハーネスのセッションの resume で続きから起動する入口と手順。(b) worktree の置き場を永続の場所へ移すこと。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。
- 入力: 題（人の1行。2026-11-10）、人の発言（2026-11-10。「再度起すときは resume したほうがいいんじゃないかなぁ」「マウントするか worktree のディレクトリを検討する必要あるね」「workspace に .worktree とかを置くとかがいいんじゃないかな？」）、Chief of Staff の経緯（A-0010・A-0033・A-0033 のレビュアーが 02:14 の起き直しで素のシェルに戻り、worktree が消えた。`andon issue start` が「branch がもうある」で止まった）、`internal/issue/start.go`・`finish.go`、`internal/spawn/spawn.go`、`internal/dispatcher/pickup.go`・`loop.go`・`fallback.go`、`package/compose.yaml`・`internal/boot/owners.go`、`package/templates/*.md`、[dispatcher.md](../dispatcher.md) §2.3・§2.8、[operate.md](../operate.md) §8（引っ越し）、[image.md](../image.md)（信頼の表）、[ADR-0005](0005-cell-repo.md) 決めたこと 2 の表、CoS skill の止まった担当の表、Claude Code 2.1.322 と Codex 0.166.0 の `--help`、セルでの実測（別の cwd から `claude --resume <id> --fork-session` が通る。`hc get` の `keep: true` と argv・cwd・ラベルの保持）。

## 問題（人の言葉で。言い直しの段落）

セルのコンテナが起き直すと、担当とレビュアーのプログラムは死に、herdr の端末は素のシェルとして残る。ハーネス（Claude Code・Codex）の会話の記録はボリュームに載っていて残るのに、Andon にはそれを続きから起動する入口が無い。`andon issue start` は新しい branch と worktree を作る一本道なので、PR と branch がある課題では「branch がもうある」で止まり、Chief of Staff は finish → todo → start でコンテキストを捨てて起動し直すしかなかった。困りごとは「起動し直すたびに担当のコンテキスト（読んだもの・決めたこと・レビューのやりとり）を捨てる」こと。worktree の置き場（`~/.herdr/worktrees`）がボリュームに無く、起き直しで消えたのは、それを悪くする別の欠け。

## 前提（同意済み。番号。人が 2026-11-10 に 1〜5 全部に同意。「ライフサイクルとして finish → todo → start はおかしい」）

1. **問題の所在**: 本当の問題は「起動し直す手段が無い」こと。worktree が消えなくても、起き直しでプログラムは必ず死ぬので、resume の入口は置き場の永続とは独立に要る。
2. **何もしないと**: 起き直しのたびに人の手で finish → todo → start をし、PR のある課題はそれで止まる。コンテキストはすべて捨てる。
3. **既にあるもので済む部分**: herdr は `keep` の端末を素のシェルで残し、元の argv・cwd・ラベル（`issue`・`project`・`role`）を保つ。Claude Code は `--resume <id>`（cwd 不問）と `--continue`（cwd の最新）、Codex は `codex resume <id>` と `codex resume --last`（cwd で絞った最新）を持つ。会話の記録は両方ともボリュームに載っている。新しく決めるのは「どのセッションを選ぶか」「worktree を同じ場所に戻す手」「入口」の3つだけ。
4. **配り方**: Andon の command（`internal/`）と `package/compose.yaml`・`init/` の変更。セルは image を上げて `bin/bootstrap` で作り直す。
5. **範囲**: `andon issue start` で起動した担当と、dispatcher が起動したレビュアー。Chief of Staff と dispatcher の起動し直しは [operate.md](../operate.md) に手順が既にあるので触らない。

## 決めたこと（番号・日付・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-10 起動し直しは resume を基本にする**（人。Chief of Staff との対話で）。新しいセッションで起動するのは resume できないときの戻りだけ。
   理由: コンテキスト（読んだもの・決めたこと・レビューのやりとり）を捨てない。会話の記録はボリュームに残っている。
   捨てた案: finish → todo → start で新しく起動する（今の手。コンテキストを捨て、PR のある課題では止まる。人: ライフサイクルとしておかしい）。
2. **2026-11-10 担当向けの指針の口は、worktree の置き場に依らせない。** セルの repo の見える所に1ファイル（名前は仮に `AGENTS.worker.md`。切る案で確定）を人が書き、Andon が担当とレビュアーを起動する最初のメッセージで両ブランドとも明示して指す（あるときだけ）。
   理由: 実測（下の出典）で、Claude Code は worktree の根より上の `AGENTS.md` を読むが Codex は読まない。置き場の上に置くと効き方がブランド（Claude は暗黙・Codex は届かない）と役（Chief Engineer にも効く・cwd が `issues/<rev>/` のレビュアーには効かない）でちぐはぐになる。Andon の役の区別で「誰に効くか」を決めれば、ハーネスが親を遡る仕様にも依らない。
   捨てた案: `/workspace/.worktrees/AGENTS.md` を置いて Claude に自動で読ませ、Codex とレビュアーには明示の1行（人の最初の案。人が書くファイルが機械の持ち物の隠しディレクトリに入る）／この題ではやらない。
   名前の確定（2026-11-12。A-0042）: `AGENTS.worker.md`、置き場はセルの repo の直下（`$ANDON_WORKSPACE`。既定 `/workspace`）。あるときだけ、`andon issue start`（`resume --fresh` も）と dispatcher のレビュアー（送り直しも）の最初のメッセージに「人の指針 `<パス>` も読む」の1行が入る。中身は人が書き、Andon は書かない。[ADR-0005](0005-cell-repo.md) 決めたこと 2 の表に1行。
3. **2026-11-10 入口は新しい command `andon issue resume <id> [--dry-run] [--fresh]`。** 起動できるのは `doing` で、台帳に `worktree`（repo のある課題）があり、端末（`agent`）が無い・終わっている・素のシェル（`spawn.BareShell`）の課題だけ。動いていれば「端末 … がもう動いている」で止まる。`start` は「新しく作る」の一本道のまま変えない。
   理由: ライフサイクルに「死んだ → resume → doing」の辺を足す（人: finish → todo → start はおかしい）。条件の違う2つを1つの動詞に畳むと `--dry-run` と失敗の文言が濁る。
   捨てた案: `start` が条件を見て自動で切り替わる／両方。
   手順（`start` と同じ骨。確かめ → 戻す → 起動する → 台帳）:
   1. worktree を戻す: 本体の checkout で `git worktree prune` し、台帳の `worktree` のパスが無ければ作る（担当と Chief Engineer は `git worktree add <path> <branch>`、レビュアーは `git worktree add --detach <path> <親の review_request.commit>`）。あれば触らない（commit していない変更を消さない）。パスは台帳の値をそのまま使う（組み直さない。旧の置き場の値でも戻る）。
   2. 素のシェルで残った端末は片付けて（`hc kill`・`hc rm`。`andon chief-engineer` の `Discard` と同じ）、同じ名前・同じラベルで新しく起動する。台帳の `agent` を新しい address に書く（`kanban set`）。status は `doing` のまま。
   3. 起動する行はプロファイルの template の `resume:`（frontmatter に足す。boot が `package/templates` から生成し、`templates.local/` が勝つのは今と同じ）。Claude 系は `command` に `--continue` を足した行、Codex 系は `codex <flags> resume --last`。`hc run` には template 名でなく argv で渡し（template の本文は要らない）、最初のメッセージは Andon が今の手（`spawn.Run` の届いたかの確かめ）で打つ: 「セルが起き直した。worktree は同じ場所に戻した。台帳（`deck list`）と git の状態を確かめ、続きから。報告・問いは今までどおり」。
   捨てた案: 素のシェルの端末に `hc send` で resume の行を打って同じ address を使い続ける（hc の agent の見分けと `foreground` の形が実測なしでは分からず、片付けて起動し直す既存の手で足りる）。
4. **2026-11-10 起動するのは dispatcher（自動）。** 見回りで `doing` の課題の端末が Gone（無い・live でない・素のシェル）なら `andon issue resume <id>` を打つ。1つの死につき1回: 控えに「resume した課題と、死んだ端末の address」を持ち、resume した先がまた Gone なら今どおり `finished` を Chief of Staff に届ける（人の go へ）。resume したことは記録だけの通知 `resumed`（[ADR-0010](0010-news-ack.md) の表に行を足す）。初めて見る台帳では打たない（§2.8 のレビュアーの起動し直しと同じ）。
   理由: resume は判断でなく復旧（「同じことをもう一度」）。起き直しのたびに人が go を言う形は前提 2 の困りごとそのもの。
   捨てた案: Chief of Staff が人に1問して人の go で打つ（今の決まり。コンテキストを捨てて金が掛かる起動し直しのための決まりで、resume には重い）／Chief of Staff が自分の判断で打つ。
   受け入れる難点: 人が意図して `hc kill` した担当も1回は resume される。止めたいときは `andon issue finish` か台帳の status を変える（開いている問い 2）。
5. **2026-11-10 セッションは記録せず、元の cwd で「その cwd の最新」を続ける**（**同日 11 で覆された**。以下は記録のまま）: Claude 系は `claude … --continue`、Codex 系は `codex … resume --last`。cwd は担当と Chief Engineer が台帳の `worktree`、レビュアーが `issues/<rev>/`（今の cwd。A-0033 の直しが入っても同じ）。どちらも課題ごとに一意なので最新で足りる。
   理由: 台帳に新しい鍵も、ハーネスの記録ファイルの形への依存も要らず、両ブランドで揃う。`CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1` は template に既にある。
   捨てた案: 起動するとき id を決めて台帳に書く（Claude は `--session-id`、Codex は起動したあと記録から引く。Codex に id を指定する口が無い）／resume のとき記録ファイル（`~/.claude/projects/<cwd>/*.jsonl`・`~/.codex/sessions/**/rollout-*.jsonl`）を探して id を取る（内部の形に依る）。
6. **2026-11-10 worktree の置き場は `/workspace/.worktrees/<repo 名>/<slug>-<id>`**（レビュアーは `…-r<回>`。`/workspace` はセルの repo の bind mount）。Andon が置き場を作るとき `/workspace/.worktrees/.gitignore` に `*` を書く（`kanban/.gitignore` と同じ手。セルの追跡される `.gitignore` は触らない）。`spawn.WorktreePath`・dispatcher の `reviewerWorktree`・`finish` の対の3か所を揃え、`finish` は旧の置き場（`~/.herdr/worktrees/…`）の値も1版だけ対として受ける。
   理由: 本体の checkout（`/workspace/.ghq`）と同じファイルシステムなので実体と `.git/worktrees/` の登録が片方だけ消えない。ホストから見える。volume を足さない。`~/.herdr` は herdr の持ち物でない。
   捨てた案: `~/.herdr` を compose の volume にする（compose 2行で済むが上の利点が無い）／`/workspace/.andon/worktrees/`（`.andon/` は bootstrap がパッケージを取り出す所で、意味が混ざる）。
   [ADR-0005](0005-cell-repo.md) 決めたこと 2 の表の「worktree」の行は「`/workspace/.worktrees/`・追跡しない（中の `.gitignore`）」に改めた（小さな改め。ADR-0005 決めたこと 7。この PR で）。
7. **2026-11-10 resume できないときは止まり、人の go で `--fresh`。** `resume` は理由を文言にして非 0 で止まる（記録が無い・ハーネスが拒んだ・起動に失敗した）。dispatcher は今どおり `finished` を届け、Chief of Staff が人に1問、人の go で `andon issue resume --fresh <id>`: 同じ worktree・branch・cwd で新しいセッション（template の `command`）。最初のメッセージは `start` のものに「前の担当の報告 `issues/<id>/report.md`（あれば）を読んで続きから」の1行を足す。
   理由: コンテキストを捨てたことに誰かが気づく形にする。
   捨てた案: resume が黙って fresh に落ちる／finish → todo → start。
8. **2026-11-10 レビュアーも同じ `resume` の対象。** profile が `*-review` なら detached worktree を親の `review_request.commit` で戻し、cwd `issues/<rev>/` で起動する。dispatcher の自動（4）の対象。
   理由: dispatcher の「Gone を見たら resume」が1本の規則になる。
   捨てた案: レビュアーは resume せず今の経路（fallback）で新しく起動する。
9. **2026-11-10 Chief Engineer も同じ `resume` の対象。** profile `chief-engineer` ならラベル `role=chief-engineer`・`herdr.space=andon-<slug>`・`herdr.tab=andon-<slug>-<id>`（指揮側）で戻す。dispatcher の自動の対象。`andon chief-engineer "<題>"` が戻った端末を片付けて新しく起動する今の手は、新しい題を起動するときだけに残る。
   理由: 人との対話のコンテキストは担当より失う代償が大きく、コストは role のラベルの分岐だけ。
10. **2026-11-10 コストの見方**（人）: resume のコンテキストの読み直し（1回分の入力）は受け入れる。微々たるもので、応答のラリーの方が高い。コンパクションの制御のほうが大事かもしれない（開いている問い 1。この題では触らない）。
11. **2026-11-10 セッションは Andon が記録し、id で resume する（5 を覆す）。** 反論（出典）で「cwd の最新」が崩れたため。
   - Claude 系: 起動するとき Andon が UUID を決めて `--session-id <uuid>` で渡し、台帳の `session` に書く（`kanban` に列を足す）。resume は `claude … --resume <uuid>`。
   - Codex 系: id を指定する口が無い。resume のとき `~/.codex/sessions/**/rollout-*.jsonl` の先頭行（`session_meta` の `cwd` と `id` の2つだけを読む）から、cwd が台帳の値と同じ最新の id を引いて `codex … resume <id>` で起動し、引けた id を台帳の `session` に書く。
   - どちらも起動する前に記録の有無を確かめ（Claude は `~/.claude/projects/*/<uuid>.jsonl` の有無、Codex は上の検索）、無ければ非 0 で止まる（7 の「記録が無い」はここで判る）。
   理由（反論の実測）: 記録の無い dir で対話の `claude --continue` は「No conversation found to continue」を出したまま終わらず（live のまま黙り、dispatcher は Gone とも stalled とも見ない）、`codex exec resume --last` は黙って新しいセッションを始める。担当が自分の worktree で `claude -p …` を1回打つと、次の `--continue` はその使い捨てセッションを続ける。「記録が無ければ止まる」をハーネスに頼れないので Andon が自分で確かめる。読むのは id と cwd だけに留め、本文は読まない。
   捨てた案: cwd の最新のまま skill に「自分の cwd で claude・codex を打たない」を足す（それでも黙りは検出できない）。skill の1行は R5 で足す（二重の守り）。
12. **2026-11-10 `andon chief-engineer` は戻った端末を片付けない（9 に足す）。** 片付けるのは resume の手順（13 の 2）と `finish` だけ。新しい題は別の課題として起動し、`doing` の旧 CE は dispatcher が resume する。
   理由（反論）: 起き直し後に人が新しい題で打つと旧 CE の端末を Discard するが台帳は `doing` のまま → 次の見回りで dispatcher が旧 CE を resume し、新旧が並ぶ。同時なら同じ端末に両方が kill を打つ。片付けのオーナーを1つにする。
   捨てた案: Discard のとき旧課題の status を変えて（`paused`）resume の対象から外す（新しい鍵が要る）。
13. **2026-11-10 反論で直した細部**（Chief Engineer の推奨。人が同意）:
   1. resume の起動のしかたは、boot が template の `resume:` 行から `<profile>-resume` の hc template を生成し、template 名＋task で起動する（3 の「argv で渡す」を改める）。argv で起動すると `first_message` が返らず、最初のメッセージの届いたかの確かめと dispatcher の送り直しが効かない。
   2. 台帳の `agent`（と `session`）は、新しい端末に最初のメッセージが届いたと確かめてから書く。旧の素のシェルを片付けるのもその後。失敗したら台帳を変えない（3 の手順 2 と 3 の順を改める）。
   3. 「1つの死につき1回」の控えは、resume した端末が働き始めたら（`turn` ≥ 1。`Adopt` と同じ手）消す。消さないと2回目の起き直しが必ず人の go になる。
   4. `resume`・`finish`・`start` は同じ lock（dispatcher の `lockState`）を命令の間ずっと取り、resume は起動する直前に status をもう一度読む。finish と重なると done の課題に live の担当と誰も片付けない worktree が残る。
   5. レビュアーの戻す commit は、親の `review_request.commit` でなく、そのレビュアーの課題の依頼書の `commit <sha>` から取る（回がずれない）。
   6. [operate.md](../operate.md) に「セルの repo で `git clean -x` を打たない（`.worktrees` が消える。`git status` には出ないが `git clean -ndx` は消すと言う）」を1行。

14. **2026-11-10 実装で決めた細部**（R2 の担当 A-0038。推奨案で進めた。戻すなら一言）:
   1. セッションの id は端末のラベル `session=<id>` で渡し、template の行が `andon issue session`（自分の端末のこのラベルを出す。無ければ何も出さずに 0）で読む。Claude 系の `command` は `s=$(andon issue session); … ${s:+--session-id "$s"}`、`resume:` は `… --resume "$s"`、Codex 系の `resume:` は `codex --no-daemon resume "$s"`。
      理由（実測 2026-11-10・hc 0.1.17）: hc の template の `command` は固定の行で、`hc run` は呼び手の環境変数を端末に渡さない（`FOO=… hc run … sh -c 'echo $FOO'` が空）。端末の中にあるのは `HC_TERMINAL_ID` だけで、ラベルは起動するときに付く。11 の「`--session-id <uuid>` で渡す」と 13 の 1 の「template 名＋task で起動する」を両立させる口がこれだった。ラベルが無ければ（人が `hc run claude` で起動したなど）`--session-id` を付けない今の行と同じ。
      捨てた案: 課題ごとに template を書く（boot の持ち物の置き場に課題の数だけ増える）／argv で起動する（13 の 1 で捨てた）。
   2. boot は `resume:` の行を元の template から除いて書く（hc に知らないキーを渡さない。hc が frontmatter の知らないキーをどう扱うかは確かめていない）。`<profile>-resume` の本文は元と同じ。
   3. lock は dispatcher の控えの lock（`lockState`）でなく、同じ置き場の別のファイル `<slug>.issue.lock`（13 の 4 を改める）。
      理由: dispatcher は見回りの間、控えの lock を持ったまま `andon issue finish` を打つ。finish が同じ lock を取ると呼び手を待って止まる（R4 で dispatcher が `resume` を打っても同じ）。resume・finish・start が互いに重ならないことは別のファイルでも守れる。
   4. 素のシェルの端末は、起動する前に片付ける（13 の 2 の「旧の素のシェルを片付けるのも届いた後」を改める）。
      理由: hc の名前は live の端末の中で1つで、素のシェルで戻った端末は live のまま同じ名前を持つ。中身はシェルだけで失うものは無く、台帳は今どおり届いてから書く（届かなければ起動した端末も片付け、台帳は変えない）。
   5. resume は repo の無い課題（cwd `issues/<id>/`）も起動する（worktree は戻さない）。Chief Engineer のラベルは `andon chief-engineer` と同じもの（`chiefengineer.Labels`）。
15. **2026-11-10 実装で決めた細部**（R3 の担当 A-0039。推奨案で進めた。戻すなら一言）:
   1. レビュアーの戻す commit は、dispatcher が依頼書に書く「対象: worktree `…`（detached）。commit <sha>。」の行から読む（13 の 5）。同じ依頼書の「再レビュー: 前の回の commit <sha>。」は読まない。行が無ければ止まる。
   2. あれば触らない確かめは「その repo の detached の worktree として登録されていること」だけ（HEAD が依頼書の commit かは見ない。レビュアーが自分で checkout を動かすことは無く、動いていても消さないほうを取る）。無ければ commit がその repo にあることを確かめて `--detach` で戻す（fetch はしない。dispatcher が起動するときに fetch した object は clone に残る）。
   3. レビュアーのラベルは `project`・`issue`・`title` と、元の課題の担当の Space・tab（`herdr.space`・`herdr.tab` = `andon-<slug>-<元の id>`。`andon issue start` が担当に付ける値）。dispatcher のように担当の端末から写さない（起き直しでは担当の端末も死んでいて読めない）。
   4. Claude 系のレビュアー（`claude-review`）にもセッションの id を渡す: dispatcher がレビュアーを起動するとき（使用量の上限で起動し直すときも）ラベル `session=<id>` を付けて台帳の `session` に書く（Codex 系へ起動し直すときは消す）。`claude-review`・`codex-review` の template に `resume:` を足す（受け入れ条件 3 をレビュアーでも成り立たせるため。dispatcher の自動〈R4〉には触らない）。
16. **2026-11-11 実装で決めた細部**（R4 の担当 A-0040。推奨案で進めた。戻すなら一言）:
   1. 「1つの死につき1回」の控えは dispatcher の控えの `Resumed`（課題 → Gone を見た端末 `from` と起動した端末 `agent`）。打つ前に保存し（打っている間に落ちても同じ死へ二度打たない）、台帳の `agent` が `from` か `agent` と同じ Gone なら打たない。台帳の `agent` が別の値に替わった（Chief of Staff が `--fresh` を打った・レビュアーを起動し直した）あとの死は別の死として1回打つ。
   2. 「働き始めた」は hc の `turn` ≥ 1（終えたターンの数。最初のメッセージを受けて動いている間は 0）。resume が最初のメッセージを届けた直後はまだ 0 なので、最初のターンを終える前に死んだ端末は「働かずに死んだ」として `finished` を届ける。
   3. resume が起動できない（記録が無い・`machine` のある課題・profile が無い・最初のメッセージが届かない）ときは、その死にもう打たず、今どおり `finished` を届ける（決めたこと 7 の「人の go で `--fresh`」へ）。ログに `andon issue resume` の理由の1行を残す。
   4. 初めて見る台帳では、そのとき Gone だった端末を控えに「見た」（`from` だけ）と持ち、次の見回りでも打たない（`finished` も今どおり知らせない）。
   5. 記録だけの通知 `resumed` の `detail` は `agent`（起動した端末）・`from`（Gone を見た端末）の2つ（`schema/notification.schema.json`）。
   6. 受け入れ条件 7（コンテナを起き直す実測）と開いている問い 4 は、担当がセルの中から測れない（起き直すと担当自身も Chief of Staff も dispatcher も死に、入っている dispatcher はこの PR の前の版）。マージして image を上げたあと、人がコンテナを起き直して測る（報告の `## Next`）。

## やらないこと（範囲の外。理由）

- Chief of Staff と dispatcher の起動し直し（[operate.md](../operate.md) に手順がある。前提 5）。
- 担当向けの指針を worktree の置き場の上の `AGENTS.md` で読ませること（決めたこと 2。Codex に届かない）。
- ハーネスの記録ファイルの本文を読むこと（読むのは Codex の `session_meta` の `cwd`・`id` と、Claude の記録ファイルの有無だけ。決めたこと 11）。
- 別のマシンに出した担当（`machine` のある課題。[ADR-0002](0002-herdr-spaces.md) 決めたこと 19）の resume。`start` と同じく止める。
- commit していない変更の救出。ボリュームに無かったものは戻らない。置き場を移せば以後は残る。
- 人が意図して止めた端末と起き直しで死んだ端末の区別（1回だけの規則で抑える。開いている問い 2）。
- コンパクション（コンテキストの圧縮）の制御（開いている問い 1）。

## 受け入れ条件（テストできる形。継ぎ目は `andon issue resume --dry-run` の計画の出力・偽の `hc` を置くシェルテスト〈`tests/andon-issue.sh`・`tests/andon-dispatcher.sh` の形〉・Go の単体テスト・セルの実測の4つ。高いものから）

1. `andon issue resume <id> --dry-run` は、`todo` の課題・端末が動いている課題・`machine` のある課題で止まり（文言に理由）、`doing` で端末が無い／素のシェルの課題で計画（worktree のパス・branch か commit・起動する行・cwd・最初のメッセージ）を出す。
2. 台帳の `worktree` のパスが無いとき、担当は `git worktree add <path> <branch>`、レビュアーは `--detach <commit>` で同じパスに戻る。あるとき（commit していない変更を置いた状態）は触らない。旧の置き場の値でも戻る。
3. `start` が Claude 系に `--session-id <uuid>` を渡し台帳の `session` に書く。resume の起動する行が profile ごとに `<profile>-resume` の template（Claude 系 `--resume <session>`、Codex 系 `resume <id>`）で、cwd が台帳の値（レビュアーは `issues/<rev>/`）。最初のメッセージが届いたあとで台帳の `agent`（Codex は `session` も）が新しい値になり、status は `doing` のまま。記録が無ければ起動せず非 0。
4. dispatcher は Gone の `doing` を見たら `resume` を1回打ち、記録だけの通知 `resumed` を書く。resume 後の端末が働かないまま（`turn` 0 のまま）また Gone なら `finished` を届け、働いたあと（`turn` ≥ 1）に死んだらまた1回 resume する。初めて見る台帳では打たない。resume の間 finish は待つ（lock）。
5. `--fresh` は template の `command` で起動し（Claude 系は新しい `--session-id`）、最初のメッセージに `report.md` を読む1行がある。resume の失敗（記録が無い・最初のメッセージが届かない）は非 0 と理由で止まり、台帳を変えない。
6. 新しい置き場: `start`・dispatcher・`finish` が `/workspace/.worktrees/<repo 名>/<slug>-<id>` を組み、`/workspace/.worktrees/.gitignore` が `*`。`git -C /workspace status --short` に `.worktrees` が出ない。`finish` は旧の置き場の値の課題も片付ける。
7. セルの実測（`package/` の変更は実測。AGENTS.md）: `doing` の担当（Claude・Codex 各1）と Chief Engineer を置いてコンテナを起き直し、dispatcher の次の見回りで3つとも resume され、最初の返事で前の作業を要約できる（コンテキストが残っている）。worktree は起き直しの前後で同じパスにあり、中身が残っている。起き直しの後に人が `andon chief-engineer "<新しい題>"` を打っても旧 CE は消えず、resume される。

## 価値の仮説

- 誰の: 人（オーナー）と Chief of Staff。
- 何が変わる: セルの起き直しのあと、人の手もコンテキストの捨ても無く `doing` の仕事が続く。
- 何で分かる: 記録だけの通知 `resumed` の数と、resume 後に `finished` が届いて人の go が要った数の比（記録の置き場 `<slug>.recorded.jsonl` から数える。measure に行は足さない）。
- いつ見る: 起き直しが2回起きたあと、または 2026-11-24。
- 外れの形: resume した担当が古い状態のまま続けて壊す（台帳や PR が進んでいるのに気づかない）。その場合は最初のメッセージに「確かめること」を増やすか、fresh を既定に戻す。
- 見る人: Chief of Staff が観察の課題で（切る案 R7）。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

- R1 置き場を `/workspace/.worktrees` に（決めたこと 6。`spawn.WorktreePath`・`reviewerWorktree`・`finish` の対と旧の値の受け入れ・`.gitignore` `*`・`operate.md`・`image.md` の表・独立レビューの skill の置き場の行・tests）。受け入れ条件 6。
- R2 `andon issue resume <id> [--dry-run] [--fresh]`（担当と Chief Engineer。決めたこと 3・7・9・11・12・13）: `kanban` の `session` 列、`start`・`chief-engineer` の `--session-id`、template の `resume:` 行と boot の `<profile>-resume` の生成、記録の有無の確かめ、worktree を戻す、起動して届いてから台帳、lock、`andon chief-engineer` の Discard を外す。受け入れ条件 1・2・3・5。R1 のあと（同じファイルに触る）。
- R3 レビュアーの resume（決めたこと 8。detached を戻す・cwd `issues/<rev>/`）。R2 のあと。
- R4 dispatcher の自動 resume（決めたこと 4・13 の 3。Gone → resume 1回・控えと働いたら消す・`resumed` の記録だけ・ADR-0010 の表）。受け入れ条件 4。R3 のあと。
- R5 文書と skill（CoS skill の「止まった担当」の表の最後の行を「dispatcher が resume する。働かずに死んだら人に1問」に、worker・CE skill に起き直し後の最初のメッセージの読み方と「自分の cwd で claude・codex を起動しない」の各1行、`dispatcher.md` §2.3、`operate.md` §8 の「worktree を戻す」を resume に、`operate.md` の `git clean -x` の1行）。R4 のあと。
- R6 担当向けの指針のファイル（決めたこと 2。名前を確定し、`spawn` の最初のメッセージに「あれば読め」の1行、ADR-0005 の表に1行）。独立。
- R7 観察の課題（価値の仮説。`not_before` 2026-11-24）。R4 のあと。
- 実測（受け入れ条件 7）は R4 の担当がセルで行う。

## 開いている問い

1. コンパクションの制御（人 2026-11-10。「むしろコンパクションをコントロールするとかのほうが大事かも」）。長く生きる担当・Chief Engineer のコンテキストをいつ・どう圧縮するか。決断の種類: 範囲（別の題にするか）。
2. 人が意図して `hc kill` した担当を dispatcher が1回 resume してしまう。抑える口（台帳の status を `paused` のようにする／ラベル）を足すか。決断の種類: 仕様（この版では受け入れた）。
3. 対話の `codex resume --last`・`codex resume <id>` が記録の無い cwd／無い id で何をするか（反論で `exec` だけ実測。黙って新規）。R2 の担当が実測し、非 0 で止まるかを確かめる。決断の種類: 事実（人には聞かない）。
   **答え（実測 2026-11-10・Codex 0.166.0・hc の端末で対話のまま）**: `codex --no-daemon resume <無い id>` は「ERROR: No saved session found with ID <id>. Run `codex resume` without an ID to choose from existing sessions.」を出して終了コード 1 で終わる（端末は finished。keep なら素のシェルに替わり、dispatcher は Gone と見る）。`codex --no-daemon resume --last` は記録の無い cwd で黙って新しいセッションを始める（live・idle のまま）。Andon は起動する前に記録から id を引いて `resume <id>` で起動するので（決めたこと 11）、`--last` は使わない。
4. herdr が端末を戻し切る前に dispatcher が not_found を見て同じ名前で起動する競合（反論の疑い。証明されていない）。R4 の実測で見る。決断の種類: 事実。
5. 今 `doing` で止まっている A-0033（旧の置き場）と、`todo` に戻した A-0010（worktree の記録を失った。セッション `7a11eb7e-…` は残っている）の扱い。A-0033 は R2 のあと resume で戻る。A-0010 は手で `--resume <id>` するか、fresh で起動する。決断の種類: 運用（Chief of Staff が人に1問）。

## 出典

- 人の発言 2026-11-10（上の入力）。
- 実測 2026-11-10（セル。Claude Code 2.1.322）: 別の cwd から `claude --resume 7a11eb7e-… --fork-session -p …` が A-0010 のセッションを見つけて通った（`--resume <id>` は cwd に縛られない。`--continue` だけが cwd の最新）。Codex 0.166.0 の `codex resume <id>` も cwd 不問（`--all` は picker の絞りを外すだけ）。
- 実測 2026-11-10（セル。Claude Code 2.1.322・Codex 0.166.0。`-p`／`codex exec` に合言葉の規則を置いて答えさせた）: worktree の根より上の親の `AGENTS.md`・`CLAUDE.md` は Claude Code が読み、Codex は読まない（`NONE`）。根の `AGENTS.md` は両方が読む。
- 反論（2026-11-10。別のコンテキストの読むだけのサブエージェント。証明できたもの 8 件を人に見せ、A・B を人が選んだ）: 記録の無い dir での `claude --continue`（対話は黙る・`-p` は黙って新規）と `codex exec resume --last`（黙って新規）の実測、`claude -p` 1回で `--continue` の先が替わる実測、`chiefengineer.go` の Discard と `pickup.go` の Gone の衝突、`finish.go` が lock を取らないこと、`hc run` の argv と `first_message` の形、`git clean -ndx` が `.worktrees/` を消すと言うこと。
- `hc get`（2026-11-10）: 端末は `keep: true` で起動し、argv・cwd・ラベル（`project`・`issue`・`role`）を保つ。herdr は失ったプログラムをその場で素のシェルに替え、元の command は打ち直さない（`hc run --help`）。
