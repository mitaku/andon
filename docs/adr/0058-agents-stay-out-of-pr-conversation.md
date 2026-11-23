# ADR-0058: エージェントが PR に書いたものを、オーナーの手直しの記録に入れない（仕様・設計。2026-11-23）

- 状態: **決定（2026-11-23）**。オーナーが対話の中で「決めた」と言った（[decisions/2026-11-23-adr-0058-agents-stay-out-of-pr-conversation.md](../decisions/2026-11-23-adr-0058-agents-stay-out-of-pr-conversation.md)）。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: オーナーの手直しの記録（`corrections.json`）の「PR の上の手直し」に、担当や Chief Engineer が書いたコメントと commit が混ざらないようにする。エージェントが PR のどこに書いてよいか、それをどこで守らせるか、commit の拾い方をどう直すかを決める。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。この文書で「エージェント」は、担当・レビュアー・Chief of Staff・Chief Engineer を指す。「会話欄」は、PR の会話のコメントと、レビューの本文と、インラインのレビューコメントを指す。PR の本文は会話欄に入れない。「メンテナー」は、オーナーと、repo に書き込めるほかの利用者を指す。
- [ADR-0018](0018-screen-check-tooling.md) 決めたこと 3 のうち、PR を出したあとの画面の写しの添付の打ち方を改める（決めたこと 6）。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は、エージェントのコメントと commit を、オーナーの手直しの記録から外すこと | この ADR | 生きている |
| 2 | どの役のエージェントも、PR の会話欄に書かない。書く場所は PR の本文だけにする | `package/environment.md` | 生きている |
| 3 | コメントを拾う規則は変えず、印も記録も足さない | `docs/contract.md` | 生きている |
| 4 | `gh pr comment` と `gh pr review` を、設定の deny と Codex の rules で断る | `package/settings.json`・`package/codex.rules` | 生きている |
| 5 | 決まりの文は、全部の役が読む環境メモと、担当の手引きに置く | `package/environment.md`・`package/skills/worker/SKILL.md` | 生きている |
| 6 | PR を出したあとの画面の写しは、`gh pr edit --attach` で本文に足す | `package/skills/worker/SKILL.md` | 生きている |
| 7 | 親が 2 つ以上の commit は、オーナーの commit に数えない | `docs/contract.md` | 生きている |
| 8 | Chief Engineer の課題では、オーナーの commit を数えない | `docs/contract.md` | 生きている |
| 9 | 担当の課題の commit の境目は、担当の最後の報告のままにする | `docs/contract.md` | 生きている |
| 10 | もう書いた手直しの記録は直さない | この ADR | 生きている |
| 11 | 断れない書き方で書かれたコメントは、今までどおり拾い、振り返りが読んで見分ける | `package/skills/retro/SKILL.md`・`docs/contract.md` | 生きている |

## 問題（オーナーの言葉で。言い直しの段落）

オーナーのテーマは「担当の PR のコメントを、オーナーの手直しに数えない」である。

オーナーの手直しの記録は、オーナーがエージェントの仕事を直した跡を集めたもので、振り返りがいちばん重く読む。ところが、この記録の「PR の上の手直し」には、担当や Chief Engineer が自分で PR に書いたものが入る。セルのエージェントはオーナーと同じ GitHub のアカウントで書くので、記録を集める dispatcher は、gate のコメントのほかを全部オーナーのものとして拾うからである。振り返りは、担当が書いた見送りの理由を、オーナーの指摘として読むことになる。直したいのは、エージェントが PR に書いたものがこの記録に入らず、「PR の上の手直し」をメンテナーの手直しとして読めることである。

モデル: 調べなかった（状態の動きを変えない。課題と問いの状態の移り方、状態を動かす command の順と断り方、dispatcher と gate の順は今のままである。変えるのは、エージェントの command を断る設定と、手直しの記録に入れる commit の規則と、手引きの文である）

## 読んで分かった事実（2026-11-23。main `4f8870e` と、Andon を開発しているセルの記録で実測）

