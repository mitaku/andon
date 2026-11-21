# オーナーが GitHub でマージした review の課題は、dispatcher が見回りで拾って片付ける

- 日付: 2026-11-09
- 出典: オーナーの依頼（2026-11-09。「マージ検知されるようになるといいな」）。A-0021
- 覆された: [2026-11-19-settle-repolls-closed-pr.md](2026-11-19-settle-repolls-closed-pr.md)（CLOSED の扱いだけ）

`merge: person`・gate の `needs_human` の課題をオーナーが GitHub でマージしたら、dispatcher が見回りで PR の状態を見て、自分でマージしたときと同じ道（`andon issue finish` → `done` の行 → 影響とオーナーの手直し）に乗せる（[dispatcher.md](../dispatcher.md) §2.7）。

- 見るのは `review` にいて `report.json` に `pr` がある課題だけで、`gh pr view` は PR ごとに1分に1回まで。dispatcher はマージしない。gate の判定も変えない。
- MERGED なら、その課題の gate の問い（`from: gate`）を「オーナーがマージした（<PR>）」と答えて受け取りまで閉じ、finish して通知 `merged`（`by: person`）。dispatcher は問いに答えないが、自分が出した gate の問いを事実で閉じるのは答えではなく記録として扱う。
- gate の外の問い（担当の `ask` など）が開いていれば片付けず、通知 `merged`（`held`）で1回知らせる（finish の拒みを黙って越えない）。問いが閉じたら次の巡で片付ける。
- CLOSED（マージなし）は片付けず、通知 `closed` を1回だけ。やめるかは Chief of Staff とオーナーが決める。
- gate の記録の `merged: true` は dispatcher がマージした印のまま（オーナーのマージでは書かない）。
