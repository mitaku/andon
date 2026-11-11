# `kanban tick` は dispatcher だけが 1 巡の 3 と 4 の間で打つ（cron は置かない）

- 日付: 2026-11-11
- 出典: ADR-0007 決めたこと 5・切る案 R1（C1）。課題 A-0066。推奨で進めたもの（人の答えは PR のマージ）

- `andon dispatcher` が、Chief of Staff の端末が生きている台帳でだけ、1 巡の 3（台帳を読む）と 4（通知の元を拾う）の間に `kanban tick` を 1 回打つ（dispatcher.md §2.1.1 の 3a）。Chief of Staff がいない台帳は巡ごと飛ばすので撃たない。留守の間の時刻は、戻った最初の巡で 1 度にまとめて撃たれる（台帳の決まり）。
- `tick` が足した課題は、3 の写しには出ないが 4a の `ready` の差で同じ巡に `ready`（`detail.routine` つき）として届く。
- `tick` の失敗（読めないルーチン・lock が取れない）はログに 1 行で、巡は止めない。
- 走らせ手は dispatcher だけ。cron は置かない。二重に撃たない守りは台帳の側（`kanban/.lock` と `state/routines.json`）にあり、人が手で打つ分もそれで守られる。dispatcher.md §7.3 の G4 を消し、§7.2 の #8 に移した。ledger.md の「cron で 5 分ごと」の一文も Andon の形に直した。

捨てた案: 見回りの最初（0 の前）に打つ（Chief of Staff がいない台帳でも撃ち、誰も読まない課題が積まれる）。4 の後に打つ（足した課題が次の巡まで届かない）。
