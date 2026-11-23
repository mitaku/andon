# ADR-0049: オーナーの介在回数を、操作の種類で数える（仕様・設計。2026-11-22）

- 状態: **決定（2026-11-22）**。オーナーが対話の中で「決めた」と言った。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書き、この行を「覆された → ADR-NNNN」に直す。小さな改めは「決めたこと」に下から足す。
- 主題: `andon measure` の一次の指標「PR あたりのオーナーの介在回数」が、オーナーの実際の操作を数えるようにする。数える操作の範囲、数える元の記録、問いの答えに足す「誰が決めたか」の印、`andon measure` の出力の形。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`、kanban の形式の正は [ledger.md](../ledger.md) と `schema/ledger/`。この文書が足す約束は「決めたこと」に形を書き、実装の課題が写す（それまでは提案）。オーナー待ちの 3 種は [ADR-0042](0042-owner-waits-do-not-stop-the-line.md) 決めたこと 8。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 介在回数に数えるのは、マージ・判断の答え・オーナーの課題の 3 種にする | `docs/contract.md`・`internal/measure/measure.go` | 生きている |
| 2 | 数え方を、履歴の書き手で見分ける形から、操作の種類で数える形に改める | `docs/contract.md`・`internal/measure/measure.go` | 生きている |
| 3 | オーナーのマージは、gate の記録にマージの印が無いことで見分け、GitHub のマージした者は読まない | `docs/contract.md`・`internal/measure/measure.go` | 生きている |
| 4 | 問いの答えに、誰が決めたかの印を足す | `docs/ledger.md` | 生きている |
| 5 | 印は kanban の問いの任意のキー `decidedBy` にし、`kanban answer --decided-by` で書く | `docs/ledger.md`・`schema/ledger/questions.v3.schema.json` | 生きている |
| 6 | 答えを書く者が、場面に合う値を書く | `docs/contract.md` | まだ実装していない（コードの書き手は付けた。skill の分が残る） |
| 7 | 判断は、印が `owner` の答えを、答えた時刻で数える | `docs/contract.md` | 生きている |
| 8 | 答えを、決めた者ごとに数えて出し、印の無い答えは介在回数に入れない | `schema/measure.schema.json`・`docs/contract.md` | 生きている |
| 9 | 問いを持たないオーナーの課題は、片付いたら 1 回と数える | `docs/contract.md` | 生きている |
| 10 | PR の上のコメントと commit は、今は数えない | この ADR | 生きている |
| 11 | 合計は期間に起きた操作の数にし、PR の行にはその PR の課題に付いた操作を出す | `schema/measure.schema.json`・`docs/contract.md` | 生きている |
| 12 | 出力に内訳を足し、`andon measure` の出力のバージョンを 2 に上げる | `schema/measure.schema.json` | 生きている |

## 問題（オーナーの言葉で。言い直しの段落）

オーナーは、任せる範囲を広げるほど自分の手が減っているかを、PR あたりの介在回数で見たい。この数は 2026-11-07 に一次の指標と決めたものである。ところが今の `andon measure` は、実態と関係なく 0 と出す。Andon を開発しているセルでは、7 日で 208 本の PR がマージされ、そのうち 118 本をオーナーが自分でマージしたのに、介在回数は 0 回だった。数が実態を映していないので、委譲を広げる根拠にも、オーナーが読むダイジェストに載せる数にも使えない。直したいのは、オーナーが実際に手を動かした回数が、種類の内訳と一緒に出ることである。

モデル: 調べなかった（状態の動きを変えない。問いと課題の状態の移り方と、自動で動く部品の順は今のままである。`kanban answer` が新しく断るのは旗の形だけで、状態を見て断る条件は変えない）

## 読んで分かった事実（2026-11-22。main `cbbf79b` と、Andon を開発しているセルの kanban で実測）

1. **今の数え方。** `andon measure` は、マージされた PR の課題の `log.jsonl` の行のうち、`by` が `terminal:` でも `routine:` でもない行を、オーナーの介在として数える（`internal/measure/measure.go` の `human`）。
2. **履歴の書き手。** `log.jsonl` の 4,855 行のうち 4,842 行が `terminal:` で、残りは `routine:` が 10 行、設定の行が 3 行だった。課題に付く行は全部 `terminal:` である。kanban の CLI は、herdr の端末の中で打たれると必ず `terminal:<端末>` と書く（`cmd/kanban/ledger.go` の `who`）。Chief of Staff も dispatcher も端末の中で動く。
3. **結果。** 2026-11-16 から 11-22 の 7 日で、マージされた PR は 208 本、介在回数は 0 回と出た。
4. **オーナーのマージ。** 同じ 7 日で、dispatcher がマージした PR は 90 本、残りの 118 本は GitHub でマージされた。dispatcher がマージした PR には、gate の記録に `merged: true` が付く。`andon measure` はもうこの印を読み、PR ごとの `auto` に出している。118 本のうち 19 本は Chief Engineer の文書の PR である。
5. **GitHub のマージした者。** `gh pr list --state merged --json mergedBy` で直近 300 本を読むと、マージした者は全部同じアカウントだった。dispatcher はセルの `gh` の token でマージし、その token はオーナーのアカウントのものだからである。GitHub の記録では、オーナーのマージと自動マージを見分けられない。
6. **問い。** kanban の問いは 116 件で、115 件に答えがあった。106 件は、オーナーのマージや PR の head の動きを受けて閉じた答えだった。104 件は command が書き、2 件は Chief of Staff がマージを見て手で書いた。中身のある答えは 9 件で、5 件は Chief of Staff が自分で答える範囲の問い、4 件は灯りの種類の問いだった。
7. **誰が決めたかの記録は無い。** 答えの 115 件は全部、書き手が `terminal:` である。オーナーの答えを Chief of Staff が代わりに書いたものと、Chief of Staff が委譲の中で自分で決めたものと、要らなくなった問いを片付けた答えが、同じ形で残る。command が閉じた答えは、答えの文の全体が決まった形に一致するかで見分けている（`internal/dispatcher/corrections.go`）。書き手と見分けの形がずれないことは、検査 `TestMachineGateAnswersMatchWriters` が、書き手のソースの形を固定して見ている。
8. **オーナーの課題。** `who: person` の課題は 31 件で、片付いたものは 17 件だった。17 件のうち問いを持つのは 3 件で、14 件は問いを持たずに片付いた。14 件のうち少なくとも 6 件は、オーナーの判断を報告に書いて閉じている。トリアージが足すオーナーの課題は問いを持たない（`internal/triage/apply.go`）。種類を決められない問いも、問いの JSON にしない決まりである（`package/skills/ask-one-by-one/SKILL.md` §3）。オーナーの課題は、判断と手作業の両方を含む。
9. **ペインからの答え。** オーナー待ちのペインの `andon waiting answer` は端末の外で動くので、`by` がユーザー名か `unknown` になる。オーナーの答えだと仕組みで分かる口はこれだけだが、この 7 日に使われた行は 0 件だった。選択肢のある問いにしか使えず、戻せない決断には使えない（ADR-0042 決めたこと 17・18）。
10. **PR の上のコメントは、誰が書いたかを見分けられない。** オーナーの手直しの記録（`corrections.json`）は 268 件あり、PR の上のコメントを持つものは 1 件、オーナーの commit を持つものは 0 件だった。その 1 件は、担当が画面の写しを添付したコメントである。担当の報告の 5 秒前に書かれ、worker の skill が担当に打たせる添付の手順と一致する。担当もオーナーも同じアカウントで書き、拾う規則が除くのは gate のコメントとインラインの Bot のコメントだけである（[contract.md](../contract.md) §14）。
11. **残っていない操作。** オーナーが端末で Chief of Staff や Chief Engineer に打った文と、Chief Engineer のダイアログへの答えは、どの記録にも無い。
12. **答えを書く場所。** コードが `kanban answer` を呼ぶ場所は 2 か所である。ペインの答え（`internal/waiting/answer.go`）と、gate の問いを閉じる関数（`internal/issue/merge.go` の `closeQuestion`。settle と merge の 3 か所が呼ぶ）。skill が `kanban answer` を打たせる行は 4 行である（`package/skills/ask-one-by-one/SKILL.md` に 1 行、`package/skills/chief-of-staff/playbooks/blocked.md` に 1 行、`finish.md` に 2 行）。
13. **手順に書かれていない答え。** 担当が出した灯りの種類の問いにオーナーが答えたとき、手順は答えを担当へ `hc send` で返すと書くだけで、`kanban answer` を打つ行が無い（`blocked.md` の「オーナーに聞くこと」）。担当は `kanban received` で問いを閉じるので、その前に誰かが答えを書く必要があり、実際には Chief of Staff が書いている。gate の問いにオーナーが「担当に直させる」と答えたときも、手順は `hc send` だけで、問いは後で command が閉じる（`playbooks/gate.md`）。
14. **kanban の旗の読み方。** kanban の CLI は、知らない旗を値ごと受け取って捨てる（`cmd/kanban/main.go` の `parseArgs`）。旗を知らない古い kanban に `--decided-by` を付けても断られず、印の無い答えが書かれる。
15. **出力の読み手。** `andon measure --json` の `human_touches` を読むコードは、`internal/measure` と schema と検査のほかに無い。読むのはオーナーと振り返りである。

## 前提（同意済み。番号）

オーナーが 2026-11-22 に同意した。

1. 数えたいのは「オーナーが手を動かした回数」で、「kanban に行を書いたのが誰か」ではない。Chief of Staff がオーナーの答えを代わりに書いても、オーナーの 1 回である。
2. 何もしなければ、一次の指標は 0 のままで、委譲を広げた効果は自動マージ率でしか見えない。
3. マージの回数は今ある記録で数えられる。足りないのは、問いの答えを誰が決めたかの記録である。
4. 新しい部品は足さず、`andon measure` の数え方と、その約束の文を直して届ける。

## 検討した案

| 案 | 中身 | 大きさ | リスク | 再利用するもの | 捨てるもの |
|---|---|---|---|---|---|
| 最小 | 記録を足さず、今ある記録から推し量る。マージは gate の印、判断は問いの出し手と種類で数える | `internal/measure` だけ | Chief of Staff が委譲の中で決めた答えと、片付けの答えも数に入る。委譲を広げても判断の数が減らない | gate の記録、`questions.yaml` | `by` で見分ける数え方 |
| 印を足す（選んだ） | 最小の案に、答えの「誰が決めたか」の印を足す。答えを書く者が書く | `internal/measure`、kanban の旗 1 つ、コードの書き手 2 か所、skill の 6 行 | Chief of Staff が旗を付け忘れる、値を取り違える。付け忘れは数に出るが、取り違えは出ない | 同上と、kanban の任意のキーの決まり | 同上 |
| 横から | オーナー待ちが解けた出来事を dispatcher が新しい記録に書き、それを数える | 新しい記録と schema、dispatcher の差分の検出 | 待ちが解けたことは分かるが、解いたのがオーナーか Chief of Staff かは分からず、印が別に要る。書き手と記録が増える | `andon waiting` の数え方 | 同上 |

選んだのは「印を足す」である。最小の案は、この指標で見たいもの（委譲を広げた効果）のところで数が狂う。横からの案は、前提 4 に反して部品が増え、それでも印が要る。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-22 介在回数に数えるのは、マージ・判断の答え・オーナーの課題の 3 種にする。**（オーナーが決めた）
   オーナーは入口で「マージ・判断の答え・PR の上の手直し」を選んだ。そのあと、選ぶときに示した事実が 2 つ違っていたと分かり、出口で事実を示し直して決め直した。PR の上のコメントは誰が書いたかを見分けられない（事実 10）。オーナーの課題は手作業だけでなく判断を多く含み、問いを通らない（事実 8）。そこで、オーナーの課題を足し（決めたこと 9）、PR の上のコメントと commit を外す（決めたこと 10）。
   理由: この 3 種は、オーナー待ちの 3 種（マージの依頼・判断・手作業）が解けた出来事に当たり、どれも今ある記録か、決めたこと 4 の印で数えられる。
   捨てた案: マージだけを数える（数が自動マージ率の裏返しになり、判断を任せた効果が見えない）。端末で打った文と Chief Engineer のダイアログへの答えも数える（オーナーの発言を新しく記録に取ることになり、直しが大きい。入口でオーナーが外した）。

2. **2026-11-22 数え方を、履歴の書き手で見分ける形から、操作の種類で数える形に改める。**（オーナーが決めた。前提 1）
   2026-11-07 にオーナーが決めた一次の指標は「PR あたりのオーナーの介在回数」で、数え方は「`log.jsonl` の `by` がオーナーの行の数」と添えてあった。指標は変えず、この数え方だけを覆す。
   理由: `by` は command を打った者の記録で、決めた者の記録ではない（事実 2・7）。オーナーの操作は Chief of Staff か dispatcher の行として残るか、GitHub の上で起きて行に残らない。
   捨てた案: `by` に `owner` と書く口を足して、Chief of Staff が代わりに書くときに使う。`by` を読む部品が 2 つあり、意味を二重にすると壊れる。dispatcher は `by` が Chief of Staff の端末の行を「自分のしたこと」として知らせから外し、`by` が端末でない答えをペインの答えとして拾っている（[dispatcher.md](../dispatcher.md) §2.2）。
   細部: `internal/measure/measure.go` の `human` と、それを呼ぶ数え方は消す。`at` か `by` の無い行を飛ばした記録にする扱いは変えず、理由の文から「オーナーの介在に数えない」を外す。

3. **2026-11-22 オーナーのマージは、gate の記録にマージの印が無いことで見分け、GitHub のマージした者は読まない。**（Chief Engineer が決めた）
   期間にマージされた kanban の PR のうち、gate の記録に `merged: true` が無いものを、オーナーのマージ 1 回と数える。`andon measure` が今 PR ごとに出している `auto` が偽の PR と同じである。
   理由: GitHub のマージした者は、自動マージでもオーナーのアカウントになる（事実 5）。gate の印は dispatcher がマージしたときにだけ付き、新しい記録が要らない（事実 4）。
   捨てた案: `gh` の `mergedBy` を読む（事実 5 で見分けられない）。settle が課題を片付けるときに、履歴にオーナーのマージの行を足す（同じことが gate の印から導けるので、書き手を増やさない）。
   細部:
   - Chief Engineer の文書の PR と、gate を通らない課題の PR も、gate の印が無いのでオーナーのマージに数える。どちらもオーナーがマージしている。repo に書き込めるほかのメンテナーがマージした PR も、オーナーのマージに数える。
   - dispatcher がマージしたのに印が付かない道が、コードに 2 つある。マージは通ったが PR の読み直しに失敗したときと、印を書くのに失敗したときである（`internal/issue/merge.go`）。この PR はオーナーのマージに数えられる。実測の記録では起きていない（通知の記録で dispatcher のマージは 90 件で、`auto` の 90 本と一致した）。直すのは、記録に現れてからにする。

4. **2026-11-22 問いの答えに、誰が決めたかの印を足す。**（オーナーが決めた）
   答えを書く者が、その答えを決めたのがオーナーか、Chief of Staff か、command かを、答えと一緒に書く。
   2026-11-09 の決定は、`andon measure` のために「新しい記録は足さず、今ある記録から導く」とした。この決定を 1 点だけ改める。数えるために、今ある記録（`questions.yaml`）に任意のキーを 1 つ足す。`andon measure` が読むだけで何も書かないことと、集計を保存しないことは変えない。
   理由: 今ある記録からは、オーナーの答えと、Chief of Staff が委譲の中で決めた答えを見分けられない（事実 7）。推し量って数えると、委譲を広げて Chief of Staff が決める答えが増えるほど、数が実態より多く出る。この指標は委譲の効果を見るためのものなので、そこで狂うと役に立たない。
   捨てた案: 印を足さず、問いの出し手と種類から推し量る（上の理由）。ペインからの答えだけを数える（オーナーが会話で答えた分を落とす。事実 9）。

5. **2026-11-22 印は kanban の問いの任意のキー `decidedBy` にし、`kanban answer --decided-by` で書く。**（Chief Engineer が決めた）
   値は `owner`・`chief-of-staff`・`machine` の 3 つで、kanban の CLI がこの 3 つ以外の値を断る。旗を付けない `kanban answer` は今までどおり通り、キーを書かない。kanban の形式のバージョンは上げない。
   理由: 答えと印が同じ場所（`questions.yaml`）に 1 回で書かれ、書き手も置き場も増えない。任意のキーを足すのは形式の変更に当たらない（[ledger.md](../ledger.md) の互換の規則）。値を書くときに確かめれば、値の書き間違いが記録に残らない。
   捨てた案: Andon の側に、答えの記録を別のファイルで持つ（答えと印が 2 か所に分かれ、約束と schema が 1 つ増える）。`log.jsonl` の行に欄を足す（`andon measure` は問いを `questions.yaml` から読むので、行に要らない）。値を kanban が確かめず、自由な文字列にする（書き間違いが、印の無い答えとして後から見つかる）。
   細部:
   - 使い方は `kanban answer <id>#<n> [--decided-by owner|chief-of-staff|machine] "<答え>"`。
   - `kanban answer` は、`--decided-by` と、どの command にも付く旗のほかの旗を断る。今は知らない旗を黙って捨てるので（事実 14）、旗の名前の書き間違いが印の無い答えになる。断るのは `kanban answer` だけで、ほかの command の旗の読み方は変えない。
   - `questions.yaml` のキーは `decidedBy`（`askedBy`・`answeredBy` と同じ書き方）。`kanban show --json` と `kanban questions --json` は全部のキーを出すので、そのまま出る。
   - `owner` はオーナーが決めた答え、`chief-of-staff` は Chief of Staff が自分で決めた答え、`machine` は command が事実を見て閉じた答えである。新しい識別子なので、役の語は [ADR-0036](0036-owner-not-hito.md) の「オーナー」に合わせて `owner` にする。前からある識別子の `person` は変えない。
   - 直す場所: `cmd/kanban/questions.go` の `questionKeys`、`schema/ledger/questions.v3.schema.json`（2 つの一致を `cmd/kanban/schema_test.go` が見る）、`ledger.md` の「Added without a version change」の表と問いのキーの表と `kanban answer` の説明。

