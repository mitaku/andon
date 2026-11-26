# ADR-0066: 成果が報告で PR が無くてもよい課題に、予約キー deliverable の値 report を足す（仕様・設計。2026-11-25）

- 状態: **決定（2026-11-26）**。オーナーが 2026-11-26 に「決めた」と答えた。実装は「依頼書に切る案」のとおり別の課題（切るのは Chief of Staff）。覆すときは新しい ADR を書く。
- 主題: PR を出さずに終えてよい課題（当てたあとの検証・点検・マージ後のレビュー）を、切った時点の印で機械に見せる。印の形、書き手と書く時、読む所ごとの振る舞い、履歴の照合、印の無い前の課題の読み方を決める。予約キーの値を 1 つ足すので、`schema/` と [contract.md](../contract.md) が変わる。
- 語は [design.md](../design.md) §0.1。機械が読む形の正は [contract.md](../contract.md) と `schema/`。この文書が足す約束は「決めたこと」に形を書き、実装の課題が写す（それまでは提案）。予約キー `deliverable` は [ADR-0006](0006-survey-fix-decide.md) 決めたこと 7・17 が決めた。この文書はその値を 1 つ足し、決めたこと 7・17 は覆さない。
- この文書で「報告の課題」は、予約キー `deliverable` が `report` の課題を指す。「PR の課題」は `deliverable` が `pr`（既定）の課題を指す。

## 決定の索引

| # | 何を決めたか | 今の正 | 今 |
|---|---|---|---|
| 1 | 範囲は、やめの見分け・gate の PR 無しの扱い・検証の雛形を同じ印にそろえ、見分けの表を 1 つ置くまでにする | `docs/contract.md` | 生きている |
| 2 | 印は予約キー `deliverable` の値 `report` で持ち、別のキーは作らない | `schema/issue-keys.schema.json` | 生きている |
| 3 | `report` の意味は「成果は報告で、PR は無くてもよい。出した PR は PR の課題と同じ道を通る」にする | `docs/contract.md` | 生きている |
| 4 | 書くのは Chief of Staff で、課題を切るときに書く。片付ける前ならあとからも書ける | `package/skills/chief-of-staff/playbooks/brief.md` | 生きている |
| 5 | 書く課題は、検証・点検・マージ後のレビュー・報告だけを求める repo のある課題にする | `package/skills/chief-of-staff/playbooks/brief.md` | 生きている |
| 6 | 報告の課題は、待つ相手がやめになったかの見分けから外す | `internal/dispatcher/abandoned.go` | 生きている |
| 7 | 報告の課題が PR 無しで返ったら、commit を見ずに gate を回さない。PR の課題の見分けは今のままにする | `internal/issue/merge.go` | 生きている |
| 8 | 手で打った `andon gate` と `andon issue ready` は、PR の無い報告の課題に PR を求める文を出さない | `internal/gate/gate.go`・`internal/issue/issue.go` | 生きている |
| 9 | 起動の前に求める予約キー・レビュアーの起動・オーナーのマージの片付け・`andon issue finish`・結果の記録は変えない | この ADR | 生きている |
| 10 | 値 `report` は、書いた時点の Chief of Staff が書いたときだけ信じる。起き直しのあとの打ち直しは要らない | `internal/ledger/ledger.go` | 生きている |
| 11 | 印の無い課題と知らない値は、今までどおり `pr` として読む。前の課題を移す作業はしない | `internal/ledger/ledger.go` | 生きている |
| 12 | 見分けの表の正は contract.md §2 に小さい節で置き、コードは値の読み方を 1 つの関数にまとめる | `docs/contract.md` | 生きている |
| 13 | このテーマでは statechart のモデルを書かない | この ADR | 生きている |

## 問題（オーナーの言葉で。言い直しの段落）

オーナー（2026-11-25。待つ相手がやめになったと知らせる句を直す課題の担当が立てた問い「PR を出さずに終える課題を完了と読むために、課題に『PR を出さない』印を予約キーで持たせるか」への答え）: 「推奨案にしましょう」。推奨案は「印を足す。Chief of Staff が検証などを切るときに書き、dispatcher はその課題を待つ相手のやめの見分けから外す。設計は別の課題で決める」だった。

言い直し（オーナーが 2026-11-25 に同意した）。PR を出さずに終える課題は、記録の上では「やめた課題」と同じ形になる。そのせいで、後ろの課題が引き取れるようになったときに「待つ相手がやめになった」と誤って知らされた。少し前には、同じ理由で gate が検証の担当に push を求めた。この 2 つは別々の課題で、別々の見分け方をオーナーに聞いた。オーナーは、印を予約キーで持たせると決めた。このテーマでは、その印の形と、誰がいつ書くかと、どこが読むかを決めて、見分け方を 1 か所にそろえる。

