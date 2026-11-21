# dispatcher と gate が担当へ送る文の先頭に、送り手の名前を角括弧で1つ置く

- 日付: 2026-11-20
- 出典: オーナーの言葉（2026-11-20。印は文の先頭に `[dispatcher]` や `[gate]` のような角括弧の名前を1つ置くだけにし、hc の包みは使わない）
- 覆す: [2026-11-19-retro-collect.md](2026-11-19-retro-collect.md) の「オーナーが打った文」のうち、定型の文を送る側の定数（`dispatcher.RetryText`・`issue.FixMarker`）で照らすこと

- dispatcher と gate（`andon issue merge` の差し戻し）は担当・レビュアーの端末の外から `hc send` するので、文に hc の包みが付かず、受け手のセッションログではオーナーが打った文と同じ印になる。振り返りの入力に、gate の差し戻しの文がオーナーの文として混ざった。
- 担当・レビュアーへ送る文には、どれも先頭に送り手の名前を角括弧で1つ置く。dispatcher は `[dispatcher] `（再開の文・最初のメッセージの送り直し・レビュアーの最初のメッセージの送り直し）、gate は `[gate] `（差し戻し）。文の中身と送る時機は変えない。
- `andon retro collect` は、送る側の定数との照らしをやめ、この印で機械の文を外す。印の無い古い文は、印が付く前のセッションログにだけ残る。
- Chief of Staff へ届ける `<news>` の形は変えない。
- 約束は [dispatcher.md](../dispatcher.md) §5 の「送り手の印」と [contract.md](../contract.md) §20 の「除く行」。