1. **今の拾い方。** dispatcher は、片付いた課題ごとに 1 回だけ手直しの記録を書く。PR のコメントは、会話のコメントと、レビューの本文と、インラインのレビューコメントのうち、`andon gate: ` で始まらないものを拾う。commit は、PR の commit のうち、担当の最後の報告（`report.json` の `commit`）より後のものを拾う。報告の commit が PR に無ければ、commit は数えない（[contract.md](../contract.md) §14、`internal/dispatcher/corrections.go`）。
2. **書き手では見分けられない。** この repo の issue と PR のコメントは全部で 402 件あり、書き手は全部同じアカウントだった。GitHub の API が返すアカウントの種類も全部同じで、どのアプリを通して書いたかの欄（`performed_via_github_app`）も全部空だった。`gh` で書いたコメントと、画面で書いたコメントを、GitHub の記録で分ける方法は無い。
3. **エージェントが書いたコメントは 3 件。** 402 件のうち 398 件は gate のコメントで、残りは 4 件だった。1 件は issue の上で、3 件は PR の上である。PR の 3 件は、見送ったレビューの指摘の理由が 2 件（PR #236 と PR #474）と、画面の写しの添付が 1 件（PR #273）だった。PR #273 のコメントは、[ADR-0049](0049-owner-touches-by-kind.md) の事実 10 が担当のものと確かめている。PR #474 のコメントは、担当のセッションの記録に、報告の command と同じ呼び出しで `gh pr comment` を打った跡があった。PR #236 は、セルに課題の記録が残っておらず、誰が書いたかを確かめていない。
4. **メンテナーが PR の会話欄に書いたものは見つからなかった。** レビューの本文は、直近 100 本の PR で 0 件だった。インラインのレビューコメントは、repo の全体で 0 件だった。
5. **手直しの記録の「PR の上の手直し」は 4 件で、4 件ともエージェントのものだった。** 手直しの記録は 301 件ある。PR の上の手直しを持つものは 4 件で、2 件はコメント、2 件は commit だった。コメントの 2 件は、事実 3 の PR #273 と PR #474 である。commit の 2 件は、Chief Engineer が文書の PR（PR #479 と PR #483）に main を取り込んだ commit だった。Chief Engineer のセッションの記録に、報告を書いたあとで `git merge origin/main` を打ち、衝突を解いて commit した跡がある。どちらも親が 2 つの merge commit である。
6. **commit の 2 件の原因は、もう別に塞がれている。** この 2 本は、ADR の一覧の行が衝突したので main を取り込んだものだった（[ADR-0055](0055-adr-prs-without-conflicts.md) の事実 1 が同じ commit を数えている）。ADR-0055 の実装で、ADR の PR は一覧に触らなくなった。
7. **担当がコメントを書く用は 2 つで、どちらも本文で足りる。** 見送った指摘の理由は、PR の本文の「トレードオフ」に書く決まりがもうある（`package/skills/worker/SKILL.md` §3 の表と §4）。PR #474 の担当は、この決まりに無いコメントを足した。画面の写しは、手引きが、PR を出したあとなら `gh pr comment <n> --attach` を打てと書いている（同 §3）。この打ち方は、[ADR-0018](0018-screen-check-tooling.md) 決めたこと 3 が「`gh pr create`／`comment --attach` で PR に添付」と決めたものである。セルの `gh`（2.103.3）は `gh pr edit <n> --attach` を持つ。`gh pr edit --help` によると、PR の本文に画像を足せて、本文に `![説明](./<画像>)` と書いて渡すと、その場所の参照が添付の URL に書き換わる。本物の PR では試していない。
8. **gate は、画面の証拠を本文からも数える。** gate は、PR の本文とコメントのどちらかに GitHub の添付の URL があれば、画面の証拠があると数える（[decisions/2026-11-11-gate-screen-attachment-detection.md](../decisions/2026-11-11-gate-screen-attachment-detection.md)、`internal/gate/decide.go`）。写しを本文に移しても、gate の判定は変わらない。
9. **command が書くコメントは、エージェントの command の設定に当たらない。** gate のコメント（`internal/gate/gate.go`）と、外の issue への書き戻し（`internal/issue/writeback.go`）は、`andon` の中から `gh` を呼ぶ。設定の deny と Codex の rules が見るのは、エージェントが打つ command の文字列だけである。
10. **Claude Code と Codex の両方で断れるのは、command の頭の形だけ。** Codex の rules は token の頭で引く。Claude Code の設定の deny は途中に `*` を持つ形も書けるが、`tests/package/04-gate-settings.sh` は、頭の形でも allow の穴を塞ぐ形でもない deny を落とす。今、頭の形で断っているのは `gh pr merge` だけである（`package/settings.json`、`package/codex.rules`）。次の書き方は、頭の形では断れない: `gh api` での書き込み、PR の番号を渡した `gh issue comment`、`gh pr close --comment` と `gh pr reopen --comment`、Codex での引数を取る wrapper の形（[image.md](../image.md) の「Codex の承認と sandbox」）。
11. **手引きと検査が PR のコメントを前提にしている所は 3 つ。** `package/skills` で `gh pr comment` を打たせる行は、事実 7 の 1 行だけである。その前の行は、PR の本文と報告と「コメント」に書いた文を読み返せと書いている。`tests/package/09-screen-check.sh` は、`gh pr comment <n> --attach` の行があることを確かめている。Chief of Staff の手引きにも、Chief Engineer の手引きにも、PR にコメントを書かせる行は無い。レビュアーには、PR へコメントしないと書いてある。
12. **報告が push に付いてくるのは担当の課題で、Chief Engineer の課題では付いてこない。** マージされた PR のうち、課題の報告に PR があるものは 239 本だった。報告の commit と PR の head が違うものは 5 本で、3 本が Chief Engineer の文書の PR、1 本が担当の PR、1 本がレビュアーの課題だった。Chief Engineer の 3 本は、事実 5 の merge commit の 2 本と、報告のあとに commit を作り直して push した 1 本（PR #484）である。PR #484 は、報告の commit が PR から消えたので、commit が数えられなかった（事実 1）。担当の 1 本（PR #290）は、マージのあとに報告が書かれたものだった。担当は、push のたびに報告を書き直してから渡し直す。Chief Engineer は、報告を 1 回書いて渡し、そのあとの push では報告を書き直さない。
13. **gate の判定は、マージの条件になっていない。** gate は、`report.json` の `commit` が PR の head と違うと差し戻す（`internal/gate/gate.go`）。ただし、オーナーのマージを見て課題を片付ける settle は、報告の commit と、マージされた head を比べない（`internal/issue/settle.go`）。gate の印を main の必須の check にする決定（PR #503 の ADR）が実装されると、担当の PR は、gate が今の head を判定するまでマージできなくなる。Chief Engineer の文書の PR は gate を通らず、その決定でも報告の commit は見ない。
14. **読み手は 3 つ。** 振り返りは、束の中の手直しの記録をそのまま読み、要約には手直しがあったかどうかを載せる（`internal/retro/retro.go`）。`andon measure` は、手直しの記録が 1 件以上ある課題の数を、対抗の指標の `counter.reopened` と `counter.corrected` に数える（`internal/measure/measure.go`）。Chief of Staff は、片付いた課題の記録を wiki の元資料に写すときに、手直しの記録も写す（`package/skills/chief-of-staff/playbooks/wiki.md`）。contract.md §14 は、今も「読み手は振り返りだけ」と書いている。
15. **手直しの記録は書き直されない。** 記録のファイルがもう在れば、dispatcher は書かない（contract.md §14）。振り返りの手引きは、手直しの記録を「オーナーの手直し。いちばん重い」と書き、エージェントの書き込みが混ざりうるとは書いていない（`package/skills/retro/SKILL.md`）。

