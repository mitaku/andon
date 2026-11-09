# ADR-0011: 信号の受け口と価値のループ（ADR-0006 前提 11 の「信号の口」と「本番を次の信号にする線」を動かす）

- 日付: 2026-11-09
- 出典: ADR-0011（[adr/0011-signals-and-value-loop.md](../adr/0011-signals-and-value-loop.md)。Chief Engineer の課題 A-0018。オーナーが対話の中で決めた）
- 覆す: [adr/0006-survey-fix-decide.md](../adr/0006-survey-fix-decide.md) 前提 11 の「範囲の外」のうち、信号の口と本番からの戻りの2つ（重みづけする常駐と「後悔しないマージの率」は範囲の外のまま）

外の出所（GitHub の main の CI の失敗と issue から。Sentry・Linear・Cloudflare の Workers Issues は形だけ）が自分からラインに入る受け口をセルの層に足す。取りに行く形で、dispatcher の1巡に `andon signal collect` を入れ、記録は `<slug>.signals.json`（`schema/signal.schema.json`。1フィンガープリント1件・本文は持たない）。重さは出所ごとにオーナーが `project.json` の `signals[]` に書き（`now`／`later`。schema の版は上げない）、`now` は通知 `kind: signal` で Chief of Staff にすぐ届いて直す課題に（フィンガープリントごとに届けた回を記録に持つ）、`later` は記録だけでトリアージの項目（`untrusted`）に。トークンは `andon signal auth` がセルの中に置き、守りは scope（Sentry・Cloudflare は読み取り専用）。課題が done になったら `write_back: true` の出所へ finish が1コメントを書く（状態は変えない。規約7 の言い直し）。Chief Engineer の文書の「価値の仮説」は必須（6 項目か「無し: 理由」）。Chief of Staff が切るとき観察の課題を足し、台帳の新しいキー `not_before`（見る日。`kanban ready` を隠すだけ）を置く。担当は `report.json` の `observation`（`hit`／`miss`／`unknown`・証拠）で返し、ラインは結果で何もせず Chief of Staff がオーナーに1行。measure の先頭に価値の行。決めたこと 18 件・やらないこと 9 件・切る案 S1〜S9 は ADR が正。
