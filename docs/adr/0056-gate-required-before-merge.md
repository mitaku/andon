# ADR-0056: gate が判定するまで、PR をマージできないようにする（仕様・設計。2026-11-23）

- 状態: **決定（2026-11-23）**。オーナーが対話の中で「決めた」と言った。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: Andon の手続き（独立レビュー・指摘の振り分け・gate の判定）が終わる前の PR と、Chief of Staff が止めた PR を、GitHub の上でマージできないようにすること。gate の印（commit status `andon/gate`）を main の必須の check にし、判定ごとの印と、gate が見る順を決め直す。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。印の文を外の読み手に向けて書くと決めたのは [ADR-0026](0026-pr-rules-for-outside-readers.md) 決めたこと 9。
- この文書で「印」は、gate が PR の head の commit に付ける commit status（名前は `andon/gate`）を指す。「緑」は `success`、「赤」は `failure`、「黄」は `pending`。「オーナー待ち」は gate の判定 `needs_human`、「差し戻し」は判定 `fix`、「通した」は判定 `approve`。「渡す」は、gate がその commit を通したかオーナー待ちと判定して、マージしてよい状態にすること。「止める」は、Chief of Staff が、ある課題の PR を、外すまで渡さないようにすること。
- [ADR-0054](0054-gate-status-by-decision.md) 決めたこと 2・3 を覆す。同 7 は、開いている PR のオーナー待ちの印について覆す。同 9 が「別のテーマにする」とした話がこの文書である。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は、担当の PR と、Chief of Staff が止めた PR | この ADR | 生きている |
| 2 | `andon/gate` を main の必須の check にする。印は 1 つのまま | この ADR | まだ実装していない |
| 3 | 通したときとオーナー待ちは緑、差し戻しは赤、止めている間は黄、判定の前は印を書かない | この ADR | まだ実装していない |
| 4 | gate は、オーナーがマージする変更かどうかを、検査とレビューのあとに見る | この ADR | まだ実装していない |
| 5 | dispatcher は、PR を読むたびに印をあるべき形に揃える | この ADR | まだ実装していない |
| 6 | Chief Engineer の文書の PR は、渡されたときに dispatcher が緑を書く | この ADR | まだ実装していない |
| 7 | 止める口は、予約キー `hold` と、止める command・外す command にする | `docs/contract.md` §2・§6・`internal/issue/hold.go`・`internal/dispatcher/hold.go` | 生きている |
| 8 | 止める command は、マージと同じ鍵を取り、緑の印を黄に書き換えてから返る | `docs/contract.md` §7・`internal/issue/hold.go` | 生きている |
| 9 | 止めている間、gate は回らない。判定のあとに止められたら、判定を捨てる | `docs/contract.md` §6・§7・`internal/gate/gate.go`・`internal/issue/merge.go` | 生きている |
| 10 | 担当が `hold` を消しても、止めは外れない | `docs/contract.md` §2・`internal/ledger/ledger.go` | 生きている |
| 11 | オーナー待ちの一覧は、止めた PR をマージの依頼に出さず、止めている問いの後ろに数える | `docs/contract.md` §25・`internal/waiting/waiting.go` | 生きている |
| 12 | dispatcher のマージが必須の check で断られたら、次の巡で打ち直す | この ADR | まだ実装していない |
| 13 | gate は、衝突を `mergeable` の欄でも読む | この ADR | まだ実装していない |
| 14 | 抜け道は作らない。ruleset の越えてよい相手は空のままにする | この ADR | まだ実装していない |
| 15 | repo の GitHub の auto-merge を切る | この ADR | まだ実装していない |
| 16 | 切り替えは、実装を当て、印が揃ったのを確かめてから、オーナーが設定を変える | この ADR | まだ実装していない |
| 17 | モデルに、印と止めの状態と、規則を 3 本足す | `model/IssueLife.tla`・`model/IssueRules.tla`・`model/README.md` | 生きている |
| 18 | 印の決まりの正は `docs/contract.md`。設定と外し方は `docs/operate.md` に書く | この ADR | まだ実装していない |

## 問題（オーナーの言葉で。言い直しの段落）

Andon の手続きは、独立レビュー、指摘の振り分け、gate の判定の順に進む。ところが GitHub の上では、この手続きが終わる前の PR も、終わった PR と同じようにマージできる。PR を開いても、手続きの途中だと分かる印は無い。そのため、レビューの途中の PR や、担当が直している途中の PR が main に入ることがある。欲しいのは、手続きが終わるまで、その PR を GitHub の上でマージできないことである。誰がマージを押したかは問わない。押せないようにする。

確かめた事実（2026-11-23。数えたのは 2026-11-19 から 2026-11-22 の 4 日。日付は UTC）:

