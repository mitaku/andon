---
読む時: gate がオーナーを要する（`<id> asked` で `from: gate`）・approve で止まった・上限を超えた回（dispatcher がレビュアーを起動しなかった）・差し戻しのとき
消す条件: needs_human の理由ごとの振り分け（2）は「gate の問いの JSON が理由ごとにオーナーへ出す文と推奨案を持つようになったら」。差し戻しの数え方は「kanban が差し戻しの回数を数え、2回目で切り直しを通知に出すようになったら」
---

# レビューと gate（オーナーの判断が要るときだけ）

**インナーループは dispatcher が回す**（`docs/dispatcher.md` §5）。Chief of Staff はターンを使わない:
担当の `andon issue review-request` → dispatcher がレビュアーを起動する（`kanban add --parent`・detached worktree・`hc run`。依頼書の宛先は担当）→ レビュアーの結果は担当へ届く →
担当の `andon issue ready` → dispatcher が `andon gate` を回す → `approve` ＋ `merge: auto` ＋ 委譲の範囲内ならマージして `andon issue finish` ／ `fix` は担当へ返す ／ `needs_human` は問い（`from: gate`）。
gate は PR の head の GitHub の checks が揃うまで `wait`（終了コード 5）で、**通知も問いも出ない**。dispatcher が見回りのたびに gate から回し直す。CI が1つも無いまま 30 分たつと `needs_human`（`asked` で届く）。
誰がどの端末をどう起動するかの一覧は `docs/operate.md` の「役の配置と起動」。
Chief of Staff が決めるのは、依頼書の Settings（`brief.md`。`review`・`reviewer`・`review_rounds`・`merge`）だけ。レビュアーを手で起動しない・`andon gate` を手で回さない・指摘を中継しない。
名指しの委譲（`andon delegation grant merge <A|B|C> --issues <id,…>`）は gate が回る前に効かせる。名指しされた課題を、担当の `andon issue ready` の前に `merge=auto` にする。gate がもう決めた課題には head が動くまで効かず、担当が `one_way` を申告した課題には効かない（どちらも今までどおりオーナーのマージ）。

## `needs_human` のとき（`<id> asked` で `from: gate`）

1. `issues/<id>/library/questions/<n>.json`（`facts` に理由・PR・gate の記録のパス）と、記録 `library/gate/<head>.json` の `reasons[]` を読む。
2. 理由が委譲（`delegation: merge=… < tier …`）だけなら照合は通っている。gate は委譲の範囲を最後に見るので、検査・CI・レビュー・受け入れ条件は済んでいる（`docs/contract.md` §7 の「決定式の順」）。オーナーにマージを頼む（3）。
   ほかの理由（conflict・base・作者・予約キーを信じられない・Must fix の見送り・同じ head で2度目の Must fix・gate が判定できない）は、オーナーへ（ask-one-by-one skill の1問ずつ）。
   gate が判定できなかった問いの推奨は「Chief of Staff が原因を直してから回し直す」（印が無いと、必須の check でオーナーもマージできない）。原因を直したら担当に `andon issue ready` を打ち直させ、gate を回し直す。
   直せば済むとオーナーが答えたら、担当へ `hc send` で理由を返す（担当が `kanban start` で戻して直し、同じ手順で返す）。
3. オーナーにマージを頼む。ほかの問いと同じく返事の本文に書いて終える（ask-one-by-one skill §2）。オーナー待ちの PR の印（`andon/gate`）は緑で、説明文の括弧に理由がある（印の読み方は `docs/operate.md` の「PR の印の読み方」）。課題の番号（タイトル）と PR の URL を書き、何が変わったかを言う。CI の結果も書く。レビューの回数と結果、見送った指摘があればそれも、それぞれ文で書く。後ろで待つ課題があれば「この後ろで N 件が待っている」を書き足す（数え方は ask-one-by-one skill §2 の 1）。**Chief of Staff はマージしない**（`gh pr merge` は managed settings で打てない）。
   オーナーがマージしたら、dispatcher が次の見回り（1分以内）で拾い、この問いを「オーナーがマージした」と閉じて片付ける（`finish.md`）。問いの答えと受け取りを手で書かない。
   マージ待ちの PR の rebase を担当に頼んだときも、この問いを手で閉じない。担当の `andon issue ready` は前の head の問いを数えずに通り、gate が今の head で決めたときに dispatcher が閉じる（今の head でもオーナーへ回れば新しい問いが来る）。
