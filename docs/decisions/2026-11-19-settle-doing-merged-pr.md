# doing のうちにオーナーがマージした PR も settle が片付ける

- 日付: 2026-11-19
- 出典: オーナーの決断（2026-11-19）。問い「doing のうちにオーナーがマージした PR も settle が片付けるべきか」に、片付けると答えた

- `andon issue settle` は `review` の課題に加えて `doing` の課題の PR も見る。dispatcher は見回りで `doing` の課題（`report.json` に `pr` がある、インナーループを通る課題）にも settle を打つ。GitHub に聞くのは今までどおり PR ごとに1分に1回。
- MERGED なら `review` のときと同じ道（gate の問いを閉じて finish → `done`・端末・worktree を片付ける。通知 `merged`〈`by: person`〉）。受け取られていない問い・commit していない変更があれば片付けず、通知 `merged`（`held` に理由）を理由ごとに1回だけ届ける。
- `review` のときの動きは変えない。settle の記録は `review` と `doing` の間を移っても残す（聞いた時刻も残るので、移った直後の巡では聞き直さない）。
