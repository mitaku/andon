# andon issue report は、HEAD が同じ間は review_request を引き継ぐ

- 日付: 2026-11-20
- 出典: AGENTS.md の守らせるものが「無し」の行の棚卸し。文だけの決まりを仕組みへ移す（[ADR-0035](../adr/0035-pstack-correct-and-verification-skill.md) 決めたこと 7）。レビュー依頼のあとに報告を書き直すと依頼が消え、同じ commit の頼み直しは断られて担当が止まったことがある。

`andon issue report` は、`HEAD` が前の `report.json` の `commit` とも `review_request.commit` とも同じなら、`review_request` を引き継ぐ（`pr`・`estimate`・`plan` と同じ形）。`HEAD` が進めば引き継がない。repo の無い課題も引き継がない。

- `--data` に `review_request` は今までどおり書けない。書くのは `andon issue review-request` だけ。
- AGENTS.md の「レビュー依頼中に報告を書き直したら、同じ commit で `andon issue review-request` を打ち直す」の行は消した。
- 前の `report.json` が壊れていても、`pr`・`estimate`・`plan` を全部渡せば今までどおり書き直せる（そのときは引き継がない）。
- dispatcher は報告の行が増えると、起動しなかった回（上限を超えた回など）を拾い直す。引き継いだ依頼でも同じで、同じ理由で起動しないなら通知 `report` をもう1つ届ける。起動した回は、台帳のレビュアーの子課題で引き継ぐので二重に起動しない。

捨てた案:

- **`HEAD` を見ずにいつも引き継ぐ**: `HEAD` が進んだあとも古い commit の依頼が残り、dispatcher が PR の head と違う commit のレビュアーを起動しに行って断る。Stop フックも古い依頼を待ちと読む。
