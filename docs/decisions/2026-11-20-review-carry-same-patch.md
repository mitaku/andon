# 衝突の無い rebase で head だけが変わったら、PR の差分が同じレビューを今の head に引き継ぐ

- 日付: 2026-11-20
- 出典: 振り返りが拾った繰り返し。衝突の無い rebase で head だけが変わり、差分が同じなのにレビューを頼み直した PR が直近7日で6件あった（mitaku/andon#288・mitaku/andon#305・mitaku/andon#335・mitaku/andon#338・mitaku/andon#345・mitaku/andon#348）。2回のレビューの結果が残る5件のうち4件は、2回が見た commit の PR の差分の patch-id が同じだった。

照合（`ledger.CheckReview`）は、最新のレビューの `findings.json` の `commit` が head と違っても、PR の差分が同じなら `current` として head に引き継ぐ。

- PR の差分は、その commit と `origin/HEAD`（既定の branch）の merge-base から commit までの diff とする。`git diff --binary --full-index` を `git patch-id --stable` に通した値が同じなら同じ差分とみなす。git は課題の worktree（`issue.yaml` の `worktree`）で打つ。
- 衝突を解いて差分が変わった rebase は patch-id が違うので、今までどおり `stale` になる。
- worktree が無い、commit が無い、`origin/HEAD` が引けない、差分が空、のどれかのときは同じとみなさない。引き継がずに頼み直すほうへ倒す。
- 引き継いでも、記録の `review.commit` はレビューが見た commit のまま。指摘の数え方（`must_fix`・`waived`・`preexisting`・前の回）は変えない。
- `andon issue review-request` は、照合が引き継いだレビューを読めるなら頼まない（終了コード 1 と理由1行）。担当は報告を書き直して `andon issue ready` へ進む。dispatcher はレビュアーを起動しない。
- 照合はゲート・`andon issue ready`・`andon issue show`・Stop フックで同じなので、どれも同じ答えになる。

引き受けたリスク:

- main 側の変更と組み合わさって初めて壊れる場合を、レビューは見なくなる。そこは CI とゲートの checks が見る。

捨てた案:

- **担当が `report.json` に「差分が同じ」と書く**: 申告でゲートを緩めることになる。patch-id は記録から機械が決められる。
- **dispatcher がレビュアーを起動する前に patch-id を比べる**: レビュアーを起動しなくても、照合は古い commit のまま stale で止まり、ready とゲートにも同じ比べ方が要る。照合の1か所で引き継げば、頼み直しもそこから決まる。
