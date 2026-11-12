# プロバイダに切られた Chief of Staff は StopFailure の asyncRewake のフックが合図で起動する。回数は控えの横に持ち、Stop で数え直す

- 日付: 2026-11-12
- 出典: ADR-0019 決めたこと 10（4 の続き）。A-0135 の実測（形 B）。課題 A-0156

- `package/settings.json` の `StopFailure` に `/opt/andon/hooks/stop-failure`（`asyncRewake: true`・`timeout: 600`）を置き、`andon dispatcher rewake` を打つ。`timeout` は最後の待ち（8 分）より長い値。
- 回数は控えの横の `<slug>.json.rewake` に持ち、Chief of Staff の `Stop`（`drain`）が消す。理由: StopFailure の入力からはターンが通ったかが分からず、通ったターンの終わりに必ず走るのが `drain` だから。数え直さないと、上限に一度届いたあと二度と起動できない。
- 5 回目のログは `<slug>.json.rewake.log` に1行。StopFailure の出力と終了コード 0 の stderr は Claude Code が捨て、dispatcher のログ（dispatcher の端末の stderr）にはフックから書けないため。
- 待ったあと、端末が `error` のままで回数も自分の書いた値のままのときだけ合図する。理由: 待つ間に人が打ったターンが通って `done` になったあとに合図すると、人の依頼が終わったあとに古い会話を起動し、外への操作を重ねかねない（レビュー A-0156-r1 の指摘）。人のターンがまた切られたら、次のフックが回数を進めて引き継ぐ。読めなければ起動しない。
- dispatcher が Chief of Staff へ「続けて」を打つ経路は W5（A-0134）で外れたまま戻さない。dispatcher の見回りが切られた Chief of Staff をログに1行書くのは残す（人が dispatcher のログで経緯を追える）。
