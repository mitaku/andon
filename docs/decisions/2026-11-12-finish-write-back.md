# finish の書き戻しは GitHub の issue へ `gh issue comment` の1コメントだけ。PR の状態が読めなければ書かない

- 日付: 2026-11-12
- 出典: ADR-0011 決めたこと 15・受け入れ条件 13。課題 A-0030（S9）

- `andon issue finish` は課題を done にしたあと、`links` のうち `write_back: true` の `github-issues` の出所の repo（`repo` が無ければ `project.json` の `repos` の全部）の issue の URL ごとに、`gh issue comment` で1コメント書く。本文は課題 id・PR の URL・結果（MERGED なら「マージ」、それ以外は「閉じ」）。状態は変えない。
- 書く先は今は GitHub の issue だけ。Linear は読む command が無い（ADR-0011 決めたこと 3。使う案件が出たときに足す）ので、書き戻しもそのときに足す。
- PR の状態が読めない・`gh issue comment` が落ちたときは書かずに1行出し、finish は戻さない（片付けは済んでいる。結果を書き違えるより書かないほうを選ぶ）。done の課題には finish を打てないので、書き直しの道は無く、手で書く。
- レビュアーの課題（`profile` が `*-review`）の finish では書かない（元の課題の finish が書く）。
