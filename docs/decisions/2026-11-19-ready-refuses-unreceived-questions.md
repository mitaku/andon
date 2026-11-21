# andon issue ready は受け取られていない問いがあるとき断る

- 日付: 2026-11-19
- 出典: オーナーの決断（2026-11-19。「ready は開いた問いがあるとき断るべきか」に「断る」）

- `andon issue ready` は、その課題に受け取られていない問い（`open`・`answered`・読めない `questions.yaml`）があれば、`doing` のまま終了コード 1 と理由1行で断る。確かめの形は `kanban done`（closing）と `andon issue finish` と同じ（同じ関数で読む）。
- 理由: 問いを残したまま `review` に入ると、gate が approve して `merge: auto` でマージしたあと、片付け（finish）が問いで断られ、マージ済みで片付かない課題が残る。
- gate の判定は変えない（守りは入口の `ready` に置く）。
