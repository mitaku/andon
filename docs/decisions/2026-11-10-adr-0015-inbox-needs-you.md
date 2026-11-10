# ADR-0015: App の inbox を Needs you と Projects（役への入口）の 2 つに絞る

- 日付: 2026-11-10
- 出典: ADR-0015（[adr/0015-inbox-needs-you.md](../adr/0015-inbox-needs-you.md)。Chief Engineer の課題 A-0074。オーナーが対話の中で決めた。元は人の気付き 2026-11-10「Done にも Working にも興味がない」「Needs you とプロジェクトで動く Agent へのアクセスが担保されていればよい」「それこそが本当の Andon」）

Andon 同梱の App の mod `inbox` は、人が手を動かすもの（Needs you: `blocked` の端末と、Chief of Staff・Chief Engineer が終えたターンの返事）と、プロジェクトで動く役への入口（Projects: プロジェクトごとに Chief of Staff → Chief Engineer。idle でも常に出る）の 2 つの節だけを出す。担当・レビュアーは `blocked` のときだけ出し、ターンを終えた・働いている・idle の担当は出さない（俯瞰は kanban-board と Spaces）。Done の節・Working／Idle／Settled の棚・settle（✓・`d`・`u`・`ctrl-shift-d`・ラベル `settled=`）・ボードのダイアログ（`ctrl-shift-u`・`mod.json` の `overlay`）を消し、mod は hc に何も書かない。役が Needs you のときは行が Needs you に上がり、見たら Projects に戻る。dispatcher は Projects に出さない。新しい約束（ラベル・schema・台帳）は足さない。範囲は inbox mod だけ（dispatcher の通知のノイズ A-0036・kanban-board・andon mod・レイアウトの割りは触らない）。決めたこと 7 件（7 は仮）・切る案 I1〜I3 は ADR が正。