6. **2026-11-22 答えを書く者が、場面に合う値を書く。**（Chief Engineer が決めた）
   理由: 誰が決めたかを知っているのは、答えを書くその場の書き手だけである。
   細部:

   | 書き手 | 場面 | 値 |
   |---|---|---|
   | `andon waiting answer` | オーナーがペインで選択肢を選んだ | `owner` |
   | `andon issue settle`・`andon issue merge` | オーナーのマージや PR の head の動きを見て gate の問いを閉じる | `machine` |
   | Chief of Staff | オーナーがその問いに答えた言葉を写す | `owner` |
   | Chief of Staff | 範囲の問いに答える・委譲の中で自分で決める | `chief-of-staff` |
   | Chief of Staff | オーナーのマージや、ほかで決まったことを見て、要らなくなった問いを片付ける | `chief-of-staff` |
   | Chief Engineer | kanban に置いた自分の問いに、オーナーの答えを写す | `owner` |

   - `owner` を書くのは、オーナーがその問いに答えたときだけである。オーナーがマージしたのを見て問いを閉じるときは、マージがもう 1 回と数えられているので `chief-of-staff` にする（事実 6 の 2 件がこの場面）。
   - コードの 2 か所（事実 12）は、旗を必ず付ける。`closeQuestion` の形を変えると、書き手のソースの形を固定した検査（事実 7）が落ちるので、同じ PR で検査も直す。
   - skill の 4 行（事実 12）は、場面に合う値の旗を付けた形に直す。
   - `blocked.md` の「オーナーに聞くこと」に、決まったら `kanban answer <id>#<n> --decided-by owner "<決定>"` で kanban に残してから `hc send` で返す、と書き足す。今も Chief of Staff がしていることを手順に書く（事実 13）。Chief of Staff が委譲の中で自分で答えるときは `chief-of-staff` と書く。
   - Chief Engineer の skill には、kanban に置いた問いに答えが出たら `kanban answer --decided-by owner` と `kanban received` で閉じる、と 1 文を足す。同じ PR で [design.md](../design.md) §1.8 も直す。
   - gate の問いにオーナーが「担当に直させる」と答えたときの手順は変えない（やらないこと）。