## 前提（同意済み。番号）

オーナーが 2026-11-23 に同意した。

1. 直したいのは、エージェントが PR に書いたものが、オーナーの手直しの記録に入ることである。オーナーやほかのメンテナーが GitHub で書いたコメントは、これからも手直しとして拾う。
2. 何もしなければ、振り返りがいちばん重く読む記録に誤りが残る。[ADR-0049](0049-owner-touches-by-kind.md) 決めたこと 10 が先に送った「PR の上の手直しを介在回数に足す」も、開けられないままになる。
3. 見分けは GitHub の書き手に頼らず、Andon の側の決まりと記録でつける。セルに別の GitHub のアカウントや GitHub App を持たせることは、この文書では扱わない。
4. 新しい部品は足さず、今ある拾い方と、手引きと、設定の直しで届ける。

## 検討した案

| 案 | 中身 | 大きさ | リスク | 再利用するもの | 捨てるもの |
|---|---|---|---|---|---|
| 手引きだけ直す | 見送りの理由と画面の写しを本文に書くよう、担当の手引きを直す。守らせる仕組みは足さない | 手引きの 2 行と検査の 1 行 | 見送りの理由は、決まりがあったのにコメントに書かれた（事実 7）。文だけでは同じことが起きる。commit の 2 件は残る | 本文の「トレードオフ」の決まり | 無し |
| 会話欄に書かない（選んだ） | どの役も会話欄に書かず、設定で `gh pr comment` と `gh pr review` を断る。commit は、merge commit と、Chief Engineer の課題のものを数えない | 設定の 2 行、rules の 4 つ、手引きと環境メモの 5 行、拾い方の 2 か所、文書の 4 か所、検査の 5 か所 | 断れない書き方が残る（事実 10）。Chief of Staff が、オーナーに頼まれたコメントを書けなくなる | `gh pr merge` を断っている形、今の拾う規則、本文の決まり | 手引きの `gh pr comment --attach` |
| 印を付けて書ける口を残す | エージェントは Andon の command を通してだけコメントを書く。command がコメントの頭に印を付け、拾う側は印のあるものを除く | 上の案に、command を 1 つと、拾う規則の 1 行と、約束の文を足す | 使われていない口を作る。エージェントが書いた 3 件はどれも本文で足りた（事実 7）。オーナーの言葉を Chief of Staff が代わりに書くと、エージェントの書き込みとして残る | gate のコメントの頭の印の考え方 | 同上 |
| Andon の側に、書いたコメントの記録を持つ | エージェントが書いたコメントの番号を kanban に残し、拾う側が引く | command を 1 つと、新しい記録と schema | 印の案と同じで、記録と約束が 1 つ増える（前提 4 に反する） | 無し | 同上 |
| セルの GitHub の身元を分ける | セルに別のアカウントか GitHub App を持たせ、書き手で見分ける | セルの立て方、認証、マージの主体、ruleset | 前提 3 でオーナーが範囲の外にした。秘密の置き場と、案件ごとの設定が増える | 無し | 書き手を見ない今の作り |