モデル: 調べなかった（決めたこと 13。PR の無い課題は repo のモデルの外にあり、課題の状態の定義も、状態を動かす command の順も、dispatcher とレビューの順も変えない。変えるのは、通知に添える句の条件と、`review` に入った PR の無い課題に gate を回さない条件の 1 項だけである）

## 読んで分かった事実（2026-11-25。main `2102d401` と、Andon を開発しているセルの kanban で実測）

数は、数えた時点のものである。確定の前に、main `bf0a6368` を読み直した。その間に入った変更は、予約キーにも、ここに書いた見分けにも触れていない。

1. **「PR を成果にする課題か」を、4 か所が別々の式で見分けている。** dispatcher が gate を回す課題の条件（`internal/dispatcher/dispatcher.go` の `gated`）、待つ相手がやめになったかを見分ける課題の条件（`internal/dispatcher/abandoned.go` の `yieldsPR`）、起動の前に求める予約キーの条件（`internal/issue/start.go` の `requiredSettings`）、`andon issue ready` の分かれ目（`internal/issue/issue.go`）である。4 か所とも、`deliverable` の値を自分で `candidates` と比べている。repo と役の条件は読み手ごとに違う（`gated` は Chief Engineer の課題を除き、`yieldsPR` はオーナーの課題を除き、`andon issue ready` は repo を見ない）。手で打った `andon gate`（`internal/gate/gate.go`）と、当てたあとの検証の道具（`.claude/skills/verify-andon/scripts/journeys/gate.sh`）も同じ比べ方を持つ。
2. **`review` に入った PR の無い課題は、5 つ目の式で見分けている。** `andon issue merge` は、`report.json` に `pr` が無く、報告の commit が origin の既定の branch に入っている課題に gate を回さない（PR #630。`internal/issue/merge.go` の `mergeOne`）。自分の commit があるのに `pr` が無い課題には、gate が「`report.json` に pr が無い」の fix を担当へ送る。この見分けは、オーナーが 2026-11-25 に「`deliverable` の値を足さずに、自分の commit が無いことで見分ける」と選んだものである。
3. **commit での見分けは、直す課題が直さずに返す場合にも効いている。** PR の課題の担当が、直さないと分かって branch を main の先頭に戻し、PR 無しで `kanban review` で返すと、gate は回らず、通知 `report` で Chief of Staff に届く。待つ相手がやめになったと知らせる句を直す課題そのものが、この形で返った。
4. **commit での見分けには、穴が 1 つ残っている。** マージ後のレビューの担当が、見る PR の head を自分の worktree で checkout して報告すると、報告の commit は既定の branch に入っていない（squash マージでは PR の head の commit が main に入らない）。gate は push を求める fix を送る。PR #630 の担当が、残したこととして報告している。
5. **待つ相手がやめになったかの見分けは、`pr` が無いことをやめと読む。** 結果の記録（`outcome.json`）の `result` が `merged` でなければやめ、`report.json` に `pr` が無ければやめである（`internal/dispatcher/abandoned.go` の `mergedState`）。repo の無い課題・調べる課題・レビュアーの課題・オーナーの課題は、見分けの外にある。
6. **完了とやめは、今ある記録では分けられない。** 句を直す課題の担当が、kanban の履歴で `done` の直前が `review` なら完了と読む直しを書き、本物の記録で両向きの反例を見つけて取り下げた。やめなのに完了と読む例は、本文を畳むのをやめて `review` で返してから閉じた課題である。完了なのにやめと読む例は、`review` のあと担当が `doing` に戻し、その間に片付いた検証の課題である。どちらも結果の記録は `none`、`pr` は無く、報告の commit は main の上にある。
7. **誤った句は、これまでに 2 回付いた。** セルの kanban には課題が 1,078 件あり、待つ関係（`after`）は 433 ある。待つ相手が「PR の課題で、PR 無しで片付いたもの」だったのは 3 回だった。2 回は検証の課題で、完了しただけなのに句が付いた（バージョンのタグを打つ課題が、2 つの検証の課題を待っていた）。1 回は正しいやめだった。
8. **検証の課題は、切る時点では PR を出すかどうかが決まらない。** 片付いた検証の課題は 49 件ある。31 件は PR 無しで返り、18 件は検証の地図を直す PR を出してマージされた。検証の雛形（`package/skills/chief-of-staff/playbooks/verify.md`）は、地図を直したら PR、直さなければ `kanban review` で返すと書いている。
9. **PR 無しで片付いた課題 570 件のうち、repo があって見分けの中にあるのは 37 件である。** 内訳は、レビュアーの課題が 471 件、repo の無い課題が 50 件、調べる課題が 10 件、Chief Engineer の課題が 2 件で、ここまでは見分けの外か別の役である。残りの 37 件は、検証が 31 件、そのほかが 6 件だった。6 件のうち、やめが 3 件、報告だけを求めた課題が 1 件、既に直っていたと分かった課題が 1 件、Chief of Staff の手の課題が 1 件である。マージ後のレビューの課題は 1 件あり、担当がほかの課題の PR を報告に書いたため、マージ済みとして記録されている。
10. **PR 無しで返った課題に commit が残っていても、片付けで失われない。** `andon issue finish` は、branch が origin の既定の branch に入っておらず、マージ済みの PR の head とも違うとき、branch を消さずに残し、そのことを 1 行出す（`internal/issue/finish.go`）。
11. **ルーチンは予約キーを書けない。** kanban のルーチンが課題に書くのは、タイトル・依頼書・プロファイル・repo・ルーチンの名前だけである（[ledger.md](../ledger.md) の Routines）。点検のルーチン（[ADR-0047](0047-idle-slots-run-upkeep.md) 決めたこと 18 の `check-isolation` と `check-verify`）が足す課題は、repo があり、PR を出さずに結果を報告して返す。
12. **今、印の無い検証の課題を待っている課題は 0 件である。** 片付いていない課題の待つ相手を全部引いた。repo のある検証の課題を待つものは無かった。
13. **`deliverable` が `candidates` の課題は、履歴の照合が外れていても調べる課題として読む。** [ADR-0006](0006-survey-fix-decide.md) 決めたこと 17 が決めた。理由は、Chief of Staff が起き直すと予約キーを書き直すまで照合が外れることと、`candidates` はマージに進まない側にあることである。

