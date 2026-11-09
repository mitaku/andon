# 人が GitHub でマージした review の課題は、dispatcher が見回りで拾って片付ける

- 日付: 2026-11-09
- 出典: 人の依頼（2026-11-09。「マージ検知されるようになるといいな」）。A-0021

`merge: person`・ゲートの `needs_human` の課題を人が GitHub でマージしたら、dispatcher が見回りで PR の状態を見て、自分でマージしたときと同じ道（`andon issue finish` → `done` の行 → 影響と人の手直し）に乗せる（[dispatcher.md](../dispatcher.md) §2.7）。

- 見るのは `review` にいて `report.json` に `pr` がある課題だけで、`gh pr view` は PR ごとに1分に1回まで。dispatcher はマージしない。ゲートの判定も変えない。
- MERGED なら、その課題のゲートの問い（`from: gate`）を「人がマージした（<PR>）」と答えて受け取りまで閉じ、finish して通知 `merged`（`by: person`）。dispatcher は問いに答えないが、自分が出したゲートの問いを事実で閉じるのは答えではなく記録として扱う。
- ゲートの外の問い（担当の `ask` など）が開いていれば片付けず、通知 `merged`（`held`）で1回知らせる（finish の拒みを黙って越えない）。問いが閉じたら次の巡で片付ける。
- CLOSED（マージなし）は片付けず、通知 `closed` を1回だけ。やめるかは Chief of Staff と人が決める。
- ゲートの記録の `merged: true` は dispatcher がマージした印のまま（人のマージでは書かない）。