選んだのは「会話欄に書かない」である。オーナーが入口で、範囲をコメントと commit にすること（決めたこと 1）と、どの役も会話欄に書かないこと（決めたこと 2）を選んだ。会話欄にあるのは gate のコメントとメンテナーのコメントだけ、と一文で言える形になり、コメントを拾う規則を変えずに済む。印を付ける口は、外のメンテナーのレビューに担当が PR の上で返事をする場面が出てきたら、あとから足せる。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-23 範囲は、エージェントが書いたコメントと、エージェントが積んだ commit を、オーナーの手直しの記録から外すことにする。**（オーナーが決めた）
   手直しの記録の「PR の上の手直し」の 4 件は、2 件がコメント、2 件が commit だった（事実 5）。コメントだけを直すと、同じ種類の誤りが半分残る。
   捨てた案: コメントだけを直す（commit の誤りが残る）。担当の手引きだけを直す（守らせる仕組みが無い）。PR の上の手直しを介在回数に足す所まで決める（数え方の決定が増える。やらないこと）。

2. **2026-11-23 どの役のエージェントも、PR の会話欄に書かない。書く場所は PR の本文だけにする。**（オーナーが決めた）
   担当も、レビュアーも、Chief of Staff も、Chief Engineer も、PR の会話のコメントとレビューを書かない。エージェントが PR に伝えたいことは、PR の本文に書く。オーナーが PR にコメントを残したいときは、オーナーが自分で GitHub に書く。
   理由: エージェントが会話欄に書いた 3 件は、どれも本文で足りた（事実 3・7）。会話欄に書くのが gate とメンテナーだけなら、拾う側は今の規則のままで、メンテナーのコメントだけを拾える。オーナーの言葉を Chief of Staff が代わりに書くと、エージェントの書き込みとして残る。オーナーが自分で書けば、書いたとおりに手直しとして記録に入る。
   捨てた案: 印を付けて書ける口を残す（検討した案の 3 行目）。担当と Chief Engineer だけ断り、Chief of Staff は書けるままにする（役で分けて断る仕組みが要り、Chief of Staff が書いたコメントがオーナーの手直しとして記録に入る）。
   細部:
   - gate のコメントと、外の issue への書き戻しは、command が書くもので、この決まりの外である（事実 9）。今までどおり書く。
   - PR を閉じるときと開け直すときに添えるコメントも、会話欄に入る。エージェントは添えない。
   - Chief of Staff がオーナーにコメントを頼まれたら、文案を返して、オーナーに書いてもらう。
   - PR の本文を書き直すこと（`gh pr edit`）は、どの役もしてよい。

3. **2026-11-23 コメントを拾う規則は変えず、印も記録も足さない。**（Chief Engineer が決めた）
   contract.md §14 の、コメントを拾う規則はそのままにする。`schema/corrections.schema.json` も変えない。
   理由: 決めたこと 2 が守られていれば、会話欄にあるのは gate のコメントとメンテナーのコメントだけで、今の規則がメンテナーのコメントだけを拾う。コメントについては、メンテナーのものを落とす向きの変更が無い。
   捨てた案: コメントの時刻で分ける（オーナーのコメントのあとに担当が直して報告を書き直すので、報告の前か後かでは分かれない）。書き手やアプリの欄で分ける（事実 2）。

4. **2026-11-23 `gh pr comment` と `gh pr review` を、設定の deny と Codex の rules で断る。**（Chief Engineer が決めた）
   `package/settings.json` の `permissions.deny` に `Bash(gh pr comment:*)` と `Bash(gh pr review:*)` を足す。`package/codex.rules` に、同じ 2 つの頭を `forbidden` にする rule を足す。形は `gh pr merge` の rule と同じで、直接の形と、引数を取らない wrapper の形の 2 つずつにする。
   理由: 決まりを文だけに置くと守られないことがある（事実 7）。`gh pr merge` を断っている形がもうあり、設定と rules の一覧がずれると落ちる検査もある（`tests/package/04-gate-settings.sh`）。レビューの本文とインラインのコメントも手直しとして拾われるので、`gh pr review` も断る。エージェントはオーナーと同じアカウントで書くので、自分の PR を承認することはもともとできない。
   捨てた案:
   - `gh issue comment` も断る。PR の番号を渡せば PR に書けるが、issue に書く用がある。Chief of Staff は、オーナーの許しを取ったうえで、外のトラッカーにコメントを書くことがある（`package/skills/chief-of-staff/playbooks/cut.md`）。command の頭では issue と PR を分けられない。
   - `gh api` を断る。読むために広く使っていて、頭の形では書き込みだけを分けられない。
   - `gh pr close --comment` と `gh pr reopen --comment` を断る。頭の形では、コメントを添える形だけを分けられない。Chief of Staff は、コメントを添えない `gh pr close` を打つことがある（`package/skills/chief-of-staff/playbooks/gate.md`）。途中に `*` を持つ deny は Codex に引けず、設定と rules の一覧を突き合わせる検査の分け方にも入らない（事実 10）。
   - Bash を断るフック（`package/hooks/pre-tool-use`）で断り、どこに書けばよいかを理由の文で返す。Codex には効かず、守る場所が 2 つになる。断られたエージェントが別の書き方で会話欄に書いた記録が出たら、そのときに足す。
   細部:
   - Codex の rules の理由の文には、書く場所を入れる（例「PR の会話欄には書かない。伝えることは PR の本文に書く」）。
   - [image.md](../image.md) の「Codex の承認と sandbox」の「今は `gh pr merge`」を、3 つの頭の一覧に直す。拒めない形の説明は、今の文のままで 3 つに当てはまる。
   - `tests/image.sh` の、rules に当たる形と当たらない形の検査に、`gh pr comment` と `gh pr review` の形を足す。当たらない側には `gh pr view`・`gh pr edit`・`gh pr create` を入れる。試しの rules を Codex の execpolicy（0.166.0）に引かせると、この分かれ方になった。