14. **書いた時点の Chief of Staff と比べる照合が、既にある。** 印 `later` は、キーを最後に変えた行の書き手を、その行の時点の Chief of Staff の端末と比べる（[ADR-0062](0062-later-owner-waits.md) 決めたこと 3。`internal/ledger/ledger.go` の `TrustedThen`）。Chief of Staff が起き直しても打ち直しが要らない。今の端末と比べる照合（`Trusted`）は、起き直したら開いている課題の予約キーを打ち直す決まりと組になっていて、片付いた課題の印には使えない（打ち直すのは開いている課題だけである）。
15. **担当が予約キーを書くことを、機械は止めていない。** 担当が `kanban set` を打たないのは skill の決まりだけで、断る設定は無い。値だけで読むキーは、担当が書いても効く。
16. **`andon issue report` は、前の `pr` を引き継ぎ、外す口が無い。** `--pr` が空なら前の報告の `pr` をそのまま書く（`internal/issue/issue.go`）。外れるのは、PR の branch が課題の branch と違うときだけである。PR を出したあとで直しをやめて PR を閉じた課題は、`review` に入るたびに gate が「PR が open でない」の fix を返す。これは今の PR の課題でも同じである。片付いた検証の課題 49 件に、PR を閉じて終えたものは無い。

## 前提（同意済み。番号）

オーナーが 2026-11-25 に、5 つとも同意した。

1. 困りごとの芯は、「この課題は PR を成果にしない」という切った時点の意図を、機械が知る手段が無いことである。commit や kanban の履歴から推し量る方法では分けられない（事実 6）。
2. 何もしないと、検証の課題を待つ課題が引き取れるようになるたびに、誤った句が付き続ける。ただし頻度は低い（事実 7）。
3. 検証の課題は、切る時点では PR を出すかどうかが決まらない（事実 8）。印の意味は「PR を出さない」ではなく、「成果は報告で、PR は無くてもよい」になる。
4. 既にあるもので済む部分がある。gate が push を求めない直し（PR #630）は捨てない。repo の無い課題と調べる課題は既に見分けの外にあり、触らない。
5. 配り方は、Chief of Staff が課題を切るときに `kanban set` で印を書く形である。検証の雛形に 1 行足すので、オーナーの手は増えない。印の無い前の課題は、今までどおりに読む。

## 決めたこと（番号・日付・誰が決めたか・理由・捨てた案。あとから覆したら上書きせず下に足す）

1. **2026-11-25 範囲は、やめの見分け・gate の PR 無しの扱い・検証の雛形を同じ印にそろえ、見分けの表を 1 つ置くまでにする。直す課題が直さずに返す場合の、commit での見分けは今のまま残す（オーナーが決めた）。**

   理由: 同じ見分け方を 2 つの課題が別々にオーナーへ聞いた。読む所ごとに式を足す形のままだと、次の読み手でも同じ問いが立つ。commit での見分けは、PR の課題が直さずに返す場合（事実 3）に効いていて、印では代われない。直さないと分かるのは返すときで、切るときには書けないためである。
   捨てた案:
   - 句だけを直す（印を読むのは、やめの見分けだけにする）。変更は一番小さいが、マージ後のレビューの穴（事実 4）が残り、見分け方は 2 か所に分かれたままになる。
   - 印だけにそろえ、commit での見分けをやめる。PR の課題が PR 無しで返ったら必ず差し戻すことになり、直さずに返すための返し方を新しく作る必要がある。オーナーが PR #630 の問いで選んだ見分けを覆す。
   - 検証の課題から PR を無くす（報告だけにして、地図の直しは別の課題に切る）。印は「PR を出さない」の意味そのままになるが、検証 49 件のうち 18 件で課題が 1 つ増え、検証の今の形（[ADR-0035](0035-pstack-correct-and-verification-skill.md)）を変える。