4. マージの依頼を出した課題はオーナー待ちになり、枠から外れる。続けて `start.md` の 1 で枠を数え直し、空いていれば起動できる課題を起動する（オーナーのマージを待たない）。

## PR を止める（オーナーに待つよう頼むとき）

別の問い（理由の課題。オーナーの課題の判断や Chief Engineer の検討）の答えが出るまで、ある課題の PR をマージさせたくないときに使う（`docs/contract.md` §2 の `hold`）。頼む文だけではマージは止まらない。

1. オーナーに待つよう頼む前に `andon issue hold <id> <理由の課題>` を打つ。成功で返ってから、オーナーに待つよう伝える（返る前は、印がまだ緑のことがある）。失敗で返ったら、理由の1行をそのままオーナーに伝え、待つよう頼まない（`hold` は書いたままで、dispatcher が次に PR を読むとき印を黄に揃える）。`kanban set <id> hold=…` で直に書かない（印を黄にするのが次の巡まで遅れる）。
2. 止めている間、gate は回らず、dispatcher はマージしない。PR の印は黄（「メンテナーの確認を待っています」）で、オーナー待ちの一覧はこの PR をマージの依頼に出さず、理由の課題の行の後ろに数える。gate が判定したあとに止めたら、その判定と gate の問いは dispatcher が捨てる。担当は止めを外せない（担当が `hold` を消しても止めたまま）。
3. 答えが出たら、答えを担当へ渡し、担当がそれを直しに入れて push し `andon issue ready` を打ってから、`andon issue unhold <id>` を打つ。答えが「直さない」なら、担当へ伝えたうえですぐ外してよい。外すと次の巡で gate が今の head で回り直して印を書く。理由の課題が done になっても止めは外れない。外し忘れは dispatcher が知らせる（`watch.md` の通知の表）。
4. レビュー中・止めている PR の URL を、マージの依頼と同じ返事に並べない（オーナーがどれを押してよいか取り違える）。

## approve で止まったとき（`<id> gate: approve` のあと `merged` が来ない）

`merge: person`、または `merge: auto` を最後に書いたのが Chief of Staff でない。オーナーにマージを頼む（上の3と同じ形）。枠は上の4と同じく数え直す。オーナーがマージしたあとは `finish.md`。

## 上限を超えた回（`<id> report` で、dispatcher がレビュアーを起動しなかった）

`report.json` の `review_request` の `round` が `review_rounds` を超えた（`extra: true`）。起動するかは Chief of Staff が決める:
上限を使い切っても critical / major が残っているなら起動する——`kanban set <id> review_rounds=<round>` を打てば、dispatcher が次の見回りで起動する（見るのは `previous..commit` の差分だけ）。
minor 以下だけが残っているなら起動せず、見送りに残して `andon issue ready` へ進むよう担当に `hc send` で返す。追加の回でも同じ指摘が続いて収まらないと見たらオーナーへ（1問ずつ）。

## 差し戻し（2回で通らなければ切り直す）

差し戻し＝担当の成果を通さずに返すこと（gate の `fix`・オーナーの差し戻し）。レビューの回（`review_rounds`）は担当の工程なので数えない。
担当の日報（`library/journal.md`）は検収の判定に使わない。日報の中身（無い・短い・不満）を理由に差し戻さない。
記録は依頼書への追記で、その数が回数:

1. 返す前に、依頼書（`issues/<id>/brief.md`）の末尾に `## 差し戻し（<日付>）` と理由を1行足す。
   gate の `fix` は dispatcher が担当へ返し済みなので、`<id> gate: fix` の行を受けたら足すだけ（届くのは担当へ送れなかったときだけ。`watch.md` の表の `gate: fix` の行）。オーナーの差し戻しは足してから `hc send <agent> "<理由>"`。
2. 依頼書に `## 差し戻し` が2つあってまだ通らない（3回目を返すことになる）なら、手直しを重ねない（advisor に相談してから決める。`cut.md` の「オーナーの入口」）:
   依頼書を直した新しい課題を `kanban add`（「なぜ」に元の番号（タイトル）と差し戻しの要点）し、オーナーに go を取って元の課題を `andon issue finish <id>`、PR を `gh pr close <n>`（branch は残る）。
   go を待つ間は担当へ `hc send <agent> "手を止めて。依頼書を直して切り直す"`。新しい課題は `start.md` の手順で起動する。