7. **2026-11-22 判断は、印が `owner` の答えを、答えた時刻で数える。**（Chief Engineer が決めた）
   `decidedBy` が `owner` で、`answered` が期間の中の答えを、問い 1 件につき 1 回と数える。どの課題の問いかも、問いの種類も問わない。オーナーの課題に置いた問いも、Chief Engineer の課題の問いも数える。レビュアーの課題は、ほかの指標と同じく数えない。
   理由: Chief of Staff がオーナーに聞く問いは、PR を持たないオーナーの課題に置かれる。PR のある課題の問いだけを数えると、この口の答えを全部落とす。
   捨てた案: 問いの種類で絞る（印が `owner` なら、範囲の問いでもオーナーが決めている）。
   細部: 同じ判断が、担当の課題とオーナーの課題の両方に問いとして置かれる手順は無い（担当の問いと gate の問いは、オーナーの課題に写しを作らない）。

8. **2026-11-22 答えを、決めた者ごとに数えて出し、印の無い答えは介在回数に入れない。**（Chief Engineer が決めた）
   `answered` が期間の中の答えを、`decidedBy` の値ごとに数えて `efficiency.answers` に出す。`decidedBy` が無いか 3 つの値のどれでもない答えは `unmarked` に数え、介在回数には入れない。
   理由: 推し量って入れると、数が合っているかを確かめられなくなる。0 と「分からない」を分けるのは、`andon measure` のほかの数と同じ扱いである。旗の付け忘れは `unmarked` で見える。Chief of Staff が自分で決めた答えの数は、委譲が効いている量として、オーナーがそのまま読める。
   捨てた案: 印の無い答えを、出し手と種類から推し量って数に入れる。範囲の問いだけは印が無くても数から外す（規則を 1 つにして、付け忘れを全部見せる）。
   細部: この決定より前の答えには印が無いので、実装の直後の期間は `unmarked` が多く出る。過去の答えに印を後から付けることはしない。

