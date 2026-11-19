# ゲートの PR のコメントと commit status、外の Issue への書き戻しを、外の人が読む文にする

- 日付: 2026-11-19
- 出典: ADR-0026 決めたこと 9（[adr/0026-pr-rules-for-outside-readers.md](../adr/0026-pr-rules-for-outside-readers.md)）・ADR-0011 決めたこと 19
- 覆す: [2026-11-12-finish-write-back.md](2026-11-12-finish-write-back.md) のうち、書き戻しの本文（課題 id・PR の URL・結果）の 1 点だけ

- ゲートが PR に書くコメントと commit status（`andon/gate`）の説明文は、記録から作った外向けの文にする。内輪の語（`needs_human`・`tier`・`delegation`・`report.json`・`issue.yaml`・条件表）を使わない。頭の `andon gate: ` は動かさない（dispatcher がゲートのコメントを見分ける頭）。判定ごとの見出しは「確かめが通った変更です」「作った側が直してから出し直す変更です」「人の判断が要る変更です」。
- ゲートの記録（`gate/<head>.json` の `reasons[]`・`tier_reasons[]`）の文は変えない。外向けの文の表に無い記録の文は「そのほかの理由」と書き、外へそのまま出さない。
- 外の Issue への書き戻しの本文から課題 id を外す。3 つの場合（マージされた・マージされずに閉じられた・PR が無い）で文を分け、PR があれば URL を添える。書く先・回数・状態を変えないことは前のまま。