5. **2026-11-23 決まりの文は、全部の役が読む環境メモと、担当の手引きに置く。**（Chief Engineer が決めた）
   `package/environment.md` の「気をつけること」に 1 行足す。中身は、PR の会話欄に書かないこと、伝えることは PR の本文に書くこと、会話欄はメンテナーが書く場所で、そこに書かれたものはオーナーの手直しとして集められること、オーナーに頼まれたら文案を返すこと、である。`package/skills/worker/SKILL.md` §3 にも、同じ決まりを 1 行で書く。§3 の「PR 本文・報告・コメントに書いた文を読み返す」の行から「コメント」を外す。§4 の見送りの行には、PR のコメントには書かないと添える。
   理由: 環境メモは、どの役も毎セッション読む。決まりは役を問わないので、役ごとの手引きに 1 つずつ書くより、1 か所に置く。担当は PR を書く当人なので、PR の書き方の節にも置く。
   捨てた案: Chief of Staff と Chief Engineer の手引きにも書く（環境メモで足りる。Chief Engineer の手引きを変えると、design.md §1.8 も同じ PR で直すことになる）。

6. **2026-11-23 PR を出したあとの画面の写しは、`gh pr edit --attach` で本文に足す。**（Chief Engineer が決めた。オーナーが決めた決めたこと 2 の帰結で、ADR-0018 決めたこと 3 の打ち方を改める）
   担当の手引き §3 の「PR を出したあとなら `gh pr comment <n> --attach <画像>`」を、`gh pr edit <n> --attach <画像>` に直す。写しは、本文の「検証結果と確かめ方」に置く。
   理由: 手引きがコメントを打たせていた（事実 7）。本文の表は、画面の添付を「検証結果と確かめ方」に書くと決めている。gate は本文の添付も数える（事実 8）。
   細部:
   - 旗だけで足すと、画像は本文の末尾に付き、メモの行の後ろになる。本文の置きたい所に `![説明](./<画像>)` と書いて、`--body-file` と `--attach` を一緒に渡す形を手引きに書く（事実 7）。この打ち方は `--help` で確かめただけなので、手引きを直す担当が、自分の PR で 1 回打って確かめる（受け入れ条件 9）。
   - ADR-0018 には、決めたこと 3 の打ち方をこの文書が改めたと、「決めたこと」の下に 1 件足す。証拠が PR への画像の添付であることと、`report.json` に URL と command を書くことは変えない。
   - `tests/package/09-screen-check.sh` は、手引きに `gh pr comment <n> --attach` があることを確かめている。`gh pr edit <n> --attach` があることと、`package/skills` に `gh pr comment` を打たせる行が無いことを確かめる形に直す。

7. **2026-11-23 親が 2 つ以上の commit は、オーナーの commit に数えない。**（Chief Engineer が決めた）
   手直しの記録の `human_commits` に、merge commit を入れない。担当の最後の報告より後の commit のうち、親が 2 つ以上と読めたものを外す。親の数が読めない commit は、今までどおり入れる。
   理由: ほかの branch を取り込む commit は、誰が積んでも、エージェントの仕事を直した跡ではない。記録にあった 2 件は、どちらも Chief Engineer が main を取り込んだ merge commit だった（事実 5）。オーナーが GitHub の画面で PR の branch を最新にしたときの commit も、同じ理由で外れる。commit については、拾う数を減らす向きの変更である。contract.md §14 の「多めに拾い、少なく拾わない」に、merge commit を除くと添える。
   捨てた案:
   - commit の記録をやめる。オーナーが担当の branch に直しを積む使い方を、拾えなくなる。
   - セルの git の commit に、セルで積んだと分かる印を付ける（committer の名前など）。印は commit と一緒に外へ出て、マージの方式によっては案件の repo の履歴に残る。オーナーの commit は記録に 1 件も無く（事実 5）、今そこまでして見分ける理由が無い。
   - エージェントが push した commit を、Andon の側の記録に残す（新しい記録が要る。前提 4）。
   細部:
   - 親の数は、今も commit ごとに打っている `gh api repos/<owner>/<repo>/commits/<sha>` の `parents` で読む。`gh` の呼び出しは増えない。
   - 報告より後の commit が merge commit だけで、コメントも無ければ、`pr` の記録を書かない。今の「どちらも無ければ書かない」と同じ扱いである。
   - contract.md §14 の表の `pr` の行の拾う規則に、この条件を足す。[design.md](../design.md) §1.9 の (b) の表の 2 の行にも、同じ条件を添える。

