# settle は閉じた PR も聞き続け、開き直してマージされたら片付ける

- 日付: 2026-11-19
- 出典: レビューの回と gate とマージの動きを状態の図とモデル検査で調べた作業で、本物のコードで再現した破れ（閉じた PR をオーナーが開き直してマージしても、課題が review のまま片付かない）
- 覆す: [2026-11-09-dispatcher-picks-person-merge.md](2026-11-09-dispatcher-picks-person-merge.md) の「CLOSED は通知 `closed` を1回だけ」

- `andon issue settle` は、CLOSED と記録した PR も、課題が `review` か `doing` にいる間は OPEN と同じく PR ごとに1分に1回 GitHub に聞き直す。もう聞かないのは MERGED と分かった PR だけ。
- 開き直してマージされたら、MERGED の道（gate の問いを閉じて finish、通知 `merged`〈`by: person`〉）で片付ける。
- 通知 `closed` は閉じるたびに1回。閉じたままなら二度知らせない。開き直されて OPEN に戻ったあとまた閉じたら、もう一度知らせる。
- 片付けないこと、やめるかを Chief of Staff とオーナーが決めることは変えない（[dispatcher.md](../dispatcher.md) §2.7）。