2. **2026-11-25 印は予約キーで持つ（オーナーが決めた）。形は、予約キー `deliverable` の 3 つ目の値 `report` にし、別のキーは作らない（Chief Engineer が決めた）。**

   `deliverable` の値は `pr`（既定）・`candidates`・`report` の 3 つになる。
   理由: `deliverable` は「課題の成果の形」を言うキーで、[ADR-0006](0006-survey-fix-decide.md) 決めたこと 7 は値の名前を型でなく成果で付けた。「成果は報告」は同じ軸の 3 つ目の値である。別のキーにすると、`deliverable: pr` と「PR を出さない」が同時に書けて、食い違う組み合わせができる。読み手も 2 つのキーを見ることになる。
   捨てた案:
   - 別の予約キー（PR が要るかどうかだけを言うキー）。上の理由。
   - タイトルの「検証: 」で見分ける。オーナーが印を予約キーで持つと決めた。タイトルは人が読む文で、マージ後のレビューや点検には別の式が要る。
   - 値を 2 つ足す（PR を出さない値と、PR を出してもよい値）。使う課題のほとんどが検証で（事実 9）、検証は切る時点で決まらない（事実 8）。PR を出さないと言い切る値を使う課題は、今 1 件しか無い。

3. **2026-11-25 `report` の意味は「成果は報告で、PR は無くてもよい。出した PR は PR の課題と同じ道を通る」にする（Chief Engineer が決めた。前提 3 としてオーナーが同意した）。**

   報告の課題の担当は、直すものが無ければ PR を出さず、`andon issue report` のあと `kanban review` で返す。依頼書が許す直し（検証なら検証の地図）を出すなら、PR の課題と同じ手順（レビュー・`andon issue ready`・gate・マージ）を通る。
   理由: 事実 8。PR を出した 18 件は、今も PR の課題と同じ道を通ってマージされている。その道を変える理由が無い。
   捨てた案: `report` の課題には PR を出させない（決めたこと 1 の 3 つ目の捨てた案と同じ）。

4. **2026-11-25 書くのは Chief of Staff で、課題を切るときに書く。片付ける前ならあとからも書ける（Chief Engineer が決めた）。**

   細部:
   - 依頼書の Settings に `Deliverable: report` を書き、Chief of Staff の端末で `kanban set <id> deliverable=report` も打つ（正は `issue.yaml`。調べる課題の `candidates` と同じ二度書き）。
   - ルーチンが足した点検の課題は、Chief of Staff が起動の前に `kanban set <id> deliverable=report` を打つ（事実 11）。
   - 担当は自分で書き換えない（今の `deliverable` の決まりと同じ）。担当が書いた `report` は、決めたこと 10 により読まれない。
   - Chief of Staff が起き直しても、`report` の打ち直しは要らない（決めたこと 10）。
   - 切ったあとで成果が報告になったと Chief of Staff が判断した課題は、片付ける前に Chief of Staff が書いてよい。待つ相手がやめになったかの見分けは、後ろの課題が引き取れるようになったとき（待つ相手が `done` になったあと）に読むので、片付ける前に書けば効く。
   理由: ほかの予約キーと同じ書き手にする。意図を知っているのは切る役である。
   捨てた案:
   - 担当が返すときに書く。担当が自分の課題を「やめではない」と言えることになり、意図の印でなくなる。
   - kanban のルーチンの定義に予約キーを書けるようにする。kanban の形式を変える話で、点検のルーチンは今 2 つしか無い。

5. **2026-11-25 `report` を書く課題は、当てたあとの検証・点検のルーチンが足した課題・マージ後のレビュー・報告だけを求める repo のある課題にする（Chief Engineer が決めた）。**

   書かない課題:
   - 直す課題。直さずに返すかもしれないと分かっていても書かない。直さずに返った直す課題は、後ろの課題から見て「待っていた PR が来なかった」ので、句が付くのが正しい。
   - 調べる課題（`candidates` のまま）。候補の一覧が成果で、読み手が違う。
   - repo の無い課題・Chief Engineer の課題・レビュアーの課題・オーナーの課題。今も見分けの外にあるか、役で見分けている。
   理由: 事実 9 の 37 件のうち、完了なのに PR が無いのは、検証と、報告だけを求めた課題である。
   捨てた案: repo のある PR 無しの課題を全部 `report` に寄せる（調べる課題も）。`candidates` は `andon issue ready` が候補の一覧を確かめる別の約束を持つ。

