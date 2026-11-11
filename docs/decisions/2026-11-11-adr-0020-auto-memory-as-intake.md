# ADR-0020: auto memory を全部の役に入れ、`/workspace/memory/` を受け皿にしてトリアージで wiki と repo へ蒸留する

- 日付: 2026-11-11
- 出典: ADR-0020（[adr/0020-auto-memory-as-intake.md](../adr/0020-auto-memory-as-intake.md)。Chief Engineer の課題 A-0143。オーナーが対話の中で決めた。元は人の発言 2026-11-11「/workspace 配下に memory を置けば永続化もされるし、定期的に蒸留するならいいのかもな」）
- 覆す: decisions.md の ADR-0004 の行のうち「memory は置かない」（ADR-0004 決めたこと 2・前提 7 の、ハーネスの auto memory を managed で止める部分。3つの家の分け方と、`projects/<slug>/` の下に `memory/` を作らない線は保つ）

エージェントの気づきを拾う口を広げるため、Claude Code の auto memory を全部の役（Chief of Staff・Chief Engineer・担当・レビュアー）に入れる。置き場はセルの repo の直下の `/workspace/memory/` で、git では追わない。memory は蒸留までの受け皿で、長く残る知識の正は wiki と repo のまま。蒸留はトリアージに乗せる（`andon triage collect` が memory の1件を `kind: memory` の項目として拾い、トリアージの担当が確かめて仕分け、Chief of Staff が反映して memory のファイルを消す）。周期は週1のトリアージと量の線（仮に 30 件）。wiki へ出すものだけ `wiki/raw/memory/` に言葉のまま写してから ingest する。書き方は環境メモに数行（確かめた事実と、条件と理由つきの学び。続き・決定・秘密は書かない。報告の `## Remember` は残す）。Codex の記憶は切ったまま（ADR-0007 R9 はそのまま進める）。入れる前にセルで実測し（置き場の形・同時書き込み・許可の確認）、managed settings の `allow` を足すのは要ると分かったときだけ。memory の設定を書くのは managed settings の1か所だけ。決めたこと 9 件・捨てた案（Chief Engineer の推奨から変えた 2 件を含む）・価値の仮説・切る案 M0〜M5 は ADR が正。
