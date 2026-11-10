# セルの wiki を LLM Wiki の手法に乗せる（ADR-0016）

- 日付: 2026-11-10
- 出典: [ADR-0016](../adr/0016-cell-wiki-llm-wiki.md)（人の決定 2026-11-10。Chief Engineer の課題 A-0084）

セルの `wiki/` を、不変の元資料の層（`raw/`: 人の発言の写し・人が持ち込む資料・finish した課題の報告と手直しの写し）と、Chief of Staff が育てる wiki の層（`sources/`・`entities/`・`topics/`・`overview.md`）に分け、`index.md`（目録）と `log.md`（追記だけの時系列）を置く。操作は ingest（元資料を置いたその場で1件ずつ）・query（index → ページ。人への良い答えは `topics/` に戻す）・lint（ingest 約10件ごとと人が頼んだとき）。書き手は Chief of Staff のまま、境界（学びと志向は wiki・仕様と作法は repo・決定は ADR）も保つ。skill §11 の「1件1ファイル」「`log.md` は作らない」を覆す。中身・理由・捨てた案は ADR が正。
