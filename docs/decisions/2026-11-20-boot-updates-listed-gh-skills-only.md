# boot は外の skill の一覧にあって出所が一覧の repo と同じ skill だけを上げ、ほかの skill は上げない

- 日付: 2026-11-20
- 出典: ADR-0038 決めたこと 11・19 の実装の PR の独立レビュー 2 回（既定の一覧が空でなくなり、gh にログインした全部のセルで `gh skill update --all` が一覧の外の skill まで上げる、の指摘。名前で絞っても、gh は全部の置き場から同じ名前の別の出所の skill を拾う、の指摘）
- 覆す: [2026-11-11-boot-bundled-and-gh-skills.md](2026-11-11-boot-bundled-and-gh-skills.md) の「毎起動 `gh skill update --all` を打つ」（一部。上げる範囲だけ）

boot の起動は、Claude Code と Codex の置き場ごとに `gh skill update --all --dir <置き場> <名前…>` を打つ。渡す名前は、外の skill の一覧（パッケージの `gh-skills.txt` とセルの `local/gh-skills.txt`）の skill のうち、その置き場にあって、`SKILL.md` の frontmatter の `metadata.github-repo`（`gh skill install` が書く出所）が一覧の repo と同じものだけ。名前を渡さない `gh skill update --all` は、利用者が自分で入れた一覧の外の skill も上げる。名前だけを渡しても、gh は全部の置き場（ほかのエージェントの置き場も含む）から同じ名前の skill を拾い、別の出所から入れた利用者の skill も上げる。`--dir` を渡すと、gh はその置き場だけを見る（gh 2.103.3 で dry-run で確かめた）。渡す名前が無い置き場には update を打たない。gh は入っていない名前だけを渡すと失敗で返すため。版を固定しないこと、起動のたびに上げることは変えない。