6. **2026-11-25 報告の課題は、待つ相手がやめになったかの見分けから外す（オーナーが決めた。問いの推奨案のとおり）。**

   引き取れるようになった課題の待つ相手が報告の課題なら、dispatcher は見分けない。通知 `ready` の `abandoned[]` にも `unchecked[]` にも入れない。報告の課題が PR を出していて、その PR が閉じられていても同じである。
   理由: 報告の課題の `done` は、報告を Chief of Staff が読んで片付けたことを言う。成果は報告で、PR のマージではない。今の決まり「repo の無い課題と調べる課題の `done` は成果の出来上がりなので、見分けない」に、報告の課題を足す。
   捨てた案:
   - `report.json` があれば出来上がり、無ければやめと読む。起動する前に閉じた報告の課題を拾えるが、調べる課題と repo の無い課題には無い見分けを 1 つ足すことになる。閉じたのは Chief of Staff 自身で、後ろの課題を起動するのも Chief of Staff である。
   - kanban の履歴で `done` の直前が `review` なら完了と読む。事実 6 の反例がある。

7. **2026-11-25 報告の課題が `review` に入り、`report.json` に `pr` が無ければ、`andon issue merge` は commit を見ずに gate を回さない。PR の課題の見分けは今のままにする（オーナーが決めた。範囲の問いの答え）。**

   細部:
   - 報告の課題で `pr` が無い: gate を回さず、fix を送らず、merge の記録にも書かない。返ったことは、今と同じく通知 `report` で Chief of Staff に届く。報告の commit が既定の branch に入っているかは見ない。
   - 報告の課題で `pr` がある: PR の課題と同じに gate を回す。出した PR を閉じても `pr` は報告に残るので（事実 16）、PR の課題と同じく fix が返る。
   - PR の課題で `pr` が無い: 今のまま。報告の commit が既定の branch に入っていれば gate を回さず、入っていなければ fix を送る。
   理由: 「PR は無くてもよい」課題に、PR を求める fix を送らない。マージ後のレビューの穴（事実 4）がふさがる。報告の課題の担当が直しを commit して PR を出し忘れた場合は、gate の fix では拾わなくなる。その commit は片付けで失われず、branch が残ったことが 1 行出る（事実 10）。報告も Chief of Staff が読む。
   捨てた案: 報告の課題でも commit を見る（自分の commit があれば fix）。PR を出し忘れた直しを gate が拾えるが、事実 4 の穴が残り、印を読む意味が無くなる。

8. **2026-11-25 手で打った `andon gate` と `andon issue ready` は、PR の無い報告の課題に PR を求める文を出さない（Chief Engineer が決めた）。**

   細部:
   - `andon gate <id>`: 報告の課題で `report.json` に `pr` が無ければ、回さずに終了コード 3 と理由 1 行（「`deliverable: report` の課題で PR が無い。gate を回すものが無い」）。記録は書かない。`pr` があれば今までどおり回す。
   - `andon issue ready <id>`: 報告の課題で `report.json` に `pr` が無ければ、今までどおり断る。理由の文を「`deliverable: report` の課題で PR が無いときは `kanban review <id>` で返す」に替える。`pr` があれば PR の課題と同じ確かめをする。
   - 値が `report` なのに照合が外れている課題（決めたこと 10）は、どちらの command も PR の課題として扱い、理由の文に「`deliverable: report` を書いたのが Chief of Staff でない」を添える。
   理由: 今の文は、どちらも「`andon issue report --pr` で書く」と案内する。マージ後のレビューの担当が、gate の fix に添えられた同じ案内に従って、ほかの課題の PR を報告に書いた（事実 9）。
   捨てた案: `andon issue ready` が、PR の無い報告の課題を通して `review` にする。PR の無い課題は `kanban review` で返すという今の手順（観察・トリアージ・振り返り・ダイジェスト・検証の手引きが同じに書いている）を、報告の課題だけ変えることになる。

9. **2026-11-25 次のものは変えない（Chief Engineer が決めた）。**

   - 起動の前に求める予約キー（`andon issue start`）: 報告の課題にも、PR の課題と同じキーを求める。PR を出すことがあり、そのとき gate とレビューが読むためである。
   - dispatcher がレビュアーを起動する条件と、`andon issue merge` を打つ課題の条件: 報告の課題は PR の課題と同じ側に置く。
   - オーナーのマージの片付け（`andon issue settle`）: `report.json` に `pr` があるときだけ動く今のまま。
   - `andon issue finish` と、結果の記録（`outcome.json`）の `result`: 今のまま。PR の無い報告の課題の `result` は `none` である。
   - 通知の種類と宛先。
   理由: 報告の課題が PR を出したときの道は、今の PR の課題の道で足りている（事実 8 の 18 件）。