1. **判定の前のマージは、4 日で 21 本あった。** dispatcher が自分でマージした PR は 97 本で、どれも gate が通した commit だった。dispatcher でない者がマージした PR は 102 本あった。そのうち 81 本は、gate が同じ commit をオーナー待ち（76 本）か通した（5 本）と判定したあとのマージだった。残りの 21 本は、マージされた commit を gate が判定していなかった。
2. **21 本の内訳。** 独立レビューが走っている最中のマージが 10 本。レビューは終わっていたが、担当が指摘を振り分けて gate に渡す前のマージが 8 本。レビューを頼む前のマージが 2 本。レビューの無い課題で、担当が渡す 16 秒前のマージが 1 本。21 本のうち 9 本は、gate が前の commit をオーナー待ちと判定したあとに、担当が push した新しい commit のマージだった。21 本は全部が担当の PR だった。
3. **Chief of Staff が待つよう頼んでいる間のマージが 1 本あった。** PR #491 は Chief Engineer の文書の PR で、Chief of Staff が別の問いの答えを待つよう頼んでいる間にマージされた。Chief Engineer の文書の PR は gate を通らないので、止める手段は頼む文だけだった。
4. **Chief Engineer が渡す前にマージされた文書の PR は 0 本。** 同じ期間の文書の PR 30 本のうちマージされた 29 本は、どれも Chief Engineer が Chief of Staff に渡したあとのマージだった。
5. **判定の前の PR に、印は無い。** gate の印は、gate が判定した commit にだけ付く。判定の前の commit には `andon/gate` の行が無く、CI が通っていれば PR の印は緑である。印の一覧に無い行は、PR を開いても気づけない。
6. **今の印は、マージを止めない。** main の ruleset の必須の check は `tests` と `go` の 2 つで、`andon/gate` は入っていない。gate が差し戻して赤い印を付けた PR も、GitHub の上ではマージできる。
7. **gate のオーナー待ちは、検査もレビューも見ていない。** gate の決定式（`internal/gate/decide.go` の `Decide`）は、変更が委譲の範囲の外（オーナーがマージする変更）かどうかを、検査・レビュー・受け入れ条件より先に見て、オーナー待ちを返す。gate の記録にあるオーナー待ち 107 件のうち、理由が委譲の外だけのものは 96 件あった。その 96 件のうち、判定のとき CI がまだ走っていたものが 29 件、レビューの結果が無かったものが 10 件、Must fix の指摘が残っていたものが 3 件あった。重なりを除くと 35 件である。理由の文には委譲のことしか出ないので、オーナーには残った指摘が見えない。Chief of Staff の手引き（`playbooks/gate.md`）は「理由が委譲だけなら照合は通っている」と書いていて、実際と違う。
8. **GitHub は、必須の check が報告されていない PR のマージを断る。** 必須の check は、PR の最新の commit で通っていなければならない。前の commit の印は数えない。報告が無い間、マージの欄は「Waiting for status to be reported」と出して止まる。draft の PR もマージできないが、誰でも 1 押しで外せる。
9. **commit status は、push できる者なら誰でも書ける。** ruleset は、印を書いてよい出どころを GitHub App に限れるが、ユーザーの token に限る手段は無い。このセルの GitHub の書き手は 1 つのアカウントである。つまり必須の check は、うっかりのマージを止めるが、わざと印を書く者は止めない。
10. **`gh pr checks` は、報告されていない必須の check を待たない。** `gh` は報告済みの check だけを一覧にする。担当が push のあとに打つ `gh pr checks --watch` は、`andon/gate` が必須になっても、CI が終われば終わる。確かめたのは `gh` のソースと、classic の branch protection を使う公開の repo である。ruleset の repo では確かめていない。
11. **`gh pr merge` は、手前で断ることがある。** `gh` は、PR の `mergeStateStatus` が `BLOCKED` だと、マージの要求を送らずに「the base branch policy prohibits the merge」と出して終わる。印を書いた直後は、GitHub の側の計算が追いつかず `BLOCKED` のままのことがある、という報告が GitHub の外に複数ある。GitHub の文書に、この遅れの記述は無い。
12. **gate は、自分の印を検査の結果に数えない。** `internal/gate/decide.go` の `SummarizeChecks` は `andon/gate` を外して読む。必須にしても、gate が自分の印を待って止まることは無い。
13. **dispatcher のマージは、1 つの鍵の中で進む。** `andon issue merge` は、kanban に 1 つの鍵（`<slug>.issue.lock`）を持ったまま、gate を回し、マージの直前に読み直し、`gh pr merge` を打つ。gate が自分で取る鍵（`.gate.lock`）は、判定が終わると放される。
14. **この repo は、GitHub の auto-merge が有効である。** 必須の check を待つ PR には、マージの代わりに auto-merge を予約するボタンが出る。予約すると、印が緑になったときに GitHub がマージする。dispatcher は GitHub の auto-merge を使っていない（自分で `gh pr merge` を打つ）。
15. **オーナー待ちの一覧は、gate の記録と問いから作る。** `andon waiting` は、gate が通した PR とオーナー待ちの問いを、マージの依頼として並べる。Chief Engineer の課題は、`review` で PR があればマージの依頼に数える。並べたものが無い動いている課題は、働いている枠に数える。
16. **Andon の外の PR は 0 本。** 2026-11-19 からマージされた 233 本は、全部が Andon の担当か Chief Engineer の branch だった。

モデル: 調べた（規則 15 本・見つけた破れ 3 件）。破れは 3 件とも捨てた形で出た。選んだ案では 0 件だった。

## 前提（同意済み。番号）

1. 困りごとは、手続きが終わる前の PR が、終わった PR と見分けがつかず、止めるものも無いことである。
2. 何もしないと、同じことが続く。4 日で 21 本あった。
3. 誰がマージを押したかは問わない。GitHub の書き手は 1 つのアカウントで、記録からは見分けられない。
4. 頼む文や手順では止まらない。止めるのは GitHub の側にする。
5. GitHub に既にある仕組み（必須の check と ruleset）で止める。Andon が GitHub App や常駐の部品を新しく持つことはしない。
6. ruleset を変えるのはオーナーである。Andon は GitHub の設定を書き換えない。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-23 範囲は、担当の PR と、Chief of Staff が止めた PR にする。**（オーナーが決めた）
   gate が今の commit を判定するまで、担当の PR をマージできないようにする。Chief of Staff が理由つきで止めた PR も、止めている間はマージできないようにする。
   理由: 21 本は全部が担当の PR だった。PR #491 の形は、担当の PR だけを止めても残る。
   捨てた案: 担当の PR だけにする（PR #491 の形が、頼む文だけのまま残る）。Andon の PR を全部にし、新しく作るプロジェクトの既定にもする（0 本の形まで作り込むことになる）。止めずに印だけ付ける（押せば今までどおり入る）。