8. **2026-11-23 Chief Engineer の課題では、オーナーの commit を数えない。**（Chief Engineer が決めた）
   プロファイルが `chief-engineer` の課題では、`human_commits` をいつも空にする。コメントは、ほかの課題と同じに拾う。
   理由: 「最後の報告より後の commit はオーナーのもの」という境目は、エージェントが push のたびに報告を書き直すときにだけ当てにできる。Chief Engineer は、報告を 1 回書いて渡し、そのあとの push では書き直さない。文書の PR 30 本ほどのうち 3 本で、報告のあとに Chief Engineer が push していた（事実 12）。merge commit でない commit を足す形で push すれば、決めたこと 7 があっても、オーナーの commit として記録に入る。
   捨てた案:
   - Chief Engineer の手引きに「報告のあとに push したら報告を書き直す」と足す。文だけの決まりで、書き直すたびに Chief of Staff へ同じ知らせがもう一度届く。
   - gate の記録がある課題でだけ commit を数える。gate が判定する前にマージされた担当の PR で、オーナーの commit を落とす。
   細部:
   - Chief Engineer の課題かどうかは、dispatcher が今も使っている見分け（`internal/dispatcher/dispatcher.go` の `isEngineer`）で見る。
   - オーナーが Chief Engineer の文書の PR に自分で commit を積んでも、手直しの記録には入らない。オーナーが文書を直させるのは対話の中で、対話の中の直しは、もともと手直しの記録が拾わない（design.md §1.9 の (b) の表の 5）。
   - contract.md §14 の `pr` の行に、この条件を足す。

9. **2026-11-23 担当の課題の commit の境目は、担当の最後の報告のままにする。**（Chief Engineer が決めた）
   担当の課題では、「`report.json` の `commit` より後の commit をオーナーのものと数える」という規則を変えない。
   理由: 担当は、push のたびに報告を書き直してから渡し直す。マージされた 239 本で、担当が報告のあとに commit を足したままマージされた例は無かった（事実 12）。gate の印を main の必須の check にする決定が実装されると、担当の PR は、gate が今の head を判定するまでマージできなくなる。gate は報告の commit と head の一致を求めるので、境目は仕組みでも守られる（事実 13）。それまでの間は、担当が手順を守ることに頼る。
   捨てた案: 担当の Stop のフックで、head と報告の commit の食い違いを止める（課題の一生の守りを 1 つ増やすことになる。上の決定の実装で要らなくなる）。
   細部: 今の規則には、数え落とす道が 2 つある。1 つは、オーナーが commit を積んだあとに、担当がその上で報告を書き直したときである。オーナーの commit は報告より前になり、記録に入らない。もう 1 つは、報告の commit が PR から消えたときである。報告のあとに履歴が書き換えられると、後ろの commit を全部数えない（事実 1）。どちらも前からある道で、この文書では直さない（やらないこと）。

10. **2026-11-23 もう書いた手直しの記録は直さない。**（Chief Engineer が決めた）
    事実 5 の 4 件は、記録に残したままにする。
    理由: 手直しの記録は 1 回だけ書く決まりである（事実 15）。振り返りは、前の回より後に片付いた課題を読むので、4 件をもう一度重く読むことは無い。`andon measure` は、この 4 件を含む期間を数えると、手直しのあった課題を 3 件多く出す。残りの 1 件は、ほかの種類の手直しも持つので、どのみち数えられる。
    捨てた案: 4 件のファイルを消して集め直す（dispatcher が集めるのは `done` の行を拾ったときだけで、消しても集め直されない。集め直す口を足すほどの数でない）。

11. **2026-11-23 断れない書き方で書かれたコメントは、今までどおり拾い、振り返りが読んで見分ける。**（Chief Engineer が決めた）
    事実 10 の書き方でエージェントが会話欄に書いたコメントは、手直しの記録に入る。振り返りの手引きに、それを見分ける 1 行を足す。
    理由: 設定と rules は守りの 1 枚で、全部の書き方は断れない（事実 10）。拾う側で落とすと、メンテナーのコメントを落とす向きの誤りが増える。振り返りは記録の中身を読むので、エージェントの文面は見分けられる。今回の 2 件も、振り返りが中身を読んで見つけた。
    細部:
    - `package/skills/retro/SKILL.md` の、手直しの記録の読み方の行に、次の中身を足す。エージェントは PR の会話欄に書かない決まりである。`pr` の記録のコメントがエージェントの文面なら、オーナーの手直しとして読まず、決まりが破られた跡として拾う。
    - その跡が、決めたこと 4 の捨てた案（フックで断る）や、印を付ける口を足すかを決める材料になる。
    - contract.md §14 の「読み手は振り返りだけ」を、事実 14 の 3 つに直す。

