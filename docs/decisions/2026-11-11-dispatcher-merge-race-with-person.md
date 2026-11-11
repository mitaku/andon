# dispatcher の `gh pr merge` が人の先のマージで失敗したら、同じ head なら問いにせず人のマージとして片付ける

- 日付: 2026-11-11
- 出典: ADR-0007 決めたこと 5・切る案 R2（A5・G3）。課題 A-0067。推奨で進めたもの（人の答えは PR のマージ）

- `merge: auto` の課題で dispatcher の `gh pr merge` が失敗したら、`gh pr view --json state,mergeCommit,headRefOid` で PR を読み直す。`MERGED` で、PR の head がゲートの記録の `head`（`--match-head-commit` に渡したもの）と同じなら、`irreversible` の問いを出さない。
- そのときは人がマージしたときの道（dispatcher.md §2.7）に渡す: 控えの `Watch` に MERGED とマージの commit を置き、同じ巡の見回りが GitHub に聞き直さずにゲートの問いを閉じて `andon issue finish` し、通知 `merged`（`by: person`）を積む。ゲートの外の問いが開いていれば §2.7 と同じく片付けずに知らせる。ゲートの記録の `merged` は書かない（dispatcher がマージした印のまま）。
- `MERGED` でない・head が違う・読み直せないときは、今までどおり問いにする。
- head を比べるため、読み直しの `gh pr view` には ADR-0007 の R2 の行にある `state,mergeCommit` に `headRefOid` を足した。
- dispatcher.md §7.3 の G3 を消し、§7.2 の #5 に移した。

捨てた案: 失敗を見たら何もせず、次の巡の §2.7 の見回り（1 分に 1 回の `gh pr view`）に任せる（問いは出ないが、片付けが最大 1 分遅れ、「なぜ失敗したか」を読まないまま黙る）。MERGED なら head を見ずに片付ける（ゲートが判定していない commit で人がマージした場合も黙って片付けてしまう）。