2. **2026-11-23 `andon/gate` を main の必須の check にする。印は 1 つのままにする。**（オーナーが決めた）
   ruleset の必須の check に `andon/gate` を足す。gate が今の commit に緑の印を書くまで、GitHub がマージを断る。担当が新しい commit を push すると、その commit には印が無いので、gate が判定し直すまでまた断られる。
   理由: GitHub の側で断るので、誰が押しても、画面からでも API からでも同じに止まる。push のあとの隙間が無い。
   払うもの: オーナーも、判定が出るまで画面からマージできない。判定の前の PR は、PR の一覧では見分けがつかない（CI が通っていれば緑）。分かるのは PR を開いたときのマージの欄である。事実 9 のとおり、わざと印を書く者は止めない。
   捨てた案: 新しい名前の印を足してそちらを必須にする（印の行が 2 つになり、読み方も 2 つになる）。担当が PR を draft で出し、gate が判定したら Andon が外す（誰でも 1 押しで外せて、push のあとに隙間が残る）。必須の check と draft の両方（守る状態が 2 つになる）。

3. **2026-11-23 通したときとオーナー待ちは緑、差し戻しは赤、止めている間は黄にする。判定の前は印を書かない。**（オーナーが決めた。黄の行と、担当へ戻ったときの行は Chief Engineer が決めた）
   ADR-0054 決めたこと 2（オーナー待ちでは印を書かない）を覆す。印が無いと、必須の check がオーナーのマージも断るからである。同 3（前の判定の印が残るときだけ黄で上書きする）は要らなくなる。オーナー待ちが赤くならないことは保つ。
   細部:

   | 状態 | 印 | GitHub の上で |
   |---|---|---|
   | 判定の前（レビュー中、担当が直している、CI を待っている） | 書かない | マージできない |
   | 通した（`approve`） | 緑 | マージできる |
   | オーナー待ち（`needs_human`） | 緑 | マージできる |
   | 差し戻し（`fix`） | 赤 | マージできない |
   | 止めている | 緑だった印を黄に書き換える | マージできない |
   | 判定のあとに、担当が課題を `doing` に戻した | 緑だった印を赤に書き換える | マージできない |

   説明文は ADR-0054 決めたこと 5 の形（「andon gate: <見出し>（<最初の理由>）」）のままにする。オーナー待ちの緑の印も、理由を括弧で持つ。黄の説明文は「andon gate: メンテナーの確認を待っています」、担当へ戻ったときの赤は「andon gate: 担当が直しています」にする。止めた理由の課題の名前は書かない（PR は外の人も読む）。同じ commit に、同じ状態と説明文の印が既にあれば、書き直さない。ADR-0054 決めたこと 4・6・8 は変えない。
   理由: 緑は「マージしてよい」を指す。オーナー待ちは、オーナーがマージしてよい状態である。担当が直している commit は、前に渡した commit でも、今は渡していない。
   捨てた案: 判定の前に黄を書く（担当の `gh pr checks --watch` が終わらなくなる。ADR-0054 が同じ理由で捨てた）。担当へ戻った commit の緑を残す（担当が直している途中の PR がマージできる。問題の文そのものである）。

4. **2026-11-23 gate は、オーナーがマージする変更かどうかを、検査とレビューのあとに見る。**（オーナーが決めた）
   決定式の順を変える。委譲の範囲の外かどうか（`delegation`）を見る所を、検査・CI・レビュー・受け入れ条件・画面の証拠・CI の待ちのあと、通す直前に移す。差し戻す理由があれば、オーナーがマージする変更でも先に担当へ差し戻す。CI が走っている間は待つ。オーナー待ちになるのは、委譲の中なら通していた PR だけになる。
   細部: 先に見るもののうち、PR の事情（衝突・base・作者・fork・ファイルの件数）、予約キーを信じられないとき、条件表に repo が無いときは、今の順のままオーナー待ちにする（gate が判定できない形である）。Must fix が同じ commit で 2 度残ったときと、担当がレビュアーを覆したときのオーナー待ちも変えない。
   理由: 事実 7 のとおり、今のオーナー待ちは、レビューも検査も見ていない commit に付く。これを緑にすると、手続きが終わっていない PR に「マージしてよい」印が付く。オーナーに届くマージの依頼も、検査とレビューが済んだものだけになる。
   払うもの: オーナーがマージする変更でも、条件表の検査が gate で走る。オーナーにマージの依頼が届くのは、CI が終わったあとになる。
   捨てた案: 順は変えず、差し戻す理由の無いオーナー待ちだけを緑にする（オーナーに、押せないマージの依頼が届く。緑かどうかを決める式が決定式と別に要る）。順も印も今のままにする（残った指摘が見えないまま緑になる）。

5. **2026-11-23 dispatcher は、PR を読むたびに印をあるべき形に揃える。**（Chief Engineer が決めた）
   今の gate は、判定したときに 1 度だけ印を書き、書けなくても先へ進む。印が必須になると、書けなかった緑は、オーナーも dispatcher もマージできない PR を残す。そこで dispatcher は、PR を持つ `review` と `doing` の課題について、今の commit の印を読み、決めたこと 3 の表と違えば書き直す。
   細部: 読むのは、dispatcher が今も PR の head を読んでいる 1 分に 1 回の読みに `statusCheckRollup` を足す形にし、呼び出しを増やさない。あるべき印は、dispatcher が今持っている判定（その commit を gate が判定した結果）と、課題の status と、止めているかどうかから決める。判定を持たない commit には書かない。書くのは、マージと同じ鍵（事実 13）の中で、止めているかを読み直してからにする。切り替えのとき、開いている PR に今ある赤いオーナー待ちの印も、この仕組みが緑に直す。ADR-0054 決めたこと 7（今ある PR の赤い印は直さない）は、この部分だけ覆す。赤いままだと、必須の check がオーナーのマージを断るからである。閉じた PR の印は直さない。
   残る隙間: 担当が課題を `doing` に戻してから、dispatcher が次に PR を読むまで（長くて 1 分ほど）、前の緑が残る。`kanban start` は GitHub に書けないので、この間は閉じない。
   理由: 書き込みは失敗する。1 度きりの書き込みに頼ると、失敗がマージできない PR になって残る。
   捨てた案: 書けなかったときにその場で数回打ち直す（GitHub が長く落ちていると残る。直す場所が 2 つになる）。