10. **2026-11-25 値 `report` は、キーを最後に変えた行の書き手が、その行の時点の Chief of Staff の端末だったときだけ信じる。照合が外れた `report` は `pr` として読む。Chief of Staff が起き直しても打ち直しは要らない（Chief Engineer が決めた）。**

    照合は、印 `later` と同じ読み方（事実 14）を使う。やめの見分けも、gate を回さない分かれ目も、手で打った command も、同じ照合で読む。`candidates` の読み方（[ADR-0006](0006-survey-fix-decide.md) 決めたこと 17。値だけで読む）は変えない。
    理由: 値だけで読むと、PR の課題の担当が自分で `report` を書いて、やめた課題から句を消せる（事実 15）。決めたこと 4 が捨てた「担当が返すときに書く」と同じことが起きる。今の端末と比べる照合は使えない。やめの見分けが印を読むのは課題が片付いたあとで、片付いた課題の予約キーは起き直しのあとに打ち直されないためである。書いた時点の Chief of Staff と比べれば、起き直しをまたいでも外れず、ADR-0006 の決めたこと 17 が避けた「起き直しの間だけ PR を求められる」も起きない。照合が外れたときに `pr` として読むのは、今の動きに戻るだけで、マージの安全は変わらない（`report.json` に `pr` があれば、gate は今と同じに予約キーの照合とレビューを見る。`pr` が無ければ dispatcher はマージしない）。
    捨てた案:
    - 値だけで読む（`candidates` と同じ）。上の理由。
    - 今の端末と比べる照合で読む。片付いた課題の印が、起き直しのたびに外れる。

11. **2026-11-25 印の無い課題と知らない値は、今までどおり `pr` として読む。前の課題を移す作業はしない（Chief Engineer が決めた）。**

    細部:
    - 前のバージョンの andon は `report` を知らない値として `pr` に読む。印を書いた課題を前のバージョンが読んでも、gate と句は今と同じ動きになる。ただし前のバージョンの `andon issue start` は、依頼書の写しの `Deliverable: report` を `Deliverable: pr` に書き直す。印を書くのは、実装がセルに当たったあとにする。
    - 片付いた前の検証の課題には印が無い。その課題を待つ課題が、あとから引き取れるようになると、今までどおり句が付く。今そのような課題は 0 件である（事実 12）。
    - 実装がセルに当たったときに動いている検証の課題があれば、Chief of Staff が `kanban set <id> deliverable=report` を打つ。検証の課題は同時に 1 つしか動かない。
    理由: 黙って既定を変えない。移す対象が無い。

12. **2026-11-25 見分けの表の正は [contract.md](../contract.md) §2 に小さい節で置き、コードは値の読み方を `internal/ledger` の 1 つの関数にまとめる（Chief Engineer が決めた）。**

    細部:
    - contract.md §2 の予約キーの表の下に小さい節を置き、下の表（値ごと・読み手ごとの振る舞い）を写す。予約キーの表の `deliverable` の行は、この節を指す。[dispatcher.md](../dispatcher.md) の §2.2 と P6a の表、手引き、skill も、この節を指す。
    - 事実 1 の読み手と `mergeOne` は、`deliverable` の値を自分で比べず、`internal/ledger` の関数（kanban の履歴と課題から、照合を済ませた成果の形を返すもの）を呼ぶ。repo と役での見分け（レビュアー・Chief Engineer・オーナーの課題）は、今までどおり読み手が持つ。

    | 読み手 | `pr`（既定） | `candidates` | `report` |
    |---|---|---|---|
    | 起動の前に求める予約キー（`andon issue start`） | `must_fix`・`merge` とレビューのキーを求める | 求めない | `pr` と同じ |
    | `andon issue ready` | PR とレビューを確かめる | 候補の一覧を確かめる | `pr` があれば `pr` と同じ。無ければ断り、`kanban review` で返すと案内する |
    | dispatcher がレビュアーを起動する・`andon issue merge` を打つ | する | しない | する |
    | `review` に入って `pr` が無いとき（`andon issue merge`） | 報告の commit が既定の branch に入っていれば gate を回さない。入っていなければ fix | gate を回さない | commit を見ずに gate を回さない |
    | 手で打った `andon gate` | 回す | 回さずに終了コード 3 | `pr` があれば回す。無ければ回さずに終了コード 3 |
    | 待つ相手がやめになったかの見分け | 見分ける | 見分けない | 見分けない |
    | 照合 | 要らない（既定） | 見ない（値だけで読む） | 書いた時点の Chief of Staff が書いたときだけ信じる。外れたら `pr` として読む |

    理由: 振り返りが、同じ見分け方を 2 つの課題が別々に聞いたことを拾い、決めた見分け方を contract.md の `deliverable` の行に書くことを直し方に挙げた。式が読み手ごとに在ると、値を足すたびに直し漏れが出る。

