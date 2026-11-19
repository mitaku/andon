# 調べる課題は deliverable の照合が外れていても PR を求めない

- 日付: 2026-11-19
- 出典: [ADR-0006](../adr/0006-survey-fix-decide.md) 決めたこと 17（同じ ADR の小さな改め）。Chief of Staff の起き直しをまたぐ動きを状態の図にし、本物のコードで再現して見つけた破れ

- `deliverable` が `candidates` なら、`andon issue ready` と dispatcher は履歴の照合を見ずに調べる課題として読む。ready は PR とレビューを要らず `candidates.json` を確かめ、dispatcher はゲートを回さずレビュアーも起動しない。`andon gate` を手で打てば今までどおり終了コード 3 で、照合が外れていればそのことを理由に書く。
- 照合で見送ったレビュー依頼は、Chief of Staff が予約キーを書き直して照合が戻れば、dispatcher が次の見回りで拾い直す。担当の頼み直しを待たない。