6. **2026-11-23 Chief Engineer の文書の PR は、渡されたときに dispatcher が緑を書く。**（オーナーが決めた）
   Chief Engineer の文書の PR は gate を通らない。必須の check は repo の全部の PR に効くので、誰かが印を書かないとマージできない。そこで dispatcher は、Chief Engineer の課題が `review` に入ったら、その PR の今の commit に緑を書く。commit が動いたら書き直す。止めている間は黄、課題が `doing` に戻ったら赤にする（決めたこと 3・5）。説明文は「andon gate: 仕様・設計の文書です。メンテナーがマージします」にする。
   帰結: Chief Engineer が渡す前の文書の PR は、印が無いのでマージできない。決めたこと 1 の範囲には入れなかった形だが、必須の check の帰結として止まる。事実 4 のとおり、今までのマージは全部が渡したあとだった。
   理由: 渡したあとは、オーナーが対話の出口で確かめた文書である。gate の判定（レビュー・受け入れ条件）は文書の PR に当てはまらない。
   捨てた案: 文書の PR も gate に通してオーナー待ちと判定させる（マージの問いがオーナーに 1 つ増える。オーナーは出口で確かめている）。PR を出した時点で緑を書く（渡す前の PR を止めないが、Chief of Staff が止める前にマージされうる）。

7. **2026-11-23 止める口は、予約キー `hold` と、止める command・外す command にする。**（Chief Engineer が決めた）
   Chief of Staff は `andon issue hold <id> <理由の課題>` で止め、`andon issue unhold <id>` で外す。`hold` の値は、止めた理由の課題（答えを待つ問いの課題）の番号である。理由の課題が `done` になっても、止めは外れない。外すのは command だけである。理由の課題が `done` になったのに止めが残っていたら、dispatcher が Chief of Staff に 1 度知らせる。
   理由: 答えが「直す」のとき、直す前の commit を渡してはいけない。自動で外すと、Chief of Staff が答えを担当へ伝える前に PR が渡り、自動マージの PR はそのままマージされる。外し忘れは、マージできない側に倒れ、知らせで気づける。
   捨てた案: kanban の待つ相手（`after`）を使い、理由の課題が済んだら自動で外す（上の理由。加えて、`kanban start` は済んでいない相手を持つ課題を断るので、止めている間、担当が課題を `doing` に戻せない。`andon measure` が、`after` を持つ課題を、待って始まった課題に数えてしまう）。PR の課題そのものに開いている問いがあれば渡さない（Chief of Staff の問いは別の課題に置く決まりで、PR の課題には付かない。PR #491 を止めていた問いもそうだった）。理由の課題を持たない真偽の印にする（止めた理由が kanban に残らない）。

8. **2026-11-23 止める command は、マージと同じ鍵を取り、緑の印を黄に書き換えてから返る。**（Chief Engineer が決めた）
   `andon issue hold` は、dispatcher のマージと同じ鍵（事実 13）と gate の鍵を、この順で取る。その中で `hold` を書き、今の commit を渡しているか（dispatcher の判定が通したかオーナー待ち、または今の印が緑）を見て、渡していれば黄を書いてから返る。黄を書けなかったら、そう言って失敗で終わる（`hold` は書いたままにし、決めたこと 5 が揃える）。Chief of Staff は、この command が成功で返ってから、オーナーに待つよう伝える。`andon issue unhold` も同じ鍵を取って `hold` を消す。印は書かない。gate も、自分の鍵を取ってから `hold` を読む。
   理由: モデルで調べると、キーだけを書いて印を dispatcher の次の巡に任せる形は、その間、止めたはずの PR に緑の印が残り、マージできた。鍵を取らない形は、止める command が返ったあとに dispatcher が自分でマージした（下の「モデルで見つけた破れ」）。Chief of Staff には、止まったことを確かめる手段が要る。
   細部: `kanban set <id> hold=…` で直接書いても、dispatcher が次に PR を読むとき黄にする。それまでの間は緑が残る。手引きは command を使うと書く。

9. **2026-11-23 止めている間、gate は回らない。判定のあとに止められたら、判定を捨てる。**（Chief Engineer が決めた）
   dispatcher は、止めている課題に gate を回さない。課題を読むだけなので、GitHub は読まない。`andon gate` を手で打っても、最初に `hold` を読み、止めていれば `wait` を返して終わる（記録も印も書かない。`--dry-run` も同じ）。待ちが長いとオーナー待ちに倒す決まり（`github_checks_wait_minutes`）は、止めの待ちには当てない。gate が通したかオーナー待ちと判定したあとに止められたら、dispatcher は、commit が動いたときと同じに扱う。gate のマージの問いを閉じ、判定を捨てる。dispatcher が自分でマージする直前の読み直しにも、止められていないかを足す。外したら、次の巡で gate から回し直し、gate が印を書く。
   理由: 止めている PR のマージの問いが残ると、オーナーは押せない PR のマージを頼まれる。止めている間に巡ごとに gate を回すと、GitHub の呼び出しと条件表の検査が、止めている間ずっと走る。
   払うもの: 止めている間は、CI が落ちても担当へ差し戻されない。外したときに差し戻される。
   捨てた案: 止めていても差し戻しだけは返す（止めている間、gate を丸ごと回し続けることになる）。新しい判定 `hold` を足す（約束の JSON の判定の値が増える。`wait` と同じ動きで足りる）。

10. **2026-11-23 担当が `hold` を消しても、止めは外れない。**（Chief Engineer が決めた）
    gate と dispatcher は、`hold` が空でも、`hold` を最後に変えた `log.jsonl` の行の書き手が、その課題かその子の課題の担当の端末なら、止めているものとして読む。足すほうは、誰が書いても止まる。
    理由: 消すと渡る。消してよいのは Chief of Staff とオーナーである。信じられない書き換えをオーナー待ちにすると、オーナー待ちは緑なので、止めた PR が渡ってしまう。
    捨てた案: ほかの予約キーと同じ照合（今の Chief of Staff の端末が最後に書いたか。`contract.md` §2）を使う（Chief of Staff が起き直すと、前の端末が書いた `hold` が全部信じられなくなる）。照合しない（担当が自分の課題の `hold` を消すと、止めが黙って外れる）。