13. **2026-11-25 このテーマでは statechart のモデルを書かない（Chief Engineer が決めた）。**

    理由: PR の無い課題の一生は、repo のモデルの外にある（[ADR-0025](0025-formal-model-in-repo.md) やらないこと 5）。課題の状態の定義、状態を動かす command の順、dispatcher とレビューの順は変えない。決めたこと 7 は、既にある「gate を回さない」分かれ目の条件に 1 項を足す。実装の PR は、この分かれ目を書いた注釈（`model/IssueLife.tla` の gate の fix の注釈・`model/issue-map.txt` の `kanban review` の行・`process/states/gate.yaml` の「PR を出さない課題だった」の行）を、新しい条件に合わせて直す。規則と控えは変わらない。

## やらないこと（範囲の外。理由）

- **commit での見分け（PR #630）をやめること。** 決めたこと 1。PR の課題が直さずに返す場合に要る。
- **直す課題が直さずに返ったことを、やめと分ける印。** 後ろの課題から見れば、待っていた PR が来なかったので、句が付くのが正しい。Chief of Staff は、成果が報告になったと判断すれば決めたこと 4 で書ける。
- **Chief Engineer の課題が文書を書かずに返った場合の句。** 今も見分けの中にあり、PR が無ければ句が付く。待たれる例がほとんど無い。要るときは、Chief of Staff が片付ける前に `deliverable=report` を書けば同じ印で外れる。
- **`andon issue report` に `pr` を外す口を足すこと。** 事実 16。PR を出したあとで直しをやめた報告の課題は、PR を閉じても PR 無しで返し直せない。今の PR の課題と同じ穴で、検証の課題 49 件に例が無い。そうなったら、担当が報告にそう書き、Chief of Staff が `doing` のまま `andon issue finish` で片付ける。
- **`candidates` の照合の読み方を変えること。** [ADR-0006](0006-survey-fix-decide.md) 決めたこと 17 のまま。
- **結果の記録（`outcome.json`）と `andon measure`・振り返り・ダイジェストの数え方の変更。** どれも `result` の `none` をやめと読んでいない。
- **kanban のルーチンの定義に予約キーを足すこと。** 決めたこと 4 の捨てた案。
- **`andon issue ready` で PR の無い課題を `review` にすること。** 決めたこと 8 の捨てた案。
- **片付いた前の課題に印を書き足すこと。** 決めたこと 11。

## 受け入れ条件（テストできる形。どの継ぎ目でテストするかを先に）

継ぎ目は、今ある検査の置き場を使う。値の読み方は Go の単体の検査、command の振る舞いは `tests/*.sh`、手引きの文は `tests/package.sh` で見る。どの条件も、直しを戻すと落ちることを担当が確かめて報告に書く。

1. **schema（`bash tests/run --changed` の中の `tests/schema.sh`）**: `schema/issue-keys.schema.json` が `deliverable: report` を通し、知らない値（`reports` など）を落とす。contract.md §2 の例が通る。見る出力は `tests/run` の表の `schema.sh` の行が `ok`。
2. **値の読み方（`go test ./internal/ledger/...`）**: 成果の形を返す関数が、Chief of Staff の書いた `report` を `report`、`candidates` を `candidates`、値の無い課題と知らない値を `pr` と返す。担当の端末が書いた `report` は `pr` と返す。Chief of Staff が `report` を書いたあとに起き直して端末が替わっても、`report` と返す。依頼書の Settings の写しが `Deliverable: report` になる。
3. **やめの見分け（`go test ./internal/dispatcher/ -run TestWaitedGone`）**: 待つ相手が報告の課題で、`done`・結果の記録が `none`・`pr` 無しのとき、`abandoned` にも `unchecked` にも入らない。同じ記録で `deliverable` の無い課題と、担当の端末が `report` を書いた課題は、今までどおり `abandoned` に入る。報告の課題が閉じた PR を持つときも入らない。
4. **gate を回さない（`bash tests/andon-dispatcher.sh`）**: 報告の課題が、`report.json` に `pr` が無く、報告の commit が既定の branch に入っていない状態で `review` に入る。gate は打たれず、担当への fix も merge の記録も無く、通知 `report` が積まれる。同じ状態の PR の課題には、今までどおり fix が届く。PR の課題で報告の commit が既定の branch に入っている場合は、今までどおり gate が打たれない。報告の課題で `pr` があれば gate が打たれる。担当の端末が `report` を書いた課題は、PR の課題と同じに扱われる。
5. **手で打った gate（`bash tests/andon-gate.sh`）**: 報告の課題で `pr` が無いとき、終了コード 3 と理由 1 行が出て、gate の記録が書かれない。
6. **ready の案内（`bash tests/andon-issue.sh`）**: 報告の課題で `pr` が無いとき、`andon issue ready` は終了コード 1 で、理由に `kanban review` が出て、`--pr` が出ない。`pr` があれば PR の課題と同じ確かめを通る。
7. **起動（`bash tests/andon-issue.sh`）**: `deliverable=report` の課題に、`andon issue start --dry-run` が出す依頼書の写しが `Deliverable: report` になる（直す前は `Deliverable: pr` に書き直す）。求める予約キーは PR の課題と同じである。
8. **手引きと skill（`bash tests/package.sh`）**: 検証の雛形の Settings に `- Deliverable: report` があり、雛形の下に `kanban set <id> deliverable=report` がある。`brief.md` の表と worker skill の表に `report` の行がある。
9. **当てたあとの検証（`.claude/skills/verify-andon/` の gate の機能）**: `gt-skip` が、当てたあとに PR 無しで `review` に入った報告の課題を、報告の commit を見ずに数え、fix を受けた形が無いことを見る。数えるのは、`deliverable=report` を書いた行より後の `review` の行だけである（印の無い間に正しく fix を受けた行を、あとから書いた印で数えない）。実装をセルに当てたあとの検証の課題が、この機能を動かす。
10. **セルの本物（オーナーが見る。`manual`）**: 実装がセルに当たったあと、印のある検証の課題を待つ課題が引き取れるようになったとき、Chief of Staff に届く `ready` の行に「待つ相手 … がやめになった」が付かない。待つ課題が無ければ、付かなかったことは見られない。