9. **2026-11-22 問いを持たないオーナーの課題は、片付いたら 1 回と数える。**（オーナーが決めた）
   `who: person` の課題のうち、問いを 1 つも持たないものが、期間に最後の列へ動いたら、1 回と数える。
   理由: オーナーの判断の多くは、問いを通らずにオーナーの課題として片付いている。実測では、問いへの中身のある答えのうち灯りの種類は 4 件で、同じ期間に問いを持たないオーナーの課題は 14 件片付き、少なくとも 6 件はオーナーの判断を報告に書いていた（事実 6・8）。数えないと、判断の半分より多くを落とす。
   捨てた案: オーナーの課題を全部問いに通す（トリアージと Chief of Staff の手順を大きく変える）。数えずに、片付いた数を別の行にだけ出す（合計が実態より少ないままになる）。
   細部:
   - 問いを持つオーナーの課題は、問いの答えで数える（決めたこと 7・8）。課題そのものは数えない。二重に数えないためである。
   - 判断と手作業は分けない。どちらもオーナーの操作である。
   - Chief of Staff が、要らなくなったオーナーの課題を片付けたときも 1 回に入る。記録では見分けられないので、多めに出る側に倒す。
   - 識別子は、オーナー待ちの種類と同じ `task` にする。

10. **2026-11-22 PR の上のコメントと commit は、今は数えない。**（オーナーが決めた）
    理由: 担当もオーナーも同じアカウントで書くので、記録では誰のコメントかを見分けられない。実測の 1 件は担当のコメントで、オーナーのものは 0 件だった（事実 10）。数に入れると、オーナーが何もしていない PR に介在が付く。
    捨てた案: 混ざるのを承知で、手直しの記録の数をそのまま足す。担当のコメントに印を付ける直しを、この文書に含める（手直しの記録の拾い方は別の約束で、直し方が 2 つ以上ある）。
    細部: 手直しの記録が担当のコメントを除けるようになったら、種類を 1 つ足す小さな改めとして、この文書の下に足す。今ある「オーナーの手直しがあった課題」の行は変えない。