11. **2026-11-23 オーナー待ちの一覧は、止めた PR をマージの依頼に出さず、止めている問いの後ろに数える。**（Chief Engineer が決めた）
    `andon waiting` は、止めている課題をマージの依頼に並べない。理由の課題の行には、止めている課題を「後ろで待つ」課題として数える。止めている課題は、働いている枠でなく、オーナー待ちの枠に数える。
    細部: オーナーの課題の行の数え方は、同じ日に書かれている ADR-0057（オーナーの課題を判断か手作業のどちらかにする）が決める。この文書が決めるのは、マージの依頼の行と、後ろで待つ課題の数え方である。
    理由: 事実 15 のとおり、今の一覧は止めを知らない。直さないと、押せないマージの依頼が並ぶか、止めた課題が働いている枠を取って、新しい担当を起動できなくなる。後ろで待つ課題の多い問いは、先にオーナーに出る。

12. **2026-11-23 dispatcher のマージが必須の check で断られたら、次の巡で打ち直す。**（Chief Engineer が決めた）
    `gh pr merge` が「the base branch policy prohibits the merge」で終わったとき、dispatcher は、base が動いたときと同じ扱い（次の巡で gate から回し直す。3 回まで）にする。3 回で通らなければ、今と同じく、マージできなかったという問いをオーナーに出す。
    理由: 事実 11 のとおり、印を書いた直後は GitHub の側の計算が追いつかないことがある。回し直しでは、印は既に緑なので書き直さない（決めたこと 3 の細部）。巡の間隔（5 秒から 1 分）を置けば追いつく見込みである。
    捨てた案: その場で数秒待って打ち直す（待つ長さの根拠が無い）。`gh` の手前の検査を飛ばす `--admin` を付ける（越えてよい相手を足したときに、必須の check を越えてしまう）。

13. **2026-11-23 gate は、衝突を `mergeable` の欄でも読む。**（Chief Engineer が決めた）
    今の gate は `mergeStateStatus` が `DIRTY` のときだけを衝突と読む。必須の check が足りない PR は `BLOCKED` になる。公開の repo で見た 26 本では、衝突と `BLOCKED` の理由が重なると `DIRTY` が返っていたが、GitHub の文書に順の決まりは無い。gate は `mergeable` が `CONFLICTING` のときも衝突と読む。
    理由: 判定の前の PR は必ず `BLOCKED` の理由を持つようになる。衝突が隠れると、gate は衝突した PR を通してしまう。

14. **2026-11-23 抜け道は作らない。ruleset の越えてよい相手は空のままにする。**（オーナーが決めた。細部の問いの並びは Chief Engineer が決めた）
    印が無くてもマージできる口は、Andon にも ruleset にも作らない。Andon の外の PR をマージするときと、Andon が止まっていて印を書けないときは、オーナーが ruleset から `andon/gate` の行を外し、済んだら戻す。手順は `docs/operate.md` に書く。
    細部: gate が判定できなかったとき（終了コード 3）に dispatcher が出す問いは、今は「オーナーが PR を読んでマージを決める」を先頭に置く。印が無いと押せないので、先頭を「Chief of Staff が原因を直して回し直す」に替える。
    理由: 越えてよい相手に管理者を足すと、同じアカウントで動く dispatcher の `gh pr merge` も越えられるようになる。Andon の外の PR は 233 本のうち 0 本だった（事実 16）。
    捨てた案: 越えてよい相手に repo の管理者を足す（上の理由）。オーナーが打つ、印を手で書く command を足す（使う場面が今は無い。同じアカウントなので、エージェントが打つのを止められない）。

15. **2026-11-23 repo の GitHub の auto-merge を切る。**（オーナーが決めた）
    オーナーが、repo の設定の「Allow auto-merge」を切る。
    理由: 事実 14 のとおり、有効のままだと、判定の前の PR に auto-merge を予約できる。予約すると、gate がオーナー待ちと判定して緑を書いたときに、理由を読まないままマージされる。dispatcher は GitHub の auto-merge を使っていないので、切っても Andon の自動マージは変わらない。この設定は、わざと越える者を止めるためのものではない（事実 9）。オーナーが、判定の前に予約を押せるかどうかを決めるものである。有効でも切っても、検査とレビューの前に入ることは無い。
    捨てた案: 有効のままにする（判定の前の 1 押しが、判定のあとのマージになる。検査とレビューは済んでいるが、オーナー待ちの理由は読まれない）。

16. **2026-11-23 切り替えは、実装を当て、印が揃ったのを確かめてから、オーナーが設定を変える。**（Chief Engineer が決めた）
    順は 3 つ。実装をセルに当てる。開いている PR のうち、オーナー待ちと判定済みのものに緑の印が付いたことを Chief of Staff が確かめる。オーナーが、main の ruleset の必須の check に `andon/gate` を足し、決めたこと 15 の設定を切る。出どころ（GitHub App）は限らない。「最新の base で検査を回し直す」は今のまま求めない。
    理由: 先に ruleset を足すと、今の赤いオーナー待ちの PR が全部マージできなくなる。設定を変えるのはオーナーである（前提 6）。
    細部: Andon が扱う repo が 2 つ以上のセルでは、repo ごとに同じ順でする。ruleset に足していない repo では、印は今までどおり目印にとどまる。