## 価値の仮説

無し: ラインの中の見分けの直しで、オーナーが見る画面と問いは変わらない。誤った句が付いたのは 2 回で（事実 7）、数で効きめを見る信号が無い。効いたかは、受け入れ条件の検査と、当てたあとの検証で確かめる。

## 依頼書に切る案（縦に薄く・依存だけ。切るのは Chief of Staff）

1. **値 `report` を足し、やめの見分けが読む。** 決めたこと 2〜6・10〜12（照合は決めたこと 10 の読み方で、値の読み方の関数に入れる）。`schema/issue-keys.schema.json`、`internal/ledger`（値の読み方と Settings の写し）、`internal/dispatcher/abandoned.go`、contract.md §2 の `deliverable` の行と新しい節（決めたこと 12 の表）・起動の前の確かめの段・§6 の `ready` の行、dispatcher.md §2.2、design.md と ledger.md の予約キーの表、用語集、worker skill の表、`brief.md` の表、検証の雛形（`verify.md`）と点検の課題の起動の 1 行。[ADR-0006](0006-survey-fix-decide.md) の索引の 7 の行の「今」に、値 `report` をこの ADR が足したことを添える。受け入れ条件 1・2・3・7・8。これだけで、誤った句は止まる。
2. **gate の PR 無しの扱いが読む（1 のあと）。** 決めたこと 7・8・13。`internal/issue/merge.go`、`internal/gate/gate.go`、`internal/issue/issue.go` の ready の理由、contract.md §4 の `andon issue ready` の段、dispatcher.md の P6a の表、手引き `watch.md` の通知 `report` の行、検証の地図の `gt-skip`（`features/gate.md` と `scripts/journeys/gate.sh`）、モデルと工程の注釈。受け入れ条件 4・5・6・9。
3. **Chief of Staff の手（1 がセルに当たったあと）。** 当たる前には `Deliverable: report` を書かない（決めたこと 11）。当たったあと、動いている検証の課題があれば `kanban set <id> deliverable=report` を打つ。マージ後のレビューを次に切るときは、依頼書に `Deliverable: report` を書く。

## 開いている問い（残った問い。決断の種類つき）

無し。

## 出典

- オーナーの発言と答え（2026-11-25）: 印を予約キーで持たせる問いへの答え、PR #630 の見分け方の問いへの答え、このテーマの前提と範囲の答え。最後の確かめの答え（2026-11-26）。
- PR #630（PR を出さない課題に gate が push を求める fix を送らない）と、その担当の報告の残したこと。
- 待つ相手がやめになったと知らせる句を直す課題の担当の報告（取り下げた直しと、本物の記録の反例）。
- 振り返りの報告（同じ見分け方を 2 つの課題が別々に聞いたこと）。
- [ADR-0006](0006-survey-fix-decide.md) 決めたこと 7・17、[ADR-0025](0025-formal-model-in-repo.md) やらないこと 5、[ADR-0035](0035-pstack-correct-and-verification-skill.md)、[ADR-0047](0047-idle-slots-run-upkeep.md) 決めたこと 18、[ADR-0062](0062-later-owner-waits.md) 決めたこと 3。
- [contract.md](../contract.md) §2・§6、[dispatcher.md](../dispatcher.md) §2.2 と P6a の表、[ledger.md](../ledger.md) の Routines。
- `internal/dispatcher/abandoned.go`・`internal/dispatcher/dispatcher.go`・`internal/issue/merge.go`・`internal/issue/issue.go`・`internal/issue/start.go`・`internal/issue/finish.go`・`internal/gate/gate.go`・`internal/ledger/ledger.go`、`package/skills/chief-of-staff/playbooks/verify.md`。
