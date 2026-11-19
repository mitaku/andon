# 人が Chief Engineer の文書の PR をマージしたら、dispatcher が settle で片付ける

- 日付: 2026-11-19
- 出典: 人の指摘（2026-11-19）「マージの検知がちゃんとできてなさそう」。同じ日に4回、人が Chief Engineer の文書の PR をマージしても課題が `review` のまま残り、その課題のあとに並べた実装の課題も起動できなかった

- dispatcher は見回りで、`review` にいる Chief Engineer の課題（`profile: chief-engineer`。repo がある）にも `andon issue settle` を打つ。片付け方は担当の課題と同じ（問いを閉じて finish → `done`・端末・worktree）。
- ゲートは回さない。レビュアーも起動しない（検収は人が対話の中でした）。
- `doing` の Chief Engineer の課題は見ない。人と話している途中で文書の PR をマージしても、Chief Engineer の端末を片付けない。
- 片付けた通知 `merged`（`by: person`）は、Chief Engineer の課題に限って Chief of Staff に届ける。担当の課題では記録だけで、片付いたことは影響（`impact`）が届けるが、Chief Engineer の課題に影響は回らない。
- レビュアーの課題・repo の無い課題・PR の無い調べる課題の扱いは変えない。