17. **2026-11-23 モデルに、印と止めの状態と、規則を 3 本足す。**（Chief Engineer が決めた）
    `model/IssueLife.tla` に次を足す。変数は、今の commit の印（無い・緑・赤・黄）、止めているか、ruleset が印を必須にしているか、gate が今の commit を渡しているか、渡していない commit か止めている間にマージされたか、の 5 つ。動きは 3 つ。Chief of Staff が止める（緑の印を黄にし、判定のあとなら判定を捨てる）。Chief of Staff が外す。dispatcher が印を揃える（止めているのに残った緑を黄に、`doing` に戻った課題の緑を赤にして、渡したことを取り消す）。commit が動く動き（担当の push・rebase・判定のあとの push）は、印を無しにし、渡したことを取り消す。gate の 3 つの判定は、止めていないときだけ起き、印を書く。オーナーのマージと dispatcher のマージは、ruleset が必須にしていれば、印が緑のときだけ起きる。dispatcher のマージは、止めていないときだけ起きる。
    足す規則は 3 本。「ruleset が印を必須にしていれば、gate が渡していない commit も、止めている間の PR も、マージされない」。「開いている PR の今の commit の印が緑なのは、gate がその commit を渡していて、止めていないときだけ」。「dispatcher は、止めている PR を自分でマージしない（ruleset が無くても）」。状態の数の控え、対応表、壊した見本は、実装の PR の担当が直す。壊した見本には、止める command が印を書かない形と、dispatcher のマージが止めを読み直さない形を足す。
    理由: 課題の一生の動きを変えるので、同じ PR でモデルを直す決まりである（`AGENTS.md`）。

18. **2026-11-23 印の決まりの正は `docs/contract.md`。設定と外し方は `docs/operate.md` に書く。**（Chief Engineer が決めた）
    `docs/contract.md` に、決めたこと 3 の表、決定式の順（決めたこと 4）、予約キー `hold`（決めたこと 7・10）、dispatcher が揃えること（決めたこと 5・6）、`andon waiting` の数え方（決めたこと 11）を書く。`docs/operate.md` に、ruleset と auto-merge の設定の手順、印の読み方、外し方（決めたこと 14）、Chief of Staff の止め方を書く。`docs/dispatcher.md` と `process/` に、印を揃える段と止めの扱いを足す。Chief of Staff の手引き（`package/skills/chief-of-staff/playbooks/`）に次を書く。オーナーに待つよう頼む前に `andon issue hold` を打つこと。答えが出たら、答えを反映してから `andon issue unhold` を打つこと。レビュー中の PR の URL を、マージの依頼と同じ返事に並べないこと。`gate.md` の「理由が委譲だけなら照合は通っている」は、決めたこと 4 で本当になる。担当の skill に、`doing` に戻ると前の commit の印が赤になることを書く。

### モデルで見つけた破れ

課題の一生のモデル（`model/`）の写しに、決めたこと 17 の変数と動きを書き足し、`model/check --draft` と TLC の両方で調べた。

- **選んだ案**: 規則 15 本を全部守った。状態は 28,004。「どの状態からも、取り消しを使わずにマージまで行ける」も守った。
- **捨てた形 1（止めるときキーだけを書き、印は dispatcher の次の巡が直す）**: 足した規則のうち 2 本が破れた。道は次のとおり。担当が PR を出す。レビュアーが返す。担当が `andon issue ready` を打つ。gate がオーナー待ちと判定して緑を書く。Chief of Staff が止める。この時点で、止めた PR に緑が残る。dispatcher が巡る前にマージが押されると、ruleset が必須にしていても通る。
- **捨てた形 2（止める command が鍵を取らず、dispatcher が判定も捨てない）**: 「dispatcher は、止めている PR を自分でマージしない」が破れた。道は次のとおり。gate が通す。Chief of Staff が止める。dispatcher が、止める前の判定のままマージする。
- **ruleset に足していない repo**: 「渡していない commit はマージされない」は、PR を出した直後のマージで破れる。今の状態である。規則が空振りでないことの確かめとして回した。

モデルが見ていないもの: 印の書き込みの失敗（決めたこと 5 が直す。直るまでの間は、前の印が残る）。担当が課題を `doing` に戻してから印が赤になるまでの間のマージ（決めたこと 5 の「残る隙間」。モデルは、印が赤になるまで、渡したままと数える）。決定式の順（決めたこと 4。モデルのオーナー待ちは、理由を区別しない）。Chief Engineer の課題（gate を通らないので、このモデルの外）。担当が `hold` を消すこと（決めたこと 10）。GitHub の側の計算の遅れ（決めたこと 12）。

## やらないこと（範囲の外。理由）

- **ruleset と repo の設定を Andon が書き換えること。** 前提 6。設定はオーナーがする。
- **新しく作るプロジェクトの既定にすること。** 決めたこと 1。`andon create` は ruleset を作らない。手順は `docs/operate.md` に書くので、ほかの repo でもオーナーが足せる。
- **ruleset に `andon/gate` が入っているかを Andon が見張ること。** 外したまま戻し忘れると、守りは黙って無くなる。見張る口は今は足さない。外す場面が出てから考える。
- **印を書いた者を確かめること。** 事実 9。GitHub App を持たない限り、出どころは限れない（前提 5）。
- **判定の前の PR を、PR の一覧で見分けられるようにすること。** 決めたこと 2 の「払うもの」。draft や黄の印は、決めたこと 2・3 で捨てた。
- **担当が課題を `doing` に戻した瞬間に印を変えること。** 決めたこと 5 の「残る隙間」。`kanban` の CLI は GitHub に書かない。
- **判定の前にマージされたことを知らせる通知を足すこと。** マージできなくなるので要らない。
- **GitHub App と check run。** 前提 5 と ADR-0054 決めたこと 4。
- **gate の記録（`library/gate/<head>.json`）の形を変えること。** 変えるのは、印の出し方と、決定式の順と、止めているときの待ちである。
- **オーナー待ちの理由のうち、委譲の外でないもの（PR の事情・予約キー・条件表）を、検査のあとに回すこと。** gate が判定できない形で、今もオーナーが見ている。

## 受け入れ条件（テストできる形）