11. **2026-11-22 合計は期間に起きた操作の数にし、PR の行にはその PR の課題に付いた操作を出す。**（Chief Engineer が決めた）
    `total` は、決めたこと 3・7・9 の 3 つの数の和である。`per_pr` は `total` を、期間にマージされた PR の数で割る。`prs[]` の行の `human_touches` は、その PR のオーナーのマージ（0 か 1）と、その課題の問いの `owner` の答え（時期を問わない）の和である。
    理由: 一次の指標は「期間にラインが PR を 1 本出すのに、オーナーが何回動いたか」である。PR に結ばれない判断とオーナーの課題も、オーナーの操作である。
    捨てた案: 合計を PR の行の和にする（オーナーの課題と、そこに置いた問いの答えが、どの行にも入らずに消える）。
    細部: 判断は答えた時刻で、オーナーの課題は片付いた時刻で、マージは PR がマージされた時刻で期間に入る。そのため `prs[]` の行の和と `total` は一致しないことがある。マージされた PR が無い期間は、`per_pr` が `null` で、`total` は 0 でないことがある。

12. **2026-11-22 出力に内訳を足し、`andon measure` の出力のバージョンを 2 に上げる。**（Chief Engineer が決めた）
    `efficiency.human_touches` に `merge`・`decision`・`task` を足し、`efficiency.answers` を足す。`total` と `per_pr` は残す。`total` の意味が変わるので、`schema/measure.schema.json` の `$id` を 2 に上げる。
    理由: キーの意味を変えるときはバージョンを上げる決まりである（contract.md の原則 4）。内訳が無いと、数が減ったのがマージか判断か分からない。
    細部:

    ```json
    "human_touches": {"total": 136, "per_pr": 0.65, "merge": 118, "decision": 4, "task": 14},
    "answers": {"owner": 4, "chief_of_staff": 5, "machine": 106, "unmarked": 0}
    ```

    - 人向けの表の行は「PR あたりのオーナーの介在回数 0.65（136回 / 208本。マージ 118・判断 4・オーナーの課題 14）」の形にする。その下に「答えを決めた者 オーナー 4・Chief of Staff 5・command 106・印なし 0」の行を足す。
    - 同じ PR で直す文: [contract.md](../contract.md) §13 の例と「数え方」の表、[design.md](../design.md) §5.1 の数え方の括弧、[operate.md](../operate.md) の「オーナーの `by` の行の数」、[vision.md](../vision.md) §4 の「測る」の行の `by`、`schema/measure.schema.json` の `prs[].human_touches` の説明。vision.md §6 の問い 4 の行は当時の記録なので書き換えず、末尾に「数え方は ADR-0049 が改めた」と足す。
    - 同じ PR で直す検査: `tests/schema.sh` の `$id` と必須の表、`tests/andon-measure.sh`、`internal/measure/measure_test.go` の手で数えた数。

