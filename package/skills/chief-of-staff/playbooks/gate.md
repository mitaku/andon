---
読む時: ゲートが人を要する（`<id> asked` で `from: gate`）・approve で止まった・上限を超えた回（dispatcher がレビュアーを起動しなかった）・差し戻しのとき
消す条件: needs_human の理由ごとの振り分け（2）は「ゲートの問いの JSON が理由ごとに人へ出す文と推奨案を持つようになったら」。差し戻しの数え方は「台帳が差し戻しの回数を数え、2回目で切り直しを通知に出すようになったら」
---

# レビューとゲート（人の判断が要るときだけ）

**インナーループは dispatcher が回す**（`docs/dispatcher.md` §5）。Chief of Staff はターンを使わない:
担当の `andon issue review-request` → dispatcher がレビュアーを起動する（`kanban add --parent`・detached worktree・`hc run`。依頼書の宛先は担当）→ レビュアーの結果は担当へ届く →
担当の `andon issue ready` → dispatcher が `andon gate` を回す → `approve` ＋ `merge: auto` ＋ 委譲の範囲内ならマージして `andon issue finish` ／ `fix` は担当へ返す ／ `needs_human` は問い（`from: gate`）。
ゲートは PR の head の GitHub の checks が揃うまで `wait`（終了コード 5）で、**通知も問いも出ない**。dispatcher が見回りのたびにゲートから回し直す。CI が1つも無いまま 30 分たつと `needs_human`（`asked` で届く）。
誰がどの端末をどう起動するかの一覧は `docs/operate.md` の「役の配置と起動」。
Chief of Staff が決めるのは、依頼書の Settings（`brief.md`。`review`・`reviewer`・`review_rounds`・`merge`）だけ。レビュアーを手で起動しない・`andon gate` を手で回さない・指摘を中継しない。

## `needs_human` のとき（`<id> asked` で `from: gate`）

1. `issues/<id>/library/questions/<n>.json`（`facts` に理由・PR・ゲートの記録のパス）と、記録 `library/gate/<head>.json` の `reasons[]` を読む。
2. 理由が委譲（`delegation: merge=… < tier …`）だけなら照合は通っている。人にマージを頼む（3）。
   ほかの理由（conflict・base・作者・予約キーを信じられない・Must fix の見送り・同じ head で2度目の Must fix・ゲートが判定できない）は、人へ（ask-one-by-one skill の1問ずつ）。
   直せば済むと人が答えたら、担当へ `hc send` で理由を返す（担当が `kanban start` で戻して直し、同じ手順で返す）。
3. 人にマージを頼む。課題の番号（題）と PR の URL を書き、何が変わったかを言う。レビューの回数と結果、見送った指摘があればそれも、それぞれ文で書く。**Chief of Staff はマージしない**（`gh pr merge` は managed settings で打てない）。
   人がマージしたら、dispatcher が次の見回り（1分以内）で拾い、この問いを「人がマージした」と閉じて片付ける（`finish.md`）。`kanban answer`・`kanban received` を手で打たない。

## approve で止まったとき（`<id> gate: approve` のあと `merged` が来ない）

`merge: person`、または `merge: auto` を最後に書いたのが Chief of Staff でない。人にマージを頼む（上の3と同じ形）。人がマージしたあとは `finish.md`。

## 上限を超えた回（`<id> report` で、dispatcher がレビュアーを起動しなかった）

`report.json` の `review_request` の `round` が `review_rounds` を超えた（`extra: true`）。起動するかは Chief of Staff が決める:
上限を使い切っても critical / major が残っているなら起動する——`kanban set <id> review_rounds=<round>` を打てば、dispatcher が次の見回りで起動する（見るのは `previous..commit` の差分だけ）。
minor 以下だけが残っているなら起動せず、見送りに残して `andon issue ready` へ進むよう担当に `hc send` で返す。追加の回でも同じ指摘が続いて収まらないと見たら人へ（1問ずつ）。

## 差し戻し（2回で通らなければ切り直す）

差し戻し＝担当の成果を通さずに返すこと（ゲートの `fix`・人の差し戻し）。レビューの回（`review_rounds`）は担当の工程なので数えない。
担当の日報（`library/journal.md`）は検収の判定に使わない。日報の中身（無い・短い・不満）を理由に差し戻さない。
記録は依頼書への追記で、その数が回数:

1. 返す前に、依頼書（`issues/<id>/brief.md`）の末尾に `## 差し戻し（<日付>）` と理由を1行足す。
   ゲートの `fix` は dispatcher が担当へ返し済みなので、`<id> gate: fix` の行を受けたら足すだけ。人の差し戻しは足してから `hc send <agent> "<理由>"`。
2. 依頼書に `## 差し戻し` が2つあってまだ通らない（3回目を返すことになる）なら、手直しを重ねない（advisor に相談してから決める。`cut.md` の「人の入口」）:
   依頼書を直した新しい課題を `kanban add`（「なぜ」に元の番号（題）と差し戻しの要点）し、人に go を取って元の課題を `andon issue finish <id>`、PR を `gh pr close <n>`（branch は残る）。
   go を待つ間は担当へ `hc send <agent> "手を止めて。依頼書を直して切り直す"`。新しい課題は `start.md` の手順で起動する。