継ぎ目は 5 つ。決定式と、印の状態と説明文を決める純粋な関数（`internal/gate/`）。`andon gate` と `andon issue hold`・`unhold` が `gh` に渡す引数（`tests/andon-gate.sh` と `tests/andon-issue.sh` の偽の `gh` が控える）。dispatcher の巡（`tests/andon-dispatcher.sh`）。オーナー待ちの一覧（`internal/waiting/`）。モデル（`model/check`）。本物の GitHub には書かない。本物でしか見られないものは、オーナーが見る。

- **A1 決定式が、委譲の外を最後に見る。** `go test ./internal/gate/` の表のテストで、委譲の外の変更について次を見る。CI が落ちていれば `fix`。レビューが無いか古ければ `fix`。Must fix が残っていれば `fix`。受け入れ条件の fail があれば `fix`。CI が走っていれば `wait`。どれも無ければ `needs_human` で、理由は委譲の 1 件。PR の事情・予約キー・条件表の理由は、今の順のまま `needs_human`。証拠は、テストの名前を含む `go test -run` の出力。
- **A2 判定ごとの印が、決めたこと 3 の表のとおりになる。** `go test ./internal/gate/`（印を決める関数の置き場は実装が決める）の表のテストで、`approve` と `needs_human` は `success`、`fix` は `failure`、`wait` は書かない、止めていて渡しているときは `pending`、止めていて渡していないときは書かない、`doing` で前の印が `success` のときは `failure`、同じ状態と説明文の印が既にあるときは書かない、を見る。証拠は A1 と同じ形。
- **A3 `andon gate` が、オーナー待ちで緑を書く。** `tests/andon-gate.sh` のオーナー待ちの場面で、偽の `gh` の控えに `context=andon/gate` と `state=success` があり、説明文が理由を括弧で持つことを見る。衝突の場面では、`mergeStateStatus` が `BLOCKED` で `mergeable` が `CONFLICTING` のとき、衝突として `needs_human` になることを見る。証拠は `tests/run --changed` の `andon-gate.sh` の行。
- **A4 止めている課題で、gate が GitHub を読まずに待つ。** `tests/andon-gate.sh` で、`hold` を持つ課題の `andon gate` が `wait` を返し、偽の `gh` が 1 度も呼ばれず、記録が無いことを見る。review に入ってから `github_checks_wait_minutes` を過ぎていても `wait` のままであることを見る。`hold` が空で、最後に変えた行の書き手が担当の端末のときも `wait`、Chief of Staff の端末のときは判定に進むことを見る。証拠は A3 と同じ行。
- **A5 止める command が、鍵を取り、印を書いてから返る。** `tests/andon-issue.sh` で次を見る。`andon issue hold` が `hold` を書き、渡している commit には `state=pending` を書いて 0 で終わる。印の書き込みを失敗させると 0 以外で終わり、`hold` は残る。渡していない commit には書かない。無い課題を理由に渡すと断る。別のプロセスがマージの鍵を持っている間は、放されるまで待つ。`andon issue unhold` が `hold` を消し、印を書かない。証拠は `tests/run --changed` の `andon-issue.sh` の行。
- **A6 dispatcher が印を揃える。** `tests/andon-dispatcher.sh` に場面を足す。判定がオーナー待ちで、偽の `gh` が印を無しか赤と返すとき、次に PR を読む巡で `state=success` が書かれる。止めたあとの巡で、gate の問いが閉じ、gate が回らず、残っていた緑が `pending` になる。外したあとの巡で、gate が回り直す。課題が `doing` に戻ったあとの巡で、残っていた緑が `failure` になる。理由の課題を `done` にすると、Chief of Staff への知らせが 1 度だけ積まれる。Chief Engineer の課題が `review` に入った巡で、その PR の commit に `state=success` が書かれ、commit が動くと新しい commit に書かれ、止めると `pending` になる。証拠は `tests/run --changed` の `andon-dispatcher.sh` の行。
- **A7 dispatcher が、止めている PR を自分でマージしない。** `go test ./internal/issue/` で、マージの直前の読み直しが、止められていれば通さないことを見る。`gh pr merge` が「the base branch policy prohibits the merge」を返したとき、回し直しになり、4 回目で問いになることも見る。gate が判定できなかったときの問いの先頭が「原因を直して回し直す」であることも見る。証拠は `go test -run` の出力。
- **A8 オーナー待ちの一覧が、止めを数える。** `go test ./internal/waiting/` で、止めている課題がマージの依頼に出ないこと（gate が通した課題、オーナー待ちの課題、Chief Engineer の課題の 3 つの形で）、理由の課題の行の「後ろで待つ」に入ること、オーナー待ちの枠に数えられて働いている枠に数えられないことを見る。証拠は `go test -run` の出力。
- **A9 モデルが、足した規則を守る。** `model/check` が通り、規則の控えに 3 本が増えている。足した 2 つの壊した見本が、控えに書いた規則で落ちる。証拠は `model/check` の出力。
- **A10 文書が揃う。** `docs/contract.md`・`docs/operate.md`・`docs/dispatcher.md` と、Chief of Staff の手引き、担当の skill に、決めたこと 18 の中身がある。`docs/process/` が作り直しても変わらない。証拠は、当たる節を `grep -n` した出力と、`tests/process.sh`・`tests/schema.sh` が通った行。
- **A11 本物の PR で、判定の前はマージできない。**（`manual`。オーナーが見る）設定を変えたあとの最初の担当の PR で、次を見る。gate の判定の前は、マージの欄が `andon/gate` を待つと出して押せず、auto-merge のボタンも出ない。オーナー待ちと判定されたあとは、`andon/gate` が緑で、押せる。担当の `gh pr checks --watch` が、CI が終わると終わる（事実 10 を ruleset の repo で確かめる）。
- **A12 本物の PR で、止めた PR はマージできない。**（`manual`。オーナーが見る）Chief of Staff が最初に `andon issue hold` を打った PR で、印が黄になり、マージの欄が押せないことと、外すと緑に戻ることを見る。

## 価値の仮説

