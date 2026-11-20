# boot は外の skill の一覧にある skill だけを上げ、一覧の外の利用者の skill は上げない

- 日付: 2026-11-20
- 出典: ADR-0038 決めたこと 11・19 の実装の PR の独立レビュー（既定の一覧が空でなくなり、gh にログインした全部のセルで `gh skill update --all` が一覧の外の skill まで上げる、の指摘）
- 覆す: [2026-11-11-boot-bundled-and-gh-skills.md](2026-11-11-boot-bundled-and-gh-skills.md) の「毎起動 `gh skill update --all` を打つ」（一部。上げる範囲だけ）

boot の起動は、外の skill の一覧（パッケージの `gh-skills.txt` とセルの `local/gh-skills.txt`）の skill のうち、Claude Code か Codex の置き場にあるものの名前を渡して `gh skill update --all <名前…>` を打つ。名前を渡さない `gh skill update --all` は、利用者が自分で入れた一覧の外の skill も上げるため。一覧の skill がどの置き場にも無いとき（install が失敗したときなど）は update を打たない。gh は入っていない名前だけを渡すと失敗で返すのと、名前を渡さない形に戻ると一覧の外まで上げるため。版を固定しないこと、起動のたびに上げることは変えない。