## やらないこと（範囲の外。理由）

- **端末で打った文と、Chief Engineer のダイアログへの答えを数えること。** 記録が無く、取るにはオーナーの発言を新しく記録することになる。入口でオーナーが範囲の外にした。
- **gate の問いにオーナーがマージのほかで答えた判断を数えること。** 今の手順は、オーナーが「担当に直させる」と答えても答えを kanban に書かず、問いは後で command が閉じる（事実 13）。書かせるには gate の問いに答える順を変えることになり、課題の状態の動きに触れる。実測では 107 件の gate の問いのうち多くて 1 件だった。数え落としとして残す。
- **オーナーの手直しの記録が、担当のコメントを拾うのを直すこと。** 事実 10。手直しの記録の拾い方は別の約束なので、この文書では変えない。Chief of Staff へ候補として渡す。
- **オーナーの手直しの記録が、command の閉じた答えを見分ける方法を替えること。** 今は答えの文の一致で見分けている（事実 7）。印の `machine` が入れば印で見分けられるが、同じく別の約束である。Chief of Staff へ候補として渡す。
- **dispatcher がペインの答えを拾う方法を替えること。** 今は `by` が端末でない答えを拾う。動いていて、この題の数え方に関わらない。
- **過去の答えに印を後から付けること。** 誰が決めたかを後から確かめられない。
- **旗を必須にすること。** kanban はほかの道具も使う一般の形式で、役を知らない。付け忘れは決めたこと 8 の `unmarked` で見る。
- **ダイジェストに介在回数を載せること。** 載せる節は [ADR-0044](0044-owner-digest.md) が決めている。数が合うようになったあとに、別に決める。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は今あるものを使う。kanban の CLI は `go test ./cmd/kanban`、数え方は `go test ./internal/measure` の手で数えた数と比べる検査（`TestMeasureMatchesHandCount`）、command の通しは `tests/andon-measure.sh`、約束の形は `tests/schema.sh` である。証拠は、走らせた command と、その出力である。