## やらないこと（範囲の外。理由）

- **PR の上の手直しを、オーナーの介在回数に足すこと。** [ADR-0049](0049-owner-touches-by-kind.md) 決めたこと 10 は、手直しの記録が担当のコメントを除けるようになったら、種類を 1 つ足すと書いている。入口でオーナーが、この文書の範囲に入れなかった。足すかどうかは、価値の仮説の見る日に、記録にエージェントのものが混ざっていないと確かめてから、ADR-0049 の小さな改めとして決める。
- **セルの GitHub の身元を分けること。** 前提 3。
- **エージェントがコメントを書く command を足すこと。** 使う場面が、今は無い（決めたこと 2）。
- **commit の数え落としの 2 つの道を直すこと。** 決めたこと 9 の細部。直すには、commit を誰が積んだかの印が要る。オーナーの commit が記録に現れてから決める。
- **settle が、報告の commit とマージされた head を比べること。** 事実 13。課題の片付け方に触れる。gate の印を必須の check にする決定の実装で、担当の PR の窓は閉じる。
- **gate が画面の証拠をコメントからも数えるのを変えること。** オーナーが画面で写しをコメントに上げることがあり、変える理由が無い（事実 8）。
- **手直しの記録の、ほかの種類の拾い方を変えること。** 問いの答えと、指摘の覆しは、この文書の範囲の外である。
- **PR #236 のコメントを確かめること。** セルに記録が残っていない（事実 3）。手直しの記録にも入っていない。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は今あるものを使う。設定と rules と手引きの文は `bash tests/package.sh`、Codex の rules が実際に断るかは CI の `tests/image.sh`、拾い方は `go test ./internal/dispatcher`、command の通しは `tests/andon-dispatcher.sh` である。証拠は、走らせた command と、その出力である。

| # | 条件 | 証拠の形 |
|---|---|---|
| 1 | `package/settings.json` の deny に `Bash(gh pr comment:*)` と `Bash(gh pr review:*)` がある。`package/codex.rules` の `forbidden` の頭の一覧が、設定の deny の頭の一覧と同じである | `bash tests/package.sh` の出力のうち `04-gate-settings.sh` の行 |
| 2 | Codex の execpolicy が、`gh pr comment 1 --body x`・`gh pr review 1 --comment --body x` と、その `env`・`command` の形を `forbidden` と判定する。`gh pr view 1`・`gh pr edit 1 --attach a.png`・`gh pr create --title t` は rules に当たらない | CI の tests の job の `tests/image.sh` の出力（docker のある runner で回る） |
| 3 | 担当の手引き §3 が、PR を出したあとの添付を `gh pr edit <n> --attach` と書く。`package/skills` の下に、`gh pr comment` を打たせる行が無い。無いことを見る検査は、同じ所で、直す前の行に当たることも確かめる。`package/environment.md` に、PR の会話欄に書かない決まりの行がある。振り返りの手引きに、決めたこと 11 の行がある | `bash tests/package.sh` の出力のうち `09-screen-check.sh` の行と、振り返りの手引きを見る検査の行 |
| 4 | 報告より後の commit に、親が 2 つのものと 1 つのものが 1 本ずつあるとき、`human_commits` に入るのは親が 1 つのものだけである。親の数が読めない commit は入る。報告より後の commit が親 2 つのものだけで、コメントも無いとき、`pr` の記録を書かない | `go test ./internal/dispatcher` の出力 |
| 5 | プロファイルが `chief-engineer` の課題では、報告より後に親が 1 つの commit があっても `human_commits` が空である。同じ課題の PR に gate の頭で始まらないコメントがあれば、`comments` には入る。プロファイルが担当の課題では、同じ commit が `human_commits` に入る | `go test ./internal/dispatcher` の出力 |
| 6 | gate の頭で始まらない会話のコメントと、レビューの本文と、インラインのレビューコメントは、今までどおり `comments` に入る | `go test ./internal/dispatcher` の出力（今ある検査が通る） |
| 7 | 偽の `gh` が、報告より後の 2 本の commit に、親 2 つと親 1 つを返す場面で、片付いた課題の `corrections.json` の `human_commits` が、親 1 つの commit の 1 本だけである | `tests/run --changed` の表の `andon-dispatcher.sh` の行 |
| 8 | contract.md §14 の例が、今の schema に通る（schema は変えない） | `tests/run --changed` の表の `schema.sh` の行 |
| 9 | 手引きを直す担当が、自分の PR に、決めたこと 6 の打ち方で画像を 1 枚足す。画像が本文の「検証結果と確かめ方」に載り、PR の会話欄にコメントが増えない。確かめたら、画像は本文から外してよい | 担当の報告に、打った command と、`gh pr view <n> --json body,comments` の出力の要点。オーナーが PR で見る（`manual`） |
| 10 | 実装をセルに当てたあと、Claude Code で動く担当の端末から `gh pr comment --help` を打つと、設定の deny で断られる。`gh pr edit --help` は、deny では断られない。どちらも GitHub に何も書かない | 検証の課題の報告に、打った command と、断られた出力の写し |

