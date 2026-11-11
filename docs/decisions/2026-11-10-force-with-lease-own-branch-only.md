# 担当の force-with-lease は自分の PR の branch にだけ go なしで許し、main に当たる形は managed の deny で断る

- 日付: 2026-11-10
- 出典: 人の決定（2026-11-10）。課題 A-0116（許可ルール）・A-0130（main の deny）

- 担当が rebase のあとに自分の PR の branch へ `git push --force-with-lease` するのは、人の go なしで許す（`package/settings.json` の allow `Bash(git push --force-with-lease *)`）。`--force` は許さない。
- main への force push は不可。allow はどの refspec にも当たるので、`--force-with-lease` を含む `git push` のうち main に当たる形（`main`・`+main`・`<src>:main`・`refs/heads/main` を、`--force-with-lease` の前でも後でも。`--all`・`--mirror` も）を managed の deny で断る。当たる・当たらないの例は `tests/package.sh`。
- 他人の branch への force push も不可だが、名前で見分ける形が無いので許可ルールでは断らず、決まり（worker skill §3）で守る。
- 許可ルールは Claude Code が書いた命令の文に当てるもので、境界ではない（`git -C . push …`・自分が main にいるときの `git push --force-with-lease` は当たらない）。サーバーの側は GitHub の ruleset `main`（force push と削除の禁止）が受け持つ。