| # | 条件 | 証拠の形 |
|---|---|---|
| 1 | `kanban answer <id>#<n> --decided-by owner "<答え>"` が `questions.yaml` に `decidedBy: owner` を書き、`kanban show --json` の問いにも出る。旗が無ければキーを書かない。3 つの値のどれでもない値と、名前の違う旗（`--decided_by` など）は断り、何も書かない | `go test ./cmd/kanban` の出力 |
| 2 | `andon waiting answer` が書いた答えに `decidedBy: owner` が付く | `go test ./internal/waiting` の出力 |
| 3 | settle と merge が閉じた gate の問いの答えに `decidedBy: machine` が付く。オーナーの手直しの記録が、その答えを今までどおり拾わない | `go test ./internal/issue ./internal/dispatcher` の出力 |
| 4 | 手で数えた数と合う。見本には次を全部入れる: dispatcher がマージした PR、GitHub でマージされた PR、印が `owner`・`chief-of-staff`・`machine` の答え、印の無い答え、3 つの値のどれでもない印の答え、期間の外の `owner` の答え、PR の無いオーナーの課題の `owner` の答え、問いを持たずに期間に片付いたオーナーの課題、問いを持って片付いたオーナーの課題、期間の外に片付いたオーナーの課題、PR のコメントを持つ手直しの記録、`by` が端末でない行。`merge`・`decision`・`task`・`total`・`per_pr`・`answers` と `prs[]` の行が、手で数えた数と一致する | `go test ./internal/measure` の出力 |
| 5 | `andon measure --json` の出力が、バージョン 2 の schema に通る。contract.md §13 の例も通る。人向けの表に、内訳の行と、答えを決めた者の行が出る | `tests/run --changed` の表の `andon-measure.sh` と `schema.sh` の行 |
| 6 | worktree で作った `andon` で、セルの本物の kanban に `andon measure --json` を打つ（読むだけ）。`human_touches.merge` が `merged_prs` から `auto_merge.count` を引いた数と等しく、マージされた PR があれば `total` が 0 でない | 打った command と、出力の `efficiency` の写し |
| 7 | `package/skills` の `kanban answer` を打たせる行が、全部 `--decided-by` を持つ。当たる行が 6 行以上あることも同じ検査で確かめる。`blocked.md` の「オーナーに聞くこと」の節に `kanban answer` の行がある | `bash tests/package.sh` の出力 |