1. **誰の何が変わるか。** オーナーが、手続きの途中の PR と、Chief of Staff が止めた PR を、うっかりマージできなくなる。マージできる PR は、gate が今の commit を判定した PR だけになる。オーナーに届くマージの依頼は、検査とレビューが済んだものだけになる。
2. **見る信号。** 信号の記録（`contract.md` §15）に当たる出所は無い。観察の担当が、kanban の記録から数える。1 つは、dispatcher の `merged` の通知のうち、dispatcher でない者のマージで、マージされた commit の gate の記録が無いか、マージより後に書かれたものの数。もう 1 つは、gate の記録のオーナー待ちのうち、理由が委譲の外だけで、CI が終わっていないか、レビューの結果が無いか、Must fix が残っているものの数。あわせて、ruleset から `andon/gate` の行を外した回数と、印が無くてマージできなかった PR があったかを、オーナーに聞く。
3. **見る日。** 2026-12-04。設定を変える日の見込みを 2026-11-27 として、その 7 日後。
4. **誰が要るか。** オーナー。ruleset に 1 行足し、auto-merge を切る。見る日に、外した回数と、困った PR があったかを一言で答える。
5. **効いたとどう分かるか。** 設定を変えてから見る日までの、判定の前のマージが 0 本。変える前は 4 日で 21 本だった。検査かレビューが済んでいないオーナー待ちが 0 件。変える前は 96 件のうち 35 件だった。オーナーが、マージしたいのに印が無くて止まった、と言った回数が 0 回。
6. **何があれば止めるか。** オーナーが ruleset の行を外した回数が 7 日で 2 回以上、または、判定済みなのに緑の印が付かずマージできない PR が 1 時間以上残ったことが 1 度でもあれば、行を外したままにして、印を揃える仕組み（決めたこと 5）から見直す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

ADR-0054 の実装がまだなので、同じ所（`internal/gate/` の印の書き方）を触る 1 本目にまとめる。ADR-0054 の実装の課題と観察の課題が kanban にあれば、1 本目に合わせて直す。ADR-0054 の価値の仮説のうち、オーナー待ちに印の行が無いことを見る部分は、緑の印があることを見る形に変わる。

1. **gate の順と印を決め直し、dispatcher が揃える。** 決めたこと 3・4・5・6・12・13 と、ADR-0054 決めたこと 5・6・8。受け入れ条件 A1・A2・A3・A6 の揃える場面・A7 の打ち直しと問い・A10 の当たる部分。触るのは `internal/gate/`、`internal/issue/merge.go`・`settle.go`、`internal/dispatcher/`、`tests/andon-gate.sh`、`tests/andon-dispatcher.sh`、`docs/contract.md`、`docs/operate.md`、`docs/dispatcher.md`、`process/`、`package/skills/chief-of-staff/playbooks/gate.md`。モデルは、印の変数と、印を揃える動きまでを足す。
2. **止める口を足す。** 決めたこと 7・8・9・10・11・17 の残りと、受け入れ条件 A4・A5・A6 の止める場面・A7 の読み直し・A8・A9・A10 の残り。1 のあと。触るのは `internal/issue/`（command `hold`・`unhold`）、`internal/gate/`、`internal/waiting/`、`schema/`（予約キーと通知）、`model/`、Chief of Staff の手引き。資料のモデル（Chief Engineer の課題の資料置き場にある下書き）を依頼書に添える。
3. **切り替える。** 決めたこと 14・15・16。1 のあと（2 を待たなくてよい）。Chief of Staff が、開いている PR の印を確かめる。オーナーが ruleset に 1 行足し、auto-merge を切る。オーナーの手作業の課題にする。受け入れ条件 A11。A12 は 2 のあと。

## 開いている問い（残った問い。決断の種類つき）

無し。出口で聞いた 4 つ（決めたこと 4・6・14・15）は、オーナーが決めた。

## 出典

- オーナーとの対話（2026-11-23）。入口の 3 つの答え（前提に同意、範囲、止め方）。出口の 4 つの答え（gate が見る順、Chief Engineer の文書の PR に緑を書く時、抜け道、auto-merge）と、最後の確かめの「決めた」。
- [ADR-0054](0054-gate-status-by-decision.md) 決めたこと 2・3・4・5・6・7・8・9 と「やらないこと」。同じ日に書かれている ADR-0057（オーナーの課題を判断か手作業のどちらかにする）の、オーナー待ちの一覧の数え方。[ADR-0026](0026-pr-rules-for-outside-readers.md) 決めたこと 9。[ADR-0038](0038-statechart-in-chief-engineer-design.md) 決めたこと 13。[ADR-0053](0053-command-system.md)。
- repo のコード: `internal/gate/gate.go`（`run`・`postGitHub`）、`internal/gate/decide.go`（`Decide`・`SummarizeChecks`・`GateContext`）、`internal/issue/merge.go`（`mergeOne`・`approve`・`headMoved`・`stillAuthorized`・`RetryMerge`）、`internal/issue/lock.go`、`internal/issue/settle.go`、`internal/ledger/ledger.go`（`Trusted`）、`internal/waiting/waiting.go`、`internal/dispatcher/dispatcher.go`、`cmd/kanban/main.go`（`after` の扱い）、`model/`。
- PR #480・#487（独立レビューの途中のマージ）、PR #491（待つよう頼んでいる間のマージ）、PR #494（base が動いたときの打ち直し）。
- GitHub の文書:
  - https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks
  - https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets
  - https://docs.github.com/en/rest/commits/statuses
  - https://docs.github.com/en/rest/repos/rules
  - https://docs.github.com/en/rest/pulls/pulls
- GitHub CLI のソース（`cli/cli` の `pkg/cmd/pr/checks/checks.go`・`aggregate.go`、`pkg/cmd/pr/merge/merge.go`。2026-11-20 の trunk）と issue:
  - https://github.com/cli/cli/issues/6448
  - https://github.com/cli/cli/issues/8855
  - https://github.com/cli/cli/issues/13388
