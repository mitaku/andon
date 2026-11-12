# トリアージの signal の項目: 課題に切られたかは `links` の `url` に加えて信号の `key` でも照らす

- 日付: 2026-11-12
- 出典: 課題 A-0168（A-0027 の報告の Next）。ADR-0011 決めたこと 6 の反論の追記 (a)

- `andon triage collect` は、台帳のどれかの課題の `links` に信号の `url` か `key`（github-ci は `<repo>/<workflow>@main`、github-issues は `<repo>#<n>`）があれば、その信号を「課題に切られた」として項目に出さない。
- 理由: github-ci の記録の `url` は run ごとに変わる。課題に切ったあとに同じ workflow がまた落ちると、`links` の `url` は古い run を指し、url だけの照合では切った信号がトリアージに出てしまう。run の URL からは workflow を引けないので、変わらない `key` を `links` に持たせる。
- Chief of Staff は信号から切った課題の `links` に `url` と `key` の両方を書く（chief-of-staff skill §1「`signal` の通知を受けたら」）。ADR-0011 の追記 (a)（`links` に `url` を持つ）はそのまま守り、`key` を足すだけ。
- 捨てた案: 信号の記録に過去の run の url を貯めて照らす（記録の形が変わり、どこまで貯めるかを決める必要がある）／run の url の repo だけで照らす（同じ repo の別の workflow の失敗まで隠れる）。