## 価値の仮説

1. **誰の何が変わるか。** オーナーが、委譲を広げる前と後で、自分の操作の数を種類ごとに比べられる。振り返りも、同じ数を委譲を広げる根拠に使える。
2. **見る信号。** オーナーの目視。見るものは、`andon measure` の人向けの表の「PR あたりのオーナーの介在回数」の行と「答えを決めた者」の行、`--json` の `efficiency.human_touches` と `efficiency.answers`。
3. **見る日。** 2026-12-02。実装がセルに当たってから、印の付いた答えが 7 日ぶん溜まる日である。
4. **誰が要るか。** オーナー（表を読む）。Chief of Staff（答えに印を書く）。
5. **効いたとどう分かるか。** 見る日の 7 日の数で、介在回数が 0 でなく、マージの数が「マージされた PR の数から自動マージの数を引いた数」と等しい。印の無い答えが、command の答えを除いた答え（`owner`・`chief_of_staff`・`unmarked` の和）の 2 割以下である。オーナーが数を見て、実感と合うと言う。
6. **何があれば止めるか。** 印の無い答えが、command の答えを除いた答えの半分を超えたままである。そのときは問いの答えを数えるのをやめ、マージとオーナーの課題だけの数に戻す。オーナーが数を実感と合わないと言ったときも、数え方を見直す。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **`andon measure` の数え方を改める。** 決めたこと 2・3・7〜9・11・12。`internal/measure`、`schema/measure.schema.json`、決めたこと 12 の文と検査、この ADR の索引。受け入れ条件 4・5・6。依存は無い。印の付いた答えがまだ無くても、マージとオーナーの課題はこの 1 本で数に出る。
2. **kanban の答えに印を足し、コードの書き手に付ける。** 決めたこと 5 と、決めたこと 6 のコードの分。`cmd/kanban`、`schema/ledger/questions.v3.schema.json`、`internal/waiting`、`internal/issue`、`internal/dispatcher` の検査、ledger.md、contract.md の問いの節。受け入れ条件 1・2・3。依存は無い。1 と並べて進められる。
3. **skill の `kanban answer` に印を付ける。** 決めたこと 6 の skill の分。`package/skills/ask-one-by-one`、`package/skills/chief-of-staff/playbooks`、`package/skills/chief-engineer` と design.md §1.8。受け入れ条件 7。2 がセルに当たったあとに当てる。旗を知らない kanban は旗を黙って捨てるので、先に当てると印の無い答えが書かれる（事実 14）。

## 開いている問い（残った問い。決断の種類つき）

無し。

## 出典（repo の中の文書・外の URL・オーナーの発言の日付だけ）

- オーナーの発言 2026-11-22（入口の言い直しと前提 4 つへの同意と数える範囲の答え。出口の、PR の上のコメント・オーナーの課題・答えの印の 3 つの答えと「決めた」）。
- [vision.md](../vision.md) §1 の特徴 3・§4 の「測る」の行・§6 の問い 4（一次の指標。オーナーの答え 2026-11-07）。
- [design.md](../design.md) §1.9 の (b)（オーナーの手直しが起きる所）と §5.1（数える指標）。
- [contract.md](../contract.md) §5（問いの JSON）・§13（流れの数え）・§14（オーナーの手直し）。
- [ledger.md](../ledger.md)（kanban の形式。問いのキーと互換の規則）。
- [dispatcher.md](../dispatcher.md) §2.2（通知を拾う元）・§2.6・§2.7。
- [decisions/2026-11-09-measure-flow-metrics.md](../decisions/2026-11-09-measure-flow-metrics.md)（`andon measure` は読むだけで、新しい記録を足さない。決めたこと 4 が 1 点改める）。
- [decisions/2026-11-09-corrections-json.md](../decisions/2026-11-09-corrections-json.md)（手直しの記録が、同じアカウントのコメントをどう拾うか）。
- [ADR-0036](0036-owner-not-hito.md)（役の語）・[ADR-0042](0042-owner-waits-do-not-stop-the-line.md)（オーナー待ちの 3 種とペインの答え）・[ADR-0044](0044-owner-digest.md)（介在回数が 0 と出たことを見つけた文書）。
- 実測: `andon measure`、`gh pr list --state merged --json mergedBy`、kanban の `log.jsonl`・`issue.yaml`・`questions.yaml`・`corrections.json` と通知の記録の集計（2026-11-22）。別のコンテキストの反論（読むだけ）で、事実 6・8・10・13・14 と決めたこと 3・6・9・10 を直した。