## 価値の仮説

1. **誰の何が変わるか。** 振り返りが読む手直しの記録から、エージェントの書き込みが消える。振り返りは、PR の上の手直しを、メンテナーの手直しとして読める。オーナーと外の読み手には、PR の会話欄が gate のコメントとメンテナーのコメントだけになる。
2. **見る信号。** 信号の記録は無い。観察の課題が、セルの kanban の手直しの記録と、GitHub の PR のコメントを読んで数える（読むだけ）。数えるのは、実装がセルに当たった日より後に片付いた課題の `corrections.json` のうち `kind` が `pr` の記録と、同じ日より後に書かれた、`andon gate: ` で始まらない PR のコメントである。
3. **見る日。** 2026-12-03。実装がセルに当たってから、7 日ぶんの PR が溜まる日である。今までの割合は、手直しの記録 301 件に 4 件だった。
4. **誰が要るか。** オーナーの手は要らない。観察の課題の担当が数える。
5. **効いたとどう分かるか。** 見る日までの記録で、`pr` の記録のうち、中身を読んでエージェントが書いたと分かるものが 0 件である。同じ期間の PR の会話欄に、エージェントが書いたコメントが 0 件である。画面の確かめが要る PR があれば、写しが本文に載っている。
6. **何があれば止めるか。** エージェントが断れない書き方で会話欄に書いたコメントが、見る日までに 2 件以上ある。そのときは、フックで断って理由を返す案か、印を付けて書ける口の案を検討し直す。担当が、写しを本文に付けられずに止まったという報告が出たときも、手引きの書き方を見直す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **エージェントが PR の会話欄に書く口を閉じる。** 決めたこと 2・4・5・6・11 の手引きの分。`package/settings.json`、`package/codex.rules`、`package/environment.md`、`package/skills/worker/SKILL.md`、`package/skills/retro/SKILL.md`、`docs/image.md`、`tests/package/04-gate-settings.sh`、`tests/package/09-screen-check.sh`、振り返りの手引きを見る検査、`tests/image.sh`、この ADR と ADR-0018 の索引。受け入れ条件 1・2・3・9・10。依存は無い。`package/` を変えるので、マージしてセルに当てたあとに、検証の課題で受け入れ条件 10 を動かす。
2. **merge commit と、Chief Engineer の課題の commit を、オーナーの commit から外す。** 決めたこと 7・8 と、決めたこと 11 の contract.md の分。`internal/dispatcher/corrections.go` とその検査、`tests/andon-dispatcher.sh`、`docs/contract.md` §14、`docs/design.md` §1.9 の (b) の表、この ADR の索引。受け入れ条件 4・5・6・7・8。依存は無い。1 と並べて進められる。

## 開いている問い（残った問い。決断の種類つき）

無し。

## 出典（repo の中の文書・外の URL・オーナーの発言の日付だけ）

- オーナーの発言 2026-11-23（入口の言い直しと前提 4 つへの同意、範囲の答え、会話欄の扱いの答え。出口の「決めた」）。
- [contract.md](../contract.md) §9（フック）・§14（オーナーの手直し）。
- [design.md](../design.md) §1.9 の (b)（オーナーの手直しが起きる所）。
- [image.md](../image.md) の「Codex の承認と sandbox」（rules が断れる形と断れない形）。
- [decisions/2026-11-09-corrections-json.md](../decisions/2026-11-09-corrections-json.md)（手直しの記録が、同じアカウントのコメントと commit をどう拾うか）。
- [decisions/2026-11-11-gate-screen-attachment-detection.md](../decisions/2026-11-11-gate-screen-attachment-detection.md)（gate が画面の証拠を本文とコメントから数える）。
- [ADR-0018](0018-screen-check-tooling.md)（決めたこと 3。画面の証拠の添付の打ち方）・[ADR-0026](0026-pr-rules-for-outside-readers.md)（PR の本文の形）・[ADR-0049](0049-owner-touches-by-kind.md)（事実 10、決めたこと 10、やらないことの 3 つ目）・[ADR-0055](0055-adr-prs-without-conflicts.md)（事実 1）。
- PR #503 の ADR（gate の印を main の必須の check にする。この文書を書いた時点ではマージの前で、事実 13 と決めたこと 9 が指す）。
- 実測（2026-11-23）: `gh api repos/<owner>/<repo>/issues/comments` と `pulls/comments` の全件、直近 100 本の PR のレビュー、`gh api repos/<owner>/<repo>/commits/<sha>` の `parents`、マージされた PR の head と課題の報告の commit の突き合わせ、セルの手直しの記録 301 件の集計、担当と Chief Engineer のセッションの記録、`gh pr comment`・`gh pr edit`・`gh pr create`・`gh pr close` の `--help`（gh 2.103.3）。別のコンテキストの反論（読むだけ）で、事実 10〜14 と決めたこと 3・4・6・7・9・10・11 と受け入れ条件 4・7・10 を直し、決めたこと 8 を足した。
