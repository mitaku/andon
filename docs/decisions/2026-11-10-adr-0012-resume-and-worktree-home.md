# ADR-0012: 止まった担当を resume で起こし直す・worktree の置き場を `/workspace/.worktrees` へ

- 日付: 2026-11-10
- 出典: ADR-0012（[adr/0012-resume-and-worktree-home.md](../adr/0012-resume-and-worktree-home.md)。Chief Engineer の課題 A-0034。オーナーが対話の中で決めた）
- 覆す: [adr/0005-cell-repo.md](../adr/0005-cell-repo.md) 決めたこと 2 の表の worktree の置き場（`~/.herdr/worktrees/`・建屋の外）。ADR-0005 決めたこと 7 として同日に改めた

セルが起き直して `doing` の担当・レビュアー・Chief Engineer の端末が素のシェルに戻ったとき、新しい command `andon issue resume <id> [--dry-run] [--fresh]` が台帳の `worktree` のパスに worktree を戻し（担当は branch、レビュアーは親の `review_request.commit` の detached）、元の cwd でハーネスのセッションを id で続ける（Claude 系は `start` が決めた `--session-id` を台帳の `session` に持ち `--resume <id>`、Codex 系は記録の先頭行の `cwd`・`id` から引いて `resume <id>`。`<profile>-resume` の template）。記録が無ければ起こさず非 0。起こすのは dispatcher で、Gone（無い・live でない・素のシェル）の `doing` を見たら1つの死につき1回打ち、記録だけの通知 `resumed` を書く。また死ねば今どおり `finished` を届け、人の go で `--fresh`（同じ worktree・branch で新しいセッション。最初のメッセージに `report.md` を読む1行）。`start` は「新しく作る」のまま。`andon chief-engineer` は戻った端末を片付けない（片付けるのは resume と finish）。resume・finish・start は同じ lock を取る。worktree の置き場は `/workspace/.worktrees/<repo 名>/<slug>-<id>`（置き場に `.gitignore` の `*`。`finish` は旧の置き場の値も1版だけ受ける）。担当向けの指針はセルの repo の見える所に1ファイル置き、Andon が最初のメッセージで両ブランドとも明示して指す（置き場の上の `AGENTS.md` にはしない。Codex は worktree の根より上を読まない）。決めたこと 10 件・やらないこと 7 件・切る案 R1〜R7 は ADR が正。
